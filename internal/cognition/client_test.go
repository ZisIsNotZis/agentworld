package cognition_test

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentworld/internal/cognition"
)

func testPrice() cognition.PriceTable {
	return cognition.PriceTable{Version: "test/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2}
}

func generousBudget() cognition.CognitiveBudget {
	return cognition.CognitiveBudget{
		MaxCalls: 1000, MaxPromptTokens: 1_000_000, MaxCompletionTokens: 1_000_000, MaxCostMicros: 10_000_000,
	}
}

func fixedRecorderConfig(l *cognition.Ledger, client cognition.ModelClient, acct *cognition.CognitiveAccount) cognition.RecorderConfig {
	return cognition.RecorderConfig{
		Ledger: l, Client: client, Budget: acct, Price: testPrice(),
		Now:   func() time.Time { return time.UnixMicro(1_700_000_000_000_000) },
		Sleep: func(context.Context, time.Duration) error { return nil },
	}
}

func newRecorder(t *testing.T, l *cognition.Ledger, client cognition.ModelClient, acct *cognition.CognitiveAccount) *cognition.Recorder {
	t.Helper()
	r, err := cognition.NewRecorder(fixedRecorderConfig(l, client, acct))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func sampleRequest(role, body string, hour int) cognition.CallRequest {
	return cognition.CallRequest{
		Role: role, Hour: hour, Model: cognition.DefaultModelID,
		Endpoint: "gw.example/v1/chat/completions", CredentialPresent: true,
		Sampling: cognition.FrozenSampling(), Request: []byte(body),
	}
}

func newAccount(t *testing.T, b cognition.CognitiveBudget) *cognition.CognitiveAccount {
	t.Helper()
	a, err := cognition.NewCognitiveAccount(b)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRecorderFakeIsDeterministicAndHermetic(t *testing.T) {
	l0, path0 := newTestLedger(t)
	l1, path1 := newTestLedger(t)
	req := sampleRequest(cognition.RoleActorDecision, `{"schema_version":1}`, 6)
	res0, err := newRecorder(t, l0, &cognition.FakeClient{}, newAccount(t, generousBudget())).Call(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := newRecorder(t, l1, &cognition.FakeClient{}, newAccount(t, generousBudget())).Call(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res0.Status != cognition.CallStatusOK || !bytes.Equal(res0.Content, res1.Content) || res0.Usage != res1.Usage {
		t.Fatalf("fake is not deterministic: %+v %+v", res0, res1)
	}
	d0, _ := os.ReadFile(path0)
	d1, _ := os.ReadFile(path1)
	if !bytes.Equal(d0, d1) {
		t.Fatalf("identical fake calls produced different ledger bytes:\n%s\n%s", d0, d1)
	}
}

func TestRecorderRetriesTransportErrorsThenUsesSuccess(t *testing.T) {
	l, _ := newTestLedger(t)
	attempts := 0
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		attempts++
		if attempts < 3 {
			return cognition.ModelResponse{}, errors.New("transport down")
		}
		return cognition.DefaultFakeResponse(), nil
	}}
	res, err := newRecorder(t, l, fake, newAccount(t, generousBudget())).Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":1}`, 6))
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempt != 3 || res.Status != cognition.CallStatusOK {
		t.Fatalf("used attempt %+v", res)
	}
	if l.Count() != 3 {
		t.Fatalf("expected every attempt recorded, got %d", l.Count())
	}
}

func TestRecorderRetriesHTTP429And5xxOnly(t *testing.T) {
	l, _ := newTestLedger(t)
	attempts := 0
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		attempts++
		if attempts == 1 {
			return cognition.ModelResponse{}, &cognition.HTTPStatusError{Status: 429}
		}
		return cognition.DefaultFakeResponse(), nil
	}}
	res, err := newRecorder(t, l, fake, newAccount(t, generousBudget())).Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":2}`, 6))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || res.Status != cognition.CallStatusOK {
		t.Fatalf("429 was not retried: %d %+v", attempts, res)
	}
	// A 400 is terminal.
	l2, _ := newTestLedger(t)
	terminal := 0
	fake2 := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		terminal++
		return cognition.ModelResponse{}, &cognition.HTTPStatusError{Status: 400}
	}}
	res2, err := newRecorder(t, l2, fake2, newAccount(t, generousBudget())).Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":3}`, 6))
	if err != nil {
		t.Fatal(err)
	}
	if terminal != 1 || res2.Status != cognition.CallStatusHTTPError {
		t.Fatalf("400 must be terminal and recorded: %d %+v", terminal, res2)
	}
}

