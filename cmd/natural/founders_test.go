package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// TestNaturalCLIFoundersZeroDefaultIdentity pins the ticket-18
// default-preservation contract at the CLI layer: the zero -founders spelling
// and the explicit full-block spelling (-founders=16 at P=2) publish
// byte-identical bundles (the L1 digest identity, observed end to end through
// the CLI), the resolved founders field reads 8P either way, and the
// default-band report carries none of the additive v8 keys while the explicit
// full-block report carries the saturated-control density block.
func TestNaturalCLIFoundersZeroDefaultIdentity(t *testing.T) {
	zero := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, checkpointDir: newBundleDir(t)})
	explicit := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, founders: 8 * world.NaturalDefaultPatches, checkpointDir: newBundleDir(t)})
	if zero.Founders != 16 || explicit.Founders != 16 {
		t.Fatalf("resolved founders drift: %d %d", zero.Founders, explicit.Founders)
	}
	if zero.Root.Digest != explicit.Root.Digest {
		t.Fatalf("the default spelling and the explicit full block diverged at the root bundle: %q %q", zero.Root.Digest, explicit.Root.Digest)
	}
	encoded, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"v8_metrics"`, `"v8_bands"`, `"density_world"`, `"founders_declared"`, `"founders_resolved"`, `"per_actor_surplus"`, `"birth_cohort"`, `"filing_kin_distance"`, `"endowment_ledger_units"`, `"mouths_by_hour"`} {
		if bytes.Contains(encoded, []byte(key)) {
			t.Fatalf("default-band report carries the additive v8 key %s", key)
		}
	}
	if explicit.Branches[0].V8Metrics == nil || len(explicit.Branches[0].V8Bands) == 0 {
		t.Fatal("explicit full-block band lost the v8 sections (the saturated control arm)")
	}
	vm := explicit.Branches[0].V8Metrics
	if vm.FoundersDeclared != 16 || vm.FoundersResolved != 16 || vm.DensityWorld != 1 {
		t.Fatalf("full-block density drift: %+v", vm)
	}
	for _, p := range vm.Patches {
		if !p.Saturated {
			t.Fatalf("full-block patch %d not saturated: %+v", p.Patch, p)
		}
	}
}

// TestNaturalCLIFoundersRunIdentityAndDensityBlock runs the gating A12 shape
// (P=2, f=12) end to end on a small horizon and pins the run identity and the
// additive density block: the resolved band, the genesis seating (8 founders
// on the saturated patch 1001, 4 on the under-capacity patch 1002), the
// per-actor home-patch attribution, and the primary V8-A2 frozen band.
func TestNaturalCLIFoundersRunIdentityAndDensityBlock(t *testing.T) {
	r := runForTest(t, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, founders: 12, checkpointDir: newBundleDir(t)})
	if r.Founders != 12 {
		t.Fatalf("resolved founders drift: %d", r.Founders)
	}
	for _, br := range r.Branches {
		if len(br.Actors) != 12 {
			t.Fatalf("founder cardinality drift: %d", len(br.Actors))
		}
		vm := br.V8Metrics
		if vm == nil {
			t.Fatal("declared-band report carries no v8_metrics")
		}
		if vm.FoundersDeclared != 12 || vm.FoundersResolved != 12 || vm.DensityWorld != 0.75 {
			t.Fatalf("density axis drift: %+v", vm)
		}
		if len(vm.Patches) != 2 {
			t.Fatalf("patch column count drift: %d", len(vm.Patches))
		}
		first := vm.Patches[0]
		if first.Patch != 1001 || first.FoundersSeated != 8 || !first.Saturated {
			t.Fatalf("saturated patch column drift: %+v", first)
		}
		second := vm.Patches[1]
		if second.Patch != 1002 || second.FoundersSeated != 4 || second.Saturated {
			t.Fatalf("under-capacity patch column drift: %+v", second)
		}
		if len(vm.PerActor) != 12 {
			t.Fatalf("per-actor surplus rows drift: %d", len(vm.PerActor))
		}
		on1001, on1002 := 0, 0
		for _, a := range vm.PerActor {
			switch a.HomePatch {
			case 1001:
				on1001++
			case 1002:
				on1002++
			default:
				t.Fatalf("per-actor home patch outside the seating: %+v", a)
			}
		}
		if on1001 != 8 || on1002 != 4 {
			t.Fatalf("home-patch attribution drift: %d on 1001, %d on 1002", on1001, on1002)
		}
		for _, p := range vm.Patches {
			if len(p.MouthsByHour) != 5 || len(p.KTotalByHour) != 5 {
				t.Fatalf("hourly series length drift: %+v", p)
			}
		}
		if vm.GenerationCount != 0 || len(vm.BirthCohort) != 0 {
			t.Fatalf("organic prefix must be sterile and generation-zero: %+v", vm)
		}
		if vm.EndowmentLedgerUnits != 0 || !vm.EndowmentCrossCheckedInG3 {
			t.Fatalf("endowment ledger drift: %+v", vm)
		}
		// The enabled branch carries the organic A set with V8-A2 as the
		// frozen band; the births-disabled branch is the C12-shaped cell and
		// carries V8-C2 as its own primary band.
		if br.Births {
			if len(br.V8Bands) != 6 || !strings.HasPrefix(br.V8Bands[0].Name, "V8-A2") {
				t.Fatalf("v8 band set drift: %+v", br.V8Bands)
			}
			if !strings.HasPrefix(br.Band.Name, "V8-A2") {
				t.Fatalf("frozen band must take the primary V8-A2 place: %+v", br.Band)
			}
		} else {
			if len(br.V8Bands) != 1 || !strings.HasPrefix(br.V8Bands[0].Name, "V8-C2") {
				t.Fatalf("births-disabled v8 band drift: %+v", br.V8Bands)
			}
			if !strings.HasPrefix(br.Band.Name, "V8-C2") {
				t.Fatalf("births-disabled frozen band must be V8-C2: %+v", br.Band)
			}
		}
	}
}

// TestNaturalCLIFounderBandFixtureCohort runs the E12 fixture cell (f=12,
// births-enabled) and pins the birth-cohort plumbing end to end: the
// fixture's recorded births appear as cohort rows with generation ≥ 1 and the
// whole-run birth firsts, with the v8 descriptive fixture band as the frozen
// band.
func TestNaturalCLIFounderBandFixtureCohort(t *testing.T) {
	r := runForTest(t, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 5, births: enabledBranchFlag, fixture: true, founders: 12, checkpointDir: newBundleDir(t)})
	if r.Root.Kind != rootKindFixture || r.Founders != 12 {
		t.Fatalf("fixture cell identity drift: %+v", r.Root)
	}
	br := r.Branches[0]
	if br.Totals.Births == 0 {
		t.Fatalf("E12 fixture cell produced no births: %+v", br.Totals)
	}
	vm := br.V8Metrics
	if vm == nil {
		t.Fatal("E12 fixture cell carries no v8_metrics")
	}
	if len(vm.BirthCohort) != int(br.Totals.Births) {
		t.Fatalf("cohort cardinality drift: %d rows vs %d births", len(vm.BirthCohort), br.Totals.Births)
	}
	if vm.GenerationCount < 1 {
		t.Fatalf("generation index drift: %+v", vm)
	}
	if vm.EndowmentLedgerUnits != 8*br.Totals.Births {
		t.Fatalf("endowment ledger drift: %d vs 8·%d", vm.EndowmentLedgerUnits, br.Totals.Births)
	}
	for _, row := range vm.BirthCohort {
		if row.BirthHour < int64(br.FirstHour) || row.ParentA == 0 || row.ParentB == 0 {
			t.Fatalf("cohort row drift: %+v", row)
		}
		if row.Generation < 1 {
			t.Fatalf("newborn generation drift: %+v", row)
		}
	}
	if !strings.HasPrefix(br.Band.Name, "V8-E12") || !br.Band.WithinBand {
		t.Fatalf("E12 descriptive band drift: %+v", br.Band)
	}
}

// TestNaturalCLIFoundersRefusedBeforePublication pins the refusal surface of
// the -founders flag: every out-of-band or undeclared-combination value is
// refused before any publication (no output file, no checkpoint directory),
// the CLI exits nonzero on an out-of-band value, and stray positional
// arguments keep the exit-2 contract with -founders present.
func TestNaturalCLIFoundersRefusedBeforePublication(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		name string
		o    options
	}{
		{"negative", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, founders: -1, checkpointDir: newBundleDir(t)}},
		{"above-8P", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, founders: 17, checkpointDir: newBundleDir(t)}},
		{"above-8P-single-patch", options{patches: 1, r: 8, seed: 0, workers: 1, hours: 2, births: bothBranchesFlag, founders: 9, checkpointDir: newBundleDir(t)}},
		{"fixture-f1", options{patches: 2, r: 8, seed: 0, workers: 1, hours: 5, births: enabledBranchFlag, fixture: true, founders: 1, checkpointDir: newBundleDir(t)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			out := filepath.Join(base, "report.json")
			dir := filepath.Join(base, "bundles")
			o := tc.o
			o.out, o.checkpointDir = out, dir
			if _, err := run(ctx, o); err == nil {
				t.Fatalf("invalid -founders accepted: %+v", o)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("output published despite invalid -founders: %v", err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("checkpoint directory created despite invalid -founders: %v", err)
			}
		})
	}
	binary := buildNaturalCLI(t)
	out := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.CommandContext(ctx, binary, "-founders=99", "-patches=2", "-hours=1", "-out="+out)
	if err := cmd.Run(); err == nil {
		t.Fatal("CLI accepted -founders=99 at patches=2")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output published despite out-of-band -founders: %v", err)
	}
	for _, extra := range []string{"pre", "post"} {
		args := []string{"-founders=12", "-patches=2", "-hours=1"}
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
			t.Fatalf("stray args with -founders: exit %v (stderr %s)", err, stderr.String())
		}
		if _, err := os.Stat(out2); !os.IsNotExist(err) {
			t.Fatalf("output published despite stray args: %v", err)
		}
	}
}

// v8FounderState builds one synthetic founder state for the detector fixtures.
func v8FounderState(home int64, k, granary, cult int64) world.NaturalActorState {
	return world.NaturalActorState{
		Body:     world.NaturalBodyState{Energy: 8},
		Bag:      world.NaturalBagState{Source: sim.EntityID(home)},
		Worksite: world.NaturalWorksiteState{Capital: k},
		Granary:  world.NaturalGranaryState{Stock: granary, CultivatedTotal: cult},
		Genome:   world.NaturalGenomeState{DiedHour: world.NaturalNeverHour},
	}
}

// v8NewbornState builds one synthetic newborn state (home patch = proposer's,
// parent rows and birth hour carried by the genome).
func v8NewbornState(home int64, parentA, parentB sim.EntityID, birthHour int64) world.NaturalActorState {
	return world.NaturalActorState{
		Body:   world.NaturalBodyState{Energy: 8},
		Bag:    world.NaturalBagState{Source: sim.EntityID(home)},
		Genome: world.NaturalGenomeState{DiedHour: world.NaturalNeverHour, ParentA: parentA, ParentB: parentB, BirthHour: birthHour},
	}
}

// v8SurplusActor plans one surplus-bearing actor: k=2, granary=1 and one
// cultivated unit per hour from its start hour (the SF′ window's only moving
// term) through the stop hour, then broken (k=0, cultivation frozen).
func v8SurplusActor(start, stop int) func(h int) world.NaturalActorState {
	return func(h int) world.NaturalActorState {
		if h < start || h >= stop {
			return v8FounderState(0, 0, 0, 0)
		}
		return v8FounderState(0, 2, 1, int64(h-start+1))
	}
}

// v8Homes seats the synthetic fixture's founders by the frozen ID geometry:
// founders 1–8 on patch 1001 (saturated), 9–12 on patch 1002 (under
// capacity).
var v8Homes = map[int]int64{1: 1001, 2: 1001, 3: 1001, 4: 1001, 5: 1001, 6: 1001, 7: 1001, 8: 1001, 9: 1002, 10: 1002, 11: 1002, 12: 1002}

// v8CohortNewborns returns the synthetic birth cohort of the detector
// fixtures: one newborn (id 2001) of founders 1 and 2 on patch 1002 at h3.
func v8CohortNewborns() []world.NaturalNewbornState {
	return []world.NaturalNewbornState{{ID: 2001, State: v8NewbornState(1002, 1, 2, 3)}}
}

// v8DetectorFixture builds 26 synthetic hourly checkpoints with 12 founders
// seated 8+4 on two patches, an optional newborn on patch 1002 from h3, one
// cultivated slot on patch 1002 from h0, and one cumulative age death. plans
// maps founder index (1-based) to a per-hour state builder; unplanned
// founders sit at k=0/granary=0 (alive, never bearing).
func v8DetectorFixture(plans map[int]func(h int) world.NaturalActorState, newborns []world.NaturalNewbornState) []world.NaturalCheckpoint {
	const hours = 26
	state := func(idx, h int) world.NaturalActorState {
		if plan, ok := plans[idx]; ok {
			s := plan(h)
			s.Bag.Source = sim.EntityID(v8Homes[idx])
			return s
		}
		return v8FounderState(v8Homes[idx], 0, 0, 0)
	}
	checks := make([]world.NaturalCheckpoint, hours)
	for h := range checks {
		c := world.NaturalCheckpoint{Hour: h, Slots: make([][]world.NaturalSlotState, 2), Metrics: world.NaturalMetrics{DeathsAge: 1}}
		for j := 1; j <= 12; j++ {
			c.Founders = append(c.Founders, state(j, h))
		}
		for _, nb := range newborns {
			if nb.State.Genome.BirthHour <= int64(h) {
				c.Newborns = append(c.Newborns, nb)
			}
		}
		c.Metrics.Alive = 12 + len(c.Newborns)
		// Patch 1001: all wild. Patch 1002: one cultivated slot (owner 9)
		// from h0 — the first tenured slot of the under-capacity patch.
		c.Slots[0] = []world.NaturalSlotState{{}}
		c.Slots[1] = []world.NaturalSlotState{{Mode: world.NaturalTenureCultivated, Owner: 9, Cultivated: 1}}
		c.Balance = world.NaturalBalance{GranaryStock: 2}
		checks[h] = c
	}
	return checks
}

// v8DetectorBranch assembles a completed organic A-cell branch report over the
// supplied checkpoints (population arrays padded to the frozen horizon so the
// A-band set evaluates).
func v8DetectorBranch(t *testing.T, checks []world.NaturalCheckpoint, births bool) *branchReport {
	t.Helper()
	final := checks[len(checks)-1]
	br := &branchReport{Births: births, FirstHour: 0, CompletedHour: world.NaturalHorizonHours}
	br.Population.AliveByHour = make([]int, world.NaturalHorizonHours+1)
	br.Population.BirthsByHour = make([]int64, world.NaturalHorizonHours+1)
	br.Population.DeathsStarvationByHour = make([]int64, world.NaturalHorizonHours+1)
	br.Population.DeathsAgeByHour = make([]int64, world.NaturalHorizonHours+1)
	for i := range br.Population.AliveByHour {
		br.Population.AliveByHour[i] = int(final.Metrics.Alive)
	}
	for _, nb := range final.Newborns {
		br.Population.BirthsByHour[int(nb.State.Genome.BirthHour)-checks[0].Hour]++
	}
	br.Population.ExtinctionHour = -1
	br.Population.FinalAlive = int(final.Metrics.Alive)
	br.Events.PairedMeals = 7
	br.ConservationVerified = true
	br.Totals.Births = int64(len(final.Newborns))
	return br
}

// TestNaturalCLIV8MetricsDetector pins the ticket-18 counter semantics on
// synthetic hourly evidence: the per-actor GATING counter keeps the true
// running maximum (reset-immune), the home-patch attribution feeds V8-A2b
// (a saturated-patch sustained run fires the band), the per-patch seating,
// mouths and Σk columns match the hand-built fixture, the birth cohort
// carries the filing kin distance and generation index, and the V8-K
// windfall clause reads the first surplus-bearing actor's share.
func TestNaturalCLIV8MetricsDetector(t *testing.T) {
	plans := map[int]func(h int) world.NaturalActorState{
		1: func(h int) world.NaturalActorState { // sustained h0–h14, broken after (reset-immunity)
			if h < 15 {
				return v8FounderState(0, 2, 1, int64(h+1))
			}
			return v8FounderState(0, 0, 0, 15)
		},
		2: v8SurplusActor(10, 26), // sustained h10–h25
		9: v8SurplusActor(0, 26),  // sustained throughout on 1002
		10: func(int) world.NaturalActorState {
			return v8FounderState(0, 3, 0, 0) // k≥1 for Σk, never bearing
		},
		11: func(int) world.NaturalActorState {
			return v8FounderState(0, 3, 0, 0)
		},
	}
	journal := []world.NaturalBatch{{Time: sim.SimTime(5 * world.NaturalHour), Attempts: []world.NaturalAttempt{{Actor: 2, Time: sim.SimTime(5 * world.NaturalHour), Kind: "birth-request"}}}}
	checks := v8DetectorFixture(plans, v8CohortNewborns())
	o := options{patches: 2, founders: 12, r: 8}
	br := v8DetectorBranch(t, checks, true)
	vm, err := buildV8Metrics(o, br, checks, journal)
	if err != nil {
		t.Fatal(err)
	}
	br.V8Metrics = &vm

	// Density columns.
	if vm.Patches[0].FoundersSeated != 8 || !vm.Patches[0].Saturated || vm.Patches[1].FoundersSeated != 4 || vm.Patches[1].Saturated {
		t.Fatalf("seating drift: %+v", vm.Patches)
	}
	if vm.DensityWorld != 0.75 {
		t.Fatalf("density drift: %v", vm.DensityWorld)
	}
	for h := range vm.Patches[0].MouthsByHour {
		// The synthetic newborn sits on patch 1002 (the proposer's patch).
		if vm.Patches[0].MouthsByHour[h] != 8 {
			t.Fatalf("patch-1001 mouths drift at h%d: %d", h, vm.Patches[0].MouthsByHour[h])
		}
		want1002 := 4
		if h >= 3 {
			want1002 = 5
		}
		if vm.Patches[1].MouthsByHour[h] != want1002 {
			t.Fatalf("patch-1002 mouths drift at h%d: %d", h, vm.Patches[1].MouthsByHour[h])
		}
	}
	// Σk: patch 1001 = 2 while only actor 1 holds k=2 (h0–h9), 2+2 while
	// both hold (h10–h14), then 0+2 after the break; patch 1002 = 2+3+3.
	for h := range vm.Patches[0].KTotalByHour {
		want := int64(2)
		if h >= 10 && h < 15 {
			want = 4
		} else if h >= 15 {
			want = 2
		}
		if vm.Patches[0].KTotalByHour[h] != want {
			t.Fatalf("patch-1001 Σk drift at h%d: %d (want %d)", h, vm.Patches[0].KTotalByHour[h], want)
		}
		if vm.Patches[1].KTotalByHour[h] != 8 {
			t.Fatalf("patch-1002 Σk drift at h%d: %d", h, vm.Patches[1].KTotalByHour[h])
		}
	}
	// Patch-attributable firsts.
	if vm.Patches[1].FirstTenuredSlotHour != 0 || vm.Patches[0].FirstTenuredSlotHour != -1 {
		t.Fatalf("tenured-slot attribution drift: %+v", vm.Patches)
	}
	// Both patches bank at h0: actor 1 (granary 1 from h0) sits on 1001,
	// actor 9 on 1002.
	if vm.Patches[1].FirstBankedHour != 0 || vm.Patches[0].FirstBankedHour != 0 {
		t.Fatalf("banked-hour attribution drift: %+v", vm.Patches)
	}
	if vm.Patches[0].FirstK2HoldHour != 0 || vm.Patches[1].FirstK2HoldHour != 0 {
		t.Fatalf("k≥2 attribution drift: %+v", vm.Patches)
	}
	if vm.Patches[0].FirstPerActorSustainedHour != 0 || vm.Patches[1].FirstPerActorSustainedHour != 0 {
		t.Fatalf("sustained attribution drift: %+v", vm.Patches)
	}
	if vm.Patches[0].FirstBirthFilingHour != 5 || vm.Patches[1].FirstBirthFilingHour != -1 {
		t.Fatalf("filing attribution drift: %+v", vm.Patches)
	}
	if vm.Patches[1].FirstBirthHour != 3 || vm.Patches[0].FirstBirthHour != -1 {
		t.Fatalf("birth attribution drift: %+v", vm.Patches)
	}

	// Per-actor counters with the true running maximum.
	byID := map[int]v8ActorSurplusReport{}
	for _, a := range vm.PerActor {
		byID[a.ID] = a
	}
	if a := byID[1]; a.MaxConsecutiveSurplusHours != 15 || a.SurplusBearingHours != 15 || a.FirstSurplusBearingHour != 0 || a.FirstPerActorSustainedHour != 0 || a.AliveHours != 26 || a.AliveHoursWithKGE1 != 15 {
		t.Fatalf("actor-1 counters drift (a v6-style end-state leak would read 0): %+v", a)
	}
	if a := byID[2]; a.MaxConsecutiveSurplusHours != 16 || a.FirstPerActorSustainedHour != 10 {
		t.Fatalf("actor-2 counters drift: %+v", a)
	}
	if a := byID[9]; a.MaxConsecutiveSurplusHours != 26 || a.FirstPerActorSustainedHour != 0 || a.HomePatch != 1002 {
		t.Fatalf("actor-9 counters drift: %+v", a)
	}
	if a := byID[2001]; a.Generation != 1 || a.HomePatch != 1002 {
		t.Fatalf("newborn surplus row drift: %+v", a)
	}
	if vm.FirstPerActorSustainedHour != 0 || vm.FirstPerActorSustainedCompletionHour != 11 || vm.ActorsWithPerActorSustainedRuns != 3 {
		t.Fatalf("sustained detection drift: %+v", vm)
	}
	if vm.FirstSustainedSurplusHourUnion != 0 || vm.MaxConsecutiveSurplusHoursUnion != 26 {
		t.Fatalf("union counters drift: %+v", vm)
	}

	// Cohort, generation index, endowment and canaries.
	if len(vm.BirthCohort) != 1 || vm.GenerationCount != 1 {
		t.Fatalf("cohort drift: %+v", vm.BirthCohort)
	}
	row := vm.BirthCohort[0]
	if row.ID != 2001 || row.BirthHour != 3 || row.HomePatch != 1002 || row.ParentA != 1 || row.ParentB != 2 || row.Generation != 1 || row.FilingKinDistance != -1 {
		t.Fatalf("cohort row drift (unrelated founders must read -1): %+v", row)
	}
	if vm.EndowmentLedgerUnits != 8 || vm.PairedMeals != 7 || !vm.EndowmentCrossCheckedInG3 {
		t.Fatalf("endowment/canary drift: %+v", vm)
	}
	if vm.TimeToFirst.FirstBirthFilingHour != 5 || vm.TimeToFirst.FirstBirthHour != 3 || vm.TimeToFirst.FirstTenuredSlotHour != 0 || vm.TimeToFirst.FirstBankedHour != 0 || vm.TimeToFirst.FirstK2HoldHour != 0 || vm.TimeToFirst.FirstPerActorSustainedHour != 0 {
		t.Fatalf("whole-run time-to-first drift: %+v", vm.TimeToFirst)
	}

	// The A-band set: A2 in, A2b FIRED by the patch-1001 sustained runs,
	// A3 in, A5 in (one death, strict subset), K in.
	bands := v8BandsFor(o, br)
	names := make([]string, 0, len(bands))
	for _, b := range bands {
		names = append(names, b.Name)
	}
	wantOrder := []string{"V8-A2", "V8-A2b", "V8-A3", "V8-A4", "V8-A5", "V8-K"}
	if len(bands) != len(wantOrder) {
		t.Fatalf("A-band set cardinality drift: %v", names)
	}
	for i, prefix := range wantOrder {
		if !strings.HasPrefix(names[i], prefix) {
			t.Fatalf("A-band order drift at %d: %v", i, names)
		}
	}
	if !bands[0].WithinBand {
		t.Fatalf("V8-A2 drift: %+v", bands[0])
	}
	if bands[1].WithinBand {
		t.Fatalf("V8-A2b must fire when a saturated-patch actor sustains: %+v", bands[1])
	}
	// V8-A3 corrected per-patch semantics (review P2): in THIS fixture the
	// only filing is on the saturated patch (whole-run filing h5) while the
	// under-capacity patch's filing is -1, so A3 must be OUT — this is the
	// exact divergence case the reviewer flagged, where the whole-run reading
	// would have reported IN with wrong-patch evidence.
	if bands[2].WithinBand {
		t.Fatalf("V8-A3 must be OUT when only a saturated-patch actor filed: %+v", bands[2])
	}
	if !strings.Contains(bands[2].Observed, "first under-capacity-patch filing_hour=-1") {
		t.Fatalf("V8-A3 must report the under-capacity filing explicitly: %s", bands[2].Observed)
	}
	// The positive direction: give the under-capacity patch a filing inside
	// the bound and require A3 to flip IN, so both directions are bound.
	ucFiling := *br
	vmCopy := *br.V8Metrics
	vmCopy.Patches = append([]v8PatchReport(nil), br.V8Metrics.Patches...)
	for i := range vmCopy.Patches {
		if vmCopy.Patches[i].Patch == 1002 {
			vmCopy.Patches[i].FirstBirthFilingHour = 7
		}
	}
	ucFiling.V8Metrics = &vmCopy
	if got := v8BandsFor(o, &ucFiling); !got[2].WithinBand {
		t.Fatalf("V8-A3 must be IN when the under-capacity patch files <= h48: %+v", got[2])
	}
	if !bands[3].WithinBand {
		t.Fatalf("V8-A4 drift: %+v", bands[3])
	}
	if !bands[4].WithinBand {
		t.Fatalf("V8-A5 drift: %+v", bands[4])
	}
	if !bands[5].WithinBand {
		t.Fatalf("V8-K drift: %+v", bands[5])
	}
	if !strings.Contains(bands[1].Observed, "[1 2]") {
		t.Fatalf("A2b attribution must name the firing saturated-patch actors: %s", bands[1].Observed)
	}

	// A2b holds when only the under-capacity patch sustains, and V8-K's
	// windfall clause fails when the first surplus-bearing actor's share
	// collapses below the floor.
	checksB := v8DetectorFixture(map[int]func(h int) world.NaturalActorState{
		9: v8SurplusActor(0, 26),
		10: func(int) world.NaturalActorState {
			return v8FounderState(0, 3, 0, 0) // k≥1 for Σk, never bearing
		},
		11: func(int) world.NaturalActorState {
			return v8FounderState(0, 3, 0, 0)
		},
	}, v8CohortNewborns())
	brB := v8DetectorBranch(t, checksB, true)
	vmB, err := buildV8Metrics(o, brB, checksB, nil)
	if err != nil {
		t.Fatal(err)
	}
	brB.V8Metrics = &vmB
	bandsB := v8BandsFor(o, brB)
	if !bandsB[1].WithinBand {
		t.Fatalf("V8-A2b must hold when only the under-capacity patch sustains: %+v", bandsB[1])
	}
	if !bandsB[5].WithinBand {
		t.Fatalf("V8-K drift on the treatment-only fixture: %+v", bandsB[5])
	}
	checksC := v8DetectorFixture(map[int]func(h int) world.NaturalActorState{
		1: func(h int) world.NaturalActorState { // bearing only h0, no sustained run
			if h == 0 {
				return v8FounderState(0, 2, 1, 1)
			}
			return v8FounderState(0, 0, 0, 1)
		},
		9: v8SurplusActor(0, 26),
	}, v8CohortNewborns())
	brC := v8DetectorBranch(t, checksC, true)
	vmC, err := buildV8Metrics(o, brC, checksC, nil)
	if err != nil {
		t.Fatal(err)
	}
	brC.V8Metrics = &vmC
	bandsC := v8BandsFor(o, brC)
	if bandsC[5].WithinBand {
		t.Fatalf("V8-K must fail when the first surplus-bearing actor certifies a windfall: %+v", bandsC[5])
	}
	if !bandsC[1].WithinBand {
		t.Fatalf("V8-A2b must hold when the windfall actor never sustains (bearing one hour is not the indicator): %+v", bandsC[1])
	}

	// V8-C2 (births-off): the sustained holder's patch decides the band.
	brB.Births = false
	c2 := v8BandsFor(o, brB)
	if len(c2) != 1 || !strings.HasPrefix(c2[0].Name, "V8-C2") || !c2[0].WithinBand {
		t.Fatalf("V8-C2 drift (holder on the under-capacity patch): %+v", c2)
	}
	// A sustained holder seated on the saturated patch fails C2 (and fires
	// A2b, which the C2 branch does not read).
	checksD := v8DetectorFixture(map[int]func(h int) world.NaturalActorState{1: v8SurplusActor(0, 26)}, v8CohortNewborns())
	brD := v8DetectorBranch(t, checksD, false)
	vmD, err := buildV8Metrics(o, brD, checksD, nil)
	if err != nil {
		t.Fatal(err)
	}
	brD.V8Metrics = &vmD
	c2miss := v8BandsFor(o, brD)
	if c2miss[0].WithinBand {
		t.Fatalf("V8-C2 must fail when the holder sits on the saturated patch: %+v", c2miss[0])
	}

	// V8-B and V8-D band shapes.
	brBZero := *brB
	brBZero.Births = true
	brBZero.Totals.Births = 0 // a B cell records no births
	brBZero.Population.ExtinctionHour = 11
	o0 := options{patches: 2, founders: 12, r: 0}
	bBands := v8BandsFor(o0, &brBZero)
	if len(bBands) != 1 || !strings.HasPrefix(bBands[0].Name, "V8-B") || !bBands[0].WithinBand {
		t.Fatalf("V8-B drift: %+v", bBands)
	}
	brBZero.Population.ExtinctionHour = 13
	if v8BandsFor(o0, &brBZero)[0].WithinBand {
		t.Fatal("V8-B must fail above h12")
	}
	brDCell := *brB
	brDCell.CultivationDisabled = true
	brDCell.Events.BuildDenialEats = 42
	dBands := v8BandsFor(o, &brDCell)
	if len(dBands) != 1 || !strings.HasPrefix(dBands[0].Name, "V8-D") || !dBands[0].WithinBand {
		t.Fatalf("V8-D drift: %+v", dBands)
	}
	if !strings.Contains(dBands[0].Observed, "42") {
		t.Fatalf("V8-D must record the denial count verbatim: %s", dBands[0].Observed)
	}
}

// TestNaturalCLIV8ReportSchema runs a small A12 cell (P=2, f=12, births both)
// and pins the additive report schema: every v8_metrics section and field,
// the per-patch columns, the v8_bands entries, and the frozen band taking the
// primary V8-A2 place.
func TestNaturalCLIV8ReportSchema(t *testing.T) {
	r := runForTest(t, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 4, births: bothBranchesFlag, founders: 12, checkpointDir: newBundleDir(t)})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["founders"].(float64) != 12 {
		t.Fatalf("report founders field: %v", doc["founders"])
	}
	branches := doc["branches"].([]any)
	if len(branches) != 2 {
		t.Fatalf("expected both branches, got %d", len(branches))
	}
	for _, raw := range branches {
		br := raw.(map[string]any)
		m, ok := br["v8_metrics"].(map[string]any)
		if !ok {
			t.Fatal("declared-band branch carries no v8_metrics object")
		}
		for _, key := range []string{"definition", "founders_declared", "founders_resolved", "density_world", "patches",
			"sf_prime_by_hour_total", "sf_prime_run_total", "first_surplus_bearing_hour",
			"first_per_actor_sustained_hour", "first_per_actor_sustained_completion_hour",
			"actors_with_per_actor_sustained_runs", "surplus_bearing_actor_hours", "per_actor_surplus",
			"first_sustained_surplus_hour_union", "max_consecutive_surplus_hours_union", "time_to_first",
			"generation_count", "birth_cohort", "endowment_ledger_units", "endowment_cross_checked_in_g3",
			"paired_meals", "parent_deaths_within_24h_of_birth_payment", "non_universality"} {
			if _, ok := m[key]; !ok {
				t.Fatalf("missing v8_metrics field %q", key)
			}
		}
		for _, rawPatch := range m["patches"].([]any) {
			for _, key := range []string{"patch", "founders_seated", "slots", "saturated", "mouths_by_hour", "k_total_by_hour",
				"first_tenured_slot_hour", "first_banked_hour", "first_k2_hold_hour", "first_per_actor_sustained_hour",
				"first_birth_filing_hour", "first_birth_hour"} {
				if _, ok := rawPatch.(map[string]any)[key]; !ok {
					t.Fatalf("missing patch column field %q", key)
				}
			}
		}
		for _, rawActor := range m["per_actor_surplus"].([]any) {
			for _, key := range []string{"id", "home_patch", "generation", "max_consecutive_surplus_hours", "surplus_bearing_hours",
				"first_surplus_bearing_hour", "first_per_actor_sustained_hour", "alive_hours", "alive_hours_with_k_ge1"} {
				if _, ok := rawActor.(map[string]any)[key]; !ok {
					t.Fatalf("missing per_actor_surplus field %q", key)
				}
			}
		}
		ttf := m["time_to_first"].(map[string]any)
		for _, key := range []string{"definition", "first_tenured_slot_hour", "first_banked_hour", "first_k2_hold_hour",
			"first_per_actor_sustained_hour", "first_birth_filing_hour", "first_birth_hour"} {
			if _, ok := ttf[key]; !ok {
				t.Fatalf("missing v8 time_to_first field %q", key)
			}
		}
		nu := m["non_universality"].(map[string]any)
		for _, key := range []string{"definition", "deaths_to_horizon", "at_first_per_actor_sustained_hour",
			"alive_at_first_sustained_hour", "surplus_bearing_actors_at_first_sustained_hour", "surplus_bearing_strict_subset_of_alive"} {
			if _, ok := nu[key]; !ok {
				t.Fatalf("missing v8 non_universality field %q", key)
			}
		}
		bands, ok := br["v8_bands"].([]any)
		if !ok || len(bands) == 0 {
			t.Fatal("declared-band branch carries no v8_bands")
		}
		for _, b := range bands {
			for _, key := range []string{"name", "criterion", "observed", "within_band"} {
				if _, ok := b.(map[string]any)[key]; !ok {
					t.Fatalf("missing v8_bands entry field %q", key)
				}
			}
		}
		if br["frozen_band"].(map[string]any)["name"] != bands[0].(map[string]any)["name"] {
			t.Fatal("frozen_band is not the primary predeclared v8 band")
		}
		// The births-enabled branch reports the declared A-band set (all
		// marked not evaluable below h168 on the truncated cell); the
		// births-disabled branch is the C12-shaped cell with V8-C2 alone.
		names := make([]string, 0, 6)
		for _, b := range bands {
			names = append(names, b.(map[string]any)["name"].(string))
		}
		if br["births"].(bool) {
			wantOrder := []string{"V8-A2:", "V8-A2b:", "V8-A3:", "V8-A4:", "V8-A5:", "V8-K:"}
			if len(names) != len(wantOrder) {
				t.Fatalf("truncated A-band set drift: %v", names)
			}
			for i, prefix := range wantOrder {
				if !strings.HasPrefix(names[i], prefix) {
					t.Fatalf("truncated A-band order drift at %d: %v", i, names)
				}
			}
		} else if len(names) != 1 || !strings.HasPrefix(names[0], "V8-C2:") {
			t.Fatalf("births-disabled band set drift: %v", names)
		}
	}
}

// TestNaturalCLIV7FoundersCombination pins the store-falsifier arm wiring:
// natural/7b at a declared band carries BOTH the v7 sections (the mechanism
// ablation) and the v8 sections (the founding-band density evidence) in one
// report, with the axis orthogonal to the ref/mechanism identity.
func TestNaturalCLIV7FoundersCombination(t *testing.T) {
	r := runForTest(t, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, policy: policy7bName, mechanism: mechanismV6Name, founders: 12, checkpointDir: newBundleDir(t)})
	if r.Revision != "natural mechanism-v6/format-v6/schema-v6/rule-v6/projection-v6/policy natural/7b" {
		t.Fatalf("revision drift: %q", r.Revision)
	}
	for _, br := range r.Branches {
		if br.V7Metrics == nil || br.V8Metrics == nil || len(br.V7Bands) == 0 || len(br.V8Bands) == 0 {
			t.Fatalf("store-falsifier wiring lost a section: v7=%t v8=%t", br.V7Metrics != nil, br.V8Metrics != nil)
		}
		if br.V8Metrics.FoundersDeclared != 12 || br.V8Metrics.DensityWorld != 0.75 {
			t.Fatalf("v8 axis drift under the v7 arm: %+v", br.V8Metrics)
		}
	}
}
