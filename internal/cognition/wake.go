package cognition

import "agentworld/internal/cognition/protocol"

// The C7 wake-policy object lives in internal/cognition/protocol so
// internal/world can consume it without importing the model client. These
// aliases and wrappers keep the historical cognition spelling.
type WakePolicy = protocol.WakePolicy

var ErrInvalidWakePolicy = protocol.ErrInvalidWakePolicy

// VS1WakePolicy is the declared VS1 instance: uniform over actors, periodic in
// time, every 6 hours from phase 0.
func VS1WakePolicy() WakePolicy { return protocol.VS1WakePolicy() }

// FullWakePolicy is the declared high-budget reference instance (period 1).
func FullWakePolicy() WakePolicy { return protocol.FullWakePolicy() }

// The VS2 priority/allocation surface (spec Appendix A.1/A.2) keeps the same
// historical cognition spelling: consumers declare a priority policy and consume
// a selection without importing the transport-free protocol package directly,
// exactly as WakePolicy above is re-exported.
type (
	PriorityPolicy      = protocol.PriorityPolicy
	PriorityStrategy    = protocol.PriorityStrategy
	ValueModelSpec      = protocol.ValueModelSpec
	ValueFeatures       = protocol.ValueFeatures
	FeatureWeight       = protocol.FeatureWeight
	CostProxy           = protocol.CostProxy
	CallPrice           = protocol.CallPrice
	CandidateScore      = protocol.CandidateScore
	PolicyPredicate     = protocol.PolicyPredicate
	FeatureThreshold    = protocol.FeatureThreshold
	EscalationCandidate = protocol.EscalationCandidate
	EscalationReason    = protocol.EscalationReason
	Selection           = protocol.Selection
	BudgetState         = protocol.BudgetState
	EscalationSelector  = protocol.EscalationSelector
)

// The closed declared strategy set (spec Appendix A.1).
const (
	StrategyPeriodic      = protocol.StrategyPeriodic
	StrategyUniformRandom = protocol.StrategyUniformRandom
	StrategyAdaptive      = protocol.StrategyAdaptive
	StrategyFull          = protocol.StrategyFull
)

// The declared fixed-point scales (spec Appendix A.1/C.2).
const (
	ExplorationScale = protocol.ExplorationScale
	PriorityScale    = protocol.PriorityScale
)
