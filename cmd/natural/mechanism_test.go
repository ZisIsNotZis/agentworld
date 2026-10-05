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

// TestNaturalCLIMechanismZeroValueEqualsV5 pins the D1 default-preservation
// contract at the CLI layer for the mechanism axis: the zero-value mechanism
// selection and the explicit v5 selection produce identical reports (after
// stripping timing), and neither carries the mechanism key.
func TestNaturalCLIMechanismZeroValueEqualsV5(t *testing.T) {
	zero := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	explicit := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, mechanism: mechanismV5Name, checkpointDir: newBundleDir(t)})
	if !reflect.DeepEqual(stripTiming(zero), stripTiming(explicit)) {
		t.Fatal("zero-value mechanism selection diverged from explicit v5")
	}
	if zero.Mechanism != "" {
		t.Fatalf("v5 mechanism key serialized: %q", zero.Mechanism)
	}
	encoded, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"mechanism"`)) {
		t.Fatal("v5-world report carries the additive mechanism key")
	}
}

// TestNaturalCLIMechanismV6RunIdentity pins the mechanism-v6 run identity at
// the CLI layer: the report names the axis, the revision carries
// mechanism-v6/format-v6, and a v6-world natural/5 run still carries none of
// the v6 or v7 metric sections (byte-compatibility of the control arm).
func TestNaturalCLIMechanismV6RunIdentity(t *testing.T) {
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, policy: policy5Name, mechanism: mechanismV6Name, checkpointDir: newBundleDir(t)})
	if r.Mechanism != mechanismV6Name {
		t.Fatalf("mechanism key drift: %q", r.Mechanism)
	}
	if r.Revision != "natural mechanism-v6/format-v6/schema-v6/rule-v6/projection-v6/policy natural/5" {
		t.Fatalf("v6-world revision drift: %q", r.Revision)
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"v6_metrics"`, `"v6_bands"`, `"v7_metrics"`, `"v7_bands"`, `"provisions"`, `"provisioned"`} {
		if bytes.Contains(encoded, []byte(key)) {
			t.Fatalf("v6-world natural/5 report carries the additive key %s", key)
		}
	}
	// The same cell under natural/6 on world v6 (control gate 1's arm) must
	// carry the v6 sections and none of the v7 sections.
	r6 := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, policy: policy6Name, mechanism: mechanismV6Name, checkpointDir: newBundleDir(t)})
	encoded6, err := json.Marshal(r6)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"v6_metrics"`, `"v6_bands"`} {
		if !bytes.Contains(encoded6, []byte(key)) {
			t.Fatalf("v6-world natural/6 report lost the key %s", key)
		}
	}
	for _, key := range []string{`"v7_metrics"`, `"v7_bands"`, `"provisions"`} {
		if bytes.Contains(encoded6, []byte(key)) {
			t.Fatalf("v6-world natural/6 report carries the v7 key %s", key)
		}
	}
}

// TestNaturalCLIV7ReportSchema runs small natural/7 and natural/7b cells on
// world v6 and pins the additive report schema: the policy identity strings,
// every v7_metrics section and field, the per-actor and union counters, the
// time-to-first and non-universality blocks, the v7_bands entries, and the
// rule-515 provisions counters, all parseable from JSON.
func TestNaturalCLIV7ReportSchema(t *testing.T) {
	for _, tc := range []struct {
		policy     string
		revision   string
		provisions bool // whether the arm may record accepted provisions
	}{
		{policy7Name, "natural mechanism-v6/format-v6/schema-v6/rule-v6/projection-v6/policy natural/7", true},
		{policy7bName, "natural mechanism-v6/format-v6/schema-v6/rule-v6/projection-v6/policy natural/7b", false},
	} {
		r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, policy: tc.policy, mechanism: mechanismV6Name, checkpointDir: newBundleDir(t)})
		if r.Policy != tc.policy || r.Revision != tc.revision {
			t.Fatalf("%s identity strings drift: %q %q", tc.policy, r.Policy, r.Revision)
		}
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(encoded, &doc); err != nil {
			t.Fatal(err)
		}
		if doc["mechanism"].(string) != mechanismV6Name {
			t.Fatalf("report mechanism field: %v", doc["mechanism"])
		}
		branches := doc["branches"].([]any)
		for _, raw := range branches {
			br := raw.(map[string]any)
			m, ok := br["v7_metrics"].(map[string]any)
			if !ok {
				t.Fatalf("%s branch report carries no v7_metrics object", tc.policy)
			}
			for _, key := range []string{"definition", "sf_prime_by_hour_total", "sf_prime_run_total", "run_max_granary_stock",
				"provisioned_total", "paired_meals", "first_surplus_bearing_hour",
				"first_per_actor_sustained_hour", "first_per_actor_sustained_completion_hour",
				"actors_with_per_actor_sustained_runs", "surplus_bearing_actor_hours", "per_actor_sustained_hours",
				"per_actor_reading_note", "first_sustained_surplus_hour_union", "first_sustained_surplus_completion_hour_union",
				"max_consecutive_surplus_hours_union", "union_reading_note", "non_universality",
				"capital_persistence", "time_to_first", "parent_deaths_within_24h_of_birth_payment"} {
				if _, ok := m[key]; !ok {
					t.Fatalf("missing v7_metrics field %q", key)
				}
			}
			for _, id := range m["per_actor_sustained_hours"].([]any) {
				for _, key := range []string{"id", "max_consecutive_surplus_hours", "surplus_bearing_hours"} {
					if _, ok := id.(map[string]any)[key]; !ok {
						t.Fatalf("missing per_actor_sustained_hours field %q", key)
					}
				}
			}
			nu := m["non_universality"].(map[string]any)
			for _, key := range []string{"definition", "deaths_to_horizon", "at_first_per_actor_sustained_hour",
				"alive_at_first_sustained_hour", "surplus_bearing_actors_at_first_sustained_hour", "surplus_bearing_strict_subset_of_alive"} {
				if _, ok := nu[key]; !ok {
					t.Fatalf("missing non_universality field %q", key)
				}
			}
			ttf := m["time_to_first"].(map[string]any)
			for _, key := range []string{"definition", "first_provision_hour", "first_banked_hour", "first_k2_hold_hour",
				"first_per_actor_sustained_hour", "first_birth_filing_hour", "first_birth_hour"} {
				if _, ok := ttf[key]; !ok {
					t.Fatalf("missing time_to_first field %q", key)
				}
			}
			bands, ok := br["v7_bands"].([]any)
			if !ok || len(bands) == 0 {
				t.Fatalf("%s branch report carries no v7_bands", tc.policy)
			}
			for _, b := range bands {
				for _, key := range []string{"name", "criterion", "observed", "within_band"} {
					if _, ok := b.(map[string]any)[key]; !ok {
						t.Fatalf("missing v7_bands entry field %q", key)
					}
				}
			}
			if br["frozen_band"].(map[string]any)["name"] != bands[0].(map[string]any)["name"] {
				t.Fatal("frozen_band is not the primary predeclared v7 band")
			}
			events := br["event_counts"].(map[string]any)
			provisions, has := events["provisions"]
			if tc.provisions && !has {
				// The organic prefix may legitimately hold zero provisions on a
				// 4-hour window; the field may be absent when zero. Nothing to
				// assert beyond schema validity here — the value check lives in
				// the matrix evaluation.
				_ = provisions
			}
			if !tc.provisions {
				if _, has := events["provisions"]; has {
					t.Fatalf("natural/7b recorded provisions: %v", events["provisions"])
				}
				if _, has := events["provision_refused"]; has {
					t.Fatalf("natural/7b recorded refused provisions: %v", events["provision_refused"])
				}
			}
		}
		// The neutral A-cell prefix carries the V7-A2/A3/A4/A5 set in declared order.
		names := make([]string, 0, 4)
		for _, b := range branches[0].(map[string]any)["v7_bands"].([]any) {
			names = append(names, b.(map[string]any)["name"].(string))
		}
		if len(names) != 4 || !strings.HasPrefix(names[0], "V7-A2") || !strings.HasPrefix(names[1], "V7-A3") ||
			!strings.HasPrefix(names[2], "V7-A4") || !strings.HasPrefix(names[3], "V7-A5") {
			t.Fatalf("A-cell v7 band set drift: %v", names)
		}
	}
	// A v7 fixture cell exercises the mechanism-aware genesis seam end to end.
	rf := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 4, births: enabledBranchFlag, fixture: true, policy: policy7Name, mechanism: mechanismV6Name, checkpointDir: newBundleDir(t)})
	if rf.Root.Kind != rootKindFixture || rf.Branches[0].V7Metrics == nil {
		t.Fatal("v7 fixture cell lost its root kind or v7 sections")
	}
}

// TestNaturalCLIV7MetricsDetector pins the ticket-17 counter semantics on
// synthetic hourly evidence: the per-actor GATING counter keeps the true
// running maximum (immune to later resets, unlike the frozen v6 reporter's
// end-state field), and the union counter counts hours where at least one
// alive actor is surplus-bearing without requiring the same actor.
func TestNaturalCLIV7MetricsDetector(t *testing.T) {
	actor := func(grants int, stock int64, broken bool) world.NaturalActorState {
		k := int64(2)
		if broken {
			k = 0
		}
		return world.NaturalActorState{
			Body:     world.NaturalBodyState{Energy: 8},
			Worksite: world.NaturalWorksiteState{Capital: k},
			Granary:  world.NaturalGranaryState{Stock: stock, CultivatedTotal: int64(grants)},
			Genome:   world.NaturalGenomeState{DiedHour: world.NaturalNeverHour},
		}
	}
	// 26 hours; both actors hold k=2, granary=1 and SF′=1 every hour → both
	// bearing throughout.
	const hours = 26
	checks := make([]world.NaturalCheckpoint, hours)
	for h := range checks {
		checks[h] = world.NaturalCheckpoint{Hour: h, Slots: [][]world.NaturalSlotState{{}}}
		checks[h].Founders = []world.NaturalActorState{actor(h+1, 1, false), actor(h+1, 1, false)}
		checks[h].Metrics.Alive = 2
		checks[h].Balance = world.NaturalBalance{Capital: 4, GranaryStock: 2, CultivatedTotal: int64(h + 1)}
	}
	br := branchReport{FirstHour: 0}
	br.Population.AliveByHour = make([]int, hours)
	for i := range br.Population.AliveByHour {
		br.Population.AliveByHour[i] = 2
	}
	br.Population.BirthsByHour = make([]int64, hours)
	m, err := buildV7Metrics(&br, checks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.FirstPerActorSustainedHour != 0 || m.FirstPerActorSustainedCompletionHour != 11 || m.ActorsWithPerActorSustainedRuns != 2 {
		t.Fatalf("per-actor sustained detection drift: %+v", m)
	}
	for _, a := range m.PerActor {
		if a.MaxConsecutiveSurplusHours != hours || a.SurplusBearingHours != hours {
			t.Fatalf("per-actor counters drift: %+v", a)
		}
	}
	if m.FirstSustainedSurplusHourUnion != 0 || m.MaxConsecutiveSurplusHoursUnion != hours {
		t.Fatalf("union counters drift: %+v", m)
	}
	if m.NonUniversality.StrictSubset {
		t.Fatalf("non-universality capture drift (universal surplus must not be strict): %+v", m.NonUniversality)
	}
	if m.NonUniversality.AliveAtFirstSustainedHour != 2 ||
		len(m.NonUniversality.SurplusBearingAtFirstSustainedHour) != 2 {
		t.Fatalf("non-universality sets drift: %+v", m.NonUniversality)
	}
	if m.TimeToFirst.BankedHour != 0 || m.TimeToFirst.K2HoldHour != 0 || m.TimeToFirst.SustainedHour != 0 {
		t.Fatalf("time-to-first drift: %+v", m.TimeToFirst)
	}
	if m.SFPrimeRunTotal != hours || len(m.SFPrimeByHourTotal) != hours {
		t.Fatalf("SF′ totals drift: run=%d len=%d", m.SFPrimeRunTotal, len(m.SFPrimeByHourTotal))
	}
	// Reset immunity + strict subset: actor 1 bears h0..h14 (15 consecutive)
	// then breaks; the v7 entry must keep the running maximum 15 (the v6
	// reporter's end-state field would read 0), the union run is 15, and at
	// the first per-actor sustained hour the bearing set {1} is a strict
	// subset of the alive set {1,2} (actor 2 never holds k≥2).
	resetChecks := make([]world.NaturalCheckpoint, hours)
	for h := range resetChecks {
		resetChecks[h] = world.NaturalCheckpoint{Hour: h, Slots: [][]world.NaturalSlotState{{}}}
		resetChecks[h].Founders = []world.NaturalActorState{actor(h+1, 1, h >= 15), actor(h+1, 1, true)}
		resetChecks[h].Metrics.Alive = 2
		resetChecks[h].Balance = world.NaturalBalance{Capital: 2, GranaryStock: 1, CultivatedTotal: int64(h + 1)}
	}
	br.Population.AliveByHour = make([]int, hours)
	for i := range br.Population.AliveByHour {
		br.Population.AliveByHour[i] = 2
	}
	br.Population.BirthsByHour = make([]int64, hours)
	m, err = buildV7Metrics(&br, resetChecks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.PerActor[0].MaxConsecutiveSurplusHours != 15 || m.PerActor[0].SurplusBearingHours != 15 {
		t.Fatalf("running-maximum drift (v6-style end-state leak): %+v", m.PerActor[0])
	}
	if m.PerActor[1].MaxConsecutiveSurplusHours != 0 {
		t.Fatalf("non-bearing actor drift: %+v", m.PerActor[1])
	}
	if m.ActorsWithPerActorSustainedRuns != 1 || m.FirstPerActorSustainedHour != 0 || m.FirstPerActorSustainedCompletionHour != 11 {
		t.Fatalf("sustained detection after reset drift: %+v", m)
	}
	if !m.NonUniversality.StrictSubset || m.NonUniversality.AliveAtFirstSustainedHour != 2 ||
		len(m.NonUniversality.SurplusBearingAtFirstSustainedHour) != 1 || m.NonUniversality.SurplusBearingAtFirstSustainedHour[0] != 1 {
		t.Fatalf("strict-subset capture drift: %+v", m.NonUniversality)
	}
	if m.FirstSustainedSurplusHourUnion != 0 || m.MaxConsecutiveSurplusHoursUnion != 15 {
		t.Fatalf("union after reset drift: %+v", m)
	}
}

// TestNaturalCLIMechanismRefusals pins the --mechanism refusal surface and
// the policy/mechanism cross-legality: values outside {v5, v6} are refused,
// natural/7 and natural/7b are refused under the v5 mechanism, all before any
// publication (no output file, no checkpoint directory); stray arguments keep
// exit status 2 with the flags present, and no-clobber holds under a v7 arm.
func TestNaturalCLIMechanismRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		policy, mechanism string
	}{
		{policy5Name, "v7"},
		{policy5Name, "V5"},
		{policy6Name, "mechanism-v6"},
		{policy7Name, mechanismV5Name},  // provision arm under the v5 world
		{policy7bName, mechanismV5Name}, // ablation arm under the v5 world
		{policy7Name, ""},               // zero mechanism is exactly v5
	} {
		base := t.TempDir()
		out := filepath.Join(base, "report.json")
		dir := filepath.Join(base, "bundles")
		if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: disabledBranchFlag, policy: tc.policy, mechanism: tc.mechanism, out: out, checkpointDir: dir}); err == nil {
			t.Fatalf("invalid combination accepted: policy=%q mechanism=%q", tc.policy, tc.mechanism)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("output published despite invalid combination %q/%q: %v", tc.policy, tc.mechanism, err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("checkpoint directory created despite invalid combination %q/%q: %v", tc.policy, tc.mechanism, err)
		}
	}
	// No-clobber holds under natural/7.
	base := t.TempDir()
	out := filepath.Join(base, "report.json")
	if err := os.WriteFile(out, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, policy: policy7Name, mechanism: mechanismV6Name, out: out, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("existing output accepted under natural/7")
	}
	// CLI level: an illegal combination exits nonzero; stray arguments with
	// -mechanism present keep the documented exit-2 contract.
	binary := buildNaturalCLI(t)
	cmd := exec.CommandContext(ctx, binary, "-policy=natural/7", "-hours=1", "-out="+filepath.Join(t.TempDir(), "x.json"))
	if err := cmd.Run(); err == nil {
		t.Fatal("CLI accepted -policy=natural/7 without -mechanism=v6")
	}
	for _, extra := range []string{"pre", "post"} {
		args := []string{"-policy=" + policy7Name, "-mechanism=v6", "-hours=1"}
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
			t.Fatalf("stray args with -mechanism: exit %v (stderr %s)", err, stderr.String())
		}
		if _, err := os.Stat(out2); !os.IsNotExist(err) {
			t.Fatalf("output published despite stray args: %v", err)
		}
	}
}
