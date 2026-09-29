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

// Capacity v3 runner (ticket 13): the productive-capacity pilot over the
// frozen clock of .scratch/13-productive-capacity/spec.md. The contract lane
// (capacity_contract*.go) owns every constant, transition, and intention
// constructor; the policy lane (internal/strategy/capacity*.go) owns the
// frozen decision cascade. This file only places validated intentions on the
// frozen clock, applies the ownership guard to every proposal, and publishes
// value-only evidence (journal, hourly checkpoints, quiescent handoff).
//
// Frozen hour lattice for hour h (CapacityPhaseTime):
//
//	h+0        pulse (world cause, frozen order): basal -> wild production ->
//	           capital yield (living owners, min(k, free), overflow typed) ->
//	           wear accrual (all owners, dead included);
//	h+1µs      claim: the policy Gather|EatStored|Wait choice commits
//	           atomically (CapacityGatherProposal and the claim-phase
//	           CapacityEatStoredProposal are pinned to this instant);
//	h+20m+1µs  gather completion: admitted gatherers' 20-minute activity
//	           ends and they receive their bag-decision wake; denied
//	           gatherers are processed here: typed denial evidence, then the
//	           policy fallback Rest, or a deferred stored meal at h+40m+4µs;
//	h+20m+2µs  bag decision: Build|Eat (CapacityEatProposal is pinned here);
//	h+20m+3µs  build start (the 20-minute build activity begins exactly
//	           here so its completion is CapacityBuildCompletion);
//	h+40m+3µs  build completion: the atomic bag→wip/k conversion commits
//	           from a fresh snapshot (CapacityBuildProposal);
//	h+40m+4µs  paired meal: fresh-snapshot granary revalidation before any
//	           paired EatStored (build pairing or denied-gather fallback);
//	           a granary raced to empty leaves the build standing and
//	           records a typed no-meal note, mutating nothing.
//
// Gather and build are the only scheduler activities: they are the two
// frozen windows whose completions carry state (bag decision, bag→wip/k
// conversion). Meals and rests commit at their frozen instants; the wake
// lattice gives each actor at most one duty per instant, so one activity per
// actor at a time is enforced by the scheduler plus the runner guards below.
var ErrCapacityRunner = errors.New("invalid capacity runner")

// capacityClockActor drives the hourly pulse; it is disjoint from the actor
// roster 1..16 (patch/slot entities are components, not scheduler fibers).
const capacityClockActor = sim.EntityID(5001)

// capacityStepLimit is the frozen step bound (~1,400/168 h): six lattice
// instants per hour plus the final basal pulse at h=168.
const capacityStepLimit = 1400

// capacityNoStoredMealNote is the typed paired-meal evidence emitted when the
// fresh snapshot revalidation finds no drawable stored meal. It mutates
// nothing: the build has already committed and stands.
const capacityNoStoredMealNote = "no-stored-meal"

type CapacityOptions struct {
	Yield   int64 // wild q per patch-hour, [0, CapacitySlotsPerPatch]
	Seed    uint64
	Workers int
	Enabled bool
	// Founders are the recorded endowment variants of the frozen matrix
	// (founder-A: actor 1 k=2; founder-B: actor 1 granary=4). They seed the
	// neutral h0 worksite/granary ledgers consistently with G2.
	Founders []CapacityFounder
}

type CapacityFounder struct {
	Actor            sim.EntityID // 1..16
	Capital, Granary int64        // [0, CapacityKMax], [0, CapacityGranaryMax]
}

// CapacityAttempt is one typed piece of runner evidence. Keyed attempts link
// to their committed event; unkeyed attempts (denials, notes, waits) are
// rejections or evidence and mutate nothing.
type CapacityAttempt struct {
	Actor     sim.EntityID
	Time      sim.SimTime
	Kind      string // gather | eat-stored | wait | eat | build | paired-meal | fallback-eatstored | rest
	Key       string
	Target    sim.EntityID // admitted gather slot; zero otherwise
	Rejection FoodFlowGatherRejection
	NoMeal    string // typed paired-meal note; empty when the meal committed or was not attempted
	Accepted  bool
	EventID   sim.EventID
}

type CapacityBatch struct {
	Time     sim.SimTime
	Version  sim.WorldVersion
	TipID    sim.EventID
	TipHash  [32]byte
	Attempts []CapacityAttempt
}

// CapacityMealIntent is the deferred stored meal of one hour: either a
// builder's paired meal (Paired) or a denied gatherer's fallback meal. It is
// consumed at the h+40m+4µs paired instant after fresh revalidation.
type CapacityMealIntent struct {
	Hour   int
	Paired bool
}

