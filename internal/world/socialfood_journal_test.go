package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"
)

func socialFoodJournalFixture(t *testing.T, options SocialFoodOptions, hour int) (*SocialFood, []byte, kernel.PortableHead) {
	t.Helper()
	f := socialFoodRunTo(t, options, hour)
	data, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return f, data, head
}

func socialFoodJournalRewrite(t *testing.T, data []byte, edit func(*socialFoodJournalWire)) []byte {
	t.Helper()
	var wire socialFoodJournalWire
	if err := json.Unmarshal(data[12:len(data)-sha256.Size], &wire); err != nil {
		t.Fatal(err)
	}
	edit(&wire)
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), data[:8]...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func TestSocialFoodJournalRoundTripAndSchedulerBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts SocialFoodOptions
		hour int
	}{
		{"q3-consent", SocialFoodOptions{Yield: 3, Seed: 0, Enabled: true, Workers: 4}, 24},
		{"q8", SocialFoodOptions{Yield: 8, Seed: 7, Enabled: true, Workers: 1}, 8},
		{"q0", SocialFoodOptions{Yield: 0, Seed: 0, Enabled: true, Workers: 1}, 12},
		{"disabled", SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}, 8},
		{"reported-claim", SocialFoodOptions{Yield: 3, Seed: 0, Enabled: true, Workers: 1, TestClaimOverrides: map[sim.EntityID]int64{8: 0}}, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, data, head := socialFoodJournalFixture(t, tc.opts, tc.hour)
			config := f.journalConfig()
			decoded, err := DecodeSocialFoodJournalWithScheduler(data, f.k, head, f.sched, config)
			if err != nil || !reflect.DeepEqual(decoded, f.Journal()) {
				t.Fatalf("scheduler-aware roundtrip: %v", err)
			}
			other, err := f.ExportJournal()
			if err != nil || !bytes.Equal(other, data) {
				t.Fatalf("noncanonical export: %v", err)
			}
			handoff, err := f.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			k, restoredHead, err := kernel.RestoreHistory(f.registry, handoff.History)
			if err != nil || restoredHead != head {
				t.Fatalf("history restore: %v", err)
			}
			s, err := scheduler.RestorePortable(k, 1, f.evaluate, handoff.SchedulerBytes, head)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeSocialFoodJournalWithScheduler(data, k, head, s, config); err != nil {
				t.Fatalf("portable restored scheduler: %v", err)
			}
			if err := f.RestoreJournal(data, head); err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeSocialFoodJournal(data, k, head, s.Time(), config); err != nil {
				t.Fatalf("history-only decoder: %v", err)
			}
			var wire socialFoodJournalWire
			if err := json.Unmarshal(data[12:len(data)-sha256.Size], &wire); err != nil {
				t.Fatal(err)
			}
			if tc.name == "q3-consent" && (wire.Counts.Gifts == 0 || wire.Counts.Refusals == 0 || wire.Counts.Expiries == 0 || wire.Counts.GatherDenied == 0 || wire.Counts.Eats == 0) {
				t.Fatalf("missing full typed outcomes: %+v", wire.Counts)
			}
			if tc.name == "reported-claim" {
				seen := false
				for _, b := range decoded {
					for _, a := range b.Attempts {
						if a.Actor == 8 && a.Kind == "request" && a.Claim == 0 && a.Truth != a.Claim {
							seen = true
						}
					}
				}
				if !seen {
					t.Fatal("lost separate reported claim/privileged actual reserve")
				}
			}
		})
	}
}

