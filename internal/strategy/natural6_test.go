package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"bytes"
	"crypto/sha256"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// natural6Fixture binds one actor to the frozen v6 policy through the
// v6-aware registration path (RegisterFrozenNatural, not the frozen v5-only
// Register, which must keep rejecting the v6 ref).
func natural6Fixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicyV6()
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	if r == nil {
		t.Fatal("nil registry for a valid patch count")
	}
	if err := r.RegisterFrozenNatural(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.NaturalBinding{Actor: actor, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b, p.Ref
}

// naturalV5V6Bounds binds both frozen refs into one registry for the
// differential grid.
func naturalV5V6Bounds(t *testing.T) (b5, b6 *strategy.NaturalBound, ref5, ref6 strategy.NaturalRef) {
	t.Helper()
	p5, p6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := r.RegisterFrozenNatural(p5); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterFrozenNatural(p6); err != nil {
		t.Fatal(err)
	}
	b5, err := r.Bind(strategy.NaturalBinding{Actor: 1, Ref: p5.Ref})
	if err != nil {
		t.Fatal(err)
	}
	b6, err = r.Bind(strategy.NaturalBinding{Actor: 1, Ref: p6.Ref})
	if err != nil {
		t.Fatal(err)
	}
	return b5, b6, p5.Ref, p6.Ref
}

// naturalChoiceShape strips the per-policy ref so the v5 and v6 choices of
// one cell compare on decision content alone.
func naturalChoiceShape(c strategy.NaturalChoice) strategy.NaturalChoice {
	c.Ref = strategy.NaturalRef{}
	return c
}

// TestNaturalV6FrozenIdentityRefSetAndRegistration pins the frozen v6
// identity, the ref-set resolver and predicate, the v6-aware registration
// path, and the untouched v5-only validation.
func TestNaturalV6FrozenIdentityRefSetAndRegistration(t *testing.T) {
	p6 := strategy.FrozenNaturalPolicyV6()
	if p6 != (strategy.NaturalPolicy{FormatVersion: 6, Ref: strategy.NaturalRef{ID: "natural", Version: 6},
		Budget: strategy.NaturalBudget{Candidates: 32, Evaluations: 32}}) {
		t.Fatalf("frozen v6 policy drifted: %+v", p6)
	}
	p5 := strategy.FrozenNaturalPolicy()
	if got, err := strategy.FrozenNaturalPolicyForRef(p5.Ref); err != nil || got != p5 {
		t.Fatalf("natural/5 resolution: %+v %v", got, err)
	}
	if got, err := strategy.FrozenNaturalPolicyForRef(p6.Ref); err != nil || got != p6 {
		t.Fatalf("natural/6 resolution: %+v %v", got, err)
	}
	for _, unknown := range []strategy.NaturalRef{
		// Ticket 20 consequence (authorised sample move): natural/9 joined the frozen ref set; the unknown sample moves to 10.
		{ID: "natural", Version: 10}, {ID: "natural", Version: 0}, {ID: "other", Version: 5}, {ID: "natural-v6", Version: 6}, {},
	} {
		if got, err := strategy.FrozenNaturalPolicyForRef(unknown); !errors.Is(err, strategy.ErrUnknownNaturalPolicy) {
			t.Fatalf("unknown ref %v resolved to %+v (%v)", unknown, got, err)
		}
	}
	if !strategy.ValidFrozenNaturalPolicy(p5) || !strategy.ValidFrozenNaturalPolicy(p6) {
		t.Fatal("frozen ref-set members must validate")
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for _, invalid := range []strategy.NaturalPolicy{
		func() strategy.NaturalPolicy { c := p6; c.FormatVersion = 5; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.FormatVersion = 7; return c }(),
		func() strategy.NaturalPolicy {
			c := p6
			c.Ref = strategy.NaturalRef{ID: "natural", Version: 5}
			return c
		}(),
		func() strategy.NaturalPolicy { c := p6; c.Ref.ID = ""; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Ref.ID = longID; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Ref.Version = 0; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Ref.Version = 7; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Budget.Candidates = 0; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Budget.Candidates = 33; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Budget.Evaluations = 0; return c }(),
		func() strategy.NaturalPolicy { c := p6; c.Budget.Evaluations = 33; return c }(),
	} {
		if strategy.ValidFrozenNaturalPolicy(invalid) {
			t.Fatalf("invalid v6 policy passed the ref-set predicate: %+v", invalid)
		}
	}
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	// The frozen v5 Register keeps its v5-only validation: it accepts v5 and
	// rejects v6, untouched.
	if err := r.Register(p5); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(p6); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("frozen v5 Register accepted the v6 ref: %v", err)
	}
	// The v6-aware path registers the v6 ref (and same-ref budget variants)
	// with the same shape semantics per ref.
	variant := p6
	variant.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
	for name, tc := range map[string]struct {
		policy strategy.NaturalPolicy
		ok     bool
	}{
		"frozen v6":               {p6, true},
		"same-ref budget variant": {variant, true},
		"frozen v5":               {p5, true},
		"format 5 with ref 6":     {func() strategy.NaturalPolicy { c := p6; c.FormatVersion = 5; return c }(), false},
		"format 6 with ref 5":     {func() strategy.NaturalPolicy { c := p5; c.FormatVersion = 6; return c }(), false},
		"budget 0":                {func() strategy.NaturalPolicy { c := p6; c.Budget.Candidates = 0; return c }(), false},
		"budget 33":               {func() strategy.NaturalPolicy { c := p6; c.Budget.Evaluations = 33; return c }(), false},
		"unfrozen ref":            {func() strategy.NaturalPolicy { c := p6; c.Ref.Version = 7; return c }(), false},
	} {
		reg := strategy.NewNaturalRegistry(naturalTestPatches)
		err := reg.RegisterFrozenNatural(tc.policy)
		if tc.ok && err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !tc.ok && !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	// Duplicate refs are rejected on the v6-aware path too.
	if err := r.RegisterFrozenNatural(p6); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterFrozenNatural(p6); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("duplicate v6 reference accepted: %v", err)
	}
	// Both frozen refs coexist in one registry and bind founders and
	// newborns by their own ref.
	b6, ref6 := natural6Fixture(t, 2017) // newborn binding through the v6 path
	if b6.Binding() != (strategy.NaturalBinding{Actor: 2017, Ref: ref6}) || b6.Policy() != p6 {
		t.Fatalf("v6 newborn binding: %+v %+v", b6.Binding(), b6.Policy())
	}
	var nilRegistry *strategy.NaturalRegistry
	if err := nilRegistry.RegisterFrozenNatural(p6); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil registry registered the v6 policy")
	}
	// The v6 cascade fails closed under any other bound identity.
	b5, ref5 := naturalFixture(t, 1)
	wrongRef := strategy.NaturalObservation{Actor: 1, Ref: ref5, Hour: 5, WorldVersion: 9,
		Energy: sim.IntegerValue(5), BagUnits: sim.IntegerValue(1)}
	if _, err := b5.EvaluateV6(wrongRef); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("v6 cascade accepted a v5 bound: %v", err)
	}
	var nilBound *strategy.NaturalBound
	if _, err := nilBound.EvaluateV6(wrongRef); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the v6 cascade")
	}
	if _, err := nilBound.EvaluateBirthRequestV6(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the v6 request cascade")
	}
	if _, err := nilBound.EvaluateBirthReplyV6(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the v6 reply cascade")
	}
	_ = ref5
}

// naturalV5ExpectedBag restates the frozen v5 bag decision for the grid.
func naturalV5ExpectedBag(energy, capital, wip, granary int64, denial strategy.NaturalDenial) strategy.NaturalAction {
	if denial == strategy.NaturalBuildDenied {
		return strategy.NaturalEat
	}
	if capital+wip < 8 && (energy >= 5 || granary >= 1) {
		return strategy.NaturalBuild
	}
	return strategy.NaturalEat
}

// naturalV6ExpectedBag restates the frozen v6 bag decision for the grid.
func naturalV6ExpectedBag(energy, capital, wip, granary int64, denial strategy.NaturalDenial) strategy.NaturalAction {
	if denial == strategy.NaturalBuildDenied {
		return strategy.NaturalEat
	}
	if capital+wip < 8 && energy >= 3 && (granary >= 1 || wip >= 1 || energy >= 4) {
		return strategy.NaturalBuild
	}
	return strategy.NaturalEat
}

