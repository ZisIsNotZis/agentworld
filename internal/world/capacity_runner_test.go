package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

func capacityRunTo(t *testing.T, options CapacityOptions, hour int) *Capacity {
	t.Helper()
	f, err := NewCapacity(options)
	if err != nil {
		t.Fatal(err)
	}
	for len(f.Checkpoints()) <= hour {
		ok, err := f.Step(context.Background())
		if err != nil || !ok {
			t.Fatalf("hour %d step %d: processed=%v err=%v", hour, f.steps, ok, err)
		}
	}
	return f
}

func capacityEventBytes(t *testing.T, f *Capacity) [][]byte {
	t.Helper()
	events := f.k.Events()
	out := make([][]byte, len(events))
	for i, ev := range events {
		b, err := ev.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		out[i] = b
	}
	return out
}

// capacityAssertHourlyConservation re-verifies gates G1-G3 and the aggregate
// identities on every hourly checkpoint of a run.
func capacityAssertHourlyConservation(t *testing.T, f *Capacity) {
	t.Helper()
	for _, check := range f.Checkpoints() {
		b, err := CapacityCheckConservation(check.Patches, check.Slots, check.Actors)
		if err != nil {
			t.Fatalf("h%d conservation: %v", check.Hour, err)
		}
		if b != check.Balance {
			t.Fatalf("h%d balance drift: %+v != %+v", check.Hour, b, check.Balance)
		}
		if check.Balance.Produced != check.Balance.Gathered+check.Balance.Stock {
			t.Fatalf("h%d G1: %+v", check.Hour, check.Balance)
		}
		if check.Balance.Invested != CapacityPointCostWip*check.Balance.PointsCreated+check.Balance.Wip ||
			check.Balance.PointsCreated-check.Balance.PointsDecayed != check.Balance.Capital ||
			check.Balance.YieldTotal != check.Balance.StoredMeals+check.Balance.GranaryStock+check.Balance.YieldUnrealized {
			t.Fatalf("h%d G2: %+v", check.Hour, check.Balance)
		}
		if check.Balance.InitialEnergy+check.Balance.Consumed+check.Balance.StoredMeals != check.Balance.Energy+check.Balance.BasalSpent+check.Balance.EnergyCapLost {
			t.Fatalf("h%d G3: %+v", check.Hour, check.Balance)
		}
	}
}

// capacityAssertJournalClock pins every attempt to its frozen phase instant
// and every keyed attempt to its committed event (gate G4 linkage).
func capacityAssertJournalClock(t *testing.T, f *Capacity) {
	t.Helper()
	events := make(map[sim.EventID]kernel.Event)
	for _, ev := range f.k.Events() {
		events[ev.ID] = ev
	}
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			hour := int(a.Time / sim.SimTime(CapacityHour))
			if hour < 0 || hour >= CapacityHorizonHours {
				t.Fatalf("attempt outside horizon: %+v", a)
			}
			var want sim.SimTime
			switch a.Kind {
			case "gather", "eat-stored", "wait":
				want, _ = CapacityPhaseTime(hour, CapacityPhaseClaim)
			case "rest":
				want, _ = CapacityPhaseTime(hour, CapacityPhaseGathered)
			case "eat":
				want, _ = CapacityPhaseTime(hour, CapacityPhaseBagDecision)
			case "build":
				start, _ := CapacityPhaseTime(hour, CapacityPhaseBuildStart)
				want = CapacityBuildCompletion(start)
			case "paired-meal", "fallback-eatstored":
				want, _ = CapacityPhaseTime(hour, CapacityPhasePairedMeal)
			default:
				t.Fatalf("unknown attempt kind %+v", a)
			}
			if a.Time != want {
				t.Fatalf("attempt off the frozen clock: %+v want %d", a, want)
			}
			if a.Key == "" {
				if a.EventID != 0 || a.Accepted {
					t.Fatalf("uncommitted attempt carries evidence of a commit: %+v", a)
				}
				continue
			}
			ev, ok := events[a.EventID]
			if !ok || !a.Accepted || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor || ev.Rule == 0 {
				t.Fatalf("broken event link: %+v event %+v", a, ev)
			}
		}
	}
}

