package world

import (
	"agentworld/internal/sim"
	"fmt"
)

// Reproduction v4 state is plain exact integers, extending the v3 capacity
// state with the genome, the birth-request row, the two new debt carries
// (basal-debt on the body, yield-debt on the granary — hourly accumulators
// can never live on the immutable genome), and the widened wear-debt bound.
// No function here mutates its inputs; every failure returns zero states and
// leaves the arguments untouched.
type ReproductionPatchState struct{ Yield, Pulses, Produced, Unrealized int64 }
type ReproductionSlotState struct{ Stock, Gathered int64 }
type ReproductionBagState struct {
	Units  int64
	Source sim.EntityID // the holder's home patch, always present
}
type ReproductionBodyState struct {
	Energy, Hunger, BasalSpent, CapLost, Consumed int64 // Consumed counts wild home-patch meals
	LastGatherHour                                int64
	BasalDebt                                     int64 // basal carry: debt+=6, drained m per energy unit
}
type ReproductionWorksiteState struct {
	Capital, Wip, WearDebt, InvestedUnits, PointsCreated, PointsDecayed int64
	LastBuildHour                                                       int64
}
type ReproductionGranaryState struct {
	Stock, YieldTotal, YieldUnrealized, StoredMeals int64
	LastStoredMealHour                              int64
	YieldDebt                                       int64 // yield carry: debt+=6k, drained y per grant
}
type ReproductionGenomeState struct {
	LocusM, LocusY, LocusW         int64        // alleles in [5,7], neutral 6, immutable post-creation
	ParentA, ParentB               sim.EntityID // zero for founders; rule 410's allocation only
	BirthHour, DeathHour, DiedHour int64        // DiedHour -1 = alive; starvation/age-death write only this
	BirthGranaryPaid               int64        // granary units this actor has paid toward births
}
type ReproductionBirthRequestState struct {
	Hour            int64        // hour of the most recent filing, -1 if never
	Addressee       sim.EntityID // zero only while never-filed
	ReportedCapital int64        // self-reported k claim, never requester truth
	ReportedGranary int64        // self-reported granary claim, never requester truth
	Status          ReproductionRequestStatus
	LastRequestHour int64
	LastReplyHour   int64
}

// ReproductionActorState is one actor's six component rows read at one
// version. The bag's source-patch is the actor's home patch: ID-derived for
// founders, the proposer's patch for a newborn.
type ReproductionActorState struct {
	Body     ReproductionBodyState
	Bag      ReproductionBagState
	Worksite ReproductionWorksiteState
	Granary  ReproductionGranaryState
	Genome   ReproductionGenomeState
	Request  ReproductionBirthRequestState
}

type ReproductionRequestStatus int64

const (
	ReproductionRequestIdle     ReproductionRequestStatus = iota // 0: never filed
	ReproductionRequestPending                                   // 1
	ReproductionRequestAccepted                                  // 2
	ReproductionRequestRefused                                   // 3
	ReproductionRequestExpired                                   // 4
)

type ReproductionProduction struct{ Produced, Unrealized int64 }
type ReproductionMeal struct{ EnergyGained, EnergyCapLost int64 }
type ReproductionBasalCost struct{ EnergySpent, UnmetEnergy int64 }
type ReproductionBuildResult struct{ PointsCompleted int64 }
type ReproductionYieldResult struct{ Realized, Unrealized int64 }
type ReproductionWearResult struct{ PointsDecayed int64 }

// ReproductionBalance carries the aggregate frozen gates: G1 wild flow, G2a
// granary ledger extended with birth-granary-paid, G2b capital ledgers, and
// G3 energy conservation over 176 initial founder units extended by exactly
// 8 per birth.
type ReproductionBalance struct {
	Produced, Unrealized, Gathered, Consumed, Stock, Held, Invested int64
	YieldTotal, YieldUnrealized, GranaryStock, StoredMeals          int64
	Capital, Wip, PointsCreated, PointsDecayed                      int64
	InitialEnergy, Energy, BasalSpent, EnergyCapLost                int64
	BirthGranaryPaid, Births                                        int64
}

// ReproductionNewbornState pairs a newborn's ID with its complete state.
type ReproductionNewbornState struct {
	ID    sim.EntityID
	State ReproductionActorState
}

func reproductionValidPatch(id sim.EntityID) bool {
	return id == 1001 || id == 1002
}

func validReproductionBody(b ReproductionBodyState) bool {
	return b.Energy >= 0 && b.Energy <= ReproductionEnergyCapacity && b.Hunger >= 0 && b.Hunger <= ReproductionHungerCapacity &&
		b.BasalSpent >= 0 && b.BasalSpent <= ReproductionBasalSpentMax && b.CapLost >= 0 && b.CapLost <= ReproductionHorizonHours &&
		b.Consumed >= 0 && b.Consumed <= ReproductionHorizonHours &&
		b.LastGatherHour >= ReproductionNeverHour && b.LastGatherHour < ReproductionHorizonHours &&
		b.BasalDebt >= 0 && b.BasalDebt <= ReproductionBasalDebtMax
}

func validReproductionBag(home sim.EntityID, b ReproductionBagState) bool {
	return b.Units >= 0 && b.Units <= ReproductionBagCapacity && b.Source == home && reproductionValidPatch(home)
}

func validReproductionWorksite(w ReproductionWorksiteState) bool {
	// G2b per-actor ledgers, verbatim v3 with the widened wear-debt bound.
	return w.Capital >= 0 && w.Capital <= ReproductionKMax && w.Wip >= 0 && w.Wip <= ReproductionWipMax &&
		w.WearDebt >= 0 && w.WearDebt <= ReproductionWearDebtMax &&
		w.InvestedUnits >= 0 && w.InvestedUnits <= ReproductionHorizonHours &&
		w.PointsCreated >= 0 && w.PointsCreated <= ReproductionHorizonHours &&
		w.PointsDecayed >= 0 && w.PointsDecayed <= ReproductionHorizonHours &&
		w.LastBuildHour >= ReproductionNeverHour && w.LastBuildHour < ReproductionHorizonHours &&
		w.InvestedUnits == ReproductionPointCostWip*w.PointsCreated+w.Wip &&
		w.PointsCreated-w.PointsDecayed == w.Capital
}

func validReproductionGranary(g ReproductionGranaryState) bool {
	max := int64(ReproductionGranaryYieldMax)
	return g.Stock >= 0 && g.Stock <= ReproductionGranaryMax &&
		g.YieldTotal >= 0 && g.YieldTotal <= max && g.YieldUnrealized >= 0 && g.YieldUnrealized <= max &&
		g.StoredMeals >= 0 && g.StoredMeals <= ReproductionHorizonHours &&
		g.LastStoredMealHour >= ReproductionNeverHour && g.LastStoredMealHour < ReproductionHorizonHours &&
		g.YieldDebt >= 0 && g.YieldDebt <= ReproductionYieldDebtMax
}

