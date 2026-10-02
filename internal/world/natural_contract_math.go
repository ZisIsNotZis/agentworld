package world

import (
	"agentworld/internal/sim"
	"fmt"
)

// Natural v5 state is plain exact integers. No function here mutates its
// inputs; every failure returns zero states and leaves the arguments
// untouched. Worksite, granary, genome, and birth-request are private
// per-actor components; the patch and its eight slots are shared fixtures
// whose tenure (mode + owner + cultivated flag) is privileged state. Every
// created food unit is accounted by the frozen identities:
//
//	G1  per patch-hour: R = cultivatedGranted + wildFilled + regenUnrealized
//	    lifetime:       R·pulses = gathered + wildStock + granted + unrealized
//	    and per patch:  gathered = consumed + held + invested, with
//	                    gathered + standing stock = the wild share produced
//	G2a per granary:    cultivatedTotal = stock + storedMeals + spoiled
//	                    + birthGranaryPaid
//	G2b per actor:      invested = 2·created + wip; created − decayed = k;
//	    land:           tenuredSlots(patch) = Σk over ALL tenure holders
//	                    (dead included) ≤ 8, each tenured slot's owner an
//	                    existing actor whose home patch is that patch
//	G3  energy:         11·8P + 8·births + Σconsumed + ΣstoredMeals =
//	                    Σenergy + ΣbasalSpent + ΣcapLoss, and per newborn
//	                    cohort 8 + consumed + stored = energy + basal + cap
type NaturalPatchState struct{ Yield, Pulses, Produced, Granted, Unrealized int64 }

// NaturalTenureMode is the slot tenure mode: wild (open-access standing
// stock) or cultivated (grants into the owner's granary).
type NaturalTenureMode int64

const (
	NaturalTenureWild       NaturalTenureMode = iota // 0
	NaturalTenureCultivated                          // 1
)

type NaturalSlotState struct {
	Stock, Gathered int64
	Mode            NaturalTenureMode
	Owner           sim.EntityID // zero iff wild
	Cultivated      int64        // 0/1 flag; must equal Mode (G2b consistency)
}
type NaturalBagState struct {
	Units  int64
	Source sim.EntityID // the holder's home patch, always present
}
type NaturalBodyState struct {
	Energy, Hunger, BasalSpent, CapLost, Consumed int64 // Consumed counts wild home-patch meals
	LastGatherHour                                int64
	BasalDebt                                     int64 // basal carry: debt+=6, drained locus-m per energy unit
}
type NaturalWorksiteState struct {
	Capital, Wip, WearDebt, InvestedUnits, PointsCreated, PointsDecayed int64
	LastBuildHour                                                       int64
}
type NaturalGranaryState struct {
	Stock, CultivatedTotal, StoredMeals int64
	LastStoredMealHour                  int64
	SpoilDebt                           int64 // spoilage carry: debt+=stock, drained 24 per rotted unit
	Spoiled                             int64
}
type NaturalGenomeState struct {
	LocusM, LocusY, LocusW         int64        // alleles in [5,7], neutral 6, immutable post-creation
	ParentA, ParentB               sim.EntityID // zero for founders; rule 511's allocation only
	BirthHour, DeathHour, DiedHour int64        // DiedHour -1 = alive; starvation/age-death write only this
	BirthGranaryPaid               int64        // granary units this actor has paid toward births
}
type NaturalBirthRequestState struct {
	Hour            int64        // hour of the most recent filing, -1 if never
	Addressee       sim.EntityID // zero only while never-filed
	ReportedCapital int64        // self-reported k claim, never requester truth
	ReportedGranary int64        // self-reported granary claim, never requester truth
	Status          NaturalRequestStatus
	LastRequestHour int64
	LastReplyHour   int64
}

// NaturalActorState is one actor's six component rows read at one version.
type NaturalActorState struct {
	Body     NaturalBodyState
	Bag      NaturalBagState
	Worksite NaturalWorksiteState
	Granary  NaturalGranaryState
	Genome   NaturalGenomeState
	Request  NaturalBirthRequestState
}

// NaturalNewbornState pairs an allocator-fresh newborn ID with its complete
// state.
type NaturalNewbornState struct {
	ID    sim.EntityID
	State NaturalActorState
}

type NaturalRequestStatus int64

const (
	NaturalRequestIdle     NaturalRequestStatus = iota // 0: never filed
	NaturalRequestPending                              // 1
	NaturalRequestAccepted                             // 2
	NaturalRequestRefused                              // 3
	NaturalRequestExpired                              // 4
)

type NaturalRegenSplitResult struct{ Cultivated, Wild, Unrealized int64 }
type NaturalMeal struct{ EnergyGained, EnergyCapLost int64 }
type NaturalBasalCost struct{ EnergySpent, UnmetEnergy int64 }
type NaturalBuildResult struct {
	PointsCompleted  int64
	ClaimedSlotIndex int // -1 when no point completed
}
type NaturalGrantResult struct{ Realized, Fallowed int64 }
type NaturalSpoilageResult struct{ Spoiled int64 }
type NaturalWearResult struct {
	PointsDecayed int64
	RevertedSlots []int // ascending slot indices reverted to wild this pulse
}

// NaturalGrantEntitlement is one living owner's regen-split share of the
// patch budget: exactly one granary unit per entitled cultivated slot,
// granted flat by rule 508 (corrigendum: no divisor loop, no locus mints
// units beyond the budget R).
type NaturalGrantEntitlement struct {
	Owner sim.EntityID
	Units int64
}

// NaturalAliveLookup is the trusted runner's owner-liveness view for the
// regen split: only living owners' cultivated slots are entitled.
type NaturalAliveLookup func(sim.EntityID) bool

// NaturalBalance carries the aggregate frozen gates G1, G2a, G2b, and G3.
type NaturalBalance struct {
	Produced, Granted, Unrealized, Gathered, Consumed, Stock, Held, Invested int64
	CultivatedTotal, GranaryStock, StoredMeals, Spoiled                      int64
	Capital, Wip, PointsCreated, PointsDecayed, TenuredSlots                 int64
	InitialEnergy, Energy, BasalSpent, EnergyCapLost                         int64
	BirthGranaryPaid, Births                                                 int64
}

// NaturalFallowError is the typed fallow outcome of a grant aimed at a dead
// (or unknown) owner: zero mutation, no yield. It matches ErrNaturalContract
// for the package's error convention.
type NaturalFallowError struct {
	Owner sim.EntityID
	Units int64
}

func (e *NaturalFallowError) Error() string {
	return fmt.Sprintf("%s: fallow grant of %d units to actor %d", ErrNaturalContract, e.Units, e.Owner)
}
func (e *NaturalFallowError) Is(target error) bool { return target == ErrNaturalContract }

// NaturalBuildRefusalReason is the typed taxonomy of the frozen build
// slot-claim refusals (zero mutation in every case).
type NaturalBuildRefusalReason uint8

const (
	NaturalBuildRefusalNone NaturalBuildRefusalReason = iota
	// NaturalBuildRefusalNoWildSlot: every slot of the home patch is
	// tenured (occupied).
	NaturalBuildRefusalNoWildSlot
	// NaturalBuildRefusalWildAllStocked: wild slots remain but every one
	// holds standing stock.
	NaturalBuildRefusalWildAllStocked
)

// NaturalBuildRefusalError is the typed refusal of a build completion whose
// slot claim cannot be satisfied: zero mutation, nothing converts. It
// matches ErrNaturalContract for the package's error convention.
type NaturalBuildRefusalError struct{ Reason NaturalBuildRefusalReason }

func (e *NaturalBuildRefusalError) Error() string {
	return fmt.Sprintf("%s: build slot-claim refusal reason %d", ErrNaturalContract, e.Reason)
}
func (e *NaturalBuildRefusalError) Is(target error) bool { return target == ErrNaturalContract }

func validNaturalPatch(p NaturalPatchState) bool {
	return p.Yield >= 0 && p.Yield <= NaturalSlotsPerPatch && p.Pulses >= 0 && p.Pulses <= NaturalHorizonHours &&
		p.Produced >= 0 && p.Produced <= NaturalGranaryStockMax && p.Granted >= 0 && p.Granted <= NaturalGranaryStockMax &&
		p.Unrealized >= 0 && p.Unrealized <= NaturalGranaryStockMax &&
		p.Produced+p.Granted+p.Unrealized == p.Yield*p.Pulses
}

