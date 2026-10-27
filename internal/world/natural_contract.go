package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Natural v5 is the frozen natural-limits world (ticket 15). Its component,
// rule, and projection identities are disjoint from v1 (food-flow), v2
// (social-food), v3 (capacity), v4 (reproduction), and the S6 world: the
// type-ID block 0x80000050-0x8000005A, rules 501-514, and version 5 never
// collide with any earlier world, so no history can be replayed across world
// versions by accident. The frozen design section of
// .scratch/15-natural-limits/spec.md (including the recorded corrigenda) is
// binding; every constant below is quoted from it, not derived. The v4
// arithmetic this world inherits (gather, consume, build economics, the
// basal and wear divisor loops, birth request/reply/consent/kinship) stays
// byte-identical; production is re-grounded in land (the patch regeneration
// budget R), storage is bounded by spoilage instead of a cap, and births are
// cap-free with entity identity from the kernel's sim.EntityAllocator.
//
// Corrigenda recorded in the spec (2026-09-29, supervisor-approved):
//   - allocator genesis last-ID = the highest fixed entity ID (2000+8P), so
//     newborn IDs ascend 2000+8P+1, 2000+8P+2, ... ("17, 18, ..." in the
//     draft was a slip); the ID space stays partitioned forever and typed
//     failure occurs only at MaxUint64 (P15).
//   - rule 508 grants are flat: exactly one granary unit per entitled
//     cultivated slot per pulse. The v4 yield divisor loop and its debt
//     field are retired with yield-unrealized (they were the
//     yield-from-nothing machinery); locus-y stays heritable but is
//     production-neutral in v5, so no locus can mint food beyond R.
//   - the land identity holds over ALL tenure holders (dead included):
//     tenured slots(patch) = Σk over owners, ≤ 8. During a dead owner's
//     decay window the alive-only equality is weaker, exactly as documented.
//
// Ticket 17 declares the mechanism-version axis on top of these frozen v5
// bytes: NaturalMechanismV5 is the zero value and reproduces every
// descriptor, version, and name below byte-identically, while
// NaturalMechanismV6 (world axis version 6 — distinct from the policy
// formats natural/5 | natural/6 | natural/7) additively registers rule 515
// "provision", the granary provisioned ledger field, the G2a′ identity, and
// the widened G5 writer admissions (granary stock, provisioned,
// last-stored-meal-hour, bag units under 515). No new component is created:
// the type-ID block stays 0x80000050-0x8000005A with 0x80000058-0x8000005A
// unused, because provision writes existing granary and bag rows plus the
// version-gated granary field. No frozen constant and no frozen v5 field
// list is mutated; if parameterization ever required that, the declared
// stop condition is to split the world into a parallel file family instead.
//
// Ticket 20 (the frozen v9 wage-contract design,
// .scratch/20-exchange-design/spec.md) declares NaturalMechanismV7 (world
// axis version 7), strictly additive over v6: rules 516 labour-offer,
// 517 labour-decision, 518 labour-settlement, and 519 labour-expiry/void,
// the two recorded-block components 0x80000058 natural-labour-offer and
// 0x80000059 natural-labour-book (0x8000005A stays spare in every
// version), the granary wage ledgers 8 wages-paid / 9 wages-earned, the
// G2a″ identity, and the enumerated G5 admissions. Everything the v5 and
// v6 mechanisms produce stays byte-identical; the v5/v6 rule tables and
// the frozen six-entry granary field list are pinned by the registry
// fingerprint goldens captured from the pre-ticket-20 revision.
var ErrNaturalContract = errors.New("invalid natural v5 contract")

const (
	// The frozen v5 version fields. They are exactly what the zero value of
	// the mechanism axis (NaturalMechanismV5) produces; NaturalMechanismV6
	// produces the corresponding version 6 fields through its accessors.
	NaturalFormatVersion     uint32                = 5
	NaturalSchemaVersion     sim.SchemaVersion     = 5
	NaturalRuleVersion       uint32                = 5
	NaturalProjectionVersion sim.ProjectionVersion = 5

	// Fixture shape: P patches is a fixture parameter (the frozen matrix
	// also uses P=32); the default fixture is P=2 with 8P founders.
	NaturalDefaultPatches = 2
	NaturalSlotsPerPatch  = 8
	NaturalHorizonHours   = 168

	// Frozen v3/v4 clock, body, and economics constants, inherited verbatim.
	NaturalHour           sim.Duration = 3_600_000_000
	NaturalGatherDuration sim.Duration = 20 * 60 * 1_000_000
	NaturalBuildDuration  sim.Duration = 20 * 60 * 1_000_000

	NaturalFounderInitialEnergy int64 = 11
	NaturalNewbornEnergy        int64 = 8
	NaturalEnergyCapacity       int64 = 12
	NaturalHungerCapacity       int64 = 24
	NaturalBagCapacity          int64 = 1
	NaturalNeverHour            int64 = -1

	NaturalPointCostWip int64 = 2
	NaturalKMax         int64 = 8
	NaturalWipMax       int64 = 1

	// Frozen genome constants: three loci with alleles in [5,7], neutral 6,
	// and the shared numerator 6 of the basal accrual. Locus-y is heritable
	// but production-neutral in v5 (corrigendum: flat grants); the basal
	// (13) and wear (14) debt maxima are the live divisor-loop transients.
	NaturalGenomeLocusMin    int64 = 5
	NaturalGenomeLocusMax    int64 = 7
	NaturalGenomeNeutral     int64 = 6
	NaturalGenomeDenominator int64 = 6
	NaturalBasalDebtMax      int64 = 13
	NaturalWearDebtMax       int64 = 14

	// Frozen birth economics verbatim: each parent pays two granary units
	// and two energy units, the energy booked through the parent's
	// basal-spent ledger; the eight converted units become the newborn's
	// energy. There is no birth-count cap anywhere in v5.
	NaturalParentGranaryCost int64 = 2
	NaturalParentEnergyCost  int64 = 2

	// Frozen consent thresholds and derived-kinship limits, verbatim v4.
	NaturalConsentMinGranary int64 = 3
	NaturalConsentMinCapital int64 = 2
	NaturalConsentMinEnergy  int64 = 3
	// NaturalHouseholdMinEnergy is the mechanism-v8 household consent gate:
	// the consenter only has to be fed (energy ≥ 2), while the proposer keeps
	// the full wealth gate above.
	NaturalHouseholdMinEnergy int64 = 2
	NaturalKinProhibitedDepth int   = 2
	NaturalKinWalkLimit       int   = 10

	// Exactly seven draws per birth from the birth stream at position
	// 7×(ordinal−1): three crossover, three mutation, one lifetime
	// (96 + draw mod 48 ∈ [96,143]). Founder lifetimes come from the
	// founder stream; the draws themselves are runner-owned.
	NaturalBirthStream        sim.StreamID = 0x47454E33
	NaturalFounderStream      sim.StreamID = 0x464F554E
	NaturalDrawsPerBirth                   = 7
	NaturalLifetimeBase                    = 96
	NaturalLifetimeSpan                    = 48
	NaturalFounderLifetimeMin              = 96
	NaturalFounderLifetimeMax              = 143

	// Frozen v5 spoilage convention S: preserved food keeps about one day;
	// a declared nature convention (like the wear period), not a cap.
	NaturalSpoilagePeriod int64 = 24

	// Widened integer-exact ledger bounds, each a horizon envelope that
	// cannot bind behavior before the horizon (P15): granary inflow is at
	// most 8 units per hour (Σk ≤ 8 slots), basal-spent drains at most two
	// units per hour (locus-m 5) plus four per hour of birth payments
	// (proposer and consenter in the same hour), and birth-granary-paid is
	// at most four units per hour.
	NaturalGranaryStockMax int64 = 8 * NaturalHorizonHours                           // 1344
	NaturalSpoilDebtMax    int64 = 8*NaturalHorizonHours + NaturalSpoilagePeriod - 1 // 1367
	NaturalSpoiledMax      int64 = 8 * NaturalHorizonHours                           // 1344

	// NaturalYieldMax is the declared upper bound of mechanism v9's Yield
	// parameter (granary units granted per cultivated slot per patch-hour):
	// at most one unit per slot of the patch, i.e. NaturalSlotsPerPatch. The
	// v9-only widened ledger envelope below is derived from it, so the food
	// ledgers of every mechanism below v9 keep exactly their frozen bounds.
	NaturalYieldMax int64 = NaturalSlotsPerPatch

	// naturalYieldGranaryMax is the mechanism-v9 ledger envelope: with every
	// cultivated slot granting NaturalYieldMax units per patch-hour, a
	// granary can receive at most NaturalSlotsPerPatch times the frozen
	// horizon envelope. Like every ledger bound it cannot bind behaviour
	// before h=168; it is a declared envelope, never a behaviour cap.
	naturalYieldGranaryMax           = NaturalGranaryStockMax * NaturalSlotsPerPatch // 10752
	NaturalBasalSpentMax       int64 = 6 * NaturalHorizonHours                       // 1008
	NaturalBirthGranaryPaidMax int64 = 4 * NaturalHorizonHours                       // 672

	// NaturalProvisionedMax is the mechanism-v6-only provisioned-ledger
	// envelope (ticket 17): at most one granary draw per actor-hour — the
	// stored-meal draw and the provision share the LastStoredMealHour
	// guard — so provisions cannot exceed the horizon.
	NaturalProvisionedMax int64 = NaturalHorizonHours // 168

	// NaturalWagesMax is the mechanism-v7-only wage-ledger envelope (ticket
	// 20): a worker earns at most one wage per hour (the bag discipline —
	// one delivery per gathered unit per hour), and an employer pays at
	// most one wage per settlement while every settlement invests exactly
	// one worksite unit, so neither wage ledger can pass the horizon
	// envelope. Like every ledger bound it is a horizon envelope that
	// cannot bind behavior before h=168 (P15), never a behaviour cap.
	NaturalWagesMax int64 = NaturalHorizonHours // 168
)

const (
	NaturalPatchIDBase = 1001
	NaturalSlotIDBase  = 2001

	// NaturalMaxPatchesFixedIDSpace is derived, not declared: the founder block
	// 1..8P must stay strictly below the patch block starting at 1001, so the
	// fixed-block ID space is disjoint for every valid P.
	NaturalMaxPatchesFixedIDSpace = (NaturalPatchIDBase - 1) / NaturalSlotsPerPatch

	// The frozen v5 type-ID block. The eight components below occupy
	// 0x80000050-0x80000057 contiguously; 0x80000058-0x8000005A stay inside
	// the frozen block. Under the v5 and v6 mechanisms all three remain
	// unused; mechanism v7 registers the wage-contract rows on 0x80000058
	// and 0x80000059 (ticket 20, inside the recorded free block), and
	// 0x8000005A stays spare in every version forever.
	NaturalTypeBlockFirst sim.ComponentTypeID = 0x80000050
	NaturalTypeBlockLast  sim.ComponentTypeID = 0x8000005A

	NaturalPatchTypeID        sim.ComponentTypeID = 0x80000050
	NaturalSlotTypeID         sim.ComponentTypeID = 0x80000051
	NaturalBagTypeID          sim.ComponentTypeID = 0x80000052
	NaturalBodyTypeID         sim.ComponentTypeID = 0x80000053
	NaturalWorksiteTypeID     sim.ComponentTypeID = 0x80000054
	NaturalGranaryTypeID      sim.ComponentTypeID = 0x80000055
	NaturalGenomeTypeID       sim.ComponentTypeID = 0x80000056
	NaturalBirthRequestTypeID sim.ComponentTypeID = 0x80000057

	// The mechanism-v7-only wage-contract components (ticket 20). The offer
	// row is the worker's addressed labour offer (the birth-request field
	// set minus the self-reported claims); the book row is the employer's
	// single engagement slot.
	NaturalLabourOfferTypeID sim.ComponentTypeID = 0x80000058
	NaturalLabourBookTypeID  sim.ComponentTypeID = 0x80000059

	NaturalRegenSplitRule   sim.RuleID = 501 // regen-split (world, pulse)
	NaturalSpoilageRule     sim.RuleID = 502 // spoilage (world, pulse)
	NaturalWearRule         sim.RuleID = 503 // wear (world, pulse; slot revert)
	NaturalBasalRule        sim.RuleID = 504 // basal (world, pulse; starvation)
	NaturalBuildRule        sim.RuleID = 505 // build (actor; slot claim)
	NaturalGatherRule       sim.RuleID = 506 // gather
	NaturalConsumeRule      sim.RuleID = 507 // consume
	NaturalGrantRule        sim.RuleID = 508 // yield-grant (world, pulse; flat)
	NaturalEatStoredRule    sim.RuleID = 509 // eat-stored
	NaturalBirthRequestRule sim.RuleID = 510 // birth-request
	NaturalBirthReplyRule   sim.RuleID = 511 // birth-reply (allocates the newborn)
	NaturalBirthRefuseRule  sim.RuleID = 512 // birth-refusal (request rows only)
	NaturalBirthExpireRule  sim.RuleID = 513 // birth-expiry (world, pulse)
	NaturalAgeDeathRule     sim.RuleID = 514 // age-death (world, pulse)

	// NaturalProvisionRule (515) exists only under NaturalMechanismV6: at
	// the frozen claim instant an actor holding no bag unit converts one
	// granary stock unit into one bag unit (stock−1, provisioned+1,
	// LastStoredMealHour = hour, bag+1). The v5 rule table stays 501-514.
	NaturalProvisionRule sim.RuleID = 515 // provision (actor; claim instant; mechanism v6 only)

	// The wage-contract rules (ticket 20) exist only under
	// NaturalMechanismV7; the v5 and v6 rule tables stay exactly 501-515.
	// Rule 516 labour-offer files the worker's addressed offer on its own
	// labour-offer row at h+40m+5µs (the birth-request instant). Rule 517
	// labour-decision is the addressed employer's accept/refuse at
	// h+40m+6µs (the birth-reply instant), one decision per actor-hour.
	// Rule 518 labour-settlement is the atomic wage settlement caused by
	// the worker at h+40m+3µs (the build-completion instant), folded with
	// the frozen point completion when wip reaches 2. Rule 519
	// labour-expiry/void is the world-caused pulse rule: unanswered offers
	// expire and void contracts clear (the 513 discipline verbatim).
	NaturalLabourOfferRule      sim.RuleID = 516 // labour-offer (actor; h+40m+5µs; mechanism v7 only)
	NaturalLabourDecisionRule   sim.RuleID = 517 // labour-decision (actor; h+40m+6µs; mechanism v7 only)
	NaturalLabourSettlementRule sim.RuleID = 518 // labour-settlement (actor; h+40m+3µs; mechanism v7 only)
	NaturalLabourExpireRule     sim.RuleID = 519 // labour-expiry/void (world, pulse; mechanism v7 only)
)

