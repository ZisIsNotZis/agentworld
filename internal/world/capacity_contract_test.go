package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

func capacityTestActor() CapacityActorState {
	return CapacityActorState{
		Body:     CapacityBodyState{Energy: CapacityInitialEnergy, LastGatherHour: CapacityNeverHour},
		Worksite: CapacityWorksiteState{LastBuildHour: CapacityNeverHour},
		Granary:  CapacityGranaryState{LastStoredMealHour: CapacityNeverHour},
	}
}

func TestCapacityConstantsClockAndDistinctIDs(t *testing.T) {
	if CapacityFormatVersion != 3 || CapacitySchemaVersion != 3 || CapacityRuleVersion != 3 || CapacityProjectionVersion != 3 {
		t.Fatal("v3 versions changed")
	}
	if CapacityPointCostWip != 2 || CapacityYieldPerPoint != 1 || CapacityKMax != 8 || CapacityWipMax != 1 ||
		CapacityGranaryMax != 8 || CapacityWearPeriod != 6 || CapacityWearDebtMax != 13 ||
		CapacityBuildDuration != 20*60*1_000_000 || CapacityPolicyTLow != 5 {
		t.Fatal("frozen capacity constants changed")
	}
	if CapacityActorCount != 16 || CapacityPatchCount != 2 || CapacitySlotsPerPatch != 8 || CapacityHorizonHours != 168 ||
		CapacityHour != 3_600_000_000 || CapacityGatherDuration != 20*60*1_000_000 || CapacityMealDuration != 10*60*1_000_000 ||
		CapacityInitialEnergy != 11 || CapacityEnergyCapacity != 12 || CapacityHungerCapacity != 24 ||
		CapacityBagCapacity != 1 || CapacityNeverHour != -1 {
		t.Fatal("fixture discipline constants changed")
	}
	for i, want := range []sim.ComponentTypeID{0x80000030, 0x80000031, 0x80000032, 0x80000033, 0x80000034, 0x80000035} {
		got := []sim.ComponentTypeID{CapacityPatchTypeID, CapacitySlotTypeID, CapacityBagTypeID, CapacityBodyTypeID, CapacityWorksiteTypeID, CapacityGranaryTypeID}[i]
		if got != want {
			t.Fatalf("type ID %d: got %#x", i, got)
		}
	}
	for i, want := range []sim.RuleID{301, 302, 303, 304, 305, 306, 307, 308} {
		got := []sim.RuleID{CapacityProduceRule, CapacityGatherRule, CapacityConsumeRule, CapacityBasalRule, CapacityBuildRule, CapacityYieldRule, CapacityWearRule, CapacityEatStoredRule}[i]
		if got != want {
			t.Fatalf("rule ID %d: got %d", i, got)
		}
	}
	seen := map[sim.EntityID]bool{}
	for actor := sim.EntityID(1); actor <= CapacityActorCount; actor++ {
		home, err := CapacityActorPatchID(actor)
		if err != nil || home != sim.EntityID(1001+(int(actor)-1)/CapacitySlotsPerPatch) {
			t.Fatalf("actor %d home %d: %v", actor, home, err)
		}
		seen[actor] = true
	}
	for p := 0; p < CapacityPatchCount; p++ {
		patch, err := CapacityPatchID(p)
		if err != nil || seen[patch] {
			t.Fatalf("patch %d: %v", p, err)
		}
		seen[patch] = true
		for s := 0; s < CapacitySlotsPerPatch; s++ {
			slot, err := CapacitySlotID(p, s)
			if err != nil || seen[slot] || slot < 2001 || slot > 2016 {
				t.Fatalf("slot %d/%d: %v", p, s, err)
			}
			seen[slot] = true
		}
	}
	if len(seen) != CapacityActorCount+CapacityPatchCount+CapacityPatchCount*CapacitySlotsPerPatch {
		t.Fatalf("entity IDs: %d", len(seen))
	}
	for _, f := range []func() error{
		func() error { _, err := CapacityPatchID(-1); return err },
		func() error { _, err := CapacityPatchID(2); return err },
		func() error { _, err := CapacitySlotID(0, 8); return err },
		func() error { _, err := CapacitySlotID(2, 0); return err },
		func() error { _, err := CapacityActorPatchID(0); return err },
		func() error { _, err := CapacityActorPatchID(17); return err },
	} {
		if !errors.Is(f(), ErrCapacityContract) {
			t.Fatal("bad entity ID accepted")
		}
	}
	if at, err := CapacityHourTime(0); err != nil || at != 0 {
		t.Fatalf("h0: %d %v", at, err)
	}
	if at, err := CapacityHourTime(168); err != nil || at != 168*sim.SimTime(CapacityHour) {
		t.Fatalf("h168: %d %v", at, err)
	}
	for _, h := range []int{-1, 169} {
		if _, err := CapacityHourTime(h); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("bad hour time %d: %v", h, err)
		}
	}
	base := sim.SimTime(5) * sim.SimTime(CapacityHour)
	for _, phase := range []struct {
		n    int
		want sim.SimTime
	}{
		{CapacityPhasePulse, base},
		{CapacityPhaseClaim, base + 1},
		{CapacityPhaseGathered, base + sim.SimTime(CapacityGatherDuration) + 1},
		{CapacityPhaseBagDecision, base + sim.SimTime(CapacityGatherDuration) + 2},
		{CapacityPhaseBuildStart, base + sim.SimTime(CapacityGatherDuration) + 3},
		{CapacityPhasePairedMeal, base + sim.SimTime(CapacityGatherDuration+CapacityBuildDuration) + 4},
	} {
		at, err := CapacityPhaseTime(5, phase.n)
		if err != nil || at != phase.want {
			t.Fatalf("phase %d: %d %v", phase.n, at, err)
		}
	}
	start, _ := CapacityPhaseTime(5, CapacityPhaseBuildStart)
	if done := CapacityBuildCompletion(start); done != base+sim.SimTime(CapacityGatherDuration+CapacityBuildDuration)+3 {
		t.Fatalf("build completion %d", done)
	}
	for _, bad := range []struct{ hour, phase int }{{-1, 0}, {168, 0}, {5, -1}, {5, 6}} {
		if _, err := CapacityPhaseTime(bad.hour, bad.phase); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("bad phase %+v: %v", bad, err)
		}
	}
}

