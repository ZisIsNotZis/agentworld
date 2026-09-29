package world

import (
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

func TestCapacityWildProductionGatherConsumeBasal(t *testing.T) {
	// Production is v1-identical: ascending slot fill, unrealized overflow.
	p := CapacityPatchState{Yield: 8}
	var slots [CapacitySlotsPerPatch]CapacitySlotState
	p, slots, result, err := CapacityProduceWild(0, p, slots)
	if err != nil || result.Produced != 8 || result.Unrealized != 0 || p.Pulses != 1 || p.Produced != 8 {
		t.Fatalf("q8 first pulse: %+v %+v %v", p, result, err)
	}
	p, slots, result, err = CapacityProduceWild(1, p, slots)
	if err != nil || result.Produced != 0 || result.Unrealized != 8 || p.Produced != 8 || p.Unrealized != 8 {
		t.Fatalf("saturated inflow counted unrealized: %+v %+v %v", p, result, err)
	}
	for _, attempt := range []struct {
		patch CapacityPatchState
		h     int
	}{
		{CapacityPatchState{Yield: -1}, 0},
		{CapacityPatchState{Yield: 9}, 0},
		{CapacityPatchState{Yield: 8}, 1}, // pulses mismatch
		{CapacityPatchState{Yield: 8}, 168},
		{CapacityPatchState{Yield: 8, Produced: 1 << 62}, 0},
		{CapacityPatchState{Yield: 2, Pulses: 1, Produced: 2}, 1},
	} {
		if _, _, _, err := CapacityProduceWild(attempt.h, attempt.patch, [CapacitySlotsPerPatch]CapacitySlotState{}); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("invalid production accepted: %+v", attempt)
		}
	}
	// 168 pulses of q3 with full gathering supply exactly 3 units per pulse.
	p = CapacityPatchState{Yield: 3}
	slots = [CapacitySlotsPerPatch]CapacitySlotState{}
	for h := 0; h < CapacityHorizonHours; h++ {
		p, slots, _, err = CapacityProduceWild(h, p, slots)
		if err != nil {
			t.Fatal(err)
		}
		var taken int64
		for i := range slots {
			if slots[i].Stock == 1 && taken < 3 {
				slots[i].Gathered += 1
				slots[i].Stock = 0
				taken++
			}
		}
	}
	if p.Pulses != 168 || p.Produced != 168*3 || p.Unrealized != 0 {
		t.Fatalf("q3 supply: %+v", p)
	}
	// Aggregate totals alone never legalize a slot supplying more units than
	// its production pulses.
	over := [CapacitySlotsPerPatch]CapacitySlotState{{Gathered: 1 << 62}}
	if _, _, _, err := CapacityProduceWild(0, CapacityPatchState{Yield: 1}, over); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("impossible slot ledger accepted")
	}

	// Gather is v1-identical, home patch bound.
	actor := capacityTestActor()
	slot := CapacitySlotState{Stock: 1}
	nextSlot, next, err := CapacityGather(1001, 2001, 1, 0, slot, actor)
	if err != nil || nextSlot != (CapacitySlotState{Gathered: 1}) || next.Bag != (CapacityBagState{Units: 1, Source: 1001}) || next.Body.LastGatherHour != 0 {
		t.Fatalf("gather: %+v %+v %v", nextSlot, next, err)
	}
	for _, attempt := range []struct {
		patch, slot, actorID sim.EntityID
		hour                 int
		slotState            CapacitySlotState
		actorState           CapacityActorState
	}{
		{1001, 2001, 9, 0, CapacitySlotState{Stock: 1}, capacityTestActor()}, // foreign home patch
		{1001, 2009, 1, 0, CapacitySlotState{Stock: 1}, capacityTestActor()}, // slot outside the patch
		{1001, 2001, 1, 0, CapacitySlotState{}, capacityTestActor()},         // empty slot
		{1001, 2001, 1, 0, CapacitySlotState{Stock: 2}, capacityTestActor()}, // forged stock
		{1001, 2001, 1, 0, CapacitySlotState{Stock: 1, Gathered: 1 << 62}, capacityTestActor()},
		{1001, 2001, 17, 0, CapacitySlotState{Stock: 1}, capacityTestActor()}, // actor out of range
		{1001, 2001, 1, -1, CapacitySlotState{Stock: 1}, capacityTestActor()},
		{1001, 2001, 1, 168, CapacitySlotState{Stock: 1}, capacityTestActor()},
		{1001, 2001, 1, 0, CapacitySlotState{Stock: 1}, func() CapacityActorState {
			a := capacityTestActor()
			a.Bag = CapacityBagState{Units: 1, Source: 1001}
			return a
		}()},
		{1001, 2001, 1, 0, CapacitySlotState{Stock: 1}, func() CapacityActorState { a := capacityTestActor(); a.Body.LastGatherHour = 0; return a }()},
		{1001, 2001, 1, 0, CapacitySlotState{Stock: 1}, func() CapacityActorState {
			a := capacityTestActor()
			a.Body.Energy = 0
			a.Body.BasalSpent = 11
			return a
		}()},
		{1002, 2001, 1, 0, CapacitySlotState{Stock: 1}, capacityTestActor()},
	} {
		before := attempt.actorState
		if _, _, err := CapacityGather(attempt.patch, attempt.slot, attempt.actorID, attempt.hour, attempt.slotState, attempt.actorState); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("invalid gather accepted: %+v", attempt)
		}
		if !reflect.DeepEqual(before, attempt.actorState) {
			t.Fatal("rejected gather mutated its input")
		}
	}

	// Consume keeps v1 meal semantics with cap-12 spill counting.
	fed := next
	fed.Body.Energy, fed.Body.Hunger = 11, 5
	fed, meal, err := CapacityConsume(1, fed)
	if err != nil || meal != (CapacityMeal{EnergyGained: 1}) || fed.Body.Energy != 12 || fed.Body.Hunger != 4 || fed.Body.Consumed != 1 || fed.Bag.Units != 0 {
		t.Fatalf("meal: %+v %+v %v", fed, meal, err)
	}
	fed.Bag = CapacityBagState{Units: 1, Source: 1001} // next hour's gather
	fed, meal, err = CapacityConsume(1, fed)
	if err != nil || meal != (CapacityMeal{EnergyCapLost: 1}) || fed.Body.Energy != 12 || fed.Body.CapLost != 1 || fed.Body.Consumed != 2 {
		t.Fatalf("cap spill: %+v %+v %v", fed, meal, err)
	}
	full := fed
	full.Body.Consumed = CapacityHorizonHours
	full.Body.CapLost = 11
	full.Body.BasalSpent = CapacityHorizonHours
	if _, _, err := CapacityConsume(1, full); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("168th wild meal accepted")
	}
	if _, _, err := CapacityConsume(1, capacityTestActor()); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("empty bag consumed")
	}
	foreign := capacityTestActor()
	foreign.Bag = CapacityBagState{Units: 1, Source: 1002}
	if _, _, err := CapacityConsume(1, foreign); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("cross-patch bag consumed")
	}

	// Basal is v1-identical, including death and post-death accounting.
	a := capacityTestActor()
	for h := int64(1); h <= 11; h++ {
		next, cost, err := CapacityBasal(1, a, 1)
		if err != nil || cost != (CapacityBasalCost{EnergySpent: 1}) || next.Body.Energy != 11-h || next.Body.Hunger != h || next.Body.BasalSpent != h {
			t.Fatalf("basal h%d: %+v %+v %v", h, next, cost, err)
		}
		a = next
	}
	dead, cost, err := CapacityBasal(1, a, 13)
	if err != nil || dead.Body.Energy != 0 || dead.Body.Hunger != 24 || cost.UnmetEnergy != 13 {
		t.Fatalf("dead basal: %+v %+v %v", dead, cost, err)
	}
	for _, attempt := range []struct {
		actor CapacityActorState
		hours int64
	}{
		{capacityTestActor(), 0},
		{capacityTestActor(), 1 << 62},
		{capacityTestActor(), 169},
		{func() CapacityActorState { a := capacityTestActor(); a.Body.BasalSpent = 168; return a }(), 1},
		{func() CapacityActorState { a := capacityTestActor(); a.Body.LastGatherHour = 168; return a }(), 1},
		{func() CapacityActorState {
			a := capacityTestActor()
			a.Worksite.WearDebt = CapacityWearDebtMax + 1
			return a
		}(), 1},
		{func() CapacityActorState { a := capacityTestActor(); a.Granary.Stock = 1; return a }(), 1}, // granary unit without yield ledger
	} {
		if _, _, err := CapacityBasal(1, attempt.actor, attempt.hours); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("invalid basal accepted: %+v", attempt)
		}
	}
}

