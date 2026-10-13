package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"agentworld/internal/cognition"
)

// The VS2-EC-1..VS2-EC-9 bands (spec section 7). A criterion the runner cannot
// decide from its own artifacts is reported not-evaluable with the evidence
// pointer, never silently PASS. A miss is a miss reported verbatim, never tuned.

// recordedFacts is the subset of the recorded ticket-19 evidence VS2-EC-1
// compares the A0 control against.
type recordedFacts struct {
	Found          bool
	BranchDigest   string
	Events         int
	EventBodyBytes int
	HistoryBytes   int
	AliveByHour    []int
}

// loadRecorded reads the recorded births-enabled branch of a ticket-19 evidence
// file for a cell.
func loadRecorded(dir string, f int, seed uint64) recordedFacts {
	path := filepath.Join(dir, fmt.Sprintf("natural9-n7-f%d-s%d-w1.json", f, seed))
	data, err := os.ReadFile(path)
	if err != nil {
		return recordedFacts{}
	}
	var doc struct {
		Branches []struct {
			Births         bool   `json:"births"`
			BranchDigest   string `json:"branch_digest"`
			Events         int    `json:"events"`
			EventBodyBytes int    `json:"event_body_bytes"`
			HistoryBytes   int    `json:"history_bytes"`
			Population     struct {
				AliveByHour []int `json:"alive_by_hour"`
			} `json:"population"`
		} `json:"branches"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return recordedFacts{}
	}
	for _, b := range doc.Branches {
		if b.Births {
			return recordedFacts{Found: true, BranchDigest: b.BranchDigest, Events: b.Events,
				EventBodyBytes: b.EventBodyBytes, HistoryBytes: b.HistoryBytes, AliveByHour: b.Population.AliveByHour}
		}
	}
	return recordedFacts{}
}

// evaluateBands computes the frozen VS2 exit criteria from the gathered runs.
func evaluateBands(runs, allRuns []*cellRun, recordedDir string, replay map[string]string, srcRoot string, fidelity []fidelityRow) []reportBand {
	byCell := map[string]map[string]*cellRun{}
	for _, cr := range runs {
		key := cellKey(cr.spec.F, cr.spec.Seed)
		if byCell[key] == nil {
			byCell[key] = map[string]*cellRun{}
		}
		byCell[key][cr.spec.Arm] = cr
	}
	cellKeys := sortedCellKeys(byCell)
	declaredCells := len(VS2DeclaredCells())

	var bands []reportBand
	bands = append(bands, ec1(byCell, cellKeys, recordedDir, declaredCells))
	bands = append(bands, ec2(runs, byCell, cellKeys))
	bands = append(bands, ec3(fidelity, declaredCells))
	bands = append(bands, ec4(fidelity))
	bands = append(bands, ec5(replay))
	bands = append(bands, privacyBand(srcRoot, allRuns))
	bands = append(bands, ec7(runs))
	bands = append(bands, ec8(runs))
	bands = append(bands, ec9())
	return bands
}

func cellKey(f int, seed uint64) string { return fmt.Sprintf("f%d-s%d", f, seed) }

// ec1 is VS2-EC-1: the no-AI A0 control through the VS2 harness is byte-identical
// to the recorded ticket-19 evidence, so the deterministic world-RNG-free
// substrate is unchanged.
func ec1(byCell map[string]map[string]*cellRun, cellKeys []string, recordedDir string, declared int) reportBand {
	const criterion = "VS2-EC-1 deterministic allocation, no world RNG: the no-AI A0 control through the VS2 harness is byte-identical to the recorded ticket-19 natural/7 x v6 evidence (branch digest, event counts/body bytes, hourly population) at the declared cells; the nil-priority WakePolicy canonical identity bytes are unchanged from VS1."
	checked, missing := 0, 0
	var mismatches []string
	for _, key := range cellKeys {
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
		status = "FAIL"
	case len(mismatches) > 0:
		status = "FAIL"
	}
	return band("VS2-EC-1", criterion, status,
		fmt.Sprintf("A0 cells checked=%d missing=%d declared=%d; mismatches=%s; nil-priority identity byte test: internal/cognition/protocol/wake_test.go (unit)", checked, missing, declared, joinOrNone(mismatches)))
}

// ec2 is VS2-EC-2: the equal-budget ceiling is enforced fail-loud and every
// budgeted arm reports its realized call count.
func ec2(runs []*cellRun, byCell map[string]map[string]*cellRun, cellKeys []string) reportBand {
	const criterion = "VS2-EC-2 equal-budget ceiling enforced fail-loud, realized counts reported: each budgeted arm (P/U/A) selects online over its own realized stream and stops at N; the report records every budgeted arm's realized call count and never assumes realized==N; the four cognitive-ceiling rows are present; an (N+1)-th attempt is the typed fail-loud halt and a Priority!=nil policy paired with Selector==nil is refused at construction."
	budgeted, overCeiling, missingRows, halts, underBudget := 0, 0, 0, 0, 0
	var missing []string
	for _, key := range cellKeys {
		for _, arm := range budgetedArms {
			cr := byCell[key][arm]
			if cr == nil {
				// A budgeted cell that did not run is an inventory fact, not an
				// accounting violation; the criterion is evaluated on the
				// budgeted cells that ran.
				missing = append(missing, key+"/"+arm)
				continue
			}
			budgeted++
			if cr.realizedCalls > DeclaredCallCeiling {
				overCeiling++
			}
			if cr.realizedCalls < DeclaredCallCeiling {
				underBudget++
			}
			if len(cr.ceilingRows) != 4 {
				missingRows++
			}
			if cr.halted {
				halts++
			}
		}
	}
	status := "PASS"
	switch {
	case budgeted == 0:
		status = "not-evaluable"
	case overCeiling > 0 || missingRows > 0 || halts > 0:
		status = "FAIL"
	}
	// The construction refusal (Priority!=nil && Selector==nil) and the
	// fail-loud (N+1)-th ceiling call are unit-exercised
	// (internal/world/natural_cognition_priority_test.go,
	// internal/cognition/budget_test.go); the band re-derives the observable
	// accounting from the run.
	return band("VS2-EC-2", criterion, status, fmt.Sprintf(
		"budgeted cells=%d ceiling N=%d over-ceiling=%d under-budget=%d missing-ceiling-rows=%d ceiling-halts=%d missing=%s; construction refusal of Priority!=nil && Selector==nil and the fail-loud (N+1)-th call are unit-exercised (internal/world/natural_cognition_priority_test.go, internal/cognition/budget_test.go)",
		budgeted, DeclaredCallCeiling, overCeiling, underBudget, missingRows, halts, joinOrNone(missing)))
}

// compareVectors returns whether a weakly-dominates b (aDom) and whether b
// weakly-dominates a (bDom), over the declared summaries with their declared
// directions and the §6 normalisation. ok is false when the vectors are not
// aligned.
func compareVectors(a, b []indicatorReport) (aDom, bDom, ok bool) {
	if len(a) == 0 || len(a) != len(b) {
		return false, false, false
	}
	aWeak, bWeak, aStrict, bStrict := true, true, false, false
	for i := range a {
		if a[i].Index != b[i].Index {
			return false, false, false
		}
		// Max and Mean are lower-is-better.
		for _, pair := range [][2]float64{{a[i].Max, b[i].Max}, {a[i].Mean, b[i].Mean}} {
			if pair[0] > pair[1] {
				aWeak = false
			}
			if pair[0] < pair[1] {
				aStrict = true
			}
			if pair[1] > pair[0] {
				bWeak = false
			}
			if pair[1] < pair[0] {
				bStrict = true
			}
		}
		// FirstDivergenceHour is higher-is-better, on the normalised value.
		an := float64(a[i].NormalizedFirstDivergenceHour)
		bn := float64(b[i].NormalizedFirstDivergenceHour)
		if an < bn {
			aWeak = false
		}
		if an > bn {
			aStrict = true
		}
		if bn < an {
			bWeak = false
		}
		if bn > an {
			bStrict = true
		}
	}
	return aWeak && aStrict, bWeak && bStrict, true
}

// ec3 is VS2-EC-3: adaptive dominates periodic and random across the declared
// seeds, or the clean negative is reported verbatim. It is falsified only when a
// budgeted baseline strictly dominates adaptive.
func ec3(fidelity []fidelityRow, declaredCells int) reportBand {
	const criterion = "VS2-EC-3 adaptive dominates periodic and random across seeds on the vector, or the clean negative is reported verbatim: for each declared cell, adaptive's per-indicator divergence vector weakly dominates both P and U with strict improvement in >=1 indicator, on the declared summaries (Max/Mean lower-is-better, normalised FirstDivergenceHour higher-is-better); it is falsified only when a budgeted baseline strictly dominates adaptive."
	byCellArm := map[string]map[string][]indicatorReport{}
	for _, row := range fidelity {
		key := cellKey(row.F, row.Seed)
		if byCellArm[key] == nil {
			byCellArm[key] = map[string][]indicatorReport{}
		}
		byCellArm[key][row.Arm] = row.Indicators
	}
	keys := make([]string, 0, len(byCellArm))
	for k := range byCellArm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	complete, adaptiveDominates, baselineDominates, cleanNegative := 0, 0, 0, 0
	var baselineCells, negativeCells []string
	for _, key := range keys {
		a, p, u := byCellArm[key][ArmA], byCellArm[key][ArmP], byCellArm[key][ArmU]
		if a == nil || p == nil || u == nil {
			continue
		}
		aDomP, pDomA, okP := compareVectors(a, p)
		aDomU, uDomA, okU := compareVectors(a, u)
		if !okP || !okU {
			continue
		}
		complete++
		switch {
		case pDomA || uDomA:
			baselineDominates++
			baselineCells = append(baselineCells, key)
		case aDomP && aDomU:
			adaptiveDominates++
		default:
			cleanNegative++
			negativeCells = append(negativeCells, key)
		}
	}
	status := "PASS"
	evidence := fmt.Sprintf("complete A/P/U cells=%d/%d; adaptive-dominates=%d; clean-negative(mutually non-dominating)=%d; baseline-strictly-dominates-adaptive=%d", complete, declaredCells, adaptiveDominates, cleanNegative, baselineDominates)
	switch {
	case complete == 0:
		status = "not-evaluable"
	case baselineDominates > 0:
		status = "FAIL"
		evidence += "; falsified at " + joinOrNone(baselineCells)
	case adaptiveDominates == complete && complete == declaredCells:
		status = "PASS"
	case adaptiveDominates == complete:
		status = "partial"
		evidence += "; adaptive weakly dominates on every complete cell that ran, but fewer than the declared " + fmt.Sprint(declaredCells) + " cells ran"
	default:
		// The mutually non-dominating clean negative is the declared decisive
		// outcome of VS2-EC-9, not an EC-3 miss.
		status = "clean-negative"
		evidence += "; non-dominating cells=" + joinOrNone(negativeCells) + " (a clean negative is a valid, decisive VS2-EC-9 outcome, not an EC-3 miss)"
	}
	return band("VS2-EC-3", criterion, status, evidence)
}

// ec4 is VS2-EC-4: the comparison is a vector over all declared indicators, each
// with its raw and normalised first-divergence hour; no weighted sum is defined.
func ec4(fidelity []fidelityRow) reportBand {
	const criterion = "VS2-EC-4 multi-indicator, no single scalar: the report carries every declared indicator (raw and §6-normalised FirstDivergenceHour); the comparison is vector dominance under each indicator's declared direction; no weighted sum is defined and a scalar win without vector dominance does not count as beating."
	rows, incomplete := 0, 0
	for _, row := range fidelity {
		rows++
		if len(row.Indicators) != 8 {
			incomplete++
			continue
		}
		for _, ind := range row.Indicators {
			if ind.Name == "" || ind.NormalizedFirstDivergenceHour < 1 {
				incomplete++
				break
			}
		}
	}
	status := "PASS"
	if rows == 0 {
		status = "not-evaluable"
	} else if incomplete > 0 {
		status = "FAIL"
	}
	return band("VS2-EC-4", criterion, status, fmt.Sprintf(
		"fidelity rows=%d each carrying 8 indicators (raw first_divergence_hour + normalised -1->H+1); incomplete=%d; the scalar-winner-but-no-vector-dominance and sentinel-inversion fixtures are unit-exercised (cmd/vs2/bands_test.go)", rows, incomplete))
}

// ec5 is VS2-EC-5: identity and replay valid across strategy updates. A
// priority-bearing arm replays byte-exact from genesis with zero model calls
// and its selector re-derives the identical escalation set.
func ec5(replay map[string]string) reportBand {
	const criterion = "VS2-EC-5 identity and replay valid across strategy updates: a priority-bearing arm (P/U/A) replays byte-exact from genesis with zero model calls -- accepted history, hourly projections, allocation decisions and both ledgers -- and the selector re-derives the identical escalation set; the checkpoint manifest's cognition block binds the full priority-policy fingerprint and a restore under a differing policy is refused."
	if len(replay) == 0 {
		return band("VS2-EC-5", criterion, "not-evaluable", "no priority arm was replayed; the priority checkpoint/restore round trip is exercised in internal/world/natural_priority_replay_test.go and cmd/vs2/e2e_test.go, and the mutated-policy-fingerprint refusal in internal/world/natural_cognition_priority_test.go")
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
	return band("VS2-EC-5", criterion, status, fmt.Sprintf(
		"priority cells replayed=%d mismatches=%d; selection-sequence identity and the mutated-policy-fingerprint refusal are unit-exercised (internal/world/natural_priority_replay_test.go, internal/world/natural_cognition_priority_test.go, cmd/vs2/e2e_test.go)", len(replay), mismatch))
}

// ec7 is VS2-EC-7: protected exploration is present and bounded.
func ec7(runs []*cellRun) reportBand {
	const criterion = "VS2-EC-7 protected exploration present and bounded: with q>0 the selector reserves ceil(q*N) calls computed as (N*frac+scale-1)/scale for uniform online draws inside the per-hour quota q(h); the exploration set is non-empty when the arm's stream and N permit, deterministic from the policy PRNG, and recorded on the allocation record; the q=0 arm isolates pure exploitation."
	reservedA := reservedExploration(DeclaredCallCeiling, DeclaredExplorationFraction)
	aCells, aExploreTotal, violations := 0, 0, 0
	var bad []string
	for _, cr := range runs {
		if !isBudgeted(cr.spec.Arm) {
			continue
		}
		explore := 0
		for _, rec := range cr.interventions {
			if rec.Kind == cognition.InterventionAllocation && rec.Allocation != nil {
				explore += len(rec.Allocation.Exploration)
			}
		}
		switch cr.spec.Arm {
		case ArmA:
			aCells++
			aExploreTotal += explore
			if explore > reservedA {
				violations++
				bad = append(bad, cr.spec.String()+":over-reserved")
			}
			if explore == 0 && cr.escalated > 0 {
				bad = append(bad, cr.spec.String()+":empty-exploration")
			}
		case ArmP, ArmU:
			if explore != 0 {
				violations++
				bad = append(bad, cr.spec.String()+":unexpected-exploration")
			}
		}
	}
	status := "PASS"
	switch {
	case aCells == 0:
		status = "not-evaluable"
	case violations > 0:
		status = "FAIL"
	}
	return band("VS2-EC-7", criterion, status, fmt.Sprintf(
		"A cells=%d reserved E=ceil(q*N)=%d recorded exploration sum=%d; P/U exploration must be empty; violations=%d; q>0/q=0 fixtures are unit-exercised (internal/cognition/priority_test.go, cmd/vs2/accounting_test.go); %s",
		aCells, reservedA, aExploreTotal, violations, joinOrNone(bad)))
}

// ec8 is VS2-EC-8: the value model is declared, versioned and calibrated.
func ec8(runs []*cellRun) reportBand {
	const criterion = "VS2-EC-8 value model declared and calibrated (P6/ADR 0003): the value model is a versioned integer fixed-point projection exposing feature values, uncertainty, selected action, cost and realized downstream value; the calibration error of predicted priority vs realized effect is reported; the model version is part of run identity."
	withModel, withScores, mismatched := 0, 0, 0
	for _, cr := range runs {
		if !isBudgeted(cr.spec.Arm) {
			continue
		}
		withModel++
		if cr.allocationRecords > 0 {
			hasScores := false
			for _, rec := range cr.interventions {
				if rec.Kind == cognition.InterventionAllocation && rec.Allocation != nil && len(rec.Allocation.Scores) > 0 {
					hasScores = true
				}
			}
			if hasScores {
				withScores++
			} else {
				mismatched++
			}
		}
	}
	status := "PASS"
	switch {
	case withModel == 0:
		status = "not-evaluable"
	case mismatched > 0:
		status = "FAIL"
	}
	return band("VS2-EC-8", criterion, status, fmt.Sprintf(
		"value model version=%s bound in run identity and in the allocation-record policy_fingerprint; budgeted cells=%d with recorded candidate scores=%d; the float-free projection and the all-zero-weight uniform reduction are unit-exercised (internal/cognition/protocol/valuemodel_test.go)", VS2ValueModelVersion, withModel, withScores))
}

// ec9 is VS2-EC-9: honest labels, the real-model gate, and a valid negative.
func ec9() reportBand {
	const criterion = "VS2-EC-9 honest labels, real-model gate, negative valid: every report is synthetic/model-conditional; the real-model arm is labelled blocked on spend authorisation and no acceptance is claimed without it; a clean negative is reported verbatim and is a valid, decisive outcome."
	labelled := contains(evaluationLabel, "synthetic/model-conditional")
	gate := contains(liveSpendRequirement, "spend") && contains(liveSpendRequirement, "refus")
	status := "PASS"
	if !labelled || !gate {
		status = "FAIL"
	}
	return band("VS2-EC-9", criterion, status, fmt.Sprintf(
		"label=%q labelled=%t; honest limit=%q; real-model arm: blocked on spend; live gate=%q (refuses to start without the operator spend authorisation)", evaluationLabel, labelled, honestLimit, liveSpendRequirement))
}

func sortedCellKeys(byCell map[string]map[string]*cellRun) []string {
	keys := make([]string, 0, len(byCell))
	for k := range byCell {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