// validReproductionGenome checks the frozen bounds and lineage shape:
// founders (IDs 1-16) have absent parents and birth-hour 0; newborns have
// both parents present, distinct, strictly earlier IDs (which also makes the
// reproduction graph acyclic), and a creation lifetime in [96,143]. died-hour
// is either -1 or the death hour itself, never below birth-hour.
func validReproductionGenome(actor sim.EntityID, g ReproductionGenomeState) bool {
	locus := func(v int64) bool { return v >= ReproductionGenomeLocusMin && v <= ReproductionGenomeLocusMax }
	if !locus(g.LocusM) || !locus(g.LocusY) || !locus(g.LocusW) {
		return false
	}
	if g.BirthGranaryPaid < 0 || g.BirthGranaryPaid > ReproductionBirthGranaryPaidMax {
		return false
	}
	if reproductionFounderEntity(actor) {
		if g.ParentA != 0 || g.ParentB != 0 || g.BirthHour != 0 {
			return false
		}
	} else {
		if g.ParentA == 0 || g.ParentB == 0 || !reproductionActorEntity(g.ParentA) || !reproductionActorEntity(g.ParentB) ||
			g.ParentA == g.ParentB || g.ParentA >= actor || g.ParentB >= actor || g.BirthHour < 0 || g.BirthHour >= ReproductionHorizonHours {
			return false
		}
	}
	if g.DeathHour < 0 || g.DeathHour > 479 || g.DeathHour-g.BirthHour < ReproductionFounderLifetimeMin || g.DeathHour-g.BirthHour > ReproductionFounderLifetimeMax {
		return false
	}
	return g.DiedHour == ReproductionNeverHour || (g.DiedHour >= g.BirthHour && g.DiedHour <= ReproductionHorizonHours)
}

func validReproductionBirthRequest(owner sim.EntityID, r ReproductionBirthRequestState) bool {
	if r.ReportedCapital < 0 || r.ReportedCapital > ReproductionKMax || r.ReportedGranary < 0 || r.ReportedGranary > ReproductionGranaryMax {
		return false
	}
	hourOK := func(h int64) bool { return h >= ReproductionNeverHour && h < ReproductionHorizonHours }
	if !hourOK(r.Hour) || !hourOK(r.LastRequestHour) || !hourOK(r.LastReplyHour) || r.Status < ReproductionRequestIdle || r.Status > ReproductionRequestExpired {
		return false
	}
	if r.Status == ReproductionRequestIdle {
		return r.Hour == ReproductionNeverHour && r.Addressee == 0 && r.ReportedCapital == 0 && r.ReportedGranary == 0 && r.LastRequestHour == ReproductionNeverHour
	}
	// Every filed row retains its latest request for the privileged audit:
	// the request hour is the last-request hour and the addressee is a
	// valid actor other than the owner.
	return r.Hour >= 0 && r.Hour == r.LastRequestHour && r.Addressee != 0 && r.Addressee != owner && reproductionActorEntity(r.Addressee)
}

func reproductionAlive(a ReproductionActorState) bool {
	return a.Genome.DiedHour == ReproductionNeverHour && a.Body.Energy > 0
}

// validReproductionActor checks the six rows of one actor: home-patch
// provenance (founder homes are ID-placed), never-gathered coherence, the
// per-actor G2a granary identity extended by birth-granary-paid, the
// per-actor G3 energy identity with the creation base (11 founder, 8
// newborn), and the alive invariant died-hour=-1 ⇒ energy>0.
func validReproductionActor(actor sim.EntityID, a ReproductionActorState) bool {
	if !reproductionActorEntity(actor) || !validReproductionBody(a.Body) || !validReproductionWorksite(a.Worksite) || !validReproductionGranary(a.Granary) || !validReproductionGenome(actor, a.Genome) || !validReproductionBirthRequest(actor, a.Request) {
		return false
	}
	home := a.Bag.Source
	if reproductionFounderEntity(actor) {
		expected, err := ReproductionFounderPatchID(actor)
		if err != nil || home != expected {
			return false
		}
	}
	if !validReproductionBag(home, a.Bag) {
		return false
	}
	if a.Body.LastGatherHour == ReproductionNeverHour && (a.Bag.Units != 0 || a.Body.Consumed != 0) {
		return false
	}
	base := ReproductionFounderInitialEnergy
	if !reproductionFounderEntity(actor) {
		base = ReproductionNewbornEnergy
	}
	// G2a extended: every yielded unit is stored, standing, unrealized, or
	// paid toward a birth.
	if a.Granary.YieldTotal != a.Granary.StoredMeals+a.Granary.Stock+a.Granary.YieldUnrealized+a.Genome.BirthGranaryPaid {
		return false
	}
	// G3 per actor.
	return base+a.Body.Consumed+a.Granary.StoredMeals == a.Body.Energy+a.Body.BasalSpent+a.Body.CapLost
}

// reproductionStock verifies the patch production ledger against its slots,
// verbatim v3 (G1).
func reproductionStock(p ReproductionPatchState, slots [ReproductionSlotsPerPatch]ReproductionSlotState) bool {
	if !validReproductionPatch(p) {
		return false
	}
	var total int64
	for _, slot := range slots {
		if !validReproductionSlot(slot) || slot.Gathered+slot.Stock > p.Pulses {
			return false
		}
		total += slot.Gathered + slot.Stock
	}
	return total == p.Produced
}

func validReproductionPatch(p ReproductionPatchState) bool {
	max := int64(ReproductionHorizonHours * ReproductionSlotsPerPatch)
	return p.Yield >= 0 && p.Yield <= ReproductionSlotsPerPatch && p.Pulses >= 0 && p.Pulses <= ReproductionHorizonHours &&
		p.Produced >= 0 && p.Produced <= max && p.Unrealized >= 0 && p.Unrealized <= max &&
		p.Produced+p.Unrealized == p.Yield*p.Pulses
}

func validReproductionSlot(s ReproductionSlotState) bool {
	return s.Stock >= 0 && s.Stock <= 1 && s.Gathered >= 0 && s.Gathered <= ReproductionHorizonHours
}

