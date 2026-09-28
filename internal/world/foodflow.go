package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var ErrFoodFlowRunner = errors.New("invalid food-flow runner")

const (
	foodFlowGatherDuration sim.Duration = 20 * 60 * 1_000_000
	foodFlowMealDuration   sim.Duration = 10 * 60 * 1_000_000
)

type FoodFlowOptions struct {
	Yield   int64
	Seed    uint64
	Workers int
}
type FoodFlowAttempt struct {
	Actor     sim.EntityID
	Time      sim.SimTime
	Key       string
	Choice    strategy.FoodFlowChoice
	Rejection FoodFlowGatherRejection
	Accepted  bool
	EventID   sim.EventID
}
type FoodFlowActivity struct {
	Actor              sim.EntityID
	Kind               strategy.FoodFlowAction
	Started, Completed sim.SimTime
	Token              uint64
	EventID            sim.EventID // Gather/Eat completion event, zero for Rest or denied Gather
}
type FoodFlowBatch struct {
	Time       sim.SimTime
	Version    sim.WorldVersion
	TipID      sim.EventID
	TipHash    [32]byte
	Attempts   []FoodFlowAttempt
	Activities []FoodFlowActivity
}

// FoodFlowHandoff is the value-only, quiescent input to the persistence lane.
// History and SchedulerBytes must be persisted together with the verified head;
// Current and Choices preserve in-flight timed intentions across a restore.
type FoodFlowHandoff struct {
	FormatVersion  uint32
	Yield          int64
	Seed           uint64
	Policy         strategy.FoodFlowRef
	Steps          int
	History        []byte
	SchedulerBytes []byte
	Head           kernel.PortableHead
	Current        map[sim.EntityID]strategy.FoodFlowAction
	Choices        map[sim.EntityID]strategy.FoodFlowChoice
	Journal        []FoodFlowBatch
	Checkpoints    []FoodFlowCheckpoint
}

type FoodFlowCheckpoint struct {
	Hour    int
	Version sim.WorldVersion
	Balance FoodFlowBalance
	Patches [FoodFlowPatchCount]FoodFlowPatchState
	Slots   [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState
	Actors  [FoodFlowActorCount]FoodFlowActorState
	Alive   int
}

// FoodFlow is an independent synthetic runner. Kernel events are authoritative;
// the in-memory batches are attempt/activity evidence, not another state store.
// Kernel and SchedulerSnapshot are the persistence lane's quiescent handoff.
type FoodFlow struct {
	mu             sync.Mutex
	k              *kernel.Kernel
	sched          *scheduler.Scheduler
	registry       component.Registry
	seeds          []component.ComponentSeed
	bound          [FoodFlowActorCount]*strategy.FoodFlowBound
	ref            strategy.FoodFlowRef
	seed           uint64
	yield          int64
	current        map[sim.EntityID]strategy.FoodFlowAction
	choices        map[sim.EntityID]strategy.FoodFlowChoice
	admissions     map[sim.EntityID]FoodFlowGatherAdmission
	pulseLifecycle [FoodFlowActorCount]scheduler.Lifecycle
	pending        *foodFlowPending
	journal        []FoodFlowBatch
	checkpoints    []FoodFlowCheckpoint
	steps          int
}
type foodFlowPending struct {
	mu        sync.Mutex
	attempts  []FoodFlowAttempt
	choices   map[sim.EntityID]strategy.FoodFlowChoice
	started   map[sim.EntityID]strategy.FoodFlowAction
	completed []sim.EntityID
}

func foodFlowSeed(entity sim.EntityID, typ sim.ComponentTypeID, fields ...component.FieldSeed) component.ComponentSeed {
	return component.ComponentSeed{Entity: entity, Component: typ, Fields: fields}
}
func foodFlowField(field sim.FieldID, n int64) component.FieldSeed {
	return component.FieldSeed{Field: field, Value: sim.IntegerValue(n)}
}
func foodFlowPatch(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, v sim.Value) component.Patch {
	return component.Patch{Entity: entity, Component: typ, SchemaVersion: FoodFlowSchemaVersion, Field: field, Value: v}
}
func foodFlowNumber(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, n int64) component.Patch {
	return foodFlowPatch(entity, typ, field, sim.IntegerValue(n))
}
func foodFlowRef(id sim.EntityID) sim.Value { v, _ := sim.EntityRefValue(id); return v }
func foodFlowMissing() sim.Value            { v, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing); return v }

