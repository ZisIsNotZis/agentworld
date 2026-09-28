package component

import (
	"agentworld/internal/sim"
	"sort"
)

type energyStore struct {
	descriptor ComponentDescriptor
	entities   []sim.EntityID
	rows       map[sim.EntityID]int
	reserves   []float64
	states     []sim.ValueState
}

func newEnergyStore(descriptor ComponentDescriptor, seeds []ComponentSeed) (*energyStore, error) {
	if descriptor.TypeID != EnergyTypeID || len(descriptor.Fields) != 1 || descriptor.Fields[0].ID != EnergyReserveField || descriptor.Fields[0].Type.Kind != sim.ScalarKind {
		return nil, ErrInvalidDescriptor
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].Entity < seeds[j].Entity })
	store := &energyStore{descriptor: cloneDescriptor(descriptor), entities: make([]sim.EntityID, len(seeds)), rows: make(map[sim.EntityID]int, len(seeds)), reserves: make([]float64, len(seeds)), states: make([]sim.ValueState, len(seeds))}
	for row, seed := range seeds {
		store.entities[row] = seed.Entity
		store.rows[seed.Entity] = row
		store.states[row] = sim.Missing
		for _, field := range seed.Fields {
			store.states[row] = field.Value.State()
			if field.Value.IsPresent() {
				store.reserves[row], _ = field.Value.Scalar()
			}
		}
	}
	return store, nil
}
func (s *energyStore) has(entity sim.EntityID) bool { _, ok := s.rows[entity]; return ok }
func (s *energyStore) read(entity sim.EntityID, fields []sim.FieldID) ([]fieldValue, error) {
	row, ok := s.rows[entity]
	if !ok {
		return nil, ErrComponentMissing
	}
	out := make([]fieldValue, len(fields))
	for i, field := range fields {
		out[i] = fieldValue{id: field, value: s.value(row)}
	}
	return out, nil
}
func (s *energyStore) scan(fields []sim.FieldID) Batch {
	batch := Batch{fields: append([]sim.FieldID(nil), fields...), entities: append([]sim.EntityID(nil), s.entities...), columns: make([][]sim.Value, len(fields)), scalarColumns: make(map[sim.FieldID]scalarColumn, 1)}
	for column, field := range fields {
		values := make([]sim.Value, len(s.entities))
		for row := range values {
			values[row] = s.value(row)
		}
		batch.columns[column] = values
		batch.scalarColumns[field] = scalarColumn{values: append([]float64(nil), s.reserves...), states: append([]sim.ValueState(nil), s.states...)}
	}
	return batch
}
func (s *energyStore) value(row int) sim.Value {
	if s.states[row] == sim.Present {
		value, _ := sim.ScalarValue(s.reserves[row])
		return value
	}
	value, _ := sim.AbsentValue(sim.ScalarKind, s.states[row])
	return value
}
