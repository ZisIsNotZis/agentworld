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
		t.Fatalf("run(q=%d seed=%d workers=%d hours=%d branch=%s founder=%s): %v", o.yield, o.seed, o.workers, o.hours, o.branch, o.founder, err)
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

func TestCapacityCLIForkSharesParentAndDivergesOnlyAfterIntervention(t *testing.T) {
	r := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 4, branch: bothBranchesFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 2 || r.Comparison == nil {
		t.Fatalf("expected both branches and a comparison: %d branches", len(r.Branches))
	}
	if r.CompletedHour != 4 || len(r.NeutralCheckpoint.Digest) != 64 || r.NeutralCheckpoint.Bytes == 0 {
		t.Fatalf("neutral checkpoint not reported: %+v", r.NeutralCheckpoint)
	}
	enabled, disabled := branchByFlag(t, r, true), branchByFlag(t, r, false)
	// Both children fork from the same verified neutral parent and carry
	// distinct child digests.
	if enabled.ParentDigest != r.NeutralCheckpoint.Digest || disabled.ParentDigest != r.NeutralCheckpoint.Digest {
		t.Fatalf("branch lineage does not share the parent digest: %q %q", enabled.ParentDigest, disabled.ParentDigest)
	}
	if enabled.BranchDigest == r.NeutralCheckpoint.Digest || disabled.BranchDigest == r.NeutralCheckpoint.Digest || enabled.BranchDigest == disabled.BranchDigest {
		t.Fatalf("branch digests not distinct: root %q enabled %q disabled %q", r.NeutralCheckpoint.Digest, enabled.BranchDigest, disabled.BranchDigest)
	}
	if !r.Comparison.ParentDigestShared {
		t.Fatal("comparison did not record the shared parent digest")
	}
	// The h0 pulse is pre-intervention: both branches must report an
	// identical alive trajectory start before the first bag decision.
	if enabled.AliveByHour[0] != world.CapacityActorCount || disabled.AliveByHour[0] != world.CapacityActorCount {
		t.Fatalf("h0 alive drifted from the neutral root: %d %d", enabled.AliveByHour[0], disabled.AliveByHour[0])
	}
	// The intervention is the only difference: the disabled continuation
	// records zero capital events and holds zero capital ledgers; the enabled
	// one builds and earns granary stock within the first hours.
	if disabled.Events.Builds != 0 || disabled.Totals.TotalK != 0 || disabled.Totals.PointsCreated != 0 {
		t.Fatalf("disabled branch recorded capital: %+v", disabled.Events)
	}
	if enabled.Events.Builds == 0 || enabled.Totals.PointsCreated == 0 || enabled.Totals.TotalGranary == 0 {
		t.Fatalf("enabled branch produced no capital: builds %d created %d granary %d",
			enabled.Events.Builds, enabled.Totals.PointsCreated, enabled.Totals.TotalGranary)
	}
	// Identical wild activity is not required once capital income changes the
	// bag decisions, but the shared prefix must leave both branches alive.
	if enabled.FinalAlive != world.CapacityActorCount || disabled.FinalAlive != world.CapacityActorCount {
		t.Fatalf("q8 h4 is pre-subsistence: alive %d/%d", enabled.FinalAlive, disabled.FinalAlive)
	}
	for _, br := range []branchReport{enabled, disabled} {
		if !br.ConservationVerified || br.TokenCost != 0 {
			t.Fatalf("conservation or token cost flags: %+v", br)
		}
		if br.WallTimeNS <= 0 || br.CPUTimeNS <= 0 || br.PeakRSSBytes <= 0 || br.HistoryBytes == 0 || br.JournalBytes == 0 || br.EventCount == 0 {
			t.Fatalf("cost accounting incomplete: wall=%d cpu=%d rss=%d history=%d journal=%d events=%d",
				br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes, br.HistoryBytes, br.JournalBytes, br.EventCount)
		}
		if br.AcceptedLinksChecked == 0 || br.UnlinkedRejections == 0 {
			t.Fatalf("event linkage not exercised: accepted=%d unlinked=%d", br.AcceptedLinksChecked, br.UnlinkedRejections)
		}
		if len(br.Actors) != world.CapacityActorCount || len(br.SurvivorIDs) != br.FinalAlive {
			t.Fatalf("actor/survivor cardinality: %d actors %d survivors", len(br.Actors), len(br.SurvivorIDs))
		}
		for _, actor := range br.Actors {
			if actor.Energy == 0 && actor.DeathHour < 0 {
				t.Fatalf("dead actor %d without a recorded death hour", actor.ID)
			}
			if actor.Energy > 0 && actor.SurvivedThroughHour != br.CompletedHour {
				t.Fatalf("survivor %d survived through %d", actor.ID, actor.SurvivedThroughHour)
			}
		}
	}
}

