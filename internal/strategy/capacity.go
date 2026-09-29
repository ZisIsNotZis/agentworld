package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
	"sync"
)

// Capacity v3 policy lane (ticket 13). The frozen design in
// .scratch/13-productive-capacity/spec.md (including the contract-lane
// corrigenda) is binding: every threshold below is quoted from it and must
// stay identical to the trusted world contract in
// internal/world/capacity_contract*.go. A choice is one intention at one
// frozen clock instant; the runner owns phase placement, build-completion
// pairing, and admission, and the world rechecks state and version.
const (
	CapacityPolicyFormatV3 uint32 = 3
	CapacityMaxActors             = 16
	CapacityMaxCandidates         = 16

	capacityHorizonHours  = 168
	capacitySlotsPerPatch = 8

	capacityEnergyCapacity int64 = 12
	capacityHungerCapacity int64 = 24
	capacityBagCapacity    int64 = 1
	capacityKMax           int64 = 8
	capacityWipMax         int64 = 1
	capacityGranaryMax     int64 = 8
	// Frozen CapacityPolicyTLow: the bag decision builds iff energy >= 5 or
	// the granary holds a fallback meal, and never with k+wip >= 8.
	capacityPolicyTLow int64 = 5
	capacityNeverHour  int64 = -1

	// The ten-minute rest of a denied gatherer with an empty granary, the v1
	// rest window; it stays inside the frozen hour's remaining phases.
	CapacityRestDuration sim.Duration = 600 * 1e6
)

var (
	ErrInvalidCapacityPolicy      = errors.New("invalid capacity policy")
	ErrUnknownCapacityPolicy      = errors.New("unknown capacity policy reference")
	ErrInvalidCapacityBinding     = errors.New("invalid capacity actor binding")
	ErrInvalidCapacityObservation = errors.New("invalid capacity observation")
)

type CapacityRef struct {
	ID      string
	Version uint32
}

type CapacityBudget struct{ Candidates, Evaluations int }

// CapacityPolicy is pinned value-only content: the frozen pilot is exactly the
// {capacity,3} reference with budget 16/16. The decision weights and rules
// (TLow=5, granary fallback, k+wip<8, gather-before-stored priority) are
// frozen code in Evaluate, so they cannot drift at registration; the declared
// identity and format are compared against this constructor, and the budget is
// bounded by its declared ceiling. Any same-ref budget variant that still
// validates here fails later at restore: VerifyCapacityPolicy compares the
// complete wire content against this executable's frozen policy.
type CapacityPolicy struct {
	FormatVersion uint32
	Ref           CapacityRef
	Budget        CapacityBudget
}

// FrozenCapacityPolicy constructs the declared v3 pilot, not a tunable family.
func FrozenCapacityPolicy() CapacityPolicy {
	return CapacityPolicy{
		FormatVersion: CapacityPolicyFormatV3,
		Ref:           CapacityRef{ID: "capacity", Version: 3},
		Budget:        CapacityBudget{Candidates: CapacityMaxCandidates, Evaluations: CapacityMaxCandidates},
	}
}

func validCapacityRef(ref CapacityRef) bool {
	return ref.ID != "" && len(ref.ID) <= MaxRefIDBytes && ref.Version != 0
}