// ReproductionProduceWild fills vacant slots in ascending order, exactly as
// v1 and v3. Full slots block inflow, accounted only as unrealized.
func ReproductionProduceWild(h int, p ReproductionPatchState, slots [ReproductionSlotsPerPatch]ReproductionSlotState) (ReproductionPatchState, [ReproductionSlotsPerPatch]ReproductionSlotState, ReproductionProduction, error) {
	if _, err := ReproductionPhaseTime(h, ReproductionPhasePulse); err != nil || int64(h) != p.Pulses || !reproductionStock(p, slots) {
		return ReproductionPatchState{}, slots, ReproductionProduction{}, ErrReproductionContract
	}
	out := slots
	var produced int64
	for i := range out {
		if out[i].Stock == 0 && produced < p.Yield {
			out[i].Stock, produced = 1, produced+1
		}
	}
	result := ReproductionProduction{Produced: produced, Unrealized: p.Yield - produced}
	p.Pulses++
	p.Produced += produced
	p.Unrealized += result.Unrealized
	if !reproductionStock(p, out) {
		return ReproductionPatchState{}, slots, ReproductionProduction{}, ErrReproductionContract
	}
	return p, out, result, nil
}

// ReproductionGather transfers one identified home-patch slot unit to the
// actor's bag, v1/v3-identical among the living: dead actors take no
// actions. The bag's home-patch source is already present and unchanged.
func ReproductionGather(patchID, slotID, actorID sim.EntityID, hour int, slot ReproductionSlotState, a ReproductionActorState) (ReproductionSlotState, ReproductionActorState, error) {
	home := a.Bag.Source
	if home != patchID || hour < 0 || hour >= ReproductionHorizonHours || !validReproductionSlot(slot) || slot.Stock != 1 ||
		slot.Gathered > int64(hour) || !validReproductionActor(actorID, a) || !reproductionAlive(a) || a.Bag.Units != 0 || a.Body.LastGatherHour >= int64(hour) {
		return ReproductionSlotState{}, ReproductionActorState{}, ErrReproductionContract
	}
	index := int(patchID - 1001)
	if slotID < sim.EntityID(2001+index*ReproductionSlotsPerPatch) || slotID >= sim.EntityID(2001+(index+1)*ReproductionSlotsPerPatch) {
		return ReproductionSlotState{}, ReproductionActorState{}, ErrReproductionContract
	}
	slot.Stock, slot.Gathered = 0, slot.Gathered+1
	a.Bag.Units = 1
	a.Body.LastGatherHour = int64(hour)
	if !validReproductionActor(actorID, a) {
		return ReproductionSlotState{}, ReproductionActorState{}, ErrReproductionContract
	}
	return slot, a, nil
}

// ReproductionConsume eats the held wild unit at the bag decision with
// v1/v3 meal semantics: +1 energy, counted spill at the cap-12 ceiling,
// -1 hunger.
func ReproductionConsume(actorID sim.EntityID, a ReproductionActorState) (ReproductionActorState, ReproductionMeal, error) {
	if !reproductionActorEntity(actorID) || !validReproductionActor(actorID, a) || !reproductionAlive(a) || a.Bag.Units != 1 || a.Body.Consumed >= ReproductionHorizonHours {
		return ReproductionActorState{}, ReproductionMeal{}, ErrReproductionContract
	}
	a.Bag.Units = 0
	a.Body.Consumed++
	result := ReproductionMeal{EnergyGained: 1}
	if a.Body.Energy == ReproductionEnergyCapacity {
		a.Body.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Body.Energy++
	}
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionMeal{}, ErrReproductionContract
	}
	return a, result, nil
}

// ReproductionBasal advances elapsed whole hours through the genome-modulated
// divisor loop: debt += 6; while debt ≥ locus-m ∧ energy ≥ 1: energy−1,
// spent+1, debt −= locus-m (rate 6/m per hour; byte-identical to v3 at the
// neutral 6). Dead actors are stopped and never invoked. When the loop
// drains the last energy unit the actor starves: died-hour is written (only
// rule 404 may write it, only from -1 downward to the current hour) and the
// runner issues its EffectStop. Hunger accrues exactly as v3.
func ReproductionBasal(hour int, actorID sim.EntityID, a ReproductionActorState, elapsedHours int64) (ReproductionActorState, ReproductionBasalCost, bool, error) {
	if _, err := ReproductionHourTime(hour); err != nil || !reproductionActorEntity(actorID) || !validReproductionActor(actorID, a) ||
		elapsedHours < 1 || elapsedHours > ReproductionHorizonHours || !reproductionAlive(a) {
		return ReproductionActorState{}, ReproductionBasalCost{}, false, ErrReproductionContract
	}
	// Reachable-state guard: two drains per hour (locus-m 5) plus the two
	// birth payments per birth can never exceed the widened ledger bound.
	if a.Body.BasalSpent+2*elapsedHours > ReproductionBasalSpentMax {
		return ReproductionActorState{}, ReproductionBasalCost{}, false, ErrReproductionContract
	}
	m := a.Genome.LocusM
	a.Body.BasalDebt += ReproductionGenomeDenominator
	if a.Body.BasalDebt > ReproductionBasalDebtMax {
		return ReproductionActorState{}, ReproductionBasalCost{}, false, ErrReproductionContract
	}
	var result ReproductionBasalCost
	for a.Body.BasalDebt >= m && a.Body.Energy >= 1 {
		a.Body.Energy--
		a.Body.BasalSpent++
		a.Body.BasalDebt -= m
		result.EnergySpent++
	}
	result.UnmetEnergy = elapsedHours - result.EnergySpent
	if elapsedHours >= ReproductionHungerCapacity-a.Body.Hunger {
		a.Body.Hunger = ReproductionHungerCapacity
	} else {
		a.Body.Hunger += elapsedHours
	}
	starved := a.Body.Energy == 0
	if starved {
		a.Genome.DiedHour = int64(hour)
	}
	if !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionBasalCost{}, false, ErrReproductionContract
	}
	return a, result, starved, nil
}

// ReproductionBuild consumes the held bag unit into work-in-progress,
// verbatim v3: bag 1→0 (home source retained), wip+1; wip=2 → k+1, wip=0.
// One capital point therefore costs exactly two bag units and two build
// windows.
func ReproductionBuild(hour int, actorID sim.EntityID, a ReproductionActorState) (ReproductionActorState, ReproductionBuildResult, error) {
	if _, err := ReproductionPhaseTime(hour, ReproductionPhaseBagDecision); err != nil {
		return ReproductionActorState{}, ReproductionBuildResult{}, ErrReproductionContract
	}
	if !reproductionActorEntity(actorID) || !validReproductionActor(actorID, a) || !reproductionAlive(a) {
		return ReproductionActorState{}, ReproductionBuildResult{}, ErrReproductionContract
	}
	if a.Bag.Units != 1 || a.Worksite.Capital+a.Worksite.Wip >= ReproductionKMax || a.Worksite.LastBuildHour >= int64(hour) {
		return ReproductionActorState{}, ReproductionBuildResult{}, ErrReproductionContract
	}
	a.Bag.Units = 0
	w := a.Worksite
	w.InvestedUnits++
	w.LastBuildHour = int64(hour)
	var result ReproductionBuildResult
	w.Wip++
	if w.Wip > ReproductionWipMax { // wip=2: one point completes atomically
		w.Wip = 0
		w.Capital++
		w.PointsCreated++
		result.PointsCompleted = 1
	}
	a.Worksite = w
	if !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionBuildResult{}, ErrReproductionContract
	}
	return a, result, nil
}

