package strategy

import (
	"agentworld/internal/sim"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestPortableStrategyRoundTripAndActorOverrides(t *testing.T) {
	parent := fixture()
	child := fixture()
	child.Ref.Version = 2
	child.ParentRef = &parent.Ref
	child.Actions[0].Terms = append(child.Actions[0].Terms, UtilityTerm{Feature: Constant, Param: 4})
	child.Defaults[4] = math.Copysign(0, -1)
	child.Defaults[3] = 2
	other := fixture()
	other.Ref.ID = "another"
	registry := NewRegistry()
	for _, policy := range []Policy{other, parent, child} {
		if err := registry.Register(policy); err != nil {
			t.Fatal(err)
		}
	}
	absent, err := registry.Bind(Binding{Ref: child.Ref})
	if err != nil {
		t.Fatal(err)
	}
	input := map[ParamID]float64{3: 2, 4: math.Copysign(0, -1)} // explicit, even though equal to defaults
	explicit, err := registry.Bind(Binding{Ref: child.Ref, Overrides: input})
	if err != nil {
		t.Fatal(err)
	}
	input[3] = 99
	if explicit.Binding().Overrides[3] != 2 {
		t.Fatal("caller changed bound overrides")
	}
	snapshot := explicit.Binding()
	snapshot.Overrides[3] = 99
	if explicit.Binding().Overrides[3] != 2 || len(absent.Binding().Overrides) != 0 {
		t.Fatal("binding snapshot changed bound overrides")
	}
	third, err := registry.Bind(Binding{Ref: child.Ref, Overrides: map[ParamID]float64{3: 1}})
	if err != nil {
		t.Fatal(err)
	}
	actors := map[sim.EntityID]*Bound{3: third, 2: explicit, 1: absent}
	encoded, err := registry.ExportPortable(actors)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaxPortableBytes {
		t.Fatal("unbounded export")
	}
	for i := 0; i < 3; i++ {
		again, err := registry.ExportPortable(actors)
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("noncanonical export: %v", err)
		}
	}
	expected := NewRegistry()
	for _, policy := range []Policy{other, parent, child} {
		if err := expected.Register(policy); err != nil {
			t.Fatal(err)
		}
	}
	restored, after, err := RestorePortable(encoded, expected)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.policies) != 3 || len(after) != len(actors) ||
		!reflect.DeepEqual(restored.policies[child.Ref], registry.policies[child.Ref]) {
		t.Fatal("policy content or lineage lost")
	}
	if math.Float64bits(restored.policies[child.Ref].Defaults[4]) != math.Float64bits(child.Defaults[4]) {
		t.Fatal("parameter sign bit lost")
	}
	for actor, before := range actors {
		if !reflect.DeepEqual(before.Binding(), after[actor].Binding()) {
			t.Fatalf("actor %d overrides lost: %+v vs %+v", actor, before.Binding(), after[actor].Binding())
		}
		obs := observation()
		obs.Actor = actor
		want, e1 := before.Evaluate(obs)
		got, e2 := after[actor].Evaluate(obs)
		if e1 != nil || e2 != nil || got != want {
			t.Fatalf("actor %d evaluation changed: %+v %v / %+v %v", actor, want, e1, got, e2)
		}
	}
	if len(after[1].Binding().Overrides) != 0 || len(after[2].Binding().Overrides) != 2 || after[3].Binding().Overrides[3] != 1 {
		t.Fatal("actor-specific explicit overrides conflated")
	}
	again, err := restored.ExportPortable(after)
	if err != nil || !bytes.Equal(encoded, again) {
		t.Fatalf("not a stable round trip: %v", err)
	}
}

// Test offsets are read from a valid bundle; each mutation replaces exact wire
// fields and refreshes its checksum so decoding, not only hashing, rejects it.
type codecOffsets struct {
	policyRef, parent, actionCount, termParam, defaultCount, defaults int
	actor, bindingRef, overrideCount, overrides                       int
}

func strategyOffsets(data []byte) codecOffsets {
	d := portableDecoder{data: data[:len(data)-sha256.Size], offset: 14}
	var o codecOffsets
	d.u32()
	o.policyRef = d.offset
	d.ref()
	o.parent = d.offset
	if d.u8() == 1 {
		d.ref()
	}
	d.u8()
	d.u8()
	o.actionCount = d.offset
	for i, count := 0, int(d.u8()); i < count; i++ {
		d.u8()
		terms := int(d.u8())
		for j := 0; j < terms; j++ {
			d.u8()
			if o.termParam == 0 {
				o.termParam = d.offset
			}
			d.u16()
		}
	}
	o.defaultCount = d.offset
	for i, count := 0, int(d.u8()); i < count; i++ {
		if i == 0 {
			o.defaults = d.offset
		}
		d.take(10)
	}
	o.actor = d.offset
	d.u64()
	o.bindingRef = d.offset
	d.ref()
	o.overrideCount = d.offset
	d.u8()
	o.overrides = d.offset
	return o
}

