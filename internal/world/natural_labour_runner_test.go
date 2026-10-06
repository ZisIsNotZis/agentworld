package world

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The natural/9 runner and persistence lane (ticket 20 L3). The frozen
// design (.scratch/20-exchange-design/spec.md) is binding: mechanism v7 is
// the wage-contract axis, policy natural/9 its only selector, and every
// labour wake sits on the frozen hourly lattice (516 at h+40m+5µs, 517 at
// h+40m+6µs, 518 at the h+40m+3µs build-completion instant, 519 world-caused
// at the pulse) — no new phase instants. Two supervisor decisions are
// recorded pre-run in the ticket and pinned here: (1) the completion-instant
// fold declares settlement precedence — settlements fold first in ascending
// worker ID, then the surviving builders' own rule-505 completions, and any
// flow whose enumerated field set intersects an earlier-folded winner is
// voided before it is proposed (typed labour-void / build note
// "settlement-precedence"), so no collision can silently discard a
// settlement; (2) the negotiation budget is wage-lane-first at both shared
// instants (516 before 510 at +5µs, 517 before 511/512 at +6µs, at most one
// filing and one decision per actor-hour).

func naturalLabourV9Options(seed uint64, workers int, births bool) NaturalOptions {
	return NaturalOptions{Patches: 2, R: 8, Seed: seed, Workers: workers, Births: births,
		PolicyRef: strategy.FrozenNaturalPolicy9().Ref, Mechanism: NaturalMechanismV7}
}

func naturalLabourV7bOptions(seed uint64, workers int, births bool, mechanism NaturalMechanism) NaturalOptions {
	return NaturalOptions{Patches: 2, R: 8, Seed: seed, Workers: workers, Births: births,
		PolicyRef: strategy.FrozenNaturalPolicy7b().Ref, Mechanism: mechanism}
}

// TestNaturalV7RefMechanismLadder pins every rung of the ref/mechanism
// ladder at every construction seam: natural/5|6 legal under any admitted
// mechanism (v7 included, the zero-use control), natural/7|8 only under
// mechanism >= v6, natural/9 ONLY under mechanism v7, unknown refs nowhere.
func TestNaturalV7RefMechanismLadder(t *testing.T) {
	ref5 := strategy.FrozenNaturalPolicy().Ref
	ref6 := strategy.FrozenNaturalPolicyV6().Ref
	ref7 := strategy.FrozenNaturalPolicy7().Ref
	ref7b := strategy.FrozenNaturalPolicy7b().Ref
	ref9 := strategy.FrozenNaturalPolicy9().Ref
	type rung struct {
		ref                      strategy.NaturalRef
		legalUnder               []NaturalMechanism
		typedRefusalUnder        []NaturalMechanism
		typedRefusalUnderSpelled []NaturalMechanism // explicit non-zero spellings
	}
	ladder := []rung{
		{ref: ref5, legalUnder: []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7}},
		{ref: ref6, legalUnder: []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7}},
		{ref: ref7, legalUnder: []NaturalMechanism{NaturalMechanismV6, NaturalMechanismV7}, typedRefusalUnderSpelled: []NaturalMechanism{NaturalMechanismV5}},
		{ref: ref7b, legalUnder: []NaturalMechanism{NaturalMechanismV6, NaturalMechanismV7}, typedRefusalUnderSpelled: []NaturalMechanism{NaturalMechanismV5}},
		{ref: ref9, legalUnder: []NaturalMechanism{NaturalMechanismV7}, typedRefusalUnderSpelled: []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6}},
	}
	for _, tc := range ladder {
		for _, mechanism := range tc.legalUnder {
			f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: tc.ref, Mechanism: mechanism})
			if err != nil {
				t.Fatalf("ref %+v under mechanism %d refused: %v", tc.ref, uint16(mechanism), err)
			}
			if f.ref != tc.ref || f.mechanism != mechanism {
				t.Fatalf("runner bound %+v/%d, want %+v/%d", f.ref, f.mechanism, tc.ref, mechanism)
			}
			if err := (NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: tc.ref, Mechanism: mechanism}).validate(); err != nil {
				t.Fatalf("restore expectation %+v/%d refused: %v", tc.ref, mechanism, err)
			}
			if !validNaturalJournalConfig(NaturalJournalConfig{Patches: 2, R: 8, Seed: 0, Policy: tc.ref, Mechanism: mechanism}) {
				t.Fatalf("journal config %+v/%d refused", tc.ref, mechanism)
			}
		}
		for _, mechanism := range tc.typedRefusalUnderSpelled {
			if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: tc.ref, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
				t.Fatalf("ref %+v accepted under mechanism %d", tc.ref, uint16(mechanism))
			}
			if err := (NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: tc.ref, Mechanism: mechanism}).validate(); !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatalf("restore expectation %+v/%d validated", tc.ref, mechanism)
			}
			if validNaturalJournalConfig(NaturalJournalConfig{Patches: 2, R: 8, Seed: 0, Policy: tc.ref, Mechanism: mechanism}) {
				t.Fatalf("journal config %+v/%d accepted", tc.ref, mechanism)
			}
		}
	}
	// The zero value is the v5 spelling everywhere: natural/9 under the
	// zero-value mechanism is the v5 refusal.
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref9}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatal("natural/9 accepted under the zero-value mechanism")
	}
	// Unknown refs fail under every mechanism, including v7.
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6, NaturalMechanismV7} {
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: strategy.NaturalRef{ID: "natural", Version: 10}, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("natural/10 accepted under mechanism %d", uint16(mechanism))
		}
	}
}

// TestNaturalV7GenesisSeedsAndActorRows pins the mechanism-7 genesis: every
// founder seeds a nine-field granary (the wage ledgers included) and the two
// idle wage-contract rows, the runner's actor reads see exactly those rows,
// and the v5/v6 genesis field lists stay frozen (six and seven granary
// fields, no labour rows).
func TestNaturalV7GenesisSeedsAndActorRows(t *testing.T) {
	readActor := func(t *testing.T, mechanism NaturalMechanism, ref strategy.NaturalRef) (NaturalActorState, kernel.Head) {
		t.Helper()
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: mechanism})
		if err != nil {
			t.Fatalf("mechanism %d construct: %v", mechanism, err)
		}
		head := f.k.SnapshotHead()
		view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
		a, err := naturalActorState(view, mechanism, 1)
		if err != nil {
			t.Fatalf("mechanism %d actor read: %v", mechanism, err)
		}
		return a, head
	}
	v5, _ := readActor(t, NaturalMechanismV5, strategy.FrozenNaturalPolicy().Ref)
	if v5.Granary.Provisioned != 0 || v5.Granary.WagesPaid != 0 || v5.Granary.WagesEarned != 0 ||
		v5.Offer != (NaturalLabourOfferState{}) || v5.Book != (NaturalLabourBookState{}) {
		t.Fatalf("v5 actor rows drifted: %+v", v5)
	}
	v6, _ := readActor(t, NaturalMechanismV6, strategy.FrozenNaturalPolicyV6().Ref)
	if v6.Granary.Provisioned != 0 || v6.Granary.WagesPaid != 0 || v6.Granary.WagesEarned != 0 {
		t.Fatalf("v6 granary rows drifted: %+v", v6.Granary)
	}
	v7, head := readActor(t, NaturalMechanismV7, ref7bForTest())
	if v7.Granary.Provisioned != 0 || v7.Granary.WagesPaid != 0 || v7.Granary.WagesEarned != 0 {
		t.Fatalf("v7 granary not seeded at zero: %+v", v7.Granary)
	}
	if v7.Offer != NaturalIdleLabourOfferState() {
		t.Fatalf("v7 offer row not idle: %+v", v7.Offer)
	}
	if v7.Book != NaturalIdleLabourBookState() {
		t.Fatalf("v7 book row not idle: %+v", v7.Book)
	}
	// The nine-field granary in the seeds: every field of the v7 granary
	// descriptor reads back at genesis (a missing field would fail the
	// read above; the count pin keeps the row complete).
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	granary, err := capacityInts(view, 1, NaturalGranaryTypeID,
		NaturalGranaryStockField, NaturalGranaryCultivatedTotalField, NaturalGranaryStoredMealsField,
		NaturalGranaryLastStoredMealHourField, NaturalGranarySpoilDebtField, NaturalGranarySpoiledField,
		NaturalGranaryProvisionedField, NaturalGranaryWagesPaidField, NaturalGranaryWagesEarnedField)
	if err != nil || len(granary) != 9 {
		t.Fatalf("v7 granary row is not nine fields: %v %v", granary, err)
	}
	// And the (v7, natural/7b) zero-use control and the (v7, natural/9)
	// world both step the frozen lattice.
	for _, ref := range []strategy.NaturalRef{strategy.FrozenNaturalPolicy7b().Ref, strategy.FrozenNaturalPolicy9().Ref} {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV7})
		if err != nil {
			t.Fatalf("v7/%+v construct: %v", ref, err)
		}
		naturalRunTo(t, f, 3)
	}
}

