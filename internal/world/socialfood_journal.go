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
)

// A journal digest detects damage, not forgery. A checkpoint must authenticate
// the whole bundle and independently retain its verified kernel head.
const (
	SocialFoodJournalFormatVersion uint32 = 1
	MaxSocialFoodJournalBytes             = 8 << 20
	MaxSocialFoodJournalBatches           = socialFoodStepLimit
	MaxSocialFoodJournalAttempts          = SocialFoodHorizonHours * SocialFoodActorCount * 7
)

var ErrSocialFoodJournal = errors.New("invalid social-food evidence journal")
var socialFoodJournalMagic = [4]byte{'A', 'W', 'S', 'J'}

type SocialFoodJournalConfig struct {
	Yield              int64
	Seed               uint64
	Enabled            bool
	Policy             strategy.SocialFoodRef
	TestClaimOverrides map[sim.EntityID]int64 // explicit privileged experiment injection, not a donor observation
}

type SocialFoodJournalCounts struct {
	GatherAdmitted, GatherDenied, Requests, Gifts, Refusals, Expiries, Eats uint32
}

type socialFoodJournalWire struct {
	Format        uint32
	Head          kernel.PortableHead
	SchedulerTime sim.SimTime
	Config        SocialFoodJournalConfig
	Counts        SocialFoodJournalCounts
	Batches       []SocialFoodBatch
}

func (f *SocialFood) journalConfig() SocialFoodJournalConfig {
	return SocialFoodJournalConfig{Yield: f.yield, Seed: f.seed, Enabled: f.enabled, Policy: f.ref, TestClaimOverrides: f.claimOverrides}
}

