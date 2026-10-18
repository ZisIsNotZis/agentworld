package main

import (
	"encoding/json"
	"strings"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// TestPrivacyScanSelectsTheDeclaredContract is the regression guard for the
// VS2-EC-6 defect: the allow-list must come from the contract a request declares
// about itself. With a single v1 list every natural-claim/v4 request failed the
// key check, the scan returned before the privileged-marker test, and the band
// reported key violations while asserting the observation key set was exact -
// the boundary check was silently disabled for the whole contract under test.
func TestPrivacyScanSelectsTheDeclaredContract(t *testing.T) {
	encode := func(t *testing.T, contract cognition.ClaimContract) []byte {
		t.Helper()
		body, err := cognition.EncodeActorSeamRequestFor(privacySentinel(), contract)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	cases := []struct {
		name            string
		contract        cognition.ClaimContract
		mutate          func(map[string]any)
		wantViolations  int
		wantCertified   int
		wantMarkerScope int
		wantMarkers     int
	}{
		{name: "v1 request is certified under v1", contract: cognition.ClaimContractV1, wantCertified: 1, wantMarkerScope: 1},
		{name: "v2 request is certified under v2", contract: cognition.ClaimContractV2, wantCertified: 1, wantMarkerScope: 1},
		{name: "v4 request is certified under v4", contract: cognition.ClaimContractV4, wantCertified: 1, wantMarkerScope: 1},
		{
			name: "a v4 request with an extra observation key is a violation", contract: cognition.ClaimContractV4,
			mutate: func(top map[string]any) {
				top["observation"].(map[string]any)["tenure_mode"] = 1
			},
			wantViolations: 1,
		},
		{
			name: "a v4 request carrying a v1 name is a violation", contract: cognition.ClaimContractV4,
			mutate: func(top map[string]any) {
				top["observation"].(map[string]any)["hunger"] = 3
			},
			wantViolations: 1,
		},
		{
			name: "an undeclared contract is a violation, never a pass", contract: cognition.ClaimContractV4,
			mutate: func(top map[string]any) {
				top["prompt_template"] = "natural-claim/v9"
			},
			wantViolations: 1,
		},
		{
			// The marker scan only runs on a block that passed the key check; this
			// case proves it is a live test inside the declared field set, not a
			// claim about a block the scan never reached.
			name: "a privileged marker inside a declared field is caught", contract: cognition.ClaimContractV4,
			mutate: func(top map[string]any) {
				top["observation"].(map[string]any)["last_refused_action"] = "owner refused"
			},
			wantCertified: 1, wantMarkerScope: 1, wantMarkers: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := encode(t, tc.contract)
			if tc.mutate != nil {
				var top map[string]any
				if err := json.Unmarshal(body, &top); err != nil {
					t.Fatal(err)
				}
				tc.mutate(top)
				mutated, err := json.Marshal(top)
				if err != nil {
					t.Fatal(err)
				}
				body = mutated
			}
			scan := newPrivacyScan()
			scan.scanRoleARequest("test#1", body)
			if len(scan.keyViolations) != tc.wantViolations {
				t.Fatalf("key violations = %v, want %d", scan.keyViolations, tc.wantViolations)
			}
			if got := scan.byContract[tc.contract]; got != tc.wantCertified {
				t.Fatalf("certified under %s = %d, want %d", tc.contract, got, tc.wantCertified)
			}
			if scan.markersScanned != tc.wantMarkerScope {
				t.Fatalf("marker scan scope = %d, want %d", scan.markersScanned, tc.wantMarkerScope)
			}
			if len(scan.markers) != tc.wantMarkers {
				t.Fatalf("markers = %v, want %d", scan.markers, tc.wantMarkers)
			}
		})
	}
}

// TestPrivacyScanCoversEveryDeclaredContract pins that the sentinel is checked
// under every declared contract: a contract whose encoder drifts from its
// declaration must fail the band rather than pass because another contract's
// sentinel was the one scanned.
func TestPrivacyScanCoversEveryDeclaredContract(t *testing.T) {
	scan := newPrivacyScan()
	scanned := 0
	for _, row := range cognition.DeclaredSeamRequestKeySets() {
		body, err := cognition.EncodeActorSeamRequestFor(privacySentinel(), row.Contract)
		if err != nil {
			t.Fatalf("%s: %v", row.Contract, err)
		}
		scan.scanRoleARequest("sentinel:"+string(row.Contract), body)
		scanned++
	}
	if scanned != 3 {
		t.Fatalf("sentinel contracts scanned = %d, want 3", scanned)
	}
	if len(scan.keyViolations) != 0 || len(scan.markers) != 0 {
		t.Fatalf("sentinel violations=%v markers=%v", scan.keyViolations, scan.markers)
	}
	evidence := contractCounts(scan.byContract)
	for _, want := range []string{"natural-claim/v1=", "natural-claim/v2=", "natural-claim/v4="} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("per-contract evidence %q is missing %q", evidence, want)
		}
	}
}

// privacySentinel is a fully populated request for every declared contract.
func privacySentinel() cognition.ActorSeamRequest {
	home, err := sim.EntityRefValue(1001)
	if err != nil {
		panic(err)
	}
	return cognition.ActorSeamRequest{
		Seam: cognition.SeamNaturalClaim, Actor: 5, Hour: 6, Version: 42,
		Observation: strategy.NaturalObservation{
			Actor: 5, Ref: strategy.NaturalRef{ID: "natural", Version: 7}, Hour: 6,
			Energy: sim.IntegerValue(6), Hunger: sim.IntegerValue(2), BagUnits: sim.IntegerValue(0),
			BagSource: home, Capital: sim.IntegerValue(1), Wip: sim.IntegerValue(0),
			GranaryStock: sim.IntegerValue(3), InvestedUnits: sim.IntegerValue(1),
			LastBuildHour: sim.IntegerValue(4), LastStoredMealHour: sim.IntegerValue(2),
			LastDenial: strategy.NaturalGatherDenied,
			HomeSlots:  []strategy.NaturalSlotView{{ID: 2001, Stock: sim.IntegerValue(1)}},
		},
		Strategy: cognition.FrozenNaturalClaimParameters(strategy.FrozenNaturalPolicy7().Ref),
		Actions:  []string{"gather", "eat-stored", "provision", "wait"},
		Identity: cognition.RunIdentity{
			Fixture: cognition.FixtureIdentity{Patches: 2, R: 8, Founders: 12, Mechanism: 6, Births: true},
			Seed:    0, BaseRef: strategy.FrozenNaturalPolicy7().Ref, HeadRef: strategy.FrozenNaturalPolicy7().Ref,
		},
	}
}
