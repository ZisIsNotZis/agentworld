package world

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The VS2 lane L4b tests: the priority-bearing recorded-seam replay. A VS2 arm
// carries an arm-local protocol.EscalationSelector, so the checkpoint save, the
// journal export, and the restore must supply a genesis-state selector for each
// reference replay; otherwise the priority run is refused by the construction
// guard (VS2-EC-2) and can be neither checkpointed nor restored. The nil-Priority
// VS1 path must stay byte-identical.

// priorityReplayPolicy is the declared adaptive arm the round trip drives: a
// per-hour-quota adaptive selector over the full 168 h horizon.
func priorityReplayPolicy() protocol.WakePolicy {
	return protocol.WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0, Priority: &protocol.PriorityPolicy{
		Strategy: protocol.StrategyAdaptive, ExplorationFraction: 250000,
		RandSeed: 7, CallCeiling: 64,
	}}
}

// prioritySelectorStep is one recorded Select call: the hour, the selected
// actors, and the post-selection accumulator state (spent-call count and PRNG
// position), which is the recorded run state VS2-EC-1 replays from.
type prioritySelectorStep struct {
	hour     int
	selected []sim.EntityID
	spent    uint64
	prngPos  uint64
}

// recordingSelector delegates to a fresh cognition.PrioritySelector and records
// every selection, so the test can compare the live arm's selection sequence to
// each replay's.
type recordingSelector struct {
	inner *cognition.PrioritySelector
	mu    sync.Mutex
	steps []prioritySelectorStep
}

func newRecordingSelector(t *testing.T, policy protocol.WakePolicy) *recordingSelector {
	t.Helper()
	inner, err := cognition.NewPrioritySelector(policy, NaturalHorizonHours, protocol.CallPrice{})
	if err != nil {
		t.Fatal(err)
	}
	return &recordingSelector{inner: inner}
}

func (s *recordingSelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	selection, err := s.inner.Select(hour, roster, budget)
	if err != nil {
		return selection, err
	}
	s.mu.Lock()
	s.steps = append(s.steps, prioritySelectorStep{
		hour: hour, selected: append([]sim.EntityID(nil), selection.Selected...),
		spent: s.inner.CallsSpent(), prngPos: s.inner.PRNGPos(),
	})
	s.mu.Unlock()
	return selection, nil
}

func (s *recordingSelector) recorded() []prioritySelectorStep {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]prioritySelectorStep(nil), s.steps...)
}

// selectorFactory is the per-replay selector factory the world consumes; it
// records every genesis-state selector it builds for one entry point.
type selectorFactory struct {
	t      *testing.T
	policy protocol.WakePolicy
	mu     sync.Mutex
	made   []*recordingSelector
}

func newSelectorFactory(t *testing.T, policy protocol.WakePolicy) *selectorFactory {
	return &selectorFactory{t: t, policy: policy}
}

func (f *selectorFactory) new() protocol.EscalationSelector {
	s := newRecordingSelector(f.t, f.policy)
	f.mu.Lock()
	f.made = append(f.made, s)
	f.mu.Unlock()
	return s
}

func (f *selectorFactory) selectors() []*recordingSelector {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*recordingSelector(nil), f.made...)
}

// priorityReplayTestRunner builds a live priority-bearing seam run: the same
// recorded-seam fixture as seamReplayTestRunner, but wired with the declared
// priority wake policy and the arm-local selector.
func priorityReplayTestRunner(t *testing.T, policy protocol.WakePolicy, selector protocol.EscalationSelector) (*Natural, *cognition.Ledger, *cognition.FakeClient, string) {
	t.Helper()
	ledger, err := cognition.CreateLedger(filepath.Join(t.TempDir(), "calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	const body = `{"schema_version":1,"action":"wait","rationale":"priority-replay","confidence":1}`
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
		WakePolicy: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: policy, Selector: selector}
	f, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 24)
	return f, ledger, fake, ledger.Path()
}

