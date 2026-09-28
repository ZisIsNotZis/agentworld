package component

import (
	"agentworld/internal/sim"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sort"
)

var ErrSnapshotEncoding = errors.New("invalid or oversized component snapshot")

const (
	SnapshotFormatVersion uint32 = 1
	MaxSnapshotBytes             = 32 << 20
	maxSnapshotRecords           = 65536
	maxSnapshotFields            = 4096
	maxSnapshotSlots             = 1 << 16 // field values retained during import
	// Includes roots and nested Values across the entire snapshot. The 16,384
	// node cap limits materialization and at most 32-deep codec cloning;
	// vector/scalar payloads are separately bounded by wire and value sizes.
	maxSnapshotValueNodes = 1 << 14
	maxRegistryBytes      = 1 << 20
)

var snapshotMagic = [4]byte{'A', 'W', 'C', 'S'}

// RegistryFingerprint identifies the complete logical registry, not its physical
// storage routing. Unordered descriptor collections are sorted by stable ID;
// projection source order remains significant because it pairs with coefficients.
func RegistryFingerprint(registry Registry) ([32]byte, error) {
	logical := make([]ComponentDescriptor, 0, len(registry.ids))
	for _, id := range registry.TypeIDs() {
		d, err := registry.Describe(id)
		if err != nil {
			return [32]byte{}, err
		}
		d.StorageClass = 0
		sort.Slice(d.Fields, func(i, j int) bool { return d.Fields[i].ID < d.Fields[j].ID })
		for i := range d.Fields {
			sort.Slice(d.Fields[i].PermittedStates, func(a, b int) bool {
				return d.Fields[i].PermittedStates[a] < d.Fields[i].PermittedStates[b]
			})
			sortValueType(&d.Fields[i].Type)
			canonicalBounds(&d.Fields[i].Bounds)
		}
		sort.Slice(d.TransitionRules, func(i, j int) bool { return d.TransitionRules[i].ID < d.TransitionRules[j].ID })
		sort.Slice(d.Projections, func(i, j int) bool { return d.Projections[i].ID < d.Projections[j].ID })
		for i := range d.Projections {
			p := &d.Projections[i]
			canonicalBounds(&p.Bounds)
			p.Intercept = canonicalSnapshotZero(p.Intercept)
			for j := range p.Coefficients {
				p.Coefficients[j] = canonicalSnapshotZero(p.Coefficients[j])
			}
		}
		sort.Slice(d.Dependencies, func(i, j int) bool { return d.Dependencies[i] < d.Dependencies[j] })
		sort.Slice(d.MigrationPaths, func(i, j int) bool {
			if d.MigrationPaths[i].From != d.MigrationPaths[j].From {
				return d.MigrationPaths[i].From < d.MigrationPaths[j].From
			}
			return d.MigrationPaths[i].To < d.MigrationPaths[j].To
		})
		logical = append(logical, d)
	}
	data, err := json.Marshal(logical)
	if err != nil || len(data) > maxRegistryBytes {
		return [32]byte{}, ErrSnapshotEncoding
	}
	return sha256.Sum256(append([]byte("component-registry-v1\x00"), data...)), nil
}

func canonicalBounds(b *Bounds) {
	if !b.HasMinimum {
		b.Minimum = 0
	} else {
		b.Minimum = canonicalSnapshotZero(b.Minimum)
	}
	if !b.HasMaximum {
		b.Maximum = 0
	} else {
		b.Maximum = canonicalSnapshotZero(b.Maximum)
	}
}

func canonicalSnapshotZero(v float64) float64 {
	if v == 0 {
		return 0
	}
	return v
}

func sortValueType(t *ValueType) {
	sort.Slice(t.Fields, func(i, j int) bool { return t.Fields[i].ID < t.Fields[j].ID })
	for i := range t.Fields {
		sortValueType(&t.Fields[i].Type)
	}
	if t.Element != nil {
		sortValueType(t.Element)
	}
	if t.Key != nil {
		sortValueType(t.Key)
	}
}

