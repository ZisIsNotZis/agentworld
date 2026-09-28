package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Format 1 is tied to the survival rule, projections, policy, and the
// source-versioned SplitMix64 stream. No migration is performed.
const survivalCheckpointFormat uint32 = 1
const survivalCompatibilitySame byte = 1
const survivalCompatibilityEatWeight byte = 2

var ErrSurvivalCheckpoint = errors.New("invalid or incompatible survival checkpoint")
var survivalManifestMagic = [4]byte{'A', 'W', 'S', 'M'}

type survivalLineage struct {
	parent        [32]byte
	parentHead    kernel.PortableHead
	compatibility byte
}

type survivalManifest struct {
	seed             uint64
	actors, hours    int
	original, active float64
	random           sim.RandomState
	head             kernel.PortableHead
	time             sim.SimTime
	initial          SurvivalMetrics
	lineage          survivalLineage
}

func encodeSurvivalManifest(m survivalManifest) []byte {
	out := append([]byte(nil), survivalManifestMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, survivalCheckpointFormat)
	out = binary.BigEndian.AppendUint64(out, m.seed)
	out = binary.BigEndian.AppendUint16(out, uint16(m.actors))
	out = binary.BigEndian.AppendUint16(out, uint16(m.hours))
	out = binary.BigEndian.AppendUint64(out, math.Float64bits(m.original))
	out = binary.BigEndian.AppendUint64(out, math.Float64bits(m.active))
	out = binary.BigEndian.AppendUint64(out, m.random.Seed)
	out = binary.BigEndian.AppendUint64(out, uint64(m.random.Stream))
	out = binary.BigEndian.AppendUint64(out, m.random.Position)
	out = appendJournalHead(out, m.head)
	out = binary.BigEndian.AppendUint64(out, uint64(m.time))
	out = binary.BigEndian.AppendUint32(out, uint32(m.initial.Alive))
	out = binary.BigEndian.AppendUint32(out, uint32(m.initial.Energy))
	for _, count := range m.initial.Hunger {
		out = binary.BigEndian.AppendUint32(out, uint32(count))
	}
	out = binary.BigEndian.AppendUint32(out, uint32(m.initial.Food))
	out = append(out, m.lineage.parent[:]...)
	out = appendJournalHead(out, m.lineage.parentHead)
	out = append(out, m.lineage.compatibility)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func decodeSurvivalManifest(data []byte) (survivalManifest, error) {
	var m survivalManifest
	// A fixed-width encoding avoids ambiguous defaults and trailing fields.
	if len(data) != len(encodeSurvivalManifest(m)) {
		return m, ErrSurvivalCheckpoint
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return m, ErrSurvivalCheckpoint
	}
	r := journalReader{data: body}
	magic, format := r.take(4), r.u32()
	m.seed, m.actors, m.hours = r.u64(), int(r.u16()), int(r.u16())
	m.original, m.active = math.Float64frombits(r.u64()), math.Float64frombits(r.u64())
	m.random = sim.RandomState{Seed: r.u64(), Stream: sim.StreamID(r.u64()), Position: r.u64()}
	m.head, m.time = r.head(), sim.SimTime(r.u64())
	m.initial.Alive, m.initial.Energy = int(r.u32()), int(r.u32())
	for i := range m.initial.Hunger {
		m.initial.Hunger[i] = int(r.u32())
	}
	m.initial.Food = int(r.u32())
	copy(m.lineage.parent[:], r.take(32))
	m.lineage.parentHead, m.lineage.compatibility = r.head(), r.u8()
	if r.bad || r.remaining() != 0 || string(magic) != string(survivalManifestMagic[:]) || format != survivalCheckpointFormat {
		return survivalManifest{}, ErrSurvivalCheckpoint
	}
	return m, nil
}

// SaveCheckpoint publishes an immutable five-section bundle only at a completed
// coordinator boundary. Neither a failed export nor a failed write alters s.
// Callers must not mutate the underlying scheduler directly during this call.
func (s *Survival) SaveCheckpoint(path string) ([32]byte, error) {
	if s == nil {
		return [32]byte{}, ErrSurvivalCheckpoint
	}
	s.boundary.Lock()
	defer s.boundary.Unlock()
	if s.pending != nil {
		return [32]byte{}, ErrSurvivalCheckpoint
	}
	history, head, err := s.kernel.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := s.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrSurvivalCheckpoint
	}
	policy, err := s.policies.ExportPortable(s.bound)
	if err != nil {
		return [32]byte{}, err
	}
	journal, err := s.ExportJournal()
	if err != nil {
		return [32]byte{}, err
	}
	m := survivalManifest{seed: s.seed, actors: s.actors, hours: s.hours, original: s.weight, active: s.activeWeight,
		random: sim.RandomState{Seed: s.seed, Stream: 1, Position: s.randomPosition}, head: head, time: s.sched.Time(), initial: s.initial, lineage: s.lineage}
	if err := validateSurvivalManifest(m); err != nil {
		return [32]byte{}, err
	}
	digest, err := checkpoint.WriteFile(path, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journal},
		{Name: checkpoint.KernelHistory, Version: 1, Data: history},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeSurvivalManifest(m)},
		{Name: checkpoint.Scheduler, Version: 1, Data: portable},
		{Name: checkpoint.Strategy, Version: 1, Data: policy},
	})
	if err != nil {
		return [32]byte{}, err
	}
	// The next publication descends from this verified boundary, not its
	// grandparent. A failed publication leaves the cursor untouched.
	s.lineage = survivalLineage{parent: digest, parentHead: head, compatibility: m.lineage.compatibility}
	return digest, nil
}

