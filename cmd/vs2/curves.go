package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// The curve statistics of the ticket-35 study (.scratch/35-vs2-curve/spec.md,
// pre-declared before any Stage-1 call). The unit of analysis is the per-hour KPI
// curve of a condition (arm, applied effort) replicated R times in one cell.
// This file owns the declared band, stability, SNR, first-separation and
// Wasserstein-1 statistics plus the report text; it runs no cell and calls no
// model.
//
// The decision rule is pre-registered and is NOT tuned from these numbers: a
// condition is distinguishable from another iff some KPI has SNR >= 2 AND its
// paired difference exceeds the replicate band for at least 24 consecutive
// hours, in both cells. Nothing here calls a condition better.

// curveScalarKPIs are the declared scalar (non-histogram) hourly KPI series, in
// the order the report prints them.
var curveScalarKPIs = []string{
	"population", "births_cum", "deaths_starve_cum", "deaths_age_cum", "wages_cum", "max_capital",
}

// curveHistogramKPIs are the declared histogram KPI families; each bucket is one
// KPI series, and the family as a whole carries the Wasserstein-1 over buckets.
var curveHistogramKPIs = []string{"capital_hist", "granary_hist"}

// curveSustainedHours is the declared consecutive-hour requirement of the
// decision rule.
const curveSustainedHours = 24

// curveSNRThreshold is the declared SNR threshold of the decision rule.
const curveSNRThreshold = 2.0

// curveCell is one replicate's KPI curves plus its per-cell accounting.
type curveCell struct {
	Condition string // "<arm>@<effort>"
	Arm       string
	Effort    string
	F         int
	Seed      uint64
	Replicate int

	KPIs   map[string][]float64   // scalar series by KPI name (histogram buckets included)
	Hist   map[string][][]float64 // histogram family -> rows of bucket counts per hour
	Hours  int
	Trunc  int // hours dropped from a longer series so all replicates align
	Calls  uint64
	Prompt uint64
	Output uint64

	DecisionErrors    int
	Escalated         int
	DecisionErrorRate float64
}

// cellLabel is the cell identity a pair is paired on: the world seed, not the
// replicate.
func (c curveCell) cellLabel() string { return fmt.Sprintf("f%d-s%d", c.F, c.Seed) }

// curveBand is one KPI's declared band over replicates: mean, 95% CI, full
// min/max envelope, and the across-replicate coefficient of variation.
type curveBand struct {
	Hours       int
	Mean        []float64
	CILo        []float64
	CIHi        []float64
	EnvLo       []float64
	EnvHi       []float64
	CV          []float64 // NaN where the hour's mean is zero (undefined, counted)
	MeanCV      float64
	WorstCV     float64
	WorstHour   int
	CVUndefined int
	Replicates  int
}

// curveComparison is one declared between-condition comparison of one KPI in
// one cell.
type curveComparison struct {
	Cell string
	KPI  string
	// RMSE is the RMSE over hours of the paired mean difference, Noise the RMSE
	// over hours of the replicate noise; SNR is their ratio.
	RMSE  float64
	Noise float64
	SNR   float64
	// Note names a degenerate denominator: a one-sided noise floor (one
	// condition deterministic), both deterministic, or a single replicate.
	Note string
	// FirstSeparation is the first hour whose paired difference exceeds the
	// replicate band (-1 when no hour separates); RunStart/RunLength describe the
	// longest consecutive separating run.
	FirstSeparation int
	RunStart        int
	RunLength       int
	Wasserstein     float64
}

// curvePair is the declared decision outcome for one pair of conditions.
type curvePair struct {
	A, B string
	// Comparisons holds every KPI comparison, per cell, in print order.
	Comparisons []curveComparison
	// Distinguishable is the declared rule's outcome: some KPI with SNR >= 2 and
	// a sustained separation in every cell.
	Distinguishable bool
	DrivingKPI      string
}

// curveStage groups the comparisons of one declared stage's condition set.
type curveStage struct {
	Label      string
	Conditions []string
	Pairs      []curvePair
}

