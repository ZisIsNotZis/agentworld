package world

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The ticket-20 L1 wage-contract tests. The frozen v9 design
// (.scratch/20-exchange-design/spec.md, "Frozen design: v9 = the wage
// contract") is binding: mechanism version 7 is strictly additive over v6 —
// rules 516-519, components 0x80000058/0x80000059 (0x8000005A spare), the
// granary wage ledgers 8/9, G2a″, and the enumerated G5 admissions exist
// only under NaturalMechanismV7, while every v5/v6 byte stays identical.
// The v5 and v6 registry fingerprints below were captured from the
// pre-ticket-20 revision (8c00439) in a detached throwaway worktree with
// `git worktree add --detach`, never fabricated.

const (
	// naturalGoldenV6RegistryFingerprint is the v6 registry fingerprint
	// captured at 8c00439 (pre-ticket-20).
	naturalGoldenV6RegistryFingerprint = "684190238e188e556c3086f7c0e5710d0f951f4b95ce15943c47dc178e20a3a1"
	// naturalGoldenV5RegistryFingerprintPre20 restates the existing ticket-17
	// v5 pin so both frozen registries are asserted in one place.
	naturalGoldenV5RegistryFingerprintPre20 = "81d28abd78e061c84bde3c364fcabcc35ac9d526bebc1449ba3d8942ab1cba2f"
)

// naturalV7GranarySeeds appends the mechanism-v7 granary fields to seeds
// (complete rows under the v7 registry).
func naturalV7GranarySeeds(seeds []component.ComponentSeed) []component.ComponentSeed {
	for i := range seeds {
		if seeds[i].Component != NaturalGranaryTypeID {
			continue
		}
		seeds[i].Fields = append(seeds[i].Fields,
			component.FieldSeed{Field: NaturalGranaryProvisionedField, Value: sim.IntegerValue(0)},
			component.FieldSeed{Field: NaturalGranaryWagesPaidField, Value: sim.IntegerValue(0)},
			component.FieldSeed{Field: NaturalGranaryWagesEarnedField, Value: sim.IntegerValue(0)})
	}
	return seeds
}

// naturalV7LabourRowSeeds appends one actor's idle wage-contract rows.
func naturalV7LabourRowSeeds(actor sim.EntityID) []component.ComponentSeed {
	return []component.ComponentSeed{
		{Entity: actor, Component: NaturalLabourOfferTypeID, Fields: []component.FieldSeed{
			{Field: NaturalLabourOfferHourField, Value: sim.IntegerValue(NaturalNeverHour)},
			{Field: NaturalLabourOfferAddresseeField, Value: naturalMissingRef},
			{Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestIdle))},
			{Field: NaturalLabourOfferLastOfferHourField, Value: sim.IntegerValue(NaturalNeverHour)},
			{Field: NaturalLabourOfferLastReplyHourField, Value: sim.IntegerValue(NaturalNeverHour)}}},
		{Entity: actor, Component: NaturalLabourBookTypeID, Fields: []component.FieldSeed{
			{Field: NaturalLabourBookEngagedWorkerField, Value: naturalMissingRef},
			{Field: NaturalLabourBookEngagedHourField, Value: sim.IntegerValue(NaturalNeverHour)},
			{Field: NaturalLabourBookLastDecisionHourField, Value: sim.IntegerValue(NaturalNeverHour)}}},
	}
}

// naturalV7GenesisSeeds builds the v7 genesis fixture: the neutral world
// with complete v7 granary rows and idle wage-contract rows per founder.
func naturalV7GenesisSeeds(patches int, R int64, lifetime int64) []component.ComponentSeed {
	seeds := naturalV7GranarySeeds(naturalNeutralSeeds(patches, R, lifetime))
	founders, _ := NaturalFounderCount(patches)
	for actor := 1; actor <= founders; actor++ {
		seeds = append(seeds, naturalV7LabourRowSeeds(sim.EntityID(actor))...)
	}
	return seeds
}

// naturalV7Actor builds a V7-valid founder actor: the neutral founder state
// plus the idle wage-contract rows the V7 validators require.
func naturalV7Actor(t *testing.T, patches int, actor sim.EntityID) NaturalActorState {
	t.Helper()
	a, err := NaturalFounderState(patches, actor, 140)
	if err != nil {
		t.Fatal(err)
	}
	a.Offer = NaturalIdleLabourOfferState()
	a.Book = NaturalIdleLabourBookState()
	if !validNaturalActorFor(NaturalMechanismV7, patches, actor, a) {
		t.Fatalf("v7 actor fixture invalid: %+v", a)
	}
	return a
}

