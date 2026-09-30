package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

func reproductionNeutralSeeds() []component.ComponentSeed {
	var seeds []component.ComponentSeed
	patchRef, _ := sim.EntityRefValue(1001)
	patch2Ref, _ := sim.EntityRefValue(1002)
	for p := 0; p < ReproductionPatchCount; p++ {
		patchID, _ := ReproductionPatchID(p)
		ref := patchRef
		if p == 1 {
			ref = patch2Ref
		}
		seeds = append(seeds, component.ComponentSeed{Entity: patchID, Component: ReproductionPatchTypeID, Fields: []component.FieldSeed{
			{Field: ReproductionPatchYieldField, Value: sim.IntegerValue(3)}, {Field: ReproductionPatchPulsesField, Value: sim.IntegerValue(0)},
			{Field: ReproductionPatchProducedField, Value: sim.IntegerValue(0)}, {Field: ReproductionPatchUnrealizedField, Value: sim.IntegerValue(0)}}})
		for s := 0; s < ReproductionSlotsPerPatch; s++ {
			slotID, _ := ReproductionSlotID(p, s)
			seeds = append(seeds, component.ComponentSeed{Entity: slotID, Component: ReproductionSlotTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionSlotStockField, Value: sim.IntegerValue(0)}, {Field: ReproductionSlotPatchField, Value: ref},
				{Field: ReproductionSlotGatheredField, Value: sim.IntegerValue(0)}}})
		}
	}
	for actor := sim.EntityID(1); actor <= ReproductionFounderCount; actor++ {
		home, _ := ReproductionFounderPatchID(actor)
		homeRef, _ := sim.EntityRefValue(home)
		genome, _ := ReproductionFounderGenome(actor, 120)
		parentA, _ := genomeRefValue(genome.ParentA)
		parentB, _ := genomeRefValue(genome.ParentB)
		idle := ReproductionIdleRequestState()
		addressee, _ := genomeRefValue(idle.Addressee)
		seeds = append(seeds,
			component.ComponentSeed{Entity: actor, Component: ReproductionBagTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionBagUnitsField, Value: sim.IntegerValue(0)}, {Field: ReproductionBagSourceField, Value: homeRef}}},
			component.ComponentSeed{Entity: actor, Component: ReproductionBodyTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(ReproductionFounderInitialEnergy)}, {Field: ReproductionBodyHungerField, Value: sim.IntegerValue(0)},
				{Field: ReproductionBodyBasalSpentField, Value: sim.IntegerValue(0)}, {Field: ReproductionBodyCapLostField, Value: sim.IntegerValue(0)},
				{Field: ReproductionBodyConsumedField, Value: sim.IntegerValue(0)}, {Field: ReproductionBodyLastGatherHourField, Value: sim.IntegerValue(ReproductionNeverHour)},
				{Field: ReproductionBodyBasalDebtField, Value: sim.IntegerValue(0)}}},
			component.ComponentSeed{Entity: actor, Component: ReproductionWorksiteTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionWorksiteCapitalField, Value: sim.IntegerValue(0)}, {Field: ReproductionWorksiteWipField, Value: sim.IntegerValue(0)},
				{Field: ReproductionWorksiteWearDebtField, Value: sim.IntegerValue(0)}, {Field: ReproductionWorksiteInvestedUnitsField, Value: sim.IntegerValue(0)},
				{Field: ReproductionWorksitePointsCreatedField, Value: sim.IntegerValue(0)}, {Field: ReproductionWorksitePointsDecayedField, Value: sim.IntegerValue(0)},
				{Field: ReproductionWorksiteLastBuildHourField, Value: sim.IntegerValue(ReproductionNeverHour)}}},
			component.ComponentSeed{Entity: actor, Component: ReproductionGranaryTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionGranaryStockField, Value: sim.IntegerValue(0)}, {Field: ReproductionGranaryYieldTotalField, Value: sim.IntegerValue(0)},
				{Field: ReproductionGranaryYieldUnrealizedField, Value: sim.IntegerValue(0)}, {Field: ReproductionGranaryStoredMealsField, Value: sim.IntegerValue(0)},
				{Field: ReproductionGranaryLastStoredMealHourField, Value: sim.IntegerValue(ReproductionNeverHour)}, {Field: ReproductionGranaryYieldDebtField, Value: sim.IntegerValue(0)}}},
			component.ComponentSeed{Entity: actor, Component: ReproductionGenomeTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionGenomeLocusMField, Value: sim.IntegerValue(genome.LocusM)}, {Field: ReproductionGenomeLocusYField, Value: sim.IntegerValue(genome.LocusY)},
				{Field: ReproductionGenomeLocusWField, Value: sim.IntegerValue(genome.LocusW)}, {Field: ReproductionGenomeParentAField, Value: parentA},
				{Field: ReproductionGenomeParentBField, Value: parentB}, {Field: ReproductionGenomeBirthHourField, Value: sim.IntegerValue(genome.BirthHour)},
				{Field: ReproductionGenomeDeathHourField, Value: sim.IntegerValue(genome.DeathHour)}, {Field: ReproductionGenomeDiedHourField, Value: sim.IntegerValue(genome.DiedHour)},
				{Field: ReproductionGenomeBirthGranaryPaidField, Value: sim.IntegerValue(genome.BirthGranaryPaid)}}},
			component.ComponentSeed{Entity: actor, Component: ReproductionBirthRequestTypeID, Fields: []component.FieldSeed{
				{Field: ReproductionBirthRequestHourField, Value: sim.IntegerValue(idle.Hour)}, {Field: ReproductionBirthRequestAddresseeField, Value: addressee},
				{Field: ReproductionBirthRequestReportedCapitalField, Value: sim.IntegerValue(idle.ReportedCapital)}, {Field: ReproductionBirthRequestReportedGranaryField, Value: sim.IntegerValue(idle.ReportedGranary)},
				{Field: ReproductionBirthRequestStatusField, Value: sim.IntegerValue(int64(idle.Status))}, {Field: ReproductionBirthRequestLastRequestHourField, Value: sim.IntegerValue(idle.LastRequestHour)},
				{Field: ReproductionBirthRequestLastReplyHourField, Value: sim.IntegerValue(idle.LastReplyHour)}}})
	}
	return seeds
}