func TestCapacityBuildRatchetEligibilityAndLedgers(t *testing.T) {
	// The frozen eligibility k+wip < 8 on the single bag decision point
	// saturates the worksite at seven standing points plus one work-in-progress
	// unit: the completion build from k=7,wip=1 is itself beyond k+wip<8, so
	// capital 8 is a declared endowment bound, not a build-reachable state.
	a := capacityTestActor()
	hour := 0
	var completed int64
	for a.Worksite.Capital+a.Worksite.Wip < CapacityKMax {
		a.Bag = CapacityBagState{Units: 1, Source: 1001}
		a.Body.LastGatherHour = int64(hour)
		want := int64(0)
		if (a.Worksite.InvestedUnits+1)%CapacityPointCostWip == 0 {
			want = 1
		}
		next, result, err := CapacityBuild(hour, 1, a)
		if err != nil || result.PointsCompleted != want {
			t.Fatalf("build h%d: %+v %v", hour, result, err)
		}
		if next.Body != a.Body {
			t.Fatal("build changed physiology")
		}
		if next.Worksite.InvestedUnits != CapacityPointCostWip*next.Worksite.PointsCreated+next.Worksite.Wip ||
			next.Worksite.PointsCreated-next.Worksite.PointsDecayed != next.Worksite.Capital {
			t.Fatalf("G2 ledger drift: %+v", next.Worksite)
		}
		completed += result.PointsCompleted
		a = next
		hour++
	}
	if a.Worksite.Capital != CapacityKMax-1 || a.Worksite.Wip != 1 || a.Worksite.InvestedUnits != 2*CapacityKMax-1 ||
		a.Worksite.PointsCreated != CapacityKMax-1 || a.Worksite.PointsDecayed != 0 || completed != CapacityKMax-1 ||
		a.Worksite.LastBuildHour != 14 {
		t.Fatalf("ratchet: %+v", a.Worksite)
	}
	// Eligibility is exactly k+wip < 8 on the single bag decision point.
	full := a
	full.Bag = CapacityBagState{Units: 1, Source: 1001}
	full.Body.LastGatherHour = 15
	if _, _, err := CapacityBuild(15, 1, full); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("build beyond k+wip accepted")
	}
	seven := capacityTestActor()
	seven.Bag = CapacityBagState{Units: 1, Source: 1001}
	seven.Worksite = CapacityWorksiteState{Capital: CapacityKMax - 1, Wip: 1, PointsCreated: CapacityKMax, PointsDecayed: 1, InvestedUnits: 2*CapacityKMax + 1, LastBuildHour: CapacityNeverHour}
	if _, _, err := CapacityBuild(16, 1, seven); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("k=7,wip=1 build accepted")
	}
	for _, attempt := range []struct {
		name  string
		actor CapacityActorState
		hour  int
	}{
		{"empty-bag", capacityTestActor(), 0},
		{"dead", func() CapacityActorState {
			a := capacityTestActor()
			a.Body.Energy = 0
			a.Body.BasalSpent = 11
			a.Bag = CapacityBagState{Units: 1, Source: 1001}
			return a
		}(), 0},
		{"duplicate-hour", func() CapacityActorState {
			a := capacityTestActor()
			a.Bag = CapacityBagState{Units: 1, Source: 1001}
			a.Worksite.LastBuildHour = 3
			return a
		}(), 3},
		{"forged-invested", func() CapacityActorState {
			a := capacityTestActor()
			a.Bag = CapacityBagState{Units: 1, Source: 1001}
			a.Worksite.InvestedUnits = 5
			return a
		}(), 0},
	} {
		if _, _, err := CapacityBuild(attempt.hour, 1, attempt.actor); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("%s: %v", attempt.name, err)
		}
		if _, _, err := CapacityBuild(attempt.hour, 17, attempt.actor); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("%s: foreign actor accepted", attempt.name)
		}
	}
	for _, hour := range []int{-1, 168} {
		bagged := capacityTestActor()
		bagged.Bag = CapacityBagState{Units: 1, Source: 1001}
		if _, _, err := CapacityBuild(hour, 1, bagged); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("hour %d build accepted", hour)
		}
	}
}

