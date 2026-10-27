package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// The curve statistics are checked against hand-computed fixtures, so the
// declared criterion is a falsifiable instrument rather than a description:
// bandOf, compareKPIs and wasserstein1 each have a worked example, and the
// decision rule is exercised end-to-end on a controlled two-condition fixture.

func TestBandOfHandComputed(t *testing.T) {
	// One hour, three replicates 1,2,3: mean 2, sample SD 1, t(0.975,2)=4.303,
	// so the 95% CI half-width is 4.303/sqrt(3) = 2.48433...; the envelope is
	// [1,3] and the coefficient of variation is 1/2.
	band := bandOf([][]float64{{1}, {2}, {3}})
	if band.Replicates != 3 || band.Hours != 1 {
		t.Fatalf("shape: replicates=%d hours=%d", band.Replicates, band.Hours)
	}
	if band.Mean[0] != 2 {
		t.Fatalf("mean: got %v want 2", band.Mean[0])
	}
	half := 4.303 / math.Sqrt(3)
	if !closeEnough(band.CILo[0], 2-half) || !closeEnough(band.CIHi[0], 2+half) {
		t.Fatalf("95%% CI: got [%v,%v] want [%v,%v]", band.CILo[0], band.CIHi[0], 2-half, 2+half)
	}
	if band.EnvLo[0] != 1 || band.EnvHi[0] != 3 {
		t.Fatalf("envelope: got [%v,%v] want [1,3]", band.EnvLo[0], band.EnvHi[0])
	}
	if !closeEnough(band.CV[0], 0.5) || !closeEnough(band.MeanCV, 0.5) || !closeEnough(band.WorstCV, 0.5) {
		t.Fatalf("CV: got %v mean %v worst %v want 0.5", band.CV[0], band.MeanCV, band.WorstCV)
	}
}

func TestBandOfDeterministicAndZeroMean(t *testing.T) {
	// A deterministic series has a zero-width band, and a zero mean leaves the
	// CV undefined rather than dividing by zero.
	band := bandOf([][]float64{{0, 5}, {0, 5}, {0, 5}})
	if band.CILo[0] != band.Mean[0] || band.CIHi[0] != band.Mean[0] {
		t.Fatalf("deterministic CI is not degenerate: [%v,%v]", band.CILo[0], band.CIHi[0])
	}
	if band.CVUndefined != 1 || !math.IsNaN(band.CV[0]) {
		t.Fatalf("zero-mean hour: undefined=%d cv=%v", band.CVUndefined, band.CV[0])
	}
	if band.CV[1] != 0 || band.MeanCV != 0 {
		t.Fatalf("defined hour CV: %v mean %v want 0", band.CV[1], band.MeanCV)
	}
}

func TestCompareKPIsHandComputedSNR(t *testing.T) {
	// A has replicate values {10,11,12} (SD 1) at every hour, B {7,8,9} (SD 1).
	// d = 3 everywhere, so RMSE_h(d) = 3. The pooled within-condition variance per
	// hour is (2+2)/4 = 1, so the replicate noise is 1 and SNR = 3.
	a := [][]float64{{10, 10, 10}, {11, 11, 11}, {12, 12, 12}}
	b := [][]float64{{7, 7, 7}, {8, 8, 8}, {9, 9, 9}}
	cmp := compareKPIs("f10-s0", "population", a, b)
	if !closeEnough(cmp.RMSE, 3) {
		t.Fatalf("rmse: got %v want 3", cmp.RMSE)
	}
	if !closeEnough(cmp.Noise, 1) {
		t.Fatalf("noise: got %v want 1", cmp.Noise)
	}
	if !closeEnough(cmp.SNR, 3) {
		t.Fatalf("snr: got %v want 3", cmp.SNR)
	}
	// The replicate band of the difference is the wider envelope half-width, 1,
	// so every hour separates and the run is the whole horizon.
	if cmp.FirstSeparation != 0 || cmp.RunLength != 3 || cmp.RunStart != 0 {
		t.Fatalf("separation: first=%d run=%d at %d want 0/3/0", cmp.FirstSeparation, cmp.RunLength, cmp.RunStart)
	}
	if cmp.Note != "" {
		t.Fatalf("note: got %q want empty (two-sided noise)", cmp.Note)
	}
}