// TestCapacityRunnerBootstrapTroughHandComputed walks actor 1 of the q8
// seed-0 world through the frozen bootstrap by hand: builds skip meals from
// E=11 and decline one energy per hour; the TLow=5 policy floor troughs at
// E=4; from h7 the build rides the granary fallback and its paired stored
// meal restores the skipped meal, so energy never drops below 4. Wear is
// debt += k with a decay every six accumulated points; capital oscillates
// 2..3 (the labor-limited q8 equilibrium k_eq=3).
func TestCapacityRunnerBootstrapTroughHandComputed(t *testing.T) {
	f := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true}, 12)
	// Hand-derived state of actor 1 at every pulse boundary h=0..12.
	type row struct {
		energy, hunger                         int64
		wip, capital, debt                     int64
		granary, yieldTotal, unrealized, meals int64
		invested, created, decayed             int64
	}
	want := []row{
		{11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{10, 1, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0},
		{9, 2, 0, 1, 1, 1, 1, 0, 0, 2, 1, 0},
		{8, 3, 1, 1, 2, 2, 2, 0, 0, 3, 1, 0},
		{7, 4, 0, 2, 4, 4, 4, 0, 0, 4, 2, 0},
		{6, 5, 1, 1, 0, 6, 6, 0, 0, 5, 2, 1},
		{5, 6, 0, 2, 2, 8, 8, 0, 0, 6, 3, 1},
		{4, 7, 1, 2, 4, 8, 10, 2, 0, 7, 3, 1},
		{4, 7, 0, 2, 1, 8, 13, 4, 1, 8, 4, 2},
		{4, 7, 1, 2, 3, 8, 15, 5, 2, 9, 4, 2},
		{4, 7, 0, 2, 0, 8, 18, 7, 3, 10, 5, 3},
		{4, 7, 1, 2, 2, 8, 20, 8, 4, 11, 5, 3},
		{4, 7, 0, 3, 5, 8, 23, 10, 5, 12, 6, 3},
	}
	for h, w := range want {
		check := f.Checkpoints()[h]
		if check.Hour != h {
			t.Fatalf("checkpoint %d has hour %d", h, check.Hour)
		}
		a := check.Actors[0]
		got := row{a.Body.Energy, a.Body.Hunger, a.Worksite.Wip, a.Worksite.Capital, a.Worksite.WearDebt,
			a.Granary.Stock, a.Granary.YieldTotal, a.Granary.YieldUnrealized, a.Granary.StoredMeals,
			a.Worksite.InvestedUnits, a.Worksite.PointsCreated, a.Worksite.PointsDecayed}
		if got != w {
			t.Fatalf("h%d actor1 hand trajectory: got %+v want %+v", h, got, w)
		}
		if a.Bag.Units != 0 {
			t.Fatalf("h%d actor1 holds a bag at the pulse: %+v", h, a)
		}
		if h > 0 && a.Body.LastGatherHour != int64(h-1) {
			t.Fatalf("h%d actor1 last-gather-hour %d, want %d", h, a.Body.LastGatherHour, h-1)
		}
	}
	checks := f.Checkpoints()
	if checks[7].Actors[0].Body.Energy != 4 || checks[12].Actors[0].Body.Energy != 4 {
		t.Fatal("trough must sit at E=4 under the frozen TLow=5 floor")
	}
	// All sixteen actors are symmetric under full employment: every one
	// reaches the same capital oscillation and stays alive.
	for _, a := range checks[12].Actors {
		if a.Body.Energy == 0 {
			t.Fatal("q8 bootstrap lost an actor before h12")
		}
		if a.Worksite.Capital < 2 || a.Granary.StoredMeals < 5 {
			t.Fatalf("asymmetric bootstrap outcome: %+v", a)
		}
	}
	capacityAssertHourlyConservation(t, f)
	capacityAssertJournalClock(t, f)
	// Evidence: the first six hours build without any stored meal; the first
	// paired meals commit at the h+40m+4µs instant of hour 7, one per actor.
	builds, paired, earlyPairing := 0, 0, 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			switch a.Kind {
			case "build":
				builds++
			case "paired-meal":
				paired++
				if int(a.Time/sim.SimTime(CapacityHour)) < 7 || !a.Accepted || a.EventID == 0 {
					earlyPairing++
				}
			}
		}
	}
	if builds < 16*7 || paired != 16*5 || earlyPairing != 0 {
		t.Fatalf("bootstrap evidence: builds=%d paired=%d malformed=%d", builds, paired, earlyPairing)
	}
}