func validNaturalSlot(s NaturalSlotState) bool {
	if s.Stock < 0 || s.Stock > 1 || s.Gathered < 0 || s.Gathered > NaturalHorizonHours {
		return false
	}
	if s.Mode != NaturalTenureWild && s.Mode != NaturalTenureCultivated {
		return false
	}
	// G2b tenure consistency: the redundant cultivated flag must equal the
	// mode, the owner ref is present iff cultivated, and a cultivated slot
	// never holds standing stock (nothing ever writes stock to one).
	if s.Cultivated != int64(s.Mode) {
		return false
	}
	if s.Mode == NaturalTenureCultivated {
		return s.Owner != 0 && s.Stock == 0
	}
	return s.Owner == 0
}

func validNaturalBody(b NaturalBodyState) bool {
	return b.Energy >= 0 && b.Energy <= NaturalEnergyCapacity && b.Hunger >= 0 && b.Hunger <= NaturalHungerCapacity &&
		b.BasalSpent >= 0 && b.BasalSpent <= NaturalBasalSpentMax && b.CapLost >= 0 && b.CapLost <= NaturalHorizonHours &&
		b.Consumed >= 0 && b.Consumed <= NaturalHorizonHours &&
		b.LastGatherHour >= NaturalNeverHour && b.LastGatherHour < NaturalHorizonHours &&
		b.BasalDebt >= 0 && b.BasalDebt <= NaturalBasalDebtMax
}

func validNaturalBag(patches int, home sim.EntityID, b NaturalBagState) bool {
	return b.Units >= 0 && b.Units <= NaturalBagCapacity && b.Source == home && naturalPatchEntity(patches, home)
}

func validNaturalWorksite(w NaturalWorksiteState) bool {
	// G2b per-actor ledgers, verbatim v3/v4.
	return w.Capital >= 0 && w.Capital <= NaturalKMax && w.Wip >= 0 && w.Wip <= NaturalWipMax &&
		w.WearDebt >= 0 && w.WearDebt <= NaturalWearDebtMax &&
		w.InvestedUnits >= 0 && w.InvestedUnits <= NaturalHorizonHours &&
		w.PointsCreated >= 0 && w.PointsCreated <= NaturalHorizonHours &&
		w.PointsDecayed >= 0 && w.PointsDecayed <= NaturalHorizonHours &&
		w.LastBuildHour >= NaturalNeverHour && w.LastBuildHour < NaturalHorizonHours &&
		w.InvestedUnits == NaturalPointCostWip*w.PointsCreated+w.Wip &&
		w.PointsCreated-w.PointsDecayed == w.Capital
}

func validNaturalGranary(g NaturalGranaryState) bool {
	return g.Stock >= 0 && g.Stock <= NaturalGranaryStockMax &&
		g.CultivatedTotal >= 0 && g.CultivatedTotal <= NaturalGranaryStockMax &&
		g.StoredMeals >= 0 && g.StoredMeals <= NaturalHorizonHours &&
		g.LastStoredMealHour >= NaturalNeverHour && g.LastStoredMealHour < NaturalHorizonHours &&
		g.SpoilDebt >= 0 && g.SpoilDebt <= NaturalSpoilDebtMax &&
		g.Spoiled >= 0 && g.Spoiled <= NaturalSpoiledMax
}

// validNaturalGenome checks the frozen bounds and lineage shape: founders
// (IDs 1..8P) have absent parents and birth-hour 0; newborns have both
// parents present, distinct, strictly earlier IDs (the allocator is
// monotonic, so the reproduction graph stays acyclic), and a creation
// lifetime in [96,143]. died-hour is either -1 or the death hour itself,
// never below birth-hour.
func validNaturalGenome(patches int, actor sim.EntityID, g NaturalGenomeState) bool {
	locus := func(v int64) bool { return v >= NaturalGenomeLocusMin && v <= NaturalGenomeLocusMax }
	if !locus(g.LocusM) || !locus(g.LocusY) || !locus(g.LocusW) {
		return false
	}
	if g.BirthGranaryPaid < 0 || g.BirthGranaryPaid > NaturalBirthGranaryPaidMax {
		return false
	}
	if naturalFounderEntity(patches, actor) {
		if g.ParentA != 0 || g.ParentB != 0 || g.BirthHour != 0 {
			return false
		}
	} else {
		if !naturalNewbornEntity(patches, actor) || g.ParentA == 0 || g.ParentB == 0 ||
			!naturalActorEntity(patches, g.ParentA) || !naturalActorEntity(patches, g.ParentB) ||
			g.ParentA == g.ParentB || g.ParentA >= actor || g.ParentB >= actor ||
			g.BirthHour < 0 || g.BirthHour >= NaturalHorizonHours {
			return false
		}
	}
	if g.DeathHour < 0 || g.DeathHour > 479 || g.DeathHour-g.BirthHour < NaturalFounderLifetimeMin || g.DeathHour-g.BirthHour > NaturalFounderLifetimeMax {
		return false
	}
	return g.DiedHour == NaturalNeverHour || (g.DiedHour >= g.BirthHour && g.DiedHour <= NaturalHorizonHours)
}

func validNaturalBirthRequest(patches int, owner sim.EntityID, r NaturalBirthRequestState) bool {
	if r.ReportedCapital < 0 || r.ReportedCapital > NaturalKMax || r.ReportedGranary < 0 || r.ReportedGranary > NaturalGranaryStockMax {
		return false
	}
	hourOK := func(h int64) bool { return h >= NaturalNeverHour && h < NaturalHorizonHours }
	if !hourOK(r.Hour) || !hourOK(r.LastRequestHour) || !hourOK(r.LastReplyHour) || r.Status < NaturalRequestIdle || r.Status > NaturalRequestExpired {
		return false
	}
	if r.Status == NaturalRequestIdle {
		return r.Hour == NaturalNeverHour && r.Addressee == 0 && r.ReportedCapital == 0 && r.ReportedGranary == 0 && r.LastRequestHour == NaturalNeverHour
	}
	// Every filed row retains its latest request for the privileged audit:
	// the request hour is the last-request hour and the addressee is a
	// valid actor other than the owner.
	return r.Hour >= 0 && r.Hour == r.LastRequestHour && r.Addressee != 0 && r.Addressee != owner && naturalActorEntity(patches, r.Addressee)
}

// naturalAlive reports the v4-verbatim liveness invariant: died-hour -1
// with positive energy. Dead actors take no actions; their granaries still
// spoil and their capital still wears.
func naturalAlive(a NaturalActorState) bool {
	return a.Genome.DiedHour == NaturalNeverHour && a.Body.Energy > 0
}

// validNaturalActor checks the six rows of one actor: home-patch provenance
// (founder homes are ID-placed; newborn homes are carried), never-gathered
// coherence, the per-actor G2a granary identity with the spoilage and birth
// ledgers, the per-actor G3 energy identity with the creation base (11
// founder, 8 newborn), and the alive invariant died-hour=-1 ⇒ energy>0.
func validNaturalActor(patches int, actor sim.EntityID, a NaturalActorState) bool {
	if !naturalActorEntity(patches, actor) || !validNaturalBody(a.Body) || !validNaturalWorksite(a.Worksite) ||
		!validNaturalGranary(a.Granary) || !validNaturalGenome(patches, actor, a.Genome) || !validNaturalBirthRequest(patches, actor, a.Request) {
		return false
	}
	home := a.Bag.Source
	if naturalFounderEntity(patches, actor) {
		expected, err := NaturalFounderPatchID(patches, actor)
		if err != nil || home != expected {
			return false
		}
	}
	if !validNaturalBag(patches, home, a.Bag) {
		return false
	}
	if a.Body.LastGatherHour == NaturalNeverHour && (a.Bag.Units != 0 || a.Body.Consumed != 0) {
		return false
	}
	base := NaturalFounderInitialEnergy
	if !naturalFounderEntity(patches, actor) {
		base = NaturalNewbornEnergy
	}
	// G2a extended: every cultivated unit received is standing, eaten,
	// spoiled, or paid toward a birth.
	if a.Granary.CultivatedTotal != a.Granary.Stock+a.Granary.StoredMeals+a.Granary.Spoiled+a.Genome.BirthGranaryPaid {
		return false
	}
	// G3 per actor.
	return base+a.Body.Consumed+a.Granary.StoredMeals == a.Body.Energy+a.Body.BasalSpent+a.Body.CapLost
}

