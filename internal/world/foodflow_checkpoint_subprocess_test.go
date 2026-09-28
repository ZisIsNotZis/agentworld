//go:build !race

package world

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// Cross-process full-horizon bytes are exercised in the normal suite; race
// coverage of publication/restore and short continuation runs separately.
func TestFoodFlowCheckpointSubprocess(t *testing.T) {
	if path := os.Getenv("FOODFLOW_RESTORE_CHILD"); path != "" {
		q, err := strconv.ParseInt(os.Getenv("FOODFLOW_Q"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		f, restoredDigest, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: q, Seed: 7, Workers: 1})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		h, err := f.Handoff()
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("FOODFLOW_ASSERT_SCARCE_FROZEN") == "1" {
			assertFoodFlowScarceDeathsFrozen(t, h.Checkpoints)
		}
		finalDigest, err := f.SaveCheckpoint(os.Getenv("FOODFLOW_FINAL_CHECKPOINT"))
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(os.Getenv("FOODFLOW_OUTPUT"))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		for _, piece := range [][]byte{h.History, h.SchedulerBytes} {
			if _, err = out.Write(piece); err != nil {
				t.Fatal(err)
			}
		}
		j, err := f.ExportJournal()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write(j); err != nil {
			t.Fatal(err)
		}
		if _, err = fmt.Fprintf(out, "%#v", h.Checkpoints); err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write(restoredDigest[:]); err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write(finalDigest[:]); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, tc := range []struct {
		name        string
		q           int64
		afterDeaths bool
	}{{"q8-active", 8, false}, {"q3-active", 3, false}, {"q0-rejected", 0, false}, {"q3-h18-mixed", 3, true}} {
		t.Run(tc.name, func(t *testing.T) {
			var original *FoodFlow
			var path string
			var savedDigest [32]byte
			if tc.afterDeaths {
				original, path, savedDigest = foodFlowSaveScarceAfterDeaths(t)
			} else {
				original, path, savedDigest = foodFlowSaveAt(t, tc.q, 2)
			}
			out := filepath.Join(t.TempDir(), "child.out")
			childFinal := filepath.Join(t.TempDir(), "child-final.bundle")
			cmd := exec.Command(os.Args[0], "-test.run=^TestFoodFlowCheckpointSubprocess$")
			frozen := "0"
			if tc.afterDeaths {
				frozen = "1"
			}
			cmd.Env = append(os.Environ(), "FOODFLOW_RESTORE_CHILD="+path, "FOODFLOW_Q="+strconv.FormatInt(tc.q, 10), "FOODFLOW_OUTPUT="+out, "FOODFLOW_FINAL_CHECKPOINT="+childFinal, "FOODFLOW_ASSERT_SCARCE_FROZEN="+frozen)
			if result, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("child: %v %s", err, result)
			}
			if err := original.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			h, err := original.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			if tc.afterDeaths {
				assertFoodFlowScarceDeathsFrozen(t, h.Checkpoints)
			}
			journal, err := original.ExportJournal()
			if err != nil {
				t.Fatal(err)
			}
			finalDigest, err := original.SaveCheckpoint(filepath.Join(t.TempDir(), "uninterrupted-final.bundle"))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			want := append(append(append([]byte(nil), h.History...), h.SchedulerBytes...), journal...)
			want = append(want, fmt.Sprintf("%#v", h.Checkpoints)...)
			want = append(want, savedDigest[:]...)
			want = append(want, finalDigest[:]...)
			if h.Checkpoints[168].Alive != map[int64]int{8: 16, 3: 6, 0: 0}[tc.q] {
				t.Fatalf("q%d unexpected horizon alive=%d", tc.q, h.Checkpoints[168].Alive)
			}
			if !bytes.Equal(actual, want) {
				t.Fatalf("subprocess continuation bytes differ: got %x want %x", sha256.Sum256(actual), sha256.Sum256(want))
			}
			if tc.afterDeaths {
				t.Logf("q3 restored after h17 deaths: h18 alive=%d h168 alive=%d accepted events=%d final digest=%x", h.Checkpoints[18].Alive, h.Checkpoints[168].Alive, len(original.k.Events()), finalDigest)
			}
		})
	}
}
