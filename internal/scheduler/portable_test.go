package scheduler

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
)

func portableRegistry(t *testing.T) component.Registry {
	t.Helper()
	b := component.NewRegistryBuilder()
	must(t, b.Register(component.EnergyDescriptor()))
	r, err := b.Freeze()
	must(t, err)
	return r
}

func portableKernel(t *testing.T, k *kernel.Kernel) (*kernel.Kernel, kernel.PortableHead) {
	t.Helper()
	wire, head, err := k.ExportHistory()
	must(t, err)
	restored, verified, err := kernel.RestoreHistory(portableRegistry(t), wire)
	must(t, err)
	if head != verified {
		t.Fatal("kernel history changed on restore")
	}
	return restored, verified
}

func portableBytes(t *testing.T, s *Scheduler) ([]byte, kernel.PortableHead) {
	t.Helper()
	wire, head, err := s.ExportPortable()
	must(t, err)
	return wire, head
}

func TestPortablePendingActivityInterruptionAndTie(t *testing.T) {
	k := world(t)
	var observed []ReadyFiber
	var mu sync.Mutex
	eval := func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		mu.Lock()
		observed = append(observed, r)
		mu.Unlock()
		return Evaluation{}, nil
	}
	s := newScheduler(t, k, 1, eval)
	for _, id := range []sim.EntityID{1, 2, 3} {
		must(t, s.Register(id))
	}
	token, err := s.Start(1, 20)
	must(t, err)
	completionToken, err := s.Start(3, 10)
	must(t, err)
	must(t, s.Interrupt(1, token, 5))
	must(t, s.Schedule(Wake{2, 5, WakeAudit}))
	must(t, s.Schedule(Wake{1, 5, WakeNeedThreshold}))
	wire, head := portableBytes(t, s)
	fresh, verified := portableKernel(t, k)
	if head != verified || fresh.SnapshotHead().OriginID == k.SnapshotHead().OriginID {
		t.Fatal("not a fresh kernel")
	}
	if _, err := Restore(fresh, 4, eval, s.Snapshot()); !errors.Is(err, ErrInvalidState) {
		t.Fatal("ordinary Restore accepted a foreign origin")
	}
	restored, err := RestorePortable(fresh, 4, eval, wire, verified)
	must(t, err)
	if !reflect.DeepEqual(s.Snapshot().Fibers, restored.Snapshot().Fibers) || !reflect.DeepEqual(s.Snapshot().Wakes, restored.Snapshot().Wakes) {
		t.Fatal("pending state changed")
	}
	reencoded, again := portableBytes(t, restored)
	if !bytes.Equal(wire, reencoded) || head != again {
		t.Fatal("portable encoding is not canonical")
	}
	if _, err := restored.Start(1, 1); !errors.Is(err, ErrInvalidState) {
		t.Fatal("started while interruption pending")
	}
	tick(t, restored)
	sort.Slice(observed, func(i, j int) bool { return observed[i].Fiber.Actor < observed[j].Fiber.Actor })
	if len(observed) != 2 || observed[0].Fiber.Actor != 1 || observed[1].Fiber.Actor != 2 ||
		!reflect.DeepEqual(observed[0].Causes, []WakeCause{WakeInterruption, WakeNeedThreshold}) || restored.Pending() != 1 {
		t.Fatalf("lost tie order or duplicated wake: %+v", observed)
	}
	tick(t, restored)
	if len(observed) != 3 || observed[2].Fiber.Actor != 3 || !reflect.DeepEqual(observed[2].Causes, []WakeCause{WakeCompletion}) || restored.Pending() != 0 {
		t.Fatal("completion wake was lost or duplicated")
	}
	if _, processed, err := restored.Step(context.Background()); err != nil || processed {
		t.Fatal("duplicate wake")
	}
	if err := restored.Cancel(1, token); !errors.Is(err, ErrStaleActivity) {
		t.Fatal("old token remained live")
	}
	if next, err := restored.Start(1, 1); err != nil || next <= completionToken {
		t.Fatalf("token counter/activity failed to continue: %d %v", next, err)
	}
}

