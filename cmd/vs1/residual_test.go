package main

import (
	"context"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

// The residual-closure tests: the two integration fixtures, the final-bundle
// save-failure report, the per-patch Sum(k) evidence, and the two band guards
// that must not PASS without the evidence they claim.

// TestDivergentMeasurementFixtureExercisesDivergentParent (R2) pins the A2g
// fixture end to end: the h72 measurement ADMITS (no rollback), so the head
// stays on the child generation and the h72 proposal is sandboxed against that
// divergent parent (a cognitive generation, not the frozen base). Reverting the
// A2g Role A script to provision-first makes the measurement roll back, and the
// assertion fails.
func TestDivergentMeasurementFixtureExercisesDivergentParent(t *testing.T) {
	root := t.TempDir()
	spec := cellSpec{Arm: ArmA2g, F: scriptedFixtureFounders, Seed: scriptedFixtureSeed, Workers: 1}
	cr, err := runCell(context.Background(), spec, scriptedFixture(), root, nil)
	if err != nil {
		t.Fatalf("run A2g fixture: %v", err)
	}
	if cr.roleBMeasured < 1 {
		t.Fatalf("the fixture recorded no measurement: %+v", cr)
	}
	if cr.roleBRollback != 0 {
		t.Fatalf("the h72 measurement rolled back (head did not stay on the child): applied=%d measured=%d rolled-back=%d", cr.roleBApplied, cr.roleBMeasured, cr.roleBRollback)
	}
	if cr.roleBDivergentParent < 1 {
		t.Fatalf("no proposal was sandboxed against the divergent child parent: %+v", cr)
	}
	// The ledger itself must show the divergent parent: a patch-validate whose
	// proposal parent is a cognitive generation.
	diverged := false
	for _, rec := range cr.interventions {
		if rec.Kind == cognition.InterventionPatchValidate && rec.Patch != nil &&
			rec.Patch.ParentRef.ID == strategy.NaturalCognitiveRefID && rec.Patch.ChildRef.ID == strategy.NaturalCognitiveRefID {
			diverged = true
		}
	}
	if !diverged {
		t.Fatal("the intervention ledger has no validate record sandboxed against a cognitive parent")
	}
}

// TestFinalBundleSaveFailureReported (R3) pins the final-bundle save-failure
// report: the post-rollback save is refused, and the failure reaches both
// cellRun.finalBundleError and cellReport.FinalBundleError rather than being
// swallowed. Deleting the assignment in finalizeStack or the copy in
// cellReportOf fails this test.
func TestFinalBundleSaveFailureReported(t *testing.T) {
	root := t.TempDir()
	spec := cellSpec{Arm: ArmA2f, F: scriptedFixtureFounders, Seed: scriptedFixtureSeed, Workers: 1}
	cr, err := runCell(context.Background(), spec, scriptedFixture(), root, nil)
	if err != nil {
		t.Fatalf("run A2f fixture: %v", err)
	}
	if cr.roleBRollback < 1 {
		t.Fatalf("the rollback fixture did not roll back: %+v", cr)
	}
	if cr.finalBundleError == "" {
		t.Fatal("the post-rollback final-bundle save failed silently: cellRun.finalBundleError is empty")
	}
	rep := cellReportOf(cr)
	if rep.FinalBundleError != cr.finalBundleError {
		t.Fatalf("cellReport.FinalBundleError %q != cellRun.finalBundleError %q", rep.FinalBundleError, cr.finalBundleError)
	}
}

// TestBuildEvidencePatchCapitalSumsPerPatch (R3) pins the per-patch Sum(k) of
// the Role B evidence packet: two actors on one patch with capital 2 and 3 must
// report patch capital 5, not 0. Dropping the patchCapital accumulation fails
// this test.
func TestBuildEvidencePatchCapitalSumsPerPatch(t *testing.T) {
	patch := sim.EntityID(1001)
	actor := func(capital int64) world.NaturalActorState {
		return world.NaturalActorState{
			Body:     world.NaturalBodyState{Energy: 5},
			Bag:      world.NaturalBagState{Source: patch},
			Worksite: world.NaturalWorksiteState{Capital: capital},
			Genome:   world.NaturalGenomeState{DiedHour: world.NaturalNeverHour},
		}
	}
	checks := []world.NaturalCheckpoint{
		{Hour: 0, Founders: []world.NaturalActorState{actor(2), actor(3)}},
		{Hour: 24, Founders: []world.NaturalActorState{actor(2), actor(3)}},
	}
	ev := buildEvidence(checks, nil, 0, 24)
	if len(ev.Patches) != 1 {
		t.Fatalf("patches %+v, want one", ev.Patches)
	}
	if ev.Patches[0].Mouths != 2 {
		t.Fatalf("patch mouths %d, want 2", ev.Patches[0].Mouths)
	}
	if ev.Patches[0].Capital != 5 {
		t.Fatalf("patch capital %d, want the per-patch Sum(k) 5", ev.Patches[0].Capital)
	}
}

// TestEC2RequiresDivergingAppliedModel (R4) is the mutation check for the
// load-bearing comparison: a cell with applied model decisions but none whose
// action differs from the frozen cascade must FAIL. Without the divergence
// comparison the band would PASS on the applied-model count alone.
func TestEC2RequiresDivergingAppliedModel(t *testing.T) {
	byCell := func(applied, diverged int) map[string]map[string]*cellRun {
		return map[string]map[string]*cellRun{
			"f10-s0": {
				ArmA0:  {proj: projections{AliveByHour: []int{1}}},
				ArmA1:  {appliedModel: applied, divergedModel: diverged, proj: projections{AliveByHour: []int{2}}},
				ArmA1i: {proj: projections{AliveByHour: []int{1}}},
			},
		}
	}
	keys := []string{"f10-s0"}
	if got := ec2(byCell(3, 0), keys); got.Status != "FAIL" {
		t.Fatalf("EC-2 %s with applied-but-not-diverging decisions; evidence %q", got.Status, got.Evidence)
	}
	if got := ec2(byCell(3, 1), keys); got.Status != "PASS" {
		t.Fatalf("EC-2 %s with a diverging applied decision; evidence %q", got.Status, got.Evidence)
	}
}

// TestReuseReplayNeverCarriesStaleVerdict (R1) is the mutation check for the
// -reuse guard: a prior evaluation's own "identical" verdict must never be
// carried over without re-derivation. The deleted shortcut returned "identical"
// straight from the prior subset; the guard records it as not-re-derived
// instead, so EC-3 fails rather than passing on stale evidence.
func TestReuseReplayNeverCarriesStaleVerdict(t *testing.T) {
	prior := evaluation{ReplaySubset: []string{"A1-f10-s0", "A2-f10-s0"}}
	got := mergeReuseReplay(prior, map[string]string{"A1-f10-s0": "identical"})
	if got["A1-f10-s0"] != "identical" {
		t.Fatalf("a re-derived verdict was overwritten: %q", got["A1-f10-s0"])
	}
	if got["A2-f10-s0"] == "identical" {
		t.Fatalf("a stale prior verdict was carried over without re-derivation: %q", got["A2-f10-s0"])
	}
}

// TestPrivacyBandRequiresRecordedRoleARequests (R4) is the mutation check for
// the EC-6 requirement that the band scanned at least one recorded Role A
// request. Removing the recordedRoleA guard PASSes on the synthetic sentinel
// alone.
func TestPrivacyBandRequiresRecordedRoleARequests(t *testing.T) {
	band := privacyBand(moduleRoot(t), nil)
	if band.Status != "FAIL" {
		t.Fatalf("EC-6 %s with zero recorded Role A requests; evidence %q", band.Status, band.Evidence)
	}
}
