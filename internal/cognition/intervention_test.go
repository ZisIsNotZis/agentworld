package cognition_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/strategy"
)

func roleAInterventionInput(decision cognition.ActorSeamDecision) cognition.RoleAInterventionInput {
	return cognition.RoleAInterventionInput{
		WakePolicy: cognition.VS1WakePolicy(), Seam: cognition.SeamNaturalClaim,
		Actor: 5, Hour: 6, SimTimeMicros: 6*3_600_000_000 + 1,
		Ref:      strategy.FrozenNaturalPolicy7().Ref,
		Decision: decision,
		Call: cognition.CallResult{
			Seq: 7, Attempt: 1, Usage: cognition.Usage{PromptTokens: 5, CompletionTokens: 3}, CostMicros: 11,
		},
	}
}

// TestRoleAInterventionRecordShape is the intervention-producer guard (spec 9,
// EC-4): a seam decision produces a role-a record carrying cause, cost,
// patch-or-role, and result. Deleting the producer's field population makes a
// case fail.
func TestRoleAInterventionRecordShape(t *testing.T) {
	l, path := newTestLedger(t)
	_, err := cognition.AppendRoleAIntervention(l, roleAInterventionInput(cognition.ActorSeamDecision{
		Action: "gather", Source: cognition.SourceModel, Confidence: 0.75, CallRef: 7,
	}))
	if err != nil {
		t.Fatal(err)
	}
	records, err := cognition.ReadInterventionRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one intervention, got %d", len(records))
	}
	r := records[0]
	if r.Kind != cognition.InterventionRoleA {
		t.Fatalf("kind = %q", r.Kind)
	}
	if r.Cause.WakePolicy != "periodic" || r.Cause.Seam != cognition.SeamNaturalClaim || r.Cause.Actor != 5 || r.Cause.Hour != 6 || r.Cause.SimTimeMicros != 6*3_600_000_000+1 {
		t.Fatalf("cause incomplete: %+v", r.Cause)
	}
	if r.Cost.Calls != 1 || r.Cost.PromptTokens != 5 || r.Cost.CompletionTokens != 3 || r.Cost.CostMicros != 11 {
		t.Fatalf("cost incomplete: %+v", r.Cost)
	}
	if r.Patch == nil || r.Patch.Scope != "role-a" || r.Patch.ParentRef.Version != 7 || r.Patch.ChildRef.Version != 7 {
		t.Fatalf("role marker/head missing: %+v", r.Patch)
	}
	if r.RoleA == nil || r.RoleA.Action != "gather" || r.RoleA.Source != cognition.SourceModel || r.RoleA.Confidence != 0.75 {
		t.Fatalf("role_a incomplete: %+v", r.RoleA)
	}
	if len(r.CallRefs) != 1 || r.CallRefs[0] != 7 {
		t.Fatalf("call refs wrong: %v", r.CallRefs)
	}
	if r.Result.Status != cognition.RoleAResultApplied || r.Result.Reason != "" {
		t.Fatalf("result wrong: %+v", r.Result)
	}
}

// TestRoleAInterventionFallbackIsRejected pins the typed fallback outcome.
func TestRoleAInterventionFallbackIsRejected(t *testing.T) {
	l, path := newTestLedger(t)
	_, err := cognition.AppendRoleAIntervention(l, roleAInterventionInput(cognition.ActorSeamDecision{
		Source: cognition.SourceFallback, Reason: string(cognition.FallbackReasonInadmissible), CallRef: 7,
	}))
	if err != nil {
		t.Fatal(err)
	}
	records, err := cognition.ReadInterventionRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].Result.Status != cognition.RoleAResultRejected || records[0].Result.Reason != string(cognition.FallbackReasonInadmissible) {
		t.Fatalf("fallback result wrong: %+v", records[0].Result)
	}
}

