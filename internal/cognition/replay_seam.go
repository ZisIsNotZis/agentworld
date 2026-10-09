package cognition

import (
	"context"
	"fmt"

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
	replay *Replay
	wake   protocol.WakePolicy
	served int
}

// NewReplaySeam validates and builds the ledger-resolving seam.
func NewReplaySeam(replay *Replay, wake protocol.WakePolicy) (*ReplaySeam, error) {
	if replay == nil {
		return nil, fmt.Errorf("%w: replay source is required", ErrRoleASeam)
	}
	if err := wake.Validate(); err != nil {
		return nil, err
	}
	return &ReplaySeam{replay: replay, wake: wake}, nil
}

// WakePolicy is the declared escalation policy the recorded run used.
func (s *ReplaySeam) WakePolicy() protocol.WakePolicy { return s.wake }

// DecideActorSeam re-presents the exact canonical request and returns the
// recorded used attempt's validated decision. A miss is the hard
// ErrLedgerMiss, never a model call.
func (s *ReplaySeam) DecideActorSeam(_ context.Context, req protocol.ActorSeamRequest) (protocol.ActorSeamDecision, error) {
	canonical, err := EncodeActorSeamRequest(req)
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	result, err := s.replay.Resolve(canonical)
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	decision, err := ValidateActorSeamContent(result.Content, req)
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	decision.CallRef = result.Seq
	s.served++
	return decision, nil
}

// ServedCalls reports how many recorded calls this seam resolved, for tests
// that assert replay performed zero model calls.
func (s *ReplaySeam) ServedCalls() int { return s.served }