// TestNaturalPrioritySeamCheckpointReplayRoundTrip is the L4b acceptance test:
// a priority-bearing run is exported, checkpointed, and restored, and every
// reference replay re-derives the arm-local selection sequence from recorded
// state with no model call. A missing selector wiring fails the construction
// guard; a selector reused across replays (rather than built fresh per replay)
// diverges at the second replay and fails this test.
func TestNaturalPrioritySeamCheckpointReplayRoundTrip(t *testing.T) {
	policy := priorityReplayPolicy()
	live := newRecordingSelector(t, policy)
	f, ledger, fake, ledgerPath := priorityReplayTestRunner(t, policy, live)

	liveSteps := live.recorded()
	if len(liveSteps) == 0 {
		t.Fatal("the priority selector was never consulted")
	}
	escalated := false
	for _, step := range liveSteps {
		if len(step.selected) > 0 {
			escalated = true
		}
	}
	if !escalated {
		t.Fatal("the priority arm selected no actor; the round trip would be vacuous")
	}
	liveCalls := fake.Calls()
	if liveCalls == 0 {
		t.Fatal("the live priority seam made no model call")
	}

	summary, err := ledger.Summary()
	if err != nil {
		t.Fatal(err)
	}
	f.SetCognitiveCheckpoint(&NaturalCognitiveCheckpoint{
		WakePolicy: policy, LedgerDigest: summary.Digest, LedgerCount: summary.Count,
	})

	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, policy)
	if err != nil {
		t.Fatal(err)
	}

	// Export reconciles the priority journal with a fresh selector per replay.
	exportFactory := newSelectorFactory(t, policy)
	if _, err := f.ExportJournalWithReplaySelector(replaySeam, policy, exportFactory.new); err != nil {
		t.Fatalf("priority ExportJournalWithReplaySelector: %v", err)
	}

	// Save + restore reproduces the priority run exactly, no model call.
	saveFactory := newSelectorFactory(t, policy)
	bundle := filepath.Join(t.TempDir(), "priority.bundle")
	if _, err := f.SaveResumeCheckpointWithReplaySelector(bundle, replaySeam, policy, saveFactory.new); err != nil {
		t.Fatalf("priority SaveResumeCheckpointWithReplaySelector: %v", err)
	}
	restoreFactory := newSelectorFactory(t, policy)
	restored, _, err := RestoreNaturalCheckpoint(bundle, NaturalRestoreOptions{
		Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 10,
		CognitiveReplaySeam: replaySeam, CognitiveReplayWake: policy,
		CognitiveReplaySelector: restoreFactory.new, CognitiveLedgerPath: ledgerPath,
	})
	if err != nil {
		t.Fatalf("priority restore: %v", err)
	}
	if !reflect.DeepEqual(f.Checkpoints(), restored.Checkpoints()) {
		t.Fatal("restored priority run differs from the live run")
	}
	if fake.Calls() != liveCalls {
		t.Fatalf("save/restore replay made %d model calls", fake.Calls()-liveCalls)
	}
	if replaySeam.ServedCalls() == 0 {
		t.Fatal("the replay seam resolved no recorded decision")
	}

	// Every replay selector reproduced the live selection sequence from
	// recorded state (same hours, same selected sets, same spent/PRNG state).
	for name, factory := range map[string]*selectorFactory{
		"export": exportFactory, "save": saveFactory, "restore": restoreFactory,
	} {
		selectors := factory.selectors()
		if len(selectors) == 0 {
			t.Fatalf("%s: no replay selector was built", name)
		}
		for i, sel := range selectors {
			if got := sel.recorded(); !reflect.DeepEqual(got, liveSteps) {
				t.Fatalf("%s: replay selector %d diverged from the live selection sequence", name, i)
			}
		}
	}
}

// TestNaturalNilPriorityReplaySelectorBytesIdentical pins the additive
// guarantee: on the nil-Priority VS1 path the selector-aware export/save
// produce byte-identical artefacts to the pre-L4b methods, so no recorded VS1
// bundle or journal changes.
func TestNaturalNilPriorityReplaySelectorBytesIdentical(t *testing.T) {
	f, _, fake, ledgerPath := seamReplayTestRunner(t)
	wake := cognition.VS1WakePolicy()
	replay, err := cognition.OpenReplay(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, wake)
	if err != nil {
		t.Fatal(err)
	}
	liveCalls := fake.Calls()

	legacy, err := f.ExportJournalWithReplay(replaySeam, wake)
	if err != nil {
		t.Fatal(err)
	}
	withNil, err := f.ExportJournalWithReplaySelector(replaySeam, wake, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacy, withNil) {
		t.Fatal("the nil selector option changed the exported journal bytes")
	}

	legacyDigest, err := f.SaveResumeCheckpointWithReplay(filepath.Join(t.TempDir(), "legacy.bundle"), replaySeam, wake)
	if err != nil {
		t.Fatal(err)
	}
	nilDigest, err := f.SaveResumeCheckpointWithReplaySelector(filepath.Join(t.TempDir(), "nil-selector.bundle"), replaySeam, wake, nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacyDigest != nilDigest {
		t.Fatal("the nil selector option changed the resume bundle digest")
	}
	if fake.Calls() != liveCalls {
		t.Fatalf("replay made %d model calls", fake.Calls()-liveCalls)
	}
}