func reproductionSnapshotActor(t *testing.T, reader component.Reader, auth component.Authority, version sim.WorldVersion, actor sim.EntityID) ReproductionActorState {
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
	return ReproductionActorState{
		Body:     ReproductionBodyState{Energy: integer(ReproductionBodyTypeID, ReproductionBodyEnergyField), Hunger: integer(ReproductionBodyTypeID, ReproductionBodyHungerField), BasalSpent: integer(ReproductionBodyTypeID, ReproductionBodyBasalSpentField), CapLost: integer(ReproductionBodyTypeID, ReproductionBodyCapLostField), Consumed: integer(ReproductionBodyTypeID, ReproductionBodyConsumedField), LastGatherHour: integer(ReproductionBodyTypeID, ReproductionBodyLastGatherHourField), BasalDebt: integer(ReproductionBodyTypeID, ReproductionBodyBasalDebtField)},
		Bag:      ReproductionBagState{Units: integer(ReproductionBagTypeID, ReproductionBagUnitsField), Source: ref(ReproductionBagTypeID, ReproductionBagSourceField)},
		Worksite: ReproductionWorksiteState{Capital: integer(ReproductionWorksiteTypeID, ReproductionWorksiteCapitalField), Wip: integer(ReproductionWorksiteTypeID, ReproductionWorksiteWipField), WearDebt: integer(ReproductionWorksiteTypeID, ReproductionWorksiteWearDebtField), InvestedUnits: integer(ReproductionWorksiteTypeID, ReproductionWorksiteInvestedUnitsField), PointsCreated: integer(ReproductionWorksiteTypeID, ReproductionWorksitePointsCreatedField), PointsDecayed: integer(ReproductionWorksiteTypeID, ReproductionWorksitePointsDecayedField), LastBuildHour: integer(ReproductionWorksiteTypeID, ReproductionWorksiteLastBuildHourField)},
		Granary:  ReproductionGranaryState{Stock: integer(ReproductionGranaryTypeID, ReproductionGranaryStockField), YieldTotal: integer(ReproductionGranaryTypeID, ReproductionGranaryYieldTotalField), YieldUnrealized: integer(ReproductionGranaryTypeID, ReproductionGranaryYieldUnrealizedField), StoredMeals: integer(ReproductionGranaryTypeID, ReproductionGranaryStoredMealsField), LastStoredMealHour: integer(ReproductionGranaryTypeID, ReproductionGranaryLastStoredMealHourField), YieldDebt: integer(ReproductionGranaryTypeID, ReproductionGranaryYieldDebtField)},
		Genome: ReproductionGenomeState{LocusM: integer(ReproductionGenomeTypeID, ReproductionGenomeLocusMField), LocusY: integer(ReproductionGenomeTypeID, ReproductionGenomeLocusYField), LocusW: integer(ReproductionGenomeTypeID, ReproductionGenomeLocusWField),
			ParentA: ref(ReproductionGenomeTypeID, ReproductionGenomeParentAField), ParentB: ref(ReproductionGenomeTypeID, ReproductionGenomeParentBField),
			BirthHour: integer(ReproductionGenomeTypeID, ReproductionGenomeBirthHourField), DeathHour: integer(ReproductionGenomeTypeID, ReproductionGenomeDeathHourField), DiedHour: integer(ReproductionGenomeTypeID, ReproductionGenomeDiedHourField), BirthGranaryPaid: integer(ReproductionGenomeTypeID, ReproductionGenomeBirthGranaryPaidField)},
		Request: ReproductionBirthRequestState{Hour: integer(ReproductionBirthRequestTypeID, ReproductionBirthRequestHourField), Addressee: ref(ReproductionBirthRequestTypeID, ReproductionBirthRequestAddresseeField), ReportedCapital: integer(ReproductionBirthRequestTypeID, ReproductionBirthRequestReportedCapitalField), ReportedGranary: integer(ReproductionBirthRequestTypeID, ReproductionBirthRequestReportedGranaryField), Status: ReproductionRequestStatus(integer(ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField)), LastRequestHour: integer(ReproductionBirthRequestTypeID, ReproductionBirthRequestLastRequestHourField), LastReplyHour: integer(ReproductionBirthRequestTypeID, ReproductionBirthRequestLastReplyHourField)},
	}
}

