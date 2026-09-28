package component

import (
	"agentworld/internal/sim"
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestSnapshotRoundTripAndStorageEquivalence(t *testing.T) {
	builtin := testRegistry(t, EnergyDescriptor())
	energyDynamic := EnergyDescriptor()
	energyDynamic.StorageClass = DynamicStorage
	dynamic := testRegistry(t, energyDynamic)
	leftHash, err := RegistryFingerprint(builtin)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := RegistryFingerprint(dynamic)
	if err != nil || leftHash != rightHash {
		t.Fatalf("storage changed logical fingerprint: %x/%x %v", leftHash, rightHash, err)
	}
	var original []byte
	for _, registry := range []Registry{builtin, dynamic} {
		reader, authority, err := NewReader(registry, 7, testSeeds(t))
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := EncodeSnapshot(registry, reader, authority, 7)
		if err != nil {
			t.Fatal(err)
		}
		if original != nil && !bytes.Equal(original, encoded) {
			t.Fatal("built-in and dynamic produced different snapshot bytes")
		}
		original = encoded
		restored, restoredAuth, version, err := DecodeSnapshot(registry, encoded)
		if err != nil || version != 7 {
			t.Fatalf("restore = %d, %v", version, err)
		}
		if _, err := restored.Read(ReadRequest{Entity: 1, Component: EnergyTypeID, WorldVersion: version, Authority: authority}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("restored authority was not fresh: %v", err)
		}
		again, err := EncodeSnapshot(registry, restored, restoredAuth, version)
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("unstable snapshot after restore: %v", err)
		}
		for _, id := range registry.TypeIDs() {
			old, err := reader.Scan(ScanRequest{Component: id, WorldVersion: 7, Authority: authority})
			if err != nil {
				t.Fatal(err)
			}
			got, err := restored.Scan(ScanRequest{Component: id, WorldVersion: 7, Authority: restoredAuth})
			if err != nil || !reflect.DeepEqual(old.EntityIDs(), got.EntityIDs()) || !reflect.DeepEqual(old.FieldIDs(), got.FieldIDs()) {
				t.Fatalf("scan metadata %d: %v", id, err)
			}
			for row := 0; row < old.Len(); row++ {
				for _, field := range old.FieldIDs() {
					before, _ := old.Value(row, field)
					after, _ := got.Value(row, field)
					if !before.Equal(after) || before.State() != sim.ValueState(row) {
						t.Fatalf("component %d row %d field %d: states %d/%d", id, row, field, before.State(), after.State())
					}
				}
			}
			d, _ := registry.Describe(id)
			for _, p := range d.Projections {
				oldMetrics, err := reader.Project(ProjectRequest{Component: id, Projection: p.ID, WorldVersion: 7, Authority: authority})
				if err != nil {
					t.Fatal(err)
				}
				newMetrics, err := restored.Project(ProjectRequest{Component: id, Projection: p.ID, WorldVersion: 7, Authority: restoredAuth})
				if err != nil || !reflect.DeepEqual(oldMetrics, newMetrics) {
					t.Fatalf("projection differs after restore: %v", err)
				}
			}
		}
	}
	// The very same serialized state can be loaded under the other physical class.
	restored, auth, version, err := DecodeSnapshot(dynamic, original)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := EncodeSnapshot(dynamic, restored, auth, version); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("cross-class restore differs: %v", err)
	}
	builtinRestored, builtinAuth, _, err := DecodeSnapshot(builtin, original)
	if err != nil {
		t.Fatal(err)
	}
	patch := []Patch{{Entity: 1, Component: EnergyTypeID, SchemaVersion: 1, Field: EnergyReserveField, Value: mustScalarValue(t, 3)}}
	left, leftAuth, leftDeltas, leftMetrics, err := Stage(builtinRestored, builtinAuth, version, 1, 1, patch)
	if err != nil {
		t.Fatal(err)
	}
	right, rightAuth, rightDeltas, rightMetrics, err := Stage(restored, auth, version, 1, 1, patch)
	if err != nil || !reflect.DeepEqual(leftDeltas, rightDeltas) || !reflect.DeepEqual(leftMetrics, rightMetrics) {
		t.Fatalf("restored storage transition differs: %v", err)
	}
	leftBytes, leftErr := EncodeSnapshot(builtin, left, leftAuth, version+1)
	rightBytes, rightErr := EncodeSnapshot(dynamic, right, rightAuth, version+1)
	if leftErr != nil || rightErr != nil || !bytes.Equal(leftBytes, rightBytes) {
		t.Fatalf("restored storage continuation differs: %v/%v", leftErr, rightErr)
	}
}

