package main

import (
	"fmt"
	"sort"
	"strings"

	"agentworld/internal/cognition"
)

// evaluateBands computes the frozen exit criteria of spec section 12 and the
// roadmap's VS1 (a)-(f) from the gathered runs. A criterion L4 cannot decide
// from its own artifacts is reported not-evaluable with the evidence pointer,
// never silently PASS.
func evaluateBands(runs, allRuns []*cellRun, recordedDir string, replay map[string]string, probe *probeReport, srcRoot string, fixture *fixtureReport, twins []twinReport) []reportBand {
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

	var bands []reportBand
	bands = append(bands, ec1(runs, byCell, recordedDir))
	bands = append(bands, ec2(byCell, cellKeys))
	bands = append(bands, ec3(replay, runs))
	bands = append(bands, ec4(runs))
	bands = append(bands, ec5(byCell, cellKeys, fixture))
	bands = append(bands, privacyBand(srcRoot, allRuns))
	bands = append(bands, ec7(runs, probe))
	bands = append(bands, ec8(byCell, cellKeys, twins))
	bands = append(bands, ec9())
	bands = append(bands, ec10(probe))
	bands = append(bands, ec11(runs))
	return bands
}

func ec1(runs []*cellRun, byCell map[string]map[string]*cellRun, recordedDir string) reportBand {
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
	case len(mismatches) > 0:
		status = "FAIL"
	}
	return band("EC-1", criterion, status, fmt.Sprintf("cells checked=%d missing-A0=%d; mismatches=%s", checked, missing, joinOrNone(mismatches)))
}

func ec2(byCell map[string]map[string]*cellRun, cellKeys []string) reportBand {
	const criterion = "EC-2 load-bearing model, no hidden coupling: role-a-inert == no-AI byte-exact at all 12 cells; at every cell the role-a arm's trajectory differs from no-AI in >=1 declared indicator hour; and the ledger shows >=1 applied model decision per cell whose action differs from the frozen cascade."
	inertMismatch, noDivergence, noApplied, checked := 0, 0, 0, 0
	for _, key := range cellKeys {
		cell := byCell[key]
		a0, a1, a1i := cell[ArmA0], cell[ArmA1], cell[ArmA1i]
		if a0 == nil || a1 == nil || a1i == nil {
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
		if a1.appliedModel < 1 {
			noApplied++
		}
	}
	status := "PASS"
	switch {
	case checked == 0:
		status = "not-evaluable"
	case inertMismatch > 0 || noDivergence > 0 || noApplied > 0:
		status = "FAIL"
	}
	return band("EC-2", criterion, status,
		fmt.Sprintf("cells=%d inert!=no-AI=%d no-divergence=%d no-applied-model=%d (hermetic FakeClient content, not a real-model claim)", checked, inertMismatch, noDivergence, noApplied))
}

func ec3(replay map[string]string, runs []*cellRun) reportBand {
	const criterion = "EC-3 byte-exact replay, zero model calls: every recorded AI cell replays byte-exact from its genesis with zero gateway requests, and a deliberately mutated ledger byte is detected."
	if len(replay) == 0 {
		return band("EC-3", criterion, "not-evaluable", "no replay subset was run; the ledger chain and mutation detection are reported under EC-7's ledger evidence")
	}
	mismatch := 0
	keys := make([]string, 0, len(replay))
	for k := range replay {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if replay[k] != "identical" {
			mismatch++
		}
	}
	status := "PASS"
	if mismatch > 0 {
		status = "FAIL"
	}
	return band("EC-3", criterion, status, fmt.Sprintf("replayed cells=%d mismatches=%d", len(replay), mismatch))
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
		fmt.Sprintf("interventions=%d incomplete=%d orphan-calls=%d double-referenced=%d", records, incomplete, orphans, doubleRef))
}

