package strategy_test

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// natural9Fixture binds one actor to the frozen natural/9 policy through the
// v6-aware registration path (RegisterFrozenNatural; the frozen v5-only
// Register must keep rejecting the ref).
func natural9Fixture(t *testing.T, actor sim.EntityID) (*strategy.NaturalBound, strategy.NaturalRef) {
	t.Helper()
	p := strategy.FrozenNaturalPolicy9()
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

// natural9Observation builds a valid natural/9 claim view at the frozen
// bounds defaults, with the actor's own never-filed offer guard.
func natural9Observation(t *testing.T, actor sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalLabourObservation {
	t.Helper()
	return strategy.NaturalLabourObservation{
		NaturalObservation: naturalObservation(t, actor, ref, hour),
		OwnOffer:           strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestIdle, LastOfferHour: -1, LastReplyHour: -1},
	}
}

// natural9OfferView builds a worker candidate: alive, empty bag, never-filed
// offer row, with the same-patch founder roster (mirror of the birth
// request view's roster construction).
func natural9OfferView(t *testing.T, actor sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalLabourOfferObservation {
	t.Helper()
	home, err := world.NaturalFounderPatchID(naturalTestPatches, actor)
	if err != nil {
		t.Fatal(err)
	}
	var mates []sim.EntityID
	for id := sim.EntityID(1); id <= sim.EntityID(world.NaturalSlotsPerPatch*naturalTestPatches); id++ {
		if id == actor {
			continue
		}
		mateHome, err := world.NaturalFounderPatchID(naturalTestPatches, id)
		if err != nil {
			t.Fatal(err)
		}
		if mateHome == home {
			mates = append(mates, id)
		}
	}
	return strategy.NaturalLabourOfferObservation{
		Actor: actor, Ref: ref, Hour: hour, WorldVersion: 9, HomePatch: home,
		Energy: sim.IntegerValue(8), BagUnits: sim.IntegerValue(0),
		OwnOffer:   strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestIdle, LastOfferHour: -1, LastReplyHour: -1},
		PatchMates: mates,
	}
}

// natural9DecisionView builds an employer deciding at the floor margins
// stock=5, k+wip=2, with one pending addressed offer.
func natural9DecisionView(employer, worker sim.EntityID, ref strategy.NaturalRef, hour int) strategy.NaturalLabourDecisionObservation {
	return strategy.NaturalLabourDecisionObservation{
		Actor: employer, Ref: ref, Hour: hour, WorldVersion: 9,
		Energy: sim.IntegerValue(8), GranaryStock: sim.IntegerValue(5),
		Capital: sim.IntegerValue(2), Wip: sim.IntegerValue(0),
		OwnLastDecisionHour: -1,
		Pending:             []strategy.NaturalLabourPendingView{{Worker: worker, Addressee: employer, Hour: hour}},
	}
}

// TestNatural9FrozenIdentityRefSetAndRegistration pins the frozen natural/9
// identity, its place in the extended ref set, the v9-aware registration
// path, and the untouched v5-only validation.
func TestNatural9FrozenIdentityRefSetAndRegistration(t *testing.T) {
	p9 := strategy.FrozenNaturalPolicy9()
	if p9 != (strategy.NaturalPolicy{FormatVersion: 9, Ref: strategy.NaturalRef{ID: "natural", Version: 9},
		Budget: strategy.NaturalBudget{Candidates: 32, Evaluations: 32}}) {
		t.Fatalf("frozen natural/9 policy drifted: %+v", p9)
	}
	frozen := []strategy.NaturalPolicy{
		strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6(),
		strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b(), p9,
	}
	for _, p := range frozen {
		got, err := strategy.FrozenNaturalPolicyForRef(p.Ref)
		if err != nil || got != p {
			t.Fatalf("natural/%d resolution: %+v %v", p.Ref.Version, got, err)
		}
		if !strategy.ValidFrozenNaturalPolicy(p) {
			t.Fatalf("natural/%d must validate as a ref-set member", p.Ref.Version)
		}
	}
	for _, unknown := range []strategy.NaturalRef{
		{ID: "natural", Version: 10}, {ID: "natural", Version: 0}, {ID: "other", Version: 9}, {ID: "natural-9", Version: 9}, {},
	} {
		if got, err := strategy.FrozenNaturalPolicyForRef(unknown); !errors.Is(err, strategy.ErrUnknownNaturalPolicy) {
			t.Fatalf("unknown ref %v resolved to %+v (%v)", unknown, got, err)
		}
		if strategy.ValidFrozenNaturalPolicy(strategy.NaturalPolicy{Ref: unknown, FormatVersion: unknown.Version,
			Budget: strategy.NaturalBudget{Candidates: 32, Evaluations: 32}}) {
			t.Fatalf("unknown ref %v passed the ref-set predicate", unknown)
		}
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for name, invalid := range map[string]strategy.NaturalPolicy{
		"format 8": func() strategy.NaturalPolicy { c := p9; c.FormatVersion = 8; return c }(),
		"format 5": func() strategy.NaturalPolicy { c := p9; c.FormatVersion = 5; return c }(),
		"ref 8": func() strategy.NaturalPolicy {
			c := p9
			c.Ref = strategy.NaturalRef{ID: "natural", Version: 8}
			return c
		}(),
		"empty ID":  func() strategy.NaturalPolicy { c := p9; c.Ref.ID = ""; return c }(),
		"long ID":   func() strategy.NaturalPolicy { c := p9; c.Ref.ID = longID; return c }(),
		"zero ref":  func() strategy.NaturalPolicy { c := p9; c.Ref = strategy.NaturalRef{}; return c }(),
		"cand 0":    func() strategy.NaturalPolicy { c := p9; c.Budget.Candidates = 0; return c }(),
		"cand 33":   func() strategy.NaturalPolicy { c := p9; c.Budget.Candidates = 33; return c }(),
		"eval 0":    func() strategy.NaturalPolicy { c := p9; c.Budget.Evaluations = 0; return c }(),
		"eval 33":   func() strategy.NaturalPolicy { c := p9; c.Budget.Evaluations = 33; return c }(),
		"foreign-9": func() strategy.NaturalPolicy { c := strategy.FrozenNaturalPolicy7b(); c.FormatVersion = 9; return c }(),
	} {
		if strategy.ValidFrozenNaturalPolicy(invalid) {
			t.Fatalf("invalid %s policy passed the ref-set predicate: %+v", name, invalid)
		}
	}
	// The frozen v5 Register keeps its v5-only validation: it rejects the
	// natural/9 ref; the v9-aware path registers it (and same-ref budget
	// variants) with the same shape semantics.
	r := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := r.Register(p9); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("frozen v5 Register accepted the natural/9 ref: %v", err)
	}
	if err := r.RegisterFrozenNatural(p9); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterFrozenNatural(p9); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("duplicate natural/9 reference accepted: %v", err)
	}
	variant := p9
	variant.Budget = strategy.NaturalBudget{Candidates: 3, Evaluations: 3}
	if err := strategy.NewNaturalRegistry(naturalTestPatches).RegisterFrozenNatural(variant); err != nil {
		t.Fatalf("same-ref budget variant must register: %v", err)
	}
	// All five frozen refs coexist in one registry.
	all := strategy.NewNaturalRegistry(naturalTestPatches)
	for _, p := range frozen {
		if err := all.RegisterFrozenNatural(p); err != nil {
			t.Fatalf("frozen natural/%d coexistence: %v", p.Ref.Version, err)
		}
	}
	b9, ref9 := natural9Fixture(t, 1)
	if b9.Binding() != (strategy.NaturalBinding{Actor: 1, Ref: ref9}) || b9.Policy() != p9 {
		t.Fatalf("natural/9 binding: %+v %+v", b9.Binding(), b9.Policy())
	}
	// The 9 cascade demands its own ref on the observation, and the frozen
	// 7b cascade rejects the natural/9 ref on its own observation type.
	b7b, ref7b := natural7bFixture(t, 1)
	wrongRef := natural9Observation(t, 1, ref7b, 5)
	if _, err := b9.Evaluate9(wrongRef); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("9 cascade accepted a foreign-ref observation: %v", err)
	}
	wrongBound := natural9Observation(t, 1, ref9, 5)
	if _, err := b7b.Evaluate7b(wrongBound.NaturalObservation); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("7b cascade accepted the natural/9 ref: %v", err)
	}
	var nilBound *strategy.NaturalBound
	if _, err := nilBound.Evaluate9(strategy.NaturalLabourObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 9 cascade")
	}
	if _, err := nilBound.EvaluateLabourOffer9(strategy.NaturalLabourOfferObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 9 offer cascade")
	}
	if _, err := nilBound.EvaluateLabourDecision9(strategy.NaturalLabourDecisionObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 9 decision cascade")
	}
	if _, err := nilBound.EvaluateBirthRequest9(strategy.NaturalRequestObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 9 request cascade")
	}
	if _, err := nilBound.EvaluateBirthReply9(strategy.NaturalReplyObservation{}); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatal("nil bound evaluated the 9 reply cascade")
	}
}