// naturalStock verifies the patch production ledger against its slots
// (G1): no slot supplies units before they are produced, and the wild share
// produced equals gathered plus current standing stock.
func naturalStock(p NaturalPatchState, slots [NaturalSlotsPerPatch]NaturalSlotState) bool {
	if !validNaturalPatch(p) {
		return false
	}
	var total int64
	for _, slot := range slots {
		if !validNaturalSlot(slot) || slot.Gathered+slot.Stock > p.Pulses {
			return false
		}
		total += slot.Gathered + slot.Stock
	}
	return total == p.Produced
}

// NaturalRegenSplit distributes the patch regeneration budget R at the
// pulse of hour h, replacing v4's produce/yield stages: cultivated slots of
// LIVING owners first (one granary unit each, ascending slot order), then
// vacant wild slots fill ascending (exactly the v1 produce-wild rule), and
// the remainder is typed regen-unrealized. Fallow slots (cultivated, owner
// dead or unknown) are skipped: grant 0, budget term unrealized, until wear
// reverts them. The per-patch-hour identity R = cultivated + wildFilled +
// unrealized holds exactly for every budget and occupancy, and grants are
// flat, so no locus can mint units beyond R. The grants are returned as one
// entitlement per owner, ordered by lowest entitled slot index, for rule
// 508's flat granary writes.
func NaturalRegenSplit(patches, hour int, patch NaturalPatchState, slots [NaturalSlotsPerPatch]NaturalSlotState, ownerAlive NaturalAliveLookup) (NaturalPatchState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalRegenSplitResult, []NaturalGrantEntitlement, error) {
	if _, err := NaturalHourTime(hour); err != nil || int64(hour) != patch.Pulses || !naturalStock(patch, slots) || ownerAlive == nil {
		return NaturalPatchState{}, slots, NaturalRegenSplitResult{}, nil, ErrNaturalContract
	}
	out := slots
	budget := patch.Yield
	var result NaturalRegenSplitResult
	grants := make([]NaturalGrantEntitlement, 0, NaturalSlotsPerPatch)
	grantIndex := make(map[sim.EntityID]int, NaturalSlotsPerPatch)
	// Pass 1: entitled cultivated slots of living owners, ascending.
	for i := range out {
		if out[i].Mode == NaturalTenureCultivated && budget > 0 && ownerAlive(out[i].Owner) {
			budget--
			result.Cultivated++
			if j, ok := grantIndex[out[i].Owner]; ok {
				grants[j].Units++
			} else {
				grantIndex[out[i].Owner] = len(grants)
				grants = append(grants, NaturalGrantEntitlement{Owner: out[i].Owner, Units: 1})
			}
		}
	}
	// Pass 2: vacant wild slots fill ascending, exactly as v1-v4.
	for i := range out {
		if out[i].Mode == NaturalTenureWild && out[i].Stock == 0 && budget > 0 {
			out[i].Stock++
			budget--
			result.Wild++
		}
	}
	result.Unrealized = budget
	patch.Pulses++
	patch.Produced += result.Wild
	patch.Granted += result.Cultivated
	patch.Unrealized += result.Unrealized
	// Frozen per-patch-hour identity: R = cultivated + wildFilled +
	// unrealized, exact for every budget and occupancy.
	if result.Cultivated+result.Wild+result.Unrealized != patch.Yield {
		return NaturalPatchState{}, slots, NaturalRegenSplitResult{}, nil, ErrNaturalContract
	}
	if !naturalStock(patch, out) {
		return NaturalPatchState{}, slots, NaturalRegenSplitResult{}, nil, ErrNaturalContract
	}
	return patch, out, result, grants, nil
}

// NaturalCultivatedGrant moves a living owner's flat regen entitlement into
// its granary under rule 508: stock and the cultivated-received ledger rise
// together by exactly the granted units. A dead owner is typed fallow:
// zero mutation, no yield (the frozen "grant to a dead owner" negative),
// leaving that budget term unrealized upstream.
func NaturalCultivatedGrant(patches int, ownerID sim.EntityID, a NaturalActorState, units int64) (NaturalActorState, NaturalGrantResult, error) {
	if !naturalActorEntity(patches, ownerID) || !validNaturalActor(patches, ownerID, a) || units < 1 || units > NaturalSlotsPerPatch {
		return NaturalActorState{}, NaturalGrantResult{}, ErrNaturalContract
	}
	if !naturalAlive(a) {
		return NaturalActorState{}, NaturalGrantResult{Fallowed: units}, &NaturalFallowError{Owner: ownerID, Units: units}
	}
	a.Granary.Stock += units
	a.Granary.CultivatedTotal += units
	if !validNaturalActor(patches, ownerID, a) {
		return NaturalActorState{}, NaturalGrantResult{}, ErrNaturalContract
	}
	return a, NaturalGrantResult{Realized: units}, nil
}

// NaturalSpoilage is the declared perishability convention that replaces
// the v4 granary cap: each pulse spoil-debt += stock, and whenever the debt
// reaches S=24 one stock unit rots (stock−1, spoiled+1, debt−=S). Dead
// owners' granaries rot identically, so an inaccessible stock decays away.
// The debt transient stays inside its horizon envelope [0,1367] for
// reachable states; anything larger is forged and rejected with zero
// mutation.
func NaturalSpoilage(patches int, ownerID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalSpoilageResult, error) {
	if !naturalActorEntity(patches, ownerID) || !validNaturalActor(patches, ownerID, a) {
		return NaturalActorState{}, NaturalSpoilageResult{}, ErrNaturalContract
	}
	g := a.Granary
	// An at-rest debt carry is always < S between pulses (the loop below
	// exits only with debt < S), so a carry at or above the period is forged
	// even when no unit rots this pulse.
	if g.SpoilDebt >= NaturalSpoilagePeriod {
		return NaturalActorState{}, NaturalSpoilageResult{}, ErrNaturalContract
	}
	g.SpoilDebt += g.Stock
	if g.SpoilDebt > NaturalSpoilDebtMax {
		return NaturalActorState{}, NaturalSpoilageResult{}, ErrNaturalContract
	}
	var result NaturalSpoilageResult
	for g.SpoilDebt >= NaturalSpoilagePeriod {
		if g.Stock == 0 { // unreachable for reachable states: carry enters only through stock
			return NaturalActorState{}, NaturalSpoilageResult{}, ErrNaturalContract
		}
		g.Stock--
		g.Spoiled++
		g.SpoilDebt -= NaturalSpoilagePeriod
		result.Spoiled++
	}
	a.Granary = g
	if !validNaturalActor(patches, ownerID, a) {
		return NaturalActorState{}, NaturalSpoilageResult{}, ErrNaturalContract
	}
	return a, result, nil
}

