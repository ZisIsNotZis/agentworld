package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Build an actually restorable generic scheduler and re-export its canonical
// portable bytes. A corrupt checksum or invalid generic snapshot would not
// exercise the pilot-specific semantic validation.
func foodFlowAlterPortable(t *testing.T, f *FoodFlow, change func(*scheduler.Snapshot)) (*scheduler.Scheduler, []byte) {
	t.Helper()
	snap := f.sched.Snapshot()
	change(&snap)
	for i := range snap.Fibers {
		fiber := &snap.Fibers[i]
		fiber.HasNextWake, fiber.NextWake = false, 0
		for _, wake := range snap.Wakes {
			if wake.Actor == fiber.Actor && (!fiber.HasNextWake || wake.At < fiber.NextWake) {
				fiber.HasNextWake, fiber.NextWake = true, wake.At
			}
		}
	}
	changed, err := scheduler.Restore(f.k, 1, f.evaluate, snap)
	if err != nil {
		t.Fatalf("generic scheduler rejected test state: %v", err)
	}
	wire, head, err := changed.ExportPortable()
	if err != nil {
		t.Fatalf("generic portable export: %v", err)
	}
	_, originalHead, err := f.k.ExportHistory()
	if err != nil || head != originalHead {
		t.Fatalf("generic portable head mismatch: %v", err)
	}
	return changed, wire
}

func foodFlowSaveHorizon(t *testing.T, q int64) (*FoodFlow, string) {
	t.Helper()
	f, err := NewFoodFlow(FoodFlowOptions{Yield: q, Seed: 7, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "horizon.bundle")
	if _, err := f.SaveCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	return f, path
}

func TestFoodFlowCheckpointHorizonNoWakes(t *testing.T) {
	f, path := foodFlowSaveHorizon(t, 0)
	snap := f.sched.Snapshot()
	if snap.Time != sim.SimTime(FoodFlowHorizonHours)*sim.SimTime(FoodFlowHour) || len(snap.Wakes) != 0 || f.checkpoints[len(f.checkpoints)-1].Hour != FoodFlowHorizonHours {
		t.Fatalf("not an h168 empty-queue boundary: %+v", snap)
	}
	before, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	restored, digest, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: 0, Seed: 7, Workers: 1})
	if err != nil || digest == [32]byte{} || restored == nil {
		t.Fatalf("h168 restore: %x %v", digest, err)
	}
	after, err := restored.Handoff()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("h168 handoff changed: %v", err)
	}
	processed, err := restored.Step(context.Background())
	if processed || err != nil {
		t.Fatalf("h168 continuation was not terminal: processed=%t err=%v", processed, err)
	}
}

