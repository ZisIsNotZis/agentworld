package component_test

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"testing"
)

func TestConsumerUsesOnlyReaderAndImmutableResults(t *testing.T) {
	builder := component.NewRegistryBuilder()
	if err := builder.Register(component.EnergyDescriptor()); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	zero, _ := sim.ScalarValue(0)
	reader, authority, err := component.NewReader(registry, 0, []component.ComponentSeed{{Entity: 1, Component: component.EnergyTypeID, Fields: []component.FieldSeed{{Field: component.EnergyReserveField, Value: zero}}}})
	if err != nil {
		t.Fatal(err)
	}
	var consumer component.Reader = reader
	view, err := consumer.Read(component.ReadRequest{Entity: 1, Component: component.EnergyTypeID, WorldVersion: 0, Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	value, err := view.Value(component.EnergyReserveField)
	if err != nil || !value.IsPresent() {
		t.Fatalf("value = %#v, %v", value, err)
	}
}
