package world

import (
	"agentworld/internal/sim"
	"errors"
	"testing"
)

// TestReproductionNeutralLoopsMatchCapacityV3 proves the inherited v3
// arithmetic is byte-identical at the neutral locus 6: the v4 basal, yield,
// and wear ledgers track the v3 functions step for step on parallel
// schedules.
func TestReproductionNeutralLoopsMatchCapacityV3(t *testing.T) {
	// Basal: identical energy/basal-spent/hunger until the v4 actor
	// starves; starvation is the new explicit died-hour bookkeeping.
	v3 := CapacityActorState{Body: CapacityBodyState{Energy: CapacityInitialEnergy}}
	v4, err := ReproductionFounderState(1, 120)
	if err != nil {
		t.Fatal(err)
	}
	for hour := 0; hour <= 10; hour++ {
		v3Next, v3Cost, err3 := CapacityBasal(1, v3, 1)
		if err3 != nil {
			t.Fatal(err3)
		}
		v4Next, v4Cost, starved, err4 := ReproductionBasal(hour, 1, v4, 1)
		if err4 != nil {
			t.Fatal(err4)
		}
		if v3Cost.EnergySpent != v4Cost.EnergySpent || v3Cost.UnmetEnergy != v4Cost.UnmetEnergy {
			t.Fatalf("hour %d basal cost: v3 %+v v4 %+v", hour, v3Cost, v4Cost)
		}
		if v3Next.Body.Energy != v4Next.Body.Energy || v3Next.Body.BasalSpent != v4Next.Body.BasalSpent || v3Next.Body.Hunger != v4Next.Body.Hunger {
			t.Fatalf("hour %d basal state: v3 %+v v4 %+v", hour, v3Next.Body, v4Next.Body)
		}
		if starved != (hour == 10) {
			t.Fatalf("hour %d starved %v", hour, starved)
		}
		v3, v4 = v3Next, v4Next
	}
	if v4.Body.Energy != 0 || v4.Body.BasalSpent != 11 || v4.Body.BasalDebt != 0 || v4.Genome.DiedHour != 10 {
		t.Fatalf("starvation bookkeeping: %+v", v4)
	}
	// Dead actors are stopped: no further basal call is valid.
	if _, _, _, err := ReproductionBasal(11, 1, v4, 1); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("basal on a dead actor admitted")
	}
	// Yield: identical stock/total/unrealized at capital 3, neutral loci.
	v3 = CapacityActorState{Body: CapacityBodyState{Energy: CapacityInitialEnergy},
		Worksite: CapacityWorksiteState{Capital: 3, PointsCreated: 3, InvestedUnits: 6},
		Granary:  CapacityGranaryState{}}
	v4, err = reproductionCapitalFixture(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	for pulse := 0; pulse < 30; pulse++ {
		v3Next, v3Result, err3 := CapacityCapitalYield(1, v3)
		if err3 != nil {
			t.Fatal(err3)
		}
		v4Next, v4Result, err4 := ReproductionCapitalYield(1, v4)
		if err4 != nil {
			t.Fatal(err4)
		}
		if v3Result.Realized != v4Result.Realized || v3Result.Unrealized != v4Result.Unrealized ||
			v3Next.Granary.Stock != v4Next.Granary.Stock || v3Next.Granary.YieldTotal != v4Next.Granary.YieldTotal ||
			v3Next.Granary.YieldUnrealized != v4Next.Granary.YieldUnrealized {
			t.Fatalf("pulse %d yield: v3 %+v %+v v4 %+v %+v", pulse, v3Next.Granary, v3Result, v4Next.Granary, v4Result)
		}
		v3, v4 = v3Next, v4Next
	}
	if v4.Granary.Stock != 8 || v4.Granary.YieldUnrealized != 82 || v4.Granary.YieldTotal != 90 {
		t.Fatalf("30 neutral pulses of capital 3: %+v", v4.Granary)
	}
	// Wear: identical capital/debt/decay, dead owners included.
	v3Dead := v3
	v3Dead.Body = CapacityBodyState{BasalSpent: ReproductionFounderInitialEnergy}
	v4Dead := v4
	v4Dead.Body.Energy = 0
	v4Dead.Body.BasalSpent = ReproductionFounderInitialEnergy
	for pulse := 0; pulse < 30; pulse++ {
		v3Next, v3Result, err3 := CapacityWear(1, v3Dead)
		if err3 != nil {
			t.Fatal(err3)
		}
		v4Next, v4Result, err4 := ReproductionWear(1, v4Dead)
		if err4 != nil {
			t.Fatal(err4)
		}
		if v3Result.PointsDecayed != v4Result.PointsDecayed || v3Next.Worksite.Capital != v4Next.Worksite.Capital || v3Next.Worksite.WearDebt != v4Next.Worksite.WearDebt {
			t.Fatalf("pulse %d wear: v3 %+v v4 %+v", pulse, v3Next.Worksite, v4Next.Worksite)
		}
		v3Dead, v4Dead = v3Next, v4Next
	}
	// Capital 3 at the neutral wear period decays one point every two
	// pulses: all three points are gone after pulse 6.
	if v4Dead.Worksite.Capital != 0 || v4Dead.Worksite.PointsDecayed != 3 || v4Dead.Worksite.WearDebt != 0 {
		t.Fatalf("neutral wear decay: %+v", v4Dead.Worksite)
	}
}

