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

// Reproduction v4 runner (ticket 14): the birth-and-genetics pilot over the
// frozen v3 world, per the frozen design section of
// .scratch/14-reproduction-genetics/spec.md. The contract lane
// (reproduction_contract*.go) owns every constant, transition, and intention
// constructor; the policy lane (internal/strategy/reproduction*.go) owns the
// frozen decision cascades. This file owns the dynamic roster (maps keyed by
// entity ID, not fixed arrays), the two new birth phases, age-death and
// request expiry at the pulse, the runner-owned heredity streams, and the
// value-only evidence seams (journal, hourly checkpoints, quiescent handoff).
//
// Frozen hour lattice for hour h (ReproductionPhaseTime): the v3 phases 0-5
// are untouched —
//
//	h+0          pulse (world cause, frozen order): age-death (413, world
//	             caused, EffectStop, energy retained) -> basal (404, may set
//	             died-hour on starvation) -> wild production (401) -> capital
//	             yield (406, living owners only) -> wear (407, dead owners
//	             included) -> pending-request expiry (412, world caused);
//	h+1µs        claim: Gather|EatStored|Wait (a newborn's first claim
//	             arrives as the reserved WakeBirth cause and is handled
//	             exactly as a claim NeedThreshold wake, distinguished only in
//	             evidence);
//	h+20m+1µs    gather completion / denied-gather fallback;
//	h+20m+2µs    bag decision: Build|Eat;
//	h+20m+3µs    build start (20-minute window opens);
//	h+40m+3µs    build completion: atomic bag→wip/k conversion, fresh
//	             snapshot;
//	h+40m+4µs    paired meal: fresh-snapshot granary revalidation;
//	h+40m+5µs    birth request (409): the proposer files an addressed
//	             request with self-reported claims (policy
//	             EvaluateBirthRequest); the reply wake is scheduled for a
//	             living addressee only — a dead addressee lets the request
//	             expire at the next pulse;
//	h+40m+6µs    birth reply (410/411): the consenter decides from its own
//	             private state, the public kin distance, and the claims; the
//	             runner then revalidates the full surplus gate on fresh truth
//	             through ReproductionBirthReply. Acceptance is one atomic
//	             entity-create proposal (parent cost patches + six complete
//	             newborn rows, as built by ReproductionBirthReplyProposal);
//	             refusal is the typed two-row rule 411; death between phases
//	             is typed zero-mutation evidence. Unresolved pending requests
//	             expire at the next pulse.
//
// The runner registers a newborn fiber at the quiescent boundary after the
// birth commits, binds the frozen policy, and schedules its first wake at
// the next claim with the reserved WakeBirth cause. All evidence below is
// value-only: the persistence lane owns encoding, authentication, restore.
var ErrReproductionRunner = errors.New("invalid reproduction runner")

// reproductionClockActor drives the hourly pulse; it is disjoint from the
// actor roster 1..16 / 10001..10016 and from the patch/slot entities.
const reproductionClockActor = sim.EntityID(5002)

// reproductionStepLimit is the frozen step bound: nine lattice instants per
// hour (v3's seven plus the two birth phases) across 168 hours plus the
// final basal pulse at h=168. Empty instants close no step, so the bound has
// headroom over the 1,513-step worst case.
const reproductionStepLimit = 1700

// reproductionLostCollision is the typed evidence note on a keyed birth
// proposal that lost its kernel batch to a newborn-ID collision: the whole
// colliding birth (cost patches and allocations) stages nothing structurally.
const reproductionLostCollision = "allocation-collision"

type ReproductionOptions struct {
	Yield   int64 // wild q per patch-hour, [0, ReproductionSlotsPerPatch]
	Seed    uint64
	Workers int
	Births  bool // the frozen branch intervention: births enabled vs disabled
}

// ReproductionMealIntent is the deferred stored meal of one hour, verbatim
// v3: a builder's paired meal or a denied gatherer's fallback meal, consumed
// at h+40m+4µs after fresh revalidation.
type ReproductionMealIntent struct {
	Hour   int
	Paired bool
}

// ReproductionBirthEvidence is privileged journal evidence for one decided
// birth reply. The reported/true claim values and the divergence flag live
// only here and in the aggregate metrics — never in any actor-visible
// observation (the policy view types have no fields for them at all).
type ReproductionBirthEvidence struct {
	Decision        ReproductionReplyDecision
	Reason          ReproductionRefusalReason
	ReportedCapital int64 // requester's self-reported k claim
	ReportedGranary int64 // requester's self-reported granary claim
	TrueCapital     int64 // requester's fresh-snapshot truth (privileged)
	TrueGranary     int64
	Diverged        bool // claims != truth at the reply's fresh snapshot
	Newborn         sim.EntityID
}

// ReproductionAttempt is one typed piece of runner evidence. Keyed attempts
// link to their committed event; the one legitimate keyless-with-key case is
// a birth that lost an allocation collision (Lost set, zero mutation).
// Unkeyed attempts (denials, notes, waits, silent refusals) mutate nothing.
type ReproductionAttempt struct {
	Actor     sim.EntityID
	Time      sim.SimTime
	Kind      string // gather | eat-stored | wait | eat | build | paired-meal | fallback-eatstored | rest | birth-request | birth-reply
	Key       string
	Target    sim.EntityID // admitted gather slot; zero otherwise
	Rejection FoodFlowGatherRejection
	NoMeal    string // typed paired-meal note; empty when the meal committed or was not attempted
	Lost      string // typed lost-birth note; empty when the proposal committed or was never made
	BirthWake bool   // claim dispatched by the reserved WakeBirth cause
	Birth     ReproductionBirthEvidence
	Accepted  bool
	EventID   sim.EventID
}

type ReproductionBatch struct {
	Time     sim.SimTime
	Version  sim.WorldVersion
	TipID    sim.EventID
	TipHash  [32]byte
	Attempts []ReproductionAttempt
}

// ReproductionMetrics are the frozen v4 pilot metrics carried by every
// hourly checkpoint (spec: alive-by-hour with cause-attributed deaths,
// genome diversity, kinship density, birth economics). Death and refusal
// counters are cumulative; diversity and kinship are over the living roster.
// Mean pairwise L1 stays integer-exact as SumPairwiseL1/GenomePairs ∈ [0,6].
type ReproductionMetrics struct {
	Alive            int
	DeathsAge        int64 // cumulative rule-413 deaths
	DeathsStarvation int64 // cumulative rule-404 starvation deaths
	Births           int64
	BirthRequests    int64
	Refusals         map[ReproductionRefusalReason]int64
	Divergences      int64
	DistinctGenomes  int
	LocusFrequencies [3][3]int // [locus m|y|w][allele 5|6|7] over the living
	SumPairwiseL1    int64
	GenomePairs      int64
	KinPairs         int64 // living pairs related within the ≤10-ref walk
	LivingPairs      int64
}

