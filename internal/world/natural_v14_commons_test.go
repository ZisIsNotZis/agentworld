package world

import (
	"errors"
	"testing"
)

// TestNaturalV14Mechanism proves mechanism v14 is admitted as world version
// 14 and inherits the v13 rule table (birth price, two-energy meal, yield
// axis, wild fill, round-robin founding band) with only the commons floor
// added.
func TestNaturalV14Mechanism(t *testing.T) {
	if !NaturalMechanismValid(NaturalMechanismV14) || NaturalMechanismV14.worldVersion() != 14 {
		t.Fatal("mechanism v14 is not admitted as world version 14")
	}
	if NaturalMechanismV14.FormatVersion() != 14 || NaturalMechanismV14.RuleVersion() != 14 ||
		NaturalMechanismV14.ProjectionVersion() != 14 {
		t.Fatal("mechanism v14 version fields")
	}
	if _, err := NaturalRegistryFor(NaturalMechanismV14); err != nil {
		t.Fatalf("v14 registry: %v", err)
	}
	// v14 keeps the v13 rule table.
	if naturalMealEnergyFor(NaturalMechanismV14) != naturalMealEnergyV12 ||
		!naturalHasYieldAxis(NaturalMechanismV14) || !naturalWildFillYield(NaturalMechanismV14) ||
		!naturalRoundRobinFounders(NaturalMechanismV14) || !naturalHasBirthCostAxis(NaturalMechanismV14) ||
		naturalSlotStockMaxFor(NaturalMechanismV14) != NaturalYieldMax {
		t.Fatal("v14 does not carry the v13 rule table")
	}
	if naturalBirthCostFor(NaturalMechanismV14, 0) != NaturalParentGranaryCost ||
		naturalBirthCostFor(NaturalMechanismV14, 4) != 4 {
		t.Fatal("v14 birth price is not the v13 declared-price axis")
	}
	// The commons floor is the declared half of the frozen patch shape.
	if NaturalCommonsWildMin != NaturalSlotsPerPatch/2 {
		t.Fatalf("commons floor %d is not half of %d", NaturalCommonsWildMin, NaturalSlotsPerPatch)
	}
	// Only v14 declares the commons rule: every earlier mechanism keeps its
	// admission exactly, so the boundary below is a v14-only refusal.
	if !naturalKeepsCommons(NaturalMechanismV14) {
		t.Fatal("v14 does not keep the commons")
	}
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7,
		NaturalMechanismV8, NaturalMechanismV9, NaturalMechanismV10, NaturalMechanismV11,
		NaturalMechanismV12, NaturalMechanismV13} {
		if naturalKeepsCommons(m) {
			t.Fatalf("mechanism %d admits the v14 commons rule", uint16(m))
		}
	}
}

// naturalV14BuildFixture builds the completing build fixture on a home patch
// with the first cultivated slots tenured by the builder and the rest wild
// and empty, so the completion's slot claim sits exactly at the commons
// boundary.
func naturalV14BuildFixture(t *testing.T, m NaturalMechanism, patches, cultivated int) (hour int, parent NaturalActorState, home [NaturalSlotsPerPatch]NaturalSlotState) {
	t.Helper()
	parent, err := NaturalFounderStateFor(m, patches, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	parent.Worksite = NaturalWorksiteState{Capital: 1, Wip: 1, PointsCreated: 1, InvestedUnits: 3, LastBuildHour: NaturalNeverHour}
	parent.Bag.Units = 1
	parent.Body.LastGatherHour = 4
	if !validNaturalActorFor(m, patches, 1, parent) {
		t.Fatal("build fixture actor invalid")
	}
	for i := 0; i < cultivated; i++ {
		home[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
	}
	return 5, parent, home
}

// TestNaturalV14CommonsFloor pins the v14 admission boundary: a build whose
// completion claim would leave exactly half the patch wild is admitted, and
// a build that would take the last wild slot of the half is refused on the
// typed build path with zero mutation — while v13 admits the same build.
func TestNaturalV14CommonsFloor(t *testing.T) {
	patches := 2
	half := NaturalCommonsWildMin

	// Exactly half the patch wild before the claim, half after it: admitted.
	hour, parent, home := naturalV14BuildFixture(t, NaturalMechanismV14, patches, NaturalSlotsPerPatch-half-1)
	after, slots, result, err := NaturalBuildFor(NaturalMechanismV14, patches, hour, 1, parent, parent.Bag.Source, home)
	if err != nil {
		t.Fatalf("v14 claim that keeps half the patch refused: %v", err)
	}
	if result.ClaimedSlotIndex != NaturalSlotsPerPatch-half-1 || result.PointsCompleted != 1 {
		t.Fatalf("v14 admitted claim: %+v", result)
	}
	wild := 0
	for _, s := range slots {
		if s.Mode == NaturalTenureWild {
			wild++
		}
	}
	if wild != half {
		t.Fatalf("v14 admitted claim left %d wild slots, want %d", wild, half)
	}
	if after.Worksite.Capital != 2 || after.Bag.Units != 0 {
		t.Fatalf("v14 admitted claim did not convert: %+v", after)
	}

	// Exactly half the patch wild before the claim: the claim would take the
	// last wild slot of the half and is refused, typed, with zero mutation.
	hour, parent, home = naturalV14BuildFixture(t, NaturalMechanismV14, patches, NaturalSlotsPerPatch-half)
	got, gotSlots, gotResult, err := NaturalBuildFor(NaturalMechanismV14, patches, hour, 1, parent, parent.Bag.Source, home)
	var refusal *NaturalBuildRefusalError
	if !errors.As(err, &refusal) || refusal.Reason != NaturalBuildRefusalNoWildSlot {
		t.Fatalf("v14 commons-floor refusal: %v", err)
	}
	if got != (NaturalActorState{}) || gotResult != (NaturalBuildResult{}) || gotSlots != home {
		t.Fatal("v14 commons-floor refusal mutated state")
	}

	// The same build is admitted under v13: earlier mechanisms keep their
	// admission exactly.
	hour, parent, home = naturalV14BuildFixture(t, NaturalMechanismV13, patches, NaturalSlotsPerPatch-half)
	after, slots, result, err = NaturalBuildFor(NaturalMechanismV13, patches, hour, 1, parent, parent.Bag.Source, home)
	if err != nil || result.ClaimedSlotIndex != NaturalSlotsPerPatch-half || result.PointsCompleted != 1 {
		t.Fatalf("v13 claim refused by the v14 rule: %+v %v", result, err)
	}
	wild = 0
	for _, s := range slots {
		if s.Mode == NaturalTenureWild {
			wild++
		}
	}
	if wild != half-1 {
		t.Fatalf("v13 claim left %d wild slots, want %d", wild, half-1)
	}
	if after.Worksite.Capital != 2 || after.Bag.Units != 0 {
		t.Fatalf("v13 admitted claim did not convert: %+v", after)
	}
}