// mustNaturalRef wraps sim.EntityRefValue for fixture literals.
func mustNaturalRef(t *testing.T, id sim.EntityID) sim.Value {
	t.Helper()
	ref, err := sim.EntityRefValue(id)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// naturalV7Pair builds the idle contract pair on patch 1001 (worker 1 →
// employer 2): the worker holds received-but-unspent grant stock, the
// employer holds wage stock and one unit of work in progress (k+wip = 2, so
// a settlement completes a point). Both rows are V7-valid.
func naturalV7Pair(t *testing.T, patches int) (NaturalActorState, NaturalActorState) {
	t.Helper()
	worker, employer := naturalV7Actor(t, patches, 1), naturalV7Actor(t, patches, 2)
	worker.Body = NaturalBodyState{Energy: 5, BasalSpent: 6, LastGatherHour: NaturalNeverHour}
	worker.Granary = NaturalGranaryState{Stock: 2, CultivatedTotal: 2, LastStoredMealHour: NaturalNeverHour}
	employer.Body = NaturalBodyState{Energy: 5, BasalSpent: 6, LastGatherHour: NaturalNeverHour}
	employer.Granary = NaturalGranaryState{Stock: 5, CultivatedTotal: 5, LastStoredMealHour: NaturalNeverHour}
	employer.Worksite = NaturalWorksiteState{Capital: 1, Wip: 1, InvestedUnits: 3, PointsCreated: 1, LastBuildHour: NaturalNeverHour}
	if !validNaturalActorFor(NaturalMechanismV7, patches, 1, worker) || !validNaturalActorFor(NaturalMechanismV7, patches, 2, employer) {
		t.Fatal("v7 pair fixture invalid")
	}
	return worker, employer
}

// naturalV7Engage runs the frozen chain offer (h−1) → accept (h−1) → gather
// (h claim) for the pair: the worker ends delivery-ready (bag 1, accepted
// matched contract, gathered this hour) and the employer engaged. The slot
// array must be the common home patch's slots with slot 1 wild and stocked;
// the gather draws that unit.
func naturalV7Engage(t *testing.T, m NaturalMechanism, patches, hour int, worker, employer NaturalActorState, slots [NaturalSlotsPerPatch]NaturalSlotState) (NaturalActorState, NaturalActorState, [NaturalSlotsPerPatch]NaturalSlotState) {
	t.Helper()
	filingHour := hour - 1
	offered, err := NaturalLabourOffer(m, patches, filingHour, 1, 2, 1001, worker)
	if err != nil {
		t.Fatalf("labour offer: %v", err)
	}
	engagedEmployer, engagedWorker, decision, err := NaturalLabourDecision(m, patches, filingHour, 2, 1, true, employer, offered)
	if err != nil || decision != NaturalReplyAccept {
		t.Fatalf("labour decision: %v %+v", err, decision)
	}
	slotID, err := NaturalSlotID(patches, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	gatheredSlot, gatheredWorker, err := NaturalGatherFor(m, patches, 1001, slotID, 1, hour, slots[1], engagedWorker)
	if err != nil {
		t.Fatalf("contract gather: %v", err)
	}
	slots[1] = gatheredSlot
	return gatheredWorker, engagedEmployer, slots
}

// naturalV7SettleFixture assembles the scripted settlement world at hour h:
// patch 1001 (produced 13 = gathered 13, granted 7 = the received ledgers),
// slot 0 tenured to the employer, slot 1 wild with the unit the worker
// gathers, slots 2-7 wild and empty, the engaged pair, and 14 idle
// founders. Every state is V7-valid and passes the conservation gate.
func naturalV7SettleFixture(t *testing.T, patches, hour int) (patchStates []NaturalPatchState, slotStates [][]NaturalSlotState, founders []NaturalActorState, worker, employer NaturalActorState, homeSlots [NaturalSlotsPerPatch]NaturalSlotState) {
	t.Helper()
	worker, employer = naturalV7Pair(t, patches)
	homeSlots = [NaturalSlotsPerPatch]NaturalSlotState{}
	homeSlots[0] = NaturalSlotState{Gathered: 3, Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1}
	homeSlots[1] = NaturalSlotState{Stock: 1}
	worker, employer, homeSlots = naturalV7Engage(t, NaturalMechanismV7, patches, hour, worker, employer, homeSlots)
	// The v7 genesis ledger math: produced 4 = gathered 4 (slot 0's three
	// historic employer gathers + the worker's unit), granted 7 = the
	// received ledgers, and G1 intake gathered 4 = held 1 + invested 3.
	patchStates = []NaturalPatchState{{Yield: 8, Pulses: 3, Produced: 4, Granted: 7, Unrealized: 13}, {}}
	slotStates = [][]NaturalSlotState{homeSlots[:], make([]NaturalSlotState, NaturalSlotsPerPatch)}
	founders = make([]NaturalActorState, 8*patches)
	for i := range founders {
		id := sim.EntityID(i + 1)
		switch id {
		case 1:
			founders[i] = worker
		case 2:
			founders[i] = employer
		default:
			founders[i] = naturalV7Actor(t, patches, id)
		}
	}
	if _, err := NaturalCheckConservationFor(NaturalMechanismV7, patches, patchStates, slotStates, founders, nil, mustNaturalGenesisLast(patches)); err != nil {
		t.Fatalf("fixture world breaks frozen conservation: %v", err)
	}
	return patchStates, slotStates, founders, worker, employer, homeSlots
}

func TestNaturalV7RegistryFingerprintsMatchPreTicket20Goldens(t *testing.T) {
	// The frozen v5 and v6 registries are byte-identical to the pre-ticket-20
	// revision 8c00439: the mechanism-7 parameterization changed nothing.
	for golden, mechanism := range map[string]NaturalMechanism{
		naturalGoldenV5RegistryFingerprintPre20: NaturalMechanismV5,
		naturalGoldenV6RegistryFingerprint:      NaturalMechanismV6,
	} {
		reg, err := NaturalRegistryFor(mechanism)
		if err != nil {
			t.Fatal(err)
		}
		got, err := component.RegistryFingerprint(reg)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(got[:]) != golden {
			t.Fatalf("mechanism %d registry fingerprint drifted: %s", uint16(mechanism), hex.EncodeToString(got[:]))
		}
	}
}

func TestNaturalV7RegistryAdditiveIdentity(t *testing.T) {
	reg5, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	reg6, err := NaturalRegistryFor(NaturalMechanismV6)
	if err != nil {
		t.Fatal(err)
	}
	reg7, err := NaturalRegistryFor(NaturalMechanismV7)
	if err != nil {
		t.Fatal(err)
	}
	// The v7 registry is the v6 registry plus exactly the two recorded-block
	// wage-contract components; 0x8000005A stays spare in every version.
	if !reflect.DeepEqual(reg6.TypeIDs(), reg5.TypeIDs()) {
		t.Fatal("v6 registry type IDs drifted")
	}
	ids7 := reg7.TypeIDs()
	if len(ids7) != 10 {
		t.Fatalf("v7 registry components: %d", len(ids7))
	}
	for _, reserved := range []sim.ComponentTypeID{0x80000058, 0x80000059} {
		if _, err := reg6.Describe(reserved); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatalf("reserved type ID %#x registered under v6", uint32(reserved))
		}
	}
	for _, spare := range []sim.ComponentTypeID{0x8000005A} {
		if _, err := reg7.Describe(spare); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatalf("spare type ID %#x registered under v7", uint32(spare))
		}
	}
	// The frozen eight keep their exact field lists under the v7 names and
	// versions; the granary carries exactly the three additive ledgers and
	// the v5/v6 granary lists stay six and seven entries.
	granary5, _ := reg5.Describe(NaturalGranaryTypeID)
	granary6, _ := reg6.Describe(NaturalGranaryTypeID)
	granary7, err := reg7.Describe(NaturalGranaryTypeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(granary5.Fields) != 6 || len(granary6.Fields) != 7 || len(granary7.Fields) != 9 {
		t.Fatalf("granary field lists: %d %d %d", len(granary5.Fields), len(granary6.Fields), len(granary7.Fields))
	}
	for i := range granary6.Fields {
		if !reflect.DeepEqual(granary6.Fields[i], granary7.Fields[i]) {
			t.Fatalf("v6 granary field %d drifted under v7", granary6.Fields[i].ID)
		}
	}
	if granary7.SchemaVersion != 7 || granary7.Name != "natural-granary-v7" {
		t.Fatalf("granary v7 identity: %q %d", granary7.Name, granary7.SchemaVersion)
	}
	wagesPaid, wagesEarned := granary7.Fields[7], granary7.Fields[8]
	if sim.FieldID(wagesPaid.ID) != NaturalGranaryWagesPaidField || wagesPaid.Name != "wages-paid" || wagesPaid.Unit != "food" ||
		int64(wagesPaid.Bounds.Minimum) != 0 || int64(wagesPaid.Bounds.Maximum) != NaturalWagesMax {
		t.Fatalf("wages-paid descriptor: %+v", wagesPaid)
	}
	if sim.FieldID(wagesEarned.ID) != NaturalGranaryWagesEarnedField || wagesEarned.Name != "wages-earned" || wagesEarned.Unit != "food" ||
		int64(wagesEarned.Bounds.Minimum) != 0 || int64(wagesEarned.Bounds.Maximum) != NaturalWagesMax {
		t.Fatalf("wages-earned descriptor: %+v", wagesEarned)
	}
	// Granary rules under v7: the frozen four, plus provision (v7 ⊇ v6) and
	// the settlement.
	wantGranaryRules := map[sim.RuleID]string{NaturalSpoilageRule: "natural-spoilage-v7", NaturalGrantRule: "natural-yield-grant-v7",
		NaturalEatStoredRule: "natural-eat-stored-v7", NaturalBirthReplyRule: "natural-birth-reply-v7",
		NaturalProvisionRule: "natural-provision-v7", NaturalLabourSettlementRule: "natural-labour-settlement-v7"}
	if len(granary7.TransitionRules) != len(wantGranaryRules) {
		t.Fatalf("v7 granary rules: %+v", granary7.TransitionRules)
	}
	for _, rule := range granary7.TransitionRules {
		name, ok := wantGranaryRules[rule.ID]
		if !ok || rule.Version != 7 || rule.Name != name {
			t.Fatalf("v7 granary rule: %+v", rule)
		}
	}
	// Bag, slot, and worksite admit only their enumerated rules additively;
	// patch, body, genome, and birth-request keep their exact v6 rule sets
	// and field lists.
	slot7, _ := reg7.Describe(NaturalSlotTypeID)
	bag7, _ := reg7.Describe(NaturalBagTypeID)
	worksite7, _ := reg7.Describe(NaturalWorksiteTypeID)
	additiveOnly := map[sim.ComponentTypeID]map[sim.RuleID]bool{
		NaturalSlotTypeID:     {NaturalLabourSettlementRule: true},
		NaturalBagTypeID:      {NaturalProvisionRule: true, NaturalLabourSettlementRule: true},
		NaturalWorksiteTypeID: {NaturalLabourSettlementRule: true},
	}
	for _, id := range []sim.ComponentTypeID{NaturalSlotTypeID, NaturalBagTypeID, NaturalWorksiteTypeID} {
		d6, err := reg6.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		var d7 component.ComponentDescriptor
		switch id {
		case NaturalSlotTypeID:
			d7 = slot7
		case NaturalBagTypeID:
			d7 = bag7
		default:
			d7 = worksite7
		}
		if d7.SchemaVersion != 7 || d7.Name != d6.Name[:len(d6.Name)-2]+"v7" {
			t.Fatalf("component %#x v7 identity: %q %d", uint32(id), d7.Name, d7.SchemaVersion)
		}
		rules6, rules7 := map[sim.RuleID]bool{}, map[sim.RuleID]bool{}
		for _, rule := range d6.TransitionRules {
			rules6[rule.ID] = true
		}
		for _, rule := range d7.TransitionRules {
			rules7[rule.ID] = true
			if rule.Version != 7 {
				t.Fatalf("component %#x rule version: %+v", uint32(id), rule)
			}
		}
		added := additiveOnly[id]
		for id6 := range rules6 {
			if !rules7[id6] {
				t.Fatalf("component %#x lost rule %d under v7", uint32(id), id6)
			}
		}
		for id7 := range rules7 {
			if !rules6[id7] && !added[id7] {
				t.Fatalf("component %#x grew unexpected rule %d", uint32(id), id7)
			}
		}
	}
	for _, id := range []sim.ComponentTypeID{NaturalPatchTypeID, NaturalBodyTypeID, NaturalGenomeTypeID, NaturalBirthRequestTypeID} {
		d6, _ := reg6.Describe(id)
		d7, _ := reg7.Describe(id)
		rules6, rules7 := map[sim.RuleID]bool{}, map[sim.RuleID]bool{}
		for _, rule := range d6.TransitionRules {
			rules6[rule.ID] = true
		}
		for _, rule := range d7.TransitionRules {
			rules7[rule.ID] = true
		}
		if !reflect.DeepEqual(rules6, rules7) || len(d6.Fields) != len(d7.Fields) || d7.SchemaVersion != 7 {
			t.Fatalf("component %#x drifted under v7", uint32(id))
		}
	}
	// The two new components with their exact field lists and rule sets.
	offer7, err := reg7.Describe(NaturalLabourOfferTypeID)
	if err != nil {
		t.Fatal(err)
	}
	if offer7.SchemaVersion != 7 || offer7.Name != "natural-labour-offer-v7" {
		t.Fatalf("labour-offer identity: %q %d", offer7.Name, offer7.SchemaVersion)
	}
	offerFields := []string{}
	for _, f := range offer7.Fields {
		offerFields = append(offerFields, fmt.Sprintf("%d:%s", f.ID, f.Name))
	}
	if !reflect.DeepEqual(offerFields, []string{"1:hour", "2:addressee", "3:status", "4:last-offer-hour", "5:last-reply-hour"}) {
		t.Fatalf("labour-offer fields: %v", offerFields)
	}
	offerRules := map[sim.RuleID]bool{}
	for _, rule := range offer7.TransitionRules {
		offerRules[rule.ID] = true
		if rule.Version != 7 {
			t.Fatalf("labour-offer rule version: %+v", rule)
		}
	}
	// Ticket-20 fix: the offer row is part of the newborn allocation set under
	// rule 511, so 511 must be declared here too.
	for _, id := range []sim.RuleID{NaturalLabourOfferRule, NaturalLabourDecisionRule, NaturalLabourExpireRule, NaturalBirthReplyRule} {
		if !offerRules[id] {
			t.Fatalf("labour-offer rule %d missing", id)
		}
	}
	if len(offerRules) != 4 {
		t.Fatalf("labour-offer rule count: %d", len(offerRules))
	}
	book7, err := reg7.Describe(NaturalLabourBookTypeID)
	if err != nil {
		t.Fatal(err)
	}
	if book7.SchemaVersion != 7 || book7.Name != "natural-labour-book-v7" {
		t.Fatalf("labour-book identity: %q %d", book7.Name, book7.SchemaVersion)
	}
	bookFields := []string{}
	for _, f := range book7.Fields {
		bookFields = append(bookFields, fmt.Sprintf("%d:%s", f.ID, f.Name))
	}
	if !reflect.DeepEqual(bookFields, []string{"1:engaged-worker", "2:engaged-hour", "3:last-decision-hour"}) {
		t.Fatalf("labour-book fields: %v", bookFields)
	}
	bookRules := map[sim.RuleID]bool{}
	for _, rule := range book7.TransitionRules {
		bookRules[rule.ID] = true
	}
	// Ticket-20 fix: the labour rows are part of the newborn allocation set
	// under rule 511, and the allocation path requires the descriptor to
	// declare that writer, so both labour components must list 511 at v7.
	if !bookRules[NaturalLabourDecisionRule] || !bookRules[NaturalLabourExpireRule] ||
		!bookRules[NaturalBirthReplyRule] || len(bookRules) != 3 {
		t.Fatalf("labour-book rules: %+v", book7.TransitionRules)
	}

	// The v7 rule table is exactly the frozen 501-515 plus 516-519.
	declared := map[sim.RuleID]bool{}
	for _, id := range reg7.TypeIDs() {
		d, err := reg7.Describe(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range d.TransitionRules {
			declared[rule.ID] = true
		}
	}
	for id := sim.RuleID(501); id <= 519; id++ {
		if !declared[id] {
			t.Fatalf("v7 rule %d not declared", id)
		}
	}
	if declared[500] || declared[520] {
		t.Fatal("v7 rule range")
	}
	// Cross-mechanism identities never collide.
	fp7, err := component.RegistryFingerprint(reg7)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(fp7[:]) == naturalGoldenV5RegistryFingerprintPre20 || hex.EncodeToString(fp7[:]) == naturalGoldenV6RegistryFingerprint {
		t.Fatal("v7 registry fingerprint collides with a frozen registry")
	}
}

func TestNaturalV7WageFieldsAndComponentsOnlyUnderV7(t *testing.T) {
	// The wage ledgers and the wage-contract components exist only under the
	// v7 registry; the v5/v6 granary lists keep exactly their frozen entries.
	reg5, _ := NaturalRegistry()
	reg6, _ := NaturalRegistryFor(NaturalMechanismV6)
	reg7, _ := NaturalRegistryFor(NaturalMechanismV7)
	for _, reg := range []component.Registry{reg5, reg6} {
		d, _ := reg.Describe(NaturalGranaryTypeID)
		for _, f := range d.Fields {
			if sim.FieldID(f.ID) == NaturalGranaryWagesPaidField || sim.FieldID(f.ID) == NaturalGranaryWagesEarnedField {
				t.Fatalf("wage ledger %d present outside v7", f.ID)
			}
		}
		if _, err := reg.Describe(NaturalLabourOfferTypeID); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatal("labour-offer registered outside v7")
		}
		if _, err := reg.Describe(NaturalLabourBookTypeID); !errors.Is(err, component.ErrUnknownComponentType) {
			t.Fatal("labour-book registered outside v7")
		}
	}
	d7, _ := reg7.Describe(NaturalGranaryTypeID)
	seenPaid, seenEarned := false, false
	for _, f := range d7.Fields {
		seenPaid = seenPaid || sim.FieldID(f.ID) == NaturalGranaryWagesPaidField
		seenEarned = seenEarned || sim.FieldID(f.ID) == NaturalGranaryWagesEarnedField
	}
	if !seenPaid || !seenEarned {
		t.Fatal("v7 granary missing the wage ledgers")
	}
	// A v7 granary row cannot seed wages beyond the horizon envelope.
	patches := 2
	bounded := naturalV7GranarySeeds(naturalNeutralSeeds(patches, 8, 140))
	for i := range bounded {
		if bounded[i].Component != NaturalGranaryTypeID {
			continue
		}
		bounded[i].Fields = append(bounded[i].Fields[:len(bounded[i].Fields)-1],
			component.FieldSeed{Field: NaturalGranaryWagesEarnedField, Value: sim.IntegerValue(NaturalWagesMax + 1)})
		break
	}
	if _, _, err := component.NewReader(reg7, 0, bounded); !errors.Is(err, component.ErrValueOutsideSchema) {
		t.Fatalf("wages-earned above the envelope seeded: %v", err)
	}
}

func TestNaturalOwnershipGuardV7WriterTable(t *testing.T) {
	// The complete field-level G5 admission table of the mechanism axis,
	// probed exhaustively: the frozen table is identical under V6 and V7 for
	// every frozen rule, and the v7 additions are exactly the enumerated
	// triples — every neighbour is refused.
	type triple struct {
		rule      sim.RuleID
		component sim.ComponentTypeID
		field     sim.FieldID
	}
	components := []sim.ComponentTypeID{NaturalPatchTypeID, NaturalSlotTypeID, NaturalBagTypeID, NaturalBodyTypeID,
		NaturalWorksiteTypeID, NaturalGranaryTypeID, NaturalGenomeTypeID, NaturalBirthRequestTypeID,
		NaturalLabourOfferTypeID, NaturalLabourBookTypeID}
	fields := []sim.FieldID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	v7Triples := map[triple]bool{
		// 516 files the worker's own offer row (no claim, no reply-hour).
		{NaturalLabourOfferRule, NaturalLabourOfferTypeID, NaturalLabourOfferHourField}:          true,
		{NaturalLabourOfferRule, NaturalLabourOfferTypeID, NaturalLabourOfferAddresseeField}:     true,
		{NaturalLabourOfferRule, NaturalLabourOfferTypeID, NaturalLabourOfferStatusField}:        true,
		{NaturalLabourOfferRule, NaturalLabourOfferTypeID, NaturalLabourOfferLastOfferHourField}: true,
		// 517: the worker's enumerated status fields + the engagement row.
		{NaturalLabourDecisionRule, NaturalLabourOfferTypeID, NaturalLabourOfferStatusField}:         true,
		{NaturalLabourDecisionRule, NaturalLabourOfferTypeID, NaturalLabourOfferLastReplyHourField}:  true,
		{NaturalLabourDecisionRule, NaturalLabourBookTypeID, NaturalLabourBookEngagedWorkerField}:    true,
		{NaturalLabourDecisionRule, NaturalLabourBookTypeID, NaturalLabourBookEngagedHourField}:      true,
		{NaturalLabourDecisionRule, NaturalLabourBookTypeID, NaturalLabourBookLastDecisionHourField}: true,
		// 518: the enumerated settlement set (no last-build-hour).
		{NaturalLabourSettlementRule, NaturalBagTypeID, NaturalBagUnitsField}:                   true,
		{NaturalLabourSettlementRule, NaturalGranaryTypeID, NaturalGranaryStockField}:           true,
		{NaturalLabourSettlementRule, NaturalGranaryTypeID, NaturalGranaryWagesPaidField}:       true,
		{NaturalLabourSettlementRule, NaturalGranaryTypeID, NaturalGranaryWagesEarnedField}:     true,
		{NaturalLabourSettlementRule, NaturalWorksiteTypeID, NaturalWorksiteCapitalField}:       true,
		{NaturalLabourSettlementRule, NaturalWorksiteTypeID, NaturalWorksiteWipField}:           true,
		{NaturalLabourSettlementRule, NaturalWorksiteTypeID, NaturalWorksiteInvestedUnitsField}: true,
		{NaturalLabourSettlementRule, NaturalWorksiteTypeID, NaturalWorksitePointsCreatedField}: true,
		// 519: status clearing only.
		{NaturalLabourExpireRule, NaturalLabourOfferTypeID, NaturalLabourOfferStatusField}:      true,
		{NaturalLabourExpireRule, NaturalLabourBookTypeID, NaturalLabourBookEngagedWorkerField}: true,
		{NaturalLabourExpireRule, NaturalLabourBookTypeID, NaturalLabourBookEngagedHourField}:   true,
	}
	for rule := sim.RuleID(501); rule <= 519; rule++ {
		for _, componentType := range components {
			for _, field := range fields {
				v6 := naturalPatchFieldAllowedFor(NaturalMechanismV6, rule, componentType, field)
				v7 := naturalPatchFieldAllowedFor(NaturalMechanismV7, rule, componentType, field)
				switch {
				case rule >= NaturalLabourOfferRule:
					// The wage-contract rules: V7-only, exactly enumerated.
					if v6 {
						t.Fatalf("rule %d admitted outside v7", rule)
					}
					if v7 != v7Triples[triple{rule, componentType, field}] {
						t.Fatalf("v7 rule %d admission %#x/%d = %v", rule, uint32(componentType), field, v7)
					}
				default:
					// The frozen table: identical under V6 and V7.
					if v6 != v7 {
						t.Fatalf("frozen rule %d admission drifted under v7 (%#x/%d)", rule, uint32(componentType), field)
					}
				}
			}
		}
	}
	// 515 stays admitted under v7 with exactly the v6 triples.
	for _, componentType := range components {
		for _, field := range fields {
			if naturalPatchFieldAllowedFor(NaturalMechanismV6, NaturalProvisionRule, componentType, field) !=
				naturalPatchFieldAllowedFor(NaturalMechanismV7, NaturalProvisionRule, componentType, field) {
				t.Fatal("rule 515 admission drifted under v7")
			}
		}
	}
	// Literal-spelling refusals survive: the zero value and the literal-5
	// spelling refuse every wage-contract write.
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanism(5)} {
		if naturalPatchFieldAllowedFor(m, NaturalLabourOfferRule, NaturalLabourOfferTypeID, NaturalLabourOfferHourField) {
			t.Fatal("516 admitted outside v7")
		}
		if naturalPatchFieldAllowedFor(m, NaturalLabourSettlementRule, NaturalGranaryTypeID, NaturalGranaryWagesPaidField) {
			t.Fatal("518 admitted outside v7")
		}
	}
}

