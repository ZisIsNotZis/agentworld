package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"errors"
	"reflect"
	"testing"
)

var capacityAbsentRef, _ = sim.AbsentValue(sim.EntityRefKind, sim.Missing)

func capacityFixture(t *testing.T, actor sim.EntityID) (*strategy.CapacityBound, strategy.CapacityRef) {
	t.Helper()
	p := strategy.FrozenCapacityPolicy()
	r := strategy.NewCapacityRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.CapacityBinding{Actor: actor, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b, p.Ref
}

// capacityObservation builds a valid mid-run view of one actor at hour 5 from
// world-contract geometry only: own rows at frozen bounds defaults, all eight
// home-patch slots empty.
func capacityObservation(actor sim.EntityID, ref strategy.CapacityRef, hour int) strategy.CapacityObservation {
	home, err := world.CapacityActorPatchID(actor)
	if err != nil {
		panic(err)
	}
	slots := make([]strategy.CapacitySlotView, 0, world.CapacitySlotsPerPatch)
	for i := 0; i < world.CapacitySlotsPerPatch; i++ {
		slot, err := world.CapacitySlotID(int(home-1001), i)
		if err != nil {
			panic(err)
		}
		slots = append(slots, strategy.CapacitySlotView{ID: slot, Stock: sim.IntegerValue(0)})
	}
	return strategy.CapacityObservation{
		Actor: actor, Ref: ref, Hour: hour, WorldVersion: 9,
		Energy: sim.IntegerValue(world.CapacityInitialEnergy), Hunger: sim.IntegerValue(0),
		BagUnits: sim.IntegerValue(0), BagSource: capacityAbsentRef,
		Capital: sim.IntegerValue(0), Wip: sim.IntegerValue(0),
		GranaryStock: sim.IntegerValue(0), InvestedUnits: sim.IntegerValue(0),
		LastBuildHour: sim.IntegerValue(world.CapacityNeverHour), LastStoredMealHour: sim.IntegerValue(world.CapacityNeverHour),
		HomeSlots: slots,
	}
}

func capacityStocked(obs strategy.CapacityObservation, slotIndexes ...int) strategy.CapacityObservation {
	for _, i := range slotIndexes {
		obs.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	return obs
}

func capacityHeld(obs strategy.CapacityObservation) strategy.CapacityObservation {
	home, _ := world.CapacityActorPatchID(obs.Actor)
	obs.BagUnits = sim.IntegerValue(1)
	obs.BagSource, _ = sim.EntityRefValue(home)
	return obs
}

func TestCapacityFrozenPolicyRegistration(t *testing.T) {
	p := strategy.FrozenCapacityPolicy()
	if p != (strategy.CapacityPolicy{FormatVersion: 3, Ref: strategy.CapacityRef{ID: "capacity", Version: 3},
		Budget: strategy.CapacityBudget{Candidates: 16, Evaluations: 16}}) {
		t.Fatalf("frozen capacity policy drifted: %+v", p)
	}
	r := strategy.NewCapacityRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for _, invalid := range []strategy.CapacityPolicy{
		func() strategy.CapacityPolicy { c := p; c.FormatVersion = 2; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Ref.ID = ""; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Ref.ID = "capacity-v3"; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Ref.ID = longID; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Ref.Version = 0; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Ref.Version = 4; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Budget.Candidates = 0; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Budget.Candidates = 17; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Budget.Evaluations = 0; return c }(),
		func() strategy.CapacityPolicy { c := p; c.Budget.Evaluations = 17; return c }(),
	} {
		if err := r.Register(invalid); !errors.Is(err, strategy.ErrInvalidCapacityPolicy) {
			t.Fatalf("invalid capacity policy accepted: %+v %v", invalid, err)
		}
	}
	if err := r.Register(p); !errors.Is(err, strategy.ErrInvalidCapacityPolicy) {
		t.Fatalf("duplicate reference accepted: %v", err)
	}
	if _, err := r.Bind(strategy.CapacityBinding{Actor: 1, Ref: strategy.CapacityRef{ID: p.Ref.ID, Version: 4}}); !errors.Is(err, strategy.ErrUnknownCapacityPolicy) {
		t.Fatalf("unregistered version bound: %v", err)
	}
	if _, err := r.Bind(strategy.CapacityBinding{Actor: 1, Ref: strategy.CapacityRef{ID: "", Version: 3}}); !errors.Is(err, strategy.ErrInvalidCapacityBinding) {
		t.Fatalf("empty ref bound: %v", err)
	}
	for _, actor := range []sim.EntityID{0, 17} {
		if _, err := r.Bind(strategy.CapacityBinding{Actor: actor, Ref: p.Ref}); !errors.Is(err, strategy.ErrInvalidCapacityBinding) {
			t.Fatalf("actor %d outside the fixed roster bound: %v", actor, err)
		}
	}
	b, err := r.Bind(strategy.CapacityBinding{Actor: 16, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	if b.Binding() != (strategy.CapacityBinding{Actor: 16, Ref: p.Ref}) || b.Policy() != p {
		t.Fatalf("binding roundtrip: %+v %+v", b.Binding(), b.Policy())
	}
	mutated := b.Policy()
	mutated.Budget.Candidates = 1
	if b.Policy() != p {
		t.Fatal("bound policy is not value-only")
	}
	other, err := r.Bind(strategy.CapacityBinding{Actor: 1, Ref: p.Ref})
	if err != nil || other.Policy() != p {
		t.Fatalf("registry content changed: %+v %v", other.Policy(), err)
	}
	var nilBound *strategy.CapacityBound
	if nilBound.Binding() != (strategy.CapacityBinding{}) || nilBound.Policy() != (strategy.CapacityPolicy{}) {
		t.Fatal("nil bound must degrade to zero values")
	}
	if _, err := nilBound.Evaluate(capacityObservation(1, p.Ref, 0)); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
		t.Fatalf("nil bound evaluated: %v", err)
	}
}

// The policy lane must stay identical to the trusted world contract: frozen
// identity, roster, budget ceiling, horizon, and the ten-minute v1 rest window.
func TestCapacityLaneMatchesWorldContract(t *testing.T) {
	p := strategy.FrozenCapacityPolicy()
	if uint32(world.CapacityFormatVersion) != p.FormatVersion || uint32(world.CapacitySchemaVersion) != p.Ref.Version ||
		uint32(world.CapacityRuleVersion) != 3 {
		t.Fatalf("v3 identity drift: world %d/%d/%d", world.CapacityFormatVersion, world.CapacitySchemaVersion, world.CapacityRuleVersion)
	}
	if strategy.CapacityMaxActors != world.CapacityActorCount || strategy.CapacityMaxCandidates != world.CapacityActorCount {
		t.Fatalf("roster/budget ceiling drift: %d %d vs %d", strategy.CapacityMaxActors, strategy.CapacityMaxCandidates, world.CapacityActorCount)
	}
	if strategy.CapacityRestDuration != sim.Duration(600*1e6) || world.CapacityMealDuration != sim.Duration(600*1e6) {
		t.Fatalf("rest window drifted from the ten-minute v1 discipline: %d vs %d", strategy.CapacityRestDuration, world.CapacityMealDuration)
	}
	if world.CapacityPolicyTLow != 5 || world.CapacityKMax != 8 || world.CapacityGranaryMax != 8 || world.CapacityWipMax != 1 {
		t.Fatal("frozen cascade thresholds changed in the world contract")
	}
	// Every home-patch slot the world names is gatherable by a home actor and
	// by nobody else; the admissible set is exactly the home patch.
	for actor := sim.EntityID(1); actor <= world.CapacityActorCount; actor++ {
		home, err := world.CapacityActorPatchID(actor)
		if err != nil {
			t.Fatal(err)
		}
		bound, ref := capacityFixture(t, actor)
		for i := 0; i < world.CapacitySlotsPerPatch; i++ {
			slot, err := world.CapacitySlotID(int(home-1001), i)
			if err != nil {
				t.Fatal(err)
			}
			obs := capacityStocked(capacityObservation(actor, ref, 0), i)
			choice, err := bound.Evaluate(obs)
			if err != nil || choice.Kind != strategy.CapacityGather || choice.TargetSlot != slot || choice.Evaluated != 1 {
				t.Fatalf("actor %d slot %d: %+v %v", actor, slot, choice, err)
			}
		}
		if _, err := bound.Evaluate(capacityObservation(actor, ref, world.CapacityHorizonHours-1)); err != nil {
			t.Fatalf("final claimable hour %d rejected: %v", world.CapacityHorizonHours-1, err)
		}
		if _, err := bound.Evaluate(capacityObservation(actor, ref, world.CapacityHorizonHours)); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
			t.Fatalf("hour %d (basal only) accepted: %v", world.CapacityHorizonHours, err)
		}
	}
}

func TestCapacityClaimCascade(t *testing.T) {
	b, ref := capacityFixture(t, 1)
	// Visible stock wins over the granary, ties break on the lowest slot ID,
	// and the whole candidate set is evaluated (all-or-nothing budget).
	obs := capacityStocked(capacityObservation(1, ref, 5), 2, 0, 5)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.CapacityChoice{Kind: strategy.CapacityGather, TargetSlot: 2001,
		ObservedVersion: 9, Ref: ref, Evaluated: 3}) {
		t.Fatalf("gather must take the lowest stocked home slot: %+v %v", choice, err)
	}
	// Input order never matters: the same view shuffled picks the same slot.
	shuffled := capacityStocked(capacityObservation(1, ref, 5), 5, 2, 0)
	shuffled.HomeSlots[3], shuffled.HomeSlots[6] = shuffled.HomeSlots[6], shuffled.HomeSlots[3]
	if choice, err := b.Evaluate(shuffled); err != nil || choice.TargetSlot != 2001 || choice.Evaluated != 3 {
		t.Fatalf("shuffled slots must not change the choice: %+v %v", choice, err)
	}
	// No visible stock, granary holds a unit: the only granary draw.
	obs = capacityObservation(1, ref, 5)
	obs.GranaryStock = sim.IntegerValue(1)
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.CapacityChoice{Kind: strategy.CapacityEatStored,
		ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("empty wild and stocked granary must draw a stored meal: %+v %v", choice, err)
	}
	// Nothing anywhere: Wait is a fallback with no evaluation.
	choice, err = b.Evaluate(capacityObservation(1, ref, 5))
	if err != nil || choice != (strategy.CapacityChoice{Kind: strategy.CapacityWait,
		ObservedVersion: 9, Ref: ref, Fallback: strategy.CapacityNoCandidate}) {
		t.Fatalf("nothing available must wait: %+v %v", choice, err)
	}
	// Gather priority over stored meals when both are visible.
	obs = capacityStocked(capacityObservation(1, ref, 5), 7)
	obs.GranaryStock = sim.IntegerValue(8)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.CapacityGather || choice.TargetSlot != 2008 {
		t.Fatalf("wild gather precedes stored meals: %+v %v", choice, err)
	}
	// A dead actor never acts.
	obs = capacityStocked(capacityObservation(1, ref, 5), 0)
	obs.Energy = sim.IntegerValue(0)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.CapacityWait || choice.Evaluated != 0 || choice.Fallback != strategy.CapacityNoCandidate {
		t.Fatalf("zero-energy actor must not act: %+v %v", choice, err)
	}
}

// The frozen bag decision (h+20m+2µs): Build iff (energy>=CapacityPolicyTLow=5
// or granary>=1) and k+wip<8, else Eat. The policy never emits EatStored here:
// a build's paired stored meal is drawn fresh by the runner at completion.
func TestCapacityBagDecisionTLowBoundaryAndCapRefusal(t *testing.T) {
	b, ref := capacityFixture(t, 1)
	base := capacityHeld(capacityObservation(1, ref, 5))
	for _, tc := range []struct {
		name                       string
		energy, capital, wip, gran int64
		want                       strategy.CapacityAction
	}{
		{"one below the frozen floor eats", 4, 0, 0, 0, strategy.CapacityEat},
		{"at the frozen floor builds", 5, 0, 0, 0, strategy.CapacityBuild},
		{"granary fallback funds a build at energy 4", 4, 0, 0, 1, strategy.CapacityBuild},
		{"full energy builds", 12, 0, 0, 0, strategy.CapacityBuild},
		{"seven points plus wip is saturated", 12, 7, 1, 0, strategy.CapacityEat},
		{"eight points is saturated", 12, 8, 0, 0, strategy.CapacityEat},
		{"seven points without wip still builds", 12, 7, 0, 0, strategy.CapacityBuild},
		{"outstanding wip completes the second unit", 5, 0, 1, 0, strategy.CapacityBuild},
		{"low energy with stocked granary builds for the runner-paired meal", 4, 0, 0, 8, strategy.CapacityBuild},
	} {
		obs := base
		obs.Energy = sim.IntegerValue(tc.energy)
		obs.Capital = sim.IntegerValue(tc.capital)
		obs.Wip = sim.IntegerValue(tc.wip)
		obs.GranaryStock = sim.IntegerValue(tc.gran)
		choice, err := b.Evaluate(obs)
		if err != nil || choice != (strategy.CapacityChoice{Kind: tc.want,
			ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
			t.Fatalf("%s: %+v %v", tc.name, choice, err)
		}
		if choice.Kind == strategy.CapacityEatStored {
			t.Fatalf("%s: bag decision emitted a stored meal", tc.name)
		}
	}
}

func TestCapacityDeniedGatherFallback(t *testing.T) {
	b, ref := capacityFixture(t, 1)
	// Denied with a stocked granary: the fallback draw, never a re-gather.
	obs := capacityStocked(capacityObservation(1, ref, 5), 0, 3)
	obs.GranaryStock = sim.IntegerValue(1)
	obs.LastDenial = strategy.CapacityGatherDenied
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.CapacityChoice{Kind: strategy.CapacityEatStored,
		ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gatherer with a stocked granary must eat stored: %+v %v", choice, err)
	}
	// Denied with an empty granary: the frozen ten-minute rest.
	obs.GranaryStock = sim.IntegerValue(0)
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.CapacityChoice{Kind: strategy.CapacityRest,
		RestDuration: strategy.CapacityRestDuration, ObservedVersion: 9, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gatherer without stores must rest: %+v %v", choice, err)
	}
	// Without a denial the same view gathers again instead.
	obs.LastDenial = strategy.CapacityNoDenial
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.CapacityGather || choice.TargetSlot != 2001 {
		t.Fatalf("undecided claim must gather: %+v %v", choice, err)
	}
}

// At most one EatStored per actor-hour: every stored-meal emission requires
// LastStoredMealHour < the observed hour, so a re-evaluation at any later
// frozen slot of the same hour can never double-draw and exhaust the frozen
// stored-meals [0,168] ceiling (two draws per hour would hit it by ~h84).
func TestCapacityOneStoredMealPerActorHour(t *testing.T) {
	b, ref := capacityFixture(t, 1)
	const hour = 5
	// Claim draw with two stored units available.
	obs := capacityObservation(1, ref, hour)
	obs.GranaryStock = sim.IntegerValue(2)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.CapacityEatStored {
		t.Fatalf("claim draw: %+v %v", choice, err)
	}
	// After the draw is admitted, the same hour's later slots see
	// LastStoredMealHour == hour and must never emit a second EatStored,
	// regardless of how many units remain.
	for _, gran := range []int64{0, 1, 2, 8} {
		reobserved := obs
		reobserved.GranaryStock = sim.IntegerValue(gran)
		reobserved.LastStoredMealHour = sim.IntegerValue(hour)
		choice, err := b.Evaluate(reobserved)
		if err != nil || choice.Kind == strategy.CapacityEatStored {
			t.Fatalf("granary %d: second draw in hour %d: %+v %v", gran, hour, choice, err)
		}
		if choice.Kind != strategy.CapacityWait {
			t.Fatalf("granary %d: spent-hour claim must wait, got %+v", gran, choice)
		}
	}
	// The denied-gather fallback obeys the same gate: rest instead of a
	// second draw when this hour already consumed a stored meal.
	denied := obs
	denied.LastDenial = strategy.CapacityGatherDenied
	denied.LastStoredMealHour = sim.IntegerValue(hour)
	if choice, err := b.Evaluate(denied); err != nil || choice.Kind != strategy.CapacityRest || choice.RestDuration != strategy.CapacityRestDuration {
		t.Fatalf("denied fallback must not double-draw: %+v %v", choice, err)
	}
	// The bag decision never emits stored meals at all, spent hour or not;
	// the runner draws a build's paired meal fresh at completion.
	held := capacityHeld(capacityObservation(1, ref, hour))
	held.GranaryStock = sim.IntegerValue(3)
	held.LastStoredMealHour = sim.IntegerValue(hour)
	if choice, err := b.Evaluate(held); err != nil || choice.Kind != strategy.CapacityBuild {
		t.Fatalf("bag decision with granary fallback builds: %+v %v", choice, err)
	}
	held.Capital = sim.IntegerValue(8) // saturated worksite: eat the held unit itself
	if choice, err := b.Evaluate(held); err != nil || choice.Kind != strategy.CapacityEat {
		t.Fatalf("saturated bag decision eats the held unit itself: %+v %v", choice, err)
	}
	// The next hour may draw again: the ceiling is one stored meal per
	// actor-hour, not per lifetime.
	next := obs
	next.Hour = hour + 1
	if choice, err := b.Evaluate(next); err != nil || choice.Kind != strategy.CapacityEatStored {
		t.Fatalf("hour %d draw after hour %d meal: %+v %v", hour+1, hour, choice, err)
	}
}

func TestCapacityBudgetExhaustion(t *testing.T) {
	limited := strategy.NewCapacityRegistry()
	p := strategy.FrozenCapacityPolicy()
	p.Budget = strategy.CapacityBudget{Candidates: 1, Evaluations: 1}
	if err := limited.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := limited.Bind(strategy.CapacityBinding{Actor: 1, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	// Two stocked slots against a one-slot budget: the whole candidate set is
	// declined, never a sorted prefix.
	obs := capacityStocked(capacityObservation(1, p.Ref, 5), 0, 1)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.CapacityChoice{Kind: strategy.CapacityWait, ObservedVersion: 9,
		Ref: p.Ref, Fallback: strategy.CapacityBudgetExhausted}) {
		t.Fatalf("all-or-nothing budget: %+v %v", choice, err)
	}
	// A single candidate within budget still gathers.
	if choice, err := b.Evaluate(capacityStocked(capacityObservation(1, p.Ref, 5), 4)); err != nil ||
		choice.Kind != strategy.CapacityGather || choice.TargetSlot != 2005 || choice.Evaluated != 1 {
		t.Fatalf("bounded candidate set: %+v %v", choice, err)
	}
	// The bag decision keeps its own degenerate-budget guard and proceeds on
	// merit: initial energy sits at or above the frozen floor.
	if choice, err := b.Evaluate(capacityHeld(capacityObservation(1, p.Ref, 5))); err != nil ||
		choice.Kind != strategy.CapacityBuild || choice.Evaluated != 1 {
		t.Fatalf("bag decision under budget: %+v %v", choice, err)
	}
	// The frozen 16/16 budget covers the maximal legal candidate set: all
	// eight home slots stocked gather with every candidate evaluated and no
	// exhaustion fallback.
	fb, ref := capacityFixture(t, 1)
	all := capacityObservation(1, ref, 5)
	for i := range all.HomeSlots {
		all.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	if choice, err := fb.Evaluate(all); err != nil || choice.Kind != strategy.CapacityGather ||
		choice.TargetSlot != 2001 || choice.Evaluated != 8 || choice.Fallback != strategy.CapacityNoFallback {
		t.Fatalf("frozen budget must never truncate a legal set: %+v %v", choice, err)
	}
}

func TestCapacityObservationValidation(t *testing.T) {
	b, ref := capacityFixture(t, 1)
	valid := capacityObservation(1, ref, 5)
	if _, err := b.Evaluate(valid); err != nil {
		t.Fatal(err)
	}
	// Structural malformations are errors, never choices.
	actorMismatch := valid
	actorMismatch.Actor = 2
	refMismatch := valid
	refMismatch.Ref = strategy.CapacityRef{ID: "other", Version: 3}
	badDenial := valid
	badDenial.LastDenial = 255
	foreign := valid
	foreign.HomeSlots = append([]strategy.CapacitySlotView(nil), valid.HomeSlots...)
	foreign.HomeSlots[0].ID = 2009 // patch 1002 slot inside actor 1's view
	duplicate := valid
	duplicate.HomeSlots[1].ID = duplicate.HomeSlots[0].ID
	nineSlots := valid
	nineSlots.HomeSlots = append(append([]strategy.CapacitySlotView(nil), valid.HomeSlots...), strategy.CapacitySlotView{ID: 2017, Stock: sim.IntegerValue(0)})
	badStock := valid
	badStock.HomeSlots[2].Stock = sim.IntegerValue(2)
	for name, obs := range map[string]strategy.CapacityObservation{
		"actor mismatch": actorMismatch, "ref mismatch": refMismatch, "unknown denial": badDenial,
		"foreign slot": foreign, "duplicate slot": duplicate, "nine slots": nineSlots, "slot stock 2": badStock,
	} {
		if _, err := b.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
			t.Fatalf("%s accepted: %+v %v", name, obs, err)
		}
	}
	for _, actor := range []sim.EntityID{1, 9, 16} {
		bound, aref := capacityFixture(t, actor)
		foreignPatch, err := world.CapacitySlotID(1-int(actor-1)/world.CapacitySlotsPerPatch, 0)
		if err != nil {
			t.Fatal(err)
		}
		obs := capacityObservation(actor, aref, 0)
		obs.HomeSlots[0].ID = foreignPatch
		if _, err := bound.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
			t.Fatalf("actor %d accepted foreign slot %d: %v", actor, foreignPatch, err)
		}
	}
	// Out-of-range or non-integer own state degrades to Wait + InvalidSelf.
	mutators := map[string]func(*strategy.CapacityObservation){
		"energy high":  func(o *strategy.CapacityObservation) { o.Energy = sim.IntegerValue(13) },
		"energy low":   func(o *strategy.CapacityObservation) { o.Energy = sim.IntegerValue(-1) },
		"energy text":  func(o *strategy.CapacityObservation) { o.Energy = sim.BoolValue(true) },
		"energy frac":  func(o *strategy.CapacityObservation) { o.Energy, _ = sim.ScalarValue(5.5) },
		"hunger high":  func(o *strategy.CapacityObservation) { o.Hunger = sim.IntegerValue(25) },
		"bag high":     func(o *strategy.CapacityObservation) { o.BagUnits = sim.IntegerValue(2) },
		"bag low":      func(o *strategy.CapacityObservation) { o.BagUnits = sim.IntegerValue(-1) },
		"capital high": func(o *strategy.CapacityObservation) { o.Capital = sim.IntegerValue(9) },
		"wip high":     func(o *strategy.CapacityObservation) { o.Wip = sim.IntegerValue(2) },
		"granary high": func(o *strategy.CapacityObservation) { o.GranaryStock = sim.IntegerValue(9) },
		"granary low":  func(o *strategy.CapacityObservation) { o.GranaryStock = sim.IntegerValue(-1) },
		"invested high": func(o *strategy.CapacityObservation) {
			o.InvestedUnits = sim.IntegerValue(world.CapacityHorizonHours + 1)
		},
		"build hour high": func(o *strategy.CapacityObservation) {
			o.LastBuildHour = sim.IntegerValue(world.CapacityHorizonHours - 1 + 1)
		},
		"meal hour high": func(o *strategy.CapacityObservation) {
			o.LastStoredMealHour = sim.IntegerValue(world.CapacityHorizonHours)
		},
		"build hour future": func(o *strategy.CapacityObservation) { o.LastBuildHour = sim.IntegerValue(int64(o.Hour) + 1) },
		"meal hour future":  func(o *strategy.CapacityObservation) { o.LastStoredMealHour = sim.IntegerValue(int64(o.Hour) + 1) },
		"held source missing": func(o *strategy.CapacityObservation) {
			o.BagUnits = sim.IntegerValue(1)
			o.BagSource = capacityAbsentRef
		},
		"held source foreign": func(o *strategy.CapacityObservation) {
			o.BagUnits = sim.IntegerValue(1)
			o.BagSource, _ = sim.EntityRefValue(1002)
		},
		"empty source present":    func(o *strategy.CapacityObservation) { o.BagSource, _ = sim.EntityRefValue(1001) },
		"empty source wrong kind": func(o *strategy.CapacityObservation) { o.BagSource = sim.IntegerValue(0) },
	}
	for name, mutate := range mutators {
		obs := capacityObservation(1, ref, 5)
		mutate(&obs)
		choice, err := b.Evaluate(obs)
		if err != nil || choice.Kind != strategy.CapacityWait || choice.Fallback != strategy.CapacityInvalidSelf || choice.Evaluated != 0 {
			t.Fatalf("%s: %+v %v", name, choice, err)
		}
	}
	// An exact scalar projection of an integer is acceptable.
	obs := capacityObservation(1, ref, 5)
	obs.Energy, _ = sim.ScalarValue(5)
	obs.BagUnits = sim.IntegerValue(1)
	obs.BagSource, _ = sim.EntityRefValue(1001)
	if choice, err := b.Evaluate(obs); err != nil || choice.Kind != strategy.CapacityBuild {
		t.Fatalf("exact scalar energy accepted: %+v %v", choice, err)
	}
	// Structural errors fire before the cascade: hours outside the frozen
	// horizon, including the basal-only h=168 boundary.
	for _, hour := range []int{-1, world.CapacityHorizonHours} {
		obs := capacityObservation(1, ref, hour)
		if _, err := b.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
			t.Fatalf("hour %d accepted: %v", hour, err)
		}
	}
}

// Privacy discipline: the observation type can carry only the frozen
// actor-visible list — no field exists through which another actor's capital,
// granary, energy, meals, or any privileged counter could arrive — and a bound
// never evaluates or exposes another actor's state.
func TestCapacityObservationCarriesOwnStateOnly(t *testing.T) {
	want := []string{"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Hunger", "BagUnits", "BagSource",
		"Capital", "Wip", "GranaryStock", "InvestedUnits", "LastBuildHour", "LastStoredMealHour", "LastDenial", "HomeSlots"}
	typ := reflect.TypeOf(strategy.CapacityObservation{})
	got := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		got = append(got, typ.Field(i).Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observation field set drifted: %v", got)
	}
	b1, ref := capacityFixture(t, 1)
	b2, _ := capacityFixture(t, 2)
	if _, err := b2.Evaluate(capacityObservation(1, ref, 5)); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
		t.Fatal("a bound evaluated another actor's observation")
	}
	if b1.Policy() != strategy.FrozenCapacityPolicy() || b1.Binding() != (strategy.CapacityBinding{Actor: 1, Ref: ref}) {
		t.Fatalf("accessors exposed non-frozen or foreign content: %+v %+v", b1.Policy(), b1.Binding())
	}
	// Home-patch slot stocks are the only shared state; another patch's slots
	// cannot enter the view at all.
	foreign, err := world.CapacitySlotID(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	obs := capacityObservation(1, ref, 5)
	obs.HomeSlots[0].ID = foreign
	if _, err := b1.Evaluate(obs); !errors.Is(err, strategy.ErrInvalidCapacityObservation) {
		t.Fatalf("foreign patch slot accepted: %v", err)
	}
}
