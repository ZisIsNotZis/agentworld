package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"fmt"
	"math"
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
var ErrNaturalContract = errors.New("invalid natural v5 contract")

const (
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
	NaturalConsentMinGranary  int64 = 3
	NaturalConsentMinCapital  int64 = 2
	NaturalConsentMinEnergy   int64 = 3
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
	NaturalGranaryStockMax     int64 = 8 * NaturalHorizonHours                           // 1344
	NaturalSpoilDebtMax        int64 = 8*NaturalHorizonHours + NaturalSpoilagePeriod - 1 // 1367
	NaturalSpoiledMax          int64 = 8 * NaturalHorizonHours                           // 1344
	NaturalBasalSpentMax       int64 = 6 * NaturalHorizonHours                           // 1008
	NaturalBirthGranaryPaidMax int64 = 4 * NaturalHorizonHours                           // 672
)

const (
	NaturalPatchIDBase = 1001
	NaturalSlotIDBase  = 2001

	// The frozen v5 type-ID block. The eight components below occupy
	// 0x80000050-0x80000057 contiguously; 0x80000058-0x8000005A stay inside
	// the frozen block and are never used by any other world version.
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

// Frozen pulse order inside hour h, byte-identical to v4's frozen phase
// offsets: pulse stages (basal, regen-split, spoilage, wear) at the hour
// boundary; claims at h+1µs; gather completion at h+20m+1µs; bag decision at
// h+20m+2µs; build start at h+20m+3µs completing at h+40m+3µs with the
// atomic bag→wip/k conversion and slot claim; paired EatStored at
// h+40m+4µs; birth request at h+40m+5µs; birth reply at h+40m+6µs.
const (
	NaturalPhasePulse int = iota
	NaturalPhaseClaim
	NaturalPhaseGathered
	NaturalPhaseBagDecision
	NaturalPhaseBuildStart
	NaturalPhasePairedMeal
	NaturalPhaseBirthRequest
	NaturalPhaseBirthReply
)

// Entity-ID space, fixed forever: actors 1..8P (founders) and the
// allocator-issued newborn block strictly above the highest fixed entity ID;
// patches 1001..1000+P; slots 2001..2000+8P. The blocks are disjoint by
// construction, so a newborn ID can never collide with a fixed entity and
// typed failure occurs only at MaxUint64 (P15).
func NaturalPatchCountValid(patches int) bool { return patches >= 1 }

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
	founders, err := NaturalFounderCount(patches)
	if err != nil || actor < 1 || actor > sim.EntityID(founders) {
		return 0, ErrNaturalContract
	}
	return NaturalPatchID(patches, (int(actor)-1)/NaturalSlotsPerPatch)
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
	if hour < 0 || hour >= NaturalHorizonHours || phase < NaturalPhasePulse || phase > NaturalPhaseBirthReply {
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
func naturalRule(id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return component.TransitionRuleDescriptor{ID: id, Version: NaturalRuleVersion, Name: "natural-" + name + "-v5"}
}

// naturalDescriptor pins the v3/v4 identity-projection pattern: every
// integer field carries a same-ID, coefficient-1 projection with the field's
// own bounds, so engine metrics are direct bounded state, never derived
// blends. Ref fields carry no projection. Worksite, granary, genome, and
// birth-request rows are private per-actor components; slot tenure is
// privileged state with no observation constructor in this package (Q7).
func naturalDescriptor(id sim.ComponentTypeID, name string, fields []component.FieldDescriptor, rules ...component.TransitionRuleDescriptor) component.ComponentDescriptor {
	projections := make([]component.ProjectionDescriptor, 0, len(fields))
	for _, f := range fields {
		if f.Type.Kind != sim.IntegerKind {
			continue
		}
		projections = append(projections, component.ProjectionDescriptor{ID: sim.ProjectionID(f.ID), Version: NaturalProjectionVersion, Name: name + "-" + f.Name, SourceFields: []sim.FieldID{f.ID}, Coefficients: []float64{1}, Unit: f.Unit, Bounds: f.Bounds, MissingBehavior: component.PreserveSourceState})
	}
	return component.ComponentDescriptor{TypeID: id, SchemaVersion: NaturalSchemaVersion, Name: name, Fields: fields, AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: rules, Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func NaturalPatchDescriptor() component.ComponentDescriptor {
	return naturalDescriptor(NaturalPatchTypeID, "natural-patch-v5", []component.FieldDescriptor{
		naturalInteger(1, "yield", "food/hour", 0, NaturalSlotsPerPatch),
		naturalInteger(2, "pulses", "hours", 0, NaturalHorizonHours),
		naturalInteger(3, "produced", "food", 0, float64(NaturalGranaryStockMax)),
		naturalInteger(4, "granted", "food", 0, float64(NaturalGranaryStockMax)),
		naturalInteger(5, "unrealized", "food", 0, float64(NaturalGranaryStockMax))},
		naturalRule(NaturalRegenSplitRule, "regen-split"))
}

func NaturalSlotDescriptor() component.ComponentDescriptor {
	// Tenure is privileged (Q7): mode 0 wild / 1 cultivated, the owner ref
	// present iff cultivated, and the redundant cultivated flag that must
	// agree with the mode (G2b consistency). Only rules 503/505/514 may
	// write tenure (G5); a cultivated slot never holds standing stock.
	return naturalDescriptor(NaturalSlotTypeID, "natural-slot-v5", []component.FieldDescriptor{
		naturalInteger(1, "stock", "food", 0, 1),
		naturalRef(2, "patch"),
		naturalInteger(3, "gathered", "food", 0, NaturalHorizonHours),
		naturalInteger(4, "tenure-mode", "state", 0, 1),
		naturalRef(5, "tenure-owner"),
		naturalInteger(6, "tenure-cultivated", "state", 0, 1)},
		naturalRule(NaturalRegenSplitRule, "regen-split"), naturalRule(NaturalWearRule, "wear"),
		naturalRule(NaturalBuildRule, "build"), naturalRule(NaturalGatherRule, "gather"))
}

func NaturalBagDescriptor() component.ComponentDescriptor {
	// The bag stays 0..1 and wild-only; it is never a store. The
	// source-patch field is always present: the holder's home patch, which
	// is where a newborn's placement lives (proposer's patch; founders are
	// ID-placed) and where its cultivation must live.
	return naturalDescriptor(NaturalBagTypeID, "natural-bag-v5", []component.FieldDescriptor{
		naturalInteger(1, "units", "food", 0, float64(NaturalBagCapacity)), naturalRef(2, "home-patch", sim.Present)},
		naturalRule(NaturalGatherRule, "gather"), naturalRule(NaturalConsumeRule, "consume"),
		naturalRule(NaturalBuildRule, "build"), naturalRule(NaturalBirthReplyRule, "birth-reply"))
}

func NaturalBodyDescriptor() component.ComponentDescriptor {
	return naturalDescriptor(NaturalBodyTypeID, "natural-body-v5", []component.FieldDescriptor{
		naturalInteger(1, "energy", "energy", 0, float64(NaturalEnergyCapacity)),
		naturalInteger(2, "hunger", "hours", 0, float64(NaturalHungerCapacity)),
		naturalInteger(3, "basal-spent", "energy", 0, float64(NaturalBasalSpentMax)),
		naturalInteger(4, "cap-lost", "energy", 0, NaturalHorizonHours),
		naturalInteger(5, "consumed-home-patch", "food", 0, NaturalHorizonHours),
		{ID: NaturalBodyLastGatherHourField, Name: "last-gather-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: naturalHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden},
		naturalInteger(7, "basal-debt", "energy", 0, float64(NaturalBasalDebtMax))},
		naturalRule(NaturalBasalRule, "basal"), naturalRule(NaturalGatherRule, "gather"),
		naturalRule(NaturalConsumeRule, "consume"), naturalRule(NaturalEatStoredRule, "eat-stored"),
		naturalRule(NaturalBirthReplyRule, "birth-reply"))
}

func NaturalWorksiteDescriptor() component.ComponentDescriptor {
	return naturalDescriptor(NaturalWorksiteTypeID, "natural-worksite-v5", []component.FieldDescriptor{
		naturalInteger(1, "capital", "points", 0, float64(NaturalKMax)),
		naturalInteger(2, "wip", "units", 0, float64(NaturalWipMax)),
		naturalInteger(3, "wear-debt", "points", 0, float64(NaturalWearDebtMax)),
		naturalInteger(4, "invested-units", "units", 0, NaturalHorizonHours),
		naturalInteger(5, "points-created", "points", 0, NaturalHorizonHours),
		naturalInteger(6, "points-decayed", "points", 0, NaturalHorizonHours),
		{ID: NaturalWorksiteLastBuildHourField, Name: "last-build-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: naturalHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}},
		naturalRule(NaturalWearRule, "wear"), naturalRule(NaturalBuildRule, "build"),
		naturalRule(NaturalBirthReplyRule, "birth-reply"))
}

func NaturalGranaryDescriptor() component.ComponentDescriptor {
	// Storage is bounded by spoilage, not by a cap (Q2): stock, the
	// cultivated-received ledger, the spoil-debt carry, and the spoiled
	// counter carry horizon-envelope bounds that cannot bind behavior
	// before h=168. The v4 yield-unrealized and yield-debt fields are
	// retired: grants are never lost to fullness and never lag the budget.
	return naturalDescriptor(NaturalGranaryTypeID, "natural-granary-v5", []component.FieldDescriptor{
		naturalInteger(1, "stock", "food", 0, float64(NaturalGranaryStockMax)),
		naturalInteger(2, "cultivated-total", "food", 0, float64(NaturalGranaryStockMax)),
		naturalInteger(3, "stored-meals", "meals", 0, NaturalHorizonHours),
		{ID: NaturalGranaryLastStoredMealHourField, Name: "last-stored-meal-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: naturalHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden},
		naturalInteger(5, "spoil-debt", "food", 0, float64(NaturalSpoilDebtMax)),
		naturalInteger(6, "spoiled", "food", 0, float64(NaturalSpoiledMax))},
		naturalRule(NaturalSpoilageRule, "spoilage"), naturalRule(NaturalGrantRule, "yield-grant"),
		naturalRule(NaturalEatStoredRule, "eat-stored"), naturalRule(NaturalBirthReplyRule, "birth-reply"))
}

func NaturalGenomeDescriptor() component.ComponentDescriptor {
	// The heritable genome. Immutable post-creation except died-hour
	// (rules 504/514); rule 511 may additionally write birth-granary-paid
	// on the two paying parents, and only rule 511's allocation sets the
	// parent refs of a newborn row. Locus-y is heritable but
	// production-neutral in v5 (corrigendum: flat grants).
	return naturalDescriptor(NaturalGenomeTypeID, "natural-genome-v5", []component.FieldDescriptor{
		naturalInteger(1, "locus-m", "allele", float64(NaturalGenomeLocusMin), float64(NaturalGenomeLocusMax)),
		naturalInteger(2, "locus-y", "allele", float64(NaturalGenomeLocusMin), float64(NaturalGenomeLocusMax)),
		naturalInteger(3, "locus-w", "allele", float64(NaturalGenomeLocusMin), float64(NaturalGenomeLocusMax)),
		naturalRef(4, "parent-a"),
		naturalRef(5, "parent-b"),
		naturalInteger(6, "birth-hour", "hour", 0, NaturalHorizonHours),
		naturalInteger(7, "death-hour", "hour", 0, 479),
		naturalInteger(8, "died-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours),
		naturalInteger(9, "birth-granary-paid", "food", 0, float64(NaturalBirthGranaryPaidMax))},
		naturalRule(NaturalBasalRule, "basal"), naturalRule(NaturalBirthReplyRule, "birth-reply"),
		naturalRule(NaturalAgeDeathRule, "age-death"))
}

func NaturalBirthRequestDescriptor() component.ComponentDescriptor {
	// The reported-granary claim carries the granary horizon envelope: a
	// self-reported claim must be able to state any reachable true value.
	return naturalDescriptor(NaturalBirthRequestTypeID, "natural-birthrequest-v5", []component.FieldDescriptor{
		naturalInteger(1, "hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalRef(2, "addressee"),
		naturalInteger(3, "reported-capital", "points", 0, float64(NaturalKMax)),
		naturalInteger(4, "reported-granary", "food", 0, float64(NaturalGranaryStockMax)),
		naturalInteger(5, "status", "state", 0, 4),
		naturalInteger(6, "last-request-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1),
		naturalInteger(7, "last-reply-hour", "hour", float64(NaturalNeverHour), NaturalHorizonHours-1)},
		naturalRule(NaturalBirthRequestRule, "birth-request"), naturalRule(NaturalBirthReplyRule, "birth-reply"),
		naturalRule(NaturalBirthRefuseRule, "birth-refusal"), naturalRule(NaturalBirthExpireRule, "birth-expiry"))
}

// NaturalRegistry is intentionally disjoint from the v1, v2, v3, v4, and S6
// registries: type IDs, rule IDs, and versions never collide, so no history
// can be replayed across world versions by accident.
func NaturalRegistry() (component.Registry, error) {
	b := component.NewRegistryBuilder()
	for _, d := range []component.ComponentDescriptor{NaturalPatchDescriptor(), NaturalSlotDescriptor(), NaturalBagDescriptor(), NaturalBodyDescriptor(), NaturalWorksiteDescriptor(), NaturalGranaryDescriptor(), NaturalGenomeDescriptor(), NaturalBirthRequestDescriptor()} {
		if err := b.Register(d); err != nil {
			return component.Registry{}, fmt.Errorf("%w: %v", ErrNaturalContract, err)
		}
	}
	return b.Freeze()
}

// naturalSlotTenureField reports whether the field is a tenure write (mode,
// owner, or the cultivated flag). G5: only rules 503 (wear revert), 505
// (build claim), and 514 (frozen dead allowance) may write tenure; rule 514
// has no tenure-writing flow in v5 and is admitted verbatim from the frozen
// gate text.
func naturalSlotTenureField(field sim.FieldID) bool {
	return field == NaturalSlotTenureModeField || field == NaturalSlotTenureOwnerField || field == NaturalSlotTenureCultivatedField
}

// naturalPatchFieldAllowed is the field-level half of the G5 ownership
// guard: the (rule, component, field) triples any proposal may patch for
// actor-owned components. Patch and slot ledgers are not actor-owned and are
// checked at rule+entity level (v3/v4 guard parity).
// Genome immutability is encoded here: only rules 504/514 may write
// died-hour and only rule 511 may write birth-granary-paid; loci, hours,
// and parent refs are never patchable after creation (parent refs are set
// only by rule 511's newborn allocation).
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

func naturalWorldRule(rule sim.RuleID) bool {
	switch rule {
	case NaturalRegenSplitRule, NaturalSpoilageRule, NaturalWearRule, NaturalBasalRule, NaturalGrantRule, NaturalBirthExpireRule, NaturalAgeDeathRule:
		return true
	}
	return false
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
	if p.RuleVersion != NaturalRuleVersion {
		return ErrNaturalContract
	}
	if naturalWorldRule(p.Rule) != p.Cause.World || (p.Cause.World && p.Cause.Actor != 0) || (!p.Cause.World && p.Cause.Actor == 0) {
		return ErrNaturalContract
	}
	causeAmongPatched := false
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
			default:
				return ErrNaturalContract
			}
			if !naturalSlotEntity(patches, patch.Entity) {
				return ErrNaturalContract
			}
		default:
			if !naturalPatchFieldAllowed(p.Rule, patch.Component, patch.Field) {
				return ErrNaturalContract
			}
			if !naturalActorEntity(patches, patch.Entity) {
				return ErrNaturalContract
			}
			switch p.Rule {
			case NaturalGatherRule, NaturalConsumeRule, NaturalBuildRule, NaturalEatStoredRule, NaturalBirthRequestRule:
				// Possession-by-identity: a causal actor may only mutate
				// its own rows; there is no transfer or salvage path.
				if patch.Entity != p.Cause.Actor {
					return ErrNaturalContract
				}
			case NaturalBirthReplyRule, NaturalBirthRefuseRule:
				if patch.Entity == p.Cause.Actor {
					causeAmongPatched = true
				}
			}
		}
	}
	if (p.Rule == NaturalBirthReplyRule || p.Rule == NaturalBirthRefuseRule) && !causeAmongPatched {
		return ErrNaturalContract
	}
	for _, seed := range p.Allocations {
		if p.Rule != NaturalBirthReplyRule {
			return ErrNaturalContract
		}
		switch seed.Component {
		case NaturalBagTypeID, NaturalBodyTypeID, NaturalWorksiteTypeID, NaturalGranaryTypeID, NaturalGenomeTypeID, NaturalBirthRequestTypeID:
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
	home := a.Bag.Source
	if _, err := NaturalPatchID(patches, int(home)-NaturalPatchIDBase); err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	at, err := NaturalPhaseTime(hour, NaturalPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	nextSlot, next, err := NaturalGather(patches, home, slot, actor, hour, slotBefore, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalGatherRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{
			{Entity: slot, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotStockField, Value: sim.IntegerValue(nextSlot.Stock)},
			{Entity: slot, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotGatheredField, Value: sim.IntegerValue(nextSlot.Gathered)},
			{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyLastGatherHourField, Value: sim.IntegerValue(next.Body.LastGatherHour)},
		}}, nil
}

// NaturalConsumeProposal eats the held wild unit at the frozen h+20m+2µs bag
// decision, the Build alternative of the single bag decision point. The
// bag's home-patch source is retained, never cleared.
func NaturalConsumeProposal(key string, patches, hour int, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseBagDecision)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, _, err := NaturalConsume(patches, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalConsumeRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyConsumedField, Value: sim.IntegerValue(next.Body.Consumed)},
		}}, nil
}

// NaturalEatStoredProposal is the only granary draw. at must be exactly the
// h+1µs claim or the h+40m+4µs paired meal of the given hour.
func NaturalEatStoredProposal(key string, patches, hour int, at sim.SimTime, actor sim.EntityID, a NaturalActorState) (kernel.Proposal, error) {
	claim, err := NaturalPhaseTime(hour, NaturalPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	paired, _ := NaturalPhaseTime(hour, NaturalPhasePairedMeal)
	if at != claim && at != paired {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, _, err := NaturalEatStored(patches, hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: NaturalEatStoredRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStockField, Value: sim.IntegerValue(next.Granary.Stock)},
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStoredMealsField, Value: sim.IntegerValue(next.Granary.StoredMeals)},
			{Entity: actor, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(next.Granary.LastStoredMealHour)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
		}}, nil
}

// NaturalBuildProposal commits the frozen atomic bag→wip/k conversion and
// the lowest-index free-wild-slot claim at the build completion instant
// h+40m+3µs; the caller's fresh snapshot is validated by NaturalBuild before
// any paired eat-stored of the same hour. A claimed slot's tenure patches
// ride on the same proposal; the slot's stock is already 0 and never written.
func NaturalBuildProposal(key string, patches, hour int, actor sim.EntityID, a NaturalActorState, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) (kernel.Proposal, error) {
	start, err := NaturalPhaseTime(hour, NaturalPhaseBuildStart)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, slots, result, err := NaturalBuild(patches, hour, actor, a, a.Bag.Source, homeSlots)
	if err != nil {
		return kernel.Proposal{}, err
	}
	patchList := []component.Patch{
		{Entity: actor, Component: NaturalBagTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalWorksiteCapitalField, Value: sim.IntegerValue(next.Worksite.Capital)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalWorksiteWipField, Value: sim.IntegerValue(next.Worksite.Wip)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalWorksiteInvestedUnitsField, Value: sim.IntegerValue(next.Worksite.InvestedUnits)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalWorksitePointsCreatedField, Value: sim.IntegerValue(next.Worksite.PointsCreated)},
		{Entity: actor, Component: NaturalWorksiteTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalWorksiteLastBuildHourField, Value: sim.IntegerValue(next.Worksite.LastBuildHour)},
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
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotTenureModeField, Value: sim.IntegerValue(int64(tenure.Mode))},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotTenureOwnerField, Value: ownerRef},
			component.Patch{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotTenureCultivatedField, Value: sim.IntegerValue(tenure.Cultivated)})
	}
	return kernel.Proposal{Key: key, Time: NaturalBuildCompletion(start), Cause: kernel.Cause{Actor: actor}, Rule: NaturalBuildRule, RuleVersion: NaturalRuleVersion, Patches: patchList}, nil
}

// NaturalBirthRequestProposal files the addressed request at the frozen
// h+40m+5µs phase with self-reported capital/granary claims. Eligibility is
// delegated to NaturalBirthRequest.
func NaturalBirthRequestProposal(key string, patches, hour int, proposer, addressee sim.EntityID, reportedCapital, reportedGranary int64, a NaturalActorState) (kernel.Proposal, error) {
	at, err := NaturalPhaseTime(hour, NaturalPhaseBirthRequest)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	next, err := NaturalBirthRequest(patches, hour, proposer, addressee, reportedCapital, reportedGranary, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	addresseeRef, err := sim.EntityRefValue(addressee)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: proposer}, Rule: NaturalBirthRequestRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestHourField, Value: sim.IntegerValue(next.Request.Hour)},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestAddresseeField, Value: addresseeRef},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestReportedCapitalField, Value: sim.IntegerValue(next.Request.ReportedCapital)},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestReportedGranaryField, Value: sim.IntegerValue(next.Request.ReportedGranary)},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(next.Request.Status))},
			{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestLastRequestHourField, Value: sim.IntegerValue(next.Request.LastRequestHour)},
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
	at, err := NaturalPhaseTime(hour, NaturalPhaseBirthReply)
	if err != nil {
		return kernel.Proposal{}, ErrNaturalContract
	}
	switch result.Decision {
	case NaturalReplyRefuseSilent:
		return kernel.Proposal{}, ErrNaturalContract
	case NaturalReplyRefuse:
		return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: consenter}, Rule: NaturalBirthRefuseRule, RuleVersion: NaturalRuleVersion,
			Patches: []component.Patch{
				{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(result.Proposer.Request.Status))},
				{Entity: consenter, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestLastReplyHourField, Value: sim.IntegerValue(result.Consenter.Request.LastReplyHour)},
			}}, nil
	case NaturalReplyAccept:
		if !validNaturalActor(patches, proposer, result.Proposer) || !validNaturalActor(patches, consenter, result.Consenter) {
			return kernel.Proposal{}, ErrNaturalContract
		}
		allocations, err := naturalNewbornAllocations(patches, result.NewbornID, result.Newborn.Bag.Source, result.Newborn)
		if err != nil {
			return kernel.Proposal{}, err
		}
		return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: consenter}, Rule: NaturalBirthReplyRule, RuleVersion: NaturalRuleVersion,
			Patches: []component.Patch{
				{Entity: proposer, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyEnergyField, Value: sim.IntegerValue(result.Proposer.Body.Energy)},
				{Entity: proposer, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyBasalSpentField, Value: sim.IntegerValue(result.Proposer.Body.BasalSpent)},
				{Entity: proposer, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStockField, Value: sim.IntegerValue(result.Proposer.Granary.Stock)},
				{Entity: proposer, Component: NaturalGenomeTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(result.Proposer.Genome.BirthGranaryPaid)},
				{Entity: proposer, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(result.Proposer.Request.Status))},
				{Entity: consenter, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyEnergyField, Value: sim.IntegerValue(result.Consenter.Body.Energy)},
				{Entity: consenter, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyBasalSpentField, Value: sim.IntegerValue(result.Consenter.Body.BasalSpent)},
				{Entity: consenter, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStockField, Value: sim.IntegerValue(result.Consenter.Granary.Stock)},
				{Entity: consenter, Component: NaturalGenomeTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(result.Consenter.Genome.BirthGranaryPaid)},
				{Entity: consenter, Component: NaturalBirthRequestTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBirthRequestLastReplyHourField, Value: sim.IntegerValue(result.Consenter.Request.LastReplyHour)},
			}, Allocations: allocations}, nil
	}
	return kernel.Proposal{}, ErrNaturalContract
}

// naturalNewbornAllocations builds the newborn's six complete rows (bag,
// body, worksite, granary, genome, birthrequest) in the kernel's
// full-schema allocation shape. This is the entity-create proposal builder
// for births; the newborn ID must be allocator-fresh (G5).
func naturalNewbornAllocations(patches int, id, home sim.EntityID, n NaturalActorState) ([]component.ComponentSeed, error) {
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
	return []component.ComponentSeed{
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
		{Entity: id, Component: NaturalGranaryTypeID, Fields: []component.FieldSeed{
			{Field: NaturalGranaryStockField, Value: sim.IntegerValue(n.Granary.Stock)},
			{Field: NaturalGranaryCultivatedTotalField, Value: sim.IntegerValue(n.Granary.CultivatedTotal)},
			{Field: NaturalGranaryStoredMealsField, Value: sim.IntegerValue(n.Granary.StoredMeals)},
			{Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(n.Granary.LastStoredMealHour)},
			{Field: NaturalGranarySpoilDebtField, Value: sim.IntegerValue(n.Granary.SpoilDebt)},
			{Field: NaturalGranarySpoiledField, Value: sim.IntegerValue(n.Granary.Spoiled)}}},
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
	}, nil
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
