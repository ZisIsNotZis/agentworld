package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// The ticket-17 mechanism-axis tests. The zero value (NaturalMechanismV5)
// must reproduce every frozen v5 descriptor, version field, and name byte
// for byte, and everything the v6 mechanism adds — rule 515 "provision",
// the granary provisioned field, the G2a′ identity, and the widened G5
// writer admissions — must exist only under NaturalMechanismV6.

func naturalV6ProvisionActor(t *testing.T, patches int, actor sim.EntityID) NaturalActorState {
	t.Helper()
	a, err := NaturalFounderState(patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	a.Granary = NaturalGranaryState{Stock: 3, CultivatedTotal: 3, LastStoredMealHour: NaturalNeverHour}
	if !validNaturalActorFor(NaturalMechanismV6, patches, actor, a) {
		t.Fatalf("v6 provision fixture invalid: %+v", a)
	}
	return a
}

// naturalMechanismSeeds builds the neutral genesis seeds of one mechanism:
// identical to naturalNeutralSeeds except that a v6 registry requires the
// provisioned field on every granary row (complete-row allocations).
func naturalMechanismSeeds(t *testing.T, m NaturalMechanism, patches int, R int64, lifetime int64) []component.ComponentSeed {
	t.Helper()
	seeds := naturalNeutralSeeds(patches, R, lifetime)
	if m != NaturalMechanismV6 {
		return seeds
	}
	for i := range seeds {
		if seeds[i].Component != NaturalGranaryTypeID {
			continue
		}
		seeds[i].Fields = append(seeds[i].Fields, component.FieldSeed{Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(0)})
	}
	return seeds
}

func TestNaturalMechanismAxisZeroValueReproducesV5(t *testing.T) {
	if NaturalMechanismV5 != 0 || NaturalMechanismV6 != 6 || NaturalMechanismV7 != 7 || NaturalProvisionRule != 515 || NaturalGranaryProvisionedField != 7 || NaturalProvisionedMax != 168 {
		t.Fatal("mechanism axis constants")
	}
	m0 := NaturalMechanismV5
	if m0.FormatVersion() != NaturalFormatVersion || m0.SchemaVersion() != NaturalSchemaVersion ||
		m0.RuleVersion() != NaturalRuleVersion || m0.ProjectionVersion() != NaturalProjectionVersion {
		t.Fatalf("zero-value version fields: %d %d %d %d", m0.FormatVersion(), m0.SchemaVersion(), m0.RuleVersion(), m0.ProjectionVersion())
	}
	if NaturalMechanismV6.FormatVersion() != 6 || NaturalMechanismV6.SchemaVersion() != 6 ||
		NaturalMechanismV6.RuleVersion() != 6 || NaturalMechanismV6.ProjectionVersion() != 6 {
		t.Fatal("v6 version fields")
	}
	if NaturalMechanismV7.FormatVersion() != 7 || NaturalMechanismV7.SchemaVersion() != 7 ||
		NaturalMechanismV7.RuleVersion() != 7 || NaturalMechanismV7.ProjectionVersion() != 7 {
		t.Fatal("v7 version fields")
	}
	// The admitted axis is derived, not a hand-maintained sample: for every
	// axis value the validity predicate must agree exactly with whether the
	// registry builds (valid ⇔ registry succeeds), and the uint16 ceiling is
	// explicitly unadmitted.
	for i := 1; i <= 65535; i++ {
		m := NaturalMechanism(i)
		valid := NaturalMechanismValid(m)
		_, err := NaturalRegistryFor(m)
		if valid != (err == nil) {
			t.Fatalf("mechanism %d: valid=%t registry=%v", m, valid, err)
		}
	}
	if NaturalMechanismValid(NaturalMechanism(65535)) {
		t.Fatal("mechanism 65535 admitted")
	}
	if !NaturalMechanismValid(NaturalMechanismV5) || !NaturalMechanismValid(NaturalMechanismV6) || !NaturalMechanismValid(NaturalMechanismV7) {
		t.Fatal("admitted mechanisms refused")
	}
	// The zero-value registry and every zero-value descriptor are the
	// frozen v5 bytes.
	frozen, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	zero, err := NaturalRegistryFor(NaturalMechanismV5)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(frozen.TypeIDs(), zero.TypeIDs()) {
		t.Fatal("zero-value registry type IDs")
	}
	for _, id := range frozen.TypeIDs() {
		a, err := frozen.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		b, err := zero.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("zero-value descriptor %d drifted", id)
		}
	}
	fpFrozen, err := component.RegistryFingerprint(frozen)
	if err != nil {
		t.Fatal(err)
	}
	fpZero, err := component.RegistryFingerprint(zero)
	if err != nil {
		t.Fatal(err)
	}
	if fpFrozen != fpZero {
		t.Fatal("zero-value registry fingerprint drifted from the frozen v5 fingerprint")
	}
}

