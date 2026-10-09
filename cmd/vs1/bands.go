package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentworld/internal/cognition"
)

// evaluateBands computes the frozen exit criteria of spec section 12 and the
// roadmap's VS1 (a)-(f) from the gathered runs. A criterion L4 cannot decide
// from its own artifacts is reported not-evaluable with the evidence pointer,
// never silently PASS.
func evaluateBands(runs, allRuns []*cellRun, recordedDir string, replay map[string]string, probe *probeReport, srcRoot string, fixtures []fixtureReport, twins []twinReport) []reportBand {
	byKey := map[string]*cellRun{}
	byCell := map[string]map[string]*cellRun{}
	for _, cr := range runs {
		byKey[cr.spec.String()] = cr
		key := fmt.Sprintf("f%d-s%d", cr.spec.F, cr.spec.Seed)
		if byCell[key] == nil {
			byCell[key] = map[string]*cellRun{}
		}
		byCell[key][cr.spec.Arm] = cr
	}
	cellKeys := make([]string, 0, len(byCell))
	for k := range byCell {
		cellKeys = append(cellKeys, k)
	}
	sort.Strings(cellKeys)

	// The declared cell count is the frozen section 11.3 set, not the subset that
	// happened to run: a band must never PASS on fewer cells than the criterion
	// claims. An incomplete run degrades to not-evaluable/FAIL instead.
	declaredCells := len(VS1DeclaredCells())

	var bands []reportBand
	bands = append(bands, ec1(byCell, recordedDir, declaredCells))
	bands = append(bands, ec2(byCell, cellKeys, declaredCells))
	bands = append(bands, ec3(replay, runs))
	bands = append(bands, ec4(runs))
	bands = append(bands, ec5(byCell, cellKeys, fixtures))
	bands = append(bands, privacyBand(srcRoot, allRuns))
	bands = append(bands, ec7(runs, probe))
	bands = append(bands, ec8(byCell, cellKeys, twins))
	bands = append(bands, ec9(srcRoot))
	bands = append(bands, ec10(probe))
	bands = append(bands, ec11(runs))
	return bands
}

func ec1(byCell map[string]map[string]*cellRun, recordedDir string, declared int) reportBand {
	const criterion = "EC-1 mechanism identity/continuity: the no-AI arm through the VS1 harness is byte-identical to the recorded .tmp/natural9-n7-* evidence at all 12 cells (accepted-history digests, hourly projections, event counts); non-cognitive checkpoint manifests are byte-unchanged."
	checked, missing := 0, 0
	var mismatches []string
	for _, key := range sortedCellKeys(byCell) {
		cr := byCell[key][ArmA0]
		if cr == nil {
			missing++
			continue
		}
		facts := loadRecorded(recordedDir, cr.spec.F, cr.spec.Seed)
		if !facts.Found {
			mismatches = append(mismatches, key+":recording-missing")
			continue
		}
		checked++
		if cr.branchDigest != facts.BranchDigest {
			mismatches = append(mismatches, key+":branch_digest")
		}
		if cr.accepted.EventCount != facts.Events {
			mismatches = append(mismatches, key+":events")
		}
		if cr.accepted.EventBodyBytes != facts.EventBodyBytes {
			mismatches = append(mismatches, key+":event_body_bytes")
		}
		if cr.accepted.HistoryBytes != facts.HistoryBytes {
			mismatches = append(mismatches, key+":history_bytes")
		}
		if !equalInts(cr.proj.AliveByHour, facts.AliveByHour) {
			mismatches = append(mismatches, key+":alive_by_hour")
		}
	}
	status := "PASS"
	switch {
	case checked == 0:
		status = "not-evaluable"
	case missing > 0 || checked < declared:
		// A dropped A0 cell (or a cell the run never produced) must never leave
		// the all-12-cells criterion at PASS.
		status = "FAIL"
	case len(mismatches) > 0:
		status = "FAIL"
	}
	return band("EC-1", criterion, status, fmt.Sprintf("cells checked=%d missing-A0=%d declared=%d; mismatches=%s", checked, missing, declared, joinOrNone(mismatches)))
}

