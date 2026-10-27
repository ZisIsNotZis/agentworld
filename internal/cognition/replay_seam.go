package cognition

import (
	"context"
	"fmt"
	"sync/atomic"

	"agentworld/internal/cognition/protocol"
)

// ReplaySeam resolves every escalated claim instant from the recorded-call
// ledger by request hash and never calls a model (spec section 7.3, ADR 0006
// decision 2). It is the Role A half of the reference replay: the world's
// journal reconciliation and checkpoint restore wire it so a seam-active run
// reproduces its recorded decisions exactly. The returned decision's content
// is re-validated deterministically, exactly as the live seam validated it;
// the reserved fallback for a recorded malformed/inadmissible completion
// reproduces identically.
type ReplaySeam struct {
	replay   *Replay
	wake     protocol.WakePolicy
	contract ClaimContract
	// model is the application model the recorded run addressed; the canonical
	// request bytes carry it, so a replay of a non-default-model run must re-encode
	// with the same model or every resolve is a ledger miss.
	model string
	// served is atomic because a run may resolve several independent calls at
	// once within one simulated hour (the declared claim concurrency, ticket 57).
	served atomic.Int64
}

// NewReplaySeam validates and builds the ledger-resolving seam under the
// declared default contract (natural-claim/v1).
func NewReplaySeam(replay *Replay, wake protocol.WakePolicy) (*ReplaySeam, error) {
	return NewReplaySeamForContract(replay, wake, ClaimContractV1)
}

// NewReplaySeamForContract is NewReplaySeam under an explicit declared claim
// contract. The canonical request bytes depend on the contract, so a replay of
// a natural-claim/v2 or /v4 run must re-encode and re-validate under the
// contract the recorded run used or every resolve is a ledger miss (ticket 57:
// the v4 reference cell could not reconcile its journal otherwise).
func NewReplaySeamForContract(replay *Replay, wake protocol.WakePolicy, contract ClaimContract) (*ReplaySeam, error) {
	return NewReplaySeamForContractModel(replay, wake, contract, DefaultModelID)
}

// NewReplaySeamForContractModel is NewReplaySeamForContract under an explicit
// application model. The model is part of the canonical request bytes, so a
// replay of a run on another model must state it; an empty model is the declared
// default.
func NewReplaySeamForContractModel(replay *Replay, wake protocol.WakePolicy, contract ClaimContract, model string) (*ReplaySeam, error) {
	if replay == nil {
		return nil, fmt.Errorf("%w: replay source is required", ErrRoleASeam)
	}
	if err := wake.Validate(); err != nil {
		return nil, err
	}
	if _, err := ParseClaimContract(string(contract)); err != nil {
		return nil, err
	}
	if model == "" {
		model = DefaultModelID
	}
	return &ReplaySeam{replay: replay, wake: wake, contract: contract, model: model}, nil
}

// WakePolicy is the declared escalation policy the recorded run used.
func (s *ReplaySeam) WakePolicy() protocol.WakePolicy { return s.wake }

// DecideActorSeam re-presents the exact canonical request and returns the
// recorded used attempt's validated decision. A miss is the hard
// ErrLedgerMiss, never a model call.
func (s *ReplaySeam) DecideActorSeam(_ context.Context, req protocol.ActorSeamRequest) (protocol.ActorSeamDecision, error) {
	canonical, err := encodeActorSeamRequestFor(req, s.contract, s.model)
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	result, err := s.replay.Resolve(canonical)
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	decision, err := ValidateActorSeamContentFor(result.Content, req, s.contract.ResponseSchemaVersion())
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	decision.CallRef = result.Seq
	s.served.Add(1)
	return decision, nil
}

// ServedCalls reports how many recorded calls this seam resolved, for tests
// that assert replay performed zero model calls.
func (s *ReplaySeam) ServedCalls() int { return int(s.served.Load()) }
