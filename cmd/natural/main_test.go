package main

import (
	"context"
	"encoding/json"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	r, err := run(ctx, o)
	if err != nil {
		t.Fatalf("run(p=%d r=%d seed=%d workers=%d hours=%d births=%s fixture=%t ablation=%t): %v", o.patches, o.r, o.seed, o.workers, o.hours, o.births, o.fixture, o.cultivationDisabled, err)
	}
	return r
}

func branchByBirths(t *testing.T, r report, births bool) branchReport {
	t.Helper()
	for _, br := range r.Branches {
		if br.Births == births {
			return br
		}
	}
	t.Fatalf("no births=%v branch report", births)
	return branchReport{}
}

// TestNaturalCLIForkSharesParentAndDivergesOnlyAfterIntervention forks both
// recorded interventions from one verified births-disabled neutral root: the
// children share the parent digest and carry distinct child digests, neither
// branch records any birth evidence (the recorded poverty trap: the frozen
// birth gate is unreachable for the neutral 16-founder genesis), and the
// trajectories are identical because the intervention cannot act yet — the
// founder-tenure fixture cell is the recorded divergence witness.
func TestNaturalCLIForkSharesParentAndDivergesOnlyAfterIntervention(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 2 {
		t.Fatalf("expected both branches: %d", len(r.Branches))
	}
	enabled, disabled := branchByBirths(t, r, true), branchByBirths(t, r, false)
	if enabled.ParentDigest != r.Root.Digest || disabled.ParentDigest != r.Root.Digest {
		t.Fatalf("branch lineage does not share the parent digest: %q %q", enabled.ParentDigest, disabled.ParentDigest)
	}
	if enabled.BranchDigest == r.Root.Digest || disabled.BranchDigest == r.Root.Digest || enabled.BranchDigest == disabled.BranchDigest {
		t.Fatalf("branch digests not distinct: root %q enabled %q disabled %q", r.Root.Digest, enabled.BranchDigest, disabled.BranchDigest)
	}
	if r.Root.Kind != rootKindNeutral || r.Root.FirstHour != 0 {
		t.Fatalf("root kind drift: %+v", r.Root)
	}
	for _, br := range []branchReport{enabled, disabled} {
		if !br.ConservationVerified || br.TokenCost != 0 {
			t.Fatalf("conservation or token cost flags: %+v", br)
		}
		if br.Events.BirthRequests != 0 || br.Events.BirthReplies != 0 || br.Totals.Births != 0 || br.Events.BirthsAccepted != 0 {
			t.Fatalf("births=%t branch recorded birth evidence under the recorded poverty trap: %+v", br.Births, br.Events)
		}
		if br.WallTimeNS <= 0 || br.CPUTimeNS <= 0 || br.PeakRSSBytes <= 0 || br.HistoryBytes == 0 || br.JournalBytes == 0 || br.EventCount == 0 {
			t.Fatalf("cost accounting incomplete: wall=%d cpu=%d rss=%d history=%d journal=%d events=%d",
				br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes, br.HistoryBytes, br.JournalBytes, br.EventCount)
		}
		if len(br.CeilingLedger) == 0 {
			t.Fatal("ceiling headroom ledger missing")
		}
		if br.Population.FinalAlive != len(br.Population.SurvivorIDs) {
			t.Fatalf("survivor cardinality: %d vs %d", br.Population.FinalAlive, len(br.Population.SurvivorIDs))
		}
		if len(br.Actors) != 16 {
			t.Fatalf("actor cardinality: %d", len(br.Actors))
		}
		for _, actor := range br.Actors {
			if actor.Alive != (actor.DiedHour == world.NaturalNeverHour && actor.Energy > 0) {
				t.Fatalf("actor %d alive invariant drift: %+v", actor.ID, actor)
			}
		}
		if len(br.Population.AliveByHour) != 5 || br.Population.AliveByHour[0] != 16 {
			t.Fatalf("trajectory drift: %v", br.Population.AliveByHour)
		}
	}
	if !reflect.DeepEqual(enabled.Population.AliveByHour, disabled.Population.AliveByHour) {
		t.Fatalf("trajectories diverged before the intervention could act: %v %v", enabled.Population.AliveByHour, disabled.Population.AliveByHour)
	}
}

