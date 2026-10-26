package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/strategy"
)

// TestNatural12SeamConstructsWorld proves {natural,12} joined the frozen ref
// set end to end at the world seam: it constructs under the same mechanisms as
// natural/7 (v6 and v7), is refused under the v5 zero value, and its checkpoint
// round-trips through the format-12 manifest slot.
func TestNatural12SeamConstructsWorld(t *testing.T) {
	ref := strategy.FrozenNaturalPolicy12().Ref
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV6, NaturalMechanismV7} {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: mechanism})
		if err != nil {
			t.Fatalf("natural/12 under mechanism %d refused: %v", mechanism, err)
		}
		if f.ref != ref || f.mechanism != mechanism {
			t.Fatalf("runner bound %+v/%d for natural/12 under %d", f.ref, f.mechanism, mechanism)
		}
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("natural/12 accepted under the v5 zero-value mechanism: %v", err)
	}

	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/natural12.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	run, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatalf("natural/12 checkpoint restore: %v", err)
	}
	if run.ref != ref || run.mechanism != NaturalMechanismV6 {
		t.Fatalf("restored runner bound %+v/%d, want natural/12 under v6", run.ref, run.mechanism)
	}
}