// ReproductionCapitalYield moves the capital yield through the
// genome-modulated divisor loop: debt += 6k; while debt ≥ locus-y: grant one
// unit (granary stock, else typed unrealized), debt −= locus-y (rate 6k/y
// per pulse; byte-identical to v3's min(k, free) at the neutral 6/6). Living
// owners only: yield stops at death.
func ReproductionCapitalYield(actorID sim.EntityID, a ReproductionActorState) (ReproductionActorState, ReproductionYieldResult, error) {
	if !reproductionActorEntity(actorID) || !validReproductionActor(actorID, a) || !reproductionAlive(a) {
		return ReproductionActorState{}, ReproductionYieldResult{}, ErrReproductionContract
	}
	g := a.Granary
	g.YieldDebt += ReproductionGenomeDenominator * a.Worksite.Capital
	if g.YieldDebt > ReproductionYieldDebtMax {
		return ReproductionActorState{}, ReproductionYieldResult{}, ErrReproductionContract
	}
	y := a.Genome.LocusY
	beforeStock, beforeUnrealized := g.Stock, g.YieldUnrealized
	for g.YieldDebt >= y {
		if g.Stock < ReproductionGranaryMax {
			g.Stock++
		} else {
			g.YieldUnrealized++
		}
		g.YieldTotal++
		g.YieldDebt -= y
	}
	a.Granary = g
	result := ReproductionYieldResult{Realized: g.Stock - beforeStock, Unrealized: g.YieldUnrealized - beforeUnrealized}
	if !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionYieldResult{}, ErrReproductionContract
	}
	return a, result, nil
}

// ReproductionWear accrues depreciation against every owner, dead owners
// included, through the v3 debt loop parameterized by the genome's locus-w:
// debt += capital; while debt ≥ w ∧ capital ≥ 1: capital−1, decayed+1,
// debt −= w. A standing point therefore survives exactly w pulses. The
// transient debt never exceeds 14 for reachable states; anything larger is
// forged and rejected with zero mutation.
func ReproductionWear(actorID sim.EntityID, a ReproductionActorState) (ReproductionActorState, ReproductionWearResult, error) {
	if !reproductionActorEntity(actorID) || !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionWearResult{}, ErrReproductionContract
	}
	w := a.Worksite
	w.WearDebt += w.Capital
	if w.WearDebt > ReproductionWearDebtMax {
		return ReproductionActorState{}, ReproductionWearResult{}, ErrReproductionContract
	}
	locus := a.Genome.LocusW
	var result ReproductionWearResult
	for w.WearDebt >= locus {
		if w.Capital == 0 { // unreachable for reachable states: guards [0,KMax]
			return ReproductionActorState{}, ReproductionWearResult{}, ErrReproductionContract
		}
		w.Capital--
		w.WearDebt -= locus
		w.PointsDecayed++
		result.PointsDecayed++
	}
	a.Worksite = w
	if !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionWearResult{}, ErrReproductionContract
	}
	return a, result, nil
}

// ReproductionEatStored is the only granary draw, with v1/v3 meal semantics;
// dead owners' granaries are frozen.
func ReproductionEatStored(hour int, actorID sim.EntityID, a ReproductionActorState) (ReproductionActorState, ReproductionMeal, error) {
	if hour < 0 || hour >= ReproductionHorizonHours || !reproductionActorEntity(actorID) || !validReproductionActor(actorID, a) || !reproductionAlive(a) ||
		a.Granary.Stock < 1 || a.Granary.StoredMeals >= ReproductionHorizonHours {
		return ReproductionActorState{}, ReproductionMeal{}, ErrReproductionContract
	}
	a.Granary.Stock--
	a.Granary.StoredMeals++
	a.Granary.LastStoredMealHour = int64(hour)
	result := ReproductionMeal{EnergyGained: 1}
	if a.Body.Energy == ReproductionEnergyCapacity {
		a.Body.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Body.Energy++
	}
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validReproductionActor(actorID, a) {
		return ReproductionActorState{}, ReproductionMeal{}, ErrReproductionContract
	}
	return a, result, nil
}

// ReproductionIdleRequestState is the canonical never-filed request row.
func ReproductionIdleRequestState() ReproductionBirthRequestState {
	return ReproductionBirthRequestState{Hour: ReproductionNeverHour, LastRequestHour: ReproductionNeverHour, LastReplyHour: ReproductionNeverHour, Status: ReproductionRequestIdle}
}

// ReproductionFounderGenome builds the all-neutral founder genome with the
// runner-resolved lifetime (stream 0x464F554E positions 0-15; the draws are
// runner-owned, this contract takes lifetimes as input). Founders are born
// at hour 0 with absent parents.
func ReproductionFounderGenome(actor sim.EntityID, lifetime int64) (ReproductionGenomeState, error) {
	if !reproductionFounderEntity(actor) || lifetime < ReproductionFounderLifetimeMin || lifetime > ReproductionFounderLifetimeMax {
		return ReproductionGenomeState{}, ErrReproductionContract
	}
	g := ReproductionGenomeState{LocusM: ReproductionGenomeNeutral, LocusY: ReproductionGenomeNeutral, LocusW: ReproductionGenomeNeutral, BirthHour: 0, DeathHour: lifetime, DiedHour: ReproductionNeverHour}
	if !validReproductionGenome(actor, g) {
		return ReproductionGenomeState{}, ErrReproductionContract
	}
	return g, nil
}

// ReproductionFounderState assembles one founder's six rows at hour 0 with
// the v3 initial body values and an idle request row.
func ReproductionFounderState(actor sim.EntityID, lifetime int64) (ReproductionActorState, error) {
	genome, err := ReproductionFounderGenome(actor, lifetime)
	if err != nil {
		return ReproductionActorState{}, err
	}
	home, err := ReproductionFounderPatchID(actor)
	if err != nil {
		return ReproductionActorState{}, err
	}
	a := ReproductionActorState{
		Body:     ReproductionBodyState{Energy: ReproductionFounderInitialEnergy, LastGatherHour: ReproductionNeverHour},
		Bag:      ReproductionBagState{Source: home},
		Worksite: ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour},
		Granary:  ReproductionGranaryState{LastStoredMealHour: ReproductionNeverHour},
		Genome:   genome,
		Request:  ReproductionIdleRequestState(),
	}
	if !validReproductionActor(actor, a) {
		return ReproductionActorState{}, ErrReproductionContract
	}
	return a, nil
}

