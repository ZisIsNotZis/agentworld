package protocol

import "agentworld/internal/sim"

// The declared value model (Appendix A.2 / C.2): a versioned, integer
// fixed-point projection of a candidate feature vector. Never prose, never a
// float, never a single scalar objective. The cost type is protocol-local so
// internal/cognition can import this package without an import cycle.

// ValueFeatures is a candidate's feature vector (Appendix C.1). Every field
// except PriorReliability is observation-only and reads the actor's own
// strategy.NaturalObservation; PriorReliability is the declared world-level
// aggregate row (a versioned cell statistic) and carries no other actor's
// private state.
type ValueFeatures struct {
	Energy              int64
	Hunger              int64
	Capital             int64
	GranaryStock        int64
	LastStoredMealHour  int64
	StockedHomeSlots    int64
	LastDenial          string
	DecisionSensitivity int64 // fixed-point; distance to the frozen thresholds
	Urgency             int64
	Impact              int64
	PropagationRisk     int64
	PriorReliability    int64
	EscalationRequested int64 // 0/1, from the actor's own EscalationRequest predicate (4.3)
}

// FeatureWeight is one declared integer fixed-point feature weight.
type FeatureWeight struct {
	Feature string
	Weight  int64
}

// CostProxy is the declared cost model used while the model is unpriced: a
// constant per call plus a projected-token term from the frozen sampling
// instance. It is reported as a proxy, never as spend.
type CostProxy struct {
	Version                   string
	ConstantMicrosPerCall     int64
	ProjectedPromptTokens     uint64
	ProjectedCompletionTokens uint64
}

// ValueModelSpec is the versioned value model: the feature-set version, the
// weights, and the cost proxy. Its version (and the declared feature set it
// projects) is part of run identity (VS2-EC-8): it rides
// WakePolicy.Identity()/Fingerprint(), so the checkpoint cognition block binds
// the feature set alongside the weights (Appendix A.1).
type ValueModelSpec struct {
	Version    string
	FeatureSet string // declared candidate feature-set version (Appendix C.1); "" == unspecified
	Weights    []FeatureWeight
	CostProxy  CostProxy
}

// CallPrice is the protocol-local cost type for the value projection. The
// projection lives in protocol, which internal/cognition imports, so it cannot
// take cognition.PriceTable without an import cycle; the runner adapts a
// declared PriceTable to CallPrice at the boundary. An empty Version means the
// model is unpriced and the declared CostProxy is used instead.
type CallPrice struct {
	Version                  string
	PromptMicrosPerToken     int64
	CompletionMicrosPerToken int64
}

// CandidateScore is the recorded per-candidate projection.
type CandidateScore struct {
	Actor         sim.EntityID `json:"actor"`
	ExpectedValue int64        `json:"expected_value"`
	ExpectedCost  int64        `json:"expected_cost"`
	Priority      int64        `json:"priority"`
	Requested     bool         `json:"requested"`
}

// featureValue maps a declared Appendix C.1 feature name to its scalar feature
// value. An unknown name contributes nothing (weight zero). LastDenial is a
// string input to the derived features, not a weighted one, so it has no case.
func featureValue(name string, f ValueFeatures) int64 {
	switch name {
	case "energy":
		return f.Energy
	case "hunger":
		return f.Hunger
	case "capital":
		return f.Capital
	case "granary_stock":
		return f.GranaryStock
	case "last_stored_meal_hour":
		return f.LastStoredMealHour
	case "stocked_home_slots":
		return f.StockedHomeSlots
	case "decision_sensitivity":
		return f.DecisionSensitivity
	case "urgency":
		return f.Urgency
	case "impact":
		return f.Impact
	case "propagation_risk":
		return f.PropagationRisk
	case "prior_reliability":
		return f.PriorReliability
	case "escalation_requested":
		return f.EscalationRequested
	default:
		return 0
	}
}

// ExpectedValue is the declared pure integer fixed-point expected
// civilizational value: the monotone declared weighted sum of the candidate's
// feature vector. An all-zero weight vector yields 0 for every feature vector,
// which is the pinned uniform-random reduction (Appendix C.2).
func ExpectedValue(m ValueModelSpec, f ValueFeatures) int64 {
	var value int64
	for _, w := range m.Weights {
		value += w.Weight * featureValue(w.Feature, f)
	}
	return value
}

// ExpectedCost is the declared pure integer fixed-point marginal cost of one
// call. With a declared price (non-empty CallPrice.Version) it is the priced
// cost over the proxy's projected usage; while the model is unpriced it is the
// declared CostProxy: the constant per call plus the projected-token term. It is
// a proxy, never recorded spend. It is feature-independent by declaration.
func ExpectedCost(m ValueModelSpec, f ValueFeatures, price CallPrice) int64 {
	promptTokens := int64(m.CostProxy.ProjectedPromptTokens)
	completionTokens := int64(m.CostProxy.ProjectedCompletionTokens)
	if price.Version != "" {
		return price.PromptMicrosPerToken*promptTokens + price.CompletionMicrosPerToken*completionTokens
	}
	return m.CostProxy.ConstantMicrosPerCall + promptTokens + completionTokens
}

// Priority is the declared integer fixed-point ratio (no floats): expected
// value over the marginal call cost, with the cost floored at 1 so an unpriced
// zero-cost proxy cannot divide by zero.
func Priority(m ValueModelSpec, f ValueFeatures, price CallPrice) int64 {
	cost := ExpectedCost(m, f, price)
	if cost < 1 {
		cost = 1
	}
	return ExpectedValue(m, f) * PriorityScale / cost
}
