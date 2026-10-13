package cognition_test

import (
	"errors"
	"reflect"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// declaredHorizon is VS2's declared evaluation horizon H.
const declaredHorizon = 168

func testValueModel() cognition.ValueModelSpec {
	return cognition.ValueModelSpec{
		Version: "vs2-value/1",
		Weights: []cognition.FeatureWeight{
			{Feature: "decision_sensitivity", Weight: 4},
			{Feature: "urgency", Weight: 3},
			{Feature: "impact", Weight: 2},
			{Feature: "propagation_risk", Weight: 2},
			{Feature: "prior_reliability", Weight: 1},
			{Feature: "escalation_requested", Weight: 5},
		},
		CostProxy: cognition.CostProxy{Version: "vs2-cost-proxy/1", ConstantMicrosPerCall: 1, ProjectedPromptTokens: 10, ProjectedCompletionTokens: 4},
	}
}

func testPolicy(strategy cognition.PriorityStrategy, seed, ceiling uint64, exploration int64, period, phase int) cognition.WakePolicy {
	p := cognition.WakePolicy{Name: "vs2", PeriodHours: period, PhaseHours: phase}
	p.Priority = &cognition.PriorityPolicy{
		Strategy:            strategy,
		ExplorationFraction: exploration,
		RandSeed:            seed,
		ValueModel:          testValueModel(),
		CallCeiling:         ceiling,
	}
	return p
}

// testRoster builds count candidates at hour with distinct ascending feature
// vectors, so priority ranking is non-trivial and the roster order is the
// declared actor order.
func testRoster(hour, count int) []cognition.EscalationCandidate {
	out := make([]cognition.EscalationCandidate, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, cognition.EscalationCandidate{
			Actor: sim.EntityID(10 * (i + 1)),
			Hour:  hour,
			Features: cognition.ValueFeatures{
				Urgency:             int64(i),
				DecisionSensitivity: int64(count - i),
				EscalationRequested: int64(i % 2),
			},
			Requested: i%2 == 0,
		})
	}
	return out
}

func mustSelector(t *testing.T, p cognition.WakePolicy) *cognition.PrioritySelector {
	t.Helper()
	s, err := cognition.NewPrioritySelector(p, declaredHorizon, protocol.CallPrice{})
	if err != nil {
		t.Fatalf("NewPrioritySelector: %v", err)
	}
	return s
}

// TestPrioritySelectorQuotaScheduleMatchesDeclared pins the declared fixed
// per-hour quota q(h) = floor(N/H) + (1 if h < N mod H else 0), so sum q(h) = N
// over the horizon. An empty roster records the schedule without spending.
func TestPrioritySelectorQuotaScheduleMatchesDeclared(t *testing.T) {
	cases := []struct {
		ceiling uint64
		firstQ  int
		secondQ int
		split   int // q(h) = firstQ for h < split, secondQ otherwise
	}{
		{256, 2, 1, 88},
		{168, 1, 1, 168},
		{100, 1, 0, 100},
	}
	for _, tc := range cases {
		s := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 1, tc.ceiling, 250000, 6, 0))
		total := 0
		for h := 0; h < declaredHorizon; h++ {
			sel, err := s.Select(h, nil, protocol.BudgetState{CallsBudget: tc.ceiling})
			if err != nil {
				t.Fatalf("N=%d hour %d: %v", tc.ceiling, h, err)
			}
			want := tc.secondQ
			if h < tc.split {
				want = tc.firstQ
			}
			if sel.Quota != want {
				t.Fatalf("N=%d q(%d) = %d, want %d", tc.ceiling, h, sel.Quota, want)
			}
			total += sel.Quota
		}
		if uint64(total) != tc.ceiling {
			t.Fatalf("N=%d: sum q(h) = %d, want %d", tc.ceiling, total, tc.ceiling)
		}
	}
}