// TestNaturalCLIFounderTenureBirths pins the fixture cell: the declared
// founder-tenure genesis is the only births-capable one, its root is the
// fixture's first pulse-boundary continuation, births commit, and the
// births-disabled and both selections are refused.
func TestNaturalCLIFounderTenureBirths(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 5, births: enabledBranchFlag, fixture: true, checkpointDir: newBundleDir(t)})
	br := r.Branches[0]
	if !r.FounderTenure || !br.Fixture || r.Root.Kind != rootKindFixture || r.Root.FirstHour != 2 {
		t.Fatalf("fixture root drift: %+v", r.Root)
	}
	if br.Totals.Births == 0 || br.Events.BirthsAccepted == 0 {
		t.Fatalf("fixture cell produced no births: totals=%+v events=%+v", br.Totals, br.Events)
	}
	if firstBirthHour(br.Population, br.FirstHour) < 0 {
		t.Fatal("no hourly birth evidence in the fixture cell")
	}
	if !br.Band.WithinBand {
		t.Fatalf("fixture foothold band falsified: %+v", br.Band)
	}
	for _, a := range br.Actors {
		if a.BirthHour > 0 && a.ParentA == 0 {
			t.Fatalf("newborn %d without parent rows: %+v", a.ID, a)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, births := range []string{disabledBranchFlag, bothBranchesFlag} {
		if _, err := run(ctx, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 5, births: births, fixture: true, checkpointDir: newBundleDir(t)}); err == nil {
			t.Fatalf("fixture -births=%s accepted", births)
		}
	}
	if _, err := run(ctx, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 5, births: enabledBranchFlag, fixture: true, cultivationDisabled: true, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("fixture+ablation combination accepted")
	}
}

// TestNaturalCLICultivationAblation pins the D cell: zero build events and
// attempts anywhere, the documented denial loop visibly engaged, and no
// capital can exist; non-declared births selections are refused.
func TestNaturalCLICultivationAblation(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: disabledBranchFlag, cultivationDisabled: true, checkpointDir: newBundleDir(t)})
	br := r.Branches[0]
	if !br.CultivationDisabled {
		t.Fatal("ablation flag lost")
	}
	if br.Events.Builds != 0 || br.Events.WorldEvents["build"] != 0 {
		t.Fatalf("ablation cell recorded build evidence: %+v", br.Events)
	}
	if br.Events.BuildDenialEats == 0 {
		t.Fatal("ablation denial loop never engaged in this window")
	}
	if br.Totals.Capital != 0 || br.Totals.Wip != 0 || br.Totals.TenuredSlots != 0 {
		t.Fatalf("ablation capital/tenure leaked: %+v", br.Totals)
	}
	if !br.Band.WithinBand {
		t.Fatalf("ablation band falsified: %+v", br.Band)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, births := range []string{enabledBranchFlag, bothBranchesFlag} {
		if _, err := run(ctx, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: births, cultivationDisabled: true, checkpointDir: newBundleDir(t)}); err == nil {
			t.Fatalf("ablation -births=%s accepted", births)
		}
	}
}

// TestNaturalCLIWorkersAgreeAfterStrippingTiming proves workers 1 and 4
// produce identical evidence: only the worker count, the checkpoint
// directory and the timing fields may differ.
func TestNaturalCLIWorkersAgreeAfterStrippingTiming(t *testing.T) {
	strip := func(r report) report {
		r.WallTimeNS, r.CPUTimeNS, r.PeakRSSBytes, r.CheckpointDir, r.Workers = 0, 0, 0, "", 0
		for i := range r.Branches {
			r.Branches[i].WallTimeNS, r.Branches[i].CPUTimeNS, r.Branches[i].PeakRSSBytes = 0, 0, 0
			// Measured-rate cost fields are timing; the budget constants are
			// not. With the measured rate stripped, the within-budget flag
			// recomputes against the stripped value.
			r.Branches[i].CostBudget.CPUPerSimulatedHour = 0
			r.Branches[i].CostBudget.CPUPerActorHour = 0
			r.Branches[i].CostBudget.CPUWithinBudget = r.Branches[i].CostBudget.CPUBudget >= 0
		}
		return r
	}
	one := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	four := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 4, hours: 3, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	if one.Root.Digest != four.Root.Digest {
		t.Fatal("worker count changed the root bundle")
	}
	if !reflect.DeepEqual(strip(one), strip(four)) {
		t.Fatalf("workers diverged:\n one: %+v\nfour: %+v", strip(one), strip(four))
	}
	fixtureOne := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 3, workers: 1, hours: 4, births: enabledBranchFlag, fixture: true, checkpointDir: newBundleDir(t)})
	fixtureFour := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 3, workers: 4, hours: 4, births: enabledBranchFlag, fixture: true, checkpointDir: newBundleDir(t)})
	if !reflect.DeepEqual(strip(fixtureOne), strip(fixtureFour)) {
		t.Fatal("fixture workers diverged")
	}
}

