package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
)

// Reproduction v4 evidence journal (ticket 14 persistence lane). The wire
// mirrors the capacity v3 seam: canonical framing around a canonical JSON
// body, a scheduler-aware decoder, and a replay that is deliberately the
// trusted production runner rather than raw rule admission. A journal digest
// detects damage, not forgery; the enclosing checkpoint authenticates the
// whole bundle and independently retains its verified kernel head. The
// configuration is run identity: wild yield, seed, births intervention flag,
// the frozen founder genome rows (which pin the seed-derived lifetimes and
// would expose any genome-variant root as a mismatch), and the pinned policy
// ref plus its exact content fingerprint. The neutral h0 world records no
// accepted birth-dependent action, so the kernel history alone cannot
// authenticate these values.
//
// Attempt coverage: every runner-recorded v4 attempt kind (gather, wait,
// eat-stored, eat, build, rest, paired-meal, fallback-eatstored,
// birth-request, birth-reply) is journaled and replayed attempt-for-attempt;
// the world-caused rules (401 produce, 404 basal, 406 yield, 407 wear, 412
// birth-expiry, 413 age-death) commit no attempt rows — they are verified
// through the full accepted event partition, whose every byte must equal the
// trusted replay, including rule 411 refusals (recorded as typed birth-reply
// evidence) and rule 410 entity-create births.
const (
	ReproductionJournalFormatVersion uint32 = 4
	MaxReproductionJournalBytes             = 8 << 20
	MaxReproductionJournalBatches           = reproductionStepLimit
	// One committed instant carries at most one attempt per roster entity;
	// the roster is bounded by the founders plus the frozen birth cap.
	MaxReproductionJournalAttempts = reproductionStepLimit * (ReproductionFounderCount + ReproductionMaxBirths)
)

var ErrReproductionJournal = errors.New("invalid reproduction evidence journal")

// Distinct v4 journal magic: no earlier pilot's journal (v1 AWFJ, S6/v2 AWSJ,
// v3 AWCJ) can decode as a reproduction v4 journal, and vice versa; the
// format versions are disjoint as well (1/2/3/4).
var reproductionJournalMagic = [4]byte{'A', 'W', 'R', 'J'}

// ReproductionFounderGenomeRow is the creation-time genome row of one founder
// in canonical run identity: the three neutral-or-variant loci, the
// seed-derived immutable lifetime (death-hour), and the frozen founding
// columns (absent parents, birth-hour 0) that validation pins exactly. The
// runtime columns died-hour and birth-granary-paid evolve after creation
// (rules 404/413/410); they are replayed from the journal, so the run-identity
// record always pins their creation values (-1 and 0).
type ReproductionFounderGenomeRow struct {
	Actor  sim.EntityID
	Genome ReproductionGenomeState
}

type ReproductionJournalConfig struct {
	Yield int64
	Seed  uint64
	// Births is the continuation flag: the recorded intervention on a branch
	// child, the runner's own flag otherwise. Boundary is the births flag of
	// the captured boundary's wake topology — the flag under which the h0
	// pulse committed. The two differ only on a Branch lineage, where the
	// shared h0 prefix was produced under Boundary and every later batch
	// under Births.
	Births            bool
	Boundary          bool
	Founders          []ReproductionFounderGenomeRow
	Policy            strategy.ReproductionRef
	PolicyFingerprint [32]byte
}

type ReproductionJournalCounts struct {
	GathersAdmitted, GathersDenied         uint32
	Eats, EatStored, Waits                 uint32
	Builds, PairedMeals, FallbackEatStored uint32
	Rests, NoStoredMealNotes               uint32
	BirthRequests, BirthReplies            uint32
	BirthsAccepted, BirthRefusals          uint32
	BirthRefusalsSilent, BirthWakeClaims   uint32
	LostBirths                             uint32
}

type reproductionJournalWire struct {
	Format        uint32
	Head          kernel.PortableHead
	SchedulerTime sim.SimTime
	Config        ReproductionJournalConfig
	Counts        ReproductionJournalCounts
	Batches       []ReproductionBatch
}

// reproductionMaxRoster is the frozen roster ceiling: sixteen founders plus
// the sixteen newborn slots.
const reproductionMaxRoster = ReproductionFounderCount + ReproductionMaxBirths