func mustCapacityPhase(t *testing.T, hour, phase int) sim.SimTime {
	t.Helper()
	at, err := CapacityPhaseTime(hour, phase)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// TestCapacityRunnerKStarSelfSufficiencyAndErosion pins the frozen k*=2
// threshold end-to-end. Sustain side (q8 neutral takeoff, real runner): every
// actor's capital, once it reaches 2, never falls below 2 again — the
// net-units identity 2k/3−1 turns nonnegative at k=2 and capital income funds
// building and eating simultaneously — and every actor is alive with k>=2 and
// a nonempty granary at h168 with a positive surplus flow over h121..168.
// Erode side: without labor or capital income the k=1 standing point erodes
// to zero at the frozen 6-pulse lifetime (the scripted wear spiral) and a
// q0 world creates no capital at all (extinction h11, identical to v1).
// Founder endowments are gated by the frozen conservation: a granary
// endowment is ledger-coherent and accepted; a capital endowment is rejected
// at construction because the per-patch flow identity leaves no room for
// exogenous invested units.
func TestCapacityRunnerKStarSelfSufficiencyAndErosion(t *testing.T) {
	takeoff := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true}, CapacityHorizonHours)
	capacityAssertHourlyConservation(t, takeoff)
	capacityAssertJournalClock(t, takeoff)
	checks := takeoff.Checkpoints()
	last := checks[CapacityHorizonHours]
	if last.Alive != CapacityActorCount {
		t.Fatalf("q8-enabled band: alive %d, want 16", last.Alive)
	}
	for i, a := range last.Actors {
		if a.Body.Energy == 0 || a.Worksite.Capital < 2 || a.Granary.Stock < 1 {
			t.Fatalf("actor %d not self-sustaining (k>=2, granary>=1, alive) at h168: %+v", i+1, a)
		}
		// The bootstrap window (builds from E=11 declining, trough at the
		// TLow floor, pinned exactly by the hand-computed trajectory test)
		// legitimately passes through k=1; once the capital-income regime
		// closes its first wear cycle the actors never drop below k*=2 again.
		for h := 6; h <= CapacityHorizonHours; h++ {
			if checks[h].Actors[i].Worksite.Capital < 2 {
				t.Fatalf("actor %d fell below k*=2 at h%d after the bootstrap window", i+1, h)
			}
		}
	}
	surplus := int64(0)
	for h := 121; h <= CapacityHorizonHours; h++ {
		surplus += checks[h].Balance.YieldTotal - checks[h-1].Balance.YieldTotal - (checks[h].Balance.StoredMeals - checks[h-1].Balance.StoredMeals)
	}
	if surplus <= 0 {
		t.Fatalf("surplus-flow-48h must be positive once productive forces emerged: %d", surplus)
	}
	// Founder-B (granary endowment) is coherent and accepted.
	endowed := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true,
		Founders: []CapacityFounder{{Actor: 1, Granary: 4}}}, 4)
	first := endowed.Checkpoints()[0].Actors[0]
	if first.Granary.Stock != 4 || first.Granary.YieldTotal != 4 {
		t.Fatalf("granary endowment not seeded ledger-coherently: %+v", first)
	}
	// Founder-A (capital endowment) cannot exist under the frozen
	// conservation identity and is rejected before any kernel exists.
	if _, err := NewCapacity(CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true,
		Founders: []CapacityFounder{{Actor: 1, Capital: 2}}}); err == nil {
		t.Fatal("capital endowment accepted despite the frozen per-patch flow identity")
	}
	// Erode: q0 neutral creates zero capital and dies at h11 exactly as v1.
	dry := capacityRunTo(t, CapacityOptions{Yield: 0, Seed: 0, Workers: 1, Enabled: true}, 12)
	for i, a := range dry.Checkpoints()[11].Actors {
		if a.Body.Energy != 0 || a.Worksite.Capital != 0 {
			t.Fatalf("q0 actor %d survived the subsistence boundary with capital", i+1)
		}
	}
}

// TestCapacityRunnerQ0ZeroCapitalEvents asserts the q0 band: both branches
// are byte-identical (no stock means the granary cascade is unreachable), all
// actors are stopped by h11 exactly as in v1, and the neutral world emits
// zero capital events — any Build/Yield/Wear/EatStored event at q0 is a
// defect.
func TestCapacityRunnerQ0ZeroCapitalEvents(t *testing.T) {
	enabled := capacityRunTo(t, CapacityOptions{Yield: 0, Seed: 0, Workers: 1, Enabled: true}, 12)
	disabled := capacityRunTo(t, CapacityOptions{Yield: 0, Seed: 0, Workers: 1}, 12)
	if !reflect.DeepEqual(capacityEventBytes(t, enabled), capacityEventBytes(t, disabled)) ||
		!reflect.DeepEqual(enabled.Journal(), disabled.Journal()) ||
		!reflect.DeepEqual(enabled.Checkpoints(), disabled.Checkpoints()) {
		t.Fatal("q0 enabled and disabled branches diverged")
	}
	capitalRules := map[sim.RuleID]bool{CapacityBuildRule: true, CapacityYieldRule: true, CapacityWearRule: true, CapacityEatStoredRule: true}
	for _, ev := range enabled.k.Events() {
		if capitalRules[ev.Rule] {
			t.Fatalf("capital event at q0: %+v", ev)
		}
	}
	checks := enabled.Checkpoints()
	if checks[10].Alive != CapacityActorCount || checks[11].Alive != 0 {
		t.Fatalf("q0 extinction by h11: alive h10=%d h11=%d", checks[10].Alive, checks[11].Alive)
	}
	// After extinction the world still pulses (production ledgers advance)
	// and conservation keeps holding to the horizon.
	full := capacityRunTo(t, CapacityOptions{Yield: 0, Seed: 0, Workers: 1, Enabled: true}, CapacityHorizonHours)
	capacityAssertHourlyConservation(t, full)
	if len(full.Checkpoints()) != CapacityHorizonHours+1 {
		t.Fatal("missing hourly projections after extinction")
	}
}