// ReproductionParentLookup returns an actor's parent-a/parent-b references;
// ok is false for unknown or founder rows.
type ReproductionParentLookup func(sim.EntityID) (parentA, parentB sim.EntityID, ok bool)

// reproductionAncestors collects every ancestor reachable within the bounded
// kin walk, mapped to its minimum depth (the actor itself at depth 0).
func reproductionAncestors(lookup ReproductionParentLookup, root sim.EntityID) map[sim.EntityID]int {
	out := map[sim.EntityID]int{root: 0}
	frontier := map[sim.EntityID]struct{}{root: {}}
	for depth := 1; depth <= ReproductionKinWalkLimit && len(frontier) > 0; depth++ {
		next := make(map[sim.EntityID]struct{})
		for id := range frontier {
			parentA, parentB, ok := lookup(id)
			if !ok {
				continue
			}
			for _, parent := range []sim.EntityID{parentA, parentB} {
				if parent == 0 {
					continue
				}
				if _, seen := out[parent]; !seen {
					out[parent] = depth
					next[parent] = struct{}{}
				}
			}
		}
		frontier = next
	}
	return out
}

// ReproductionKinDistance returns the minimal ancestor-walk distance between
// two actors (parent-child 1, siblings 2, grandparent-grandchild 2, cousins
// 4) and whether they are related within the frozen ≤10-ref walk. The walk
// is derived only from genome parent references; it is never hand-declared.
func ReproductionKinDistance(lookup ReproductionParentLookup, a, b sim.EntityID) (int, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	if a == b {
		return 0, true
	}
	ancestorsA := reproductionAncestors(lookup, a)
	ancestorsB := reproductionAncestors(lookup, b)
	best, found := 0, false
	for id, depthA := range ancestorsA {
		if depthB, ok := ancestorsB[id]; ok {
			if depth := depthA + depthB; depth <= ReproductionKinWalkLimit && (!found || depth < best) {
				best, found = depth, true
			}
		}
	}
	return best, found
}

// ReproductionKinProhibited reports the frozen inbreeding rule: related
// pairs sharing an ancestor at walk depth ≤2 may not reproduce.
func ReproductionKinProhibited(distance int, related bool) bool {
	return related && distance <= ReproductionKinProhibitedDepth
}

// ReproductionConsentAllows is the frozen consent formula, evaluated by the
// consenter from its own private state, the PUBLIC derived kin distance,
// and the requester's self-reported claims — never the requester's truth:
// own granary≥3 ∧ own k≥2 ∧ own energy≥3 ∧ reported k≥2 ∧ reported
// granary≥3 ∧ (kin-distance≥3 ∨ unrelated).
func ReproductionConsentAllows(ownGranary, ownCapital, ownEnergy, reportedCapital, reportedGranary int64, kinAllowed bool) bool {
	return ownGranary >= ReproductionConsentMinGranary &&
		ownCapital >= ReproductionConsentMinCapital &&
		ownEnergy >= ReproductionConsentMinEnergy &&
		reportedCapital >= ReproductionConsentMinCapital &&
		reportedGranary >= ReproductionConsentMinGranary &&
		kinAllowed
}

// ReproductionSurplusGateAllows is the frozen runner-side surplus gate on
// fresh truth, both parents: alive ∧ k≥2 ∧ granary≥3 ∧ energy≥3 ∧ kin-ok ∧
// same patch ∧ births<16. Its margins exactly preserve the frozen ticket-13
// indicator.
func ReproductionSurplusGateAllows(proposer, consenter ReproductionActorState, kinAllowed bool, births int64) bool {
	if births < 0 || births >= ReproductionMaxBirths {
		return false
	}
	return reproductionAlive(proposer) && reproductionAlive(consenter) &&
		proposer.Worksite.Capital >= ReproductionConsentMinCapital && consenter.Worksite.Capital >= ReproductionConsentMinCapital &&
		proposer.Granary.Stock >= ReproductionConsentMinGranary && consenter.Granary.Stock >= ReproductionConsentMinGranary &&
		proposer.Body.Energy >= ReproductionConsentMinEnergy && consenter.Body.Energy >= ReproductionConsentMinEnergy &&
		kinAllowed &&
		proposer.Bag.Source == consenter.Bag.Source
}

// ReproductionCrossover records the exact heredity outcome of one birth's
// seven draws, including clamped no-op mutations.
type ReproductionCrossover struct {
	LocusM, LocusY, LocusW       int64
	MutatedM, MutatedY, MutatedW bool
	ClampedM, ClampedY, ClampedW bool
	Lifetime                     int64
}

// ReproductionChildGenome applies the frozen heredity to exactly seven
// draws: child locus = one parent's allele per locus (crossover draw mod 2:
// 0 proposer, 1 consenter), then per-locus mutation draw mutating iff
// draw mod 16 == 0 with direction (draw div 16) mod 2 and allele ±1 clamped
// into [5,7]; lifetime = 96 + draw mod 48 ∈ [96,143]. The genome row carries
// the proposer as parent-a and the consenter as parent-b, the birth hour,
// the derived death hour, died-hour -1, and zero birth-granary-paid.
func ReproductionChildGenome(proposerID, consenterID sim.EntityID, proposer, consenter ReproductionGenomeState, birthHour int, draws []uint64) (ReproductionGenomeState, ReproductionCrossover, error) {
	if !reproductionActorEntity(proposerID) || !reproductionActorEntity(consenterID) || proposerID == consenterID ||
		!validReproductionGenome(proposerID, proposer) || !validReproductionGenome(consenterID, consenter) ||
		birthHour < 0 || birthHour >= ReproductionHorizonHours || len(draws) != ReproductionDrawsPerBirth {
		return ReproductionGenomeState{}, ReproductionCrossover{}, ErrReproductionContract
	}
	var cross ReproductionCrossover
	allele := func(proposerAllele, consenterAllele int64, crossoverDraw, mutationDraw uint64) (int64, bool, bool) {
		value := proposerAllele
		if crossoverDraw%2 == 1 {
			value = consenterAllele
		}
		mutated := mutationDraw%16 == 0
		clamped := false
		if mutated {
			if (mutationDraw/16)%2 == 0 {
				value--
			} else {
				value++
			}
			if value < ReproductionGenomeLocusMin {
				value, clamped = ReproductionGenomeLocusMin, true
			}
			if value > ReproductionGenomeLocusMax {
				value, clamped = ReproductionGenomeLocusMax, true
			}
		}
		return value, mutated, clamped
	}
	cross.LocusM, cross.MutatedM, cross.ClampedM = allele(proposer.LocusM, consenter.LocusM, draws[0], draws[3])
	cross.LocusY, cross.MutatedY, cross.ClampedY = allele(proposer.LocusY, consenter.LocusY, draws[1], draws[4])
	cross.LocusW, cross.MutatedW, cross.ClampedW = allele(proposer.LocusW, consenter.LocusW, draws[2], draws[5])
	cross.Lifetime = ReproductionLifetimeBase + int64(draws[6]%ReproductionLifetimeSpan)
	genome := ReproductionGenomeState{
		LocusM: cross.LocusM, LocusY: cross.LocusY, LocusW: cross.LocusW,
		ParentA: proposerID, ParentB: consenterID,
		BirthHour: int64(birthHour), DeathHour: int64(birthHour) + cross.Lifetime, DiedHour: ReproductionNeverHour,
	}
	return genome, cross, nil
}

