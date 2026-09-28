package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
)

// The journal retains rejected-only steps that have no kernel event. Its digest
// detects damage, not forgery: only the enclosing checkpoint can authenticate
// rejection reasons, which cannot be recomputed without historical observations.
const (
	SurvivalJournalFormatVersion uint32 = 1
	MaxSurvivalJournalBytes             = 16 << 20
	MaxSurvivalJournalBatches           = maxSurvivalHours
	MaxSurvivalJournalAttempts          = maxSurvivalHours * maxSurvivalActors
)

var (
	ErrSurvivalJournal   = errors.New("invalid survival attempt journal")
	survivalJournalMagic = [4]byte{'A', 'W', 'S', 'J'}
)

// SurvivalJournalConfig identifies the original causal run, not a branch's
// post-checkpoint intervention. Worker count is deliberately excluded: it does
// not affect deterministic survival outcomes.
type SurvivalJournalConfig struct {
	Seed      uint64
	Actors    int
	Hours     int
	EatWeight float64
	Policy    strategy.Ref
}

func (s *Survival) journalConfig() SurvivalJournalConfig {
	return SurvivalJournalConfig{Seed: s.seed, Actors: s.actors, Hours: s.hours, EatWeight: s.weight, Policy: s.ref}
}

// ExportJournal captures ordered attempt/outcome batches at a quiescent
// survival boundary. The kernel history is the accepted-event oracle; later
// rejected attempts have observed counts, not independently certified coverage.
func (s *Survival) ExportJournal() ([]byte, error) {
	if s == nil || s.kernel == nil || s.sched == nil {
		return nil, ErrSurvivalJournal
	}
	_, head, err := s.kernel.ExportHistory()
	if err != nil {
		return nil, err
	}
	config := s.journalConfig()
	if err = verifySurvivalJournal(s.journal, s.kernel.Events(), head, s.sched.Time(), config); err != nil {
		return nil, err
	}
	out := append([]byte(nil), survivalJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, SurvivalJournalFormatVersion)
	out = appendJournalHead(out, head)
	out = binary.BigEndian.AppendUint64(out, uint64(s.sched.Time()))
	out = binary.BigEndian.AppendUint16(out, uint16(config.Actors))
	out = binary.BigEndian.AppendUint64(out, config.Seed)
	out = binary.BigEndian.AppendUint16(out, uint16(config.Hours))
	out = binary.BigEndian.AppendUint64(out, math.Float64bits(config.EatWeight))
	out = append(out, byte(len(config.Policy.ID)))
	out = append(out, config.Policy.ID...)
	out = binary.BigEndian.AppendUint32(out, config.Policy.Version)
	out = binary.BigEndian.AppendUint16(out, uint16(len(s.journal)))
	for _, batch := range s.journal {
		out = binary.BigEndian.AppendUint64(out, uint64(batch.Time))
		out = binary.BigEndian.AppendUint64(out, uint64(batch.Version))
		out = binary.BigEndian.AppendUint64(out, uint64(batch.TipID))
		out = append(out, batch.TipHash[:]...)
		out = binary.BigEndian.AppendUint16(out, uint16(len(batch.Attempts)))
		for _, a := range batch.Attempts {
			out = binary.BigEndian.AppendUint64(out, uint64(a.Actor))
			out = append(out, byte(len(a.Key)))
			out = append(out, a.Key...)
			out = append(out, byte(a.Choice.Kind))
			out = binary.BigEndian.AppendUint64(out, uint64(a.Choice.Target))
			out = binary.BigEndian.AppendUint64(out, uint64(a.Choice.ObservedVersion))
			out = binary.BigEndian.AppendUint64(out, math.Float64bits(a.Choice.Score))
			out = append(out, byte(a.Choice.Evaluated), byte(a.Choice.Fallback), byte(a.Status), byte(a.Reason))
			out = binary.BigEndian.AppendUint64(out, uint64(a.EventID))
		}
		if len(out)+sha256.Size > MaxSurvivalJournalBytes {
			return nil, ErrSurvivalJournal
		}
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// RestoreJournal verifies the complete kernel history head, accepted links and
// scheduler time before replacing in-memory history. It does not publish files.
// Call only at a quiescent boundary with the independently verified kernel head.
func (s *Survival) RestoreJournal(data []byte, verified kernel.PortableHead) error {
	if s == nil || s.kernel == nil || s.sched == nil {
		return ErrSurvivalJournal
	}
	journal, err := DecodeSurvivalJournal(data, s.kernel, verified, s.sched.Time(), s.journalConfig())
	if err != nil {
		return err
	}
	s.journal = journal
	return nil
}

// DecodeSurvivalJournal is the portable integration API. Supply the original
// run config before applying an explicit branch intervention. The head must
// come from independently verified kernel history; no data is returned on failure.
func DecodeSurvivalJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config SurvivalJournalConfig) ([]SurvivalBatch, error) {
	if k == nil || len(data) < 4+4+sha256.Size || len(data) > MaxSurvivalJournalBytes {
		return nil, ErrSurvivalJournal
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [sha256.Size]byte(data[len(body):]) {
		return nil, ErrSurvivalJournal
	}
	r := journalReader{data: body}
	magic, format := r.take(4), r.u32()
	head := r.head()
	at, actorCount := sim.SimTime(r.u64()), int(r.u16())
	seed, hours, weightBits := r.u64(), int(r.u16()), r.u64()
	refSize := r.u8()
	if refSize == 0 || refSize > strategy.MaxRefIDBytes {
		return nil, ErrSurvivalJournal
	}
	storedRef := strategy.Ref{ID: string(r.take(int(refSize))), Version: r.u32()}
	count := r.u16()
	if r.bad || !bytes.Equal(magic, survivalJournalMagic[:]) || format != SurvivalJournalFormatVersion || head != verified || at != schedulerTime || actorCount != config.Actors || seed != config.Seed || hours != config.Hours || weightBits != math.Float64bits(config.EatWeight) || storedRef != config.Policy || count > MaxSurvivalJournalBatches {
		return nil, ErrSurvivalJournal
	}
	batches := make([]SurvivalBatch, 0, count)
	attemptCount := 0
	for range int(count) {
		b := SurvivalBatch{Time: sim.SimTime(r.u64()), Version: sim.WorldVersion(r.u64()), TipID: sim.EventID(r.u64())}
		copy(b.TipHash[:], r.take(32))
		n := int(r.u16())
		if r.bad || n == 0 || n > config.Actors || n > MaxSurvivalJournalAttempts-attemptCount {
			return nil, ErrSurvivalJournal
		}
		attemptCount += n
		// A minimum record has 8+1+1+8+8+8+4+8 bytes. Avoid allocating
		// from a corrupt count before checking the remaining input.
		if n > r.remaining()/46 {
			return nil, ErrSurvivalJournal
		}
		b.Attempts = make([]SurvivalAttempt, n)
		for i := range b.Attempts {
			a := &b.Attempts[i]
			a.Actor, a.Time, a.Ref = sim.EntityID(r.u64()), b.Time, config.Policy
			keySize := r.u8()
			if keySize == 0 || keySize > 128 {
				return nil, ErrSurvivalJournal
			}
			a.Key = string(r.take(int(keySize)))
			a.Choice = strategy.Choice{Kind: strategy.ActionKind(r.u8()), Target: sim.EntityID(r.u64()), ObservedVersion: sim.WorldVersion(r.u64()), Score: math.Float64frombits(r.u64()), Evaluated: int(r.u8()), Fallback: strategy.FallbackReason(r.u8()), Ref: config.Policy}
			a.Status, a.Reason, a.EventID = Status(r.u8()), SurvivalReason(r.u8()), sim.EventID(r.u64())
		}
		batches = append(batches, b)
	}
	if r.bad || r.remaining() != 0 {
		return nil, ErrSurvivalJournal
	}
	_, actual, err := k.ExportHistory()
	if err != nil || actual != verified || verifySurvivalJournal(batches, k.Events(), actual, schedulerTime, config) != nil {
		return nil, ErrSurvivalJournal
	}
	return batches, nil
}

func verifySurvivalJournal(batches []SurvivalBatch, events []kernel.Event, head kernel.PortableHead, schedulerTime sim.SimTime, config SurvivalJournalConfig) error {
	policy := survivalPolicy()
	actors, ref := config.Actors, config.Policy
	if actors < 1 || actors > maxSurvivalActors || config.Hours < 1 || config.Hours > maxSurvivalHours || math.IsNaN(config.EatWeight) || math.IsInf(config.EatWeight, 0) || config.EatWeight < 0 || config.EatWeight > 100 || ref != policy.Ref || schedulerTime < 0 || schedulerTime > sim.SimTime(config.Hours)*sim.SimTime(hour) || len(batches) > MaxSurvivalJournalBatches || len(events) > kernel.MaxHistoryEvents || head.TipID != sim.EventID(len(events)) || uint64(head.GenesisVersion) > ^uint64(0)-uint64(len(events)) || head.Version != head.GenesisVersion+sim.WorldVersion(len(events)) {
		return ErrSurvivalJournal
	}
	if len(batches) == 0 {
		if schedulerTime != 0 || len(events) != 0 {
			return ErrSurvivalJournal
		}
		return nil
	}
	if batches[len(batches)-1].Time != schedulerTime {
		return ErrSurvivalJournal
	}
	index, total := 0, 0
	previousVersion := head.GenesisVersion
	var previousHash [32]byte
	for i, batch := range batches {
		// NewSurvival schedules every actor for the first hour; every later
		// survival wake is scheduled at precisely the following hour. This
		// proves initial coverage and rules out omitted whole hourly batches,
		// but cannot prove the count of later rejected attempts.
		if batch.Time != sim.SimTime(i+1)*sim.SimTime(hour) || batch.Time > schedulerTime || len(batch.Attempts) == 0 || len(batch.Attempts) > actors || len(batch.Attempts) > MaxSurvivalJournalAttempts-total || (i == 0 && len(batch.Attempts) != actors) {
			return ErrSurvivalJournal
		}
		total += len(batch.Attempts)
		var previousKey string
		for j, a := range batch.Attempts {
			if i == 0 && a.Actor != sim.EntityID(j+1) {
				return ErrSurvivalJournal
			}
			c := a.Choice
			if a.Actor < 1 || a.Actor > sim.EntityID(actors) || a.Time != batch.Time || a.Ref != ref || c.Ref != ref || a.Key != survivalKey(ref, batch.Time, a.Actor) || len(a.Key) > 128 || a.Key <= previousKey || math.IsNaN(c.Score) || math.IsInf(c.Score, 0) || c.Evaluated < 0 || c.Evaluated > policy.Budget.Evaluations || c.Fallback > strategy.BudgetExhausted {
				return ErrSurvivalJournal
			}
			previousKey = a.Key
			switch c.Kind {
			case strategy.Wait:
				if c.Target != 0 || c.Fallback == strategy.NoFallback || c.Score != 0 || c.Evaluated != 0 {
					return ErrSurvivalJournal
				}
			case strategy.Rest:
				if c.Target != 0 || c.Fallback != strategy.NoFallback {
					return ErrSurvivalJournal
				}
			case strategy.Eat:
				if sim.ValidateEntityID(c.Target) != nil || c.Fallback != strategy.NoFallback {
					return ErrSurvivalJournal
				}
			default:
				return ErrSurvivalJournal
			}
			if a.Status == Rejected {
				if a.EventID != 0 || a.Reason <= SurvivalNoReason || a.Reason > SurvivalCollision ||
					(a.Reason == SurvivalStale && c.ObservedVersion == previousVersion) ||
					(a.Reason != SurvivalStale && c.ObservedVersion != previousVersion) ||
					(a.Reason == SurvivalWait && c.Kind != strategy.Wait) ||
					(a.Reason != SurvivalWait && a.Reason != SurvivalStale && c.Kind == strategy.Wait) ||
					((a.Reason == SurvivalCollision || a.Reason == SurvivalIneligible || a.Reason == SurvivalInvalidNumber) && c.Kind == strategy.Wait) ||
					((a.Reason == SurvivalInvisible || a.Reason == SurvivalInsufficientFood) && c.Kind != strategy.Eat) {
					return ErrSurvivalJournal
				}
				continue
			}
			if a.Status != Accepted || a.Reason != SurvivalNoReason || c.Kind == strategy.Wait || c.ObservedVersion != previousVersion || index == len(events) {
				return ErrSurvivalJournal
			}
			ev := events[index]
			if a.EventID != ev.ID || ev.ID != sim.EventID(index+1) || ev.Time != batch.Time || ev.Key != a.Key || ev.Cause.World || ev.Cause.Actor != a.Actor || ev.RuleVersion != 1 || ev.Kind != "component-patch" || ev.BeforeVersion != head.GenesisVersion+sim.WorldVersion(index) || ev.AfterVersion != ev.BeforeVersion+1 || ev.PreviousHash != previousHash || (c.Kind == strategy.Rest && ev.Rule != 1) || (c.Kind == strategy.Eat && ev.Rule != survivalRule) || checkSurvivalEvent(ev) != nil {
				return ErrSurvivalJournal
			}
			if c.Kind == strategy.Eat {
				visible := sim.EntityID(1001 + (int(a.Actor)-1)%4)
				other := sim.EntityID(1001 + int(a.Actor)%4)
				if (c.Target != visible && c.Target != other) || !eventUsesSurvivalTarget(ev, c.Target) {
					return ErrSurvivalJournal
				}
			}
			previousHash = ev.Hash
			index++
		}
		if batch.Version != head.GenesisVersion+sim.WorldVersion(index) || batch.TipID != sim.EventID(index) || batch.TipHash != previousHash {
			return ErrSurvivalJournal
		}
		previousVersion = batch.Version
	}
	if index != len(events) || previousVersion != head.Version || previousHash != head.TipHash || (index != 0 && (events[index-1].Time != head.TipTime || events[index-1].ID != head.TipID)) || (index == 0 && (head.TipTime != 0 || head.TipHash != [32]byte{})) {
		return ErrSurvivalJournal
	}
	return nil
}

func eventUsesSurvivalTarget(ev kernel.Event, target sim.EntityID) bool {
	for _, d := range ev.Deltas {
		if d.Component == component.CacheStockTypeID && d.Entity == target {
			return true
		}
	}
	return false
}

func appendJournalHead(out []byte, h kernel.PortableHead) []byte {
	out = append(out, h.RegistryFingerprint[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(h.GenesisVersion))
	out = append(out, h.GenesisHash[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(h.Version))
	out = binary.BigEndian.AppendUint64(out, uint64(h.TipID))
	out = binary.BigEndian.AppendUint64(out, uint64(h.TipTime))
	out = append(out, h.TipHash[:]...)
	out = append(out, h.SnapshotHash[:]...)
	return append(out, h.ProjectionHash[:]...)
}

type journalReader struct {
	data   []byte
	offset int
	bad    bool
}

func (r *journalReader) remaining() int { return len(r.data) - r.offset }
func (r *journalReader) take(n int) []byte {
	if r.bad || n < 0 || n > r.remaining() {
		r.bad = true
		return nil
	}
	part := r.data[r.offset : r.offset+n]
	r.offset += n
	return part
}
func (r *journalReader) u8() byte {
	p := r.take(1)
	if p == nil {
		return 0
	}
	return p[0]
}
func (r *journalReader) u16() uint16 {
	p := r.take(2)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint16(p)
}
func (r *journalReader) u32() uint32 {
	p := r.take(4)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint32(p)
}
func (r *journalReader) u64() uint64 {
	p := r.take(8)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint64(p)
}
func (r *journalReader) head() kernel.PortableHead {
	var h kernel.PortableHead
	copy(h.RegistryFingerprint[:], r.take(32))
	h.GenesisVersion = sim.WorldVersion(r.u64())
	copy(h.GenesisHash[:], r.take(32))
	h.Version = sim.WorldVersion(r.u64())
	h.TipID = sim.EventID(r.u64())
	h.TipTime = sim.SimTime(r.u64())
	copy(h.TipHash[:], r.take(32))
	copy(h.SnapshotHash[:], r.take(32))
	copy(h.ProjectionHash[:], r.take(32))
	return h
}