// reproductionCapitalFixture builds a valid actor with standing capital and
// an empty granary (all loci neutral).
func reproductionCapitalFixture(actor sim.EntityID, capital int64) (ReproductionActorState, error) {
	a, err := ReproductionFounderState(actor, 140)
	if err != nil {
		return ReproductionActorState{}, err
	}
	a.Worksite = ReproductionWorksiteState{Capital: capital, PointsCreated: capital, InvestedUnits: ReproductionPointCostWip * capital, LastBuildHour: ReproductionNeverHour}
	if !validReproductionActor(actor, a) {
		return ReproductionActorState{}, ErrReproductionContract
	}
	return a, nil
}

// TestReproductionBasalDivisorLoopStarvationAndBounds walks the basal loop
// for every locus-m without food: the drain rate is 6/m per hour, the last
// energy unit writes died-hour (rule 404), and the carry never exceeds the
// frozen bound 13.
func TestReproductionBasalDivisorLoopStarvationAndBounds(t *testing.T) {
	for m := int64(ReproductionGenomeLocusMin); m <= ReproductionGenomeLocusMax; m++ {
		a, err := ReproductionFounderState(1, 140)
		if err != nil {
			t.Fatal(err)
		}
		a.Genome.LocusM = m
		if !validReproductionActor(1, a) {
			t.Fatal("fixture")
		}
		var totalSpent int64
		starvedHour := int64(-1)
		for hour := 0; hour <= int(2*ReproductionEnergyCapacity); hour++ {
			next, cost, starved, err := ReproductionBasal(hour, 1, a, 1)
			if err != nil {
				t.Fatalf("m=%d hour %d: %v", m, hour, err)
			}
			if cost.EnergySpent > 2 {
				t.Fatalf("m=%d drained %d in one hour", m, cost.EnergySpent)
			}
			if next.Body.BasalDebt < 0 || next.Body.BasalDebt > ReproductionBasalDebtMax {
				t.Fatalf("m=%d debt out of bounds: %d", m, next.Body.BasalDebt)
			}
			totalSpent += cost.EnergySpent
			a = next
			if starved {
				starvedHour = int64(hour)
				break
			}
		}
		// With no intake the actor drains all 11 founder units at rate
		// 6/m per hour; the death lands on the first hour whose elapsed
		// floor(6·(n+1)/m) reaches the stock.
		wantHour := int64(-1)
		for n := int64(1); n <= 64; n++ {
			if 6*n/m >= ReproductionFounderInitialEnergy {
				wantHour = n - 1
				break
			}
		}
		if starvedHour != wantHour || totalSpent != ReproductionFounderInitialEnergy {
			t.Fatalf("m=%d starved at %d (want %d) after %d units", m, starvedHour, wantHour, totalSpent)
		}
		if a.Genome.DiedHour != starvedHour {
			t.Fatalf("m=%d died-hour %d", m, a.Genome.DiedHour)
		}
	}
	// Elapsed-hour batching and the forged-state rejection.
	a, _ := ReproductionFounderState(1, 140)
	if _, _, _, err := ReproductionBasal(0, 1, a, 0); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("zero elapsed admitted")
	}
	if _, _, _, err := ReproductionBasal(169, 1, a, 1); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("hour beyond the pulse horizon admitted")
	}
}

// TestReproductionYieldWearLoopsAllAlleles walks the yield and wear loops
// exhaustively over capital and alleles, checking the closed-form grant and
// decay counts, the carry bounds, and the per-actor G2a/G2b identities.
func TestReproductionYieldWearLoopsAllAlleles(t *testing.T) {
	for k := int64(1); k <= ReproductionKMax; k++ {
		for y := int64(ReproductionGenomeLocusMin); y <= ReproductionGenomeLocusMax; y++ {
			a, err := reproductionCapitalFixture(1, k)
			if err != nil {
				t.Fatal(err)
			}
			a.Genome.LocusY = y
			for pulse := 0; pulse < 168; pulse++ {
				next, _, err := ReproductionCapitalYield(1, a)
				if err != nil {
					t.Fatalf("k=%d y=%d pulse %d: %v", k, y, pulse, err)
				}
				if next.Granary.YieldDebt < 0 || next.Granary.YieldDebt >= y {
					t.Fatalf("k=%d y=%d carry %d not drained below %d", k, y, next.Granary.YieldDebt, y)
				}
				if !validReproductionActor(1, next) {
					t.Fatalf("k=%d y=%d pulse %d: invalid state %+v", k, y, pulse, next)
				}
				// Closed form: cumulative grants after n pulses are
				// floor(6·k·n / y); every grant is stock or typed
				// unrealized.
				n := int64(pulse + 1)
				want := 6 * k * n / y
				if got := next.Granary.Stock + next.Granary.YieldUnrealized; got != want {
					t.Fatalf("k=%d y=%d n=%d grants %d want %d", k, y, n, got, want)
				}
				a = next
			}
		}
	}
	for w := int64(ReproductionGenomeLocusMin); w <= ReproductionGenomeLocusMax; w++ {
		for capital := int64(1); capital <= ReproductionKMax; capital++ {
			a, err := reproductionCapitalFixture(1, capital)
			if err != nil {
				t.Fatal(err)
			}
			a.Genome.LocusW = w
			var firstDecay int64 = -1
			for pulse := 0; pulse < int(w)*2; pulse++ {
				next, result, err := ReproductionWear(1, a)
				if err != nil {
					t.Fatalf("w=%d k=%d pulse %d: %v", w, capital, pulse, err)
				}
				if next.Worksite.WearDebt < 0 || next.Worksite.WearDebt >= w {
					t.Fatalf("w=%d k=%d carry %d not drained below %d", w, capital, next.Worksite.WearDebt, w)
				}
				if result.PointsDecayed > 0 && firstDecay == -1 {
					firstDecay = int64(pulse + 1)
				}
				if !validReproductionActor(1, next) {
					t.Fatalf("w=%d k=%d pulse %d: invalid state", w, capital, pulse)
				}
				a = next
			}
			// The leading point decays once k-accrual crosses w: at
			// pulse ceil(w/k) for capital k.
			if firstDecay != (w+capital-1)/capital {
				t.Fatalf("w=%d k=%d first decay at %d", w, capital, firstDecay)
			}
		}
	}
}