// TestPrioritySelectorExactNOnAmpleStream pins VS2-EC-2 on an ample roster:
// P/U/A each realize exactly N over the declared horizon and never exceed it.
func TestPrioritySelectorExactNOnAmpleStream(t *testing.T) {
	const ceiling uint64 = 256
	for _, strategy := range []cognition.PriorityStrategy{
		cognition.StrategyPeriodic, cognition.StrategyUniformRandom, cognition.StrategyAdaptive,
	} {
		exploration := int64(250000)
		if strategy == cognition.StrategyPeriodic {
			exploration = 0
		}
		s := mustSelector(t, testPolicy(strategy, 7, ceiling, exploration, 1, 0))
		for h := 0; h < declaredHorizon; h++ {
			sel, err := s.Select(h, testRoster(h, 8), protocol.BudgetState{CallsBudget: ceiling})
			if err != nil {
				t.Fatalf("%s hour %d: %v", strategy, h, err)
			}
			if sel.Budget.CallsSpent > ceiling {
				t.Fatalf("%s overspent at hour %d: %d > %d", strategy, h, sel.Budget.CallsSpent, ceiling)
			}
			if sel.Budget.CallsBudget != ceiling {
				t.Fatalf("%s hour %d: recorded ceiling %d", strategy, h, sel.Budget.CallsBudget)
			}
		}
		if got := s.CallsSpent(); got != ceiling {
			t.Fatalf("%s realized %d calls, want exactly N=%d", strategy, got, ceiling)
		}
	}
}

// TestPrioritySelectorUnderSpendOnShortStream pins the declared under-budget
// outcome (spec section 5.3): a thin roster that cannot fill the schedule leaves
// the arm below N and records the per-hour shortfall, never padding with
// duplicate calls.
func TestPrioritySelectorUnderSpendOnShortStream(t *testing.T) {
	const ceiling uint64 = 256
	s := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 3, ceiling, 250000, 6, 0))
	shortfall := 0
	for h := 0; h < declaredHorizon; h++ {
		sel, err := s.Select(h, testRoster(h, 1), protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatalf("hour %d: %v", h, err)
		}
		if len(sel.Selected) != 1 {
			t.Fatalf("hour %d selected %d of 1, want 1", h, len(sel.Selected))
		}
		if sel.Shortfall != 0 && sel.Shortfall != 1 {
			t.Fatalf("hour %d shortfall = %d, want 0 or 1 (q(h) in {1,2})", h, sel.Shortfall)
		}
		shortfall += sel.Shortfall
	}
	if got := s.CallsSpent(); got != declaredHorizon {
		t.Fatalf("thin roster realized %d calls, want %d (one per hour)", got, declaredHorizon)
	}
	if s.CallsSpent() >= ceiling {
		t.Fatalf("thin roster did not end under budget: %d", s.CallsSpent())
	}
	if shortfall == 0 {
		t.Fatal("thin roster recorded no shortfall against the declared quota")
	}
}

// TestPrioritySelectorAdaptiveExplorationInterleaving pins the declared A rule
// (spec section 4.5): E = ceil(q*N) exploration slots spread by T(t) = ceil(E*t/H),
// exploration is a subset of the selected set, the E budget is exactly spent on
// an ample stream, and q = 0 isolates pure exploitation.
func TestPrioritySelectorAdaptiveExplorationInterleaving(t *testing.T) {
	const ceiling uint64 = 256

	q25 := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 11, ceiling, 250000, 6, 0))
	explored := 0
	selected := 0
	for h := 0; h < declaredHorizon; h++ {
		sel, err := q25.Select(h, testRoster(h, 8), protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatalf("hour %d: %v", h, err)
		}
		inSelected := map[sim.EntityID]bool{}
		for _, id := range sel.Selected {
			inSelected[id] = true
		}
		for _, id := range sel.Exploration {
			if !inSelected[id] {
				t.Fatalf("hour %d: explored actor %d not in the selected set", h, id)
			}
		}
		explored += len(sel.Exploration)
		selected += len(sel.Selected)
	}
	if want := 64; explored != want { // E = ceil(0.25*256)
		t.Fatalf("q=0.25 explored %d calls, want E=%d", explored, want)
	}
	if selected != int(ceiling) {
		t.Fatalf("q=0.25 selected %d calls, want N=%d", selected, ceiling)
	}

	q0 := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 11, ceiling, 0, 6, 0))
	for h := 0; h < declaredHorizon; h++ {
		sel, err := q0.Select(h, testRoster(h, 8), protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatalf("q=0 hour %d: %v", h, err)
		}
		if len(sel.Exploration) != 0 {
			t.Fatalf("q=0 hour %d explored %v, want none", h, sel.Exploration)
		}
	}
	if q0.CallsSpent() != ceiling {
		t.Fatalf("q=0 realized %d calls, want N=%d", q0.CallsSpent(), ceiling)
	}
}