// ReproductionCheckpoint is the hourly projection at the pulse boundary:
// the complete authoritative v4 state, the extended conservation balance,
// and the frozen v4 metrics.
type ReproductionCheckpoint struct {
	Hour     int
	Version  sim.WorldVersion
	Balance  ReproductionBalance
	Patches  [ReproductionPatchCount]ReproductionPatchState
	Slots    [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState
	Founders [ReproductionFounderCount]ReproductionActorState
	Newborns []ReproductionNewbornState
	Metrics  ReproductionMetrics
}

// ReproductionHandoff is the value-only, quiescent input to the persistence
// lane (v2/v3 seam): portable kernel and scheduler bytes plus the verified
// head must be persisted together. Current, Denied, Meals, Newborns, and
// BirthWakes preserve in-flight and roster runner state across a restore;
// the persistence lane re-binds every roster entity (founders and newborns)
// to the carried frozen policy ref.
type ReproductionHandoff struct {
	FormatVersion  uint32
	Yield          int64
	Seed           uint64
	Births         bool
	Policy         strategy.ReproductionRef
	Steps          int
	History        []byte
	SchedulerBytes []byte
	Head           kernel.PortableHead
	Newborns       []sim.EntityID
	BirthWakes     map[sim.EntityID]int64
	Current        map[sim.EntityID]strategy.ReproductionAction
	Denied         map[sim.EntityID]int64
	Meals          map[sim.EntityID]ReproductionMealIntent
	Journal        []ReproductionBatch
	Checkpoints    []ReproductionCheckpoint
}

type Reproduction struct {
	mu             sync.Mutex
	k              *kernel.Kernel
	sched          *scheduler.Scheduler
	registry       component.Registry
	seeds          []component.ComponentSeed
	policies       *strategy.ReproductionRegistry
	ref            strategy.ReproductionRef
	bound          map[sim.EntityID]*strategy.ReproductionBound
	seed           uint64
	yield          int64
	births         bool
	roster         []sim.EntityID // founders 1..16 then newborns ascending
	newborns       []sim.EntityID // registration (= birth) order
	pulseLifecycle map[sim.EntityID]scheduler.Lifecycle
	current        map[sim.EntityID]strategy.ReproductionAction // Gather|Build activity in flight
	denied         map[sim.EntityID]int64                       // last denied-gather hour, typed evidence
	meals          map[sim.EntityID]ReproductionMealIntent      // deferred paired/fallback meals of the running hour
	birthWakes     map[sim.EntityID]int64                       // newborn -> hour of its reserved WakeBirth first claim
	admissions     map[sim.EntityID]FoodFlowGatherAdmission     // claim-step allocation, set before the step
	pending        *reproductionPending
	journal        []ReproductionBatch
	checkpoints    []ReproductionCheckpoint
	steps          int
	deathsAge      int64
	deathsStarve   int64
	birthFiles     int64
	divergences    int64
	refusals       map[ReproductionRefusalReason]int64
}

type reproductionDeath struct {
	Actor sim.EntityID
	Cause string // "starvation" (404) | "age" (413)
}

// reproductionPending accumulates one step's evidence and post-commit runner
// state under its own lock: worker goroutines append, the coordinator applies
// after the scheduler atomically closes the timestamp.
type reproductionPending struct {
	mu              sync.Mutex
	attempts        []ReproductionAttempt
	started         map[sim.EntityID]strategy.ReproductionAction
	completed       []sim.EntityID
	denied          []sim.EntityID
	meals           map[sim.EntityID]ReproductionMealIntent
	mealsCleared    []sim.EntityID
	birthWakesUsed  []sim.EntityID
	deaths          []reproductionDeath
	replyReasons    []ReproductionRefusalReason
	divergedReplies int
	birthFiles      int
}

func reproductionSeed(entity sim.EntityID, typ sim.ComponentTypeID, fields ...component.FieldSeed) component.ComponentSeed {
	return component.ComponentSeed{Entity: entity, Component: typ, Fields: fields}
}
func reproductionField(field sim.FieldID, n int64) component.FieldSeed {
	return component.FieldSeed{Field: field, Value: sim.IntegerValue(n)}
}
func reproductionKey(kind string, at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("reproduction/%s/%016x/%04x", kind, uint64(at), uint64(actor))
}
func reproductionProposal(kind string, at sim.SimTime, actor sim.EntityID, rule sim.RuleID, patches ...component.Patch) kernel.Proposal {
	return kernel.Proposal{Key: reproductionKey(kind, at, actor), Time: at, Cause: kernel.Cause{Actor: actor, World: actor == 0}, Rule: rule, RuleVersion: ReproductionRuleVersion, Patches: patches}
}

// NewReproduction builds the neutral v4 h0 world: two patches with wild
// yield q, sixteen all-neutral founders whose immutable lifetimes are the
// runner-owned stream 0x464F554E positions 0-15 (96 + draw mod 48), and the
// frozen policy bound to every roster entity. The h0 world is gated by the
// frozen extended conservation before any kernel exists.
func NewReproduction(o ReproductionOptions) (*Reproduction, error) {
	if o.Workers < 1 || o.Yield < 0 || o.Yield > ReproductionSlotsPerPatch {
		return nil, ErrReproductionRunner
	}
	reg, err := ReproductionRegistry()
	if err != nil {
		return nil, err
	}
	policy := strategy.FrozenReproductionPolicy()
	policies := strategy.NewReproductionRegistry()
	if err = policies.Register(policy); err != nil {
		return nil, err
	}
	f := &Reproduction{registry: reg, policies: policies, ref: policy.Ref, seed: o.Seed, yield: o.Yield, births: o.Births,
		bound:          make(map[sim.EntityID]*strategy.ReproductionBound),
		pulseLifecycle: make(map[sim.EntityID]scheduler.Lifecycle),
		current:        make(map[sim.EntityID]strategy.ReproductionAction),
		denied:         make(map[sim.EntityID]int64),
		meals:          make(map[sim.EntityID]ReproductionMealIntent),
		birthWakes:     make(map[sim.EntityID]int64),
		refusals:       make(map[ReproductionRefusalReason]int64)}
	// The neutral h0 ledgers, built once and gated by the frozen extended
	// conservation before any kernel exists. Founder lifetimes are the
	// runner-owned founder-stream draws; the genome rows are immutable
	// post-creation, so the draws leave no stream state to checkpoint
	// (positions derive from committed state).
	var patches [ReproductionPatchCount]ReproductionPatchState
	var slots [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState
	var actors [ReproductionFounderCount]ReproductionActorState
	lifetimes := sim.NewRandomStream(sim.RandomState{Seed: o.Seed, Stream: ReproductionFounderStream})
	for i := range actors {
		actor := sim.EntityID(i + 1)
		lifetime := ReproductionLifetimeBase + int64(lifetimes.Uint64()%ReproductionLifetimeSpan)
		actors[i], err = ReproductionFounderState(actor, lifetime)
		if err != nil {
			return nil, err
		}
	}
	for i := range patches {
		patches[i] = ReproductionPatchState{Yield: o.Yield}
	}
	if _, err = ReproductionCheckConservation(patches, slots, actors, nil); err != nil {
		return nil, fmt.Errorf("%w: neutral h0 world breaks frozen conservation: %v", ErrReproductionRunner, err)
	}
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	for i := range patches {
		patch, _ := ReproductionPatchID(i)
		f.seeds = append(f.seeds, reproductionSeed(patch, ReproductionPatchTypeID, reproductionField(ReproductionPatchYieldField, o.Yield), reproductionField(ReproductionPatchPulsesField, 0), reproductionField(ReproductionPatchProducedField, 0), reproductionField(ReproductionPatchUnrealizedField, 0)))
		for j := range slots[i] {
			slot, _ := ReproductionSlotID(i, j)
			f.seeds = append(f.seeds, reproductionSeed(slot, ReproductionSlotTypeID, reproductionField(ReproductionSlotStockField, 0), reproductionField(ReproductionSlotPatchField, 0), reproductionField(ReproductionSlotGatheredField, 0)))
			f.seeds[len(f.seeds)-1].Fields[1].Value = missing
		}
	}
	for i := range actors {
		actor := sim.EntityID(i + 1)
		a := actors[i]
		bagSource, _ := sim.EntityRefValue(a.Bag.Source)
		addressee, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		parentA, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		parentB, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		f.seeds = append(f.seeds,
			reproductionSeed(actor, ReproductionBagTypeID, reproductionField(ReproductionBagUnitsField, a.Bag.Units), component.FieldSeed{Field: ReproductionBagSourceField, Value: bagSource}),
			reproductionSeed(actor, ReproductionBodyTypeID,
				reproductionField(ReproductionBodyEnergyField, a.Body.Energy), reproductionField(ReproductionBodyHungerField, a.Body.Hunger),
				reproductionField(ReproductionBodyBasalSpentField, a.Body.BasalSpent), reproductionField(ReproductionBodyCapLostField, a.Body.CapLost),
				reproductionField(ReproductionBodyConsumedField, a.Body.Consumed), reproductionField(ReproductionBodyLastGatherHourField, a.Body.LastGatherHour),
				reproductionField(ReproductionBodyBasalDebtField, a.Body.BasalDebt)),
			reproductionSeed(actor, ReproductionWorksiteTypeID,
				reproductionField(ReproductionWorksiteCapitalField, a.Worksite.Capital), reproductionField(ReproductionWorksiteWipField, a.Worksite.Wip),
				reproductionField(ReproductionWorksiteWearDebtField, a.Worksite.WearDebt), reproductionField(ReproductionWorksiteInvestedUnitsField, a.Worksite.InvestedUnits),
				reproductionField(ReproductionWorksitePointsCreatedField, a.Worksite.PointsCreated), reproductionField(ReproductionWorksitePointsDecayedField, a.Worksite.PointsDecayed),
				reproductionField(ReproductionWorksiteLastBuildHourField, a.Worksite.LastBuildHour)),
			reproductionSeed(actor, ReproductionGranaryTypeID,
				reproductionField(ReproductionGranaryStockField, a.Granary.Stock), reproductionField(ReproductionGranaryYieldTotalField, a.Granary.YieldTotal),
				reproductionField(ReproductionGranaryYieldUnrealizedField, a.Granary.YieldUnrealized), reproductionField(ReproductionGranaryStoredMealsField, a.Granary.StoredMeals),
				reproductionField(ReproductionGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour), reproductionField(ReproductionGranaryYieldDebtField, a.Granary.YieldDebt)),
			reproductionSeed(actor, ReproductionGenomeTypeID,
				reproductionField(ReproductionGenomeLocusMField, a.Genome.LocusM), reproductionField(ReproductionGenomeLocusYField, a.Genome.LocusY),
				reproductionField(ReproductionGenomeLocusWField, a.Genome.LocusW),
				component.FieldSeed{Field: ReproductionGenomeParentAField, Value: parentA}, component.FieldSeed{Field: ReproductionGenomeParentBField, Value: parentB},
				reproductionField(ReproductionGenomeBirthHourField, a.Genome.BirthHour), reproductionField(ReproductionGenomeDeathHourField, a.Genome.DeathHour),
				reproductionField(ReproductionGenomeDiedHourField, a.Genome.DiedHour), reproductionField(ReproductionGenomeBirthGranaryPaidField, a.Genome.BirthGranaryPaid)),
			reproductionSeed(actor, ReproductionBirthRequestTypeID,
				reproductionField(ReproductionBirthRequestHourField, a.Request.Hour), component.FieldSeed{Field: ReproductionBirthRequestAddresseeField, Value: addressee},
				reproductionField(ReproductionBirthRequestReportedCapitalField, a.Request.ReportedCapital), reproductionField(ReproductionBirthRequestReportedGranaryField, a.Request.ReportedGranary),
				reproductionField(ReproductionBirthRequestStatusField, int64(a.Request.Status)), reproductionField(ReproductionBirthRequestLastRequestHourField, a.Request.LastRequestHour),
				reproductionField(ReproductionBirthRequestLastReplyHourField, a.Request.LastReplyHour)))
		if _, err = policies.Bind(strategy.ReproductionBinding{Actor: actor, Ref: policy.Ref}); err != nil {
			return nil, err
		}
		f.roster = append(f.roster, actor)
		f.pulseLifecycle[actor] = scheduler.Alive
	}
	f.k, err = kernel.New(reg, 0, f.seeds)
	if err != nil {
		return nil, err
	}
	f.sched, err = scheduler.New(f.k, o.Workers, f.evaluate)
	if err != nil {
		return nil, err
	}
	for _, actor := range f.roster {
		f.bound[actor], err = policies.Bind(strategy.ReproductionBinding{Actor: actor, Ref: policy.Ref})
		if err != nil {
			return nil, err
		}
		if err = f.sched.Register(actor); err != nil {
			return nil, err
		}
	}
	if err = f.sched.Register(reproductionClockActor); err != nil {
		return nil, err
	}
	if err = f.sched.Schedule(scheduler.Wake{Actor: reproductionClockActor, At: 0, Cause: scheduler.WakeAudit}); err != nil {
		return nil, err
	}
	return f, nil
}

// reproductionActorState reads one actor's six component rows at one version.
func reproductionActorState(view scheduler.SnapshotView, actor sim.EntityID) (ReproductionActorState, error) {
	body, err := capacityInts(view, actor, ReproductionBodyTypeID,
		ReproductionBodyEnergyField, ReproductionBodyHungerField, ReproductionBodyBasalSpentField, ReproductionBodyCapLostField, ReproductionBodyConsumedField, ReproductionBodyLastGatherHourField, ReproductionBodyBasalDebtField)
	if err != nil {
		return ReproductionActorState{}, err
	}
	bagValues, err := capacityRead(view, actor, ReproductionBagTypeID, ReproductionBagUnitsField, ReproductionBagSourceField)
	if err != nil {
		return ReproductionActorState{}, err
	}
	units, err := bagValues[0].Integer()
	if err != nil {
		return ReproductionActorState{}, err
	}
	source, err := capacityRefValue(bagValues[1])
	if err != nil {
		return ReproductionActorState{}, err
	}
	worksite, err := capacityInts(view, actor, ReproductionWorksiteTypeID,
		ReproductionWorksiteCapitalField, ReproductionWorksiteWipField, ReproductionWorksiteWearDebtField, ReproductionWorksiteInvestedUnitsField, ReproductionWorksitePointsCreatedField, ReproductionWorksitePointsDecayedField, ReproductionWorksiteLastBuildHourField)
	if err != nil {
		return ReproductionActorState{}, err
	}
	granary, err := capacityInts(view, actor, ReproductionGranaryTypeID,
		ReproductionGranaryStockField, ReproductionGranaryYieldTotalField, ReproductionGranaryYieldUnrealizedField, ReproductionGranaryStoredMealsField, ReproductionGranaryLastStoredMealHourField, ReproductionGranaryYieldDebtField)
	if err != nil {
		return ReproductionActorState{}, err
	}
	genomeValues, err := capacityRead(view, actor, ReproductionGenomeTypeID,
		ReproductionGenomeLocusMField, ReproductionGenomeLocusYField, ReproductionGenomeLocusWField, ReproductionGenomeParentAField, ReproductionGenomeParentBField,
		ReproductionGenomeBirthHourField, ReproductionGenomeDeathHourField, ReproductionGenomeDiedHourField, ReproductionGenomeBirthGranaryPaidField)
	if err != nil {
		return ReproductionActorState{}, err
	}
	genomeInts := make([]int64, len(genomeValues))
	for i, v := range genomeValues {
		if i == 3 || i == 4 {
			continue
		}
		if genomeInts[i], err = v.Integer(); err != nil {
			return ReproductionActorState{}, err
		}
	}
	parentA, err := capacityRefValue(genomeValues[3])
	if err != nil {
		return ReproductionActorState{}, err
	}
	parentB, err := capacityRefValue(genomeValues[4])
	if err != nil {
		return ReproductionActorState{}, err
	}
	requestValues, err := capacityRead(view, actor, ReproductionBirthRequestTypeID,
		ReproductionBirthRequestHourField, ReproductionBirthRequestAddresseeField, ReproductionBirthRequestReportedCapitalField, ReproductionBirthRequestReportedGranaryField,
		ReproductionBirthRequestStatusField, ReproductionBirthRequestLastRequestHourField, ReproductionBirthRequestLastReplyHourField)
	if err != nil {
		return ReproductionActorState{}, err
	}
	requestInts := make([]int64, len(requestValues))
	for i, v := range requestValues {
		if i == 1 {
			continue
		}
		if requestInts[i], err = v.Integer(); err != nil {
			return ReproductionActorState{}, err
		}
	}
	addressee, err := capacityRefValue(requestValues[1])
	if err != nil {
		return ReproductionActorState{}, err
	}
	a := ReproductionActorState{
		Body:     ReproductionBodyState{Energy: body[0], Hunger: body[1], BasalSpent: body[2], CapLost: body[3], Consumed: body[4], LastGatherHour: body[5], BasalDebt: body[6]},
		Bag:      ReproductionBagState{Units: units, Source: source},
		Worksite: ReproductionWorksiteState{Capital: worksite[0], Wip: worksite[1], WearDebt: worksite[2], InvestedUnits: worksite[3], PointsCreated: worksite[4], PointsDecayed: worksite[5], LastBuildHour: worksite[6]},
		Granary:  ReproductionGranaryState{Stock: granary[0], YieldTotal: granary[1], YieldUnrealized: granary[2], StoredMeals: granary[3], LastStoredMealHour: granary[4], YieldDebt: granary[5]},
		Genome:   ReproductionGenomeState{LocusM: genomeInts[0], LocusY: genomeInts[1], LocusW: genomeInts[2], ParentA: parentA, ParentB: parentB, BirthHour: genomeInts[5], DeathHour: genomeInts[6], DiedHour: genomeInts[7], BirthGranaryPaid: genomeInts[8]},
		Request:  ReproductionBirthRequestState{Hour: requestInts[0], Addressee: addressee, ReportedCapital: requestInts[2], ReportedGranary: requestInts[3], Status: ReproductionRequestStatus(requestInts[4]), LastRequestHour: requestInts[5], LastReplyHour: requestInts[6]},
	}
	if a.Request.Status < ReproductionRequestIdle || a.Request.Status > ReproductionRequestExpired {
		return ReproductionActorState{}, ErrReproductionContract
	}
	return a, nil
}

// reproductionWorldState reads the full authoritative v4 state at one
// version: the two patches, their slots, and every roster actor's six rows
// (founders 1..16 plus the newborn prefix 10001.. discovered by genome scan,
// the frozen next-ID rule).
func reproductionWorldState(view scheduler.SnapshotView) ([ReproductionPatchCount]ReproductionPatchState, [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState, map[sim.EntityID]ReproductionActorState, error) {
	var patches [ReproductionPatchCount]ReproductionPatchState
	var slots [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState
	actors := make(map[sim.EntityID]ReproductionActorState, ReproductionFounderCount)
	for i := range patches {
		patch, _ := ReproductionPatchID(i)
		n, err := capacityInts(view, patch, ReproductionPatchTypeID, ReproductionPatchYieldField, ReproductionPatchPulsesField, ReproductionPatchProducedField, ReproductionPatchUnrealizedField)
		if err != nil {
			return patches, slots, actors, err
		}
		patches[i] = ReproductionPatchState{n[0], n[1], n[2], n[3]}
		for j := range slots[i] {
			slot, _ := ReproductionSlotID(i, j)
			n, err := capacityInts(view, slot, ReproductionSlotTypeID, ReproductionSlotStockField, ReproductionSlotGatheredField)
			if err != nil {
				return patches, slots, actors, err
			}
			slots[i][j] = ReproductionSlotState{n[0], n[1]}
		}
	}
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		a, err := reproductionActorState(view, actor)
		if err != nil {
			return patches, slots, actors, err
		}
		actors[actor] = a
	}
	births, err := reproductionBirthCount(view)
	if err != nil {
		return patches, slots, actors, err
	}
	for i := int64(0); i < births; i++ {
		id, err := ReproductionNewbornID(i)
		if err != nil {
			return patches, slots, actors, err
		}
		a, err := reproductionActorState(view, id)
		if err != nil {
			return patches, slots, actors, err
		}
		actors[id] = a
	}
	return patches, slots, actors, nil
}

// reproductionBirthCount derives the committed birth count by genome scan:
// the newborn IDs are a prefix of 10001..10016 (frozen next = 10000+births+1).
func reproductionBirthCount(view scheduler.SnapshotView) (int64, error) {
	births := int64(0)
	for i := 0; i < ReproductionMaxBirths; i++ {
		id, err := ReproductionNewbornID(int64(i))
		if err != nil {
			return 0, err
		}
		has, err := view.Reader.Has(component.HasRequest{Entity: id, Component: ReproductionGenomeTypeID, WorldVersion: view.Version, Authority: view.Authority})
		if err != nil {
			return 0, err
		}
		if !has {
			break
		}
		births++
	}
	return births, nil
}

func reproductionSortedIDs(actors map[sim.EntityID]ReproductionActorState) []sim.EntityID {
	ids := make([]sim.EntityID, 0, len(actors))
	for id := range actors {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// observe builds the actor-visible v4 claim view: the v3 own-set plus the
// home-patch slot stocks and the denial guard. The home patch is the bag's
// always-present source row (ID-placed for founders, proposer-placed for
// newborns). Others' state has no field in the observation type at all.
func (f *Reproduction) observe(view scheduler.SnapshotView, actor sim.EntityID, hour int, denial strategy.ReproductionDenial) (strategy.ReproductionObservation, error) {
	a, err := reproductionActorState(view, actor)
	if err != nil {
		return strategy.ReproductionObservation{}, err
	}
	home := a.Bag.Source
	index := int(home - 1001)
	if index < 0 || index >= ReproductionPatchCount {
		return strategy.ReproductionObservation{}, ErrReproductionRunner
	}
	obs := strategy.ReproductionObservation{
		Actor: actor, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		BagSource: bagSourceValue(a.Bag.Source),
	}
	body, err := capacityRead(view, actor, ReproductionBodyTypeID, ReproductionBodyEnergyField, ReproductionBodyHungerField)
	if err != nil {
		return strategy.ReproductionObservation{}, err
	}
	worksite, err := capacityRead(view, actor, ReproductionWorksiteTypeID, ReproductionWorksiteCapitalField, ReproductionWorksiteWipField, ReproductionWorksiteInvestedUnitsField, ReproductionWorksiteLastBuildHourField)
	if err != nil {
		return strategy.ReproductionObservation{}, err
	}
	granary, err := capacityRead(view, actor, ReproductionGranaryTypeID, ReproductionGranaryStockField, ReproductionGranaryLastStoredMealHourField)
	if err != nil {
		return strategy.ReproductionObservation{}, err
	}
	bag, err := capacityRead(view, actor, ReproductionBagTypeID, ReproductionBagUnitsField)
	if err != nil {
		return strategy.ReproductionObservation{}, err
	}
	obs.Energy, obs.Hunger, obs.BagUnits = body[0], body[1], bag[0]
	obs.Capital, obs.Wip, obs.InvestedUnits, obs.LastBuildHour = worksite[0], worksite[1], worksite[2], worksite[3]
	obs.GranaryStock, obs.LastStoredMealHour = granary[0], granary[1]
	obs.LastDenial = denial
	for j := 0; j < ReproductionSlotsPerPatch; j++ {
		slot, _ := ReproductionSlotID(index, j)
		stock, err := capacityRead(view, slot, ReproductionSlotTypeID, ReproductionSlotStockField)
		if err != nil {
			return strategy.ReproductionObservation{}, err
		}
		obs.HomeSlots = append(obs.HomeSlots, strategy.ReproductionSlotView{ID: slot, Stock: stock[0]})
	}
	return obs, nil
}

func bagSourceValue(home sim.EntityID) sim.Value {
	v, err := sim.EntityRefValue(home)
	if err != nil {
		return sim.Value{}
	}
	return v
}

// guard is the runner-side ownership duty: every reproduction proposal passes
// ReproductionCheckOwnership before it can be planned (the kernel validates
// schema and rule admission only).
func (f *Reproduction) guard(p kernel.Proposal) error {
	if p.RuleVersion != ReproductionRuleVersion {
		return ErrReproductionRunner
	}
	return ReproductionCheckOwnership(p)
}

// reproductionHomePatch returns the actor's home patch: the fixed founder
// placement, or the newborn's proposer-placed bag source row.
func reproductionHomePatch(actor sim.EntityID, a ReproductionActorState) (sim.EntityID, error) {
	if reproductionFounderEntity(actor) {
		return ReproductionFounderPatchID(actor)
	}
	if !reproductionValidPatch(a.Bag.Source) {
		return 0, ErrReproductionContract
	}
	return a.Bag.Source, nil
}

func (f *Reproduction) pulse(ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	at := ready.At
	h := int(at / sim.SimTime(ReproductionHour))
	if h > ReproductionHorizonHours || at != sim.SimTime(h)*sim.SimTime(ReproductionHour) {
		return scheduler.Evaluation{}, ErrReproductionRunner
	}
	patches, slots, actors, err := reproductionWorldState(view)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	if len(actors) != len(f.roster) {
		return scheduler.Evaluation{}, ErrReproductionRunner
	}
	for _, hour := range f.birthWakes {
		if hour < int64(h) {
			return scheduler.Evaluation{}, ErrReproductionRunner // a reserved first wake never fired
		}
	}
	if !f.births && len(f.birthWakes) != 0 {
		return scheduler.Evaluation{}, ErrReproductionRunner
	}
	// Actor quiescence at the pulse (every activity and deferred meal of the
	// previous hour closed) was verified by Step; here the lifecycle must
	// match the frozen alive invariant exactly: alive ⇔ died-hour == -1 ∧
	// energy > 0.
	for _, id := range f.roster {
		a, ok := actors[id]
		if !ok {
			return scheduler.Evaluation{}, ErrReproductionRunner
		}
		if (f.pulseLifecycle[id] == scheduler.Alive) != reproductionAlive(a) {
			return scheduler.Evaluation{}, ErrReproductionRunner
		}
	}
	var out scheduler.Evaluation
	live := make(map[sim.EntityID]bool, len(f.roster))
	death := func(id sim.EntityID, cause string) {
		f.pending.mu.Lock()
		f.pending.deaths = append(f.pending.deaths, reproductionDeath{Actor: id, Cause: cause})
		f.pending.mu.Unlock()
	}
	// Frozen pulse order, stage 1: world-caused age-death (rule 413). The
	// death-hour is immutable; a dying actor stops with its energy retained
	// (v1 stopped-state convention) and never pays basal again.
	for _, id := range f.roster {
		a := actors[id]
		if !reproductionAlive(a) {
			continue
		}
		if int64(h) < a.Genome.DeathHour {
			live[id] = true
			continue
		}
		genome, died, err := ReproductionAgeDeath(h, id, a.Genome)
		if err != nil || !died {
			return out, ErrReproductionRunner
		}
		proposal := reproductionProposal("age-death", at, id, ReproductionAgeDeathRule,
			reproductionNumber(id, ReproductionGenomeTypeID, ReproductionGenomeDiedHourField, genome.DiedHour))
		proposal.Cause = kernel.Cause{World: true}
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
		a.Genome = genome
		actors[id] = a
		death(id, "age")
	}
	// Stage 2: basal for the living (rule 404). A starved actor writes its
	// died-hour (only rules 404/413 may) and stops; energy hits zero.
	if h >= 1 {
		for _, id := range f.roster {
			if !live[id] {
				continue
			}
			a := actors[id]
			next, _, starved, err := ReproductionBasal(h, id, a, 1)
			if err != nil {
				return out, err
			}
			ps := []component.Patch{
				reproductionNumber(id, ReproductionBodyTypeID, ReproductionBodyEnergyField, next.Body.Energy),
				reproductionNumber(id, ReproductionBodyTypeID, ReproductionBodyHungerField, next.Body.Hunger),
				reproductionNumber(id, ReproductionBodyTypeID, ReproductionBodyBasalSpentField, next.Body.BasalSpent)}
			if next.Body.BasalDebt != a.Body.BasalDebt {
				ps = append(ps, reproductionNumber(id, ReproductionBodyTypeID, ReproductionBodyBasalDebtField, next.Body.BasalDebt))
			}
			if starved {
				// Rule 404 writes the died-hour (only rules 404/413 may).
				ps = append(ps, reproductionNumber(id, ReproductionGenomeTypeID, ReproductionGenomeDiedHourField, next.Genome.DiedHour))
			}
			proposal := reproductionProposal("basal", at, id, ReproductionBasalRule, ps...)
			proposal.Cause = kernel.Cause{World: true} // elapsed need is a world pulse, not an actor action
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			if starved {
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStop, Actor: id})
				live[id] = false
				death(id, "starvation")
			}
			actors[id] = next
		}
	}
	// Stage 3: wild production, verbatim v3.
	if h < ReproductionHorizonHours {
		for i, p := range patches {
			next, stock, _, err := ReproductionProduceWild(h, p, slots[i])
			if err != nil {
				return out, err
			}
			patch, _ := ReproductionPatchID(i)
			ps := []component.Patch{
				reproductionNumber(patch, ReproductionPatchTypeID, ReproductionPatchPulsesField, next.Pulses),
				reproductionNumber(patch, ReproductionPatchTypeID, ReproductionPatchProducedField, next.Produced),
				reproductionNumber(patch, ReproductionPatchTypeID, ReproductionPatchUnrealizedField, next.Unrealized)}
			for j, s := range stock {
				if s.Stock != slots[i][j].Stock {
					slot, _ := ReproductionSlotID(i, j)
					ps = append(ps, reproductionNumber(slot, ReproductionSlotTypeID, ReproductionSlotStockField, s.Stock))
				}
			}
			proposal := reproductionProposal(fmt.Sprintf("produce-%d", i), at, 0, ReproductionProduceRule, ps...)
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			patches[i] = next
			slots[i] = stock
		}
	}
	// Stages 4-5: capital yield for living owners only, then wear accrual for
	// every owner, dead owners included (dead worksites decay, dead granaries
	// freeze). h=168 is basal and mortality only, as in v1.
	if h >= 1 && h < ReproductionHorizonHours {
		for _, id := range f.roster {
			a := actors[id]
			if live[id] && a.Worksite.Capital > 0 {
				next, _, err := ReproductionCapitalYield(id, a)
				if err != nil {
					return out, err
				}
				ps := []component.Patch{reproductionNumber(id, ReproductionGranaryTypeID, ReproductionGranaryYieldTotalField, next.Granary.YieldTotal)}
				if next.Granary.Stock != a.Granary.Stock {
					ps = append(ps, reproductionNumber(id, ReproductionGranaryTypeID, ReproductionGranaryStockField, next.Granary.Stock))
				}
				if next.Granary.YieldUnrealized != a.Granary.YieldUnrealized {
					ps = append(ps, reproductionNumber(id, ReproductionGranaryTypeID, ReproductionGranaryYieldUnrealizedField, next.Granary.YieldUnrealized))
				}
				if next.Granary.YieldDebt != a.Granary.YieldDebt {
					ps = append(ps, reproductionNumber(id, ReproductionGranaryTypeID, ReproductionGranaryYieldDebtField, next.Granary.YieldDebt))
				}
				proposal := reproductionProposal("yield", at, id, ReproductionYieldRule, ps...)
				proposal.Cause = kernel.Cause{World: true}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				a = next
			}
			if a.Worksite.Capital > 0 {
				next, _, err := ReproductionWear(id, a)
				if err != nil {
					return out, err
				}
				ps := []component.Patch{reproductionNumber(id, ReproductionWorksiteTypeID, ReproductionWorksiteWearDebtField, next.Worksite.WearDebt)}
				if next.Worksite.Capital != a.Worksite.Capital {
					ps = append(ps, reproductionNumber(id, ReproductionWorksiteTypeID, ReproductionWorksiteCapitalField, next.Worksite.Capital))
				}
				if next.Worksite.PointsDecayed != a.Worksite.PointsDecayed {
					ps = append(ps, reproductionNumber(id, ReproductionWorksiteTypeID, ReproductionWorksitePointsDecayedField, next.Worksite.PointsDecayed))
				}
				proposal := reproductionProposal("wear", at, id, ReproductionWearRule, ps...)
				proposal.Cause = kernel.Cause{World: true}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				a = next
			}
			actors[id] = a
		}
	}
	// Stage 6: world-caused expiry (rule 412). Every request still pending
	// from h-1 expires at this pulse; accepted, refused, and already-resolved
	// rows are untouched.
	if h >= 1 {
		for _, id := range f.roster {
			r := actors[id].Request
			if r.Status != ReproductionRequestPending || r.Hour != int64(h)-1 {
				continue
			}
			next, err := ReproductionBirthExpiry(h, r)
			if err != nil {
				return out, err
			}
			proposal := reproductionProposal("expiry", at, id, ReproductionBirthExpireRule,
				reproductionNumber(id, ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField, int64(next.Status)))
			proposal.Cause = kernel.Cause{World: true}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			expired := actors[id]
			expired.Request = next
			actors[id] = expired
		}
	}
	// Stage 7: wakes. Every living actor claims at h+1µs — except a newborn
	// whose first claim is already reserved as its WakeBirth wake, which is
	// handled as a claim distinguished only in evidence — and, when births
	// are enabled, evaluates the birth-request cascade at h+40m+5µs.
	if h < ReproductionHorizonHours {
		claim, err := ReproductionPhaseTime(h, ReproductionPhaseClaim)
		if err != nil {
			return out, err
		}
		var requestAt sim.SimTime
		if f.births {
			requestAt, err = ReproductionPhaseTime(h, ReproductionPhaseBirthRequest)
			if err != nil {
				return out, err
			}
		}
		for _, id := range f.roster {
			if !live[id] {
				continue
			}
			reserved, hasReserved := f.birthWakes[id]
			if f.births && hasReserved && reserved == int64(h) {
				// The reserved WakeBirth first claim is already scheduled;
				// never double-schedule this instant.
			} else {
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeNeedThreshold}})
			}
			if f.births {
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: requestAt, Cause: scheduler.WakeAudit}})
			}
		}
		next, _ := ReproductionHourTime(h + 1)
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: reproductionClockActor, Wake: scheduler.Wake{Actor: reproductionClockActor, At: next, Cause: scheduler.WakeAudit}})
	}
	return out, nil
}

