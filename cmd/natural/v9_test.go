package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// v9Scripted builds a hand-made branch fixture for the v9 detectors: the
// roster is founder 1 (worker, patch 1002) and founder 2 (employer, patch
// 1002) unless stated otherwise, and the hourly projections carry the given
// per-hour readings.
type v9Reading struct {
	hour        int
	workerK     int64
	workerStock int64
	workerCult  int64
	workerWages int64
	workerCash  int64 // wages paid by the worker (0: it only earns)
	employK     int64
	employStock int64
	employPaid  int64
	workerE     int64 // worker pulse-boundary energy (default 8)
}

func v9Actor(id int, patchEntity int, k, stock, cult, wagesEarned, wagesPaid, energy int64) world.NaturalActorState {
	return world.NaturalActorState{
		Body:     world.NaturalBodyState{Energy: energy},
		Bag:      world.NaturalBagState{Source: sim.EntityID(patchEntity)},
		Worksite: world.NaturalWorksiteState{Capital: k},
		Granary:  world.NaturalGranaryState{Stock: stock, CultivatedTotal: cult, WagesEarned: wagesEarned, WagesPaid: wagesPaid},
		Genome:   world.NaturalGenomeState{DiedHour: world.NaturalNeverHour},
	}
}

func v9Checks(readings []v9Reading) []world.NaturalCheckpoint {
	patches := 2
	out := make([]world.NaturalCheckpoint, len(readings))
	for i, r := range readings {
		c := world.NaturalCheckpoint{Hour: r.hour, Slots: make([][]world.NaturalSlotState, patches)}
		c.Founders = []world.NaturalActorState{
			v9Actor(1, world.NaturalPatchIDBase+1, r.workerK, r.workerStock, r.workerCult, r.workerWages, r.workerCash, r.workerE),
			v9Actor(2, world.NaturalPatchIDBase+1, r.employK, r.employStock, 0, 0, r.employPaid, 8),
		}
		c.Balance = world.NaturalBalance{Capital: r.workerK + r.employK,
			GranaryStock: r.workerStock + r.employStock, CultivatedTotal: r.workerCult,
			WagesPaid: r.employPaid + r.workerCash, WagesEarned: r.workerWages}
		c.Metrics.Alive = 2
		out[i] = c
	}
	return out
}

func v9Populate(br *branchReport, checks []world.NaturalCheckpoint) {
	br.FirstHour = checks[0].Hour
	br.CompletedHour = checks[len(checks)-1].Hour
	br.Population.AliveByHour = make([]int, len(checks))
	br.Population.BirthsByHour = make([]int64, len(checks))
	for i := range checks {
		br.Population.AliveByHour[i] = checks[i].Metrics.Alive
	}
	br.ConservationVerified = true
}

func v9PhaseTime(hour int) sim.SimTime {
	return sim.SimTime(hour) * sim.SimTime(world.NaturalHour)
}

func v9IntDelta(entity sim.EntityID, comp sim.ComponentTypeID, field sim.FieldID, before, after int64) component.FieldDelta {
	return component.FieldDelta{Entity: entity, Component: comp, Field: field, Before: sim.IntegerValue(before), After: sim.IntegerValue(after)}
}

func v9RefDelta(entity sim.EntityID, comp sim.ComponentTypeID, field sim.FieldID, before, after sim.EntityID) component.FieldDelta {
	beforeV, err := sim.EntityRefValue(before)
	if err != nil {
		panic(err)
	}
	afterV, err := sim.EntityRefValue(after)
	if err != nil {
		panic(err)
	}
	return component.FieldDelta{Entity: entity, Component: comp, Field: field, Before: beforeV, After: afterV}
}

// v9OfferEvent builds an accepted rule-516 event: worker files on its own
// offer row addressed to the employer.
func v9OfferEvent(id sim.EventID, hour int, worker, employer sim.EntityID) kernel.Event {
	return kernel.Event{
		ID: id, Time: v9PhaseTime(hour), Key: "offer", Cause: kernel.Cause{Actor: worker},
		Rule: world.NaturalLabourOfferRule,
		Deltas: []component.FieldDelta{
			v9IntDelta(worker, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferHourField, 0, int64(hour)),
			v9RefDelta(worker, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferAddresseeField, worker, employer),
			v9IntDelta(worker, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferStatusField, 0, int64(world.NaturalRequestPending)),
			v9IntDelta(worker, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferLastOfferHourField, 0, int64(hour)),
		},
	}
}

// v9AcceptEvent builds an accepted rule-517 accept event.
func v9AcceptEvent(id sim.EventID, hour int, employer, worker sim.EntityID) kernel.Event {
	return kernel.Event{
		ID: id, Time: v9PhaseTime(hour), Key: "accept", Cause: kernel.Cause{Actor: employer},
		Rule: world.NaturalLabourDecisionRule,
		Deltas: []component.FieldDelta{
			v9IntDelta(worker, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferStatusField, int64(world.NaturalRequestPending), int64(world.NaturalRequestAccepted)),
			v9IntDelta(worker, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferLastReplyHourField, 0, int64(hour)),
			v9RefDelta(employer, world.NaturalLabourBookTypeID, world.NaturalLabourBookEngagedWorkerField, employer, worker),
			v9IntDelta(employer, world.NaturalLabourBookTypeID, world.NaturalLabourBookEngagedHourField, 0, int64(hour)),
			v9IntDelta(employer, world.NaturalLabourBookTypeID, world.NaturalLabourBookLastDecisionHourField, 0, int64(hour)),
		},
	}
}

