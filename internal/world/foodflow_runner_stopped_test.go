package world

import (
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"context"
	"errors"
	"testing"
)

func TestFoodFlowRunnerStoppedActorsHaveNoPostmortemBasal(t *testing.T) {
	f, err := NewFoodFlow(FoodFlowOptions{Yield: 0, Seed: 0, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checks := f.Checkpoints()
	if len(checks) != FoodFlowHorizonHours+1 || checks[11].Alive != 0 || checks[168].Alive != 0 {
		t.Fatalf("zero-flow extinction/audit missing: checkpoints=%d h11=%d h168=%d", len(checks), checks[11].Alive, checks[168].Alive)
	}
	for i, atDeath := range checks[11].Actors {
		id := sim.EntityID(i + 1)
		fiber, ok := f.sched.Fiber(id)
		if !ok || fiber.Lifecycle != scheduler.Stopped || atDeath.Energy != 0 || atDeath.Hunger != 11 || atDeath.BasalSpent != 11 {
			t.Fatalf("actor %d not stopped with h11 need: fiber=%+v state=%+v", id, fiber, atDeath)
		}
		for _, h := range []int{12, 24, 72, 168} {
			if got := checks[h].Actors[i]; got != atDeath {
				t.Errorf("actor %d postmortem state at h%d changed: h11=%+v h%d=%+v", id, h, atDeath, h, got)
				break
			}
		}
	}
	basal, postmortem := 0, 0
	for _, ev := range f.Kernel().Events() {
		if ev.Rule != FoodFlowBasalRule {
			continue
		}
		basal++
		if ev.Time > 11*sim.SimTime(FoodFlowHour) {
			postmortem++
		}
	}
	if basal != 11*FoodFlowActorCount || postmortem != 0 {
		t.Fatalf("zero-flow basal events=%d, postmortem=%d; want exactly %d events through h11", basal, postmortem, 11*FoodFlowActorCount)
	}
	// Even if a scheduler lifecycle were inconsistent with the authoritative
	// energy snapshot, a stopped actor must not silently receive another pulse.
	f.pulseLifecycle[0] = scheduler.Alive
	head := f.k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	at, _ := FoodFlowHourTime(FoodFlowHorizonHours)
	if _, err := f.pulse(scheduler.ReadyFiber{At: at}, view); !errors.Is(err, ErrFoodFlowRunner) {
		t.Fatalf("live fiber with zero energy passed pulse validation: %v", err)
	}
}

func TestFoodFlowRunnerScarceDeathsFreezeActorState(t *testing.T) {
	f, err := NewFoodFlow(FoodFlowOptions{Yield: 3, Seed: 0, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checks := f.Checkpoints()
	var deathHour [FoodFlowActorCount]int
	stopped := 0
	for i := range checks[0].Actors {
		for h := 1; h <= FoodFlowHorizonHours; h++ {
			if checks[h].Actors[i].Energy == 0 {
				deathHour[i] = h
				stopped++
				for later := h + 1; later <= FoodFlowHorizonHours; later++ {
					if checks[later].Actors[i] != checks[h].Actors[i] {
						t.Fatalf("actor %d changed after death h%d: h%d=%+v h%d=%+v", i+1, h, h, checks[h].Actors[i], later, checks[later].Actors[i])
					}
				}
				break
			}
		}
	}
	if checks[72].Alive != 6 || checks[168].Alive != 6 || stopped != 10 {
		t.Fatalf("scarce survivors h72=%d h168=%d stopped=%d", checks[72].Alive, checks[168].Alive, stopped)
	}
	for _, ev := range f.Kernel().Events() {
		if ev.Rule != FoodFlowBasalRule {
			continue
		}
		actor := ev.Deltas[0].Entity
		if actor < 1 || actor > FoodFlowActorCount {
			t.Fatalf("basal event %d has invalid actor %d", ev.ID, actor)
		}
		if hour := deathHour[actor-1]; hour != 0 && ev.Time > sim.SimTime(hour)*sim.SimTime(FoodFlowHour) {
			t.Fatalf("basal event %d after actor %d stopped at h%d", ev.ID, actor, hour)
		}
	}
}
