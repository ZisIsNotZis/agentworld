package protocol

import (
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The VS2 feature-derivation tests (spec Appendix C.1, section 4.2/4.3): every
// derived row is non-vacuous on a fixture, each is monotone in its declared
// direction, the last_denial encoding is pinned, and the escalation predicate
// is observation-only (it can never read the world-level prior_reliability row).

func featureObs(energy, hunger, capital, granary, lastMeal int64, stocked int, hour int, denial strategy.NaturalDenial) strategy.NaturalObservation {
	slots := make([]strategy.NaturalSlotView, 0, stocked)
	for i := 0; i < stocked; i++ {
		slots = append(slots, strategy.NaturalSlotView{ID: sim.EntityID(2001 + i), Stock: sim.IntegerValue(1)})
	}
	return strategy.NaturalObservation{
		Energy:             sim.IntegerValue(energy),
		Hunger:             sim.IntegerValue(hunger),
		Capital:            sim.IntegerValue(capital),
		GranaryStock:       sim.IntegerValue(granary),
		LastStoredMealHour: sim.IntegerValue(lastMeal),
		Hour:               hour,
		LastDenial:         denial,
		HomeSlots:          slots,
	}
}

// TestObservationFeaturesRawRowsAndZeroSchedulerRows pins the observation-only
// projection: the raw rows and stocked-slot count are read directly, the two
// scheduler-level rows (prior_reliability, escalation_requested) are left zero
// so the escalation predicate cannot read them.
func TestObservationFeaturesRawRowsAndZeroSchedulerRows(t *testing.T) {
	got := ObservationFeatures(featureObs(7, 3, 5, 2, 9, 2, 9, strategy.NaturalNoDenial))
	if got.Energy != 7 || got.Hunger != 3 || got.Capital != 5 || got.GranaryStock != 2 || got.LastStoredMealHour != 9 {
		t.Fatalf("raw rows = %+v", got)
	}
	if got.StockedHomeSlots != 2 {
		t.Fatalf("stocked home slots = %d, want 2", got.StockedHomeSlots)
	}
	if got.PriorReliability != 0 || got.EscalationRequested != 0 {
		t.Fatalf("scheduler rows must project as zero, got prior=%d requested=%d", got.PriorReliability, got.EscalationRequested)
	}
}

// TestObservationFeaturesDerivedRowsNonVacuous pins that every derived row is
// exercised with a non-trivial fixture value (not a vacuous "always zero"
// derivation).
func TestObservationFeaturesDerivedRowsNonVacuous(t *testing.T) {
	mid := ObservationFeatures(featureObs(6, 12, 2, 3, 4, 4, 5, strategy.NaturalNoDenial))
	if mid.Urgency != 500 {
		t.Fatalf("urgency = %d, want 500", mid.Urgency)
	}
	if mid.Impact != DerivedFeatureScale {
		t.Fatalf("impact = %d, want %d (consent gate met)", mid.Impact, DerivedFeatureScale)
	}
	if mid.PropagationRisk != 500 {
		t.Fatalf("propagation_risk = %d, want 500", mid.PropagationRisk)
	}
	if mid.DecisionSensitivity == 0 {
		t.Fatal("decision_sensitivity is zero on a boundary-adjacent fixture")
	}

	saturated := ObservationFeatures(featureObs(0, 24, 2, 3, -1, 8, 5, strategy.NaturalNoDenial))
	if saturated.Urgency != DerivedFeatureScale {
		t.Fatalf("saturated urgency = %d, want %d", saturated.Urgency, DerivedFeatureScale)
	}
	if saturated.PropagationRisk != DerivedFeatureScale {
		t.Fatalf("saturated propagation_risk = %d, want %d", saturated.PropagationRisk, DerivedFeatureScale)
	}
}

// TestObservationFeaturesMonotoneDirection pins the declared direction of every
// derived row: expected value is monotone in each (section 4.4), so the derived
// rows themselves carry the declared direction.
func TestObservationFeaturesMonotoneDirection(t *testing.T) {
	// urgency: non-increasing in energy, non-decreasing in hunger.
	if derivedUrgency(3, 10) <= derivedUrgency(10, 10) {
		t.Fatal("urgency did not decrease as energy rose")
	}
	if derivedUrgency(10, 5) >= derivedUrgency(10, 20) {
		t.Fatal("urgency did not increase as hunger rose")
	}
	// impact: non-decreasing in capital and granary.
	if derivedImpact(1, 3) >= derivedImpact(2, 3) {
		t.Fatal("impact did not increase with capital")
	}
	if derivedImpact(2, 1) >= derivedImpact(2, 3) {
		t.Fatal("impact did not increase with granary")
	}
	// propagation_risk: non-decreasing in stocked slots.
	if derivedPropagationRisk(1) >= derivedPropagationRisk(6) {
		t.Fatal("propagation_risk did not increase with stocked slots")
	}
	// decision_sensitivity: higher when a value sits nearer a frozen threshold.
	near := derivedDecisionSensitivity(5, 0, 0, 0, 0, 0, 100)
	far := derivedDecisionSensitivity(9, 10, 4, 5, 50, 4, 100)
	if far != 0 {
		t.Fatalf("decision_sensitivity far from every threshold = %d, want 0", far)
	}
	if near <= far {
		t.Fatalf("decision_sensitivity did not rise near a threshold: near=%d far=%d", near, far)
	}
}

// TestObservationFeaturesDenialEncoding pins the declared last_denial string
// encoding.
func TestObservationFeaturesDenialEncoding(t *testing.T) {
	cases := []struct {
		denial strategy.NaturalDenial
		want   string
	}{
		{strategy.NaturalNoDenial, ""},
		{strategy.NaturalGatherDenied, "gather"},
		{strategy.NaturalBuildDenied, "build"},
	}
	for _, c := range cases {
		if got := ObservationFeatures(featureObs(6, 12, 2, 3, 0, 0, 5, c.denial)).LastDenial; got != c.want {
			t.Fatalf("last_denial(%d) = %q, want %q", c.denial, got, c.want)
		}
	}
}

// TestEscalationRequestObservationOnly pins the predicate's observation
// boundary (C5/EC-6): it evaluates the observation-derived rows, fires on the
// declared direction, and fails closed for the scheduler-level
// prior_reliability row or an unknown feature, so it can never read the
// world-level aggregate.
func TestEscalationRequestObservationOnly(t *testing.T) {
	urgent := PolicyPredicate{Version: "test", Thresholds: []FeatureThreshold{{Feature: "urgency", Threshold: 500}}}
	if ok, reason := EscalationRequest(urgent, featureObs(6, 12, 0, 0, 0, 0, 5, strategy.NaturalNoDenial)); !ok || reason != EscalationRequestedByActor {
		t.Fatalf("urgent observation: ok=%t reason=%q, want a request", ok, reason)
	}
	if ok, _ := EscalationRequest(urgent, featureObs(12, 0, 0, 0, 0, 0, 5, strategy.NaturalNoDenial)); ok {
		t.Fatal("non-urgent observation requested escalation")
	}

	// prior_reliability is a scheduler-level row: naming it fails closed even
	// at a threshold every value would meet.
	priorRow := PolicyPredicate{Version: "test", Thresholds: []FeatureThreshold{{Feature: "prior_reliability", Threshold: 0}}}
	if ok, _ := EscalationRequest(priorRow, featureObs(6, 12, 0, 0, 0, 0, 5, strategy.NaturalNoDenial)); ok {
		t.Fatal("a predicate naming prior_reliability was evaluated observation-only")
	}
	unknown := PolicyPredicate{Version: "test", Thresholds: []FeatureThreshold{{Feature: "not_a_feature", Threshold: 0}}}
	if ok, _ := EscalationRequest(unknown, featureObs(6, 12, 0, 0, 0, 0, 5, strategy.NaturalNoDenial)); ok {
		t.Fatal("a predicate naming an unknown feature was accepted")
	}
	empty := PolicyPredicate{Version: "test"}
	if ok, _ := EscalationRequest(empty, featureObs(0, 24, 0, 0, 0, 0, 0, strategy.NaturalNoDenial)); ok {
		t.Fatal("an empty predicate requested escalation")
	}
}

// TestEscalationRequestDeclaredInstanceExercisesDerivedRows pins that the
// declared predicate instance is non-vacuous: it fires on a maximally urgent,
// boundary-adjacent observation and not on a comfortable one.
func TestEscalationRequestDeclaredInstanceExercisesDerivedRows(t *testing.T) {
	p := DeclaredEscalationPredicate()
	if p.Version != EscalationPredicateVersion || len(p.Thresholds) == 0 {
		t.Fatalf("declared predicate = %+v", p)
	}
	if ok, _ := EscalationRequest(p, featureObs(5, 24, 2, 3, 5, 1, 5, strategy.NaturalNoDenial)); !ok {
		t.Fatal("declared predicate did not fire on a maximally urgent observation")
	}
	if ok, _ := EscalationRequest(p, featureObs(12, 0, 8, 8, 0, 0, 5, strategy.NaturalNoDenial)); ok {
		t.Fatal("declared predicate fired on a comfortable observation")
	}
}