// TestReproductionChildGenomeSevenDrawSemantics pins the frozen heredity:
// crossover draw mod 2 picks proposer/consenter, mutation happens iff
// draw mod 16 == 0 with direction (draw div 16) mod 2 and clamping into
// [5,7], and the lifetime draw is 96 + draw mod 48.
func TestReproductionChildGenomeSevenDrawSemantics(t *testing.T) {
	proposer, consenter := mustGenome(t, 1), mustGenome(t, 2)
	// All-neutral parents, draws choosing consenter for locus-y and
	// mutating every locus downward (mutation draw 0), lifetime draw 0.
	child, cross, err := ReproductionChildGenome(1, 2, proposer, consenter, 7, []uint64{0, 1, 0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if child.LocusM != 5 || child.LocusY != 5 || child.LocusW != 5 || !cross.MutatedM || !cross.MutatedY || !cross.MutatedW ||
		cross.ClampedM || cross.ClampedY || cross.ClampedW || cross.Lifetime != 96 {
		t.Fatalf("neutral draw vector: %+v %+v", child, cross)
	}
	if child.ParentA != 1 || child.ParentB != 2 || child.BirthHour != 7 || child.DeathHour != 103 || child.DiedHour != -1 || child.BirthGranaryPaid != 0 {
		t.Fatalf("child genome row: %+v", child)
	}
	// Direction up (mutation draw 16 → (16/16)%2 = 1; 48 also up),
	// lifetime draw 47.
	child, cross, err = ReproductionChildGenome(1, 2, proposer, consenter, 0, []uint64{1, 0, 1, 16, 48, 16, 47})
	if err != nil {
		t.Fatal(err)
	}
	if child.LocusM != 7 || child.LocusY != 7 || child.LocusW != 7 || cross.Lifetime != 143 {
		t.Fatalf("up-mutation vector: %+v %+v", child, cross)
	}
	// Clamping at both ends with mixed parent alleles:
	// proposer (5,6,7), consenter (6,7,5); crossover [0,1,1] picks
	// m=5, y=7, w=5; mutation [0,16,0] clamps m down at 5, clamps y up
	// at 7, and clamps w down at 5.
	tallProposer, tallConsenter := proposer, consenter
	tallProposer.LocusM, tallProposer.LocusY, tallProposer.LocusW = 5, 6, 7
	tallConsenter.LocusM, tallConsenter.LocusY, tallConsenter.LocusW = 6, 7, 5
	child, cross, err = ReproductionChildGenome(1, 2, tallProposer, tallConsenter, 0, []uint64{0, 1, 1, 0, 16, 0, 20})
	if err != nil {
		t.Fatal(err)
	}
	if child.LocusM != 5 || child.LocusY != 7 || child.LocusW != 5 ||
		!cross.ClampedM || !cross.ClampedY || !cross.ClampedW || cross.Lifetime != 96+20 {
		t.Fatalf("clamp vector: %+v %+v", child, cross)
	}
	// Exactly seven draws.
	if _, _, err := ReproductionChildGenome(1, 2, proposer, consenter, 0, drawsOf(6)); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("six draws admitted")
	}
	if _, _, err := ReproductionChildGenome(1, 2, proposer, consenter, 0, drawsOf(8)); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("eight draws admitted")
	}
	if _, _, err := ReproductionChildGenome(1, 1, proposer, consenter, 0, drawsOf(7)); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("self-parenting admitted")
	}
	if _, _, err := ReproductionChildGenome(1, 2, proposer, consenter, 168, drawsOf(7)); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("birth beyond the horizon admitted")
	}
}

func drawsOf(n int) []uint64 {
	draws := make([]uint64, n)
	for i := range draws {
		draws[i] = 1 // never mutating: 1 mod 16 != 0
	}
	return draws
}