func TestCapacityRegistryIsolationAndIdentityProjections(t *testing.T) {
	reg, err := CapacityRegistry()
	if err != nil || len(reg.TypeIDs()) != 6 {
		t.Fatalf("registry: %v %v", reg.TypeIDs(), err)
	}
	descriptors := []component.ComponentDescriptor{CapacityPatchDescriptor(), CapacitySlotDescriptor(), CapacityBagDescriptor(), CapacityBodyDescriptor(), CapacityWorksiteDescriptor(), CapacityGranaryDescriptor()}
	ruleIDs := map[sim.RuleID]bool{}
	for _, d := range descriptors {
		got, err := reg.Describe(d.TypeID)
		if err != nil || !reflect.DeepEqual(got, d) {
			t.Fatalf("descriptor %d: %v", d.TypeID, err)
		}
		if got.SchemaVersion != CapacitySchemaVersion || got.MigrationPolicy != component.MigrationRejectUnlisted || got.StorageClass != component.DynamicStorage {
			t.Fatalf("descriptor %d identity: %+v", d.TypeID, got)
		}
		localRules := map[sim.RuleID]bool{}
		for _, rule := range got.TransitionRules {
			if rule.Version != CapacityRuleVersion || rule.ID < 301 || rule.ID > 308 || localRules[rule.ID] {
				t.Fatalf("rule %+v", rule)
			}
			localRules[rule.ID] = true
			ruleIDs[rule.ID] = true
		}
		for _, field := range got.Fields {
			for _, projection := range got.Projections {
				if projection.Version != CapacityProjectionVersion || projection.MissingBehavior != component.PreserveSourceState {
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
	for id := 301; id <= 308; id++ {
		if !ruleIDs[sim.RuleID(id)] {
			t.Fatalf("missing rule %d", id)
		}
	}
	priors := make([]component.Registry, 0, 3)
	for _, build := range []func() (component.Registry, error){FoodFlowRegistry, SocialFoodRegistry, survivalRegistry} {
		old, err := build()
		if err != nil {
			t.Fatal(err)
		}
		priors = append(priors, old)
	}
	for _, prior := range priors {
		for _, typ := range reg.TypeIDs() {
			if _, err := prior.Describe(typ); !errors.Is(err, component.ErrUnknownComponentType) {
				t.Fatalf("prior registry recognizes v3 type %d", typ)
			}
			d, _ := reg.Describe(typ)
			for _, rule := range d.TransitionRules {
				for _, oldType := range prior.TypeIDs() {
					oldDescriptor, _ := prior.Describe(oldType)
					for _, oldRule := range oldDescriptor.TransitionRules {
						if oldRule.ID == rule.ID {
							t.Fatalf("rule ID %d collides with a prior registry", rule.ID)
						}
					}
				}
			}
		}
		for _, typ := range prior.TypeIDs() {
			if _, err := reg.Describe(typ); !errors.Is(err, component.ErrUnknownComponentType) {
				t.Fatalf("v3 recognizes prior type %d", typ)
			}
		}
		oldKernel, err := kernel.New(prior, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		history, head, err := oldKernel.ExportHistory()
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := kernel.RestoreHistory(reg, history); err == nil {
			t.Fatal("prior history restored into v3")
		}
		fingerprint, err := component.RegistryFingerprint(reg)
		if err != nil || fingerprint == head.RegistryFingerprint {
			t.Fatalf("v3 reused a prior registry identity: %v", err)
		}
	}
	k, err := kernel.New(reg, 0, capacityNeutralSeeds(8))
	if err != nil {
		t.Fatal(err)
	}
	history, _, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	replayed, _, err := kernel.RestoreHistory(reg, history)
	if err != nil {
		t.Fatalf("v3 history failed v3 replay: %v", err)
	}
	for _, prior := range priors {
		if _, _, err := kernel.RestoreHistory(prior, history); err == nil {
			t.Fatal("v3 history restored into a prior registry")
		}
	}
	if replayed == nil {
		t.Fatal("missing replayed kernel")
	}
}

func capacityNeutralSeeds(yield int64) []component.ComponentSeed {
	var seeds []component.ComponentSeed
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	for p := 0; p < CapacityPatchCount; p++ {
		patchID, _ := CapacityPatchID(p)
		patchRef, _ := sim.EntityRefValue(patchID)
		seeds = append(seeds, component.ComponentSeed{Entity: patchID, Component: CapacityPatchTypeID, Fields: []component.FieldSeed{
			{Field: CapacityPatchYieldField, Value: sim.IntegerValue(yield)}, {Field: CapacityPatchPulsesField, Value: sim.IntegerValue(0)},
			{Field: CapacityPatchProducedField, Value: sim.IntegerValue(0)}, {Field: CapacityPatchUnrealizedField, Value: sim.IntegerValue(0)}}})
		for s := 0; s < CapacitySlotsPerPatch; s++ {
			slotID, _ := CapacitySlotID(p, s)
			seeds = append(seeds, component.ComponentSeed{Entity: slotID, Component: CapacitySlotTypeID, Fields: []component.FieldSeed{
				{Field: CapacitySlotStockField, Value: sim.IntegerValue(0)}, {Field: CapacitySlotPatchField, Value: patchRef},
				{Field: CapacitySlotGatheredField, Value: sim.IntegerValue(0)}}})
		}
	}
	for actor := sim.EntityID(1); actor <= CapacityActorCount; actor++ {
		seeds = append(seeds,
			component.ComponentSeed{Entity: actor, Component: CapacityBagTypeID, Fields: []component.FieldSeed{
				{Field: CapacityBagUnitsField, Value: sim.IntegerValue(0)}, {Field: CapacityBagSourceField, Value: missing}}},
			component.ComponentSeed{Entity: actor, Component: CapacityBodyTypeID, Fields: []component.FieldSeed{
				{Field: CapacityBodyEnergyField, Value: sim.IntegerValue(CapacityInitialEnergy)}, {Field: CapacityBodyHungerField, Value: sim.IntegerValue(0)},
				{Field: CapacityBodyBasalSpentField, Value: sim.IntegerValue(0)}, {Field: CapacityBodyCapLostField, Value: sim.IntegerValue(0)},
				{Field: CapacityBodyConsumedField, Value: sim.IntegerValue(0)}, {Field: CapacityBodyLastGatherHourField, Value: sim.IntegerValue(CapacityNeverHour)}}},
			component.ComponentSeed{Entity: actor, Component: CapacityWorksiteTypeID, Fields: []component.FieldSeed{
				{Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(0)}, {Field: CapacityWorksiteWipField, Value: sim.IntegerValue(0)},
				{Field: CapacityWorksiteWearDebtField, Value: sim.IntegerValue(0)}, {Field: CapacityWorksiteInvestedUnitsField, Value: sim.IntegerValue(0)},
				{Field: CapacityWorksitePointsCreatedField, Value: sim.IntegerValue(0)}, {Field: CapacityWorksitePointsDecayedField, Value: sim.IntegerValue(0)},
				{Field: CapacityWorksiteLastBuildHourField, Value: sim.IntegerValue(CapacityNeverHour)}}},
			component.ComponentSeed{Entity: actor, Component: CapacityGranaryTypeID, Fields: []component.FieldSeed{
				{Field: CapacityGranaryStockField, Value: sim.IntegerValue(0)}, {Field: CapacityGranaryYieldTotalField, Value: sim.IntegerValue(0)},
				{Field: CapacityGranaryYieldUnrealizedField, Value: sim.IntegerValue(0)}, {Field: CapacityGranaryStoredMealsField, Value: sim.IntegerValue(0)},
				{Field: CapacityGranaryLastStoredMealHourField, Value: sim.IntegerValue(CapacityNeverHour)}}})
	}
	return seeds
}

func capacitySnapshotActor(t *testing.T, reader component.Reader, auth component.Authority, version sim.WorldVersion, actor sim.EntityID) CapacityActorState {
	t.Helper()
	integer := func(typ sim.ComponentTypeID, field sim.FieldID) int64 {
		view, err := reader.Read(component.ReadRequest{Entity: actor, Component: typ, Fields: []sim.FieldID{field}, WorldVersion: version, Authority: auth})
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
	view, err := reader.Read(component.ReadRequest{Entity: actor, Component: CapacityBagTypeID, Fields: []sim.FieldID{CapacityBagSourceField}, WorldVersion: version, Authority: auth})
	if err != nil {
		t.Fatal(err)
	}
	sourceValue, err := view.Value(CapacityBagSourceField)
	if err != nil {
		t.Fatal(err)
	}
	var source sim.EntityID
	if sourceValue.IsPresent() {
		source, err = sourceValue.EntityRef()
		if err != nil {
			t.Fatal(err)
		}
	}
	return CapacityActorState{
		Body: CapacityBodyState{
			Energy: integer(CapacityBodyTypeID, CapacityBodyEnergyField), Hunger: integer(CapacityBodyTypeID, CapacityBodyHungerField),
			BasalSpent: integer(CapacityBodyTypeID, CapacityBodyBasalSpentField), CapLost: integer(CapacityBodyTypeID, CapacityBodyCapLostField),
			Consumed: integer(CapacityBodyTypeID, CapacityBodyConsumedField), LastGatherHour: integer(CapacityBodyTypeID, CapacityBodyLastGatherHourField)},
		Bag: CapacityBagState{Units: integer(CapacityBagTypeID, CapacityBagUnitsField), Source: source},
		Worksite: CapacityWorksiteState{
			Capital: integer(CapacityWorksiteTypeID, CapacityWorksiteCapitalField), Wip: integer(CapacityWorksiteTypeID, CapacityWorksiteWipField),
			WearDebt: integer(CapacityWorksiteTypeID, CapacityWorksiteWearDebtField), InvestedUnits: integer(CapacityWorksiteTypeID, CapacityWorksiteInvestedUnitsField),
			PointsCreated: integer(CapacityWorksiteTypeID, CapacityWorksitePointsCreatedField), PointsDecayed: integer(CapacityWorksiteTypeID, CapacityWorksitePointsDecayedField),
			LastBuildHour: integer(CapacityWorksiteTypeID, CapacityWorksiteLastBuildHourField)},
		Granary: CapacityGranaryState{
			Stock: integer(CapacityGranaryTypeID, CapacityGranaryStockField), YieldTotal: integer(CapacityGranaryTypeID, CapacityGranaryYieldTotalField),
			YieldUnrealized: integer(CapacityGranaryTypeID, CapacityGranaryYieldUnrealizedField), StoredMeals: integer(CapacityGranaryTypeID, CapacityGranaryStoredMealsField),
			LastStoredMealHour: integer(CapacityGranaryTypeID, CapacityGranaryLastStoredMealHourField)},
	}
}

// TestCapacityKernelAdmissionAndFreshSnapshotBuild proves the frozen rule
// attachments admit exactly the validated intentions: gather then build
// commit through the real kernel, the build reads a fresh snapshot, and the
// kernel's schema-only authority (raw in-range patches it cannot refuse) is
// closed by CapacityCheckOwnership at the harness boundary.
func TestCapacityKernelAdmissionAndFreshSnapshotBuild(t *testing.T) {
	reg, err := CapacityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	k, err := kernel.New(reg, 0, capacityNeutralSeeds(8))
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ := k.Snapshot()
	gather, err := CapacityGatherProposal("gather/1/0", 0, 1, 2001, CapacitySlotState{Stock: 1}, capacityTestActor())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := k.Plan(gather, authority)
	if err != nil {
		t.Fatalf("gather plan: %v", err)
	}
	events, err := k.CommitBatch([]kernel.Plan{plan})
	if err != nil || len(events) != 1 || len(events[0].Deltas) != 5 {
		t.Fatalf("gather commit: %d %v", len(events), err)
	}
	reader, authority, version := k.Snapshot()
	if version != 1 {
		t.Fatalf("version %d", version)
	}
	state := capacitySnapshotActor(t, reader, authority, version, 1)
	if state.Bag != (CapacityBagState{Units: 1, Source: 1001}) || state.Body.LastGatherHour != 0 {
		t.Fatalf("kernel gather state: %+v", state)
	}
	build, err := CapacityBuildProposal("build/1/0", 0, 1, state)
	if err != nil {
		t.Fatal(err)
	}
	if plan, err = k.Plan(build, authority); err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if events, err = k.CommitBatch([]kernel.Plan{plan}); err != nil || len(events) != 1 || len(events[0].Deltas) != 7 {
		t.Fatalf("build commit: %d %v", len(events), err)
	}
	reader, authority, version = k.Snapshot()
	state = capacitySnapshotActor(t, reader, authority, version, 1)
	if state.Worksite.Wip != 1 || state.Worksite.InvestedUnits != 1 || state.Worksite.LastBuildHour != 0 || state.Bag.Units != 0 || state.Body.Energy != CapacityInitialEnergy {
		t.Fatalf("kernel build state: %+v", state)
	}
	if !validCapacityActor(1, state) {
		t.Fatal("kernel-committed state failed semantic validation")
	}
	_, auth, _ := k.Snapshot()
	for _, bad := range []kernel.Proposal{
		// Kernel-level G7: schema admission still refuses out-of-bounds values.
		{Key: "bounds", Time: 2_000_000_003, Cause: kernel.Cause{Actor: 1}, Rule: CapacityBuildRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 1, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(CapacityKMax + 1)}}},
		// No v1/v2 component is reachable under a v3 rule.
		{Key: "foreign", Time: 2_000_000_003, Cause: kernel.Cause{Actor: 1}, Rule: CapacityConsumeRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 1, Component: FoodFlowBagTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowBagUnitsField, Value: sim.IntegerValue(1)}}},
		// Undeclared rule and wrong rule version.
		{Key: "rule", Time: 2_000_000_003, Cause: kernel.Cause{Actor: 1}, Rule: 999, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 1, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(0)}}},
		{Key: "ruleversion", Time: 2_000_000_003, Cause: kernel.Cause{Actor: 1}, Rule: CapacityBuildRule, RuleVersion: 1, Patches: []component.Patch{{Entity: 1, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(0)}}},
	} {
		if _, err := k.Plan(bad, auth); err == nil {
			t.Fatalf("kernel admitted invalid proposal %s", bad.Key)
		}
	}
	// The kernel alone cannot refuse a schema-valid cross-owner worksite
	// patch; the ownership guard is the enforcement point for the runner and
	// journal replay.
	cross := kernel.Proposal{Key: "cross", Time: 2_000_000_003, Cause: kernel.Cause{Actor: 5}, Rule: CapacityBuildRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 6, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(1)}}}
	if _, err := k.Plan(cross, auth); err != nil {
		t.Fatalf("expected raw kernel schema admission: %v", err)
	}
	if err := CapacityCheckOwnership(cross); !errors.Is(err, ErrCapacityContract) {
		t.Fatalf("cross-owner mutation survived the guard: %v", err)
	}
}

