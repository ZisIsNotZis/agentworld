package strategy_test

import (
	"reflect"
	"testing"

	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The Role B versioned-strategy tests (spec sections 5 and 10). The central
// pin is the all-parent-values reduction: a cognitive generation carrying the
// frozen natural/7 vector must reproduce the frozen cascade's decisions
// exactly, so a patch is a strict data substitution and never a behaviour
// reinterpretation.

func cognitiveBound7(t *testing.T) *strategy.NaturalBound {
	t.Helper()
	reg := strategy.NewNaturalRegistry(naturalTestPatches)
	if err := reg.RegisterFrozenNatural(strategy.FrozenNaturalPolicy7()); err != nil {
		t.Fatal(err)
	}
	bound, err := reg.Bind(strategy.NaturalBinding{Actor: 1, Ref: strategy.FrozenNaturalPolicy7().Ref})
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

func cognitiveParentGeneration(t *testing.T) strategy.NaturalCognitiveGeneration {
	t.Helper()
	gen, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy7().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, strategy.FrozenCognitiveParams())
	if err != nil {
		t.Fatal(err)
	}
	return gen
}

func TestCognitiveParameterContractShape(t *testing.T) {
	contract := strategy.CognitiveParameterContract()
	if len(contract) != 7 {
		t.Fatalf("contract rows %d, want 7", len(contract))
	}
	frozen := strategy.FrozenCognitiveParams()
	for _, row := range contract {
		if row.DomainMin > row.DomainMax {
			t.Fatalf("%s: domain [%d,%d]", row.ID, row.DomainMin, row.DomainMax)
		}
		value, ok := frozen.Value(row.ID)
		if !ok || value != row.ParentValue {
			t.Fatalf("%s: frozen vector %d, contract parent %d (ok %v)", row.ID, value, row.ParentValue, ok)
		}
		min, max, ok := strategy.CognitiveParameterDomain(row.ID)
		if !ok || min != row.DomainMin || max != row.DomainMax {
			t.Fatalf("%s: domain lookup mismatch", row.ID)
		}
	}
}

// TestCognitiveClaimAllParentReduction is the pinned reduction property: the
// frozen parent vector reproduces the frozen natural/7 claim and bag decisions
// exactly across a decision grid.
func TestCognitiveClaimAllParentReduction(t *testing.T) {
	bound := cognitiveBound7(t)
	gen := cognitiveParentGeneration(t)
	if err := gen.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, hour := range []int{1, 3, 5} {
		for _, energy := range []int64{0, 1, 2, 3, 4, 5, 8} {
			for _, granary := range []int64{0, 1, 3} {
				for _, capital := range []int64{0, 1, 2} {
					for _, bag := range []int64{0, 1} {
						for _, denial := range []strategy.NaturalDenial{strategy.NaturalNoDenial, strategy.NaturalGatherDenied, strategy.NaturalBuildDenied} {
							if denial == strategy.NaturalBuildDenied && bag == 0 {
								continue // a build denial with an empty bag is a malformed observation for both cascades
							}
							obs := naturalObservation(t, 1, strategy.FrozenNaturalPolicy7().Ref, hour)
							if bag == 1 {
								obs = naturalHeldAtMargins(t, 1, strategy.FrozenNaturalPolicy7().Ref, hour)
							} else {
								obs = naturalStocked(obs, 0, 3)
							}
							obs.Energy = sim.IntegerValue(energy)
							obs.GranaryStock = sim.IntegerValue(granary)
							obs.Capital = sim.IntegerValue(capital)
							obs.LastDenial = denial
							want, err := bound.Evaluate7(obs)
							if err != nil {
								t.Fatalf("frozen: %v", err)
							}
							got, err := gen.EvaluateClaim(naturalTestPatches, 1, obs)
							if err != nil {
								t.Fatalf("cognitive: %v", err)
							}
							if !reflect.DeepEqual(want, got) {
								t.Fatalf("reduction mismatch hour %d energy %d granary %d k %d bag %d denial %d:\n frozen %+v\n cog    %+v",
									hour, energy, granary, capital, bag, denial, want, got)
							}
						}
					}
				}
			}
		}
	}
}

// TestCognitiveBirthAllParentReduction pins the same property for the proposer
// filing cascade.
func TestCognitiveBirthAllParentReduction(t *testing.T) {
	bound := cognitiveBound7(t)
	gen := cognitiveParentGeneration(t)
	for _, energy := range []int64{0, 3, 4, 8} {
		for _, status := range []strategy.NaturalRequestStatus{strategy.NaturalRequestIdle, strategy.NaturalRequestPending} {
			obs := naturalRequestView(t, 1, strategy.FrozenNaturalPolicy7().Ref, 3)
			obs.Energy = sim.IntegerValue(energy)
			obs.OwnRequest = strategy.NaturalRequestGuard{Status: status, LastRequestHour: -1}
			if status == strategy.NaturalRequestPending {
				obs.OwnRequest.LastRequestHour = 2
			}
			want, err := bound.EvaluateBirthRequest7(obs)
			if err != nil {
				t.Fatalf("frozen: %v", err)
			}
			got, err := gen.EvaluateBirthRequest(naturalTestPatches, 1, obs)
			if err != nil {
				t.Fatalf("cognitive: %v", err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("reduction mismatch energy %d status %d:\n frozen %+v\n cog    %+v", energy, status, want, got)
			}
		}
	}
}

// TestCognitiveCodecRoundTripAndTamper pins the canonical wire: a round trip
// preserves the generation, the fingerprint is stable, and a mutated byte is
// rejected.
func TestCognitiveCodecRoundTripAndTamper(t *testing.T) {
	gen := cognitiveParentGeneration(t)
	params := strategy.FrozenCognitiveParams()
	params.ClaimProvisionEnergyFloor = 3
	child, err := strategy.NewNaturalCognitiveGeneration(gen.Ref, [32]byte{9}, strategy.FrozenNaturalPolicy7().Ref, true, params)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := strategy.EncodeNaturalCognitiveGeneration(child)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := strategy.DecodeNaturalCognitiveGeneration(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, child) {
		t.Fatalf("round trip mismatch:\n want %+v\n got  %+v", child, decoded)
	}
	fp1, err := strategy.NaturalCognitiveGenerationFingerprint(child)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := strategy.NaturalCognitiveGenerationFingerprint(decoded)
	if err != nil || fp1 != fp2 {
		t.Fatalf("fingerprint unstable: %v %x %x", err, fp1, fp2)
	}
	tampered := append([]byte(nil), wire...)
	tampered[len(tampered)-1] ^= 1
	if _, err := strategy.DecodeNaturalCognitiveGeneration(tampered); err == nil {
		t.Fatal("tampered wire accepted")
	}
}

// TestCognitiveGenerationValidation pins the distinct id/format space, the
// version rule, and domain enforcement.
func TestCognitiveGenerationValidation(t *testing.T) {
	if strategy.NaturalCognitiveRefID == "natural" {
		t.Fatal("cognitive generations must not mint refs inside the frozen natural space")
	}
	if _, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy7().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, strategy.FrozenCognitiveParams()); err != nil {
		t.Fatalf("valid parent generation rejected: %v", err)
	}
	// Out-of-domain value refused.
	bad := strategy.FrozenCognitiveParams()
	bad.ClaimProvisionEnergyFloor = 9
	if _, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy7().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, bad); err == nil {
		t.Fatal("out-of-domain vector accepted")
	}
	// Unknown / foreign parent refused.
	if _, err := strategy.NewNaturalCognitiveGeneration(
		strategy.FrozenNaturalPolicy9().Ref, [32]byte{}, strategy.FrozenNaturalPolicy7().Ref, true, strategy.FrozenCognitiveParams()); err == nil {
		t.Fatal("foreign parent accepted")
	}
	// Version rule: parent.Version+1.
	gen := cognitiveParentGeneration(t)
	if gen.Ref.Version != strategy.FrozenNaturalPolicy7().Ref.Version+1 || gen.Ref.ID != strategy.NaturalCognitiveRefID {
		t.Fatalf("child ref %+v", gen.Ref)
	}
	child, err := strategy.NewNaturalCognitiveGeneration(gen.Ref, [32]byte{}, gen.BaseRef, gen.Provision, strategy.FrozenCognitiveParams())
	if err != nil {
		t.Fatal(err)
	}
	if child.Ref.Version != gen.Ref.Version+1 {
		t.Fatalf("chained version %d, want %d", child.Ref.Version, gen.Ref.Version+1)
	}
}
