package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
)

// The claim-call concurrency tests (ticket 57). A full R@max reference cell is
// ~1,434 Role A calls at 5-10 s each, so the calls within one simulated hour
// must be issued at the same time. The hour's decisions are independent — each
// request is built from its actor's own observation before any call is issued —
// and the responses are applied serially in the declared wake order, so the
// recorded world must be byte-identical to the serial run. These tests pin that:
//
//   - the concurrent run overlaps its calls in wall time (a 50 ms fake latency),
//   - its journal, checkpoints and per-actor decisions equal the serial run's,
//   - a ledger replay (zero model calls) reproduces the same world,
//   - the serial default and a concurrency larger than the selected set are safe,
//   - a priority-bearing arm stays serial, and
//   - a typed ceiling hit still halts the run exactly once.
const (
	claimConcurrencyTestHours   = 8
	claimConcurrencyTestFounder = 8
	claimConcurrencyTestLatency = 50 * time.Millisecond
)

// claimConcurrencyAction is the deterministic per-actor scripted answer, chosen
// from the actor's own request: gather when the observation shows a stocked
// home slot (the frozen rule's own target), otherwise wait. Two different
// actors therefore receive different actions, so a response swapped between
// actors by an out-of-order apply would change the world and fail the equality.
func claimConcurrencyAction(req cognition.ModelRequest) (string, error) {
	var wire struct {
		Observation struct {
			HomeSlots []struct {
				Stock int64 `json:"stock"`
			} `json:"home_slots"`
		} `json:"observation"`
	}
	if err := json.Unmarshal(req.Request, &wire); err != nil {
		return "", err
	}
	for _, slot := range wire.Observation.HomeSlots {
		if slot.Stock == 1 {
			return "gather", nil
		}
	}
	return "wait", nil
}

// claimConcurrencyResponder is a hermetic responder with a declared latency and
// a served-call counter.
func claimConcurrencyResponder(latency time.Duration, served *atomic.Int64) func(cognition.ModelRequest) (cognition.ModelResponse, error) {
	return func(req cognition.ModelRequest) (cognition.ModelResponse, error) {
		action, err := claimConcurrencyAction(req)
		if err != nil {
			return cognition.ModelResponse{}, err
		}
		time.Sleep(latency)
		served.Add(1)
		body := []byte(fmt.Sprintf(`{"schema_version":1,"action":%q,"rationale":"scripted","confidence":1}`, action))
		return cognition.ModelResponse{
			Body: body, Content: body,
			Usage: cognition.Usage{PromptTokens: 4, CompletionTokens: 2}, Status: cognition.CallStatusOK,
		}, nil
	}
}

