package world

import "agentworld/internal/sim"

// Capacity v3 state is plain exact integers. Worksite and granary are private
// per-actor components; the patch and slots are shared wild fixtures with the
// v1 accounting. No function here mutates its inputs; every failure returns
// zero states and leaves the arguments untouched.
type CapacityPatchState struct{ Yield, Pulses, Produced, Unrealized int64 }
type CapacitySlotState struct{ Stock, Gathered int64 }
type CapacityBagState struct {
	Units  int64
	Source sim.EntityID // zero iff empty; always the holder's wild home patch
}
type CapacityBodyState struct {
	Energy, Hunger, BasalSpent, CapLost, Consumed int64 // Consumed counts wild home-patch meals
	LastGatherHour                                int64
}
type CapacityWorksiteState struct {
	Capital, Wip, WearDebt, InvestedUnits, PointsCreated, PointsDecayed int64
	LastBuildHour                                                       int64
}
type CapacityGranaryState struct {
	Stock, YieldTotal, YieldUnrealized, StoredMeals int64
	LastStoredMealHour                              int64
}

// CapacityActorState is one actor's four component rows read at one version.
type CapacityActorState struct {
	Body     CapacityBodyState
	Bag      CapacityBagState
	Worksite CapacityWorksiteState
	Granary  CapacityGranaryState
}

type CapacityProduction struct{ Produced, Unrealized int64 }
type CapacityMeal struct{ EnergyGained, EnergyCapLost int64 }
type CapacityBasalCost struct{ EnergySpent, UnmetEnergy int64 }
type CapacityBuildResult struct{ PointsCompleted int64 }
type CapacityYieldResult struct{ Realized, Unrealized int64 }
type CapacityWearResult struct{ PointsDecayed int64 }

// CapacityBalance carries the aggregate frozen gates: G1 wild flow, G2
// capital and granary ledgers, and G3 energy conservation over
// CapacityInitialEnergy*CapacityActorCount = 176 initial energy units.
type CapacityBalance struct {
	Produced, Unrealized, Gathered, Consumed, Stock, Held, Invested int64
	YieldTotal, YieldUnrealized, GranaryStock, StoredMeals          int64
	Capital, Wip, PointsCreated, PointsDecayed                      int64
	InitialEnergy, Energy, BasalSpent, EnergyCapLost                int64
}

func validCapacityPatch(p CapacityPatchState) bool {
	max := int64(CapacityHorizonHours * CapacitySlotsPerPatch)
	return p.Yield >= 0 && p.Yield <= CapacitySlotsPerPatch && p.Pulses >= 0 && p.Pulses <= CapacityHorizonHours &&
		p.Produced >= 0 && p.Produced <= max && p.Unrealized >= 0 && p.Unrealized <= max &&
		p.Produced+p.Unrealized == p.Yield*p.Pulses
}
func validCapacitySlot(s CapacitySlotState) bool {
	return s.Stock >= 0 && s.Stock <= 1 && s.Gathered >= 0 && s.Gathered <= CapacityHorizonHours
}
func validCapacityBody(b CapacityBodyState) bool {
	return b.Energy >= 0 && b.Energy <= CapacityEnergyCapacity && b.Hunger >= 0 && b.Hunger <= CapacityHungerCapacity &&
		b.BasalSpent >= 0 && b.BasalSpent <= CapacityHorizonHours && b.CapLost >= 0 && b.CapLost <= CapacityHorizonHours &&
		b.Consumed >= 0 && b.Consumed <= CapacityHorizonHours &&
		b.LastGatherHour >= CapacityNeverHour && b.LastGatherHour < CapacityHorizonHours
}
func validCapacityBag(home sim.EntityID, b CapacityBagState) bool {
	return b.Units >= 0 && b.Units <= CapacityBagCapacity && (b.Units == 0) == (b.Source == 0) &&
		(b.Units == 0 || b.Source == home) // wild-only, home patch provenance
}
func validCapacityWorksite(w CapacityWorksiteState) bool {
	// G2 per-actor ledgers: invested units equal the two units behind every
	// completed point plus the outstanding work-in-progress unit, and created
	// minus decayed is exactly the standing capital.
	return w.Capital >= 0 && w.Capital <= CapacityKMax && w.Wip >= 0 && w.Wip <= CapacityWipMax &&
		w.WearDebt >= 0 && w.WearDebt <= CapacityWearDebtMax &&
		w.InvestedUnits >= 0 && w.InvestedUnits <= CapacityHorizonHours &&
		w.PointsCreated >= 0 && w.PointsCreated <= CapacityHorizonHours &&
		w.PointsDecayed >= 0 && w.PointsDecayed <= CapacityHorizonHours &&
		w.LastBuildHour >= CapacityNeverHour && w.LastBuildHour < CapacityHorizonHours &&
		w.InvestedUnits == CapacityPointCostWip*w.PointsCreated+w.Wip &&
		w.PointsCreated-w.PointsDecayed == w.Capital
}
func validCapacityGranary(g CapacityGranaryState) bool {
	// G2 granary ledger: every yielded unit is stored, eaten, or unrealized.
	max := int64(CapacityHorizonHours * CapacitySlotsPerPatch)
	return g.Stock >= 0 && g.Stock <= CapacityGranaryMax &&
		g.YieldTotal >= 0 && g.YieldTotal <= max && g.YieldUnrealized >= 0 && g.YieldUnrealized <= max &&
		g.StoredMeals >= 0 && g.StoredMeals <= CapacityHorizonHours &&
		g.LastStoredMealHour >= CapacityNeverHour && g.LastStoredMealHour < CapacityHorizonHours &&
		g.YieldTotal == g.StoredMeals+g.Stock+g.YieldUnrealized
}

