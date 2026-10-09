package cognition

import (
	"errors"
	"fmt"
	"math"
)

// The cognitive budget is our compute and authorized spend, instrumented and
// fail-loud. It is never a modelling device and no world mechanism depends on
// it (P15, spec 8 and 8.4). The four rows are added to the ceiling ledger as
// cognitive-calls, cognitive-prompt-tokens, cognitive-completion-tokens, and
// cognitive-cost.
var (
	ErrInvalidBudget    = errors.New("invalid cognitive budget")
	ErrInvalidPrice     = errors.New("invalid price table")
	ErrCognitiveCeiling = errors.New("cognitive ceiling crossed")
	ErrPriceOverflow    = errors.New("cognitive cost overflows int64")
)

// CognitiveCeiling names one instrumented cognitive ceiling row.
type CognitiveCeiling string

const (
	CognitiveCeilingCalls            CognitiveCeiling = "cognitive-calls"
	CognitiveCeilingPromptTokens     CognitiveCeiling = "cognitive-prompt-tokens"
	CognitiveCeilingCompletionTokens CognitiveCeiling = "cognitive-completion-tokens"
	CognitiveCeilingCost             CognitiveCeiling = "cognitive-cost"
)

// cognitiveCeilingOrder is the fixed row order.
var cognitiveCeilingOrder = []CognitiveCeiling{
	CognitiveCeilingCalls, CognitiveCeilingPromptTokens,
	CognitiveCeilingCompletionTokens, CognitiveCeilingCost,
}

// CognitiveBudget is the declared ceiling (spec 8.1). The values come from our
// compute and authorized spend, justified by the pre-run projection.
type CognitiveBudget struct {
	MaxCalls            uint64 `json:"max_calls"`
	MaxPromptTokens     uint64 `json:"max_prompt_tokens"`
	MaxCompletionTokens uint64 `json:"max_completion_tokens"`
	MaxCostMicros       int64  `json:"max_cost_micros"`
}

// Validate requires a positive declared ceiling on every axis.
func (b CognitiveBudget) Validate() error {
	if b.MaxCalls == 0 || b.MaxPromptTokens == 0 || b.MaxCompletionTokens == 0 || b.MaxCostMicros <= 0 {
		return fmt.Errorf("%w: every ceiling must be declared and positive", ErrInvalidBudget)
	}
	return nil
}

// CognitiveRow is one instrumented ceiling row, shaped like the existing
// compute-ceiling rows: threshold, observed, trailing-hour rate, and the
// projected binding hour (-1 when not projectable).
type CognitiveRow struct {
	Ceiling              CognitiveCeiling `json:"ceiling"`
	Threshold            uint64           `json:"threshold"`
	Observed             uint64           `json:"observed"`
	RateLastHour         int64            `json:"rate_last_hour"`
	ProjectedBindingHour int64            `json:"projected_binding_hour"`
}

// CognitiveTotals is the per-run accumulator.
type CognitiveTotals struct {
	Calls            uint64
	PromptTokens     uint64
	CompletionTokens uint64
	CostMicros       int64
}

// CognitiveCeilingHit is the typed first-class evidence of a crossing.
type CognitiveCeilingHit struct {
	Ceiling   CognitiveCeiling `json:"ceiling"`
	Threshold uint64           `json:"threshold"`
	Observed  uint64           `json:"observed"`
	Hour      int              `json:"hour"`
}

// CognitiveCeilingHitError carries the typed hit; the run stops fail-loud.
type CognitiveCeilingHitError struct{ Hit CognitiveCeilingHit }

func (e *CognitiveCeilingHitError) Error() string {
	return fmt.Sprintf("%s: ceiling %q threshold %d observed %d at hour %d",
		ErrCognitiveCeiling, string(e.Hit.Ceiling), e.Hit.Threshold, e.Hit.Observed, e.Hit.Hour)
}

func (e *CognitiveCeilingHitError) Is(target error) bool { return target == ErrCognitiveCeiling }

// PriceTable converts usage to cost. It is versioned; the version is recorded
// with every call.
type PriceTable struct {
	Version                  string
	PromptMicrosPerToken     int64
	CompletionMicrosPerToken int64
}

// Validate requires a named version and non-negative rates.
func (p PriceTable) Validate() error {
	if p.Version == "" || p.PromptMicrosPerToken < 0 || p.CompletionMicrosPerToken < 0 {
		return fmt.Errorf("%w: version and non-negative rates required", ErrInvalidPrice)
	}
	return nil
}

// CostMicros prices one attempt, refusing overflow rather than saturating.
func (p PriceTable) CostMicros(u Usage) (int64, error) {
	prompt, err := mulCost(u.PromptTokens, p.PromptMicrosPerToken)
	if err != nil {
		return 0, err
	}
	completion, err := mulCost(u.CompletionTokens, p.CompletionMicrosPerToken)
	if err != nil {
		return 0, err
	}
	if prompt > math.MaxInt64-completion {
		return 0, ErrPriceOverflow
	}
	return prompt + completion, nil
}

func mulCost(tokens uint64, microsPerToken int64) (int64, error) {
	if tokens == 0 || microsPerToken == 0 {
		return 0, nil
	}
	if tokens > math.MaxInt64 || microsPerToken > math.MaxInt64 {
		return 0, ErrPriceOverflow
	}
	if uint64(microsPerToken) > uint64(math.MaxInt64)/tokens {
		return 0, ErrPriceOverflow
	}
	return int64(tokens) * microsPerToken, nil
}

