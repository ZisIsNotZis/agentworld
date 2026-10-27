package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// TestNaturalV12Mechanism proves mechanism v12 is admitted as world version
// 12, carries the v11 rule table verbatim, and declares the one price change:
// an eaten bag unit is a meal of two energy instead of one.
func TestNaturalV12Mechanism(t *testing.T) {
	if !NaturalMechanismValid(NaturalMechanismV12) || NaturalMechanismV12.worldVersion() != 12 {
		t.Fatal("mechanism v12 is not admitted as world version 12")
	}
	if NaturalMechanismV12.FormatVersion() != 12 || NaturalMechanismV12.RuleVersion() != 12 ||
		NaturalMechanismV12.ProjectionVersion() != 12 {
		t.Fatal("mechanism v12 version fields")
	}
	if _, err := NaturalRegistryFor(NaturalMechanismV12); err != nil {
		t.Fatalf("v12 registry: %v", err)
	}
	// v12 keeps v11's round-robin founding band, its yield axis, and its
	// yield-scaled wild fill; the declared price is the only difference.
	if !naturalRoundRobinFounders(NaturalMechanismV12) || !naturalHasYieldAxis(NaturalMechanismV12) ||
		!naturalWildFillYield(NaturalMechanismV12) || naturalSlotStockMaxFor(NaturalMechanismV12) != NaturalYieldMax {
		t.Fatal("v12 does not carry the v11 rule table")
	}
	if naturalMealEnergyFor(NaturalMechanismV12) != naturalMealEnergyV12 || naturalMealEnergyV12 != 2 {
		t.Fatal("v12 meal price")
	}
	if naturalCapLostMaxFor(NaturalMechanismV12) != 2*NaturalHorizonHours {
		t.Fatalf("v12 cap-loss envelope %d", naturalCapLostMaxFor(NaturalMechanismV12))
	}
	// The frozen mechanisms keep the one-energy meal and their envelopes.
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7,
		NaturalMechanismV8, NaturalMechanismV9, NaturalMechanismV10, NaturalMechanismV11} {
		if naturalMealEnergyFor(m) != 1 {
			t.Fatalf("mechanism %d changed the meal price", m)
		}
		if naturalCapLostMaxFor(m) != NaturalHorizonHours {
			t.Fatalf("mechanism %d widened the cap-loss envelope", m)
		}
	}
	// The body descriptor carries the same declared envelope (one home).
	body := NaturalBodyDescriptorFor(NaturalMechanismV12)
	for _, f := range body.Fields {
		if f.Name == "cap-lost" && f.Bounds.Maximum != float64(2*NaturalHorizonHours) {
			t.Fatalf("v12 cap-lost field bound %v", f.Bounds.Maximum)
		}
	}
}

// mealOutcome gathers one wild unit and eats it under the declared mechanism,
// starting from the frozen founder at the given hour.
func mealOutcome(t *testing.T, m NaturalMechanism, hour int64, a NaturalActorState) (NaturalActorState, NaturalMeal) {
	t.Helper()
	patches, patchID, slotID := 1, sim.EntityID(NaturalPatchIDBase), sim.EntityID(NaturalSlotIDBase)
	actor := sim.EntityID(1)
	_, a, err := NaturalGatherFor(m, patches, patchID, slotID, actor, int(hour), NaturalSlotState{Stock: 1}, a)
	if err != nil {
		t.Fatalf("mechanism %d hour %d gather: %v", m, hour, err)
	}
	next, meal, err := NaturalConsumeFor(m, patches, actor, a)
	if err != nil {
		t.Fatalf("mechanism %d hour %d consume: %v", m, hour, err)
	}
	return next, meal
}