func TestReproductionConstantsClockRangesAndDistinctIDs(t *testing.T) {
	if ReproductionFormatVersion != 4 || ReproductionSchemaVersion != 4 || ReproductionRuleVersion != 4 || ReproductionProjectionVersion != 4 {
		t.Fatal("v4 versions")
	}
	typeIDs := []sim.ComponentTypeID{ReproductionPatchTypeID, ReproductionSlotTypeID, ReproductionBagTypeID, ReproductionBodyTypeID, ReproductionWorksiteTypeID, ReproductionGranaryTypeID, ReproductionGenomeTypeID, ReproductionBirthRequestTypeID}
	seen := map[sim.ComponentTypeID]bool{}
	for i, id := range typeIDs {
		if id != 0x80000040+sim.ComponentTypeID(i) || seen[id] {
			t.Fatalf("type ID %d at %d", id, i)
		}
		seen[id] = true
	}
	if sim.ComponentTypeID(ReproductionGenomeTypeID) != 0x80000046 {
		t.Fatal("frozen genome type ID")
	}
	for id := sim.RuleID(401); id <= 413; id++ {
		if !reproductionRulesDeclared[id] {
			t.Fatalf("rule %d not declared on any descriptor", id)
		}
	}
	if reproductionRulesDeclared[400] || reproductionRulesDeclared[414] {
		t.Fatal("rule range 401-413")
	}
	// Entity ranges: founders 1-16, newborns 10001-10016, patches 1001-2,
	// slots 2001-2016.
	if patch, err := ReproductionPatchID(0); err != nil || patch != 1001 {
		t.Fatalf("patch 0: %d %v", patch, err)
	}
	if patch, err := ReproductionPatchID(1); err != nil || patch != 1002 {
		t.Fatalf("patch 1: %d %v", patch, err)
	}
	if _, err := ReproductionPatchID(2); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("patch 2")
	}
	if slot, err := ReproductionSlotID(1, 7); err != nil || slot != 2016 {
		t.Fatalf("last slot: %d %v", slot, err)
	}
	if _, err := ReproductionSlotID(2, 0); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("slot out of range")
	}
	if home, err := ReproductionFounderPatchID(9); err != nil || home != 1002 {
		t.Fatalf("founder 9 home: %d %v", home, err)
	}
	if _, err := ReproductionFounderPatchID(17); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("founder 17")
	}
	if id, err := ReproductionNewbornID(0); err != nil || id != 10001 {
		t.Fatalf("first newborn: %d %v", id, err)
	}
	if id, err := ReproductionNewbornID(15); err != nil || id != 10016 {
		t.Fatalf("last newborn: %d %v", id, err)
	}
	if _, err := ReproductionNewbornID(16); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("newborn cap")
	}
	// Phase clock: v3 phases byte-identical, birth phases at h+40m+5µs/6µs.
	base := sim.SimTime(5) * sim.SimTime(ReproductionHour)
	for _, phase := range []struct {
		n    int
		want sim.SimTime
	}{
		{ReproductionPhasePulse, base},
		{ReproductionPhaseClaim, base + 1},
		{ReproductionPhaseGathered, base + sim.SimTime(ReproductionGatherDuration) + 1},
		{ReproductionPhaseBagDecision, base + sim.SimTime(ReproductionGatherDuration) + 2},
		{ReproductionPhaseBuildStart, base + sim.SimTime(ReproductionGatherDuration) + 3},
		{ReproductionPhasePairedMeal, base + sim.SimTime(ReproductionGatherDuration+ReproductionBuildDuration) + 4},
		{ReproductionPhaseBirthRequest, base + sim.SimTime(ReproductionGatherDuration+ReproductionBuildDuration) + 5},
		{ReproductionPhaseBirthReply, base + sim.SimTime(ReproductionGatherDuration+ReproductionBuildDuration) + 6},
	} {
		at, err := ReproductionPhaseTime(5, phase.n)
		if err != nil || at != phase.want {
			t.Fatalf("phase %d: %d %v", phase.n, at, err)
		}
	}
	for _, bad := range []struct{ hour, phase int }{{-1, 0}, {168, 0}, {5, -1}, {5, 8}} {
		if _, err := ReproductionPhaseTime(bad.hour, bad.phase); !errors.Is(err, ErrReproductionContract) {
			t.Fatalf("bad phase %+v", bad)
		}
	}
	// Frozen stream constants and draw positions.
	if ReproductionBirthStream != 0x47454E33 || ReproductionFounderStream != 0x464F554E {
		t.Fatal("frozen streams")
	}
	position, err := ReproductionBirthDrawPosition(0)
	if err != nil || position != 0 {
		t.Fatalf("first ordinal position: %d %v", position, err)
	}
	position, err = ReproductionBirthDrawPosition(4)
	if err != nil || position != 28 {
		t.Fatalf("fifth ordinal position: %d %v", position, err)
	}
	if _, err := ReproductionBirthDrawPosition(16); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("draw position cap")
	}
}