// TestNatural9RefSetBackwardCompatPins proves the ref-set seam extension is
// byte-inert for the pre-existing {natural/5, natural/6, natural/7,
// natural/7b} set: the wires, fingerprints, and resolved policy values below
// were captured from the executable BEFORE the natural6.go extension (at
// 46029dc, in a throwaway detached worktree) and are hard-coded here, so any
// drift in the pre-edit encodings, digests, or resolution fails loudly.
func TestNatural9RefSetBackwardCompatPins(t *testing.T) {
	const (
		// Captured at 46029dc before the ref-set extension.
		pinWire5 = "41574e5000000005076e61747572616c000000052020"
		pinSHA5  = "d73d24f8f4c5219f8bfcd1b6c49af1702936d83d2fa4e82da49218807ae11b5b"
		pinWire6 = "41574e5000000006076e61747572616c000000062020"
		pinSHA6  = "370c971e21f9a24c448368acf48adcdd75e45fd00bcb6bc55fd8947eb496cd72"
		pinWire7 = "41574e5000000007076e61747572616c000000072020"
		pinSHA7  = "ee1af56cd93720c62ccee22f2e22479971208ca60da8d082a8eaa03a584970eb"
		pinWire8 = "41574e5000000008076e61747572616c000000082020"
		pinSHA8  = "42af9533f9cc0a3bf32e1ec6f5312f8374ba9609d166d6a31ddd12241200e202"
	)
	p5, p6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	p7, p7b := strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b()
	p9 := strategy.FrozenNaturalPolicy9()
	for _, tc := range []struct {
		p         strategy.NaturalPolicy
		wire, sha string
	}{
		{p5, pinWire5, pinSHA5},
		{p6, pinWire6, pinSHA6},
		{p7, pinWire7, pinSHA7},
		{p7b, pinWire8, pinSHA8},
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
	// The frozen v5-only codec still produces exactly the pinned v5 bytes.
	v5Wire, err := strategy.EncodeNaturalPolicy(p5)
	if err != nil || hex.EncodeToString(v5Wire) != pinWire5 {
		t.Fatalf("v5 codec wire drifted: %x (%v)", v5Wire, err)
	}
	// The new natural/9 content encodes on the identical untouched framing.
	wire9, err := strategy.EncodeFrozenNaturalPolicy(p9)
	if err != nil || hex.EncodeToString(wire9) != "41574e5000000009076e61747572616c000000092020" {
		t.Fatalf("natural/9 canonical wire drifted: %x (%v)", wire9, err)
	}
	if len(wire9) != 22 {
		t.Fatalf("natural/9 wire length %d, want 22", len(wire9))
	}
	// The resolved values are exactly the frozen contents, unchanged.
	for _, p := range []strategy.NaturalPolicy{p5, p6, p7, p7b, p9} {
		if got, err := strategy.FrozenNaturalPolicyForRef(p.Ref); err != nil || got != p {
			t.Fatalf("natural/%d resolution drifted: %+v %v", p.Ref.Version, got, err)
		}
	}
	// The frozen v5-only codec keeps refusing the new content.
	if _, err := strategy.EncodeNaturalPolicy(p9); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("v5 codec encoded natural/9: %v", err)
	}
	if _, err := strategy.DecodeNaturalPolicy(wire9); err == nil {
		t.Fatal("v5 codec decoded the natural/9 wire")
	}
	if err := strategy.VerifyNaturalPolicy(wire9, p5); err == nil {
		t.Fatal("v5 codec verified the natural/9 wire")
	}
	if _, err := strategy.NaturalPolicyFingerprint(p9); err == nil {
		t.Fatal("v5-only fingerprint accepted natural/9")
	}
}

// TestNatural9Format9Codec pins the format-9 wire: the identical untouched
// AWNP framing, canonical bytes, fingerprint identity, cross-ref
// verification failures across the five-member set, and the byte-level
// canonicality discipline of the frozen codec.
func TestNatural9Format9Codec(t *testing.T) {
	p5, p6 := strategy.FrozenNaturalPolicy(), strategy.FrozenNaturalPolicyV6()
	p7, p7b, p9 := strategy.FrozenNaturalPolicy7(), strategy.FrozenNaturalPolicy7b(), strategy.FrozenNaturalPolicy9()
	wire, err := strategy.EncodeFrozenNaturalPolicy(p9)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire[:4], []byte("AWNP")) || wire[4] != 0 || wire[5] != 0 || wire[6] != 0 || wire[7] != 9 {
		t.Fatalf("natural/9 wire magic/format drifted: %x", wire[:8])
	}
	if wire[8] != 7 || !bytes.Equal(wire[9:16], []byte("natural")) {
		t.Fatalf("natural/9 wire ID framing drifted: %x", wire[8:16])
	}
	again, err := strategy.EncodeFrozenNaturalPolicy(p9)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatalf("natural/9 encoding is not deterministic: %x %x %v", wire, again, err)
	}
	if got, err := strategy.DecodeFrozenNaturalPolicy(wire); err != nil || got != p9 {
		t.Fatalf("natural/9 roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(wire, p9); err != nil {
		t.Fatalf("frozen natural/9 wire must verify: %v", err)
	}
	if sum, err := strategy.FrozenNaturalPolicyFingerprint(p9); err != nil || sum != sha256.Sum256(wire) {
		t.Fatalf("natural/9 fingerprint: %v %v", sum, err)
	}
	// Every single-byte mutation either fails decoding or fails ref-set
	// verification against the executable's frozen policy.
	for i := range wire {
		bad := append([]byte(nil), wire...)
		bad[i] ^= 0xff
		if _, err := strategy.DecodeFrozenNaturalPolicy(bad); err == nil {
			if err := strategy.VerifyFrozenNaturalPolicy(bad, p9); err == nil {
				t.Fatalf("mutated natural/9 wire byte %d accepted: %x", i, bad)
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
			t.Fatalf("%s natural/9 wire decoded: %v", name, err)
		}
		if err := strategy.VerifyFrozenNaturalPolicy(edit(wire), p9); err == nil {
			t.Fatalf("%s natural/9 wire verified", name)
		}
	}
	// Cross-ref verification fails closed in every direction across the
	// five-member set.
	v5Wire, err := strategy.EncodeNaturalPolicy(p5)
	if err != nil {
		t.Fatal(err)
	}
	wires := map[uint32][]byte{5: v5Wire}
	for _, p := range []strategy.NaturalPolicy{p6, p7, p7b, p9} {
		w, err := strategy.EncodeFrozenNaturalPolicy(p)
		if err != nil {
			t.Fatal(err)
		}
		wires[p.Ref.Version] = w
	}
	policies := map[uint32]strategy.NaturalPolicy{5: p5, 6: p6, 7: p7, 8: p7b, 9: p9}
	for wireOf, w := range wires {
		for expectOf, expect := range policies {
			err := strategy.VerifyFrozenNaturalPolicy(w, expect)
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
	variant := p9
	variant.Budget = strategy.NaturalBudget{Candidates: 2, Evaluations: 2}
	variantWire, err := strategy.EncodeFrozenNaturalPolicy(variant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := strategy.DecodeFrozenNaturalPolicy(variantWire); err != nil || got != variant {
		t.Fatalf("natural/9 variant roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(variantWire, p9); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("same-ref natural/9 budget drift restored: %v", err)
	}
	if err := strategy.VerifyFrozenNaturalPolicy(wire, variant); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("non-frozen natural/9 expectation accepted: %v", err)
	}
	longID := string(make([]byte, strategy.MaxRefIDBytes+1))
	for name, drifted := range map[string]strategy.NaturalPolicy{
		"ref 8":    func() strategy.NaturalPolicy { c := p9; c.Ref.Version = 8; return c }(),
		"format 8": func() strategy.NaturalPolicy { c := p9; c.FormatVersion = 8; return c }(),
		"foreign ID": func() strategy.NaturalPolicy {
			c := p9
			c.Ref.ID = "natural-9"
			return c
		}(),
		"long ID": func() strategy.NaturalPolicy {
			c := p9
			c.Ref.ID = longID
			return c
		}(),
		"zero budget": func() strategy.NaturalPolicy {
			c := p9
			c.Budget.Candidates = 0
			return c
		}(),
		"over budget": func() strategy.NaturalPolicy {
			c := p9
			c.Budget.Evaluations = 33
			return c
		}(),
	} {
		if _, err := strategy.EncodeFrozenNaturalPolicy(drifted); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
			t.Fatalf("drifted %s policy encoded: %v", name, err)
		}
	}
}

// TestNatural97bDifferentialGrid walks the full shared-observation grid —
// energy 0..12 × capital 0..8 × wip {0,1} × granary {0,1,3,8} × bag {0,1} ×
// denial {none, gather, build} × one stocked home slot {no, yes} × own offer
// guard {never, pending, due-accepted, stale-accepted, refused, expired} —
// and asserts, per cell, the exact frozen-design delta set of natural/9
// against the natural/7b control: flips occur EXACTLY in the declared trade
// cell, the bag decision with the actor's own accepted contract inside its
// single delivery hour and one held unit, taken after the frozen build-
// denial leg (a pending contract never attempts a build, so that unreachable
// corner stays unflipped in both cascades). Every other cell must be
// identical, so any additional flip family fails here rather than surviving
// into a run.
func TestNatural97bDifferentialGrid(t *testing.T) {
	b7b, ref7b := natural7bFixture(t, 1)
	b9, ref9 := natural9Fixture(t, 1)
	const hour = 5

	guards := []struct {
		name  string
		guard strategy.NaturalLabourOfferGuard
		due   bool
	}{
		{"never", strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestIdle, LastOfferHour: -1, LastReplyHour: -1}, false},
		{"pending", strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestPending, LastOfferHour: 4, LastReplyHour: -1}, false},
		{"due", strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestAccepted, LastOfferHour: 4, LastReplyHour: 4}, true},
		{"stale", strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestAccepted, LastOfferHour: 3, LastReplyHour: 3}, false},
		{"refused", strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestRefused, LastOfferHour: 4, LastReplyHour: 4}, false},
		{"expired", strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestExpired, LastOfferHour: 3, LastReplyHour: -1}, false},
	}

	type margins struct {
		energy, capital, wip, granary, bag int64
		denial                             strategy.NaturalDenial
		stocked                            bool
	}
	cell := func(m margins, guard strategy.NaturalLabourOfferGuard) (o7b strategy.NaturalObservation, o9 strategy.NaturalLabourObservation) {
		base := naturalObservation(t, 1, ref7b, hour)
		base.HomeSlots = append([]strategy.NaturalSlotView(nil), base.HomeSlots...)
		base.Energy = sim.IntegerValue(m.energy)
		base.Capital = sim.IntegerValue(m.capital)
		base.Wip = sim.IntegerValue(m.wip)
		base.GranaryStock = sim.IntegerValue(m.granary)
		base.BagUnits = sim.IntegerValue(m.bag)
		base.LastDenial = m.denial
		if m.stocked {
			base.HomeSlots[0].Stock = sim.IntegerValue(1)
		}
		o9 = strategy.NaturalLabourObservation{NaturalObservation: base, OwnOffer: guard}
		o9.Ref = ref9
		return base, o9
	}

	var cells, structuralErrors, bagCells, claimCells, denialCornerCells int
	flips, replacedBuild, replacedEat := 0, 0, 0
	for energy := int64(0); energy <= 12; energy++ {
		for capital := int64(0); capital <= 8; capital++ {
			for wip := int64(0); wip <= 1; wip++ {
				for _, granary := range []int64{0, 1, 3, 8} {
					for bag := int64(0); bag <= 1; bag++ {
						for _, denial := range []strategy.NaturalDenial{strategy.NaturalNoDenial, strategy.NaturalGatherDenied, strategy.NaturalBuildDenied} {
							for _, stocked := range []bool{false, true} {
								for _, guard := range guards {
									m := margins{energy, capital, wip, granary, bag, denial, stocked}
									o7b, o9 := cell(m, guard.guard)
									c7b, err7b := b7b.Evaluate7b(o7b)
									c9, err9 := b9.Evaluate9(o9)
									cells++
									// Structural acceptance is identical: a
									// build denial with an empty bag is
									// malformed for both, everything else
									// evaluates.
									if (err7b == nil) != (err9 == nil) {
										t.Fatalf("cell %+v %s: structural acceptance diverged: %v vs %v", m, guard.name, err7b, err9)
									}
									if err7b != nil {
										if !errors.Is(err7b, strategy.ErrInvalidNaturalObservation) || !errors.Is(err9, strategy.ErrInvalidNaturalObservation) {
											t.Fatalf("cell %+v %s: unexpected error class: %v %v", m, guard.name, err7b, err9)
										}
										structuralErrors++
										continue
									}
									declaredCell := guard.due && bag == 1 && denial != strategy.NaturalBuildDenied && energy > 0
									if bag == 1 {
										bagCells++
										if guard.due && denial == strategy.NaturalBuildDenied && energy > 0 {
											denialCornerCells++
										}
									} else {
										claimCells++
									}
									if declaredCell {
										// The declared trade cell: the
										// delivery replaces whatever the 7b
										// bag decision chose (Build at the
										// invested margins, Eat elsewhere).
										want7b := naturalV6ExpectedBag(energy, capital, wip, granary, denial)
										if c7b.Kind != want7b || c7b.Evaluated != 1 || c7b.Fallback != strategy.NaturalNoFallback {
											t.Fatalf("cell %+v: 7b control drifted: %+v", m, c7b)
										}
										want := strategy.NaturalChoice{Kind: strategy.NaturalDeliver, ObservedVersion: 9,
											Ref: ref9, Evaluated: 1, Fallback: strategy.NaturalNoFallback}
										if c9 != want {
											t.Fatalf("cell %+v: natural/9 must deliver: got %+v, want %+v", m, c9, want)
										}
										if naturalChoiceShape(c7b) == naturalChoiceShape(c9) {
											t.Fatalf("cell %+v: declared flip did not flip: %+v", m, c9)
										}
										flips++
										if want7b == strategy.NaturalBuild {
											replacedBuild++
										} else {
											replacedEat++
										}
										continue
									}
									if naturalChoiceShape(c7b) != naturalChoiceShape(c9) {
										t.Fatalf("cell %+v %s diverges outside the declared trade cells: 7b %+v vs 9 %+v",
											m, guard.name, naturalChoiceShape(c7b), naturalChoiceShape(c9))
									}
								}
							}
						}
					}
				}
			}
		}
	}
	// Grid shape and the exact flip count: 13 energies × 9 capitals × 2 wip
	// × 4 granaries × 2 bags × 3 denials × 2 stocked × 6 guards. The flip
	// needs the bag phase, the single due guard state, a living actor, no
	// build denial — energies {1..12} (12) × capitals (9) × wip (2) ×
	// granaries (4) × denial {none, gather} (2) × stocked (2) = 3456 cells,
	// and the unreachable denial corner (both cascades eat) adds 1728 bag
	// cells.
	if cells != 67392 || structuralErrors != 11232 || bagCells != 33696 || claimCells != 22464 {
		t.Fatalf("grid shape drifted: cells=%d structural=%d bag=%d claim=%d", cells, structuralErrors, bagCells, claimCells)
	}
	if flips != 3456 || denialCornerCells != 1728 {
		t.Fatalf("flip counts drifted: flips=%d denialCorner=%d, want exactly 3456 flips and 1728 corner cells", flips, denialCornerCells)
	}
	if replacedBuild == 0 || replacedEat == 0 {
		t.Fatalf("flip family vacuous: the delivery must replace both 7b bag outcomes (Build=%d, Eat=%d)", replacedBuild, replacedEat)
	}
	t.Logf("differential grid: %d cells, %d structural rejections, %d bag cells, %d claim cells; "+
		"flips = %d (exactly the declared delivery cells, replacing 7b Build=%d/Eat=%d); "+
		"%d unreachable denial-corner cells stay verbatim; all other cells identical",
		cells, structuralErrors, bagCells, claimCells, flips, replacedBuild, replacedEat, denialCornerCells)
}