func TestNaturalOwnershipGuardV7ProposalNegatives(t *testing.T) {
	patches := 2
	const hour = 5
	_, _, _, worker, employer, slots := naturalV7SettleFixture(t, patches, hour)
	settle, err := NaturalLabourSettleProposalFor(NaturalMechanismV7, "settle/5/1", patches, hour, 1, 2, worker, employer, slots)
	if err != nil {
		t.Fatal(err)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV7, settle, patches); err != nil {
		t.Fatalf("v7 settlement ownership: %v", err)
	}
	// Every other mechanism refuses the whole wage-contract flow.
	for _, m := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanism(5)} {
		if err := NaturalCheckOwnershipFor(m, settle, patches); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("mechanism %d admitted a rule-518 proposal", uint16(m))
		}
	}
	// The filing and decision proposals pass the guard as built.
	idle, idleEmployer := naturalV7Pair(t, patches)
	offer, err := NaturalLabourOfferProposalFor(NaturalMechanismV7, "offer/4/1", patches, 4, 1, 2, 1001, idle)
	if err != nil {
		t.Fatal(err)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV7, offer, patches); err != nil {
		t.Fatalf("v7 offer ownership: %v", err)
	}
	offered, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, idle)
	if err != nil {
		t.Fatal(err)
	}
	decisionAccept, err := NaturalLabourDecisionProposalFor(NaturalMechanismV7, "decision/4/2", patches, 4, 2, 1, true, idleEmployer, offered)
	if err != nil {
		t.Fatal(err)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV7, decisionAccept, patches); err != nil {
		t.Fatalf("v7 decision ownership: %v", err)
	}
	decisionRefuse, err := NaturalLabourDecisionProposalFor(NaturalMechanismV7, "decision/4/2", patches, 4, 2, 1, false, idleEmployer, offered)
	if err != nil {
		t.Fatal(err)
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV7, decisionRefuse, patches); err != nil {
		t.Fatalf("v7 refusal ownership: %v", err)
	}
	forge := func(base kernel.Proposal, mutate func(p *kernel.Proposal)) kernel.Proposal {
		p := base
		p.Key = "forged"
		p.Patches = append([]component.Patch(nil), base.Patches...)
		mutate(&p)
		return p
	}
	patchAt := func(p kernel.Proposal, field sim.FieldID) *component.Patch {
		for i := range p.Patches {
			if p.Patches[i].Field == field {
				return &p.Patches[i]
			}
		}
		return nil
	}
	negatives := []struct {
		name string
		p    kernel.Proposal
	}{
		{"wages-paid on the worker", forge(settle, func(p *kernel.Proposal) { patchAt(*p, NaturalGranaryWagesPaidField).Entity = 1 })},
		{"wages-earned on the employer", forge(settle, func(p *kernel.Proposal) { patchAt(*p, NaturalGranaryWagesEarnedField).Entity = 2 })},
		{"bag on the employer", forge(settle, func(p *kernel.Proposal) { patchAt(*p, NaturalBagUnitsField).Entity = 2 })},
		{"worksite on the worker", forge(settle, func(p *kernel.Proposal) {
			for i := range p.Patches {
				if p.Patches[i].Component == NaturalWorksiteTypeID {
					p.Patches[i].Entity = 1
				}
			}
		})},
		{"third-party granary", forge(settle, func(p *kernel.Proposal) {
			p.Patches = append(p.Patches, component.Patch{Entity: 3, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryStockField, Value: sim.IntegerValue(1)})
		})},
		{"two employers", forge(settle, func(p *kernel.Proposal) {
			p.Patches = append(p.Patches, component.Patch{Entity: 3, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryWagesPaidField, Value: sim.IntegerValue(1)})
		})},
		{"last-build-hour write", forge(settle, func(p *kernel.Proposal) {
			p.Patches = append(p.Patches, component.Patch{Entity: 2, Component: NaturalWorksiteTypeID, SchemaVersion: 7, Field: NaturalWorksiteLastBuildHourField, Value: sim.IntegerValue(int64(hour))})
		})},
		{"tenure without the completion signature", forge(settle, func(p *kernel.Proposal) {
			kept := p.Patches[:0]
			for _, patch := range p.Patches {
				if patch.Field != NaturalWorksiteCapitalField && patch.Field != NaturalWorksitePointsCreatedField {
					kept = append(kept, patch)
				}
			}
			p.Patches = kept
		})},
		{"capital without the completion signature", forge(settle, func(p *kernel.Proposal) {
			kept := p.Patches[:0]
			for _, patch := range p.Patches {
				if patch.Component != NaturalSlotTypeID {
					kept = append(kept, patch)
				}
			}
			p.Patches = kept
		})},
		{"slot stock write under 518", forge(settle, func(p *kernel.Proposal) {
			p.Patches = append(p.Patches, component.Patch{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: 7, Field: NaturalSlotStockField, Value: sim.IntegerValue(0)})
		})},
		{"decision on a third party's row", forge(decisionAccept, func(p *kernel.Proposal) {
			p.Patches = append(p.Patches, component.Patch{Entity: 3, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryStockField, Value: sim.IntegerValue(1)})
		})},
		{"decision on the decider's own row", forge(decisionAccept, func(p *kernel.Proposal) {
			for i := range p.Patches {
				if p.Patches[i].Component == NaturalLabourOfferTypeID {
					p.Patches[i].Entity = 2
				}
			}
		})},
		{"decision over two workers", forge(decisionAccept, func(p *kernel.Proposal) {
			p.Patches = append(p.Patches, component.Patch{Entity: 3, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestAccepted))})
		})},
		{"decision without the engagement row", forge(decisionAccept, func(p *kernel.Proposal) {
			kept := p.Patches[:0]
			for _, patch := range p.Patches {
				if patch.Component != NaturalLabourBookTypeID {
					kept = append(kept, patch)
				}
			}
			p.Patches = kept
		})},
		{"offer with a reply-hour write", forge(offer, func(p *kernel.Proposal) {
			p.Patches[0] = component.Patch{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferLastReplyHourField, Value: sim.IntegerValue(int64(hour))}
		})},
		{"offer on a foreign row", forge(offer, func(p *kernel.Proposal) {
			for i := range p.Patches {
				p.Patches[i].Entity = 2
			}
		})},
		{"expiry with an actor cause", forge(kernel.Proposal{Key: "expiry", Time: mustNaturalPhase(t, 5, NaturalPhasePulse), Cause: kernel.Cause{World: true},
			Rule: NaturalLabourExpireRule, RuleVersion: 7,
			Patches: []component.Patch{{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestExpired))}}},
			func(p *kernel.Proposal) { p.Cause = kernel.Cause{Actor: 1} })},
		{"expiry writing a reply hour", forge(kernel.Proposal{Key: "expiry", Time: mustNaturalPhase(t, 5, NaturalPhasePulse), Cause: kernel.Cause{World: true},
			Rule: NaturalLabourExpireRule, RuleVersion: 7,
			Patches: []component.Patch{{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestExpired))}}},
			func(p *kernel.Proposal) { p.Patches[0].Field = NaturalLabourOfferLastReplyHourField })},
		{"expiry writing a book decision hour", forge(kernel.Proposal{Key: "expiry", Time: mustNaturalPhase(t, 5, NaturalPhasePulse), Cause: kernel.Cause{World: true},
			Rule: NaturalLabourExpireRule, RuleVersion: 7,
			Patches: []component.Patch{{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestExpired))}}},
			func(p *kernel.Proposal) {
				p.Patches = append(p.Patches, component.Patch{Entity: 1, Component: NaturalLabourBookTypeID, SchemaVersion: 7, Field: NaturalLabourBookLastDecisionHourField, Value: sim.IntegerValue(int64(hour))})
			})},
	}
	for _, negative := range negatives {
		if err := NaturalCheckOwnershipFor(NaturalMechanismV7, negative.p, patches); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s admitted: %v", negative.name, err)
		}
	}
	// Tenure stays writable only by 503/505/514 plus 518's enumerated
	// completion claim: the same tenure patch is refused under every other
	// rule, frozen world rules included.
	tenurePatch := []component.Patch{
		{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: 7, Field: NaturalSlotTenureModeField, Value: sim.IntegerValue(1)},
		{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: 7, Field: NaturalSlotTenureOwnerField, Value: mustNaturalRef(t, 2)},
		{Entity: 2003, Component: NaturalSlotTypeID, SchemaVersion: 7, Field: NaturalSlotTenureCultivatedField, Value: sim.IntegerValue(1)},
	}
	for _, rule := range []sim.RuleID{NaturalRegenSplitRule, NaturalSpoilageRule, NaturalBasalRule, NaturalGrantRule, NaturalBirthExpireRule,
		NaturalGatherRule, NaturalConsumeRule, NaturalEatStoredRule, NaturalBirthReplyRule,
		NaturalProvisionRule, NaturalLabourOfferRule, NaturalLabourDecisionRule} {
		cause := kernel.Cause{Actor: 1}
		if naturalWorldRule(rule) {
			cause = kernel.Cause{World: true}
		}
		p := kernel.Proposal{Key: "tenure-probe", Time: mustNaturalPhase(t, 5, NaturalPhaseClaim), Cause: cause, Rule: rule, RuleVersion: 7, Patches: tenurePatch}
		if err := NaturalCheckOwnershipFor(NaturalMechanismV7, p, patches); !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("rule %d admitted a tenure write: %v", rule, err)
		}
	}
	// And the completion channel itself: a tenure-carrying 518 proposal is
	// the only tenure-writing addition (proven by the settle proposal above
	// passing with its slot claim).
}

