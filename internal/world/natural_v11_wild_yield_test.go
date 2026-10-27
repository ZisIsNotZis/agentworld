package world

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// TestNaturalV11Mechanism proves mechanism v11 is admitted as world version
// 11 and that the admitted axis still derives from one closed set.
func TestNaturalV11Mechanism(t *testing.T) {
	if !NaturalMechanismValid(NaturalMechanismV11) || NaturalMechanismV11.worldVersion() != 11 {
		t.Fatal("mechanism v11 is not admitted as world version 11")
	}
	if NaturalMechanismV11.FormatVersion() != 11 || NaturalMechanismV11.RuleVersion() != 11 ||
		NaturalMechanismV11.ProjectionVersion() != 11 {
		t.Fatal("mechanism v11 version fields")
	}
	if _, err := NaturalRegistryFor(NaturalMechanismV11); err != nil {
		t.Fatalf("v11 registry: %v", err)
	}
	// v11 keeps v10's round-robin founding band and declares the yield axis.
	if !naturalRoundRobinFounders(NaturalMechanismV11) || !naturalHasYieldAxis(NaturalMechanismV11) ||
		!naturalWildFillYield(NaturalMechanismV11) {
		t.Fatal("v11 does not carry the v10 placement with the wild-fill yield")
	}
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7, NaturalMechanismV8, NaturalMechanismV9, NaturalMechanismV10} {
		if naturalWildFillYield(m) {
			t.Fatalf("mechanism %d admitted the wild-fill yield", m)
		}
		if naturalSlotStockMaxFor(m) != 1 {
			t.Fatalf("mechanism %d widened the slot stock envelope", m)
		}
	}
	if naturalSlotStockMaxFor(NaturalMechanismV11) != NaturalYieldMax {
		t.Fatal("v11 slot stock envelope")
	}
}

// TestNaturalV11YieldOneIsV10 proves the neutral value: a v11 run at yield 1
// is v10 byte for byte (same checkpoint states, same ledger).
func TestNaturalV11YieldOneIsV10(t *testing.T) {
	ctx := context.Background()
	run := func(m NaturalMechanism) []NaturalCheckpoint {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 1, Seed: 0, Workers: 1, Founders: 10,
			PolicyRef: strategy.FrozenNaturalPolicy14().Ref, Mechanism: m})
		if err != nil {
			t.Fatalf("mechanism %d world: %v", m, err)
		}
		for i := 0; i < 4; i++ {
			if ok, err := f.Step(ctx); err != nil || !ok {
				t.Fatalf("mechanism %d step %d: %t %v", m, i, ok, err)
			}
		}
		return f.Checkpoints()
	}
	if a, b := run(NaturalMechanismV10), run(NaturalMechanismV11); !reflect.DeepEqual(a, b) {
		t.Fatal("v11 at yield 1 is not v10")
	}
}

// TestNaturalV11RegenSplitScalesWildFill proves the multiplication site: a
// vacant wild slot consumes one R budget unit but stands the declared yield,
// and the produced ledger counts those food units. The frozen split and v9
// stay one wild unit per slot.
func TestNaturalV11RegenSplitScalesWildFill(t *testing.T) {
	// Two cultivated grants and two wild fills already recorded at yield 2.
	patch := NaturalPatchState{Yield: 8, Pulses: 1, Produced: 4, Granted: 4, Unrealized: 4}
	slots := [NaturalSlotsPerPatch]NaturalSlotState{}
	for i := 0; i < 3; i++ {
		slots[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: sim.EntityID(i + 1), Cultivated: 1}
	}
	slots[3] = NaturalSlotState{Stock: 2}
	slots[4] = NaturalSlotState{Stock: 2}
	alive := NaturalAliveLookup(func(sim.EntityID) bool { return true })

	next, stock, result, grants, err := NaturalRegenSplitFor(NaturalMechanismV11, 2, 1, 1, patch, slots, alive)
	if err != nil {
		t.Fatalf("v11 regen split: %v", err)
	}
	if result.Cultivated != 6 || result.Wild != 6 || result.Unrealized != 2 {
		t.Fatalf("v11 split %+v, want cultivated 6, wild 6, unrealized 2", result)
	}
	if next.Granted != 10 || next.Produced != 10 || next.Unrealized != 6 {
		t.Fatalf("v11 patch ledger %+v", next)
	}
	for i := 5; i < NaturalSlotsPerPatch; i++ {
		if stock[i].Stock != 2 {
			t.Fatalf("v11 wild slot %d stands %d units, want the declared yield 2", i, stock[i].Stock)
		}
	}
	if len(grants) != 3 || grants[0].Units != 2 {
		t.Fatalf("v11 grants %+v, want two units per cultivated slot", grants)
	}
	// The same call under v10 keeps the frozen one-unit wild fill, so the
	// v11-shaped start is not even a legal v10 ledger: the wild units are
	// counted one per slot there.
	frozenPatch := NaturalPatchState{Yield: 8, Pulses: 1, Produced: 2, Granted: 2, Unrealized: 5}
	frozenSlots := slots
	frozenSlots[3] = NaturalSlotState{Stock: 1}
	frozenSlots[4] = NaturalSlotState{Stock: 1}
	frozenNext, _, frozenResult, _, err := NaturalRegenSplitFor(NaturalMechanismV10, 2, 1, 1, frozenPatch, frozenSlots, alive)
	if err != nil {
		t.Fatalf("v10 regen split: %v", err)
	}
	if frozenResult.Wild != 3 || frozenNext.Produced != 5 {
		t.Fatalf("v10 wild fill changed: %+v %+v", frozenResult, frozenNext)
	}
}

