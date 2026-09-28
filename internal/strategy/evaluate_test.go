package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"math"
	"testing"
)

func scalar(v float64) sim.Value {
	value, err := sim.ScalarValue(v)
	if err != nil {
		panic(err)
	}
	return value
}

func fixture() Policy {
	return Policy{FormatVersion: FormatV1, Ref: Ref{ID: "survival", Version: 1},
		Budget: Budget{Candidates: MaxCandidates, Evaluations: MaxEvaluations},
		Actions: []Action{
			{Kind: Eat, Terms: []UtilityTerm{{Feature: OwnHunger, Param: 1}, {Feature: TargetFoodStock, Param: 2}}},
			{Kind: Rest, Terms: []UtilityTerm{{Feature: Constant, Param: 3}}},
		},
		Defaults: map[ParamID]float64{1: 1, 2: 0, 3: 3},
	}
}

func observation() Observation {
	return Observation{Actor: 1, Time: 0, WorldVersion: 9, Energy: scalar(4), Hunger: scalar(3),
		PublicFood: []FoodView{{ID: 11, Stock: scalar(5)}, {ID: 10, Stock: scalar(3)}}}
}

func bound(t *testing.T, p Policy, overrides map[ParamID]float64) *Bound {
	t.Helper()
	r := NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(Binding{Ref: p.Ref, Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEvaluateOrderTiesAndOverrides(t *testing.T) {
	p := fixture()
	b := bound(t, p, nil)
	obs := observation()
	got, err := b.Evaluate(obs)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Choice{Kind: Rest, Ref: p.Ref, ObservedVersion: 9, Score: 3, Evaluated: 3}) {
		t.Fatalf("rest wins exact tie: %+v", got)
	}
	first, second := obs.PublicFood[0], obs.PublicFood[1]
	obs.PublicFood[0], obs.PublicFood[1] = second, first
	again, err := b.Evaluate(obs)
	if err != nil || again != got {
		t.Fatalf("reordered views: %+v, %v", again, err)
	}

	// Distinct actors bind overrides without copying or changing the shared
	// action specification. Only permitted cache views enter either evaluation.
	eater := bound(t, p, map[ParamID]float64{3: 2})
	obs.Actor = 2
	eaten, err := eater.Evaluate(obs)
	if err != nil {
		t.Fatal(err)
	}
	if eaten.Kind != Eat || eaten.Target != 10 || eaten.Score != 3 || eaten.Evaluated != 3 || eaten.Ref != p.Ref {
		t.Fatalf("lowest cache ID wins eat tie: %+v", eaten)
	}
	obs.Actor = 1
	if restAgain, _ := b.Evaluate(obs); restAgain.Kind != Rest {
		t.Fatal("actor override mutated another binding")
	}
	stockSensitive := bound(t, p, map[ParamID]float64{2: 1, 3: 2})
	if stockChoice, err := stockSensitive.Evaluate(obs); err != nil || stockChoice.Kind != Eat || stockChoice.Target != 11 || stockChoice.Score != 8 {
		t.Fatalf("stock utility should prefer larger visible cache: %+v, %v", stockChoice, err)
	}
}

func TestEvaluateMissingInvalidAndOverflow(t *testing.T) {
	p := fixture()
	b := bound(t, p, nil)
	obs := observation()
	missing, _ := sim.AbsentValue(sim.ScalarKind, sim.Missing)
	unknown, _ := sim.AbsentValue(sim.ScalarKind, sim.Unknown)
	for _, invalid := range []sim.Value{missing, unknown, sim.BoolValue(true), scalar(-1), scalar(.5), scalar(float64(1 << 53))} {
		obs.Energy = invalid
		choice, err := b.Evaluate(obs)
		if err != nil || choice.Kind != Wait || choice.Fallback != NoCandidate || choice.Evaluated != 0 {
			t.Fatalf("invalid own energy: %+v, %v", choice, err)
		}
	}
	obs = observation()
	obs.Hunger = missing
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != Wait {
		t.Fatalf("missing hunger: %+v, %v", choice, err)
	}
	obs = observation()
	obs.PublicFood[0].Stock = unknown
	obs.PublicFood[1].Stock = scalar(1)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != Rest || choice.Evaluated != 1 {
		t.Fatalf("invalid food removes only eat candidates: %+v, %v", choice, err)
	}
	obs = observation()
	obs.Energy, obs.Hunger = scalar(0), scalar(0)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != Wait || choice.Fallback != NoCandidate {
		t.Fatalf("no candidates: %+v, %v", choice, err)
	}
	// Finite coefficients can still overflow during utility evaluation; only
	// the affected candidate becomes invalid, not a nonfinite Choice.Score.
	p.Actions[0].Terms = []UtilityTerm{{Feature: TargetFoodStock, Param: 2}}
	p.Defaults[2] = math.MaxFloat64
	b = bound(t, p, nil)
	obs = observation()
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != Rest || choice.Evaluated != 3 {
		t.Fatalf("overflowing eat score: %+v, %v", choice, err)
	}
	p.Actions[1].Terms = []UtilityTerm{{Feature: OwnEnergy, Param: 2}}
	b = bound(t, p, nil)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != Wait || choice.Fallback != NoCandidate || choice.Evaluated != 3 {
		t.Fatalf("all scores overflow: %+v, %v", choice, err)
	}
}

func TestEvaluateLimitsAndMalformedViews(t *testing.T) {
	p := fixture()
	p.Budget = Budget{Candidates: 2, Evaluations: 2}
	b := bound(t, p, nil)
	obs := observation()
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != Wait || choice.Fallback != BudgetExhausted || choice.Evaluated != 0 {
		t.Fatalf("candidate budget: %+v, %v", choice, err)
	}
	p.Budget = Budget{Candidates: 3, Evaluations: 2}
	b = bound(t, p, nil)
	if choice, err := b.Evaluate(obs); err != nil || choice.Fallback != BudgetExhausted {
		t.Fatalf("evaluation budget: %+v, %v", choice, err)
	}
	p.Budget = Budget{Candidates: MaxCandidates, Evaluations: MaxEvaluations}
	b = bound(t, p, nil)
	obs.PublicFood = make([]FoodView, MaxFoodViews)
	for i := range obs.PublicFood {
		obs.PublicFood[i] = FoodView{ID: sim.EntityID(i + 10), Stock: scalar(2)}
	}
	if choice, err := b.Evaluate(obs); err != nil || choice.Evaluated != MaxCandidates {
		t.Fatalf("boundary candidates: %+v, %v", choice, err)
	}
	obs.PublicFood = append(obs.PublicFood, FoodView{ID: 1000, Stock: scalar(2)})
	if _, err := b.Evaluate(obs); !errors.Is(err, ErrInvalidObservation) {
		t.Fatalf("too many views: %v", err)
	}
	obs = observation()
	obs.PublicFood[1].ID = obs.PublicFood[0].ID
	if _, err := b.Evaluate(obs); !errors.Is(err, ErrInvalidObservation) {
		t.Fatalf("duplicate views: %v", err)
	}
	obs.PublicFood[1].ID = 0
	if _, err := b.Evaluate(obs); !errors.Is(err, ErrInvalidObservation) {
		t.Fatalf("zero cache ID: %v", err)
	}
	obs = observation()
	obs.Actor = 0
	if _, err := b.Evaluate(obs); !errors.Is(err, ErrInvalidObservation) {
		t.Fatalf("zero actor: %v", err)
	}
	obs = observation()
	obs.Time = -1
	if _, err := b.Evaluate(obs); !errors.Is(err, ErrInvalidObservation) {
		t.Fatalf("negative time: %v", err)
	}
}

func TestRegistrationAndReferenceValidation(t *testing.T) {
	base := fixture()
	tests := []struct {
		name   string
		change func(*Policy)
	}{
		{"unknown format", func(p *Policy) { p.FormatVersion = 2 }},
		{"empty ref", func(p *Policy) { p.Ref.ID = "" }},
		{"long ref", func(p *Policy) { p.Ref.ID = string(make([]byte, MaxRefIDBytes+1)) }},
		{"zero version", func(p *Policy) { p.Ref.Version = 0 }},
		{"zero budget", func(p *Policy) { p.Budget.Candidates = 0 }},
		{"excess candidates", func(p *Policy) { p.Budget.Candidates = MaxCandidates + 1 }},
		{"excess evaluations", func(p *Policy) { p.Budget.Evaluations = MaxEvaluations + 1 }},
		{"missing action", func(p *Policy) { p.Actions = p.Actions[:1] }},
		{"duplicate action", func(p *Policy) { p.Actions[1].Kind = Eat }},
		{"unknown action", func(p *Policy) { p.Actions[1].Kind = Wait }},
		{"unknown feature", func(p *Policy) { p.Actions[0].Terms[0].Feature = 200 }},
		{"rest target feature", func(p *Policy) { p.Actions[1].Terms[0].Feature = TargetFoodStock }},
		{"missing parameter", func(p *Policy) { p.Actions[0].Terms[0].Param = 9 }},
		{"zero parameter", func(p *Policy) { p.Actions[0].Terms[0].Param = 0 }},
		{"nonfinite default", func(p *Policy) { p.Defaults[1] = math.NaN() }},
		{"infinite default", func(p *Policy) { p.Defaults[1] = math.Inf(1) }},
		{"excess defaults", func(p *Policy) {
			for i := 4; i <= MaxParameters+1; i++ {
				p.Defaults[ParamID(i)] = 1
			}
		}},
		{"excess terms", func(p *Policy) { p.Actions[0].Terms = make([]UtilityTerm, MaxUtilityTerms+1) }},
		{"missing parent", func(p *Policy) { p.Ref.Version = 2; p.ParentRef = &base.Ref }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture()
			tc.change(&p)
			if err := NewRegistry().Register(p); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("expected invalid policy: %v", err)
			}
		})
	}
	r := NewRegistry()
	if err := r.Register(base); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(base); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("duplicate ref: %v", err)
	}
	p := fixture()
	p.Ref.Version = 2
	p.ParentRef = &base.Ref
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	p.Ref.Version = 3
	p.ParentRef = &p.Ref
	if err := r.Register(p); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("self-parent: %v", err)
	}
	p = fixture()
	p.Ref = Ref{ID: "unrelated", Version: 2}
	p.ParentRef = &base.Ref
	if err := r.Register(p); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("incompatible parent: %v", err)
	}
	for _, ref := range []Ref{{ID: "survival", Version: 3}, {ID: "unknown", Version: 1}} {
		if _, err := r.Bind(Binding{Ref: ref}); !errors.Is(err, ErrUnknownRef) {
			t.Fatalf("unknown ref %+v: %v", ref, err)
		}
	}
	if _, err := r.Bind(Binding{Ref: Ref{ID: "survival"}}); !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("invalid ref: %v", err)
	}
	tooManyOverrides := make(map[ParamID]float64)
	for id := 1; id <= MaxParameters+1; id++ {
		tooManyOverrides[ParamID(id)] = 1
	}
	for _, overrides := range []map[ParamID]float64{{99: 1}, {1: math.NaN()}, {1: math.Inf(-1)}, tooManyOverrides} {
		if _, err := r.Bind(Binding{Ref: base.Ref, Overrides: overrides}); !errors.Is(err, ErrInvalidBinding) {
			t.Fatalf("invalid override: %v", err)
		}
	}
}

func TestRegistrationAndBindingDeepCopy(t *testing.T) {
	p := fixture()
	r := NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	overrides := map[ParamID]float64{3: 2}
	b, err := r.Bind(Binding{Ref: p.Ref, Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	p.Actions[0].Terms[0].Feature = Constant
	p.Actions[0].Terms[0].Param = 3
	p.Defaults[1] = 99
	p.Ref.Version = 99
	overrides[3] = 100
	obs := observation()
	choice, err := b.Evaluate(obs)
	if err != nil || choice.Kind != Eat || choice.Score != 3 || choice.Ref.Version != 1 {
		t.Fatalf("source mutations affected bound policy: %+v, %v", choice, err)
	}
	second, err := r.Bind(Binding{Ref: Ref{ID: "survival", Version: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if choice, err := second.Evaluate(obs); err != nil || choice.Kind != Rest {
		t.Fatalf("source mutations affected registered policy: %+v, %v", choice, err)
	}
}
