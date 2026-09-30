package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"fmt"
)

// Reproduction v4 is the frozen reproduction-and-genetics pilot (ticket 14).
// Its component, rule, and projection identities are disjoint from v1
// (food-flow), v2 (social-food), v3 (capacity), and the S6 world: type IDs
// 0x80000040-47, rules 401-413, and version 4 never collide with any earlier
// world, so no history can be replayed across world versions by accident.
// The frozen design section of .scratch/14-reproduction-genetics/spec.md is
// binding; every constant below is quoted from it, not derived. The v3
// arithmetic this world inherits (produce, gather, consume, build,
// eat-stored, and the basal/yield/wear divisor loops) stays byte-identical
// at the neutral locus 6.
var ErrReproductionContract = errors.New("invalid reproduction v4 contract")

const (
	ReproductionFormatVersion     uint32                = 4
	ReproductionSchemaVersion     sim.SchemaVersion     = 4
	ReproductionRuleVersion       uint32                = 4
	ReproductionProjectionVersion sim.ProjectionVersion = 4

	ReproductionFounderCount   = 16
	ReproductionMaxBirths      = 16
	ReproductionFirstNewbornID = 10001
	ReproductionPatchCount     = 2
	ReproductionSlotsPerPatch  = 8
	ReproductionHorizonHours   = 168

	// Frozen v3 clock and body constants, inherited verbatim.
	ReproductionHour           sim.Duration = 3_600_000_000
	ReproductionGatherDuration sim.Duration = 20 * 60 * 1_000_000
	ReproductionMealDuration   sim.Duration = 10 * 60 * 1_000_000
	ReproductionBuildDuration  sim.Duration = 20 * 60 * 1_000_000

	ReproductionFounderInitialEnergy int64 = 11 // v3 CapacityInitialEnergy
	ReproductionNewbornEnergy        int64 = 8
	ReproductionEnergyCapacity       int64 = 12
	ReproductionHungerCapacity       int64 = 24
	ReproductionBagCapacity          int64 = 1
	ReproductionNeverHour            int64 = -1

	// Frozen v3 capacity constants inherited by the unchanged build/yield
	// economics. ReproductionWearPeriod is the neutral locus value: the v4
	// wear loop drains by the genome's locus-w instead.
	ReproductionPointCostWip  int64 = 2
	ReproductionYieldPerPoint int64 = 1
	ReproductionKMax          int64 = 8
	ReproductionWipMax        int64 = 1
	ReproductionGranaryMax    int64 = 8
	ReproductionWearPeriod    int64 = 6
	ReproductionPolicyTLow    int64 = 5

	// Frozen genome constants: three loci with alleles in [5,7], neutral 6,
	// and the shared numerator 6 of the basal (debt+=6) and yield (debt+=6k)
	// accruals. Debt bounds are the frozen transients of the divisor loops.
	ReproductionGenomeLocusMin    int64 = 5
	ReproductionGenomeLocusMax    int64 = 7
	ReproductionGenomeNeutral     int64 = 6
	ReproductionGenomeDenominator int64 = 6
	ReproductionBasalDebtMax      int64 = 13
	ReproductionYieldDebtMax      int64 = 55
	ReproductionWearDebtMax       int64 = 14

	// Frozen birth economics: each parent pays two granary units and two
	// energy units, the energy booked through the parent's basal-spent
	// ledger; the eight converted units become the newborn's energy.
	ReproductionParentGranaryCost   int64 = 2
	ReproductionParentEnergyCost    int64 = 2
	ReproductionBirthGranaryPaidMax int64 = 32 // 2 per birth × MaxBirths

	// Frozen consent thresholds (consenter's own state and the requester's
	// reported claims) and the derived-kinship limits.
	ReproductionConsentMinGranary  int64 = 3
	ReproductionConsentMinCapital  int64 = 2
	ReproductionConsentMinEnergy   int64 = 3
	ReproductionKinProhibitedDepth int   = 2
	ReproductionKinWalkLimit       int   = 10

	// Exactly seven draws per birth from the birth stream at position
	// 7×(ordinal−1): three crossover, three mutation, one lifetime
	// (96 + draw mod 48 ∈ [96,143]). Founder lifetimes come from the
	// founder stream positions 0-15; the draws themselves are runner-owned.
	ReproductionBirthStream        sim.StreamID = 0x47454E33
	ReproductionFounderStream      sim.StreamID = 0x464F554E
	ReproductionDrawsPerBirth                   = 7
	ReproductionLifetimeBase                    = 96
	ReproductionLifetimeSpan                    = 48
	ReproductionFounderLifetimeMin              = 96
	ReproductionFounderLifetimeMax              = 143

	// Widened v3-inherited ledger bounds, each the exact reachable maximum
	// under locus modulation: basal-spent drains at most two units per hour
	// (locus-m 5) plus two per birth as a parent; granary yield grants at
	// most ten units per pulse (locus-y 5, capital 8, carry 6).
	ReproductionBasalSpentMax   int64 = 2*168 + 2*16
	ReproductionGranaryYieldMax int64 = 1680
)

const (
	ReproductionPatchTypeID        sim.ComponentTypeID = 0x80000040
	ReproductionSlotTypeID         sim.ComponentTypeID = 0x80000041
	ReproductionBagTypeID          sim.ComponentTypeID = 0x80000042
	ReproductionBodyTypeID         sim.ComponentTypeID = 0x80000043
	ReproductionWorksiteTypeID     sim.ComponentTypeID = 0x80000044
	ReproductionGranaryTypeID      sim.ComponentTypeID = 0x80000045
	ReproductionGenomeTypeID       sim.ComponentTypeID = 0x80000046
	ReproductionBirthRequestTypeID sim.ComponentTypeID = 0x80000047

	ReproductionProduceRule      sim.RuleID = 401 // produce-wild
	ReproductionGatherRule       sim.RuleID = 402 // gather
	ReproductionConsumeRule      sim.RuleID = 403 // consume
	ReproductionBasalRule        sim.RuleID = 404 // basal (may write genome died-hour on starvation)
	ReproductionBuildRule        sim.RuleID = 405 // build
	ReproductionYieldRule        sim.RuleID = 406 // yield
	ReproductionWearRule         sim.RuleID = 407 // wear
	ReproductionEatStoredRule    sim.RuleID = 408 // eat-stored
	ReproductionBirthRequestRule sim.RuleID = 409 // birth-request
	ReproductionBirthReplyRule   sim.RuleID = 410 // birth-reply (allocates the newborn)
	ReproductionBirthRefuseRule  sim.RuleID = 411 // birth-refusal (request status only)
	ReproductionBirthExpireRule  sim.RuleID = 412 // birth-expiry (world-caused, pulse)
	ReproductionAgeDeathRule     sim.RuleID = 413 // age-death (world-caused, pulse)
)