// TestNaturalCLIReportJSONParsesWithRequiredFields round-trips a full report
// and asserts every documented field is present, including the ceiling
// headroom ledger rows and the ceiling-hit/verdict key set.
func TestNaturalCLIReportJSONParsesWithRequiredFields(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 0, seed: 7, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"status", "revision", "patches", "founders", "r_per_patch_per_hour", "seed", "workers",
		"requested_hours", "completed_hour", "policy", "branch_selection", "founder_tenure", "cultivation_disabled",
		"checkpoint_dir", "root_checkpoint", "branches", "wall_time_ns", "cpu_time_ns", "peak_rss_bytes", "token_cost"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("missing top-level report field %q", key)
		}
	}
	if doc["token_cost"].(float64) != 0 {
		t.Fatal("token cost must be zero")
	}
	root := doc["root_checkpoint"].(map[string]any)
	for _, key := range []string{"kind", "first_hour", "name", "digest", "bytes"} {
		if _, ok := root[key]; !ok {
			t.Fatalf("missing root_checkpoint field %q", key)
		}
	}
	branches := doc["branches"].([]any)
	if len(branches) != 2 {
		t.Fatalf("expected both branches, got %d", len(branches))
	}
	for _, raw := range branches {
		br := raw.(map[string]any)
		for _, key := range []string{"births", "founder_tenure_genesis", "cultivation_ablation", "parent_digest", "branch_digest",
			"branch_checkpoint", "first_hour", "completed_hour", "halted_by_ceiling_hit", "ceiling_hit", "ceiling_ledger",
			"population", "regen_split", "granary", "malthus_index", "actors", "totals", "event_counts", "frozen_band",
			"cost_budget", "conservation_verified", "events", "event_body_bytes", "accepted_event_links_checked",
			"unlinked_rejections_checked", "history_bytes", "journal_bytes", "wall_time_ns", "cpu_time_ns", "peak_rss_bytes", "token_cost"} {
			if _, ok := br[key]; !ok {
				t.Fatalf("missing branch field %q", key)
			}
		}
		pop := br["population"].(map[string]any)
		for _, key := range []string{"alive_by_hour", "births_by_hour", "deaths_starvation_by_hour", "deaths_age_by_hour",
			"final_alive", "survivor_ids", "extinction_hour"} {
			if _, ok := pop[key]; !ok {
				t.Fatalf("missing population field %q", key)
			}
		}
		regen := br["regen_split"].(map[string]any)
		for _, key := range []string{"cultivated_by_hour", "wild_by_hour", "unrealized_by_hour",
			"cultivated_slots_total_by_hour", "cultivated_slots_per_patch_final", "total_cultivated", "total_wild", "total_unrealized"} {
			if _, ok := regen[key]; !ok {
				t.Fatalf("missing regen_split field %q", key)
			}
		}
		granary := br["granary"].(map[string]any)
		for _, key := range []string{"inflow_total", "spoiled_total", "stored_meals_total", "stock_final", "spoil_debt_final",
			"inflow_by_hour", "spoiled_by_hour", "stored_meals_by_hour"} {
			if _, ok := granary[key]; !ok {
				t.Fatalf("missing granary field %q", key)
			}
		}
		malthus := br["malthus_index"].(map[string]any)
		for _, key := range []string{"definition", "index_by_hour", "final"} {
			if _, ok := malthus[key]; !ok {
				t.Fatalf("missing malthus_index field %q", key)
			}
		}
		if _, ok := br["ceiling_hit"]; !ok {
			t.Fatal("missing ceiling_hit key")
		}
		ledger := br["ceiling_ledger"].([]any)
		if len(ledger) == 0 {
			t.Fatal("empty ceiling ledger")
		}
		for _, rawRow := range ledger {
			row := rawRow.(map[string]any)
			for _, key := range []string{"ceiling", "threshold", "observed", "rate_last_hour", "projected_binding_hour"} {
				if _, ok := row[key]; !ok {
					t.Fatalf("missing ceiling ledger row field %q", key)
				}
			}
		}
		band := br["frozen_band"].(map[string]any)
		for _, key := range []string{"name", "criterion", "observed", "within_band"} {
			if _, ok := band[key]; !ok {
				t.Fatalf("missing frozen_band field %q", key)
			}
		}
		costs := br["cost_budget"].(map[string]any)
		for _, key := range []string{"cpu_seconds_per_simulated_hour", "cpu_budget_3x_v4", "cpu_within_budget",
			"history_bytes_per_simulated_hour", "bytes_budget_2x_v4", "bytes_within_budget", "actor_hours_extrapolation_only",
			"bytes_per_actor_hour", "cpu_seconds_per_actor_hour", "extrapolation_label"} {
			if _, ok := costs[key]; !ok {
				t.Fatalf("missing cost_budget field %q", key)
			}
		}
		if br["totals"].(map[string]any)["total_births"].(float64) != 0 {
			t.Fatal("R=0 cell recorded births")
		}
	}
}