// TestPrioritySelectorPeriodicGate pins P's declared rule: it escalates only in
// hours h ≡ phase (mod period), in the declared deterministic actor order, with
// Quota = remaining budget and Shortfall = 0, and stops at N.
func TestPrioritySelectorPeriodicGate(t *testing.T) {
	const ceiling uint64 = 256
	s := mustSelector(t, testPolicy(cognition.StrategyPeriodic, 0, ceiling, 0, 6, 0))
	for h := 0; h < declaredHorizon; h++ {
		before := s.CallsSpent()
		sel, err := s.Select(h, testRoster(h, 10), protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatalf("hour %d: %v", h, err)
		}
		if sel.Shortfall != 0 {
			t.Fatalf("periodic hour %d recorded shortfall %d, want 0", h, sel.Shortfall)
		}
		if h%6 != 0 {
			if len(sel.Selected) != 0 {
				t.Fatalf("periodic selected %d in a gated-off hour %d", len(sel.Selected), h)
			}
			continue
		}
		// An escalated hour selects whenever budget remains; once the arm has
		// spent N it selects nothing and only then records Quota 0.
		if before < ceiling && len(sel.Selected) == 0 {
			t.Fatalf("periodic selected none in an escalated hour %d with %d budget remaining", h, ceiling-before)
		}
	}
	if s.CallsSpent() != ceiling {
		t.Fatalf("periodic realized %d calls, want N=%d", s.CallsSpent(), ceiling)
	}
}

// TestPrioritySelectorStopsAtNAndHonoursBudget pins the hard ceiling: the
// selector never selects past N, its own recorded state is authoritative (an
// unrelated world budget view does not perturb the selection), and a budget
// ceiling that disagrees with the declared one is refused.
func TestPrioritySelectorStopsAtNAndHonoursBudget(t *testing.T) {
	const ceiling uint64 = 2
	s := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 5, ceiling, 0, 6, 0))
	wantPerHour := []int{1, 1, 0, 0}
	for h, want := range wantPerHour {
		sel, err := s.Select(h, testRoster(h, 9), protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatalf("hour %d: %v", h, err)
		}
		if len(sel.Selected) != want {
			t.Fatalf("hour %d selected %d, want %d", h, len(sel.Selected), want)
		}
	}
	if s.CallsSpent() != ceiling {
		t.Fatalf("realized %d calls, want N=%d", s.CallsSpent(), ceiling)
	}

	// The accumulator owns the budget state: an in-sync world view (non-zero
	// calls spent) yields the identical selection for the same stream.
	a := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 5, 256, 0, 6, 0))
	b := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 5, 256, 0, 6, 0))
	selA, err := a.Select(0, testRoster(0, 9), protocol.BudgetState{CallsBudget: 256})
	if err != nil {
		t.Fatal(err)
	}
	selB, err := b.Select(0, testRoster(0, 9), protocol.BudgetState{CallsSpent: 0, CallsRemaining: 256, CallsBudget: 256})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selA, selB) {
		t.Fatalf("world budget view perturbed the selection: %+v vs %+v", selA, selB)
	}

	// A budget ceiling that disagrees with the declared ceiling is refused.
	if _, err := a.Select(0, testRoster(0, 9), protocol.BudgetState{CallsBudget: 99}); !errors.Is(err, cognition.ErrPrioritySelector) {
		t.Fatalf("mismatched ceiling accepted: %v", err)
	}
}

