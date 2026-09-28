package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"errors"
	"reflect"
	"testing"
)

func socialFoodFixture(t *testing.T, actor sim.EntityID) (*strategy.SocialFoodBound, strategy.SocialFoodPolicy) {
	t.Helper()
	p := strategy.FrozenSocialFoodPolicy(strategy.SocialFoodRef{ID: "social-v2-pilot", Version: 2})
	r := strategy.NewSocialFoodRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.SocialFoodBinding{Actor: actor, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b, p
}

func socialFoodRequestView(actor sim.EntityID, ref strategy.SocialFoodRef) strategy.SocialFoodRequestObservation {
	patch := sim.EntityID(3001)
	if actor > 8 {
		patch = 3002
	}
	return strategy.SocialFoodRequestObservation{Actor: actor, Ref: ref, Patch: patch, Hour: 0, WorldVersion: 7,
		OwnEnergy: sim.IntegerValue(8), OwnBagUnits: sim.IntegerValue(0), DeniedGatherHour: 0,
		OwnRequest: strategy.SocialFoodRequestGuard{Hour: -1, Status: strategy.SocialFoodIdle}}
}

func socialFoodReplyView(donor, requester sim.EntityID, ref strategy.SocialFoodRef, claim int64) strategy.SocialFoodReplyObservation {
	patch := sim.EntityID(3001)
	if donor > 8 {
		patch = 3002
	}
	return strategy.SocialFoodReplyObservation{Actor: donor, Ref: ref, Patch: patch, Hour: 0, WorldVersion: 9,
		OwnEnergy: sim.IntegerValue(7), OwnHunger: sim.IntegerValue(0), OwnBagUnits: sim.IntegerValue(1), OwnLastReplyHour: -1,
		Pending: []strategy.SocialFoodPendingView{{Requester: requester, Addressee: donor, RequestID: int64(requester), Hour: 0, Claim: claim}}}
}

func TestSocialFoodDirectedRequestAndIndependentHighAffinityConsent(t *testing.T) {
	for actor := sim.EntityID(1); actor <= 16; actor++ {
		b, p := socialFoodFixture(t, actor)
		obs := socialFoodRequestView(actor, p.Ref)
		c, err := b.EvaluateRequest(obs)
		target, affinity, worldErr := world.SocialFoodKnownAddressee(actor)
		if err != nil || worldErr != nil || c.Kind != strategy.SocialFoodRequest || c.Target != target || c.Claim != 2 || c.Ref != p.Ref || c.Evaluated != 1 || c.ObservedVersion != 7 || p.Ties[actor-1] != (strategy.SocialFoodTie{Target: target, Affinity: affinity}) {
			t.Fatalf("actor %d request %+v err %v world %v", actor, c, err, worldErr)
		}
	}
	b, p := socialFoodFixture(t, 5)
	obs := socialFoodReplyView(5, 4, p.Ref, 2)
	c, err := b.EvaluateReply(obs)
	if err != nil || c.Kind != strategy.SocialFoodAccept || c.Target != 4 || c.RequestID != 4 || c.Score != 4 || c.Evaluated != 1 || c.ObservedVersion != 9 {
		t.Fatalf("high affinity consent: %+v %v", c, err)
	}
	// The decision is distinct from physical transfer; the world still needs a
	// checked pending request, donor/recipient states, and a bound consent token.
	if _, _, _, _, _, err := world.SocialFoodAccept(0, 5, 4, world.SocialFoodConsent{RequestID: c.RequestID, Donor: 5, Hour: 0, Accept: true}, world.SocialFoodRequestState{}, world.SocialFoodActorState{}, world.SocialFoodActorState{}, world.SocialFoodLedgerState{}, world.SocialFoodLedgerState{}); err == nil {
		t.Fatal("policy intention alone transferred food")
	}
}

func TestSocialFoodReserveScoreAndReportedClaimPrivacy(t *testing.T) {
	b, p := socialFoodFixture(t, 4)
	obs := socialFoodRequestView(4, p.Ref)
	claim := int64(0)
	obs.TestClaimOverride = &claim
	request, err := b.EvaluateRequest(obs)
	if err != nil || request.Kind != strategy.SocialFoodRequest || request.Target != 5 || request.Claim != 0 || !reflect.DeepEqual(obs.OwnEnergy, sim.IntegerValue(8)) {
		t.Fatalf("override changed private eligibility or identity: %+v %v", request, err)
	}
	obs.OwnEnergy = sim.IntegerValue(9)
	if c, err := b.EvaluateRequest(obs); err != nil || c.Kind != strategy.SocialFoodWait {
		t.Fatalf("false claim bypassed private energy: %+v %v", c, err)
	}
	donor, _ := socialFoodFixture(t, 5)
	view := socialFoodReplyView(5, 4, p.Ref, request.Claim)
	decision, err := donor.EvaluateReply(view)
	if err != nil || decision.Kind != strategy.SocialFoodRefuse || decision.Score != 2 {
		t.Fatalf("claim 0 must not be treated as true urgency: %+v %v", decision, err)
	}
	view.Pending[0].Claim = 2
	view.OwnEnergy = sim.IntegerValue(3)
	decision, err = donor.EvaluateReply(view)
	if err != nil || decision.Kind != strategy.SocialFoodRefuse || decision.Score != 3 {
		t.Fatalf("reserve floor refusal: %+v %v", decision, err)
	}
	view.OwnEnergy = sim.IntegerValue(6)
	view.OwnHunger = sim.IntegerValue(10)
	decision, err = donor.EvaluateReply(view)
	if err != nil || decision.Kind != strategy.SocialFoodRefuse || decision.Score != 2 {
		t.Fatalf("own hunger/energy penalties: %+v %v", decision, err)
	}
	view.OwnEnergy = sim.IntegerValue(7)
	view.OwnHunger = sim.IntegerValue(0)
	view.OwnBagUnits = sim.IntegerValue(0)
	decision, err = donor.EvaluateReply(view)
	if err != nil || decision.Kind != strategy.SocialFoodRefuse || decision.Score != 4 {
		t.Fatalf("no held meal to give: %+v %v", decision, err)
	}
	// Donor input has no authoritative requester energy or history. Different
	// private requester energies cannot affect an otherwise identical view.
	view.OwnBagUnits = sim.IntegerValue(1)
	first, err := donor.EvaluateReply(view)
	if err != nil || first.Kind != strategy.SocialFoodAccept || !reflect.DeepEqual(first, decisionWithKind(decision, strategy.SocialFoodAccept)) {
		t.Fatalf("public-only consent: %+v %v", first, err)
	}
	lowAffinityDonor, _ := socialFoodFixture(t, 2)
	lowAffinity, err := lowAffinityDonor.EvaluateReply(socialFoodReplyView(2, 1, p.Ref, 2))
	if err != nil || lowAffinity.Kind != strategy.SocialFoodRefuse || lowAffinity.Score != 2 {
		t.Fatalf("low-affinity directed score: %+v %v", lowAffinity, err)
	}
	// Keep the donor observation's type-level privacy boundary visible in
	// tests; neither the addressed claim nor the choice includes private truth.
	for _, typ := range []reflect.Type{reflect.TypeOf(strategy.SocialFoodReplyObservation{}), reflect.TypeOf(strategy.SocialFoodPendingView{})} {
		for _, forbidden := range []string{"RequesterEnergy", "RequesterHunger", "RequesterHistory", "RequesterBag", "RequesterDenial"} {
			if _, present := typ.FieldByName(forbidden); present {
				t.Fatalf("donor view leaked %s", forbidden)
			}
		}
	}
}

func decisionWithKind(c strategy.SocialFoodChoice, kind strategy.SocialFoodAction) strategy.SocialFoodChoice {
	c.Kind = kind
	return c
}

func TestSocialFoodHourlyGuardsBindingsAndFallback(t *testing.T) {
	b, p := socialFoodFixture(t, 8)
	obs := socialFoodRequestView(8, p.Ref)
	for _, guard := range []strategy.SocialFoodRequestGuard{
		{Hour: 0, Status: strategy.SocialFoodPending}, {Hour: 0, Status: strategy.SocialFoodAccepted},
		{Hour: 0, Status: strategy.SocialFoodRefused}, {Hour: 0, Status: strategy.SocialFoodWithdrawn},
	} {
		obs.OwnRequest = guard
		if c, err := b.EvaluateRequest(obs); err != nil || c.Kind != strategy.SocialFoodWait {
			t.Fatalf("second request after %+v: %+v %v", guard, c, err)
		}
	}
	obs.OwnRequest = strategy.SocialFoodRequestGuard{Hour: -1, Status: strategy.SocialFoodIdle}
	obs.DeniedGatherHour = -1
	if c, err := b.EvaluateRequest(obs); err != nil || c.Kind != strategy.SocialFoodWait {
		t.Fatalf("no own denial: %+v %v", c, err)
	}
	obs.DeniedGatherHour = 0
	obs.OwnEnergy = sim.IntegerValue(0)
	if c, err := b.EvaluateRequest(obs); err != nil || c.Kind != strategy.SocialFoodWait {
		t.Fatalf("dead requester: %+v %v", c, err)
	}
	obs.OwnEnergy = sim.BoolValue(true)
	if c, err := b.EvaluateRequest(obs); err != nil || c.Fallback != strategy.SocialFoodInvalidSelf {
		t.Fatalf("unknown own energy: %+v %v", c, err)
	}
	obs.OwnEnergy = sim.IntegerValue(8)
	for _, invalid := range []strategy.SocialFoodRequestObservation{
		func() strategy.SocialFoodRequestObservation { o := obs; o.Actor = 7; return o }(),
		func() strategy.SocialFoodRequestObservation { o := obs; o.Ref.Version++; return o }(),
		func() strategy.SocialFoodRequestObservation { o := obs; o.Patch = 3002; return o }(),
		func() strategy.SocialFoodRequestObservation { o := obs; o.OwnRequest.Hour = 1; return o }(),
	} {
		if _, err := b.EvaluateRequest(invalid); !errors.Is(err, strategy.ErrInvalidSocialFoodObservation) {
			t.Fatalf("invalid request identity/guard: %+v %v", invalid, err)
		}
	}
	donor, _ := socialFoodFixture(t, 1)
	reply := socialFoodReplyView(1, 8, p.Ref, 2)
	if c, err := donor.EvaluateReply(reply); err != nil || c.Kind != strategy.SocialFoodAccept || c.Score != 4 {
		t.Fatalf("wraparound dyad: %+v %v", c, err)
	}
	reply.OwnLastReplyHour = 0
	if c, err := donor.EvaluateReply(reply); err != nil || c.Kind != strategy.SocialFoodWait {
		t.Fatalf("duplicate reply: %+v %v", c, err)
	}
	reply.OwnLastReplyHour = -1
	for _, invalid := range []strategy.SocialFoodReplyObservation{
		func() strategy.SocialFoodReplyObservation { o := reply; o.Actor = 2; return o }(),
		func() strategy.SocialFoodReplyObservation { o := reply; o.Ref.ID = "changed"; return o }(),
		func() strategy.SocialFoodReplyObservation {
			o := reply
			o.Pending = append([]strategy.SocialFoodPendingView(nil), reply.Pending...)
			o.Pending[0].Requester = 16
			return o
		}(),
		func() strategy.SocialFoodReplyObservation {
			o := reply
			o.Pending = append([]strategy.SocialFoodPendingView(nil), reply.Pending...)
			o.Pending[0].Addressee = 2
			return o
		}(),
		func() strategy.SocialFoodReplyObservation {
			o := reply
			o.Pending = append([]strategy.SocialFoodPendingView(nil), reply.Pending...)
			o.Pending[0].RequestID++
			return o
		}(),
		func() strategy.SocialFoodReplyObservation {
			o := reply
			o.Pending = append([]strategy.SocialFoodPendingView(nil), reply.Pending...)
			o.Pending = append(o.Pending, o.Pending[0])
			return o
		}(),
	} {
		if _, err := donor.EvaluateReply(invalid); !errors.Is(err, strategy.ErrInvalidSocialFoodObservation) {
			t.Fatalf("invalid addressed request: %+v %v", invalid, err)
		}
	}
	limited := strategy.NewSocialFoodRegistry()
	q := p
	q.Budget = strategy.SocialFoodBudget{Candidates: 1, Evaluations: 1}
	if err := limited.Register(q); err != nil {
		t.Fatal(err)
	}
	if err := limited.Register(q); !errors.Is(err, strategy.ErrInvalidSocialFoodPolicy) {
		t.Fatalf("same-ref duplicate registration: %v", err)
	}
	if _, err := limited.Bind(strategy.SocialFoodBinding{Actor: 0, Ref: q.Ref}); !errors.Is(err, strategy.ErrInvalidSocialFoodBinding) {
		t.Fatalf("invalid actor binding: %v", err)
	}
	if _, err := limited.Bind(strategy.SocialFoodBinding{Actor: 1, Ref: strategy.SocialFoodRef{ID: q.Ref.ID, Version: 1}}); !errors.Is(err, strategy.ErrUnknownSocialFoodPolicy) {
		t.Fatalf("unknown version: %v", err)
	}
}