// validCapacityActor checks the four rows of one actor, the home-patch bag
// provenance, never-gathered coherence, and the per-actor G3 energy identity
// 11 + wild meals + stored meals = energy + basal + cap spill.
func validCapacityActor(actor sim.EntityID, a CapacityActorState) bool {
	home, err := CapacityActorPatchID(actor)
	if err != nil || !validCapacityBody(a.Body) || !validCapacityBag(home, a.Bag) || !validCapacityWorksite(a.Worksite) || !validCapacityGranary(a.Granary) {
		return false
	}
	if a.Body.LastGatherHour == CapacityNeverHour && (a.Bag.Units != 0 || a.Body.Consumed != 0) {
		return false
	}
	return CapacityInitialEnergy+a.Body.Consumed+a.Granary.StoredMeals == a.Body.Energy+a.Body.BasalSpent+a.Body.CapLost
}

// capacityStock verifies the patch production ledger against its slots: no
// slot supplies units before they are produced, and the produced total equals
// gathered plus current stock (G1).
func capacityStock(p CapacityPatchState, slots [CapacitySlotsPerPatch]CapacitySlotState) bool {
	if !validCapacityPatch(p) {
		return false
	}
	var total int64
	for _, slot := range slots {
		if !validCapacitySlot(slot) || slot.Gathered+slot.Stock > p.Pulses {
			return false
		}
		total += slot.Gathered + slot.Stock
	}
	return total == p.Produced
}

// CapacityProduceWild fills vacant slots in ascending order, exactly as v1.
// Full slots block inflow, accounted only as unrealized. The runner commits
// the patch ledger and changed slot stocks in one world event per pulse.
func CapacityProduceWild(h int, p CapacityPatchState, slots [CapacitySlotsPerPatch]CapacitySlotState) (CapacityPatchState, [CapacitySlotsPerPatch]CapacitySlotState, CapacityProduction, error) {
	if _, err := CapacityPhaseTime(h, CapacityPhasePulse); err != nil || int64(h) != p.Pulses || !capacityStock(p, slots) {
		return CapacityPatchState{}, slots, CapacityProduction{}, ErrCapacityContract
	}
	out := slots
	var produced int64
	for i := range out {
		if out[i].Stock == 0 && produced < p.Yield {
			out[i].Stock, produced = 1, produced+1
		}
	}
	result := CapacityProduction{Produced: produced, Unrealized: p.Yield - produced}
	p.Pulses++
	p.Produced += produced
	p.Unrealized += result.Unrealized
	if !capacityStock(p, out) {
		return CapacityPatchState{}, slots, CapacityProduction{}, ErrCapacityContract
	}
	return p, out, result, nil
}

