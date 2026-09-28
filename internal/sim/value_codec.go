package sim

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

var ErrValueEncoding = errors.New("invalid or oversized value encoding")

const MaxValueBytes = 1 << 20
const maxValueDepth = 32
const maxValueItems = 4096

// EncodeValue uses a canonical, bounded, big-endian encoding of the closed Value algebra.
func EncodeValue(v Value) ([]byte, error) {
	var b bytes.Buffer
	if err := encodeValue(&b, v, 0); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func encodeValue(b *bytes.Buffer, v Value, depth int) error {
	if depth >= maxValueDepth || !v.valid() || b.Len() > MaxValueBytes {
		return ErrValueEncoding
	}
	b.WriteByte(byte(v.kind))
	b.WriteByte(byte(v.state))
	if v.state != Present {
		return nil
	}
	put := func(x any) { _ = binary.Write(b, binary.BigEndian, x) }
	count := func(n int) error {
		if n > maxValueItems {
			return ErrValueEncoding
		}
		put(uint32(n))
		return nil
	}
	child := func(x Value) error { return encodeValue(b, x, depth+1) }
	switch v.kind {
	case BoolKind:
		if v.boolean {
			b.WriteByte(1)
		} else {
			b.WriteByte(0)
		}
	case IntegerKind:
		put(v.integer)
	case ScalarKind, ProbabilityKind:
		put(math.Float64bits(v.scalar))
	case EnumKind:
		put(uint32(v.enum.Enum))
		put(v.enum.Value)
	case TimeKind:
		put(int64(v.time))
	case DurationKind:
		put(int64(v.duration))
	case EntityRefKind:
		put(uint64(v.entity))
	case EventRefKind:
		put(uint64(v.event))
	case VectorKind:
		if err := count(len(v.vector)); err != nil {
			return err
		}
		for _, x := range v.vector {
			put(math.Float64bits(x))
		}
	case DistributionKind:
		if err := count(len(v.distribution)); err != nil {
			return err
		}
		for _, x := range v.distribution {
			if err := child(x.Outcome); err != nil {
				return err
			}
			put(math.Float64bits(x.Probability))
		}
	case RecordKind:
		if err := count(len(v.record)); err != nil {
			return err
		}
		for _, x := range v.record {
			put(uint32(x.ID))
			if err := child(x.Value); err != nil {
				return err
			}
		}
	case OptionalKind:
		if err := child(v.elements[0]); err != nil {
			return err
		}
	case ListKind, SetKind:
		if err := count(len(v.elements)); err != nil {
			return err
		}
		for _, x := range v.elements {
			if err := child(x); err != nil {
				return err
			}
		}
	case SparseMapKind:
		if err := count(len(v.sparse)); err != nil {
			return err
		}
		for _, x := range v.sparse {
			if err := child(x.Key); err != nil {
				return err
			}
			if err := child(x.Value); err != nil {
				return err
			}
		}
	default:
		return ErrValueEncoding
	}
	if b.Len() > MaxValueBytes {
		return ErrValueEncoding
	}
	return nil
}

// CountValueNodes scans the bounded value wire grammar without constructing
// Values. It includes the root and all nested values, fails once maxNodes is
// exceeded, and checks framing/depth; DecodeValue still verifies canonical
// ordering, scalar validity, and the remaining semantic constraints.
func CountValueNodes(data []byte, maxNodes uint64) (uint64, error) {
	if len(data) > MaxValueBytes || maxNodes == 0 {
		return 0, ErrValueEncoding
	}
	s := valueNodeScanner{data: data, remaining: maxNodes}
	if err := s.scan(0); err != nil || s.offset != len(data) {
		return 0, ErrValueEncoding
	}
	return maxNodes - s.remaining, nil
}

type valueNodeScanner struct {
	data      []byte
	offset    int
	remaining uint64
}

func (s *valueNodeScanner) skip(n int) error {
	if n > len(s.data)-s.offset {
		return ErrValueEncoding
	}
	s.offset += n
	return nil
}

func (s *valueNodeScanner) count() (int, error) {
	if err := s.skip(4); err != nil {
		return 0, err
	}
	n := binary.BigEndian.Uint32(s.data[s.offset-4 : s.offset])
	if n > maxValueItems {
		return 0, ErrValueEncoding
	}
	return int(n), nil
}

func (s *valueNodeScanner) scan(depth int) error {
	if depth >= maxValueDepth || s.remaining == 0 || s.skip(2) != nil {
		return ErrValueEncoding
	}
	s.remaining--
	kind, state := Kind(s.data[s.offset-2]), ValueState(s.data[s.offset-1])
	if !validKind(kind) || state > Inapplicable {
		return ErrValueEncoding
	}
	if state != Present {
		return nil
	}
	switch kind {
	case BoolKind:
		return s.skip(1)
	case IntegerKind, ScalarKind, ProbabilityKind, EnumKind, TimeKind, DurationKind, EntityRefKind, EventRefKind:
		return s.skip(8)
	case VectorKind:
		n, err := s.count()
		if err != nil {
			return err
		}
		return s.skip(n * 8)
	case DistributionKind:
		n, err := s.count()
		if err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			if err := s.scan(depth + 1); err != nil {
				return err
			}
			if err := s.skip(8); err != nil {
				return err
			}
		}
	case RecordKind:
		n, err := s.count()
		if err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			if err := s.skip(4); err != nil {
				return err
			}
			if err := s.scan(depth + 1); err != nil {
				return err
			}
		}
	case OptionalKind:
		return s.scan(depth + 1)
	case ListKind, SetKind:
		n, err := s.count()
		if err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			if err := s.scan(depth + 1); err != nil {
				return err
			}
		}
	case SparseMapKind:
		n, err := s.count()
		if err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			if err := s.scan(depth + 1); err != nil {
				return err
			}
			if err := s.scan(depth + 1); err != nil {
				return err
			}
		}
	default:
		return ErrValueEncoding
	}
	return nil
}