// ReproductionBirthRequest files an addressed request at hour h with
// self-reported claims. Filing needs an alive proposer, no currently pending
// request, and no earlier request this hour (one request per actor-hour);
// it never consults the proposer's truth against the claims.
func ReproductionBirthRequest(hour int, proposerID, addressee sim.EntityID, reportedCapital, reportedGranary int64, proposer ReproductionActorState) (ReproductionActorState, error) {
	if _, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthRequest); err != nil ||
		!reproductionActorEntity(proposerID) || !reproductionActorEntity(addressee) || addressee == proposerID ||
		reportedCapital < 0 || reportedCapital > ReproductionKMax || reportedGranary < 0 || reportedGranary > ReproductionGranaryMax ||
		!validReproductionActor(proposerID, proposer) || !reproductionAlive(proposer) ||
		proposer.Request.Status == ReproductionRequestPending || proposer.Request.LastRequestHour >= int64(hour) {
		return ReproductionActorState{}, ErrReproductionContract
	}
	proposer.Request.Hour = int64(hour)
	proposer.Request.Addressee = addressee
	proposer.Request.ReportedCapital = reportedCapital
	proposer.Request.ReportedGranary = reportedGranary
	proposer.Request.Status = ReproductionRequestPending
	proposer.Request.LastRequestHour = int64(hour)
	if !validReproductionActor(proposerID, proposer) {
		return ReproductionActorState{}, ErrReproductionContract
	}
	return proposer, nil
}

// ReproductionReplyDecision separates the three frozen reply outcomes.
type ReproductionReplyDecision uint8

const (
	ReproductionReplyNone ReproductionReplyDecision = iota
	ReproductionReplyAccept
	// ReproductionReplyRefuse is a typed refusal that mutates only the two
	// request rows.
	ReproductionReplyRefuse
	// ReproductionReplyRefuseSilent is the frozen death-between-phases
	// refusal: typed, zero mutation, no proposal.
	ReproductionReplyRefuseSilent
)

// ReproductionRefusalReason is the typed refusal taxonomy of the frozen
// negative fixtures.
type ReproductionRefusalReason uint8

const (
	ReproductionRefusalNone           ReproductionRefusalReason = iota
	ReproductionRefusalNotPending                               // reply without a matching pending request
	ReproductionRefusalDuplicateReply                           // consenter already replied this hour
	ReproductionRefusalAddresseeDead
	ReproductionRefusalProposerDead
	ReproductionRefusalConsentOwn     // consenter's own state fails the consent formula
	ReproductionRefusalConsentClaims  // reported claims fail the consent formula
	ReproductionRefusalKin            // kin walk distance ≤ 2
	ReproductionRefusalBirthCap       // births = 16
	ReproductionRefusalCrossPatch     // parents on different patches
	ReproductionRefusalGateThresholds // fresh-truth gate thresholds unmet
)

// ReproductionRefusalError carries the typed reason of an invalid reply
// call (zero mutation: no matching pending request, duplicate reply). It
// matches ErrReproductionContract for the package's error convention.
type ReproductionRefusalError struct{ Reason ReproductionRefusalReason }

func (e *ReproductionRefusalError) Error() string {
	return fmt.Sprintf("%s: refusal reason %d", ErrReproductionContract, e.Reason)
}

func (e *ReproductionRefusalError) Is(target error) bool { return target == ErrReproductionContract }

// ReproductionReplyResult carries the decided reply: the frozen outcome, the
// typed refusal reason, both parents' after-states, and the newborn on
// acceptance.
type ReproductionReplyResult struct {
	Decision  ReproductionReplyDecision
	Reason    ReproductionRefusalReason
	Proposer  ReproductionActorState
	Consenter ReproductionActorState
	NewbornID sim.EntityID
	Newborn   ReproductionActorState
}

