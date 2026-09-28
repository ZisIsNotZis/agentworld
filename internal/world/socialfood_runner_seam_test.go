package world

import (
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

func socialFoodAt(t *testing.T, f *SocialFood, at sim.SimTime) {
	t.Helper()
	for {
		snapshot := f.SchedulerSnapshot()
		if len(snapshot.Wakes) == 0 || snapshot.Wakes[0].At > at {
			t.Fatalf("missing wake at %d", at)
		}
		if snapshot.Wakes[0].At == at {
			return
		}
		ok, err := f.Step(context.Background())
		if err != nil || !ok {
			t.Fatalf("step to %d: %v", at, err)
		}
	}
}
func socialFoodAuthoritative(t *testing.T, f *SocialFood) ([2]SocialFoodPatchState, [2][8]SocialFoodSlotState, [16]SocialFoodActorState, [16]SocialFoodRequestState, [16]SocialFoodLedgerState) {
	t.Helper()
	head := f.k.SnapshotHead()
	p, s, a, r, l, e := socialFoodStates(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version})
	if e != nil {
		t.Fatal(e)
	}
	return p, s, a, r, l
}
func TestSocialFoodRunnerPhaseFinalizationAndNoPendingAtPulse(t *testing.T) {
	f, e := NewSocialFood(SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4, Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	expired, answered := 0, 0
	for h := 0; h < 24; h++ {
		for phase := 0; phase <= 4; phase++ {
			at, _ := SocialFoodPhaseTime(h, phase)
			socialFoodAt(t, f, at)
			ok, err := f.Step(context.Background())
			if err != nil || !ok {
				t.Fatalf("h%d phase%d step: %v", h, phase, err)
			}
			_, _, _, requests, _ := socialFoodAuthoritative(t, f)
			for i, r := range requests {
				if phase >= 3 && r.Status == SocialFoodPending {
					t.Fatalf("h%d phase%d actor%d pending beyond finalization", h, phase, i+1)
				}
				if phase == 3 && r.Hour == int64(h) {
					if r.Status == SocialFoodExpired {
						expired++
					}
					if r.Status == SocialFoodRefused || r.Status == SocialFoodAccepted {
						answered++
					}
				}
			}
		}
		socialFoodAt(t, f, sim.SimTime(h+1)*sim.SimTime(SocialFoodHour))
		_, _, _, requests, _ := socialFoodAuthoritative(t, f)
		for i, r := range requests {
			if r.Status == SocialFoodPending {
				t.Fatalf("h%d actor%d pending before pulse", h+1, i+1)
			}
		}
	}
	if expired == 0 || answered == 0 {
		t.Fatalf("no expiry/answer fixtures %d/%d", expired, answered)
	}
	var firstExpiry SocialFoodAttempt
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Kind == "expire" && firstExpiry.Kind == "" {
				firstExpiry = a
			}
		}
	}
	expiredRunner := socialFoodRunTo(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}, 0)
	socialFoodAt(t, expiredRunner, firstExpiry.Time)
	if ok, err := expiredRunner.Step(context.Background()); !ok || err != nil {
		t.Fatalf("expiry commit: %v", err)
	}
	_, _, expiredActors, expiredRequests, expiredLedgers := socialFoodAuthoritative(t, expiredRunner)
	hour := int(firstExpiry.Time / sim.SimTime(SocialFoodHour))
	owner, donor := firstExpiry.Actor, firstExpiry.Target
	if expiredRequests[owner-1].Status != SocialFoodExpired {
		t.Fatalf("not expired: %+v", expiredRequests[owner-1])
	}
	if _, err := SocialFoodGiftProposal("expired-gift", firstExpiry.Time-1, hour, donor, owner,
		SocialFoodConsent{RequestID: firstExpiry.RequestID, Donor: donor, Hour: hour, Accept: true},
		expiredRequests[owner-1], expiredActors[donor-1], expiredActors[owner-1], expiredLedgers[donor-1], expiredLedgers[owner-1]); err == nil {
		t.Fatal("expired request accepted a gift")
	}
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			h := int(a.Time / sim.SimTime(SocialFoodHour))
			phase := -1
			switch a.Kind {
			case "gather":
				phase = 0
			case "request":
				phase = 1
			case "gift", "refuse":
				phase = 2
			case "expire":
				phase = 3
			case "eat":
				phase = 4
			}
			if phase >= 0 {
				want, _ := SocialFoodPhaseTime(h, phase)
				if a.Time != want {
					t.Fatalf("wrong phase %+v expected %d", a, want)
				}
			}
		}
	}
	t.Logf("same-hour finalizations: expired=%d answered=%d", expired, answered)
}
func TestSocialFoodRunnerGiftRefusalAndCheckedRejections(t *testing.T) {
	f := socialFoodRunTo(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4, Enabled: true}, 24)
	var gift SocialFoodAttempt
	lowReserve := 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Kind == "gift" && gift.Kind == "" {
				gift = a
			}
			if a.Kind == "refuse" && a.DonorEnergy == 3 {
				lowReserve++
			}
		}
	}
	if gift.Kind == "" || gift.Score < 3 || gift.DonorEnergy < 4 || gift.Claim != 2 || gift.Truth <= 0 || lowReserve == 0 {
		t.Fatalf("missing truthful score gift or low-reserve refusal: gift=%+v low=%d", gift, lowReserve)
	}
	at := gift.Time
	hour := int(at / sim.SimTime(SocialFoodHour))
	before := socialFoodRunTo(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}, 0)
	socialFoodAt(t, before, at)
	p, s, actors, requests, ledgers := socialFoodAuthoritative(t, before)
	donor, recipient := gift.Actor, gift.Target
	c := SocialFoodConsent{RequestID: gift.RequestID, Donor: donor, Hour: hour, Accept: true}
	proposal, err := SocialFoodGiftProposal("checked-valid", at, hour, donor, recipient, c, requests[recipient-1], actors[donor-1], actors[recipient-1], ledgers[donor-1], ledgers[recipient-1])
	if err != nil || proposal.Rule != SocialFoodGiftRule {
		t.Fatalf("valid independently scored gift: %v", err)
	}
	foreign := sim.EntityID(9)
	if recipient >= 9 {
		foreign = 1
	}
	if _, err = SocialFoodGiftProposal("foreign", at, hour, donor, foreign, c, requests[recipient-1], actors[donor-1], actors[recipient-1], ledgers[donor-1], ledgers[recipient-1]); err == nil {
		t.Fatal("foreign recipient accepted")
	}
	ok, err := before.Step(context.Background())
	if !ok || err != nil {
		t.Fatalf("gift commit: %v", err)
	}
	p2, s2, a2, r2, l2 := socialFoodAuthoritative(t, before)
	if p != p2 || s != s2 || actors[donor-1].Bag.Units != 1 || actors[recipient-1].Bag.Units != 0 || a2[donor-1].Bag.Units != 0 || a2[recipient-1].Bag.Units != 1 || a2[recipient-1].Bag.Source != actors[donor-1].Bag.Source || a2[recipient-1].Bag.Gatherer != actors[donor-1].Bag.Gatherer || a2[recipient-1].Bag.LastDonor != donor || r2[recipient-1].Status != SocialFoodAccepted || l2[donor-1].Given != ledgers[donor-1].Given+1 {
		t.Fatalf("non-atomic gift donor=%+v -> %+v recipient=%+v -> %+v", actors[donor-1], a2[donor-1], actors[recipient-1], a2[recipient-1])
	}
	if _, err = SocialFoodGiftProposal("duplicate", at, hour, donor, recipient, c, r2[recipient-1], a2[donor-1], a2[recipient-1], l2[donor-1], l2[recipient-1]); err == nil {
		t.Fatal("duplicate accepted")
	}
	finalize, _ := SocialFoodPhaseTime(hour, 3)
	socialFoodAt(t, before, finalize)
	if _, err = SocialFoodExpireProposal("answered-expiry", hour, finalize, recipient, r2[recipient-1]); err == nil {
		t.Fatal("accepted gift expired")
	}
	ok, err = before.Step(context.Background())
	if !ok || err != nil {
		t.Fatalf("finalize: %v", err)
	}
	meal, _ := SocialFoodPhaseTime(hour, 4)
	socialFoodAt(t, before, meal)
	ok, err = before.Step(context.Background())
	if !ok || err != nil {
		t.Fatalf("meal: %v", err)
	}
	_, _, a3, _, _ := socialFoodAuthoritative(t, before)
	if a3[recipient-1].Consumed != a2[recipient-1].Consumed+1 || a3[recipient-1].Energy != a2[recipient-1].Energy+1 || a3[recipient-1].Hunger != a2[recipient-1].Hunger-1 || a3[donor-1].Consumed != a2[donor-1].Consumed || a3[donor-1].Energy != a2[donor-1].Energy {
		t.Fatalf("recipient/donor Eat consequence: %+v/%+v -> %+v/%+v", a2[recipient-1], a2[donor-1], a3[recipient-1], a3[donor-1])
	}
	t.Logf("gift h%d %d->%d score%d donor energy%d recipient truth%d; low-reserve refusals=%d; recipient Eat/energy/hunger %d/%d/%d -> %d/%d/%d; donor Eat/energy %d/%d -> %d/%d", hour, donor, recipient, gift.Score, gift.DonorEnergy, gift.Truth, lowReserve, a2[recipient-1].Consumed, a2[recipient-1].Energy, a2[recipient-1].Hunger, a3[recipient-1].Consumed, a3[recipient-1].Energy, a3[recipient-1].Hunger, a2[donor-1].Consumed, a2[donor-1].Energy, a3[donor-1].Consumed, a3[donor-1].Energy)
}
func TestSocialFoodRunnerFalseClaimDoesNotRevealTruth(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4, Enabled: true, TestClaimOverrides: map[sim.EntityID]int64{8: 0}}
	f := socialFoodRunTo(t, opts, 12)
	opts.TestClaimOverrides[8] = 2
	handoff, err := f.Handoff()
	if err != nil || handoff.TestClaimOverrides[8] != 0 {
		t.Fatalf("unrecorded or aliased intervention: %v", err)
	}
	requests, refusals := 0, 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			if a.Actor == 8 && a.Kind == "request" {
				requests++
				if a.Claim != 0 || a.Truth < 1 || a.Truth > 8 || a.Target != 1 {
					t.Fatalf("override leaked into eligibility/target %+v", a)
				}
			}
			if a.Actor == 1 && a.Target == 8 && a.Kind == "refuse" && a.Claim == 0 {
				refusals++
				if a.Score >= 3 {
					t.Fatalf("false claim raised score %+v", a)
				}
			}
		}
	}
	if requests == 0 || refusals == 0 {
		t.Fatalf("missing false claim and independent refusal %d/%d", requests, refusals)
	}
	for _, ev := range f.k.Events() {
		if ev.Rule != SocialFoodRequestRule && ev.Rule != SocialFoodGiftRule && ev.Rule != SocialFoodRefuseRule {
			continue
		}
		for _, d := range ev.Deltas {
			if d.Component == SocialFoodBodyTypeID {
				t.Fatalf("private reserve published by social event %d", ev.ID)
			}
		}
	}
	t.Logf("actor8 false-claim requests=%d actor1 claim-only refusals=%d", requests, refusals)
}