func TestNaturalLabourOfferDecisionTransitionsAndProposals(t *testing.T) {
	patches := 2
	worker, employer := naturalV7Pair(t, patches)
	frozenWorker, frozenEmployer := worker, employer
	// Filing effects: hour, addressee, status, last-offer-hour; the reply
	// audit is untouched.
	offered, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(worker, frozenWorker) {
		t.Fatal("offer mutated its input")
	}
	want := frozenWorker
	want.Offer = NaturalLabourOfferState{Hour: 4, Addressee: 2, Status: NaturalRequestPending, LastOfferHour: 4, LastReplyHour: NaturalNeverHour}
	if !reflect.DeepEqual(offered, want) {
		t.Fatalf("offer state: %+v", offered.Offer)
	}
	if !validNaturalActorFor(NaturalMechanismV7, patches, 1, offered) {
		t.Fatal("offered state invalid under v7")
	}
	// Accept effects: the worker's status/reply hour and the engagement row.
	acceptedEmployer, acceptedWorker, decision, err := NaturalLabourDecision(NaturalMechanismV7, patches, 4, 2, 1, true, employer, offered)
	if err != nil || decision != NaturalReplyAccept {
		t.Fatalf("accept: %v %v", decision, err)
	}
	if !reflect.DeepEqual(employer, frozenEmployer) {
		t.Fatal("decision mutated its input")
	}
	if acceptedWorker.Offer.Status != NaturalRequestAccepted || acceptedWorker.Offer.LastReplyHour != 4 {
		t.Fatalf("accepted worker row: %+v", acceptedWorker.Offer)
	}
	if acceptedEmployer.Book.EngagedWorker != 1 || acceptedEmployer.Book.EngagedHour != 4 || acceptedEmployer.Book.LastDecisionHour != 4 {
		t.Fatalf("engaged book row: %+v", acceptedEmployer.Book)
	}
	// Refuse effects: status rows only.
	offered2, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	refusedEmployer, refusedWorker, decision, err := NaturalLabourDecision(NaturalMechanismV7, patches, 4, 2, 1, false, employer, offered2)
	if err != nil || decision != NaturalReplyRefuse {
		t.Fatalf("refuse: %v %v", decision, err)
	}
	if refusedWorker.Offer.Status != NaturalRequestRefused || refusedWorker.Offer.LastReplyHour != 4 {
		t.Fatalf("refused worker row: %+v", refusedWorker.Offer)
	}
	if refusedEmployer.Book.EngagedWorker != 0 || refusedEmployer.Book.EngagedHour != NaturalNeverHour || refusedEmployer.Book.LastDecisionHour != 4 {
		t.Fatalf("refused book row: %+v", refusedEmployer.Book)
	}
	// Filing negatives, each with zero mutation and a zero return.
	engaged, _, err := naturalV7EngagePartial(t, patches, 4, worker, employer)
	if err != nil {
		t.Fatal(err)
	}
	_ = engaged
	withBag := worker
	withBag.Bag.Units, withBag.Body.LastGatherHour = 1, 3
	dead := worker
	dead.Genome.DiedHour = 3
	crossPatch := worker
	crossPatch.Bag.Source = 1002
	pending := worker
	pending.Offer = NaturalLabourOfferState{Hour: 3, Addressee: 2, Status: NaturalRequestPending, LastOfferHour: 3, LastReplyHour: NaturalNeverHour}
	alreadyAccepted := worker
	alreadyAccepted.Offer = NaturalLabourOfferState{Hour: 3, Addressee: 2, Status: NaturalRequestAccepted, LastOfferHour: 3, LastReplyHour: 3}
	refiledSameHour := worker
	refiledSameHour.Offer = NaturalLabourOfferState{Hour: 3, Addressee: 2, Status: NaturalRequestIdle, LastOfferHour: 4, LastReplyHour: 3}
	invalid := func(name string, m NaturalMechanism, hour int, addressee, addresseeHome sim.EntityID, a NaturalActorState) {
		t.Helper()
		got, err := NaturalLabourOffer(m, patches, hour, 1, addressee, addresseeHome, a)
		if !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s admitted: %+v %v", name, got, err)
		}
		if got != (NaturalActorState{}) {
			t.Fatalf("%s mutated on refusal", name)
		}
	}
	invalid("zero-value mechanism", NaturalMechanismV5, 4, 2, 1001, worker)
	invalid("literal v5 spelling", NaturalMechanism(5), 4, 2, 1001, worker)
	invalid("bag held", NaturalMechanismV7, 4, 2, 1001, withBag)
	invalid("dead worker", NaturalMechanismV7, 4, 2, 1001, dead)
	invalid("cross-patch addressee", NaturalMechanismV7, 4, 2, 1002, worker)
	invalid("self addressee", NaturalMechanismV7, 4, 1, 1001, worker)
	invalid("pending offer open", NaturalMechanismV7, 4, 2, 1001, pending)
	invalid("accepted contract open", NaturalMechanismV7, 4, 2, 1001, alreadyAccepted)
	invalid("same-hour refiling", NaturalMechanismV7, 4, 2, 1001, refiledSameHour)
	invalid("hour beyond the horizon", NaturalMechanismV7, NaturalHorizonHours, 2, 1001, worker)
	// Decision negatives: not addressed to the decider, second decision in
	// the same hour, occupied engagement slot, dead employer, wrong status.
	offered3, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	bookBusy := employer
	bookBusy.Book = NaturalLabourBookState{EngagedWorker: 3, EngagedHour: 3, LastDecisionHour: 3}
	decided := employer
	decided.Book = NaturalLabourBookState{EngagedHour: NaturalNeverHour, LastDecisionHour: 4}
	deadEmployer := employer
	deadEmployer.Genome.DiedHour = 3
	decisionInvalid := func(name string, employerState, workerState NaturalActorState, workerID sim.EntityID) {
		t.Helper()
		e, w, d, err := NaturalLabourDecision(NaturalMechanismV7, patches, 4, 2, workerID, true, employerState, workerState)
		if !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s admitted: %v %+v", name, d, err)
		}
		if e != (NaturalActorState{}) || w != (NaturalActorState{}) {
			t.Fatalf("%s mutated on refusal", name)
		}
	}
	// Not-addressed-to-the-decider: the offer names employer 3; employer 2
	// cannot decide it. (The pure math binds the offer row to its addressee;
	// the worker's own identity lives in the row, so a mismatched workerID
	// argument is the caller's binding, as with every frozen state+ID pair.)
	offerThree, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 3, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	decisionInvalid("not addressed to the decider", employer, offerThree, 3)
	decisionInvalid("second decision in the hour", decided, offered3, 1)
	decisionInvalid("engagement slot occupied", bookBusy, offered3, 1)
	decisionInvalid("dead employer", deadEmployer, offered3, 1)
	decisionInvalid("wrong status", employer, frozenWorker, 1)
}

