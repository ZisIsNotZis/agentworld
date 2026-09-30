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
// the capturing process. This includes a mid-horizon continuation restore
// after at least one birth (the q8 fixture births at h4), which exercises the
// dynamic roster, the variable-length binding table and the reserved
// WakeBirth reservations across a real process boundary, and a fresh-process
// run of a births-enabled root to the full frozen horizon.
func TestReproductionCheckpointSubprocess(t *testing.T) {
	const (
		subQ             = int64(8)
		subSeed          = uint64(0)
		subHour          = 6
		resumeHour       = 6
		continuationHour = 9
	)
	rootOpts := ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 4}
	branchOpts := rootOpts
	branchOpts.Births = true
	horizonOpts := ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 2, Births: true}
	if out := os.Getenv("REPRODUCTION_RESTORE_CHILD"); out != "" {
		q, err := strconv.ParseInt(os.Getenv("REPRODUCTION_Q"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seed, err := strconv.ParseUint(os.Getenv("REPRODUCTION_SEED"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		for _, tc := range []struct {
			path   string
			births bool
		}{{os.Getenv("REPRODUCTION_ROOT"), false}, {os.Getenv("REPRODUCTION_BRANCH"), true}} {
			// The child continues at a worker count the capturing process
			// never used (capture: 4, continuation: 1).
			runner, digest, err := RestoreReproductionCheckpoint(tc.path, ReproductionOptions{Yield: q, Seed: seed, Workers: 1, Births: tc.births})
			if err != nil {
				t.Fatal(err)
			}
			reproductionRestoreStepToHour(t, runner, subHour)
			h, err := runner.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			if h.Births != tc.births {
				t.Fatalf("child restored flag %v want %v", h.Births, tc.births)
			}
			// The births branch continuation's journal is not reproducible
			// from genesis under a single flag (the intervention decided at
			// the h0 boundary skipped the parent's request window), so the
			// evidence comparison uses the typed journal dump; ExportJournal's
			// scheduler-aware reconciliation applies to roots instead.
			buf.Write(h.History)
			buf.Write(h.SchedulerBytes)
			fmt.Fprintf(&buf, "%#v", runner.Journal())
			fmt.Fprintf(&buf, "%#v", runner.Checkpoints())
			buf.Write(digest[:])
		}
		// Mid-horizon continuation: restore the births branch at a second
		// worker count, pass the first birth, publish a pulse-boundary resume
		// bundle, and restore it at a third worker count.
		resumed, _, err := RestoreReproductionCheckpoint(os.Getenv("REPRODUCTION_BRANCH"), ReproductionOptions{Yield: q, Seed: seed, Workers: 2, Births: true})
		if err != nil {
			t.Fatal(err)
		}
		reproductionRestoreStepToHour(t, resumed, resumeHour)
		if resumed.Checkpoints()[resumeHour].Metrics.Births == 0 {
			t.Fatalf("no birth before the resume boundary at h%d", resumeHour)
		}
		resumePath := filepath.Join(t.TempDir(), "subprocess-resume.bundle")
		if _, err := resumed.SaveResumeCheckpoint(resumePath); err != nil {
			t.Fatal(err)
		}
		continuation, resumeDigest, err := RestoreReproductionCheckpoint(resumePath, ReproductionOptions{Yield: q, Seed: seed, Workers: 3, Births: true})
		if err != nil {
			t.Fatal(err)
		}
		reproductionRestoreStepToHour(t, continuation, continuationHour)
		h, err := continuation.Handoff()
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(h.History)
		buf.Write(h.SchedulerBytes)
		fmt.Fprintf(&buf, "%#v", continuation.Journal())
		fmt.Fprintf(&buf, "%#v", continuation.Checkpoints())
		buf.Write(resumeDigest[:])
		// A fresh process must also carry a births-enabled root to the full
		// frozen horizon through the runner's own tail checks.
		horizon, _, err := RestoreReproductionCheckpoint(os.Getenv("REPRODUCTION_HORIZON"), ReproductionOptions{Yield: q, Seed: seed, Workers: 1, Births: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := horizon.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		checks := horizon.Checkpoints()
		if len(checks) != ReproductionHorizonHours+1 || checks[ReproductionHorizonHours].Hour != ReproductionHorizonHours {
			t.Fatalf("restored horizon run incomplete: len=%d", len(checks))
		}
		if _, err := horizon.ExportJournal(); err != nil {
			t.Fatalf("restored horizon journal: %v", err)
		}
		if err := os.WriteFile(out, buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	_, rootPath, _ := reproductionCaptureNeutral(t, rootOpts)
	branchPath := filepath.Join(t.TempDir(), "subprocess-branch.bundle")
	if _, err := BranchReproductionCheckpoint(rootPath, rootOpts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	horizonPath := filepath.Join(t.TempDir(), "subprocess-horizon.bundle")
	// Publish the births-enabled neutral root for the child's horizon run.
	if _, err := mustReproductionAtH0(t, horizonOpts).SaveCheckpoint(horizonPath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "child.out")
	cmd := exec.Command(os.Args[0], "-test.run=^TestReproductionCheckpointSubprocess$")
	cmd.Env = append(os.Environ(),
		"REPRODUCTION_RESTORE_CHILD="+out,
		"REPRODUCTION_ROOT="+rootPath,
		"REPRODUCTION_BRANCH="+branchPath,
		"REPRODUCTION_HORIZON="+horizonPath,
		"REPRODUCTION_Q="+strconv.FormatInt(subQ, 10),
		"REPRODUCTION_SEED="+strconv.FormatUint(subSeed, 10))
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
	for _, tc := range []struct {
		path   string
		births bool
	}{{rootPath, false}, {branchPath, true}} {
		opts := ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 1, Births: tc.births}
		runner, digest, err := RestoreReproductionCheckpoint(tc.path, opts)
		if err != nil {
			t.Fatal(err)
		}
		reproductionRestoreStepToHour(t, runner, subHour)
		h, err := runner.Handoff()
		if err != nil {
			t.Fatal(err)
		}
		var baseline *Reproduction
		if !tc.births {
			baseline = reproductionRunTo(t, opts, subHour)
		} else {
			// The intervention baseline applies the recorded decision at the
			// branch's verified boundary (the h0 pulse ran births-disabled).
			baseline = mustReproductionAtH0(t, ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 1})
			baseline.births = true
			reproductionRestoreStepToHour(t, baseline, subHour)
		}
		if !reflect.DeepEqual(h, mustReproductionHandoff(t, baseline)) {
			t.Fatalf("births=%v subprocess scenario diverged from uninterrupted run", tc.births)
		}
		want.Write(h.History)
		want.Write(h.SchedulerBytes)
		fmt.Fprintf(&want, "%#v", runner.Journal())
		fmt.Fprintf(&want, "%#v", runner.Checkpoints())
		want.Write(digest[:])
	}
	// The parent's own mid-horizon resume sequence must match the child's.
	resumed, _, err := RestoreReproductionCheckpoint(branchPath, ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 2, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	reproductionRestoreStepToHour(t, resumed, resumeHour)
	resumePath := filepath.Join(t.TempDir(), "parent-resume.bundle")
	if _, err := resumed.SaveResumeCheckpoint(resumePath); err != nil {
		t.Fatal(err)
	}
	parentContinuation, resumeDigest, err := RestoreReproductionCheckpoint(resumePath, ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 3, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	reproductionRestoreStepToHour(t, parentContinuation, continuationHour)
	h, err := parentContinuation.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	// The resume continuation must equal the uninterrupted counterpart of its
	// own lineage: the intervention baseline (births decided at the same h0
	// boundary) carried straight to the continuation hour without an
	// intermediate checkpoint restore.
	interventionBaseline := mustReproductionAtH0(t, ReproductionOptions{Yield: subQ, Seed: subSeed, Workers: 1})
	interventionBaseline.births = true
	reproductionRestoreStepToHour(t, interventionBaseline, continuationHour)
	if !reflect.DeepEqual(h, mustReproductionHandoff(t, interventionBaseline)) {
		t.Fatal("resume continuation diverged from the intervention baseline")
	}
	want.Write(h.History)
	want.Write(h.SchedulerBytes)
	fmt.Fprintf(&want, "%#v", parentContinuation.Journal())
	fmt.Fprintf(&want, "%#v", parentContinuation.Checkpoints())
	want.Write(resumeDigest[:])
	if !bytes.Equal(actual, want.Bytes()) {
		t.Fatal("subprocess continuation bytes differ from in-process restores")
	}
	t.Logf("subprocess h%d: births=%d events=%d", continuationHour,
		parentContinuation.Checkpoints()[continuationHour].Metrics.Births, len(parentContinuation.k.Events()))
}