// CapacityCheckpoint is the hourly projection: frozen per-actor and aggregate
// metrics (capital, granary, wip, wear-debt, invested-units, points
// created/decayed, yield-total/unrealized, stored/wild meals, alive, totals)
// read from the authoritative version at the pulse boundary.
type CapacityCheckpoint struct {
	Hour    int
	Version sim.WorldVersion
	Balance CapacityBalance
	Patches [CapacityPatchCount]CapacityPatchState
	Slots   [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState
	Actors  [CapacityActorCount]CapacityActorState
	Alive   int
}

// CapacityHandoff is the value-only, quiescent input to the persistence lane
// (the v2 seam): portable kernel and scheduler bytes plus the verified head
// must be persisted together; Current, Denied, and Meals preserve in-flight
// hour-scoped runner state across a restore.
type CapacityHandoff struct {
	FormatVersion  uint32
	Yield          int64
	Seed           uint64
	Enabled        bool
	Founders       []CapacityFounder
	Policy         strategy.CapacityRef
	Steps          int
	History        []byte
	SchedulerBytes []byte
	Head           kernel.PortableHead
	Current        map[sim.EntityID]strategy.CapacityAction
	Denied         map[sim.EntityID]int64
	Meals          map[sim.EntityID]CapacityMealIntent
	Journal        []CapacityBatch
	Checkpoints    []CapacityCheckpoint
}

type Capacity struct {
	mu             sync.Mutex
	k              *kernel.Kernel
	sched          *scheduler.Scheduler
	registry       component.Registry
	seeds          []component.ComponentSeed
	bound          [CapacityActorCount]*strategy.CapacityBound
	ref            strategy.CapacityRef
	seed           uint64
	yield          int64
	enabled        bool
	founders       []CapacityFounder
	current        map[sim.EntityID]strategy.CapacityAction // Gather|Build activity in flight
	denied         map[sim.EntityID]int64                   // last denied-gather hour, typed evidence
	meals          map[sim.EntityID]CapacityMealIntent      // deferred paired/fallback meals of the running hour
	admissions     map[sim.EntityID]FoodFlowGatherAdmission // claim-step allocation, set before the step
	pulseLifecycle [CapacityActorCount]scheduler.Lifecycle
	pending        *capacityPending
	journal        []CapacityBatch
	checkpoints    []CapacityCheckpoint
	steps          int
}

// capacityPending accumulates one step's evidence and post-commit runner
// state under its own lock: worker goroutines append, the coordinator applies
// after the scheduler atomically closes the timestamp.
type capacityPending struct {
	mu           sync.Mutex
	attempts     []CapacityAttempt
	started      map[sim.EntityID]strategy.CapacityAction
	completed    []sim.EntityID
	denied       []sim.EntityID
	meals        map[sim.EntityID]CapacityMealIntent
	mealsCleared []sim.EntityID
}

func capacitySeed(entity sim.EntityID, typ sim.ComponentTypeID, fields ...component.FieldSeed) component.ComponentSeed {
	return component.ComponentSeed{Entity: entity, Component: typ, Fields: fields}
}
func capacityField(field sim.FieldID, n int64) component.FieldSeed {
	return component.FieldSeed{Field: field, Value: sim.IntegerValue(n)}
}
func capacityValueRef(id sim.EntityID) sim.Value {
	if id == 0 {
		v, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		return v
	}
	v, _ := sim.EntityRefValue(id)
	return v
}
func capacityPatchValue(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, v sim.Value) component.Patch {
	return component.Patch{Entity: entity, Component: typ, SchemaVersion: CapacitySchemaVersion, Field: field, Value: v}
}
func capacityNumber(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, n int64) component.Patch {
	return capacityPatchValue(entity, typ, field, sim.IntegerValue(n))
}
func capacityKey(kind string, at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("capacity/%s/%016x/%04x", kind, uint64(at), uint64(actor))
}
func capacityProposal(kind string, at sim.SimTime, actor sim.EntityID, rule sim.RuleID, patches ...component.Patch) kernel.Proposal {
	return kernel.Proposal{Key: capacityKey(kind, at, actor), Time: at, Cause: kernel.Cause{Actor: actor, World: actor == 0}, Rule: rule, RuleVersion: CapacityRuleVersion, Patches: patches}
}

// NewCapacity builds the neutral v3 h0 world (capacityNeutralSeeds ledger
// values) with the frozen policy bound to every roster actor, plus any
// recorded founder endowments.
func NewCapacity(o CapacityOptions) (*Capacity, error) {
	if o.Workers < 1 || o.Yield < 0 || o.Yield > CapacitySlotsPerPatch {
		return nil, ErrCapacityRunner
	}
	seen := make(map[sim.EntityID]bool, len(o.Founders))
	for _, founder := range o.Founders {
		if _, err := CapacityActorPatchID(founder.Actor); err != nil || seen[founder.Actor] ||
			founder.Capital < 0 || founder.Capital > CapacityKMax ||
			founder.Granary < 0 || founder.Granary > CapacityGranaryMax {
			return nil, ErrCapacityRunner
		}
		seen[founder.Actor] = true
	}
	reg, err := CapacityRegistry()
	if err != nil {
		return nil, err
	}
	policy := strategy.FrozenCapacityPolicy()
	policies := strategy.NewCapacityRegistry()
	if err = policies.Register(policy); err != nil {
		return nil, err
	}
	f := &Capacity{registry: reg, ref: policy.Ref, seed: o.Seed, yield: o.Yield, enabled: o.Enabled, founders: append([]CapacityFounder(nil), o.Founders...),
		current: make(map[sim.EntityID]strategy.CapacityAction), denied: make(map[sim.EntityID]int64), meals: make(map[sim.EntityID]CapacityMealIntent)}
	endowment := make(map[sim.EntityID]CapacityFounder, len(o.Founders))
	for _, founder := range o.Founders {
		endowment[founder.Actor] = founder
	}
	// The neutral v3 h0 ledgers plus any recorded endowment, built once and
	// gated by the frozen conservation before any kernel exists. A capital
	// endowment cannot satisfy the per-patch flow identity (every invested
	// unit must have been a home-patch gather, and no gather exists at h0),
	// so such founder variants are rejected at construction instead of
	// failing the hourly integrity gates mid-run.
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	var patches [CapacityPatchCount]CapacityPatchState
	var slots [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState
	var actors [CapacityActorCount]CapacityActorState
	for i := range patches {
		patches[i] = CapacityPatchState{Yield: o.Yield}
	}
	for i := 0; i < CapacityActorCount; i++ {
		actor := sim.EntityID(i + 1)
		founder := endowment[actor]
		actors[i] = CapacityActorState{
			Body:     CapacityBodyState{Energy: CapacityInitialEnergy, LastGatherHour: CapacityNeverHour},
			Worksite: CapacityWorksiteState{Capital: founder.Capital, InvestedUnits: CapacityPointCostWip * founder.Capital, PointsCreated: founder.Capital, LastBuildHour: CapacityNeverHour},
			Granary:  CapacityGranaryState{Stock: founder.Granary, YieldTotal: founder.Granary, LastStoredMealHour: CapacityNeverHour},
		}
	}
	if _, err = CapacityCheckConservation(patches, slots, actors); err != nil {
		return nil, fmt.Errorf("%w: founder endowment breaks frozen conservation: %v", ErrCapacityRunner, err)
	}
	for i := range patches {
		patch, _ := CapacityPatchID(i)
		f.seeds = append(f.seeds, capacitySeed(patch, CapacityPatchTypeID, capacityField(CapacityPatchYieldField, o.Yield), capacityField(CapacityPatchPulsesField, 0), capacityField(CapacityPatchProducedField, 0), capacityField(CapacityPatchUnrealizedField, 0)))
		for j := range slots[i] {
			slot, _ := CapacitySlotID(i, j)
			f.seeds = append(f.seeds, capacitySeed(slot, CapacitySlotTypeID, capacityField(CapacitySlotStockField, slots[i][j].Stock), capacityField(CapacitySlotPatchField, 0), capacityField(CapacitySlotGatheredField, 0)))
			f.seeds[len(f.seeds)-1].Fields[1].Value = capacityValueRef(patch)
		}
	}
	for i := 0; i < CapacityActorCount; i++ {
		actor := sim.EntityID(i + 1)
		a := actors[i]
		f.seeds = append(f.seeds,
			capacitySeed(actor, CapacityBagTypeID, capacityField(CapacityBagUnitsField, a.Bag.Units), capacityField(CapacityBagSourceField, 0)))
		f.seeds[len(f.seeds)-1].Fields[1].Value = missing
		f.seeds = append(f.seeds,
			capacitySeed(actor, CapacityBodyTypeID, capacityField(CapacityBodyEnergyField, a.Body.Energy), capacityField(CapacityBodyHungerField, a.Body.Hunger), capacityField(CapacityBodyBasalSpentField, a.Body.BasalSpent), capacityField(CapacityBodyCapLostField, a.Body.CapLost), capacityField(CapacityBodyConsumedField, a.Body.Consumed), capacityField(CapacityBodyLastGatherHourField, a.Body.LastGatherHour)),
			capacitySeed(actor, CapacityWorksiteTypeID, capacityField(CapacityWorksiteCapitalField, a.Worksite.Capital), capacityField(CapacityWorksiteWipField, a.Worksite.Wip), capacityField(CapacityWorksiteWearDebtField, a.Worksite.WearDebt), capacityField(CapacityWorksiteInvestedUnitsField, a.Worksite.InvestedUnits), capacityField(CapacityWorksitePointsCreatedField, a.Worksite.PointsCreated), capacityField(CapacityWorksitePointsDecayedField, a.Worksite.PointsDecayed), capacityField(CapacityWorksiteLastBuildHourField, a.Worksite.LastBuildHour)),
			capacitySeed(actor, CapacityGranaryTypeID, capacityField(CapacityGranaryStockField, a.Granary.Stock), capacityField(CapacityGranaryYieldTotalField, a.Granary.YieldTotal), capacityField(CapacityGranaryYieldUnrealizedField, a.Granary.YieldUnrealized), capacityField(CapacityGranaryStoredMealsField, a.Granary.StoredMeals), capacityField(CapacityGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour)))
		f.bound[i], err = policies.Bind(strategy.CapacityBinding{Actor: actor, Ref: policy.Ref})
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
	for i := 1; i <= CapacityActorCount; i++ {
		if err = f.sched.Register(sim.EntityID(i)); err != nil {
			return nil, err
		}
	}
	if err = f.sched.Register(capacityClockActor); err != nil {
		return nil, err
	}
	if err = f.sched.Schedule(scheduler.Wake{Actor: capacityClockActor, At: 0, Cause: scheduler.WakeAudit}); err != nil {
		return nil, err
	}
	return f, nil
}

func capacityRead(view scheduler.SnapshotView, entity sim.EntityID, typ sim.ComponentTypeID, fields ...sim.FieldID) ([]sim.Value, error) {
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
func capacityInts(view scheduler.SnapshotView, entity sim.EntityID, typ sim.ComponentTypeID, fields ...sim.FieldID) ([]int64, error) {
	values, err := capacityRead(view, entity, typ, fields...)
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
func capacityRefValue(v sim.Value) (sim.EntityID, error) {
	if v.State() == sim.Missing && v.Kind() == sim.EntityRefKind {
		return 0, nil
	}
	return v.EntityRef()
}

// capacityStates reads the full authoritative v3 state at one version.
func capacityStates(view scheduler.SnapshotView) ([CapacityPatchCount]CapacityPatchState, [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState, [CapacityActorCount]CapacityActorState, error) {
	var patches [CapacityPatchCount]CapacityPatchState
	var slots [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState
	var actors [CapacityActorCount]CapacityActorState
	for i := range patches {
		patch, _ := CapacityPatchID(i)
		n, err := capacityInts(view, patch, CapacityPatchTypeID, CapacityPatchYieldField, CapacityPatchPulsesField, CapacityPatchProducedField, CapacityPatchUnrealizedField)
		if err != nil {
			return patches, slots, actors, err
		}
		patches[i] = CapacityPatchState{n[0], n[1], n[2], n[3]}
		for j := range slots[i] {
			slot, _ := CapacitySlotID(i, j)
			n, err := capacityInts(view, slot, CapacitySlotTypeID, CapacitySlotStockField, CapacitySlotGatheredField)
			if err != nil {
				return patches, slots, actors, err
			}
			slots[i][j] = CapacitySlotState{n[0], n[1]}
		}
	}
	for i := range actors {
		actor := sim.EntityID(i + 1)
		body, err := capacityInts(view, actor, CapacityBodyTypeID, CapacityBodyEnergyField, CapacityBodyHungerField, CapacityBodyBasalSpentField, CapacityBodyCapLostField, CapacityBodyConsumedField, CapacityBodyLastGatherHourField)
		if err != nil {
			return patches, slots, actors, err
		}
		bagValues, err := capacityRead(view, actor, CapacityBagTypeID, CapacityBagUnitsField, CapacityBagSourceField)
		if err != nil {
			return patches, slots, actors, err
		}
		units, err := bagValues[0].Integer()
		if err != nil {
			return patches, slots, actors, err
		}
		source, err := capacityRefValue(bagValues[1])
		if err != nil {
			return patches, slots, actors, err
		}
		worksite, err := capacityInts(view, actor, CapacityWorksiteTypeID, CapacityWorksiteCapitalField, CapacityWorksiteWipField, CapacityWorksiteWearDebtField, CapacityWorksiteInvestedUnitsField, CapacityWorksitePointsCreatedField, CapacityWorksitePointsDecayedField, CapacityWorksiteLastBuildHourField)
		if err != nil {
			return patches, slots, actors, err
		}
		granary, err := capacityInts(view, actor, CapacityGranaryTypeID, CapacityGranaryStockField, CapacityGranaryYieldTotalField, CapacityGranaryYieldUnrealizedField, CapacityGranaryStoredMealsField, CapacityGranaryLastStoredMealHourField)
		if err != nil {
			return patches, slots, actors, err
		}
		actors[i] = CapacityActorState{
			Body:     CapacityBodyState{Energy: body[0], Hunger: body[1], BasalSpent: body[2], CapLost: body[3], Consumed: body[4], LastGatherHour: body[5]},
			Bag:      CapacityBagState{Units: units, Source: source},
			Worksite: CapacityWorksiteState{Capital: worksite[0], Wip: worksite[1], WearDebt: worksite[2], InvestedUnits: worksite[3], PointsCreated: worksite[4], PointsDecayed: worksite[5], LastBuildHour: worksite[6]},
			Granary:  CapacityGranaryState{Stock: granary[0], YieldTotal: granary[1], YieldUnrealized: granary[2], StoredMeals: granary[3], LastStoredMealHour: granary[4]},
		}
	}
	return patches, slots, actors, nil
}

// observe builds the actor-visible v3 view: the owner's own rows, the public
// home-patch slot stocks, and the ledger guards the frozen privacy list
// allows. Others' capital, granaries, energy, meals, unrealized counters, and
// the seed have no field in the observation type at all.
func (f *Capacity) observe(view scheduler.SnapshotView, actor sim.EntityID, hour int, denial strategy.CapacityDenial) (strategy.CapacityObservation, error) {
	body, err := capacityRead(view, actor, CapacityBodyTypeID, CapacityBodyEnergyField, CapacityBodyHungerField, CapacityBodyLastGatherHourField)
	if err != nil {
		return strategy.CapacityObservation{}, err
	}
	bag, err := capacityRead(view, actor, CapacityBagTypeID, CapacityBagUnitsField, CapacityBagSourceField)
	if err != nil {
		return strategy.CapacityObservation{}, err
	}
	worksite, err := capacityRead(view, actor, CapacityWorksiteTypeID, CapacityWorksiteCapitalField, CapacityWorksiteWipField, CapacityWorksiteInvestedUnitsField, CapacityWorksiteLastBuildHourField)
	if err != nil {
		return strategy.CapacityObservation{}, err
	}
	granary, err := capacityRead(view, actor, CapacityGranaryTypeID, CapacityGranaryStockField, CapacityGranaryLastStoredMealHourField)
	if err != nil {
		return strategy.CapacityObservation{}, err
	}
	home, err := CapacityActorPatchID(actor)
	if err != nil {
		return strategy.CapacityObservation{}, ErrCapacityRunner
	}
	index := int(home - 1001)
	obs := strategy.CapacityObservation{
		Actor: actor, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		Energy: body[0], Hunger: body[1], BagUnits: bag[0], BagSource: bag[1],
		Capital: worksite[0], Wip: worksite[1], GranaryStock: granary[0],
		InvestedUnits: worksite[2], LastBuildHour: worksite[3], LastStoredMealHour: granary[1],
		LastDenial: denial,
	}
	for j := 0; j < CapacitySlotsPerPatch; j++ {
		slot, _ := CapacitySlotID(index, j)
		stock, err := capacityRead(view, slot, CapacitySlotTypeID, CapacitySlotStockField)
		if err != nil {
			return obs, err
		}
		obs.HomeSlots = append(obs.HomeSlots, strategy.CapacitySlotView{ID: slot, Stock: stock[0]})
	}
	return obs, nil
}

// guard is the runner-side ownership duty: every capacity proposal passes
// CapacityCheckOwnership before it can be planned (the kernel validates
// schema and rule admission only).
func (f *Capacity) guard(p kernel.Proposal) error {
	if p.RuleVersion != CapacityRuleVersion {
		return ErrCapacityRunner
	}
	return CapacityCheckOwnership(p)
}

func (f *Capacity) pulse(ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	at := ready.At
	h := int(at / sim.SimTime(CapacityHour))
	if h > CapacityHorizonHours || at != sim.SimTime(h)*sim.SimTime(CapacityHour) {
		return scheduler.Evaluation{}, ErrCapacityRunner
	}
	patches, slots, actors, err := capacityStates(view)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	var out scheduler.Evaluation
	// Actor quiescence at the pulse (every activity and deferred meal of the
	// previous hour closed) was verified by Step; here the lifecycle must
	// match the authoritative energy exactly.
	for i := range actors {
		if (f.pulseLifecycle[i] == scheduler.Alive) != (actors[i].Body.Energy > 0) {
			return out, ErrCapacityRunner
		}
		if f.pulseLifecycle[i] == scheduler.Stopped || h == 0 {
			continue
		}
		id := sim.EntityID(i + 1)
		next, _, err := CapacityBasal(id, actors[i], 1)
		if err != nil {
			return out, err
		}
		proposal := capacityProposal("basal", at, id, CapacityBasalRule,
			capacityNumber(id, CapacityBodyTypeID, CapacityBodyEnergyField, next.Body.Energy),
			capacityNumber(id, CapacityBodyTypeID, CapacityBodyHungerField, next.Body.Hunger),
			capacityNumber(id, CapacityBodyTypeID, CapacityBodyBasalSpentField, next.Body.BasalSpent))
		proposal.Cause = kernel.Cause{World: true} // elapsed need is a world pulse, not an actor action
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		if next.Body.Energy == 0 {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
		}
		actors[i] = next
	}
	if h < CapacityHorizonHours {
		for i, p := range patches {
			next, stock, _, err := CapacityProduceWild(h, p, slots[i])
			if err != nil {
				return out, err
			}
			patch, _ := CapacityPatchID(i)
			ps := []component.Patch{
				capacityNumber(patch, CapacityPatchTypeID, CapacityPatchPulsesField, next.Pulses),
				capacityNumber(patch, CapacityPatchTypeID, CapacityPatchProducedField, next.Produced),
				capacityNumber(patch, CapacityPatchTypeID, CapacityPatchUnrealizedField, next.Unrealized)}
			for j, s := range stock {
				if s.Stock != slots[i][j].Stock {
					slot, _ := CapacitySlotID(i, j)
					ps = append(ps, capacityNumber(slot, CapacitySlotTypeID, CapacitySlotStockField, s.Stock))
				}
			}
			proposal := capacityProposal(fmt.Sprintf("produce-%d", i), at, 0, CapacityProduceRule, ps...)
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
		}
	}
	// Frozen pulse order: capital yield for living owners only, then wear
	// accrual for every owner, dead owners included. h=168 is basal only, as
	// in v1: production, yield, and wear end at the horizon boundary.
	if h >= 1 && h < CapacityHorizonHours {
		for i := range actors {
			id := sim.EntityID(i + 1)
			if actors[i].Body.Energy > 0 && actors[i].Worksite.Capital > 0 {
				next, _, err := CapacityCapitalYield(id, actors[i])
				if err != nil {
					return out, err
				}
				ps := []component.Patch{capacityNumber(id, CapacityGranaryTypeID, CapacityGranaryYieldTotalField, next.Granary.YieldTotal)}
				if next.Granary.Stock != actors[i].Granary.Stock {
					ps = append(ps, capacityNumber(id, CapacityGranaryTypeID, CapacityGranaryStockField, next.Granary.Stock))
				}
				if next.Granary.YieldUnrealized != actors[i].Granary.YieldUnrealized {
					ps = append(ps, capacityNumber(id, CapacityGranaryTypeID, CapacityGranaryYieldUnrealizedField, next.Granary.YieldUnrealized))
				}
				proposal := capacityProposal("yield", at, id, CapacityYieldRule, ps...)
				proposal.Cause = kernel.Cause{World: true}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				actors[i] = next
			}
			if actors[i].Worksite.Capital > 0 {
				next, _, err := CapacityWear(id, actors[i])
				if err != nil {
					return out, err
				}
				ps := []component.Patch{capacityNumber(id, CapacityWorksiteTypeID, CapacityWorksiteWearDebtField, next.Worksite.WearDebt)}
				if next.Worksite.Capital != actors[i].Worksite.Capital {
					ps = append(ps, capacityNumber(id, CapacityWorksiteTypeID, CapacityWorksiteCapitalField, next.Worksite.Capital))
				}
				if next.Worksite.PointsDecayed != actors[i].Worksite.PointsDecayed {
					ps = append(ps, capacityNumber(id, CapacityWorksiteTypeID, CapacityWorksitePointsDecayedField, next.Worksite.PointsDecayed))
				}
				proposal := capacityProposal("wear", at, id, CapacityWearRule, ps...)
				proposal.Cause = kernel.Cause{World: true}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				actors[i] = next
			}
		}
	}
	if h < CapacityHorizonHours {
		claim, err := CapacityPhaseTime(h, CapacityPhaseClaim)
		if err != nil {
			return out, err
		}
		for i := range actors {
			if actors[i].Body.Energy > 0 {
				id := sim.EntityID(i + 1)
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeNeedThreshold}})
			}
		}
		next, _ := CapacityHourTime(h + 1)
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: capacityClockActor, Wake: scheduler.Wake{Actor: capacityClockActor, At: next, Cause: scheduler.WakeAudit}})
	}
	return out, nil
}

// capacityPairIntent is the frozen complement of the bag-decision Build
// condition: a build chosen through the granary fallback (energy below the
// frozen TLow with a drawable stored meal) is paired with that meal at the
// build-completion instant. Builds from energy >= TLow skip meals.
func capacityPairIntent(a CapacityActorState, hour int) bool {
	return a.Body.Energy < CapacityPolicyTLow && a.Granary.Stock >= 1 && a.Granary.LastStoredMealHour < int64(hour)
}

// capacityPairedMealGate revalidates the fresh snapshot before any paired
// EatStored at h+40m+4µs: the owner must be alive, the granary must still
// hold one unit, and no stored meal may have been drawn earlier this hour.
func capacityPairedMealGate(a CapacityActorState, hour int) bool {
	return a.Body.Energy > 0 && a.Granary.Stock >= 1 && a.Granary.LastStoredMealHour < int64(hour)
}

// pairedMealDecision consumes one deferred meal intent against a fresh
// snapshot. A raced granary yields the typed no-meal note with zero
// mutation; the pairing build itself is already standing.
func (f *Capacity) pairedMealDecision(actor sim.EntityID, hour int, intent CapacityMealIntent, a CapacityActorState) (kernel.Proposal, CapacityAttempt, error) {
	kind := "fallback-eatstored"
	if intent.Paired {
		kind = "paired-meal"
	}
	paired, err := CapacityPhaseTime(hour, CapacityPhasePairedMeal)
	if err != nil {
		return kernel.Proposal{}, CapacityAttempt{}, err
	}
	attempt := CapacityAttempt{Actor: actor, Time: paired, Kind: kind}
	if !capacityPairedMealGate(a, hour) {
		attempt.NoMeal = capacityNoStoredMealNote
		return kernel.Proposal{}, attempt, nil
	}
	proposal, err := CapacityEatStoredProposal(capacityKey(kind, paired, actor), hour, paired, actor, a)
	if err != nil {
		return kernel.Proposal{}, CapacityAttempt{}, err
	}
	if err = f.guard(proposal); err != nil {
		return kernel.Proposal{}, CapacityAttempt{}, err
	}
	attempt.Key = proposal.Key
	return proposal, attempt, nil
}

func (f *Capacity) evaluate(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	if ready.Fiber.Actor == capacityClockActor {
		return f.pulse(ready, view)
	}
	id := ready.Fiber.Actor
	if id < 1 || id > CapacityActorCount || len(ready.Causes) != 1 {
		return scheduler.Evaluation{}, ErrCapacityRunner
	}
	h := int(ready.At / sim.SimTime(CapacityHour))
	phase := func(n int) (sim.SimTime, error) { return CapacityPhaseTime(h, n) }
	claim, err := phase(CapacityPhaseClaim)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	gathered, _ := phase(CapacityPhaseGathered)
	bagDecision, _ := phase(CapacityPhaseBagDecision)
	buildStart, _ := phase(CapacityPhaseBuildStart)
	buildCompletion := CapacityBuildCompletion(buildStart)
	paired, _ := phase(CapacityPhasePairedMeal)
	at := ready.At
	var out scheduler.Evaluation
	record := func(a CapacityAttempt) {
		f.pending.mu.Lock()
		f.pending.attempts = append(f.pending.attempts, a)
		f.pending.mu.Unlock()
	}
	switch {
	case ready.Causes[0] == scheduler.WakeNeedThreshold && at == claim:
		// Claim: policy Gather|EatStored|Wait; the gather and the claim-phase
		// stored meal commit atomically at this instant.
		if f.current[id] != 0 {
			return out, ErrCapacityRunner
		}
		actors, err := capacityActorRows(view)
		if err != nil {
			return out, err
		}
		a := actors[id-1]
		if a.Bag.Units != 0 {
			return out, ErrCapacityRunner
		}
		obs, err := f.observe(view, id, h, strategy.CapacityNoDenial)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[id-1].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrCapacityRunner
		}
		switch choice.Kind {
		case strategy.CapacityGather:
			admission, ok := f.admissions[id]
			if !ok || admission.Actor != id {
				return out, ErrCapacityRunner
			}
			attempt := CapacityAttempt{Actor: id, Time: at, Kind: "gather", Target: admission.Slot, Rejection: admission.Rejection}
			if admission.Rejection == FoodFlowGatherAdmitted {
				slots, err := capacitySlotRows(view)
				if err != nil {
					return out, err
				}
				home, err := CapacityActorPatchID(id)
				if err != nil || admission.Patch != home {
					return out, ErrCapacityRunner
				}
				index := int(admission.Slot - 2001)
				if index < 0 || index >= CapacityPatchCount*CapacitySlotsPerPatch {
					return out, ErrCapacityRunner
				}
				slotBefore := slots[index/CapacitySlotsPerPatch][index%CapacitySlotsPerPatch]
				proposal, err := CapacityGatherProposal(capacityKey("gather", at, id), h, id, admission.Slot, slotBefore, a)
				if err != nil {
					return out, err
				}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				// The gather activity is guarded by its own commit: no
				// phantom window without the atomic slot→bag transfer.
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: CapacityGatherDuration, IfKey: proposal.Key})
				f.pending.mu.Lock()
				f.pending.started[id] = strategy.CapacityGather
				f.pending.mu.Unlock()
			} else {
				f.pending.mu.Lock()
				f.pending.denied = append(f.pending.denied, id)
				f.pending.mu.Unlock()
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: gathered, Cause: scheduler.WakeAudit}})
			}
			record(attempt)
		case strategy.CapacityEatStored:
			proposal, err := CapacityEatStoredProposal(capacityKey("eat-stored", at, id), h, at, id, a)
			if err != nil {
				return out, err
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(CapacityAttempt{Actor: id, Time: at, Kind: "eat-stored", Key: proposal.Key})
		case strategy.CapacityWait:
			record(CapacityAttempt{Actor: id, Time: at, Kind: "wait"})
		default:
			return out, ErrCapacityRunner
		}
	case ready.Causes[0] == scheduler.WakeCompletion && at == gathered:
		// Admitted gather completion: the atomic gather committed at claim;
		// the actor now receives its frozen bag-decision wake.
		if f.current[id] != strategy.CapacityGather {
			return out, ErrCapacityRunner
		}
		f.pending.mu.Lock()
		f.pending.completed = append(f.pending.completed, id)
		f.pending.mu.Unlock()
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: bagDecision, Cause: scheduler.WakeAudit}})
	case ready.Causes[0] == scheduler.WakeAudit && at == gathered:
		// Denied-gather completion: typed denial evidence was recorded at the
		// claim; here the policy fallback fires: Rest, or a deferred stored
		// meal drawn fresh at the paired instant.
		if f.denied[id] != int64(h) || f.current[id] != 0 {
			return out, ErrCapacityRunner
		}
		obs, err := f.observe(view, id, h, strategy.CapacityGatherDenied)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[id-1].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrCapacityRunner
		}
		switch choice.Kind {
		case strategy.CapacityRest:
			if choice.RestDuration != strategy.CapacityRestDuration {
				return out, ErrCapacityRunner
			}
			record(CapacityAttempt{Actor: id, Time: at, Kind: "rest"})
		case strategy.CapacityEatStored:
			f.pending.mu.Lock()
			f.pending.meals[id] = CapacityMealIntent{Hour: h, Paired: false}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
		default:
			return out, ErrCapacityRunner
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == bagDecision:
		// Bag decision: Build | Eat per the frozen policy on the held unit.
		if f.current[id] != 0 {
			return out, ErrCapacityRunner
		}
		actors, err := capacityActorRows(view)
		if err != nil {
			return out, err
		}
		a := actors[id-1]
		if a.Bag.Units != 1 {
			return out, ErrCapacityRunner
		}
		obs, err := f.observe(view, id, h, strategy.CapacityNoDenial)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[id-1].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrCapacityRunner
		}
		build := choice.Kind == strategy.CapacityBuild && f.enabled // disabled never emits capital events
		switch {
		case build:
			f.pending.mu.Lock()
			if capacityPairIntent(a, h) {
				f.pending.meals[id] = CapacityMealIntent{Hour: h, Paired: true}
			}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: buildStart, Cause: scheduler.WakeAudit}})
		case choice.Kind == strategy.CapacityEat || (choice.Kind == strategy.CapacityBuild && !f.enabled):
			proposal, err := CapacityEatProposal(capacityKey("eat", at, id), h, id, a)
			if err != nil {
				return out, err
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(CapacityAttempt{Actor: id, Time: at, Kind: "eat", Key: proposal.Key})
		default:
			return out, ErrCapacityRunner
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == buildStart:
		// Build start: the 20-minute window opens exactly at the frozen
		// instant so its completion is CapacityBuildCompletion.
		actors, err := capacityActorRows(view)
		if err != nil {
			return out, err
		}
		if actors[id-1].Bag.Units != 1 || f.current[id] != 0 {
			return out, ErrCapacityRunner
		}
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: CapacityBuildDuration})
		f.pending.mu.Lock()
		f.pending.started[id] = strategy.CapacityBuild
		f.pending.mu.Unlock()
	case ready.Causes[0] == scheduler.WakeCompletion && at == buildCompletion:
		// Build completion: the atomic bag→wip/k conversion from a FRESH
		// snapshot. Nothing else writes the builder's bag, so failure here is
		// a linkage break (stop condition), not typed evidence.
		if f.current[id] != strategy.CapacityBuild {
			return out, ErrCapacityRunner
		}
		f.pending.mu.Lock()
		f.pending.completed = append(f.pending.completed, id)
		f.pending.mu.Unlock()
		actors, err := capacityActorRows(view)
		if err != nil {
			return out, err
		}
		proposal, err := CapacityBuildProposal(capacityKey("build", at, id), h, id, actors[id-1])
		if err != nil {
			return out, err
		}
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		record(CapacityAttempt{Actor: id, Time: at, Kind: "build", Key: proposal.Key})
		if intent, ok := f.meals[id]; ok && intent.Hour == h && intent.Paired {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == paired:
		// Paired instant: fresh-snapshot granary revalidation before any
		// paired EatStored (builder pairing or denied-gather fallback).
		intent, ok := f.meals[id]
		if !ok || intent.Hour != h || f.current[id] != 0 {
			return out, ErrCapacityRunner
		}
		f.pending.mu.Lock()
		f.pending.mealsCleared = append(f.pending.mealsCleared, id)
		f.pending.mu.Unlock()
		actors, err := capacityActorRows(view)
		if err != nil {
			return out, err
		}
		proposal, attempt, err := f.pairedMealDecision(id, h, intent, actors[id-1])
		if err != nil {
			return out, err
		}
		if proposal.Key != "" {
			out.Proposals = append(out.Proposals, proposal)
		}
		record(attempt)
	default:
		return out, ErrCapacityRunner
	}
	return out, nil
}