func TestPortableClosedTimestampWithoutKernelEvent(t *testing.T) {
	k := world(t)
	s := newScheduler(t, k, 1, noOp)
	must(t, s.Register(1))
	must(t, s.Schedule(Wake{1, 9, WakeAudit}))
	tick(t, s)
	wire, head := portableBytes(t, s)
	if head.TipID != 0 || head.TipTime != 0 {
		t.Fatal("empty step unexpectedly committed")
	}
	fresh, verified := portableKernel(t, k)
	restored, err := RestorePortable(fresh, 1, noOp, wire, verified)
	must(t, err)
	if restored.Time() != 9 || !restored.Snapshot().HasClosed || restored.Snapshot().Closed != 9 {
		t.Fatal("lost closed timestamp")
	}
	if err := restored.Schedule(Wake{1, 9, WakeAudit}); !errors.Is(err, ErrInvalidState) {
		t.Fatal("reopened closed timestamp")
	}
	must(t, restored.Schedule(Wake{1, 10, WakeAudit}))
	tick(t, restored)
	if restored.Time() != 10 {
		t.Fatal("continuation failed")
	}
}

func TestPortableHistoryContinuationAcrossWorkerCounts(t *testing.T) {
	k := world(t)
	eval := func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		p := proposed(t, r.Fiber.Actor, fmt.Sprintf("actor-%d-at-%d", r.Fiber.Actor, r.At), r.At)
		p.Patches[0].Value = value(t, float64(r.At)/100)
		return Evaluation{Proposals: []kernel.Proposal{p}}, nil
	}
	s := newScheduler(t, k, 1, eval)
	for _, id := range []sim.EntityID{1, 2, 3} {
		must(t, s.Register(id))
		must(t, s.Schedule(Wake{id, 5, WakeBirth}))
	}
	tick(t, s)
	for _, id := range []sim.EntityID{3, 1, 2} {
		must(t, s.Schedule(Wake{id, 10, WakeAudit}))
	}
	wire, head := portableBytes(t, s)
	fresh, verified := portableKernel(t, k)
	if head != verified {
		t.Fatal("different kernel heads")
	}
	restored, err := RestorePortable(fresh, 4, eval, wire, verified)
	must(t, err)
	originalEvents, restoredEvents := tick(t, s), tick(t, restored)
	if !reflect.DeepEqual(originalEvents, restoredEvents) {
		t.Fatal("future events/metrics diverged")
	}
	originalHistory, originalHead, err := k.ExportHistory()
	must(t, err)
	restoredHistory, restoredHead, err := fresh.ExportHistory()
	must(t, err)
	if originalHead != restoredHead || !bytes.Equal(originalHistory, restoredHistory) {
		t.Fatal("future kernel state diverged")
	}
	one, oneHead := portableBytes(t, s)
	two, twoHead := portableBytes(t, restored)
	if oneHead != twoHead || !bytes.Equal(one, two) {
		t.Fatal("future scheduler state diverged")
	}
}

func TestPortableRejectsWrongKernelStaleWorldAndHead(t *testing.T) {
	k := world(t)
	s := newScheduler(t, k, 1, noOp)
	must(t, s.Register(1))
	wire, head := portableBytes(t, s)
	fresh, verified := portableKernel(t, k)
	wrong, _ := portableKernel(t, worldWithValue(t, .25))
	if _, err := RestorePortable(wrong, 1, noOp, wire, verified); err == nil {
		t.Fatal("accepted same tip with different world")
	}
	badHead := head
	badHead.ProjectionHash[0] ^= 1
	if _, err := RestorePortable(fresh, 1, noOp, wire, badHead); err == nil {
		t.Fatal("accepted substituted verified head")
	}
	must(t, s.Schedule(Wake{1, 1, WakeAudit}))
	tick(t, s)
	if _, _, err := s.ExportPortable(); err != nil {
		t.Fatal(err)
	}
	// An external commit after the scheduler's capture invalidates export.
	h := k.SnapshotHead()
	p, err := k.Plan(proposed(t, 1, "external", 2), h.Authority)
	must(t, err)
	_, err = k.CommitBatch([]kernel.Plan{p})
	must(t, err)
	if _, err := RestorePortable(k, 1, noOp, wire, head); err == nil {
		t.Fatal("accepted advanced kernel")
	}
	if _, _, err := s.ExportPortable(); !errors.Is(err, ErrStaleWorld) {
		t.Fatalf("accepted stale scheduler: %v", err)
	}
}