func reproductionNumber(entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID, n int64) component.Patch {
	return component.Patch{Entity: entity, Component: typ, SchemaVersion: ReproductionSchemaVersion, Field: field, Value: sim.IntegerValue(n)}
}

// capacityPairIntent is the frozen complement of the bag-decision Build
// condition, verbatim v3 with the frozen v4 policy floor.
func reproductionPairIntent(a ReproductionActorState, hour int) bool {
	return a.Body.Energy < ReproductionPolicyTLow && a.Granary.Stock >= 1 && a.Granary.LastStoredMealHour < int64(hour)
}

// reproductionPairedMealGate revalidates the fresh snapshot before any
// paired EatStored at h+40m+4µs, verbatim v3.
func reproductionPairedMealGate(a ReproductionActorState, hour int) bool {
	return a.Body.Energy > 0 && a.Granary.Stock >= 1 && a.Granary.LastStoredMealHour < int64(hour)
}

// pairedMealDecision consumes one deferred meal intent against a fresh
// snapshot. A raced granary yields the typed no-meal note with zero
// mutation; the pairing build itself is already standing.
func (f *Reproduction) pairedMealDecision(actor sim.EntityID, hour int, intent ReproductionMealIntent, a ReproductionActorState) (kernel.Proposal, ReproductionAttempt, error) {
	kind := "fallback-eatstored"
	if intent.Paired {
		kind = "paired-meal"
	}
	paired, err := ReproductionPhaseTime(hour, ReproductionPhasePairedMeal)
	if err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	attempt := ReproductionAttempt{Actor: actor, Time: paired, Kind: kind}
	if !reproductionPairedMealGate(a, hour) {
		attempt.NoMeal = capacityNoStoredMealNote
		return kernel.Proposal{}, attempt, nil
	}
	proposal, err := ReproductionEatStoredProposal(reproductionKey(kind, paired, actor), hour, paired, actor, a)
	if err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	if err = f.guard(proposal); err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	attempt.Key = proposal.Key
	return proposal, attempt, nil
}

