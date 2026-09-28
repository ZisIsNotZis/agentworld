package strategy_test

import (
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

func TestSocialFoodCanonicalPolicyContentAndImmutableBindings(t *testing.T) {
	p := strategy.FrozenSocialFoodPolicy(strategy.SocialFoodRef{ID: "social-v2", Version: 2})
	r := strategy.NewSocialFoodRegistry()
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(strategy.SocialFoodBinding{Actor: 4, Ref: p.Ref})
	if err != nil {
		t.Fatal(err)
	}
	original := b.Policy()
	p.Weights.AcceptThreshold = 0
	p.Ties[3].Affinity = 0
	p.Ref.ID = "mutated"
	if b.Policy() != original {
		t.Fatal("source policy mutated registered content")
	}
	copy := b.Policy()
	copy.Ties[3].Target = 7
	copy.Weights.ReserveFloor = 0
	if b.Policy() != original || b.Binding() != (strategy.SocialFoodBinding{Actor: 4, Ref: original.Ref}) {
		t.Fatal("returned policy mutated bound content")
	}
	wire, err := strategy.EncodeSocialFoodPolicy(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := strategy.DecodeSocialFoodPolicy(wire)
	if err != nil || decoded != original {
		t.Fatalf("policy round-trip: %+v %v", decoded, err)
	}
	if err := strategy.VerifySocialFoodPolicy(wire, strategy.FrozenSocialFoodPolicy(original.Ref)); err != nil {
		t.Fatalf("independently reconstructed policy rejected: %v", err)
	}
	hash, err := strategy.SocialFoodPolicyFingerprint(original)
	if err != nil || hash != sha256.Sum256(wire) {
		t.Fatalf("full content fingerprint: %x %v", hash, err)
	}
	wire[0] = 'X'
	if b.Policy() != original || (*strategy.SocialFoodBound)(nil).Policy() != (strategy.SocialFoodPolicy{}) {
		t.Fatal("encoded bytes or nil bound changed registered policy")
	}
}

func TestSocialFoodRestoreRejectsSameRefChangedWeightsAndRoster(t *testing.T) {
	p := strategy.FrozenSocialFoodPolicy(strategy.SocialFoodRef{ID: "social-v2", Version: 2})
	wire, err := strategy.EncodeSocialFoodPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	// The world v2 contract freezes the weights and dyads, so even a
	// self-consistent replacement at the same ref is not a valid policy.
	for _, change := range []func(*strategy.SocialFoodPolicy){
		func(p *strategy.SocialFoodPolicy) { p.Weights.UrgencyBonus++ },
		func(p *strategy.SocialFoodPolicy) { p.Weights.ReserveFloor-- },
		func(p *strategy.SocialFoodPolicy) { p.Ties[3].Affinity-- },
		func(p *strategy.SocialFoodPolicy) { p.Ties[3].Target++ },
	} {
		changed := p
		change(&changed)
		if _, err := strategy.EncodeSocialFoodPolicy(changed); !errors.Is(err, strategy.ErrInvalidSocialFoodPolicy) {
			t.Fatalf("changed frozen content encoded: %+v %v", changed, err)
		}
		if err := strategy.VerifySocialFoodPolicy(wire, changed); !errors.Is(err, strategy.ErrInvalidSocialFoodPolicy) {
			t.Fatalf("changed expected content accepted: %v", err)
		}
	}
	for _, changed := range []strategy.SocialFoodPolicy{
		func() strategy.SocialFoodPolicy { c := p; c.Budget.Candidates = 1; return c }(),
		func() strategy.SocialFoodPolicy { c := p; c.Ref.Version++; return c }(),
	} {
		other, err := strategy.EncodeSocialFoodPolicy(changed)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(wire, other) || strategy.VerifySocialFoodPolicy(other, p) != strategy.ErrSocialFoodPolicyMismatch {
			t.Fatal("same-ref budget / changed version accepted at restore")
		}
	}
	for _, tc := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"empty", func([]byte) []byte { return nil }},
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }},
		{"trailing", func(b []byte) []byte { return append(b, 0) }},
		{"magic", func(b []byte) []byte { b[0] = 'X'; return b }},
		{"format", func(b []byte) []byte { b[7] = 1; return b }},
		{"length", func(b []byte) []byte { b[8] = 0; return b }},
		{"weight", func(b []byte) []byte { b[9+len(p.Ref.ID)+4+2+3]++; return b }},
		{"affinity", func(b []byte) []byte { b[len(b)-1]++; return b }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := tc.edit(append([]byte(nil), wire...))
			if _, err := strategy.DecodeSocialFoodPolicy(bad); !errors.Is(err, strategy.ErrInvalidSocialFoodPolicyEncoding) {
				t.Fatalf("malformed / changed content decoded: %x %v", bad, err)
			}
			if err := strategy.VerifySocialFoodPolicy(bad, p); !errors.Is(err, strategy.ErrInvalidSocialFoodPolicyEncoding) {
				t.Fatalf("malformed / changed content restored: %v", err)
			}
		})
	}
}
