package cognition_test

import (
	"errors"
	"testing"

	"agentworld/internal/cognition"
)

func TestBudgetAllowCallRefusesBeforeSpend(t *testing.T) {
	acct := newAccount(t, cognition.CognitiveBudget{MaxCalls: 1, MaxPromptTokens: 100, MaxCompletionTokens: 100, MaxCostMicros: 1000})
	if err := acct.AllowCall(0); err != nil {
		t.Fatal(err)
	}
	if err := acct.Charge(0, cognition.Usage{PromptTokens: 10, CompletionTokens: 5}, 20); err != nil {
		t.Fatal(err)
	}
	err := acct.AllowCall(0)
	var hit *cognition.CognitiveCeilingHitError
	if !errors.As(err, &hit) || hit.Hit.Ceiling != cognition.CognitiveCeilingCalls {
		t.Fatalf("want the pre-call call-ceiling refusal, got %v", err)
	}
	if hit.Hit.Observed != 2 || hit.Hit.Threshold != 1 {
		t.Fatalf("hit %+v", hit.Hit)
	}
}

func TestBudgetCeilingFailsLoudOnEveryAxis(t *testing.T) {
	cases := map[string]struct {
		budget cognition.CognitiveBudget
		usage  cognition.Usage
		cost   int64
		want   cognition.CognitiveCeiling
	}{
		"prompt": {
			cognition.CognitiveBudget{MaxCalls: 10, MaxPromptTokens: 10, MaxCompletionTokens: 100, MaxCostMicros: 1000},
			cognition.Usage{PromptTokens: 11}, 0, cognition.CognitiveCeilingPromptTokens,
		},
		"completion": {
			cognition.CognitiveBudget{MaxCalls: 10, MaxPromptTokens: 100, MaxCompletionTokens: 10, MaxCostMicros: 1000},
			cognition.Usage{CompletionTokens: 11}, 0, cognition.CognitiveCeilingCompletionTokens,
		},
		"cost": {
			cognition.CognitiveBudget{MaxCalls: 10, MaxPromptTokens: 100, MaxCompletionTokens: 100, MaxCostMicros: 10},
			cognition.Usage{}, 11, cognition.CognitiveCeilingCost,
		},
		"calls": {
			cognition.CognitiveBudget{MaxCalls: 0, MaxPromptTokens: 100, MaxCompletionTokens: 100, MaxCostMicros: 1000},
			cognition.Usage{}, 0, "",
		},
	}
	for name, tc := range cases {
		if name == "calls" {
			if err := tc.budget.Validate(); !errors.Is(err, cognition.ErrInvalidBudget) {
				t.Fatalf("zero call ceiling must be invalid: %v", err)
			}
			continue
		}
		acct := newAccount(t, tc.budget)
		err := acct.Charge(24, tc.usage, tc.cost)
		var hit *cognition.CognitiveCeilingHitError
		if !errors.As(err, &hit) {
			t.Fatalf("%s: want typed halt, got %v", name, err)
		}
		if hit.Hit.Ceiling != tc.want {
			t.Fatalf("%s: ceiling %q, want %q", name, hit.Hit.Ceiling, tc.want)
		}
		if hit.Hit.Hour != 24 {
			t.Fatalf("%s: hour %d", name, hit.Hit.Hour)
		}
	}
}

func TestBudgetStaysWithinCeiling(t *testing.T) {
	acct := newAccount(t, cognition.CognitiveBudget{MaxCalls: 3, MaxPromptTokens: 30, MaxCompletionTokens: 30, MaxCostMicros: 300})
	for i := 0; i < 3; i++ {
		if err := acct.AllowCall(0); err != nil {
			t.Fatal(err)
		}
		if err := acct.Charge(0, cognition.Usage{PromptTokens: 10, CompletionTokens: 10}, 100); err != nil {
			t.Fatalf("charge %d: %v", i, err)
		}
	}
	if got := acct.Totals(); got.Calls != 3 || got.PromptTokens != 30 || got.CostMicros != 300 {
		t.Fatalf("totals %+v", got)
	}
	if _, hit := acct.Hit(); hit {
		t.Fatal("unexpected hit")
	}
}

func TestBudgetHitIsIdempotentFailLoud(t *testing.T) {
	acct := newAccount(t, cognition.CognitiveBudget{MaxCalls: 1, MaxPromptTokens: 100, MaxCompletionTokens: 100, MaxCostMicros: 1000})
	if err := acct.AllowCall(0); err != nil {
		t.Fatal(err)
	}
	if err := acct.Charge(0, cognition.Usage{PromptTokens: 1}, 1); err != nil {
		t.Fatal(err)
	}
	first := acct.AllowCall(0)
	if first == nil {
		t.Fatal("expected a halt")
	}
	second := acct.Charge(0, cognition.Usage{PromptTokens: 1}, 1)
	if second == nil {
		t.Fatal("a halted account must keep failing loud")
	}
	if got := acct.Totals().Calls; got != 1 {
		t.Fatalf("a halted charge must not advance the totals: %d", got)
	}
}

func TestBudgetRowsCarryTrailingRateAndProjection(t *testing.T) {
	acct := newAccount(t, cognition.CognitiveBudget{MaxCalls: 10, MaxPromptTokens: 1000, MaxCompletionTokens: 1000, MaxCostMicros: 100000})
	for i := 0; i < 4; i++ {
		if err := acct.Charge(6, cognition.Usage{PromptTokens: 10, CompletionTokens: 4}, 18); err != nil {
			t.Fatal(err)
		}
	}
	rows := acct.ObserveHour(6)
	if len(rows) != 4 {
		t.Fatalf("want four rows, got %d", len(rows))
	}
	if rows[0].Ceiling != cognition.CognitiveCeilingCalls || rows[0].Observed != 4 {
		t.Fatalf("calls row %+v", rows[0])
	}
	// headroom 6 at rate 4/h from hour 6 -> hour 8.
	if rows[0].ProjectedBindingHour != 8 {
		t.Fatalf("projected binding hour %d", rows[0].ProjectedBindingHour)
	}
	// A second observation with no further calls has a zero trailing rate.
	rows = acct.ObserveHour(7)
	if rows[0].RateLastHour != 0 || rows[0].ProjectedBindingHour != -1 {
		t.Fatalf("flat hour row %+v", rows[0])
	}
}

func TestPriceTableRefusesOverflow(t *testing.T) {
	p := cognition.PriceTable{Version: "test/overflow", PromptMicrosPerToken: 1 << 40, CompletionMicrosPerToken: 0}
	if _, err := p.CostMicros(cognition.Usage{PromptTokens: 1 << 40}); !errors.Is(err, cognition.ErrPriceOverflow) {
		t.Fatalf("overflow not refused: %v", err)
	}
	if err := (cognition.PriceTable{}).Validate(); !errors.Is(err, cognition.ErrInvalidPrice) {
		t.Fatalf("empty price accepted: %v", err)
	}
}
