package strategy_test

import (
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestCapacityCanonicalCodecRoundtripAndDrift(t *testing.T) {
	p := strategy.FrozenCapacityPolicy()
	wire, err := strategy.EncodeCapacityPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	// Canonical full content: format, ref identity, and budget, fixed order,
	// exact framing (4 magic + 4 format + 1 len + 8 id + 4 version + 2 budget).
	const frozenWireLen = 4 + 4 + 1 + len("capacity") + 4 + 1 + 1
	if len(wire) != frozenWireLen {
		t.Fatalf("wire length %d, want %d: %x", len(wire), frozenWireLen, wire)
	}
	again, err := strategy.EncodeCapacityPolicy(p)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatalf("encoding is not deterministic: %x %x %v", wire, again, err)
	}
	got, err := strategy.DecodeCapacityPolicy(wire)
	if err != nil || got != p {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyCapacityPolicy(wire, p); err != nil {
		t.Fatalf("frozen wire must verify: %v", err)
	}
	if sum, err := strategy.CapacityPolicyFingerprint(p); err != nil || sum != sha256.Sum256(wire) {
		t.Fatalf("fingerprint: %v %v", sum, err)
	}
	// Every single-byte mutation either fails decoding or fails registry
	// verification against the executable's frozen policy.
	for i := range wire {
		bad := append([]byte(nil), wire...)
		bad[i] ^= 0xff
		if _, err := strategy.DecodeCapacityPolicy(bad); err == nil {
			if err := strategy.VerifyCapacityPolicy(bad, p); err == nil {
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
		if _, err := strategy.DecodeCapacityPolicy(edit(wire)); !errors.Is(err, strategy.ErrInvalidCapacityPolicyEncoding) {
			t.Fatalf("%s wire decoded: %v", name, err)
		}
		if err := strategy.VerifyCapacityPolicy(edit(wire), p); err == nil {
			t.Fatalf("%s wire verified", name)
		}
	}
	// A same-ref budget variant is a well-formed policy but can never be
	// restored against the frozen expectation: full-content comparison.
	variant := p
	variant.Budget = strategy.CapacityBudget{Candidates: 2, Evaluations: 2}
	variantWire, err := strategy.EncodeCapacityPolicy(variant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := strategy.DecodeCapacityPolicy(variantWire); err != nil || got != variant {
		t.Fatalf("variant roundtrip: %+v %v", got, err)
	}
	if err := strategy.VerifyCapacityPolicy(variantWire, p); !errors.Is(err, strategy.ErrCapacityPolicyMismatch) {
		t.Fatalf("same-ref budget drift restored: %v", err)
	}
	if err := strategy.VerifyCapacityPolicy(wire, variant); !errors.Is(err, strategy.ErrCapacityPolicyMismatch) {
		t.Fatalf("frozen wire verified against variant: %v", err)
	}
	// Drifted expectations fail closed before any bytes are compared.
	drifted := p
	drifted.Ref.Version = 4
	if _, err := strategy.EncodeCapacityPolicy(drifted); !errors.Is(err, strategy.ErrInvalidCapacityPolicy) {
		t.Fatalf("drifted policy encoded: %v", err)
	}
	if err := strategy.VerifyCapacityPolicy(wire, drifted); !errors.Is(err, strategy.ErrInvalidCapacityPolicy) {
		t.Fatalf("drifted expectation accepted: %v", err)
	}
}