func TestSocialFoodJournalRejectsRehashedConsentForgeryAndConfig(t *testing.T) {
	f, data, head := socialFoodJournalFixture(t, SocialFoodOptions{Yield: 3, Seed: 0, Enabled: true, Workers: 1}, 9)
	config := f.journalConfig()
	cases := map[string]func(*socialFoodJournalWire){
		"claims": func(w *socialFoodJournalWire) {
			w.Config.TestClaimOverrides = map[sim.EntityID]int64{1: 2}
		},
		"duplicate-consent": func(w *socialFoodJournalWire) {
			for i := range w.Batches {
				for _, a := range w.Batches[i].Attempts {
					if a.Kind == "gift" {
						w.Batches[i].Attempts = append(w.Batches[i].Attempts, a)
						return
					}
				}
			}
		},
		"foreign-donor": func(w *socialFoodJournalWire) {
			for i := range w.Batches {
				for j := range w.Batches[i].Attempts {
					if w.Batches[i].Attempts[j].Kind == "gift" {
						w.Batches[i].Attempts[j].Actor = 13
						return
					}
				}
			}
		},
		"cross-patch": func(w *socialFoodJournalWire) {
			for i := range w.Batches {
				for j := range w.Batches[i].Attempts {
					if w.Batches[i].Attempts[j].Kind == "gift" {
						w.Batches[i].Attempts[j].Target = 1
						return
					}
				}
			}
		},
		"late-request": func(w *socialFoodJournalWire) {
			for i := range w.Batches {
				for j := range w.Batches[i].Attempts {
					if w.Batches[i].Attempts[j].Kind == "gift" {
						w.Batches[i].Attempts[j].RequestID -= 16
						return
					}
				}
			}
		},
		"request-is-not-consent": func(w *socialFoodJournalWire) {
			for i := range w.Batches {
				for j := range w.Batches[i].Attempts {
					if w.Batches[i].Attempts[j].Kind == "refuse" {
						w.Batches[i].Attempts[j].Kind = "gift"
						w.Batches[i].Attempts[j].Score = 3
						return
					}
				}
			}
		},
		"private-truth": func(w *socialFoodJournalWire) {
			for i := range w.Batches {
				for j := range w.Batches[i].Attempts {
					if w.Batches[i].Attempts[j].Kind == "request" {
						w.Batches[i].Attempts[j].Truth++
						return
					}
				}
			}
		},
		"head":    func(w *socialFoodJournalWire) { w.Head.TipHash[0]++ },
		"seed":    func(w *socialFoodJournalWire) { w.Config.Seed++ },
		"policy":  func(w *socialFoodJournalWire) { w.Config.Policy.Version++ },
		"enabled": func(w *socialFoodJournalWire) { w.Config.Enabled = false },
		"q":       func(w *socialFoodJournalWire) { w.Config.Yield++ },
		"counts":  func(w *socialFoodJournalWire) { w.Counts.Expiries++ },
	}
	original := f.Journal()
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			corrupt := socialFoodJournalRewrite(t, data, mutate)
			if _, err := DecodeSocialFoodJournal(corrupt, f.k, head, f.sched.Time(), config); err == nil {
				t.Fatal("accepted rehashed counterfeit")
			}
			if err := f.RestoreJournal(corrupt, head); err == nil || !reflect.DeepEqual(f.Journal(), original) {
				t.Fatal("failed restore replaced committed journal")
			}
		})
	}
	corrupt := append([]byte(nil), data...)
	corrupt[20] ^= 1
	if _, err := DecodeSocialFoodJournal(corrupt, f.k, head, f.sched.Time(), config); err == nil {
		t.Fatal("accepted corrupted digest")
	}
	if _, err := DecodeSocialFoodJournal(data[:len(data)-1], f.k, head, f.sched.Time(), config); err == nil {
		t.Fatal("accepted truncated digest")
	}
	duplicate := bytes.Replace(data[12:len(data)-sha256.Size], []byte(`"Format":1,`), []byte(`"Format":1,"Format":1,`), 1)
	if bytes.Equal(duplicate, data[12:len(data)-sha256.Size]) {
		t.Fatal("missing format field in fixture")
	}
	noncanonical := append([]byte(nil), data[:8]...)
	noncanonical = binary.BigEndian.AppendUint32(noncanonical, uint32(len(duplicate)))
	noncanonical = append(noncanonical, duplicate...)
	digest := sha256.Sum256(noncanonical)
	noncanonical = append(noncanonical, digest[:]...)
	if _, err := DecodeSocialFoodJournal(noncanonical, f.k, head, f.sched.Time(), config); err == nil {
		t.Fatal("accepted duplicate JSON field with valid digest")
	}
	if _, err := DecodeSocialFoodJournal(make([]byte, MaxSocialFoodJournalBytes+1), f.k, head, f.sched.Time(), config); err == nil {
		t.Fatal("accepted oversized journal")
	}
}

