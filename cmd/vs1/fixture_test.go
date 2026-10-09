package main

import (
	"testing"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/strategy"
)

// TestScriptedFixtureDeltaAdmitsAndIsInDomain pins the scripted-proposer
// fixture's Role B delta and that the declared arms keep the honest-null
// provision-floor proposal unchanged.
func TestScriptedFixtureDeltaAdmitsAndIsInDomain(t *testing.T) {
	deltas, _ := patchedDeltasFor(ArmA2f)
	if len(deltas) != 1 || deltas[0].Param != protocol.ParamBagBuildEnergyFloor || deltas[0].Value != 2 {
		t.Fatalf("fixture deltas %+v, want bag.build_energy_floor=2", deltas)
	}
	for _, d := range deltas {
		min, max, ok := strategy.CognitiveParameterDomain(strategy.CognitiveParamID(d.Param))
		if !ok || d.Value < min || d.Value > max {
			t.Fatalf("fixture delta %s=%d outside the declared domain [%d,%d]", d.Param, d.Value, min, max)
		}
	}
	for _, arm := range []string{ArmA2, ArmA3} {
		deltas, _ := patchedDeltasFor(arm)
		if len(deltas) != 1 || deltas[0].Param != protocol.ParamClaimProvisionEnergyFloor || deltas[0].Value != 3 {
			t.Fatalf("declared arm %s delta changed: %+v", arm, deltas)
		}
	}
}

// TestFixtureLabelIsScripted pins the fixture label carried into the report.
func TestFixtureLabelIsScripted(t *testing.T) {
	if FixtureLabel == "" || !contains(FixtureLabel, "scripted-proposer-fixture") {
		t.Fatalf("fixture label %q", FixtureLabel)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestUnscriptedA2Counts pins the honest-null accounting kept separately beside
// the fixture.
func TestUnscriptedA2Counts(t *testing.T) {
	a2 := &cellRun{spec: cellSpec{Arm: ArmA2}, roleBProposed: 4, roleBRejected: 4}
	fixtureRun := &cellRun{spec: cellSpec{Arm: ArmA2f}, roleBProposed: 6, roleBValidated: 1, roleBApplied: 1, roleBMeasured: 1, roleBRollback: 1}
	proposed, applied, rolled, rejected, validated := unscriptedA2Counts([]*cellRun{a2, fixtureRun})
	if proposed != 4 || applied != 0 || rolled != 0 || rejected != 4 || validated != 0 {
		t.Fatalf("unscripted counts proposed=%d applied=%d rolled=%d rejected=%d validated=%d", proposed, applied, rolled, rejected, validated)
	}
}