// EncodeSnapshot captures a complete logical state at version using its bound
// authority. Every member includes every schema field (including Missing), so
// membership and all four value states survive a change of storage class.
// The format contains no pointers, capabilities, or kernel authority.
func EncodeSnapshot(registry Registry, reader Reader, authority Authority, version sim.WorldVersion) ([]byte, error) {
	s, ok := reader.(*system)
	if !ok || s == nil {
		return nil, ErrInvalidRequest
	}
	if err := s.authorize(authority, version); err != nil {
		return nil, err
	}
	fingerprint, err := RegistryFingerprint(registry)
	if err != nil {
		return nil, err
	}
	actual, err := RegistryFingerprint(s.registry)
	if err != nil || fingerprint != actual {
		return nil, ErrSnapshotEncoding
	}
	var body bytes.Buffer
	if err := binary.Write(&body, binary.BigEndian, snapshotMagic); err != nil {
		return nil, err
	}
	writeSnapshotNumber(&body, SnapshotFormatVersion)
	body.Write(fingerprint[:])
	writeSnapshotNumber(&body, uint64(version))
	// Reserve a fixed-width count, filled after all records are gathered.
	countOffset := body.Len()
	writeSnapshotNumber(&body, uint32(0))
	// Preflight before even the ID-only scan allocates an entity slice. The
	// existing stores expose ordered IDs; only their length is inspected here.
	// No physical implementation is selected for encoding or interpretation.
	type source struct {
		id         sim.ComponentTypeID
		descriptor ComponentDescriptor
		fields     []sim.FieldID
		count      int
	}
	var sources []source
	var records, slots, minimumBytes uint64
	for _, id := range registry.TypeIDs() {
		d, _ := registry.Describe(id)
		if len(d.Fields) == 0 || len(d.Fields) > maxSnapshotFields {
			return nil, ErrSnapshotEncoding
		}
		store := s.stores[id]
		var entityCount int
		switch typed := store.(type) {
		case *energyStore:
			if typed == nil {
				return nil, ErrSnapshotEncoding
			}
			entityCount = len(typed.entities)
		case *dynamicStore:
			if typed == nil {
				return nil, ErrSnapshotEncoding
			}
			entityCount = len(typed.entities)
		default:
			return nil, ErrSnapshotEncoding
		}
		records += uint64(entityCount)
		slots += uint64(entityCount) * uint64(len(d.Fields))
		minimumBytes += uint64(entityCount) * (20 + 10*uint64(len(d.Fields)))
		if records > maxSnapshotRecords || slots > maxSnapshotSlots || minimumBytes > MaxSnapshotBytes-52 {
			return nil, ErrSnapshotEncoding
		}
		fields, err := selectFields(d, nil)
		if err != nil {
			return nil, ErrSnapshotEncoding
		}
		sources = append(sources, source{id: id, descriptor: d, fields: fields, count: entityCount})
	}
	var count uint32
	var nodes uint64
	for _, src := range sources {
		// A zero-field scan yields only sorted entity identities, not one
		// column of values per schema field. Read one complete row at a time.
		batch := s.stores[src.id].scan(nil)
		if batch.Len() != src.count {
			return nil, ErrSnapshotEncoding
		}
		entities := batch.entities
		for row, entity := range entities {
			if sim.ValidateEntityID(entity) != nil || (row > 0 && entity <= entities[row-1]) {
				return nil, ErrSnapshotEncoding
			}
			view, err := reader.Read(ReadRequest{Entity: entity, Component: src.id, WorldVersion: version, Authority: authority})
			if err != nil || len(view.fields) != len(src.fields) {
				return nil, ErrSnapshotEncoding
			}
			writeSnapshotNumber(&body, uint32(src.id))
			writeSnapshotNumber(&body, uint32(src.descriptor.SchemaVersion))
			writeSnapshotNumber(&body, uint64(entity))
			writeSnapshotNumber(&body, uint32(len(src.fields)))
			seed := ComponentSeed{Entity: entity, Component: src.id, Fields: make([]FieldSeed, 0, len(src.fields))}
			for i, field := range src.fields {
				if view.fields[i].id != field {
					return nil, ErrSnapshotEncoding
				}
				value := view.fields[i].value
				seed.Fields = append(seed.Fields, FieldSeed{Field: field, Value: value})
				data, err := sim.EncodeValue(value)
				if err != nil || len(data) > sim.MaxValueBytes || len(data) > MaxSnapshotBytes-body.Len()-8 {
					return nil, ErrSnapshotEncoding
				}
				counted, err := sim.CountValueNodes(data, maxSnapshotValueNodes-nodes)
				if err != nil {
					return nil, ErrSnapshotEncoding
				}
				nodes += counted
				writeSnapshotNumber(&body, uint32(field))
				writeSnapshotNumber(&body, uint32(len(data)))
				body.Write(data)
			}
			if err := validateSeed(src.descriptor, seed.Fields); err != nil || body.Len() > MaxSnapshotBytes {
				return nil, ErrSnapshotEncoding
			}
			count++
		}
	}
	binary.BigEndian.PutUint32(body.Bytes()[countOffset:countOffset+4], count)
	return body.Bytes(), nil
}

