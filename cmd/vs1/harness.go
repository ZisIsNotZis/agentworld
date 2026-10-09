package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/strategy"
	"agentworld/internal/world"
)

// prepareRunDir creates a fresh per-cell run directory. The CLI requires a
// unique directory, so a rerun never clobbers a recorded cell.
func prepareRunDir(root string, spec cellSpec) (string, error) {
	dir := filepath.Join(root, spec.String())
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("run directory already exists: %s", dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func naturalOptions(fp fixture) world.NaturalOptions {
	return world.NaturalOptions{
		Patches: fp.Patches, R: fp.R, Seed: fp.Seed, Workers: 1, Births: false,
		PolicyRef: basePolicy(), Mechanism: mechanism(), Founders: fp.Founders,
	}
}

func restoreOptions(fp fixture, births bool) world.NaturalRestoreOptions {
	return world.NaturalRestoreOptions{
		Patches: fp.Patches, R: fp.R, Seed: fp.Seed, Workers: 1, Births: births,
		PolicyRef: basePolicy(), Mechanism: mechanism(), Founders: fp.Founders,
	}
}

// buildBranchRunner reproduces the recorded ticket-19 genesis exactly: the
// neutral h0 pulse captured births-disabled, then the births-enable branch
// child. Its digests are the EC-1 continuity witnesses.
func buildBranchRunner(ctx context.Context, fp fixture, dir string) (*world.Natural, string, string, error) {
	root, err := world.NewNatural(naturalOptions(fp))
	if err != nil {
		return nil, "", "", err
	}
	if ok, err := root.Step(ctx); err != nil || !ok {
		return nil, "", "", fmt.Errorf("neutral h0 pulse: processed=%t err=%v", ok, err)
	}
	rootPath := filepath.Join(dir, "natural-root.bundle")
	rootDigest, err := root.SaveCheckpoint(rootPath)
	if err != nil {
		return nil, "", "", err
	}
	branchPath := filepath.Join(dir, "natural-branch-enabled.bundle")
	branchDigest, err := world.BranchNaturalCheckpoint(rootPath, restoreOptions(fp, false), true, branchPath)
	if err != nil {
		return nil, "", "", err
	}
	f, restored, err := world.RestoreNaturalCheckpoint(branchPath, restoreOptions(fp, true))
	if err != nil {
		return nil, "", "", err
	}
	if restored != branchDigest {
		return nil, "", "", errors.New("restored branch digest differs from the published branch digest")
	}
	return f, digestString(rootDigest), digestString(branchDigest), nil
}

// cognitionStack is the per-cell recorded-call machinery.
type cognitionStack struct {
	ledger     *cognition.Ledger
	account    *cognition.CognitiveAccount
	recorder   *cognition.Recorder
	fake       *cognition.FakeClient
	endpoint   string
	roleB      *roleBSession
	ledgerPath string
}

// roleBSession tracks the Role B controller and the sandbox branch alongside
// the generation lineage the concrete branch needs.
type roleBSession struct {
	ctrl             *cognition.RoleBController
	sandbox          *naturalSandboxBranch
	head             *strategy.NaturalCognitiveGeneration
	pendingParent    *strategy.NaturalCognitiveGeneration
	pending          bool
	auditRecords     []cognition.InterventionRecord
	applied          int
	rollback         int
	applyHour        int
	proposedRecords  int
	rejectedValidate int
	noTrigger        int
}

// wireStack attaches the arm's cognition: L1's FakeClient recorder, the Role A
// seam (or the inert recording seam), and, for A2/A3, the Role B lifecycle. The
// declared budget is passed in so the scale/ceiling probe can lower it. When
// replay is non-nil the recorder's transport answers from a recorded ledger
// instead of the fake, which is the replay-only run of EC-3.
func wireStack(f *world.Natural, spec cellSpec, fp fixture, dir string, budget cognition.CognitiveBudget, replay *cognition.Replay) (*cognitionStack, error) {
	if spec.Arm == ArmA0 {
		return nil, f.SetCognition(nil)
	}
	ledgerPath := filepath.Join(dir, "calls.jsonl")
	ledger, err := cognition.CreateLedger(ledgerPath)
	if err != nil {
		return nil, err
	}
	price := cognition.PriceTable{Version: "vs1-hermetic/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2}
	fake := &cognition.FakeClient{Respond: scriptedResponder(spec)}
	var transport cognition.ModelClient = fake
	if replay != nil {
		transport = &replayModelClient{replay: replay}
	}
	account, err := cognition.NewCognitiveAccount(budget)
	if err != nil {
		return nil, err
	}
	recorder, err := cognition.NewRecorder(cognition.RecorderConfig{
		Ledger: ledger, Client: transport, Budget: account, Price: price,
		Now:   time.Now,
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		return nil, err
	}
	endpoint := "hermetic.local/v1/chat/completions"
	wake := wakeFor(spec.Arm)
	var seam world.ClaimSeam
	if spec.Arm == ArmA1i {
		seam = &inertSeam{recorder: recorder, ledger: ledger, endpoint: endpoint, wake: wake}
	} else {
		rs, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
			Recorder: recorder, Ledger: ledger, Endpoint: endpoint,
			CredentialPresent: false, WakePolicy: wake,
		})
		if err != nil {
			return nil, err
		}
		seam = rs
	}
	if err := f.SetCognition(&world.NaturalCognition{Seam: seam, WakePolicy: wake}); err != nil {
		return nil, err
	}
	stack := &cognitionStack{ledger: ledger, account: account, recorder: recorder, fake: fake, endpoint: endpoint, ledgerPath: ledgerPath}
	if usesRoleB(spec.Arm) {
		applier := world.NewNaturalCognitiveApplier(f)
		sandbox := &naturalSandboxBranch{live: f, dir: dir, windowHours: 24, restore: restoreOptions(fp, true)}
		proposer, err := cognition.NewRoleBProposer(recorder, endpoint, false)
		if err != nil {
			return nil, err
		}
		ctrl, err := cognition.NewRoleBController(cognition.RoleBControllerConfig{
			Ledger: ledger, Proposer: proposer, Sandbox: sandbox, Applier: applier,
			Operator: cognition.DefaultModelID + "#hermetic-fake", MeasurementWindowHours: 24,
		})
		if err != nil {
			return nil, err
		}
		if err := ctrl.Start(basePolicy()); err != nil {
			return nil, err
		}
		stack.roleB = &roleBSession{ctrl: ctrl, sandbox: sandbox}
	}
	return stack, nil
}

// inertSeam is the A1i seam: the model is called and its response recorded in
// the ledger and the intervention ledger, but the returned decision is the
// frozen fallback so the world discards the model's content. The world path is
// therefore byte-identical to A0 (EC-2), which is what makes the A1 divergence
// attributable to content rather than to the act of calling.
type inertSeam struct {
	recorder *cognition.Recorder
	ledger   *cognition.Ledger
	endpoint string
	wake     protocol.WakePolicy
}

func (s *inertSeam) DecideActorSeam(ctx context.Context, req protocol.ActorSeamRequest) (protocol.ActorSeamDecision, error) {
	canonical, err := cognition.EncodeActorSeamRequest(req)
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	call, err := s.recorder.Call(ctx, cognition.CallRequest{
		Role: cognition.RoleActorDecision, Hour: req.Hour, Model: cognition.DefaultModelID,
		Endpoint: s.endpoint, Sampling: cognition.FrozenSampling(), Request: canonical,
	})
	if err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	// Validate the recorded content so the inert arm's ledger is the same
	// class of evidence as A1's, then discard the decision.
	if _, err := cognition.ValidateActorSeamContent(call.Content, req); err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	recorded := protocol.ActorSeamDecision{Source: protocol.SourceFallback, Reason: "inert", CallRef: call.Seq}
	if _, err := cognition.AppendRoleAIntervention(s.ledger, cognition.RoleAInterventionInput{
		WakePolicy: s.wake, Seam: req.Seam, Actor: req.Actor, Hour: req.Hour,
		SimTimeMicros: int64(req.Hour)*3_600_000_000 + 1, Ref: req.Identity.HeadRef,
		Decision: recorded, Call: call,
	}); err != nil {
		return protocol.ActorSeamDecision{}, err
	}
	return recorded, nil
}