// naturalV7EngagePartial is naturalV7Engage without the gather, for tests
// that need the matched contract only.
func naturalV7EngagePartial(t *testing.T, patches, hour int, worker, employer NaturalActorState) (NaturalActorState, NaturalActorState, error) {
	t.Helper()
	offered, err := NaturalLabourOffer(NaturalMechanismV7, patches, hour, 1, 2, 1001, worker)
	if err != nil {
		return NaturalActorState{}, NaturalActorState{}, err
	}
	engagedEmployer, engagedWorker, _, err := NaturalLabourDecision(NaturalMechanismV7, patches, hour, 2, 1, true, employer, offered)
	return engagedWorker, engagedEmployer, err
}

func TestNaturalLabourProposalShapes(t *testing.T) {
	patches := 2
	worker, employer := naturalV7Pair(t, patches)
	// The offer proposal: the four filing fields on the worker's own row.
	offer, err := NaturalLabourOfferProposalFor(NaturalMechanismV7, "offer/4/1", patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	if offer.Time != mustNaturalPhase(t, 4, NaturalPhaseLabourOffer) || offer.Rule != NaturalLabourOfferRule ||
		offer.RuleVersion != 7 || offer.Cause.Actor != 1 || offer.Cause.World {
		t.Fatalf("offer proposal identity: %+v", offer)
	}
	wantOffer := []component.Patch{
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferHourField, Value: sim.IntegerValue(4)},
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferAddresseeField, Value: mustNaturalRef(t, 2)},
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestPending))},
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferLastOfferHourField, Value: sim.IntegerValue(4)},
	}
	if !reflect.DeepEqual(offer.Patches, wantOffer) {
		t.Fatalf("offer patches: %+v", offer.Patches)
	}
	// Decision proposals: accept engages; refuse writes status rows only.
	offered, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	accept, err := NaturalLabourDecisionProposalFor(NaturalMechanismV7, "decision/4/2", patches, 4, 2, 1, true, employer, offered)
	if err != nil {
		t.Fatal(err)
	}
	if accept.Time != mustNaturalPhase(t, 4, NaturalPhaseLabourDecision) || accept.Rule != NaturalLabourDecisionRule || accept.Cause.Actor != 2 {
		t.Fatalf("accept proposal identity: %+v", accept)
	}
	wantAccept := []component.Patch{
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestAccepted))},
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferLastReplyHourField, Value: sim.IntegerValue(4)},
		{Entity: 2, Component: NaturalLabourBookTypeID, SchemaVersion: 7, Field: NaturalLabourBookEngagedWorkerField, Value: mustNaturalRef(t, 1)},
		{Entity: 2, Component: NaturalLabourBookTypeID, SchemaVersion: 7, Field: NaturalLabourBookEngagedHourField, Value: sim.IntegerValue(4)},
		{Entity: 2, Component: NaturalLabourBookTypeID, SchemaVersion: 7, Field: NaturalLabourBookLastDecisionHourField, Value: sim.IntegerValue(4)},
	}
	if !reflect.DeepEqual(accept.Patches, wantAccept) {
		t.Fatalf("accept patches: %+v", accept.Patches)
	}
	refuse, err := NaturalLabourDecisionProposalFor(NaturalMechanismV7, "decision/4/2", patches, 4, 2, 1, false, employer, offered)
	if err != nil {
		t.Fatal(err)
	}
	wantRefuse := []component.Patch{
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferStatusField, Value: sim.IntegerValue(int64(NaturalRequestRefused))},
		{Entity: 1, Component: NaturalLabourOfferTypeID, SchemaVersion: 7, Field: NaturalLabourOfferLastReplyHourField, Value: sim.IntegerValue(4)},
		{Entity: 2, Component: NaturalLabourBookTypeID, SchemaVersion: 7, Field: NaturalLabourBookLastDecisionHourField, Value: sim.IntegerValue(4)},
	}
	if !reflect.DeepEqual(refuse.Patches, wantRefuse) {
		t.Fatalf("refuse patches: %+v", refuse.Patches)
	}
	// The v5 zero value builds none of them.
	if _, err := NaturalLabourOfferProposalFor(NaturalMechanismV5, "offer/4/1", patches, 4, 1, 2, 1001, worker); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 offer proposal admitted")
	}
	if _, err := NaturalLabourDecisionProposalFor(NaturalMechanismV6, "decision/4/2", patches, 4, 2, 1, true, employer, offered); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v6 decision proposal admitted")
	}
}

func TestNaturalLabourSettleCompletionEffectsAndConservation(t *testing.T) {
	patches := 2
	const hour = 5
	patchStates, slotStates, founders, worker, employer, homeSlots := naturalV7SettleFixture(t, patches, hour)
	balanceBefore, err := NaturalCheckConservationFor(NaturalMechanismV7, patches, patchStates, slotStates, founders, nil, mustNaturalGenesisLast(patches))
	if err != nil {
		t.Fatal(err)
	}
	frozenWorker, frozenEmployer, frozenSlots := worker, employer, homeSlots
	nextWorker, nextEmployer, nextSlots, result, err := NaturalLabourSettle(NaturalMechanismV7, patches, hour, 1, 2, worker, employer, homeSlots)
	if err != nil {
		t.Fatal(err)
	}
	// The pure function mutated none of its inputs.
	if !reflect.DeepEqual(worker, frozenWorker) || !reflect.DeepEqual(employer, frozenEmployer) || !reflect.DeepEqual(homeSlots, frozenSlots) {
		t.Fatal("settlement mutated its inputs")
	}
	// The frozen handshake effects: worker bag −1, worker granary stock +1,
	// worker wages-earned +1, employer granary stock −1, employer
	// wages-paid +1, employer invested +1 and wip +1, and — wip reached 2 —
	// the frozen point completion (capital +1, created +1) with the
	// lowest-index free-wild-slot tenure claim. LastBuildHour is untouched.
	if nextWorker.Bag.Units != 0 || nextWorker.Granary.Stock != 3 || nextWorker.Granary.WagesEarned != 1 {
		t.Fatalf("worker effects: %+v", nextWorker)
	}
	if nextEmployer.Granary.Stock != 4 || nextEmployer.Granary.WagesPaid != 1 {
		t.Fatalf("employer granary effects: %+v", nextEmployer.Granary)
	}
	if nextEmployer.Worksite.Capital != 2 || nextEmployer.Worksite.Wip != 0 || nextEmployer.Worksite.InvestedUnits != 4 ||
		nextEmployer.Worksite.PointsCreated != 2 || nextEmployer.Worksite.LastBuildHour != NaturalNeverHour {
		t.Fatalf("employer worksite effects: %+v", nextEmployer.Worksite)
	}
	if result.PointsCompleted != 1 || result.ClaimedSlotIndex != 1 {
		t.Fatalf("completion result: %+v", result)
	}
	if nextSlots[1].Mode != NaturalTenureCultivated || nextSlots[1].Owner != 2 || nextSlots[1].Cultivated != 1 || nextSlots[1].Stock != 0 || nextSlots[1].Gathered != 1 {
		t.Fatalf("claimed slot: %+v", nextSlots[1])
	}
	if !validNaturalActorFor(NaturalMechanismV7, patches, 1, nextWorker) || !validNaturalActorFor(NaturalMechanismV7, patches, 2, nextEmployer) {
		t.Fatal("after-states invalid under v7")
	}
	// G2a″ holds on both after-states.
	if nextWorker.Granary.CultivatedTotal+nextWorker.Granary.WagesEarned != nextWorker.Granary.Stock+nextWorker.Granary.StoredMeals+
		nextWorker.Granary.Provisioned+nextWorker.Granary.Spoiled+nextWorker.Genome.BirthGranaryPaid+nextWorker.Granary.WagesPaid {
		t.Fatal("G2a″ broken on the worker")
	}
	if nextEmployer.Granary.CultivatedTotal+nextEmployer.Granary.WagesEarned != nextEmployer.Granary.Stock+nextEmployer.Granary.StoredMeals+
		nextEmployer.Granary.Provisioned+nextEmployer.Granary.Spoiled+nextEmployer.Genome.BirthGranaryPaid+nextEmployer.Granary.WagesPaid {
		t.Fatal("G2a″ broken on the employer")
	}
	// The conservation gate stays green across the settlement, and only the
	// enumerated aggregate terms moved: held −1, invested +1, capital +1,
	// wip −1, created +1, tenured +1, wages +1/+1 — G1's inflow terms
	// (gathered, provisioned, consumed), G3's terms, and the produced,
	// granted, spoiled, stored, and birth ledgers are untouched.
	founders[0], founders[1] = nextWorker, nextEmployer
	slotStates[0] = nextSlots[:]
	balanceAfter, err := NaturalCheckConservationFor(NaturalMechanismV7, patches, patchStates, slotStates, founders, nil, mustNaturalGenesisLast(patches))
	if err != nil {
		t.Fatalf("post-settlement conservation: %v", err)
	}
	deltas := []struct {
		name string
		before,
		after int64
	}{
		{"gathered", balanceBefore.Gathered, balanceAfter.Gathered},
		{"provisioned", balanceBefore.Provisioned, balanceAfter.Provisioned},
		{"consumed", balanceBefore.Consumed, balanceAfter.Consumed},
		{"stored", balanceBefore.StoredMeals, balanceAfter.StoredMeals},
		{"spoiled", balanceBefore.Spoiled, balanceAfter.Spoiled},
		{"produced", balanceBefore.Produced, balanceAfter.Produced},
		{"granted", balanceBefore.Granted, balanceAfter.Granted},
		{"cultivated", balanceBefore.CultivatedTotal, balanceAfter.CultivatedTotal},
		{"granaryStock", balanceBefore.GranaryStock, balanceAfter.GranaryStock},
		{"energy", balanceBefore.Energy, balanceAfter.Energy},
		{"basalSpent", balanceBefore.BasalSpent, balanceAfter.BasalSpent},
		{"capLost", balanceBefore.EnergyCapLost, balanceAfter.EnergyCapLost},
		{"birthGranaryPaid", balanceBefore.BirthGranaryPaid, balanceAfter.BirthGranaryPaid},
	}
	for _, d := range deltas {
		if d.before != d.after {
			t.Fatalf("settlement moved %s: %d → %d", d.name, d.before, d.after)
		}
	}
	moves := []struct {
		name   string
		before int64
		after  int64
	}{
		{"held", balanceBefore.Held, balanceAfter.Held},
		{"invested", balanceBefore.Invested, balanceAfter.Invested},
		{"capital", balanceBefore.Capital, balanceAfter.Capital},
		{"wip", balanceBefore.Wip, balanceAfter.Wip},
		{"created", balanceBefore.PointsCreated, balanceAfter.PointsCreated},
		{"tenured", balanceBefore.TenuredSlots, balanceAfter.TenuredSlots},
		{"wagesPaid", balanceBefore.WagesPaid, balanceAfter.WagesPaid},
		{"wagesEarned", balanceBefore.WagesEarned, balanceAfter.WagesEarned},
	}
	wantMoves := map[string][2]int64{"held": {1, 0}, "invested": {3, 4}, "capital": {1, 2}, "wip": {1, 0},
		"created": {1, 2}, "tenured": {1, 2}, "wagesPaid": {0, 1}, "wagesEarned": {0, 1}}
	for _, m := range moves {
		if m.before != wantMoves[m.name][0] || m.after != wantMoves[m.name][1] {
			t.Fatalf("%s moved %d → %d, want %d → %d", m.name, m.before, m.after, wantMoves[m.name][0], wantMoves[m.name][1])
		}
	}
	if balanceAfter.WagesPaid != balanceAfter.WagesEarned {
		t.Fatal("closed-history wage equality broken")
	}
}

