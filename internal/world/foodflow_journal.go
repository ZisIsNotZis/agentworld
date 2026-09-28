package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
)

// A journal digest detects damage, not forgery. The enclosing checkpoint must
// authenticate these bytes and the independently verified accepted history.
// Seed is recorded run identity: zero-flow history has no seed-dependent
// accepted action, so kernel history alone cannot authenticate that value.
const (
	FoodFlowJournalFormatVersion uint32 = 1
	MaxFoodFlowJournalBytes             = 8 << 20
	MaxFoodFlowJournalBatches           = 700
	MaxFoodFlowJournalAttempts          = 168 * FoodFlowActorCount * 3
	MaxFoodFlowJournalActivities        = 168 * FoodFlowActorCount * 3
	foodFlowNoRank                      = 255
)

var ErrFoodFlowJournal = errors.New("invalid food-flow evidence journal")
var foodFlowJournalMagic = [4]byte{'A', 'W', 'F', 'J'}

type FoodFlowJournalConfig struct {
	Yield  int64
	Seed   uint64
	Policy strategy.FoodFlowRef
}

func (f *FoodFlow) journalConfig() FoodFlowJournalConfig {
	return FoodFlowJournalConfig{Yield: f.yield, Seed: f.seed, Policy: f.ref}
}

// ExportJournal captures only committed coordinator evidence. Workers are not
// causal and are not part of the run identity. No file is published here.
func (f *FoodFlow) ExportJournal() ([]byte, error) {
	if f == nil {
		return nil, ErrFoodFlowJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return nil, ErrFoodFlowJournal
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		return nil, err
	}
	_, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return nil, ErrFoodFlowJournal
	}
	if err := verifyFoodFlowJournalScheduler(f.journal, f.sched.Snapshot(), f.k.SnapshotHead()); err != nil {
		return nil, err
	}
	return EncodeFoodFlowJournal(f.journal, f.k, head, f.sched.Time(), f.journalConfig())
}

// RestoreJournal replaces only the journal, after all checks pass. Kernel and
// scheduler restore are separate checkpoint responsibilities. It will not
// silently import a journal from another seed, yield or policy version.
func (f *FoodFlow) RestoreJournal(data []byte, verified kernel.PortableHead) error {
	if f == nil {
		return ErrFoodFlowJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return ErrFoodFlowJournal
	}
	journal, err := DecodeFoodFlowJournalWithScheduler(data, f.k, verified, f.sched, f.journalConfig())
	if err != nil {
		return err
	}
	f.journal = journal
	return nil
}

type foodFlowJournalAttempt struct {
	FoodFlowAttempt
	Phase       byte
	Rank        byte         // 255 when the claim was not allocator-eligible, or not a Gather
	Slot        sim.EntityID // actual allocated slot, not the strategy's target affordance
	SourcePatch sim.EntityID
}