func (f *Reproduction) evaluate(_ context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	if ready.Fiber.Actor == reproductionClockActor {
		return f.pulse(ready, view)
	}
	id := ready.Fiber.Actor
	if !f.rosterContains(id) || len(ready.Causes) != 1 {
		return scheduler.Evaluation{}, ErrReproductionRunner
	}
	h := int(ready.At / sim.SimTime(ReproductionHour))
	phase := func(n int) (sim.SimTime, error) { return ReproductionPhaseTime(h, n) }
	claim, err := phase(ReproductionPhaseClaim)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	gathered, _ := phase(ReproductionPhaseGathered)
	bagDecision, _ := phase(ReproductionPhaseBagDecision)
	buildStart, _ := phase(ReproductionPhaseBuildStart)
	buildCompletion := ReproductionBuildCompletion(buildStart)
	paired, _ := phase(ReproductionPhasePairedMeal)
	birthRequestAt, _ := phase(ReproductionPhaseBirthRequest)
	birthReplyAt, _ := phase(ReproductionPhaseBirthReply)
	at := ready.At
	var out scheduler.Evaluation
	record := func(a ReproductionAttempt) {
		f.pending.mu.Lock()
		f.pending.attempts = append(f.pending.attempts, a)
		f.pending.mu.Unlock()
	}
	switch {
	case at == claim && (ready.Causes[0] == scheduler.WakeNeedThreshold || ready.Causes[0] == scheduler.WakeBirth):
		// Claim: policy Gather|EatStored|Wait. A newborn's first claim
		// arrives as the reserved WakeBirth cause and is handled exactly as
		// a claim NeedThreshold wake, distinguished only in evidence.
		birthWake := ready.Causes[0] == scheduler.WakeBirth
		reserved, hasReserved := f.birthWakes[id]
		if birthWake != (hasReserved && reserved == int64(h)) {
			return out, ErrReproductionRunner // wake-topology violation
		}
		if birthWake {
			f.pending.mu.Lock()
			f.pending.birthWakesUsed = append(f.pending.birthWakesUsed, id)
			f.pending.mu.Unlock()
		}
		if f.current[id] != 0 {
			return out, ErrReproductionRunner
		}
		_, _, actors, err := reproductionWorldState(view)
		if err != nil {
			return out, err
		}
		a, ok := actors[id]
		if !ok {
			return out, ErrReproductionRunner
		}
		if a.Bag.Units != 0 {
			return out, ErrReproductionRunner
		}
		obs, err := f.observe(view, id, h, strategy.ReproductionNoDenial)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[id].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrReproductionRunner
		}
		switch choice.Kind {
		case strategy.ReproductionGather:
			admission, ok := f.admissions[id]
			if !ok || admission.Actor != id {
				return out, ErrReproductionRunner
			}
			attempt := ReproductionAttempt{Actor: id, Time: at, Kind: "gather", Target: admission.Slot, Rejection: admission.Rejection, BirthWake: birthWake}
			if admission.Rejection == FoodFlowGatherAdmitted {
				_, slots, _, err := reproductionWorldState(view)
				if err != nil {
					return out, err
				}
				home := a.Bag.Source
				if admission.Patch != home {
					return out, ErrReproductionRunner
				}
				index := int(admission.Slot - 2001)
				if index < 0 || index >= ReproductionPatchCount*ReproductionSlotsPerPatch {
					return out, ErrReproductionRunner
				}
				slotBefore := slots[index/ReproductionSlotsPerPatch][index%ReproductionSlotsPerPatch]
				proposal, err := ReproductionGatherProposal(reproductionKey("gather", at, id), h, id, admission.Slot, slotBefore, a)
				if err != nil {
					return out, err
				}
				if err = f.guard(proposal); err != nil {
					return out, err
				}
				out.Proposals = append(out.Proposals, proposal)
				// The gather activity is guarded by its own commit: no
				// phantom window without the atomic slot→bag transfer.
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: ReproductionGatherDuration, IfKey: proposal.Key})
				f.pending.mu.Lock()
				f.pending.started[id] = strategy.ReproductionGather
				f.pending.mu.Unlock()
			} else {
				f.pending.mu.Lock()
				f.pending.denied = append(f.pending.denied, id)
				f.pending.mu.Unlock()
				out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: gathered, Cause: scheduler.WakeAudit}})
			}
			record(attempt)
		case strategy.ReproductionEatStored:
			proposal, err := ReproductionEatStoredProposal(reproductionKey("eat-stored", at, id), h, at, id, a)
			if err != nil {
				return out, err
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(ReproductionAttempt{Actor: id, Time: at, Kind: "eat-stored", Key: proposal.Key, BirthWake: birthWake})
		case strategy.ReproductionWait:
			record(ReproductionAttempt{Actor: id, Time: at, Kind: "wait", BirthWake: birthWake})
		default:
			return out, ErrReproductionRunner
		}
	case ready.Causes[0] == scheduler.WakeCompletion && at == gathered:
		// Admitted gather completion: the atomic gather committed at claim;
		// the actor now receives its frozen bag-decision wake.
		if f.current[id] != strategy.ReproductionGather {
			return out, ErrReproductionRunner
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
			return out, ErrReproductionRunner
		}
		obs, err := f.observe(view, id, h, strategy.ReproductionGatherDenied)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[id].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrReproductionRunner
		}
		switch choice.Kind {
		case strategy.ReproductionRest:
			if choice.RestDuration != strategy.ReproductionRestDuration {
				return out, ErrReproductionRunner
			}
			record(ReproductionAttempt{Actor: id, Time: at, Kind: "rest"})
		case strategy.ReproductionEatStored:
			f.pending.mu.Lock()
			f.pending.meals[id] = ReproductionMealIntent{Hour: h, Paired: false}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
		default:
			return out, ErrReproductionRunner
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == bagDecision:
		// Bag decision: Build | Eat per the frozen policy on the held unit.
		// v4 has no capacity-disable branch: the capital machinery that feeds
		// the surplus gate is always on; only births are the intervention.
		if f.current[id] != 0 {
			return out, ErrReproductionRunner
		}
		_, _, actors, err := reproductionWorldState(view)
		if err != nil {
			return out, err
		}
		a, ok := actors[id]
		if !ok {
			return out, ErrReproductionRunner
		}
		if a.Bag.Units != 1 {
			return out, ErrReproductionRunner
		}
		obs, err := f.observe(view, id, h, strategy.ReproductionNoDenial)
		if err != nil {
			return out, err
		}
		choice, err := f.bound[id].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return out, ErrReproductionRunner
		}
		switch choice.Kind {
		case strategy.ReproductionBuild:
			f.pending.mu.Lock()
			if reproductionPairIntent(a, h) {
				f.pending.meals[id] = ReproductionMealIntent{Hour: h, Paired: true}
			}
			f.pending.mu.Unlock()
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: buildStart, Cause: scheduler.WakeAudit}})
		case strategy.ReproductionEat:
			proposal, err := ReproductionConsumeProposal(reproductionKey("eat", at, id), h, id, a)
			if err != nil {
				return out, err
			}
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
			record(ReproductionAttempt{Actor: id, Time: at, Kind: "eat", Key: proposal.Key})
		default:
			return out, ErrReproductionRunner
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == buildStart:
		// Build start: the 20-minute window opens exactly at the frozen
		// instant so its completion is ReproductionBuildCompletion.
		_, _, actors, err := reproductionWorldState(view)
		if err != nil {
			return out, err
		}
		a, ok := actors[id]
		if !ok || a.Bag.Units != 1 || f.current[id] != 0 {
			return out, ErrReproductionRunner
		}
		out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectStart, Actor: id, Duration: ReproductionBuildDuration})
		f.pending.mu.Lock()
		f.pending.started[id] = strategy.ReproductionBuild
		f.pending.mu.Unlock()
	case ready.Causes[0] == scheduler.WakeCompletion && at == buildCompletion:
		// Build completion: the atomic bag→wip/k conversion from a FRESH
		// snapshot. Nothing else writes the builder's bag, so failure here is
		// a linkage break (stop condition), not typed evidence.
		if f.current[id] != strategy.ReproductionBuild {
			return out, ErrReproductionRunner
		}
		f.pending.mu.Lock()
		f.pending.completed = append(f.pending.completed, id)
		f.pending.mu.Unlock()
		_, _, actors, err := reproductionWorldState(view)
		if err != nil {
			return out, err
		}
		a, ok := actors[id]
		if !ok {
			return out, ErrReproductionRunner
		}
		proposal, err := ReproductionBuildProposal(reproductionKey("build", at, id), h, id, a)
		if err != nil {
			return out, err
		}
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		record(ReproductionAttempt{Actor: id, Time: at, Kind: "build", Key: proposal.Key})
		if intent, ok := f.meals[id]; ok && intent.Hour == h && intent.Paired {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: id, Wake: scheduler.Wake{Actor: id, At: paired, Cause: scheduler.WakeAudit}})
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == paired:
		// Paired instant: fresh-snapshot granary revalidation before any
		// paired EatStored (builder pairing or denied-gather fallback).
		intent, ok := f.meals[id]
		if !ok || intent.Hour != h || f.current[id] != 0 {
			return out, ErrReproductionRunner
		}
		f.pending.mu.Lock()
		f.pending.mealsCleared = append(f.pending.mealsCleared, id)
		f.pending.mu.Unlock()
		_, _, actors, err := reproductionWorldState(view)
		if err != nil {
			return out, err
		}
		a, ok := actors[id]
		if !ok {
			return out, ErrReproductionRunner
		}
		proposal, attempt, err := f.pairedMealDecision(id, h, intent, a)
		if err != nil {
			return out, err
		}
		if proposal.Key != "" {
			out.Proposals = append(out.Proposals, proposal)
		}
		record(attempt)
	case ready.Causes[0] == scheduler.WakeAudit && at == birthRequestAt:
		// Birth request (rule 409): the proposer files an addressed request
		// with self-reported claims. Eligibility and the deterministic
		// same-patch addressee are the frozen policy's; the world re-checks
		// state and one-request-per-actor-hour. Waits are not journaled.
		if !f.births || f.current[id] != 0 {
			return out, ErrReproductionRunner
		}
		proposal, filed, err := f.birthRequestDecision(view, h, at, id)
		if err != nil {
			return out, err
		}
		if !filed {
			return out, nil
		}
		if err = f.guard(proposal); err != nil {
			return out, err
		}
		out.Proposals = append(out.Proposals, proposal)
		record(ReproductionAttempt{Actor: id, Time: at, Kind: "birth-request", Key: proposal.Key})
		f.pending.mu.Lock()
		f.pending.birthFiles++
		f.pending.mu.Unlock()
		// The reply wake goes to a LIVING addressee only. Nothing can die
		// between +5µs and +6µs (basal and mortality run only at pulses), so
		// this snapshot verdict is final for the hour; a dead addressee lets
		// the request expire at the next pulse.
		wake, err := f.birthReplyWake(view, h, proposal)
		if err != nil {
			return out, err
		}
		if wake != (scheduler.Wake{}) {
			out.Effects = append(out.Effects, scheduler.Effect{Kind: scheduler.EffectSchedule, Actor: wake.Actor, Wake: wake})
		}
	case ready.Causes[0] == scheduler.WakeAudit && at == birthReplyAt:
		// Birth reply (rules 410/411): the consenter decides; the runner
		// revalidates the full surplus gate on fresh truth through the
		// contract. Acceptance commits one atomic entity-create; refusal
		// commits the typed two-row rule 411; a death between phases would
		// be typed zero-mutation evidence. No pending request: no evidence.
		if !f.births || f.current[id] != 0 {
			return out, ErrReproductionRunner
		}
		proposal, attempt, err := reproductionBirthReply(view, f.seed, f.ref, f.bound[id], h, id)
		if err != nil {
			return out, err
		}
		if attempt.Actor == 0 {
			return out, nil // no pending request addressed to this consenter
		}
		if proposal.Key != "" {
			if err = f.guard(proposal); err != nil {
				return out, err
			}
			out.Proposals = append(out.Proposals, proposal)
		}
		record(attempt)
		f.pending.mu.Lock()
		if attempt.Birth.Decision == ReproductionReplyAccept {
			if attempt.Birth.Diverged {
				f.pending.divergedReplies++
			}
		} else {
			f.pending.replyReasons = append(f.pending.replyReasons, attempt.Birth.Reason)
			if attempt.Birth.Diverged {
				f.pending.divergedReplies++
			}
		}
		f.pending.mu.Unlock()
	default:
		return out, ErrReproductionRunner
	}
	return out, nil
}