// v9SettleEvent builds an accepted rule-518 settlement event with the frozen
// enumerated patch set: worker bag −1, worker stock +1, worker wages-earned
// +1, employer stock −1, employer wages-paid +1, employer invested +1 and
// wip +1.
func v9SettleEvent(id sim.EventID, hour int, worker, employer sim.EntityID, wStock, wWages, eStock, ePaid, eInvested, eWip int64) kernel.Event {
	return kernel.Event{
		ID: id, Time: v9PhaseTime(hour), Key: fmt.Sprintf("settle-%d", id), Cause: kernel.Cause{Actor: worker},
		Rule: world.NaturalLabourSettlementRule,
		Deltas: []component.FieldDelta{
			v9IntDelta(worker, world.NaturalBagTypeID, world.NaturalBagUnitsField, 1, 0),
			v9IntDelta(worker, world.NaturalGranaryTypeID, world.NaturalGranaryStockField, wStock, wStock+1),
			v9IntDelta(worker, world.NaturalGranaryTypeID, world.NaturalGranaryWagesEarnedField, wWages, wWages+1),
			v9IntDelta(employer, world.NaturalGranaryTypeID, world.NaturalGranaryStockField, eStock, eStock-1),
			v9IntDelta(employer, world.NaturalGranaryTypeID, world.NaturalGranaryWagesPaidField, ePaid, ePaid+1),
			v9IntDelta(employer, world.NaturalWorksiteTypeID, world.NaturalWorksiteInvestedUnitsField, eInvested, eInvested+1),
			v9IntDelta(employer, world.NaturalWorksiteTypeID, world.NaturalWorksiteWipField, eWip, eWip+1),
		},
	}
}

func v9Events(events ...kernel.Event) map[sim.EventID]kernel.Event {
	byID := make(map[sim.EventID]kernel.Event, len(events))
	for _, ev := range events {
		byID[ev.ID] = ev
	}
	return byID
}

func v9Attempt(actor sim.EntityID, hour int, kind, key string, id sim.EventID) world.NaturalAttempt {
	return world.NaturalAttempt{Actor: actor, Time: v9PhaseTime(hour), Kind: kind, Key: key, Accepted: true, EventID: id}
}

// v9Options is the declared cell shape for the scripted detectors: an A12
// designated-trade shape (not the saturated control, whose T6 clause forces
// zero settlements).
func v9Options() options {
	return options{patches: 2, r: 8, seed: 0, workers: 1, hours: 8, births: bothBranchesFlag,
		policy: policy9Name, mechanism: mechanismV7Name, founders: 12}
}

// v9Build runs the scripted fixture through the real pipeline: the journal
// classification fills the attempt counters, then the v9 metric builder and
// the band evaluator read the same evidence.
func v9Build(t *testing.T, o options, checks []world.NaturalCheckpoint, journal []world.NaturalBatch, byID map[sim.EventID]kernel.Event) (branchReport, v9MetricsReport) {
	t.Helper()
	br := branchReport{}
	v9Populate(&br, checks)
	if err := classifyJournal(&br, journal, byID); err != nil {
		t.Fatalf("classifyJournal: %v", err)
	}
	m, err := buildV9Metrics(o, &br, checks, journal, byID)
	if err != nil {
		t.Fatalf("buildV9Metrics: %v", err)
	}
	br.V9Metrics = &m
	br.V9Bands = v9BandsFor(o, &br)
	return br, m
}

// TestNaturalCLIV9FlagRefusals pins the ticket-20 flag surface: natural/9 is
// refused under the v5 zero value and under v6 before any publication (no
// output file, no checkpoint directory); unknown policy and mechanism values
// keep the frozen refusal set; natural/7 is legal under v7; stray arguments
// keep exit status 2 with the new flags present; no-clobber holds under
// natural/9.
func TestNaturalCLIV9FlagRefusals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		policy, mechanism string
	}{
		{policy9Name, ""}, // zero mechanism is exactly v5
		{policy9Name, mechanismV5Name},
		{policy9Name, mechanismV6Name}, // natural/9 needs the rules 516-519 world
		{"natural/8", mechanismV7Name}, // unknown ref
		{policy9Name, "v8"},            // unknown mechanism
		{policy9Name, "V7"},            // case-exact spellings only
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
	// natural/7 under v7 is the legal control rung; no-clobber holds under
	// natural/9.
	base := t.TempDir()
	out := filepath.Join(base, "report.json")
	if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 2, births: disabledBranchFlag, policy: policy7Name, mechanism: mechanismV7Name, checkpointDir: newBundleDir(t)}); err != nil {
		t.Fatalf("natural/7 refused under v7: %v", err)
	}
	if err := os.WriteFile(out, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, options{patches: 2, r: 8, seed: 0, workers: 1, hours: 1, births: disabledBranchFlag, policy: policy9Name, mechanism: mechanismV7Name, out: out, checkpointDir: newBundleDir(t)}); err == nil {
		t.Fatal("existing output accepted under natural/9")
	}
	// CLI level: stray arguments with the new flags present keep the
	// documented exit-2 contract, and an illegal policy/mechanism pair exits
	// nonzero before publication.
	binary := buildNaturalCLI(t)
	cmd := exec.CommandContext(ctx, binary, "-policy="+policy9Name, "-hours=1", "-out="+filepath.Join(t.TempDir(), "x.json"))
	if err := cmd.Run(); err == nil {
		t.Fatal("CLI accepted -policy=natural/9 without -mechanism=v7")
	}
	for _, extra := range []string{"pre", "post"} {
		args := []string{"-policy=" + policy9Name, "-mechanism=" + mechanismV7Name, "-hours=1"}
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
			t.Fatalf("stray args with natural/9: exit %v (stderr %s)", err, stderr.String())
		}
		if _, err := os.Stat(out2); !os.IsNotExist(err) {
			t.Fatalf("output published despite stray args: %v", err)
		}
	}
}

