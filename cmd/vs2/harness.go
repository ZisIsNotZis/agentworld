package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/world"
)

// The per-cell world and cognition wiring (spec section 8.2/10). It mirrors the
// cmd/vs1 harness: reproduce the recorded ticket-19 genesis, wire the arm's
// cognition, run to the horizon or a typed ceiling halt, and gather the
// evidence.

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

// naturalOptions is the declared construction shape of a cell's genesis.
func naturalOptions(fp fixture) world.NaturalOptions {
	return world.NaturalOptions{
		Patches: fp.Patches, R: fp.R, Seed: fp.Seed, Workers: fp.effectiveWorkers(), Births: false,
		PolicyRef: basePolicy(), Mechanism: mechanism(), Founders: fp.Founders,
	}
}

// restoreOptions is the declared restore shape of a cell.
func restoreOptions(fp fixture, births bool) world.NaturalRestoreOptions {
	return world.NaturalRestoreOptions{
		Patches: fp.Patches, R: fp.R, Seed: fp.Seed, Workers: fp.effectiveWorkers(), Births: births,
		PolicyRef: basePolicy(), Mechanism: mechanism(), Founders: fp.Founders,
	}
}

// buildBranchRunner reproduces the recorded ticket-19 genesis exactly: the
// neutral h0 pulse captured births-disabled, then the births-enable branch
// child. Its digests are the VS2-EC-1 continuity witnesses (the same A0
// substrate VS1 records).
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
	ledgerPath string
	selector   *armSelector
	wake       protocol.WakePolicy
}

// wireStack attaches the arm's cognition: the deterministic hermetic FakeClient
// recorder and the Role A seam, plus the L3 arm-local selector for a
// priority-bearing arm. When replay is non-nil the recorder's transport answers
// from a recorded ledger instead of the fake, which is the replay-only run of
// VS2-EC-5. The declared budget is passed in so a probe can lower it.
func wireStack(f *world.Natural, spec cellSpec, fp fixture, dir string, budget cognition.CognitiveBudget, replay *cognition.Replay) (*cognitionStack, error) {
	if spec.Arm == ArmA0 {
		return nil, f.SetCognition(nil)
	}
	ledgerPath := filepath.Join(dir, "calls.jsonl")
	ledger, err := cognition.CreateLedger(ledgerPath)
	if err != nil {
		return nil, err
	}
	price := cognition.Pricer(cognition.PriceTable{Version: "vs2-hermetic/1", PromptMicrosPerToken: 1, CompletionMicrosPerToken: 2})
	fake := &cognition.FakeClient{Respond: hermeticResponder(spec)}
	var transport cognition.ModelClient = fake
	endpoint := hermeticEndpoint(declaredGatewayWire)
	if activeLive != nil {
		transport = activeLive.transport
		price = activeLive.price
		endpoint = activeLive.endpoint
	}
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
	wake := declaredPolicy(spec.Arm, spec.Seed)
	seam, err := cognition.NewRoleASeam(cognition.RoleASeamConfig{
		Recorder: recorder, Ledger: ledger, Endpoint: endpoint,
		CredentialPresent: false, WakePolicy: wake,
	})
	if err != nil {
		return nil, err
	}
	stack := &cognitionStack{ledger: ledger, account: account, recorder: recorder, fake: fake, endpoint: endpoint, ledgerPath: ledgerPath, wake: wake}
	cognition := &world.NaturalCognition{Seam: seam, WakePolicy: wake}
	if wake.Priority != nil {
		selector, err := newArmSelector(wake, fp.Hours, selectorPrice(), ledger)
		if err != nil {
			return nil, err
		}
		cognition.Selector = selector
		stack.selector = selector
	}
	if err := f.SetCognition(cognition); err != nil {
		return nil, err
	}
	return stack, nil
}

// replayModelClient answers every recorded call from the ledger by request
// hash (spec section 7.3). It has no network path at all; a miss is a hard
// error.
type replayModelClient struct {
	replay *cognition.Replay
	served int
}

func (c *replayModelClient) Complete(_ context.Context, req cognition.ModelRequest) (cognition.ModelResponse, error) {
	res, err := c.replay.Resolve(req.Request)
	if err != nil {
		return cognition.ModelResponse{}, err
	}
	c.served++
	return cognition.ModelResponse{Body: res.ResponseBody, Content: res.Content, Usage: res.Usage, Status: res.Status}, nil
}

// slotWire is the observation home-slot row the hermetic responder reads.
type slotWire struct {
	Stock int64 `json:"stock"`
}

// hermeticResponder is the deterministic Role A responder. It is a pure
// function of the canonical request bytes, so every arm is deterministic and
// replayable, and it stands in for the unauthorised real model: the content is
// a declared scripted proposal, never a model judgment (spec section 9).
func hermeticResponder(spec cellSpec) func(cognition.ModelRequest) (cognition.ModelResponse, error) {
	return func(req cognition.ModelRequest) (cognition.ModelResponse, error) {
		var envelope struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(req.Request, &envelope); err != nil {
			return cognition.ModelResponse{}, fmt.Errorf("hermetic responder: %w", err)
		}
		if envelope.Role != cognition.RoleActorDecision {
			return cognition.ModelResponse{}, fmt.Errorf("hermetic responder: unknown role %q", envelope.Role)
		}
		var wire struct {
			Actor struct {
				Hour int `json:"hour"`
			} `json:"actor"`
			Observation struct {
				BagUnits           int64      `json:"bag_units"`
				GranaryStock       int64      `json:"granary_stock"`
				LastStoredMealHour int64      `json:"last_stored_meal_hour"`
				HomeSlots          []slotWire `json:"home_slots"`
			} `json:"observation"`
		}
		if err := json.Unmarshal(req.Request, &wire); err != nil {
			return cognition.ModelResponse{}, fmt.Errorf("hermetic responder: %w", err)
		}
		action := hermeticActorAction(wire.Observation.BagUnits, wire.Observation.GranaryStock,
			wire.Observation.LastStoredMealHour, wire.Actor.Hour, wire.Observation.HomeSlots)
		content := []byte(fmt.Sprintf(
			`{"schema_version":1,"action":%q,"rationale":"hermetic scripted provision-first proposal","confidence":1}`,
			action))
		return cognition.ModelResponse{
			Body: content, Content: content,
			Usage:  cognition.Usage{PromptTokens: uint64(len(req.Request)), CompletionTokens: uint64(len(content))},
			Status: cognition.CallStatusOK,
		}, nil
	}
}

// hermeticActorAction is the declared provision-first Role A script: if a
// granary unit may be provisioned, do so; otherwise take a stored meal;
// otherwise gather a stocked home slot; otherwise wait.
func hermeticActorAction(bag, granary, lastStoredMeal int64, hour int, slots []slotWire) string {
	if bag == 0 && granary >= 1 && lastStoredMeal < int64(hour) {
		return "provision"
	}
	if granary >= 1 && lastStoredMeal < int64(hour) {
		return "eat-stored"
	}
	for _, slot := range slots {
		if slot.Stock >= 1 {
			return "gather"
		}
	}
	return "wait"
}
