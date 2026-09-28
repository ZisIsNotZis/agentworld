package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// Pilot bundles have a distinct manifest and never migrate S6 history.
const foodFlowCheckpointFormat uint32 = 1

var ErrFoodFlowCheckpoint = errors.New("invalid or incompatible food-flow checkpoint")
var foodFlowCheckpointMagic = [4]byte{'A', 'W', 'F', 'M'}
var foodFlowStrategyMagic = [4]byte{'A', 'W', 'F', 'S'}

type foodFlowManifest struct {
	yield    int64
	seed     uint64
	steps    int
	time     sim.SimTime
	head     kernel.PortableHead
	sections [4][32]byte // journal, history, scheduler, strategy
}

func encodeFoodFlowManifest(m foodFlowManifest) []byte {
	out := append([]byte(nil), foodFlowCheckpointMagic[:]...)
	for _, v := range []uint32{foodFlowCheckpointFormat, FoodFlowFormatVersion, uint32(FoodFlowSchemaVersion), FoodFlowRuleVersion, uint32(FoodFlowProjectionVersion), strategy.FoodFlowPolicyFormatV1, FoodFlowActorCount, FoodFlowPatchCount, FoodFlowSlotsPerPatch, FoodFlowHorizonHours} {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	for _, v := range []int64{int64(FoodFlowHour), int64(foodFlowGatherDuration), int64(foodFlowMealDuration), FoodFlowInitialEnergy, FoodFlowEnergyCapacity, FoodFlowHungerCapacity, FoodFlowBagCapacity, FoodFlowNeverGatheredHour} {
		out = binary.BigEndian.AppendUint64(out, uint64(v))
	}
	out = binary.BigEndian.AppendUint64(out, uint64(m.yield))
	out = binary.BigEndian.AppendUint64(out, m.seed)
	out = binary.BigEndian.AppendUint32(out, uint32(m.steps))
	out = binary.BigEndian.AppendUint64(out, uint64(m.time))
	out = appendJournalHead(out, m.head)
	for _, hash := range m.sections {
		out = append(out, hash[:]...)
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func decodeFoodFlowManifest(data []byte) (foodFlowManifest, error) {
	var m foodFlowManifest
	if len(data) != len(encodeFoodFlowManifest(m)) {
		return m, ErrFoodFlowCheckpoint
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return m, ErrFoodFlowCheckpoint
	}
	r := journalReader{data: body}
	if !bytes.Equal(r.take(4), foodFlowCheckpointMagic[:]) {
		return m, ErrFoodFlowCheckpoint
	}
	for _, expected := range []uint32{foodFlowCheckpointFormat, FoodFlowFormatVersion, uint32(FoodFlowSchemaVersion), FoodFlowRuleVersion, uint32(FoodFlowProjectionVersion), strategy.FoodFlowPolicyFormatV1, FoodFlowActorCount, FoodFlowPatchCount, FoodFlowSlotsPerPatch, FoodFlowHorizonHours} {
		if r.u32() != expected {
			return m, ErrFoodFlowCheckpoint
		}
	}
	for _, expected := range []int64{int64(FoodFlowHour), int64(foodFlowGatherDuration), int64(foodFlowMealDuration), FoodFlowInitialEnergy, FoodFlowEnergyCapacity, FoodFlowHungerCapacity, FoodFlowBagCapacity, FoodFlowNeverGatheredHour} {
		if int64(r.u64()) != expected {
			return m, ErrFoodFlowCheckpoint
		}
	}
	m.yield, m.seed, m.steps, m.time = int64(r.u64()), r.u64(), int(r.u32()), sim.SimTime(r.u64())
	m.head = r.head()
	for i := range m.sections {
		copy(m.sections[i][:], r.take(32))
	}
	if r.bad || r.remaining() != 0 || m.yield < 0 || m.yield > FoodFlowSlotsPerPatch || m.steps < 0 || m.steps > MaxFoodFlowJournalBatches || m.time < 0 || m.time > sim.SimTime(FoodFlowHorizonHours)*sim.SimTime(FoodFlowHour) || m.head.GenesisVersion != 0 || m.head.Version != sim.WorldVersion(m.head.TipID) {
		return foodFlowManifest{}, ErrFoodFlowCheckpoint
	}
	return m, nil
}

// The strategy section owns the exact policy and actor bindings, plus only the
// in-flight runner state not reconstructible from the journal and scheduler.
func encodeFoodFlowStrategy(f *FoodFlow, journal []FoodFlowBatch, snap scheduler.Snapshot) ([]byte, error) {
	policy, err := strategy.EncodeFoodFlowPolicy(f.bound[0].Policy())
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), foodFlowStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, 1)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policy)))
	out = append(out, policy...)
	out = append(out, FoodFlowActorCount)
	for i, b := range f.bound {
		if b == nil || b.Policy() != f.bound[0].Policy() || b.Binding() != (strategy.FoodFlowBinding{Actor: sim.EntityID(i + 1), Ref: f.ref}) {
			return nil, ErrFoodFlowCheckpoint
		}
		out = binary.BigEndian.AppendUint64(out, uint64(b.Binding().Actor))
		out = binary.BigEndian.AppendUint32(out, b.Binding().Ref.Version)
	}
	for i := 1; i <= FoodFlowActorCount; i++ {
		id := sim.EntityID(i)
		kind := f.current[id]
		out = append(out, byte(kind))
		if kind == strategy.FoodFlowGather {
			c, ok := f.choices[id]
			if !ok {
				return nil, ErrFoodFlowCheckpoint
			}
			out = binary.BigEndian.AppendUint64(out, uint64(c.TargetPatch))
			out = binary.BigEndian.AppendUint64(out, uint64(c.TargetSlot))
			out = binary.BigEndian.AppendUint64(out, uint64(c.ObservedVersion))
			out = append(out, byte(c.Evaluated))
		}
	}
	if err := verifyFoodFlowRunnerState(f.current, f.choices, journal, snap, f.ref); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeFoodFlowStrategy(data []byte, f *FoodFlow, journal []FoodFlowBatch, snap scheduler.Snapshot) (map[sim.EntityID]strategy.FoodFlowAction, map[sim.EntityID]strategy.FoodFlowChoice, error) {
	r := journalReader{data: data}
	if !bytes.Equal(r.take(4), foodFlowStrategyMagic[:]) || r.u32() != 1 {
		return nil, nil, ErrFoodFlowCheckpoint
	}
	size := int(r.u16())
	if size < 1 || size > 87 || strategy.VerifyFoodFlowPolicy(r.take(size), f.bound[0].Policy()) != nil || r.u8() != FoodFlowActorCount {
		return nil, nil, ErrFoodFlowCheckpoint
	}
	for i, b := range f.bound {
		actor, version := sim.EntityID(r.u64()), r.u32()
		if b == nil || actor != sim.EntityID(i+1) || version != b.Binding().Ref.Version || b.Binding() != (strategy.FoodFlowBinding{Actor: actor, Ref: f.ref}) || b.Policy() != f.bound[0].Policy() {
			return nil, nil, ErrFoodFlowCheckpoint
		}
	}
	current := make(map[sim.EntityID]strategy.FoodFlowAction)
	choices := make(map[sim.EntityID]strategy.FoodFlowChoice)
	for i := 1; i <= FoodFlowActorCount; i++ {
		id, kind := sim.EntityID(i), strategy.FoodFlowAction(r.u8())
		if kind != 0 {
			current[id] = kind
		}
		if kind == strategy.FoodFlowGather {
			choices[id] = strategy.FoodFlowChoice{Kind: kind, TargetPatch: sim.EntityID(r.u64()), TargetSlot: sim.EntityID(r.u64()), ObservedVersion: sim.WorldVersion(r.u64()), Evaluated: int(r.u8()), Ref: f.ref}
		}
	}
	if r.bad || r.remaining() != 0 || verifyFoodFlowRunnerState(current, choices, journal, snap, f.ref) != nil {
		return nil, nil, ErrFoodFlowCheckpoint
	}
	return current, choices, nil
}

