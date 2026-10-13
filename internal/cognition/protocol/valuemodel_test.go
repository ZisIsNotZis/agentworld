package protocol

import "testing"

// vs2TestValueModel is the declared Appendix C.2 frozen instance, used only by
// the hermetic projection tests (the instance itself remains operator-
// confirmable at spec section 11).
func vs2TestValueModel() ValueModelSpec {
	return ValueModelSpec{
		Version: "vs2-value/1",
		Weights: []FeatureWeight{
			{Feature: "decision_sensitivity", Weight: 4},
			{Feature: "urgency", Weight: 3},
			{Feature: "impact", Weight: 2},
			{Feature: "propagation_risk", Weight: 2},
			{Feature: "prior_reliability", Weight: 1},
			{Feature: "escalation_requested", Weight: 5},
		},
		CostProxy: CostProxy{
			Version:                   "vs2-cost-proxy/1",
			ConstantMicrosPerCall:     1,
			ProjectedPromptTokens:     10,
			ProjectedCompletionTokens: 4,
		},
	}
}

func vs2TestFeatures() ValueFeatures {
	return ValueFeatures{
		Energy:              20,
		Hunger:              3,
		Capital:             7,
		GranaryStock:        9,
		LastStoredMealHour:  40,
		StockedHomeSlots:    2,
		LastDenial:          "gather",
		DecisionSensitivity: 10,
		Urgency:             5,
		Impact:              4,
		PropagationRisk:     3,
		PriorReliability:    2,
		EscalationRequested: 1,
	}
}

// TestExpectedValueSumsDeclaredWeights pins the declared integer fixed-point
// weighted sum on the Appendix C.2 weights and feature names.
func TestExpectedValueSumsDeclaredWeights(t *testing.T) {
	// 4*10 + 3*5 + 2*4 + 2*3 + 1*2 + 5*1 = 76.
	if got := ExpectedValue(vs2TestValueModel(), vs2TestFeatures()); got != 76 {
		t.Fatalf("ExpectedValue = %d, want 76", got)
	}
}

// TestExpectedValueUnknownFeatureAndZeroWeights pins the two reductions: an
// unknown feature name contributes nothing, and an all-zero weight vector yields
// zero for every candidate (the uniform-random reduction of Appendix C.2).
func TestExpectedValueUnknownFeatureAndZeroWeights(t *testing.T) {
	unknown := ValueModelSpec{Version: "test/unknown", Weights: []FeatureWeight{{Feature: "not_a_feature", Weight: 7}}}
	if got := ExpectedValue(unknown, vs2TestFeatures()); got != 0 {
		t.Fatalf("unknown feature contributed %d, want 0", got)
	}

	zero := ValueModelSpec{Version: "test/zero", Weights: []FeatureWeight{
		{Feature: "decision_sensitivity", Weight: 0},
		{Feature: "urgency", Weight: 0},
		{Feature: "escalation_requested", Weight: 0},
	}}
	f := vs2TestFeatures()
	if got := ExpectedValue(zero, f); got != 0 {
		t.Fatalf("all-zero weights gave ExpectedValue %d, want 0", got)
	}
	// All candidates tie on the projection, so priority cannot rank them; the
	// uniform-random baseline is the reduction.
	if got := Priority(zero, f, CallPrice{}); got != 0 {
		t.Fatalf("all-zero weights gave Priority %d, want 0", got)
	}
}

// TestExpectedCostProxyAndPriced pins the unpriced proxy versus the priced
// cost-over-projected-usage.
func TestExpectedCostProxyAndPriced(t *testing.T) {
	m, f := vs2TestValueModel(), vs2TestFeatures()
	// Unpriced: declared proxy, ConstantMicrosPerCall + projected tokens = 1 + 10 + 4.
	if got := ExpectedCost(m, f, CallPrice{}); got != 15 {
		t.Fatalf("unpriced ExpectedCost = %d, want 15", got)
	}
	// Priced: 2*10 + 5*4 = 40 over the proxy's projected usage.
	priced := CallPrice{Version: "vs2-price/1", PromptMicrosPerToken: 2, CompletionMicrosPerToken: 5}
	if got := ExpectedCost(m, f, priced); got != 40 {
		t.Fatalf("priced ExpectedCost = %d, want 40", got)
	}
}

// TestPriorityFixedPointRatio pins Priority = ExpectedValue*PriorityScale /
// max(1, ExpectedCost) in integer arithmetic.
func TestPriorityFixedPointRatio(t *testing.T) {
	m, f := vs2TestValueModel(), vs2TestFeatures()
	if got := Priority(m, f, CallPrice{}); got != 76*PriorityScale/15 {
		t.Fatalf("unpriced Priority = %d, want %d", got, 76*PriorityScale/15)
	}
	priced := CallPrice{Version: "vs2-price/1", PromptMicrosPerToken: 2, CompletionMicrosPerToken: 5}
	if got := Priority(m, f, priced); got != 76*PriorityScale/40 {
		t.Fatalf("priced Priority = %d, want %d", got, 76*PriorityScale/40)
	}
}

// TestPriorityZeroCostFlooredAtOne pins that an unpriced zero-cost proxy cannot
// divide by zero: the declared max(1, ExpectedCost) floor binds.
func TestPriorityZeroCostFlooredAtOne(t *testing.T) {
	m := ValueModelSpec{Version: "test/zero-cost", Weights: []FeatureWeight{{Feature: "urgency", Weight: 3}}}
	f := ValueFeatures{Urgency: 2}
	if got, want := Priority(m, f, CallPrice{}), int64(6)*PriorityScale; got != want {
		t.Fatalf("zero-cost Priority = %d, want %d", got, want)
	}
}
