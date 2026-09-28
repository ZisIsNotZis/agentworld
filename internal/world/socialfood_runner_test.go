package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"bytes"
	"context"
	"reflect"
	"testing"
)

func socialFoodRunTo(t *testing.T, options SocialFoodOptions, hour int) *SocialFood {
	t.Helper()
	f, err := NewSocialFood(options)
	if err != nil {
		t.Fatal(err)
	}
	for len(f.Checkpoints()) <= hour {
		ok, e := f.Step(context.Background())
		if e != nil || !ok {
			t.Fatalf("hour %d step %d: processed=%v err=%v", hour, f.steps, ok, e)
		}
	}
	return f
}
func socialFoodEventBytes(t *testing.T, f *SocialFood) [][]byte {
	t.Helper()
	events := f.k.Events()
	out := make([][]byte, len(events))
	for i, ev := range events {
		b, e := ev.Bytes()
		if e != nil {
			t.Fatal(e)
		}
		out[i] = b
	}
	return out
}
func TestSocialFoodRunnerFrozenControlsAndWorkerEvidence(t *testing.T) {
	for _, tc := range []struct {
		q    int64
		hour int
	}{{3, 24}, {8, 24}, {0, 12}} {
		for _, seed := range []uint64{0, 7} {
			opts := SocialFoodOptions{Yield: tc.q, Seed: seed, Workers: 1, Enabled: true}
			one := socialFoodRunTo(t, opts, tc.hour)
			opts.Workers = 4
			four := socialFoodRunTo(t, opts, tc.hour)
			if !reflect.DeepEqual(socialFoodEventBytes(t, one), socialFoodEventBytes(t, four)) || !reflect.DeepEqual(one.Journal(), four.Journal()) || !reflect.DeepEqual(one.Checkpoints(), four.Checkpoints()) {
				t.Fatalf("q%d seed%d worker-dependent evidence", tc.q, seed)
			}
			history, _, err := four.k.ExportHistory()
			if err != nil {
				t.Fatal(err)
			}
			restored, _, err := kernel.RestoreHistory(four.registry, history)
			if err != nil {
				t.Fatal(err)
			}
			original := socialFoodEventBytes(t, four)
			for i, ev := range restored.Events() {
				b, e := ev.Bytes()
				if e != nil || !bytes.Equal(b, original[i]) {
					t.Fatalf("q%d seed%d replay event%d: %v", tc.q, seed, i, e)
				}
			}
			events := make(map[sim.EventID]kernel.Event)
			for _, ev := range four.k.Events() {
				events[ev.ID] = ev
			}
			counts := map[string]int{}
			requestsPerHour := map[[2]int64]bool{}
			repliesPerHour := map[[2]int64]bool{}
			for _, batch := range four.Journal() {
				for _, a := range batch.Attempts {
					counts[a.Kind]++
					if a.Key == "" {
						if a.EventID != 0 || a.Accepted {
							t.Fatalf("uncommitted attempt %+v", a)
						}
						continue
					}
					ev, ok := events[a.EventID]
					if !ok || !a.Accepted || ev.Key != a.Key || ev.Time != a.Time {
						t.Fatalf("broken event link %+v", a)
					}
					if ev.Cause.Actor != a.Actor || ev.Rule == 0 {
						t.Fatalf("broken cause/rule link %+v", a)
					}
					hour := int64(a.Time / sim.SimTime(SocialFoodHour))
					if a.Kind == "request" {
						if a.Claim != 2 || a.Truth <= 0 || a.Target == 0 || a.RequestID != hour*16+int64(a.Actor) || requestsPerHour[[2]int64{int64(a.Actor), hour}] {
							t.Fatalf("invalid or duplicate directed request %+v", a)
						}
						requestsPerHour[[2]int64{int64(a.Actor), hour}] = true
					}
					if a.Kind == "gift" || a.Kind == "refuse" {
						if repliesPerHour[[2]int64{int64(a.Actor), hour}] || a.RequestID != hour*16+int64(a.Target) {
							t.Fatalf("invalid or duplicate donor reply %+v", a)
						}
						repliesPerHour[[2]int64{int64(a.Actor), hour}] = true
					}
				}
			}
			for _, c := range four.Checkpoints() {
				if c.Balance.Produced != c.Balance.Consumed+c.Balance.Held+c.Balance.Stock || c.Balance.InitialEnergy+c.Balance.Consumed != c.Balance.Energy+c.Balance.BasalSpent+c.Balance.CapLost {
					t.Fatalf("q%d seed%d h%d balance %+v", tc.q, seed, c.Hour, c.Balance)
				}
			}
			t.Logf("q%d seed%d h%d alive=%d actor1=%+v actor8=%+v actor16=%+v requests=%d gifts=%d refusals=%d", tc.q, seed, tc.hour, four.Checkpoints()[tc.hour].Alive, four.Checkpoints()[tc.hour].Actors[0], four.Checkpoints()[tc.hour].Actors[7], four.Checkpoints()[tc.hour].Actors[15], counts["request"], counts["gift"], counts["refuse"])
		}
	}
}
func TestSocialFoodRunnerOpportunityAndEnabledBranch(t *testing.T) {
	enabled := socialFoodRunTo(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4, Enabled: true}, 24)
	disabled := socialFoodRunTo(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4}, 24)
	gathered, denied := false, false
	for _, b := range enabled.Journal() {
		for _, a := range b.Attempts {
			if a.Kind == "gather" && a.Time < sim.SimTime(SocialFoodHour) {
				if a.Actor == 1 && a.Accepted {
					gathered = true
				}
				if a.Actor == 8 && !a.Accepted {
					denied = true
				}
			}
		}
	}
	if !gathered || !denied {
		t.Fatal("missing seed0 h0 8-to-1 opportunity")
	}
	if enabled.Checkpoints()[0] != disabled.Checkpoints()[0] {
		t.Fatal("non-neutral h0 pulse")
	}
	for _, b := range disabled.Journal() {
		for _, a := range b.Attempts {
			if a.Kind == "request" || a.Kind == "gift" || a.Kind == "refuse" {
				t.Fatalf("disabled branch transferred: %+v", a)
			}
		}
	}
	t.Logf("enabled h24 alive=%d disabled=%d (no improvement required)", enabled.Checkpoints()[24].Alive, disabled.Checkpoints()[24].Alive)
}