// newClaimConcurrencyWorld builds a full-wake (nil Priority) runner with the
// declared claim concurrency.
func newClaimConcurrencyWorld(t *testing.T, concurrency int, responder func(cognition.ModelRequest) (cognition.ModelResponse, error)) (*Natural, *cognition.Ledger) {
	t.Helper()
	l := naturalCognitionTestLedger(t)
	acct, err := cognition.NewCognitiveAccount(cognition.CognitiveBudget{
		MaxCalls: 1_000_000, MaxPromptTokens: 1_000_000_000,
		MaxCompletionTokens: 1_000_000_000, MaxCostMicros: 1_000_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: l, Client: &cognition.FakeClient{Respond: responder}, Budget: acct,
		Price: cognition.PriceTable{Version: "test/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2},
		Now:   func() time.Time { return time.UnixMicro(1) },
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	wake := cognition.FullWakePolicy()
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: rec, Ledger: l, Endpoint: "gw.example/v1/chat/completions",
		CredentialPresent: true, WakePolicy: wake,
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := naturalCognitionTestOptions()
	opts.Founders = claimConcurrencyTestFounder
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: wake, ClaimConcurrency: concurrency}
	f, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	return f, l
}

// claimConcurrencyJournal exports the runner's journal, which reconciles it
// against a trusted ledger-resolving replay: a successful export already proves
// the recorded decisions reproduce the world without a model call.
func claimConcurrencyJournal(t *testing.T, f *Natural, l *cognition.Ledger) []byte {
	t.Helper()
	replay, err := cognition.OpenReplay(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	seam, err := cognition.NewReplaySeam(replay, cognition.FullWakePolicy())
	if err != nil {
		t.Fatal(err)
	}
	journal, err := f.ExportJournalWithReplay(seam, cognition.FullWakePolicy())
	if err != nil {
		t.Fatalf("journal reconciliation (ledger replay) failed: %v", err)
	}
	return journal
}

// claimConcurrencyDecisions is the per-actor-hour decision map of a ledger: the
// applied order the world sees, keyed by (hour, actor). THE MAP IS THE PIN: two
// runs may write the ledger in any order under concurrency, but every actor must
// receive its own decision.
func claimConcurrencyDecisions(t *testing.T, l *cognition.Ledger) map[string]string {
	t.Helper()
	interventions, err := cognition.ReadInterventionRecords(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, rec := range interventions {
		if rec.Kind != cognition.InterventionRoleA || rec.RoleA == nil {
			continue
		}
		out[fmt.Sprintf("%d/%d", rec.Cause.Hour, rec.Cause.Actor)] = rec.RoleA.Source + ":" + rec.RoleA.Action
	}
	return out
}

// TestNaturalClaimConcurrencyDeterministic is the ticket-57 acceptance test:
// serial and concurrent runs of the same fixture produce the identical world,
// and the concurrent run's wall clock overlaps its calls.
func TestNaturalClaimConcurrencyDeterministic(t *testing.T) {
	var serialServed, concurrentServed atomic.Int64
	serial, serialLedger := newClaimConcurrencyWorld(t, 1, claimConcurrencyResponder(claimConcurrencyTestLatency, &serialServed))
	start := time.Now()
	naturalRunTo(t, serial, claimConcurrencyTestHours)
	serialElapsed := time.Since(start)

	concurrent, concurrentLedger := newClaimConcurrencyWorld(t, claimConcurrencyTestFounder, claimConcurrencyResponder(claimConcurrencyTestLatency, &concurrentServed))
	start = time.Now()
	naturalRunTo(t, concurrent, claimConcurrencyTestHours)
	concurrentElapsed := time.Since(start)

	// (a) The calls overlap: a serial hour costs one latency per actor, a
	// concurrent hour costs roughly one latency for the whole batch.
	if serialServed.Load() != concurrentServed.Load() {
		t.Fatalf("served calls serial %d, concurrent %d", serialServed.Load(), concurrentServed.Load())
	}
	if concurrentElapsed*2 > serialElapsed {
		t.Fatalf("no overlap: serial %s, concurrent %s for %d calls",
			serialElapsed, concurrentElapsed, serialServed.Load())
	}

	// (b) The world is identical: journal bytes and hourly checkpoints.
	serialJournal := claimConcurrencyJournal(t, serial, serialLedger)
	concurrentJournal := claimConcurrencyJournal(t, concurrent, concurrentLedger)
	if !bytes.Equal(serialJournal, concurrentJournal) {
		t.Fatal("concurrent journal differs from the serial run")
	}
	if !reflect.DeepEqual(serial.Checkpoints(), concurrent.Checkpoints()) {
		t.Fatal("concurrent checkpoints differ from the serial run")
	}

	// The apply order is pinned per actor-hour: the full-wake reference selects
	// every actor each hour, and every actor's own scripted decision is applied
	// to its own actor in both runs.
	serialDecisions := claimConcurrencyDecisions(t, serialLedger)
	concurrentDecisions := claimConcurrencyDecisions(t, concurrentLedger)
	for hour := 0; hour < claimConcurrencyTestHours; hour++ {
		got := 0
		for key := range serialDecisions {
			var h int
			if _, err := fmt.Sscanf(key, "%d/", &h); err == nil && h == hour {
				got++
			}
		}
		if got != claimConcurrencyTestFounder {
			t.Fatalf("hour %d selected %d actors, want %d", hour, got, claimConcurrencyTestFounder)
		}
	}
	if len(serialDecisions) != claimConcurrencyTestHours*claimConcurrencyTestFounder {
		t.Fatalf("serial decisions = %d, want %d", len(serialDecisions), claimConcurrencyTestHours*claimConcurrencyTestFounder)
	}
	if !reflect.DeepEqual(serialDecisions, concurrentDecisions) {
		t.Fatal("concurrent per-actor decisions differ from the serial run")
	}

	// (c) A ledger replay of the concurrent cell resolves every call from the
	// ledger, makes no model call, and reproduces the same state.
	replay, err := cognition.OpenReplay(concurrentLedger.Path())
	if err != nil {
		t.Fatal(err)
	}
	replaySeam, err := cognition.NewReplaySeam(replay, cognition.FullWakePolicy())
	if err != nil {
		t.Fatal(err)
	}
	opts := naturalCognitionTestOptions()
	opts.Founders = claimConcurrencyTestFounder
	opts.Cognition = &NaturalCognition{Seam: replaySeam, WakePolicy: cognition.FullWakePolicy(), ClaimConcurrency: 3}
	replayed, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, replayed, claimConcurrencyTestHours)
	if served := replaySeam.ServedCalls(); served != claimConcurrencyTestHours*claimConcurrencyTestFounder {
		t.Fatalf("replay served %d calls, want %d", served, claimConcurrencyTestHours*claimConcurrencyTestFounder)
	}
	replayJournal, err := replayed.ExportJournalWithReplay(replaySeam, cognition.FullWakePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replayJournal, concurrentJournal) {
		t.Fatal("ledger replay journal differs from the concurrent run")
	}
}

// TestNaturalClaimConcurrencyAboveSelectedSet is (4): a declared concurrency
// larger than the selected count is safe and equals the serial run.
func TestNaturalClaimConcurrencyAboveSelectedSet(t *testing.T) {
	var served atomic.Int64
	serial, serialLedger := newClaimConcurrencyWorld(t, 1, claimConcurrencyResponder(0, &served))
	naturalRunTo(t, serial, claimConcurrencyTestHours)
	wide, wideLedger := newClaimConcurrencyWorld(t, 64, claimConcurrencyResponder(0, &served))
	naturalRunTo(t, wide, claimConcurrencyTestHours)
	if !bytes.Equal(claimConcurrencyJournal(t, serial, serialLedger), claimConcurrencyJournal(t, wide, wideLedger)) {
		t.Fatal("concurrency above the selected set changed the world")
	}
}

// TestNaturalClaimConcurrencySerialDefault pins the declared default: zero and
// one both mean serial, and any priority-bearing policy (the budgeted P/U/A
// arms) stays byte-for-byte serial whatever the declared number says.
func TestNaturalClaimConcurrencySerialDefault(t *testing.T) {
	opts := naturalCognitionTestOptions()
	opts.Cognition = &NaturalCognition{Seam: stubClaimSeam{}, WakePolicy: cognition.FullWakePolicy()}
	f, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.naturalClaimConcurrency(); got != 1 {
		t.Fatalf("default concurrency = %d, want 1 (serial)", got)
	}
	opts.Cognition.ClaimConcurrency = 1
	if got := f.naturalClaimConcurrency(); got != 1 {
		t.Fatalf("concurrency 1 = %d, want 1 (serial)", got)
	}
	opts.Cognition.ClaimConcurrency = 8
	if got := f.naturalClaimConcurrency(); got != 8 {
		t.Fatalf("concurrency 8 = %d, want 8", got)
	}
	// A priority-bearing wake policy keeps the serial path even when the
	// declared number is greater than one.
	policy := protocol.WakePolicy{
		Name: "test-priority", PeriodHours: 1,
		Priority: &protocol.PriorityPolicy{
			Strategy: protocol.StrategyAdaptive, RandSeed: 0, CallCeiling: 8,
			ValueModel: protocol.ValueModelSpec{Version: "test"},
		},
	}
	opts.Cognition = &NaturalCognition{Seam: stubClaimSeam{}, WakePolicy: policy, Selector: noopSelector{}, ClaimConcurrency: 8}
	f, err = NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.naturalClaimConcurrency(); got != 1 {
		t.Fatalf("priority arm concurrency = %d, want 1 (serial)", got)
	}
}

// TestNaturalClaimConcurrencyCeilingHaltsOnce pins (3): a typed ceiling hit in
// a concurrent batch still stops the run with the typed error, and the hit is
// recorded exactly once. A concurrent batch can cross the call ceiling by at
// most its own width, because calls already issued cannot be recalled; every
// one of them is recorded in the ledger.
func TestNaturalClaimConcurrencyCeilingHaltsOnce(t *testing.T) {
	l := naturalCognitionTestLedger(t)
	acct, err := cognition.NewCognitiveAccount(cognition.CognitiveBudget{
		MaxCalls: 3, MaxPromptTokens: 1_000_000_000,
		MaxCompletionTokens: 1_000_000_000, MaxCostMicros: 1_000_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: l, Client: &cognition.FakeClient{Respond: claimConcurrencyResponder(0, &atomic.Int64{})}, Budget: acct,
		Price: cognition.PriceTable{Version: "test/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2},
		Now:   func() time.Time { return time.UnixMicro(1) },
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	wake := cognition.FullWakePolicy()
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: rec, Ledger: l, Endpoint: "gw.example/v1/chat/completions", WakePolicy: wake,
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := naturalCognitionTestOptions()
	opts.Founders = claimConcurrencyTestFounder
	opts.Cognition = &NaturalCognition{Seam: seam, WakePolicy: wake, ClaimConcurrency: claimConcurrencyTestFounder}
	f, err := NewNatural(opts)
	if err != nil {
		t.Fatal(err)
	}
	var hit *cognition.CognitiveCeilingHitError
	for i := 0; i < 4 && err == nil; i++ {
		_, err = f.Step(context.Background())
	}
	if !errors.As(err, &hit) {
		t.Fatalf("step error = %v, want the typed ceiling hit", err)
	}
	recorded, ok := acct.Hit()
	if !ok || recorded.Ceiling != cognition.CognitiveCeilingCalls {
		t.Fatalf("account hit = %+v, %t; want the calls ceiling", recorded, ok)
	}
	if got := acct.Totals().Calls; got > claimConcurrencyTestFounder {
		t.Fatalf("calls charged %d, want at most one batch width %d", got, claimConcurrencyTestFounder)
	}
	// The hit is sticky and is reported identically on the next attempt.
	if err := acct.AllowCall(0); !errors.As(err, &hit) {
		t.Fatalf("second AllowCall = %v, want the same typed hit", err)
	}
	if again, _ := acct.Hit(); again != recorded {
		t.Fatalf("hit changed: %+v then %+v", recorded, again)
	}
}

// noopSelector is a minimal escalation selector for the serial-scope pin.
type noopSelector struct{}

func (noopSelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	return protocol.Selection{}, nil
}

var _ protocol.EscalationSelector = noopSelector{}
