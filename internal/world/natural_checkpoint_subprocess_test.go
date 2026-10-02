//go:build !race

package world

import (
	"bytes"
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
// the capturing process. This covers all three declared v5 bundle shapes:
// a neutral h0 root, a default-world pulse-boundary continuation whose slots
// tenure through the frozen build flow, and a declared founder-tenure
// fixture continuation whose births commit under the frozen policy before
// the boundary (the organic cell never births, so the post-birth state is
// constructed explicitly). The dynamic roster, the variable-length binding
// table and the reserved WakeBirth reservations all cross the process
// boundary in the fixture case.
func TestNaturalCheckpointSubprocess(t *testing.T) {
	const (
		subSeed    = uint64(11)
		fixtureSub = uint64(13)
	)
	rootOpts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: subSeed, Workers: 4}
	if out := os.Getenv("NATURAL_RESTORE_CHILD"); out != "" {
		seed, err := strconv.ParseUint(os.Getenv("NATURAL_SEED"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		fixtureSeed, err := strconv.ParseUint(os.Getenv("NATURAL_FIXTURE_SEED"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		// The child continues at worker counts the capturing process never
		// used (capture: 4, continuations: 1 and 2).
		for _, tc := range []struct {
			path     string
			opts     NaturalRestoreOptions
			stepHour int
		}{
			{os.Getenv("NATURAL_ROOT"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1}, 3},
			{os.Getenv("NATURAL_BRANCH"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 2, Births: true}, 3},
			{os.Getenv("NATURAL_RESUME"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: true}, 6},
			{os.Getenv("NATURAL_FIXTURE"), NaturalRestoreOptions{Patches: 2, R: 8, Seed: fixtureSeed, Workers: 1, Births: true, FounderTenure: true}, 8},
		} {
			runner, digest, err := RestoreNaturalCheckpoint(tc.path, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			naturalStepToBoundary(t, runner, tc.stepHour)
			h, err := runner.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			if h.Births != tc.opts.Births || h.Patches != tc.opts.Patches || h.R != tc.opts.R || h.Seed != tc.opts.Seed {
				t.Fatalf("child restored scenario %+v want %+v", h, tc.opts)
			}
			buf.Write(h.History)
			buf.Write(h.SchedulerBytes)
			fmt.Fprintf(&buf, "%#v", runner.Journal())
			fmt.Fprintf(&buf, "%#v", runner.Checkpoints())
			buf.Write(digest[:])
		}
		if err := os.WriteFile(out, buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	_, rootPath, _ := naturalCaptureRoot(t, subSeed, false, 4)
	// A births-enabled continuation of the births-disabled root requires the
	// recorded branch.
	branchPath := filepath.Join(t.TempDir(), "subprocess-branch.bundle")
	if _, err := BranchNaturalCheckpoint(rootPath, rootOpts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	resume := naturalNeutralAt(t, subSeed, true, 4)
	naturalStepToBoundary(t, resume, 4)
	if naturalCheckpointAt(t, resume, 4).Balance.TenuredSlots == 0 {
		t.Fatal("no tenure change before the subprocess capture boundary")
	}
	resumePath := filepath.Join(t.TempDir(), "subprocess-resume.bundle")
	if _, err := resume.SaveResumeCheckpoint(resumePath); err != nil {
		t.Fatal(err)
	}
	fixture := naturalJournalFixture(t, fixtureSub, true, 4, 5)
	if naturalCheckpointAt(t, fixture, 5).Balance.Births == 0 {
		t.Fatal("fixture produced no births before the subprocess capture boundary")
	}
	fixturePath := filepath.Join(t.TempDir(), "subprocess-fixture.bundle")
	if _, err := fixture.SaveResumeCheckpoint(fixturePath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "child.out")
	cmd := exec.Command(os.Args[0], "-test.run=^TestNaturalCheckpointSubprocess$")
	cmd.Env = append(os.Environ(),
		"NATURAL_RESTORE_CHILD="+out,
		"NATURAL_ROOT="+rootPath,
		"NATURAL_BRANCH="+branchPath,
		"NATURAL_RESUME="+resumePath,
		"NATURAL_FIXTURE="+fixturePath,
		"NATURAL_SEED="+strconv.FormatUint(subSeed, 10),
		"NATURAL_FIXTURE_SEED="+strconv.FormatUint(fixtureSub, 10))
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
		path     string
		opts     NaturalRestoreOptions
		stepHour int
		baseline func() *Natural
	}{
		{rootPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: subSeed, Workers: 1}, 3,
			func() *Natural { return naturalNeutralAt(t, subSeed, false, 1) }},
		{branchPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: subSeed, Workers: 1, Births: true}, 3,
			func() *Natural {
				// The intervention baseline applies the recorded decision at
				// the branch's verified boundary (the h0 pulse ran
				// births-disabled, so the h0 request window is absent).
				b := naturalNeutralAt(t, subSeed, false, 1)
				b.births = true
				return b
			}},
		{resumePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: subSeed, Workers: 2, Births: true}, 6,
			func() *Natural { return naturalNeutralAt(t, subSeed, true, 2) }},
		{fixturePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: fixtureSub, Workers: 1, Births: true, FounderTenure: true}, 8,
			func() *Natural { return naturalJournalFixture(t, fixtureSub, true, 1, 8) }},
	} {
		runner, digest, err := RestoreNaturalCheckpoint(tc.path, tc.opts)
		if err != nil {
			t.Fatal(err)
		}
		naturalStepToBoundary(t, runner, tc.stepHour)
		h, err := runner.Handoff()
		if err != nil {
			t.Fatal(err)
		}
		baseline := tc.baseline()
		naturalStepToBoundary(t, baseline, tc.stepHour)
		if !reflect.DeepEqual(h, naturalMustHandoff(t, baseline)) {
			t.Fatalf("scenario %+v subprocess restore diverged from uninterrupted run", tc.opts)
		}
		want.Write(h.History)
		want.Write(h.SchedulerBytes)
		fmt.Fprintf(&want, "%#v", runner.Journal())
		fmt.Fprintf(&want, "%#v", runner.Checkpoints())
		want.Write(digest[:])
	}
	if !bytes.Equal(actual, want.Bytes()) {
		t.Fatal("subprocess continuation bytes differ from in-process restores")
	}
	// A fresh process must also carry a births-enabled fixture world through
	// the runner's own conservation checks past several further births.
	horizonFixture, _, err := RestoreNaturalCheckpoint(fixturePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: fixtureSub, Workers: 1, Births: true, FounderTenure: true})
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, horizonFixture, 10)
	if got := naturalCheckpointAt(t, horizonFixture, 10).Balance.Births; got < 2 {
		t.Fatalf("fixture births by h10: %d", got)
	}
}
