package main

import (
	"errors"
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

// TestQuotaScheduleSumAndShape pins q(h) = floor((N-R)/H) + (1 if h < (N-R) mod H)
// and sum_{h<H} q(h) = N - R: the declared schedule stops the transport reserve
// short of the ceiling.
func TestQuotaScheduleSumAndShape(t *testing.T) {
	q := quotaSchedule(256, 168)
	if len(q) != 168 {
		t.Fatalf("schedule length %d", len(q))
	}
	if got := sumInts(q); got != 248 {
		t.Fatalf("sum q(h) = %d, want N-R = 248", got)
	}
	if q[0] != 2 || q[79] != 2 || q[80] != 1 || q[167] != 1 {
		t.Fatalf("q(h) shape: q0=%d q79=%d q80=%d q167=%d", q[0], q[79], q[80], q[167])
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

// TestArmStopsAtDecisionBudget pins that every budgeted strategy realizes
// exactly the decision budget N-R on an ample stream and never exceeds it.
func TestArmStopsAtDecisionBudget(t *testing.T) {
	const ceiling, horizon = 16, 8
	want := cognition.DecisionBudget(ceiling)
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
			if uint64(total) > want {
				t.Fatalf("%s exceeded the decision budget at hour %d: %d", arm, h, total)
			}
		}
		if uint64(total) != want {
			t.Fatalf("%s realized %d, want exactly N-R=%d", arm, total, want)
		}
		if sel.CallsSpent() != want {
			t.Fatalf("%s CallsSpent=%d, want %d", arm, sel.CallsSpent(), want)
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

// TestDeclaredScheduleLeavesTheTransportReserve pins the declared pair the
// schedule is built from (ticket 40): N = 256 is the fail-loud attempt ceiling
// and R = 8 is the transport reserve, so the recorded schedule selects 248
// decisions and leaves 8 attempts unconsumed for retries.
func TestDeclaredScheduleLeavesTheTransportReserve(t *testing.T) {
	if DeclaredCallCeiling != 256 {
		t.Fatalf("declared call ceiling N = %d, want the declared 256", DeclaredCallCeiling)
	}
	if cognition.DeclaredTransportReserveAttempts != 8 {
		t.Fatalf("declared transport reserve R = %d, want the declared 8", cognition.DeclaredTransportReserveAttempts)
	}
	if got := sumInts(quotaSchedule(DeclaredCallCeiling, DeclaredHorizonHours)); got != 248 {
		t.Fatalf("sum q(h) = %d, want N-R = 248", got)
	}
	if got := uint64(sumInts(quotaSchedule(DeclaredCallCeiling, DeclaredHorizonHours))) + cognition.DeclaredTransportReserveAttempts; got != DeclaredCallCeiling {
		t.Fatalf("schedule + reserve = %d, want N = %d", got, DeclaredCallCeiling)
	}
}

// reserveCellRun is one declared budgeted cell driven through both accounting
// units: decisions (the schedule) and physical attempts (the fail-loud account).
type reserveCellRun struct {
	decisions uint64
	attempts  uint64
	unspent   uint64
	hit       *cognition.CognitiveCeilingHit
}

// runReserveCell drives arm's declared selector and its declared cognitive
// account together over the declared horizon, charging one physical attempt per
// selected decision plus retries[h] extra attempts in hour h. A retry buys no
// decision but costs an attempt, which is the unit mismatch ticket 40 repairs
// (ticket 39 recorded one 300 s timeout retried once inside the attempt
// ceiling). It stops at the first typed crossing and reports what was spent.
func runReserveCell(t *testing.T, arm string, retries map[int]int) reserveCellRun {
	t.Helper()
	sel, err := cognition.NewPrioritySelector(declaredPolicy(arm, 0), DeclaredHorizonHours, selectorPrice())
	if err != nil {
		t.Fatalf("%s: new selector: %v", arm, err)
	}
	acct, err := cognition.NewCognitiveAccount(budgetFor(arm))
	if err != nil {
		t.Fatalf("%s: new account: %v", arm, err)
	}
	run := reserveCellRun{}
	stop := func(err error, hour int) reserveCellRun {
		t.Helper()
		var hit *cognition.CognitiveCeilingHitError
		if !errors.As(err, &hit) {
			t.Fatalf("%s hour %d: want the typed fail-loud halt, got %v", arm, hour, err)
		}
		run.hit = &hit.Hit
		run.unspent = DeclaredCallCeiling - run.attempts
		return run
	}
	for h := 0; h < DeclaredHorizonHours; h++ {
		selection, err := sel.Select(h, testRoster(8, h), protocol.BudgetState{CallsBudget: DeclaredCallCeiling})
		if err != nil {
			t.Fatalf("%s hour %d: select: %v", arm, h, err)
		}
		run.decisions += uint64(len(selection.Selected))
		for i := 0; i < len(selection.Selected)+retries[h]; i++ {
			if err := acct.AllowCall(h); err != nil {
				return stop(err, h)
			}
			if err := acct.Charge(h, cognition.Usage{PromptTokens: 16, CompletionTokens: 8}, 0); err != nil {
				return stop(err, h)
			}
			run.attempts++
		}
	}
	run.unspent = DeclaredCallCeiling - run.attempts
	return run
}

// TestCellWithoutRetriesLeavesTheReserveUnspent pins the first consequence: a
// quota-bound cell that never retries completes all N-R decisions and finishes
// with exactly R attempts unspent under the ceiling.
func TestCellWithoutRetriesLeavesTheReserveUnspent(t *testing.T) {
	for _, arm := range []string{ArmU, ArmA, ArmA1} {
		run := runReserveCell(t, arm, nil)
		if run.hit != nil {
			t.Fatalf("%s halted without a retry: %+v", arm, *run.hit)
		}
		if run.decisions != 248 || run.attempts != 248 {
			t.Fatalf("%s made %d decisions in %d attempts, want 248/248", arm, run.decisions, run.attempts)
		}
		if run.unspent != cognition.DeclaredTransportReserveAttempts {
			t.Fatalf("%s left %d attempts under the ceiling, want the declared reserve R=%d",
				arm, run.unspent, cognition.DeclaredTransportReserveAttempts)
		}
	}
}

// TestCellRetryingWithinTheReserveStillCompletes pins the repair: a cell whose
// transport retries consume exactly R attempts still makes all N-R decisions and
// never crosses the ceiling, which is what ticket 39's A1-r1 could not do at the
// old zero reserve.
func TestCellRetryingWithinTheReserveStillCompletes(t *testing.T) {
	retries := map[int]int{}
	for h := 10; h <= 80; h += 10 { // R = 8 single retries
		retries[h] = 1
	}
	for _, arm := range []string{ArmU, ArmA, ArmA1} {
		run := runReserveCell(t, arm, retries)
		if run.hit != nil {
			t.Fatalf("%s halted inside the reserve: %+v", arm, *run.hit)
		}
		if run.decisions != 248 {
			t.Fatalf("%s made %d decisions, want 248", arm, run.decisions)
		}
		if run.attempts != DeclaredCallCeiling {
			t.Fatalf("%s spent %d attempts, want the ceiling N=%d exactly", arm, run.attempts, DeclaredCallCeiling)
		}
		if run.unspent != 0 {
			t.Fatalf("%s left %d attempts, want 0 after R retries", arm, run.unspent)
		}
	}
}

// TestReservePlusOneRetryStillHaltsTypedOnTheNPlusOnethAttempt pins that the
// frozen instrument is untouched (VS2-EC-2): the reserve changes only how many
// decisions the schedule selects, so one retry beyond it is refused on the
// (N+1)-th attempt with the same typed call-ceiling halt -- here in the last
// hour, after every decision of a completed schedule has been made.
func TestReservePlusOneRetryStillHaltsTypedOnTheNPlusOnethAttempt(t *testing.T) {
	retries := map[int]int{}
	for h := 10; h <= 80; h += 10 { // R = 8 retries, all inside the reserve
		retries[h] = 1
	}
	retries[DeclaredHorizonHours-1] = 1 // the ninth, one past the reserve
	run := runReserveCell(t, ArmA1, retries)
	if run.hit == nil {
		t.Fatal("a retry past the reserve must cross the declared ceiling")
	}
	if run.hit.Ceiling != cognition.CognitiveCeilingCalls {
		t.Fatalf("halted on %q, want %q", run.hit.Ceiling, cognition.CognitiveCeilingCalls)
	}
	if run.hit.Threshold != DeclaredCallCeiling || run.hit.Observed != DeclaredCallCeiling+1 {
		t.Fatalf("threshold/observed = %d/%d, want %d/%d",
			run.hit.Threshold, run.hit.Observed, DeclaredCallCeiling, DeclaredCallCeiling+1)
	}
	if run.hit.Hour != DeclaredHorizonHours-1 {
		t.Fatalf("halted at hour %d, want the last hour", run.hit.Hour)
	}
	if run.decisions != 248 {
		t.Fatalf("made %d decisions before the halt, want the whole schedule 248", run.decisions)
	}
	if run.attempts != DeclaredCallCeiling {
		t.Fatalf("spent %d attempts before the refusal, want N=%d", run.attempts, DeclaredCallCeiling)
	}
}

// TestPeriodicArmReserveIsNotItsBindingStop pins that P is unaffected in kind:
// its gate supplies fewer decisions than the decision budget, so it ends with at
// least the reserve unspent and never retries into the ceiling.
func TestPeriodicArmReserveIsNotItsBindingStop(t *testing.T) {
	run := runReserveCell(t, ArmP, nil)
	if run.hit != nil {
		t.Fatalf("P halted: %+v", *run.hit)
	}
	if run.decisions >= 248 {
		t.Fatalf("P made %d decisions; this fixture expects its gate to bind first", run.decisions)
	}
	if run.unspent < cognition.DeclaredTransportReserveAttempts {
		t.Fatalf("P left %d attempts, want at least the reserve R=%d", run.unspent, cognition.DeclaredTransportReserveAttempts)
	}
}
