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
// journal, scheduler bytes and projections. Full flag-equivalence across
// worker counts is exercised in the normal suite; this test proves the bundle
// survives a real process boundary.
func TestSocialFoodCheckpointSubprocess(t *testing.T) {
	const (
		subQ    = int64(3)
		subSeed = uint64(0)
		subHour = 24
	)
	rootOpts := SocialFoodOptions{Yield: subQ, Seed: subSeed, Workers: 4}
	branchOpts := rootOpts
	branchOpts.Enabled = true
	if out := os.Getenv("SOCIALFOOD_RESTORE_CHILD"); out != "" {
		q, err := strconv.ParseInt(os.Getenv("SOCIALFOOD_Q"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seed, err := strconv.ParseUint(os.Getenv("SOCIALFOOD_SEED"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		for _, tc := range []struct {
			path    string
			enabled bool
		}{{os.Getenv("SOCIALFOOD_ROOT"), false}, {os.Getenv("SOCIALFOOD_BRANCH"), true}} {
			runner, digest, err := RestoreSocialFoodCheckpoint(tc.path, SocialFoodOptions{Yield: q, Seed: seed, Workers: 1, Enabled: tc.enabled})
			if err != nil {
				t.Fatal(err)
			}
			socialFoodStepToHour(t, runner, subHour)
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
		horizon, _, err := RestoreSocialFoodCheckpoint(os.Getenv("SOCIALFOOD_BRANCH"), SocialFoodOptions{Yield: q, Seed: seed, Workers: 2, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := horizon.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		checks := horizon.Checkpoints()
		if len(checks) != SocialFoodHorizonHours+1 || checks[168].Hour != 168 || checks[168].Alive != 6 {
			t.Fatalf("restored horizon run incomplete: len=%d alive=%d", len(checks), checks[168].Alive)
		}
		if _, err := horizon.ExportJournal(); err != nil {
			t.Fatal(err)
		}
		return
	}
	_, rootPath, _ := socialFoodCaptureNeutral(t, rootOpts)
	branchPath := filepath.Join(t.TempDir(), "subprocess-branch.bundle")
	if _, err := BranchSocialFoodCheckpoint(rootPath, rootOpts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "child.out")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSocialFoodCheckpointSubprocess$")
	cmd.Env = append(os.Environ(),
		"SOCIALFOOD_RESTORE_CHILD="+out,
		"SOCIALFOOD_ROOT="+rootPath,
		"SOCIALFOOD_BRANCH="+branchPath,
		"SOCIALFOOD_Q="+strconv.FormatInt(subQ, 10),
		"SOCIALFOOD_SEED="+strconv.FormatUint(subSeed, 10))
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
	restored := map[bool]*SocialFood{}
	for _, tc := range []struct {
		path    string
		enabled bool
	}{{rootPath, false}, {branchPath, true}} {
		opts := SocialFoodOptions{Yield: subQ, Seed: subSeed, Workers: 1, Enabled: tc.enabled}
		runner, digest, err := RestoreSocialFoodCheckpoint(tc.path, opts)
		if err != nil {
			t.Fatal(err)
		}
		socialFoodStepToHour(t, runner, subHour)
		h, err := runner.Handoff()
		if err != nil {
			t.Fatal(err)
		}
		j, err := runner.ExportJournal()
		if err != nil {
			t.Fatal(err)
		}
		baseline := socialFoodRunTo(t, opts, subHour)
		if !reflect.DeepEqual(h, mustSocialFoodHandoff(t, baseline)) {
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
