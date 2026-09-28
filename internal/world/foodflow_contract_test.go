package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

func foodFlowActors() [FoodFlowActorCount]FoodFlowActorState {
	var actors [FoodFlowActorCount]FoodFlowActorState
	for i := range actors {
		actors[i].Energy = FoodFlowInitialEnergy
		actors[i].LastGatherHour = FoodFlowNeverGatheredHour
	}
	return actors
}

func TestFoodFlowClockAndDistinctIDs(t *testing.T) {
	if FoodFlowFormatVersion != 1 || FoodFlowHour != 3_600_000_000 || FoodFlowHorizonHours != 168 || FoodFlowInitialEnergy != 11 {
		t.Fatal("v1 clock or seed changed")
	}
	seen := map[sim.EntityID]bool{}
	for actor := sim.EntityID(1); actor <= FoodFlowActorCount; actor++ {
		owner, err := FoodFlowActorPatchID(actor)
		if err != nil || owner != sim.EntityID(1001+(actor-1)/8) {
			t.Fatalf("actor %d patch %d: %v", actor, owner, err)
		}
		seen[actor] = true
	}
	for p := 0; p < FoodFlowPatchCount; p++ {
		patch, err := FoodFlowPatchID(p)
		if err != nil || seen[patch] {
			t.Fatalf("patch %d: %v", patch, err)
		}
		seen[patch] = true
		for s := 0; s < FoodFlowSlotsPerPatch; s++ {
			slot, err := FoodFlowSlotID(p, s)
			if err != nil || seen[slot] {
				t.Fatalf("slot %d/%d: %v", p, s, err)
			}
			seen[slot] = true
		}
	}
	if len(seen) != 34 {
		t.Fatalf("IDs: %d", len(seen))
	}
	for _, h := range []int{-1, 169} {
		if _, err := FoodFlowHourTime(h); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("bad hour %d: %v", h, err)
		}
	}
	for _, h := range []int{-1, 168} {
		if _, err := FoodFlowClaimTime(h); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("bad claim hour %d: %v", h, err)
		}
	}
	start, err := FoodFlowClaimTime(0)
	if err != nil || start != 1 {
		t.Fatalf("first claim: %d %v", start, err)
	}
	end, err := FoodFlowClaimTime(167)
	if err != nil || end != 167*sim.SimTime(FoodFlowHour)+1 {
		t.Fatalf("last claim: %d %v", end, err)
	}
	for _, f := range []func() error{
		func() error { _, err := FoodFlowPatchID(-1); return err },
		func() error { _, err := FoodFlowPatchID(2); return err },
		func() error { _, err := FoodFlowSlotID(0, 8); return err },
		func() error { _, err := FoodFlowActorPatchID(17); return err },
	} {
		if !errors.Is(f(), ErrFoodFlowContract) {
			t.Fatal("bad ID accepted")
		}
	}
}

