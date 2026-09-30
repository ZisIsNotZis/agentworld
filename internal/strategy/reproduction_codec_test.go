package strategy_test

import (
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestReproductionCanonicalCodecRoundtripAndDrift(t *testing.T) {
	p := strategy.FrozenReproductionPolicy()
	wire, err := strategy.EncodeReproductionPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	// Canonical full content: format, ref identity, and budget, fixed order,
	// exact framing (4 magic + 4 format + 1 len + 12 id + 4 version + 2
	// budget).
	const frozenWireLen = 4 + 4 + 1 + len("reproduction") + 4 + 1 + 1
	if len(wire) != frozenWireLen {
		t.Fatalf("wire length %d, want %d: %x", len(wire), frozenWireLen, wire)
	}
	again, err := strategy.EncodeReproductionPolicy(p)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatalf("encoding is not deterministic: %x %x %v", wire, again, err)
	}
	got, err := strategy.DecodeReproductionPolicy(wire)
	if err != nil || got != p {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyReproductionPolicy(wire, p); err != nil {
		t.Fatalf("frozen wire must verify: %v", err)
	}
	if sum, err := strategy.ReproductionPolicyFingerprint(p); err != nil || sum != sha256.Sum256(wire) {
		t.Fatalf("fingerprint: %v %v", sum, err)
	}
	// Every single-byte mutation either fails decoding or fails registry
	// verification against the executable's frozen policy.
	for i := range wire {
		bad := append([]byte(nil), wire...)
		bad[i] ^= 0xff
		if _, err := strategy.DecodeReproductionPolicy(bad); err == nil {
			if err := strategy.VerifyReproductionPolicy(bad, p); err == nil {
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
		if _, err := strategy.DecodeReproductionPolicy(edit(wire)); !errors.Is(err, strategy.ErrInvalidReproductionPolicyEncoding) {
			t.Fatalf("%s wire decoded: %v", name, err)
		}
		if err := strategy.VerifyReproductionPolicy(edit(wire), p); err == nil {
			t.Fatalf("%s wire verified", name)
		}
	}
	// A same-ref budget variant is a well-formed policy but can never be
	// restored against the frozen expectation: full-content comparison.
	variant := p
	variant.Budget = strategy.ReproductionBudget{Candidates: 2, Evaluations: 2}
	variantWire, err := strategy.EncodeReproductionPolicy(variant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := strategy.DecodeReproductionPolicy(variantWire); err != nil || got != variant {
		t.Fatalf("variant roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyReproductionPolicy(variantWire, p); !errors.Is(err, strategy.ErrReproductionPolicyMismatch) {
		t.Fatalf("same-ref budget drift restored: %v", err)
	}
	if err := strategy.VerifyReproductionPolicy(wire, variant); !errors.Is(err, strategy.ErrReproductionPolicyMismatch) {
		t.Fatalf("frozen wire verified against variant: %v", err)
	}
	// Drifted expectations fail closed before any bytes are compared.
	drifted := p
	drifted.Ref.Version = 5
	if _, err := strategy.EncodeReproductionPolicy(drifted); !errors.Is(err, strategy.ErrInvalidReproductionPolicy) {
		t.Fatalf("drifted policy encoded: %v", err)
	}
	if err := strategy.VerifyReproductionPolicy(wire, drifted); !errors.Is(err, strategy.ErrInvalidReproductionPolicy) {
		t.Fatalf("drifted expectation accepted: %v", err)
	}
}

// The v4 magic is disjoint from every other policy wire in the executable:
// no earlier codec decodes it and DecodeReproductionPolicy decodes none of
// them, so no checkpoint section can be replayed as another.
func TestReproductionMagicDisjointFromOtherPolicyWires(t *testing.T) {
	v4, err := strategy.EncodeReproductionPolicy(strategy.FrozenReproductionPolicy())
	if err != nil {
		t.Fatal(err)
	}
	others := map[string][]byte{
		"capacity-v3": func() []byte {
			w, err := strategy.EncodeCapacityPolicy(strategy.FrozenCapacityPolicy())
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
	// The food-flow and portable-strategy encoders need bound instances the
	// frozen constructors do not expose here; their published magics are the
	// disjointness contract's remaining witnesses.
	for name, wire := range others {
		if bytes.Equal(v4[:4], wire[:4]) {
			t.Fatalf("%s shares the v4 magic prefix %q", name, v4[:4])
		}
		if _, err := strategy.DecodeReproductionPolicy(wire); !errors.Is(err, strategy.ErrInvalidReproductionPolicyEncoding) {
			t.Fatalf("%s wire decoded as reproduction: %v", name, err)
		}
	}
	for _, magic := range []string{"AWCP", "AWFP", "AWSP", "AWST"} {
		if string(v4[:4]) == magic {
			t.Fatalf("v4 magic collides with %s", magic)
		}
		foreign := append([]byte(nil), v4...)
		copy(foreign, magic)
		if _, err := strategy.DecodeReproductionPolicy(foreign); !errors.Is(err, strategy.ErrInvalidReproductionPolicyEncoding) {
			t.Fatalf("%s-prefixed wire decoded as reproduction: %v", magic, err)
		}
	}
	// Conversely, the v4 wire must decode as reproduction and nothing earlier
	// claims it: the v4 magic bytes are exactly AWRP.
	if string(v4[:4]) != "AWRP" {
		t.Fatalf("unexpected v4 magic %q", v4[:4])
	}
}
