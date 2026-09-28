package world

import (
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"slices"
	"testing"
)

func foodFlowAllocationClaims(patch int) []FoodFlowGatherClaim {
	claims := make([]FoodFlowGatherClaim, FoodFlowSlotsPerPatch)
	patchID, _ := FoodFlowPatchID(patch)
	slotID, _ := FoodFlowSlotID(patch, 0)
	for i := range claims {
		claims[i] = FoodFlowGatherClaim{
			Actor: sim.EntityID(1 + patch*FoodFlowSlotsPerPatch + i), TargetPatch: patchID,
			TargetSlot: slotID, Energy: FoodFlowInitialEnergy, LastGatherHour: FoodFlowNeverGatheredHour,
		}
	}
	return claims
}

func foodFlowAllocationStock(patch, count int) []FoodFlowStockedSlot {
	stock := make([]FoodFlowStockedSlot, count)
	patchID, _ := FoodFlowPatchID(patch)
	for i := range stock {
		id, _ := FoodFlowSlotID(patch, i)
		stock[i] = FoodFlowStockedSlot{ID: id, Patch: patchID}
	}
	return stock
}

func foodFlowCheckInjective(t *testing.T, got []FoodFlowGatherAdmission, want int) {
	t.Helper()
	seen := make(map[sim.EntityID]bool)
	for _, result := range got {
		if result.Rejection != FoodFlowGatherAdmitted {
			if result.Slot != 0 {
				t.Fatalf("rejected claim assigned slot: %+v", result)
			}
			continue
		}
		if result.Slot == 0 || seen[result.Slot] {
			t.Fatalf("zero/duplicate slot: %+v in %+v", result, got)
		}
		seen[result.Slot] = true
	}
	if len(seen) != want {
		t.Fatalf("admitted %d, want %d: %+v", len(seen), want, got)
	}
}

func TestFoodFlowAllocateGathersEightOfEight(t *testing.T) {
	for _, seed := range []uint64{0, 1, 7, 918271} {
		stock := foodFlowAllocationStock(0, 8)
		claims := foodFlowAllocationClaims(0) // all choose the same low-ID affordance
		originalStock := slices.Clone(stock)
		originalClaims := slices.Clone(claims)
		got, err := FoodFlowAllocateGathers(0, seed, stock, claims)
		if err != nil {
			t.Fatal(err)
		}
		foodFlowCheckInjective(t, got, 8)
		if !reflect.DeepEqual(stock, originalStock) || !reflect.DeepEqual(claims, originalClaims) {
			t.Fatal("allocation mutated snapshot or intentions")
		}
		for _, admission := range got {
			if admission.Rejection != FoodFlowGatherAdmitted || admission.Patch != 1001 {
				t.Fatalf("full stock rejected a claimant: %+v", admission)
			}
		}
	}
}

func TestFoodFlowAllocateGathersFullStockDoesNotPinLowActorsToLowSlots(t *testing.T) {
	seen := make(map[sim.EntityID]bool)
	for hour := 0; hour < FoodFlowSlotsPerPatch; hour++ {
		got, err := FoodFlowAllocateGathers(hour, 7, foodFlowAllocationStock(0, 8), foodFlowAllocationClaims(0))
		if err != nil {
			t.Fatal(err)
		}
		foodFlowCheckInjective(t, got, 8)
		seen[got[0].Slot] = true
	}
	if len(seen) != FoodFlowSlotsPerPatch {
		t.Fatalf("actor 1 stuck on low slot IDs: %+v", seen)
	}
}

func TestFoodFlowAllocateGathersScarceEightHourRotation(t *testing.T) {
	for _, seed := range []uint64{0, 1, 7, 918271} {
		for patch := 0; patch < FoodFlowPatchCount; patch++ {
			var wins [FoodFlowSlotsPerPatch]int
			var firstHour [FoodFlowSlotsPerPatch]bool
			for h := 0; h < 8; h++ {
				got, err := FoodFlowAllocateGathers(h, seed, foodFlowAllocationStock(patch, 3), foodFlowAllocationClaims(patch))
				if err != nil {
					t.Fatal(err)
				}
				foodFlowCheckInjective(t, got, 3)
				for _, result := range got {
					index := int(result.Actor-1) % FoodFlowSlotsPerPatch
					if result.Rejection == FoodFlowGatherAdmitted {
						wins[index]++
						if h == 0 {
							firstHour[index] = true
						}
					} else if result.Rejection != FoodFlowGatherCapacity {
						t.Fatalf("seed=%d patch=%d hour=%d unexpected rejection: %+v", seed, patch, h, result)
					}
				}
			}
			for i, won := range wins {
				if won != 3 {
					t.Fatalf("seed=%d patch=%d actor index=%d wins=%d, want 3", seed, patch, i, won)
				}
			}
			if seed == 7 && patch == 0 && firstHour[0] && firstHour[1] && firstHour[2] {
				t.Fatal("first hour still preferentially admits the three lowest actor IDs")
			}
		}
	}
}

