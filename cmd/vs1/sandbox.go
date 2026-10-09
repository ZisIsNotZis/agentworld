package main

import (
	"context"
	"errors"
	"fmt"

	"agentworld/internal/cognition"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

// naturalSandboxBranch is the concrete SandboxBranch L3 injected: a
// deterministic fork pair from the live run's own committed audit-hour
// checkpoint, restored with the seam disabled, run to the declared window
// horizon (spec section 10.2). RunBranch(nil) is the unpatched parent
// continuation (the head generation at the audit boundary, or the frozen base);
// RunBranch(&candidate) activates the candidate generation on the restored
// roster. Neither branch wires a Role A seam, so no model call happens in a
// fork.
type naturalSandboxBranch struct {
	live             *world.Natural
	dir              string
	windowHours      int
	restore          world.NaturalRestoreOptions
	checkpointPath   string
	parentGeneration *strategy.NaturalCognitiveGeneration
}

// RunBranch implements cognition.SandboxBranch. RunBranch(nil) is the
// unpatched parent continuation; the restored runner carries no live seam, so
// both forks are seam-disabled and the recorded Role A decisions are
// reproduced only by the restore-time reference replay.
func (b *naturalSandboxBranch) RunBranch(ctx context.Context, gen *strategy.NaturalCognitiveGeneration) (cognition.SandboxOutcome, error) {
	if b == nil || b.live == nil || b.checkpointPath == "" {
		return cognition.SandboxOutcome{}, fmt.Errorf("sandbox branch requires a committed audit-hour checkpoint")
	}
	f, _, err := world.RestoreNaturalCheckpoint(b.checkpointPath, b.restore)
	if err != nil {
		return cognition.SandboxOutcome{}, err
	}
	active := gen
	if active == nil {
		active = b.parentGeneration
	}
	if active == nil {
		// The restored runner may carry the block's head generation; the
		// frozen-base parent fork must clear it.
		if err := f.ClearCognitiveGeneration(); err != nil {
			return cognition.SandboxOutcome{}, err
		}
	} else {
		applier := world.NewNaturalCognitiveApplier(f)
		actors, err := applier.RosterActors(ctx)
		if err != nil {
			return cognition.SandboxOutcome{}, err
		}
		if err := f.ActivateCognitiveGeneration(*active, actors); err != nil {
			return cognition.SandboxOutcome{}, err
		}
	}
	checks := f.Checkpoints()
	if len(checks) == 0 {
		return cognition.SandboxOutcome{}, fmt.Errorf("sandbox branch has no checkpoint")
	}
	start := checks[len(checks)-1].Hour
	halted := false
	for {
		checks = f.Checkpoints()
		if checks[len(checks)-1].Hour >= start+b.windowHours {
			break
		}
		ok, err := f.Step(ctx)
		if err != nil {
			var hit *world.NaturalCeilingHitError
			if errors.As(err, &hit) {
				halted = true
				break
			}
			return cognition.SandboxOutcome{}, err
		}
		if !ok {
			break
		}
	}
	checks = f.Checkpoints()
	journal := f.Journal()
	evidence := buildEvidence(checks, journal, start, b.windowHours)
	extinct := len(checks) > 0 && checks[len(checks)-1].Metrics.Alive == 0
	return cognition.SandboxOutcome{
		BranchDigest:    branchStateDigest(checks, journal),
		Indicator:       triggerIndicatorOf(evidence),
		GatingRegressed: halted || extinct,
	}, nil
}