// TestNaturalV11GatherFromDeepSlot proves one gather still moves exactly one
// unit out of a wild slot standing the declared yield, while the frozen
// mechanisms admit only the one-unit slot.
func TestNaturalV11GatherFromDeepSlot(t *testing.T) {
	patches, patchID, slotID, actor := 1, sim.EntityID(NaturalPatchIDBase), sim.EntityID(NaturalSlotIDBase), sim.EntityID(1)
	a, err := NaturalFounderStateFor(NaturalMechanismV11, patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	if !validNaturalActorFor(NaturalMechanismV11, patches, actor, a) {
		t.Fatal("v11 gather fixture actor invalid")
	}
	deep := NaturalSlotState{Stock: 3}
	nextSlot, next, err := NaturalGatherFor(NaturalMechanismV11, patches, patchID, slotID, actor, 0, deep, a)
	if err != nil {
		t.Fatalf("v11 gather from a deep wild slot: %v", err)
	}
	if nextSlot.Stock != 2 || nextSlot.Gathered != 1 || next.Bag.Units != 1 {
		t.Fatalf("v11 gather %+v / bag %d, want one unit out of three", nextSlot, next.Bag.Units)
	}
	if _, _, err := NaturalGatherFor(NaturalMechanismV10, patches, patchID, slotID, actor, 0, deep, a); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("v10 gather admitted a three-unit standing slot: %v", err)
	}
	// The actor-visible hint stays the declared stocked flag.
	if n, err := naturalSlotStockHint(sim.IntegerValue(3)).Integer(); err != nil || n != 1 {
		t.Fatalf("slot stock hint %d/%v, want the stocked flag 1", n, err)
	}
	if n, err := naturalSlotStockHint(sim.IntegerValue(0)).Integer(); err != nil || n != 0 {
		t.Fatalf("empty slot stock hint %d/%v, want 0", n, err)
	}
}

// TestNaturalV11CheckpointIdentity proves the v11 mechanism plus declared
// yield is run identity end to end: the bundle round-trips, a mismatched
// yield or mechanism fails closed, and natural/14 stays the legal policy.
func TestNaturalV11CheckpointIdentity(t *testing.T) {
	ctx := context.Background()
	ref14 := strategy.FrozenNaturalPolicy14().Ref
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 3, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy13().Ref, Mechanism: NaturalMechanismV11}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("natural/13 accepted under v11: %v", err)
	}
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 3, Seed: 0, Workers: 1, Founders: 10,
		PolicyRef: ref14, Mechanism: NaturalMechanismV11})
	if err != nil {
		t.Fatalf("v11/natural/14 world: %v", err)
	}
	if ok, err := f.Step(ctx); err != nil || !ok {
		t.Fatalf("v11 h0 pulse: %t %v", ok, err)
	}
	path := t.TempDir() + "/v11.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatalf("v11 save: %v", err)
	}
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 3, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV11})
	if err != nil {
		t.Fatalf("v11 restore: %v", err)
	}
	if restored.mechanism != NaturalMechanismV11 || restored.yield != 3 || restored.ref != ref14 {
		t.Fatalf("restored v11 identity %d/%d/%+v", restored.mechanism, restored.yield, restored.ref)
	}
	if _, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 2, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV11}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v11 yield-2 restore accepted a yield-3 bundle: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 3, Seed: 0,
		Workers: 1, Founders: 10, PolicyRef: ref14, Mechanism: NaturalMechanismV10}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v10 restore accepted a v11 bundle: %v", err)
	}
}
