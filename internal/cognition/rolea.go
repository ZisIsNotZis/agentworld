package cognition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"agentworld/internal/cognition/protocol"
)

// The Role A declared decision seam (spec section 4). The seam decides the
// actor's claim-instant action; the world's existing paths validate and commit
// it. This file owns the level-1 validation ladder (schema and admissibility,
// from the observation alone) and the recorded seam that wraps the encoder,
// the recorded-call ledger, and the level-1 check: exactly one model call and
// exactly one intervention record per escalated claim decision.
//
// A level-1 failure never mutates the world: it returns the declared fallback
// (the frozen cascade's own choice, substituted by the caller) with a typed
// reason, counted in the decision-error rate. A level-3 transport failure is
// the fail-loud ErrModelUnavailable halt from the Recorder; the ledger is
// persisted and the caller does nothing.
var (
	ErrRoleASeam            = errors.New("role A seam")
	ErrRoleASeamNoLedger    = errors.New("role A seam requires a recorded-call ledger")
	ErrRoleASeamObservation = errors.New("role A seam observation")
)

// Source values recorded on ActorSeamDecision.Source (the transport-free
// protocol constants).
const (
	SourceModel    = protocol.SourceModel
	SourceFallback = protocol.SourceFallback
)

// FallbackReason is the typed level-1 fallback reason (spec section 4.3).
type FallbackReason string

const (
	FallbackReasonMalformed    FallbackReason = "malformed"
	FallbackReasonInadmissible FallbackReason = "inadmissible"
)

// The declared Role A seam instant: the claim runs at h·3600s + 1µs of the
// frozen hour lattice (spec section 4.1). It is recorded on the intervention
// cause; it is not a new phase instant.
const (
	roleAHourMicros  = 3_600_000_000
	roleAClaimOffset = 1
)

// RoleASeamConfig wires the recorded Role A seam. Recorder and Ledger are
// required: the seam cannot be entered without the recorded-call path (the
// call is flushed before its result is consumed and every escalated decision
// appends exactly one intervention record).
type RoleASeamConfig struct {
	Recorder          *Recorder
	Ledger            *Ledger
	Endpoint          string
	CredentialPresent bool
	WakePolicy        WakePolicy
}

// RoleASeam is the concrete recorded Role A decider. It implements the
// DecideActorSeam half of the cognition protocol; the Role B proposal method
// belongs to the patch lane.
type RoleASeam struct {
	recorder          *Recorder
	ledger            *Ledger
	endpoint          string
	credentialPresent bool
	wakePolicy        WakePolicy
}

// NewRoleASeam validates and builds the recorded seam.
func NewRoleASeam(cfg RoleASeamConfig) (*RoleASeam, error) {
	if cfg.Recorder == nil || cfg.Ledger == nil {
		return nil, ErrRoleASeamNoLedger
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("%w: endpoint is required", ErrRoleASeam)
	}
	if err := cfg.WakePolicy.Validate(); err != nil {
		return nil, err
	}
	return &RoleASeam{
		recorder:          cfg.Recorder,
		ledger:            cfg.Ledger,
		endpoint:          cfg.Endpoint,
		credentialPresent: cfg.CredentialPresent,
		wakePolicy:        cfg.WakePolicy,
	}, nil
}

// WakePolicy is the declared escalation policy this seam was built under.
func (s *RoleASeam) WakePolicy() WakePolicy { return s.wakePolicy }

// DecideActorSeam encodes the canonical request, performs exactly one recorded
// call, runs the level-1 ladder, and appends exactly one intervention record.
// It returns the model decision when the output is admissible, and the typed
// fallback otherwise; a transport failure is the fail-loud ErrModelUnavailable
// error, never a substituted decision.
func (s *RoleASeam) DecideActorSeam(ctx context.Context, req ActorSeamRequest) (ActorSeamDecision, error) {
	if s == nil || s.recorder == nil || s.ledger == nil {
		return ActorSeamDecision{}, ErrRoleASeamNoLedger
	}
	canonical, err := EncodeActorSeamRequest(req)
	if err != nil {
		return ActorSeamDecision{}, err
	}
	call, err := s.recorder.Call(ctx, CallRequest{
		Role: RoleActorDecision, Hour: req.Hour, Model: DefaultModelID,
		Endpoint: s.endpoint, CredentialPresent: s.credentialPresent,
		Sampling: FrozenSampling(), Request: canonical,
	})
	if err != nil {
		return ActorSeamDecision{}, err
	}
	decision, err := ValidateActorSeamContent(call.Content, req)
	if err != nil {
		return ActorSeamDecision{}, err
	}
	decision.CallRef = call.Seq
	if _, err := AppendRoleAIntervention(s.ledger, RoleAInterventionInput{
		WakePolicy: s.wakePolicy, Seam: req.Seam, Actor: req.Actor, Hour: req.Hour,
		SimTimeMicros: roleASimTimeMicros(req.Hour), Ref: req.Identity.HeadRef,
		Decision: decision, Call: call,
	}); err != nil {
		return ActorSeamDecision{}, err
	}
	return decision, nil
}