type socialFoodCancelAfterEvaluation struct {
	context.Context
	cancel           context.CancelFunc
	runner           *SocialFood
	checks, attempts int
}

func (c *socialFoodCancelAfterEvaluation) Err() error {
	c.checks++
	if c.runner.pending != nil {
		c.runner.pending.mu.Lock()
		c.attempts = len(c.runner.pending.attempts)
		c.runner.pending.mu.Unlock()
	}
	c.cancel()
	return c.Context.Err()
}
func TestSocialFoodRunnerFailedReplyRollbackAndRetry(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4, Enabled: true}
	interrupted := socialFoodRunTo(t, opts, 0)
	control := socialFoodRunTo(t, opts, 0)
	// A later reply batch includes independently evaluated donor decisions.
	var at sim.SimTime
	for h := 1; h < 12; h++ {
		candidate, _ := SocialFoodPhaseTime(h, 2)
		socialFoodAt(t, interrupted, candidate)
		_, _, _, requests, _ := socialFoodAuthoritative(t, interrupted)
		for _, r := range requests {
			if r.Hour == int64(h) && r.Status == SocialFoodPending {
				at = candidate
				break
			}
		}
		if at != 0 {
			break
		}
		ok, e := interrupted.Step(context.Background())
		if e != nil || !ok {
			t.Fatalf("reply step: %v", e)
		}
	}
	if at == 0 {
		t.Fatal("missing evaluated reply fixture")
	}
	socialFoodAt(t, control, at)
	before, err := interrupted.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	sched := interrupted.SchedulerSnapshot()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	late := &socialFoodCancelAfterEvaluation{Context: base, cancel: cancel, runner: interrupted}
	processed, err := interrupted.Step(late)
	if processed || !errors.Is(err, context.Canceled) || late.checks != 1 || late.attempts == 0 {
		t.Fatalf("cancelled reply processed=%v err=%v checks=%d evaluated=%d", processed, err, late.checks, late.attempts)
	}
	after, err := interrupted.Handoff()
	if err != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(sched, interrupted.SchedulerSnapshot()) || interrupted.pending != nil {
		t.Fatal("failed reply changed authoritative or runner state")
	}
	for _, r := range []*SocialFood{interrupted, control} {
		processed, err = r.Step(context.Background())
		if err != nil || !processed {
			t.Fatalf("retry/control: %v", err)
		}
	}
	retried, err := interrupted.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	uninterrupted, err := control.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retried.History, uninterrupted.History) || !reflect.DeepEqual(retried, uninterrupted) {
		t.Fatal("retry diverged from uninterrupted reply")
	}
	t.Logf("cancelled h%d reply with %d evaluated donor decisions; retry matched", int(at/sim.SimTime(SocialFoodHour)), late.attempts)
}
