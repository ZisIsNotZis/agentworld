package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// The hermetic end-to-end tests: a bounded matrix produces a complete report, a
// priority arm round-trips through checkpoint/restore/replay with no drift, and
// the live spend gate refuses by default.

const testHorizon = 8

func testFixture(f int, seed uint64) fixture {
	return fixture{Patches: 2, R: 8, Founders: f, Seed: seed, Hours: testHorizon, Workers: 1}
}

// runTestMatrix runs the five declared arms over one bounded cell.
func runTestMatrix(t *testing.T) []*cellRun {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	var runs []*cellRun
	for _, arm := range declaredArms {
		spec := cellSpec{Arm: arm, F: 10, Seed: 0}
		cr, err := runCell(ctx, spec, testFixture(10, 0), root, nil)
		if err != nil {
			t.Fatalf("%s: %v", arm, err)
		}
		runs = append(runs, cr)
	}
	sortRuns(runs)
	return runs
}

func TestHermeticEndToEndReport(t *testing.T) {
	runs := runTestMatrix(t)
	replay := runReplaySubset(context.Background(), runs, t.TempDir(), 2)
	declared := []string{}
	for _, arm := range declaredArms {
		declared = append(declared, arm+"-f10-s0")
	}
	ev := buildEvaluation(runs, t.TempDir(), nil, declared, replay, filepath.Join("..", ".."), testHorizon)

	if ev.ModelID != cognition.DefaultModelID || ev.WireMode != string(cognition.WireResponses) {
		t.Fatalf("revision binding model=%q wire=%q", ev.ModelID, ev.WireMode)
	}
	if ev.ModelIdentity == "" || len(ev.ModelIdentity) != 64 {
		t.Fatalf("model identity digest = %q", ev.ModelIdentity)
	}
	if len(ev.Cells) != len(declaredArms) {
		t.Fatalf("cells = %d, want %d", len(ev.Cells), len(declaredArms))
	}
	// Every budgeted arm reports a realized count and the under-budget flag.
	for _, c := range ev.Cells {
		if !isBudgeted(c.Arm) {
			continue
		}
		if c.CallCeiling != DeclaredCallCeiling {
			t.Fatalf("%s ceiling = %d", c.Arm, c.CallCeiling)
		}
		if c.RealizedCalls > DeclaredCallCeiling {
			t.Fatalf("%s realized %d exceeds N", c.Arm, c.RealizedCalls)
		}
		if !c.UnderBudget {
			t.Fatalf("%s should be under budget at a %d-hour horizon", c.Arm, testHorizon)
		}
		if c.BudgetShortfall != DeclaredCallCeiling-c.RealizedCalls {
			t.Fatalf("%s shortfall %d inconsistent with realized %d", c.Arm, c.BudgetShortfall, c.RealizedCalls)
		}
	}
	// Three divergence rows, each carrying all eight indicators.
	if len(ev.Fidelity) != len(budgetedArms) {
		t.Fatalf("fidelity rows = %d, want %d", len(ev.Fidelity), len(budgetedArms))
	}
	for _, row := range ev.Fidelity {
		if len(row.Indicators) != 8 {
			t.Fatalf("%s indicators = %d, want 8", row.Arm, len(row.Indicators))
		}
		for _, ind := range row.Indicators {
			if ind.NormalizedFirstDivergenceHour < 1 || ind.NormalizedFirstDivergenceHour > testHorizon+1 {
				t.Fatalf("%s %s normalized first = %d", row.Arm, ind.Name, ind.NormalizedFirstDivergenceHour)
			}
		}
	}
	// All nine VS2 exit-criteria bands are present.
	wantBands := map[string]bool{}
	for i := 1; i <= 9; i++ {
		wantBands["VS2-EC-"+itoa(i)] = false
	}
	for _, b := range ev.Bands {
		if _, ok := wantBands[b.ID]; ok {
			wantBands[b.ID] = true
		}
	}
	for id, seen := range wantBands {
		if !seen {
			t.Fatalf("band %s missing", id)
		}
	}
	// EC-4 (vector completeness), EC-7 (exploration), EC-8 (value model) and
	// EC-9 (labels) must hold on this bounded matrix.
	for _, b := range ev.Bands {
		switch b.ID {
		case "VS2-EC-4", "VS2-EC-6", "VS2-EC-7", "VS2-EC-8", "VS2-EC-9":
			if b.Status != "PASS" {
				t.Fatalf("%s status %q: %s", b.ID, b.Status, b.Evidence)
			}
		}
	}
	// The report round-trips through JSON.
	out := filepath.Join(t.TempDir(), "eval.json")
	if err := writeEvaluation(ev, out, filepath.Join(t.TempDir(), "eval.txt")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var back evaluation
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("report JSON does not round-trip: %v", err)
	}
	if back.CallCeiling != DeclaredCallCeiling || len(back.Bands) != len(ev.Bands) {
		t.Fatalf("round-tripped report differs: ceiling=%d bands=%d", back.CallCeiling, len(back.Bands))
	}
}

// testRecordingSelector delegates to a fresh L3 accumulator and records every
// selection step, so a checkpoint/restore replay can be compared against the
// live run.
type testRecordingSelector struct {
	inner *cognition.PrioritySelector
	mu    sync.Mutex
	steps []selectionStep
}

func (s *testRecordingSelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	selection, err := s.inner.Select(hour, roster, budget)
	if err != nil {
		return selection, err
	}
	s.mu.Lock()
	s.steps = append(s.steps, selectionStep{Hour: hour, Selected: append([]sim.EntityID(nil), selection.Selected...), Spent: s.inner.CallsSpent(), PRNGPos: s.inner.PRNGPos()})
	s.mu.Unlock()
	return selection, nil
}

