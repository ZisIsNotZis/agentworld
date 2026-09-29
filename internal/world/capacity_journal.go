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
	"reflect"
	"sort"
)

// Capacity v3 evidence journal (ticket 13 persistence lane). The wire mirrors
// the social-food v2 seam: canonical framing around a canonical JSON body, a
// scheduler-aware decoder, and a replay that is deliberately the trusted
// production runner rather than raw rule admission. A journal digest detects
// damage, not forgery; the enclosing checkpoint authenticates the whole bundle
// and independently retains its verified kernel head. The configuration is
// run identity: wild yield, seed, intervention flag, recorded founder
// endowments, and the pinned policy ref plus its exact content fingerprint.
// Zero capital flow at q0 carries no seed-dependent accepted action, so the
// kernel history alone cannot authenticate these values.
const (
	CapacityJournalFormatVersion uint32 = 3
	MaxCapacityJournalBytes             = 8 << 20
	MaxCapacityJournalBatches           = capacityStepLimit
	// One actor records at most three attempts per hour: an admitted gather
	// plus its build and paired meal, or a denial plus one fallback.
	MaxCapacityJournalAttempts = CapacityHorizonHours * CapacityActorCount * 3
)

var ErrCapacityJournal = errors.New("invalid capacity evidence journal")
var capacityJournalMagic = [4]byte{'A', 'W', 'C', 'J'}

type CapacityJournalConfig struct {
	Yield             int64
	Seed              uint64
	Enabled           bool
	Founders          []CapacityFounder
	Policy            strategy.CapacityRef
	PolicyFingerprint [32]byte
}

type CapacityJournalCounts struct {
	GathersAdmitted, GathersDenied         uint32
	Eats, EatStored, Waits                 uint32
	Builds, PairedMeals, FallbackEatStored uint32
	Rests, NoStoredMealNotes               uint32
}

type capacityJournalWire struct {
	Format        uint32
	Head          kernel.PortableHead
	SchedulerTime sim.SimTime
	Config        CapacityJournalConfig
	Counts        CapacityJournalCounts
	Batches       []CapacityBatch
}

// capacityNormalizeFounders is the canonical founder order: sorted by actor,
// never nil, so identical endowments compare equal regardless of input order.
func capacityNormalizeFounders(founders []CapacityFounder) []CapacityFounder {
	out := make([]CapacityFounder, 0, len(founders))
	out = append(out, founders...)
	sort.Slice(out, func(i, j int) bool { return out[i].Actor < out[j].Actor })
	return out
}

// capacityValidFounders checks the frozen founder bounds: roster actors only,
// strictly increasing (already normalized), inside the capital and granary
// ceilings. The capital endowment variant remains unreachable —
// NewCapacity rejects it through the frozen conservation — but the journal
// and manifest must still reject forged records with typed errors.
func capacityValidFounders(founders []CapacityFounder) bool {
	var previous sim.EntityID
	for _, founder := range founders {
		if _, err := CapacityActorPatchID(founder.Actor); err != nil || founder.Actor <= previous ||
			founder.Capital < 0 || founder.Capital > CapacityKMax ||
			founder.Granary < 0 || founder.Granary > CapacityGranaryMax {
			return false
		}
		previous = founder.Actor
	}
	return true
}

func capacityJournalFingerprint() ([32]byte, error) {
	return strategy.CapacityPolicyFingerprint(strategy.FrozenCapacityPolicy())
}

func (f *Capacity) journalConfig() CapacityJournalConfig {
	fingerprint, err := capacityJournalFingerprint()
	if err != nil || f.bound[0] == nil || f.bound[0].Policy() != strategy.FrozenCapacityPolicy() {
		// An altered same-ref policy must never export under the frozen
		// fingerprint; leave a fingerprint no decoder accepts.
		fingerprint = [32]byte{}
	}
	return CapacityJournalConfig{Yield: f.yield, Seed: f.seed, Enabled: f.enabled,
		Founders: capacityNormalizeFounders(f.founders), Policy: f.ref, PolicyFingerprint: fingerprint}
}

