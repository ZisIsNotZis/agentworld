package protocol

import (
	"sort"
	"testing"

	"agentworld/internal/strategy"
)

// TestClaimActionVocabularyIsABijection is the proof the declared single-name
// exception requires: every internal claim-action ID has exactly one display
// label, every display label has exactly one internal ID, and no token appears
// twice on either side. A duplicate or a missing row fails here rather than
// becoming a request the model cannot answer or an answer the ladder cannot
// map.
func TestClaimActionVocabularyIsABijection(t *testing.T) {
	ids := make([]string, 0, len(ClaimActionVocabularyTable))
	labels := make([]string, 0, len(ClaimActionVocabularyTable))
	seenID := map[string]bool{}
	seenLabel := map[string]bool{}
	for _, row := range ClaimActionVocabularyTable {
		if row.ID == "" || row.Label == "" {
			t.Fatalf("empty vocabulary row: %+v", row)
		}
		if seenID[row.ID] {
			t.Fatalf("duplicate internal id %q", row.ID)
		}
		if seenLabel[row.Label] {
			t.Fatalf("duplicate label %q", row.Label)
		}
		seenID[row.ID], seenLabel[row.Label] = true, true
		ids = append(ids, row.ID)
		labels = append(labels, row.Label)

		label, ok := ClaimActionLabel(row.ID)
		if !ok || label != row.Label {
			t.Fatalf("ClaimActionLabel(%q) = %q,%t want %q,true", row.ID, label, ok, row.Label)
		}
		id, ok := ClaimActionID(row.Label)
		if !ok || id != row.ID {
			t.Fatalf("ClaimActionID(%q) = %q,%t want %q,true", row.Label, id, ok, row.ID)
		}
	}
	sort.Strings(ids)
	sort.Strings(labels)
	want := []string{"eat-stored", "gather", "provision", "wait"}
	if len(ids) != len(want) {
		t.Fatalf("internal ids = %v, want exactly %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Fatalf("internal ids = %v, want exactly %v", ids, want)
		}
	}
	// The build label is a non-claim action label: present in the vocabulary a
	// refusal is stated in, absent from the table that maps answers back.
	if _, ok := ClaimActionID(ClaimActionBuildLabel); ok {
		t.Fatalf("build label %q must not map back to a claim action", ClaimActionBuildLabel)
	}
}

// TestRefusedActionLabelIsARequestValue pins the refusal spellings: no internal
// ID reaches the v4 request, and no denial invents a label.
func TestRefusedActionLabelIsARequestValue(t *testing.T) {
	cases := []struct {
		denial strategy.NaturalDenial
		want   string
	}{
		{denial: strategy.NaturalNoDenial, want: RefusedActionNone},
		{denial: strategy.NaturalGatherDenied, want: "take_slot_food_into_bag"},
		{denial: strategy.NaturalBuildDenied, want: "build_capital_point"},
	}
	for _, tc := range cases {
		if got := RefusedActionLabel(tc.denial); got != tc.want {
			t.Fatalf("RefusedActionLabel(%v) = %q want %q", tc.denial, got, tc.want)
		}
	}
}