// ExportJournal publishes committed evidence only, not a disk bundle. The
// coordinator is locked across the history and scheduler consistency checks.
func (f *SocialFood) ExportJournal() ([]byte, error) {
	if f == nil {
		return nil, ErrSocialFoodJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return nil, ErrSocialFoodJournal
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		return nil, err
	}
	before, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || schedulerHead != head {
		return nil, ErrSocialFoodJournal
	}
	data, err := EncodeSocialFoodJournal(f.journal, f.k, head, f.sched.Time(), f.journalConfig())
	if err != nil {
		return nil, err
	}
	reference, err := replaySocialFoodJournal(f.journal, f.journalConfig())
	if err != nil {
		return nil, err
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != head || !bytes.Equal(before, want) {
		return nil, ErrSocialFoodJournal
	}
	after, afterHead, err := f.sched.ExportPortable()
	if err != nil || afterHead != head || !bytes.Equal(before, after) {
		return nil, ErrSocialFoodJournal
	}
	return data, nil
}

// RestoreJournal replaces only the journal and only after full validation.
// Kernel, scheduler and other runner state are checkpoint responsibilities.
func (f *SocialFood) RestoreJournal(data []byte, verified kernel.PortableHead) error {
	if f == nil {
		return ErrSocialFoodJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return ErrSocialFoodJournal
	}
	batches, err := DecodeSocialFoodJournalWithScheduler(data, f.k, verified, f.sched, f.journalConfig())
	if err != nil {
		return err
	}
	f.journal = batches
	return nil
}

// EncodeSocialFoodJournal validates accepted history and typed evidence at a
// closed timestamp. It does not certify open activities or scheduler wakes;
// ExportJournal and the scheduler-aware decoder do that at publication/restore.
func EncodeSocialFoodJournal(batches []SocialFoodBatch, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config SocialFoodJournalConfig) ([]byte, error) {
	if err := verifySocialFoodJournal(batches, k, verified, schedulerTime, config); err != nil {
		return nil, err
	}
	wire := socialFoodJournalWire{Format: SocialFoodJournalFormatVersion, Head: verified, SchedulerTime: schedulerTime, Config: config, Counts: socialFoodJournalCounts(batches), Batches: batches}
	body, err := json.Marshal(wire)
	if err != nil || len(body)+12+sha256.Size > MaxSocialFoodJournalBytes {
		return nil, ErrSocialFoodJournal
	}
	out := append([]byte(nil), socialFoodJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, SocialFoodJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// DecodeSocialFoodJournal validates canonical wire, configuration, typed
// request/consent/transfer links and the entire accepted event partition. This
// is history-only partial validation: a schedulerTime cannot prove open fibers,
// activity tokens, pending wakes, or the runner's outstanding activity maps.
// Gather denial labels and their counts are historical evidence, not a claim
// that rejected allocation reasons can be reconstructed from accepted events.
func DecodeSocialFoodJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config SocialFoodJournalConfig) ([]SocialFoodBatch, error) {
	if len(data) < 12+sha256.Size || len(data) > MaxSocialFoodJournalBytes || !bytes.Equal(data[:4], socialFoodJournalMagic[:]) || binary.BigEndian.Uint32(data[4:8]) != SocialFoodJournalFormatVersion || int(binary.BigEndian.Uint32(data[8:12])) != len(data)-12-sha256.Size {
		return nil, ErrSocialFoodJournal
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return nil, ErrSocialFoodJournal
	}
	var wire socialFoodJournalWire
	decoder := json.NewDecoder(bytes.NewReader(data[12:len(body)]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || wire.Format != SocialFoodJournalFormatVersion || wire.Head != verified || wire.SchedulerTime != schedulerTime || !sameSocialFoodJournalConfig(wire.Config, config) || wire.Counts != socialFoodJournalCounts(wire.Batches) {
		return nil, ErrSocialFoodJournal
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, data[12:len(body)]) || verifySocialFoodJournal(wire.Batches, k, verified, schedulerTime, config) != nil {
		return nil, ErrSocialFoodJournal
	}
	return wire.Batches, nil
}

// DecodeSocialFoodJournalWithScheduler is the complete checkpoint seam. The
// scheduler must first be restored independently against the verified head.
// Its canonical portable state must match the trusted replay of these batches,
// including open activities, tokens, wakes, request statuses and stopped fibers.
func DecodeSocialFoodJournalWithScheduler(data []byte, k *kernel.Kernel, verified kernel.PortableHead, sched *scheduler.Scheduler, config SocialFoodJournalConfig) ([]SocialFoodBatch, error) {
	if sched == nil {
		return nil, ErrSocialFoodJournal
	}
	before, head, err := sched.ExportPortable()
	if err != nil || head != verified {
		return nil, ErrSocialFoodJournal
	}
	batches, err := DecodeSocialFoodJournal(data, k, verified, sched.Time(), config)
	if err != nil {
		return nil, ErrSocialFoodJournal
	}
	reference, err := replaySocialFoodJournal(batches, config)
	if err != nil {
		return nil, err
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != verified || !bytes.Equal(before, want) {
		return nil, ErrSocialFoodJournal
	}
	after, afterHead, err := sched.ExportPortable()
	if err != nil || afterHead != verified || !bytes.Equal(before, after) {
		return nil, ErrSocialFoodJournal
	}
	return batches, nil
}

func sameSocialFoodJournalConfig(a, b SocialFoodJournalConfig) bool {
	return a.Yield == b.Yield && a.Seed == b.Seed && a.Enabled == b.Enabled && a.Policy == b.Policy && reflect.DeepEqual(normalizeSocialFoodOverrides(a.TestClaimOverrides), normalizeSocialFoodOverrides(b.TestClaimOverrides))
}
func normalizeSocialFoodOverrides(overrides map[sim.EntityID]int64) map[sim.EntityID]int64 {
	out := make(map[sim.EntityID]int64, len(overrides))
	for id, value := range overrides {
		out[id] = value
	}
	return out
}
func socialFoodJournalCounts(batches []SocialFoodBatch) SocialFoodJournalCounts {
	var c SocialFoodJournalCounts
	for _, b := range batches {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "gather":
				if a.Accepted {
					c.GatherAdmitted++
				} else {
					c.GatherDenied++
				}
			case "request":
				c.Requests++
			case "gift":
				c.Gifts++
			case "refuse":
				c.Refusals++
			case "expire":
				c.Expiries++
			case "eat":
				c.Eats++
			}
		}
	}
	return c
}

// Replay is deliberately the trusted production runner, not raw gift-rule
// admission: the kernel alone accepts schema-valid status-only counterfeit
// transfers. Every event body is compared, so all required entity/component/
// field deltas, before/after values, and bag source/gatherer/last-donor values
// must agree with fresh checked proposals and an independent donor evaluation.
// Historical Gather rejection labels are the exception: they are checked for
// type and admitted-vs-denied, but not represented as recomputable outcomes.
func verifySocialFoodJournal(batches []SocialFoodBatch, k *kernel.Kernel, verified kernel.PortableHead, at sim.SimTime, config SocialFoodJournalConfig) error {
	if k == nil || config.Yield < 0 || config.Yield > 8 || config.Policy != (strategy.SocialFoodRef{ID: "social-food", Version: 2}) || at < 0 || at > sim.SimTime(SocialFoodHorizonHours)*sim.SimTime(SocialFoodHour) || len(batches) > MaxSocialFoodJournalBatches {
		return ErrSocialFoodJournal
	}
	for id, claim := range config.TestClaimOverrides {
		if _, err := SocialFoodActorPatchID(id); err != nil || claim < 0 || claim > 2 {
			return ErrSocialFoodJournal
		}
	}
	_, head, err := k.ExportHistory()
	if err != nil || head != verified {
		return ErrSocialFoodJournal
	}
	reference, err := replaySocialFoodJournal(batches, config)
	if err != nil || reference.sched.Time() != at {
		return ErrSocialFoodJournal
	}
	_, expectedHead, err := reference.k.ExportHistory()
	if err != nil || expectedHead != verified {
		return ErrSocialFoodJournal
	}
	got, expected := k.Events(), reference.k.Events()
	if len(got) != len(expected) {
		return ErrSocialFoodJournal
	}
	for i := range got {
		original, e1 := got[i].Bytes()
		trusted, e2 := expected[i].Bytes()
		if e1 != nil || e2 != nil || !bytes.Equal(original, trusted) || got[i].Hash != expected[i].Hash || got[i].PreviousHash != expected[i].PreviousHash {
			return ErrSocialFoodJournal
		}
	}
	return nil
}

func replaySocialFoodJournal(batches []SocialFoodBatch, config SocialFoodJournalConfig) (*SocialFood, error) {
	if len(batches) > MaxSocialFoodJournalBatches {
		return nil, ErrSocialFoodJournal
	}
	reference, err := NewSocialFood(SocialFoodOptions{Yield: config.Yield, Seed: config.Seed, Enabled: config.Enabled, Workers: 1, TestClaimOverrides: normalizeSocialFoodOverrides(config.TestClaimOverrides)})
	if err != nil {
		return nil, ErrSocialFoodJournal
	}
	var count int
	for i, b := range batches {
		if len(b.Attempts) > SocialFoodActorCount || count+len(b.Attempts) > MaxSocialFoodJournalAttempts {
			return nil, ErrSocialFoodJournal
		}
		count += len(b.Attempts)
		processed, stepErr := reference.Step(context.Background())
		if stepErr != nil || !processed || !sameSocialFoodJournalBatch(b, reference.journal[i]) {
			return nil, ErrSocialFoodJournal
		}
	}
	return reference, nil
}
func sameSocialFoodJournalBatch(got, expected SocialFoodBatch) bool {
	if got.Time != expected.Time || got.Version != expected.Version || got.TipID != expected.TipID || got.TipHash != expected.TipHash || len(got.Attempts) != len(expected.Attempts) {
		return false
	}
	for i, a := range got.Attempts {
		// The reference replay re-derives every attempt deterministically from the
		// pinned seed and configuration, including allocator denial reasons, so
		// the recorded label must match exactly.
		if a != expected.Attempts[i] {
			return false
		}
	}
	return true
}
