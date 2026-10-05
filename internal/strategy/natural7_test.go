package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// natural7Fixture binds one actor to the frozen natural/7 policy through the
// ref-set-aware registration path (RegisterFrozenNatural).
func natural7Fixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy7()
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

// natural7bFixture binds one actor to the frozen natural/7b ablation policy.
func natural7bFixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy7b()
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

// naturalV6V7V7bBounds binds the three cascades under differential test into
// one registry: the v6 control, the natural/7 candidate, and the natural/7b
// ablation.
func naturalV6V7V7bBounds(t *testing.T) (b6, b7, b7b *strategy.NaturalBound, ref6, ref7, ref7b strategy.NaturalRef) {
	t.Helper()
	p6, p7, p7b := strategy.FrozenNaturalPolicyV6(), strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b()
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	for _, p := range []strategy.NaturalPolicy{p6, p7, p7b} {
		if err := r.RegisterFrozenNatural(p); err != nil {
			t.Fatal(err)
		}
	}
	bind := func(p strategy.NaturalPolicy) *strategy.NaturalBound {
		b, err := r.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	return bind(p6), bind(p7), bind(p7b), p6.Ref, p7.Ref, p7b.Ref
}

// TestNatural7FrozenIdentityRefSetAndRegistration pins the frozen natural/7
// and natural/7b identities, the extended ref-set resolver and predicate,
// the registration path, and the untouched v5-only validation.
func TestNatural7FrozenIdentityRefSetAndRegistration(t *testing.T) {
	p7, p7b := strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b()
	if p7 != (strategy.NaturalPolicy{FormatVersion: 7, Ref: strategy.NaturalRef{ID: "natural", Version: 7},
		Budget: strategy.NaturalBudget{Candidates: 32, Evaluations: 32}}) {
		t.Fatalf("frozen natural/7 policy drifted: %+v", p7)
	}
	if p7b != (strategy.NaturalPolicy{FormatVersion: 8, Ref: strategy.NaturalRef{ID: "natural", Version: 8},
		Budget: strategy.NaturalBudget{Candidates: 32, Evaluations: 32}}) {
		t.Fatalf("frozen natural/7b policy drifted: %+v", p7b)
	}
	p5, p6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	// The resolver covers the full frozen set {natural/5..8} and still fails
	// closed on everything else.
	for _, p := range []strategy.NaturalPolicy{p5, p6, p7, p7b} {
		if got, err := strategy.FrozenNaturalPolicyForRef(p.Ref); err != nil || got != p {
			t.Fatalf("natural/%d resolution: %+v %v", p.Ref.Version, got, err)
		}
	}
	for _, unknown := range []strategy.NaturalRef{
		{ID: "natural", Version: 9}, {ID: "natural", Version: 0}, {ID: "other", Version: 7},
		{ID: "natural-7", Version: 7}, {ID: "natural-7b", Version: 8}, {},
	} {
		if got, err := strategy.FrozenNaturalPolicyForRef(unknown); !errors.Is(err, strategy.ErrUnknownNaturalPolicy) {
			t.Fatalf("unknown ref %v resolved to %+v (%v)", unknown, got, err)
		}
	}
	if !strategy.ValidFrozenNaturalPolicy(p5) || !strategy.ValidFrozenNaturalPolicy(p6) ||
		!strategy.ValidFrozenNaturalPolicy(p7) || !strategy.ValidFrozenNaturalPolicy(p7b) {
		t.Fatal("frozen ref-set members must validate")
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	mutate := func(p strategy.NaturalPolicy, edit func(*strategy.NaturalPolicy)) strategy.NaturalPolicy {
		c := p
		edit(&c)
		return c
	}
	for _, invalid := range []strategy.NaturalPolicy{
		mutate(p7, func(c *strategy.NaturalPolicy) { c.FormatVersion = 5 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.FormatVersion = 6 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.FormatVersion = 8 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref = strategy.NaturalRef{ID: "natural", Version: 6} }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref = strategy.NaturalRef{ID: "natural", Version: 8} }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref.ID = "" }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref.ID = longID }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref.Version = 0 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref.Version = 9 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Budget.Candidates = 0 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Budget.Candidates = 33 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Budget.Evaluations = 0 }),
		mutate(p7, func(c *strategy.NaturalPolicy) { c.Budget.Evaluations = 33 }),
		mutate(p7b, func(c *strategy.NaturalPolicy) { c.FormatVersion = 7 }),
		mutate(p7b, func(c *strategy.NaturalPolicy) { c.Ref = strategy.NaturalRef{ID: "natural", Version: 7} }),
		mutate(p7b, func(c *strategy.NaturalPolicy) { c.Ref.Version = 9 }),
		mutate(p7b, func(c *strategy.NaturalPolicy) { c.Budget.Evaluations = 33 }),
	} {
		if strategy.ValidFrozenNaturalPolicy(invalid) {
			t.Fatalf("invalid policy passed the ref-set predicate: %+v", invalid)
		}
	}
	// The frozen v5 Register keeps its v5-only validation: it accepts v5 and
	// rejects v6, v7, and v8, untouched.
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := r.Register(p5); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]strategy.NaturalPolicy{"v6": p6, "v7": p7, "v7b": p7b} {
		if err := r.Register(p); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("frozen v5 Register accepted the %s ref: %v", name, err)
		}
	}
	// The ref-set-aware path registers every frozen ref (and same-ref budget
	// variants) with the same shape semantics per ref.
	variant := p7
	variant.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
	for name, tc := range map[string]struct {
		policy strategy.NaturalPolicy
		ok     bool
	}{
		"frozen v5":               {p5, true},
		"frozen v6":               {p6, true},
		"frozen v7":               {p7, true},
		"frozen v7b":              {p7b, true},
		"same-ref budget variant": {variant, true},
		"format 7 with ref 8":     {mutate(p7, func(c *strategy.NaturalPolicy) { c.FormatVersion = 8 }), false},
		"format 8 with ref 7":     {mutate(p7b, func(c *strategy.NaturalPolicy) { c.FormatVersion = 7 }), false},
		"budget 0":                {mutate(p7, func(c *strategy.NaturalPolicy) { c.Budget.Candidates = 0 }), false},
		"budget 33":               {mutate(p7b, func(c *strategy.NaturalPolicy) { c.Budget.Evaluations = 33 }), false},
		"unfrozen ref 9":          {mutate(p7, func(c *strategy.NaturalPolicy) { c.Ref.Version = 9 }), false},
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
	// Duplicate refs are rejected on the ref-set-aware path too.
	if err := r.RegisterFrozenNatural(p7); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterFrozenNatural(p7); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("duplicate natural/7 reference accepted: %v", err)
	}
	// All four frozen refs coexist in one registry and bind founders and
	// newborns by their own ref.
	all := strategy.NewNaturalRegistry(naturalTestPatches)
	for _, p := range []strategy.NaturalPolicy{p5, p6, p7, p7b} {
		if err := all.RegisterFrozenNatural(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		actor sim.EntityID
		p     strategy.NaturalPolicy
	}{{1, p5}, {2, p6}, {2017, p7}, {2018, p7b}} {
		if err := all.RegisterFrozenNatural(tc.p); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("duplicate natural/%d registration must fail: %v", tc.p.Ref.Version, err)
		}
		b, err := all.Bind(strategy.NaturalBinding{Actor: tc.actor, Ref: tc.p.Ref})
		if err != nil || b.Policy() != tc.p || b.Binding() != (strategy.NaturalBinding{Actor: tc.actor, Ref: tc.p.Ref}) {
			t.Fatalf("binding actor %d to natural/%d: %+v %v", tc.actor, tc.p.Ref.Version, b, err)
		}
	}
	// A ref absent from a registry's registered set still fails at Bind.
	only5 := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := only5.RegisterFrozenNatural(p5); err != nil {
		t.Fatal(err)
	}
	if _, err := only5.Bind(strategy.NaturalBinding{Actor: 1, Ref: p7.Ref}); !errors.Is(err, strategy.ErrUnknownNaturalPolicy) {
		t.Fatalf("unregistered natural/7 ref bound: %v", err)
	}
	var nilRegistry *strategy.NaturalRegistry
	if err := nilRegistry.RegisterFrozenNatural(p7); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil registry registered the natural/7 policy")
	}
	// The 7/7b cascades fail closed under any other bound identity.
	b5, ref5 := naturalFixture(t, 1)
	b7, ref7 := natural7Fixture(t, 1)
	b7b, ref7b := natural7bFixture(t, 1)
	mkObs := func(ref strategy.NaturalRef) strategy.NaturalObservation {
		o := naturalObservation(t, 1, ref, 5)
		o.Energy = sim.IntegerValue(4)
		o.GranaryStock = sim.IntegerValue(1)
		return o
	}
	if _, err := b5.Evaluate7(mkObs(ref5)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("7 cascade accepted a v5 bound: %v", err)
	}
	if _, err := b7.Evaluate7b(mkObs(ref7)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("7b cascade accepted a natural/7 bound: %v", err)
	}
	if _, err := b7b.Evaluate7(mkObs(ref7b)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("7 cascade accepted a natural/7b bound: %v", err)
	}
	var nilBound *strategy.NaturalBound
	if _, err := nilBound.Evaluate7(strategy.NaturalObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 7 cascade")
	}
	if _, err := nilBound.Evaluate7b(strategy.NaturalObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 7b cascade")
	}
	if _, err := nilBound.EvaluateBirthRequest7(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 7 request cascade")
	}
	if _, err := nilBound.EvaluateBirthRequest7b(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 7b request cascade")
	}
	if _, err := nilBound.EvaluateBirthReply7(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 7 reply cascade")
	}
	if _, err := nilBound.EvaluateBirthReply7b(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 7b reply cascade")
	}
}

