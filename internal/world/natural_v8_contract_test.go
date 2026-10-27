package world

import (
	"testing"

	"agentworld/internal/sim"
)

// naturalV8HouseholdConsenter builds a valid consenter at the v8 household
// gate: energy 2, granary 0, capital 0 (and the G3 closure intact).
func naturalV8HouseholdConsenter(t *testing.T, patches int, actor sim.EntityID) NaturalActorState {
	t.Helper()
	a, err := NaturalFounderState(patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	a.Worksite = NaturalWorksiteState{}
	a.Granary = NaturalGranaryState{LastStoredMealHour: NaturalNeverHour}
	a.Body = NaturalBodyState{Energy: 2, BasalSpent: 9, LastGatherHour: NaturalNeverHour}
	if !validNaturalActorFor(NaturalMechanismV8, patches, actor, a) {
		t.Fatalf("household consenter fixture invalid: %+v", a)
	}
	return a
}

// TestNaturalV8HouseholdConsentGate proves the first mechanism-v8 rule change:
// a consenter with energy 2, granary 0, and capital 0 consents under v8 and is
// refused consent-own under v6, while every frozen mechanism keeps the full
// consenter wealth gate.
func TestNaturalV8HouseholdConsentGate(t *testing.T) {
	patches := 2
	proposer := naturalGateParent(t, patches, 1)
	consenter := naturalV8HouseholdConsenter(t, patches, 2)
	proposerRequest, err := NaturalBirthRequestFor(NaturalMechanismV8, patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	draws := naturalDrawsForSeed(t, 0)

	result, err := NaturalBirthReplyFor(NaturalMechanismV8, patches, 5, true, draws, 2017, 1, 2, proposerRequest, consenter)
	if err != nil {
		t.Fatalf("v8 household reply: %v", err)
	}
	if result.Decision != NaturalReplyAccept {
		t.Fatalf("v8 household consenter refused: %+v", result)
	}

	// The same shape under v6 is refused at the consenter's own wealth gate.
	request6, err := NaturalBirthRequestFor(NaturalMechanismV6, patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	v6Result, err := NaturalBirthReplyFor(NaturalMechanismV6, patches, 5, true, draws, 2017, 1, 2, request6, consenter)
	if err != nil {
		t.Fatalf("v6 household reply: %v", err)
	}
	if v6Result.Decision != NaturalReplyRefuse || v6Result.Reason != NaturalRefusalConsentOwn {
		t.Fatalf("v6 household consenter: %+v, want consent-own refusal", v6Result)
	}
	v5Result, err := NaturalBirthReply(patches, 5, true, draws, 2017, 1, 2, request6, consenter)
	if err != nil || v5Result.Decision != NaturalReplyRefuse {
		t.Fatalf("v5 wrapper household consenter: %+v %v, want refusal", v5Result, err)
	}
}

// TestNaturalV8BandWideMatePool proves the second mechanism-v8 rule change: a
// cross-patch pair conceives under v8 (the co-location gate is gone) and is
// refused cross-patch under v6.
func TestNaturalV8BandWideMatePool(t *testing.T) {
	patches := 2
	// Founder 1 lives on patch 1001, founder 9 on patch 1002.
	proposer := naturalGateParent(t, patches, 1)
	consenter := naturalGateParent(t, patches, 9)
	if proposer.Bag.Source == consenter.Bag.Source {
		t.Fatal("fixture parents share a patch")
	}
	draws := naturalDrawsForSeed(t, 0)

	request8, err := NaturalBirthRequestFor(NaturalMechanismV8, patches, 5, 1, 9, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NaturalBirthReplyFor(NaturalMechanismV8, patches, 5, true, draws, 2017, 1, 9, request8, consenter)
	if err != nil {
		t.Fatalf("v8 cross-patch reply: %v", err)
	}
	if result.Decision != NaturalReplyAccept {
		t.Fatalf("v8 cross-patch pair refused: %+v", result)
	}
	if result.Newborn.Bag.Source != proposer.Bag.Source {
		t.Fatalf("newborn home %d, want the proposer's patch %d", result.Newborn.Bag.Source, proposer.Bag.Source)
	}

	request6, err := NaturalBirthRequestFor(NaturalMechanismV6, patches, 5, 1, 9, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	v6Result, err := NaturalBirthReplyFor(NaturalMechanismV6, patches, 5, true, draws, 2017, 1, 9, request6, consenter)
	if err != nil {
		t.Fatalf("v6 cross-patch reply: %v", err)
	}
	if v6Result.Decision != NaturalReplyRefuse || v6Result.Reason != NaturalRefusalCrossPatch {
		t.Fatalf("v6 cross-patch pair: %+v, want cross-patch refusal", v6Result)
	}
}