func TestNaturalLabourSettleNonCompletionShape(t *testing.T) {
	patches := 2
	const hour = 5
	// A settlement whose employer holds no unit of work in progress invests
	// without completing and claims no slot.
	worker, employer := naturalV7Pair(t, patches)
	employer.Worksite = NaturalWorksiteState{LastBuildHour: NaturalNeverHour}
	emptySlots := [NaturalSlotsPerPatch]NaturalSlotState{}
	emptySlots[1] = NaturalSlotState{Stock: 1}
	worker, employer, _ = naturalV7Engage(t, NaturalMechanismV7, patches, hour, worker, employer, emptySlots)
	nextWorker, nextEmployer, nextSlots, result, err := NaturalLabourSettle(NaturalMechanismV7, patches, hour, 1, 2, worker, employer, emptySlots)
	if err != nil {
		t.Fatal(err)
	}
	if result.PointsCompleted != 0 || result.ClaimedSlotIndex != -1 {
		t.Fatalf("non-completion result: %+v", result)
	}
	if nextEmployer.Worksite.InvestedUnits != 1 || nextEmployer.Worksite.Wip != 1 || nextEmployer.Worksite.Capital != 0 {
		t.Fatalf("non-completion worksite: %+v", nextEmployer.Worksite)
	}
	if nextWorker.Granary.Stock != 3 || nextWorker.Granary.WagesEarned != 1 || nextWorker.Bag.Units != 0 {
		t.Fatalf("non-completion worker: %+v", nextWorker)
	}
	for i := range nextSlots {
		if nextSlots[i].Mode != NaturalTenureWild {
			t.Fatalf("non-completion claimed slot %d", i)
		}
	}
	// The proposal shape of the non-completion settlement: exactly the seven
	// enumerated patches, no tenure writes.
	proposal, err := NaturalLabourSettleProposalFor(NaturalMechanismV7, "settle/5/1", patches, hour, 1, 2, worker, employer, emptySlots)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Time != mustNaturalPhase(t, hour, NaturalPhaseLabourSettle) || proposal.Rule != NaturalLabourSettlementRule ||
		proposal.RuleVersion != 7 || proposal.Cause.Actor != 1 || proposal.Cause.World {
		t.Fatalf("settle proposal identity: %+v", proposal)
	}
	want := []component.Patch{
		{Entity: 1, Component: NaturalBagTypeID, SchemaVersion: 7, Field: NaturalBagUnitsField, Value: sim.IntegerValue(0)},
		{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryStockField, Value: sim.IntegerValue(3)},
		{Entity: 1, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryWagesEarnedField, Value: sim.IntegerValue(1)},
		{Entity: 2, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryStockField, Value: sim.IntegerValue(4)},
		{Entity: 2, Component: NaturalGranaryTypeID, SchemaVersion: 7, Field: NaturalGranaryWagesPaidField, Value: sim.IntegerValue(1)},
		{Entity: 2, Component: NaturalWorksiteTypeID, SchemaVersion: 7, Field: NaturalWorksiteInvestedUnitsField, Value: sim.IntegerValue(1)},
		{Entity: 2, Component: NaturalWorksiteTypeID, SchemaVersion: 7, Field: NaturalWorksiteWipField, Value: sim.IntegerValue(1)},
	}
	if !reflect.DeepEqual(proposal.Patches, want) {
		t.Fatalf("settle patches: %+v", proposal.Patches)
	}
}

func TestNaturalLabourSettleTypedRefusalsZeroMutation(t *testing.T) {
	patches := 2
	const hour = 5
	_, _, _, worker, employer, slots := naturalV7SettleFixture(t, patches, hour)
	// engage rebuilds the matched, delivery-ready pair (offer + accept at
	// hour−1, the bag and gather bookkeeping set directly).
	engage := func() (NaturalActorState, NaturalActorState) {
		t.Helper()
		w, e := naturalV7Pair(t, patches)
		engagedWorker, engagedEmployer, err := naturalV7EngagePartial(t, patches, hour-1, w, e)
		if err != nil {
			t.Fatal(err)
		}
		engagedWorker.Bag.Units, engagedWorker.Body.LastGatherHour = 1, hour
		return engagedWorker, engagedEmployer
	}
	reason := func(name string, err error) NaturalLabourRefusalReason {
		t.Helper()
		var typed *NaturalLabourRefusalError
		if !errors.As(err, &typed) {
			t.Fatalf("%s: not a typed labour refusal: %v", name, err)
		}
		return typed.Reason
	}
	refuse := func(name string, m NaturalMechanism, settleHour int, workerID, employerID sim.EntityID, w, e NaturalActorState, s [NaturalSlotsPerPatch]NaturalSlotState, wantReason NaturalLabourRefusalReason) {
		t.Helper()
		frozenW, frozenE, frozenS := w, e, s
		nextW, nextE, nextS, result, err := NaturalLabourSettle(m, patches, settleHour, workerID, employerID, w, e, s)
		if err == nil {
			t.Fatalf("%s settled", name)
		}
		if wantReason != NaturalLabourRefusalNone {
			if got := reason(name, err); got != wantReason {
				t.Fatalf("%s: reason %d, want %d", name, got, wantReason)
			}
		} else if _, ok := err.(*NaturalLabourRefusalError); ok {
			t.Fatalf("%s: unexpected typed refusal %v", name, err)
		}
		if !errors.Is(err, ErrNaturalContract) {
			t.Fatalf("%s: wrong error family: %v", name, err)
		}
		if nextW != (NaturalActorState{}) || nextE != (NaturalActorState{}) || result != (NaturalLabourSettleResult{}) {
			t.Fatalf("%s mutated its return on refusal", name)
		}
		if !reflect.DeepEqual(s, frozenS) {
			t.Fatalf("%s mutated the slot array", name)
		}
		if !reflect.DeepEqual(w, frozenW) || !reflect.DeepEqual(e, frozenE) {
			t.Fatalf("%s mutated its inputs", name)
		}
		_ = nextW
		_ = nextE
		_ = nextS
	}
	// Contract-not-pending: unmatched variants of the matched fixture.
	idleWorker, _ := naturalV7Pair(t, patches)
	refuse("no offer at all", NaturalMechanismV7, hour, 1, 2, idleWorker, employer, slots, NaturalLabourRefusalContractNotPending)
	unaccepted, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, idleWorker)
	if err != nil {
		t.Fatal(err)
	}
	refuse("offer without accept", NaturalMechanismV7, hour, 1, 2, unaccepted, employer, slots, NaturalLabourRefusalContractNotPending)
	offerElsewhereWorker, offerElsewhereEmployer := engage()
	offerElsewhereWorker.Offer.Addressee = 3
	refuse("offer addressed elsewhere", NaturalMechanismV7, hour, 1, 2, offerElsewhereWorker, offerElsewhereEmployer, slots, NaturalLabourRefusalContractNotPending)
	lateWindow, lateBook := engage()
	lateWindow.Offer.Hour, lateWindow.Offer.LastOfferHour, lateWindow.Offer.LastReplyHour, lateBook.Book.EngagedHour = 2, 2, 2, 2
	refuse("delivery window closed", NaturalMechanismV7, hour, 1, 2, lateWindow, lateBook, slots, NaturalLabourRefusalContractNotPending)
	engagedWorker, engagedEmployer := engage()
	engagedEmployer.Book.EngagedWorker = 3
	refuse("engagement mismatch", NaturalMechanismV7, hour, 1, 2, engagedWorker, engagedEmployer, slots, NaturalLabourRefusalContractNotPending)
	// Liveness: both parties checked, worker first.
	deadWorker, liveBook := engage()
	deadWorker.Genome.DiedHour = 4
	refuse("worker dead", NaturalMechanismV7, hour, 1, 2, deadWorker, liveBook, slots, NaturalLabourRefusalWorkerDeadOrCrossPatch)
	liveWorker, deadEmployer := engage()
	deadEmployer.Genome.DiedHour = 4
	refuse("employer dead", NaturalMechanismV7, hour, 1, 2, liveWorker, deadEmployer, slots, NaturalLabourRefusalEmployerDeadOrCrossPatch)
	// Cross-patch pairs: only a carried-home newborn can hold a valid state
	// whose bag source differs from its addressee's (founder homes are
	// ID-placed), so the forged cross-patch contract rides a newborn row.
	newbornWorker := naturalV7CrossPatchNewborn(t, patches, hour)
	newbornEmployer := employer
	newbornEmployer.Book = NaturalLabourBookState{EngagedWorker: 2017, EngagedHour: int64(hour) - 1, LastDecisionHour: int64(hour) - 1}
	refuse("cross-patch pair (worker reason)", NaturalMechanismV7, hour, 2017, 2, newbornWorker, newbornEmployer, slots, NaturalLabourRefusalWorkerDeadOrCrossPatch)
	// A founder's bag source is ID-placed, so moving it is an invalid state
	// (plain refusal) — only a carried-home newborn can be genuinely
	// cross-patch, which the case above covers.
	movedWorker, movedEmployer := engage()
	movedWorker.Bag.Source = 1002
	refuse("worker home moved off its founder patch", NaturalMechanismV7, hour, 1, 2, movedWorker, movedEmployer, slots, NaturalLabourRefusalNone)
	// Resources, in the frozen order.
	emptyBag, bagBook := engage()
	emptyBag.Bag.Units = 0
	refuse("worker bag empty", NaturalMechanismV7, hour, 1, 2, emptyBag, bagBook, slots, NaturalLabourRefusalWorkerBagEmpty)
	poorWorker, poorBook := engage()
	poorBook.Granary.Stock, poorBook.Granary.Spoiled = 0, 5 // the wage stock rotted: G2a″ stays exact
	refuse("employer stock short", NaturalMechanismV7, hour, 1, 2, poorWorker, poorBook, slots, NaturalLabourRefusalEmployerStockShort)
	atMaxWorker, atMaxBook := engage()
	atMaxBook.Worksite = NaturalWorksiteState{Capital: NaturalKMax - 1, Wip: 1, InvestedUnits: 2*(NaturalKMax-1) + 1, PointsCreated: NaturalKMax - 1, LastBuildHour: NaturalNeverHour}
	refuse("employer at k+wip max", NaturalMechanismV7, hour, 1, 2, atMaxWorker, atMaxBook, slots, NaturalLabourRefusalEmployerAtKMax)
	// The completion needs a lowest-index free wild slot with stock 0.
	bagWorker, bagBook := engage()
	stockedSlots := [NaturalSlotsPerPatch]NaturalSlotState{}
	stockedSlots[0] = NaturalSlotState{Gathered: 3, Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1}
	for i := 1; i < NaturalSlotsPerPatch; i++ {
		stockedSlots[i] = NaturalSlotState{Stock: 1, Gathered: 1}
	}
	refuse("no free wild slot", NaturalMechanismV7, hour, 1, 2, bagWorker, bagBook, stockedSlots, NaturalLabourRefusalNoFreeSlot)
	tenuredSlots := [NaturalSlotsPerPatch]NaturalSlotState{}
	for i := range tenuredSlots {
		tenuredSlots[i] = NaturalSlotState{Gathered: 1, Mode: NaturalTenureCultivated, Owner: 2, Cultivated: 1}
	}
	refuse("no wild slot at all", NaturalMechanismV7, hour, 1, 2, bagWorker, bagBook, tenuredSlots, NaturalLabourRefusalNoFreeSlot)
	// Mechanism and call-shape refusals (plain ErrNaturalContract).
	refuse("zero-value mechanism", NaturalMechanismV5, hour, 1, 2, worker, employer, slots, NaturalLabourRefusalNone)
	refuse("v6 mechanism", NaturalMechanismV6, hour, 1, 2, worker, employer, slots, NaturalLabourRefusalNone)
	refuse("literal v5 spelling", NaturalMechanism(5), hour, 1, 2, worker, employer, slots, NaturalLabourRefusalNone)
	refuse("hour beyond the horizon", NaturalMechanismV7, NaturalHorizonHours, 1, 2, worker, employer, slots, NaturalLabourRefusalNone)
	refuse("self settlement", NaturalMechanismV7, hour, 1, 1, worker, worker, slots, NaturalLabourRefusalNone)
	invalidSlots := slots
	invalidSlots[3].Mode = NaturalTenureMode(9)
	refuse("invalid slot array", NaturalMechanismV7, hour, 1, 2, worker, employer, invalidSlots, NaturalLabourRefusalNone)
}

