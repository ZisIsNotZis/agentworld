package strategy

import (
	"agentworld/internal/sim"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"sort"
)

// A portable strategy bundle contains complete registered policies and the
// explicit binding overrides for each actor, never just a policy reference.
const (
	PortableFormatVersion uint32 = 1
	MaxPortableBytes             = 2 << 20
	MaxPortablePolicies          = 256
	MaxPortableBindings          = 4096
)

var (
	ErrPortableEncoding = errors.New("invalid or oversized strategy bundle")
	portableMagic       = [4]byte{'A', 'W', 'S', 'T'}
)

// ExportPortable encodes the registry and actor bindings in a bounded canonical
// order. The caller must hold its actor-binding map stable during export.
func (r *Registry) ExportPortable(bindings map[sim.EntityID]*Bound) ([]byte, error) {
	if r == nil || len(bindings) > MaxPortableBindings {
		return nil, ErrPortableEncoding
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.policies == nil || len(r.policies) > MaxPortablePolicies {
		return nil, ErrPortableEncoding
	}
	refs := make([]Ref, 0, len(r.policies))
	for ref := range r.policies {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refLess(refs[i], refs[j]) })
	actors := make([]sim.EntityID, 0, len(bindings))
	for actor := range bindings {
		actors = append(actors, actor)
	}
	sort.Slice(actors, func(i, j int) bool { return actors[i] < actors[j] })

	out := append([]byte(nil), portableMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, PortableFormatVersion)
	out = binary.BigEndian.AppendUint16(out, uint16(len(refs)))
	out = binary.BigEndian.AppendUint32(out, uint32(len(actors)))
	for _, ref := range refs {
		policy := r.policies[ref]
		if policy.Ref != ref || policy.FormatVersion != FormatV1 {
			return nil, ErrPortableEncoding
		}
		var err error
		out, err = appendPolicy(out, policy)
		if err != nil {
			return nil, err
		}
	}
	for _, actor := range actors {
		bound := bindings[actor]
		if sim.ValidateEntityID(actor) != nil || bound == nil {
			return nil, ErrPortableEncoding
		}
		policy, ok := r.policies[bound.policy.Ref]
		if !ok || !samePolicy(policy, bound.policy) || !validBound(bound, policy) {
			return nil, ErrPortableEncoding
		}
		out = binary.BigEndian.AppendUint64(out, uint64(actor))
		out = appendRef(out, bound.policy.Ref)
		ids := make([]ParamID, 0, len(bound.overrides))
		for id := range bound.overrides {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		out = append(out, byte(len(ids)))
		for _, id := range ids {
			out = binary.BigEndian.AppendUint16(out, uint16(id))
			out = binary.BigEndian.AppendUint64(out, math.Float64bits(bound.overrides[id]))
		}
	}
	if len(out)+sha256.Size > MaxPortableBytes {
		return nil, ErrPortableEncoding
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// RestorePortable requires the executable's independently constructed policy
// registry. It compares the entire immutable policy set, not just references,
// before returning any bindings. Unknown versions, malformed or noncanonical
// content, and invalid parent/parameter references fail closed. The digest
// detects accidental corruption; the enclosing checkpoint must protect the
// entire bundle against deliberate replacement.
func RestorePortable(data []byte, expected *Registry) (*Registry, map[sim.EntityID]*Bound, error) {
	if expected == nil || len(data) < 4+4+2+4+sha256.Size || len(data) > MaxPortableBytes {
		return nil, nil, ErrPortableEncoding
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [sha256.Size]byte(data[len(body):]) {
		return nil, nil, ErrPortableEncoding
	}
	d := portableDecoder{data: body}
	magic := d.take(4)
	version, count, actorCount := d.u32(), d.u16(), d.u32()
	if d.bad || !bytes.Equal(magic, portableMagic[:]) || version != PortableFormatVersion ||
		count > MaxPortablePolicies || actorCount > MaxPortableBindings {
		return nil, nil, ErrPortableEncoding
	}
	r := NewRegistry()
	var previous Ref
	for i := 0; i < int(count); i++ {
		policy := d.policy()
		if d.bad || (i > 0 && !refLess(previous, policy.Ref)) || r.Register(policy) != nil {
			return nil, nil, ErrPortableEncoding
		}
		previous = policy.Ref
	}
	expected.mu.RLock()
	matches := expected.policies != nil && len(expected.policies) == len(r.policies)
	if matches {
		for ref, policy := range r.policies {
			other, ok := expected.policies[ref]
			if !ok || !samePolicy(policy, other) {
				matches = false
				break
			}
		}
	}
	expected.mu.RUnlock()
	if !matches {
		return nil, nil, ErrPortableEncoding
	}
	bindings := make(map[sim.EntityID]*Bound, actorCount)
	var lastActor sim.EntityID
	for i := 0; i < int(actorCount); i++ {
		actor := sim.EntityID(d.u64())
		binding := d.binding()
		if d.bad || sim.ValidateEntityID(actor) != nil || actor <= lastActor {
			return nil, nil, ErrPortableEncoding
		}
		bound, err := r.Bind(binding)
		if err != nil {
			return nil, nil, ErrPortableEncoding
		}
		bindings[actor] = bound
		lastActor = actor
	}
	if d.bad || d.offset != len(body) {
		return nil, nil, ErrPortableEncoding
	}
	return r, bindings, nil
}

func refLess(a, b Ref) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.Version < b.Version
}

func appendRef(out []byte, ref Ref) []byte {
	out = append(out, byte(len(ref.ID)))
	out = append(out, ref.ID...)
	return binary.BigEndian.AppendUint32(out, ref.Version)
}

func appendPolicy(out []byte, p Policy) ([]byte, error) {
	if p.FormatVersion != FormatV1 || !validRef(p.Ref) ||
		p.Budget.Candidates < 1 || p.Budget.Candidates > MaxCandidates ||
		p.Budget.Evaluations < 1 || p.Budget.Evaluations > MaxEvaluations ||
		len(p.Actions) != 2 || len(p.Defaults) > MaxParameters ||
		(p.ParentRef != nil && !validRef(*p.ParentRef)) {
		return nil, ErrPortableEncoding
	}
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = appendRef(out, p.Ref)
	if p.ParentRef != nil {
		out = append(out, 1)
		out = appendRef(out, *p.ParentRef)
	} else {
		out = append(out, 0)
	}
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations), byte(len(p.Actions)))
	for _, action := range p.Actions {
		if len(action.Terms) > MaxUtilityTerms {
			return nil, ErrPortableEncoding
		}
		out = append(out, byte(action.Kind), byte(len(action.Terms)))
		for _, term := range action.Terms {
			out = append(out, byte(term.Feature))
			out = binary.BigEndian.AppendUint16(out, uint16(term.Param))
		}
	}
	ids := make([]ParamID, 0, len(p.Defaults))
	for id := range p.Defaults {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out = append(out, byte(len(ids)))
	for _, id := range ids {
		value := p.Defaults[id]
		if id == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, ErrPortableEncoding
		}
		out = binary.BigEndian.AppendUint16(out, uint16(id))
		out = binary.BigEndian.AppendUint64(out, math.Float64bits(value))
	}
	return out, nil
}

func samePolicy(a, b Policy) bool {
	x, err := appendPolicy(nil, a)
	if err != nil {
		return false
	}
	y, err := appendPolicy(nil, b)
	return err == nil && bytes.Equal(x, y)
}

func validBound(b *Bound, p Policy) bool {
	if len(b.overrides) > MaxParameters || len(b.params) != len(p.Defaults) {
		return false
	}
	for id, value := range b.overrides {
		if _, ok := p.Defaults[id]; !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	for id, value := range p.Defaults {
		if override, ok := b.overrides[id]; ok {
			value = override
		}
		resolved, ok := b.params[id]
		if !ok || math.Float64bits(resolved) != math.Float64bits(value) {
			return false
		}
	}
	return true
}

type portableDecoder struct {
	data   []byte
	offset int
	bad    bool
}

func (d *portableDecoder) take(n int) []byte {
	if d.bad || n < 0 || n > len(d.data)-d.offset {
		d.bad = true
		return nil
	}
	part := d.data[d.offset : d.offset+n]
	d.offset += n
	return part
}
func (d *portableDecoder) u8() byte {
	b := d.take(1)
	if b == nil {
		return 0
	}
	return b[0]
}
func (d *portableDecoder) u16() uint16 {
	b := d.take(2)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint16(b)
}
func (d *portableDecoder) u32() uint32 {
	b := d.take(4)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}
func (d *portableDecoder) u64() uint64 {
	b := d.take(8)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
func (d *portableDecoder) ref() Ref {
	length := d.u8()
	if length == 0 || length > MaxRefIDBytes {
		d.bad = true
		return Ref{}
	}
	id := d.take(int(length))
	ref := Ref{ID: string(id), Version: d.u32()}
	if !validRef(ref) {
		d.bad = true
	}
	return ref
}
func (d *portableDecoder) policy() Policy {
	p := Policy{FormatVersion: d.u32(), Ref: d.ref()}
	hasParent := d.u8()
	if hasParent == 1 {
		parent := d.ref()
		p.ParentRef = &parent
	} else if hasParent != 0 {
		d.bad = true
	}
	p.Budget = Budget{Candidates: int(d.u8()), Evaluations: int(d.u8())}
	count := d.u8()
	if count != 2 {
		d.bad = true
		return p
	}
	p.Actions = make([]Action, count)
	for i := range p.Actions {
		p.Actions[i].Kind = ActionKind(d.u8())
		n := d.u8()
		if n > MaxUtilityTerms {
			d.bad = true
			return p
		}
		p.Actions[i].Terms = make([]UtilityTerm, n)
		for j := range p.Actions[i].Terms {
			p.Actions[i].Terms[j] = UtilityTerm{Feature: FeatureID(d.u8()), Param: ParamID(d.u16())}
		}
	}
	n := d.u8()
	if n > MaxParameters {
		d.bad = true
		return p
	}
	p.Defaults = make(map[ParamID]float64, n)
	var previous ParamID
	for i := 0; i < int(n); i++ {
		id := ParamID(d.u16())
		value := math.Float64frombits(d.u64())
		if id <= previous || math.IsNaN(value) || math.IsInf(value, 0) {
			d.bad = true
			return p
		}
		p.Defaults[id] = value
		previous = id
	}
	return p
}
func (d *portableDecoder) binding() Binding {
	b := Binding{Ref: d.ref()}
	n := d.u8()
	if n > MaxParameters {
		d.bad = true
		return b
	}
	b.Overrides = make(map[ParamID]float64, n)
	var previous ParamID
	for i := 0; i < int(n); i++ {
		id := ParamID(d.u16())
		value := math.Float64frombits(d.u64())
		if id <= previous || math.IsNaN(value) || math.IsInf(value, 0) {
			d.bad = true
			return b
		}
		b.Overrides[id] = value
		previous = id
	}
	return b
}
