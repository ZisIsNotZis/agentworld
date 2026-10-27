package main

import (
	"os"
	"path/filepath"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
)

// appendDecision records one hermetic Role A call and its intervention at hour.
func appendDecision(t *testing.T, l *cognition.Ledger, hour int, actor int) {
	t.Helper()
	call, err := l.AppendCall(cognition.CallRecord{Role: cognition.RoleActorDecision, Status: cognition.CallStatusOK})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.AppendIntervention(cognition.InterventionRecord{
		Kind: cognition.InterventionRoleA, Cause: cognition.InterventionCause{Hour: hour, Actor: sim.EntityID(actor + 1)},
		CallRefs: []uint64{call.Seq},
	}); err != nil {
		t.Fatal(err)
	}
}

// TestCompleteLedgerHoursAndTruncate pins the resume boundary: the complete-hour
// count is one past the last recorded intervention hour, and truncating a
// partially recorded hour leaves a hash chain that still verifies. A resume that
// guessed the boundary would replay into a half-recorded hour and either miss a
// call or duplicate one.
func TestCompleteLedgerHoursAndTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	l, err := cognition.CreateLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	for hour := 0; hour < 3; hour++ {
		for actor := 0; actor < 2; actor++ {
			appendDecision(t, l, hour, actor)
		}
	}
	// A killed process leaves a partial hour: calls recorded, no intervention.
	for actor := 0; actor < 1; actor++ {
		if _, err := l.AppendCall(cognition.CallRecord{Role: cognition.RoleActorDecision, Status: cognition.CallStatusOK}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	hours, err := completeLedgerHours(path)
	if err != nil {
		t.Fatal(err)
	}
	if hours != 3 {
		t.Fatalf("completeLedgerHours = %d, want 3", hours)
	}
	if err := truncateLedgerAtHour(path, hours); err != nil {
		t.Fatal(err)
	}
	// The truncated prefix must still be a valid hash chain and cover 3 hours.
	reopened, err := cognition.OpenLedger(path)
	if err != nil {
		t.Fatalf("truncated ledger does not verify: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := completeLedgerHours(path)
	if err != nil {
		t.Fatal(err)
	}
	if again != 3 {
		t.Fatalf("after truncation completeLedgerHours = %d, want 3", again)
	}
	// Truncating at the boundary is idempotent.
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := truncateLedgerAtHour(path, hours); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("truncate at the boundary changed the ledger")
	}
}