const (
	ReproductionPatchYieldField sim.FieldID = 1 + iota
	ReproductionPatchPulsesField
	ReproductionPatchProducedField
	ReproductionPatchUnrealizedField
)
const (
	ReproductionSlotStockField sim.FieldID = 1 + iota
	ReproductionSlotPatchField
	ReproductionSlotGatheredField
)
const (
	ReproductionBagUnitsField sim.FieldID = 1 + iota
	ReproductionBagSourceField
)
const (
	ReproductionBodyEnergyField sim.FieldID = 1 + iota
	ReproductionBodyHungerField
	ReproductionBodyBasalSpentField
	ReproductionBodyCapLostField
	ReproductionBodyConsumedField
	ReproductionBodyLastGatherHourField
	ReproductionBodyBasalDebtField
)
const (
	ReproductionWorksiteCapitalField sim.FieldID = 1 + iota
	ReproductionWorksiteWipField
	ReproductionWorksiteWearDebtField
	ReproductionWorksiteInvestedUnitsField
	ReproductionWorksitePointsCreatedField
	ReproductionWorksitePointsDecayedField
	ReproductionWorksiteLastBuildHourField
)
const (
	ReproductionGranaryStockField sim.FieldID = 1 + iota
	ReproductionGranaryYieldTotalField
	ReproductionGranaryYieldUnrealizedField
	ReproductionGranaryStoredMealsField
	ReproductionGranaryLastStoredMealHourField
	ReproductionGranaryYieldDebtField
)
const (
	ReproductionGenomeLocusMField sim.FieldID = 1 + iota
	ReproductionGenomeLocusYField
	ReproductionGenomeLocusWField
	ReproductionGenomeParentAField
	ReproductionGenomeParentBField
	ReproductionGenomeBirthHourField
	ReproductionGenomeDeathHourField
	ReproductionGenomeDiedHourField
	ReproductionGenomeBirthGranaryPaidField
)
const (
	ReproductionBirthRequestHourField sim.FieldID = 1 + iota
	ReproductionBirthRequestAddresseeField
	ReproductionBirthRequestReportedCapitalField
	ReproductionBirthRequestReportedGranaryField
	ReproductionBirthRequestStatusField
	ReproductionBirthRequestLastRequestHourField
	ReproductionBirthRequestLastReplyHourField
)

// Frozen pulse order inside hour h, extending the v3 phases 0-5 untouched:
// PhaseBirthRequest at h+40m+5µs and PhaseBirthReply at h+40m+6µs.
const (
	ReproductionPhasePulse int = iota
	ReproductionPhaseClaim
	ReproductionPhaseGathered
	ReproductionPhaseBagDecision
	ReproductionPhaseBuildStart
	ReproductionPhasePairedMeal
	ReproductionPhaseBirthRequest
	ReproductionPhaseBirthReply
)

// Actor, patch, and slot entity ranges are fixed and disjoint from each
// other and from every earlier world. Founders are 1-16; newborns are
// 10001-10016 (next = 10000+births+1, derived by genome scan). A newborn's
// home patch is the proposer's, carried by its bag source-patch row rather
// than derived from its ID.
func ReproductionPatchID(index int) (sim.EntityID, error) {
	if index < 0 || index >= ReproductionPatchCount {
		return 0, ErrReproductionContract
	}
	return sim.EntityID(1001 + index), nil
}

func ReproductionSlotID(patchIndex, slotIndex int) (sim.EntityID, error) {
	if patchIndex < 0 || patchIndex >= ReproductionPatchCount || slotIndex < 0 || slotIndex >= ReproductionSlotsPerPatch {
		return 0, ErrReproductionContract
	}
	return sim.EntityID(2001 + patchIndex*ReproductionSlotsPerPatch + slotIndex), nil
}

// ReproductionFounderPatchID maps only the ID-placed founders; newborn home
// patches live in their bag source-patch field.
func ReproductionFounderPatchID(actor sim.EntityID) (sim.EntityID, error) {
	if actor < 1 || actor > ReproductionFounderCount {
		return 0, ErrReproductionContract
	}
	return ReproductionPatchID((int(actor) - 1) / ReproductionSlotsPerPatch)
}

// ReproductionNewbornID is the frozen next-ID rule: 10000+births+1.
func ReproductionNewbornID(births int64) (sim.EntityID, error) {
	if births < 0 || births >= ReproductionMaxBirths {
		return 0, ErrReproductionContract
	}
	return sim.EntityID(ReproductionFirstNewbornID + births), nil
}

func reproductionActorEntity(entity sim.EntityID) bool {
	return (entity >= 1 && entity <= ReproductionFounderCount) ||
		(entity >= ReproductionFirstNewbornID && entity < sim.EntityID(ReproductionFirstNewbornID+ReproductionMaxBirths))
}

func reproductionFounderEntity(entity sim.EntityID) bool {
	return entity >= 1 && entity <= ReproductionFounderCount
}

func reproductionSlotEntity(entity sim.EntityID) bool {
	return entity >= 2001 && entity < sim.EntityID(2001+ReproductionPatchCount*ReproductionSlotsPerPatch)
}