// EncodeFoodFlowJournal validates the complete accepted stream (including
// world pulses), then encodes typed attempt and timed activity evidence. The
// allocation rank is the seed/hour rotation among recorded eligible claims;
// rejection reasons are historical observations, never recomputed outcomes.
// This history-only API cannot certify open fibers; runner publication must
// use ExportJournal, which reconciles them with the scheduler snapshot.
func EncodeFoodFlowJournal(batches []FoodFlowBatch, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config FoodFlowJournalConfig) ([]byte, error) {
	if k == nil {
		return nil, ErrFoodFlowJournal
	}
	_, head, err := k.ExportHistory()
	if err != nil || head != verified || !foodFlowJournalYieldMatches(k, config.Yield) {
		return nil, ErrFoodFlowJournal
	}
	records, err := verifyFoodFlowJournal(batches, k.Events(), head, schedulerTime, config)
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), foodFlowJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, FoodFlowJournalFormatVersion)
	out = appendJournalHead(out, head)
	out = binary.BigEndian.AppendUint64(out, uint64(schedulerTime))
	out = binary.BigEndian.AppendUint64(out, uint64(config.Yield))
	out = binary.BigEndian.AppendUint64(out, config.Seed)
	out = append(out, byte(len(config.Policy.ID)))
	out = append(out, config.Policy.ID...)
	out = binary.BigEndian.AppendUint32(out, config.Policy.Version)
	out = binary.BigEndian.AppendUint16(out, uint16(len(batches)))
	for i, b := range batches {
		out = binary.BigEndian.AppendUint64(out, uint64(b.Time))
		out = append(out, foodFlowPhase(b.Time))
		out = binary.BigEndian.AppendUint64(out, uint64(b.Version))
		out = binary.BigEndian.AppendUint64(out, uint64(b.TipID))
		out = append(out, b.TipHash[:]...)
		out = append(out, byte(len(b.Attempts)), byte(len(b.Activities)))
		for _, r := range records[i] {
			a, c := r.FoodFlowAttempt, r.Choice
			out = binary.BigEndian.AppendUint64(out, uint64(a.Actor))
			out = binary.BigEndian.AppendUint64(out, uint64(a.Time))
			out = append(out, r.Phase, byte(len(a.Key)))
			out = append(out, a.Key...)
			out = append(out, byte(c.Kind))
			out = binary.BigEndian.AppendUint64(out, uint64(c.TargetPatch))
			out = binary.BigEndian.AppendUint64(out, uint64(c.TargetSlot))
			out = binary.BigEndian.AppendUint64(out, uint64(c.RestDuration))
			out = binary.BigEndian.AppendUint64(out, uint64(c.ObservedVersion))
			out = append(out, byte(c.Evaluated), byte(c.Fallback), byte(a.Rejection))
			if a.Accepted {
				out = append(out, 1)
			} else {
				out = append(out, 0)
			}
			out = binary.BigEndian.AppendUint64(out, uint64(a.EventID))
			out = append(out, r.Rank)
			out = binary.BigEndian.AppendUint64(out, uint64(r.Slot))
			out = binary.BigEndian.AppendUint64(out, uint64(r.SourcePatch))
		}
		for _, a := range b.Activities {
			out = binary.BigEndian.AppendUint64(out, uint64(a.Actor))
			out = append(out, byte(a.Kind))
			out = binary.BigEndian.AppendUint64(out, uint64(a.Started))
			out = binary.BigEndian.AppendUint64(out, uint64(a.Completed))
			out = binary.BigEndian.AppendUint64(out, a.Token)
			out = binary.BigEndian.AppendUint64(out, uint64(a.EventID))
		}
		if len(out)+sha256.Size > MaxFoodFlowJournalBytes {
			return nil, ErrFoodFlowJournal
		}
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// DecodeFoodFlowJournal validates history/head, scenario identity, recorded
// scheduler time, canonical wire and event links. It is partial validation:
// schedulerTime alone cannot prove that open journal activities match fibers,
// completion wakes or tokens. For complete checkpoint validation, use
// DecodeFoodFlowJournalWithScheduler with a verified/restored scheduler.
func DecodeFoodFlowJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config FoodFlowJournalConfig) ([]FoodFlowBatch, error) {
	if k == nil || len(data) < 4+4+sha256.Size || len(data) > MaxFoodFlowJournalBytes {
		return nil, ErrFoodFlowJournal
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return nil, ErrFoodFlowJournal
	}
	r := journalReader{data: body}
	magic, format := r.take(4), r.u32()
	head := r.head()
	at, yield, seed := sim.SimTime(r.u64()), int64(r.u64()), r.u64()
	size := r.u8()
	if size == 0 || size > strategy.MaxRefIDBytes {
		return nil, ErrFoodFlowJournal
	}
	ref := strategy.FoodFlowRef{ID: string(r.take(int(size))), Version: r.u32()}
	count := int(r.u16())
	if r.bad || !bytes.Equal(magic, foodFlowJournalMagic[:]) || format != FoodFlowJournalFormatVersion || head != verified || at != schedulerTime || yield != config.Yield || seed != config.Seed || ref != config.Policy || count > MaxFoodFlowJournalBatches {
		return nil, ErrFoodFlowJournal
	}
	batches := make([]FoodFlowBatch, 0, count)
	recorded := make([][]foodFlowJournalAttempt, 0, count)
	attempts, activities := 0, 0
	for range count {
		b := FoodFlowBatch{Time: sim.SimTime(r.u64())}
		phase := r.u8()
		b.Version, b.TipID = sim.WorldVersion(r.u64()), sim.EventID(r.u64())
		copy(b.TipHash[:], r.take(32))
		na, nc := int(r.u8()), int(r.u8())
		if r.bad || phase != foodFlowPhase(b.Time) || na > FoodFlowActorCount || nc > FoodFlowActorCount || na > MaxFoodFlowJournalAttempts-attempts || nc > MaxFoodFlowJournalActivities-activities || na*88+nc*41 > r.remaining() {
			return nil, ErrFoodFlowJournal
		}
		attempts += na
		activities += nc
		if na != 0 {
			b.Attempts = make([]FoodFlowAttempt, na)
		}
		recordsAt := make([]foodFlowJournalAttempt, na)
		for j := range b.Attempts {
			a := &b.Attempts[j]
			a.Actor, a.Time = sim.EntityID(r.u64()), sim.SimTime(r.u64())
			aphase, keySize := r.u8(), r.u8()
			if keySize == 0 || keySize > 128 {
				return nil, ErrFoodFlowJournal
			}
			a.Key = string(r.take(int(keySize)))
			a.Choice = strategy.FoodFlowChoice{Kind: strategy.FoodFlowAction(r.u8()), TargetPatch: sim.EntityID(r.u64()), TargetSlot: sim.EntityID(r.u64()), RestDuration: sim.Duration(r.u64()), ObservedVersion: sim.WorldVersion(r.u64()), Ref: config.Policy, Evaluated: int(r.u8()), Fallback: strategy.FoodFlowFallback(r.u8())}
			a.Rejection = FoodFlowGatherRejection(r.u8())
			accepted := r.u8()
			a.Accepted = accepted == 1
			a.EventID = sim.EventID(r.u64())
			rank := r.u8()
			slot, source := sim.EntityID(r.u64()), sim.EntityID(r.u64())
			if r.bad || accepted > 1 || aphase != phase {
				return nil, ErrFoodFlowJournal
			}
			recordsAt[j] = foodFlowJournalAttempt{FoodFlowAttempt: *a, Phase: aphase, Rank: rank, Slot: slot, SourcePatch: source}
		}
		if nc != 0 {
			b.Activities = make([]FoodFlowActivity, nc)
		}
		for j := range b.Activities {
			a := &b.Activities[j]
			a.Actor, a.Kind = sim.EntityID(r.u64()), strategy.FoodFlowAction(r.u8())
			a.Started, a.Completed = sim.SimTime(r.u64()), sim.SimTime(r.u64())
			a.Token, a.EventID = r.u64(), sim.EventID(r.u64())
		}
		batches = append(batches, b)
		recorded = append(recorded, recordsAt)
	}
	if r.bad || r.remaining() != 0 {
		return nil, ErrFoodFlowJournal
	}
	_, actual, err := k.ExportHistory()
	if err != nil || actual != verified || !foodFlowJournalYieldMatches(k, config.Yield) {
		return nil, ErrFoodFlowJournal
	}
	canonical, err := verifyFoodFlowJournal(batches, k.Events(), actual, schedulerTime, config)
	if err != nil || len(canonical) != len(recorded) {
		return nil, ErrFoodFlowJournal
	}
	for i := range canonical {
		for j := range canonical[i] {
			if canonical[i][j] != recorded[i][j] {
				return nil, ErrFoodFlowJournal
			}
		}
	}
	return batches, nil
}

