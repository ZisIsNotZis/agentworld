package component

import (
	"agentworld/internal/sim"
	"sort"
)

type dynamicStore struct {
	descriptor ComponentDescriptor
	entities   []sim.EntityID
	records    map[sim.EntityID]map[sim.FieldID]sim.Value
}

func newDynamicStore(descriptor ComponentDescriptor, seeds []ComponentSeed) (*dynamicStore, error) {
	store := &dynamicStore{descriptor: cloneDescriptor(descriptor), records: make(map[sim.EntityID]map[sim.FieldID]sim.Value, len(seeds))}
	for _, seed := range seeds {
		record := make(map[sim.FieldID]sim.Value, len(seed.Fields))
		for _, field := range seed.Fields {
			record[field.Field] = field.Value
		}
		store.records[seed.Entity] = record
		store.entities = append(store.entities, seed.Entity)
	}
	sort.Slice(store.entities, func(i, j int) bool { return store.entities[i] < store.entities[j] })
	return store, nil
}
func (s *dynamicStore) has(entity sim.EntityID) bool { _, ok := s.records[entity]; return ok }
func (s *dynamicStore) read(entity sim.EntityID, fields []sim.FieldID) ([]fieldValue, error) {
	record, ok := s.records[entity]
	if !ok {
		return nil, ErrComponentMissing
	}
	out := make([]fieldValue, len(fields))
	for i, fieldID := range fields {
		value, ok := record[fieldID]
		if !ok {
			descriptor, _ := findField(s.descriptor, fieldID)
			value, _ = sim.AbsentValue(descriptor.Type.Kind, sim.Missing)
		}
		out[i] = fieldValue{id: fieldID, value: value}
	}
	return out, nil
}
func (s *dynamicStore) scan(fields []sim.FieldID) Batch {
	batch := Batch{fields: append([]sim.FieldID(nil), fields...), entities: append([]sim.EntityID(nil), s.entities...), columns: make([][]sim.Value, len(fields)), scalarColumns: make(map[sim.FieldID]scalarColumn)}
	for column, fieldID := range fields {
		batch.columns[column] = make([]sim.Value, len(s.entities))
		field, _ := findField(s.descriptor, fieldID)
		var scalar scalarColumn
		if field.Type.Kind == sim.ScalarKind {
			scalar = scalarColumn{values: make([]float64, len(s.entities)), states: make([]sim.ValueState, len(s.entities))}
		}
		for row, entity := range s.entities {
			value, ok := s.records[entity][fieldID]
			if !ok {
				value, _ = sim.AbsentValue(field.Type.Kind, sim.Missing)
			}
			batch.columns[column][row] = value
			if scalar.states != nil {
				scalar.states[row] = value.State()
				if value.IsPresent() {
					scalar.values[row], _ = value.Scalar()
				}
			}
		}
		if scalar.states != nil {
			batch.scalarColumns[fieldID] = scalar
		}
	}
	return batch
}