// reproductionNormalizeFounders is the canonical founder order: sorted by
// actor, never nil, so identical genome sets compare equal regardless of
// input order.
func reproductionNormalizeFounders(founders []ReproductionFounderGenomeRow) []ReproductionFounderGenomeRow {
	out := make([]ReproductionFounderGenomeRow, 0, len(founders))
	out = append(out, founders...)
	sort.Slice(out, func(i, j int) bool { return out[i].Actor < out[j].Actor })
	return out
}

// validReproductionFounderGenomes checks the frozen founder set: exactly the
// sixteen ID-placed founders in strict ascending order, alleles inside the
// frozen locus bounds, and lifetimes inside the frozen 96+mod48 span.
func validReproductionFounderGenomes(founders []ReproductionFounderGenomeRow) bool {
	if len(founders) != ReproductionFounderCount {
		return false
	}
	for i, founder := range founders {
		g := founder.Genome
		if founder.Actor != sim.EntityID(i+1) ||
			g.LocusM < ReproductionGenomeLocusMin || g.LocusM > ReproductionGenomeLocusMax ||
			g.LocusY < ReproductionGenomeLocusMin || g.LocusY > ReproductionGenomeLocusMax ||
			g.LocusW < ReproductionGenomeLocusMin || g.LocusW > ReproductionGenomeLocusMax ||
			g.ParentA != 0 || g.ParentB != 0 || g.BirthHour != 0 ||
			g.DeathHour < ReproductionLifetimeBase || g.DeathHour >= ReproductionLifetimeBase+ReproductionLifetimeSpan ||
			g.DiedHour != ReproductionNeverHour || g.BirthGranaryPaid != 0 {
			return false
		}
	}
	return true
}

func equalReproductionFounderGenomes(a, b []ReproductionFounderGenomeRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// expectedReproductionFounderGenomes derives the frozen founder genome rows
// from the seed exactly as NewReproduction does: the runner-owned founder
// stream (0x464F554E, positions 0-15) yields the immutable lifetimes and the
// loci start neutral. A genome-variant root would record different rows and
// can never match this derivation, so variants fail closed against the
// pinned neutral format.
func expectedReproductionFounderGenomes(seed uint64) ([]ReproductionFounderGenomeRow, error) {
	stream := sim.NewRandomStream(sim.RandomState{Seed: seed, Stream: ReproductionFounderStream})
	out := make([]ReproductionFounderGenomeRow, 0, ReproductionFounderCount)
	for i := 0; i < ReproductionFounderCount; i++ {
		lifetime := ReproductionLifetimeBase + int64(stream.Uint64()%ReproductionLifetimeSpan)
		a, err := ReproductionFounderState(sim.EntityID(i+1), lifetime)
		if err != nil {
			return nil, err
		}
		out = append(out, ReproductionFounderGenomeRow{Actor: sim.EntityID(i + 1), Genome: a.Genome})
	}
	return out, nil
}

// reproductionFounderGenomeRecords reads the immutable founder genome rows
// from one authoritative view.
func reproductionFounderGenomeRecords(view scheduler.SnapshotView) ([]ReproductionFounderGenomeRow, error) {
	out := make([]ReproductionFounderGenomeRow, 0, ReproductionFounderCount)
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		a, err := reproductionActorState(view, actor)
		if err != nil {
			return nil, err
		}
		genome := a.Genome
		genome.DiedHour = ReproductionNeverHour
		genome.BirthGranaryPaid = 0
		out = append(out, ReproductionFounderGenomeRow{Actor: actor, Genome: genome})
	}
	return out, nil
}

func reproductionJournalFingerprint() ([32]byte, error) {
	return strategy.ReproductionPolicyFingerprint(strategy.FrozenReproductionPolicy())
}

// journalConfig pins the reproduction journal's run identity under the
// coordinator lock. An altered same-ref policy must never export under the
// frozen fingerprint: the fingerprint is left at a value no decoder accepts.
func (f *Reproduction) journalConfig() ReproductionJournalConfig {
	fingerprint, err := reproductionJournalFingerprint()
	if err != nil || f.k == nil || f.bound[1] == nil || f.bound[1].Policy() != strategy.FrozenReproductionPolicy() {
		fingerprint = [32]byte{}
	}
	var founders []ReproductionFounderGenomeRow
	if f.k != nil {
		head := f.k.SnapshotHead()
		view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
		if recorded, err := reproductionFounderGenomeRecords(view); err == nil {
			founders = reproductionNormalizeFounders(recorded)
		}
	}
	return ReproductionJournalConfig{Yield: f.yield, Seed: f.seed, Births: f.births, Boundary: f.births,
		Founders: founders, Policy: f.ref, PolicyFingerprint: fingerprint}
}