const (
	NaturalPatchYieldField sim.FieldID = 1 + iota
	NaturalPatchPulsesField
	NaturalPatchProducedField
	NaturalPatchGrantedField
	NaturalPatchUnrealizedField
)
const (
	NaturalSlotStockField sim.FieldID = 1 + iota
	NaturalSlotPatchField
	NaturalSlotGatheredField
	NaturalSlotTenureModeField
	NaturalSlotTenureOwnerField
	NaturalSlotTenureCultivatedField
)
const (
	NaturalBagUnitsField sim.FieldID = 1 + iota
	NaturalBagSourceField
)
const (
	NaturalBodyEnergyField sim.FieldID = 1 + iota
	NaturalBodyHungerField
	NaturalBodyBasalSpentField
	NaturalBodyCapLostField
	NaturalBodyConsumedField
	NaturalBodyLastGatherHourField
	NaturalBodyBasalDebtField
)
const (
	NaturalWorksiteCapitalField sim.FieldID = 1 + iota
	NaturalWorksiteWipField
	NaturalWorksiteWearDebtField
	NaturalWorksiteInvestedUnitsField
	NaturalWorksitePointsCreatedField
	NaturalWorksitePointsDecayedField
	NaturalWorksiteLastBuildHourField
)
const (
	NaturalGranaryStockField sim.FieldID = 1 + iota
	NaturalGranaryCultivatedTotalField
	NaturalGranaryStoredMealsField
	NaturalGranaryLastStoredMealHourField
	NaturalGranarySpoilDebtField
	NaturalGranarySpoiledField

	// NaturalGranaryProvisionedField (7) is the mechanism-v6-only ledger of
	// granary units drawn as worksite provisions (rule 515). It is appended
	// after the frozen v5 fields so the frozen six identities never shift;
	// the v5 granary schema keeps exactly fields 1-6.
	NaturalGranaryProvisionedField

	// NaturalGranaryWagesPaidField (8) and NaturalGranaryWagesEarnedField
	// (9) are the mechanism-v7-only wage ledgers of rule 518 (ticket 20):
	// units paid as wages by an employer and received as wages by a worker.
	// They are appended after the provisioned field so the frozen six
	// identities and the v6 seven-field list never shift.
	NaturalGranaryWagesPaidField
	NaturalGranaryWagesEarnedField
)
const (
	NaturalGenomeLocusMField sim.FieldID = 1 + iota
	NaturalGenomeLocusYField
	NaturalGenomeLocusWField
	NaturalGenomeParentAField
	NaturalGenomeParentBField
	NaturalGenomeBirthHourField
	NaturalGenomeDeathHourField
	NaturalGenomeDiedHourField
	NaturalGenomeBirthGranaryPaidField
)
const (
	NaturalBirthRequestHourField sim.FieldID = 1 + iota
	NaturalBirthRequestAddresseeField
	NaturalBirthRequestReportedCapitalField
	NaturalBirthRequestReportedGranaryField
	NaturalBirthRequestStatusField
	NaturalBirthRequestLastRequestHourField
	NaturalBirthRequestLastReplyHourField
)
const (
	// The mechanism-v7-only labour-offer row: the birth-request field set
	// minus the self-reported claims, terms fixed by the frozen design —
	// there is no claim field and no price field (the wage is derived 1:1).
	NaturalLabourOfferHourField sim.FieldID = 1 + iota
	NaturalLabourOfferAddresseeField
	NaturalLabourOfferStatusField
	NaturalLabourOfferLastOfferHourField
	NaturalLabourOfferLastReplyHourField
)
const (
	// The mechanism-v7-only labour-book row: the employer's single
	// engagement slot (engaged worker and the hour of engagement) plus the
	// one-decision-per-actor-hour bookkeeping.
	NaturalLabourBookEngagedWorkerField sim.FieldID = 1 + iota
	NaturalLabourBookEngagedHourField
	NaturalLabourBookLastDecisionHourField
)

// Frozen pulse order inside hour h, byte-identical to v4's frozen phase
// offsets: pulse stages (basal, regen-split, spoilage, wear) at the hour
// boundary; claims at h+1µs; gather completion at h+20m+1µs; bag decision at
// h+20m+2µs; build start at h+20m+3µs completing at h+40m+3µs with the
// atomic bag→wip/k conversion and slot claim; paired EatStored at
// h+40m+4µs; birth request at h+40m+5µs; birth reply at h+40m+6µs. The
// mechanism-v7 labour phases reuse the frozen instants additively — no new
// phase instant exists (ticket 20): settlement at the h+40m+3µs build
// completion, offer at the h+40m+5µs request instant, decision at the
// h+40m+6µs reply instant.
const (
	NaturalPhasePulse int = iota
	NaturalPhaseClaim
	NaturalPhaseGathered
	NaturalPhaseBagDecision
	NaturalPhaseBuildStart
	NaturalPhasePairedMeal
	NaturalPhaseBirthRequest
	NaturalPhaseBirthReply
	NaturalPhaseLabourSettle
	NaturalPhaseLabourOffer
	NaturalPhaseLabourDecision
)

// NaturalMechanism is the declared mechanism-version axis of the natural
// world (ticket 17 v7 design; extended by ticket 20's frozen v9 wage
// contract). The zero value is exactly the frozen v5 mechanism: every
// descriptor, version field, and name it produces is byte-identical to the
// frozen v5 bytes. NaturalMechanismV6 (world axis version 6) and
// NaturalMechanismV7 (world axis version 7) are the only other admitted
// values and are strictly additive: rule 515, the granary provisioned
// field, G2a′, and the widened writer admissions exist only under v6 and
// v7; the wage-contract rules 516-519, the two recorded-block components,
// the granary wage ledgers, G2a″, and their G5 admissions exist only under
// v7. Recording caution pinned by the design: this is the WORLD axis, never
// to be conflated with the policy formats natural/5 | natural/6 |
// natural/7 (world-axis 7 colliding typographically with policy natural/7
// is the pinned ticket-17 naming caution).
type NaturalMechanism uint16

const (
	// NaturalMechanismV5 is the zero value ≡ the frozen v5 world.
	NaturalMechanismV5 NaturalMechanism = 0
	// NaturalMechanismV6 adds rule 515 "provision" (world axis version 6).
	NaturalMechanismV6 NaturalMechanism = 6
	// NaturalMechanismV7 adds the wage contract (world axis version 7,
	// ticket 20): rules 516-519, components 0x80000058/0x80000059, granary
	// wage ledgers, G2a″, and the enumerated G5 admissions.
	NaturalMechanismV7 NaturalMechanism = 7
	// NaturalMechanismV8 (world axis version 8, ticket 49) is the v6 rule
	// table plus exactly two conception changes: the band-wide mate pool
	// (no co-location requirement) and the household consent gate (the
	// consenter only needs energy ≥ 2). Everything else, including rule 515
	// and the provisioned ledger, is v6 verbatim; the wage contract of v7
	// does not exist here.
	NaturalMechanismV8 NaturalMechanism = 8
	// NaturalMechanismV9 (world axis version 9, ticket 50) is the v8 rule
	// table plus exactly one declared parameter: Yield, the granary units
	// granted per cultivated slot per patch-hour (default 1; admitted
	// 1..NaturalSlotsPerPatch). Everything else, including rule 515 and the
	// provisioned ledger, is v8 verbatim.
	NaturalMechanismV9 NaturalMechanism = 9
	// NaturalMechanismV10 (world axis version 10, ticket 50) is the v9 rule
	// table plus exactly one placement change: the founding band is seated
	// round-robin across the declared patches instead of in ID blocks of
	// eight, so a band smaller than the land is spread over every patch.
	// Everything else, the declared yield included, is v9 verbatim.
	NaturalMechanismV10 NaturalMechanism = 10
)

// NaturalMechanismValid reports the admitted axis values: the zero value
// (≡ v5), 6, 7, 8, 9 and 10. Literal 5 is not a second spelling of v5 — the
// zero value is the one v5 spelling — and everything else fails closed.
func NaturalMechanismValid(m NaturalMechanism) bool {
	return m == NaturalMechanismV5 || m == NaturalMechanismV6 || m == NaturalMechanismV7 ||
		m == NaturalMechanismV8 || m == NaturalMechanismV9 || m == NaturalMechanismV10
}

// worldVersion is the world axis version number of the mechanism: 5 for the
// zero value, 6 for NaturalMechanismV6, 7 for NaturalMechanismV7, 8 for
// NaturalMechanismV8. Anything else is unreachable through NaturalRegistryFor
// and falls back to the frozen v5 identity.
func (m NaturalMechanism) worldVersion() uint32 {
	switch m {
	case NaturalMechanismV6:
		return 6
	case NaturalMechanismV7:
		return 7
	case NaturalMechanismV8:
		return 8
	case NaturalMechanismV9:
		return 9
	case NaturalMechanismV10:
		return 10
	}
	return 5
}

// FormatVersion parameterizes the frozen NaturalFormatVersion.
func (m NaturalMechanism) FormatVersion() uint32 { return m.worldVersion() }

// SchemaVersion parameterizes the frozen NaturalSchemaVersion.
func (m NaturalMechanism) SchemaVersion() sim.SchemaVersion {
	return sim.SchemaVersion(m.worldVersion())
}

// RuleVersion parameterizes the frozen NaturalRuleVersion.
func (m NaturalMechanism) RuleVersion() uint32 { return m.worldVersion() }

// ProjectionVersion parameterizes the frozen NaturalProjectionVersion.
func (m NaturalMechanism) ProjectionVersion() sim.ProjectionVersion {
	return sim.ProjectionVersion(m.worldVersion())
}

// Entity-ID space, fixed forever: actors 1..8P (founders) and the
// allocator-issued newborn block strictly above the highest fixed entity ID;
// patches 1001..1000+P; slots 2001..2000+8P. The blocks are disjoint for
// every valid P — the binding constraint is that the founder block must stay
// below the patch block, i.e. 8P < NaturalPatchIDBase — so a newborn ID can
// never collide with a fixed entity and typed failure occurs only at
// MaxUint64 (P15). P is a declared fixture parameter, so this is an explicit,
// typed precondition rather than a silent overlap; deriving the block bases
// from P (removing the precondition entirely) is the recorded follow-up if a
// fixture larger than NaturalMaxPatchesFixedIDSpace is ever needed.
func NaturalPatchCountValid(patches int) bool {
	return patches >= 1 && patches <= NaturalMaxPatchesFixedIDSpace
}