var reproductionRulesDeclared = func() map[sim.RuleID]bool {
	declared := map[sim.RuleID]bool{}
	for _, d := range []component.ComponentDescriptor{ReproductionPatchDescriptor(), ReproductionSlotDescriptor(), ReproductionBagDescriptor(), ReproductionBodyDescriptor(), ReproductionWorksiteDescriptor(), ReproductionGranaryDescriptor(), ReproductionGenomeDescriptor(), ReproductionBirthRequestDescriptor()} {
		for _, rule := range d.TransitionRules {
			declared[rule.ID] = true
		}
	}
	return declared
}()

func TestReproductionRegistryIsolationAndIdentityProjections(t *testing.T) {
	reg, err := ReproductionRegistry()
	if err != nil || len(reg.TypeIDs()) != 8 {
		t.Fatalf("registry: %v %v", reg.TypeIDs(), err)
	}
	descriptors := []component.ComponentDescriptor{ReproductionPatchDescriptor(), ReproductionSlotDescriptor(), ReproductionBagDescriptor(), ReproductionBodyDescriptor(), ReproductionWorksiteDescriptor(), ReproductionGranaryDescriptor(), ReproductionGenomeDescriptor(), ReproductionBirthRequestDescriptor()}
	ruleIDs := map[sim.RuleID]bool{}
	for _, d := range descriptors {
		got, err := reg.Describe(d.TypeID)
		if err != nil || !reflect.DeepEqual(got, d) {
			t.Fatalf("descriptor %d: %v", d.TypeID, err)
		}
		if got.SchemaVersion != ReproductionSchemaVersion || got.MigrationPolicy != component.MigrationRejectUnlisted || got.StorageClass != component.DynamicStorage {
			t.Fatalf("descriptor %d identity: %+v", d.TypeID, got)
		}
		localRules := map[sim.RuleID]bool{}
		for _, rule := range got.TransitionRules {
			if rule.Version != ReproductionRuleVersion || rule.ID < 401 || rule.ID > 413 || localRules[rule.ID] {
				t.Fatalf("rule %+v", rule)
			}
			localRules[rule.ID] = true
			ruleIDs[rule.ID] = true
		}
		for _, field := range got.Fields {
			for _, projection := range got.Projections {
				if projection.Version != ReproductionProjectionVersion || projection.MissingBehavior != component.PreserveSourceState {
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
	for id := 401; id <= 413; id++ {
		if !ruleIDs[sim.RuleID(id)] {
			t.Fatalf("missing rule %d", id)
		}
	}
	// Frozen bounds on the inherited and new fields.
	bounds := map[sim.ComponentTypeID]map[sim.FieldID][2]int64{
		ReproductionBodyTypeID:     {ReproductionBodyBasalDebtField: {0, 13}, ReproductionBodyBasalSpentField: {0, 368}},
		ReproductionWorksiteTypeID: {ReproductionWorksiteWearDebtField: {0, 14}},
		ReproductionGranaryTypeID:  {ReproductionGranaryYieldDebtField: {0, 55}, ReproductionGranaryYieldTotalField: {0, 1680}},
		ReproductionGenomeTypeID: {ReproductionGenomeLocusMField: {5, 7}, ReproductionGenomeLocusYField: {5, 7}, ReproductionGenomeLocusWField: {5, 7},
			ReproductionGenomeBirthHourField: {0, 168}, ReproductionGenomeDeathHourField: {0, 479}, ReproductionGenomeDiedHourField: {-1, 168}, ReproductionGenomeBirthGranaryPaidField: {0, 32}},
		ReproductionBirthRequestTypeID: {ReproductionBirthRequestHourField: {-1, 167}, ReproductionBirthRequestReportedCapitalField: {0, 8}, ReproductionBirthRequestReportedGranaryField: {0, 8}, ReproductionBirthRequestStatusField: {0, 4}, ReproductionBirthRequestLastRequestHourField: {-1, 167}, ReproductionBirthRequestLastReplyHourField: {-1, 167}},
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
			bounds := byField[field].Bounds
			if !bounds.HasMinimum || !bounds.HasMaximum || int64(bounds.Minimum) != want[0] || int64(bounds.Maximum) != want[1] {
				t.Fatalf("type %d field %d bounds %+v want %v", typeID, field, bounds, want)
			}
		}
	}
	// The registry is disjoint from every earlier world.
	priors := []component.Registry{}
	for _, build := range []func() (component.Registry, error){FoodFlowRegistry, SocialFoodRegistry, CapacityRegistry, survivalRegistry} {
		old, err := build()
		if err != nil {
			t.Fatal(err)
		}
		priors = append(priors, old)
	}
	for _, prior := range priors {
		for _, priorID := range prior.TypeIDs() {
			if _, err := reg.Describe(priorID); err == nil {
				t.Fatalf("v4 registry collides with prior type %d", priorID)
			}
		}
	}
	// Bidirectional cross-version rejection: a v4 seed set cannot restore
	// into any earlier registry, and no earlier seed set can restore into
	// the v4 registry.
	if _, _, err := component.NewReader(reg, 7, reproductionNeutralSeeds()); err != nil {
		t.Fatalf("v4 seeds into v4 registry: %v", err)
	}
	if _, _, err := component.NewReader(reg, 7, capacityNeutralSeeds(8)); err == nil {
		t.Fatal("v3 seeds restored into the v4 registry")
	}
	priorRegistry, err := CapacityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var v4Seed component.ComponentSeed
	for _, seed := range reproductionNeutralSeeds() {
		v4Seed = seed
		break
	}
	if _, _, err := component.NewReader(priorRegistry, 7, []component.ComponentSeed{v4Seed}); err == nil {
		t.Fatal("v4 seed restored into the v3 registry")
	}
}

func TestReproductionOwnershipGuardGenomeImmutability(t *testing.T) {
	patch := func(rule sim.RuleID, cause kernel.Cause, entity sim.EntityID, componentType sim.ComponentTypeID, field sim.FieldID, key string) kernel.Proposal {
		return kernel.Proposal{Key: key, Time: 5, Cause: cause, Rule: rule, RuleVersion: ReproductionRuleVersion,
			Patches: []component.Patch{{Entity: entity, Component: componentType, SchemaVersion: ReproductionSchemaVersion, Field: field, Value: sim.IntegerValue(0)}}}
	}
	world := kernel.Cause{World: true}
	genome := func(rule sim.RuleID, cause kernel.Cause, field sim.FieldID, key string) kernel.Proposal {
		return patch(rule, cause, 1, ReproductionGenomeTypeID, field, key)
	}
	// G5 genome immutability: died-hour only by rules 404/413.
	for _, key := range []string{"died-by-basal", "died-by-age-death"} {
		rule := ReproductionBasalRule
		if key == "died-by-age-death" {
			rule = ReproductionAgeDeathRule
		}
		if err := ReproductionCheckOwnership(genome(rule, world, ReproductionGenomeDiedHourField, key)); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	for _, key := range []string{"died-by-yield", "died-by-reply", "died-by-request"} {
		rule := ReproductionYieldRule
		cause := world
		if key == "died-by-reply" {
			rule, cause = ReproductionBirthReplyRule, kernel.Cause{Actor: 2}
		}
		if key == "died-by-request" {
			rule, cause = ReproductionBirthRequestRule, kernel.Cause{Actor: 1}
		}
		if err := ReproductionCheckOwnership(genome(rule, cause, ReproductionGenomeDiedHourField, key)); !errors.Is(err, ErrReproductionContract) {
			t.Fatalf("%s survived the guard", key)
		}
	}
	// Loci, hours, and parent refs are never patchable post-creation.
	for _, field := range []sim.FieldID{ReproductionGenomeLocusMField, ReproductionGenomeLocusYField, ReproductionGenomeLocusWField, ReproductionGenomeParentAField, ReproductionGenomeParentBField, ReproductionGenomeBirthHourField, ReproductionGenomeDeathHourField} {
		for _, rule := range []sim.RuleID{ReproductionBasalRule, ReproductionAgeDeathRule, ReproductionBirthReplyRule, ReproductionBirthRequestRule, ReproductionYieldRule} {
			cause := world
			if rule == ReproductionBirthRequestRule || rule == ReproductionBirthReplyRule {
				cause = kernel.Cause{Actor: 2}
			}
			if err := ReproductionCheckOwnership(genome(rule, cause, field, "forged")); !errors.Is(err, ErrReproductionContract) {
				t.Fatalf("forged genome field %d under rule %d survived", field, rule)
			}
		}
	}
	// birth-granary-paid: rule 410 only, and the consenter's own row must
	// be part of the proposal (the cause is always one of the parents).
	replyPaid := kernel.Proposal{Key: "paid-by-reply", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: ReproductionBirthReplyRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{
			{Entity: 1, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGenomeBirthGranaryPaidField, Value: sim.IntegerValue(2)},
			{Entity: 2, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGenomeBirthGranaryPaidField, Value: sim.IntegerValue(2)},
		}}
	if err := ReproductionCheckOwnership(replyPaid); err != nil {
		t.Fatalf("paid-by-reply: %v", err)
	}
	replyPaid.Patches = replyPaid.Patches[:1]
	if err := ReproductionCheckOwnership(replyPaid); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("reply without the consenter's own row survived")
	}
	if err := ReproductionCheckOwnership(genome(ReproductionBasalRule, world, ReproductionGenomeBirthGranaryPaidField, "paid-by-basal")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("paid-by-basal survived")
	}
	// Rule/component mismatches: world rules need a world cause; actor rules
	// need their own rows.
	if err := ReproductionCheckOwnership(patch(ReproductionProduceRule, kernel.Cause{Actor: 1}, 1001, ReproductionPatchTypeID, ReproductionPatchYieldField, "actor-produce")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("actor-caused produce survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionYieldRule, kernel.Cause{Actor: 1}, 1, ReproductionGranaryTypeID, ReproductionGranaryStockField, "actor-yield")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("actor-caused yield survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionGatherRule, kernel.Cause{Actor: 1}, 1, ReproductionGranaryTypeID, ReproductionGranaryStockField, "gather-granary")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("gather into granary survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionBirthRequestRule, kernel.Cause{Actor: 1}, 2, ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField, "cross-request")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("cross-owner request survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionBirthRequestRule, kernel.Cause{Actor: 1}, 1, ReproductionGranaryTypeID, ReproductionGranaryStockField, "request-granary")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("request into granary survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionBirthRefuseRule, kernel.Cause{Actor: 2}, 1, ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField, "refuse-no-cause")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("refusal without the cause's own row survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionBirthRefuseRule, kernel.Cause{Actor: 2}, 1, ReproductionGranaryTypeID, ReproductionGranaryStockField, "refuse-granary")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("refusal into granary survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionBirthExpireRule, kernel.Cause{Actor: 2}, 1, ReproductionBirthRequestTypeID, ReproductionBirthRequestStatusField, "actor-expire")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("actor-caused expiry survived")
	}
	if err := ReproductionCheckOwnership(patch(ReproductionAgeDeathRule, kernel.Cause{Actor: 1}, 1, ReproductionGenomeTypeID, ReproductionGenomeDiedHourField, "actor-age-death")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("actor-caused age-death survived")
	}
	// Foreign components are unreachable under v4 rules.
	if err := ReproductionCheckOwnership(patch(ReproductionConsumeRule, kernel.Cause{Actor: 1}, 1, CapacityBodyTypeID, CapacityBodyEnergyField, "v3-body")); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("v3 component under v4 rule survived")
	}
	// Allocations exist only under rule 410 and only for per-actor rows.
	seed, err := reproductionNewbornAllocations(10001, 1001, testNewbornState(t))
	if err != nil {
		t.Fatal(err)
	}
	notReply := kernel.Proposal{Key: "alloc-not-reply", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: ReproductionYieldRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryStockField, Value: sim.IntegerValue(0)}}, Allocations: []component.ComponentSeed{seed[0]}}
	if err := ReproductionCheckOwnership(notReply); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("allocation under non-reply rule survived")
	}
	foreignAlloc := kernel.Proposal{Key: "alloc-foreign", Time: 5, Cause: kernel.Cause{Actor: 2}, Rule: ReproductionBirthReplyRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGranaryStockField, Value: sim.IntegerValue(0)}}, Allocations: []component.ComponentSeed{{Entity: 10001, Component: CapacityBodyTypeID}}}
	if err := ReproductionCheckOwnership(foreignAlloc); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("foreign allocation survived")
	}
	// Wrong rule version is rejected outright.
	wrongVersion := patch(ReproductionBasalRule, world, 1, ReproductionBodyTypeID, ReproductionBodyEnergyField, "wrong-version")
	wrongVersion.RuleVersion = 3
	if err := ReproductionCheckOwnership(wrongVersion); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("v3 rule version survived")
	}
	// A well-formed basal row passes.
	if err := ReproductionCheckOwnership(patch(ReproductionBasalRule, world, 1, ReproductionBodyTypeID, ReproductionBodyEnergyField, "basal-ok")); err != nil {
		t.Fatalf("basal-ok: %v", err)
	}
}

