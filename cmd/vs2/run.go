package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/experiment"
	"agentworld/internal/world"
)

// cellRun is one arm-in-cell execution and its gathered evidence.
type cellRun struct {
	spec          cellSpec
	fp            fixture
	dir           string
	rootDigest    string
	branchDigest  string
	checks        []world.NaturalCheckpoint
	journal       []world.NaturalBatch
	proj          experiment.Projections
	accepted      acceptedEvidence
	haveAccepted  bool
	calls         []cognition.CallRecord
	interventions []cognition.InterventionRecord
	totals        cognition.CognitiveTotals
	ceilingHit    *cognition.CognitiveCeilingHit
	halted        bool
	runErr        error
	durationMS    int64

	ledgerPath   string
	ledgerCount  uint64
	ledgerDigest string
	finalDigest  string

	sumAliveHours  int64
	escalated      int
	decisionErrors int
	appliedModel   int
	divergedModel  int
	realizedCalls  uint64

	allocationRecords int
	actorCalls        map[uint64]int
	ceilingRows       []cognition.CognitiveRow
	selectorSteps     []selectionStep
	finalCallsSpent   uint64
	finalPRNGPos      uint64
	finalBundleError  string

	callAttempts int
}

// runCell reproduces the recorded genesis, wires the arm's cognition (including
// the L3 arm-local selector for a budgeted arm), runs to the declared horizon
// (or a typed ceiling halt), and gathers the evidence. When replay is non-nil
// the run is a replay-only run whose transport answers from a recorded ledger
// (VS2-EC-5).
func runCell(ctx context.Context, spec cellSpec, fp fixture, root string, replay *cognition.Replay) (*cellRun, error) {
	_, _, cr, err := runCellStack(ctx, spec, fp, root, replay)
	if err != nil {
		return nil, err
	}
	return cr, cr.runErr
}

// runCellStack is runCell returning the constructed runner and cognition stack
// (used by the checkpoint/replay tests). The returned error is the construction
// error; soft run errors are carried on cr.runErr.
func runCellStack(ctx context.Context, spec cellSpec, fp fixture, root string, replay *cognition.Replay) (*world.Natural, *cognitionStack, *cellRun, error) {
	dir, err := prepareRunDir(root, spec)
	if err != nil {
		return nil, nil, nil, err
	}
	f, rootDigest, branchDigest, err := buildBranchRunner(ctx, fp, dir)
	if err != nil {
		return nil, nil, nil, err
	}
	return executeCell(ctx, f, spec, fp, dir, rootDigest, branchDigest, replay)
}

// executeCell wires the arm's cognition onto an already-constructed runner, runs
// to the horizon, and gathers the evidence. The runner is either the recorded
// ticket-19 branch child (the matrix path) or, for the direct-replay tests, a
// births-enabled genesis runner.
func executeCell(ctx context.Context, f *world.Natural, spec cellSpec, fp fixture, dir, rootDigest, branchDigest string, replay *cognition.Replay) (*world.Natural, *cognitionStack, *cellRun, error) {
	stack, err := wireStack(f, spec, fp, dir, budgetFor(spec.Arm), replay)
	if err != nil {
		return nil, nil, nil, err
	}
	cr := &cellRun{spec: spec, fp: fp, dir: dir, rootDigest: rootDigest, branchDigest: branchDigest}
	if stack != nil {
		cr.ledgerPath = stack.ledgerPath
	}
	start := time.Now()
	for {
		checks := f.Checkpoints()
		if len(checks) > 0 && checks[len(checks)-1].Hour >= fp.Hours {
			break
		}
		ok, err := f.Step(ctx)
		if err != nil {
			var hit *cognition.CognitiveCeilingHitError
			if errors.As(err, &hit) {
				h := hit.Hit
				cr.ceilingHit = &h
				cr.halted = true
				break
			}
			cr.runErr = err
			break
		}
		if !ok {
			break
		}
	}
	cr.durationMS = time.Since(start).Milliseconds()
	cr.checks = f.Checkpoints()
	if len(cr.checks) == 0 {
		cr.runErr = fmt.Errorf("runner has no hourly projection")
	}
	cr.proj = experiment.ExtractProjections(cr.checks)
	cr.journal = f.Journal()
	for _, n := range cr.proj.AliveByHour {
		cr.sumAliveHours += int64(n)
	}
	if !cr.halted && cr.runErr == nil {
		handoff, err := f.Handoff()
		if err != nil {
			cr.runErr = err
		} else {
			accepted, err := extractAccepted(handoff, cr.fp.Mechanism)
			if err != nil {
				cr.runErr = err
			} else {
				cr.accepted = accepted
				cr.haveAccepted = true
			}
		}
	}
	if stack != nil {
		if err := finalizeStack(f, stack, cr); err != nil && cr.runErr == nil {
			cr.runErr = fmt.Errorf("finalize: %w", err)
		}
	}
	return f, stack, cr, nil
}

