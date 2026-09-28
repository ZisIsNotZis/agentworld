package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func foodFlowFixture(t *testing.T) (*strategy.FoodFlowRegistry, strategy.FoodFlowRef) {
	t.Helper()
	r := strategy.NewFoodFlowRegistry()
	ref := strategy.FoodFlowRef{ID: "pilot", Version: 1}
	if err := r.Register(strategy.FoodFlowPolicy{FormatVersion: strategy.FoodFlowPolicyFormatV1, Ref: ref,
		Budget:       strategy.FoodFlowBudget{Candidates: strategy.FoodFlowMaxCandidates, Evaluations: strategy.FoodFlowMaxCandidates},
		RestDuration: world.FoodFlowHour}); err != nil {
		t.Fatal(err)
	}
	return r, ref
}

func foodFlowObservation(actor sim.EntityID) strategy.FoodFlowObservation {
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	return strategy.FoodFlowObservation{Actor: actor, Time: 1, WorldVersion: 3,
		Energy: sim.IntegerValue(11), Hunger: sim.IntegerValue(0), BagUnits: sim.IntegerValue(0),
		BagSource: missing, LastGatherHour: sim.IntegerValue(-1),
		Patches: []strategy.FoodFlowPatchView{{ID: 1001, Slots: []strategy.FoodFlowSlotView{
			{ID: 2002, Stock: sim.IntegerValue(1)}, {ID: 2001, Stock: sim.IntegerValue(1)},
		}}}}
}

func foodFlowBound(t *testing.T, r *strategy.FoodFlowRegistry, ref strategy.FoodFlowRef, actor sim.EntityID) *strategy.FoodFlowBound {
	t.Helper()
	b, err := r.Bind(strategy.FoodFlowBinding{Actor: actor, Ref: ref})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFoodFlowHeldEatEmptyGatherDeniedRest(t *testing.T) {
	r, ref := foodFlowFixture(t)
	b := foodFlowBound(t, r, ref, 1)
	obs := foodFlowObservation(1)
	choice, err := b.Evaluate(obs)
	if err != nil || choice != (strategy.FoodFlowChoice{Kind: strategy.FoodFlowGather, TargetPatch: 1001, TargetSlot: 2001,
		ObservedVersion: 3, Ref: ref, Evaluated: 2}) {
		t.Fatalf("empty bag gather sorted visible slot: %+v %v", choice, err)
	}
	obs.BagUnits = sim.IntegerValue(1)
	obs.BagSource, _ = sim.EntityRefValue(1001)
	obs.LastGatherHour = sim.IntegerValue(0)
	obs.LastDenial = strategy.FoodFlowGatherDenied
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.FoodFlowChoice{Kind: strategy.FoodFlowEat, TargetPatch: 1001,
		ObservedVersion: 3, Ref: ref, Evaluated: 1}) {
		t.Fatalf("held source-attributed food precedes denial: %+v %v", choice, err)
	}
	obs = foodFlowObservation(1)
	obs.LastDenial = strategy.FoodFlowGatherDenied
	choice, err = b.Evaluate(obs)
	if err != nil || choice != (strategy.FoodFlowChoice{Kind: strategy.FoodFlowRest, RestDuration: world.FoodFlowHour,
		ObservedVersion: 3, Ref: ref, Evaluated: 1}) {
		t.Fatalf("denied gather must rest for one hour: %+v %v", choice, err)
	}
	obs.Energy = sim.IntegerValue(0)
	choice, err = b.Evaluate(obs)
	if err != nil || choice.Kind != strategy.FoodFlowWait || choice.RestDuration != 0 || choice.Evaluated != 0 {
		t.Fatalf("zero-energy actor must not act: %+v %v", choice, err)
	}
	obs = foodFlowObservation(1)
	obs.Time = sim.SimTime(world.FoodFlowHour) + 1
	obs.LastGatherHour = sim.IntegerValue(1)
	choice, err = b.Evaluate(obs)
	if err != nil || choice.Kind != strategy.FoodFlowWait || choice.Fallback != strategy.FoodFlowNoCandidate {
		t.Fatalf("no second gather in an hour: %+v %v", choice, err)
	}
}

