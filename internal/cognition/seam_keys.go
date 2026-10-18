package cognition

// The declared wire key sets of every Role A request contract, in one place.
//
// VS2-EC-6 certifies that "every recorded Role A request carries exactly the
// declared key set and no privileged marker". That check is only meaningful if
// the declaration belongs to the contract that produced the request: while the
// allow-list was a single v1 list, every natural-claim/v4 request failed the key
// check and the scan returned before the marker test, so the boundary check was
// silently disabled for the contract the ticket-36 runs used. The declaration
// therefore lives here, next to the encoders, one row per contract, and
// TestDeclaredSeamRequestKeySetsMatchTheEncoders proves each row equals the key
// set its encoder actually writes for the same request value - so a rename in an
// encoder cannot leave a stale list behind.
//
// The lists are in the encoder's field order (canonical request bytes depend on
// it); a set comparison is made by the callers that only need membership.

// SeamRequestKeySet is one contract's declared allow-list: the exact top-level
// keys of the canonical request and the exact keys of its observation block.
type SeamRequestKeySet struct {
	Contract    ClaimContract
	Request     []string
	Observation []string
}

// v1ObservationKeys is the v1/v2 observation block: the world's own field names,
// including the four rows the v4 audit found inert (`hunger`, `invested_units`,
// `home_patch`, and the timestamp behind each gate boolean).
var v1ObservationKeys = []string{
	"energy", "hunger", "bag_units", "capital", "wip", "invested_units",
	"last_build_hour", "granary_stock", "last_stored_meal_hour", "home_patch", "home_slots", "last_denial",
}

// v4ObservationKeys is the display-vocabulary observation block: every field is
// read by the frozen cascade, by a claim-admission gate, or by the action the
// actor is choosing (the audit is in .scratch/36-harness-vocabulary/spec.md).
var v4ObservationKeys = []string{
	"energy", "bag_food_units", "capital", "build_point_in_progress",
	"build_done_this_hour", "granary_food_units", "granary_draw_used_this_hour",
	"food_units_waiting_to_pick_up", "last_refused_action",
}

// declaredSeamRequestKeySets is the one declaration of every contract's wire
// key sets. A new contract is added here in the same commit as its encoder.
var declaredSeamRequestKeySets = []SeamRequestKeySet{
	{
		Contract:    ClaimContractV1,
		Request:     []string{"schema_version", "role", "seam", "prompt_template", "model", "run", "actor", "observation", "strategy", "actions", "sampling"},
		Observation: v1ObservationKeys,
	},
	{
		Contract:    ClaimContractV2,
		Request:     []string{"schema_version", "role", "seam", "prompt_template", "model", "run", "actor", "observation", "strategy", "actions", "admissible_actions", "sampling"},
		Observation: v1ObservationKeys,
	},
	{
		Contract:    ClaimContractV4,
		Request:     []string{"schema_version", "role", "seam", "prompt_template", "model", "run", "actor", "observation", "actions", "sampling"},
		Observation: v4ObservationKeys,
	},
}

// DeclaredSeamRequestKeySets returns every declared contract's key set, in
// declared contract order.
func DeclaredSeamRequestKeySets() []SeamRequestKeySet {
	out := make([]SeamRequestKeySet, len(declaredSeamRequestKeySets))
	copy(out, declaredSeamRequestKeySets)
	return out
}

// DeclaredSeamRequestKeySet returns one contract's declared key set. The second
// result is false for an undeclared contract, which a caller must treat as a
// violation rather than as "nothing to check".
func DeclaredSeamRequestKeySet(c ClaimContract) (SeamRequestKeySet, bool) {
	for _, row := range declaredSeamRequestKeySets {
		if row.Contract == c {
			return row, true
		}
	}
	return SeamRequestKeySet{}, false
}
