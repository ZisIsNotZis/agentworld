package world

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"context"
	"errors"
	"testing"
)

func TestFoodFlowJournalEmptyAndMissingSchedulerEvidence(t *testing.T) {
	f, err := NewFoodFlow(FoodFlowOptions{Yield: 0, Seed: 7, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFoodFlowJournal(wire, f.k, head, 0, f.journalConfig())
	if err != nil || len(decoded) != 0 {
		t.Fatalf("empty boundary: %v", err)
	}
	for i := 0; i < 4; i++ {
		ok, e := f.Step(context.Background())
		if !ok || e != nil {
			t.Fatalf("step %d: %v", i, e)
		}
	}
	_, head, err = f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	original := f.Journal()
	for _, batch := range []int{1, 3} {
		corrupt := cloneFoodFlowJournal(original)
		corrupt[batch].Attempts = corrupt[batch].Attempts[1:]
		if _, err = EncodeFoodFlowJournal(corrupt, f.k, head, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
			t.Fatalf("omitted rejected-only claim at batch %d: %v", batch, err)
		}
	}
	// A later empty-of-actors hour still needs its basal/production pulse.
	corrupt := cloneFoodFlowJournal(original)
	corrupt = append(corrupt[:2:2], corrupt[3:]...)
	if _, err = EncodeFoodFlowJournal(corrupt, f.k, head, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("omitted pulse: %v", err)
	}
}

func TestFoodFlowJournalSourceAndRejectionEvidence(t *testing.T) {
	f, _, head := foodFlowJournalFixture(t, 3, 4)
	batches := f.Journal()
	foundAccepted, foundDenied := false, false
	for i := range batches {
		for j := range batches[i].Attempts {
			a := &batches[i].Attempts[j]
			if a.Choice.Kind == strategy.FoodFlowGather && a.Accepted && !foundAccepted {
				foundAccepted = true
				changed := cloneFoodFlowJournal(batches)
				changed[i].Attempts[j].Choice.TargetPatch = 1002
				if a.Actor > 8 {
					changed[i].Attempts[j].Choice.TargetPatch = 1001
				}
				if _, err := EncodeFoodFlowJournal(changed, f.k, head, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
					t.Fatalf("foreign accepted source: %v", err)
				}
			}
			if a.Choice.Kind == strategy.FoodFlowGather && !a.Accepted && !foundDenied {
				foundDenied = true
				changed := cloneFoodFlowJournal(batches)
				changed[i].Attempts[j].Rejection = FoodFlowGatherAdmitted
				if _, err := EncodeFoodFlowJournal(changed, f.k, head, f.sched.Time(), f.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
					t.Fatalf("denial disguised as admission: %v", err)
				}
			}
		}
	}
	if !foundAccepted || !foundDenied {
		t.Fatalf("missing mixed Gather evidence: %v %v", foundAccepted, foundDenied)
	}
	wrongConfig := f.journalConfig()
	wrongConfig.Yield = 4
	if _, err := EncodeFoodFlowJournal(batches, f.k, head, f.sched.Time(), wrongConfig); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatalf("history genesis accepted other yield: %v", err)
	}
	var accepted, rejected int
	for _, b := range batches {
		for _, a := range b.Attempts {
			if a.Accepted {
				accepted++
			} else {
				rejected++
			}
		}
	}
	if accepted == 0 || rejected == 0 || head.TipID <= sim.EventID(accepted) {
		t.Fatalf("world pulses absent from accepted history: %+v", head)
	}
	// The direct decoding API needs an independently verified kernel head.
	other, err := NewFoodFlow(FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	valid, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeFoodFlowJournal(valid, other.k, head, f.sched.Time(), other.journalConfig()); !errors.Is(err, ErrFoodFlowJournal) {
		t.Fatal("unverified kernel accepted")
	}
}
