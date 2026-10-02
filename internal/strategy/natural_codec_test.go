package strategy_test

import (
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestNaturalCanonicalCodecRoundtripAndDrift(t *testing.T) {
	p := strategy.FrozenNaturalPolicy()
	wire, err := strategy.EncodeNaturalPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	// Canonical full content: format, ref identity, and budget, fixed order,
	// exact framing (4 magic + 4 format + 1 len + 7 id + 4 version + 2
	// budget).
	const frozenWireLen = 4 + 4 + 1 + len("natural") + 4 + 1 + 1
	if len(wire) != frozenWireLen {
		t.Fatalf("wire length %d, want %d: %x", len(wire), frozenWireLen, wire)
	}
	again, err := strategy.EncodeNaturalPolicy(p)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatalf("encoding is not deterministic: %x %x %v", wire, again, err)
	}
	got, err := strategy.DecodeNaturalPolicy(wire)
	if err != nil || got != p {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyNaturalPolicy(wire, p); err != nil {
		t.Fatalf("frozen wire must verify: %v", err)
	}
	if sum, err := strategy.NaturalPolicyFingerprint(p); err != nil || sum != sha256.Sum256(wire) {
		t.Fatalf("fingerprint: %v %v", sum, err)
	}
	// Every single-byte mutation either fails decoding or fails registry
	// verification against the executable's frozen policy.
	for i := range wire {
		bad := append([]byte(nil), wire...)
		bad[i] ^= 0xff
		if _, err := strategy.DecodeNaturalPolicy(bad); err == nil {
			if err := strategy.VerifyNaturalPolicy(bad, p); err == nil {
				t.Fatalf("mutated byte %d accepted: %x", i, bad)
			}
		}
	}
	// Every edit clones first: none of these probes may corrupt the frozen
	// wire that later assertions in this test still decode.
	for name, edit := range map[string]func([]byte) []byte{
		"empty":     func([]byte) []byte { return nil },
		"truncated": func(b []byte) []byte { return b[:len(b)-1] },
		"trailing":  func(b []byte) []byte { return append(b, 0) },
		"magic":     func(b []byte) []byte { b = append([]byte(nil), b...); b[0] = 'X'; return b },
		"length":    func(b []byte) []byte { b = append([]byte(nil), b...); b[8] = 0; return b },
	} {
		if _, err := strategy.DecodeNaturalPolicy(edit(wire)); !errors.Is(err, strategy.ErrInvalidNaturalPolicyEncoding) {
			t.Fatalf("%s wire decoded: %v", name, err)
		}
		if err := strategy.VerifyNaturalPolicy(edit(wire), p); err == nil {
			t.Fatalf("%s wire verified", name)
		}
	}
	// A same-ref budget variant is a well-formed policy but can never be
	// restored against the frozen expectation: full-content comparison.
	variant := p
	variant.Budget = strategy.NaturalBudget{Candidates: 2, Evaluations: 2}
	variantWire, err := strategy.EncodeNaturalPolicy(variant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := strategy.DecodeNaturalPolicy(variantWire); err != nil || got != variant {
		t.Fatalf("variant roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyNaturalPolicy(variantWire, p); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("same-ref budget drift restored: %v", err)
	}
	if err := strategy.VerifyNaturalPolicy(wire, variant); !errors.Is(err, strategy.ErrNaturalPolicyMismatch) {
		t.Fatalf("frozen wire verified against variant: %v", err)
	}
	// Drifted expectations fail closed before any bytes are compared.
	drifted := p
	drifted.Ref.Version = 6
	if _, err := strategy.EncodeNaturalPolicy(drifted); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("drifted policy encoded: %v", err)
	}
	if err := strategy.VerifyNaturalPolicy(wire, drifted); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("drifted expectation accepted: %v", err)
	}
}

// The v5 magic AWNP is disjoint from every other policy wire in the
// executable: no earlier codec decodes it and DecodeNaturalPolicy decodes
// none of them, so no checkpoint section can be replayed as another.
func TestNaturalMagicDisjointFromOtherPolicyWires(t *testing.T) {
	v5, err := strategy.EncodeNaturalPolicy(strategy.FrozenNaturalPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if naturalMagic := string(v5[:4]); naturalMagic != "AWNP" {
		t.Fatalf("natural magic drifted: %q", naturalMagic)
	}
	// Every four-byte wire magic on disk (policy, checkpoint, journal,
	// history, snapshot, portable) as published witnesses of the
	// disjointness contract; the frozen design names the policy magics
	// explicitly (AWCP/AWRP/AWFP/AWSP/AWST/AWCS/AWKH/...).
	priorMagics := map[string]string{
		"capacity-policy":      "AWCP",
		"reproduction-policy":  "AWRP",
		"foodflow-policy":      "AWFP",
		"socialfood-policy":    "AWSP",
		"portable-strategy":    "AWST",
		"component-snapshot":   "AWCS",
		"kernel-history":       "AWKH",
		"capacity-checkpoint":  "AWC3",
		"capacity-bundle":      "AWCB",
		"capacity-journal":     "AWCJ",
		"capacity-strategy":    "AWCT",
		"foodflow-journal":     "AWFJ",
		"foodflow-checkpoint":  "AWFM",
		"foodflow-strategy":    "AWFS",
		"reproduction-ckpt":    "AWR4",
		"reproduction-journal": "AWRJ",
		"reproduction-strgy":   "AWRT",
		"socialfood-ckpt":      "AWS2",
		"socialfood-strategy":  "AWSF",
		"socialfood-journal":   "AWSJ",
		"survival-manifest":    "AWSM",
	}
	if _, collides := priorMagics["AWNP"]; collides {
		t.Fatal("the natural magic collides with a prior wire")
	}
	naturalMagic := string(v5[:4])
	for name, magic := range priorMagics {
		if naturalMagic == magic {
			t.Fatalf("natural magic collides with %s (%s)", name, magic)
		}
	}
	// No earlier policy codec accepts the v5 wire; the v5 codec accepts none
	// of theirs. The food-flow and portable-strategy encoders need bound
	// instances the frozen constructors do not expose here; their magics
	// above are the remaining disjointness witnesses.
	others := map[string][]byte{
		"capacity-v3": func() []byte {
			w, err := strategy.EncodeCapacityPolicy(strategy.FrozenCapacityPolicy())
			if err != nil {
				t.Fatal(err)
			}
			return w
		}(),
		"reproduction-v4": func() []byte {
			w, err := strategy.EncodeReproductionPolicy(strategy.FrozenReproductionPolicy())
			if err != nil {
				t.Fatal(err)
			}
			return w
		}(),
		"socialfood-v2": func() []byte {
			w, err := strategy.EncodeSocialFoodPolicy(strategy.FrozenSocialFoodPolicy(strategy.SocialFoodRef{ID: "social-v2", Version: 2}))
			if err != nil {
				t.Fatal(err)
			}
			return w
		}(),
	}
	for name, wire := range others {
		if string(wire[:4]) == naturalMagic {
			t.Fatalf("%s wire carries the natural magic", name)
		}
		if _, err := strategy.DecodeNaturalPolicy(wire); !errors.Is(err, strategy.ErrInvalidNaturalPolicyEncoding) {
			t.Fatalf("natural codec decoded %s wire: %v", name, err)
		}
		if _, err := strategy.DecodeCapacityPolicy(v5); err == nil {
			t.Fatalf("capacity codec decoded the natural wire")
		}
		if _, err := strategy.DecodeReproductionPolicy(v5); err == nil {
			t.Fatalf("reproduction codec decoded the natural wire")
		}
		if _, err := strategy.DecodeSocialFoodPolicy(v5); err == nil {
			t.Fatalf("social-food codec decoded the natural wire")
		}
		if _, err := strategy.DecodeFoodFlowPolicy(v5); err == nil {
			t.Fatalf("food-flow codec decoded the natural wire")
		}
	}
}