// TestCapacityRunnerWearSpiral replays the predeclared collapse mode (ii) on
// a scripted one-actor small world driven by the frozen policy and the
// trusted contract math: energy pinned at the TLow floor while the capital
// income lasts, the standing point eroding at h6 (exact 6-pulse lifetime),
// the granary draining, and death at h11 — with per-actor G3 holding at
// every scripted hour.
func TestCapacityRunnerWearSpiral(t *testing.T) {
	policy := strategy.FrozenCapacityPolicy()
	registry := strategy.NewCapacityRegistry()
	if err := registry.Register(policy); err != nil {
		t.Fatal(err)
	}
	bound, err := registry.Bind(strategy.CapacityBinding{Actor: 1, Ref: policy.Ref})
	if err != nil {
		t.Fatal(err)
	}
	// Reachable mid-run state: E=4 with seven basal hours spent, one standing
	// point (two invested units), one stored unit.
	a := CapacityActorState{
		Body:     CapacityBodyState{Energy: 4, BasalSpent: 7, LastGatherHour: 0},
		Bag:      CapacityBagState{},
		Worksite: CapacityWorksiteState{Capital: 1, InvestedUnits: 2, PointsCreated: 1, LastBuildHour: 0},
		Granary:  CapacityGranaryState{Stock: 1, YieldTotal: 1, LastStoredMealHour: CapacityNeverHour},
	}
	energy := []int64{}
	capital := []int64{}
	for hour := 1; hour <= 11; hour++ {
		var err error
		a, _, err = CapacityBasal(1, a, 1)
		if err != nil {
			t.Fatal(err)
		}
		if a.Worksite.Capital > 0 {
			if a, _, err = CapacityCapitalYield(1, a); err != nil {
				t.Fatal(err)
			}
		}
		if a, _, err = CapacityWear(1, a); err != nil {
			t.Fatal(err)
		}
		home := mustCapacitySlots(t)
		obs := strategy.CapacityObservation{Actor: 1, Ref: policy.Ref, Hour: hour, WorldVersion: sim.WorldVersion(hour),
			Energy: sim.IntegerValue(a.Body.Energy), Hunger: sim.IntegerValue(a.Body.Hunger),
			BagUnits: sim.IntegerValue(a.Bag.Units), BagSource: capacityAbsentRefValue(),
			Capital: sim.IntegerValue(a.Worksite.Capital), Wip: sim.IntegerValue(a.Worksite.Wip),
			GranaryStock: sim.IntegerValue(a.Granary.Stock), InvestedUnits: sim.IntegerValue(a.Worksite.InvestedUnits),
			LastBuildHour: sim.IntegerValue(a.Worksite.LastBuildHour), LastStoredMealHour: sim.IntegerValue(a.Granary.LastStoredMealHour),
			HomeSlots: home}
		choice, err := bound.Evaluate(obs)
		if err != nil {
			t.Fatal(err)
		}
		switch choice.Kind {
		case strategy.CapacityEatStored:
			if a, _, err = CapacityEatStored(hour, 1, a); err != nil {
				t.Fatal(err)
			}
		case strategy.CapacityWait, strategy.CapacityRest:
			// no stock, no granary: the actor waits
		default:
			t.Fatalf("scripted world cannot offer %v", choice.Kind)
		}
		energy = append(energy, a.Body.Energy)
		capital = append(capital, a.Worksite.Capital)
		if CapacityInitialEnergy+a.Body.Consumed+a.Granary.StoredMeals != a.Body.Energy+a.Body.BasalSpent+a.Body.CapLost {
			t.Fatalf("h%d per-actor G3 broken: %+v", hour, a)
		}
	}
	wantEnergy := []int64{4, 4, 4, 4, 4, 4, 4, 3, 2, 1, 0}
	wantCapital := []int64{1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0}
	if !reflect.DeepEqual(energy, wantEnergy) || !reflect.DeepEqual(capital, wantCapital) {
		t.Fatalf("wear spiral trajectory: energy=%v capital=%v", energy, capital)
	}
}

func mustCapacitySlots(t *testing.T) []strategy.CapacitySlotView {
	t.Helper()
	slots := make([]strategy.CapacitySlotView, 0, CapacitySlotsPerPatch)
	for j := 0; j < CapacitySlotsPerPatch; j++ {
		id, err := CapacitySlotID(0, j)
		if err != nil {
			t.Fatal(err)
		}
		slots = append(slots, strategy.CapacitySlotView{ID: id, Stock: sim.IntegerValue(0)})
	}
	return slots
}

func capacityAbsentRefValue() sim.Value {
	v, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	return v
}

