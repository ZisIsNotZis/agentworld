package main

import (
	"fmt"
	"strings"
)

// renderText is the durable text evaluation: the label, the cell inventory,
// the per-arm totals, the seven-indicator divergence summaries, and the
// verbatim frozen exit-criteria band table.
func renderText(ev evaluation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "VS1 hermetic experiment evaluation\n")
	fmt.Fprintf(&b, "status: %s\n", ev.Status)
	fmt.Fprintf(&b, "revision: %s\n", ev.Revision)
	fmt.Fprintf(&b, "label: %s\n", ev.Label)
	fmt.Fprintf(&b, "honest limit: %s\n", ev.HonestLimit)

	fmt.Fprintf(&b, "\nCELL INVENTORY\n")
	fmt.Fprintf(&b, "declared=%d ran=%d skipped=%d\n", ev.Inventory.Declared, ev.Inventory.Ran, len(ev.Inventory.Skipped))
	if len(ev.Inventory.Skipped) == 0 {
		fmt.Fprintf(&b, "  skipped: none\n")
	}
	for _, s := range ev.Inventory.Skipped {
		fmt.Fprintf(&b, "  skipped: %s\n", s)
	}

	fmt.Fprintf(&b, "\nPER-CELL RESULTS\n")
	fmt.Fprintf(&b, "%-4s %-3s %-2s %10s %8s %8s %8s %10s %10s %8s %6s\n",
		"arm", "f", "s", "calls", "escal", "model", "errs", "cost_us", "calls/ah", "b_app", "halt")
	for _, c := range ev.Cells {
		fmt.Fprintf(&b, "%-4s %-3d %-2d %10d %8d %8d %8d %10d %10.4f %8d %6t\n",
			c.Arm, c.F, c.Seed, c.CallCount, c.Escalated, c.AppliedModel, c.DecisionErrors,
			c.CostMicros, c.CallsPerActorHour, c.RoleBApplied, c.Halted)
	}

	fmt.Fprintf(&b, "\nFIDELITY-VECTOR SCAFFOLD (emitted, not gated; budgeted arm vs A3 reference)\n")
	fmt.Fprintf(&b, "%-4s %-3s %-2s %-4s %-24s %8s %8s %6s\n", "arm", "f", "s", "ref", "indicator", "max", "mean", "first")
	for _, row := range ev.Fidelity {
		for _, ind := range row.Indicators {
			fmt.Fprintf(&b, "%-4s %-3d %-2d %-4s %-24s %8.4f %8.4f %6d\n",
				row.Arm, row.F, row.Seed, row.Reference, ind.Name, ind.Max, ind.Mean, ind.FirstDivergenceHour)
		}
	}
	if len(ev.Fidelity) == 0 {
		fmt.Fprintf(&b, "  none (no A3 reference run)\n")
	}

	if len(ev.Fixtures) > 0 {
		fmt.Fprintf(&b, "\nSCRIPTED-PROPOSER FIXTURE (labeled; not a declared arm)\n")
		for _, f := range ev.Fixtures {
			fmt.Fprintf(&b, "%s\n", f.Label)
			fmt.Fprintf(&b, "%s f%d-s%d: proposed=%d admitted=%d applied=%d measured=%d rolled-back=%d measured_window=%dh\n",
				f.Arm, f.F, f.Seed, f.Proposed, f.Admitted, f.Applied, f.Measured, f.RolledBack, f.MeasuredWindowHours)
			fmt.Fprintf(&b, "unscripted A2 honest null (separate): proposed=%d validated=%d applied=%d rejected=%d rolled-back=%d\n",
				f.UnscriptedA2Proposed, f.UnscriptedA2Validated, f.UnscriptedA2Applied, f.UnscriptedA2Rejected, f.UnscriptedA2Rolled)
		}
	}

	if len(ev.Twins) > 0 {
		fmt.Fprintf(&b, "\nDETERMINISM TWINS (EC-8; workers-4 vs workers-1)\n")
		for _, tw := range ev.Twins {
			fmt.Fprintf(&b, "%s f%d-s%d w%d vs w1: identical=%t w1_tip=%s w4_tip=%s w1_branch=%s w4_branch=%s %s\n",
				tw.Arm, tw.F, tw.Seed, tw.Workers, tw.Identical, tw.W1TipHash, tw.W4TipHash, tw.W1BranchDigest, tw.W4BranchDigest, tw.Note)
		}
	}

	if ev.Probe != nil {
		fmt.Fprintf(&b, "\nSCALE/CEILING PROBE (descriptive)\n")
		fmt.Fprintf(&b, "P=32 founders=%d max_calls=%d ceiling=%s threshold=%d observed=%d hour=%d ledger=%d replayable=%t\n",
			ev.Probe.Founders, ev.Probe.MaxCalls, ev.Probe.Ceiling, ev.Probe.Threshold, ev.Probe.Observed,
			ev.Probe.HitHour, ev.Probe.LedgerCount, ev.Probe.Replayable)
		fmt.Fprintf(&b, "P15: %s\n", ev.Probe.P15Verdict)
	}

	fmt.Fprintf(&b, "\nFROZEN EXIT CRITERIA BANDS\n")
	for _, band := range ev.Bands {
		fmt.Fprintf(&b, "[%s] %s\n", band.Status, band.ID)
		fmt.Fprintf(&b, "  criterion: %s\n", band.Criterion)
		fmt.Fprintf(&b, "  evidence: %s\n", band.Evidence)
	}

	fmt.Fprintf(&b, "\nREPLAY SUBSET\n")
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
