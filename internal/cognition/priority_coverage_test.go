package cognition_test

import (
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// identicalRoster builds a static-observation actor set: every actor carries the
// SAME feature vector, so priority ranking alone has no reason to prefer any of
// them (the tie-break is the smallest actor id). This is the shape that collapsed
// coverage under vs2-value/1 (43-49 calls on five actors, 1-3 on three others at
// f10-s0), so it is the regression fixture for the vs2-value/2 coverage floor.
func identicalRoster(hour, count int) []cognition.EscalationCandidate {
	out := make([]cognition.EscalationCandidate, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, cognition.EscalationCandidate{
			Actor: sim.EntityID(10 * (i + 1)),
			Hour:  hour,
			Features: cognition.ValueFeatures{
				Urgency:             3,
				DecisionSensitivity: 2,
				EscalationRequested: 0,
			},
		})
	}
	return out
}

// coveragePolicy builds an adaptive policy over one value model, one cell's worth
// of hours and a per-hour quota of one call.
func coveragePolicy(model cognition.ValueModelSpec, ceiling uint64) cognition.WakePolicy {
	p := cognition.WakePolicy{Name: "vs2-coverage", PeriodHours: 1, PhaseHours: 0}
	p.Priority = &cognition.PriorityPolicy{
		Strategy: protocol.StrategyAdaptive, ExplorationFraction: 0,
		RandSeed: 1, ValueModel: model, CallCeiling: ceiling,
	}
	return p
}

// selectStream runs the selector over `hours` hours of an identical roster and
// returns the per-actor realised call counts.
func selectStream(t *testing.T, policy cognition.WakePolicy, hours, actors int) map[sim.EntityID]int64 {
	t.Helper()
	s, err := cognition.NewPrioritySelector(policy, hours, protocol.CallPrice{})
	if err != nil {
		t.Fatal(err)
	}
	for hour := 0; hour < hours; hour++ {
		if _, err := s.Select(hour, identicalRoster(hour, actors), protocol.BudgetState{CallsBudget: policy.Priority.CallCeiling}); err != nil {
			t.Fatal(err)
		}
	}
	return s.CallsPerActor()
}

// TestValueModelV2KeepsCoverageNonDegenerate is the regression test for the
// coverage collapse: with a static-observation actor set and a persistent (but
// identical) urgency signal, vs2-value/1 spends every slot on the same few
// actors, while vs2-value/2 spreads the same budget over every actor.
func TestValueModelV2KeepsCoverageNonDegenerate(t *testing.T) {
	const actors, hours = 6, 60
	ceiling := uint64(hours) // one call per hour

	v1 := testValueModel()
	countsV1 := selectStream(t, coveragePolicy(v1, ceiling), hours, actors)

	v2 := testValueModel()
	v2.Version = "vs2-value/2"
	v2.StalenessRefreshHours = protocol.DefaultStalenessRefreshHours
	v2.StalenessFloorBasisPoints = protocol.DefaultStalenessFloorBasisPoints
	v2.CoverageFloor = true
	countsV2 := selectStream(t, coveragePolicy(v2, ceiling), hours, actors)

	coveredV1, minV1, maxV1 := covered(countsV1, actors)
	coveredV2, minV2, maxV2 := covered(countsV2, actors)
	if coveredV2 != actors {
		t.Fatalf("vs2-value/2 covered %d/%d actors (min %d max %d), want every actor", coveredV2, actors, minV2, maxV2)
	}
	if minV2 == 0 || maxV2-minV2 > 1 {
		t.Fatalf("vs2-value/2 coverage min %d max %d over %d calls, want a flat spread", minV2, maxV2, ceiling)
	}
	if coveredV1 == actors && maxV1-minV1 <= 1 {
		t.Fatalf("vs2-value/1 fixture no longer collapses (covered %d, min %d max %d); the regression fixture is wrong", coveredV1, minV1, maxV1)
	}
	if coveredV2 <= coveredV1 {
		t.Fatalf("vs2-value/2 coverage (%d) is not wider than vs2-value/1 (%d)", coveredV2, coveredV1)
	}
}

// covered returns how many actors received at least one call, plus the min and
// max per-actor counts.
func covered(counts map[sim.EntityID]int64, actors int) (int, int64, int64) {
	covered, min, max := 0, int64(-1), int64(0)
	for i := 0; i < actors; i++ {
		calls := counts[sim.EntityID(10*(i+1))]
		if calls > 0 {
			covered++
		}
		if min < 0 || calls < min {
			min = calls
		}
		if calls > max {
			max = calls
		}
	}
	if min < 0 {
		min = 0
	}
	return covered, min, max
}