// TestCapacityDescriptorBoundsRejectOutOfRangeSeeds is gate G7 at schema
// level: every declared integer bound rejects one step outside and admits
// both endpoints, for at least one seeded entity per component.
func TestCapacityDescriptorBoundsRejectOutOfRangeSeeds(t *testing.T) {
	reg, err := CapacityRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := component.NewReader(reg, 7, capacityNeutralSeeds(8)); err != nil {
		t.Fatalf("neutral v3 h0 seeds: %v", err)
	}
	first := map[sim.ComponentTypeID]sim.EntityID{}
	for _, seed := range capacityNeutralSeeds(8) {
		if _, ok := first[seed.Component]; !ok {
			first[seed.Component] = seed.Entity
		}
	}
	for _, typ := range reg.TypeIDs() {
		descriptor, _ := reg.Describe(typ)
		entity := first[typ]
		for _, field := range descriptor.Fields {
			if field.Type.Kind != sim.IntegerKind || !field.Bounds.HasMinimum || !field.Bounds.HasMaximum {
				continue
			}
			min, max := int64(field.Bounds.Minimum), int64(field.Bounds.Maximum)
			for _, attempt := range []struct {
				value int64
				valid bool
			}{{min - 1, false}, {min, true}, {max, true}, {max + 1, false}} {
				seeds := capacityNeutralSeeds(8)
				for i := range seeds {
					if seeds[i].Entity != entity || seeds[i].Component != typ {
						continue
					}
					seeds[i].Fields = append([]component.FieldSeed(nil), seeds[i].Fields...)
					for j := range seeds[i].Fields {
						if seeds[i].Fields[j].Field == field.ID {
							seeds[i].Fields[j].Value = sim.IntegerValue(attempt.value)
						}
					}
				}
				_, _, err := component.NewReader(reg, 7, seeds)
				if attempt.valid != (err == nil) {
					t.Fatalf("type %d field %d value %d: %v", typ, field.ID, attempt.value, err)
				}
				if !attempt.valid && !errors.Is(err, component.ErrValueOutsideSchema) {
					t.Fatalf("type %d field %d value %d: %v", typ, field.ID, attempt.value, err)
				}
			}
		}
	}
}