// ReproductionHourTime is the pulse boundary of hour h=0..168; h=168 is the
// final pulse, as in v3.
func ReproductionHourTime(h int) (sim.SimTime, error) {
	if h < 0 || h > ReproductionHorizonHours {
		return 0, ErrReproductionContract
	}
	return sim.SimTime(h) * sim.SimTime(ReproductionHour), nil
}

func ReproductionPhaseTime(hour int, phase int) (sim.SimTime, error) {
	if hour < 0 || hour >= ReproductionHorizonHours || phase < ReproductionPhasePulse || phase > ReproductionPhaseBirthReply {
		return 0, ErrReproductionContract
	}
	at, _ := ReproductionHourTime(hour)
	switch phase {
	case ReproductionPhaseClaim:
		return at + 1, nil
	case ReproductionPhaseGathered:
		return at + sim.SimTime(ReproductionGatherDuration) + 1, nil
	case ReproductionPhaseBagDecision:
		return at + sim.SimTime(ReproductionGatherDuration) + 2, nil
	case ReproductionPhaseBuildStart:
		return at + sim.SimTime(ReproductionGatherDuration) + 3, nil
	case ReproductionPhasePairedMeal:
		return at + sim.SimTime(ReproductionGatherDuration+ReproductionBuildDuration) + 4, nil
	case ReproductionPhaseBirthRequest:
		return at + sim.SimTime(ReproductionGatherDuration+ReproductionBuildDuration) + 5, nil
	case ReproductionPhaseBirthReply:
		return at + sim.SimTime(ReproductionGatherDuration+ReproductionBuildDuration) + 6, nil
	}
	return at, nil // pulse
}

// ReproductionBuildCompletion is the frozen atomic bag→wip/k conversion
// instant of a build started at the exact ReproductionPhaseBuildStart time.
func ReproductionBuildCompletion(start sim.SimTime) sim.SimTime {
	return start + sim.SimTime(ReproductionBuildDuration)
}

func reproductionBounds(min, max float64) component.Bounds {
	return component.Bounds{HasMinimum: true, Minimum: min, HasMaximum: true, Maximum: max}
}
func reproductionHourBounds() component.Bounds {
	return reproductionBounds(float64(ReproductionNeverHour), float64(ReproductionHorizonHours-1))
}
func reproductionInteger(id sim.FieldID, name, unit string, min, max float64) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.IntegerKind}, Unit: unit, Bounds: reproductionBounds(min, max), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}
}
func reproductionRef(id sim.FieldID, name string, states ...sim.ValueState) component.FieldDescriptor {
	if len(states) == 0 {
		states = []sim.ValueState{sim.Present, sim.Missing}
	}
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.EntityRefKind}, Unit: "entity-id", PermittedStates: states, Uncertainty: component.UncertaintyForbidden}
}
func reproductionRule(id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return component.TransitionRuleDescriptor{ID: id, Version: ReproductionRuleVersion, Name: "reproduction-" + name + "-v4"}
}

