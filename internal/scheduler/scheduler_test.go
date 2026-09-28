package scheduler

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func world(t testing.TB) *kernel.Kernel { return worldWithValue(t, 0) }
func worldWithValue(t testing.TB, initial float64) *kernel.Kernel {
	t.Helper()
	b := component.NewRegistryBuilder()
	if err := b.Register(component.EnergyDescriptor()); err != nil {
		t.Fatal(err)
	}
	r, err := b.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	seeds := []component.ComponentSeed{}
	for i := sim.EntityID(1); i <= 4; i++ {
		seeds = append(seeds, component.ComponentSeed{Entity: i, Component: component.EnergyDescriptor().TypeID, Fields: []component.FieldSeed{{Field: component.EnergyReserveField, Value: value(t, initial)}}})
	}
	k, err := kernel.New(r, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	return k
}
func value(t testing.TB, n float64) sim.Value {
	t.Helper()
	v, err := sim.ScalarValue(n)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func newScheduler(t *testing.T, k *kernel.Kernel, n int, eval Evaluator) *Scheduler {
	t.Helper()
	s, err := New(k, n, eval)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func noOp(_ context.Context, _ ReadyFiber, _ SnapshotView) (Evaluation, error) {
	return Evaluation{}, nil
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func tick(t *testing.T, s *Scheduler) []kernel.Event {
	t.Helper()
	ev, processed, err := s.Step(context.Background())
	if err != nil || !processed {
		t.Fatalf("step processed=%v err=%v", processed, err)
	}
	return ev
}

func TestActivityTieCancelInterruptStaleAndOverflow(t *testing.T) {
	k := world(t)
	var got []ReadyFiber
	s := newScheduler(t, k, 1, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		got = append(got, r)
		return Evaluation{}, nil
	})
	must(t, s.Register(1))
	first, err := s.Start(1, 20)
	must(t, err)
	must(t, s.Schedule(Wake{1, 20, WakeNeedThreshold}))
	must(t, s.Schedule(Wake{1, 20, WakeNeedThreshold}))
	must(t, s.Interrupt(1, first, 20))
	if s.Pending() != 2 {
		t.Fatalf("duplicate/timer retained: %d", s.Pending())
	}
	if _, err := s.Start(1, 1); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("pending interruption made actor idle: %v", err)
	}
	tick(t, s)
	if err := s.Schedule(Wake{1, 20, WakeAudit}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("reopened empty timestamp: %v", err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Causes, []WakeCause{WakeInterruption, WakeNeedThreshold}) {
		t.Fatalf("tie: %+v", got)
	}
	if f, ok := s.Fiber(1); !ok || f.Activity != nil {
		t.Fatal("interruption did not end activity")
	}
	if err := s.Cancel(1, first); !errors.Is(err, ErrStaleActivity) {
		t.Fatal(err)
	}
	if _, err := s.Start(1, sim.Duration(1<<63-1)); !errors.Is(err, sim.ErrOverflow) {
		t.Fatalf("duration overflow: %v", err)
	}
	second, err := s.Start(1, 2)
	must(t, err)
	if err := s.Interrupt(1, first, 21); !errors.Is(err, ErrStaleActivity) {
		t.Fatal(err)
	}
	must(t, s.Cancel(1, second))
	if s.Pending() != 0 {
		t.Fatal("cancel left timer")
	}
	third, err := s.Start(1, 3)
	must(t, err)
	if third == second {
		t.Fatal("reused token")
	}
	tick(t, s)
	if len(got) != 2 || !reflect.DeepEqual(got[1].Causes, []WakeCause{WakeCompletion}) || got[1].At != 23 {
		t.Fatalf("completion: %+v", got)
	}
	if _, processed, err := s.Step(context.Background()); err != nil || processed {
		t.Fatalf("duplicate wake: %v %v", processed, err)
	}
}

func TestInterruptBeforeDeadlineAndEventDoesNotInterrupt(t *testing.T) {
	k := world(t)
	var got []ReadyFiber
	s := newScheduler(t, k, 1, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		got = append(got, r)
		return Evaluation{}, nil
	})
	must(t, s.Register(1))
	token, err := s.Start(1, 20)
	must(t, err)
	must(t, s.Schedule(Wake{1, 3, WakePerceivedEvent}))
	must(t, s.Interrupt(1, token, 5))
	if s.Pending() != 2 {
		t.Fatal("pending interruption must coexist with earlier wake")
	}
	if _, err := s.Start(1, 1); !errors.Is(err, ErrInvalidState) {
		t.Fatal("started replacement before interruption")
	}
	pending := s.Snapshot()
	other, err := Restore(k, 1, noOp, pending)
	must(t, err)
	if !reflect.DeepEqual(pending, other.Snapshot()) {
		t.Fatal("lost pending interruption on restore")
	}
	bad := pending
	bad.Wakes = append([]Wake(nil), pending.Wakes...)
	for i := range bad.Wakes {
		if bad.Wakes[i].Cause == WakeInterruption {
			bad.Wakes[i].At++
		}
	}
	if _, err := Restore(k, 1, noOp, bad); !errors.Is(err, ErrInvalidState) {
		t.Fatal("accepted mismatched interruption timer")
	}
	*pending.Fibers[0].Activity.InterruptAt = 4
	if f, _ := s.Fiber(1); f.Activity == nil || *f.Activity.InterruptAt != 5 {
		t.Fatal("snapshot shares pending-interruption pointer with live state")
	}
	tick(t, s)
	if got[0].Fiber.Activity == nil || got[0].Fiber.Activity.Token != token || got[0].Fiber.Activity.InterruptAt == nil || *got[0].Fiber.Activity.InterruptAt != 5 {
		t.Fatal("earlier wake lost active token or pending interruption")
	}
	if s.Pending() != 1 {
		t.Fatal("completion timer retained")
	}
	tick(t, s)
	if got[1].At != 5 || !reflect.DeepEqual(got[1].Causes, []WakeCause{WakeInterruption}) {
		t.Fatal("incorrect interruption wake")
	}
	if _, processed, err := s.Step(context.Background()); err != nil || processed {
		t.Fatal("unexpected completion")
	}
	replacement, err := s.Start(1, 2)
	must(t, err)
	if err := s.Cancel(1, token); !errors.Is(err, ErrStaleActivity) {
		t.Fatal("old token canceled replacement")
	}
	if f, _ := s.Fiber(1); f.Activity == nil || f.Activity.Token != replacement {
		t.Fatal("replacement activity changed")
	}
}

func TestCancelPendingInterruptionRemovesWake(t *testing.T) {
	s := newScheduler(t, world(t), 1, noOp)
	must(t, s.Register(1))
	token, err := s.Start(1, 20)
	must(t, err)
	must(t, s.Interrupt(1, token, 5))
	must(t, s.Cancel(1, token))
	if s.Pending() != 0 {
		t.Fatal("canceled interruption left a wake")
	}
	replacement, err := s.Start(1, 7)
	must(t, err)
	if err := s.Interrupt(1, token, 3); !errors.Is(err, ErrStaleActivity) {
		t.Fatal("old token interrupted replacement")
	}
	f, _ := s.Fiber(1)
	if f.Activity == nil || f.Activity.Token != replacement || f.NextWake != 7 {
		t.Fatal("replacement timer changed")
	}
	tick(t, s)
	if s.Time() != 7 {
		t.Fatal("stale interruption woke replacement")
	}
}

func TestImmediateInterruptionAtGenesisRestores(t *testing.T) {
	k := world(t)
	s := newScheduler(t, k, 1, noOp)
	must(t, s.Register(1))
	token, err := s.Start(1, 5)
	must(t, err)
	must(t, s.Interrupt(1, token, 0))
	restored, err := Restore(k, 1, noOp, s.Snapshot())
	must(t, err)
	tick(t, s)
	tick(t, restored)
	if !reflect.DeepEqual(s.Snapshot(), restored.Snapshot()) {
		t.Fatal("genesis interruption restore diverged")
	}
}

func TestStepRollbackRestoreAndCorruption(t *testing.T) {
	k := world(t)
	fail := true
	eval := func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		if fail {
			return Evaluation{}, errors.New("evaluator failed")
		}
		if r.At == 10 {
			return Evaluation{Effects: []Effect{{Kind: EffectSchedule, Actor: 1, Wake: Wake{1, 12, WakeAudit}}}}, nil
		}
		return Evaluation{}, nil
	}
	s := newScheduler(t, k, 2, eval)
	must(t, s.Register(1))
	must(t, s.Schedule(Wake{1, 10, WakeBirth}))
	before := s.Snapshot()
	if _, err := Restore(worldWithValue(t, .25), 1, noOp, before); err == nil {
		t.Fatal("accepted a different seeded kernel at the same version and empty tip")
	}
	if _, processed, err := s.Step(context.Background()); err == nil || processed || !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("failure published advancement")
	}
	fail = false
	tick(t, s)
	snap := s.Snapshot()
	restored, err := Restore(k, 2, eval, snap)
	must(t, err)
	snap.Fibers[0].Revision = 999
	snap.Wakes[0].At = 999
	if !reflect.DeepEqual(s.Snapshot(), restored.Snapshot()) {
		t.Fatal("snapshot alias")
	}
	tick(t, s)
	tick(t, restored)
	if !reflect.DeepEqual(s.Snapshot(), restored.Snapshot()) {
		t.Fatal("restore diverged")
	}
	bad := before
	bad.Wakes = append([]Wake(nil), before.Wakes...)
	bad.Wakes = append(bad.Wakes, bad.Wakes[0])
	if _, err := Restore(k, 1, noOp, bad); err == nil {
		t.Fatal("accepted bad snapshot")
	}
	bad = s.Snapshot()
	bad.Wakes = append(bad.Wakes, Wake{1, 1, WakeAudit})
	if _, err := Restore(k, 1, noOp, bad); err == nil {
		t.Fatal("accepted past wake")
	}
	other := world(t)
	h := other.SnapshotHead()
	plan, err := other.Plan(proposed(t, 1, "different-head", 1), h.Authority)
	must(t, err)
	_, err = other.CommitBatch([]kernel.Plan{plan})
	must(t, err)
	if _, err := Restore(other, 1, noOp, s.Snapshot()); err == nil {
		t.Fatal("accepted wrong kernel")
	}
	bad = s.Snapshot()
	bad.Fibers[0].Activity = &Activity{Token: 1, Deadline: 100}
	if _, err := Restore(k, 1, noOp, bad); err == nil {
		t.Fatal("accepted missing activity timer")
	}
}

