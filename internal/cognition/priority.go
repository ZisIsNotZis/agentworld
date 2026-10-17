package cognition

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// The arm-local deterministic online accumulator (spec section 4.1 item 4,
// section 5.1, VS2-EC-1/EC-2/EC-7). It is the cognition-side implementation of
// the L1 protocol.EscalationSelector interface: one selector per budgeted VS2
// arm, seeded from the declared call ceiling N and the policy PRNG seed. It is
// online (it consumes each hour's roster as it arrives), spends at most the
// declared per-hour quota q(h) for U and A (or is period-gated for P), and stops
// once it has spent N. It consumes no world random stream and needs no
// run-total candidate count, no reservoir, and no model-free pre-pass; replay
// from the declared policy identity and the arm's own candidate stream
// re-derives the identical selection sequence without a model call.
var ErrPrioritySelector = errors.New("invalid priority selector")

// PrioritySelector is the declared arm-local online accumulator for one
// budgeted arm. Its state is exactly the recorded accumulator state: the
// spent-call count and the consumed policy-PRNG position, both echoed on every
// Selection (spec section 4.6).
type PrioritySelector struct {
	strategy     protocol.PriorityStrategy
	periodHours  int
	phaseHours   int
	callCeiling  uint64
	horizonHours int
	exploration  int64 // ExplorationFraction over ExplorationScale, q in [0,1)
	valueModel   protocol.ValueModelSpec
	price        protocol.CallPrice

	prng    *rand.Rand
	spent   uint64
	prngPos uint64

	// lastSelectedHour and callsPerActor are the declared vs2-value/2 selector
	// state: when each actor was last reasoned about, and how many calls it has
	// realised. One selector owns both, so replay re-derives them from the
	// recorded selection sequence. They are inert for a spec that declares no
	// staleness or coverage term.
	lastSelectedHour map[sim.EntityID]int
	callsPerActor    map[sim.EntityID]int64
}

// NewPrioritySelector builds the arm-local accumulator for a priority-bearing
// WakePolicy. It refuses a policy with a nil Priority (that is the degenerate
// WakePolicy.Select path, used by the reference R and the VS1 path), the
// StrategyFull marker, an undeclared ceiling or horizon, an out-of-range
// exploration fraction, and a periodic strategy without a valid period/phase.
// horizonHours is the declared evaluation horizon H (168 h for VS2); price is
// the declared protocol-local call price, zero when the model is unpriced.
func NewPrioritySelector(policy protocol.WakePolicy, horizonHours int, price protocol.CallPrice) (*PrioritySelector, error) {
	p := policy.Priority
	if p == nil {
		return nil, fmt.Errorf("%w: nil Priority is the degenerate WakePolicy.Select path", ErrPrioritySelector)
	}
	switch p.Strategy {
	case protocol.StrategyPeriodic, protocol.StrategyUniformRandom, protocol.StrategyAdaptive:
	case protocol.StrategyFull:
		return nil, fmt.Errorf("%w: strategy %q carries a nil Priority", ErrPrioritySelector, p.Strategy)
	default:
		return nil, fmt.Errorf("%w: unknown strategy %q", ErrPrioritySelector, p.Strategy)
	}
	if p.CallCeiling == 0 {
		return nil, fmt.Errorf("%w: undeclared call ceiling", ErrPrioritySelector)
	}
	if horizonHours < 1 {
		return nil, fmt.Errorf("%w: horizon %d", ErrPrioritySelector, horizonHours)
	}
	if p.ExplorationFraction < 0 || p.ExplorationFraction >= protocol.ExplorationScale {
		return nil, fmt.Errorf("%w: exploration fraction %d outside [0,%d)", ErrPrioritySelector, p.ExplorationFraction, protocol.ExplorationScale)
	}
	if p.Strategy == protocol.StrategyPeriodic && !periodValid(policy.PeriodHours, policy.PhaseHours) {
		return nil, fmt.Errorf("%w: periodic period %d phase %d", ErrPrioritySelector, policy.PeriodHours, policy.PhaseHours)
	}
	return &PrioritySelector{
		strategy:     p.Strategy,
		periodHours:  policy.PeriodHours,
		phaseHours:   policy.PhaseHours,
		callCeiling:  p.CallCeiling,
		horizonHours: horizonHours,
		exploration:  p.ExplorationFraction,
		valueModel:   p.ValueModel,
		price:        price,
		prng:         rand.New(rand.NewSource(int64(p.RandSeed))),

		lastSelectedHour: map[sim.EntityID]int{},
		callsPerActor:    map[sim.EntityID]int64{},
	}, nil
}

