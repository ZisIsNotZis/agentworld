package cognition

import (
	"context"

	"agentworld/internal/strategy"
)

// The minimum honest sandbox / counterfactual (spec section 10.2). The sandbox
// is a deterministic fork pair from the run's own committed checkpoint at the
// audit hour — the patched candidate versus the unpatched parent — both run
// with the seam disabled, same seed, same mechanism, same horizon window. It is
// not a model-conditioned prediction and does not depend on the model; its cost
// is deterministic engine compute.
//
// The world forking is injected through SandboxBranch, so this package never
// imports internal/world (EC-6). The concrete branch is wired by the caller.

// SandboxOutcome is one deterministic branch's declared indicators at the
// window horizon.
type SandboxOutcome struct {
	// BranchDigest is the branch's accepted-history digest over the window; it
	// is recorded so an outside party can re-run the fork.
	BranchDigest [32]byte
	// Indicator is the declared trigger indicator's value at the horizon (the
	// sandbox admits a strictly higher value).
	Indicator int64
	// GatingRegressed reports whether any declared gating band or invariant
	// regressed on this branch.
	GatingRegressed bool
}

// SandboxBranch runs one deterministic fork of the run's committed audit-hour
// checkpoint with the seam disabled. A nil generation is the unpatched parent
// continuation.
type SandboxBranch interface {
	RunBranch(ctx context.Context, gen *strategy.NaturalCognitiveGeneration) (SandboxOutcome, error)
}

// Sandbox verdict reasons.
const (
	SandboxReasonAdmitted      = "admitted"
	SandboxReasonGating        = "gating-regression"
	SandboxReasonNoImprovement = "no-improvement"
)

// SandboxVerdict is the recorded counterfactual verdict.
type SandboxVerdict struct {
	Admitted    bool
	Reason      string
	Parent      SandboxOutcome
	Patched     SandboxOutcome
	WindowHours int
}

// RunSandbox runs the deterministic fork pair and applies the declared
// admission rule: admit iff the patched branch strictly improves the trigger
// indicator and no declared gating band or invariant regresses on either
// branch.
func RunSandbox(ctx context.Context, branch SandboxBranch, candidate strategy.NaturalCognitiveGeneration, windowHours int) (SandboxVerdict, error) {
	if branch == nil {
		return SandboxVerdict{}, ErrPatchRejected
	}
	parent, err := branch.RunBranch(ctx, nil)
	if err != nil {
		return SandboxVerdict{}, err
	}
	patched, err := branch.RunBranch(ctx, &candidate)
	if err != nil {
		return SandboxVerdict{}, err
	}
	verdict := SandboxVerdict{Parent: parent, Patched: patched, WindowHours: windowHours}
	switch {
	case patched.GatingRegressed || parent.GatingRegressed:
		verdict.Reason = SandboxReasonGating
	case patched.Indicator <= parent.Indicator:
		verdict.Reason = SandboxReasonNoImprovement
	default:
		verdict.Admitted = true
		verdict.Reason = SandboxReasonAdmitted
	}
	return verdict, nil
}