func NaturalFounderCount(patches int) (int, error) {
	if !NaturalPatchCountValid(patches) {
		return 0, ErrNaturalContract
	}
	return NaturalSlotsPerPatch * patches, nil
}

func NaturalPatchID(patches, index int) (sim.EntityID, error) {
	if index < 0 || index >= patches {
		return 0, ErrNaturalContract
	}
	return sim.EntityID(NaturalPatchIDBase + index), nil
}

func NaturalSlotID(patches, patchIndex, slotIndex int) (sim.EntityID, error) {
	if patchIndex < 0 || patchIndex >= patches || slotIndex < 0 || slotIndex >= NaturalSlotsPerPatch {
		return 0, ErrNaturalContract
	}
	return sim.EntityID(NaturalSlotIDBase + patchIndex*NaturalSlotsPerPatch + slotIndex), nil
}

// NaturalFounderPatchID maps only the ID-placed founders; a newborn's home
// patch is carried by its bag source-patch row, not derived from its ID.
func NaturalFounderPatchID(patches int, actor sim.EntityID) (sim.EntityID, error) {
	return NaturalFounderPatchIDFor(NaturalMechanismV5, patches, actor)
}

// NaturalFounderPatchIDFor is the mechanism-versioned founder placement.
// Every mechanism below v10 seats the founders in ID blocks of
// NaturalSlotsPerPatch (founder i on patch (i-1)/8), so their bytes are
// unchanged; mechanism v10 seats the band round-robin across the declared
// patch count (founder i on patch (i-1) mod P), so a band smaller than the
// land leaves free ground on every patch instead of crowding one.
func NaturalFounderPatchIDFor(m NaturalMechanism, patches int, actor sim.EntityID) (sim.EntityID, error) {
	founders, err := NaturalFounderCount(patches)
	if err != nil || actor < 1 || actor > sim.EntityID(founders) {
		return 0, ErrNaturalContract
	}
	index := (int(actor) - 1) / NaturalSlotsPerPatch
	if naturalRoundRobinFounders(m) {
		index = (int(actor) - 1) % patches
	}
	return NaturalPatchID(patches, index)
}

// NaturalHighestFixedEntityID is the highest fixed entity ID (2000+8P): the
// last slot of the last patch. It is the allocator's genesis last-ID.
func NaturalHighestFixedEntityID(patches int) (sim.EntityID, error) {
	founders, err := NaturalFounderCount(patches)
	if err != nil {
		return 0, err
	}
	return sim.EntityID(NaturalSlotIDBase - 1 + founders), nil
}

// NaturalGenesisAllocator is the frozen genesis state of the entity
// allocator: last-ID at the highest fixed entity ID, so the first newborn
// receives 2000+8P+1 and IDs ascend without range or cap.
func NaturalGenesisAllocator(patches int) (sim.EntityAllocator, error) {
	last, err := NaturalHighestFixedEntityID(patches)
	if err != nil {
		return sim.EntityAllocator{}, err
	}
	return sim.NewEntityAllocator(last), nil
}

// NaturalNextNewbornID derives the newborn ID for one birth from the
// runner-owned allocator. Typed failure only at MaxUint64 (P15); there is no
// range and no cap.
func NaturalNextNewbornID(allocator *sim.EntityAllocator) (sim.EntityID, error) {
	return allocator.Next()
}

func naturalFounderEntity(patches int, entity sim.EntityID) bool {
	founders, err := NaturalFounderCount(patches)
	return err == nil && entity >= 1 && entity <= sim.EntityID(founders)
}

// naturalNewbornEntity is the newborn validity predicate: strictly above the
// highest fixed entity ID, i.e. allocator-fresh forever. Fixed-block IDs
// (founders, patches, slots, and the unassigned stretch between them) are
// never valid newborn IDs.
func naturalNewbornEntity(patches int, entity sim.EntityID) bool {
	last, err := NaturalHighestFixedEntityID(patches)
	return err == nil && entity > last && entity != sim.EntityID(^uint64(0))
}

func naturalActorEntity(patches int, entity sim.EntityID) bool {
	return naturalFounderEntity(patches, entity) || naturalNewbornEntity(patches, entity)
}

func naturalSlotEntity(patches int, entity sim.EntityID) bool {
	founders, err := NaturalFounderCount(patches)
	return err == nil && entity >= NaturalSlotIDBase && entity < sim.EntityID(NaturalSlotIDBase+founders)
}

func naturalPatchEntity(patches int, entity sim.EntityID) bool {
	_, err := NaturalPatchID(patches, int(entity)-NaturalPatchIDBase)
	return err == nil
}

// NaturalHourTime is the pulse boundary of hour h=0..168; h=168 is the final
// pulse, as in v1-v4.
func NaturalHourTime(h int) (sim.SimTime, error) {
	if h < 0 || h > NaturalHorizonHours {
		return 0, ErrNaturalContract
	}
	return sim.SimTime(h) * sim.SimTime(NaturalHour), nil
}

func NaturalPhaseTime(hour int, phase int) (sim.SimTime, error) {
	if hour < 0 || hour >= NaturalHorizonHours || phase < NaturalPhasePulse || phase > NaturalPhaseLabourDecision {
		return 0, ErrNaturalContract
	}
	at, _ := NaturalHourTime(hour)
	switch phase {
	case NaturalPhaseClaim:
		return at + 1, nil
	case NaturalPhaseGathered:
		return at + sim.SimTime(NaturalGatherDuration) + 1, nil
	case NaturalPhaseBagDecision:
		return at + sim.SimTime(NaturalGatherDuration) + 2, nil
	case NaturalPhaseBuildStart:
		return at + sim.SimTime(NaturalGatherDuration) + 3, nil
	case NaturalPhasePairedMeal:
		return at + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 4, nil
	case NaturalPhaseBirthRequest:
		return at + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 5, nil
	case NaturalPhaseBirthReply:
		return at + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 6, nil
	case NaturalPhaseLabourSettle:
		// The frozen build-completion instant (h+40m+3µs): rule 518 shares
		// it with rule 505 completions so the kernel's ascending-cause-ID
		// fold arbitrates both deterministically.
		return at + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 3, nil
	case NaturalPhaseLabourOffer:
		return at + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 5, nil
	case NaturalPhaseLabourDecision:
		return at + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 6, nil
	}
	return at, nil // pulse
}

// NaturalBuildCompletion is the frozen atomic bag→wip/k conversion and slot
// claim instant of a build started at the exact NaturalPhaseBuildStart time.
func NaturalBuildCompletion(start sim.SimTime) sim.SimTime {
	return start + sim.SimTime(NaturalBuildDuration)
}

func naturalBounds(min, max float64) component.Bounds {
	return component.Bounds{HasMinimum: true, Minimum: min, HasMaximum: true, Maximum: max}
}
func naturalHourBounds() component.Bounds {
	return naturalBounds(float64(NaturalNeverHour), float64(NaturalHorizonHours-1))
}
func naturalInteger(id sim.FieldID, name, unit string, min, max float64) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.IntegerKind}, Unit: unit, Bounds: naturalBounds(min, max), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}
}
func naturalRef(id sim.FieldID, name string, states ...sim.ValueState) component.FieldDescriptor {
	if len(states) == 0 {
		states = []sim.ValueState{sim.Present, sim.Missing}
	}
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.EntityRefKind}, Unit: "entity-id", PermittedStates: states, Uncertainty: component.UncertaintyForbidden}
}

// naturalNameSuffix is the descriptor/rule name suffix of the mechanism
// axis: "v5" for the zero value, "v6" under NaturalMechanismV6.
func naturalNameSuffix(m NaturalMechanism) string {
	return "v" + strconv.FormatUint(uint64(m.worldVersion()), 10)
}

func naturalRuleFor(m NaturalMechanism, id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return component.TransitionRuleDescriptor{ID: id, Version: m.RuleVersion(), Name: "natural-" + name + "-" + naturalNameSuffix(m)}
}

// naturalHasProvisionLedger reports the mechanisms whose granary carries the
// rule-515 provisioned ledger and admits the provision contract: v6, v8 and
// v9 (v7 carries it alongside the wage ledgers).
func naturalHasProvisionLedger(m NaturalMechanism) bool {
	return m == NaturalMechanismV6 || m == NaturalMechanismV8 || naturalHasYieldAxis(m)
}

// naturalHasYieldAxis reports the mechanisms that declare the granary Yield
// parameter: v9 and every later mechanism (v10 is v9 plus founder placement).
func naturalHasYieldAxis(m NaturalMechanism) bool {
	return m == NaturalMechanismV9 || m == NaturalMechanismV10
}

// naturalRoundRobinFounders reports the mechanisms whose founding band is
// seated round-robin across the patches: v10 only.
func naturalRoundRobinFounders(m NaturalMechanism) bool {
	return m == NaturalMechanismV10
}

// naturalGranaryBoundsFor is the declared granary ledger envelope of a
// mechanism. Mechanism v9 scales the rule-508 grant by Yield, so its stock,
// cultivated-total and spoiled ledgers carry the yield-widened envelope; every
// frozen mechanism keeps exactly NaturalGranaryStockMax/NaturalSpoiledMax.
func naturalGranaryBoundsFor(m NaturalMechanism) (stock, spoiled int64) {
	if naturalHasYieldAxis(m) {
		return naturalYieldGranaryMax, naturalYieldGranaryMax
	}
	return NaturalGranaryStockMax, NaturalSpoiledMax
}

func naturalRule(id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return naturalRuleFor(NaturalMechanismV5, id, name)
}

// naturalDescriptor pins the v3/v4 identity-projection pattern: every
// integer field carries a same-ID, coefficient-1 projection with the field's
// own bounds, so engine metrics are direct bounded state, never derived
// blends. Ref fields carry no projection. Worksite, granary, genome, and
// birth-request rows are private per-actor components; slot tenure is
// privileged state with no observation constructor in this package (Q7).
func naturalDescriptorFor(m NaturalMechanism, id sim.ComponentTypeID, name string, fields []component.FieldDescriptor, rules ...component.TransitionRuleDescriptor) component.ComponentDescriptor {
	projections := make([]component.ProjectionDescriptor, 0, len(fields))
	for _, f := range fields {
		if f.Type.Kind != sim.IntegerKind {
			continue
		}
		projections = append(projections, component.ProjectionDescriptor{ID: sim.ProjectionID(f.ID), Version: m.ProjectionVersion(), Name: name + "-" + f.Name, SourceFields: []sim.FieldID{f.ID}, Coefficients: []float64{1}, Unit: f.Unit, Bounds: f.Bounds, MissingBehavior: component.PreserveSourceState})
	}
	return component.ComponentDescriptor{TypeID: id, SchemaVersion: m.SchemaVersion(), Name: name, Fields: fields, AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: rules, Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func naturalDescriptor(id sim.ComponentTypeID, name string, fields []component.FieldDescriptor, rules ...component.TransitionRuleDescriptor) component.ComponentDescriptor {
	return naturalDescriptorFor(NaturalMechanismV5, id, name, fields, rules...)
}

func NaturalPatchDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	grantedMax := NaturalGranaryStockMax
	if naturalHasYieldAxis(m) {
		grantedMax = naturalYieldGranaryMax
	}
	return naturalDescriptorFor(m, NaturalPatchTypeID, "natural-patch-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "yield", "food/hour", 0, NaturalSlotsPerPatch),
		naturalInteger(2, "pulses", "hours", 0, NaturalHorizonHours),
		naturalInteger(3, "produced", "food", 0, float64(NaturalGranaryStockMax)),
		naturalInteger(4, "granted", "food", 0, float64(grantedMax)),
		naturalInteger(5, "unrealized", "food", 0, float64(NaturalGranaryStockMax))},
		naturalRuleFor(m, NaturalRegenSplitRule, "regen-split"))
}

func NaturalPatchDescriptor() component.ComponentDescriptor {
	return NaturalPatchDescriptorFor(NaturalMechanismV5)
}

func NaturalSlotDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	// Tenure is privileged (Q7): mode 0 wild / 1 cultivated, the owner ref
	// present iff cultivated, and the redundant cultivated flag that must
	// agree with the mode (G2b consistency). Only rules 503/505/514 may
	// write tenure (G5) — plus, under the v7 mechanism only, rule 518's
	// point-completion claim when a settlement completes a capital point
	// through the frozen 505 fold (the one enumerated settlement channel;
	// ticket 20). A cultivated slot never holds standing stock.
	rules := []component.TransitionRuleDescriptor{
		naturalRuleFor(m, NaturalRegenSplitRule, "regen-split"), naturalRuleFor(m, NaturalWearRule, "wear"),
		naturalRuleFor(m, NaturalBuildRule, "build"), naturalRuleFor(m, NaturalGatherRule, "gather")}
	if m == NaturalMechanismV7 {
		rules = append(rules, naturalRuleFor(m, NaturalLabourSettlementRule, "labour-settlement"))
	}
	return naturalDescriptorFor(m, NaturalSlotTypeID, "natural-slot-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "stock", "food", 0, 1),
		naturalRef(2, "patch"),
		naturalInteger(3, "gathered", "food", 0, NaturalHorizonHours),
		naturalInteger(4, "tenure-mode", "state", 0, 1),
		naturalRef(5, "tenure-owner"),
		naturalInteger(6, "tenure-cultivated", "state", 0, 1)},
		rules...)
}

func NaturalSlotDescriptor() component.ComponentDescriptor {
	return NaturalSlotDescriptorFor(NaturalMechanismV5)
}

func NaturalBagDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	// The bag stays 0..1 and wild-only; it is never a store. The
	// source-patch field is always present: the holder's home patch, which
	// is where a newborn's placement lives (proposer's patch; founders are
	// ID-placed) and where its cultivation must live. Under the v6
	// mechanism the bag additionally admits rule 515's bag+1 write; under
	// v7 also rule 518's worker delivery (bag−1).
	rules := []component.TransitionRuleDescriptor{
		naturalRuleFor(m, NaturalGatherRule, "gather"), naturalRuleFor(m, NaturalConsumeRule, "consume"),
		naturalRuleFor(m, NaturalBuildRule, "build"), naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply")}
	if naturalHasProvisionLedger(m) {
		rules = append(rules, naturalRuleFor(m, NaturalProvisionRule, "provision"))
	}
	if m == NaturalMechanismV7 {
		rules = append(rules, naturalRuleFor(m, NaturalProvisionRule, "provision"),
			naturalRuleFor(m, NaturalLabourSettlementRule, "labour-settlement"))
	}
	return naturalDescriptorFor(m, NaturalBagTypeID, "natural-bag-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "units", "food", 0, float64(NaturalBagCapacity)), naturalRef(2, "home-patch", sim.Present)}, rules...)
}

func NaturalBagDescriptor() component.ComponentDescriptor {
	return NaturalBagDescriptorFor(NaturalMechanismV5)
}

func NaturalBodyDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	return naturalDescriptorFor(m, NaturalBodyTypeID, "natural-body-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "energy", "energy", 0, float64(NaturalEnergyCapacity)),
		naturalInteger(2, "hunger", "hours", 0, float64(NaturalHungerCapacity)),
		naturalInteger(3, "basal-spent", "energy", 0, float64(NaturalBasalSpentMax)),
		naturalInteger(4, "cap-lost", "energy", 0, NaturalHorizonHours),
		naturalInteger(5, "consumed-home-patch", "food", 0, NaturalHorizonHours),
		{ID: NaturalBodyLastGatherHourField, Name: "last-gather-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: naturalHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden},
		naturalInteger(7, "basal-debt", "energy", 0, float64(NaturalBasalDebtMax))},
		naturalRuleFor(m, NaturalBasalRule, "basal"), naturalRuleFor(m, NaturalGatherRule, "gather"),
		naturalRuleFor(m, NaturalConsumeRule, "consume"), naturalRuleFor(m, NaturalEatStoredRule, "eat-stored"),
		naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply"))
}

func NaturalBodyDescriptor() component.ComponentDescriptor {
	return NaturalBodyDescriptorFor(NaturalMechanismV5)
}

func NaturalWorksiteDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	rules := []component.TransitionRuleDescriptor{
		naturalRuleFor(m, NaturalWearRule, "wear"), naturalRuleFor(m, NaturalBuildRule, "build"),
		naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply")}
	// Under the v7 mechanism the settlement invests the employer's worksite
	// (invested-units and wip on every settlement; capital and points-created
	// only when the frozen point completion fires). LastBuildHour is never a
	// settlement write: the employer spends no build-hour (ticket 20).
	if m == NaturalMechanismV7 {
		rules = append(rules, naturalRuleFor(m, NaturalLabourSettlementRule, "labour-settlement"))
	}
	return naturalDescriptorFor(m, NaturalWorksiteTypeID, "natural-worksite-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "capital", "points", 0, float64(NaturalKMax)),
		naturalInteger(2, "wip", "units", 0, float64(NaturalWipMax)),
		naturalInteger(3, "wear-debt", "points", 0, float64(NaturalWearDebtMax)),
		naturalInteger(4, "invested-units", "units", 0, NaturalHorizonHours),
		naturalInteger(5, "points-created", "points", 0, NaturalHorizonHours),
		naturalInteger(6, "points-decayed", "points", 0, NaturalHorizonHours),
		{ID: NaturalWorksiteLastBuildHourField, Name: "last-build-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: naturalHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}},
		rules...)
}

func NaturalWorksiteDescriptor() component.ComponentDescriptor {
	return NaturalWorksiteDescriptorFor(NaturalMechanismV5)
}

func NaturalGranaryDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	// Storage is bounded by spoilage, not by a cap (Q2): stock, the
	// cultivated-received ledger, the spoil-debt carry, and the spoiled
	// counter carry horizon-envelope bounds that cannot bind behavior
	// before h=168. The v4 yield-unrealized and yield-debt fields are
	// retired: grants are never lost to fullness and never lag the budget.
	// Under the v6 mechanism the descriptor additively carries field 7
	// "provisioned" (rule 515's withdrawal ledger) and admits rule 515;
	// under the v7 mechanism it additionally carries the wage ledgers 8
	// wages-paid / 9 wages-earned and admits rule 518. The v5 zero value
	// keeps exactly the frozen six-field list.
	stockMax, spoiledMax := naturalGranaryBoundsFor(m)
	fields := []component.FieldDescriptor{
		naturalInteger(1, "stock", "food", 0, float64(stockMax)),
		naturalInteger(2, "cultivated-total", "food", 0, float64(stockMax)),
		naturalInteger(3, "stored-meals", "meals", 0, NaturalHorizonHours),
		{ID: NaturalGranaryLastStoredMealHourField, Name: "last-stored-meal-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: naturalHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden},
		naturalInteger(5, "spoil-debt", "food", 0, float64(NaturalSpoilDebtMax)),
		naturalInteger(6, "spoiled", "food", 0, float64(spoiledMax))}
	rules := []component.TransitionRuleDescriptor{
		naturalRuleFor(m, NaturalSpoilageRule, "spoilage"), naturalRuleFor(m, NaturalGrantRule, "yield-grant"),
		naturalRuleFor(m, NaturalEatStoredRule, "eat-stored"), naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply")}
	if naturalHasProvisionLedger(m) {
		fields = append(fields, naturalInteger(NaturalGranaryProvisionedField, "provisioned", "meals", 0, float64(NaturalProvisionedMax)))
		rules = append(rules, naturalRuleFor(m, NaturalProvisionRule, "provision"))
	}
	if m == NaturalMechanismV7 {
		fields = append(fields,
			naturalInteger(NaturalGranaryProvisionedField, "provisioned", "meals", 0, float64(NaturalProvisionedMax)),
			naturalInteger(NaturalGranaryWagesPaidField, "wages-paid", "food", 0, float64(NaturalWagesMax)),
			naturalInteger(NaturalGranaryWagesEarnedField, "wages-earned", "food", 0, float64(NaturalWagesMax)))
		rules = append(rules, naturalRuleFor(m, NaturalProvisionRule, "provision"),
			naturalRuleFor(m, NaturalLabourSettlementRule, "labour-settlement"))
	}
	return naturalDescriptorFor(m, NaturalGranaryTypeID, "natural-granary-"+naturalNameSuffix(m), fields, rules...)
}

func NaturalGranaryDescriptor() component.ComponentDescriptor {
	return NaturalGranaryDescriptorFor(NaturalMechanismV5)
}

func NaturalGenomeDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	// The heritable genome. Immutable post-creation except died-hour
	// (rules 504/514); rule 511 may additionally write birth-granary-paid
	// on the two paying parents, and only rule 511's allocation sets the
	// parent refs of a newborn row. Locus-y is heritable but
	// production-neutral in v5 (corrigendum: flat grants).
	return naturalDescriptorFor(m, NaturalGenomeTypeID, "natural-genome-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "locus-m", "allele", float64(NaturalGenomeLocusMin), float64(NaturalGenomeLocusMax)),
		naturalInteger(2, "locus-y", "allele", float64(NaturalGenomeLocusMin), float64(NaturalGenomeLocusMax)),
		naturalInteger(3, "locus-w", "allele", float64(NaturalGenomeLocusMin), float64(NaturalGenomeLocusMax)),
		naturalRef(4, "parent-a"),
		naturalRef(5, "parent-b"),
		naturalInteger(6, "birth-hour", "hour", 0, NaturalHorizonHours),
		naturalInteger(7, "death-hour", "hour", 0, 479),
		naturalInteger(8, "died-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours),
		naturalInteger(9, "birth-granary-paid", "food", 0, float64(NaturalBirthGranaryPaidMax))},
		naturalRuleFor(m, NaturalBasalRule, "basal"), naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply"),
		naturalRuleFor(m, NaturalAgeDeathRule, "age-death"))
}

func NaturalGenomeDescriptor() component.ComponentDescriptor {
	return NaturalGenomeDescriptorFor(NaturalMechanismV5)
}

func NaturalBirthRequestDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	// The reported-granary claim carries the granary horizon envelope: a
	// self-reported claim must be able to state any reachable true value
	// (widened under mechanism v9 exactly like the granary stock).
	stockMax, _ := naturalGranaryBoundsFor(m)
	return naturalDescriptorFor(m, NaturalBirthRequestTypeID, "natural-birthrequest-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalRef(2, "addressee"),
		naturalInteger(3, "reported-capital", "points", 0, float64(NaturalKMax)),
		naturalInteger(4, "reported-granary", "food", 0, float64(stockMax)),
		naturalInteger(5, "status", "state", 0, 4),
		naturalInteger(6, "last-request-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalInteger(7, "last-reply-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1)},
		naturalRuleFor(m, NaturalBirthRequestRule, "birth-request"), naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply"),
		naturalRuleFor(m, NaturalBirthRefuseRule, "birth-refusal"), naturalRuleFor(m, NaturalBirthExpireRule, "birth-expiry"))
}

func NaturalBirthRequestDescriptor() component.ComponentDescriptor {
	return NaturalBirthRequestDescriptorFor(NaturalMechanismV5)
}

// NaturalLabourOfferDescriptorFor builds the worker's addressed labour-offer
// row (mechanism v7 only, ticket 20): the birth-request field set minus the
// self-reported claims — hour, addressee, status, last-offer-hour,
// last-reply-hour — with terms fixed (the wage is the derived 1:1 rate; no
// claim and no price field exists). Rules: 516 files on the worker's own
// row, 517 writes the enumerated decision fields (the 511 two-party
// precedent), 519 expires and clears.
func NaturalLabourOfferDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	return naturalDescriptorFor(m, NaturalLabourOfferTypeID, "natural-labour-offer-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalInteger(1, "hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalRef(2, "addressee"),
		naturalInteger(3, "status", "state", 0, 4),
		naturalInteger(4, "last-offer-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalInteger(5, "last-reply-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1)},
		naturalRuleFor(m, NaturalLabourOfferRule, "labour-offer"), naturalRuleFor(m, NaturalLabourDecisionRule, "labour-decision"),
		naturalRuleFor(m, NaturalLabourExpireRule, "labour-expiry"),
		// Ticket-20 fix: a newborn allocation writes this row under rule 511,
		// and the allocation path requires the component descriptor to declare
		// that writer (the frozen per-row discipline), so 511 must be listed
		// here or every v7 birth fails the kernel's rule-admission check.
		naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply"))
}

