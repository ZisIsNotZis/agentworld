package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
)

// The model axis (W3 Stage M prerequisite). These tests pin that -model is a
// recorded axis, not an inferred one: a hermetic run on a non-default model
// records that id in the evaluation, the revision string, the model identity
// digests and every Role A request; the default stays the declared luna id; an
// empty id is refused; and the curve report keeps model-distinct conditions
// apart instead of merging them.

const testModelAlt = "zai/glm-5.3-flash"

// TestDeclaredModelDefaultIsLuna pins the default so the model axis can only be
// moved by an explicit flag.
func TestDeclaredModelDefaultIsLuna(t *testing.T) {
	if cognition.DefaultModelID != "github_copilot/gpt-6-luna" {
		t.Fatalf("declared application model = %q, want github_copilot/gpt-6-luna", cognition.DefaultModelID)
	}
	if declaredModelID() != cognition.DefaultModelID {
		t.Fatalf("package default model = %q, want %q", declaredModelID(), cognition.DefaultModelID)
	}
}

func TestEmptyModelIsRefused(t *testing.T) {
	for _, in := range []string{"", "   ", "\t"} {
		if _, err := parseApplicationModel(in); err == nil {
			t.Fatalf("parseApplicationModel(%q) accepted an empty id", in)
		}
	}
	if got, err := parseApplicationModel(" github_copilot/gpt-6-sol "); err != nil || got != "github_copilot/gpt-6-sol" {
		t.Fatalf("parseApplicationModel = (%q, %v), want the trimmed id", got, err)
	}
}

// TestNonDefaultModelRecordedEverywhere runs one hermetic reference cell on a
// non-default model and checks the id reaches every recorded surface.
func TestNonDefaultModelRecordedEverywhere(t *testing.T) {
	restore := declaredApplicationModel
	t.Cleanup(func() { declaredApplicationModel = restore })
	declaredApplicationModel = testModelAlt

	ctx := context.Background()
	root := t.TempDir()
	cr, err := runCell(ctx, cellSpec{Arm: ArmR, F: 10, Seed: 0}, testFixture(10, 0), root, nil)
	if err != nil {
		t.Fatalf("R on %s: %v", testModelAlt, err)
	}
	declared := []string{"R-f10-s0"}
	ev := buildEvaluation([]*cellRun{cr}, t.TempDir(), nil, declared, nil, filepath.Join("..", ".."), testHorizon)
	ev.Revision = revisionString()

	if ev.ModelID != testModelAlt {
		t.Fatalf("evaluation model_id = %q, want %q", ev.ModelID, testModelAlt)
	}
	if !containsSubstring(ev.Revision, "model "+testModelAlt) {
		t.Fatalf("revision does not name the model: %q", ev.Revision)
	}
	wantIdentity := cognition.ModelIdentityDigest(testModelAlt, declaredGatewayWire, cognition.FrozenSampling())
	if ev.ModelIdentity != hex.EncodeToString(wantIdentity[:]) {
		t.Fatalf("model identity digest = %q, want %x (the digest of %s)", ev.ModelIdentity, wantIdentity, testModelAlt)
	}
	wantFeature := cognition.ModelFeatureIdentityDigest(testModelAlt, declaredGatewayWire, cognition.FrozenSampling(), protocol.FeatureSetVersion)
	if ev.ModelFeatureIdentity != hex.EncodeToString(wantFeature[:]) {
		t.Fatalf("model+feature identity digest = %q, want %x", ev.ModelFeatureIdentity, wantFeature)
	}
	// The default's digest must differ, so a model change cannot share an identity.
	def := cognition.ModelIdentityDigest(cognition.DefaultModelID, declaredGatewayWire, cognition.FrozenSampling())
	if ev.ModelIdentity == hex.EncodeToString(def[:]) {
		t.Fatal("alt-model evaluation carries the default model identity digest")
	}

	if len(cr.calls) == 0 {
		t.Fatal("the reference cell recorded no calls; the request check would be vacuous")
	}
	for _, call := range cr.calls {
		if call.Role != cognition.RoleActorDecision {
			continue
		}
		if call.ModelID != testModelAlt {
			t.Fatalf("%s#%d recorded model_id = %q, want %q", cr.spec, call.Seq, call.ModelID, testModelAlt)
		}
		var wire struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(call.RequestBytes, &wire); err != nil {
			t.Fatalf("%s#%d request: %v", cr.spec, call.Seq, err)
		}
		if wire.Model != testModelAlt {
			t.Fatalf("%s#%d request model = %q, want %q", cr.spec, call.Seq, wire.Model, testModelAlt)
		}
	}
}

