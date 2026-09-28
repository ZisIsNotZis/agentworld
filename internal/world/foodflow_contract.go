package world

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"errors"
	"fmt"
)

// Food-flow v1 has its own component and rule identity. It never reuses the
// finite-cache S6 energy, hunger, stock, or transition schema.
const (
	FoodFlowFormatVersion     uint32                = 1
	FoodFlowSchemaVersion     sim.SchemaVersion     = 1
	FoodFlowRuleVersion       uint32                = 1
	FoodFlowProjectionVersion sim.ProjectionVersion = 1

	FoodFlowActorCount                     = 16
	FoodFlowPatchCount                     = 2
	FoodFlowSlotsPerPatch                  = 8
	FoodFlowHorizonHours                   = 168
	FoodFlowHour              sim.Duration = 3_600_000_000
	FoodFlowInitialEnergy     int64        = 11
	FoodFlowEnergyCapacity    int64        = 12
	FoodFlowHungerCapacity    int64        = 24
	FoodFlowBagCapacity       int64        = 1
	FoodFlowNeverGatheredHour int64        = -1

	FoodFlowPatchTypeID sim.ComponentTypeID = 0x80000010
	FoodFlowSlotTypeID  sim.ComponentTypeID = 0x80000011
	FoodFlowBagTypeID   sim.ComponentTypeID = 0x80000012
	FoodFlowBodyTypeID  sim.ComponentTypeID = 0x80000013

	FoodFlowPatchYieldField         sim.FieldID = 1
	FoodFlowPatchPulsesField        sim.FieldID = 2
	FoodFlowPatchProducedField      sim.FieldID = 3
	FoodFlowPatchUnrealizedField    sim.FieldID = 4
	FoodFlowSlotStockField          sim.FieldID = 1
	FoodFlowSlotPatchField          sim.FieldID = 2
	FoodFlowSlotGatheredField       sim.FieldID = 3
	FoodFlowBagUnitsField           sim.FieldID = 1
	FoodFlowBagSourceField          sim.FieldID = 2
	FoodFlowBodyEnergyField         sim.FieldID = 1
	FoodFlowBodyHungerField         sim.FieldID = 2
	FoodFlowBodyBasalSpentField     sim.FieldID = 3
	FoodFlowBodyCapLostField        sim.FieldID = 4
	FoodFlowBodyConsumedField       sim.FieldID = 5
	FoodFlowBodyLastGatherHourField sim.FieldID = 6

	FoodFlowProduceRule sim.RuleID = 101
	FoodFlowGatherRule  sim.RuleID = 102
	FoodFlowConsumeRule sim.RuleID = 103
	FoodFlowBasalRule   sim.RuleID = 104
)

var ErrFoodFlowContract = errors.New("invalid food-flow v1 contract")

// IDs are fixed, disjoint, and are not allocated by the S6 entity allocator.
func FoodFlowPatchID(index int) (sim.EntityID, error) {
	if index < 0 || index >= FoodFlowPatchCount {
		return 0, ErrFoodFlowContract
	}
	return sim.EntityID(1001 + index), nil
}

func FoodFlowSlotID(patchIndex, slotIndex int) (sim.EntityID, error) {
	if patchIndex < 0 || patchIndex >= FoodFlowPatchCount || slotIndex < 0 || slotIndex >= FoodFlowSlotsPerPatch {
		return 0, ErrFoodFlowContract
	}
	return sim.EntityID(2001 + patchIndex*FoodFlowSlotsPerPatch + slotIndex), nil
}

func FoodFlowActorPatchID(actor sim.EntityID) (sim.EntityID, error) {
	if actor < 1 || actor > FoodFlowActorCount {
		return 0, ErrFoodFlowContract
	}
	return FoodFlowPatchID((int(actor) - 1) / FoodFlowSlotsPerPatch)
}

