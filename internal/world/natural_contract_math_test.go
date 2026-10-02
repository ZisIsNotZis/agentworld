package world

import (
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

// naturalOccupancyPattern encodes one slot arrangement: 0 wild empty,
// 1 wild stocked, 2 cultivated (living owner), 3 cultivated (dead owner).
type naturalOccupancyPattern [NaturalSlotsPerPatch]int

func naturalOccupancySlots(pattern naturalOccupancyPattern) ([NaturalSlotsPerPatch]NaturalSlotState, []sim.EntityID, []sim.EntityID) {
	var slots [NaturalSlotsPerPatch]NaturalSlotState
	var living, dead []sim.EntityID
	for i, cell := range pattern {
		switch cell {
		case 1:
			slots[i] = NaturalSlotState{Stock: 1}
		case 2:
			slots[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
			living = append(living, 1)
		case 3:
			slots[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1}
			dead = append(dead, 2)
		}
	}
	return slots, living, dead
}

// naturalExpectedSplit independently re-derives the frozen distribution for
// one pattern and budget: cultivated slots of living owners first ascending
// (one unit each), then vacant wild slots ascending, remainder unrealized.
func naturalExpectedSplit(pattern naturalOccupancyPattern, R int64) (cultivated, wild, unrealized int64, grants []NaturalGrantEntitlement) {
	budget := R
	for i, cell := range pattern {
		if cell == 2 && budget > 0 {
			budget--
			cultivated++
			if len(grants) > 0 && grants[len(grants)-1].Owner == 1 {
				grants[len(grants)-1].Units++
			} else {
				grants = append(grants, NaturalGrantEntitlement{Owner: 1, Units: 1})
			}
		}
		_ = i
	}
	for _, cell := range pattern {
		if cell == 0 && budget > 0 {
			budget--
			wild++
		}
	}
	return cultivated, wild, budget, grants
}

// TestNaturalRegenSplitIdentityAllBudgetsAndOccupancies walks every slot
// occupancy pattern (wild empty/stocked, cultivated with living owner,
// fallow with dead owner) against every budget R in [0,8]: the per-patch
// identity R = cultivated + wildFilled + unrealized holds exactly, the
// ascending priority is respected when the budget runs short, fallow slots
// never consume budget, and the grants are one entitlement per owner
// ordered by lowest entitled slot.
func TestNaturalRegenSplitIdentityAllBudgetsAndOccupancies(t *testing.T) {
	patches := 1
	alive := NaturalAliveLookup(func(id sim.EntityID) bool { return id == 1 })
	patterns := int(1) << (2 * NaturalSlotsPerPatch) // 4^8 arrangements
	for index := 0; index < patterns; index++ {
		var pattern naturalOccupancyPattern
		for i := range pattern {
			pattern[i] = (index >> (2 * i)) & 3
		}
		slots, _, _ := naturalOccupancySlots(pattern)
		budgets := []int64{0, 8}
		if index%97 == 0 { // deterministic sample for the non-matrix budgets
			budgets = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8}
		}
		stocked := int64(0)
		for _, cell := range pattern {
			if cell == 1 {
				stocked++
			}
		}
		for _, R := range budgets {
			if R == 0 && stocked > 0 {
				continue // standing stock implies a prior nonzero budget
			}
			patch := NaturalPatchState{Yield: R, Pulses: 8, Produced: stocked, Unrealized: R*8 - stocked}
			wantCultivated, wantWild, wantUnrealized, wantGrants := naturalExpectedSplit(pattern, R)
			gotPatch, gotSlots, result, grants, err := NaturalRegenSplit(patches, 8, patch, slots, alive)
			if err != nil {
				t.Fatalf("pattern %v R=%d: %v", pattern, R, err)
			}
			if result.Cultivated != wantCultivated || result.Wild != wantWild || result.Unrealized != wantUnrealized {
				t.Fatalf("pattern %v R=%d: got %+v want (%d,%d,%d)", pattern, R, result, wantCultivated, wantWild, wantUnrealized)
			}
			if result.Cultivated+result.Wild+result.Unrealized != R {
				t.Fatalf("pattern %v R=%d: identity broken %+v", pattern, R, result)
			}
			if len(grants) != len(wantGrants) {
				t.Fatalf("pattern %v R=%d: grants %+v want %+v", pattern, R, grants, wantGrants)
			}
			for j := range grants {
				if grants[j] != wantGrants[j] {
					t.Fatalf("pattern %v R=%d: grants %+v want %+v", pattern, R, grants, wantGrants)
				}
			}
			var grantSum int64
			for _, g := range grants {
				grantSum += g.Units
			}
			if grantSum != result.Cultivated {
				t.Fatalf("pattern %v R=%d: grants %d != cultivated %d", pattern, R, grantSum, result.Cultivated)
			}
			// The patch ledger books exactly the split; the lifetime G1
			// identity holds on the updated state.
			if gotPatch.Pulses != 9 || gotPatch.Produced != patch.Produced+result.Wild || gotPatch.Granted != result.Cultivated || gotPatch.Unrealized != patch.Unrealized+result.Unrealized {
				t.Fatalf("pattern %v R=%d: patch ledger %+v", pattern, R, gotPatch)
			}
			if !validNaturalPatch(gotPatch) {
				t.Fatalf("pattern %v R=%d: patch invalid %+v", pattern, R, gotPatch)
			}
			// Ascending wild fills: the first wildEmpty-rank slots carry
			// the wild share, in index order; nothing else moved.
			wildEmptyRank := 0
			for i := range gotSlots {
				if pattern[i] == 0 {
					filled := gotSlots[i].Stock == 1
					if filled != (wildEmptyRank < int(result.Wild)) {
						t.Fatalf("pattern %v R=%d: slot %d fill %v breaks the ascending order", pattern, R, i, filled)
					}
					wildEmptyRank++
				}
				if !validNaturalSlot(gotSlots[i]) {
					t.Fatalf("pattern %v R=%d: slot %d invalid %+v", pattern, R, i, gotSlots[i])
				}
			}
		}
	}
	// R=0: nothing granted, nothing filled, nothing unrealized.
	patch := NaturalPatchState{Pulses: 8}
	slots, _, _ := naturalOccupancySlots(naturalOccupancyPattern{2, 0, 0, 0, 0, 0, 0, 0})
	_, _, result, grants, err := NaturalRegenSplit(patches, 8, patch, slots, alive)
	if err != nil || result != (NaturalRegenSplitResult{}) || len(grants) != 0 {
		t.Fatalf("R=0: %+v %v %v", result, grants, err)
	}
}

// TestNaturalRegenSplitNoLocusMints is the corrigendum's binding proof: for
// every locus combination m,y,w ∈ {5,6,7}, thirty pulses of split → grant
// → spoilage keep the per-patch identity exact and the granary receipts
// exactly three units per pulse (the cultivated entitlement), so no locus
// can mint food beyond the budget R.
func TestNaturalRegenSplitNoLocusMints(t *testing.T) {
	patches := 2
	for m := int64(5); m <= 7; m++ {
		for y := int64(5); y <= 7; y++ {
			for w := int64(5); w <= 7; w++ {
				owner, err := NaturalFounderState(patches, 1, 140)
				if err != nil {
					t.Fatal(err)
				}
				owner.Genome.LocusM, owner.Genome.LocusY, owner.Genome.LocusW = m, y, w
				patch := NaturalPatchState{Yield: 8}
				var slots [NaturalSlotsPerPatch]NaturalSlotState
				for i := 0; i < 3; i++ {
					slots[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
				}
				alive := NaturalAliveLookup(func(id sim.EntityID) bool { return id == 1 })
				for pulse := 0; pulse < 30; pulse++ {
					nextPatch, nextSlots, result, grants, err := NaturalRegenSplit(patches, pulse, patch, slots, alive)
					if err != nil {
						t.Fatalf("m=%d y=%d w=%d pulse %d: %v", m, y, w, pulse, err)
					}
					if result.Cultivated+result.Wild+result.Unrealized != 8 || result.Cultivated != 3 {
						t.Fatalf("m=%d y=%d w=%d pulse %d: split %+v", m, y, w, pulse, result)
					}
					if len(grants) != 1 || grants[0].Owner != 1 || grants[0].Units != 3 {
						t.Fatalf("m=%d y=%d w=%d pulse %d: grants %+v", m, y, w, pulse, grants)
					}
					owner, _, err = NaturalCultivatedGrant(patches, 1, owner, grants[0].Units)
					if err != nil {
						t.Fatalf("m=%d y=%d w=%d pulse %d: grant %v", m, y, w, pulse, err)
					}
					owner, _, err = NaturalSpoilage(patches, 1, owner)
					if err != nil {
						t.Fatalf("m=%d y=%d w=%d pulse %d: spoilage %v", m, y, w, pulse, err)
					}
					patch, slots = nextPatch, nextSlots
					// Flat receipts: cultivatedTotal == 3·(pulse+1) exactly,
					// whatever the loci.
					if owner.Granary.CultivatedTotal != 3*int64(pulse+1) {
						t.Fatalf("m=%d y=%d w=%d pulse %d: minted %+v", m, y, w, pulse, owner.Granary)
					}
					// G2a holds every pulse.
					if !validNaturalActor(patches, 1, owner) {
						t.Fatalf("m=%d y=%d w=%d pulse %d: actor invalid %+v", m, y, w, pulse, owner)
					}
				}
			}
		}
	}
}

// TestNaturalSpoilageExactLoopBoundsAndForgedDebt pins the S=24 loop: one
// rotted unit per full debt period, the carry transient inside [0,1367],
// dead owners rot identically, and forged debts are rejected with zero
// mutation.
func TestNaturalSpoilageExactLoopBoundsAndForgedDebt(t *testing.T) {
	patches := 2
	build := func(stock, debt int64, dead bool) (sim.EntityID, NaturalActorState) {
		a, err := NaturalFounderState(patches, 1, 140)
		if err != nil {
			t.Fatal(err)
		}
		a.Granary = NaturalGranaryState{Stock: stock, CultivatedTotal: stock, SpoilDebt: debt, LastStoredMealHour: NaturalNeverHour}
		if dead {
			a.Body.Energy = 0
			a.Body.BasalSpent = NaturalFounderInitialEnergy
			a.Genome.DiedHour = 10
		}
		if !validNaturalActor(patches, 1, a) {
			t.Fatalf("fixture invalid: %+v", a)
		}
		return 1, a
	}
	cases := []struct {
		stock, debt int64
		wantStock   int64
		wantSpoiled int64
		wantDebt    int64
		dead        bool
		wantErr     bool
	}{
		{0, 0, 0, 0, 0, false, false},
		{5, 0, 5, 0, 5, false, false},    // carry only
		{23, 0, 23, 0, 23, false, false}, // still below S
		{24, 0, 23, 1, 0, false, false},  // exactly one rot
		{30, 0, 29, 1, 6, false, false},  // one rot plus carry
		{72, 0, 69, 3, 0, false, false},  // three rots
		{1, 23, 0, 1, 0, false, false},   // carry crosses S
		{3, 10, 3, 0, 13, false, false},  // mixed carry below S
		{5, 0, 5, 0, 5, true, false},     // dead owners rot identically
		{0, 24, 0, 0, 24, false, true},   // forged: debt ≥ S with no stock
		{1, 1367, 0, 0, 0, false, true},  // forged: 1367+1 > 1367
	}
	for i, c := range cases {
		owner, a := build(c.stock, c.debt, c.dead)
		got, result, err := NaturalSpoilage(patches, owner, a)
		if c.wantErr {
			if !errors.Is(err, ErrNaturalContract) {
				t.Fatalf("case %d: forged debt admitted: %v", i, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if got.Granary.Stock != c.wantStock || got.Granary.Spoiled != c.wantSpoiled || got.Granary.SpoilDebt != c.wantDebt {
			t.Fatalf("case %d: got %+v want stock %d spoiled %d debt %d", i, got.Granary, c.wantStock, c.wantSpoiled, c.wantDebt)
		}
		if result.Spoiled != c.wantSpoiled {
			t.Fatalf("case %d: result %+v", i, result)
		}
		if !validNaturalActor(patches, owner, got) {
			t.Fatalf("case %d: invalid after spoilage %+v", i, got)
		}
	}
	// The cultivated-received ledger bounds the spoiled counter: a full
	// 1344-unit granary rots 56 units in one pulse and stays in bounds.
	owner, a := build(NaturalGranaryStockMax, 0, false)
	got, result, err := NaturalSpoilage(patches, owner, a)
	if err != nil {
		t.Fatal(err)
	}
	if result.Spoiled != 56 || got.Granary.Stock != NaturalGranaryStockMax-56 || got.Granary.SpoilDebt != 0 {
		t.Fatalf("full granary: %+v %+v", got.Granary, result)
	}
}

// TestNaturalWearRevertsLowestIndexIncludesDeadOwnersAndForgedDebt pins the
// v4-verbatim divisor loop plus the slot revert: each decayed point reverts
// the owner's lowest-index cultivated slot to wild with stock 0 (gather
// history retained), dead owners wear identically, k=0 is a no-op, forged
// debts and capital without slots are rejected with zero mutation.
func TestNaturalWearRevertsLowestIndexIncludesDeadOwnersAndForgedDebt(t *testing.T) {
	patches := 2
	build := func(capital, debt int64, tenured []int, dead bool) (sim.EntityID, NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState) {
		a, err := NaturalFounderState(patches, 1, 140)
		if err != nil {
			t.Fatal(err)
		}
		a.Worksite = NaturalWorksiteState{Capital: capital, WearDebt: debt, PointsCreated: capital, InvestedUnits: NaturalPointCostWip * capital, LastBuildHour: NaturalNeverHour}
		var slots [NaturalSlotsPerPatch]NaturalSlotState
		for _, i := range tenured {
			slots[i] = NaturalSlotState{Gathered: int64(i), Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
		}
		if dead {
			a.Body.Energy = 0
			a.Body.BasalSpent = NaturalFounderInitialEnergy
			a.Genome.DiedHour = 10
		}
		if !validNaturalActor(patches, 1, a) {
			t.Fatalf("fixture invalid: %+v", a)
		}
		return 1, a, slots
	}
	// Neutral locus w=6: capital 2 decays when the debt reaches 6; the
	// tenured slots at indices 5 and 2 revert lowest-index first.
	owner, a, slots := build(2, 0, []int{5, 2}, false)
	// Per pulse at the neutral locus 6: debt carries 2, then decays at 6.
	wantCapital := []int64{2, 2, 1, 1, 1, 1, 1, 1, 0}
	wantDebt := []int64{2, 4, 0, 1, 2, 3, 4, 5, 0}
	wantTenured := [][]int{{5, 2}, {5, 2}, {5}, {5}, {5}, {5}, {5}, {5}, {}}
	for pulse := 0; pulse < 9; pulse++ {
		next, gotSlots, result, err := NaturalWear(patches, owner, a, 1001, slots)
		if err != nil {
			t.Fatalf("pulse %d: %v", pulse, err)
		}
		a, slots = next, gotSlots
		if result.PointsDecayed > 1 {
			t.Fatalf("pulse %d decayed %d", pulse, result.PointsDecayed)
		}
		if pulse == 2 { // debt 6: first decay reverts the LOWEST index, 2
			if !reflect.DeepEqual(result.RevertedSlots, []int{2}) {
				t.Fatalf("pulse 1: %+v", result)
			}
			if slots[2].Mode != NaturalTenureWild || slots[2].Owner != 0 || slots[2].Cultivated != 0 || slots[2].Stock != 0 || slots[2].Gathered != 2 {
				t.Fatalf("pulse 1 revert shape: %+v", slots[2])
			}
		}
		if pulse == 8 { // debt 6: second decay reverts slot 5
			if !reflect.DeepEqual(result.RevertedSlots, []int{5}) {
				t.Fatalf("pulse 3: %+v", result)
			}
			if slots[5].Gathered != 5 || slots[5].Mode != NaturalTenureWild {
				t.Fatalf("pulse 3 revert shape: %+v", slots[5])
			}
		}
		if a.Worksite.Capital != wantCapital[pulse] || a.Worksite.WearDebt != wantDebt[pulse] {
			t.Fatalf("pulse %d: worksite %+v want capital %d debt %d", pulse, a.Worksite, wantCapital[pulse], wantDebt[pulse])
		}
		for _, i := range wantTenured[pulse] {
			if slots[i].Mode != NaturalTenureCultivated {
				t.Fatalf("pulse %d: tenured slot %d disturbed", pulse, i)
			}
		}
		if !validNaturalActor(patches, owner, a) {
			t.Fatalf("pulse %d invalid: %+v", pulse, a)
		}
	}
	if a.Worksite.PointsDecayed != 2 || a.Worksite.Capital != 0 || a.Worksite.WearDebt != 0 {
		t.Fatalf("final worksite: %+v", a.Worksite)
	}
	// k=0 is a no-op: no decay, no revert, no error.
	owner, a, slots = build(0, 0, nil, false)
	next, gotSlots, result, err := NaturalWear(patches, owner, a, 1001, slots)
	if err != nil || result.PointsDecayed != 0 || len(result.RevertedSlots) != 0 || next.Worksite.WearDebt != 0 || gotSlots != slots {
		t.Fatalf("k=0: %+v %+v %v", result, gotSlots, err)
	}
	// Dead owners wear identically (the fallow window).
	owner, a, slots = build(2, 4, []int{0, 1}, true)
	next, gotSlots, result, err = NaturalWear(patches, owner, a, 1001, slots)
	if err != nil || result.PointsDecayed != 1 || !reflect.DeepEqual(result.RevertedSlots, []int{0}) {
		t.Fatalf("dead owner wear: %+v %v", result, err)
	}
	if next.Worksite.Capital != 1 || next.Worksite.WearDebt != 0 || gotSlots[0].Mode != NaturalTenureWild || gotSlots[1].Owner != 1 {
		t.Fatalf("dead owner after decay: %+v %+v", next.Worksite, gotSlots)
	}
	// Locus w=5 drains faster: a carried debt of 4 crosses 5 immediately.
	owner, a, slots = build(1, 4, []int{3}, false)
	a.Genome.LocusW = 5
	next, gotSlots, result, err = NaturalWear(patches, owner, a, 1001, slots)
	if err != nil || result.PointsDecayed != 1 || !reflect.DeepEqual(result.RevertedSlots, []int{3}) {
		t.Fatalf("locus 5 wear: %+v %v", result, err)
	}
	if next.Worksite.Capital != 0 {
		t.Fatalf("locus 5 capital: %+v", next.Worksite)
	}
	// Forged debts exceed the transient bound 14: rejected, zero mutation.
	owner, a, slots = build(8, 7, []int{0}, false)
	if _, _, _, err := NaturalWear(patches, owner, a, 1001, slots); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("forged wear debt admitted")
	}
	// A capital point without a tenured slot is an inconsistent state:
	// rejected, zero mutation.
	owner, a, slots = build(1, 5, nil, false)
	if _, _, _, err := NaturalWear(patches, owner, a, 1001, slots); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("capital without slot admitted")
	}
	// A cross-patch revert is an invalid call with zero mutation.
	owner, a, slots = build(1, 0, []int{0}, false)
	if _, _, _, err := NaturalWear(patches, owner, a, 1002, slots); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("cross-patch wear admitted")
	}
}

// TestNaturalBuildSlotClaimAndRefusalsZeroMutation pins the completion
// claim: the lowest-index free wild slot with stock 0 in the builder's home
// patch, and the typed refusals (no wild slot, wild slots all stocked,
// cross-patch) with zero mutation.
func TestNaturalBuildSlotClaimAndRefusalsZeroMutation(t *testing.T) {
	patches := 2
	hour := 5
	parent, err := NaturalFounderState(patches, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	// One standing point (slot 0), one in progress, the bag unit in hand.
	parent.Worksite = NaturalWorksiteState{Capital: 1, Wip: 1, PointsCreated: 1, InvestedUnits: 3, LastBuildHour: NaturalNeverHour}
	parent.Bag.Units = 1
	parent.Body.LastGatherHour = 4
	if !validNaturalActor(patches, 1, parent) {
		t.Fatal("fixture")
	}
	slots := func() [NaturalSlotsPerPatch]NaturalSlotState {
		var s [NaturalSlotsPerPatch]NaturalSlotState
		s[0] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
		return s
	}
	// Claim: slots 0 tenured, 1 stocked wild, 2 and 3 free → index 2.
	home := slots()
	home[1] = NaturalSlotState{Stock: 1}
	after, gotSlots, result, err := NaturalBuild(patches, hour, 1, parent, 1001, home)
	if err != nil || result.PointsCompleted != 1 || result.ClaimedSlotIndex != 2 {
		t.Fatalf("claim: %+v %v", result, err)
	}
	if gotSlots[2].Mode != NaturalTenureCultivated || gotSlots[2].Owner != 1 || gotSlots[2].Cultivated != 1 || gotSlots[2].Stock != 0 {
		t.Fatalf("claimed slot: %+v", gotSlots[2])
	}
	if gotSlots[1].Stock != 1 || gotSlots[1].Mode != NaturalTenureWild {
		t.Fatalf("stocked slot disturbed: %+v", gotSlots[1])
	}
	if after.Worksite.Capital != 2 || after.Worksite.Wip != 0 || after.Worksite.InvestedUnits != 4 || after.Bag.Units != 0 || after.Worksite.LastBuildHour != int64(hour) {
		t.Fatalf("after claim: %+v", after)
	}
	if !validNaturalActor(patches, 1, after) {
		t.Fatal("invalid after claim")
	}
	// Refusal: every slot tenured (occupied) — typed, zero mutation.
	allTenured := slots()
	for i := range allTenured {
		allTenured[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
	}
	var refusalErr *NaturalBuildRefusalError
	_, _, _, err = NaturalBuild(patches, hour, 1, parent, 1001, allTenured)
	if !errors.As(err, &refusalErr) || refusalErr.Reason != NaturalBuildRefusalNoWildSlot {
		t.Fatalf("occupied refusal: %v", err)
	}
	if !reflect.DeepEqual(parent, naturalBuildRefusalProbe(t, patches)) || parent.Bag.Units != 1 {
		t.Fatal("occupied refusal mutated the actor")
	}
	// Refusal: wild slots remain but all hold standing stock.
	allStocked := slots()
	for i := 1; i < NaturalSlotsPerPatch; i++ {
		allStocked[i] = NaturalSlotState{Stock: 1}
	}
	_, _, _, err = NaturalBuild(patches, hour, 1, parent, 1001, allStocked)
	if !errors.As(err, &refusalErr) || refusalErr.Reason != NaturalBuildRefusalWildAllStocked {
		t.Fatalf("stocked refusal: %v", err)
	}
	if parent.Bag.Units != 1 || parent.Worksite.Wip != 1 {
		t.Fatal("stocked refusal mutated the actor")
	}
	// Cross-patch: an invalid call with zero mutation.
	_, _, _, err = NaturalBuild(patches, hour, 1, parent, 1002, home)
	if !errors.Is(err, ErrNaturalContract) || parent.Bag.Units != 1 {
		t.Fatal("cross-patch build admitted")
	}
	// Empty bag: v4-verbatim invalid call.
	empty := parent
	empty.Bag.Units = 0
	if _, _, _, err := NaturalBuild(patches, hour, 1, empty, 1001, home); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("empty-bag build admitted")
	}
	// k+wip at the cap: v4-verbatim invalid call.
	full := parent
	full.Worksite.Capital = 8
	full.Worksite.Wip = 0
	full.Worksite.InvestedUnits = 16
	full.Worksite.PointsCreated = 8
	if _, _, _, err := NaturalBuild(patches, hour, 1, full, 1001, home); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("cap build admitted")
	}
	// One build window per hour: v4-verbatim invalid call.
	repeat := parent
	repeat.Worksite.LastBuildHour = int64(hour)
	if _, _, _, err := NaturalBuild(patches, hour, 1, repeat, 1001, home); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("second build window admitted")
	}
}

// TestNaturalInheritedLoopsMatchV4Verbatim proves the inherited v4
// arithmetic is byte-identical: basal (starvation included), consume,
// eat-stored, gather, the build conversion, and the wear loop track the v4
// functions step for step on parallel neutral-locus states.
func TestNaturalInheritedLoopsMatchV4Verbatim(t *testing.T) {
	// Basal: identical costs and state until v5 starves and writes its
	// died-hour.
	v4, err := ReproductionFounderState(1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v5, err := NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	for hour := 0; hour <= 10; hour++ {
		v4Next, v4Cost, _, err4 := ReproductionBasal(hour, 1, v4, 1)
		if err4 != nil {
			t.Fatal(err4)
		}
		v5Next, v5Cost, starved, err5 := NaturalBasal(2, hour, 1, v5, 1)
		if err5 != nil {
			t.Fatal(err5)
		}
		if v4Cost != (ReproductionBasalCost{EnergySpent: v5Cost.EnergySpent, UnmetEnergy: v5Cost.UnmetEnergy}) {
			t.Fatalf("hour %d basal cost: v4 %+v v5 %+v", hour, v4Cost, v5Cost)
		}
		if v4Next.Body.Energy != v5Next.Body.Energy || v4Next.Body.BasalSpent != v5Next.Body.BasalSpent || v4Next.Body.Hunger != v5Next.Body.Hunger || v4Next.Body.BasalDebt != v5Next.Body.BasalDebt {
			t.Fatalf("hour %d basal state: v4 %+v v5 %+v", hour, v4Next.Body, v5Next.Body)
		}
		if starved != (hour == 10) || v5Next.Genome.DiedHour != 10 && hour == 10 {
			t.Fatalf("hour %d starvation bookkeeping", hour)
		}
		v4, v5 = v4Next, v5Next
	}
	if _, _, _, err := NaturalBasal(2, 11, 1, v5, 1); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("basal on a dead actor admitted")
	}
	// Consume.
	v4, err = ReproductionFounderState(1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v4.Bag.Units = 1
	v4.Body.LastGatherHour = 0
	v5, err = NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v5.Bag.Units = 1
	v5.Body.LastGatherHour = 0
	v4Next, v4Meal, _ := ReproductionConsume(1, v4)
	v5Next, v5Meal, err := NaturalConsume(2, 1, v5)
	if err != nil || v4Meal != (ReproductionMeal{EnergyGained: v5Meal.EnergyGained, EnergyCapLost: v5Meal.EnergyCapLost}) ||
		v4Next.Body.Energy != v5Next.Body.Energy || v4Next.Body.Hunger != v5Next.Body.Hunger || v4Next.Bag.Units != v5Next.Bag.Units {
		t.Fatalf("consume: v4 %+v v5 %+v %v", v4Next.Body, v5Next.Body, err)
	}
	// Eat-stored at the energy ceiling: counted spill.
	v4, err = ReproductionFounderState(1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v4.Granary = ReproductionGranaryState{Stock: 5, YieldTotal: 5}
	v5, err = NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v5.Granary = NaturalGranaryState{Stock: 5, CultivatedTotal: 5, LastStoredMealHour: NaturalNeverHour}
	v4Next, v4Meal, _ = ReproductionEatStored(0, 1, v4)
	v5Next, v5Meal, err = NaturalEatStored(2, 0, 1, v5)
	if err != nil || v4Meal != (ReproductionMeal{EnergyGained: v5Meal.EnergyGained, EnergyCapLost: v5Meal.EnergyCapLost}) ||
		v4Next.Body.Energy != v5Next.Body.Energy || v4Next.Body.CapLost != v5Next.Body.CapLost ||
		v4Next.Granary.Stock != v5Next.Granary.Stock || v4Next.Granary.StoredMeals != v5Next.Granary.StoredMeals {
		t.Fatalf("eat-stored: v4 %+v v5 %+v %v", v4Next, v5Next, err)
	}
	// Gather from a stocked home slot.
	v4, err = ReproductionFounderState(1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v5, err = NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v4Slot, v4Next, _ := ReproductionGather(1001, 2001, 1, 5, ReproductionSlotState{Stock: 1}, v4)
	v5Slot, v5Next, err := NaturalGather(2, 1001, 2001, 1, 5, NaturalSlotState{Stock: 1}, v5)
	if err != nil || v4Slot.Stock != v5Slot.Stock || v4Slot.Gathered != v5Slot.Gathered ||
		v4Next.Bag.Units != v5Next.Bag.Units || v4Next.Body.LastGatherHour != v5Next.Body.LastGatherHour {
		t.Fatalf("gather: v4 %+v v5 %+v %v", v4Next, v5Next, err)
	}
	// Build conversion with a completing point (v5 additionally claims the
	// lowest free wild slot; the worksite matches v4 exactly).
	v4, err = ReproductionFounderState(1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v4.Bag.Units = 1
	v4.Body.LastGatherHour = 0
	v4.Worksite = ReproductionWorksiteState{Capital: 1, Wip: 1, PointsCreated: 1, InvestedUnits: 3}
	v5, err = NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v5.Bag.Units = 1
	v5.Body.LastGatherHour = 0
	v5.Worksite = NaturalWorksiteState{Capital: 1, Wip: 1, PointsCreated: 1, InvestedUnits: 3}
	var wild [NaturalSlotsPerPatch]NaturalSlotState
	v4Next, v4Result, _ := ReproductionBuild(5, 1, v4)
	v5Next, gotBuildSlots, v5Result, err := NaturalBuild(2, 5, 1, v5, 1001, wild)
	if err != nil || v4Result.PointsCompleted != v5Result.PointsCompleted || v4Result.PointsCompleted != 1 ||
		v4Next.Worksite.Capital != v5Next.Worksite.Capital || v4Next.Worksite.Wip != v5Next.Worksite.Wip ||
		v4Next.Worksite.InvestedUnits != v5Next.Worksite.InvestedUnits || v4Next.Worksite.PointsCreated != v5Next.Worksite.PointsCreated ||
		v4Next.Bag.Units != v5Next.Bag.Units {
		t.Fatalf("build: v4 %+v v5 %+v %v", v4Next.Worksite, v5Next.Worksite, err)
	}
	if v5Result.ClaimedSlotIndex != 0 || gotBuildSlots[0].Mode != NaturalTenureCultivated || gotBuildSlots[0].Owner != 1 {
		t.Fatalf("build claim: %+v %+v", v5Result, gotBuildSlots[0])
	}
	// Wear: v5 with a full tenure set matches the v4 loop exactly (each
	// decay reverts one slot, so capital and decayed track v4).
	v4, err = ReproductionFounderState(1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v4.Worksite = ReproductionWorksiteState{Capital: 3, PointsCreated: 3, InvestedUnits: 6}
	v5, err = NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	v5.Worksite = NaturalWorksiteState{Capital: 3, PointsCreated: 3, InvestedUnits: 6}
	var tenured [NaturalSlotsPerPatch]NaturalSlotState
	for i := 0; i < 3; i++ {
		tenured[i] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
	}
	for pulse := 0; pulse < 20; pulse++ {
		v4Next, v4Result, _ := ReproductionWear(1, v4)
		v5Next, _, v5Result, err := NaturalWear(2, 1, v5, 1001, tenured)
		if err != nil || v4Result.PointsDecayed != v5Result.PointsDecayed ||
			v4Next.Worksite.Capital != v5Next.Worksite.Capital || v4Next.Worksite.WearDebt != v5Next.Worksite.WearDebt ||
			v4Next.Worksite.PointsDecayed != v5Next.Worksite.PointsDecayed {
			t.Fatalf("pulse %d wear: v4 %+v v5 %+v %v", pulse, v4Next.Worksite, v5Next.Worksite, err)
		}
		v4, v5 = v4Next, v5Next
	}
	// Neutral wear on capital 3 ends at zero with the debt settled.
	if v5.Worksite.Capital != 0 || v5.Worksite.WearDebt != 0 || v5.Worksite.PointsDecayed != 3 {
		t.Fatalf("wear tail: %+v", v5.Worksite)
	}
}

// TestNaturalCultivatedGrantFallowAndG2aLedger pins rule 508: a living
// owner's flat grant rises stock and the cultivated-received ledger
// together (G2a exact); a dead owner's grant is typed fallow with zero
// mutation.
func TestNaturalCultivatedGrantFallowAndG2aLedger(t *testing.T) {
	patches := 2
	owner, err := NaturalFounderState(patches, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	after, result, err := NaturalCultivatedGrant(patches, 1, owner, 3)
	if err != nil || result.Realized != 3 || result.Fallowed != 0 {
		t.Fatalf("grant: %+v %v", result, err)
	}
	if after.Granary.Stock != 3 || after.Granary.CultivatedTotal != 3 {
		t.Fatalf("granary: %+v", after.Granary)
	}
	if !validNaturalActor(patches, 1, after) {
		t.Fatal("invalid after grant")
	}
	// Bound: a grant beyond the eight slots of the home patch is invalid.
	if _, _, err := NaturalCultivatedGrant(patches, 1, after, 9); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("oversized grant admitted")
	}
	if _, _, err := NaturalCultivatedGrant(patches, 1, after, 0); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("empty grant admitted")
	}
	// Dead owner: typed fallow, zero mutation, no yield.
	dead := owner
	dead.Body.Energy = 0
	dead.Body.BasalSpent = NaturalFounderInitialEnergy
	dead.Genome.DiedHour = 10
	var fallowErr *NaturalFallowError
	_, result, err = NaturalCultivatedGrant(patches, 1, dead, 3)
	if !errors.As(err, &fallowErr) || fallowErr.Owner != 1 || fallowErr.Units != 3 {
		t.Fatalf("fallow: %v", err)
	}
	if result.Realized != 0 || result.Fallowed != 3 || !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("fallow result: %+v", result)
	}
	if !reflect.DeepEqual(dead.Granary, owner.Granary) {
		t.Fatalf("fallow mutated the granary: %+v", dead.Granary)
	}
}

// TestNaturalLandIdentityDeadOwnerWindow pins the corrigendum's G2b form:
// during a dead owner's decay window the all-tenure-holders identity holds
// every hour while the alive-only equality fails by exactly the dead
// owner's standing capital; the documented violations are all rejected.
func TestNaturalLandIdentityDeadOwnerWindow(t *testing.T) {
	patches := 2
	buildWorld := func(deadCapital int64, tenured int) ([]NaturalPatchState, [][]NaturalSlotState, []NaturalActorState) {
		dead, err := NaturalFounderState(patches, 1, 140)
		if err != nil {
			t.Fatal(err)
		}
		dead.Worksite = NaturalWorksiteState{Capital: deadCapital, PointsCreated: deadCapital, InvestedUnits: NaturalPointCostWip * deadCapital}
		// The dead owner starved at hour 10 with a full ledger.
		dead.Body.Energy = 0
		dead.Body.BasalSpent = NaturalFounderInitialEnergy
		dead.Genome.DiedHour = 10
		// An inaccessible granary with a coherent G2a ledger; the two
		// tenured slots carry the gather history behind the invested units.
		dead.Granary = NaturalGranaryState{Stock: 3, CultivatedTotal: 3, LastStoredMealHour: NaturalNeverHour}
		var slots [NaturalSlotsPerPatch]NaturalSlotState
		for i := 0; i < tenured; i++ {
			slots[i] = NaturalSlotState{Gathered: 2, Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
		}
		founders := make([]NaturalActorState, 16)
		for i := range founders {
			id := sim.EntityID(i + 1)
			if id == 1 {
				founders[i] = dead
				continue
			}
			founders[i], err = NaturalFounderState(patches, id, 140)
			if err != nil {
				t.Fatal(err)
			}
		}
		patchStates := []NaturalPatchState{{Yield: 8, Pulses: 2, Produced: 4, Granted: 3, Unrealized: 9}, {}}
		slotStates := [][]NaturalSlotState{slots[:], make([]NaturalSlotState, NaturalSlotsPerPatch)}
		return patchStates, slotStates, founders
	}
	// Dead owner with capital 2 and both slots still tenured: the exact
	// invariant holds; the alive-only equality is weaker by exactly 2.
	patchStates, slotStates, founders := buildWorld(2, 2)
	balance, err := NaturalCheckConservation(patches, patchStates, slotStates, founders, nil, 2016)
	if err != nil {
		t.Fatalf("dead-owner window not conserved: %v", err)
	}
	if balance.TenuredSlots != 2 || balance.Capital != 2 {
		t.Fatalf("dead-owner window balance: %+v", balance)
	}
	var aliveCapital, aliveTenured int64
	for id, a := range map[sim.EntityID]NaturalActorState{1: founders[0], 2: founders[1]} {
		_ = id
		if naturalAlive(a) {
			aliveCapital += a.Worksite.Capital
		}
	}
	for _, slot := range slotStates[0] {
		if slot.Mode == NaturalTenureCultivated {
			aliveTenured++
		}
	}
	if aliveCapital != 0 || aliveTenured != 2 || aliveCapital == aliveTenured {
		t.Fatalf("documented alive-only gap: capital %d tenured %d", aliveCapital, aliveTenured)
	}
	// Tenured slots without their owner in the roster: rejected.
	_, _, founders = buildWorld(1, 2)
	rogue := founders[0]
	rogue.Worksite.Capital = 1 // ledger says one point, two slots tenured
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, replaceFounder(founders, 0, rogue), nil, 2016); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("capital/tenure mismatch admitted")
	}
	// A tenured slot whose owner lives on another patch: rejected.
	_, slotStates, founders = buildWorld(1, 1)
	misplaced := founders[1]
	misplaced.Bag = NaturalBagState{Source: 1002}
	slotStates[0][0].Owner = 2
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, replaceFounder(founders, 1, misplaced), nil, 2016); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("cross-patch tenure admitted")
	}
	// A tenured slot owned by an unknown actor: rejected.
	_, slotStates, founders = buildWorld(0, 1)
	slotStates[0][0].Owner = 2017
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, founders, nil, 2016); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("unregistered tenure owner admitted")
	}
}

