package sim

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

type ValueState uint8

const (
	Present ValueState = iota
	Missing
	Unknown
	Inapplicable
)

type Kind uint8

const (
	BoolKind Kind = iota + 1
	IntegerKind
	ScalarKind
	ProbabilityKind
	EnumKind
	TimeKind
	DurationKind
	EntityRefKind
	EventRefKind
	VectorKind
	DistributionKind
	RecordKind
	OptionalKind
	ListKind
	SetKind
	SparseMapKind
)

var (
	ErrInvalidKind        = errors.New("invalid value kind")
	ErrInvalidState       = errors.New("invalid value state")
	ErrWrongKind          = errors.New("value has a different kind")
	ErrNonFinite          = errors.New("numeric value must be finite")
	ErrInvalidProbability = errors.New("probability must be in [0,1]")
	ErrDuplicateValue     = errors.New("duplicate canonical value")
)

type EnumValue struct {
	Enum  EnumID
	Value uint32
}

type RecordField struct {
	ID    FieldID
	Value Value
}

type DistributionPoint struct {
	Outcome     Value
	Probability float64
}

type SparseEntry struct {
	Key   Value
	Value Value
}

// Value is a closed, immutable typed value. Its representation is private;
// composite constructors and accessors copy their data defensively.
type Value struct {
	kind         Kind
	state        ValueState
	boolean      bool
	integer      int64
	scalar       float64
	enum         EnumValue
	time         SimTime
	duration     Duration
	entity       EntityID
	event        EventID
	vector       []float64
	distribution []DistributionPoint
	record       []RecordField
	elements     []Value
	sparse       []SparseEntry
}

func AbsentValue(kind Kind, state ValueState) (Value, error) {
	if !validKind(kind) {
		return Value{}, ErrInvalidKind
	}
	if state != Missing && state != Unknown && state != Inapplicable {
		return Value{}, ErrInvalidState
	}
	return Value{kind: kind, state: state}, nil
}