func TestRecorderNeverRetriesSchemaInvalidCompletion(t *testing.T) {
	l, _ := newTestLedger(t)
	attempts := 0
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		attempts++
		return cognition.ModelResponse{Body: []byte("not json"), Content: []byte("not json"), Status: cognition.CallStatusMalformed}, nil
	}}
	res, err := newRecorder(t, l, fake, newAccount(t, generousBudget())).Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":4}`, 6))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || res.Status != cognition.CallStatusMalformed {
		t.Fatalf("malformed completion was retried: %d %+v", attempts, res)
	}
}

func TestRecorderExhaustedRetriesHaltTyped(t *testing.T) {
	l, _ := newTestLedger(t)
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		return cognition.ModelResponse{}, errors.New("transport down")
	}}
	_, err := newRecorder(t, l, fake, newAccount(t, generousBudget())).Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":5}`, 6))
	if !errors.Is(err, cognition.ErrModelUnavailable) {
		t.Fatalf("want ErrModelUnavailable, got %v", err)
	}
	if l.Count() != cognition.DefaultMaxAttempts {
		t.Fatalf("every failed attempt must be recorded: %d", l.Count())
	}
}

func TestRecorderRecordsOversizedByDigestAndLength(t *testing.T) {
	l, path := newTestLedger(t)
	big := bytes.Repeat([]byte("x"), cognition.MaxResponseBytes+1)
	fake := &cognition.FakeClient{Respond: func(cognition.ModelRequest) (cognition.ModelResponse, error) {
		return cognition.ModelResponse{Body: big, Content: big, Status: cognition.CallStatusOK}, nil
	}}
	res, err := newRecorder(t, l, fake, newAccount(t, generousBudget())).Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":6}`, 6))
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
	if len(records) != 1 || records[0].ResponseLength != len(big) || records[0].ResponseDigest == "" {
		t.Fatalf("oversized record missing digest/length: %+v", records)
	}
}

func TestRecorderBudgetCeilingHaltsBeforeSpend(t *testing.T) {
	l, _ := newTestLedger(t)
	fake := &cognition.FakeClient{}
	acct := newAccount(t, cognition.CognitiveBudget{MaxCalls: 1, MaxPromptTokens: 1000, MaxCompletionTokens: 1000, MaxCostMicros: 1_000_000})
	rec := newRecorder(t, l, fake, acct)
	if _, err := rec.Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":7}`, 6)); err != nil {
		t.Fatal(err)
	}
	_, err := rec.Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":8}`, 6))
	var hit *cognition.CognitiveCeilingHitError
	if !errors.As(err, &hit) || hit.Hit.Ceiling != cognition.CognitiveCeilingCalls {
		t.Fatalf("want a typed call-ceiling halt, got %v", err)
	}
	if fake.Calls() != 1 {
		t.Fatalf("ceiling did not refuse the spend: %d calls", fake.Calls())
	}
	if l.Count() != 1 {
		t.Fatalf("refused call must not be recorded: %d", l.Count())
	}
}

