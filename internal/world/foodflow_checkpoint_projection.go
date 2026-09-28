package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
)

// Hourly projections are derived from verified accepted deltas rather than a
// second mutable checkpoint ledger. This also checks the runner's in-memory
// hourly trace before publication and reconstructs it after process restore.
func foodFlowCheckpointsFromHistory(k *kernel.Kernel, journal []FoodFlowBatch, yield int64) ([]FoodFlowCheckpoint, error) {
	var patches [FoodFlowPatchCount]FoodFlowPatchState
	var slots [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState
	var actors [FoodFlowActorCount]FoodFlowActorState
	for i := range patches {
		patches[i].Yield = yield
	}
	for i := range actors {
		actors[i] = FoodFlowActorState{Energy: FoodFlowInitialEnergy, LastGatherHour: FoodFlowNeverGatheredHour}
	}
	events := k.Events()
	index := 0
	out := make([]FoodFlowCheckpoint, 0, FoodFlowHorizonHours+1)
	for _, batch := range journal {
		for index < len(events) && events[index].Time == batch.Time {
			for _, delta := range events[index].Deltas {
				if err := applyFoodFlowCheckpointDelta(&patches, &slots, &actors, delta); err != nil {
					return nil, err
				}
			}
			index++
		}
		if batch.Version != sim.WorldVersion(index) {
			return nil, ErrFoodFlowCheckpoint
		}
		if foodFlowPhase(batch.Time) != 0 {
			continue
		}
		hour := int(batch.Time / sim.SimTime(FoodFlowHour))
		if hour != len(out) {
			return nil, ErrFoodFlowCheckpoint
		}
		balance, err := FoodFlowCheckConservation(patches, slots, actors)
		if err != nil {
			return nil, err
		}
		check := FoodFlowCheckpoint{Hour: hour, Version: batch.Version, Balance: balance, Patches: patches, Slots: slots, Actors: actors}
		for _, actor := range actors {
			if actor.Energy > 0 {
				check.Alive++
			}
		}
		out = append(out, check)
	}
	if index != len(events) {
		return nil, ErrFoodFlowCheckpoint
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	p, s, a, err := foodFlowStates(view)
	if err != nil || p != patches || s != slots || a != actors {
		return nil, ErrFoodFlowCheckpoint
	}
	return out, nil
}

func applyFoodFlowCheckpointDelta(p *[FoodFlowPatchCount]FoodFlowPatchState, s *[FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState, a *[FoodFlowActorCount]FoodFlowActorState, d component.FieldDelta) error {
	if d.Component == FoodFlowBagTypeID && d.Field == FoodFlowBagSourceField {
		if d.Entity < 1 || d.Entity > FoodFlowActorCount {
			return ErrFoodFlowCheckpoint
		}
		if d.After.State() == sim.Missing {
			a[d.Entity-1].BagSource = 0
			return nil
		}
		id, err := d.After.EntityRef()
		if err != nil {
			return ErrFoodFlowCheckpoint
		}
		a[d.Entity-1].BagSource = id
		return nil
	}
	n, err := d.After.Integer()
	if err != nil {
		return ErrFoodFlowCheckpoint
	}
	switch d.Component {
	case FoodFlowPatchTypeID:
		if d.Entity < 1001 || d.Entity > 1002 {
			return ErrFoodFlowCheckpoint
		}
		v := &p[d.Entity-1001]
		switch d.Field {
		case FoodFlowPatchPulsesField:
			v.Pulses = n
		case FoodFlowPatchProducedField:
			v.Produced = n
		case FoodFlowPatchUnrealizedField:
			v.Unrealized = n
		default:
			return ErrFoodFlowCheckpoint
		}
	case FoodFlowSlotTypeID:
		if d.Entity < 2001 || d.Entity > 2016 {
			return ErrFoodFlowCheckpoint
		}
		v := &s[(d.Entity-2001)/FoodFlowSlotsPerPatch][(d.Entity-2001)%FoodFlowSlotsPerPatch]
		switch d.Field {
		case FoodFlowSlotStockField:
			v.Stock = n
		case FoodFlowSlotGatheredField:
			v.Gathered = n
		default:
			return ErrFoodFlowCheckpoint
		}
	case FoodFlowBagTypeID, FoodFlowBodyTypeID:
		if d.Entity < 1 || d.Entity > FoodFlowActorCount {
			return ErrFoodFlowCheckpoint
		}
		v := &a[d.Entity-1]
		if d.Component == FoodFlowBagTypeID {
			if d.Field != FoodFlowBagUnitsField {
				return ErrFoodFlowCheckpoint
			}
			v.Bag = n
		} else {
			switch d.Field {
			case FoodFlowBodyEnergyField:
				v.Energy = n
			case FoodFlowBodyHungerField:
				v.Hunger = n
			case FoodFlowBodyBasalSpentField:
				v.BasalSpent = n
			case FoodFlowBodyCapLostField:
				v.CapLost = n
			case FoodFlowBodyConsumedField:
				v.Consumed = n
			case FoodFlowBodyLastGatherHourField:
				v.LastGatherHour = n
			default:
				return ErrFoodFlowCheckpoint
			}
		}
	default:
		return ErrFoodFlowCheckpoint
	}
	return nil
}