// CapacityGather transfers one identified home-patch slot unit to the actor's
// bag, v1-identical: alive, empty bag, at most one gather per hour, slot
// inside the actor's own patch. The runner commits slot stock/gathered, bag
// units/source, and last-gather-hour atomically in one actor event.
func CapacityGather(patchID, slotID, actorID sim.EntityID, hour int, slot CapacitySlotState, a CapacityActorState) (CapacitySlotState, CapacityActorState, error) {
	owner, err := CapacityActorPatchID(actorID)
	if err != nil || owner != patchID || hour < 0 || hour >= CapacityHorizonHours || !validCapacitySlot(slot) || slot.Stock != 1 ||
		slot.Gathered > int64(hour) || !validCapacityActor(actorID, a) || a.Body.Energy == 0 || a.Bag.Units != 0 || a.Body.LastGatherHour >= int64(hour) {
		return CapacitySlotState{}, CapacityActorState{}, ErrCapacityContract
	}
	index := int(patchID - 1001)
	if slotID < sim.EntityID(2001+index*CapacitySlotsPerPatch) || slotID >= sim.EntityID(2001+(index+1)*CapacitySlotsPerPatch) {
		return CapacitySlotState{}, CapacityActorState{}, ErrCapacityContract
	}
	slot.Stock, slot.Gathered = 0, slot.Gathered+1
	a.Bag = CapacityBagState{Units: 1, Source: patchID}
	a.Body.LastGatherHour = int64(hour)
	if !validCapacityActor(actorID, a) {
		return CapacitySlotState{}, CapacityActorState{}, ErrCapacityContract
	}
	return slot, a, nil
}

// CapacityConsume eats the held wild unit at the bag decision with v1 meal
// semantics: +1 energy, counted spill at the cap-12 ceiling, −1 hunger.
func CapacityConsume(actorID sim.EntityID, a CapacityActorState) (CapacityActorState, CapacityMeal, error) {
	home, err := CapacityActorPatchID(actorID)
	if err != nil || !validCapacityActor(actorID, a) || a.Body.Energy == 0 || a.Bag.Units != 1 || a.Bag.Source != home || a.Body.Consumed >= CapacityHorizonHours {
		return CapacityActorState{}, CapacityMeal{}, ErrCapacityContract
	}
	a.Bag = CapacityBagState{}
	a.Body.Consumed++
	result := CapacityMeal{EnergyGained: 1}
	if a.Body.Energy == CapacityEnergyCapacity {
		a.Body.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Body.Energy++
	}
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityMeal{}, ErrCapacityContract
	}
	return a, result, nil
}

// CapacityBasal advances elapsed whole hours even after rejected intentions;
// the runner invokes it at integer pulses before that hour's claims.
func CapacityBasal(actorID sim.EntityID, a CapacityActorState, elapsedHours int64) (CapacityActorState, CapacityBasalCost, error) {
	if _, err := CapacityActorPatchID(actorID); err != nil || !validCapacityActor(actorID, a) || elapsedHours < 1 || elapsedHours > CapacityHorizonHours || a.Body.BasalSpent > CapacityHorizonHours-elapsedHours {
		return CapacityActorState{}, CapacityBasalCost{}, ErrCapacityContract
	}
	spent := elapsedHours
	if spent > a.Body.Energy {
		spent = a.Body.Energy
	}
	result := CapacityBasalCost{EnergySpent: spent, UnmetEnergy: elapsedHours - spent}
	a.Body.Energy -= spent
	a.Body.BasalSpent += spent
	if elapsedHours >= CapacityHungerCapacity-a.Body.Hunger {
		a.Body.Hunger = CapacityHungerCapacity
	} else {
		a.Body.Hunger += elapsedHours
	}
	if !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityBasalCost{}, ErrCapacityContract
	}
	return a, result, nil
}

