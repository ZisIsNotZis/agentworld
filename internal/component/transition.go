package component

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
)

var ErrUnknownRule = errors.New("unknown transition rule")
var ErrSchemaVersion = errors.New("transition schema version mismatch")

// Allocation staging failures are typed so callers can distinguish an entity
// ID collision, an incomplete newborn row, a duplicated seed, and a patch
// that reaches into an entity born in the same transition.
var (
	ErrEntityExists        = errors.New("allocated entity already has component rows")
	ErrPartialAllocation   = errors.New("allocation seed is not a complete component row")
	ErrDuplicateAllocation = errors.New("duplicate allocation seed")
	ErrAllocationOverlap   = errors.New("patch targets an entity allocated by the same transition")
)

type Patch struct {
	Entity        sim.EntityID
	Component     sim.ComponentTypeID
	SchemaVersion sim.SchemaVersion
	Field         sim.FieldID
	Value         sim.Value
}

type FieldDelta struct {
	Entity        sim.EntityID
	Component     sim.ComponentTypeID
	SchemaVersion sim.SchemaVersion
	Field         sim.FieldID
	Before, After sim.Value
}

type MetricDelta struct {
	Before, After Metric
}

// Stage validates replacements and newborn rows and returns a new immutable
// snapshot. It never changes its input. Publishing a staged snapshot is
// reserved to the kernel. Allocations create complete rows for entities that
// do not exist yet; patches and allocations share one version bump, and any
// error returns without touching the input snapshot.
func Stage(reader Reader, authority Authority, version sim.WorldVersion, rule sim.RuleID, ruleVersion uint32, patches []Patch, allocations []ComponentSeed) (Reader, Authority, []FieldDelta, []MetricDelta, error) {
	s, ok := reader.(*system)
	if !ok {
		return nil, Authority{}, nil, nil, ErrInvalidRequest
	}
	if err := s.authorize(authority, version); err != nil {
		return nil, Authority{}, nil, nil, err
	}
	if version == ^sim.WorldVersion(0) || len(patches) == 0 || rule == 0 || ruleVersion == 0 {
		return nil, Authority{}, nil, nil, ErrInvalidRequest
	}
	ordered := append([]Patch(nil), patches...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		if a.Component != b.Component {
			return a.Component < b.Component
		}
		return a.Field < b.Field
	})
	next := &system{registry: s.registry, version: version + 1, stores: make(map[sim.ComponentTypeID]componentStore, len(s.stores))}
	for id, store := range s.stores {
		next.stores[id] = store
	}
	allocDeltas, err := stageAllocations(s, next, rule, ruleVersion, ordered, allocations)
	if err != nil {
		return nil, Authority{}, nil, nil, err
	}
	deltas := make([]FieldDelta, 0, len(allocDeltas)+len(ordered))
	deltas = append(deltas, allocDeltas...)
	for i, p := range ordered {
		if sim.ValidateEntityID(p.Entity) != nil || p.Field == 0 || (i > 0 && p.Entity == ordered[i-1].Entity && p.Component == ordered[i-1].Component && p.Field == ordered[i-1].Field) {
			return nil, Authority{}, nil, nil, ErrInvalidRequest
		}
		descriptor, err := s.registry.Describe(p.Component)
		if err != nil {
			return nil, Authority{}, nil, nil, err
		}
		if descriptor.SchemaVersion != p.SchemaVersion {
			return nil, Authority{}, nil, nil, ErrSchemaVersion
		}
		found := false
		for _, r := range descriptor.TransitionRules {
			if r.ID == rule && r.Version == ruleVersion {
				found = true
				break
			}
		}
		if !found {
			return nil, Authority{}, nil, nil, ErrUnknownRule
		}
		field, err := findField(descriptor, p.Field)
		if err != nil {
			return nil, Authority{}, nil, nil, err
		}
		if err = validateValue(field, p.Value); err != nil {
			return nil, Authority{}, nil, nil, err
		}
		store := next.stores[p.Component]
		before, err := store.read(p.Entity, []sim.FieldID{p.Field})
		if err != nil {
			return nil, Authority{}, nil, nil, err
		}
		// Copy a store only when its first field is replaced in this transition.
		if next.stores[p.Component] == s.stores[p.Component] {
			switch old := store.(type) {
			case *energyStore:
				copyStore := *old
				copyStore.reserves = append([]float64(nil), old.reserves...)
				copyStore.states = append([]sim.ValueState(nil), old.states...)
				next.stores[p.Component] = &copyStore
			case *dynamicStore:
				copyStore := *old
				copyStore.records = make(map[sim.EntityID]map[sim.FieldID]sim.Value, len(old.records))
				for id, record := range old.records {
					copyStore.records[id] = record
				}
				next.stores[p.Component] = &copyStore
			default:
				return nil, Authority{}, nil, nil, ErrInvalidRequest
			}
		}
		switch writable := next.stores[p.Component].(type) {
		case *energyStore:
			row := writable.rows[p.Entity]
			writable.states[row] = p.Value.State()
			writable.reserves[row] = 0
			if p.Value.IsPresent() {
				writable.reserves[row], _ = p.Value.Scalar()
			}
		case *dynamicStore:
			old := writable.records[p.Entity]
			record := make(map[sim.FieldID]sim.Value, len(old)+1)
			for id, value := range old {
				record[id] = value
			}
			record[p.Field] = p.Value
			writable.records[p.Entity] = record
		}
		deltas = append(deltas, FieldDelta{p.Entity, p.Component, p.SchemaVersion, p.Field, before[0].value, p.Value})
	}
	metrics := make([]MetricDelta, 0)
	// Iteration follows sorted patches, with each entity/component visited once.
	visited := make(map[[2]uint64]struct{})
	for _, p := range ordered {
		key := [2]uint64{uint64(p.Entity), uint64(p.Component)}
		if _, ok := visited[key]; ok {
			continue
		}
		visited[key] = struct{}{}
		descriptor, _ := s.registry.Describe(p.Component)
		projections := append([]ProjectionDescriptor(nil), descriptor.Projections...)
		sort.Slice(projections, func(i, j int) bool { return projections[i].ID < projections[j].ID })
		for _, projection := range projections {
			relevant := false
			for _, source := range projection.SourceFields {
				for _, patch := range ordered {
					if patch.Entity == p.Entity && patch.Component == p.Component && patch.Field == source {
						relevant = true
					}
				}
			}
			if !relevant {
				continue
			}
			oldFields, err := s.stores[p.Component].read(p.Entity, projection.SourceFields)
			if err != nil {
				return nil, Authority{}, nil, nil, err
			}
			newFields, err := next.stores[p.Component].read(p.Entity, projection.SourceFields)
			if err != nil {
				return nil, Authority{}, nil, nil, err
			}
			before, err := evaluateProjection(descriptor, projection, p.Entity, version, oldFields)
			if err != nil {
				return nil, Authority{}, nil, nil, err
			}
			after, err := evaluateProjection(descriptor, projection, p.Entity, version+1, newFields)
			if err != nil {
				return nil, Authority{}, nil, nil, err
			}
			metrics = append(metrics, MetricDelta{before, after})
		}
	}
	return next, Authority{owner: next}, deltas, metrics, nil
}

