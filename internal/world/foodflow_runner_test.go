package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"reflect"
	"testing"
)

func runFoodFlowFixture(t *testing.T, q int64, seed uint64, workers int) *FoodFlow {
	t.Helper()
	f, err := NewFoodFlow(FoodFlowOptions{Yield: q, Seed: seed, Workers: workers})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.steps > 700 {
		t.Fatalf("unbounded steps %d", f.steps)
	}
	return f
}
func foodFlowEventBytes(t *testing.T, f *FoodFlow) [][]byte {
	t.Helper()
	events := f.Kernel().Events()
	out := make([][]byte, len(events))
	for i, ev := range events {
		var err error
		out[i], err = ev.Bytes()
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func TestFoodFlowRunnerFrozenFixtures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		q        int64
		endAlive int
	}{{"abundant", 8, 15}, {"scarce", 3, 8}, {"zero", 0, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			f := runFoodFlowFixture(t, tc.q, 0, 4)
			checks := f.Checkpoints()
			if len(checks) != 169 {
				t.Fatalf("checkpoints %d", len(checks))
			}
			for h, c := range checks {
				if c.Hour != h {
					t.Fatalf("missing hour %d: %+v", h, c)
				}
				if c.Balance.Produced != c.Balance.Consumed+c.Balance.Held+c.Balance.Stock || c.Balance.InitialEnergy+c.Balance.Consumed != c.Balance.Energy+c.Balance.BasalSpent+c.Balance.EnergyCapLost {
					t.Fatalf("hour %d: %+v", h, c.Balance)
				}
			}
			switch tc.q {
			case 8:
				if checks[168].Alive < tc.endAlive {
					t.Fatalf("h168 alive=%d", checks[168].Alive)
				}
			case 3:
				if checks[72].Alive > tc.endAlive {
					t.Fatalf("h72 alive=%d", checks[72].Alive)
				}
				successes := [FoodFlowActorCount]int{}
				claims := [FoodFlowActorCount]int{}
				for _, b := range f.Journal() {
					for _, a := range b.Attempts {
						if a.Choice.Kind != strategy.FoodFlowGather || int(a.Time/sim.SimTime(FoodFlowHour)) >= 8 {
							continue
						}
						claims[a.Actor-1]++
						if a.Accepted {
							successes[a.Actor-1]++
						}
					}
				}
				for i := range successes {
					if successes[i] != 3 || claims[i] != 8 {
						t.Fatalf("actor %d first eight claims=%d successes=%d", i+1, claims[i], successes[i])
					}
				}
			case 0:
				if checks[11].Alive != 0 {
					t.Fatalf("zero extinction at h11: alive=%d", checks[11].Alive)
				}
			}
			gathered, eaten, capacity := 0, 0, 0
			for _, b := range f.Journal() {
				for _, a := range b.Attempts {
					if a.Choice.Kind == strategy.FoodFlowGather {
						if a.Accepted {
							gathered++
						}
						if a.Rejection == FoodFlowGatherCapacity {
							capacity++
						}
					}
					if a.Choice.Kind == strategy.FoodFlowEat && a.Accepted {
						eaten++
					}
				}
			}
			if tc.q == 0 && (gathered != 0 || eaten != 0) {
				t.Fatalf("zero gather/eat %d/%d", gathered, eaten)
			}
			if tc.q == 8 && capacity != 0 {
				t.Fatalf("abundant capacity rejection %d", capacity)
			}
			t.Logf("q=%d steps=%d events=%d h1=%d h4=%d h12=%d h24=%d h72=%d h120=%d h168=%d gathered=%d eaten=%d capacity=%d balance=%+v", tc.q, f.steps, len(f.Kernel().Events()), checks[1].Alive, checks[4].Alive, checks[12].Alive, checks[24].Alive, checks[72].Alive, checks[120].Alive, checks[168].Alive, gathered, eaten, capacity, checks[168].Balance)
		})
	}
}

func TestFoodFlowRunnerWorkerReplayLinksAndTrajectory(t *testing.T) {
	for _, q := range []int64{0, 3, 8} {
		one := runFoodFlowFixture(t, q, 3, 1)
		four := runFoodFlowFixture(t, q, 3, 4)
		if !reflect.DeepEqual(foodFlowEventBytes(t, one), foodFlowEventBytes(t, four)) || !reflect.DeepEqual(one.Journal(), four.Journal()) {
			t.Fatalf("q%d worker-dependent events/journal", q)
		}
		history, _, err := four.Kernel().ExportHistory()
		if err != nil {
			t.Fatal(err)
		}
		restored, _, err := kernel.RestoreHistory(four.registry, history)
		if err != nil {
			t.Fatal(err)
		}
		original := foodFlowEventBytes(t, four)
		for i, ev := range restored.Events() {
			got, err := ev.Bytes()
			if err != nil || !bytes.Equal(got, original[i]) {
				t.Fatalf("q%d replay event %d: %v", q, i, err)
			}
		}
		if restored.SnapshotHead().TipHash != four.Kernel().SnapshotHead().TipHash {
			t.Fatalf("q%d replay tip", q)
		}
		byID := make(map[sim.EventID]kernel.Event)
		for _, ev := range four.Kernel().Events() {
			byID[ev.ID] = ev
		}
		for _, b := range four.Journal() {
			for _, a := range b.Attempts {
				if a.Accepted {
					ev, ok := byID[a.EventID]
					if !ok || ev.Key != a.Key || ev.Time != a.Time || ev.Cause.Actor != a.Actor {
						t.Fatalf("q%d broken attempt link %+v", q, a)
					}
				} else if a.EventID != 0 {
					t.Fatalf("q%d rejected linked %+v", q, a)
				}
			}
		}
		for _, c := range four.Checkpoints() {
			for _, id := range []int{0, 7, 15} {
				a := c.Actors[id]
				if a.Energy < 0 || a.Hunger < 0 || a.Hunger > FoodFlowHungerCapacity {
					t.Fatalf("q%d h%d actor%d %+v", q, c.Hour, id+1, a)
				}
			}
			for _, patch := range c.Slots {
				for _, slot := range patch {
					if slot.Stock < 0 || slot.Stock > 1 {
						t.Fatalf("q%d h%d slot %+v", q, c.Hour, slot)
					}
				}
			}
		}
	}
}

