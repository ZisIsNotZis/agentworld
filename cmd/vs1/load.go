package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"agentworld/internal/cognition"
	"agentworld/internal/experiment"
	"agentworld/internal/world"
)

// loadCell reconstructs a matrix cell's evidence from its committed run
// directory without re-simulating the world. It is the reuse path: the final
// bundle supplies the hourly projections and the accepted history, the ledger
// supplies the call and intervention inventories, and the prior report
// supplies the recorder accounting the bundle does not carry (per-run totals,
// ceiling rows, duration). Every reconstructed world fact is checked against
// the prior report, so reuse can never silently diverge from the recorded
// cell; a mismatch is a hard error and the harness declines to reuse it.
func loadCell(prior cellReport, spec cellSpec, fp fixture, root string) (*cellRun, error) {
	dir := filepath.Join(root, spec.String())
	ledgerPath := filepath.Join(dir, "calls.jsonl")
	opts := restoreOptions(fp, true)
	if spec.Arm != ArmA0 {
		// Every cognitive lane (including the seam-only inert arm) binds its
		// recorded-call ledger in the manifest cognition block, so the restore
		// requires and verifies the ledger path.
		opts.CognitiveLedgerPath = ledgerPath
	}
	if spec.Arm != ArmA0 && spec.Arm != ArmA1i {
		replay, err := cognition.OpenReplay(ledgerPath)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", spec.String(), err)
		}
		seam, err := cognition.NewReplaySeam(replay, wakeFor(spec.Arm))
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", spec.String(), err)
		}
		opts.CognitiveReplaySeam = seam
		opts.CognitiveReplayWake = wakeFor(spec.Arm)
	}
	f, _, err := world.RestoreNaturalCheckpoint(filepath.Join(dir, "natural-final.bundle"), opts)
	if err != nil {
		return nil, fmt.Errorf("load %s final bundle: %w", spec.String(), err)
	}
	cr := &cellRun{
		spec: spec, fp: fp, dir: dir, frozenBase: basePolicy(), ledgerPath: ledgerPath,
		rootDigest: prior.RootDigest, branchDigest: prior.BranchDigest,
		totals: cognition.CognitiveTotals{
			Calls: prior.CallCount, PromptTokens: prior.PromptTokens,
			CompletionTokens: prior.CompletionTokens, CostMicros: prior.CostMicros,
		},
		ceilingRows: prior.CeilingRows, durationMS: prior.DurationMS, halted: prior.Halted,
		finalBundleError: prior.FinalBundleError,
	}
	if prior.CeilingHit != "" {
		cr.ceilingHit = &cognition.CognitiveCeilingHit{Ceiling: cognition.CognitiveCeiling(prior.CeilingHit)}
	}
	cr.checks = f.Checkpoints()
	cr.proj = experiment.ExtractProjections(cr.checks)
	cr.journal = f.Journal()
	handoff, err := f.Handoff()
	if err != nil {
		return nil, fmt.Errorf("load %s handoff: %w", spec.String(), err)
	}
	accepted, err := extractAccepted(handoff)
	if err != nil {
		return nil, fmt.Errorf("load %s accepted history: %w", spec.String(), err)
	}
	cr.accepted, cr.haveAccepted = accepted, true
	for _, n := range cr.proj.AliveByHour {
		cr.sumAliveHours += int64(n)
	}
	if spec.Arm != ArmA0 {
		data, err := os.ReadFile(ledgerPath)
		if err != nil {
			return nil, fmt.Errorf("load %s ledger: %w", spec.String(), err)
		}
		summary, err := cognition.VerifyLedgerData(data)
		if err != nil {
			return nil, fmt.Errorf("load %s ledger verify: %w", spec.String(), err)
		}
		cr.ledgerCount = summary.Count
		cr.ledgerDigest = hex.EncodeToString(summary.Digest[:])
		calls, err := cognition.ReadCallRecords(ledgerPath)
		if err != nil {
			return nil, fmt.Errorf("load %s calls: %w", spec.String(), err)
		}
		cr.calls = calls
		cr.callAttempts = len(calls)
		for _, call := range calls {
			cr.callLatencyUS += call.LatencyMicros
		}
		interventions, err := cognition.ReadInterventionRecords(ledgerPath)
		if err != nil {
			return nil, fmt.Errorf("load %s interventions: %w", spec.String(), err)
		}
		cr.interventions = interventions
		cr.countInterventions()
	}
	if err := validateLoadedCell(prior, cr); err != nil {
		return nil, err
	}
	return cr, nil
}

// validateLoadedCell refuses a reuse whose reconstructed world facts differ
// from the prior report. It is the guard that keeps reuse evidence-honest.
func validateLoadedCell(prior cellReport, cr *cellRun) error {
	mismatch := func(field, got, want string) error {
		return fmt.Errorf("load %s: %s %s, prior report %s", cr.spec.String(), field, got, want)
	}
	if cr.accepted.TipHash != prior.TipHash {
		return mismatch("tip hash", cr.accepted.TipHash, prior.TipHash)
	}
	if cr.accepted.ProjectionHash != prior.ProjectionHash {
		return mismatch("projection hash", cr.accepted.ProjectionHash, prior.ProjectionHash)
	}
	if cr.accepted.EventCount != prior.EventCount {
		return mismatch("event count", fmt.Sprint(cr.accepted.EventCount), fmt.Sprint(prior.EventCount))
	}
	if cr.accepted.HistoryBytes != prior.HistoryBytes {
		return mismatch("history bytes", fmt.Sprint(cr.accepted.HistoryBytes), fmt.Sprint(prior.HistoryBytes))
	}
	if !equalInts(cr.proj.AliveByHour, prior.AliveByHour) {
		return mismatch("alive-by-hour", "differs", "differs")
	}
	return nil
}
