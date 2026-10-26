package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/strategy"
)

// TestNatural11SeamConstructsWorld proves {natural,11} joined the frozen ref
// set end to end at the world seam: it constructs under the same mechanisms as
// natural/7 (v6 and v7), is refused under the v5 zero value, and its checkpoint
// round-trips through the format-11 manifest slot.
func TestNatural11SeamConstructsWorld(t *testing.T) {
	ref := strategy.FrozenNaturalPolicy11().Ref
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV6, NaturalMechanismV7} {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: mechanism})
		if err != nil {
			t.Fatalf("natural/11 under mechanism %d refused: %v", mechanism, err)
		}
		if f.ref != ref || f.mechanism != mechanism {
			t.Fatalf("runner bound %+v/%d for natural/11 under %d", f.ref, f.mechanism, mechanism)
		}
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("natural/11 accepted under the v5 zero-value mechanism: %v", err)
	}

	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/natural11.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	run, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatalf("natural/11 checkpoint restore: %v", err)
	}
	if run.ref != ref || run.mechanism != NaturalMechanismV6 {
		t.Fatalf("restored runner bound %+v/%d, want natural/11 under v6", run.ref, run.mechanism)
	}
}