func ref7bForTest() strategy.NaturalRef { return strategy.FrozenNaturalPolicy7b().Ref }

// TestNaturalLabourRunnerEndToEndSettlement drives a full (v7, natural/9)
// organic run and pins the settlement discipline end to end: the journal
// chain 516 → 517-accept → 518 within one hour (the V9-T2 band), the
// committed event's enumerated patch set, the wage ledgers read back from
// the committed state, and never a silently discarded settlement.
func TestNaturalLabourRunnerEndToEndSettlement(t *testing.T) {
	f, err := NewNatural(naturalLabourV9Options(0, 2, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	journal := f.Journal()
	settlements := 0
	for _, b := range journal {
		for _, a := range b.Attempts {
			if a.Kind == "labour-settlement" {
				settlements++
				if !a.Accepted || a.EventID == 0 {
					t.Fatal("a settlement attempt was silently discarded by the kernel batch")
				}
			}
		}
	}
	if settlements == 0 {
		t.Fatal("no settlement committed in the designated cell")
	}
	// Chain discipline: every settlement's offer was filed in the previous
	// hour and decided (accepted) in that same hour, so the chain
	// 516 → 517-accept → 518 spans at most 2 hours, and the offer/decision
	// instants are the frozen +5µs/+6µs phases.
	// The offer event names its addressee; the accept decision is the
	// addressee's rule-517 event in the filing hour turning this worker's
	// pending row accepted. Events commit in order, so walking them
	// chronologically keeps each worker's latest offer and accept; every
	// settlement must chain through the two events of its filing hour.
	latestOffer := map[sim.EntityID]kernel.Event{}
	latestAccept := map[sim.EntityID]kernel.Event{}
	settled := 0
	for _, ev := range f.k.Events() {
		switch ev.Rule {
		case NaturalLabourOfferRule:
			latestOffer[ev.Cause.Actor] = ev
		case NaturalLabourDecisionRule:
			for _, d := range ev.Deltas {
				if d.Component == NaturalLabourOfferTypeID && d.Field == NaturalLabourOfferStatusField {
					after, _ := d.After.Integer()
					if after == int64(NaturalRequestAccepted) {
						latestAccept[d.Entity] = ev
					}
				}
			}
		case NaturalLabourSettlementRule:
			worker := ev.Cause.Actor
			settleHour := int64(ev.Time) / int64(NaturalHour)
			settled++
			offer, ok := latestOffer[worker]
			if !ok || int64(offer.Time)/int64(NaturalHour) != settleHour-1 {
				t.Fatalf("settlement for actor %d at hour %d has no previous-hour offer", worker, settleHour)
			}
			var addressee sim.EntityID
			for _, d := range offer.Deltas {
				if d.Field == NaturalLabourOfferAddresseeField {
					addressee, _ = naturalDeltaRef(d.After)
				}
			}
			accept, ok := latestAccept[worker]
			if !ok || accept.Cause.Actor != addressee || int64(accept.Time)/int64(NaturalHour) != settleHour-1 {
				t.Fatalf("settlement for actor %d at hour %d has no same-hour accept by addressee %d", worker, settleHour, addressee)
			}
		}
	}
	if settled == 0 {
		t.Fatal("no rule-518 event in the designated cell")
	}
	// The committed wage ledgers read back from the final checkpoint: the
	// aggregate wage units are conserved and non-zero, and every settlement
	// moved exactly one unit each way (Σ paid == Σ earned == settlements).
	last := f.Checkpoints()[len(f.Checkpoints())-1]
	var paid, earned int64
	for _, founder := range last.Founders {
		paid += founder.Granary.WagesPaid
		earned += founder.Granary.WagesEarned
	}
	for _, newborn := range last.Newborns {
		paid += newborn.State.Granary.WagesPaid
		earned += newborn.State.Granary.WagesEarned
	}
	if paid != int64(settlements) || earned != int64(settlements) {
		t.Fatalf("wage ledgers %d/%d diverge from %d settlements", paid, earned, settlements)
	}
}

// TestNaturalLabourWakeInstantsAndBudgets pins, over a full (v7, natural/9)
// run: every labour rule commits at exactly its frozen lattice instant (no
// new phase instants), the negotiation budgets hold (at most one filing and
// one decision per actor-hour, wage lane first), and the activity budget
// holds (one gather; one build or one delivery per actor-hour).
func TestNaturalLabourWakeInstantsAndBudgets(t *testing.T) {
	f, err := NewNatural(naturalLabourV9Options(0, 2, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	head := f.k.SnapshotHead()
	type budgetKey struct {
		actor sim.EntityID
		hour  int64
	}
	filings := map[budgetKey]int{}
	decisions := map[budgetKey]int{}
	gathers := map[budgetKey]int{}
	conversions := map[budgetKey]int{}
	phase := struct{ offer, decision, settle int64 }{
		offer:    int64(NaturalGatherDuration + NaturalBuildDuration + 5),
		decision: int64(NaturalGatherDuration + NaturalBuildDuration + 6),
		settle:   int64(NaturalGatherDuration + NaturalBuildDuration + 3),
	}
	for _, ev := range f.k.Events() {
		hourStart := int64(ev.Time) / int64(NaturalHour) * int64(NaturalHour)
		switch ev.Rule {
		case NaturalLabourOfferRule:
			if int64(ev.Time)-hourStart != phase.offer {
				t.Fatalf("rule 516 committed off the +5µs phase at %d", ev.Time)
			}
		case NaturalLabourDecisionRule:
			if int64(ev.Time)-hourStart != phase.decision {
				t.Fatalf("rule 517 committed off the +6µs phase at %d", ev.Time)
			}
		case NaturalLabourSettlementRule:
			if int64(ev.Time)-hourStart != phase.settle {
				t.Fatalf("rule 518 committed off the completion instant at %d", ev.Time)
			}
		case NaturalLabourExpireRule:
			if int64(ev.Time) != hourStart {
				t.Fatalf("rule 519 committed off the pulse at %d", ev.Time)
			}
		}
	}
	for _, b := range f.Journal() {
		hour := int64(b.Time) / int64(NaturalHour)
		for _, a := range b.Attempts {
			key := budgetKey{a.Actor, hour}
			switch a.Kind {
			case "birth-request", "labour-offer":
				filings[key]++
			case "birth-reply", "labour-decision":
				decisions[key]++
			case "gather":
				if a.Accepted {
					gathers[key]++
				}
			case "build":
				if a.Accepted {
					conversions[key]++
				}
			case "labour-settlement":
				conversions[key]++
			}
		}
	}
	for key, n := range filings {
		if n > 1 {
			t.Fatalf("actor %d filed %d negotiations in hour %d", key.actor, n, key.hour)
		}
	}
	for key, n := range decisions {
		if n > 1 {
			t.Fatalf("actor %d decided %d negotiations in hour %d", key.actor, n, key.hour)
		}
	}
	for key, n := range gathers {
		if n > 1 {
			t.Fatalf("actor %d gathered %d units in hour %d", key.actor, n, key.hour)
		}
	}
	for key, n := range conversions {
		if n > 1 {
			t.Fatalf("actor %d converted %d units in hour %d", key.actor, n, key.hour)
		}
	}
	if head.Version == 0 {
		t.Fatal("no events committed")
	}
}

// TestNaturalLabourSettlementStructuralDiscipline walks a full run's
// accepted events: every rule-518 event carries exactly the enumerated
// two-party patch set (7 patches, 12 with the completion's tenure claim), a
// partial settlement is structurally impossible, tenure is written only by
// the admitted rules with the settlement's triple riding a completion and
// naming the employer, and Σ wages-paid == Σ wages-earned in every hour.
func TestNaturalLabourSettlementStructuralDiscipline(t *testing.T) {
	f, err := NewNatural(naturalLabourV9Options(0, 2, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	tenureFields := map[sim.FieldID]bool{
		NaturalSlotTenureModeField: true, NaturalSlotTenureOwnerField: true, NaturalSlotTenureCultivatedField: true}
	settlements := 0
	hourPaid := map[int64]int64{}
	hourEarned := map[int64]int64{}
	for _, ev := range f.k.Events() {
		hour := int64(ev.Time) / int64(NaturalHour)
		if ev.Rule != NaturalLabourSettlementRule {
			// G5: tenure is writable only by rules 503 wear, 505 build,
			// 514 age-death revert, and — mechanism v7 — the settlement's
			// completion claim (rule 518, handled below).
			switch ev.Rule {
			case NaturalWearRule, NaturalBuildRule, NaturalAgeDeathRule:
			default:
				for _, d := range ev.Deltas {
					if d.Component == NaturalSlotTypeID && tenureFields[d.Field] {
						t.Fatalf("rule %d mutated slot tenure at hour %d", ev.Rule, hour)
					}
				}
			}
			continue
		}
		settlements++
		if len(ev.Deltas) != 7 && len(ev.Deltas) != 12 {
			t.Fatalf("settlement event at hour %d carries %d deltas, want exactly 7 or 12", hour, len(ev.Deltas))
		}
		fieldsByComponent := map[sim.ComponentTypeID]map[sim.FieldID]bool{
			NaturalBagTypeID:      {NaturalBagUnitsField: true},
			NaturalWorksiteTypeID: {NaturalWorksiteCapitalField: true, NaturalWorksiteWipField: true, NaturalWorksiteInvestedUnitsField: true, NaturalWorksitePointsCreatedField: true},
			NaturalGranaryTypeID:  {NaturalGranaryStockField: true, NaturalGranaryWagesPaidField: true, NaturalGranaryWagesEarnedField: true},
			NaturalSlotTypeID:     tenureFields,
		}
		tenured := false
		for _, d := range ev.Deltas {
			fields, ok := fieldsByComponent[d.Component]
			if !ok || !fields[d.Field] {
				t.Fatalf("settlement event at hour %d writes foreign field %d/%d", hour, d.Component, d.Field)
			}
			if d.Component == NaturalSlotTypeID {
				tenured = true
				if d.Field == NaturalSlotTenureOwnerField {
					owner, err := naturalDeltaRef(d.After)
					if err != nil || owner == ev.Cause.Actor {
						t.Fatalf("settlement tenure owner %d is not the employer at hour %d", owner, hour)
					}
				}
			}
			switch d.Field {
			case NaturalGranaryWagesPaidField:
				n, _ := d.After.Integer()
				before, _ := d.Before.Integer()
				hourPaid[hour] += n - before
			case NaturalGranaryWagesEarnedField:
				n, _ := d.After.Integer()
				before, _ := d.Before.Integer()
				hourEarned[hour] += n - before
			}
		}
		if tenured && len(ev.Deltas) != 12 {
			t.Fatalf("tenure claim without the completion shape at hour %d", hour)
		}
		if !tenured && len(ev.Deltas) != 7 {
			t.Fatalf("completion shape without tenure at hour %d", hour)
		}
	}
	for hour, paid := range hourPaid {
		if paid != hourEarned[hour] {
			t.Fatalf("hour %d wage conservation broken: paid %d != earned %d", hour, paid, hourEarned[hour])
		}
	}
	if settlements == 0 {
		t.Fatal("no settlement in the designated cell")
	}
	// The contract taxonomy is unchanged by the lane: the seven typed
	// refusal reasons keep their frozen note spellings, and the only
	// runner-level structural note is the recorded precedence note.
	if naturalLabourRefusalNote(NaturalLabourRefusalNone) != "" ||
		naturalLabourRefusalNote(NaturalLabourRefusalContractNotPending) != "contract-not-pending" ||
		naturalLabourRefusalNote(NaturalLabourRefusalWorkerDeadOrCrossPatch) != "worker-dead-or-cross-patch" ||
		naturalLabourRefusalNote(NaturalLabourRefusalEmployerDeadOrCrossPatch) != "employer-dead-or-cross-patch" ||
		naturalLabourRefusalNote(NaturalLabourRefusalWorkerBagEmpty) != "worker-bag-empty" ||
		naturalLabourRefusalNote(NaturalLabourRefusalEmployerStockShort) != "employer-stock-short" ||
		naturalLabourRefusalNote(NaturalLabourRefusalEmployerAtKMax) != "employer-at-kmax" ||
		naturalLabourRefusalNote(NaturalLabourRefusalNoFreeSlot) != "no-free-slot" {
		t.Fatal("contract refusal note taxonomy drifted")
	}
}

// TestNaturalLabourVoidsZeroMutationAndDenialPath pins the typed void
// discipline end to end: every void is keyless with a typed note, commits
// nothing (no rule-518 event, no committed conversion in that actor-hour),
// and a voided worker still holding its unit re-enters the documented
// denial path (the paired-instant re-eat).
func TestNaturalLabourVoidsZeroMutationAndDenialPath(t *testing.T) {
	f, err := NewNatural(naturalLabourV9Options(0, 2, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	type voidKey struct {
		actor sim.EntityID
		hour  int64
	}
	voids := map[voidKey]string{}
	settlementHours := map[voidKey]bool{}
	denialEats := map[voidKey]bool{}
	for _, b := range f.Journal() {
		hour := int64(b.Time) / int64(NaturalHour)
		for _, a := range b.Attempts {
			switch {
			case a.Kind == "labour-void":
				if a.Key != "" || a.Refused == "" || a.Accepted || a.EventID != 0 {
					t.Fatalf("void attempt is not keyless typed evidence: %+v", a)
				}
				voids[voidKey{a.Actor, hour}] = a.Refused
			case a.Kind == "labour-settlement":
				settlementHours[voidKey{a.Actor, hour}] = true
			case a.Kind == "build-denied-eat":
				denialEats[voidKey{a.Actor, hour}] = true
			}
		}
	}
	if len(voids) == 0 {
		t.Fatal("no typed void in the designated cell")
	}
	for key := range voids {
		if settlementHours[key] {
			t.Fatalf("actor %d hour %d recorded both a void and a settlement", key.actor, key.hour)
		}
	}
	// A voided worker that kept its unit re-eats at the paired instant; a
	// voided worker whose gather was denied has no unit and the frozen
	// denied-gather fallback already decided the hour (rest or deferred
	// meal). Every void therefore carries either the paired re-eat or a
	// denied gather in the same hour.
	deniedGathers := map[voidKey]bool{}
	for _, b := range f.Journal() {
		hour := int64(b.Time) / int64(NaturalHour)
		for _, a := range b.Attempts {
			if a.Kind == "gather" && !a.Accepted {
				deniedGathers[voidKey{a.Actor, hour}] = true
			}
		}
	}
	for key := range voids {
		if !denialEats[key] && !deniedGathers[key] {
			t.Fatalf("void for actor %d hour %d entered neither the denial re-eat nor the denied-gather fallback", key.actor, key.hour)
		}
	}
}

// TestNaturalLabourLifecycle pins the frozen status lifecycle over a full
// run: an offer lives only inside its filing hour (decided same-hour by
// rule 517 or expired at the next pulse by rule 519), accept/refuse are
// same-hour, an accepted contract has exactly one delivery hour with at
// most one settlement attempt in it, and accept and engagement clear
// together at the accept+2 pulse.
func TestNaturalLabourLifecycle(t *testing.T) {
	f, err := NewNatural(naturalLabourV9Options(0, 2, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	type offerKey struct {
		worker sim.EntityID
		hour   int64
	}
	filed := map[offerKey]bool{}
	decided := map[offerKey]bool{}
	expired := map[offerKey]bool{}
	accepted := map[offerKey]bool{}
	settleAttempts := map[offerKey]int{}
	acceptHours := map[sim.EntityID][]int64{}
	for _, b := range f.Journal() {
		hour := int64(b.Time) / int64(NaturalHour)
		for _, a := range b.Attempts {
			switch a.Kind {
			case "labour-offer":
				filed[offerKey{a.Actor, hour}] = true
			case "labour-decision":
				decided[offerKey{a.Actor, hour}] = true
			case "labour-void":
				if a.Refused != naturalLabourEngagementBusyNote {
					continue
				}
				// A busy-book accept void leaves the offer pending; it
				// expires at the next pulse like an unanswered offer.
			case "labour-settlement":
				settleAttempts[offerKey{a.Actor, hour}]++
			}
		}
	}
	for _, ev := range f.k.Events() {
		hour := int64(ev.Time) / int64(NaturalHour)
		switch ev.Rule {
		case NaturalLabourDecisionRule:
			for _, d := range ev.Deltas {
				if d.Component != NaturalLabourOfferTypeID || d.Field != NaturalLabourOfferStatusField {
					continue
				}
				before, _ := d.Before.Integer()
				after, _ := d.After.Integer()
				if before != int64(NaturalRequestPending) {
					t.Fatalf("rule 517 decided a non-pending offer at hour %d", hour)
				}
				worker := d.Entity
				decided[offerKey{worker, hour}] = true
				if after == int64(NaturalRequestAccepted) {
					accepted[offerKey{worker, hour}] = true
					acceptHours[worker] = append(acceptHours[worker], hour)
				}
			}
		case NaturalLabourExpireRule:
			for _, d := range ev.Deltas {
				if d.Component != NaturalLabourOfferTypeID || d.Field != NaturalLabourOfferStatusField {
					continue
				}
				before, _ := d.Before.Integer()
				after, _ := d.After.Integer()
				worker := d.Entity
				if before == int64(NaturalRequestPending) && after == int64(NaturalRequestExpired) {
					expired[offerKey{worker, hour - 1}] = true // the filing hour
				}
				if before == int64(NaturalRequestAccepted) {
					if after != int64(NaturalRequestIdle) {
						t.Fatalf("accepted offer cleared to %d at hour %d", after, hour)
					}
					// The clear happens exactly at the accept+2 pulse with
					// the engagement cleared by the same event or its twin.
				}
			}
		}
	}
	for key := range filed {
		if !decided[key] && !expired[key] {
			t.Fatalf("offer of worker %d filed in hour %d was neither decided same-hour nor expired at the next pulse", key.worker, key.hour)
		}
	}
	for key := range accepted {
		if settled := settleAttempts[key]; settled > 1 {
			t.Fatalf("accepted contract of worker %d (hour %d) had %d settlement attempts in one delivery hour", key.worker, key.hour, settled)
		}
		if settled := settleAttempts[offerKey{key.worker, key.hour + 1}]; settled > 1 {
			t.Fatalf("accepted contract of worker %d (hour %d) had %d settlement attempts in its delivery hour", key.worker, key.hour, settled)
		}
	}
	// Engagement bookkeeping: every accept engages the book at the decision
	// hour and the book clears at the accept+2 pulse.
	for _, ev := range f.k.Events() {
		if ev.Rule != NaturalLabourDecisionRule {
			continue
		}
		hour := int64(ev.Time) / int64(NaturalHour)
		engaged := false
		for _, d := range ev.Deltas {
			if d.Component == NaturalLabourBookTypeID && d.Field == NaturalLabourBookEngagedWorkerField {
				if id, err := naturalDeltaRef(d.After); err != nil || id == 0 {
					t.Fatalf("accept engaged a zero worker at hour %d", hour)
				}
				engaged = true
			}
		}
		if !engaged {
			continue
		}
		cleared := false
		for _, other := range f.k.Events() {
			ohour := int64(other.Time) / int64(NaturalHour)
			if other.Rule != NaturalLabourExpireRule || ohour != hour+2 {
				continue
			}
			for _, d := range other.Deltas {
				if d.Component == NaturalLabourBookTypeID && d.Field == NaturalLabourBookEngagedWorkerField {
					if id, err := naturalDeltaRef(d.After); err == nil && id == 0 {
						cleared = true
					}
				}
			}
		}
		if !cleared {
			t.Fatalf("engagement accepted in hour %d never cleared at the accept+2 pulse", hour)
		}
	}
	_ = acceptHours
}

// naturalLabourHybrid builds a (mechanism v7, natural/9) hybrid world over
// custom seeds for direct fold fixtures (the frozen v5 hybrid pattern): no
// scheduler choreography, the fold and the decision helpers called
// directly on constructed kernel state.
func naturalLabourHybrid(t *testing.T) (*Natural, *kernel.Kernel) {
	t.Helper()
	seeds, _, err := naturalGenesisSeeds(NaturalMechanismV7, 2, 8, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return naturalLabourHybridSeeds(t, seeds)
}

// naturalLabourRewrite seeds one integer field of one component row.
func naturalLabourRewrite(t *testing.T, seeds []component.ComponentSeed, entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64) {
	t.Helper()
	hit := false
	for i := range seeds {
		if seeds[i].Entity != entity || seeds[i].Component != componentType {
			continue
		}
		hit = true
		for j := range seeds[i].Fields {
			if v, ok := values[seeds[i].Fields[j].Field]; ok {
				seeds[i].Fields[j].Value = sim.IntegerValue(v)
			}
		}
	}
	if !hit {
		t.Fatalf("no seed row for entity %d component %d", entity, componentType)
	}
}

// naturalLabourRewriteRef seeds one entity-ref field of one component row.
func naturalLabourRewriteRef(t *testing.T, seeds []component.ComponentSeed, entity sim.EntityID, componentType sim.ComponentTypeID, field sim.FieldID, target sim.EntityID) {
	t.Helper()
	var value sim.Value
	var err error
	if target == 0 {
		value, err = sim.AbsentValue(sim.EntityRefKind, sim.Missing)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		value, err = sim.EntityRefValue(target)
		if err != nil {
			t.Fatal(err)
		}
	}
	hit := false
	for i := range seeds {
		if seeds[i].Entity != entity || seeds[i].Component != componentType {
			continue
		}
		hit = true
		for j := range seeds[i].Fields {
			if seeds[i].Fields[j].Field == field {
				seeds[i].Fields[j].Value = value
			}
		}
	}
	if !hit {
		t.Fatalf("no seed row for entity %d component %d", entity, componentType)
	}
}

// naturalLabourEngageSeeds writes the hour-4 engagement of worker →
// employer (offer accepted at hour 4, book engaged) plus the delivery-hour
// body/bag margins, directly into the seeds: the worker holds one gathered
// unit (LastGatherHour 5), the employer holds employerStock with
// employerWip/employerCapital worksite progress, both alive at energy
// 5/basal 6 (the frozen G3 identity 11 = 5 + 6).
func naturalLabourEngageSeeds(t *testing.T, seeds []component.ComponentSeed, worker, employer sim.EntityID, employerStock, employerWip, employerCapital int64) {
	t.Helper()
	naturalLabourRewrite(t, seeds, worker, NaturalBodyTypeID, map[sim.FieldID]int64{
		NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6, NaturalBodyLastGatherHourField: 5})
	naturalLabourRewrite(t, seeds, worker, NaturalBagTypeID, map[sim.FieldID]int64{NaturalBagUnitsField: 1})
	naturalLabourRewrite(t, seeds, worker, NaturalGranaryTypeID, map[sim.FieldID]int64{
		NaturalGranaryStockField: 0, NaturalGranaryCultivatedTotalField: 0})
	naturalLabourRewrite(t, seeds, worker, NaturalLabourOfferTypeID, map[sim.FieldID]int64{
		NaturalLabourOfferHourField: 4, NaturalLabourOfferStatusField: int64(NaturalRequestAccepted),
		NaturalLabourOfferLastOfferHourField: 4, NaturalLabourOfferLastReplyHourField: 4})
	naturalLabourRewriteRef(t, seeds, worker, NaturalLabourOfferTypeID, NaturalLabourOfferAddresseeField, employer)
	naturalLabourRewrite(t, seeds, employer, NaturalBodyTypeID, map[sim.FieldID]int64{
		NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6, NaturalBodyLastGatherHourField: 5})
	naturalLabourRewrite(t, seeds, employer, NaturalGranaryTypeID, map[sim.FieldID]int64{
		NaturalGranaryStockField: employerStock, NaturalGranaryCultivatedTotalField: employerStock})
	naturalLabourRewrite(t, seeds, employer, NaturalWorksiteTypeID, map[sim.FieldID]int64{
		NaturalWorksiteCapitalField: employerCapital, NaturalWorksiteWipField: employerWip,
		NaturalWorksiteInvestedUnitsField: 2*employerCapital + employerWip, NaturalWorksitePointsCreatedField: employerCapital})
	naturalLabourRewrite(t, seeds, employer, NaturalLabourBookTypeID, map[sim.FieldID]int64{
		NaturalLabourBookEngagedHourField: 4, NaturalLabourBookLastDecisionHourField: NaturalNeverHour})
	naturalLabourRewriteRef(t, seeds, employer, NaturalLabourBookTypeID, NaturalLabourBookEngagedWorkerField, worker)
}

// naturalLabourHybridSeeds builds the hybrid world over the rewritten seeds.
func naturalLabourHybridSeeds(t *testing.T, seeds []component.ComponentSeed) (*Natural, *kernel.Kernel) {
	t.Helper()
	reg, err := NaturalRegistryFor(NaturalMechanismV7)
	if err != nil {
		t.Fatal(err)
	}
	policy := strategy.FrozenNaturalPolicy9()
	policies := strategy.NewNaturalRegistry(2)
	if err := policies.RegisterFrozenNatural(policy); err != nil {
		t.Fatal(err)
	}
	allocator, err := NaturalGenesisAllocator(2)
	if err != nil {
		t.Fatal(err)
	}
	f := &Natural{registry: reg, policies: policies, ref: policy.Ref, mechanism: NaturalMechanismV7,
		founders: naturalFoundersResolved(2, 0), patches: 2, seed: 0, r: 8, births: false,
		bound: make(map[sim.EntityID]*strategy.NaturalBound), allocator: allocator, ledger: newFrozenNaturalLedger(), thresholds: frozenNaturalThresholds(),
		pulseLifecycle: make(map[sim.EntityID]scheduler.Lifecycle), current: make(map[sim.EntityID]strategy.NaturalAction),
		denied: make(map[sim.EntityID]int64), deniedBuild: make(map[sim.EntityID]int64), meals: make(map[sim.EntityID]NaturalMealIntent),
		birthWakes: make(map[sim.EntityID]int64), refusals: make(map[NaturalRefusalReason]int64)}
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	sched, err := scheduler.New(k, 1, f.evaluate)
	if err != nil {
		t.Fatal(err)
	}
	f.k, f.sched = k, sched
	for i := sim.EntityID(1); i <= 16; i++ {
		bound, err := policies.Bind(strategy.NaturalBinding{Actor: i, Ref: policy.Ref})
		if err != nil {
			t.Fatal(err)
		}
		f.bound[i] = bound
		f.roster = append(f.roster, i)
		f.pulseLifecycle[i] = scheduler.Alive
	}
	return f, k
}

func naturalLabourFoldAt(t *testing.T, f *Natural, k *kernel.Kernel, hour int, completions, settleActors []sim.EntityID) {
	t.Helper()
	start, err := NaturalPhaseTime(hour, NaturalPhaseBuildStart)
	if err != nil {
		t.Fatal(err)
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	for _, id := range completions {
		f.current[id] = strategy.NaturalBuild
	}
	if err := f.arbitrateCompletions(view, NaturalBuildCompletion(start), completions, settleActors); err != nil {
		t.Fatal(err)
	}
}

// TestNaturalLabourSettlementPrecedenceFold pins the completion-instant fold
// directly on constructed v7 worlds (the supervisor-ruled settlement
// precedence, recorded pre-run): the settlement wins its field set, the
// employer's own same-instant rule-505 completion is voided BEFORE it is
// proposed (typed runner-level note, zero mutation — the kernel state never
// moves), a chained later settlement is voided the same way, an unrelated
// builder is unaffected, and a contract refusal voids with the contract's
// own typed reason.
func TestNaturalLabourSettlementPrecedenceFold(t *testing.T) {
	t.Run("employer build voided settlement committed", func(t *testing.T) {
		seeds, _, err := naturalGenesisSeeds(NaturalMechanismV7, 2, 8, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		naturalLabourEngageSeeds(t, seeds, 1, 2, 5, 1, 1)
		f, k := naturalLabourHybridSeeds(t, seeds)
		naturalLabourFoldAt(t, f, k, 5, []sim.EntityID{2}, []sim.EntityID{1})
		outcome, ok := f.settles[1]
		if !ok || outcome.void != "" || outcome.proposal.Key == "" {
			t.Fatalf("settlement outcome: %+v", outcome)
		}
		if len(outcome.proposal.Patches) != 12 {
			t.Fatalf("settlement proposal carries %d patches, want the completion shape 12", len(outcome.proposal.Patches))
		}
		build, ok := f.builds[2]
		if !ok || build.voidNote != naturalSettlementPrecedenceNote || build.proposal.Key != "" || build.refused != NaturalBuildRefusalNone {
			t.Fatalf("employer build outcome: %+v", build)
		}
		// Zero mutation at the kernel: the fold proposes, it never commits.
		if _, _, version := k.Snapshot(); version != 0 {
			t.Fatalf("fold mutated the kernel: version %d", version)
		}
	})
	t.Run("chained settlement voided by precedence", func(t *testing.T) {
		seeds, _, err := naturalGenesisSeeds(NaturalMechanismV7, 2, 8, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Actor 2 is employer for worker 1 and simultaneously worker for
		// employer 3: two settlements share actor 2's granary stock. The
		// second engage call rewrote actor 2's employer-side margins as its
		// worker-side ones, so its employer stock is restored afterwards.
		naturalLabourEngageSeeds(t, seeds, 1, 2, 5, 0, 0)
		naturalLabourEngageSeeds(t, seeds, 2, 3, 5, 0, 0)
		naturalLabourRewrite(t, seeds, 2, NaturalGranaryTypeID, map[sim.FieldID]int64{
			NaturalGranaryStockField: 2, NaturalGranaryCultivatedTotalField: 2})
		f, k := naturalLabourHybridSeeds(t, seeds)
		naturalLabourFoldAt(t, f, k, 5, nil, []sim.EntityID{1, 2})
		first, ok := f.settles[1]
		if !ok || first.void != "" || first.proposal.Key == "" {
			t.Fatalf("first settlement outcome: %+v", first)
		}
		second, ok := f.settles[2]
		if !ok || second.void != naturalSettlementPrecedenceNote || second.proposal.Key != "" {
			t.Fatalf("chained settlement outcome: %+v", second)
		}
	})
	t.Run("unrelated builder unaffected", func(t *testing.T) {
		seeds, _, err := naturalGenesisSeeds(NaturalMechanismV7, 2, 8, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		naturalLabourEngageSeeds(t, seeds, 1, 2, 5, 1, 1)
		// Builder 3: one gathered unit, worksite at wip 1, stock 1.
		naturalLabourRewrite(t, seeds, 3, NaturalBodyTypeID, map[sim.FieldID]int64{
			NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6, NaturalBodyLastGatherHourField: 5})
		naturalLabourRewrite(t, seeds, 3, NaturalBagTypeID, map[sim.FieldID]int64{NaturalBagUnitsField: 1})
		naturalLabourRewrite(t, seeds, 3, NaturalGranaryTypeID, map[sim.FieldID]int64{
			NaturalGranaryStockField: 1, NaturalGranaryCultivatedTotalField: 1})
		naturalLabourRewrite(t, seeds, 3, NaturalWorksiteTypeID, map[sim.FieldID]int64{
			NaturalWorksiteWipField: 1, NaturalWorksiteInvestedUnitsField: 1})
		f, k := naturalLabourHybridSeeds(t, seeds)
		naturalLabourFoldAt(t, f, k, 5, []sim.EntityID{3}, []sim.EntityID{1})
		if outcome := f.settles[1]; outcome.void != "" || outcome.proposal.Key == "" {
			t.Fatalf("settlement outcome: %+v", outcome)
		}
		build, ok := f.builds[3]
		if !ok || build.voidNote != "" || build.proposal.Key == "" {
			t.Fatalf("unrelated builder outcome: %+v", build)
		}
	})
	t.Run("contract typed void employer stock short", func(t *testing.T) {
		seeds, _, err := naturalGenesisSeeds(NaturalMechanismV7, 2, 8, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		naturalLabourEngageSeeds(t, seeds, 1, 2, 0, 0, 0)
		f, k := naturalLabourHybridSeeds(t, seeds)
		naturalLabourFoldAt(t, f, k, 5, nil, []sim.EntityID{1})
		outcome, ok := f.settles[1]
		if !ok || outcome.void != "employer-stock-short" || outcome.proposal.Key != "" {
			t.Fatalf("stock-short void outcome: %+v", outcome)
		}
		if _, _, version := k.Snapshot(); version != 0 {
			t.Fatalf("typed void mutated the kernel: version %d", version)
		}
	})
}

// TestNaturalLabourEngagementBusyVoid pins the E12-class void: a policy
// accept the world refuses (the one engagement slot still occupied by the
// previous hour's live engagement) stages nothing — typed zero-mutation
// evidence, the pending offer left for the next pulse's rule 519.
func TestNaturalLabourEngagementBusyVoid(t *testing.T) {
	seeds, _, err := naturalGenesisSeeds(NaturalMechanismV7, 2, 8, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Employer 2 is engaged to worker 1 (engaged-hour 4, delivery hour 5)
	// with wage-capable margins, and worker 3's offer pends addressed to 2
	// in hour 5: the policy floor passes, the book is busy.
	naturalLabourEngageSeeds(t, seeds, 1, 2, 5, 0, 0)
	naturalLabourRewrite(t, seeds, 3, NaturalLabourOfferTypeID, map[sim.FieldID]int64{
		NaturalLabourOfferHourField: 5, NaturalLabourOfferStatusField: int64(NaturalRequestPending),
		NaturalLabourOfferLastOfferHourField: 5, NaturalLabourOfferLastReplyHourField: NaturalNeverHour})
	naturalLabourRewriteRef(t, seeds, 3, NaturalLabourOfferTypeID, NaturalLabourOfferAddresseeField, 2)
	f, k := naturalLabourHybridSeeds(t, seeds)
	at, err := NaturalPhaseTime(5, NaturalPhaseLabourDecision)
	if err != nil {
		t.Fatal(err)
	}
	proposal, attempt, err := f.labourDecisionDecision(f.stateView(), 5, at, 2)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Key != "" {
		t.Fatalf("busy-book accept staged a proposal: %+v", proposal)
	}
	if attempt.Actor != 2 || attempt.Kind != "labour-void" || attempt.Refused != naturalLabourEngagementBusyNote {
		t.Fatalf("busy-book void evidence: %+v", attempt)
	}
	if _, _, version := k.Snapshot(); version != 0 {
		t.Fatalf("decision void mutated the kernel: version %d", version)
	}
}

func naturalLabourCommit(t *testing.T, k *kernel.Kernel, proposal kernel.Proposal) {
	t.Helper()
	_, authority, _ := k.Snapshot()
	plan, err := k.Plan(proposal, authority)
	if err != nil {
		t.Fatalf("plan %s: %v", proposal.Key, err)
	}
	if _, err := k.CommitBatch([]kernel.Plan{plan}); err != nil {
		t.Fatalf("commit %s: %v", proposal.Key, err)
	}
}

// TestNaturalV7ZeroUseControlGate is the frozen zero-use gate as a test: a
// (mechanism 7, natural/7b) world with zero 516/517/518 attempts must
// reproduce a (mechanism 6, natural/7b) run of the same cell field for
// field — declared as a trajectory comparison (the hourly checkpoints
// DeepEqual, every metric and ledger row included) — with the journal and
// bundle digests stable within each version (the rule versions differ
// between the axes, so the axis bytes differ; the trajectories must not).
func TestNaturalV7ZeroUseControlGate(t *testing.T) {
	const seed = uint64(3)
	v6, err := NewNatural(naturalLabourV7bOptions(seed, 2, true, NaturalMechanismV6))
	if err != nil {
		t.Fatal(err)
	}
	v7, err := NewNatural(naturalLabourV7bOptions(seed, 2, true, NaturalMechanismV7))
	if err != nil {
		t.Fatal(err)
	}
	if err := v6.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := v7.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	v6Checks, v7Checks := v6.Checkpoints(), v7.Checkpoints()
	if len(v6Checks) != len(v7Checks) {
		t.Fatalf("checkpoint counts diverge: %d vs %d", len(v6Checks), len(v7Checks))
	}
	// The V7-only rows are the one spelling difference: they exist (idle)
	// under mechanism 7 and do not exist (zero rows) under mechanism 6.
	// The gate asserts them idle — the zero-use proof — and then normalizes
	// them to the v6 spelling, so the remaining comparison is the shared
	// trajectory field for field.
	normalizeV7Rows := func(checks []NaturalCheckpoint) []NaturalCheckpoint {
		out := cloneNaturalCheckpoints(checks)
		zeroOffer, zeroBook := NaturalLabourOfferState{}, NaturalLabourBookState{}
		for i := range out {
			for j := range out[i].Founders {
				if out[i].Founders[j].Offer != NaturalIdleLabourOfferState() || out[i].Founders[j].Book != NaturalIdleLabourBookState() {
					t.Fatalf("zero-use control founder %d left the wage rows non-idle at hour %d", j+1, out[i].Hour)
				}
				out[i].Founders[j].Offer, out[i].Founders[j].Book = zeroOffer, zeroBook
			}
			for j := range out[i].Newborns {
				if out[i].Newborns[j].State.Offer != NaturalIdleLabourOfferState() || out[i].Newborns[j].State.Book != NaturalIdleLabourBookState() {
					t.Fatalf("zero-use control newborn left the wage rows non-idle at hour %d", out[i].Hour)
				}
				out[i].Newborns[j].State.Offer, out[i].Newborns[j].State.Book = zeroOffer, zeroBook
			}
		}
		return out
	}
	v7Checks = normalizeV7Rows(v7Checks)
	// Trajectory comparison: every ecology field must match field for
	// field — state, balance, allocator, metrics. The ceiling ledger's
	// byte/node rows are compute instrumentation measured on axis-exact
	// bytes (rule version 7 encodes differently from 6), so they are the
	// one declared exclusion; their thresholds and runner/fiber counts
	// still match.
	for i := range v6Checks {
		a, b := v6Checks[i], v7Checks[i]
		if a.Hour != b.Hour || a.Version != b.Version || a.Balance != b.Balance ||
			a.AllocatorLast != b.AllocatorLast || !reflect.DeepEqual(a.Metrics, b.Metrics) ||
			!reflect.DeepEqual(a.Patches, b.Patches) || !reflect.DeepEqual(a.Slots, b.Slots) ||
			!reflect.DeepEqual(a.Founders, b.Founders) || !reflect.DeepEqual(a.Newborns, b.Newborns) {
			t.Fatalf("zero-use trajectory diverges at hour %d", a.Hour)
		}
		if len(a.Ledger) != len(b.Ledger) {
			t.Fatalf("ledger row count diverges at hour %d", a.Hour)
		}
		for j := range a.Ledger {
			if a.Ledger[j].Ceiling != b.Ledger[j].Ceiling || a.Ledger[j].Threshold != b.Ledger[j].Threshold {
				t.Fatalf("ledger identity diverges at hour %d row %d", a.Hour, j)
			}
			switch a.Ledger[j].Ceiling {
			case NaturalCeilingHistoryEvents, NaturalCeilingHistoryPatches, NaturalCeilingHistoryValueNodes,
				NaturalCeilingHistoryBytes, NaturalCeilingEventBytes, NaturalCeilingValueBytes,
				NaturalCeilingSnapshotBytes, NaturalCeilingProjectedMetrics, NaturalCeilingProjectionBytes:
				// Axis-exact byte and encoding instrumentation: declared
				// exclusion (the rule/schema versions differ by design).
			default:
				if a.Ledger[j] != b.Ledger[j] {
					t.Fatalf("ledger row %s diverges at hour %d: %+v vs %+v", string(a.Ledger[j].Ceiling), a.Hour, a.Ledger[j], b.Ledger[j])
				}
			}
		}
	}
	// No labour attempt ever fired in the control.
	for _, b := range v7.Journal() {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "labour-offer", "labour-decision", "labour-settlement", "labour-void":
				t.Fatalf("zero-use control recorded a labour attempt: %+v", a)
			}
		}
	}
	// Digest stability within each version (export is a pure function of
	// the committed evidence), and rule-version-exact journals.
	for _, f := range []*Natural{v6, v7} {
		first, err := f.ExportJournal()
		if err != nil {
			t.Fatal(err)
		}
		second, err := f.ExportJournal()
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("journal export for mechanism %d is not stable", f.mechanism)
		}
	}
	// The two bundles differ exactly in the mechanism block: every other
	// manifest field (identity, head, sections, flags, lineage) is equal.
	dir := t.TempDir()
	if _, err := v6.SaveResumeCheckpoint(dir + "/v6.bundle"); err != nil {
		t.Fatal(err)
	}
	if _, err := v7.SaveResumeCheckpoint(dir + "/v7.bundle"); err != nil {
		t.Fatal(err)
	}
	_, _, m6, err := naturalCheckedSections(dir + "/v6.bundle")
	if err != nil {
		t.Fatal(err)
	}
	_, _, m7, err := naturalCheckedSections(dir + "/v7.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if m6.mechanism != NaturalMechanismV6 || m7.mechanism != NaturalMechanismV7 {
		t.Fatalf("manifest mechanisms: %d vs %d", m6.mechanism, m7.mechanism)
	}
	// The declared run identity is identical field for field; the axis-exact
	// bytes (kernel head hash and the four section digests, both carrying
	// the rule versions) differ by design, and the policy identity is the
	// same natural/7b content fingerprint on both axes.
	if m6.patches != m7.patches || m6.r != m7.r || m6.seed != m7.seed ||
		m6.births != m7.births || m6.boundary != m7.boundary || m6.tenure != m7.tenure ||
		m6.cultivation != m7.cultivation || m6.steps != m7.steps || m6.time != m7.time ||
		m6.founders != m7.founders || m6.policyFormat != m7.policyFormat ||
		m6.policyFingerprint != m7.policyFingerprint || m6.parent != m7.parent ||
		m6.parentHead != m7.parentHead || m6.depth != m7.depth {
		t.Fatal("the axis bundles differ in their declared run identity")
	}
	// Review fix (parent): the earlier form of this assertion demanded equal
	// genesis hashes, which contradicts the design — the v7 genesis seeds the
	// V7 rows the contract validators require (idle labour-offer rows, the
	// labour book, nine-field granaries), so the genesis hash legitimately
	// differs, and the axis-exact bytes (the tip hash, carrying the rule
	// versions, and the four section digests) differ too. What the design's
	// zero-use control gate actually requires is: the same event count, a
	// DIFFERENT registry fingerprint (the versions differ), and identical
	// trajectories once the inert V7 rows are normalised away — which the
	// per-hour comparison above performs field for field.
	if m6.head.TipID != m7.head.TipID {
		t.Fatalf("zero-use control event counts diverged: %d vs %d", m6.head.TipID, m7.head.TipID)
	}
	if m6.head.RegistryFingerprint == m7.head.RegistryFingerprint {
		t.Fatal("the two mechanism registries must fingerprint differently")
	}
	if m6.head.GenesisHash == m7.head.GenesisHash {
		t.Fatal("the v7 genesis seeds the V7 rows, so the genesis hash must differ")
	}
}

// TestNaturalV7ManifestTupleAndRefusals pins the mechanism-7 identity
// surface: the manifest tuple (7,7,7,7) byte-exact in the four mechanism
// slots, the journal Mechanism=7, the bidirectional {5,6,7} refusal of
// restore and journal restore before any work, and publish-nothing on a
// refused branch.
func TestNaturalV7ManifestTupleAndRefusals(t *testing.T) {
	dir := t.TempDir()
	f, err := NewNatural(naturalLabourV9Options(0, 1, false))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	digest, err := f.SaveCheckpoint(filepath.Join(dir, "v7.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	sections, gotDigest, m, err := naturalCheckedSections(filepath.Join(dir, "v7.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	if gotDigest != digest || m.mechanism != NaturalMechanismV7 {
		t.Fatalf("manifest identity: digest %x mechanism %d", gotDigest, m.mechanism)
	}
	// The four mechanism slots carry exactly 7 (world, schema, rule,
	// projection): the raw u32 slots at the pinned positions.
	data := sections[2].Data
	for _, slot := range naturalManifestMechanismSlots {
		v := uint32(data[4+4*slot])<<24 | uint32(data[5+4*slot])<<16 | uint32(data[6+4*slot])<<8 | uint32(data[7+4*slot])
		if v != 7 {
			t.Fatalf("mechanism slot %d carries %d, want the (7,7,7,7) tuple", slot, v)
		}
	}
	// The journal's declared mechanism is 7.
	journal, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if _, head, err := f.k.ExportHistory(); err != nil {
		t.Fatal(err)
	} else {
		batches, err := DecodeNaturalJournal(journal, f.k, head, f.sched.Time(), NaturalJournalConfig{Patches: 2, R: 8, Seed: 0,
			Boundary: false, Births: false, Mechanism: NaturalMechanismV7, Policy: strategy.FrozenNaturalPolicy9().Ref,
			PolicyFingerprint: func() [32]byte { fp, _ := naturalJournalFingerprint(strategy.FrozenNaturalPolicy9().Ref); return fp }()})
		if err != nil || len(batches) == 0 {
			t.Fatalf("mechanism-7 journal identity refused: %v", err)
		}
	}
	// Bidirectional refusal: a v7 bundle never restores under a v5 or v6
	// expectation, and a v6 bundle never restores under a v7 expectation.
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV5, NaturalMechanismV6} {
		if _, _, err := RestoreNaturalCheckpoint(filepath.Join(dir, "v7.bundle"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1,
			PolicyRef: strategy.FrozenNaturalPolicy9().Ref, Mechanism: mechanism}); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("v7 bundle restored under mechanism %d", mechanism)
		}
	}
	v6, err := NewNatural(naturalLabourV7bOptions(0, 1, false, NaturalMechanismV6))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := v6.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := v6.SaveCheckpoint(filepath.Join(dir, "v6.bundle")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreNaturalCheckpoint(filepath.Join(dir, "v6.bundle"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy7b().Ref, Mechanism: NaturalMechanismV7}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("v6 bundle restored under a v7 expectation")
	}
	// Journal-level bidirectional refusal (publish-nothing: the runner's
	// journal is untouched after a refused restore).
	_, portableHead, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	v6Journal, err := v6.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.Journal())
	if err := f.RestoreJournal(v6Journal, portableHead); !errors.Is(err, ErrNaturalJournal) {
		t.Fatal("v7 runner accepted a mechanism-v6 journal")
	}
	if len(f.Journal()) != before {
		t.Fatal("a refused journal restore mutated the journal")
	}
	// Branch publish-nothing: a refused expectation writes no destination.
	dest := filepath.Join(dir, "branch.bundle")
	if _, err := BranchNaturalCheckpoint(filepath.Join(dir, "v7.bundle"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy9().Ref, Mechanism: NaturalMechanismV6}, true, dest); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("branch accepted a v6 expectation for a v7 bundle")
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused branch published its destination")
	}
}

// TestNaturalLabourCheckpointRoundTrip proves the persistence lane end to
// end on a trade cell: a pulse-boundary capture after committed settlements
// restores into an independent executable whose continued run reproduces
// the uninterrupted run's checkpoints and journal exactly (the trusted
// replay re-derives the full offer → accept → settle linkage, never a
// status-only patch).
func TestNaturalLabourCheckpointRoundTrip(t *testing.T) {
	const seed = uint64(0)
	dir := t.TempDir()
	whole, err := NewNatural(naturalLabourV9Options(seed, 1, false))
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, whole, 8)
	path := filepath.Join(dir, "resume.bundle")
	if _, err := whole.SaveResumeCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 3,
		PolicyRef: strategy.FrozenNaturalPolicy9().Ref, Mechanism: NaturalMechanismV7})
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, whole, 24)
	naturalRunTo(t, restored, 24)
	if !reflect.DeepEqual(whole.Checkpoints(), restored.Checkpoints()) {
		t.Fatal("restored continuation diverged from the uninterrupted run")
	}
	wholeJournal, err := whole.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	restoredJournal, err := restored.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if string(wholeJournal) != string(restoredJournal) {
		t.Fatal("restored journal diverged from the uninterrupted run")
	}
}

// TestNaturalLabourDeterminismWorkers1And4 pins worker-count determinism on
// a (v7, natural/9) births cell: one worker and four workers produce
// identical checkpoints and identical journal digests.
func TestNaturalLabourDeterminismWorkers1And4(t *testing.T) {
	const seed = uint64(5)
	one, err := NewNatural(naturalLabourV9Options(seed, 1, true))
	if err != nil {
		t.Fatal(err)
	}
	four, err := NewNatural(naturalLabourV9Options(seed, 4, true))
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, one, 20)
	naturalRunTo(t, four, 20)
	if !reflect.DeepEqual(one.Checkpoints(), four.Checkpoints()) {
		t.Fatal("worker count changed the trajectory")
	}
	oneJournal, err := one.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	fourJournal, err := four.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if string(oneJournal) != string(fourJournal) {
		t.Fatal("worker count changed the journal")
	}
}

// TestNaturalLabourForgedSettlementReplayRefused is the v2-v7 lesson as a
// test: a settlement is never a status-only patch. A journal whose
// offer/decision chain was stripped (a settlement attempt without a matched
// offer plus accept) is refused at encode and at restore — the trusted
// replay re-derives the chain from genesis and the divergence fails closed.
func TestNaturalLabourForgedSettlementReplayRefused(t *testing.T) {
	f, err := NewNatural(naturalLabourV9Options(0, 2, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	journal := f.Journal()
	settlements := 0
	for _, b := range journal {
		for _, a := range b.Attempts {
			if a.Kind == "labour-settlement" {
				settlements++
			}
		}
	}
	if settlements == 0 {
		t.Fatal("no settlement to forge against")
	}
	// Strip every labour-offer and labour-decision attempt: the forged
	// history carries settlements without their matched chain.
	for i := range journal {
		kept := journal[i].Attempts[:0]
		for _, a := range journal[i].Attempts {
			if a.Kind == "labour-offer" || a.Kind == "labour-decision" {
				continue
			}
			kept = append(kept, a)
		}
		journal[i].Attempts = append([]NaturalAttempt(nil), kept...)
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	config := f.journalGenesisKind()
	if _, err := encodeNaturalJournal(journal, f.k, head, f.sched.Time(), config); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("forged settlement journal encoded: %v", err)
	}
	// Framed bytes reach the same refusal at restore (publish-nothing).
	wire := naturalJournalWire{Format: NaturalJournalFormatVersion, Head: head, SchedulerTime: f.sched.Time(),
		Config: config, Counts: naturalJournalCounts(journal, f.k.Events()), Batches: journal}
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	forged := naturalFrameJournal(body)
	if err := f.RestoreJournal(forged, head); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("forged journal restored: %v", err)
	}
}

// naturalFrameJournal wraps a canonical journal body in the frozen framing
// with a correct digest, so a forgery fails on verification, never on
// framing.
func naturalFrameJournal(body []byte) []byte {
	out := append([]byte(nil), naturalJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, NaturalJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}