// CallsSpent is the recorded spent-call count (the accumulator run state).
func (s *PrioritySelector) CallsSpent() uint64 { return s.spent }

// PRNGPos is the recorded consumed policy-PRNG position (the number of draws).
func (s *PrioritySelector) PRNGPos() uint64 { return s.prngPos }

// CallCeiling is the declared call ceiling N the selector was seeded with.
func (s *PrioritySelector) CallCeiling() uint64 { return s.callCeiling }

// Select implements protocol.EscalationSelector for one candidate hour. It is
// online: it spends at most the declared per-hour quota q(h) (U/A) or the
// remaining budget (P), stops at N, and records the selection, the per-candidate
// scores, the exploration set, the budget state, and the PRNG position. The
// accumulator owns the authoritative budget state (spec section 4.1 item 5), so
// the selection depends only on the declared policy identity, the candidate
// stream, and the accumulator state; the passed budget is the world's declared
// view and is used only to reject a ceiling that disagrees with the declared N.
func (s *PrioritySelector) Select(hour int, roster []protocol.EscalationCandidate, budget protocol.BudgetState) (protocol.Selection, error) {
	if budget.CallsBudget != 0 && budget.CallsBudget != s.callCeiling {
		return protocol.Selection{}, fmt.Errorf("%w: budget ceiling %d, declared %d", ErrPrioritySelector, budget.CallsBudget, s.callCeiling)
	}
	remaining := s.remaining()
	switch s.strategy {
	case protocol.StrategyPeriodic:
		return s.selectPeriodic(hour, roster, remaining), nil
	case protocol.StrategyUniformRandom:
		return s.selectUniform(hour, roster, remaining), nil
	case protocol.StrategyAdaptive:
		return s.selectAdaptive(hour, roster, remaining), nil
	default:
		return protocol.Selection{}, fmt.Errorf("%w: strategy %q", ErrPrioritySelector, s.strategy)
	}
}

// selectPeriodic is the period-gated baseline P: it escalates candidates only
// in selected hours, in the declared deterministic roster order (sorted by
// actor), and stops at the remaining budget. It has no per-hour quota, so its
// Quota is the remaining budget (N - CallsSpent) and its Shortfall is 0 (spec
// section 5.1).
func (s *PrioritySelector) selectPeriodic(hour int, roster []protocol.EscalationCandidate, remaining uint64) protocol.Selection {
	order := sortedRoster(roster)
	scores := s.scoreCandidates(order, hour)
	selected := make([]sim.EntityID, 0, len(order))
	if periodGate(s.periodHours, s.phaseHours, hour) {
		for _, c := range order {
			if uint64(len(selected)) >= remaining {
				break
			}
			selected = append(selected, c.Actor)
		}
		s.spent += uint64(len(selected))
		s.recordSelection(selected, hour)
	}
	return s.finish(hour, selected, nil, scores, int(remaining), 0)
}

