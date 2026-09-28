package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
	"sync"
)

// Food-flow policy is separate from the finite-cache S6 policy and registry.
// Its v1 choice is a single intention, not a multi-step plan or an authority
// to mutate the world. The runner must recheck access, state and version.
const (
	FoodFlowPolicyFormatV1   uint32       = 1
	FoodFlowMaxPatches                    = 2
	FoodFlowMaxSlotsPerPatch              = 8
	FoodFlowMaxCandidates                 = FoodFlowMaxPatches * FoodFlowMaxSlotsPerPatch
	FoodFlowRestDuration     sim.Duration = 600 * 1e6 // 10 minutes in microseconds
	foodFlowHour             sim.Duration = 3_600_000_000
	foodFlowHorizonHours                  = 168
)

var (
	ErrInvalidFoodFlowPolicy      = errors.New("invalid food-flow policy")
	ErrUnknownFoodFlowPolicy      = errors.New("unknown food-flow policy reference")
	ErrInvalidFoodFlowBinding     = errors.New("invalid food-flow actor binding")
	ErrInvalidFoodFlowObservation = errors.New("invalid food-flow observation")
)

type FoodFlowRef struct {
	ID      string
	Version uint32
}

type FoodFlowBudget struct {
	Candidates  int
	Evaluations int
}

// The budget limits the whole candidate set; it never truncates a sorted
// prefix, which would otherwise privilege low slot IDs on overflow.
type FoodFlowPolicy struct {
	FormatVersion uint32
	Ref           FoodFlowRef
	Budget        FoodFlowBudget
	RestDuration  sim.Duration
}

type FoodFlowBinding struct {
	Actor sim.EntityID
	Ref   FoodFlowRef
}

type FoodFlowRegistry struct {
	mu       sync.RWMutex
	policies map[FoodFlowRef]*FoodFlowPolicy
}

func NewFoodFlowRegistry() *FoodFlowRegistry {
	return &FoodFlowRegistry{policies: make(map[FoodFlowRef]*FoodFlowPolicy)}
}

func validFoodFlowRef(ref FoodFlowRef) bool {
	return ref.ID != "" && len(ref.ID) <= MaxRefIDBytes && ref.Version != 0
}

func validFoodFlowPolicy(policy FoodFlowPolicy) bool {
	return policy.FormatVersion == FoodFlowPolicyFormatV1 && validFoodFlowRef(policy.Ref) &&
		policy.Budget.Candidates >= 1 && policy.Budget.Candidates <= FoodFlowMaxCandidates &&
		policy.Budget.Evaluations >= 1 && policy.Budget.Evaluations <= FoodFlowMaxCandidates &&
		policy.RestDuration == FoodFlowRestDuration
}

