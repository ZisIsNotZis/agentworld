package component

import "agentworld/internal/sim"

const (
	EnergyTypeID       sim.ComponentTypeID = 1
	EnergyReserveField sim.FieldID         = 1
	EnergyProjection   sim.ProjectionID    = 1
	FatigueTypeID      sim.ComponentTypeID = 0x80000001
	FatigueLevelField  sim.FieldID         = 1
	FatigueProjection  sim.ProjectionID    = 1
	CacheStockTypeID   sim.ComponentTypeID = 0x80000002
	CacheStockField    sim.FieldID         = 1
	CacheProjection    sim.ProjectionID    = 1
	WithdrawRuleID     sim.RuleID          = 2
)

type StorageClass uint8

const (
	BuiltinStorage StorageClass = iota + 1
	DynamicStorage
)

type AccessPolicy uint8

const (
	AccessUnspecified AccessPolicy = iota
	AccessAuthorizedReadProject
)

type UncertaintyPolicy uint8

const (
	UncertaintyForbidden UncertaintyPolicy = iota + 1
	UncertaintyPermitted
)

type MigrationPolicy uint8

const (
	MigrationUnspecified MigrationPolicy = iota
	MigrationRejectUnlisted
	MigrationExplicitPaths
)

type MissingBehavior uint8

const (
	PreserveSourceState MissingBehavior = iota + 1
)

type ValueType struct {
	Kind    sim.Kind
	Element *ValueType
	Key     *ValueType
	Fields  []ValueFieldType
}

type ValueFieldType struct {
	ID   sim.FieldID
	Name string
	Type ValueType
}

type Bounds struct {
	HasMinimum bool
	Minimum    float64
	HasMaximum bool
	Maximum    float64
}

type FieldDescriptor struct {
	ID              sim.FieldID
	Name            string
	Type            ValueType
	Unit            string
	Bounds          Bounds
	PermittedStates []sim.ValueState
	Uncertainty     UncertaintyPolicy
}

type TransitionRuleDescriptor struct {
	ID      sim.RuleID
	Version uint32
	Name    string
}

type ProjectionDescriptor struct {
	ID              sim.ProjectionID
	Version         sim.ProjectionVersion
	Name            string
	SourceFields    []sim.FieldID
	Coefficients    []float64
	Intercept       float64
	Unit            string
	Bounds          Bounds
	MissingBehavior MissingBehavior
}

type MigrationPath struct {
	From sim.SchemaVersion
	To   sim.SchemaVersion
}

type ComponentDescriptor struct {
	TypeID          sim.ComponentTypeID
	SchemaVersion   sim.SchemaVersion
	Name            string
	Fields          []FieldDescriptor
	AccessPolicy    AccessPolicy
	TransitionRules []TransitionRuleDescriptor
	Projections     []ProjectionDescriptor
	Dependencies    []sim.ComponentTypeID
	MigrationPaths  []MigrationPath
	MigrationPolicy MigrationPolicy
	StorageClass    StorageClass
	ComplexityCost  uint32
}

func EnergyDescriptor() ComponentDescriptor {
	return ComponentDescriptor{
		TypeID: EnergyTypeID, SchemaVersion: 1, Name: "energy",
		Fields: []FieldDescriptor{{
			ID: EnergyReserveField, Name: "reserve", Type: ValueType{Kind: sim.ScalarKind}, Unit: "joules",
			Bounds: Bounds{HasMinimum: true, Minimum: 0}, PermittedStates: allStates(), Uncertainty: UncertaintyPermitted,
		}},
		AccessPolicy:    AccessAuthorizedReadProject,
		TransitionRules: []TransitionRuleDescriptor{{ID: 1, Version: 1, Name: "energy-transition-v1"}, {ID: WithdrawRuleID, Version: 1, Name: "withdraw-energy-v1"}},
		Projections: []ProjectionDescriptor{{
			ID: EnergyProjection, Version: 1, Name: "energy-reserve", SourceFields: []sim.FieldID{EnergyReserveField},
			Coefficients: []float64{1}, Unit: "joules", Bounds: Bounds{HasMinimum: true, Minimum: 0}, MissingBehavior: PreserveSourceState,
		}},
		MigrationPolicy: MigrationRejectUnlisted, StorageClass: BuiltinStorage, ComplexityCost: 1,
	}
}

func FatigueDescriptor() ComponentDescriptor {
	return ComponentDescriptor{
		TypeID: FatigueTypeID, SchemaVersion: 1, Name: "fatigue",
		Fields: []FieldDescriptor{{
			ID: FatigueLevelField, Name: "level", Type: ValueType{Kind: sim.ScalarKind}, Unit: "ratio",
			Bounds: Bounds{HasMinimum: true, Minimum: 0, HasMaximum: true, Maximum: 1}, PermittedStates: allStates(), Uncertainty: UncertaintyPermitted,
		}},
		AccessPolicy:    AccessAuthorizedReadProject,
		TransitionRules: []TransitionRuleDescriptor{{ID: 1, Version: 1, Name: "fatigue-transition-v1"}},
		Projections: []ProjectionDescriptor{{
			ID: FatigueProjection, Version: 1, Name: "fatigue-level", SourceFields: []sim.FieldID{FatigueLevelField},
			Coefficients: []float64{1}, Unit: "ratio", Bounds: Bounds{HasMinimum: true, Minimum: 0, HasMaximum: true, Maximum: 1}, MissingBehavior: PreserveSourceState,
		}},
		MigrationPolicy: MigrationRejectUnlisted, StorageClass: DynamicStorage, ComplexityCost: 1,
	}
}

// CacheStockDescriptor is dynamic; the one-unit withdrawal rule also appears
// on EnergyDescriptor so both fields can be committed in one kernel event.
func CacheStockDescriptor() ComponentDescriptor {
	return ComponentDescriptor{
		TypeID: CacheStockTypeID, SchemaVersion: 1, Name: "cache-stock",
		Fields: []FieldDescriptor{{
			ID: CacheStockField, Name: "stock", Type: ValueType{Kind: sim.ScalarKind}, Unit: "joules",
			Bounds: Bounds{HasMinimum: true, Minimum: 0}, PermittedStates: allStates(), Uncertainty: UncertaintyPermitted,
		}},
		AccessPolicy:    AccessAuthorizedReadProject,
		TransitionRules: []TransitionRuleDescriptor{{ID: WithdrawRuleID, Version: 1, Name: "withdraw-energy-v1"}},
		Projections: []ProjectionDescriptor{{
			ID: CacheProjection, Version: 1, Name: "cache-stock", SourceFields: []sim.FieldID{CacheStockField},
			Coefficients: []float64{1}, Unit: "joules", Bounds: Bounds{HasMinimum: true, Minimum: 0}, MissingBehavior: PreserveSourceState,
		}},
		MigrationPolicy: MigrationRejectUnlisted, StorageClass: DynamicStorage, ComplexityCost: 1,
	}
}

func allStates() []sim.ValueState {
	return []sim.ValueState{sim.Present, sim.Missing, sim.Unknown, sim.Inapplicable}
}