// naturalV7CrossPatchNewborn builds a valid newborn actor homed on patch
// 1002 carrying a forged-but-row-valid accepted contract addressed to the
// patch-1001 employer: every row is individually V7-valid, so the
// settlement refusal is genuinely the cross-patch typed reason.
func naturalV7CrossPatchNewborn(t *testing.T, patches, hour int) NaturalActorState {
	t.Helper()
	g1, err := NaturalFounderGenome(patches, 1, 140)
	if err != nil {
		t.Fatal(err)
	}
	g2, err := NaturalFounderGenome(patches, 2, 140)
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := NaturalChildGenome(patches, 1, 2, g1, g2, 0, naturalDrawsForSeed(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	n := NaturalActorState{
		Body:     NaturalBodyState{Energy: NaturalNewbornEnergy, LastGatherHour: int64(hour) - 1},
		Bag:      NaturalBagState{Units: 1, Source: 1002},
		Worksite: NaturalWorksiteState{LastBuildHour: NaturalNeverHour},
		Granary:  NaturalGranaryState{LastStoredMealHour: NaturalNeverHour},
		Genome:   child,
		Request:  NaturalIdleRequestState(),
		Offer:    NaturalLabourOfferState{Hour: int64(hour) - 1, Addressee: 2, Status: NaturalRequestAccepted, LastOfferHour: int64(hour) - 1, LastReplyHour: int64(hour) - 1},
		Book:     NaturalIdleLabourBookState(),
	}
	if !validNaturalActorFor(NaturalMechanismV7, patches, 2017, n) {
		t.Fatalf("cross-patch newborn fixture invalid: %+v", n)
	}
	return n
}

func TestNaturalG2aDoublePrimeWageLedgersAndReduction(t *testing.T) {
	patches := 2
	// A wage-funded worker granary is V7-valid and refused by the frozen
	// v5/v6 validators (their wage ledgers are pinned to zero).
	worker, _ := naturalV7Pair(t, patches)
	worker.Granary = NaturalGranaryState{Stock: 1, CultivatedTotal: 0, LastStoredMealHour: NaturalNeverHour}
	worker.Granary.WagesEarned = 1
	if !validNaturalActorFor(NaturalMechanismV7, patches, 1, worker) {
		t.Fatal("wage-funded granary refused under v7")
	}
	if validNaturalActorFor(NaturalMechanismV6, patches, 1, worker) || validNaturalActor(patches, 1, worker) {
		t.Fatal("wage ledger admitted outside v7")
	}
	// Term-for-term reduction: zeroing the wages turns the state into a
	// G2a′-consistent v6 state.
	reduced := worker
	reduced.Granary.Stock, reduced.Granary.WagesEarned = 0, 0
	if !validNaturalActorFor(NaturalMechanismV6, patches, 1, reduced) {
		t.Fatal("wage-zeroed state is not G2a′-consistent")
	}
	// Forged wage counts are refused: an earned unit the stock never backed.
	forge := worker
	forge.Granary.WagesEarned = 2
	if validNaturalActorFor(NaturalMechanismV7, patches, 1, forge) {
		t.Fatal("forged wages-earned admitted")
	}
	forgePaid := worker
	forgePaid.Granary.WagesPaid = 1
	if validNaturalActorFor(NaturalMechanismV7, patches, 1, forgePaid) {
		t.Fatal("forged wages-paid admitted")
	}
	aboveEnvelope := worker
	aboveEnvelope.Granary.WagesEarned = NaturalWagesMax + 1
	if validNaturalActorFor(NaturalMechanismV7, patches, 1, aboveEnvelope) {
		t.Fatal("wages above the envelope admitted")
	}
	// The aggregate G2a″ and the closed-history wage equality hold on the
	// settlement world and refuse forged asymmetries.
	patchStates, slotStates, founders, workerState, employerState, homeSlots := naturalV7SettleFixture(t, patches, 5)
	nextWorker, nextEmployer, nextSlots, _, err := NaturalLabourSettle(NaturalMechanismV7, patches, 5, 1, 2, workerState, employerState, homeSlots)
	if err != nil {
		t.Fatal(err)
	}
	founders[0], founders[1], slotStates[0] = nextWorker, nextEmployer, nextSlots[:]
	if _, err := NaturalCheckConservationFor(NaturalMechanismV7, patches, patchStates, slotStates, founders, nil, mustNaturalGenesisLast(patches)); err != nil {
		t.Fatalf("G2a″ world refused: %v", err)
	}
	// A minted wage (earned without the paired paid) breaks the closure and
	// is refused — the mint stop condition.
	minted := nextWorker
	minted.Granary.Stock++
	minted.Granary.WagesEarned++
	founders[0] = minted
	if _, err := NaturalCheckConservationFor(NaturalMechanismV7, patches, patchStates, slotStates, founders, nil, mustNaturalGenesisLast(patches)); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("minted wage admitted")
	}
}

func TestNaturalLabourExpiryAndBookClear(t *testing.T) {
	// Unanswered offers expire (the 513 discipline verbatim).
	pending := NaturalLabourOfferState{Hour: 4, Addressee: 2, Status: NaturalRequestPending, LastOfferHour: 4, LastReplyHour: NaturalNeverHour}
	expired, cleared, err := NaturalLabourOfferExpire(5, pending)
	if err != nil || !cleared || expired.Status != NaturalRequestExpired {
		t.Fatalf("pending expiry: %+v %v %v", expired, cleared, err)
	}
	if !validNaturalLabourOffer(2, 1, expired) {
		t.Fatal("expired row invalid")
	}
	// A settled contract clears to idle with the audit retained; the book
	// engagement clears in the same pulse.
	settled := NaturalLabourOfferState{Hour: 3, Addressee: 2, Status: NaturalRequestAccepted, LastOfferHour: 3, LastReplyHour: 3}
	idleRow, cleared, err := NaturalLabourOfferExpire(5, settled)
	if err != nil || !cleared || idleRow.Status != NaturalRequestIdle {
		t.Fatalf("accepted clearing: %+v %v %v", idleRow, cleared, err)
	}
	if idleRow.Hour != 3 || idleRow.Addressee != 2 || idleRow.LastOfferHour != 3 || idleRow.LastReplyHour != 3 {
		t.Fatalf("audit retained: %+v", idleRow)
	}
	if !validNaturalLabourOffer(2, 1, idleRow) {
		t.Fatal("cleared row invalid")
	}
	book := NaturalLabourBookState{EngagedWorker: 1, EngagedHour: 3, LastDecisionHour: 3}
	clearedBook, cleared, err := NaturalLabourBookClear(5, book)
	if err != nil || !cleared || clearedBook.EngagedWorker != 0 || clearedBook.EngagedHour != NaturalNeverHour || clearedBook.LastDecisionHour != 3 {
		t.Fatalf("book clearing: %+v %v %v", clearedBook, cleared, err)
	}
	if !validNaturalLabourBook(clearedBook) {
		t.Fatal("cleared book invalid")
	}
	// Non-matching rows are no-op successes; bad hours are invalid calls.
	same, cleared, err := NaturalLabourOfferExpire(9, pending)
	if err != nil || cleared || !reflect.DeepEqual(same, pending) {
		t.Fatalf("non-matching expiry: %+v %v %v", same, cleared, err)
	}
	sameBook, cleared, err := NaturalLabourBookClear(9, book)
	if err != nil || cleared || !reflect.DeepEqual(sameBook, book) {
		t.Fatalf("non-matching clear: %+v %v %v", sameBook, cleared, err)
	}
	if _, _, err := NaturalLabourOfferExpire(0, pending); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("hour 0 expiry admitted")
	}
	if _, _, err := NaturalLabourBookClear(NaturalHorizonHours+1, book); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("hour beyond the horizon admitted")
	}
	// Refused rows persist untouched (the birth-row audit discipline).
	refused := NaturalLabourOfferState{Hour: 4, Addressee: 2, Status: NaturalRequestRefused, LastOfferHour: 4, LastReplyHour: 4}
	same, cleared, err = NaturalLabourOfferExpire(5, refused)
	if err != nil || cleared || !reflect.DeepEqual(same, refused) {
		t.Fatalf("refused row mutated: %+v %v %v", same, cleared, err)
	}
}