func TestReproductionKernelAdmissionBirthEntityCreateAndNegativeFixtures(t *testing.T) {
	reg, err := ReproductionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	// Two gate-passing parents on patch 1001: the committed rows must be
	// ledger-consistent with the contract fixture, so the seeds for actors
	// 1 and 2 carry the same capital/granary/body values.
	seeds := reproductionNeutralSeeds()
	reproductionSeedGateParent(t, seeds, 1)
	reproductionSeedGateParent(t, seeds, 2)
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ := k.Snapshot()

	proposer := testParentState(t, 1)
	consenter := testParentState(t, 2)
	request, err := ReproductionBirthRequest(0, 1, 2, 3, 4, proposer)
	if err != nil {
		t.Fatal(err)
	}
	requestProposal, err := ReproductionBirthRequestProposal("request/0/1", 0, 1, 2, 3, 4, proposer)
	if err != nil {
		t.Fatal(err)
	}
	if plan, err := k.Plan(requestProposal, authority); err != nil {
		t.Fatalf("request plan: %v", err)
	} else if _, err := k.CommitBatch([]kernel.Plan{plan}); err != nil {
		t.Fatalf("request commit: %v", err)
	}
	_, authority, _ = k.Snapshot()
	draws := reproductionDrawsForSeed(t, 0)
	result, err := ReproductionBirthReply(0, 0, true, draws, 1, 2, request, consenter)
	if err != nil || result.Decision != ReproductionReplyAccept {
		t.Fatalf("reply: %+v %v", result, err)
	}
	replyProposal, err := ReproductionBirthReplyProposal("reply/0/2", 0, 1, 2, result)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := k.Plan(replyProposal, authority)
	if err != nil {
		t.Fatalf("birth plan: %v", err)
	}
	events, err := k.CommitBatch([]kernel.Plan{plan})
	if err != nil || len(events) != 1 {
		t.Fatalf("birth commit: %v", err)
	}
	reader, auth, version := k.Snapshot()
	newborn := reproductionSnapshotActor(t, reader, auth, version, 10001)
	if newborn.Body.Energy != ReproductionNewbornEnergy || newborn.Body.Hunger != 0 || newborn.Genome.ParentA != 1 || newborn.Genome.ParentB != 2 || newborn.Bag.Source != 1001 {
		t.Fatalf("newborn rows: %+v", newborn)
	}
	parent := reproductionSnapshotActor(t, reader, auth, version, 1)
	if parent.Granary.Stock != 5-ReproductionParentGranaryCost || parent.Genome.BirthGranaryPaid != ReproductionParentGranaryCost ||
		parent.Request.Status != ReproductionRequestAccepted || !validReproductionActor(1, parent) {
		t.Fatalf("proposer after birth: %+v", parent)
	}
	consenterAfter := reproductionSnapshotActor(t, reader, auth, version, 2)
	if consenterAfter.Request.LastReplyHour != 0 || consenterAfter.Genome.BirthGranaryPaid != ReproductionParentGranaryCost {
		t.Fatalf("consenter after birth: %+v", consenterAfter)
	}

	// Negative fixture: ID collision — allocating an existing ID is
	// rejected with zero mutation.
	collide := replyProposal
	collide.Key = "id-collision"
	for i := range collide.Allocations {
		collide.Allocations[i].Entity = 5
	}
	if _, err := k.Plan(collide, auth); err == nil {
		t.Fatal("ID collision admitted")
	}
	// Negative fixture: partial allocation rows.
	partial := replyProposal
	partial.Key = "partial-rows"
	partial.Allocations = []component.ComponentSeed{{Entity: 10002, Component: ReproductionBodyTypeID, Fields: partial.Allocations[1].Fields[:3]}}
	if _, err := k.Plan(partial, auth); err == nil {
		t.Fatal("partial allocation admitted")
	}
	// Negative fixture: an allocation may not overlap this batch's patches.
	overlap := replyProposal
	overlap.Key = "overlap"
	overlap.Patches = append(overlap.Patches, component.Patch{Entity: 10001, Component: ReproductionBodyTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionBodyEnergyField, Value: sim.IntegerValue(9)})
	if _, err := k.Plan(overlap, auth); err == nil {
		t.Fatal("allocation/patch overlap admitted")
	}
	// Negative fixture: a second entity-create in one batch cannot reuse
	// the newborn ID.
	duplicate := replyProposal
	duplicate.Key = "duplicate-create"
	duplicate.Allocations = append(duplicate.Allocations, component.ComponentSeed{Entity: 10001, Component: CapacityBodyTypeID})
	if _, err := k.Plan(duplicate, auth); err == nil {
		t.Fatal("duplicate allocation admitted")
	}
	// The kernel alone cannot refuse a schema-valid forged genome write;
	// the ownership guard is the enforcement point (v3 discipline).
	forged := kernel.Proposal{Key: "forged-locus", Time: 2_000_000_003, Cause: kernel.Cause{Actor: 2}, Rule: ReproductionBirthReplyRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion, Field: ReproductionGenomeLocusMField, Value: sim.IntegerValue(7)}}}
	if _, err := k.Plan(forged, auth); err != nil {
		t.Fatalf("expected raw kernel schema admission: %v", err)
	}
	if err := ReproductionCheckOwnership(forged); !errors.Is(err, ErrReproductionContract) {
		t.Fatalf("forged locus write survived the guard: %v", err)
	}
}