// finalizeStack flushes the ledger, reconciles the journal against the
// recorded-seam replay (with the per-replay selector factory for a
// priority-bearing arm), attaches the checkpoint binding, and records the call
// and intervention inventories.
func finalizeStack(f *world.Natural, stack *cognitionStack, cr *cellRun) error {
	if err := stack.ledger.Flush(); err != nil {
		return err
	}
	summary, err := stack.ledger.Summary()
	if err != nil {
		return err
	}
	cr.ledgerCount = summary.Count
	cr.ledgerDigest = hexString(summary.Digest[:])
	cr.totals = stack.account.Totals()
	seam, factory, err := replaySeamFor(stack, cr.fp)
	if err != nil {
		return err
	}
	f.SetCognitiveCheckpoint(&world.NaturalCognitiveCheckpoint{
		WakePolicy:   stack.wake,
		ModelDigest:  modelIdentityDigestBytes(),
		LedgerDigest: summary.Digest,
		LedgerCount:  summary.Count,
	})
	bundlePath := filepath.Join(cr.dir, "natural-final.bundle")
	digest, err := f.SaveResumeCheckpointWithReplaySelector(bundlePath, seam, stack.wake, factory)
	if err != nil {
		// A post-halt export is refused; the binding is still attached and the
		// ledger summary is recorded. The failure is reported, never swallowed.
		cr.finalBundleError = err.Error()
	} else {
		cr.finalDigest = digestString(digest)
	}
	if !cr.halted {
		calls, err := cognition.ReadCallRecords(cr.ledgerPath)
		if err != nil {
			return err
		}
		cr.calls = calls
		cr.callAttempts = len(calls)
		for _, call := range calls {
			if call.Role == cognition.RoleActorDecision {
				cr.realizedCalls++
			}
		}
		interventions, err := cognition.ReadInterventionRecords(cr.ledgerPath)
		if err != nil {
			return err
		}
		cr.interventions = interventions
		cr.countInterventions()
	}
	if stack.selector != nil {
		cr.selectorSteps = stack.selector.recorded()
		cr.finalCallsSpent = stack.selector.CallsSpent()
		cr.finalPRNGPos = stack.selector.PRNGPos()
	}
	if len(cr.checks) > 0 {
		cr.ceilingRows = stack.account.ObserveHour(cr.checks[len(cr.checks)-1].Hour)
	}
	return nil
}

// replaySeamFor builds the ledger-resolving Role A seam and the per-replay
// selector factory for a run. A0 reconciles with no seam; every other arm
// resolves each recorded decision from the ledger, never a model call. A
// priority-bearing arm additionally supplies a fresh genesis-state selector per
// reference replay (L4b), so its journal reconciles without the construction
// refusal and without a model call.
func replaySeamFor(stack *cognitionStack, fp fixture) (world.ClaimSeam, func() protocol.EscalationSelector, error) {
	if stack == nil {
		return nil, nil, nil
	}
	replay, err := cognition.OpenReplay(stack.ledgerPath)
	if err != nil {
		return nil, nil, err
	}
	seam, err := cognition.NewReplaySeamForContract(replay, stack.wake, declaredClaimContract)
	if err != nil {
		return nil, nil, err
	}
	var factory func() protocol.EscalationSelector
	if stack.wake.Priority != nil {
		factory = newSelectorFactory(stack.wake, fp.Hours, selectorPrice())
	}
	return seam, factory, nil
}

func (cr *cellRun) countInterventions() {
	cr.actorCalls = map[uint64]int{}
	for _, rec := range cr.interventions {
		switch rec.Kind {
		case cognition.InterventionRoleA:
			cr.escalated++
			cr.actorCalls[uint64(rec.Cause.Actor)]++
			if rec.RoleA != nil {
				if rec.RoleA.Source == cognition.SourceModel {
					cr.appliedModel++
					if rec.RoleA.FrozenAction != "" && rec.RoleA.Action != rec.RoleA.FrozenAction {
						cr.divergedModel++
					}
				} else {
					cr.decisionErrors++
				}
			}
		case cognition.InterventionAllocation:
			cr.allocationRecords++
		}
	}
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

func hexString(b []byte) string      { return hex.EncodeToString(b) }
func digestString(d [32]byte) string { return hex.EncodeToString(d[:]) }

// valueModelVersion is the value-model version this cell's declared wake policy
// carried (empty for the arms with no priority policy).
func (cr *cellRun) valueModelVersion() string {
	wake := declaredPolicy(cr.spec.Arm, cr.spec.Seed)
	if wake.Priority == nil {
		return ""
	}
	return wake.Priority.ValueModel.Version
}
