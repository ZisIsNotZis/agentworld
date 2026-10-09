package main

import (
	"encoding/json"
	"fmt"
	"strings"

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
			deltas, intent := patchedDeltasFor(spec.Arm)
			content = []byte(fmt.Sprintf(
				`{"schema_version":1,"propose":true,"parent_ref":{"id":%q,"version":%d},"deltas":%s,"intent":%q}`,
				wire.ParentRef.ID, wire.ParentRef.Version, encodeScriptedDeltas(deltas), intent))
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

// patchedDeltasFor returns the declared Role B script of an arm. The declared
// arms propose the single provision-floor child (the hermetic honest-null
// proposal the concrete sandbox declines at every declared cell). The
// scripted-proposer fixture proposes the single build-floor delta measured to
// pass the concrete sandbox admission rule at the fixture cell
// (bag.build_energy_floor=2 over the parent's 3), so the apply/measure/rollback
// path is reachable. It is a fixture, not a tuning of the declared arms (see
// FixtureLabel).
func patchedDeltasFor(arm string) ([]protocol.ParameterDelta, string) {
	if arm == ArmA2f {
		return []protocol.ParameterDelta{
			{Param: protocol.ParamBagBuildEnergyFloor, Value: 2},
		}, "scripted-proposer fixture: relax the bag build energy floor"
	}
	return []protocol.ParameterDelta{{Param: protocol.ParamClaimProvisionEnergyFloor, Value: 3}},
		"hermetic scripted lower the provision energy floor"
}

// encodeScriptedDeltas renders a delta set as canonical response JSON.
func encodeScriptedDeltas(deltas []protocol.ParameterDelta) string {
	parts := make([]string, 0, len(deltas))
	for _, d := range deltas {
		parts = append(parts, fmt.Sprintf(`{"param":%q,"value":%d}`, d.Param, d.Value))
	}
	return "[" + strings.Join(parts, ",") + "]"
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