// At integer h=0, production precedes first claims at h+1 microsecond.
// At h=1..167, basal and production precede that hour's claims; h=168
// is basal only. There are 168 production/claim windows, h=0..167.
func FoodFlowHourTime(h int) (sim.SimTime, error) {
	if h < 0 || h > FoodFlowHorizonHours {
		return 0, ErrFoodFlowContract
	}
	return sim.SimTime(h) * sim.SimTime(FoodFlowHour), nil
}

func FoodFlowClaimTime(h int) (sim.SimTime, error) {
	if h < 0 || h >= FoodFlowHorizonHours {
		return 0, ErrFoodFlowContract
	}
	at, _ := FoodFlowHourTime(h)
	return at + 1, nil
}

func foodFlowBounds(max float64) component.Bounds {
	return component.Bounds{HasMinimum: true, Minimum: 0, HasMaximum: true, Maximum: max}
}
func foodFlowHourBounds() component.Bounds {
	return component.Bounds{HasMinimum: true, Minimum: float64(FoodFlowNeverGatheredHour), HasMaximum: true, Maximum: FoodFlowHorizonHours - 1}
}
func foodFlowInteger(id sim.FieldID, name, unit string, max float64) component.FieldDescriptor {
	return component.FieldDescriptor{ID: id, Name: name, Type: component.ValueType{Kind: sim.IntegerKind}, Unit: unit, Bounds: foodFlowBounds(max), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}
}
func foodFlowProjection(id sim.FieldID, name, unit string, max float64) component.ProjectionDescriptor {
	return component.ProjectionDescriptor{ID: sim.ProjectionID(id), Version: FoodFlowProjectionVersion, Name: name, SourceFields: []sim.FieldID{id}, Coefficients: []float64{1}, Unit: unit, Bounds: foodFlowBounds(max), MissingBehavior: component.PreserveSourceState}
}
func foodFlowRule(id sim.RuleID, name string) component.TransitionRuleDescriptor {
	return component.TransitionRuleDescriptor{ID: id, Version: FoodFlowRuleVersion, Name: name}
}

