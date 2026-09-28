package sim

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func TestValueCodecRoundTrip(t *testing.T) {
	scalar, _ := ScalarValue(.25)
	prob, _ := ProbabilityValue(.75)
	enum, _ := EnumTypedValue(EnumValue{Enum: 1, Value: 2})
	time, _ := TimeValue(42)
	duration, _ := DurationValue(12)
	entity, _ := EntityRefValue(1)
	event, _ := EventRefValue(2)
	vector, _ := VectorValue([]float64{0, -2})
	distribution, _ := DistributionValue([]DistributionPoint{{Outcome: BoolValue(true), Probability: .75}, {Outcome: BoolValue(false), Probability: .25}})
	record, _ := RecordValue([]RecordField{{ID: 2, Value: scalar}, {ID: 1, Value: BoolValue(true)}})
	optional, _ := OptionalValue(scalar)
	list, _ := ListValue([]Value{scalar, IntegerValue(-4)})
	set, _ := SetValue([]Value{IntegerValue(2), IntegerValue(1)})
	sparse, _ := SparseMapValue([]SparseEntry{{Key: IntegerValue(2), Value: scalar}, {Key: IntegerValue(1), Value: BoolValue(false)}})
	values := []Value{BoolValue(false), IntegerValue(-42), scalar, prob, enum, time, duration, entity, event, vector, distribution, record, optional, list, set, sparse}
	for kind := BoolKind; kind <= SparseMapKind; kind++ {
		for _, state := range []ValueState{Missing, Unknown, Inapplicable} {
			v, _ := AbsentValue(kind, state)
			values = append(values, v)
		}
	}
	for _, v := range values {
		data, err := EncodeValue(v)
		if err != nil {
			t.Fatalf("encode kind %d: %v", v.Kind(), err)
		}
		decoded, err := DecodeValue(data)
		if err != nil || !reflect.DeepEqual(v, decoded) {
			t.Fatalf("round trip kind %d: %v", v.Kind(), err)
		}
		again, _ := EncodeValue(decoded)
		if !bytes.Equal(data, again) {
			t.Fatal("unstable encoding")
		}
	}
}
func TestValueCodecRejectsMalformedOrDeepInput(t *testing.T) {
	malformed := [][]byte{{}, {0}, {255, 0}, {1, 255}, {1, 0, 2}, {2, 0, 1}, {3, 0, 0xff, 0xf0, 0, 0, 0, 0, 0, 0}, {3, 0, 0x80, 0, 0, 0, 0, 0, 0, 0}, {13, 0}, append([]byte{1, 1}, 0), bytes.Repeat([]byte{1}, MaxValueBytes+1)}
	for i, data := range malformed {
		if _, err := DecodeValue(data); err == nil {
			t.Fatalf("accepted malformed %d", i)
		}
	}
	v := BoolValue(true)
	for i := 0; i < maxValueDepth; i++ {
		next, err := OptionalValue(v)
		if err != nil {
			t.Fatal(err)
		}
		v = next
	}
	if _, err := EncodeValue(v); !errors.Is(err, ErrValueEncoding) {
		t.Fatalf("accepted deep value: %v", err)
	}
	// A noncanonical set order is rejected even when its members are individually valid.
	a, _ := EncodeValue(IntegerValue(2))
	b, _ := EncodeValue(IntegerValue(1))
	raw := append([]byte{byte(SetKind), byte(Present), 0, 0, 0, 2}, a...)
	raw = append(raw, b...)
	if _, err := DecodeValue(raw); !errors.Is(err, ErrValueEncoding) {
		t.Fatalf("accepted noncanonical set: %v", err)
	}
}