func verifyFoodFlowRunnerState(current map[sim.EntityID]strategy.FoodFlowAction, choices map[sim.EntityID]strategy.FoodFlowChoice, journal []FoodFlowBatch, snap scheduler.Snapshot, ref strategy.FoodFlowRef) error {
	open := make(map[sim.EntityID]FoodFlowActivity)
	for _, b := range journal {
		for _, a := range b.Activities {
			if a.Completed == 0 {
				open[a.Actor] = a
			} else {
				delete(open, a.Actor)
			}
		}
	}
	if len(open) != len(current) {
		return ErrFoodFlowCheckpoint
	}
	for actor, a := range open {
		if current[actor] != a.Kind {
			return ErrFoodFlowCheckpoint
		}
		c, exists := choices[actor]
		if (a.Kind == strategy.FoodFlowGather) != exists {
			return ErrFoodFlowCheckpoint
		}
		if exists {
			home, _ := FoodFlowActorPatchID(actor)
			if c.Kind != strategy.FoodFlowGather || c.Ref != ref || c.TargetPatch != home || !foodFlowSlotBelongsToPatch(c.TargetSlot, int(home-1001)) || c.RestDuration != 0 || c.Fallback != strategy.FoodFlowNoFallback || c.Evaluated < 1 || c.Evaluated > FoodFlowSlotsPerPatch {
				return ErrFoodFlowCheckpoint
			}
			found := false
			for _, b := range journal {
				if b.Time == a.Started-1 && b.Version == c.ObservedVersion {
					found = true
					break
				}
			}
			if !found {
				return ErrFoodFlowCheckpoint
			}
		}
	}
	if len(choices) > len(current) || len(snap.Fibers) != FoodFlowActorCount+1 {
		return ErrFoodFlowCheckpoint
	}
	return nil
}