// curveReport is the whole curve artefact.
type curveReport struct {
	Sources          []string
	Cells            int
	Replicates       int
	Horizon          int
	Conditions       []string
	CellsByCondition map[string][]curveCell
	Bands            map[string]map[string]map[string]curveBand // condition -> cell -> KPI -> band
	Stages           []curveStage
	Truncations      []string
}

// curveKPINames is the declared print order of every KPI series of one cell:
// the scalars, then each histogram bucket in bucket order.
func curveKPINames(c curveCell) []string {
	names := append([]string(nil), curveScalarKPIs...)
	for _, family := range curveHistogramKPIs {
		for b := 0; b < c.histBuckets(family); b++ {
			names = append(names, fmt.Sprintf("%s[%d]", family, b))
		}
	}
	return names
}

// histBuckets is the declared bucket count of a histogram family in this cell.
func (c curveCell) histBuckets(family string) int {
	rows := c.Hist[family]
	if len(rows) == 0 {
		return 0
	}
	return len(rows[0])
}

// curveCellsOf extracts the replicate curves of one evaluation cell row and
// truncates every series to the shortest common horizon, so hours pair exactly.
func curveCellsOf(ev evaluation) ([]curveCell, []string) {
	out := make([]curveCell, 0, len(ev.Cells))
	var truncations []string
	for _, rep := range ev.Cells {
		if !rep.Ran || len(rep.Hours) == 0 {
			continue
		}
		c := curveCell{
			Condition: conditionOf(rep), Arm: rep.Arm, Effort: rep.Effort,
			F: rep.F, Seed: rep.Seed, Replicate: rep.Replicate,
			KPIs: map[string][]float64{}, Hist: map[string][][]float64{},
			Calls: rep.RealizedCalls, Prompt: rep.PromptTokens, Output: rep.CompletionTokens,
			DecisionErrors: rep.DecisionErrors, Escalated: rep.Escalated,
		}
		if rep.Escalated > 0 {
			c.DecisionErrorRate = float64(rep.DecisionErrors) / float64(rep.Escalated)
		}
		c.KPIs["population"] = intsToFloats(rep.AliveByHour)
		c.KPIs["births_cum"] = int64sToFloats(rep.BirthsCum)
		c.KPIs["deaths_starve_cum"] = int64sToFloats(rep.DeathsStarveCum)
		c.KPIs["deaths_age_cum"] = int64sToFloats(rep.DeathsAgeCum)
		c.KPIs["wages_cum"] = int64sToFloats(rep.WagesCum)
		c.KPIs["max_capital"] = int64sToFloats(rep.MaxCapital)
		c.Hist["capital_hist"] = int64RowsToFloats(rep.CapitalHist)
		c.Hist["granary_hist"] = int64RowsToFloats(rep.GranaryHist)
		for family, rows := range c.Hist {
			for b := 0; b < len(rows[0]); b++ {
				series := make([]float64, len(rows))
				for h, row := range rows {
					series[h] = row[b]
				}
				c.KPIs[fmt.Sprintf("%s[%d]", family, b)] = series
			}
		}
		c.Hours = len(rep.Hours)
		out = append(out, c)
	}
	// Align every series to the shortest horizon across the loaded cells: a
	// shorter cell is a recorded short projection, never silently extrapolated.
	shortest := -1
	for _, c := range out {
		if shortest < 0 || c.Hours < shortest {
			shortest = c.Hours
		}
	}
	if shortest < 0 {
		return nil, truncations
	}
	for i := range out {
		if out[i].Hours > shortest {
			truncations = append(truncations, fmt.Sprintf("%s: %d hours truncated to %d for pairing",
				out[i].Condition+" "+out[i].cellLabel()+" r"+fmt.Sprint(out[i].Replicate), out[i].Hours, shortest))
			out[i].Trunc = out[i].Hours - shortest
			out[i].Hours = shortest
			for name, series := range out[i].KPIs {
				out[i].KPIs[name] = series[:shortest]
			}
			for family, rows := range out[i].Hist {
				out[i].Hist[family] = rows[:shortest]
			}
		}
	}
	return out, truncations
}