// TestNaturalV5V6DifferentialGrid walks the full shared-observation grid —
// energy 0..12 × capital 0..8 × wip {0,1} × granary {0,1,3,8} × bag {0,1} ×
// denial {none, gather, build} × one stocked home slot {no, yes} — and
// asserts, per cell, the exact frozen-design delta set: (a) E in {1,2} with
// granary >= 1 flips Build -> Eat; (b) E = 3, granary = 0, wip >= 1 flips
// Eat -> Build; (c) E = 4, granary = 0, wip = 0 flips Eat -> Build; and
// (d) E = 4, granary = 0, wip >= 1 flips Eat -> Build — the rule-implied
// family the frozen design's delta enumeration missed (its TFinish = 3
// reading fires the wip leg at every E >= 3; recorded supervisor ruling,
// ticket 16). Every other cell must be identical, so any additional flip
// family fails the test here rather than surviving into a run.
func TestNaturalV5V6DifferentialGrid(t *testing.T) {
	b5, b6, ref5, ref6 := naturalV5V6Bounds(t)
	const hour = 5
	base5 := naturalObservation(t, 1, ref5, hour)
	base6 := naturalObservation(t, 1, ref6, hour)

	type margins struct {
		energy, capital, wip, granary, bag int64
		denial                             strategy.NaturalDenial
		stocked                            bool
	}
	cell := func(m margins) (o5, o6 strategy.NaturalObservation) {
		o5, o6 = base5, base6
		for _, o := range []*strategy.NaturalObservation{&o5, &o6} {
			o.HomeSlots = append([]strategy.NaturalSlotView(nil), o.HomeSlots...)
			o.Energy = sim.IntegerValue(m.energy)
			o.Capital = sim.IntegerValue(m.capital)
			o.Wip = sim.IntegerValue(m.wip)
			o.GranaryStock = sim.IntegerValue(m.granary)
			o.BagUnits = sim.IntegerValue(m.bag)
			o.LastDenial = m.denial
			if m.stocked {
				o.HomeSlots[0].Stock = sim.IntegerValue(1)
			}
		}
		return o5, o6
	}

	var cells, structuralErrors, bagCells, claimCells int
	flips := map[string]int{}
	for energy := int64(0); energy <= 12; energy++ {
		for capital := int64(0); capital <= 8; capital++ {
			for wip := int64(0); wip <= 1; wip++ {
				for _, granary := range []int64{0, 1, 3, 8} {
					for bag := int64(0); bag <= 1; bag++ {
						for _, denial := range []strategy.NaturalDenial{strategy.NaturalNoDenial, strategy.NaturalGatherDenied, strategy.NaturalBuildDenied} {
							for _, stocked := range []bool{false, true} {
								m := margins{energy, capital, wip, granary, bag, denial, stocked}
								o5, o6 := cell(m)
								c5, err5 := b5.Evaluate(o5)
								c6, err6 := b6.EvaluateV6(o6)
								cells++
								// Structural acceptance is identical: a build
								// denial with an empty bag is malformed for
								// both, everything else evaluates.
								if (err5 == nil) != (err6 == nil) {
									t.Fatalf("cell %+v: structural acceptance diverged: %v vs %v", m, err5, err6)
								}
								if err5 != nil {
									if !errors.Is(err5, strategy.ErrInvalidNaturalObservation) || !errors.Is(err6, strategy.ErrInvalidNaturalObservation) {
										t.Fatalf("cell %+v: unexpected error class: %v %v", m, err5, err6)
									}
									structuralErrors++
									continue
								}
								// Expected choices restated from the frozen
								// cascades for the grid's fixed shape (hour 5,
								// no stored meal yet, the actor's own eight
								// home slots, at most slot 0 stocked).
								var want5Kind, want6Kind strategy.NaturalAction
								want := strategy.NaturalChoice{ObservedVersion: 9, Fallback: strategy.NaturalNoFallback}
								if bag == 1 {
									bagCells++
									if energy == 0 {
										want5Kind, want6Kind = strategy.NaturalWait, strategy.NaturalWait
										want.Fallback = strategy.NaturalNoCandidate
									} else {
										want5Kind = naturalV5ExpectedBag(energy, capital, wip, granary, denial)
										want6Kind = naturalV6ExpectedBag(energy, capital, wip, granary, denial)
										want.Evaluated = 1
									}
								} else {
									claimCells++
									// Below the bag decision the cascade is
									// verbatim v5: identical for both policies.
									if energy == 0 {
										want5Kind, want6Kind = strategy.NaturalWait, strategy.NaturalWait
										want.Fallback = strategy.NaturalNoCandidate
									} else if denial == strategy.NaturalGatherDenied {
										if granary >= 1 {
											want5Kind, want6Kind = strategy.NaturalEatStored, strategy.NaturalEatStored
										} else {
											want5Kind, want6Kind = strategy.NaturalRest, strategy.NaturalRest
											want.RestDuration = strategy.NaturalRestDuration
										}
										want.Evaluated = 1
									} else if stocked {
										want5Kind, want6Kind = strategy.NaturalGather, strategy.NaturalGather
										want.TargetSlot, want.Evaluated = 2001, 1
									} else if granary >= 1 {
										want5Kind, want6Kind = strategy.NaturalEatStored, strategy.NaturalEatStored
										want.Evaluated = 1
									} else {
										want5Kind, want6Kind = strategy.NaturalWait, strategy.NaturalWait
										want.Fallback = strategy.NaturalNoCandidate
									}
								}
								for _, tc := range []struct {
									b    *strategy.NaturalBound
									ref  strategy.NaturalRef
									got  strategy.NaturalChoice
									want strategy.NaturalAction
								}{{b5, ref5, c5, want5Kind}, {b6, ref6, c6, want6Kind}} {
									expected := want
									expected.Kind, expected.Ref = tc.want, tc.ref
									if tc.got != expected {
										t.Fatalf("cell %+v: frozen cascade drift: got %+v, want %+v", m, tc.got, expected)
									}
								}
								// The delta classification: flips only at the
								// bag decision, never under the build-denial
								// fallback (both policies eat there).
								bagDecides := bag == 1 && energy > 0 && denial != strategy.NaturalBuildDenied
								// (a) E in {1,2} and granary >= 1: Build -> Eat.
								flipA := bagDecides && energy <= 2 && granary >= 1 && capital+wip < 8
								// (b) E = 3, granary = 0, wip >= 1: Eat -> Build.
								flipB := bagDecides && energy == 3 && granary == 0 && wip >= 1 && capital+wip < 8
								// (c) E = 4, granary = 0, wip = 0: Eat -> Build.
								flipC := bagDecides && energy == 4 && granary == 0 && wip == 0 && capital+wip < 8
								// (d) E = 4, granary = 0, wip >= 1: Eat ->
								// Build — the rule-implied family the frozen
								// delta enumeration missed; recorded ruling.
								flipD := bagDecides && energy == 4 && granary == 0 && wip >= 1 && capital+wip < 8
								families := 0
								name := ""
								for _, f := range []struct {
									on bool
									id string
								}{{flipA, "a"}, {flipB, "b"}, {flipC, "c"}, {flipD, "d"}} {
									if f.on {
										families++
										name = f.id
									}
								}
								if families > 1 {
									t.Fatalf("cell %+v matches several delta families", m)
								}
								same := naturalChoiceShape(c5) == naturalChoiceShape(c6)
								if families == 1 {
									if same {
										t.Fatalf("cell %+v must flip (family %s): v5=%v v6=%v", m, name, c5.Kind, c6.Kind)
									}
									if flipA && (c5.Kind != strategy.NaturalBuild || c6.Kind != strategy.NaturalEat) {
										t.Fatalf("cell %+v: family (a) must flip Build->Eat: v5=%v v6=%v", m, c5.Kind, c6.Kind)
									}
									if (flipB || flipC || flipD) && (c5.Kind != strategy.NaturalEat || c6.Kind != strategy.NaturalBuild) {
										t.Fatalf("cell %+v: family must flip Eat->Build: v5=%v v6=%v", m, c5.Kind, c6.Kind)
									}
									flips[name]++
								} else if !same {
									t.Fatalf("cell %+v diverges outside the frozen delta set: v5=%+v v6=%+v", m, naturalChoiceShape(c5), naturalChoiceShape(c6))
								}
							}
						}
					}
				}
			}
		}
	}
	// Grid shape and exact delta counts (13 energies × 9 capitals × 2 wip ×
	// 4 granaries × 2 bags × 3 denials × 2 stocked): flips need the bag
	// decision, a living actor, and no build denial, with k+wip < 8 —
	// (capital,wip) unsaturated pairs: 8 with wip=0, 7 with wip=1. The bag
	// decision ignores slot stocks, so the stocked dimension doubles every
	// family.
	//
	//   (a) energies {1,2} (2) × granary {1,3,8} (3) × 15 pairs × 2 denials × 2 stocked = 360
	//   (b) E=3, granary=0, wip=1: capitals 0..6 (7) × 2 denials × 2 stocked            =  28
	//   (c) E=4, granary=0, wip=0: capitals 0..7 (8) × 2 denials × 2 stocked            =  32
	//   (d) E=4, granary=0, wip=1: capitals 0..6 (7) × 2 denials × 2 stocked            =  28
	if cells != 11232 || structuralErrors != 13*9*2*4*2 || bagCells != 13*9*2*4*3*2 || claimCells != 3744 {
		t.Fatalf("grid shape drifted: cells=%d structural=%d bag=%d claim=%d", cells, structuralErrors, bagCells, claimCells)
	}
	want := map[string]int{"a": 360, "b": 28, "c": 32, "d": 28}
	for _, family := range []string{"a", "b", "c", "d"} {
		if flips[family] == 0 {
			t.Fatalf("delta family (%s) never fired: the grid is vacuous", family)
		}
		if flips[family] != want[family] {
			t.Fatalf("delta family (%s) fired %d times, want exactly %d", family, flips[family], want[family])
		}
	}
	total := flips["a"] + flips["b"] + flips["c"] + flips["d"]
	if total != 448 {
		t.Fatalf("total flips %d, want exactly 448 (360+28+32+28)", total)
	}
	t.Logf("differential grid: %d cells, %d structural rejections, %d bag cells, %d claim cells, "+
		"flips a=%d b=%d c=%d d=%d (total %d); all other cells identical",
		cells, structuralErrors, bagCells, claimCells, flips["a"], flips["b"], flips["c"], flips["d"], total)

	// Descent-family boundary pin: at E >= 5 with outstanding wip and an
	// empty granary both policies build — no flip between the (b)/(c)/(d)
	// families and the unchanged descent clause — and the fed band, the
	// saturation ceiling, and the sub-floor region are shared verbatim.
	for _, tc := range []struct {
		m    margins
		want strategy.NaturalAction
	}{
		{margins{energy: 5, capital: 0, wip: 1, granary: 0, bag: 1}, strategy.NaturalBuild},
		{margins{energy: 6, capital: 3, wip: 1, granary: 0, bag: 1}, strategy.NaturalBuild},
		{margins{energy: 12, capital: 7, wip: 0, granary: 0, bag: 1}, strategy.NaturalBuild},
		{margins{energy: 12, capital: 0, wip: 1, granary: 0, bag: 1}, strategy.NaturalBuild},
		{margins{energy: 3, capital: 0, wip: 0, granary: 1, bag: 1}, strategy.NaturalBuild},
		{margins{energy: 4, capital: 0, wip: 0, granary: 1, bag: 1}, strategy.NaturalBuild},
		{margins{energy: 12, capital: 7, wip: 1, granary: 8, bag: 1}, strategy.NaturalEat}, // saturated
		{margins{energy: 12, capital: 8, wip: 0, granary: 8, bag: 1}, strategy.NaturalEat}, // saturated
		{margins{energy: 2, capital: 0, wip: 0, granary: 0, bag: 1}, strategy.NaturalEat},  // below both floors
		{margins{energy: 3, capital: 0, wip: 0, granary: 0, bag: 1}, strategy.NaturalEat},  // the exact TStart miss
	} {
		o5, o6 := cell(tc.m)
		c5, err5 := b5.Evaluate(o5)
		c6, err6 := b6.EvaluateV6(o6)
		if err5 != nil || err6 != nil || c5.Kind != tc.want || c6.Kind != tc.want ||
			naturalChoiceShape(c5) != naturalChoiceShape(c6) {
			t.Fatalf("boundary cell %+v: v5=%v/%v v6=%v/%v, want both %v", tc.m, c5.Kind, err5, c6.Kind, err6, tc.want)
		}
	}
}

