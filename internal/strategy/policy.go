// Package strategy evaluates bounded, versioned policy data over actor-safe values.
// A choice is an intention; the trusted world must validate physical effects.
package strategy

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

const (
	FormatV1        uint32 = 1
	MaxFoodViews           = 64
	MaxCandidates          = MaxFoodViews + 1
	MaxEvaluations         = MaxCandidates
	MaxUtilityTerms        = 8 // per action
	MaxParameters          = 16
	MaxRefIDBytes          = 64
)

var (
	ErrInvalidPolicy  = errors.New("invalid strategy policy")
	ErrUnknownRef     = errors.New("unknown strategy policy reference")
	ErrInvalidBinding = errors.New("invalid strategy binding")
)

// Ref pins a policy to an exact ID and version. Versions are never inferred.
type Ref struct {
	ID      string
	Version uint32
}

type ParamID uint16

type FeatureID uint8

const (
	OwnEnergy FeatureID = iota + 1
	OwnHunger
	TargetFoodStock
	Constant
)

type ActionKind uint8

const (
	Eat ActionKind = iota + 1
	Rest
	Wait // a fallback only, never a registered action
)

type Budget struct {
	Candidates  int
	Evaluations int
}

// A term contributes the indicated feature multiplied by its parameter.
// Terms are summed in their declared order; no callbacks or other operators exist.
type UtilityTerm struct {
	Feature FeatureID
	Param   ParamID
}

type Action struct {
	Kind  ActionKind
	Terms []UtilityTerm
}

// ParentRef is an exact lineage reference, not implicit inheritance: actions,
// defaults and budget are always complete in this policy.
type Policy struct {
	FormatVersion uint32
	Ref           Ref
	ParentRef     *Ref
	Budget        Budget
	Actions       []Action
	Defaults      map[ParamID]float64
}

type Binding struct {
	Ref       Ref
	Overrides map[ParamID]float64
}

// Registry copies every policy on registration and never exposes its stored
// slices or maps. Register policies before sharing the registry with workers.
type Registry struct {
	mu       sync.RWMutex
	policies map[Ref]Policy
}

func NewRegistry() *Registry { return &Registry{policies: make(map[Ref]Policy)} }

func validRef(ref Ref) bool {
	return ref.ID != "" && len(ref.ID) <= MaxRefIDBytes && ref.Version != 0
}

func (r *Registry) Register(p Policy) error {
	if r == nil || p.FormatVersion != FormatV1 || !validRef(p.Ref) ||
		p.Budget.Candidates < 1 || p.Budget.Candidates > MaxCandidates ||
		p.Budget.Evaluations < 1 || p.Budget.Evaluations > MaxEvaluations ||
		len(p.Actions) != 2 || len(p.Defaults) > MaxParameters {
		return ErrInvalidPolicy
	}
	seen := make(map[ActionKind]bool, len(p.Actions))
	for _, action := range p.Actions {
		if (action.Kind != Rest && action.Kind != Eat) || seen[action.Kind] || len(action.Terms) > MaxUtilityTerms {
			return ErrInvalidPolicy
		}
		seen[action.Kind] = true
		for _, term := range action.Terms {
			if term.Feature < OwnEnergy || term.Feature > Constant || term.Param == 0 ||
				(action.Kind == Rest && term.Feature == TargetFoodStock) {
				return ErrInvalidPolicy
			}
			if _, ok := p.Defaults[term.Param]; !ok {
				return fmt.Errorf("%w: missing default for parameter %d", ErrInvalidPolicy, term.Param)
			}
		}
	}
	if !seen[Rest] || !seen[Eat] {
		return ErrInvalidPolicy
	}
	for id, value := range p.Defaults {
		if id == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return ErrInvalidPolicy
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policies == nil {
		return ErrInvalidPolicy
	}
	if _, exists := r.policies[p.Ref]; exists {
		return fmt.Errorf("%w: duplicate reference", ErrInvalidPolicy)
	}
	if p.ParentRef != nil {
		parent, ok := r.policies[*p.ParentRef]
		if !validRef(*p.ParentRef) || !ok || parent.FormatVersion != FormatV1 ||
			p.ParentRef.ID != p.Ref.ID || p.ParentRef.Version >= p.Ref.Version {
			return fmt.Errorf("%w: missing or incompatible parent", ErrInvalidPolicy)
		}
	}
	copy := Policy{FormatVersion: p.FormatVersion, Ref: p.Ref, Budget: p.Budget,
		Actions: make([]Action, len(p.Actions)), Defaults: make(map[ParamID]float64, len(p.Defaults))}
	if p.ParentRef != nil {
		parent := *p.ParentRef
		copy.ParentRef = &parent
	}
	for i, action := range p.Actions {
		copy.Actions[i] = Action{Kind: action.Kind, Terms: append([]UtilityTerm(nil), action.Terms...)}
	}
	for id, value := range p.Defaults {
		copy.Defaults[id] = value
	}
	r.policies[p.Ref] = copy
	return nil
}

// Bound holds a private policy snapshot and resolved parameters. It remains
// stable if the source policy or binding is subsequently mutated.
type Bound struct {
	policy Policy
	params map[ParamID]float64
}

func (r *Registry) Bind(binding Binding) (*Bound, error) {
	if r == nil || !validRef(binding.Ref) || len(binding.Overrides) > MaxParameters {
		return nil, ErrInvalidBinding
	}
	r.mu.RLock()
	policy, ok := r.policies[binding.Ref]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownRef
	}
	params := make(map[ParamID]float64, len(policy.Defaults))
	for id, value := range policy.Defaults {
		params[id] = value
	}
	for id, value := range binding.Overrides {
		if _, ok := params[id]; !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, ErrInvalidBinding
		}
		params[id] = value
	}
	return &Bound{policy: policy, params: params}, nil
}

func (b *Bound) Ref() Ref {
	if b == nil {
		return Ref{}
	}
	return b.policy.Ref
}