func BoolValue(v bool) Value     { return Value{kind: BoolKind, state: Present, boolean: v} }
func IntegerValue(v int64) Value { return Value{kind: IntegerKind, state: Present, integer: v} }
func TimeValue(v SimTime) (Value, error) {
	if v < 0 {
		return Value{}, ErrNegativeTime
	}
	return Value{kind: TimeKind, state: Present, time: v}, nil
}
func DurationValue(v Duration) (Value, error) {
	if v < 0 {
		return Value{}, ErrNegativeTime
	}
	return Value{kind: DurationKind, state: Present, duration: v}, nil
}
func EntityRefValue(v EntityID) (Value, error) {
	if err := ValidateEntityID(v); err != nil {
		return Value{}, err
	}
	return Value{kind: EntityRefKind, state: Present, entity: v}, nil
}
func EventRefValue(v EventID) (Value, error) {
	if err := ValidateEventID(v); err != nil {
		return Value{}, err
	}
	return Value{kind: EventRefKind, state: Present, event: v}, nil
}
func EnumTypedValue(v EnumValue) (Value, error) {
	if v.Enum == 0 {
		return Value{}, ErrInvalidID
	}
	return Value{kind: EnumKind, state: Present, enum: v}, nil
}
func ScalarValue(v float64) (Value, error) {
	if !finite(v) {
		return Value{}, ErrNonFinite
	}
	return Value{kind: ScalarKind, state: Present, scalar: canonicalZero(v)}, nil
}
func ProbabilityValue(v float64) (Value, error) {
	if !finite(v) || v < 0 || v > 1 {
		return Value{}, ErrInvalidProbability
	}
	return Value{kind: ProbabilityKind, state: Present, scalar: canonicalZero(v)}, nil
}
func VectorValue(values []float64) (Value, error) {
	out := append([]float64(nil), values...)
	for i, v := range out {
		if !finite(v) {
			return Value{}, ErrNonFinite
		}
		out[i] = canonicalZero(v)
	}
	return Value{kind: VectorKind, state: Present, vector: out}, nil
}
func DistributionValue(points []DistributionPoint) (Value, error) {
	out := make([]DistributionPoint, len(points))
	total := 0.0
	for i, point := range points {
		if !point.Outcome.valid() {
			return Value{}, ErrInvalidKind
		}
		if !finite(point.Probability) || point.Probability < 0 || point.Probability > 1 {
			return Value{}, ErrInvalidProbability
		}
		if point.Outcome.state != Present {
			return Value{}, ErrInvalidState
		}
		probability := canonicalZero(point.Probability)
		out[i] = DistributionPoint{Outcome: point.Outcome.clone(), Probability: probability}
		total += probability
	}
	if len(out) == 0 || !finite(total) || math.Abs(total-1) > 1e-12 {
		return Value{}, ErrInvalidProbability
	}
	sort.Slice(out, func(i, j int) bool { return canonicalKey(out[i].Outcome) < canonicalKey(out[j].Outcome) })
	for i := 1; i < len(out); i++ {
		if canonicalKey(out[i-1].Outcome) == canonicalKey(out[i].Outcome) {
			return Value{}, ErrDuplicateValue
		}
	}
	return Value{kind: DistributionKind, state: Present, distribution: out}, nil
}
func RecordValue(fields []RecordField) (Value, error) {
	out := make([]RecordField, len(fields))
	for i, field := range fields {
		if field.ID == 0 {
			return Value{}, ErrInvalidID
		}
		if !field.Value.valid() {
			return Value{}, ErrInvalidKind
		}
		out[i] = RecordField{ID: field.ID, Value: field.Value.clone()}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := 1; i < len(out); i++ {
		if out[i-1].ID == out[i].ID {
			return Value{}, ErrDuplicateValue
		}
	}
	return Value{kind: RecordKind, state: Present, record: out}, nil
}
func OptionalValue(value Value) (Value, error) {
	if !value.valid() {
		return Value{}, ErrInvalidKind
	}
	return Value{kind: OptionalKind, state: Present, elements: []Value{value.clone()}}, nil
}
func ListValue(values []Value) (Value, error) {
	for _, value := range values {
		if !value.valid() {
			return Value{}, ErrInvalidKind
		}
	}
	return Value{kind: ListKind, state: Present, elements: cloneValues(values)}, nil
}
func SetValue(values []Value) (Value, error) {
	for _, value := range values {
		if !value.valid() {
			return Value{}, ErrInvalidKind
		}
	}
	out := cloneValues(values)
	sort.Slice(out, func(i, j int) bool { return canonicalKey(out[i]) < canonicalKey(out[j]) })
	for i := 1; i < len(out); i++ {
		if canonicalKey(out[i-1]) == canonicalKey(out[i]) {
			return Value{}, ErrDuplicateValue
		}
	}
	return Value{kind: SetKind, state: Present, elements: out}, nil
}
func SparseMapValue(entries []SparseEntry) (Value, error) {
	out := make([]SparseEntry, len(entries))
	for i, entry := range entries {
		if !entry.Value.valid() {
			return Value{}, ErrInvalidKind
		}
		if !isScalarKey(entry.Key) {
			return Value{}, ErrWrongKind
		}
		out[i] = SparseEntry{Key: entry.Key.clone(), Value: entry.Value.clone()}
	}
	sort.Slice(out, func(i, j int) bool { return canonicalKey(out[i].Key) < canonicalKey(out[j].Key) })
	for i := 1; i < len(out); i++ {
		if canonicalKey(out[i-1].Key) == canonicalKey(out[i].Key) {
			return Value{}, ErrDuplicateValue
		}
	}
	return Value{kind: SparseMapKind, state: Present, sparse: out}, nil
}

func (v Value) Kind() Kind        { return v.kind }
func (v Value) State() ValueState { return v.state }
func (v Value) IsPresent() bool   { return v.state == Present }
func (v Value) Bool() (bool, error) {
	if err := v.require(BoolKind); err != nil {
		return false, err
	}
	return v.boolean, nil
}
func (v Value) Integer() (int64, error) {
	if err := v.require(IntegerKind); err != nil {
		return 0, err
	}
	return v.integer, nil
}
func (v Value) Scalar() (float64, error) {
	if v.kind != ScalarKind && v.kind != ProbabilityKind {
		return 0, ErrWrongKind
	}
	if v.state != Present {
		return 0, ErrInvalidState
	}
	return v.scalar, nil
}
func (v Value) Enum() (EnumValue, error) {
	if err := v.require(EnumKind); err != nil {
		return EnumValue{}, err
	}
	return v.enum, nil
}
func (v Value) Time() (SimTime, error) {
	if err := v.require(TimeKind); err != nil {
		return 0, err
	}
	return v.time, nil
}
func (v Value) Duration() (Duration, error) {
	if err := v.require(DurationKind); err != nil {
		return 0, err
	}
	return v.duration, nil
}
func (v Value) EntityRef() (EntityID, error) {
	if err := v.require(EntityRefKind); err != nil {
		return 0, err
	}
	return v.entity, nil
}
func (v Value) EventRef() (EventID, error) {
	if err := v.require(EventRefKind); err != nil {
		return 0, err
	}
	return v.event, nil
}
func (v Value) Vector() ([]float64, error) {
	if err := v.require(VectorKind); err != nil {
		return nil, err
	}
	return append([]float64(nil), v.vector...), nil
}
func (v Value) Distribution() ([]DistributionPoint, error) {
	if err := v.require(DistributionKind); err != nil {
		return nil, err
	}
	return cloneDistribution(v.distribution), nil
}
func (v Value) Record() ([]RecordField, error) {
	if err := v.require(RecordKind); err != nil {
		return nil, err
	}
	return cloneRecord(v.record), nil
}
func (v Value) Optional() (Value, error) {
	if err := v.require(OptionalKind); err != nil {
		return Value{}, err
	}
	return v.elements[0].clone(), nil
}
func (v Value) List() ([]Value, error) {
	if err := v.require(ListKind); err != nil {
		return nil, err
	}
	return cloneValues(v.elements), nil
}
func (v Value) Set() ([]Value, error) {
	if err := v.require(SetKind); err != nil {
		return nil, err
	}
	return cloneValues(v.elements), nil
}
func (v Value) SparseMap() ([]SparseEntry, error) {
	if err := v.require(SparseMapKind); err != nil {
		return nil, err
	}
	return cloneSparse(v.sparse), nil
}

func (v Value) Equal(other Value) bool { return canonicalKey(v) == canonicalKey(other) }

func (v Value) require(kind Kind) error {
	if v.kind != kind {
		return ErrWrongKind
	}
	if v.state != Present {
		return ErrInvalidState
	}
	return nil
}

func (v Value) clone() Value {
	v.vector = append([]float64(nil), v.vector...)
	v.distribution = cloneDistribution(v.distribution)
	v.record = cloneRecord(v.record)
	v.elements = cloneValues(v.elements)
	v.sparse = cloneSparse(v.sparse)
	return v
}

func cloneValues(values []Value) []Value {
	out := make([]Value, len(values))
	for i := range values {
		out[i] = values[i].clone()
	}
	return out
}
func cloneDistribution(values []DistributionPoint) []DistributionPoint {
	out := make([]DistributionPoint, len(values))
	for i, value := range values {
		out[i] = DistributionPoint{Outcome: value.Outcome.clone(), Probability: value.Probability}
	}
	return out
}
func cloneRecord(values []RecordField) []RecordField {
	out := make([]RecordField, len(values))
	for i, value := range values {
		out[i] = RecordField{ID: value.ID, Value: value.Value.clone()}
	}
	return out
}
func cloneSparse(values []SparseEntry) []SparseEntry {
	out := make([]SparseEntry, len(values))
	for i, value := range values {
		out[i] = SparseEntry{Key: value.Key.clone(), Value: value.Value.clone()}
	}
	return out
}

func canonicalKey(v Value) string {
	base := fmt.Sprintf("%02d:%d:", v.kind, v.state)
	if v.state != Present {
		return base
	}
	switch v.kind {
	case BoolKind:
		return fmt.Sprintf("%s%t", base, v.boolean)
	case IntegerKind:
		return fmt.Sprintf("%s%020d", base, v.integer)
	case ScalarKind, ProbabilityKind:
		return fmt.Sprintf("%s%016x", base, math.Float64bits(v.scalar))
	case EnumKind:
		return fmt.Sprintf("%s%d:%d", base, v.enum.Enum, v.enum.Value)
	case TimeKind:
		return fmt.Sprintf("%s%d", base, v.time)
	case DurationKind:
		return fmt.Sprintf("%s%d", base, v.duration)
	case EntityRefKind:
		return fmt.Sprintf("%s%d", base, v.entity)
	case EventRefKind:
		return fmt.Sprintf("%s%d", base, v.event)
	case VectorKind:
		for _, x := range v.vector {
			base += fmt.Sprintf("%016x,", math.Float64bits(x))
		}
	case DistributionKind:
		for _, x := range v.distribution {
			base += canonicalKey(x.Outcome) + fmt.Sprintf("@%016x,", math.Float64bits(x.Probability))
		}
	case RecordKind:
		for _, x := range v.record {
			base += fmt.Sprintf("%d=", x.ID) + canonicalKey(x.Value) + ","
		}
	case OptionalKind, ListKind, SetKind:
		for _, x := range v.elements {
			base += canonicalKey(x) + ","
		}
	case SparseMapKind:
		for _, x := range v.sparse {
			base += canonicalKey(x.Key) + "=" + canonicalKey(x.Value) + ","
		}
	}
	return base
}

func isScalarKey(v Value) bool {
	if v.state != Present {
		return false
	}
	switch v.kind {
	case BoolKind, IntegerKind, ScalarKind, ProbabilityKind, EnumKind, TimeKind, DurationKind, EntityRefKind, EventRefKind:
		return true
	default:
		return false
	}
}
func (v Value) valid() bool {
	return validKind(v.kind) && v.state <= Inapplicable
}
func validKind(kind Kind) bool { return kind >= BoolKind && kind <= SparseMapKind }
func canonicalZero(v float64) float64 {
	if v == 0 {
		return 0
	}
	return v
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
