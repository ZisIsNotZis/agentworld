package cognition_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agentworld/internal/cognition"
)

// applyBoundClient is a transport that applies a declared reasoning effort, an
// output-token bound and a recorded-body bound, and reports them back the way
// the gateway client does.
type applyBoundClient struct {
	body []byte
}

func (c applyBoundClient) Complete(context.Context, cognition.ModelRequest) (cognition.ModelResponse, error) {
	return cognition.ModelResponse{
		Body: c.body, Content: []byte(`{"schema_version":1,"action":"wait","rationale":"x","confidence":1}`),
		Usage: cognition.Usage{PromptTokens: 10, CompletionTokens: 5}, Status: cognition.CallStatusOK,
	}, nil
}

func (c applyBoundClient) WireModel() string                { return cognition.DefaultModelID }
func (c applyBoundClient) OutputMode() cognition.OutputMode { return cognition.OutputJSON }
func (c applyBoundClient) AppliedReasoningEffort() string   { return "max" }
func (c applyBoundClient) AppliedMaxOutputTokens() int      { return 32768 }

// TestRecorderRecordsAppliedBounds checks the two halves of the provenance fix:
// a run that applies non-default wire settings records them on every call record
// beside the declared sampling instance, and a hermetic run that applies the
// declared state adds nothing to the record, so its bytes stay unchanged.
func TestRecorderRecordsAppliedBounds(t *testing.T) {
	body := make([]byte, 100<<10) // 100 KiB: over the 64 KiB default, under 1 MiB
	for i := range body {
		body[i] = 'x'
	}
	cases := []struct {
		name       string
		bound      int
		wantStatus cognition.CallStatus
		wantBound  int
		wantBytes  int
	}{
		{name: "declared 1 MiB bound accepts a 100 KiB body", bound: 1 << 20, wantStatus: cognition.CallStatusOK, wantBound: 1 << 20, wantBytes: len(body)},
		{name: "hermetic default rejects it as oversized", bound: 0, wantStatus: cognition.CallStatusOversized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ledger, err := cognition.CreateLedger(filepath.Join(dir, "calls.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			account, err := cognition.NewCognitiveAccount(cognition.CognitiveBudget{MaxCalls: 4, MaxPromptTokens: 1 << 40, MaxCompletionTokens: 1 << 40, MaxCostMicros: 1 << 40})
			if err != nil {
				t.Fatal(err)
			}
			recorder, err := cognition.NewRecorder(cognition.RecorderConfig{
				Ledger: ledger, Client: applyBoundClient{body: body}, Budget: account,
				Price: cognition.UnpricedPrice{}, MaxResponseBytes: tc.bound,
				Sleep: func(context.Context, time.Duration) error { return nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := recorder.Call(context.Background(), cognition.CallRequest{
				Role: cognition.RoleActorDecision, Hour: 0, Model: cognition.DefaultModelID,
				Endpoint: "gateway.local/v1/responses", Sampling: cognition.FrozenSampling(),
				Request: []byte(`{"role":"actor-decision"}`),
			}); err != nil {
				t.Fatal(err)
			}
			records, err := cognition.ReadCallRecords(filepath.Join(dir, "calls.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != 1 {
				t.Fatalf("records=%d", len(records))
			}
			rec := records[0]
			if rec.Status != tc.wantStatus {
				t.Fatalf("status=%s want %s", rec.Status, tc.wantStatus)
			}
			if rec.AppliedReasoningEffort != "max" || rec.AppliedMaxOutputTokens != 32768 {
				t.Fatalf("applied sampling: %q/%d", rec.AppliedReasoningEffort, rec.AppliedMaxOutputTokens)
			}
			if rec.Sampling.MaxTokens != 128 {
				t.Fatalf("declared sampling must stay recorded unchanged, got max_tokens=%d", rec.Sampling.MaxTokens)
			}
			if rec.ResponseByteBound != tc.wantBound || rec.ResponseBytes != tc.wantBytes {
				t.Fatalf("response bound/bytes = %d/%d want %d/%d", rec.ResponseByteBound, rec.ResponseBytes, tc.wantBound, tc.wantBytes)
			}
		})
	}
}
