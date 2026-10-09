package cognition

import (
	"bytes"
	"os"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The Role A intervention-record producer (spec section 9, EC-4). Every
// escalated seam decision appends exactly one hash-chained role-a record with
// cause, cost, the role marker plus the applied-version head, and a result;
// every call record of that decision — including a retried physical attempt —
// is referenced by exactly one intervention, so there are no orphan calls.
//
// Appendix D.5: a Role A decision has no patch, so the patch block carries the
// explicit role marker and the applied-version head, and the RoleA block
// carries the decision. The patch block proper is reserved for Role B.
const roleAInterventionScope = "role-a"

// Result statuses recorded for a Role A intervention (Appendix B).
const (
	RoleAResultApplied  = "applied"
	RoleAResultRejected = "rejected"
)

// RoleAInterventionInput is one escalated decision's evidence.
type RoleAInterventionInput struct {
	WakePolicy    WakePolicy
	Seam          string
	Actor         sim.EntityID
	Hour          int
	SimTimeMicros int64
	Ref           strategy.NaturalRef
	Decision      ActorSeamDecision
	Call          CallResult
}

// AppendRoleAIntervention appends the role-a intervention record and returns
// it with its assigned sequence, prev_hash, and hash.
func AppendRoleAIntervention(l *Ledger, in RoleAInterventionInput) (InterventionRecord, error) {
	if l == nil {
		return InterventionRecord{}, ErrRoleASeamNoLedger
	}
	status, reason := RoleAResultApplied, ""
	if in.Decision.Source != SourceModel {
		status, reason = RoleAResultRejected, in.Decision.Reason
	}
	record := InterventionRecord{
		Kind: InterventionRoleA,
		Cause: InterventionCause{
			WakePolicy:    in.WakePolicy.Name,
			Seam:          in.Seam,
			Actor:         in.Actor,
			Hour:          in.Hour,
			SimTimeMicros: in.SimTimeMicros,
		},
		CallRefs: callRefRange(in.Call),
		Cost: InterventionCost{
			Calls:            1,
			PromptTokens:     in.Call.Usage.PromptTokens,
			CompletionTokens: in.Call.Usage.CompletionTokens,
			CostMicros:       in.Call.CostMicros,
		},
		Patch: &PatchRecord{
			ParentRef: NaturalRefRecordOf(in.Ref),
			ChildRef:  NaturalRefRecordOf(in.Ref),
			Scope:     roleAInterventionScope,
		},
		RoleA: &RoleARecord{
			Action:         in.Decision.Action,
			Source:         in.Decision.Source,
			Reason:         in.Decision.Reason,
			Confidence:     in.Decision.Confidence,
			FallbackReason: fallbackReasonOf(in.Decision),
		},
		Result: InterventionResult{Status: status, Reason: reason, IndicatorDeltas: []IndicatorDelta{}},
	}
	return l.AppendIntervention(record)
}

// callRefRange references every physical attempt record of one recorded call.
// The Recorder appends one contiguous run of records per Call and returns the
// used attempt's sequence and index, so the run is exactly
// [seq-(attempt-1), seq].
func callRefRange(call CallResult) []uint64 {
	return callRefRefs(call.Seq, call.Attempt)
}

// callRefRefs is the shared attempt-range expansion.
func callRefRefs(seq uint64, attempt int) []uint64 {
	if attempt <= 1 {
		return []uint64{seq}
	}
	refs := make([]uint64, 0, attempt)
	start := seq - uint64(attempt-1)
	for s := start; s <= seq; s++ {
		refs = append(refs, s)
	}
	return refs
}

func fallbackReasonOf(d ActorSeamDecision) string {
	if d.Source == SourceFallback {
		return d.Reason
	}
	return ""
}

// ReadInterventionRecords verifies a ledger file's chain and returns its
// intervention records in order (EC-4 verification surface).
func ReadInterventionRecords(path string) ([]InterventionRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if _, err := VerifyLedgerData(data); err != nil {
		return nil, err
	}
	var out []InterventionRecord
	for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		kind, err := recordKindOf(line)
		if err != nil {
			return nil, err
		}
		if kind == callRecordKind {
			continue
		}
		rec, err := DecodeInterventionRecord(line)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}
