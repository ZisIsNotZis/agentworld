package cognition_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
)

// --- hermetic wire fixtures -------------------------------------------------

func chatWireBody(text string, prompt, completion uint64) []byte {
	body, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": text},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": prompt, "completion_tokens": completion},
	})
	return body
}

func responsesWireBody(text string, input, output uint64) []byte {
	body, _ := json.Marshal(map[string]any{
		"id":     "resp_test",
		"object": "response",
		"status": "completed",
		"output": []any{
			map[string]any{"type": "reasoning", "content": []any{}},
			map[string]any{"type": "message", "content": []any{
				map[string]any{"type": "output_text", "text": text},
			}},
		},
		"usage": map[string]any{"input_tokens": input, "output_tokens": output},
	})
	return body
}

const actorDecisionJSON = `{"schema_version":1,"action":"gather","rationale":"model","confidence":0.5}`

const strategyPatchJSON = `{"schema_version":1,"propose":true,"parent_ref":{"id":"natural","version":7},` +
	`"deltas":[{"param":"claim.gather_energy_floor","value":2}],"intent":"try"}`

func gatewayForTest(t *testing.T, srv *httptest.Server, mode cognition.WireMode) *cognition.GatewayClient {
	t.Helper()
	gw, err := cognition.NewGatewayClient(cognition.GatewayConfig{
		BaseURL: srv.URL, Authorized: true, CredentialPresent: true, APIKey: "test-key",
		WireMode: mode, Model: cognition.DefaultGatewayModel, HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return gw
}

func gatewayCall(gw *cognition.GatewayClient, role string, hour int) cognition.CallRequest {
	return cognition.CallRequest{
		Role: role, Hour: hour, Model: cognition.DefaultModelID,
		Endpoint: gw.WireEndpoint(), CredentialPresent: true,
		Sampling: cognition.FrozenSampling(), Request: []byte(`{"schema_version":1}`),
	}
}

func recorderWith(t *testing.T, l *cognition.Ledger, client cognition.ModelClient, price cognition.Pricer,
	acct *cognition.CognitiveAccount, cfg cognition.RecorderConfig) *cognition.Recorder {
	t.Helper()
	cfg.Ledger, cfg.Client, cfg.Budget, cfg.Price = l, client, acct, price
	cfg.Now = func() time.Time { return time.UnixMicro(1_700_000_000_000_000) }
	if cfg.Sleep == nil {
		cfg.Sleep = func(context.Context, time.Duration) error { return nil }
	}
	rec, err := cognition.NewRecorder(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// --- wire construction and decoding ----------------------------------------

// TestGatewayChatCompletionsWireBuildsAndDecodes pins the chat wire: the request
// is the canonical-request user message plus the JSON output mode, and
// prompt_tokens/completion_tokens are extracted.
func TestGatewayChatCompletionsWireBuildsAndDecodes(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatWireBody(actorDecisionJSON, 41, 27))
	}))
	defer srv.Close()

	gw := gatewayForTest(t, srv, cognition.WireChatCompletions)
	resp, err := gw.Complete(context.Background(), cognition.ModelRequest{
		Endpoint: gw.WireEndpoint(), Sampling: cognition.FrozenSampling(), Request: []byte(`{"schema_version":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != cognition.CallStatusOK || string(resp.Content) != actorDecisionJSON {
		t.Fatalf("chat completion not decoded: %+v", resp)
	}
	if resp.Usage != (cognition.Usage{PromptTokens: 41, CompletionTokens: 27}) {
		t.Fatalf("chat usage not extracted: %+v", resp.Usage)
	}
	if gotPath != "/v1/chat/completions" || gotAuth != "Bearer test-key" {
		t.Fatalf("chat request path/auth = %q %q", gotPath, gotAuth)
	}
	if gotBody["model"] != cognition.DefaultGatewayModel {
		t.Fatalf("wire model = %v, want %s", gotBody["model"], cognition.DefaultGatewayModel)
	}
	if format, ok := gotBody["response_format"].(map[string]any); !ok || format["type"] != "json_object" {
		t.Fatalf("chat JSON mode not requested: %v", gotBody["response_format"])
	}
	messages, _ := gotBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("chat messages = %v", gotBody["messages"])
	}
}

// TestGatewayResponsesWireMapsDecisionAndProposal is the core Responses mapping
// proof: a /v1/responses body (with a preceding reasoning item) yields the same
// content a chat body would, and that content maps to the same internal
// ActorSeamDecision and PatchProposal types.
func TestGatewayResponsesWireMapsDecisionAndProposal(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responsesWireBody(actorDecisionJSON, 41, 27))
	}))
	defer srv.Close()

	gw := gatewayForTest(t, srv, cognition.WireResponses)
	resp, err := gw.Complete(context.Background(), cognition.ModelRequest{
		Endpoint: gw.WireEndpoint(), Sampling: cognition.FrozenSampling(), Request: []byte(`{"schema_version":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/responses" {
		t.Fatalf("responses request path = %q", gotPath)
	}
	if resp.Usage != (cognition.Usage{PromptTokens: 41, CompletionTokens: 27}) {
		t.Fatalf("responses usage (input/output) not mapped to prompt/completion: %+v", resp.Usage)
	}
	if format, ok := gotBody["text"].(map[string]any); !ok || format["format"].(map[string]any)["type"] != "json_object" {
		t.Fatalf("responses JSON mode not requested: %v", gotBody["text"])
	}
	if _, ok := gotBody["max_output_tokens"]; !ok {
		t.Fatalf("responses request missing max_output_tokens: %v", gotBody)
	}
	// The extracted content must be the actor-decision object, and it must map
	// to the same internal decision type the chat wire produces.
	decision, err := cognition.ValidateActorSeamContent(resp.Content, sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Source != cognition.SourceModel || decision.Action != "gather" {
		t.Fatalf("responses content did not map to a model ActorSeamDecision: %+v (content %q)", decision, resp.Content)
	}

	// A Responses body carrying the Role B schema maps to PatchProposal too.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(responsesWireBody(strategyPatchJSON, 10, 5))
	}))
	defer srv2.Close()
	gw2 := gatewayForTest(t, srv2, cognition.WireResponses)
	resp2, err := gw2.Complete(context.Background(), cognition.ModelRequest{
		Endpoint: gw2.WireEndpoint(), Sampling: cognition.FrozenSampling(), Request: []byte(`{"schema_version":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := cognition.DecodeStrategyPatchContent(resp2.Content, protocol.PatchRequest{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !proposal.Propose || len(proposal.Deltas) != 1 || proposal.Deltas[0].Param != protocol.ParamClaimGatherEnergyFloor {
		t.Fatalf("responses content did not map to a PatchProposal: %+v", proposal)
	}
}

// TestGatewayWireEndpointNamesTheWireMode pins that the recorded endpoint
// host+path names the wire, so the ledger records which wire was used without a
// new record field.
func TestGatewayWireEndpointNamesTheWireMode(t *testing.T) {
	for _, tc := range []struct {
		mode cognition.WireMode
		want string
	}{
		{cognition.WireChatCompletions, "/v1/chat/completions"},
		{cognition.WireResponses, "/v1/responses"},
	} {
		gw, err := cognition.NewGatewayClient(cognition.GatewayConfig{BaseURL: "http://127.0.0.1:4000", WireMode: tc.mode})
		if err != nil {
			t.Fatal(err)
		}
		if gw.Endpoint() != "127.0.0.1:4000/" {
			t.Fatalf("recorded endpoint = %q", gw.Endpoint())
		}
		if gw.WireEndpoint() != "127.0.0.1:4000"+tc.want {
			t.Fatalf("wire endpoint for %s = %q, want suffix %q", tc.mode, gw.WireEndpoint(), tc.want)
		}
		if gw.WireMode() != tc.mode {
			t.Fatalf("wire mode = %q", gw.WireMode())
		}
	}
}

// TestGatewayRejectsMismatchedWireEndpoint pins that the recorded endpoint
// cannot silently disagree with the wire actually used.
func TestGatewayRejectsMismatchedWireEndpoint(t *testing.T) {
	gw, err := cognition.NewGatewayClient(cognition.GatewayConfig{
		BaseURL: "http://127.0.0.1:4000", Authorized: true, HTTPClient: &http.Client{}, WireMode: cognition.WireResponses,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw.Complete(context.Background(), cognition.ModelRequest{
		Endpoint: "127.0.0.1:4000/v1/chat/completions", Request: []byte("{}"),
	})
	if !errors.Is(err, cognition.ErrInvalidModelRequest) {
		t.Fatalf("mismatched wire endpoint accepted: %v", err)
	}
}

// --- recorder-level typed outcomes -----------------------------------------

func TestGatewayHTTPStatusIsTypedAndRetriedOnlyWhenDeclared(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(chatWireBody(actorDecisionJSON, 3, 1))
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireChatCompletions)

	l, _ := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
	res, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempt != 2 || res.Status != cognition.CallStatusOK || attempts != 2 {
		t.Fatalf("429 was not retried through the recorder: attempts=%d res=%+v", attempts, res)
	}
	if l.Count() != 2 {
		t.Fatalf("every gateway attempt must be recorded: %d", l.Count())
	}

	// A 400 is terminal.
	srv400 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv400.Close()
	gw400 := gatewayForTest(t, srv400, cognition.WireChatCompletions)
	l2, path2 := newTestLedger(t)
	rec2 := recorderWith(t, l2, gw400, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
	res2, err := rec2.Call(context.Background(), gatewayCall(gw400, cognition.RoleActorDecision, 6))
	if err != nil {
		t.Fatal(err)
	}
	if res2.Status != cognition.CallStatusHTTPError || l2.Count() != 1 {
		t.Fatalf("400 not terminal: res=%+v records=%d", res2, l2.Count())
	}
	records, err := cognition.ReadCallRecords(path2)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].HTTPStatus != 400 {
		t.Fatalf("400 http_status not recorded: %+v", records)
	}
}

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

func TestGatewayTransportErrorIsTypedAndRecorded(t *testing.T) {
	gw, err := cognition.NewGatewayClient(cognition.GatewayConfig{
		BaseURL: "http://127.0.0.1:4000", Authorized: true, CredentialPresent: true, WireMode: cognition.WireResponses,
		HTTPClient: &http.Client{Transport: failingTransport{err: errors.New("dial refused")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	l, path := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{MaxAttempts: 1})
	_, err = rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if !errors.Is(err, cognition.ErrModelUnavailable) {
		t.Fatalf("exhausted transport error must be the typed halt: %v", err)
	}
	records, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != cognition.CallStatusTransportError {
		t.Fatalf("transport error not recorded: %+v", records)
	}
}

func TestGatewayTimeoutIsTypedAndRecorded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(300 * time.Millisecond):
			_, _ = w.Write(chatWireBody(actorDecisionJSON, 1, 1))
		}
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireChatCompletions)
	l, path := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{
		MaxAttempts: 1, AttemptTimeout: 50 * time.Millisecond, CallDeadline: 2 * time.Second,
	})
	_, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if !errors.Is(err, cognition.ErrModelUnavailable) {
		t.Fatalf("timeout must halt typed after exhaustion: %v", err)
	}
	records, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != cognition.CallStatusTimeout {
		t.Fatalf("timeout not recorded as timeout: %+v", records)
	}
}

func TestGatewayMalformedBodyIsRecordedAndNeverRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireChatCompletions)
	l, _ := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{MaxAttempts: 3})
	res, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != cognition.CallStatusMalformed || l.Count() != 1 {
		t.Fatalf("malformed completion must be terminal and recorded once: res=%+v records=%d", res, l.Count())
	}
}

func TestGatewayOversizedBodyIsRecordedByDigestAndLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(chatWireBody(strings.Repeat("x", cognition.MaxResponseBytes), 1, 1))
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireChatCompletions)
	l, path := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
	res, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != cognition.CallStatusOversized || res.ResponseBody != nil {
		t.Fatalf("oversized body not replaced by digest: %+v", res)
	}
	records, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ResponseDigest == "" || records[0].ResponseLength <= cognition.MaxResponseBytes {
		t.Fatalf("oversized record missing digest/length: %+v", records)
	}
	if !records[0].ResponseTruncatedPrefix {
		t.Fatalf("oversized digest/length must be labelled a truncated prefix: %+v", records[0])
	}
}

// --- cost and identity ------------------------------------------------------

// TestGatewayUnpricedCostIsTypedNotZero pins requirement 4: a model with no
// price entry records the typed unpriced state (price_version "unpriced",
// cost_micros 0) rather than a fabricated price.
func TestGatewayUnpricedCostIsTypedNotZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(chatWireBody(actorDecisionJSON, 100, 50))
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireChatCompletions)
	l, path := newTestLedger(t)
	rec := recorderWith(t, l, gw, cognition.UnpricedPrice{}, newAccount(t, generousBudget()), cognition.RecorderConfig{})
	res, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if err != nil {
		t.Fatal(err)
	}
	if !res.CostUnpriced || res.CostMicros != 0 {
		t.Fatalf("unpriced cost not typed: %+v", res)
	}
	records, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].PriceVersion != cognition.UnpricedPriceVersion || records[0].CostMicros != 0 {
		t.Fatalf("unpriced state not recorded: %+v", records[0])
	}
	// A priced table still records a real price (the pluggable surface works).
	l2, path2 := newTestLedger(t)
	rec2 := recorderWith(t, l2, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
	if _, err := rec2.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6)); err != nil {
		t.Fatal(err)
	}
	priced, _ := cognition.ReadCallRecords(path2)
	if len(priced) != 1 || priced[0].PriceVersion != "test/1" || priced[0].CostMicros != 100*1+50*2 {
		t.Fatalf("priced cost not recorded: %+v", priced[0])
	}
}

// TestGatewayRecordsConfiguredModelAndWireEndpoint pins requirement 3: the
// configured wire model and the wire endpoint host+path reach the ledger, and
// the hermetic path is unchanged (the fake transport reports no identity).
func TestGatewayRecordsConfiguredModelAndWireEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(responsesWireBody(actorDecisionJSON, 1, 1))
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireResponses)
	l, path := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
	if _, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6)); err != nil {
		t.Fatal(err)
	}
	records, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d", len(records))
	}
	if records[0].ModelID != cognition.DefaultGatewayModel {
		t.Fatalf("recorded model = %q, want %q", records[0].ModelID, cognition.DefaultGatewayModel)
	}
	if records[0].Endpoint != gw.WireEndpoint() || !strings.Contains(records[0].Endpoint, "/v1/responses") {
		t.Fatalf("recorded endpoint = %q", records[0].Endpoint)
	}
	if !records[0].CredentialPresent {
		t.Fatal("credential_present not recorded")
	}

	// The fake path records the request's declared model and no secret, and its
	// bytes are unchanged.
	l2, path2 := newTestLedger(t)
	if _, err := newRecorder(t, l2, &cognition.FakeClient{}, newAccount(t, generousBudget())).Call(
		context.Background(), sampleRequest(cognition.RoleActorDecision, `{"schema_version":1}`, 6)); err != nil {
		t.Fatal(err)
	}
	fakeRecords, err := cognition.ReadCallRecords(path2)
	if err != nil {
		t.Fatal(err)
	}
	if fakeRecords[0].ModelID != cognition.DefaultModelID {
		t.Fatalf("fake model changed: %q", fakeRecords[0].ModelID)
	}
}

// --- probe discovery --------------------------------------------------------

// TestDiscoverGatewayContractParsesMetadataOnly pins the probe parse against the
// discovered application-model shape, including the underscore model id, the
// responses-only endpoint set, and the absent cost fields (unpriced).
func TestDiscoverGatewayContractParsesMetadataOnly(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("probe path = %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"github_copilot/gpt-6-luna","model_info":{
			"mode":"responses","supported_endpoints":["/v1/responses"],
			"max_input_tokens":272000,"max_output_tokens":128000,
			"supports_response_schema":true,"supports_reasoning":true,
			"supports_streaming":true,"supports_function_calling":true}}]}`))
	}))
	defer srv.Close()

	contract, err := cognition.DiscoverGatewayContract(context.Background(), cognition.GatewayConfig{
		BaseURL: srv.URL, APIKey: "test-key", Authorized: true, HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("probe auth = %q", gotAuth)
	}
	model, ok := contract.Model("github_copilot/gpt-6-luna")
	if !ok {
		t.Fatalf("application model not discovered: %+v", contract.Models)
	}
	if model.Mode != "responses" || model.MaxInputTokens != 272000 || model.MaxOutputTokens != 128000 ||
		!model.SupportsResponseSchema || !model.SupportsReasoning || !model.SupportsStreaming || !model.SupportsFunctionCalling {
		t.Fatalf("model contract not parsed: %+v", model)
	}
	if model.Priced {
		t.Fatal("a model without cost fields must be discovered unpriced")
	}
	mode, err := cognition.WireModeForEndpoints(model.SupportedEndpoints)
	if err != nil {
		t.Fatal(err)
	}
	if mode != cognition.WireResponses {
		t.Fatalf("wire mode from endpoints = %q", mode)
	}
	if contract.Endpoint == "" || strings.Contains(contract.Endpoint, srv.URL) {
		t.Fatalf("probe endpoint must be host+path only: %q", contract.Endpoint)
	}
}

