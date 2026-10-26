package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"errors"
	"reflect"
	"testing"
)

// natural11Fixture binds one actor to the frozen natural/11 policy through the
// ref-set-aware registration path.
func natural11Fixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy11()
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

// TestNatural11AddresseeRotatesWithHour proves the one behavioural difference:
// on a same-patch roster of three actors (two mates) the addressee alternates
// with the hour over the ascending roster, while natural/7 keeps filing to the
// same successor.
func TestNatural11AddresseeRotatesWithHour(t *testing.T) {
	b11, ref11 := natural11Fixture(t, 1)
	b7, ref7 := natural7Fixture(t, 1)

	view := func(ref strategy.NaturalRef, hour int) strategy.NaturalRequestObservation {
		obs := naturalRequestView(t, 1, ref, hour)
		obs.PatchMates = []sim.EntityID{2, 3} // ascending, same patch as founder 1
		return obs
	}

	want11 := []sim.EntityID{2, 3, 2, 3}
	for hour, want := range want11 {
		choice, err := b11.EvaluateBirthRequest11(view(ref11, hour))
		if err != nil {
			t.Fatalf("hour %d: %v", hour, err)
		}
		if choice.Kind != strategy.NaturalRequestFile || choice.Addressee != want {
			t.Fatalf("natural/11 hour %d: kind %v addressee %d, want file to %d", hour, choice.Kind, choice.Addressee, want)
		}
		control, err := b7.EvaluateBirthRequest7(view(ref7, hour))
		if err != nil {
			t.Fatalf("natural/7 hour %d: %v", hour, err)
		}
		if control.Addressee != 2 {
			t.Fatalf("natural/7 hour %d: addressee %d, want the fixed successor 2", hour, control.Addressee)
		}
	}
}

// TestNatural11OtherChoicesMatchNatural7 proves the differential claim: with
// the ref field normalised, every natural/11 choice equals natural/7's for the
// same observation, across the claim/bag grid and the birth request/reply
// cascades (only the request addressee is allowed to differ).
func TestNatural11OtherChoicesMatchNatural7(t *testing.T) {
	b11, ref11 := natural11Fixture(t, 1)
	b7, ref7 := natural7Fixture(t, 1)

	// Claim + bag cascade: every cell of a compact grid must be identical.
	for _, energy := range []int64{0, 2, 3, 4, 5, 8} {
		for _, capital := range []int64{0, 1, 8} {
			for _, wip := range []int64{0, 1} {
				for _, granary := range []int64{0, 1, 3} {
					for _, bag := range []int64{0, 1} {
						for _, denial := range []strategy.NaturalDenial{strategy.NaturalNoDenial, strategy.NaturalGatherDenied, strategy.NaturalBuildDenied} {
							for _, stocked := range []bool{false, true} {
								obs := func(ref strategy.NaturalRef) strategy.NaturalObservation {
									o := naturalObservation(t, 1, ref, 5)
									o.Energy = sim.IntegerValue(energy)
									o.Capital = sim.IntegerValue(capital)
									o.Wip = sim.IntegerValue(wip)
									o.GranaryStock = sim.IntegerValue(granary)
									o.BagUnits = sim.IntegerValue(bag)
									o.LastDenial = denial
									if stocked {
										o = naturalStocked(o, 0)
									}
									return o
								}
								got11, err11 := b11.Evaluate11(obs(ref11))
								got7, err7 := b7.Evaluate7(obs(ref7))
								if !errors.Is(err11, err7) {
									t.Fatalf("claim grid E%d K%d W%d G%d B%d D%d S%v: errors %v vs %v", energy, capital, wip, granary, bag, denial, stocked, err11, err7)
								}
								got11.Ref, got7.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
								if got11 != got7 {
									t.Fatalf("claim grid E%d K%d W%d G%d B%d D%d S%v: natural/11 %+v != natural/7 %+v", energy, capital, wip, granary, bag, denial, stocked, got11, got7)
								}
							}
						}
					}
				}
			}
		}
	}

	// Birth request: every field except the addressee must be identical.
	for _, hour := range []int{0, 1, 2, 3, 7} {
		for _, energy := range []int64{2, 4, 8} {
			obs := func(ref strategy.NaturalRef) strategy.NaturalRequestObservation {
				o := naturalRequestView(t, 1, ref, hour)
				o.PatchMates = []sim.EntityID{2, 3, 4}
				o.Energy = sim.IntegerValue(energy)
				return o
			}
			got11, err11 := b11.EvaluateBirthRequest11(obs(ref11))
			got7, err7 := b7.EvaluateBirthRequest7(obs(ref7))
			if !errors.Is(err11, err7) {
				t.Fatalf("request hour %d E%d: errors %v vs %v", hour, energy, err11, err7)
			}
			if got11.Kind == strategy.NaturalRequestFile && got11.Addressee != []sim.EntityID{2, 3, 4}[hour%3] {
				t.Fatalf("request hour %d: natural/11 addressee %d, want %d", hour, got11.Addressee, []sim.EntityID{2, 3, 4}[hour%3])
			}
			got11.Ref, got7.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
			got11.Addressee, got7.Addressee = 0, 0
			if got11 != got7 {
				t.Fatalf("request hour %d E%d: natural/11 %+v != natural/7 %+v", hour, energy, got11, got7)
			}
		}
	}

	// Birth reply: the whole choice must be identical.
	for _, hour := range []int{0, 3} {
		obs := func(ref strategy.NaturalRef) strategy.NaturalReplyObservation {
			return naturalReplyView(1, 3, ref, hour)
		}
		got11, err11 := b11.EvaluateBirthReply11(obs(ref11))
		got7, err7 := b7.EvaluateBirthReply7(obs(ref7))
		if !errors.Is(err11, err7) {
			t.Fatalf("reply hour %d: errors %v vs %v", hour, err11, err7)
		}
		got11.Ref, got7.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
		if !reflect.DeepEqual(got11, got7) {
			t.Fatalf("reply hour %d: natural/11 %+v != natural/7 %+v", hour, got11, got7)
		}
	}
}