func TestCompareKPIsOneSidedNoiseFloor(t *testing.T) {
	// One condition deterministic (the A0 control has no cognition), the other
	// replicated: the denominator is the non-degenerate side and the comparison
	// says so instead of pretending a two-sided noise floor.
	a := [][]float64{{5, 5, 5}, {5, 5, 5}, {5, 5, 5}}
	b := [][]float64{{7, 7, 7}, {8, 8, 8}, {9, 9, 9}}
	cmp := compareKPIs("f10-s0", "population", a, b)
	if cmp.Note != "one-sided noise floor (one condition deterministic)" {
		t.Fatalf("note: got %q", cmp.Note)
	}
	// d = 3, pooled within-condition SD = sqrt((0 + 2)/4) = 0.70711.
	if !closeEnough(cmp.SNR, 3/math.Sqrt(0.5)) {
		t.Fatalf("snr: got %v want %v", cmp.SNR, 3/math.Sqrt(0.5))
	}
}

func TestCompareKPIsBothDeterministicReportsNotApplicable(t *testing.T) {
	// The frozen criterion: when both conditions are deterministic the SNR is
	// undefined and reported `n/a (both deterministic)` with the raw RMSE of the
	// difference. It is never an infinite ratio, because an undefined denominator
	// must not fire the decision rule.
	a := [][]float64{{4, 4}, {4, 4}, {4, 4}}
	cmp := compareKPIs("f10-s0", "population", a, a)
	if !math.IsNaN(cmp.SNR) || cmp.Note != "n/a (both deterministic)" {
		t.Fatalf("identical deterministic curves: snr=%v note=%q", cmp.SNR, cmp.Note)
	}
	if fmtSNR(cmp.SNR) != "n/a" || !closeEnough(cmp.RMSE, 0) {
		t.Fatalf("rendered snr=%q rmse=%v", fmtSNR(cmp.SNR), cmp.RMSE)
	}
	// Differing deterministic curves keep the raw RMSE and stay n/a, so a KPI the
	// rule cannot express never drives a verdict.
	b := [][]float64{{6, 6}, {6, 6}, {6, 6}}
	cmp = compareKPIs("f10-s0", "population", a, b)
	if !math.IsNaN(cmp.SNR) || cmp.Note != "n/a (both deterministic)" || !closeEnough(cmp.RMSE, 2) {
		t.Fatalf("differing deterministic curves: snr=%v note=%q rmse=%v", cmp.SNR, cmp.Note, cmp.RMSE)
	}
	if cmp.SNR >= curveSNRThreshold {
		t.Fatalf("an n/a SNR must not satisfy the threshold")
	}
}

