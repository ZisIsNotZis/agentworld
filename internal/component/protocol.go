package component

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
)

var (
	ErrWorldVersionUnavailable = errors.New("requested world version is unavailable")
	ErrComponentMissing        = errors.New("entity does not have component")
	ErrUnauthorized            = errors.New("component request is unauthorized")
	ErrInvalidRequest          = errors.New("invalid component request")
)

type Authority struct{ owner *system }

type HasRequest struct {
	Entity       sim.EntityID
	Component    sim.ComponentTypeID
	WorldVersion sim.WorldVersion
	Authority    Authority
}

type ReadRequest struct {
	Entity       sim.EntityID
	Component    sim.ComponentTypeID
	Fields       []sim.FieldID
	WorldVersion sim.WorldVersion
	Authority    Authority
}

type ScanRequest struct {
	Component    sim.ComponentTypeID
	Fields       []sim.FieldID
	WorldVersion sim.WorldVersion
	Authority    Authority
}

type ProjectRequest struct {
	Component    sim.ComponentTypeID
	Projection   sim.ProjectionID
	Entities     []sim.EntityID
	WorldVersion sim.WorldVersion
	Authority    Authority
}

type Reader interface {
	Describe(sim.ComponentTypeID) (ComponentDescriptor, error)
	Has(HasRequest) (bool, error)
	Read(ReadRequest) (View, error)
	Scan(ScanRequest) (Batch, error)
	Project(ProjectRequest) (MetricBatch, error)
}

type fieldValue struct {
	id    sim.FieldID
	value sim.Value
}

type View struct {
	entity        sim.EntityID
	component     sim.ComponentTypeID
	worldVersion  sim.WorldVersion
	schemaVersion sim.SchemaVersion
	fields        []fieldValue
}

func (v View) EntityID() sim.EntityID               { return v.entity }
func (v View) ComponentTypeID() sim.ComponentTypeID { return v.component }
func (v View) WorldVersion() sim.WorldVersion       { return v.worldVersion }
func (v View) SchemaVersion() sim.SchemaVersion     { return v.schemaVersion }
func (v View) FieldIDs() []sim.FieldID {
	out := make([]sim.FieldID, len(v.fields))
	for i := range v.fields {
		out[i] = v.fields[i].id
	}
	return out
}
func (v View) Value(field sim.FieldID) (sim.Value, error) {
	i := sort.Search(len(v.fields), func(i int) bool { return v.fields[i].id >= field })
	if i == len(v.fields) || v.fields[i].id != field {
		return sim.Value{}, ErrUnknownField
	}
	return v.fields[i].value, nil
}

type scalarColumn struct {
	values []float64
	states []sim.ValueState
}

type Batch struct {
	component     sim.ComponentTypeID
	worldVersion  sim.WorldVersion
	schemaVersion sim.SchemaVersion
	fields        []sim.FieldID
	entities      []sim.EntityID
	columns       [][]sim.Value
	scalarColumns map[sim.FieldID]scalarColumn
}

func (b Batch) ComponentTypeID() sim.ComponentTypeID { return b.component }
func (b Batch) WorldVersion() sim.WorldVersion       { return b.worldVersion }
func (b Batch) SchemaVersion() sim.SchemaVersion     { return b.schemaVersion }
func (b Batch) Len() int                             { return len(b.entities) }
func (b Batch) EntityIDs() []sim.EntityID            { return append([]sim.EntityID(nil), b.entities...) }
func (b Batch) FieldIDs() []sim.FieldID              { return append([]sim.FieldID(nil), b.fields...) }
func (b Batch) Value(row int, field sim.FieldID) (sim.Value, error) {
	if row < 0 || row >= len(b.entities) {
		return sim.Value{}, ErrInvalidRequest
	}
	column := sort.Search(len(b.fields), func(i int) bool { return b.fields[i] >= field })
	if column == len(b.fields) || b.fields[column] != field {
		return sim.Value{}, ErrUnknownField
	}
	return b.columns[column][row], nil
}

// CopyScalarColumn copies one complete scalar hot-path column without
// per-field interface dispatch. It returns ErrInvalidRequest if either output
// is too short and does not partially write.
func (b Batch) CopyScalarColumn(field sim.FieldID, values []float64, states []sim.ValueState) (int, error) {
	index := sort.Search(len(b.fields), func(i int) bool { return b.fields[i] >= field })
	if index == len(b.fields) || b.fields[index] != field {
		return 0, ErrUnknownField
	}
	column, ok := b.scalarColumns[field]
	if !ok {
		return 0, sim.ErrWrongKind
	}
	if len(values) < len(b.entities) || len(states) < len(b.entities) {
		return 0, ErrInvalidRequest
	}
	copy(values, column.values)
	copy(states, column.states)
	return len(b.entities), nil
}

type Provenance struct {
	Entity        sim.EntityID
	Component     sim.ComponentTypeID
	SourceFields  []sim.FieldID
	WorldVersion  sim.WorldVersion
	SchemaVersion sim.SchemaVersion
}

type Metric struct {
	value             sim.Value
	uncertainty       sim.Value
	projection        sim.ProjectionID
	projectionVersion sim.ProjectionVersion
	unit              string
	provenance        Provenance
}

func (m Metric) Value() sim.Value                         { return m.value }
func (m Metric) Uncertainty() sim.Value                   { return m.uncertainty }
func (m Metric) ProjectionID() sim.ProjectionID           { return m.projection }
func (m Metric) ProjectionVersion() sim.ProjectionVersion { return m.projectionVersion }
func (m Metric) Unit() string                             { return m.unit }
func (m Metric) Provenance() Provenance {
	p := m.provenance
	p.SourceFields = append([]sim.FieldID(nil), p.SourceFields...)
	return p
}

type MetricBatch struct{ metrics []Metric }

func (b MetricBatch) Len() int { return len(b.metrics) }
func (b MetricBatch) At(index int) (Metric, error) {
	if index < 0 || index >= len(b.metrics) {
		return Metric{}, ErrInvalidRequest
	}
	metric := b.metrics[index]
	metric.provenance.SourceFields = append([]sim.FieldID(nil), metric.provenance.SourceFields...)
	return metric, nil
}
func (b MetricBatch) EntityIDs() []sim.EntityID {
	out := make([]sim.EntityID, len(b.metrics))
	for i := range b.metrics {
		out[i] = b.metrics[i].provenance.Entity
	}
	return out
}
