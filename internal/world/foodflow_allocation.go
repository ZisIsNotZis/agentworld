package world

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
)

var ErrFoodFlowAllocationLimit = errors.New("food-flow allocation batch exceeds fixture bounds")

// FoodFlowStockedSlot is one unit of stock in the authoritative common
// snapshot. An absent slot has no available stock; Patch must match its fixed
// slot identity, not a claimant's observation.
type FoodFlowStockedSlot struct {
	ID, Patch sim.EntityID
}

// FoodFlowGatherClaim carries a strategy Gather intention and the actor's
// authoritative eligibility fields from that same snapshot. TargetSlot is a
// patch-local affordance, not a reservation: competing claims can be matched
// to any stocked slot in their accessible patch.
type FoodFlowGatherClaim struct {
	Actor, TargetPatch, TargetSlot   sim.EntityID
	Energy, BagUnits, LastGatherHour int64
}

type FoodFlowGatherRejection uint8

const (
	FoodFlowGatherAdmitted FoodFlowGatherRejection = iota
	FoodFlowGatherNoStock
	FoodFlowGatherCapacity
	FoodFlowGatherIneligible
	FoodFlowGatherNoAccess
	FoodFlowGatherDuplicateActor
)

type FoodFlowGatherAdmission struct {
	Actor, Patch, Slot sim.EntityID // Slot is zero on rejection.
	Rejection          FoodFlowGatherRejection
}

// FoodFlowAllocateGathers is pure: all claims and stock are from one common
// snapshot at authoritative hour. Result order is actor/patch/target-slot
// order, independent of worker completion, input order and map iteration.
// When stock exists, eligible actors compete in rotating seed-offset home-ID
// order, not proposal-key order; each admitted actor gets a distinct slot.
// No hourly-yield (q) cap is imposed on gathering existing stock.
func FoodFlowAllocateGathers(hour int, seed uint64, stocked []FoodFlowStockedSlot, claims []FoodFlowGatherClaim) ([]FoodFlowGatherAdmission, error) {
	if len(stocked) > FoodFlowPatchCount*FoodFlowSlotsPerPatch || len(claims) > FoodFlowActorCount {
		return nil, ErrFoodFlowAllocationLimit
	}
	if _, err := FoodFlowClaimTime(hour); err != nil {
		return nil, ErrFoodFlowContract
	}
	var slots [FoodFlowPatchCount][]sim.EntityID
	seenSlots := make(map[sim.EntityID]bool, len(stocked))
	for _, slot := range stocked {
		index := int(slot.Patch) - 1001
		if index < 0 || index >= FoodFlowPatchCount || seenSlots[slot.ID] || !foodFlowSlotBelongsToPatch(slot.ID, index) {
			return nil, ErrFoodFlowContract
		}
		seenSlots[slot.ID] = true
		slots[index] = append(slots[index], slot.ID)
	}
	for i := range slots {
		sort.Slice(slots[i], func(a, b int) bool { return slots[i][a] < slots[i][b] })
	}
	ordered := append([]FoodFlowGatherClaim(nil), claims...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.Actor != b.Actor {
			return a.Actor < b.Actor
		}
		if a.TargetPatch != b.TargetPatch {
			return a.TargetPatch < b.TargetPatch
		}
		if a.TargetSlot != b.TargetSlot {
			return a.TargetSlot < b.TargetSlot
		}
		if a.Energy != b.Energy {
			return a.Energy < b.Energy
		}
		if a.BagUnits != b.BagUnits {
			return a.BagUnits < b.BagUnits
		}
		return a.LastGatherHour < b.LastGatherHour
	})
	out := make([]FoodFlowGatherAdmission, len(ordered))
	var eligible [FoodFlowPatchCount][]int
	for i, claim := range ordered {
		out[i] = FoodFlowGatherAdmission{Actor: claim.Actor, Patch: claim.TargetPatch}
		// Reject *all* duplicate claims, including inconsistent copies, so an
		// actor cannot gain priority by submitting more than one intention.
		if (i > 0 && ordered[i-1].Actor == claim.Actor) || (i+1 < len(ordered) && ordered[i+1].Actor == claim.Actor) {
			out[i].Rejection = FoodFlowGatherDuplicateActor
			continue
		}
		patch, err := FoodFlowActorPatchID(claim.Actor)
		index := int(claim.TargetPatch) - 1001
		if err != nil || patch != claim.TargetPatch || index < 0 || index >= FoodFlowPatchCount || !foodFlowSlotBelongsToPatch(claim.TargetSlot, index) {
			out[i].Rejection = FoodFlowGatherNoAccess
			continue
		}
		if claim.Energy <= 0 || claim.Energy > FoodFlowEnergyCapacity || claim.BagUnits != 0 || claim.LastGatherHour < FoodFlowNeverGatheredHour || claim.LastGatherHour >= int64(hour) {
			out[i].Rejection = FoodFlowGatherIneligible
			continue
		}
		eligible[index] = append(eligible[index], i)
	}
	for patch := range eligible {
		// Home IDs are contiguous within each patch. Rotating over all eight
		// home positions (not just present claimants) also handles subsets.
		start := (seed + uint64(hour) + uint64(patch)) % FoodFlowSlotsPerPatch
		sort.Slice(eligible[patch], func(a, b int) bool {
			left := (uint64((ordered[eligible[patch][a]].Actor-1)%FoodFlowSlotsPerPatch) + FoodFlowSlotsPerPatch - start) % FoodFlowSlotsPerPatch
			right := (uint64((ordered[eligible[patch][b]].Actor-1)%FoodFlowSlotsPerPatch) + FoodFlowSlotsPerPatch - start) % FoodFlowSlotsPerPatch
			return left < right
		})
		available := slots[patch]
		for rank, i := range eligible[patch] {
			switch {
			case len(available) == 0:
				out[i].Rejection = FoodFlowGatherNoStock
			case rank >= len(available):
				out[i].Rejection = FoodFlowGatherCapacity
			default:
				// Rotate the available slot order too; no preference among
				// equally stocked IDs is introduced by the allocator.
				position := (int((seed+2*uint64(hour)+uint64(patch))%uint64(len(available))) + rank) % len(available)
				out[i].Slot = available[position]
			}
		}
	}
	return out, nil
}

func foodFlowSlotBelongsToPatch(slot sim.EntityID, patchIndex int) bool {
	first := sim.EntityID(2001 + patchIndex*FoodFlowSlotsPerPatch)
	return slot >= first && slot < first+FoodFlowSlotsPerPatch
}