// CapacityBuild consumes the held bag unit into work-in-progress: bag 1→0,
// wip+1; wip=2 → k+1, wip=0. One capital point therefore costs exactly two
// bag units and two build windows. Eligibility is the frozen bag-decision
// phase, aliveness, one build per hour, and k+wip < CapacityKMax; the stored
// bag is never itself a store, so an empty bag cannot build.
func CapacityBuild(hour int, actorID sim.EntityID, a CapacityActorState) (CapacityActorState, CapacityBuildResult, error) {
	if _, err := CapacityPhaseTime(hour, CapacityPhaseBagDecision); err != nil {
		return CapacityActorState{}, CapacityBuildResult{}, ErrCapacityContract
	}
	if _, err := CapacityActorPatchID(actorID); err != nil || !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityBuildResult{}, ErrCapacityContract
	}
	if a.Body.Energy == 0 || a.Bag.Units != 1 || a.Worksite.Capital+a.Worksite.Wip >= CapacityKMax || a.Worksite.LastBuildHour >= int64(hour) {
		return CapacityActorState{}, CapacityBuildResult{}, ErrCapacityContract
	}
	a.Bag = CapacityBagState{}
	w := a.Worksite
	w.InvestedUnits++
	w.LastBuildHour = int64(hour)
	var result CapacityBuildResult
	w.Wip++
	if w.Wip > CapacityWipMax { // wip=2: one point completes atomically
		w.Wip = 0
		w.Capital++
		w.PointsCreated++
		result.PointsCompleted = 1
	}
	a.Worksite = w
	if !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityBuildResult{}, ErrCapacityContract
	}
	return a, result, nil
}

// CapacityCapitalYield moves min(k, free granary space) food units into the
// living owner's granary, one per standing point per pulse, counting overflow
// as typed yield-unrealized. Dead owners yield nothing: their pulse row is
// simply never eligible.
func CapacityCapitalYield(actorID sim.EntityID, a CapacityActorState) (CapacityActorState, CapacityYieldResult, error) {
	if _, err := CapacityActorPatchID(actorID); err != nil || !validCapacityActor(actorID, a) || a.Body.Energy == 0 {
		return CapacityActorState{}, CapacityYieldResult{}, ErrCapacityContract
	}
	free := CapacityGranaryMax - a.Granary.Stock
	realized := a.Worksite.Capital
	if realized > free {
		realized = free
	}
	// The granary ledger counts the full per-point yield: every yielded unit
	// is stored or typed unrealized, so G2's identity survives short storage.
	a.Granary.Stock += realized
	a.Granary.YieldTotal += a.Worksite.Capital
	a.Granary.YieldUnrealized += a.Worksite.Capital - realized
	result := CapacityYieldResult{Realized: realized, Unrealized: a.Worksite.Capital - realized}
	if !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityYieldResult{}, ErrCapacityContract
	}
	return a, result, nil
}

// CapacityWear accrues depreciation against every owner, dead owners included:
// debt += k; while debt ≥ CapacityWearPeriod: k−1, debt−=CapacityWearPeriod.
// A standing point therefore survives exactly six pulses. The transient debt
// never exceeds CapacityWearDebtMax for reachable states; anything larger is
// forged and rejected with zero mutation.
func CapacityWear(actorID sim.EntityID, a CapacityActorState) (CapacityActorState, CapacityWearResult, error) {
	if _, err := CapacityActorPatchID(actorID); err != nil || !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityWearResult{}, ErrCapacityContract
	}
	w := a.Worksite
	w.WearDebt += w.Capital
	if w.WearDebt > CapacityWearDebtMax {
		return CapacityActorState{}, CapacityWearResult{}, ErrCapacityContract
	}
	var result CapacityWearResult
	for w.WearDebt >= CapacityWearPeriod {
		if w.Capital == 0 { // unreachable for reachable states: guards [0,KMax]
			return CapacityActorState{}, CapacityWearResult{}, ErrCapacityContract
		}
		w.Capital--
		w.WearDebt -= CapacityWearPeriod
		w.PointsDecayed++
		result.PointsDecayed++
	}
	a.Worksite = w
	if !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityWearResult{}, ErrCapacityContract
	}
	return a, result, nil
}