func TestCapacityCLIQ3DisabledMatchesV1Baseline(t *testing.T) {
	// The disabled branch reproduces the v1 q3 fixture exactly: six survivors
	// at h24 on the unchanged physiology.
	r := runForTest(t, options{yield: 3, seed: 0, workers: 1, hours: 24, branch: disabledBranchFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 1 || r.Comparison != nil {
		t.Fatalf("single disabled branch expected: %d branches", len(r.Branches))
	}
	br := r.Branches[0]
	if br.Enabled || br.FinalAlive != 6 {
		t.Fatalf("q3 h24 disabled baseline drift: enabled=%t alive=%d", br.Enabled, br.FinalAlive)
	}
	if br.Totals.TotalK != 0 || br.Totals.PointsCreated != 0 || br.Totals.StoredMeals != 0 {
		t.Fatalf("disabled baseline recorded capital: %+v", br.Totals)
	}
	// Gather classification against the unchanged v1 allocator: every living
	// actor claims once per hour; every admitted gather reaches the bag and
	// is eaten on the disabled branch; denials carry typed labels only.
	var claimHours int
	for _, alive := range br.AliveByHour[:br.CompletedHour] {
		claimHours += alive
	}
	counts := br.Events
	if counts.GatherClaims != int64(claimHours) || counts.GatherClaims != counts.GatherAccepted+
		counts.GatherDenied.NoStock+counts.GatherDenied.Capacity+counts.GatherDenied.Ineligible+
		counts.GatherDenied.NoAccess+counts.GatherDenied.DuplicateActor {
		t.Fatalf("gather claims do not partition: %+v over %d claim-hours", counts, claimHours)
	}
	if counts.GatherAccepted != counts.Eat {
		t.Fatalf("disabled branch admitted gathers %d but ate %d", counts.GatherAccepted, counts.Eat)
	}
	if counts.GatherAccepted == 0 || counts.GatherDenied.Capacity == 0 {
		t.Fatalf("q3 fixture produced no allocation evidence: %+v", counts)
	}
}

func TestCapacityCLIWorkersAgreeOnTrajectories(t *testing.T) {
	strip := func(br branchReport) branchReport {
		br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes = 0, 0, 0
		return br
	}
	one := runForTest(t, options{yield: 3, seed: 0, workers: 1, hours: 3, branch: bothBranchesFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
	four := runForTest(t, options{yield: 3, seed: 0, workers: 4, hours: 3, branch: bothBranchesFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
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

func TestCapacityCLIReportJSONParsesWithRequiredFields(t *testing.T) {
	r := runForTest(t, options{yield: 5, seed: 7, workers: 1, hours: 2, branch: bothBranchesFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"status", "revision", "yield_per_patch_per_hour", "seed", "workers", "requested_hours",
		"completed_hour", "policy", "branch_selection", "founder", "checkpoint_dir", "neutral_checkpoint", "branches",
		"wall_time_ns", "cpu_time_ns", "peak_rss_bytes", "token_cost"} {
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
		for _, key := range []string{"enabled", "founder", "parent_digest", "branch_digest", "branch_checkpoint", "completed_hour",
			"alive_by_hour", "final_alive", "survivor_ids", "actors", "totals", "surplus_flow_48h",
			"patch_wild_consumption_gini", "capital_held_gini", "event_counts", "conservation_verified", "events",
			"event_body_bytes", "accepted_event_links_checked", "unlinked_rejections_checked", "history_bytes",
			"journal_bytes", "wall_time_ns", "cpu_time_ns", "peak_rss_bytes", "token_cost"} {
			if _, ok := br[key]; !ok {
				t.Fatalf("missing branch field %q", key)
			}
		}
		if len(br["alive_by_hour"].([]any)) != 3 {
			t.Fatalf("alive_by_hour must cover h0..h%d", 2)
		}
		if len(br["actors"].([]any)) != world.CapacityActorCount {
			t.Fatal("per-actor cardinality")
		}
		if len(br["patch_wild_consumption_gini"].([]any)) != world.CapacityPatchCount {
			t.Fatal("per-patch wild consumption gini required")
		}
		if _, ok := br["capital_held_gini"].(map[string]any); !ok {
			t.Fatal("capital-held gini required")
		}
		if br["token_cost"].(float64) != 0 {
			t.Fatal("branch token cost must be zero")
		}
		totals := br["totals"].(map[string]any)
		for _, key := range []string{"alive", "total_k", "total_granary", "total_wip", "total_invested_units",
			"total_points_created", "total_points_decayed", "total_wear_debt", "total_yield", "total_yield_unrealized",
			"total_stored_meals", "total_wild_meals"} {
			if _, ok := totals[key]; !ok {
				t.Fatalf("missing totals field %q", key)
			}
		}
		flow := br["surplus_flow_48h"].(map[string]any)
		for _, key := range []string{"window_start_hour", "window_end_hour", "hours", "clamped", "capital_yield", "stored_meals", "net"} {
			if _, ok := flow[key]; !ok {
				t.Fatalf("missing surplus_flow field %q", key)
			}
		}
		counts := br["event_counts"].(map[string]any)
		for _, key := range []string{"gather_claims", "gather_accepted", "gather_denied", "eat", "eat_stored", "wait",
			"rest", "builds", "paired_meals", "paired_meal_no_meal_notes", "fallback_eatstored", "fallback_no_meal_notes"} {
			if _, ok := counts[key]; !ok {
				t.Fatalf("missing event_counts field %q", key)
			}
		}
	}
	comparison := doc["branch_comparison"].(map[string]any)
	for _, key := range []string{"parent_digest_shared", "alive_differences", "final_alive_enabled", "final_alive_disabled",
		"builds_enabled", "builds_disabled", "total_k_enabled", "total_k_disabled"} {
		if _, ok := comparison[key]; !ok {
			t.Fatalf("missing comparison field %q", key)
		}
	}
}

func TestCapacityCLISingleBranchSelectionAndSurplusClamp(t *testing.T) {
	r := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 2, branch: enabledBranchFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 1 || !r.Branches[0].Enabled || r.Comparison != nil {
		t.Fatalf("single enabled branch expected: %d branches comparison=%v", len(r.Branches), r.Comparison)
	}
	// Before h120 the predeclared window is empty and clamped, reporting zero.
	flow := r.Branches[0].SurplusFlow
	if !flow.Clamped || flow.Hours != 0 || flow.Net != 0 || flow.WindowEndHour != 2 || flow.WindowStartHour != 120 {
		t.Fatalf("pre-window surplus flow not clamped to zero: %+v", flow)
	}
	r = runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 2, branch: disabledBranchFlag, founder: founderNoneFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 1 || r.Branches[0].Enabled || r.Comparison != nil {
		t.Fatalf("single disabled branch expected: %d branches comparison=%v", len(r.Branches), r.Comparison)
	}
}

func TestCapacityCLIFounderBEnabledOnly(t *testing.T) {
	r := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 3, branch: enabledBranchFlag, founder: founderBFlag, checkpointDir: newBundleDir(t)})
	if r.Founder != founderBFlag || len(r.Branches) != 1 {
		t.Fatalf("founder-B run not reported: founder=%q branches=%d", r.Founder, len(r.Branches))
	}
	br := r.Branches[0]
	if !br.Enabled || br.Founder != founderBFlag {
		t.Fatalf("founder branch flags: enabled=%t founder=%q", br.Enabled, br.Founder)
	}
	// Founder-B carries actor 1's granary from the verified neutral root: the
	// enabled child may draw it (stored meal), and the conservation gates
	// already cover the endowment. The disabled continuation must be refused
	// before anything is published.
	_, err := run(context.Background(), options{yield: 8, seed: 0, workers: 1, hours: 3, branch: disabledBranchFlag, founder: founderBFlag, checkpointDir: newBundleDir(t)})
	if err == nil {
		t.Fatal("founder-B accepted on the disabled branch")
	}
	if _, err := run(context.Background(), options{yield: 8, seed: 0, workers: 1, hours: 3, branch: bothBranchesFlag, founder: founderBFlag, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("founder-B accepted on both branches")
	}
	// Founder-A (capital endowment) is not constructible under the frozen
	// conservation and is not a flag value.
	if _, err := run(context.Background(), options{yield: 8, seed: 0, workers: 1, hours: 3, branch: enabledBranchFlag, founder: "A", checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("founder-A accepted as a flag value")
	}
}

func TestCapacityCLIRejectsInvalidFlags(t *testing.T) {
	for _, tc := range []options{
		{yield: 3, seed: 0, workers: 1, hours: 0, branch: bothBranchesFlag},
		{yield: 3, seed: 0, workers: 1, hours: world.CapacityHorizonHours + 1, branch: bothBranchesFlag},
		{yield: -1, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag},
		{yield: 9, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag},
		{yield: 3, seed: 0, workers: 0, hours: 2, branch: bothBranchesFlag},
		{yield: 3, seed: 0, workers: 1, hours: 2, branch: "bogus"},
		{yield: 3, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag, founder: "bogus"},
	} {
		if _, err := run(context.Background(), tc); err == nil {
			t.Fatalf("accepted invalid options %+v", tc)
		}
	}
	// A pre-existing -out is refused before any checkpoint directory is created.
	base := t.TempDir()
	used := filepath.Join(base, "report.json")
	if err := os.WriteFile(used, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(base, "must-not-be-created")
	if _, err := run(context.Background(), options{yield: 3, seed: 0, workers: 1, hours: 2, branch: bothBranchesFlag, founder: founderNoneFlag, out: used, checkpointDir: absent}); err == nil {
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

func TestCapacityCLIGiniDefinition(t *testing.T) {
	if got := gini(make([]int64, 8)); got != 0 {
		t.Fatalf("zero total must be defined as zero inequality, got %v", got)
	}
	uniform := make([]int64, 8)
	for i := range uniform {
		uniform[i] = 3
	}
	if got := gini(uniform); got != 0 {
		t.Fatalf("uniform population has zero gini, got %v", got)
	}
	concentrated := make([]int64, 8)
	concentrated[0] = 8
	if got := gini(concentrated); got != 0.875 {
		t.Fatalf("concentrated capital gini %v want 0.875", got)
	}
	sixteen := make([]int64, 16)
	for i := range sixteen {
		sixteen[i] = int64(i % 2)
	}
	if got := gini(sixteen); got != 0.5 {
		t.Fatalf("half-held capital gini %v want 0.5", got)
	}
}
