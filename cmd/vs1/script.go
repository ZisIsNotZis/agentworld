package main

import (
	"encoding/json"
	"fmt"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
)

// slotWire is the observation home-slot row the scripted policy reads.
type slotWire struct {
	Stock int64 `json:"stock"`
}

// The scripted hermetic responder. It is a pure function of the canonical
// request bytes, so the arms are deterministic and replayable, and it stands in
// for the unauthorised real model: the content is a declared scripted proposal,
// not a model judgment. Two roles share one client, exactly as the recorded
// path shares one Recorder.
//
// Role A ("provision-first"): if a granary unit may be provisioned, do so;
// otherwise take a stored meal; otherwise gather a stocked home slot; otherwise
// wait. This deliberately overrides the frozen cascade's own energy thresholds,
// which is the seam's point, and is the mechanism the f=12 natural/7-versus-7b
// sensitivity datum already found decision-relevant.
//
// Role B: propose the declared single-delta child that lowers
// claim.provision_energy_floor to 3, echoing the request's parent ref.
func scriptedResponder(spec cellSpec) func(cognition.ModelRequest) (cognition.ModelResponse, error) {
	return func(req cognition.ModelRequest) (cognition.ModelResponse, error) {
		var envelope struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(req.Request, &envelope); err != nil {
			return cognition.ModelResponse{}, fmt.Errorf("scripted responder: %w", err)
		}
		var content []byte
		switch envelope.Role {
		case cognition.RoleActorDecision:
			var wire struct {
				Actor struct {
					Hour int `json:"hour"`
				} `json:"actor"`
				Observation struct {
					BagUnits           int64      `json:"bag_units"`
					GranaryStock       int64      `json:"granary_stock"`
					LastStoredMealHour int64      `json:"last_stored_meal_hour"`
					HomeSlots          []slotWire `json:"home_slots"`
				} `json:"observation"`
			}
			if err := json.Unmarshal(req.Request, &wire); err != nil {
				return cognition.ModelResponse{}, fmt.Errorf("scripted responder: %w", err)
			}
			action := scriptedActorAction(wire.Observation.BagUnits, wire.Observation.GranaryStock,
				wire.Observation.LastStoredMealHour, wire.Actor.Hour, wire.Observation.HomeSlots)
			content = []byte(fmt.Sprintf(
				`{"schema_version":1,"action":%q,"rationale":"hermetic scripted provision-first proposal","confidence":1}`,
				action))
		case cognition.RoleStrategyPatch:
			var wire struct {
				ParentRef struct {
					ID      string `json:"id"`
					Version uint32 `json:"version"`
				} `json:"parent_ref"`
			}
			if err := json.Unmarshal(req.Request, &wire); err != nil {
				return cognition.ModelResponse{}, fmt.Errorf("scripted responder: %w", err)
			}
			content = []byte(fmt.Sprintf(
				`{"schema_version":1,"propose":true,"parent_ref":{"id":%q,"version":%d},"deltas":[{"param":%q,"value":3}],"intent":"hermetic scripted lower the provision energy floor"}`,
				wire.ParentRef.ID, wire.ParentRef.Version, protocol.ParamClaimProvisionEnergyFloor))
		default:
			return cognition.ModelResponse{}, fmt.Errorf("scripted responder: unknown role %q", envelope.Role)
		}
		return cognition.ModelResponse{
			Body: content, Content: content,
			Usage:  cognition.Usage{PromptTokens: uint64(len(req.Request)), CompletionTokens: uint64(len(content))},
			Status: cognition.CallStatusOK,
		}, nil
	}
}

func scriptedActorAction(bag, granary, lastStoredMeal int64, hour int, slots []slotWire) string {
	if bag == 0 && granary >= 1 && lastStoredMeal < int64(hour) {
		return "provision"
	}
	if granary >= 1 && lastStoredMeal < int64(hour) {
		return "eat-stored"
	}
	for _, slot := range slots {
		if slot.Stock >= 1 {
			return "gather"
		}
	}
	return "wait"
}

// triggerIndicatorOf mirrors the declared Role B trigger indicator
// (internal/cognition.roleb.go triggerIndicator): the binding problem's own
// reading. The sandbox branch re-derives it from a fork's own evidence so the
// concrete branch can score a candidate without importing the unexported
// function.
func triggerIndicatorOf(evidence protocol.TrajectoryEvidence) int64 {
	switch {
	case evidence.BirthsTotal == 0:
		return evidence.MaxCapital
	case evidence.BuildsCommitted == 0:
		return evidence.BuildsCommitted
	default:
		return evidence.MaxCapital
	}
}