// TestNaturalCLIR0Extinction pins the R=0 band on both branches: extinction
// by h12 with zero birth evidence anywhere.
func TestNaturalCLIR0Extinction(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 0, seed: 0, workers: 1, hours: 12, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	enabled, disabled := branchByBirths(t, r, true), branchByBirths(t, r, false)
	for _, br := range []branchReport{enabled, disabled} {
		if br.Population.ExtinctionHour < 0 || br.Population.ExtinctionHour > 12 {
			t.Fatalf("R=0 extinction band drift: %d", br.Population.ExtinctionHour)
		}
		if br.Totals.Births != 0 || br.Events.BirthRequests != 0 || br.Events.BirthReplies != 0 {
			t.Fatalf("R=0 recorded birth evidence: %+v", br.Events)
		}
		if !br.Band.WithinBand {
			t.Fatalf("R=0 band flagged: %+v", br.Band)
		}
	}
	if !reflect.DeepEqual(enabled.Population.AliveByHour, disabled.Population.AliveByHour) {
		t.Fatal("R=0 branches diverged")
	}
}

// TestNaturalCLIVerdictClassifier pins the frozen P15 verdict table's four
// branches plus the no-branch outcome on synthetic evidence.
func TestNaturalCLIVerdictClassifier(t *testing.T) {
	checks := func(alive []int, starvation int64, fibers, wakes uint64) []world.NaturalCheckpoint {
		out := make([]world.NaturalCheckpoint, len(alive))
		for i, n := range alive {
			out[i] = world.NaturalCheckpoint{Hour: i, Metrics: world.NaturalMetrics{Alive: n, DeathsStarvation: starvation}}
			out[i].Ledger = []world.NaturalCeilingRow{
				{Ceiling: world.NaturalCeilingPortableFibers, Observed: fibers},
				{Ceiling: world.NaturalCeilingPortableWakes, Observed: wakes},
			}
		}
		return out
	}
	o := options{patches: 32, r: 8} // N* = P·R = 256
	yes := true
	no := false
	alivePlateau := make([]int, 51)
	for i := range alivePlateau {
		alivePlateau[i] = 250
	}
	// Outgrew the harness: green, near N*, turnover, replay identical.
	v := classifyVerdict(o, world.NaturalCeilingHit{Hour: 50, Alive: 250, Births: 11, Deaths: 10}, true, &yes, checks(alivePlateau, 0, 257, 200))
	if v.Verdict != verdictOutgrew {
		t.Fatalf("outgrew-harness misclassified: %+v", v)
	}
	// Implementation defect: replay equality failed.
	v = classifyVerdict(o, world.NaturalCeilingHit{Hour: 50, Alive: 250, Births: 11, Deaths: 10}, true, &no, checks(alivePlateau, 0, 257, 200))
	if v.Verdict != verdictImplementation {
		t.Fatalf("implementation defect misclassified: %+v", v)
	}
	// Design defect: green, starvation deaths exceed births, large unrealized share.
	v = classifyVerdict(o, world.NaturalCeilingHit{Hour: 10, Alive: 20, Births: 0, Deaths: 40, RegenUnrealized: 1000}, true, &yes, checks(make([]int, 11), 40, 21, 20))
	if v.Verdict != verdictDesignDefect {
		t.Fatalf("design defect misclassified: %+v", v)
	}
	// Runtime defect: flat population with a fiber/wake storm.
	v = classifyVerdict(o, world.NaturalCeilingHit{Hour: 10, Alive: 16, Births: 0, Deaths: 0}, true, &yes, checks([]int{16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16}, 0, 4000, 40000))
	if v.Verdict != verdictRuntime {
		t.Fatalf("runtime defect misclassified: %+v", v)
	}
	// The flat all-founders population AT N* with green conservation and
	// identical replay legitimately classifies as outgrew-harness — that is
	// the expected F-cell outcome, with the zero-turnover poverty-trap
	// context recorded in the evidence fields.
	flatFull := make([]int, 51)
	for i := range flatFull {
		flatFull[i] = 256
	}
	v = classifyVerdict(o, world.NaturalCeilingHit{Hour: 50, Alive: 256, Births: 0, Deaths: 0}, true, &yes, checks(flatFull, 0, 257, 200))
	if v.Verdict != verdictOutgrew {
		t.Fatalf("at-N* flat population misclassified: %+v", v)
	}
	if !v.PovertyTrapContext || !v.PopulationFlat || !v.TurnoverHolds {
		t.Fatalf("outgrew evidence fields drift: %+v", v)
	}
	// No frozen branch fired: population well below the N* bar, no
	// starvation excess, no unrealized share, no storm — none of the four
	// frozen branches applies verbatim.
	flatLow := make([]int, 51)
	for i := range flatLow {
		flatLow[i] = 100
	}
	v = classifyVerdict(o, world.NaturalCeilingHit{Hour: 50, Alive: 100, Births: 0, Deaths: 0}, true, &yes, checks(flatLow, 0, 101, 200))
	if v.Verdict != verdictNoBranchFired {
		t.Fatalf("no-branch outcome misclassified: %+v", v)
	}
}