// DecodeSnapshot checks the canonical, bounded format and exact logical schema
// before rebuilding a reader via NewReader. The returned authority is a fresh
// in-process capability, never one recovered from the bytes.
func DecodeSnapshot(registry Registry, data []byte) (Reader, Authority, sim.WorldVersion, error) {
	fail := func() (Reader, Authority, sim.WorldVersion, error) {
		return nil, Authority{}, 0, ErrSnapshotEncoding
	}
	if len(data) > MaxSnapshotBytes {
		return fail()
	}
	fingerprint, err := RegistryFingerprint(registry)
	if err != nil {
		return fail()
	}
	r := bytes.NewReader(data)
	var magic [4]byte
	var format uint32
	var actual [32]byte
	var version uint64
	var count uint32
	if binary.Read(r, binary.BigEndian, &magic) != nil || binary.Read(r, binary.BigEndian, &format) != nil || binary.Read(r, binary.BigEndian, &actual) != nil || binary.Read(r, binary.BigEndian, &version) != nil || binary.Read(r, binary.BigEndian, &count) != nil || magic != snapshotMagic || format != SnapshotFormatVersion || actual != fingerprint || count > maxSnapshotRecords || uint64(count)*30 > uint64(r.Len()) {
		return fail()
	}
	for _, id := range registry.TypeIDs() {
		d, err := registry.Describe(id)
		if err != nil || len(d.Fields) == 0 || len(d.Fields) > maxSnapshotFields {
			return fail()
		}
	}
	seeds := make([]ComponentSeed, 0, count)
	type schemaFields struct {
		descriptor ComponentDescriptor
		ids        []sim.FieldID
	}
	byType := make(map[sim.ComponentTypeID]schemaFields)
	var prevType uint32
	var prevEntity uint64
	var slots, nodes uint64
	for i := uint32(0); i < count; i++ {
		var typeID, schema uint32
		var entity uint64
		var fieldCount uint32
		if binary.Read(r, binary.BigEndian, &typeID) != nil || binary.Read(r, binary.BigEndian, &schema) != nil || binary.Read(r, binary.BigEndian, &entity) != nil || binary.Read(r, binary.BigEndian, &fieldCount) != nil || (i > 0 && (typeID < prevType || (typeID == prevType && entity <= prevEntity))) || sim.ValidateEntityID(sim.EntityID(entity)) != nil {
			return fail()
		}
		id := sim.ComponentTypeID(typeID)
		known, ok := byType[id]
		if !ok {
			d, err := registry.Describe(id)
			if err != nil || len(d.Fields) == 0 || len(d.Fields) > maxSnapshotFields {
				return fail()
			}
			fields, err := selectFields(d, nil)
			if err != nil {
				return fail()
			}
			known = schemaFields{descriptor: d, ids: fields}
			byType[id] = known
		}
		if schema != uint32(known.descriptor.SchemaVersion) || fieldCount != uint32(len(known.ids)) {
			return fail()
		}
		slots += uint64(fieldCount)
		// Every field uses at least 8 framing bytes and 2 kind/state bytes;
		// reserve 20 bytes per remaining record before allocating field slots.
		if slots > maxSnapshotSlots || uint64(r.Len()) < uint64(fieldCount)*10+uint64(count-i-1)*20 {
			return fail()
		}
		seed := ComponentSeed{Entity: sim.EntityID(entity), Component: id, Fields: make([]FieldSeed, 0, fieldCount)}
		for j := uint32(0); j < fieldCount; j++ {
			var field, size uint32
			if binary.Read(r, binary.BigEndian, &field) != nil || binary.Read(r, binary.BigEndian, &size) != nil || field != uint32(known.ids[j]) || size > sim.MaxValueBytes || size > uint32(r.Len()) {
				return fail()
			}
			encoded := data[len(data)-r.Len() : len(data)-r.Len()+int(size)]
			if _, err := r.Seek(int64(size), io.SeekCurrent); err != nil {
				return fail()
			}
			counted, err := sim.CountValueNodes(encoded, maxSnapshotValueNodes-nodes)
			if err != nil {
				return fail()
			}
			nodes += counted
			value, err := sim.DecodeValue(encoded)
			if err != nil {
				return fail()
			}
			seed.Fields = append(seed.Fields, FieldSeed{Field: known.ids[j], Value: value})
		}
		if err := validateSeed(known.descriptor, seed.Fields); err != nil {
			return fail()
		}
		seeds = append(seeds, seed)
		prevType, prevEntity = typeID, entity
	}
	if r.Len() != 0 {
		return fail()
	}
	reader, authority, err := NewReader(registry, sim.WorldVersion(version), seeds)
	if err != nil {
		return fail()
	}
	return reader, authority, sim.WorldVersion(version), nil
}

func writeSnapshotNumber(b *bytes.Buffer, value any) {
	_ = binary.Write(b, binary.BigEndian, value) // fixed-width primitive into bytes.Buffer
}
