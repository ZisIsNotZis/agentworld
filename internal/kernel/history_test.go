package kernel

import (
	"agentworld/internal/component"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func historyFixture(t *testing.T, mode component.StorageClass) (*Kernel, component.Registry, []component.ComponentSeed) {
	t.Helper()
	f := fixtures()[0]
	f.descriptor.StorageClass = mode
	return setup(t, f)
}

func TestHistoryEmptyRoundTripAndFreshCapabilities(t *testing.T) {
	original, registry, _ := historyFixture(t, component.BuiltinStorage)
	data, head, err := original.ExportHistory()
	if err != nil || head.Version != 7 || head.GenesisVersion != 7 || head.TipID != 0 || head.TipHash != [32]byte{} {
		t.Fatalf("empty export: %+v %v", head, err)
	}
	restored, gotHead, err := RestoreHistory(registry, data)
	if err != nil || head != gotHead {
		t.Fatalf("empty restore: %+v %v", gotHead, err)
	}
	if original.SnapshotHead().OriginID == restored.SnapshotHead().OriginID || original.SnapshotHead().Authority == restored.SnapshotHead().Authority {
		t.Fatal("origin or authority crossed process boundary")
	}
	again, againHead, err := restored.ExportHistory()
	if err != nil || !bytes.Equal(data, again) || againHead != head {
		t.Fatalf("empty bytes unstable: %v", err)
	}
	_, auth, _ := restored.Snapshot()
	p, err := restored.Plan(proposal(fixtures()[0], "continuation", 1, scalar(t, .3)), auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restored.CommitBatch([]Plan{p}); err != nil {
		t.Fatalf("restored kernel cannot continue: %v", err)
	}
}

func TestHistoryMultiTimeReplayAndStorageEquivalence(t *testing.T) {
	var first []byte
	var firstHead PortableHead
	for _, mode := range []component.StorageClass{component.BuiltinStorage, component.DynamicStorage} {
		k, registry, seeds := historyFixture(t, mode)
		_, auth, _ := k.Snapshot()
		plans := make([]Plan, 0, 2)
		for _, p := range []Proposal{proposal(fixtures()[0], "a", 1, scalar(t, .2)), proposal(fixtures()[0], "b", 2, scalar(t, .3))} {
			plan, err := k.Plan(p, auth)
			if err != nil {
				t.Fatal(err)
			}
			plans = append(plans, plan)
		}
		if _, err := k.CommitBatch(plans); err != nil {
			t.Fatal(err)
		}
		_, auth, _ = k.Snapshot()
		p := proposal(fixtures()[0], "c", 3, scalar(t, .4))
		p.Time = 21
		plan, err := k.Plan(p, auth)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := k.CommitBatch([]Plan{plan}); err != nil {
			t.Fatal(err)
		}
		// The initial reader and private snapshot must not use the caller's seed slice.
		seeds[0].Fields[0].Value = scalar(t, .99)
		seeds[1].Entity = 999
		encoded, head, err := k.ExportHistory()
		if err != nil || head.TipID != 3 || head.TipTime != 21 || head.Version != 10 {
			t.Fatalf("export: %+v %v", head, err)
		}
		if first == nil {
			first, firstHead = encoded, head
		} else if !bytes.Equal(first, encoded) || firstHead != head {
			t.Fatal("physical storage changed portable history")
		}
		restored, got, err := RestoreHistory(registry, first)
		if err != nil || got != head {
			t.Fatalf("cross-class restore: %+v %v", got, err)
		}
		if !reflect.DeepEqual(k.Events(), restored.Events()) {
			t.Fatal("event or metric replay differs")
		}
		reencoded, _, err := restored.ExportHistory()
		if err != nil || !bytes.Equal(reencoded, first) {
			t.Fatalf("round-trip changed wire: %v", err)
		}
		_, auth, _ = restored.Snapshot()
		p = proposal(fixtures()[0], "after", 4, scalar(t, .5))
		p.Time = 22
		plan, err = restored.Plan(p, auth)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = restored.CommitBatch([]Plan{plan}); err != nil {
			t.Fatal(err)
		}
	}
}

func historyCommitted(t *testing.T) (*Kernel, component.Registry, []byte, PortableHead) {
	t.Helper()
	k, registry, _ := historyFixture(t, component.BuiltinStorage)
	_, auth, _ := k.Snapshot()
	plan, err := k.Plan(proposal(fixtures()[0], "one", 1, scalar(t, .5)), auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.CommitBatch([]Plan{plan}); err != nil {
		t.Fatal(err)
	}
	wire, head, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return k, registry, wire, head
}

func historyFrameOffset(data []byte) int {
	return 212 + int(binary.BigEndian.Uint32(data[40:44])) + int(binary.BigEndian.Uint32(data[44:48]))
}

func TestHistoryRejectsTruncationLengthsAndTampering(t *testing.T) {
	_, registry, original, head := historyCommitted(t)
	frame := historyFrameOffset(original)
	body := frame + 68
	cases := []struct {
		name   string
		change func([]byte) []byte
	}{
		{"magic", func(b []byte) []byte { b[0] ^= 1; return b }},
		{"format", func(b []byte) []byte { b[7]++; return b }},
		{"fingerprint", func(b []byte) []byte { b[8] ^= 1; return b }},
		{"genesis hash", func(b []byte) []byte { b[60] ^= 1; return b }},
		{"snapshot hash", func(b []byte) []byte { b[148] ^= 1; return b }},
		{"projection hash", func(b []byte) []byte { b[180] ^= 1; return b }},
		{"tip", func(b []byte) []byte { b[147] ^= 1; return b }},
		{"tip id", func(b []byte) []byte { b[107]++; return b }},
		{"frame count", func(b []byte) []byte { b[51]++; return b }},
		{"frame size overflow", func(b []byte) []byte { binary.BigEndian.PutUint32(b[frame:frame+4], ^uint32(0)); return b }},
		{"frame size short", func(b []byte) []byte { b[frame+3]--; return b }},
		{"frame size zero", func(b []byte) []byte { binary.BigEndian.PutUint32(b[frame:frame+4], 0); return b }},
		{"body mutation", func(b []byte) []byte { b[body+7] ^= 1; return b }},
		{"chain mutation", func(b []byte) []byte { b[frame+4] ^= 1; return b }},
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }},
		{"short header", func(b []byte) []byte { return b[:5] }},
		{"trailing", func(b []byte) []byte { return append(b, 0) }},
		{"over budget", func(b []byte) []byte { return make([]byte, MaxHistoryBytes+1) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mutated := test.change(bytes.Clone(original))
			if got, _, err := RestoreHistory(registry, mutated); !errors.Is(err, ErrEventIntegrity) || got != nil {
				t.Fatalf("accepted tampering: %v", err)
			}
		})
	}
	if _, gotHead, err := RestoreHistory(registry, original); err != nil || gotHead != head {
		t.Fatalf("control rejected: %v", err)
	}
}

func TestHistoryRejectsRehashedInvalidEventsAndSnapshotMismatch(t *testing.T) {
	_, registry, original, _ := historyCommitted(t)
	frame := historyFrameOffset(original)
	body := frame + 68
	// Recompute a coherent frame and header tip. Replay, not just hashing,
	// must reject changes to event versions, source rules, and projections.
	for _, test := range []struct {
		name string
		edit func([]byte)
	}{
		{"version", func(b []byte) { b[body+31]++ }},
		{"invalid key length", func(b []byte) {
			p := body + 32
			p += 4 + int(binary.BigEndian.Uint32(b[p:p+4]))
			binary.BigEndian.PutUint32(b[p:p+4], ^uint32(0))
		}},
		{"invalid delta count", func(b []byte) {
			p := body + 32
			p += 4 + int(binary.BigEndian.Uint32(b[p:p+4]))
			p += 4 + int(binary.BigEndian.Uint32(b[p:p+4]))
			p += 9 + 4 + 4
			binary.BigEndian.PutUint32(b[p:p+4], 4097)
		}},
		{"rule", func(b []byte) {
			// Body: IDs/time/versions (32), kind length+kind, key length+key,
			// world byte+actor (9), then rule ID.
			p := body + 32
			p += 4 + int(binary.BigEndian.Uint32(b[p:p+4]))
			p += 4 + int(binary.BigEndian.Uint32(b[p:p+4]))
			p += 9
			b[p+3] = 99
		}},
		{"noncanonical metric", func(b []byte) { b[len(b)-1] ^= 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.Clone(original)
			test.edit(b)
			h := sha256.New()
			h.Write(b[frame+4 : frame+36])
			h.Write(b[body:])
			copy(b[frame+36:frame+68], h.Sum(nil))
			copy(b[116:148], b[frame+36:frame+68])
			if _, _, err := RestoreHistory(registry, b); !errors.Is(err, ErrEventIntegrity) {
				t.Fatalf("accepted rehashed event: %v", err)
			}
		})
	}
	// Replace a complete, schema-correct final snapshot and update its digest.
	// A mere snapshot decode would pass; replay must compare every component.
	other, _, _ := historyFixture(t, component.BuiltinStorage)
	_, auth, _ := other.Snapshot()
	p, err := other.Plan(proposal(fixtures()[0], "one", 2, scalar(t, .9)), auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitBatch([]Plan{p}); err != nil {
		t.Fatal(err)
	}
	otherWire, _, err := other.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	start := 212 + int(binary.BigEndian.Uint32(original[40:44]))
	end := frame
	b := bytes.Clone(original)
	copy(b[start:end], otherWire[start:end])
	digest := sha256.Sum256(b[start:end])
	copy(b[148:180], digest[:])
	if _, _, err := RestoreHistory(registry, b); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted alternate final state: %v", err)
	}

	changed := component.EnergyDescriptor()
	changed.Projections[0].Version++
	builder := component.NewRegistryBuilder()
	if err := builder.Register(changed); err != nil {
		t.Fatal(err)
	}
	wrong, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreHistory(wrong, original); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted projection mismatch: %v", err)
	}
	changed = component.EnergyDescriptor()
	changed.SchemaVersion++
	builder = component.NewRegistryBuilder()
	if err := builder.Register(changed); err != nil {
		t.Fatal(err)
	}
	wrong, err = builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreHistory(wrong, original); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted schema mismatch: %v", err)
	}
	changed = component.EnergyDescriptor()
	changed.TransitionRules[0].Version++
	builder = component.NewRegistryBuilder()
	if err := builder.Register(changed); err != nil {
		t.Fatal(err)
	}
	wrong, err = builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreHistory(wrong, original); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted rule mismatch: %v", err)
	}
}