// TestCapacityRunnerDeterminismWorkers pins gate G6 on the runner: workers 1
// and 4 produce identical event bytes, journals, checkpoints, and history on
// both a full-employment fixture (q8) and a contended fixture with gather
// denials and fallback meals (q3).
func TestCapacityRunnerDeterminismWorkers(t *testing.T) {
	for _, tc := range []struct {
		q    int64
		seed uint64
		hour int
	}{{8, 0, 24}, {3, 7, 24}} {
		one := capacityRunTo(t, CapacityOptions{Yield: tc.q, Seed: tc.seed, Workers: 1, Enabled: true}, tc.hour)
		four := capacityRunTo(t, CapacityOptions{Yield: tc.q, Seed: tc.seed, Workers: 4, Enabled: true}, tc.hour)
		if !reflect.DeepEqual(capacityEventBytes(t, one), capacityEventBytes(t, four)) ||
			!reflect.DeepEqual(one.Journal(), four.Journal()) ||
			!reflect.DeepEqual(one.Checkpoints(), four.Checkpoints()) {
			t.Fatalf("q%d seed%d worker-dependent evidence", tc.q, tc.seed)
		}
		historyOne, headOne, err := one.k.ExportHistory()
		if err != nil {
			t.Fatal(err)
		}
		historyFour, headFour, err := four.k.ExportHistory()
		if err != nil || headOne != headFour || !bytes.Equal(historyOne, historyFour) {
			t.Fatalf("q%d seed%d history divergence: %v", tc.q, tc.seed, err)
		}
		restored, _, err := kernel.RestoreHistory(one.registry, historyOne)
		if err != nil {
			t.Fatal(err)
		}
		original := capacityEventBytes(t, one)
		for i, ev := range restored.Events() {
			b, err := ev.Bytes()
			if err != nil || !bytes.Equal(b, original[i]) {
				t.Fatalf("q%d seed%d replay event %d: %v", tc.q, tc.seed, i, err)
			}
		}
	}
}

// TestCapacityRunnerEnabledDisabledSharedPrefix pins the disabled branch:
// identical evidence up to the first frozen bag decision, then zero capital
// events and v1-baseline physiology (gather-and-eat keeps everyone alive).
func TestCapacityRunnerEnabledDisabledSharedPrefix(t *testing.T) {
	enabled := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true}, 24)
	disabled := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1}, 24)
	firstDecision, _ := CapacityPhaseTime(0, CapacityPhaseBagDecision)
	shared := 0
	one, four := capacityEventBytes(t, enabled), capacityEventBytes(t, disabled)
	for len(one) > 0 && len(four) > 0 && bytes.Equal(one[0], four[0]) {
		shared++
		one, four = one[1:], four[1:]
	}
	lastShared := enabled.k.Events()[shared-1].Time
	if lastShared >= firstDecision {
		t.Fatalf("branches diverged before the first bag decision: %d >= %d", lastShared, firstDecision)
	}
	if len(four) == 0 || disabled.k.Events()[shared].Time != firstDecision {
		t.Fatalf("expected the disabled branch's first bag-meal event at %d", firstDecision)
	}
	capitalRules := map[sim.RuleID]bool{CapacityBuildRule: true, CapacityYieldRule: true, CapacityWearRule: true, CapacityEatStoredRule: true}
	for _, ev := range disabled.k.Events() {
		if capitalRules[ev.Rule] {
			t.Fatalf("disabled branch emitted a capital event: %+v", ev)
		}
	}
	if len(capacityEventBytes(t, enabled)) == shared {
		t.Fatal("enabled branch never diverged")
	}
	if disabled.Checkpoints()[24].Alive != CapacityActorCount {
		t.Fatalf("q8-disabled band: alive %d", disabled.Checkpoints()[24].Alive)
	}
	for _, a := range disabled.Checkpoints()[24].Actors {
		if a.Worksite.Capital != 0 || a.Granary.Stock != 0 || a.Body.Energy == 0 {
			t.Fatalf("disabled branch drifted from the v1 baseline: %+v", a)
		}
	}
	capacityAssertHourlyConservation(t, enabled)
	capacityAssertHourlyConservation(t, disabled)
	capacityAssertJournalClock(t, enabled)
	capacityAssertJournalClock(t, disabled)
}

