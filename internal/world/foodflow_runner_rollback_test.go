package world

import (
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

// Scheduler calls Err after all evaluator workers join, before planning or
// publishing their proposals. Cancel at that boundary rather than before Step:
// this proves the in-flight Gather attempts themselves are discarded.
type foodFlowCancelAfterEvaluation struct {
	context.Context
	cancel      context.CancelFunc
	runner      *FoodFlow
	checks      int
	attempts    []FoodFlowAttempt
	completions int
	admitted    int
}

func (c *foodFlowCancelAfterEvaluation) Err() error {
	c.checks++
	if c.runner.pending != nil {
		c.runner.pending.mu.Lock()
		c.attempts = append([]FoodFlowAttempt(nil), c.runner.pending.attempts...)
		c.completions = len(c.runner.pending.completed)
		c.runner.pending.mu.Unlock()
	}
	for _, admission := range c.runner.admissions {
		if admission.Rejection == FoodFlowGatherAdmitted {
			c.admitted++
		}
	}
	c.cancel()
	return c.Context.Err()
}

func TestFoodFlowRunnerCancelledGatherCompletionRollsBackAndRetries(t *testing.T) {
	options := FoodFlowOptions{Yield: 3, Seed: 7, Workers: 4}
	interrupted, err := NewFoodFlow(options)
	if err != nil {
		t.Fatal(err)
	}
	control, err := NewFoodFlow(options)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		for _, runner := range []*FoodFlow{interrupted, control} {
			processed, stepErr := runner.Step(context.Background())
			if !processed || stepErr != nil {
				t.Fatalf("setup step %d: processed=%v err=%v", i, processed, stepErr)
			}
		}
	}
	beforeScheduler := interrupted.SchedulerSnapshot()
	gatherTime := sim.SimTime(foodFlowGatherDuration) + 1
	completions := 0
	for _, wake := range beforeScheduler.Wakes {
		if wake.At == gatherTime && wake.Cause == scheduler.WakeCompletion {
			completions++
		}
	}
	if completions != FoodFlowActorCount {
		t.Fatalf("expected %d active Gather completions at %d, got %d", FoodFlowActorCount, gatherTime, completions)
	}
	beforeHistory, beforeHead, err := interrupted.Kernel().ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	beforeJournal := interrupted.Journal()
	beforeHandoff, err := interrupted.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	late := &foodFlowCancelAfterEvaluation{Context: base, cancel: cancel, runner: interrupted}
	processed, err := interrupted.Step(late)
	if processed || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled completion processed=%v err=%v", processed, err)
	}
	if late.checks != 1 || late.completions != FoodFlowActorCount || len(late.attempts) != FoodFlowActorCount || len(interrupted.admissions) != FoodFlowActorCount || int64(late.admitted) != 2*options.Yield {
		t.Fatalf("cancel did not follow evaluated Gather batch: checks=%d completions=%d attempts=%d admissions=%d admitted=%d", late.checks, late.completions, len(late.attempts), len(interrupted.admissions), late.admitted)
	}
	seen := make(map[sim.EntityID]bool, FoodFlowActorCount)
	for _, a := range late.attempts {
		if a.Choice.Kind != strategy.FoodFlowGather || seen[a.Actor] || a.Time != gatherTime {
			t.Fatalf("unexpected evaluated Gather attempt: %+v", a)
		}
		seen[a.Actor] = true
	}
	for actor := sim.EntityID(1); actor <= FoodFlowActorCount; actor++ {
		if !seen[actor] {
			t.Fatalf("actor %d did not complete Gather evaluation", actor)
		}
	}
	if interrupted.pending != nil {
		t.Fatal("failed step retained pending attempts")
	}
	afterHistory, afterHead, err := interrupted.Kernel().ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	afterHandoff, err := interrupted.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeScheduler, interrupted.SchedulerSnapshot()) || !bytes.Equal(beforeHistory, afterHistory) || beforeHead != afterHead || !reflect.DeepEqual(beforeJournal, interrupted.Journal()) || !reflect.DeepEqual(beforeHandoff, afterHandoff) {
		t.Fatal("cancelled Gather completion changed scheduler, kernel history/head, journal, or portable handoff")
	}
	for _, runner := range []*FoodFlow{interrupted, control} {
		processed, stepErr := runner.Step(context.Background())
		if !processed || stepErr != nil {
			t.Fatalf("retry/control completion processed=%v err=%v", processed, stepErr)
		}
	}
	retried, retryHead, err := interrupted.Kernel().ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	uninterrupted, controlHead, err := control.Kernel().ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	retryHandoff, err := interrupted.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	controlHandoff, err := control.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retried, uninterrupted) || retryHead != controlHead || !reflect.DeepEqual(retryHandoff, controlHandoff) || !reflect.DeepEqual(interrupted.Journal(), control.Journal()) {
		t.Fatal("retry diverged from uninterrupted accepted history or activity evidence")
	}
	if retryHead.TipID <= beforeHead.TipID || retryHandoff.Steps != 3 {
		t.Fatalf("retry did not commit the Gather completion: tip=%d steps=%d", retryHead.TipID, retryHandoff.Steps)
	}
	t.Logf("cancelled after %d evaluated h0 Gather attempts with %d planned admissions; retry committed %d new events and matched uninterrupted portable history", len(late.attempts), late.admitted, retryHead.TipID-beforeHead.TipID)
}
