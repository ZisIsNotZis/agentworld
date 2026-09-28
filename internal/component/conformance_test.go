package component

import (
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"sync"
	"testing"
)

type conformanceCase struct {
	name       string
	typeID     sim.ComponentTypeID
	fieldID    sim.FieldID
	projection sim.ProjectionID
	present    float64
}

func TestReadProtocolConformance(t *testing.T) {
	reader, authority := newTestReader(t)
	cases := []conformanceCase{
		{name: "builtin-energy", typeID: EnergyTypeID, fieldID: EnergyReserveField, projection: EnergyProjection, present: 0},
		{name: "dynamic-fatigue", typeID: FatigueTypeID, fieldID: FatigueLevelField, projection: FatigueProjection, present: .5},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { runReadProtocolConformance(t, reader, authority, test) })
	}
}

func runReadProtocolConformance(t *testing.T, reader Reader, authority Authority, test conformanceCase) {
	descriptor, err := reader.Describe(test.typeID)
	if err != nil || descriptor.TypeID != test.typeID {
		t.Fatalf("Describe = %#v, %v", descriptor, err)
	}
	if descriptor.StorageClass != 0 {
		t.Fatalf("Describe leaked storage class %v", descriptor.StorageClass)
	}
	descriptor.Fields[0].Name = "mutated"
	again, _ := reader.Describe(test.typeID)
	if again.Fields[0].Name == "mutated" {
		t.Fatal("Describe returned mutable registry storage")
	}
	has, err := reader.Has(HasRequest{Entity: 1, Component: test.typeID, WorldVersion: 7, Authority: authority})
	if err != nil || !has {
		t.Fatalf("Has present = %t, %v", has, err)
	}
	has, err = reader.Has(HasRequest{Entity: 99, Component: test.typeID, WorldVersion: 7, Authority: authority})
	if err != nil || has {
		t.Fatalf("Has absent = %t, %v", has, err)
	}
	if has, err := reader.Has(HasRequest{Entity: 1, Component: test.typeID, WorldVersion: 7}); !errors.Is(err, ErrUnauthorized) || has {
		t.Fatalf("unauthorized has = %t, %v", has, err)
	}
	if _, err := reader.Read(ReadRequest{Entity: 99, Component: test.typeID, WorldVersion: 7, Authority: authority}); !errors.Is(err, ErrComponentMissing) {
		t.Fatalf("absent read = %v", err)
	}
	if _, err := reader.Read(ReadRequest{Entity: 1, Component: test.typeID, WorldVersion: 7}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized read = %v", err)
	}
	if batch, err := reader.Scan(ScanRequest{Component: test.typeID, WorldVersion: 7}); !errors.Is(err, ErrUnauthorized) || batch.Len() != 0 {
		t.Fatalf("unauthorized scan = %d, %v", batch.Len(), err)
	}
	if metrics, err := reader.Project(ProjectRequest{Component: test.typeID, Projection: test.projection, WorldVersion: 7}); !errors.Is(err, ErrUnauthorized) || metrics.Len() != 0 {
		t.Fatalf("unauthorized projection = %d, %v", metrics.Len(), err)
	}
	if _, err := reader.Read(ReadRequest{Entity: 1, Component: test.typeID, WorldVersion: 8, Authority: authority}); !errors.Is(err, ErrWorldVersionUnavailable) {
		t.Fatalf("unavailable read = %v", err)
	}
	if _, err := reader.Read(ReadRequest{Entity: 1, Component: test.typeID, Fields: []sim.FieldID{999}, WorldVersion: 7, Authority: authority}); !errors.Is(err, ErrUnknownField) {
		t.Fatalf("unknown field = %v", err)
	}
	if _, err := reader.Project(ProjectRequest{Component: test.typeID, Projection: 999, WorldVersion: 7, Authority: authority}); !errors.Is(err, ErrUnknownProjection) {
		t.Fatalf("unknown projection = %v", err)
	}
	view, err := reader.Read(ReadRequest{Entity: 1, Component: test.typeID, Fields: []sim.FieldID{test.fieldID, test.fieldID}, WorldVersion: 7, Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(view.FieldIDs(), []sim.FieldID{test.fieldID}) {
		t.Fatalf("selector = %v", view.FieldIDs())
	}
	value, _ := view.Value(test.fieldID)
	got, _ := value.Scalar()
	if value.State() != sim.Present || got != test.present {
		t.Fatalf("present value = %v/%g", value.State(), got)
	}
	states := []sim.ValueState{sim.Present, sim.Missing, sim.Unknown, sim.Inapplicable}
	for index, state := range states {
		view, err := reader.Read(ReadRequest{Entity: sim.EntityID(index + 1), Component: test.typeID, WorldVersion: 7, Authority: authority})
		if err != nil {
			t.Fatal(err)
		}
		value, _ := view.Value(test.fieldID)
		if value.State() != state || value.Kind() != sim.ScalarKind {
			t.Fatalf("entity %d state/kind = %v/%v", index+1, value.State(), value.Kind())
		}
	}
	batch, err := reader.Scan(ScanRequest{Component: test.typeID, WorldVersion: 7, Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batch.EntityIDs(), []sim.EntityID{1, 2, 3, 4}) {
		t.Fatalf("scan order = %v", batch.EntityIDs())
	}
	values := make([]float64, batch.Len())
	copiedStates := make([]sim.ValueState, batch.Len())
	n, err := batch.CopyScalarColumn(test.fieldID, values, copiedStates)
	if err != nil || n != 4 || !reflect.DeepEqual(copiedStates, states) {
		t.Fatalf("scalar column = %d, %v, %v", n, copiedStates, err)
	}
	metrics, err := reader.Project(ProjectRequest{Component: test.typeID, Projection: test.projection, Entities: []sim.EntityID{4, 1, 1, 3, 2}, WorldVersion: 7, Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(metrics.EntityIDs(), []sim.EntityID{1, 2, 3, 4}) {
		t.Fatalf("metric order = %v", metrics.EntityIDs())
	}
	for i, state := range states {
		metric, err := metrics.At(i)
		if err != nil {
			t.Fatal(err)
		}
		if metric.Value().State() != state || metric.Uncertainty().State() != sim.Unknown || metric.ProjectionID() != test.projection || metric.ProjectionVersion() != 1 || metric.Unit() == "" {
			t.Fatalf("metric %d incomplete: %#v", i, metric)
		}
		provenance := metric.Provenance()
		if provenance.Entity != sim.EntityID(i+1) || provenance.Component != test.typeID || provenance.WorldVersion != 7 || provenance.SchemaVersion != 1 || !reflect.DeepEqual(provenance.SourceFields, []sim.FieldID{test.fieldID}) {
			t.Fatalf("provenance = %#v", provenance)
		}
	}
}

func TestAuthorityIsBoundToItsSnapshot(t *testing.T) {
	first, firstAuthority := newTestReader(t)
	_, secondAuthority := newTestReader(t)
	if has, err := first.Has(HasRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: 7, Authority: secondAuthority}); !errors.Is(err, ErrUnauthorized) || has {
		t.Fatalf("cross-snapshot has = %t, %v", has, err)
	}
	if _, err := first.Read(ReadRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: 7, Authority: secondAuthority}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("cross-snapshot authority = %v", err)
	}
	if _, err := first.Read(ReadRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: 7, Authority: firstAuthority}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentImmutableReadProtocol(t *testing.T) {
	reader, authority := newTestReader(t)
	var wait sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := 0; i < 100; i++ {
				if _, err := reader.Read(ReadRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: 7, Authority: authority}); err != nil {
					t.Error(err)
					return
				}
				if _, err := reader.Scan(ScanRequest{Component: FatigueTypeID, WorldVersion: 7, Authority: authority}); err != nil {
					t.Error(err)
					return
				}
				if _, err := reader.Project(ProjectRequest{Component: EnergyTypeID, Projection: EnergyProjection, WorldVersion: 7, Authority: authority}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wait.Wait()
}