func validSurvivalWeight(w float64) bool {
	return !math.IsNaN(w) && !math.IsInf(w, 0) && w >= 0 && w <= 100
}

func validateSurvivalManifest(m survivalManifest) error {
	if m.actors < 1 || m.actors > maxSurvivalActors || m.hours < 1 || m.hours > maxSurvivalHours ||
		!validSurvivalWeight(m.original) || !validSurvivalWeight(m.active) ||
		m.random != (sim.RandomState{Seed: m.seed, Stream: 1, Position: uint64(4 + m.actors)}) ||
		m.time < 0 || m.time > sim.SimTime(m.hours)*sim.SimTime(hour) ||
		m.head.GenesisVersion != 0 || m.head.Version != sim.WorldVersion(m.head.TipID) ||
		m.initial.Alive != m.actors || m.initial.Energy < 0 || m.initial.Food < 0 {
		return ErrSurvivalCheckpoint
	}
	if m.lineage.parent == [32]byte{} {
		if m.lineage.parentHead != (kernel.PortableHead{}) || m.lineage.compatibility != survivalCompatibilitySame || math.Float64bits(m.active) != math.Float64bits(m.original) {
			return ErrSurvivalCheckpoint
		}
	} else {
		p := m.lineage.parentHead
		if p.RegistryFingerprint != m.head.RegistryFingerprint || p.GenesisHash != m.head.GenesisHash || p.GenesisVersion != m.head.GenesisVersion ||
			p.Version != sim.WorldVersion(p.TipID) || p.Version > m.head.Version || p.TipID > m.head.TipID || p.TipTime > m.time ||
			(m.lineage.compatibility != survivalCompatibilitySame && m.lineage.compatibility != survivalCompatibilityEatWeight) ||
			(m.lineage.compatibility == survivalCompatibilitySame && math.Float64bits(m.active) != math.Float64bits(m.original)) {
			return ErrSurvivalCheckpoint
		}
	}
	return nil
}