// TestNaturalV6BirthMarginTruthTable pins the second v6 change: filing and
// accepting require own energy >= 4, so E = 3 files/accepts in v5 and is
// refused in v6, and E >= 4 is identical; every other margin, the reported-
// claims margins, the kin clause, and the addressee rotation stay verbatim.
func TestNaturalV6BirthMarginTruthTable(t *testing.T) {
	b5, b6, ref5, ref6 := naturalV5V6Bounds(t)
	// Filing at the frozen capital/granary margins (2/3) across own energy.
	for _, tc := range []struct {
		energy       int64
		want5, want6 strategy.NaturalRequestAction
	}{
		{2, strategy.NaturalRequestWait, strategy.NaturalRequestWait},
		{3, strategy.NaturalRequestFile, strategy.NaturalRequestWait}, // the pinned v6 margin raise
		{4, strategy.NaturalRequestFile, strategy.NaturalRequestFile},
		{8, strategy.NaturalRequestFile, strategy.NaturalRequestFile},
	} {
		view5 := naturalRequestView(t, 1, ref5, 5)
		view5.Energy = sim.IntegerValue(tc.energy)
		view6 := naturalRequestView(t, 1, ref6, 5)
		view6.Energy = sim.IntegerValue(tc.energy)
		c5, err5 := b5.EvaluateBirthRequest(view5)
		c6, err6 := b6.EvaluateBirthRequestV6(view6)
		if err5 != nil || err6 != nil || c5.Kind != tc.want5 || c6.Kind != tc.want6 {
			t.Fatalf("filing at E=%d: v5=%v/%v v6=%v/%v", tc.energy, c5.Kind, err5, c6.Kind, err6)
		}
		if tc.want6 == strategy.NaturalRequestFile && (c6.Addressee != 2 || c6.ReportedCapital != 2 ||
			c6.ReportedGranary != 3 || c6.Evaluated != 7 || c6.Fallback != strategy.NaturalNoFallback) {
			t.Fatalf("v6 filing at E=%d drifted: %+v", tc.energy, c6)
		}
		if tc.want5 == strategy.NaturalRequestWait && c5.Fallback != strategy.NaturalNoCandidate {
			t.Fatalf("v5 condition miss at E=%d must be a plain wait: %+v", tc.energy, c5)
		}
		if tc.want6 == strategy.NaturalRequestWait && c6.Fallback != strategy.NaturalNoCandidate {
			t.Fatalf("v6 condition miss at E=%d must be a plain wait: %+v", tc.energy, c6)
		}
	}
	// Only the energy margin moved: at E = 3 with capital and granary far
	// above the margins v6 still waits while v5 files; at E >= 4 the v6
	// capital/granary boundaries sit exactly where v5's do.
	highMargins := naturalRequestView(t, 1, ref6, 5)
	highMargins.Energy = sim.IntegerValue(3)
	highMargins.Capital = sim.IntegerValue(8)
	highMargins.GranaryStock = sim.IntegerValue(8)
	high5 := highMargins
	high5.Ref = ref5
	if c5, err := b5.EvaluateBirthRequest(high5); err != nil || c5.Kind != strategy.NaturalRequestFile {
		t.Fatalf("v5 must file at E=3 with capital/granary 8: %+v %v", c5, err)
	}
	if c6, err := b6.EvaluateBirthRequestV6(highMargins); err != nil || c6.Kind != strategy.NaturalRequestWait {
		t.Fatalf("v6 must wait at E=3 regardless of the other margins: %+v %v", c6, err)
	}
	for _, tc := range []struct {
		name             string
		capital, granary int64
		want             strategy.NaturalRequestAction
	}{
		{"capital one below waits", 1, 3, strategy.NaturalRequestWait},
		{"capital at the margin files", 2, 3, strategy.NaturalRequestFile},
		{"granary one below waits", 2, 2, strategy.NaturalRequestWait},
		{"granary at the margin files", 2, 3, strategy.NaturalRequestFile},
	} {
		view := naturalRequestView(t, 1, ref6, 5)
		view.Energy = sim.IntegerValue(4)
		view.Capital = sim.IntegerValue(tc.capital)
		view.GranaryStock = sim.IntegerValue(tc.granary)
		if c, err := b6.EvaluateBirthRequestV6(view); err != nil || c.Kind != tc.want {
			t.Fatalf("v6 %s: %+v %v", tc.name, c, err)
		}
	}
	// Wraparound addressee rotation is verbatim: actor 8 wraps to 1.
	b8v5, ref8v5 := naturalFixture(t, 8)
	b8v6, ref8v6 := natural6Fixture(t, 8)
	for _, tc := range []struct {
		b   *strategy.NaturalBound
		ref strategy.NaturalRef
		v6  bool
	}{{b8v5, ref8v5, false}, {b8v6, ref8v6, true}} {
		view := naturalRequestView(t, 8, tc.ref, 5)
		var c strategy.NaturalRequestChoice
		var err error
		if tc.v6 {
			c, err = tc.b.EvaluateBirthRequestV6(view)
		} else {
			c, err = tc.b.EvaluateBirthRequest(view)
		}
		if err != nil || c.Kind != strategy.NaturalRequestFile || c.Addressee != 1 {
			t.Fatalf("wraparound addressee (v6=%v): %+v %v", tc.v6, c, err)
		}
	}
	// The reply sections act as consenter 2 with requester 1.
	b2v5, _ := naturalFixture(t, 2)
	b2v6, _ := natural6Fixture(t, 2)
	// Accept across own energy: E = 3 accepts in v5 and refuses with
	// ConsentOwn in v6; E >= 4 is identical; E = 2 refuses in both.
	for _, tc := range []struct {
		energy       int64
		want5, want6 strategy.NaturalReplyAction
		reason6      strategy.NaturalReplyReason
	}{
		{2, strategy.NaturalReplyRefuse, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn},
		{3, strategy.NaturalReplyAccept, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn},
		{4, strategy.NaturalReplyAccept, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
		{8, strategy.NaturalReplyAccept, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
	} {
		view5 := naturalReplyView(2, 1, ref5, 5)
		view5.Energy = sim.IntegerValue(tc.energy)
		view6 := naturalReplyView(2, 1, ref6, 5)
		view6.Energy = sim.IntegerValue(tc.energy)
		c5, err5 := b2v5.EvaluateBirthReply(view5)
		c6, err6 := b2v6.EvaluateBirthReplyV6(view6)
		if err5 != nil || err6 != nil || c5.Kind != tc.want5 || c6.Kind != tc.want6 ||
			(tc.want6 == strategy.NaturalReplyRefuse && c6.Reason != tc.reason6) {
			t.Fatalf("accept at own E=%d: v5=%v/%v v6=%v/%v reason %v (errs %v %v)", tc.energy, c5.Kind, err5, c6.Kind, err6, c6.Reason, err5, err6)
		}
	}
	// The reported-claims margins and the kin clause are verbatim v6: at
	// own E = 4 the refusal reasons sit exactly where v5's do.
	for _, tc := range []struct {
		name                             string
		reportedCapital, reportedGranary int64
		kinRelated                       bool
		kinDistance                      int
		want                             strategy.NaturalReplyAction
		reason                           strategy.NaturalReplyReason
	}{
		{"reported capital one below refuses", 1, 3, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentClaims},
		{"reported granary one below refuses", 2, 2, false, 0, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentClaims},
		{"sibling distance 2 prohibited", 2, 3, true, 2, strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonKin},
		{"cousin distance 3 allowed", 2, 3, true, 3, strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone},
	} {
		view := naturalReplyView(2, 1, ref6, 5)
		view.Energy = sim.IntegerValue(4)
		view.Pending[0].ReportedCapital = tc.reportedCapital
		view.Pending[0].ReportedGranary = tc.reportedGranary
		view.Pending[0].KinRelated = tc.kinRelated
		view.Pending[0].KinDistance = tc.kinDistance
		c, err := b2v6.EvaluateBirthReplyV6(view)
		if err != nil || c.Kind != tc.want || c.Reason != tc.reason || c.Requester != 1 {
			t.Fatalf("v6 %s: %+v %v", tc.name, c, err)
		}
	}
	// The shared guard rails are verbatim: one reply per actor-hour, a dead
	// consenter, and an empty addressed set all stay silent in v6.
	guard := naturalReplyView(2, 1, ref6, 5)
	guard.OwnLastReplyHour = 5
	if c, err := b2v6.EvaluateBirthReplyV6(guard); err != nil || c.Kind != strategy.NaturalReplyNone {
		t.Fatalf("v6 second reply in one hour: %+v %v", c, err)
	}
	dead := naturalReplyView(2, 1, ref6, 5)
	dead.Energy = sim.IntegerValue(0)
	if c, err := b2v6.EvaluateBirthReplyV6(dead); err != nil || c.Kind != strategy.NaturalReplyNone || c.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("v6 dead consenter: %+v %v", c, err)
	}
	empty := naturalReplyView(2, 1, ref6, 5)
	empty.Pending = nil
	if c, err := b2v6.EvaluateBirthReplyV6(empty); err != nil || c.Kind != strategy.NaturalReplyNone {
		t.Fatalf("v6 empty addressed set: %+v %v", c, err)
	}
	// Exhaustive small-grid parity with the world's frozen consent evaluator:
	// v6 accepts exactly when that evaluator accepts with the own-energy leg
	// projected onto the v6 floor (energy >= 4 reads as 3, else 2).
	for _, granary := range []int64{2, 3, 8} {
		for _, capital := range []int64{1, 2, 8} {
			for _, energy := range []int64{2, 3, 4, 8} {
				for _, reportedCapital := range []int64{1, 2} {
					for _, reportedGranary := range []int64{2, 3, 1344} {
						for _, kin := range []struct {
							related  bool
							distance int
						}{{false, 0}, {true, 1}, {true, 2}, {true, 3}, {true, 10}} {
							view := naturalReplyView(2, 1, ref6, 5)
							view.GranaryStock = sim.IntegerValue(granary)
							view.Capital = sim.IntegerValue(capital)
							view.Energy = sim.IntegerValue(energy)
							view.Pending[0].ReportedCapital = reportedCapital
							view.Pending[0].ReportedGranary = reportedGranary
							view.Pending[0].KinRelated = kin.related
							view.Pending[0].KinDistance = kin.distance
							c, err := b2v6.EvaluateBirthReplyV6(view)
							projected := int64(2)
							if energy >= 4 {
								projected = 3
							}
							kinAllowed := !world.NaturalKinProhibited(kin.distance, kin.related)
							allowed := world.NaturalConsentAllows(granary, capital, projected, reportedCapital, reportedGranary, kinAllowed)
							if err != nil || (c.Kind == strategy.NaturalReplyAccept) != allowed {
								t.Fatalf("v6 consent grid diverges from the world evaluator: own %d/%d/%d claims %d/%d kin %v/%d: %+v %v world=%v",
									granary, capital, energy, reportedCapital, reportedGranary, kin.related, kin.distance, c, err, allowed)
							}
						}
					}
				}
			}
		}
	}
}

// TestNaturalV6PrivacyFieldSetsParityWithV5 reflects the shared observation,
// view, and choice types and asserts the exact frozen v5 field lists: the v6
// lane introduces no new type, no new field, and therefore no channel for
// slot tenure, cultivation assignment, debt carries, spoilage counters,
// allocator state, R, streams, other actors' state, death hours, or
// claim-vs-truth divergence — the v6 cascades evaluate these very types.
func TestNaturalV6PrivacyFieldSetsParityWithV5(t *testing.T) {
	want := map[reflect.Type][]string{
		reflect.TypeOf(strategy.NaturalObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Hunger", "BagUnits",
			"BagSource", "Capital", "Wip", "GranaryStock", "InvestedUnits", "LastBuildHour", "LastStoredMealHour", "LastDenial", "HomeSlots"},
		reflect.TypeOf(strategy.NaturalSlotView{}): {"ID", "Stock"},
		reflect.TypeOf(strategy.NaturalRequestObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "HomePatch", "Energy", "Capital",
			"GranaryStock", "OwnRequest", "PatchMates", "TestClaims"},
		reflect.TypeOf(strategy.NaturalRequestGuard{}):  {"Status", "LastRequestHour"},
		reflect.TypeOf(strategy.NaturalClaimOverride{}): {"Capital", "Granary"},
		reflect.TypeOf(strategy.NaturalReplyObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Capital", "GranaryStock",
			"OwnLastReplyHour", "Pending"},
		reflect.TypeOf(strategy.NaturalPendingView{}): {"Requester", "Addressee", "Hour", "ReportedCapital", "ReportedGranary",
			"KinDistance", "KinRelated"},
		reflect.TypeOf(strategy.NaturalChoice{}): {"Kind", "TargetSlot", "RestDuration", "ObservedVersion", "Ref", "Evaluated", "Fallback"},
		reflect.TypeOf(strategy.NaturalRequestChoice{}): {"Kind", "Addressee", "ReportedCapital", "ReportedGranary", "ObservedVersion",
			"Ref", "Evaluated", "Fallback"},
		reflect.TypeOf(strategy.NaturalReplyChoice{}): {"Kind", "Requester", "Reason", "ObservedVersion", "Ref", "Evaluated", "Fallback"},
	}
	for typ, fields := range want {
		got := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Name)
		}
		if !reflect.DeepEqual(got, fields) {
			t.Fatalf("%s field set drifted from the frozen v5 parity list: %v", typ, got)
		}
		forbiddenNames := []string{"Tenure", "TenureMode", "TenureOwner", "Owner", "Mode", "Cultivated", "CultivatedSlot",
			"Debt", "BasalDebt", "YieldDebt", "WearDebt", "SpoilDebt", "Spoil", "Spoiled", "Allocator", "Seed", "R",
			"RegenBudget", "DeathHour", "DiedHour", "Stream", "ClaimDelta", "ClaimMatchesTruth"}
		for _, forbidden := range forbiddenNames {
			if _, present := typ.FieldByName(forbidden); present {
				t.Fatalf("%s leaked %s", typ, forbidden)
			}
		}
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			for _, word := range []string{"Tenure", "Owner", "Mode", "Debt", "Spoil", "Allocator", "Seed", "Death", "Died", "Stream"} {
				if strings.Contains(name, word) {
					t.Fatalf("%s field %s carries privileged %s data", typ, name, word)
				}
			}
		}
	}
	// A v6 bound never evaluates another actor's observation, request view,
	// or reply view.
	b1, ref6 := natural6Fixture(t, 1)
	b2, _ := natural6Fixture(t, 2)
	if _, err := b2.EvaluateV6(naturalObservation(t, 1, ref6, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a v6 bound evaluated another actor's claim view")
	}
	if _, err := b2.EvaluateBirthRequestV6(naturalRequestView(t, 1, ref6, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a v6 bound evaluated another actor's request view")
	}
	if _, err := b2.EvaluateBirthReplyV6(naturalReplyView(1, 3, ref6, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a v6 bound evaluated another actor's reply view")
	}
	if b1.Policy() != strategy.FrozenNaturalPolicyV6() || b1.Binding() != (strategy.NaturalBinding{Actor: 1, Ref: ref6}) {
		t.Fatalf("v6 accessors exposed non-frozen or foreign content: %+v %+v", b1.Policy(), b1.Binding())
	}
	mutated := b1.Policy()
	mutated.Ref.ID = "mutated"
	if b1.Policy() != strategy.FrozenNaturalPolicyV6() {
		t.Fatal("returned policy mutated bound v6 content")
	}
}

// TestNaturalV6ObservationValidationParityWithV5 runs the frozen v5
// structural-validation sets through both cascades and requires identical
// outcomes: errors for structural malformations, Wait + NaturalInvalidSelf
// for out-of-range own state, and identical valid-path decisions.
func TestNaturalV6ObservationValidationParityWithV5(t *testing.T) {
	b5, b6, ref5, ref6 := naturalV5V6Bounds(t)
	type outcome struct {
		err        bool
		kind       strategy.NaturalAction
		fallback   strategy.NaturalFallback
		evaluatedN int
	}
	runClaim := func(b *strategy.NaturalBound, obs strategy.NaturalObservation) outcome {
		c, err := b.EvaluateV6(obs)
		if b.Policy().FormatVersion == 5 {
			c, err = b.Evaluate(obs)
		}
		return outcome{err != nil, c.Kind, c.Fallback, c.Evaluated}
	}
	base5 := naturalObservation(t, 1, ref5, 5)
	base6 := naturalObservation(t, 1, ref6, 5)
	const (
		wantErr = iota
		wantInvalidSelf
		wantValid
	)
	for _, tc := range []struct {
		name   string
		class  int
		kind   strategy.NaturalAction
		mutate func(*strategy.NaturalObservation)
	}{
		{"actor mismatch", wantErr, 0, func(o *strategy.NaturalObservation) { o.Actor = 2 }},
		{"ref mismatch", wantErr, 0, func(o *strategy.NaturalObservation) { o.Ref = strategy.NaturalRef{ID: "nonsense", Version: 9} }},
		{"unknown denial", wantErr, 0, func(o *strategy.NaturalObservation) { o.LastDenial = 255 }},
		{"foreign slot", wantErr, 0, func(o *strategy.NaturalObservation) { o.HomeSlots[0].ID = 2009 }},
		{"duplicate slot", wantErr, 0, func(o *strategy.NaturalObservation) { o.HomeSlots[1].ID = o.HomeSlots[0].ID }},
		{"nine slots", wantErr, 0, func(o *strategy.NaturalObservation) {
			o.HomeSlots = append(append([]strategy.NaturalSlotView(nil), o.HomeSlots...), strategy.NaturalSlotView{ID: 2017, Stock: sim.IntegerValue(0)})
		}},
		{"slot stock 2", wantErr, 0, func(o *strategy.NaturalObservation) { o.HomeSlots[2].Stock = sim.IntegerValue(2) }},
		{"hour -1", wantErr, 0, func(o *strategy.NaturalObservation) { o.Hour = -1 }},
		{"hour 168", wantErr, 0, func(o *strategy.NaturalObservation) { o.Hour = world.NaturalHorizonHours }},
		{"energy high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.Energy = sim.IntegerValue(13) }},
		{"energy low", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.Energy = sim.IntegerValue(-1) }},
		{"energy text", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.Energy = sim.BoolValue(true) }},
		{"hunger high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.Hunger = sim.IntegerValue(25) }},
		{"bag high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.BagUnits = sim.IntegerValue(2) }},
		{"bag low", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.BagUnits = sim.IntegerValue(-1) }},
		{"capital high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.Capital = sim.IntegerValue(9) }},
		{"wip high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.Wip = sim.IntegerValue(2) }},
		{"granary high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.GranaryStock = sim.IntegerValue(1345) }},
		{"granary low", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.GranaryStock = sim.IntegerValue(-1) }},
		{"invested high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) {
			o.InvestedUnits = sim.IntegerValue(world.NaturalHorizonHours + 1)
		}},
		{"build hour high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.LastBuildHour = sim.IntegerValue(world.NaturalHorizonHours) }},
		{"meal hour high", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) {
			o.LastStoredMealHour = sim.IntegerValue(world.NaturalHorizonHours)
		}},
		{"build hour future", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.LastBuildHour = sim.IntegerValue(int64(o.Hour) + 1) }},
		{"meal hour future", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.LastStoredMealHour = sim.IntegerValue(int64(o.Hour) + 1) }},
		{"source wrong kind", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.BagSource = sim.IntegerValue(1001) }},
		{"source invalid patch", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.BagSource, _ = sim.EntityRefValue(1003) }},
		{"founder foreign patch", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) { o.BagSource, _ = sim.EntityRefValue(1002) }},
		{"held source missing", wantInvalidSelf, 0, func(o *strategy.NaturalObservation) {
			o.BagUnits = sim.IntegerValue(1)
			absent, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
			o.BagSource = absent
		}},
		{"build denial without a held unit", wantErr, 0, func(o *strategy.NaturalObservation) { o.LastDenial = strategy.NaturalBuildDenied }},
		{"envelope granary", wantValid, strategy.NaturalEatStored, func(o *strategy.NaturalObservation) { o.GranaryStock = sim.IntegerValue(world.NaturalGranaryStockMax) }},
		{"held at the shared band builds", wantValid, strategy.NaturalBuild, func(o *strategy.NaturalObservation) {
			home, _ := world.NaturalFounderPatchID(naturalTestPatches, 1)
			o.Energy = sim.IntegerValue(5)
			o.BagUnits = sim.IntegerValue(1)
			o.BagSource, _ = sim.EntityRefValue(home)
		}},
	} {
		o5, o6 := base5, base6
		o5.HomeSlots = append([]strategy.NaturalSlotView(nil), o5.HomeSlots...)
		o6.HomeSlots = append([]strategy.NaturalSlotView(nil), o6.HomeSlots...)
		tc.mutate(&o5)
		tc.mutate(&o6)
		r5, r6 := runClaim(b5, o5), runClaim(b6, o6)
		if r5 != r6 {
			t.Fatalf("%s: v5 and v6 validation diverged: %+v vs %+v", tc.name, r5, r6)
		}
		switch tc.class {
		case wantErr:
			if !r5.err {
				t.Fatalf("%s accepted: %+v", tc.name, r5)
			}
		case wantInvalidSelf:
			if r5.err || r5.kind != strategy.NaturalWait || r5.fallback != strategy.NaturalInvalidSelf || r5.evaluatedN != 0 {
				t.Fatalf("%s: want Wait + NaturalInvalidSelf, got %+v", tc.name, r5)
			}
		case wantValid:
			if r5.err || r5.kind != tc.kind {
				t.Fatalf("%s: want %v, got %+v", tc.name, tc.kind, r5)
			}
		}
	}
	// Request and reply structural sets behave identically; the base views
	// sit above both margins (energy 8) so only structure is under test.
	runRequest := func(b *strategy.NaturalBound, obs strategy.NaturalRequestObservation) outcome {
		c, err := b.EvaluateBirthRequestV6(obs)
		if b.Policy().FormatVersion == 5 {
			c, err = b.EvaluateBirthRequest(obs)
		}
		return outcome{err != nil, strategy.NaturalAction(c.Kind), c.Fallback, c.Evaluated}
	}
	baseRequest5 := naturalRequestView(t, 1, ref5, 5)
	baseRequest6 := naturalRequestView(t, 1, ref6, 5)
	for _, tc := range []struct {
		name   string
		class  int
		mutate func(*strategy.NaturalRequestObservation)
	}{
		{"request actor mismatch", wantErr, func(o *strategy.NaturalRequestObservation) { o.Actor = 2 }},
		{"request ref mismatch", wantErr, func(o *strategy.NaturalRequestObservation) { o.Ref = strategy.NaturalRef{ID: "nonsense", Version: 9} }},
		{"self mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{1, 2} }},
		{"duplicate mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{2, 2} }},
		{"cross-patch mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{9} }},
		{"stretch mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{17} }},
		{"patch mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{1001} }},
		{"slot mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{2001} }},
		{"genesis mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{2016} }},
		{"sentinel mate", wantErr, func(o *strategy.NaturalRequestObservation) { o.PatchMates = []sim.EntityID{sim.EntityID(^uint64(0))} }},
		{"unknown status", wantErr, func(o *strategy.NaturalRequestObservation) { o.OwnRequest.Status = 5 }},
		{"future guard", wantErr, func(o *strategy.NaturalRequestObservation) { o.OwnRequest.LastRequestHour = 6 }},
		{"idle with hour", wantErr, func(o *strategy.NaturalRequestObservation) { o.OwnRequest.LastRequestHour = 4 }},
		{"filed without hour", wantErr, func(o *strategy.NaturalRequestObservation) {
			o.OwnRequest = strategy.NaturalRequestGuard{Status: strategy.NaturalRequestRefused, LastRequestHour: -1}
		}},
		{"wrong founder patch", wantErr, func(o *strategy.NaturalRequestObservation) { o.HomePatch = 1002 }},
		{"invalid patch", wantErr, func(o *strategy.NaturalRequestObservation) { o.HomePatch = 1003 }},
		{"claim capital high", wantErr, func(o *strategy.NaturalRequestObservation) {
			o.TestClaims = &strategy.NaturalClaimOverride{Capital: 9, Granary: 3}
		}},
		{"claim granary negative", wantErr, func(o *strategy.NaturalRequestObservation) {
			o.TestClaims = &strategy.NaturalClaimOverride{Capital: 2, Granary: -1}
		}},
		{"claim past envelope", wantErr, func(o *strategy.NaturalRequestObservation) {
			o.TestClaims = &strategy.NaturalClaimOverride{Capital: 2, Granary: 1345}
		}},
		{"request hour 168", wantErr, func(o *strategy.NaturalRequestObservation) { o.Hour = world.NaturalHorizonHours }},
		{"own capital high", wantInvalidSelf, func(o *strategy.NaturalRequestObservation) { o.Capital = sim.IntegerValue(9) }},
		{"own granary past envelope", wantInvalidSelf, func(o *strategy.NaturalRequestObservation) { o.GranaryStock = sim.IntegerValue(1345) }},
		{"own energy unknown", wantInvalidSelf, func(o *strategy.NaturalRequestObservation) { o.Energy = sim.BoolValue(true) }},
	} {
		o5, o6 := baseRequest5, baseRequest6
		o5.PatchMates = append([]sim.EntityID(nil), o5.PatchMates...)
		o6.PatchMates = append([]sim.EntityID(nil), o6.PatchMates...)
		tc.mutate(&o5)
		tc.mutate(&o6)
		r5, r6 := runRequest(b5, o5), runRequest(b6, o6)
		if r5 != r6 {
			t.Fatalf("%s: v5 and v6 validation diverged: %+v vs %+v", tc.name, r5, r6)
		}
		if (tc.class == wantErr && !r5.err) || (tc.class == wantInvalidSelf && (r5.err || r5.fallback != strategy.NaturalInvalidSelf)) {
			t.Fatalf("%s: unexpected outcome class %+v", tc.name, r5)
		}
	}
	runReply := func(b *strategy.NaturalBound, obs strategy.NaturalReplyObservation) outcome {
		c, err := b.EvaluateBirthReplyV6(obs)
		if b.Policy().FormatVersion == 5 {
			c, err = b.EvaluateBirthReply(obs)
		}
		return outcome{err != nil, strategy.NaturalAction(c.Kind), c.Fallback, c.Evaluated}
	}
	baseReply5 := naturalReplyView(1, 2, ref5, 5)
	baseReply6 := naturalReplyView(1, 2, ref6, 5)
	for _, tc := range []struct {
		name   string
		class  int
		mutate func(*strategy.NaturalReplyObservation)
	}{
		{"reply actor mismatch", wantErr, func(o *strategy.NaturalReplyObservation) { o.Actor = 2 }},
		{"reply ref mismatch", wantErr, func(o *strategy.NaturalReplyObservation) { o.Ref = strategy.NaturalRef{ID: "nonsense", Version: 9} }},
		{"future reply hour", wantErr, func(o *strategy.NaturalReplyObservation) { o.OwnLastReplyHour = 6 }},
		{"requester self", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 1 }},
		{"requester stretch", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 17 }},
		{"requester patch", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 1001 }},
		{"requester slot", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 2016 }},
		{"requester sentinel", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = sim.EntityID(^uint64(0)) }},
		{"requester zero", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Requester = 0 }},
		{"addressee other", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Addressee = 2 }},
		{"stale hour", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].Hour = 4 }},
		{"claim high", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].ReportedCapital = 9 }},
		{"claim negative", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].ReportedGranary = -1 }},
		{"claim past envelope", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending[0].ReportedGranary = 1345 }},
		{"related zero distance", wantErr, func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = true, 0
		}},
		{"distance beyond walk", wantErr, func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = true, 11
		}},
		{"unrelated with distance", wantErr, func(o *strategy.NaturalReplyObservation) {
			o.Pending[0].KinRelated, o.Pending[0].KinDistance = false, 1
		}},
		{"duplicate requester", wantErr, func(o *strategy.NaturalReplyObservation) { o.Pending = append(o.Pending, o.Pending[0]) }},
		{"reply hour 168", wantErr, func(o *strategy.NaturalReplyObservation) { o.Hour = world.NaturalHorizonHours }},
		{"own reply granary past envelope", wantInvalidSelf, func(o *strategy.NaturalReplyObservation) { o.GranaryStock = sim.IntegerValue(1345) }},
	} {
		o5, o6 := baseReply5, baseReply6
		tc.mutate(&o5)
		tc.mutate(&o6)
		r5, r6 := runReply(b5, o5), runReply(b6, o6)
		if r5 != r6 {
			t.Fatalf("%s: v5 and v6 validation diverged: %+v vs %+v", tc.name, r5, r6)
		}
		if (tc.class == wantErr && !r5.err) || (tc.class == wantInvalidSelf && (r5.err || r5.fallback != strategy.NaturalInvalidSelf)) {
			t.Fatalf("%s: unexpected outcome class %+v", tc.name, r5)
		}
	}
}

