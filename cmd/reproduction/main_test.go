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
		t.Fatalf("run(q=%d seed=%d workers=%d hours=%d births=%s variant=%t): %v", o.yield, o.seed, o.workers, o.hours, o.births, o.genomeVariant, err)
	}
	return r
}

func branchByFlag(t *testing.T, r report, births bool) branchReport {
	t.Helper()
	for _, br := range r.Branches {
		if br.Births == births {
			return br
		}
	}
	t.Fatalf("no births=%v branch report", births)
	return branchReport{}
}

// TestReproductionCLIForkSharesParentAndDivergesOnlyAfterIntervention forks
// both recorded interventions from one verified births-disabled neutral
// root: the children share the parent digest and carry distinct child
// digests, the disabled branch records zero birth evidence, the enabled
// branch commits births, and the alive trajectories stay identical until the
// first committed birth, which only the enabled branch gains.
func TestReproductionCLIForkSharesParentAndDivergesOnlyAfterIntervention(t *testing.T) {
	r := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 12, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	if len(r.Branches) != 2 || r.Comparison == nil {
		t.Fatalf("expected both branches and a comparison: %d branches", len(r.Branches))
	}
	if r.CompletedHour != 12 || len(r.NeutralCheckpoint.Digest) != 64 || r.NeutralCheckpoint.Bytes == 0 {
		t.Fatalf("neutral checkpoint not reported: %+v", r.NeutralCheckpoint)
	}
	enabled, disabled := branchByFlag(t, r, true), branchByFlag(t, r, false)
	if enabled.ParentDigest != r.NeutralCheckpoint.Digest || disabled.ParentDigest != r.NeutralCheckpoint.Digest {
		t.Fatalf("branch lineage does not share the parent digest: %q %q", enabled.ParentDigest, disabled.ParentDigest)
	}
	if enabled.BranchDigest == r.NeutralCheckpoint.Digest || disabled.BranchDigest == r.NeutralCheckpoint.Digest || enabled.BranchDigest == disabled.BranchDigest {
		t.Fatalf("branch digests not distinct: root %q enabled %q disabled %q", r.NeutralCheckpoint.Digest, enabled.BranchDigest, disabled.BranchDigest)
	}
	if !r.Comparison.ParentDigestShared {
		t.Fatal("comparison did not record the shared parent digest")
	}
	// The h0 pulse is pre-intervention on both branches.
	if enabled.Population.AliveByHour[0] != 16 || disabled.Population.AliveByHour[0] != 16 {
		t.Fatalf("h0 alive drifted from the neutral root: %d %d", enabled.Population.AliveByHour[0], disabled.Population.AliveByHour[0])
	}
	// The intervention is the only content difference: the disabled branch
	// records zero birth evidence anywhere; the enabled one commits births
	// within the fixture window and its trajectory gains them.
	if disabled.Events.BirthRequests != 0 || disabled.Events.BirthReplies != 0 || disabled.Totals.Births != 0 ||
		disabled.BirthEconomics.Births != 0 || disabled.BirthEconomics.BirthRequests != 0 {
		t.Fatalf("disabled branch recorded birth evidence: %+v", disabled.Events)
	}
	if enabled.Totals.Births == 0 || enabled.Events.BirthsAccepted == 0 {
		t.Fatalf("enabled branch produced no births: births %d accepted %d", enabled.Totals.Births, enabled.Events.BirthsAccepted)
	}
	firstBirth := -1
	for h, n := range enabled.Population.BirthsByHour {
		if n > 0 {
			firstBirth = h
			break
		}
	}
	if firstBirth < 0 {
		t.Fatal("no hourly birth evidence on the enabled branch")
	}
	// A birth committed inside hour h-1's lattice first appears in the h
	// checkpoint's alive count, so the trajectories must stay identical up
	// to the checkpoint before the first recorded birth gain.
	for h := 0; h < firstBirth; h++ {
		if enabled.Population.AliveByHour[h] != disabled.Population.AliveByHour[h] {
			t.Fatalf("branches diverged before the recorded intervention could act: h%d %d %d", h, enabled.Population.AliveByHour[h], disabled.Population.AliveByHour[h])
		}
	}
	if enabled.Population.AliveByHour[firstBirth] < disabled.Population.AliveByHour[firstBirth] {
		t.Fatalf("the enabled branch lost population at the first birth checkpoint: h%d %d %d", firstBirth,
			enabled.Population.AliveByHour[firstBirth], disabled.Population.AliveByHour[firstBirth])
	}
	if enabled.Population.FinalAlive <= disabled.Population.FinalAlive {
		t.Fatalf("the enabled branch never gains population: %d %d", enabled.Population.FinalAlive, disabled.Population.FinalAlive)
	}
	for _, br := range []branchReport{enabled, disabled} {
		if !br.ConservationVerified || br.TokenCost != 0 {
			t.Fatalf("conservation or token cost flags: %+v", br)
		}
		if br.WallTimeNS <= 0 || br.CPUTimeNS <= 0 || br.PeakRSSBytes <= 0 || br.HistoryBytes == 0 || br.JournalBytes == 0 || br.EventCount == 0 {
			t.Fatalf("cost accounting incomplete: wall=%d cpu=%d rss=%d history=%d journal=%d events=%d",
				br.WallTimeNS, br.CPUTimeNS, br.PeakRSSBytes, br.HistoryBytes, br.JournalBytes, br.EventCount)
		}
		if br.AcceptedLinksChecked == 0 || br.UnlinkedRejectionsChecked == 0 {
			t.Fatalf("event linkage not exercised: accepted=%d unlinked=%d", br.AcceptedLinksChecked, br.UnlinkedRejectionsChecked)
		}
		if len(br.Actors) < 16 || len(br.Population.SurvivorIDs) != br.Population.FinalAlive {
			t.Fatalf("actor/survivor cardinality: %d actors %d survivors", len(br.Actors), br.Population.FinalAlive)
		}
		for _, actor := range br.Actors {
			if !actor.Alive && actor.DiedHour < 0 {
				t.Fatalf("dead actor %d without a recorded died-hour", actor.ID)
			}
			if actor.Alive && actor.DiedHour != -1 {
				t.Fatalf("live actor %d carries a died-hour", actor.ID)
			}
		}
	}
	// Kinship is derived only from the reproduction graph: the enabled
	// branch must have related living pairs within the fixture window.
	if enabled.Kinship.KinPairs == 0 || enabled.Kinship.LivingPairs == 0 {
		t.Fatalf("enabled branch produced no kinship evidence: %+v", enabled.Kinship)
	}
}

