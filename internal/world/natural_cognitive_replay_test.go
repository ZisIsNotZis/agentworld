package world

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/strategy"
)

// The seam-active replay tests (ADR 0006 section 7.3, VS1 EC-3). A recorded
// Role A seam resolves every escalated claim instant from the recorded-call
// ledger and never calls a model, so a seam-active run's journal reconciles
// and its checkpoint restores exactly.

func seamReplayTestRunner(t *testing.T) (*Natural, *cognition.Ledger, *cognition.FakeClient, string) {
	t.Helper()
	ledger, err := cognition.CreateLedger(filepath.Join(t.TempDir(), "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	const body = `{"schema_version":1,"action":"wait","rationale":"replay-test","confidence":1}`
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		return cognition.ModelResponse{
			Body: []byte(body), Content: []byte(body),
			Usage: cognition.Usage{PromptTokens: 4, CompletionTokens: 2}, Status: cognition.CallStatusOK,
		}, nil
	}}
	account, err := cognition.NewCognitiveAccount(cognition.CognitiveBudget{
		MaxCalls: 1_000_000, MaxPromptTokens: 1_000_000_000,
		MaxCompletionTokens: 1_000_000_000, MaxCostMicros: 1_000_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: ledger, Client: fake, Budget: account,
		Price: cognition.PriceTable{Version: "test/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2},
		Now:   func() time.Time { return time.UnixMicro(1) },
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: recorder, Ledger: ledger, Endpoint: "gw.example/v1/chat/completions",
		WakePolicy: cognition.VS1WakePolicy(),
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: cognition.VS1WakePolicy()}
	f, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 24)
	return f, ledger, fake, ledger.Path()
}

func TestNaturalSeamReplayJournalAndCheckpoint(t *testing.T) {
	f, _, fake, ledgerPath := seamReplayTestRunner(t)
	liveCalls := fake.Calls()
	if liveCalls == 0 {
		t.Fatal("the live seam made no call")
	}

	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replay.NetworkClient().Complete(context.Background(), cognition.ModelRequest{
		Model: cognition.DefaultModelID, Endpoint: "sentinel", Request: []byte("{}"),
	}); !errors.Is(err, cognition.ErrReplayNetworkForbidden) {
		t.Fatalf("replay transport is not network-forbidden: %v", err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, cognition.VS1WakePolicy())
	if err != nil {
		t.Fatal(err)
	}

	// ExportJournal reconciliation reproduces the seam-active journal without a
	// model call.
	if _, err := f.ExportJournalWithReplay(replaySeam, cognition.VS1WakePolicy()); err != nil {
		t.Fatalf("seam-active ExportJournalWithReplay: %v", err)
	}

	// Save + restore reproduces the run exactly, again without a model call.
	bundle := filepath.Join(t.TempDir(), "seam.bundle")
	if _, err := f.SaveResumeCheckpointWithReplay(bundle, replaySeam, cognition.VS1WakePolicy()); err != nil {
		t.Fatalf("SaveResumeCheckpointWithReplay: %v", err)
	}
	if fake.Calls() != liveCalls {
		t.Fatalf("replay made %d model calls", fake.Calls()-liveCalls)
	}
	restoreOpts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
		CognitiveReplaySeam: replaySeam, CognitiveReplayWake: cognition.VS1WakePolicy()}
	restored, _, err := RestoreNaturalCheckpoint(bundle, restoreOpts)
	if err != nil {
		t.Fatalf("seam-active restore: %v", err)
	}
	if !reflect.DeepEqual(f.Checkpoints(), restored.Checkpoints()) {
		t.Fatal("restored seam-active run differs from the live run")
	}
	if replaySeam.ServedCalls() == 0 {
		t.Fatal("the replay seam resolved no recorded decision")
	}
	if fake.Calls() != liveCalls {
		t.Fatalf("restore replay made %d model calls", fake.Calls()-liveCalls)
	}
}

// TestNaturalSeamOnlyCheckpointBindsLedger pins ADR 0006 6.1 / spec 7.2 for a
// seam-only lane: a runner with a cognitive seam but no active generation still
// writes a manifest cognition block binding the recorded-call ledger, and the
// restore refuses a missing ledger. Reverting the block condition to require an
// active generation, or the restore check to opt-in, fails this test.
func TestNaturalSeamOnlyCheckpointBindsLedger(t *testing.T) {
	f, ledger, _, ledgerPath := seamReplayTestRunner(t)
	if _, ok := f.ActiveCognitiveGeneration(); ok {
		t.Fatal("the seam-only runner unexpectedly carries a generation")
	}
	summary, err := ledger.Summary()
	if err != nil {
		t.Fatal(err)
	}
	f.SetCognitiveCheckpoint(&NaturalCognitiveCheckpoint{WakePolicy: cognition.VS1WakePolicy(), LedgerDigest: summary.Digest, LedgerCount: summary.Count})
	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, cognition.VS1WakePolicy())
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "seam-only.bundle")
	if _, err := f.SaveResumeCheckpointWithReplay(bundle, replaySeam, cognition.VS1WakePolicy()); err != nil {
		t.Fatal(err)
	}
	_, _, m, err := naturalCheckedSections(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if m.cognitive == nil {
		t.Fatal("a seam-only bundle wrote no manifest cognition block")
	}
	if m.cognitive.LedgerCount != summary.Count || m.cognitive.LedgerDigest != summary.Digest {
		t.Fatalf("seam-only block bound ledger count=%d digest mismatch=%t", m.cognitive.LedgerCount, m.cognitive.LedgerDigest != summary.Digest)
	}
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
		CognitiveReplaySeam: replaySeam, CognitiveReplayWake: cognition.VS1WakePolicy()}
	if _, _, err := RestoreNaturalCheckpoint(bundle, opts); err == nil {
		t.Fatal("a seam-only bundle restored without the recorded-call ledger")
	}
	opts.CognitiveLedgerPath = ledgerPath
	if _, _, err := RestoreNaturalCheckpoint(bundle, opts); err != nil {
		t.Fatalf("seam-only restore with the ledger: %v", err)
	}
}

