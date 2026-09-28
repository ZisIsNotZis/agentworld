package component

import (
	"agentworld/internal/sim"
	"testing"
)

func mustAbsentValue(t testing.TB, state sim.ValueState) sim.Value {
	t.Helper()
	value, err := sim.AbsentValue(sim.ScalarKind, state)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustScalarValue(t testing.TB, scalar float64) sim.Value {
	t.Helper()
	value, err := sim.ScalarValue(scalar)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func testRegistry(t testing.TB, energy ComponentDescriptor) Registry {
	t.Helper()
	builder := NewRegistryBuilder()
	if err := builder.Register(energy); err != nil {
		t.Fatal(err)
	}
	if err := builder.Register(FatigueDescriptor()); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func testSeeds(t testing.TB) []ComponentSeed {
	t.Helper()
	unknown := mustAbsentValue(t, sim.Unknown)
	inapplicable := mustAbsentValue(t, sim.Inapplicable)
	zero := mustScalarValue(t, 0)
	half := mustScalarValue(t, .5)
	return []ComponentSeed{
		{Entity: 4, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: inapplicable}}},
		{Entity: 2, Component: EnergyTypeID},
		{Entity: 1, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: zero}}},
		{Entity: 3, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: unknown}}},
		{Entity: 4, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: inapplicable}}},
		{Entity: 2, Component: FatigueTypeID},
		{Entity: 1, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: half}}},
		{Entity: 3, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: unknown}}},
	}
}

func newTestReader(t testing.TB) (Reader, Authority) {
	t.Helper()
	reader, authority, err := NewReader(testRegistry(t, EnergyDescriptor()), 7, testSeeds(t))
	if err != nil {
		t.Fatal(err)
	}
	return reader, authority
}