// TestNatural11BoundIdentityFailsClosed proves the frozen 11 entry points take
// only a natural/11 bound, and that the other cascades refuse a natural/11
// bound.
func TestNatural11BoundIdentityFailsClosed(t *testing.T) {
	b11, ref11 := natural11Fixture(t, 1)
	b7, ref7 := natural7Fixture(t, 1)
	b5, ref5 := naturalFixture(t, 1)

	claim := func(ref strategy.NaturalRef) strategy.NaturalObservation {
		o := naturalObservation(t, 1, ref, 5)
		o.Energy = sim.IntegerValue(4)
		o.GranaryStock = sim.IntegerValue(1)
		return o
	}
	if _, err := b7.Evaluate11(claim(ref7)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/11 claim accepted a natural/7 bound: %v", err)
	}
	if _, err := b5.Evaluate11(claim(ref5)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/11 claim accepted a natural/5 bound: %v", err)
	}
	if _, err := b7.EvaluateBirthRequest11(naturalRequestView(t, 1, ref7, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/11 request accepted a natural/7 bound: %v", err)
	}
	if _, err := b7.EvaluateBirthReply11(naturalReplyView(1, 3, ref7, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/11 reply accepted a natural/7 bound: %v", err)
	}
	if _, err := b11.Evaluate7(claim(ref11)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/7 claim accepted a natural/11 bound: %v", err)
	}
	if _, err := b11.EvaluateBirthRequest7(naturalRequestView(t, 1, ref11, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/7 request accepted a natural/11 bound: %v", err)
	}
	if _, err := b11.EvaluateBirthReply7(naturalReplyView(1, 3, ref11, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/7 reply accepted a natural/11 bound: %v", err)
	}
	var nilBound *strategy.NaturalBound
	if _, err := nilBound.Evaluate11(strategy.NaturalObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/11 claim cascade")
	}
	if _, err := nilBound.EvaluateBirthRequest11(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/11 request cascade")
	}
	if _, err := nilBound.EvaluateBirthReply11(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/11 reply cascade")
	}
}