// DecodeFoodFlowJournalWithScheduler is the complete checkpoint integration
// API. The scheduler must have been independently restored with RestorePortable
// against verified; both exports bracket the snapshot to reject a changed
// coordinator boundary. No runner state is changed on failure.
func DecodeFoodFlowJournalWithScheduler(data []byte, k *kernel.Kernel, verified kernel.PortableHead, sched *scheduler.Scheduler, config FoodFlowJournalConfig) ([]FoodFlowBatch, error) {
	if sched == nil {
		return nil, ErrFoodFlowJournal
	}
	before, schedulerHead, err := sched.ExportPortable()
	if err != nil || schedulerHead != verified {
		return nil, ErrFoodFlowJournal
	}
	snap := sched.Snapshot()
	batches, err := DecodeFoodFlowJournal(data, k, verified, snap.Time, config)
	if err != nil || verifyFoodFlowJournalScheduler(batches, snap, k.SnapshotHead()) != nil {
		return nil, ErrFoodFlowJournal
	}
	after, afterHead, err := sched.ExportPortable()
	if err != nil || afterHead != verified || !bytes.Equal(before, after) {
		return nil, ErrFoodFlowJournal
	}
	return batches, nil
}

func verifyFoodFlowJournalScheduler(batches []FoodFlowBatch, snap scheduler.Snapshot, head kernel.Head) error {
	anchor := scheduler.KernelAnchor{Version: head.Version, OriginID: head.OriginID, TipID: head.TipID, TipTime: head.TipTime, TipHash: head.TipHash}
	at := sim.SimTime(0)
	if len(batches) != 0 {
		at = batches[len(batches)-1].Time
	}
	if snap.Kernel != anchor || snap.Time != at || snap.HasClosed != (len(batches) != 0) || snap.Closed != at || len(snap.Fibers) != FoodFlowActorCount+1 {
		return ErrFoodFlowJournal
	}
	open := make(map[sim.EntityID]FoodFlowActivity, FoodFlowActorCount)
	var lastToken uint64
	for _, b := range batches {
		for _, a := range b.Activities {
			lastToken = a.Token
			if a.Completed == 0 {
				if _, exists := open[a.Actor]; exists {
					return ErrFoodFlowJournal
				}
				open[a.Actor] = a
			}
		}
	}
	if snap.NextToken != lastToken {
		return ErrFoodFlowJournal
	}
	for i, fiber := range snap.Fibers {
		actor := sim.EntityID(i + 1)
		if i == FoodFlowActorCount {
			actor = 1001
		}
		if fiber.Actor != actor {
			return ErrFoodFlowJournal
		}
		activity, exists := open[actor]
		if !exists {
			if fiber.Activity != nil {
				return ErrFoodFlowJournal
			}
			continue
		}
		deadline := activity.Started
		switch activity.Kind {
		case strategy.FoodFlowGather:
			deadline += sim.SimTime(foodFlowGatherDuration)
		case strategy.FoodFlowEat:
			deadline += sim.SimTime(foodFlowMealDuration)
		case strategy.FoodFlowRest:
			deadline += sim.SimTime(strategy.FoodFlowRestDuration)
		default:
			return ErrFoodFlowJournal
		}
		if fiber.Activity == nil || fiber.Activity.Token != activity.Token || fiber.Activity.Deadline != deadline || fiber.Activity.InterruptAt != nil {
			return ErrFoodFlowJournal
		}
	}
	wakes := make(map[sim.EntityID]bool, len(open))
	for _, wake := range snap.Wakes {
		if wake.Cause == scheduler.WakeInterruption {
			return ErrFoodFlowJournal
		}
		if wake.Cause != scheduler.WakeCompletion {
			continue
		}
		a, exists := open[wake.Actor]
		if !exists || wakes[wake.Actor] {
			return ErrFoodFlowJournal
		}
		var duration sim.SimTime
		switch a.Kind {
		case strategy.FoodFlowGather:
			duration = sim.SimTime(foodFlowGatherDuration)
		case strategy.FoodFlowEat:
			duration = sim.SimTime(foodFlowMealDuration)
		case strategy.FoodFlowRest:
			duration = sim.SimTime(strategy.FoodFlowRestDuration)
		default:
			return ErrFoodFlowJournal
		}
		if wake.At != a.Started+duration {
			return ErrFoodFlowJournal
		}
		wakes[wake.Actor] = true
	}
	if len(wakes) != len(open) {
		return ErrFoodFlowJournal
	}
	return nil
}