// TestNaturalCLIV9DefaultReportByteCompatibility pins the ticket-20
// default-preservation contract: every pre-v9 report shape (the v5 default,
// the v6 arms, the v6 provision arms and the v7 zero-use control) marshals
// without any of the additive v9 keys, and the published neutral root
// bundles keep their pre-change digests byte-for-byte (captured at 9b39f49
// before the L4a edits, deterministic by the replay discipline).
func TestNaturalCLIV9DefaultReportByteCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, policy, mechanism, wantDigest string
	}{
		{"default", "", "", "97a14338dba9b92c68204571472a1c6ef168be35cea52cbb673d16ecfe8faa94"},
		{"v6-n7b", policy7bName, mechanismV6Name, "acfe3c71edfcc26baf9dc01b6412e84cbbad794ef5f95c9ff0fbc71bda60620e"},
		{"v6-n6", policy6Name, mechanismV6Name, "729ffcedd113908750123c76874c878ef017b4061d3c41b0fc5eadef510c09d0"},
	} {
		r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 3, births: bothBranchesFlag, policy: tc.policy, mechanism: tc.mechanism, checkpointDir: newBundleDir(t)})
		if r.Root.Digest != tc.wantDigest {
			t.Fatalf("%s root digest drifted from the pre-change pin: got %s want %s", tc.name, r.Root.Digest, tc.wantDigest)
		}
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{`"v9_metrics"`, `"v9_bands"`, `"v9_verdict"`, `"labour_offers"`, `"labour_decisions"`,
			`"labour_accepts"`, `"labour_refusals"`, `"labour_settlements"`, `"labour_voids"`, `"labour_void_causes"`,
			`"wages_paid"`, `"wages_earned"`, `"settlement_rows"`, `"counterparties"`, `"coexistence"`,
			`"sf_double_prime"`, `"wage_banked_firsts"`, `"wage_meal_draws"`, `"employer_k_by_hour"`, `"time_to_k8_hour"`} {
			if bytes.Contains(encoded, []byte(key)) {
				t.Fatalf("%s report carries the additive v9 key %s", tc.name, key)
			}
		}
	}
}