func corrupted(data []byte, change func([]byte, codecOffsets)) []byte {
	copy := append([]byte(nil), data...)
	change(copy, strategyOffsets(copy))
	body := copy[:len(copy)-sha256.Size]
	sum := sha256.Sum256(body)
	copy = append(body, sum[:]...)
	return copy
}

func rawPortable(t *testing.T, policies []Policy, actors []struct {
	id      sim.EntityID
	binding Binding
}) []byte {
	t.Helper()
	out := append([]byte(nil), portableMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, PortableFormatVersion)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policies)))
	out = binary.BigEndian.AppendUint32(out, uint32(len(actors)))
	for _, p := range policies {
		var err error
		out, err = appendPolicy(out, p)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, actor := range actors {
		out = binary.BigEndian.AppendUint64(out, uint64(actor.id))
		out = appendRef(out, actor.binding.Ref)
		out = append(out, byte(len(actor.binding.Overrides)))
		for id, value := range actor.binding.Overrides {
			out = binary.BigEndian.AppendUint16(out, uint16(id))
			out = binary.BigEndian.AppendUint64(out, math.Float64bits(value))
		}
	}
	sum := sha256.Sum256(out)
	return append(out, sum[:]...)
}

func TestPortableStrategyRejectsNoncanonicalAndInvalidData(t *testing.T) {
	p := fixture()
	r := NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	bound, err := r.Bind(Binding{Ref: p.Ref, Overrides: map[ParamID]float64{1: 1, 3: 3}})
	if err != nil {
		t.Fatal(err)
	}
	valid, err := r.ExportPortable(map[sim.EntityID]*Bound{1: bound})
	if err != nil {
		t.Fatal(err)
	}
	actor := struct {
		id      sim.EntityID
		binding Binding
	}{id: 1, binding: bound.Binding()}
	q := fixture()
	q.Ref.Version++
	q.ParentRef = &p.Ref
	cases := map[string][]byte{
		"empty":                   nil,
		"truncated":               valid[:len(valid)-1],
		"oversized":               make([]byte, MaxPortableBytes+1),
		"checksum":                append(append([]byte(nil), valid[:15]...), append([]byte{valid[15] ^ 1}, valid[16:]...)...),
		"unknown bundle version":  corrupted(valid, func(b []byte, _ codecOffsets) { binary.BigEndian.PutUint32(b[4:], 2) }),
		"excess policy count":     corrupted(valid, func(b []byte, _ codecOffsets) { binary.BigEndian.PutUint16(b[8:], MaxPortablePolicies+1) }),
		"excess binding count":    corrupted(valid, func(b []byte, _ codecOffsets) { binary.BigEndian.PutUint32(b[10:], MaxPortableBindings+1) }),
		"unknown policy format":   corrupted(valid, func(b []byte, _ codecOffsets) { binary.BigEndian.PutUint32(b[14:], 2) }),
		"unknown policy ref":      corrupted(valid, func(b []byte, o codecOffsets) { b[o.policyRef+1] = 0 }),
		"unknown binding ref":     corrupted(valid, func(b []byte, o codecOffsets) { b[o.bindingRef+1] = 'x' }),
		"oversized ref":           corrupted(valid, func(b []byte, o codecOffsets) { b[o.policyRef] = MaxRefIDBytes + 1 }),
		"invalid parent flag":     corrupted(valid, func(b []byte, o codecOffsets) { b[o.parent] = 2 }),
		"excess terms":            corrupted(valid, func(b []byte, o codecOffsets) { b[o.actionCount+2] = MaxUtilityTerms + 1 }),
		"excess defaults":         corrupted(valid, func(b []byte, o codecOffsets) { b[o.defaultCount] = MaxParameters + 1 }),
		"excess overrides":        corrupted(valid, func(b []byte, o codecOffsets) { b[o.overrideCount] = MaxParameters + 1 }),
		"duplicate policies":      rawPortable(t, []Policy{p, p}, nil),
		"unsorted policies":       rawPortable(t, []Policy{q, p}, nil),
		"parent absent":           rawPortable(t, []Policy{q}, nil),
		"invalid parent relation": rawPortable(t, []Policy{p, func() Policy { c := q; c.ParentRef = &Ref{ID: "other", Version: 1}; return c }()}, nil),
		"duplicate actors": rawPortable(t, []Policy{p}, []struct {
			id      sim.EntityID
			binding Binding
		}{actor, actor}),
		"unsorted actors": rawPortable(t, []Policy{p}, []struct {
			id      sim.EntityID
			binding Binding
		}{{2, actor.binding}, actor}),
		"invalid actor": corrupted(valid, func(b []byte, o codecOffsets) { binary.BigEndian.PutUint64(b[o.actor:], 0) }),
		"nonfinite default": corrupted(valid, func(b []byte, o codecOffsets) {
			binary.BigEndian.PutUint64(b[o.defaults+2:], math.Float64bits(math.Inf(1)))
		}),
		"invalid budget": corrupted(valid, func(b []byte, o codecOffsets) { b[o.actionCount-2] = 0 }),
		"nonfinite override": corrupted(valid, func(b []byte, o codecOffsets) {
			binary.BigEndian.PutUint64(b[o.overrides+2:], math.Float64bits(math.NaN()))
		}),
		"duplicate defaults": corrupted(valid, func(b []byte, o codecOffsets) { copy(b[o.defaults+10:o.defaults+20], b[o.defaults:o.defaults+10]) }),
		"unsorted defaults": corrupted(valid, func(b []byte, o codecOffsets) {
			first := append([]byte(nil), b[o.defaults:o.defaults+10]...)
			copy(b[o.defaults:o.defaults+10], b[o.defaults+10:o.defaults+20])
			copy(b[o.defaults+10:o.defaults+20], first)
		}),
		"duplicate overrides": corrupted(valid, func(b []byte, o codecOffsets) { copy(b[o.overrides+10:o.overrides+20], b[o.overrides:o.overrides+10]) }),
		"unsorted overrides": corrupted(valid, func(b []byte, o codecOffsets) {
			first := append([]byte(nil), b[o.overrides:o.overrides+10]...)
			copy(b[o.overrides:o.overrides+10], b[o.overrides+10:o.overrides+20])
			copy(b[o.overrides+10:o.overrides+20], first)
		}),
		"unknown parameter":    corrupted(valid, func(b []byte, o codecOffsets) { binary.BigEndian.PutUint16(b[o.overrides+10:], 9) }),
		"missing term default": corrupted(valid, func(b []byte, o codecOffsets) { binary.BigEndian.PutUint16(b[o.termParam:], 9) }),
		"unknown action":       corrupted(valid, func(b []byte, o codecOffsets) { b[o.actionCount+1] = byte(Wait) }),
		"trailing data":        corrupted(append(valid[:len(valid)-sha256.Size:len(valid)-sha256.Size], append([]byte{0}, valid[len(valid)-sha256.Size:]...)...), func([]byte, codecOffsets) {}),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := RestorePortable(input, r); !errors.Is(err, ErrPortableEncoding) {
				t.Fatalf("accepted invalid wire: %v", err)
			}
		})
	}
}