// NaturalWear accrues depreciation against every owner, dead owners
// included, through the v4-verbatim divisor loop: debt += capital; while
// debt ≥ locus-w ∧ capital ≥ 1: capital−1, decayed+1, debt −= locus-w. Each
// decayed capital point reverts the owner's LOWEST-INDEX cultivated slot on
// its home patch back to wild with stock 0 in the same atomic pulse; a
// capital point without a tenured slot is an inconsistent (forged) state.
// The transient debt never exceeds 14 for reachable states; anything larger
// is rejected with zero mutation.
func NaturalWear(patches int, ownerID sim.EntityID, a NaturalActorState, homePatch sim.EntityID, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalWearResult, error) {
	if !naturalActorEntity(patches, ownerID) || !validNaturalActor(patches, ownerID, a) {
		return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
	}
	// Cultivation is home-patch-only: the slot array must be the owner's
	// own home patch, so a cross-patch revert is an invalid call.
	if homePatch != a.Bag.Source {
		return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
	}
	if _, err := NaturalPatchID(patches, int(a.Bag.Source)-NaturalPatchIDBase); err != nil {
		return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
	}
	out := homeSlots
	w := a.Worksite
	w.WearDebt += w.Capital
	if w.WearDebt > NaturalWearDebtMax {
		return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
	}
	locus := a.Genome.LocusW
	var result NaturalWearResult
	for w.WearDebt >= locus {
		if w.Capital == 0 { // unreachable for reachable states: guards [0,KMax]
			return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
		}
		w.Capital--
		w.WearDebt -= locus
		w.PointsDecayed++
		result.PointsDecayed++
		reverted := false
		for i := range out {
			if out[i].Mode == NaturalTenureCultivated && out[i].Owner == ownerID {
				// Lowest-index slot of THIS owner reverts to wild with
				// stock 0; its gather history is retained.
				out[i] = NaturalSlotState{Gathered: out[i].Gathered}
				result.RevertedSlots = append(result.RevertedSlots, i)
				reverted = true
				break
			}
		}
		if !reverted { // G2b violated: a capital point without its slot
			return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
		}
	}
	a.Worksite = w
	if !validNaturalActor(patches, ownerID, a) {
		return NaturalActorState{}, homeSlots, NaturalWearResult{}, ErrNaturalContract
	}
	return a, out, result, nil
}

// NaturalBasal advances elapsed whole hours through the v4-verbatim
// genome-modulated divisor loop: debt += 6; while debt ≥ locus-m ∧ energy ≥
// 1: energy−1, spent+1, debt −= locus-m (rate 6/m per hour; byte-identical
// to v3 at the neutral 6). Dead actors are stopped and never invoked. When
// the loop drains the last energy unit the actor starves: died-hour is
// written (only rule 504 may write it, only from -1 down to the current
// hour) and the runner issues its EffectStop. Hunger accrues exactly as v3.
func NaturalBasal(patches, hour int, actorID sim.EntityID, a NaturalActorState, elapsedHours int64) (NaturalActorState, NaturalBasalCost, bool, error) {
	if _, err := NaturalHourTime(hour); err != nil || !naturalActorEntity(patches, actorID) || !validNaturalActor(patches, actorID, a) ||
		elapsedHours < 1 || elapsedHours > NaturalHorizonHours || !naturalAlive(a) {
		return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
	}
	// Reachable-state guard: two drains per hour (locus-m 5) plus four
	// units per hour of birth payments can never exceed the widened ledger
	// bound 1008.
	if a.Body.BasalSpent+2*elapsedHours > NaturalBasalSpentMax {
		return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
	}
	m := a.Genome.LocusM
	var result NaturalBasalCost
	// Accrual and drain happen once per elapsed hour so the batched path
	// reproduces the frozen hourly lattice exactly (v3 byte-identity holds
	// at neutral 6 for any elapsedHours, not only 1).
	for h := int64(0); h < elapsedHours; h++ {
		a.Body.BasalDebt += NaturalGenomeDenominator
		if a.Body.BasalDebt > NaturalBasalDebtMax {
			return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
		}
		for a.Body.BasalDebt >= m && a.Body.Energy >= 1 {
			a.Body.Energy--
			a.Body.BasalSpent++
			a.Body.BasalDebt -= m
			result.EnergySpent++
		}
	}
	result.UnmetEnergy = elapsedHours - result.EnergySpent
	if elapsedHours >= NaturalHungerCapacity-a.Body.Hunger {
		a.Body.Hunger = NaturalHungerCapacity
	} else {
		a.Body.Hunger += elapsedHours
	}
	starved := a.Body.Energy == 0
	if starved {
		a.Genome.DiedHour = int64(hour)
	}
	if !validNaturalActor(patches, actorID, a) {
		return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
	}
	return a, result, starved, nil
}

// NaturalGather transfers one identified home-patch slot unit to the actor's
// bag, v1-v4-identical among the living: dead actors take no actions. The
// bag's home-patch source is already present and unchanged.
func NaturalGather(patches int, patchID, slotID, actorID sim.EntityID, hour int, slot NaturalSlotState, a NaturalActorState) (NaturalSlotState, NaturalActorState, error) {
	home := a.Bag.Source
	if home != patchID || hour < 0 || hour >= NaturalHorizonHours || !validNaturalSlot(slot) || slot.Stock != 1 ||
		slot.Gathered > int64(hour) || !validNaturalActor(patches, actorID, a) || !naturalAlive(a) || a.Bag.Units != 0 || a.Body.LastGatherHour >= int64(hour) {
		return NaturalSlotState{}, NaturalActorState{}, ErrNaturalContract
	}
	index := int(patchID - NaturalPatchIDBase)
	first, err := NaturalSlotID(patches, index, 0)
	if err != nil {
		return NaturalSlotState{}, NaturalActorState{}, ErrNaturalContract
	}
	last, err := NaturalSlotID(patches, index, NaturalSlotsPerPatch-1)
	if err != nil || slotID < first || slotID > last {
		return NaturalSlotState{}, NaturalActorState{}, ErrNaturalContract
	}
	slot.Stock, slot.Gathered = 0, slot.Gathered+1
	a.Bag.Units = 1
	a.Body.LastGatherHour = int64(hour)
	if !validNaturalActor(patches, actorID, a) {
		return NaturalSlotState{}, NaturalActorState{}, ErrNaturalContract
	}
	return slot, a, nil
}

// NaturalConsume eats the held wild unit at the bag decision with v1-v4
// meal semantics: +1 energy, counted spill at the cap-12 ceiling, -1
// hunger.
func NaturalConsume(patches int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalMeal, error) {
	if !naturalActorEntity(patches, actorID) || !validNaturalActor(patches, actorID, a) || !naturalAlive(a) || a.Bag.Units != 1 || a.Body.Consumed >= NaturalHorizonHours {
		return NaturalActorState{}, NaturalMeal{}, ErrNaturalContract
	}
	a.Bag.Units = 0
	a.Body.Consumed++
	result := NaturalMeal{EnergyGained: 1}
	if a.Body.Energy == NaturalEnergyCapacity {
		a.Body.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Body.Energy++
	}
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validNaturalActor(patches, actorID, a) {
		return NaturalActorState{}, NaturalMeal{}, ErrNaturalContract
	}
	return a, result, nil
}