func TestFoodFlowRegistryVersionsAndProjectionProvenance(t *testing.T) {
	reg, err := FoodFlowRegistry()
	if err != nil || len(reg.TypeIDs()) != 4 {
		t.Fatalf("registry: %v %v", reg.TypeIDs(), err)
	}
	old, err := survivalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []component.ComponentDescriptor{FoodFlowPatchDescriptor(), FoodFlowSlotDescriptor(), FoodFlowBagDescriptor(), FoodFlowBodyDescriptor()} {
		if _, err := old.Describe(d.TypeID); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatalf("S6 recognized pilot type %d", d.TypeID)
		}
		got, err := reg.Describe(d.TypeID)
		if err != nil || !reflect.DeepEqual(got, d) || got.SchemaVersion != FoodFlowSchemaVersion || got.MigrationPolicy != component.MigrationRejectUnlisted {
			t.Fatalf("bad schema %d: %v", d.TypeID, err)
		}
		for _, rule := range got.TransitionRules {
			if rule.ID == 1 || rule.ID == component.WithdrawRuleID || rule.Version != FoodFlowRuleVersion {
				t.Fatalf("S6 rule alias or unpinned version: %+v", rule)
			}
		}
		for _, projection := range got.Projections {
			if projection.Version != FoodFlowProjectionVersion || !reflect.DeepEqual(projection.SourceFields, []sim.FieldID{sim.FieldID(projection.ID)}) || !reflect.DeepEqual(projection.Coefficients, []float64{1}) {
				t.Fatalf("projection not direct state: %+v", projection)
			}
		}
	}
	for _, id := range old.TypeIDs() {
		if _, err := reg.Describe(id); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatalf("pilot recognized S6 type %d", id)
		}
	}
	oldKernel, err := kernel.New(old, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldHistory, oldHead, err := oldKernel.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	pilotFingerprint, err := component.RegistryFingerprint(reg)
	if err != nil || pilotFingerprint == oldHead.RegistryFingerprint {
		t.Fatalf("S6 registry reused: %v", err)
	}
	if _, _, err := kernel.RestoreHistory(reg, oldHistory); err == nil {
		t.Fatal("S6 history accepted as food-flow v1")
	}
	patchID, _ := FoodFlowPatchID(0)
	slotID, _ := FoodFlowSlotID(0, 0)
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	patchRef, _ := sim.EntityRefValue(patchID)
	seeds := []component.ComponentSeed{
		{Entity: patchID, Component: FoodFlowPatchTypeID, Fields: []component.FieldSeed{{Field: FoodFlowPatchYieldField, Value: sim.IntegerValue(8)}, {Field: FoodFlowPatchPulsesField, Value: sim.IntegerValue(0)}, {Field: FoodFlowPatchProducedField, Value: sim.IntegerValue(0)}, {Field: FoodFlowPatchUnrealizedField, Value: sim.IntegerValue(0)}}},
		{Entity: slotID, Component: FoodFlowSlotTypeID, Fields: []component.FieldSeed{{Field: FoodFlowSlotStockField, Value: sim.IntegerValue(0)}, {Field: FoodFlowSlotPatchField, Value: patchRef}, {Field: FoodFlowSlotGatheredField, Value: sim.IntegerValue(0)}}},
		{Entity: 1, Component: FoodFlowBagTypeID, Fields: []component.FieldSeed{{Field: FoodFlowBagUnitsField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBagSourceField, Value: missing}}},
		{Entity: 1, Component: FoodFlowBodyTypeID, Fields: []component.FieldSeed{{Field: FoodFlowBodyEnergyField, Value: sim.IntegerValue(11)}, {Field: FoodFlowBodyHungerField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBodyBasalSpentField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBodyCapLostField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBodyConsumedField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBodyLastGatherHourField, Value: sim.IntegerValue(FoodFlowNeverGatheredHour)}}},
	}
	reader, authority, err := component.NewReader(reg, 7, seeds)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		typ   sim.ComponentTypeID
		id    sim.EntityID
		field sim.FieldID
		value int64
	}{
		{FoodFlowPatchTypeID, patchID, FoodFlowPatchYieldField, 8},
		{FoodFlowSlotTypeID, slotID, FoodFlowSlotStockField, 0},
		{FoodFlowSlotTypeID, slotID, FoodFlowSlotGatheredField, 0},
		{FoodFlowBagTypeID, 1, FoodFlowBagUnitsField, 0},
		{FoodFlowBodyTypeID, 1, FoodFlowBodyEnergyField, 11},
		{FoodFlowBodyTypeID, 1, FoodFlowBodyConsumedField, 0},
		{FoodFlowBodyTypeID, 1, FoodFlowBodyLastGatherHourField, FoodFlowNeverGatheredHour},
	} {
		metrics, err := reader.Project(component.ProjectRequest{Component: test.typ, Projection: sim.ProjectionID(test.field), Entities: []sim.EntityID{test.id}, WorldVersion: 7, Authority: authority})
		if err != nil || metrics.Len() != 1 {
			t.Fatalf("project %d: %v", test.typ, err)
		}
		metric, _ := metrics.At(0)
		got, err := metric.Value().Scalar() // engine projections produce scalar metrics from exact bounded integer source
		if err != nil || got != float64(test.value) || metric.ProjectionVersion() != FoodFlowProjectionVersion || metric.Uncertainty().State() != sim.Unknown || metric.Provenance().Entity != test.id || metric.Provenance().Component != test.typ || metric.Provenance().SchemaVersion != FoodFlowSchemaVersion || metric.Provenance().WorldVersion != 7 || !reflect.DeepEqual(metric.Provenance().SourceFields, []sim.FieldID{test.field}) {
			t.Fatalf("incorrect projection: %+v %v", metric, err)
		}
	}
	bad := append([]component.ComponentSeed(nil), seeds...)
	bad[0] = seeds[0]
	bad[0].Fields = append([]component.FieldSeed(nil), seeds[0].Fields...)
	bad[0].Fields[0].Value = sim.IntegerValue(9)
	if _, _, err := component.NewReader(reg, 7, bad); !errors.Is(err, component.ErrValueOutsideSchema) {
		t.Fatalf("invalid yield: %v", err)
	}
	bad = append([]component.ComponentSeed(nil), seeds...)
	bad[3] = seeds[3]
	bad[3].Fields = append([]component.FieldSeed(nil), seeds[3].Fields...)
	bad[3].Fields[1].Value = sim.IntegerValue(25)
	if _, _, err := component.NewReader(reg, 7, bad); !errors.Is(err, component.ErrValueOutsideSchema) {
		t.Fatalf("invalid hunger: %v", err)
	}
	for _, hour := range []int64{-2, 168} {
		bad = append([]component.ComponentSeed(nil), seeds...)
		bad[3].Fields = append([]component.FieldSeed(nil), seeds[3].Fields...)
		bad[3].Fields[5].Value = sim.IntegerValue(hour)
		if _, _, err := component.NewReader(reg, 7, bad); !errors.Is(err, component.ErrValueOutsideSchema) {
			t.Fatalf("invalid last gather hour %d: %v", hour, err)
		}
	}
}