// conditionOf is the declared condition label of a cell row: the arm plus the
// applied effort, so two efforts of one arm are two conditions.
func conditionOf(rep cellReport) string {
	effort := rep.Effort
	if effort == "" {
		effort = "model-default"
	}
	return rep.Arm + "@" + effort
}

// bandOf computes the declared band and stability of one KPI over replicates.
// series[r][h] is replicate r's value at hour h; every series has one length.
func bandOf(series [][]float64) curveBand {
	h := 0
	if len(series) > 0 {
		h = len(series[0])
	}
	band := curveBand{
		Hours: h, Replicates: len(series),
		Mean: make([]float64, h), CILo: make([]float64, h), CIHi: make([]float64, h),
		EnvLo: make([]float64, h), EnvHi: make([]float64, h), CV: make([]float64, h),
		WorstCV: math.Inf(-1),
	}
	if len(series) == 0 {
		band.WorstCV = 0
		return band
	}
	t := tCritical95(len(series))
	cvSum, cvN := 0.0, 0
	for i := 0; i < h; i++ {
		values := make([]float64, len(series))
		for r := range series {
			values[r] = series[r][i]
		}
		mean, sd := meanSD(values)
		envLo, envHi := values[0], values[0]
		for _, v := range values {
			envLo = math.Min(envLo, v)
			envHi = math.Max(envHi, v)
		}
		half := 0.0
		if len(values) > 1 {
			half = t * sd / math.Sqrt(float64(len(values)))
		}
		band.Mean[i], band.EnvLo[i], band.EnvHi[i] = mean, envLo, envHi
		band.CILo[i], band.CIHi[i] = mean-half, mean+half
		if mean != 0 {
			cv := sd / math.Abs(mean)
			band.CV[i] = cv
			cvSum += cv
			cvN++
			if cv > band.WorstCV {
				band.WorstCV, band.WorstHour = cv, i
			}
		} else {
			band.CV[i] = math.NaN()
			band.CVUndefined++
		}
	}
	if cvN > 0 {
		band.MeanCV = cvSum / float64(cvN)
	} else {
		band.WorstCV = 0
	}
	return band
}

// compareKPIs computes the declared paired comparison of one KPI between two
// conditions inside one cell. a and b are the replicate series (replicates 3 by
// declaration). The replicate band of the difference is the wider of the two
// single-condition envelope half-widths at each hour; a separation is an hour
// whose |d| exceeds it.
func compareKPIs(cell, kpi string, a, b [][]float64) curveComparison {
	cmp := curveComparison{Cell: cell, KPI: kpi, FirstSeparation: -1, RunStart: -1}
	h := 0
	if len(a) > 0 {
		h = len(a[0])
	}
	if h == 0 || len(b) == 0 {
		cmp.Note = "no overlapping hours"
		return cmp
	}
	var sqDiff, sqNoise float64
	sep := make([]bool, h)
	aDiffSum, bDiffSum := 0.0, 0.0
	for i := 0; i < h; i++ {
		av := valuesAt(a, i)
		bv := valuesAt(b, i)
		am, asd := meanSD(av)
		bm, bsd := meanSD(bv)
		aDiffSum += (asd * asd)
		bDiffSum += (bsd * bsd)
		d := am - bm
		sqDiff += d * d
		// Pooled within-condition variance at this hour.
		dof := float64((len(av) - 1) + (len(bv) - 1))
		noise := 0.0
		if dof > 0 {
			inner := 0.0
			for _, v := range av {
				inner += (v - am) * (v - am)
			}
			for _, v := range bv {
				inner += (v - bm) * (v - bm)
			}
			noise = math.Sqrt(inner / dof)
		}
		sqNoise += noise * noise
		aEnvLo, aEnvHi := minMax(av)
		bEnvLo, bEnvHi := minMax(bv)
		bandWidth := math.Max(math.Max(aEnvHi-am, am-aEnvLo), math.Max(bEnvHi-bm, bm-bEnvLo))
		sep[i] = math.Abs(d) > bandWidth
	}
	cmp.RMSE = math.Sqrt(sqDiff / float64(h))
	cmp.Noise = math.Sqrt(sqNoise / float64(h))
	cmp.Wasserstein = math.NaN() // filled by the caller for the right KPI shape
	oneSidedA := aDiffSum == 0 && bDiffSum > 0
	oneSidedB := bDiffSum == 0 && aDiffSum > 0
	// The frozen criterion's degenerate-denominator rule, applied literally:
	// when one condition is deterministic the denominator is the other side's
	// replicate variance and the comparison is flagged as a one-sided noise floor;
	// when BOTH are deterministic the SNR is undefined and reported as
	// `n/a (both deterministic)` with the raw RMSE of the difference, never as an
	// infinite ratio (an undefined denominator is not an infinitely strong effect,
	// and an SNR the rule cannot express must not fire the rule).
	switch {
	case cmp.Noise > 0:
		cmp.SNR = cmp.RMSE / cmp.Noise
		if oneSidedA || oneSidedB {
			cmp.Note = "one-sided noise floor (one condition deterministic)"
		}
	default:
		cmp.SNR = math.NaN()
		cmp.Note = "n/a (both deterministic)"
	}
	if len(a) < 2 || len(b) < 2 {
		cmp.Note = "single replicate: no noise estimate"
	}
	// First separation and the longest separating run.
	runStart, runLen := -1, 0
	for i := 0; i < h; i++ {
		if !sep[i] {
			runStart = -1
			continue
		}
		if cmp.FirstSeparation < 0 {
			cmp.FirstSeparation = i
		}
		if runStart < 0 {
			runStart = i
		}
		if length := i - runStart + 1; length > runLen {
			runLen, cmp.RunStart = length, runStart
		}
	}
	cmp.RunLength = runLen
	return cmp
}