func TestHistoryEntityCreateVersionedFormats(t *testing.T) {
	// v1 regression: a patch-only history must keep exporting byte-identical
	// format-1 bytes, and v1/v2/v3-world restores stay green.
	patchKernel, registry, _ := historyFixture(t, component.BuiltinStorage)
	_, auth, _ := patchKernel.Snapshot()
	plan, err := patchKernel.Plan(proposal(fixtures()[0], "one", 1, scalar(t, .5)), auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := patchKernel.CommitBatch([]Plan{plan}); err != nil {
		t.Fatal(err)
	}
	wireV1, headV1, err := patchKernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if format := binary.BigEndian.Uint32(wireV1[4:8]); format != HistoryFormatV1 {
		t.Fatalf("patch-only history exported format %d", format)
	}
	restoredV1, gotV1, err := RestoreHistory(registry, wireV1)
	if err != nil || gotV1 != headV1 {
		t.Fatalf("v1 restore: %+v %v", gotV1, err)
	}
	againV1, _, err := restoredV1.ExportHistory()
	if err != nil || !bytes.Equal(againV1, wireV1) {
		t.Fatalf("v1 bytes changed: %v", err)
	}
	// A format-2 header without any entity-create event is not a canonical
	// encoder output and must be rejected.
	forged := bytes.Clone(wireV1)
	binary.BigEndian.PutUint32(forged[4:8], HistoryFormatV2)
	if _, _, err := RestoreHistory(registry, forged); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted v2 header without entity-create: %v", err)
	}

	// A birth switches the export to format 2; restores reproduce the events
	// and the newborn rows exactly.
	k, _, _ := historyFixture(t, component.BuiltinStorage)
	_, auth, _ = k.Snapshot()
	birth := proposal(fixtures()[0], "birth", 1, scalar(t, .5))
	birth.Allocations = []component.ComponentSeed{{
		Entity: 10001, Component: fixtures()[0].descriptor.TypeID,
		Fields: []component.FieldSeed{{Field: fixtures()[0].field, Value: scalar(t, .125)}},
	}}
	birthPlan, err := k.Plan(birth, auth)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := k.CommitBatch([]Plan{birthPlan})
	if err != nil || len(accepted) != 1 || accepted[0].Kind != KindEntityCreate {
		t.Fatalf("birth commit: %+v %v", accepted, err)
	}
	wireV2, headV2, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if format := binary.BigEndian.Uint32(wireV2[4:8]); format != HistoryFormatV2 {
		t.Fatalf("entity-create history exported format %d", format)
	}
	restoredV2, gotV2, err := RestoreHistory(registry, wireV2)
	if err != nil || gotV2 != headV2 {
		t.Fatalf("v2 restore: %+v %v", gotV2, err)
	}
	if !reflect.DeepEqual(k.Events(), restoredV2.Events()) {
		t.Fatal("restored events differ")
	}
	if !read(t, restoredV2, fixtures()[0], 10001).Equal(scalar(t, .125)) {
		t.Fatal("restored kernel lost the newborn row")
	}
	againV2, _, err := restoredV2.ExportHistory()
	if err != nil || !bytes.Equal(againV2, wireV2) {
		t.Fatalf("v2 bytes changed: %v", err)
	}
	// An entity-create event is invalid in a format-1 history.
	forgedV2 := bytes.Clone(wireV2)
	binary.BigEndian.PutUint32(forgedV2[4:8], HistoryFormatV1)
	if _, _, err := RestoreHistory(registry, forgedV2); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted entity-create in v1 history: %v", err)
	}
	// A restored kernel continues committing births and stays portable.
	_, auth, _ = restoredV2.Snapshot()
	next := proposal(fixtures()[0], "next", 10001, scalar(t, .3))
	next.Time = 21
	nextPlan, err := restoredV2.Plan(next, auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restoredV2.CommitBatch([]Plan{nextPlan}); err != nil {
		t.Fatal(err)
	}
	continuedWire, continuedHead, err := restoredV2.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, gotContinued, err := RestoreHistory(registry, continuedWire); err != nil || gotContinued != continuedHead {
		t.Fatalf("continued restore: %+v %v", gotContinued, err)
	}
}

