package world_test

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/world"
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"
)

func scalar(t *testing.T, n float64) sim.Value {
	t.Helper()
	v, err := sim.ScalarValue(n)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func fixture(t *testing.T, stock sim.Value, energy sim.Value) (component.Registry, []component.ComponentSeed, *kernel.Kernel) {
	t.Helper()
	builder := component.NewRegistryBuilder()
	for _, descriptor := range []component.ComponentDescriptor{component.EnergyDescriptor(), component.CacheStockDescriptor(), component.FatigueDescriptor()} {
		if err := builder.Register(descriptor); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := builder.Freeze()
	if err != nil {
		t.Fatal(err)
	}
	seeds := []component.ComponentSeed{
		{Entity: 1, Component: component.EnergyTypeID, Fields: []component.FieldSeed{{Field: component.EnergyReserveField, Value: energy}}},
		{Entity: 2, Component: component.EnergyTypeID, Fields: []component.FieldSeed{{Field: component.EnergyReserveField, Value: scalar(t, 2)}}},
		{Entity: 2, Component: component.FatigueTypeID, Fields: []component.FieldSeed{{Field: component.FatigueLevelField, Value: scalar(t, .5)}}},
		{Entity: 10, Component: component.CacheStockTypeID, Fields: []component.FieldSeed{{Field: component.CacheStockField, Value: stock}}},
		{Entity: 11, Component: component.CacheStockTypeID, Fields: []component.FieldSeed{{Field: component.CacheStockField, Value: scalar(t, 2)}}},
		{Entity: 12, Component: component.CacheStockTypeID, Fields: []component.FieldSeed{{Field: component.CacheStockField, Value: scalar(t, 9)}}},
	}
	k, err := kernel.New(registry, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	return registry, seeds, k
}

func setup(t *testing.T, k *kernel.Kernel, workers int, public map[sim.EntityID][]sim.EntityID, decide world.Decide, actors ...sim.EntityID) *world.Runner {
	t.Helper()
	r, err := world.New(k, workers, public, decide)
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range actors {
		if err := r.Register(actor); err != nil {
			t.Fatal(err)
		}
		if err := r.Schedule(scheduler.Wake{Actor: actor, At: 1, Cause: scheduler.WakeAudit}); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func step(t *testing.T, r *world.Runner) {
	t.Helper()
	processed, err := r.Step(context.Background())
	if err != nil || !processed {
		t.Fatalf("step processed=%v err=%v", processed, err)
	}
}

func field(t *testing.T, k *kernel.Kernel, actor sim.EntityID, typ sim.ComponentTypeID, id sim.FieldID) sim.Value {
	t.Helper()
	reader, auth, version := k.Snapshot()
	v, err := reader.Read(component.ReadRequest{Entity: actor, Component: typ, Fields: []sim.FieldID{id}, WorldVersion: version, Authority: auth})
	if err != nil {
		t.Fatal(err)
	}
	result, err := v.Value(id)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestObservationIsActorSpecificAndCannotCarryAuthority(t *testing.T) {
	_, _, k := fixture(t, scalar(t, 3), scalar(t, 4))
	wantFields := []string{"Actor", "Time", "Version", "Energy", "PublicCaches"}
	typ := reflect.TypeOf(world.Observation{})
	if typ.NumField() != len(wantFields) {
		t.Fatalf("unexpected observation fields: %v", typ)
	}
	for i, name := range wantFields {
		if typ.Field(i).Name != name {
			t.Fatalf("observation leaked field %v", typ.Field(i))
		}
	}
	public := map[sim.EntityID][]sim.EntityID{1: {10}, 2: {11}}
	var mu sync.Mutex
	got := map[sim.EntityID]world.Observation{}
	r := setup(t, k, 2, public, func(_ context.Context, obs world.Observation) (world.Intention, error) {
		mu.Lock()
		saved := obs
		saved.PublicCaches = append([]world.CacheStock(nil), obs.PublicCaches...)
		got[obs.Actor] = saved
		mu.Unlock()
		// This mutation must not modify trusted visibility or rule inputs.
		obs.PublicCaches[0].CacheID = 12
		return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: 12, ObservedVersion: obs.Version}}, nil
	}, 1, 2)
	public[1][0] = 12
	step(t, r)
	for actor, id := range map[sim.EntityID]sim.EntityID{1: 10, 2: 11} {
		obs := got[actor]
		if obs.Actor != actor || obs.Version != 0 || obs.Time != 1 || len(obs.PublicCaches) != 1 || obs.PublicCaches[0].CacheID != id || !reflect.DeepEqual(obs.Energy, field(t, k, actor, component.EnergyTypeID, component.EnergyReserveField)) {
			t.Fatalf("incorrect actor-specific observation: %+v", obs)
		}
		out, ok := r.OutcomeFor(actor)
		if !ok || out.Status != world.Rejected || out.Reason != world.InvisibleCache || out.EventID != 0 {
			t.Fatalf("hidden attempt: %+v %v", out, ok)
		}
	}
	if len(k.Events()) != 0 || !reflect.DeepEqual(field(t, k, 12, component.CacheStockTypeID, component.CacheStockField), scalar(t, 9)) {
		t.Fatal("hidden target mutated")
	}
}

func TestRejectedAttemptsHaveBoundedReasonsAndNoMutation(t *testing.T) {
	missing, _ := sim.AbsentValue(sim.ScalarKind, sim.Missing)
	unknown, _ := sim.AbsentValue(sim.ScalarKind, sim.Unknown)
	cases := []struct {
		name          string
		stock, energy sim.Value
		target        sim.EntityID
		version       sim.WorldVersion
		reason        world.Reason
	}{
		{"stale", scalar(t, 2), scalar(t, 3), 10, 99, world.StaleObservation},
		{"invisible", scalar(t, 2), scalar(t, 3), 12, 0, world.InvisibleCache},
		{"invalid target", scalar(t, 2), scalar(t, 3), 0, 0, world.InvalidTarget},
		{"missing component", scalar(t, 2), scalar(t, 3), 13, 0, world.UnavailableStock},
		{"missing stock", missing, scalar(t, 3), 10, 0, world.UnavailableStock},
		{"unknown stock", unknown, scalar(t, 3), 10, 0, world.UnavailableStock},
		{"insufficient", scalar(t, .5), scalar(t, 3), 10, 0, world.InsufficientStock},
		{"ineligible", scalar(t, 2), unknown, 10, 0, world.IneligibleActor},
		{"overflow", scalar(t, 2), scalar(t, math.MaxFloat64), 10, 0, world.InvalidNumber},
		{"lost precision", scalar(t, 1e30), scalar(t, 3), 10, 0, world.InvalidNumber},
		{"stock subtraction rounds by two", scalar(t, 9007199254740994), scalar(t, 0), 10, 0, world.InvalidNumber},
		{"energy addition rounds by two", scalar(t, 2), scalar(t, 9007199254740994), 10, 0, world.InvalidNumber},
		{"fractional energy rounds difference to one", scalar(t, 2), scalar(t, 0.1), 10, 0, world.InvalidNumber},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, k := fixture(t, tc.stock, tc.energy)
			before := k.SnapshotHead()
			r := setup(t, k, 1, map[sim.EntityID][]sim.EntityID{1: {10, 13}}, func(_ context.Context, _ world.Observation) (world.Intention, error) {
				return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: tc.target, ObservedVersion: tc.version}}, nil
			}, 1)
			step(t, r)
			out, _ := r.OutcomeFor(1)
			if out.Reason != tc.reason || out.Status != world.Rejected || out.EventID != 0 || out.Key != "0000000000000001:0000000000000001" {
				t.Fatalf("outcome: %+v", out)
			}
			if len(k.Events()) != 0 || !before.Same(k.SnapshotHead()) {
				t.Fatal("rejection changed world")
			}
			journal := r.Journal()
			if len(journal) != 1 || journal[0].Time != 1 || journal[0].Version != 0 || journal[0].TipID != 0 || !reflect.DeepEqual(journal[0].Attempts, []world.Outcome{out}) {
				t.Fatalf("journal: %+v", journal)
			}
			journal[0].Attempts[0].Reason = world.NoReason
			if r.Journal()[0].Attempts[0].Reason != tc.reason {
				t.Fatal("journal returned mutable storage")
			}
			if err := r.Schedule(scheduler.Wake{Actor: 1, At: 1, Cause: scheduler.WakeAudit}); err == nil {
				t.Fatal("rejected-only timestamp reopened")
			}
		})
	}
}