// wasserstein1 is the 1-D Wasserstein-1 distance between two empirical samples:
// the integral of |F_a - F_b| over the sorted union of the samples.
func wasserstein1(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return math.NaN()
	}
	as := append([]float64(nil), a...)
	bs := append([]float64(nil), b...)
	sort.Float64s(as)
	sort.Float64s(bs)
	na, nb := float64(len(as)), float64(len(bs))
	i, j := 0, 0
	prev := math.Min(as[0], bs[0])
	total := 0.0
	for i < len(as) || j < len(bs) {
		var next float64
		switch {
		case i >= len(as):
			next = bs[j]
		case j >= len(bs):
			next = as[i]
		default:
			next = math.Min(as[i], bs[j])
		}
		total += math.Abs(float64(i)/na-float64(j)/nb) * (next - prev)
		for i < len(as) && as[i] == next {
			i++
		}
		for j < len(bs) && bs[j] == next {
			j++
		}
		prev = next
	}
	return total
}

// wassersteinHistograms is the declared histogram distance: each hour's bucket
// vector is normalised to a probability distribution over bucket indices, and
// the 1-D Wasserstein-1 in bucket units is averaged over hours.
func wassersteinHistograms(a, b [][]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return math.NaN()
	}
	h := len(a)
	if len(b) < h {
		h = len(b)
	}
	total := 0.0
	for i := 0; i < h; i++ {
		total += wasserstein1(normalizeRows(a[i]), normalizeRows(b[i]))
	}
	return total / float64(h)
}

// normalizeRows turns bucket counts into a distribution over bucket indices: the
// sample is the bucket index repeated count times.
func normalizeRows(row []float64) []float64 {
	var out []float64
	for bucket, count := range row {
		if count <= 0 {
			continue
		}
		n := int(math.Round(count))
		for k := 0; k < n; k++ {
			out = append(out, float64(bucket))
		}
	}
	if len(out) == 0 {
		// An empty histogram is a declared empty state; every unit sits in
		// bucket 0 only when the population is zero, which the caller reports.
		return []float64{0}
	}
	return out
}