// capacitySlotRows reads the shared wild slot stocks at one version.
func capacitySlotRows(view scheduler.SnapshotView) ([CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState, error) {
	_, slots, _, err := capacityStates(view)
	return slots, err
}

// capacityActorRows reads the sixteen actor rows (body, bag, worksite,
// granary) at one version.
func capacityActorRows(view scheduler.SnapshotView) ([CapacityActorCount]CapacityActorState, error) {
	_, _, actors, err := capacityStates(view)
	return actors, err
}

// allocate precomputes the claim-step gather admissions with the unchanged v1
// allocator. The claim commits the atomic gather at h+1µs, so contention must
// be resolved before the claim step: the coordinator evaluates each waking
// actor's frozen policy choice on the same authoritative head the workers
// will see (pure, deterministic), then admits through FoodFlowAllocateGathers.
func (f *Capacity) allocate(at sim.SimTime) error {
	f.admissions = make(map[sim.EntityID]FoodFlowGatherAdmission)
	h := int(at / sim.SimTime(CapacityHour))
	claim, err := CapacityPhaseTime(h, CapacityPhaseClaim)
	if err != nil || at != claim {
		return nil
	}
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	_, slots, actors, err := capacityStates(view)
	if err != nil {
		return err
	}
	var stocked []FoodFlowStockedSlot
	for i := range slots {
		patch, _ := CapacityPatchID(i)
		for j := range slots[i] {
			if slots[i][j].Stock == 1 {
				slot, _ := CapacitySlotID(i, j)
				stocked = append(stocked, FoodFlowStockedSlot{ID: slot, Patch: patch})
			}
		}
	}
	var claims []FoodFlowGatherClaim
	for _, wake := range f.sched.Snapshot().Wakes {
		if wake.At != at || wake.Cause != scheduler.WakeNeedThreshold {
			continue
		}
		id := wake.Actor
		if id < 1 || id > CapacityActorCount {
			return ErrCapacityRunner
		}
		a := actors[id-1]
		if a.Body.Energy == 0 || a.Bag.Units != 0 {
			return ErrCapacityRunner
		}
		obs, err := f.observe(view, id, h, strategy.CapacityNoDenial)
		if err != nil {
			return err
		}
		choice, err := f.bound[id-1].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return ErrCapacityRunner
		}
		switch choice.Kind {
		case strategy.CapacityGather:
			home, err := CapacityActorPatchID(id)
			if err != nil || choice.TargetSlot == 0 {
				return ErrCapacityRunner
			}
			claims = append(claims, FoodFlowGatherClaim{Actor: id, TargetPatch: home, TargetSlot: choice.TargetSlot, Energy: a.Body.Energy, BagUnits: a.Bag.Units, LastGatherHour: a.Body.LastGatherHour})
		case strategy.CapacityEatStored, strategy.CapacityWait:
			// No allocation needed; the worker re-evaluates and commits.
		default:
			return ErrCapacityRunner
		}
	}
	if len(claims) == 0 {
		return nil
	}
	admitted, err := FoodFlowAllocateGathers(h, f.seed, stocked, claims)
	if err != nil {
		return err
	}
	for _, a := range admitted {
		f.admissions[a.Actor] = a
	}
	return nil
}