// RestoreSurvivalCheckpoint validates every section against independently
// constructed executable registries and the original causal configuration.
// Workers may differ; EatWeight must identify the original pre-fork policy.
// The returned digest identifies this bundle for later lineage references.
func RestoreSurvivalCheckpoint(path string, expected SurvivalOptions) (*Survival, [32]byte, error) {
	fail := func(err error) (*Survival, [32]byte, error) {
		return nil, [32]byte{}, fmt.Errorf("%w: %v", ErrSurvivalCheckpoint, err)
	}
	sections, digest, err := checkpoint.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	m, err := decodeSurvivalManifest(sections[2].Data)
	if err != nil || validateSurvivalManifest(m) != nil {
		return fail(ErrSurvivalCheckpoint)
	}
	if m.seed != expected.Seed || m.actors != expected.Actors || m.hours != expected.Hours || math.Float64bits(m.original) != math.Float64bits(expected.EatWeight) {
		return fail(ErrSurvivalCheckpoint)
	}
	fresh, err := NewSurvival(expected)
	if err != nil {
		return fail(err)
	}
	_, genesis, err := fresh.kernel.ExportHistory()
	if err != nil || genesis.RegistryFingerprint != m.head.RegistryFingerprint || genesis.GenesisHash != m.head.GenesisHash || fresh.randomPosition != m.random.Position || fresh.initial != m.initial {
		return fail(ErrSurvivalCheckpoint)
	}
	k, head, err := kernel.RestoreHistory(fresh.registry, sections[1].Data)
	if err != nil || head != m.head {
		return fail(ErrSurvivalCheckpoint)
	}
	if m.lineage.parent != [32]byte{} {
		p := m.lineage.parentHead
		switch {
		case p.TipID == head.TipID:
			if p != head {
				return fail(ErrSurvivalCheckpoint)
			}
		case p.TipID == 0:
			if p != genesis {
				return fail(ErrSurvivalCheckpoint)
			}
		default:
			events := k.Events()
			// An ancestor is a complete coordinator boundary, never a prefix
			// ending partway through one committed timestamp.
			if events[p.TipID].Time <= p.TipTime {
				return fail(ErrSurvivalCheckpoint)
			}
			prefix, err := kernel.Replay(fresh.registry, 0, fresh.seeds, events[:p.TipID])
			if err != nil {
				return fail(err)
			}
			_, prefixHead, err := prefix.ExportHistory()
			if err != nil || prefixHead != p {
				return fail(ErrSurvivalCheckpoint)
			}
		}
	}
	policies, bound, err := strategy.RestorePortable(sections[4].Data, fresh.policies)
	if err != nil || len(bound) != m.actors {
		return fail(ErrSurvivalCheckpoint)
	}
	for a := 1; a <= m.actors; a++ {
		b := bound[sim.EntityID(a)]
		if b == nil || b.Ref() != fresh.ref {
			return fail(ErrSurvivalCheckpoint)
		}
		overrides := b.Binding().Overrides
		weight, present := overrides[hungerWeight]
		if len(overrides) != 1 || !present || math.Float64bits(weight) != math.Float64bits(m.active) {
			return fail(ErrSurvivalCheckpoint)
		}
	}
	fresh.kernel, fresh.policies, fresh.bound = k, policies, bound
	sched, err := scheduler.RestorePortable(k, expected.Workers, fresh.evaluate, sections[3].Data, head)
	if err != nil || sched.Time() != m.time {
		return fail(ErrSurvivalCheckpoint)
	}
	snap := sched.Snapshot()
	if len(snap.Fibers) != m.actors {
		return fail(ErrSurvivalCheckpoint)
	}
	horizon := sim.SimTime(m.hours) * sim.SimTime(hour)
	for _, wake := range snap.Wakes {
		if wake.At > horizon {
			return fail(ErrSurvivalCheckpoint)
		}
	}
	for i, f := range snap.Fibers {
		if f.Actor != sim.EntityID(i+1) {
			return fail(ErrSurvivalCheckpoint)
		}
		if f.Activity != nil && (f.Activity.Deadline > horizon || (f.Activity.InterruptAt != nil && *f.Activity.InterruptAt > horizon)) {
			return fail(ErrSurvivalCheckpoint)
		}
	}
	fresh.sched = sched
	journal, err := DecodeSurvivalJournal(sections[0].Data, k, head, m.time, fresh.journalConfig())
	if err != nil {
		return fail(err)
	}
	fresh.journal, fresh.activeWeight, fresh.lineage = journal, m.active, survivalLineage{parent: digest, parentHead: head, compatibility: m.lineage.compatibility}
	if _, err := fresh.metrics(k); err != nil {
		return fail(err)
	}
	return fresh, digest, nil
}

// BranchSurvivalCheckpoint verifies the source's entire common prefix before
// rebinding only the future Eat utility. Historical journal identity is kept.
func BranchSurvivalCheckpoint(source, destination string, expected SurvivalOptions, eatWeight float64) (*Survival, [32]byte, error) {
	if !validSurvivalWeight(eatWeight) {
		return nil, [32]byte{}, ErrSurvivalCheckpoint
	}
	s, _, err := RestoreSurvivalCheckpoint(source, expected)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if math.Float64bits(eatWeight) == math.Float64bits(s.activeWeight) {
		return nil, [32]byte{}, ErrSurvivalCheckpoint
	}
	for a := 1; a <= s.actors; a++ {
		b, err := s.policies.Bind(strategy.Binding{Ref: s.ref, Overrides: map[strategy.ParamID]float64{hungerWeight: eatWeight}})
		if err != nil {
			return nil, [32]byte{}, err
		}
		s.bound[sim.EntityID(a)] = b
	}
	s.activeWeight = eatWeight
	s.lineage.compatibility = survivalCompatibilityEatWeight
	digest, err := s.SaveCheckpoint(destination)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return s, digest, nil
}