// TestNatural9DeliverTruthTable pins the delivery window exactly: the due
// predicate fires only on the accepted row whose decision hour is the
// previous hour (the contract's single delivery hour), the bag must hold
// exactly one unit, the frozen build-denial leg stays ahead of the
// substitution, and out-of-range margins degrade verbatim to InvalidSelf.
func TestNatural9DeliverTruthTable(t *testing.T) {
	b9, ref9 := natural9Fixture(t, 1)
	evaluate := func(o strategy.NaturalLabourObservation) strategy.NaturalChoice {
		t.Helper()
		c, err := b9.Evaluate9(o)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	due := func(hour int, lastReply int64, bag int64) strategy.NaturalLabourObservation {
		o := natural9Observation(t, 1, ref9, hour)
		o.BagUnits = sim.IntegerValue(bag)
		o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestAccepted, LastOfferHour: lastReply, LastReplyHour: lastReply}
		return o
	}
	// The delivery replaces both 7b bag outcomes (the invested Build and the
	// margin-failed Eat): the hour's unit conversion is the contract's.
	for _, tc := range []struct {
		name                          string
		energy, capital, wip, granary int64
		want7b                        strategy.NaturalAction
	}{
		{"deliver replaces the invested Build", 5, 0, 0, 0, strategy.NaturalBuild},
		{"deliver replaces the fed-larder Build", 3, 0, 0, 1, strategy.NaturalBuild},
		{"deliver replaces the wip-finish Build", 3, 0, 1, 0, strategy.NaturalBuild},
		{"deliver replaces the margin-failed Eat", 1, 0, 0, 0, strategy.NaturalEat},
		{"deliver replaces the saturated Eat", 12, 7, 1, 8, strategy.NaturalEat},
	} {
		o := due(5, 4, 1)
		o.Energy = sim.IntegerValue(tc.energy)
		o.Capital = sim.IntegerValue(tc.capital)
		o.Wip = sim.IntegerValue(tc.wip)
		o.GranaryStock = sim.IntegerValue(tc.granary)
		got := evaluate(o)
		if want := (strategy.NaturalChoice{Kind: strategy.NaturalDeliver, ObservedVersion: 9,
			Ref: ref9, Evaluated: 1, Fallback: strategy.NaturalNoFallback}); got != want {
			t.Fatalf("%s: %+v, want %+v", tc.name, got, want)
		}
		if control := naturalV6ExpectedBag(tc.energy, tc.capital, tc.wip, tc.granary, strategy.NaturalNoDenial); control != tc.want7b {
			t.Fatalf("%s: grid control drifted: %v vs %v", tc.name, control, tc.want7b)
		}
	}
	// The window boundaries: exactly LastReplyHour = hour-1; every other
	// accepted shape stays on the frozen 7b bag decision (here Build at
	// E = 11), and an accepted row without its decision hour is malformed.
	for _, tc := range []struct {
		name       string
		hour       int
		lastReply  int64
		deliver    bool
		structural bool
	}{
		{"decision hour last hour: due", 5, 4, true, false},
		{"one hour early: not due", 5, 3, false, false},
		{"same-hour accept is not a bag-decision input", 5, 5, false, false},
		{"an accepted row must carry its decision hour", 5, -1, false, true},
		{"hour 0 can never be a delivery hour", 0, -1, false, true},
	} {
		o := due(tc.hour, tc.lastReply, 1)
		c, err := b9.Evaluate9(o)
		if tc.structural {
			if !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
				t.Fatalf("%s: accepted malformed guard: %v", tc.name, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if tc.deliver && c.Kind != strategy.NaturalDeliver {
			t.Fatalf("%s: %+v", tc.name, c)
		}
		if !tc.deliver && c.Kind != strategy.NaturalBuild {
			t.Fatalf("%s: %+v", tc.name, c)
		}
	}
	// The bag gate: a due contract with an empty bag stays on the frozen
	// claim cascade (a denied gather voids the settlement on the world's
	// typed path; the policy adds nothing to it).
	empty := due(5, 4, 0)
	if got := evaluate(empty); got.Kind != strategy.NaturalWait {
		t.Fatalf("due contract with an empty bag must follow the 7b claim cascade: %+v", got)
	}
	empty.LastDenial = strategy.NaturalGatherDenied
	if got := evaluate(empty); got.Kind != strategy.NaturalRest || got.RestDuration != strategy.NaturalRestDuration {
		t.Fatalf("denied gather under a due contract must take the frozen fallback: %+v", got)
	}
	// The frozen build-denial leg stays ahead of the substitution: both
	// cascades eat the held unit there (a pending contract never attempts a
	// build, so the corner is unreachable in the world).
	denied := due(5, 4, 1)
	denied.LastDenial = strategy.NaturalBuildDenied
	if got := evaluate(denied); got != (strategy.NaturalChoice{Kind: strategy.NaturalEat, ObservedVersion: 9,
		Ref: ref9, Evaluated: 1, Fallback: strategy.NaturalNoFallback}) {
		t.Fatalf("build-denied delivery corner must stay verbatim: %+v", got)
	}
	// A malformed guard is a structural error, verbatim 7b classes.
	badStatus := due(5, 4, 1)
	badStatus.OwnOffer.Status = strategy.NaturalRequestStatus(5)
	if _, err := b9.Evaluate9(badStatus); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatalf("out-of-range offer status accepted: %v", err)
	}
	unfiled := due(5, 4, 1)
	unfiled.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestAccepted, LastOfferHour: -1, LastReplyHour: 4}
	if _, err := b9.Evaluate9(unfiled); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("an accepted row must carry its filing hour")
	}
	future := due(5, 4, 1)
	future.OwnOffer.LastOfferHour = 6
	if _, err := b9.Evaluate9(future); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a future filing hour accepted")
	}
	// Out-of-range own values degrade to InvalidSelf, verbatim 7b.
	degraded := due(5, 4, 1)
	degraded.Energy = sim.IntegerValue(13)
	if got := evaluate(degraded); got.Kind != strategy.NaturalWait || got.Fallback != strategy.NaturalInvalidSelf {
		t.Fatalf("out-of-range energy must degrade: %+v", got)
	}
	// The bag decision needs only a nonzero budget, verbatim v6 semantics.
	limitedPolicy := strategy.FrozenNaturalPolicy9()
	limitedPolicy.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
	limitedReg := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := limitedReg.RegisterFrozenNatural(limitedPolicy); err != nil {
		t.Fatal(err)
	}
	limited, err := limitedReg.Bind(strategy.NaturalBinding{Actor: 1, Ref: limitedPolicy.Ref})
	if err != nil {
		t.Fatal(err)
	}
	held := due(5, 4, 1)
	held.Energy = sim.IntegerValue(5)
	if c, err := limited.Evaluate9(held); err != nil || c.Kind != strategy.NaturalDeliver {
		t.Fatalf("delivery under a 1/1 budget: %+v %v", c, err)
	}
	held.Energy = sim.IntegerValue(0)
	if c, err := limited.Evaluate9(held); err != nil || c.Kind != strategy.NaturalWait || c.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("a dead actor never delivers: %+v %v", c, err)
	}
}