func replaceFounder(founders []NaturalActorState, index int, state NaturalActorState) []NaturalActorState {
	out := append([]NaturalActorState(nil), founders...)
	out[index] = state
	return out
}

// TestNaturalCheckConservationScriptedWorlds cross-checks the extended
// gates against small scripted worlds: genesis, the gate-parent fixture,
// the post-birth cohort, and the typed corruption negatives.
func TestNaturalCheckConservationScriptedWorlds(t *testing.T) {
	patches := 2
	// Genesis: 176 initial units, nothing else.
	patchStates := []NaturalPatchState{{}, {}}
	slotStates := make([][]NaturalSlotState, 2)
	for p := range slotStates {
		slotStates[p] = make([]NaturalSlotState, NaturalSlotsPerPatch)
	}
	founders := make([]NaturalActorState, 16)
	for i := range founders {
		var err error
		founders[i], err = NaturalFounderState(patches, sim.EntityID(i+1), 140)
		if err != nil {
			t.Fatal(err)
		}
	}
	balance, err := NaturalCheckConservation(patches, patchStates, slotStates, founders, nil, 2016)
	if err != nil {
		t.Fatal(err)
	}
	if balance.InitialEnergy != 176 || balance.Births != 0 || balance.Energy != 176 || balance.BasalSpent != 0 {
		t.Fatalf("genesis balance: %+v", balance)
	}
	// The gate-parent fixture world.
	gatePatches, gateSlots, gateFounders := naturalGateFixtureWorld(t, patches)
	if _, err := NaturalCheckConservation(patches, gatePatches, gateSlots, gateFounders, nil, 2016); err != nil {
		t.Fatalf("gate fixture: %v", err)
	}
	// Corruption negatives, each rejected with the typed contract error.
	corrupt := func(mutate func([]NaturalPatchState, [][]NaturalSlotState, []NaturalActorState, []NaturalNewbornState)) {
		patchesState := []NaturalPatchState{gatePatches[0], gatePatches[1]}
		slots := make([][]NaturalSlotState, 2)
		for p := range gateSlots {
			slots[p] = append([]NaturalSlotState(nil), gateSlots[p]...)
		}
		actors := append([]NaturalActorState(nil), gateFounders...)
		var newborns []NaturalNewbornState
		mutate(patchesState, slots, actors, newborns)
		if _, err := NaturalCheckConservation(patches, patchesState, slots, actors, newborns, 2016); !errors.Is(err, ErrNaturalContract) {
			t.Fatal("corruption admitted")
		}
	}
	// Unaccounted food: stock without a received ledger (G2a).
	corrupt(func(p []NaturalPatchState, s [][]NaturalSlotState, a []NaturalActorState, _ []NaturalNewbornState) {
		a[0].Granary.Stock++
	})
	// Slot gathered beyond the production pulses (G1).
	corrupt(func(_ []NaturalPatchState, s [][]NaturalSlotState, _ []NaturalActorState, _ []NaturalNewbornState) {
		s[0][7].Gathered = 12
	})
	// An invested unit without a gathered source (G1 intake).
	corrupt(func(_ []NaturalPatchState, _ [][]NaturalSlotState, a []NaturalActorState, _ []NaturalNewbornState) {
		a[2].Worksite.InvestedUnits++
	})
	// A created point without its invested units (G2b).
	corrupt(func(_ []NaturalPatchState, _ [][]NaturalSlotState, a []NaturalActorState, _ []NaturalNewbornState) {
		a[2].Worksite.PointsCreated++
		a[2].Worksite.Capital++
	})
	// Energy from nothing (G3).
	corrupt(func(_ []NaturalPatchState, _ [][]NaturalSlotState, a []NaturalActorState, _ []NaturalNewbornState) {
		a[3].Body.Energy++
	})
	// Roster-size-free: the newborn count plays no gate role; only the
	// per-newborn cohort identity does.
	newborns := []NaturalNewbornState{naturalScriptedNewborn(t, patches, 2017, 1, 2, 1001), naturalScriptedNewborn(t, patches, 2018, 3, 4, 1002)}
	big := make([]NaturalActorState, 16)
	copy(big, founders)
	balance, err = NaturalCheckConservation(patches, patchStates, slotStates, big, newborns, 2018)
	if err != nil {
		t.Fatalf("newborn cohort: %v", err)
	}
	if balance.Births != 2 || balance.InitialEnergy+2*NaturalNewbornEnergy != balance.Energy+balance.BasalSpent+balance.EnergyCapLost {
		t.Fatalf("newborn balance: %+v", balance)
	}
	// Allocator mismatch fails closed.
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, big, newborns, 2016); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("allocator mismatch admitted")
	}
	// Newborn IDs must be strictly ascending.
	reversed := []NaturalNewbornState{newborns[1], newborns[0]}
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, big, reversed, 2018); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("descending newborn IDs admitted")
	}
	// Duplicate newborn IDs.
	duplicate := []NaturalNewbornState{newborns[0], newborns[0]}
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, big, duplicate, 2017); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("duplicate newborn admitted")
	}
	// A newborn lineage violation: a parent at or after the newborn ID.
	badLineage := naturalForgedNewborn(t, patches, 2017, 1, 2018, 1001)
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, big, []NaturalNewbornState{badLineage, newborns[1]}, 2018); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("forward lineage admitted")
	}
	// A prohibited pair: full siblings (shared parents 1,2) reproducing
	// (kin depth 2).
	siblingsA := naturalScriptedNewborn(t, patches, 2017, 1, 2, 1001)
	siblingsB := naturalScriptedNewborn(t, patches, 2018, 1, 2, 1001)
	kinChild := naturalForgedNewborn(t, patches, 2019, 2017, 2018, 1001)
	big = append(big[:0], founders...)
	if _, err := NaturalCheckConservation(patches, patchStates, slotStates, big, []NaturalNewbornState{siblingsA, siblingsB, kinChild}, 2019); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("sibling parents admitted")
	}
}

