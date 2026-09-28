package world

import "agentworld/internal/sim"

// FoodFlowPatchState has only the one-writer production ledger. Gathered food
// is accounted by independent slot counters, and consumed food by each actor's
// home-patch counter. No actor proposal writes the shared patch component.
type FoodFlowPatchState struct{ Yield, Pulses, Produced, Unrealized int64 }
type FoodFlowSlotState struct{ Stock, Gathered int64 }
type FoodFlowActorState struct {
	Bag                                           int64
	BagSource                                     sim.EntityID // zero iff empty; stored as Missing in the bag
	Energy, Hunger, BasalSpent, CapLost, Consumed int64        // Consumed is from the actor's fixed home patch
	LastGatherHour                                int64        // -1 until first successful Gather; stored in the body
}
type FoodFlowProduction struct{ Produced, Unrealized int64 }
type FoodFlowConsumption struct{ EnergyGained, EnergyCapLost int64 }
type FoodFlowBasalCost struct{ EnergySpent, UnmetEnergy int64 }
type FoodFlowBalance struct {
	Produced, Unrealized, Gathered, Consumed, Stock, Held int64
	InitialEnergy, Energy, BasalSpent, EnergyCapLost      int64
}

func validFoodFlowPatch(p FoodFlowPatchState) bool {
	max := int64(FoodFlowHorizonHours * FoodFlowSlotsPerPatch)
	return p.Yield >= 0 && p.Yield <= FoodFlowSlotsPerPatch && p.Pulses >= 0 && p.Pulses <= FoodFlowHorizonHours &&
		p.Produced >= 0 && p.Produced <= max && p.Unrealized >= 0 && p.Unrealized <= max &&
		p.Produced+p.Unrealized == p.Yield*p.Pulses
}
func validFoodFlowSlot(s FoodFlowSlotState) bool {
	return s.Stock >= 0 && s.Stock <= 1 && s.Gathered >= 0 && s.Gathered <= FoodFlowHorizonHours
}
func validFoodFlowActor(a FoodFlowActorState) bool {
	if a.Bag < 0 || a.Bag > FoodFlowBagCapacity || (a.Bag == 0) != (a.BagSource == 0) ||
		a.Energy < 0 || a.Energy > FoodFlowEnergyCapacity || a.Hunger < 0 || a.Hunger > FoodFlowHungerCapacity ||
		a.BasalSpent < 0 || a.BasalSpent > FoodFlowHorizonHours || a.CapLost < 0 || a.CapLost > FoodFlowHorizonHours ||
		a.Consumed < 0 || a.Consumed > FoodFlowHorizonHours ||
		a.LastGatherHour < FoodFlowNeverGatheredHour || a.LastGatherHour >= FoodFlowHorizonHours {
		return false
	}
	if a.Bag != 0 && a.BagSource != 1001 && a.BagSource != 1002 {
		return false
	}
	if a.LastGatherHour == FoodFlowNeverGatheredHour && (a.Bag != 0 || a.Consumed != 0) {
		return false
	}
	// BasalSpent counts only energy actually spent, not unmet need after death.
	return FoodFlowInitialEnergy+a.Consumed == a.Energy+a.BasalSpent+a.CapLost
}
func foodFlowStock(p FoodFlowPatchState, slots [FoodFlowSlotsPerPatch]FoodFlowSlotState) bool {
	if !validFoodFlowPatch(p) {
		return false
	}
	var total int64
	for _, slot := range slots {
		if !validFoodFlowSlot(slot) {
			return false
		}
		if slot.Gathered+slot.Stock > p.Pulses {
			return false
		}
		total += slot.Gathered + slot.Stock
	}
	return total == p.Produced // initial slots are empty
}

// FoodFlowProduce fills vacant slots in ascending order. h=0..167 are the
// production hours; full slots block inflow, accounted only as unrealized.
// The runner commits the patch ledger and changed slot stocks in one event.
func FoodFlowProduce(p FoodFlowPatchState, slots [FoodFlowSlotsPerPatch]FoodFlowSlotState, h int) (FoodFlowPatchState, [FoodFlowSlotsPerPatch]FoodFlowSlotState, FoodFlowProduction, error) {
	if h < 0 || h >= FoodFlowHorizonHours || int64(h) != p.Pulses || !foodFlowStock(p, slots) {
		return FoodFlowPatchState{}, slots, FoodFlowProduction{}, ErrFoodFlowContract
	}
	out := slots
	var produced int64
	for i := range out {
		if out[i].Stock == 0 && produced < p.Yield {
			out[i].Stock, produced = 1, produced+1
		}
	}
	result := FoodFlowProduction{Produced: produced, Unrealized: p.Yield - produced}
	p.Pulses++
	p.Produced += produced
	p.Unrealized += result.Unrealized
	if !foodFlowStock(p, out) {
		return FoodFlowPatchState{}, slots, FoodFlowProduction{}, ErrFoodFlowContract
	}
	return p, out, result, nil
}

