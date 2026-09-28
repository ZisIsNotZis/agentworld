package kernel

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"sort"
)

const (
	HistoryFormatVersion       uint32 = 1
	MaxHistoryBytes                   = 128 << 20
	MaxHistoryEvents                  = 65536
	MaxHistoryPatches                 = 65536
	MaxHistoryValueNodes              = 1 << 18
	MaxHistoryProjectionBytes         = 32 << 20
	MaxHistoryProjectedMetrics        = 65536
)

var historyMagic = [4]byte{'A', 'W', 'K', 'H'}

// PortableHead is value-only evidence for an accepted-event tip and complete
// component state. A caller must retain/trust a head separately to detect a
// wholesale replacement of an otherwise valid history; hashes are not signatures.
// Unlike Head, this contains no process-local origin, reader, or authority.
type PortableHead struct {
	RegistryFingerprint [32]byte
	GenesisVersion      sim.WorldVersion
	GenesisHash         [32]byte
	Version             sim.WorldVersion
	TipID               sim.EventID
	TipTime             sim.SimTime
	TipHash             [32]byte
	SnapshotHash        [32]byte
	ProjectionHash      [32]byte
}

// ExportHistory captures one consistent genesis, accepted event chain and
// complete canonical component snapshot. The returned bytes and head are owned
// by the caller. It does not publish a file or record rejected proposals.
func (k *Kernel) ExportHistory() ([]byte, PortableHead, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.events) > MaxHistoryEvents || len(k.genesis) > component.MaxSnapshotBytes {
		return nil, PortableHead{}, ErrEventIntegrity
	}
	fingerprint, err := component.RegistryFingerprint(k.registry)
	if err != nil {
		return nil, PortableHead{}, err
	}
	snapshot, err := component.EncodeSnapshot(k.registry, k.reader, k.authority, k.version)
	if err != nil {
		return nil, PortableHead{}, err
	}
	projections, err := hashHistoryProjections(k.registry, k.reader, k.authority, k.version)
	if err != nil {
		return nil, PortableHead{}, err
	}
	head := PortableHead{RegistryFingerprint: fingerprint, GenesisVersion: k.genesisVersion,
		GenesisHash: sha256.Sum256(k.genesis), Version: k.version, SnapshotHash: sha256.Sum256(snapshot), ProjectionHash: projections}
	if len(k.events) != 0 {
		last := k.events[len(k.events)-1]
		head.TipID, head.TipTime, head.TipHash = last.ID, last.Time, last.Hash
	}
	if uint64(k.genesisVersion) > ^uint64(0)-uint64(len(k.events)) || k.version != k.genesisVersion+sim.WorldVersion(len(k.events)) {
		return nil, PortableHead{}, ErrEventIntegrity
	}
	var out bytes.Buffer
	writeHistoryNumber(&out, historyMagic)
	writeHistoryNumber(&out, HistoryFormatVersion)
	out.Write(head.RegistryFingerprint[:])
	writeHistoryNumber(&out, uint32(len(k.genesis)))
	writeHistoryNumber(&out, uint32(len(snapshot)))
	writeHistoryNumber(&out, uint32(len(k.events)))
	writeHistoryNumber(&out, uint64(head.GenesisVersion))
	out.Write(head.GenesisHash[:])
	writeHistoryNumber(&out, uint64(head.Version))
	writeHistoryNumber(&out, uint64(head.TipID))
	writeHistoryNumber(&out, int64(head.TipTime))
	out.Write(head.TipHash[:])
	out.Write(head.SnapshotHash[:])
	out.Write(head.ProjectionHash[:])
	out.Write(k.genesis)
	out.Write(snapshot)
	if out.Len() > MaxHistoryBytes {
		return nil, PortableHead{}, ErrEventIntegrity
	}
	var previous [32]byte
	var patches, nodes uint64
	for _, event := range k.events {
		body, err := event.Bytes()
		if err != nil || len(body) > maxEventBytes || event.PreviousHash != previous {
			return nil, PortableHead{}, ErrEventIntegrity
		}
		hash, err := hashEvent(event)
		if err != nil || hash != event.Hash || len(body)+68 > MaxHistoryBytes-out.Len() || uint64(len(event.Deltas)) > MaxHistoryPatches-patches {
			return nil, PortableHead{}, ErrEventIntegrity
		}
		_, used, err := decodeHistoryProposal(body, event.ID, event.BeforeVersion, MaxHistoryValueNodes-nodes)
		if err != nil {
			return nil, PortableHead{}, ErrEventIntegrity
		}
		patches += uint64(len(event.Deltas))
		nodes += used
		writeHistoryNumber(&out, uint32(len(body)))
		out.Write(event.PreviousHash[:])
		out.Write(event.Hash[:])
		out.Write(body)
		previous = event.Hash
	}
	return out.Bytes(), head, nil
}

