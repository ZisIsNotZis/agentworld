package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

// cellRun is one arm-in-cell execution and its gathered evidence.
type cellRun struct {
	spec            cellSpec
	fp              fixture
	dir             string
	rootDigest      string
	branchDigest    string
	checks          []world.NaturalCheckpoint
	journal         []world.NaturalBatch
	proj            projections
	accepted        acceptedEvidence
	haveAccepted    bool
	calls           []cognition.CallRecord
	interventions   []cognition.InterventionRecord
	totals          cognition.CognitiveTotals
	ceilingHit      *cognition.CognitiveCeilingHit
	halted          bool
	runErr          error
	durationMS      int64
	ledgerPath      string
	ledgerCount     uint64
	ledgerDigest    string
	finalDigest     string
	sumAliveHours   int64
	escalated       int
	decisionErrors  int
	appliedModel    int
	frozenBase      strategy.NaturalRef
	roleBApplied    int
	roleBRollback   int
	roleBProposed   int
	roleBRejected   int
	roleBNoTrigger  int
	modelDigest     string
	finalBundleName string
	ceilingRows     []cognition.CognitiveRow
	callAttempts    int
	callLatencyUS   int64
}

func isAuditHour(hour int) bool {
	for _, h := range cognition.VS1AuditHours {
		if h == hour {
			return true
		}
	}
	return false
}