// TestNaturalCLIRefusals pins the invalid-input refusals and no-clobber.
func TestNaturalCLIRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		name string
		o    options
	}{
		{"patches-zero", options{patches: 0, r: 8, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"patches-above-id-space", options{patches: world.NaturalMaxPatchesFixedIDSpace + 1, r: 8, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"r-negative", options{patches: 2, r: -1, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"r-above-slots", options{patches: 2, r: world.NaturalSlotsPerPatch + 1, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"workers-zero", options{patches: 2, r: 8, seed: 0, workers: 0, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"hours-zero", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 0, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"hours-above-horizon", options{patches: 2, r: 8, seed: 0, workers: 1, hours: world.NaturalHorizonHours + 1, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"bad-births", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: "maybe", checkpointDir: newBundleDir(t)}},
		{"fixture-disabled", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 5, births: disabledBranchFlag, fixture: true, checkpointDir: newBundleDir(t)}},
		{"fixture-both", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 5, births: bothBranchesFlag, fixture: true, checkpointDir: newBundleDir(t)}},
		{"fixture-below-first-boundary", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 3, births: enabledBranchFlag, fixture: true, checkpointDir: newBundleDir(t)}},
		{"fixture-ablation", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 5, births: enabledBranchFlag, fixture: true, cultivationDisabled: true, checkpointDir: newBundleDir(t)}},
		{"ablation-enabled", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: enabledBranchFlag, cultivationDisabled: true, checkpointDir: newBundleDir(t)}},
		{"ablation-both", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, cultivationDisabled: true, checkpointDir: newBundleDir(t)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := run(ctx, tc.o); err == nil {
				t.Fatalf("invalid options accepted: %+v", tc.o)
			}
		})
	}
	// No-clobber: an existing output file is refused before any run, and an
	// existing checkpoint directory is never populated.
	base := t.TempDir()
	out := filepath.Join(base, "report.json")
	if err := os.WriteFile(out, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, out: out, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("existing output accepted")
	}
	dir := filepath.Join(base, "bundles")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, checkpointDir: dir}); err == nil {
		t.Fatal("existing checkpoint directory accepted")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("existing checkpoint directory populated: %v %v", entries, err)
	}
}
