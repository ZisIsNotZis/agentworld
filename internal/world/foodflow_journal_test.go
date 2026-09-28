package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
)

func foodFlowJournalFixture(t *testing.T, q int64, steps int) (*FoodFlow, []byte, kernel.PortableHead) {
	t.Helper()
	f, err := NewFoodFlow(FoodFlowOptions{Yield: q, Seed: 7, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if steps == 0 {
		if err = f.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	} else {
		for range steps {
			processed, e := f.Step(context.Background())
			if !processed || e != nil {
				t.Fatalf("step: %v", e)
			}
		}
	}
	data, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return f, data, head
}
func TestFoodFlowJournalRoundTripFixturesAndHistoricCounts(t *testing.T) {
	for _, q := range []int64{8, 3, 0} {
		t.Run(string(rune('0'+q)), func(t *testing.T) {
			f, data, head := foodFlowJournalFixture(t, q, 0)
			decoded, err := DecodeFoodFlowJournal(data, f.k, head, f.sched.Time(), f.journalConfig())
			if err != nil || !reflect.DeepEqual(decoded, f.Journal()) {
				t.Fatalf("round trip: %v", err)
			}
			accepted, rejected, rest := 0, 0, 0
			for _, b := range decoded {
				for _, a := range b.Attempts {
					if a.Accepted {
						accepted++
					} else {
						rejected++
					}
				}
				for _, a := range b.Activities {
					if a.Kind == strategy.FoodFlowRest {
						rest++
					}
				}
			}
			if q == 0 && (accepted != 0 || rejected == 0 || rest != 0) || q == 3 && (accepted == 0 || rejected == 0 || rest == 0) || q == 8 && accepted == 0 {
				t.Fatalf("counts q%d accepted=%d rejected=%d rest=%d", q, accepted, rejected, rest)
			}
			other, err := NewFoodFlow(FoodFlowOptions{Yield: q, Seed: 7, Workers: 1})
			if err != nil {
				t.Fatal(err)
			}
			// Restore into a compatible existing runner at the same verified boundary.
			other.k, other.sched = f.k, f.sched
			other.journal = []FoodFlowBatch{{Time: 123}}
			if err = other.RestoreJournal(data, head); err != nil || !reflect.DeepEqual(other.Journal(), decoded) {
				t.Fatalf("restore: %v", err)
			}
			again, err := other.ExportJournal()
			if err != nil || !bytes.Equal(again, data) {
				t.Fatalf("noncanonical re-export: %v", err)
			}
			t.Logf("q%d steps=%d journal=%d accepted=%d rejected=%d rest=%d", q, len(decoded), len(data), accepted, rejected, rest)
		})
	}
}
func TestFoodFlowJournalActiveAndRejectedOnlyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		q     int64
		steps int
	}{{3, 2}, {0, 2}, {3, 3}, {3, 4}} {
		f, data, head := foodFlowJournalFixture(t, tc.q, tc.steps)
		decoded, err := DecodeFoodFlowJournal(data, f.k, head, f.sched.Time(), f.journalConfig())
		if err != nil || !reflect.DeepEqual(decoded, f.Journal()) {
			t.Fatalf("q%d steps%d: %v", tc.q, tc.steps, err)
		}
		if tc.q == 0 && (len(decoded[1].Attempts) != 16 || decoded[1].TipID != decoded[0].TipID) {
			t.Fatal("lost rejected-only timestamp")
		}
		if tc.q == 3 && tc.steps == 2 && len(decoded[1].Activities) != 16 {
			t.Fatal("lost active Gather")
		}
	}
}
func TestFoodFlowJournalCorruptionAndMismatch(t *testing.T) {
	f, data, head := foodFlowJournalFixture(t, 3, 4)
	reject := func(name string, wire []byte, verified kernel.PortableHead, at sim.SimTime, c FoodFlowJournalConfig) {
		t.Helper()
		got, err := DecodeFoodFlowJournal(wire, f.k, verified, at, c)
		if !errors.Is(err, ErrFoodFlowJournal) || got != nil {
			t.Fatalf("%s: decoded corrupt evidence: %v", name, err)
		}
	}
	for _, n := range []int{0, 5, len(data) - 1, len(data) - sha256.Size} {
		reject("truncated", data[:n], head, f.sched.Time(), f.journalConfig())
	}
	reject("oversize", make([]byte, MaxFoodFlowJournalBytes+1), head, f.sched.Time(), f.journalConfig())
	reject("clock", data, head, f.sched.Time()+1, f.journalConfig())
	wrong := head
	wrong.TipHash[0] ^= 1
	reject("head", data, wrong, f.sched.Time(), f.journalConfig())
	for _, c := range []FoodFlowJournalConfig{{Yield: 4, Seed: 7, Policy: f.ref}, {Yield: 3, Seed: 8, Policy: f.ref}, {Yield: 3, Seed: 7, Policy: strategy.FoodFlowRef{ID: "food-flow", Version: 2}}} {
		reject("run identity", data, head, f.sched.Time(), c)
	}
	damaged := bytes.Clone(data)
	damaged[12] ^= 1
	reject("checksum", damaged, head, f.sched.Time(), f.journalConfig())
	// Rehash a valid-shape wrong event link: the digest is not a signature.
	first := f.Journal()
	found := false
	for i := range first {
		for j := range first[i].Attempts {
			if first[i].Attempts[j].Accepted {
				first[i].Attempts[j].EventID++
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no accepted action")
	}
	if _, err := EncodeFoodFlowJournal(first, f.k, head, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("accepted mismatched event ID: %v", err)
	}
	// Change an accepted ID directly in the wire, retaining a valid checksum.
	r := journalReader{data: data[:len(data)-sha256.Size]}
	r.take(4)
	r.u32()
	r.head()
	r.u64()
	r.u64()
	r.u64()
	size := r.u8()
	r.take(int(size))
	r.u32()
	r.u16()
	mutated := false
	for !mutated && r.remaining() > 0 {
		r.u64()
		r.u8()
		r.u64()
		r.u64()
		r.take(32)
		na, nc := r.u8(), r.u8()
		for range int(na) {
			r.u64()
			r.u64()
			r.u8()
			sz := r.u8()
			keyOffset := r.offset
			r.take(int(sz))
			r.u8()
			r.take(8 * 4)
			r.take(3)
			accepted := r.u8()
			eventOffset := r.offset
			r.u64()
			rankOffset := r.offset
			r.u8()
			slotOffset := r.offset
			r.u64()
			sourceOffset := r.offset
			r.u64()
			if accepted == 1 && !mutated {
				for _, tc := range []struct {
					name   string
					offset int
				}{
					{"event link", eventOffset + 7}, {"event key", keyOffset}, {"allocator rank", rankOffset}, {"allocated slot", slotOffset + 7}, {"source patch", sourceOffset + 7},
				} {
					wire := bytes.Clone(data)
					wire[tc.offset] ^= 1
					sum := sha256.Sum256(wire[:len(wire)-32])
					copy(wire[len(wire)-32:], sum[:])
					reject("rehashed "+tc.name, wire, head, f.sched.Time(), f.journalConfig())
				}
				mutated = true
			}
		}
		r.take(int(nc) * 41)
	}
	if !mutated {
		t.Fatal("no encoded accepted action")
	}
	before := f.Journal()
	if err := f.RestoreJournal(damaged, head); !errors.Is(err, ErrFoodFlowJournal) || !reflect.DeepEqual(f.Journal(), before) {
		t.Fatalf("non-atomic restore: %v", err)
	}
	for _, opts := range []FoodFlowOptions{{Yield: 4, Seed: 7, Workers: 1}, {Yield: 3, Seed: 8, Workers: 1}} {
		other, err := NewFoodFlow(opts)
		if err != nil {
			t.Fatal(err)
		}
		other.k, other.sched = f.k, f.sched
		other.journal = []FoodFlowBatch{{Time: 123}}
		if err = other.RestoreJournal(data, head); !errors.Is(err, ErrFoodFlowJournal) || len(other.Journal()) != 1 {
			t.Fatalf("mismatched runner config %+v: %v", opts, err)
		}
	}
}
func TestFoodFlowJournalRejectsLaterHeadForEarlierPrefix(t *testing.T) {
	f, early, earlyHead := foodFlowJournalFixture(t, 3, 3)
	if _, err := DecodeFoodFlowJournal(early, f.k, earlyHead, f.sched.Time(), f.journalConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, laterHead, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeFoodFlowJournal(early, f.k, earlyHead, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("earlier prefix on later kernel: %v", err)
	}
	if _, err = DecodeFoodFlowJournal(early, f.k, laterHead, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("earlier prefix on later head: %v", err)
	}
}