func (f *Reproduction) rosterContains(id sim.EntityID) bool {
	_, ok := f.bound[id]
	return ok
}

// birthRequestDecision evaluates the frozen proposer cascade at h+40m+5µs
// and builds the validated rule-409 proposal when the policy files.
func (f *Reproduction) birthRequestDecision(view scheduler.SnapshotView, hour int, at sim.SimTime, proposer sim.EntityID) (kernel.Proposal, bool, error) {
	_, _, actors, err := reproductionWorldState(view)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	a, ok := actors[proposer]
	if !ok || !reproductionAlive(a) {
		return kernel.Proposal{}, false, ErrReproductionRunner
	}
	home, err := reproductionHomePatch(proposer, a)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	mates := make([]sim.EntityID, 0, len(actors))
	for _, id := range f.roster {
		if id == proposer {
			continue
		}
		mate, ok := actors[id]
		if !ok {
			return kernel.Proposal{}, false, ErrReproductionRunner
		}
		mateHome, err := reproductionHomePatch(id, mate)
		if err != nil {
			return kernel.Proposal{}, false, err
		}
		if mateHome == home {
			mates = append(mates, id)
		}
	}
	obs := strategy.ReproductionRequestObservation{
		Actor: proposer, Ref: f.ref, Hour: hour, WorldVersion: view.Version,
		HomePatch:    home,
		Energy:       integerObservation(a.Body.Energy),
		Capital:      integerObservation(a.Worksite.Capital),
		GranaryStock: integerObservation(a.Granary.Stock),
		OwnRequest:   strategy.ReproductionRequestGuard{Status: strategy.ReproductionRequestStatus(a.Request.Status), LastRequestHour: a.Request.LastRequestHour},
		PatchMates:   mates,
	}
	choice, err := f.bound[proposer].EvaluateBirthRequest(obs)
	if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
		return kernel.Proposal{}, false, ErrReproductionRunner
	}
	if choice.Kind != strategy.ReproductionRequestFile {
		return kernel.Proposal{}, false, nil
	}
	proposal, err := ReproductionBirthRequestProposal(reproductionKey("birth-request", at, proposer), hour, proposer, choice.Addressee, choice.ReportedCapital, choice.ReportedGranary, a)
	if err != nil {
		return kernel.Proposal{}, false, err
	}
	return proposal, true, nil
}