func TestCapacityYieldWearAndExactPointLifetime(t *testing.T) {
	// One standing point survives exactly six pulses: decay lands on the
	// sixth wear, with debt returning to zero.
	one := capacityTestActor()
	one.Worksite = CapacityWorksiteState{Capital: 1, PointsCreated: 1, InvestedUnits: 2, LastBuildHour: 0}
	for pulse := int64(1); pulse <= 5; pulse++ {
		yielded, yieldResult, err := CapacityCapitalYield(1, one)
		if err != nil || yieldResult != (CapacityYieldResult{Realized: 1}) || yielded.Granary.Stock != pulse {
			t.Fatalf("yield pulse %d: %+v %v", pulse, yieldResult, err)
		}
		var wearResult CapacityWearResult
		one, wearResult, err = CapacityWear(1, yielded)
		if err != nil || wearResult.PointsDecayed != 0 || one.Worksite.WearDebt != pulse || one.Worksite.Capital != 1 {
			t.Fatalf("wear pulse %d: %+v %+v %v", pulse, one.Worksite, wearResult, err)
		}
	}
	yielded, yieldResult, err := CapacityCapitalYield(1, one)
	if err != nil || yieldResult.Realized != 1 || yielded.Granary.Stock != 6 || yielded.Granary.YieldTotal != 6 {
		t.Fatalf("final yield: %+v %+v %v", yielded.Granary, yieldResult, err)
	}
	worn, wearResult, err := CapacityWear(1, yielded)
	if err != nil || wearResult.PointsDecayed != 1 || worn.Worksite.Capital != 0 || worn.Worksite.WearDebt != 0 || worn.Worksite.PointsDecayed != 1 {
		t.Fatalf("sixth pulse must decay the exact 6-hour point: %+v %+v %v", worn.Worksite, wearResult, err)
	}
	one = worn
	if one.Granary.YieldTotal != one.Granary.StoredMeals+one.Granary.Stock+one.Granary.YieldUnrealized {
		t.Fatal("granary ledger drift")
	}
	// Storage binding: min(k, free space), overflow typed, full yield counted.
	full := capacityTestActor()
	full.Worksite = CapacityWorksiteState{Capital: 3, PointsCreated: 3, InvestedUnits: 6, LastBuildHour: 0}
	full.Granary = CapacityGranaryState{Stock: CapacityGranaryMax, YieldTotal: CapacityGranaryMax, LastStoredMealHour: CapacityNeverHour}
	full, yieldResult, err = CapacityCapitalYield(1, full)
	if err != nil || yieldResult != (CapacityYieldResult{Unrealized: 3}) || full.Granary.Stock != CapacityGranaryMax ||
		full.Granary.YieldTotal != CapacityGranaryMax+3 || full.Granary.YieldUnrealized != 3 {
		t.Fatalf("overflow: %+v %+v %v", full.Granary, yieldResult, err)
	}
	partial := capacityTestActor()
	partial.Worksite = CapacityWorksiteState{Capital: 3, PointsCreated: 3, InvestedUnits: 6, LastBuildHour: 0}
	partial.Granary = CapacityGranaryState{Stock: 7, YieldTotal: 7, LastStoredMealHour: CapacityNeverHour}
	partial, yieldResult, err = CapacityCapitalYield(1, partial)
	if err != nil || yieldResult != (CapacityYieldResult{Realized: 1, Unrealized: 2}) || partial.Granary.Stock != 8 || partial.Granary.YieldTotal != 10 {
		t.Fatalf("partial space: %+v %+v %v", partial.Granary, yieldResult, err)
	}
	// Dead owners yield nothing but still depreciate.
	dead := capacityTestActor()
	dead.Body.Energy, dead.Body.BasalSpent = 0, 11
	dead.Worksite = CapacityWorksiteState{Capital: 1, PointsCreated: 1, InvestedUnits: 2, LastBuildHour: 0}
	if _, _, err := CapacityCapitalYield(1, dead); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("dead owner yielded")
	}
	for pulse := 0; pulse < 6; pulse++ {
		dead, wearResult, err = CapacityWear(1, dead)
		if err != nil {
			t.Fatal(err)
		}
		_ = wearResult
	}
	if dead.Worksite.Capital != 0 || dead.Worksite.PointsDecayed != 1 || dead.Worksite.WearDebt != 0 {
		t.Fatalf("wear must apply to dead owners: %+v", dead.Worksite)
	}
	// Transient debt peaks at the frozen 13: debt 5 + k 8 decays two points.
	peak := capacityTestActor()
	peak.Worksite = CapacityWorksiteState{Capital: 8, PointsCreated: 8, InvestedUnits: 16, LastBuildHour: 0, WearDebt: 5}
	peak, wearResult, err = CapacityWear(1, peak)
	if err != nil || wearResult.PointsDecayed != 2 || peak.Worksite.Capital != 6 || peak.Worksite.WearDebt != 1 {
		t.Fatalf("peak debt: %+v %+v %v", peak.Worksite, wearResult, err)
	}
	// Forged and bounds-invalid worksites are rejected with zero mutation.
	forged := capacityTestActor()
	forged.Worksite.WearDebt = CapacityWearDebtMax + 1
	if _, _, err := CapacityWear(1, forged); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("forged wear-debt > 13 accepted")
	}
	unbacked := capacityTestActor()
	unbacked.Worksite.WearDebt = CapacityWearPeriod // debt with zero capital cannot decay anything
	if _, _, err := CapacityWear(1, unbacked); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("debt without capital accepted")
	}
}