func NewFoodFlow(o FoodFlowOptions) (*FoodFlow, error) {
	if o.Workers < 1 || o.Yield < 0 || o.Yield > FoodFlowSlotsPerPatch {
		return nil, ErrFoodFlowRunner
	}
	reg, err := FoodFlowRegistry()
	if err != nil {
		return nil, err
	}
	p := strategy.FoodFlowPolicy{FormatVersion: strategy.FoodFlowPolicyFormatV1, Ref: strategy.FoodFlowRef{ID: "food-flow", Version: 1}, Budget: strategy.FoodFlowBudget{Candidates: 16, Evaluations: 16}, RestDuration: strategy.FoodFlowRestDuration}
	policies := strategy.NewFoodFlowRegistry()
	if err = policies.Register(p); err != nil {
		return nil, err
	}
	f := &FoodFlow{registry: reg, ref: p.Ref, seed: o.Seed, yield: o.Yield, current: make(map[sim.EntityID]strategy.FoodFlowAction), choices: make(map[sim.EntityID]strategy.FoodFlowChoice)}
	for i := 0; i < FoodFlowPatchCount; i++ {
		patch, _ := FoodFlowPatchID(i)
		f.seeds = append(f.seeds, foodFlowSeed(patch, FoodFlowPatchTypeID, foodFlowField(FoodFlowPatchYieldField, o.Yield), foodFlowField(FoodFlowPatchPulsesField, 0), foodFlowField(FoodFlowPatchProducedField, 0), foodFlowField(FoodFlowPatchUnrealizedField, 0)))
		for j := 0; j < FoodFlowSlotsPerPatch; j++ {
			slot, _ := FoodFlowSlotID(i, j)
			f.seeds = append(f.seeds, foodFlowSeed(slot, FoodFlowSlotTypeID, foodFlowField(FoodFlowSlotStockField, 0), component.FieldSeed{Field: FoodFlowSlotPatchField, Value: foodFlowRef(patch)}, foodFlowField(FoodFlowSlotGatheredField, 0)))
		}
	}
	for i := 0; i < FoodFlowActorCount; i++ {
		actor := sim.EntityID(i + 1)
		f.seeds = append(f.seeds, foodFlowSeed(actor, FoodFlowBagTypeID, foodFlowField(FoodFlowBagUnitsField, 0), component.FieldSeed{Field: FoodFlowBagSourceField, Value: foodFlowMissing()}), foodFlowSeed(actor, FoodFlowBodyTypeID, foodFlowField(FoodFlowBodyEnergyField, FoodFlowInitialEnergy), foodFlowField(FoodFlowBodyHungerField, 0), foodFlowField(FoodFlowBodyBasalSpentField, 0), foodFlowField(FoodFlowBodyCapLostField, 0), foodFlowField(FoodFlowBodyConsumedField, 0), foodFlowField(FoodFlowBodyLastGatherHourField, FoodFlowNeverGatheredHour)))
		f.bound[i], err = policies.Bind(strategy.FoodFlowBinding{Actor: actor, Ref: p.Ref})
		if err != nil {
			return nil, err
		}
	}
	f.k, err = kernel.New(reg, 0, f.seeds)
	if err != nil {
		return nil, err
	}
	f.sched, err = scheduler.New(f.k, o.Workers, f.evaluate)
	if err != nil {
		return nil, err
	}
	for i := 0; i < FoodFlowActorCount; i++ {
		if err = f.sched.Register(sim.EntityID(i + 1)); err != nil {
			return nil, err
		}
	}
	if err = f.sched.Register(1001); err != nil {
		return nil, err
	}
	if err = f.sched.Schedule(scheduler.Wake{Actor: 1001, At: 0, Cause: scheduler.WakeAudit}); err != nil {
		return nil, err
	}
	return f, nil
}