// NaturalLabourBookDescriptorFor builds the employer's engagement row
// (mechanism v7 only, ticket 20): the single engagement slot (engaged worker
// ref present iff engaged) plus the one-decision-per-actor-hour bookkeeping.
// Rules: 517 writes it (accept engages, refuse books the decision hour), 519
// clears the engagement.
func NaturalLabourBookDescriptorFor(m NaturalMechanism) component.ComponentDescriptor {
	return naturalDescriptorFor(m, NaturalLabourBookTypeID, "natural-labour-book-"+naturalNameSuffix(m), []component.FieldDescriptor{
		naturalRef(1, "engaged-worker"),
		naturalInteger(2, "engaged-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalInteger(3, "last-decision-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1)},
		naturalRuleFor(m, NaturalLabourDecisionRule, "labour-decision"),
		naturalRuleFor(m, NaturalLabourExpireRule, "labour-expiry"),
		// Ticket-20 fix: the labour book row is part of the newborn
		// allocation set under rule 511 (see the offer descriptor).
		naturalRuleFor(m, NaturalBirthReplyRule, "birth-reply"))
}

// NaturalRegistryFor builds the registry of the mechanism axis: the zero
// value reproduces the frozen v5 registry byte-identically;
// NaturalMechanismV6 additively carries rule 515 on the granary and bag
// descriptors and the provisioned field on the granary; NaturalMechanismV7
// (ticket 20) additionally registers the wage-contract rows 0x80000058/
// 0x80000059, the granary wage ledgers, and rules 516-519, while
// 0x8000005A stays spare. Any other value fails closed. The registry stays
// intentionally disjoint from the v1, v2, v3, v4, and S6 registries: type
// IDs, rule IDs, and versions never collide, so no history can be replayed
// across world versions by accident.
func NaturalRegistryFor(m NaturalMechanism) (component.Registry, error) {
	if !NaturalMechanismValid(m) {
		return component.Registry{}, fmt.Errorf("%w: mechanism %d is not an admitted mechanism version", ErrNaturalContract, uint16(m))
	}
	descriptors := []component.ComponentDescriptor{NaturalPatchDescriptorFor(m), NaturalSlotDescriptorFor(m), NaturalBagDescriptorFor(m), NaturalBodyDescriptorFor(m), NaturalWorksiteDescriptorFor(m), NaturalGranaryDescriptorFor(m), NaturalGenomeDescriptorFor(m), NaturalBirthRequestDescriptorFor(m)}
	if m == NaturalMechanismV7 {
		descriptors = append(descriptors, NaturalLabourOfferDescriptorFor(m), NaturalLabourBookDescriptorFor(m))
	}
	b := component.NewRegistryBuilder()
	for _, d := range descriptors {
		if err := b.Register(d); err != nil {
			return component.Registry{}, fmt.Errorf("%w: %v", ErrNaturalContract, err)
		}
	}
	return b.Freeze()
}

// NaturalRegistry is the frozen v5 registry (the mechanism zero value).
func NaturalRegistry() (component.Registry, error) {
	return NaturalRegistryFor(NaturalMechanismV5)
}

// naturalSlotTenureField reports whether the field is a tenure write (mode,
// owner, or the cultivated flag). G5: only rules 503 (wear revert), 505
// (build claim), and 514 (frozen dead allowance) may write tenure; rule 514
// has no tenure-writing flow in v5 and is admitted verbatim from the frozen
// gate text.
func naturalSlotTenureField(field sim.FieldID) bool {
	return field == NaturalSlotTenureModeField || field == NaturalSlotTenureOwnerField || field == NaturalSlotTenureCultivatedField
}

// naturalWorldRule reports the world-caused rules (the cause must be the
// world, never an actor). Rule 519 labour-expiry/void joins under its own
// mechanism gate: the field guard refuses every 519 patch outside v7, so
// adding it here changes no v5/v6 admission.
func naturalWorldRule(rule sim.RuleID) bool {
	switch rule {
	case NaturalRegenSplitRule, NaturalSpoilageRule, NaturalWearRule, NaturalBasalRule, NaturalGrantRule, NaturalBirthExpireRule, NaturalAgeDeathRule, NaturalLabourExpireRule:
		return true
	}
	return false
}

func naturalPatchFieldAllowed(rule sim.RuleID, componentType sim.ComponentTypeID, field sim.FieldID) bool {
	switch rule {
	case NaturalGatherRule:
		switch componentType {
		case NaturalBagTypeID:
			return field == NaturalBagUnitsField
		case NaturalBodyTypeID:
			return field == NaturalBodyLastGatherHourField
		}
	case NaturalConsumeRule:
		switch componentType {
		case NaturalBagTypeID:
			return field == NaturalBagUnitsField
		case NaturalBodyTypeID:
			return field == NaturalBodyEnergyField || field == NaturalBodyHungerField || field == NaturalBodyCapLostField || field == NaturalBodyConsumedField
		}
	case NaturalBasalRule:
		switch componentType {
		case NaturalBodyTypeID:
			return field == NaturalBodyEnergyField || field == NaturalBodyHungerField || field == NaturalBodyBasalSpentField || field == NaturalBodyBasalDebtField
		case NaturalGenomeTypeID:
			return field == NaturalGenomeDiedHourField
		}
	case NaturalBuildRule:
		switch componentType {
		case NaturalBagTypeID:
			return field == NaturalBagUnitsField
		case NaturalWorksiteTypeID:
			return field == NaturalWorksiteCapitalField || field == NaturalWorksiteWipField || field == NaturalWorksiteInvestedUnitsField || field == NaturalWorksitePointsCreatedField || field == NaturalWorksiteLastBuildHourField
		}
	case NaturalSpoilageRule:
		if componentType == NaturalGranaryTypeID {
			return field == NaturalGranaryStockField || field == NaturalGranarySpoilDebtField || field == NaturalGranarySpoiledField
		}
	case NaturalGrantRule:
		if componentType == NaturalGranaryTypeID {
			return field == NaturalGranaryStockField || field == NaturalGranaryCultivatedTotalField
		}
	case NaturalWearRule:
		if componentType == NaturalWorksiteTypeID {
			return field == NaturalWorksiteCapitalField || field == NaturalWorksiteWearDebtField || field == NaturalWorksitePointsDecayedField
		}
	case NaturalEatStoredRule:
		switch componentType {
		case NaturalGranaryTypeID:
			return field == NaturalGranaryStockField || field == NaturalGranaryStoredMealsField || field == NaturalGranaryLastStoredMealHourField
		case NaturalBodyTypeID:
			return field == NaturalBodyEnergyField || field == NaturalBodyHungerField || field == NaturalBodyCapLostField
		}
	case NaturalBirthRequestRule:
		if componentType == NaturalBirthRequestTypeID {
			return field == NaturalBirthRequestHourField || field == NaturalBirthRequestAddresseeField || field == NaturalBirthRequestReportedCapitalField || field == NaturalBirthRequestReportedGranaryField || field == NaturalBirthRequestStatusField || field == NaturalBirthRequestLastRequestHourField
		}
	case NaturalBirthReplyRule:
		switch componentType {
		case NaturalBodyTypeID:
			return field == NaturalBodyEnergyField || field == NaturalBodyBasalSpentField
		case NaturalGranaryTypeID:
			return field == NaturalGranaryStockField
		case NaturalGenomeTypeID:
			return field == NaturalGenomeBirthGranaryPaidField
		case NaturalBirthRequestTypeID:
			return field == NaturalBirthRequestStatusField || field == NaturalBirthRequestLastReplyHourField
		}
	case NaturalBirthRefuseRule:
		if componentType == NaturalBirthRequestTypeID {
			return field == NaturalBirthRequestStatusField || field == NaturalBirthRequestLastReplyHourField
		}
	case NaturalBirthExpireRule:
		if componentType == NaturalBirthRequestTypeID {
			return field == NaturalBirthRequestStatusField
		}
	case NaturalAgeDeathRule:
		if componentType == NaturalGenomeTypeID {
			return field == NaturalGenomeDiedHourField
		}
	}
	return false
}

// naturalPatchFieldAllowedFor is the mechanism-parameterized field-level
// half of the G5 ownership guard. The v5 zero value is exactly the frozen
// v5 writer set. NaturalMechanismV6 additively admits rule 515's four
// (component, field) admissions — granary stock, granary provisioned,
// granary last-stored-meal-hour, bag units — and nothing else; every 515
// write under the v5 zero value is refused, so the widened writer set
// exists only under the new mechanism version. NaturalMechanismV7 admits
// everything v6 admits plus exactly the enumerated wage-contract triples
// (ticket 20): 516 files the worker's own offer row (no claim and no price
// field exists on it); 517 writes the worker's enumerated decision fields
// and the employer's own engagement row; 518 writes the enumerated
// settlement fields (worker bag/granary stock/wages-earned, employer
// granary stock/wages-paid, employer worksite invested/wip plus the
// completion's capital/points-created — role binding happens at entity
// level in NaturalCheckOwnershipFor, and last-build-hour is never a
// settlement write); 519 clears status/engagement fields only. Tenure
// fields are not actor-component fields and are handled in the slot branch
// of NaturalCheckOwnershipFor.
func naturalPatchFieldAllowedFor(m NaturalMechanism, rule sim.RuleID, componentType sim.ComponentTypeID, field sim.FieldID) bool {
	if rule == NaturalProvisionRule {
		if m != NaturalMechanismV6 && m != NaturalMechanismV7 && !naturalHasProvisionLedger(m) {
			return false
		}
		switch componentType {
		case NaturalGranaryTypeID:
			return field == NaturalGranaryStockField || field == NaturalGranaryProvisionedField || field == NaturalGranaryLastStoredMealHourField
		case NaturalBagTypeID:
			return field == NaturalBagUnitsField
		}
		return false
	}
	if m == NaturalMechanismV7 {
		switch rule {
		case NaturalLabourOfferRule:
			if componentType == NaturalLabourOfferTypeID {
				return field == NaturalLabourOfferHourField || field == NaturalLabourOfferAddresseeField ||
					field == NaturalLabourOfferStatusField || field == NaturalLabourOfferLastOfferHourField
			}
			return false
		case NaturalLabourDecisionRule:
			switch componentType {
			case NaturalLabourOfferTypeID:
				return field == NaturalLabourOfferStatusField || field == NaturalLabourOfferLastReplyHourField
			case NaturalLabourBookTypeID:
				return field == NaturalLabourBookEngagedWorkerField || field == NaturalLabourBookEngagedHourField || field == NaturalLabourBookLastDecisionHourField
			}
			return false
		case NaturalLabourSettlementRule:
			switch componentType {
			case NaturalBagTypeID:
				return field == NaturalBagUnitsField
			case NaturalGranaryTypeID:
				return field == NaturalGranaryStockField || field == NaturalGranaryWagesPaidField || field == NaturalGranaryWagesEarnedField
			case NaturalWorksiteTypeID:
				return field == NaturalWorksiteCapitalField || field == NaturalWorksiteWipField ||
					field == NaturalWorksiteInvestedUnitsField || field == NaturalWorksitePointsCreatedField
			}
			return false
		case NaturalLabourExpireRule:
			switch componentType {
			case NaturalLabourOfferTypeID:
				return field == NaturalLabourOfferStatusField
			case NaturalLabourBookTypeID:
				return field == NaturalLabourBookEngagedWorkerField || field == NaturalLabourBookEngagedHourField
			}
			return false
		}
	}
	return naturalPatchFieldAllowed(rule, componentType, field)
}

// NaturalCheckOwnership enforces frozen possession-by-identity (gate G5) at
// the trusted harness boundary, extended to tenure: world-caused rules (501
// regen-split, 502 spoilage, 503 wear, 504 basal, 508 yield-grant, 513
// expiry, 514 age-death) carry a world cause; every other patch targets the
// causal actor's own rows; slot tenure (mode/owner/cultivated) is written
// only by rules 503/505/514, slot stock only by 501/503/506, and slot
// gathered only by 506; the birth-reply (511) may patch only its enumerated
// parent-cost fields on the two parents and the birth-refusal (512) only the
// two request rows; only rule 511 allocates, and only the six per-actor v5
// components for allocator-fresh newborn IDs. The kernel validates schema
// and rule admission, not ownership; the runner and journal replay must
// apply this check to every natural proposal.
func NaturalCheckOwnership(p kernel.Proposal, patches int) error {
	return NaturalCheckOwnershipFor(NaturalMechanismV5, p, patches)
}