func TestCapacityOwnershipGuardAndIntentProposals(t *testing.T) {
	actor := capacityTestActor()
	gathered := actor
	gathered.Bag = CapacityBagState{Units: 1, Source: 1001}
	gathered.Body.LastGatherHour = 0
	// The frozen constructor set: every intention proposal passes the
	// ownership guard, patches only the cause actor's own rows (plus the
	// claimed slot), and is pinned to the exact frozen phase time.
	gatherProposal, err := CapacityGatherProposal("g/1/0", 0, 1, 2001, CapacitySlotState{Stock: 1}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if gatherProposal.Time != 1 || gatherProposal.Rule != CapacityGatherRule || len(gatherProposal.Patches) != 5 {
		t.Fatalf("gather proposal %+v", gatherProposal)
	}
	if err := CapacityCheckOwnership(gatherProposal); err != nil {
		t.Fatalf("gather ownership: %v", err)
	}
	slotPatches, actorPatches := 0, 0
	for _, p := range gatherProposal.Patches {
		if p.Entity == 2001 && p.Component == CapacitySlotTypeID {
			slotPatches++
		}
		if p.Entity == 1 {
			actorPatches++
		}
	}
	if slotPatches != 2 || actorPatches != 3 {
		t.Fatalf("gather patch split %d/%d", slotPatches, actorPatches)
	}
	eatProposal, err := CapacityEatProposal("e/1/0", 0, 1, gathered)
	if err != nil {
		t.Fatal(err)
	}
	if eatProposal.Time != sim.SimTime(CapacityGatherDuration)+2 || eatProposal.Rule != CapacityConsumeRule {
		t.Fatalf("eat proposal %+v", eatProposal)
	}
	if err := CapacityCheckOwnership(eatProposal); err != nil {
		t.Fatalf("eat ownership: %v", err)
	}
	withGranary := capacityTestActor()
	withGranary.Granary = CapacityGranaryState{Stock: 2, YieldTotal: 2, LastStoredMealHour: CapacityNeverHour}
	claim, _ := CapacityPhaseTime(3, CapacityPhaseClaim)
	paired, _ := CapacityPhaseTime(3, CapacityPhasePairedMeal)
	for _, at := range []sim.SimTime{claim, paired} {
		proposal, err := CapacityEatStoredProposal("es/1/3", 3, at, 1, withGranary)
		if err != nil {
			t.Fatal(err)
		}
		if proposal.Time != at || proposal.Rule != CapacityEatStoredRule || len(proposal.Patches) != 6 {
			t.Fatalf("eat-stored proposal %+v", proposal)
		}
		if err := CapacityCheckOwnership(proposal); err != nil {
			t.Fatalf("eat-stored ownership: %v", err)
		}
	}
	if _, err := CapacityEatStoredProposal("es/wrong-time", 3, claim+1, 1, withGranary); !errors.Is(err, ErrCapacityContract) {
		t.Fatalf("off-phase stored meal: %v", err)
	}
	buildState := gathered
	buildState.Worksite = CapacityWorksiteState{LastBuildHour: CapacityNeverHour}
	buildProposal, err := CapacityBuildProposal("b/1/0", 0, 1, buildState)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := CapacityPhaseTime(0, CapacityPhaseBuildStart); buildProposal.Time != CapacityBuildCompletion(want) || buildProposal.Rule != CapacityBuildRule {
		t.Fatalf("build time %+v", buildProposal)
	}
	if err := CapacityCheckOwnership(buildProposal); err != nil {
		t.Fatalf("build ownership: %v", err)
	}
	bagPatches, worksitePatches := 0, 0
	for _, p := range buildProposal.Patches {
		if p.Component == CapacityBagTypeID {
			bagPatches++
		}
		if p.Component == CapacityWorksiteTypeID {
			worksitePatches++
		}
		if p.Entity != 1 {
			t.Fatalf("build patch on foreign entity %+v", p)
		}
	}
	if bagPatches != 2 || worksitePatches != 5 {
		t.Fatalf("build patch split %d/%d", bagPatches, worksitePatches)
	}
	// Negative fixtures from the frozen gate list, expressed as raw proposals
	// the guard must reject with zero mutation.
	inRange := component.Patch{Entity: 1, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(1)}
	for _, bad := range []struct {
		name     string
		proposal kernel.Proposal
	}{
		{"build-on-anothers-worksite", kernel.Proposal{Key: "x1", Time: 1, Cause: kernel.Cause{Actor: 5}, Rule: CapacityBuildRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 6, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(1)}}}},
		{"stored-meal-from-anothers-granary", kernel.Proposal{Key: "x2", Time: 1, Cause: kernel.Cause{Actor: 5}, Rule: CapacityEatStoredRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 6, Component: CapacityGranaryTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityGranaryStockField, Value: sim.IntegerValue(1)}}}},
		{"gather-patching-anothers-bag", kernel.Proposal{Key: "x3", Time: 1, Cause: kernel.Cause{Actor: 5}, Rule: CapacityGatherRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 6, Component: CapacityBagTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityBagUnitsField, Value: sim.IntegerValue(1)}}}},
		{"produce-patching-an-actor", kernel.Proposal{Key: "x4", Time: 1, Cause: kernel.Cause{World: true}, Rule: CapacityProduceRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{inRange}}},
		{"gather-patching-the-shared-ledger", kernel.Proposal{Key: "x5", Time: 1, Cause: kernel.Cause{Actor: 1}, Rule: CapacityGatherRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 1001, Component: CapacityPatchTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityPatchPulsesField, Value: sim.IntegerValue(1)}}}},
		{"build-patching-the-granary", kernel.Proposal{Key: "x6", Time: 1, Cause: kernel.Cause{Actor: 1}, Rule: CapacityBuildRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 1, Component: CapacityGranaryTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityGranaryStockField, Value: sim.IntegerValue(1)}}}},
		{"wear-on-a-slot", kernel.Proposal{Key: "x7", Time: 1, Cause: kernel.Cause{World: true}, Rule: CapacityWearRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 2001, Component: CapacitySlotTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacitySlotStockField, Value: sim.IntegerValue(0)}}}},
		{"foreign-v1-component", kernel.Proposal{Key: "x8", Time: 1, Cause: kernel.Cause{Actor: 1}, Rule: CapacityConsumeRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 1, Component: FoodFlowBagTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowBagUnitsField, Value: sim.IntegerValue(1)}}}},
		{"unknown-v3-slot-entity", kernel.Proposal{Key: "x9", Time: 1, Cause: kernel.Cause{Actor: 1}, Rule: CapacityGatherRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 2017, Component: CapacitySlotTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacitySlotStockField, Value: sim.IntegerValue(0)}}}},
		{"foreign-actor-entity-range", kernel.Proposal{Key: "x10", Time: 1, Cause: kernel.Cause{Actor: 17}, Rule: CapacityWearRule, RuleVersion: CapacityRuleVersion, Patches: []component.Patch{{Entity: 17, Component: CapacityWorksiteTypeID, SchemaVersion: CapacitySchemaVersion, Field: CapacityWorksiteCapitalField, Value: sim.IntegerValue(0)}}}},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if err := CapacityCheckOwnership(bad.proposal); !errors.Is(err, ErrCapacityContract) {
				t.Fatalf("cross-owner or foreign mutation accepted: %v", err)
			}
		})
	}
	// Constructor eligibility mirrors the math layer exactly.
	empty := capacityTestActor()
	if _, err := CapacityBuildProposal("empty-bag", 0, 1, empty); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("empty-bag build constructed")
	}
	dup := gathered
	dup.Worksite = CapacityWorksiteState{Wip: 1, InvestedUnits: 1, LastBuildHour: 0}
	if _, err := CapacityBuildProposal("duplicate", 0, 1, dup); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("duplicate-hour build constructed")
	}
	dead := gathered
	dead.Body.Energy = 0
	dead.Body.BasalSpent = 11
	if _, err := CapacityBuildProposal("dead", 0, 1, dead); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("dead build constructed")
	}
	for _, hour := range []int{-1, 168} {
		if _, err := CapacityBuildProposal("hours", hour, 1, gathered); !errors.Is(err, ErrCapacityContract) {
			t.Fatalf("hour %d build constructed", hour)
		}
	}
	if _, err := CapacityGatherProposal("foreign-patch", 0, 9, 2001, CapacitySlotState{Stock: 1}, capacityTestActor()); !errors.Is(err, ErrCapacityContract) {
		t.Fatal("foreign-patch gather constructed")
	}
}