// TestReproductionKinshipWalkDistances pins the derived kinship: parent-child
// 1, siblings 2, grandparent-grandchild 2 prohibited, cousins 4 allowed,
// unrelated pairs allowed, and the ≤10-ref walk bound.
func TestReproductionKinshipWalkDistances(t *testing.T) {
	genomes := map[sim.EntityID]ReproductionGenomeState{
		1: mustGenome(t, 1), 2: mustGenome(t, 2), 3: mustGenome(t, 3), 4: mustGenome(t, 4),
		5: mustGenome(t, 5), 6: mustGenome(t, 6),
	}
	add := func(id, a, b sim.EntityID, birth int) {
		child, _, err := ReproductionChildGenome(a, b, genomes[a], genomes[b], birth, drawsOf(7))
		if err != nil {
			t.Fatal(err)
		}
		genomes[id] = child
	}
	add(10001, 1, 2, 10)     // child of founders 1,2
	add(10002, 3, 4, 10)     // child of founders 3,4
	add(10003, 10001, 2, 20) // grandchild of founder 1 (and child of 2: distance 1)
	add(10004, 1, 2, 15)     // sibling of 10001
	add(10005, 10001, 3, 30)
	add(10007, 5, 6, 10) // unrelated branch
	add(10006, 10004, 10007, 40)
	lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		g, ok := genomes[id]
		if !ok {
			return 0, 0, false
		}
		return g.ParentA, g.ParentB, true
	})
	for _, want := range []struct {
		a, b                sim.EntityID
		distance            int
		related, prohibited bool
	}{
		{1, 10001, 1, true, true},       // parent-child
		{10001, 10004, 2, true, true},   // siblings
		{1, 10003, 2, true, true},       // grandparent-grandchild
		{2, 10003, 1, true, true},       // parent-child
		{10005, 10006, 4, true, false},  // cousins through shared grandparents 1,2
		{10002, 10001, 0, false, false}, // unrelated branches
		{1, 2, 0, false, false},         // unrelated founders
		{1, 1, 0, true, true},           // self
	} {
		distance, related := ReproductionKinDistance(lookup, want.a, want.b)
		if distance != want.distance || related != want.related || ReproductionKinProhibited(distance, related) != want.prohibited {
			t.Fatalf("kin %d→%d: (%d,%v) want (%d,%v,%v)", want.a, want.b, distance, related, want.distance, want.related, want.prohibited)
		}
	}
	// The walk is bounded at 10 refs: a twelfth-generation descendant is
	// unrelated within the bound.
	chain := map[sim.EntityID]ReproductionGenomeState{1: mustGenome(t, 1), 2: mustGenome(t, 2)}
	previous := sim.EntityID(1)
	for gen := 0; gen < 12; gen++ {
		id := sim.EntityID(10001 + gen)
		child, _, err := ReproductionChildGenome(previous, 2, chain[previous], chain[2], gen, drawsOf(7))
		if err != nil {
			t.Fatal(err)
		}
		chain[id] = child
		previous = id
	}
	chainLookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		g, ok := chain[id]
		if !ok {
			return 0, 0, false
		}
		return g.ParentA, g.ParentB, true
	})
	distance, related := ReproductionKinDistance(chainLookup, 1, previous)
	if related || ReproductionKinProhibited(distance, related) {
		t.Fatalf("walk bound: distance %d related %v", distance, related)
	}
	// Within the bound the fifth-generation descendant is still visible.
	distance, related = ReproductionKinDistance(chainLookup, 1, sim.EntityID(10001+4))
	if !related || distance != 5 {
		t.Fatalf("deep lineage: distance %d related %v", distance, related)
	}
}

// TestReproductionConsentFormulaAndSurplusGate pins the frozen thresholds at
// their boundaries.
func TestReproductionConsentFormulaAndSurplusGate(t *testing.T) {
	for _, want := range []struct {
		ownGranary, ownCapital, ownEnergy, repCapital, repGranary int64
		kin, allowed                                              bool
	}{
		{3, 2, 3, 2, 3, true, true},
		{2, 2, 3, 2, 3, true, false},  // own granary 2
		{3, 1, 3, 2, 3, true, false},  // own k 1
		{3, 2, 2, 2, 3, true, false},  // own energy 2
		{3, 2, 3, 1, 3, true, false},  // reported k 1
		{3, 2, 3, 2, 2, true, false},  // reported granary 2
		{3, 2, 3, 2, 3, false, false}, // kin distance ≤2
	} {
		if got := ReproductionConsentAllows(want.ownGranary, want.ownCapital, want.ownEnergy, want.repCapital, want.repGranary, want.kin); got != want.allowed {
			t.Fatalf("consent %+v: %v", want, got)
		}
	}
	proposer := testParentState(t, 1)
	consenter := testParentState(t, 2)
	if !ReproductionSurplusGateAllows(proposer, consenter, true, 0) {
		t.Fatal("gate-passing fixture rejected")
	}
	if ReproductionSurplusGateAllows(proposer, consenter, true, ReproductionMaxBirths) {
		t.Fatal("birth cap passed the gate")
	}
	if !ReproductionSurplusGateAllows(proposer, consenter, true, ReproductionMaxBirths-1) {
		t.Fatal("births 15 rejected")
	}
	if ReproductionSurplusGateAllows(proposer, consenter, false, 0) {
		t.Fatal("kin-prohibited pair passed the gate")
	}
	cross := consenter
	cross.Bag.Source = 1002
	if ReproductionSurplusGateAllows(proposer, cross, true, 0) {
		t.Fatal("cross-patch pair passed the gate")
	}
	dead := proposer
	dead.Genome.DiedHour = 3
	if ReproductionSurplusGateAllows(dead, consenter, true, 0) {
		t.Fatal("dead proposer passed the gate")
	}
	zeroEnergy := proposer
	zeroEnergy.Body.Energy = 0
	if ReproductionSurplusGateAllows(zeroEnergy, consenter, true, 0) {
		t.Fatal("zero-energy proposer passed the alive invariant")
	}
}