// NaturalBuild consumes the held bag unit into work-in-progress, verbatim
// v4: bag 1→0 (home source retained), wip+1; wip=2 → k+1, wip=0. One
// capital point therefore costs exactly two bag units and two build
// windows. Additionally, at the completion instant the new point claims the
// LOWEST-INDEX free wild slot with stock 0 in the builder's home patch
// (the caller's fresh-snapshot revalidation is a runner duty); a tenured
// slot never holds standing stock. No eligible slot ⇒ typed refusal with
// zero mutation: nothing converts, the bag keeps its unit.
func NaturalBuild(patches, hour int, actorID sim.EntityID, a NaturalActorState, homePatch sim.EntityID, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalBuildResult, error) {
	if _, err := NaturalPhaseTime(hour, NaturalPhaseBagDecision); err != nil {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	// Cultivation is home-patch-only: the claimed slot must live in the
	// builder's own home patch, so a cross-patch claim is an invalid call.
	if homePatch != a.Bag.Source {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	if _, err := NaturalPatchID(patches, int(a.Bag.Source)-NaturalPatchIDBase); err != nil {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	if !naturalActorEntity(patches, actorID) || !validNaturalActor(patches, actorID, a) || !naturalAlive(a) {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	if a.Bag.Units != 1 || a.Worksite.Capital+a.Worksite.Wip >= NaturalKMax || a.Worksite.LastBuildHour >= int64(hour) {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	for _, slot := range homeSlots {
		if !validNaturalSlot(slot) {
			return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
		}
	}
	a.Bag.Units = 0
	w := a.Worksite
	w.InvestedUnits++
	w.LastBuildHour = int64(hour)
	// -1 until a completion actually claims a slot; the zero value 0 is a
	// legitimate slot index and must never stand for "no claim".
	result := NaturalBuildResult{ClaimedSlotIndex: -1}
	w.Wip++
	if w.Wip > NaturalWipMax { // wip=2: one point completes atomically
		w.Wip = 0
		w.Capital++
		w.PointsCreated++
		result.PointsCompleted = 1
	}
	a.Worksite = w
	if result.PointsCompleted == 1 {
		wildSeen := false
		for i := range homeSlots {
			if homeSlots[i].Mode == NaturalTenureWild {
				wildSeen = true
				if homeSlots[i].Stock == 0 {
					homeSlots[i].Mode = NaturalTenureCultivated
					homeSlots[i].Owner = actorID
					homeSlots[i].Cultivated = 1
					result.ClaimedSlotIndex = i
					break
				}
			}
		}
		if result.ClaimedSlotIndex < 0 {
			// Typed refusal, zero mutation: no free wild slot with stock 0.
			reason := NaturalBuildRefusalWildAllStocked
			if !wildSeen {
				reason = NaturalBuildRefusalNoWildSlot
			}
			return NaturalActorState{}, homeSlots, NaturalBuildResult{}, &NaturalBuildRefusalError{Reason: reason}
		}
	}
	if !validNaturalActor(patches, actorID, a) {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	return a, homeSlots, result, nil
}

// NaturalEatStored is the only granary draw, with v1-v4 meal semantics;
// dead owners' granaries are frozen (they only spoil).
func NaturalEatStored(patches, hour int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalMeal, error) {
	if hour < 0 || hour >= NaturalHorizonHours || !naturalActorEntity(patches, actorID) || !validNaturalActor(patches, actorID, a) || !naturalAlive(a) ||
		a.Granary.Stock < 1 || a.Granary.StoredMeals >= NaturalHorizonHours {
		return NaturalActorState{}, NaturalMeal{}, ErrNaturalContract
	}
	a.Granary.Stock--
	a.Granary.StoredMeals++
	a.Granary.LastStoredMealHour = int64(hour)
	result := NaturalMeal{EnergyGained: 1}
	if a.Body.Energy == NaturalEnergyCapacity {
		a.Body.CapLost++
		result.EnergyGained, result.EnergyCapLost = 0, 1
	} else {
		a.Body.Energy++
	}
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validNaturalActor(patches, actorID, a) {
		return NaturalActorState{}, NaturalMeal{}, ErrNaturalContract
	}
	return a, result, nil
}

// NaturalIdleRequestState is the canonical never-filed request row.
func NaturalIdleRequestState() NaturalBirthRequestState {
	return NaturalBirthRequestState{Hour: NaturalNeverHour, LastRequestHour: NaturalNeverHour, LastReplyHour: NaturalNeverHour, Status: NaturalRequestIdle}
}

// NaturalFounderGenome builds the all-neutral founder genome with the
// runner-resolved lifetime (founder stream positions 0..8P−1; the draws are
// runner-owned, this contract takes lifetimes as input). Founders are born
// at hour 0 with absent parents.
func NaturalFounderGenome(patches int, actor sim.EntityID, lifetime int64) (NaturalGenomeState, error) {
	if !naturalFounderEntity(patches, actor) || lifetime < NaturalFounderLifetimeMin || lifetime > NaturalFounderLifetimeMax {
		return NaturalGenomeState{}, ErrNaturalContract
	}
	g := NaturalGenomeState{LocusM: NaturalGenomeNeutral, LocusY: NaturalGenomeNeutral, LocusW: NaturalGenomeNeutral, BirthHour: 0, DeathHour: lifetime, DiedHour: NaturalNeverHour}
	if !validNaturalGenome(patches, actor, g) {
		return NaturalGenomeState{}, ErrNaturalContract
	}
	return g, nil
}

// NaturalFounderState assembles one founder's six rows at hour 0 with the
// v3/v4 initial body values and an idle request row.
func NaturalFounderState(patches int, actor sim.EntityID, lifetime int64) (NaturalActorState, error) {
	genome, err := NaturalFounderGenome(patches, actor, lifetime)
	if err != nil {
		return NaturalActorState{}, err
	}
	home, err := NaturalFounderPatchID(patches, actor)
	if err != nil {
		return NaturalActorState{}, err
	}
	a := NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalFounderInitialEnergy, LastGatherHour: NaturalNeverHour},
		Bag:      NaturalBagState{Source: home},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   genome,
		Request:  NaturalIdleRequestState(),
	}
	if !validNaturalActor(patches, actor, a) {
		return NaturalActorState{}, ErrNaturalContract
	}
	return a, nil
}

// NaturalParentLookup returns an actor's parent-a/parent-b references;
// ok is false for unknown or founder rows.
type NaturalParentLookup func(sim.EntityID) (parentA, parentB sim.EntityID, ok bool)

// naturalAncestors collects every ancestor reachable within the bounded
// kin walk, mapped to its minimum depth (the actor itself at depth 0).
func naturalAncestors(lookup NaturalParentLookup, root sim.EntityID) map[sim.EntityID]int {
	out := map[sim.EntityID]int{root: 0}
	frontier := map[sim.EntityID]struct{}{root: {}}
	for depth := 1; depth <= NaturalKinWalkLimit && len(frontier) > 0; depth++ {
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

// NaturalKinDistance returns the minimal ancestor-walk distance between two
// actors (parent-child 1, siblings 2, grandparent-grandchild 2, cousins 4)
// and whether they are related within the frozen ≤10-ref walk. The walk is
// derived only from genome parent references; it is never hand-declared.
func NaturalKinDistance(lookup NaturalParentLookup, a, b sim.EntityID) (int, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	if a == b {
		return 0, true
	}
	ancestorsA := naturalAncestors(lookup, a)
	ancestorsB := naturalAncestors(lookup, b)
	best, found := 0, false
	for id, depthA := range ancestorsA {
		if depthB, ok := ancestorsB[id]; ok {
			if depth := depthA + depthB; depth <= NaturalKinWalkLimit && (!found || depth < best) {
				best, found = depth, true
			}
		}
	}
	return best, found
}

// NaturalKinProhibited reports the frozen inbreeding rule: related pairs
// sharing an ancestor at walk depth ≤2 may not reproduce.
func NaturalKinProhibited(distance int, related bool) bool {
	return related && distance <= NaturalKinProhibitedDepth
}

// NaturalConsentAllows is the frozen consent formula, evaluated by the
// consenter from its own private state, the PUBLIC derived kin distance,
// and the requester's self-reported claims — never the requester's truth:
// own granary≥3 ∧ own k≥2 ∧ own energy≥3 ∧ reported k≥2 ∧ reported
// granary≥3 ∧ kinAllowed.
func NaturalConsentAllows(ownGranary, ownCapital, ownEnergy, reportedCapital, reportedGranary int64, kinAllowed bool) bool {
	return ownGranary >= NaturalConsentMinGranary &&
		ownCapital >= NaturalConsentMinCapital &&
		ownEnergy >= NaturalConsentMinEnergy &&
		reportedCapital >= NaturalConsentMinCapital &&
		reportedGranary >= NaturalConsentMinGranary &&
		kinAllowed
}

// NaturalSurplusGateAllows is the frozen runner-side surplus gate on fresh
// truth, both parents: alive ∧ k≥2 ∧ granary≥3 ∧ energy≥3 ∧ kin-ok ∧ same
// patch. There is no birth-count term: births are bounded only by ecology
// (P15).
func NaturalSurplusGateAllows(proposer, consenter NaturalActorState, kinAllowed bool) bool {
	return naturalAlive(proposer) && naturalAlive(consenter) &&
		proposer.Worksite.Capital >= NaturalConsentMinCapital && consenter.Worksite.Capital >= NaturalConsentMinCapital &&
		proposer.Granary.Stock >= NaturalConsentMinGranary && consenter.Granary.Stock >= NaturalConsentMinGranary &&
		proposer.Body.Energy >= NaturalConsentMinEnergy && consenter.Body.Energy >= NaturalConsentMinEnergy &&
		kinAllowed &&
		proposer.Bag.Source == consenter.Bag.Source
}

// NaturalCrossover records the exact heredity outcome of one birth's seven
// draws, including clamped no-op mutations.
type NaturalCrossover struct {
	LocusM, LocusY, LocusW       int64
	MutatedM, MutatedY, MutatedW bool
	ClampedM, ClampedY, ClampedW bool
	Lifetime                     int64
}

// NaturalChildGenome applies the frozen v4 heredity to exactly seven draws:
// child locus = one parent's allele per locus (crossover draw mod 2: 0
// proposer, 1 consenter), then per-locus mutation draw mutating iff draw
// mod 16 == 0 with direction (draw div 16) mod 2 and allele ±1 clamped into
// [5,7]; lifetime = 96 + draw mod 48 ∈ [96,143]. The genome row carries the
// proposer as parent-a and the consenter as parent-b, the birth hour, the
// derived death hour, died-hour -1, and zero birth-granary-paid. The child
// genome is validated against its allocator-fresh newborn ID by NaturalBirth
// and NaturalBirthReply, which own that ID.
func NaturalChildGenome(patches int, proposerID, consenterID sim.EntityID, proposer, consenter NaturalGenomeState, birthHour int, draws []uint64) (NaturalGenomeState, NaturalCrossover, error) {
	if !naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, consenterID) || proposerID == consenterID ||
		!validNaturalGenome(patches, proposerID, proposer) || !validNaturalGenome(patches, consenterID, consenter) ||
		birthHour < 0 || birthHour >= NaturalHorizonHours || len(draws) != NaturalDrawsPerBirth {
		return NaturalGenomeState{}, NaturalCrossover{}, ErrNaturalContract
	}
	var cross NaturalCrossover
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
			if value < NaturalGenomeLocusMin {
				value, clamped = NaturalGenomeLocusMin, true
			}
			if value > NaturalGenomeLocusMax {
				value, clamped = NaturalGenomeLocusMax, true
			}
		}
		return value, mutated, clamped
	}
	cross.LocusM, cross.MutatedM, cross.ClampedM = allele(proposer.LocusM, consenter.LocusM, draws[0], draws[3])
	cross.LocusY, cross.MutatedY, cross.ClampedY = allele(proposer.LocusY, consenter.LocusY, draws[1], draws[4])
	cross.LocusW, cross.MutatedW, cross.ClampedW = allele(proposer.LocusW, consenter.LocusW, draws[2], draws[5])
	cross.Lifetime = NaturalLifetimeBase + int64(draws[6]%NaturalLifetimeSpan)
	genome := NaturalGenomeState{
		LocusM: cross.LocusM, LocusY: cross.LocusY, LocusW: cross.LocusW,
		ParentA: proposerID, ParentB: consenterID,
		BirthHour: int64(birthHour), DeathHour: int64(birthHour) + cross.Lifetime, DiedHour: NaturalNeverHour,
	}
	return genome, cross, nil
}

// NaturalBirthRequest files an addressed request at hour h with
// self-reported claims. Filing needs an alive proposer, no currently pending
// request, and no earlier request this hour (one request per actor-hour);
// it never consults the proposer's truth against the claims.
func NaturalBirthRequest(patches, hour int, proposerID, addressee sim.EntityID, reportedCapital, reportedGranary int64, proposer NaturalActorState) (NaturalActorState, error) {
	if _, err := NaturalPhaseTime(hour, NaturalPhaseBirthRequest); err != nil ||
		!naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, addressee) || addressee == proposerID ||
		reportedCapital < 0 || reportedCapital > NaturalKMax || reportedGranary < 0 || reportedGranary > NaturalGranaryStockMax ||
		!validNaturalActor(patches, proposerID, proposer) || !naturalAlive(proposer) ||
		proposer.Request.Status == NaturalRequestPending || proposer.Request.LastRequestHour >= int64(hour) {
		return NaturalActorState{}, ErrNaturalContract
	}
	proposer.Request.Hour = int64(hour)
	proposer.Request.Addressee = addressee
	proposer.Request.ReportedCapital = reportedCapital
	proposer.Request.ReportedGranary = reportedGranary
	proposer.Request.Status = NaturalRequestPending
	proposer.Request.LastRequestHour = int64(hour)
	if !validNaturalActor(patches, proposerID, proposer) {
		return NaturalActorState{}, ErrNaturalContract
	}
	return proposer, nil
}

// NaturalReplyDecision separates the three frozen reply outcomes.
type NaturalReplyDecision uint8

const (
	NaturalReplyNone NaturalReplyDecision = iota
	NaturalReplyAccept
	// NaturalReplyRefuse is a typed refusal that mutates only the two
	// request rows.
	NaturalReplyRefuse
	// NaturalReplyRefuseSilent is the frozen death-between-phases refusal:
	// typed, zero mutation, no proposal.
	NaturalReplyRefuseSilent
)

// NaturalRefusalReason is the typed refusal taxonomy of the frozen negative
// fixtures. There is no birth-cap reason: births are uncapped in v5.
type NaturalRefusalReason uint8

const (
	NaturalRefusalNone           NaturalRefusalReason = iota
	NaturalRefusalNotPending                          // reply without a matching pending request
	NaturalRefusalDuplicateReply                      // consenter already replied this hour
	NaturalRefusalAddresseeDead
	NaturalRefusalProposerDead
	NaturalRefusalConsentOwn     // consenter's own state fails the consent formula
	NaturalRefusalConsentClaims  // reported claims fail the consent formula
	NaturalRefusalKin            // kin walk distance ≤ 2
	NaturalRefusalCrossPatch     // parents on different patches
	NaturalRefusalGateThresholds // fresh-truth gate thresholds unmet
)

// NaturalRefusalError carries the typed reason of an invalid reply call
// (zero mutation: no matching pending request, duplicate reply). It matches
// ErrNaturalContract for the package's error convention.
type NaturalRefusalError struct{ Reason NaturalRefusalReason }

func (e *NaturalRefusalError) Error() string {
	return fmt.Sprintf("%s: refusal reason %d", ErrNaturalContract, e.Reason)
}
func (e *NaturalRefusalError) Is(target error) bool { return target == ErrNaturalContract }

// NaturalReplyResult carries the decided reply: the frozen outcome, the
// typed refusal reason, both parents' after-states, and the
// allocator-fresh newborn on acceptance.
type NaturalReplyResult struct {
	Decision  NaturalReplyDecision
	Reason    NaturalRefusalReason
	Proposer  NaturalActorState
	Consenter NaturalActorState
	NewbornID sim.EntityID
	Newborn   NaturalActorState
}

// NaturalBirthReply evaluates the frozen reply at hour h from fresh
// snapshots. Structural mismatches (no matching pending request, duplicate
// reply) are invalid calls with zero mutation. A dead proposer or consenter
// yields the zero-mutation typed refusal. Otherwise the consent formula runs
// on own truth plus reported claims, then the runner surplus gate on truth;
// any failure refuses by writing only the request status and the
// consenter's last-reply-hour (one reply per actor-hour). Acceptance is one
// atomic birth: both parents pay, the newborn is built under the
// allocator-fresh newbornID, and the request is consumed as accepted.
// kinAllowed must be the trusted runner's kin-walk verdict for the pair.
func NaturalBirthReply(patches, hour int, kinAllowed bool, draws []uint64, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState) (NaturalReplyResult, error) {
	if _, err := NaturalPhaseTime(hour, NaturalPhaseBirthReply); err != nil ||
		!naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, consenterID) || proposerID == consenterID ||
		!validNaturalActor(patches, proposerID, proposer) || !validNaturalActor(patches, consenterID, consenter) {
		return NaturalReplyResult{}, ErrNaturalContract
	}
	request := proposer.Request
	if request.Status != NaturalRequestPending || request.Hour != int64(hour) || request.Addressee != consenterID {
		return NaturalReplyResult{}, &NaturalRefusalError{Reason: NaturalRefusalNotPending}
	}
	if consenter.Request.LastReplyHour >= int64(hour) {
		return NaturalReplyResult{}, &NaturalRefusalError{Reason: NaturalRefusalDuplicateReply}
	}
	refuse := func(decision NaturalReplyDecision, reason NaturalRefusalReason) (NaturalReplyResult, error) {
		result := NaturalReplyResult{Decision: decision, Reason: reason, Proposer: proposer, Consenter: consenter}
		if decision == NaturalReplyRefuse {
			result.Proposer.Request.Status = NaturalRequestRefused
			result.Consenter.Request.LastReplyHour = int64(hour)
			if !validNaturalActor(patches, proposerID, result.Proposer) || !validNaturalActor(patches, consenterID, result.Consenter) {
				return NaturalReplyResult{}, ErrNaturalContract
			}
		}
		return result, nil
	}
	if !naturalAlive(consenter) {
		return refuse(NaturalReplyRefuseSilent, NaturalRefusalAddresseeDead)
	}
	if !naturalAlive(proposer) {
		return refuse(NaturalReplyRefuseSilent, NaturalRefusalProposerDead)
	}
	// The frozen consent formula in its exact conjunct order: the
	// consenter's own private state first, then the reported claims, then
	// the public kin distance.
	switch {
	case consenter.Granary.Stock < NaturalConsentMinGranary || consenter.Worksite.Capital < NaturalConsentMinCapital || consenter.Body.Energy < NaturalConsentMinEnergy:
		return refuse(NaturalReplyRefuse, NaturalRefusalConsentOwn)
	case request.ReportedCapital < NaturalConsentMinCapital || request.ReportedGranary < NaturalConsentMinGranary:
		return refuse(NaturalReplyRefuse, NaturalRefusalConsentClaims)
	case !kinAllowed:
		return refuse(NaturalReplyRefuse, NaturalRefusalKin)
	}
	// The runner surplus gate on fresh truth, both parents.
	if !NaturalSurplusGateAllows(proposer, consenter, kinAllowed) {
		if proposer.Bag.Source != consenter.Bag.Source {
			return refuse(NaturalReplyRefuse, NaturalRefusalCrossPatch)
		}
		return refuse(NaturalReplyRefuse, NaturalRefusalGateThresholds)
	}
	child, _, err := NaturalChildGenome(patches, proposerID, consenterID, proposer.Genome, consenter.Genome, hour, draws)
	if err != nil {
		return NaturalReplyResult{}, err
	}
	birth, err := NaturalBirth(patches, hour, kinAllowed, newbornID, proposerID, consenterID, proposer, consenter, child)
	if err != nil {
		return NaturalReplyResult{}, err
	}
	birth.Proposer.Request.Status = NaturalRequestAccepted
	birth.Consenter.Request.LastReplyHour = int64(hour)
	if !validNaturalActor(patches, proposerID, birth.Proposer) || !validNaturalActor(patches, consenterID, birth.Consenter) || !validNaturalActor(patches, newbornID, birth.Newborn) {
		return NaturalReplyResult{}, ErrNaturalContract
	}
	return NaturalReplyResult{Decision: NaturalReplyAccept, Proposer: birth.Proposer, Consenter: birth.Consenter, NewbornID: newbornID, Newborn: birth.Newborn}, nil
}

// NaturalBirthResult is one atomic birth's complete after-state.
type NaturalBirthResult struct {
	ProposerID, ConsenterID sim.EntityID
	Proposer, Consenter     NaturalActorState
	NewbornID               sim.EntityID
	Newborn                 NaturalActorState
}

// NaturalBirth is the frozen zero-length gestation: the accepted reply IS
// the birth. Each parent pays two granary units and two energy units,
// symmetrically, with the energy booked through the parent's basal-spent
// ledger and the granary units through the parent's genome
// birth-granary-paid accumulator, so both extended conservation identities
// stay exact. The newborn is converted, never created from nothing: energy
// 8, hunger 0, all capital zero, home patch = the proposer's. The newbornID
// must be allocator-fresh (there is no birth-count cap; P15). kinAllowed
// must be the trusted runner's kin-walk verdict for the pair.
func NaturalBirth(patches, hour int, kinAllowed bool, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState, child NaturalGenomeState) (NaturalBirthResult, error) {
	if hour < 0 || hour >= NaturalHorizonHours || !naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, consenterID) || proposerID == consenterID ||
		!naturalNewbornEntity(patches, newbornID) ||
		!validNaturalActor(patches, proposerID, proposer) || !validNaturalActor(patches, consenterID, consenter) ||
		!validNaturalGenome(patches, newbornID, child) ||
		child.ParentA != proposerID || child.ParentB != consenterID || child.BirthHour != int64(hour) {
		return NaturalBirthResult{}, ErrNaturalContract
	}
	if !NaturalSurplusGateAllows(proposer, consenter, kinAllowed) {
		return NaturalBirthResult{}, ErrNaturalContract
	}
	// Affordability is implied by the gate margins (granary≥3, energy≥3)
	// but the cost is checked against the paying rows themselves.
	for _, parent := range []NaturalActorState{proposer, consenter} {
		if parent.Granary.Stock < NaturalParentGranaryCost || parent.Body.Energy < NaturalParentEnergyCost {
			return NaturalBirthResult{}, ErrNaturalContract
		}
	}
	home := proposer.Bag.Source
	pay := func(a NaturalActorState) NaturalActorState {
		a.Granary.Stock -= NaturalParentGranaryCost
		a.Genome.BirthGranaryPaid += NaturalParentGranaryCost
		a.Body.Energy -= NaturalParentEnergyCost
		a.Body.BasalSpent += NaturalParentEnergyCost
		return a
	}
	proposer, consenter = pay(proposer), pay(consenter)
	newborn := NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: NaturalNeverHour},
		Bag:      NaturalBagState{Source: home},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   child,
		Request:  NaturalIdleRequestState(),
	}
	if !validNaturalActor(patches, proposerID, proposer) || !validNaturalActor(patches, consenterID, consenter) || !validNaturalActor(patches, newbornID, newborn) {
		return NaturalBirthResult{}, ErrNaturalContract
	}
	return NaturalBirthResult{ProposerID: proposerID, ConsenterID: consenterID, Proposer: proposer, Consenter: consenter, NewbornID: newbornID, Newborn: newborn}, nil
}