// runCell reproduces the recorded genesis, wires the arm's cognition, runs to
// the horizon (or a typed ceiling halt), and gathers the evidence.
func runCell(ctx context.Context, spec cellSpec, fp fixture, root string, replay *cognition.Replay) (*cellRun, error) {
	dir, err := prepareRunDir(root, spec)
	if err != nil {
		return nil, err
	}
	f, rootDigest, branchDigest, err := buildBranchRunner(ctx, fp, dir)
	if err != nil {
		return nil, err
	}
	stack, err := wireStack(f, spec, fp, dir, budgetFor(spec.Arm), replay)
	if err != nil {
		return nil, err
	}
	cr := &cellRun{spec: spec, fp: fp, dir: dir, rootDigest: rootDigest, branchDigest: branchDigest, frozenBase: basePolicy()}
	if stack != nil {
		cr.ledgerPath = stack.ledgerPath
		cr.modelDigest = modelIdentityDigest()
	}
	start := time.Now()
	applyHour := 0
	auditedHour := -1
	for {
		checks := f.Checkpoints()
		if len(checks) == 0 {
			cr.runErr = fmt.Errorf("runner has no hourly projection")
			break
		}
		last := checks[len(checks)-1].Hour
		if last >= fp.Hours {
			break
		}
		if stack != nil && stack.roleB != nil && isAuditHour(last) && last != auditedHour {
			auditedHour = last
			if err := runAudit(ctx, f, stack, dir, fp, cr.spec, last); err != nil {
				cr.runErr = err
				break
			}
			if stack.roleB.applied > 0 && stack.roleB.applyHour > applyHour {
				applyHour = stack.roleB.applyHour
			}
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
	cr.proj = extractProjections(cr.checks)
	cr.journal = f.Journal()
	for _, n := range cr.proj.AliveByHour {
		cr.sumAliveHours += int64(n)
	}
	if !cr.halted && cr.runErr == nil {
		handoff, err := f.Handoff()
		if err != nil {
			cr.runErr = err
		} else {
			accepted, err := extractAccepted(handoff)
			if err != nil {
				cr.runErr = err
			} else {
				cr.accepted = accepted
				cr.haveAccepted = true
			}
		}
	}
	if stack != nil {
		if err := finalizeStack(f, stack, cr, applyHour); err != nil && cr.runErr == nil {
			cr.runErr = fmt.Errorf("finalize: %w", err)
		}
	}
	if cr.haveAccepted {
		_ = cr.accepted
	}
	return cr, cr.runErr
}

// finalizeStack flushes the ledger, reconciles the journal against the
// recorded-seam replay, attaches the checkpoint binding, and records the call
// and intervention inventories.
func finalizeStack(f *world.Natural, stack *cognitionStack, cr *cellRun, applyHour int) error {
	if err := stack.ledger.Flush(); err != nil {
		return err
	}
	summary, err := stack.ledger.Summary()
	if err != nil {
		return err
	}
	cr.ledgerCount = summary.Count
	cr.ledgerDigest = hex.EncodeToString(summary.Digest[:])
	cr.totals = stack.account.Totals()
	seam, wake, err := replaySeamFor(stack, cr.spec)
	if err != nil {
		return err
	}
	binding := &world.NaturalCognitiveCheckpoint{
		WakePolicy:   wakeFor(cr.spec.Arm),
		ModelDigest:  modelIdentityDigestBytes(),
		LedgerDigest: summary.Digest,
		LedgerCount:  summary.Count,
		ApplyHour:    applyHour,
	}
	f.SetCognitiveCheckpoint(binding)
	bundlePath := filepath.Join(cr.dir, "natural-final.bundle")
	digest, err := f.SaveResumeCheckpointWithReplay(bundlePath, seam, wake)
	if err != nil {
		// A post-halt export is refused; the binding is still attached and the
		// ledger summary is recorded.
		_ = err
	} else {
		cr.finalDigest = digestString(digest)
		cr.finalBundleName = filepath.Base(bundlePath)
	}
	if !cr.halted {
		calls, err := cognition.ReadCallRecords(cr.ledgerPath)
		if err != nil {
			return err
		}
		cr.calls = calls
		cr.callAttempts = len(calls)
		for _, call := range calls {
			cr.callLatencyUS += call.LatencyMicros
		}
		interventions, err := cognition.ReadInterventionRecords(cr.ledgerPath)
		if err != nil {
			return err
		}
		cr.interventions = interventions
		cr.countInterventions()
	}
	if len(cr.checks) > 0 {
		cr.ceilingRows = stack.account.ObserveHour(cr.checks[len(cr.checks)-1].Hour)
	}
	return nil
}

// replaySeamFor builds the ledger-resolving Role A seam for a run. The inert
// arm's journal equals the frozen path, so it reconciles with no seam; every
// other model arm resolves each recorded decision from the ledger, never a
// model call.
func replaySeamFor(stack *cognitionStack, spec cellSpec) (world.ClaimSeam, protocol.WakePolicy, error) {
	if spec.Arm == ArmA0 || spec.Arm == ArmA1i {
		return nil, protocol.WakePolicy{}, nil
	}
	replay, err := cognition.OpenReplay(stack.ledgerPath)
	if err != nil {
		return nil, protocol.WakePolicy{}, err
	}
	seam, err := cognition.NewReplaySeam(replay, wakeFor(spec.Arm))
	if err != nil {
		return nil, protocol.WakePolicy{}, err
	}
	return seam, wakeFor(spec.Arm), nil
}

// attachBinding sets the cognition checkpoint binding with the current ledger
// summary, so a save records the block for an active generation.
func attachBinding(f *world.Natural, stack *cognitionStack, session *roleBSession, spec cellSpec) error {
	summary, err := stack.ledger.Summary()
	if err != nil {
		return err
	}
	f.SetCognitiveCheckpoint(&world.NaturalCognitiveCheckpoint{
		WakePolicy:   wakeFor(spec.Arm),
		ModelDigest:  modelIdentityDigestBytes(),
		LedgerDigest: summary.Digest,
		LedgerCount:  summary.Count,
		ApplyHour:    session.applyHour,
	})
	return nil
}

func (cr *cellRun) countInterventions() {
	for _, rec := range cr.interventions {
		switch rec.Kind {
		case cognition.InterventionRoleA:
			cr.escalated++
			if rec.RoleA != nil {
				if rec.RoleA.Source == cognition.SourceModel {
					cr.appliedModel++
				} else {
					cr.decisionErrors++
				}
			}
		case cognition.InterventionPatchApply:
			cr.roleBApplied++
		case cognition.InterventionPatchRollback:
			cr.roleBRollback++
		case cognition.InterventionPatchPropose:
			cr.roleBProposed++
		case cognition.InterventionPatchValidate:
			if rec.Result.Status == "rejected" {
				cr.roleBRejected++
			}
		case cognition.InterventionNoTriggerAudit:
			cr.roleBNoTrigger++
		}
	}
}

// runAudit performs one Role B audit hour: save a committed audit-hour
// checkpoint for the sandbox fork (with the recorded-seam replay so a
// seam-active run's journal reconciles), build the trailing-window evidence
// packet, and drive propose -> validate -> sandbox -> apply -> measure ->
// rollback.
func runAudit(ctx context.Context, f *world.Natural, stack *cognitionStack, dir string, fp fixture, spec cellSpec, hour int) error {
	session := stack.roleB
	// A modification audit has no roster to bind once the population is
	// extinct: the apply/rollback seam refuses an empty roster (a declared
	// no-op, not an error). Record the zero-cost no-effect audit and leave
	// every world byte untouched.
	if checks := f.Checkpoints(); len(checks) > 0 && checks[len(checks)-1].Metrics.Alive == 0 {
		_, err := stack.ledger.AppendIntervention(cognition.InterventionRecord{
			Kind:   cognition.InterventionNoTriggerAudit,
			Cause:  cognition.InterventionCause{WakePolicy: "role-b", Hour: hour, Trigger: "roster-extinct"},
			Cost:   cognition.InterventionCost{},
			Result: cognition.InterventionResult{Status: "no-effect", Reason: "roster-extinct", IndicatorDeltas: []cognition.IndicatorDelta{}},
		})
		if err != nil {
			return fmt.Errorf("audit h%d extinct: %w", hour, err)
		}
		session.auditRecords = append(session.auditRecords, cognition.InterventionRecord{Kind: cognition.InterventionNoTriggerAudit})
		session.noTrigger++
		return nil
	}
	seam, wake, err := replaySeamFor(stack, spec)
	if err != nil {
		return fmt.Errorf("audit h%d replay seam: %w", hour, err)
	}
	if err := attachBinding(f, stack, session, spec); err != nil {
		return fmt.Errorf("audit h%d binding: %w", hour, err)
	}
	cpPath := filepath.Join(dir, fmt.Sprintf("audit-h%03d.bundle", hour))
	if _, err := f.SaveResumeCheckpointWithReplay(cpPath, seam, wake); err != nil {
		return fmt.Errorf("audit h%d save: %w", hour, err)
	}
	session.sandbox.checkpointPath = cpPath
	session.sandbox.restore = restoreOptions(fp, true)
	session.sandbox.restore.CognitiveReplaySeam = seam
	session.sandbox.restore.CognitiveReplayWake = wake
	session.sandbox.restore.CognitiveLedgerPath = stack.ledgerPath
	if session.pending {
		session.sandbox.parentGeneration = session.pendingParent
	} else {
		session.sandbox.parentGeneration = session.head
	}
	startHour := hour - 24
	if startHour < 0 {
		startHour = 0
	}
	evidence := buildEvidence(f.Checkpoints(), f.Journal(), startHour, 24)
	records, err := session.ctrl.Audit(ctx, hour, evidence)
	if err != nil {
		return fmt.Errorf("audit h%d lifecycle: %w", hour, err)
	}
	session.auditRecords = append(session.auditRecords, records...)
	for _, rec := range records {
		switch rec.Kind {
		case cognition.InterventionPatchApply:
			session.pendingParent = session.head
			session.pending = true
			session.applyHour = hour
			session.applied++
		case cognition.InterventionPatchRollback:
			session.pending = false
			session.rollback++
		}
	}
	if gen, ok := f.ActiveCognitiveGeneration(); ok {
		g := gen
		session.head = &g
	} else {
		session.head = nil
	}
	return nil
}

// modelIdentityDigest pins the declared model/sampling identity recorded in
// the checkpoint binding.
func modelIdentityDigest() string {
	d := modelIdentityDigestBytes()
	return hex.EncodeToString(d[:])
}

func modelIdentityDigestBytes() [32]byte {
	return sha256.Sum256([]byte(cognition.DefaultModelID + "|temperature=0.7|top_p=1|max_tokens=128|seed=absent"))
}
