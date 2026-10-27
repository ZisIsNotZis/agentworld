package world

import (
	"context"
	"errors"
	"testing"

	"agentworld/internal/strategy"
)

// TestNatural13SeamConstructsWorld proves {natural,13} joined the frozen ref
// set end to end at the world seam: it constructs under mechanism v8 only, is
// refused under the v5/v6/v7 mechanisms, and its checkpoint round-trips through
// the format-13 manifest slot.
func TestNatural13SeamConstructsWorld(t *testing.T) {
	ref := strategy.FrozenNaturalPolicy13().Ref
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7} {
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("natural/13 accepted under mechanism %d: %v", mechanism, err)
		}
	}
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV8})
	if err != nil {
		t.Fatalf("natural/13 under mechanism v8 refused: %v", err)
	}
	if f.ref != ref || f.mechanism != NaturalMechanismV8 {
		t.Fatalf("runner bound %+v/%d for natural/13 under v8", f.ref, f.mechanism)
	}
	if _, err := f.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/natural13.bundle"
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	run, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV8})
	if err != nil {
		t.Fatalf("natural/13 checkpoint restore: %v", err)
	}
	if run.ref != ref || run.mechanism != NaturalMechanismV8 {
		t.Fatalf("restored runner bound %+v/%d, want natural/13 under v8", run.ref, run.mechanism)
	}
}
