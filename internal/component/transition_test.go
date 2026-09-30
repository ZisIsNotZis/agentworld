package component

import (
	"agentworld/internal/sim"
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func allocationSeeds(t *testing.T, entity sim.EntityID) []ComponentSeed {
	t.Helper()
	return []ComponentSeed{
		{Entity: entity, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustScalarValue(t, .25)}}},
		{Entity: entity, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: mustAbsentValue(t, sim.Missing)}}},
	}
}

func TestStageAllocationCreatesCompleteRowsWithAbsentBeforeDeltas(t *testing.T) {
	reader, authority := newTestReader(t)
	parent := []Patch{{Entity: 1, Component: EnergyTypeID, SchemaVersion: 1, Field: EnergyReserveField, Value: mustScalarValue(t, .75)}}
	next, nextAuthority, deltas, metrics, err := Stage(reader, authority, 7, 1, 1, parent, allocationSeeds(t, 100))
	if err != nil {
		t.Fatal(err)
	}
	// Newborn rows lead in canonical (entity, component, field) order, each
	// with an absent-before value; the parent patch delta follows.
	for i := range 2 {
		if deltas[i].Entity != 100 || deltas[i].Before.Kind() != sim.ScalarKind || deltas[i].Before.State() != sim.Missing || !deltas[i].Before.Equal(mustAbsentValue(t, sim.Missing)) {
			t.Fatalf("allocation delta %d absent-before: %+v", i, deltas[i])
		}
	}
	if deltas[0].Component != EnergyTypeID || !deltas[0].After.Equal(mustScalarValue(t, .25)) || deltas[1].Component != FatigueTypeID || !deltas[1].After.Equal(mustAbsentValue(t, sim.Missing)) {
		t.Fatalf("newborn rows wrong: %+v", deltas[:2])
	}
	if deltas[2].Entity != 1 || !deltas[2].After.Equal(mustScalarValue(t, .75)) {
		t.Fatalf("patch delta not last: %+v", deltas[2])
	}
	if len(metrics) != 1 || metrics[0].Before.Provenance().Entity != 1 {
		t.Fatalf("newborn rows must not carry metrics: %+v", metrics)
	}
	// One version bump covers allocation and patches together; the input
	// snapshot is untouched.
	if _, _, _, _, err := Stage(reader, authority, 8, 1, 1, parent, nil); !errors.Is(err, ErrWorldVersionUnavailable) {
		t.Fatalf("staging did not bump to one next version: %v", err)
	}
	if has, err := reader.Has(HasRequest{Entity: 100, Component: EnergyTypeID, WorldVersion: 7, Authority: authority}); err != nil || has {
		t.Fatalf("input snapshot mutated: %v %v", has, err)
	}
	view, err := reader.Read(ReadRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: 7, Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := view.Value(EnergyReserveField); !value.Equal(mustScalarValue(t, 0)) {
		t.Fatalf("input parent row mutated: %v", value)
	}
	newborn, err := next.Read(ReadRequest{Entity: 100, Component: FatigueTypeID, WorldVersion: 8, Authority: nextAuthority})
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := newborn.Value(FatigueLevelField); !value.Equal(mustAbsentValue(t, sim.Missing)) {
		t.Fatalf("newborn fatigue row wrong: %v", value)
	}
	registry := testRegistry(t, EnergyDescriptor())
	encoded, err := EncodeSnapshot(registry, next, nextAuthority, 8)
	if err != nil {
		t.Fatalf("staged snapshot with newborn rows not encodable: %v", err)
	}
	if _, _, _, err := DecodeSnapshot(registry, encoded); err != nil {
		t.Fatalf("newborn rows did not survive snapshot round trip: %v", err)
	}
}

func TestStageAllocationRejectsInvalidSeedsWithoutMutation(t *testing.T) {
	reader, authority := newTestReader(t)
	parent := []Patch{{Entity: 1, Component: EnergyTypeID, SchemaVersion: 1, Field: EnergyReserveField, Value: mustScalarValue(t, .5)}}
	cases := []struct {
		name        string
		patches     []Patch
		allocations []ComponentSeed
		want        error
	}{
		{"existing row same component", parent, []ComponentSeed{{Entity: 1, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustScalarValue(t, .25)}}}}, ErrEntityExists},
		{"existing row other component", parent, []ComponentSeed{{Entity: 1, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: mustScalarValue(t, .25)}}}}, ErrEntityExists},
		{"partial row", parent, []ComponentSeed{{Entity: 100, Component: EnergyTypeID}}, ErrPartialAllocation},
		{"cross-seed duplicate", parent, []ComponentSeed{allocationSeeds(t, 100)[0], {Entity: 100, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustScalarValue(t, .5)}}}}, ErrDuplicateAllocation},
		{"patch on allocated entity", []Patch{{Entity: 100, Component: EnergyTypeID, SchemaVersion: 1, Field: EnergyReserveField, Value: mustScalarValue(t, .5)}}, allocationSeeds(t, 100), ErrAllocationOverlap},
		{"seed outside bounds", parent, []ComponentSeed{{Entity: 100, Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: mustScalarValue(t, 2)}}}}, ErrValueOutsideSchema},
		{"unknown rule", parent, allocationSeeds(t, 100), ErrUnknownRule},
		{"zero entity", parent, []ComponentSeed{{Entity: 0, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustScalarValue(t, .25)}}}}, ErrInvalidRequest},
		{"unknown field", parent, []ComponentSeed{{Entity: 100, Component: EnergyTypeID, Fields: []FieldSeed{{Field: 99, Value: mustScalarValue(t, .25)}}}}, ErrUnknownField},
		{"unknown component", parent, []ComponentSeed{{Entity: 100, Component: 99, Fields: []FieldSeed{{Field: 1, Value: mustScalarValue(t, .25)}}}}, ErrUnknownComponentType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := sim.RuleID(1)
			if tc.name == "unknown rule" {
				rule = 99
			}
			if _, _, _, _, err := Stage(reader, authority, 7, rule, 1, tc.patches, tc.allocations); !errors.Is(err, tc.want) {
				t.Fatalf("staging error: %v", err)
			}
			if has, err := reader.Has(HasRequest{Entity: 100, Component: EnergyTypeID, WorldVersion: 7, Authority: authority}); err != nil || has {
				t.Fatalf("rejected allocation mutated membership: %v %v", has, err)
			}
			view, err := reader.Read(ReadRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: 7, Authority: authority})
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := view.Value(EnergyReserveField); !value.Equal(mustScalarValue(t, 0)) {
				t.Fatalf("rejected allocation mutated state: %v", value)
			}
		})
	}
	// A permitted absent state seeds a valid newborn row (control).
	next, nextAuthority, deltas, _, err := Stage(reader, authority, 7, 1, 1, parent, []ComponentSeed{{Entity: 100, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustAbsentValue(t, sim.Inapplicable)}}}})
	if err != nil || len(deltas) != 2 || !deltas[0].After.Equal(mustAbsentValue(t, sim.Inapplicable)) {
		t.Fatalf("permitted absent state rejected: %v %+v", err, deltas)
	}
	if _, err := next.Read(ReadRequest{Entity: 100, Component: EnergyTypeID, WorldVersion: 8, Authority: nextAuthority}); err != nil {
		t.Fatalf("newborn row unreadable: %v", err)
	}
}