// curveReportFromFiles loads evaluation JSONs and computes the whole curve
// report. Conditions are grouped by (arm, applied effort); replicates are the
// rows of one condition in one cell.
func curveReportFromFiles(paths []string) (curveReport, error) {
	report := curveReport{CellsByCondition: map[string][]curveCell{}, Bands: map[string]map[string]map[string]curveBand{}}
	var stageOrder []string
	stageConditions := map[string][]string{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return curveReport{}, err
		}
		var ev evaluation
		if err := json.Unmarshal(raw, &ev); err != nil {
			return curveReport{}, fmt.Errorf("%s: %w", path, err)
		}
		cells, truncations := curveCellsOf(ev)
		report.Truncations = append(report.Truncations, truncations...)
		report.Sources = append(report.Sources, path)
		stage := stageLabelOf(path)
		for _, c := range cells {
			if _, ok := report.CellsByCondition[c.Condition]; !ok {
				report.Conditions = append(report.Conditions, c.Condition)
			}
			report.CellsByCondition[c.Condition] = append(report.CellsByCondition[c.Condition], c)
			if !containsString(stageConditions[stage], c.Condition) {
				stageConditions[stage] = append(stageConditions[stage], c.Condition)
			}
			if !containsString(stageOrder, stage) {
				stageOrder = append(stageOrder, stage)
			}
		}
	}
	sort.Strings(report.Conditions)
	sort.Strings(stageOrder)
	allCells := map[string]struct{}{}
	for _, condition := range report.Conditions {
		report.Bands[condition] = map[string]map[string]curveBand{}
		byCell := groupByCell(report.CellsByCondition[condition])
		for cell, cellsInCell := range byCell {
			allCells[cell] = struct{}{}
			report.Bands[condition][cell] = map[string]curveBand{}
			for _, kpi := range curveKPINames(cellsInCell[0]) {
				series := make([][]float64, 0, len(cellsInCell))
				for _, c := range cellsInCell {
					series = append(series, c.KPIs[kpi])
				}
				report.Bands[condition][cell][kpi] = bandOf(series)
			}
			if len(cellsInCell) > report.Replicates {
				report.Replicates = len(cellsInCell)
			}
			if hours := len(cellsInCell[0].KPIs["population"]); hours > report.Horizon {
				report.Horizon = hours
			}
		}
	}
	report.Cells = len(allCells)
	for _, stage := range stageOrder {
		conditions := stageConditions[stage]
		sort.Strings(conditions)
		report.Stages = append(report.Stages, curveStage{
			Label: stage, Conditions: conditions,
			Pairs: curvePairsOf(report, conditions),
		})
	}
	return report, nil
}

// stageLabelOf names the study stage a source file belongs to, from its path.
func stageLabelOf(path string) string {
	lower := strings.ToLower(path)
	for _, stage := range []string{"stage1", "stage2", "stage3", "stage-1", "stage-2", "stage-3"} {
		if strings.Contains(lower, stage) {
			return strings.ReplaceAll(stage, "-", "")
		}
	}
	return "stage1"
}