func TestReproductionIntentProposalsRejectOffPhaseInstants(t *testing.T) {
	actor := testParentState(t, 1)
	slot := ReproductionSlotState{Stock: 1}
	if _, err := ReproductionGatherProposal("gather", 0, 1, 2001, slot, actor); err != nil {
		t.Fatalf("gather at claim: %v", err)
	}
	held := actor
	held.Bag.Units = 1
	held.Body.LastGatherHour = 0
	if _, err := ReproductionConsumeProposal("consume", 0, 1, held); err != nil {
		t.Fatalf("consume at bag decision: %v", err)
	}
	if _, err := ReproductionBuildProposal("build", 0, 1, held); err != nil {
		t.Fatalf("build: %v", err)
	}
	claim, _ := ReproductionPhaseTime(0, ReproductionPhaseClaim)
	paired, _ := ReproductionPhaseTime(0, ReproductionPhasePairedMeal)
	if _, err := ReproductionEatStoredProposal("eat-claim", 0, claim, 1, actor); err != nil {
		t.Fatalf("eat-stored at claim: %v", err)
	}
	if _, err := ReproductionEatStoredProposal("eat-paired", 0, paired, 1, actor); err != nil {
		t.Fatalf("eat-stored at paired meal: %v", err)
	}
	offPhase, _ := ReproductionPhaseTime(0, ReproductionPhaseBirthRequest)
	if _, err := ReproductionEatStoredProposal("eat-off", 0, offPhase, 1, actor); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("eat-stored off-phase admitted")
	}
	if _, err := ReproductionBirthRequestProposal("request", 0, 1, 2, 2, 3, actor); err != nil {
		t.Fatalf("request at birth-request phase: %v", err)
	}
	if _, err := ReproductionGatherProposal("gather-off", 168, 1, 2001, slot, actor); !errors.Is(err, ErrReproductionContract) {
		t.Fatal("gather beyond the horizon admitted")
	}
}

