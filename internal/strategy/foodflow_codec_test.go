package strategy_test

import (
	"agentworld/internal/strategy"
	"agentworld/internal/world"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestFoodFlowPolicyContentCopiesAndCanonicalWire(t *testing.T) {
	p := strategy.FoodFlowPolicy{
		FormatVersion: strategy.FoodFlowPolicyFormatV1,
		Ref:           strategy.FoodFlowRef{ID: "pilot", Version: 1},
		Budget:        strategy.FoodFlowBudget{Candidates: 16, Evaluations: 16},
		RestDuration:  strategy.FoodFlowRestDuration,
	}
	r := strategy.NewFoodFlowRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b := foodFlowBound(t, r, p.Ref, 1)
	original := b.Policy()
	p.Budget.Candidates, p.Ref.ID, p.RestDuration = 1, "mutated", world.FoodFlowHour
	if got := b.Policy(); got != original {
		t.Fatalf("registration retained caller's policy: %+v", got)
	}
	copy := b.Policy()
	copy.Budget.Evaluations, copy.Ref.ID = 2, "changed"
	if got := b.Policy(); got != original {
		t.Fatalf("exported policy mutated shared binding: %+v", got)
	}
	second := foodFlowBound(t, r, original.Ref, 2)
	if second.Policy() != original || second.Binding() != (strategy.FoodFlowBinding{Actor: 2, Ref: original.Ref}) ||
		b.Binding() != (strategy.FoodFlowBinding{Actor: 1, Ref: original.Ref}) {
		t.Fatal("per-actor bindings did not share independent pinned content")
	}
	wire, err := strategy.EncodeFoodFlowPolicy(original)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString("41574650000000010570696c6f740000000110100000000023c34600")
	if !bytes.Equal(wire, want) {
		t.Fatalf("noncanonical policy wire: %x", wire)
	}
	decoded, err := strategy.DecodeFoodFlowPolicy(wire)
	if err != nil || decoded != original {
		t.Fatalf("round-trip: %+v %v", decoded, err)
	}
	if err := strategy.VerifyFoodFlowPolicy(wire, second.Policy()); err != nil {
		t.Fatalf("independent equal content rejected: %v", err)
	}
	hash, err := strategy.FoodFlowPolicyFingerprint(original)
	if err != nil || hash != sha256.Sum256(wire) {
		t.Fatalf("canonical fingerprint: %x %v", hash, err)
	}
	wire[0] = 'X'
	if b.Policy() != original {
		t.Fatal("wire mutation affected bound content")
	}
	if nilBound := (*strategy.FoodFlowBound)(nil).Policy(); nilBound != (strategy.FoodFlowPolicy{}) {
		t.Fatalf("nil bound exposed policy: %+v", nilBound)
	}
	copyWire, err := strategy.EncodeFoodFlowPolicy(copy)
	if err != nil || bytes.Equal(copyWire, want) {
		t.Fatalf("changed copy must produce distinct valid content: %x %v", copyWire, err)
	}
}

func TestFoodFlowPolicyContentMismatchAndMalformedWire(t *testing.T) {
	r, ref := foodFlowFixture(t)
	original := foodFlowBound(t, r, ref, 1).Policy()
	wire, err := strategy.EncodeFoodFlowPolicy(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*strategy.FoodFlowPolicy){
		func(p *strategy.FoodFlowPolicy) { p.Budget.Candidates-- },
		func(p *strategy.FoodFlowPolicy) { p.Budget.Evaluations-- },
		func(p *strategy.FoodFlowPolicy) { p.Ref.Version++ },
	} {
		different := original
		change(&different)
		if err := strategy.VerifyFoodFlowPolicy(wire, different); !errors.Is(err, strategy.ErrFoodFlowPolicyMismatch) {
			t.Fatalf("changed expected policy accepted: %+v %v", different, err)
		}
		other, err := strategy.EncodeFoodFlowPolicy(different)
		if err != nil {
			t.Fatal(err)
		}
		if err := strategy.VerifyFoodFlowPolicy(other, original); !errors.Is(err, strategy.ErrFoodFlowPolicyMismatch) {
			t.Fatalf("changed stored policy accepted: %+v %v", different, err)
		}
		hash, err := strategy.FoodFlowPolicyFingerprint(different)
		originalHash, _ := strategy.FoodFlowPolicyFingerprint(original)
		if err != nil || hash == originalHash {
			t.Fatalf("fingerprint failed to cover %+v: %v", different, err)
		}
	}
	for _, corrupt := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"empty", func(_ []byte) []byte { return nil }},
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }},
		{"trailing", func(b []byte) []byte { return append(b, 0) }},
		{"magic", func(b []byte) []byte { b[0] = 'X'; return b }},
		{"format", func(b []byte) []byte { b[7] = 2; return b }},
		{"empty ref", func(b []byte) []byte { b[8] = 0; return b }},
		{"oversize ref", func(b []byte) []byte { b[8] = 65; return b }},
		{"ref version", func(b []byte) []byte { b[17] = 0; return b }},
		{"zero budget", func(b []byte) []byte { b[18] = 0; return b }},
		{"duration", func(b []byte) []byte { b[len(b)-1]++; return b }},
	} {
		t.Run(corrupt.name, func(t *testing.T) {
			bad := corrupt.edit(append([]byte(nil), wire...))
			if _, err := strategy.DecodeFoodFlowPolicy(bad); !errors.Is(err, strategy.ErrInvalidFoodFlowPolicyEncoding) {
				t.Fatalf("invalid policy wire accepted: %x %v", bad, err)
			}
			if err := strategy.VerifyFoodFlowPolicy(bad, original); !errors.Is(err, strategy.ErrInvalidFoodFlowPolicyEncoding) {
				t.Fatalf("invalid policy wire verified: %x %v", bad, err)
			}
		})
	}
	for _, invalid := range []strategy.FoodFlowPolicy{
		{FormatVersion: 2, Ref: ref, Budget: original.Budget, RestDuration: original.RestDuration},
		{FormatVersion: 1, Ref: ref, Budget: original.Budget, RestDuration: world.FoodFlowHour},
	} {
		if _, err := strategy.EncodeFoodFlowPolicy(invalid); !errors.Is(err, strategy.ErrInvalidFoodFlowPolicy) {
			t.Fatalf("invalid policy encoded: %+v %v", invalid, err)
		}
		if err := strategy.VerifyFoodFlowPolicy(wire, invalid); !errors.Is(err, strategy.ErrInvalidFoodFlowPolicy) {
			t.Fatalf("invalid expected policy verified: %+v %v", invalid, err)
		}
	}
}