// curvePairsOf compares every unordered pair of conditions in one stage, per
// cell, over every KPI.
func curvePairsOf(report curveReport, conditions []string) []curvePair {
	var pairs []curvePair
	for i := 0; i < len(conditions); i++ {
		for j := i + 1; j < len(conditions); j++ {
			pair := curvePair{A: conditions[i], B: conditions[j]}
			cells := map[string]struct{}{}
			for _, c := range report.CellsByCondition[pair.A] {
				cells[c.cellLabel()] = struct{}{}
			}
			cellNames := make([]string, 0, len(cells))
			for name := range cells {
				cellNames = append(cellNames, name)
			}
			sort.Strings(cellNames)
			perCellSustained := map[string]map[string]bool{}
			for _, cell := range cellNames {
				aCells := sortedReplicates(report.CellsByCondition[pair.A], cell)
				bCells := sortedReplicates(report.CellsByCondition[pair.B], cell)
				if len(aCells) == 0 || len(bCells) == 0 {
					continue
				}
				sustained := map[string]bool{}
				for _, kpi := range curveKPINames(aCells[0]) {
					cmp := compareKPIs(cell, kpi, seriesOf(aCells, kpi), seriesOf(bCells, kpi))
					if family, bucket, ok := histogramKPI(kpi); ok {
						cmp.Wasserstein = wassersteinHistograms(histRows(aCells, family, bucket), histRows(bCells, family, bucket))
					} else {
						cmp.Wasserstein = wasserstein1(poolSamples(aCells, kpi), poolSamples(bCells, kpi))
					}
					if cmp.SNR >= curveSNRThreshold && cmp.RunLength >= curveSustainedHours {
						sustained[kpi] = true
					}
					pair.Comparisons = append(pair.Comparisons, cmp)
				}
				perCellSustained[cell] = sustained
			}
			// The declared rule: one KPI holds it in every cell.
			if len(cellNames) > 0 && len(perCellSustained) == len(cellNames) {
				first := sortedReplicates(report.CellsByCondition[pair.A], cellNames[0])
				if len(first) > 0 {
					for _, kpi := range curveKPINames(first[0]) {
						inAll := true
						for _, sustained := range perCellSustained {
							if !sustained[kpi] {
								inAll = false
								break
							}
						}
						if inAll {
							pair.Distinguishable = true
							pair.DrivingKPI = kpi
							break
						}
					}
				}
			}
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

// histogramKPI splits a "<family>[<bucket>]" KPI name into its parts.
func histogramKPI(kpi string) (string, int, bool) {
	open := strings.Index(kpi, "[")
	if open < 0 || !strings.HasSuffix(kpi, "]") {
		return "", 0, false
	}
	family := kpi[:open]
	bucket := 0
	if _, err := fmt.Sscanf(kpi[open+1:len(kpi)-1], "%d", &bucket); err != nil {
		return "", 0, false
	}
	return family, bucket, true
}

// seriesOf is the replicate-major view of one KPI for a set of replicates.
func seriesOf(cells []curveCell, kpi string) [][]float64 {
	out := make([][]float64, 0, len(cells))
	for _, c := range cells {
		out = append(out, c.KPIs[kpi])
	}
	return out
}

// histRows is the replicate-major bucket-index view of a histogram family.
func histRows(cells []curveCell, family string, bucket int) [][]float64 {
	out := make([][]float64, 0, len(cells))
	for _, c := range cells {
		rows := c.Hist[family]
		series := make([]float64, len(rows))
		for h, row := range rows {
			if bucket < len(row) {
				series[h] = row[bucket]
			}
		}
		out = append(out, series)
	}
	return out
}

// poolSamples is every (hour, replicate) value of one KPI, the empirical sample
// the Wasserstein-1 distance is measured between.
func poolSamples(cells []curveCell, kpi string) []float64 {
	var out []float64
	for _, c := range cells {
		out = append(out, c.KPIs[kpi]...)
	}
	return out
}

// sortedReplicates returns the replicates of one condition in one cell, ordered
// by replicate index.
func sortedReplicates(cells []curveCell, cell string) []curveCell {
	var out []curveCell
	for _, c := range cells {
		if c.cellLabel() == cell {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Replicate < out[j].Replicate })
	return out
}

func intsToFloats(in []int) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}

func int64sToFloats(in []int64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}

func int64RowsToFloats(in [][]int64) [][]float64 {
	out := make([][]float64, len(in))
	for i, row := range in {
		out[i] = int64sToFloats(row)
	}
	return out
}

func valuesAt(series [][]float64, hour int) []float64 {
	out := make([]float64, 0, len(series))
	for _, s := range series {
		if hour < len(s) {
			out = append(out, s[hour])
		}
	}
	return out
}

func meanSD(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	if len(values) == 1 {
		return mean, 0
	}
	ss := 0.0
	for _, v := range values {
		ss += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(ss / float64(len(values)-1))
}

func minMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	return lo, hi
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// tCritical95 is the two-sided 95% Student-t critical value for n-1 degrees of
// freedom, tabulated through eight replicates and the normal value beyond.
func tCritical95(n int) float64 {
	switch n {
	case 2:
		return 12.706
	case 3:
		return 4.303
	case 4:
		return 3.182
	case 5:
		return 2.776
	case 6:
		return 2.571
	case 7:
		return 2.447
	case 8:
		return 2.365
	}
	return 1.96
}

// renderCurveReport is the durable text curve report: the declared rule, the
// noise floor per condition and cell, every stage's paired comparisons, and the
// plain statement of what the data does and does not establish.
func renderCurveReport(report curveReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "VS2 curve report (ticket 35; criterion pre-declared before data)\n")
	fmt.Fprintf(&b, "decision rule: distinguishable iff some KPI has SNR >= %.1f AND its paired difference exceeds the\n", curveSNRThreshold)
	fmt.Fprintf(&b, "  replicate band for >= %d consecutive hours in every cell; no condition is called better on a scalar.\n", curveSustainedHours)
	fmt.Fprintf(&b, "conditions: %s\n", strings.Join(report.Conditions, ", "))
	fmt.Fprintf(&b, "cells: %d replicates: %d horizon: %d\n", report.Cells, report.Replicates, report.Horizon)
	for _, src := range report.Sources {
		fmt.Fprintf(&b, "source: %s\n", src)
	}
	for _, trunc := range report.Truncations {
		fmt.Fprintf(&b, "truncated: %s\n", trunc)
	}

	fmt.Fprintf(&b, "\nNOISE FLOOR (band over replicates: mean [95%% CI] envelope, and across-replicate CV)\n")
	for _, condition := range report.Conditions {
		var cells []string
		for cell := range report.Bands[condition] {
			cells = append(cells, cell)
		}
		sort.Strings(cells)
		for _, cell := range cells {
			fmt.Fprintf(&b, "\n[%s] %s (columns are the band AT the final hour H; CV is the across-replicate coefficient of variation averaged over hours, with the worst hour)\n", condition, cell)
			fmt.Fprintf(&b, "%-20s %12s %12s %12s %16s %9s %9s\n", "kpi", "mean@H", "ci_lo@H", "ci_hi@H", "envelope@H", "meanCV", "worstCV")
			bands := report.Bands[condition][cell]
			for _, kpi := range sortedKPIs(bands) {
				band := bands[kpi]
				if band.Hours == 0 {
					continue
				}
				last := band.Hours - 1
				fmt.Fprintf(&b, "%-20s %12s %12s %12s %16s %9s %9s\n", kpi,
					fmtFloat(band.Mean[last]), fmtFloat(band.CILo[last]), fmtFloat(band.CIHi[last]),
					fmt.Sprintf("%s..%s", fmtFloat(band.EnvLo[last]), fmtFloat(band.EnvHi[last])),
					fmtFloat(band.MeanCV), fmtFloat(band.WorstCV))
				if band.CVUndefined > 0 && band.CVUndefined != band.Hours {
					fmt.Fprintf(&b, "%-20s   (%d hours excluded from CV: mean 0)\n", "", band.CVUndefined)
				}
			}
		}
	}

	fmt.Fprintf(&b, "\nPER-CONDITION ACCOUNTING (call-count budget axis; token totals for equal-token re-analysis)\n")
	fmt.Fprintf(&b, "%-14s %-10s %8s %12s %12s %8s %8s\n", "condition", "cell", "reps", "calls", "in_tokens", "out_tok", "err_rate")
	for _, condition := range report.Conditions {
		byCell := groupByCell(report.CellsByCondition[condition])
		var cells []string
		for cell := range byCell {
			cells = append(cells, cell)
		}
		sort.Strings(cells)
		for _, cell := range cells {
			cellsInCell := byCell[cell]
			var calls, prompt, output uint64
			errs, escalated := 0, 0
			for _, c := range cellsInCell {
				calls += c.Calls
				prompt += c.Prompt
				output += c.Output
				errs += c.DecisionErrors
				escalated += c.Escalated
			}
			rate := 0.0
			if escalated > 0 {
				rate = float64(errs) / float64(escalated)
			}
			fmt.Fprintf(&b, "%-14s %-10s %8d %12d %12d %8d %8s\n", condition, cell, len(cellsInCell), calls, prompt, output, fmtFloat(rate))
		}
	}

	for _, stage := range report.Stages {
		fmt.Fprintf(&b, "\nSTAGE %s — conditions %s\n", strings.ToUpper(stage.Label), strings.Join(stage.Conditions, ", "))
		for _, pair := range stage.Pairs {
			fmt.Fprintf(&b, "\npair %s vs %s: %s\n", pair.A, pair.B, distinguishability(pair))
			fmt.Fprintf(&b, "  %-10s %-20s %10s %10s %8s %8s %8s %8s %10s %s\n",
				"cell", "kpi", "rmse_d", "noise", "snr", "firstsep", "run_len", "run_at", "wasserstein", "note")
			for _, cmp := range pair.Comparisons {
				fmt.Fprintf(&b, "  %-10s %-20s %10s %10s %8s %8d %8d %8d %10s %s\n",
					cmp.Cell, cmp.KPI, fmtFloat(cmp.RMSE), fmtFloat(cmp.Noise), fmtSNR(cmp.SNR),
					cmp.FirstSeparation, cmp.RunLength, cmp.RunStart, fmtFloat(cmp.Wasserstein), cmp.Note)
			}
		}
	}

	fmt.Fprintf(&b, "\nWHAT THE DATA DOES AND DOES NOT ESTABLISH\n")
	fmt.Fprintf(&b, "%s\n", curveConclusion(report))
	return b.String()
}

// distinguishability renders the declared decision outcome of one pair.
func distinguishability(pair curvePair) string {
	if pair.Distinguishable {
		return fmt.Sprintf("DISTINGUISHABLE (KPI %s holds SNR >= %.1f with a >= %d-hour separation in every cell)",
			pair.DrivingKPI, curveSNRThreshold, curveSustainedHours)
	}
	return "NOT distinguishable at this noise floor (no KPI holds the declared SNR and sustained separation in every cell)"
}

// curveConclusion states the measurable outcome without a quality verdict.
func curveConclusion(report curveReport) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Measured: %d conditions over %d cell(s), %d replicates each, %d hours.",
		len(report.Conditions), report.Cells, report.Replicates, report.Horizon))
	distinguishable := 0
	var named []string
	for _, stage := range report.Stages {
		for _, pair := range stage.Pairs {
			if pair.Distinguishable {
				distinguishable++
				named = append(named, fmt.Sprintf("%s vs %s via %s", pair.A, pair.B, pair.DrivingKPI))
			}
		}
	}
	if distinguishable == 0 {
		lines = append(lines, "Every compared pair is not distinguishable at this noise floor: no KPI held SNR >= 2 with a >= 24-hour separation in every cell.")
	} else {
		lines = append(lines, fmt.Sprintf("%d pair(s) are distinguishable under the declared rule: %s.", distinguishable, strings.Join(named, "; ")))
	}
	lines = append(lines, "Not established: which condition is better. The study measures the noise floor, the effort ladder's reach and the between-condition separation; the frozen EC-3 instrument and the milestone review own any quality verdict, and the ticket-34 verdict (central claim NOT accepted under EC-3 as written) stands unchanged.")
	return strings.Join(lines, "\n")
}