// TestNatural9BirthTwinsVerbatim7b pins the birth cascades of natural/9
// against the frozen natural/7b cascades: the filing and accept margins at
// own energy >= 4, the reported-claims margins, the kin clause, the typed
// refusal reasons, the wraparound addressee rotation, and the shared guard
// rails are identical in every probed cell — the birth machinery is
// untouched by the wage additions.
func TestNatural9BirthTwinsVerbatim7b(t *testing.T) {
	ref7b := strategy.FrozenNaturalPolicy7b().Ref
	ref9 := strategy.FrozenNaturalPolicy9().Ref
	reg := strategy.NewNaturalRegistry(naturalTestPatches)
	for _, p := range []strategy.NaturalPolicy{strategy.FrozenNaturalPolicy7b(), strategy.FrozenNaturalPolicy9()} {
		if err := reg.RegisterFrozenNatural(p); err != nil {
			t.Fatal(err)
		}
	}
	bound := map[strategy.NaturalRef]*strategy.NaturalBound{}
	for _, ref := range []strategy.NaturalRef{ref7b, ref9} {
		b, err := reg.Bind(strategy.NaturalBinding{Actor: 2, Ref: ref})
		if err != nil {
			t.Fatal(err)
		}
		bound[ref] = b
	}
	request := func(ref strategy.NaturalRef, energy int64) strategy.NaturalRequestChoice {
		t.Helper()
		view := naturalRequestView(t, 2, ref, 5)
		view.Energy = sim.IntegerValue(energy)
		var c strategy.NaturalRequestChoice
		var err error
		if ref == ref9 {
			c, err = bound[ref].EvaluateBirthRequest9(view)
		} else {
			c, err = bound[ref].EvaluateBirthRequest7b(view)
		}
		if err != nil {
			t.Fatal(err)
		}
		c.Ref = strategy.NaturalRef{}
		return c
	}
	for _, energy := range []int64{0, 2, 3, 4, 8, 12} {
		c7b, c9 := request(ref7b, energy), request(ref9, energy)
		if c7b != c9 {
			t.Fatalf("filing at E=%d diverged: 7b=%+v 9=%+v", energy, c7b, c9)
		}
		want := strategy.NaturalRequestWait
		if energy >= 4 {
			want = strategy.NaturalRequestFile
		}
		if c9.Kind != want {
			t.Fatalf("filing at E=%d: %v, want %v", energy, c9.Kind, want)
		}
		if want == strategy.NaturalRequestFile && (c9.Addressee != 3 || c9.ReportedCapital != 2 ||
			c9.ReportedGranary != 3 || c9.Evaluated != 7) {
			t.Fatalf("filing at E=%d drifted: %+v", energy, c9)
		}
	}
	reply := func(ref strategy.NaturalRef, energy int64) strategy.NaturalReplyChoice {
		t.Helper()
		view := naturalReplyView(2, 1, ref, 5)
		view.Energy = sim.IntegerValue(energy)
		var c strategy.NaturalReplyChoice
		var err error
		if ref == ref9 {
			c, err = bound[ref].EvaluateBirthReply9(view)
		} else {
			c, err = bound[ref].EvaluateBirthReply7b(view)
		}
		if err != nil {
			t.Fatal(err)
		}
		c.Ref = strategy.NaturalRef{}
		return c
	}
	for _, energy := range []int64{0, 1, 2, 3, 4, 8, 12} {
		c7b, c9 := reply(ref7b, energy), reply(ref9, energy)
		if c7b != c9 {
			t.Fatalf("reply at E=%d diverged: 7b=%+v 9=%+v", energy, c7b, c9)
		}
		want, reason := strategy.NaturalReplyAccept, strategy.NaturalReplyReasonNone
		if energy == 0 {
			want, reason = strategy.NaturalReplyNone, strategy.NaturalReplyReasonNone // a dead consenter never acts
		} else if energy <= 3 {
			want, reason = strategy.NaturalReplyRefuse, strategy.NaturalReplyReasonConsentOwn
		}
		if c9.Kind != want || c9.Reason != reason {
			t.Fatalf("reply at E=%d: %v/%v, want %v/%v", energy, c9.Kind, c9.Reason, want, reason)
		}
	}
	// The reported-claims margins and the kin clause are verbatim: the
	// refusal reasons sit exactly where 7b's do.
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
		view9 := naturalReplyView(2, 1, ref9, 5)
		view9.Energy = sim.IntegerValue(4)
		view9.Pending[0].ReportedCapital = tc.reportedCapital
		view9.Pending[0].ReportedGranary = tc.reportedGranary
		view9.Pending[0].KinRelated = tc.kinRelated
		view9.Pending[0].KinDistance = tc.kinDistance
		view7b := view9
		view7b.Ref = ref7b
		c9, err := bound[ref9].EvaluateBirthReply9(view9)
		if err != nil {
			t.Fatal(err)
		}
		c7b, err := bound[ref7b].EvaluateBirthReply7b(view7b)
		if err != nil {
			t.Fatal(err)
		}
		if c9.Kind != tc.want || c9.Reason != tc.reason || c9.Requester != 1 {
			t.Fatalf("%s: %+v", tc.name, c9)
		}
		c9.Ref, c7b.Ref = strategy.NaturalRef{}, strategy.NaturalRef{}
		if c9 != c7b {
			t.Fatalf("%s: 9 diverged from 7b: %+v vs %+v", tc.name, c9, c7b)
		}
	}
	// The wraparound addressee rotation is verbatim: actor 8 wraps to 1.
	for _, ref := range []strategy.NaturalRef{ref7b, ref9} {
		b8, err := reg.Bind(strategy.NaturalBinding{Actor: 8, Ref: ref})
		if err != nil {
			t.Fatal(err)
		}
		view := naturalRequestView(t, 8, ref, 5)
		var c strategy.NaturalRequestChoice
		if ref == ref9 {
			c, err = b8.EvaluateBirthRequest9(view)
		} else {
			c, err = b8.EvaluateBirthRequest7b(view)
		}
		if err != nil || c.Kind != strategy.NaturalRequestFile || c.Addressee != 1 {
			t.Fatalf("wraparound addressee: %+v %v", c, err)
		}
	}
	// The shared guard rails are verbatim: one request per actor-hour, one
	// reply per actor-hour, and dead actors stay silent — identically on
	// both refs.
	view := naturalRequestView(t, 2, ref9, 5)
	view.OwnRequest = strategy.NaturalRequestGuard{Status: strategy.NaturalRequestPending, LastRequestHour: 4}
	if c, err := bound[ref9].EvaluateBirthRequest9(view); err != nil || c.Kind != strategy.NaturalRequestWait {
		t.Fatalf("pending request must resolve first: %+v %v", c, err)
	}
	guard := naturalReplyView(2, 1, ref9, 5)
	guard.OwnLastReplyHour = 5
	if c, err := bound[ref9].EvaluateBirthReply9(guard); err != nil || c.Kind != strategy.NaturalReplyNone {
		t.Fatalf("second reply in one hour: %+v %v", c, err)
	}
	dead := naturalRequestView(t, 2, ref9, 5)
	dead.Energy = sim.IntegerValue(0)
	if c, err := bound[ref9].EvaluateBirthRequest9(dead); err != nil || c.Kind != strategy.NaturalRequestWait || c.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("dead proposer: %+v %v", c, err)
	}
}