func TestRepresentableWithdrawalConservesOneJoule(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		stock, energy, afterStock, afterEnergy float64
	}{
		{"fractional energy", 2, 0.5, 1, 1.5},
		{"fractional stock", 1.5, 0.5, 0.5, 1.5},
		{"large boundary", 9007199254740992, 9007199254740991, 9007199254740991, 9007199254740992},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, k := fixture(t, scalar(t, tc.stock), scalar(t, tc.energy))
			r := setup(t, k, 1, map[sim.EntityID][]sim.EntityID{1: {10}}, func(_ context.Context, o world.Observation) (world.Intention, error) {
				return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: 10, ObservedVersion: o.Version}}, nil
			}, 1)
			step(t, r)
			out, _ := r.OutcomeFor(1)
			if out.Status != world.Accepted || out.EventID != 1 || len(k.Events()) != 1 ||
				!reflect.DeepEqual(field(t, k, 10, component.CacheStockTypeID, component.CacheStockField), scalar(t, tc.afterStock)) ||
				!reflect.DeepEqual(field(t, k, 1, component.EnergyTypeID, component.EnergyReserveField), scalar(t, tc.afterEnergy)) {
				t.Fatalf("representable withdrawal did not conserve: %+v", out)
			}
		})
	}
}

func TestMissingActorEnergyRejectsWithoutEvent(t *testing.T) {
	registry, seeds, _ := fixture(t, scalar(t, 2), scalar(t, 0))
	seeds = seeds[1:] // actor 1 has no energy component
	k, err := kernel.New(registry, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	r := setup(t, k, 1, map[sim.EntityID][]sim.EntityID{1: {10}}, func(_ context.Context, o world.Observation) (world.Intention, error) {
		if o.Energy.State() != sim.Missing {
			return world.Intention{}, errors.New("missing actor energy was not reported missing")
		}
		return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: 10, ObservedVersion: o.Version}}, nil
	}, 1)
	step(t, r)
	out, _ := r.OutcomeFor(1)
	if out.Reason != world.IneligibleActor || len(k.Events()) != 0 {
		t.Fatalf("missing actor energy accepted: %+v", out)
	}
}