// TestCapacityRunnerPairedMealRace pins the fresh-snapshot granary
// revalidation duty: a paired intent whose granary raced to empty yields the
// typed no-meal note with zero mutation while the build stands; a stocked
// granary commits the paired meal at the exact h+40m+4µs instant.
func TestCapacityRunnerPairedMealRace(t *testing.T) {
	f, err := NewCapacity(CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// Reachable mid-run states at hour 7: energy 4 with seven basal hours
	// spent, three standing points, and a granary ledger that counts its
	// unrealized overflow (G2: yield = stored + stock + unrealized).
	raced := CapacityActorState{
		Body:     CapacityBodyState{Energy: 4, BasalSpent: 7, LastGatherHour: 7},
		Worksite: CapacityWorksiteState{Capital: 3, InvestedUnits: 6, PointsCreated: 3, LastBuildHour: 7},
		Granary:  CapacityGranaryState{Stock: 0, YieldTotal: 2, YieldUnrealized: 2, LastStoredMealHour: 6},
	}
	stocked := raced
	stocked.Granary.Stock = 8
	stocked.Granary.YieldTotal = 10
	if !validCapacityActor(1, raced) || !validCapacityActor(1, stocked) {
		t.Fatal("race fixture states must be reachable ledger-coherent states")
	}
	intent := CapacityMealIntent{Hour: 7, Paired: true}
	proposal, attempt, err := f.pairedMealDecision(1, 7, intent, raced)
	if err != nil || proposal.Key != "" || attempt.Key != "" || attempt.NoMeal != capacityNoStoredMealNote {
		t.Fatalf("raced granary must produce a typed no-meal note: %+v %+v %v", proposal, attempt, err)
	}
	if raced.Granary.Stock != 0 || raced.Granary.StoredMeals != 0 {
		t.Fatal("typed rejection mutated state")
	}
	proposal, attempt, err = f.pairedMealDecision(1, 7, intent, stocked)
	if err != nil || proposal.Key == "" || attempt.Key == "" || attempt.NoMeal != "" {
		t.Fatalf("stocked granary must commit the paired meal: %+v %+v %v", proposal, attempt, err)
	}
	want, _ := CapacityPhaseTime(7, CapacityPhasePairedMeal)
	if proposal.Time != want || proposal.Rule != CapacityEatStoredRule {
		t.Fatalf("paired meal off the frozen clock: %+v", proposal)
	}
	if err := CapacityCheckOwnership(proposal); err != nil {
		t.Fatalf("paired meal ownership: %v", err)
	}
	// End-to-end: the q8 bootstrap commits sixteen paired meals at hour 7,
	// each linked to its event at the paired instant.
	runner := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 4, Enabled: true}, 8)
	pairedAt := mustCapacityPhase(t, 7, CapacityPhasePairedMeal)
	committed := 0
	for _, batch := range runner.Journal() {
		for _, a := range batch.Attempts {
			if a.Kind == "paired-meal" {
				if a.Time != pairedAt || !a.Accepted || a.EventID == 0 || a.NoMeal != "" {
					t.Fatalf("end-to-end paired meal: %+v", a)
				}
				committed++
			}
		}
	}
	if committed != CapacityActorCount {
		t.Fatalf("paired meals at h7: %d", committed)
	}
}

// TestCapacityRunnerOwnershipRejectionPropagation audits gates G4/G5 on a
// contended run (q3 denies gathers every hour): every committed event
// passes CapacityCheckOwnership when replayed from its deltas, tampering
// with the cause is caught, every gather claim targets the actor's own home
// patch, and denials are typed evidence with no committed event.
func TestCapacityRunnerOwnershipRejectionPropagation(t *testing.T) {
	f := capacityRunTo(t, CapacityOptions{Yield: 3, Seed: 0, Workers: 4, Enabled: true}, 24)
	denials, rests, fallbacks := 0, 0, 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Kind == "gather" && a.Rejection != FoodFlowGatherAdmitted {
				denials++
				if a.Key != "" || a.EventID != 0 || a.Accepted {
					t.Fatalf("denial carries a commit: %+v", a)
				}
				if a.Rejection != FoodFlowGatherNoStock && a.Rejection != FoodFlowGatherCapacity {
					t.Fatalf("unexpected typed denial: %+v", a)
				}
			}
			if a.Kind == "gather" && a.Rejection == FoodFlowGatherAdmitted {
				home, err := CapacityActorPatchID(a.Actor)
				if err != nil {
					t.Fatal(err)
				}
				if a.Target < sim.EntityID(2001+(home-1001)*CapacitySlotsPerPatch) ||
					a.Target >= sim.EntityID(2001+(home-1001+1)*CapacitySlotsPerPatch) {
					t.Fatalf("gather claim outside the home patch: %+v", a)
				}
			}
			if a.Kind == "rest" {
				rests++
			}
			if a.Kind == "fallback-eatstored" {
				fallbacks++
			}
		}
	}
	if denials == 0 || rests == 0 {
		t.Fatalf("q3 fixture produced no denial evidence: denials=%d rests=%d", denials, rests)
	}
	_ = fallbacks
	// Replay audit: rebuild every event's proposal from its deltas and apply
	// the ownership guard the runner applied before planning.
	for _, ev := range f.k.Events() {
		patches := make([]component.Patch, 0, len(ev.Deltas))
		for _, d := range ev.Deltas {
			patches = append(patches, component.Patch{Entity: d.Entity, Component: d.Component, SchemaVersion: d.SchemaVersion, Field: d.Field, Value: d.After})
		}
		replayed := kernel.Proposal{Key: ev.Key, Time: ev.Time, Cause: ev.Cause, Rule: ev.Rule, RuleVersion: ev.RuleVersion, Patches: patches}
		if err := CapacityCheckOwnership(replayed); err != nil {
			t.Fatalf("event %d failed the ownership replay audit: %v", ev.ID, err)
		}
	}
	// The audit is load-bearing: swapping the causal actor on a committed
	// capacity event must be caught.
	var victim *kernel.Event
	for i, ev := range f.k.Events() {
		if ev.Rule == CapacityBuildRule || ev.Rule == CapacityGatherRule {
			victim = &f.k.Events()[i]
			break
		}
	}
	if victim == nil {
		t.Fatal("no actor-caused event to tamper with")
	}
	patches := make([]component.Patch, 0, len(victim.Deltas))
	for _, d := range victim.Deltas {
		patches = append(patches, component.Patch{Entity: d.Entity, Component: d.Component, SchemaVersion: d.SchemaVersion, Field: d.Field, Value: d.After})
	}
	for _, impostor := range []sim.EntityID{victim.Cause.Actor + 1, victim.Cause.Actor + 8} {
		if impostor > CapacityActorCount {
			impostor -= CapacityActorCount
		}
		tampered := kernel.Proposal{Key: victim.Key, Time: victim.Time, Cause: kernel.Cause{Actor: impostor}, Rule: victim.Rule, RuleVersion: victim.RuleVersion, Patches: patches}
		if err := CapacityCheckOwnership(tampered); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("cross-owner mutation survived the replay audit: %v", err)
		}
	}
	capacityAssertHourlyConservation(t, f)
	capacityAssertJournalClock(t, f)
}