// curveEvalFixture builds a minimal one-cell evaluation a curve report can read:
// one arm condition with a two-hour KPI series.
func curveEvalFixture(model, effort string) evaluation {
	return evaluation{
		ModelID: model,
		Cells: []cellReport{{
			Arm: ArmR, F: 10, Seed: 0, Ran: true, Effort: effort,
			Hours:           []int{0, 1},
			AliveByHour:     []int{10, 10},
			BirthsCum:       []int64{0, 0},
			DeathsStarveCum: []int64{0, 0},
			DeathsAgeCum:    []int64{0, 0},
			WagesCum:        []int64{0, 0},
			MaxCapital:      []int64{0, 0},
			CapitalHist:     [][]int64{{0, 0}, {0, 0}},
			GranaryHist:     [][]int64{{0, 0}, {0, 0}},
		}},
	}
}

func writeCurveEval(t *testing.T, dir, name string, ev evaluation) string {
	t.Helper()
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCurveLabelsNeverMergeDifferentModels pins the curve-condition identity:
// a single-model report keeps the historical arm@effort label, while a report
// over two models labels each with its model and never merges them.
func TestCurveLabelsNeverMergeDifferentModels(t *testing.T) {
	const other = "github_copilot/gpt-6-sol"
	dir := t.TempDir()
	luna := writeCurveEval(t, dir, "stage1-luna.json", curveEvalFixture(cognition.DefaultModelID, "max"))
	luna2 := writeCurveEval(t, dir, "stage1-luna-rep.json", curveEvalFixture(cognition.DefaultModelID, "max"))
	sol := writeCurveEval(t, dir, "stage1-sol.json", curveEvalFixture(other, "max"))

	single, err := curveReportFromFiles([]string{luna})
	if err != nil {
		t.Fatal(err)
	}
	if len(single.Conditions) != 1 || single.Conditions[0] != "R@max" {
		t.Fatalf("single-model conditions = %v, want [R@max] (backward-compatible label)", single.Conditions)
	}

	// A single-model report keeps the historical labels but still names its model
	// in the header, so a non-default model's report cannot be read as the default.
	soloAlt, err := curveReportFromFiles([]string{sol})
	if err != nil {
		t.Fatal(err)
	}
	if len(soloAlt.Models) != 1 || soloAlt.Models[0] != other {
		t.Fatalf("single alt-model report models = %v, want [%s]", soloAlt.Models, other)
	}
	if !containsSubstring(renderCurveReport(soloAlt), "models: "+other) {
		t.Fatalf("single alt-model report does not name its model")
	}

	sameModel, err := curveReportFromFiles([]string{luna, luna2})
	if err != nil {
		t.Fatal(err)
	}
	if len(sameModel.Conditions) != 1 || sameModel.Conditions[0] != "R@max" {
		t.Fatalf("same-model conditions = %v, want one merged R@max condition", sameModel.Conditions)
	}
	if sameModel.Replicates != 2 {
		t.Fatalf("same-model replicates = %d, want 2", sameModel.Replicates)
	}

	mixed, err := curveReportFromFiles([]string{luna, sol})
	if err != nil {
		t.Fatal(err)
	}
	wantA := "R@max@" + cognition.DefaultModelID
	wantB := "R@max@" + other
	if len(mixed.Conditions) != 2 || mixed.Conditions[0] != wantA || mixed.Conditions[1] != wantB {
		t.Fatalf("mixed-model conditions = %v, want [%s %s]", mixed.Conditions, wantA, wantB)
	}
	if len(mixed.Stages) == 0 || len(mixed.Stages[0].Conditions) != 2 {
		t.Fatalf("mixed-model stage conditions = %+v, want both models", mixed.Stages)
	}
	// The report text names the models in its condition list, so a reader cannot
	// mistake one model's band for another's.
	text := renderCurveReport(mixed)
	if !containsSubstring(text, wantA) || !containsSubstring(text, wantB) {
		t.Fatalf("curve report text does not name both model-qualified conditions")
	}
}