func (r *FoodFlowRegistry) Register(policy FoodFlowPolicy) error {
	if r == nil || !validFoodFlowPolicy(policy) {
		return ErrInvalidFoodFlowPolicy
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policies == nil {
		return ErrInvalidFoodFlowPolicy
	}
	if _, exists := r.policies[policy.Ref]; exists {
		return ErrInvalidFoodFlowPolicy
	}
	copy := policy
	r.policies[policy.Ref] = &copy
	return nil
}

// Bind pins one actor to a shared immutable policy snapshot. No actor owns or
// can mutate the registry's policy, and different actors may use the same ref.
func (r *FoodFlowRegistry) Bind(binding FoodFlowBinding) (*FoodFlowBound, error) {
	if r == nil || sim.ValidateEntityID(binding.Actor) != nil || !validFoodFlowRef(binding.Ref) {
		return nil, ErrInvalidFoodFlowBinding
	}
	r.mu.RLock()
	policy := r.policies[binding.Ref]
	r.mu.RUnlock()
	if policy == nil {
		return nil, ErrUnknownFoodFlowPolicy
	}
	return &FoodFlowBound{actor: binding.Actor, policy: policy}, nil
}

type FoodFlowBound struct {
	actor  sim.EntityID
	policy *FoodFlowPolicy
}

func (b *FoodFlowBound) Binding() FoodFlowBinding {
	if b == nil || b.policy == nil {
		return FoodFlowBinding{}
	}
	return FoodFlowBinding{Actor: b.actor, Ref: b.policy.Ref}
}

// Only accessible patches and their actor-visible slot affordances belong in
// this observation. Stock is a hint, never a reservation or physical proof.
type FoodFlowSlotView struct {
	ID    sim.EntityID
	Stock sim.Value
}

type FoodFlowPatchView struct {
	ID    sim.EntityID
	Slots []FoodFlowSlotView
}

type FoodFlowDenial uint8

const (
	FoodFlowNoDenial     FoodFlowDenial = iota
	FoodFlowGatherDenied                // supplied only after the actor's own Gather was rejected
)

type FoodFlowObservation struct {
	Actor          sim.EntityID
	Time           sim.SimTime
	WorldVersion   sim.WorldVersion
	Energy         sim.Value
	Hunger         sim.Value
	BagUnits       sim.Value
	BagSource      sim.Value // Missing EntityRef when the bag is empty
	LastGatherHour sim.Value // -1 until the first successful Gather
	Patches        []FoodFlowPatchView
	LastDenial     FoodFlowDenial
}

type FoodFlowAction uint8

const (
	FoodFlowEat FoodFlowAction = iota + 1
	FoodFlowGather
	FoodFlowRest
	FoodFlowWait
)

type FoodFlowFallback uint8

const (
	FoodFlowNoFallback FoodFlowFallback = iota
	FoodFlowNoCandidate
	FoodFlowBudgetExhausted
	FoodFlowInvalidSelf
)

// TargetPatch/TargetSlot identify a visible Gather affordance. Eat's patch
// is the bag's recorded source. RestDuration is nonzero only for Rest.
type FoodFlowChoice struct {
	Kind            FoodFlowAction
	TargetPatch     sim.EntityID
	TargetSlot      sim.EntityID
	RestDuration    sim.Duration
	ObservedVersion sim.WorldVersion
	Ref             FoodFlowRef
	Evaluated       int
	Fallback        FoodFlowFallback
}

func foodFlowInteger(value sim.Value, minimum, maximum int64) (int64, bool) {
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

func (b *FoodFlowBound) Evaluate(obs FoodFlowObservation) (FoodFlowChoice, error) {
	if b == nil || b.policy == nil || obs.Actor != b.actor || obs.Time < 0 ||
		obs.Time > sim.SimTime(foodFlowHorizonHours)*sim.SimTime(foodFlowHour) ||
		len(obs.Patches) > FoodFlowMaxPatches || obs.LastDenial > FoodFlowGatherDenied {
		return FoodFlowChoice{}, ErrInvalidFoodFlowObservation
	}
	choice := FoodFlowChoice{Kind: FoodFlowWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: FoodFlowNoCandidate}
	patches := append([]FoodFlowPatchView(nil), obs.Patches...)
	sort.Slice(patches, func(i, j int) bool { return patches[i].ID < patches[j].ID })
	seenSlots := make(map[sim.EntityID]bool, FoodFlowMaxCandidates)
	for i, patch := range patches {
		if sim.ValidateEntityID(patch.ID) != nil || (i > 0 && patches[i-1].ID == patch.ID) || len(patch.Slots) > FoodFlowMaxSlotsPerPatch {
			return FoodFlowChoice{}, ErrInvalidFoodFlowObservation
		}
		for _, slot := range patch.Slots {
			if sim.ValidateEntityID(slot.ID) != nil || seenSlots[slot.ID] {
				return FoodFlowChoice{}, ErrInvalidFoodFlowObservation
			}
			seenSlots[slot.ID] = true
		}
	}
	energy, energyOK := foodFlowInteger(obs.Energy, 0, 12)
	_, hungerOK := foodFlowInteger(obs.Hunger, 0, 24)
	bag, bagOK := foodFlowInteger(obs.BagUnits, 0, 1)
	last, lastOK := foodFlowInteger(obs.LastGatherHour, -1, foodFlowHorizonHours-1)
	if !energyOK || !hungerOK || !bagOK || !lastOK || last > int64(obs.Time/sim.SimTime(foodFlowHour)) {
		choice.Fallback = FoodFlowInvalidSelf
		return choice, nil
	}
	source := sim.EntityID(0)
	if bag == 1 {
		var err error
		source, err = obs.BagSource.EntityRef()
		if err != nil || sim.ValidateEntityID(source) != nil {
			choice.Fallback = FoodFlowInvalidSelf
			return choice, nil
		}
	} else if obs.BagSource.Kind() != sim.EntityRefKind || obs.BagSource.State() != sim.Missing {
		choice.Fallback = FoodFlowInvalidSelf
		return choice, nil
	}
	if energy == 0 || obs.Time == sim.SimTime(foodFlowHorizonHours)*sim.SimTime(foodFlowHour) {
		return choice, nil
	}
	if bag == 1 {
		if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 {
			choice.Fallback = FoodFlowBudgetExhausted
			return choice, nil
		}
		choice.Kind, choice.TargetPatch, choice.Evaluated, choice.Fallback = FoodFlowEat, source, 1, FoodFlowNoFallback
		return choice, nil
	}
	if obs.LastDenial == FoodFlowGatherDenied {
		choice.Kind, choice.RestDuration, choice.Evaluated, choice.Fallback = FoodFlowRest, b.policy.RestDuration, 1, FoodFlowNoFallback
		return choice, nil
	}
	if last == int64(obs.Time/sim.SimTime(foodFlowHour)) {
		return choice, nil // a second Gather in the same hour is never proposed
	}
	type candidate struct{ patch, slot sim.EntityID }
	candidates := make([]candidate, 0, len(seenSlots))
	for _, patch := range patches {
		for _, slot := range patch.Slots {
			if stock, ok := foodFlowInteger(slot.Stock, 0, 1); ok && stock == 1 {
				candidates = append(candidates, candidate{patch.ID, slot.ID})
			}
		}
	}
	if len(candidates) > b.policy.Budget.Candidates || len(candidates) > b.policy.Budget.Evaluations {
		choice.Fallback = FoodFlowBudgetExhausted
		return choice, nil
	}
	if len(candidates) == 0 {
		return choice, nil
	}
	// Equal-cost candidates tie on (patch ID, slot ID), not view or worker order.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].patch != candidates[j].patch {
			return candidates[i].patch < candidates[j].patch
		}
		return candidates[i].slot < candidates[j].slot
	})
	choice.Kind, choice.TargetPatch, choice.TargetSlot = FoodFlowGather, candidates[0].patch, candidates[0].slot
	choice.Evaluated, choice.Fallback = len(candidates), FoodFlowNoFallback
	return choice, nil
}