// NaturalCheckOwnershipFor is the mechanism-parameterized ownership guard:
// the zero value is byte-identical to NaturalCheckOwnership (rule 515 and
// its widened admissions do not exist there); NaturalMechanismV6 admits the
// provision rule 515 as an actor-caused own-rows rule over its four
// enumerated (component, field) admissions and requires the v6 rule
// version. NaturalMechanismV7 (ticket 20) admits exactly the enumerated
// wage-contract flows and nothing else:
//
//   - 516 is an own-rows rule: the worker patches only its own offer row,
//     over the four filing fields (no claim and no price field exists).
//   - 517 is the 511 two-party precedent: the addressed employer patches
//     the single worker's enumerated status fields (status,
//     last-reply-hour) plus its own engagement row, and the cause must be
//     among the patched rows; a third party's row is refused.
//   - 518 is the one sanctioned possession breach: the worker (cause) and
//     exactly one employer patch the enumerated settlement fields — bag
//     units / granary stock / wages-earned on the worker, granary stock /
//     wages-paid / worksite invested-units / wip on the employer, with
//     worksite capital and points-created admitted only together with the
//     completion's slot-tenure claim (the frozen 505 fold; the coupling is
//     exact in both directions). Tenure mode/owner otherwise remain
//     writable only by rules 503/505/514, and last-build-hour is never a
//     settlement write (the employer spends no build-hour).
//   - 519 is world-caused status clearing only (offer status, engagement
//     worker/hour) — never a resource or ledger field.
func NaturalCheckOwnershipFor(m NaturalMechanism, p kernel.Proposal, patches int) error {
	if p.RuleVersion != m.RuleVersion() {
		return ErrNaturalContract
	}
	if naturalWorldRule(p.Rule) != p.Cause.World || (p.Cause.World && p.Cause.Actor != 0) || (!p.Cause.World && p.Cause.Actor == 0) {
		return ErrNaturalContract
	}
	causeAmongPatched := false
	// The wage-contract two-party discipline (v7 only): the single
	// non-cause actor patched by rules 517/518 (the addressed worker, the
	// paying employer) and rule 518's completion signature. All three stay
	// untouched by every v5/v6 rule, so the v5/v6 behavior is unchanged.
	secondParty := sim.EntityID(0)
	slotTenurePatched, capitalPatched, createdPatched := false, false, false
	bindSecondParty := func(entity sim.EntityID) error {
		if secondParty == 0 {
			secondParty = entity
			return nil
		}
		if secondParty != entity {
			return ErrNaturalContract
		}
		return nil
	}
	for _, patch := range p.Patches {
		switch patch.Component {
		case NaturalPatchTypeID:
			if p.Rule != NaturalRegenSplitRule || !naturalPatchEntity(patches, patch.Entity) {
				return ErrNaturalContract
			}
		case NaturalSlotTypeID:
			switch p.Rule {
			case NaturalRegenSplitRule:
				if patch.Field != NaturalSlotStockField {
					return ErrNaturalContract
				}
			case NaturalWearRule:
				if patch.Field != NaturalSlotStockField && !naturalSlotTenureField(patch.Field) {
					return ErrNaturalContract
				}
			case NaturalAgeDeathRule:
				// Frozen dead allowance: tenure under 514 is admitted by
				// the gate text; no v5 flow writes it.
				if !naturalSlotTenureField(patch.Field) {
					return ErrNaturalContract
				}
			case NaturalBuildRule:
				if !naturalSlotTenureField(patch.Field) {
					return ErrNaturalContract
				}
			case NaturalGatherRule:
				if patch.Field != NaturalSlotStockField && patch.Field != NaturalSlotGatheredField {
					return ErrNaturalContract
				}
			case NaturalLabourSettlementRule:
				// The v7-only completion claim: tenure fields on a slot,
				// admitted only with the completion signature (enforced
				// after the loop) and only under the v7 mechanism.
				if m != NaturalMechanismV7 || !naturalSlotTenureField(patch.Field) {
					return ErrNaturalContract
				}
				slotTenurePatched = true
			default:
				return ErrNaturalContract
			}
			if !naturalSlotEntity(patches, patch.Entity) {
				return ErrNaturalContract
			}
		default:
			if !naturalPatchFieldAllowedFor(m, p.Rule, patch.Component, patch.Field) {
				return ErrNaturalContract
			}
			if !naturalActorEntity(patches, patch.Entity) {
				return ErrNaturalContract
			}
			switch p.Rule {
			case NaturalGatherRule, NaturalConsumeRule, NaturalBuildRule, NaturalEatStoredRule, NaturalBirthRequestRule, NaturalProvisionRule:
				// Possession-by-identity: a causal actor may only mutate
				// its own rows; there is no transfer or salvage path. Rule
				// 515 (admitted only under mechanism v6 by the field guard
				// above) is an own-rows rule like the other draw rules.
				if patch.Entity != p.Cause.Actor {
					return ErrNaturalContract
				}
			case NaturalBirthReplyRule, NaturalBirthRefuseRule:
				if patch.Entity == p.Cause.Actor {
					causeAmongPatched = true
				}
			case NaturalLabourOfferRule:
				// Own-rows filing: the worker patches only its own offer
				// row (field-level admissions already exclude the reply
				// hour and every foreign component).
				if patch.Entity != p.Cause.Actor {
					return ErrNaturalContract
				}
				causeAmongPatched = true
			case NaturalLabourDecisionRule:
				// The 511 two-party precedent: the enumerated status
				// fields on the addressed worker's offer row plus the
				// decider's own engagement row, one worker per decision.
				switch patch.Component {
				case NaturalLabourOfferTypeID:
					if patch.Entity == p.Cause.Actor {
						return ErrNaturalContract
					}
					if err := bindSecondParty(patch.Entity); err != nil {
						return err
					}
				case NaturalLabourBookTypeID:
					if patch.Entity != p.Cause.Actor {
						return ErrNaturalContract
					}
					causeAmongPatched = true
				}
			case NaturalLabourSettlementRule:
				// Role binding of the enumerated settlement set: bag units
				// and wages-earned and granary stock on the worker (the
				// cause); granary stock and wages-paid and worksite fields
				// on exactly one other actor (the employer).
				switch patch.Component {
				case NaturalBagTypeID:
					if patch.Entity != p.Cause.Actor {
						return ErrNaturalContract
					}
					causeAmongPatched = true
				case NaturalGranaryTypeID:
					if patch.Field == NaturalGranaryWagesEarnedField && patch.Entity != p.Cause.Actor {
						return ErrNaturalContract
					}
					if patch.Field == NaturalGranaryWagesPaidField && patch.Entity == p.Cause.Actor {
						return ErrNaturalContract
					}
					if patch.Entity == p.Cause.Actor {
						causeAmongPatched = true
					} else if err := bindSecondParty(patch.Entity); err != nil {
						return err
					}
				case NaturalWorksiteTypeID:
					if patch.Entity == p.Cause.Actor {
						return ErrNaturalContract
					}
					if err := bindSecondParty(patch.Entity); err != nil {
						return err
					}
					if patch.Field == NaturalWorksiteCapitalField {
						capitalPatched = true
					}
					if patch.Field == NaturalWorksitePointsCreatedField {
						createdPatched = true
					}
				}
			}
		}
	}
	if (p.Rule == NaturalBirthReplyRule || p.Rule == NaturalBirthRefuseRule || p.Rule == NaturalLabourOfferRule || p.Rule == NaturalLabourDecisionRule || p.Rule == NaturalLabourSettlementRule) && !causeAmongPatched {
		return ErrNaturalContract
	}
	// The completion coupling is exact: rule 518 carries the slot-tenure
	// claim if and only if it carries the frozen completion's capital and
	// points-created writes. Tenure outside that channel is refused.
	if p.Rule == NaturalLabourSettlementRule && slotTenurePatched != (capitalPatched && createdPatched) {
		return ErrNaturalContract
	}
	// A decision addresses exactly one worker's offer row.
	if p.Rule == NaturalLabourDecisionRule && secondParty == 0 {
		return ErrNaturalContract
	}
	for _, seed := range p.Allocations {
		if p.Rule != NaturalBirthReplyRule {
			return ErrNaturalContract
		}
		switch seed.Component {
		case NaturalBagTypeID, NaturalBodyTypeID, NaturalWorksiteTypeID, NaturalGranaryTypeID, NaturalGenomeTypeID, NaturalBirthRequestTypeID:
		case NaturalLabourOfferTypeID, NaturalLabourBookTypeID:
			// Ticket-20 fix (L4 escalation): under mechanism v7 a newborn is
			// allocated eight rows — the six v5 per-actor components plus the
			// idle labour-offer and labour-book rows that the v7 validators
			// require. The allocation whitelist must admit them under v7 only,
			// or every organic birth under v7 fails the guard and the world
			// halts fail-loud (the L1 builder already emits eight rows; only
			// the whitelist missed the two new ones).
			if m != NaturalMechanismV7 {
				return ErrNaturalContract
			}
		default:
			return ErrNaturalContract
		}
		if !naturalNewbornEntity(patches, seed.Entity) {
			return ErrNaturalContract
		}
	}
	return nil
}

// NaturalGatherProposal builds the validated gather intention at the frozen
// h+1µs claim time: slot stock/gathered plus the gatherer's own bag and body
// rows. Eligibility is delegated entirely to NaturalGather.
func NaturalGatherProposal(key string, patches, hour int, actor, slot sim.EntityID, slotBefore NaturalSlotState, a NaturalActorState) (kernel.Proposal, error) {
	return NaturalGatherProposalFor(NaturalMechanismV5, key, patches, hour, actor, slot, slotBefore, a)
}

func NaturalGatherProposalFor(m NaturalMechanism, key string, patches, hour int, actor, slot sim.EntityID, slotBefore NaturalSlotState, a NaturalActorState) (kernel.Proposal, error) {
	home := a.Bag.Source
	if _, err := NaturalPatchID(patches, int(home)-NaturalPatchIDBase); err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	at, err := NaturalPhaseTime(hour, NaturalPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	nextSlot, next, err := NaturalGatherFor(m, patches, home, slot, actor, hour, slotBefore, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalGatherRule, RuleVersion: m.RuleVersion(),
		Patches: []component.Patch{
			{Entity: slot, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotStockField, Value: sim.IntegerValue(nextSlot.Stock)},
			{Entity: slot, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotGatheredField, Value: sim.IntegerValue(nextSlot.Gathered)},
			{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyLastGatherHourField, Value: sim.IntegerValue(next.Body.LastGatherHour)},
		}}, nil
}

// NaturalConsumeProposal eats the held wild unit at the frozen h+20m+2µs bag
// decision, the Build alternative of the single bag decision point. The
// bag's home-patch source is retained, never cleared.
func NaturalConsumeProposal(key string, patches, hour int, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	return NaturalConsumeProposalFor(NaturalMechanismV5, key, patches, hour, actor, a)
}

func NaturalConsumeProposalFor(m NaturalMechanism, key string, patches, hour int, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseBagDecision)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, _, err := NaturalConsumeFor(m, patches, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalConsumeRule, RuleVersion: m.RuleVersion(),
		Patches: []component.Patch{
			{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyConsumedField, Value: sim.IntegerValue(next.Body.Consumed)},
		}}, nil
}

// NaturalEatStoredProposal is the only granary draw. at must be exactly the
// h+1µs claim or the h+40m+4µs paired meal of the given hour.
func NaturalEatStoredProposal(key string, patches, hour int, at sim.SimTime, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	return NaturalEatStoredProposalFor(NaturalMechanismV5, key, patches, hour, at, actor, a)
}

func NaturalEatStoredProposalFor(m NaturalMechanism, key string, patches, hour int, at sim.SimTime, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	claim, err := NaturalPhaseTime(hour, NaturalPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	paired, _ := NaturalPhaseTime(hour, NaturalPhasePairedMeal)
	if at != claim && at != paired {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, _, err := NaturalEatStoredFor(m, patches, hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalEatStoredRule, RuleVersion: m.RuleVersion(),
		Patches: []component.Patch{
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStockField, Value: sim.IntegerValue(next.Granary.Stock)},
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStoredMealsField, Value: sim.IntegerValue(next.Granary.StoredMeals)},
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(next.Granary.LastStoredMealHour)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
		}}, nil
}

