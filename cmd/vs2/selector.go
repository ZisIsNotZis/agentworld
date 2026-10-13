package main

import (
	"fmt"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
	"agentworld/internal/world"
)

// The runner-side wiring of the L3 arm-local online accumulator into the
// world's Selector seam (spec section 10, lane L4c; VS2-EC-1/EC-2/EC-7/EC-8).
//
// cognition.PrioritySelector owns the deterministic selection and the recorded
// accumulator state. The runner wraps it in armSelector so that one `allocation`
// intervention record (C8, Appendix B) is appended per escalated hour through
// the same hash-chained ledger the recorded calls use. The world builds the
// candidate roster and calls Select once per candidate hour; the wrapper never
// changes the selection.

// newArmSelector builds the live/replay arm selector: the L3 accumulator plus
// the allocation-record producer. It refuses a non-priority policy (the
// degenerate WakePolicy.Select path) exactly as the L3 constructor does.
func newArmSelector(policy protocol.WakePolicy, horizonHours int, price protocol.CallPrice, ledger *cognition.Ledger) (*armSelector, error) {
	inner, err := cognition.NewPrioritySelector(policy, horizonHours, price)
	if err != nil {
		return nil, err
	}
	if ledger == nil {
		return nil, fmt.Errorf("vs2: arm selector requires the allocation ledger")
	}
	return &armSelector{inner: inner, ledger: ledger, wake: policy}, nil
}

// armSelector is the runner's EscalationSelector: it delegates to the L3
// accumulator and appends one allocation record per hour that escalated at
// least one decision. The accumulator state (spent-call count, PRNG position) is
// the recorded run state, so replay re-derives the identical sequence.
type armSelector struct {
	inner  *cognition.PrioritySelector
	ledger *cognition.Ledger
	wake   protocol.WakePolicy
	steps  []selectionStep
}

// Select implements protocol.EscalationSelector. It appends the allocation
// record for the hour before returning the selection. The world calls it only
// from the coordinator's allocation phase, so there is no concurrent access to
// the underlying PRNG.
func (s *armSelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	selection, err := s.inner.Select(hour, roster, budget)
	if err != nil {
		return selection, err
	}
	s.steps = append(s.steps, selectionStep{
		Hour: hour, Selected: append([]sim.EntityID(nil), selection.Selected...),
		Spent: s.inner.CallsSpent(), PRNGPos: s.inner.PRNGPos(),
	})
	// Append one allocation record per escalated hour: an hour that selected a
	// decision, or a U/A candidate hour whose roster fell short of the declared
	// quota q(h) (spec section 5.1 requires the shortfall to be recorded for
	// that hour). A period-gated P hour with no selection records nothing.
	if len(selection.Selected) > 0 || selection.Shortfall > 0 {
		if _, err := cognition.AppendAllocationIntervention(s.ledger, cognition.AllocationInterventionInput{
			WakePolicy:    s.wake,
			SimTimeMicros: claimInstantMicros(hour),
			RosterSize:    len(roster),
			Selection:     selection,
		}); err != nil {
			return selection, err
		}
	}
	return selection, nil
}

// recorded returns a copy of the recorded selection steps, the accumulator run
// state the replay tests compare.
func (s *armSelector) recorded() []selectionStep {
	return append([]selectionStep(nil), s.steps...)
}

// CallsSpent and PRNGPos expose the recorded accumulator state for tests.
func (s *armSelector) CallsSpent() uint64 { return s.inner.CallsSpent() }
func (s *armSelector) PRNGPos() uint64    { return s.inner.PRNGPos() }

// claimInstantMicros is the declared claim instant h*NaturalHour + 1 micro,
// matching the world's claim phase (spec section 4.3).
func claimInstantMicros(hour int) int64 {
	return int64(hour)*int64(world.NaturalHour) + 1
}

// newSelectorFactory is the per-replay factory the world's recorded-seam replay
// and restore options consume (L4b). Each call returns a fresh genesis-state L3
// accumulator and NO ledger: the world's internal reconciliation replay must
// re-derive the selection sequence without appending duplicate allocation
// records to the live ledger.
func newSelectorFactory(policy protocol.WakePolicy, horizonHours int, price protocol.CallPrice) func() protocol.EscalationSelector {
	return func() protocol.EscalationSelector {
		selector, err := cognition.NewPrioritySelector(policy, horizonHours, price)
		if err != nil {
			// The policy was validated when the arm was wired; a factory-time
			// failure would be a programming error. Panic is refused by the
			// caller's construction-time checks, so surface it as nil and let
			// the world's construction guard refuse the replay.
			return nil
		}
		return selector
	}
}

// selectorPrice is the declared protocol-local call price the value projection
// is evaluated with (spec section 4.4). The application model is unpriced
// (VS2-EC-8, spec section 1.3.2), so the empty (unpriced) state is passed and
// the declared CostProxy applies; no price is fabricated.
func selectorPrice() protocol.CallPrice { return protocol.CallPrice{} }

// selectionSteps records one Select call for the determinism tests: the hour,
// the selected set, and the post-selection accumulator state.
type selectionStep struct {
	Hour     int
	Selected []sim.EntityID
	Spent    uint64
	PRNGPos  uint64
}
