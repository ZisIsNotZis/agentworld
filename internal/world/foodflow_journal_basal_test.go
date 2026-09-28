package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"context"
	"errors"
	"testing"
)

// Build a replayable accepted history from the pre-death boundary to test
// journal coverage independently of the runner and scheduler. These synthetic
// continuations still use kernel planning, commit, export and accepted replay.
func foodFlowJournalPulse(t *testing.T, k *kernel.Kernel, hour int, actors []sim.EntityID) FoodFlowBatch {
	t.Helper()
	at, _ := FoodFlowHourTime(hour)
	head := k.SnapshotHead()
	patches, slots, bodies, err := foodFlowStates(scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version})
	if err != nil {
		t.Fatal(err)
	}
	var proposals []kernel.Proposal
	for _, actor := range actors {
		next, _, err := FoodFlowBasal(bodies[actor-1], 1)
		if err != nil {
			t.Fatal(err)
		}
		ps := []component.Patch{
			foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyEnergyField, next.Energy),
			foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyHungerField, next.Hunger),
			foodFlowNumber(actor, FoodFlowBodyTypeID, FoodFlowBodyBasalSpentField, next.BasalSpent),
		}
		p := foodFlowProposal("basal", at, actor, FoodFlowBasalRule, ps)
		p.Cause = kernel.Cause{World: true}
		proposals = append(proposals, p)
	}
	if hour < FoodFlowHorizonHours {
		for i, p := range patches {
			next, stock, _, err := FoodFlowProduce(p, slots[i], hour)
			if err != nil {
				t.Fatal(err)
			}
			id, _ := FoodFlowPatchID(i)
			ps := []component.Patch{
				foodFlowNumber(id, FoodFlowPatchTypeID, FoodFlowPatchPulsesField, next.Pulses),
				foodFlowNumber(id, FoodFlowPatchTypeID, FoodFlowPatchProducedField, next.Produced),
				foodFlowNumber(id, FoodFlowPatchTypeID, FoodFlowPatchUnrealizedField, next.Unrealized),
			}
			for j, s := range stock {
				if s.Stock != slots[i][j].Stock {
					slot, _ := FoodFlowSlotID(i, j)
					ps = append(ps, foodFlowNumber(slot, FoodFlowSlotTypeID, FoodFlowSlotStockField, s.Stock))
				}
			}
			proposals = append(proposals, foodFlowProposal("produce-"+string(rune('0'+i)), at, 0, FoodFlowProduceRule, ps))
		}
	}
	plans := make([]kernel.Plan, 0, len(proposals))
	for _, p := range proposals {
		plan, err := k.Plan(p, head.Authority)
		if err != nil {
			t.Fatalf("h%d plan %s: %v", hour, p.Key, err)
		}
		plans = append(plans, plan)
	}
	if _, err = k.CommitBatch(plans); err != nil {
		t.Fatalf("h%d commit: %v", hour, err)
	}
	_, verified, err := k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return FoodFlowBatch{Time: at, Version: verified.Version, TipID: verified.TipID, TipHash: verified.TipHash}
}
func foodFlowJournalUntil(t *testing.T, q int64, hour int, finishActivities bool) *FoodFlow {
	t.Helper()
	f, err := NewFoodFlow(FoodFlowOptions{Yield: q, Seed: 7, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	at, _ := FoodFlowHourTime(hour)
	if finishActivities {
		at += 1 + sim.SimTime(foodFlowGatherDuration) + sim.SimTime(foodFlowMealDuration)
	}
	for f.sched.Time() < at || len(f.Journal()) == 0 {
		ok, e := f.Step(context.Background())
		if !ok || e != nil {
			t.Fatalf("h%d step: %v", hour, e)
		}
	}
	if f.sched.Time() != at {
		t.Fatalf("wrong boundary: got %d want %d", f.sched.Time(), at)
	}
	return f
}
func foodFlowJournalValidHistory(t *testing.T, f *FoodFlow) kernel.PortableHead {
	t.Helper()
	wire, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	_, actual, err := kernel.RestoreHistory(f.registry, wire)
	if err != nil || actual != head {
		t.Fatalf("synthetic accepted history is not replayable: %v", err)
	}
	return head
}
func TestFoodFlowJournalStoppedZeroHasNoPostmortemBasal(t *testing.T) {
	f := foodFlowJournalUntil(t, 0, 11, false)
	batches := f.Journal()
	if f.Checkpoints()[11].Alive != 0 {
		t.Fatal("fixture did not stop at h11")
	}
	for hour := 12; hour <= FoodFlowHorizonHours; hour++ {
		batches = append(batches, foodFlowJournalPulse(t, f.k, hour, nil))
	}
	head := foodFlowJournalValidHistory(t, f)
	state := f.k.SnapshotHead()
	_, _, after, err := foodFlowStates(scheduler.SnapshotView{Reader: state.Reader, Authority: state.Authority, Version: state.Version})
	if err != nil || after != f.Checkpoints()[11].Actors {
		t.Fatalf("postmortem actor state changed: %v", err)
	}
	var basalAfterDeath int
	for _, ev := range f.k.Events() {
		if ev.Rule == FoodFlowBasalRule && ev.Time > 11*sim.SimTime(FoodFlowHour) {
			basalAfterDeath++
		}
	}
	if basalAfterDeath != 0 {
		t.Fatalf("%d postmortem events", basalAfterDeath)
	}
	encoded, err := EncodeFoodFlowJournal(batches, f.k, head, 168*sim.SimTime(FoodFlowHour), f.journalConfig())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeFoodFlowJournal(encoded, f.k, head, 168*sim.SimTime(FoodFlowHour), f.journalConfig())
	if err != nil || len(decoded) != len(batches) {
		t.Fatalf("q0 no-postmortem continuation: %v", err)
	}
	for _, b := range decoded {
		if b.Time >= 12*sim.SimTime(FoodFlowHour) && len(b.Attempts) != 0 {
			t.Fatal("dead actor attempted action")
		}
	}
	t.Logf("q0 no-postmortem h12..168: %d replayed events, %d evidence batches", head.TipID, len(decoded))
}
func TestFoodFlowJournalStoppedScarceAndForgedBasalLinks(t *testing.T) {
	f := foodFlowJournalUntil(t, 3, 17, true)
	var alive, dead []sim.EntityID
	for i, a := range f.Checkpoints()[17].Actors {
		id := sim.EntityID(i + 1)
		if a.Energy > 0 {
			alive = append(alive, id)
		} else {
			dead = append(dead, id)
		}
	}
	if len(alive) != 6 || len(dead) != 10 {
		t.Fatalf("not a partial extinction: alive=%v dead=%v", alive, dead)
	}
	prefix, _, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	newKernel := func() *kernel.Kernel {
		t.Helper()
		k, _, e := kernel.RestoreHistory(f.registry, prefix)
		if e != nil {
			t.Fatal(e)
		}
		return k
	}
	cases := []struct {
		name   string
		actors []sim.EntityID
		valid  bool
	}{
		{"six survivors", alive, true},
		{"missing-live basal", append([]sim.EntityID(nil), alive[1:]...), false},
		{"extra-dead basal", append(append([]sim.EntityID(nil), alive...), dead[0]), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := newKernel()
			batches := append(cloneFoodFlowJournal(f.Journal()), foodFlowJournalPulse(t, k, 18, tc.actors))
			wire, head, err := k.ExportHistory()
			if err != nil {
				t.Fatal(err)
			}
			_, replayed, err := kernel.RestoreHistory(f.registry, wire)
			if err != nil || replayed != head {
				t.Fatalf("forged accepted history not valid: %v", err)
			}
			_, err = EncodeFoodFlowJournal(batches, k, head, 18*sim.SimTime(FoodFlowHour), f.journalConfig())
			if tc.valid && err != nil {
				t.Fatalf("six survivors rejected: %v", err)
			}
			if !tc.valid && !errors.Is(err, ErrFoodFlowJournal) {
				t.Fatalf("forged %s accepted: %v", tc.name, err)
			}
			if tc.valid {
				state := k.SnapshotHead()
				_, _, actors, err := foodFlowStates(scheduler.SnapshotView{Reader: state.Reader, Authority: state.Authority, Version: state.Version})
				if err != nil {
					t.Fatal(err)
				}
				for _, actor := range dead {
					if actors[actor-1] != f.Checkpoints()[17].Actors[actor-1] {
						t.Fatalf("actor %d changed after stopping", actor)
					}
				}
				t.Logf("q3 h18: %d surviving basal events, %d stopped actors unchanged", len(alive), len(dead))
			}
		})
	}
}