// RestoreHistory rejects incompatible or malformed history, regenerates each
// event (including its deltas and projection metrics), and compares its full
// canonical body and final component snapshot. The returned kernel has fresh
// authority and origin. Compare the returned head to an independently retained
// PortableHead when a trusted continuation boundary is required.
func RestoreHistory(registry component.Registry, data []byte) (*Kernel, PortableHead, error) {
	fail := func() (*Kernel, PortableHead, error) { return nil, PortableHead{}, ErrEventIntegrity }
	if len(data) > MaxHistoryBytes {
		return fail()
	}
	r := historyReader{data: data}
	magic := r.take(4)
	format := r.u32()
	var head PortableHead
	copy(head.RegistryFingerprint[:], r.take(32))
	genesisSize, snapshotSize, eventCount := r.u32(), r.u32(), r.u32()
	head.GenesisVersion = sim.WorldVersion(r.u64())
	copy(head.GenesisHash[:], r.take(32))
	head.Version = sim.WorldVersion(r.u64())
	head.TipID = sim.EventID(r.u64())
	head.TipTime = sim.SimTime(r.u64())
	copy(head.TipHash[:], r.take(32))
	copy(head.SnapshotHash[:], r.take(32))
	copy(head.ProjectionHash[:], r.take(32))
	fingerprint, err := component.RegistryFingerprint(registry)
	if r.bad || !bytes.Equal(magic, historyMagic[:]) || format != HistoryFormatVersion || head.RegistryFingerprint != fingerprint ||
		genesisSize > component.MaxSnapshotBytes || snapshotSize > component.MaxSnapshotBytes || eventCount > MaxHistoryEvents ||
		uint64(head.GenesisVersion) > ^uint64(0)-uint64(eventCount) || head.Version != head.GenesisVersion+sim.WorldVersion(eventCount) ||
		head.TipID != sim.EventID(eventCount) || (eventCount == 0 && (head.TipTime != 0 || head.TipHash != [32]byte{})) {
		return fail()
	}
	genesis := r.take(int(genesisSize))
	snapshot := r.take(int(snapshotSize))
	if r.bad || sha256.Sum256(genesis) != head.GenesisHash || sha256.Sum256(snapshot) != head.SnapshotHash {
		return fail()
	}
	genesisReader, authority, version, err := component.DecodeSnapshot(registry, genesis)
	if err != nil || version != head.GenesisVersion {
		return fail()
	}
	seeds := make([]component.ComponentSeed, 0)
	for _, id := range registry.TypeIDs() {
		batch, err := genesisReader.Scan(component.ScanRequest{Component: id, WorldVersion: version, Authority: authority})
		if err != nil {
			return fail()
		}
		for _, entity := range batch.EntityIDs() {
			view, err := genesisReader.Read(component.ReadRequest{Entity: entity, Component: id, WorldVersion: version, Authority: authority})
			if err != nil {
				return fail()
			}
			seed := component.ComponentSeed{Entity: entity, Component: id}
			for _, field := range view.FieldIDs() {
				value, err := view.Value(field)
				if err != nil {
					return fail()
				}
				seed.Fields = append(seed.Fields, component.FieldSeed{Field: field, Value: value})
			}
			seeds = append(seeds, seed)
		}
	}
	k, err := New(registry, version, seeds)
	if err != nil || !bytes.Equal(genesis, k.genesis) {
		return fail()
	}
	type encodedEvent struct {
		body     []byte
		hash     [32]byte
		proposal Proposal
	}
	var group []encodedEvent
	flush := func() error {
		if len(group) == 0 {
			return nil
		}
		plans := make([]Plan, 0, len(group))
		for _, encoded := range group {
			plan, err := k.Plan(encoded.proposal, k.authority)
			if err != nil {
				return ErrEventIntegrity
			}
			plans = append(plans, plan)
		}
		accepted, err := k.CommitBatch(plans)
		if err != nil || len(accepted) != len(group) {
			return ErrEventIntegrity
		}
		for i, event := range accepted {
			body, err := event.Bytes()
			if err != nil || !bytes.Equal(body, group[i].body) || event.Hash != group[i].hash {
				return ErrEventIntegrity
			}
		}
		group = nil
		return nil
	}
	var previous [32]byte
	var lastTime sim.SimTime
	var patches, nodes uint64
	for i := uint32(0); i < eventCount; i++ {
		size := r.u32()
		linked := r.take(32)
		hashBytes := r.take(32)
		if r.bad || size > maxEventBytes || int(size) > r.remaining() || !bytes.Equal(linked, previous[:]) {
			return fail()
		}
		body := r.take(int(size))
		var hash [32]byte
		copy(hash[:], hashBytes)
		computed := sha256.New()
		computed.Write(previous[:])
		computed.Write(body)
		if !bytes.Equal(computed.Sum(nil), hash[:]) {
			return fail()
		}
		proposal, used, err := decodeHistoryProposal(body, sim.EventID(i+1), head.GenesisVersion+sim.WorldVersion(i), MaxHistoryValueNodes-nodes)
		if err != nil || uint64(len(proposal.Patches)) > MaxHistoryPatches-patches || (i > 0 && proposal.Time < lastTime) {
			return fail()
		}
		patches += uint64(len(proposal.Patches))
		nodes += used
		if i > 0 && proposal.Time != lastTime {
			if flush() != nil {
				return fail()
			}
		}
		group = append(group, encodedEvent{body: body, hash: hash, proposal: proposal})
		lastTime, previous = proposal.Time, hash
	}
	if r.bad || r.remaining() != 0 || flush() != nil || (eventCount != 0 && (head.TipTime != lastTime || head.TipHash != previous)) {
		return fail()
	}
	final, err := component.EncodeSnapshot(registry, k.reader, k.authority, k.version)
	if err != nil || !bytes.Equal(final, snapshot) || k.version != head.Version {
		return fail()
	}
	projections, err := hashHistoryProjections(registry, k.reader, k.authority, k.version)
	if err != nil || projections != head.ProjectionHash {
		return fail()
	}
	return k, head, nil
}