func TestNaturalMechanismV6RegistryAdditiveIdentity(t *testing.T) {
	reg5, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	reg6, err := NaturalRegistryFor(NaturalMechanismV6)
	if err != nil {
		t.Fatal(err)
	}
	ids5 := reg5.TypeIDs()
	if !reflect.DeepEqual(ids5, reg6.TypeIDs()) || len(ids6(t, reg6)) != 8 {
		t.Fatal("v6 registry type IDs")
	}
	// No new component: the reserved 0x80000058-0x8000005A room stays free.
	for _, reserved := range []sim.ComponentTypeID{0x80000058, 0x80000059, 0x8000005A} {
		if _, err := reg6.Describe(reserved); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatalf("reserved type ID %d admitted: %v", reserved, err)
		}
	}
	granary5, err := reg5.Describe(NaturalGranaryTypeID)
	if err != nil {
		t.Fatal(err)
	}
	granary6, err := reg6.Describe(NaturalGranaryTypeID)
	if err != nil {
		t.Fatal(err)
	}
	if granary5.SchemaVersion != 5 || granary6.SchemaVersion != 6 || granary6.Name != "natural-granary-v6" {
		t.Fatalf("granary identity: %q %d", granary6.Name, granary6.SchemaVersion)
	}
	if len(granary6.Fields) != len(granary5.Fields)+1 {
		t.Fatalf("v6 granary fields: %d", len(granary6.Fields))
	}
	// The frozen six-field list is unchanged field for field.
	for i := range granary5.Fields {
		if !reflect.DeepEqual(granary5.Fields[i], granary6.Fields[i]) {
			t.Fatalf("frozen granary field %d drifted", granary5.Fields[i].ID)
		}
	}
	provisioned := granary6.Fields[6]
	if provisioned.ID != sim.FieldID(NaturalGranaryProvisionedField) || provisioned.Name != "provisioned" ||
		provisioned.Type.Kind != sim.IntegerKind || provisioned.Unit != "meals" ||
		!provisioned.Bounds.HasMinimum || !provisioned.Bounds.HasMaximum ||
		int64(provisioned.Bounds.Minimum) != 0 || int64(provisioned.Bounds.Maximum) != NaturalProvisionedMax {
		t.Fatalf("provisioned field descriptor: %+v", provisioned)
	}
	// Rules: the frozen four keep their IDs under the v6 version; rule 515
	// is additive with the new descriptor name suffix.
	wantRules := map[sim.RuleID]string{NaturalSpoilageRule: "natural-spoilage-v6", NaturalGrantRule: "natural-yield-grant-v6",
		NaturalEatStoredRule: "natural-eat-stored-v6", NaturalBirthReplyRule: "natural-birth-reply-v6", NaturalProvisionRule: "natural-provision-v6"}
	if len(granary6.TransitionRules) != len(wantRules) {
		t.Fatalf("v6 granary rules: %+v", granary6.TransitionRules)
	}
	for _, rule := range granary6.TransitionRules {
		name, ok := wantRules[rule.ID]
		if !ok || rule.Version != 6 || rule.Name != name {
			t.Fatalf("v6 granary rule: %+v", rule)
		}
	}
	// The provisioned field carries its own identity projection.
	projectionSeen := false
	for _, p := range granary6.Projections {
		if sim.FieldID(p.ID) != NaturalGranaryProvisionedField {
			continue
		}
		projectionSeen = true
		if p.Version != 6 || p.Name != "natural-granary-v6-provisioned" || !reflect.DeepEqual(p.SourceFields, []sim.FieldID{NaturalGranaryProvisionedField}) ||
			!reflect.DeepEqual(p.Coefficients, []float64{1}) || p.MissingBehavior != component.PreserveSourceState {
			t.Fatalf("provisioned projection: %+v", p)
		}
	}
	if !projectionSeen {
		t.Fatal("provisioned field has no identity projection")
	}
	// The bag admits 515; every other component keeps its exact frozen
	// field list and rule-ID set under the v6 versions and names.
	bag6, err := reg6.Describe(NaturalBagTypeID)
	if err != nil {
		t.Fatal(err)
	}
	bagRules := map[sim.RuleID]bool{}
	for _, rule := range bag6.TransitionRules {
		bagRules[rule.ID] = true
		if rule.Version != 6 {
			t.Fatalf("bag rule version: %+v", rule)
		}
	}
	for _, id := range []sim.RuleID{NaturalBuildRule, NaturalGatherRule, NaturalConsumeRule, NaturalBirthReplyRule, NaturalProvisionRule} {
		if !bagRules[id] {
			t.Fatalf("bag rule %d missing", id)
		}
	}
	for _, id := range []sim.ComponentTypeID{NaturalPatchTypeID, NaturalSlotTypeID, NaturalBodyTypeID, NaturalWorksiteTypeID, NaturalGenomeTypeID, NaturalBirthRequestTypeID} {
		d5, err := reg5.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		d6, err := reg6.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		if d6.SchemaVersion != 6 || d6.Name != d5.Name[:len(d5.Name)-2]+"v6" {
			t.Fatalf("component %d identity: %q %d", id, d6.Name, d6.SchemaVersion)
		}
		if !reflect.DeepEqual(d5.Fields, d6.Fields) {
			t.Fatalf("component %d fields drifted under v6", id)
		}
		rules5, rules6 := map[sim.RuleID]bool{}, map[sim.RuleID]bool{}
		for _, rule := range d5.TransitionRules {
			rules5[rule.ID] = true
		}
		for _, rule := range d6.TransitionRules {
			rules6[rule.ID] = true
			if rule.Version != 6 {
				t.Fatalf("component %d rule version: %+v", id, rule)
			}
		}
		if !reflect.DeepEqual(rules5, rules6) {
			t.Fatalf("component %d rule set drifted under v6", id)
		}
	}
	// The v6 rule table is exactly the frozen 501-514 plus 515.
	declared := map[sim.RuleID]bool{}
	for _, id := range reg6.TypeIDs() {
		d, err := reg6.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range d.TransitionRules {
			declared[rule.ID] = true
		}
	}
	for id := sim.RuleID(501); id <= 515; id++ {
		if !declared[id] {
			t.Fatalf("v6 rule %d not declared", id)
		}
	}
	if declared[500] || declared[516] {
		t.Fatal("v6 rule range")
	}
	// The registries are distinct bytes: cross-mechanism identities can
	// never collide.
	fp5, err := component.RegistryFingerprint(reg5)
	if err != nil {
		t.Fatal(err)
	}
	fp6, err := component.RegistryFingerprint(reg6)
	if err != nil {
		t.Fatal(err)
	}
	if fp5 == fp6 {
		t.Fatal("v5 and v6 registry fingerprints collide")
	}
}