// TestNatural9OfferFilingPreconditions pins the rule-516 worker decision:
// the exact frozen filing list (alive, bag = 0, no open offer, last-offer-
// hour < hour), the deterministic same-patch addressee rotation, the typed
// budget fallback, structural rejection of malformed rosters and guards, and
// the offer choice publishing nothing beyond the addressee.
func TestNatural9OfferFilingPreconditions(t *testing.T) {
	b9, ref9 := natural9Fixture(t, 2)
	evaluate := func(o strategy.NaturalLabourOfferObservation) (strategy.NaturalLabourOfferChoice, error) {
		return b9.EvaluateLabourOffer9(o)
	}
	view := natural9OfferView(t, 2, ref9, 5)
	got, err := evaluate(view)
	if err != nil {
		t.Fatal(err)
	}
	if want := (strategy.NaturalLabourOfferChoice{Kind: strategy.NaturalLabourOfferFile, Addressee: 3,
		ObservedVersion: 9, Ref: ref9, Evaluated: 7, Fallback: strategy.NaturalNoFallback}); got != want {
		t.Fatalf("baseline filing drifted: %+v, want %+v", got, want)
	}
	// The filing preconditions, one boundary at a time.
	for _, tc := range []struct {
		name  string
		edit  func(o *strategy.NaturalLabourOfferObservation)
		files bool
	}{
		{"dead actor waits", func(o *strategy.NaturalLabourOfferObservation) { o.Energy = sim.IntegerValue(0) }, false},
		{"held bag waits", func(o *strategy.NaturalLabourOfferObservation) { o.BagUnits = sim.IntegerValue(1) }, false},
		{"pending offer waits", func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestPending, LastOfferHour: 4, LastReplyHour: -1}
		}, false},
		{"accepted offer waits", func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestAccepted, LastOfferHour: 4, LastReplyHour: 4}
		}, false},
		{"same-hour filing waits", func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestRefused, LastOfferHour: 5, LastReplyHour: 5}
		}, false},
		{"refused offer refiles", func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestRefused, LastOfferHour: 4, LastReplyHour: 4}
		}, true},
		{"expired offer refiles", func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestExpired, LastOfferHour: 3, LastReplyHour: -1}
		}, true},
		{"cleared contract refiles", func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestIdle, LastOfferHour: 3, LastReplyHour: 3}
		}, true},
	} {
		o := view
		tc.edit(&o)
		got, err := evaluate(o)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.files && got.Kind != strategy.NaturalLabourOfferFile {
			t.Fatalf("%s must file: %+v", tc.name, got)
		}
		if !tc.files && got.Kind != strategy.NaturalLabourOfferWait {
			t.Fatalf("%s must wait: %+v", tc.name, got)
		}
	}
	// An empty same-patch roster never files.
	alone := view
	alone.PatchMates = nil
	if got, err := evaluate(alone); err != nil || got.Kind != strategy.NaturalLabourOfferWait || got.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("empty roster: %+v %v", got, err)
	}
	// The wraparound rotation: actor 8 wraps to 1; actor 1 files to 2.
	for _, tc := range []struct {
		actor, want sim.EntityID
	}{
		{1, 2}, {8, 1},
	} {
		b, ref := natural9Fixture(t, tc.actor)
		o := natural9OfferView(t, tc.actor, ref, 5)
		got, err := b.EvaluateLabourOffer9(o)
		if err != nil || got.Kind != strategy.NaturalLabourOfferFile || got.Addressee != tc.want {
			t.Fatalf("rotation for actor %d: %+v %v", tc.actor, got, err)
		}
	}
	// Order independence and determinism: shuffled rosters decide
	// identically, and repeats are stable.
	base, err := evaluate(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, perm := range [][]sim.EntityID{
		{view.PatchMates[6], view.PatchMates[2], view.PatchMates[0], view.PatchMates[5], view.PatchMates[1], view.PatchMates[4], view.PatchMates[3]},
		{8, 7, 6, 5, 4, 3, 1},
	} {
		shuffled := view
		shuffled.PatchMates = perm
		got, err := evaluate(shuffled)
		if err != nil || got != base {
			t.Fatalf("shuffled roster changed the filing: %+v vs %+v (%v)", got, base, err)
		}
	}
	// Structural rejections, verbatim birth-request classes.
	for name, edit := range map[string]func(o *strategy.NaturalLabourOfferObservation){
		"self mate":      func(o *strategy.NaturalLabourOfferObservation) { o.PatchMates = []sim.EntityID{1, 2} },
		"duplicate mate": func(o *strategy.NaturalLabourOfferObservation) { o.PatchMates = []sim.EntityID{1, 1} },
		"foreign mate":   func(o *strategy.NaturalLabourOfferObservation) { o.PatchMates = []sim.EntityID{1001} },
		"slot mate":      func(o *strategy.NaturalLabourOfferObservation) { o.PatchMates = []sim.EntityID{2001} },
		"zero mate":      func(o *strategy.NaturalLabourOfferObservation) { o.PatchMates = []sim.EntityID{0} },
		"cross-patch mate": func(o *strategy.NaturalLabourOfferObservation) {
			o.PatchMates = []sim.EntityID{1, 9}
		},
		"bad status":    func(o *strategy.NaturalLabourOfferObservation) { o.OwnOffer.Status = strategy.NaturalRequestStatus(5) },
		"future filing": func(o *strategy.NaturalLabourOfferObservation) { o.OwnOffer.LastOfferHour = 6 },
		"future reply":  func(o *strategy.NaturalLabourOfferObservation) { o.OwnOffer.LastReplyHour = 6 },
		"unfiled refused": func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestRefused, LastOfferHour: -1, LastReplyHour: 4}
		},
		"unreplied accepted": func(o *strategy.NaturalLabourOfferObservation) {
			o.OwnOffer = strategy.NaturalLabourOfferGuard{Status: strategy.NaturalRequestAccepted, LastOfferHour: 4, LastReplyHour: -1}
		},
		"bad patch": func(o *strategy.NaturalLabourOfferObservation) { o.HomePatch = 1002 },
		"foreign ref": func(o *strategy.NaturalLabourOfferObservation) {
			o.Ref = strategy.FrozenNaturalPolicy7b().Ref
		},
		"wrong actor": func(o *strategy.NaturalLabourOfferObservation) { o.Actor = 3 },
	} {
		o := view
		edit(&o)
		if _, err := evaluate(o); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	// Out-of-range own values degrade to InvalidSelf, verbatim birth- and
	// claim-cascade classes.
	for name, edit := range map[string]func(o *strategy.NaturalLabourOfferObservation){
		"bad energy": func(o *strategy.NaturalLabourOfferObservation) { o.Energy = sim.IntegerValue(13) },
		"bad bag":    func(o *strategy.NaturalLabourOfferObservation) { o.BagUnits = sim.IntegerValue(2) },
	} {
		o := view
		edit(&o)
		got, err := evaluate(o)
		if err != nil || got.Kind != strategy.NaturalLabourOfferWait || got.Fallback != strategy.NaturalInvalidSelf {
			t.Fatalf("%s must degrade to InvalidSelf: %+v %v", name, got, err)
		}
	}
}