// NaturalProvisionProposal builds the rule-515 intention at the frozen
// h+1µs claim instant, mechanism v6 only (the zero value refuses): granary
// stock/provisioned/last-stored-meal-hour plus the actor's own bag row. The
// converted unit takes the normal bag decision afterward; the actor's body
// is untouched — the meal is taken as worksite provision, not eaten.
func NaturalProvisionProposal(m NaturalMechanism, key string, patches, hour int, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	if !naturalHasProvisionLedger(m) {
		return kernel.Proposal{}, ErrNaturalContract
	}
	at, err := NaturalPhaseTime(hour, NaturalPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, err := NaturalProvision(m, patches, hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalProvisionRule, RuleVersion: m.RuleVersion(),
		Patches: []component.Patch{
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStockField, Value: sim.IntegerValue(next.Granary.Stock)},
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(next.Granary.Provisioned)},
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(next.Granary.LastStoredMealHour)},
			{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
		}}, nil
}

// NaturalBuildProposal commits the frozen atomic bag→wip/k conversion and
// the lowest-index free-wild-slot claim at the build completion instant
// h+40m+3µs; the caller's fresh snapshot is validated by NaturalBuild before
// any paired eat-stored of the same hour. A claimed slot's tenure patches
// ride on the same proposal; the slot's stock is already 0 and never written.
func NaturalBuildProposal(key string, patches, hour int, actor sim.EntityID, a NaturalActorState, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (kernel.Proposal, error) {
	return NaturalBuildProposalFor(NaturalMechanismV5, key, patches, hour, actor, a, homeSlots)
}

func NaturalBuildProposalFor(m NaturalMechanism, key string, patches, hour int, actor sim.EntityID, a NaturalActorState, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (kernel.Proposal, error) {
	start, err := NaturalPhaseTime(hour, NaturalPhaseBuildStart)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, slots, result, err := NaturalBuildFor(m, patches, hour, actor, a, a.Bag.Source, homeSlots)
	if err != nil {
		return kernel.Proposal{}, err
	}
	patchList := []component.Patch{
		{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteCapitalField, Value: sim.IntegerValue(next.Worksite.Capital)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteWipField, Value: sim.IntegerValue(next.Worksite.Wip)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteInvestedUnitsField, Value: sim.IntegerValue(next.Worksite.InvestedUnits)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksitePointsCreatedField, Value: sim.IntegerValue(next.Worksite.PointsCreated)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteLastBuildHourField, Value: sim.IntegerValue(next.Worksite.LastBuildHour)},
	}
	if result.ClaimedSlotIndex >= 0 {
		slotID, err := NaturalSlotID(patches, int(a.Bag.Source)-NaturalPatchIDBase, result.ClaimedSlotIndex)
		if err != nil {
			return kernel.Proposal{}, ErrNaturalContract
		}
		tenure := slots[result.ClaimedSlotIndex]
		ownerRef, err := sim.EntityRefValue(tenure.Owner)
		if err != nil {
			return kernel.Proposal{}, ErrNaturalContract
		}
		patchList = append(patchList,
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotTenureModeField, Value: sim.IntegerValue(int64(tenure.Mode))},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotTenureOwnerField, Value: ownerRef},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotTenureCultivatedField, Value: sim.IntegerValue(tenure.Cultivated)})
	}
	return kernel.Proposal{Key: key, Time: NaturalBuildCompletion(start), Cause: kernel.Cause{Actor: actor}, Rule: NaturalBuildRule, RuleVersion: m.RuleVersion(), Patches: patchList}, nil
}

// NaturalBirthRequestProposal files the addressed request at the frozen
// h+40m+5µs phase with self-reported capital/granary claims. Eligibility is
// delegated to NaturalBirthRequest.
func NaturalBirthRequestProposal(key string, patches, hour int, proposer, addressee sim.EntityID, reportedCapital, reportedGranary int64, a NaturalActorState) (kernel.Proposal, error) {
	return NaturalBirthRequestProposalFor(NaturalMechanismV5, key, patches, hour, proposer, addressee, reportedCapital, reportedGranary, a)
}

func NaturalBirthRequestProposalFor(m NaturalMechanism, key string, patches, hour int, proposer, addressee sim.EntityID, reportedCapital, reportedGranary int64, a NaturalActorState) (kernel.Proposal, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseBirthRequest)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, err := NaturalBirthRequestFor(m, patches, hour, proposer, addressee, reportedCapital, reportedGranary, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	addresseeRef, err := sim.EntityRefValue(addressee)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: proposer}, Rule: NaturalBirthRequestRule, RuleVersion: m.RuleVersion(),
		Patches: []component.Patch{
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestHourField, Value: sim.IntegerValue(next.Request.Hour)},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestAddresseeField, Value: addresseeRef},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestReportedCapitalField, Value: sim.IntegerValue(next.Request.ReportedCapital)},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestReportedGranaryField, Value: sim.IntegerValue(next.Request.ReportedGranary)},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(next.Request.Status))},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestLastRequestHourField, Value: sim.IntegerValue(next.Request.LastRequestHour)},
		}}, nil
}

// NaturalBirthReplyProposal builds the single atomic proposal of the frozen
// h+40m+6µs reply phase from a decided NaturalReplyResult. An accepted reply
// patches both parents' enumerated cost rows (energy and basal-spent booked
// through the body, granary stock, genome birth-granary-paid) plus the two
// request rows, and allocates the newborn's six complete rows as one
// entity-create whose ID is allocator-fresh. A refusal patches only the
// proposer's request status and the consenter's last-reply-hour. A
// zero-mutation refusal (a death between phases) has no proposal at all.
func NaturalBirthReplyProposal(key string, patches, hour int, proposer, consenter sim.EntityID, result NaturalReplyResult) (kernel.Proposal, error) {
	return NaturalBirthReplyProposalFor(NaturalMechanismV5, key, patches, hour, proposer, consenter, result)
}

func NaturalBirthReplyProposalFor(m NaturalMechanism, key string, patches, hour int, proposer, consenter sim.EntityID, result NaturalReplyResult) (kernel.Proposal, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseBirthReply)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	switch result.Decision {
	case NaturalReplyRefuseSilent:
		return kernel.Proposal{}, ErrNaturalContract
	case NaturalReplyRefuse:
		return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: consenter}, Rule: NaturalBirthRefuseRule, RuleVersion: m.RuleVersion(),
			Patches: []component.Patch{
				{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(result.Proposer.Request.Status))},
				{Entity: consenter, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestLastReplyHourField, Value: sim.IntegerValue(result.Consenter.Request.LastReplyHour)},
			}}, nil
	case NaturalReplyAccept:
		if !validNaturalActorFor(m, patches, proposer, result.Proposer) || !validNaturalActorFor(m, patches, consenter, result.Consenter) {
			return kernel.Proposal{}, ErrNaturalContract
		}
		allocations, err := naturalNewbornAllocationsFor(m, patches, result.NewbornID, result.Newborn.Bag.Source, result.Newborn)
		if err != nil {
			return kernel.Proposal{}, err
		}
		return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: consenter}, Rule: NaturalBirthReplyRule, RuleVersion: m.RuleVersion(),
			Patches: []component.Patch{
				{Entity: proposer, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyEnergyField, Value: sim.IntegerValue(result.Proposer.Body.Energy)},
				{Entity: proposer, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyBasalSpentField, Value: sim.IntegerValue(result.Proposer.Body.BasalSpent)},
				{Entity: proposer, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStockField, Value: sim.IntegerValue(result.Proposer.Granary.Stock)},
				{Entity: proposer, Component: NaturalGenomeTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(result.Proposer.Genome.BirthGranaryPaid)},
				{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(result.Proposer.Request.Status))},
				{Entity: consenter, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyEnergyField, Value: sim.IntegerValue(result.Consenter.Body.Energy)},
				{Entity: consenter, Component: NaturalBodyTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBodyBasalSpentField, Value: sim.IntegerValue(result.Consenter.Body.BasalSpent)},
				{Entity: consenter, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStockField, Value: sim.IntegerValue(result.Consenter.Granary.Stock)},
				{Entity: consenter, Component: NaturalGenomeTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(result.Consenter.Genome.BirthGranaryPaid)},
				{Entity: consenter, Component: NaturalBirthRequestTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBirthRequestLastReplyHourField, Value: sim.IntegerValue(result.Consenter.Request.LastReplyHour)},
			}, Allocations: allocations}, nil
	}
	return kernel.Proposal{}, ErrNaturalContract
}

// naturalNewbornAllocationsFor builds the newborn's six complete rows (bag,
// body, worksite, granary, genome, birthrequest) in the kernel's
// full-schema allocation shape. This is the entity-create proposal builder
// for births; the newborn ID must be allocator-fresh (G5). Under the v6
// mechanism the granary row carries the provisioned field (0 for every
// newborn); the v5 zero value keeps the frozen six-field granary list. Under
// the v7 mechanism the newborn additionally allocates its idle labour-offer
// and labour-book rows (complete rows, like every v7 registry component).
func naturalNewbornAllocationsFor(m NaturalMechanism, patches int, id, home sim.EntityID, n NaturalActorState) ([]component.ComponentSeed, error) {
	if !naturalNewbornEntity(patches, id) || n.Bag.Source != home {
		return nil, ErrNaturalContract
	}
	homeRef, err := sim.EntityRefValue(home)
	if err != nil {
		return nil, ErrNaturalContract
	}
	parentARef, err := genomeRefValue(n.Genome.ParentA)
	if err != nil {
		return nil, err
	}
	parentBRef, err := genomeRefValue(n.Genome.ParentB)
	if err != nil {
		return nil, err
	}
	addresseeRef, err := genomeRefValue(n.Request.Addressee)
	if err != nil {
		return nil, err
	}
	granary := []component.FieldSeed{
		{Field: NaturalGranaryStockField, Value: sim.IntegerValue(n.Granary.Stock)},
		{Field: NaturalGranaryCultivatedTotalField, Value: sim.IntegerValue(n.Granary.CultivatedTotal)},
		{Field: NaturalGranaryStoredMealsField, Value: sim.IntegerValue(n.Granary.StoredMeals)},
		{Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(n.Granary.LastStoredMealHour)},
		{Field: NaturalGranarySpoilDebtField, Value: sim.IntegerValue(n.Granary.SpoilDebt)},
		{Field: NaturalGranarySpoiledField, Value: sim.IntegerValue(n.Granary.Spoiled)}}
	if naturalHasProvisionLedger(m) {
		granary = append(granary, component.FieldSeed{Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(n.Granary.Provisioned)})
	}
	if m == NaturalMechanismV7 {
		granary = append(granary,
			component.FieldSeed{Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(n.Granary.Provisioned)},
			component.FieldSeed{Field: NaturalGranaryWagesPaidField, Value: sim.IntegerValue(n.Granary.WagesPaid)},
			component.FieldSeed{Field: NaturalGranaryWagesEarnedField, Value: sim.IntegerValue(n.Granary.WagesEarned)})
	}
	seeds := []component.ComponentSeed{
		{Entity: id, Component: NaturalBagTypeID, Fields: []component.FieldSeed{
			{Field: NaturalBagUnitsField, Value: sim.IntegerValue(n.Bag.Units)},
			{Field: NaturalBagSourceField, Value: homeRef}}},
		{Entity: id, Component: NaturalBodyTypeID, Fields: []component.FieldSeed{
			{Field: NaturalBodyEnergyField, Value: sim.IntegerValue(n.Body.Energy)},
			{Field: NaturalBodyHungerField, Value: sim.IntegerValue(n.Body.Hunger)},
			{Field: NaturalBodyBasalSpentField, Value: sim.IntegerValue(n.Body.BasalSpent)},
			{Field: NaturalBodyCapLostField, Value: sim.IntegerValue(n.Body.CapLost)},
			{Field: NaturalBodyConsumedField, Value: sim.IntegerValue(n.Body.Consumed)},
			{Field: NaturalBodyLastGatherHourField, Value: sim.IntegerValue(n.Body.LastGatherHour)},
			{Field: NaturalBodyBasalDebtField, Value: sim.IntegerValue(n.Body.BasalDebt)}}},
		{Entity: id, Component: NaturalWorksiteTypeID, Fields: []component.FieldSeed{
			{Field: NaturalWorksiteCapitalField, Value: sim.IntegerValue(n.Worksite.Capital)},
			{Field: NaturalWorksiteWipField, Value: sim.IntegerValue(n.Worksite.Wip)},
			{Field: NaturalWorksiteWearDebtField, Value: sim.IntegerValue(n.Worksite.WearDebt)},
			{Field: NaturalWorksiteInvestedUnitsField, Value: sim.IntegerValue(n.Worksite.InvestedUnits)},
			{Field: NaturalWorksitePointsCreatedField, Value: sim.IntegerValue(n.Worksite.PointsCreated)},
			{Field: NaturalWorksitePointsDecayedField, Value: sim.IntegerValue(n.Worksite.PointsDecayed)},
			{Field: NaturalWorksiteLastBuildHourField, Value: sim.IntegerValue(n.Worksite.LastBuildHour)}}},
		{Entity: id, Component: NaturalGranaryTypeID, Fields: granary},
		{Entity: id, Component: NaturalGenomeTypeID, Fields: []component.FieldSeed{
			{Field: NaturalGenomeLocusMField, Value: sim.IntegerValue(n.Genome.LocusM)},
			{Field: NaturalGenomeLocusYField, Value: sim.IntegerValue(n.Genome.LocusY)},
			{Field: NaturalGenomeLocusWField, Value: sim.IntegerValue(n.Genome.LocusW)},
			{Field: NaturalGenomeParentAField, Value: parentARef},
			{Field: NaturalGenomeParentBField, Value: parentBRef},
			{Field: NaturalGenomeBirthHourField, Value: sim.IntegerValue(n.Genome.BirthHour)},
			{Field: NaturalGenomeDeathHourField, Value: sim.IntegerValue(n.Genome.DeathHour)},
			{Field: NaturalGenomeDiedHourField, Value: sim.IntegerValue(n.Genome.DiedHour)},
			{Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(n.Genome.BirthGranaryPaid)}}},
		{Entity: id, Component: NaturalBirthRequestTypeID, Fields: []component.FieldSeed{
			{Field: NaturalBirthRequestHourField, Value: sim.IntegerValue(n.Request.Hour)},
			{Field: NaturalBirthRequestAddresseeField, Value: addresseeRef},
			{Field: NaturalBirthRequestReportedCapitalField, Value: sim.IntegerValue(n.Request.ReportedCapital)},
			{Field: NaturalBirthRequestReportedGranaryField, Value: sim.IntegerValue(n.Request.ReportedGranary)},
			{Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(n.Request.Status))},
			{Field: NaturalBirthRequestLastRequestHourField, Value: sim.IntegerValue(n.Request.LastRequestHour)},
			{Field: NaturalBirthRequestLastReplyHourField, Value: sim.IntegerValue(n.Request.LastReplyHour)}}},
	}
	if m == NaturalMechanismV7 {
		offereeRef, err := genomeRefValue(n.Offer.Addressee)
		if err != nil {
			return nil, err
		}
		engagedRef, err := genomeRefValue(n.Book.EngagedWorker)
		if err != nil {
			return nil, err
		}
		seeds = append(seeds,
			component.ComponentSeed{Entity: id, Component: NaturalLabourOfferTypeID, Fields: []component.FieldSeed{
				{Field: NaturalLabourOfferHourField, Value: sim.IntegerValue(n.Offer.Hour)},
				{Field: NaturalLabourOfferAddresseeField, Value: offereeRef},
				{Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(n.Offer.Status))},
				{Field: NaturalLabourOfferLastOfferHourField, Value: sim.IntegerValue(n.Offer.LastOfferHour)},
				{Field: NaturalLabourOfferLastReplyHourField, Value: sim.IntegerValue(n.Offer.LastReplyHour)}}},
			component.ComponentSeed{Entity: id, Component: NaturalLabourBookTypeID, Fields: []component.FieldSeed{
				{Field: NaturalLabourBookEngagedWorkerField, Value: engagedRef},
				{Field: NaturalLabourBookEngagedHourField, Value: sim.IntegerValue(n.Book.EngagedHour)},
				{Field: NaturalLabourBookLastDecisionHourField, Value: sim.IntegerValue(n.Book.LastDecisionHour)}}})
	}
	return seeds, nil
}