func ids6(t *testing.T, reg component.Registry) []sim.ComponentTypeID {
	t.Helper()
	return reg.TypeIDs()
}

func TestNaturalOwnershipGuardV6WriterSet(t *testing.T) {
	patches := 2
	reg6, err := NaturalRegistryFor(NaturalMechanismV6)
	if err != nil {
		t.Fatal(err)
	}
	a := naturalV6ProvisionActor(t, patches, 1)
	proposal, err := NaturalProvisionProposal(NaturalMechanismV6, "provision/5/1", patches, 5, 1, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV6, proposal, patches); err != nil {
		t.Fatalf("v6 provision ownership: %v", err)
	}
	// The v5 zero value refuses every rule-515 proposal: the widened
	// writer set exists only under the new mechanism version.
	if err := NaturalCheckOwnership(proposal, patches); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("v5 ownership admitted rule 515: %v", err)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV5, proposal, patches); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("zero-value ownership admitted rule 515: %v", err)
	}
	// Rule version, cause, possession, and field-admission negatives.
	forged := func(mutate func(*kernel.Proposal)) kernel.Proposal {
		p := proposal
		p.Key = "forged"
		p.Patches = append([]component.Patch(nil), proposal.Patches...)
		mutate(&p)
		return p
	}
	negatives := []struct {
		name string
		p    kernel.Proposal
		m    NaturalMechanism
	}{
		{"foreign-rows", forged(func(p *kernel.Proposal) { p.Cause = kernel.Cause{Actor: 2} }), NaturalMechanismV6},
		{"world-cause", forged(func(p *kernel.Proposal) { p.Cause = kernel.Cause{World: true} }), NaturalMechanismV6},
		{"v5-rule-version", forged(func(p *kernel.Proposal) { p.RuleVersion = 5 }), NaturalMechanismV6},
		{"v6-rule-version-under-v5", forged(func(p *kernel.Proposal) {}), NaturalMechanismV5},
		{"granary-cultivated-total", forged(func(p *kernel.Proposal) { p.Patches[0].Field = NaturalGranaryCultivatedTotalField }), NaturalMechanismV6},
		{"granary-spoiled", forged(func(p *kernel.Proposal) { p.Patches[0].Field = NaturalGranarySpoiledField }), NaturalMechanismV6},
		{"bag-source", forged(func(p *kernel.Proposal) { p.Patches[3].Field = NaturalBagSourceField }), NaturalMechanismV6},
		{"body-energy", forged(func(p *kernel.Proposal) {
			p.Patches = append([]component.Patch(nil), p.Patches...)
			p.Patches[0] = component.Patch{Entity: 1, Component: NaturalBodyTypeID, SchemaVersion: 6, Field: NaturalBodyEnergyField, Value: sim.IntegerValue(9)}
		}), NaturalMechanismV6},
		{"slot-stock", forged(func(p *kernel.Proposal) {
			p.Patches = append([]component.Patch(nil), p.Patches...)
			p.Patches[0] = component.Patch{Entity: 2001, Component: NaturalSlotTypeID, SchemaVersion: 6, Field: NaturalSlotStockField, Value: sim.IntegerValue(0)}
		}), NaturalMechanismV6},
		{"granary-stock-under-v5", forged(func(p *kernel.Proposal) {}), NaturalMechanismV5},
		// A 515 proposal forged to look v5-versioned must still be refused by
		// the FIELD guard (not merely the version gate), so the v5 writer set
		// is proven closed at field level too.
		{"v5-versioned-515-field-write", forged(func(p *kernel.Proposal) { p.RuleVersion = 5 }), NaturalMechanismV5},
	}
	for _, negative := range negatives {
		if err := NaturalCheckOwnershipFor(negative.m, negative.p, patches); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s admitted: %v", negative.name, err)
		}
	}
	// Kernel schema/rule admission: rule 515 stages against the v6
	// registry only, with the v6 rule version, the v6 schema version, and
	// provisioned values inside the envelope.
	seeds6 := naturalMechanismSeeds(t, NaturalMechanismV6, patches, 8, 140)
	reader6, auth6, err := component.NewReader(reg6, 0, seeds6)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := component.Stage(reader6, auth6, 0, NaturalProvisionRule, 6, proposal.Patches, nil); err != nil {
		t.Fatalf("v6 stage: %v", err)
	}
	bounded := append([]component.Patch(nil), proposal.Patches...)
	bounded[1] = component.Patch{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: 6, Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(NaturalProvisionedMax + 1)}
	if _, _, _, _, err := component.Stage(reader6, auth6, 0, NaturalProvisionRule, 6, bounded, nil); !errors.Is(err, component.ErrValueOutsideSchema) {
		t.Fatalf("provisioned above the envelope staged: %v", err)
	}
	if _, _, _, _, err := component.Stage(reader6, auth6, 0, NaturalProvisionRule, 5, proposal.Patches, nil); !errors.Is(err, component.ErrUnknownRule) {
		t.Fatalf("515 under the v5 rule version staged: %v", err)
	}
	v5Schema := append([]component.Patch(nil), proposal.Patches...)
	v5Schema[0].SchemaVersion = NaturalSchemaVersion
	if _, _, _, _, err := component.Stage(reader6, auth6, 0, NaturalProvisionRule, 6, v5Schema, nil); !errors.Is(err, component.ErrSchemaVersion) {
		t.Fatalf("v5-schema patch staged against the v6 registry: %v", err)
	}
	reg5, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	reader5, auth5, err := component.NewReader(reg5, 0, naturalNeutralSeeds(patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	v5Shape := append([]component.Patch(nil), proposal.Patches...)
	for i := range v5Shape {
		v5Shape[i].SchemaVersion = NaturalSchemaVersion
	}
	if _, _, _, _, err := component.Stage(reader5, auth5, 0, NaturalProvisionRule, 5, v5Shape, nil); !errors.Is(err, component.ErrUnknownRule) {
		t.Fatalf("515 staged against the v5 registry: %v", err)
	}
}

func TestNaturalProvisionMathEffectsConservationAndNegatives(t *testing.T) {
	patches := 2
	before := naturalV6ProvisionActor(t, patches, 1)
	frozenInput := before
	after, err := NaturalProvision(NaturalMechanismV6, patches, 5, 1, before)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, frozenInput) {
		t.Fatal("provision mutated its input")
	}
	// Effects: stock−1, provisioned+1, LastStoredMealHour = hour, bag+1;
	// the body and the cultivated-received ledger are untouched.
	if after.Granary.Stock != 2 || after.Granary.Provisioned != 1 || after.Granary.LastStoredMealHour != 5 || after.Bag.Units != 1 {
		t.Fatalf("provision effects: %+v", after.Granary)
	}
	if after.Granary.CultivatedTotal != 3 || after.Body.Energy != NaturalFounderInitialEnergy || after.Body.Hunger != 0 ||
		after.Body.Consumed != 0 || after.Body.BasalSpent != 0 {
		t.Fatalf("provision touched the body or the received ledger: %+v", after)
	}
	// Exact per-draw conservation: stock−1 = bag+1, one unit transferred.
	if before.Granary.Stock-after.Granary.Stock != after.Bag.Units-before.Bag.Units || before.Granary.Stock-after.Granary.Stock != 1 {
		t.Fatal("per-draw conservation broken")
	}
	// G2a′ on the after-state.
	if after.Granary.CultivatedTotal != after.Granary.Stock+after.Granary.StoredMeals+after.Granary.Provisioned+after.Granary.Spoiled+after.Genome.BirthGranaryPaid {
		t.Fatal("G2a′ broken")
	}
	if !validNaturalActorFor(NaturalMechanismV6, patches, 1, after) {
		t.Fatal("after-state invalid under v6")
	}
	// Next hour — after the bag decision consumed the provisioned unit
	// (G3-exact: 11 base + 1 consumed = 12 energy) — the guard releases.
	after.Bag.Units, after.Body.Consumed, after.Body.Energy = 0, 1, 12
	next, err := NaturalProvision(NaturalMechanismV6, patches, 6, 1, after)
	if err != nil || next.Granary.Provisioned != 2 || next.Granary.Stock != 1 || next.Bag.Units != 1 {
		t.Fatalf("provision at hour 6: %+v %v", next, err)
	}
	// Negatives, each refused with zero mutation of its input.
	negative := func(name string, m NaturalMechanism, hour int, a NaturalActorState) {
		t.Helper()
		frozen := a
		got, err := NaturalProvision(m, patches, hour, 1, a)
		if !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s admitted: %+v %v", name, got, err)
		}
		if got != (NaturalActorState{}) {
			t.Fatalf("%s mutated on refusal", name)
		}
		if !reflect.DeepEqual(a, frozen) {
			t.Fatalf("%s mutated its input", name)
		}
	}
	negative("zero-value mechanism", NaturalMechanismV5, 5, naturalV6ProvisionActor(t, patches, 1))
	negative("literal v5 spelling", NaturalMechanism(5), 5, naturalV6ProvisionActor(t, patches, 1))
	negative("unknown mechanism", NaturalMechanism(7), 5, naturalV6ProvisionActor(t, patches, 1))
	withBag := naturalV6ProvisionActor(t, patches, 1)
	withBag.Bag.Units, withBag.Body.LastGatherHour, withBag.Body.Consumed = 1, 3, 1
	negative("bag held", NaturalMechanismV6, 5, withBag)
	empty := naturalV6ProvisionActor(t, patches, 1)
	empty.Granary = NaturalGranaryState{LastStoredMealHour: NaturalNeverHour}
	negative("stock 0", NaturalMechanismV6, 5, empty)
	dead := naturalV6ProvisionActor(t, patches, 1)
	dead.Genome.DiedHour = 4
	negative("dead owner", NaturalMechanismV6, 5, dead)
	negative("hour -1", NaturalMechanismV6, -1, naturalV6ProvisionActor(t, patches, 1))
	negative("hour 168", NaturalMechanismV6, NaturalHorizonHours, naturalV6ProvisionActor(t, patches, 1))
	// Forged double draw in one hour: after a stored meal, and after a
	// provision, the shared LastStoredMealHour guard refuses the same-hour
	// provision.
	afterMeal, _, err := NaturalEatStoredFor(NaturalMechanismV6, patches, 5, 1, naturalV6ProvisionActor(t, patches, 1))
	if err != nil {
		t.Fatal(err)
	}
	negative("double draw after stored meal", NaturalMechanismV6, 5, afterMeal)
	afterProvision, err := NaturalProvision(NaturalMechanismV6, patches, 5, 1, naturalV6ProvisionActor(t, patches, 1))
	if err != nil {
		t.Fatal(err)
	}
	negative("double provision", NaturalMechanismV6, 5, afterProvision)
	// The v6 contract enforces the shared guard on the stored-meal draw
	// too; the frozen v5 contract keeps the guard runner-owned: two
	// same-hour stored-meal draws stay contract-legal under the zero value
	// on a v5-valid state (the runner's naturalPairIntent and
	// naturalPairedMealGate own the discipline there).
	if _, _, err := NaturalEatStoredFor(NaturalMechanismV6, patches, 5, 1, afterProvision); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v6 stored-meal draw after a provision admitted")
	}
	plain := naturalV6ProvisionActor(t, patches, 1)
	firstDraw, _, err := NaturalEatStoredFor(NaturalMechanismV5, patches, 5, 1, plain)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := NaturalEatStoredFor(NaturalMechanismV5, patches, 5, 1, firstDraw); err != nil {
		t.Fatalf("v5 eat-stored semantics drifted: %v", err)
	}
	if _, _, err := NaturalEatStoredFor(NaturalMechanismV6, patches, 5, 1, firstDraw); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v6 stored-meal double draw admitted")
	}
	// Ledger envelope: the provisioned ledger at its horizon maximum
	// refuses a further draw (G2a′-consistent forged state).
	atMax := naturalV6ProvisionActor(t, patches, 1)
	atMax.Granary = NaturalGranaryState{Stock: 1, CultivatedTotal: NaturalProvisionedMax + 1, Provisioned: NaturalProvisionedMax, LastStoredMealHour: NaturalNeverHour}
	negative("provisioned at the envelope", NaturalMechanismV6, 5, atMax)
	// Tampered input: a provisioned count the received ledger never
	// backed is refused before any effect.
	tampered := naturalV6ProvisionActor(t, patches, 1)
	tampered.Granary.Provisioned = 1
	negative("tampered provisioned ledger", NaturalMechanismV6, 5, tampered)
}