func proposed(t *testing.T, id sim.EntityID, key string, at sim.SimTime) kernel.Proposal {
	return kernel.Proposal{Key: key, Time: at, Cause: kernel.Cause{Actor: id}, Rule: 1, RuleVersion: 1, Patches: []component.Patch{{Entity: id, Component: component.EnergyDescriptor().TypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: value(t, .5)}}}
}
func TestWorkerOrderingAndConditionalEffects(t *testing.T) {
	var previous []kernel.Event
	var previousState []sim.Value
	for _, workers := range []int{1, 4} {
		k := world(t)
		common := k.SnapshotHead()
		var active, peak atomic.Int32
		s := newScheduler(t, k, workers, func(_ context.Context, r ReadyFiber, v SnapshotView) (Evaluation, error) {
			n := active.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			defer active.Add(-1)
			time.Sleep(time.Duration(5-int(r.Fiber.Actor)) * time.Millisecond)
			if v.Version != common.Version || v.Reader != common.Reader || v.Authority != common.Authority {
				return Evaluation{}, ErrStaleWorld
			}
			p := proposed(t, r.Fiber.Actor, fmt.Sprint(r.Fiber.Actor), r.At)
			// Actor 2 loses its field collision to actor 1.
			if r.Fiber.Actor == 2 {
				p.Patches[0].Entity = 1
			}
			return Evaluation{Proposals: []kernel.Proposal{p}, Effects: []Effect{{Kind: EffectSchedule, Actor: r.Fiber.Actor, Wake: Wake{r.Fiber.Actor, r.At + 1, WakeAudit}, IfKey: p.Key}}}, nil
		})
		for id := sim.EntityID(1); id <= 4; id++ {
			must(t, s.Register(id))
			must(t, s.Schedule(Wake{id, 10, WakeBirth}))
		}
		ev := tick(t, s)
		if len(ev) != 3 || s.Pending() != 3 || s.Time() != 10 || peak.Load() != int32(workers) {
			t.Fatalf("workers=%d events=%d pending=%d peak=%d", workers, len(ev), s.Pending(), peak.Load())
		}
		if f, _ := s.Fiber(2); f.HasNextWake {
			t.Fatal("losing effect scheduled")
		}
		state := make([]sim.Value, 4)
		h := k.SnapshotHead()
		for id := sim.EntityID(1); id <= 4; id++ {
			v, err := h.Reader.Read(component.ReadRequest{Entity: id, Component: component.EnergyDescriptor().TypeID, WorldVersion: h.Version, Authority: h.Authority})
			must(t, err)
			state[id-1], err = v.Value(component.EnergyReserveField)
			must(t, err)
		}
		if previous != nil && (!reflect.DeepEqual(ev, previous) || !reflect.DeepEqual(state, previousState)) {
			t.Fatal("worker order changed event bytes/hash or final state")
		}
		previous, previousState = ev, state
	}
}