func FoodFlowPatchDescriptor() component.ComponentDescriptor {
	fields := []component.FieldDescriptor{
		foodFlowInteger(FoodFlowPatchYieldField, "hourly-yield", "food/hour", FoodFlowSlotsPerPatch),
		foodFlowInteger(FoodFlowPatchPulsesField, "production-pulses", "hours", FoodFlowHorizonHours),
		foodFlowInteger(FoodFlowPatchProducedField, "produced", "food", FoodFlowHorizonHours*FoodFlowSlotsPerPatch),
		foodFlowInteger(FoodFlowPatchUnrealizedField, "unrealized-inflow", "food", FoodFlowHorizonHours*FoodFlowSlotsPerPatch),
	}
	projections := make([]component.ProjectionDescriptor, len(fields))
	for i, f := range fields {
		projections[i] = foodFlowProjection(f.ID, f.Name, f.Unit, f.Bounds.Maximum)
	}
	return component.ComponentDescriptor{TypeID: FoodFlowPatchTypeID, SchemaVersion: FoodFlowSchemaVersion, Name: "foodflow-patch-v1", Fields: fields,
		AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: []component.TransitionRuleDescriptor{foodFlowRule(FoodFlowProduceRule, "foodflow-produce-v1")},
		Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func FoodFlowSlotDescriptor() component.ComponentDescriptor {
	return component.ComponentDescriptor{TypeID: FoodFlowSlotTypeID, SchemaVersion: FoodFlowSchemaVersion, Name: "foodflow-slot-v1",
		Fields:       []component.FieldDescriptor{foodFlowInteger(FoodFlowSlotStockField, "stock", "food", 1), {ID: FoodFlowSlotPatchField, Name: "patch", Type: component.ValueType{Kind: sim.EntityRefKind}, Unit: "patch-id", PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden}, foodFlowInteger(FoodFlowSlotGatheredField, "gathered", "food", FoodFlowHorizonHours)},
		AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: []component.TransitionRuleDescriptor{foodFlowRule(FoodFlowProduceRule, "foodflow-produce-v1"), foodFlowRule(FoodFlowGatherRule, "foodflow-gather-v1")},
		Projections:     []component.ProjectionDescriptor{foodFlowProjection(FoodFlowSlotStockField, "slot-stock", "food", 1), foodFlowProjection(FoodFlowSlotGatheredField, "slot-gathered", "food", FoodFlowHorizonHours)},
		MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func FoodFlowBagDescriptor() component.ComponentDescriptor {
	return component.ComponentDescriptor{TypeID: FoodFlowBagTypeID, SchemaVersion: FoodFlowSchemaVersion, Name: "foodflow-bag-v1",
		Fields:       []component.FieldDescriptor{foodFlowInteger(FoodFlowBagUnitsField, "units", "food", 1), {ID: FoodFlowBagSourceField, Name: "source-patch", Type: component.ValueType{Kind: sim.EntityRefKind}, Unit: "patch-id", PermittedStates: []sim.ValueState{sim.Present, sim.Missing}, Uncertainty: component.UncertaintyForbidden}},
		AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: []component.TransitionRuleDescriptor{foodFlowRule(FoodFlowGatherRule, "foodflow-gather-v1"), foodFlowRule(FoodFlowConsumeRule, "foodflow-consume-v1")},
		Projections:     []component.ProjectionDescriptor{foodFlowProjection(FoodFlowBagUnitsField, "held-food", "food", 1)},
		MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

func FoodFlowBodyDescriptor() component.ComponentDescriptor {
	fields := []component.FieldDescriptor{
		foodFlowInteger(FoodFlowBodyEnergyField, "energy", "energy", float64(FoodFlowEnergyCapacity)),
		foodFlowInteger(FoodFlowBodyHungerField, "hunger", "hours", float64(FoodFlowHungerCapacity)),
		foodFlowInteger(FoodFlowBodyBasalSpentField, "basal-spent", "energy", FoodFlowHorizonHours),
		foodFlowInteger(FoodFlowBodyCapLostField, "cap-lost", "energy", FoodFlowHorizonHours),
		foodFlowInteger(FoodFlowBodyConsumedField, "consumed-home-patch", "food", FoodFlowHorizonHours),
		{ID: FoodFlowBodyLastGatherHourField, Name: "last-gather-hour", Type: component.ValueType{Kind: sim.IntegerKind}, Unit: "hour", Bounds: foodFlowHourBounds(), PermittedStates: []sim.ValueState{sim.Present}, Uncertainty: component.UncertaintyForbidden},
	}
	projections := make([]component.ProjectionDescriptor, len(fields))
	for i, f := range fields {
		projections[i] = foodFlowProjection(f.ID, f.Name, f.Unit, f.Bounds.Maximum)
		projections[i].Bounds = f.Bounds
	}
	return component.ComponentDescriptor{TypeID: FoodFlowBodyTypeID, SchemaVersion: FoodFlowSchemaVersion, Name: "foodflow-body-v1", Fields: fields,
		AccessPolicy: component.AccessAuthorizedReadProject, TransitionRules: []component.TransitionRuleDescriptor{foodFlowRule(FoodFlowGatherRule, "foodflow-gather-v1"), foodFlowRule(FoodFlowConsumeRule, "foodflow-consume-v1"), foodFlowRule(FoodFlowBasalRule, "foodflow-basal-v1")},
		Projections: projections, MigrationPolicy: component.MigrationRejectUnlisted, StorageClass: component.DynamicStorage, ComplexityCost: 1}
}

// FoodFlowRegistry is intentionally disjoint from survivalRegistry: a restored
// S6 history cannot satisfy these type/rule/projection identities by accident.
func FoodFlowRegistry() (component.Registry, error) {
	builder := component.NewRegistryBuilder()
	for _, d := range []component.ComponentDescriptor{FoodFlowPatchDescriptor(), FoodFlowSlotDescriptor(), FoodFlowBagDescriptor(), FoodFlowBodyDescriptor()} {
		if err := builder.Register(d); err != nil {
			return component.Registry{}, fmt.Errorf("%w: %v", ErrFoodFlowContract, err)
		}
	}
	return builder.Freeze()
}