func reproductionGateWorld(t *testing.T) ([ReproductionPatchCount]ReproductionPatchState, [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState, [ReproductionFounderCount]ReproductionActorState) {
	t.Helper()
	var patches [ReproductionPatchCount]ReproductionPatchState
	var slots [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState
	// Patch 1001 produced and was gathered 12 times by the two gate
	// parents' six investments each; patch 1002 stays untouched.
	patches[0] = ReproductionPatchState{Yield: 1, Pulses: 12, Produced: 12}
	slots[0][0] = ReproductionSlotState{Gathered: 6}
	slots[0][1] = ReproductionSlotState{Gathered: 6}
	var founders [ReproductionFounderCount]ReproductionActorState
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		a, err := ReproductionFounderState(actor, 140)
		if err != nil {
			t.Fatal(err)
		}
		founders[actor-1] = a
	}
	for _, actor := range []sim.EntityID{1, 2} {
		a := testParentState(t, actor)
		a.Body.LastGatherHour = 0
		founders[actor-1] = a
	}
	return patches, slots, founders
}

// TestReproductionBirthEconomicsAndExtendedConservation runs the accepted
// birth end to end: the enumerated payments, the newborn's converted eight
// units, the extended G2a/G3 identities over the whole world, and the
// conservation rejection of every corrupted variant.
func TestReproductionBirthEconomicsAndExtendedConservation(t *testing.T) {
	patches, slots, founders := reproductionGateWorld(t)
	if _, err := ReproductionCheckConservation(patches, slots, founders, nil); err != nil {
		t.Fatalf("pre-birth world: %v", err)
	}
	hour := 5
	proposer := founders[0]
	consenter := founders[1]
	requested, err := ReproductionBirthRequest(hour, 1, 2, 3, 4, proposer)
	if err != nil {
		t.Fatal(err)
	}
	draws := reproductionDrawsForSeed(t, 0)
	lifetime := int64(ReproductionLifetimeBase + draws[6]%ReproductionLifetimeSpan)
	result, err := ReproductionBirthReply(hour, 0, true, draws, 1, 2, requested, consenter)
	if err != nil || result.Decision != ReproductionReplyAccept {
		t.Fatalf("reply: %+v %v", result, err)
	}
	// Enumerated parent payments: granary −2, energy −2, basal-spent +2,
	// birth-granary-paid +2, symmetric on both parents.
	for _, after := range []ReproductionActorState{result.Proposer, result.Consenter} {
		if after.Granary.Stock != 3 || after.Body.Energy != 3 || after.Body.BasalSpent != 8 || after.Genome.BirthGranaryPaid != 2 {
			t.Fatalf("parent after birth: %+v", after)
		}
		if after.Worksite.Capital != 3 || !reproductionAlive(after) {
			t.Fatalf("parent gate margins preserved: %+v", after)
		}
	}
	if result.Proposer.Request.Status != ReproductionRequestAccepted || result.Consenter.Request.LastReplyHour != int64(hour) {
		t.Fatalf("request rows after birth: %+v %+v", result.Proposer.Request, result.Consenter.Request)
	}
	// The newborn: converted, never created — 8 energy, 0 hunger, zero
	// capital, proposer's home patch, full genome row.
	newborn := result.Newborn
	if result.NewbornID != 10001 || newborn.Body.Energy != ReproductionNewbornEnergy || newborn.Body.Hunger != 0 ||
		newborn.Worksite != (ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour}) ||
		newborn.Granary != (ReproductionGranaryState{LastStoredMealHour: ReproductionNeverHour}) ||
		newborn.Bag.Source != 1001 || newborn.Request != ReproductionIdleRequestState() {
		t.Fatalf("newborn state: %+v", newborn)
	}
	if newborn.Genome.ParentA != 1 || newborn.Genome.ParentB != 2 || newborn.Genome.BirthHour != int64(hour) ||
		newborn.Genome.DeathHour != int64(hour)+lifetime || newborn.Genome.DiedHour != -1 || newborn.Genome.BirthGranaryPaid != 0 {
		t.Fatalf("newborn genome: %+v (lifetime %d)", newborn.Genome, lifetime)
	}
	// Conservation over the whole world with one newborn, then corruption
	// variants each rejected with zero tolerance.
	founders[0], founders[1] = result.Proposer, result.Consenter
	newborns := []ReproductionNewbornState{{ID: result.NewbornID, State: result.Newborn}}
	if _, err := ReproductionCheckConservation(patches, slots, founders, newborns); err != nil {
		t.Fatalf("post-birth world: %v", err)
	}
	corrupt := func(label string, mutate func(*[ReproductionFounderCount]ReproductionActorState, *[]ReproductionNewbornState)) {
		patches, slots := patches, slots
		founders := founders
		newborns := append([]ReproductionNewbornState(nil), newborns...)
		mutate(&founders, &newborns)
		if _, err := ReproductionCheckConservation(patches, slots, founders, newborns); !errors.Is(err, ErrReproductionContract) {
			t.Fatalf("%s: corrupted world conserved", label)
		}
	}
	corrupt("parent G2a", func(f *[ReproductionFounderCount]ReproductionActorState, _ *[]ReproductionNewbornState) {
		f[0].Genome.BirthGranaryPaid = 3
	})
	corrupt("aggregate G3", func(f *[ReproductionFounderCount]ReproductionActorState, _ *[]ReproductionNewbornState) {
		f[1].Body.Energy++
	})
	corrupt("cohort identity", func(_ *[ReproductionFounderCount]ReproductionActorState, n *[]ReproductionNewbornState) {
		(*n)[0].State.Body.Energy--
	})
	corrupt("ID sequence", func(_ *[ReproductionFounderCount]ReproductionActorState, n *[]ReproductionNewbornState) {
		(*n)[0].ID = 10002
	})
	corrupt("unknown parent", func(_ *[ReproductionFounderCount]ReproductionActorState, n *[]ReproductionNewbornState) {
		(*n)[0].State.Genome.ParentA = 42
	})
	corrupt("unbacked paid", func(f *[ReproductionFounderCount]ReproductionActorState, _ *[]ReproductionNewbornState) {
		f[2].Genome.BirthGranaryPaid = 1
	})
	// A forged mating between kin (parent 1 with its own child 10001) is
	// rejected by the derived-kinship re-check.
	kinChild, _, err := ReproductionChildGenome(1, 10001, founders[0].Genome, newborns[0].State.Genome, 20, drawsOf(7))
	if err != nil {
		t.Fatal(err)
	}
	kinNewborn := ReproductionActorState{
		Body:     ReproductionBodyState{Energy: ReproductionNewbornEnergy, LastGatherHour: ReproductionNeverHour},
		Bag:      ReproductionBagState{Source: 1001},
		Worksite: ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour},
		Granary:  ReproductionGranaryState{LastStoredMealHour: ReproductionNeverHour},
		Genome:   kinChild,
		Request:  ReproductionIdleRequestState(),
	}
	if _, err := ReproductionCheckConservation(patches, slots, founders, append(newborns, ReproductionNewbornState{ID: 10002, State: kinNewborn})); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("kin mating survived conservation")
	}
	// The gate itself is closed to the same mating before any birth
	// exists: parent 1 and its own child 10001 are at kin distance 1, so
	// the trusted walk reports kinAllowed=false.
	if ReproductionSurplusGateAllows(founders[0], newborns[0].State, false, 1) {
		t.Fatal("kin-prohibited pair passed the gate")
	}
}