type testRecordingFactory struct {
	t         *testing.T
	policy    protocol.WakePolicy
	horizon   int
	mu        sync.Mutex
	selectors []*testRecordingSelector
}

func (f *testRecordingFactory) new() protocol.EscalationSelector {
	inner, err := cognition.NewPrioritySelector(f.policy, f.horizon, selectorPrice())
	if err != nil {
		f.t.Fatal(err)
	}
	s := &testRecordingSelector{inner: inner}
	f.mu.Lock()
	f.selectors = append(f.selectors, s)
	f.mu.Unlock()
	return s
}

func TestPriorityCheckpointReplayRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fp := testFixture(10, 0)
	spec := cellSpec{Arm: ArmU, F: 10, Seed: 0}
	// A direct births-enabled genesis runner (not the branch child) so the
	// journal export reconciliation applies; the matrix path uses the branch
	// child and reconciles through SaveResumeCheckpointWithReplaySelector.
	opts := naturalOptions(fp)
	opts.Births = true
	f, err := world.NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	f, _, cr, err := executeCell(ctx, f, spec, fp, dir, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cr.selectorSteps) == 0 {
		t.Fatal("the priority selector was never consulted")
	}
	if cr.realizedCalls == 0 {
		t.Fatal("the priority arm made no model call")
	}
	if cr.finalBundleError != "" {
		t.Fatalf("final bundle save failed: %s", cr.finalBundleError)
	}

	policy := declaredPolicy(ArmU, spec.Seed)
	replay, err := cognition.OpenReplay(cr.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	seam, err := cognition.NewReplaySeam(replay, policy)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := cognition.VerifyLedgerData(mustRead(t, cr.ledgerPath))
	if err != nil {
		t.Fatal(err)
	}
	f.SetCognitiveCheckpoint(&world.NaturalCognitiveCheckpoint{
		WakePolicy: policy, LedgerDigest: summary.Digest, LedgerCount: summary.Count,
	})

	export := &testRecordingFactory{t: t, policy: policy, horizon: testHorizon}
	journal, err := f.ExportJournalWithReplaySelector(seam, policy, export.new)
	if err != nil {
		t.Fatalf("ExportJournalWithReplaySelector: %v", err)
	}
	if len(journal) == 0 {
		t.Fatal("empty exported journal")
	}
	save := &testRecordingFactory{t: t, policy: policy, horizon: testHorizon}
	bundle := filepath.Join(t.TempDir(), "priority.bundle")
	if _, err := f.SaveResumeCheckpointWithReplaySelector(bundle, seam, policy, save.new); err != nil {
		t.Fatalf("SaveResumeCheckpointWithReplaySelector: %v", err)
	}
	restore := &testRecordingFactory{t: t, policy: policy, horizon: testHorizon}
	restored, _, err := world.RestoreNaturalCheckpoint(bundle, world.NaturalRestoreOptions{
		Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: basePolicy(), Mechanism: mechanism(), Founders: 10,
		CognitiveReplaySeam: seam, CognitiveReplayWake: policy,
		CognitiveReplaySelector: restore.new, CognitiveLedgerPath: cr.ledgerPath,
	})
	if err != nil {
		t.Fatalf("priority restore: %v", err)
	}
	if !reflect.DeepEqual(f.Checkpoints(), restored.Checkpoints()) {
		t.Fatal("restored priority run differs from the live run")
	}
	for name, factory := range map[string]*testRecordingFactory{"export": export, "save": save, "restore": restore} {
		if len(factory.selectors) == 0 {
			t.Fatalf("%s: no replay selector was built", name)
		}
		for i, sel := range factory.selectors {
			if !reflect.DeepEqual(sel.steps, cr.selectorSteps) {
				t.Fatalf("%s: replay selector %d diverged from the live selection sequence", name, i)
			}
		}
	}
}

// TestReplaySubsetIdentical pins that the runner's own replay lane reports the
// priority arm identical.
func TestReplaySubsetIdentical(t *testing.T) {
	runs := runTestMatrix(t)
	replay := runReplaySubset(context.Background(), runs, t.TempDir(), 2)
	if len(replay) != len(budgetedArms) {
		t.Fatalf("replayed %d cells, want %d", len(replay), len(budgetedArms))
	}
	for k, v := range replay {
		if v != "identical" {
			t.Fatalf("%s: %s", k, v)
		}
	}
}

func TestLiveGateRefusesWithoutAuthorization(t *testing.T) {
	t.Setenv(EnvSpendAuthorization, "")
	if err := authorizeLive(false); err != nil {
		t.Fatalf("hermetic mode refused: %v", err)
	}
	if err := authorizeLive(true); !errors.Is(err, ErrLiveRefused) {
		t.Fatalf("live mode without authorization: %v, want ErrLiveRefused", err)
	}
	t.Setenv(EnvSpendAuthorization, DeclaredSpendToken)
	if err := authorizeLive(true); err != nil {
		t.Fatalf("authorized live mode refused: %v", err)
	}
}

func TestHermeticResponderIsDeterministic(t *testing.T) {
	resp := hermeticResponder(cellSpec{Arm: ArmU, F: 10, Seed: 0})
	req := cognition.ModelRequest{
		Request: []byte(`{"role":"actor-decision","actor":{"hour":3},"observation":{"bag_units":0,"granary_stock":2,"last_stored_meal_hour":0,"home_slots":[{"stock":1}]}}`),
	}
	first, err := resp(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := resp(req)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Content) != string(second.Content) {
		t.Fatalf("responder not deterministic: %q vs %q", first.Content, second.Content)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