// NaturalBirthExpiry is the frozen world-caused expiry: at the pulse of
// hour h every request still pending from h-1 expires. Anything else is an
// invalid call with zero mutation.
func NaturalBirthExpiry(hour int, request NaturalBirthRequestState) (NaturalBirthRequestState, error) {
	if hour < 1 || hour > NaturalHorizonHours || request.Status != NaturalRequestPending || request.Hour != int64(hour-1) {
		return NaturalBirthRequestState{}, ErrNaturalContract
	}
	request.Status = NaturalRequestExpired
	return request, nil
}

// NaturalAgeDeath is the frozen pulse-phase world-caused rule 514: when the
// hour reaches the immutable death-hour, died-hour is written (only from
// -1, only rules 504/514) and the runner issues its EffectStop with the v1
// stopped-state convention (energy retained). Before the death-hour the
// call is a no-op success; after death the actor is never invoked again.
func NaturalAgeDeath(patches, hour int, actorID sim.EntityID, genome NaturalGenomeState) (NaturalGenomeState, bool, error) {
	if hour < 0 || hour > NaturalHorizonHours || !naturalActorEntity(patches, actorID) || !validNaturalGenome(patches, actorID, genome) || genome.DiedHour != NaturalNeverHour {
		return NaturalGenomeState{}, false, ErrNaturalContract
	}
	if int64(hour) < genome.DeathHour {
		return genome, false, nil
	}
	genome.DiedHour = int64(hour)
	return genome, true, nil
}

