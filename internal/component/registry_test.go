package component

import (
	"agentworld/internal/sim"
	"errors"
	"math"
	"testing"
)

func TestRegistryRejectsInvalidDescriptors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ComponentDescriptor)
	}{
		{"zero type", func(d *ComponentDescriptor) { d.TypeID = 0 }},
		{"empty name", func(d *ComponentDescriptor) { d.Name = "" }},
		{"empty field unit", func(d *ComponentDescriptor) { d.Fields[0].Unit = "" }},
		{"zero schema", func(d *ComponentDescriptor) { d.SchemaVersion = 0 }},
		{"duplicate field", func(d *ComponentDescriptor) { d.Fields = append(d.Fields, d.Fields[0]) }},
		{"duplicate nested record field name", func(d *ComponentDescriptor) {
			d.Fields = append(d.Fields, FieldDescriptor{ID: 2, Name: "metadata", Unit: "dimensionless", PermittedStates: allStates(), Uncertainty: UncertaintyForbidden, Type: ValueType{Kind: sim.RecordKind, Fields: []ValueFieldType{{ID: 1, Name: "duplicate", Type: ValueType{Kind: sim.BoolKind}}, {ID: 2, Name: "duplicate", Type: ValueType{Kind: sim.IntegerKind}}}}})
		}},
		{"invalid bounds", func(d *ComponentDescriptor) {
			d.Fields[0].Bounds = Bounds{HasMinimum: true, Minimum: 2, HasMaximum: true, Maximum: 1}
		}},
		{"nonfinite projection", func(d *ComponentDescriptor) { d.Projections[0].Intercept = math.NaN() }},
		{"unknown projection field", func(d *ComponentDescriptor) { d.Projections[0].SourceFields[0] = 99 }},
		{"missing access", func(d *ComponentDescriptor) { d.AccessPolicy = AccessUnspecified }},
		{"missing transition", func(d *ComponentDescriptor) { d.TransitionRules = nil }},
		{"missing projection", func(d *ComponentDescriptor) { d.Projections = nil }},
		{"missing migration policy", func(d *ComponentDescriptor) { d.MigrationPolicy = MigrationUnspecified }},
		{"incompatible migration", func(d *ComponentDescriptor) {
			d.MigrationPolicy = MigrationExplicitPaths
			d.MigrationPaths = []MigrationPath{{From: 2, To: 1}}
		}},
		{"missing complexity", func(d *ComponentDescriptor) { d.ComplexityCost = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := FatigueDescriptor()
			test.mutate(&descriptor)
			if err := NewRegistryBuilder().Register(descriptor); err == nil {
				t.Fatal("invalid descriptor accepted")
			}
		})
	}
}

func TestRegistryDuplicatesDependenciesAndImmutability(t *testing.T) {
	builder := NewRegistryBuilder()
	energy := EnergyDescriptor()
	if err := builder.Register(energy); err != nil {
		t.Fatal(err)
	}
	if err := builder.Register(energy); !errors.Is(err, ErrDuplicateDescriptor) {
		t.Fatalf("duplicate = %v", err)
	}
	fatigue := FatigueDescriptor()
	fatigue.Dependencies = []sim.ComponentTypeID{EnergyTypeID}
	if err := builder.Register(fatigue); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	energy.Fields[0].Name = "changed"
	first, _ := registry.Describe(EnergyTypeID)
	first.Fields[0].Name = "also-changed"
	second, _ := registry.Describe(EnergyTypeID)
	if second.Fields[0].Name != "reserve" {
		t.Fatal("registry descriptor was mutable")
	}
}

func TestSeedValidationRejectsOutOfBoundsAndWrongKinds(t *testing.T) {
	registry := singleRegistry(t, FatigueDescriptor())
	tooHigh, _ := sim.ScalarValue(2)
	if _, _, err := NewReader(registry, 0, []ComponentSeed{{Entity: 1, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: tooHigh}}}}); !errors.Is(err, ErrValueOutsideSchema) {
		t.Fatalf("out of bounds seed = %v", err)
	}
	if _, _, err := NewReader(registry, 0, []ComponentSeed{{Entity: 1, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: sim.BoolValue(true)}}}}); !errors.Is(err, ErrValueOutsideSchema) {
		t.Fatalf("wrong-kind seed = %v", err)
	}
}

func TestRegistryRejectsUnknownDependencyAndCycle(t *testing.T) {
	builder := NewRegistryBuilder()
	energy := EnergyDescriptor()
	energy.Dependencies = []sim.ComponentTypeID{FatigueTypeID}
	if err := builder.Register(energy); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Freeze(); !errors.Is(err, ErrUnknownComponentType) {
		t.Fatalf("unknown dependency = %v", err)
	}
	builder = NewRegistryBuilder()
	energy = EnergyDescriptor()
	energy.Dependencies = []sim.ComponentTypeID{FatigueTypeID}
	fatigue := FatigueDescriptor()
	fatigue.Dependencies = []sim.ComponentTypeID{EnergyTypeID}
	if err := builder.Register(energy); err != nil {
		t.Fatal(err)
	}
	if err := builder.Register(fatigue); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Freeze(); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle = %v", err)
	}
}