func TestCollisionIndependentCachesAndReplay(t *testing.T) {
	var reference []world.Outcome
	for _, workers := range []int{1, 2} {
		for _, reverse := range []bool{false, true} {
			t.Run(string(rune('0'+workers))+map[bool]string{true: "reverse", false: "forward"}[reverse], func(t *testing.T) {
				registry, seeds, k := fixture(t, scalar(t, 3), scalar(t, 0))
				var observed sync.Mutex
				snapshots := make(map[sim.EntityID]world.Observation)
				r := setup(t, k, workers, map[sim.EntityID][]sim.EntityID{1: {10}, 2: {10}}, func(_ context.Context, obs world.Observation) (world.Intention, error) {
					observed.Lock()
					snapshots[obs.Actor] = obs
					observed.Unlock()
					if (obs.Actor == 1) == reverse {
						time.Sleep(5 * time.Millisecond)
					}
					return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: 10, ObservedVersion: obs.Version}}, nil
				}, 1, 2)
				step(t, r)
				for _, actor := range []sim.EntityID{1, 2} {
					obs := snapshots[actor]
					if obs.Version != 0 || obs.Time != 1 || len(obs.PublicCaches) != 1 || !reflect.DeepEqual(obs.PublicCaches[0].Stock, scalar(t, 3)) {
						t.Fatalf("actors did not see a common snapshot: %+v", snapshots)
					}
				}
				batch := r.Journal()[0]
				if reference == nil {
					reference = batch.Attempts
				} else if !reflect.DeepEqual(reference, batch.Attempts) {
					t.Fatalf("nondeterministic outcomes: %+v vs %+v", reference, batch.Attempts)
				}
				if len(batch.Attempts) != 2 || batch.Attempts[0].Status != world.Accepted || batch.Attempts[0].EventID != 1 || batch.Attempts[1].Reason != world.CacheCollision || batch.Attempts[1].EventID != 0 || batch.Version != 1 || batch.TipID != 1 {
					t.Fatalf("batch: %+v", batch)
				}
				events := k.Events()
				if len(events) != 1 || events[0].Cause.Actor != 1 || events[0].Rule != component.WithdrawRuleID || events[0].Key != batch.Attempts[0].Key || len(events[0].Deltas) != 2 || len(events[0].Metrics) != 2 || batch.TipHash != events[0].Hash {
					t.Fatalf("event evidence: %+v", events)
				}
				if events[0].Deltas[0].Entity != 1 || events[0].Deltas[0].Before.State() != sim.Present || !reflect.DeepEqual(events[0].Deltas[0].After, scalar(t, 1)) || events[0].Deltas[1].Entity != 10 || !reflect.DeepEqual(events[0].Deltas[1].After, scalar(t, 2)) {
					t.Fatalf("incorrect typed event deltas: %+v", events[0].Deltas)
				}
				if !reflect.DeepEqual(field(t, k, 10, component.CacheStockTypeID, component.CacheStockField), scalar(t, 2)) || !reflect.DeepEqual(field(t, k, 1, component.EnergyTypeID, component.EnergyReserveField), scalar(t, 1)) || !reflect.DeepEqual(field(t, k, 2, component.EnergyTypeID, component.EnergyReserveField), scalar(t, 2)) {
					t.Fatal("incorrect winner state")
				}
				replayed, err := kernel.Replay(registry, 0, seeds, events)
				if err != nil || !reflect.DeepEqual(replayed.Events(), events) || !reflect.DeepEqual(field(t, replayed, 10, component.CacheStockTypeID, component.CacheStockField), scalar(t, 2)) {
					t.Fatalf("replay: %v", err)
				}
			})
		}
	}
	_, _, k := fixture(t, scalar(t, 2), scalar(t, 0))
	r := setup(t, k, 2, map[sim.EntityID][]sim.EntityID{1: {10}, 2: {11}}, func(_ context.Context, o world.Observation) (world.Intention, error) {
		return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: o.PublicCaches[0].CacheID, ObservedVersion: o.Version}}, nil
	}, 1, 2)
	step(t, r)
	if len(k.Events()) != 2 || r.Journal()[0].Attempts[1].Status != world.Accepted || !reflect.DeepEqual(field(t, k, 11, component.CacheStockTypeID, component.CacheStockField), scalar(t, 1)) {
		t.Fatal("independent cache did not commit")
	}
}