// ReproductionBirthReply evaluates the frozen reply at hour h from fresh
// snapshots. Structural mismatches (no matching pending request, duplicate
// reply) are invalid calls with zero mutation. A dead proposer or consenter
// yields the zero-mutation typed refusal. Otherwise the consent formula runs
// on own truth plus reported claims, then the runner surplus gate on truth;
// any failure refuses by writing only the request status and the
// consenter's last-reply-hour (one reply per actor-hour). Acceptance is one
// atomic birth: both parents pay, the newborn is built, and the request is
// consumed as accepted. kinAllowed must be the trusted runner's kin-walk
// verdict for the pair.
func ReproductionBirthReply(hour int, births int64, kinAllowed bool, draws []uint64, proposerID, consenterID sim.EntityID, proposer, consenter ReproductionActorState) (ReproductionReplyResult, error) {
	if _, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthReply); err != nil ||
		!reproductionActorEntity(proposerID) || !reproductionActorEntity(consenterID) || proposerID == consenterID ||
		!validReproductionActor(proposerID, proposer) || !validReproductionActor(consenterID, consenter) {
		return ReproductionReplyResult{}, ErrReproductionContract
	}
	request := proposer.Request
	if request.Status != ReproductionRequestPending || request.Hour != int64(hour) || request.Addressee != consenterID {
		return ReproductionReplyResult{}, &ReproductionRefusalError{Reason: ReproductionRefusalNotPending}
	}
	if consenter.Request.LastReplyHour >= int64(hour) {
		return ReproductionReplyResult{}, &ReproductionRefusalError{Reason: ReproductionRefusalDuplicateReply}
	}
	refuse := func(decision ReproductionReplyDecision, reason ReproductionRefusalReason) (ReproductionReplyResult, error) {
		result := ReproductionReplyResult{Decision: decision, Reason: reason, Proposer: proposer, Consenter: consenter}
		if decision == ReproductionReplyRefuse {
			result.Proposer.Request.Status = ReproductionRequestRefused
			result.Consenter.Request.LastReplyHour = int64(hour)
			if !validReproductionActor(proposerID, result.Proposer) || !validReproductionActor(consenterID, result.Consenter) {
				return ReproductionReplyResult{}, ErrReproductionContract
			}
		}
		return result, nil
	}
	if !reproductionAlive(consenter) {
		return refuse(ReproductionReplyRefuseSilent, ReproductionRefusalAddresseeDead)
	}
	if !reproductionAlive(proposer) {
		return refuse(ReproductionReplyRefuseSilent, ReproductionRefusalProposerDead)
	}
	// The frozen consent formula in its exact conjunct order: the
	// consenter's own private state first, then the reported claims, then
	// the public kin distance.
	switch {
	case consenter.Granary.Stock < ReproductionConsentMinGranary || consenter.Worksite.Capital < ReproductionConsentMinCapital || consenter.Body.Energy < ReproductionConsentMinEnergy:
		return refuse(ReproductionReplyRefuse, ReproductionRefusalConsentOwn)
	case request.ReportedCapital < ReproductionConsentMinCapital || request.ReportedGranary < ReproductionConsentMinGranary:
		return refuse(ReproductionReplyRefuse, ReproductionRefusalConsentClaims)
	case !kinAllowed:
		return refuse(ReproductionReplyRefuse, ReproductionRefusalKin)
	}
	// The runner surplus gate on fresh truth, both parents.
	if births < 0 || births >= ReproductionMaxBirths {
		return refuse(ReproductionReplyRefuse, ReproductionRefusalBirthCap)
	}
	if !ReproductionSurplusGateAllows(proposer, consenter, kinAllowed, births) {
		if proposer.Bag.Source != consenter.Bag.Source {
			return refuse(ReproductionReplyRefuse, ReproductionRefusalCrossPatch)
		}
		return refuse(ReproductionReplyRefuse, ReproductionRefusalGateThresholds)
	}
	newbornID, err := ReproductionNewbornID(births)
	if err != nil {
		return ReproductionReplyResult{}, err
	}
	child, _, err := ReproductionChildGenome(proposerID, consenterID, proposer.Genome, consenter.Genome, hour, draws)
	if err != nil {
		return ReproductionReplyResult{}, err
	}
	birth, err := ReproductionBirth(hour, births, kinAllowed, proposerID, consenterID, proposer, consenter, child)
	if err != nil {
		return ReproductionReplyResult{}, err
	}
	birth.Proposer.Request.Status = ReproductionRequestAccepted
	birth.Consenter.Request.LastReplyHour = int64(hour)
	if !validReproductionActor(proposerID, birth.Proposer) || !validReproductionActor(consenterID, birth.Consenter) || !validReproductionActor(newbornID, birth.Newborn) {
		return ReproductionReplyResult{}, ErrReproductionContract
	}
	return ReproductionReplyResult{Decision: ReproductionReplyAccept, Proposer: birth.Proposer, Consenter: birth.Consenter, NewbornID: newbornID, Newborn: birth.Newborn}, nil
}

// ReproductionBirth is the frozen zero-length gestation: the accepted reply
// IS the birth. Each parent pays two granary units and two energy units,
// symmetrically, with the energy booked through the parent's basal-spent
// ledger and the granary units through the parent's genome
// birth-granary-paid accumulator, so both extended conservation identities
// stay exact. The newborn is converted, never created: energy 8, hunger 0,
// all capital zero, home patch = the proposer's. kinAllowed must be the
// trusted runner's kin-walk verdict for the pair.
func ReproductionBirth(hour int, births int64, kinAllowed bool, proposerID, consenterID sim.EntityID, proposer, consenter ReproductionActorState, child ReproductionGenomeState) (ReproductionBirthResult, error) {
	if hour < 0 || hour >= ReproductionHorizonHours || !reproductionActorEntity(proposerID) || !reproductionActorEntity(consenterID) || proposerID == consenterID ||
		!validReproductionActor(proposerID, proposer) || !validReproductionActor(consenterID, consenter) ||
		!validReproductionNewbornGenome(births, child) ||
		child.ParentA != proposerID || child.ParentB != consenterID || child.BirthHour != int64(hour) {
		return ReproductionBirthResult{}, ErrReproductionContract
	}
	if !ReproductionSurplusGateAllows(proposer, consenter, kinAllowed, births) {
		return ReproductionBirthResult{}, ErrReproductionContract
	}
	newbornID, err := ReproductionNewbornID(births)
	if err != nil {
		return ReproductionBirthResult{}, err
	}
	// Affordability is implied by the gate margins (granary≥3, energy≥3)
	// but the cost is checked against the paying rows themselves.
	for _, parent := range []ReproductionActorState{proposer, consenter} {
		if parent.Granary.Stock < ReproductionParentGranaryCost || parent.Body.Energy < ReproductionParentEnergyCost {
			return ReproductionBirthResult{}, ErrReproductionContract
		}
	}
	home := proposer.Bag.Source
	pay := func(a ReproductionActorState) ReproductionActorState {
		a.Granary.Stock -= ReproductionParentGranaryCost
		a.Genome.BirthGranaryPaid += ReproductionParentGranaryCost
		a.Body.Energy -= ReproductionParentEnergyCost
		a.Body.BasalSpent += ReproductionParentEnergyCost
		return a
	}
	proposer, consenter = pay(proposer), pay(consenter)
	newborn := ReproductionActorState{
		Body:     ReproductionBodyState{Energy: ReproductionNewbornEnergy, LastGatherHour: ReproductionNeverHour},
		Bag:      ReproductionBagState{Source: home},
		Worksite: ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour},
		Granary:  ReproductionGranaryState{LastStoredMealHour: ReproductionNeverHour},
		Genome:   child,
		Request:  ReproductionIdleRequestState(),
	}
	if !validReproductionActor(proposerID, proposer) || !validReproductionActor(consenterID, consenter) || !validReproductionActor(newbornID, newborn) {
		return ReproductionBirthResult{}, ErrReproductionContract
	}
	return ReproductionBirthResult{ProposerID: proposerID, ConsenterID: consenterID, Proposer: proposer, Consenter: consenter, NewbornID: newbornID, Newborn: newborn}, nil
}

// ReproductionBirthResult is one atomic birth's complete after-state.
type ReproductionBirthResult struct {
	ProposerID, ConsenterID sim.EntityID
	Proposer, Consenter     ReproductionActorState
	NewbornID               sim.EntityID
	Newborn                 ReproductionActorState
}

// validReproductionNewbornGenome validates a child genome against its
// derived newborn ID without materializing the row.
func validReproductionNewbornGenome(births int64, g ReproductionGenomeState) bool {
	id, err := ReproductionNewbornID(births)
	if err != nil {
		return false
	}
	return validReproductionGenome(id, g)
}

