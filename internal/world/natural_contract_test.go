package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

var naturalMissingRef = func() sim.Value {
	v, err := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	if err != nil {
		panic(err)
	}
	return v
}()

// naturalNeutralSeeds builds the v5 genesis fixture: P patches with the
// frozen budget R, every slot wild and empty, and the 8P founders in the
// v3/v4 initial state with the given lifetime.
func naturalNeutralSeeds(patches int, R int64, lifetime int64) []component.ComponentSeed {
	var seeds []component.ComponentSeed
	for p := 0; p < patches; p++ {
		patchID, _ := NaturalPatchID(patches, p)
		patchRef, _ := sim.EntityRefValue(patchID)
		seeds = append(seeds, component.ComponentSeed{Entity: patchID, Component: NaturalPatchTypeID, Fields: []component.FieldSeed{
			{Field: NaturalPatchYieldField, Value: sim.IntegerValue(R)}, {Field: NaturalPatchPulsesField, Value: sim.IntegerValue(0)},
			{Field: NaturalPatchProducedField, Value: sim.IntegerValue(0)}, {Field: NaturalPatchGrantedField, Value: sim.IntegerValue(0)},
			{Field: NaturalPatchUnrealizedField, Value: sim.IntegerValue(0)}}})
		for s := 0; s < NaturalSlotsPerPatch; s++ {
			slotID, _ := NaturalSlotID(patches, p, s)
			seeds = append(seeds, component.ComponentSeed{Entity: slotID, Component: NaturalSlotTypeID, Fields: []component.FieldSeed{
				{Field: NaturalSlotStockField, Value: sim.IntegerValue(0)}, {Field: NaturalSlotPatchField, Value: patchRef},
				{Field: NaturalSlotGatheredField, Value: sim.IntegerValue(0)}, {Field: NaturalSlotTenureModeField, Value: sim.IntegerValue(int64(NaturalTenureWild))},
				{Field: NaturalSlotTenureOwnerField, Value: naturalMissingRef}, {Field: NaturalSlotTenureCultivatedField, Value: sim.IntegerValue(0)}}})
		}
	}
	founders, _ := NaturalFounderCount(patches)
	for actor := 1; actor <= founders; actor++ {
		state, err := NaturalFounderState(patches, sim.EntityID(actor), lifetime)
		if err != nil {
			panic(err)
		}
		seeds = append(seeds, naturalActorSeeds(sim.EntityID(actor), state)...)
	}
	return seeds
}

// naturalActorSeeds converts one actor's six rows into component seeds.
func naturalActorSeeds(actor sim.EntityID, a NaturalActorState) []component.ComponentSeed {
	homeRef, _ := sim.EntityRefValue(a.Bag.Source)
	parentARef, _ := genomeRefValue(a.Genome.ParentA)
	parentBRef, _ := genomeRefValue(a.Genome.ParentB)
	addresseeRef, _ := genomeRefValue(a.Request.Addressee)
	return []component.ComponentSeed{
		{Entity: actor, Component: NaturalBagTypeID, Fields: []component.FieldSeed{
			{Field: NaturalBagUnitsField, Value: sim.IntegerValue(a.Bag.Units)}, {Field: NaturalBagSourceField, Value: homeRef}}},
		{Entity: actor, Component: NaturalBodyTypeID, Fields: []component.FieldSeed{
			{Field: NaturalBodyEnergyField, Value: sim.IntegerValue(a.Body.Energy)}, {Field: NaturalBodyHungerField, Value: sim.IntegerValue(a.Body.Hunger)},
			{Field: NaturalBodyBasalSpentField, Value: sim.IntegerValue(a.Body.BasalSpent)}, {Field: NaturalBodyCapLostField, Value: sim.IntegerValue(a.Body.CapLost)},
			{Field: NaturalBodyConsumedField, Value: sim.IntegerValue(a.Body.Consumed)}, {Field: NaturalBodyLastGatherHourField, Value: sim.IntegerValue(a.Body.LastGatherHour)},
			{Field: NaturalBodyBasalDebtField, Value: sim.IntegerValue(a.Body.BasalDebt)}}},
		{Entity: actor, Component: NaturalWorksiteTypeID, Fields: []component.FieldSeed{
			{Field: NaturalWorksiteCapitalField, Value: sim.IntegerValue(a.Worksite.Capital)}, {Field: NaturalWorksiteWipField, Value: sim.IntegerValue(a.Worksite.Wip)},
			{Field: NaturalWorksiteWearDebtField, Value: sim.IntegerValue(a.Worksite.WearDebt)}, {Field: NaturalWorksiteInvestedUnitsField, Value: sim.IntegerValue(a.Worksite.InvestedUnits)},
			{Field: NaturalWorksitePointsCreatedField, Value: sim.IntegerValue(a.Worksite.PointsCreated)}, {Field: NaturalWorksitePointsDecayedField, Value: sim.IntegerValue(a.Worksite.PointsDecayed)},
			{Field: NaturalWorksiteLastBuildHourField, Value: sim.IntegerValue(a.Worksite.LastBuildHour)}}},
		{Entity: actor, Component: NaturalGranaryTypeID, Fields: []component.FieldSeed{
			{Field: NaturalGranaryStockField, Value: sim.IntegerValue(a.Granary.Stock)}, {Field: NaturalGranaryCultivatedTotalField, Value: sim.IntegerValue(a.Granary.CultivatedTotal)},
			{Field: NaturalGranaryStoredMealsField, Value: sim.IntegerValue(a.Granary.StoredMeals)}, {Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(a.Granary.LastStoredMealHour)},
			{Field: NaturalGranarySpoilDebtField, Value: sim.IntegerValue(a.Granary.SpoilDebt)}, {Field: NaturalGranarySpoiledField, Value: sim.IntegerValue(a.Granary.Spoiled)}}},
		{Entity: actor, Component: NaturalGenomeTypeID, Fields: []component.FieldSeed{
			{Field: NaturalGenomeLocusMField, Value: sim.IntegerValue(a.Genome.LocusM)}, {Field: NaturalGenomeLocusYField, Value: sim.IntegerValue(a.Genome.LocusY)},
			{Field: NaturalGenomeLocusWField, Value: sim.IntegerValue(a.Genome.LocusW)}, {Field: NaturalGenomeParentAField, Value: parentARef},
			{Field: NaturalGenomeParentBField, Value: parentBRef}, {Field: NaturalGenomeBirthHourField, Value: sim.IntegerValue(a.Genome.BirthHour)},
			{Field: NaturalGenomeDeathHourField, Value: sim.IntegerValue(a.Genome.DeathHour)}, {Field: NaturalGenomeDiedHourField, Value: sim.IntegerValue(a.Genome.DiedHour)},
			{Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(a.Genome.BirthGranaryPaid)}}},
		{Entity: actor, Component: NaturalBirthRequestTypeID, Fields: []component.FieldSeed{
			{Field: NaturalBirthRequestHourField, Value: sim.IntegerValue(a.Request.Hour)}, {Field: NaturalBirthRequestAddresseeField, Value: addresseeRef},
			{Field: NaturalBirthRequestReportedCapitalField, Value: sim.IntegerValue(a.Request.ReportedCapital)}, {Field: NaturalBirthRequestReportedGranaryField, Value: sim.IntegerValue(a.Request.ReportedGranary)},
			{Field: NaturalBirthRequestStatusField, Value: sim.IntegerValue(int64(a.Request.Status))}, {Field: NaturalBirthRequestLastRequestHourField, Value: sim.IntegerValue(a.Request.LastRequestHour)},
			{Field: NaturalBirthRequestLastReplyHourField, Value: sim.IntegerValue(a.Request.LastReplyHour)}}},
	}
}

