package main

import (
	"testing"

	"agentworld/internal/experiment"
)

// The VS2 exit-criteria band tests (VS2-EC-3/VS2-EC-4): the declared
// no-divergence normalisation, the direction-aware vector dominance, the
// sentinel-inversion regression, and the scalar-winner-but-no-vector-dominance
// fixture.

var testIndicatorNames = []string{
	"population", "births", "deaths/starvation", "deaths/age",
	"capital-histogram", "granary-histogram", "exchange-volume", "decision-error-rate",
}

// mkIndicators builds an 8-indicator vector from per-indicator maxima, means,
// and raw first-divergence hours (-1 == never diverged).
func mkIndicators(maxes, means []float64, firsts []int, horizon int) []indicatorReport {
	out := make([]indicatorReport, 8)
	for i := 0; i < 8; i++ {
		out[i] = indicatorReport{
			FidelityIndicator: experiment.FidelityIndicator{
				Index: i + 1, Name: testIndicatorNames[i], Max: maxes[i], Mean: means[i], FirstDivergenceHour: firsts[i],
			},
			NormalizedFirstDivergenceHour: normalizeFirstDivergence(firsts[i], horizon),
		}
	}
	return out
}

func fidRow(arm string, f int, seed uint64, inds []indicatorReport) fidelityRow {
	return fidelityRow{Arm: arm, F: f, Seed: seed, Reference: ArmR, Indicators: inds}
}

func TestNormalizeFirstDivergence(t *testing.T) {
	if got := normalizeFirstDivergence(-1, 168); got != 169 {
		t.Fatalf("raw -1 normalised to %d, want H+1=169", got)
	}
	if got := normalizeFirstDivergence(5, 168); got != 5 {
		t.Fatalf("raw 5 normalised to %d, want 5", got)
	}
}

// TestSentinelInversionRegression pins the §6 no-divergence encoding: an
// identical (never-diverged) arm must not be judged worse than a diverged arm.
func TestSentinelInversionRegression(t *testing.T) {
	const h = 168
	zeros := make([]float64, 8)
	never := []int{-1, -1, -1, -1, -1, -1, -1, -1}
	diverged := []int{-1, -1, -1, -1, -1, -1, -1, 3}
	a := mkIndicators(zeros, zeros, never, h)
	b := mkIndicators(zeros, zeros, diverged, h)

	aDom, bDom, ok := compareVectors(a, b)
	if !ok {
		t.Fatal("vectors not comparable")
	}
	if !aDom {
		t.Fatal("the never-diverged arm must weakly-and-strictly dominate the diverged arm")
	}
	if bDom {
		t.Fatal("the diverged arm must not dominate the never-diverged arm (sentinel inversion)")
	}
}

// TestScalarWinnerNotVectorDominance pins VS2-EC-4: a lower scalar aggregate
// without vector dominance is not beating.
func TestScalarWinnerNotVectorDominance(t *testing.T) {
	const h = 168
	zeros := make([]float64, 8)
	never := []int{-1, -1, -1, -1, -1, -1, -1, -1}

	aMax := append([]float64(nil), zeros...)
	aMax[0] = 0.1 // a is worse on indicator 1
	bMax := append([]float64(nil), zeros...)
	bMax[7] = 0.3 // b is worse on indicator 8
	a := mkIndicators(aMax, zeros, never, h)
	b := mkIndicators(bMax, zeros, never, h)

	// a has the lower scalar sum (0.1 < 0.3) but is worse on indicator 1.
	aSum, bSum := 0.0, 0.0
	for _, ind := range a {
		aSum += ind.Max
	}
	for _, ind := range b {
		bSum += ind.Max
	}
	if aSum >= bSum {
		t.Fatalf("fixture invalid: a sum %v should be lower than b sum %v", aSum, bSum)
	}
	aDom, bDom, ok := compareVectors(a, b)
	if !ok {
		t.Fatal("vectors not comparable")
	}
	if aDom || bDom {
		t.Fatalf("a scalar winner without vector dominance must not dominate: aDom=%t bDom=%t", aDom, bDom)
	}
}