func foodFlowRead(view scheduler.SnapshotView, entity sim.EntityID, typ sim.ComponentTypeID, fields ...sim.FieldID) ([]sim.Value, error) {
	row, err := view.Reader.Read(component.ReadRequest{Entity: entity, Component: typ, Fields: fields, WorldVersion: view.Version, Authority: view.Authority})
	if err != nil {
		return nil, err
	}
	out := make([]sim.Value, len(fields))
	for i, field := range fields {
		out[i], err = row.Value(field)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func foodFlowInts(view scheduler.SnapshotView, entity sim.EntityID, typ sim.ComponentTypeID, fields ...sim.FieldID) ([]int64, error) {
	values, err := foodFlowRead(view, entity, typ, fields...)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(values))
	for i, v := range values {
		out[i], err = v.Integer()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func foodFlowStates(view scheduler.SnapshotView) (patches [FoodFlowPatchCount]FoodFlowPatchState, slots [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState, actors [FoodFlowActorCount]FoodFlowActorState, err error) {
	for i := range patches {
		patch, _ := FoodFlowPatchID(i)
		var n []int64
		n, err = foodFlowInts(view, patch, FoodFlowPatchTypeID, FoodFlowPatchYieldField, FoodFlowPatchPulsesField, FoodFlowPatchProducedField, FoodFlowPatchUnrealizedField)
		if err != nil {
			return
		}
		patches[i] = FoodFlowPatchState{n[0], n[1], n[2], n[3]}
		for j := range slots[i] {
			slot, _ := FoodFlowSlotID(i, j)
			n, err = foodFlowInts(view, slot, FoodFlowSlotTypeID, FoodFlowSlotStockField, FoodFlowSlotGatheredField)
			if err != nil {
				return
			}
			slots[i][j] = FoodFlowSlotState{n[0], n[1]}
		}
	}
	for i := range actors {
		actor := sim.EntityID(i + 1)
		var n []int64
		n, err = foodFlowInts(view, actor, FoodFlowBodyTypeID, FoodFlowBodyEnergyField, FoodFlowBodyHungerField, FoodFlowBodyBasalSpentField, FoodFlowBodyCapLostField, FoodFlowBodyConsumedField, FoodFlowBodyLastGatherHourField)
		if err != nil {
			return
		}
		var b []sim.Value
		b, err = foodFlowRead(view, actor, FoodFlowBagTypeID, FoodFlowBagUnitsField, FoodFlowBagSourceField)
		if err != nil {
			return
		}
		var held int64
		held, err = b[0].Integer()
		if err != nil {
			return
		}
		var source sim.EntityID
		if held == 1 {
			source, err = b[1].EntityRef()
			if err != nil {
				return
			}
		} else if b[1].State() != sim.Missing {
			err = ErrFoodFlowRunner
			return
		}
		actors[i] = FoodFlowActorState{Bag: held, BagSource: source, Energy: n[0], Hunger: n[1], BasalSpent: n[2], CapLost: n[3], Consumed: n[4], LastGatherHour: n[5]}
	}
	return
}
func foodFlowKey(kind string, at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("foodflow/%s/%016x/%04x", kind, uint64(at), uint64(actor))
}
func foodFlowProposal(kind string, at sim.SimTime, actor sim.EntityID, rule sim.RuleID, patches []component.Patch) kernel.Proposal {
	return kernel.Proposal{Key: foodFlowKey(kind, at, actor), Time: at, Cause: kernel.Cause{Actor: actor, World: actor == 0}, Rule: rule, RuleVersion: FoodFlowRuleVersion, Patches: patches}
}
func (f *FoodFlow) pulse(ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	h := int(ready.At / sim.SimTime(FoodFlowHour))
	at, _ := FoodFlowHourTime(h)
	if ready.At != at || h > FoodFlowHorizonHours {
		return scheduler.Evaluation{}, ErrFoodFlowRunner
	}
	patches, slots, actors, err := foodFlowStates(view)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	var result scheduler.Evaluation
	for i, a := range actors {
		lifecycle := f.pulseLifecycle[i]
		if lifecycle != scheduler.Alive && lifecycle != scheduler.Stopped || (lifecycle == scheduler.Alive) != (a.Energy > 0) {
			return result, ErrFoodFlowRunner
		}
		if lifecycle == scheduler.Stopped || h == 0 {
			continue
		}
		next, _, e := FoodFlowBasal(a, 1)
		if e != nil {
			return result, e
		}
		id := sim.EntityID(i + 1)
		ps := []component.Patch{foodFlowNumber(id, FoodFlowBodyTypeID, FoodFlowBodyEnergyField, next.Energy), foodFlowNumber(id, FoodFlowBodyTypeID, FoodFlowBodyHungerField, next.Hunger), foodFlowNumber(id, FoodFlowBodyTypeID, FoodFlowBodyBasalSpentField, next.BasalSpent)}
		proposal := foodFlowProposal("basal", at, id, FoodFlowBasalRule, ps)
		proposal.Cause = kernel.Cause{World: true} // elapsed need is a world pulse, not an actor action
		result.Proposals = append(result.Proposals, proposal)
		if next.Energy == 0 {
			result.Effects = append(result.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
		}
		actors[i] = next
	}
	if h < FoodFlowHorizonHours {
		for i, p := range patches {
			next, stock, _, e := FoodFlowProduce(p, slots[i], h)
			if e != nil {
				return result, e
			}
			id, _ := FoodFlowPatchID(i)
			ps := []component.Patch{foodFlowNumber(id, FoodFlowPatchTypeID, FoodFlowPatchPulsesField, next.Pulses), foodFlowNumber(id, FoodFlowPatchTypeID, FoodFlowPatchProducedField, next.Produced), foodFlowNumber(id, FoodFlowPatchTypeID, FoodFlowPatchUnrealizedField, next.Unrealized)}
			for j, s := range stock {
				if s.Stock != slots[i][j].Stock {
					slot, _ := FoodFlowSlotID(i, j)
					ps = append(ps, foodFlowNumber(slot, FoodFlowSlotTypeID, FoodFlowSlotStockField, s.Stock))
				}
			}
			result.Proposals = append(result.Proposals, foodFlowProposal(fmt.Sprintf("produce-%d", i), at, 0, FoodFlowProduceRule, ps))
		}
		claim, _ := FoodFlowClaimTime(h)
		for i, a := range actors {
			if a.Energy > 0 {
				result.Effects = append(result.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: sim.EntityID(i + 1), Wake: scheduler.Wake{Actor: sim.EntityID(i + 1), At: claim, Cause: scheduler.WakeNeedThreshold}})
			}
		}
	}
	if h < FoodFlowHorizonHours {
		next, _ := FoodFlowHourTime(h + 1)
		result.Effects = append(result.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: 1001, Wake: scheduler.Wake{Actor: 1001, At: next, Cause: scheduler.WakeAudit}})
	}
	return result, nil
}

func (f *FoodFlow) observe(actor sim.EntityID, at sim.SimTime, view scheduler.SnapshotView, denial strategy.FoodFlowDenial) (strategy.FoodFlowObservation, error) {
	body, err := foodFlowRead(view, actor, FoodFlowBodyTypeID, FoodFlowBodyEnergyField, FoodFlowBodyHungerField, FoodFlowBodyLastGatherHourField)
	if err != nil {
		return strategy.FoodFlowObservation{}, err
	}
	bag, err := foodFlowRead(view, actor, FoodFlowBagTypeID, FoodFlowBagUnitsField, FoodFlowBagSourceField)
	if err != nil {
		return strategy.FoodFlowObservation{}, err
	}
	patch, _ := FoodFlowActorPatchID(actor)
	obs := strategy.FoodFlowObservation{Actor: actor, Time: at, WorldVersion: view.Version, Energy: body[0], Hunger: body[1], LastGatherHour: body[2], BagUnits: bag[0], BagSource: bag[1], LastDenial: denial, Patches: []strategy.FoodFlowPatchView{{ID: patch}}}
	index := int(patch - 1001)
	for j := 0; j < FoodFlowSlotsPerPatch; j++ {
		id, _ := FoodFlowSlotID(index, j)
		values, e := foodFlowRead(view, id, FoodFlowSlotTypeID, FoodFlowSlotStockField)
		if e != nil {
			return obs, e
		}
		obs.Patches[0].Slots = append(obs.Patches[0].Slots, strategy.FoodFlowSlotView{ID: id, Stock: values[0]})
	}
	return obs, nil
}
func (f *FoodFlow) evaluate(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	if ready.Fiber.Actor == 1001 {
		return f.pulse(ready, view)
	}
	actor := ready.Fiber.Actor
	if actor < 1 || actor > FoodFlowActorCount {
		return scheduler.Evaluation{}, ErrFoodFlowRunner
	}
	if len(ready.Causes) != 1 {
		return scheduler.Evaluation{}, ErrFoodFlowRunner
	}
	var out scheduler.Evaluation
	switch ready.Causes[0] {
	case scheduler.WakeNeedThreshold:
		obs, err := f.observe(actor, ready.At, view, strategy.FoodFlowNoDenial)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[actor-1].Evaluate(obs)
		if err != nil {
			return out, err
		}
		if choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrFoodFlowRunner
		}
		f.pending.mu.Lock()
		if choice.Kind == strategy.FoodFlowGather {
			f.pending.started[actor] = strategy.FoodFlowGather
			f.pending.choices[actor] = choice
		} else {
			f.pending.attempts = append(f.pending.attempts, FoodFlowAttempt{Actor: actor, Time: ready.At, Key: foodFlowKey("wait", ready.At, actor), Choice: choice, Rejection: FoodFlowGatherIneligible})
		}
		f.pending.mu.Unlock()
		if choice.Kind == strategy.FoodFlowGather {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: actor, Duration: foodFlowGatherDuration})
		} else if choice.Kind != strategy.FoodFlowWait {
			return out, ErrFoodFlowRunner
		}
	case scheduler.WakeCompletion:
		kind := f.current[actor]
		if kind == 0 {
			return out, ErrFoodFlowRunner
		}
		f.pending.mu.Lock()
		f.pending.completed = append(f.pending.completed, actor)
		f.pending.mu.Unlock()
		switch kind {
		case strategy.FoodFlowGather:
			choice := f.choices[actor]
			admission, ok := f.admissions[actor]
			if !ok {
				return out, ErrFoodFlowRunner
			}
			a := FoodFlowAttempt{Actor: actor, Time: ready.At, Key: foodFlowKey("gather-denied", ready.At, actor), Choice: choice, Rejection: admission.Rejection}
			if admission.Rejection == FoodFlowGatherAdmitted {
				if choice.ObservedVersion > view.Version || admission.Patch != choice.TargetPatch {
					return out, ErrFoodFlowRunner
				}
				_, slots, actors, err := foodFlowStates(view)
				if err != nil {
					return out, err
				}
				slotIndex := int(admission.Slot) - 2001 - int(admission.Patch-1001)*FoodFlowSlotsPerPatch
				if slotIndex < 0 || slotIndex >= FoodFlowSlotsPerPatch {
					return out, ErrFoodFlowRunner
				}
				hour := int(ready.At / sim.SimTime(FoodFlowHour))
				nextSlot, nextActor, err := FoodFlowGather(admission.Patch, admission.Slot, actor, hour, slots[admission.Patch-1001][slotIndex], actors[actor-1])
				if err != nil {
					return out, err
				}
				ps := []component.Patch{foodFlowNumber(admission.Slot, FoodFlowSlotTypeID, FoodFlowSlotStockField, nextSlot.Stock), foodFlowNumber(admission.Slot, FoodFlowSlotTypeID, FoodFlowSlotGatheredField, nextSlot.Gathered), foodFlowNumber(actor, FoodFlowBagTypeID, FoodFlowBagUnitsField, nextActor.Bag), foodFlowPatch(actor, FoodFlowBagTypeID, FoodFlowBagSourceField, foodFlowRef(nextActor.BagSource)), foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyLastGatherHourField, nextActor.LastGatherHour)}
				p := foodFlowProposal("gather", ready.At, actor, FoodFlowGatherRule, ps)
				a.Key = p.Key
				out.Proposals = append(out.Proposals, p)
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: actor, Duration: foodFlowMealDuration, IfKey: p.Key})
				f.pending.mu.Lock()
				f.pending.started[actor] = strategy.FoodFlowEat
				f.pending.mu.Unlock()
			} else {
				obs, err := f.observe(actor, ready.At, view, strategy.FoodFlowGatherDenied)
				if err != nil {
					return out, err
				}
				rest, err := f.bound[actor-1].Evaluate(obs)
				if err != nil || rest.Kind != strategy.FoodFlowRest || rest.Ref != f.ref || rest.ObservedVersion != view.Version || rest.RestDuration != strategy.FoodFlowRestDuration {
					return out, ErrFoodFlowRunner
				}
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: actor, Duration: rest.RestDuration})
				f.pending.mu.Lock()
				f.pending.started[actor] = strategy.FoodFlowRest
				f.pending.mu.Unlock()
			}
			f.pending.mu.Lock()
			f.pending.attempts = append(f.pending.attempts, a)
			f.pending.mu.Unlock()
		case strategy.FoodFlowEat:
			obs, err := f.observe(actor, ready.At, view, strategy.FoodFlowNoDenial)
			if err != nil {
				return out, err
			}
			choice, err := f.bound[actor-1].Evaluate(obs)
			if err != nil || choice.Kind != strategy.FoodFlowEat {
				return out, ErrFoodFlowRunner
			}
			_, _, actors, err := foodFlowStates(view)
			if err != nil {
				return out, err
			}
			next, _, err := FoodFlowConsume(choice.TargetPatch, actor, actors[actor-1])
			if err != nil {
				return out, err
			}
			source := foodFlowMissing()
			ps := []component.Patch{foodFlowNumber(actor, FoodFlowBagTypeID, FoodFlowBagUnitsField, next.Bag), foodFlowPatch(actor, FoodFlowBagTypeID, FoodFlowBagSourceField, source), foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyConsumedField, next.Consumed), foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyEnergyField, next.Energy), foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyCapLostField, next.CapLost), foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyHungerField, next.Hunger)}
			p := foodFlowProposal("eat", ready.At, actor, FoodFlowConsumeRule, ps)
			out.Proposals = append(out.Proposals, p)
			f.pending.mu.Lock()
			f.pending.attempts = append(f.pending.attempts, FoodFlowAttempt{Actor: actor, Time: ready.At, Key: p.Key, Choice: choice})
			f.pending.mu.Unlock()
		case strategy.FoodFlowRest: // no physiological bonus; basal is independent
		default:
			return out, ErrFoodFlowRunner
		}
	default:
		return out, ErrFoodFlowRunner
	}
	return out, nil
}