func TestCallbackFailureDiscardsBatchAndRetriesWakes(t *testing.T) {
	_, _, k := fixture(t, scalar(t, 2), scalar(t, 0))
	fail := true
	r := setup(t, k, 1, map[sim.EntityID][]sim.EntityID{1: {10}}, func(_ context.Context, o world.Observation) (world.Intention, error) {
		if fail {
			fail = false
			return world.Intention{}, errors.New("temporary callback failure")
		}
		return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: 10, ObservedVersion: o.Version}}, nil
	}, 1)
	if _, err := r.Step(context.Background()); err == nil || len(r.Journal()) != 0 || len(k.Events()) != 0 {
		t.Fatalf("failed callback published state: %v", err)
	}
	step(t, r)
	if len(r.Journal()) != 1 || len(k.Events()) != 1 || r.Journal()[0].Attempts[0].Status != world.Accepted {
		t.Fatal("failed callback consumed wake")
	}
}

func TestFailedStepDiscardsAttemptsAndRetainsWakes(t *testing.T) {
	_, _, k := fixture(t, scalar(t, 2), scalar(t, 0))
	begin := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		<-begin
		_, auth, _ := k.Snapshot()
		value, _ := sim.ScalarValue(3)
		p, err := k.Plan(kernel.Proposal{Key: "external", Time: 1, Cause: kernel.Cause{Actor: 2}, Rule: 1, RuleVersion: 1, Patches: []component.Patch{{Entity: 2, Component: component.EnergyTypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: value}}}, auth)
		if err == nil {
			_, err = k.CommitBatch([]kernel.Plan{p})
		}
		finished <- err
	}()
	r := setup(t, k, 1, map[sim.EntityID][]sim.EntityID{1: {10}}, func(_ context.Context, o world.Observation) (world.Intention, error) {
		close(begin)
		if err := <-finished; err != nil {
			return world.Intention{}, err
		}
		return world.Intention{WithdrawEnergy: world.WithdrawEnergy{CacheID: 10, ObservedVersion: o.Version}}, nil
	}, 1)
	if _, err := r.Step(context.Background()); !errors.Is(err, scheduler.ErrStaleWorld) {
		t.Fatalf("expected stale commit failure, got %v", err)
	}
	if len(r.Journal()) != 0 {
		t.Fatal("published failed attempt")
	}
	if _, ok := r.OutcomeFor(1); ok {
		t.Fatal("published failed outcome")
	}
	// The wake remains queued on failure; a new runner can continue from the
	// external writer's head, while the stale scheduler correctly refuses retry.
	if _, err := r.Step(context.Background()); !errors.Is(err, scheduler.ErrStaleWorld) {
		t.Fatalf("expected stale scheduler, got %v", err)
	}
}
