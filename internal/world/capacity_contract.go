package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"fmt"
)

// Capacity v3 is the frozen productive-capacity pilot (ticket 13). Its
// component, rule, and projection identities are disjoint from v1 (food-flow)
// and v2 (social-food): a restored v1/v2 history cannot satisfy these
// identities by accident, and a v3 history cannot restore into a v1/v2
// registry. The frozen design section of
// .scratch/13-productive-capacity/spec.md is binding; every constant below is
// quoted from it, not derived.
var ErrCapacityContract = errors.New("invalid capacity v3 contract")

const (
	CapacityFormatVersion     uint32                = 3
	CapacitySchemaVersion     sim.SchemaVersion     = 3
	CapacityRuleVersion       uint32                = 3
	CapacityProjectionVersion sim.ProjectionVersion = 3

	CapacityActorCount    = 16
	CapacityPatchCount    = 2
	CapacitySlotsPerPatch = 8
	CapacityHorizonHours  = 168

	CapacityHour           sim.Duration = 3_600_000_000
	CapacityGatherDuration sim.Duration = 20 * 60 * 1_000_000
	CapacityMealDuration   sim.Duration = 10 * 60 * 1_000_000

	CapacityInitialEnergy  int64 = 11
	CapacityEnergyCapacity int64 = 12
	CapacityHungerCapacity int64 = 24
	CapacityBagCapacity    int64 = 1
	CapacityNeverHour      int64 = -1

	// Frozen v3 capacity constants. One capital point costs two bag units
	// (two food units plus two activity-hours), yields exactly one food unit
	// per pulse into the owner's granary, and survives exactly
	// CapacityWearPeriod pulses of depreciation.
	CapacityPointCostWip  int64        = 2
	CapacityYieldPerPoint int64        = 1
	CapacityKMax          int64        = 8
	CapacityWipMax        int64        = 1
	CapacityGranaryMax    int64        = 8
	CapacityWearPeriod    int64        = 6
	CapacityWearDebtMax   int64        = 13
	CapacityBuildDuration sim.Duration = 20 * 60 * 1_000_000
	CapacityPolicyTLow    int64        = 5

	CapacityPatchTypeID    sim.ComponentTypeID = 0x80000030
	CapacitySlotTypeID     sim.ComponentTypeID = 0x80000031
	CapacityBagTypeID      sim.ComponentTypeID = 0x80000032
	CapacityBodyTypeID     sim.ComponentTypeID = 0x80000033
	CapacityWorksiteTypeID sim.ComponentTypeID = 0x80000034
	CapacityGranaryTypeID  sim.ComponentTypeID = 0x80000035

	CapacityProduceRule   sim.RuleID = 301 // produce-wild
	CapacityGatherRule    sim.RuleID = 302 // gather
	CapacityConsumeRule   sim.RuleID = 303 // consume
	CapacityBasalRule     sim.RuleID = 304 // basal
	CapacityBuildRule     sim.RuleID = 305 // build
	CapacityYieldRule     sim.RuleID = 306 // yield
	CapacityWearRule      sim.RuleID = 307 // wear
	CapacityEatStoredRule sim.RuleID = 308 // eat-stored
)

const (
	CapacityPatchYieldField sim.FieldID = 1 + iota
	CapacityPatchPulsesField
	CapacityPatchProducedField
	CapacityPatchUnrealizedField
)
const (
	CapacitySlotStockField sim.FieldID = 1 + iota
	CapacitySlotPatchField
	CapacitySlotGatheredField
)
const (
	CapacityBagUnitsField sim.FieldID = 1 + iota
	CapacityBagSourceField
)
const (
	CapacityBodyEnergyField sim.FieldID = 1 + iota
	CapacityBodyHungerField
	CapacityBodyBasalSpentField
	CapacityBodyCapLostField
	CapacityBodyConsumedField
	CapacityBodyLastGatherHourField
)
const (
	CapacityWorksiteCapitalField sim.FieldID = 1 + iota
	CapacityWorksiteWipField
	CapacityWorksiteWearDebtField
	CapacityWorksiteInvestedUnitsField
	CapacityWorksitePointsCreatedField
	CapacityWorksitePointsDecayedField
	CapacityWorksiteLastBuildHourField
)
const (
	CapacityGranaryStockField sim.FieldID = 1 + iota
	CapacityGranaryYieldTotalField
	CapacityGranaryYieldUnrealizedField
	CapacityGranaryStoredMealsField
	CapacityGranaryLastStoredMealHourField
)