func (f *FoodFlow) allocate(at sim.SimTime) error {
	snap := f.sched.Snapshot()
	f.admissions = make(map[sim.EntityID]FoodFlowGatherAdmission)
	var gather []sim.EntityID
	for _, wake := range snap.Wakes {
		if wake.At == at && wake.Cause == scheduler.WakeCompletion && f.current[wake.Actor] == strategy.FoodFlowGather {
			gather = append(gather, wake.Actor)
		}
	}
	if len(gather) == 0 {
		return nil
	}
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	_, slots, actors, err := foodFlowStates(view)
	if err != nil {
		return err
	}
	var stock []FoodFlowStockedSlot
	var claims []FoodFlowGatherClaim
	for i := range slots {
		for j, s := range slots[i] {
			if s.Stock == 1 {
				patch, _ := FoodFlowPatchID(i)
				slot, _ := FoodFlowSlotID(i, j)
				stock = append(stock, FoodFlowStockedSlot{ID: slot, Patch: patch})
			}
		}
	}
	for _, actor := range gather {
		choice := f.choices[actor]
		a := actors[actor-1]
		claims = append(claims, FoodFlowGatherClaim{Actor: actor, TargetPatch: choice.TargetPatch, TargetSlot: choice.TargetSlot, Energy: a.Energy, BagUnits: a.Bag, LastGatherHour: a.LastGatherHour})
	}
	admitted, err := FoodFlowAllocateGathers(int(at/sim.SimTime(FoodFlowHour)), f.seed, stock, claims)
	if err != nil {
		return err
	}
	for _, a := range admitted {
		f.admissions[a.Actor] = a
	}
	return nil
}

