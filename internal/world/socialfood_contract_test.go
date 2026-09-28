package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"reflect"
	"testing"
)

func socialFoodActors() [16]SocialFoodActorState {
	var actors [16]SocialFoodActorState
	for i := range actors {
		actors[i] = SocialFoodActorState{Energy: 11, LastGatherHour: -1}
	}
	return actors
}
func socialFoodRequests() [16]SocialFoodRequestState {
	var requests [16]SocialFoodRequestState
	for i := range requests {
		requests[i].Hour = -1
	}
	return requests
}
func socialFoodLedgers() [16]SocialFoodLedgerState {
	var ledgers [16]SocialFoodLedgerState
	for i := range ledgers {
		ledgers[i].LastReplyHour = -1
	}
	return ledgers
}
func TestSocialFoodIdentityClockAndHistoryIsolation(t *testing.T) {
	reg, err := SocialFoodRegistry()
	if err != nil || len(reg.TypeIDs()) != 6 {
		t.Fatalf("registry %v %v", reg.TypeIDs(), err)
	}
	old, err := FoodFlowRegistry()
	if err != nil {
		t.Fatal(err)
	}
	s6, err := survivalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range reg.TypeIDs() {
		d, _ := reg.Describe(typ)
		if d.SchemaVersion != 2 || d.MigrationPolicy != component.MigrationRejectUnlisted {
			t.Fatalf("v2 descriptor %+v", d)
		}
		for _, r := range d.TransitionRules {
			if r.Version != 2 || r.ID < 201 {
				t.Fatalf("rule %+v", r)
			}
		}
		for _, p := range d.Projections {
			if p.Version != 2 {
				t.Fatalf("projection %+v", p)
			}
		}
		for _, prior := range []component.Registry{old, s6} {
			if _, err := prior.Describe(typ); !errors.Is(err, component.ErrUnknownComponentType) {
				t.Fatalf("old registry recognizes %d", typ)
			}
		}
	}
	for _, prior := range []component.Registry{old, s6} {
		for _, typ := range prior.TypeIDs() {
			if _, err := reg.Describe(typ); !errors.Is(err, component.ErrUnknownComponentType) {
				t.Fatalf("v2 recognizes old type %d", typ)
			}
		}
		k, err := kernel.New(prior, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		history, _, err := k.ExportHistory()
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := kernel.RestoreHistory(reg, history); err == nil {
			t.Fatal("old history restored into v2")
		}
	}
	ledgerDesc := SocialFoodLedgerDescriptor()
	var netProjection component.ProjectionDescriptor
	for _, p := range ledgerDesc.Projections {
		if p.ID == sim.ProjectionID(SocialFoodNetField) {
			netProjection = p
		}
	}
	if !reflect.DeepEqual(netProjection.SourceFields, []sim.FieldID{SocialFoodReceivedField, SocialFoodGivenField}) || !reflect.DeepEqual(netProjection.Coefficients, []float64{1, -1}) || netProjection.Unit != "food" {
		t.Fatalf("net provenance: %+v", netProjection)
	}
	reader, authority, err := component.NewReader(reg, 7, []component.ComponentSeed{{Entity: 4, Component: SocialFoodLedgerTypeID, Fields: []component.FieldSeed{
		{Field: SocialFoodLastReplyHourField, Value: sim.IntegerValue(-1)}, {Field: SocialFoodReceivedField, Value: sim.IntegerValue(5)}, {Field: SocialFoodGivenField, Value: sim.IntegerValue(2)}, {Field: SocialFoodNetField, Value: sim.IntegerValue(0)},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := reader.Project(component.ProjectRequest{Component: SocialFoodLedgerTypeID, Projection: sim.ProjectionID(SocialFoodNetField), Entities: []sim.EntityID{4}, WorldVersion: 7, Authority: authority})
	if err != nil || metrics.Len() != 1 {
		t.Fatalf("net project %v", err)
	}
	metric, _ := metrics.At(0)
	value, err := metric.Value().Scalar()
	if err != nil || value != 3 || metric.Unit() != "food" || metric.Provenance().WorldVersion != 7 || !reflect.DeepEqual(metric.Provenance().SourceFields, []sim.FieldID{SocialFoodReceivedField, SocialFoodGivenField}) {
		t.Fatalf("derived net: %+v %v", metric, err)
	}
	// Stored net=0 here is deliberately inconsistent; the projection must not
	// silently expose it as the received-minus-given quantity.
	if validSocialFoodLedger(SocialFoodLedgerState{LastReplyHour: -1, Received: 5, Given: 2, Net: 0}) {
		t.Fatal("invalid stored net admitted")
	}
	for actor := sim.EntityID(1); actor <= 16; actor++ {
		target, affinity, err := SocialFoodKnownAddressee(actor)
		if err != nil || ((actor-1)/8 != (target-1)/8) || ((actor == 4 || actor == 8 || actor == 12 || actor == 16) != (affinity == 3)) {
			t.Fatalf("dyad %d -> %d score %d: %v", actor, target, affinity, err)
		}
	}
	for _, phase := range []struct {
		n    int
		want sim.SimTime
	}{{0, 1}, {1, 2}, {2, 3}, {3, 4}} {
		at, err := SocialFoodPhaseTime(0, phase.n)
		if err != nil || at != sim.SimTime(SocialFoodGatherDuration)+phase.want {
			t.Fatalf("phase %d: %d %v", phase.n, at, err)
		}
	}
	complete, err := SocialFoodPhaseTime(0, 4)
	if err != nil || complete != sim.SimTime(SocialFoodGatherDuration+SocialFoodMealDuration)+4 {
		t.Fatalf("meal completion: %d %v", complete, err)
	}
	for _, invalid := range []struct{ hour, phase int }{{-1, 0}, {168, 0}, {0, 5}} {
		if _, err := SocialFoodPhaseTime(invalid.hour, invalid.phase); err == nil {
			t.Fatalf("bad clock %+v", invalid)
		}
	}
}
func TestSocialFoodOpportunityGiftConsumptionAndConservation(t *testing.T) {
	actors := socialFoodActors()
	requests := socialFoodRequests()
	ledgers := socialFoodLedgers()
	patches := [2]SocialFoodPatchState{{Yield: 1}}
	var slots [2][8]SocialFoodSlotState
	var err error
	patches[0], slots[0], err = SocialFoodProduce(0, patches[0], slots[0])
	if err != nil {
		t.Fatal(err)
	}
	before, err := SocialFoodCheckConservation(patches, slots, actors)
	if err != nil || before.Stock != 1 {
		t.Fatalf("before: %+v %v", before, err)
	}
	slots[0][0], actors[4], err = SocialFoodGather(0, 3001, 4001, 5, slots[0][0], actors[4])
	if err != nil {
		t.Fatal(err)
	}
	actors[3], err = SocialFoodBasal(4, actors[3], 3)
	if err != nil {
		t.Fatal(err)
	}
	// True urgency and high affinity produce a score of 3+2-1=4.
	requests[3], err = SocialFoodRequest(0, 4, 5, true, 2, requests[3], actors[3])
	if err != nil || requests[3].Claim != 2 || actors[3].Energy != 8 {
		t.Fatalf("claim %+v %v", requests[3], err)
	}
	if _, err := SocialFoodRequest(0, 4, 5, true, 2, requests[3], actors[3]); err == nil {
		t.Fatal("duplicate request")
	}
	score, err := SocialFoodProjectAssistance(4, 7, requests[3])
	if err != nil || score.Requester != 4 || score.Addressee != 5 || score.Fraction != 1.0/3 || score.WorldVersion != 7 || score.Unit != "fraction" || score.SchemaVersion != 2 || score.ProjectionVersion != 2 || score.SourceFields != ([2]sim.FieldID{SocialFoodRequestsWitnessedField, SocialFoodGiftsWitnessedField}) {
		t.Fatalf("assistance %+v %v", score, err)
	}
	// A high-scoring eligible donor must choose accept; a request alone never
	// becomes a gift without its separate matching donor intention.
	c := SocialFoodConsent{RequestID: requests[3].ID, Donor: 5, Hour: 0, Accept: true}
	if socialFoodConsentScore(4, requests[3], actors[4]) != 4 {
		t.Fatal("high-affinity score drift")
	}
	if _, _, err := SocialFoodRefuse(0, 5, 4, SocialFoodConsent{RequestID: c.RequestID, Donor: 5, Hour: 0}, requests[3], actors[4], ledgers[4]); err == nil {
		t.Fatal("eligible high-scoring donor refused")
	}
	// An independently consenting donor can gift even if recipient never gathered.
	at, _ := SocialFoodPhaseTime(0, 2)
	proposal, err := SocialFoodGiftProposal("gift/4/0", at, 0, 5, 4, c, requests[3], actors[4], actors[3], ledgers[4], ledgers[3])
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Rule != SocialFoodGiftRule || proposal.Cause.Actor != 5 || len(proposal.Patches) != 15 {
		t.Fatalf("gift proposal %+v", proposal)
	}
	expected := map[[3]uint64]int64{{4, uint64(SocialFoodRequestTypeID), uint64(SocialFoodRequestStatusField)}: int64(SocialFoodAccepted), {4, uint64(SocialFoodRequestTypeID), uint64(SocialFoodGiftsWitnessedField)}: 1, {5, uint64(SocialFoodBagTypeID), uint64(SocialFoodBagUnitsField)}: 0, {4, uint64(SocialFoodBagTypeID), uint64(SocialFoodBagUnitsField)}: 1, {5, uint64(SocialFoodLedgerTypeID), uint64(SocialFoodNetField)}: -1, {4, uint64(SocialFoodLedgerTypeID), uint64(SocialFoodNetField)}: 1}
	for _, p := range proposal.Patches {
		key := [3]uint64{uint64(p.Entity), uint64(p.Component), uint64(p.Field)}
		if n, ok := expected[key]; ok {
			got, e := p.Value.Integer()
			if e != nil || got != n {
				t.Fatalf("wrong delta %+v", p)
			}
			delete(expected, key)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("missing atomic fields %v", expected)
	}
	requests[3], actors[4], actors[3], ledgers[4], ledgers[3], err = SocialFoodAccept(0, 5, 4, c, requests[3], actors[4], actors[3], ledgers[4], ledgers[3])
	if err != nil {
		t.Fatal(err)
	}
	if actors[3].LastGatherHour != -1 || actors[3].Bag != (SocialFoodBagState{Units: 1, Source: 3001, Gatherer: 5, LastDonor: 5}) || ledgers[3].Net != 1 || ledgers[4].Net != -1 {
		t.Fatalf("gift provenance/ledger: %+v %+v %+v", actors[3], ledgers[3], ledgers[4])
	}
	if err := SocialFoodCheckWitnesses(requests, ledgers); err != nil {
		t.Fatalf("witnesses: %v", err)
	}
	fake := ledgers
	fake[4].Given++
	if err := SocialFoodCheckWitnesses(requests, fake); err == nil {
		t.Fatal("unwitnessed gift admitted")
	}
	after, err := SocialFoodCheckConservation(patches, slots, actors)
	if err != nil || after.Held != before.Stock || after.Produced != before.Produced || after.Consumed != before.Consumed {
		t.Fatalf("gift conservation %+v %v", after, err)
	}
	giftEnergy := after.Energy
	if validFoodFlowActor(FoodFlowActorState{Bag: 1, BagSource: 1001, Energy: 8, BasalSpent: 3, LastGatherHour: -1}) {
		t.Fatal("v1 admitted never-gathered gift")
	}
	actors[3], err = SocialFoodConsume(4, actors[3])
	if err != nil || actors[3].LastGatherHour != -1 || actors[3].Consumed != 1 {
		t.Fatalf("never-gathered eat %+v %v", actors[3], err)
	}
	after, err = SocialFoodCheckConservation(patches, slots, actors)
	if err != nil || after.Held != 0 || after.Consumed != 1 || after.Energy != giftEnergy+1 {
		t.Fatalf("meal balance %+v %v", after, err)
	}
	altered := actors
	altered[3].Bag = SocialFoodBagState{Units: 1, Source: 3002, Gatherer: 5, LastDonor: 5}
	if _, err := SocialFoodCheckConservation(patches, slots, altered); err == nil {
		t.Fatal("cross-patch provenance admitted")
	}
}
func TestSocialFoodRefusalExpiryAndInvalidConsent(t *testing.T) {
	actors := socialFoodActors()
	requests := socialFoodRequests()
	ledgers := socialFoodLedgers()
	actors[3].Energy = 8
	actors[3].BasalSpent = 3
	actors[4].Energy = 3
	actors[4].BasalSpent = 8
	actors[4].Bag = SocialFoodBagState{Units: 1, Source: 3001, Gatherer: 5}
	r, err := SocialFoodRequest(0, 4, 5, true, 2, requests[3], actors[3])
	if err != nil {
		t.Fatal(err)
	}
	consent := SocialFoodConsent{RequestID: r.ID, Donor: 5, Hour: 0, Accept: true}
	at, _ := SocialFoodPhaseTime(0, 2)
	if _, err := SocialFoodGiftProposal("low-reserve", at, 0, 5, 4, consent, r, actors[4], actors[3], ledgers[4], ledgers[3]); err == nil {
		t.Fatal("low reserve transferred")
	}
	consent.Accept = false
	refused, l, err := SocialFoodRefuse(0, 5, 4, consent, r, actors[4], ledgers[4])
	if err != nil || refused.Refusals != 1 || l.LastReplyHour != 0 {
		t.Fatalf("refusal %+v %+v %v", refused, l, err)
	}
	if _, _, err := SocialFoodRefuse(0, 5, 4, consent, r, actors[4], l); err == nil {
		t.Fatal("duplicate reply")
	}
	finalize, _ := SocialFoodPhaseTime(0, 3)
	if _, err := SocialFoodExpire(0, finalize, 4, refused); err == nil {
		t.Fatal("refused request expired")
	}
	expired, err := SocialFoodExpire(0, finalize, 4, r)
	if err != nil || expired.Status != SocialFoodExpired {
		t.Fatalf("expiry %+v %v", expired, err)
	}
	consent.Accept = true
	bad := []struct {
		name             string
		donor, recipient sim.EntityID
		consent          SocialFoodConsent
		r                SocialFoodRequestState
		giver, receiver  SocialFoodActorState
		given, received  SocialFoodLedgerState
	}{
		{"expired", 5, 4, consent, expired, actors[4], actors[3], ledgers[4], ledgers[3]},
		{"duplicate", 5, 4, consent, refused, actors[4], actors[3], l, ledgers[3]},
		{"no-consent", 5, 4, SocialFoodConsent{RequestID: r.ID, Donor: 5, Hour: 0}, r, actors[4], actors[3], ledgers[4], ledgers[3]},
		{"foreign-donor", 6, 4, consent, r, actors[5], actors[3], ledgers[5], ledgers[3]},
		{"cross-patch", 13, 4, consent, r, actors[12], actors[3], ledgers[12], ledgers[3]},
		{"foreign-actor", 5, 17, consent, r, actors[4], actors[3], ledgers[4], ledgers[3]},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SocialFoodGiftProposal(tc.name, at, 0, tc.donor, tc.recipient, tc.consent, tc.r, tc.giver, tc.receiver, tc.given, tc.received); err == nil {
				t.Fatal("invalid proposal accepted")
			}
		})
	}
	if _, err := SocialFoodRequest(0, 4, 13, true, 2, requests[3], actors[3]); err == nil {
		t.Fatal("cross-patch request")
	}
	if _, err := SocialFoodRequest(0, 4, 5, false, 2, requests[3], actors[3]); err == nil {
		t.Fatal("not denied Gather")
	}
	if _, err := SocialFoodWithdraw(0, 4, r); err != nil {
		t.Fatal(err)
	}
	if _, err := SocialFoodExpire(0, finalize, 4, SocialFoodRequestState{Status: SocialFoodPending}); err == nil {
		t.Fatal("forged pending")
	}
	if !reflect.DeepEqual(actors[4].Bag, SocialFoodBagState{Units: 1, Source: 3001, Gatherer: 5}) {
		t.Fatal("failed transfer mutated donor")
	}
}
func TestSocialFoodExpireFinalizationBoundaryAndStatusOnlyProposal(t *testing.T) {
	actor := socialFoodActors()[3]
	actor.Energy, actor.BasalSpent = 8, 3
	pending, err := SocialFoodRequest(0, 4, 5, true, 2, socialFoodRequests()[3], actor)
	if err != nil {
		t.Fatal(err)
	}
	finalize, _ := SocialFoodPhaseTime(0, 3)
	for _, tc := range []struct {
		name        string
		hour, phase int
		request     SocialFoodRequestState
	}{
		{"request-phase-premature", 0, 1, pending},
		{"reply-phase-premature", 0, 2, pending},
		{"next-hour-stale", 1, 3, pending},
		{"already-accepted", 0, 3, SocialFoodRequestState{ID: pending.ID, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodAccepted, Requests: 1, Gifts: 1}},
		{"already-refused", 0, 3, SocialFoodRequestState{ID: pending.ID, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodRefused, Requests: 1, Refusals: 1}},
		{"already-expired", 0, 3, SocialFoodRequestState{ID: pending.ID, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodExpired, Requests: 1}},
		{"already-withdrawn", 0, 3, SocialFoodRequestState{ID: pending.ID, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodWithdrawn, Requests: 1}},
		{"answered-pending", 0, 3, SocialFoodRequestState{ID: pending.ID, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodPending, Requests: 1, Gifts: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at, _ := SocialFoodPhaseTime(tc.hour, tc.phase)
			original := tc.request
			result, err := SocialFoodExpire(tc.hour, at, 4, tc.request)
			if !errors.Is(err, ErrSocialFoodContract) || result != (SocialFoodRequestState{}) || tc.request != original {
				t.Fatalf("invalid expiry mutated: %+v %v", result, err)
			}
			if _, err := SocialFoodExpireProposal(tc.name, tc.hour, at, 4, tc.request); !errors.Is(err, ErrSocialFoodContract) {
				t.Fatalf("invalid event constructed: %v", err)
			}
		})
	}
	if _, err := SocialFoodExpire(0, finalize-1, 4, pending); err == nil {
		t.Fatal("early timestamp admitted")
	}
	if _, err := SocialFoodExpire(0, finalize+1, 4, pending); err == nil {
		t.Fatal("late timestamp admitted")
	}
	if _, err := SocialFoodExpire(0, finalize, 5, pending); err == nil {
		t.Fatal("foreign owner admitted")
	}
	expired, err := SocialFoodExpire(0, finalize, 4, pending)
	if err != nil || expired.Status != SocialFoodExpired || expired.ID != pending.ID || expired.Requests != pending.Requests || expired.Gifts != pending.Gifts || expired.Refusals != pending.Refusals || pending.Status != SocialFoodPending || !validSocialFoodRequest(4, expired) {
		t.Fatalf("expiry invariant: %+v %v", expired, err)
	}
	proposal, err := SocialFoodExpireProposal("expire/0/4", 0, finalize, 4, pending)
	if err != nil || proposal.Time != finalize || proposal.Cause.Actor != 4 || proposal.Rule != SocialFoodExpireRule || proposal.RuleVersion != SocialFoodRuleVersion || len(proposal.Patches) != 1 {
		t.Fatalf("expiry event: %+v %v", proposal, err)
	}
	patch := proposal.Patches[0]
	status, err := patch.Value.Integer()
	if err != nil || patch.Entity != 4 || patch.Component != SocialFoodRequestTypeID || patch.SchemaVersion != SocialFoodSchemaVersion || patch.Field != SocialFoodRequestStatusField || status != int64(SocialFoodExpired) {
		t.Fatalf("non-status expiry patch: %+v %v", patch, err)
	}
	if _, err := SocialFoodExpire(0, finalize, 4, expired); err == nil {
		t.Fatal("duplicate expiry admitted")
	}
	if _, err := SocialFoodExpireProposal("duplicate", 0, finalize, 4, expired); err == nil {
		t.Fatal("duplicate expiry event constructed")
	}
	if _, err := SocialFoodRequest(0, 4, 5, true, 2, expired, actor); err == nil {
		t.Fatal("second same-hour request after expiry")
	}
	if next, err := SocialFoodRequest(1, 4, 5, true, 2, expired, actor); err != nil || next.Status != SocialFoodPending || next.Requests != 2 {
		t.Fatalf("next-hour request: %+v %v", next, err)
	}
}
func TestSocialFoodClaimZeroRefusalAndOutstandingRequest(t *testing.T) {
	actors := socialFoodActors()
	requests := socialFoodRequests()
	ledgers := socialFoodLedgers()
	actors[3].Energy = 8
	actors[3].BasalSpent = 3
	actors[4].Bag = SocialFoodBagState{Units: 1, Source: 3001, Gatherer: 5}
	// An injected claim=0 changes neither private reserve nor the dyad.
	r, err := SocialFoodRequest(0, 4, 5, true, 0, requests[3], actors[3])
	if err != nil {
		t.Fatal(err)
	}
	if r.Addressee != 5 || actors[3].Energy != 8 || socialFoodConsentScore(4, r, actors[4]) != 2 {
		t.Fatalf("claim/score %+v", r)
	}
	c := SocialFoodConsent{RequestID: r.ID, Donor: 5, Hour: 0, Accept: true}
	at, _ := SocialFoodPhaseTime(0, 2)
	if _, err := SocialFoodGiftProposal("wrong-choice", at, 0, 5, 4, c, r, actors[4], actors[3], ledgers[4], ledgers[3]); err == nil {
		t.Fatal("claim=0 forged acceptance despite score 2")
	}
	c.Accept = false
	refused, l, err := SocialFoodRefuse(0, 5, 4, c, r, actors[4], ledgers[4])
	if err != nil || refused.Status != SocialFoodRefused || refused.Refusals != 1 || l.LastReplyHour != 0 || actors[4].Bag.Units != 1 || actors[3].Bag.Units != 0 {
		t.Fatalf("refusal boundary %+v %+v %v", refused, l, err)
	}
	// Request 1 already has a reply: forging its status back to pending must
	// not allow either a second gift or another refusal against that same ID.
	for _, answered := range []SocialFoodRequestState{{ID: r.ID, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodPending, Requests: 1, Gifts: 1}, {ID: r.ID, Hour: 0, Addressee: 5, Claim: 0, Status: SocialFoodPending, Requests: 1, Refusals: 1}} {
		if validSocialFoodRequest(4, answered) {
			t.Fatalf("answered request still pending %+v", answered)
		}
		if _, err := SocialFoodGiftProposal("answered", at, 0, 5, 4, SocialFoodConsent{RequestID: r.ID, Donor: 5, Hour: 0, Accept: true}, answered, actors[4], actors[3], ledgers[4], ledgers[3]); err == nil {
			t.Fatal("answered request gifted again")
		}
	}
	// Prior completed outcomes are allowed, but the current pending request
	// must remain outstanding (strictly more requests than answers).
	prior := SocialFoodRequestState{ID: socialFoodRequestID(1, 4), Hour: 1, Addressee: 5, Claim: 2, Status: SocialFoodPending, Requests: 2, Gifts: 1}
	if !validSocialFoodRequest(4, prior) {
		t.Fatal("fresh pending after prior gift rejected")
	}
}

// Kernel rules are schema labels, not semantic transition validators. A raw
// caller holding trusted authority can commit status-only in-range gift-rule
// patches without a donor, consent token, or corresponding bag delta.
func TestSocialFoodRawKernelGiftPatchBypassesHelper(t *testing.T) {
	reg, err := SocialFoodRegistry()
	if err != nil {
		t.Fatal(err)
	}
	addressee, _ := sim.EntityRefValue(5)
	seeds := []component.ComponentSeed{{Entity: 4, Component: SocialFoodRequestTypeID, Fields: []component.FieldSeed{
		{Field: SocialFoodRequestIDField, Value: sim.IntegerValue(4)}, {Field: SocialFoodRequestHourField, Value: sim.IntegerValue(0)}, {Field: SocialFoodRequestAddresseeField, Value: addressee}, {Field: SocialFoodRequestClaimField, Value: sim.IntegerValue(2)}, {Field: SocialFoodRequestStatusField, Value: sim.IntegerValue(int64(SocialFoodPending))}, {Field: SocialFoodRequestsWitnessedField, Value: sim.IntegerValue(1)}, {Field: SocialFoodGiftsWitnessedField, Value: sim.IntegerValue(0)}, {Field: SocialFoodRefusalsWitnessedField, Value: sim.IntegerValue(0)},
	}}}
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, authority, _ := k.Snapshot()
	proposal := kernel.Proposal{Key: "forged-status", Time: 1, Cause: kernel.Cause{Actor: 4}, Rule: SocialFoodGiftRule, RuleVersion: SocialFoodRuleVersion, Patches: []component.Patch{{Entity: 4, Component: SocialFoodRequestTypeID, SchemaVersion: SocialFoodSchemaVersion, Field: SocialFoodRequestStatusField, Value: sim.IntegerValue(int64(SocialFoodAccepted))}}}
	plan, err := k.Plan(proposal, authority)
	if err != nil {
		t.Fatalf("expected raw kernel schema admission: %v", err)
	}
	events, err := k.CommitBatch([]kernel.Plan{plan})
	if err != nil || len(events) != 1 || len(events[0].Deltas) != 1 {
		t.Fatalf("raw unpaired gift patch: %d %v", len(events), err)
	}
	// The pure validator rejects the resulting fabricated accepted status, but
	// schema-only kernel replay cannot; runner/journal must verify linkage.
	if validSocialFoodRequest(4, SocialFoodRequestState{ID: 4, Hour: 0, Addressee: 5, Claim: 2, Status: SocialFoodAccepted, Requests: 1}) {
		t.Fatal("fabricated accepted state passed semantic validation")
	}
}
func TestSocialFoodInvalidKernelProposalGuard(t *testing.T) {
	reg, err := SocialFoodRegistry()
	if err != nil {
		t.Fatal(err)
	}
	seeds := []component.ComponentSeed{{Entity: 5, Component: SocialFoodBagTypeID, Fields: []component.FieldSeed{{Field: SocialFoodBagUnitsField, Value: sim.IntegerValue(0)}}}}
	k, err := kernel.New(reg, 0, seeds)
	if err != nil {
		t.Fatal(err)
	}
	_, auth, _ := k.Snapshot()
	// A gift rule cannot patch unrelated old component IDs or out-of-bound v2 units.
	for _, p := range []component.Patch{{Entity: 5, Component: FoodFlowBagTypeID, SchemaVersion: FoodFlowSchemaVersion, Field: FoodFlowBagUnitsField, Value: sim.IntegerValue(1)}, {Entity: 5, Component: SocialFoodBagTypeID, SchemaVersion: SocialFoodSchemaVersion, Field: SocialFoodBagUnitsField, Value: sim.IntegerValue(2)}} {
		if _, err := k.Plan(kernel.Proposal{Key: "bad", Time: 1, Cause: kernel.Cause{Actor: 5}, Rule: SocialFoodGiftRule, RuleVersion: SocialFoodRuleVersion, Patches: []component.Patch{p}}, auth); err == nil {
			t.Fatalf("invalid kernel patch admitted: %+v", p)
		}
	}
}
