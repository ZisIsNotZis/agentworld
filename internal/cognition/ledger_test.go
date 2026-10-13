package cognition_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentworld/internal/cognition"
)

func sampleCall(request string) cognition.CallRecord {
	return cognition.CallRecord{
		Role:                cognition.RoleActorDecision,
		RequestHash:         cognitionHash([]byte(request)),
		RequestBytes:        []byte(request),
		ResponseBody:        []byte(`{"schema_version":1,"action":"gather"}`),
		Content:             []byte(`{"schema_version":1,"action":"gather"}`),
		ModelID:             cognition.DefaultModelID,
		Endpoint:            "gw.example/v1/chat/completions",
		CredentialPresent:   true,
		Sampling:            cognition.FrozenSampling(),
		Usage:               cognition.Usage{PromptTokens: 10, CompletionTokens: 4},
		CostMicros:          18,
		PriceVersion:        "test/1",
		Attempt:             1,
		Status:              cognition.CallStatusOK,
		StartedAtUnixMicros: 1,
		LatencyMicros:       2,
	}
}

func newTestLedger(t *testing.T) (*cognition.Ledger, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	l, err := cognition.CreateLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, path
}

func TestLedgerChainsMixedRecordsAndReloads(t *testing.T) {
	l, path := newTestLedger(t)
	if _, err := l.AppendCall(sampleCall("req-0")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.AppendIntervention(cognition.InterventionRecord{
		Kind: cognition.InterventionNoTriggerAudit,
		Cause: cognition.InterventionCause{
			WakePolicy: "periodic(6)", Hour: 24, Trigger: "no-trigger",
		},
		Result: cognition.InterventionResult{Status: "no-effect"},
	}); err != nil {
		t.Fatal(err)
	}
	third, err := l.AppendCall(sampleCall("req-1"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Count() != 3 {
		t.Fatalf("count %d", l.Count())
	}
	if third.Seq != 2 {
		t.Fatalf("seq %d", third.Seq)
	}
	summary, err := l.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Count != 3 || summary.Head != l.Head() {
		t.Fatalf("summary %+v head %x", summary, l.Head())
	}
	// Reopening verifies the chain and resumes at the head.
	rel, err := cognition.OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rel.Close()
	if rel.Count() != 3 || rel.Head() != summary.Head {
		t.Fatalf("reopen count %d head %x", rel.Count(), rel.Head())
	}
	fourth, err := rel.AppendCall(sampleCall("req-2"))
	if err != nil {
		t.Fatal(err)
	}
	if fourth.Seq != 3 || fourth.PrevHash != hex.EncodeToString(summary.Head[:]) {
		t.Fatalf("append after reopen did not chain: %+v", fourth)
	}
}

func TestLedgerFlushesBeforeUse(t *testing.T) {
	l, path := newTestLedger(t)
	rec, err := l.AppendCall(sampleCall("flush-me"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSuffix(string(data), "\n")
	if !strings.Contains(line, `"kind":"call"`) || !strings.Contains(line, rec.Hash) {
		t.Fatalf("record was not flushed before use: %q", line)
	}
	if strings.Contains(line, "\n") {
		t.Fatalf("record did not end at a line boundary")
	}
}

func TestLedgerDetectsTamperedField(t *testing.T) {
	l, path := newTestLedger(t)
	if _, err := l.AppendCall(sampleCall("req-0")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.AppendCall(sampleCall("req-1")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte(cognition.DefaultModelID), []byte(cognition.DefaultModelID+"X"), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("tamper probe did not change the bytes")
	}
	if _, err := cognition.VerifyLedgerData(tampered); !errors.Is(err, cognition.ErrLedgerTampered) {
		t.Fatalf("tampered field accepted: %v", err)
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cognition.OpenLedger(path); err == nil {
		t.Fatal("OpenLedger accepted a tampered file")
	}
}

func TestLedgerDetectsTamperedHash(t *testing.T) {
	l, path := newTestLedger(t)
	rec, err := l.AppendCall(sampleCall("req-0"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Flip one hex digit of the stored hash itself.
	mutated := bytes.Replace(data, []byte(rec.Hash), []byte("f"+rec.Hash[1:]), 1)
	if _, err := cognition.VerifyLedgerData(mutated); !errors.Is(err, cognition.ErrLedgerTampered) {
		t.Fatalf("tampered hash accepted: %v", err)
	}
}

// A mid-chain rewrite that recomputes the rewritten record's own hash is still
// detected, because the next record's prev_hash no longer matches.
func TestLedgerDetectsRehashedMiddleRecord(t *testing.T) {
	l, path := newTestLedger(t)
	if _, err := l.AppendCall(sampleCall("req-0")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.AppendCall(sampleCall("req-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.AppendCall(sampleCall("req-2")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	middle, err := cognition.DecodeCallRecord([]byte(lines[1]))
	if err != nil {
		t.Fatal(err)
	}
	middle.ModelID = "attacker/swap"
	chain, err := cognition.CallRecordChainBytes(middle)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(chain)
	middle.Hash = hex.EncodeToString(sum[:])
	rehashed, err := cognition.EncodeCallRecord(middle)
	if err != nil {
		t.Fatal(err)
	}
	lines[1] = string(rehashed)
	if _, err := cognition.VerifyLedgerData([]byte(strings.Join(lines, "\n") + "\n")); !errors.Is(err, cognition.ErrLedgerChain) {
		t.Fatalf("rehashed middle record accepted: %v", err)
	}
}

func TestLedgerDetectsTruncationAgainstBound(t *testing.T) {
	l, path := newTestLedger(t)
	for _, req := range []string{"req-0", "req-1", "req-2"} {
		if _, err := l.AppendCall(sampleCall(req)); err != nil {
			t.Fatal(err)
		}
	}
	bound, err := l.Summary()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	truncated := []byte(strings.Join(lines[:2], "\n") + "\n")
	// The prefix chain is internally consistent: only the bound count/digest
	// reveals the missing tail.
	if summary, err := cognition.VerifyLedgerData(truncated); err != nil || summary.Count != 2 {
		t.Fatalf("prefix chain should verify: %+v %v", summary, err)
	}
	if err := os.WriteFile(path, truncated, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cognition.VerifyLedgerSummary(path, bound); !errors.Is(err, cognition.ErrLedgerTruncated) {
		t.Fatalf("truncation accepted: %v", err)
	}
}

func TestLedgerRefusesClobberAndEmptyLines(t *testing.T) {
	_, path := newTestLedger(t)
	if _, err := cognition.CreateLedger(path); !errors.Is(err, cognition.ErrLedger) {
		t.Fatalf("CreateLedger clobbered an existing file: %v", err)
	}
	if _, err := cognition.VerifyLedgerData([]byte("\n")); !errors.Is(err, cognition.ErrLedgerTampered) {
		t.Fatalf("empty line accepted: %v", err)
	}
	if _, err := cognition.VerifyLedgerData([]byte(`{"seq":0}`)); !errors.Is(err, cognition.ErrLedgerTampered) {
		t.Fatalf("unhashed line accepted: %v", err)
	}
}
