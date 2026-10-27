package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// TestNaturalV10RoundRobinFounderPlacement proves mechanism v10's one
// placement change: founder i lives on patch (i-1) mod P, while every earlier
// mechanism keeps the frozen ID-block placement byte-identically.
func TestNaturalV10RoundRobinFounderPlacement(t *testing.T) {
	if !NaturalMechanismValid(NaturalMechanismV10) || NaturalMechanismV10.worldVersion() != 10 {
		t.Fatal("mechanism v10 is not admitted as world version 10")
	}
	patches := 2
	for i := 1; i <= 16; i++ {
		actor := sim.EntityID(i)
		blocked, err := NaturalFounderPatchID(patches, actor)
		if err != nil {
			t.Fatal(err)
		}
		wantBlocked := sim.EntityID(NaturalPatchIDBase + (i-1)/NaturalSlotsPerPatch)
		if blocked != wantBlocked {
			t.Fatalf("founder %d blocked home %d, want %d", i, blocked, wantBlocked)
		}
		rotated, err := NaturalFounderPatchIDFor(NaturalMechanismV10, patches, actor)
		if err != nil {
			t.Fatal(err)
		}
		wantRotated := sim.EntityID(NaturalPatchIDBase + (i-1)%patches)
		if rotated != wantRotated {
			t.Fatalf("founder %d v10 home %d, want %d", i, rotated, wantRotated)
		}
	}
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7, NaturalMechanismV8, NaturalMechanismV9} {
		got, err := NaturalFounderPatchIDFor(m, patches, 10)
		if err != nil || got != sim.EntityID(NaturalPatchIDBase+1) {
			t.Fatalf("mechanism %d founder-10 home %d (%v), want the blocked patch 1002", m, got, err)
		}
	}
	// The runner materializes the round-robin band and conserves at genesis.
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1, Founders: 10,
		PolicyRef: strategy.FrozenNaturalPolicy14().Ref, Mechanism: NaturalMechanismV10})
	if err != nil {
		t.Fatalf("v10/natural/14 world: %v", err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatalf("v10 h0 pulse: %t %v", ok, err)
	}
	wantHomes := []sim.EntityID{1001, 1002, 1001, 1002, 1001, 1002, 1001, 1002, 1001, 1002}
	for i, a := range f.checkpoints[0].Founders {
		if a.Bag.Source != wantHomes[i] {
			t.Fatalf("founder %d home %d, want %d", i+1, a.Bag.Source, wantHomes[i])
		}
	}
}

// TestNaturalV10PolicyAndIdentity proves natural/14 is legal under v10 only,
// natural/13 stays legal under v8/v9 only, and the v10 checkpoint round-trips
// under its declared yield.
func TestNaturalV10PolicyAndIdentity(t *testing.T) {
	ref13, ref14 := strategy.FrozenNaturalPolicy13().Ref, strategy.FrozenNaturalPolicy14().Ref
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1,
		PolicyRef: ref14, Mechanism: NaturalMechanismV9}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("natural/14 accepted under v9: %v", err)
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1,
		PolicyRef: ref13, Mechanism: NaturalMechanismV10}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("natural/13 accepted under v10: %v", err)
	}
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Yield: 2, Seed: 0, Workers: 1,
		PolicyRef: ref14, Mechanism: NaturalMechanismV10})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if ok, err := f.Step(ctx); err != nil || !ok {
		t.Fatalf("v10 h0 pulse: %t %v", ok, err)
	}
	path := t.TempDir() + "/v10.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatalf("v10 save: %v", err)
	}
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 2, Seed: 0,
		Workers: 1, PolicyRef: ref14, Mechanism: NaturalMechanismV10})
	if err != nil {
		t.Fatalf("v10 restore: %v", err)
	}
	if restored.mechanism != NaturalMechanismV10 || restored.yield != 2 || restored.ref != ref14 {
		t.Fatalf("restored v10 identity %d/%d/%+v", restored.mechanism, restored.yield, restored.ref)
	}
	if _, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Yield: 3, Seed: 0,
		Workers: 1, PolicyRef: ref14, Mechanism: NaturalMechanismV10}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v10 yield-3 restore accepted a yield-2 bundle: %v", err)
	}
}