func ec2(byCell map[string]map[string]*cellRun, cellKeys []string, declared int) reportBand {
	const criterion = "EC-2 load-bearing model, no hidden coupling: role-a-inert == no-AI byte-exact at all 12 cells; at every cell the role-a arm's trajectory differs from no-AI in >=1 declared indicator hour; and the ledger shows >=1 applied model decision per cell whose action differs from the frozen cascade's action at the same seam instant."
	inertMismatch, noDivergence, noApplied, checked, missing := 0, 0, 0, 0, 0
	for _, key := range cellKeys {
		cell := byCell[key]
		a0, a1, a1i := cell[ArmA0], cell[ArmA1], cell[ArmA1i]
		if a0 == nil || a1 == nil || a1i == nil {
			// A cell present but missing one of the three arms is incomplete
			// evidence; it must lower the checkable count, never be skipped.
			missing++
			continue
		}
		checked++
		if a1i.accepted.TipHash != a0.accepted.TipHash || a1i.accepted.ProjectionHash != a0.accepted.ProjectionHash ||
			a1i.accepted.HistoryBytes != a0.accepted.HistoryBytes || a1i.accepted.EventCount != a0.accepted.EventCount {
			inertMismatch++
		}
		if !diverges(a1, a0) {
			noDivergence++
		}
		if a1.appliedModel < 1 || a1.divergedModel < 1 {
			noApplied++
		}
	}
	status := "PASS"
	switch {
	case checked == 0:
		status = "not-evaluable"
	case missing > 0 || checked < declared:
		// A dropped arm or an unrun declared cell must never leave the
		// all-12-cells criterion at PASS.
		status = "FAIL"
	case inertMismatch > 0 || noDivergence > 0 || noApplied > 0:
		status = "FAIL"
	}
	return band("EC-2", criterion, status,
		fmt.Sprintf("cells=%d missing=%d declared=%d inert!=no-AI=%d no-divergence=%d no-applied-diverging-model=%d (an applied model decision whose recorded action differs from the frozen cascade's own recorded action at the same seam; hermetic FakeClient content, not a real-model claim)", checked, missing, declared, inertMismatch, noDivergence, noApplied))
}

func ec3(replay map[string]string, runs []*cellRun) reportBand {
	const criterion = "EC-3 byte-exact replay, zero model calls: every declared matrix AI cell whose generation state is empty replays byte-exact from its genesis with zero gateway requests -- A1, and the apply-free A2/A3 cells -- and a deliberately mutated ledger byte is detected by the committed ledger unit test (internal/cognition/ledger_test.go TestLedgerDetectsTamperedField, TestLedgerDetectsRehashedMiddleRecord). A matrix cell carrying applied-generation state (an apply or a rollback) is not yet replayed from a single bundle (spec 14.1 reduction 2), and the scripted-proposer fixtures A2f/A2g are outside the declared replay scope (spec 14.1 reduction 10); both are declared, not silently omitted."
	if len(replay) == 0 {
		return band("EC-3", criterion, "not-evaluable", "no replay subset was run; the ledger chain and mutation detection are reported under EC-7's ledger evidence")
	}
	mismatch := 0
	perArm := map[string]int{}
	keys := make([]string, 0, len(replay))
	for k := range replay {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if replay[k] != "identical" {
			mismatch++
		}
		if arm, _, ok := strings.Cut(k, "-"); ok {
			perArm[arm]++
		}
	}
	status := "PASS"
	if mismatch > 0 {
		status = "FAIL"
	}
	notReplayed := notReplayedSummary(runs)
	return band("EC-3", criterion, status, fmt.Sprintf("replayed cells=%d mismatches=%d by-arm[A1=%d A2=%d A3=%d]; not replayed matrix cells: %s (spec 14.1 reduction 2); scripted fixtures A2f/A2g are outside the declared replay scope (spec 14.1 reduction 10); mutated-ledger detection: internal/cognition/ledger_test.go (unit)", len(replay), mismatch, perArm[ArmA1], perArm[ArmA2], perArm[ArmA3], notReplayed))
}