func TestFoodFlowCheckpointRejectsValidPortableWrongWakeTopology(t *testing.T) {
	for _, tc := range []struct {
		name   string
		q      int64
		steps  int
		change func(*scheduler.Snapshot)
	}{
		{"missing genesis audit", 3, 0, func(s *scheduler.Snapshot) { s.Wakes = nil }},
		{"extra audit after claim", 3, 2, func(s *scheduler.Snapshot) {
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 1001, At: 2, Cause: scheduler.WakeAudit})
		}},
		{"extra audit after h168", 0, -2, func(s *scheduler.Snapshot) {
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 1001, At: sim.SimTime(FoodFlowHorizonHours)*sim.SimTime(FoodFlowHour) + 1, Cause: scheduler.WakeAudit})
		}},
		{"extra claim beside active Gather completion", 3, 2, func(s *scheduler.Snapshot) {
			for _, wake := range s.Wakes {
				if wake.Actor == 1 && wake.Cause == scheduler.WakeCompletion {
					s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 1, At: wake.At, Cause: scheduler.WakeNeedThreshold})
					return
				}
			}
			t.Fatal("no active Gather completion")
		}},
		{"wrong claim wake time", 3, 1, func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == 1 {
					s.Wakes[i].At = 2
					return
				}
			}
			t.Fatal("no claim wake")
		}},
		{"live actor marked stopped", 0, 2, func(s *scheduler.Snapshot) { s.Fibers[0].Lifecycle = scheduler.Stopped; s.Fibers[0].Revision++ }},
		{"stopped actor marked alive", 0, -1, func(s *scheduler.Snapshot) {
			s.Fibers[0].Lifecycle = scheduler.Alive
			s.Fibers[0].Revision++
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f *FoodFlow
			var path string
			if tc.steps == -2 {
				f, path = foodFlowSaveHorizon(t, tc.q)
			} else if tc.steps == -1 {
				var err error
				f, err = NewFoodFlow(FoodFlowOptions{Yield: 0, Seed: 7, Workers: 4})
				if err != nil {
					t.Fatal(err)
				}
				for len(f.checkpoints) <= 11 {
					processed, err := f.Step(context.Background())
					if err != nil || !processed {
						t.Fatalf("extinction pulse: %v", err)
					}
				}
				path = filepath.Join(t.TempDir(), "original.bundle")
				if _, err := f.SaveCheckpoint(path); err != nil {
					t.Fatal(err)
				}
			} else {
				f, path, _ = foodFlowSaveAt(t, tc.q, tc.steps)
			}
			changed, wire := foodFlowAlterPortable(t, f, tc.change)
			sections, _, err := checkpoint.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			_, head, err := f.k.ExportHistory()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeFoodFlowJournalWithScheduler(sections[0].Data, f.k, head, changed, f.journalConfig()); err != nil {
				t.Fatalf("test state was already rejected by generic/journal validator: %v", err)
			}
			if err := verifyFoodFlowCheckpointScheduler(f.k, f.journal, changed.Snapshot()); !errors.Is(err, ErrFoodFlowCheckpoint) {
				t.Fatalf("pilot topology validator did not reject test state: %v", err)
			}
			manifest, err := decodeFoodFlowManifest(sections[2].Data)
			if err != nil {
				t.Fatal(err)
			}
			manifest.sections[2] = sha256.Sum256(wire)
			changedSections := append([]checkpoint.Section(nil), sections...)
			changedSections[2].Data = encodeFoodFlowManifest(manifest)
			changedSections[3].Data = wire
			forged := filepath.Join(t.TempDir(), "validly-reencoded.bundle")
			if _, err := checkpoint.WriteFile(forged, changedSections); err != nil {
				t.Fatal(err)
			}
			if restored, digest, err := RestoreFoodFlowCheckpoint(forged, FoodFlowOptions{Yield: tc.q, Seed: 7, Workers: 1}); restored != nil || digest != [32]byte{} || !errors.Is(err, ErrFoodFlowCheckpoint) {
				t.Fatalf("accepted invalid topology: runner=%v digest=%x err=%v", restored, digest, err)
			}
			f.sched = changed
			absentDir := t.TempDir()
			if digest, err := f.SaveCheckpoint(filepath.Join(absentDir, "unpublished.bundle")); digest != [32]byte{} || !errors.Is(err, ErrFoodFlowCheckpoint) {
				t.Fatalf("published invalid topology: digest=%x err=%v", digest, err)
			}
			entries, err := os.ReadDir(absentDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("partial publication: entries=%v err=%v", entries, err)
			}
		})
	}
}

// No completion activity is required to ensure a later pulse remains queued:
// the zero-inflow fixture has only audit wakes after the claim window closes.
func TestFoodFlowCheckpointRejectedOnlyAuditChain(t *testing.T) {
	f, path, _ := foodFlowSaveAt(t, 0, 2)
	if len(f.sched.Snapshot().Wakes) != 1 || f.sched.Snapshot().Wakes[0] != (scheduler.Wake{Actor: 1001, At: sim.SimTime(FoodFlowHour), Cause: scheduler.WakeAudit}) {
		t.Fatal("unexpected rejected-only wake topology")
	}
	if _, _, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: 0, Seed: 7, Workers: 1}); err != nil {
		t.Fatal(err)
	}
}