func TestFoodFlowInputFailuresAndVisibleBoundary(t *testing.T) {
	r, ref := foodFlowFixture(t)
	b := foodFlowBound(t, r, ref, 1)
	obs := foodFlowObservation(1)
	missing, _ := sim.AbsentValue(sim.IntegerKind, sim.Missing)
	for _, invalid := range []sim.Value{missing, sim.BoolValue(true), sim.IntegerValue(-1), sim.IntegerValue(13)} {
		bad := obs
		bad.Energy = invalid
		c, err := b.Evaluate(bad)
		if err != nil || c.Kind != strategy.FoodFlowWait || c.Fallback != strategy.FoodFlowInvalidSelf {
			t.Fatalf("invalid energy: %+v %v", c, err)
		}
	}
	for _, field := range []string{"hunger", "bag", "last gather"} {
		bad := obs
		switch field {
		case "hunger":
			bad.Hunger = missing
		case "bag":
			bad.BagUnits = missing
		case "last gather":
			bad.LastGatherHour = missing
		}
		if c, err := b.Evaluate(bad); err != nil || c.Kind != strategy.FoodFlowWait || c.Fallback != strategy.FoodFlowInvalidSelf {
			t.Fatalf("missing %s: %+v %v", field, c, err)
		}
	}
	bad := obs
	bad.BagUnits = sim.IntegerValue(1)
	if c, err := b.Evaluate(bad); err != nil || c.Fallback != strategy.FoodFlowInvalidSelf {
		t.Fatalf("missing source must not Eat: %+v %v", c, err)
	}
	bad = obs
	bad.LastGatherHour = sim.IntegerValue(1)
	if c, err := b.Evaluate(bad); err != nil || c.Fallback != strategy.FoodFlowInvalidSelf {
		t.Fatalf("future gather hour: %+v %v", c, err)
	}
	bad = foodFlowObservation(1)
	bad.Patches[0].Slots = []strategy.FoodFlowSlotView{{ID: 2001, Stock: missing}}
	if c, err := b.Evaluate(bad); err != nil || c.Kind != strategy.FoodFlowWait || c.Fallback != strategy.FoodFlowNoCandidate {
		t.Fatalf("unknown stock does not enable Gather: %+v %v", c, err)
	}
	bad = obs
	bad.Actor = 9
	if _, err := b.Evaluate(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowObservation) {
		t.Fatalf("wrong actor binding: %v", err)
	}
	bad = foodFlowObservation(1)
	bad.Patches[0].Slots[1].ID = 2002
	if _, err := b.Evaluate(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowObservation) {
		t.Fatalf("duplicate slot: %v", err)
	}
	bad = obs
	bad.Patches = append(bad.Patches, strategy.FoodFlowPatchView{ID: 1001})
	if _, err := b.Evaluate(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowObservation) {
		t.Fatalf("duplicate patch: %v", err)
	}
	bad = obs
	bad.Patches = append(bad.Patches, strategy.FoodFlowPatchView{ID: 1003}, strategy.FoodFlowPatchView{ID: 1004})
	if _, err := b.Evaluate(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowObservation) {
		t.Fatalf("too many patch views: %v", err)
	}
	bad = obs
	bad.Patches = []strategy.FoodFlowPatchView{{ID: 1001, Slots: make([]strategy.FoodFlowSlotView, strategy.FoodFlowMaxSlotsPerPatch+1)}}
	if _, err := b.Evaluate(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowObservation) {
		t.Fatalf("too many slot views: %v", err)
	}
	bad = obs
	bad.LastDenial = 255
	if _, err := b.Evaluate(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowObservation) {
		t.Fatalf("unknown denial: %v", err)
	}
	// Only authorized views enter the policy. A forged foreign Gather is still
	// denied by the independently trusted world contract (actor 1 owns 1001).
	if c, err := b.Evaluate(obs); err != nil || c.TargetPatch != 1001 || c.TargetSlot != 2001 {
		t.Fatalf("policy generated unseen target: %+v %v", c, err)
	}
	actor := world.FoodFlowActorState{Energy: world.FoodFlowInitialEnergy, LastGatherHour: -1}
	if _, _, err := world.FoodFlowGather(1002, 2009, 1, 0, world.FoodFlowSlotState{Stock: 1}, actor); !errors.Is(err, world.ErrFoodFlowContract) {
		t.Fatalf("world accepted forged inaccessible patch: %v", err)
	}
}