// TestReproductionBirthRequestAndReplyNegativeFixtures walks the frozen
// negative-fixture list at the contract level.
func TestReproductionBirthRequestAndReplyNegativeFixtures(t *testing.T) {
	hour := 5
	proposer := testParentState(t, 1)
	consenter := testParentState(t, 2)
	otherPatch := testParentState(t, 9)

	// Filing rules: valid claim bounds, no duplicate per hour, alive only.
	if _, err := ReproductionBirthRequest(hour, 1, 1, 2, 3, proposer); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("self-addressed request admitted")
	}
	if _, err := ReproductionBirthRequest(hour, 1, 2, 9, 3, proposer); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("reported capital 9 admitted")
	}
	if _, err := ReproductionBirthRequest(hour, 1, 2, 2, 9, proposer); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("reported granary 9 admitted")
	}
	if _, err := ReproductionBirthRequest(168, 1, 2, 2, 3, proposer); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("request beyond the horizon admitted")
	}
	filed, err := ReproductionBirthRequest(hour, 1, 2, 2, 3, proposer)
	if err != nil {
		t.Fatal(err)
	}
	if filed.Request.Status != ReproductionRequestPending || filed.Request.Hour != int64(hour) || filed.Request.Addressee != 2 || filed.Request.LastRequestHour != int64(hour) {
		t.Fatalf("filed row: %+v", filed.Request)
	}
	if _, err := ReproductionBirthRequest(hour, 1, 3, 2, 3, filed); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("duplicate same-hour request admitted")
	}
	if _, err := ReproductionBirthRequest(hour, 1, 4, 2, 3, filed); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("second pending request admitted")
	}
	deadProposer := proposer
	deadProposer.Genome.DiedHour = 2
	if _, err := ReproductionBirthRequest(hour, 1, 2, 2, 3, deadProposer); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("dead proposer filed a request")
	}
	// While still pending the proposer cannot refile at all; expiry at
	// the next pulse guarantees the pending state resolves.
	if _, err := ReproductionBirthRequest(hour+1, 1, 3, 2, 3, filed); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("refile while pending admitted")
	}

	// Reply: reply without pending is a zero-mutation typed error.
	var refusal *ReproductionRefusalError
	idle := proposer
	if _, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, idle, consenter); !errors.As(err, &refusal) || refusal.Reason != ReproductionRefusalNotPending {
		t.Fatalf("reply without pending: %v", err)
	}
	// Wrong addressee in the pending row.
	misaddressed := filed
	misaddressed.Request.Addressee = 3
	if _, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, misaddressed, consenter); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("misaddressed reply admitted")
	}
	// Duplicate reply in the same hour.
	replied := consenter
	replied.Request.LastReplyHour = int64(hour)
	if _, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, filed, replied); !errors.As(err, &refusal) || refusal.Reason != ReproductionRefusalDuplicateReply {
		t.Fatalf("duplicate reply: %v", err)
	}
	// Dead addressee: typed refusal, zero mutation.
	deadConsenter := consenter
	deadConsenter.Genome.DiedHour = 2
	silent, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, filed, deadConsenter)
	if err != nil || silent.Decision != ReproductionReplyRefuseSilent || silent.Reason != ReproductionRefusalAddresseeDead ||
		silent.Proposer != filed || silent.Consenter != deadConsenter {
		t.Fatalf("dead addressee: %+v %v", silent, err)
	}
	// Proposer death mid-phase: typed refusal, zero mutation.
	deadFiled := filed
	deadFiled.Genome.DiedHour = int64(hour)
	silent, err = ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, deadFiled, consenter)
	if err != nil || silent.Decision != ReproductionReplyRefuseSilent || silent.Reason != ReproductionRefusalProposerDead {
		t.Fatalf("proposer death mid-phase: %+v %v", silent, err)
	}
	// Forged claims accepted by the consenter's formula are refused by the
	// runner gate on truth. Reported 2/3 with truth below the gate must give
	// a typed GateThresholds refusal, never an accept.
	claims := filed
	claims.Request.ReportedCapital, claims.Request.ReportedGranary = 2, 3
	poor := proposer
	poor.Granary = ReproductionGranaryState{Stock: 0, YieldTotal: 0, LastStoredMealHour: ReproductionNeverHour}
	poor.Worksite = ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour}
	poorClaims := claims
	poorClaims.Granary, poorClaims.Worksite = poor.Granary, poor.Worksite
	refused, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, poorClaims, consenter)
	if err != nil || refused.Decision != ReproductionReplyRefuse || refused.Reason != ReproductionRefusalGateThresholds {
		t.Fatalf("forged-claim truth refusal: %+v %v", refused, err)
	}
	// Reported claims below the consent formula refuse with the claims reason
	// even when the proposer's truth would qualify.
	underReported := filed
	underReported.Request.ReportedCapital, underReported.Request.ReportedGranary = 1, 1
	claimsRefused, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, underReported, consenter)
	if err != nil || claimsRefused.Decision != ReproductionReplyRefuse || claimsRefused.Reason != ReproductionRefusalConsentClaims {
		t.Fatalf("reported-claim refusal: %+v %v", claimsRefused, err)
	}
	// Consenter's own state below the consent formula.
	poorConsenter := consenter
	poorConsenter.Granary.Stock = 2
	poorConsenter.Granary.YieldTotal = 2
	consentRefused, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, claims, poorConsenter)
	if err != nil || consentRefused.Decision != ReproductionReplyRefuse || consentRefused.Reason != ReproductionRefusalConsentOwn {
		t.Fatalf("consent-own refusal: %+v %v", consentRefused, err)
	}
	if consentRefused.Proposer.Request.Status != ReproductionRequestRefused || consentRefused.Consenter.Request.LastReplyHour != int64(hour) {
		t.Fatalf("refusal must mutate only the two request rows: %+v %+v", consentRefused.Proposer.Request, consentRefused.Consenter.Request)
	}
	// Kin ≤2 refused at the policy layer (and the gate layer).
	kinRefused, err := ReproductionBirthReply(hour, 0, false, drawsOf(7), 1, 2, claims, consenter)
	if err != nil || kinRefused.Reason != ReproductionRefusalKin {
		t.Fatalf("kin refusal: %+v %v", kinRefused, err)
	}
	// Cross-patch refused at the gate layer.
	crossFiled, err := ReproductionBirthRequest(hour, 1, 9, 2, 3, proposer)
	if err != nil {
		t.Fatal(err)
	}
	crossRefused, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 9, crossFiled, otherPatch)
	if err != nil || crossRefused.Decision != ReproductionReplyRefuse || crossRefused.Reason != ReproductionRefusalCrossPatch {
		t.Fatalf("cross-patch refusal: %+v %v", crossRefused, err)
	}
	// Gate thresholds raced below cost after filing (proposer truth
	// granary 2 < 3).
	raced := claims
	raced.Granary.Stock = 2
	raced.Granary.YieldTotal = 2
	gateRefused, err := ReproductionBirthReply(hour, 0, true, drawsOf(7), 1, 2, raced, consenter)
	if err != nil || gateRefused.Reason != ReproductionRefusalGateThresholds {
		t.Fatalf("raced granary refusal: %+v %v", gateRefused, err)
	}
	// Birth cap.
	capRefused, err := ReproductionBirthReply(hour, ReproductionMaxBirths, true, drawsOf(7), 1, 2, claims, consenter)
	if err != nil || capRefused.Reason != ReproductionRefusalBirthCap {
		t.Fatalf("birth cap refusal: %+v %v", capRefused, err)
	}
	// Off-hour reply (request filed for hour 5, answered at 6).
	if _, err := ReproductionBirthReply(hour+1, 0, true, drawsOf(7), 1, 2, claims, consenter); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("off-hour reply admitted")
	}
}

