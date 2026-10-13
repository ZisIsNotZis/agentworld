package main

import (
	"fmt"

	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// The accepted-history evidence shared by the arms. Everything here is a pure
// function of the published handoff the runner already exposes (spec section
// 5.1): no actor-private state and no world reader.

// worldRuleNames names every natural rule that can appear in accepted history.
// It mirrors cmd/natural's map; an unknown rule is a hard error, never a
// silent bucket.
var worldRuleNames = map[sim.RuleID]string{
	world.NaturalRegenSplitRule:       "regen_split",
	world.NaturalSpoilageRule:         "spoilage",
	world.NaturalWearRule:             "wear",
	world.NaturalBasalRule:            "basal",
	world.NaturalBuildRule:            "build",
	world.NaturalGatherRule:           "gather",
	world.NaturalConsumeRule:          "consume",
	world.NaturalGrantRule:            "grant",
	world.NaturalEatStoredRule:        "eat_stored",
	world.NaturalBirthRequestRule:     "birth_request",
	world.NaturalBirthReplyRule:       "birth_reply",
	world.NaturalBirthRefuseRule:      "birth_refuse",
	world.NaturalBirthExpireRule:      "birth_expiry",
	world.NaturalAgeDeathRule:         "age_death",
	world.NaturalProvisionRule:        "provision",
	world.NaturalLabourOfferRule:      "labour_offer",
	world.NaturalLabourDecisionRule:   "labour_decision",
	world.NaturalLabourSettlementRule: "labour_settlement",
	world.NaturalLabourExpireRule:     "labour_expiry",
}

// acceptedEvidence is the accepted-history identity and event inventory.
type acceptedEvidence struct {
	TipHash        string
	ProjectionHash string
	Version        uint64
	HistoryBytes   int
	EventCount     int
	EventBodyBytes int
}

// extractAccepted replays the accepted history into its event inventory. It
// refuses (fail loud) an unknown rule or a head mismatch.
func extractAccepted(handoff world.NaturalHandoff) (acceptedEvidence, error) {
	reg, err := world.NaturalRegistryFor(world.NaturalMechanismV6)
	if err != nil {
		return acceptedEvidence{}, err
	}
	k, head, err := kernel.RestoreHistory(reg, handoff.History)
	if err != nil {
		return acceptedEvidence{}, fmt.Errorf("accepted history replay: %w", err)
	}
	if head != handoff.Head {
		return acceptedEvidence{}, fmt.Errorf("restored history head mismatch")
	}
	events := k.Events()
	out := acceptedEvidence{
		TipHash:        hexString(head.TipHash[:]),
		ProjectionHash: hexString(head.ProjectionHash[:]),
		Version:        uint64(head.Version),
		HistoryBytes:   len(handoff.History),
		EventCount:     len(events),
	}
	for _, ev := range events {
		body, err := ev.Bytes()
		if err != nil {
			return acceptedEvidence{}, err
		}
		out.EventBodyBytes += len(body)
		if _, ok := worldRuleNames[ev.Rule]; !ok {
			return acceptedEvidence{}, fmt.Errorf("unknown natural rule %d in accepted history", ev.Rule)
		}
	}
	return out, nil
}