func TestFoodFlowAllocateGathersFirstHourSeedOffset(t *testing.T) {
	for _, tc := range []struct {
		seed  uint64
		patch int
		want  []sim.EntityID
	}{
		{0, 0, []sim.EntityID{1, 2, 3}},
		{0, 1, []sim.EntityID{10, 11, 12}},
		{7, 0, []sim.EntityID{1, 2, 8}},
		{7, 1, []sim.EntityID{9, 10, 11}},
	} {
		got, err := FoodFlowAllocateGathers(0, tc.seed, foodFlowAllocationStock(tc.patch, 3), foodFlowAllocationClaims(tc.patch))
		if err != nil {
			t.Fatal(err)
		}
		var winners []sim.EntityID
		for _, result := range got {
			if result.Rejection == FoodFlowGatherAdmitted {
				winners = append(winners, result.Actor)
			}
		}
		if !reflect.DeepEqual(winners, tc.want) {
			t.Fatalf("seed=%d patch=%d winners=%v, want %v", tc.seed, tc.patch, winners, tc.want)
		}
	}
}

func TestFoodFlowAllocateGathersNonPrefixStockRematchesUnstockedTarget(t *testing.T) {
	claims := append(foodFlowAllocationClaims(0), foodFlowAllocationClaims(1)...)
	stock := []FoodFlowStockedSlot{
		{ID: 2008, Patch: 1001}, {ID: 2016, Patch: 1002},
		{ID: 2004, Patch: 1001}, {ID: 2012, Patch: 1002},
	}
	got, err := FoodFlowAllocateGathers(0, 0, stock, claims)
	if err != nil {
		t.Fatal(err)
	}
	foodFlowCheckInjective(t, got, 4)
	remaining := make(map[sim.EntityID]bool)
	for _, slot := range stock {
		remaining[slot.ID] = true
	}
	for _, result := range got {
		if result.Rejection == FoodFlowGatherAdmitted {
			if !remaining[result.Slot] {
				t.Fatalf("assigned a slot absent from the stock snapshot: %+v", result)
			}
			delete(remaining, result.Slot)
		} else if result.Rejection != FoodFlowGatherCapacity {
			t.Fatalf("unstocked preferred slot should not block rematching: %+v", result)
		}
	}
	if len(remaining) != 0 {
		t.Fatalf("stock unused despite eligible claims: %+v", remaining)
	}
}

func TestFoodFlowAllocateGathersSubsetAndCompetingPatches(t *testing.T) {
	left, right := foodFlowAllocationClaims(0), foodFlowAllocationClaims(1)
	claims := []FoodFlowGatherClaim{left[7], right[5], left[0], right[0], left[3], right[7]}
	stock := append(foodFlowAllocationStock(0, 2), foodFlowAllocationStock(1, 1)...)
	for _, seed := range []uint64{0, 4, 32767} {
		got, err := FoodFlowAllocateGathers(13, seed, stock, claims)
		if err != nil {
			t.Fatal(err)
		}
		foodFlowCheckInjective(t, got, 3)
		var accepted [FoodFlowPatchCount]int
		for _, result := range got {
			if result.Rejection == FoodFlowGatherAdmitted {
				accepted[int(result.Patch-1001)]++
				if int(result.Slot-2001)/FoodFlowSlotsPerPatch != int(result.Patch-1001) {
					t.Fatalf("cross-patch assignment: %+v", result)
				}
			} else if result.Rejection != FoodFlowGatherCapacity {
				t.Fatalf("unexpected result %+v", result)
			}
		}
		if accepted != [FoodFlowPatchCount]int{2, 1} {
			t.Fatalf("capacity not allocated per patch: %+v", accepted)
		}
	}
}

func TestFoodFlowAllocateGathersStockAccessAndEligibility(t *testing.T) {
	claims := foodFlowAllocationClaims(0)
	claims[1].Energy = 0
	claims[2].BagUnits = 1
	claims[3].LastGatherHour = 5
	claims[4].TargetPatch = 1002
	claims[5].TargetSlot = 2009
	claims[6].Actor = 17
	claims[7].LastGatherHour = -2
	for _, tc := range []struct {
		name  string
		stock []FoodFlowStockedSlot
		first FoodFlowGatherRejection
	}{
		{"empty", nil, FoodFlowGatherNoStock},
		{"stocked", foodFlowAllocationStock(0, 1), FoodFlowGatherAdmitted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FoodFlowAllocateGathers(5, 0, tc.stock, claims)
			if err != nil {
				t.Fatal(err)
			}
			foodFlowCheckInjective(t, got, len(tc.stock))
			want := []FoodFlowGatherRejection{tc.first, FoodFlowGatherIneligible, FoodFlowGatherIneligible, FoodFlowGatherIneligible, FoodFlowGatherNoAccess, FoodFlowGatherNoAccess, FoodFlowGatherIneligible, FoodFlowGatherNoAccess}
			for i, result := range got {
				if result.Rejection != want[i] {
					t.Fatalf("actor=%d reason=%v, want %v", result.Actor, result.Rejection, want[i])
				}
			}
		})
	}
}