// TestReproductionAgeDeathAndExpiry pins the pulse-phase world rules 413 and
// 412 and their monotonicity.
func TestReproductionAgeDeathAndExpiry(t *testing.T) {
	genome := mustGenome(t, 1) // death-hour 120
	if _, died, err := ReproductionAgeDeath(119, 1, genome); err != nil || died {
		t.Fatalf("early age-death: %v %v", died, err)
	}
	died, diedFlag, err := ReproductionAgeDeath(120, 1, genome)
	if err != nil || !diedFlag || died.DiedHour != 120 || died.DeathHour != 120 {
		t.Fatalf("age-death: %+v %v %v", died, diedFlag, err)
	}
	// Death-hour never increases; died-hour moves only from -1 to h.
	if died.LocusM != genome.LocusM || died.BirthGranaryPaid != genome.BirthGranaryPaid {
		t.Fatal("age-death mutated more than died-hour")
	}
	if _, _, err := ReproductionAgeDeath(121, 1, died); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("second age-death admitted")
	}
	if _, _, err := ReproductionAgeDeath(120, 1, genome); err != nil {
		t.Fatalf("first age-death at the death-hour: %v", err)
	}
	// Expiry at the next pulse only.
	filed, err := ReproductionBirthRequest(5, 1, 2, 2, 3, testParentState(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReproductionBirthExpiry(5, filed.Request); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("same-hour expiry admitted")
	}
	if _, err := ReproductionBirthExpiry(7, filed.Request); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("stale expiry admitted")
	}
	expired, err := ReproductionBirthExpiry(6, filed.Request)
	if err != nil || expired.Status != ReproductionRequestExpired || expired.Hour != 5 || expired.Addressee != 2 {
		t.Fatalf("expired row: %+v %v", expired, err)
	}
	if _, err := ReproductionBirthExpiry(6, expired); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("double expiry admitted")
	}
	// Refiling after expiry is legal.
	refiled, err := ReproductionBirthRequest(8, 1, 3, 2, 3, func() ReproductionActorState { a := filed; a.Request = expired; return a }())
	if err != nil || refiled.Request.Status != ReproductionRequestPending || refiled.Request.Addressee != 3 {
		t.Fatalf("refile after expiry: %+v %v", refiled.Request, err)
	}
}

