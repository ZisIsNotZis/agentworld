package main

import (
	"fmt"
	"strings"
)

// renderText is the durable text evaluation: the revision binding, the cell
// inventory, the per-arm realized counts and under-budget flags, the
// divergence-vector summaries with raw and normalised first-divergence hours,
// and the verbatim VS2 exit-criteria band table.
func renderText(ev evaluation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "VS2 attention experiment evaluation\n")
	fmt.Fprintf(&b, "status: %s\n", ev.Status)
	fmt.Fprintf(&b, "revision: %s\n", ev.Revision)
	fmt.Fprintf(&b, "model: %s wire: %s identity: %s\n", ev.ModelID, ev.WireMode, ev.ModelIdentity)
	fmt.Fprintf(&b, "label: %s\n", ev.Label)
	fmt.Fprintf(&b, "honest limit: %s\n", ev.HonestLimit)
	fmt.Fprintf(&b, "horizon H=%d call ceiling N=%d value model=%s\n", ev.HorizonHours, ev.CallCeiling, ev.ValueModel)

	fmt.Fprintf(&b, "\nCELL INVENTORY\n")
	fmt.Fprintf(&b, "declared=%d ran=%d skipped=%d\n", ev.Inventory.Declared, ev.Inventory.Ran, len(ev.Inventory.Skipped))
	for _, s := range ev.Inventory.Skipped {
		fmt.Fprintf(&b, "  skipped: %s\n", s)
	}

	fmt.Fprintf(&b, "\nPER-CELL RESULTS (realized calls and under-budget, VS2-EC-2)\n")
	fmt.Fprintf(&b, "%-3s %-3s %-2s %9s %8s %5s %8s %8s %8s %6s\n",
		"arm", "f", "s", "ceiling", "realized", "under", "escal", "model", "alloc", "halt")
	for _, c := range ev.Cells {
		fmt.Fprintf(&b, "%-3s %-3d %-2d %9d %8d %5t %8d %8d %8d %6t\n",
			c.Arm, c.F, c.Seed, c.CallCeiling, c.RealizedCalls, c.UnderBudget, c.Escalated, c.AppliedModel, c.AllocationRecords, c.Halted)
	}

	fmt.Fprintf(&b, "\nDIVERGENCE VECTOR (budgeted arm vs R reference; raw and normalised first-divergence hour)\n")
	fmt.Fprintf(&b, "%-3s %-3s %-2s %-24s %9s %9s %6s %6s\n", "arm", "f", "s", "indicator", "max", "mean", "first", "norm")
	for _, row := range ev.Fidelity {
		for _, ind := range row.Indicators {
			fmt.Fprintf(&b, "%-3s %-3d %-2d %-24s %9s %9s %6d %6d\n",
				row.Arm, row.F, row.Seed, ind.Name, fmtFloat(ind.Max), fmtFloat(ind.Mean), ind.FirstDivergenceHour, ind.NormalizedFirstDivergenceHour)
		}
	}
	if len(ev.Fidelity) == 0 {
		fmt.Fprintf(&b, "  none (no complete A/P/U vs R cell)\n")
	}

	fmt.Fprintf(&b, "\nVALUE-MODEL CALIBRATION (VS2-EC-8)\n")
	for _, c := range ev.Cells {
		if !isBudgeted(c.Arm) {
			continue
		}
		cal := c.Calibration
		fmt.Fprintf(&b, "%-3s f%d-s%d: model=%s candidates=%d predicted_mean=%s normalized=%s realized_divergence=%s error=%s\n",
			c.Arm, c.F, c.Seed, cal.ValueModelVersion, cal.EscalatedCandidates,
			fmtFloat(cal.PredictedPriorityMean), fmtFloat(cal.NormalizedPredictedMean),
			fmtFloat(cal.RealizedDivergenceRate), fmtFloat(cal.CalibrationError))
	}

	fmt.Fprintf(&b, "\nFROZEN EXIT CRITERIA BANDS\n")
	for _, band := range ev.Bands {
		fmt.Fprintf(&b, "[%s] %s\n", band.Status, band.ID)
		fmt.Fprintf(&b, "  criterion: %s\n", band.Criterion)
		fmt.Fprintf(&b, "  evidence: %s\n", band.Evidence)
	}

	fmt.Fprintf(&b, "\nREPLAY SUBSET (priority arms, VS2-EC-5)\n")
	if len(ev.ReplaySubset) == 0 {
		fmt.Fprintf(&b, "  none\n")
	}
	for _, k := range ev.ReplaySubset {
		result := "identical"
		for _, m := range ev.ReplayMismatches {
			if strings.HasPrefix(m, k+":") {
				result = strings.TrimPrefix(m, k+":")
			}
		}
		fmt.Fprintf(&b, "  %s: %s\n", k, result)
	}
	return b.String()
}
