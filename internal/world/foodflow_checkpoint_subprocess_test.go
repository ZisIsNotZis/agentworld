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
		f, _, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: q, Seed: 7, Workers: 1})
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
		return
	}
	for _, q := range []int64{8, 3, 0} {
		t.Run(strconv.FormatInt(q, 10), func(t *testing.T) {
			original, path, _ := foodFlowSaveAt(t, q, 2)
			out := filepath.Join(t.TempDir(), "child.out")
			cmd := exec.Command(os.Args[0], "-test.run=^TestFoodFlowCheckpointSubprocess$")
			cmd.Env = append(os.Environ(), "FOODFLOW_RESTORE_CHILD="+path, "FOODFLOW_Q="+strconv.FormatInt(q, 10), "FOODFLOW_OUTPUT="+out)
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
			journal, err := original.ExportJournal()
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			want := append(append(append([]byte(nil), h.History...), h.SchedulerBytes...), journal...)
			want = append(want, fmt.Sprintf("%#v", h.Checkpoints)...)
			if h.Checkpoints[168].Alive != map[int64]int{8: 16, 3: 6, 0: 0}[q] {
				t.Fatalf("q%d unexpected horizon alive=%d", q, h.Checkpoints[168].Alive)
			}
			if !bytes.Equal(actual, want) {
				t.Fatalf("subprocess continuation bytes differ: got %x want %x", sha256.Sum256(actual), sha256.Sum256(want))
			}
		})
	}
}