func TestFoodFlowFixtureSupplyAndProductionCap(t *testing.T) {
	for _, fixture := range []struct {
		name string
		q    int64
	}{{"zero", 0}, {"scarce", 3}, {"abundant", 8}} {
		t.Run(fixture.name, func(t *testing.T) {
			p := FoodFlowPatchState{Yield: fixture.q}
			var slots [FoodFlowSlotsPerPatch]FoodFlowSlotState
			var firstEight int64
			for h := 0; h < FoodFlowHorizonHours; h++ {
				var result FoodFlowProduction
				var err error
				p, slots, result, err = FoodFlowProduce(p, slots, h)
				if err != nil || result.Produced != fixture.q || result.Unrealized != 0 {
					t.Fatalf("h%d: %+v %+v %v", h, p, result, err)
				}
				// Each window starts at h+1µs, before the next production pulse.
				for i := range slots {
					if h < 8 {
						firstEight += slots[i].Stock
					}
					slots[i].Gathered += slots[i].Stock
					slots[i].Stock = 0
				}
			}
			if firstEight != 8*fixture.q || p.Pulses != 168 || p.Produced != 168*fixture.q || p.Unrealized != 0 || !foodFlowStock(p, slots) {
				t.Fatalf("supply: first eight=%d patch=%+v", firstEight, p)
			}
			// With q=3 and eight actors on this patch, 24 slots are supplied
			// over h=0..7: precisely three per actor if admission rotates.
		})
	}
	p := FoodFlowPatchState{Yield: 8}
	var slots [FoodFlowSlotsPerPatch]FoodFlowSlotState
	p, slots, _, _ = FoodFlowProduce(p, slots, 0)
	p, slots, result, err := FoodFlowProduce(p, slots, 1)
	if err != nil || result != (FoodFlowProduction{Unrealized: 8}) || p.Produced != 8 || p.Unrealized != 8 {
		t.Fatalf("blocked inflow counted as unrealized: %+v %+v %v", p, result, err)
	}
	for _, attempt := range []struct {
		patch FoodFlowPatchState
		slots [8]FoodFlowSlotState
		h     int
	}{
		{FoodFlowPatchState{Yield: -1}, [8]FoodFlowSlotState{}, 0},
		{FoodFlowPatchState{Yield: 9}, [8]FoodFlowSlotState{}, 0},
		{FoodFlowPatchState{Yield: 8}, [8]FoodFlowSlotState{{Stock: 2}}, 0},
		{FoodFlowPatchState{Yield: 8}, [8]FoodFlowSlotState{}, 1},
		{FoodFlowPatchState{Yield: 8}, [8]FoodFlowSlotState{}, 168},
		{FoodFlowPatchState{Yield: 8, Produced: 1 << 62}, [8]FoodFlowSlotState{}, 0},
		{FoodFlowPatchState{Yield: 8}, [8]FoodFlowSlotState{{Gathered: 1 << 62}}, 0},
		{FoodFlowPatchState{Yield: 2, Pulses: 1, Produced: 2}, [8]FoodFlowSlotState{{Gathered: 2}}, 1},
	} {
		if _, _, _, err := FoodFlowProduce(attempt.patch, attempt.slots, attempt.h); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("invalid production: %+v", attempt)
		}
	}
	// Aggregate totals alone appear valid, but a single slot cannot have
	// supplied two units after only one production opportunity.
	badSlots := [FoodFlowSlotsPerPatch]FoodFlowSlotState{{Gathered: 2}}
	patch := FoodFlowPatchState{Yield: 2, Pulses: 1, Produced: 2}
	if foodFlowStock(patch, badSlots) {
		t.Fatal("slot minted more units than its production pulses")
	}
	actors := foodFlowActors()
	actors[0] = FoodFlowActorState{Energy: 12, Consumed: 1, LastGatherHour: 0}
	actors[1] = FoodFlowActorState{Energy: 11, Bag: 1, BagSource: 1001, LastGatherHour: 0}
	if _, err := FoodFlowCheckConservation([FoodFlowPatchCount]FoodFlowPatchState{patch}, [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState{badSlots}, actors); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatal("aggregate-consistent but per-slot impossible balance accepted")
	}
}

