package cognition

import (
	"errors"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// The allocation-intervention producer (spec Appendix B, C8). One hash-chained
// `allocation` record is appended per escalated hour through the same Ledger as
// every other intervention, carrying the declared policy fingerprint, the
// candidate roster size, the per-candidate priority scores, the selected and
// protected-exploration sets, the budget state, the declared call ceiling, the
// policy-PRNG position, and the cumulative realized calls, so an allocation
// decision is auditable like every other intervention. The record is appended
// before the world commits and its result is the level-1 allocation outcome
// (spec section 12).
var ErrAllocationNoLedger = errors.New("allocation intervention requires a ledger")

// AllocationResultSelected is the level-1 allocation outcome recorded on an
// allocation intervention: the hour's selection was made.
const AllocationResultSelected = "selected"

// AllocationInterventionInput is one hour's allocation evidence.
type AllocationInterventionInput struct {
	WakePolicy    WakePolicy
	SimTimeMicros int64
	RosterSize    int // candidates offered at this hour
	Selection     Selection
}

// AppendAllocationIntervention appends the allocation record for one hour's
// Selection and returns it with its assigned sequence, prev_hash, and hash. The
// selection's recorded budget (CallsBudget/CallsSpent) supplies the declared
// ceiling N and the cumulative realized calls, so the record is derived from the
// same accumulator state that produced the selection.
func AppendAllocationIntervention(l *Ledger, in AllocationInterventionInput) (InterventionRecord, error) {
	if l == nil {
		return InterventionRecord{}, ErrAllocationNoLedger
	}
	sel := in.Selection
	record := InterventionRecord{
		Kind: InterventionAllocation,
		Cause: InterventionCause{
			WakePolicy:    in.WakePolicy.Name,
			Hour:          sel.Hour,
			SimTimeMicros: in.SimTimeMicros,
		},
		CallRefs: []uint64{},
		Cost:     InterventionCost{Calls: uint64(len(sel.Selected))},
		Allocation: &AllocationRecord{
			PolicyFingerprint: formatHash(in.WakePolicy.Fingerprint()),
			RosterSize:        in.RosterSize,
			Quota:             sel.Quota,
			Shortfall:         sel.Shortfall,
			Selected:          nonNilEntityIDs(sel.Selected),
			Exploration:       nonNilEntityIDs(sel.Exploration),
			Scores:            nonNilScores(sel.Scores),
			Budget:            sel.Budget,
			CallCeiling:       sel.Budget.CallsBudget,
			PRNGPos:           sel.PRNGPos,
			RealizedCalls:     sel.Budget.CallsSpent,
		},
		Result: InterventionResult{Status: AllocationResultSelected, IndicatorDeltas: []IndicatorDelta{}},
	}
	return l.AppendIntervention(record)
}

// nonNilEntityIDs never records a null entity list.
func nonNilEntityIDs(ids []sim.EntityID) []sim.EntityID {
	if ids == nil {
		return []sim.EntityID{}
	}
	return ids
}

// nonNilScores never records a null score list.
func nonNilScores(scores []protocol.CandidateScore) []protocol.CandidateScore {
	if scores == nil {
		return []protocol.CandidateScore{}
	}
	return scores
}