func integerObservation(n int64) sim.Value { return sim.IntegerValue(n) }

// birthReplyWake derives the consenter's reply wake from the committed
// request proposal: the addressee, if alive in this snapshot. A dead
// addressee never replies; the request expires at the next pulse.
func (f *Reproduction) birthReplyWake(view scheduler.SnapshotView, hour int, request kernel.Proposal) (scheduler.Wake, error) {
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthReply)
	if err != nil {
		return scheduler.Wake{}, err
	}
	var addressee sim.EntityID
	for _, patch := range request.Patches {
		if patch.Field == ReproductionBirthRequestAddresseeField {
			addressee, err = capacityRefValue(patch.Value)
			if err != nil {
				return scheduler.Wake{}, err
			}
		}
	}
	if addressee == 0 {
		return scheduler.Wake{}, ErrReproductionRunner
	}
	state, err := reproductionActorState(view, addressee)
	if err != nil {
		return scheduler.Wake{}, err
	}
	if !reproductionAlive(state) {
		return scheduler.Wake{}, nil
	}
	return scheduler.Wake{Actor: addressee, At: at, Cause: scheduler.WakeAudit}, nil
}

// reproductionBirthReply is the trusted reply-phase decision for one
// consenter at h+40m+6µs, factored for direct fixture testing: fresh full
// state, the derived pending views (claims plus the PUBLIC kin walk), the
// frozen policy cascade, then the contract's own consent + surplus-gate
// revalidation on truth. Nothing here mutates; the caller owns the returned
// proposal and evidence.
func reproductionBirthReply(view scheduler.SnapshotView, seed uint64, ref strategy.ReproductionRef, bound *strategy.ReproductionBound, hour int, consenter sim.EntityID) (kernel.Proposal, ReproductionAttempt, error) {
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthReply)
	if err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	_, _, actors, err := reproductionWorldState(view)
	if err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	self, ok := actors[consenter]
	if !ok {
		return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
	}
	births, err := reproductionBirthCount(view)
	if err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		a, ok := actors[id]
		if !ok {
			return 0, 0, false
		}
		return a.Genome.ParentA, a.Genome.ParentB, true
	})
	ids := reproductionSortedIDs(actors)
	var pending []strategy.ReproductionPendingView
	for _, id := range ids {
		if id == consenter {
			continue
		}
		r := actors[id].Request
		if r.Status != ReproductionRequestPending || r.Hour != int64(hour) || r.Addressee != consenter {
			continue
		}
		distance, related := ReproductionKinDistance(lookup, id, consenter)
		pending = append(pending, strategy.ReproductionPendingView{
			Requester: id, Addressee: consenter, Hour: hour,
			ReportedCapital: r.ReportedCapital, ReportedGranary: r.ReportedGranary,
			KinDistance: distance, KinRelated: related,
		})
	}
	obs := strategy.ReproductionReplyObservation{
		Actor: consenter, Ref: ref, Hour: hour, WorldVersion: view.Version,
		Energy:           integerObservation(self.Body.Energy),
		Capital:          integerObservation(self.Worksite.Capital),
		GranaryStock:     integerObservation(self.Granary.Stock),
		OwnLastReplyHour: self.Request.LastReplyHour,
		Pending:          pending,
	}
	choice, err := bound.EvaluateBirthReply(obs)
	if err != nil || choice.Ref != ref || choice.ObservedVersion != view.Version {
		return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
	}
	if choice.Kind == strategy.ReproductionReplyNone {
		return kernel.Proposal{}, ReproductionAttempt{}, nil
	}
	requester := choice.Requester
	proposer, ok := actors[requester]
	if !ok || requester == consenter {
		return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
	}
	distance, related := ReproductionKinDistance(lookup, requester, consenter)
	kinAllowed := !ReproductionKinProhibited(distance, related)
	var draws []uint64
	if choice.Kind == strategy.ReproductionReplyAccept {
		// The consent formula accepted on claims and the PUBLIC kin distance;
		// the runner's own kin walk must agree before any draw is spent. At
		// the frozen birth cap no draws are spent: the contract refuses with
		// the typed ReproductionRefusalBirthCap.
		if !kinAllowed {
			return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
		}
		if births < ReproductionMaxBirths {
			position, err := ReproductionBirthDrawPosition(births)
			if err != nil {
				return kernel.Proposal{}, ReproductionAttempt{}, err
			}
			stream := sim.NewRandomStream(sim.RandomState{Seed: seed, Stream: ReproductionBirthStream, Position: position})
			draws = make([]uint64, ReproductionDrawsPerBirth)
			for i := range draws {
				draws[i] = stream.Uint64()
			}
		}
	}
	result, err := ReproductionBirthReply(hour, births, kinAllowed, draws, requester, consenter, proposer, self)
	if err != nil {
		return kernel.Proposal{}, ReproductionAttempt{}, err
	}
	// Decision-level agreement: the policy may accept while the runner-side
	// fresh-truth gate refuses (BirthCap, CrossPatch, GateThresholds), but a
	// consent-phase disagreement is a linkage break, not typed evidence.
	switch choice.Kind {
	case strategy.ReproductionReplyAccept:
		switch result.Decision {
		case ReproductionReplyAccept:
		case ReproductionReplyRefuse:
			switch result.Reason {
			case ReproductionRefusalBirthCap, ReproductionRefusalCrossPatch, ReproductionRefusalGateThresholds:
			default:
				return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
			}
		case ReproductionReplyRefuseSilent:
			// The frozen death-between-phases refusal: the policy saw claims
			// and public kinship only; fresh truth found a dead parent.
			switch result.Reason {
			case ReproductionRefusalProposerDead, ReproductionRefusalAddresseeDead:
			default:
				return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
			}
		default:
			return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
		}
	case strategy.ReproductionReplyRefuse:
		if result.Decision != ReproductionReplyRefuse {
			return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
		}
		switch choice.Reason {
		case strategy.ReproductionReplyReasonConsentOwn:
			if result.Reason != ReproductionRefusalConsentOwn {
				return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
			}
		case strategy.ReproductionReplyReasonConsentClaims:
			if result.Reason != ReproductionRefusalConsentClaims {
				return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
			}
		case strategy.ReproductionReplyReasonKin:
			if result.Reason != ReproductionRefusalKin || kinAllowed {
				return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
			}
		default:
			return kernel.Proposal{}, ReproductionAttempt{}, ErrReproductionRunner
		}
	}
	var proposal kernel.Proposal
	if result.Decision != ReproductionReplyRefuseSilent {
		// A zero-mutation refusal (death between phases) has no proposal at
		// all; accept and refuse commit their typed proposals.
		var err error
		proposal, err = ReproductionBirthReplyProposal(reproductionKey("birth-reply", at, consenter), hour, requester, consenter, result)
		if err != nil {
			return kernel.Proposal{}, ReproductionAttempt{}, err
		}
	}
	request := proposer.Request
	attempt := ReproductionAttempt{
		Actor: consenter, Time: at, Kind: "birth-reply",
		Birth: ReproductionBirthEvidence{
			Decision:        result.Decision,
			Reason:          result.Reason,
			ReportedCapital: request.ReportedCapital, ReportedGranary: request.ReportedGranary,
			TrueCapital: proposer.Worksite.Capital, TrueGranary: proposer.Granary.Stock,
			Diverged: request.ReportedCapital != proposer.Worksite.Capital || request.ReportedGranary != proposer.Granary.Stock,
			Newborn:  result.NewbornID,
		},
	}
	if proposal.Key != "" {
		attempt.Key = proposal.Key
	}
	return proposal, attempt, nil
}

