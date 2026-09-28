package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"
)

func journalSample(t *testing.T) (*Survival, []byte, kernel.PortableHead) {
	t.Helper()
	s := runSurvivalTest(t, SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 4, EatWeight: 1})
	data, err := s.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := s.kernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return s, data, head
}

func TestSurvivalJournalRoundTripAndFullRunCounts(t *testing.T) {
	s, data, head := journalSample(t)
	var accepted, rejected, collisions int
	for _, b := range s.journal {
		for _, a := range b.Attempts {
			if a.Status == Accepted {
				accepted++
			} else {
				rejected++
				if a.Reason == SurvivalCollision {
					collisions++
				}
			}
		}
	}
	if accepted != len(s.kernel.Events()) || rejected == 0 || collisions == 0 {
		t.Fatalf("not a mixed journal: accepted=%d rejected=%d", accepted, rejected)
	}
	first, err := DecodeSurvivalJournal(data, s.kernel, head, s.sched.Time(), s.journalConfig())
	if err != nil || !reflect.DeepEqual(first, s.Journal()) {
		t.Fatalf("decoded journal differs: %v", err)
	}
	oldReport, err := s.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	schedulerWire, schedulerHead, err := s.sched.ExportPortable()
	if err != nil || schedulerHead != head {
		t.Fatalf("scheduler export: %v", err)
	}
	restored, gotHead, err := kernel.RestoreHistory(s.registry, mustHistory(t, s))
	if err != nil || gotHead != head {
		t.Fatalf("restored kernel: %v", err)
	}
	s.kernel = restored
	s.sched, err = scheduler.RestorePortable(restored, 4, s.evaluate, schedulerWire, head)
	if err != nil {
		t.Fatal(err)
	}
	s.journal = nil
	if err := s.RestoreJournal(data, head); err != nil {
		t.Fatal(err)
	}
	newReport, err := s.Report(0, 0)
	if err != nil || !reflect.DeepEqual(oldReport, newReport) {
		t.Fatalf("lost full-run counts: %v old=%+v new=%+v", err, oldReport, newReport)
	}
	again, err := s.ExportJournal()
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("noncanonical round trip: %v", err)
	}
}

func mustHistory(t *testing.T, s *Survival) []byte {
	t.Helper()
	b, _, err := s.kernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSurvivalJournalRejectsDifferentOriginalRunConfigWithoutMutation(t *testing.T) {
	s, data, head := journalSample(t)
	for _, tc := range []struct {
		name   string
		change func(*SurvivalOptions)
	}{
		{"weight", func(o *SurvivalOptions) { o.EatWeight = 0 }},
		{"hours", func(o *SurvivalOptions) { o.Hours = 13 }},
		{"seed", func(o *SurvivalOptions) { o.Seed = 8 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := SurvivalOptions{Actors: s.actors, Seed: s.seed, Hours: s.hours, Workers: 2, EatWeight: s.weight}
			tc.change(&opts)
			other, err := NewSurvival(opts)
			if err != nil {
				t.Fatal(err)
			}
			// At a direct restore boundary the independently restored kernel and
			// scheduler can match even if the constructed adapter config does not.
			other.kernel, other.sched = s.kernel, s.sched
			other.journal = []SurvivalBatch{{Time: 123}}
			before := other.Journal()
			if err = other.RestoreJournal(data, head); !errors.Is(err, ErrSurvivalJournal) || !reflect.DeepEqual(before, other.Journal()) {
				t.Fatalf("accepted mismatched original config or mutated journal: %v", err)
			}
			if got, err := DecodeSurvivalJournal(data, s.kernel, head, s.sched.Time(), other.journalConfig()); !errors.Is(err, ErrSurvivalJournal) || got != nil {
				t.Fatalf("decoded mismatched config: %v", err)
			}
		})
	}
	// Worker parallelism is not causal; a different worker count may read
	// the same original journal if the policy weight and horizon are equal.
	other, err := NewSurvival(SurvivalOptions{Actors: s.actors, Seed: s.seed, Hours: s.hours, Workers: 2, EatWeight: s.weight})
	if err != nil {
		t.Fatal(err)
	}
	other.kernel, other.sched = s.kernel, s.sched
	if err := other.RestoreJournal(data, head); err != nil || !reflect.DeepEqual(other.Journal(), s.Journal()) {
		t.Fatalf("noncausal worker count changed journal: %v", err)
	}
}

func TestSurvivalJournalRejectsOmittedFirstHourActorsAndSkippedHours(t *testing.T) {
	s, _, _ := journalSample(t)
	original := s.Journal()
	reset := func() {
		s.journal = make([]SurvivalBatch, len(original))
		for i, b := range original {
			s.journal[i] = b
			s.journal[i].Attempts = append([]SurvivalAttempt(nil), b.Attempts...)
		}
	}
	removeRejected := func(batch int) bool {
		for i, a := range s.journal[batch].Attempts {
			if a.Status == Rejected && a.Reason == SurvivalCollision {
				s.journal[batch].Attempts = append(s.journal[batch].Attempts[:i:i], s.journal[batch].Attempts[i+1:]...)
				return true
			}
		}
		return false
	}
	if !removeRejected(0) {
		t.Fatal("fixture lacks a first-hour collision")
	}
	if _, err := s.ExportJournal(); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatalf("omitted first-hour loser: %v", err)
	}
	reset()
	s.journal[1].Time += sim.SimTime(hour)
	for j := range s.journal[1].Attempts {
		s.journal[1].Attempts[j].Time += sim.SimTime(hour)
	}
	if _, err := s.ExportJournal(); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatalf("skipped hour accepted: %v", err)
	}
	reset()
	// No historical scheduler observations prove later rejected actor
	// coverage: the codec certifies accepted links and observed counts only.
	found := false
	for i := 1; i < len(s.journal); i++ {
		if removeRejected(i) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fixture lacks later collisions")
	}
	if _, err := s.ExportJournal(); err != nil {
		t.Fatalf("later rejection omission cannot be independently proven: %v", err)
	}
}

