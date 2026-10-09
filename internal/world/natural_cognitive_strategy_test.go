package world

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// testCognitiveLedger writes an empty recorded-call ledger and returns its path
// and whole-file digest, so a cognitive checkpoint binding can be attached and
// the restore's ledger requirement satisfied.
func testCognitiveLedger(t *testing.T) (string, [32]byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, sha256.Sum256(nil)
}

// The world-side Role B tests (spec sections 5.4 and 7.2): the active
// generation changes the run, clearing it restores the frozen path, and the
// checkpoint cognition block round-trips the generation and its bindings.

func divergentCognitiveGeneration(t *testing.T) strategy.NaturalCognitiveGeneration {
	t.Helper()
	params := strategy.FrozenCognitiveParams()
	params.ClaimGatherEnergyFloor = 8
	params.ClaimBankEnergyCeiling = 0
	params.ClaimProvisionEnergyFloor = 8
	gen, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy7().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, params)
	if err != nil {
		t.Fatal(err)
	}
	return gen
}

func liveActors(t *testing.T, f *Natural) []sim.EntityID {
	t.Helper()
	actors, err := NewNaturalCognitiveApplier(f).RosterActors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(actors) == 0 {
		t.Fatal("no living actors")
	}
	return actors
}

// TestNaturalCognitiveDispatchDiverges proves the active generation is
// load-bearing: the same fixture diverges from the frozen run, and clearing the
// generation before any step leaves the frozen path intact.
func TestNaturalCognitiveDispatchDiverges(t *testing.T) {
	frozen, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, frozen, 24)

	cognitive, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, cognitive, 24)
	if err := cognitive.ActivateCognitiveGeneration(divergentCognitiveGeneration(t), liveActors(t, cognitive)); err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, cognitive, 30)
	naturalRunTo(t, frozen, 30)
	if reflect.DeepEqual(frozen.Checkpoints(), cognitive.Checkpoints()) {
		t.Fatal("the active generation did not diverge from the frozen run")
	}

	// A generation activated and cleared before any step leaves the frozen
	// path byte-identical.
	cleared, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, cleared, 24)
	if err := cleared.ActivateCognitiveGeneration(divergentCognitiveGeneration(t), liveActors(t, cleared)); err != nil {
		t.Fatal(err)
	}
	if err := cleared.ClearCognitiveGeneration(); err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, cleared, 30)
	if !reflect.DeepEqual(frozen.Checkpoints(), cleared.Checkpoints()) {
		t.Fatal("clearing the generation did not restore the frozen path")
	}
}

// TestNaturalCognitiveActivationGuards pins that a rebind refuses a non-roster
// actor before mutating anything.
func TestNaturalCognitiveActivationGuards(t *testing.T) {
	f, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 2)
	if err := f.ActivateCognitiveGeneration(divergentCognitiveGeneration(t), nil); err == nil {
		t.Fatal("empty actor set accepted")
	}
	if err := f.ActivateCognitiveGeneration(divergentCognitiveGeneration(t), []sim.EntityID{999999}); err == nil {
		t.Fatal("non-roster actor accepted")
	}
	if f.CognitiveActorBound(1) {
		t.Fatal("a refused rebind mutated the runner")
	}
}

// TestNaturalCognitiveCheckpointRoundTrip pins the checkpoint cognition block:
// a cognitive resume bundle restores the generation and its bindings, and the
// journal replay reproduces the recorded decisions from the apply boundary.
func TestNaturalCognitiveCheckpointRoundTrip(t *testing.T) {
	f, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 2)
	gen := divergentCognitiveGeneration(t)
	if err := f.ActivateCognitiveGeneration(gen, liveActors(t, f)); err != nil {
		t.Fatal(err)
	}
	ledgerPath, ledgerDigest := testCognitiveLedger(t)
	f.SetCognitiveCheckpoint(&NaturalCognitiveCheckpoint{WakePolicy: protocol.VS1WakePolicy(), LedgerDigest: ledgerDigest, ApplyHour: 2})
	naturalRunTo(t, f, 4)

	path := t.TempDir() + "/cog.bundle"
	if _, err := f.SaveResumeCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{
		Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
		CognitiveLedgerPath: ledgerPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := restored.ActiveCognitiveGeneration()
	if !ok || got.Ref != gen.Ref {
		t.Fatalf("restored generation %+v ok %v, want %+v", got, ok, gen.Ref)
	}
	if !restored.CognitiveActorBound(1) {
		t.Fatal("restored binding table lost actor 1")
	}
	// The restored journal advances identically to the saved one.
	naturalRunTo(t, f, 6)
	naturalRunTo(t, restored, 6)
	if !reflect.DeepEqual(f.Checkpoints(), restored.Checkpoints()) {
		t.Fatal("a restored cognitive run diverged from its origin")
	}
}
