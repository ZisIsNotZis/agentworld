package main

import (
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// The equal-budget accounting tests (VS2-EC-2/VS2-EC-7): exact-N, the declared
// q(h) schedule, stop-at-N, under-budget reporting, and the protected
// exploration bound. They drive the L3 arm-local accumulator directly over a
// synthetic roster (no world), because the runner wires exactly this selector
// into the world's Select seam.

func testPolicy(strategy protocol.PriorityStrategy, ceiling uint64, fraction int64, seed uint64, period, phase int) protocol.WakePolicy {
	return protocol.WakePolicy{
		Name: "test-" + string(strategy), PeriodHours: period, PhaseHours: phase,
		Priority: &protocol.PriorityPolicy{
			Strategy: strategy, ExplorationFraction: fraction, RandSeed: seed,
			ValueModel: declaredValueModel(), CallCeiling: ceiling,
		},
	}
}

// testRoster builds an actor-sorted roster of n candidates.
func testRoster(n, hour int) []protocol.EscalationCandidate {
	roster := make([]protocol.EscalationCandidate, 0, n)
	for i := 1; i <= n; i++ {
		roster = append(roster, protocol.EscalationCandidate{
			Actor:    simID(i),
			Hour:     hour,
			Features: protocol.ValueFeatures{Energy: int64(i)},
		})
	}
	return roster
}

func simID(i int) sim.EntityID { return sim.EntityID(i) }

// TestQuotaScheduleSumAndShape pins q(h) = floor(N/H) + (1 if h < N mod H) and
// sum_{h<H} q(h) = N.
func TestQuotaScheduleSumAndShape(t *testing.T) {
	q := quotaSchedule(256, 168)
	if len(q) != 168 {
		t.Fatalf("schedule length %d", len(q))
	}
	if got := sumInts(q); got != 256 {
		t.Fatalf("sum q(h) = %d, want 256", got)
	}
	if q[0] != 2 || q[87] != 2 || q[88] != 1 || q[167] != 1 {
		t.Fatalf("q(h) shape: q0=%d q87=%d q88=%d q167=%d", q[0], q[87], q[88], q[167])
	}
}

// TestQuotaScheduleMirrorsSelector pins the runner's declared schedule against
// the L3 accumulator's own per-hour Quota, so the two copies cannot drift.
func TestQuotaScheduleMirrorsSelector(t *testing.T) {
	const ceiling, horizon = 24, 8
	want := quotaSchedule(ceiling, horizon)
	for _, arm := range []protocol.PriorityStrategy{protocol.StrategyUniformRandom, protocol.StrategyAdaptive} {
		policy := testPolicy(arm, ceiling, 0, 3, 2, 0)
		sel, err := cognition.NewPrioritySelector(policy, horizon, protocol.CallPrice{})
		if err != nil {
			t.Fatal(err)
		}
		for h := 0; h < horizon; h++ {
			selection, err := sel.Select(h, testRoster(8, h), protocol.BudgetState{CallsBudget: ceiling, CallsRemaining: ceiling})
			if err != nil {
				t.Fatal(err)
			}
			if selection.Quota != want[h] {
				t.Fatalf("%s hour %d Quota=%d, want %d", arm, h, selection.Quota, want[h])
			}
		}
	}
}

// TestArmStopsAtCeiling pins that every budgeted strategy realizes exactly N on
// an ample stream and never exceeds N.
func TestArmStopsAtCeiling(t *testing.T) {
	const ceiling, horizon = 16, 8
	for _, arm := range []protocol.PriorityStrategy{protocol.StrategyPeriodic, protocol.StrategyUniformRandom, protocol.StrategyAdaptive} {
		policy := testPolicy(arm, ceiling, DeclaredExplorationFraction, 5, 2, 0)
		sel, err := cognition.NewPrioritySelector(policy, horizon, protocol.CallPrice{})
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for h := 0; h < horizon; h++ {
			selection, err := sel.Select(h, testRoster(8, h), protocol.BudgetState{CallsBudget: ceiling, CallsRemaining: ceiling})
			if err != nil {
				t.Fatal(err)
			}
			total += len(selection.Selected)
			if uint64(total) > ceiling {
				t.Fatalf("%s exceeded ceiling at hour %d: %d", arm, h, total)
			}
		}
		if uint64(total) != ceiling {
			t.Fatalf("%s realized %d, want exactly %d", arm, total, ceiling)
		}
		if sel.CallsSpent() != ceiling {
			t.Fatalf("%s CallsSpent=%d, want %d", arm, sel.CallsSpent(), ceiling)
		}
	}
}

// TestUnderBudgetReportsShortfall pins the under-budget outcome: a thin roster
// spends what it offers, records the per-hour shortfall, and realizes below N
// without error.
func TestUnderBudgetReportsShortfall(t *testing.T) {
	const ceiling, horizon = 16, 4
	policy := testPolicy(protocol.StrategyUniformRandom, ceiling, 0, 1, 2, 0)
	sel, err := cognition.NewPrioritySelector(policy, horizon, protocol.CallPrice{})
	if err != nil {
		t.Fatal(err)
	}
	total, shortfall := 0, 0
	for h := 0; h < horizon; h++ {
		selection, err := sel.Select(h, testRoster(1, h), protocol.BudgetState{CallsBudget: ceiling, CallsRemaining: ceiling})
		if err != nil {
			t.Fatal(err)
		}
		total += len(selection.Selected)
		shortfall += selection.Shortfall
	}
	if uint64(total) >= ceiling {
		t.Fatalf("a thin roster realized %d, expected under budget", total)
	}
	if shortfall == 0 {
		t.Fatal("no per-hour shortfall recorded on a thin roster")
	}
	if sel.CallsSpent() != uint64(total) {
		t.Fatalf("CallsSpent %d != realized %d", sel.CallsSpent(), total)
	}
}

// TestProtectedExplorationBound pins E = ceil(q*N) reserved exploration for the
// adaptive arm and pure exploitation for q = 0.
func TestProtectedExplorationBound(t *testing.T) {
	const ceiling, horizon = 16, 8
	wantE := reservedExploration(ceiling, DeclaredExplorationFraction)
	if wantE != 4 {
		t.Fatalf("E = %d, want ceil(0.25*16)=4", wantE)
	}
	explore := func(arm protocol.PriorityStrategy, fraction int64) int {
		policy := testPolicy(arm, ceiling, fraction, 9, 2, 0)
		sel, err := cognition.NewPrioritySelector(policy, horizon, protocol.CallPrice{})
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for h := 0; h < horizon; h++ {
			selection, err := sel.Select(h, testRoster(8, h), protocol.BudgetState{CallsBudget: ceiling, CallsRemaining: ceiling})
			if err != nil {
				t.Fatal(err)
			}
			total += len(selection.Exploration)
		}
		return total
	}
	if got := explore(protocol.StrategyAdaptive, DeclaredExplorationFraction); got != wantE {
		t.Fatalf("adaptive exploration = %d, want E = %d", got, wantE)
	}
	if got := explore(protocol.StrategyAdaptive, 0); got != 0 {
		t.Fatalf("q=0 exploration = %d, want 0 (pure exploitation)", got)
	}
	if got := explore(protocol.StrategyUniformRandom, DeclaredExplorationFraction); got != 0 {
		t.Fatalf("uniform-random exploration = %d, want 0", got)
	}
}

// TestSelectorConsumesNoWorldRNGState pins determinism: two identical selectors
// over the identical stream produce the identical selection sequence (the
// accumulator state is the recorded run state).
func TestSelectorConsumesNoWorldRNGState(t *testing.T) {
	const ceiling, horizon = 16, 8
	policy := testPolicy(protocol.StrategyAdaptive, ceiling, DeclaredExplorationFraction, 11, 2, 0)
	runOnce := func() []protocol.Selection {
		sel, err := cognition.NewPrioritySelector(policy, horizon, protocol.CallPrice{})
		if err != nil {
			t.Fatal(err)
		}
		var out []protocol.Selection
		for h := 0; h < horizon; h++ {
			selection, err := sel.Select(h, testRoster(6, h), protocol.BudgetState{CallsBudget: ceiling, CallsRemaining: ceiling})
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, selection)
		}
		return out
	}
	a, b := runOnce(), runOnce()
	for i := range a {
		if len(a[i].Selected) != len(b[i].Selected) || a[i].PRNGPos != b[i].PRNGPos || a[i].Budget.CallsSpent != b[i].Budget.CallsSpent {
			t.Fatalf("selection %d differs between identical runs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestDeclaredValueModelIsIntegerOnly pins the declared value model's integer
// fixed-point priority (VS2-EC-8): the priority is an int64 with no float path,
// and the all-zero weight vector reduces to a constant priority.
func TestDeclaredValueModelIsIntegerOnly(t *testing.T) {
	features := protocol.ValueFeatures{DecisionSensitivity: 3, Urgency: 2}
	p := protocol.Priority(declaredValueModel(), features, protocol.CallPrice{})
	if p <= 0 {
		t.Fatalf("priority = %d, want a positive integer", p)
	}
	zero := declaredValueModel()
	for i := range zero.Weights {
		zero.Weights[i].Weight = 0
	}
	if got := protocol.Priority(zero, features, protocol.CallPrice{}); got != 0 {
		t.Fatalf("all-zero weights priority = %d, want 0 (the uniform reduction)", got)
	}
}