func TestSnapshotTypedFieldsAndMembership(t *testing.T) {
	d := FatigueDescriptor()
	d.Fields = append(d.Fields, FieldDescriptor{
		ID: 2, Name: "history", Type: ValueType{Kind: sim.ListKind, Element: &ValueType{Kind: sim.IntegerKind}},
		Unit: "events", PermittedStates: allStates(), Uncertainty: UncertaintyForbidden,
	})
	d.Fields = append(d.Fields, FieldDescriptor{
		ID: 3, Name: "signals", Type: ValueType{Kind: sim.RecordKind, Fields: []ValueFieldType{
			{ID: 2, Name: "flag", Type: ValueType{Kind: sim.BoolKind}},
			{ID: 1, Name: "amount", Type: ValueType{Kind: sim.ScalarKind}},
		}}, Unit: "signals", PermittedStates: allStates(), Uncertainty: UncertaintyForbidden,
	})
	reg := singleRegistry(t, d)
	history, _ := sim.ListValue([]sim.Value{sim.IntegerValue(5), sim.IntegerValue(-2)})
	record, _ := sim.RecordValue([]sim.RecordField{{ID: 2, Value: sim.BoolValue(true)}, {ID: 1, Value: mustScalarValue(t, 2)}})
	reader, authority, err := NewReader(reg, 9, []ComponentSeed{
		{Entity: 2, Component: d.TypeID},
		{Entity: 1, Component: d.TypeID, Fields: []FieldSeed{{Field: 3, Value: record}, {Field: 2, Value: history}, {Field: 1, Value: mustScalarValue(t, .5)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeSnapshot(reg, reader, authority, 9)
	if err != nil {
		t.Fatal(err)
	}
	restored, auth, version, err := DecodeSnapshot(reg, encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range []sim.EntityID{1, 2} {
		has, err := restored.Has(HasRequest{Entity: entity, Component: d.TypeID, WorldVersion: version, Authority: auth})
		if err != nil || !has {
			t.Fatalf("lost membership %d: %v", entity, err)
		}
		view, err := restored.Read(ReadRequest{Entity: entity, Component: d.TypeID, WorldVersion: version, Authority: auth})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range d.Fields {
			value, err := view.Value(f.ID)
			if err != nil || (entity == 2 && (value.State() != sim.Missing || value.Kind() != f.Type.Kind)) {
				t.Fatalf("entity %d field %d: %v/%v", entity, f.ID, value, err)
			}
		}
	}
	if has, _ := restored.Has(HasRequest{Entity: 3, Component: d.TypeID, WorldVersion: version, Authority: auth}); has {
		t.Fatal("created unseeded membership")
	}
	encodedAgain, err := EncodeSnapshot(reg, restored, auth, version)
	if err != nil || !bytes.Equal(encoded, encodedAgain) {
		t.Fatalf("typed values changed: %v", err)
	}
}

func TestRegistryFingerprintTracksLogicalRulesAndProjections(t *testing.T) {
	base := EnergyDescriptor()
	original, err := RegistryFingerprint(singleRegistry(t, base))
	if err != nil {
		t.Fatal(err)
	}
	changes := []struct {
		name string
		edit func(*ComponentDescriptor)
	}{
		{"schema", func(d *ComponentDescriptor) { d.SchemaVersion++ }},
		{"field unit", func(d *ComponentDescriptor) { d.Fields[0].Unit = "watts" }},
		{"field bounds", func(d *ComponentDescriptor) { d.Fields[0].Bounds.Minimum = 1 }},
		{"rule version", func(d *ComponentDescriptor) { d.TransitionRules[0].Version++ }},
		{"rule name", func(d *ComponentDescriptor) { d.TransitionRules[0].Name += "-changed" }},
		{"projection version", func(d *ComponentDescriptor) { d.Projections[0].Version++ }},
		{"projection coefficient", func(d *ComponentDescriptor) { d.Projections[0].Coefficients[0] = 2 }},
		{"projection intercept", func(d *ComponentDescriptor) { d.Projections[0].Intercept = 1 }},
		{"projection bounds", func(d *ComponentDescriptor) { d.Projections[0].Bounds.Minimum = 1 }},
		{"projection unit", func(d *ComponentDescriptor) { d.Projections[0].Unit = "calories" }},
	}
	for _, test := range changes {
		t.Run(test.name, func(t *testing.T) {
			d := EnergyDescriptor()
			test.edit(&d)
			got, err := RegistryFingerprint(singleRegistry(t, d))
			if err != nil || got == original {
				t.Fatalf("logical change did not change fingerprint: %x %v", got, err)
			}
		})
	}
	permuted := EnergyDescriptor()
	permuted.TransitionRules[0], permuted.TransitionRules[1] = permuted.TransitionRules[1], permuted.TransitionRules[0]
	permuted.Fields[0].PermittedStates[0], permuted.Fields[0].PermittedStates[1] = permuted.Fields[0].PermittedStates[1], permuted.Fields[0].PermittedStates[0]
	got, err := RegistryFingerprint(singleRegistry(t, permuted))
	if err != nil || got != original {
		t.Fatalf("unordered declaration changed fingerprint: %x %v", got, err)
	}
	// An inactive bound and IEEE negative zero do not alter logical behavior.
	zeroEquivalent := EnergyDescriptor()
	zeroEquivalent.Fields[0].Bounds.Maximum = 123
	zeroEquivalent.Projections[0].Bounds.Maximum = 456
	zeroEquivalent.Projections[0].Intercept = math.Copysign(0, -1)
	got, err = RegistryFingerprint(singleRegistry(t, zeroEquivalent))
	if err != nil || got != original {
		t.Fatalf("inactive bounds changed fingerprint: %x %v", got, err)
	}
}

func TestSnapshotRejectsTamperingAndWrongAuthority(t *testing.T) {
	registry := singleRegistry(t, EnergyDescriptor())
	reader, authority, err := NewReader(registry, 7, []ComponentSeed{
		{Entity: 1, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustScalarValue(t, 1)}}},
		{Entity: 2, Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: mustScalarValue(t, 2)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	original, err := EncodeSnapshot(registry, reader, authority, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeSnapshot(registry, Reader((*system)(nil)), Authority{}, 7); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("nil reader accepted: %v", err)
	}
	if _, err := EncodeSnapshot(registry, reader, Authority{}, 7); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized export: %v", err)
	}
	if _, err := EncodeSnapshot(registry, reader, authority, 8); !errors.Is(err, ErrWorldVersionUnavailable) {
		t.Fatalf("wrong version export: %v", err)
	}
	changed := EnergyDescriptor()
	changed.TransitionRules[0].Version++
	if _, _, _, err := DecodeSnapshot(singleRegistry(t, changed), original); !errors.Is(err, ErrSnapshotEncoding) {
		t.Fatalf("accepted registry mismatch: %v", err)
	}
	// Header: magic[0:4], format[4:8], fingerprint[8:40], version[40:48], count[48:52].
	// First record: type[52:56], schema[56:60], entity[60:68], fields[68:72], field ID[72:76], length[76:80], value[80:90].
	cases := []struct {
		name   string
		modify func([]byte) []byte
	}{
		{"magic", func(b []byte) []byte { b[0] ^= 1; return b }},
		{"format", func(b []byte) []byte { b[7]++; return b }},
		{"fingerprint", func(b []byte) []byte { b[8] ^= 1; return b }},
		{"unknown schema", func(b []byte) []byte { b[59]++; return b }},
		{"unknown field", func(b []byte) []byte { b[75]++; return b }},
		{"wrong value kind", func(b []byte) []byte { b[80] = byte(sim.BoolKind); return b }},
		{"wrong value state", func(b []byte) []byte { b[81] = byte(sim.Missing); return b }},
		{"duplicate entity", func(b []byte) []byte { binary.BigEndian.PutUint64(b[98:106], 1); return b }},
		{"reverse valid entities", func(b []byte) []byte { binary.BigEndian.PutUint64(b[60:68], 3); return b }},
		{"record count", func(b []byte) []byte { b[51]++; return b }},
		{"oversized value", func(b []byte) []byte { binary.BigEndian.PutUint32(b[76:80], sim.MaxValueBytes+1); return b }},
		{"short value", func(b []byte) []byte { b[79]--; return b }},
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }},
		{"trailing byte", func(b []byte) []byte { return append(b, 0) }},
		{"over limit", func(b []byte) []byte { return make([]byte, MaxSnapshotBytes+1) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			b := test.modify(bytes.Clone(original))
			if _, _, _, err := DecodeSnapshot(registry, b); !errors.Is(err, ErrSnapshotEncoding) {
				t.Fatalf("accepted malformed snapshot: %v", err)
			}
		})
	}
}

func snapshotWideDescriptor(fields int) ComponentDescriptor {
	d := FatigueDescriptor()
	for id := 2; id <= fields; id++ {
		d.Fields = append(d.Fields, FieldDescriptor{
			ID: sim.FieldID(id), Name: "field-" + strconv.Itoa(id),
			Type: ValueType{Kind: sim.ScalarKind}, Unit: "ratio",
			PermittedStates: allStates(), Uncertainty: UncertaintyForbidden,
		})
	}
	return d
}

func TestSnapshotRejectsNoncanonicalFieldsAndPreflightsResources(t *testing.T) {
	registry := singleRegistry(t, snapshotWideDescriptor(2))
	reader, authority, err := NewReader(registry, 5, []ComponentSeed{
		{Entity: 2, Component: FatigueTypeID, Fields: []FieldSeed{{Field: 1, Value: mustScalarValue(t, .5)}, {Field: 2, Value: mustScalarValue(t, .5)}}},
		{Entity: 1, Component: FatigueTypeID, Fields: []FieldSeed{{Field: 1, Value: mustScalarValue(t, .5)}, {Field: 2, Value: mustScalarValue(t, .5)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	original, err := EncodeSnapshot(registry, reader, authority, 5)
	if err != nil {
		t.Fatal(err)
	}
	// First record begins at byte 52, with scalar fields at 72 and 90.
	for _, test := range []struct {
		name string
		edit func([]byte)
	}{
		{"duplicate field", func(b []byte) { binary.BigEndian.PutUint32(b[90:94], 1) }},
		{"decreasing fields", func(b []byte) {
			binary.BigEndian.PutUint32(b[72:76], 2)
			binary.BigEndian.PutUint32(b[90:94], 1)
		}},
		{"field count mismatch", func(b []byte) { binary.BigEndian.PutUint32(b[68:72], 4096) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.Clone(original)
			test.edit(b)
			if _, _, _, err := DecodeSnapshot(registry, b); !errors.Is(err, ErrSnapshotEncoding) {
				t.Fatalf("accepted malformed fields: %v", err)
			}
		})
	}
	large := singleRegistry(t, snapshotWideDescriptor(128))
	fingerprint, _ := RegistryFingerprint(large)
	var raw bytes.Buffer
	writeSnapshotNumber(&raw, snapshotMagic)
	writeSnapshotNumber(&raw, SnapshotFormatVersion)
	raw.Write(fingerprint[:])
	writeSnapshotNumber(&raw, uint64(5))
	writeSnapshotNumber(&raw, uint32(maxSnapshotSlots/128+1))
	for entity := 1; entity <= maxSnapshotSlots/128+1; entity++ {
		writeSnapshotNumber(&raw, uint32(FatigueTypeID))
		writeSnapshotNumber(&raw, uint32(1))
		writeSnapshotNumber(&raw, uint64(entity))
		writeSnapshotNumber(&raw, uint32(128))
		for id := 1; id <= 128; id++ {
			writeSnapshotNumber(&raw, uint32(id))
			writeSnapshotNumber(&raw, uint32(2))
			raw.Write([]byte{byte(sim.ScalarKind), byte(sim.Missing)})
		}
	}
	if _, _, _, err := DecodeSnapshot(large, raw.Bytes()); !errors.Is(err, ErrSnapshotEncoding) {
		t.Fatalf("accepted aggregate slot overflow: %v", err)
	}
	// The same large schema and ordinary sparse seed remain valid below the cap.
	wideReader, wideAuth, err := NewReader(large, 5, []ComponentSeed{{Entity: 1, Component: FatigueTypeID, Fields: []FieldSeed{{Field: 1, Value: mustScalarValue(t, .5)}}}})
	if err != nil {
		t.Fatal(err)
	}
	wideBytes, err := EncodeSnapshot(large, wideReader, wideAuth, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := DecodeSnapshot(large, wideBytes); err != nil {
		t.Fatalf("rejected ordinary sparse seed: %v", err)
	}
	// All identities are valid; a late failure on invalid IDs cannot pass.
	oversizeIDs := make([]sim.EntityID, maxSnapshotRecords+1)
	for i := range oversizeIDs {
		oversizeIDs[i] = sim.EntityID(i + 1)
	}
	oversize := &system{registry: registry, version: 5, stores: map[sim.ComponentTypeID]componentStore{
		FatigueTypeID: &dynamicStore{entities: oversizeIDs},
	}}
	if _, err := EncodeSnapshot(registry, oversize, Authority{owner: oversize}, 5); !errors.Is(err, ErrSnapshotEncoding) {
		t.Fatalf("did not reject record count before scanning: %v", err)
	}
	// This reader is built from valid seeds. A wide scan would allocate its
	// full 128-column batch before returning an error, exceeding the ceiling.
	boundedReader, boundedAuth, boundedRegistry := snapshotOverBudgetReader(t, 128)
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := EncodeSnapshot(boundedRegistry, boundedReader, boundedAuth, 5); !errors.Is(err, ErrSnapshotEncoding) {
				b.Fatalf("did not reject valid slot-heavy source: %v", err)
			}
		}
	})
	if got := result.AllocedBytesPerOp(); got > 1<<20 {
		t.Fatalf("preflight allocated %d B/op, likely scanned columns before rejecting", got)
	}
}

func snapshotOverBudgetReader(t testing.TB, fields int) (Reader, Authority, Registry) {
	t.Helper()
	registry := singleRegistry(t, snapshotWideDescriptor(fields))
	seeds := make([]ComponentSeed, maxSnapshotSlots/fields+1)
	for i := range seeds {
		seeds[i] = ComponentSeed{Entity: sim.EntityID(i + 1), Component: FatigueTypeID}
	}
	reader, auth, err := NewReader(registry, 5, seeds)
	if err != nil {
		t.Fatal(err)
	}
	return reader, auth, registry
}

func BenchmarkSnapshotRejectsOverBudgetBeforeScan(b *testing.B) {
	b.StopTimer()
	reader, auth, registry := snapshotOverBudgetReader(b, 128)
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if _, err := EncodeSnapshot(registry, reader, auth, 5); !errors.Is(err, ErrSnapshotEncoding) {
			b.Fatalf("over-budget source accepted: %v", err)
		}
	}
}

// snapshotNestedListWire constructs a schema-correct canonical value wire
// without materializing the huge Value tree being tested.
func snapshotNestedListWire(t testing.TB, groups, perGroup int) ([]byte, Registry) {
	t.Helper()
	d := FatigueDescriptor()
	d.Fields = append(d.Fields, FieldDescriptor{
		ID: 2, Name: "nested", Unit: "values", PermittedStates: allStates(),
		Type:        ValueType{Kind: sim.ListKind, Element: &ValueType{Kind: sim.ListKind, Element: &ValueType{Kind: sim.BoolKind}}},
		Uncertainty: UncertaintyForbidden,
	})
	registry := singleRegistry(t, d)
	fingerprint, err := RegistryFingerprint(registry)
	if err != nil {
		t.Fatal(err)
	}
	var nested bytes.Buffer
	nested.Write([]byte{byte(sim.ListKind), byte(sim.Present)})
	writeSnapshotNumber(&nested, uint32(groups))
	for group := 0; group < groups; group++ {
		nested.Write([]byte{byte(sim.ListKind), byte(sim.Present)})
		writeSnapshotNumber(&nested, uint32(perGroup))
		for item := 0; item < perGroup; item++ {
			nested.Write([]byte{byte(sim.BoolKind), byte(sim.Present), 0})
		}
	}
	var wire bytes.Buffer
	writeSnapshotNumber(&wire, snapshotMagic)
	writeSnapshotNumber(&wire, SnapshotFormatVersion)
	wire.Write(fingerprint[:])
	writeSnapshotNumber(&wire, uint64(5))
	writeSnapshotNumber(&wire, uint32(1))
	writeSnapshotNumber(&wire, uint32(FatigueTypeID))
	writeSnapshotNumber(&wire, uint32(1))
	writeSnapshotNumber(&wire, uint64(1))
	writeSnapshotNumber(&wire, uint32(2))
	writeSnapshotNumber(&wire, uint32(1))
	writeSnapshotNumber(&wire, uint32(2))
	wire.Write([]byte{byte(sim.ScalarKind), byte(sim.Missing)})
	writeSnapshotNumber(&wire, uint32(2))
	writeSnapshotNumber(&wire, uint32(nested.Len()))
	wire.Write(nested.Bytes())
	return wire.Bytes(), registry
}

func TestSnapshotRejectsNestedNodeAmplificationBeforeMaterialization(t *testing.T) {
	valid, registry := snapshotNestedListWire(t, 1, 2)
	reader, auth, version, err := DecodeSnapshot(registry, valid)
	if err != nil {
		t.Fatalf("valid nested list rejected: %v", err)
	}
	again, err := EncodeSnapshot(registry, reader, auth, version)
	if err != nil || !bytes.Equal(again, valid) {
		t.Fatalf("valid nested list did not round-trip: %v", err)
	}
	bomb, registry := snapshotNestedListWire(t, 5, 4096)
	if len(bomb) >= sim.MaxValueBytes {
		t.Fatalf("test wire not compact: %d bytes", len(bomb))
	}
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, _, _, err := DecodeSnapshot(registry, bomb); !errors.Is(err, ErrSnapshotEncoding) {
				b.Fatalf("nested node budget bypassed: %v", err)
			}
		}
	})
	if got := result.AllocedBytesPerOp(); got > 1<<20 {
		t.Fatalf("nested rejection allocated %d B/op, likely materialized Values", got)
	}
	// Each value is below the limit; together their nodes exceed it.
	one, registry := snapshotNestedListWire(t, 2, 4096)
	if _, _, _, err := DecodeSnapshot(registry, one); err != nil {
		t.Fatalf("individual nested value should fit budget: %v", err)
	}
	two := bytes.Clone(one)
	binary.BigEndian.PutUint32(two[48:52], 2)
	second := bytes.Clone(one[52:])
	binary.BigEndian.PutUint64(second[8:16], 2)
	two = append(two, second...)
	if _, _, _, err := DecodeSnapshot(registry, two); !errors.Is(err, ErrSnapshotEncoding) {
		t.Fatalf("aggregate nested node budget bypassed: %v", err)
	}
}

func BenchmarkSnapshotRejectsNestedNodes(b *testing.B) {
	b.StopTimer()
	wire, registry := snapshotNestedListWire(b, 5, 4096)
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if _, _, _, err := DecodeSnapshot(registry, wire); !errors.Is(err, ErrSnapshotEncoding) {
			b.Fatalf("nested node budget bypassed: %v", err)
		}
	}
}

func BenchmarkSnapshotWideDynamicRoundTrip(b *testing.B) {
	registry, err := func() (Registry, error) {
		builder := NewRegistryBuilder()
		if err := builder.Register(snapshotWideDescriptor(64)); err != nil {
			return Registry{}, err
		}
		return builder.Freeze()
	}()
	if err != nil {
		b.Fatal(err)
	}
	seeds := make([]ComponentSeed, 100)
	for i := range seeds {
		seeds[i] = ComponentSeed{Entity: sim.EntityID(i + 1), Component: FatigueTypeID}
	}
	reader, auth, err := NewReader(registry, 3, seeds)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		data, err := EncodeSnapshot(registry, reader, auth, 3)
		if err != nil {
			b.Fatal(err)
		}
		if _, _, _, err := DecodeSnapshot(registry, data); err != nil {
			b.Fatal(err)
		}
	}
}