// TestNaturalV6BudgetExhaustion pins the shared budget semantics: the frozen
// v6 ceiling is 32/32, same-ref budget variants register, the bag decision
// only requires a nonzero budget, and every list input degrades all-or-
// nothing to the typed NaturalBudgetExhausted fallback — never a prefix.
func TestNaturalV6BudgetExhaustion(t *testing.T) {
	limited := strategy.NewNaturalRegistry(naturalTestPatches)
	p := strategy.FrozenNaturalPolicyV6()
	p.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
	if err := limited.RegisterFrozenNatural(p); err != nil {
		t.Fatal(err)
	}
	b, err := limited.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	// Two stocked slots against a one-slot budget: the whole candidate set
	// is declined, never a sorted prefix.
	obs := naturalStocked(naturalObservation(t, 1, p.Ref, 5), 0, 1)
	choice, err := b.EvaluateV6(obs)
	if err != nil || choice != (strategy.NaturalChoice{Kind: strategy.NaturalWait, ObservedVersion: 9,
		Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing v6 claim budget: %+v %v", choice, err)
	}
	if choice, err := b.EvaluateV6(naturalStocked(naturalObservation(t, 1, p.Ref, 5), 4)); err != nil ||
		choice.Kind != strategy.NaturalGather || choice.TargetSlot != 2005 || choice.Evaluated != 1 {
		t.Fatalf("bounded v6 candidate set: %+v %v", choice, err)
	}
	// The bag decision needs only a nonzero budget, verbatim v5 semantics:
	// at E = 5 it builds under a 1/1 budget, and the denial fallback eats.
	held := naturalHeldAtMargins(t, 1, p.Ref, 5)
	held.Energy = sim.IntegerValue(5)
	if choice, err := b.EvaluateV6(held); err != nil || choice.Kind != strategy.NaturalBuild || choice.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("v6 bag decision under a 1/1 budget: %+v %v", choice, err)
	}
	held.LastDenial = strategy.NaturalBuildDenied
	if choice, err := b.EvaluateV6(held); err != nil || choice.Kind != strategy.NaturalEat {
		t.Fatalf("v6 denial fallback under a 1/1 budget: %+v %v", choice, err)
	}
	// Two same-patch mates against a one-mate request budget: no filing.
	request := naturalRequestView(t, 1, p.Ref, 5)
	request.Energy = sim.IntegerValue(4)
	choice2, err := b.EvaluateBirthRequestV6(request)
	if err != nil || choice2 != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestWait,
		ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing v6 request budget: %+v %v", choice2, err)
	}
	request.PatchMates = []sim.EntityID{2}
	if c, err := b.EvaluateBirthRequestV6(request); err != nil || c.Kind != strategy.NaturalRequestFile || c.Addressee != 2 {
		t.Fatalf("bounded v6 request set: %+v %v", c, err)
	}
	// Two pending requests against a one-reply budget: no reply at all.
	reply := naturalReplyView(1, 2, p.Ref, 5)
	reply.Pending = append(reply.Pending, strategy.NaturalPendingView{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	choice3, err := b.EvaluateBirthReplyV6(reply)
	if err != nil || choice3 != (strategy.NaturalReplyChoice{Kind: strategy.NaturalReplyNone,
		ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing v6 reply budget: %+v %v", choice3, err)
	}
	// The frozen 32/32 v6 budget covers every maximal legal set: all eight
	// stocked slots, a 32-mate roster, and a 32-pending list evaluate
	// without exhaustion; the 33rd roster entry is a typed budget event.
	fb, ref6 := natural6Fixture(t, 1)
	all := naturalObservation(t, 1, ref6, 5)
	for i := range all.HomeSlots {
		all.HomeSlots[i].Stock = sim.IntegerValue(1)
	}
	if c, err := fb.EvaluateV6(all); err != nil || c.Kind != strategy.NaturalGather ||
		c.TargetSlot != 2001 || c.Evaluated != 8 || c.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("frozen v6 budget must never truncate the legal slot set: %+v %v", c, err)
	}
	big := naturalRequestView(t, 1, ref6, 5)
	big.Energy = sim.IntegerValue(4)
	for id := sim.EntityID(2017); len(big.PatchMates) < 32; id++ {
		big.PatchMates = append(big.PatchMates, id)
	}
	if c, err := fb.EvaluateBirthRequestV6(big); err != nil || c.Kind != strategy.NaturalRequestFile ||
		c.Addressee != 2 || c.Evaluated != 32 || c.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("frozen v6 budget must never truncate the legal roster: %+v %v", c, err)
	}
	big.PatchMates = append(big.PatchMates, 2049)
	if c, err := fb.EvaluateBirthRequestV6(big); err != nil ||
		c != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestWait,
			ObservedVersion: 9, Ref: ref6, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("over-budget v6 roster must be a typed budget event: %+v %v", c, err)
	}
	full := naturalReplyView(1, 2, ref6, 5)
	for id := sim.EntityID(3); id <= 8; id++ {
		full.Pending = append(full.Pending, strategy.NaturalPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	}
	for id := sim.EntityID(2017); len(full.Pending) < 32; id++ {
		full.Pending = append(full.Pending, strategy.NaturalPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
	}
	if c, err := fb.EvaluateBirthReplyV6(full); err != nil || c.Kind != strategy.NaturalReplyAccept ||
		c.Requester != 2 || c.Evaluated != 1 || c.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("frozen v6 budget must never truncate the legal pending set: %+v %v", c, err)
	}
}