func TestNaturalConstantsClockRangesAndDistinctIDs(t *testing.T) {
	if NaturalFormatVersion != 5 || NaturalSchemaVersion != 5 || NaturalRuleVersion != 5 || NaturalProjectionVersion != 5 {
		t.Fatal("v5 versions")
	}
	typeIDs := []sim.ComponentTypeID{NaturalPatchTypeID, NaturalSlotTypeID, NaturalBagTypeID, NaturalBodyTypeID, NaturalWorksiteTypeID, NaturalGranaryTypeID, NaturalGenomeTypeID, NaturalBirthRequestTypeID}
	seen := map[sim.ComponentTypeID]bool{}
	for i, id := range typeIDs {
		if id != NaturalTypeBlockFirst+sim.ComponentTypeID(i) || seen[id] {
			t.Fatalf("type ID %d at %d", id, i)
		}
		if id < NaturalTypeBlockFirst || id > NaturalTypeBlockLast {
			t.Fatalf("type ID %d outside the frozen block", id)
		}
		seen[id] = true
	}
	for id := sim.RuleID(501); id <= 514; id++ {
		if !naturalRulesDeclared[id] {
			t.Fatalf("rule %d not declared on any descriptor", id)
		}
	}
	if naturalRulesDeclared[500] || naturalRulesDeclared[515] {
		t.Fatal("rule range 501-514")
	}
	// Entity ranges at the default P=2 and the matrix P=32.
	if patch, err := NaturalPatchID(2, 0); err != nil || patch != 1001 {
		t.Fatalf("patch 0: %d %v", patch, err)
	}
	if patch, err := NaturalPatchID(2, 1); err != nil || patch != 1002 {
		t.Fatalf("patch 1: %d %v", patch, err)
	}
	if _, err := NaturalPatchID(2, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("patch 2 at P=2")
	}
	if slot, err := NaturalSlotID(2, 1, 7); err != nil || slot != 2016 {
		t.Fatalf("last slot at P=2: %d %v", slot, err)
	}
	if _, err := NaturalSlotID(2, 2, 0); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("slot out of range")
	}
	if home, err := NaturalFounderPatchID(2, 9); err != nil || home != 1002 {
		t.Fatalf("founder 9 home: %d %v", home, err)
	}
	if _, err := NaturalFounderPatchID(2, 17); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("founder 17 at P=2")
	}
	if founders, err := NaturalFounderCount(2); err != nil || founders != 16 {
		t.Fatalf("founders at P=2: %d %v", founders, err)
	}
	if founders, err := NaturalFounderCount(32); err != nil || founders != 256 {
		t.Fatalf("founders at P=32: %d %v", founders, err)
	}
	if _, err := NaturalFounderCount(0); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("P=0 admitted")
	}
	// The allocator genesis last-ID is the highest fixed entity ID
	// (2000+8P); newborn IDs ascend strictly above with no range and no
	// cap.
	if last, err := NaturalHighestFixedEntityID(2); err != nil || last != 2016 {
		t.Fatalf("highest fixed at P=2: %d %v", last, err)
	}
	if last, err := NaturalHighestFixedEntityID(32); err != nil || last != 2256 {
		t.Fatalf("highest fixed at P=32: %d %v", last, err)
	}
	alloc, err := NaturalGenesisAllocator(2)
	if err != nil || alloc.Last() != 2016 {
		t.Fatalf("genesis allocator: %v %v", alloc.Last(), err)
	}
	first, err := NaturalNextNewbornID(&alloc)
	if err != nil || first != 2017 {
		t.Fatalf("first newborn: %d %v", first, err)
	}
	second, err := NaturalNextNewbornID(&alloc)
	if err != nil || second != 2018 {
		t.Fatalf("second newborn: %d %v", second, err)
	}
	// Phase clock: byte-identical to the v4 frozen offsets.
	base := sim.SimTime(5) * sim.SimTime(NaturalHour)
	for _, phase := range []struct {
		n    int
		want sim.SimTime
	}{
		{NaturalPhasePulse, base},
		{NaturalPhaseClaim, base + 1},
		{NaturalPhaseGathered, base + sim.SimTime(NaturalGatherDuration) + 1},
		{NaturalPhaseBagDecision, base + sim.SimTime(NaturalGatherDuration) + 2},
		{NaturalPhaseBuildStart, base + sim.SimTime(NaturalGatherDuration) + 3},
		{NaturalPhasePairedMeal, base + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 4},
		{NaturalPhaseBirthRequest, base + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 5},
		{NaturalPhaseBirthReply, base + sim.SimTime(NaturalGatherDuration+NaturalBuildDuration) + 6},
	} {
		at, err := NaturalPhaseTime(5, phase.n)
		if err != nil || at != phase.want {
			t.Fatalf("phase %d: %d %v", phase.n, at, err)
		}
		v4At, v4Err := ReproductionPhaseTime(5, phase.n)
		if v4Err != nil || v4At != at {
			t.Fatalf("phase %d drifts from v4: %d %v", phase.n, v4At, v4Err)
		}
	}
	for _, bad := range []struct{ hour, phase int }{{-1, 0}, {168, 0}, {5, -1}, {5, 8}} {
		if _, err := NaturalPhaseTime(bad.hour, bad.phase); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("bad phase %+v", bad)
		}
	}
	// Frozen stream constants and unbounded draw positions.
	if NaturalBirthStream != 0x47454E33 || NaturalFounderStream != 0x464F554E {
		t.Fatal("frozen streams")
	}
	position, err := NaturalBirthDrawPosition(0)
	if err != nil || position != 0 {
		t.Fatalf("first ordinal position: %d %v", position, err)
	}
	position, err = NaturalBirthDrawPosition(4)
	if err != nil || position != 28 {
		t.Fatalf("fifth ordinal position: %d %v", position, err)
	}
	position, err = NaturalBirthDrawPosition(1_000_000)
	if err != nil || position != 7_000_000 {
		t.Fatalf("unbounded ordinal position: %d %v", position, err)
	}
	if _, err := NaturalBirthDrawPosition(-1); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("negative ordinal admitted")
	}
}

var naturalRulesDeclared = func() map[sim.RuleID]bool {
	declared := map[sim.RuleID]bool{}
	for _, d := range []component.ComponentDescriptor{NaturalPatchDescriptor(), NaturalSlotDescriptor(), NaturalBagDescriptor(), NaturalBodyDescriptor(), NaturalWorksiteDescriptor(), NaturalGranaryDescriptor(), NaturalGenomeDescriptor(), NaturalBirthRequestDescriptor()} {
		for _, rule := range d.TransitionRules {
			declared[rule.ID] = true
		}
	}
	return declared
}()

