package world

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"agentworld/internal/strategy"
)

// The ticket-16 experiment-lane fixture seam (supervisor-approved scoped
// exception recorded in .scratch/16-surplus-behaviour/spec.md): the exported
// ref-carrying founder-tenure fixture constructor. These tests pin exactly
// the authorized surface: the natural/5 ref is default-preserving
// (byte-identical continuation bundles against NewNaturalFixture), the
// natural/6 ref produces a fixture whose run identity carries ref 6 with the
// existing bidirectional cross-ref rejection, and unknown or zero refs fail
// closed with a typed error before any runner exists.

// TestNaturalFixtureForPolicyV5DefaultPreserving proves the wrapper at the
// natural/5 ref is exactly the legacy constructor: identical checkpoints and
// a digest-identical published continuation bundle.
func TestNaturalFixtureForPolicyV5DefaultPreserving(t *testing.T) {
	const seed = uint64(5)
	legacy, err := NewNaturalFixture(2, 8, seed, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := NewNaturalFixtureForPolicy(2, 8, seed, true, 1, strategy.FrozenNaturalPolicy().Ref)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, legacy, 3)
	naturalStepToBoundary(t, wrapped, 3)
	if !reflect.DeepEqual(legacy.Checkpoints(), wrapped.Checkpoints()) {
		t.Fatal("wrapped v5 fixture diverged from the legacy constructor")
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy-fixture.bundle")
	legacyDigest, err := legacy.SaveResumeCheckpoint(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	wrappedPath := filepath.Join(t.TempDir(), "wrapped-fixture.bundle")
	wrappedDigest, err := wrapped.SaveResumeCheckpoint(wrappedPath)
	if err != nil {
		t.Fatal(err)
	}
	if legacyDigest != wrappedDigest {
		t.Fatalf("v5 wrapper is not default-preserving: digest %x vs %x", legacyDigest, wrappedDigest)
	}
}

// TestNaturalFixtureForMechanismV5DefaultPreserving proves the ticket-17
// mechanism-aware wrapper (supervisor-approved scoped extension recorded in
// .scratch/17-surplus-mechanism/spec.md) at the natural/5 + v5 combination is
// exactly the legacy constructor: identical checkpoints and a digest-identical
// published continuation bundle.
func TestNaturalFixtureForMechanismV5DefaultPreserving(t *testing.T) {
	const seed = uint64(5)
	legacy, err := NewNaturalFixture(2, 8, seed, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := NewNaturalFixtureForMechanism(2, 8, seed, true, 1, strategy.FrozenNaturalPolicy().Ref, NaturalMechanismV5)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, legacy, 3)
	naturalStepToBoundary(t, wrapped, 3)
	if !reflect.DeepEqual(legacy.Checkpoints(), wrapped.Checkpoints()) {
		t.Fatal("mechanism-aware v5 fixture diverged from the legacy constructor")
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy-fixture.bundle")
	legacyDigest, err := legacy.SaveResumeCheckpoint(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	wrappedPath := filepath.Join(t.TempDir(), "wrapped-fixture.bundle")
	wrappedDigest, err := wrapped.SaveResumeCheckpoint(wrappedPath)
	if err != nil {
		t.Fatal(err)
	}
	if legacyDigest != wrappedDigest {
		t.Fatalf("mechanism-aware v5 wrapper is not default-preserving: digest %x vs %x", legacyDigest, wrappedDigest)
	}
}

// TestNaturalFixtureForMechanismFailClosed pins the fail-closed surface of
// the mechanism-aware wrapper: the provision policies natural/7 and 7b are
// refused under the v5 mechanism (the affordance they select does not exist
// there), and unknown mechanism versions are refused before any runner
// exists.
func TestNaturalFixtureForMechanismFailClosed(t *testing.T) {
	for _, tc := range []struct {
		ref       strategy.NaturalRef
		mechanism NaturalMechanism
	}{
		{strategy.FrozenNaturalPolicy7().Ref, NaturalMechanismV5},
		{strategy.FrozenNaturalPolicy7b().Ref, NaturalMechanismV5},
		{strategy.FrozenNaturalPolicy().Ref, NaturalMechanism(5)},
		{strategy.FrozenNaturalPolicy().Ref, NaturalMechanism(7)},
	} {
		if _, err := NewNaturalFixtureForMechanism(2, 8, 5, true, 1, tc.ref, tc.mechanism); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("fixture for ref %+v mechanism %d accepted: %v", tc.ref, tc.mechanism, err)
		}
	}
}

// TestNaturalFixtureForPolicyV6RunIdentity proves the natural/6 fixture
// carries ref 6 end to end: it restores only under the declared ref, a
// natural/5 expectation (the zero ref included) is bidirectionally refused,
// and the legacy constructor's bundle is likewise refused under a v6
// expectation.
func TestNaturalFixtureForPolicyV6RunIdentity(t *testing.T) {
	const seed = uint64(5)
	six, err := NewNaturalFixtureForPolicy(2, 8, seed, true, 1, strategy.FrozenNaturalPolicyV6().Ref)
	if err != nil {
		t.Fatal(err)
	}
	if six.ref != strategy.FrozenNaturalPolicyV6().Ref {
		t.Fatalf("v6 fixture bound %+v", six.ref)
	}
	naturalStepToBoundary(t, six, 3)
	path := filepath.Join(t.TempDir(), "v6-fixture.bundle")
	digest, err := six.SaveResumeCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, got, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed,
		Workers: 4, Births: true, FounderTenure: true, PolicyRef: strategy.FrozenNaturalPolicyV6().Ref})
	if err != nil || got != digest {
		t.Fatalf("v6 fixture restore under its own ref: digest=%x want=%x err=%v", got, digest, err)
	}
	if restored.ref != strategy.FrozenNaturalPolicyV6().Ref {
		t.Fatalf("restored fixture bound %+v", restored.ref)
	}
	for _, name := range []string{"zero-ref", "explicit-v5-ref"} {
		expected := NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: true, FounderTenure: true}
		if name == "explicit-v5-ref" {
			expected.PolicyRef = strategy.FrozenNaturalPolicy().Ref
		}
		if _, _, err := RestoreNaturalCheckpoint(path, expected); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("%s: v6 fixture restored under %s: %v", name, name, err)
		}
	}
	// The direction is bidirectional: a legacy v5 fixture bundle is refused
	// under a v6 expectation.
	legacy, err := NewNaturalFixture(2, 8, seed, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, legacy, 3)
	legacyPath := filepath.Join(t.TempDir(), "v5-fixture.bundle")
	if _, err := legacy.SaveResumeCheckpoint(legacyPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreNaturalCheckpoint(legacyPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed,
		Workers: 1, Births: true, FounderTenure: true, PolicyRef: strategy.FrozenNaturalPolicyV6().Ref}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v5 fixture restored under a v6 expectation: %v", err)
	}
}

// TestNaturalFixtureForPolicyRefusals proves the fail-closed surface: the
// zero ref (use NewNaturalFixture for the default) and refs outside the
// frozen natural ref set are refused with the typed checkpoint error before
// any runner is constructed.
func TestNaturalFixtureForPolicyRefusals(t *testing.T) {
	for _, ref := range []strategy.NaturalRef{
		{},                          // the zero ref is not silently natural/5 here
		{ID: "natural", Version: 9}, // outside the extended ref set {5,6,7,8}
		{ID: "social", Version: 5},
	} {
		if _, err := NewNaturalFixtureForPolicy(2, 8, 5, true, 1, ref); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("ref %+v accepted: %v", ref, err)
		}
	}
}