// TestNaturalV6DeterminismAndOrderIndependence pins the memoryless v6
// evaluation: shuffled slot, roster, and pending orders decide identically,
// and repeated evaluation of the same observation returns the same choice.
func TestNaturalV6DeterminismAndOrderIndependence(t *testing.T) {
	b, ref := natural6Fixture(t, 1)
	// Claim: the lowest stocked slot ID wins regardless of input order.
	shuffled := naturalStocked(naturalObservation(t, 1, ref, 5), 2, 0, 5)
	first, err := b.EvaluateV6(shuffled)
	if err != nil || first.Kind != strategy.NaturalGather || first.TargetSlot != 2001 || first.Evaluated != 3 {
		t.Fatalf("v6 gather must take the lowest stocked slot: %+v %v", first, err)
	}
	reordered := naturalStocked(naturalObservation(t, 1, ref, 5), 5, 2, 0)
	reordered.HomeSlots[3], reordered.HomeSlots[6] = reordered.HomeSlots[6], reordered.HomeSlots[3]
	if again, err := b.EvaluateV6(reordered); err != nil || naturalChoiceShape(again) != naturalChoiceShape(first) {
		t.Fatalf("v6 shuffled slots must not change the choice: %+v %v", again, err)
	}
	// Bag decision and stored-meal guard are repeatable.
	held := naturalHeldAtMargins(t, 1, ref, 5)
	held.Energy = sim.IntegerValue(4) // the exact re-pioneer margin: builds in v6
	want := strategy.NaturalChoice{Kind: strategy.NaturalBuild, ObservedVersion: 9, Ref: ref, Evaluated: 1}
	for i := 0; i < 3; i++ {
		if c, err := b.EvaluateV6(held); err != nil || c != want {
			t.Fatalf("v6 bag decision repeat %d: %+v %v", i, c, err)
		}
	}
	meal := naturalObservation(t, 1, ref, 5)
	meal.GranaryStock = sim.IntegerValue(2)
	mealChoice := strategy.NaturalChoice{Kind: strategy.NaturalEatStored, ObservedVersion: 9, Ref: ref, Evaluated: 1}
	for i := 0; i < 3; i++ {
		if c, err := b.EvaluateV6(meal); err != nil || c != mealChoice {
			t.Fatalf("v6 stored-meal repeat %d: %+v %v", i, c, err)
		}
	}
	// Request: a shuffled roster files the same addressee with the same
	// claims and the same evaluated count.
	request := naturalRequestView(t, 1, ref, 5)
	request.Energy = sim.IntegerValue(4)
	file, err := b.EvaluateBirthRequestV6(request)
	if err != nil || file.Kind != strategy.NaturalRequestFile || file.Addressee != 2 ||
		file.ReportedCapital != 2 || file.ReportedGranary != 3 || file.Evaluated != 7 {
		t.Fatalf("v6 filing: %+v %v", file, err)
	}
	descending := naturalRequestView(t, 1, ref, 5)
	descending.Energy = sim.IntegerValue(4)
	sort.Slice(descending.PatchMates, func(i, j int) bool { return descending.PatchMates[i] > descending.PatchMates[j] })
	if again, err := b.EvaluateBirthRequestV6(descending); err != nil || again != file {
		t.Fatalf("v6 shuffled roster must not change the filing: %+v %v", again, err)
	}
	// Reply: shuffled pendings accept the first acceptable in ascending
	// requester order, identically.
	multi := naturalReplyView(1, 3, ref, 5)
	multi.Pending = []strategy.NaturalPendingView{
		{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 1, ReportedGranary: 3}, // claims fail
		{Requester: 5, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3}, // acceptable
		{Requester: 2017, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3},
	}
	accepted, err := b.EvaluateBirthReplyV6(multi)
	if err != nil || accepted.Kind != strategy.NaturalReplyAccept || accepted.Requester != 5 || accepted.Evaluated != 2 {
		t.Fatalf("v6 first acceptable pending must win: %+v %v", accepted, err)
	}
	reversed := multi
	reversed.Pending = []strategy.NaturalPendingView{multi.Pending[2], multi.Pending[1], multi.Pending[0]}
	if again, err := b.EvaluateBirthReplyV6(reversed); err != nil || again != accepted {
		t.Fatalf("v6 input order must not change the reply: %+v %v", again, err)
	}
}

