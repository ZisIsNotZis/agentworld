package sim

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestValueKindsAndFourStates(t *testing.T) {
	scalarZero, _ := ScalarValue(0)
	timeValue, _ := TimeValue(3)
	durationValue, _ := DurationValue(4)
	entity, _ := EntityRefValue(5)
	event, _ := EventRefValue(6)
	enum, _ := EnumTypedValue(EnumValue{Enum: 1, Value: 2})
	vector, _ := VectorValue([]float64{1, 2})
	distribution, _ := DistributionValue([]DistributionPoint{{Outcome: BoolValue(true), Probability: 1}})
	record, _ := RecordValue([]RecordField{{ID: 1, Value: IntegerValue(1)}})
	set, _ := SetValue([]Value{IntegerValue(2), IntegerValue(1)})
	sparse, _ := SparseMapValue([]SparseEntry{{Key: IntegerValue(1), Value: BoolValue(true)}})
	probability, _ := ProbabilityValue(.5)
	optional, _ := OptionalValue(BoolValue(true))
	list, _ := ListValue([]Value{BoolValue(true)})
	values := []Value{BoolValue(true), IntegerValue(1), scalarZero, probability, enum, timeValue, durationValue, entity, event, vector, distribution, record, optional, list, set, sparse}
	for i, value := range values {
		if value.Kind() != Kind(i+1) || value.State() != Present {
			t.Fatalf("value %d kind/state = %d/%d", i, value.Kind(), value.State())
		}
	}
	for _, state := range []ValueState{Missing, Unknown, Inapplicable} {
		value, err := AbsentValue(ScalarKind, state)
		if err != nil || value.State() != state || value.Kind() != ScalarKind {
			t.Fatalf("absent %d = %#v, %v", state, value, err)
		}
	}
	if got, _ := scalarZero.Scalar(); got != 0 || !scalarZero.IsPresent() {
		t.Fatal("present zero was not preserved")
	}
	if _, err := scalarZero.Integer(); !errors.Is(err, ErrWrongKind) {
		t.Fatalf("wrong accessor = %v", err)
	}
}

func TestValueValidationAndDefensiveCopies(t *testing.T) {
	if _, err := ScalarValue(math.NaN()); !errors.Is(err, ErrNonFinite) {
		t.Fatal("NaN accepted")
	}
	if _, err := ProbabilityValue(1.1); !errors.Is(err, ErrInvalidProbability) {
		t.Fatal("invalid probability accepted")
	}
	input := []float64{2, 1}
	value, _ := VectorValue(input)
	input[0] = 99
	first, _ := value.Vector()
	first[1] = 88
	second, _ := value.Vector()
	if !reflect.DeepEqual(second, []float64{2, 1}) {
		t.Fatalf("vector mutated: %v", second)
	}
	child, _ := VectorValue([]float64{1})
	list, _ := ListValue([]Value{child})
	got, _ := list.List()
	vector, _ := got[0].Vector()
	vector[0] = 9
	again, _ := list.List()
	nested, _ := again[0].Vector()
	if nested[0] != 1 {
		t.Fatal("nested value mutated")
	}
}

func TestSignedZeroCanonicalization(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	negativeProbability, _ := ProbabilityValue(negativeZero)
	positiveProbability, _ := ProbabilityValue(0)
	probability, _ := negativeProbability.Scalar()
	if math.Signbit(probability) || !negativeProbability.Equal(positiveProbability) {
		t.Fatal("probability retained signed zero")
	}
	if _, err := SetValue([]Value{negativeProbability, positiveProbability}); !errors.Is(err, ErrDuplicateValue) {
		t.Fatalf("probability set did not detect canonical duplicate: %v", err)
	}

	negativeVector, _ := VectorValue([]float64{negativeZero})
	positiveVector, _ := VectorValue([]float64{0})
	vector, _ := negativeVector.Vector()
	if math.Signbit(vector[0]) || !negativeVector.Equal(positiveVector) {
		t.Fatal("vector retained signed zero")
	}
	if _, err := SetValue([]Value{negativeVector, positiveVector}); !errors.Is(err, ErrDuplicateValue) {
		t.Fatalf("vector set did not detect canonical duplicate: %v", err)
	}

	negativeDistribution, _ := DistributionValue([]DistributionPoint{{Outcome: BoolValue(false), Probability: negativeZero}, {Outcome: BoolValue(true), Probability: 1}})
	positiveDistribution, _ := DistributionValue([]DistributionPoint{{Outcome: BoolValue(false), Probability: 0}, {Outcome: BoolValue(true), Probability: 1}})
	points, _ := negativeDistribution.Distribution()
	if math.Signbit(points[0].Probability) || !negativeDistribution.Equal(positiveDistribution) {
		t.Fatal("distribution retained signed zero")
	}
	if _, err := SetValue([]Value{negativeDistribution, positiveDistribution}); !errors.Is(err, ErrDuplicateValue) {
		t.Fatalf("distribution set did not detect canonical duplicate: %v", err)
	}
}

func TestCanonicalCompositeValues(t *testing.T) {
	set, err := SetValue([]Value{IntegerValue(2), IntegerValue(1)})
	if err != nil {
		t.Fatal(err)
	}
	values, _ := set.Set()
	first, _ := values[0].Integer()
	second, _ := values[1].Integer()
	if first != 1 || second != 2 {
		t.Fatalf("set order = %d,%d", first, second)
	}
	if _, err := SetValue([]Value{IntegerValue(1), IntegerValue(1)}); !errors.Is(err, ErrDuplicateValue) {
		t.Fatalf("duplicate set = %v", err)
	}
	record, err := RecordValue([]RecordField{{ID: 2, Value: BoolValue(true)}, {ID: 1, Value: BoolValue(false)}})
	if err != nil {
		t.Fatal(err)
	}
	fields, _ := record.Record()
	if fields[0].ID != 1 || fields[1].ID != 2 {
		t.Fatalf("record order = %v", fields)
	}
}
