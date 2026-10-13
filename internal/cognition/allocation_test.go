package cognition_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
)

func sampleSelection() cognition.Selection {
	return cognition.Selection{
		Hour:        12,
		Selected:    []sim.EntityID{10, 20},
		Exploration: []sim.EntityID{10},
		Scores: []cognition.CandidateScore{
			{Actor: 10, ExpectedValue: 8, ExpectedCost: 15, Priority: 533333, Requested: true},
			{Actor: 20, ExpectedValue: 3, ExpectedCost: 15, Priority: 200000, Requested: false},
		},
		Budget:    cognition.BudgetState{CallsSpent: 7, CallsRemaining: 249, CallsBudget: 256},
		Quota:     2,
		Shortfall: 0,
		PRNGPos:   3,
	}
}

func sampleAllocationInput() cognition.AllocationInterventionInput {
	return cognition.AllocationInterventionInput{
		WakePolicy:    testPolicy(cognition.StrategyAdaptive, 1, 256, 250000, 6, 0),
		SimTimeMicros: 12*3600*1_000_000 + 1,
		RosterSize:    4,
		Selection:     sampleSelection(),
	}
}

// TestAllocationInterventionRoundTripAndChain pins the additive allocation
// record (Appendix B, VS2-EC-8/EC-9): it appends through the same hash chain as
// every intervention, carries the declared policy fingerprint and the recorded
// accumulator state, and decodes back byte-exact.
func TestAllocationInterventionRoundTripAndChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cognition.jsonl")
	l, err := cognition.CreateLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := cognition.AppendAllocationIntervention(l, sampleAllocationInput())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	if rec.Kind != cognition.InterventionAllocation {
		t.Fatalf("kind = %q, want %q", rec.Kind, cognition.InterventionAllocation)
	}
	if rec.Allocation == nil {
		t.Fatal("allocation payload missing")
	}
	fp := sampleAllocationInput().WakePolicy.Fingerprint()
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"policy_fingerprint", rec.Allocation.PolicyFingerprint, hex.EncodeToString(fp[:])},
		{"roster_size", rec.Allocation.RosterSize, 4},
		{"quota", rec.Allocation.Quota, 2},
		{"shortfall", rec.Allocation.Shortfall, 0},
		{"call_ceiling", rec.Allocation.CallCeiling, uint64(256)},
		{"prng_pos", rec.Allocation.PRNGPos, uint64(3)},
		{"realized_calls", rec.Allocation.RealizedCalls, uint64(7)},
		{"cause_hour", rec.Cause.Hour, 12},
		{"result_status", rec.Result.Status, cognition.AllocationResultSelected},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Fatalf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(rec.Allocation.Selected) != 2 || len(rec.Allocation.Exploration) != 1 || len(rec.Allocation.Scores) != 2 {
		t.Fatalf("allocation lists = %+v", rec.Allocation)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cognition.VerifyLedgerData(data); err != nil {
		t.Fatalf("ledger chain invalid: %v", err)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("ledger has %d lines, want 1", len(lines))
	}
	decoded, err := cognition.DecodeInterventionRecord(lines[0])
	if err != nil {
		t.Fatalf("decode allocation record: %v", err)
	}
	if decoded.Allocation == nil || decoded.Allocation.PolicyFingerprint != rec.Allocation.PolicyFingerprint {
		t.Fatalf("decoded allocation mismatch: %+v", decoded.Allocation)
	}

	records, err := cognition.ReadInterventionRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Kind != cognition.InterventionAllocation {
		t.Fatalf("ReadInterventionRecords = %+v", records)
	}
}

// TestAllocationPayloadLeavesExistingBytesUnchanged is the additivity guard for
// the codec change: the nil Allocation field is omitted, so the recorded
// pre-change intervention golden bytes are unchanged.
func TestAllocationPayloadLeavesExistingBytesUnchanged(t *testing.T) {
	golden := sealIntervention(t, goldenInterventionRecord())
	wire, err := cognition.EncodeInterventionRecord(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(wire); got != goldenInterventionWire {
		t.Fatalf("pre-change intervention bytes drifted:\n got  %s\n want %s", got, goldenInterventionWire)
	}
	if bytes.Contains(wire, []byte(`"allocation"`)) {
		t.Fatalf("nil Allocation leaked into the canonical wire: %s", wire)
	}

	// The same record with an allocation payload adds only the new key.
	withAlloc := golden
	withAlloc.Allocation = &cognition.AllocationRecord{
		PolicyFingerprint: golden.Patch.ParentFingerprint,
		RosterSize:        2,
		CallCeiling:       256,
		Selected:          []sim.EntityID{},
		Exploration:       []sim.EntityID{},
		Scores:            []cognition.CandidateScore{},
	}
	allocationWire, err := cognition.EncodeInterventionRecord(sealIntervention(t, withAlloc))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(allocationWire, []byte(`"allocation"`)) {
		t.Fatalf("non-nil Allocation missing from the wire: %s", allocationWire)
	}
}

// TestAppendAllocationInterventionRequiresLedger pins the typed refusal.
func TestAppendAllocationInterventionRequiresLedger(t *testing.T) {
	if _, err := cognition.AppendAllocationIntervention(nil, sampleAllocationInput()); !errors.Is(err, cognition.ErrAllocationNoLedger) {
		t.Fatalf("nil ledger accepted: %v", err)
	}
}