func DecodeValue(data []byte) (Value, error) {
	if len(data) > MaxValueBytes {
		return Value{}, ErrValueEncoding
	}
	r := bytes.NewReader(data)
	v, err := decodeValue(r, 0)
	if err != nil || r.Len() != 0 {
		return Value{}, ErrValueEncoding
	}
	canonical, err := EncodeValue(v)
	if err != nil || !bytes.Equal(canonical, data) {
		return Value{}, ErrValueEncoding
	}
	return v, nil
}
func decodeValue(r *bytes.Reader, depth int) (Value, error) {
	fail := func() (Value, error) { return Value{}, ErrValueEncoding }
	if depth >= maxValueDepth {
		return fail()
	}
	kindByte, err := r.ReadByte()
	if err != nil {
		return fail()
	}
	stateByte, err := r.ReadByte()
	if err != nil {
		return fail()
	}
	kind, state := Kind(kindByte), ValueState(stateByte)
	if !validKind(kind) || state > Inapplicable {
		return fail()
	}
	if state != Present {
		return AbsentValue(kind, state)
	}
	get := func(x any) error { return binary.Read(r, binary.BigEndian, x) }
	length := func() (int, error) {
		var n uint32
		if get(&n) != nil || n > maxValueItems {
			return 0, ErrValueEncoding
		}
		return int(n), nil
	}
	child := func() (Value, error) { return decodeValue(r, depth+1) }
	switch kind {
	case BoolKind:
		x, err := r.ReadByte()
		if err != nil || x > 1 {
			return fail()
		}
		return BoolValue(x == 1), nil
	case IntegerKind:
		var x int64
		if get(&x) != nil {
			return fail()
		}
		return IntegerValue(x), nil
	case ScalarKind, ProbabilityKind:
		var bits uint64
		if get(&bits) != nil {
			return fail()
		}
		if kind == ScalarKind {
			return ScalarValue(math.Float64frombits(bits))
		}
		return ProbabilityValue(math.Float64frombits(bits))
	case EnumKind:
		var id, value uint32
		if get(&id) != nil || get(&value) != nil {
			return fail()
		}
		return EnumTypedValue(EnumValue{Enum: EnumID(id), Value: value})
	case TimeKind:
		var x int64
		if get(&x) != nil {
			return fail()
		}
		return TimeValue(SimTime(x))
	case DurationKind:
		var x int64
		if get(&x) != nil {
			return fail()
		}
		return DurationValue(Duration(x))
	case EntityRefKind:
		var x uint64
		if get(&x) != nil {
			return fail()
		}
		return EntityRefValue(EntityID(x))
	case EventRefKind:
		var x uint64
		if get(&x) != nil {
			return fail()
		}
		return EventRefValue(EventID(x))
	case VectorKind:
		n, err := length()
		if err != nil {
			return fail()
		}
		xs := make([]float64, n)
		for i := range xs {
			var bits uint64
			if get(&bits) != nil {
				return fail()
			}
			xs[i] = math.Float64frombits(bits)
		}
		return VectorValue(xs)
	case DistributionKind:
		n, err := length()
		if err != nil {
			return fail()
		}
		xs := make([]DistributionPoint, n)
		for i := range xs {
			v, e := child()
			if e != nil {
				return fail()
			}
			var bits uint64
			if get(&bits) != nil {
				return fail()
			}
			xs[i] = DistributionPoint{v, math.Float64frombits(bits)}
		}
		return DistributionValue(xs)
	case RecordKind:
		n, err := length()
		if err != nil {
			return fail()
		}
		xs := make([]RecordField, n)
		for i := range xs {
			var id uint32
			if get(&id) != nil {
				return fail()
			}
			v, e := child()
			if e != nil {
				return fail()
			}
			xs[i] = RecordField{FieldID(id), v}
		}
		return RecordValue(xs)
	case OptionalKind:
		v, err := child()
		if err != nil {
			return fail()
		}
		return OptionalValue(v)
	case ListKind, SetKind:
		n, err := length()
		if err != nil {
			return fail()
		}
		xs := make([]Value, n)
		for i := range xs {
			xs[i], err = child()
			if err != nil {
				return fail()
			}
		}
		if kind == ListKind {
			return ListValue(xs)
		}
		return SetValue(xs)
	case SparseMapKind:
		n, err := length()
		if err != nil {
			return fail()
		}
		xs := make([]SparseEntry, n)
		for i := range xs {
			xs[i].Key, err = child()
			if err != nil {
				return fail()
			}
			xs[i].Value, err = child()
			if err != nil {
				return fail()
			}
		}
		return SparseMapValue(xs)
	}
	return fail()
}