// ExportJournal publishes committed evidence only, not a disk bundle. The
// coordinator is locked across the history and scheduler consistency checks,
// and the journal is reconciled against a trusted replay before publication.
func (f *Capacity) ExportJournal() ([]byte, error) {
	if f == nil {
		return nil, ErrCapacityJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return nil, ErrCapacityJournal
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		return nil, err
	}
	before, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || schedulerHead != head {
		return nil, ErrCapacityJournal
	}
	data, err := EncodeCapacityJournal(f.journal, f.k, head, f.sched.Time(), f.journalConfig())
	if err != nil {
		return nil, err
	}
	reference, err := replayCapacityJournal(f.journal, f.journalConfig())
	if err != nil {
		return nil, err
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != head || !bytes.Equal(before, want) {
		return nil, ErrCapacityJournal
	}
	after, afterHead, err := f.sched.ExportPortable()
	if err != nil || afterHead != head || !bytes.Equal(before, after) {
		return nil, ErrCapacityJournal
	}
	return data, nil
}

// RestoreJournal replaces only the journal and only after full validation.
// Kernel, scheduler and other runner state are checkpoint responsibilities.
// It will not silently import a journal from another seed, yield, flag,
// endowment or policy fingerprint.
func (f *Capacity) RestoreJournal(data []byte, verified kernel.PortableHead) error {
	if f == nil {
		return ErrCapacityJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return ErrCapacityJournal
	}
	batches, err := DecodeCapacityJournalWithScheduler(data, f.k, verified, f.sched, f.journalConfig())
	if err != nil {
		return err
	}
	f.journal = batches
	return nil
}

// EncodeCapacityJournal validates accepted history and typed evidence at a
// closed timestamp. It does not certify open activities or scheduler wakes;
// ExportJournal and the scheduler-aware decoder do that at publication and
// restore.
func EncodeCapacityJournal(batches []CapacityBatch, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config CapacityJournalConfig) ([]byte, error) {
	if err := verifyCapacityJournal(batches, k, verified, schedulerTime, config); err != nil {
		return nil, err
	}
	wire := capacityJournalWire{Format: CapacityJournalFormatVersion, Head: verified, SchedulerTime: schedulerTime,
		Config: config, Counts: capacityJournalCounts(batches), Batches: batches}
	body, err := json.Marshal(wire)
	if err != nil || len(body)+12+sha256.Size > MaxCapacityJournalBytes {
		return nil, ErrCapacityJournal
	}
	out := append([]byte(nil), capacityJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, CapacityJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// DecodeCapacityJournal validates canonical wire, run identity (including the
// intervention flag, founder endowments, policy ref and content fingerprint),
// typed request links and the entire accepted event partition. This is
// history-only partial validation: a schedulerTime cannot prove open fibers,
// activity tokens, pending wakes, or the runner's outstanding activity maps.
// Gather denial labels are historical evidence, not a claim that rejected
// allocation reasons can be reconstructed from accepted events.
func DecodeCapacityJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config CapacityJournalConfig) ([]CapacityBatch, error) {
	if len(data) < 12+sha256.Size || len(data) > MaxCapacityJournalBytes || !bytes.Equal(data[:4], capacityJournalMagic[:]) ||
		binary.BigEndian.Uint32(data[4:8]) != CapacityJournalFormatVersion || int(binary.BigEndian.Uint32(data[8:12])) != len(data)-12-sha256.Size {
		return nil, ErrCapacityJournal
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return nil, ErrCapacityJournal
	}
	var wire capacityJournalWire
	decoder := json.NewDecoder(bytes.NewReader(data[12:len(body)]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || wire.Format != CapacityJournalFormatVersion || wire.Head != verified ||
		wire.SchedulerTime != schedulerTime || !sameCapacityJournalConfig(wire.Config, config) ||
		wire.Counts != capacityJournalCounts(wire.Batches) {
		return nil, ErrCapacityJournal
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, data[12:len(body)]) || verifyCapacityJournal(wire.Batches, k, verified, schedulerTime, config) != nil {
		return nil, ErrCapacityJournal
	}
	return wire.Batches, nil
}

// DecodeCapacityJournalWithScheduler is the complete checkpoint seam. The
// scheduler must first be restored independently against the verified head.
// Its canonical portable state must match the trusted replay of these batches,
// including open activities, tokens, wakes and stopped fibers.
func DecodeCapacityJournalWithScheduler(data []byte, k *kernel.Kernel, verified kernel.PortableHead, sched *scheduler.Scheduler, config CapacityJournalConfig) ([]CapacityBatch, error) {
	if sched == nil {
		return nil, ErrCapacityJournal
	}
	before, head, err := sched.ExportPortable()
	if err != nil || head != verified {
		return nil, ErrCapacityJournal
	}
	batches, err := DecodeCapacityJournal(data, k, verified, sched.Time(), config)
	if err != nil {
		return nil, ErrCapacityJournal
	}
	reference, err := replayCapacityJournal(batches, config)
	if err != nil {
		return nil, ErrCapacityJournal
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != verified || !bytes.Equal(before, want) {
		return nil, ErrCapacityJournal
	}
	after, afterHead, err := sched.ExportPortable()
	if err != nil || afterHead != verified || !bytes.Equal(before, after) {
		return nil, ErrCapacityJournal
	}
	return batches, nil
}

func sameCapacityJournalConfig(a, b CapacityJournalConfig) bool {
	return a.Yield == b.Yield && a.Seed == b.Seed && a.Enabled == b.Enabled && a.Policy == b.Policy &&
		a.PolicyFingerprint == b.PolicyFingerprint &&
		reflect.DeepEqual(capacityNormalizeFounders(a.Founders), capacityNormalizeFounders(b.Founders))
}

func capacityJournalCounts(batches []CapacityBatch) CapacityJournalCounts {
	var c CapacityJournalCounts
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
			default:
				continue
			}
			if a.NoMeal != "" {
				c.NoStoredMealNotes++
			}
		}
	}
	return c
}

// Replay is deliberately the trusted production runner, not raw rule
// admission: the kernel alone accepts schema-valid counterfeit proposals.
// Every event body is compared, so all required entity/component/field
// deltas, before/after values, and bag-source values must agree with fresh
// checked proposals under the frozen policy. A raw-kernel injection that
// bypasses the runner (including a cross-owner mutation) breaks the event
// stream here. Historical gather rejection labels are the exception: they are
// checked for type and admitted-vs-denied, but the allocator's denial reason
// is re-derived deterministically, so the recorded label must match exactly.
func verifyCapacityJournal(batches []CapacityBatch, k *kernel.Kernel, verified kernel.PortableHead, at sim.SimTime, config CapacityJournalConfig) error {
	if k == nil || config.Yield < 0 || config.Yield > CapacitySlotsPerPatch ||
		config.Policy != strategy.FrozenCapacityPolicy().Ref || !capacityValidFounders(config.Founders) ||
		at < 0 || at > sim.SimTime(CapacityHorizonHours)*sim.SimTime(CapacityHour) || len(batches) > MaxCapacityJournalBatches {
		return ErrCapacityJournal
	}
	fingerprint, err := capacityJournalFingerprint()
	if err != nil || fingerprint != config.PolicyFingerprint {
		return ErrCapacityJournal
	}
	_, head, err := k.ExportHistory()
	if err != nil || head != verified {
		return ErrCapacityJournal
	}
	reference, err := replayCapacityJournal(batches, config)
	if err != nil || reference.sched.Time() != at {
		return ErrCapacityJournal
	}
	_, expectedHead, err := reference.k.ExportHistory()
	if err != nil || expectedHead != verified {
		return ErrCapacityJournal
	}
	got, expected := k.Events(), reference.k.Events()
	if len(got) != len(expected) {
		return ErrCapacityJournal
	}
	for i := range got {
		original, e1 := got[i].Bytes()
		trusted, e2 := expected[i].Bytes()
		if e1 != nil || e2 != nil || !bytes.Equal(original, trusted) || got[i].Hash != expected[i].Hash || got[i].PreviousHash != expected[i].PreviousHash {
			return ErrCapacityJournal
		}
	}
	return nil
}

func replayCapacityJournal(batches []CapacityBatch, config CapacityJournalConfig) (*Capacity, error) {
	if len(batches) > MaxCapacityJournalBatches {
		return nil, ErrCapacityJournal
	}
	reference, err := NewCapacity(CapacityOptions{Yield: config.Yield, Seed: config.Seed, Enabled: config.Enabled,
		Workers: 1, Founders: capacityNormalizeFounders(config.Founders)})
	if err != nil {
		return nil, ErrCapacityJournal
	}
	var count int
	for i, b := range batches {
		if len(b.Attempts) > CapacityActorCount || count+len(b.Attempts) > MaxCapacityJournalAttempts {
			return nil, ErrCapacityJournal
		}
		count += len(b.Attempts)
		processed, stepErr := reference.Step(context.Background())
		if stepErr != nil || !processed || !sameCapacityJournalBatch(b, reference.journal[i]) {
			return nil, ErrCapacityJournal
		}
	}
	return reference, nil
}

func sameCapacityJournalBatch(got, expected CapacityBatch) bool {
	if got.Time != expected.Time || got.Version != expected.Version || got.TipID != expected.TipID || got.TipHash != expected.TipHash || len(got.Attempts) != len(expected.Attempts) {
		return false
	}
	for i, a := range got.Attempts {
		// The reference replay re-derives every attempt deterministically from
		// the pinned seed, endowments and configuration, including allocator
		// denial reasons, so the recorded label must match exactly.
		if a != expected.Attempts[i] {
			return false
		}
	}
	return true
}