// reproductionSeedGateParent rewrites one founder's seed rows to the
// gate-passing testParentState ledger so kernel-committed state stays
// G2a/G3-consistent across a birth.
func reproductionSeedGateParent(t *testing.T, seeds []component.ComponentSeed, actor sim.EntityID) {
	t.Helper()
	values := map[sim.ComponentTypeID]map[sim.FieldID]int64{
		ReproductionBodyTypeID:     {ReproductionBodyEnergyField: 5, ReproductionBodyBasalSpentField: 6},
		ReproductionWorksiteTypeID: {ReproductionWorksiteCapitalField: 3, ReproductionWorksiteInvestedUnitsField: 6, ReproductionWorksitePointsCreatedField: 3},
		ReproductionGranaryTypeID:  {ReproductionGranaryStockField: 5, ReproductionGranaryYieldTotalField: 5},
	}
	for i := range seeds {
		fields, ok := values[seeds[i].Component]
		if !ok || seeds[i].Entity != actor {
			continue
		}
		for j := range seeds[i].Fields {
			if value, ok := fields[seeds[i].Fields[j].Field]; ok {
				seeds[i].Fields[j].Value = sim.IntegerValue(value)
			}
		}
	}
}

// testParentState builds a gate-passing parent: capital 3, granary 5,
// energy 5, on its founder home patch.
func testParentState(t *testing.T, actor sim.EntityID) ReproductionActorState {
	t.Helper()
	a, err := ReproductionFounderState(actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	a.Worksite = ReproductionWorksiteState{Capital: 3, InvestedUnits: 6, PointsCreated: 3, LastBuildHour: ReproductionNeverHour}
	a.Granary = ReproductionGranaryState{Stock: 5, YieldTotal: 5, LastStoredMealHour: ReproductionNeverHour}
	a.Body = ReproductionBodyState{Energy: 5, BasalSpent: 6, LastGatherHour: ReproductionNeverHour}
	if !validReproductionActor(actor, a) {
		t.Fatalf("parent fixture invalid: %+v", a)
	}
	return a
}

func testNewbornState(t *testing.T) ReproductionActorState {
	t.Helper()
	draws := []uint64{0, 0, 0, 1 << 4, 0, 0, 0} // mutate locus-m down, clamp-safe
	child, _, err := ReproductionChildGenome(1, 2, mustGenome(t, 1), mustGenome(t, 2), 0, draws)
	if err != nil {
		t.Fatal(err)
	}
	newborn := ReproductionActorState{
		Body:     ReproductionBodyState{Energy: ReproductionNewbornEnergy, LastGatherHour: ReproductionNeverHour},
		Bag:      ReproductionBagState{Source: 1001},
		Worksite: ReproductionWorksiteState{LastBuildHour: ReproductionNeverHour},
		Granary:  ReproductionGranaryState{LastStoredMealHour: ReproductionNeverHour},
		Genome:   child,
		Request:  ReproductionIdleRequestState(),
	}
	if !validReproductionActor(10001, newborn) {
		t.Fatalf("newborn fixture invalid: %+v", newborn)
	}
	return newborn
}

func mustGenome(t *testing.T, actor sim.EntityID) ReproductionGenomeState {
	t.Helper()
	genome, err := ReproductionFounderGenome(actor, 120)
	if err != nil {
		t.Fatal(err)
	}
	return genome
}

// reproductionDrawsForSeed derives the seven birth draws exactly as the
// runner will: the frozen birth stream at position 7×(ordinal−1).
func reproductionDrawsForSeed(t *testing.T, births int64) []uint64 {
	t.Helper()
	position, err := ReproductionBirthDrawPosition(births)
	if err != nil {
		t.Fatal(err)
	}
	stream := sim.NewRandomStream(sim.RandomState{Seed: 7, Stream: ReproductionBirthStream, Position: position})
	draws := make([]uint64, ReproductionDrawsPerBirth)
	for i := range draws {
		draws[i] = stream.Uint64()
	}
	return draws
}