// TestNaturalV6Format6Codec pins the format-6 wire: identical untouched AWNP
// framing, canonical bytes, fingerprint identity, cross-ref verification
// failures, and the frozen v5 codec continuing to refuse v6 content.
func TestNaturalV6Format6Codec(t *testing.T) {
	v5, v6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	wire, err := strategy.EncodeFrozenNaturalPolicy(v6)
	if err != nil {
		t.Fatal(err)
	}
	const frozenWireLen = 4 + 4 + 1 + len("natural") + 4 + 1 + 1
	if len(wire) != frozenWireLen {
		t.Fatalf("v6 wire length %d, want %d: %x", len(wire), frozenWireLen, wire)
	}
	if !bytes.Equal(wire[:4], []byte("AWNP")) || wire[4] != 0 || wire[5] != 0 || wire[6] != 0 || wire[7] != 6 {
		t.Fatalf("v6 wire magic/format drifted: %x", wire[:8])
	}
	again, err := strategy.EncodeFrozenNaturalPolicy(v6)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatalf("v6 encoding is not deterministic: %x %x %v", wire, again, err)
	}
	if got, err := strategy.DecodeFrozenNaturalPolicy(wire); err != nil || got != v6 {
		t.Fatalf("v6 roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(wire, v6); err != nil {
		t.Fatalf("frozen v6 wire must verify: %v", err)
	}
	if sum, err := strategy.FrozenNaturalPolicyFingerprint(v6); err != nil || sum != sha256.Sum256(wire) {
		t.Fatalf("v6 fingerprint: %v %v", sum, err)
	}
	// Every single-byte mutation either fails decoding or fails ref-set
	// verification against the executable's frozen v6 policy.
	for i := range wire {
		bad := append([]byte(nil), wire...)
		bad[i] ^= 0xff
		if _, err := strategy.DecodeFrozenNaturalPolicy(bad); err == nil {
			if err := strategy.VerifyFrozenNaturalPolicy(bad, v6); err == nil {
				t.Fatalf("mutated v6 wire byte %d accepted: %x", i, bad)
			}
		}
	}
	for name, edit := range map[string]func([]byte) []byte{
		"empty":     func([]byte) []byte { return nil },
		"truncated": func(b []byte) []byte { return b[:len(b)-1] },
		"trailing":  func(b []byte) []byte { return append(b, 0) },
		"magic":     func(b []byte) []byte { b = append([]byte(nil), b...); b[0] = 'X'; return b },
		"length":    func(b []byte) []byte { b = append([]byte(nil), b...); b[8] = 0; return b },
	} {
		if _, err := strategy.DecodeFrozenNaturalPolicy(edit(wire)); !errors.Is(err, strategy.ErrInvalidNaturalPolicyEncoding) {
			t.Fatalf("%s v6 wire decoded: %v", name, err)
		}
		if err := strategy.VerifyFrozenNaturalPolicy(edit(wire), v6); err == nil {
			t.Fatalf("%s v6 wire verified", name)
		}
	}
	// Cross-ref verification fails closed in both directions.
	v5Wire, err := strategy.EncodeNaturalPolicy(v5)
	if err != nil {
		t.Fatal(err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(wire, v5); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("v6 wire verified against the v5 expectation: %v", err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(v5Wire, v6); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("v5 wire verified against the v6 expectation: %v", err)
	}
	// A same-ref budget variant is well-formed but can never be restored
	// against the frozen expectation; a drifted expectation fails closed
	// before any bytes are compared.
	variant := v6
	variant.Budget = strategy.NaturalBudget{Candidates: 2, Evaluations: 2}
	variantWire, err := strategy.EncodeFrozenNaturalPolicy(variant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := strategy.DecodeFrozenNaturalPolicy(variantWire); err != nil || got != variant {
		t.Fatalf("v6 variant roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(variantWire, v6); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("same-ref v6 budget drift restored: %v", err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(wire, variant); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("non-frozen v6 expectation accepted: %v", err)
	}
	drifted := v6
	drifted.Ref.Version = 7
	if _, err := strategy.EncodeFrozenNaturalPolicy(drifted); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("drifted v6 policy encoded: %v", err)
	}
	// The frozen v5 codec is untouched and keeps refusing v6 content, while
	// v5 bytes verify identically under both codecs.
	if _, err := strategy.EncodeNaturalPolicy(v6); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("v5 codec encoded the v6 policy: %v", err)
	}
	if _, err := strategy.DecodeNaturalPolicy(wire); err == nil {
		t.Fatal("v5 codec decoded the v6 wire")
	}
	if err := strategy.VerifyNaturalPolicy(wire, v5); err == nil {
		t.Fatal("v5 codec verified the v6 wire")
	}
	if _, err := strategy.NaturalPolicyFingerprint(v6); err == nil {
		t.Fatal("v5-only fingerprint accepted the v6 policy")
	}
	v5ViaSet, err := strategy.EncodeFrozenNaturalPolicy(v5)
	if err != nil || !bytes.Equal(v5ViaSet, v5Wire) {
		t.Fatalf("ref-set encoder diverges from the frozen v5 encoder: %x vs %x (%v)", v5ViaSet, v5Wire, err)
	}
	if got, err := strategy.DecodeFrozenNaturalPolicy(v5Wire); err != nil || got != v5 {
		t.Fatalf("ref-set decoder of v5 bytes: %+v %v", got, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(v5Wire, v5); err != nil {
		t.Fatalf("v5 bytes must verify under the ref-set codec: %v", err)
	}
	if sumSet, err := strategy.FrozenNaturalPolicyFingerprint(v5); err != nil {
		t.Fatal(err)
	} else if sumPlain, err := strategy.NaturalPolicyFingerprint(v5); err != nil || sumSet != sumPlain {
		t.Fatalf("v5 fingerprint identity broke: %v vs %v", sumSet, sumPlain)
	}
	if got, err := strategy.DecodeNaturalPolicy(v5Wire); err != nil || got != v5 {
		t.Fatalf("frozen v5 codec regression on v5 bytes: %+v %v", got, err)
	}
}