// selectUniform is the uniform-random baseline U: at each hour it spends at most
// the declared per-hour quota q(h) on candidates chosen by the policy PRNG,
// drawn without replacement from that hour's roster (sorted by actor), and stops
// at N.
func (s *PrioritySelector) selectUniform(hour int, roster []protocol.EscalationCandidate, remaining uint64) protocol.Selection {
	order := sortedRoster(roster)
	scores := s.scoreCandidates(order, hour)
	quota := s.quota(hour)
	shortfall := shortfallOf(quota, len(order))
	selected := []sim.EntityID{}
	pool := actorsOf(order)
	for i, limit := 0, min(quota, int(remaining), len(pool)); i < limit; i++ {
		idx := s.draw(len(pool))
		selected = append(selected, pool[idx])
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	s.spent += uint64(len(selected))
	s.recordSelection(selected, hour)
	return s.finish(hour, selected, nil, scores, quota, shortfall)
}

// selectAdaptive is the adaptive-priority candidate A. At each hour it offers up
// to the declared per-hour quota q(h) slots over that hour's candidates in the
// declared deterministic order (sorted by actor): the first min(e(h), slots
// offered) slots are protected-exploration slots and the rest are priority-
// ranked exploitation slots, per the declared section 4.5 interleaving rule.
// Exploration draws uniformly from the hour's not-yet-selected candidates with
// the policy PRNG; exploitation takes the highest-Priority remaining candidate,
// ties broken by actor id. The arm stops at N. With q = 0 every slot is
// exploitation.
func (s *PrioritySelector) selectAdaptive(hour int, roster []protocol.EscalationCandidate, remaining uint64) protocol.Selection {
	order := sortedRoster(roster)
	scores := s.scoreCandidates(order, hour)
	quota := s.quota(hour)
	shortfall := shortfallOf(quota, len(order))
	slots := min(quota, int(remaining), len(order))
	exploreSlots := min(s.explorationSlots(hour), slots)

	pool := append([]protocol.EscalationCandidate(nil), order...)
	selected := []sim.EntityID{}
	exploration := []sim.EntityID{}
	fairShare := s.fairShare(len(order))
	for i := 0; i < slots; i++ {
		var idx int
		if i < exploreSlots {
			idx = s.draw(len(pool))
			exploration = append(exploration, pool[idx].Actor)
		} else {
			idx = s.bestAdjustedIndex(pool, hour, fairShare)
		}
		selected = append(selected, pool[idx].Actor)
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	s.spent += uint64(len(selected))
	s.recordSelection(selected, hour)
	return s.finish(hour, selected, exploration, scores, quota, shortfall)
}

// finish builds the recorded Selection and echoes the post-selection
// accumulator state. Every entity/score slice is non-nil so the canonical wire
// is deterministic ([] never null).
func (s *PrioritySelector) finish(hour int, selected, exploration []sim.EntityID, scores []protocol.CandidateScore, quota, shortfall int) protocol.Selection {
	if selected == nil {
		selected = []sim.EntityID{}
	}
	if exploration == nil {
		exploration = []sim.EntityID{}
	}
	if scores == nil {
		scores = []protocol.CandidateScore{}
	}
	return protocol.Selection{
		Hour:        hour,
		Selected:    selected,
		Exploration: exploration,
		Scores:      scores,
		Budget: protocol.BudgetState{
			CallsSpent:     s.spent,
			CallsRemaining: s.callCeiling - s.spent,
			CallsBudget:    s.callCeiling,
		},
		Quota:     quota,
		Shortfall: shortfall,
		PRNGPos:   s.prngPos,
	}
}

// remaining is the undecremented budget: N - CallsSpent, floored at 0.
func (s *PrioritySelector) remaining() uint64 {
	if s.spent >= s.callCeiling {
		return 0
	}
	return s.callCeiling - s.spent
}

// quota is the declared fixed per-hour schedule q(h) = floor(N/H) +
// (1 if h < N mod H else 0), so sum_{h<H} q(h) = N. It depends on no arm's
// realized stream (spec section 5.1).
func (s *PrioritySelector) quota(hour int) int {
	q := s.callCeiling / uint64(s.horizonHours)
	if h := uint64(hour); h < s.callCeiling%uint64(s.horizonHours) {
		q++
	}
	return int(q)
}

// reservedExploration is E = ceil(q*N) = (N*frac + scale - 1)/scale, the
// declared reserved protected-exploration calls (spec section 4.5).
func (s *PrioritySelector) reservedExploration() int64 {
	return (int64(s.callCeiling)*s.exploration + protocol.ExplorationScale - 1) / protocol.ExplorationScale
}

// explorationTarget is T(t) = ceil(E*t/H), clamped so the cumulative target
// reaches E at the horizon and never exceeds it. It is a declared fixed schedule
// independent of any realized stream.
func (s *PrioritySelector) explorationTarget(t int) int64 {
	if t < 0 {
		t = 0
	}
	if t > s.horizonHours {
		t = s.horizonHours
	}
	return ceilDiv(s.reservedExploration()*int64(t), int64(s.horizonHours))
}

// explorationSlots is e(h) = T(h+1) - T(h), hour h's declared exploration
// allocation.
func (s *PrioritySelector) explorationSlots(hour int) int {
	return int(s.explorationTarget(hour+1) - s.explorationTarget(hour))
}

// draw advances the policy PRNG by one uniform draw over n items and returns the
// index. prngPos counts consumed draws, so replay re-derives the exact stream.
func (s *PrioritySelector) draw(n int) int {
	idx := s.prng.Intn(n)
	s.prngPos++
	return idx
}

// bestAdjustedIndex returns the index of the highest adjusted-priority candidate
// in the actor-sorted pool, so an equal-priority tie keeps the smallest actor id.
// Under vs2-value/2 the adjustment folds in the staleness and coverage terms;
// under vs2-value/1 it is the pure projection.
func (s *PrioritySelector) bestAdjustedIndex(pool []protocol.EscalationCandidate, hour int, fairShare int64) int {
	best, bestPriority := 0, s.adjustedPriority(pool[0].Actor, pool[0].Features, hour, fairShare)
	for i := 1; i < len(pool); i++ {
		if p := s.adjustedPriority(pool[i].Actor, pool[i].Features, hour, fairShare); p > bestPriority {
			best, bestPriority = i, p
		}
	}
	return best
}

// adjustedPriority is one candidate's selection priority under the declared value
// model. hoursSinceCall is the elapsed hours since the actor was last selected;
// an actor never selected is treated as fully stale so the coverage floor, not
// the staleness term, gives it its first slot.
func (s *PrioritySelector) adjustedPriority(actor sim.EntityID, features protocol.ValueFeatures, hour int, fairShare int64) int64 {
	since := s.horizonHours + 1
	if last, ok := s.lastSelectedHour[actor]; ok {
		since = hour - last
	}
	return protocol.AdjustedPriority(s.valueModel, features, s.price, since, s.callsPerActor[actor], fairShare)
}

// fairShare is the realised per-actor fair share of the spent budget: the calls
// already spent divided by the roster size of this hour. It is the coverage
// floor's reference point, so the boost vanishes once an actor is at share.
func (s *PrioritySelector) fairShare(rosterSize int) int64 {
	if rosterSize <= 0 {
		return 0
	}
	return int64(s.spent) / int64(rosterSize)
}

// recordSelection records the vs2-value/2 selection state for one hour's selected
// actors: each selected actor's last-selected hour and realised call count.
func (s *PrioritySelector) recordSelection(selected []sim.EntityID, hour int) {
	for _, actor := range selected {
		s.lastSelectedHour[actor] = hour
		s.callsPerActor[actor]++
	}
}

// CallsPerActor is the recorded per-actor realised call count of the selector
// (the coverage the vs2-value/2 floor acts on).
func (s *PrioritySelector) CallsPerActor() map[sim.EntityID]int64 {
	out := make(map[sim.EntityID]int64, len(s.callsPerActor))
	for k, v := range s.callsPerActor {
		out[k] = v
	}
	return out
}

// scoreCandidates records the declared per-candidate projection for every
// roster candidate, in the declared actor-sorted order.
func (s *PrioritySelector) scoreCandidates(order []protocol.EscalationCandidate, hour int) []protocol.CandidateScore {
	scores := make([]protocol.CandidateScore, 0, len(order))
	fairShare := s.fairShare(len(order))
	for _, c := range order {
		score := protocol.CandidateScore{
			Actor:         c.Actor,
			ExpectedValue: protocol.ExpectedValue(s.valueModel, c.Features),
			ExpectedCost:  protocol.ExpectedCost(s.valueModel, c.Features, s.price),
			Priority:      protocol.Priority(s.valueModel, c.Features, s.price),
			Requested:     c.Requested,
		}
		if s.valueModel.StalenessRefreshHours > 0 || s.valueModel.CoverageFloor {
			score.AdjustedPriority = s.adjustedPriority(c.Actor, c.Features, hour, fairShare)
		}
		scores = append(scores, score)
	}
	return scores
}

// sortedRoster returns the roster in the declared deterministic order (sorted
// by actor), leaving the caller's slice untouched.
func sortedRoster(roster []protocol.EscalationCandidate) []protocol.EscalationCandidate {
	out := append([]protocol.EscalationCandidate(nil), roster...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Actor < out[j].Actor })
	return out
}

// actorsOf projects an actor-sorted roster to its actor ids.
func actorsOf(order []protocol.EscalationCandidate) []sim.EntityID {
	out := make([]sim.EntityID, 0, len(order))
	for _, c := range order {
		out = append(out, c.Actor)
	}
	return out
}

// shortfallOf is the recorded per-hour shortfall: max(0, q(h) - roster size).
func shortfallOf(quota, rosterSize int) int {
	if quota <= rosterSize {
		return 0
	}
	return quota - rosterSize
}

// ceilDiv is the non-negative integer ceiling division a/b.
func ceilDiv(a, b int64) int64 { return (a + b - 1) / b }

// periodValid reports whether a periodic policy's period/phase can gate hours.
func periodValid(period, phase int) bool {
	return period >= 1 && phase >= 0 && phase < period
}

// periodGate is the pure periodic escalation predicate h ≡ phase (mod period).
func periodGate(period, phase, hour int) bool {
	if !periodValid(period, phase) {
		return false
	}
	offset := (hour - phase) % period
	if offset < 0 {
		offset += period
	}
	return offset == 0
}

// CallPrice adapts the declared cognition.PriceTable to the protocol-local
// protocol.CallPrice the pure value projection takes. internal/cognition imports
// protocol, so the adapter lives on this side; protocol cannot import PriceTable
// without an import cycle. An empty version is the declared unpriced state, which
// the projection reads as "use the declared CostProxy".
func (p PriceTable) CallPrice() protocol.CallPrice {
	return protocol.CallPrice{
		Version:                  p.Version,
		PromptMicrosPerToken:     p.PromptMicrosPerToken,
		CompletionMicrosPerToken: p.CompletionMicrosPerToken,
	}
}