// NaturalBirthDrawPosition is the frozen birth-stream position of one
// birth's seven draws: 7×(ordinal−1), where the birth's ordinal is
// births+1 derived from committed state (births = the births before it).
// The stream itself is runner-owned; this contract only pins the position
// arithmetic. The ordinal is unbounded (no cap; P15).
func NaturalBirthDrawPosition(births int64) (uint64, error) {
	if births < 0 || births > math.MaxInt64/NaturalDrawsPerBirth {
		return 0, ErrNaturalContract
	}
	return uint64(NaturalDrawsPerBirth * births), nil
}

// NaturalLabourOfferProposal builds the rule-516 filing intention at the
// frozen h+40m+5µs phase (the birth-request instant), mechanism v7 only
// (every other mechanism refuses): the four filing fields of the worker's
// own labour-offer row. addresseeHome is the trusted runner's fresh view of
// the addressee's home patch; the same-patch filing precondition cannot be
// checked from the filing row alone. Eligibility is delegated to
// NaturalLabourOffer.
func NaturalLabourOfferProposalFor(m NaturalMechanism, key string, patches, hour int, workerID, addressee, addresseeHome sim.EntityID, worker NaturalActorState) (kernel.Proposal, error) {
	if m != NaturalMechanismV7 {
		return kernel.Proposal{}, ErrNaturalContract
	}
	at, err := NaturalPhaseTime(hour, NaturalPhaseLabourOffer)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, err := NaturalLabourOffer(m, patches, hour, workerID, addressee, addresseeHome, worker)
	if err != nil {
		return kernel.Proposal{}, err
	}
	addresseeRef, err := sim.EntityRefValue(next.Offer.Addressee)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: workerID}, Rule: NaturalLabourOfferRule, RuleVersion: m.RuleVersion(),
		Patches: []component.Patch{
			{Entity: workerID, Component: NaturalLabourOfferTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourOfferHourField, Value: sim.IntegerValue(next.Offer.Hour)},
			{Entity: workerID, Component: NaturalLabourOfferTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourOfferAddresseeField, Value: addresseeRef},
			{Entity: workerID, Component: NaturalLabourOfferTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(next.Offer.Status))},
			{Entity: workerID, Component: NaturalLabourOfferTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourOfferLastOfferHourField, Value: sim.IntegerValue(next.Offer.LastOfferHour)},
		}}, nil
}

// NaturalLabourDecisionProposal builds the single rule-517 proposal of the
// frozen h+40m+6µs decision phase (the birth-reply instant), mechanism v7
// only: an accept patches the worker's enumerated status fields plus the
// employer's own engagement row; a refusal patches the worker's status
// fields plus only the employer's last-decision-hour (status rows only, the
// 512 shape). Eligibility is delegated to NaturalLabourDecision.
func NaturalLabourDecisionProposalFor(m NaturalMechanism, key string, patches, hour int, employerID, workerID sim.EntityID, accept bool, employer, worker NaturalActorState) (kernel.Proposal, error) {
	if m != NaturalMechanismV7 {
		return kernel.Proposal{}, ErrNaturalContract
	}
	at, err := NaturalPhaseTime(hour, NaturalPhaseLabourDecision)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	nextEmployer, nextWorker, decision, err := NaturalLabourDecision(m, patches, hour, employerID, workerID, accept, employer, worker)
	if err != nil {
		return kernel.Proposal{}, err
	}
	patchesList := []component.Patch{
		{Entity: workerID, Component: NaturalLabourOfferTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(nextWorker.Offer.Status))},
		{Entity: workerID, Component: NaturalLabourOfferTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourOfferLastReplyHourField, Value: sim.IntegerValue(nextWorker.Offer.LastReplyHour)},
	}
	if decision == NaturalReplyAccept {
		workerRef, err := sim.EntityRefValue(nextEmployer.Book.EngagedWorker)
		if err != nil {
			return kernel.Proposal{}, ErrNaturalContract
		}
		patchesList = append(patchesList,
			component.Patch{Entity: employerID, Component: NaturalLabourBookTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourBookEngagedWorkerField, Value: workerRef},
			component.Patch{Entity: employerID, Component: NaturalLabourBookTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourBookEngagedHourField, Value: sim.IntegerValue(nextEmployer.Book.EngagedHour)})
	}
	patchesList = append(patchesList,
		component.Patch{Entity: employerID, Component: NaturalLabourBookTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalLabourBookLastDecisionHourField, Value: sim.IntegerValue(nextEmployer.Book.LastDecisionHour)})
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: employerID}, Rule: NaturalLabourDecisionRule, RuleVersion: m.RuleVersion(),
		Patches: patchesList}, nil
}

// NaturalLabourSettleProposal commits the rule-518 atomic wage settlement at
// the frozen h+40m+3µs completion instant (the build-completion instant, so
// the kernel's ascending-cause-ID fold arbitrates it with rule 505
// completions), mechanism v7 only. The enumerated patch set: worker bag −1,
// worker granary stock +1, worker wages-earned +1, employer granary stock
// −1, employer wages-paid +1, employer worksite invested-units +1 and wip
// +1 — and on point completion the frozen capital/points-created writes plus
// the lowest-index free-wild-slot tenure claim, exactly as the frozen 505
// completion does (the employer's LastBuildHour is never written: it spends
// no build-hour). The caller's fresh snapshot is validated by
// NaturalLabourSettle before any commit. A claimed slot's tenure patches ride
// on the same proposal.
func NaturalLabourSettleProposalFor(m NaturalMechanism, key string, patches, hour int, workerID, employerID sim.EntityID, worker, employer NaturalActorState, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (kernel.Proposal, error) {
	if m != NaturalMechanismV7 {
		return kernel.Proposal{}, ErrNaturalContract
	}
	at, err := NaturalPhaseTime(hour, NaturalPhaseLabourSettle)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	nextWorker, nextEmployer, slots, result, err := NaturalLabourSettle(m, patches, hour, workerID, employerID, worker, employer, homeSlots)
	if err != nil {
		return kernel.Proposal{}, err
	}
	patchList := []component.Patch{
		{Entity: workerID, Component: NaturalBagTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalBagUnitsField, Value: sim.IntegerValue(nextWorker.Bag.Units)},
		{Entity: workerID, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStockField, Value: sim.IntegerValue(nextWorker.Granary.Stock)},
		{Entity: workerID, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryWagesEarnedField, Value: sim.IntegerValue(nextWorker.Granary.WagesEarned)},
		{Entity: employerID, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryStockField, Value: sim.IntegerValue(nextEmployer.Granary.Stock)},
		{Entity: employerID, Component: NaturalGranaryTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalGranaryWagesPaidField, Value: sim.IntegerValue(nextEmployer.Granary.WagesPaid)},
		{Entity: employerID, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteInvestedUnitsField, Value: sim.IntegerValue(nextEmployer.Worksite.InvestedUnits)},
		{Entity: employerID, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteWipField, Value: sim.IntegerValue(nextEmployer.Worksite.Wip)},
	}
	if result.PointsCompleted == 1 {
		if result.ClaimedSlotIndex < 0 || int(worker.Bag.Source)-NaturalPatchIDBase < 0 {
			return kernel.Proposal{}, ErrNaturalContract
		}
		slotID, err := NaturalSlotID(patches, int(worker.Bag.Source)-NaturalPatchIDBase, result.ClaimedSlotIndex)
		if err != nil {
			return kernel.Proposal{}, ErrNaturalContract
		}
		tenure := slots[result.ClaimedSlotIndex]
		ownerRef, err := sim.EntityRefValue(tenure.Owner)
		if err != nil {
			return kernel.Proposal{}, ErrNaturalContract
		}
		patchList = append(patchList,
			component.Patch{Entity: employerID, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksiteCapitalField, Value: sim.IntegerValue(nextEmployer.Worksite.Capital)},
			component.Patch{Entity: employerID, Component: NaturalWorksiteTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalWorksitePointsCreatedField, Value: sim.IntegerValue(nextEmployer.Worksite.PointsCreated)},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotTenureModeField, Value: sim.IntegerValue(int64(tenure.Mode))},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotTenureOwnerField, Value: ownerRef},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: m.SchemaVersion(), Field: NaturalSlotTenureCultivatedField, Value: sim.IntegerValue(tenure.Cultivated)})
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: workerID}, Rule: NaturalLabourSettlementRule, RuleVersion: m.RuleVersion(), Patches: patchList}, nil
}

// Rule 519 (labour-expiry/void) has no proposal builder here by the frozen
// precedent: like rule 513, the world-caused pulse patches are assembled by
// the runner's naturalProposal machinery from the pure after-states returned
// by NaturalLabourOfferExpire and NaturalLabourBookClear in
// natural_contract_math.go.