func TestInvalidPlanAndFailedCommitRetry(t *testing.T) {
	k := world(t)
	invalid := true
	s := newScheduler(t, k, 1, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		p := proposed(t, 1, "p", r.At)
		if invalid {
			p.RuleVersion = 0
		}
		return Evaluation{Proposals: []kernel.Proposal{p}}, nil
	})
	must(t, s.Register(1))
	must(t, s.Schedule(Wake{1, 10, WakeBirth}))
	before := s.Snapshot()
	if _, _, err := s.Step(context.Background()); err == nil || !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("invalid plan advanced")
	}
	invalid = false
	// An external kernel writer invalidates the head before the next attempt.
	h := k.SnapshotHead()
	plan, err := k.Plan(proposed(t, 1, "external", 5), h.Authority)
	must(t, err)
	_, err = k.CommitBatch([]kernel.Plan{plan})
	must(t, err)
	if _, _, err := s.Step(context.Background()); !errors.Is(err, ErrStaleWorld) || !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatalf("stale head advanced: %v", err)
	}
	if _, _, err := k.CommitBatchAtHead(h, nil); !errors.Is(err, kernel.ErrStalePlan) {
		t.Fatal("empty batch accepted old head")
	}
}

func TestPostCommitHeadDoesNotAdoptExternalWriter(t *testing.T) {
	k := world(t)
	s := newScheduler(t, k, 1, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		return Evaluation{Proposals: []kernel.Proposal{proposed(t, 1, "scheduled", r.At)}, Effects: []Effect{{Kind: EffectSchedule, Actor: 1, Wake: Wake{1, 12, WakeAudit}}}}, nil
	})
	must(t, s.Register(1))
	must(t, s.Schedule(Wake{1, 10, WakeBirth}))
	s.afterCommit = func() {
		// The test hook forces a second writer after the guarded commit has
		// released the kernel lock but before the scheduler publishes its draft.
		h := k.SnapshotHead()
		if h.TipID != 1 {
			t.Fatal("guarded commit not yet visible")
		}
		plan, err := k.Plan(proposed(t, 1, "external", 11), h.Authority)
		must(t, err)
		_, err = k.CommitBatch([]kernel.Plan{plan})
		must(t, err)
	}
	events := tick(t, s)
	if len(events) != 1 {
		t.Fatal("scheduler did not commit")
	}
	if snap := s.Snapshot(); snap.Kernel.TipID != events[0].ID || snap.Kernel.TipHash != events[0].Hash {
		t.Fatal("scheduler adopted the external writer's post-commit head")
	}
	if _, processed, err := s.Step(context.Background()); !errors.Is(err, ErrStaleWorld) || processed {
		t.Fatalf("did not detect post-commit external write: %v", err)
	}
}