// TestNatural7RefSetBackwardCompatPins proves the ref-set seam extension is
// byte-inert for the pre-existing {natural/5, natural/6} set: the wires,
// fingerprints, and resolved policy values below were captured from the
// executable BEFORE the natural6.go extension and are hard-coded here, so
// any drift in the {5,6} encodings, digests, or resolution fails loudly.
func TestNatural7RefSetBackwardCompatPins(t *testing.T) {
	const (
		// Captured at 2e67e5f before the ref-set extension.
		pinWire5 = "41574e5000000005076e61747572616c000000052020"
		pinSHA5  = "d73d24f8f4c5219f8bfcd1b6c49af1702936d83d2fa4e82da49218807ae11b5b"
		pinWire6 = "41574e5000000006076e61747572616c000000062020"
		pinSHA6  = "370c971e21f9a24c448368acf48adcdd75e45fd00bcb6bc55fd8947eb496cd72"
	)
	p5, p6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	for _, tc := range []struct {
		p         strategy.NaturalPolicy
		wire, sha string
	}{
		{p5, pinWire5, pinSHA5},
		{p6, pinWire6, pinSHA6},
	} {
		setWire, err := strategy.EncodeFrozenNaturalPolicy(tc.p)
		if err != nil || hex.EncodeToString(setWire) != tc.wire {
			t.Fatalf("natural/%d ref-set wire drifted from the pre-extension pin: %x (%v)", tc.p.Ref.Version, setWire, err)
		}
		setSum, err := strategy.FrozenNaturalPolicyFingerprint(tc.p)
		if err != nil || hex.EncodeToString(setSum[:]) != tc.sha {
			t.Fatalf("natural/%d ref-set fingerprint drifted from the pre-extension pin: %x (%v)", tc.p.Ref.Version, setSum, err)
		}
		wantWire, _ := hex.DecodeString(tc.wire)
		wantSum := sha256.Sum256(wantWire)
		if setSum != wantSum {
			t.Fatalf("natural/%d fingerprint is not the digest of the pinned wire", tc.p.Ref.Version)
		}
	}
	// The frozen v5-only codec still produces exactly the pinned v5 bytes
	// and fingerprint.
	v5Wire, err := strategy.EncodeNaturalPolicy(p5)
	if err != nil || hex.EncodeToString(v5Wire) != pinWire5 {
		t.Fatalf("v5 codec wire drifted: %x (%v)", v5Wire, err)
	}
	if sum, err := strategy.NaturalPolicyFingerprint(p5); err != nil || hex.EncodeToString(sum[:]) != pinSHA5 {
		t.Fatalf("v5 codec fingerprint drifted: %x (%v)", sum, err)
	}
	// The resolved {5,6} values are exactly the frozen contents, unchanged.
	if got, err := strategy.FrozenNaturalPolicyForRef(p5.Ref); err != nil || got != p5 {
		t.Fatalf("natural/5 resolution drifted: %+v %v", got, err)
	}
	if got, err := strategy.FrozenNaturalPolicyForRef(p6.Ref); err != nil || got != p6 {
		t.Fatalf("natural/6 resolution drifted: %+v %v", got, err)
	}
	// The new refs resolve to exactly the frozen 7/7b contents; unknown refs
	// still fail closed.
	if got, err := strategy.FrozenNaturalPolicyForRef(strategy.FrozenNaturalPolicy7().Ref); err != nil || got != strategy.FrozenNaturalPolicy7() {
		t.Fatalf("natural/7 resolution: %+v %v", got, err)
	}
	if got, err := strategy.FrozenNaturalPolicyForRef(strategy.FrozenNaturalPolicy7b().Ref); err != nil || got != strategy.FrozenNaturalPolicy7b() {
		t.Fatalf("natural/8 resolution: %+v %v", got, err)
	}
	if _, err := strategy.FrozenNaturalPolicyForRef(strategy.NaturalRef{ID: "natural", Version: 9}); !errors.Is(err, strategy.ErrUnknownNaturalPolicy) {
		t.Fatalf("natural/9 resolved: %v", err)
	}
}