func TestWasserstein1HandComputed(t *testing.T) {
	cases := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{0, 1, 2}, []float64{0, 1, 2}, 0},
		{"disjoint shift", []float64{0, 0}, []float64{3, 3}, 3},
		// F_a = 0 up to 0, 0.5 on [0,10), 1 after; F_b = 0 before 5, 1 after.
		// The integral of |F_a - F_b| is 0.5*5 + 0.5*5 = 5.
		{"mass split", []float64{0, 10}, []float64{5, 5}, 5},
		{"single point vs single point", []float64{7}, []float64{2}, 5},
	}
	for _, tc := range cases {
		if got := wasserstein1(tc.a, tc.b); !closeEnough(got, tc.want) {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestWassersteinHistogramsHandComputed(t *testing.T) {
	// All mass moves from bucket 0 to bucket 1: one bucket unit at distance 1.
	a := [][]float64{{3, 0, 0}}
	b := [][]float64{{0, 3, 0}}
	if got := wassersteinHistograms(a, b); !closeEnough(got, 1) {
		t.Fatalf("histogram W1: got %v want 1", got)
	}
	// Half the mass moving one bucket is 0.5 in expectation.
	c := [][]float64{{2, 2, 0}}
	if got := wassersteinHistograms(a, c); !closeEnough(got, 0.5) {
		t.Fatalf("histogram W1 partial: got %v want 0.5", got)
	}
}

// TestWassersteinHistogramsPerHourMeanRowsHandComputed pins the declared shape:
// each hour's row is the mean bucket count over the condition's replicates, the
// W1 is in bucket units at that hour, and the reported value is its mean over
// hours. Feeding replicate-major bucket series here would measure the hour axis
// instead and cannot produce these numbers.
func TestWassersteinHistogramsPerHourMeanRowsHandComputed(t *testing.T) {
	a := []curveCell{{Hist: map[string][][]float64{"capital_hist": {
		{0, 0, 2}, // two bucket units from b at hour 0
		{0, 1, 1}, // one bucket unit from b at hour 1
	}}}, {Hist: map[string][][]float64{"capital_hist": {
		{0, 0, 2},
		{0, 1, 1},
	}}}}
	b := []curveCell{{Hist: map[string][][]float64{"capital_hist": {
		{2, 0, 0},
		{1, 1, 0},
	}}}, {Hist: map[string][][]float64{"capital_hist": {
		{2, 0, 0},
		{1, 1, 0},
	}}}}
	// Hour 0: mass at index 2 vs index 0 -> W1 = 2.
	// Hour 1: {0, 1, 1} vs {1, 1, 0} -> W1 = 0.5 + 0.5 = 1.
	// Mean over hours: (2 + 1) / 2 = 1.5.
	if got := wassersteinHistograms(histHourRows(a, "capital_hist"), histHourRows(b, "capital_hist")); !closeEnough(got, 1.5) {
		t.Fatalf("histogram W1 by hour: got %v want 1.5", got)
	}
}

func TestWassersteinHistogramsWithinBucketRange(t *testing.T) {
	// Unit mass at opposite ends of a 7-bucket family: the distance is 6 bucket
	// units, the largest a W1 over bucket indices can be, never an hour index.
	const width = 7
	a := []curveCell{{Hist: map[string][][]float64{"granary_hist": {{1, 0, 0, 0, 0, 0, 0}}}}}
	b := []curveCell{{Hist: map[string][][]float64{"granary_hist": {{0, 0, 0, 0, 0, 0, 1}}}}}
	got := wassersteinHistograms(histHourRows(a, "granary_hist"), histHourRows(b, "granary_hist"))
	if !closeEnough(got, width-1) {
		t.Fatalf("histogram W1: got %v want %d", got, width-1)
	}
	if got > width-1 {
		t.Fatalf("histogram W1 %v exceeds the bucket range 0..%d", got, width-1)
	}
}

// curveFixtureEvaluation builds a two-condition, two-cell, three-replicate
// evaluation whose population curves are controlled: condition A is mu, B is
// mu+delta, and the replicate spread is `spread` at every hour.
func curveFixtureEvaluation(hours int, delta, spread float64, otherKPIs []float64) evaluation {
	var cells []cellReport
	for _, f := range []int{10, 12} {
		for rep := 1; rep <= 3; rep++ {
			for _, condition := range []string{"X@low", "Y@low"} {
				base := 10.0
				if condition == "Y@low" {
					base += delta
				}
				pop := make([]int, hours)
				for h := range pop {
					pop[h] = int(base + spread*float64(rep-2))
				}
				row := cellReport{
					Arm: "X", F: f, Seed: 0, Ran: true, Replicate: rep, Effort: "low",
					Hours: makeInts(hours), AliveByHour: pop,
					BirthsCum: makeInt64s(hours, otherKPIs[0]), DeathsStarveCum: makeInt64s(hours, otherKPIs[1]),
					DeathsAgeCum: makeInt64s(hours, otherKPIs[2]), WagesCum: makeInt64s(hours, otherKPIs[3]),
					MaxCapital:  makeInt64s(hours, otherKPIs[4]),
					CapitalHist: makeHist(hours, 9), GranaryHist: makeHist(hours, 6),
					RealizedCalls: 100, PromptTokens: 1000, CompletionTokens: 200,
					Escalated: 10, DecisionErrors: 1,
				}
				if condition == "Y@low" {
					row.Arm = "Y"
				}
				cells = append(cells, row)
			}
		}
	}
	return evaluation{Cells: cells}
}

func TestCurveReportDecisionRuleSustainedDifference(t *testing.T) {
	// 40 hours, replicates spread +-1, condition difference 4 in population and
	// nothing in the other KPIs: SNR is high and the separation runs the whole
	// horizon, so the declared rule fires for population only.
	dir := t.TempDir()
	path := filepath.Join(dir, "stage2", "evaluation.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	ev := curveFixtureEvaluation(40, 4, 1, []float64{0, 0, 0, 0, 0})
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := curveReportFromFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Stages) != 1 || len(report.Stages[0].Pairs) != 1 {
		t.Fatalf("stages=%d pairs=%d", len(report.Stages), len(report.Stages[0].Pairs))
	}
	pair := report.Stages[0].Pairs[0]
	if !pair.Distinguishable || pair.DrivingKPI != "population" {
		t.Fatalf("pair: distinguishable=%t kpi=%q want true/population", pair.Distinguishable, pair.DrivingKPI)
	}
	// The other KPIs are identical and deterministic: they never drive the rule.
	for _, cmp := range pair.Comparisons {
		if cmp.KPI == "population" {
			continue
		}
		if cmp.SNR >= curveSNRThreshold && cmp.RunLength >= curveSustainedHours {
			t.Fatalf("unexpected driving KPI %s: snr=%v run=%d", cmp.KPI, cmp.SNR, cmp.RunLength)
		}
	}
}

func TestCurveReportDecisionRuleShortHorizonNotSustained(t *testing.T) {
	// The same difference over 12 hours: SNR still passes, but the separation
	// cannot be sustained for 24 consecutive hours, so the declared rule does NOT
	// fire. This is the check that a big difference alone is not enough.
	dir := t.TempDir()
	path := filepath.Join(dir, "stage2", "evaluation.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	ev := curveFixtureEvaluation(12, 4, 1, []float64{0, 0, 0, 0, 0})
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := curveReportFromFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	pair := report.Stages[0].Pairs[0]
	if pair.Distinguishable {
		t.Fatalf("12-hour horizon must not satisfy the 24-hour sustained rule (kpi %q)", pair.DrivingKPI)
	}
	for _, cmp := range pair.Comparisons {
		if cmp.KPI == "population" && cmp.RunLength != 12 {
			t.Fatalf("population separation run: got %d want 12", cmp.RunLength)
		}
	}
}

func TestCurveReportRuleNeedsBothCells(t *testing.T) {
	// A difference that exists in one cell only must not satisfy the rule: the
	// criterion requires both cells.
	dir := t.TempDir()
	path := filepath.Join(dir, "stage2", "evaluation.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	ev := curveFixtureEvaluation(40, 0, 1, []float64{0, 0, 0, 0, 0})
	for i := range ev.Cells {
		if ev.Cells[i].Arm == "Y" && ev.Cells[i].F == 10 {
			for h := range ev.Cells[i].AliveByHour {
				ev.Cells[i].AliveByHour[h] += 4
			}
		}
	}
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := curveReportFromFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	pair := report.Stages[0].Pairs[0]
	if pair.Distinguishable {
		t.Fatalf("one-cell-only difference must not satisfy the declared rule (kpi %q)", pair.DrivingKPI)
	}
}

func TestCurveReportRendersConditionsAndNegative(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stage3", "evaluation.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	ev := curveFixtureEvaluation(30, 0, 1, []float64{0, 0, 0, 0, 0})
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := curveReportFromFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	text := renderCurveReport(report)
	if !containsString(report.Conditions, "X@low") || !containsString(report.Conditions, "Y@low") {
		t.Fatalf("conditions: %v", report.Conditions)
	}
	for _, want := range []string{"NOISE FLOOR", "PER-CONDITION ACCOUNTING", "NOT distinguishable", "Not established: which condition is better"} {
		if !containsSubstring(text, want) {
			t.Fatalf("report text missing %q", want)
		}
	}
}

func makeInts(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func makeInt64s(n int, v float64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(v)
	}
	return out
}

func makeHist(n, buckets int) [][]int64 {
	out := make([][]int64, n)
	for i := range out {
		out[i] = make([]int64, buckets)
	}
	return out
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) <= 1e-9*math.Max(1, math.Abs(want))
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