// TestNaturalCLIV9ReportSchema runs a small natural/9 cell on world v7 and
// pins the additive report schema: the identity strings, every v9_metrics
// section, the six predeclared bands in declared order with frozen_band = T1,
// the run-level v9_verdict with the frozen labels, and the labour attempt
// counters, all parseable from JSON.
func TestNaturalCLIV9ReportSchema(t *testing.T) {
	// Fourteen hours: the first settlements land around h8 on this cell, so
	// the settlement-derived keys are present in event_counts.
	r := runForTest(t, options{patches: world.NaturalDefaultPatches, r: 8, seed: 0, workers: 1, hours: 14, births: enabledBranchFlag, policy: policy9Name, mechanism: mechanismV7Name, checkpointDir: newBundleDir(t)})
	if r.Policy != policy9Name || r.Mechanism != mechanismV7Name {
		t.Fatalf("identity strings drift: %q %q", r.Policy, r.Mechanism)
	}
	if r.Revision != "natural mechanism-v7/format-v7/schema-v7/rule-v7/projection-v7/policy natural/9" {
		t.Fatalf("v7 revision drift: %q", r.Revision)
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	verdict, ok := doc["v9_verdict"].(map[string]any)
	if !ok {
		t.Fatal("natural/9 report carries no v9_verdict")
	}
	switch verdict["verdict"] {
	case v9VerdictSupported, v9VerdictNullTrade, v9VerdictFailed, v9VerdictTradeInert:
	default:
		t.Fatalf("verdict outside the frozen label set: %v", verdict["verdict"])
	}
	for _, key := range []string{"t1_trade_occurrence_in", "t2_protocol_discipline_in", "t3_conservation_in", "t6_non_distortion_in", "t4_worker_channel_observed", "t5_employer_channel_observed", "settlements", "first_settlement_hour", "rationale"} {
		if _, ok := verdict[key]; !ok {
			t.Fatalf("missing v9_verdict field %q", key)
		}
	}
	branches := doc["branches"].([]any)
	if len(branches) != 1 {
		t.Fatalf("expected the enabled branch, got %d", len(branches))
	}
	for _, raw := range branches {
		br := raw.(map[string]any)
		m, ok := br["v9_metrics"].(map[string]any)
		if !ok {
			t.Fatal("natural/9 branch report carries no v9_metrics object")
		}
		for _, key := range []string{"definition", "link_evidence_available", "offers", "decisions", "accepts", "refusals",
			"expiries", "voids", "settlements", "first_settlement_hour", "settlement_rows", "latency_definition",
			"latency_min_hours", "latency_max_hours", "latency_total_hours", "chain_discipline_definition",
			"chain_violations", "accept_without_offer_violations", "field_set_violations",
			"settlement_food_delta_violations", "per_actor_trade", "per_patch_trade", "wage_ledger",
			"wage_banked_firsts", "wage_banked_first_definition", "wage_meal_draws", "wage_meal_definition",
			"employer_k_by_hour", "employer_channel_definition", "time_to_k8_hour", "counterparty_definition",
			"coexistence", "sf_prime_by_hour_total", "sf_double_prime_by_hour_total", "sf_prime_run_total",
			"sf_double_prime_run_total", "sf_definition"} {
			if _, ok := m[key]; !ok {
				t.Fatalf("missing v9_metrics field %q", key)
			}
		}
		wl := m["wage_ledger"].(map[string]any)
		for _, key := range []string{"definition", "wages_paid_total", "wages_earned_total", "per_patch_wages_paid_final",
			"per_patch_wages_earned_final", "patch_hour_checks", "patch_hour_imbalance_count"} {
			if _, ok := wl[key]; !ok {
				t.Fatalf("missing wage_ledger field %q", key)
			}
		}
		for _, row := range m["per_actor_trade"].([]any) {
			for _, key := range []string{"id", "home_patch", "generation", "offers", "accepts", "refusals", "expiries", "voids", "settlements", "wages_paid", "wages_earned"} {
				if _, ok := row.(map[string]any)[key]; !ok {
					t.Fatalf("missing per_actor_trade field %q", key)
				}
			}
		}
		events := br["event_counts"].(map[string]any)
		for _, key := range []string{"labour_offers", "labour_decisions", "labour_settlements"} {
			if _, ok := events[key]; !ok {
				t.Fatalf("missing event_counts key %q", key)
			}
		}
		bands, ok := br["v9_bands"].([]any)
		if !ok || len(bands) != 6 {
			t.Fatalf("natural/9 branch carries %d v9_bands, want the six predeclared bands", len(bands))
		}
		for _, b := range bands {
			for _, key := range []string{"name", "criterion", "observed", "within_band"} {
				if _, ok := b.(map[string]any)[key]; !ok {
					t.Fatalf("missing v9_bands entry field %q", key)
				}
			}
		}
		names := make([]string, 0, 6)
		for _, b := range bands {
			names = append(names, b.(map[string]any)["name"].(string))
		}
		for i, prefix := range []string{"V9-T1", "V9-T2", "V9-T3", "V9-T4", "V9-T5", "V9-T6"} {
			if !strings.HasPrefix(names[i], prefix) {
				t.Fatalf("v9 band order drift at %d: %v", i, names)
			}
		}
		if br["frozen_band"].(map[string]any)["name"] != bands[0].(map[string]any)["name"] {
			t.Fatal("frozen_band is not the primary predeclared V9-T1 band")
		}
	}
}

// TestNaturalCLIV9BandDetectors pins the V9 band detectors on scripted
// journals: the clean chain reports T1/T2/T3 IN with a 1-hour latency and a
// zero settlement food delta; the T1 null (zero settlements) holds with the
// NULL-TRADE verdict; a settlement without a matched same-hour accepted
// offer, an accept whose offer is addressed elsewhere, an undeclared 516
// field, and a nonzero settlement food delta each break their gate verbatim;
// the patch-1001 clause and the saturated-control clause bind as frozen.
func TestNaturalCLIV9BandDetectors(t *testing.T) {
	worker, employer := sim.EntityID(1), sim.EntityID(2)
	// Hourly readings: the worker holds no stock until the settlement lands
	// at h6 (so its first granary hour is the wage), the employer pays from
	// stock 4 (the derived floor reading, descriptive).
	readings := []v9Reading{
		{hour: 4, workerE: 3},
		{hour: 5, workerE: 3},
		{hour: 6, workerStock: 1, workerWages: 1, employStock: 3, employPaid: 1, workerE: 4},
		{hour: 7, workerStock: 1, workerWages: 1, employStock: 3, employPaid: 1, workerE: 8},
	}
	checks := v9Checks(readings)
	events := v9Events(
		v9OfferEvent(1, 5, worker, employer),
		v9AcceptEvent(2, 5, employer, worker),
		v9SettleEvent(3, 6, worker, employer, 0, 0, 4, 0, 0, 0),
	)
	journal := []world.NaturalBatch{
		{Time: v9PhaseTime(5), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 5, "labour-offer", "offer", 1),
			v9Attempt(employer, 5, "labour-decision", "accept", 2),
		}},
		{Time: v9PhaseTime(6), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 6, "labour-settlement", "settle-3", 3),
			{Actor: worker, Time: v9PhaseTime(6), Kind: "labour-void", Refused: "settlement-precedence"},
		}},
	}
	o := v9Options()
	br, m := v9Build(t, o, checks, journal, events)
	if m.Settlements != 1 || m.FirstSettlementHour != 6 || m.Accepts != 1 || m.Refusals != 0 {
		t.Fatalf("clean-chain counters drift: %+v", m)
	}
	if m.ChainViolations != 0 || m.AcceptWithoutOfferViolations != 0 || m.FieldSetViolations != 0 || m.SettlementFoodDeltaViolations != 0 {
		t.Fatalf("clean-chain discipline drift: chain=%d accept_wo=%d field=%d food=%d", m.ChainViolations, m.AcceptWithoutOfferViolations, m.FieldSetViolations, m.SettlementFoodDeltaViolations)
	}
	if len(m.SettlementRows) != 1 || m.SettlementRows[0].LatencyHours != 1 || m.SettlementRows[0].FoodDelta != 0 || m.SettlementRows[0].Patch != world.NaturalPatchIDBase+1 {
		t.Fatalf("settlement row drift: %+v", m.SettlementRows)
	}
	if m.Voids != 1 || m.VoidCauses["settlement-precedence"] != 1 {
		t.Fatalf("typed void evidence drift: %d %v", m.Voids, m.VoidCauses)
	}
	if m.WageBankedFirsts != 1 || len(m.WageBankedFirstActors) != 1 || m.WageBankedFirstActors[0] != 1 {
		t.Fatalf("wage-banked first drift: %d %v", m.WageBankedFirsts, m.WageBankedFirstActors)
	}
	if m.WageLedger.WagesPaidTotal != 1 || m.WageLedger.WagesEarnedTotal != 1 || m.WageLedger.PatchHourImbalanceCount != 0 {
		t.Fatalf("wage ledger drift: %+v", m.WageLedger)
	}
	bands := map[string]bandReport{}
	for _, b := range br.V9Bands {
		bands[strings.SplitN(b.Name, ":", 2)[0]] = b
	}
	if !bands["V9-T1"].WithinBand || !bands["V9-T2"].WithinBand || !bands["V9-T3"].WithinBand {
		t.Fatalf("clean chain must hold T1/T2/T3: %+v", br.V9Bands)
	}
	if !bands["V9-T4"].WithinBand {
		t.Fatalf("the wage-banked first is T4 channel evidence: %+v", bands["V9-T4"])
	}
	// T6's death clause: a settlement-hosting branch needs a death before
	// h168; the fixture records none, so T6 reports OUT verbatim.
	if bands["V9-T6"].WithinBand {
		t.Fatal("T6 death clause must fail without a recorded death")
	}
	// The verdict over the same single branch: gates T1/T2/T3 IN, T6 OUT ⇒
	// FAILED.
	v := v9VerdictFor(report{Branches: []branchReport{br}})
	if v.Verdict != v9VerdictFailed {
		t.Fatalf("verdict with T6 OUT must be FAILED: %+v", v)
	}

	// T1 null: the same cell with an empty labour journal reports zero
	// settlements with every discipline gate IN, and the verdict is the
	// frozen NULL-TRADE.
	emptyBr, emptyM := v9Build(t, o, v9Checks(readings[:1]), nil, map[sim.EventID]kernel.Event{})
	if emptyM.Settlements != 0 || emptyM.FirstSettlementHour != -1 {
		t.Fatalf("null-trade counters drift: %+v", emptyM)
	}
	nullBands := map[string]bandReport{}
	for _, b := range emptyBr.V9Bands {
		nullBands[strings.SplitN(b.Name, ":", 2)[0]] = b
	}
	if nullBands["V9-T1"].WithinBand || !nullBands["V9-T2"].WithinBand || !nullBands["V9-T3"].WithinBand {
		t.Fatalf("null-trade gates drift: %+v", emptyBr.V9Bands)
	}
	v = v9VerdictFor(report{Branches: []branchReport{emptyBr}})
	if v.Verdict != v9VerdictNullTrade {
		t.Fatalf("zero settlements with gates IN must be NULL-TRADE: %+v", v)
	}

	// A settlement without a matched same-hour accepted offer: the accept is
	// missing, so the chain check fails T2 verbatim.
	chainlessEvents := v9Events(
		v9OfferEvent(1, 5, worker, employer),
		v9SettleEvent(3, 6, worker, employer, 0, 0, 4, 0, 0, 0),
	)
	chainlessJournal := []world.NaturalBatch{
		{Time: v9PhaseTime(5), Attempts: []world.NaturalAttempt{v9Attempt(worker, 5, "labour-offer", "offer", 1)}},
		{Time: v9PhaseTime(6), Attempts: []world.NaturalAttempt{v9Attempt(worker, 6, "labour-settlement", "settle-3", 3)}},
	}
	_, chainlessM := v9Build(t, o, checks, chainlessJournal, chainlessEvents)
	if chainlessM.ChainViolations == 0 || chainlessM.ChainViolationNote == "" {
		t.Fatalf("settlement without a matched accept must break the chain check: %+v", chainlessM)
	}

	// An accept whose worker's filed offer is addressed to another employer
	// breaks the accept clause verbatim.
	stranger := sim.EntityID(3)
	misaddressedEvents := v9Events(
		v9OfferEvent(1, 5, worker, stranger),
		v9AcceptEvent(2, 5, employer, worker),
	)
	misaddressedJournal := []world.NaturalBatch{
		{Time: v9PhaseTime(5), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 5, "labour-offer", "offer", 1),
			v9Attempt(employer, 5, "labour-decision", "accept", 2),
		}},
	}
	_, misM := v9Build(t, o, checks, misaddressedJournal, misaddressedEvents)
	if misM.AcceptWithoutOfferViolations == 0 || misM.AcceptWithoutOfferNote == "" {
		t.Fatalf("misaddressed accept must be a violation: %+v", misM)
	}

	// An undeclared 516 field breaks the privacy-pinned field set verbatim.
	leaky := v9OfferEvent(1, 5, worker, employer)
	leaky.Deltas = append(leaky.Deltas, v9IntDelta(worker, world.NaturalLabourOfferTypeID, sim.FieldID(9), 0, 1))
	leakyEvents := v9Events(leaky, v9AcceptEvent(2, 5, employer, worker))
	leakyJournal := []world.NaturalBatch{
		{Time: v9PhaseTime(5), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 5, "labour-offer", "offer", 1),
			v9Attempt(employer, 5, "labour-decision", "accept", 2),
		}},
	}
	_, leakyM := v9Build(t, o, checks, leakyJournal, leakyEvents)
	if leakyM.FieldSetViolations == 0 || leakyM.FieldSetViolationNote == "" {
		t.Fatalf("undeclared offer field must be a field-set violation: %+v", leakyM)
	}

	// A settlement whose food deltas do not aggregate to zero (a minted
	// worker stock unit) breaks T3 verbatim.
	minted := v9SettleEvent(3, 6, worker, employer, 0, 0, 4, 0, 0, 0)
	minted.Deltas[1] = v9IntDelta(worker, world.NaturalGranaryTypeID, world.NaturalGranaryStockField, 0, 2)
	mintedEvents := v9Events(
		v9OfferEvent(1, 5, worker, employer),
		v9AcceptEvent(2, 5, employer, worker),
		minted,
	)
	mintedJournal := []world.NaturalBatch{
		{Time: v9PhaseTime(5), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 5, "labour-offer", "offer", 1),
			v9Attempt(employer, 5, "labour-decision", "accept", 2),
		}},
		{Time: v9PhaseTime(6), Attempts: []world.NaturalAttempt{v9Attempt(worker, 6, "labour-settlement", "settle-3", 3)}},
	}
	_, mintedM := v9Build(t, o, checks, mintedJournal, mintedEvents)
	if mintedM.SettlementFoodDeltaViolations == 0 || mintedM.SettlementFoodDeltaNote == "" {
		t.Fatalf("minted stock must break the food-delta check: %+v", mintedM)
	}

	// The saturated-control clause: at the full founding band T6 forces zero
	// settlements even with the clean chain.
	saturated := v9Options()
	saturated.founders = 0 // resolves to 8·P = the saturated control
	satBr, _ := v9Build(t, saturated, checks, journal, events)
	satBands := map[string]bandReport{}
	for _, b := range satBr.V9Bands {
		satBands[strings.SplitN(b.Name, ":", 2)[0]] = b
	}
	if satBands["V9-T6"].WithinBand {
		t.Fatalf("the saturated control must force zero settlements: %+v", satBands["V9-T6"])
	}

	// The patch-1001 clause: a settlement whose worker sits on patch 1001
	// breaks T6's patch clause verbatim.
	patch1001 := v9Checks(readings)
	for i := range patch1001 {
		patch1001[i].Founders[0].Bag.Source = sim.EntityID(world.NaturalPatchIDBase)
		patch1001[i].Founders[1].Bag.Source = sim.EntityID(world.NaturalPatchIDBase)
	}
	_, p1M := v9Build(t, o, patch1001, journal, events)
	on1001 := false
	for _, row := range p1M.SettlementRows {
		if row.Patch == world.NaturalPatchIDBase {
			on1001 = true
		}
	}
	if !on1001 {
		t.Fatalf("fixture must seat the worker on patch 1001: %+v", p1M.SettlementRows)
	}
}