// TestReproductionCLIWorkersAgreeAfterStrippingTiming proves workers 1 and 4
// produce identical evidence: only the worker count, the checkpoint
// directory and the timing fields may differ.
func TestReproductionCLIWorkersAgreeAfterStrippingTiming(t *testing.T) {
	strip := func(r report) report {
		r.WallTimeNS, r.CPUTimeNS, r.PeakRSSBytes, r.CheckpointDir, r.Workers = 0, 0, 0, "", 0
		for i := range r.Branches {
			r.Branches[i].WallTimeNS, r.Branches[i].CPUTimeNS, r.Branches[i].PeakRSSBytes = 0, 0, 0
		}
		return r
	}
	one := runForTest(t, options{yield: 3, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	four := runForTest(t, options{yield: 3, seed: 0, workers: 4, hours: 4, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	if one.NeutralCheckpoint.Digest != four.NeutralCheckpoint.Digest {
		t.Fatal("worker count changed the neutral bundle")
	}
	if !reflect.DeepEqual(strip(one), strip(four)) {
		t.Fatalf("workers diverged:\n one: %+v\nfour: %+v", strip(one), strip(four))
	}
}

// TestReproductionCLIReportJSONParsesWithRequiredFields round-trips a full
// report and asserts every documented field is present.
func TestReproductionCLIReportJSONParsesWithRequiredFields(t *testing.T) {
	r := runForTest(t, options{yield: 5, seed: 7, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"status", "revision", "yield_per_patch_per_hour", "seed", "workers", "requested_hours",
		"completed_hour", "policy", "branch_selection", "genome_variant", "checkpoint_dir", "neutral_checkpoint", "branches",
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
		for _, key := range []string{"births", "genome_variant", "parent_digest", "branch_digest", "branch_checkpoint", "completed_hour",
			"population", "actors", "totals", "genome_diversity", "kinship", "birth_economics", "event_counts",
			"conservation_verified", "events", "event_body_bytes", "accepted_event_links_checked", "unlinked_rejections_checked",
			"history_bytes", "journal_bytes", "wall_time_ns", "cpu_time_ns", "peak_rss_bytes", "token_cost"} {
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
		div := br["genome_diversity"].(map[string]any)
		for _, key := range []string{"sum_pairwise_l1", "genome_pairs", "mean_pairwise_l1", "distinct_genomes", "locus_frequencies"} {
			if _, ok := div[key]; !ok {
				t.Fatalf("missing genome_diversity field %q", key)
			}
		}
		kin := br["kinship"].(map[string]any)
		for _, key := range []string{"living_pairs", "kin_pairs", "related_pair_fraction", "mean_finite_kin_distance",
			"inbreeding_refusals", "per_actor_living_children"} {
			if _, ok := kin[key]; !ok {
				t.Fatalf("missing kinship field %q", key)
			}
		}
		econ := br["birth_economics"].(map[string]any)
		for _, key := range []string{"births", "birth_requests", "claim_vs_truth_divergences", "refusals"} {
			if _, ok := econ[key]; !ok {
				t.Fatalf("missing birth_economics field %q", key)
			}
		}
		events := br["event_counts"].(map[string]any)
		for _, key := range []string{"gather_claims", "gather_admitted", "gather_denied", "eat", "eat_stored", "wait", "rest",
			"builds", "paired_meals", "paired_meal_no_meal_notes", "fallback_eatstored", "fallback_no_meal_notes",
			"birth_requests", "birth_replies", "births_accepted", "birth_refusals", "birth_refusals_silent",
			"birth_wake_claims", "lost_births"} {
			if _, ok := events[key]; !ok {
				t.Fatalf("missing event_counts field %q", key)
			}
		}
	}
}

// TestReproductionCLIQ0ExtinctionZeroBirths pins the q0 outcome band on both
// branches: identical extinction at h11 and zero birth evidence anywhere.
func TestReproductionCLIQ0ExtinctionZeroBirths(t *testing.T) {
	r := runForTest(t, options{yield: 0, seed: 0, workers: 1, hours: 12, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	for _, br := range r.Branches {
		if br.Population.ExtinctionHour != 11 || br.Population.AliveByHour[12] != 0 {
			t.Fatalf("q0 extinction band drift: births=%t extinction=%d alive=%v", br.Births, br.Population.ExtinctionHour, br.Population.AliveByHour)
		}
		if br.Events.BirthRequests != 0 || br.Events.BirthReplies != 0 || br.Totals.Births != 0 {
			t.Fatalf("q0 recorded birth evidence on births=%t: %+v", br.Births, br.Events)
		}
	}
	enabled, disabled := branchByFlag(t, r, true), branchByFlag(t, r, false)
	if !reflect.DeepEqual(enabled.Population.AliveByHour, disabled.Population.AliveByHour) {
		t.Fatal("q0 branches diverged")
	}
}

// TestReproductionCLIGenomeVariantFixture walks the recorded founder
// fixture: the report self-describes the declared loci, the founder rows
// carry them, the variant root digest differs from the neutral root digest
// of the same cell, and births-disabled variant cells are refused.
func TestReproductionCLIGenomeVariantFixture(t *testing.T) {
	variant := genomeVariantLoci()
	if len(variant) != 16 || variant[0] != 7 || variant[7] != 7 || variant[8] != 5 || variant[15] != 5 {
		t.Fatalf("recorded fixture drift: %v", variant)
	}
	r := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 12, births: enabledBranchFlag, genomeVariant: true, checkpointDir: newBundleDir(t)})
	br := r.Branches[0]
	if !r.GenomeVariant || !br.GenomeVariant || !reflect.DeepEqual(r.GenomeVariantLoci, variant) {
		t.Fatalf("variant fixture not self-described: %+v", r.GenomeVariantLoci)
	}
	for _, actor := range br.Actors {
		if actor.ID <= 16 && actor.LocusM != variant[actor.ID-1] {
			t.Fatalf("founder %d locus-m %d, want %d", actor.ID, actor.LocusM, variant[actor.ID-1])
		}
	}
	neutral := runForTest(t, options{yield: 8, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, checkpointDir: newBundleDir(t)})
	if neutral.NeutralCheckpoint.Digest == r.NeutralCheckpoint.Digest {
		t.Fatal("variant root digest equals the neutral root digest")
	}
	// The frozen matrix declares variant cells births-enabled-only.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, births := range []string{disabledBranchFlag, bothBranchesFlag} {
		if _, err := run(ctx, options{yield: 8, seed: 0, workers: 1, hours: 1, births: births, genomeVariant: true, checkpointDir: newBundleDir(t)}); err == nil {
			t.Fatalf("variant -births=%s accepted", births)
		}
	}
}

// TestReproductionCLIV3EqualityComparator compares the v4 disabled branch
// against v3-reference JSON in both directions: an exact self-reference is
// equal (alive trajectory, per-actor finals, totals), and a single perturbed
// field is detected and recorded verbatim.
func TestReproductionCLIV3EqualityComparator(t *testing.T) {
	r := runForTest(t, options{yield: 0, seed: 0, workers: 1, hours: 12, births: disabledBranchFlag, checkpointDir: newBundleDir(t)})
	br := r.Branches[0]
	reference := v3ReferenceReport{CompletedHour: br.CompletedHour, Branches: []v3ReferenceBranch{{
		Enabled: false, AliveByHour: br.Population.AliveByHour,
		Actors: []v3ReferenceActor{{
			ID: br.Actors[0].ID, Energy: br.Actors[0].Energy, Hunger: br.Actors[0].Hunger,
			Capital: br.Actors[0].Capital, Wip: br.Actors[0].Wip,
			GranaryStock: br.Actors[0].GranaryStock, InvestedUnits: br.Actors[0].InvestedUnits,
			PointsCreated: br.Actors[0].PointsCreated, PointsDecayed: br.Actors[0].PointsDecayed,
			YieldTotal: br.Actors[0].YieldTotal, YieldUnrealized: br.Actors[0].YieldUnrealized,
			StoredMeals: br.Actors[0].StoredMeals, WildMeals: br.Actors[0].WildMeals,
		}},
		Totals: v3ReferenceTotals{
			Alive: br.Totals.Alive, TotalK: br.Totals.Capital, TotalGranary: br.Totals.GranaryStock,
			TotalWip: br.Totals.Wip, TotalInvested: br.Totals.Invested,
			PointsCreated: br.Totals.PointsCreated, PointsDecayed: br.Totals.PointsDecayed,
			TotalWearDebt: br.Totals.TotalWearDebt, TotalYield: br.Totals.YieldTotal,
			TotalUnrealized: br.Totals.YieldUnrealized, TotalStoredMeals: br.Totals.StoredMeals,
			TotalWildMeals: br.Totals.Consumed, TotalHeld: br.Totals.Held, TotalStock: br.Totals.Stock,
			TotalPatchUnreal: br.Totals.Unrealized, TotalBasalSpent: br.Totals.BasalSpent, TotalCapLost: br.Totals.EnergyCapLost,
		},
	}}}
	data, err := json.Marshal(reference)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "reference.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	eq, err := compareV3Reference(path, options{hours: br.CompletedHour}, &br)
	if err != nil {
		t.Fatal(err)
	}
	if !eq.AliveEqual || !eq.FinalStateEqual || !eq.FinalStateCompared || len(eq.AliveMismatches) != 0 || len(eq.ActorMismatches) != 0 || len(eq.TotalsMismatches) != 0 {
		t.Fatalf("self-reference flagged: %+v", eq)
	}
	// A single perturbed trajectory hour must be detected verbatim.
	perturbed := reference
	perturbed.Branches[0].AliveByHour = append([]int(nil), br.Population.AliveByHour...)
	perturbed.Branches[0].AliveByHour[3]++
	data, err = json.Marshal(perturbed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	eq, err = compareV3Reference(path, options{hours: br.CompletedHour}, &br)
	if err != nil {
		t.Fatal(err)
	}
	if eq.AliveEqual || len(eq.AliveMismatches) != 1 || eq.AliveMismatches[0].Hour != 3 || eq.AliveMismatches[0].V3 != br.Population.AliveByHour[3]+1 {
		t.Fatalf("perturbed reference not detected: %+v", eq)
	}
	// A truncated reference horizon clamps the comparison instead of
	// comparing finals.
	reference.CompletedHour = 5
	reference.Branches[0].AliveByHour = br.Population.AliveByHour[:6]
	data, err = json.Marshal(reference)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	eq, err = compareV3Reference(path, options{hours: br.CompletedHour}, &br)
	if err != nil {
		t.Fatal(err)
	}
	if eq.ComparedThroughHour != 5 || eq.FinalStateCompared || !eq.AliveEqual {
		t.Fatalf("horizon clamp drift: %+v", eq)
	}
}

// TestReproductionCLIDisabledBranchBirthFreeEvents pins the history-level
// birth-free assertion: the disabled branch commits no rule 409-412 event.
func TestReproductionCLIDisabledBranchBirthFreeEvents(t *testing.T) {
	r := runForTest(t, options{yield: 5, seed: 7, workers: 1, hours: 3, births: disabledBranchFlag, checkpointDir: newBundleDir(t)})
	br := r.Branches[0]
	if !birthEvidenceKind("birth-request") || !birthEvidenceKind("birth-reply") || birthEvidenceKind("eat") {
		t.Fatal("birth evidence classification drifted")
	}
	if br.Population.ExtinctionHour >= 0 {
		t.Fatalf("q5 disabled went extinct inside the fixture: %d", br.Population.ExtinctionHour)
	}
}

// TestReproductionCLIRefusals pins the invalid-input refusals.
func TestReproductionCLIRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		name string
		o    options
	}{
		{"q-above-max", options{yield: 9, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"q-negative", options{yield: -1, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"workers-zero", options{yield: 3, seed: 0, workers: 0, hours: 2, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"hours-zero", options{yield: 3, seed: 0, workers: 1, hours: 0, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"hours-above-horizon", options{yield: 3, seed: 0, workers: 1, hours: world.ReproductionHorizonHours + 1, births: bothBranchesFlag, checkpointDir: newBundleDir(t)}},
		{"bad-births", options{yield: 3, seed: 0, workers: 1, hours: 2, births: "maybe", checkpointDir: newBundleDir(t)}},
		{"variant-disabled", options{yield: 3, seed: 0, workers: 1, hours: 2, births: disabledBranchFlag, genomeVariant: true, checkpointDir: newBundleDir(t)}},
		{"variant-both", options{yield: 3, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, genomeVariant: true, checkpointDir: newBundleDir(t)}},
		{"v3-reference-enabled", options{yield: 3, seed: 0, workers: 1, hours: 2, births: enabledBranchFlag, v3Reference: "x.json", checkpointDir: newBundleDir(t)}},
		{"v3-reference-variant", options{yield: 3, seed: 0, workers: 1, hours: 2, births: disabledBranchFlag, genomeVariant: true, v3Reference: "x.json", checkpointDir: newBundleDir(t)}},
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
	if _, err := run(ctx, options{yield: 3, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, out: out, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("existing output accepted")
	}
	dir := filepath.Join(base, "bundles")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, options{yield: 3, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, checkpointDir: dir}); err == nil {
		t.Fatal("existing checkpoint directory accepted")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("existing checkpoint directory populated: %v %v", entries, err)
	}
	// A missing v3 reference file is a typed failure.
	if _, err := run(ctx, options{yield: 0, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, v3Reference: filepath.Join(base, "absent.json"), checkpointDir: newBundleDir(t)}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing v3 reference mishandled: %v", err)
	}
}
