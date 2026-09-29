package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agentworld/internal/world"
)

// newBundleDir returns a previously absent bundle directory inside the test
// scratch space; the CLI refuses to populate an existing directory.
func newBundleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "bundles")
}

// runForTest executes a short fixture prefix; unit tests never run the full
// 168-hour horizon.
func runForTest(t *testing.T, o options) report {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	r, err := run(ctx, o)
	if err != nil {
		t.Fatalf("run(q=%d seed=%d workers=%d hours=%d branch=%s): %v", o.yield, o.seed, o.workers, o.hours, o.branch, err)
	}
	return r
}

func branchByFlag(t *testing.T, r report, enabled bool) branchReport {
	t.Helper()
	for _, br := range r.Branches {
		if br.Enabled == enabled {
			return br
		}
	}
	t.Fatalf("no %v branch report", enabled)
	return branchReport{}
}

func TestSocialFoodCLIForkSharesParentAndDiffersOnlyByIntervention(t *testing.T) {
	r := runForTest(t, options{yield: 3, seed: 0, workers: 1, hours: 8, branch: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 2 || r.Comparison == nil {
		t.Fatalf("expected both branches and a comparison: %d branches", len(r.Branches))
	}
	if r.CompletedHour != 8 || len(r.NeutralCheckpoint.Digest) != 64 || r.NeutralCheckpoint.Bytes == 0 {
		t.Fatalf("neutral checkpoint not reported: %+v", r.NeutralCheckpoint)
	}
	enabled, disabled := branchByFlag(t, r, true), branchByFlag(t, r, false)
	// Both children fork from the same verified neutral parent.
	if enabled.ParentDigest != r.NeutralCheckpoint.Digest || disabled.ParentDigest != r.NeutralCheckpoint.Digest {
		t.Fatalf("branch lineage does not share the parent digest: %q %q", enabled.ParentDigest, disabled.ParentDigest)
	}
	if enabled.BranchDigest == r.NeutralCheckpoint.Digest || disabled.BranchDigest == r.NeutralCheckpoint.Digest || enabled.BranchDigest == disabled.BranchDigest {
		t.Fatalf("branch digests not distinct: root %q enabled %q disabled %q", r.NeutralCheckpoint.Digest, enabled.BranchDigest, disabled.BranchDigest)
	}
	if !r.Comparison.ParentDigestShared {
		t.Fatal("comparison did not record the shared parent digest")
	}
	// The intervention is the only difference: the disabled continuation
	// witnesses no social evidence at all; the enabled one does.
	if enabled.Social.Requests == 0 || enabled.Social.Gifts == 0 {
		t.Fatalf("enabled branch observed no social evidence: %+v", enabled.Social)
	}
	for _, zero := range []int64{disabled.Social.Requests, disabled.Social.Gifts, disabled.Social.Refusals, disabled.Social.Expiries} {
		if zero != 0 {
			t.Fatalf("disabled branch recorded social evidence: %+v", disabled.Social)
		}
	}
	if enabled.Social.Requests != enabled.Social.Gifts+enabled.Social.Refusals+enabled.Social.Expiries {
		t.Fatalf("enabled requests did not resolve: %+v", enabled.Social)
	}
	// Every gift transfers exactly one unit with a privileged audit trail.
	for _, gift := range enabled.GiftEvents {
		if gift.Score < 3 || gift.DonorEnergy < 4 || gift.Claim != 2 {
			t.Fatalf("gift event violates the frozen consent floor: %+v", gift)
		}
	}
	// Identical gather activity: the intervention changes only social phases.
	if enabled.Social.GatherClaims != disabled.Social.GatherClaims || enabled.Social.GatherAccepted != disabled.Social.GatherAccepted {
		t.Fatalf("gather activity diverged beyond the intervention: %+v vs %+v", enabled.Social, disabled.Social)
	}
	if len(enabled.Dyads) != world.SocialFoodActorCount || len(enabled.Actors) != world.SocialFoodActorCount {
		t.Fatalf("actor/dyad cardinality: %d dyads %d actors", len(enabled.Dyads), len(enabled.Actors))
	}
	for i, dyad := range enabled.Dyads {
		wantAffinity := int64(1)
		if dyad.Requester == 4 || dyad.Requester == 8 || dyad.Requester == 12 || dyad.Requester == 16 {
			wantAffinity = 3
		}
		if dyad.Affinity != wantAffinity {
			t.Fatalf("dyad %d affinity %d want %d", i+1, dyad.Affinity, wantAffinity)
		}
	}
	if !enabled.ConservationVerified || !disabled.ConservationVerified || enabled.TokenCost != 0 || disabled.TokenCost != 0 {
		t.Fatalf("conservation or token cost flags: %+v %+v", enabled.TokenCost, disabled.TokenCost)
	}
	if enabled.WallTimeNS <= 0 || enabled.CPUTimeNS <= 0 || enabled.PeakRSSBytes <= 0 || enabled.HistoryBytes == 0 || enabled.JournalBytes == 0 || enabled.Events == 0 {
		t.Fatalf("cost accounting incomplete: wall=%d cpu=%d rss=%d history=%d journal=%d events=%d",
			enabled.WallTimeNS, enabled.CPUTimeNS, enabled.PeakRSSBytes, enabled.HistoryBytes, enabled.JournalBytes, enabled.Events)
	}
	if enabled.AcceptedLinksChecked == 0 || enabled.UnlinkedRejections == 0 {
		t.Fatalf("event linkage not exercised: accepted=%d unlinked=%d", enabled.AcceptedLinksChecked, enabled.UnlinkedRejections)
	}
}

func TestSocialFoodCLIQ3H24FrozenFixture(t *testing.T) {
	r := runForTest(t, options{yield: 3, seed: 0, workers: 1, hours: 24, branch: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	enabled, disabled := branchByFlag(t, r, true), branchByFlag(t, r, false)
	// The predeclared q3 seed-0 fixture: six survivors at h24 on both branches.
	if enabled.FinalAlive != 6 || disabled.FinalAlive != 6 {
		t.Fatalf("q3 h24 fixture drift: enabled=%d disabled=%d want 6", enabled.FinalAlive, disabled.FinalAlive)
	}
	if len(r.Comparison.AliveDifferences) == 0 {
		t.Fatal("h24 branches show no alive divergence to report")
	}
	if disabled.Social.Gifts != 0 || enabled.Social.Gifts == 0 {
		t.Fatalf("gift counts: enabled %d disabled %d", enabled.Social.Gifts, disabled.Social.Gifts)
	}
	for _, br := range []branchReport{enabled, disabled} {
		for _, actor := range br.Actors {
			if actor.Energy == 0 && actor.DeathHour < 0 {
				t.Fatalf("dead actor %d without a recorded death hour", actor.ID)
			}
			if actor.Energy > 0 && actor.SurvivedThroughHour != 24 {
				t.Fatalf("survivor %d survived through %d", actor.ID, actor.SurvivedThroughHour)
			}
		}
	}
}

func TestSocialFoodCLIWorkersAgreeOnTrajectories(t *testing.T) {
	strip := func(br branchReport) branchReport {
		br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes = 0, 0, 0
		return br
	}
	one := runForTest(t, options{yield: 3, seed: 0, workers: 1, hours: 4, branch: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	four := runForTest(t, options{yield: 3, seed: 0, workers: 4, hours: 4, branch: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	if one.NeutralCheckpoint.Digest != four.NeutralCheckpoint.Digest {
		t.Fatal("worker count changed the neutral bundle")
	}
	for _, enabled := range []bool{true, false} {
		a, b := strip(branchByFlag(t, one, enabled)), strip(branchByFlag(t, four, enabled))
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("enabled=%t workers diverged:\n%+v\n%+v", enabled, a, b)
		}
	}
}

func TestSocialFoodCLIReportJSONParsesWithRequiredFields(t *testing.T) {
	r := runForTest(t, options{yield: 3, seed: 7, workers: 1, hours: 2, branch: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"status", "revision", "yield_per_patch_per_hour", "seed", "workers", "requested_hours",
		"completed_hour", "policy", "branch_selection", "checkpoint_dir", "neutral_checkpoint", "branches", "wall_time_ns",
		"cpu_time_ns", "peak_rss_bytes", "token_cost"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("missing top-level report field %q", key)
		}
	}
	if doc["token_cost"].(float64) != 0 {
		t.Fatal("token cost must be zero")
	}
	neutral := doc["neutral_checkpoint"].(map[string]any)
	for _, key := range []string{"name", "digest", "bytes"} {
		if _, ok := neutral[key]; !ok {
			t.Fatalf("missing neutral_checkpoint field %q", key)
		}
	}
	branches := doc["branches"].([]any)
	if len(branches) != 2 {
		t.Fatalf("expected both branches, got %d", len(branches))
	}
	for _, raw := range branches {
		br := raw.(map[string]any)
		for _, key := range []string{"enabled", "parent_digest", "branch_digest", "branch_checkpoint", "completed_hour",
			"alive_by_hour", "final_alive", "survivor_ids", "actors", "actor_traces", "dyads", "social", "gift_events",
			"patch_consumed_gini", "final_balance", "conservation_verified", "events", "event_body_bytes",
			"accepted_event_links_checked", "unlinked_rejections_checked", "history_bytes", "journal_bytes",
			"wall_time_ns", "cpu_time_ns", "peak_rss_bytes", "token_cost"} {
			if _, ok := br[key]; !ok {
				t.Fatalf("missing branch field %q", key)
			}
		}
		if len(br["alive_by_hour"].([]any)) != 3 {
			t.Fatalf("alive_by_hour must cover h0..h%d", 2)
		}
		if len(br["actors"].([]any)) != world.SocialFoodActorCount || len(br["dyads"].([]any)) != world.SocialFoodActorCount {
			t.Fatal("actor/dyad cardinality")
		}
		if len(br["actor_traces"].(map[string]any)) != 3 {
			t.Fatal("actor 1/8/16 traces required")
		}
		if len(br["patch_consumed_gini"].([]any)) != world.SocialFoodPatchCount {
			t.Fatal("per-patch gini required")
		}
		if br["token_cost"].(float64) != 0 {
			t.Fatal("branch token cost must be zero")
		}
	}
	comparison := doc["branch_comparison"].(map[string]any)
	for _, key := range []string{"parent_digest_shared", "alive_differences", "final_alive_enabled", "final_alive_disabled",
		"survivor_ids_enabled", "survivor_ids_disabled", "gifts_enabled", "gifts_disabled", "donor_given_units_enabled",
		"donor_given_units_disabled", "total_consumed_enabled", "total_consumed_disabled"} {
		if _, ok := comparison[key]; !ok {
			t.Fatalf("missing comparison field %q", key)
		}
	}
}

func TestSocialFoodCLISingleBranchSelection(t *testing.T) {
	r := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 2, branch: enabledBranchFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 1 || !r.Branches[0].Enabled || r.Comparison != nil {
		t.Fatalf("single enabled branch expected: %d branches comparison=%v", len(r.Branches), r.Comparison)
	}
	r = runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 2, branch: disabledBranchFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 1 || r.Branches[0].Enabled || r.Comparison != nil {
		t.Fatalf("single disabled branch expected: %d branches comparison=%v", len(r.Branches), r.Comparison)
	}
}

func TestSocialFoodCLIRejectsInvalidFlags(t *testing.T) {
	base := t.TempDir()
	for _, tc := range []options{
		{yield: 3, seed: 0, workers: 1, hours: 0, branch: bothBranchesFlag},
		{yield: 3, seed: 0, workers: 1, hours: world.SocialFoodHorizonHours + 1, branch: bothBranchesFlag},
		{yield: -1, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag},
		{yield: 9, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag},
		{yield: 3, seed: 0, workers: 0, hours: 2, branch: bothBranchesFlag},
		{yield: 3, seed: 0, workers: 1, hours: 2, branch: "bogus"},
	} {
		if _, err := run(context.Background(), tc); err == nil {
			t.Fatalf("accepted invalid options %+v", tc)
		}
	}
	// A pre-existing -out is refused before any checkpoint directory is created.
	used := filepath.Join(base, "report.json")
	if err := os.WriteFile(used, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(base, "must-not-be-created")
	if _, err := run(context.Background(), options{yield: 3, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag, out: used, checkpointDir: absent}); err == nil {
		t.Fatal("accepted existing output path")
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("checkpoint directory created despite refused output: %v", err)
	}
	content, err := os.ReadFile(used)
	if err != nil || string(content) != "keep" {
		t.Fatalf("existing output modified: %q %v", content, err)
	}
}

func TestSocialFoodCLIConsumedGiniDefinition(t *testing.T) {
	eight := func(values ...int64) []int64 {
		for len(values) < 8 {
			values = append(values, 0)
		}
		return values
	}
	if g := consumedGini(eight()); g != 0 {
		t.Fatalf("zero total must be defined as zero inequality, got %v", g)
	}
	if g := consumedGini(eight(1, 1, 1, 1, 1, 1, 1, 1)); g != 0 {
		t.Fatalf("uniform consumption has zero gini, got %v", g)
	}
	// One actor consumed everything: maximum inequality (n-1)/n = 7/8.
	if g := consumedGini(eight(8)); g != 0.875 {
		t.Fatalf("concentrated consumption gini %v want 0.875", g)
	}
	half := consumedGini(eight(1, 1, 1, 1, 0, 0, 0, 0))
	if half != 0.5 {
		t.Fatalf("half-fed patch gini %v want 0.5", half)
	}
}