// TestNaturalCLIV9WageLedgerGenerationTable pins the wage-ledger and
// counterparty-generation computations on a hand-built fixture: per-patch and
// per-actor wage ledgers aggregate from the frozen granary fields 8/9, the
// per-patch-hour Σwages-paid = Σwages-earned equality flags a scripted
// imbalance, the counterparty table separates founder from newborn
// counterparties through the derived parent map, and SF″ = SF′ − wagesPaid +
// wagesEarned moves by exactly the wage window beside the frozen SF′.
func TestNaturalCLIV9WageLedgerGenerationTable(t *testing.T) {
	worker, employer := sim.EntityID(1), sim.EntityID(2)
	newbornID := sim.EntityID(17)
	// The newborn (parents 1, 2) is the second settlement's worker: the table
	// must distinguish the founder→founder shift from the founder→newborn
	// transfer.
	checks := v9Checks([]v9Reading{
		{hour: 4, workerE: 3},
		{hour: 5, workerE: 3},
		{hour: 6, workerStock: 1, workerWages: 1, employStock: 2, employPaid: 1, workerE: 8},
		{hour: 7, workerStock: 1, workerWages: 1, employStock: 0, employPaid: 2, workerE: 8},
	})
	nb := v9Actor(17, world.NaturalPatchIDBase+1, 0, 1, 0, 1, 0, 8)
	nb.Genome.ParentA, nb.Genome.ParentB = worker, employer
	nb.Genome.BirthHour = 4
	nbBefore := nb
	nbBefore.Granary.Stock, nbBefore.Granary.WagesEarned = 0, 0
	checks[2].Newborns = []world.NaturalNewbornState{{ID: newbornID, State: nbBefore}}
	checks[3].Newborns = []world.NaturalNewbornState{{ID: newbornID, State: nb}}
	checks[2].Balance.GranaryStock = 4
	checks[3].Balance.GranaryStock = 2
	checks[2].Balance.WagesPaid, checks[2].Balance.WagesEarned = 1, 1
	checks[3].Balance.WagesPaid, checks[3].Balance.WagesEarned = 2, 2
	checks[3].Founders[1] = v9Actor(2, world.NaturalPatchIDBase+1, 0, 0, 0, 0, 2, 8)
	// Scripted imbalance at h7: the worker's earned ledger advances a second
	// unit with no payer's paid ledger matching on the patch.
	bad := append([]world.NaturalCheckpoint(nil), checks...)
	bad[3].Founders = append([]world.NaturalActorState(nil), checks[3].Founders...) // the Founders slices share backing storage
	bad[3].Founders[0] = v9Actor(1, world.NaturalPatchIDBase+1, 0, 1, 0, 2, 0, 8)

	events := v9Events(
		v9OfferEvent(1, 5, worker, employer),
		v9AcceptEvent(2, 5, employer, worker),
		v9SettleEvent(3, 6, worker, employer, 0, 0, 3, 0, 0, 0),
		kernel.Event{ID: 4, Time: v9PhaseTime(6), Key: "offer2", Cause: kernel.Cause{Actor: newbornID}, Rule: world.NaturalLabourOfferRule,
			Deltas: []component.FieldDelta{
				v9IntDelta(newbornID, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferHourField, 0, 6),
				v9RefDelta(newbornID, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferAddresseeField, newbornID, employer),
				v9IntDelta(newbornID, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferStatusField, 0, int64(world.NaturalRequestPending)),
				v9IntDelta(newbornID, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferLastOfferHourField, 0, 6),
			}},
		kernel.Event{ID: 5, Time: v9PhaseTime(6), Key: "accept2", Cause: kernel.Cause{Actor: employer}, Rule: world.NaturalLabourDecisionRule,
			Deltas: []component.FieldDelta{
				v9IntDelta(newbornID, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferStatusField, int64(world.NaturalRequestPending), int64(world.NaturalRequestAccepted)),
				v9IntDelta(newbornID, world.NaturalLabourOfferTypeID, world.NaturalLabourOfferLastReplyHourField, 0, 6),
				v9RefDelta(employer, world.NaturalLabourBookTypeID, world.NaturalLabourBookEngagedWorkerField, employer, newbornID),
				v9IntDelta(employer, world.NaturalLabourBookTypeID, world.NaturalLabourBookEngagedHourField, 0, 6),
				v9IntDelta(employer, world.NaturalLabourBookTypeID, world.NaturalLabourBookLastDecisionHourField, 0, 6),
			}},
		v9SettleEvent(6, 7, newbornID, employer, 0, 0, 1, 1, 1, 0),
	)
	journal := []world.NaturalBatch{
		{Time: v9PhaseTime(5), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 5, "labour-offer", "offer", 1),
			v9Attempt(employer, 5, "labour-decision", "accept", 2),
		}},
		{Time: v9PhaseTime(6), Attempts: []world.NaturalAttempt{
			v9Attempt(worker, 6, "labour-settlement", "settle-3", 3),
			v9Attempt(newbornID, 6, "labour-offer", "offer2", 4),
			v9Attempt(employer, 6, "labour-decision", "accept2", 5),
			{Actor: worker, Time: v9PhaseTime(6), Kind: "eat-stored"},
		}},
		{Time: v9PhaseTime(7), Attempts: []world.NaturalAttempt{
			v9Attempt(newbornID, 7, "labour-settlement", "settle-6", 6),
		}},
	}
	o := v9Options()
	br, m := v9Build(t, o, checks, journal, events)
	if len(m.SettlementRows) != 2 {
		t.Fatalf("settlement rows drift: %+v", m.SettlementRows)
	}
	for _, b := range br.V9Bands {
		if strings.HasPrefix(b.Name, "V9-T1") && !b.WithinBand {
			t.Fatalf("two settled contracts at first h6 must hold T1: %+v", b)
		}
	}
	table := map[[2]int]v9CounterpartyRow{}
	for _, row := range m.Counterparties {
		table[[2]int{row.EmployerGeneration, row.WorkerGeneration}] = row
	}
	ff, okFF := table[[2]int{0, 0}]
	fn, okFN := table[[2]int{0, 1}]
	if !okFF || ff.Settlements != 1 || ff.EmployerClass != "founder" || ff.WorkerClass != "founder" {
		t.Fatalf("founder→founder counterparty row drift: %+v (%v)", ff, m.Counterparties)
	}
	if !okFN || fn.Settlements != 1 || fn.WorkerClass != "newborn" {
		t.Fatalf("founder→newborn counterparty row drift: %+v (%v)", fn, m.Counterparties)
	}
	perActor := map[int]v9ActorTradeRow{}
	for _, row := range m.PerActor {
		perActor[row.ID] = row
	}
	if perActor[1].WagesEarned != 1 || perActor[2].WagesPaid != 2 || perActor[17].WagesEarned != 1 || perActor[17].Generation != 1 {
		t.Fatalf("per-actor wage ledger or generation drift: %+v", perActor)
	}
	if m.WageLedger.WagesPaidTotal != 2 || m.WageLedger.WagesEarnedTotal != 2 {
		t.Fatalf("wage totals drift: %+v", m.WageLedger)
	}
	if m.WageLedger.PatchHourImbalanceCount != 0 {
		t.Fatalf("the balanced fixture must not flag imbalances: %+v", m.WageLedger)
	}
	// SF″ = SF′ − wagesPaid + wagesEarned with trailing-24h wage-ledger
	// deltas, recomputed independently from the fixture: the paid flows are
	// the employer's hourly ledger deltas and the earned flows the worker's
	// plus the newborn's, each summed over the trailing 24-hour window.
	if len(m.SFDoublePrimeByHourTotal) != len(m.SFPrimeByHourTotal) {
		t.Fatalf("SF series length drift: %d vs %d", len(m.SFPrimeByHourTotal), len(m.SFDoublePrimeByHourTotal))
	}
	// Employer wages-paid deltas at h4..h7; earned deltas are the worker's
	// (its ledger rises once at h6) plus the newborn's (at h7). The trailing
	// sums coincide hour-for-hour here because every settlement is same-patch
	// and the per-patch-hour wage equality holds — so the aggregate SF″
	// coincides with the frozen SF′, which is exactly the closure the
	// printed-beside figure cross-checks; per actor the adjustment shows in
	// the wage ledgers below.
	paidDeltas := []int64{0, 0, 1, 1}
	earnedDeltas := []int64{0, 0, 1, 1}
	trailing24 := func(deltas []int64, i int) int64 {
		var sum int64
		for j := i; j >= 0 && i-j < sfWindowHours; j-- {
			sum += deltas[j]
		}
		return sum
	}
	for i := range m.SFDoublePrimeByHourTotal {
		want := m.SFPrimeByHourTotal[i] - trailing24(paidDeltas, i) + trailing24(earnedDeltas, i)
		if got := m.SFDoublePrimeByHourTotal[i]; got != want {
			t.Fatalf("SF″ drift at h%d: got %d want %d (SF′ %d)", checks[i].Hour, got, want, m.SFPrimeByHourTotal[i])
		}
	}
	if m.SFDoublePrimeRunTotal != m.SFPrimeRunTotal-m.WageLedger.WagesPaidTotal+m.WageLedger.WagesEarnedTotal {
		t.Fatalf("SF″ run-total identity drift: %d vs %d", m.SFDoublePrimeRunTotal, m.SFPrimeRunTotal)
	}
	// The worker's first granary hour is its settlement hour and no stock
	// sighting precedes it, so the wage-banked first is counted; the eat at
	// E≤3 on the prior boundary is the strict subset.
	if m.WageBankedFirsts != 2 || m.WageMealDraws != 1 || m.WageMealDrawsBoundaryE3 != 1 {
		t.Fatalf("worker channel drift: firsts=%d meals=%d e3=%d", m.WageBankedFirsts, m.WageMealDraws, m.WageMealDrawsBoundaryE3)
	}
	// The scripted imbalance breaks T3 verbatim.
	brBad, mBad := v9Build(t, o, bad, journal, events)
	if mBad.WageLedger.PatchHourImbalanceCount == 0 || len(mBad.WageLedger.PatchHourImbalances) == 0 {
		t.Fatalf("scripted imbalance must be flagged: %+v", mBad.WageLedger)
	}
	for _, b := range brBad.V9Bands {
		if strings.HasPrefix(b.Name, "V9-T3") && b.WithinBand {
			t.Fatal("T3 must report OUT with a patch-hour wage imbalance")
		}
	}
}

// Review P1 (ticket-20 final): the reporter must emit the corner label the
// ticket declared BEFORE the matrix ran (TRADE-INERT), not the superseded
// placeholder, so the report bytes and the recorded vocabulary agree. The
// three frozen branches must stay unchanged.
func TestNaturalCLIV9TradeInertLabelIsTheDeclaredOne(t *testing.T) {
	if v9VerdictTradeInert != "TRADE-INERT" {
		t.Fatalf("declared corner label changed: %q", v9VerdictTradeInert)
	}
	if v9VerdictSupported != "SUPPORTED" || v9VerdictNullTrade != "NULL-TRADE" || v9VerdictFailed != "FAILED" {
		t.Fatal("the three frozen verdict labels changed")
	}
	if strings.Contains(strings.Join([]string{v9VerdictSupported, v9VerdictNullTrade, v9VerdictFailed, v9VerdictTradeInert}, ","), "no-frozen") {
		t.Fatal("the superseded placeholder is still among the labels")
	}
}