func TestNaturalRegistryIsolationAndIdentityProjections(t *testing.T) {
	reg, err := NaturalRegistry()
	if err != nil || len(reg.TypeIDs()) != 8 {
		t.Fatalf("registry: %v %v", reg.TypeIDs(), err)
	}
	descriptors := []component.ComponentDescriptor{NaturalPatchDescriptor(), NaturalSlotDescriptor(), NaturalBagDescriptor(), NaturalBodyDescriptor(), NaturalWorksiteDescriptor(), NaturalGranaryDescriptor(), NaturalGenomeDescriptor(), NaturalBirthRequestDescriptor()}
	ruleIDs := map[sim.RuleID]bool{}
	for _, d := range descriptors {
		got, err := reg.Describe(d.TypeID)
		if err != nil || !reflect.DeepEqual(got, d) {
			t.Fatalf("descriptor %d: %v", d.TypeID, err)
		}
		if got.SchemaVersion != NaturalSchemaVersion || got.MigrationPolicy != component.MigrationRejectUnlisted || got.StorageClass != component.DynamicStorage {
			t.Fatalf("descriptor %d identity: %+v", d.TypeID, got)
		}
		localRules := map[sim.RuleID]bool{}
		for _, rule := range got.TransitionRules {
			if rule.Version != NaturalRuleVersion || rule.ID < 501 || rule.ID > 514 || localRules[rule.ID] {
				t.Fatalf("rule %+v", rule)
			}
			localRules[rule.ID] = true
			ruleIDs[rule.ID] = true
		}
		for _, field := range got.Fields {
			for _, projection := range got.Projections {
				if projection.Version != NaturalProjectionVersion || projection.MissingBehavior != component.PreserveSourceState {
					t.Fatalf("projection identity: %+v", projection)
				}
			}
			if field.Type.Kind != sim.IntegerKind {
				continue // ref fields carry no projection
			}
			var projection component.ProjectionDescriptor
			for _, p := range got.Projections {
				if sim.FieldID(p.ID) == field.ID {
					projection = p
				}
			}
			if !reflect.DeepEqual(projection.SourceFields, []sim.FieldID{field.ID}) || !reflect.DeepEqual(projection.Coefficients, []float64{1}) ||
				projection.Unit != field.Unit || projection.Bounds != field.Bounds || projection.ID == 0 {
				t.Fatalf("field %d projection not identity: %+v", field.ID, projection)
			}
		}
	}
	for id := 501; id <= 514; id++ {
		if !ruleIDs[sim.RuleID(id)] {
			t.Fatalf("missing rule %d", id)
		}
	}
	// Frozen bounds on every integer field, quoted from the design.
	bounds := map[sim.ComponentTypeID]map[sim.FieldID][2]int64{
		NaturalPatchTypeID: {NaturalPatchYieldField: {0, 8}, NaturalPatchPulsesField: {0, 168}, NaturalPatchProducedField: {0, 1344}, NaturalPatchGrantedField: {0, 1344}, NaturalPatchUnrealizedField: {0, 1344}},
		NaturalSlotTypeID:  {NaturalSlotStockField: {0, 1}, NaturalSlotGatheredField: {0, 168}, NaturalSlotTenureModeField: {0, 1}, NaturalSlotTenureCultivatedField: {0, 1}},
		NaturalBagTypeID:   {NaturalBagUnitsField: {0, 1}},
		NaturalBodyTypeID: {NaturalBodyEnergyField: {0, 12}, NaturalBodyHungerField: {0, 24}, NaturalBodyBasalSpentField: {0, 1008}, NaturalBodyCapLostField: {0, 168},
			NaturalBodyConsumedField: {0, 168}, NaturalBodyLastGatherHourField: {-1, 167}, NaturalBodyBasalDebtField: {0, 13}},
		NaturalWorksiteTypeID: {NaturalWorksiteCapitalField: {0, 8}, NaturalWorksiteWipField: {0, 1}, NaturalWorksiteWearDebtField: {0, 14}, NaturalWorksiteInvestedUnitsField: {0, 168},
			NaturalWorksitePointsCreatedField: {0, 168}, NaturalWorksitePointsDecayedField: {0, 168}, NaturalWorksiteLastBuildHourField: {-1, 167}},
		NaturalGranaryTypeID: {NaturalGranaryStockField: {0, 1344}, NaturalGranaryCultivatedTotalField: {0, 1344}, NaturalGranaryStoredMealsField: {0, 168},
			NaturalGranaryLastStoredMealHourField: {-1, 167}, NaturalGranarySpoilDebtField: {0, 1367}, NaturalGranarySpoiledField: {0, 1344}},
		NaturalGenomeTypeID: {NaturalGenomeLocusMField: {5, 7}, NaturalGenomeLocusYField: {5, 7}, NaturalGenomeLocusWField: {5, 7},
			NaturalGenomeBirthHourField: {0, 168}, NaturalGenomeDeathHourField: {0, 479}, NaturalGenomeDiedHourField: {-1, 168}, NaturalGenomeBirthGranaryPaidField: {0, 672}},
		NaturalBirthRequestTypeID: {NaturalBirthRequestHourField: {-1, 167}, NaturalBirthRequestReportedCapitalField: {0, 8}, NaturalBirthRequestReportedGranaryField: {0, 1344},
			NaturalBirthRequestStatusField: {0, 4}, NaturalBirthRequestLastRequestHourField: {-1, 167}, NaturalBirthRequestLastReplyHourField: {-1, 167}},
	}
	for typeID, fields := range bounds {
		got, err := reg.Describe(typeID)
		if err != nil {
			t.Fatal(err)
		}
		byField := map[sim.FieldID]component.FieldDescriptor{}
		for _, field := range got.Fields {
			byField[field.ID] = field
		}
		for field, want := range fields {
			if _, ok := byField[field]; !ok {
				t.Fatalf("type %d missing field %d", typeID, field)
			}
			gotBounds := byField[field].Bounds
			if !gotBounds.HasMinimum || !gotBounds.HasMaximum || int64(gotBounds.Minimum) != want[0] || int64(gotBounds.Maximum) != want[1] {
				t.Fatalf("type %d field %d bounds %+v want %v", typeID, field, gotBounds, want)
			}
		}
	}
	// The registry is disjoint from every earlier world.
	priors := []component.Registry{}
	for _, build := range []func() (component.Registry, error){FoodFlowRegistry, SocialFoodRegistry, CapacityRegistry, ReproductionRegistry, survivalRegistry} {
		old, err := build()
		if err != nil {
			t.Fatal(err)
		}
		priors = append(priors, old)
	}
	for _, prior := range priors {
		for _, priorID := range prior.TypeIDs() {
			if _, err := reg.Describe(priorID); err == nil {
				t.Fatalf("v5 registry collides with prior type %d", priorID)
			}
		}
	}
	// Bidirectional cross-version rejection: a v5 seed set cannot restore
	// into any earlier registry, and no earlier seed set can restore into
	// the v5 registry.
	if _, _, err := component.NewReader(reg, 7, naturalNeutralSeeds(2, 8, 120)); err != nil {
		t.Fatalf("v5 seeds into v5 registry: %v", err)
	}
	if _, _, err := component.NewReader(reg, 7, capacityNeutralSeeds(8)); err == nil {
		t.Fatal("v3 seeds restored into the v5 registry")
	}
	priorRegistry, err := CapacityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	v4Registry, err := ReproductionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range naturalNeutralSeeds(2, 8, 120) {
		if _, _, err := component.NewReader(priorRegistry, 7, []component.ComponentSeed{seed}); err == nil {
			t.Fatal("v5 seed restored into the v3 registry")
		}
		if _, _, err := component.NewReader(v4Registry, 7, []component.ComponentSeed{seed}); err == nil {
			t.Fatal("v5 seed restored into the v4 registry")
		}
		break
	}
	if _, _, err := component.NewReader(reg, 7, reproductionNeutralSeeds()); err == nil {
		t.Fatal("v4 seeds restored into the v5 registry")
	}
}