// Step closes one timestamp; the frozen 168h horizon takes at most
// capacityStepLimit steps. The commit head is guarded by the scheduler's
// stale-world check and the kernel's CommitBatchAtHead.
func (f *Capacity) Step(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.steps >= capacityStepLimit {
		return false, ErrCapacityRunner
	}
	snap := f.sched.Snapshot()
	if len(snap.Wakes) == 0 {
		return false, nil
	}
	at := snap.Wakes[0].At
	if at%sim.SimTime(CapacityHour) == 0 {
		if len(snap.Fibers) != CapacityActorCount+1 || len(f.meals) != 0 {
			return false, ErrCapacityRunner
		}
		for i := range f.pulseLifecycle {
			fiber := snap.Fibers[i]
			if fiber.Actor != sim.EntityID(i+1) || fiber.Activity != nil {
				return false, ErrCapacityRunner
			}
			f.pulseLifecycle[i] = fiber.Lifecycle
		}
	}
	if err := f.allocate(at); err != nil {
		return false, err
	}
	f.pending = &capacityPending{started: make(map[sim.EntityID]strategy.CapacityAction), meals: make(map[sim.EntityID]CapacityMealIntent)}
	defer func() { f.pending = nil }()
	events, processed, err := f.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	ids := make(map[string]sim.EventID, len(events))
	for _, ev := range events {
		ids[ev.Key] = ev.ID
	}
	head := f.k.SnapshotHead()
	batch := CapacityBatch{Time: at, Version: head.Version, TipID: head.TipID, TipHash: head.TipHash}
	for _, a := range f.pending.attempts {
		if a.Key != "" {
			a.EventID = ids[a.Key]
			if a.EventID == 0 {
				return false, ErrCapacityRunner
			}
			a.Accepted = true
		}
		batch.Attempts = append(batch.Attempts, a)
	}
	sort.Slice(batch.Attempts, func(i, j int) bool {
		a, b := batch.Attempts[i], batch.Attempts[j]
		if a.Actor != b.Actor {
			return a.Actor < b.Actor
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Time < b.Time
	})
	for _, id := range f.pending.completed {
		if _, ok := f.current[id]; !ok {
			return false, ErrCapacityRunner
		}
		delete(f.current, id)
	}
	for id, kind := range f.pending.started {
		fiber, ok := f.sched.Fiber(id)
		if !ok || fiber.Activity == nil {
			return false, ErrCapacityRunner
		}
		f.current[id] = kind
	}
	for _, id := range f.pending.denied {
		f.denied[id] = int64(at / sim.SimTime(CapacityHour))
	}
	for id, intent := range f.pending.meals {
		f.meals[id] = intent
	}
	for _, id := range f.pending.mealsCleared {
		delete(f.meals, id)
	}
	f.steps++
	f.journal = append(f.journal, batch)
	if at%sim.SimTime(CapacityHour) == 0 {
		hour := int(at / sim.SimTime(CapacityHour))
		check, err := f.checkpoint(hour)
		if err != nil {
			return false, fmt.Errorf("capacity hour %d: %w", hour, err)
		}
		f.checkpoints = append(f.checkpoints, check)
	}
	return true, nil
}

func (f *Capacity) checkpoint(h int) (CapacityCheckpoint, error) {
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	patches, slots, actors, err := capacityStates(view)
	if err != nil {
		return CapacityCheckpoint{}, err
	}
	// Gates G1-G3 at every hour boundary; every actor row is revalidated.
	balance, err := CapacityCheckConservation(patches, slots, actors)
	if err != nil {
		return CapacityCheckpoint{}, err
	}
	check := CapacityCheckpoint{Hour: h, Version: head.Version, Balance: balance, Patches: patches, Slots: slots, Actors: actors}
	for _, actor := range actors {
		if actor.Body.Energy > 0 {
			check.Alive++
		}
	}
	return check, nil
}

func (f *Capacity) Run(ctx context.Context) error {
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
	if len(checks) != CapacityHorizonHours+1 || checks[len(checks)-1].Hour != CapacityHorizonHours {
		return ErrCapacityRunner
	}
	return nil
}

func (f *Capacity) SchedulerSnapshot() scheduler.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sched.Snapshot()
}

