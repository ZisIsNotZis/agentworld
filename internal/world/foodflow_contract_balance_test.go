package world

import (
	"agentworld/internal/sim"
	"errors"
	"testing"
)

func TestFoodFlowPatchAttributionAndHourlyAdmission(t *testing.T) {
	var patches [FoodFlowPatchCount]FoodFlowPatchState
	var slots [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState
	actors := foodFlowActors()
	patches[0].Yield, patches[1].Yield = 2, 1
	for i := range patches {
		var err error
		patches[i], slots[i], _, err = FoodFlowProduce(patches[i], slots[i], 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	firstSlot, _ := FoodFlowSlotID(0, 0)
	var err error
	slots[0][0], actors[0], err = FoodFlowGather(1001, firstSlot, 1, 0, slots[0][0], actors[0])
	if err != nil {
		t.Fatal(err)
	}
	actors[0], _, err = FoodFlowConsume(1001, 1, actors[0])
	if err != nil {
		t.Fatal(err)
	}
	secondSlot, _ := FoodFlowSlotID(0, 1)
	if _, _, err := FoodFlowGather(1001, secondSlot, 1, 0, slots[0][1], actors[0]); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatalf("second gather in h0 accepted: %v", err)
	}
	// A new pulse and elapsed basal cost permit the same actor in h1.
	actors[0], _, err = FoodFlowBasal(actors[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	patches[0], slots[0], _, err = FoodFlowProduce(patches[0], slots[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	slots[0][1], actors[0], err = FoodFlowGather(1001, secondSlot, 1, 1, slots[0][1], actors[0])
	if err != nil {
		t.Fatal(err)
	}
	actors[0], _, err = FoodFlowConsume(1001, 1, actors[0])
	if err != nil {
		t.Fatal(err)
	}
	id, _ := FoodFlowSlotID(1, 0)
	slots[1][0], actors[8], err = FoodFlowGather(1002, id, 9, 0, slots[1][0], actors[8])
	if err != nil {
		t.Fatal(err)
	}
	balance, err := FoodFlowCheckConservation(patches, slots, actors)
	if err != nil || balance.Produced != 5 || balance.Gathered != 3 || balance.Consumed != 2 || balance.Held != 1 || balance.Stock != 2 || balance.EnergyCapLost != 0 || balance.BasalSpent != 1 || balance.Energy != balance.InitialEnergy+1 {
		t.Fatalf("two-patch balance: %+v %v", balance, err)
	}
	invalid := actors
	invalid[8].BagSource = sim.EntityID(1001)
	if _, err := FoodFlowCheckConservation(patches, slots, invalid); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatal("cross-patch bag attribution accepted")
	}
	slots[0][0].Gathered++
	if _, err := FoodFlowCheckConservation(patches, slots, actors); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatal("unproduced gather accepted")
	}
}

// This isolated bounded state exercises energy-cap accounting. The standard
// one-gather/hour and one-basal/hour trajectory does not normally spill energy.
func TestFoodFlowIsolatedEnergyCapBalance(t *testing.T) {
	actors := foodFlowActors()
	actors[0] = FoodFlowActorState{Energy: 12, Consumed: 1, Bag: 1, BagSource: 1001, LastGatherHour: 1}
	patches := [FoodFlowPatchCount]FoodFlowPatchState{{Yield: 2, Pulses: 2, Produced: 2, Unrealized: 2}}
	var slots [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState
	slots[0][0].Gathered, slots[0][1].Gathered = 1, 1
	before, err := FoodFlowCheckConservation(patches, slots, actors)
	if err != nil || before.Consumed != 1 || before.Held != 1 {
		t.Fatalf("before cap spill: %+v %v", before, err)
	}
	var loss FoodFlowConsumption
	actors[0], loss, err = FoodFlowConsume(1001, 1, actors[0])
	if err != nil || loss != (FoodFlowConsumption{EnergyCapLost: 1}) {
		t.Fatalf("cap loss: %+v %v", loss, err)
	}
	after, err := FoodFlowCheckConservation(patches, slots, actors)
	if err != nil || after.Consumed != 2 || after.Held != 0 || after.EnergyCapLost != 1 || after.Energy != before.Energy {
		t.Fatalf("after cap spill: %+v %v", after, err)
	}
}