// reproductionDescriptor pins the v3 identity-projection pattern: every
// integer field carries a same-ID, coefficient-1 projection with the field's
// own bounds, so engine metrics are direct bounded state, never derived
// blends. Ref fields carry no projection.
func reproductionDescriptor(id sim.ComponentTypeID, name string, fields []component.FieldDescriptor, rules ...component.TransitionRuleDescriptor) component.ComponentDescriptor {
	projections := make([]component.ProjectionDescriptor, 0, len(fields))
	for _, f := range fields {
		if f.Type.Kind != sim.IntegerKind {
			continue
		}
		projections = append(projections, component.ProjectionDescriptor{ID: sim.ProjectionID(f.ID), Version: ReproductionProjectionVersion, Name: name + "-" + f.Name, SourceFields: []sim.FieldID{f.ID}, Coefficients: []float64{1}, Unit: f.Unit, Bounds: f.Bounds, MissingBehavior: component.PreserveSourceState})
	}
	return component.ComponentDescriptor{TypeID: id, SchemaVersion: ReproductionSchemaVersion, Name: name, Fields: fields, AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: rules, Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func ReproductionPatchDescriptor() component.ComponentDescriptor {
	return reproductionDescriptor(ReproductionPatchTypeID, "reproduction-patch-v4", []component.FieldDescriptor{
		reproductionInteger(1, "yield", "food/hour", 0, ReproductionSlotsPerPatch), reproductionInteger(2, "pulses", "hours", 0, ReproductionHorizonHours), reproductionInteger(3, "produced", "food", 0, ReproductionHorizonHours*ReproductionSlotsPerPatch), reproductionInteger(4, "unrealized", "food", 0, ReproductionHorizonHours*ReproductionSlotsPerPatch)}, reproductionRule(ReproductionProduceRule, "produce-wild"))
}

func ReproductionSlotDescriptor() component.ComponentDescriptor {
	return reproductionDescriptor(ReproductionSlotTypeID, "reproduction-slot-v4", []component.FieldDescriptor{
		reproductionInteger(1, "stock", "food", 0, 1), reproductionRef(2, "patch"), reproductionInteger(3, "gathered", "food", 0, ReproductionHorizonHours)}, reproductionRule(ReproductionProduceRule, "produce-wild"), reproductionRule(ReproductionGatherRule, "gather"))
}

func ReproductionBagDescriptor() component.ComponentDescriptor {
	// Unlike v3, the source-patch field is always present: it is the
	// holder's home patch, which is where a newborn's placement lives
	// (proposer's patch; founders are ID-placed). Held provenance is
	// unchanged — a bag unit is always gathered at home.
	return reproductionDescriptor(ReproductionBagTypeID, "reproduction-bag-v4", []component.FieldDescriptor{
		reproductionInteger(1, "units", "food", 0, float64(ReproductionBagCapacity)), reproductionRef(2, "home-patch", sim.Present)}, reproductionRule(ReproductionGatherRule, "gather"), reproductionRule(ReproductionConsumeRule, "consume"), reproductionRule(ReproductionBuildRule, "build"), reproductionRule(ReproductionBirthReplyRule, "birth-reply"))
}

func ReproductionBodyDescriptor() component.ComponentDescriptor {
	return reproductionDescriptor(ReproductionBodyTypeID, "reproduction-body-v4", []component.FieldDescriptor{
		reproductionInteger(1, "energy", "energy", 0, float64(ReproductionEnergyCapacity)), reproductionInteger(2, "hunger", "hours", 0, float64(ReproductionHungerCapacity)), reproductionInteger(3, "basal-spent", "energy", 0, float64(ReproductionBasalSpentMax)), reproductionInteger(4, "cap-lost", "energy", 0, ReproductionHorizonHours), reproductionInteger(5, "consumed-home-patch", "food", 0, ReproductionHorizonHours), {ID: ReproductionBodyLastGatherHourField, Name: "last-gather-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: reproductionHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}, reproductionInteger(7, "basal-debt", "energy", 0, float64(ReproductionBasalDebtMax))}, reproductionRule(ReproductionGatherRule, "gather"), reproductionRule(ReproductionConsumeRule, "consume"), reproductionRule(ReproductionBasalRule, "basal"), reproductionRule(ReproductionEatStoredRule, "eat-stored"), reproductionRule(ReproductionBirthReplyRule, "birth-reply"))
}

func ReproductionWorksiteDescriptor() component.ComponentDescriptor {
	// The v3 worksite with one widened bound: wear-debt [0,14] — the v4
	// carry loop drains by locus-w ∈ [5,7], so the transient reaches
	// (w−1)+capital ≤ 6+8 = 14.
	return reproductionDescriptor(ReproductionWorksiteTypeID, "reproduction-worksite-v4", []component.FieldDescriptor{
		reproductionInteger(1, "capital", "points", 0, float64(ReproductionKMax)),
		reproductionInteger(2, "wip", "units", 0, float64(ReproductionWipMax)),
		reproductionInteger(3, "wear-debt", "points", 0, float64(ReproductionWearDebtMax)),
		reproductionInteger(4, "invested-units", "units", 0, ReproductionHorizonHours),
		reproductionInteger(5, "points-created", "points", 0, ReproductionHorizonHours),
		reproductionInteger(6, "points-decayed", "points", 0, ReproductionHorizonHours),
		{ID: ReproductionWorksiteLastBuildHourField, Name: "last-build-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: reproductionHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}}, reproductionRule(ReproductionBuildRule, "build"), reproductionRule(ReproductionWearRule, "wear"), reproductionRule(ReproductionBirthReplyRule, "birth-reply"))
}

func ReproductionGranaryDescriptor() component.ComponentDescriptor {
	// Yield ledgers widen to [0,1680]: the carry loop grants at most ten
	// units per pulse (locus-y 5, capital 8) against v3's eight. Yield-debt
	// [0,55] is the frozen loop transient.
	return reproductionDescriptor(ReproductionGranaryTypeID, "reproduction-granary-v4", []component.FieldDescriptor{
		reproductionInteger(1, "stock", "food", 0, float64(ReproductionGranaryMax)),
		reproductionInteger(2, "yield-total", "food", 0, float64(ReproductionGranaryYieldMax)),
		reproductionInteger(3, "yield-unrealized", "food", 0, float64(ReproductionGranaryYieldMax)),
		reproductionInteger(4, "stored-meals", "meals", 0, ReproductionHorizonHours),
		{ID: ReproductionGranaryLastStoredMealHourField, Name: "last-stored-meal-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: reproductionHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden},
		reproductionInteger(6, "yield-debt", "food", 0, float64(ReproductionYieldDebtMax))}, reproductionRule(ReproductionYieldRule, "yield"), reproductionRule(ReproductionEatStoredRule, "eat-stored"), reproductionRule(ReproductionBirthReplyRule, "birth-reply"))
}

func ReproductionGenomeDescriptor() component.ComponentDescriptor {
	// The heritable genome. Immutable post-creation except died-hour
	// (rules 404/413); rule 410 may additionally write birth-granary-paid
	// on the two paying parents, and only rule 410's allocation sets the
	// parent refs of a newborn row.
	return reproductionDescriptor(ReproductionGenomeTypeID, "reproduction-genome-v4", []component.FieldDescriptor{
		reproductionInteger(1, "locus-m", "allele", float64(ReproductionGenomeLocusMin), float64(ReproductionGenomeLocusMax)),
		reproductionInteger(2, "locus-y", "allele", float64(ReproductionGenomeLocusMin), float64(ReproductionGenomeLocusMax)),
		reproductionInteger(3, "locus-w", "allele", float64(ReproductionGenomeLocusMin), float64(ReproductionGenomeLocusMax)),
		reproductionRef(4, "parent-a"),
		reproductionRef(5, "parent-b"),
		reproductionInteger(6, "birth-hour", "hour", 0, ReproductionHorizonHours),
		reproductionInteger(7, "death-hour", "hour", 0, 479),
		reproductionInteger(8, "died-hour", "hour", float64(ReproductionNeverHour), ReproductionHorizonHours),
		reproductionInteger(9, "birth-granary-paid", "food", 0, float64(ReproductionBirthGranaryPaidMax))}, reproductionRule(ReproductionBasalRule, "basal"), reproductionRule(ReproductionBirthReplyRule, "birth-reply"), reproductionRule(ReproductionAgeDeathRule, "age-death"))
}

func ReproductionBirthRequestDescriptor() component.ComponentDescriptor {
	return reproductionDescriptor(ReproductionBirthRequestTypeID, "reproduction-birthrequest-v4", []component.FieldDescriptor{
		reproductionInteger(1, "hour", "hour", float64(ReproductionNeverHour), ReproductionHorizonHours-1),
		reproductionRef(2, "addressee"),
		reproductionInteger(3, "reported-capital", "points", 0, float64(ReproductionKMax)),
		reproductionInteger(4, "reported-granary", "food", 0, float64(ReproductionGranaryMax)),
		reproductionInteger(5, "status", "state", 0, 4),
		reproductionInteger(6, "last-request-hour", "hour", float64(ReproductionNeverHour), ReproductionHorizonHours-1),
		reproductionInteger(7, "last-reply-hour", "hour", float64(ReproductionNeverHour), ReproductionHorizonHours-1)}, reproductionRule(ReproductionBirthRequestRule, "birth-request"), reproductionRule(ReproductionBirthReplyRule, "birth-reply"), reproductionRule(ReproductionBirthRefuseRule, "birth-refusal"), reproductionRule(ReproductionBirthExpireRule, "birth-expiry"))
}

// ReproductionRegistry is intentionally disjoint from the v1, v2, v3, and S6
// registries: type IDs, rule IDs, and versions never collide, so no history
// can be replayed across world versions by accident.
func ReproductionRegistry() (component.Registry, error) {
	b := component.NewRegistryBuilder()
	for _, d := range []component.ComponentDescriptor{ReproductionPatchDescriptor(), ReproductionSlotDescriptor(), ReproductionBagDescriptor(), ReproductionBodyDescriptor(), ReproductionWorksiteDescriptor(), ReproductionGranaryDescriptor(), ReproductionGenomeDescriptor(), ReproductionBirthRequestDescriptor()} {
		if err := b.Register(d); err != nil {
			return component.Registry{}, fmt.Errorf("%w: %v", ErrReproductionContract, err)
		}
	}
	return b.Freeze()
}

// reproductionPatchFieldAllowed is the field-level half of the G5 ownership
// guard: the (rule, component, field) triples any proposal may patch for
// actor-owned components. Patch and slot ledgers are not actor-owned and are
// checked at rule+entity level instead (v3 guard parity).
// Genome immutability is encoded here: only rules 404/413 may write
// died-hour and only rule 410 may write birth-granary-paid; loci, hours,
// and parent refs are never patchable after creation (parent refs are set
// only by rule 410's newborn allocation).
func reproductionPatchFieldAllowed(rule sim.RuleID, componentType sim.ComponentTypeID, field sim.FieldID) bool {
	switch rule {
	case ReproductionGatherRule:
		switch componentType {
		case ReproductionBagTypeID:
			return field == ReproductionBagUnitsField
		case ReproductionBodyTypeID:
			return field == ReproductionBodyLastGatherHourField
		}
	case ReproductionConsumeRule:
		switch componentType {
		case ReproductionBagTypeID:
			return field == ReproductionBagUnitsField
		case ReproductionBodyTypeID:
			return field == ReproductionBodyEnergyField || field == ReproductionBodyHungerField || field == ReproductionBodyCapLostField || field == ReproductionBodyConsumedField
		}
	case ReproductionBasalRule:
		switch componentType {
		case ReproductionBodyTypeID:
			return field == ReproductionBodyEnergyField || field == ReproductionBodyHungerField || field == ReproductionBodyBasalSpentField || field == ReproductionBodyBasalDebtField
		case ReproductionGenomeTypeID:
			return field == ReproductionGenomeDiedHourField
		}
	case ReproductionBuildRule:
		switch componentType {
		case ReproductionBagTypeID:
			return field == ReproductionBagUnitsField
		case ReproductionWorksiteTypeID:
			return field == ReproductionWorksiteCapitalField || field == ReproductionWorksiteWipField || field == ReproductionWorksiteInvestedUnitsField || field == ReproductionWorksitePointsCreatedField || field == ReproductionWorksiteLastBuildHourField
		}
	case ReproductionYieldRule:
		if componentType == ReproductionGranaryTypeID {
			return field == ReproductionGranaryStockField || field == ReproductionGranaryYieldTotalField || field == ReproductionGranaryYieldUnrealizedField || field == ReproductionGranaryYieldDebtField
		}
	case ReproductionWearRule:
		if componentType == ReproductionWorksiteTypeID {
			return field == ReproductionWorksiteCapitalField || field == ReproductionWorksiteWearDebtField || field == ReproductionWorksitePointsDecayedField
		}
	case ReproductionEatStoredRule:
		switch componentType {
		case ReproductionGranaryTypeID:
			return field == ReproductionGranaryStockField || field == ReproductionGranaryStoredMealsField || field == ReproductionGranaryLastStoredMealHourField
		case ReproductionBodyTypeID:
			return field == ReproductionBodyEnergyField || field == ReproductionBodyHungerField || field == ReproductionBodyCapLostField
		}
	case ReproductionBirthRequestRule:
		if componentType == ReproductionBirthRequestTypeID {
			return field == ReproductionBirthRequestHourField || field == ReproductionBirthRequestAddresseeField || field == ReproductionBirthRequestReportedCapitalField || field == ReproductionBirthRequestReportedGranaryField || field == ReproductionBirthRequestStatusField || field == ReproductionBirthRequestLastRequestHourField
		}
	case ReproductionBirthReplyRule:
		switch componentType {
		case ReproductionBodyTypeID:
			return field == ReproductionBodyEnergyField || field == ReproductionBodyBasalSpentField
		case ReproductionGranaryTypeID:
			return field == ReproductionGranaryStockField
		case ReproductionGenomeTypeID:
			return field == ReproductionGenomeBirthGranaryPaidField
		case ReproductionBirthRequestTypeID:
			return field == ReproductionBirthRequestStatusField || field == ReproductionBirthRequestLastReplyHourField
		}
	case ReproductionBirthRefuseRule:
		if componentType == ReproductionBirthRequestTypeID {
			return field == ReproductionBirthRequestStatusField || field == ReproductionBirthRequestLastReplyHourField
		}
	case ReproductionBirthExpireRule:
		if componentType == ReproductionBirthRequestTypeID {
			return field == ReproductionBirthRequestStatusField
		}
	case ReproductionAgeDeathRule:
		if componentType == ReproductionGenomeTypeID {
			return field == ReproductionGenomeDiedHourField
		}
	}
	return false
}

func reproductionWorldRule(rule sim.RuleID) bool {
	switch rule {
	case ReproductionProduceRule, ReproductionBasalRule, ReproductionYieldRule, ReproductionWearRule, ReproductionBirthExpireRule, ReproductionAgeDeathRule:
		return true
	}
	return false
}

// ReproductionCheckOwnership enforces frozen possession-by-identity (gate
// G5) at the trusted harness boundary: world-caused rules (401 produce, 404
// basal, 406 yield, 407 wear, 412 expiry, 413 age-death) carry a world
// cause; every other patch targets the causal actor's own rows; the
// birth-reply (410) may patch only its enumerated parent-cost fields on the
// two parents and the birth-refusal (411) only the two request rows; only
// rule 410 allocates, and only the six per-actor v4 components. The kernel
// validates schema and rule admission, not ownership; the runner and
// journal replay must apply this check to every reproduction proposal.
func ReproductionCheckOwnership(p kernel.Proposal) error {
	if p.RuleVersion != ReproductionRuleVersion {
		return ErrReproductionContract
	}
	if reproductionWorldRule(p.Rule) != p.Cause.World || (p.Cause.World && p.Cause.Actor != 0) || (!p.Cause.World && p.Cause.Actor == 0) {
		return ErrReproductionContract
	}
	causeAmongPatched := false
	for _, patch := range p.Patches {
		switch patch.Component {
		case ReproductionPatchTypeID:
			if p.Rule != ReproductionProduceRule {
				return ErrReproductionContract
			}
			if _, err := ReproductionPatchID(int(patch.Entity) - 1001); err != nil {
				return ErrReproductionContract
			}
		case ReproductionSlotTypeID:
			if p.Rule != ReproductionProduceRule && p.Rule != ReproductionGatherRule {
				return ErrReproductionContract
			}
			if !reproductionSlotEntity(patch.Entity) {
				return ErrReproductionContract
			}
		default:
			if !reproductionPatchFieldAllowed(p.Rule, patch.Component, patch.Field) {
				return ErrReproductionContract
			}
			if !reproductionActorEntity(patch.Entity) {
				return ErrReproductionContract
			}
			switch p.Rule {
			case ReproductionGatherRule, ReproductionConsumeRule, ReproductionBuildRule, ReproductionEatStoredRule, ReproductionBirthRequestRule:
				// Possession-by-identity: a causal actor may only mutate
				// its own rows; there is no transfer or salvage path.
				if patch.Entity != p.Cause.Actor {
					return ErrReproductionContract
				}
			case ReproductionBirthReplyRule, ReproductionBirthRefuseRule:
				if patch.Entity == p.Cause.Actor {
					causeAmongPatched = true
				}
			}
		}
	}
	if (p.Rule == ReproductionBirthReplyRule || p.Rule == ReproductionBirthRefuseRule) && !causeAmongPatched {
		return ErrReproductionContract
	}
	for _, seed := range p.Allocations {
		if p.Rule != ReproductionBirthReplyRule {
			return ErrReproductionContract
		}
		switch seed.Component {
		case ReproductionBagTypeID, ReproductionBodyTypeID, ReproductionWorksiteTypeID, ReproductionGranaryTypeID, ReproductionGenomeTypeID, ReproductionBirthRequestTypeID:
		default:
			return ErrReproductionContract
		}
		if !reproductionActorEntity(seed.Entity) {
			return ErrReproductionContract
		}
	}
	return nil
}

// ReproductionGatherProposal builds the validated gather intention at the
// frozen h+1µs claim time: slot stock/gathered plus the gatherer's own bag
// and body rows. Eligibility is delegated entirely to ReproductionGather.
func ReproductionGatherProposal(key string, hour int, actor, slot sim.EntityID, slotBefore ReproductionSlotState, a ReproductionActorState) (kernel.Proposal, error) {
	home := a.Bag.Source
	if _, err := ReproductionPatchID(int(home) - 1001); err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	nextSlot, next, err := ReproductionGather(home, slot, actor, hour, slotBefore, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: ReproductionGatherRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{
			{Entity: slot, Component: ReproductionSlotTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionSlotStockField, Value: sim.IntegerValue(nextSlot.Stock)},
			{Entity: slot, Component: ReproductionSlotTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionSlotGatheredField, Value: sim.IntegerValue(nextSlot.Gathered)},
			{Entity: actor, Component: ReproductionBagTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyLastGatherHourField, Value: sim.IntegerValue(next.Body.LastGatherHour)},
		}}, nil
}

// patchOfActor is the founder ID placement; it rejects newborn IDs because a
// newborn's home patch is carried by its bag row, not its ID.
func patchOfActor(actor sim.EntityID) sim.EntityID {
	if !reproductionFounderEntity(actor) {
		return 0
	}
	patch, err := ReproductionFounderPatchID(actor)
	if err != nil {
		return 0
	}
	return patch
}

// ReproductionConsumeProposal eats the held wild unit at the frozen
// h+20m+2µs bag decision, the Build alternative of the single bag decision
// point. The bag's home-patch source is retained, never cleared.
func ReproductionConsumeProposal(key string, hour int, actor sim.EntityID, a ReproductionActorState) (kernel.Proposal, error) {
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseBagDecision)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	next, _, err := ReproductionConsume(actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: ReproductionConsumeRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: ReproductionBagTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyConsumedField, Value: sim.IntegerValue(next.Body.Consumed)},
		}}, nil
}

// ReproductionEatStoredProposal is the only granary draw. at must be exactly
// the h+1µs claim or the h+40m+4µs paired meal of the given hour.
func ReproductionEatStoredProposal(key string, hour int, at sim.SimTime, actor sim.EntityID, a ReproductionActorState) (kernel.Proposal, error) {
	claim, err := ReproductionPhaseTime(hour, ReproductionPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	paired, _ := ReproductionPhaseTime(hour, ReproductionPhasePairedMeal)
	if at != claim && at != paired {
		return kernel.Proposal{}, ErrReproductionContract
	}
	next, _, err := ReproductionEatStored(hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: ReproductionEatStoredRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryStockField, Value: sim.IntegerValue(next.Granary.Stock)},
			{Entity: actor, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryStoredMealsField, Value: sim.IntegerValue(next.Granary.StoredMeals)},
			{Entity: actor, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryLastStoredMealHourField, Value: sim.IntegerValue(next.Granary.LastStoredMealHour)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
		}}, nil
}

// ReproductionBuildProposal commits the frozen atomic bag→wip/k conversion
// at the build completion instant h+40m+3µs; the caller's fresh snapshot is
// validated by ReproductionBuild before any paired eat-stored of the hour.
func ReproductionBuildProposal(key string, hour int, actor sim.EntityID, a ReproductionActorState) (kernel.Proposal, error) {
	start, err := ReproductionPhaseTime(hour, ReproductionPhaseBuildStart)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	next, _, err := ReproductionBuild(hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: ReproductionBuildCompletion(start), Cause: kernel.Cause{Actor: actor}, Rule: ReproductionBuildRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: ReproductionBagTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: ReproductionWorksiteTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionWorksiteCapitalField, Value: sim.IntegerValue(next.Worksite.Capital)},
			{Entity: actor, Component: ReproductionWorksiteTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionWorksiteWipField, Value: sim.IntegerValue(next.Worksite.Wip)},
			{Entity: actor, Component: ReproductionWorksiteTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionWorksiteInvestedUnitsField, Value: sim.IntegerValue(next.Worksite.InvestedUnits)},
			{Entity: actor, Component: ReproductionWorksiteTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionWorksitePointsCreatedField, Value: sim.IntegerValue(next.Worksite.PointsCreated)},
			{Entity: actor, Component: ReproductionWorksiteTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionWorksiteLastBuildHourField, Value: sim.IntegerValue(next.Worksite.LastBuildHour)},
		}}, nil
}

// ReproductionBirthRequestProposal files the addressed request at the frozen
// h+40m+5µs phase with self-reported capital/granary claims. Eligibility is
// delegated to ReproductionBirthRequest.
func ReproductionBirthRequestProposal(key string, hour int, proposer, addressee sim.EntityID, reportedCapital, reportedGranary int64, a ReproductionActorState) (kernel.Proposal, error) {
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthRequest)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	next, err := ReproductionBirthRequest(hour, proposer, addressee, reportedCapital, reportedGranary, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	addresseeRef, err := sim.EntityRefValue(addressee)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: proposer}, Rule: ReproductionBirthRequestRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{
			{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestHourField, Value: sim.IntegerValue(next.Request.Hour)},
			{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestAddresseeField, Value: addresseeRef},
			{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestReportedCapitalField, Value: sim.IntegerValue(next.Request.ReportedCapital)},
			{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestReportedGranaryField, Value: sim.IntegerValue(next.Request.ReportedGranary)},
			{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestStatusField, Value: sim.IntegerValue(int64(next.Request.Status))},
			{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestLastRequestHourField, Value: sim.IntegerValue(next.Request.LastRequestHour)},
		}}, nil
}

// ReproductionBirthReplyProposal builds the single atomic proposal of the
// frozen h+40m+6µs reply phase from a decided ReproductionBirthReplyResult.
// An accepted reply patches both parents' enumerated cost rows (energy and
// basal-spent booked through the body, granary stock, genome
// birth-granary-paid) plus the two request rows, and allocates the
// newborn's six complete rows as one entity-create. A refusal patches only
// the proposer's request status and the consenter's last-reply-hour. A
// zero-mutation refusal (a death between phases) has no proposal at all.
func ReproductionBirthReplyProposal(key string, hour int, proposer, consenter sim.EntityID, result ReproductionReplyResult) (kernel.Proposal, error) {
	at, err := ReproductionPhaseTime(hour, ReproductionPhaseBirthReply)
	if err != nil {
		return kernel.Proposal{}, ErrReproductionContract
	}
	switch result.Decision {
	case ReproductionReplyRefuseSilent:
		return kernel.Proposal{}, ErrReproductionContract
	case ReproductionReplyRefuse:
		return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: consenter}, Rule: ReproductionBirthRefuseRule, RuleVersion: ReproductionRuleVersion,
			Patches: []component.Patch{
				{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestStatusField, Value: sim.IntegerValue(int64(result.Proposer.Request.Status))},
				{Entity: consenter, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestLastReplyHourField, Value: sim.IntegerValue(result.Consenter.Request.LastReplyHour)},
			}}, nil
	case ReproductionReplyAccept:
		if !validReproductionActor(proposer, result.Proposer) || !validReproductionActor(consenter, result.Consenter) {
			return kernel.Proposal{}, ErrReproductionContract
		}
		allocations, err := reproductionNewbornAllocations(result.NewbornID, result.Newborn.Bag.Source, result.Newborn)
		if err != nil {
			return kernel.Proposal{}, err
		}
		return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: consenter}, Rule: ReproductionBirthReplyRule, RuleVersion: ReproductionRuleVersion,
			Patches: []component.Patch{
				{Entity: proposer, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(result.Proposer.Body.Energy)},
				{Entity: proposer, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyBasalSpentField, Value: sim.IntegerValue(result.Proposer.Body.BasalSpent)},
				{Entity: proposer, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryStockField, Value: sim.IntegerValue(result.Proposer.Granary.Stock)},
				{Entity: proposer, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGenomeBirthGranaryPaidField, Value: sim.IntegerValue(result.Proposer.Genome.BirthGranaryPaid)},
				{Entity: proposer, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestStatusField, Value: sim.IntegerValue(int64(result.Proposer.Request.Status))},
				{Entity: consenter, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(result.Consenter.Body.Energy)},
				{Entity: consenter, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyBasalSpentField, Value: sim.IntegerValue(result.Consenter.Body.BasalSpent)},
				{Entity: consenter, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryStockField, Value: sim.IntegerValue(result.Consenter.Granary.Stock)},
				{Entity: consenter, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGenomeBirthGranaryPaidField, Value: sim.IntegerValue(result.Consenter.Genome.BirthGranaryPaid)},
				{Entity: consenter, Component: ReproductionBirthRequestTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBirthRequestLastReplyHourField, Value: sim.IntegerValue(result.Consenter.Request.LastReplyHour)},
			}, Allocations: allocations}, nil
	}
	return kernel.Proposal{}, ErrReproductionContract
}

