package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func runSurvivalTest(t *testing.T, o SurvivalOptions) *Survival {
	t.Helper()
	s, err := NewSurvival(o)
	if err != nil {
		t.Fatal(err)
	}
	for {
		ok, err := s.Step(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
	}
	return s
}

func TestSurvivalRepeatedWakesContentionAndReplay(t *testing.T) {
	opts := SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 1, EatWeight: 1}
	one := runSurvivalTest(t, opts)
	opts.Workers = 4
	many := runSurvivalTest(t, opts)
	if len(one.Journal()) != 12 || len(many.Journal()) != 12 {
		t.Fatal("missing repeated wakes")
	}
	if !reflect.DeepEqual(one.Journal(), many.Journal()) {
		t.Fatal("worker-dependent attempts or event tips")
	}
	x, y := one.kernel.Events(), many.kernel.Events()
	if !reflect.DeepEqual(x, y) {
		t.Fatal("worker-dependent accepted events")
	}
	report, err := many.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReplayOK || report.Actors != 48 || report.AcceptedEat < 2 || report.AcceptedRest < 2 || report.Rejected[SurvivalCollision] < 1 || report.Initial.Alive != 48 || report.Dissipated != report.Events {
		t.Fatalf("incorrect survival metrics: %+v", report)
	}
	for _, batch := range many.Journal() {
		if len(batch.Attempts) != 48 {
			t.Fatalf("lost persistent actor at %d", batch.Time)
		}
		for _, a := range batch.Attempts {
			if a.Status == Rejected && a.EventID != 0 {
				t.Fatal("rejected attempt linked to event")
			}
		}
	}
	for _, e := range x {
		if err := checkSurvivalEvent(e); err != nil {
			t.Fatalf("incorrect event accounting: %+v: %v", e, err)
		}
	}
}