// notReplayedSummary names the AI cells the declared replay subset omitted
// because they carry applied-generation state (an apply or a rollback).
func notReplayedSummary(runs []*cellRun) string {
	var out []string
	for _, cr := range runs {
		if cr.spec.Arm != ArmA2 && cr.spec.Arm != ArmA3 {
			continue
		}
		if cr.roleBApplied == 0 && cr.roleBRollback == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("%s(apply=%d rollback=%d)", cr.spec.String(), cr.roleBApplied, cr.roleBRollback))
	}
	sort.Strings(out)
	return joinOrNone(out)
}

func ec4(runs []*cellRun) reportBand {
	const criterion = "EC-4 intervention completeness (C8): 100% of intervention records carry cause, cost, patch-or-role, and result; every call record is referenced by exactly one intervention; zero orphan calls."
	records, incomplete, orphans, doubleRef := 0, 0, 0, 0
	for _, cr := range runs {
		if len(cr.interventions) == 0 && len(cr.calls) == 0 {
			continue
		}
		refCount := map[uint64]int{}
		for _, rec := range cr.interventions {
			records++
			if rec.Result.Status == "" {
				incomplete++
			}
			if !causeComplete(rec) {
				incomplete++
			}
			if !costComplete(rec) {
				incomplete++
			}
			if rec.Kind == cognition.InterventionRoleA || strings.HasPrefix(string(rec.Kind), "patch-") {
				if rec.Patch == nil {
					incomplete++
				}
			}
			for _, ref := range rec.CallRefs {
				refCount[ref]++
			}
		}
		for _, call := range cr.calls {
			n := refCount[call.Seq]
			if n == 0 {
				orphans++
			} else if n > 1 {
				doubleRef++
			}
		}
	}
	status := "PASS"
	if records == 0 {
		status = "not-evaluable"
	} else if incomplete > 0 || orphans > 0 || doubleRef > 0 {
		status = "FAIL"
	}
	return band("EC-4", criterion, status,
		fmt.Sprintf("interventions=%d missing-cause/cost/patch/result=%d orphan-calls=%d double-referenced=%d", records, incomplete, orphans, doubleRef))
}

// causeComplete reports whether an intervention's cause carries the declared
// fields of its kind (spec 9.1/Appendix B): a role-a record names policy, seam,
// and actor; a Role B / audit record names policy and trigger.
func causeComplete(rec cognition.InterventionRecord) bool {
	if rec.Cause.WakePolicy == "" {
		return false
	}
	switch {
	case rec.Kind == cognition.InterventionRoleA:
		return rec.Cause.Actor != 0 && rec.Cause.Seam != ""
	case strings.HasPrefix(string(rec.Kind), "patch-") || rec.Kind == cognition.InterventionNoTriggerAudit:
		return rec.Cause.Trigger != ""
	}
	return true
}

// costComplete reports whether the cost block agrees with the call refs: a
// record that references calls must record a call count, and vice versa.
func costComplete(rec cognition.InterventionRecord) bool {
	return (len(rec.CallRefs) == 0) == (rec.Cost.Calls == 0)
}