// Hash every final projection, including untouched component members. The
// canonical Metric encoding used in events commits values, uncertainty,
// projection versions, units, and provenance. Bound the total work even when
// a registry defines many projections per entity.
func hashHistoryProjections(registry component.Registry, reader component.Reader, authority component.Authority, version sim.WorldVersion) ([32]byte, error) {
	h := sha256.New()
	h.Write([]byte("kernel-projections-v1\x00"))
	var size, count uint64
	for _, id := range registry.TypeIDs() {
		descriptor, err := registry.Describe(id)
		if err != nil {
			return [32]byte{}, ErrEventIntegrity
		}
		projections := append([]component.ProjectionDescriptor(nil), descriptor.Projections...)
		sort.Slice(projections, func(i, j int) bool { return projections[i].ID < projections[j].ID })
		for _, projection := range projections {
			batch, err := reader.Project(component.ProjectRequest{Component: id, Projection: projection.ID, WorldVersion: version, Authority: authority})
			if err != nil || uint64(batch.Len()) > MaxHistoryProjectedMetrics-count {
				return [32]byte{}, ErrEventIntegrity
			}
			writeHistoryHashNumber(h, uint32(id))
			writeHistoryHashNumber(h, uint32(projection.ID))
			writeHistoryHashNumber(h, uint32(batch.Len()))
			count += uint64(batch.Len())
			for i := 0; i < batch.Len(); i++ {
				metric, err := batch.At(i)
				if err != nil {
					return [32]byte{}, ErrEventIntegrity
				}
				body, err := (Event{Metrics: []component.MetricDelta{{Before: metric, After: metric}}}).Bytes()
				if err != nil || uint64(len(body))+size > MaxHistoryProjectionBytes {
					return [32]byte{}, ErrEventIntegrity
				}
				size += uint64(len(body))
				writeHistoryHashNumber(h, uint32(len(body)))
				h.Write(body)
			}
		}
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

func writeHistoryHashNumber(h hash.Hash, number uint32) {
	var data [4]byte
	binary.BigEndian.PutUint32(data[:], number)
	h.Write(data[:])
}

// decodeHistoryProposal reads only the proposal-bearing prefix. The replayed
// canonical event body must equal the entire input, so no untrusted metric
// can be accepted without reproducing its schema/projection and all its bytes.
func decodeHistoryProposal(body []byte, id sim.EventID, version sim.WorldVersion, maxNodes uint64) (Proposal, uint64, error) {
	r := historyReader{data: body}
	eventID, time := sim.EventID(r.u64()), sim.SimTime(r.u64())
	before, after := r.u64(), r.u64()
	if r.bad || eventID != id || time < 0 || before != uint64(version) || before == ^uint64(0) || after != before+1 {
		return Proposal{}, 0, ErrEventIntegrity
	}
	kind := r.text()
	key := r.text()
	world := r.take(1)
	actor := sim.EntityID(r.u64())
	rule, ruleVersion := sim.RuleID(r.u32()), r.u32()
	count := r.u32()
	if r.bad || kind != "component-patch" || len(key) == 0 || len(key) > 4096 || len(world) != 1 || world[0] > 1 || count == 0 || count > 4096 || uint64(count)*32 > uint64(r.remaining()) {
		return Proposal{}, 0, ErrEventIntegrity
	}
	p := Proposal{Key: key, Time: time, Cause: Cause{Actor: actor, World: world[0] == 1}, Rule: rule, RuleVersion: ruleVersion, Patches: make([]component.Patch, 0, count)}
	if !p.Cause.valid() || p.Rule == 0 || p.RuleVersion == 0 {
		return Proposal{}, 0, ErrEventIntegrity
	}
	var nodes uint64
	for j := uint32(0); j < count; j++ {
		patch := component.Patch{Entity: sim.EntityID(r.u64()), Component: sim.ComponentTypeID(r.u32()), SchemaVersion: sim.SchemaVersion(r.u32()), Field: sim.FieldID(r.u32())}
		before := r.value()
		encoded := r.value()
		if r.bad || len(before) == 0 || len(encoded) == 0 || nodes >= maxNodes {
			return Proposal{}, 0, ErrEventIntegrity
		}
		used, err := sim.CountValueNodes(before, maxNodes-nodes)
		if err != nil {
			return Proposal{}, 0, ErrEventIntegrity
		}
		nodes += used
		used, err = sim.CountValueNodes(encoded, maxNodes-nodes)
		if err != nil {
			return Proposal{}, 0, ErrEventIntegrity
		}
		nodes += used
		value, err := sim.DecodeValue(encoded)
		if err != nil {
			return Proposal{}, 0, ErrEventIntegrity
		}
		patch.Value = value
		p.Patches = append(p.Patches, patch)
	}
	if r.bad || r.u32() > 4096 || r.bad {
		return Proposal{}, 0, ErrEventIntegrity
	}
	return p, nodes, nil
}

type historyReader struct {
	data   []byte
	offset int
	bad    bool
}

func (r *historyReader) remaining() int { return len(r.data) - r.offset }
func (r *historyReader) take(n int) []byte {
	if r.bad || n < 0 || n > r.remaining() {
		r.bad = true
		return nil
	}
	b := r.data[r.offset : r.offset+n]
	r.offset += n
	return b
}
func (r *historyReader) u32() uint32 {
	b := r.take(4)
	if r.bad {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}
func (r *historyReader) u64() uint64 {
	b := r.take(8)
	if r.bad {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
func (r *historyReader) text() string {
	n := r.u32()
	if n > maxEventBytes || int(n) > r.remaining() {
		r.bad = true
		return ""
	}
	return string(r.take(int(n)))
}
func (r *historyReader) value() []byte {
	n := r.u32()
	if n > sim.MaxValueBytes || int(n) > r.remaining() {
		r.bad = true
		return nil
	}
	return r.take(int(n))
}
func writeHistoryNumber(b *bytes.Buffer, value any) {
	_ = binary.Write(b, binary.BigEndian, value) // fixed-width primitives into bytes.Buffer
}
