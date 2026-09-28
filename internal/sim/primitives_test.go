package sim

import (
	"errors"
	"math"
	"testing"
)

func TestEntityAllocatorAndIDs(t *testing.T) {
	validators := []func() error{
		func() error { return ValidateEntityID(0) }, func() error { return ValidateEventID(0) },
		func() error { return ValidateComponentTypeID(0) }, func() error { return ValidateFieldID(0) },
		func() error { return ValidateProjectionID(0) }, func() error { return ValidateSchemaVersion(0) },
		func() error { return ValidateProjectionVersion(0) }, func() error { return ValidateRuleID(0) }, func() error { return ValidateEnumID(0) },
	}
	for index, validate := range validators {
		if !errors.Is(validate(), ErrInvalidID) {
			t.Fatalf("zero accepted by validator %d", index)
		}
	}
	if WorldVersion(0) != 0 {
		t.Fatal("zero world version must be valid")
	}
	allocator := NewEntityAllocator(41)
	first, err := allocator.Next()
	if err != nil || first != 42 {
		t.Fatalf("Next() = %d, %v", first, err)
	}
	second, _ := allocator.Next()
	if second != 43 {
		t.Fatalf("second ID = %d", second)
	}
	overflow := NewEntityAllocator(EntityID(math.MaxUint64))
	if _, err := overflow.Next(); !errors.Is(err, ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
}

func TestTimeCheckedArithmetic(t *testing.T) {
	if _, err := NewSimTime(-1); !errors.Is(err, ErrNegativeTime) {
		t.Fatal("negative time accepted")
	}
	if _, err := NewDuration(-1); !errors.Is(err, ErrNegativeTime) {
		t.Fatal("negative duration accepted")
	}
	time, _ := NewSimTime(10)
	duration, _ := NewDuration(7)
	got, err := time.Add(duration)
	if err != nil || got != 17 {
		t.Fatalf("Add = %d, %v", got, err)
	}
	delta, err := got.Sub(time)
	if err != nil || delta != duration {
		t.Fatalf("Sub = %d, %v", delta, err)
	}
	if _, err := SimTime(math.MaxInt64).Add(1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("overflow = %v", err)
	}
}

func TestRandomStreamGoldenRestoreAndSeparation(t *testing.T) {
	stream := NewRandomStream(RandomState{Seed: 42, Stream: 7})
	want := []uint64{0xd0e2f2e045bcec7e, 0x3a7298de2f6f12db, 0xbba6a6d5db658362, 0x65463a2bb0128feb}
	for i, expected := range want {
		if got := stream.Uint64(); got != expected {
			t.Fatalf("value %d = %#x, want %#x", i, got, expected)
		}
	}
	restored := NewRandomStream(RandomState{Seed: 42, Stream: 7, Position: 2})
	if got := restored.Uint64(); got != want[2] {
		t.Fatalf("restored = %#x", got)
	}
	other := NewRandomStream(RandomState{Seed: 42, Stream: 8})
	if got := other.Uint64(); got == want[0] {
		t.Fatal("streams did not separate")
	}
	first := NewRandomStream(RandomState{Seed: 1, Stream: 1})
	second := NewRandomStream(RandomState{Seed: 1, Stream: 1})
	if first.Uint64() != second.Uint64() {
		t.Fatal("stream is not repeatable")
	}
}

func TestRandomPositionOverflowPanicsWithoutAdvancing(t *testing.T) {
	stream := NewRandomStream(RandomState{Position: math.MaxUint64})
	defer func() {
		if recover() == nil {
			t.Fatal("expected overflow panic")
		}
		if stream.State().Position != math.MaxUint64 {
			t.Fatal("overflow advanced stream")
		}
	}()
	stream.Uint64()
}