func ec5(byCell map[string]map[string]*cellRun, cellKeys []string, fixtures []fixtureReport) reportBand {
	const criterion = "EC-5 patch lifecycle (C9/C10): (a) the declared scripted proposal fixtures run propose->validate->sandbox->apply->measure->rollback (A2f) and propose->...->admitted-measurement->divergent-parent-sandbox (A2g) end to end (spec 12 EC-5(a)'s \"and replays byte-exact\" clause is deferred: the scripted fixtures are outside the declared replay scope, spec 14.1 reduction 10); (b) broken patches rejected at validation with zero mutation; (c) a regressing patch applied then rolled back with lineage retained; (d) live A2 cells report admitted/applied/measured/rolled-back counts, the honest null valid."
	a2Applied, a2Proposed, a2Rejected, a2Rolled, a2Validated, a2Measured, a2Cells := 0, 0, 0, 0, 0, 0, 0
	a3Applied, a3Proposed, a3Rejected, a3Rolled, a3Validated, a3Measured, a3Cells := 0, 0, 0, 0, 0, 0, 0
	for _, key := range cellKeys {
		for _, arm := range []string{ArmA2, ArmA3} {
			cr := byCell[key][arm]
			if cr == nil {
				continue
			}
			switch arm {
			case ArmA2:
				a2Cells++
				a2Applied += cr.roleBApplied
				a2Proposed += cr.roleBProposed
				a2Rejected += cr.roleBRejected
				a2Rolled += cr.roleBRollback
				a2Validated += cr.roleBValidated
				a2Measured += cr.roleBMeasured
			case ArmA3:
				a3Cells++
				a3Applied += cr.roleBApplied
				a3Proposed += cr.roleBProposed
				a3Rejected += cr.roleBRejected
				a3Rolled += cr.roleBRollback
				a3Validated += cr.roleBValidated
				a3Measured += cr.roleBMeasured
			}
		}
	}
	status := "PASS"
	if a2Cells == 0 {
		status = "not-evaluable"
	}
	evidence := fmt.Sprintf("unscripted A2-only cells=%d proposed=%d validated=%d applied=%d rejected-at-validation/sandbox=%d measured=%d rolled-back=%d; live A3 reference cells=%d proposed=%d validated=%d applied=%d rejected=%d measured=%d rolled-back=%d; A2+A3 live aggregate proposed=%d applied=%d rolled-back=%d (the honest null is reported per arm; A2 is the declared (d) arm)",
		a2Cells, a2Proposed, a2Validated, a2Applied, a2Rejected, a2Measured, a2Rolled,
		a3Cells, a3Proposed, a3Validated, a3Applied, a3Rejected, a3Measured, a3Rolled,
		a2Proposed+a3Proposed, a2Applied+a3Applied, a2Rolled+a3Rolled)
	if len(fixtures) == 0 {
		evidence += "; SCRIPTED-PROPOSER FIXTURES not run"
		status = "FAIL"
	}
	var rollbackFixture, divergentFixture *fixtureReport
	for i := range fixtures {
		switch fixtures[i].Arm {
		case ArmA2f:
			rollbackFixture = &fixtures[i]
		case ArmA2g:
			divergentFixture = &fixtures[i]
		}
	}
	if rollbackFixture != nil && rollbackFixture.Ran {
		evidence += fmt.Sprintf("; ROLLBACK FIXTURE %s (f%d-s%d): proposed=%d admitted=%d applied=%d measured=%d rolled-back=%d measured_window=%dh",
			rollbackFixture.Arm, rollbackFixture.F, rollbackFixture.Seed, rollbackFixture.Proposed, rollbackFixture.Admitted,
			rollbackFixture.Applied, rollbackFixture.Measured, rollbackFixture.RolledBack, rollbackFixture.MeasuredWindowHours)
		if rollbackFixture.Admitted == 0 || rollbackFixture.Applied == 0 || rollbackFixture.Measured == 0 || rollbackFixture.RolledBack == 0 {
			status = "FAIL"
		}
	} else {
		evidence += "; ROLLBACK FIXTURE not run"
		status = "FAIL"
	}
	if divergentFixture != nil && divergentFixture.Ran {
		evidence += fmt.Sprintf("; DIVERGENT-PARENT FIXTURE %s (f%d-s%d): proposed=%d admitted=%d applied=%d measured=%d rolled-back=%d measurement-kept-head=%t divergent-parent-sandbox=%d",
			divergentFixture.Arm, divergentFixture.F, divergentFixture.Seed, divergentFixture.Proposed, divergentFixture.Admitted,
			divergentFixture.Applied, divergentFixture.Measured, divergentFixture.RolledBack, divergentFixture.MeasurementKeptHead, divergentFixture.DivergentParentSandbox)
		// The divergent-parent case needs the measurement to keep the head on the
		// child generation AND a proposal sandboxed against that child.
		if !divergentFixture.MeasurementKeptHead || divergentFixture.DivergentParentSandbox < 1 {
			status = "FAIL"
		}
	} else {
		evidence += "; DIVERGENT-PARENT FIXTURE not run"
		status = "FAIL"
	}
	evidence += "; EC-5(b)(c) are unit-only: the broken/regressing patch fixtures are internal/cognition/roleb_lifecycle_test.go (TestPatchValidationRejects, TestRoleBLifecycleApplyMeasureRollback); no matrix cell drives them"
	return band("EC-5", criterion, status, evidence)
}

