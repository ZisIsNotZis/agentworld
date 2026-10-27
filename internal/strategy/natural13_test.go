package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"errors"
	"reflect"
	"testing"
)

// natural13Fixture binds one actor to the frozen natural/13 policy through the
// ref-set-aware registration path.
func natural13Fixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy13()
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := r.RegisterFrozenNatural(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.NaturalBinding{Actor: actor, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b, p.Ref
}

// TestNatural13SamePatchChoicesMatchNatural12 proves the differential claim on
// a same-patch roster: with the ref field normalised, every natural/13 choice
// equals natural/12's for the same observation (the band-wide pool only widens
// what the world feeds in; the cascade itself is unchanged).
func TestNatural13SamePatchChoicesMatchNatural12(t *testing.T) {
	b13, ref13 := natural13Fixture(t, 1)
	b12, ref12 := natural12Fixture(t, 1)

	for _, energy := range []int64{0, 3, 4, 5} {
		for _, capital := range []int64{0, 1, 8} {
			for _, wip := range []int64{0, 1} {
				for _, granary := range []int64{0, 1, 3} {
					for _, bag := range []int64{0, 1} {
						obs := func(ref strategy.NaturalRef) strategy.NaturalObservation {
							o := naturalObservation(t, 1, ref, 5)
							o.Energy = sim.IntegerValue(energy)
							o.Capital = sim.IntegerValue(capital)
							o.Wip = sim.IntegerValue(wip)
							o.GranaryStock = sim.IntegerValue(granary)
							o.BagUnits = sim.IntegerValue(bag)
							return o
						}
						got13, err13 := b13.Evaluate13(obs(ref13))
						got12, err12 := b12.Evaluate12(obs(ref12))
						if !errors.Is(err13, err12) {
							t.Fatalf("claim grid E%d K%d W%d G%d B%d: errors %v vs %v", energy, capital, wip, granary, bag, err13, err12)
						}
						got13.Ref, got12.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
						if got13 != got12 {
							t.Fatalf("claim grid E%d K%d W%d G%d B%d: natural/13 %+v != natural/12 %+v", energy, capital, wip, granary, bag, got13, got12)
						}
					}
				}
			}
		}
	}

	views := []strategy.NaturalMateView{
		{ID: 2, KinRelated: true, KinDistance: 1},
		{ID: 3, KinRelated: false, KinDistance: 0},
		{ID: 4, KinRelated: false, KinDistance: 0},
	}
	for _, hour := range []int{0, 1, 2, 7} {
		for _, energy := range []int64{2, 4, 8} {
			obs := func(ref strategy.NaturalRef) strategy.NaturalRequestObservation {
				o := naturalRequestView(t, 1, ref, hour)
				o.PatchMates = []sim.EntityID{2, 3, 4}
				o.MateViews = views
				o.Energy = sim.IntegerValue(energy)
				return o
			}
			got13, err13 := b13.EvaluateBirthRequest13(obs(ref13))
			got12, err12 := b12.EvaluateBirthRequest12(obs(ref12))
			if !errors.Is(err13, err12) {
				t.Fatalf("request hour %d E%d: errors %v vs %v", hour, energy, err13, err12)
			}
			got13.Ref, got12.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
			if !reflect.DeepEqual(got13, got12) {
				t.Fatalf("request hour %d E%d: natural/13 %+v != natural/12 %+v", hour, energy, got13, got12)
			}
		}
	}
}

// TestNatural13ReplyConsentFloor proves the one behavioural difference: a
// consenter at the household gate (energy 2, granary 0, capital 0) consents
// under natural/13 while natural/12's filing floor refuses consent-own.
func TestNatural13ReplyConsentFloor(t *testing.T) {
	b13, ref13 := natural13Fixture(t, 1)
	b12, ref12 := natural12Fixture(t, 1)

	obs := func(ref strategy.NaturalRef, energy, granary, capital int64) strategy.NaturalReplyObservation {
		o := naturalReplyView(1, 3, ref, 0)
		o.Energy = sim.IntegerValue(energy)
		o.GranaryStock = sim.IntegerValue(granary)
		o.Capital = sim.IntegerValue(capital)
		return o
	}
	got13, err := b13.EvaluateBirthReply13(obs(ref13, 2, 0, 0))
	if err != nil {
		t.Fatalf("natural/13 household consenter: %v", err)
	}
	if got13.Kind != strategy.NaturalReplyAccept {
		t.Fatalf("natural/13 consent at energy 2: kind %v reason %v, want accept", got13.Kind, got13.Reason)
	}
	got12, err := b12.EvaluateBirthReply12(obs(ref12, 2, 0, 0))
	if err != nil {
		t.Fatalf("natural/12 household consenter: %v", err)
	}
	if got12.Kind != strategy.NaturalReplyRefuse || got12.Reason != strategy.NaturalReplyReasonConsentOwn {
		t.Fatalf("natural/12 consent at energy 2: %+v, want consent-own refusal", got12)
	}
	if _, err := b13.EvaluateBirthReply13(obs(ref13, 1, 3, 2)); err != nil {
		t.Fatalf("natural/13 energy 1: %v", err)
	}
	refused, _ := b13.EvaluateBirthReply13(obs(ref13, 1, 3, 2))
	if refused.Kind != strategy.NaturalReplyRefuse || refused.Reason != strategy.NaturalReplyReasonConsentOwn {
		t.Fatalf("natural/13 consent at energy 1: %+v, want consent-own refusal", refused)
	}
}

// TestNatural13BandMatePool proves the proposer side accepts the band-wide
// roster mechanism v8 supplies: a non-kin mate on the other patch is a legal
// addressee, while natural/12's same-patch roster contract refuses it.
func TestNatural13BandMatePool(t *testing.T) {
	b13, ref13 := natural13Fixture(t, 1)
	b12, ref12 := natural12Fixture(t, 1)

	views := []strategy.NaturalMateView{{ID: 9, KinRelated: false, KinDistance: 0}}
	obs := func(ref strategy.NaturalRef) strategy.NaturalRequestObservation {
		o := naturalRequestView(t, 1, ref, 3)
		o.PatchMates = []sim.EntityID{9}
		o.MateViews = views
		return o
	}
	got, err := b13.EvaluateBirthRequest13(obs(ref13))
	if err != nil {
		t.Fatalf("natural/13 cross-patch mate: %v", err)
	}
	if got.Kind != strategy.NaturalRequestFile || got.Addressee != 9 {
		t.Fatalf("natural/13 cross-patch mate: %+v, want file to 9", got)
	}
	if _, err := b12.EvaluateBirthRequest12(obs(ref12)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("natural/12 accepted a cross-patch founder mate: %v", err)
	}
}

// TestNatural13BoundIdentityFailsClosed proves the frozen 13 entry points take
// only a natural/13 bound and that the natural/12 cascades refuse one.
func TestNatural13BoundIdentityFailsClosed(t *testing.T) {
	b13, ref13 := natural13Fixture(t, 1)
	b12, ref12 := natural12Fixture(t, 1)

	claim := func(ref strategy.NaturalRef) strategy.NaturalObservation {
		o := naturalObservation(t, 1, ref, 5)
		o.Energy = sim.IntegerValue(4)
		o.GranaryStock = sim.IntegerValue(1)
		return o
	}
	views := []strategy.NaturalMateView{{ID: 2, KinRelated: false}}
	request := func(ref strategy.NaturalRef) strategy.NaturalRequestObservation {
		o := naturalRequestView(t, 1, ref, 0)
		o.PatchMates = []sim.EntityID{2}
		o.MateViews = views
		return o
	}
	if _, err := b12.Evaluate13(claim(ref12)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/13 claim accepted a natural/12 bound: %v", err)
	}
	if _, err := b13.Evaluate12(claim(ref13)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/12 claim accepted a natural/13 bound: %v", err)
	}
	if _, err := b12.EvaluateBirthRequest13(request(ref12)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/13 request accepted a natural/12 bound: %v", err)
	}
	if _, err := b13.EvaluateBirthReply12(naturalReplyView(1, 3, ref13, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/12 reply accepted a natural/13 bound: %v", err)
	}
	var nilBound *strategy.NaturalBound
	if _, err := nilBound.Evaluate13(strategy.NaturalObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/13 claim cascade")
	}
	if _, err := nilBound.EvaluateBirthRequest13(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/13 request cascade")
	}
	if _, err := nilBound.EvaluateBirthReply13(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/13 reply cascade")
	}
}
