package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agentworld/internal/world"
)

// stripTiming removes the non-reproducible measured-rate and timing fields
// (the checkpoint directory and all wall/CPU/RSS measurements); everything
// else — trajectories, digests, byte counts, bands — must be identical
// between two deterministic runs.
func stripTiming(r report) report {
	r.WallTimeNS, r.CPUTimeNS, r.PeakRSSBytes, r.CheckpointDir = 0, 0, 0, ""
	for i := range r.Branches {
		r.Branches[i].WallTimeNS, r.Branches[i].CPUTimeNS, r.Branches[i].PeakRSSBytes = 0, 0, 0
		r.Branches[i].CostBudget.CPUPerSimulatedHour = 0
		r.Branches[i].CostBudget.CPUPerActorHour = 0
		r.Branches[i].CostBudget.CPUWithinBudget = r.Branches[i].CostBudget.CPUBudget >= 0
	}
	return r
}

// TestNaturalCLIPolicyZeroValueEqualsNatural5 pins the D1 default-preserving
// contract at the CLI layer: the zero-value policy selection and the explicit
// natural/5 selection produce identical reports (after stripping timing).
func TestNaturalCLIPolicyZeroValueEqualsNatural5(t *testing.T) {
	zero := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	explicit := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, policy: policy5Name, checkpointDir: newBundleDir(t)})
	if !reflect.DeepEqual(stripTiming(zero), stripTiming(explicit)) {
		t.Fatal("zero-value policy selection diverged from explicit natural/5")
	}
	if zero.Policy != policy5Name || zero.Revision != "natural format-v5/schema-v5/rule-v5/projection-v5/policy natural/5" {
		t.Fatalf("v5 identity strings drift: %q %q", zero.Policy, zero.Revision)
	}
}

