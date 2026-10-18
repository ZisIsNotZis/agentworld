package cognition_test

import (
	"encoding/json"
	"strings"
	"testing"

	"agentworld/internal/cognition"
)

// TestDeclaredSeamRequestKeySetsMatchTheEncoders is the EC-6 declaration guard:
// for every declared contract, the declared request and observation key sets
// must equal exactly what that contract's encoder writes for the same request
// value. Without it a rename inside one encoder leaves the boundary check
// certifying a set no request carries - which is how natural-claim/v4 requests
// came to fail a v1 allow-list, disabling the marker scan for that whole run.
func TestDeclaredSeamRequestKeySetsMatchTheEncoders(t *testing.T) {
	declared := cognition.DeclaredSeamRequestKeySets()
	if len(declared) != 3 {
		t.Fatalf("declared contracts = %d, want 3 (v1, v2, v4)", len(declared))
	}
	seen := map[cognition.ClaimContract]bool{}
	for _, row := range declared {
		if seen[row.Contract] {
			t.Fatalf("contract %s declared twice", row.Contract)
		}
		seen[row.Contract] = true
		body, err := cognition.EncodeActorSeamRequestFor(sampleSeamRequest(), row.Contract)
		if err != nil {
			t.Fatalf("%s: %v", row.Contract, err)
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(body, &top); err != nil {
			t.Fatalf("%s: %v", row.Contract, err)
		}
		if got, want := keyList(top), row.Request; !sameStrings(got, want) {
			t.Fatalf("%s request keys = %v, declared %v", row.Contract, sortedCopy(got), sortedCopy(want))
		}
		obs, ok := top["observation"]
		if !ok {
			t.Fatalf("%s: no observation block", row.Contract)
		}
		var observation map[string]json.RawMessage
		if err := json.Unmarshal(obs, &observation); err != nil {
			t.Fatalf("%s: %v", row.Contract, err)
		}
		if got, want := keyList(observation), row.Observation; !sameStrings(got, want) {
			t.Fatalf("%s observation keys = %v, declared %v", row.Contract, sortedCopy(got), sortedCopy(want))
		}
		// No privileged marker may appear in a declared observation block: the
		// declaration is what the boundary scan trusts.
		lower := strings.ToLower(string(obs))
		for _, marker := range []string{"tenure", "owner", "debt", "spoil", "allocator", "regeneration", "kin", "other", "parent", "genome"} {
			if strings.Contains(lower, marker) {
				t.Fatalf("%s observation carries the privileged marker %q: %s", row.Contract, marker, obs)
			}
		}
	}
	for _, want := range []cognition.ClaimContract{cognition.ClaimContractV1, cognition.ClaimContractV2, cognition.ClaimContractV4} {
		if _, ok := cognition.DeclaredSeamRequestKeySet(want); !ok {
			t.Fatalf("contract %s has no declared key set", want)
		}
	}
	if _, ok := cognition.DeclaredSeamRequestKeySet(cognition.ClaimContract("natural-claim/v9")); ok {
		t.Fatal("an undeclared contract must not resolve to a key set")
	}
}

func keyList(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := sortedCopy(a), sortedCopy(b)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}