func TestNaturalProvisionProposalShapeAndMechanismRefusals(t *testing.T) {
	patches := 2
	a := naturalV6ProvisionActor(t, patches, 1)
	proposal, err := NaturalProvisionProposal(NaturalMechanismV6, "provision/5/1", patches, 5, 1, a)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Time != mustNaturalPhase(t, 5, NaturalPhaseClaim) || proposal.Rule != NaturalProvisionRule ||
		proposal.RuleVersion != 6 || proposal.Cause.Actor != 1 || proposal.Cause.World {
		t.Fatalf("proposal identity: %+v", proposal)
	}
	want := []component.Patch{
		{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: 6, Field: NaturalGranaryStockField, Value: sim.IntegerValue(2)},
		{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: 6, Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(1)},
		{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: 6, Field: NaturalGranaryLastStoredMealHourField, Value: sim.IntegerValue(5)},
		{Entity: 1, Component: NaturalBagTypeID, SchemaVersion: 6, Field: NaturalBagUnitsField, Value: sim.IntegerValue(1)},
	}
	if !reflect.DeepEqual(proposal.Patches, want) {
		t.Fatalf("proposal patches: %+v", proposal.Patches)
	}
	if len(proposal.Allocations) != 0 {
		t.Fatal("provision allocates nothing")
	}
	// Under the v5 zero value the proposal does not exist.
	if _, err := NaturalProvisionProposal(NaturalMechanismV5, "provision/5/1", patches, 5, 1, a); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 provision proposal admitted")
	}
	if _, err := NaturalProvisionProposal(NaturalMechanism(7), "provision/5/1", patches, 5, 1, a); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("unknown-mechanism provision proposal admitted")
	}
	// End to end through the kernel under the v6 registry.
	reg6, err := NaturalRegistryFor(NaturalMechanismV6)
	if err != nil {
		t.Fatal(err)
	}
	k6, err := kernel.New(reg6, 0, naturalMechanismSeeds(t, NaturalMechanismV6, patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ := k6.Snapshot()
	plan, err := k6.Plan(proposal, authority)
	if err != nil {
		t.Fatalf("v6 plan: %v", err)
	}
	if _, err := k6.CommitBatch([]kernel.Plan{plan}); err != nil {
		t.Fatalf("v6 commit: %v", err)
	}
	reader, auth, version := k6.Snapshot()
	view, err := reader.Read(component.ReadRequest{Entity: 1, Component: NaturalGranaryTypeID,
		Fields: []sim.FieldID{NaturalGranaryStockField, NaturalGranaryProvisionedField, NaturalGranaryLastStoredMealHourField}, WorldVersion: version, Authority: auth})
	if err != nil {
		t.Fatal(err)
	}
	value, err := view.Value(NaturalGranaryProvisionedField)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := value.Integer()
	if err != nil || committed != 1 {
		t.Fatalf("committed provisioned ledger: %d %v", committed, err)
	}
	// The same proposal plans against nothing else: a v5 kernel refuses
	// the unknown rule, and a v5-schema proposal refuses the v6 kernel.
	reg5, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	k5, err := kernel.New(reg5, 0, naturalNeutralSeeds(patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	_, authority5, _ := k5.Snapshot()
	if _, err := k5.Plan(proposal, authority5); err == nil {
		t.Fatal("v5 kernel planned a rule-515 proposal")
	}
	v5Shape, err := NaturalEatStoredProposal("eat/5/1", patches, 5, mustNaturalPhase(t, 5, NaturalPhaseClaim), 1, naturalV6ProvisionActor(t, patches, 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k6.Plan(v5Shape, authority); err == nil {
		t.Fatal("v6 kernel planned a v5-schema proposal")
	}
}

func TestNaturalG2aPrimeVerificationAndCrossMechanismRefusals(t *testing.T) {
	patches := 2
	consistent := naturalV6ProvisionActor(t, patches, 1)
	consistent.Granary = NaturalGranaryState{Stock: 2, CultivatedTotal: 3, Provisioned: 1, LastStoredMealHour: NaturalNeverHour}
	if !validNaturalActorFor(NaturalMechanismV6, patches, 1, consistent) {
		t.Fatal("G2a′-consistent provisioned state refused")
	}
	// The frozen v5 validator refuses every provisioned≠0 state: the field
	// does not exist under the zero value.
	if validNaturalActor(patches, 1, consistent) {
		t.Fatal("v5 validator admitted a provisioned state")
	}
	tampered := consistent
	tampered.Granary.Provisioned = 2
	if validNaturalActorFor(NaturalMechanismV6, patches, 1, tampered) {
		t.Fatal("forged provisioned count admitted")
	}
	aboveEnvelope := consistent
	aboveEnvelope.Granary.Provisioned = NaturalProvisionedMax + 1
	if validNaturalActorFor(NaturalMechanismV6, patches, 1, aboveEnvelope) {
		t.Fatal("provisioned above the envelope admitted")
	}
	// Scripted world with one provision drawn from the gate-fixture
	// grants: conserved under the v6 gate, refused by the frozen v5 gate,
	// and a forged count is refused under both.
	type scriptedWorld struct {
		patches  []NaturalPatchState
		slots    [][]NaturalSlotState
		founders []NaturalActorState
	}
	buildWorld := func() scriptedWorld {
		patchStates, slotStates, founders := naturalGateFixtureWorld(t, patches)
		// The proposer drew one granted unit into its bag (rule 515): the
		// bag-intake identity balances as gathered(12) + provisioned(1) =
		// consumed(0) + held(1) + invested(12).
		founders[0].Granary = NaturalGranaryState{Stock: 4, CultivatedTotal: 5, Provisioned: 1, LastStoredMealHour: 3}
		founders[0].Bag.Units = 1
		return scriptedWorld{patchStates, slotStates, founders}
	}
	world := buildWorld()
	balance, err := NaturalCheckConservationFor(NaturalMechanismV6, patches, world.patches, world.slots, world.founders, nil, 2016)
	if err != nil {
		t.Fatalf("v6 conservation: %v", err)
	}
	if balance.Provisioned != 1 {
		t.Fatalf("balance provisioned: %d", balance.Provisioned)
	}
	if _, err := NaturalCheckConservation(patches, world.patches, world.slots, world.founders, nil, 2016); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 conservation admitted a provisioned world")
	}
	tamperedWorld := buildWorld()
	tamperedWorld.founders[0].Granary.Provisioned = 2
	if _, err := NaturalCheckConservationFor(NaturalMechanismV6, patches, tamperedWorld.patches, tamperedWorld.slots, tamperedWorld.founders, nil, 2016); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v6 conservation admitted a forged provisioned count")
	}
	// Cross-mechanism restore refusal both ways at the contract seam: a
	// bundle is only decodable against its own mechanism's registry
	// (fingerprint), so v5 bytes never decode as v6 and vice versa.
	reg5, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	reg6, err := NaturalRegistryFor(NaturalMechanismV6)
	if err != nil {
		t.Fatal(err)
	}
	reader5, auth5, err := component.NewReader(reg5, 0, naturalNeutralSeeds(patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	bundle5, err := component.EncodeSnapshot(reg5, reader5, auth5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := component.DecodeSnapshot(reg5, bundle5); err != nil {
		t.Fatalf("v5 bundle under the v5 registry: %v", err)
	}
	if _, _, _, err := component.DecodeSnapshot(reg6, bundle5); err == nil {
		t.Fatal("v5 bundle restored under the v6 registry")
	}
	reader6, auth6, err := component.NewReader(reg6, 0, naturalMechanismSeeds(t, NaturalMechanismV6, patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	bundle6, err := component.EncodeSnapshot(reg6, reader6, auth6, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := component.DecodeSnapshot(reg6, bundle6); err != nil {
		t.Fatalf("v6 bundle under the v6 registry: %v", err)
	}
	if _, _, _, err := component.DecodeSnapshot(reg5, bundle6); err == nil {
		t.Fatal("v6 bundle restored under the v5 registry")
	}
}

func TestNaturalV6ThreadingThroughSharedRules(t *testing.T) {
	patches := 2
	// A provisioned actor flows through every shared rule under the v6
	// mechanism while the zero value refuses the same states.
	provisioned, err := NaturalProvision(NaturalMechanismV6, patches, 5, 1, naturalV6ProvisionActor(t, patches, 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := NaturalEatStoredFor(NaturalMechanismV6, patches, 6, 1, provisioned); err != nil {
		t.Fatalf("v6 eat-stored after a provisioned hour: %v", err)
	}
	if validNaturalActor(patches, 1, provisioned) {
		t.Fatal("v5 path admitted a provisioned actor")
	}
	if _, _, err := NaturalEatStored(patches, 6, 1, provisioned); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("v5 eat-stored admitted a provisioned actor: %v", err)
	}
	granted, _, err := NaturalCultivatedGrantFor(NaturalMechanismV6, patches, 1, provisioned, 1)
	if err != nil || granted.Granary.Stock != 3 || granted.Granary.Provisioned != 1 || granted.Granary.CultivatedTotal != 4 {
		t.Fatalf("v6 grant on a provisioned granary: %+v %v", granted, err)
	}
	if _, _, err := NaturalCultivatedGrant(patches, 1, provisioned, 1); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 grant admitted a provisioned actor")
	}
	// The birth path under v6: a provisioned parent pays the frozen costs
	// and the newborn's granary row carries the provisioned field at zero.
	proposer := naturalGateParent(t, patches, 1)
	proposer.Granary = NaturalGranaryState{Stock: 4, CultivatedTotal: 5, Provisioned: 1, LastStoredMealHour: NaturalNeverHour}
	consenter := naturalGateParent(t, patches, 2)
	request, err := NaturalBirthRequestFor(NaturalMechanismV6, patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	draws := naturalDrawsForSeed(t, 0)
	result, err := NaturalBirthReplyFor(NaturalMechanismV6, patches, 5, true, draws, 2017, 1, 2, request, consenter)
	if err != nil || result.Decision != NaturalReplyAccept {
		t.Fatalf("v6 birth reply: %+v %v", result.Decision, err)
	}
	if result.Proposer.Granary.Stock != 2 || result.Proposer.Granary.Provisioned != 1 || result.Proposer.Genome.BirthGranaryPaid != 2 {
		t.Fatalf("provisioned parent after paying: %+v", result.Proposer.Granary)
	}
	if result.Newborn.Granary.Provisioned != 0 {
		t.Fatalf("newborn provisioned ledger: %+v", result.Newborn.Granary)
	}
	// The same flow refuses under the zero value, and the v6 reply
	// proposal carries the seven-field newborn granary row.
	if _, err := NaturalBirthReply(patches, 5, true, draws, 2017, 1, 2, request, consenter); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 birth reply admitted a provisioned parent")
	}
	reply, err := NaturalBirthReplyProposalFor(NaturalMechanismV6, "reply/5/2", patches, 5, 1, 2, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Allocations) != 6 {
		t.Fatalf("newborn rows: %d", len(reply.Allocations))
	}
	granaryFields := 0
	for _, seed := range reply.Allocations {
		if seed.Component == NaturalGranaryTypeID {
			granaryFields = len(seed.Fields)
		}
	}
	if granaryFields != 7 {
		t.Fatalf("v6 newborn granary row: %d fields", granaryFields)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV6, reply, patches); err != nil {
		t.Fatalf("v6 birth-reply ownership: %v", err)
	}
	// The v5 wrapper builds the frozen six-field row for a provisioned-free
	// result and refuses the mechanism axis directly.
	v5Result, err := NaturalBirthReply(patches, 5, true, draws, 2017, 1, 2,
		mustNaturalGateRequest(t, patches), consenter)
	if err != nil || v5Result.Decision != NaturalReplyAccept {
		t.Fatalf("v5 birth reply: %+v %v", v5Result.Decision, err)
	}
	v5Reply, err := NaturalBirthReplyProposal("reply/5/2", patches, 5, 1, 2, v5Result)
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range v5Reply.Allocations {
		if seed.Component == NaturalGranaryTypeID && len(seed.Fields) != 6 {
			t.Fatalf("v5 newborn granary row: %d fields", len(seed.Fields))
		}
	}
}

// mustNaturalGateRequest files the gate-parent request through the frozen
// v5 path for the v5 wrapper comparison.
func mustNaturalGateRequest(t *testing.T, patches int) NaturalActorState {
	t.Helper()
	proposer := naturalGateParent(t, patches, 1)
	request, err := NaturalBirthRequest(patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

// Review P1: the zero-value ≡ v5 assertion must not be circular.
// NaturalRegistry() IS NaturalRegistryFor(NaturalMechanismV5), so comparing the
// two proves nothing about the shared builders. These literals were recorded
// from the pre-ticket-17 revision (2e67e5f) by reading the frozen v5 registry
// directly, so any drift in the v5 descriptor names, rule names/versions,
// schema versions or field names now fails loudly instead of silently breaking
// byte-identity with the recorded ticket-15/16 evidence.
const naturalGoldenV5RegistryFingerprint = "81d28abd78e061c84bde3c364fcabcc35ac9d526bebc1449ba3d8942ab1cba2f"

func TestNaturalV5RegistryMatchesPreTicket17Golden(t *testing.T) {
	reg, err := NaturalRegistryFor(NaturalMechanismV5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := component.RegistryFingerprint(reg)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(got[:]) != naturalGoldenV5RegistryFingerprint {
		t.Fatalf("v5 registry fingerprint drifted: %s", hex.EncodeToString(got[:]))
	}
	type golden struct {
		name   string
		schema sim.SchemaVersion
		rules  []string
		fields []string
	}
	want := map[sim.ComponentTypeID]golden{
		0x80000050: {"natural-patch-v5", 5, []string{"501:natural-regen-split-v5:v5"}, []string{"1:yield", "2:pulses", "3:produced", "4:granted", "5:unrealized"}},
		0x80000051: {"natural-slot-v5", 5, []string{"501:natural-regen-split-v5:v5", "503:natural-wear-v5:v5", "505:natural-build-v5:v5", "506:natural-gather-v5:v5"}, []string{"1:stock", "2:patch", "3:gathered", "4:tenure-mode", "5:tenure-owner", "6:tenure-cultivated"}},
		0x80000052: {"natural-bag-v5", 5, []string{"505:natural-build-v5:v5", "506:natural-gather-v5:v5", "507:natural-consume-v5:v5", "511:natural-birth-reply-v5:v5"}, []string{"1:units", "2:home-patch"}},
		0x80000053: {"natural-body-v5", 5, []string{"504:natural-basal-v5:v5", "506:natural-gather-v5:v5", "507:natural-consume-v5:v5", "509:natural-eat-stored-v5:v5", "511:natural-birth-reply-v5:v5"}, []string{"1:energy", "2:hunger", "3:basal-spent", "4:cap-lost", "5:consumed-home-patch", "6:last-gather-hour", "7:basal-debt"}},
		0x80000054: {"natural-worksite-v5", 5, []string{"503:natural-wear-v5:v5", "505:natural-build-v5:v5", "511:natural-birth-reply-v5:v5"}, []string{"1:capital", "2:wip", "3:wear-debt", "4:invested-units", "5:points-created", "6:points-decayed", "7:last-build-hour"}},
		0x80000055: {"natural-granary-v5", 5, []string{"502:natural-spoilage-v5:v5", "508:natural-yield-grant-v5:v5", "509:natural-eat-stored-v5:v5", "511:natural-birth-reply-v5:v5"}, []string{"1:stock", "2:cultivated-total", "3:stored-meals", "4:last-stored-meal-hour", "5:spoil-debt", "6:spoiled"}},
		0x80000056: {"natural-genome-v5", 5, []string{"504:natural-basal-v5:v5", "511:natural-birth-reply-v5:v5", "514:natural-age-death-v5:v5"}, []string{"1:locus-m", "2:locus-y", "3:locus-w", "4:parent-a", "5:parent-b", "6:birth-hour", "7:death-hour", "8:died-hour", "9:birth-granary-paid"}},
		0x80000057: {"natural-birthrequest-v5", 5, []string{"510:natural-birth-request-v5:v5", "511:natural-birth-reply-v5:v5", "512:natural-birth-refusal-v5:v5", "513:natural-birth-expiry-v5:v5"}, []string{"1:hour", "2:addressee", "3:reported-capital", "4:reported-granary", "5:status", "6:last-request-hour", "7:last-reply-hour"}},
	}
	for id, g := range want {
		d, err := reg.Describe(id)
		if err != nil {
			t.Fatalf("%#x: %v", uint32(id), err)
		}
		if d.Name != g.name || d.SchemaVersion != g.schema {
			t.Fatalf("%#x: name/schema %q/%d, want %q/%d", uint32(id), d.Name, d.SchemaVersion, g.name, g.schema)
		}
		var rules []string
		for _, r := range d.TransitionRules {
			rules = append(rules, fmt.Sprintf("%d:%s:v%d", r.ID, r.Name, r.Version))
		}
		sort.Strings(rules)
		if !reflect.DeepEqual(rules, g.rules) {
			t.Fatalf("%#x rules %v, want %v", uint32(id), rules, g.rules)
		}
		var fields []string
		for _, f := range d.Fields {
			fields = append(fields, fmt.Sprintf("%d:%s", f.ID, f.Name))
		}
		if !reflect.DeepEqual(fields, g.fields) {
			t.Fatalf("%#x fields %v, want %v", uint32(id), fields, g.fields)
		}
	}
}
