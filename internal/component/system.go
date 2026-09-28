package component

import (
	"agentworld/internal/sim"
	"fmt"
	"sort"
)

type FieldSeed struct {
	Field sim.FieldID
	Value sim.Value
}

type ComponentSeed struct {
	Entity    sim.EntityID
	Component sim.ComponentTypeID
	Fields    []FieldSeed
}

type componentStore interface {
	has(sim.EntityID) bool
	read(sim.EntityID, []sim.FieldID) ([]fieldValue, error)
	scan([]sim.FieldID) Batch
}

type system struct {
	registry Registry
	version  sim.WorldVersion
	stores   map[sim.ComponentTypeID]componentStore
}

// NewReader validates logical seed records and builds a one-version immutable
// component snapshot. The concrete system and stores are not returned.
func NewReader(registry Registry, version sim.WorldVersion, seeds []ComponentSeed) (Reader, Authority, error) {
	grouped := make(map[sim.ComponentTypeID][]ComponentSeed)
	seen := make(map[[2]uint64]struct{})
	for _, seed := range seeds {
		if sim.ValidateEntityID(seed.Entity) != nil {
			return nil, Authority{}, ErrInvalidRequest
		}
		descriptor, err := registry.Describe(seed.Component)
		if err != nil {
			return nil, Authority{}, err
		}
		key := [2]uint64{uint64(seed.Component), uint64(seed.Entity)}
		if _, ok := seen[key]; ok {
			return nil, Authority{}, fmt.Errorf("%w: duplicate component seed", ErrInvalidRequest)
		}
		seen[key] = struct{}{}
		if err := validateSeed(descriptor, seed.Fields); err != nil {
			return nil, Authority{}, err
		}
		grouped[seed.Component] = append(grouped[seed.Component], cloneSeed(seed))
	}
	s := &system{registry: registry, version: version, stores: make(map[sim.ComponentTypeID]componentStore)}
	for _, typeID := range registry.TypeIDs() {
		descriptor, _ := registry.Describe(typeID)
		var store componentStore
		var err error
		// This is the only physical-storage routing branch.
		switch descriptor.StorageClass {
		case BuiltinStorage:
			store, err = newEnergyStore(descriptor, grouped[typeID])
		case DynamicStorage:
			store, err = newDynamicStore(descriptor, grouped[typeID])
		default:
			err = ErrInvalidDescriptor
		}
		if err != nil {
			return nil, Authority{}, err
		}
		s.stores[typeID] = store
	}
	return Reader(s), Authority{owner: s}, nil
}

func (s *system) Describe(typeID sim.ComponentTypeID) (ComponentDescriptor, error) {
	descriptor, err := s.registry.Describe(typeID)
	if err != nil {
		return ComponentDescriptor{}, err
	}
	// Storage selection is bootstrap metadata, not part of the consumer view.
	descriptor.StorageClass = 0
	return descriptor, nil
}
func (s *system) Has(request HasRequest) (bool, error) {
	if err := s.authorize(request.Authority, request.WorldVersion); err != nil {
		return false, err
	}
	if sim.ValidateEntityID(request.Entity) != nil {
		return false, ErrInvalidRequest
	}
	store, ok := s.stores[request.Component]
	if !ok {
		return false, ErrUnknownComponentType
	}
	return store.has(request.Entity), nil
}
func (s *system) Read(request ReadRequest) (View, error) {
	if err := s.authorize(request.Authority, request.WorldVersion); err != nil {
		return View{}, err
	}
	if sim.ValidateEntityID(request.Entity) != nil {
		return View{}, ErrInvalidRequest
	}
	descriptor, fields, store, err := s.prepare(request.Component, request.Fields)
	if err != nil {
		return View{}, err
	}
	values, err := store.read(request.Entity, fields)
	if err != nil {
		return View{}, err
	}
	return View{entity: request.Entity, component: request.Component, worldVersion: s.version, schemaVersion: descriptor.SchemaVersion, fields: values}, nil
}
func (s *system) Scan(request ScanRequest) (Batch, error) {
	if err := s.authorize(request.Authority, request.WorldVersion); err != nil {
		return Batch{}, err
	}
	descriptor, fields, store, err := s.prepare(request.Component, request.Fields)
	if err != nil {
		return Batch{}, err
	}
	batch := store.scan(fields)
	batch.component = request.Component
	batch.worldVersion = s.version
	batch.schemaVersion = descriptor.SchemaVersion
	return batch, nil
}
func (s *system) Project(request ProjectRequest) (MetricBatch, error) {
	if err := s.authorize(request.Authority, request.WorldVersion); err != nil {
		return MetricBatch{}, err
	}
	descriptor, err := s.registry.Describe(request.Component)
	if err != nil {
		return MetricBatch{}, err
	}
	projection, err := findProjection(descriptor, request.Projection)
	if err != nil {
		return MetricBatch{}, err
	}
	store := s.stores[request.Component]
	entities := append([]sim.EntityID(nil), request.Entities...)
	if len(entities) == 0 {
		batch := store.scan(projection.SourceFields)
		entities = batch.EntityIDs()
	} else {
		for _, entity := range entities {
			if sim.ValidateEntityID(entity) != nil {
				return MetricBatch{}, ErrInvalidRequest
			}
		}
		sort.Slice(entities, func(i, j int) bool { return entities[i] < entities[j] })
		entities = dedupeEntities(entities)
	}
	metrics := make([]Metric, 0, len(entities))
	for _, entity := range entities {
		fields, readErr := store.read(entity, projection.SourceFields)
		if readErr != nil {
			return MetricBatch{}, readErr
		}
		metric, projectionErr := evaluateProjection(descriptor, projection, entity, s.version, fields)
		if projectionErr != nil {
			return MetricBatch{}, projectionErr
		}
		metrics = append(metrics, metric)
	}
	return MetricBatch{metrics: metrics}, nil
}

func (s *system) authorize(authority Authority, version sim.WorldVersion) error {
	if version != s.version {
		return ErrWorldVersionUnavailable
	}
	if authority.owner != s {
		return ErrUnauthorized
	}
	return nil
}
func (s *system) prepare(typeID sim.ComponentTypeID, requested []sim.FieldID) (ComponentDescriptor, []sim.FieldID, componentStore, error) {
	descriptor, err := s.registry.Describe(typeID)
	if err != nil {
		return ComponentDescriptor{}, nil, nil, err
	}
	fields, err := selectFields(descriptor, requested)
	if err != nil {
		return ComponentDescriptor{}, nil, nil, err
	}
	return descriptor, fields, s.stores[typeID], nil
}
func selectFields(descriptor ComponentDescriptor, requested []sim.FieldID) ([]sim.FieldID, error) {
	if len(requested) == 0 {
		fields := make([]sim.FieldID, len(descriptor.Fields))
		for i := range descriptor.Fields {
			fields[i] = descriptor.Fields[i].ID
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i] < fields[j] })
		return fields, nil
	}
	fields := append([]sim.FieldID(nil), requested...)
	sort.Slice(fields, func(i, j int) bool { return fields[i] < fields[j] })
	fields = dedupeFields(fields)
	for _, field := range fields {
		if _, err := findField(descriptor, field); err != nil {
			return nil, err
		}
	}
	return fields, nil
}
func dedupeFields(values []sim.FieldID) []sim.FieldID {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
func dedupeEntities(values []sim.EntityID) []sim.EntityID {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
func cloneSeed(seed ComponentSeed) ComponentSeed {
	seed.Fields = append([]FieldSeed(nil), seed.Fields...)
	return seed
}