func TestFoodFlowRunnerActivityClockAndDistinctGather(t *testing.T) {
	f := runFoodFlowFixture(t, 3, 0, 4)
	events := map[sim.EventID]kernel.Event{}
	for _, ev := range f.Kernel().Events() {
		events[ev.ID] = ev
	}
	gathered := map[[2]int64]bool{}
	started, completed, denied, rested := 0, 0, 0, 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Choice.Kind == strategy.FoodFlowGather && !a.Accepted {
				denied++
			}
		}
	}
	for _, batch := range f.Journal() {
		for _, a := range batch.Activities {
			started++
			if a.Token == 0 || a.Completed == 0 {
				t.Fatalf("unfinished activity %+v", a)
			}
			completed++
			duration := sim.SimTime(foodFlowMealDuration)
			switch a.Kind {
			case strategy.FoodFlowGather:
				duration = sim.SimTime(foodFlowGatherDuration)
			case strategy.FoodFlowRest:
				duration = sim.SimTime(strategy.FoodFlowRestDuration)
			}
			if a.Completed-a.Started != duration {
				t.Fatalf("wrong activity duration %+v", a)
			}
			switch a.Kind {
			case strategy.FoodFlowGather:
				if a.EventID != 0 {
					ev, ok := events[a.EventID]
					if !ok || ev.Rule != FoodFlowGatherRule || ev.Time != a.Completed || ev.Cause.Actor != a.Actor {
						t.Fatalf("bad gather completion %+v", a)
					}
					key := [2]int64{int64(a.Actor), int64(a.Completed / sim.SimTime(FoodFlowHour))}
					if gathered[key] {
						t.Fatalf("twice per hour actor %d", a.Actor)
					}
					gathered[key] = true
				}
			case strategy.FoodFlowEat:
				if ev, ok := events[a.EventID]; !ok || ev.Rule != FoodFlowConsumeRule || ev.Time != a.Completed || ev.Cause.Actor != a.Actor {
					t.Fatalf("bad meal completion %+v", a)
				}
			case strategy.FoodFlowRest:
				rested++
				if a.EventID != 0 {
					t.Fatalf("rest had accepted event %+v", a)
				}
			default:
				t.Fatalf("unknown activity %+v", a)
			}
		}
	}
	if started != completed || started == 0 || denied != rested || rested == 0 {
		t.Fatalf("activities started=%d completed=%d denied=%d rested=%d", started, completed, denied, rested)
	}
}

func TestFoodFlowRunnerPortableHandoffAtActiveGather(t *testing.T) {
	f, err := NewFoodFlow(FoodFlowOptions{Yield: 3, Seed: 7, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		processed, e := f.Step(context.Background())
		if e != nil || !processed {
			t.Fatalf("step %d: %v", i, e)
		}
	}
	handoff, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if handoff.FormatVersion != FoodFlowFormatVersion || handoff.Yield != 3 || handoff.Seed != 7 || handoff.Steps != 2 || len(handoff.Current) != 16 || len(handoff.Choices) != 16 || len(handoff.Checkpoints) != 1 || len(handoff.History) == 0 || len(handoff.SchedulerBytes) == 0 {
		t.Fatalf("incomplete handoff %+v", handoff)
	}
	restored, head, err := kernel.RestoreHistory(f.registry, handoff.History)
	if err != nil || head != handoff.Head || restored.SnapshotHead().Version != f.Kernel().SnapshotHead().Version {
		t.Fatalf("invalid history handoff: %v", err)
	}
	handoff.Current[1] = strategy.FoodFlowWait
	handoff.Choices[1] = strategy.FoodFlowChoice{}
	handoff.Journal[1].Activities = nil
	if another, err := f.Handoff(); err != nil || another.Current[1] != strategy.FoodFlowGather || another.Choices[1].Kind != strategy.FoodFlowGather || len(another.Journal[1].Activities) != 16 {
		t.Fatalf("handoff aliases runner memory: %v", err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatalf("handoff disrupted continuation: %v", err)
	}
}

func TestFoodFlowRunnerRejectedGatherNeverDefersBasal(t *testing.T) {
	f := runFoodFlowFixture(t, 3, 0, 1)
	checks := f.Checkpoints()
	var denied sim.EntityID
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Choice.Kind == strategy.FoodFlowGather && !a.Accepted && a.Time < sim.SimTime(FoodFlowHour) {
				denied = a.Actor
				break
			}
		}
		if denied != 0 {
			break
		}
	}
	if denied == 0 {
		t.Fatal("expected first-hour denied gather")
	}
	before, after := checks[0].Actors[denied-1], checks[1].Actors[denied-1]
	if after.BasalSpent != before.BasalSpent+1 || after.Energy != before.Energy-1 || after.Hunger != before.Hunger+1 {
		t.Fatalf("denied actor %d basal deferred: %+v -> %+v", denied, before, after)
	}
}