func (f *Capacity) Journal() []CapacityBatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneCapacityJournal(f.journal)
}

func cloneCapacityJournal(batches []CapacityBatch) []CapacityBatch {
	out := make([]CapacityBatch, len(batches))
	for i, b := range batches {
		out[i] = b
		out[i].Attempts = append([]CapacityAttempt(nil), b.Attempts...)
	}
	return out
}

func (f *Capacity) Checkpoints() []CapacityCheckpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]CapacityCheckpoint(nil), f.checkpoints...)
}

// Handoff captures the portable kernel and scheduler bytes plus runner-only
// state under the coordinator boundary. Encoding, authentication, and restore
// belong to the persistence lane, not this runner.
func (f *Capacity) Handoff() (CapacityHandoff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return CapacityHandoff{}, err
	}
	portable, schedHead, err := f.sched.ExportPortable()
	if err != nil {
		return CapacityHandoff{}, err
	}
	if head != schedHead {
		return CapacityHandoff{}, ErrCapacityRunner
	}
	out := CapacityHandoff{FormatVersion: CapacityFormatVersion, Yield: f.yield, Seed: f.seed, Enabled: f.enabled,
		Founders: append([]CapacityFounder(nil), f.founders...), Policy: f.ref, Steps: f.steps,
		History: history, SchedulerBytes: portable, Head: head,
		Current: make(map[sim.EntityID]strategy.CapacityAction, len(f.current)),
		Denied:  make(map[sim.EntityID]int64, len(f.denied)),
		Meals:   make(map[sim.EntityID]CapacityMealIntent, len(f.meals)),
		Journal: cloneCapacityJournal(f.journal), Checkpoints: append([]CapacityCheckpoint(nil), f.checkpoints...)}
	for id, v := range f.current {
		out.Current[id] = v
	}
	for id, v := range f.denied {
		out.Denied[id] = v
	}
	for id, v := range f.meals {
		out.Meals[id] = v
	}
	return out, nil
}
