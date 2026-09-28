package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"math"
	"sort"
)

var ErrInvalidObservation = errors.New("invalid strategy observation")

type FoodView struct {
	ID    sim.EntityID
	Stock sim.Value
}

// Observation is a value-only view; no authority, reader, or private peer data.
type Observation struct {
	Actor        sim.EntityID
	Time         sim.SimTime
	WorldVersion sim.WorldVersion
	Energy       sim.Value
	Hunger       sim.Value
	PublicFood   []FoodView
}

type FallbackReason uint8

const (
	NoFallback FallbackReason = iota
	NoCandidate
	BudgetExhausted
)

// Choice is an intention only. Score is not physical authority; the world
// rechecks both visibility and action preconditions against trusted state.
type Choice struct {
	Kind            ActionKind
	Target          sim.EntityID // zero for Rest or Wait
	ObservedVersion sim.WorldVersion
	Ref             Ref
	Score           float64
	Evaluated       int
	Fallback        FallbackReason
}

// v1 uses whole nonnegative energy units, leaving room for a precise +/- 1
// float64 update. The world independently validates stored patch arithmetic.
const maxSafeUnits = float64(1<<53 - 2)

func units(v sim.Value) (float64, bool) {
	n, err := v.Scalar()
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 && n <= maxSafeUnits && math.Trunc(n) == n
}

func (b *Bound) Evaluate(obs Observation) (Choice, error) {
	if b == nil || sim.ValidateEntityID(obs.Actor) != nil || obs.Time < 0 || len(obs.PublicFood) > MaxFoodViews {
		return Choice{}, ErrInvalidObservation
	}
	choice := Choice{Kind: Wait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NoCandidate}
	food := append([]FoodView(nil), obs.PublicFood...)
	sort.Slice(food, func(i, j int) bool { return food[i].ID < food[j].ID })
	for i, view := range food {
		if sim.ValidateEntityID(view.ID) != nil || (i > 0 && food[i-1].ID == view.ID) {
			return Choice{}, ErrInvalidObservation
		}
	}
	energy, energyOK := units(obs.Energy)
	hunger, hungerOK := units(obs.Hunger)
	if !energyOK || !hungerOK {
		return choice, nil
	}
	type candidate struct {
		kind   ActionKind
		target sim.EntityID
		stock  float64
	}
	candidates := make([]candidate, 0, len(food)+1)
	// Rest precedes Eat at equal scores, regardless of action declaration order.
	if energy >= 1 && hunger < maxSafeUnits {
		candidates = append(candidates, candidate{kind: Rest})
	}
	if hunger >= 1 && energy < maxSafeUnits {
		for _, view := range food {
			if stock, ok := units(view.Stock); ok && stock >= 2 {
				candidates = append(candidates, candidate{kind: Eat, target: view.ID, stock: stock})
			}
		}
	}
	// A small budget cannot silently discard later candidates and bias a tie.
	if len(candidates) > b.policy.Budget.Candidates || len(candidates) > b.policy.Budget.Evaluations {
		choice.Fallback = BudgetExhausted
		return choice, nil
	}
	for _, candidate := range candidates {
		choice.Evaluated++
		action := b.action(candidate.kind)
		score, valid := b.score(action.Terms, energy, hunger, candidate.stock)
		if !valid {
			continue
		}
		if choice.Kind == Wait || score > choice.Score {
			choice.Kind, choice.Target, choice.Score = candidate.kind, candidate.target, score
			choice.Fallback = NoFallback
		}
	}
	return choice, nil
}

func (b *Bound) action(kind ActionKind) Action {
	for _, action := range b.policy.Actions {
		if action.Kind == kind {
			return action
		}
	}
	return Action{} // unreachable for a validated policy
}

func (b *Bound) score(terms []UtilityTerm, energy, hunger, stock float64) (float64, bool) {
	score := 0.0
	for _, term := range terms {
		value := 1.0
		switch term.Feature {
		case OwnEnergy:
			value = energy
		case OwnHunger:
			value = hunger
		case TargetFoodStock:
			value = stock
		}
		score += value * b.params[term.Param]
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return 0, false
		}
	}
	return score, true
}