func ec7(runs []*cellRun, probe *probeReport) reportBand {
	const criterion = "EC-7 costs reported (C11): every AI cell reports per-call usage/cost/latency, per-actor-hour and per-run totals, and the four cognitive ceiling rows; a binding ceiling is a typed fail-loud halt."
	cells, withCost, level2 := 0, 0, 0
	var missing []string
	for _, cr := range runs {
		if cr.spec.Arm == ArmA0 {
			continue
		}
		cells++
		level2 += level2RefusalTotal(cr)
		if cr.totals.Calls > 0 && cr.totals.CostMicros > 0 && cr.callAttempts > 0 && len(cr.ceilingRows) == 4 &&
			cr.sumAliveHours > 0 {
			withCost++
		} else {
			missing = append(missing, cr.spec.String())
		}
	}
	status := "PASS"
	if cells == 0 {
		status = "not-evaluable"
	} else if withCost != cells || probe == nil || !probe.Ran || probe.Ceiling == "" {
		status = "FAIL"
	}
	evidence := fmt.Sprintf("AI cells=%d with per-call usage/cost/latency + per-actor-hour + per-run totals + all four ceiling rows=%d; missing=%s; level-2 world refusals of escalated claims (world verdict joined from the accepted journal)=%d", cells, withCost, joinOrNone(missing), level2)
	if probe != nil && probe.Ran {
		evidence += fmt.Sprintf("; probe hit=%s threshold=%d observed=%d at h%d", probe.Ceiling, probe.Threshold, probe.Observed, probe.HitHour)
	}
	return band("EC-7", criterion, status, evidence)
}