// TestNatural9EmployerDecisionFloor pins the rule-517 employer decision: the
// derived accept floor (own stock >= 4 = R*+2, own k+wip < 8), the typed
// refusal reasons in the settlement's clause order, the ascending-worker
// determinism, the one-decision-per-actor-hour guard, and structural
// rejection of foreign, stale, or duplicated pending offers.
func TestNatural9EmployerDecisionFloor(t *testing.T) {
	b9, ref9 := natural9Fixture(t, 1)
	evaluate := func(o strategy.NaturalLabourDecisionObservation) (strategy.NaturalLabourDecisionChoice, error) {
		return b9.EvaluateLabourDecision9(o)
	}
	// The stock boundary: 3 refused (survival stock), 4 accepted.
	low := natural9DecisionView(1, 2, ref9, 5)
	low.GranaryStock = sim.IntegerValue(3)
	got, err := evaluate(low)
	if err != nil {
		t.Fatal(err)
	}
	if want := (strategy.NaturalLabourDecisionChoice{Kind: strategy.NaturalLabourDecisionRefuse, Worker: 2,
		Reason: strategy.NaturalLabourReasonFloorStock, ObservedVersion: 9, Ref: ref9, Evaluated: 1,
		Fallback: strategy.NaturalNoFallback}); got != want {
		t.Fatalf("stock 3 must refuse on the floor: %+v, want %+v", got, want)
	}
	high := natural9DecisionView(1, 2, ref9, 5)
	got, err = evaluate(high)
	if err != nil {
		t.Fatal(err)
	}
	if want := (strategy.NaturalLabourDecisionChoice{Kind: strategy.NaturalLabourDecisionAccept, Worker: 2,
		ObservedVersion: 9, Ref: ref9, Evaluated: 1, Fallback: strategy.NaturalNoFallback}); got != want {
		t.Fatalf("stock 4 must accept: %+v, want %+v", got, want)
	}
	// The KMax boundary: k+wip 7 accepts, 8 refuses.
	sat := natural9DecisionView(1, 2, ref9, 5)
	sat.Capital = sim.IntegerValue(7)
	sat.Wip = sim.IntegerValue(1)
	if got, err := evaluate(sat); err != nil || got.Kind != strategy.NaturalLabourDecisionRefuse ||
		got.Reason != strategy.NaturalLabourReasonFloorKMax || got.Worker != 2 || got.Evaluated != 1 {
		t.Fatalf("k+wip 8 must refuse on KMax: %+v %v", got, err)
	}
	edge := natural9DecisionView(1, 2, ref9, 5)
	edge.Capital = sim.IntegerValue(7)
	if got, err := evaluate(edge); err != nil || got.Kind != strategy.NaturalLabourDecisionAccept {
		t.Fatalf("k+wip 7 must accept: %+v %v", got, err)
	}
	// The refusal clause order: stock first, then KMax.
	both := natural9DecisionView(1, 2, ref9, 5)
	both.GranaryStock = sim.IntegerValue(3)
	both.Capital = sim.IntegerValue(8)
	if got, err := evaluate(both); err != nil || got.Reason != strategy.NaturalLabourReasonFloorStock {
		t.Fatalf("the stock clause must dominate: %+v %v", got, err)
	}
	// Ascending-worker determinism with several pending offers; the input
	// order must not matter; refusals name the lowest worker.
	multi := natural9DecisionView(1, 5, ref9, 5)
	multi.Pending = []strategy.NaturalLabourPendingView{
		{Worker: 5, Addressee: 1, Hour: 5},
		{Worker: 3, Addressee: 1, Hour: 5},
		{Worker: 2017, Addressee: 1, Hour: 5},
	}
	got, err = evaluate(multi)
	if err != nil || got.Kind != strategy.NaturalLabourDecisionAccept || got.Worker != 3 || got.Evaluated != 1 {
		t.Fatalf("first ascending acceptable worker must win: %+v %v", got, err)
	}
	reversed := multi
	reversed.Pending = []strategy.NaturalLabourPendingView{multi.Pending[2], multi.Pending[0], multi.Pending[1]}
	if again, err := evaluate(reversed); err != nil || again != got {
		t.Fatalf("input order must not change the decision: %+v vs %+v (%v)", again, got, err)
	}
	refusing := multi
	refusing.GranaryStock = sim.IntegerValue(3)
	got, err = evaluate(refusing)
	if err != nil || got.Kind != strategy.NaturalLabourDecisionRefuse || got.Worker != 3 || got.Evaluated != 3 ||
		got.Reason != strategy.NaturalLabourReasonFloorStock {
		t.Fatalf("an unacceptable floor must refuse the lowest worker: %+v %v", got, err)
	}
	// The one-decision-per-actor-hour guard and the dead/empty silences.
	decided := natural9DecisionView(1, 2, ref9, 5)
	decided.OwnLastDecisionHour = 5
	if got, err := evaluate(decided); err != nil || got.Kind != strategy.NaturalLabourDecisionNone {
		t.Fatalf("second decision in one hour: %+v %v", got, err)
	}
	previous := natural9DecisionView(1, 2, ref9, 5)
	previous.OwnLastDecisionHour = 4
	if got, err := evaluate(previous); err != nil || got.Kind != strategy.NaturalLabourDecisionAccept {
		t.Fatalf("a decision guarded only in a previous hour must decide: %+v %v", got, err)
	}
	dead := natural9DecisionView(1, 2, ref9, 5)
	dead.Energy = sim.IntegerValue(0)
	if got, err := evaluate(dead); err != nil || got.Kind != strategy.NaturalLabourDecisionNone || got.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("a dead employer never decides: %+v %v", got, err)
	}
	quiet := natural9DecisionView(1, 2, ref9, 5)
	quiet.Pending = nil
	if got, err := evaluate(quiet); err != nil || got.Kind != strategy.NaturalLabourDecisionNone || got.Fallback != strategy.NaturalNoCandidate {
		t.Fatalf("no pending offers: %+v %v", got, err)
	}
	// Structural rejections: foreign, stale, misaddressed, duplicated, and
	// out-of-shape inputs.
	for name, edit := range map[string]func(o *strategy.NaturalLabourDecisionObservation){
		"self worker":    func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Worker = 1 },
		"foreign worker": func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Worker = 1001 },
		"slot worker":    func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Worker = 2001 },
		"zero worker":    func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Worker = 0 },
		"misaddressed":   func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Addressee = 2 },
		"stale hour":     func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Hour = 4 },
		"future hour":    func(o *strategy.NaturalLabourDecisionObservation) { o.Pending[0].Hour = 6 },
		"dup worker":     func(o *strategy.NaturalLabourDecisionObservation) { o.Pending = append(o.Pending, o.Pending[0]) },
		"future decision": func(o *strategy.NaturalLabourDecisionObservation) {
			o.OwnLastDecisionHour = 6
		},
		"early decision": func(o *strategy.NaturalLabourDecisionObservation) { o.OwnLastDecisionHour = -2 },
		"foreign ref":    func(o *strategy.NaturalLabourDecisionObservation) { o.Ref = strategy.FrozenNaturalPolicy7b().Ref },
		"wrong actor":    func(o *strategy.NaturalLabourDecisionObservation) { o.Actor = 2 },
		"out of horizon": func(o *strategy.NaturalLabourDecisionObservation) { o.Hour = 168 },
		"negative hour":  func(o *strategy.NaturalLabourDecisionObservation) { o.Hour = -1 },
	} {
		o := natural9DecisionView(1, 2, ref9, 5)
		edit(&o)
		if _, err := evaluate(o); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	// Out-of-range own values degrade to InvalidSelf, verbatim.
	for name, edit := range map[string]func(o *strategy.NaturalLabourDecisionObservation){
		"bad energy":  func(o *strategy.NaturalLabourDecisionObservation) { o.Energy = sim.IntegerValue(-1) },
		"bad stock":   func(o *strategy.NaturalLabourDecisionObservation) { o.GranaryStock = sim.IntegerValue(1345) },
		"bad capital": func(o *strategy.NaturalLabourDecisionObservation) { o.Capital = sim.IntegerValue(9) },
		"bad wip":     func(o *strategy.NaturalLabourDecisionObservation) { o.Wip = sim.IntegerValue(2) },
	} {
		o := natural9DecisionView(1, 2, ref9, 5)
		edit(&o)
		got, err := evaluate(o)
		if err != nil || got.Kind != strategy.NaturalLabourDecisionNone || got.Fallback != strategy.NaturalInvalidSelf {
			t.Fatalf("%s must degrade to InvalidSelf: %+v %v", name, got, err)
		}
	}
	// Determinism: repeated evaluation returns the identical choice.
	fresh := natural9DecisionView(1, 2, ref9, 5)
	first, err := evaluate(fresh)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if again, err := evaluate(fresh); err != nil || again != first {
			t.Fatalf("repeated decision drifted: %+v vs %+v (%v)", again, first, err)
		}
	}
}

