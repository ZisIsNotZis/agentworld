package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// naturalV9YieldWorld builds the small declared v9 calibration fixture: one
// patch, the frozen budget, a declared yield, and natural/13.
func naturalV9YieldWorld(t *testing.T, yield int64, patches int) *Natural {
	t.Helper()
	f, err := NewNatural(NaturalOptions{Patches: patches, R: 8, Yield: yield, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy13().Ref, Mechanism: NaturalMechanismV9})
	if err != nil {
		t.Fatalf("v9 yield %d world: %v", yield, err)
	}
	return f
}

// TestNaturalV9YieldAxisIsIdentity proves the declared yield is refused by
// every mechanism below v9, admitted under v9 in 1..NaturalYieldMax, and
// carried through the world seam.
func TestNaturalV9YieldAxisIsIdentity(t *testing.T) {
	if !NaturalMechanismValid(NaturalMechanismV9) || NaturalMechanismV9.worldVersion() != 9 {
		t.Fatal("mechanism v9 is not admitted as world version 9")
	}
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7, NaturalMechanismV8} {
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1,
			PolicyRef: strategy.FrozenNaturalPolicy13().Ref, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("yield 2 accepted under mechanism %d: %v", mechanism, err)
		}
	}
	for _, yield := range []int64{0, 1, 2, NaturalYieldMax} {
		f := naturalV9YieldWorld(t, yield, 1)
		want := yield
		if want == 0 {
			want = 1
		}
		if f.yield != want {
			t.Fatalf("v9 yield %d resolved to %d, want %d", yield, f.yield, want)
		}
		if f.mechanism != NaturalMechanismV9 || f.declaredYield() != want {
			t.Fatalf("v9 world bound mechanism %d / declared yield %d", f.mechanism, f.declaredYield())
		}
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: NaturalYieldMax + 1, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy13().Ref, Mechanism: NaturalMechanismV9}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("yield %d accepted: %v", NaturalYieldMax+1, err)
	}
	// A journal config may only declare a yield under v9.
	if validNaturalJournalConfig(NaturalJournalConfig{Patches: 2, R: 8, Yield: 2, Mechanism: NaturalMechanismV8,
		Policy: strategy.FrozenNaturalPolicy13().Ref}) {
		t.Fatal("v8 journal config admitted a yield")
	}
	if f := naturalV9YieldWorld(t, 0, 1); f.declaredYield() != 1 {
		t.Fatalf("v9 default yield declared as %d, want 1", f.declaredYield())
	}
	if f, err := NewNatural(NaturalOptions{Patches: 1, R: 8, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy13().Ref, Mechanism: NaturalMechanismV8}); err != nil || f.declaredYield() != 0 {
		t.Fatalf("v8 default declared yield %d (%v), want 0 (omitted)", f.declaredYield(), err)
	}
}

// naturalV9ConservationFixture builds a ledger-coherent one-patch v9 world
// with three cultivated slots (owners 1..3, two food units each) and five
// wild slots, three of them having yielded two gathered units each.
func naturalV9ConservationFixture(t *testing.T) ([]NaturalPatchState, [][]NaturalSlotState, []NaturalActorState) {
	t.Helper()
	patches := 1
	patch := NaturalPatchState{Yield: 8, Pulses: 2, Produced: 6, Granted: 6, Unrealized: 7}
	slots := make([]NaturalSlotState, NaturalSlotsPerPatch)
	for i := 0; i < 3; i++ {
		slots[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: sim.EntityID(i + 1), Cultivated: 1}
	}
	for i := 3; i < 6; i++ {
		slots[i] = NaturalSlotState{Gathered: 2}
	}
	founders := make([]NaturalActorState, 8)
	for i := range founders {
		id := sim.EntityID(i + 1)
		a, err := NaturalFounderState(patches, id, 140)
		if err != nil {
			t.Fatal(err)
		}
		if id <= 3 {
			a.Granary = NaturalGranaryState{Stock: 2, CultivatedTotal: 2, LastStoredMealHour: NaturalNeverHour}
			a.Worksite = NaturalWorksiteState{Capital: 1, InvestedUnits: 2, PointsCreated: 1, LastBuildHour: NaturalNeverHour}
			if !validNaturalActorFor(NaturalMechanismV9, patches, id, a) {
				t.Fatalf("fixture owner %d invalid: %+v", id, a)
			}
		}
		founders[i] = a
	}
	return []NaturalPatchState{patch}, [][]NaturalSlotState{slots}, founders
}