func TestWireModeAndOutputModeValidation(t *testing.T) {
	if _, err := cognition.ParseWireMode("bogus"); !errors.Is(err, cognition.ErrInvalidModelRequest) {
		t.Fatalf("bogus wire mode accepted: %v", err)
	}
	if _, err := cognition.ParseWireMode("responses"); err != nil {
		t.Fatal(err)
	}
	if _, err := cognition.NewGatewayClient(cognition.GatewayConfig{
		BaseURL: "http://127.0.0.1:4000", OutputMode: cognition.OutputMode("bogus"),
	}); !errors.Is(err, cognition.ErrInvalidModelRequest) {
		t.Fatalf("bogus output mode accepted: %v", err)
	}
	if _, err := cognition.WireModeForEndpoints([]string{"/v1/embeddings"}); !errors.Is(err, cognition.ErrInvalidModelRequest) {
		t.Fatalf("unknown endpoints accepted: %v", err)
	}
}

// TestGatewayEmptyResponsesMappingFallsBack is the negative counterpart of the
// Responses mapping: when the output_text cannot be found, content is empty and
// the level-1 ladder takes the declared fallback, proving the test above can
// detect a broken extraction.
func TestGatewayEmptyResponsesMappingFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[]}],"usage":{}}`))
	}))
	defer srv.Close()
	gw := gatewayForTest(t, srv, cognition.WireResponses)
	l, _ := newTestLedger(t)
	rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
	res, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6))
	if err != nil {
		t.Fatal(err)
	}
	decision, err := cognition.ValidateActorSeamContent(res.Content, sampleSeamRequest())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Source != cognition.SourceFallback || decision.Reason != string(cognition.FallbackReasonMalformed) {
		t.Fatalf("empty responses content must fall back: %+v", decision)
	}
}

// --- repair round (ticket 22 v2): output mode, identity, probe gate --------

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestDiscoverGatewayContractRefusesUnauthorizedWithoutNetwork pins that the
// metadata probe itself refuses an unauthorized config with a typed error and
// never opens a connection.
func TestDiscoverGatewayContractRefusesUnauthorizedWithoutNetwork(t *testing.T) {
	called := false
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("network must not be touched")
	})}
	_, err := cognition.DiscoverGatewayContract(context.Background(), cognition.GatewayConfig{
		BaseURL: "http://127.0.0.1:4000", Authorized: false, HTTPClient: client,
	})
	if !errors.Is(err, cognition.ErrGatewayUnauthorized) {
		t.Fatalf("unauthorized discovery accepted: %v", err)
	}
	if called {
		t.Fatal("unauthorized discovery opened a connection")
	}
}

// TestGatewayOutputPlainOmitsJSONFormat pins the plain output mode on both
// wires: the chat wire omits response_format and the Responses wire omits
// text/format.
func TestGatewayOutputPlainOmitsJSONFormat(t *testing.T) {
	for _, tc := range []struct {
		mode      cognition.WireMode
		formatKey string
	}{
		{cognition.WireChatCompletions, "response_format"},
		{cognition.WireResponses, "text"},
	} {
		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			if tc.mode == cognition.WireChatCompletions {
				_, _ = w.Write(chatWireBody(actorDecisionJSON, 1, 1))
			} else {
				_, _ = w.Write(responsesWireBody(actorDecisionJSON, 1, 1))
			}
		}))
		gw, err := cognition.NewGatewayClient(cognition.GatewayConfig{
			BaseURL: srv.URL, Authorized: true, WireMode: tc.mode,
			Model: cognition.DefaultGatewayModel, OutputMode: cognition.OutputPlain, HTTPClient: srv.Client(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := gw.Complete(context.Background(), cognition.ModelRequest{
			Endpoint: gw.WireEndpoint(), Sampling: cognition.FrozenSampling(), Request: []byte(`{"schema_version":1}`),
		}); err != nil {
			t.Fatal(err)
		}
		if _, ok := gotBody[tc.formatKey]; ok {
			t.Fatalf("%s wire in plain mode requested %q: %v", tc.mode, tc.formatKey, gotBody)
		}
		srv.Close()
	}
}

// TestGatewayRecordsRequestedOutputMode pins that the requested JSON output
// mode (json_object or plain text) reaches the ledger per call, while the
// hermetic fake transport records none and its bytes stay unchanged.
func TestGatewayRecordsRequestedOutputMode(t *testing.T) {
	for _, tc := range []struct {
		output cognition.OutputMode
		want   string
	}{
		{cognition.OutputJSON, string(cognition.OutputJSON)},
		{cognition.OutputPlain, string(cognition.OutputPlain)},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(chatWireBody(actorDecisionJSON, 1, 1))
		}))
		gw, err := cognition.NewGatewayClient(cognition.GatewayConfig{
			BaseURL: srv.URL, Authorized: true, CredentialPresent: true, WireMode: cognition.WireChatCompletions,
			OutputMode: tc.output, HTTPClient: srv.Client(),
		})
		if err != nil {
			t.Fatal(err)
		}
		l, path := newTestLedger(t)
		rec := recorderWith(t, l, gw, testPrice(), newAccount(t, generousBudget()), cognition.RecorderConfig{})
		if _, err := rec.Call(context.Background(), gatewayCall(gw, cognition.RoleActorDecision, 6)); err != nil {
			t.Fatal(err)
		}
		records, err := cognition.ReadCallRecords(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 1 || records[0].OutputMode != tc.want {
			t.Fatalf("requested output mode %s not recorded: %+v", tc.output, records)
		}
		srv.Close()
	}

	// The fake declares no output mode, so its record stays empty.
	l, path := newTestLedger(t)
	if _, err := newRecorder(t, l, &cognition.FakeClient{}, newAccount(t, generousBudget())).Call(
		context.Background(), sampleRequest(cognition.RoleActorDecision, `{"schema_version":1}`, 6)); err != nil {
		t.Fatal(err)
	}
	fakeRecords, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fakeRecords) != 1 || fakeRecords[0].OutputMode != "" {
		t.Fatalf("fake path must record no output mode: %+v", fakeRecords)
	}
}

// TestModelIdentityDigestBindsModelAndWire pins the P1 repair: the recorded
// model identity digest is derived from the declared model and wire, so two
// runs on different models or wires get different digests.
func TestModelIdentityDigestBindsModelAndWire(t *testing.T) {
	sampling := cognition.FrozenSampling()
	base := cognition.ModelIdentityDigest(cognition.DefaultModelID, cognition.WireChatCompletions, sampling)
	if base != cognition.ModelIdentityDigest(cognition.DefaultModelID, cognition.WireChatCompletions, sampling) {
		t.Fatal("identity digest is not deterministic")
	}
	if base == cognition.ModelIdentityDigest("other/model", cognition.WireChatCompletions, sampling) {
		t.Fatal("two models share an identity digest")
	}
	if base == cognition.ModelIdentityDigest(cognition.DefaultModelID, cognition.WireResponses, sampling) {
		t.Fatal("two wires share an identity digest")
	}
	if base != cognition.ModelIdentityDigest(cognition.DefaultModelID, "", sampling) {
		t.Fatal("the empty wire is not the declared chat default")
	}
	// One declared application-model identity: the gateway-exposed underscore id.
	if cognition.DefaultModelID != cognition.DefaultGatewayModel {
		t.Fatalf("DefaultModelID %q and DefaultGatewayModel %q differ",
			cognition.DefaultModelID, cognition.DefaultGatewayModel)
	}
	if cognition.DefaultModelID != "github_copilot/gpt-6-luna" {
		t.Fatalf("declared application model = %q", cognition.DefaultModelID)
	}
}

type fixedPricer struct {
	version string
	cost    cognition.Cost
}

func (p fixedPricer) PriceVersion() string                         { return p.version }
func (p fixedPricer) Cost(cognition.Usage) (cognition.Cost, error) { return p.cost, nil }
func (p fixedPricer) Validate() error                              { return nil }

// TestRecorderRejectsInconsistentPricer pins that the typed Cost.Unpriced state
// and the reserved `unpriced` price version must agree, so the recorded
// unpriced flag cannot silently disagree with the pricer.
func TestRecorderRejectsInconsistentPricer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pricer fixedPricer
	}{
		{"unpriced with a priced version", fixedPricer{version: "test/1", cost: cognition.Cost{Unpriced: true}}},
		{"priced with the reserved unpriced version", fixedPricer{version: cognition.UnpricedPriceVersion, cost: cognition.Cost{Micros: 5}}},
	} {
		l, _ := newTestLedger(t)
		rec, err := cognition.NewRecorder(cognition.RecorderConfig{
			Ledger: l, Client: &cognition.FakeClient{}, Budget: newAccount(t, generousBudget()),
			Price: tc.pricer, Now: func() time.Time { return time.UnixMicro(1_700_000_000_000_000) },
			Sleep: func(context.Context, time.Duration) error { return nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rec.Call(context.Background(),
			sampleRequest(cognition.RoleActorDecision, `{"schema_version":1}`, 6)); !errors.Is(err, cognition.ErrInvalidModelRequest) {
			t.Fatalf("%s: inconsistent pricer accepted: %v", tc.name, err)
		}
	}
}