// TestNaturalSeamRestoreWithoutReplayRefused pins the guard: without the
// recorded-seam replay the restore of a seam-active bundle fails closed
// (deleting the ledger-resolution path fails this test).
func TestNaturalSeamRestoreWithoutReplayRefused(t *testing.T) {
	f, _, _, ledgerPath := seamReplayTestRunner(t)
	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, cognition.VS1WakePolicy())
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "seam.bundle")
	if _, err := f.SaveResumeCheckpointWithReplay(bundle, replaySeam, cognition.VS1WakePolicy()); err != nil {
		t.Fatal(err)
	}
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10}
	if _, _, err := RestoreNaturalCheckpoint(bundle, opts); err == nil {
		t.Fatal("a seam-active bundle restored without the recorded-seam replay")
	}
}

// TestNaturalCognitiveCheckpointDivergingJournalRoundTrip is the live-path
// guard for the recorded-seam save: a generation whose parameter change
// actually alters the recorded decisions must still reconcile, save, and
// restore. Removing the cognitive context from the journal encode/decode makes
// this fail (the reference replay falls back to the frozen base).
func TestNaturalCognitiveCheckpointDivergingJournalRoundTrip(t *testing.T) {
	frozen, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, frozen, 30)

	f, err := NewNatural(naturalCognitionTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 24)
	params := strategy.FrozenCognitiveParams()
	params.BagBuildEnergyFloor = 8
	params.BagBuildStartEnergyFloor = 8
	params.BagBuildFinishEnergyFloor = 8
	gen, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy7().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, params)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ActivateCognitiveGeneration(gen, liveActors(t, f)); err != nil {
		t.Fatal(err)
	}
	f.SetCognitiveCheckpoint(&NaturalCognitiveCheckpoint{WakePolicy: cognition.VS1WakePolicy(), LedgerDigest: sha256.Sum256(nil), ApplyHour: 24})
	naturalRunTo(t, f, 30)
	if reflect.DeepEqual(frozen.Checkpoints(), f.Checkpoints()) {
		t.Fatal("the generation did not change the journal; the round trip would be vacuous")
	}

	ledgerPath, _ := testCognitiveLedger(t)
	bundle := filepath.Join(t.TempDir(), "cog-diverging.bundle")
	if _, err := f.SaveResumeCheckpoint(bundle); err != nil {
		t.Fatalf("save of a generation-diverged journal: %v", err)
	}
	restored, _, err := RestoreNaturalCheckpoint(bundle, NaturalRestoreOptions{
		Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
		CognitiveLedgerPath: ledgerPath,
	})
	if err != nil {
		t.Fatalf("restore of a generation-diverged journal: %v", err)
	}
	if !reflect.DeepEqual(f.Checkpoints(), restored.Checkpoints()) {
		t.Fatal("restored diverging-generation run differs from its origin")
	}
}