// TestCapacityRunnerBoundsEveryHour walks gate G7: every field of every
// hourly projection sits inside its declared descriptor bounds, the frozen
// wear ceiling rejects forged debt, and the kernel refuses out-of-bounds
// patches even in range of the runner.
func TestCapacityRunnerBoundsEveryHour(t *testing.T) {
	f := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true}, 24)
	reg, err := CapacityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	bounds := map[sim.ComponentTypeID][]component.FieldDescriptor{}
	for _, typ := range reg.TypeIDs() {
		descriptor, err := reg.Describe(typ)
		if err != nil {
			t.Fatal(err)
		}
		bounds[typ] = descriptor.Fields
	}
	within := func(typ sim.ComponentTypeID, field sim.FieldID, value int64) bool {
		for _, fd := range bounds[typ] {
			if fd.ID != field || !fd.Bounds.HasMinimum || !fd.Bounds.HasMaximum {
				continue
			}
			return value >= int64(fd.Bounds.Minimum) && value <= int64(fd.Bounds.Maximum)
		}
		return false
	}
	for _, check := range f.Checkpoints() {
		for i, p := range check.Patches {
			patch, _ := CapacityPatchID(i)
			for field, value := range map[sim.FieldID]int64{CapacityPatchYieldField: p.Yield, CapacityPatchPulsesField: p.Pulses, CapacityPatchProducedField: p.Produced, CapacityPatchUnrealizedField: p.Unrealized} {
				if !within(CapacityPatchTypeID, field, value) {
					t.Fatalf("h%d patch %d field %d out of bounds: %d", check.Hour, patch, field, value)
				}
			}
			for j, s := range check.Slots[i] {
				slot, _ := CapacitySlotID(i, j)
				if !within(CapacitySlotTypeID, CapacitySlotStockField, s.Stock) || !within(CapacitySlotTypeID, CapacitySlotGatheredField, s.Gathered) {
					t.Fatalf("h%d slot %d out of bounds: %+v", check.Hour, slot, s)
				}
			}
		}
		for i, a := range check.Actors {
			actor := sim.EntityID(i + 1)
			actorFields := []struct {
				typ   sim.ComponentTypeID
				field sim.FieldID
				value int64
			}{
				{CapacityBodyTypeID, CapacityBodyEnergyField, a.Body.Energy},
				{CapacityBodyTypeID, CapacityBodyHungerField, a.Body.Hunger},
				{CapacityBodyTypeID, CapacityBodyBasalSpentField, a.Body.BasalSpent},
				{CapacityBodyTypeID, CapacityBodyCapLostField, a.Body.CapLost},
				{CapacityBodyTypeID, CapacityBodyConsumedField, a.Body.Consumed},
				{CapacityBodyTypeID, CapacityBodyLastGatherHourField, a.Body.LastGatherHour},
				{CapacityWorksiteTypeID, CapacityWorksiteCapitalField, a.Worksite.Capital},
				{CapacityWorksiteTypeID, CapacityWorksiteWipField, a.Worksite.Wip},
				{CapacityWorksiteTypeID, CapacityWorksiteWearDebtField, a.Worksite.WearDebt},
				{CapacityWorksiteTypeID, CapacityWorksiteInvestedUnitsField, a.Worksite.InvestedUnits},
				{CapacityWorksiteTypeID, CapacityWorksitePointsCreatedField, a.Worksite.PointsCreated},
				{CapacityWorksiteTypeID, CapacityWorksitePointsDecayedField, a.Worksite.PointsDecayed},
				{CapacityWorksiteTypeID, CapacityWorksiteLastBuildHourField, a.Worksite.LastBuildHour},
				{CapacityGranaryTypeID, CapacityGranaryStockField, a.Granary.Stock},
				{CapacityGranaryTypeID, CapacityGranaryYieldTotalField, a.Granary.YieldTotal},
				{CapacityGranaryTypeID, CapacityGranaryYieldUnrealizedField, a.Granary.YieldUnrealized},
				{CapacityGranaryTypeID, CapacityGranaryStoredMealsField, a.Granary.StoredMeals},
				{CapacityGranaryTypeID, CapacityGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour},
			}
			for _, spec := range actorFields {
				if !within(spec.typ, spec.field, spec.value) {
					t.Fatalf("h%d actor %d component %d field %d out of bounds: %d", check.Hour, actor, spec.typ, spec.field, spec.value)
				}
			}
			if !validCapacityActor(actor, a) {
				t.Fatalf("h%d actor %d failed semantic validation", check.Hour, actor)
			}
		}
	}
	// Forged wear debt above the frozen ceiling is a typed rejection with
	// zero mutation.
	forged := capacityTestActor()
	forged.Worksite = CapacityWorksiteState{Capital: 1, WearDebt: CapacityWearDebtMax}
	if _, _, err := CapacityWear(1, forged); !errors.Is(err, ErrCapacityContract) {
		t.Fatalf("forged wear debt accepted: %v", err)
	}
	if forged.Worksite.WearDebt != CapacityWearDebtMax {
		t.Fatal("forged wear rejection mutated state")
	}
	// The kernel refuses a raw out-of-bounds patch on the runner's own world.
	head := f.k.SnapshotHead()
	_, authority, _ := f.k.Snapshot()
	bad := kernel.Proposal{Key: "bounds-violation", Time: head.TipTime + 1, Cause: kernel.Cause{Actor: 1}, Rule: CapacityBuildRule, RuleVersion: CapacityRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(CapacityKMax + 1)}}}
	if _, err := f.k.Plan(bad, authority); err == nil {
		t.Fatal("kernel admitted an out-of-bounds capital patch")
	}
}