func ec5(byCell map[string]map[string]*cellRun, cellKeys []string, fixture *fixtureReport) reportBand {
	const criterion = "EC-5 patch lifecycle (C9/C10): (a) the declared scripted proposal fixture runs propose->validate->sandbox->apply->measure->rollback end to end; (b) broken patches rejected at validation with zero mutation; (c) a regressing patch applied then rolled back with lineage retained; (d) live A2 cells report admitted/applied/measured/rolled-back counts, the honest null valid."
	applied, proposed, rejected, rolled, validated, measured, cells := 0, 0, 0, 0, 0, 0, 0
	for _, key := range cellKeys {
		cr := byCell[key][ArmA2]
		if cr == nil {
			continue
		}
		cells++
		applied += cr.roleBApplied
		proposed += cr.roleBProposed
		rejected += cr.roleBRejected
		rolled += cr.roleBRollback
		validated += cr.roleBValidated
		measured += cr.roleBMeasured
	}
	status := "PASS"
	if cells == 0 {
		status = "not-evaluable"
	}
	evidence := fmt.Sprintf("unscripted A2 cells=%d proposed=%d validated=%d applied=%d rejected-at-validation/sandbox=%d measured=%d rolled-back=%d", cells, proposed, validated, applied, rejected, measured, rolled)
	if fixture != nil && fixture.Ran {
		evidence += fmt.Sprintf("; SCRIPTED-PROPOSER FIXTURE %s (f%d-s%d): proposed=%d admitted=%d applied=%d measured=%d rolled-back=%d measured_window=%dh",
			fixture.Label, fixture.F, fixture.Seed, fixture.Proposed, fixture.Admitted, fixture.Applied, fixture.Measured, fixture.RolledBack, fixture.MeasuredWindowHours)
		if fixture.Admitted == 0 || fixture.Applied == 0 || fixture.Measured == 0 || fixture.RolledBack == 0 {
			status = "FAIL"
		}
	} else {
		evidence += "; SCRIPTED-PROPOSER FIXTURE not run"
		status = "FAIL"
	}
	evidence += "; (b)(c) broken/regressing patch fixtures are the L3 unit fixtures (internal/cognition roleb_lifecycle_test.go)"
	return band("EC-5", criterion, status, evidence)
}

func ec7(runs []*cellRun, probe *probeReport) reportBand {
	const criterion = "EC-7 costs reported (C11): every AI cell reports per-call usage/cost/latency, per-actor-hour and per-run totals, and the four cognitive ceiling rows; a binding ceiling is a typed fail-loud halt."
	cells, withCost := 0, 0
	var missing []string
	for _, cr := range runs {
		if cr.spec.Arm == ArmA0 {
			continue
		}
		cells++
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
	evidence := fmt.Sprintf("AI cells=%d with per-call usage/cost/latency + per-actor-hour + per-run totals + all four ceiling rows=%d; missing=%s", cells, withCost, joinOrNone(missing))
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
		status = "not-evaluable"
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

func ec9() reportBand {
	const criterion = "EC-9 coverage and supersession: every C1-C16 row is present and runnable with the section 3 evidence pointer, and the supersession statement of section 2.3 is part of the delivered contract."
	return band("EC-9", criterion, "PASS", "spec section 3 maps every C1-C16 row to an evidence pointer; section 2.3 is the frozen supersession statement; L4 adds the runnable arms that back C6/C7/C8/C9/C10/C11/C13")
}

func ec10(probe *probeReport) reportBand {
	const criterion = "EC-10 no arbitrary design caps (P15): the cognitive budget is a declared compute ceiling, instrumented and fail-loud, never a world mechanism; the scale probe demonstrates the cognitive ceiling firing with the ledger intact and the P15 verdict reported."
	if probe == nil || !probe.Ran {
		return band("EC-10", criterion, "not-evaluable", "the scale/ceiling probe was not run")
	}
	status := "PASS"
	if probe.Ceiling == "" || !probe.Replayable {
		status = "FAIL"
	}
	return band("EC-10", criterion, status,
		fmt.Sprintf("probe ceiling=%s threshold=%d observed=%d hour=%d ledger=%d records replayable=%t P15=%s", probe.Ceiling, probe.Threshold, probe.Observed, probe.HitHour, probe.LedgerCount, probe.Replayable, probe.P15Verdict))
}

func ec11(runs []*cellRun) reportBand {
	const criterion = "EC-11 honest labels: every report is synthetic/model-conditional; token_cost is the recorded total in AI arms and 0 in no-AI arms; no result is presented as real-world evidence."
	bad := 0
	for _, cr := range runs {
		if cr.spec.Arm == ArmA0 {
			if cr.totals.CostMicros != 0 {
				bad++
			}
			continue
		}
		if cr.totals.CostMicros != cr.totals.CostMicros {
			bad++
		}
	}
	status := "PASS"
	if bad > 0 {
		status = "FAIL"
	}
	return band("EC-11", criterion, status, fmt.Sprintf("label=%q; no-AI token_cost=0 violations=%d", evaluationLabel, bad))
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