// Phase is tied to the scheduler clock, not a caller-supplied description.
// 0=pulse, 1=claim, 2=Gather completion, 3=meal/rest completion.
func foodFlowJournalYieldMatches(k *kernel.Kernel, yield int64) bool {
	head := k.SnapshotHead()
	for patch := sim.EntityID(1001); patch <= 1002; patch++ {
		row, err := head.Reader.Read(component.ReadRequest{Entity: patch, Component: FoodFlowPatchTypeID, Fields: []sim.FieldID{FoodFlowPatchYieldField}, WorldVersion: head.Version, Authority: head.Authority})
		if err != nil {
			return false
		}
		value, err := row.Value(FoodFlowPatchYieldField)
		if err != nil {
			return false
		}
		n, err := value.Integer()
		if err != nil || n != yield {
			return false
		}
	}
	return true
}

func foodFlowPhase(at sim.SimTime) byte {
	if at < 0 || at > sim.SimTime(FoodFlowHorizonHours)*sim.SimTime(FoodFlowHour) {
		return 255
	}
	offset := at % sim.SimTime(FoodFlowHour)
	switch offset {
	case 0:
		return 0
	case 1:
		return 1
	case 1 + sim.SimTime(foodFlowGatherDuration):
		return 2
	case 1 + sim.SimTime(foodFlowGatherDuration) + sim.SimTime(foodFlowMealDuration):
		return 3
	default:
		return 255
	}
}

type foodFlowDeltaKey struct {
	entity    sim.EntityID
	component sim.ComponentTypeID
	field     sim.FieldID
}

func foodFlowJournalEventSource(ev kernel.Event, kind strategy.FoodFlowAction, actor sim.EntityID, source, slot sim.EntityID) bool {
	if ev.Kind != "component-patch" || ev.RuleVersion != FoodFlowRuleVersion || ev.Cause.World || ev.Cause.Actor != actor {
		return false
	}
	var expected []foodFlowDeltaKey
	switch kind {
	case strategy.FoodFlowGather:
		if ev.Rule != FoodFlowGatherRule || !foodFlowSlotBelongsToPatch(slot, int(source-1001)) {
			return false
		}
		expected = []foodFlowDeltaKey{
			{slot, FoodFlowSlotTypeID, FoodFlowSlotStockField}, {slot, FoodFlowSlotTypeID, FoodFlowSlotGatheredField},
			{actor, FoodFlowBagTypeID, FoodFlowBagUnitsField}, {actor, FoodFlowBagTypeID, FoodFlowBagSourceField},
			{actor, FoodFlowBodyTypeID, FoodFlowBodyLastGatherHourField},
		}
	case strategy.FoodFlowEat:
		if ev.Rule != FoodFlowConsumeRule || slot != 0 {
			return false
		}
		expected = []foodFlowDeltaKey{
			{actor, FoodFlowBagTypeID, FoodFlowBagUnitsField}, {actor, FoodFlowBagTypeID, FoodFlowBagSourceField},
			{actor, FoodFlowBodyTypeID, FoodFlowBodyConsumedField}, {actor, FoodFlowBodyTypeID, FoodFlowBodyEnergyField},
			{actor, FoodFlowBodyTypeID, FoodFlowBodyCapLostField}, {actor, FoodFlowBodyTypeID, FoodFlowBodyHungerField},
		}
	default:
		return false
	}
	if len(ev.Deltas) != len(expected) {
		return false
	}
	seen := make(map[foodFlowDeltaKey]bool, len(expected))
	for _, d := range ev.Deltas {
		key := foodFlowDeltaKey{d.Entity, d.Component, d.Field}
		allowed := false
		for _, want := range expected {
			if key == want {
				allowed = true
				break
			}
		}
		if !allowed || seen[key] {
			return false
		}
		seen[key] = true
		if d.Component == FoodFlowBagTypeID && d.Field == FoodFlowBagSourceField {
			value := d.After
			if kind == strategy.FoodFlowEat {
				value = d.Before
				if d.After.State() != sim.Missing {
					return false
				}
			} else if d.Before.State() != sim.Missing {
				return false
			}
			id, err := value.EntityRef()
			if err != nil || id != source {
				return false
			}
		}
	}
	return true
}