// FoodFlowGather transfers one identified slot unit to its home-patch actor's
// bag. The runner commits slot stock/gathered and bag/source in one actor event,
// with at most one successful gather per actor per hour. The caller reads
// LastGatherHour from the kernel snapshot and patches it atomically with the
// slot and bag. Distinct actors/slots touch no shared patch fields.
func FoodFlowGather(patchID, slotID, actorID sim.EntityID, hour int, slot FoodFlowSlotState, a FoodFlowActorState) (FoodFlowSlotState, FoodFlowActorState, error) {
	owner, err := FoodFlowActorPatchID(actorID)
	if err != nil || owner != patchID || hour < 0 || hour >= FoodFlowHorizonHours || !validFoodFlowSlot(slot) || slot.Stock != 1 || slot.Gathered > int64(hour) || !validFoodFlowActor(a) || a.LastGatherHour >= int64(hour) || a.Energy == 0 || a.Bag != 0 {
		return FoodFlowSlotState{}, FoodFlowActorState{}, ErrFoodFlowContract
	}
	index := int(patchID - 1001)
	if slotID < sim.EntityID(2001+index*FoodFlowSlotsPerPatch) || slotID >= sim.EntityID(2001+(index+1)*FoodFlowSlotsPerPatch) {
		return FoodFlowSlotState{}, FoodFlowActorState{}, ErrFoodFlowContract
	}
	slot.Stock, slot.Gathered = 0, slot.Gathered+1
	a.Bag, a.BagSource, a.LastGatherHour = 1, patchID, int64(hour)
	return slot, a, nil
}

// FoodFlowConsume removes a held unit and increments that actor's home-patch
// consumption counter. A bag cannot come from another patch. Energy spill at
// capacity is counted, while hunger relief remains independent of energy.
func FoodFlowConsume(patchID, actorID sim.EntityID, a FoodFlowActorState) (FoodFlowActorState, FoodFlowConsumption, error) {
	owner, err := FoodFlowActorPatchID(actorID)
	if err != nil || owner != patchID || !validFoodFlowActor(a) || a.Energy == 0 || a.Bag != 1 || a.BagSource != patchID || a.Consumed >= FoodFlowHorizonHours {
		return FoodFlowActorState{}, FoodFlowConsumption{}, ErrFoodFlowContract
	}
	a.Bag, a.BagSource = 0, 0
	a.Consumed++
	result := FoodFlowConsumption{EnergyGained: 1}
	if a.Energy == FoodFlowEnergyCapacity {
		a.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Energy++
	}
	if a.Hunger > 0 {
		a.Hunger--
	}
	if !validFoodFlowActor(a) {
		return FoodFlowActorState{}, FoodFlowConsumption{}, ErrFoodFlowContract
	}
	return a, result, nil
}

// FoodFlowBasal advances elapsed whole hours even after rejected intentions.
// The runner invokes it at integer h=1..168, before any h<=167 claims.
func FoodFlowBasal(a FoodFlowActorState, elapsedHours int64) (FoodFlowActorState, FoodFlowBasalCost, error) {
	if !validFoodFlowActor(a) || elapsedHours < 1 || elapsedHours > FoodFlowHorizonHours || a.BasalSpent > FoodFlowHorizonHours-elapsedHours {
		return FoodFlowActorState{}, FoodFlowBasalCost{}, ErrFoodFlowContract
	}
	spent := elapsedHours
	if spent > a.Energy {
		spent = a.Energy
	}
	result := FoodFlowBasalCost{EnergySpent: spent, UnmetEnergy: elapsedHours - spent}
	a.Energy -= spent
	a.BasalSpent += spent
	if elapsedHours >= FoodFlowHungerCapacity-a.Hunger {
		a.Hunger = FoodFlowHungerCapacity
	} else {
		a.Hunger += elapsedHours
	}
	if !validFoodFlowActor(a) {
		return FoodFlowActorState{}, FoodFlowBasalCost{}, ErrFoodFlowContract
	}
	return a, result, nil
}

// FoodFlowCheckConservation aggregates patch totals from slot and home-actor
// counters. It checks each patch and actor, then global mass/energy identities:
// produced - consumed = stock + held (no food loss in v1), and
// initial energy + consumed = energy + basal spent + cap loss.
func FoodFlowCheckConservation(patches [FoodFlowPatchCount]FoodFlowPatchState, slots [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState, actors [FoodFlowActorCount]FoodFlowActorState) (FoodFlowBalance, error) {
	var b FoodFlowBalance
	b.InitialEnergy = FoodFlowInitialEnergy * FoodFlowActorCount
	for i, p := range patches {
		if !foodFlowStock(p, slots[i]) {
			return FoodFlowBalance{}, ErrFoodFlowContract
		}
		b.Produced += p.Produced
		b.Unrealized += p.Unrealized
		var gathered, consumed, held int64
		for _, slot := range slots[i] {
			gathered += slot.Gathered
			b.Stock += slot.Stock
		}
		for j := i * FoodFlowSlotsPerPatch; j < (i+1)*FoodFlowSlotsPerPatch; j++ {
			a := actors[j]
			if !validFoodFlowActor(a) || (a.Bag != 0 && a.BagSource != sim.EntityID(1001+i)) {
				return FoodFlowBalance{}, ErrFoodFlowContract
			}
			consumed += a.Consumed
			held += a.Bag
		}
		if gathered != consumed+held {
			return FoodFlowBalance{}, ErrFoodFlowContract
		}
		b.Gathered += gathered
		b.Consumed += consumed
		b.Held += held
	}
	for _, a := range actors {
		b.Energy += a.Energy
		b.BasalSpent += a.BasalSpent
		b.EnergyCapLost += a.CapLost
	}
	if b.Produced != b.Consumed+b.Stock+b.Held || b.InitialEnergy+b.Consumed != b.Energy+b.BasalSpent+b.EnergyCapLost {
		return FoodFlowBalance{}, ErrFoodFlowContract
	}
	return b, nil
}