func TestSocialFoodJournalRejectsLateExpiryOrConsent(t *testing.T) {
	f, data, head := socialFoodJournalFixture(t, SocialFoodOptions{Yield: 3, Seed: 0, Enabled: true, Workers: 1}, 24)
	found := false
	for _, b := range f.Journal() {
		for _, a := range b.Attempts {
			found = found || a.Kind == "expire"
		}
	}
	if !found {
		t.Fatal("fixture has no expiry")
	}
	for _, kind := range []string{"late-expiry", "expired-is-not-consent"} {
		t.Run(kind, func(t *testing.T) {
			forged := socialFoodJournalRewrite(t, data, func(w *socialFoodJournalWire) {
				for i := range w.Batches {
					for j := range w.Batches[i].Attempts {
						a := &w.Batches[i].Attempts[j]
						if a.Kind == "expire" {
							if kind == "late-expiry" {
								a.Time += sim.SimTime(SocialFoodHour)
							} else {
								a.Kind = "gift"
							}
							return
						}
					}
				}
			})
			if _, err := DecodeSocialFoodJournal(forged, f.k, head, f.sched.Time(), f.journalConfig()); err == nil {
				t.Fatal("accepted expired or late consent")
			}
		})
	}
}

func TestSocialFoodJournalRejectsRawKernelStatusOnlyGift(t *testing.T) {
	f, data, _ := socialFoodJournalFixture(t, SocialFoodOptions{Yield: 3, Enabled: true, Workers: 1}, 0)
	// A schema-valid, single-field raw gift can be admitted by the kernel
	// without a pending request, donor, bag, or recipient consent.
	at := f.sched.Time() + 1
	_, authority, _ := f.k.Snapshot()
	proposal := socialFoodProposal("raw-gift", at, 1, SocialFoodGiftRule,
		socialFoodNumber(1, SocialFoodRequestTypeID, SocialFoodRequestStatusField, int64(SocialFoodAccepted)))
	plan, err := f.k.Plan(proposal, authority)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.k.CommitBatch([]kernel.Plan{plan}); err != nil {
		t.Fatal(err)
	}
	_, forgedHead, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	forged := socialFoodJournalRewrite(t, data, func(w *socialFoodJournalWire) {
		w.Head = forgedHead
		w.Batches[len(w.Batches)-1].Version = forgedHead.Version
		w.Batches[len(w.Batches)-1].TipID = forgedHead.TipID
		w.Batches[len(w.Batches)-1].TipHash = forgedHead.TipHash
	})
	if _, err := DecodeSocialFoodJournal(forged, f.k, forgedHead, f.sched.Time(), f.journalConfig()); err == nil {
		t.Fatal("schema-valid raw gift passed typed history partition")
	}
}

func TestSocialFoodJournalPartialVersusSchedulerTopology(t *testing.T) {
	f, data, head := socialFoodJournalFixture(t, SocialFoodOptions{Yield: 3, Enabled: true, Workers: 1}, 0)
	// The same verified history/time can accompany a distinct, otherwise
	// valid scheduler: add a future audit wake to an independently restored
	// coordinator without modifying the kernel or the journal.
	other, err := scheduler.Restore(f.k, 1, f.evaluate, f.sched.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Schedule(scheduler.Wake{Actor: 1, At: sim.SimTime(SocialFoodHour) + 10, Cause: scheduler.WakeAudit}); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSocialFoodJournal(data, f.k, head, other.Time(), f.journalConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSocialFoodJournalWithScheduler(data, f.k, head, other, f.journalConfig()); err == nil {
		t.Fatal("accepted unmatched wake at same verified head")
	}
	trusted := f.sched
	f.sched = other
	if _, err := f.ExportJournal(); err == nil {
		t.Fatal("exported an unmatched wake")
	}
	if err := f.RestoreJournal(data, head); err == nil {
		t.Fatal("restored journal against an unmatched wake")
	}
	// Keep the trusted scheduler for subsequent continuation.
	f.sched = trusted
	if ok, err := f.Step(context.Background()); !ok || err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSocialFoodJournal(data, f.k, head, f.sched.Time(), f.journalConfig()); err == nil {
		t.Fatal("accepted mismatched head/time")
	}
}