func TestCapacityEatStoredSemantics(t *testing.T) {
	a := capacityTestActor()
	a.Granary = CapacityGranaryState{Stock: 2, YieldTotal: 2, LastStoredMealHour: CapacityNeverHour}
	a.Body.Hunger = 5
	a, meal, err := CapacityEatStored(4, 1, a)
	if err != nil || meal != (CapacityMeal{EnergyGained: 1}) || a.Granary.Stock != 1 || a.Granary.StoredMeals != 1 ||
		a.Granary.LastStoredMealHour != 4 || a.Body.Energy != 12 || a.Body.Hunger != 4 {
		t.Fatalf("stored meal: %+v %+v %v", a, meal, err)
	}
	a, meal, err = CapacityEatStored(5, 1, a)
	if err != nil || meal != (CapacityMeal{EnergyCapLost: 1}) || a.Body.Energy != 12 || a.Body.CapLost != 1 || a.Granary.Stock != 0 || a.Granary.StoredMeals != 2 {
		t.Fatalf("stored cap spill: %+v %+v %v", a, meal, err)
	}
	if a.Granary.YieldTotal != a.Granary.StoredMeals+a.Granary.Stock+a.Granary.YieldUnrealized {
		t.Fatal("granary identity broken by stored meals")
	}
	for _, attempt := range []struct {
		actor CapacityActorState
		hour  int
	}{
		{capacityTestActor(), 0}, // empty granary
		{func() CapacityActorState {
			a := capacityTestActor()
			a.Body.Energy = 0
			a.Body.BasalSpent = 11
			a.Granary = CapacityGranaryState{Stock: 1, YieldTotal: 1}
			return a
		}(), 0},
		{func() CapacityActorState {
			a := capacityTestActor()
			a.Granary = CapacityGranaryState{Stock: 1, YieldTotal: CapacityHorizonHours + 1, StoredMeals: CapacityHorizonHours, YieldUnrealized: 1}
			return a
		}(), 0},
		{a, -1},
		{a, 168},
	} {
		if _, _, err := CapacityEatStored(attempt.hour, 1, attempt.actor); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("invalid stored meal accepted: %+v", attempt)
		}
	}
}