// Step closes one timestamp; a finite 168h run takes at most 700 steps.
func (f *FoodFlow) Step(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.steps >= 700 {
		return false, ErrFoodFlowRunner
	}
	snap := f.sched.Snapshot()
	if len(snap.Wakes) == 0 {
		return false, nil
	}
	at := snap.Wakes[0].At
	if at%sim.SimTime(FoodFlowHour) == 0 {
		if len(snap.Fibers) != FoodFlowActorCount+1 {
			return false, ErrFoodFlowRunner
		}
		for i := range f.pulseLifecycle {
			fiber := snap.Fibers[i]
			if fiber.Actor != sim.EntityID(i+1) || fiber.Activity != nil {
				return false, ErrFoodFlowRunner
			}
			f.pulseLifecycle[i] = fiber.Lifecycle
		}
	}
	if err := f.allocate(at); err != nil {
		return false, err
	}
	f.pending = &foodFlowPending{started: make(map[sim.EntityID]strategy.FoodFlowAction), choices: make(map[sim.EntityID]strategy.FoodFlowChoice)}
	defer func() { f.pending = nil }()
	events, processed, err := f.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	f.steps++
	ids := make(map[string]sim.EventID, len(events))
	for _, ev := range events {
		ids[ev.Key] = ev.ID
	}
	head := f.k.SnapshotHead()
	batch := FoodFlowBatch{Time: at, Version: head.Version, TipID: head.TipID, TipHash: head.TipHash}
	for _, a := range f.pending.attempts {
		if a.Choice.Kind == strategy.FoodFlowEat || (a.Choice.Kind == strategy.FoodFlowGather && a.Rejection == FoodFlowGatherAdmitted) {
			a.EventID = ids[a.Key]
			a.Accepted = a.EventID != 0
			if !a.Accepted {
				return false, ErrFoodFlowRunner
			}
		} else if ids[a.Key] != 0 {
			return false, ErrFoodFlowRunner
		}
		batch.Attempts = append(batch.Attempts, a)
	}
	sort.Slice(batch.Attempts, func(i, j int) bool {
		if batch.Attempts[i].Actor != batch.Attempts[j].Actor {
			return batch.Attempts[i].Actor < batch.Attempts[j].Actor
		}
		return batch.Attempts[i].Time < batch.Attempts[j].Time
	})
	for _, actor := range f.pending.completed {
		kind := f.current[actor]
		delete(f.current, actor)
		delete(f.choices, actor)
		for i := len(f.journal) - 1; i >= 0; i-- {
			found := false
			for j := range f.journal[i].Activities {
				activity := &f.journal[i].Activities[j]
				if activity.Actor == actor && activity.Kind == kind && activity.Completed == 0 {
					activity.Completed = at
					activity.EventID = ids[foodFlowKey(map[strategy.FoodFlowAction]string{strategy.FoodFlowGather: "gather", strategy.FoodFlowEat: "eat"}[kind], at, actor)]
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	for actor, kind := range f.pending.started {
		fiber, ok := f.sched.Fiber(actor)
		if !ok || fiber.Activity == nil {
			return false, ErrFoodFlowRunner
		}
		batch.Activities = append(batch.Activities, FoodFlowActivity{Actor: actor, Kind: kind, Started: at, Token: fiber.Activity.Token})
		f.current[actor] = kind
		if kind == strategy.FoodFlowGather {
			f.choices[actor] = f.pending.choices[actor]
		}
	}
	sort.Slice(batch.Activities, func(i, j int) bool { return batch.Activities[i].Actor < batch.Activities[j].Actor })
	f.journal = append(f.journal, batch)
	if at%sim.SimTime(FoodFlowHour) == 0 {
		hour := int(at / sim.SimTime(FoodFlowHour))
		check, err := f.checkpoint(hour)
		if err != nil {
			return false, fmt.Errorf("food-flow hour %d: %w", hour, err)
		}
		f.checkpoints = append(f.checkpoints, check)
	}
	return true, nil
}

func (f *FoodFlow) checkpoint(hour int) (FoodFlowCheckpoint, error) {
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	patches, slots, actors, err := foodFlowStates(view)
	if err != nil {
		return FoodFlowCheckpoint{}, err
	}
	balance, err := FoodFlowCheckConservation(patches, slots, actors)
	if err != nil {
		return FoodFlowCheckpoint{}, err
	}
	check := FoodFlowCheckpoint{Hour: hour, Version: head.Version, Balance: balance, Patches: patches, Slots: slots, Actors: actors}
	for _, a := range actors {
		if a.Energy > 0 {
			check.Alive++
		}
	}
	// Check every direct numerical projection against the same authoritative
	// version, entity, source field and exact integer value (including -1).
	for _, spec := range []struct {
		typ    sim.ComponentTypeID
		ids    []sim.EntityID
		fields []sim.FieldID
	}{
		{FoodFlowPatchTypeID, []sim.EntityID{1001, 1002}, []sim.FieldID{1, 2, 3, 4}},
		{FoodFlowSlotTypeID, func() []sim.EntityID {
			ids := make([]sim.EntityID, 16)
			for i := range ids {
				ids[i] = sim.EntityID(2001 + i)
			}
			return ids
		}(), []sim.FieldID{1, 3}},
		{FoodFlowBagTypeID, func() []sim.EntityID {
			ids := make([]sim.EntityID, 16)
			for i := range ids {
				ids[i] = sim.EntityID(i + 1)
			}
			return ids
		}(), []sim.FieldID{1}},
		{FoodFlowBodyTypeID, func() []sim.EntityID {
			ids := make([]sim.EntityID, 16)
			for i := range ids {
				ids[i] = sim.EntityID(i + 1)
			}
			return ids
		}(), []sim.FieldID{1, 2, 3, 4, 5, 6}},
	} {
		for _, field := range spec.fields {
			metrics, e := view.Reader.Project(component.ProjectRequest{Component: spec.typ, Projection: sim.ProjectionID(field), Entities: spec.ids, WorldVersion: view.Version, Authority: view.Authority})
			if e != nil {
				return check, e
			}
			for i, id := range spec.ids {
				v, e := foodFlowInts(view, id, spec.typ, field)
				if e != nil {
					return check, e
				}
				m, e := metrics.At(i)
				if e != nil {
					return check, e
				}
				n, e := m.Value().Scalar()
				if e != nil || n != float64(v[0]) || m.ProjectionVersion() != FoodFlowProjectionVersion || m.Provenance().Entity != id || m.Provenance().Component != spec.typ || m.Provenance().WorldVersion != view.Version || len(m.Provenance().SourceFields) != 1 || m.Provenance().SourceFields[0] != field {
					return check, ErrFoodFlowRunner
				}
			}
		}
	}
	return check, nil
}

// Handoff captures portable kernel and scheduler bytes plus runner-only state
// under the coordinator boundary. Encoding, authentication and restore belong
// to the separate persistence lane, not this runner.
func (f *FoodFlow) Handoff() (FoodFlowHandoff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return FoodFlowHandoff{}, err
	}
	portable, schedHead, err := f.sched.ExportPortable()
	if err != nil {
		return FoodFlowHandoff{}, err
	}
	if head != schedHead {
		return FoodFlowHandoff{}, ErrFoodFlowRunner
	}
	out := FoodFlowHandoff{FormatVersion: FoodFlowFormatVersion, Yield: f.yield, Seed: f.seed, Policy: f.ref, Steps: f.steps, History: history, SchedulerBytes: portable, Head: head, Current: make(map[sim.EntityID]strategy.FoodFlowAction, len(f.current)), Choices: make(map[sim.EntityID]strategy.FoodFlowChoice, len(f.choices)), Checkpoints: append([]FoodFlowCheckpoint(nil), f.checkpoints...)}
	for id, kind := range f.current {
		out.Current[id] = kind
	}
	for id, choice := range f.choices {
		out.Choices[id] = choice
	}
	out.Journal = cloneFoodFlowJournal(f.journal)
	return out, nil
}

func (f *FoodFlow) Kernel() *kernel.Kernel { return f.k }
func (f *FoodFlow) SchedulerSnapshot() scheduler.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sched.Snapshot()
}
func (f *FoodFlow) Journal() []FoodFlowBatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneFoodFlowJournal(f.journal)
}
func cloneFoodFlowJournal(journal []FoodFlowBatch) []FoodFlowBatch {
	out := make([]FoodFlowBatch, len(journal))
	for i, b := range journal {
		out[i] = b
		out[i].Attempts = append([]FoodFlowAttempt(nil), b.Attempts...)
		out[i].Activities = append([]FoodFlowActivity(nil), b.Activities...)
	}
	return out
}
func (f *FoodFlow) Checkpoints() []FoodFlowCheckpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FoodFlowCheckpoint(nil), f.checkpoints...)
}
func (f *FoodFlow) Run(ctx context.Context) error {
	for {
		processed, err := f.Step(ctx)
		if err != nil {
			return err
		}
		if !processed {
			break
		}
	}
	checks := f.Checkpoints()
	if len(checks) != FoodFlowHorizonHours+1 || checks[len(checks)-1].Hour != FoodFlowHorizonHours {
		return ErrFoodFlowRunner
	}
	return nil
}