// Frozen pulse order inside hour h: basal, wild production, capital yield
// (living owners only), wear accrual (all owners) at h+0; claims at h+1µs;
// gather completion at h+20m+1µs; bag decision at h+20m+2µs; build start at
// h+20m+3µs completing at h+40m+3µs with the atomic bag→wip/k conversion;
// paired EatStored at h+40m+4µs after fresh-snapshot revalidation.
const (
	CapacityPhasePulse int = iota
	CapacityPhaseClaim
	CapacityPhaseGathered
	CapacityPhaseBagDecision
	CapacityPhaseBuildStart
	CapacityPhasePairedMeal
)

// Actor, patch, and slot entity ranges are fixed and disjoint from each other
// and from the S6 allocator. Worksite and granary components live on the
// actors themselves; there are no separate worksite or granary entities.
func CapacityPatchID(index int) (sim.EntityID, error) {
	if index < 0 || index >= CapacityPatchCount {
		return 0, ErrCapacityContract
	}
	return sim.EntityID(1001 + index), nil
}

func CapacitySlotID(patchIndex, slotIndex int) (sim.EntityID, error) {
	if patchIndex < 0 || patchIndex >= CapacityPatchCount || slotIndex < 0 || slotIndex >= CapacitySlotsPerPatch {
		return 0, ErrCapacityContract
	}
	return sim.EntityID(2001 + patchIndex*CapacitySlotsPerPatch + slotIndex), nil
}

func CapacityActorPatchID(actor sim.EntityID) (sim.EntityID, error) {
	if actor < 1 || actor > CapacityActorCount {
		return 0, ErrCapacityContract
	}
	return CapacityPatchID((int(actor) - 1) / CapacitySlotsPerPatch)
}

// Hour h=0..168 is the pulse boundary; h=168 is basal only, as in v1.
func CapacityHourTime(h int) (sim.SimTime, error) {
	if h < 0 || h > CapacityHorizonHours {
		return 0, ErrCapacityContract
	}
	return sim.SimTime(h) * sim.SimTime(CapacityHour), nil
}

func CapacityPhaseTime(hour int, phase int) (sim.SimTime, error) {
	if hour < 0 || hour >= CapacityHorizonHours || phase < CapacityPhasePulse || phase > CapacityPhasePairedMeal {
		return 0, ErrCapacityContract
	}
	at, _ := CapacityHourTime(hour)
	switch phase {
	case CapacityPhaseClaim:
		return at + 1, nil
	case CapacityPhaseGathered:
		return at + sim.SimTime(CapacityGatherDuration) + 1, nil
	case CapacityPhaseBagDecision:
		return at + sim.SimTime(CapacityGatherDuration) + 2, nil
	case CapacityPhaseBuildStart:
		return at + sim.SimTime(CapacityGatherDuration) + 3, nil
	case CapacityPhasePairedMeal:
		return at + sim.SimTime(CapacityGatherDuration+CapacityBuildDuration) + 4, nil
	}
	return at, nil // pulse
}

// CapacityBuildCompletion is the frozen atomic bag→wip/k conversion instant of
// a build started at the exact CapacityPhaseBuildStart time of its hour.
func CapacityBuildCompletion(start sim.SimTime) sim.SimTime {
	return start + sim.SimTime(CapacityBuildDuration)
}

