package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"errors"
	"reflect"
	"testing"
)

// natural12Fixture binds one actor to the frozen natural/12 policy through the
// ref-set-aware registration path.
func natural12Fixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy12()
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

// natural12RequestView builds a natural/12 proposer view whose public kin walk
// annotates the three same-patch mates (2, 3, 4) with the caller's relations.
func natural12RequestView(t *testing.T, ref strategy.NaturalRef, hour int, views []strategy.NaturalMateView) strategy.NaturalRequestObservation {
	t.Helper()
	obs := naturalRequestView(t, 1, ref, hour)
	obs.PatchMates = []sim.EntityID{2, 3, 4}
	obs.MateViews = views
	return obs
}

// TestNatural12AddresseeRotatesOverNonKin proves the one behavioural
// difference of ticket 47: the rotation runs over the non-kin mates only, so a
// related mate the world would refuse is never asked.
func TestNatural12AddresseeRotatesOverNonKin(t *testing.T) {
	b12, ref12 := natural12Fixture(t, 1)
	b11, ref11 := natural11Fixture(t, 1)

	// Mate 2 is kin (public walk distance 1), 3 and 4 are unrelated.
	views := []strategy.NaturalMateView{
		{ID: 2, KinRelated: true, KinDistance: 1},
		{ID: 3, KinRelated: false, KinDistance: 0},
		{ID: 4, KinRelated: false, KinDistance: 0},
	}
	want12 := []sim.EntityID{3, 4, 3, 4} // nonKin[hour % 2]
	want11 := []sim.EntityID{2, 3, 4, 2} // mates[hour % 3]
	for hour := range want12 {
		choice, err := b12.EvaluateBirthRequest12(natural12RequestView(t, ref12, hour, views))
		if err != nil {
			t.Fatalf("natural/12 hour %d: %v", hour, err)
		}
		if choice.Kind != strategy.NaturalRequestFile || choice.Addressee != want12[hour] {
			t.Fatalf("natural/12 hour %d: kind %v addressee %d, want file to %d", hour, choice.Kind, choice.Addressee, want12[hour])
		}
		control, err := b11.EvaluateBirthRequest11(natural12RequestView(t, ref11, hour, views))
		if err != nil {
			t.Fatalf("natural/11 hour %d: %v", hour, err)
		}
		if control.Addressee != want11[hour] {
			t.Fatalf("natural/11 hour %d: addressee %d, want the kin-blind rotation %d", hour, control.Addressee, want11[hour])
		}
	}
}

// TestNatural12WaitsWhenNoNonKin proves the fail-closed alternative to filing:
// with every same-patch mate related, the policy waits instead of asking a mate
// the world must refuse; the empty roster waits too.
func TestNatural12WaitsWhenNoNonKin(t *testing.T) {
	b12, ref12 := natural12Fixture(t, 1)

	allKin := []strategy.NaturalMateView{
		{ID: 2, KinRelated: true, KinDistance: 1},
		{ID: 3, KinRelated: true, KinDistance: 2},
		{ID: 4, KinRelated: true, KinDistance: 10},
	}
	choice, err := b12.EvaluateBirthRequest12(natural12RequestView(t, ref12, 3, allKin))
	if err != nil {
		t.Fatalf("all-kin roster: %v", err)
	}
	if choice.Kind != strategy.NaturalRequestWait || choice.Addressee != 0 {
		t.Fatalf("all-kin roster: kind %v addressee %d, want wait", choice.Kind, choice.Addressee)
	}

	lone := naturalRequestView(t, 1, ref12, 3)
	lone.PatchMates, lone.MateViews = nil, nil
	choice, err = b12.EvaluateBirthRequest12(lone)
	if err != nil {
		t.Fatalf("empty roster: %v", err)
	}
	if choice.Kind != strategy.NaturalRequestWait {
		t.Fatalf("empty roster: kind %v, want wait", choice.Kind)
	}
}

// TestNatural12MateViewFailsClosed proves the observation contract: the kin
// view must match the roster entry for entry, so a proposer can never rotate
// over a walk that disagrees with the IDs it annotates.
func TestNatural12MateViewFailsClosed(t *testing.T) {
	b12, ref12 := natural12Fixture(t, 1)
	good := []strategy.NaturalMateView{
		{ID: 2, KinRelated: true, KinDistance: 2},
		{ID: 3, KinRelated: false, KinDistance: 0},
		{ID: 4, KinRelated: false, KinDistance: 0},
	}
	if _, err := b12.EvaluateBirthRequest12(natural12RequestView(t, ref12, 3, good)); err != nil {
		t.Fatalf("valid kin view refused: %v", err)
	}
	cases := map[string][]strategy.NaturalMateView{
		"short view":   {{ID: 2, KinRelated: true, KinDistance: 2}, {ID: 3}},
		"long view":    append(append([]strategy.NaturalMateView(nil), good...), strategy.NaturalMateView{ID: 5}),
		"wrong id":     {{ID: 2}, {ID: 4}, {ID: 3}},
		"kin at 0":     {{ID: 2}, {ID: 3}, {ID: 4, KinRelated: true}},
		"kin past 10":  {{ID: 2, KinRelated: true, KinDistance: 11}, {ID: 3}, {ID: 4}},
		"non-kin at 1": {{ID: 2}, {ID: 3, KinDistance: 1}, {ID: 4}},
	}
	for name, views := range cases {
		obs := natural12RequestView(t, ref12, 3, views)
		if _, err := b12.EvaluateBirthRequest12(obs); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("%s accepted: %v", name, err)
		}
		// The earlier frozen refs ignore the new field: the same observation
		// is still a valid natural/11 request.
		b11, ref11 := natural11Fixture(t, 1)
		if _, err := b11.EvaluateBirthRequest11(natural12RequestView(t, ref11, 3, views)); err != nil {
			t.Fatalf("%s: natural/11 refused the ignored field: %v", name, err)
		}
	}
}

