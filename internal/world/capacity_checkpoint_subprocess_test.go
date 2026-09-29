//go:build !race

package world

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// Cross-process restore must continue with identical accepted event bytes,
// journal, scheduler bytes and projections — at worker counts different from
// the capturing process. Flag equivalence across worker counts is exercised
// in the normal suite; this test proves the bundle survives a real process
// boundary and that a restored runner reaches the full frozen horizon.
func TestCapacityCheckpointSubprocess(t *testing.T) {
	const (
		subQ    = int64(3)
		subSeed = uint64(0)
		subHour = 12
	)
	rootOpts := CapacityOptions{Yield: subQ, Seed: subSeed, Workers: 4}
	branchOpts := rootOpts
	branchOpts.Enabled = true
	horizonOpts := CapacityOptions{Yield: 8, Seed: subSeed, Workers: 2, Enabled: true}
	if out := os.Getenv("CAPACITY_RESTORE_CHILD"); out != "" {
		q, err := strconv.ParseInt(os.Getenv("CAPACITY_Q"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seed, err := strconv.ParseUint(os.Getenv("CAPACITY_SEED"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		for _, tc := range []struct {
			path    string
			enabled bool
		}{{os.Getenv("CAPACITY_ROOT"), false}, {os.Getenv("CAPACITY_BRANCH"), true}} {
			// The child continues at a worker count the capturing process
			// never used (capture: 4, continuation: 1).
			runner, digest, err := RestoreCapacityCheckpoint(tc.path, CapacityOptions{Yield: q, Seed: seed, Workers: 1, Enabled: tc.enabled})
			if err != nil {
				t.Fatal(err)
			}
			capacityStepToHour(t, runner, subHour)
			h, err := runner.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			j, err := runner.ExportJournal()
			if err != nil {
				t.Fatal(err)
			}
			if h.Enabled != tc.enabled {
				t.Fatalf("child restored flag %v want %v", h.Enabled, tc.enabled)
			}
			buf.Write(h.History)
			buf.Write(h.SchedulerBytes)
			buf.Write(j)
			fmt.Fprintf(&buf, "%#v", runner.Checkpoints())
			buf.Write(digest[:])
		}
		if err := os.WriteFile(out, buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		// A fresh process must also carry a restored runner to the full frozen
		// horizon through the runner's own tail checks.
		horizon, _, err := RestoreCapacityCheckpoint(os.Getenv("CAPACITY_HORIZON"), horizonOpts)
		if err != nil {
			t.Fatal(err)
		}
		if err := horizon.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		checks := horizon.Checkpoints()
		if len(checks) != CapacityHorizonHours+1 || checks[CapacityHorizonHours].Hour != CapacityHorizonHours || checks[CapacityHorizonHours].Alive != CapacityActorCount {
			t.Fatalf("restored horizon run incomplete: len=%d alive=%d", len(checks), checks[CapacityHorizonHours].Alive)
		}
		if _, err := horizon.ExportJournal(); err != nil {
			t.Fatalf("restored horizon journal: %v", err)
		}
		return
	}
	_, rootPath, _ := capacityCaptureNeutral(t, rootOpts)
	branchPath := filepath.Join(t.TempDir(), "subprocess-branch.bundle")
	if _, err := BranchCapacityCheckpoint(rootPath, rootOpts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	horizonPath := filepath.Join(t.TempDir(), "subprocess-horizon.bundle")
	// Publish the q8-enabled neutral root for the child's horizon run.
	if _, err := mustCapacityAtH0(t, horizonOpts).SaveCheckpoint(horizonPath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "child.out")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCapacityCheckpointSubprocess$")
	cmd.Env = append(os.Environ(),
		"CAPACITY_RESTORE_CHILD="+out,
		"CAPACITY_ROOT="+rootPath,
		"CAPACITY_BRANCH="+branchPath,
		"CAPACITY_HORIZON="+horizonPath,
		"CAPACITY_Q="+strconv.FormatInt(subQ, 10),
		"CAPACITY_SEED="+strconv.FormatUint(subSeed, 10))
	if result, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v %s", err, result)
	}
	actual, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// The parent repeats the same restores in-process and against fresh
	// same-flag baselines; every byte must agree.
	var want bytes.Buffer
	restored := map[bool]*Capacity{}
	for _, tc := range []struct {
		path    string
		enabled bool
	}{{rootPath, false}, {branchPath, true}} {
		opts := CapacityOptions{Yield: subQ, Seed: subSeed, Workers: 1, Enabled: tc.enabled}
		runner, digest, err := RestoreCapacityCheckpoint(tc.path, opts)
		if err != nil {
			t.Fatal(err)
		}
		capacityStepToHour(t, runner, subHour)
		h, err := runner.Handoff()
		if err != nil {
			t.Fatal(err)
		}
		j, err := runner.ExportJournal()
		if err != nil {
			t.Fatal(err)
		}
		baseline := capacityRunTo(t, opts, subHour)
		if !reflect.DeepEqual(h, mustCapacityHandoff(t, baseline)) {
			t.Fatalf("enabled=%v subprocess scenario diverged from uninterrupted run", tc.enabled)
		}
		want.Write(h.History)
		want.Write(h.SchedulerBytes)
		want.Write(j)
		fmt.Fprintf(&want, "%#v", runner.Checkpoints())
		want.Write(digest[:])
		restored[tc.enabled] = runner
	}
	if !bytes.Equal(actual, want.Bytes()) {
		t.Fatal("subprocess continuation bytes differ from in-process restores")
	}
	if enabled, disabled := restored[true], restored[false]; reflect.DeepEqual(enabled.Journal(), disabled.Journal()) {
		t.Fatal("branch comparison did not diverge across process restore")
	}
	t.Logf("subprocess h%d: enabled alive=%d disabled alive=%d events=%d", subHour,
		restored[true].Checkpoints()[subHour].Alive, restored[false].Checkpoints()[subHour].Alive, len(restored[true].k.Events()))
}
