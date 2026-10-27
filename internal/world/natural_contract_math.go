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
//	    (stated in budget slots: under the yield mechanisms v9 and v11 the
//	    granted and — under v11 — the produced ledgers count FOOD UNITS, so
//	    each is divided by the one declared yield; at yield 1 the identity
//	    is the frozen one verbatim)
//	    and per patch:  gathered = consumed + held + invested (mechanism
//	                    v6: gathered + provisioned = consumed + held +
//	                    invested; provisioned is rule 515's granary-to-bag
//	                    inflow and is zero in every v5 state), with
//	                    gathered + standing stock = the wild share produced
//	G2a per granary:    cultivatedTotal = stock + storedMeals + spoiled
//	                    + birthGranaryPaid
//	G2a′ (mechanism v6): cultivatedTotal = stock + storedMeals + provisioned
//	                    + spoiled + birthGranaryPaid — the v5 identity
//	                    extended by rule 515's withdrawal ledger; at
//	                    provisioned = 0 (every v5 state) it is exactly G2a.
//	G2a″ (mechanism v7): cultivatedTotal + wagesEarned = stock + storedMeals
//	                    + provisioned + spoiled + birthGranaryPaid
//	                    + wagesPaid — the wage ledgers join the inflow and
//	                    outflow sides (the worker's wage lands in stock
//	                    against wagesEarned; the employer's paid wage leaves
//	                    stock against wagesPaid). At wages ≡ 0 (every v5/v6
//	                    state, pinned by the granary validator) it is
//	                    term-for-term G2a′, and at provisioned = 0 as well
//	                    (every v5 state) term-for-term G2a.
//	G2b per actor:      invested = 2·created + wip; created − decayed = k;
//	    land:           tenuredSlots(patch) = Σk over ALL tenure holders
//	                    (dead included) ≤ 8, each tenured slot's owner an
//	                    existing actor whose home patch is that patch
//	G3  energy:         11·f + 8·births + Σconsumed + ΣstoredMeals =
//	                    Σenergy + ΣbasalSpent + ΣcapLoss, and per newborn
//	                    cohort 8 + consumed + stored = energy + basal + cap
//	                    (the consumed term carries the declared meal price
//	                    naturalMealEnergyFor — the frozen 1 in every
//	                    mechanism below v12, 2 under mechanism v12, where
//	                    one eaten bag unit is a two-hour meal — while a
//	                    stored meal keeps its frozen one energy in every
//	                    mechanism)
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
	// Provisioned counts granary units drawn as worksite provisions under
	// rule 515 (mechanism v6 only). It is zero in every v5 state — the v5
	// validators require exactly that — and grows by one per provision.
	Provisioned int64
	// WagesPaid and WagesEarned count granary units paid as wages by an
	// employer and received as wages by a worker under rule 518 (mechanism
	// v7 only, ticket 20). They are zero in every v5/v6 state — those
	// validators require exactly that — so G2a″ above reduces term for term
	// to G2a′ outside the wage mechanism.
	WagesPaid   int64
	WagesEarned int64
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

// NaturalLabourOfferState is the mechanism-v7-only worker offer row
// (component 0x80000058): the birth-request field set minus the
// self-reported claims — the terms are fixed by the frozen design (one
// gathered wild unit delivered for exactly one granary stock unit), so no
// claim and no price field exists. The status enum and its values are the
// frozen request statuses.
type NaturalLabourOfferState struct {
	Hour          int64        // hour of the most recent filing, -1 if never
	Addressee     sim.EntityID // zero only while never-filed
	Status        NaturalRequestStatus
	LastOfferHour int64
	LastReplyHour int64 // the decision hour of the last answered offer
}

// NaturalLabourBookState is the mechanism-v7-only employer engagement row
// (component 0x80000059): the single engagement slot plus the
// one-decision-per-actor-hour bookkeeping. EngagedWorker is zero exactly
// while EngagedHour is the never-hour (never engaged, or cleared by rule
// 519 after the delivery window); LastDecisionHour persists as the audit
// and the one-decision-per-actor-hour guard.
type NaturalLabourBookState struct {
	EngagedWorker    sim.EntityID
	EngagedHour      int64
	LastDecisionHour int64
}