// TestNaturalIDSpacePartitionAndAllocatorFreshness pins the corrigendum's
// P15 property: the fixed blocks (actors 1..8P, patches 1001..1000+P, slots
// 2001..2000+8P) are pairwise disjoint and strictly below the allocator
// range, so a newborn ID can never collide with a fixed entity, the
// newborn validity predicate rejects every fixed-block ID, and both the
// allocator and the predicate keep working arbitrarily far past the former
// legible range (synthetic states; no real births are run).
func TestNaturalIDSpacePartitionAndAllocatorFreshness(t *testing.T) {
	for _, patches := range []int{1, 2, 32} {
		founders, _ := NaturalFounderCount(patches)
		genesisLast, err := NaturalHighestFixedEntityID(patches)
		if err != nil || genesisLast != sim.EntityID(2000+8*patches) {
			t.Fatalf("P=%d genesis last: %d %v", patches, genesisLast, err)
		}
		// Pairwise block disjointness.
		fixed := map[sim.EntityID]bool{}
		for id := 1; id <= founders; id++ {
			fixed[sim.EntityID(id)] = true
		}
		for p := 0; p < patches; p++ {
			patchID, _ := NaturalPatchID(patches, p)
			if patchID <= sim.EntityID(founders) || patchID > genesisLast || fixed[patchID] {
				t.Fatalf("P=%d patch block collides: %d", patches, patchID)
			}
			fixed[patchID] = true
			for s := 0; s < NaturalSlotsPerPatch; s++ {
				slotID, _ := NaturalSlotID(patches, p, s)
				if slotID <= sim.EntityID(founders) || slotID > genesisLast || fixed[slotID] {
					t.Fatalf("P=%d slot block collides: %d", patches, slotID)
				}
				fixed[slotID] = true
			}
		}
		if len(fixed) != founders+patches+NaturalSlotsPerPatch*patches {
			t.Fatalf("P=%d fixed block overlap", patches)
		}
		// The newborn predicate accepts only allocator-fresh IDs.
		if !naturalNewbornEntity(patches, genesisLast+1) {
			t.Fatalf("P=%d first newborn rejected", patches)
		}
		// The actor predicate covers founders plus allocator-fresh IDs.
		for id := sim.EntityID(0); id <= genesisLast+1; id++ {
			want := (id >= 1 && id <= sim.EntityID(founders)) || id > genesisLast
			if naturalActorEntity(patches, id) != want {
				t.Fatalf("P=%d actor predicate at %d", patches, id)
			}
		}
		for id := range fixed {
			if naturalNewbornEntity(patches, id) {
				t.Fatalf("P=%d fixed ID %d accepted as newborn", patches, id)
			}
		}
		if naturalNewbornEntity(patches, 0) || naturalNewbornEntity(patches, math.MaxUint64) {
			t.Fatal("zero or MaxUint64 accepted as newborn")
		}
	}
	// Synthetic high ordinal: 5000 allocator draws past the former patch
	// block, then a constructed newborn state at that ID remains valid and
	// the world stays conserved.
	alloc, err := NaturalGenesisAllocator(2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5000; i++ {
		if _, err := alloc.Next(); err != nil {
			t.Fatal(err)
		}
	}
	highID, err := NaturalNextNewbornID(&alloc)
	if err != nil || highID != 7017 {
		t.Fatalf("high-ordinal newborn: %d %v", highID, err)
	}
	proposer, err := NaturalFounderState(2, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	consenter, err := NaturalFounderState(2, 2, 140)
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := NaturalChildGenome(2, 1, 2, proposer.Genome, consenter.Genome, 0, []uint64{0, 0, 0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if !validNaturalGenome(2, highID, child) || !naturalNewbornEntity(2, highID) {
		t.Fatal("high-ordinal newborn genome rejected")
	}
	newborn := NaturalNewbornState{ID: highID, State: NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: NaturalNeverHour},
		Bag:      NaturalBagState{Source: 1001},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   child,
		Request:  NaturalIdleRequestState(),
	}}
	if !validNaturalActor(2, highID, newborn.State) {
		t.Fatal("high-ordinal actor rejected")
	}
	patchState := NaturalPatchState{}
	slotStates := make([][]NaturalSlotState, 2)
	for p := range slotStates {
		slotStates[p] = make([]NaturalSlotState, NaturalSlotsPerPatch)
	}
	founders := make([]NaturalActorState, 16)
	for i := range founders {
		founders[i], err = NaturalFounderState(2, sim.EntityID(i+1), 140)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NaturalCheckConservation(2, []NaturalPatchState{patchState, patchState}, slotStates, founders, []NaturalNewbornState{newborn}, highID); err != nil {
		t.Fatalf("high-ordinal world conserved: %v", err)
	}
	// Allocator overflow is the only typed failure (P15).
	exhausted := sim.NewEntityAllocator(math.MaxUint64)
	if _, err := NaturalNextNewbornID(&exhausted); !errors.Is(err, sim.ErrOverflow) {
		t.Fatalf("overflow: %v", err)
	}
	// The entity-create builder rejects every fixed-block ID as a newborn.
	for _, bad := range []sim.EntityID{0, 1, 8, 1001, 1002, 2001, 2016} {
		if _, err := naturalNewbornAllocationsFor(NaturalMechanismV5, 2, bad, 1001, newborn.State); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("fixed-block allocation %d survived", bad)
		}
	}
}

func TestNaturalOwnershipGuardTenureGenomeAndNegativeFixtures(t *testing.T) {
	patch := func(rule sim.RuleID, cause kernel.Cause, entity sim.EntityID, componentType sim.ComponentTypeID, field sim.FieldID, key string) kernel.Proposal {
		return kernel.Proposal{Key: key, Time: 5, Cause: cause, Rule: rule, RuleVersion: NaturalRuleVersion,
			Patches: []component.Patch{{Entity: entity, Component: componentType, SchemaVersion: NaturalSchemaVersion, Field: field, Value: sim.IntegerValue(0)}}}
	}
	world := kernel.Cause{World: true}
	slot := func(rule sim.RuleID, cause kernel.Cause, field sim.FieldID, key string) kernel.Proposal {
		return patch(rule, cause, 2001, NaturalSlotTypeID, field, key)
	}
	// G5 tenure: only rules 503/505/514 may write mode, owner, or flag.
	for _, key := range []string{"tenure-by-wear", "tenure-by-build", "tenure-by-age-death"} {
		rule, cause := NaturalWearRule, world
		switch key {
		case "tenure-by-build":
			rule, cause = NaturalBuildRule, kernel.Cause{Actor: 1}
		case "tenure-by-age-death":
			rule, cause = NaturalAgeDeathRule, world
		}
		for _, field := range []sim.FieldID{NaturalSlotTenureModeField, NaturalSlotTenureOwnerField, NaturalSlotTenureCultivatedField} {
			if err := NaturalCheckOwnership(slot(rule, cause, field, key), 2); err != nil {
				t.Fatalf("%s field %d: %v", key, field, err)
			}
		}
	}
	for _, rule := range []sim.RuleID{NaturalRegenSplitRule, NaturalSpoilageRule, NaturalBasalRule, NaturalGatherRule, NaturalConsumeRule, NaturalGrantRule, NaturalEatStoredRule, NaturalBirthRequestRule, NaturalBirthReplyRule, NaturalBirthRefuseRule, NaturalBirthExpireRule} {
		cause := world
		if !naturalWorldRule(rule) {
			cause = kernel.Cause{Actor: 1}
		}
		for _, field := range []sim.FieldID{NaturalSlotTenureModeField, NaturalSlotTenureOwnerField, NaturalSlotTenureCultivatedField} {
			if err := NaturalCheckOwnership(slot(rule, cause, field, "forged-tenure"), 2); !errors.Is(err, ErrNaturalContract) {
				t.Fatalf("forged tenure under rule %d survived", rule)
			}
		}
	}
	// Slot stock: regen-split (world), wear (world), gather (actor); slot
	// gathered: gather only. Everything else is rejected.
	if err := NaturalCheckOwnership(slot(NaturalRegenSplitRule, world, NaturalSlotStockField, "fill"), 2); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := NaturalCheckOwnership(slot(NaturalGatherRule, kernel.Cause{Actor: 1}, NaturalSlotGatheredField, "gathered"), 2); err != nil {
		t.Fatalf("gathered: %v", err)
	}
	if err := NaturalCheckOwnership(slot(NaturalConsumeRule, kernel.Cause{Actor: 1}, NaturalSlotStockField, "consume-stock"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("consume writes slot stock")
	}
	if err := NaturalCheckOwnership(slot(NaturalGrantRule, world, NaturalSlotStockField, "grant-stock"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("grant writes slot stock")
	}
	// G5 genome immutability: died-hour only by rules 504/514.
	for _, key := range []string{"died-by-basal", "died-by-age-death"} {
		rule := NaturalBasalRule
		if key == "died-by-age-death" {
			rule = NaturalAgeDeathRule
		}
		if err := NaturalCheckOwnership(patch(rule, world, 1, NaturalGenomeTypeID, NaturalGenomeDiedHourField, key), 2); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	for _, key := range []string{"died-by-spoilage", "died-by-reply", "died-by-request"} {
		rule, cause := NaturalSpoilageRule, world
		if key == "died-by-reply" {
			rule, cause = NaturalBirthReplyRule, kernel.Cause{Actor: 2}
		}
		if key == "died-by-request" {
			rule, cause = NaturalBirthRequestRule, kernel.Cause{Actor: 1}
		}
		if err := NaturalCheckOwnership(patch(rule, cause, 1, NaturalGenomeTypeID, NaturalGenomeDiedHourField, key), 2); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s survived the guard", key)
		}
	}
	// Loci, hours, and parent refs are never patchable post-creation.
	for _, field := range []sim.FieldID{NaturalGenomeLocusMField, NaturalGenomeLocusYField, NaturalGenomeLocusWField, NaturalGenomeParentAField, NaturalGenomeParentBField, NaturalGenomeBirthHourField, NaturalGenomeDeathHourField} {
		for _, rule := range []sim.RuleID{NaturalBasalRule, NaturalAgeDeathRule, NaturalBirthReplyRule, NaturalBirthRequestRule, NaturalSpoilageRule} {
			cause := world
			if rule == NaturalBirthRequestRule || rule == NaturalBirthReplyRule {
				cause = kernel.Cause{Actor: 2}
			}
			if err := NaturalCheckOwnership(patch(rule, cause, 1, NaturalGenomeTypeID, field, "forged"), 2); !errors.Is(err, ErrNaturalContract) {
				t.Fatalf("forged genome field %d under rule %d survived", field, rule)
			}
		}
	}
	// birth-granary-paid: rule 511 only, and the consenter's own row must
	// be part of the proposal (the cause is always one of the parents).
	replyPaid := kernel.Proposal{Key: "paid-by-reply", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: NaturalBirthReplyRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{
			{Entity: 1, Component: NaturalGenomeTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(2)},
			{Entity: 2, Component: NaturalGenomeTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGenomeBirthGranaryPaidField, Value: sim.IntegerValue(2)},
		}}
	if err := NaturalCheckOwnership(replyPaid, 2); err != nil {
		t.Fatalf("paid-by-reply: %v", err)
	}
	replyPaid.Patches = replyPaid.Patches[:1]
	if err := NaturalCheckOwnership(replyPaid, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("reply without the consenter's own row survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalSpoilageRule, world, 1, NaturalGenomeTypeID, NaturalGenomeBirthGranaryPaidField, "paid-by-spoilage"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("paid-by-spoilage survived")
	}
	// Rule/component mismatches: world rules need a world cause; actor
	// rules need their own rows; spoilage and grant are world rules on the
	// granary.
	if err := NaturalCheckOwnership(patch(NaturalRegenSplitRule, kernel.Cause{Actor: 1}, 1001, NaturalPatchTypeID, NaturalPatchYieldField, "actor-split"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("actor-caused regen-split survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalSpoilageRule, kernel.Cause{Actor: 1}, 1, NaturalGranaryTypeID, NaturalGranaryStockField, "actor-spoilage"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("actor-caused spoilage survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalGrantRule, world, 1, NaturalGranaryTypeID, NaturalGranarySpoilDebtField, "grant-debt"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("grant wrote the spoil-debt field")
	}
	if err := NaturalCheckOwnership(patch(NaturalGatherRule, kernel.Cause{Actor: 1}, 1, NaturalGranaryTypeID, NaturalGranaryStockField, "gather-granary"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("gather into granary survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalBirthRequestRule, kernel.Cause{Actor: 1}, 2, NaturalBirthRequestTypeID, NaturalBirthRequestStatusField, "cross-request"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("cross-owner request survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalBirthRefuseRule, kernel.Cause{Actor: 2}, 1, NaturalBirthRequestTypeID, NaturalBirthRequestStatusField, "refuse-no-cause"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("refusal without the cause's own row survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalBirthExpireRule, kernel.Cause{Actor: 2}, 1, NaturalBirthRequestTypeID, NaturalBirthRequestStatusField, "actor-expire"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("actor-caused expiry survived")
	}
	if err := NaturalCheckOwnership(patch(NaturalAgeDeathRule, kernel.Cause{Actor: 1}, 1, NaturalGenomeTypeID, NaturalGenomeDiedHourField, "actor-age-death"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("actor-caused age-death survived")
	}
	// Foreign components are unreachable under v5 rules.
	if err := NaturalCheckOwnership(patch(NaturalConsumeRule, kernel.Cause{Actor: 1}, 1, CapacityBodyTypeID, CapacityBodyEnergyField, "v3-body"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v3 component under v5 rule survived")
	}
	// Entity-space checks: an out-of-fixture patch or founder index is
	// rejected at the declared P.
	if err := NaturalCheckOwnership(patch(NaturalRegenSplitRule, world, 1003, NaturalPatchTypeID, NaturalPatchYieldField, "patch-3"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("patch 1003 survived at P=2")
	}
	if err := NaturalCheckOwnership(patch(NaturalAgeDeathRule, world, 17, NaturalGenomeTypeID, NaturalGenomeDiedHourField, "founder-17"), 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("founder 17 survived at P=2")
	}
	// Allocations exist only under rule 511 and only for the six per-actor
	// components with allocator-fresh newborn IDs.
	newbornState := NaturalActorState{
		Body: NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: NaturalNeverHour},
		Bag:  NaturalBagState{Source: 1001}, Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary: NaturalGranaryState{LastStoredMealHour: NaturalNeverHour}, Request: NaturalIdleRequestState(),
	}
	newbornState.Genome = NaturalGenomeState{LocusM: 6, LocusY: 6, LocusW: 6, ParentA: 1, ParentB: 2, DeathHour: 96, DiedHour: NaturalNeverHour}
	seed, err := naturalNewbornAllocationsFor(NaturalMechanismV5, 2, 2017, 1001, newbornState)
	if err != nil {
		t.Fatal(err)
	}
	notReply := kernel.Proposal{Key: "alloc-not-reply", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: NaturalSpoilageRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStockField, Value: sim.IntegerValue(0)}}, Allocations: []component.ComponentSeed{seed[0]}}
	if err := NaturalCheckOwnership(notReply, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("allocation under non-reply rule survived")
	}
	foreignAlloc := kernel.Proposal{Key: "alloc-foreign", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: NaturalBirthReplyRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStockField, Value: sim.IntegerValue(0)}}, Allocations: []component.ComponentSeed{{Entity: 2017, Component: CapacityBodyTypeID}}}
	if err := NaturalCheckOwnership(foreignAlloc, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("foreign allocation survived")
	}
	fixedAlloc := kernel.Proposal{Key: "alloc-fixed", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: NaturalBirthReplyRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalGranaryStockField, Value: sim.IntegerValue(0)}}, Allocations: []component.ComponentSeed{{Entity: 2016, Component: NaturalBodyTypeID}}}
	if err := NaturalCheckOwnership(fixedAlloc, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("fixed-block allocation survived")
	}
	// Wrong rule version is rejected outright.
	wrongVersion := patch(NaturalBasalRule, world, 1, NaturalBodyTypeID, NaturalBodyEnergyField, "wrong-version")
	wrongVersion.RuleVersion = 4
	if err := NaturalCheckOwnership(wrongVersion, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v4 rule version survived")
	}
	// A well-formed basal row passes.
	if err := NaturalCheckOwnership(patch(NaturalBasalRule, world, 1, NaturalBodyTypeID, NaturalBodyEnergyField, "basal-ok"), 2); err != nil {
		t.Fatalf("basal-ok: %v", err)
	}
}

// TestNaturalNoCapConstantsPresent pins the P15 removal: the v5 lane files
// contain no birth cap, no first-newborn ID, no granary cap, and no
// gather-claims cap, and the exposed ordinal arithmetic is unbounded.
func TestNaturalNoCapConstantsPresent(t *testing.T) {
	// Assembled by concatenation so this file does not itself contain the
	// forbidden literals it scans for.
	forbidden := []string{"Max" + "Births", "First" + "NewbornID", "Granary" + "Max", "Claims" + "Max", "Max" + "Candidates", "Birth" + "Cap"}
	for _, file := range []string{"natural_contract.go", "natural_contract_math.go", "natural_contract_test.go", "natural_contract_math_test.go"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range forbidden {
			if strings.Contains(string(content), name) {
				t.Fatalf("%s contains forbidden cap identifier %q", file, name)
			}
		}
	}
	// Behavioral: the ordinal space is unbounded.
	if position, err := NaturalBirthDrawPosition(1 << 40); err != nil || position != uint64(7*(1<<40)) {
		t.Fatalf("huge ordinal: %d %v", position, err)
	}
}

func TestNaturalIntentProposalsRejectOffPhaseInstants(t *testing.T) {
	patches := 2
	actor, err := NaturalFounderState(patches, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	held := actor
	held.Bag.Units = 1
	held.Body.LastGatherHour = 0
	slot := NaturalSlotState{Stock: 1}
	if _, err := NaturalGatherProposal("gather", patches, 0, 1, 2001, slot, actor); err != nil {
		t.Fatalf("gather at claim: %v", err)
	}
	if _, err := NaturalConsumeProposal("consume", patches, 0, 1, held); err != nil {
		t.Fatalf("consume at bag decision: %v", err)
	}
	var wildSlots [NaturalSlotsPerPatch]NaturalSlotState
	if _, _, _, err := NaturalBuild(patches, 0, 1, held, 1001, wildSlots); err != nil {
		t.Fatalf("build without completion: %v", err)
	}
	claim, _ := NaturalPhaseTime(0, NaturalPhaseClaim)
	paired, _ := NaturalPhaseTime(0, NaturalPhasePairedMeal)
	fedGranary := actor
	fedGranary.Granary = NaturalGranaryState{Stock: 1, CultivatedTotal: 1, LastStoredMealHour: NaturalNeverHour}
	if _, err := NaturalEatStoredProposal("eat-claim", patches, 0, claim, 1, fedGranary); err != nil {
		t.Fatalf("eat-stored at claim: %v", err)
	}
	if _, err := NaturalEatStoredProposal("eat-paired", patches, 0, paired, 1, fedGranary); err != nil {
		t.Fatalf("eat-stored at paired meal: %v", err)
	}
	offPhase, _ := NaturalPhaseTime(0, NaturalPhaseBirthRequest)
	if _, err := NaturalEatStoredProposal("eat-off", patches, 0, offPhase, 1, fedGranary); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("eat-stored off-phase admitted")
	}
	if _, err := NaturalBirthRequestProposal("request", patches, 0, 1, 2, 2, 3, actor); err != nil {
		t.Fatalf("request at birth-request phase: %v", err)
	}
	if _, err := NaturalGatherProposal("gather-off", patches, 168, 1, 2001, slot, actor); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("gather beyond the horizon admitted")
	}
	if _, _, _, err := NaturalBuild(patches, 168, 1, held, 1001, wildSlots); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("build beyond the horizon admitted")
	}
	// The build constructor carries the frozen completion instant.
	held2 := held
	held2.Worksite = NaturalWorksiteState{Capital: 1, Wip: 1, InvestedUnits: 3, PointsCreated: 1, LastBuildHour: NaturalNeverHour}
	buildProposal, err := NaturalBuildProposal("build/0/1", patches, 0, 1, held2, wildSlots)
	if err != nil {
		t.Fatalf("build proposal: %v", err)
	}
	start, _ := NaturalPhaseTime(0, NaturalPhaseBuildStart)
	if buildProposal.Time != NaturalBuildCompletion(start) {
		t.Fatal("build completion instant")
	}
	// A claim in the wrong patch is an invalid call with zero mutation.
	if _, _, _, err := NaturalBuild(patches, 0, 1, held2, 1002, wildSlots); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("cross-patch build claim admitted")
	}
}

// mustNaturalGenome builds a founder genome or fails the test.
func mustNaturalGenome(t *testing.T, patches int, actor sim.EntityID, lifetime int64) NaturalGenomeState {
	t.Helper()
	genome, err := NaturalFounderGenome(patches, actor, lifetime)
	if err != nil {
		t.Fatal(err)
	}
	return genome
}

func mustNaturalPhase(t *testing.T, hour, phase int) sim.SimTime {
	t.Helper()
	at, err := NaturalPhaseTime(hour, phase)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// naturalDrawsForSeed derives the seven birth draws exactly as the runner
// will: the frozen birth stream at position 7×(ordinal−1).
func naturalDrawsForSeed(t *testing.T, births int64) []uint64 {
	t.Helper()
	position, err := NaturalBirthDrawPosition(births)
	if err != nil {
		t.Fatal(err)
	}
	stream := sim.NewRandomStream(sim.RandomState{Seed: 7, Stream: NaturalBirthStream, Position: position})
	draws := make([]uint64, NaturalDrawsPerBirth)
	for i := range draws {
		draws[i] = stream.Uint64()
	}
	return draws
}

// naturalSnapshotActor reads one actor's six rows from a committed
// snapshot.
func naturalSnapshotActor(t *testing.T, reader component.Reader, auth component.Authority, version sim.WorldVersion, actor sim.EntityID) NaturalActorState {
	t.Helper()
	integer := func(typ sim.ComponentTypeID, field sim.FieldID) int64 {
		view, err := reader.Read(component.ReadRequest{Entity: actor, Component: typ, Fields: []sim.FieldID{field}, WorldVersion: version, Authority: auth})
		if err != nil {
			t.Fatalf("read %d/%d: %v", typ, field, err)
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
	ref := func(typ sim.ComponentTypeID, field sim.FieldID) sim.EntityID {
		view, err := reader.Read(component.ReadRequest{Entity: actor, Component: typ, Fields: []sim.FieldID{field}, WorldVersion: version, Authority: auth})
		if err != nil {
			t.Fatalf("read %d/%d: %v", typ, field, err)
		}
		value, err := view.Value(field)
		if err != nil {
			t.Fatal(err)
		}
		if !value.IsPresent() {
			return 0
		}
		id, err := value.EntityRef()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	return NaturalActorState{
		Body:     NaturalBodyState{Energy: integer(NaturalBodyTypeID, NaturalBodyEnergyField), Hunger: integer(NaturalBodyTypeID, NaturalBodyHungerField), BasalSpent: integer(NaturalBodyTypeID, NaturalBodyBasalSpentField), CapLost: integer(NaturalBodyTypeID, NaturalBodyCapLostField), Consumed: integer(NaturalBodyTypeID, NaturalBodyConsumedField), LastGatherHour: integer(NaturalBodyTypeID, NaturalBodyLastGatherHourField), BasalDebt: integer(NaturalBodyTypeID, NaturalBodyBasalDebtField)},
		Bag:      NaturalBagState{Units: integer(NaturalBagTypeID, NaturalBagUnitsField), Source: ref(NaturalBagTypeID, NaturalBagSourceField)},
		Worksite: NaturalWorksiteState{Capital: integer(NaturalWorksiteTypeID, NaturalWorksiteCapitalField), Wip: integer(NaturalWorksiteTypeID, NaturalWorksiteWipField), WearDebt: integer(NaturalWorksiteTypeID, NaturalWorksiteWearDebtField), InvestedUnits: integer(NaturalWorksiteTypeID, NaturalWorksiteInvestedUnitsField), PointsCreated: integer(NaturalWorksiteTypeID, NaturalWorksitePointsCreatedField), PointsDecayed: integer(NaturalWorksiteTypeID, NaturalWorksitePointsDecayedField), LastBuildHour: integer(NaturalWorksiteTypeID, NaturalWorksiteLastBuildHourField)},
		Granary:  NaturalGranaryState{Stock: integer(NaturalGranaryTypeID, NaturalGranaryStockField), CultivatedTotal: integer(NaturalGranaryTypeID, NaturalGranaryCultivatedTotalField), StoredMeals: integer(NaturalGranaryTypeID, NaturalGranaryStoredMealsField), LastStoredMealHour: integer(NaturalGranaryTypeID, NaturalGranaryLastStoredMealHourField), SpoilDebt: integer(NaturalGranaryTypeID, NaturalGranarySpoilDebtField), Spoiled: integer(NaturalGranaryTypeID, NaturalGranarySpoiledField)},
		Genome: NaturalGenomeState{LocusM: integer(NaturalGenomeTypeID, NaturalGenomeLocusMField), LocusY: integer(NaturalGenomeTypeID, NaturalGenomeLocusYField), LocusW: integer(NaturalGenomeTypeID, NaturalGenomeLocusWField),
			ParentA: ref(NaturalGenomeTypeID, NaturalGenomeParentAField), ParentB: ref(NaturalGenomeTypeID, NaturalGenomeParentBField),
			BirthHour: integer(NaturalGenomeTypeID, NaturalGenomeBirthHourField), DeathHour: integer(NaturalGenomeTypeID, NaturalGenomeDeathHourField), DiedHour: integer(NaturalGenomeTypeID, NaturalGenomeDiedHourField), BirthGranaryPaid: integer(NaturalGenomeTypeID, NaturalGenomeBirthGranaryPaidField)},
		Request: NaturalBirthRequestState{Hour: integer(NaturalBirthRequestTypeID, NaturalBirthRequestHourField), Addressee: ref(NaturalBirthRequestTypeID, NaturalBirthRequestAddresseeField), ReportedCapital: integer(NaturalBirthRequestTypeID, NaturalBirthRequestReportedCapitalField), ReportedGranary: integer(NaturalBirthRequestTypeID, NaturalBirthRequestReportedGranaryField), Status: NaturalRequestStatus(integer(NaturalBirthRequestTypeID, NaturalBirthRequestStatusField)), LastRequestHour: integer(NaturalBirthRequestTypeID, NaturalBirthRequestLastRequestHourField), LastReplyHour: integer(NaturalBirthRequestTypeID, NaturalBirthRequestLastReplyHourField)},
	}
}

// naturalSnapshotSlot reads one slot row from a committed snapshot.
func naturalSnapshotSlot(t *testing.T, reader component.Reader, auth component.Authority, version sim.WorldVersion, slot sim.EntityID) NaturalSlotState {
	t.Helper()
	integer := func(field sim.FieldID) int64 {
		view, err := reader.Read(component.ReadRequest{Entity: slot, Component: NaturalSlotTypeID, Fields: []sim.FieldID{field}, WorldVersion: version, Authority: auth})
		if err != nil {
			t.Fatalf("read slot %d: %v", field, err)
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
	owner := sim.EntityID(0)
	view, err := reader.Read(component.ReadRequest{Entity: slot, Component: NaturalSlotTypeID, Fields: []sim.FieldID{NaturalSlotTenureOwnerField}, WorldVersion: version, Authority: auth})
	if err != nil {
		t.Fatal(err)
	}
	if value, err := view.Value(NaturalSlotTenureOwnerField); err != nil {
		t.Fatal(err)
	} else if value.IsPresent() {
		if owner, err = value.EntityRef(); err != nil {
			t.Fatal(err)
		}
	}
	return NaturalSlotState{Stock: integer(NaturalSlotStockField), Gathered: integer(NaturalSlotGatheredField), Mode: NaturalTenureMode(integer(NaturalSlotTenureModeField)), Owner: owner, Cultivated: integer(NaturalSlotTenureCultivatedField)}
}

// naturalBuildFixture builds the scripted build-completion world: founder 1
// holds one completed point (slot 0), one unit of work in progress, and the
// bag's last gather; slot 1 is wild with standing stock; every other slot
// is wild and empty. The ledgers satisfy G1 intake (gathered 4 = held 1 +
// invested 3), the grant seam (granted 5 = the granary's received 5),
// G2a, G2b, and G3.
func naturalBuildFixture(t *testing.T, patches int) ([]component.ComponentSeed, NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState) {
	t.Helper()
	seeds := naturalNeutralSeeds(patches, 8, 140)
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
		for i := range seeds {
			if seeds[i].Entity != entity || seeds[i].Component != componentType {
				continue
			}
			for j := range seeds[i].Fields {
				if v, ok := values[seeds[i].Fields[j].Field]; ok {
					seeds[i].Fields[j].Value = sim.IntegerValue(v)
				}
			}
		}
	}
	// Patch 1001: three pulses; produced 5 = gathered 4 + standing 1;
	// granted 5 = the granary's received ledger.
	rewrite(1001, NaturalPatchTypeID, map[sim.FieldID]int64{NaturalPatchPulsesField: 3, NaturalPatchProducedField: 5, NaturalPatchGrantedField: 5, NaturalPatchUnrealizedField: 14})
	// Slot 0: tenured to founder 1 with its pre-tenure gather history.
	ownerRef, _ := sim.EntityRefValue(1)
	rewrite(2001, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotGatheredField: 3, NaturalSlotTenureModeField: 1, NaturalSlotTenureCultivatedField: 1})
	for i := range seeds {
		if seeds[i].Entity == 2001 && seeds[i].Component == NaturalSlotTypeID {
			for j := range seeds[i].Fields {
				if seeds[i].Fields[j].Field == NaturalSlotTenureOwnerField {
					seeds[i].Fields[j].Value = ownerRef
				}
			}
		}
	}
	// Slot 1: wild with standing stock, gathered once.
	rewrite(2002, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotStockField: 1, NaturalSlotGatheredField: 1})
	// Founder 1: bag unit in hand, one standing point, one in progress.
	rewrite(1, NaturalBagTypeID, map[sim.FieldID]int64{NaturalBagUnitsField: 1})
	rewrite(1, NaturalBodyTypeID, map[sim.FieldID]int64{NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6, NaturalBodyLastGatherHourField: 2})
	rewrite(1, NaturalWorksiteTypeID, map[sim.FieldID]int64{NaturalWorksiteCapitalField: 1, NaturalWorksiteWipField: 1, NaturalWorksiteInvestedUnitsField: 3, NaturalWorksitePointsCreatedField: 1})
	rewrite(1, NaturalGranaryTypeID, map[sim.FieldID]int64{NaturalGranaryStockField: 5, NaturalGranaryCultivatedTotalField: 5})
	state := NaturalActorState{
		Body:     NaturalBodyState{Energy: 5, BasalSpent: 6, LastGatherHour: 2},
		Bag:      NaturalBagState{Units: 1, Source: 1001},
		Worksite: NaturalWorksiteState{Capital: 1, Wip: 1, InvestedUnits: 3, PointsCreated: 1, LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{Stock: 5, CultivatedTotal: 5, LastStoredMealHour: NaturalNeverHour},
		Genome:   mustNaturalGenome(t, patches, 1, 140),
		Request:  NaturalIdleRequestState(),
	}
	var slots [NaturalSlotsPerPatch]NaturalSlotState
	slots[0] = NaturalSlotState{Gathered: 3, Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
	slots[1] = NaturalSlotState{Stock: 1, Gathered: 1}
	if !validNaturalActor(patches, 1, state) {
		t.Fatalf("build fixture invalid: %+v", state)
	}
	return seeds, state, slots
}

// naturalSeedGateParents rewrites founders 1 and 2 into gate-passing
// parents on patch 1001 with capital 3 (tenured slots 0/2/3 for founder 1,
// 4/5/6 for founder 2) and granary 5, keeping every frozen gate exact:
// gathered 12 = invested 12; pulses 11; produced 13 = gathered 12 +
// standing 1; granted 10 = the two granaries' received ledgers;
// unrealized 65; R·pulses = 88 = 13+10+65.
func naturalSeedGateParents(t *testing.T, patches int, seeds []component.ComponentSeed) []component.ComponentSeed {
	t.Helper()
	out := append([]component.ComponentSeed(nil), seeds...)
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
		for i := range out {
			if out[i].Entity != entity || out[i].Component != componentType {
				continue
			}
			for j := range out[i].Fields {
				if v, ok := values[out[i].Fields[j].Field]; ok {
					out[i].Fields[j].Value = sim.IntegerValue(v)
				}
			}
		}
	}
	setTenure := func(slotIndex int, owner sim.EntityID, gathered int64) {
		slotID, err := NaturalSlotID(patches, 0, slotIndex)
		if err != nil {
			t.Fatal(err)
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotGatheredField: gathered})
		if owner == 0 {
			return
		}
		ownerRef, err := sim.EntityRefValue(owner)
		if err != nil {
			t.Fatal(err)
		}
		for i := range out {
			if out[i].Entity != slotID || out[i].Component != NaturalSlotTypeID {
				continue
			}
			for j := range out[i].Fields {
				switch out[i].Fields[j].Field {
				case NaturalSlotTenureOwnerField:
					out[i].Fields[j].Value = ownerRef
				case NaturalSlotTenureModeField:
					out[i].Fields[j].Value = sim.IntegerValue(1)
				case NaturalSlotTenureCultivatedField:
					out[i].Fields[j].Value = sim.IntegerValue(1)
				}
			}
		}
	}
	rewrite(1001, NaturalPatchTypeID, map[sim.FieldID]int64{NaturalPatchPulsesField: 11, NaturalPatchProducedField: 13, NaturalPatchGrantedField: 10, NaturalPatchUnrealizedField: 65})
	setTenure(0, 1, 11)
	setTenure(1, 0, 1)
	rewrite(2002, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotStockField: 1, NaturalSlotGatheredField: 1})
	setTenure(2, 1, 0)
	setTenure(3, 1, 0)
	setTenure(4, 2, 0)
	setTenure(5, 2, 0)
	setTenure(6, 2, 0)
	for _, actor := range []sim.EntityID{1, 2} {
		rewrite(actor, NaturalBagTypeID, map[sim.FieldID]int64{NaturalBagUnitsField: 0})
		rewrite(actor, NaturalBodyTypeID, map[sim.FieldID]int64{NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6, NaturalBodyLastGatherHourField: NaturalNeverHour, NaturalBodyConsumedField: 0})
		rewrite(actor, NaturalWorksiteTypeID, map[sim.FieldID]int64{NaturalWorksiteCapitalField: 3, NaturalWorksiteWipField: 0, NaturalWorksiteInvestedUnitsField: 6, NaturalWorksitePointsCreatedField: 3, NaturalWorksiteLastBuildHourField: NaturalNeverHour})
		rewrite(actor, NaturalGranaryTypeID, map[sim.FieldID]int64{NaturalGranaryStockField: 5, NaturalGranaryCultivatedTotalField: 5})
	}
	return out
}

// naturalGateParent builds the same gate-passing parent in contract state.
func naturalGateParent(t *testing.T, patches int, actor sim.EntityID) NaturalActorState {
	t.Helper()
	a, err := NaturalFounderState(patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	a.Worksite = NaturalWorksiteState{Capital: 3, InvestedUnits: 6, PointsCreated: 3, LastBuildHour: NaturalNeverHour}
	a.Granary = NaturalGranaryState{Stock: 5, CultivatedTotal: 5, LastStoredMealHour: NaturalNeverHour}
	a.Body = NaturalBodyState{Energy: 5, BasalSpent: 6, LastGatherHour: NaturalNeverHour}
	if !validNaturalActor(patches, actor, a) {
		t.Fatalf("gate parent fixture invalid: %+v", a)
	}
	return a
}

// naturalGateFixtureWorld assembles the whole gate-parent world in contract
// state (patches, slots, founders) so conservation can be checked before
// and after a scripted birth.
func naturalGateFixtureWorld(t *testing.T, patches int) ([]NaturalPatchState, [][]NaturalSlotState, []NaturalActorState) {
	t.Helper()
	patchStates := []NaturalPatchState{
		{Yield: 8, Pulses: 11, Produced: 13, Granted: 10, Unrealized: 65},
		{},
	}
	slotStates := make([][]NaturalSlotState, 2)
	slotStates[0] = []NaturalSlotState{
		{Gathered: 11, Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1},
		{Stock: 1, Gathered: 1},
		{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1},
		{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1},
		{Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1},
		{Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1},
		{Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1},
		{},
	}
	slotStates[1] = make([]NaturalSlotState, NaturalSlotsPerPatch)
	founders := make([]NaturalActorState, 16)
	for i := range founders {
		id := sim.EntityID(i + 1)
		if id == 1 || id == 2 {
			founders[i] = naturalGateParent(t, patches, id)
			continue
		}
		state, err := NaturalFounderState(patches, id, 140)
		if err != nil {
			t.Fatal(err)
		}
		founders[i] = state
	}
	return patchStates, slotStates, founders
}

// TestNaturalKernelAdmissionBuildClaimBirthEntityCreateAndNegativeFixtures
// drives the two entity-mutating v5 flows through the real kernel: the
// build completion with its lowest-index slot claim, and the birth reply
// with its allocator-fresh entity-create, plus the typed negative fixtures
// (forged tenure write, ID collision, partial rows, allocation/patch
// overlap). The scripted worlds are cross-checked by
// NaturalCheckConservation before and after each mutation.
func TestNaturalKernelAdmissionBuildClaimBirthEntityCreateAndNegativeFixtures(t *testing.T) {
	patches := 2
	reg, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	seeds, state, slots := naturalBuildFixture(t, patches)
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ := k.Snapshot()
	buildProposal, err := NaturalBuildProposal("build/5/1", patches, 5, 1, state, slots)
	if err != nil {
		t.Fatalf("build proposal: %v", err)
	}
	start := mustNaturalPhase(t, 5, NaturalPhaseBuildStart)
	if buildProposal.Time != NaturalBuildCompletion(start) {
		t.Fatal("build completion instant")
	}
	plan, err := k.Plan(buildProposal, authority)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if _, err := k.CommitBatch([]kernel.Plan{plan}); err != nil {
		t.Fatalf("build commit: %v", err)
	}
	reader, auth, version := k.Snapshot()
	// The claim landed on the LOWEST-INDEX free wild slot with stock 0:
	// slot index 2 (slot 0 is tenured, slot 1 is wild with stock 1).
	claimed := naturalSnapshotSlot(t, reader, auth, version, 2003)
	if claimed.Mode != NaturalTenureCultivated || claimed.Owner != 1 || claimed.Cultivated != 1 || claimed.Stock != 0 {
		t.Fatalf("claimed slot: %+v", claimed)
	}
	stocked := naturalSnapshotSlot(t, reader, auth, version, 2002)
	if stocked.Mode != NaturalTenureWild || stocked.Stock != 1 {
		t.Fatalf("stocked slot disturbed: %+v", stocked)
	}
	builder := naturalSnapshotActor(t, reader, auth, version, 1)
	if builder.Worksite.Capital != 2 || builder.Worksite.Wip != 0 || builder.Worksite.InvestedUnits != 4 || builder.Bag.Units != 0 {
		t.Fatalf("builder after completion: %+v", builder)
	}
	if !validNaturalActor(patches, 1, builder) {
		t.Fatalf("builder invalid: %+v", builder)
	}
	// The post-build world satisfies the land identity: tenured slots
	// 0 and 2 equal the builder's standing capital 2.
	postSlots := slots
	postSlots[2] = NaturalSlotState{Mode: NaturalTenureCultivated, Owner: 1, Cultivated: 1}
	postState := state
	postState.Worksite = builder.Worksite
	postState.Bag = builder.Bag
	postState.Body = builder.Body
	patchStates := []NaturalPatchState{{Yield: 8, Pulses: 3, Produced: 5, Granted: 5, Unrealized: 14}, {}}
	founders := make([]NaturalActorState, 16)
	for i := range founders {
		id := sim.EntityID(i + 1)
		if id == 1 {
			founders[i] = postState
			continue
		}
		founders[i], err = NaturalFounderState(patches, id, 140)
		if err != nil {
			t.Fatal(err)
		}
	}
	worldSlots := [][]NaturalSlotState{postSlots[:], make([]NaturalSlotState, NaturalSlotsPerPatch)}
	if _, err := NaturalCheckConservation(patches, patchStates, worldSlots, founders, nil, 2016); err != nil {
		t.Fatalf("post-build world not conserved: %v", err)
	}

	// Birth flow: request at h+40m+5µs, reply at h+40m+6µs with the
	// allocator-fresh newborn ID, over the gate-parent fixture world.
	parentSeeds := naturalSeedGateParents(t, patches, seeds)
	patchStates, worldSlots, founders = naturalGateFixtureWorld(t, patches)
	if _, err := NaturalCheckConservation(patches, patchStates, worldSlots, founders, nil, 2016); err != nil {
		t.Fatalf("gate fixture world not conserved: %v", err)
	}
	k2, err := kernel.New(reg, 0, parentSeeds)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ = k2.Snapshot()
	proposer := naturalGateParent(t, patches, 1)
	consenter := naturalGateParent(t, patches, 2)
	request, err := NaturalBirthRequest(patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	requestProposal, err := NaturalBirthRequestProposal("request/5/1", patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	plan, err = k2.Plan(requestProposal, authority)
	if err != nil {
		t.Fatalf("request plan: %v", err)
	}
	if _, err := k2.CommitBatch([]kernel.Plan{plan}); err != nil {
		t.Fatalf("request commit: %v", err)
	}
	_, authority, _ = k2.Snapshot()
	alloc, err := NaturalGenesisAllocator(patches)
	if err != nil {
		t.Fatal(err)
	}
	newbornID, err := NaturalNextNewbornID(&alloc)
	if err != nil || newbornID != 2017 {
		t.Fatalf("newborn ID: %d %v", newbornID, err)
	}
	draws := naturalDrawsForSeed(t, 0)
	result, err := NaturalBirthReply(patches, 5, true, draws, newbornID, 1, 2, request, consenter)
	if err != nil || result.Decision != NaturalReplyAccept {
		t.Fatalf("reply: %+v %v", result, err)
	}
	replyProposal, err := NaturalBirthReplyProposal("reply/5/2", patches, 5, 1, 2, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(replyProposal.Allocations) != 6 {
		t.Fatalf("newborn rows: %d", len(replyProposal.Allocations))
	}
	plan, err = k2.Plan(replyProposal, authority)
	if err != nil {
		t.Fatalf("birth plan: %v", err)
	}
	events, err := k2.CommitBatch([]kernel.Plan{plan})
	if err != nil || len(events) != 1 {
		t.Fatalf("birth commit: %v", err)
	}
	reader, auth, version = k2.Snapshot()
	born := naturalSnapshotActor(t, reader, auth, version, newbornID)
	if born.Body.Energy != NaturalNewbornEnergy || born.Body.Hunger != 0 || born.Genome.ParentA != 1 || born.Genome.ParentB != 2 || born.Bag.Source != 1001 {
		t.Fatalf("newborn rows: %+v", born)
	}
	if !validNaturalActor(patches, newbornID, born) {
		t.Fatalf("newborn invalid: %+v", born)
	}
	parentAfter := naturalSnapshotActor(t, reader, auth, version, 1)
	if parentAfter.Granary.Stock != 5-NaturalParentGranaryCost || parentAfter.Genome.BirthGranaryPaid != NaturalParentGranaryCost ||
		parentAfter.Request.Status != NaturalRequestAccepted {
		t.Fatalf("proposer after birth: %+v", parentAfter)
	}
	consenterAfter := naturalSnapshotActor(t, reader, auth, version, 2)
	if consenterAfter.Request.LastReplyHour != 5 || consenterAfter.Genome.BirthGranaryPaid != NaturalParentGranaryCost {
		t.Fatalf("consenter after birth: %+v", consenterAfter)
	}
	// The post-birth world satisfies G3 extended by exactly 8 and the
	// allocator cross-check (last ID = the newborn).
	balance, err := NaturalCheckConservation(patches, patchStates, worldSlots, []NaturalActorState{result.Proposer, result.Consenter, founders[2], founders[3], founders[4], founders[5], founders[6], founders[7], founders[8], founders[9], founders[10], founders[11], founders[12], founders[13], founders[14], founders[15]}, []NaturalNewbornState{{ID: newbornID, State: result.Newborn}}, newbornID)
	if err != nil {
		t.Fatalf("post-birth world not conserved: %v", err)
	}
	if balance.Births != 1 || balance.InitialEnergy+NaturalNewbornEnergy != balance.Energy+balance.BasalSpent+balance.EnergyCapLost {
		t.Fatalf("post-birth balance: %+v", balance)
	}

	// Negative fixture: ID collision — allocating an existing ID is
	// rejected with zero mutation.
	collide := replyProposal
	collide.Key = "id-collision"
	for i := range collide.Allocations {
		collide.Allocations[i].Entity = 5
	}
	if _, err := k2.Plan(collide, auth); err == nil {
		t.Fatal("ID collision admitted")
	}
	// Negative fixture: partial allocation rows.
	partial := replyProposal
	partial.Key = "partial-rows"
	partial.Allocations = []component.ComponentSeed{{Entity: 2018, Component: NaturalBodyTypeID, Fields: partial.Allocations[1].Fields[:3]}}
	if _, err := k2.Plan(partial, auth); err == nil {
		t.Fatal("partial allocation admitted")
	}
	// Negative fixture: an allocation may not overlap this batch's
	// patches.
	overlap := replyProposal
	overlap.Key = "overlap"
	overlap.Patches = append(overlap.Patches, component.Patch{Entity: newbornID, Component: NaturalBodyTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBodyEnergyField, Value: sim.IntegerValue(9)})
	if _, err := k2.Plan(overlap, auth); err == nil {
		t.Fatal("allocation/patch overlap admitted")
	}
	// Negative fixture: a forged tenure write under the gather rule is
	// schema-valid for the kernel; the ownership guard is the enforcement
	// point (v3/v4 discipline).
	ownerRef, _ := sim.EntityRefValue(1)
	forged := kernel.Proposal{Key: "forged-tenure", Time: NaturalBuildCompletion(start), Cause: kernel.Cause{Actor: 2}, Rule: NaturalGatherRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{
			{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotTenureOwnerField, Value: ownerRef},
			{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotTenureModeField, Value: sim.IntegerValue(1)},
			{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalSlotTenureCultivatedField, Value: sim.IntegerValue(1)},
			{Entity: 2, Component: NaturalBagTypeID, SchemaVersion: NaturalSchemaVersion, Field: NaturalBagUnitsField, Value: sim.IntegerValue(1)},
		}}
	if _, err := k2.Plan(forged, auth); err != nil {
		t.Fatalf("expected raw kernel schema admission: %v", err)
	}
	if err := NaturalCheckOwnership(forged, patches); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("forged tenure write survived the guard: %v", err)
	}
}

// The fixed-block ID space is disjoint for every valid P; larger P is refused
// with a typed error instead of silently overlapping blocks (P15: an explicit
// fixture precondition, never a hidden ID-range bound).
func TestNaturalPatchCountPreconditionKeepsIDBlocksDisjoint(t *testing.T) {
	if NaturalMaxPatchesFixedIDSpace != 125 {
		t.Fatalf("derived max patches changed: %d", NaturalMaxPatchesFixedIDSpace)
	}
	if !NaturalPatchCountValid(NaturalMaxPatchesFixedIDSpace) || NaturalPatchCountValid(NaturalMaxPatchesFixedIDSpace+1) {
		t.Fatal("patch-count precondition is not exactly the derived bound")
	}
	for _, patches := range []int{1, 2, 32, 125} {
		founders, err := NaturalFounderCount(patches)
		if err != nil {
			t.Fatalf("P=%d: %v", patches, err)
		}
		lastPatch, err := NaturalPatchID(patches, patches-1)
		if err != nil {
			t.Fatal(err)
		}
		if sim.EntityID(founders) >= lastPatch {
			t.Fatalf("P=%d: founder block reaches the patch block (%d >= %d)", patches, founders, lastPatch)
		}
		highest, err := NaturalHighestFixedEntityID(patches)
		if err != nil {
			t.Fatal(err)
		}
		firstNewborn := highest + 1
		lastSlot, err := NaturalSlotID(patches, patches-1, NaturalSlotsPerPatch-1)
		if err != nil {
			t.Fatal(err)
		}
		if firstNewborn <= lastSlot {
			t.Fatalf("P=%d: newborn block overlaps slots (%d <= %d)", patches, firstNewborn, lastSlot)
		}
	}
	if _, err := NaturalFounderCount(NaturalMaxPatchesFixedIDSpace + 1); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("P above the derived bound admitted")
	}
}