// NaturalCheckConservation validates every patch, slot, and actor and then
// the aggregate frozen gates, extended verbatim from v4:
//
//		G1  per patch: produced+granted+unrealized = R·pulses; gathered+stock =
//		    produced; gathered = consumed + held + invested over the actors
//		    whose home is that patch.
//		G2a ΣcultivatedTotal = Σstored + Σstock + Σspoiled + Σbirth-granary-paid;
//	   and per patch the entitlement seam is exact: grants(patch) =
//	   ΣcultivatedTotal over the actors whose home is that patch (rule 508
//	   moves every entitlement into its owner's granary in the same pulse).
//		G2b invested = 2·created + wip; created − decayed = capital; and the
//		    land identity tenuredSlots(patch) = Σk over ALL tenure holders
//		    (dead included) ≤ 8, each tenured slot's owner an existing actor
//		    whose home is that patch. (During a dead owner's decay window the
//		    alive-only equality is weaker by exactly the dead owner's standing
//		    capital; the all-tenure-holders form is the exact invariant.)
//		G3  11·8P + 8·births + Σconsumed + ΣstoredMeals = Σenergy + Σbasal +
//		    ΣcapLoss, with the per-newborn cohort identity 8 + consumed +
//		    storedMeals = energy + basalSpent + capLoss.
//
// It also verifies the derived reproduction graph (every newborn's parents
// exist among strictly earlier IDs, no parent pair shares an ancestor at
// walk depth ≤2, founder genomes carry no parents and no paid units) and
// the allocator cross-check: allocatorLast equals the genesis base when no
// birth has happened and the maximum newborn ID otherwise (mismatch fails
// closed, per the frozen restore discipline).
func NaturalCheckConservation(patches int, patchStates []NaturalPatchState, slotStates [][]NaturalSlotState, founders []NaturalActorState, newborns []NaturalNewbornState, allocatorLast sim.EntityID) (NaturalBalance, error) {
	founderCount, err := NaturalFounderCount(patches)
	if err != nil || len(patchStates) != patches || len(slotStates) != patches || len(founders) != founderCount {
		return NaturalBalance{}, ErrNaturalContract
	}
	genesisLast, err := NaturalHighestFixedEntityID(patches)
	if err != nil {
		return NaturalBalance{}, ErrNaturalContract
	}
	var b NaturalBalance
	b.InitialEnergy = NaturalFounderInitialEnergy * int64(founderCount)
	// Pass 1: every actor row and the parent map for the derived-kinship
	// re-check.
	parents := make(map[sim.EntityID][2]sim.EntityID, founderCount+len(newborns))
	ids := make([]sim.EntityID, 0, founderCount+len(newborns))
	actors := make(map[sim.EntityID]NaturalActorState, founderCount+len(newborns))
	for i, founder := range founders {
		actor := sim.EntityID(i + 1)
		if !validNaturalActor(patches, actor, founder) {
			return NaturalBalance{}, ErrNaturalContract
		}
		parents[actor] = [2]sim.EntityID{}
		ids = append(ids, actor)
		actors[actor] = founder
	}
	seen := make(map[sim.EntityID]bool, len(newborns))
	var lastNewborn sim.EntityID
	for i, newborn := range newborns {
		if !naturalNewbornEntity(patches, newborn.ID) || seen[newborn.ID] || (i > 0 && newborn.ID <= newborns[i-1].ID) {
			return NaturalBalance{}, ErrNaturalContract
		}
		seen[newborn.ID] = true
		lastNewborn = newborn.ID
		if !validNaturalActor(patches, newborn.ID, newborn.State) {
			return NaturalBalance{}, ErrNaturalContract
		}
		parents[newborn.ID] = [2]sim.EntityID{newborn.State.Genome.ParentA, newborn.State.Genome.ParentB}
		ids = append(ids, newborn.ID)
		actors[newborn.ID] = newborn.State
		// Per-newborn cohort identity: 8 + consumed + stored = energy +
		// basal + capLoss.
		if NaturalNewbornEnergy+newborn.State.Body.Consumed+newborn.State.Granary.StoredMeals != newborn.State.Body.Energy+newborn.State.Body.BasalSpent+newborn.State.Body.CapLost {
			return NaturalBalance{}, ErrNaturalContract
		}
	}
	b.Births = int64(len(newborns))
	expectedLast := genesisLast
	if len(newborns) > 0 {
		expectedLast = lastNewborn
	}
	if allocatorLast != expectedLast {
		return NaturalBalance{}, ErrNaturalContract
	}
	// Pass 2: lineage — parents must exist among strictly earlier IDs and
	// no parent pair may share an ancestor at walk depth ≤2.
	lookup := NaturalParentLookup(func(id sim.EntityID) (sim.EntityID, sim.EntityID, bool) {
		p, ok := parents[id]
		return p[0], p[1], ok
	})
	for id, pair := range parents {
		if pair[0] == 0 {
			continue // founder
		}
		for _, parent := range pair {
			if _, ok := parents[parent]; !ok || parent >= id {
				return NaturalBalance{}, ErrNaturalContract
			}
		}
		if distance, related := NaturalKinDistance(lookup, pair[0], pair[1]); NaturalKinProhibited(distance, related) {
			return NaturalBalance{}, ErrNaturalContract
		}
	}
	// Pass 3: patch ledgers, per-patch intake identity, and the land
	// identity over ALL tenure holders (dead included).
	tenuredByOwner := make(map[sim.EntityID]int64, len(actors))
	for i, p := range patchStates {
		if len(slotStates[i]) != NaturalSlotsPerPatch {
			return NaturalBalance{}, ErrNaturalContract
		}
		var patchSlots [NaturalSlotsPerPatch]NaturalSlotState
		copy(patchSlots[:], slotStates[i])
		if !naturalStock(p, patchSlots) {
			return NaturalBalance{}, ErrNaturalContract
		}
		b.Produced += p.Produced
		b.Granted += p.Granted
		b.Unrealized += p.Unrealized
		patchID := sim.EntityID(NaturalPatchIDBase + i)
		var gathered, consumed, held, invested, tenured, cultivated int64
		for _, slot := range patchSlots {
			gathered += slot.Gathered
			b.Stock += slot.Stock
			if slot.Mode == NaturalTenureCultivated {
				tenured++
				owner, ok := actors[slot.Owner]
				if !ok || owner.Bag.Source != patchID {
					return NaturalBalance{}, ErrNaturalContract
				}
				tenuredByOwner[slot.Owner]++
			}
		}
		b.TenuredSlots += tenured
		if tenured > NaturalSlotsPerPatch {
			return NaturalBalance{}, ErrNaturalContract
		}
		for _, id := range ids {
			a := actors[id]
			if a.Bag.Source != patchID {
				continue
			}
			consumed += a.Body.Consumed
			held += a.Bag.Units
			invested += a.Worksite.InvestedUnits
			cultivated += a.Granary.CultivatedTotal
			b.CultivatedTotal += a.Granary.CultivatedTotal
			b.GranaryStock += a.Granary.Stock
			b.StoredMeals += a.Granary.StoredMeals
			b.Spoiled += a.Granary.Spoiled
			b.Capital += a.Worksite.Capital
			b.Wip += a.Worksite.Wip
			b.PointsCreated += a.Worksite.PointsCreated
			b.PointsDecayed += a.Worksite.PointsDecayed
			b.Energy += a.Body.Energy
			b.BasalSpent += a.Body.BasalSpent
			b.EnergyCapLost += a.Body.CapLost
			b.BirthGranaryPaid += a.Genome.BirthGranaryPaid
		}
		if gathered != consumed+held+invested {
			return NaturalBalance{}, ErrNaturalContract
		}
		// Flat-grant seam: every entitlement the split granted on this patch
		// was received by a homed granary in the same pulse.
		if cultivated != p.Granted {
			return NaturalBalance{}, ErrNaturalContract
		}
		b.Gathered += gathered
		b.Consumed += consumed
		b.Held += held
		b.Invested += invested
	}
	// G2b land identity: every actor's tenured slots equal its standing
	// capital exactly (dead owners included until wear reverts their
	// slots), so per patch tenuredSlots = Σk over that patch's owners ≤ 8
	// follows.
	for id, a := range actors {
		if tenuredByOwner[id] != a.Worksite.Capital {
			return NaturalBalance{}, ErrNaturalContract
		}
	}
	// Pass 4: aggregate identities, G1, G2b, G2a extended, G3 extended.
	if b.Gathered != b.Consumed+b.Held+b.Invested || b.Produced != b.Gathered+b.Stock ||
		b.Invested != NaturalPointCostWip*b.PointsCreated+b.Wip || b.PointsCreated-b.PointsDecayed != b.Capital ||
		b.CultivatedTotal != b.StoredMeals+b.GranaryStock+b.Spoiled+b.BirthGranaryPaid ||
		b.InitialEnergy+NaturalNewbornEnergy*b.Births+b.Consumed+b.StoredMeals != b.Energy+b.BasalSpent+b.EnergyCapLost {
		return NaturalBalance{}, ErrNaturalContract
	}
	return b, nil
}