func TestSurvivalJournalEmptyBoundary(t *testing.T) {
	s, err := NewSurvival(SurvivalOptions{Actors: 2, Seed: 7, Hours: 2, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := s.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := s.kernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	journal, err := DecodeSurvivalJournal(wire, s.kernel, head, s.sched.Time(), s.journalConfig())
	if err != nil || len(journal) != 0 {
		t.Fatalf("empty boundary: %v %v", journal, err)
	}
}

func TestSurvivalJournalRejectedOnlyTimestampEvidence(t *testing.T) {
	// Zero energy produces a rejected Eat attempt in the normal step.
	s := oneEnergySurvival(t, 1, 1)
	var err error
	for i := range s.seeds {
		if s.seeds[i].Entity != 1 {
			continue
		}
		if s.seeds[i].Component == component.EnergyTypeID {
			s.seeds[i].Fields[0].Value = scalarUnit(0)
		}
		if s.seeds[i].Component == HungerTypeID {
			s.seeds[i].Fields[0].Value = scalarUnit(capacity)
		}
	}
	s.kernel, err = kernel.New(s.registry, 0, s.seeds)
	if err != nil {
		t.Fatal(err)
	}
	s.sched, err = newJournalTestScheduler(s)
	if err != nil {
		t.Fatal(err)
	}
	processed, err := s.Step(context.Background())
	if err != nil || !processed || len(s.kernel.Events()) != 0 {
		t.Fatalf("rejection-only step: %v %v", processed, err)
	}
	data, err := s.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := s.kernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSurvivalJournal(data, s.kernel, head, s.sched.Time(), s.journalConfig())
	if err != nil || len(decoded) != 1 || decoded[0].Time != sim.SimTime(hour) || decoded[0].TipID != 0 || decoded[0].Attempts[0].Status != Rejected || decoded[0].Attempts[0].EventID != 0 {
		t.Fatalf("lost rejection-only evidence: %+v %v", decoded, err)
	}
	s.journal[0].Attempts[0].EventID = 1
	if _, err := s.ExportJournal(); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatalf("rejected-only claimed accepted event: %v", err)
	}
}

func newJournalTestScheduler(s *Survival) (*scheduler.Scheduler, error) {
	q, err := scheduler.New(s.kernel, 1, s.evaluate)
	if err != nil {
		return nil, err
	}
	if err = q.Register(1); err != nil {
		return nil, err
	}
	return q, q.Schedule(scheduler.Wake{Actor: 1, At: sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold})
}

func TestSurvivalJournalRejectsRehashedCorruptionAndMismatchedHeads(t *testing.T) {
	s, data, head := journalSample(t)
	check := func(name string, wire []byte, k *kernel.Kernel, h kernel.PortableHead, at sim.SimTime) {
		t.Helper()
		if journal, err := DecodeSurvivalJournal(wire, k, h, at, s.journalConfig()); !errors.Is(err, ErrSurvivalJournal) || journal != nil {
			t.Fatalf("%s: accepted journal: %v", name, err)
		}
	}
	for _, n := range []int{0, 10, len(data) - 1, len(data) - sha256.Size} {
		check("truncated", data[:n], s.kernel, head, s.sched.Time())
	}
	flipped := bytes.Clone(data)
	flipped[len(flipped)-sha256.Size-3] ^= 1
	check("unhashed damage", flipped, s.kernel, head, s.sched.Time())
	check("scheduler time", data, s.kernel, head, s.sched.Time()+1)
	otherHead := head
	otherHead.SnapshotHash[0] ^= 1
	check("forged expected head", data, s.kernel, otherHead, s.sched.Time())
	other, err := NewSurvival(SurvivalOptions{Actors: 48, Seed: 8, Hours: 12, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	check("other kernel", data, other.kernel, head, s.sched.Time())
	// The digest is deliberately not an authenticity signature. Rehashing
	// after changing a canonical reference must still fail semantic checks.
	batchOffset := journalBatchOffset(s.ref)
	for _, tc := range []struct {
		name   string
		offset int
		value  byte
	}{
		{"rehashed batch time", batchOffset + 7, 0},
		{"rehashed version", batchOffset + 15, 9},
		{"rehashed tip", batchOffset + 23, 9},
		{"rehashed hash", batchOffset + 24, 9},
		{"rehashed attempt key", batchOffset + 8 + 8 + 8 + 32 + 2 + 8 + 1, 99},
		{"rehashed attempt event", journalFirstEventIDOffset(data, batchOffset), 99},
	} {
		wire := bytes.Clone(data)
		wire[tc.offset] ^= tc.value | 1
		rehashJournal(wire)
		check(tc.name, wire, s.kernel, head, s.sched.Time())
	}
}

func journalBatchOffset(ref strategy.Ref) int {
	return 4 + 4 + 32 + 8 + 32 + 8 + 8 + 8 + 32 + 32 + 32 + 8 + 2 + 8 + 2 + 8 + 1 + len(ref.ID) + 4 + 2
}
func journalFirstEventIDOffset(data []byte, batchOffset int) int {
	// batch header: time, version, id, hash, count; attempt: actor, key,
	// choice kind, target, observed version, score, evaluated, fallback,
	// status, reason, then event ID.
	keyAt := batchOffset + 8 + 8 + 8 + 32 + 2 + 8
	return keyAt + 1 + int(data[keyAt]) + 1 + 8 + 8 + 8 + 4
}
func rehashJournal(data []byte) {
	digest := sha256.Sum256(data[:len(data)-sha256.Size])
	copy(data[len(data)-sha256.Size:], digest[:])
}

func TestSurvivalJournalRejectsDuplicateOrderingAndInvalidChoices(t *testing.T) {
	s, _, _ := journalSample(t)
	original := s.Journal()
	checks := []struct {
		name   string
		mutate func()
	}{
		{"duplicate attempt", func() { s.journal[0].Attempts[1] = s.journal[0].Attempts[0] }},
		{"out of order attempts", func() {
			s.journal[0].Attempts[0], s.journal[0].Attempts[1] = s.journal[0].Attempts[1], s.journal[0].Attempts[0]
		}},
		{"duplicate batch time", func() { s.journal[1].Time = s.journal[0].Time }},
		{"missing batch", func() { s.journal = s.journal[1:] }},
		{"invalid actor", func() { s.journal[0].Attempts[0].Actor = 0 }},
		{"invalid target", func() {
			for i := range s.journal {
				for j := range s.journal[i].Attempts {
					a := &s.journal[i].Attempts[j]
					if a.Status == Accepted && a.Choice.Kind == strategy.Eat {
						a.Choice.Target = 0
						return
					}
				}
			}
		}},
		{"invalid choice", func() { s.journal[0].Attempts[0].Choice.Kind = 99 }},
		{"wrong policy ref", func() { s.journal[0].Attempts[0].Ref.Version++ }},
		{"wrong observed version", func() { s.journal[0].Attempts[0].Choice.ObservedVersion++ }},
		{"nonfinite score", func() { s.journal[0].Attempts[0].Choice.Score = math.Inf(1) }},
		{"rejected claims event", func() {
			for i := range s.journal {
				for j := range s.journal[i].Attempts {
					if s.journal[i].Attempts[j].Status == Rejected {
						s.journal[i].Attempts[j].EventID = 1
						return
					}
				}
			}
		}},
		{"accepted claims rejection", func() {
			for i := range s.journal {
				for j := range s.journal[i].Attempts {
					if s.journal[i].Attempts[j].Status == Accepted {
						s.journal[i].Attempts[j].Reason = SurvivalCollision
						return
					}
				}
			}
		}},
		{"accepted target mismatch", func() {
			for i := range s.journal {
				for j := range s.journal[i].Attempts {
					a := &s.journal[i].Attempts[j]
					if a.Status == Accepted && a.Choice.Kind == strategy.Eat {
						first := sim.EntityID(1001 + (int(a.Actor)-1)%4)
						second := sim.EntityID(1001 + int(a.Actor)%4)
						original := a.Choice.Target
						a.Choice.Target = first
						if original == first {
							a.Choice.Target = second
						}
						return
					}
				}
			}
		}},
		{"invalid reason", func() { s.journal[0].Attempts[0].Reason = 99 }},
		{"invalid status", func() { s.journal[0].Attempts[0].Status = 99 }},
		{"accepted bad ID", func() {
			for i := range s.journal {
				for j := range s.journal[i].Attempts {
					if s.journal[i].Attempts[j].Status == Accepted {
						s.journal[i].Attempts[j].EventID++
						return
					}
				}
			}
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			s.journal = s.Journal()
			tc.mutate()
			if _, err := s.ExportJournal(); !errors.Is(err, ErrSurvivalJournal) {
				t.Fatalf("accepted invalid journal: %v", err)
			}
			s.journal = original
		})
	}
}

func TestSurvivalJournalBudgetAndCanonicalDecoder(t *testing.T) {
	s, data, head := journalSample(t)
	if _, err := DecodeSurvivalJournal(make([]byte, MaxSurvivalJournalBytes+1), s.kernel, head, s.sched.Time(), s.journalConfig()); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatal("accepted oversized wire")
	}
	tooMany := s.Journal()
	for len(tooMany) <= MaxSurvivalJournalBatches {
		tooMany = append(tooMany, tooMany[len(tooMany)-1])
	}
	s.journal = tooMany
	if _, err := s.ExportJournal(); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatal("accepted overbudget batch count")
	}
	// A digest-valid oversized count fails before allocating its attempts.
	countOffset := journalBatchOffset(s.ref) + 8 + 8 + 8 + 32
	wire := bytes.Clone(data)
	binary.BigEndian.PutUint16(wire[countOffset:], 65535)
	rehashJournal(wire)
	if _, err := DecodeSurvivalJournal(wire, s.kernel, head, s.sched.Time(), s.journalConfig()); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatal("accepted overbudget attempt count")
	}
	wire = append(bytes.Clone(data[:len(data)-sha256.Size]), 0)
	digest := sha256.Sum256(wire)
	wire = append(wire, digest[:]...)
	if _, err := DecodeSurvivalJournal(wire, s.kernel, head, s.sched.Time(), s.journalConfig()); !errors.Is(err, ErrSurvivalJournal) {
		t.Fatal("accepted trailing byte")
	}
}