// TestNatural9BudgetExhaustion pins the shared budget semantics for
// natural/9: the frozen ceiling is 32/32, same-ref budget variants register,
// every list input degrades all-or-nothing to the typed
// NaturalBudgetExhausted fallback — never a prefix — and the frozen budget
// never truncates a legal maximal set.
func TestNatural9BudgetExhaustion(t *testing.T) {
	// A one-item budget cannot take a two-item roster or pending list.
	limited := strategy.FrozenNaturalPolicy9()
	limited.Budget = strategy.NaturalBudget{Candidates: 1, Evaluations: 1}
	lr := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := lr.RegisterFrozenNatural(limited); err != nil {
		t.Fatal(err)
	}
	lb, lref := func() (*strategy.NaturalBound, strategy.NaturalRef) {
		t.Helper()
		b, err := lr.Bind(strategy.NaturalBinding{Actor: 1, Ref: limited.Ref})
		if err != nil {
			t.Fatal(err)
		}
		return b, limited.Ref
	}()
	request := natural9OfferView(t, 1, lref, 5)
	request.PatchMates = append(request.PatchMates, 2017) // 8 mates > 1
	if got, err := lb.EvaluateLabourOffer9(request); err != nil ||
		got != (strategy.NaturalLabourOfferChoice{Kind: strategy.NaturalLabourOfferWait, ObservedVersion: 9,
			Ref: lref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing offer budget: %+v %v", got, err)
	}
	request.PatchMates = []sim.EntityID{2}
	if got, err := lb.EvaluateLabourOffer9(request); err != nil || got.Kind != strategy.NaturalLabourOfferFile || got.Addressee != 2 {
		t.Fatalf("bounded offer set: %+v %v", got, err)
	}
	reply := natural9DecisionView(1, 2, lref, 5)
	reply.Pending = append(reply.Pending, strategy.NaturalLabourPendingView{Worker: 3, Addressee: 1, Hour: 5})
	if got, err := lb.EvaluateLabourDecision9(reply); err != nil ||
		got != (strategy.NaturalLabourDecisionChoice{Kind: strategy.NaturalLabourDecisionNone, ObservedVersion: 9,
			Ref: lref, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("all-or-nothing decision budget: %+v %v", got, err)
	}
	// The frozen 32/32 budget covers every maximal legal set: a 32-mate
	// roster files and a 32-pending list decides; the 33rd entry is a typed
	// budget event, never a truncation.
	b9, ref9 := natural9Fixture(t, 1)
	big := natural9OfferView(t, 1, ref9, 5)
	for id := sim.EntityID(2017); len(big.PatchMates) < 32; id++ {
		big.PatchMates = append(big.PatchMates, id)
	}
	sort.Slice(big.PatchMates, func(i, j int) bool { return big.PatchMates[i] < big.PatchMates[j] })
	got, err := b9.EvaluateLabourOffer9(big)
	if err != nil || got.Kind != strategy.NaturalLabourOfferFile || got.Addressee != 2 || got.Evaluated != 32 ||
		got.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("the frozen budget must never truncate the legal roster: %+v %v", got, err)
	}
	big.PatchMates = append(big.PatchMates, 2049)
	if got, err := b9.EvaluateLabourOffer9(big); err != nil ||
		got != (strategy.NaturalLabourOfferChoice{Kind: strategy.NaturalLabourOfferWait, ObservedVersion: 9,
			Ref: ref9, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("over-budget roster must be a typed budget event: %+v %v", got, err)
	}
	full := natural9DecisionView(1, 2, ref9, 5)
	for id := sim.EntityID(3); id <= 8; id++ {
		full.Pending = append(full.Pending, strategy.NaturalLabourPendingView{Worker: id, Addressee: 1, Hour: 5})
	}
	for id := sim.EntityID(2017); len(full.Pending) < 32; id++ {
		full.Pending = append(full.Pending, strategy.NaturalLabourPendingView{Worker: id, Addressee: 1, Hour: 5})
	}
	if got, err := b9.EvaluateLabourDecision9(full); err != nil || got.Kind != strategy.NaturalLabourDecisionAccept ||
		got.Worker != 2 || got.Evaluated != 1 || got.Fallback != strategy.NaturalNoFallback {
		t.Fatalf("the frozen budget must never truncate the legal pending set: %+v %v", got, err)
	}
	full.Pending = append(full.Pending, strategy.NaturalLabourPendingView{Worker: 2049, Addressee: 1, Hour: 5})
	if got, err := b9.EvaluateLabourDecision9(full); err != nil ||
		got != (strategy.NaturalLabourDecisionChoice{Kind: strategy.NaturalLabourDecisionNone, ObservedVersion: 9,
			Ref: ref9, Fallback: strategy.NaturalBudgetExhausted}) {
		t.Fatalf("over-budget pending list must be a typed budget event: %+v %v", got, err)
	}
	// A zero budget cannot register.
	zeroed := strategy.FrozenNaturalPolicy9()
	zeroed.Budget.Evaluations = 0
	if err := strategy.NewNaturalRegistry(naturalTestPatches).RegisterFrozenNatural(zeroed); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("zero budget registered: %v", err)
	}
}

// natural9WorldState builds a V7-valid founder state at the given wage-side
// margins (granary stock backed by its cultivated ledger, capital with its
// G2b base) and the idle labour rows.
func natural9WorldState(t *testing.T, actor sim.EntityID, stock, capital int64) world.NaturalActorState {
	t.Helper()
	a, err := world.NaturalFounderState(naturalTestPatches, actor, world.NaturalFounderLifetimeMax)
	if err != nil {
		t.Fatal(err)
	}
	a.Offer = world.NaturalIdleLabourOfferState()
	a.Book = world.NaturalIdleLabourBookState()
	a.Worksite = world.NaturalWorksiteState{Capital: capital, PointsCreated: capital, InvestedUnits: 2 * capital, LastBuildHour: 1}
	a.Granary = world.NaturalGranaryState{Stock: stock, CultivatedTotal: stock, LastStoredMealHour: world.NaturalNeverHour}
	a.Body.Energy = 8
	a.Body.BasalSpent = 3 // G3: 11 initial = 8 energy + 3 basal spent
	return a
}

// natural9Guard projects a world offer row onto the policy guard.
func natural9Guard(o world.NaturalLabourOfferState) strategy.NaturalLabourOfferGuard {
	return strategy.NaturalLabourOfferGuard{
		Status:        strategy.NaturalRequestStatus(o.Status),
		LastOfferHour: o.LastOfferHour,
		LastReplyHour: o.LastReplyHour,
	}
}

// TestNatural9LifecycleConformance drives the frozen world transitions
// (rules 516/517/518/519) against the policy decisions and proves the
// lifecycle conformance the frozen design pins: a pending offer lives only
// within its filing hour, accept/refuse are same-hour, an accepted contract
// has exactly one delivery hour, the settled worker cannot refile until the
// clearing pulse, and every policy verdict is world-admissible.
func TestNatural9LifecycleConformance(t *testing.T) {
	const (
		m    = world.NaturalMechanismV7
		hour = 5
	)
	b9, ref9 := natural9Fixture(t, 2)
	employer := natural9WorldState(t, 3, 5, 2) // stock 5 >= 4, k+wip 2 < 8; the rotation addressee of worker 2
	worker := natural9WorldState(t, 2, 0, 0)

	// The worker files at h: the policy's addressee is the employer; the
	// world's rule 516 admits exactly that filing.
	view := natural9OfferView(t, 2, ref9, hour)
	filing, err := b9.EvaluateLabourOffer9(view)
	if err != nil || filing.Kind != strategy.NaturalLabourOfferFile || filing.Addressee != 3 {
		t.Fatalf("worker filing: %+v %v", filing, err)
	}
	offered, err := world.NaturalLabourOffer(m, naturalTestPatches, hour, 2, filing.Addressee, 1001, worker)
	if err != nil {
		t.Fatal(err)
	}
	if offered.Offer.Status != world.NaturalRequestPending || offered.Offer.Hour != hour ||
		offered.Offer.LastOfferHour != hour || offered.Offer.LastReplyHour != world.NaturalNeverHour {
		t.Fatalf("filed row shape: %+v", offered.Offer)
	}
	// The open offer blocks refiling in the same hour.
	view.OwnOffer = natural9Guard(offered.Offer)
	if again, err := b9.EvaluateLabourOffer9(view); err != nil || again.Kind != strategy.NaturalLabourOfferWait {
		t.Fatalf("refile over an open offer: %+v %v", again, err)
	}
	// The employer's policy decision at h accepts (the floor holds), and
	// the world's rule 517 commits exactly that accept.
	acceptor, acceptorRef := natural9Fixture(t, 3)
	decision := natural9DecisionView(3, 2, acceptorRef, hour)
	choice, err := acceptor.EvaluateLabourDecision9(decision)
	if err != nil || choice.Kind != strategy.NaturalLabourDecisionAccept || choice.Worker != 2 {
		t.Fatalf("employer accept: %+v %v", choice, err)
	}
	engagedEmployer, engagedWorker, verdict, err := world.NaturalLabourDecision(m, naturalTestPatches, hour, 3, 2, true, employer, offered)
	if err != nil || verdict != world.NaturalReplyAccept {
		t.Fatalf("world accept: %v %+v", err, verdict)
	}
	if engagedWorker.Offer.Status != world.NaturalRequestAccepted || engagedWorker.Offer.LastReplyHour != hour ||
		engagedEmployer.Book.EngagedWorker != 2 || engagedEmployer.Book.EngagedHour != hour {
		t.Fatalf("engaged rows: %+v %+v", engagedWorker.Offer, engagedEmployer.Book)
	}
	// Same-hour refusal decisions stay admissible: a worker's offer goes to
	// its rotation addressee (worker 1 files to 2), the policy refuses when
	// the floor fails, and the world commits the typed refusal.
	rejector, rejectorRef := natural9Fixture(t, 2)
	poor := natural9WorldState(t, 2, 3, 2) // stock 3 < 4: below the wage floor
	w1, err := world.NaturalLabourOffer(m, naturalTestPatches, hour, 1, 2, 1001, natural9WorldState(t, 1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	refuseView := natural9DecisionView(2, 1, rejectorRef, hour)
	refuseView.GranaryStock = sim.IntegerValue(3)
	if c, err := rejector.EvaluateLabourDecision9(refuseView); err != nil ||
		c.Kind != strategy.NaturalLabourDecisionRefuse || c.Reason != strategy.NaturalLabourReasonFloorStock {
		t.Fatalf("floor refusal: %+v %v", c, err)
	}
	if _, _, v, err := world.NaturalLabourDecision(m, naturalTestPatches, hour, 2, 1, false, poor, w1); err != nil || v != world.NaturalReplyRefuse {
		t.Fatalf("world refuse: %v %+v", err, v)
	}
	// The delivery hour: the worker's accepted row is due exactly at h+1;
	// the claim stays verbatim 7b (gather), the bag decision delivers, and
	// the world's rule 518 settles the wage.
	deliveryHour := hour + 1
	gatherSlot, err := world.NaturalSlotID(naturalTestPatches, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	slots := [world.NaturalSlotsPerPatch]world.NaturalSlotState{}
	slots[1] = world.NaturalSlotState{Stock: 1}
	gatheredSlot, held, err := world.NaturalGatherFor(m, naturalTestPatches, 1001, gatherSlot, 2, deliveryHour, slots[1], engagedWorker)
	if err != nil || held.Bag.Units != 1 {
		t.Fatalf("contract gather: %v %+v", err, held.Bag)
	}
	slots[1] = gatheredSlot
	claim := natural9Observation(t, 2, ref9, deliveryHour)
	claim.OwnOffer = natural9Guard(held.Offer)
	claim.HomeSlots[1].Stock = sim.IntegerValue(1)
	if got, err := b9.Evaluate9(claim); err != nil || got.Kind != strategy.NaturalGather || got.TargetSlot != gatherSlot {
		t.Fatalf("the claim stays verbatim 7b under a due contract: %+v %v", got, err)
	}
	due := claim
	due.BagUnits = sim.IntegerValue(1)
	deliver, err := b9.Evaluate9(due)
	if err != nil || deliver.Kind != strategy.NaturalDeliver {
		t.Fatalf("the delivery hour must deliver: %+v %v", deliver, err)
	}
	settledWorker, settledEmployer, _, result, err := world.NaturalLabourSettle(m, naturalTestPatches, deliveryHour, 2, 3, held, engagedEmployer, slots)
	if err != nil {
		t.Fatal(err)
	}
	if result.PointsCompleted != 0 || settledWorker.Granary.Stock != 1 || settledWorker.Granary.WagesEarned != 1 ||
		settledEmployer.Granary.Stock != 4 || settledEmployer.Granary.WagesPaid != 1 ||
		settledEmployer.Worksite.Wip != 1 || settledWorker.Bag.Units != 0 {
		t.Fatalf("settlement deltas: %+v %+v %+v", result, settledWorker.Granary, settledEmployer)
	}
	// The settled row stays accepted until the clearing pulse, so the
	// worker cannot refile in the settlement hour, and the delivery window
	// is exactly one hour: at h+2 the row is no longer due.
	view.OwnOffer = natural9Guard(settledWorker.Offer)
	view.Hour = deliveryHour
	if again, err := b9.EvaluateLabourOffer9(view); err != nil || again.Kind != strategy.NaturalLabourOfferWait {
		t.Fatalf("refile in the settlement hour: %+v %v", again, err)
	}
	notDue := natural9Observation(t, 2, ref9, deliveryHour+1)
	notDue.OwnOffer = natural9Guard(settledWorker.Offer)
	notDue.BagUnits = sim.IntegerValue(1)
	if got, err := b9.Evaluate9(notDue); err != nil || got.Kind != strategy.NaturalBuild {
		t.Fatalf("h+2 must not be a delivery hour: %+v %v", got, err)
	}
	// The h+2 pulse clears the finished contract: the offer row clears to
	// idle-with-audit and the book clears, and the worker can refile.
	clearedOffer, moved, err := world.NaturalLabourOfferExpire(deliveryHour+1, settledWorker.Offer)
	if err != nil || !moved || clearedOffer.Status != world.NaturalRequestIdle {
		t.Fatalf("rule-519 offer clear: %+v %v %v", clearedOffer, moved, err)
	}
	clearedBook, moved, err := world.NaturalLabourBookClear(deliveryHour+1, settledEmployer.Book)
	if err != nil || !moved || clearedBook.EngagedWorker != 0 {
		t.Fatalf("rule-519 book clear: %+v %v %v", clearedBook, moved, err)
	}
	view.OwnOffer = natural9Guard(clearedOffer)
	view.Hour = deliveryHour + 1
	if again, err := b9.EvaluateLabourOffer9(view); err != nil || again.Kind != strategy.NaturalLabourOfferFile || again.Addressee != 3 {
		t.Fatalf("refile after the clearing pulse: %+v %v", again, err)
	}
	// The unanswered pending offer expires at the next pulse and the worker
	// refiles: a pending offer lives only within its filing hour.
	pendingRow, err := world.NaturalLabourOffer(m, naturalTestPatches, hour, 4, 1, 1001, natural9WorldState(t, 4, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	expired, moved, err := world.NaturalLabourOfferExpire(hour+1, pendingRow.Offer)
	if err != nil || !moved || expired.Status != world.NaturalRequestExpired {
		t.Fatalf("rule-519 expiry: %+v %v %v", expired, moved, err)
	}
	view4 := natural9OfferView(t, 4, ref9, hour+1)
	view4.OwnOffer = natural9Guard(expired)
	refiler, _ := natural9Fixture(t, 4)
	if again, err := refiler.EvaluateLabourOffer9(view4); err != nil || again.Kind != strategy.NaturalLabourOfferFile {
		t.Fatalf("refile after expiry: %+v %v", again, err)
	}
	// The one-decision-per-actor-hour guard: the employer's committed
	// decision hour blocks a second policy decision in the same hour.
	booked := natural9DecisionView(3, 2, acceptorRef, hour)
	booked.OwnLastDecisionHour = int64(hour)
	if c, err := acceptor.EvaluateLabourDecision9(booked); err != nil || c.Kind != strategy.NaturalLabourDecisionNone {
		t.Fatalf("second decision in the committed hour: %+v %v", c, err)
	}
}

// TestNatural9PrivacyFieldSetsAndEnumRuling reflects every new type and
// asserts the exact field lists, scans the counterparty-facing views for
// foreign-state field names, and proves the enum ruling: the new action and
// reason values carry no String method and no textual rendering anywhere.
func TestNatural9PrivacyFieldSetsAndEnumRuling(t *testing.T) {
	// The embedded base is the frozen v5 observation, whole: no field
	// added, removed, or retyped.
	obs := reflect.TypeOf(strategy.NaturalLabourObservation{})
	if obs.NumField() != 2 || !obs.Field(0).Anonymous || obs.Field(0).Name != "NaturalObservation" ||
		obs.Field(0).Type != reflect.TypeOf(strategy.NaturalObservation{}) ||
		obs.Field(1).Name != "OwnOffer" || obs.Field(1).Type != reflect.TypeOf(strategy.NaturalLabourOfferGuard{}) {
		t.Fatalf("natural/9 claim view drifted: %+v", obs)
	}
	want := map[reflect.Type][]string{
		reflect.TypeOf(strategy.NaturalObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "Hunger", "BagUnits",
			"BagSource", "Capital", "Wip", "GranaryStock", "InvestedUnits", "LastBuildHour", "LastStoredMealHour", "LastDenial", "HomeSlots"},
		reflect.TypeOf(strategy.NaturalLabourOfferGuard{}): {"Status", "LastOfferHour", "LastReplyHour"},
		reflect.TypeOf(strategy.NaturalLabourOfferObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "HomePatch", "Energy",
			"BagUnits", "OwnOffer", "PatchMates"},
		reflect.TypeOf(strategy.NaturalLabourOfferChoice{}): {"Kind", "Addressee", "ObservedVersion", "Ref", "Evaluated", "Fallback"},
		reflect.TypeOf(strategy.NaturalLabourPendingView{}): {"Worker", "Addressee", "Hour"},
		reflect.TypeOf(strategy.NaturalLabourDecisionObservation{}): {"Actor", "Ref", "Hour", "WorldVersion", "Energy", "GranaryStock",
			"Capital", "Wip", "OwnLastDecisionHour", "Pending"},
		reflect.TypeOf(strategy.NaturalLabourDecisionChoice{}): {"Kind", "Worker", "Reason", "ObservedVersion", "Ref", "Evaluated", "Fallback"},
	}
	// The counterparty-facing types must carry no foreign private state:
	// the pending view is identity-and-hour only, the filing observation has
	// no surplus margins (no granary, capital, or wip — filing needs none),
	// and no choice carries a claim, price, wage, or terms field.
	forbiddenByType := map[reflect.Type][]string{
		reflect.TypeOf(strategy.NaturalLabourOfferObservation{}): {"Granary", "Capital", "Wip", "Hunger", "Slot", "Kin",
			"Reported", "Claim", "Price", "Wage"},
		reflect.TypeOf(strategy.NaturalLabourOfferChoice{}): {"Reported", "Claim", "Price", "Wage", "Terms", "Rate"},
		reflect.TypeOf(strategy.NaturalLabourPendingView{}): {"Energy", "Hunger", "Granary", "Capital", "Wip", "Bag", "Slot",
			"Tenure", "Owner", "Mode", "Debt", "Spoil", "Kin", "Reported", "Claim", "Price", "Wage", "Locus", "Genome",
			"Death", "Died", "Stream", "Allocator", "Book"},
	}
	for typ, fields := range want {
		got := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			got = append(got, typ.Field(i).Name)
		}
		if !reflect.DeepEqual(got, fields) {
			t.Fatalf("%s field set drifted: %v", typ, got)
		}
		for _, forbidden := range forbiddenByType[typ] {
			if _, present := typ.FieldByName(forbidden); present {
				t.Fatalf("%s leaked %s", typ, forbidden)
			}
		}
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			for _, word := range forbiddenByType[typ] {
				if strings.Contains(name, word) {
					t.Fatalf("%s field %s carries forbidden %s data", typ, name, word)
				}
			}
		}
	}
	// The enum ruling: the new action values carry no methods at all — no
	// String, no textual rendering — and NaturalDeliver appends exactly one
	// past NaturalProvision.
	for _, value := range []any{
		strategy.NaturalDeliver, strategy.NaturalLabourOfferWait, strategy.NaturalLabourOfferFile,
		strategy.NaturalLabourDecisionNone, strategy.NaturalLabourDecisionAccept, strategy.NaturalLabourDecisionRefuse,
		strategy.NaturalLabourReasonNone, strategy.NaturalLabourReasonFloorStock, strategy.NaturalLabourReasonFloorKMax,
	} {
		if n := reflect.TypeOf(value).NumMethod(); n != 0 {
			t.Fatalf("%v carries %d methods; the enum ruling allows none", value, n)
		}
	}
	if strategy.NaturalDeliver != strategy.NaturalProvision+1 {
		t.Fatalf("NaturalDeliver must append one past NaturalProvision: %d vs %d", strategy.NaturalDeliver, strategy.NaturalProvision+1)
	}
	if strategy.NaturalLabourOfferWait != 0 || strategy.NaturalLabourOfferFile != 1 {
		t.Fatal("labour offer action values drifted")
	}
	if strategy.NaturalLabourDecisionNone != 0 || strategy.NaturalLabourDecisionAccept != 1 || strategy.NaturalLabourDecisionRefuse != 2 {
		t.Fatal("labour decision action values drifted")
	}
	if strategy.NaturalLabourReasonNone != 0 || strategy.NaturalLabourReasonFloorStock != 1 || strategy.NaturalLabourReasonFloorKMax != 2 {
		t.Fatal("labour decision reason values drifted")
	}
	// A natural/9 bound never evaluates another actor's view at any of its
	// five decision points.
	_, ref9 := natural9Fixture(t, 1)
	b2, _ := natural9Fixture(t, 2)
	o1 := natural9Observation(t, 1, ref9, 5)
	if _, err := b2.Evaluate9(o1); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 9 bound evaluated another actor's claim view")
	}
	if _, err := b2.EvaluateLabourOffer9(natural9OfferView(t, 1, ref9, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 9 bound evaluated another actor's offer view")
	}
	if _, err := b2.EvaluateLabourDecision9(natural9DecisionView(1, 3, ref9, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 9 bound evaluated another actor's decision view")
	}
	if _, err := b2.EvaluateBirthRequest9(naturalRequestView(t, 1, ref9, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 9 bound evaluated another actor's request view")
	}
	if _, err := b2.EvaluateBirthReply9(naturalReplyView(1, 3, ref9, 5)); !errors.Is(err, strategy.ErrInvalidNaturalObservation) {
		t.Fatal("a 9 bound evaluated another actor's reply view")
	}
	b1, _ := natural9Fixture(t, 1)
	if b1.Policy() != strategy.FrozenNaturalPolicy9() {
		t.Fatal("accessor exposed non-frozen content")
	}
}