// Recompute the unkeyed digest to exercise structural and semantic validation
// independently of the byte-corruption detector.
func resignPortable(b []byte) []byte {
	b = bytes.Clone(b)
	digest := sha256.Sum256(b[:len(b)-portableDigestSize])
	copy(b[len(b)-portableDigestSize:], digest[:])
	return b
}
func TestPortableRejectsTamperingTruncationAndNoncanonicalState(t *testing.T) {
	k := world(t)
	s := newScheduler(t, k, 1, noOp)
	must(t, s.Register(1))
	must(t, s.Register(2))
	token, err := s.Start(1, 10)
	must(t, err)
	must(t, s.Interrupt(1, token, 5))
	must(t, s.Schedule(Wake{2, 5, WakeAudit}))
	wire, head := portableBytes(t, s)
	fresh, verified := portableKernel(t, k)
	if head != verified {
		t.Fatal("head mismatch")
	}
	const fiber = portableHeaderSize
	const wake = fiber + 2*portableFiberSize
	cases := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"short", func(b []byte) []byte { return b[:len(b)-1] }},
		{"oversize", func([]byte) []byte { return make([]byte, MaxPortableBytes+1) }},
		{"trailing", func(b []byte) []byte { return append(b, 0) }},
		{"digest", func(b []byte) []byte { b[fiber] ^= 1; return b }},
		{"version", func(b []byte) []byte { b[7]++; return resignPortable(b) }},
		{"embedded head", func(b []byte) []byte { b[8]++; return resignPortable(b) }},
		{"negative clock", func(b []byte) []byte { b[200] = 0xff; return resignPortable(b) }},
		{"different closed time", func(b []byte) []byte { binary.BigEndian.PutUint64(b[208:216], 3); b[216] = 1; return resignPortable(b) }},
		{"bad flag", func(b []byte) []byte { b[216] = 2; return resignPortable(b) }},
		{"unsorted fibers", func(b []byte) []byte {
			first := bytes.Clone(b[fiber : fiber+portableFiberSize])
			copy(b[fiber:fiber+portableFiberSize], b[fiber+portableFiberSize:fiber+2*portableFiberSize])
			copy(b[fiber+portableFiberSize:fiber+2*portableFiberSize], first)
			return resignPortable(b)
		}},
		{"unused activity bytes", func(b []byte) []byte { b[fiber+portableFiberSize+18] = 1; return resignPortable(b) }},
		{"duplicate actor", func(b []byte) []byte {
			copy(b[fiber+portableFiberSize:fiber+portableFiberSize+8], b[fiber:fiber+8])
			return resignPortable(b)
		}},
		{"bad activity token", func(b []byte) []byte { binary.BigEndian.PutUint64(b[fiber+18:fiber+26], 0); return resignPortable(b) }},
		{"orphan interruption", func(b []byte) []byte { b[fiber+34] = 0; return resignPortable(b) }},
		{"bad wake", func(b []byte) []byte { b[wake+16] = 0; return resignPortable(b) }},
		{"duplicate wake", func(b []byte) []byte {
			copy(b[wake+portableWakeSize:wake+2*portableWakeSize], b[wake:wake+portableWakeSize])
			return resignPortable(b)
		}},
		{"unsorted wakes", func(b []byte) []byte {
			first := bytes.Clone(b[wake : wake+portableWakeSize])
			copy(b[wake:wake+portableWakeSize], b[wake+portableWakeSize:wake+2*portableWakeSize])
			copy(b[wake+portableWakeSize:wake+2*portableWakeSize], first)
			return resignPortable(b)
		}},
		{"missing wake", func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[229:233], 1)
			copy(b[wake:wake+portableWakeSize], b[wake+portableWakeSize:wake+2*portableWakeSize])
			b = append(b[:wake+portableWakeSize], b[wake+2*portableWakeSize:]...)
			return resignPortable(b)
		}},
		{"bad count", func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[225:229], MaxPortableFibers+1)
			return resignPortable(b)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := RestorePortable(fresh, 1, noOp, tc.mutate(bytes.Clone(wire)), verified); err == nil || got != nil {
				t.Fatalf("accepted corrupt snapshot: %v", err)
			}
		})
	}
	for n := 0; n < len(wire); n++ {
		if got, err := RestorePortable(fresh, 1, noOp, wire[:n], verified); err == nil || got != nil {
			t.Fatalf("accepted truncated snapshot at %d", n)
		}
	}
}

func TestPortableExportBounds(t *testing.T) {
	s := newScheduler(t, world(t), 1, noOp)
	for id := sim.EntityID(1); id <= MaxPortableFibers+1; id++ {
		must(t, s.Register(id))
	}
	if data, _, err := s.ExportPortable(); !errors.Is(err, ErrInvalidState) || data != nil {
		t.Fatal("accepted over-budget fibers")
	}
}