func TestEc3CleanNegative(t *testing.T) {
	const h = 168
	zeros := make([]float64, 8)
	never := []int{-1, -1, -1, -1, -1, -1, -1, -1}
	aMax := append([]float64(nil), zeros...)
	aMax[0] = 0.1
	pMax := append([]float64(nil), zeros...)
	pMax[1] = 0.2
	uMax := append([]float64(nil), zeros...)
	uMax[2] = 0.2
	fid := []fidelityRow{
		fidRow(ArmA, 12, 0, mkIndicators(aMax, zeros, never, h)),
		fidRow(ArmP, 12, 0, mkIndicators(pMax, zeros, never, h)),
		fidRow(ArmU, 12, 0, mkIndicators(uMax, zeros, never, h)),
	}
	band := ec3(fid, 12)
	if band.Status != "clean-negative" {
		t.Fatalf("status %q, want clean-negative (mutually non-dominating)", band.Status)
	}
}

func TestEc3BaselineStrictlyDominatesAdaptive(t *testing.T) {
	const h = 168
	zeros := make([]float64, 8)
	never := []int{-1, -1, -1, -1, -1, -1, -1, -1}
	// Periodic is strictly better than adaptive on indicator 1.
	aMax := append([]float64(nil), zeros...)
	aMax[0] = 0.4
	pMax := append([]float64(nil), zeros...)
	pMax[0] = 0.1
	fid := []fidelityRow{
		fidRow(ArmA, 10, 0, mkIndicators(aMax, zeros, never, h)),
		fidRow(ArmP, 10, 0, mkIndicators(pMax, zeros, never, h)),
		fidRow(ArmU, 10, 0, mkIndicators(zeros, zeros, never, h)),
	}
	band := ec3(fid, 12)
	if band.Status != "FAIL" {
		t.Fatalf("status %q, want FAIL when a baseline strictly dominates adaptive", band.Status)
	}
}

func TestEc3AdaptiveDominates(t *testing.T) {
	const h = 168
	zeros := make([]float64, 8)
	never := []int{-1, -1, -1, -1, -1, -1, -1, -1}
	pMax := append([]float64(nil), zeros...)
	pMax[0] = 0.2
	uMax := append([]float64(nil), zeros...)
	uMax[0] = 0.2
	fid := []fidelityRow{
		fidRow(ArmA, 10, 0, mkIndicators(zeros, zeros, never, h)), // strictly better on indicator 1
		fidRow(ArmP, 10, 0, mkIndicators(pMax, zeros, never, h)),
		fidRow(ArmU, 10, 0, mkIndicators(uMax, zeros, never, h)),
	}
	band := ec3(fid, 12)
	if band.Status != "partial" {
		t.Fatalf("status %q, want partial (adaptive dominates on the cells that ran but fewer than declared)", band.Status)
	}
	band = ec3(fid, 1)
	if band.Status != "PASS" {
		t.Fatalf("status %q, want PASS when all declared cells dominate", band.Status)
	}
}

func TestEc4RefusesAnIncompleteVector(t *testing.T) {
	good := fidRow(ArmP, 10, 0, mkIndicators(make([]float64, 8), make([]float64, 8), []int{-1, -1, -1, -1, -1, -1, -1, -1}, 168))
	if band := ec4([]fidelityRow{good}); band.Status != "PASS" {
		t.Fatalf("complete vector status %q, want PASS", band.Status)
	}
	short := good
	short.Indicators = good.Indicators[:7]
	if band := ec4([]fidelityRow{short}); band.Status != "FAIL" {
		t.Fatalf("incomplete vector status %q, want FAIL", band.Status)
	}
}

func TestEc1NotEvaluableWithoutCells(t *testing.T) {
	band := ec1(map[string]map[string]*cellRun{}, nil, t.TempDir(), 12)
	if band.Status != "not-evaluable" {
		t.Fatalf("status %q, want not-evaluable", band.Status)
	}
}