// An unfinished Gather exists only at its claim boundary. Re-evaluating its
// pinned policy against the authoritative snapshot binds the strategy's full
// choice (including slot and candidate count) to the saved world, not merely
// to a plausible-looking actor and prior version.
func verifyFoodFlowOpenChoices(f *FoodFlow, snap scheduler.Snapshot) error {
	if len(f.choices) == 0 {
		return nil
	}
	if len(f.journal) == 0 || foodFlowPhase(snap.Time) != 1 || snap.Time != f.journal[len(f.journal)-1].Time {
		return ErrFoodFlowCheckpoint
	}
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	for actor, stored := range f.choices {
		if actor < 1 || actor > FoodFlowActorCount || stored.ObservedVersion != head.Version {
			return ErrFoodFlowCheckpoint
		}
		obs, err := f.observe(actor, snap.Time, view, strategy.FoodFlowNoDenial)
		if err != nil {
			return err
		}
		choice, err := f.bound[actor-1].Evaluate(obs)
		if err != nil || choice != stored {
			return ErrFoodFlowCheckpoint
		}
	}
	return nil
}

// SaveCheckpoint captures all five sections under the same coordinator lock.
// It never changes the runner or replaces an existing destination.
func (f *FoodFlow) SaveCheckpoint(path string) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrFoodFlowCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.k == nil || f.sched == nil || f.steps != len(f.journal) || f.bound[0] == nil {
		return [32]byte{}, ErrFoodFlowCheckpoint
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later. The independent constructor pins the current policy contract.
	expected, err := NewFoodFlow(FoodFlowOptions{Yield: f.yield, Seed: f.seed, Workers: 1})
	if err != nil || f.ref != expected.ref || f.bound[0].Policy() != expected.bound[0].Policy() {
		return [32]byte{}, ErrFoodFlowCheckpoint
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrFoodFlowCheckpoint
	}
	snap := f.sched.Snapshot()
	if err := verifyFoodFlowJournalScheduler(f.journal, snap, f.k.SnapshotHead()); err != nil {
		return [32]byte{}, err
	}
	if err := verifyFoodFlowCheckpointScheduler(f.k, f.journal, snap); err != nil {
		return [32]byte{}, err
	}
	journal, err := EncodeFoodFlowJournal(f.journal, f.k, head, snap.Time, f.journalConfig())
	if err != nil {
		return [32]byte{}, err
	}
	strategyBytes, err := encodeFoodFlowStrategy(f, f.journal, snap)
	if err != nil || verifyFoodFlowOpenChoices(f, snap) != nil {
		return [32]byte{}, ErrFoodFlowCheckpoint
	}
	checks, err := foodFlowCheckpointsFromHistory(f.k, f.journal, f.yield)
	if err != nil || !equalFoodFlowCheckpoints(checks, f.checkpoints) {
		return [32]byte{}, ErrFoodFlowCheckpoint
	}
	m := foodFlowManifest{yield: f.yield, seed: f.seed, steps: f.steps, time: snap.Time, head: head}
	parts := [][]byte{journal, history, portable, strategyBytes}
	for i, part := range parts {
		m.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(path, []checkpoint.Section{{Name: checkpoint.Journal, Version: 1, Data: journal}, {Name: checkpoint.KernelHistory, Version: 1, Data: history}, {Name: checkpoint.ManifestLineage, Version: 1, Data: encodeFoodFlowManifest(m)}, {Name: checkpoint.Scheduler, Version: 1, Data: portable}, {Name: checkpoint.Strategy, Version: 1, Data: strategyBytes}})
}

// RestoreFoodFlowCheckpoint rebuilds independent executable registries. A
// different worker count is allowed; every causal option and stored section is
// checked before the returned runner becomes visible.
func RestoreFoodFlowCheckpoint(path string, expected FoodFlowOptions) (*FoodFlow, [32]byte, error) {
	fail := func(err error) (*FoodFlow, [32]byte, error) {
		return nil, [32]byte{}, fmt.Errorf("%w: %v", ErrFoodFlowCheckpoint, err)
	}
	sections, digest, err := checkpoint.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	m, err := decodeFoodFlowManifest(sections[2].Data)
	if err != nil || m.yield != expected.Yield || m.seed != expected.Seed {
		return fail(ErrFoodFlowCheckpoint)
	}
	for i, part := range [][]byte{sections[0].Data, sections[1].Data, sections[3].Data, sections[4].Data} {
		if sha256.Sum256(part) != m.sections[i] {
			return fail(ErrFoodFlowCheckpoint)
		}
	}
	fresh, err := NewFoodFlow(expected)
	if err != nil {
		return fail(err)
	}
	_, genesis, err := fresh.k.ExportHistory()
	if err != nil || genesis.GenesisHash != m.head.GenesisHash || genesis.RegistryFingerprint != m.head.RegistryFingerprint {
		return fail(ErrFoodFlowCheckpoint)
	}
	k, head, err := kernel.RestoreHistory(fresh.registry, sections[1].Data)
	if err != nil || head != m.head {
		return fail(ErrFoodFlowCheckpoint)
	}
	fresh.k = k
	sched, err := scheduler.RestorePortable(k, expected.Workers, fresh.evaluate, sections[3].Data, head)
	if err != nil || sched.Time() != m.time {
		return fail(ErrFoodFlowCheckpoint)
	}
	fresh.sched = sched
	journal, err := DecodeFoodFlowJournalWithScheduler(sections[0].Data, k, head, sched, fresh.journalConfig())
	if err != nil || len(journal) != m.steps {
		return fail(ErrFoodFlowCheckpoint)
	}
	snap := sched.Snapshot()
	if err := verifyFoodFlowCheckpointScheduler(k, journal, snap); err != nil {
		return fail(err)
	}
	current, choices, err := decodeFoodFlowStrategy(sections[4].Data, fresh, journal, snap)
	if err != nil {
		return fail(err)
	}
	fresh.current, fresh.choices, fresh.journal = current, choices, journal
	if err := verifyFoodFlowOpenChoices(fresh, sched.Snapshot()); err != nil {
		return fail(err)
	}
	checks, err := foodFlowCheckpointsFromHistory(k, journal, m.yield)
	if err != nil {
		return fail(err)
	}
	fresh.current, fresh.choices, fresh.journal, fresh.checkpoints, fresh.steps = current, choices, journal, checks, m.steps
	return fresh, digest, nil
}

func equalFoodFlowCheckpoints(a, b []FoodFlowCheckpoint) bool {
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