func roleASimTimeMicros(hour int) int64 {
	return int64(hour)*roleAHourMicros + roleAClaimOffset
}

// actorDecisionContent is the declared actor-decision response schema
// (spec section 6.3). Unknown fields are a schema violation.
type actorDecisionContent struct {
	SchemaVersion *int     `json:"schema_version"`
	Action        string   `json:"action"`
	Rationale     string   `json:"rationale"`
	Confidence    *float64 `json:"confidence"`
}

// ValidateActorSeamContent is the level-1 ladder: schema and admissibility
// from the observation alone. It never applies the frozen policy's own energy
// thresholds; overriding those is the point of the seam. A malformed or
// inadmissible output yields the declared fallback with its typed reason and
// an empty action; a valid output yields the model decision. An error means
// the request's own observation is undecodable (a caller defect), never a
// model outcome.
func ValidateActorSeamContent(content []byte, req ActorSeamRequest) (ActorSeamDecision, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	var body actorDecisionContent
	if err := dec.Decode(&body); err != nil {
		return fallbackDecision(FallbackReasonMalformed), nil
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fallbackDecision(FallbackReasonMalformed), nil
	}
	if body.SchemaVersion == nil || *body.SchemaVersion != ActorSeamSchemaVersion ||
		body.Action == "" || len(body.Rationale) > 512 ||
		(body.Confidence != nil && (*body.Confidence < 0 || *body.Confidence > 1)) {
		return fallbackDecision(FallbackReasonMalformed), nil
	}
	admissible, err := admissibleAction(body.Action, req)
	if err != nil {
		return ActorSeamDecision{}, err
	}
	if !admissible {
		return fallbackDecision(FallbackReasonInadmissible), nil
	}
	confidence := 0.0
	if body.Confidence != nil {
		confidence = *body.Confidence
	}
	return ActorSeamDecision{
		Action: body.Action, Rationale: body.Rationale, Confidence: confidence,
		Source: SourceModel,
	}, nil
}

func fallbackDecision(reason FallbackReason) ActorSeamDecision {
	return ActorSeamDecision{Source: SourceFallback, Reason: string(reason)}
}

// admissibleAction checks only what the observation makes impossible. gather
// requires at least one stocked home slot; eat-stored requires granary >= 1
// and no stored meal yet this hour; provision additionally requires an empty
// bag; wait is always admissible. The action must also be in the seam's legal
// set (a property of the seam and mechanism, not the bound ref).
func admissibleAction(action string, req ActorSeamRequest) (bool, error) {
	legal := false
	for _, candidate := range req.Actions {
		if candidate == action {
			legal = true
			break
		}
	}
	if !legal {
		return false, nil
	}
	granary, err := req.Observation.GranaryStock.Integer()
	if err != nil {
		return false, fmt.Errorf("%w: granary_stock", ErrRoleASeamObservation)
	}
	lastMeal, err := req.Observation.LastStoredMealHour.Integer()
	if err != nil {
		return false, fmt.Errorf("%w: last_stored_meal_hour", ErrRoleASeamObservation)
	}
	bag, err := req.Observation.BagUnits.Integer()
	if err != nil {
		return false, fmt.Errorf("%w: bag_units", ErrRoleASeamObservation)
	}
	switch action {
	case "gather":
		for _, slot := range req.Observation.HomeSlots {
			stock, err := slot.Stock.Integer()
			if err != nil {
				return false, fmt.Errorf("%w: home_slot stock", ErrRoleASeamObservation)
			}
			if stock >= 1 {
				return true, nil
			}
		}
		return false, nil
	case "eat-stored":
		return granary >= 1 && lastMeal < int64(req.Hour), nil
	case "provision":
		return bag == 0 && granary >= 1 && lastMeal < int64(req.Hour), nil
	case "wait":
		return true, nil
	default:
		return false, nil
	}
}

// RoleASeam implements the Role A half of the sim/strategy-typed seam the
// world consumes.
var _ = func(s *RoleASeam) interface {
	DecideActorSeam(context.Context, ActorSeamRequest) (ActorSeamDecision, error)
} {
	return s
}