func TestFoodFlowAllocateGathersDuplicateActorRejectsAll(t *testing.T) {
	claims := foodFlowAllocationClaims(0)[:2]
	claims = append(claims, claims[0])
	claims[2].TargetSlot = 2002
	got, err := FoodFlowAllocateGathers(0, 0, foodFlowAllocationStock(0, 2), claims)
	if err != nil {
		t.Fatal(err)
	}
	foodFlowCheckInjective(t, got, 1)
	if got[0].Rejection != FoodFlowGatherDuplicateActor || got[1].Rejection != FoodFlowGatherDuplicateActor || got[2].Actor != 2 || got[2].Rejection != FoodFlowGatherAdmitted {
		t.Fatalf("duplicate actor claimed a slot: %+v", got)
	}
}

func TestFoodFlowAllocateGathersOrderIndependent(t *testing.T) {
	claims := append(foodFlowAllocationClaims(0), foodFlowAllocationClaims(1)...)
	stock := append(foodFlowAllocationStock(0, 3), foodFlowAllocationStock(1, 7)...)
	for _, seed := range []uint64{0, 2, 7, 13579} {
		for _, hour := range []int{0, 1, 7, 36, 167} {
			want, err := FoodFlowAllocateGathers(hour, seed, stock, claims)
			if err != nil {
				t.Fatal(err)
			}
			foodFlowCheckInjective(t, want, 10)
			reverseClaims, reverseStock := slices.Clone(claims), slices.Clone(stock)
			slices.Reverse(reverseClaims)
			slices.Reverse(reverseStock)
			got, err := FoodFlowAllocateGathers(hour, seed, reverseStock, reverseClaims)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("reversed input differs: seed=%d hour=%d got=%+v want=%+v err=%v", seed, hour, got, want, err)
			}
			claimMap := make(map[sim.EntityID]FoodFlowGatherClaim)
			stockMap := make(map[sim.EntityID]FoodFlowStockedSlot)
			for _, claim := range claims {
				claimMap[claim.Actor] = claim
			}
			for _, slot := range stock {
				stockMap[slot.ID] = slot
			}
			var mappedClaims []FoodFlowGatherClaim
			var mappedStock []FoodFlowStockedSlot
			for _, claim := range claimMap {
				mappedClaims = append(mappedClaims, claim)
			}
			for _, slot := range stockMap {
				mappedStock = append(mappedStock, slot)
			}
			got, err = FoodFlowAllocateGathers(hour, seed, mappedStock, mappedClaims)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("map iteration differs: seed=%d hour=%d got=%+v want=%+v err=%v", seed, hour, got, want, err)
			}
		}
	}
}

func TestFoodFlowAllocateGathersBatchLimitBeforeValidation(t *testing.T) {
	claims := append(foodFlowAllocationClaims(0), foodFlowAllocationClaims(1)...)
	stock := append(foodFlowAllocationStock(0, 8), foodFlowAllocationStock(1, 8)...)
	got, err := FoodFlowAllocateGathers(0, 0, stock, claims)
	if err != nil {
		t.Fatal(err)
	}
	foodFlowCheckInjective(t, got, 16) // the fixture's full inventory and claim batch are valid

	for _, tc := range []struct {
		name   string
		stock  []FoodFlowStockedSlot
		claims []FoodFlowGatherClaim
	}{
		{"stock", append(slices.Clone(stock), FoodFlowStockedSlot{}), claims},
		{"claims", stock, append(slices.Clone(claims), FoodFlowGatherClaim{})},
		{"both", append(slices.Clone(stock), FoodFlowStockedSlot{}), append(slices.Clone(claims), FoodFlowGatherClaim{})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The extra element is itself invalid; a limit error instead of an
			// invalid-snapshot result proves the bound precedes inspection.
			got, err := FoodFlowAllocateGathers(0, 0, tc.stock, tc.claims)
			if got != nil || !errors.Is(err, ErrFoodFlowAllocationLimit) {
				t.Fatalf("oversized batch: got=%+v err=%v", got, err)
			}
		})
	}
}

func TestFoodFlowAllocateGathersRejectsInvalidSnapshotAndHour(t *testing.T) {
	stock := foodFlowAllocationStock(0, 1)
	claims := foodFlowAllocationClaims(0)[:1]
	for _, hour := range []int{-1, FoodFlowHorizonHours} {
		if _, err := FoodFlowAllocateGathers(hour, 0, stock, claims); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("hour=%d: %v", hour, err)
		}
	}
	for _, invalid := range [][]FoodFlowStockedSlot{
		append(slices.Clone(stock), stock[0]),
		{{ID: 2009, Patch: 1001}},
		{{ID: 9999, Patch: 1001}},
		{{ID: 2001, Patch: 9999}},
	} {
		if _, err := FoodFlowAllocateGathers(0, 0, invalid, claims); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("invalid snapshot %+v: %v", invalid, err)
		}
	}
}
