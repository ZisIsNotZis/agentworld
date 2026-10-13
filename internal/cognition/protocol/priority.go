package protocol

import "agentworld/internal/sim"

// The VS2 priority/allocation extension surface (spec section 4, Appendix A.1).
// It extends VS1's WakePolicy additively: a nil Priority Policy is the VS1 path,
// and the value projection and selection are pure integer fixed-point functions
// that consume no world random stream.

// PriorityStrategy is the closed set of declared allocation strategies.
type PriorityStrategy string

const (
	// StrategyPeriodic is the period-gated baseline P (no per-hour quota).
	StrategyPeriodic PriorityStrategy = "periodic"
	// StrategyUniformRandom is the uniform-random baseline U.
	StrategyUniformRandom PriorityStrategy = "uniform-random"
	// StrategyAdaptive is the adaptive-priority candidate under test A.
	StrategyAdaptive PriorityStrategy = "adaptive"
	// StrategyFull is the declared full reference R (every candidate).
	StrategyFull PriorityStrategy = "full"
)

// ExplorationScale is the fixed-point denominator of ExplorationFraction: with
// q = ExplorationFraction/ExplorationScale, the reserved exploration calls are
// ceil(q*N) = (N*ExplorationFraction + ExplorationScale - 1) / ExplorationScale
// in integer arithmetic (spec section 4.5).
const ExplorationScale int64 = 1_000_000

// PriorityScale is the fixed-point denominator of Priority (Appendix C.2):
// Priority = ExpectedValue * PriorityScale / max(1, ExpectedCost).
const PriorityScale int64 = 1_000_000

// PriorityPolicy is the declared allocation instance carried by a non-nil
// WakePolicy.Priority. It holds only declared run identity: the strategy, the
// integer fixed-point exploration fraction, the policy PRNG seed, the value
// model, and the declared call ceiling N.
type PriorityPolicy struct {
	Strategy            PriorityStrategy `json:"strategy"`
	ExplorationFraction int64            `json:"exploration_fraction"` // q = frac/ExplorationScale, q in [0,1)
	RandSeed            uint64           `json:"rand_seed"`            // policy PRNG seed, from run identity
	ValueModel          ValueModelSpec   `json:"value_model"`
	CallCeiling         uint64           `json:"call_ceiling"` // declared call ceiling N; 0 for the reference
}

// PolicyPredicate is the versioned declared escalation-request predicate
// evaluated over the actor's own observation (spec section 4.3): integer
// fixed-point and observation-only.
type PolicyPredicate struct {
	Version    string             `json:"version"`
	Thresholds []FeatureThreshold `json:"thresholds"`
}

// FeatureThreshold is one declared fixed-point feature threshold of a
// PolicyPredicate.
type FeatureThreshold struct {
	Feature   string
	Threshold int64 // integer fixed-point
}

// BudgetState is the declared budget view carried into and out of Select.
type BudgetState struct {
	CallsSpent     uint64 `json:"calls_spent"`
	CallsRemaining uint64 `json:"calls_remaining"`
	CallsBudget    uint64 `json:"calls_budget"` // N
}

// EscalationReason is the typed reason recorded on an escalation request and,
// when it leads to an allocation decision, on the allocation record.
type EscalationReason string

const (
	// EscalationRequestedValue is a high-value candidate.
	EscalationRequestedValue EscalationReason = "high-value"
	// EscalationRequestedByActor is the actor's own explicit request.
	EscalationRequestedByActor EscalationReason = "actor-requested"
	// EscalationExploration is a protected-exploration slot.
	EscalationExploration EscalationReason = "protected-exploration"
)

// EscalationCandidate is one roster candidate: the actor, the hour, its
// observation-derived feature vector (plus the declared world-level aggregate
// PriorReliability), and the actor-raised request and its typed reason.
type EscalationCandidate struct {
	Actor     sim.EntityID     `json:"actor"`
	Hour      int              `json:"hour"`
	Features  ValueFeatures    `json:"features"`
	Requested bool             `json:"requested"`
	Reason    EscalationReason `json:"reason"`
}

// Selection is one hour's recorded selection and accumulator state. Quota and
// Shortfall are the declared per-hour quota q(h) and max(0, q(h)-len(roster))
// for U and A; for the period-gated P, Quota is the remaining budget
// (N - CallsSpent) and Shortfall is 0 (spec section 5.1).
type Selection struct {
	Hour        int              `json:"hour"`
	Selected    []sim.EntityID   `json:"selected"`
	Exploration []sim.EntityID   `json:"exploration"`
	Scores      []CandidateScore `json:"scores"` // per candidate, recorded
	Budget      BudgetState      `json:"budget"`
	Quota       int              `json:"quota"`     // U/A: declared per-hour quota q(h); P: remaining budget (5.1)
	Shortfall   int              `json:"shortfall"` // U/A: max(0, q(h) - len(roster)); P: 0 (5.1)
	PRNGPos     uint64           `json:"prng_pos"`  // consumed position of the policy PRNG
}

// EscalationSelector is the deterministic arm-local online accumulator (spec
// section 4.1 item 4). It is seeded from the declared call ceiling N
// (BudgetState.CallsBudget) and RandSeed; Budget.CallsSpent and PRNGPos are
// recorded run state, so replay re-derives the identical selection sequence. It
// is online: it consumes each hour's roster as it arrives, spends at most the
// declared per-hour quota q(h) for U and A (or is period-gated for P), and stops
// once it has spent N. It needs no run-total candidate count and never exceeds N.
type EscalationSelector interface {
	Select(hour int, roster []EscalationCandidate, budget BudgetState) (Selection, error)
}