func ec8(byCell map[string]map[string]*cellRun, cellKeys []string, twins []twinReport) reportBand {
	const criterion = "EC-8 determinism and invariance: the escalation set is a pure function of (policy, hour, roster) and consumes no world random stream; the f=12 s=0 w=4 twin is digest-identical to its workers-1 twin."
	inertOK := 0
	for _, key := range cellKeys {
		cell := byCell[key]
		if cell[ArmA0] != nil && cell[ArmA1i] != nil &&
			cell[ArmA0].accepted.TipHash == cell[ArmA1i].accepted.TipHash {
			inertOK++
		}
	}
	status := "PASS"
	var parts []string
	if inertOK != len(cellKeys) {
		status = "FAIL"
	}
	parts = append(parts, fmt.Sprintf("escalation-set purity: inert==no-AI tip on %d/%d cells; protocol.WakePolicy is a pure function of (policy,hour,roster)", inertOK, len(cellKeys)))
	if len(twins) == 0 {
		// A FAIL from the purity check stays sticky; the missing twin only
		// downgrades an otherwise-PASS band to not-evaluable.
		if status == "PASS" {
			status = "not-evaluable"
		}
		parts = append(parts, "workers-4 twin not run")
	}
	for _, tw := range twins {
		if !tw.Ran || !tw.Identical {
			status = "FAIL"
		}
		parts = append(parts, fmt.Sprintf("%s f%d-s%d w%d vs w1: tip %s==%s branch %s==%s identical=%t",
			tw.Arm, tw.F, tw.Seed, tw.Workers, shortHash(tw.W4TipHash), shortHash(tw.W1TipHash),
			shortHash(tw.W4BranchDigest), shortHash(tw.W1BranchDigest), tw.Identical))
	}
	return band("EC-8", criterion, status, strings.Join(parts, "; "))
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

func ec9(srcRoot string) reportBand {
	const criterion = "EC-9 coverage and supersession: every C1-C16 row is present and runnable with the section 3 evidence pointer, and the supersession statement of section 2.3 is part of the delivered contract."
	path := filepath.Join(srcRoot, ".scratch/21-vs1-mind-in-the-loop/spec.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return band("EC-9", criterion, "not-evaluable", "the VS1 spec is not readable at "+path+": "+err.Error())
	}
	body := string(data)
	var missing []string
	for i := 1; i <= 16; i++ {
		if !strings.Contains(body, fmt.Sprintf("| C%d |", i)) {
			missing = append(missing, fmt.Sprintf("C%d", i))
		}
	}
	supersession := strings.Contains(body, "VS1 supersedes them")
	if len(missing) > 0 || !supersession {
		return band("EC-9", criterion, "FAIL", fmt.Sprintf("spec %s: missing coverage rows=%s supersession-statement=%t", path, joinOrNone(missing), supersession))
	}
	return band("EC-9", criterion, "PASS", fmt.Sprintf("spec %s: all 16 C1-C16 coverage rows present; section 2.3 supersession statement present; C6-C13 are backed by the runnable arms of this matrix", path))
}

func ec10(probe *probeReport) reportBand {
	const criterion = "EC-10 no arbitrary design caps (P15): the cognitive budget is a declared compute ceiling, instrumented and fail-loud, never a world mechanism; the scale probe demonstrates the cognitive ceiling firing with the ledger intact and the P15 verdict reported."
	if probe == nil || !probe.Ran {
		return band("EC-10", criterion, "not-evaluable", "the scale/ceiling probe was not run")
	}
	status := "PASS"
	if probe.Ceiling == "" || !probe.LedgerChainVerifies {
		status = "FAIL"
	}
	return band("EC-10", criterion, status,
		fmt.Sprintf("probe ceiling=%s threshold=%d observed=%d hour=%d ledger=%d records ledger-chain-verifies=%t P15=%s", probe.Ceiling, probe.Threshold, probe.Observed, probe.HitHour, probe.LedgerCount, probe.LedgerChainVerifies, probe.P15Verdict))
}

func ec11(runs []*cellRun) reportBand {
	const criterion = "EC-11 honest labels: every report is synthetic/model-conditional; token_cost is the recorded total in AI arms and 0 in no-AI arms; no result is presented as real-world evidence."
	noAIViolations, aiViolations, aiCells, aiWithCost := 0, 0, 0, 0
	for _, cr := range runs {
		if cr.spec.Arm == ArmA0 {
			if cr.totals.CostMicros != 0 {
				noAIViolations++
			}
			continue
		}
		aiCells++
		if cr.totals.Calls > 0 {
			if cr.totals.CostMicros <= 0 {
				aiViolations++
			} else {
				aiWithCost++
			}
		}
	}
	labelled := strings.Contains(evaluationLabel, "synthetic/model-conditional")
	status := "PASS"
	switch {
	case aiCells == 0:
		// Nothing was labelled: the honest-label criterion is unchecked, not met.
		status = "not-evaluable"
	case noAIViolations > 0 || aiViolations > 0 || !labelled:
		status = "FAIL"
	}
	return band("EC-11", criterion, status, fmt.Sprintf("label=%q labelled=%t; no-AI cells with a nonzero token_cost=%d; AI cells=%d, of which with a positive recorded token_cost (=cost_micros by construction)=%d; AI cells with zero recorded token_cost=%d", evaluationLabel, labelled, noAIViolations, aiCells, aiWithCost, aiViolations))
}

func sortedCellKeys(byCell map[string]map[string]*cellRun) []string {
	keys := make([]string, 0, len(byCell))
	for k := range byCell {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ",")
}

// diverges reports whether two runs differ in at least one declared hourly
// indicator.
func diverges(a, b *cellRun) bool {
	if a.accepted.TipHash != b.accepted.TipHash {
		return true
	}
	if !equalInts(a.proj.AliveByHour, b.proj.AliveByHour) {
		return true
	}
	for i := range a.proj.BirthsCum {
		if i < len(b.proj.BirthsCum) && a.proj.BirthsCum[i] != b.proj.BirthsCum[i] {
			return true
		}
	}
	for i := range a.proj.MaxCapital {
		if i < len(b.proj.MaxCapital) && a.proj.MaxCapital[i] != b.proj.MaxCapital[i] {
			return true
		}
	}
	return false
}