// reproductionNewbornAllocations builds the newborn's six complete rows
// (bag, body, worksite, granary, genome, birthrequest) in the kernel's
// full-schema allocation shape.
func reproductionNewbornAllocations(id sim.EntityID, home sim.EntityID, n ReproductionActorState) ([]component.ComponentSeed, error) {
	if !reproductionActorEntity(id) || n.Bag.Source != home {
		return nil, ErrReproductionContract
	}
	homeRef, err := sim.EntityRefValue(home)
	if err != nil {
		return nil, ErrReproductionContract
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
		{Entity: id, Component: ReproductionBagTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionBagUnitsField, Value: sim.IntegerValue(n.Bag.Units)},
			{Field: ReproductionBagSourceField, Value: homeRef}}},
		{Entity: id, Component: ReproductionBodyTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(n.Body.Energy)},
			{Field: ReproductionBodyHungerField, Value: sim.IntegerValue(n.Body.Hunger)},
			{Field: ReproductionBodyBasalSpentField, Value: sim.IntegerValue(n.Body.BasalSpent)},
			{Field: ReproductionBodyCapLostField, Value: sim.IntegerValue(n.Body.CapLost)},
			{Field: ReproductionBodyConsumedField, Value: sim.IntegerValue(n.Body.Consumed)},
			{Field: ReproductionBodyLastGatherHourField, Value: sim.IntegerValue(n.Body.LastGatherHour)},
			{Field: ReproductionBodyBasalDebtField, Value: sim.IntegerValue(n.Body.BasalDebt)}}},
		{Entity: id, Component: ReproductionWorksiteTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionWorksiteCapitalField, Value: sim.IntegerValue(n.Worksite.Capital)},
			{Field: ReproductionWorksiteWipField, Value: sim.IntegerValue(n.Worksite.Wip)},
			{Field: ReproductionWorksiteWearDebtField, Value: sim.IntegerValue(n.Worksite.WearDebt)},
			{Field: ReproductionWorksiteInvestedUnitsField, Value: sim.IntegerValue(n.Worksite.InvestedUnits)},
			{Field: ReproductionWorksitePointsCreatedField, Value: sim.IntegerValue(n.Worksite.PointsCreated)},
			{Field: ReproductionWorksitePointsDecayedField, Value: sim.IntegerValue(n.Worksite.PointsDecayed)},
			{Field: ReproductionWorksiteLastBuildHourField, Value: sim.IntegerValue(n.Worksite.LastBuildHour)}}},
		{Entity: id, Component: ReproductionGranaryTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionGranaryStockField, Value: sim.IntegerValue(n.Granary.Stock)},
			{Field: ReproductionGranaryYieldTotalField, Value: sim.IntegerValue(n.Granary.YieldTotal)},
			{Field: ReproductionGranaryYieldUnrealizedField, Value: sim.IntegerValue(n.Granary.YieldUnrealized)},
			{Field: ReproductionGranaryStoredMealsField, Value: sim.IntegerValue(n.Granary.StoredMeals)},
			{Field: ReproductionGranaryLastStoredMealHourField, Value: sim.IntegerValue(n.Granary.LastStoredMealHour)},
			{Field: ReproductionGranaryYieldDebtField, Value: sim.IntegerValue(n.Granary.YieldDebt)}}},
		{Entity: id, Component: ReproductionGenomeTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionGenomeLocusMField, Value: sim.IntegerValue(n.Genome.LocusM)},
			{Field: ReproductionGenomeLocusYField, Value: sim.IntegerValue(n.Genome.LocusY)},
			{Field: ReproductionGenomeLocusWField, Value: sim.IntegerValue(n.Genome.LocusW)},
			{Field: ReproductionGenomeParentAField, Value: parentARef},
			{Field: ReproductionGenomeParentBField, Value: parentBRef},
			{Field: ReproductionGenomeBirthHourField, Value: sim.IntegerValue(n.Genome.BirthHour)},
			{Field: ReproductionGenomeDeathHourField, Value: sim.IntegerValue(n.Genome.DeathHour)},
			{Field: ReproductionGenomeDiedHourField, Value: sim.IntegerValue(n.Genome.DiedHour)},
			{Field: ReproductionGenomeBirthGranaryPaidField, Value: sim.IntegerValue(n.Genome.BirthGranaryPaid)}}},
		{Entity: id, Component: ReproductionBirthRequestTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionBirthRequestHourField, Value: sim.IntegerValue(n.Request.Hour)},
			{Field: ReproductionBirthRequestAddresseeField, Value: addresseeRef},
			{Field: ReproductionBirthRequestReportedCapitalField, Value: sim.IntegerValue(n.Request.ReportedCapital)},
			{Field: ReproductionBirthRequestReportedGranaryField, Value: sim.IntegerValue(n.Request.ReportedGranary)},
			{Field: ReproductionBirthRequestStatusField, Value: sim.IntegerValue(int64(n.Request.Status))},
			{Field: ReproductionBirthRequestLastRequestHourField, Value: sim.IntegerValue(n.Request.LastRequestHour)},
			{Field: ReproductionBirthRequestLastReplyHourField, Value: sim.IntegerValue(n.Request.LastReplyHour)}}},
	}, nil
}

// ReproductionBirthDrawPosition is the frozen birth-stream position of a
// birth's seven draws: 7×(ordinal−1), with ordinal = births+1 derived from
// committed state. The stream itself is runner-owned; this contract only
// pins the position arithmetic.
func ReproductionBirthDrawPosition(births int64) (uint64, error) {
	if births < 0 || births >= ReproductionMaxBirths {
		return 0, ErrReproductionContract
	}
	return uint64(ReproductionDrawsPerBirth * births), nil
}

func genomeRefValue(id sim.EntityID) (sim.Value, error) {
	if id == 0 {
		return sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	}
	return sim.EntityRefValue(id)
}