func foodFlowSnapshotInteger(t *testing.T, reader component.Reader, auth component.Authority, version sim.WorldVersion, entity sim.EntityID, typ sim.ComponentTypeID, field sim.FieldID) int64 {
	t.Helper()
	view, err := reader.Read(component.ReadRequest{Entity: entity, Component: typ, Fields: []sim.FieldID{field}, WorldVersion: version, Authority: auth})
	if err != nil {
		t.Fatal(err)
	}
	value, err := view.Value(field)
	if err != nil {
		t.Fatal(err)
	}
	n, err := value.Integer()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestFoodFlowTransferConservationAndEightIndependentClaims(t *testing.T) {
	var patches [FoodFlowPatchCount]FoodFlowPatchState
	var slots [FoodFlowPatchCount][FoodFlowSlotsPerPatch]FoodFlowSlotState
	actors := foodFlowActors()
	patches[0].Yield = 8
	patches[0], slots[0], _, _ = FoodFlowProduce(patches[0], slots[0], 0)
	var seeds []component.ComponentSeed
	patchRef, _ := sim.EntityRefValue(1001)
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	seeds = append(seeds, component.ComponentSeed{Entity: 1001, Component: FoodFlowPatchTypeID, Fields: []component.FieldSeed{
		{Field: FoodFlowPatchYieldField, Value: sim.IntegerValue(8)}, {Field: FoodFlowPatchPulsesField, Value: sim.IntegerValue(1)}, {Field: FoodFlowPatchProducedField, Value: sim.IntegerValue(8)}, {Field: FoodFlowPatchUnrealizedField, Value: sim.IntegerValue(0)}}})
	for i := 0; i < 8; i++ {
		actor := sim.EntityID(i + 1)
		slotID, _ := FoodFlowSlotID(0, i)
		seeds = append(seeds,
			component.ComponentSeed{Entity: slotID, Component: FoodFlowSlotTypeID, Fields: []component.FieldSeed{{Field: FoodFlowSlotStockField, Value: sim.IntegerValue(1)}, {Field: FoodFlowSlotPatchField, Value: patchRef}, {Field: FoodFlowSlotGatheredField, Value: sim.IntegerValue(0)}}},
			component.ComponentSeed{Entity: actor, Component: FoodFlowBagTypeID, Fields: []component.FieldSeed{{Field: FoodFlowBagUnitsField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBagSourceField, Value: missing}}},
			component.ComponentSeed{Entity: actor, Component: FoodFlowBodyTypeID, Fields: []component.FieldSeed{
				{Field: FoodFlowBodyEnergyField, Value: sim.IntegerValue(FoodFlowInitialEnergy)}, {Field: FoodFlowBodyHungerField, Value: sim.IntegerValue(0)},
				{Field: FoodFlowBodyBasalSpentField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBodyCapLostField, Value: sim.IntegerValue(0)},
				{Field: FoodFlowBodyConsumedField, Value: sim.IntegerValue(0)}, {Field: FoodFlowBodyLastGatherHourField, Value: sim.IntegerValue(FoodFlowNeverGatheredHour)}}})
	}
	reg, err := FoodFlowRegistry()
	if err != nil {
		t.Fatal(err)
	}
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ := k.Snapshot()
	var plans []kernel.Plan
	for i := 0; i < 8; i++ {
		actor := sim.EntityID(i + 1)
		slotID, _ := FoodFlowSlotID(0, i)
		var gathered FoodFlowSlotState
		gathered, actors[i], err = FoodFlowGather(1001, slotID, actor, 0, slots[0][i], actors[i])
		if err != nil {
			t.Fatal(err)
		}
		slots[0][i] = gathered
		proposal := kernel.Proposal{Key: string(rune('a' + i)), Time: 1, Cause: kernel.Cause{Actor: actor}, Rule: FoodFlowGatherRule, RuleVersion: FoodFlowRuleVersion,
			Patches: []component.Patch{
				{Entity: slotID, Component: FoodFlowSlotTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowSlotStockField, Value: sim.IntegerValue(0)},
				{Entity: slotID, Component: FoodFlowSlotTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowSlotGatheredField, Value: sim.IntegerValue(1)},
				{Entity: actor, Component: FoodFlowBagTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowBagUnitsField, Value: sim.IntegerValue(1)},
				{Entity: actor, Component: FoodFlowBagTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowBagSourceField, Value: patchRef},
				{Entity: actor, Component: FoodFlowBodyTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowBodyLastGatherHourField, Value: sim.IntegerValue(0)},
			}}
		plan, err := k.Plan(proposal, authority)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	// Reversed completion order must not drop any independent actor event.
	for i, j := 0, len(plans)-1; i < j; i, j = i+1, j-1 {
		plans[i], plans[j] = plans[j], plans[i]
	}
	events, err := k.CommitBatch(plans)
	if err != nil || len(events) != 8 {
		t.Fatalf("same-time actor events=%d: %v", len(events), err)
	}
	// Reconstruct the transferred state from the committed kernel snapshot,
	// not the helper's independently calculated arrays.
	reader, auth, version := k.Snapshot()
	if version != 8 {
		t.Fatalf("accepted version=%d, want 8", version)
	}
	history, _, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	replayed, _, err := kernel.RestoreHistory(reg, history)
	if err != nil {
		t.Fatalf("gather admission state failed history replay: %v", err)
	}
	replayedReader, replayedAuth, replayedVersion := replayed.Snapshot()
	if replayedVersion != version || foodFlowSnapshotInteger(t, replayedReader, replayedAuth, replayedVersion, 1, FoodFlowBodyTypeID, FoodFlowBodyLastGatherHourField) != 0 {
		t.Fatal("gather admission did not survive replay")
	}
	for i := 0; i < 8; i++ {
		actor := sim.EntityID(i + 1)
		slotID, _ := FoodFlowSlotID(0, i)
		slots[0][i] = FoodFlowSlotState{
			Stock:    foodFlowSnapshotInteger(t, reader, auth, version, slotID, FoodFlowSlotTypeID, FoodFlowSlotStockField),
			Gathered: foodFlowSnapshotInteger(t, reader, auth, version, slotID, FoodFlowSlotTypeID, FoodFlowSlotGatheredField),
		}
		actors[i].Bag = foodFlowSnapshotInteger(t, reader, auth, version, actor, FoodFlowBagTypeID, FoodFlowBagUnitsField)
		actors[i].LastGatherHour = foodFlowSnapshotInteger(t, reader, auth, version, actor, FoodFlowBodyTypeID, FoodFlowBodyLastGatherHourField)
		bag, err := reader.Read(component.ReadRequest{Entity: actor, Component: FoodFlowBagTypeID, Fields: []sim.FieldID{FoodFlowBagSourceField}, WorldVersion: version, Authority: auth})
		if err != nil {
			t.Fatal(err)
		}
		source, err := bag.Value(FoodFlowBagSourceField)
		if err != nil {
			t.Fatal(err)
		}
		actors[i].BagSource, err = source.EntityRef()
		if err != nil || actors[i].BagSource != 1001 || actors[i].LastGatherHour != 0 || slots[0][i] != (FoodFlowSlotState{Gathered: 1}) {
			t.Fatalf("actor=%d kernel gather state: slot=%+v bag=%+v %v", actor, slots[0][i], actors[i], err)
		}
	}
	balance, err := FoodFlowCheckConservation(patches, slots, actors)
	if err != nil || balance.Produced != 8 || balance.Gathered != 8 || balance.Held != 8 || balance.Stock != 0 {
		t.Fatalf("eight claims: %+v %v", balance, err)
	}
	for i := 0; i < 8; i++ {
		var meal FoodFlowConsumption
		actors[i], meal, err = FoodFlowConsume(1001, sim.EntityID(i+1), actors[i])
		if err != nil || meal.EnergyGained != 1 || actors[i].Energy != 12 {
			t.Fatalf("eat %d: %+v %v", i, meal, err)
		}
	}
	balance, err = FoodFlowCheckConservation(patches, slots, actors)
	if err != nil || balance.Consumed != 8 || balance.Held != 0 || balance.Energy != 184 {
		t.Fatalf("meals: %+v %v", balance, err)
	}
	actors[0].Consumed++
	actors[0].Energy++
	if _, err := FoodFlowCheckConservation(patches, slots, actors); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatal("duplicate food reward accepted")
	}
}

func TestFoodFlowRejectionsEnergyCapAndZeroInflux(t *testing.T) {
	a := FoodFlowActorState{Energy: FoodFlowInitialEnergy, LastGatherHour: FoodFlowNeverGatheredHour}
	for _, attempt := range []struct {
		patch, slot, actor sim.EntityID
		hour               int
		slotState          FoodFlowSlotState
		actorState         FoodFlowActorState
	}{
		{1001, 2001, 1, 0, FoodFlowSlotState{}, a},
		{1001, 2001, 9, 0, FoodFlowSlotState{Stock: 1}, a},
		{1001, 2009, 1, 0, FoodFlowSlotState{Stock: 1}, a},
		{1001, 2001, 1, 0, FoodFlowSlotState{Stock: 2}, a},
		{1001, 2001, 1, 0, FoodFlowSlotState{Stock: 1, Gathered: 1 << 62}, a},
		{1001, 2001, 17, 0, FoodFlowSlotState{Stock: 1}, a},
		{1001, 2001, 1, -1, FoodFlowSlotState{Stock: 1}, a},
		{1001, 2001, 1, 168, FoodFlowSlotState{Stock: 1}, a},
		{1001, 2001, 1, 0, FoodFlowSlotState{Stock: 1}, FoodFlowActorState{Energy: 11, Bag: 1, BagSource: 1001, LastGatherHour: 0}},
		{1001, 2001, 1, 0, FoodFlowSlotState{Stock: 1}, FoodFlowActorState{Energy: 13, LastGatherHour: FoodFlowNeverGatheredHour}},
		{1001, 2001, 1, 0, FoodFlowSlotState{Stock: 1}, FoodFlowActorState{Energy: 11, LastGatherHour: 0}},
	} {
		if _, _, err := FoodFlowGather(attempt.patch, attempt.slot, attempt.actor, attempt.hour, attempt.slotState, attempt.actorState); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("invalid gather: %+v", attempt)
		}
	}
	if _, _, err := FoodFlowConsume(1001, 1, a); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatalf("empty bag consumed: %v", err)
	}
	if _, _, err := FoodFlowBasal(a, 1<<62); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatalf("overflow time: %v", err)
	}
	if _, _, err := FoodFlowBasal(a, 0); !errors.Is(err, ErrFoodFlowContract) {
		t.Fatalf("zero time: %v", err)
	}
	for _, invalid := range []FoodFlowActorState{{Energy: 13, LastGatherHour: FoodFlowNeverGatheredHour}, {Energy: 11, Hunger: 25, LastGatherHour: FoodFlowNeverGatheredHour}, {Energy: 11, Consumed: 1 << 62, LastGatherHour: FoodFlowNeverGatheredHour}, {Energy: 11, BasalSpent: 1 << 62, LastGatherHour: FoodFlowNeverGatheredHour}, {Energy: 11, LastGatherHour: 168}, {Energy: 11, LastGatherHour: -2}} {
		if _, _, err := FoodFlowBasal(invalid, 1); !errors.Is(err, ErrFoodFlowContract) {
			t.Fatalf("invalid actor accepted: %+v", invalid)
		}
	}
	for h := int64(1); h <= 11; h++ {
		zero, _, err := FoodFlowBasal(a, h)
		if err != nil || zero.Energy != 11-h || zero.Hunger != h {
			t.Fatalf("zero inflow h%d: %+v %v", h, zero, err)
		}
	}
	dead := FoodFlowActorState{Energy: 0, Hunger: 11, BasalSpent: 11, LastGatherHour: FoodFlowNeverGatheredHour}
	dead, cost, err := FoodFlowBasal(dead, 13)
	if err != nil || dead.Hunger != 24 || dead.Energy != 0 || cost.UnmetEnergy != 13 {
		t.Fatalf("elapsed hunger after rejection/death: %+v %+v %v", dead, cost, err)
	}
}
