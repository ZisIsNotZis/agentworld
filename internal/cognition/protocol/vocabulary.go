package protocol

import "agentworld/internal/strategy"

// The declared claim-action vocabulary (natural-claim/v4, ticket 36).
//
// DECLARED EXCEPTION to the one-name-one-thing rule: each claim action has two
// names. The internal ID ("gather", "eat-stored", "provision", "wait") is the
// frozen wire and storage token: it appears in every recorded request, in the
// world's claim-action set, in the role-a intervention records, and in the
// ladder that decides admissibility, so renaming it would invalidate every
// recorded ledger and every strategy fingerprint. The display label is the
// canonical domain name: it states what the action does, and it is what the
// model is asked for and what a reader should use. The two are bound by
// ClaimActionVocabularyTable below and by nothing else: ClaimActionLabel maps
// exactly one ID to exactly one label, ClaimActionID maps it back, and
// TestClaimActionVocabularyIsABijection proves the mapping is total in both
// directions with no duplicate on either side.
//
// The migration path is a version boundary that does not exist yet: at a
// future contract version the internal IDs are renamed to the labels and a
// replay-preserving alias table maps the old tokens onto the new ones, so an
// old ledger still replays. Until then the duplication is declared here, in
// .scratch/36-harness-vocabulary/spec.md and in docs/ARCHITECTURE.md, and it is
// invisible to the model, which only ever sees the label.
type ClaimActionVocabulary struct {
	ID    string
	Label string
}

// ClaimActionVocabularyTable is the one declared binding of the claim-action
// names. Order is the declared claim order (gather, eat-stored, provision,
// wait), which is the order the world's claim-action set uses.
var ClaimActionVocabularyTable = [...]ClaimActionVocabulary{
	{ID: "gather", Label: "take_slot_food_into_bag"},
	{ID: "eat-stored", Label: "eat_granary_food"},
	{ID: "provision", Label: "take_granary_food_into_bag"},
	{ID: "wait", Label: "wait_this_hour"},
}

// ClaimActionBuildLabel is the display label of the frozen bag-decision build
// action. Build is not in the claim-action set (the model never chooses it; the
// frozen cascade decides at h+20m+2µs), so it has no internal claim ID and no
// table row; the label exists so a refused build can be reported in the same
// vocabulary as the actions the actor does choose.
const ClaimActionBuildLabel = "build_capital_point"

// ClaimActionLabel returns the display label of one internal claim-action ID.
func ClaimActionLabel(id string) (string, bool) {
	for _, row := range ClaimActionVocabularyTable {
		if row.ID == id {
			return row.Label, true
		}
	}
	return "", false
}

// ClaimActionID returns the internal claim-action ID of one display label.
func ClaimActionID(label string) (string, bool) {
	for _, row := range ClaimActionVocabularyTable {
		if row.Label == label {
			return row.ID, true
		}
	}
	return "", false
}

// Refused-action values of the request's last_refused_action field. The two
// refusals the world records are a refused gather (NaturalGatherDenied, the
// claim action the actor also chooses) and a refused build
// (NaturalBuildDenied, the frozen bag decision).
const (
	RefusedActionNone = "none"
)

// RefusedActionLabel is the declared wire spelling of one denial in the v4
// vocabulary: the refused action's own display label, so the request never
// carries an internal ID.
func RefusedActionLabel(d strategy.NaturalDenial) string {
	switch d {
	case strategy.NaturalGatherDenied:
		label, _ := ClaimActionLabel("gather")
		return label
	case strategy.NaturalBuildDenied:
		return ClaimActionBuildLabel
	default:
		return RefusedActionNone
	}
}