func TestFailedCommitRetainsWake(t *testing.T) {
	k := world(t)
	h := k.SnapshotHead()
	plan, err := k.Plan(proposed(t, 1, "duplicate", 1), h.Authority)
	must(t, err)
	_, err = k.CommitBatch([]kernel.Plan{plan})
	must(t, err)
	s := newScheduler(t, k, 1, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		return Evaluation{Proposals: []kernel.Proposal{proposed(t, 1, "duplicate", r.At)}}, nil
	})
	must(t, s.Register(1))
	must(t, s.Schedule(Wake{1, 2, WakeAudit}))
	before := s.Snapshot()
	if _, processed, err := s.Step(context.Background()); !errors.Is(err, kernel.ErrDuplicateKey) || processed || !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatalf("failed commit published state: %v", err)
	}
	if len(k.Events()) != 1 {
		t.Fatal("failed commit appended event")
	}
}

func TestDormantPopulationTimeJump(t *testing.T) {
	k := world(t)
	s := newScheduler(t, k, 3, noOp)
	for id := sim.EntityID(1); id <= 20000; id++ {
		must(t, s.Register(id))
	}
	if s.Pending() != 0 {
		t.Fatal("dormant actors gained timers")
	}
	must(t, s.Schedule(Wake{1, 1_000_000_000, WakeAudit}))
	if s.Pending() != 1 {
		t.Fatal("queue scaled with dormant fibers")
	}
	tick(t, s)
	if s.Time() != 1_000_000_000 || s.Pending() != 0 {
		t.Fatal("time did not jump")
	}
	t.Logf("dormant fibers=%d queue before=1 queue after=%d bounded workers=%d", 20000, s.Pending(), s.workers)
}

func TestNextWakeUpdatesAfterRemovingEarliestCause(t *testing.T) {
	s := newScheduler(t, world(t), 1, noOp)
	must(t, s.Register(1))
	must(t, s.Schedule(Wake{1, 10, WakeAudit}))
	token, err := s.Start(1, 5)
	must(t, err)
	if f, _ := s.Fiber(1); f.NextWake != 5 {
		t.Fatal("missing earlier activity wake")
	}
	must(t, s.Cancel(1, token))
	if f, _ := s.Fiber(1); f.NextWake != 10 || s.Pending() != 1 {
		t.Fatal("next wake not recomputed after removing minimum")
	}
}