func TestStageAllocationKeepsEntityOrderAscending(t *testing.T) {
	energyDynamic := EnergyDescriptor()
	energyDynamic.StorageClass = DynamicStorage
	for _, registry := range []Registry{singleRegistry(t, EnergyDescriptor()), singleRegistry(t, energyDynamic), singleRegistry(t, FatigueDescriptor())} {
		id := registry.TypeIDs()[0]
		descriptor, _ := registry.Describe(id)
		field := descriptor.Fields[0].ID
		seeds := []ComponentSeed{
			{Entity: 2, Component: id, Fields: []FieldSeed{{Field: field, Value: mustScalarValue(t, .2)}}},
			{Entity: 4, Component: id, Fields: []FieldSeed{{Field: field, Value: mustScalarValue(t, .4)}}},
		}
		reader, authority, err := NewReader(registry, 3, seeds)
		if err != nil {
			t.Fatal(err)
		}
		patch := []Patch{{Entity: 2, Component: id, SchemaVersion: descriptor.SchemaVersion, Field: field, Value: mustScalarValue(t, .9)}}
		// Insert below the minimum, between rows, and above the maximum.
		allocations := []ComponentSeed{
			{Entity: 3, Component: id, Fields: []FieldSeed{{Field: field, Value: mustScalarValue(t, .3)}}},
			{Entity: 5, Component: id, Fields: []FieldSeed{{Field: field, Value: mustScalarValue(t, .5)}}},
			{Entity: 1, Component: id, Fields: []FieldSeed{{Field: field, Value: mustScalarValue(t, .1)}}},
		}
		next, nextAuthority, deltas, _, err := Stage(reader, authority, 3, 1, 1, patch, allocations)
		if err != nil {
			t.Fatal(err)
		}
		if len(deltas) != 4 || deltas[0].Entity != 1 || deltas[1].Entity != 3 || deltas[2].Entity != 5 || deltas[3].Entity != 2 {
			t.Fatalf("allocation deltas not in canonical order: %+v", deltas)
		}
		batch, err := next.Scan(ScanRequest{Component: id, WorldVersion: 4, Authority: nextAuthority})
		if err != nil {
			t.Fatal(err)
		}
		if got := batch.EntityIDs(); !reflect.DeepEqual(got, []sim.EntityID{1, 2, 3, 4, 5}) {
			t.Fatalf("entities not ascending after allocation: %v", got)
		}
		encoded, err := EncodeSnapshot(registry, next, nextAuthority, 4)
		if err != nil {
			t.Fatalf("allocated rows broke snapshot encoding: %v", err)
		}
		restored, restoredAuthority, version, err := DecodeSnapshot(registry, encoded)
		if err != nil {
			t.Fatal(err)
		}
		again, err := EncodeSnapshot(registry, restored, restoredAuthority, version)
		if err != nil || !bytes.Equal(again, encoded) {
			t.Fatalf("allocated rows unstable across restore: %v", err)
		}
		// The input reader must have kept its original two rows only.
		before, err := reader.Scan(ScanRequest{Component: id, WorldVersion: 3, Authority: authority})
		if err != nil {
			t.Fatal(err)
		}
		if got := before.EntityIDs(); !reflect.DeepEqual(got, []sim.EntityID{2, 4}) {
			t.Fatalf("input store mutated: %v", got)
		}
	}
}