func TestCapacityOwnershipRejectsActorCausedProduce(t *testing.T) {
	// Report-only review fix: produce rows on shared patch/slot ledgers are
	// world-caused pulses; an actor-caused forged produce proposal must fail
	// the ownership guard even though the entity ranges match.
	actor := sim.EntityID(1)
	patch, _ := CapacityPatchID(0)
	slot, _ := CapacitySlotID(0, 0)
	p := kernel.Proposal{Rule: CapacityProduceRule, Cause: kernel.Cause{Actor: actor},
		Patches: []component.Patch{{Entity: patch, Component: CapacityPatchTypeID}}}
	if err := CapacityCheckOwnership(p); err == nil {
		t.Fatal("actor-caused patch produce accepted")
	}
	p = kernel.Proposal{Rule: CapacityProduceRule, Cause: kernel.Cause{Actor: actor},
		Patches: []component.Patch{{Entity: slot, Component: CapacitySlotTypeID}}}
	if err := CapacityCheckOwnership(p); err == nil {
		t.Fatal("actor-caused slot produce accepted")
	}
	world := kernel.Proposal{Rule: CapacityProduceRule, Cause: kernel.Cause{World: true},
		Patches: []component.Patch{
			{Entity: patch, Component: CapacityPatchTypeID},
			{Entity: slot, Component: CapacitySlotTypeID},
		}}
	if err := CapacityCheckOwnership(world); err != nil {
		t.Fatalf("world-caused produce rejected: %v", err)
	}
}
