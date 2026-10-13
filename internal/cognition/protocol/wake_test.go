package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"agentworld/internal/sim"
)

// TestWakePolicyNilPriorityKeepsVS1Bytes is the VS2-EC-1 byte-identity guard:
// the additive Priority field is `omitempty`, so a nil Priority leaves the VS1
// and full-reference canonical identities and fingerprints byte-identical to
// their recorded pre-VS2 values.
func TestWakePolicyNilPriorityKeepsVS1Bytes(t *testing.T) {
	const (
		wantVS1Identity    = `{"name":"periodic","period_hours":6,"phase_hours":0}`
		wantFullIdentity   = `{"name":"periodic","period_hours":1,"phase_hours":0}`
		wantVS1Fingerprint = "83c377e50bcf01b3aea9326f38ac78f83aaebc7025d38fa43418e8f54bce66de"
	)

	vs1 := VS1WakePolicy()
	if vs1.Priority != nil {
		t.Fatalf("VS1 policy carries a non-nil Priority: %+v", vs1.Priority)
	}
	if got := vs1.Identity(); got != wantVS1Identity {
		t.Fatalf("VS1 identity = %q, want %q", got, wantVS1Identity)
	}
	fp := vs1.Fingerprint()
	if got := hex.EncodeToString(fp[:]); got != wantVS1Fingerprint {
		t.Fatalf("VS1 fingerprint = %s, want the recorded pre-VS2 value %s", got, wantVS1Fingerprint)
	}
	if fp != sha256.Sum256([]byte(wantVS1Identity)) {
		t.Fatal("VS1 fingerprint is not sha256 over the recorded identity")
	}
	if got := FullWakePolicy().Identity(); got != wantFullIdentity {
		t.Fatalf("full-reference identity = %q, want %q", got, wantFullIdentity)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(vs1.Identity()), &decoded); err != nil {
		t.Fatalf("identity is not JSON: %v", err)
	}
	if _, present := decoded["priority"]; present {
		t.Fatalf("nil Priority leaked into the canonical identity: %s", vs1.Identity())
	}
}

// TestWakePolicyPriorityEncodesAndRoundTrips pins that a non-nil Priority is
// carried by the canonical identity and survives a JSON round trip, so the
// additive field is live rather than silently ignored.
func TestWakePolicyPriorityEncodesAndRoundTrips(t *testing.T) {
	priority := &PriorityPolicy{
		Strategy:            StrategyAdaptive,
		ExplorationFraction: 250000,
		RandSeed:            42,
		ValueModel: ValueModelSpec{
			Version:   "vs2-value/1",
			Weights:   []FeatureWeight{{Feature: "urgency", Weight: 3}, {Feature: "escalation_requested", Weight: 5}},
			CostProxy: CostProxy{Version: "vs2-cost-proxy/1", ConstantMicrosPerCall: 1, ProjectedPromptTokens: 10, ProjectedCompletionTokens: 4},
		},
		CallCeiling: 256,
	}
	p := VS1WakePolicy()
	p.Priority = priority

	body, err := canonicalJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == VS1WakePolicy().Identity() {
		t.Fatal("a non-nil Priority did not change the canonical identity")
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, present := decoded["priority"]; !present {
		t.Fatalf("non-nil Priority missing from the canonical identity: %s", body)
	}

	var back WakePolicy
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Priority, priority) {
		t.Fatalf("priority round-trip mismatch:\n got %+v\nwant %+v", back.Priority, priority)
	}
}

// TestWakePolicySelectDegenerateNilPriority pins the degenerate nil-Priority
// selector: it is a pure function that selects every roster candidate of an
// escalated hour (the declared full reference and the VS1 path), selects none in
// an unselected hour, consumes no world random stream (PRNGPos 0), and is
// deterministic across calls.
func TestWakePolicySelectDegenerateNilPriority(t *testing.T) {
	policy := VS1WakePolicy()
	roster := []EscalationCandidate{
		{Actor: sim.EntityID(7), Hour: 6, Requested: true, Reason: EscalationRequestedByActor},
		{Actor: sim.EntityID(3), Hour: 6},
		{Actor: sim.EntityID(9), Hour: 6},
	}
	budget := BudgetState{CallsSpent: 4, CallsRemaining: 252, CallsBudget: 256}

	got, err := policy.Select(6, roster, budget)
	if err != nil {
		t.Fatalf("Select(escalated hour) error: %v", err)
	}
	want := Selection{
		Hour:     6,
		Selected: []sim.EntityID{7, 3, 9},
		Budget:   budget,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("escalated selection = %+v, want %+v", got, want)
	}

	skipped, err := policy.Select(5, roster, budget)
	if err != nil {
		t.Fatalf("Select(unselected hour) error: %v", err)
	}
	if len(skipped.Selected) != 0 {
		t.Fatalf("unselected hour selected %v, want none", skipped.Selected)
	}
	if skipped.Hour != 5 || skipped.PRNGPos != 0 {
		t.Fatalf("unselected selection = %+v, want hour 5 with no PRNG use", skipped)
	}

	// Pure: the same inputs give the same selection, and the world RNG is never
	// involved (PRNGPos stays zero on the degenerate path).
	again, err := policy.Select(6, roster, budget)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) {
		t.Fatalf("degenerate Select is not deterministic:\n%+v\n%+v", got, again)
	}
	if got.PRNGPos != 0 {
		t.Fatalf("degenerate Select advanced a PRNG: pos %d", got.PRNGPos)
	}

	// FullWakePolicy escalates every hour and therefore selects the whole roster.
	full, err := FullWakePolicy().Select(0, roster, budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Selected) != len(roster) {
		t.Fatalf("full reference selected %d of %d candidates", len(full.Selected), len(roster))
	}
}

// TestWakePolicyPriorityStrategiesAreDeclared pins the closed strategy set.
func TestWakePolicyPriorityStrategiesAreDeclared(t *testing.T) {
	for _, want := range []PriorityStrategy{StrategyPeriodic, StrategyUniformRandom, StrategyAdaptive, StrategyFull} {
		if want == "" {
			t.Fatal("empty strategy constant")
		}
	}
	if ExplorationScale != 1_000_000 || PriorityScale != 1_000_000 {
		t.Fatalf("fixed-point scales = %d/%d, want 1_000_000 each", ExplorationScale, PriorityScale)
	}
}