func TestRecorderCostCeilingHaltsWithRecordPersisted(t *testing.T) {
	l, _ := newTestLedger(t)
	price := cognition.PriceTable{Version: "test/high", PromptMicrosPerToken: 1000, CompletionMicrosPerToken: 1000}
	acct := newAccount(t, cognition.CognitiveBudget{MaxCalls: 10, MaxPromptTokens: 1_000_000, MaxCompletionTokens: 1_000_000, MaxCostMicros: 100})
	rec, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: l, Client: &cognition.FakeClient{}, Budget: acct, Price: price,
		Now: func() time.Time { return time.UnixMicro(1) }, Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = rec.Call(context.Background(), sampleRequest(cognition.RoleActorDecision, `{"r":9}`, 6))
	var hit *cognition.CognitiveCeilingHitError
	if !errors.As(err, &hit) || hit.Hit.Ceiling != cognition.CognitiveCeilingCost {
		t.Fatalf("want a typed cost-ceiling halt, got %v", err)
	}
	if l.Count() != 1 {
		t.Fatalf("the crossing call record must be persisted: %d", l.Count())
	}
}

func TestRecorderRejectsInvalidRequestAndNilClient(t *testing.T) {
	l, _ := newTestLedger(t)
	rec := newRecorder(t, l, &cognition.FakeClient{}, newAccount(t, generousBudget()))
	if _, err := rec.Call(context.Background(), cognition.CallRequest{Role: cognition.RoleActorDecision}); !errors.Is(err, cognition.ErrInvalidModelRequest) {
		t.Fatalf("invalid request accepted: %v", err)
	}
	if _, err := cognition.NewRecorder(cognition.RecorderConfig{Ledger: l, Budget: newAccount(t, generousBudget()), Price: testPrice()}); !errors.Is(err, cognition.ErrModelClient) {
		t.Fatalf("nil client accepted: %v", err)
	}
}

func TestGatewayStubIsConfigGatedAndCannotReachNetwork(t *testing.T) {
	stub, err := cognition.NewGatewayClient(cognition.GatewayConfig{BaseURL: "https://gateway.example/v1/?token=secret", CredentialPresent: true})
	if err != nil {
		t.Fatal(err)
	}
	if stub.Endpoint() != "gateway.example/v1/" {
		t.Fatalf("endpoint must be host+path with no query: %q", stub.Endpoint())
	}
	if _, err := stub.Complete(context.Background(), cognition.ModelRequest{}); !errors.Is(err, cognition.ErrGatewayUnauthorized) {
		t.Fatalf("unauthorized stub did not refuse: %v", err)
	}
	authorized, err := cognition.NewGatewayClient(cognition.GatewayConfig{BaseURL: "https://gateway.example/v1", Authorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authorized.Complete(context.Background(), cognition.ModelRequest{}); !errors.Is(err, cognition.ErrGatewayUnimplemented) {
		t.Fatalf("authorized stub must still refuse (no transport): %v", err)
	}
}

// gatewayNetworkFile is the package's single declared network boundary: the
// real LiteLLM gateway transport. Every other source file stays network-free,
// so the fake, replay, ledger, and recorder paths cannot reach the network; the
// world import is forbidden everywhere.
const gatewayNetworkFile = "gateway.go"

// TestCognitionSourcesImportNoNetworkOrWorld is the structural privacy and
// hermeticity guard: the only network dependency in the package is the
// declared gateway transport, so every hermetic path cannot make a network
// call, and the world package cannot import the client.
func TestCognitionSourcesImportNoNetworkOrWorld(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	networkFiles := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if path == "agentworld/internal/world" {
				t.Fatalf("%s imports internal/world: the observation boundary is violated", name)
			}
			if path == "net" || strings.HasPrefix(path, "net/") {
				if name != gatewayNetworkFile {
					t.Fatalf("%s imports %q: only %s may depend on the network", name, path, gatewayNetworkFile)
				}
				networkFiles[name] = true
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files checked")
	}
	if !networkFiles[gatewayNetworkFile] {
		t.Fatalf("the declared network boundary %s imports no network package", gatewayNetworkFile)
	}
}