// TestNaturalCLIPolicyV5ReportCarriesNoV6Keys pins v5 byte-compatibility: a
// natural/5 report marshals without any of the additive v6 keys.
func TestNaturalCLIPolicyV5ReportCarriesNoV6Keys(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"v6_metrics"`, `"v6_bands"`} {
		if bytes.Contains(encoded, []byte(key)) {
			t.Fatalf("natural/5 report carries the additive key %s", key)
		}
	}
}

// TestNaturalCLIPolicyV6ReportSchema runs a small v6 cell and pins the
// additive report schema: the policy identity strings, every v6_metrics
// section and field, and the v6_bands entries, all parseable from JSON.
func TestNaturalCLIPolicyV6ReportSchema(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, policy: policy6Name, checkpointDir: newBundleDir(t)})
	if r.Policy != policy6Name || r.Revision != "natural format-v5/schema-v5/rule-v5/projection-v5/policy natural/6" {
		t.Fatalf("v6 identity strings drift: %q %q", r.Policy, r.Revision)
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["policy"].(string) != policy6Name {
		t.Fatalf("report policy field: %v", doc["policy"])
	}
	branches := doc["branches"].([]any)
	if len(branches) != 2 {
		t.Fatalf("expected both branches, got %d", len(branches))
	}
	for _, raw := range branches {
		br := raw.(map[string]any)
		m, ok := br["v6_metrics"].(map[string]any)
		if !ok {
			t.Fatal("v6 branch report carries no v6_metrics object")
		}
		for _, key := range []string{"definition", "sf_by_hour_total", "sf_run_total", "run_max_granary_stock",
			"first_surplus_bearing_hour", "first_sustained_surplus_hour", "first_sustained_surplus_completion_hour",
			"max_consecutive_surplus_hours", "surplus_bearing_actor_hours", "per_actor_sustained_hours",
			"capital_persistence", "time_to_first", "parent_deaths_within_24h_of_birth_payment"} {
			if _, ok := m[key]; !ok {
				t.Fatalf("missing v6_metrics field %q", key)
			}
		}
		for _, id := range m["per_actor_sustained_hours"].([]any) {
			for _, key := range []string{"id", "max_consecutive_surplus_hours", "surplus_bearing_hours"} {
				if _, ok := id.(map[string]any)[key]; !ok {
					t.Fatalf("missing per_actor_sustained_hours field %q", key)
				}
			}
		}
		capital := m["capital_persistence"].(map[string]any)
		for _, key := range []string{"definition", "share_alive_hours_k_ge1", "k_total_by_hour", "slot_turnover_total", "slot_turnover_per_patch_final_24h"} {
			if _, ok := capital[key]; !ok {
				t.Fatalf("missing capital_persistence field %q", key)
			}
		}
		ttf := m["time_to_first"].(map[string]any)
		for _, key := range []string{"definition", "first_build_choice_hour", "first_build_attempt_hour",
			"first_committed_build_hour", "first_build_denial_attempt_hour", "first_tenured_slot_hour",
			"first_birth_filing_hour", "first_birth_hour"} {
			if _, ok := ttf[key]; !ok {
				t.Fatalf("missing time_to_first field %q", key)
			}
		}
		deaths := m["parent_deaths_within_24h_of_birth_payment"].(map[string]any)
		for _, key := range []string{"definition", "parents", "entries"} {
			if _, ok := deaths[key]; !ok {
				t.Fatalf("missing parent_deaths field %q", key)
			}
		}
		bands, ok := br["v6_bands"].([]any)
		if !ok || len(bands) == 0 {
			t.Fatal("v6 branch report carries no v6_bands")
		}
		for _, b := range bands {
			for _, key := range []string{"name", "criterion", "observed", "within_band"} {
				if _, ok := b.(map[string]any)[key]; !ok {
					t.Fatalf("missing v6_bands entry field %q", key)
				}
			}
		}
		if len(r.Branches) > 0 && br["frozen_band"].(map[string]any)["name"] != bands[0].(map[string]any)["name"] {
			t.Fatal("frozen_band is not the primary predeclared v6 band")
		}
	}
	// The neutral A-cell prefix carries the A2/A3/A4 set in declared order.
	names := make([]string, 0, 3)
	for _, b := range branches[0].(map[string]any)["v6_bands"].([]any) {
		names = append(names, b.(map[string]any)["name"].(string))
	}
	if len(names) != 3 || !strings.HasPrefix(names[0], "A2") || !strings.HasPrefix(names[1], "A3") || !strings.HasPrefix(names[2], "A4") {
		t.Fatalf("A-cell v6 band set drift: %v", names)
	}
}

// TestNaturalCLIPolicyV6FixtureCell pins the v6 fixture path (cell E under
// natural/6, through the NewNaturalFixtureForPolicy seam): births commit and
// the E band is evaluated under the v6 criterion.
func TestNaturalCLIPolicyV6FixtureCell(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 5, births: enabledBranchFlag, fixture: true, policy: policy6Name, checkpointDir: newBundleDir(t)})
	if r.Policy != policy6Name || !r.FounderTenure || r.Root.Kind != rootKindFixture {
		t.Fatalf("v6 fixture root drift: %+v", r.Root)
	}
	br := r.Branches[0]
	if br.Totals.Births == 0 || br.V6Metrics == nil || br.V6Bands == nil {
		t.Fatalf("v6 fixture cell produced no births or no v6 sections: births=%d metrics=%v", br.Totals.Births, br.V6Metrics)
	}
	if !strings.HasPrefix(br.Band.Name, "E") || !br.Band.WithinBand {
		t.Fatalf("v6 E band drift: %+v", br.Band)
	}
	if br.V6Metrics.TimeToFirst.BirthHour < 0 {
		t.Fatalf("v6 time-to-first-birth missing: %+v", br.V6Metrics.TimeToFirst)
	}
}

// TestNaturalCLIPolicyRefusals pins the --policy refusal surface: values
// outside the frozen natural ref set are refused before any publication
// (no output file, no checkpoint directory), at both the run and CLI level;
// stray arguments keep exit status 2 with the flag present, and no-clobber
// holds under natural/6.
func TestNaturalCLIPolicyRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, policy := range []string{"natural/7", "natural", "natural/6 extra", "v6", "NATURAL/6"} {
		base := t.TempDir()
		out := filepath.Join(base, "report.json")
		dir := filepath.Join(base, "bundles")
		if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: disabledBranchFlag, policy: policy, out: out, checkpointDir: dir}); err == nil {
			t.Fatalf("invalid -policy %q accepted", policy)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("output published despite invalid -policy %q: %v", policy, err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("checkpoint directory created despite invalid -policy %q: %v", policy, err)
		}
	}
	// No-clobber holds under natural/6.
	base := t.TempDir()
	out := filepath.Join(base, "report.json")
	if err := os.WriteFile(out, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, policy: policy6Name, out: out, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("existing output accepted under natural/6")
	}
	// CLI level: an invalid ref exits nonzero, and stray arguments with the
	// flag present keep the documented exit-2 contract.
	binary := buildNaturalCLI(t)
	cmd := exec.CommandContext(ctx, binary, "-policy=natural/7", "-hours=1", "-out="+filepath.Join(t.TempDir(), "x.json"))
	if err := cmd.Run(); err == nil {
		t.Fatal("CLI accepted -policy=natural/7")
	}
	for _, extra := range []string{"pre", "post"} {
		args := []string{"-policy=" + policy6Name, "-hours=1"}
		out2 := filepath.Join(t.TempDir(), "y.json")
		if extra == "pre" {
			args = append([]string{"stray"}, args...)
		} else {
			args = append(args, "stray")
		}
		args = append(args, "-out="+out2)
		cmd := exec.CommandContext(ctx, binary, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatalf("stray args with -policy: exit %v (stderr %s)", err, stderr.String())
		}
		if _, err := os.Stat(out2); !os.IsNotExist(err) {
			t.Fatalf("output published despite stray args: %v", err)
		}
	}
}

// TestNaturalCLIV6SurplusRunDetector pins the sustained-surplus detector on
// synthetic hourly evidence: a 13-hour qualifying run is detected with its
// first hour at h0 and completion at h11, and a broken run (granary emptied
// at h6) yields max 6 consecutive hours and no sustained surplus.
func TestNaturalCLIV6SurplusRunDetector(t *testing.T) {
	actor := func(grants int, stock int64) world.NaturalActorState {
		return world.NaturalActorState{
			Body:     world.NaturalBodyState{Energy: 8},
			Worksite: world.NaturalWorksiteState{Capital: 2},
			Granary:  world.NaturalGranaryState{Stock: stock, CultivatedTotal: int64(grants)},
			Genome:   world.NaturalGenomeState{DiedHour: world.NaturalNeverHour},
		}
	}
	checks := func(broken bool) []world.NaturalCheckpoint {
		out := make([]world.NaturalCheckpoint, 13)
		for h := range out {
			grants := h + 1 // +1 cultivated grant per hour: SF = 1 every hour
			stock := int64(1)
			k := int64(2)
			if broken && h >= 6 {
				stock, k = 0, 2 // granary emptied: the surplus-bearing predicate fails
			}
			out[h] = world.NaturalCheckpoint{Hour: h, Slots: [][]world.NaturalSlotState{{}}}
			out[h].Founders = []world.NaturalActorState{actor(grants, stock)}
			out[h].Founders[0].Worksite.Capital = k
			out[h].Balance = world.NaturalBalance{Capital: k, GranaryStock: stock, CultivatedTotal: int64(grants)}
		}
		return out
	}
	br := branchReport{FirstHour: 0}
	br.Population.AliveByHour = make([]int, 13)
	for i := range br.Population.AliveByHour {
		br.Population.AliveByHour[i] = 1
	}
	br.Population.BirthsByHour = make([]int64, 13)
	m, err := buildV6Metrics(&br, checks(false), nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.FirstSustainedSurplusHour != 0 || m.FirstSustainedSurplusCompletionHour != 11 || m.MaxConsecutiveSurplusHours != 13 {
		t.Fatalf("sustained-run detection drift: %+v", m)
	}
	if m.SFRunTotal != 13 || m.FirstSurplusBearingHour != 0 {
		t.Fatalf("SF totals drift: run_total=%d first_bearing=%d", m.SFRunTotal, m.FirstSurplusBearingHour)
	}
	m, err = buildV6Metrics(&br, checks(true), nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.MaxConsecutiveSurplusHours != 6 || m.FirstSustainedSurplusHour != -1 || m.FirstSustainedSurplusCompletionHour != -1 {
		t.Fatalf("broken-run detection drift: %+v", m)
	}
}

// TestNaturalCLIV6JournalTimeToFirst pins the journal-derived time-to-first
// fields on a small v6 cell: the first birth filing and the build-choice
// hour are absolute hours carried into the report.
func TestNaturalCLIV6JournalTimeToFirst(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, policy: policy6Name, checkpointDir: newBundleDir(t)})
	br := r.Branches[0]
	m := br.V6Metrics
	if m == nil {
		t.Fatal("no v6 metrics")
	}
	if m.TimeToFirst.BuildChoiceHour < 0 {
		t.Fatalf("no build choice recorded in the trap window: %+v", m.TimeToFirst)
	}
	if m.TimeToFirst.BuildChoiceHour > 3 {
		t.Fatalf("build choice hour out of the run window: %+v", m.TimeToFirst)
	}
	// The v6 re-pioneering flips ((c)/(d): build at E=4 from an empty larder)
	// commit a slot claim inside this prefix — the empirical witness that the
	// v6 cascade diverges from the v5 trap, pinned deterministic by the world.
	if m.TimeToFirst.SlotTenureHour < 0 || m.TimeToFirst.SlotTenureHour > 3 {
		t.Fatalf("v6 slot tenure not in the run window: %d", m.TimeToFirst.SlotTenureHour)
	}
	if len(m.SFByHourTotal) != 5 || len(m.CapitalPersistence.KTotalByHour) != 5 {
		t.Fatalf("per-hour series length drift: %d %d", len(m.SFByHourTotal), len(m.CapitalPersistence.KTotalByHour))
	}
	var actorHours int64
	for _, a := range br.Population.AliveByHour {
		actorHours += int64(a)
	}
	if actorHours > 0 && (m.CapitalPersistence.ShareAliveHoursKGE1 < 0 || m.CapitalPersistence.ShareAliveHoursKGE1 > 1) {
		t.Fatalf("capital persistence share out of range: %v", m.CapitalPersistence.ShareAliveHoursKGE1)
	}
	// The v5 control arm of the same cell must not carry the sections.
	v5 := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	for _, b := range v5.Branches {
		if b.V6Metrics != nil || b.V6Bands != nil {
			t.Fatal("natural/5 branch carries v6 sections")
		}
	}
}
