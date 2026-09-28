package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFoodFlowJournalRejectsForgedWaitAtActiveGatherBoundary(t *testing.T) {
	f, _, head := foodFlowJournalFixture(t, 3, 2)
	original := f.Journal()
	if len(original) != 2 || len(original[1].Activities) != FoodFlowActorCount {
		t.Fatal("not an active Gather boundary")
	}
	forged := cloneFoodFlowJournal(original)
	claim := &forged[1]
	claim.Activities = nil
	for actor := sim.EntityID(1); actor <= FoodFlowActorCount; actor++ {
		claim.Attempts = append(claim.Attempts, FoodFlowAttempt{
			Actor: actor, Time: claim.Time, Key: foodFlowKey("wait", claim.Time, actor),
			Choice:    strategy.FoodFlowChoice{Kind: strategy.FoodFlowWait, ObservedVersion: original[0].Version, Ref: f.ref, Fallback: strategy.FoodFlowNoCandidate},
			Rejection: FoodFlowGatherIneligible,
		})
	}
	// The history-only API cannot detect that a rehashed journal substituted
	// Wait for 16 active fibers: no Gather has committed a kernel event yet.
	wire, err := EncodeFoodFlowJournal(forged, f.k, head, f.sched.Time(), f.journalConfig())
	if err != nil {
		t.Fatalf("invalid adversarial fixture: %v", err)
	}
	if _, err = DecodeFoodFlowJournal(wire, f.k, head, f.sched.Time(), f.journalConfig()); err != nil {
		t.Fatalf("history-only mode should be partial: %v", err)
	}
	if decoded, err := DecodeFoodFlowJournalWithScheduler(wire, f.k, head, f.sched, f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) || decoded != nil {
		t.Fatalf("forged Wait passed complete decoder: %v", err)
	}
	f.journal = forged
	if data, err := f.ExportJournal(); !errors.Is(err, ErrFoodFlowJournal) || data != nil {
		t.Fatalf("forged Wait exported: %v", err)
	}
	f.journal = original
	if err = f.RestoreJournal(wire, head); !errors.Is(err, ErrFoodFlowJournal) || !reflect.DeepEqual(f.Journal(), original) {
		t.Fatalf("forged Wait restored or mutated journal: %v", err)
	}
}

