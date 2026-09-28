package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The generic portable scheduler proves that its queue is internally valid;
// this pilot additionally proves that the queue is exactly the one produced
// by its pulse/claim/activity protocol. No other wake causes are authorized.
func verifyFoodFlowCheckpointScheduler(k *kernel.Kernel, journal []FoodFlowBatch, snap scheduler.Snapshot) error {
	if k == nil || len(snap.Fibers) != FoodFlowActorCount+1 {
		return ErrFoodFlowCheckpoint
	}
	phase, hour := byte(0), 0
	if len(journal) != 0 {
		at := journal[len(journal)-1].Time
		phase, hour = foodFlowPhase(at), int(at/sim.SimTime(FoodFlowHour))
		if phase == 255 || snap.Time != at {
			return ErrFoodFlowCheckpoint
		}
	} else if snap.Time != 0 || snap.HasClosed {
		return ErrFoodFlowCheckpoint
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	_, _, actors, err := foodFlowStates(view)
	if err != nil {
		return err
	}
	var revisions [FoodFlowActorCount]uint64
	var open [FoodFlowActorCount]*FoodFlowActivity
	for _, b := range journal {
		for _, activity := range b.Activities {
			if activity.Actor < 1 || activity.Actor > FoodFlowActorCount {
				return ErrFoodFlowCheckpoint
			}
			index := activity.Actor - 1
			revisions[index]++ // EffectStart
			if activity.Completed != 0 {
				revisions[index]++ // completion wake
			} else {
				copy := activity
				open[index] = &copy
			}
		}
	}
	expected := make(map[scheduler.Wake]bool, FoodFlowActorCount+1)
	if len(journal) == 0 {
		expected[scheduler.Wake{Actor: 1001, At: 0, Cause: scheduler.WakeAudit}] = true
	} else if hour < FoodFlowHorizonHours {
		next, _ := FoodFlowHourTime(hour + 1)
		expected[scheduler.Wake{Actor: 1001, At: next, Cause: scheduler.WakeAudit}] = true
	}
	for i, actor := range actors {
		fiber := snap.Fibers[i]
		id := sim.EntityID(i + 1)
		alive := actor.Energy > 0
		lifecycle := scheduler.Stopped
		if alive {
			lifecycle = scheduler.Alive
		}
		revision := revisions[i] + 1
		if !alive {
			revision++ // the zero-energy basal pulse stops the actor
		}
		if fiber.Actor != id || fiber.Lifecycle != lifecycle || fiber.Revision != revision {
			return ErrFoodFlowCheckpoint
		}
		activity := open[i]
		if activity != nil {
			if !alive || fiber.Activity == nil || fiber.Activity.Token != activity.Token {
				return ErrFoodFlowCheckpoint
			}
			var duration sim.Duration
			switch activity.Kind {
			case strategy.FoodFlowGather:
				if phase != 1 {
					return ErrFoodFlowCheckpoint
				}
				duration = foodFlowGatherDuration
			case strategy.FoodFlowEat:
				if phase != 2 {
					return ErrFoodFlowCheckpoint
				}
				duration = foodFlowMealDuration
			case strategy.FoodFlowRest:
				if phase != 2 {
					return ErrFoodFlowCheckpoint
				}
				duration = strategy.FoodFlowRestDuration
			default:
				return ErrFoodFlowCheckpoint
			}
			expected[scheduler.Wake{Actor: id, At: activity.Started + sim.SimTime(duration), Cause: scheduler.WakeCompletion}] = true
		} else if fiber.Activity != nil {
			return ErrFoodFlowCheckpoint
		}
		if len(journal) != 0 && phase == 0 && hour < FoodFlowHorizonHours && alive {
			at, _ := FoodFlowClaimTime(hour)
			expected[scheduler.Wake{Actor: id, At: at, Cause: scheduler.WakeNeedThreshold}] = true
		}
	}
	pulse := snap.Fibers[FoodFlowActorCount]
	if pulse.Actor != 1001 || pulse.Lifecycle != scheduler.Alive || pulse.Revision != 1 || pulse.Activity != nil || len(expected) != len(snap.Wakes) {
		return ErrFoodFlowCheckpoint
	}
	for _, wake := range snap.Wakes {
		if !expected[wake] {
			return ErrFoodFlowCheckpoint
		}
		delete(expected, wake)
	}
	if len(expected) != 0 {
		return ErrFoodFlowCheckpoint
	}
	return nil
}
