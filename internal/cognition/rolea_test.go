package cognition_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

func roleAClient(body string) *cognition.FakeClient {
	return &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		return cognition.ModelResponse{
			Body: []byte(body), Content: []byte(body),
			Usage:  cognition.Usage{PromptTokens: 5, CompletionTokens: 3},
			Status: cognition.CallStatusOK,
		}, nil
	}}
}

func newRoleASeam(t *testing.T, body string) (*cognition.RoleASeam, *cognition.Ledger, string) {
	t.Helper()
	l, path := newTestLedger(t)
	rec := newRecorder(t, l, roleAClient(body), newAccount(t, generousBudget()))
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: rec, Ledger: l, Endpoint: "gw.example/v1/chat/completions",
		CredentialPresent: true, WakePolicy: cognition.VS1WakePolicy(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return seam, l, path
}

// TestRoleAValidationLadderRejectsIllegalOutputs is the level-1 guard (spec
// 4.3): each malformed or inadmissible model output returns the declared
// fallback with its typed reason and no action; a valid output returns the
// model decision. Deleting any admissibility check makes the corresponding
// case fail.
func TestRoleAValidationLadderRejectsIllegalOutputs(t *testing.T) {
	unstocked := func() cognition.ActorSeamRequest {
		req := sampleSeamRequest()
		req.Observation.HomeSlots = []strategy.NaturalSlotView{
			{ID: 2001, Stock: sim.IntegerValue(0)},
			{ID: 2002, Stock: sim.IntegerValue(0)},
		}
		return req
	}
	noGranary := func() cognition.ActorSeamRequest {
		req := sampleSeamRequest()
		req.Observation.GranaryStock = sim.IntegerValue(0)
		return req
	}
	bagHeld := func() cognition.ActorSeamRequest {
		req := sampleSeamRequest()
		req.Observation.BagUnits = sim.IntegerValue(1)
		return req
	}

	cases := []struct {
		name       string
		content    string
		req        cognition.ActorSeamRequest
		wantSource string
		wantReason string
		wantAction string
	}{
		{"valid gather", `{"schema_version":1,"action":"gather","rationale":"stocked","confidence":0.9}`, sampleSeamRequest(), cognition.SourceModel, "", "gather"},
		{"valid wait", `{"schema_version":1,"action":"wait"}`, sampleSeamRequest(), cognition.SourceModel, "", "wait"},
		{"valid eat-stored", `{"schema_version":1,"action":"eat-stored"}`, sampleSeamRequest(), cognition.SourceModel, "", "eat-stored"},
		{"valid provision", `{"schema_version":1,"action":"provision"}`, sampleSeamRequest(), cognition.SourceModel, "", "provision"},
		{"gather without stock", `{"schema_version":1,"action":"gather"}`, unstocked(), cognition.SourceFallback, string(cognition.FallbackReasonInadmissible), ""},
		{"eat-stored without granary", `{"schema_version":1,"action":"eat-stored"}`, noGranary(), cognition.SourceFallback, string(cognition.FallbackReasonInadmissible), ""},
		{"provision without granary", `{"schema_version":1,"action":"provision"}`, noGranary(), cognition.SourceFallback, string(cognition.FallbackReasonInadmissible), ""},
		{"provision with a held bag", `{"schema_version":1,"action":"provision"}`, bagHeld(), cognition.SourceFallback, string(cognition.FallbackReasonInadmissible), ""},
		{"unknown action", `{"schema_version":1,"action":"teleport"}`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonInadmissible), ""},
		{"not json", `{oops`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonMalformed), ""},
		{"wrong schema version", `{"schema_version":2,"action":"gather"}`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonMalformed), ""},
		{"missing action", `{"schema_version":1}`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonMalformed), ""},
		{"long rationale", `{"schema_version":1,"action":"gather","rationale":"` + strings.Repeat("x", 513) + `"}`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonMalformed), ""},
		{"confidence out of range", `{"schema_version":1,"action":"gather","confidence":2}`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonMalformed), ""},
		{"unknown field", `{"schema_version":1,"action":"gather","privileged":true}`, sampleSeamRequest(), cognition.SourceFallback, string(cognition.FallbackReasonMalformed), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := cognition.ValidateActorSeamContent([]byte(tc.content), tc.req)
			if err != nil {
				t.Fatalf("validation error: %v", err)
			}
			if decision.Source != tc.wantSource || decision.Reason != tc.wantReason || decision.Action != tc.wantAction {
				t.Fatalf("got %+v, want source=%q reason=%q action=%q", decision, tc.wantSource, tc.wantReason, tc.wantAction)
			}
		})
	}
}

// TestRoleASeamRecordsEveryDecision is the "seam cannot be entered without a
// recorded ledger entry" regression: every escalated decision records the
// call and appends exactly one intervention referencing it.
func TestRoleASeamRecordsEveryDecision(t *testing.T) {
	seam, l, path := newRoleASeam(t, `{"schema_version":1,"action":"gather","confidence":1}`)
	decision, err := seam.DecideActorSeam(context.Background(), sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Source != cognition.SourceModel || decision.Action != "gather" {
		t.Fatalf("unexpected decision %+v", decision)
	}
	if l.Count() != 2 {
		t.Fatalf("expected one call record and one intervention, got %d", l.Count())
	}
	calls, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || decision.CallRef != calls[0].Seq {
		t.Fatalf("decision CallRef %d does not reference the call record %+v", decision.CallRef, calls)
	}
	interventions, err := cognition.ReadInterventionRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(interventions) != 1 {
		t.Fatalf("expected exactly one intervention, got %d", len(interventions))
	}
	refs := interventions[0].CallRefs
	if len(refs) != 1 || refs[0] != calls[0].Seq {
		t.Fatalf("intervention call_refs %v do not reference the call %d", refs, calls[0].Seq)
	}
}

// TestRoleASeamCannotBeEnteredWithoutLedger is the regression's guard: a seam
// without the recorded-call path refuses construction and refuses the call.
// Removing the nil-ledger guard makes this fail.
func TestRoleASeamCannotBeEnteredWithoutLedger(t *testing.T) {
	l, _ := newTestLedger(t)
	rec := newRecorder(t, l, roleAClient(`{"schema_version":1,"action":"wait"}`), newAccount(t, generousBudget()))
	if _, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: rec, Endpoint: "gw.example/v1/chat/completions", WakePolicy: cognition.VS1WakePolicy(),
	}); !errors.Is(err, cognition.ErrRoleASeamNoLedger) {
		t.Fatalf("nil ledger accepted: %v", err)
	}
	if _, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Ledger: l, Endpoint: "gw.example/v1/chat/completions", WakePolicy: cognition.VS1WakePolicy(),
	}); !errors.Is(err, cognition.ErrRoleASeamNoLedger) {
		t.Fatalf("nil recorder accepted: %v", err)
	}
	var zero cognition.RoleASeam
	if _, err := zero.DecideActorSeam(context.Background(), sampleSeamRequest()); !errors.Is(err, cognition.ErrRoleASeamNoLedger) {
		t.Fatalf("zero seam did not refuse: %v", err)
	}
}

// TestRoleASeamInvalidWakePolicyRefused pins the run-identity validation.
func TestRoleASeamInvalidWakePolicyRefused(t *testing.T) {
	l, _ := newTestLedger(t)
	rec := newRecorder(t, l, roleAClient(`{"schema_version":1,"action":"wait"}`), newAccount(t, generousBudget()))
	if _, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: rec, Ledger: l, Endpoint: "gw.example/v1/chat/completions",
	}); err == nil {
		t.Fatal("invalid wake policy accepted")
	}
}