// TestPrioritySelectorDeterministicReplayFromRecordedState is VS2-EC-1's replay
// proof: a fresh selector seeded from the same declared policy identity and fed
// the same candidate stream re-derives byte-identical selections, including the
// recorded spent-call count and PRNG position, with no model call.
func TestPrioritySelectorDeterministicReplayFromRecordedState(t *testing.T) {
	const ceiling uint64 = 256
	for _, strategy := range []cognition.PriorityStrategy{
		cognition.StrategyUniformRandom, cognition.StrategyAdaptive,
	} {
		policy := testPolicy(strategy, 99, ceiling, 250000, 6, 0)
		stream := make([][]cognition.EscalationCandidate, declaredHorizon)
		for h := range stream {
			stream[h] = testRoster(h, 3+h%5)
		}
		first, firstPos := runStream(t, policy, stream)
		second, secondPos := runStream(t, policy, stream)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("%s replay diverged:\n%+v\n%+v", strategy, first, second)
		}
		if firstPos != secondPos {
			t.Fatalf("%s replay PRNG position diverged: %d vs %d", strategy, firstPos, secondPos)
		}
		if strategy == cognition.StrategyAdaptive && firstPos == 0 {
			t.Fatalf("%s consumed no policy PRNG draws", strategy)
		}
	}
}

func runStream(t *testing.T, policy cognition.WakePolicy, stream [][]cognition.EscalationCandidate) ([]cognition.Selection, uint64) {
	t.Helper()
	s, err := cognition.NewPrioritySelector(policy, declaredHorizon, protocol.CallPrice{})
	if err != nil {
		t.Fatalf("NewPrioritySelector: %v", err)
	}
	out := make([]cognition.Selection, 0, len(stream))
	for h, roster := range stream {
		sel, err := s.Select(h, roster, protocol.BudgetState{CallsBudget: policy.Priority.CallCeiling})
		if err != nil {
			t.Fatalf("hour %d: %v", h, err)
		}
		out = append(out, sel)
	}
	return out, s.PRNGPos()
}

// TestPrioritySelectorSeedDrivesPolicyPRNG pins that the policy PRNG seed is
// live: two seeds produce distinct uniform selections over a long stream, while
// the selection stays a subset of the roster.
func TestPrioritySelectorSeedDrivesPolicyPRNG(t *testing.T) {
	const ceiling uint64 = 256
	a := mustSelector(t, testPolicy(cognition.StrategyUniformRandom, 1, ceiling, 0, 6, 0))
	b := mustSelector(t, testPolicy(cognition.StrategyUniformRandom, 2, ceiling, 0, 6, 0))
	same := true
	for h := 0; h < declaredHorizon; h++ {
		roster := testRoster(h, 6)
		rosterSet := map[sim.EntityID]bool{}
		for _, c := range roster {
			rosterSet[c.Actor] = true
		}
		selA, err := a.Select(h, roster, protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatal(err)
		}
		selB, err := b.Select(h, roster, protocol.BudgetState{CallsBudget: ceiling})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range selA.Selected {
			if !rosterSet[id] {
				t.Fatalf("hour %d selected actor %d not in the roster", h, id)
			}
		}
		if !reflect.DeepEqual(selA.Selected, selB.Selected) {
			same = false
		}
	}
	if same {
		t.Fatal("two distinct RandSeed values produced identical uniform selections")
	}
}