func TestFoodFlowJournalRejectsForeignActorDeltasFromReplayableHistory(t *testing.T) {
	newBoundary := func() *FoodFlow {
		f, err := NewFoodFlow(FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			ok, e := f.Step(context.Background())
			if !ok || e != nil {
				t.Fatalf("setup: %v", e)
			}
		}
		return f
	}
	commitAndRestore := func(f *FoodFlow, p kernel.Proposal) kernel.Event {
		t.Helper()
		plan, err := f.k.Plan(p, f.k.SnapshotHead().Authority)
		if err != nil {
			t.Fatalf("foreign-actor plan: %v", err)
		}
		events, err := f.k.CommitBatch([]kernel.Plan{plan})
		if err != nil || len(events) != 1 {
			t.Fatalf("foreign-actor commit: %v", err)
		}
		history, head, err := f.k.ExportHistory()
		if err != nil {
			t.Fatal(err)
		}
		replay, replayHead, err := kernel.RestoreHistory(f.registry, history)
		if err != nil || replayHead != head {
			t.Fatalf("history failed accepted replay: %v", err)
		}
		return replay.Events()[len(replay.Events())-1]
	}
	f := newBoundary()
	gatherAt := sim.SimTime(foodFlowGatherDuration) + 1
	gather := foodFlowProposal("gather", gatherAt, 1, FoodFlowGatherRule, []component.Patch{
		foodFlowNumber(2001, FoodFlowSlotTypeID, FoodFlowSlotStockField, 0),
		foodFlowNumber(2001, FoodFlowSlotTypeID, FoodFlowSlotGatheredField, 1),
		foodFlowNumber(2, FoodFlowBagTypeID, FoodFlowBagUnitsField, 1), // actor B, not causal actor A
		foodFlowPatch(1, FoodFlowBagTypeID, FoodFlowBagSourceField, foodFlowRef(1001)),
		foodFlowNumber(1, FoodFlowBodyTypeID, FoodFlowBodyLastGatherHourField, 0),
	})
	journalForGather := func(f *FoodFlow, ev kernel.Event) []FoodFlowBatch {
		batches := f.Journal()
		completion := FoodFlowBatch{Time: gatherAt, Version: ev.AfterVersion, TipID: ev.ID, TipHash: ev.Hash}
		for i := range batches[1].Activities {
			a := &batches[1].Activities[i]
			a.Completed = gatherAt
			attempt := FoodFlowAttempt{Actor: a.Actor, Time: gatherAt, Choice: f.choices[a.Actor], Rejection: FoodFlowGatherCapacity, Key: foodFlowKey("gather-denied", gatherAt, a.Actor)}
			kind := strategy.FoodFlowRest
			if a.Actor == 1 {
				a.EventID = ev.ID
				attempt.Accepted, attempt.EventID, attempt.Rejection = true, ev.ID, FoodFlowGatherAdmitted
				attempt.Key = foodFlowKey("gather", gatherAt, a.Actor)
				kind = strategy.FoodFlowEat
			}
			completion.Attempts = append(completion.Attempts, attempt)
			completion.Activities = append(completion.Activities, FoodFlowActivity{Actor: a.Actor, Kind: kind, Started: gatherAt, Token: a.Token + FoodFlowActorCount})
		}
		return append(batches, completion)
	}
	ev := commitAndRestore(f, gather)
	if foodFlowJournalEventSource(ev, strategy.FoodFlowGather, 1, 1001, 2001) {
		t.Fatal("valid-history foreign Gather bag accepted as actor A evidence")
	}
	_, foreignHead, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EncodeFoodFlowJournal(journalForGather(f, ev), f.k, foreignHead, gatherAt, f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("foreign actor passed accepted journal link: %v", err)
	}
	f = newBoundary()
	legitimate := foodFlowProposal("gather", gatherAt, 1, FoodFlowGatherRule, []component.Patch{
		foodFlowNumber(2001, FoodFlowSlotTypeID, FoodFlowSlotStockField, 0),
		foodFlowNumber(2001, FoodFlowSlotTypeID, FoodFlowSlotGatheredField, 1),
		foodFlowNumber(1, FoodFlowBagTypeID, FoodFlowBagUnitsField, 1),
		foodFlowPatch(1, FoodFlowBagTypeID, FoodFlowBagSourceField, foodFlowRef(1001)),
		foodFlowNumber(1, FoodFlowBodyTypeID, FoodFlowBodyLastGatherHourField, 0),
	})
	ev = commitAndRestore(f, legitimate)
	if !foodFlowJournalEventSource(ev, strategy.FoodFlowGather, 1, 1001, 2001) {
		t.Fatal("legitimate Gather source rejected")
	}
	_, legitimateHead, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EncodeFoodFlowJournal(journalForGather(f, ev), f.k, legitimateHead, gatherAt, f.journalConfig()); err != nil {
		t.Fatalf("legitimate linked journal fixture invalid: %v", err)
	}
	eatAt := gatherAt + sim.SimTime(foodFlowMealDuration)
	eat := foodFlowProposal("eat", eatAt, 1, FoodFlowConsumeRule, []component.Patch{
		foodFlowNumber(1, FoodFlowBagTypeID, FoodFlowBagUnitsField, 0),
		foodFlowPatch(1, FoodFlowBagTypeID, FoodFlowBagSourceField, foodFlowMissing()),
		foodFlowNumber(1, FoodFlowBodyTypeID, FoodFlowBodyConsumedField, 1),
		foodFlowNumber(1, FoodFlowBodyTypeID, FoodFlowBodyEnergyField, 12),
		foodFlowNumber(1, FoodFlowBodyTypeID, FoodFlowBodyCapLostField, 0),
		foodFlowNumber(2, FoodFlowBodyTypeID, FoodFlowBodyHungerField, 0), // actor B, not causal actor A
	})
	ev = commitAndRestore(f, eat)
	if foodFlowJournalEventSource(ev, strategy.FoodFlowEat, 1, 1001, 0) {
		t.Fatal("valid-history foreign Eat hunger accepted as actor A evidence")
	}
	batches := journalForGather(f, f.k.Events()[2])
	for i := range batches[2].Activities {
		batches[2].Activities[i].Completed = eatAt
		if batches[2].Activities[i].Actor == 1 {
			batches[2].Activities[i].EventID = ev.ID
		}
	}
	batches = append(batches, FoodFlowBatch{
		Time: eatAt, Version: ev.AfterVersion, TipID: ev.ID, TipHash: ev.Hash,
		Attempts: []FoodFlowAttempt{{Actor: 1, Time: eatAt, Key: foodFlowKey("eat", eatAt, 1), Choice: strategy.FoodFlowChoice{Kind: strategy.FoodFlowEat, TargetPatch: 1001, ObservedVersion: ev.BeforeVersion, Ref: f.ref, Evaluated: 1}, Accepted: true, EventID: ev.ID}},
	})
	_, foreignEatHead, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EncodeFoodFlowJournal(batches, f.k, foreignEatHead, eatAt, f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("foreign Eat actor passed accepted journal link: %v", err)
	}
}