func TestEffectStopManyActorsClearsOnlyTheirWakes(t *testing.T) {
	const stopped = 1000
	k := world(t)
	s := newScheduler(t, k, 4, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
		if r.Fiber.Actor <= stopped {
			return Evaluation{Effects: []Effect{{Kind: EffectStop, Actor: r.Fiber.Actor}}}, nil
		}
		return Evaluation{}, nil
	})
	for id := sim.EntityID(1); id <= stopped+1; id++ {
		must(t, s.Register(id))
		must(t, s.Schedule(Wake{id, 1, WakeBirth}))
		if id <= stopped {
			must(t, s.Schedule(Wake{id, 10, WakeAudit}))
			must(t, s.Schedule(Wake{id, 20, WakeCommitment}))
		} else {
			must(t, s.Schedule(Wake{id, 30, WakeAudit}))
		}
	}
	if s.Pending() != 3*stopped+2 {
		t.Fatal("missing setup wake")
	}
	tick(t, s)
	if s.Pending() != 1 {
		t.Fatalf("stopped fibers retained wakes: %d", s.Pending())
	}
	for id := sim.EntityID(1); id <= stopped; id++ {
		f, ok := s.Fiber(id)
		if !ok || f.Lifecycle != Stopped || f.HasNextWake || f.Revision != 2 {
			t.Fatalf("stop failed for actor %d: %+v", id, f)
		}
	}
	if err := s.Schedule(Wake{1, 50, WakeAudit}); !errors.Is(err, ErrInvalidState) {
		t.Fatal("stopped actor accepted wake")
	}
	snap := s.Snapshot()
	if len(snap.Wakes) != 1 || snap.Wakes[0] != (Wake{stopped + 1, 30, WakeAudit}) {
		t.Fatalf("wrong survivor wake: %+v", snap.Wakes)
	}
	restored, err := Restore(k, 4, noOp, snap)
	must(t, err)
	tick(t, restored)
	if restored.Time() != 30 || restored.Pending() != 0 {
		t.Fatal("survivor wake did not survive restoration")
	}
}

func BenchmarkEffectStopManyActors(b *testing.B) {
	k := world(b)
	for _, actors := range []int{1000, 4000} {
		b.Run(fmt.Sprintf("actors%d", actors), func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				s, err := New(k, 4, func(_ context.Context, r ReadyFiber, _ SnapshotView) (Evaluation, error) {
					return Evaluation{Effects: []Effect{{Kind: EffectStop, Actor: r.Fiber.Actor}}}, nil
				})
				if err != nil {
					b.Fatal(err)
				}
				for id := sim.EntityID(1); id <= sim.EntityID(actors); id++ {
					if err := s.Register(id); err != nil {
						b.Fatal(err)
					}
					for _, w := range []Wake{{id, 1, WakeBirth}, {id, 10, WakeAudit}, {id, 20, WakeCommitment}} {
						if err := s.Schedule(w); err != nil {
							b.Fatal(err)
						}
					}
				}
				b.StartTimer()
				_, processed, err := s.Step(context.Background())
				b.StopTimer()
				if err != nil || !processed || s.Pending() != 0 {
					b.Fatalf("stop batch failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkManyQueuedActors(b *testing.B) {
	k := world(b)
	const actors = 10000
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		s, err := New(k, 4, noOp)
		if err != nil {
			b.Fatal(err)
		}
		for id := sim.EntityID(1); id <= actors; id++ {
			if err := s.Register(id); err != nil {
				b.Fatal(err)
			}
			if err := s.Schedule(Wake{id, sim.SimTime(id), WakeAudit}); err != nil {
				b.Fatal(err)
			}
		}
		if s.Pending() != actors {
			b.Fatal("wakes lost")
		}
	}
}

func BenchmarkDormantRegistrationAndJump(b *testing.B) {
	k := world(b)
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		s, err := New(k, 4, noOp)
		if err != nil {
			b.Fatal(err)
		}
		for id := sim.EntityID(1); id <= 20000; id++ {
			if err := s.Register(id); err != nil {
				b.Fatal(err)
			}
		}
		if err := s.Schedule(Wake{1, 1_000_000_000, WakeAudit}); err != nil {
			b.Fatal(err)
		}
		if _, processed, err := s.Step(context.Background()); err != nil || !processed || s.Pending() != 0 {
			b.Fatalf("jump: %v", err)
		}
	}
}