func capacityBounds(min, max float64) component.Bounds {
	return component.Bounds{HasMinimum: true, Minimum: min, HasMaximum: true, Maximum: max}
}
func capacityHourBounds() component.Bounds {
	return capacityBounds(float64(CapacityNeverHour), float64(CapacityHorizonHours-1))
}
func capacityInteger(id sim.FieldID, name, unit string, min, max float64) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.IntegerKind}, Unit: unit, Bounds: capacityBounds(min, max), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}
}
func capacityRef(id sim.FieldID, name string) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.EntityRefKind}, Unit: "entity-id", PermittedStates: []sim.ValueState{sim.Present, sim.Missing}, Uncertainty: component.UncertaintyForbidden}
}
func capacityRule(id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return component.TransitionRuleDescriptor{ID: id, Version: CapacityRuleVersion, Name: "capacity-" + name + "-v3"}
}

// capacityDescriptor pins the v2 identity-projection pattern: every integer
// field carries a same-ID, coefficient-1 projection with the field's own
// bounds, so engine metrics are direct bounded state, never derived blends.
// Ref fields carry no projection. Worksite and granary rows are private
// per-actor components; no cross-actor observation constructor exists in this
// package, and only the owner's validated intentions may mutate them.
func capacityDescriptor(id sim.ComponentTypeID, name string, fields []component.FieldDescriptor, rules ...component.TransitionRuleDescriptor) component.ComponentDescriptor {
	projections := make([]component.ProjectionDescriptor, 0, len(fields))
	for _, f := range fields {
		if f.Type.Kind != sim.IntegerKind {
			continue
		}
		projections = append(projections, component.ProjectionDescriptor{ID: sim.ProjectionID(f.ID), Version: CapacityProjectionVersion, Name: name + "-" + f.Name, SourceFields: []sim.FieldID{f.ID}, Coefficients: []float64{1}, Unit: f.Unit, Bounds: f.Bounds, MissingBehavior: component.PreserveSourceState})
	}
	return component.ComponentDescriptor{TypeID: id, SchemaVersion: CapacitySchemaVersion, Name: name, Fields: fields, AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: rules, Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func CapacityPatchDescriptor() component.ComponentDescriptor {
	return capacityDescriptor(CapacityPatchTypeID, "capacity-patch-v3", []component.FieldDescriptor{
		capacityInteger(1, "yield", "food/hour", 0, CapacitySlotsPerPatch), capacityInteger(2, "pulses", "hours", 0, CapacityHorizonHours), capacityInteger(3, "produced", "food", 0, CapacityHorizonHours*CapacitySlotsPerPatch), capacityInteger(4, "unrealized", "food", 0, CapacityHorizonHours*CapacitySlotsPerPatch)}, capacityRule(CapacityProduceRule, "produce-wild"))
}

func CapacitySlotDescriptor() component.ComponentDescriptor {
	return capacityDescriptor(CapacitySlotTypeID, "capacity-slot-v3", []component.FieldDescriptor{
		capacityInteger(1, "stock", "food", 0, 1), capacityRef(2, "patch"), capacityInteger(3, "gathered", "food", 0, CapacityHorizonHours)}, capacityRule(CapacityProduceRule, "produce-wild"), capacityRule(CapacityGatherRule, "gather"))
}

func CapacityBagDescriptor() component.ComponentDescriptor {
	// The bag stays 0..1 and wild-only; it is never a store. Every consumed
	// bag unit is either a wild meal or one invested build unit.
	return capacityDescriptor(CapacityBagTypeID, "capacity-bag-v3", []component.FieldDescriptor{
		capacityInteger(1, "units", "food", 0, float64(CapacityBagCapacity)), capacityRef(2, "source-patch")}, capacityRule(CapacityGatherRule, "gather"), capacityRule(CapacityConsumeRule, "consume"), capacityRule(CapacityBuildRule, "build"))
}

func CapacityBodyDescriptor() component.ComponentDescriptor {
	return capacityDescriptor(CapacityBodyTypeID, "capacity-body-v3", []component.FieldDescriptor{
		capacityInteger(1, "energy", "energy", 0, float64(CapacityEnergyCapacity)), capacityInteger(2, "hunger", "hours", 0, float64(CapacityHungerCapacity)), capacityInteger(3, "basal-spent", "energy", 0, CapacityHorizonHours), capacityInteger(4, "cap-lost", "energy", 0, CapacityHorizonHours), capacityInteger(5, "consumed-home-patch", "food", 0, CapacityHorizonHours), {ID: CapacityBodyLastGatherHourField, Name: "last-gather-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: capacityHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}}, capacityRule(CapacityGatherRule, "gather"), capacityRule(CapacityConsumeRule, "consume"), capacityRule(CapacityBasalRule, "basal"), capacityRule(CapacityEatStoredRule, "eat-stored"))
}

func CapacityWorksiteDescriptor() component.ComponentDescriptor {
	return capacityDescriptor(CapacityWorksiteTypeID, "capacity-worksite-v3", []component.FieldDescriptor{
		capacityInteger(1, "capital", "points", 0, float64(CapacityKMax)),
		capacityInteger(2, "wip", "units", 0, float64(CapacityWipMax)),
		capacityInteger(3, "wear-debt", "points", 0, float64(CapacityWearDebtMax)),
		capacityInteger(4, "invested-units", "units", 0, CapacityHorizonHours),
		capacityInteger(5, "points-created", "points", 0, CapacityHorizonHours),
		capacityInteger(6, "points-decayed", "points", 0, CapacityHorizonHours),
		{ID: CapacityWorksiteLastBuildHourField, Name: "last-build-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: capacityHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}}, capacityRule(CapacityBuildRule, "build"), capacityRule(CapacityWearRule, "wear"))
}

func CapacityGranaryDescriptor() component.ComponentDescriptor {
	return capacityDescriptor(CapacityGranaryTypeID, "capacity-granary-v3", []component.FieldDescriptor{
		capacityInteger(1, "stock", "food", 0, float64(CapacityGranaryMax)),
		capacityInteger(2, "yield-total", "food", 0, CapacityHorizonHours*CapacitySlotsPerPatch),
		capacityInteger(3, "yield-unrealized", "food", 0, CapacityHorizonHours*CapacitySlotsPerPatch),
		capacityInteger(4, "stored-meals", "meals", 0, CapacityHorizonHours),
		{ID: CapacityGranaryLastStoredMealHourField, Name: "last-stored-meal-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: capacityHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}}, capacityRule(CapacityYieldRule, "yield"), capacityRule(CapacityEatStoredRule, "eat-stored"))
}

// CapacityRegistry is intentionally disjoint from the v1, v2, and S6
// registries: type IDs, rule IDs, and versions never collide, so no history
// can be replayed across world versions by accident.
func CapacityRegistry() (component.Registry, error) {
	b := component.NewRegistryBuilder()
	for _, d := range []component.ComponentDescriptor{CapacityPatchDescriptor(), CapacitySlotDescriptor(), CapacityBagDescriptor(), CapacityBodyDescriptor(), CapacityWorksiteDescriptor(), CapacityGranaryDescriptor()} {
		if err := b.Register(d); err != nil {
			return component.Registry{}, fmt.Errorf("%w: %v", ErrCapacityContract, err)
		}
	}
	return b.Freeze()
}

func capacitySlotEntity(entity sim.EntityID) bool {
	return entity >= 2001 && entity < sim.EntityID(2001+CapacityPatchCount*CapacitySlotsPerPatch)
}

// capacityRuleTouchesComponent reports whether the rule may patch the
// component at all; entity ownership is checked separately.
func capacityRuleTouchesComponent(rule sim.RuleID, component sim.ComponentTypeID) bool {
	switch rule {
	case CapacityGatherRule:
		return component == CapacityBagTypeID || component == CapacityBodyTypeID
	case CapacityConsumeRule:
		return component == CapacityBagTypeID || component == CapacityBodyTypeID
	case CapacityBasalRule:
		return component == CapacityBodyTypeID
	case CapacityBuildRule:
		return component == CapacityBagTypeID || component == CapacityWorksiteTypeID
	case CapacityYieldRule:
		return component == CapacityGranaryTypeID
	case CapacityWearRule:
		return component == CapacityWorksiteTypeID
	case CapacityEatStoredRule:
		return component == CapacityBodyTypeID || component == CapacityGranaryTypeID
	}
	return false
}

// CapacityCheckOwnership enforces frozen possession-by-identity (gate G5) at
// the trusted harness boundary: an actor's bag, body, worksite, or granary may
// only be patched by that owner's own intention (or by an authorized world
// pulse row for the entity itself), produce-wild is the only writer of the
// shared patch ledger, and gather additionally writes open-access slot stock.
// The kernel validates schema and rule admission, not ownership; the runner
// and journal replay must apply this check to every capacity proposal.
func CapacityCheckOwnership(p kernel.Proposal) error {
	for _, patch := range p.Patches {
		switch patch.Component {
		case CapacityPatchTypeID:
			if p.Rule != CapacityProduceRule || !p.Cause.World {
				return ErrCapacityContract
			}
			if _, err := CapacityPatchID(int(patch.Entity) - 1001); err != nil {
				return ErrCapacityContract
			}
		case CapacitySlotTypeID:
			if (p.Rule != CapacityProduceRule || !p.Cause.World) && p.Rule != CapacityGatherRule {
				return ErrCapacityContract
			}
			if !capacitySlotEntity(patch.Entity) {
				return ErrCapacityContract
			}
		case CapacityBagTypeID, CapacityBodyTypeID, CapacityWorksiteTypeID, CapacityGranaryTypeID:
			if p.Rule == CapacityProduceRule || !capacityRuleTouchesComponent(p.Rule, patch.Component) {
				return ErrCapacityContract
			}
			if _, err := CapacityActorPatchID(patch.Entity); err != nil {
				return ErrCapacityContract
			}
			// Possession-by-identity: a causal actor may only mutate its own
			// rows; there is no transfer, inheritance, or salvage path.
			if !p.Cause.World && patch.Entity != p.Cause.Actor {
				return ErrCapacityContract
			}
		default:
			// v1/v2/S6 components are unreachable under v3 rules.
			return ErrCapacityContract
		}
	}
	return nil
}

// CapacityGatherProposal builds the validated gather intention at the frozen
// h+1µs claim time: slot stock/gathered plus the gatherer's own bag and body
// rows. Eligibility is delegated entirely to CapacityGather.
func CapacityGatherProposal(key string, hour int, actor, slot sim.EntityID, slotBefore CapacitySlotState, a CapacityActorState) (kernel.Proposal, error) {
	patch, err := CapacityActorPatchID(actor)
	if err != nil {
		return kernel.Proposal{}, ErrCapacityContract
	}
	at, err := CapacityPhaseTime(hour, CapacityPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrCapacityContract
	}
	nextSlot, next, err := CapacityGather(patch, slot, actor, hour, slotBefore, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	patchRef, _ := sim.EntityRefValue(patch)
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: CapacityGatherRule, RuleVersion: CapacityRuleVersion,
		Patches: []component.Patch{
			{Entity: slot, Component: CapacitySlotTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacitySlotStockField, Value: sim.IntegerValue(nextSlot.Stock)},
			{Entity: slot, Component: CapacitySlotTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacitySlotGatheredField, Value: sim.IntegerValue(nextSlot.Gathered)},
			{Entity: actor, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagSourceField, Value: patchRef},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyLastGatherHourField, Value: sim.IntegerValue(next.Body.LastGatherHour)},
		}}, nil
}

// CapacityEatProposal builds the wild bag meal at the frozen h+20m+2µs bag
// decision, the Build alternative of the single bag decision point.
func CapacityEatProposal(key string, hour int, actor sim.EntityID, a CapacityActorState) (kernel.Proposal, error) {
	at, err := CapacityPhaseTime(hour, CapacityPhaseBagDecision)
	if err != nil {
		return kernel.Proposal{}, ErrCapacityContract
	}
	next, _, err := CapacityConsume(actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: CapacityConsumeRule, RuleVersion: CapacityRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagSourceField, Value: missing},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyConsumedField, Value: sim.IntegerValue(next.Body.Consumed)},
		}}, nil
}

// CapacityEatStoredProposal builds the only granary draw. at must be exactly
// the h+1µs claim or the h+40m+4µs paired meal of the given hour.
func CapacityEatStoredProposal(key string, hour int, at sim.SimTime, actor sim.EntityID, a CapacityActorState) (kernel.Proposal, error) {
	claim, err := CapacityPhaseTime(hour, CapacityPhaseClaim)
	if err != nil {
		return kernel.Proposal{}, ErrCapacityContract
	}
	paired, _ := CapacityPhaseTime(hour, CapacityPhasePairedMeal)
	if at != claim && at != paired {
		return kernel.Proposal{}, ErrCapacityContract
	}
	next, _, err := CapacityEatStored(hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: actor}, Rule: CapacityEatStoredRule, RuleVersion: CapacityRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: CapacityGranaryTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityGranaryStockField, Value: sim.IntegerValue(next.Granary.Stock)},
			{Entity: actor, Component: CapacityGranaryTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityGranaryStoredMealsField, Value: sim.IntegerValue(next.Granary.StoredMeals)},
			{Entity: actor, Component: CapacityGranaryTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityGranaryLastStoredMealHourField, Value: sim.IntegerValue(next.Granary.LastStoredMealHour)},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyEnergyField, Value: sim.IntegerValue(next.Body.Energy)},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyHungerField, Value: sim.IntegerValue(next.Body.Hunger)},
			{Entity: actor, Component: CapacityBodyTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBodyCapLostField, Value: sim.IntegerValue(next.Body.CapLost)},
		}}, nil
}

// CapacityBuildProposal commits the frozen atomic bag→wip/k conversion at the
// build completion instant h+40m+3µs; the caller's fresh snapshot is validated
// by CapacityBuild before any paired EatStored of the same hour.
func CapacityBuildProposal(key string, hour int, actor sim.EntityID, a CapacityActorState) (kernel.Proposal, error) {
	start, err := CapacityPhaseTime(hour, CapacityPhaseBuildStart)
	if err != nil {
		return kernel.Proposal{}, ErrCapacityContract
	}
	next, _, err := CapacityBuild(hour, actor, a)
	if err != nil {
		return kernel.Proposal{}, err
	}
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	return kernel.Proposal{Key: key, Time: CapacityBuildCompletion(start), Cause: kernel.Cause{Actor: actor}, Rule: CapacityBuildRule, RuleVersion: CapacityRuleVersion,
		Patches: []component.Patch{
			{Entity: actor, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagUnitsField, Value: sim.IntegerValue(next.Bag.Units)},
			{Entity: actor, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagSourceField, Value: missing},
			{Entity: actor, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(next.Worksite.Capital)},
			{Entity: actor, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteWipField, Value: sim.IntegerValue(next.Worksite.Wip)},
			{Entity: actor, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteInvestedUnitsField, Value: sim.IntegerValue(next.Worksite.InvestedUnits)},
			{Entity: actor, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksitePointsCreatedField, Value: sim.IntegerValue(next.Worksite.PointsCreated)},
			{Entity: actor, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteLastBuildHourField, Value: sim.IntegerValue(next.Worksite.LastBuildHour)},
		}}, nil
}
