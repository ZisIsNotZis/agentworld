package component

import (
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

func TestDynamicAndBuiltinEnergyReadProtocolEquivalence(t *testing.T) {
	builtinDescriptor := EnergyDescriptor()
	dynamicDescriptor := EnergyDescriptor()
	dynamicDescriptor.StorageClass = DynamicStorage
	builtinRegistry := singleRegistry(t, builtinDescriptor)
	dynamicRegistry := singleRegistry(t, dynamicDescriptor)
	var seeds []ComponentSeed
	for _, seed := range testSeeds(t) {
		if seed.Component == EnergyTypeID {
			seeds = append(seeds, seed)
		}
	}
	builtin, builtinAuth, err := NewReader(builtinRegistry, 11, seeds)
	if err != nil {
		t.Fatal(err)
	}
	dynamic, dynamicAuth, err := NewReader(dynamicRegistry, 11, seeds)
	if err != nil {
		t.Fatal(err)
	}
	builtinLogical, err := builtin.Describe(EnergyTypeID)
	if err != nil {
		t.Fatal(err)
	}
	dynamicLogical, err := dynamic.Describe(EnergyTypeID)
	if err != nil {
		t.Fatal(err)
	}
	if builtinLogical.StorageClass != 0 || dynamicLogical.StorageClass != 0 {
		t.Fatalf("Describe leaked storage classes: %v/%v", builtinLogical.StorageClass, dynamicLogical.StorageClass)
	}
	if !reflect.DeepEqual(builtinLogical, dynamicLogical) {
		t.Fatal("logical descriptors differ across stores")
	}
	for _, entity := range []sim.EntityID{1, 2, 3, 4, 99} {
		left, leftErr := builtin.Read(ReadRequest{Entity: entity, Component: EnergyTypeID, WorldVersion: 11, Authority: builtinAuth})
		right, rightErr := dynamic.Read(ReadRequest{Entity: entity, Component: EnergyTypeID, WorldVersion: 11, Authority: dynamicAuth})
		if !sameError(leftErr, rightErr) {
			t.Fatalf("entity %d errors = %v/%v", entity, leftErr, rightErr)
		}
		if leftErr == nil {
			leftValue, _ := left.Value(EnergyReserveField)
			rightValue, _ := right.Value(EnergyReserveField)
			if !leftValue.Equal(rightValue) {
				t.Fatalf("entity %d values differ", entity)
			}
		}
	}
	leftBatch, _ := builtin.Scan(ScanRequest{Component: EnergyTypeID, WorldVersion: 11, Authority: builtinAuth})
	rightBatch, _ := dynamic.Scan(ScanRequest{Component: EnergyTypeID, WorldVersion: 11, Authority: dynamicAuth})
	if !reflect.DeepEqual(leftBatch.EntityIDs(), rightBatch.EntityIDs()) {
		t.Fatal("batch entities differ")
	}
	for row := 0; row < leftBatch.Len(); row++ {
		left, _ := leftBatch.Value(row, EnergyReserveField)
		right, _ := rightBatch.Value(row, EnergyReserveField)
		if !left.Equal(right) {
			t.Fatalf("batch row %d differs", row)
		}
	}
	leftMetrics, _ := builtin.Project(ProjectRequest{Component: EnergyTypeID, Projection: EnergyProjection, WorldVersion: 11, Authority: builtinAuth})
	rightMetrics, _ := dynamic.Project(ProjectRequest{Component: EnergyTypeID, Projection: EnergyProjection, WorldVersion: 11, Authority: dynamicAuth})
	if !reflect.DeepEqual(leftMetrics.EntityIDs(), rightMetrics.EntityIDs()) {
		t.Fatal("metric entities differ")
	}
	for i := 0; i < leftMetrics.Len(); i++ {
		left, _ := leftMetrics.At(i)
		right, _ := rightMetrics.At(i)
		if !left.Value().Equal(right.Value()) || !left.Uncertainty().Equal(right.Uncertainty()) || !reflect.DeepEqual(left.Provenance(), right.Provenance()) {
			t.Fatalf("metric %d differs", i)
		}
	}
}

func singleRegistry(t testing.TB, descriptor ComponentDescriptor) Registry {
	t.Helper()
	builder := NewRegistryBuilder()
	if err := builder.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}
func sameError(left, right error) bool {
	for _, target := range []error{ErrComponentMissing, ErrUnauthorized, ErrWorldVersionUnavailable, ErrUnknownField} {
		if errors.Is(left, target) || errors.Is(right, target) {
			return errors.Is(left, target) == errors.Is(right, target)
		}
	}
	return left == nil && right == nil
}