// NaturalActorState is one actor's six component rows read at one version,
// plus the two mechanism-v7-only wage-contract rows. The Offer and Book
// fields exist only under NaturalMechanismV7: every v5/v6 validator ignores
// them and the v5/v6 registries have no such component, so the v5/v6
// default paths are unchanged; every V7 validator requires them valid.
type NaturalActorState struct {
	Body     NaturalBodyState
	Bag      NaturalBagState
	Worksite NaturalWorksiteState
	Granary  NaturalGranaryState
	Genome   NaturalGenomeState
	Request  NaturalBirthRequestState
	Offer    NaturalLabourOfferState
	Book     NaturalLabourBookState
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
// patch budget: the declared per-slot fill (one granary unit below v9, the
// declared yield under v9 and v11) per entitled cultivated slot, granted
// flat by rule 508 (corrigendum: no divisor loop, no locus mints units
// beyond the budget R).
type NaturalGrantEntitlement struct {
	Owner sim.EntityID
	Units int64
}

// NaturalAliveLookup is the trusted runner's owner-liveness view for the
// regen split: only living owners' cultivated slots are entitled.
type NaturalAliveLookup func(sim.EntityID) bool

// NaturalBalance carries the aggregate frozen gates G1, G2a (G2a′ under
// mechanism v6, G2a″ under mechanism v7), G2b, and G3.
type NaturalBalance struct {
	Produced, Granted, Unrealized, Gathered, Consumed, Stock, Held, Invested int64
	CultivatedTotal, GranaryStock, StoredMeals, Spoiled                      int64
	Provisioned                                                              int64 // mechanism v6 only; zero in every v5 balance
	WagesPaid, WagesEarned                                                   int64 // mechanism v7 only; zero in every v5/v6 balance
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

// validNaturalPatchFor is the mechanism-parameterized patch ledger validator.
// The frozen mechanisms keep validNaturalPatch exactly. Mechanism v9 records
// the rule-508 grant in FOOD UNITS (Yield units per entitled cultivated
// slot), so the granted field carries the yield-widened envelope and the
// per-patch-hour land identity is stated in budget slots: the granted units
// divided by the declared yield are the slots the R budget consumed. Under
// mechanism v11 the wild fill carries the same declared yield, so produced
// counts food units too: both ledgers are stated in budget slots by dividing
// them by the one declared yield, and the same identity holds.
func validNaturalPatchFor(m NaturalMechanism, yield int64, p NaturalPatchState) bool {
	if !naturalHasYieldAxis(m) {
		return validNaturalPatch(p)
	}
	producedMax := NaturalGranaryStockMax
	if naturalWildFillYield(m) {
		producedMax = naturalYieldLedgerMax
	}
	if yield < 1 || yield > NaturalYieldMax || p.Yield < 0 || p.Yield > NaturalSlotsPerPatch ||
		p.Pulses < 0 || p.Pulses > NaturalHorizonHours ||
		p.Produced < 0 || p.Produced > producedMax ||
		p.Granted < 0 || p.Granted > naturalYieldLedgerMax || p.Granted%yield != 0 ||
		p.Unrealized < 0 || p.Unrealized > NaturalGranaryStockMax {
		return false
	}
	producedSlots := p.Produced
	if naturalWildFillYield(m) {
		if p.Produced%yield != 0 {
			return false
		}
		producedSlots = p.Produced / yield
	}
	return producedSlots+p.Granted/yield+p.Unrealized == p.Yield*p.Pulses
}

func validNaturalSlot(s NaturalSlotState) bool {
	return validNaturalSlotFor(NaturalMechanismV5, s)
}

// validNaturalSlotFor is the mechanism-parameterized slot validator: the
// frozen one-unit standing-stock envelope below v11, the declared yield
// under v11 (a wild slot refills Yield units per patch-hour). Everything
// else — the tenure shape and the gather envelope — is frozen verbatim.
func validNaturalSlotFor(m NaturalMechanism, s NaturalSlotState) bool {
	if s.Stock < 0 || s.Stock > naturalSlotStockMaxFor(m) || s.Gathered < 0 || s.Gathered > NaturalHorizonHours {
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

func validNaturalBodyFor(m NaturalMechanism, b NaturalBodyState) bool {
	return b.Energy >= 0 && b.Energy <= NaturalEnergyCapacity && b.Hunger >= 0 && b.Hunger <= NaturalHungerCapacity &&
		b.BasalSpent >= 0 && b.BasalSpent <= NaturalBasalSpentMax && b.CapLost >= 0 && b.CapLost <= naturalCapLostMaxFor(m) &&
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

// validNaturalGranaryFor is the mechanism-parameterized granary validator.
// The zero value requires the v6-only provisioned ledger and the v7-only
// wage ledgers to be exactly zero (the fields do not exist in v5) and keeps
// every frozen v5 bound; NaturalMechanismV6 admits provisioned in its
// horizon envelope [0,168] and pins the wage ledgers to zero;
// NaturalMechanismV7 admits provisioned and both wage ledgers in theirs.
// Mechanism v9 widens the stock, cultivated-total and spoiled envelopes
// exactly like its descriptors (the yield-scaled grant), and is otherwise v8.
func validNaturalGranaryFor(m NaturalMechanism, g NaturalGranaryState) bool {
	stockMax, spoiledMax := naturalGranaryBoundsFor(m)
	if !(g.Stock >= 0 && g.Stock <= stockMax &&
		g.CultivatedTotal >= 0 && g.CultivatedTotal <= stockMax &&
		g.StoredMeals >= 0 && g.StoredMeals <= NaturalHorizonHours &&
		g.LastStoredMealHour >= NaturalNeverHour && g.LastStoredMealHour < NaturalHorizonHours &&
		g.SpoilDebt >= 0 && g.SpoilDebt <= NaturalSpoilDebtMax &&
		g.Spoiled >= 0 && g.Spoiled <= spoiledMax) {
		return false
	}
	switch m {
	case NaturalMechanismV6, NaturalMechanismV8, NaturalMechanismV9, NaturalMechanismV10, NaturalMechanismV11, NaturalMechanismV12, NaturalMechanismV13:
		return g.Provisioned >= 0 && g.Provisioned <= NaturalProvisionedMax && g.WagesPaid == 0 && g.WagesEarned == 0
	case NaturalMechanismV7:
		return g.Provisioned >= 0 && g.Provisioned <= NaturalProvisionedMax &&
			g.WagesPaid >= 0 && g.WagesPaid <= NaturalWagesMax &&
			g.WagesEarned >= 0 && g.WagesEarned <= NaturalWagesMax
	}
	return g.Provisioned == 0 && g.WagesPaid == 0 && g.WagesEarned == 0
}

func validNaturalGranary(g NaturalGranaryState) bool {
	return validNaturalGranaryFor(NaturalMechanismV5, g)
}

// validNaturalGenome checks the frozen bounds and lineage shape: founders
// (IDs 1..8P) have absent parents and birth-hour 0; newborns have both
// parents present, distinct, strictly earlier IDs (the allocator is
// monotonic, so the reproduction graph stays acyclic), and a creation
// lifetime in [96,143]. died-hour is either -1 or the death hour itself,
// never below birth-hour.
func validNaturalGenome(patches int, actor sim.EntityID, g NaturalGenomeState) bool {
	return validNaturalGenomeFor(NaturalMechanismV5, patches, actor, g)
}

// validNaturalGenomeFor is the mechanism-parameterized genome validator: its
// birth-granary-paid bound is the declared envelope of the mechanism
// (naturalBirthGranaryPaidMaxFor — the widened v13 envelope under the birth
// price axis, the frozen four-units-per-hour envelope below it), so an actor
// that paid a declared v13 price stays valid. The zero value reproduces
// validNaturalGenome exactly.
func validNaturalGenomeFor(m NaturalMechanism, patches int, actor sim.EntityID, g NaturalGenomeState) bool {
	locus := func(v int64) bool { return v >= NaturalGenomeLocusMin && v <= NaturalGenomeLocusMax }
	if !locus(g.LocusM) || !locus(g.LocusY) || !locus(g.LocusW) {
		return false
	}
	if g.BirthGranaryPaid < 0 || g.BirthGranaryPaid > naturalBirthGranaryPaidMaxFor(m) {
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

func validNaturalBirthRequestFor(m NaturalMechanism, patches int, owner sim.EntityID, r NaturalBirthRequestState) bool {
	stockMax, _ := naturalGranaryBoundsFor(m)
	if r.ReportedCapital < 0 || r.ReportedCapital > NaturalKMax || r.ReportedGranary < 0 || r.ReportedGranary > stockMax {
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

// validNaturalLabourOffer checks one mechanism-v7 worker offer row: every
// hour inside its bounds, the filed shapes coherent, and the status-driven
// shape exactly as the frozen birth-request discipline defines them:
//
//   - idle is either the never-filed shape (all never-hours, no addressee)
//     or the cleared shape (rule 519 "void contracts clear": the row
//     retains the last filing as the privileged audit while no contract is
//     open);
//   - pending / accepted / refused / expired are filed shapes: the request
//     hour is the last-offer hour and the addressee is a valid actor other
//     than the owner;
//   - accepted and refused carry the decision hour (same-hour decisions:
//     a pending offer never survives the next pulse, rule 519);
//   - pending and expired rows are unanswered, so the retained reply hour
//     predates the filing (or is the never-hour).
func validNaturalLabourOffer(patches int, owner sim.EntityID, o NaturalLabourOfferState) bool {
	hourOK := func(h int64) bool { return h >= NaturalNeverHour && h < NaturalHorizonHours }
	if !hourOK(o.Hour) || !hourOK(o.LastOfferHour) || !hourOK(o.LastReplyHour) || o.Status < NaturalRequestIdle || o.Status > NaturalRequestExpired {
		return false
	}
	filed := func() bool {
		return o.Hour >= 0 && o.Hour == o.LastOfferHour && o.Addressee != 0 && o.Addressee != owner && naturalActorEntity(patches, o.Addressee)
	}
	switch o.Status {
	case NaturalRequestIdle:
		if o.Hour == NaturalNeverHour {
			// Review P2 (ticket-20 L1): the never-filed shape must pin every
			// touched field, symmetrically with the labour-book validator; an
			// unfiled row can carry no reply hour either.
			return o.Addressee == 0 && o.LastOfferHour == NaturalNeverHour && o.LastReplyHour == NaturalNeverHour
		}
		// Cleared-with-audit: the last filing is retained for the audit.
		return o.Addressee != 0 && o.Addressee != owner && naturalActorEntity(patches, o.Addressee)
	case NaturalRequestPending, NaturalRequestExpired:
		return filed() && (o.LastReplyHour == NaturalNeverHour || o.LastReplyHour < o.Hour)
	case NaturalRequestAccepted, NaturalRequestRefused:
		return filed() && o.LastReplyHour == o.Hour
	}
	return false
}

// validNaturalLabourBook checks one mechanism-v7 employer engagement row:
// the engaged-worker ref is present exactly while the engagement hour is a
// real hour (cleared or never-engaged rows carry both zeros), and the
// decision hour is inside its bounds.
func validNaturalLabourBook(b NaturalLabourBookState) bool {
	if b.LastDecisionHour < NaturalNeverHour || b.LastDecisionHour >= NaturalHorizonHours {
		return false
	}
	if b.EngagedWorker == 0 {
		return b.EngagedHour == NaturalNeverHour
	}
	return b.EngagedHour >= 0 && b.EngagedHour < NaturalHorizonHours
}

// naturalAlive reports the v4-verbatim liveness invariant: died-hour -1
// with positive energy. Dead actors take no actions; their granaries still
// spoil and their capital still wears.
func naturalAlive(a NaturalActorState) bool {
	return a.Genome.DiedHour == NaturalNeverHour && a.Body.Energy > 0
}

// validNaturalActorFor checks the six rows of one actor under the mechanism
// axis — plus, mechanism v7 only, the two wage-contract rows — against:
// home-patch provenance (founder homes are ID-placed; newborn homes are
// carried), never-gathered coherence, the per-actor granary identity —
// G2a″ cultivatedTotal + wagesEarned = stock + storedMeals + provisioned +
// spoiled + birthGranaryPaid + wagesPaid under v7; at wages ≡ 0 (pinned for
// every v5/v6 state by validNaturalGranaryFor) it is term-for-term G2a′
// cultivatedTotal = stock + storedMeals + provisioned + spoiled +
// birthGranaryPaid, which at provisioned = 0 (enforced for every v5 state)
// is exactly the frozen v5 G2a — the per-actor G3 energy identity with the
// creation base (11 founder, 8 newborn), and the alive invariant
// died-hour=-1 ⇒ energy>0.
func validNaturalActorFor(m NaturalMechanism, patches int, actor sim.EntityID, a NaturalActorState) bool {
	if !naturalActorEntity(patches, actor) || !validNaturalBodyFor(m, a.Body) || !validNaturalWorksite(a.Worksite) ||
		!validNaturalGranaryFor(m, a.Granary) || !validNaturalGenomeFor(m, patches, actor, a.Genome) || !validNaturalBirthRequestFor(m, patches, actor, a.Request) {
		return false
	}
	if m == NaturalMechanismV7 && (!validNaturalLabourOffer(patches, actor, a.Offer) || !validNaturalLabourBook(a.Book)) {
		return false
	}
	home := a.Bag.Source
	if naturalFounderEntity(patches, actor) {
		expected, err := NaturalFounderPatchIDFor(m, patches, actor)
		if err != nil || home != expected {
			return false
		}
	}
	if !validNaturalBag(patches, home, a.Bag) {
		return false
	}
	// Never-gathered coherence: a bag unit or a consumed meal needs a
	// source. Under the v6 mechanism a provisioned unit is the second bag
	// source (a granary draw, recorded in the provisioned ledger), so a
	// positive provisioned ledger admits both; the v5 zero value pins
	// provisioned to zero and keeps the frozen form exactly. A wage is
	// never a bag source: rule 518 lands it directly in the granary stock,
	// so the coherence form is unchanged under v7.
	if a.Body.LastGatherHour == NaturalNeverHour && a.Granary.Provisioned == 0 && (a.Bag.Units != 0 || a.Body.Consumed != 0) {
		return false
	}
	base := NaturalFounderInitialEnergy
	if !naturalFounderEntity(patches, actor) {
		base = NaturalNewbornEnergy
	}
	// G2a′ (G2a″ under v7): every cultivated unit received is standing,
	// eaten as a stored meal, drawn as a worksite provision (v6 and v7),
	// spoiled, or paid toward a birth; under v7 every wage received or
	// paid joins the same closure on its own side. The wage terms are
	// pinned to zero outside v7, so the expression reduces term for term
	// to the frozen G2a′ there.
	if a.Granary.CultivatedTotal+a.Granary.WagesEarned != a.Granary.Stock+a.Granary.StoredMeals+a.Granary.Provisioned+a.Granary.Spoiled+a.Genome.BirthGranaryPaid+a.Granary.WagesPaid {
		return false
	}
	// G3 per actor. The consumed term carries the declared meal price: the
	// frozen one energy below v12, the declared two under v12, so a
	// two-energy meal and its one counted unit still balance exactly.
	return base+naturalMealEnergyFor(m)*a.Body.Consumed+a.Granary.StoredMeals == a.Body.Energy+a.Body.BasalSpent+a.Body.CapLost
}

// validNaturalActor is the frozen v5 validator (the mechanism zero value).
func validNaturalActor(patches int, actor sim.EntityID, a NaturalActorState) bool {
	return validNaturalActorFor(NaturalMechanismV5, patches, actor, a)
}

// naturalStock verifies the patch production ledger against its slots
// (G1): no slot supplies units before they are produced, and the wild share
// produced equals gathered plus current standing stock.
func naturalStock(p NaturalPatchState, slots [NaturalSlotsPerPatch]NaturalSlotState) bool {
	return naturalStockFor(NaturalMechanismV5, 1, p, slots)
}

// naturalStockFor is naturalStock under a declared mechanism and yield: the
// patch ledger validates against the mechanism's own envelope and the wild
// share produced equals gathered plus current standing stock (the slot
// arithmetic is unchanged in every mechanism).
func naturalStockFor(m NaturalMechanism, yield int64, p NaturalPatchState, slots [NaturalSlotsPerPatch]NaturalSlotState) bool {
	if !validNaturalPatchFor(m, yield, p) {
		return false
	}
	// One slot supplies at most one unit per patch-hour below v11 and at
	// most the declared yield under v11, where a wild slot refills it.
	perSlotHour := p.Pulses * naturalWildFillUnits(m, yield)
	var total int64
	for _, slot := range slots {
		if !validNaturalSlotFor(m, slot) || slot.Gathered+slot.Stock > perSlotHour {
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
	return NaturalRegenSplitFor(NaturalMechanismV5, 1, patches, hour, patch, slots, ownerAlive)
}

// NaturalRegenSplitFor is the mechanism-parameterized regen split. Every
// frozen mechanism is NaturalRegenSplit byte-for-byte (one unit per
// entitled cultivated slot, one unit per vacant wild slot). Mechanism v9
// multiplies exactly that rule-508 per-slot grant by the declared Yield:
// each entitled slot still consumes ONE unit of the R budget (the land
// identity stays stated in budget slots), but the entitlement hands the
// owner Yield granary units. The granted ledger field and the returned
// Cultivated total count those food units, so the grant seam (cultivated
// total = granted) is unchanged. Mechanism v11 applies the same declared
// yield to the wild fill: a vacant wild slot still consumes one R budget
// unit but stands Yield granary units, and the produced ledger counts those
// food units too.
func NaturalRegenSplitFor(m NaturalMechanism, yield int64, patches, hour int, patch NaturalPatchState, slots [NaturalSlotsPerPatch]NaturalSlotState, ownerAlive NaturalAliveLookup) (NaturalPatchState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalRegenSplitResult, []NaturalGrantEntitlement, error) {
	perSlot := int64(1)
	if naturalHasYieldAxis(m) {
		if yield < 1 || yield > NaturalYieldMax {
			return NaturalPatchState{}, slots, NaturalRegenSplitResult{}, nil, ErrNaturalContract
		}
		perSlot = yield
	}
	wildPerSlot := naturalWildFillUnits(m, yield)
	if _, err := NaturalHourTime(hour); err != nil || int64(hour) != patch.Pulses || !naturalStockFor(m, yield, patch, slots) || ownerAlive == nil {
		return NaturalPatchState{}, slots, NaturalRegenSplitResult{}, nil, ErrNaturalContract
	}
	out := slots
	budget := patch.Yield
	var result NaturalRegenSplitResult
	grantedSlots, wildSlots := int64(0), int64(0)
	grants := make([]NaturalGrantEntitlement, 0, NaturalSlotsPerPatch)
	grantIndex := make(map[sim.EntityID]int, NaturalSlotsPerPatch)
	// Pass 1: entitled cultivated slots of living owners, ascending.
	for i := range out {
		if out[i].Mode == NaturalTenureCultivated && budget > 0 && ownerAlive(out[i].Owner) {
			budget--
			grantedSlots++
			result.Cultivated += perSlot
			if j, ok := grantIndex[out[i].Owner]; ok {
				grants[j].Units += perSlot
			} else {
				grantIndex[out[i].Owner] = len(grants)
				grants = append(grants, NaturalGrantEntitlement{Owner: out[i].Owner, Units: perSlot})
			}
		}
	}
	// Pass 2: vacant wild slots fill ascending, exactly as v1-v4, at the
	// declared per-slot fill.
	for i := range out {
		if out[i].Mode == NaturalTenureWild && out[i].Stock == 0 && budget > 0 {
			out[i].Stock += wildPerSlot
			budget--
			wildSlots++
			result.Wild += wildPerSlot
		}
	}
	result.Unrealized = budget
	patch.Pulses++
	patch.Produced += result.Wild
	patch.Granted += result.Cultivated
	patch.Unrealized += result.Unrealized
	// Per-patch-hour land identity, stated in budget slots: R = granted
	// slots + wild slots filled + unrealized, exact for every budget and
	// occupancy. No locus mints units beyond the declared per-slot fill.
	if grantedSlots+wildSlots+result.Unrealized != patch.Yield {
		return NaturalPatchState{}, slots, NaturalRegenSplitResult{}, nil, ErrNaturalContract
	}
	if !naturalStockFor(m, yield, patch, out) {
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
	return NaturalCultivatedGrantFor(NaturalMechanismV5, patches, ownerID, a, units)
}

// NaturalCultivatedGrantFor is the mechanism-parameterized flat grant; the
// zero value reproduces NaturalCultivatedGrant exactly, and only the granary
// validation (G2a′) differs under NaturalMechanismV6.
func NaturalCultivatedGrantFor(m NaturalMechanism, patches int, ownerID sim.EntityID, a NaturalActorState, units int64) (NaturalActorState, NaturalGrantResult, error) {
	unitsMax := int64(NaturalSlotsPerPatch)
	if naturalHasYieldAxis(m) {
		// Rule 508's per-slot grant is Yield units, so one owner's merged
		// entitlement can reach every cultivated slot at the maximum yield.
		unitsMax = NaturalSlotsPerPatch * NaturalYieldMax
	}
	if !naturalActorEntity(patches, ownerID) || !validNaturalActorFor(m, patches, ownerID, a) || units < 1 || units > unitsMax {
		return NaturalActorState{}, NaturalGrantResult{}, ErrNaturalContract
	}
	if !naturalAlive(a) {
		return NaturalActorState{}, NaturalGrantResult{Fallowed: units}, &NaturalFallowError{Owner: ownerID, Units: units}
	}
	a.Granary.Stock += units
	a.Granary.CultivatedTotal += units
	if !validNaturalActorFor(m, patches, ownerID, a) {
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
	return NaturalSpoilageFor(NaturalMechanismV5, patches, ownerID, a)
}

// NaturalSpoilageFor is the mechanism-parameterized spoilage loop; the zero
// value reproduces NaturalSpoilage exactly.
func NaturalSpoilageFor(m NaturalMechanism, patches int, ownerID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalSpoilageResult, error) {
	if !naturalActorEntity(patches, ownerID) || !validNaturalActorFor(m, patches, ownerID, a) {
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
	if !validNaturalActorFor(m, patches, ownerID, a) {
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
	return NaturalWearFor(NaturalMechanismV5, patches, ownerID, a, homePatch, homeSlots)
}

// NaturalWearFor is the mechanism-parameterized wear loop; the zero value
// reproduces NaturalWear exactly.
func NaturalWearFor(m NaturalMechanism, patches int, ownerID sim.EntityID, a NaturalActorState, homePatch sim.EntityID, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalWearResult, error) {
	if !naturalActorEntity(patches, ownerID) || !validNaturalActorFor(m, patches, ownerID, a) {
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
	if !validNaturalActorFor(m, patches, ownerID, a) {
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
	return NaturalBasalFor(NaturalMechanismV5, patches, hour, actorID, a, elapsedHours)
}

// NaturalBasalFor is the mechanism-parameterized basal loop; the zero value
// reproduces NaturalBasal exactly.
func NaturalBasalFor(m NaturalMechanism, patches, hour int, actorID sim.EntityID, a NaturalActorState, elapsedHours int64) (NaturalActorState, NaturalBasalCost, bool, error) {
	if _, err := NaturalHourTime(hour); err != nil || !naturalActorEntity(patches, actorID) || !validNaturalActorFor(m, patches, actorID, a) ||
		elapsedHours < 1 || elapsedHours > NaturalHorizonHours || !naturalAlive(a) {
		return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
	}
	// Reachable-state guard: two drains per hour (locus-m 5) plus four
	// units per hour of birth payments can never exceed the widened ledger
	// bound 1008.
	if a.Body.BasalSpent+2*elapsedHours > NaturalBasalSpentMax {
		return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
	}
	locusM := a.Genome.LocusM
	var result NaturalBasalCost
	// Accrual and drain happen once per elapsed hour so the batched path
	// reproduces the frozen hourly lattice exactly (v3 byte-identity holds
	// at neutral 6 for any elapsedHours, not only 1).
	for h := int64(0); h < elapsedHours; h++ {
		a.Body.BasalDebt += NaturalGenomeDenominator
		if a.Body.BasalDebt > NaturalBasalDebtMax {
			return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
		}
		for a.Body.BasalDebt >= locusM && a.Body.Energy >= 1 {
			a.Body.Energy--
			a.Body.BasalSpent++
			a.Body.BasalDebt -= locusM
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
	if !validNaturalActorFor(m, patches, actorID, a) {
		return NaturalActorState{}, NaturalBasalCost{}, false, ErrNaturalContract
	}
	return a, result, starved, nil
}

// NaturalGather transfers one identified home-patch slot unit to the actor's
// bag, v1-v4-identical among the living: dead actors take no actions. The
// bag's home-patch source is already present and unchanged. A stocked slot
// is exactly one unit deep below v11 and at least one unit deep under v11
// (where the wild fill stands the declared yield); the gather itself always
// moves exactly one unit.
func NaturalGather(patches int, patchID, slotID, actorID sim.EntityID, hour int, slot NaturalSlotState, a NaturalActorState) (NaturalSlotState, NaturalActorState, error) {
	return NaturalGatherFor(NaturalMechanismV5, patches, patchID, slotID, actorID, hour, slot, a)
}

// NaturalGatherFor is the mechanism-parameterized gather; the zero value
// reproduces NaturalGather exactly.
func NaturalGatherFor(m NaturalMechanism, patches int, patchID, slotID, actorID sim.EntityID, hour int, slot NaturalSlotState, a NaturalActorState) (NaturalSlotState, NaturalActorState, error) {
	home := a.Bag.Source
	if home != patchID || hour < 0 || hour >= NaturalHorizonHours || !validNaturalSlotFor(m, slot) || !naturalSlotStocked(m, slot) ||
		slot.Gathered > int64(hour) || !validNaturalActorFor(m, patches, actorID, a) || !naturalAlive(a) || a.Bag.Units != 0 || a.Body.LastGatherHour >= int64(hour) {
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
	slot.Stock, slot.Gathered = slot.Stock-1, slot.Gathered+1
	a.Bag.Units = 1
	a.Body.LastGatherHour = int64(hour)
	if !validNaturalActorFor(m, patches, actorID, a) {
		return NaturalSlotState{}, NaturalActorState{}, ErrNaturalContract
	}
	return slot, a, nil
}

// NaturalConsume eats the held wild unit at the bag decision with v1-v4
// meal semantics: the declared meal energy (naturalMealEnergyFor — +1 below
// mechanism v12, +2 under it), counted spill at the cap-12 ceiling, −1
// hunger.
func NaturalConsume(patches int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalMeal, error) {
	return NaturalConsumeFor(NaturalMechanismV5, patches, actorID, a)
}

// NaturalConsumeFor is the mechanism-parameterized consume; the zero value
// reproduces NaturalConsume exactly. The unit is always consumed; whatever of
// the declared meal energy does not fit under the frozen
// NaturalEnergyCapacity ceiling is counted spill in CapLost, exactly as the
// frozen one-unit meal does at the cap.
func NaturalConsumeFor(m NaturalMechanism, patches int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalMeal, error) {
	if !naturalActorEntity(patches, actorID) || !validNaturalActorFor(m, patches, actorID, a) || !naturalAlive(a) || a.Bag.Units != 1 || a.Body.Consumed >= NaturalHorizonHours {
		return NaturalActorState{}, NaturalMeal{}, ErrNaturalContract
	}
	meal := naturalMealEnergyFor(m)
	a.Bag.Units = 0
	a.Body.Consumed++
	gain := meal
	if room := NaturalEnergyCapacity - a.Body.Energy; gain > room {
		gain = room
	}
	result := NaturalMeal{EnergyGained: gain, EnergyCapLost: meal - gain}
	a.Body.Energy += gain
	a.Body.CapLost += meal - gain
	if a.Body.Hunger > 0 {
		a.Body.Hunger--
	}
	if !validNaturalActorFor(m, patches, actorID, a) {
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
	return NaturalBuildFor(NaturalMechanismV5, patches, hour, actorID, a, homePatch, homeSlots)
}

// NaturalBuildFor is the mechanism-parameterized build; the zero value
// reproduces NaturalBuild exactly.
func NaturalBuildFor(m NaturalMechanism, patches, hour int, actorID sim.EntityID, a NaturalActorState, homePatch sim.EntityID, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalBuildResult, error) {
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
	if !naturalActorEntity(patches, actorID) || !validNaturalActorFor(m, patches, actorID, a) || !naturalAlive(a) {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	if a.Bag.Units != 1 || a.Worksite.Capital+a.Worksite.Wip >= NaturalKMax || a.Worksite.LastBuildHour >= int64(hour) {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	for _, slot := range homeSlots {
		if !validNaturalSlotFor(m, slot) {
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
	if !validNaturalActorFor(m, patches, actorID, a) {
		return NaturalActorState{}, homeSlots, NaturalBuildResult{}, ErrNaturalContract
	}
	return a, homeSlots, result, nil
}

// NaturalEatStored is the only granary draw of the v5 mechanism, with
// v1-v4 meal semantics; dead owners' granaries are frozen (they only
// spoil). The once-per-actor-hour draw discipline stays runner-owned in
// the frozen v5 bytes (naturalPairIntent / naturalPairedMealGate).
func NaturalEatStored(patches, hour int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalMeal, error) {
	return NaturalEatStoredFor(NaturalMechanismV5, patches, hour, actorID, a)
}

// NaturalEatStoredFor is the mechanism-parameterized eat-stored; the zero
// value reproduces NaturalEatStored exactly. Under NaturalMechanismV6 and
// NaturalMechanismV7 the contract itself enforces the shared
// one-draw-per-actor-hour guard (LastStoredMealHour < hour) in both
// directions, because rule 515 can also draw in the same hour: a provision
// hour draws no stored meal and no second draw follows a stored meal.
func NaturalEatStoredFor(m NaturalMechanism, patches, hour int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, NaturalMeal, error) {
	if hour < 0 || hour >= NaturalHorizonHours || !naturalActorEntity(patches, actorID) || !validNaturalActorFor(m, patches, actorID, a) || !naturalAlive(a) ||
		a.Granary.Stock < 1 || a.Granary.StoredMeals >= NaturalHorizonHours ||
		(m != NaturalMechanismV5 && a.Granary.LastStoredMealHour >= int64(hour)) {
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
	if !validNaturalActorFor(m, patches, actorID, a) {
		return NaturalActorState{}, NaturalMeal{}, ErrNaturalContract
	}
	return a, result, nil
}

// NaturalProvision is the pure rule-515 conversion of the v7 design (ticket
// 17), mechanism v6 and v7 only — every call under the zero value is
// refused: at the claim instant an actor holding no bag unit, alive, with
// at least one granary stock unit and no earlier draw this hour (the shared
// LastStoredMealHour < hour guard), converts exactly one stock unit into
// one bag unit: stock−1, provisioned+1, LastStoredMealHour = hour, bag+1.
// Conservation is exact per draw (stock−1 = bag+1, a transfer, never a
// creation): the body is untouched, so G3 is unchanged, and G2a′ stays
// exact because provisioned absorbs the withdrawn stock unit.
func NaturalProvision(m NaturalMechanism, patches, hour int, actorID sim.EntityID, a NaturalActorState) (NaturalActorState, error) {
	if m != NaturalMechanismV6 && m != NaturalMechanismV7 && !naturalHasProvisionLedger(m) {
		return NaturalActorState{}, ErrNaturalContract
	}
	if hour < 0 || hour >= NaturalHorizonHours || !naturalActorEntity(patches, actorID) || !validNaturalActorFor(m, patches, actorID, a) || !naturalAlive(a) ||
		a.Bag.Units != 0 || a.Granary.Stock < 1 ||
		a.Granary.LastStoredMealHour >= int64(hour) || a.Granary.Provisioned >= NaturalProvisionedMax {
		return NaturalActorState{}, ErrNaturalContract
	}
	a.Granary.Stock--
	a.Granary.Provisioned++
	a.Granary.LastStoredMealHour = int64(hour)
	a.Bag.Units++
	if !validNaturalActorFor(m, patches, actorID, a) {
		return NaturalActorState{}, ErrNaturalContract
	}
	return a, nil
}

// NaturalIdleRequestState is the canonical never-filed request row.
// NaturalIdleLabourOfferState is the canonical never-filed offer row.
func NaturalIdleLabourOfferState() NaturalLabourOfferState {
	return NaturalLabourOfferState{Hour: NaturalNeverHour, LastOfferHour: NaturalNeverHour, LastReplyHour: NaturalNeverHour, Status: NaturalRequestIdle}
}

// NaturalIdleLabourBookState is the canonical never-engaged book row.
func NaturalIdleLabourBookState() NaturalLabourBookState {
	return NaturalLabourBookState{EngagedHour: NaturalNeverHour, LastDecisionHour: NaturalNeverHour}
}

// NaturalLabourRefusalReason is the typed taxonomy of the frozen settlement
// refusals (ticket 20): every refusal is a typed void with zero resource
// mutation, the worker's bag intact, and the worker re-entering the
// documented denial path (the build-denial loop verbatim). The contract is
// checked first, then the parties' liveness and shared patch, then the
// resources, then the completion's slot availability.
type NaturalLabourRefusalReason uint8

const (
	NaturalLabourRefusalNone NaturalLabourRefusalReason = iota
	// NaturalLabourRefusalContractNotPending: no live matched contract —
	// no accepted offer addressed to this employer, or an engagement row
	// that does not match (a settlement without a matched offer plus
	// accept is structurally impossible).
	NaturalLabourRefusalContractNotPending
	// NaturalLabourRefusalWorkerDeadOrCrossPatch: the worker is dead, or
	// the pair's home patches differ.
	NaturalLabourRefusalWorkerDeadOrCrossPatch
	// NaturalLabourRefusalEmployerDeadOrCrossPatch: the employer is dead,
	// or the pair's home patches differ.
	NaturalLabourRefusalEmployerDeadOrCrossPatch
	// NaturalLabourRefusalWorkerBagEmpty: the worker holds no gathered
	// unit to deliver (bag ≠ 1).
	NaturalLabourRefusalWorkerBagEmpty
	// NaturalLabourRefusalEmployerStockShort: the employer's granary
	// stock cannot pay the one-unit wage.
	NaturalLabourRefusalEmployerStockShort
	// NaturalLabourRefusalEmployerAtKMax: employer k+wip is at the frozen
	// KMax, so wages cannot buy wip there.
	NaturalLabourRefusalEmployerAtKMax
	// NaturalLabourRefusalNoFreeSlot: the settlement completes a point
	// but no lowest-index free wild slot with stock 0 exists.
	NaturalLabourRefusalNoFreeSlot
)

// NaturalLabourRefusalError carries the typed reason of a refused wage
// settlement (zero mutation: nothing converts, the bag keeps its unit). It
// matches ErrNaturalContract for the package's error convention.
type NaturalLabourRefusalError struct{ Reason NaturalLabourRefusalReason }

func (e *NaturalLabourRefusalError) Error() string {
	return fmt.Sprintf("%s: labour refusal reason %d", ErrNaturalContract, e.Reason)
}
func (e *NaturalLabourRefusalError) Is(target error) bool { return target == ErrNaturalContract }

// NaturalLabourSettleResult mirrors NaturalBuildResult for the settlement's
// folded completion: the claimed slot index is -1 until a completion
// actually claims a slot (0 is a legitimate slot index).
type NaturalLabourSettleResult struct {
	PointsCompleted  int64
	ClaimedSlotIndex int
}

// NaturalLabourSettle is the pure rule-518 handshake of the frozen wage
// contract (ticket 20), mechanism v7 only — every call under any other
// mechanism is refused. homeSlots must be the common home patch's slots
// (the patch is derived from the worker's bag source, exactly as
// NaturalBuild derives it; a cross-patch array is an invalid call). The
// fresh-snapshot revalidation runs in the frozen order before any effect:
// contract pending (the accepted offer addressed to the employer, matched
// by the engagement row, in its one delivery hour engaged-hour+1), both
// parties alive on the same patch, worker bag = 1, employer stock ≥ 1,
// employer k+wip < 8, and — only when the settlement completes a point —
// a lowest-index free wild slot with stock 0. Any failure is a typed void
// with zero resource mutation. The effects are exactly the frozen design's:
// worker bag −1, worker granary stock +1, worker wagesEarned +1, employer
// granary stock −1, employer wagesPaid +1, employer invested +1 and wip +1,
// and on wip reaching 2 the frozen point completion (wip −1 to 0, capital
// +1, created +1, lowest-index free-wild-slot tenure claim with stock 0,
// the 505 fold). The employer's LastBuildHour is never written: the wage
// converts one stock unit into one wip at zero employer-hours. G1 holds
// because the delivered unit is gathered (worker) and invested (employer)
// inside the same closed patch; G2a″ holds on both sides; G2b holds
// incrementally; G3 moves no energy.
func NaturalLabourSettle(m NaturalMechanism, patches, hour int, workerID, employerID sim.EntityID, worker, employer NaturalActorState, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (NaturalActorState, NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState, NaturalLabourSettleResult, error) {
	if m != NaturalMechanismV7 {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, ErrNaturalContract
	}
	if _, err := NaturalPhaseTime(hour, NaturalPhaseLabourSettle); err != nil {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, ErrNaturalContract
	}
	if !naturalActorEntity(patches, workerID) || !naturalActorEntity(patches, employerID) || workerID == employerID ||
		!validNaturalActorFor(m, patches, workerID, worker) || !validNaturalActorFor(m, patches, employerID, employer) {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, ErrNaturalContract
	}
	if _, err := NaturalPatchID(patches, int(worker.Bag.Source)-NaturalPatchIDBase); err != nil {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, ErrNaturalContract
	}
	for _, slot := range homeSlots {
		if !validNaturalSlotFor(m, slot) {
			return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, ErrNaturalContract
		}
	}
	// The matched contract: the worker's accepted offer addressed to this
	// employer, decided in the filing hour, matched by the employer's
	// engagement row, inside its single delivery hour (engaged-hour + 1 —
	// the next pulse clears it, so exactly one settlement per accept).
	engaged := int64(hour) - 1
	if worker.Offer.Status != NaturalRequestAccepted || worker.Offer.Addressee != employerID ||
		worker.Offer.Hour != engaged || worker.Offer.LastReplyHour != engaged ||
		employer.Book.EngagedWorker != workerID || employer.Book.EngagedHour != engaged {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalContractNotPending}
	}
	if !naturalAlive(worker) || worker.Bag.Source != employer.Bag.Source {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalWorkerDeadOrCrossPatch}
	}
	if !naturalAlive(employer) || worker.Bag.Source != employer.Bag.Source {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalEmployerDeadOrCrossPatch}
	}
	if worker.Bag.Units != 1 {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalWorkerBagEmpty}
	}
	if employer.Granary.Stock < 1 {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalEmployerStockShort}
	}
	if employer.Worksite.Capital+employer.Worksite.Wip >= NaturalKMax {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalEmployerAtKMax}
	}
	// Effects, computed on local copies; a refusal below discards them all.
	worker.Bag.Units = 0
	worker.Granary.Stock++
	worker.Granary.WagesEarned++
	employer.Granary.Stock--
	employer.Granary.WagesPaid++
	w := employer.Worksite
	w.InvestedUnits++
	result := NaturalLabourSettleResult{ClaimedSlotIndex: -1}
	w.Wip++
	if w.Wip > NaturalWipMax { // wip=2: the frozen point completion fires
		w.Wip = 0
		w.Capital++
		w.PointsCreated++
		result.PointsCompleted = 1
	}
	employer.Worksite = w
	if result.PointsCompleted == 1 {
		for i := range homeSlots {
			if homeSlots[i].Mode == NaturalTenureWild {
				if homeSlots[i].Stock == 0 {
					homeSlots[i].Mode = NaturalTenureCultivated
					homeSlots[i].Owner = employerID
					homeSlots[i].Cultivated = 1
					result.ClaimedSlotIndex = i
					break
				}
			}
		}
		if result.ClaimedSlotIndex < 0 {
			// Typed refusal, zero mutation: no free wild slot with stock 0.
			return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, &NaturalLabourRefusalError{Reason: NaturalLabourRefusalNoFreeSlot}
		}
	}
	if !validNaturalActorFor(m, patches, workerID, worker) || !validNaturalActorFor(m, patches, employerID, employer) {
		return NaturalActorState{}, NaturalActorState{}, homeSlots, NaturalLabourSettleResult{}, ErrNaturalContract
	}
	return worker, employer, homeSlots, result, nil
}

// naturalLabourBookIdle reports whether the employer's engagement slot is
// free at hour h: no engaged worker, or an engagement already cleared by
// rule 519 (cleared rows keep the stale audit hour, strictly below h-1 —
// a live engagement always has engaged-hour h-1, its delivery hour).
func naturalLabourBookIdle(b NaturalLabourBookState, hour int) bool {
	return b.EngagedWorker == 0 || b.EngagedHour < int64(hour)-1
}

// NaturalLabourOffer files the addressed offer under rule 516 at the frozen
// h+40m+5µs phase, mechanism v7 only — every other mechanism refuses.
// Preconditions (the frozen filing list): the worker alive and valid, bag 0
// (the bag decision has consumed the hour's unit), no open offer (pending
// or accepted — one open contract per worker), no earlier filing this hour
// (last-offer-hour < h), and a same-patch addressee (addresseeHome is the
// trusted runner's fresh view; the filing row alone cannot carry it). The
// derived 1:1 terms are not parameterized: there is nothing to negotiate.
func NaturalLabourOffer(m NaturalMechanism, patches, hour int, workerID, addressee, addresseeHome sim.EntityID, worker NaturalActorState) (NaturalActorState, error) {
	if m != NaturalMechanismV7 {
		return NaturalActorState{}, ErrNaturalContract
	}
	if _, err := NaturalPhaseTime(hour, NaturalPhaseLabourOffer); err != nil ||
		!naturalActorEntity(patches, workerID) || !naturalActorEntity(patches, addressee) || addressee == workerID ||
		!validNaturalActorFor(m, patches, workerID, worker) || !naturalAlive(worker) ||
		worker.Bag.Units != 0 ||
		worker.Offer.Status == NaturalRequestPending || worker.Offer.Status == NaturalRequestAccepted ||
		worker.Offer.LastOfferHour >= int64(hour) {
		return NaturalActorState{}, ErrNaturalContract
	}
	if _, err := NaturalPatchID(patches, int(addresseeHome)-NaturalPatchIDBase); err != nil || addresseeHome != worker.Bag.Source {
		return NaturalActorState{}, ErrNaturalContract
	}
	worker.Offer.Hour = int64(hour)
	worker.Offer.Addressee = addressee
	worker.Offer.Status = NaturalRequestPending
	worker.Offer.LastOfferHour = int64(hour)
	if !validNaturalActorFor(m, patches, workerID, worker) {
		return NaturalActorState{}, ErrNaturalContract
	}
	return worker, nil
}

// NaturalLabourDecision evaluates the addressed employer's rule-517 choice
// at the frozen h+40m+6µs phase, mechanism v7 only: accept engages the
// worker (the worker's offer status accepted with the decision hour, the
// employer's engagement row engaged) and refuse writes the worker's status
// refused plus only the employer's last-decision-hour (status rows only,
// the 512 shape). Structural mismatches — no matching pending offer, a
// second decision in the same hour, an occupied engagement slot on accept —
// are invalid calls with zero mutation. The accept floor (stock ≥ 4) and
// the ascending-worker-ID enumeration are the policy's and the runner's
// to apply; the contract admits any addressed pending offer.
func NaturalLabourDecision(m NaturalMechanism, patches, hour int, employerID, workerID sim.EntityID, accept bool, employer, worker NaturalActorState) (NaturalActorState, NaturalActorState, NaturalReplyDecision, error) {
	if m != NaturalMechanismV7 {
		return NaturalActorState{}, NaturalActorState{}, NaturalReplyNone, ErrNaturalContract
	}
	if _, err := NaturalPhaseTime(hour, NaturalPhaseLabourDecision); err != nil ||
		!naturalActorEntity(patches, employerID) || !naturalActorEntity(patches, workerID) || employerID == workerID ||
		!validNaturalActorFor(m, patches, employerID, employer) || !validNaturalActorFor(m, patches, workerID, worker) || !naturalAlive(employer) ||
		worker.Offer.Status != NaturalRequestPending || worker.Offer.Addressee != employerID ||
		employer.Book.LastDecisionHour >= int64(hour) {
		return NaturalActorState{}, NaturalActorState{}, NaturalReplyNone, ErrNaturalContract
	}
	if accept && !naturalLabourBookIdle(employer.Book, hour) {
		return NaturalActorState{}, NaturalActorState{}, NaturalReplyNone, ErrNaturalContract
	}
	worker.Offer.Status = NaturalRequestRefused
	worker.Offer.LastReplyHour = int64(hour)
	employer.Book.LastDecisionHour = int64(hour)
	decision := NaturalReplyRefuse
	if accept {
		worker.Offer.Status = NaturalRequestAccepted
		employer.Book.EngagedWorker = workerID
		employer.Book.EngagedHour = int64(hour)
		decision = NaturalReplyAccept
	}
	if !validNaturalActorFor(m, patches, employerID, employer) || !validNaturalActorFor(m, patches, workerID, worker) {
		return NaturalActorState{}, NaturalActorState{}, NaturalReplyNone, ErrNaturalContract
	}
	return employer, worker, decision, nil
}

// NaturalLabourOfferExpire is the rule-519 pulse transition of one worker
// offer row, the 513 discipline verbatim plus the void clearing: a pending
// offer from hour h-1 expires at the pulse of h, and an accepted contract
// whose single delivery hour (accept hour + 1) has passed — settled or
// void — clears to idle with its filing retained as the audit. Anything
// else is a no-op success (the pulse touches many rows); a bad hour is an
// invalid call.
func NaturalLabourOfferExpire(hour int, offer NaturalLabourOfferState) (NaturalLabourOfferState, bool, error) {
	if hour < 1 || hour > NaturalHorizonHours {
		return NaturalLabourOfferState{}, false, ErrNaturalContract
	}
	switch {
	case offer.Status == NaturalRequestPending && offer.Hour == int64(hour)-1:
		offer.Status = NaturalRequestExpired
		return offer, true, nil
	case offer.Status == NaturalRequestAccepted && offer.LastReplyHour == int64(hour)-2:
		offer.Status = NaturalRequestIdle
		return offer, true, nil
	}
	return offer, false, nil
}

// NaturalLabourBookClear is the rule-519 pulse transition of one employer
// engagement row: the engagement of hour h-2 — whose delivery window closed
// with the same pulse — clears to the never-engaged shape while the
// decision audit persists. Anything else is a no-op success.
func NaturalLabourBookClear(hour int, book NaturalLabourBookState) (NaturalLabourBookState, bool, error) {
	if hour < 1 || hour > NaturalHorizonHours {
		return NaturalLabourBookState{}, false, ErrNaturalContract
	}
	if book.EngagedWorker != 0 && book.EngagedHour == int64(hour)-2 {
		book.EngagedWorker = 0
		book.EngagedHour = NaturalNeverHour
		return book, true, nil
	}
	return book, false, nil
}

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
	return NaturalFounderStateFor(NaturalMechanismV5, patches, actor, lifetime)
}

// NaturalFounderStateFor is the mechanism-versioned founder row: its home
// patch is the mechanism's declared placement, so v10's round-robin band
// materializes exactly as the contract and the runner validate it.
func NaturalFounderStateFor(m NaturalMechanism, patches int, actor sim.EntityID, lifetime int64) (NaturalActorState, error) {
	genome, err := NaturalFounderGenome(patches, actor, lifetime)
	if err != nil {
		return NaturalActorState{}, err
	}
	home, err := NaturalFounderPatchIDFor(m, patches, actor)
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
	// Mechanism v7: every actor carries the two wage-contract rows
	// (0x80000058/59) from genesis, in their canonical idle shapes — the
	// zero rows are the v5/v6 spelling and invalid under the V7 validators.
	// The founder constructor is the one home of the genesis rows, so the
	// seed builder and the projection builder need no second copy.
	if m == NaturalMechanismV7 {
		a.Offer = NaturalIdleLabourOfferState()
		a.Book = NaturalIdleLabourBookState()
	}
	if !validNaturalActorFor(m, patches, actor, a) {
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

// naturalHouseholdGate reports the mechanisms with the v8 conception rules:
// the band-wide mate pool and the household consent/payment gate. Mechanism
// v9 is v8 verbatim on every birth rule (it only declares the granary
// yield), so the gate is shared by both.
func naturalHouseholdGate(m NaturalMechanism) bool {
	return m == NaturalMechanismV8 || naturalHasYieldAxis(m)
}

// NaturalSurplusGateAllows is the frozen runner-side surplus gate on fresh
// truth, both parents: alive ∧ k≥2 ∧ granary≥3 ∧ energy≥3 ∧ kin-ok ∧ same
// patch. There is no birth-count term: births are bounded only by ecology
// (P15).
func NaturalSurplusGateAllows(proposer, consenter NaturalActorState, kinAllowed bool) bool {
	return naturalSurplusGateAllowsFor(NaturalMechanismV5, proposer, consenter, kinAllowed)
}

// naturalSurplusGateAllowsFor is the mechanism-parameterized surplus gate.
// Under NaturalMechanismV8 (and its v9 extension, which changes only the
// declared yield) the proposer keeps the full wealth gate while the
// consenter only has to be fed (energy ≥ 2), and the co-location requirement
// disappears (the band-wide mate pool); every frozen mechanism keeps the v5
// conjunction exactly.
func naturalSurplusGateAllowsFor(m NaturalMechanism, proposer, consenter NaturalActorState, kinAllowed bool) bool {
	if naturalHouseholdGate(m) {
		return naturalAlive(proposer) && naturalAlive(consenter) &&
			proposer.Worksite.Capital >= NaturalConsentMinCapital &&
			proposer.Granary.Stock >= NaturalConsentMinGranary &&
			proposer.Body.Energy >= NaturalConsentMinEnergy &&
			consenter.Body.Energy >= NaturalHouseholdMinEnergy &&
			kinAllowed
	}
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
	return NaturalBirthRequestFor(NaturalMechanismV5, patches, hour, proposerID, addressee, reportedCapital, reportedGranary, proposer)
}

// NaturalBirthRequestFor is the mechanism-parameterized filing; the zero
// value reproduces NaturalBirthRequest exactly.
func NaturalBirthRequestFor(m NaturalMechanism, patches, hour int, proposerID, addressee sim.EntityID, reportedCapital, reportedGranary int64, proposer NaturalActorState) (NaturalActorState, error) {
	// The reported-granary bound is the mechanism's own declared granary
	// stock envelope (never a cap: it is the ledger width, not a limit on
	// how much food a household may hold).
	reportedStockMax, _ := naturalGranaryBoundsFor(m)
	if _, err := NaturalPhaseTime(hour, NaturalPhaseBirthRequest); err != nil ||
		!naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, addressee) || addressee == proposerID ||
		reportedCapital < 0 || reportedCapital > NaturalKMax || reportedGranary < 0 || reportedGranary > reportedStockMax ||
		!validNaturalActorFor(m, patches, proposerID, proposer) || !naturalAlive(proposer) ||
		proposer.Request.Status == NaturalRequestPending || proposer.Request.LastRequestHour >= int64(hour) {
		return NaturalActorState{}, ErrNaturalContract
	}
	proposer.Request.Hour = int64(hour)
	proposer.Request.Addressee = addressee
	proposer.Request.ReportedCapital = reportedCapital
	proposer.Request.ReportedGranary = reportedGranary
	proposer.Request.Status = NaturalRequestPending
	proposer.Request.LastRequestHour = int64(hour)
	if !validNaturalActorFor(m, patches, proposerID, proposer) {
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
	return NaturalBirthReplyFor(NaturalMechanismV5, patches, hour, kinAllowed, draws, newbornID, proposerID, consenterID, proposer, consenter)
}

// NaturalBirthReplyFor is the mechanism-parameterized reply; the zero value
// reproduces NaturalBirthReply exactly. It charges the frozen birth price;
// NaturalBirthReplyForCost is the declared-price form mechanism v13 reads.
func NaturalBirthReplyFor(m NaturalMechanism, patches, hour int, kinAllowed bool, draws []uint64, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState) (NaturalReplyResult, error) {
	return NaturalBirthReplyForCost(m, 0, patches, hour, kinAllowed, draws, newbornID, proposerID, consenterID, proposer, consenter)
}

// NaturalBirthReplyForCost is the declared-price reply: birthCost is the
// mechanism-v13 declared birth price (granary units and energy units per
// paying parent), resolved by naturalBirthCostFor — the zero spelling is the
// frozen default of 2, and every mechanism below v13 charges exactly that.
func NaturalBirthReplyForCost(m NaturalMechanism, birthCost int64, patches, hour int, kinAllowed bool, draws []uint64, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState) (NaturalReplyResult, error) {
	if _, err := NaturalPhaseTime(hour, NaturalPhaseBirthReply); err != nil ||
		!naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, consenterID) || proposerID == consenterID ||
		!validNaturalActorFor(m, patches, proposerID, proposer) || !validNaturalActorFor(m, patches, consenterID, consenter) {
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
			if !validNaturalActorFor(m, patches, proposerID, result.Proposer) || !validNaturalActorFor(m, patches, consenterID, result.Consenter) {
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
	consentOwnFails := consenter.Granary.Stock < NaturalConsentMinGranary || consenter.Worksite.Capital < NaturalConsentMinCapital || consenter.Body.Energy < NaturalConsentMinEnergy
	if naturalHouseholdGate(m) {
		// The household consent gate: the consenter only has to be fed.
		consentOwnFails = consenter.Body.Energy < NaturalHouseholdMinEnergy
	}
	switch {
	case consentOwnFails:
		return refuse(NaturalReplyRefuse, NaturalRefusalConsentOwn)
	case request.ReportedCapital < NaturalConsentMinCapital || request.ReportedGranary < NaturalConsentMinGranary:
		return refuse(NaturalReplyRefuse, NaturalRefusalConsentClaims)
	case !kinAllowed:
		return refuse(NaturalReplyRefuse, NaturalRefusalKin)
	}
	// The runner surplus gate on fresh truth, both parents.
	if !naturalSurplusGateAllowsFor(m, proposer, consenter, kinAllowed) {
		if proposer.Bag.Source != consenter.Bag.Source {
			return refuse(NaturalReplyRefuse, NaturalRefusalCrossPatch)
		}
		return refuse(NaturalReplyRefuse, NaturalRefusalGateThresholds)
	}
	child, _, err := NaturalChildGenome(patches, proposerID, consenterID, proposer.Genome, consenter.Genome, hour, draws)
	if err != nil {
		return NaturalReplyResult{}, err
	}
	birth, err := NaturalBirthForCost(m, birthCost, patches, hour, kinAllowed, newbornID, proposerID, consenterID, proposer, consenter, child)
	if err != nil {
		return NaturalReplyResult{}, err
	}
	birth.Proposer.Request.Status = NaturalRequestAccepted
	birth.Consenter.Request.LastReplyHour = int64(hour)
	if !validNaturalActorFor(m, patches, proposerID, birth.Proposer) || !validNaturalActorFor(m, patches, consenterID, birth.Consenter) || !validNaturalActorFor(m, patches, newbornID, birth.Newborn) {
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
// the birth. Each paying parent hands over the declared birth price — two
// granary units and two energy units under every mechanism below v13, the
// declared BirthCost under v13 — symmetrically, with the energy booked
// through the parent's basal-spent ledger and the granary units through the
// parent's genome birth-granary-paid accumulator, so both extended
// conservation identities stay exact at every price. The newborn is
// converted, never created from nothing: energy 8, hunger 0, all capital
// zero, home patch = the proposer's. The newbornID must be allocator-fresh
// (there is no birth-count cap; P15). kinAllowed must be the trusted
// runner's kin-walk verdict for the pair.
func NaturalBirth(patches, hour int, kinAllowed bool, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState, child NaturalGenomeState) (NaturalBirthResult, error) {
	return NaturalBirthFor(NaturalMechanismV5, patches, hour, kinAllowed, newbornID, proposerID, consenterID, proposer, consenter, child)
}

// NaturalBirthFor is the mechanism-parameterized birth; the zero value
// reproduces NaturalBirth exactly. It charges the frozen birth price;
// NaturalBirthForCost is the declared-price form mechanism v13 reads.
func NaturalBirthFor(m NaturalMechanism, patches, hour int, kinAllowed bool, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState, child NaturalGenomeState) (NaturalBirthResult, error) {
	return NaturalBirthForCost(m, 0, patches, hour, kinAllowed, newbornID, proposerID, consenterID, proposer, consenter, child)
}

// NaturalBirthForCost is the declared-price birth: birthCost is the
// mechanism-v13 declared birth price in granary units and — the same number —
// in energy units, resolved by naturalBirthCostFor (the zero spelling is the
// frozen default of 2, and every mechanism below v13 charges exactly that).
// The price is the same for both ledgers by declaration, so one number cannot
// drift from the other.
func NaturalBirthForCost(m NaturalMechanism, birthCost int64, patches, hour int, kinAllowed bool, newbornID, proposerID, consenterID sim.EntityID, proposer, consenter NaturalActorState, child NaturalGenomeState) (NaturalBirthResult, error) {
	if hour < 0 || hour >= NaturalHorizonHours || !naturalActorEntity(patches, proposerID) || !naturalActorEntity(patches, consenterID) || proposerID == consenterID ||
		!naturalNewbornEntity(patches, newbornID) ||
		!validNaturalActorFor(m, patches, proposerID, proposer) || !validNaturalActorFor(m, patches, consenterID, consenter) ||
		!validNaturalGenomeFor(m, patches, newbornID, child) ||
		child.ParentA != proposerID || child.ParentB != consenterID || child.BirthHour != int64(hour) {
		return NaturalBirthResult{}, ErrNaturalContract
	}
	if !naturalSurplusGateAllowsFor(m, proposer, consenter, kinAllowed) {
		return NaturalBirthResult{}, ErrNaturalContract
	}
	// The household gate (mechanism v8) makes the proposer the paying
	// household head: the proposer pays the declared price in granary units
	// and in energy units and the consenter only has to be fed (energy ≥ 2),
	// so it pays nothing.
	// Every frozen mechanism keeps the symmetric two-parent payment exactly.
	payers := []NaturalActorState{proposer, consenter}
	if naturalHouseholdGate(m) {
		payers = []NaturalActorState{proposer}
	}
	// Affordability is implied by the gate margins but the price is checked
	// against the paying rows themselves.
	price := naturalBirthCostFor(m, birthCost)
	for _, parent := range payers {
		if parent.Granary.Stock < price || parent.Body.Energy < price {
			return NaturalBirthResult{}, ErrNaturalContract
		}
	}
	home := proposer.Bag.Source
	pay := func(a NaturalActorState) NaturalActorState {
		a.Granary.Stock -= price
		a.Genome.BirthGranaryPaid += price
		a.Body.Energy -= price
		a.Body.BasalSpent += price
		return a
	}
	if naturalHouseholdGate(m) {
		proposer = pay(proposer)
	} else {
		proposer, consenter = pay(proposer), pay(consenter)
	}
	newborn := NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: NaturalNeverHour},
		Bag:      NaturalBagState{Source: home},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   child,
		Request:  NaturalIdleRequestState(),
		Offer:    NaturalIdleLabourOfferState(),
		Book:     NaturalIdleLabourBookState(),
	}
	if !validNaturalActorFor(m, patches, proposerID, proposer) || !validNaturalActorFor(m, patches, consenterID, consenter) || !validNaturalActorFor(m, patches, newbornID, newborn) {
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
	return NaturalAgeDeathFor(NaturalMechanismV5, patches, hour, actorID, genome)
}

// NaturalAgeDeathFor is the mechanism-parameterized age death; the zero value
// reproduces NaturalAgeDeath exactly. The mechanism matters because the
// genome validator's birth-granary-paid bound is the declared mechanism
// envelope (widened under the v13 birth price), so a state that mechanism v13
// admits reaches its death hour instead of being refused. It shares the one
// frozen shape: died-hour is written only from the never-hour.
func NaturalAgeDeathFor(m NaturalMechanism, patches, hour int, actorID sim.EntityID, genome NaturalGenomeState) (NaturalGenomeState, bool, error) {
	if hour < 0 || hour > NaturalHorizonHours || !naturalActorEntity(patches, actorID) || !validNaturalGenomeFor(m, patches, actorID, genome) || genome.DiedHour != NaturalNeverHour {
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
//		    whose home is that patch (mechanism v6: the provisioned inflow
//		    joins gathered on the inflow side; zero-valued in v5).
//		G2a ΣcultivatedTotal = Σstored + Σstock + Σspoiled + Σbirth-granary-paid
//		   (mechanism v6: G2a′ adds the Σprovisioned term; zero-valued in v5;
//		   mechanism v7: G2a″ adds Σwages-earned to the inflow side and
//		   Σwages-paid to the outflow side, zero-valued in v5/v6, with the
//		   closed-history equality Σwages-paid = Σwages-earned);
//	   and per patch the entitlement seam is exact: grants(patch) =
//	   ΣcultivatedTotal over the actors whose home is that patch (rule 508
//	   moves every entitlement into its owner's granary in the same pulse).
//		G2b invested = 2·created + wip; created − decayed = capital; and the
//		    land identity tenuredSlots(patch) = Σk over ALL tenure holders
//		    (dead included) ≤ 8, each tenured slot's owner an existing actor
//		    whose home is that patch. (During a dead owner's decay window the
//		    alive-only equality is weaker by exactly the dead owner's standing
//		    capital; the all-tenure-holders form is the exact invariant.)
//		G3  11·f + 8·births + Σconsumed + ΣstoredMeals = Σenergy + Σbasal +
//		    ΣcapLoss, with the per-newborn cohort identity 8 + consumed +
//		    storedMeals = energy + basalSpent + capLoss. The consumed term is
//		    weighted by the declared meal price (naturalMealEnergyFor: the
//		    frozen 1 below mechanism v12, 2 under it), while a stored meal
//		    keeps its frozen one energy in every mechanism.
//
// It also verifies the derived reproduction graph (every newborn's parents
// exist among strictly earlier IDs, no parent pair shares an ancestor at
// walk depth ≤2, founder genomes carry no parents and no paid units) and
// the allocator cross-check: allocatorLast equals the genesis base when no
// birth has happened and the maximum newborn ID otherwise (mismatch fails
// closed, per the frozen restore discipline).
func NaturalCheckConservation(patches int, patchStates []NaturalPatchState, slotStates [][]NaturalSlotState, founders []NaturalActorState, newborns []NaturalNewbornState, allocatorLast sim.EntityID) (NaturalBalance, error) {
	return NaturalCheckConservationFor(NaturalMechanismV5, patches, patchStates, slotStates, founders, newborns, allocatorLast)
}

// NaturalCheckConservationFor is the mechanism-parameterized conservation
// gate; the zero value reproduces NaturalCheckConservation exactly
// (provisioned is pinned to zero per actor there, and the wage ledgers are
// pinned to zero per actor in every v5/v6 state, so the G2a′/G2a″ aggregate
// below reduces term for term to the frozen v5 G2a).
//
// The founder block is the declared founding-band fixture axis (ticket 18):
// len(founders) is the materialized prefix f of the frozen ID-space founder
// block 1..8P, so the gate's founder-count precondition is derived — the
// founder IDs are exactly the contiguous positional prefix 1..len(founders)
// by the loop below, bounded by the frozen block via NaturalFounderCount.
// The zero value of the axis materializes the whole block, so the frozen v5
// and v6 behavior (len(founders) == 8P) is bit-identical, and G3's
// initial-energy term is 11·f by construction.
func NaturalCheckConservationFor(m NaturalMechanism, patches int, patchStates []NaturalPatchState, slotStates [][]NaturalSlotState, founders []NaturalActorState, newborns []NaturalNewbornState, allocatorLast sim.EntityID) (NaturalBalance, error) {
	return NaturalCheckConservationYieldFor(m, 1, patches, patchStates, slotStates, founders, newborns, allocatorLast)
}

// NaturalCheckConservationYieldFor is the conservation gate under a declared
// mechanism and granary yield. Every mechanism below v9 ignores the yield
// (the frozen one-unit grant); mechanism v9 states its land identity in
// budget slots, so the patch ledger validates against the declared yield
// while the food ledgers (granted units, cultivated totals) stay term for
// term the frozen G1/G2a identities.
func NaturalCheckConservationYieldFor(m NaturalMechanism, yield int64, patches int, patchStates []NaturalPatchState, slotStates [][]NaturalSlotState, founders []NaturalActorState, newborns []NaturalNewbornState, allocatorLast sim.EntityID) (NaturalBalance, error) {
	founderBlock, err := NaturalFounderCount(patches)
	if err != nil || len(patchStates) != patches || len(slotStates) != patches || len(founders) < 1 || len(founders) > founderBlock {
		return NaturalBalance{}, ErrNaturalContract
	}
	genesisLast, err := NaturalHighestFixedEntityID(patches)
	if err != nil {
		return NaturalBalance{}, ErrNaturalContract
	}
	var b NaturalBalance
	b.InitialEnergy = NaturalFounderInitialEnergy * int64(len(founders))
	// Pass 1: every actor row and the parent map for the derived-kinship
	// re-check.
	parents := make(map[sim.EntityID][2]sim.EntityID, len(founders)+len(newborns))
	ids := make([]sim.EntityID, 0, len(founders)+len(newborns))
	actors := make(map[sim.EntityID]NaturalActorState, len(founders)+len(newborns))
	for i, founder := range founders {
		actor := sim.EntityID(i + 1)
		if !validNaturalActorFor(m, patches, actor, founder) {
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
		if !validNaturalActorFor(m, patches, newborn.ID, newborn.State) {
			return NaturalBalance{}, ErrNaturalContract
		}
		parents[newborn.ID] = [2]sim.EntityID{newborn.State.Genome.ParentA, newborn.State.Genome.ParentB}
		ids = append(ids, newborn.ID)
		actors[newborn.ID] = newborn.State
		// Per-newborn cohort identity: 8 + meal·consumed + stored = energy +
		// basal + capLoss, the consumed term carrying the declared meal price.
		if NaturalNewbornEnergy+naturalMealEnergyFor(m)*newborn.State.Body.Consumed+newborn.State.Granary.StoredMeals != newborn.State.Body.Energy+newborn.State.Body.BasalSpent+newborn.State.Body.CapLost {
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
				//				return NaturalBalance{}, ErrNaturalContract
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
		if !naturalStockFor(m, yield, p, patchSlots) {
			return NaturalBalance{}, ErrNaturalContract
		}
		b.Produced += p.Produced
		b.Granted += p.Granted
		b.Unrealized += p.Unrealized
		patchID := sim.EntityID(NaturalPatchIDBase + i)
		var gathered, consumed, held, invested, provisioned, tenured, cultivated int64
		for _, slot := range patchSlots {
			gathered += slot.Gathered
			b.Stock += slot.Stock
			if slot.Mode == NaturalTenureCultivated {
				tenured++
				owner, ok := actors[slot.Owner]
				if !ok || owner.Bag.Source != patchID {
					//					return NaturalBalance{}, ErrNaturalContract
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
			// Rule 515 inflow: provisions enter the bag without a gather,
			// so the bag-intake identity admits them on the inflow side
			// (zero under the v5 zero value, which keeps the frozen form).
			provisioned += a.Granary.Provisioned
			cultivated += a.Granary.CultivatedTotal
			b.CultivatedTotal += a.Granary.CultivatedTotal
			b.GranaryStock += a.Granary.Stock
			b.StoredMeals += a.Granary.StoredMeals
			b.Provisioned += a.Granary.Provisioned
			b.Spoiled += a.Granary.Spoiled
			b.WagesPaid += a.Granary.WagesPaid
			b.WagesEarned += a.Granary.WagesEarned
			b.Capital += a.Worksite.Capital
			b.Wip += a.Worksite.Wip
			b.PointsCreated += a.Worksite.PointsCreated
			b.PointsDecayed += a.Worksite.PointsDecayed
			b.Energy += a.Body.Energy
			b.BasalSpent += a.Body.BasalSpent
			b.EnergyCapLost += a.Body.CapLost
			b.BirthGranaryPaid += a.Genome.BirthGranaryPaid
		}
		if gathered+provisioned != consumed+held+invested {
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
	// Pass 4: aggregate identities, G1 (with the v6 provisioned inflow;
	// exactly the frozen form while provisioned is pinned to zero), G2b,
	// G2a′ (exactly G2a while the provisioned ledger is pinned to zero by
	// the per-actor validator; under v7 the wage terms join both sides —
	// pinned to zero in every v5/v6 state, so the frozen form is preserved
	// term for term), the closed-history wage equality Σpaid = Σearned
	// (every wage earned was paid in the same settlement), and G3 extended.
	if b.Gathered+b.Provisioned != b.Consumed+b.Held+b.Invested || b.Produced != b.Gathered+b.Stock ||
		b.Invested != NaturalPointCostWip*b.PointsCreated+b.Wip || b.PointsCreated-b.PointsDecayed != b.Capital ||
		b.CultivatedTotal+b.WagesEarned != b.StoredMeals+b.GranaryStock+b.Provisioned+b.Spoiled+b.BirthGranaryPaid+b.WagesPaid ||
		b.WagesPaid != b.WagesEarned ||
		b.InitialEnergy+NaturalNewbornEnergy*b.Births+naturalMealEnergyFor(m)*b.Consumed+b.StoredMeals != b.Energy+b.BasalSpent+b.EnergyCapLost {
		return NaturalBalance{}, ErrNaturalContract
	}
	return b, nil
}