// TestNaturalV9ConservationScalesGrant proves the v9 ledger keeps balancing
// while the rule-508 grant carries food units: the same fixture balances at
// yield 2 (exactly two units per cultivated slot) and is refused at yield 1,
// where the granted units cannot be stated in the declared budget.
func TestNaturalV9ConservationScalesGrant(t *testing.T) {
	patchStates, slots, founders := naturalV9ConservationFixture(t)
	last, err := NaturalHighestFixedEntityID(1)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := NaturalCheckConservationYieldFor(NaturalMechanismV9, 2, 1, patchStates, slots, founders, nil, last)
	if err != nil {
		t.Fatalf("yield-2 v9 ledger not conserved: %v", err)
	}
	if balance.Granted != 6 || balance.CultivatedTotal != 6 {
		t.Fatalf("yield-2 granted/cultivated %d/%d, want 6/6", balance.Granted, balance.CultivatedTotal)
	}
	if _, err := NaturalCheckConservationYieldFor(NaturalMechanismV9, 1, 1, patchStates, slots, founders, nil, last); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("yield-1 ledger accepted six units over a one-unit grant: %v", err)
	}
}

// TestNaturalV9RegenSplitMultipliesPerSlotGrant proves the multiplication
// site: each entitled cultivated slot consumes one R budget unit but grants
// the declared yield, while the frozen split keeps one unit per slot.
func TestNaturalV9RegenSplitMultipliesPerSlotGrant(t *testing.T) {
	patch := NaturalPatchState{Yield: 8, Pulses: 1, Produced: 2, Unrealized: 6}
	slots := [NaturalSlotsPerPatch]NaturalSlotState{}
	for i := 0; i < 3; i++ {
		slots[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: sim.EntityID(i + 1), Cultivated: 1}
	}
	slots[3] = NaturalSlotState{Stock: 1}
	slots[4] = NaturalSlotState{Stock: 1}
	alive := NaturalAliveLookup(func(sim.EntityID) bool { return true })

	next, _, result, grants, err := NaturalRegenSplitFor(NaturalMechanismV9, 2, 1, 1, patch, slots, alive)
	if err != nil {
		t.Fatalf("v9 regen split: %v", err)
	}
	if result.Cultivated != 6 || result.Wild != 3 || result.Unrealized != 2 {
		t.Fatalf("v9 split %+v, want cultivated 6, wild 3, unrealized 2", result)
	}
	if next.Granted != 6 || next.Produced != 5 || next.Unrealized != 8 {
		t.Fatalf("v9 patch ledger %+v", next)
	}
	if len(grants) != 3 {
		t.Fatalf("v9 grants %+v, want three owners", grants)
	}
	for _, g := range grants {
		if g.Units != 2 {
			t.Fatalf("v9 grant %+v, want two units per cultivated slot", g)
		}
	}
	// The frozen split is the one-unit ledger, byte for byte.
	frozen, _, frozenResult, frozenGrants, err := NaturalRegenSplit(1, 1, patch, slots, alive)
	if err != nil {
		t.Fatal(err)
	}
	if frozenResult.Cultivated != 3 || frozen.Granted != 3 || frozenGrants[0].Units != 1 {
		t.Fatalf("frozen split changed: %+v %+v", frozenResult, frozen)
	}
	if _, _, _, _, err := NaturalRegenSplitFor(NaturalMechanismV9, 0, 1, 1, patch, slots, alive); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("v9 split admitted yield 0: %v", err)
	}
}

// TestNaturalV9YieldCheckpointIdentity proves the declared yield is run
// identity end to end: a v9 bundle restores only under its own yield, and
// both journal and manifest reject a mismatched value.
func TestNaturalV9YieldCheckpointIdentity(t *testing.T) {
	ctx := context.Background()
	f := naturalV9YieldWorld(t, 2, 2)
	if ok, err := f.Step(ctx); err != nil || !ok {
		t.Fatalf("v9 h0 pulse: %t %v", ok, err)
	}
	path := t.TempDir() + "/v9.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	ref := strategy.FrozenNaturalPolicy13().Ref
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 2, Seed: 0,
		Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV9})
	if err != nil {
		t.Fatalf("v9 yield-2 restore: %v", err)
	}
	if restored.yield != 2 {
		t.Fatalf("restored yield %d, want 2", restored.yield)
	}
	for _, mismatched := range []int64{1, 3} {
		if _, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: mismatched, Seed: 0,
			Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV9}); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("yield %d restore accepted a yield-2 bundle: %v", mismatched, err)
		}
	}
	// The matching config round-trips: the save path already exports and
	// replays the journal under the declared yield, so a journal-config
	// disagreement fails closed (pinned by the mismatched restores above).
	if _, err := f.ExportJournal(); err != nil {
		t.Fatalf("v9 journal export under yield 2: %v", err)
	}
}