// ReproductionBirthExpiry is the frozen world-caused expiry: at the pulse of
// hour h every request still pending from h-1 expires. Anything else is an
// invalid call with zero mutation.
func ReproductionBirthExpiry(hour int, request ReproductionBirthRequestState) (ReproductionBirthRequestState, error) {
	if hour < 1 || hour > ReproductionHorizonHours || request.Status != ReproductionRequestPending || request.Hour != int64(hour-1) {
		return ReproductionBirthRequestState{}, ErrReproductionContract
	}
	request.Status = ReproductionRequestExpired
	return request, nil
}

// ReproductionAgeDeath is the frozen pulse-phase world-caused rule 413: when
// the hour reaches the immutable death-hour, died-hour is written (only from
// -1, only rules 404/413) and the runner issues its EffectStop with the
// v1 stopped-state convention (energy retained). Before the death-hour the
// call is a no-op success; after death the actor is never invoked again.
func ReproductionAgeDeath(hour int, actorID sim.EntityID, genome ReproductionGenomeState) (ReproductionGenomeState, bool, error) {
	if hour < 0 || hour > ReproductionHorizonHours || !reproductionActorEntity(actorID) || !validReproductionGenome(actorID, genome) || genome.DiedHour != ReproductionNeverHour {
		return ReproductionGenomeState{}, false, ErrReproductionContract
	}
	if int64(hour) < genome.DeathHour {
		return genome, false, nil
	}
	genome.DiedHour = int64(hour)
	return genome, true, nil
}

// ReproductionCheckConservation validates every patch, slot, and actor and
// then the aggregate frozen gates, extended verbatim:
//
//	G1: wild produced = gathered + stock, and produced + unrealized =
//	    q·pulses; gathered = consumed + held + invested per patch.
//	G2a: ΣyieldTotal = Σstored + Σstock + Σunrealized + Σbirth-granary-paid.
//	G2b: invested = 2·created + wip; created − decayed = capital.
//	G3: 176 + 8·births + Σconsumed + ΣstoredMeals = Σenergy + Σbasal +
//	    ΣcapLoss (exact under the basal-spent booking of the parent energy
//	    payment), with the per-newborn cohort identity 8 + consumed +
//	    storedMeals = energy + basal + capLoss.
//
// It also verifies the derived reproduction graph: every newborn's parents
// exist among strictly earlier IDs, no parent pair shares an ancestor at
// walk depth ≤2, and founder genomes carry no parents and no paid units.
func ReproductionCheckConservation(patches [ReproductionPatchCount]ReproductionPatchState, slots [ReproductionPatchCount][ReproductionSlotsPerPatch]ReproductionSlotState, founders [ReproductionFounderCount]ReproductionActorState, newborns []ReproductionNewbornState) (ReproductionBalance, error) {
	var b ReproductionBalance
	b.InitialEnergy = ReproductionFounderInitialEnergy * ReproductionFounderCount
	// Pass 1: every actor row and the newborn ID sequence, and the parent
	// map for the derived-kinship re-check.
	parents := make(map[sim.EntityID][2]sim.EntityID, ReproductionFounderCount+len(newborns))
	for i, founder := range founders {
		actor := sim.EntityID(i + 1)
		if !validReproductionActor(actor, founder) {
			return ReproductionBalance{}, ErrReproductionContract
		}
		parents[actor] = [2]sim.EntityID{}
	}
	seen := make(map[sim.EntityID]bool, len(newborns))
	for i, newborn := range newborns {
		if !reproductionActorEntity(newborn.ID) || reproductionFounderEntity(newborn.ID) || seen[newborn.ID] || newborn.ID != sim.EntityID(ReproductionFirstNewbornID+i) {
			return ReproductionBalance{}, ErrReproductionContract
		}
		seen[newborn.ID] = true
		if !validReproductionActor(newborn.ID, newborn.State) {
			return ReproductionBalance{}, ErrReproductionContract
		}
		parents[newborn.ID] = [2]sim.EntityID{newborn.State.Genome.ParentA, newborn.State.Genome.ParentB}
		// Per-newborn cohort identity: 8 + consumed + stored = energy +
		// basal + capLoss.
		if ReproductionNewbornEnergy+newborn.State.Body.Consumed+newborn.State.Granary.StoredMeals != newborn.State.Body.Energy+newborn.State.Body.BasalSpent+newborn.State.Body.CapLost {
			return ReproductionBalance{}, ErrReproductionContract
		}
	}
	b.Births = int64(len(newborns))
	// Pass 2: lineage — parents must exist among strictly earlier IDs and
	// no parent pair may share an ancestor at walk depth ≤2.
	lookup := ReproductionParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		p, ok := parents[id]
		return p[0], p[1], ok
	})
	for id, pair := range parents {
		if pair[0] == 0 {
			continue // founder
		}
		for _, parent := range pair {
			if _, ok := parents[parent]; !ok || parent >= id {
				return ReproductionBalance{}, ErrReproductionContract
			}
		}
		if distance, related := ReproductionKinDistance(lookup, pair[0], pair[1]); ReproductionKinProhibited(distance, related) {
			return ReproductionBalance{}, ErrReproductionContract
		}
	}
	// Pass 3: patch ledgers and per-patch intake identity over the actors
	// whose home is that patch.
	for i, p := range patches {
		if !reproductionStock(p, slots[i]) {
			return ReproductionBalance{}, ErrReproductionContract
		}
		b.Produced += p.Produced
		b.Unrealized += p.Unrealized
		var gathered, consumed, held, invested int64
		for _, slot := range slots[i] {
			gathered += slot.Gathered
			b.Stock += slot.Stock
		}
		sum := func(a ReproductionActorState) {
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
			b.BirthGranaryPaid += a.Genome.BirthGranaryPaid
		}
		for j := range founders {
			if founders[j].Bag.Source == sim.EntityID(1001+i) {
				sum(founders[j])
			}
		}
		for _, newborn := range newborns {
			if newborn.State.Bag.Source == sim.EntityID(1001+i) {
				sum(newborn.State)
			}
		}
		if gathered != consumed+held+invested {
			return ReproductionBalance{}, ErrReproductionContract
		}
		b.Gathered += gathered
		b.Consumed += consumed
		b.Held += held
		b.Invested += invested
	}
	// Pass 4: aggregate identities, G1, G2b, G2a extended, G3 extended.
	if b.Gathered != b.Consumed+b.Held+b.Invested || b.Produced != b.Gathered+b.Stock ||
		b.Invested != ReproductionPointCostWip*b.PointsCreated+b.Wip || b.PointsCreated-b.PointsDecayed != b.Capital ||
		b.YieldTotal != b.StoredMeals+b.GranaryStock+b.YieldUnrealized+b.BirthGranaryPaid ||
		b.InitialEnergy+ReproductionNewbornEnergy*b.Births+b.Consumed+b.StoredMeals != b.Energy+b.BasalSpent+b.EnergyCapLost {
		return ReproductionBalance{}, ErrReproductionContract
	}
	return b, nil
}
