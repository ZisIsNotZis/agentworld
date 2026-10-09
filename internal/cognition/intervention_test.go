package cognition_test

import (
	"context"
	"testing"

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