// reproductionAllocateGathers admits newborn gather claims with the v1
// rotation discipline generalized to birth-ordinal positions within the
// home patch: the rotation start is (seed+hour+patch) mod 8, eligible
// newborns rotate by (id−10001) mod 8 with ascending-ID tie-break, and the
// slot order rotates the same way as v1. Founder claims are admitted by the
// unchanged FoodFlowAllocateGathers; the runner sequences the two calls so
// the newborn competition only sees the slots the founders did not take,
// which keeps founder admissions byte-identical to v3. Pure and
// order-independent, like the v1 allocator.
func reproductionAllocateGathers(hour int, seed uint64, stocked []FoodFlowStockedSlot, claims []FoodFlowGatherClaim) ([]FoodFlowGatherAdmission, error) {
	if len(stocked) > ReproductionPatchCount*ReproductionSlotsPerPatch || len(claims) > ReproductionMaxBirths {
		return nil, ErrFoodFlowAllocationLimit
	}
	if _, err := ReproductionPhaseTime(hour, ReproductionPhaseClaim); err != nil {
		return nil, ErrReproductionContract
	}
	var slots [ReproductionPatchCount][]sim.EntityID
	seenSlots := make(map[sim.EntityID]bool, len(stocked))
	for _, slot := range stocked {
		index := int(slot.Patch) - 1001
		if index < 0 || index >= ReproductionPatchCount || seenSlots[slot.ID] || !reproductionSlotBelongsToPatch(slot.ID, index) {
			return nil, ErrReproductionContract
		}
		seenSlots[slot.ID] = true
		slots[index] = append(slots[index], slot.ID)
	}
	for i := range slots {
		sort.Slice(slots[i], func(a, b int) bool { return slots[i][a] < slots[i][b] })
	}
	ordered := append([]FoodFlowGatherClaim(nil), claims...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.Actor != b.Actor {
			return a.Actor < b.Actor
		}
		if a.TargetPatch != b.TargetPatch {
			return a.TargetPatch < b.TargetPatch
		}
		if a.TargetSlot != b.TargetSlot {
			return a.TargetSlot < b.TargetSlot
		}
		if a.Energy != b.Energy {
			return a.Energy < b.Energy
		}
		if a.BagUnits != b.BagUnits {
			return a.BagUnits < b.BagUnits
		}
		return a.LastGatherHour < b.LastGatherHour
	})
	out := make([]FoodFlowGatherAdmission, len(ordered))
	var eligible [ReproductionPatchCount][]int
	for i, claim := range ordered {
		out[i] = FoodFlowGatherAdmission{Actor: claim.Actor, Patch: claim.TargetPatch}
		if (i > 0 && ordered[i-1].Actor == claim.Actor) || (i+1 < len(ordered) && ordered[i+1].Actor == claim.Actor) {
			out[i].Rejection = FoodFlowGatherDuplicateActor
			continue
		}
		index := int(claim.TargetPatch) - 1001
		if !reproductionActorEntity(claim.Actor) || reproductionFounderEntity(claim.Actor) ||
			index < 0 || index >= ReproductionPatchCount || !reproductionSlotBelongsToPatch(claim.TargetSlot, index) {
			out[i].Rejection = FoodFlowGatherNoAccess
			continue
		}
		if claim.Energy <= 0 || claim.Energy > ReproductionEnergyCapacity || claim.BagUnits != 0 || claim.LastGatherHour < ReproductionNeverHour || claim.LastGatherHour >= int64(hour) {
			out[i].Rejection = FoodFlowGatherIneligible
			continue
		}
		eligible[index] = append(eligible[index], i)
	}
	for patch := range eligible {
		start := (seed + uint64(hour) + uint64(patch)) % ReproductionSlotsPerPatch
		sort.Slice(eligible[patch], func(a, b int) bool {
			posA := uint64((ordered[eligible[patch][a]].Actor - ReproductionFirstNewbornID) % ReproductionSlotsPerPatch)
			posB := uint64((ordered[eligible[patch][b]].Actor - ReproductionFirstNewbornID) % ReproductionSlotsPerPatch)
			left := (posA + ReproductionSlotsPerPatch - start) % ReproductionSlotsPerPatch
			right := (posB + ReproductionSlotsPerPatch - start) % ReproductionSlotsPerPatch
			if left != right {
				return left < right
			}
			return ordered[eligible[patch][a]].Actor < ordered[eligible[patch][b]].Actor
		})
		available := slots[patch]
		for rank, i := range eligible[patch] {
			switch {
			case len(available) == 0:
				out[i].Rejection = FoodFlowGatherNoStock
			case rank >= len(available):
				out[i].Rejection = FoodFlowGatherCapacity
			default:
				position := (int((seed+2*uint64(hour)+uint64(patch))%uint64(len(available))) + rank) % len(available)
				out[i].Slot = available[position]
			}
		}
	}
	return out, nil
}

func reproductionSlotBelongsToPatch(slot sim.EntityID, patchIndex int) bool {
	first := sim.EntityID(2001 + patchIndex*ReproductionSlotsPerPatch)
	return slot >= first && slot < first+ReproductionSlotsPerPatch
}

// allocate precomputes the claim-step gather admissions with the unchanged
// v1 allocator for the founders and the ordinal rotation for newborns. The
// claim commits the atomic gather at h+1µs, so contention must be resolved
// before the claim step: the coordinator evaluates each waking actor's
// frozen policy choice on the same authoritative head the workers will see
// (pure, deterministic), then admits through the two allocators in sequence.
func (f *Reproduction) allocate(at sim.SimTime) error {
	f.admissions = make(map[sim.EntityID]FoodFlowGatherAdmission)
	h := int(at / sim.SimTime(ReproductionHour))
	claim, err := ReproductionPhaseTime(h, ReproductionPhaseClaim)
	if err != nil || at != claim {
		return nil
	}
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	_, slots, actors, err := reproductionWorldState(view)
	if err != nil {
		return err
	}
	var stocked []FoodFlowStockedSlot
	for i := range slots {
		patch, _ := ReproductionPatchID(i)
		for j := range slots[i] {
			if slots[i][j].Stock == 1 {
				slot, _ := ReproductionSlotID(i, j)
				stocked = append(stocked, FoodFlowStockedSlot{ID: slot, Patch: patch})
			}
		}
	}
	var founderClaims, newbornClaims []FoodFlowGatherClaim
	for _, wake := range f.sched.Snapshot().Wakes {
		if wake.At != at || (wake.Cause != scheduler.WakeNeedThreshold && wake.Cause != scheduler.WakeBirth) {
			continue
		}
		id := wake.Actor
		if !f.rosterContains(id) {
			return ErrReproductionRunner
		}
		a, ok := actors[id]
		if !ok || !reproductionAlive(a) || a.Bag.Units != 0 {
			return ErrReproductionRunner
		}
		obs, err := f.observe(view, id, h, strategy.ReproductionNoDenial)
		if err != nil {
			return err
		}
		choice, err := f.bound[id].Evaluate(obs)
		if err != nil || choice.Ref != f.ref || choice.ObservedVersion != view.Version {
			return ErrReproductionRunner
		}
		claim := FoodFlowGatherClaim{Actor: id, TargetPatch: a.Bag.Source, TargetSlot: choice.TargetSlot, Energy: a.Body.Energy, BagUnits: a.Bag.Units, LastGatherHour: a.Body.LastGatherHour}
		switch choice.Kind {
		case strategy.ReproductionGather:
			if choice.TargetSlot == 0 {
				return ErrReproductionRunner
			}
			if reproductionFounderEntity(id) {
				founderClaims = append(founderClaims, claim)
			} else {
				newbornClaims = append(newbornClaims, claim)
			}
		case strategy.ReproductionEatStored, strategy.ReproductionWait:
			// No allocation needed; the worker re-evaluates and commits.
		default:
			return ErrReproductionRunner
		}
	}
	if len(founderClaims) > 0 {
		admitted, err := FoodFlowAllocateGathers(h, f.seed, stocked, founderClaims)
		if err != nil {
			return err
		}
		for _, a := range admitted {
			f.admissions[a.Actor] = a
			if a.Slot != 0 {
				for i, s := range stocked {
					if s.ID == a.Slot {
						stocked = append(stocked[:i], stocked[i+1:]...)
						break
					}
				}
			}
		}
	}
	if len(newbornClaims) > 0 {
		admitted, err := reproductionAllocateGathers(h, f.seed, stocked, newbornClaims)
		if err != nil {
			return err
		}
		for _, a := range admitted {
			f.admissions[a.Actor] = a
		}
	}
	return nil
}

// reproductionLinkAttempt binds a keyed attempt to its committed event. The
// one legitimate keyless outcome is a birth that lost its kernel batch to a
// newborn-ID collision: typed evidence, zero mutation. Anything else
// unlinked is a linkage break.
func reproductionLinkAttempt(a *ReproductionAttempt, ids map[string]sim.EventID) error {
	if a.Key == "" {
		return nil
	}
	if id, ok := ids[a.Key]; ok {
		a.EventID, a.Accepted = id, true
		return nil
	}
	if a.Kind == "birth-reply" && a.Birth.Decision == ReproductionReplyAccept {
		a.Lost = reproductionLostCollision
		return nil
	}
	return ErrReproductionRunner
}

// Step closes one timestamp; the frozen 168h horizon takes at most
// reproductionStepLimit steps. The commit head is guarded by the scheduler's
// stale-world check and the kernel's CommitBatchAtHead. Newborn fibers are
// registered at the quiescent boundary after a birth commits, with the
// first wake reserved at the next claim as the WakeBirth cause.
func (f *Reproduction) Step(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.steps >= reproductionStepLimit {
		return false, ErrReproductionRunner
	}
	snap := f.sched.Snapshot()
	if len(snap.Wakes) == 0 {
		return false, nil
	}
	at := snap.Wakes[0].At
	hour := sim.SimTime(ReproductionHour)
	if at%hour == 0 {
		// Quiescent pulse boundary: every roster fiber exists, is idle, and
		// its lifecycle matches the frozen alive invariant; the clock fiber
		// completes the set. Fibers are keyed by actor ID — the scheduler
		// sorts by ID, so the clock (5002) interleaves with newborn IDs.
		if len(snap.Fibers) != len(f.roster)+1 || len(f.meals) != 0 {
			return false, ErrReproductionRunner
		}
		byActor := make(map[sim.EntityID]scheduler.Fiber, len(snap.Fibers))
		for _, fiber := range snap.Fibers {
			byActor[fiber.Actor] = fiber
		}
		if _, ok := byActor[reproductionClockActor]; !ok {
			return false, ErrReproductionRunner
		}
		for _, id := range f.roster {
			fiber, ok := byActor[id]
			if !ok || fiber.Activity != nil {
				return false, ErrReproductionRunner
			}
			f.pulseLifecycle[id] = fiber.Lifecycle
		}
	}
	if err := f.allocate(at); err != nil {
		return false, err
	}
	f.pending = &reproductionPending{started: make(map[sim.EntityID]strategy.ReproductionAction), meals: make(map[sim.EntityID]ReproductionMealIntent)}
	defer func() { f.pending = nil }()
	events, processed, err := f.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	// Newborn registration at the quiescent boundary, post-commit: allocate
	// the fiber, bind the frozen policy, and reserve the first wake at the
	// next claim with the WakeBirth cause (never within the horizon's final
	// hour, where no claim exists).
	h := int(at / hour)
	for _, ev := range events {
		if ev.Kind != kernel.KindEntityCreate || ev.Rule != ReproductionBirthReplyRule {
			continue
		}
		if err := f.registerNewborn(ev, h); err != nil {
			return false, err
		}
	}
	ids := make(map[string]sim.EventID, len(events))
	for _, ev := range events {
		ids[ev.Key] = ev.ID
	}
	head := f.k.SnapshotHead()
	batch := ReproductionBatch{Time: at, Version: head.Version, TipID: head.TipID, TipHash: head.TipHash}
	for _, a := range f.pending.attempts {
		if err := reproductionLinkAttempt(&a, ids); err != nil {
			return false, err
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
			return false, ErrReproductionRunner
		}
		delete(f.current, id)
	}
	for id, kind := range f.pending.started {
		fiber, ok := f.sched.Fiber(id)
		if !ok || fiber.Activity == nil {
			return false, ErrReproductionRunner
		}
		f.current[id] = kind
	}
	for _, id := range f.pending.denied {
		f.denied[id] = int64(at / hour)
	}
	for id, intent := range f.pending.meals {
		f.meals[id] = intent
	}
	for _, id := range f.pending.mealsCleared {
		delete(f.meals, id)
	}
	for _, id := range f.pending.birthWakesUsed {
		if _, ok := f.birthWakes[id]; !ok {
			return false, ErrReproductionRunner
		}
		delete(f.birthWakes, id)
	}
	for _, d := range f.pending.deaths {
		switch d.Cause {
		case "age":
			f.deathsAge++
		case "starvation":
			f.deathsStarve++
		default:
			return false, ErrReproductionRunner
		}
		fiber, ok := f.sched.Fiber(d.Actor)
		if !ok || fiber.Lifecycle != scheduler.Stopped {
			return false, ErrReproductionRunner // every recorded death stopped its fiber
		}
		f.pulseLifecycle[d.Actor] = scheduler.Stopped
	}
	f.birthFiles += int64(f.pending.birthFiles)
	f.divergences += int64(f.pending.divergedReplies)
	for _, reason := range f.pending.replyReasons {
		f.refusals[reason]++
	}
	f.steps++
	f.journal = append(f.journal, batch)
	if at%hour == 0 {
		check, err := f.checkpoint(int(at / hour))
		if err != nil {
			return false, fmt.Errorf("reproduction hour %d: %w", at/hour, err)
		}
		f.checkpoints = append(f.checkpoints, check)
	}
	return true, nil
}