// TestNatural7V6DifferentialGrid walks the full shared-observation grid —
// energy 0..12 × capital 0..8 × wip {0,1} × granary {0,1,3,8} × bag {0,1} ×
// denial {none, gather, build} × one stocked home slot {no, yes} — and
// asserts, per cell, the exact frozen-design delta set of each arm against
// the natural/6 control: flips occur EXACTLY at the provision branch and the
// two claim floors, i.e. exactly where the claim cascade replaces v6's bank
// draw — bag = 0, no denial, a living actor, no visible stocked slot,
// granary >= 1, and energy >= 4 (the provision floor; below it the E <= 3
// bank floor keeps v6's EatStored verbatim). Arm natural/7 flips
// EatStored -> Provision there; arm natural/7b flips EatStored -> Wait (the
// bank refused above the survival floor with the conversion disabled). Every
// other cell must be identical, so any additional flip family fails here
// rather than surviving into a run.
func TestNatural7V6DifferentialGrid(t *testing.T) {
	b6, b7, b7b, ref6, ref7, ref7b := naturalV6V7V7bBounds(t)
	const hour = 5
	base6 := naturalObservation(t, 1, ref6, hour)
	base7 := naturalObservation(t, 1, ref7, hour)
	base7b := naturalObservation(t, 1, ref7b, hour)

	type margins struct {
		energy, capital, wip, granary, bag int64
		denial                             strategy.NaturalDenial
		stocked                            bool
	}
	cell := func(m margins) (o6, o7, o7b strategy.NaturalObservation) {
		o6, o7, o7b = base6, base7, base7b
		for _, o := range []*strategy.NaturalObservation{&o6, &o7, &o7b} {
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
		return o6, o7, o7b
	}

	var cells, structuralErrors, bagCells, claimCells int
	flips := map[string]int{} // "7" and "7b"
	for energy := int64(0); energy <= 12; energy++ {
		for capital := int64(0); capital <= 8; capital++ {
			for wip := int64(0); wip <= 1; wip++ {
				for _, granary := range []int64{0, 1, 3, 8} {
					for bag := int64(0); bag <= 1; bag++ {
						for _, denial := range []strategy.NaturalDenial{strategy.NaturalNoDenial, strategy.NaturalGatherDenied, strategy.NaturalBuildDenied} {
							for _, stocked := range []bool{false, true} {
								m := margins{energy, capital, wip, granary, bag, denial, stocked}
								o6, o7, o7b := cell(m)
								c6, err6 := b6.EvaluateV6(o6)
								c7, err7 := b7.Evaluate7(o7)
								c7b, err7b := b7b.Evaluate7b(o7b)
								cells++
								// Structural acceptance is identical: a build
								// denial with an empty bag is malformed for
								// all three, everything else evaluates.
								if (err6 == nil) != (err7 == nil) || (err6 == nil) != (err7b == nil) {
									t.Fatalf("cell %+v: structural acceptance diverged: %v / %v / %v", m, err6, err7, err7b)
								}
								if err6 != nil {
									if !errors.Is(err6, strategy.ErrInvalidNaturalObservation) ||
										!errors.Is(err7, strategy.ErrInvalidNaturalObservation) ||
										!errors.Is(err7b, strategy.ErrInvalidNaturalObservation) {
										t.Fatalf("cell %+v: unexpected error class: %v %v %v", m, err6, err7, err7b)
									}
									structuralErrors++
									continue
								}
								// Expected choices restated from the frozen
								// cascades for the grid's fixed shape (hour 5,
								// no stored meal yet, the actor's own eight
								// home slots, at most slot 0 stocked). A Wait
								// is the no-affordance fallback (NoCandidate);
								// every decided action carries NoFallback.
								want := strategy.NaturalChoice{ObservedVersion: 9, Fallback: strategy.NaturalNoCandidate}
								want6, want7, want7b := strategy.NaturalWait, strategy.NaturalWait, strategy.NaturalWait
								if bag == 1 {
									bagCells++
									if energy > 0 { // a dead actor never acts
										want6 = naturalV6ExpectedBag(energy, capital, wip, granary, denial)
										want7, want7b = want6, want6 // the bag decision is verbatim v6
										want.Evaluated = 1
										want.Fallback = strategy.NaturalNoFallback
									}
								} else {
									claimCells++
									if energy > 0 {
										// The single flip family: the claim
										// bank draw at the provision floor.
										claimBankCell := denial == strategy.NaturalNoDenial && !stocked && granary >= 1 && energy >= 4
										if denial == strategy.NaturalGatherDenied {
											// Denied-gather fallback verbatim
											// v6 for both arms: the bank at
											// any energy, no provision.
											if granary >= 1 {
												want6, want7, want7b = strategy.NaturalEatStored, strategy.NaturalEatStored, strategy.NaturalEatStored
											} else {
												want6, want7, want7b = strategy.NaturalRest, strategy.NaturalRest, strategy.NaturalRest
												want.RestDuration = strategy.NaturalRestDuration
											}
											want.Evaluated = 1
											want.Fallback = strategy.NaturalNoFallback
										} else if stocked {
											want6, want7, want7b = strategy.NaturalGather, strategy.NaturalGather, strategy.NaturalGather
											want.TargetSlot, want.Evaluated = 2001, 1
											want.Fallback = strategy.NaturalNoFallback
										} else if claimBankCell {
											// The provision branch (natural/7)
											// against v6's bank draw; 7b waits.
											want6 = strategy.NaturalEatStored
											want7 = strategy.NaturalProvision
											want.Evaluated = 1
											want.Fallback = strategy.NaturalNoFallback
										} else if granary >= 1 {
											// Below the provision floor the
											// E <= 3 bank floor keeps v6's
											// EatStored verbatim in both arms.
											want6, want7, want7b = strategy.NaturalEatStored, strategy.NaturalEatStored, strategy.NaturalEatStored
											want.Evaluated = 1
											want.Fallback = strategy.NaturalNoFallback
										}
										// granary = 0: all three Wait.
										shared := []struct {
											ref  strategy.NaturalRef
											got  strategy.NaturalChoice
											want strategy.NaturalAction
										}{{ref6, c6, want6}, {ref7, c7, want7}}
										if !claimBankCell {
											shared = append(shared, struct {
												ref  strategy.NaturalRef
												got  strategy.NaturalChoice
												want strategy.NaturalAction
											}{ref7b, c7b, want7b})
										}
										for _, tc := range shared {
											expected := want
											expected.Kind, expected.Ref = tc.want, tc.ref
											if tc.got != expected {
												t.Fatalf("cell %+v: frozen cascade drift: got %+v, want %+v", m, tc.got, expected)
											}
										}
										if claimBankCell {
											if c6.Kind != strategy.NaturalEatStored {
												t.Fatalf("cell %+v: control must draw the bank: %v", m, c6.Kind)
											}
											if c7.Kind != strategy.NaturalProvision {
												t.Fatalf("cell %+v: natural/7 must flip EatStored->Provision: %v", m, c7.Kind)
											}
											if c7b != (strategy.NaturalChoice{Kind: strategy.NaturalWait, ObservedVersion: 9,
												Ref: ref7b, Fallback: strategy.NaturalNoCandidate}) {
												t.Fatalf("cell %+v: natural/7b must flip EatStored->Wait: %+v", m, c7b)
											}
											flips["7"]++
											flips["7b"]++
										} else if naturalChoiceShape(c6) != naturalChoiceShape(c7) {
											t.Fatalf("cell %+v diverges outside the frozen delta set (7): %+v vs %+v", m, naturalChoiceShape(c6), naturalChoiceShape(c7))
										} else if naturalChoiceShape(c6) != naturalChoiceShape(c7b) {
											t.Fatalf("cell %+v diverges outside the frozen delta set (7b): %+v vs %+v", m, naturalChoiceShape(c6), naturalChoiceShape(c7b))
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	// Grid shape and exact flip counts (13 energies × 9 capitals × 2 wip ×
	// 4 granaries × 2 bags × 3 denials × 2 stocked): the flip needs the
	// claim phase, a living actor, no denial, no stocked slot, a stocked
	// granary, and E >= 4 — energies {4..12} (9) × capitals (9) × wip (2) ×
	// granary {1,3,8} (3) = 486 cells per arm. The stocked dimension gathers
	// in both policies (no flip), the gather-denial path is verbatim v6 (no
	// flip), and the bag decision is verbatim v6 (no flip).
	if cells != 11232 || structuralErrors != 13*9*2*4*2 || bagCells != 13*9*2*4*3*2 || claimCells != 13*9*2*4*2*2 {
		t.Fatalf("grid shape drifted: cells=%d structural=%d bag=%d claim=%d", cells, structuralErrors, bagCells, claimCells)
	}
	if flips["7"] == 0 || flips["7b"] == 0 {
		t.Fatalf("flip family never fired: the grid is vacuous (%+v)", flips)
	}
	if flips["7"] != 486 || flips["7b"] != 486 {
		t.Fatalf("flip counts drifted: 7=%d 7b=%d, want exactly 486 each", flips["7"], flips["7b"])
	}
	t.Logf("differential grid: %d cells, %d structural rejections, %d bag cells, %d claim cells, "+
		"flips 7=%d 7b=%d (EatStored->Provision / EatStored->Wait at E>=4 ∧ granary>=1 claim cells); all other cells identical",
		cells, structuralErrors, bagCells, claimCells, flips["7"], flips["7b"])
}

// TestNatural7ProvisionFloorTruthTable pins the two claim floors exactly:
// the provision floor (E = 3 refused, E >= 4 chosen, granary 0 refused) and
// the eat-stored floor (E <= 3 with no draw yet draws the bank; E >= 4 never
// draws it), plus the denial path staying verbatim v6 (bank at any energy,
// no provision) and the bag decision staying verbatim v6 (3/3/4/4).
func TestNatural7ProvisionFloorTruthTable(t *testing.T) {
	b7, ref7 := natural7Fixture(t, 1)
	b7b, ref7b := natural7bFixture(t, 1)
	claim := func(b *strategy.NaturalBound, ref strategy.NaturalRef, energy, granary, lastMeal int64) strategy.NaturalChoice {
		t.Helper()
		o := naturalObservation(t, 1, ref, 5)
		o.Energy = sim.IntegerValue(energy)
		o.GranaryStock = sim.IntegerValue(granary)
		o.LastStoredMealHour = sim.IntegerValue(lastMeal)
		c, err := b.Evaluate7(o)
		if b.Policy().Ref.Version == 8 {
			c, err = b.Evaluate7b(o)
		}
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// The provision floor on natural/7: E = 3 is refused (the survival-floor
	// bank draw keeps the body), E >= 4 is chosen, granary 0 is refused.
	for _, tc := range []struct {
		name            string
		energy, granary int64
		want            strategy.NaturalAction
	}{
		{"E=3 granary=1 draws the bank", 3, 1, strategy.NaturalEatStored},
		{"E=3 granary=8 draws the bank", 3, 8, strategy.NaturalEatStored},
		{"E=4 granary=1 provisions", 4, 1, strategy.NaturalProvision},
		{"E=12 granary=1 provisions", 12, 1, strategy.NaturalProvision},
		{"E=4 granary=0 waits", 4, 0, strategy.NaturalWait},
		{"E=12 granary=0 waits", 12, 0, strategy.NaturalWait},
	} {
		if got := claim(b7, ref7, tc.energy, tc.granary, -1).Kind; got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	// The eat-stored floor on natural/7: the bank is food only at the
	// survival floor; above it the same cell provisions instead, and an
	// already-drawn hour waits (the draw guard is verbatim v6).
	if got := claim(b7, ref7, 2, 8, -1).Kind; got != strategy.NaturalEatStored {
		t.Fatalf("E=2 granary=8 must draw the bank: %v", got)
	}
	if got := claim(b7, ref7, 4, 8, -1).Kind; got != strategy.NaturalProvision {
		t.Fatalf("E=4 granary=8 must provision, not draw: %v", got)
	}
	if got := claim(b7, ref7, 3, 1, 5).Kind; got != strategy.NaturalWait {
		t.Fatalf("E=3 with this hour's meal already drawn must wait: %v", got)
	}
	// The provision clause carries no policy-side draw guard (the world's
	// rule 515 owns the once-per-hour discipline), exactly the design text.
	if got := claim(b7, ref7, 4, 1, 5).Kind; got != strategy.NaturalProvision {
		t.Fatalf("provision is the design's two-condition clause: %v", got)
	}
	// The denied-gather fallback is verbatim v6: the bank at ANY energy, no
	// provision, rest with an empty granary.
	denied := func(b *strategy.NaturalBound, ref strategy.NaturalRef, energy, granary int64) strategy.NaturalChoice {
		t.Helper()
		o := naturalObservation(t, 1, ref, 5)
		o.Energy = sim.IntegerValue(energy)
		o.GranaryStock = sim.IntegerValue(granary)
		o.LastDenial = strategy.NaturalGatherDenied
		var c strategy.NaturalChoice
		var err error
		if b.Policy().Ref.Version == 8 {
			c, err = b.Evaluate7b(o)
		} else {
			c, err = b.Evaluate7(o)
		}
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	for _, arm := range []struct {
		b   *strategy.NaturalBound
		ref strategy.NaturalRef
	}{{b7, ref7}, {b7b, ref7b}} {
		if c := denied(arm.b, arm.ref, 8, 1); c.Kind != strategy.NaturalEatStored || c.Evaluated != 1 {
			t.Fatalf("denied gatherer at E=8 granary=1 must eat stored verbatim v6: %+v", c)
		}
		if c := denied(arm.b, arm.ref, 1, 0); c.Kind != strategy.NaturalRest || c.RestDuration != strategy.NaturalRestDuration {
			t.Fatalf("denied gatherer with an empty granary must rest: %+v", c)
		}
		if c := denied(arm.b, arm.ref, 8, 0); c.Kind != strategy.NaturalRest {
			t.Fatalf("denied gatherer at E=8 granary=0 must rest: %+v", c)
		}
	}
	// The banking floors on natural/7b with the conversion disabled: the
	// bank is refused above the survival floor (the floor still bites) and
	// still available at it.
	for _, tc := range []struct {
		name            string
		energy, granary int64
		want            strategy.NaturalAction
	}{
		{"7b E=3 granary=1 draws the bank", 3, 1, strategy.NaturalEatStored},
		{"7b E=1 granary=8 draws the bank", 1, 8, strategy.NaturalEatStored},
		{"7b E=4 granary=1 refuses the bank", 4, 1, strategy.NaturalWait},
		{"7b E=12 granary=8 refuses the bank", 12, 8, strategy.NaturalWait},
		{"7b E=4 granary=0 waits", 4, 0, strategy.NaturalWait},
	} {
		if got := claim(b7b, ref7b, tc.energy, tc.granary, -1).Kind; got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	// The bag decision is verbatim v6 in both arms (constants 3/3/4/4),
	// including the build-denial leg.
	for _, arm := range []struct {
		b   *strategy.NaturalBound
		ref strategy.NaturalRef
	}{{b7, ref7}, {b7b, ref7b}} {
		held := func(energy, capital, wip, granary int64, denial strategy.NaturalDenial) strategy.NaturalChoice {
			t.Helper()
			o := naturalHeldAtMargins(t, 1, arm.ref, 5)
			o.Energy = sim.IntegerValue(energy)
			o.Capital = sim.IntegerValue(capital)
			o.Wip = sim.IntegerValue(wip)
			o.GranaryStock = sim.IntegerValue(granary)
			o.LastDenial = denial
			var c strategy.NaturalChoice
			var err error
			if arm.b.Policy().Ref.Version == 8 {
				c, err = arm.b.Evaluate7b(o)
			} else {
				c, err = arm.b.Evaluate7(o)
			}
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
		for _, tc := range []struct {
			name                          string
			energy, capital, wip, granary int64
			denial                        strategy.NaturalDenial
			want                          strategy.NaturalAction
		}{
			{"the exact TStart miss eats", 3, 0, 0, 0, strategy.NaturalNoDenial, strategy.NaturalEat},
			{"the wip finish leg builds at E=3", 3, 0, 1, 0, strategy.NaturalNoDenial, strategy.NaturalBuild},
			{"the start leg builds at E=4", 4, 0, 0, 0, strategy.NaturalNoDenial, strategy.NaturalBuild},
			{"the fed larder leaves E=2 eating", 2, 0, 0, 1, strategy.NaturalNoDenial, strategy.NaturalEat},
			{"the fed larder builds at E=3", 3, 0, 0, 1, strategy.NaturalNoDenial, strategy.NaturalBuild},
			{"the descent family builds", 5, 0, 1, 0, strategy.NaturalNoDenial, strategy.NaturalBuild},
			{"saturated eats", 12, 7, 1, 8, strategy.NaturalNoDenial, strategy.NaturalEat},
			{"build-denied eats the held unit", 4, 0, 0, 0, strategy.NaturalBuildDenied, strategy.NaturalEat},
		} {
			got := held(tc.energy, tc.capital, tc.wip, tc.granary, tc.denial).Kind
			if got != tc.want {
				t.Fatalf("%v (%s): got %v, want %v", arm.ref, tc.name, got, tc.want)
			}
			if want := naturalV6ExpectedBag(tc.energy, tc.capital, tc.wip, tc.granary, tc.denial); got != want {
				t.Fatalf("%v bag decision diverges from the frozen v6 rule: %v vs %v", arm.ref, got, want)
			}
		}
	}
}

// TestNatural7bAblationNeverProvisionsFloorsBite walks the full grid with
// the natural/7b bound and proves the predeclared mechanism-necessity
// property: the provision affordance is NEVER emitted in any cell, while the
// banking floors still bite (the bank is refused above the survival floor in
// every claim cell where v6 would draw). The design's ladder step is then
// expressible at one cell: (v6,n/6) draws, (v6,n/7b) waits, (v6,n/7)
// provisions.
func TestNatural7bAblationNeverProvisionsFloorsBite(t *testing.T) {
	b6, _, b7b, _, _, ref7b := naturalV6V7V7bBounds(t)
	const hour = 5
	base6 := naturalObservation(t, 1, strategy.FrozenNaturalPolicyV6().Ref, hour)
	base7b := naturalObservation(t, 1, ref7b, hour)
	var cells, drawsRefused, bankDraws int
	for energy := int64(0); energy <= 12; energy++ {
		for capital := int64(0); capital <= 8; capital++ {
			for wip := int64(0); wip <= 1; wip++ {
				for _, granary := range []int64{0, 1, 3, 8} {
					for bag := int64(0); bag <= 1; bag++ {
						for _, denial := range []strategy.NaturalDenial{strategy.NaturalNoDenial, strategy.NaturalGatherDenied, strategy.NaturalBuildDenied} {
							for _, stocked := range []bool{false, true} {
								o6, o7b := base6, base7b
								o6.HomeSlots = append([]strategy.NaturalSlotView(nil), o6.HomeSlots...)
								o7b.HomeSlots = append([]strategy.NaturalSlotView(nil), o7b.HomeSlots...)
								for _, o := range []*strategy.NaturalObservation{&o6, &o7b} {
									o.Energy = sim.IntegerValue(energy)
									o.Capital = sim.IntegerValue(capital)
									o.Wip = sim.IntegerValue(wip)
									o.GranaryStock = sim.IntegerValue(granary)
									o.BagUnits = sim.IntegerValue(bag)
									o.LastDenial = denial
									if stocked {
										o.HomeSlots[0].Stock = sim.IntegerValue(1)
									}
								}
								c6, err6 := b6.EvaluateV6(o6)
								c7b, err7b := b7b.Evaluate7b(o7b)
								if (err6 == nil) != (err7b == nil) {
									t.Fatalf("cell e=%d k=%d wip=%d g=%d bag=%d denial=%d stocked=%v: structural divergence", energy, capital, wip, granary, bag, denial, stocked)
								}
								if err7b != nil {
									continue
								}
								cells++
								if c7b.Kind == strategy.NaturalProvision {
									t.Fatalf("natural/7b emitted a provision at e=%d k=%d wip=%d g=%d bag=%d denial=%d stocked=%v", energy, capital, wip, granary, bag, denial, stocked)
								}
								if bag == 0 && energy > 0 && denial == strategy.NaturalNoDenial && !stocked && granary >= 1 {
									if energy >= 4 {
										if c6.Kind != strategy.NaturalEatStored || c7b.Kind != strategy.NaturalWait {
											t.Fatalf("floor must bite at e=%d g=%d: v6=%v 7b=%v", energy, granary, c6.Kind, c7b.Kind)
										}
										drawsRefused++
									} else {
										if c7b.Kind != strategy.NaturalEatStored {
											t.Fatalf("survival-floor draw must survive at e=%d g=%d: %v", energy, granary, c7b.Kind)
										}
										bankDraws++
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if drawsRefused != 486 || bankDraws != 162 { // E in {4..12} vs {1..3} × 9 capitals × 2 wip × 3 stocked granaries
		t.Fatalf("floor-cell counts drifted: refused=%d draws=%d, want 486/162", drawsRefused, bankDraws)
	}
	if cells != 9360 { // 11232 minus the 1872 structurally-refused build-denial cells
		t.Fatalf("grid shape drifted: %d cells", cells)
	}
	// The ladder step at one cell: the same observation through the three
	// policies — v6 draws the bank, 7b waits (floors only), 7 provisions
	// (the predicted flip step is exactly the conversion).
	p7 := strategy.FrozenNaturalPolicy7()
	reg := strategy.NewNaturalRegistry(naturalTestPatches)
	for _, p := range []strategy.NaturalPolicy{strategy.FrozenNaturalPolicyV6(), p7, strategy.FrozenNaturalPolicy7b()} {
		if err := reg.RegisterFrozenNatural(p); err != nil {
			t.Fatal(err)
		}
	}
	ladder := func(p strategy.NaturalPolicy) strategy.NaturalChoice {
		t.Helper()
		b, err := reg.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref})
		if err != nil {
			t.Fatal(err)
		}
		o := naturalObservation(t, 1, p.Ref, 5)
		o.Energy = sim.IntegerValue(5)
		o.GranaryStock = sim.IntegerValue(1)
		var c strategy.NaturalChoice
		switch p.Ref.Version {
		case 6:
			c, err = b.EvaluateV6(o)
		case 7:
			c, err = b.Evaluate7(o)
		default:
			c, err = b.Evaluate7b(o)
		}
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if got := ladder(strategy.FrozenNaturalPolicyV6()); got.Kind != strategy.NaturalEatStored {
		t.Fatalf("ladder (v6,n/6): %v", got.Kind)
	}
	if got := ladder(strategy.FrozenNaturalPolicy7b()); got.Kind != strategy.NaturalWait {
		t.Fatalf("ladder (v6,n/7b): %v", got.Kind)
	}
	if got := ladder(p7); got.Kind != strategy.NaturalProvision {
		t.Fatalf("ladder (v6,n/7): %v", got.Kind)
	}
}

// TestNatural7BirthMarginsVerbatimV6 pins the birth cascades of both 7
// identities against the frozen v6 cascades: filing and accepting at own
// energy >= 4, every other margin, the reported-claims margins, the kin
// clause, and the addressee rotation verbatim.
func TestNatural7BirthMarginsVerbatimV6(t *testing.T) {
	refs := map[uint32]strategy.NaturalRef{
		6: strategy.FrozenNaturalPolicyV6().Ref, 7: strategy.FrozenNaturalPolicy7().Ref, 8: strategy.FrozenNaturalPolicy7b().Ref,
	}
	bound := map[uint32]*strategy.NaturalBound{}
	reg := strategy.NewNaturalRegistry(naturalTestPatches)
	for _, p := range []strategy.NaturalPolicy{strategy.FrozenNaturalPolicyV6(), strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b()} {
		if err := reg.RegisterFrozenNatural(p); err != nil {
			t.Fatal(err)
		}
	}
	for v, ref := range refs {
		b, err := reg.Bind(strategy.NaturalBinding{Actor: 2, Ref: ref})
		if err != nil {
			t.Fatal(err)
		}
		bound[v] = b
	}
	request := func(v uint32, energy int64) strategy.NaturalRequestChoice {
		t.Helper()
		view := naturalRequestView(t, 2, refs[v], 5)
		view.Energy = sim.IntegerValue(energy)
		var c strategy.NaturalRequestChoice
		var err error
		switch v {
		case 6:
			c, err = bound[v].EvaluateBirthRequestV6(view)
		case 7:
			c, err = bound[v].EvaluateBirthRequest7(view)
		default:
			c, err = bound[v].EvaluateBirthRequest7b(view)
		}
		if err != nil {
			t.Fatal(err)
		}
		c.Ref = strategy.NaturalRef{}
		return c
	}
	// Filing across own energy: E = 3 waits (the frozen v6 floor), E >= 4
	// files; 7 and 7b are identical to v6 in every cell.
	for _, energy := range []int64{2, 3, 4, 8} {
		c6, c7, c7b := request(6, energy), request(7, energy), request(8, energy)
		if c6 != c7 || c6 != c7b {
			t.Fatalf("filing at E=%d diverged: v6=%+v 7=%+v 7b=%+v", energy, c6, c7, c7b)
		}
		want := strategy.NaturalRequestWait
		if energy >= 4 {
			want = strategy.NaturalRequestFile
		}
		if c6.Kind != want {
			t.Fatalf("filing at E=%d: %v, want %v", energy, c6.Kind, want)
		}
		if want == strategy.NaturalRequestFile && (c6.Addressee != 3 || c6.ReportedCapital != 2 ||
			c6.ReportedGranary != 3 || c6.Evaluated != 7) {
			t.Fatalf("filing at E=%d drifted: %+v", energy, c6)
		}
	}
	// Accepting across own energy with the typed refusal reason: E = 2/3
	// refuse with ConsentOwn, E >= 4 accepts; 7 and 7b identical to v6.
	reply := func(v uint32, energy int64) strategy.NaturalReplyChoice {
		t.Helper()
		view := naturalReplyView(2, 1, refs[v], 5)
		view.Energy = sim.IntegerValue(energy)
		var c strategy.NaturalReplyChoice
		var err error
		switch v {
		case 6:
			c, err = bound[v].EvaluateBirthReplyV6(view)
		case 7:
			c, err = bound[v].EvaluateBirthReply7(view)
		default:
			c, err = bound[v].EvaluateBirthReply7b(view)
		}
		if err != nil {
			t.Fatal(err)
		}
		c.Ref = strategy.NaturalRef{}
		return c
	}
	for _, energy := range []int64{2, 3, 4, 8} {
		c6, c7, c7b := reply(6, energy), reply(7, energy), reply(8, energy)
		if c6 != c7 || c6 != c7b {
			t.Fatalf("reply at E=%d diverged: v6=%+v 7=%+v 7b=%+v", energy, c6, c7, c7b)
		}
		want := strategy.NaturalReplyAccept
		reason := strategy.NaturalReplyReasonNone
		if energy <= 3 {
			want, reason = strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn
		}
		if c6.Kind != want || c6.Reason != reason {
			t.Fatalf("reply at E=%d: %v/%v, want %v/%v", energy, c6.Kind, c6.Reason, want, reason)
		}
	}
	// The reported-claims margins and the kin clause are verbatim: at own
	// E = 4 the refusal reasons sit exactly where v6's do, for both 7 arms.
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
		for _, v := range []uint32{7, 8} {
			view := naturalReplyView(2, 1, refs[v], 5)
			view.Energy = sim.IntegerValue(4)
			view.Pending[0].ReportedCapital = tc.reportedCapital
			view.Pending[0].ReportedGranary = tc.reportedGranary
			view.Pending[0].KinRelated = tc.kinRelated
			view.Pending[0].KinDistance = tc.kinDistance
			c, err := bound[v].EvaluateBirthReply7(view)
			if v == 8 {
				c, err = bound[v].EvaluateBirthReply7b(view)
			}
			if err != nil || c.Kind != tc.want || c.Reason != tc.reason || c.Requester != 1 {
				t.Fatalf("v%d %s: %+v %v", v, tc.name, c, err)
			}
		}
	}
	// The wraparound addressee rotation is verbatim: actor 8 wraps to 1.
	for _, v := range []uint32{7, 8} {
		b8, err := reg.Bind(strategy.NaturalBinding{Actor: 8, Ref: refs[v]})
		if err != nil {
			t.Fatal(err)
		}
		view := naturalRequestView(t, 8, refs[v], 5)
		var c strategy.NaturalRequestChoice
		if v == 7 {
			c, err = b8.EvaluateBirthRequest7(view)
		} else {
			c, err = b8.EvaluateBirthRequest7b(view)
		}
		if err != nil || c.Kind != strategy.NaturalRequestFile || c.Addressee != 1 {
			t.Fatalf("v%d wraparound addressee: %+v %v", v, c, err)
		}
	}
	// The shared guard rails are verbatim: one request per actor-hour, one
	// reply per actor-hour, and dead actors stay silent.
	view := naturalRequestView(t, 2, refs[7], 5)
	view.OwnRequest = strategy.NaturalRequestGuard{Status: strategy.NaturalRequestPending, LastRequestHour: 4}
	if c, err := bound[7].EvaluateBirthRequest7(view); err != nil || c.Kind != strategy.NaturalRequestWait {
		t.Fatalf("pending request must resolve first: %+v %v", c, err)
	}
	guard := naturalReplyView(2, 1, refs[8], 5)
	guard.OwnLastReplyHour = 5
	if c, err := bound[8].EvaluateBirthReply7b(guard); err != nil || c.Kind != strategy.NaturalReplyNone {
		t.Fatalf("7b second reply in one hour: %+v %v", c, err)
	}
	dead := naturalRequestView(t, 2, refs[7], 5)
	dead.Energy = sim.IntegerValue(0)
	if c, err := bound[7].EvaluateBirthRequest7(dead); err != nil || c.Kind != strategy.NaturalRequestWait || c.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("7 dead proposer: %+v %v", c, err)
	}
}

// TestNatural7PrivacyFieldSetsParityWithV5 reflects the shared observation,
// view, and choice types and asserts the exact frozen v5 field lists: the 7
// lane introduces no new type and no new observation field (the provision
// choice is the one new action constant), and therefore no channel for slot
// tenure, cultivation assignment, debt carries, spoilage counters, allocator
// state, R, streams, other actors' state, death hours, or claim-vs-truth
// divergence — the 7 cascades evaluate these very types.
func TestNatural7PrivacyFieldSetsParityWithV5(t *testing.T) {
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
			"Debt", "BasalDebt", "YieldDebt", "WearDebt", "SpoilDebt", "Spoil", "Spoiled", "Provisioned", "Allocator", "Seed", "R",
			"RegenBudget", "DeathHour", "DiedHour", "Stream", "ClaimDelta", "ClaimMatchesTruth"}
		for _, forbidden := range forbiddenNames {
			if _, present := typ.FieldByName(forbidden); present {
				t.Fatalf("%s leaked %s", typ, forbidden)
			}
		}
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			for _, word := range []string{"Tenure", "Owner", "Mode", "Debt", "Spoil", "Provision", "Allocator", "Seed", "Death", "Died", "Stream"} {
				if strings.Contains(name, word) {
					t.Fatalf("%s field %s carries privileged %s data", typ, name, word)
				}
			}
		}
	}
	// A 7 or 7b bound never evaluates another actor's observation, request
	// view, or reply view.
	b1, ref7 := natural7Fixture(t, 1)
	b1b, ref7b := natural7bFixture(t, 1)
	b2, _ := natural7Fixture(t, 2)
	b2b, _ := natural7bFixture(t, 2)
	obs1 := naturalObservation(t, 1, ref7, 5)
	if _, err := b2.Evaluate7(obs1); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 7 bound evaluated another actor's claim view")
	}
	obs1b := naturalObservation(t, 1, ref7b, 5)
	if _, err := b2b.Evaluate7b(obs1b); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 7b bound evaluated another actor's claim view")
	}
	if _, err := b2.EvaluateBirthRequest7(naturalRequestView(t, 1, ref7, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 7 bound evaluated another actor's request view")
	}
	if _, err := b2b.EvaluateBirthRequest7b(naturalRequestView(t, 1, ref7b, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 7b bound evaluated another actor's request view")
	}
	if _, err := b2.EvaluateBirthReply7(naturalReplyView(1, 3, ref7, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 7 bound evaluated another actor's reply view")
	}
	if _, err := b2b.EvaluateBirthReply7b(naturalReplyView(1, 3, ref7b, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 7b bound evaluated another actor's reply view")
	}
	for _, tc := range []struct {
		b      *strategy.NaturalBound
		frozen strategy.NaturalPolicy
	}{{b1, strategy.FrozenNaturalPolicy7()}, {b1b, strategy.FrozenNaturalPolicy7b()}} {
		if tc.b.Policy() != tc.frozen {
			t.Fatalf("accessors exposed non-frozen content: %+v", tc.b.Policy())
		}
		mutated := tc.b.Policy()
		mutated.Ref.ID = "mutated"
		if tc.b.Policy() != tc.frozen {
			t.Fatal("returned policy mutated bound content")
		}
	}
}

// TestNatural7BudgetExhaustion pins the shared budget semantics for both 7
// identities: the frozen ceiling is 32/32, same-ref budget variants register,
// the bag decision and the single-target claim legs only require a nonzero
// budget, and every list input degrades all-or-nothing to the typed
// NaturalBudgetExhausted fallback — never a prefix.
func TestNatural7BudgetExhaustion(t *testing.T) {
	for _, v := range []uint32{7, 8} {
		p := strategy.FrozenNaturalPolicy7()
		if v == 8 {
			p = strategy.FrozenNaturalPolicy7b()
		}
		p.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
		limited := strategy.NewNaturalRegistry(naturalTestPatches)
		if err := limited.RegisterFrozenNatural(p); err != nil {
			t.Fatal(err)
		}
		b, err := limited.Bind(strategy.NaturalBinding{Actor: 1, Ref: p.Ref})
		if err != nil {
			t.Fatal(err)
		}
		evaluate := func(o strategy.NaturalObservation) strategy.NaturalChoice {
			t.Helper()
			var c strategy.NaturalChoice
			var err error
			if v == 8 {
				c, err = b.Evaluate7b(o)
			} else {
				c, err = b.Evaluate7(o)
			}
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
		// Two stocked slots against a one-slot budget: the whole candidate
		// set is declined, never a sorted prefix.
		obs := naturalStocked(naturalObservation(t, 1, p.Ref, 5), 0, 1)
		if choice := evaluate(obs); choice != (strategy.NaturalChoice{Kind: strategy.NaturalWait, ObservedVersion: 9,
			Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
			t.Fatalf("v%d all-or-nothing claim budget: %+v", v, choice)
		}
		if choice := evaluate(naturalStocked(naturalObservation(t, 1, p.Ref, 5), 4)); choice.Kind != strategy.NaturalGather ||
			choice.TargetSlot != 2005 || choice.Evaluated != 1 {
			t.Fatalf("v%d bounded candidate set: %+v", v, choice)
		}
		// The claim bank legs need no list budget: at E = 4 with a stocked
		// granary and no visible slot, 7 provisions and 7b waits (floors, not
		// budget), both under the 1/1 variant.
		bank := naturalObservation(t, 1, p.Ref, 5)
		bank.Energy = sim.IntegerValue(4)
		bank.GranaryStock = sim.IntegerValue(1)
		choice := evaluate(bank)
		if v == 7 && (choice.Kind != strategy.NaturalProvision || choice.Fallback != strategy.NaturalNoFallback) {
			t.Fatalf("v7 provision leg under a 1/1 budget: %+v", choice)
		}
		if v == 8 && (choice.Kind != strategy.NaturalWait || choice.Fallback != strategy.NaturalNoCandidate) {
			t.Fatalf("7b floor refusal under a 1/1 budget: %+v", choice)
		}
		// The bag decision needs only a nonzero budget, verbatim v5/v6
		// semantics: at E = 5 it builds, and the denial fallback eats.
		held := naturalHeldAtMargins(t, 1, p.Ref, 5)
		held.Energy = sim.IntegerValue(5)
		if choice := evaluate(held); choice.Kind != strategy.NaturalBuild || choice.Fallback != strategy.NaturalNoFallback {
			t.Fatalf("v%d bag decision under a 1/1 budget: %+v", v, choice)
		}
		held.LastDenial = strategy.NaturalBuildDenied
		if choice := evaluate(held); choice.Kind != strategy.NaturalEat {
			t.Fatalf("v%d denial fallback under a 1/1 budget: %+v", v, choice)
		}
		// Two same-patch mates against a one-mate request budget: no filing.
		request := naturalRequestView(t, 1, p.Ref, 5)
		request.Energy = sim.IntegerValue(4)
		choice2, err := b.EvaluateBirthRequest7(request)
		if v == 8 {
			choice2, err = b.EvaluateBirthRequest7b(request)
		}
		if err != nil || choice2 != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestWait,
			ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
			t.Fatalf("v%d all-or-nothing request budget: %+v %v", v, choice2, err)
		}
		request.PatchMates = []sim.EntityID{2}
		if v == 7 {
			if c, err := b.EvaluateBirthRequest7(request); err != nil || c.Kind != strategy.NaturalRequestFile || c.Addressee != 2 {
				t.Fatalf("bounded v7 request set: %+v %v", c, err)
			}
		} else {
			if c, err := b.EvaluateBirthRequest7b(request); err != nil || c.Kind != strategy.NaturalRequestFile || c.Addressee != 2 {
				t.Fatalf("bounded 7b request set: %+v %v", c, err)
			}
		}
		// Two pending requests against a one-reply budget: no reply at all.
		reply := naturalReplyView(1, 2, p.Ref, 5)
		reply.Pending = append(reply.Pending, strategy.NaturalPendingView{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
		choice3, err := b.EvaluateBirthReply7(reply)
		if v == 8 {
			choice3, err = b.EvaluateBirthReply7b(reply)
		}
		if err != nil || choice3 != (strategy.NaturalReplyChoice{Kind: strategy.NaturalReplyNone,
			ObservedVersion: 9, Ref: p.Ref, Fallback: strategy.NaturalBudgetExhausted}) {
			t.Fatalf("v%d all-or-nothing reply budget: %+v %v", v, choice3, err)
		}
	}
	// The frozen 32/32 budget covers every maximal legal set for both
	// identities: all eight stocked slots, a 32-mate roster, and a
	// 32-pending list evaluate without exhaustion; the 33rd entry is a typed
	// budget event.
	for _, v := range []uint32{7, 8} {
		var b *strategy.NaturalBound
		var ref strategy.NaturalRef
		if v == 7 {
			b, ref = natural7Fixture(t, 1)
		} else {
			b, ref = natural7bFixture(t, 1)
		}
		all := naturalObservation(t, 1, ref, 5)
		for i := range all.HomeSlots {
			all.HomeSlots[i].Stock = sim.IntegerValue(1)
		}
		var c strategy.NaturalChoice
		var err error
		if v == 7 {
			c, err = b.Evaluate7(all)
		} else {
			c, err = b.Evaluate7b(all)
		}
		if err != nil || c.Kind != strategy.NaturalGather || c.TargetSlot != 2001 || c.Evaluated != 8 || c.Fallback != strategy.NaturalNoFallback {
			t.Fatalf("v%d frozen budget must never truncate the legal slot set: %+v %v", v, c, err)
		}
		big := naturalRequestView(t, 1, ref, 5)
		big.Energy = sim.IntegerValue(4)
		for id := sim.EntityID(2017); len(big.PatchMates) < 32; id++ {
			big.PatchMates = append(big.PatchMates, id)
		}
		var fc strategy.NaturalRequestChoice
		if v == 7 {
			fc, err = b.EvaluateBirthRequest7(big)
		} else {
			fc, err = b.EvaluateBirthRequest7b(big)
		}
		if err != nil || fc.Kind != strategy.NaturalRequestFile || fc.Addressee != 2 || fc.Evaluated != 32 || fc.Fallback != strategy.NaturalNoFallback {
			t.Fatalf("v%d frozen budget must never truncate the legal roster: %+v %v", v, fc, err)
		}
		big.PatchMates = append(big.PatchMates, 2049)
		if v == 7 {
			fc, err = b.EvaluateBirthRequest7(big)
		} else {
			fc, err = b.EvaluateBirthRequest7b(big)
		}
		if err != nil || fc != (strategy.NaturalRequestChoice{Kind: strategy.NaturalRequestWait,
			ObservedVersion: 9, Ref: ref, Fallback: strategy.NaturalBudgetExhausted}) {
			t.Fatalf("v%d over-budget roster must be a typed budget event: %+v %v", v, fc, err)
		}
		full := naturalReplyView(1, 2, ref, 5)
		for id := sim.EntityID(3); id <= 8; id++ {
			full.Pending = append(full.Pending, strategy.NaturalPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
		}
		for id := sim.EntityID(2017); len(full.Pending) < 32; id++ {
			full.Pending = append(full.Pending, strategy.NaturalPendingView{Requester: id, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3})
		}
		var rc strategy.NaturalReplyChoice
		if v == 7 {
			rc, err = b.EvaluateBirthReply7(full)
		} else {
			rc, err = b.EvaluateBirthReply7b(full)
		}
		if err != nil || rc.Kind != strategy.NaturalReplyAccept || rc.Requester != 2 || rc.Evaluated != 1 || rc.Fallback != strategy.NaturalNoFallback {
			t.Fatalf("v%d frozen budget must never truncate the legal pending set: %+v %v", v, rc, err)
		}
	}
	// A zero budget cannot register for either identity.
	zeroed7 := strategy.FrozenNaturalPolicy7()
	zeroed7.Budget.Candidates = 0
	zeroed7b := strategy.FrozenNaturalPolicy7b()
	zeroed7b.Budget.Evaluations = 0
	for _, p := range []strategy.NaturalPolicy{zeroed7, zeroed7b} {
		reg := strategy.NewNaturalRegistry(naturalTestPatches)
		if err := reg.RegisterFrozenNatural(p); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("zero budget registered: %+v %v", p, err)
		}
	}
}

// TestNatural7DeterminismAndOrderIndependence pins the memoryless 7/7b
// evaluation: shuffled slot, roster, and pending orders decide identically,
// and repeated evaluation of the same observation returns the same choice.
func TestNatural7DeterminismAndOrderIndependence(t *testing.T) {
	b7, ref7 := natural7Fixture(t, 1)
	b7b, ref7b := natural7bFixture(t, 1)
	// Claim: the lowest stocked slot ID wins regardless of input order, in
	// both arms.
	shuffled := naturalStocked(naturalObservation(t, 1, ref7, 5), 2, 0, 5)
	first, err := b7.Evaluate7(shuffled)
	if err != nil || first.Kind != strategy.NaturalGather || first.TargetSlot != 2001 || first.Evaluated != 3 {
		t.Fatalf("7 gather must take the lowest stocked slot: %+v %v", first, err)
	}
	shuffled7b := naturalStocked(naturalObservation(t, 1, ref7b, 5), 2, 0, 5)
	first7b, err := b7b.Evaluate7b(shuffled7b)
	if err != nil || first7b.Kind != strategy.NaturalGather || first7b.TargetSlot != 2001 {
		t.Fatalf("7b gather must take the lowest stocked slot: %+v %v", first7b, err)
	}
	reordered := naturalStocked(naturalObservation(t, 1, ref7, 5), 5, 2, 0)
	reordered.HomeSlots[3], reordered.HomeSlots[6] = reordered.HomeSlots[6], reordered.HomeSlots[3]
	if again, err := b7.Evaluate7(reordered); err != nil || naturalChoiceShape(again) != naturalChoiceShape(first) {
		t.Fatalf("7 shuffled slots must not change the choice: %+v %v", again, err)
	}
	// The provision choice is repeatable.
	provision := naturalObservation(t, 1, ref7, 5)
	provision.Energy = sim.IntegerValue(4)
	provision.GranaryStock = sim.IntegerValue(1)
	want := strategy.NaturalChoice{Kind: strategy.NaturalProvision, ObservedVersion: 9, Ref: ref7, Evaluated: 1}
	for i := 0; i < 3; i++ {
		if c, err := b7.Evaluate7(provision); err != nil || c != want {
			t.Fatalf("7 provision repeat %d: %+v %v", i, c, err)
		}
	}
	// The 7b floor refusal is repeatable.
	floor := naturalObservation(t, 1, ref7b, 5)
	floor.Energy = sim.IntegerValue(4)
	floor.GranaryStock = sim.IntegerValue(1)
	wantWait := strategy.NaturalChoice{Kind: strategy.NaturalWait, ObservedVersion: 9, Ref: ref7b, Fallback: strategy.NaturalNoCandidate}
	for i := 0; i < 3; i++ {
		if c, err := b7b.Evaluate7b(floor); err != nil || c != wantWait {
			t.Fatalf("7b floor refusal repeat %d: %+v %v", i, c, err)
		}
	}
	// Bag decision and stored-meal guard are repeatable (v6 rule).
	held := naturalHeldAtMargins(t, 1, ref7, 5)
	held.Energy = sim.IntegerValue(4) // the exact re-pioneer margin: builds in v6
	wantBuild := strategy.NaturalChoice{Kind: strategy.NaturalBuild, ObservedVersion: 9, Ref: ref7, Evaluated: 1}
	for i := 0; i < 3; i++ {
		if c, err := b7.Evaluate7(held); err != nil || c != wantBuild {
			t.Fatalf("7 bag decision repeat %d: %+v %v", i, c, err)
		}
	}
	meal := naturalObservation(t, 1, ref7b, 5)
	meal.Energy = sim.IntegerValue(3)
	meal.GranaryStock = sim.IntegerValue(2)
	mealChoice := strategy.NaturalChoice{Kind: strategy.NaturalEatStored, ObservedVersion: 9, Ref: ref7b, Evaluated: 1}
	for i := 0; i < 3; i++ {
		if c, err := b7b.Evaluate7b(meal); err != nil || c != mealChoice {
			t.Fatalf("7b stored-meal repeat %d: %+v %v", i, c, err)
		}
	}
	// Request: a shuffled roster files the same addressee with the same
	// claims and the same evaluated count.
	request := naturalRequestView(t, 1, ref7, 5)
	request.Energy = sim.IntegerValue(4)
	file, err := b7.EvaluateBirthRequest7(request)
	if err != nil || file.Kind != strategy.NaturalRequestFile || file.Addressee != 2 ||
		file.ReportedCapital != 2 || file.ReportedGranary != 3 || file.Evaluated != 7 {
		t.Fatalf("7 filing: %+v %v", file, err)
	}
	descending := naturalRequestView(t, 1, ref7, 5)
	descending.Energy = sim.IntegerValue(4)
	sort.Slice(descending.PatchMates, func(i, j int) bool { return descending.PatchMates[i] > descending.PatchMates[j] })
	if again, err := b7.EvaluateBirthRequest7(descending); err != nil || again != file {
		t.Fatalf("7 shuffled roster must not change the filing: %+v %v", again, err)
	}
	// Reply: shuffled pendings accept the first acceptable in ascending
	// requester order, identically.
	multi := naturalReplyView(1, 3, ref7b, 5)
	multi.Pending = []strategy.NaturalPendingView{
		{Requester: 3, Addressee: 1, Hour: 5, ReportedCapital: 1, ReportedGranary: 3}, // claims fail
		{Requester: 5, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3}, // acceptable
		{Requester: 2017, Addressee: 1, Hour: 5, ReportedCapital: 2, ReportedGranary: 3},
	}
	accepted, err := b7b.EvaluateBirthReply7b(multi)
	if err != nil || accepted.Kind != strategy.NaturalReplyAccept || accepted.Requester != 5 || accepted.Evaluated != 2 {
		t.Fatalf("7b first acceptable pending must win: %+v %v", accepted, err)
	}
	reversed := multi
	reversed.Pending = []strategy.NaturalPendingView{multi.Pending[2], multi.Pending[1], multi.Pending[0]}
	if again, err := b7b.EvaluateBirthReply7b(reversed); err != nil || again != accepted {
		t.Fatalf("7b input order must not change the reply: %+v %v", again, err)
	}
}

// TestNatural7Format7And8Codec pins the format-7 and format-8 wires:
// identical untouched AWNP framing, canonical bytes, fingerprint identity,
// cross-ref verification failures, the pre-extension {5,6} byte pins, and
// the frozen v5 codec continuing to refuse the new content.
func TestNatural7Format7And8Codec(t *testing.T) {
	p5, p6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	p7, p7b := strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b()
	const frozenWireLen = 4 + 4 + 1 + len("natural") + 4 + 1 + 1
	wires := map[uint32][]byte{}
	for _, p := range []strategy.NaturalPolicy{p7, p7b} {
		wire, err := strategy.EncodeFrozenNaturalPolicy(p)
		if err != nil {
			t.Fatal(err)
		}
		wires[p.Ref.Version] = wire
		if len(wire) != frozenWireLen {
			t.Fatalf("natural/%d wire length %d, want %d: %x", p.Ref.Version, len(wire), frozenWireLen, wire)
		}
		formatByte := byte(7)
		if p.Ref.Version == 8 {
			formatByte = 8
		}
		if !bytes.Equal(wire[:4], []byte("AWNP")) || wire[4] != 0 || wire[5] != 0 || wire[6] != 0 || wire[7] != formatByte {
			t.Fatalf("natural/%d wire magic/format drifted: %x", p.Ref.Version, wire[:8])
		}
		again, err := strategy.EncodeFrozenNaturalPolicy(p)
		if err != nil || !bytes.Equal(wire, again) {
			t.Fatalf("natural/%d encoding is not deterministic: %x %x %v", p.Ref.Version, wire, again, err)
		}
		if got, err := strategy.DecodeFrozenNaturalPolicy(wire); err != nil || got != p {
			t.Fatalf("natural/%d roundtrip: %+v %v", p.Ref.Version, got, err)
		}
		if err := strategy.VerifyFrozenNaturalPolicy(wire, p); err != nil {
			t.Fatalf("frozen natural/%d wire must verify: %v", p.Ref.Version, err)
		}
		if sum, err := strategy.FrozenNaturalPolicyFingerprint(p); err != nil || sum != sha256.Sum256(wire) {
			t.Fatalf("natural/%d fingerprint: %v %v", p.Ref.Version, sum, err)
		}
		// Every single-byte mutation either fails decoding or fails ref-set
		// verification against the executable's frozen policy.
		for i := range wire {
			bad := append([]byte(nil), wire...)
			bad[i] ^= 0xff
			if _, err := strategy.DecodeFrozenNaturalPolicy(bad); err == nil {
				if err := strategy.VerifyFrozenNaturalPolicy(bad, p); err == nil {
					t.Fatalf("mutated natural/%d wire byte %d accepted: %x", p.Ref.Version, i, bad)
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
				t.Fatalf("%s natural/%d wire decoded: %v", name, p.Ref.Version, err)
			}
			if err := strategy.VerifyFrozenNaturalPolicy(edit(wire), p); err == nil {
				t.Fatalf("%s natural/%d wire verified", name, p.Ref.Version)
			}
		}
	}
	// The canonical wire bytes for the new formats, restated from the
	// untouched framing (magic, format, ID length, ID, ref version, budget).
	for _, tc := range []struct {
		version int
		want    string
	}{
		{7, "41574e5000000007076e61747572616c000000072020"},
		{8, "41574e5000000008076e61747572616c000000082020"},
	} {
		if got := hex.EncodeToString(wires[uint32(tc.version)]); got != tc.want {
			t.Fatalf("natural/%d canonical wire drifted: %s, want %s", tc.version, got, tc.want)
		}
	}
	// Cross-ref verification fails closed in every direction across the
	// four-member set.
	v5Wire, err := strategy.EncodeNaturalPolicy(p5)
	if err != nil {
		t.Fatal(err)
	}
	v6Wire, err := strategy.EncodeFrozenNaturalPolicy(p6)
	if err != nil {
		t.Fatal(err)
	}
	otherWires := map[uint32][]byte{5: v5Wire, 6: v6Wire, 7: wires[7], 8: wires[8]}
	policies := map[uint32]strategy.NaturalPolicy{5: p5, 6: p6, 7: p7, 8: p7b}
	for wireOf, wire := range otherWires {
		for expectOf, expect := range policies {
			err := strategy.VerifyFrozenNaturalPolicy(wire, expect)
			if wireOf == expectOf {
				if err != nil {
					t.Fatalf("natural/%d wire must verify against its own expectation: %v", wireOf, err)
				}
				continue
			}
			if !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
				t.Fatalf("natural/%d wire verified against the natural/%d expectation: %v", wireOf, expectOf, err)
			}
		}
	}
	// A same-ref budget variant is well-formed but can never be restored
	// against the frozen expectation; a non-frozen expectation fails closed
	// before any bytes are compared.
	variant := p7
	variant.Budget = strategy.NaturalBudget{Candidates: 2, Evaluations: 2}
	variantWire, err := strategy.EncodeFrozenNaturalPolicy(variant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := strategy.DecodeFrozenNaturalPolicy(variantWire); err != nil || got != variant {
		t.Fatalf("natural/7 variant roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(variantWire, p7); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("same-ref natural/7 budget drift restored: %v", err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(wires[7], variant); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("non-frozen natural/7 expectation accepted: %v", err)
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for name, drifted := range map[string]strategy.NaturalPolicy{
		"ref 9":       func() strategy.NaturalPolicy { c := p7; c.Ref.Version = 9; return c }(),
		"format 6":    func() strategy.NaturalPolicy { c := p7; c.FormatVersion = 6; return c }(),
		"foreign ID":  func() strategy.NaturalPolicy { c := p7b; c.Ref.ID = "natural-7b"; return c }(),
		"long ID":     func() strategy.NaturalPolicy { c := p7b; c.Ref.ID = longID; return c }(),
		"zero budget": func() strategy.NaturalPolicy { c := p7; c.Budget.Candidates = 0; return c }(),
	} {
		if _, err := strategy.EncodeFrozenNaturalPolicy(drifted); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("drifted %s policy encoded: %v", name, err)
		}
	}
	// The frozen v5 codec is untouched and keeps refusing the new content,
	// while v5 bytes verify identically under both codecs.
	for _, p := range []strategy.NaturalPolicy{p7, p7b} {
		if _, err := strategy.EncodeNaturalPolicy(p); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("v5 codec encoded natural/%d: %v", p.Ref.Version, err)
		}
		if _, err := strategy.DecodeNaturalPolicy(wires[p.Ref.Version]); err == nil {
			t.Fatalf("v5 codec decoded the natural/%d wire", p.Ref.Version)
		}
		if err := strategy.VerifyNaturalPolicy(wires[p.Ref.Version], p5); err == nil {
			t.Fatalf("v5 codec verified the natural/%d wire", p.Ref.Version)
		}
		if _, err := strategy.NaturalPolicyFingerprint(p); err == nil {
			t.Fatalf("v5-only fingerprint accepted natural/%d", p.Ref.Version)
		}
	}
	for _, tc := range []struct {
		version int
		wirePin string
		shaPin  string
	}{
		// Pre-extension pins (captured at 2e67e5f): the {5,6} bytes and
		// digests are unchanged by the ref-set extension.
		{5, "41574e5000000005076e61747572616c000000052020", "d73d24f8f4c5219f8bfcd1b6c49af1702936d83d2fa4e82da49218807ae11b5b"},
		{6, "41574e5000000006076e61747572616c000000062020", "370c971e21f9a24c448368acf48adcdd75e45fd00bcb6bc55fd8947eb496cd72"},
	} {
		wire, err := strategy.EncodeFrozenNaturalPolicy(policies[uint32(tc.version)])
		if err != nil || hex.EncodeToString(wire) != tc.wirePin {
			t.Fatalf("natural/%d wire drifted from its pre-extension pin: %x (%v)", tc.version, wire, err)
		}
		sum, err := strategy.FrozenNaturalPolicyFingerprint(policies[uint32(tc.version)])
		if err != nil || hex.EncodeToString(sum[:]) != tc.shaPin {
			t.Fatalf("natural/%d fingerprint drifted from its pre-extension pin: %x (%v)", tc.version, sum, err)
		}
		if got, err := strategy.DecodeFrozenNaturalPolicy(wire); err != nil || got != policies[uint32(tc.version)] {
			t.Fatalf("natural/%d ref-set decode: %+v %v", tc.version, got, err)
		}
	}
	v5ViaSet, err := strategy.EncodeFrozenNaturalPolicy(p5)
	if err != nil || !bytes.Equal(v5ViaSet, v5Wire) {
		t.Fatalf("ref-set encoder diverges from the frozen v5 encoder: %x vs %x (%v)", v5ViaSet, v5Wire, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(v5Wire, p5); err != nil {
		t.Fatalf("v5 bytes must verify under the ref-set codec: %v", err)
	}
	if sumSet, err := strategy.FrozenNaturalPolicyFingerprint(p5); err != nil {
		t.Fatal(err)
	} else if sumPlain, err := strategy.NaturalPolicyFingerprint(p5); err != nil || sumSet != sumPlain {
		t.Fatalf("v5 fingerprint identity broke: %v vs %v", sumSet, sumPlain)
	}
	if got, err := strategy.DecodeNaturalPolicy(v5Wire); err != nil || got != p5 {
		t.Fatalf("frozen v5 codec regression on v5 bytes: %+v %v", got, err)
	}
}