// TestNaturalV12MealGrantsTwoEnergy proves the multiplication site: one eaten
// bag unit grants two energy, and whatever does not fit under the frozen cap
// is counted spill, so the G3 identity keeps balancing.
func TestNaturalV12MealGrantsTwoEnergy(t *testing.T) {
	patches, actor := 1, sim.EntityID(1)
	start, err := NaturalFounderStateFor(NaturalMechanismV12, patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	// Hour 0: energy 11 → 12, one unit of the meal spills over the cap.
	a, meal := mealOutcome(t, NaturalMechanismV12, 0, start)
	if a.Body.Energy != 12 || a.Body.Consumed != 1 || a.Body.CapLost != 1 {
		t.Fatalf("v12 first meal %+v", a.Body)
	}
	if meal.EnergyGained != 1 || meal.EnergyCapLost != 1 {
		t.Fatalf("v12 first meal result %+v", meal)
	}
	if !validNaturalActorFor(NaturalMechanismV12, patches, actor, a) {
		t.Fatal("v12 first meal broke the G3 identity")
	}
	// Hour 1: energy already at the cap, so the whole two-energy meal spills.
	a, meal = mealOutcome(t, NaturalMechanismV12, 1, a)
	if a.Body.Energy != 12 || a.Body.Consumed != 2 || a.Body.CapLost != 3 {
		t.Fatalf("v12 capped meal %+v", a.Body)
	}
	if meal.EnergyGained != 0 || meal.EnergyCapLost != 2 {
		t.Fatalf("v12 capped meal result %+v", meal)
	}
	if !validNaturalActorFor(NaturalMechanismV12, patches, actor, a) {
		t.Fatal("v12 capped meal broke the G3 identity")
	}
	// The frozen v11 meal is unchanged: +1 energy, one unit of spill at most.
	frozen, err := NaturalFounderStateFor(NaturalMechanismV11, patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	frozen, meal = mealOutcome(t, NaturalMechanismV11, 0, frozen)
	if frozen.Body.Energy != 12 || frozen.Body.Consumed != 1 || frozen.Body.CapLost != 0 {
		t.Fatalf("v11 first meal %+v", frozen.Body)
	}
	if meal.EnergyGained != 1 || meal.EnergyCapLost != 0 {
		t.Fatalf("v11 first meal result %+v", meal)
	}
	frozen, meal = mealOutcome(t, NaturalMechanismV11, 1, frozen)
	if frozen.Body.Energy != 12 || frozen.Body.CapLost != 1 || meal.EnergyCapLost != 1 {
		t.Fatalf("v11 capped meal %+v %+v", frozen.Body, meal)
	}
	if !validNaturalActorFor(NaturalMechanismV11, patches, actor, frozen) {
		t.Fatal("v11 meal broke the G3 identity")
	}
}

// TestNaturalV12WorldBalances proves a v12 world runs with the declared price:
// every pulse's conservation gate (G1/G2a/G2b/G3) balances, so the two-energy
// meal is booked exactly once.
func TestNaturalV12WorldBalances(t *testing.T) {
	ctx := context.Background()
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 1, Seed: 0, Workers: 1, Founders: 10,
		PolicyRef: strategy.FrozenNaturalPolicy14().Ref, Mechanism: NaturalMechanismV12})
	if err != nil {
		t.Fatalf("v12 world: %v", err)
	}
	for i := 0; i < 40; i++ {
		if ok, err := f.Step(ctx); err != nil || !ok {
			t.Fatalf("v12 step %d: %t %v", i, ok, err)
		}
	}
	last := f.Checkpoints()[len(f.Checkpoints())-1]
	if last.Balance.Consumed == 0 {
		t.Fatal("v12 world consumed nothing over forty hours")
	}
	if last.Balance.InitialEnergy+NaturalNewbornEnergy*0+naturalMealEnergyV12*last.Balance.Consumed+last.Balance.StoredMeals !=
		last.Balance.Energy+last.Balance.BasalSpent+last.Balance.EnergyCapLost {
		t.Fatalf("v12 balance does not close: %+v", last.Balance)
	}
}

// TestNaturalV12CheckpointIdentity proves the v12 mechanism is run identity
// end to end: the bundle round-trips under natural/14, a mismatched mechanism
// fails closed, and natural/13 stays refused.
func TestNaturalV12CheckpointIdentity(t *testing.T) {
	ctx := context.Background()
	ref14 := strategy.FrozenNaturalPolicy14().Ref
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy13().Ref, Mechanism: NaturalMechanismV12}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("natural/13 accepted under v12: %v", err)
	}
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1, Founders: 10,
		PolicyRef: ref14, Mechanism: NaturalMechanismV12})
	if err != nil {
		t.Fatalf("v12/natural/14 world: %v", err)
	}
	if ok, err := f.Step(ctx); err != nil || !ok {
		t.Fatalf("v12 h0 pulse: %t %v", ok, err)
	}
	path := t.TempDir() + "/v12.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatalf("v12 save: %v", err)
	}
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 2, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV12})
	if err != nil {
		t.Fatalf("v12 restore: %v", err)
	}
	if restored.mechanism != NaturalMechanismV12 || restored.yield != 2 || restored.ref != ref14 {
		t.Fatalf("restored v12 identity %d/%d/%+v", restored.mechanism, restored.yield, restored.ref)
	}
	if _, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 1, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV12}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v12 yield-1 restore accepted a yield-2 bundle: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 2, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV11}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v11 restore accepted a v12 bundle: %v", err)
	}
}