// stageAllocations validates every allocation seed against the current
// snapshot, inserts complete rows into copy-on-write store copies on next,
// and returns the newborns' absent-before deltas in canonical
// (entity, component, field) order. The rows exist only on next; the input
// stores are never written.
func stageAllocations(s *system, next *system, rule sim.RuleID, ruleVersion uint32, patches []Patch, allocations []ComponentSeed) ([]FieldDelta, error) {
	if len(allocations) == 0 {
		return nil, nil
	}
	type planned struct {
		seed       ComponentSeed
		descriptor ComponentDescriptor
		fields     []FieldSeed
	}
	plans := make([]planned, 0, len(allocations))
	seenSeeds := make(map[[2]uint64]struct{}, len(allocations))
	created := make(map[sim.EntityID]struct{}, len(allocations))
	for _, seed := range allocations {
		if sim.ValidateEntityID(seed.Entity) != nil {
			return nil, ErrInvalidRequest
		}
		descriptor, err := s.registry.Describe(seed.Component)
		if err != nil {
			return nil, err
		}
		key := [2]uint64{uint64(seed.Entity), uint64(seed.Component)}
		if _, ok := seenSeeds[key]; ok {
			return nil, ErrDuplicateAllocation
		}
		seenSeeds[key] = struct{}{}
		created[seed.Entity] = struct{}{}
		// A length match plus a clean validateSeed means the row covers every
		// schema field: validateSeed rejects unknown and duplicate fields.
		if len(seed.Fields) != len(descriptor.Fields) {
			return nil, ErrPartialAllocation
		}
		fields := append([]FieldSeed(nil), seed.Fields...)
		sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
		if err := validateSeed(descriptor, fields); err != nil {
			return nil, err
		}
		authorized := false
		for _, r := range descriptor.TransitionRules {
			if r.ID == rule && r.Version == ruleVersion {
				authorized = true
				break
			}
		}
		if !authorized {
			return nil, ErrUnknownRule
		}
		plans = append(plans, planned{seed: seed, descriptor: descriptor, fields: fields})
	}
	// An allocated ID must be new across every registered component, not just
	// the seeded ones.
	for entity := range created {
		for _, typeID := range s.registry.TypeIDs() {
			if s.stores[typeID].has(entity) {
				return nil, ErrEntityExists
			}
		}
	}
	// Patches address pre-existing rows only; keeping the two disjoint makes
	// the event's absent-before boundary exact for replay.
	for _, p := range patches {
		if _, ok := created[p.Entity]; ok {
			return nil, ErrAllocationOverlap
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].seed.Entity != plans[j].seed.Entity {
			return plans[i].seed.Entity < plans[j].seed.Entity
		}
		return plans[i].seed.Component < plans[j].seed.Component
	})
	total := 0
	for _, plan := range plans {
		total += len(plan.fields)
	}
	deltas := make([]FieldDelta, 0, total)
	for _, plan := range plans {
		if err := stageAllocationRow(s, next, plan.seed, plan.descriptor, plan.fields); err != nil {
			return nil, err
		}
		for _, field := range plan.fields {
			fieldDescriptor, err := findField(plan.descriptor, field.Field)
			if err != nil {
				return nil, err
			}
			before, err := sim.AbsentValue(fieldDescriptor.Type.Kind, sim.Missing)
			if err != nil {
				return nil, err
			}
			deltas = append(deltas, FieldDelta{plan.seed.Entity, plan.seed.Component, plan.descriptor.SchemaVersion, field.Field, before, field.Value})
		}
	}
	return deltas, nil
}

