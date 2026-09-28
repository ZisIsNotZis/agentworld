package component

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
)

var ErrUnknownRule = errors.New("unknown transition rule")
var ErrSchemaVersion = errors.New("transition schema version mismatch")

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

// Stage validates replacements and returns a new immutable snapshot. It never
// changes its input. Publishing a staged snapshot is reserved to the kernel.
func Stage(reader Reader, authority Authority, version sim.WorldVersion, rule sim.RuleID, ruleVersion uint32, patches []Patch) (Reader, Authority, []FieldDelta, []MetricDelta, error) {
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
	deltas := make([]FieldDelta, 0, len(ordered))
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