// TestNatural12OtherChoicesMatchNatural11 proves the differential claim: with
// the ref field normalised, every natural/12 choice equals natural/11's for the
// same observation except the request addressee (and the wait the non-kin rule
// reaches when no non-kin mate exists).
func TestNatural12OtherChoicesMatchNatural11(t *testing.T) {
	b12, ref12 := natural12Fixture(t, 1)
	b11, ref11 := natural11Fixture(t, 1)

	// Claim + bag cascade: a compact grid must be identical.
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
						got12, err12 := b12.Evaluate12(obs(ref12))
						got11, err11 := b11.Evaluate11(obs(ref11))
						if !errors.Is(err12, err11) {
							t.Fatalf("claim grid E%d K%d W%d G%d B%d: errors %v vs %v", energy, capital, wip, granary, bag, err12, err11)
						}
						got12.Ref, got11.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
						if got12 != got11 {
							t.Fatalf("claim grid E%d K%d W%d G%d B%d: natural/12 %+v != natural/11 %+v", energy, capital, wip, granary, bag, got12, got11)
						}
					}
				}
			}
		}
	}

	// Birth request: every field except the addressee must be identical.
	views := []strategy.NaturalMateView{
		{ID: 2, KinRelated: true, KinDistance: 1},
		{ID: 3, KinRelated: false, KinDistance: 0},
		{ID: 4, KinRelated: false, KinDistance: 0},
	}
	for _, hour := range []int{0, 1, 2, 7} {
		for _, energy := range []int64{2, 4, 8} {
			obs := func(ref strategy.NaturalRef) strategy.NaturalRequestObservation {
				o := natural12RequestView(t, ref, hour, views)
				o.Energy = sim.IntegerValue(energy)
				return o
			}
			got12, err12 := b12.EvaluateBirthRequest12(obs(ref12))
			got11, err11 := b11.EvaluateBirthRequest11(obs(ref11))
			if !errors.Is(err12, err11) {
				t.Fatalf("request hour %d E%d: errors %v vs %v", hour, energy, err12, err11)
			}
			got12.Ref, got11.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
			got12.Addressee, got11.Addressee = 0, 0
			if got12 != got11 {
				t.Fatalf("request hour %d E%d: natural/12 %+v != natural/11 %+v", hour, energy, got12, got11)
			}
		}
	}

	// Birth reply: the whole choice must be identical.
	for _, hour := range []int{0, 3} {
		obs := func(ref strategy.NaturalRef) strategy.NaturalReplyObservation {
			return naturalReplyView(1, 3, ref, hour)
		}
		got12, err12 := b12.EvaluateBirthReply12(obs(ref12))
		got11, err11 := b11.EvaluateBirthReply11(obs(ref11))
		if !errors.Is(err12, err11) {
			t.Fatalf("reply hour %d: errors %v vs %v", hour, err12, err11)
		}
		got12.Ref, got11.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
		if !reflect.DeepEqual(got12, got11) {
			t.Fatalf("reply hour %d: natural/12 %+v != natural/11 %+v", hour, got12, got11)
		}
	}
}

// TestNatural12BoundIdentityFailsClosed proves the frozen 12 entry points take
// only a natural/12 bound, and that the natural/11 cascades refuse a natural/12
// bound.
func TestNatural12BoundIdentityFailsClosed(t *testing.T) {
	b12, ref12 := natural12Fixture(t, 1)
	b11, ref11 := natural11Fixture(t, 1)

	claim := func(ref strategy.NaturalRef) strategy.NaturalObservation {
		o := naturalObservation(t, 1, ref, 5)
		o.Energy = sim.IntegerValue(4)
		o.GranaryStock = sim.IntegerValue(1)
		return o
	}
	views := []strategy.NaturalMateView{{ID: 2, KinRelated: false}, {ID: 3, KinRelated: false}, {ID: 4, KinRelated: false}}
	if _, err := b11.Evaluate12(claim(ref11)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/12 claim accepted a natural/11 bound: %v", err)
	}
	if _, err := b11.EvaluateBirthRequest12(natural12RequestView(t, ref11, 0, views)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/12 request accepted a natural/11 bound: %v", err)
	}
	if _, err := b11.EvaluateBirthReply12(naturalReplyView(1, 3, ref11, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/12 reply accepted a natural/11 bound: %v", err)
	}
	if _, err := b12.Evaluate11(claim(ref12)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/11 claim accepted a natural/12 bound: %v", err)
	}
	if _, err := b12.EvaluateBirthRequest11(natural12RequestView(t, ref12, 0, views)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("natural/11 request accepted a natural/12 bound: %v", err)
	}
	var nilBound *strategy.NaturalBound
	if _, err := nilBound.Evaluate12(strategy.NaturalObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/12 claim cascade")
	}
	if _, err := nilBound.EvaluateBirthRequest12(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/12 request cascade")
	}
	if _, err := nilBound.EvaluateBirthReply12(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the natural/12 reply cascade")
	}
}