func TestHistoryRejectsEntityCreateSnapshotTampering(t *testing.T) {
	registry := singleHistoryRegistry(t)
	k, _, _ := historyFixture(t, component.BuiltinStorage)
	_, auth, _ := k.Snapshot()
	birth := proposal(fixtures()[0], "birth", 1, scalar(t, .5))
	birth.Allocations = []component.ComponentSeed{{
		Entity: 10001, Component: fixtures()[0].descriptor.TypeID,
		Fields: []component.FieldSeed{{Field: fixtures()[0].field, Value: scalar(t, .125)}},
	}}
	plan, err := k.Plan(birth, auth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.CommitBatch([]Plan{plan}); err != nil {
		t.Fatal(err)
	}
	wire, _, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	// Rewrite the newborn's seeded value inside the event body and rehash the
	// event chain and header tip. Replay still recomputes different deltas,
	// so the recorded final snapshot can no longer match.
	body := historyFrameOffset(wire) + 68
	tampered := bytes.Clone(wire)
	tampered[body+112] ^= 0x01
	frame := historyFrameOffset(tampered)
	h := sha256.New()
	h.Write(tampered[frame+4 : frame+36])
	h.Write(tampered[body:])
	copy(tampered[frame+36:frame+68], h.Sum(nil))
	copy(tampered[116:148], tampered[frame+36:frame+68])
	if _, _, err := RestoreHistory(registry, tampered); !errors.Is(err, ErrEventIntegrity) {
		t.Fatalf("accepted rehashed entity-create tamper: %v", err)
	}
}

func singleHistoryRegistry(t *testing.T) component.Registry {
	t.Helper()
	builder := component.NewRegistryBuilder()
	if err := builder.Register(component.EnergyDescriptor()); err != nil {
		t.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}