func verifyFoodFlowJournal(batches []FoodFlowBatch, events []kernel.Event, head kernel.PortableHead, at sim.SimTime, config FoodFlowJournalConfig) ([][]foodFlowJournalAttempt, error) {
	fail := func() ([][]foodFlowJournalAttempt, error) { return nil, ErrFoodFlowJournal }
	if config.Yield < 0 || config.Yield > FoodFlowSlotsPerPatch || config.Policy != (strategy.FoodFlowRef{ID: "food-flow", Version: 1}) || at < 0 || at > sim.SimTime(FoodFlowHorizonHours)*sim.SimTime(FoodFlowHour) || len(batches) > MaxFoodFlowJournalBatches || head.GenesisVersion != 0 || head.TipID != sim.EventID(len(events)) || head.Version != sim.WorldVersion(len(events)) {
		return fail()
	}
	fresh, err := NewFoodFlow(FoodFlowOptions{Yield: config.Yield, Seed: config.Seed, Workers: 1})
	if err != nil {
		return fail()
	}
	_, genesis, err := fresh.k.ExportHistory()
	if err != nil || genesis.GenesisHash != head.GenesisHash || genesis.RegistryFingerprint != head.RegistryFingerprint {
		return fail()
	}
	if len(batches) == 0 {
		if at != 0 || len(events) != 0 {
			return fail()
		}
		return [][]foodFlowJournalAttempt{}, nil
	}
	if batches[0].Time != 0 || batches[len(batches)-1].Time != at {
		return fail()
	}
	byID := make(map[sim.EventID]kernel.Event, len(events))
	for _, ev := range events {
		byID[ev.ID] = ev
	}
	records := make([][]foodFlowJournalAttempt, len(batches))
	index, attempts, activities := 0, 0, 0
	var lastTime sim.SimTime = -1
	var lastHash [32]byte
	var lastVersion sim.WorldVersion
	var lastToken uint64
	var open [FoodFlowActorCount + 1]*FoodFlowActivity
	// Only an accepted basal event can reduce energy to zero. Accepted Eat
	// deltas update the same ledger before the next hourly pulse; dead actors
	// cannot be resurrected by an unlinked journal choice.
	var energy [FoodFlowActorCount + 1]int64
	for actor := 1; actor <= FoodFlowActorCount; actor++ {
		energy[actor] = FoodFlowInitialEnergy
	}
	var expectedPhase byte
	for bi, b := range batches {
		phase := foodFlowPhase(b.Time)
		if phase == 255 || b.Time <= lastTime || len(b.Attempts) > FoodFlowActorCount || len(b.Activities) > FoodFlowActorCount || attempts+len(b.Attempts) > MaxFoodFlowJournalAttempts || activities+len(b.Activities) > MaxFoodFlowJournalActivities {
			return fail()
		}
		// A timestamp with only rejected attempts, rest completions or no
		// events still has a scheduler batch. Do not infer its presence from
		// the accepted-event stream alone.
		if phase != expectedPhase || (bi > 0 && b.Time/sim.SimTime(FoodFlowHour) != lastTime/sim.SimTime(FoodFlowHour)+boolHour(phase == 0)) {
			return fail()
		}
		attempts += len(b.Attempts)
		activities += len(b.Activities)
		var completed [FoodFlowActorCount + 1]FoodFlowActivity
		for actor := 1; actor <= FoodFlowActorCount; actor++ {
			a := open[actor]
			if a == nil {
				continue
			}
			if phase == 2 && a.Kind == strategy.FoodFlowGather || phase == 3 && (a.Kind == strategy.FoodFlowEat || a.Kind == strategy.FoodFlowRest) {
				if a.Completed != b.Time {
					return fail()
				}
			} else if a.Completed == b.Time {
				return fail()
			}
			if a.Completed == b.Time {
				completed[actor] = *a
				open[actor] = nil
			}
		}
		// World events precede the accepted actor proposals at this timestamp.
		if phase == 0 {
			h := int(b.Time / sim.SimTime(FoodFlowHour))
			// The scheduler pulses only actors alive immediately before this
			// integer hour, in actor order. Production remains independent.
			var basalActors []sim.EntityID
			if h > 0 {
				for actor := 1; actor <= FoodFlowActorCount; actor++ {
					if energy[actor] > 0 {
						basalActors = append(basalActors, sim.EntityID(actor))
					}
				}
			}
			wanted := len(basalActors)
			if h < FoodFlowHorizonHours {
				wanted += FoodFlowPatchCount
			}
			for n := 0; n < wanted; n++ {
				if index >= len(events) {
					return fail()
				}
				ev := events[index]
				kind, actor, rule := "produce-0", sim.EntityID(0), FoodFlowProduceRule
				if n < len(basalActors) {
					kind = "basal"
					actor = basalActors[n]
					rule = FoodFlowBasalRule
				} else if n == len(basalActors)+1 {
					kind = "produce-1"
				}
				if ev.Time != b.Time || ev.Key != foodFlowKey(kind, b.Time, actor) || ev.Rule != rule || ev.RuleVersion != FoodFlowRuleVersion || ev.Kind != "component-patch" || !ev.Cause.World || ev.Cause.Actor != 0 {
					return fail()
				}
				if rule == FoodFlowBasalRule {
					if len(ev.Deltas) != 3 {
						return fail()
					}
					fields := map[sim.FieldID]bool{FoodFlowBodyEnergyField: false, FoodFlowBodyHungerField: false, FoodFlowBodyBasalSpentField: false}
					for _, d := range ev.Deltas {
						used, exists := fields[d.Field]
						if d.Entity != actor || d.Component != FoodFlowBodyTypeID || !exists || used {
							return fail()
						}
						fields[d.Field] = true
						if d.Field == FoodFlowBodyEnergyField {
							before, beforeErr := d.Before.Integer()
							after, afterErr := d.After.Integer()
							if beforeErr != nil || afterErr != nil || before != energy[actor] || before <= 0 || after != before-1 {
								return fail()
							}
							energy[actor] = after
						}
					}
					for _, used := range fields {
						if !used {
							return fail()
						}
					}
				} else {
					patch := 0
					if kind == "produce-1" {
						patch = 1
					}
					fields := map[sim.FieldID]bool{FoodFlowPatchPulsesField: false, FoodFlowPatchProducedField: false, FoodFlowPatchUnrealizedField: false}
					seenSlots := make(map[sim.EntityID]bool, FoodFlowSlotsPerPatch)
					for _, d := range ev.Deltas {
						switch d.Component {
						case FoodFlowPatchTypeID:
							used, exists := fields[d.Field]
							if d.Entity != sim.EntityID(1001+patch) || !exists || used {
								return fail()
							}
							fields[d.Field] = true
						case FoodFlowSlotTypeID:
							if !foodFlowSlotBelongsToPatch(d.Entity, patch) || d.Field != FoodFlowSlotStockField || seenSlots[d.Entity] {
								return fail()
							}
							seenSlots[d.Entity] = true
						default:
							return fail()
						}
					}
					for _, used := range fields {
						if !used {
							return fail()
						}
					}
				}
				index++
			}
		}
		records[bi] = make([]foodFlowJournalAttempt, len(b.Attempts))
		var seen [FoodFlowActorCount + 1]bool
		accepted := make([]FoodFlowAttempt, 0, len(b.Attempts))
		for j, a := range b.Attempts {
			c := a.Choice
			if a.Actor < 1 || a.Actor > FoodFlowActorCount || seen[a.Actor] || a.Time != b.Time || c.Ref != config.Policy || c.ObservedVersion > lastVersion || c.Evaluated < 0 || c.Evaluated > 16 || c.Fallback > strategy.FoodFlowInvalidSelf || a.Rejection > FoodFlowGatherDuplicateActor {
				return fail()
			}
			seen[a.Actor] = true
			if j > 0 && b.Attempts[j-1].Actor >= a.Actor {
				return fail()
			}
			kind := ""
			switch c.Kind {
			case strategy.FoodFlowWait:
				kind = "wait"
				if phase != 1 || a.Accepted || a.EventID != 0 || a.Rejection != FoodFlowGatherIneligible || c.TargetPatch != 0 || c.TargetSlot != 0 || c.RestDuration != 0 || c.Evaluated != 0 || c.Fallback == strategy.FoodFlowNoFallback {
					return fail()
				}
			case strategy.FoodFlowGather:
				kind = "gather-denied"
				if phase != 2 || c.Fallback != strategy.FoodFlowNoFallback || c.Evaluated < 1 || c.RestDuration != 0 || c.TargetPatch == 0 || !foodFlowSlotBelongsToPatch(c.TargetSlot, int(c.TargetPatch-1001)) || completed[a.Actor].Kind != strategy.FoodFlowGather || c.ObservedVersion != lastVersion {
					return fail()
				}
				if a.Accepted {
					kind = "gather"
					if a.Rejection != FoodFlowGatherAdmitted {
						return fail()
					}
				} else if a.EventID != 0 || a.Rejection == FoodFlowGatherAdmitted {
					return fail()
				}
			case strategy.FoodFlowEat:
				kind = "eat"
				if phase != 3 || !a.Accepted || a.Rejection != FoodFlowGatherAdmitted || c.Fallback != strategy.FoodFlowNoFallback || c.Evaluated != 1 || c.TargetSlot != 0 || c.RestDuration != 0 || c.ObservedVersion != lastVersion || completed[a.Actor].Kind != strategy.FoodFlowEat {
					return fail()
				}
			default:
				return fail()
			}
			if c.Kind == strategy.FoodFlowGather || c.Kind == strategy.FoodFlowEat {
				home, _ := FoodFlowActorPatchID(a.Actor)
				if c.TargetPatch != home { // No-access Gather denials are evidence, not a valid accepted source.
					if c.Kind != strategy.FoodFlowGather || a.Rejection != FoodFlowGatherNoAccess {
						return fail()
					}
				}
			}
			if phase == 1 && (energy[a.Actor] <= 0 || c.ObservedVersion != lastVersion) {
				return fail()
			}
			if a.Key != foodFlowKey(kind, b.Time, a.Actor) || len(a.Key) > 128 {
				return fail()
			}
			if a.Accepted {
				if a.EventID == 0 {
					return fail()
				}
				accepted = append(accepted, a)
			}
			records[bi][j] = foodFlowJournalAttempt{FoodFlowAttempt: a, Phase: phase, Rank: foodFlowNoRank, SourcePatch: c.TargetPatch}
		}
		sort.Slice(accepted, func(i, j int) bool { return accepted[i].Key < accepted[j].Key })
		for _, a := range accepted {
			if index >= len(events) {
				return fail()
			}
			ev := events[index]
			rec := &records[bi][indexOfFoodFlowAttempt(b.Attempts, a.Actor)]
			if ev.ID != a.EventID || ev.Key != a.Key || ev.Time != a.Time || ev.BeforeVersion != sim.WorldVersion(index) || ev.AfterVersion != sim.WorldVersion(index+1) || ev.PreviousHash != lastHash {
				return fail()
			}
			if a.Choice.Kind == strategy.FoodFlowGather {
				for _, d := range ev.Deltas {
					if d.Component == FoodFlowSlotTypeID {
						rec.Slot = d.Entity
						break
					}
				}
			}
			if !foodFlowJournalEventSource(ev, a.Choice.Kind, a.Actor, a.Choice.TargetPatch, rec.Slot) || energy[a.Actor] <= 0 {
				return fail()
			}
			if a.Choice.Kind == strategy.FoodFlowEat {
				for _, d := range ev.Deltas {
					if d.Component == FoodFlowBodyTypeID && d.Field == FoodFlowBodyEnergyField {
						before, beforeErr := d.Before.Integer()
						after, afterErr := d.After.Integer()
						if beforeErr != nil || afterErr != nil || before != energy[a.Actor] || after < before || after > FoodFlowEnergyCapacity {
							return fail()
						}
						energy[a.Actor] = after
					}
				}
			}
			index++
			lastHash = ev.Hash
		}
		// The pulse (if any) also advances the accepted hash chain.
		if index > 0 {
			lastHash = events[index-1].Hash
		}
		if b.Version != sim.WorldVersion(index) || b.TipID != sim.EventID(index) || b.TipHash != lastHash {
			return fail()
		}
		for _, a := range b.Attempts {
			if a.Choice.Kind == strategy.FoodFlowGather || a.Choice.Kind == strategy.FoodFlowEat {
				if completed[a.Actor].EventID != a.EventID {
					return fail()
				}
			}
		}
		for actor := 1; actor <= FoodFlowActorCount; actor++ {
			c := completed[actor]
			if c.Kind == strategy.FoodFlowGather && !seen[actor] {
				return fail()
			}
			if c.Kind == strategy.FoodFlowEat && (!seen[actor] || b.Attempts[indexOfFoodFlowAttempt(b.Attempts, sim.EntityID(actor))].Choice.Kind != strategy.FoodFlowEat) {
				return fail()
			}
			if c.Kind == strategy.FoodFlowRest && seen[actor] {
				return fail()
			}
		}
		var previous sim.EntityID
		for _, a := range b.Activities {
			duration := sim.SimTime(0)
			switch a.Kind {
			case strategy.FoodFlowGather:
				duration = sim.SimTime(foodFlowGatherDuration)
			case strategy.FoodFlowEat:
				duration = sim.SimTime(foodFlowMealDuration)
			case strategy.FoodFlowRest:
				duration = sim.SimTime(strategy.FoodFlowRestDuration)
			default:
				return fail()
			}
			if a.Actor <= previous || a.Actor > FoodFlowActorCount || a.Started != b.Time || a.Token != lastToken+1 || open[a.Actor] != nil || (a.Completed != 0 && (a.Completed != a.Started+duration || a.Completed > at)) || (a.Completed == 0 && a.EventID != 0) || (a.Kind == strategy.FoodFlowRest && a.EventID != 0) {
				return fail()
			}
			if a.Kind == strategy.FoodFlowGather && (phase != 1 || seen[a.Actor]) || a.Kind != strategy.FoodFlowGather && phase != 2 {
				return fail()
			}
			if a.Kind == strategy.FoodFlowEat && (!seen[a.Actor] || !b.Attempts[indexOfFoodFlowAttempt(b.Attempts, a.Actor)].Accepted) || a.Kind == strategy.FoodFlowRest && (!seen[a.Actor] || b.Attempts[indexOfFoodFlowAttempt(b.Attempts, a.Actor)].Accepted) {
				return fail()
			}
			if a.Completed == 0 && a.Started+duration <= at {
				return fail()
			}
			if a.Completed != 0 {
				if a.Kind == strategy.FoodFlowEat || a.Kind == strategy.FoodFlowGather {
					ev, ok := byID[a.EventID]
					if (a.EventID != 0 && (!ok || ev.Time != a.Completed || ev.Cause.Actor != a.Actor || (a.Kind == strategy.FoodFlowGather && ev.Rule != FoodFlowGatherRule) || (a.Kind == strategy.FoodFlowEat && ev.Rule != FoodFlowConsumeRule))) || (a.Kind == strategy.FoodFlowEat && a.EventID == 0) {
						return fail()
					}
				}
			}
			copy := a
			open[a.Actor] = &copy
			previous = a.Actor
			lastToken = a.Token
		}
		// Compute only the ordinal of recorded eligible competitors. Neither the
		// stock nor a rejected reason can be authenticated from an event link.
		if phase == 2 {
			for patch := 0; patch < FoodFlowPatchCount; patch++ {
				eligible := make([]int, 0, FoodFlowSlotsPerPatch)
				for j, a := range b.Attempts {
					if a.Choice.Kind == strategy.FoodFlowGather && a.Choice.TargetPatch == sim.EntityID(1001+patch) && (a.Accepted || a.Rejection == FoodFlowGatherNoStock || a.Rejection == FoodFlowGatherCapacity) {
						eligible = append(eligible, j)
					}
				}
				hour := uint64(b.Time / sim.SimTime(FoodFlowHour))
				start := (config.Seed + hour + uint64(patch)) % FoodFlowSlotsPerPatch
				sort.Slice(eligible, func(i, j int) bool {
					rank := func(index int) uint64 {
						return (uint64((b.Attempts[index].Actor-1)%FoodFlowSlotsPerPatch) + FoodFlowSlotsPerPatch - start) % FoodFlowSlotsPerPatch
					}
					return rank(eligible[i]) < rank(eligible[j])
				})
				for rank, j := range eligible {
					records[bi][j].Rank = byte(rank)
				}
			}
		}
		switch phase {
		case 0:
			if len(b.Attempts) != 0 || len(b.Activities) != 0 {
				return fail()
			}
			expectedPhase = 0
			if b.Time < sim.SimTime(FoodFlowHorizonHours)*sim.SimTime(FoodFlowHour) {
				for actor := 1; actor <= FoodFlowActorCount; actor++ {
					if energy[actor] > 0 {
						expectedPhase = 1
						break
					}
				}
			}
		case 1:
			for actor := 1; actor <= FoodFlowActorCount; actor++ {
				if (energy[actor] > 0) != (seen[actor] || open[actor] != nil) || seen[actor] && open[actor] != nil {
					return fail()
				}
			}
			expectedPhase = 0
			if len(b.Activities) != 0 {
				expectedPhase = 2
			}
		case 2:
			expectedPhase = 0
			if len(b.Activities) != 0 {
				expectedPhase = 3
			}
		case 3:
			expectedPhase = 0
		}
		lastVersion = b.Version
		lastTime = b.Time
	}
	if index != len(events) || head.TipHash != lastHash || head.Version != lastVersion || head.TipTime != eventsTipTime(events) {
		return fail()
	}
	return records, nil
}
func boolHour(value bool) sim.SimTime {
	if value {
		return 1
	}
	return 0
}
func indexOfFoodFlowAttempt(attempts []FoodFlowAttempt, actor sim.EntityID) int {
	for i, a := range attempts {
		if a.Actor == actor {
			return i
		}
	}
	return -1
}
func eventsTipTime(events []kernel.Event) sim.SimTime {
	if len(events) == 0 {
		return 0
	}
	return events[len(events)-1].Time
}