// naturalScriptedNewborn builds a conserved newborn row with the given
// parents and home patch (energy 8, zero ledgers otherwise).
func naturalScriptedNewborn(t *testing.T, patches int, id, parentA, parentB sim.EntityID, home sim.EntityID) NaturalNewbornState {
	t.Helper()
	proposer, err := NaturalFounderState(patches, parentA, 140)
	if err != nil {
		t.Fatal(err)
	}
	consenter, err := NaturalFounderState(patches, parentB, 140)
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := NaturalChildGenome(patches, parentA, parentB, proposer.Genome, consenter.Genome, 0, []uint64{0, 0, 0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	state := NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: NaturalNeverHour},
		Bag:      NaturalBagState{Source: home},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   child,
		Request:  NaturalIdleRequestState(),
	}
	if !validNaturalActor(patches, id, state) {
		t.Fatalf("scripted newborn %d invalid", id)
	}
	return NaturalNewbornState{ID: id, State: state}
}

// TestNaturalBirthFlowRefusalsAndCapFreeAccepts pins the v4-verbatim birth
// machinery with the caps gone: the full acceptance at an arbitrary birth
// ordinal, the typed refusal taxonomy, expiry, age-death, and the heredity
// parity with v4.
func TestNaturalBirthFlowRefusalsAndCapFreeAccepts(t *testing.T) {
	patches := 2
	proposer := naturalGateParent(t, patches, 1)
	consenter := naturalGateParent(t, patches, 2)
	request, err := NaturalBirthRequest(patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	if request.Request.Status != NaturalRequestPending || request.Request.Hour != 5 || request.Request.Addressee != 2 {
		t.Fatalf("request row: %+v", request.Request)
	}
	// The ordinal is unbounded: a birth far past any former cap derives
	// its draws and its allocator ID the same way.
	farPosition, err := NaturalBirthDrawPosition(5000)
	if err != nil || farPosition != 35000 {
		t.Fatalf("far ordinal: %d %v", farPosition, err)
	}
	alloc, _ := NaturalGenesisAllocator(patches)
	for i := 0; i < 5000; i++ {
		alloc.Next()
	}
	farID, err := NaturalNextNewbornID(&alloc)
	if err != nil || farID != 7017 {
		t.Fatalf("far newborn ID: %d %v", farID, err)
	}
	farID2, err := NaturalNextNewbornID(&alloc)
	if err != nil || farID2 != 7018 {
		t.Fatalf("far sibling ID: %d %v", farID2, err)
	}
	draws := naturalDrawsForSeed(t, 0)
	result, err := NaturalBirthReply(patches, 5, true, draws, farID, 1, 2, request, consenter)
	if err != nil || result.Decision != NaturalReplyAccept {
		t.Fatalf("accept: %+v %v", result, err)
	}
	if result.Proposer.Granary.Stock != 3 || result.Proposer.Genome.BirthGranaryPaid != 2 || result.Proposer.Body.Energy != 3 || result.Proposer.Body.BasalSpent != 8 {
		t.Fatalf("proposer: %+v", result.Proposer)
	}
	if result.Newborn.Body.Energy != NaturalNewbornEnergy || result.Newborn.Genome.ParentA != 1 || result.Newborn.Genome.ParentB != 2 || result.Newborn.Bag.Source != 1001 {
		t.Fatalf("newborn: %+v", result.Newborn)
	}
	if !validNaturalActor(patches, 1, result.Proposer) || !validNaturalActor(patches, 2, result.Consenter) || !validNaturalActor(patches, farID, result.Newborn) {
		t.Fatal("invalid after birth")
	}
	// Heredity parity with v4 on identical draws.
	v4Child, v4Cross, err := ReproductionChildGenome(1, 2, mustV4Genome(t, 1), mustV4Genome(t, 2), 5, draws)
	if err != nil {
		t.Fatal(err)
	}
	v5Child, v5Cross, err := NaturalChildGenome(patches, 1, 2, naturalGateParent(t, patches, 1).Genome, naturalGateParent(t, patches, 2).Genome, 5, draws)
	if err != nil {
		t.Fatal(err)
	}
	if v4Child.LocusM != v5Child.LocusM || v4Child.LocusY != v5Child.LocusY || v4Child.LocusW != v5Child.LocusW ||
		v4Child.BirthHour != v5Child.BirthHour || v4Child.DeathHour != v5Child.DeathHour ||
		v4Cross.Lifetime != v5Cross.Lifetime || v4Cross.MutatedM != v5Cross.MutatedM || v4Cross.ClampedW != v5Cross.ClampedW {
		t.Fatalf("heredity drift: v4 %+v v5 %+v", v4Cross, v5Cross)
	}
	// Refusals, verbatim v4 taxonomy.
	var refusalErr *NaturalRefusalError
	// No matching pending request.
	_, err = NaturalBirthReply(patches, 5, true, draws, farID, 2, 1, consenter, proposer)
	if !errors.As(err, &refusalErr) || refusalErr.Reason != NaturalRefusalNotPending {
		t.Fatalf("not-pending: %v", err)
	}
	// Duplicate reply in one hour.
	repliedConsenter := consenter
	repliedConsenter.Request.LastReplyHour = 5
	_, err = NaturalBirthReply(patches, 5, true, draws, farID, 1, 2, request, repliedConsenter)
	if !errors.As(err, &refusalErr) || refusalErr.Reason != NaturalRefusalDuplicateReply {
		t.Fatalf("duplicate: %v", err)
	}
	// Consenter's own state below the consent margins.
	poorConsenter := consenter
	poorConsenter.Granary.Stock = 2
	poorConsenter.Granary.CultivatedTotal = 2
	pending := request
	result, err = NaturalBirthReply(patches, 5, true, draws, farID, 1, 2, pending, poorConsenter)
	if err != nil || result.Decision != NaturalReplyRefuse || result.Reason != NaturalRefusalConsentOwn {
		t.Fatalf("consent-own: %+v %v", result, err)
	}
	if result.Proposer.Request.Status != NaturalRequestRefused || result.Consenter.Request.LastReplyHour != 5 {
		t.Fatalf("refusal rows: %+v %+v", result.Proposer.Request, result.Consenter.Request)
	}
	// Reported claims below the margins.
	underClaimed := request
	underClaimed.Request.ReportedCapital = 1
	result, err = NaturalBirthReply(patches, 5, true, draws, farID, 1, 2, underClaimed, consenter)
	if err != nil || result.Reason != NaturalRefusalConsentClaims {
		t.Fatalf("claims: %+v %v", result, err)
	}
	// Kin prohibition.
	result, err = NaturalBirthReply(patches, 5, false, draws, farID, 1, 2, request, consenter)
	if err != nil || result.Reason != NaturalRefusalKin {
		t.Fatalf("kin: %+v %v", result, err)
	}
	// Cross-patch: the proposer is a gate-passing newborn whose carried
	// home is patch 1002, so the same-patch gate term fails for a
	// legitimate pair (a founder cannot move; a newborn can be born away).
	awayProposer := naturalScriptedNewborn(t, patches, farID2, 2, 3, 1002).State
	awayProposer.Worksite = NaturalWorksiteState{Capital: 2, PointsCreated: 2, InvestedUnits: 4, LastBuildHour: NaturalNeverHour}
	awayProposer.Granary = NaturalGranaryState{Stock: 3, CultivatedTotal: 3, LastStoredMealHour: NaturalNeverHour}
	awayProposer.Body = NaturalBodyState{Energy: 3, BasalSpent: 5, LastGatherHour: NaturalNeverHour}
	if !validNaturalActor(patches, farID2, awayProposer) {
		t.Fatalf("away proposer invalid: %+v", awayProposer)
	}
	awayRequest, err := NaturalBirthRequest(patches, 5, farID2, 1, 3, 5, awayProposer)
	if err != nil {
		t.Fatal(err)
	}
	result, err = NaturalBirthReply(patches, 5, true, draws, farID, farID2, 1, awayRequest, consenter)
	if err != nil || result.Reason != NaturalRefusalCrossPatch {
		t.Fatalf("cross-patch: %+v %v", result, err)
	}
	// Fresh-truth gate thresholds: the proposer's truth fails the gate
	// while every consent-formula margin still passes.
	tired := request
	tired.Body.Energy = 2
	tired.Body.BasalSpent = 9
	result, err = NaturalBirthReply(patches, 5, true, draws, farID, 1, 2, tired, consenter)
	if err != nil || result.Reason != NaturalRefusalGateThresholds {
		t.Fatalf("thresholds: %+v %v", result, err)
	}
	// Death between phases is the silent zero-mutation refusal.
	deadConsenter := consenter
	deadConsenter.Body.Energy = 0
	deadConsenter.Body.BasalSpent = NaturalFounderInitialEnergy
	deadConsenter.Genome.DiedHour = 4
	result, err = NaturalBirthReply(patches, 5, true, draws, farID, 1, 2, request, deadConsenter)
	if err != nil || result.Decision != NaturalReplyRefuseSilent || result.Reason != NaturalRefusalAddresseeDead {
		t.Fatalf("silent: %+v %v", result, err)
	}
	if reflect.DeepEqual(result.Consenter.Request, consenter.Request) == false {
		t.Fatal("silent refusal mutated the request row")
	}
	// Expiry at the next pulse, verbatim v4.
	expired, err := NaturalBirthExpiry(6, request.Request)
	if err != nil || expired.Status != NaturalRequestExpired {
		t.Fatalf("expiry: %+v %v", expired, err)
	}
	if _, err := NaturalBirthExpiry(7, request.Request); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("expiry of a non-pending request admitted")
	}
	// Age-death before and at the immutable death-hour.
	genome := mustNaturalGenome(t, patches, 1, 120)
	unchanged, died, err := NaturalAgeDeath(patches, 119, 1, genome)
	if err != nil || died || unchanged.DiedHour != NaturalNeverHour {
		t.Fatalf("pre-death: %+v %v", unchanged, err)
	}
	diedGenome, died, err := NaturalAgeDeath(patches, 120, 1, genome)
	if err != nil || !died || diedGenome.DiedHour != 120 {
		t.Fatalf("death: %+v %v %v", diedGenome, died, err)
	}
	if _, _, err := NaturalAgeDeath(patches, 121, 1, diedGenome); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("second death admitted")
	}
	// Kinship parity with v4.
	lookup := NaturalParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		switch id {
		case 2017:
			return 1, 2, true
		case 2018:
			return 1, 2, true
		case 2019:
			return 2017, 2018, true
		}
		return 0, 0, false
	})
	v4Lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		a, b, ok := lookup(id)
		return a, b, ok
	})
	for _, pair := range [][2]sim.EntityID{{1, 2}, {1, 2017}, {2017, 2018}, {2017, 2019}, {3, 2019}} {
		v4Distance, v4Related := ReproductionKinDistance(v4Lookup, pair[0], pair[1])
		v5Distance, v5Related := NaturalKinDistance(lookup, pair[0], pair[1])
		if v4Distance != v5Distance || v4Related != v5Related {
			t.Fatalf("kin %v: v4 (%d,%v) v5 (%d,%v)", pair, v4Distance, v4Related, v5Distance, v5Related)
		}
	}
	if distance, related := NaturalKinDistance(lookup, 2017, 2018); !related || distance != 2 || !NaturalKinProhibited(distance, related) {
		t.Fatalf("siblings: %d %v", distance, related)
	}
	if distance, related := NaturalKinDistance(lookup, 1, 2019); !related || distance != 2 {
		t.Fatalf("grandparent: %d %v", distance, related)
	}
	// Consent formula parity with v4.
	for _, tc := range []struct{ g, k, e, rk, rg int64 }{
		{3, 2, 3, 2, 3}, {2, 2, 3, 2, 3}, {3, 1, 3, 2, 3}, {3, 2, 2, 2, 3}, {3, 2, 3, 1, 3}, {3, 2, 3, 2, 2},
	} {
		if ReproductionConsentAllows(tc.g, tc.k, tc.e, tc.rk, tc.rg, true) != NaturalConsentAllows(tc.g, tc.k, tc.e, tc.rk, tc.rg, true) {
			t.Fatalf("consent drift: %+v", tc)
		}
	}
	// The surplus gate carries no birth-count term: the same ecology gate
	// holds regardless of how many births happened.
	if !NaturalSurplusGateAllows(naturalGateParent(t, patches, 1), naturalGateParent(t, patches, 2), true) {
		t.Fatal("gate refused an eligible pair")
	}
}