func validCapacityPolicy(p CapacityPolicy) bool {
	frozen := FrozenCapacityPolicy()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

type CapacityBinding struct {
	Actor sim.EntityID
	Ref   CapacityRef
}

type CapacityRegistry struct {
	mu       sync.RWMutex
	policies map[CapacityRef]CapacityPolicy
}

func NewCapacityRegistry() *CapacityRegistry {
	return &CapacityRegistry{policies: make(map[CapacityRef]CapacityPolicy)}
}

func (r *CapacityRegistry) Register(policy CapacityPolicy) error {
	if r == nil || !validCapacityPolicy(policy) {
		return ErrInvalidCapacityPolicy
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policies == nil {
		return ErrInvalidCapacityPolicy
	}
	if _, exists := r.policies[policy.Ref]; exists {
		return ErrInvalidCapacityPolicy
	}
	r.policies[policy.Ref] = policy // value-only; no caller-owned slices or maps
	return nil
}

// Bind pins one actor of the fixed 16-actor roster to the shared frozen
// policy. No actor owns or can mutate the registry's policy.
func (r *CapacityRegistry) Bind(binding CapacityBinding) (*CapacityBound, error) {
	if r == nil || binding.Actor < 1 || binding.Actor > CapacityMaxActors || !validCapacityRef(binding.Ref) {
		return nil, ErrInvalidCapacityBinding
	}
	r.mu.RLock()
	policy, ok := r.policies[binding.Ref]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownCapacityPolicy
	}
	return &CapacityBound{actor: binding.Actor, policy: policy}, nil
}

type CapacityBound struct {
	actor  sim.EntityID
	policy CapacityPolicy
}

func (b *CapacityBound) Binding() CapacityBinding {
	if b == nil {
		return CapacityBinding{}
	}
	return CapacityBinding{Actor: b.actor, Ref: b.policy.Ref}
}

// Policy returns the value-only frozen content; it carries no per-actor state.
func (b *CapacityBound) Policy() CapacityPolicy {
	if b == nil {
		return CapacityPolicy{}
	}
	return b.policy
}

// CapacitySlotView is one open-access home-patch slot's public stock hint.
// Stock is never a reservation or physical proof; the world validates gather.
type CapacitySlotView struct {
	ID    sim.EntityID
	Stock sim.Value
}

// CapacityDenial is supplied only after the actor's own Gather was rejected.
type CapacityDenial uint8

const (
	CapacityNoDenial     CapacityDenial = iota
	CapacityGatherDenied                // the actor's own claim-phase gather was rejected
)

// CapacityObservation is the actor-visible v3 view. By construction it can
// carry only the owner's own private rows (energy, hunger, bag, worksite,
// granary), the public home-patch slot stocks, and the ledger guards the
// frozen privacy list allows. Others' capital, granaries, energy, meals,
// unrealized counters, rotation internals, and the seed have no field here at
// all. The runner must build it from actor-readable components only.
type CapacityObservation struct {
	Actor              sim.EntityID
	Ref                CapacityRef
	Hour               int
	WorldVersion       sim.WorldVersion
	Energy             sim.Value
	Hunger             sim.Value
	BagUnits           sim.Value
	BagSource          sim.Value // Missing EntityRef when the bag is empty
	Capital            sim.Value
	Wip                sim.Value
	GranaryStock       sim.Value
	InvestedUnits      sim.Value
	LastBuildHour      sim.Value // -1 until the first committed build
	LastStoredMealHour sim.Value // -1 until the first stored meal
	LastDenial         CapacityDenial
	HomeSlots          []CapacitySlotView
}

type CapacityAction uint8

const (
	CapacityEat       CapacityAction = iota + 1 // eat the held wild unit
	CapacityGather                              // claim one visible home slot
	CapacityEatStored                           // draw the only permitted granary meal
	CapacityBuild                               // invest the held unit into the worksite
	CapacityRest                                // ten-minute rest after a denied gather
	CapacityWait                                // a fallback only, never a planned action
)

type CapacityFallback uint8

const (
	CapacityNoFallback CapacityFallback = iota
	CapacityNoCandidate
	CapacityBudgetExhausted
	CapacityInvalidSelf
)

// TargetSlot identifies the visible Gather affordance; every other action
// targets the actor's own rows implicitly. RestDuration is nonzero only for
// Rest. The runner places the intention at the frozen clock instant.
type CapacityChoice struct {
	Kind            CapacityAction
	TargetSlot      sim.EntityID
	RestDuration    sim.Duration
	ObservedVersion sim.WorldVersion
	Ref             CapacityRef
	Evaluated       int
	Fallback        CapacityFallback
}

func capacityInteger(value sim.Value, minimum, maximum int64) (int64, bool) {
	if value.Kind() == sim.IntegerKind {
		n, err := value.Integer()
		return n, err == nil && n >= minimum && n <= maximum
	}
	// Direct integer projections may arrive as exact scalar metrics.
	if value.Kind() == sim.ScalarKind {
		n, err := value.Scalar()
		return int64(n), err == nil && n >= float64(minimum) && n <= float64(maximum) && n == float64(int64(n))
	}
	return 0, false
}

// capacityHomePatch is the fixed v3 roster geometry: actors 1..8 share patch
// 1001, actors 9..16 share patch 1002 (world.CapacityActorPatchID).
func capacityHomePatch(actor sim.EntityID) sim.EntityID {
	return sim.EntityID(1001 + (int(actor)-1)/capacitySlotsPerPatch)
}

// capacityFirstSlot is the lowest slot ID of the home patch
// (world.CapacitySlotID(patchIndex, 0)).
func capacityFirstSlot(home sim.EntityID) sim.EntityID {
	return sim.EntityID(2001 + (int(home)-1001)*capacitySlotsPerPatch)
}

// Evaluate is the frozen hourly decision cascade, a pure function of one
// actor-readable observation:
//
//	bag decision (bag=1, h+20m+2µs): Build iff (energy>=5 or granary>=1) and
//	  k+wip<8, else Eat; the paired stored meal is drawn fresh at build
//	  completion and is the runner's duty, never decided here;
//	claim (bag=0, h+1µs): visible home stock -> Gather (lowest slot ID), else
//	  granary>=1 and no stored meal yet this hour -> EatStored, else Wait;
//	denied-gather re-evaluation (bag=0 after the actor's own gather was
//	  rejected): EatStored if granary>=1 and none drawn yet this hour, else
//	  Rest — a second gather in the same hour is never proposed.
//
// A stored meal is emitted at most once per actor-hour: every EatStored
// emission requires LastStoredMealHour < the observed hour, so the frozen
// stored-meals [0,168] ceiling can never be exhausted by a repeated
// evaluation at the claim, denial, or paired-meal slot of the same hour. The
// paired meal of a build is drawn by the runner at completion after fresh
// granary revalidation; this policy only ever reports the Build|Eat intent.
// The runner evaluates bag=0 observations only at the claim instant or with
// CapacityGatherDenied set, so no cascade path can double-claim. Out-of-range
// own values leave a Wait choice with CapacityInvalidSelf instead of an
// intention; structurally malformed observations are errors.
func (b *CapacityBound) Evaluate(obs CapacityObservation) (CapacityChoice, error) {
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= capacityHorizonHours || obs.LastDenial > CapacityGatherDenied ||
		len(obs.HomeSlots) > capacitySlotsPerPatch {
		return CapacityChoice{}, ErrInvalidCapacityObservation
	}
	choice := CapacityChoice{Kind: CapacityWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: CapacityNoCandidate}
	home := capacityHomePatch(obs.Actor)
	first := capacityFirstSlot(home)
	slots := append([]CapacitySlotView(nil), obs.HomeSlots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].ID < slots[j].ID })
	for i, view := range slots {
		if view.ID < first || view.ID >= first+capacitySlotsPerPatch || (i > 0 && slots[i-1].ID == view.ID) {
			return CapacityChoice{}, ErrInvalidCapacityObservation
		}
		if _, ok := capacityInteger(view.Stock, 0, 1); !ok {
			return CapacityChoice{}, ErrInvalidCapacityObservation
		}
	}
	energy, energyOK := capacityInteger(obs.Energy, 0, capacityEnergyCapacity)
	_, hungerOK := capacityInteger(obs.Hunger, 0, capacityHungerCapacity)
	bag, bagOK := capacityInteger(obs.BagUnits, 0, capacityBagCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, capacityKMax)
	wip, wipOK := capacityInteger(obs.Wip, 0, capacityWipMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, capacityGranaryMax)
	_, investedOK := capacityInteger(obs.InvestedUnits, 0, capacityHorizonHours)
	lastBuild, lastBuildOK := capacityInteger(obs.LastBuildHour, capacityNeverHour, capacityHorizonHours-1)
	lastMeal, lastMealOK := capacityInteger(obs.LastStoredMealHour, capacityNeverHour, capacityHorizonHours-1)
	if !energyOK || !hungerOK || !bagOK || !capitalOK || !wipOK || !granaryOK || !investedOK || !lastBuildOK || !lastMealOK ||
		lastBuild > int64(obs.Hour) || lastMeal > int64(obs.Hour) {
		choice.Fallback = CapacityInvalidSelf
		return choice, nil
	}
	// The bag is wild-only with home-patch provenance: full iff the source
	// names the actor's own patch, empty iff the source is Missing.
	if bag == 1 {
		source, err := obs.BagSource.EntityRef()
		if err != nil || source != home {
			choice.Fallback = CapacityInvalidSelf
			return choice, nil
		}
	} else if obs.BagSource.Kind() != sim.EntityRefKind || obs.BagSource.State() != sim.Missing {
		choice.Fallback = CapacityInvalidSelf
		return choice, nil
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if bag == 1 { // frozen bag decision: Build | Eat
		if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 {
			choice.Fallback = CapacityBudgetExhausted
			return choice, nil
		}
		if capital+wip < capacityKMax && (energy >= capacityPolicyTLow || granary >= 1) {
			choice.Kind, choice.Evaluated, choice.Fallback = CapacityBuild, 1, CapacityNoFallback
		} else {
			choice.Kind, choice.Evaluated, choice.Fallback = CapacityEat, 1, CapacityNoFallback
		}
		return choice, nil
	}
	if obs.LastDenial == CapacityGatherDenied { // denied-gather fallback
		if granary >= 1 && lastMeal < int64(obs.Hour) {
			choice.Kind, choice.Evaluated, choice.Fallback = CapacityEatStored, 1, CapacityNoFallback
		} else {
			choice.Kind, choice.RestDuration, choice.Evaluated, choice.Fallback = CapacityRest, CapacityRestDuration, 1, CapacityNoFallback
		}
		return choice, nil
	}
	candidates := make([]sim.EntityID, 0, len(slots))
	for _, view := range slots {
		if stock, ok := capacityInteger(view.Stock, 0, 1); ok && stock == 1 {
			candidates = append(candidates, view.ID)
		}
	}
	if len(candidates) > b.policy.Budget.Candidates || len(candidates) > b.policy.Budget.Evaluations {
		choice.Fallback = CapacityBudgetExhausted // all-or-nothing: never a sorted prefix
		return choice, nil
	}
	if len(candidates) > 0 {
		choice.Kind, choice.TargetSlot, choice.Evaluated, choice.Fallback = CapacityGather, candidates[0], len(candidates), CapacityNoFallback
		return choice, nil
	}
	if granary >= 1 && lastMeal < int64(obs.Hour) { // one stored meal per actor-hour at most
		choice.Kind, choice.Evaluated, choice.Fallback = CapacityEatStored, 1, CapacityNoFallback
	}
	return choice, nil
}