// TestCapacitySelfSufficiencyIntegerArithmetic pins the frozen derivation A
// threshold with exact integers: a standing capital k yields
// CapacityYieldPerPoint per pulse for six pulses, while eating costs one unit
// per pulse and maintenance costs two units per point per six pulses. Self-
// sufficiency k ≥ 1 + 2k/6 therefore first holds at k* = 2. The per-k hourly
// net rates quoted in prose (spec: −0.75/+0.5; task brief: −0.25/+1/3) are not
// used here; only the integer threshold and mechanism costs are binding.
func TestCapacitySelfSufficiencyIntegerArithmetic(t *testing.T) {
	var threshold int64 = -1
	for k := int64(0); k <= CapacityKMax; k++ {
		income := CapacityWearPeriod * k * CapacityYieldPerPoint
		cost := CapacityWearPeriod + CapacityPointCostWip*k
		selfSufficient := income >= cost
		if selfSufficient && threshold < 0 {
			threshold = k
		}
		if selfSufficient != (k >= 2) {
			t.Fatalf("k=%d: income %d cost %d self-sufficient=%v", k, income, cost, selfSufficient)
		}
	}
	if threshold != 2 {
		t.Fatalf("k* = %d", threshold)
	}
	if deficit := (CapacityWearPeriod + CapacityPointCostWip) - CapacityWearPeriod*CapacityYieldPerPoint; deficit != 2 {
		t.Fatalf("k=1 six-hour deficit %d units", deficit)
	}
	if surplus := CapacityWearPeriod*2*CapacityYieldPerPoint - (CapacityWearPeriod + CapacityPointCostWip*2); surplus != 2 {
		t.Fatalf("k=2 six-hour surplus %d units", surplus)
	}
	// Mechanism-level: exactly two invested units rebuild one decayed point,
	// and the rebuilt state satisfies both G2 ledgers.
	a := capacityTestActor()
	a.Worksite = CapacityWorksiteState{Capital: 1, PointsCreated: 1, InvestedUnits: 2, LastBuildHour: 0}
	for pulse := 0; pulse < int(CapacityWearPeriod); pulse++ {
		next, _, err := CapacityWear(1, a)
		if err != nil {
			t.Fatal(err)
		}
		a = next
	}
	if a.Worksite.Capital != 0 || a.Worksite.WearDebt != 0 || a.Worksite.PointsDecayed != 1 {
		t.Fatalf("decay: %+v", a.Worksite)
	}
	for build := 0; build < int(CapacityPointCostWip); build++ {
		a.Bag = CapacityBagState{Units: 1, Source: 1001}
		a.Body.LastGatherHour = int64(build + 1)
		next, _, err := CapacityBuild(build+1, 1, a)
		if err != nil {
			t.Fatal(err)
		}
		a = next
	}
	if a.Worksite.Capital != 1 || a.Worksite.PointsCreated != 2 || a.Worksite.InvestedUnits != 4 ||
		a.Worksite.InvestedUnits != CapacityPointCostWip*a.Worksite.PointsCreated+a.Worksite.Wip ||
		a.Worksite.PointsCreated-a.Worksite.PointsDecayed != a.Worksite.Capital {
		t.Fatalf("rebuild: %+v", a.Worksite)
	}
}