// registerNewborn allocates the newborn's fiber from a committed
// entity-create event: the newborn ID is the allocated genome row, the
// frozen policy is bound fresh (heredity rides the genome, not the ref),
// and the first wake is reserved at the next claim as WakeBirth.
func (f *Reproduction) registerNewborn(ev kernel.Event, hour int) error {
	newborn := sim.EntityID(0)
	for _, d := range ev.Deltas {
		if d.Component == ReproductionGenomeTypeID && d.Entity >= sim.EntityID(ReproductionFirstNewbornID) {
			if newborn != 0 && d.Entity != newborn {
				return ErrReproductionRunner
			}
			newborn = d.Entity
		}
	}
	if !reproductionActorEntity(newborn) || reproductionFounderEntity(newborn) || f.rosterContains(newborn) {
		return ErrReproductionRunner
	}
	if _, exists := f.sched.Fiber(newborn); exists {
		return ErrReproductionRunner
	}
	bound, err := f.policies.Bind(strategy.ReproductionBinding{Actor: newborn, Ref: f.ref})
	if err != nil {
		return err
	}
	if err = f.sched.Register(newborn); err != nil {
		return err
	}
	f.bound[newborn] = bound
	f.roster = append(f.roster, newborn)
	f.newborns = append(f.newborns, newborn)
	f.pulseLifecycle[newborn] = scheduler.Alive
	if hour+1 <= ReproductionHorizonHours-1 {
		claim, err := ReproductionPhaseTime(hour+1, ReproductionPhaseClaim)
		if err != nil {
			return err
		}
		if err = f.sched.Schedule(scheduler.Wake{Actor: newborn, At: claim, Cause: scheduler.WakeBirth}); err != nil {
			return err
		}
		f.birthWakes[newborn] = int64(hour + 1)
	}
	return nil
}

func (f *Reproduction) checkpoint(h int) (ReproductionCheckpoint, error) {
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	patches, slots, actors, err := reproductionWorldState(view)
	if err != nil {
		return ReproductionCheckpoint{}, err
	}
	if len(actors) != len(f.roster) {
		return ReproductionCheckpoint{}, ErrReproductionRunner
	}
	var founders [ReproductionFounderCount]ReproductionActorState
	newborns := make([]ReproductionNewbornState, 0, len(f.newborns))
	for i := range founders {
		a, ok := actors[sim.EntityID(i+1)]
		if !ok {
			return ReproductionCheckpoint{}, ErrReproductionRunner
		}
		founders[i] = a
	}
	for _, id := range f.newborns {
		a, ok := actors[id]
		if !ok {
			return ReproductionCheckpoint{}, ErrReproductionRunner
		}
		newborns = append(newborns, ReproductionNewbornState{ID: id, State: a})
	}
	// Gates G1/G2a/G2b/G3 (extended) at every hour boundary; every row of
	// every actor is revalidated inside.
	balance, err := ReproductionCheckConservation(patches, slots, founders, newborns)
	if err != nil {
		return ReproductionCheckpoint{}, err
	}
	alive := 0
	for _, id := range f.roster {
		if reproductionAlive(actors[id]) {
			alive++
		}
		if (f.pulseLifecycle[id] == scheduler.Alive) != reproductionAlive(actors[id]) {
			return ReproductionCheckpoint{}, ErrReproductionRunner
		}
	}
	metrics := f.metrics(founders, newborns, balance)
	metrics.Alive = alive
	return ReproductionCheckpoint{Hour: h, Version: head.Version, Balance: balance, Patches: patches, Slots: slots, Founders: founders, Newborns: newborns, Metrics: metrics}, nil
}

// metrics computes the frozen v4 pilot metrics over the living roster:
// cause-attributed deaths (cumulative), birth economics (filings, typed
// refusal histogram, claim-vs-truth divergences), genome diversity (locus
// frequencies, distinct genomes, integer-exact mean pairwise L1), and
// kinship density (related living pairs within the ≤10-ref walk).
func (f *Reproduction) metrics(founders [ReproductionFounderCount]ReproductionActorState, newborns []ReproductionNewbornState, balance ReproductionBalance) ReproductionMetrics {
	m := ReproductionMetrics{
		DeathsAge: f.deathsAge, DeathsStarvation: f.deathsStarve,
		Births: balance.Births, BirthRequests: f.birthFiles,
		Refusals:    make(map[ReproductionRefusalReason]int64, len(f.refusals)),
		Divergences: f.divergences,
	}
	for reason, n := range f.refusals {
		m.Refusals[reason] = n
	}
	type livingEntry struct {
		id    sim.EntityID
		state ReproductionActorState
	}
	var living []livingEntry
	for i := range founders {
		if reproductionAlive(founders[i]) {
			living = append(living, livingEntry{id: sim.EntityID(i + 1), state: founders[i]})
		}
	}
	for _, newborn := range newborns {
		if reproductionAlive(newborn.State) {
			living = append(living, livingEntry{id: newborn.ID, state: newborn.State})
		}
	}
	alleleIndex := func(v int64) int { return int(v - ReproductionGenomeLocusMin) }
	distinct := make(map[[3]int64]struct{}, len(living))
	for _, entry := range living {
		m.LocusFrequencies[0][alleleIndex(entry.state.Genome.LocusM)]++
		m.LocusFrequencies[1][alleleIndex(entry.state.Genome.LocusY)]++
		m.LocusFrequencies[2][alleleIndex(entry.state.Genome.LocusW)]++
		distinct[[3]int64{entry.state.Genome.LocusM, entry.state.Genome.LocusY, entry.state.Genome.LocusW}] = struct{}{}
	}
	m.DistinctGenomes = len(distinct)
	for i := 0; i < len(living); i++ {
		for j := i + 1; j < len(living); j++ {
			m.SumPairwiseL1 += abs64(living[i].state.Genome.LocusM-living[j].state.Genome.LocusM) +
				abs64(living[i].state.Genome.LocusY-living[j].state.Genome.LocusY) +
				abs64(living[i].state.Genome.LocusW-living[j].state.Genome.LocusW)
		}
	}
	m.GenomePairs = int64(len(living) * (len(living) - 1) / 2)
	m.LivingPairs = m.GenomePairs
	if len(living) > 0 {
		lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
			if reproductionFounderEntity(id) && id >= 1 && id <= ReproductionFounderCount {
				g := founders[id-1].Genome
				return g.ParentA, g.ParentB, true
			}
			for _, newborn := range newborns {
				if newborn.ID == id {
					return newborn.State.Genome.ParentA, newborn.State.Genome.ParentB, true
				}
			}
			return 0, 0, false
		})
		for i := 0; i < len(living); i++ {
			for j := i + 1; j < len(living); j++ {
				if _, related := ReproductionKinDistance(lookup, living[i].id, living[j].id); related {
					m.KinPairs++
				}
			}
		}
	}
	return m
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (f *Reproduction) Run(ctx context.Context) error {
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
	if len(checks) != ReproductionHorizonHours+1 || checks[len(checks)-1].Hour != ReproductionHorizonHours {
		return ErrReproductionRunner
	}
	return nil
}

func (f *Reproduction) SchedulerSnapshot() scheduler.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sched.Snapshot()
}

func (f *Reproduction) Journal() []ReproductionBatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneReproductionJournal(f.journal)
}

func cloneReproductionJournal(batches []ReproductionBatch) []ReproductionBatch {
	out := make([]ReproductionBatch, len(batches))
	for i, b := range batches {
		out[i] = b
		out[i].Attempts = append([]ReproductionAttempt(nil), b.Attempts...)
	}
	return out
}

func (f *Reproduction) Checkpoints() []ReproductionCheckpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneReproductionCheckpoints(f.checkpoints)
}

func cloneReproductionCheckpoints(checks []ReproductionCheckpoint) []ReproductionCheckpoint {
	out := make([]ReproductionCheckpoint, len(checks))
	for i := range checks {
		out[i] = checks[i]
		out[i].Newborns = append([]ReproductionNewbornState(nil), checks[i].Newborns...)
		out[i].Metrics.Refusals = make(map[ReproductionRefusalReason]int64, len(checks[i].Metrics.Refusals))
		for reason, n := range checks[i].Metrics.Refusals {
			out[i].Metrics.Refusals[reason] = n
		}
	}
	return out
}

// Handoff captures the portable kernel and scheduler bytes plus runner-only
// state under the coordinator boundary. Encoding, authentication, and restore
// belong to the persistence lane, not this runner.
func (f *Reproduction) Handoff() (ReproductionHandoff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return ReproductionHandoff{}, err
	}
	portable, schedHead, err := f.sched.ExportPortable()
	if err != nil {
		return ReproductionHandoff{}, err
	}
	if head != schedHead {
		return ReproductionHandoff{}, ErrReproductionRunner
	}
	out := ReproductionHandoff{FormatVersion: ReproductionFormatVersion, Yield: f.yield, Seed: f.seed, Births: f.births,
		Policy: f.ref, Steps: f.steps, History: history, SchedulerBytes: portable, Head: head,
		Newborns:   append([]sim.EntityID(nil), f.newborns...),
		BirthWakes: make(map[sim.EntityID]int64, len(f.birthWakes)),
		Current:    make(map[sim.EntityID]strategy.ReproductionAction, len(f.current)),
		Denied:     make(map[sim.EntityID]int64, len(f.denied)),
		Meals:      make(map[sim.EntityID]ReproductionMealIntent, len(f.meals)),
		Journal:    cloneReproductionJournal(f.journal), Checkpoints: cloneReproductionCheckpoints(f.checkpoints)}
	for id, v := range f.birthWakes {
		out.BirthWakes[id] = v
	}
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