func TestSurvivalActorOneTrace(t *testing.T) {
	s := runSurvivalTest(t, SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 4, EatWeight: 1})
	attempts, accepted, eat, rest := 0, 0, 0, 0
	for _, batch := range s.Journal() {
		for _, a := range batch.Attempts {
			if a.Actor != 1 {
				continue
			}
			attempts++
			if a.Status == Accepted {
				accepted++
			}
			if a.Choice.Kind == strategy.Eat {
				eat++
			}
			if a.Choice.Kind == strategy.Rest {
				rest++
			}
			t.Logf("actor=1 hour=%d choice=%v target=%d status=%v reason=%v event=%d", a.Time/sim.SimTime(hour), a.Choice.Kind, a.Choice.Target, a.Status, a.Reason, a.EventID)
		}
	}
	if attempts != 12 || accepted == 0 || eat == 0 || rest == 0 {
		t.Fatalf("missing repeated Eat/Rest outcomes: attempts=%d accepted=%d eat=%d rest=%d", attempts, accepted, eat, rest)
	}
	for _, ev := range s.kernel.Events() {
		if ev.Cause.Actor == 1 {
			if err := checkSurvivalEvent(ev); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSurvivalZeroEnergyStopsWithoutAnotherWake(t *testing.T) {
	o := SurvivalOptions{Actors: 32, Seed: 7, Hours: 12, Workers: 4, EatWeight: 0}
	s := runSurvivalTest(t, o)
	report, err := s.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReplayOK || report.Stopped != 32 || report.Final.Alive != 0 || report.AcceptedEat != 0 || report.AcceptedRest != report.Initial.Energy || report.Final.Food != report.Initial.Food {
		t.Fatalf("zero-energy lifecycle/accounting: %+v", report)
	}
	if s.sched.Pending() != 0 {
		t.Fatal("stopped actor still scheduled")
	}
	if len(s.Journal()) != 6 || len(report.Rejected) != 0 {
		t.Fatalf("unexpected wake or rejection after final Rest: %+v", report)
	}
}

// oneEnergySurvival isolates a final Rest from the random 2..6 unit seeds.
func oneEnergySurvival(t *testing.T, hours, actors int) *Survival {
	t.Helper()
	s, err := NewSurvival(SurvivalOptions{Actors: actors, Seed: 7, Hours: hours, Workers: 2, EatWeight: 0})
	if err != nil {
		t.Fatal(err)
	}
	for i := range s.seeds {
		if s.seeds[i].Entity != 1 {
			continue
		}
		switch s.seeds[i].Component {
		case component.EnergyTypeID:
			s.seeds[i].Fields = []component.FieldSeed{{Field: component.EnergyReserveField, Value: scalarUnit(1)}}
		case HungerTypeID:
			s.seeds[i].Fields = []component.FieldSeed{{Field: HungerField, Value: scalarUnit(capacity - 1)}}
		}
	}
	s.kernel, err = kernel.New(s.registry, 0, s.seeds)
	if err != nil {
		t.Fatal(err)
	}
	s.initial, err = s.metrics(s.kernel)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSurvivalRestAtOneStopsOnWinningAction(t *testing.T) {
	for _, hours := range []int{1, 2} {
		t.Run(string(rune('0'+hours))+"-hour-horizon", func(t *testing.T) {
			s := oneEnergySurvival(t, hours, 1)
			var err error
			s.sched, err = scheduler.New(s.kernel, 1, s.evaluate)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.sched.Register(1); err != nil {
				t.Fatal(err)
			}
			if err = s.sched.Schedule(scheduler.Wake{Actor: 1, At: sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold}); err != nil {
				t.Fatal(err)
			}
			processed, err := s.Step(context.Background())
			if err != nil || !processed {
				t.Fatalf("step: %v %v", processed, err)
			}
			fiber, _ := s.sched.Fiber(1)
			if fiber.Lifecycle != scheduler.Stopped || fiber.HasNextWake || s.sched.Pending() != 0 {
				t.Fatalf("accepted final Rest did not stop actor: %+v", fiber)
			}
			processed, err = s.Step(context.Background())
			if err != nil || processed {
				t.Fatalf("stopped actor awoke again: %v %v", processed, err)
			}
			report, err := s.Report(0, 0)
			if err != nil || !report.ReplayOK || report.AcceptedRest != 1 || report.Final.Alive != 0 || report.Stopped != 1 || report.Final.Energy != 0 || report.SimulatedHours != 1 {
				t.Fatalf("accepted final Rest lifecycle and replay: %+v %v", report, err)
			}
		})
	}
}

func TestSurvivalRejectedZeroEnergyAttemptStopsWithoutMutation(t *testing.T) {
	s := oneEnergySurvival(t, 1, 1)
	for i := range s.seeds {
		if s.seeds[i].Entity != 1 {
			continue
		}
		switch s.seeds[i].Component {
		case component.EnergyTypeID:
			s.seeds[i].Fields = []component.FieldSeed{{Field: component.EnergyReserveField, Value: scalarUnit(0)}}
		case HungerTypeID:
			s.seeds[i].Fields = []component.FieldSeed{{Field: HungerField, Value: scalarUnit(capacity)}}
		}
	}
	var err error
	s.kernel, err = kernel.New(s.registry, 0, s.seeds)
	if err != nil {
		t.Fatal(err)
	}
	s.sched, err = scheduler.New(s.kernel, 1, s.evaluate)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.sched.Register(1); err != nil {
		t.Fatal(err)
	}
	if err = s.sched.Schedule(scheduler.Wake{Actor: 1, At: sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold}); err != nil {
		t.Fatal(err)
	}
	before := s.kernel.SnapshotHead()
	processed, err := s.Step(context.Background())
	if err != nil || !processed {
		t.Fatalf("step: %v %v", processed, err)
	}
	attempt := s.Journal()[0].Attempts[0]
	fiber, _ := s.sched.Fiber(1)
	if !before.Same(s.kernel.SnapshotHead()) || len(s.kernel.Events()) != 0 || attempt.Status != Rejected || attempt.Reason != SurvivalIneligible || attempt.EventID != 0 || fiber.Lifecycle != scheduler.Stopped {
		t.Fatalf("rejected-only wake mutated state or missed stop: %+v %+v", attempt, fiber)
	}
}

func TestSurvivalLosingRestDoesNotStopActor(t *testing.T) {
	for _, hours := range []int{1, 2} {
		t.Run(string(rune('0'+hours))+"-hour-horizon", func(t *testing.T) {
			s := oneEnergySurvival(t, hours, 2)
			var err error
			s.sched, err = scheduler.New(s.kernel, 2, func(ctx context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
				if ready.Fiber.Actor == 1 {
					return s.evaluate(ctx, ready, view)
				}
				// Trusted test evaluator places a no-op field patch ahead of actor 1's
				// Rest to force a whole-proposal collision without changing energy.
				return scheduler.Evaluation{Proposals: []kernel.Proposal{{Key: "0", Time: ready.At, Cause: kernel.Cause{Actor: 2}, Rule: 1, RuleVersion: 1,
					Patches: []component.Patch{{Entity: 1, Component: component.EnergyTypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: scalarUnit(1)}}}}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, actor := range []sim.EntityID{1, 2} {
				if err = s.sched.Register(actor); err != nil {
					t.Fatal(err)
				}
				if err = s.sched.Schedule(scheduler.Wake{Actor: actor, At: sim.SimTime(hour), Cause: scheduler.WakeNeedThreshold}); err != nil {
					t.Fatal(err)
				}
			}
			processed, err := s.Step(context.Background())
			if err != nil || !processed {
				t.Fatalf("step: %v %v", processed, err)
			}
			attempt := s.Journal()[0].Attempts[0]
			fiber, _ := s.sched.Fiber(1)
			head := s.kernel.SnapshotHead()
			energy, _, err := readValue(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}, 1, component.EnergyTypeID, component.EnergyReserveField)
			if err != nil {
				t.Fatal(err)
			}
			reserve, err := unit(energy)
			if err != nil || reserve != 1 || attempt.Status != Rejected || attempt.Reason != SurvivalCollision || fiber.Lifecycle != scheduler.Alive || fiber.HasNextWake != (hours > 1) || (hours > 1 && fiber.NextWake != sim.SimTime(2*hour)) {
				t.Fatalf("losing Rest stopped or changed actor: attempt=%+v fiber=%+v energy=%d err=%v", attempt, fiber, reserve, err)
			}
		})
	}
}

func TestSurvivalReportTimeFieldsNameNanoseconds(t *testing.T) {
	data, err := json.Marshal(SurvivalReport{WallTime: 3*time.Second + 1, CPUTime: 2 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["wall_time_ns"]) != "3000000001" || string(fields["cpu_time_ns"]) != "2000000" || fields["WallTime"] != nil || fields["CPUTime"] != nil {
		t.Fatalf("timing unit ambiguous: %s", data)
	}
}

func TestSurvivalChoiceValidationFailsClosed(t *testing.T) {
	s, err := NewSurvival(SurvivalOptions{Actors: 2, Seed: 7, Hours: 1, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	head := s.kernel.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	tests := []struct {
		choice strategy.Choice
		want   SurvivalReason
	}{
		{strategy.Choice{Kind: strategy.Eat, Target: 9999, Ref: s.ref}, SurvivalInvisible},
		{strategy.Choice{Kind: strategy.Rest, Target: 1001, Ref: s.ref}, SurvivalInvalidChoice},
		{strategy.Choice{Kind: strategy.Eat, Target: 1001, Ref: strategy.Ref{ID: "survival", Version: 2}}, SurvivalStale},
		{strategy.Choice{Kind: strategy.Eat, Target: 1001, Ref: s.ref, ObservedVersion: 1}, SurvivalStale},
	}
	for _, tt := range tests {
		_, reason, err := s.validateChoice(view, 1, sim.SimTime(hour), tt.choice)
		if err != nil || reason != tt.want {
			t.Fatalf("choice %+v: %v %v", tt.choice, reason, err)
		}
	}
	if len(s.kernel.Events()) != 0 || s.kernel.SnapshotHead().Version != 0 {
		t.Fatal("validation mutated state")
	}
}

func TestSurvivalRejectsNonIntegerAndUnrepresentableFood(t *testing.T) {
	s, err := NewSurvival(SurvivalOptions{Actors: 2, Seed: 7, Hours: 1, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, stock := range []float64{1.5, float64(1 << 53)} {
		seeds := append([]component.ComponentSeed(nil), s.seeds...)
		for i := range seeds {
			if seeds[i].Entity == 1001 && seeds[i].Component == component.CacheStockTypeID {
				value, er := sim.ScalarValue(stock)
				if er != nil {
					t.Fatal(er)
				}
				seeds[i].Fields = []component.FieldSeed{{Field: component.CacheStockField, Value: value}}
			}
		}
		k, er := kernel.New(s.registry, 0, seeds)
		if er != nil {
			t.Fatal(er)
		}
		head := k.SnapshotHead()
		view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
		_, reason, er := s.validateChoice(view, 1, sim.SimTime(hour), strategy.Choice{Kind: strategy.Eat, Target: 1001, ObservedVersion: head.Version, Ref: s.ref})
		if er != nil || reason != SurvivalInvalidNumber || len(k.Events()) != 0 {
			t.Fatalf("stock %g: reason %v err %v", stock, reason, er)
		}
	}
}

func TestSurvivalPolicyAndSeedSensitivity(t *testing.T) {
	base := SurvivalOptions{Actors: 48, Seed: 7, Hours: 12, Workers: 2, EatWeight: 1}
	a := runSurvivalTest(t, base)
	ar, err := a.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	base.EatWeight = 0
	b := runSurvivalTest(t, base)
	br, err := b.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ar.AcceptedEat == br.AcceptedEat || ar.Final.Food == br.Final.Food || !ar.ReplayOK || !br.ReplayOK {
		t.Fatalf("policy parameter did not change consumption: %d/%d food %d/%d", ar.AcceptedEat, br.AcceptedEat, ar.Final.Food, br.Final.Food)
	}
	base.EatWeight = 1
	base.Seed = 8
	c := runSurvivalTest(t, base)
	cr, err := c.Report(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ar.Initial.Food == cr.Initial.Food && ar.Initial.Energy == cr.Initial.Energy {
		t.Fatal("seed did not change initial resources")
	}
}

func TestSurvivalAccountingDetectsTampering(t *testing.T) {
	event := kernel.Event{Rule: 2, RuleVersion: 1, Cause: kernel.Cause{Actor: 1}, Deltas: []component.FieldDelta{
		{Entity: 1001, Component: component.CacheStockTypeID, SchemaVersion: 1, Before: scalarUnit(4), After: scalarUnit(2)},
		{Entity: 1, Component: component.EnergyTypeID, SchemaVersion: 1, Before: scalarUnit(2), After: scalarUnit(4)},
		{Entity: 1, Component: HungerTypeID, SchemaVersion: 1, Before: scalarUnit(6), After: scalarUnit(5)},
	}, Metrics: make([]component.MetricDelta, 3)}
	if checkSurvivalEvent(event) == nil {
		t.Fatal("created energy accepted")
	}
}