// groupByCell indexes one condition's replicates by cell.
func groupByCell(cells []curveCell) map[string][]curveCell {
	out := map[string][]curveCell{}
	for _, c := range cells {
		out[c.cellLabel()] = append(out[c.cellLabel()], c)
	}
	for cell := range out {
		sort.Slice(out[cell], func(i, j int) bool { return out[cell][i].Replicate < out[cell][j].Replicate })
	}
	return out
}

// sortedKPIs orders the KPI names of a cell's bands for stable printing.
func sortedKPIs(bands map[string]curveBand) []string {
	names := make([]string, 0, len(bands))
	for name := range bands {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return kpiOrder(names[i]) < kpiOrder(names[j]) })
	return names
}

// kpiOrder is the declared print order: scalar KPIs first in declaration order,
// then histogram buckets by family and bucket index.
func kpiOrder(name string) int {
	for i, kpi := range curveScalarKPIs {
		if name == kpi {
			return i
		}
	}
	if family, bucket, ok := histogramKPI(name); ok {
		base := len(curveScalarKPIs)
		for i, f := range curveHistogramKPIs {
			if f == family {
				return base + i*100 + bucket
			}
		}
		return base + 1000 + bucket
	}
	return 10_000
}

// fmtSNR renders a comparison's SNR: a number when the denominator is defined,
// `n/a` when both conditions are deterministic (the frozen criterion's declared
// state, never an infinite ratio).
func fmtSNR(snr float64) string {
	if math.IsNaN(snr) {
		return "n/a"
	}
	return fmtFloat(snr)
}