// reproductionBoundaryBirths recovers the boundary flag of a committed
// journal: the h0 request instant closes a batch exactly when the h0 pulse
// ran births-enabled (the request wakes exist only then), so the journal
// itself records the flag its shared prefix was produced under. At the h0
// boundary itself (steps == 1) the instant has not closed yet, so captures
// pin the boundary from the runner's own flag instead.
func reproductionBoundaryBirths(journal []ReproductionBatch) bool {
	at, err := ReproductionPhaseTime(0, ReproductionPhaseBirthRequest)
	if err != nil {
		return false
	}
	for _, batch := range journal {
		if batch.Time == at {
			return true
		}
		if batch.Time > at {
			return false
		}
	}
	return false
}

// ExportJournal publishes committed evidence only, not a disk bundle. The
// coordinator is locked across the history and scheduler consistency checks,
// and the journal is reconciled against a trusted replay before publication.
// The reconciliation replays under the runner's own births flag and compares
// the scheduler bytes, which is exact for every live runner and for restores
// of roots and pulse-boundary continuations; a restored Branch child carries
// its parent's boundary topology, so its journal verifies through the
// history-aware decode and the checkpoint restore paths instead.
func (f *Reproduction) ExportJournal() ([]byte, error) {
	if f == nil {
		return nil, ErrReproductionJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return nil, ErrReproductionJournal
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		return nil, err
	}
	before, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || schedulerHead != head {
		return nil, ErrReproductionJournal
	}
	data, err := EncodeReproductionJournal(f.journal, f.k, head, f.sched.Time(), f.journalConfig())
	if err != nil {
		return nil, err
	}
	reference, err := replayReproductionJournal(f.journal, f.journalConfig())
	if err != nil {
		return nil, err
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != head || !bytes.Equal(before, want) {
		return nil, ErrReproductionJournal
	}
	after, afterHead, err := f.sched.ExportPortable()
	if err != nil || afterHead != head || !bytes.Equal(before, after) {
		return nil, ErrReproductionJournal
	}
	return data, nil
}

// RestoreJournal replaces only the journal and only after full validation.
// Kernel, scheduler and other runner state are checkpoint responsibilities.
// It will not silently import a journal from another seed, yield, births
// flag, founder genome set, or policy fingerprint.
func (f *Reproduction) RestoreJournal(data []byte, verified kernel.PortableHead) error {
	if f == nil {
		return ErrReproductionJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return ErrReproductionJournal
	}
	batches, err := DecodeReproductionJournalWithScheduler(data, f.k, verified, f.sched, f.journalConfig())
	if err != nil {
		return err
	}
	f.journal = batches
	return nil
}

// EncodeReproductionJournal validates accepted history and typed evidence at
// a closed timestamp. It does not certify open activities or scheduler wakes;
// ExportJournal and the scheduler-aware decoder do that at publication and
// restore.
func EncodeReproductionJournal(batches []ReproductionBatch, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config ReproductionJournalConfig) ([]byte, error) {
	if err := verifyReproductionJournal(batches, k, verified, schedulerTime, config); err != nil {
		return nil, err
	}
	wire := reproductionJournalWire{Format: ReproductionJournalFormatVersion, Head: verified, SchedulerTime: schedulerTime,
		Config: config, Counts: reproductionJournalCounts(batches), Batches: batches}
	body, err := json.Marshal(wire)
	if err != nil || len(body)+12+sha256.Size > MaxReproductionJournalBytes {
		return nil, ErrReproductionJournal
	}
	out := append([]byte(nil), reproductionJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, ReproductionJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// DecodeReproductionJournal validates canonical wire, run identity (including
// the births intervention flag, founder genome rows, policy ref and content
// fingerprint), typed request links and the entire accepted event partition.
// This is history-only partial validation: a schedulerTime cannot prove open
// fibers, activity tokens, pending wakes, or the runner's outstanding maps.
func DecodeReproductionJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config ReproductionJournalConfig) ([]ReproductionBatch, error) {
	if len(data) < 12+sha256.Size || len(data) > MaxReproductionJournalBytes || !bytes.Equal(data[:4], reproductionJournalMagic[:]) ||
		binary.BigEndian.Uint32(data[4:8]) != ReproductionJournalFormatVersion || int(binary.BigEndian.Uint32(data[8:12])) != len(data)-12-sha256.Size {
		return nil, ErrReproductionJournal
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return nil, ErrReproductionJournal
	}
	var wire reproductionJournalWire
	decoder := json.NewDecoder(bytes.NewReader(data[12:len(body)]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || wire.Format != ReproductionJournalFormatVersion || wire.Head != verified ||
		wire.SchedulerTime != schedulerTime || !sameReproductionJournalConfig(wire.Config, config) ||
		wire.Counts != reproductionJournalCounts(wire.Batches) {
		return nil, ErrReproductionJournal
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, data[12:len(body)]) || verifyReproductionJournal(wire.Batches, k, verified, schedulerTime, config) != nil {
		return nil, ErrReproductionJournal
	}
	return wire.Batches, nil
}

// DecodeReproductionJournalWithScheduler is the complete checkpoint seam. The
// scheduler must first be restored independently against the verified head.
// Its canonical portable state must match the trusted replay of these
// batches, including open activities, tokens, pending wakes (the reserved
// WakeBirth first claims included) and stopped fibers.
func DecodeReproductionJournalWithScheduler(data []byte, k *kernel.Kernel, verified kernel.PortableHead, sched *scheduler.Scheduler, config ReproductionJournalConfig) ([]ReproductionBatch, error) {
	if sched == nil {
		return nil, ErrReproductionJournal
	}
	before, head, err := sched.ExportPortable()
	if err != nil || head != verified {
		return nil, ErrReproductionJournal
	}
	batches, err := DecodeReproductionJournal(data, k, verified, sched.Time(), config)
	if err != nil {
		return nil, ErrReproductionJournal
	}
	reference, err := replayReproductionJournal(batches, config)
	if err != nil {
		return nil, ErrReproductionJournal
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != verified || !bytes.Equal(before, want) {
		return nil, ErrReproductionJournal
	}
	after, afterHead, err := sched.ExportPortable()
	if err != nil || afterHead != verified || !bytes.Equal(before, after) {
		return nil, ErrReproductionJournal
	}
	return batches, nil
}

func sameReproductionJournalConfig(a, b ReproductionJournalConfig) bool {
	return a.Yield == b.Yield && a.Seed == b.Seed && a.Births == b.Births && a.Boundary == b.Boundary &&
		a.Policy == b.Policy &&
		a.PolicyFingerprint == b.PolicyFingerprint &&
		equalReproductionFounderGenomes(reproductionNormalizeFounders(a.Founders), reproductionNormalizeFounders(b.Founders))
}

func reproductionJournalCounts(batches []ReproductionBatch) ReproductionJournalCounts {
	var c ReproductionJournalCounts
	for _, b := range batches {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "gather":
				if a.Accepted {
					c.GathersAdmitted++
				} else {
					c.GathersDenied++
				}
			case "eat":
				c.Eats++
			case "eat-stored":
				c.EatStored++
			case "wait":
				c.Waits++
			case "build":
				c.Builds++
			case "paired-meal":
				c.PairedMeals++
			case "fallback-eatstored":
				c.FallbackEatStored++
			case "rest":
				c.Rests++
			case "birth-request":
				c.BirthRequests++
			case "birth-reply":
				c.BirthReplies++
				switch a.Birth.Decision {
				case ReproductionReplyAccept:
					c.BirthsAccepted++
				case ReproductionReplyRefuse:
					c.BirthRefusals++
				case ReproductionReplyRefuseSilent:
					c.BirthRefusalsSilent++
				}
			default:
				continue
			}
			if a.NoMeal != "" {
				c.NoStoredMealNotes++
			}
			if a.BirthWake {
				c.BirthWakeClaims++
			}
			if a.Lost != "" {
				c.LostBirths++
			}
		}
	}
	return c
}

// Replay is deliberately the trusted production runner, not raw rule
// admission: the kernel alone accepts schema-valid counterfeit proposals.
// Every event body is compared, so all required entity/component/field
// deltas, before/after values, and allocation rows must agree with fresh
// checked proposals under the frozen policy — including rule 410's
// entity-create births and every world-caused pulse rule. A raw-kernel
// injection that bypasses the runner (a cross-owner mutation or a genome
// locus tamper under a rule that may write died-hour only) breaks the event
// stream here. Historical gather rejection labels are the exception: they
// are checked for type and admitted-vs-denied, but the allocator's denial
// reason is re-derived deterministically, so the recorded label must match
// exactly.
func verifyReproductionJournal(batches []ReproductionBatch, k *kernel.Kernel, verified kernel.PortableHead, at sim.SimTime, config ReproductionJournalConfig) error {
	if k == nil || config.Yield < 0 || config.Yield > ReproductionSlotsPerPatch ||
		config.Policy != strategy.FrozenReproductionPolicy().Ref ||
		!validReproductionFounderGenomes(config.Founders) ||
		!equalReproductionFounderGenomes(reproductionNormalizeFounders(config.Founders), mustReproductionFounderGenomes(config.Seed)) ||
		at < 0 || at > sim.SimTime(ReproductionHorizonHours)*sim.SimTime(ReproductionHour) || len(batches) > MaxReproductionJournalBatches {
		return ErrReproductionJournal
	}
	fingerprint, err := reproductionJournalFingerprint()
	if err != nil || fingerprint != config.PolicyFingerprint {
		return ErrReproductionJournal
	}
	_, head, err := k.ExportHistory()
	if err != nil || head != verified {
		return ErrReproductionJournal
	}
	reference, err := replayReproductionJournal(batches, config)
	if err != nil || reference.sched.Time() != at {
		return ErrReproductionJournal
	}
	_, expectedHead, err := reference.k.ExportHistory()
	if err != nil || expectedHead != verified {
		return ErrReproductionJournal
	}
	got, expected := k.Events(), reference.k.Events()
	if len(got) != len(expected) {
		return ErrReproductionJournal
	}
	for i := range got {
		original, e1 := got[i].Bytes()
		trusted, e2 := expected[i].Bytes()
		if e1 != nil || e2 != nil || !bytes.Equal(original, trusted) || got[i].Hash != expected[i].Hash || got[i].PreviousHash != expected[i].PreviousHash {
			return ErrReproductionJournal
		}
	}
	return nil
}

func mustReproductionFounderGenomes(seed uint64) []ReproductionFounderGenomeRow {
	out, err := expectedReproductionFounderGenomes(seed)
	if err != nil {
		return nil
	}
	return out
}

// replayReproductionJournal rebuilds a trusted reference runner from the
// pinned configuration and steps it through the recorded batches. The
// reference is also the source of the runner-only state (dynamic roster,
// reserved birth wakes, denial evidence, cumulative economics) that a
// checkpoint restore re-assembles; determinism makes it authoritative.
func replayReproductionJournal(batches []ReproductionBatch, config ReproductionJournalConfig) (*Reproduction, error) {
	if len(batches) > MaxReproductionJournalBatches {
		return nil, ErrReproductionJournal
	}
	// The reference replays the shared h0 prefix under the boundary flag and
	// applies the recorded intervention after the h0 pulse: a Branch lineage's
	// journal is reproducible from genesis only under that two-flag schedule,
	// and for every other bundle boundary and continuation flag are equal.
	reference, err := NewReproduction(ReproductionOptions{Yield: config.Yield, Seed: config.Seed,
		Workers: 1, Births: config.Boundary})
	if err != nil {
		return nil, ErrReproductionJournal
	}
	var count int
	for i, b := range batches {
		if len(b.Attempts) > reproductionMaxRoster || count+len(b.Attempts) > MaxReproductionJournalAttempts {
			return nil, ErrReproductionJournal
		}
		count += len(b.Attempts)
		processed, stepErr := reference.Step(context.Background())
		if stepErr != nil || !processed || !sameReproductionJournalBatch(b, reference.journal[i]) {
			return nil, ErrReproductionJournal
		}
		if i == 0 {
			reference.births = config.Births
		}
	}
	return reference, nil
}

func sameReproductionJournalBatch(got, expected ReproductionBatch) bool {
	if got.Time != expected.Time || got.Version != expected.Version || got.TipID != expected.TipID || got.TipHash != expected.TipHash || len(got.Attempts) != len(expected.Attempts) {
		return false
	}
	for i, a := range got.Attempts {
		// The reference replay re-derives every attempt deterministically from
		// the pinned seed, founder genomes and configuration, including
		// allocator denial reasons and birth evidence, so the recorded attempt
		// must match exactly.
		if a != expected.Attempts[i] {
			return false
		}
	}
	return true
}