// TestReproductionFounderNeutrality pins the founder-neutral genome
// construction and the runner-owned lifetime input.
func TestReproductionFounderNeutrality(t *testing.T) {
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		genome, err := ReproductionFounderGenome(actor, 120)
		if err != nil {
			t.Fatal(err)
		}
		neutral := ReproductionGenomeState{LocusM: 6, LocusY: 6, LocusW: 6, BirthHour: 0, DeathHour: 120, DiedHour: -1}
		if genome != neutral {
			t.Fatalf("founder %d genome: %+v", actor, genome)
		}
		state, err := ReproductionFounderState(actor, 96+int64(actor-1)%48)
		if err != nil {
			t.Fatal(err)
		}
		if !validReproductionActor(actor, state) || state.Body.Energy != ReproductionFounderInitialEnergy || state.Request != ReproductionIdleRequestState() {
			t.Fatalf("founder %d state: %+v", actor, state)
		}
	}
	for _, bad := range []struct {
		actor    sim.EntityID
		lifetime int64
	}{{0, 120}, {17, 120}, {1, 95}, {1, 144}, {1, 479}} {
		if _, err := ReproductionFounderGenome(bad.actor, bad.lifetime); !errors.Is(err, ErrReproductionContract) {
			t.Fatalf("founder genome %+v admitted", bad)
		}
	}
	// The idle request row is the canonical zero hour state.
	idle := ReproductionIdleRequestState()
	if idle != (ReproductionBirthRequestState{Hour: -1, LastRequestHour: -1, LastReplyHour: -1}) {
		t.Fatalf("idle row: %+v", idle)
	}
}

// Review P1: the batched basal path must equal repeated hourly invocation, so
// the frozen hourly lattice and v3 byte-identity hold for any elapsedHours.
func TestReproductionBasalBatchedEqualsHourly(t *testing.T) {
	for _, m := range []int64{5, 6, 7} {
		seed, err := ReproductionFounderState(1, 120)
		if err != nil {
			t.Fatal(err)
		}
		seed.Genome.LocusM = m
		batched := seed
		var wantSpent, wantUnmet int64
		for h := 1; h <= 5; h++ {
			next, cost, _, err := ReproductionBasal(h, 1, batched, 1)
			if err != nil {
				t.Fatalf("m=%d hourly h=%d: %v", m, h, err)
			}
			batched = next
			wantSpent += cost.EnergySpent
			wantUnmet += cost.UnmetEnergy
		}
		single, cost, _, err := ReproductionBasal(1, 1, seed, 5)
		if err != nil {
			t.Fatalf("m=%d batched: %v", m, err)
		}
		if single.Body.Energy != batched.Body.Energy || single.Body.BasalDebt != batched.Body.BasalDebt ||
			single.Body.Hunger != batched.Body.Hunger || cost.EnergySpent != wantSpent || cost.UnmetEnergy != wantUnmet {
			t.Fatalf("m=%d batched %+v/%+v != hourly %+v/%d,%d", m, single.Body, cost, batched.Body, wantSpent, wantUnmet)
		}
	}
}