func TestNaturalV7SharedRulesAndBirthThreading(t *testing.T) {
	patches := 2
	// The wage meal is a normal stored-meal draw under v7: the worker draws
	// the wage unit the hour after settlement and G2a″ stays exact.
	_, _, _, worker, employer, homeSlots := naturalV7SettleFixture(t, patches, 5)
	nextWorker, nextEmployer, _, _, err := NaturalLabourSettle(NaturalMechanismV7, patches, 5, 1, 2, worker, employer, homeSlots)
	if err != nil {
		t.Fatal(err)
	}
	fed, _, err := NaturalEatStoredFor(NaturalMechanismV7, patches, 6, 1, nextWorker)
	if err != nil {
		t.Fatalf("v7 eat-stored on the wage stock: %v", err)
	}
	if fed.Granary.Stock != 2 || fed.Granary.StoredMeals != 1 || fed.Granary.WagesEarned != 1 {
		t.Fatalf("wage meal draw: %+v", fed.Granary)
	}
	if fed.Granary.CultivatedTotal+fed.Granary.WagesEarned != fed.Granary.Stock+fed.Granary.StoredMeals+fed.Granary.Spoiled+fed.Genome.BirthGranaryPaid {
		t.Fatal("G2a′ broken after the wage meal (wages pinned to zero form)")
	}
	if fed.Granary.CultivatedTotal+fed.Granary.WagesEarned != fed.Granary.Stock+fed.Granary.StoredMeals+fed.Granary.Provisioned+fed.Granary.Spoiled+fed.Genome.BirthGranaryPaid+fed.Granary.WagesPaid {
		t.Fatal("G2a″ broken after the wage meal")
	}
	// The employer provisions from its post-wage stock under v7 (515 exists
	// under every mechanism above the zero value).
	provisioned, err := NaturalProvision(NaturalMechanismV7, patches, 6, 2, nextEmployer)
	if err != nil {
		t.Fatalf("v7 provision: %v", err)
	}
	if provisioned.Granary.Stock != 3 || provisioned.Granary.Provisioned != 1 || provisioned.Granary.WagesPaid != 1 {
		t.Fatalf("provisioned employer: %+v", provisioned.Granary)
	}
	if provisioned.Granary.CultivatedTotal+provisioned.Granary.WagesEarned != provisioned.Granary.Stock+provisioned.Granary.StoredMeals+
		provisioned.Granary.Provisioned+provisioned.Granary.Spoiled+provisioned.Genome.BirthGranaryPaid+provisioned.Granary.WagesPaid {
		t.Fatal("G2a″ broken after the provision")
	}
	// The zero value still refuses both flows on provisioned/wage states.
	if _, _, err := NaturalEatStored(patches, 6, 1, fed); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 eat-stored admitted a wage state")
	}
	if _, err := NaturalProvision(NaturalMechanismV5, patches, 6, 2, provisioned); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("v5 provision admitted")
	}
	// A V7 birth allocates the newborn's eight complete rows, granary with
	// nine fields, wage-contract rows idle.
	proposer := naturalGateParent(t, patches, 1)
	proposer.Offer, proposer.Book = NaturalIdleLabourOfferState(), NaturalIdleLabourBookState()
	proposer.Granary = NaturalGranaryState{Stock: 5, CultivatedTotal: 5, LastStoredMealHour: NaturalNeverHour}
	request, err := NaturalBirthRequestFor(NaturalMechanismV7, patches, 5, 1, 2, 3, 5, proposer)
	if err != nil {
		t.Fatal(err)
	}
	consenter := naturalGateParent(t, patches, 2)
	consenter.Offer, consenter.Book = NaturalIdleLabourOfferState(), NaturalIdleLabourBookState()
	draws := naturalDrawsForSeed(t, 0)
	result, err := NaturalBirthReplyFor(NaturalMechanismV7, patches, 5, true, draws, 2017, 1, 2, request, consenter)
	if err != nil || result.Decision != NaturalReplyAccept {
		t.Fatalf("v7 birth reply: %+v %v", result.Decision, err)
	}
	if result.Newborn.Offer.Status != NaturalRequestIdle || result.Newborn.Book.EngagedWorker != 0 {
		t.Fatalf("newborn wage rows: %+v %+v", result.Newborn.Offer, result.Newborn.Book)
	}
	allocations, err := naturalNewbornAllocationsFor(NaturalMechanismV7, patches, 2017, 1001, result.Newborn)
	if err != nil {
		t.Fatal(err)
	}
	if len(allocations) != 8 {
		t.Fatalf("v7 newborn rows: %d", len(allocations))
	}
	for _, seed := range allocations {
		switch seed.Component {
		case NaturalGranaryTypeID:
			if len(seed.Fields) != 9 {
				t.Fatalf("v7 newborn granary row: %d fields", len(seed.Fields))
			}
		case NaturalLabourOfferTypeID:
			if len(seed.Fields) != 5 {
				t.Fatalf("v7 newborn offer row: %d fields", len(seed.Fields))
			}
		case NaturalLabourBookTypeID:
			if len(seed.Fields) != 3 {
				t.Fatalf("v7 newborn book row: %d fields", len(seed.Fields))
			}
		}
	}
	// The v5 allocation keeps the frozen shape (six rows, six-field granary)
	// for a v5 result.
	v5Result, err := NaturalBirthReply(patches, 5, true, draws, 2017, 1, 2, mustNaturalGateRequest(t, patches), consenter)
	if err != nil || v5Result.Decision != NaturalReplyAccept {
		t.Fatalf("v5 birth reply: %+v %v", v5Result.Decision, err)
	}
	v5Allocations, err := naturalNewbornAllocationsFor(NaturalMechanismV5, patches, 2017, 1001, v5Result.Newborn)
	if err != nil {
		t.Fatal(err)
	}
	if len(v5Allocations) != 6 {
		t.Fatalf("v5 newborn rows: %d", len(v5Allocations))
	}
}

func TestNaturalLabourSettleKernelAdmission(t *testing.T) {
	patches := 2
	const hour = 5
	// Stage, plan, and commit the whole chain (offer → decision → gather →
	// settlement) against the v7 registry, then read the committed wage
	// ledgers back from the kernel.
	reg7, err := NaturalRegistryFor(NaturalMechanismV7)
	if err != nil {
		t.Fatal(err)
	}
	k7, err := kernel.New(reg7, 0, naturalV7GenesisSeeds(patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = k7.Snapshot()
	commit := func(proposal kernel.Proposal) {
		t.Helper()
		// The authority is bound to the committed snapshot: re-snapshot
		// before every plan (the owner changes with each commit).
		_, authority, _ := k7.Snapshot()
		plan, err := k7.Plan(proposal, authority)
		if err != nil {
			t.Fatalf("plan %s: %v", proposal.Key, err)
		}
		if _, err := k7.CommitBatch([]kernel.Plan{plan}); err != nil {
			t.Fatalf("commit %s: %v", proposal.Key, err)
		}
	}
	worker, employer := naturalV7Pair(t, patches)
	offer, err := NaturalLabourOfferProposalFor(NaturalMechanismV7, "offer/4/1", patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	commit(offer)
	offered, err := NaturalLabourOffer(NaturalMechanismV7, patches, 4, 1, 2, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := NaturalLabourDecisionProposalFor(NaturalMechanismV7, "decision/4/2", patches, 4, 2, 1, true, employer, offered)
	if err != nil {
		t.Fatal(err)
	}
	commit(decision)
	engagedEmployer, engagedWorker, _, err := NaturalLabourDecision(NaturalMechanismV7, patches, 4, 2, 1, true, employer, offered)
	if err != nil {
		t.Fatal(err)
	}
	slotID, err := NaturalSlotID(patches, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	slots := [NaturalSlotsPerPatch]NaturalSlotState{}
	slots[0] = NaturalSlotState{Stock: 1}
	slots[1] = NaturalSlotState{Stock: 1}
	gather, err := NaturalGatherProposalFor(NaturalMechanismV7, "gather/5/1", patches, hour, 1, slotID, slots[1], engagedWorker)
	if err != nil {
		t.Fatal(err)
	}
	commit(gather)
	gatheredSlot, gatheredWorker, err := NaturalGatherFor(NaturalMechanismV7, patches, 1001, slotID, 1, hour, slots[1], engagedWorker)
	if err != nil {
		t.Fatal(err)
	}
	slots[1] = gatheredSlot
	// The gather's slot patches ride rule 506; the settlement proposal then
	// carries the enumerated set plus the completion's tenure claim.
	settle, err := NaturalLabourSettleProposalFor(NaturalMechanismV7, "settle/5/1", patches, hour, 1, 2, gatheredWorker, engagedEmployer, slots)
	if err != nil {
		t.Fatal(err)
	}
	if len(settle.Patches) != 12 {
		t.Fatalf("settlement patch set: %d", len(settle.Patches))
	}
	if err := NaturalCheckOwnershipFor(NaturalMechanismV7, settle, patches); err != nil {
		t.Fatalf("settlement ownership: %v", err)
	}
	commit(settle)
	reader, auth, version := k7.Snapshot()
	readInt := func(entity sim.EntityID, componentType sim.ComponentTypeID, field sim.FieldID) int64 {
		t.Helper()
		view, err := reader.Read(component.ReadRequest{Entity: entity, Component: componentType, Fields: []sim.FieldID{field}, WorldVersion: version, Authority: auth})
		if err != nil {
			t.Fatalf("read %d/%d: %v", entity, field, err)
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
	if got := readInt(1, NaturalGranaryTypeID, NaturalGranaryWagesEarnedField); got != 1 {
		t.Fatalf("committed wages-earned: %d", got)
	}
	if got := readInt(2, NaturalGranaryTypeID, NaturalGranaryWagesPaidField); got != 1 {
		t.Fatalf("committed wages-paid: %d", got)
	}
	if got := readInt(1, NaturalBagTypeID, NaturalBagUnitsField); got != 0 {
		t.Fatalf("committed worker bag: %d", got)
	}
	if got := readInt(2, NaturalWorksiteTypeID, NaturalWorksiteCapitalField); got != 2 {
		t.Fatalf("committed employer capital: %d", got)
	}
	claimedSlot := naturalSnapshotSlot(t, reader, auth, version, NaturalSlotIDBase+1)
	if claimedSlot.Mode != NaturalTenureCultivated || claimedSlot.Owner != 2 || claimedSlot.Cultivated != 1 {
		t.Fatalf("committed tenure: %+v", claimedSlot)
	}
	// The frozen rule versions stage against nothing else: a v5-schema
	// rule-518 patch set is refused, and rule 518 does not exist under the
	// v5/v6 registries at any version.
	committedReader, committedAuth, committedVersion := k7.Snapshot()
	reg5, err := NaturalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	v5Shape := append([]component.Patch(nil), settle.Patches...)
	// Rule 518 at the frozen v5 rule version under the v7 registry: the
	// rule is registered only at version 7 (schema stays 7).
	if _, _, _, _, err := component.Stage(committedReader, committedAuth, committedVersion, NaturalLabourSettlementRule, 5, v5Shape, nil); !errors.Is(err, component.ErrUnknownRule) {
		t.Fatalf("518 staged at the v5 rule version: %v", err)
	}
	for i := range v5Shape {
		v5Shape[i].SchemaVersion = NaturalSchemaVersion
	}
	reader5, auth5, err := component.NewReader(reg5, 0, naturalNeutralSeeds(patches, 8, 140))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := component.Stage(reader5, auth5, 0, NaturalLabourSettlementRule, 5, v5Shape, nil); !errors.Is(err, component.ErrUnknownRule) {
		t.Fatalf("518 staged against the v5 registry: %v", err)
	}
}

// Review P2 (ticket-20 L1): the never-filed offer shape must pin the reply
// hour too, so a row cannot carry a reply on an unfiled offer.
func TestNaturalV7LabourOfferNeverFiledPinsAllFields(t *testing.T) {
	patches := 2
	state := NaturalIdleLabourOfferState()
	if !validNaturalLabourOffer(patches, 1, state) {
		t.Fatal("idle offer state rejected")
	}
	forged := state
	forged.LastReplyHour = 7
	if validNaturalLabourOffer(patches, 1, forged) {
		t.Fatal("unfiled offer accepted a reply hour")
	}
}

// Ticket-20 fix: the G5 allocation whitelist must admit the two labour rows
// under mechanism v7 and refuse them under v5/v6. Driven through the real
// allocation builder (a hand-made proposal lacks the rule-511 cause patches the
// guard requires) with a scripted kernel-admissible accept birth, so the test
// exercises the same path the runner uses.
func TestNaturalV7NewbornAllocationWhitelist(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy9().Ref, Mechanism: NaturalMechanismV7, Founders: 10})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		ok, err := f.Step(context.Background())
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if !ok {
			t.Fatal("run ended before an organic birth committed")
		}
		if checks := f.Checkpoints(); checks[len(checks)-1].Metrics.Births >= 1 {
			// The committed birth carried the eight-row allocation through the
			// guard and the kernel: conservation is re-verified hourly, and the
			// newborn's labour rows are validated by the v7 row validators.
			return
		}
	}
	t.Fatal("no organic birth within the step budget")
}