// TestCapacityConservationGatesG1G2G3 drives a scripted three-hour world
// through the pure functions and then checks every aggregate gate, including
// the exact balance totals computed by hand.
func TestCapacityConservationGatesG1G2G3(t *testing.T) {
	var patches [CapacityPatchCount]CapacityPatchState
	var slots [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState
	var actors [CapacityActorCount]CapacityActorState
	for i := range actors {
		actors[i] = capacityTestActor()
	}
	for i := range patches {
		patches[i].Yield = 8
	}
	builder := func(id sim.EntityID) CapacityActorState { return actors[id-1] }
	commit := func(id sim.EntityID, next CapacityActorState) { actors[id-1] = next }
	for h := 0; h < 3; h++ {
		for id := sim.EntityID(1); id <= CapacityActorCount; id++ {
			next, _, err := CapacityBasal(id, builder(id), 1)
			if err != nil {
				t.Fatal(err)
			}
			commit(id, next)
		}
		for p := 0; p < CapacityPatchCount; p++ {
			var err error
			patches[p], slots[p], _, err = CapacityProduceWild(h, patches[p], slots[p])
			if err != nil {
				t.Fatal(err)
			}
		}
		for id := sim.EntityID(1); id <= CapacityActorCount; id++ {
			home, _ := CapacityActorPatchID(id)
			next, _, err := CapacityCapitalYield(id, builder(id))
			if err != nil {
				t.Fatal(err)
			}
			commit(id, next)
			next, _, err = CapacityWear(id, builder(id))
			if err != nil {
				t.Fatal(err)
			}
			commit(id, next)
			slotID, _ := CapacitySlotID(int(home)-1001, int(id-1)%CapacitySlotsPerPatch)
			slot := slots[int(home)-1001][int(id-1)%CapacitySlotsPerPatch]
			gathered, next, err := CapacityGather(home, slotID, id, h, slot, builder(id))
			if err != nil {
				t.Fatal(err)
			}
			slots[int(home)-1001][int(id-1)%CapacitySlotsPerPatch] = gathered
			commit(id, next)
			if int(id)%2 == 0 { // even actors eat their wild unit
				next, _, err = CapacityConsume(id, builder(id))
			} else { // odd actors invest it; Build never touches physiology
				next, _, err = CapacityBuild(h, id, builder(id))
			}
			if err != nil {
				t.Fatal(err)
			}
			commit(id, next)
		}
	}
	// Hand-computed exact totals: every actor gathered and resolved its bag
	// three times; odd actors stand at k=1 with wip=1, even actors ate 3 meals.
	balance, err := CapacityCheckConservation(patches, slots, actors)
	if err != nil {
		t.Fatalf("conservation: %v", err)
	}
	want := CapacityBalance{
		Produced: 48, Gathered: 48, Consumed: 24, Invested: 24,
		Stock: 0, Held: 0,
		Capital: 8, Wip: 8, PointsCreated: 8, PointsDecayed: 0,
		YieldTotal: 8, GranaryStock: 8, StoredMeals: 0, YieldUnrealized: 0,
		InitialEnergy: 176, Energy: 152, BasalSpent: 48, EnergyCapLost: 0,
	}
	if balance != want {
		t.Fatalf("balance: got %+v want %+v", balance, want)
	}
	if balance.InitialEnergy+balance.Consumed+balance.StoredMeals != balance.Energy+balance.BasalSpent+balance.EnergyCapLost {
		t.Fatal("G3 violated")
	}
	for _, tamper := range []struct {
		name   string
		mutate func(*[CapacityPatchCount]CapacityPatchState, *[CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState, *[CapacityActorCount]CapacityActorState)
	}{
		{"minted-energy", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			a[0].Body.Energy++
		}},
		{"minted-capital", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			a[0].Worksite.Capital++
		}},
		{"minted-granary", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			a[0].Granary.Stock++
		}},
		{"lost-gather", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			s[0][0].Gathered--
		}},
		{"minted-production", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) { p[0].Produced++ }},
		{"shifted-slot-history", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			s[0][0].Gathered += 2 // aggregate total restored elsewhere below
			s[0][1].Gathered -= 2 // one slot now claims more units than pulses allow
		}},
		{"forged-invested", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			a[1].Worksite.InvestedUnits--
		}},
		{"forged-stored-meal", func(p *[2]CapacityPatchState, s *[2][8]CapacitySlotState, a *[16]CapacityActorState) {
			a[1].Granary.StoredMeals++
			a[1].Body.Energy++
		}},
	} {
		t.Run(tamper.name, func(t *testing.T) {
			p, s, a := patches, slots, actors
			tamper.mutate(&p, &s, &a)
			if _, err := CapacityCheckConservation(p, s, a); !errors.Is(err, ErrCapacityContract) {
				t.Fatal("tampered world passed conservation")
			}
		})
	}
}