// CognitiveAccount accumulates calls, tokens, and cost and fails loud at the
// first ceiling crossing. It never truncates a trajectory and never silently
// substitutes a cheap decision for a call.
type CognitiveAccount struct {
	budget     CognitiveBudget
	calls      uint64
	prompt     uint64
	completion uint64
	costMicros int64
	lastHour   int
	lastTotals [4]uint64
	rows       []CognitiveRow
	hit        *CognitiveCeilingHit
}

// NewCognitiveAccount builds the fail-loud account for a declared budget.
func NewCognitiveAccount(budget CognitiveBudget) (*CognitiveAccount, error) {
	if err := budget.Validate(); err != nil {
		return nil, err
	}
	return &CognitiveAccount{budget: budget, lastHour: -1}, nil
}

// Budget returns the declared ceiling.
func (a *CognitiveAccount) Budget() CognitiveBudget { return a.budget }

// AllowCall refuses before the spend when no call remains under MaxCalls. It is
// the pre-call guard; Charge is the post-call guard.
func (a *CognitiveAccount) AllowCall(hour int) error {
	if a.hit != nil {
		return &CognitiveCeilingHitError{Hit: *a.hit}
	}
	if a.calls+1 > a.budget.MaxCalls {
		return a.cross(CognitiveCeilingCalls, a.budget.MaxCalls, a.calls+1, hour)
	}
	return nil
}

// Charge records one attempt's usage and cost and fails loud at the first
// crossing. A crossing is typed: the caller persists the ledger record and
// halts; nothing is truncated.
func (a *CognitiveAccount) Charge(hour int, usage Usage, costMicros int64) error {
	if a.hit != nil {
		return &CognitiveCeilingHitError{Hit: *a.hit}
	}
	a.calls++
	a.prompt += usage.PromptTokens
	a.completion += usage.CompletionTokens
	a.costMicros += costMicros
	if a.calls > a.budget.MaxCalls {
		return a.cross(CognitiveCeilingCalls, a.budget.MaxCalls, a.calls, hour)
	}
	if a.prompt > a.budget.MaxPromptTokens {
		return a.cross(CognitiveCeilingPromptTokens, a.budget.MaxPromptTokens, a.prompt, hour)
	}
	if a.completion > a.budget.MaxCompletionTokens {
		return a.cross(CognitiveCeilingCompletionTokens, a.budget.MaxCompletionTokens, a.completion, hour)
	}
	if a.costMicros > a.budget.MaxCostMicros {
		return a.cross(CognitiveCeilingCost, uint64(a.budget.MaxCostMicros), uint64(a.costMicros), hour)
	}
	return nil
}

func (a *CognitiveAccount) cross(ceiling CognitiveCeiling, threshold, observed uint64, hour int) error {
	a.hit = &CognitiveCeilingHit{Ceiling: ceiling, Threshold: threshold, Observed: observed, Hour: hour}
	return &CognitiveCeilingHitError{Hit: *a.hit}
}

// Hit returns the recorded crossing, if any.
func (a *CognitiveAccount) Hit() (CognitiveCeilingHit, bool) {
	if a.hit == nil {
		return CognitiveCeilingHit{}, false
	}
	return *a.hit, true
}

// Totals returns the per-run accumulator.
func (a *CognitiveAccount) Totals() CognitiveTotals {
	return CognitiveTotals{Calls: a.calls, PromptTokens: a.prompt, CompletionTokens: a.completion, CostMicros: a.costMicros}
}

// TotalsForHour returns the totals at the given hour for the run's ceiling
// report.
func (a *CognitiveAccount) observedValues() [4]uint64 {
	return [4]uint64{a.calls, a.prompt, a.completion, uint64(a.costMicros)}
}

// ObserveHour snapshots the current totals as hour's trailing window and
// returns the four ceiling rows with trailing rate and projected binding hour.
func (a *CognitiveAccount) ObserveHour(hour int) []CognitiveRow {
	current := a.observedValues()
	rows := make([]CognitiveRow, 0, len(cognitiveCeilingOrder))
	for i, ceiling := range cognitiveCeilingOrder {
		threshold := a.threshold(ceiling)
		rate := int64(current[i]) - int64(a.lastTotals[i])
		rows = append(rows, CognitiveRow{
			Ceiling:              ceiling,
			Threshold:            threshold,
			Observed:             current[i],
			RateLastHour:         rate,
			ProjectedBindingHour: projectBindingHour(threshold, current[i], rate, hour),
		})
	}
	a.lastTotals = current
	a.lastHour = hour
	a.rows = rows
	return rows
}

// Rows returns the last observed ceiling rows.
func (a *CognitiveAccount) Rows() []CognitiveRow {
	return append([]CognitiveRow(nil), a.rows...)
}

func (a *CognitiveAccount) threshold(ceiling CognitiveCeiling) uint64 {
	switch ceiling {
	case CognitiveCeilingCalls:
		return a.budget.MaxCalls
	case CognitiveCeilingPromptTokens:
		return a.budget.MaxPromptTokens
	case CognitiveCeilingCompletionTokens:
		return a.budget.MaxCompletionTokens
	case CognitiveCeilingCost:
		return uint64(a.budget.MaxCostMicros)
	default:
		return 0
	}
}

// projectBindingHour is the pure projection shared with the compute ceiling
// ledger: with a positive trailing rate the ceiling binds after
// ceil(headroom/rate) further hours; already over binds now.
func projectBindingHour(threshold, observed uint64, rate int64, hour int) int64 {
	if observed > threshold {
		return int64(hour)
	}
	if rate <= 0 {
		return -1
	}
	headroom := int64(threshold - observed)
	return int64(hour) + (headroom+rate-1)/rate
}