// TestPrioritySelectorRefusesBadConfig pins the construction-time refusals: a
// nil Priority (the degenerate path), the full marker, an undeclared ceiling or
// horizon, an out-of-range exploration fraction, and an invalid periodic gate.
func TestPrioritySelectorRefusesBadConfig(t *testing.T) {
	degenerate := cognition.WakePolicy{Name: "periodic", PeriodHours: 6}
	if _, err := cognition.NewPrioritySelector(degenerate, declaredHorizon, protocol.CallPrice{}); !errors.Is(err, cognition.ErrPrioritySelector) {
		t.Fatalf("nil Priority accepted: %v", err)
	}
	bad := []cognition.WakePolicy{
		testPolicy(cognition.StrategyFull, 1, 256, 0, 6, 0),
		testPolicy(cognition.StrategyAdaptive, 1, 0, 0, 6, 0),
		testPolicy(cognition.StrategyAdaptive, 1, 256, 1_000_000, 6, 0),
		testPolicy(cognition.StrategyAdaptive, 1, 256, -1, 6, 0),
		testPolicy(cognition.StrategyPeriodic, 1, 256, 0, 6, 6),
	}
	for _, p := range bad {
		if _, err := cognition.NewPrioritySelector(p, declaredHorizon, protocol.CallPrice{}); !errors.Is(err, cognition.ErrPrioritySelector) {
			t.Fatalf("bad policy accepted: %+v", p.Priority)
		}
	}
	if _, err := cognition.NewPrioritySelector(testPolicy(cognition.StrategyAdaptive, 1, 256, 0, 6, 0), 0, protocol.CallPrice{}); !errors.Is(err, cognition.ErrPrioritySelector) {
		t.Fatalf("zero horizon accepted: %v", err)
	}
}

// TestPrioritySelectorScoresEveryRosterCandidate pins the recorded per-candidate
// projection: the score list covers the whole actor-sorted roster and its fields
// are the pure declared value projection.
func TestPrioritySelectorScoresEveryRosterCandidate(t *testing.T) {
	s := mustSelector(t, testPolicy(cognition.StrategyAdaptive, 1, 256, 250000, 6, 0))
	roster := testRoster(0, 3)
	sel, err := s.Select(0, roster, protocol.BudgetState{CallsBudget: 256})
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Scores) != len(roster) {
		t.Fatalf("recorded %d scores for %d candidates", len(sel.Scores), len(roster))
	}
	for i, score := range sel.Scores {
		// Roster order is actor-ascending in the fixture, so scores line up.
		want := protocol.Priority(testValueModel(), roster[i].Features, protocol.CallPrice{})
		if score.Priority != want {
			t.Fatalf("candidate %d priority = %d, want %d", i, score.Priority, want)
		}
		if score.Actor != roster[i].Actor {
			t.Fatalf("score %d actor = %d, want %d", i, score.Actor, roster[i].Actor)
		}
		if score.ExpectedValue != protocol.ExpectedValue(testValueModel(), roster[i].Features) {
			t.Fatalf("candidate %d expected value mismatch", i)
		}
	}
}

// TestPriceTableCallPriceAdapter pins the cognition.PriceTable -> protocol.CallPrice
// adapter: an unpriced table stays the declared unpriced zero price (the
// projection falls back to the CostProxy), and a priced table carries the rates
// into the priced projection.
func TestPriceTableCallPriceAdapter(t *testing.T) {
	f := cognition.ValueFeatures{Urgency: 1, EscalationRequested: 1}

	if got := (cognition.PriceTable{}).CallPrice(); got != (protocol.CallPrice{}) {
		t.Fatalf("unpriced table adapted to %+v, want zero", got)
	}
	model := testValueModel()
	unpricedCost := protocol.ExpectedCost(model, f, cognition.PriceTable{}.CallPrice())
	if want := model.CostProxy.ConstantMicrosPerCall + 10 + 4; unpricedCost != want {
		t.Fatalf("unpriced cost = %d, want proxy %d", unpricedCost, want)
	}

	priced := cognition.PriceTable{Version: "vs2-price/1", PromptMicrosPerToken: 3, CompletionMicrosPerToken: 7}
	want := protocol.CallPrice{Version: "vs2-price/1", PromptMicrosPerToken: 3, CompletionMicrosPerToken: 7}
	if got := priced.CallPrice(); got != want {
		t.Fatalf("priced table adapted to %+v, want %+v", got, want)
	}
	if got := protocol.ExpectedCost(model, f, priced.CallPrice()); got != 3*10+7*4 {
		t.Fatalf("priced cost = %d, want %d", got, 3*10+7*4)
	}
}