// TestRoleAInterventionsReferenceEveryCall is the EC-4 completeness check: no
// orphan call records; every call record is referenced by exactly one
// intervention.
func TestRoleAInterventionsReferenceEveryCall(t *testing.T) {
	seam, _, path := newRoleASeam(t, `{"schema_version":1,"action":"wait"}`)
	for i := 0; i < 3; i++ {
		req := sampleSeamRequest()
		req.Hour = 6 + i
		if _, err := seam.DecideActorSeam(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	calls, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	interventions, err := cognition.ReadInterventionRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(interventions) != 3 {
		t.Fatalf("expected 3 interventions, got %d", len(interventions))
	}
	referenced := make(map[uint64]int)
	for _, r := range interventions {
		for _, seq := range r.CallRefs {
			referenced[seq]++
		}
	}
	for _, call := range calls {
		if referenced[call.Seq] != 1 {
			t.Fatalf("call %d referenced %d times (orphan or double)", call.Seq, referenced[call.Seq])
		}
	}
}

// TestRoleAInterventionRefsExactUnderConcurrency reproduces the Stage-E defect:
// two calls run concurrently and the first call's retry record lands after the
// second call's used attempt. The intervention must reference exactly its own
// attempts, never a contiguous range that includes the other call's record, and
// replay must rebuild the same exact list.
func TestRoleAInterventionRefsExactUnderConcurrency(t *testing.T) {
	l, path := newTestLedger(t)
	acct := newAccount(t, generousBudget())

	var mu sync.Mutex
	attempts := map[string]int{}
	fake := &cognition.FakeClient{Respond: func(req cognition.ModelRequest) (cognition.ModelResponse, error) {
		mu.Lock()
		attempts[string(req.Request)]++
		n := attempts[string(req.Request)]
		mu.Unlock()
		if n == 1 {
			return cognition.ModelResponse{}, errors.New("transport down")
		}
		return cognition.DefaultFakeResponse(), nil
	}}

	aSleeping := make(chan struct{})
	releaseA := make(chan struct{})
	var sleepOnce sync.Once
	cfg := fixedRecorderConfig(l, fake, acct)
	cfg.Sleep = func(context.Context, time.Duration) error {
		first := false
		sleepOnce.Do(func() { first = true })
		if first {
			close(aSleeping)
			<-releaseA
		}
		return nil
	}
	rec, err := cognition.NewRecorder(cfg)
	if err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		result cognition.CallResult
		err    error
	}
	reqA := sampleRequest(cognition.RoleActorDecision, `{"call":"A"}`, 6)
	reqB := sampleRequest(cognition.RoleActorDecision, `{"call":"B"}`, 7)
	aDone := make(chan outcome, 1)
	go func() {
		result, err := rec.Call(context.Background(), reqA)
		aDone <- outcome{result, err}
	}()

	// A's first attempt (seq 0) is recorded and it is now sleeping before retry.
	<-aSleeping
	// B runs to completion while A waits: B1 (seq 1) and B2 (seq 2).
	bResult, err := rec.Call(context.Background(), reqB)
	if err != nil {
		t.Fatal(err)
	}
	// A retries: its used attempt is seq 3, not the contiguous seq 2 the old
	// range [2,3] would have named.
	close(releaseA)
	a := <-aDone
	if a.err != nil {
		t.Fatal(a.err)
	}
	if a.result.Seq != 3 || a.result.Attempt != 2 || bResult.Seq != 2 || bResult.Attempt != 2 {
		t.Fatalf("ledger order: A seq=%d attempt=%d, B seq=%d attempt=%d", a.result.Seq, a.result.Attempt, bResult.Seq, bResult.Attempt)
	}
	if !reflect.DeepEqual(a.result.AttemptSeqs, []uint64{0, 3}) || !reflect.DeepEqual(bResult.AttemptSeqs, []uint64{1, 2}) {
		t.Fatalf("attempt seqs: A=%v B=%v want A=[0 3] B=[1 2]", a.result.AttemptSeqs, bResult.AttemptSeqs)
	}

	decision := cognition.ActorSeamDecision{Action: "gather", Source: cognition.SourceModel, Confidence: 0.5}
	inA := roleAInterventionInput(decision)
	inA.Hour, inA.Call = 6, a.result
	inB := roleAInterventionInput(decision)
	inB.Hour, inB.Call = 7, bResult
	if _, err := cognition.AppendRoleAIntervention(l, inA); err != nil {
		t.Fatal(err)
	}
	if _, err := cognition.AppendRoleAIntervention(l, inB); err != nil {
		t.Fatal(err)
	}

	interventions, err := cognition.ReadInterventionRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(interventions) != 2 {
		t.Fatalf("interventions=%d want 2", len(interventions))
	}
	if !reflect.DeepEqual(interventions[0].CallRefs, []uint64{0, 3}) {
		t.Fatalf("intervention A refs=%v want the call's exact attempts [0 3]", interventions[0].CallRefs)
	}
	if !reflect.DeepEqual(interventions[1].CallRefs, []uint64{1, 2}) {
		t.Fatalf("intervention B refs=%v want [1 2]", interventions[1].CallRefs)
	}
	calls, err := cognition.ReadCallRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	referenced := map[uint64]int{}
	for _, record := range interventions {
		for _, seq := range record.CallRefs {
			referenced[seq]++
		}
	}
	for _, call := range calls {
		if referenced[call.Seq] != 1 {
			t.Fatalf("call %d referenced %d times (want exactly once)", call.Seq, referenced[call.Seq])
		}
	}

	// Replay rebuilds the exact attempt list from the ledger, so a replayed
	// intervention records the same refs as the live one.
	replay, err := cognition.OpenReplay(path)
	if err != nil {
		t.Fatal(err)
	}
	replayedA, err := replay.Resolve(reqA.Request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayedA.AttemptSeqs, []uint64{0, 3}) {
		t.Fatalf("replayed A attempt seqs=%v want [0 3]", replayedA.AttemptSeqs)
	}
}