func TestPortableStrategyRejectsChangedExecutablePolicy(t *testing.T) {
	p := fixture()
	stored := NewRegistry()
	if err := stored.Register(p); err != nil {
		t.Fatal(err)
	}
	bundle, err := stored.ExportPortable(nil)
	if err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*Policy){
		"defaults":     func(p *Policy) { p.Defaults[1] = 2 },
		"terms":        func(p *Policy) { p.Actions[0].Terms[0].Feature = Constant },
		"budget":       func(p *Policy) { p.Budget.Candidates-- },
		"action order": func(p *Policy) { p.Actions[0], p.Actions[1] = p.Actions[1], p.Actions[0] },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			current := fixture()
			change(&current)
			expected := NewRegistry()
			if err := expected.Register(current); err != nil {
				t.Fatal(err)
			}
			if _, _, err := RestorePortable(bundle, expected); !errors.Is(err, ErrPortableEncoding) {
				t.Fatalf("same-Ref changed executable policy accepted: %v", err)
			}
		})
	}
	if _, _, err := RestorePortable(bundle, nil); !errors.Is(err, ErrPortableEncoding) {
		t.Fatalf("accepted missing expected policy registry: %v", err)
	}
}

func TestPortableStrategyRejectsMismatchedBoundAndLimits(t *testing.T) {
	p := fixture()
	r := NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(Binding{Ref: p.Ref, Overrides: map[ParamID]float64{3: 2}})
	if err != nil {
		t.Fatal(err)
	}
	different := NewRegistry()
	p.Defaults[3]++
	if err := different.Register(p); err != nil {
		t.Fatal(err)
	}
	for name, registry := range map[string]*Registry{"different policy": different, "nil registry": nil} {
		t.Run(name, func(t *testing.T) {
			if _, err := registry.ExportPortable(map[sim.EntityID]*Bound{1: b}); !errors.Is(err, ErrPortableEncoding) {
				t.Fatalf("export accepted policy mismatch: %v", err)
			}
		})
	}
	for name, bindings := range map[string]map[sim.EntityID]*Bound{
		"invalid actor": {0: b}, "nil bound": {1: nil},
		"excess actors": func() map[sim.EntityID]*Bound {
			m := make(map[sim.EntityID]*Bound, MaxPortableBindings+1)
			for i := 1; i <= MaxPortableBindings+1; i++ {
				m[sim.EntityID(i)] = b
			}
			return m
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.ExportPortable(bindings); !errors.Is(err, ErrPortableEncoding) {
				t.Fatalf("export accepted invalid bindings: %v", err)
			}
		})
	}
}