// CapacityEatStored is the only granary draw, with v1 meal semantics: +1
// energy, counted spill at the cap-12 ceiling, −1 hunger, and the stored-meal
// ledger and last-stored-meal hour are advanced atomically.
func CapacityEatStored(hour int, actorID sim.EntityID, a CapacityActorState) (CapacityActorState, CapacityMeal, error) {
	if hour < 0 || hour >= CapacityHorizonHours || !validCapacityActor(actorID, a) || a.Body.Energy == 0 || a.Granary.Stock < 1 || a.Granary.StoredMeals >= CapacityHorizonHours {
		return CapacityActorState{}, CapacityMeal{}, ErrCapacityContract
	}
	a.Granary.Stock--
	a.Granary.StoredMeals++
	a.Granary.LastStoredMealHour = int64(hour)
	result := CapacityMeal{EnergyGained: 1}
	if a.Body.Energy == CapacityEnergyCapacity {
		a.Body.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Body.Energy++
	}
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validCapacityActor(actorID, a) {
		return CapacityActorState{}, CapacityMeal{}, ErrCapacityContract
	}
	return a, result, nil
}

// CapacityCheckConservation validates every patch, slot, and actor and then
// the aggregate frozen gates:
//
//	G1: wild produced = gathered + stock, and produced + unrealized = q·pulses.
//	G2: invested = 2·created + wip; created − decayed = capital;
//	    granary yield = stored meals + stock + unrealized.
//	G3: 176 + wild meals + stored meals = Σenergy + basal + cap spill.
//
// The v1 wild identity extends by investment: gathered = consumed + held +
// invested per patch, because every invested bag unit was a home-patch gather.
func CapacityCheckConservation(patches [CapacityPatchCount]CapacityPatchState, slots [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState, actors [CapacityActorCount]CapacityActorState) (CapacityBalance, error) {
	var b CapacityBalance
	b.InitialEnergy = CapacityInitialEnergy * CapacityActorCount
	for i, p := range patches {
		if !capacityStock(p, slots[i]) {
			return CapacityBalance{}, ErrCapacityContract
		}
		b.Produced += p.Produced
		b.Unrealized += p.Unrealized
		var gathered, consumed, held, invested int64
		for _, slot := range slots[i] {
			gathered += slot.Gathered
			b.Stock += slot.Stock
		}
		for j := i * CapacitySlotsPerPatch; j < (i+1)*CapacitySlotsPerPatch; j++ {
			a := actors[j]
			if !validCapacityActor(sim.EntityID(j+1), a) {
				return CapacityBalance{}, ErrCapacityContract
			}
			consumed += a.Body.Consumed
			held += a.Bag.Units
			invested += a.Worksite.InvestedUnits
			b.YieldTotal += a.Granary.YieldTotal
			b.GranaryStock += a.Granary.Stock
			b.YieldUnrealized += a.Granary.YieldUnrealized
			b.StoredMeals += a.Granary.StoredMeals
			b.Capital += a.Worksite.Capital
			b.Wip += a.Worksite.Wip
			b.PointsCreated += a.Worksite.PointsCreated
			b.PointsDecayed += a.Worksite.PointsDecayed
			b.Energy += a.Body.Energy
			b.BasalSpent += a.Body.BasalSpent
			b.EnergyCapLost += a.Body.CapLost
		}
		if gathered != consumed+held+invested {
			return CapacityBalance{}, ErrCapacityContract
		}
		b.Gathered += gathered
		b.Consumed += consumed
		b.Held += held
		b.Invested += invested
	}
	if b.Gathered != b.Consumed+b.Held+b.Invested || b.Produced != b.Gathered+b.Stock ||
		b.Invested != CapacityPointCostWip*b.PointsCreated+b.Wip || b.PointsCreated-b.PointsDecayed != b.Capital ||
		b.YieldTotal != b.StoredMeals+b.GranaryStock+b.YieldUnrealized ||
		b.InitialEnergy+b.Consumed+b.StoredMeals != b.Energy+b.BasalSpent+b.EnergyCapLost {
		return CapacityBalance{}, ErrCapacityContract
	}
	return b, nil
}
