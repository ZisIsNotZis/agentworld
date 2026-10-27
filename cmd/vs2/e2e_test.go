package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/experiment"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// The hermetic end-to-end tests: a bounded matrix produces a complete report, a
// priority arm round-trips through checkpoint/restore/replay with no drift, and
// the live spend gate refuses by default.

const testHorizon = 8

func testFixture(f int, seed uint64) fixture {
	fp := declaredWorld(f, seed)
	fp.Hours = testHorizon
	return fp
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
	// The report names the declared feature set and binds it into a separate
	// model+feature identity digest (the model identity digest is unchanged).
	if ev.FeatureSet != protocol.FeatureSetVersion {
		t.Fatalf("feature set = %q, want %q", ev.FeatureSet, protocol.FeatureSetVersion)
	}
	if ev.ModelFeatureIdentity == "" || len(ev.ModelFeatureIdentity) != 64 {
		t.Fatalf("model+feature identity digest = %q", ev.ModelFeatureIdentity)
	}
	wantFeatureIdentity := cognition.ModelFeatureIdentityDigest(cognition.DefaultModelID, cognition.WireResponses, cognition.FrozenSampling(), protocol.FeatureSetVersion)
	if hex.EncodeToString(wantFeatureIdentity[:]) != ev.ModelFeatureIdentity {
		t.Fatalf("model+feature identity = %q, want %x", ev.ModelFeatureIdentity, wantFeatureIdentity)
	}
	// The evaluation names the whole declared W1 world, not just its mechanism:
	// base policy, mechanism, yield and birth price are recorded, so a reader never
	// has to infer the axes from the mechanism version.
	if ev.BasePolicy != declaredBasePolicy().ID || ev.Mechanism != int(declaredMechanism()) ||
		ev.Yield != DeclaredYield || ev.BirthCost != DeclaredBirthCost {
		t.Fatalf("evaluation world identity = policy %q mechanism %d yield %d birth-cost %d, want natural/14, v14, 1, 2",
			ev.BasePolicy, ev.Mechanism, ev.Yield, ev.BirthCost)
	}
	if ev.HorizonHours != testHorizon {
		t.Fatalf("evaluation horizon = %d, want %d", ev.HorizonHours, testHorizon)
	}
	// Every recorded Role A request carries the same declared fixture identity: the
	// v14 mechanism and the natural/14 base ref the world was built from.
	for _, cr := range runs {
		for _, call := range cr.calls {
			if call.Role != cognition.RoleActorDecision {
				continue
			}
			var wire struct {
				Run struct {
					Fixture struct {
						Mechanism uint32 `json:"mechanism"`
					} `json:"fixture"`
					BaseRef struct {
						ID      string `json:"id"`
						Version uint32 `json:"version"`
					} `json:"base_ref"`
				} `json:"run"`
			}
			if err := json.Unmarshal(call.RequestBytes, &wire); err != nil {
				t.Fatalf("%s#%d request: %v", cr.spec, call.Seq, err)
			}
			want := declaredWorld(10, 0)
			if wire.Run.Fixture.Mechanism != uint32(want.Mechanism) ||
				wire.Run.BaseRef.ID != want.PolicyRef.ID || wire.Run.BaseRef.Version != want.PolicyRef.Version {
				t.Fatalf("%s#%d fixture identity = mechanism %d base %s/%d, want mechanism %d base %s/%d",
					cr.spec, call.Seq, wire.Run.Fixture.Mechanism, wire.Run.BaseRef.ID, wire.Run.BaseRef.Version,
					want.Mechanism, want.PolicyRef.ID, want.PolicyRef.Version)
			}
		}
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
	restoreOpts := restoreOptions(fp, true)
	restoreOpts.CognitiveReplaySeam = seam
	restoreOpts.CognitiveReplayWake = policy
	restoreOpts.CognitiveReplaySelector = restore.new
	restoreOpts.CognitiveLedgerPath = cr.ledgerPath
	restored, _, err := world.RestoreNaturalCheckpoint(bundle, restoreOpts)
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

// TestReplaySubsetIdentical pins that the runner's own replay lane reports every
// ledger-bearing arm identical: the budgeted priority arms and the R reference
// (nil Priority) added by ticket 57.
func TestReplaySubsetIdentical(t *testing.T) {
	runs := runTestMatrix(t)
	replay := runReplaySubset(context.Background(), runs, t.TempDir(), 2)
	if len(replay) != len(budgetedArms)+1 {
		t.Fatalf("replayed %d cells, want %d (budgeted arms + R)", len(replay), len(budgetedArms)+1)
	}
	for k, v := range replay {
		if v != "identical" {
			t.Fatalf("%s: %s", k, v)
		}
	}
}

// TestShortProjectionRecordedInReport pins that an arm whose hourly projection
// is shorter than the declared horizon is named in the report's
// short_projections field rather than silently absent: the arm, its actual
// projection length and the missing hours are recorded, and the divergence
// vector is refused (never truncated, spec section 5.3).
func TestShortProjectionRecordedInReport(t *testing.T) {
	full := experiment.Projections{Hours: make([]int, testHorizon+1)}
	for i := range full.Hours {
		full.Hours[i] = i
	}
	const missing = 3
	short := experiment.Projections{Hours: make([]int, testHorizon+1-missing)}
	for i := range short.Hours {
		short.Hours[i] = i
	}
	runs := []*cellRun{
		{spec: cellSpec{Arm: ArmR, F: 10, Seed: 0}, fp: fixture{Patches: 2, Hours: testHorizon}, proj: full},
		{spec: cellSpec{Arm: ArmU, F: 10, Seed: 0}, fp: fixture{Patches: 2, Hours: testHorizon}, proj: short},
	}
	rows, shorts := buildFidelity(runs, testHorizon)
	if len(rows) != 0 {
		t.Fatalf("fidelity rows = %d, want 0 (the short arm is refused, not truncated)", len(rows))
	}
	if len(shorts) != 1 {
		t.Fatalf("short projections = %d, want 1", len(shorts))
	}
	got := shorts[0]
	if got.Arm != "U-f10-s0" || got.ProjectionSamples != testHorizon+1-missing ||
		got.RequiredSamples != testHorizon+1 || got.MissingHours != missing {
		t.Fatalf("short projection %+v, want U-f10-s0 %d of %d missing %d",
			got, testHorizon+1-missing, testHorizon+1, missing)
	}

	// The report carries the short arm in its declared field and round-trips.
	ev := buildEvaluation(runs, t.TempDir(), nil, []string{"R-f10-s0", "U-f10-s0"}, nil, filepath.Join("..", ".."), testHorizon)
	if len(ev.ShortProjections) != 1 || ev.ShortProjections[0].Arm != "U-f10-s0" {
		t.Fatalf("report short_projections = %+v, want the short U arm", ev.ShortProjections)
	}
	out := filepath.Join(t.TempDir(), "eval.json")
	if err := writeEvaluation(ev, out, filepath.Join(t.TempDir(), "eval.txt")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"short_projections"`) {
		t.Fatal("report JSON omits short_projections")
	}
	var back evaluation
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("report JSON does not round-trip: %v", err)
	}
	if len(back.ShortProjections) != 1 || back.ShortProjections[0].MissingHours != missing {
		t.Fatalf("round-tripped short_projections = %+v", back.ShortProjections)
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

// TestDeclaredWorldA0ReproducesTicket55 pins the W1 declared cell through the
// runner's own machinery: the A0 arm on the declared world (mechanism v14,
// natural/14, Yield 1, BirthCost 2, H=144, f10-s0) must reproduce the committed
// ticket-55 calibration exactly. It fails loud if the runner's genesis (neutral
// h0 pulse + births-enable branch), its step loop, or its world construction
// drifts from the world the W1 verdict was measured on; the world is never tuned
// to fit the runner.
func TestDeclaredWorldA0ReproducesTicket55(t *testing.T) {
	ctx := context.Background()
	fp := declaredWorld(10, 0)
	cr, err := runCell(ctx, cellSpec{Arm: ArmA0, F: 10, Seed: 0}, fp, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("A0: %v", err)
	}
	if cr.runErr != nil {
		t.Fatalf("A0 run: %v", cr.runErr)
	}
	last := cr.checks[len(cr.checks)-1]
	got := last.Metrics
	if got.Alive != 11 || got.Births != 12 || got.DeathsStarvation != 1 || got.DeathsAge != 10 {
		t.Fatalf("declared cell %s: alive=%d births=%d starvation=%d age=%d, want the ticket-55 cell 11/12/1/10",
			cellSpec{Arm: ArmA0, F: 10, Seed: 0}, got.Alive, got.Births, got.DeathsStarvation, got.DeathsAge)
	}
	if bal := last.Balance; bal.Energy != 184 || bal.Produced != 1145 {
		t.Fatalf("declared cell balance drifted: energy=%d produced=%d", bal.Energy, bal.Produced)
	}
}
