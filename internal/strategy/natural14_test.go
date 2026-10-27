package strategy_test

import (
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// TestNatural14RoundRobinFounderHome proves natural/14 differs from
// natural/13 in exactly the founder-home observation clause: the same
// round-robin founder home is accepted under natural/14 and refused under
// natural/13, and every other validation is unchanged.
func TestNatural14RoundRobinFounderHome(t *testing.T) {
	if !strategy.ValidFrozenNaturalPolicy(strategy.FrozenNaturalPolicy14()) {
		t.Fatal("natural/14 is not in the frozen ref set")
	}
	ref := strategy.FrozenNaturalPolicy14().Ref
	registry := strategy.NewNaturalRegistry(2)
	if err := registry.RegisterFrozenNatural(strategy.FrozenNaturalPolicy14()); err != nil {
		t.Fatal(err)
	}
	bound, err := registry.Bind(strategy.NaturalBinding{Actor: 2, Ref: ref})
	if err != nil {
		t.Fatal(err)
	}
	if bound.Policy() != strategy.FrozenNaturalPolicy14() {
		t.Fatalf("bound policy %+v", bound.Policy())
	}
	patch2 := sim.EntityID(1002)
	ref2, err := sim.EntityRefValue(patch2)
	if err != nil {
		t.Fatal(err)
	}
	obs := strategy.NaturalObservation{
		Actor: 2, Ref: ref, WorldVersion: 10, Hour: 1,
		Energy: sim.IntegerValue(11), Hunger: sim.IntegerValue(0), BagUnits: sim.IntegerValue(0),
		Capital: sim.IntegerValue(0), Wip: sim.IntegerValue(0), GranaryStock: sim.IntegerValue(0),
		InvestedUnits: sim.IntegerValue(0), LastBuildHour: sim.IntegerValue(-1),
		LastStoredMealHour: sim.IntegerValue(-1), BagSource: ref2,
	}
	// Founder 2 lives on patch 1002 under round-robin placement.
	if _, err := bound.Evaluate14(obs); err != nil {
		t.Fatalf("natural/14 refused a round-robin founder home: %v", err)
	}
	// The same observation under natural/13's blocked geometry degrades to
	// InvalidSelf (founder 2 is declared on patch 1001 there).
	reg13 := strategy.NewNaturalRegistry(2)
	if err := reg13.RegisterFrozenNatural(strategy.FrozenNaturalPolicy13()); err != nil {
		t.Fatal(err)
	}
	bound13, err := reg13.Bind(strategy.NaturalBinding{Actor: 2, Ref: strategy.FrozenNaturalPolicy13().Ref})
	if err != nil {
		t.Fatal(err)
	}
	obs13 := obs
	obs13.Ref = strategy.FrozenNaturalPolicy13().Ref
	choice, err := bound13.Evaluate13(obs13)
	if err != nil {
		t.Fatal(err)
	}
	if choice.Fallback != strategy.NaturalInvalidSelf {
		t.Fatalf("natural/13 accepted the round-robin home: %+v", choice)
	}
}