// stageAllocationRow copies the component's store on first write and inserts
// one complete row for the new entity, keeping entity order ascending for
// canonical scans and snapshots.
func stageAllocationRow(s *system, next *system, seed ComponentSeed, descriptor ComponentDescriptor, fields []FieldSeed) error {
	switch writable := next.stores[seed.Component].(type) {
	case *energyStore:
		if next.stores[seed.Component] == s.stores[seed.Component] {
			copied := *writable
			// Rows are addressed through the map, so the insertion needs its
			// own; value arrays are replaced wholesale by insertAt below.
			copied.rows = make(map[sim.EntityID]int, len(writable.rows)+1)
			for entity, row := range writable.rows {
				copied.rows[entity] = row
			}
			next.stores[seed.Component] = &copied
			writable = &copied
		}
		pos := sort.Search(len(writable.entities), func(i int) bool { return writable.entities[i] > seed.Entity })
		writable.entities = insertAt(writable.entities, pos, seed.Entity)
		writable.reserves = insertAt(writable.reserves, pos, 0)
		writable.states = insertAt(writable.states, pos, sim.Missing)
		for entity, row := range writable.rows {
			if row >= pos {
				writable.rows[entity] = row + 1
			}
		}
		writable.rows[seed.Entity] = pos
		for _, field := range fields {
			writable.states[pos] = field.Value.State()
			if field.Value.IsPresent() {
				writable.reserves[pos], _ = field.Value.Scalar()
			}
		}
	case *dynamicStore:
		if next.stores[seed.Component] == s.stores[seed.Component] {
			copied := *writable
			copied.records = make(map[sim.EntityID]map[sim.FieldID]sim.Value, len(writable.records)+1)
			for entity, record := range writable.records {
				copied.records[entity] = record
			}
			next.stores[seed.Component] = &copied
			writable = &copied
		}
		pos := sort.Search(len(writable.entities), func(i int) bool { return writable.entities[i] > seed.Entity })
		writable.entities = insertAt(writable.entities, pos, seed.Entity)
		record := make(map[sim.FieldID]sim.Value, len(fields))
		for _, field := range fields {
			record[field.Field] = field.Value
		}
		writable.records[seed.Entity] = record
	default:
		return ErrInvalidRequest
	}
	return nil
}

// insertAt returns a fresh slice with item spliced in at pos; the input is
// never written, so shared store arrays stay immutable.
func insertAt[T any](values []T, pos int, item T) []T {
	out := make([]T, len(values)+1)
	copy(out, values[:pos])
	out[pos] = item
	copy(out[pos+1:], values[pos:])
	return out
}
