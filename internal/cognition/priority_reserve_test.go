package cognition_test

import (
	"testing"

	"agentworld/internal/cognition"
)

// TestDecisionBudgetReservesTransportAttempts pins the derivation the quota
// schedule now spends: the decision budget is the declared call ceiling N less
// the declared transport reserve R, and a ceiling at or below R floors at 0
// instead of underflowing.
func TestDecisionBudgetReservesTransportAttempts(t *testing.T) {
	cases := []struct{ ceiling, want uint64 }{
		{256, 248},
		{168, 160},
		{24, 16},
		{cognition.DeclaredTransportReserveAttempts, 0},
		{4, 0},
		{1, 0},
	}
	for _, tc := range cases {
		if got := cognition.DecisionBudget(tc.ceiling); got != tc.want {
			t.Fatalf("DecisionBudget(%d) = %d, want %d", tc.ceiling, got, tc.want)
		}
		if tc.ceiling > cognition.DeclaredTransportReserveAttempts {
			if got := cognition.DecisionBudget(tc.ceiling) + cognition.DeclaredTransportReserveAttempts; got != tc.ceiling {
				t.Fatalf("ceiling %d is not preserved: decisions+reserve = %d", tc.ceiling, got)
			}
		}
	}
}