func TestFoodFlowPinnedPolicyAndBudget(t *testing.T) {
	r, ref := foodFlowFixture(t)
	b := foodFlowBound(t, r, ref, 1)
	for _, invalid := range []strategy.FoodFlowPolicy{
		{FormatVersion: 2, Ref: strategy.FoodFlowRef{ID: "wrong", Version: 1}, Budget: strategy.FoodFlowBudget{1, 1}, RestDuration: world.FoodFlowHour},
		{FormatVersion: 1, Ref: strategy.FoodFlowRef{ID: "bad", Version: 1}, Budget: strategy.FoodFlowBudget{0, 1}, RestDuration: world.FoodFlowHour},
		{FormatVersion: 1, Ref: strategy.FoodFlowRef{ID: "bad", Version: 1}, Budget: strategy.FoodFlowBudget{1, 1}, RestDuration: 0},
	} {
		if err := r.Register(invalid); !errors.Is(err, strategy.ErrInvalidFoodFlowPolicy) {
			t.Fatalf("bad policy accepted: %v", err)
		}
	}
	if err := r.Register(strategy.FoodFlowPolicy{FormatVersion: 1, Ref: ref, Budget: strategy.FoodFlowBudget{1, 1}, RestDuration: world.FoodFlowHour}); !errors.Is(err, strategy.ErrInvalidFoodFlowPolicy) {
		t.Fatalf("duplicate version: %v", err)
	}
	if _, err := r.Bind(strategy.FoodFlowBinding{Actor: 1, Ref: strategy.FoodFlowRef{ID: ref.ID, Version: 2}}); !errors.Is(err, strategy.ErrUnknownFoodFlowPolicy) {
		t.Fatalf("unregistered version: %v", err)
	}
	if _, err := r.Bind(strategy.FoodFlowBinding{Actor: 0, Ref: ref}); !errors.Is(err, strategy.ErrInvalidFoodFlowBinding) {
		t.Fatalf("invalid actor: %v", err)
	}
	limited := strategy.NewFoodFlowRegistry()
	limitedRef := strategy.FoodFlowRef{ID: "limited", Version: 1}
	if err := limited.Register(strategy.FoodFlowPolicy{FormatVersion: 1, Ref: limitedRef, Budget: strategy.FoodFlowBudget{Candidates: 1, Evaluations: 1}, RestDuration: world.FoodFlowHour}); err != nil {
		t.Fatal(err)
	}
	choice, err := foodFlowBound(t, limited, limitedRef, 1).Evaluate(foodFlowObservation(1))
	if err != nil || choice.Kind != strategy.FoodFlowWait || choice.Fallback != strategy.FoodFlowBudgetExhausted || choice.Evaluated != 0 {
		t.Fatalf("all-or-nothing bounded evaluation: %+v %v", choice, err)
	}
	obs := foodFlowObservation(1)
	obs.Patches[0].Slots = obs.Patches[0].Slots[:1]
	if c, err := b.Evaluate(obs); err != nil || c.Kind != strategy.FoodFlowGather || c.Evaluated != 1 {
		t.Fatalf("registered policy changed: %+v %v", c, err)
	}
	obs = foodFlowObservation(1)
	obs.Patches = append(obs.Patches, strategy.FoodFlowPatchView{ID: 1000, Slots: []strategy.FoodFlowSlotView{{ID: 1999, Stock: sim.IntegerValue(1)}}})
	if c, err := b.Evaluate(obs); err != nil || c.Kind != strategy.FoodFlowGather || c.TargetPatch != 1000 || c.TargetSlot != 1999 || c.Evaluated != 3 {
		t.Fatalf("patch/slot tie order: %+v %v", c, err)
	}
}

func TestFoodFlowWorkerOrderIndependentBindings(t *testing.T) {
	r, ref := foodFlowFixture(t)
	bindings := make([]*strategy.FoodFlowBound, 16)
	for i := range bindings {
		bindings[i] = foodFlowBound(t, r, ref, sim.EntityID(i+1))
		if bindings[i].Binding() != (strategy.FoodFlowBinding{Actor: sim.EntityID(i + 1), Ref: ref}) {
			t.Fatal("binding did not retain actor and ref")
		}
	}
	run := func(reverse bool) []strategy.FoodFlowChoice {
		choices := make([]strategy.FoodFlowChoice, len(bindings))
		var wg sync.WaitGroup
		for j := range bindings {
			i := j
			if reverse {
				i = len(bindings) - 1 - j
			}
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				obs := foodFlowObservation(sim.EntityID(i + 1))
				if i >= 8 {
					obs.Patches[0].ID = 1002
					obs.Patches[0].Slots[0].ID, obs.Patches[0].Slots[1].ID = 2010, 2009
				}
				if reverse {
					obs.Patches[0].Slots[0], obs.Patches[0].Slots[1] = obs.Patches[0].Slots[1], obs.Patches[0].Slots[0]
				}
				var err error
				choices[i], err = bindings[i].Evaluate(obs)
				if err != nil {
					t.Errorf("actor %d: %v", i+1, err)
				}
			}(i)
		}
		wg.Wait()
		return choices
	}
	first, reversed := run(false), run(true)
	if !reflect.DeepEqual(first, reversed) {
		t.Fatalf("worker/view order changed pure choices:\n%+v\n%+v", first, reversed)
	}
	for i, c := range first {
		target := sim.EntityID(2001)
		if i >= 8 {
			target = 2009
		}
		if c.TargetSlot != target || c.Ref != ref || c.Evaluated != 2 {
			t.Fatalf("actor %d: %+v", i+1, c)
		}
	}
}