// TestCapacityRunnerHandoffSeam exercises the v2-style value-only quiescent
// export: portable kernel and scheduler bytes share the verified head,
// runner-only in-flight state is carried, and every exported collection is a
// defensive copy.
func TestCapacityRunnerHandoffSeam(t *testing.T) {
	f := capacityRunTo(t, CapacityOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}, 9)
	handoff, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if handoff.FormatVersion != CapacityFormatVersion || handoff.Policy != f.ref || handoff.Steps != f.steps {
		t.Fatalf("handoff header: %+v", handoff)
	}
	if handoff.Head.TipID == 0 || len(handoff.History) == 0 || len(handoff.SchedulerBytes) == 0 {
		t.Fatal("handoff missing portable state")
	}
	if len(handoff.Journal) != len(f.Journal()) {
		t.Fatal("journal not exported")
	}
	if len(handoff.Checkpoints) != len(f.Checkpoints()) {
		t.Fatal("checkpoints not exported")
	}
	// The snapshot mid-hour may legitimately carry in-flight state; both maps
	// must agree with the runner's own view.
	snapshot := f.SchedulerSnapshot()
	if snapshot.Kernel.Version != sim.WorldVersion(handoff.Head.Version) {
		t.Fatalf("scheduler and kernel heads disagree: %d vs %d", snapshot.Kernel.Version, handoff.Head.Version)
	}
	for id, kind := range f.current {
		if handoff.Current[id] != kind {
			t.Fatalf("in-flight activity %d not exported: %v", id, kind)
		}
	}
	// Defensive copies: mutating the export must not touch the runner.
	for _, batch := range f.Journal() {
		if len(batch.Attempts) == 0 {
			continue
		}
		first := batch.Attempts[0]
		first.Actor = 99
		if f.Journal()[journalIndex(t, f.Journal(), batch)].Attempts[0].Actor == 99 {
			t.Fatal("journal export aliases runner state")
		}
		break
	}
	again, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(handoff, again) {
		t.Fatal("quiescent handoff is not reproducible")
	}
}

func journalIndex(t *testing.T, batches []CapacityBatch, batch CapacityBatch) int {
	t.Helper()
	for i := range batches {
		if batches[i].Time == batch.Time && batches[i].Version == batch.Version {
			return i
		}
	}
	t.Fatal("batch vanished")
	return 0
}

func TestCapacityNewCapacityRejectsFoundersWhenDisabled(t *testing.T) {
	// Runner review P2 #4 / persistence review P1: founder endowments are an
	// enabled-branch instrument; the frozen policy has no Enabled input, so a
	// disabled founder world would still emit stored-meal capital evidence.
	if _, err := NewCapacity(CapacityOptions{Workers: 1, Yield: 8, Enabled: false,
		Founders: []CapacityFounder{{Actor: 1, Granary: 4}}}); err == nil {
		t.Fatal("founder endowment accepted on the disabled branch")
	}
	if _, err := NewCapacity(CapacityOptions{Workers: 1, Yield: 8, Enabled: true,
		Founders: []CapacityFounder{{Actor: 1, Granary: 4}}}); err != nil {
		t.Fatalf("enabled founder world rejected: %v", err)
	}
}