func mustV4Genome(t *testing.T, actor sim.EntityID) ReproductionGenomeState {
	t.Helper()
	genome, err := ReproductionFounderGenome(actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	return genome
}

// naturalBuildRefusalProbe rebuilds the pristine gate fixture so the
// refusal zero-mutation assertions compare against a fresh copy.
func naturalBuildRefusalProbe(t *testing.T, patches int) NaturalActorState {
	t.Helper()
	probe, err := NaturalFounderState(patches, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	probe.Worksite = NaturalWorksiteState{Capital: 1, Wip: 1, PointsCreated: 1, InvestedUnits: 3, LastBuildHour: NaturalNeverHour}
	probe.Bag.Units = 1
	probe.Body.LastGatherHour = 4
	if !validNaturalActor(patches, 1, probe) {
		t.Fatal("probe fixture invalid")
	}
	return probe
}

// naturalForgedNewborn builds a complete newborn row whose genome carries
// the given parent references (used only for negative conservation
// fixtures; the state is otherwise the neutral newborn shape).
func naturalForgedNewborn(t *testing.T, patches int, id, parentA, parentB sim.EntityID, home sim.EntityID) NaturalNewbornState {
	t.Helper()
	state := NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: NaturalNeverHour},
		Bag:      NaturalBagState{Source: home},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   NaturalGenomeState{LocusM: 6, LocusY: 6, LocusW: 6, ParentA: parentA, ParentB: parentB, BirthHour: 0, DeathHour: 96, DiedHour: NaturalNeverHour},
		Request:  NaturalIdleRequestState(),
	}
	return NaturalNewbornState{ID: id, State: state}
}
