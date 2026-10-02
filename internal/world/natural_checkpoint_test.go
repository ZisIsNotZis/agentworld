package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// naturalManifestFlagOffset is the manifest offset of the raw births flag
// byte: magic + 16 u32 constants + 31 u64 constants + patches + R + seed.
const naturalManifestFlagOffset = 4 + 16*4 + 31*8 + 8 + 8 + 8

func naturalCaptureRoot(t *testing.T, seed uint64, births bool, workers int) (*Natural, string, [32]byte) {
	t.Helper()
	f := naturalNeutralAt(t, seed, births, workers)
	if f.steps != 1 || f.sched.Time() != 0 || len(f.Checkpoints()) != 1 || f.Checkpoints()[0].Hour != 0 || len(f.Journal()[0].Attempts) != 0 {
		t.Fatalf("not a neutral h0 pulse boundary: steps=%d time=%d", f.steps, f.sched.Time())
	}
	path := filepath.Join(t.TempDir(), "natural-neutral-h0.bundle")
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	return f, path, digest
}

func naturalEditManifest(t *testing.T, sections []checkpoint.Section, edit func(*naturalManifest)) {
	t.Helper()
	m, err := decodeNaturalManifest(sections[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	edit(&m)
	sections[2].Data = encodeNaturalManifest(m)
}

func naturalRewriteBundle(t *testing.T, path string, edit func([]checkpoint.Section)) string {
	t.Helper()
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sections = append([]checkpoint.Section(nil), sections...)
	for i := range sections {
		sections[i].Data = bytes.Clone(sections[i].Data)
	}
	edit(sections)
	dest := filepath.Join(t.TempDir(), "forged-natural.bundle")
	if _, err := checkpoint.WriteFile(dest, sections); err != nil {
		t.Fatal(err)
	}
	return dest
}

// naturalAlterPortable exports an altered-but-generically-valid portable
// scheduler state: the generic scheduler accepts the change, so only the
// natural-specific protocol checks can reject it.
func naturalAlterPortable(t *testing.T, f *Natural, change func(*scheduler.Snapshot)) (*scheduler.Scheduler, []byte) {
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

func naturalMustHandoff(t *testing.T, f *Natural) NaturalHandoff {
	t.Helper()
	h, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func naturalEventBytes(t *testing.T, f *Natural) [][]byte {
	t.Helper()
	events := f.k.Events()
	out := make([][]byte, 0, len(events))
	for _, ev := range events {
		body, err := ev.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, body)
	}
	return out
}

func TestNaturalCheckpointNeutralCaptureRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		seed   uint64
		births bool
	}{{"disabled", 0, false}, {"births-enabled", 0, true}, {"other-seed", 7, true}} {
		t.Run(tc.name, func(t *testing.T) {
			expected := NaturalRestoreOptions{Patches: 2, R: 8, Seed: tc.seed, Workers: 1, Births: tc.births}
			f, path, digest := naturalCaptureRoot(t, tc.seed, tc.births, 1)
			before := naturalMustHandoff(t, f)
			restored, got, err := RestoreNaturalCheckpoint(path, expected)
			if err != nil || got != digest {
				t.Fatalf("restore: digest=%x want=%x err=%v", got, digest, err)
			}
			if !reflect.DeepEqual(naturalMustHandoff(t, restored), before) {
				t.Fatal("handoff did not round trip through the neutral bundle")
			}
			if !reflect.DeepEqual(restored.Checkpoints(), f.Checkpoints()) || !reflect.DeepEqual(restored.Journal(), f.Journal()) {
				t.Fatal("h0 projection or journal changed across restore")
			}
			// The bundle does not pin the worker count: any count restores.
			for _, workers := range []int{1, 2, 4} {
				rebind := expected
				rebind.Workers = workers
				if _, _, err := RestoreNaturalCheckpoint(path, rebind); err != nil {
					t.Fatalf("worker count %d refused: %v", workers, err)
				}
			}
			sections, _, err := checkpoint.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := decodeNaturalManifest(sections[2].Data)
			if err != nil {
				t.Fatal(err)
			}
			if m.depth != 0 || m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) ||
				m.births != tc.births || m.boundary != tc.births || m.tenure || m.patches != 2 ||
				m.r != 8 || m.seed != tc.seed || m.steps != 1 || m.time != 0 {
				t.Fatalf("root manifest lineage/scenario wrong: %+v", m)
			}
			fingerprint, err := strategy.NaturalPolicyFingerprint(strategy.FrozenNaturalPolicy())
			if err != nil || m.policyFingerprint != fingerprint {
				t.Fatalf("manifest policy fingerprint not pinned: %x %v", m.policyFingerprint, err)
			}
			if exported, err := restored.ExportJournal(); err != nil || !bytes.Equal(exported, sections[0].Data) {
				t.Fatalf("restored journal config not carried: %v", err)
			}
			// Re-saving the restored runner reproduces the identical root bundle.
			resave := filepath.Join(t.TempDir(), "resaved.bundle")
			if again, err := restored.SaveCheckpoint(resave); err != nil || again != digest {
				t.Fatalf("re-save digest=%x want=%x err=%v", again, digest, err)
			}
			// Saving refuses any boundary that is not the neutral h0 pulse, and
			// a resume bundle is refused at the neutral root boundary.
			naturalStepToBoundary(t, f, 1)
			if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "late.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatalf("published non-neutral boundary: %v", err)
			}
			h0 := naturalNeutralAt(t, tc.seed, tc.births, 1)
			if _, err := h0.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "h0-resume.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatal("published a resume bundle at the neutral root boundary")
			}
			// The declared founder-tenure fixture world has no neutral root:
			// its first pulse is naturalFixtureStartHour.
			fixture, err := newNaturalFixtureRunner(2, 8, 3, true, 1)
			if err != nil {
				t.Fatal(err)
			}
			naturalStepToBoundary(t, fixture, naturalFixtureStartHour)
			if fixture.Checkpoints()[0].Hour != naturalFixtureStartHour {
				t.Fatalf("fixture first checkpoint %d", fixture.Checkpoints()[0].Hour)
			}
			if _, err := fixture.SaveCheckpoint(filepath.Join(t.TempDir(), "fixture-root.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatal("published a fixture root as a neutral h0 bundle")
			}
		})
	}
}

// TestNaturalCheckpointResumeAfterTenureAndBirths exercises the
// pulse-boundary continuation path end to end for the two declared v5
// scenarios: the default world, whose slots tenure through the frozen build
// flow (at least one tenure change before the boundary), and the declared
// founder-tenure fixture, whose births commit under the frozen policy
// before the boundary (the organic cell never births, so the post-birth
// state is constructed explicitly). Restores at other worker counts continue
// byte-identically against uninterrupted baselines.
func TestNaturalCheckpointResumeAfterTenureAndBirths(t *testing.T) {
	t.Run("default-world-tenure", func(t *testing.T) {
		opts := NaturalOptions{Patches: 2, R: 8, Seed: 11, Workers: 4, Births: true}
		baseline := naturalNeutralAt(t, opts.Seed, true, 4)
		naturalStepToBoundary(t, baseline, 7)
		f := naturalNeutralAt(t, opts.Seed, true, 4)
		naturalStepToBoundary(t, f, 4)
		tenured := 0
		for _, row := range f.Checkpoints()[4].Slots {
			for _, s := range row {
				if s.Mode == NaturalTenureCultivated {
					tenured++
				}
			}
		}
		if tenured == 0 {
			t.Fatal("no tenure change before the capture boundary")
		}
		resumePath := filepath.Join(t.TempDir(), "resume-h4.bundle")
		digest, err := f.SaveResumeCheckpoint(resumePath)
		if err != nil {
			t.Fatal(err)
		}
		if boundary := naturalMustHandoff(t, f); !reflect.DeepEqual(naturalMustHandoff(t, restoredNatural(t, resumePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 11, Workers: 2, Births: true})), boundary) {
			t.Fatal("boundary handoff diverged across restore")
		}
		for _, workers := range []int{1, 2, 3} {
			continued, _, err := RestoreNaturalCheckpoint(resumePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 11, Workers: workers, Births: true})
			if err != nil {
				t.Fatalf("workers=%d restore: %v", workers, err)
			}
			naturalStepToBoundary(t, continued, 7)
			if !reflect.DeepEqual(naturalMustHandoff(t, continued), naturalMustHandoff(t, baseline)) {
				t.Fatalf("workers=%d continuation diverged from the uninterrupted run", workers)
			}
			if !reflect.DeepEqual(naturalEventBytes(t, continued), naturalEventBytes(t, baseline)) {
				t.Fatalf("workers=%d continuation event bytes diverged", workers)
			}
		}
		// Re-saving the restored boundary reproduces the identical bundle.
		restored := restoredNatural(t, resumePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 11, Workers: 1, Births: true})
		again := filepath.Join(t.TempDir(), "resume-resaved.bundle")
		if resaved, err := restored.SaveResumeCheckpoint(again); err != nil || resaved != digest {
			t.Fatalf("resume re-save digest=%x want=%x err=%v", resaved, digest, err)
		}
		// A capture between instants (in-flight hour) is refused.
		midHour := naturalNeutralAt(t, opts.Seed, true, 4)
		naturalStepToBoundary(t, midHour, 4)
		if ok, err := midHour.Step(context.Background()); err != nil || !ok {
			t.Fatalf("mid-hour step: %v %v", ok, err)
		}
		if _, err := midHour.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "midhour.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatal("published a resume bundle mid-hour")
		}
		// A resume bundle is not a branch parent.
		branchDir := t.TempDir()
		if _, err := BranchNaturalCheckpoint(resumePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 11, Workers: 1, Births: true}, false, filepath.Join(branchDir, "nope.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatal("branched from a continuation bundle")
		}
		if entries, err := os.ReadDir(branchDir); err != nil || len(entries) != 0 {
			t.Fatalf("refused branch left entries behind: %v %v", entries, err)
		}
		// A continuation restores only under its exact declared scenario.
		for _, wrong := range []NaturalRestoreOptions{
			{Patches: 2, R: 7, Seed: 11, Workers: 1, Births: true},
			{Patches: 2, R: 8, Seed: 12, Workers: 1, Births: true},
			{Patches: 2, R: 8, Seed: 11, Workers: 1},
			{Patches: 2, R: 8, Seed: 11, Workers: 0, Births: true},
			{Patches: 2, R: 8, Seed: 11, Workers: 1, Births: true, FounderTenure: true},
		} {
			if r, _, err := RestoreNaturalCheckpoint(resumePath, wrong); r != nil || !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatalf("mismatched scenario %+v accepted: %v", wrong, err)
			}
		}
		// A forged lineage on a continuation bundle fails at decode.
		forged := naturalRewriteBundle(t, resumePath, func(s []checkpoint.Section) {
			naturalEditManifest(t, s, func(m *naturalManifest) {
				m.parent = [32]byte{9}
				m.parentHead = m.head
				m.depth = 1
			})
		})
		if r, _, err := RestoreNaturalCheckpoint(forged, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 11, Workers: 1, Births: true}); r != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("forged continuation lineage accepted: %v", err)
		}
	})
	t.Run("fixture-world-births", func(t *testing.T) {
		expected := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 3, Workers: 4, Births: true, FounderTenure: true}
		baseline := naturalJournalFixture(t, 3, true, 4, 8)
		f := naturalJournalFixture(t, 3, true, 4, 4)
		checks := f.Checkpoints()
		if naturalCheckpointAt(t, f, 4).Balance.Births == 0 {
			t.Fatal("fixture produced no births before the capture boundary")
		}
		_ = checks
		resumePath := filepath.Join(t.TempDir(), "resume-births.bundle")
		digest, err := f.SaveResumeCheckpoint(resumePath)
		if err != nil {
			t.Fatal(err)
		}
		restored := restoredNatural(t, resumePath, expected)
		if !reflect.DeepEqual(naturalMustHandoff(t, restored), naturalMustHandoff(t, f)) {
			t.Fatal("resume boundary handoff diverged")
		}
		if len(naturalCheckpointAt(t, restored, 4).Newborns) < 2 {
			t.Fatal("dynamic roster not restored")
		}
		wakeClaims := 0
		for _, b := range restored.Journal() {
			for _, a := range b.Attempts {
				if a.BirthWake {
					wakeClaims++
				}
			}
		}
		if wakeClaims == 0 {
			t.Fatal("no reserved WakeBirth claim in the restored journal")
		}
		// Continuation at worker counts neither capture nor first restore used.
		for _, workers := range []int{1, 2, 3} {
			continued, _, err := RestoreNaturalCheckpoint(resumePath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 3, Workers: workers, Births: true, FounderTenure: true})
			if err != nil {
				t.Fatalf("workers=%d restore: %v", workers, err)
			}
			naturalStepToBoundary(t, continued, 8)
			if !reflect.DeepEqual(naturalMustHandoff(t, continued), naturalMustHandoff(t, baseline)) {
				t.Fatalf("workers=%d continuation diverged from the uninterrupted run", workers)
			}
			if !reflect.DeepEqual(naturalEventBytes(t, continued), naturalEventBytes(t, baseline)) {
				t.Fatalf("workers=%d continuation event bytes diverged", workers)
			}
		}
		// Re-saving the restored boundary reproduces the identical bundle.
		again := filepath.Join(t.TempDir(), "resume-resaved.bundle")
		if resaved, err := restored.SaveResumeCheckpoint(again); err != nil || resaved != digest {
			t.Fatalf("resume re-save digest=%x want=%x err=%v", resaved, digest, err)
		}
		// A continuation cannot be captured at the fixture's own genesis
		// boundary (steps == 1).
		fresh, err := newNaturalFixtureRunner(2, 8, 3, true, 1)
		if err != nil {
			t.Fatal(err)
		}
		naturalStepToBoundary(t, fresh, naturalFixtureStartHour)
		if _, err := fresh.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "genesis.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatal("published a resume bundle at the fixture genesis boundary")
		}
		// A forged wake topology on a continuation bundle fails the boundary
		// check: drop the reserved WakeBirth claim of a newborn (the fibers'
		// earliest-wake pointers are repaired so only the natural boundary
		// check can reject the state).
		altered, _ := naturalAlterPortable(t, f, func(s *scheduler.Snapshot) {
			dropped := false
			kept := s.Wakes[:0]
			for _, wake := range s.Wakes {
				if wake.Cause == scheduler.WakeBirth && !dropped {
					dropped = true
					continue
				}
				kept = append(kept, wake)
			}
			if !dropped {
				t.Fatal("fixture has no pending WakeBirth reservation")
			}
			s.Wakes = kept
		})
		f.sched = altered
		if _, err := f.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "bad-topology.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatal("published a continuation with a dropped WakeBirth reservation")
		}
	})
}

// naturalCheckpointAt returns the runner's hourly checkpoint at hour (the
// fixture's checkpoint hours start at naturalFixtureStartHour).
func naturalCheckpointAt(t *testing.T, f *Natural, hour int) NaturalCheckpoint {
	t.Helper()
	for _, check := range f.Checkpoints() {
		if check.Hour == hour {
			return check
		}
	}
	t.Fatalf("no checkpoint at hour %d", hour)
	return NaturalCheckpoint{}
}

func restoredNatural(t *testing.T, path string, expected NaturalRestoreOptions) *Natural {
	t.Helper()
	f, _, err := RestoreNaturalCheckpoint(path, expected)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNaturalCheckpointBranchRecordsIntervention(t *testing.T) {
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 4}
	enabledOpts := opts
	enabledOpts.Births = true
	_, disabledPath, disabledDigest := naturalCaptureRoot(t, 0, false, 4)
	_, enabledPath, enabledDigest := naturalCaptureRoot(t, 0, true, 4)
	if disabledDigest == enabledDigest {
		t.Fatal("flag did not change the neutral bundle")
	}
	disabledSections, _, err := checkpoint.ReadFile(disabledPath)
	if err != nil {
		t.Fatal(err)
	}
	enabledSections, _, err := checkpoint.ReadFile(enabledPath)
	if err != nil {
		t.Fatal(err)
	}
	// The v5 h0 pulse is births-asymmetric in wake topology by frozen design
	// (the request phase exists only when births are enabled): history and
	// pinned policy bytes are identical, while the scheduler queue and the
	// journal configuration record the flag.
	for _, index := range []int{1, 4} {
		if !bytes.Equal(disabledSections[index].Data, enabledSections[index].Data) {
			t.Fatalf("flag leaked into section %d", index)
		}
	}
	if bytes.Equal(disabledSections[3].Data, enabledSections[3].Data) {
		t.Fatal("flag did not change the h0 request wakes")
	}
	if bytes.Equal(disabledSections[0].Data, enabledSections[0].Data) {
		t.Fatal("journal configuration did not pin the flag")
	}
	// An explicit branch intervention flips the flag into a child bundle whose
	// lineage records the parent digest and head; policy, history and
	// scheduler bytes stay untouched.
	branchPath := filepath.Join(t.TempDir(), "intervention-births.bundle")
	branchDigest, err := BranchNaturalCheckpoint(disabledPath, opts, true, branchPath)
	if err != nil {
		t.Fatal(err)
	}
	branchSections, gotBranch, err := checkpoint.ReadFile(branchPath)
	if err != nil || gotBranch != branchDigest {
		t.Fatalf("branch bundle: %x %v", gotBranch, err)
	}
	if !bytes.Equal(branchSections[4].Data, disabledSections[4].Data) || !bytes.Equal(branchSections[1].Data, disabledSections[1].Data) || !bytes.Equal(branchSections[3].Data, disabledSections[3].Data) {
		t.Fatal("branch edited serialized policy, history or scheduler bytes")
	}
	m, err := decodeNaturalManifest(branchSections[2].Data)
	if err != nil || !m.births || m.boundary || m.depth != 1 || m.parent != disabledDigest || m.parentHead != m.head {
		t.Fatalf("branch lineage not recorded: %+v err=%v", m, err)
	}
	branchRunner := restoredNatural(t, branchPath, enabledOpts)
	if !branchRunner.births {
		t.Fatal("restored run does not match the intervention flag")
	}
	// The restored child's journal configuration matches the recorded
	// intervention, proven through the history-aware decode against the
	// child's own verified head (the scheduler-aware reconciliation replays
	// under the runner's own flag, which cannot reproduce the
	// parent-boundary portable bytes of a branch child).
	branchHead := naturalVerifiedHead(t, branchRunner)
	interventionConfig := branchRunner.journalGenesisKind()
	interventionConfig.Boundary = m.boundary
	if batches, err := DecodeNaturalJournal(branchSections[0].Data, branchRunner.k, branchHead, branchRunner.sched.Time(), interventionConfig); err != nil || len(batches) == 0 {
		t.Fatalf("restored journal config does not match the recorded intervention: %v", err)
	}
	// The intervention baseline applies the recorded decision at the same
	// verified boundary: the h0 pulse ran births-disabled, so the h0 request
	// window is absent, and the child diverges from a births-enabled h0 run
	// exactly there.
	baselineDisabled := naturalNeutralAt(t, 0, false, 1)
	naturalStepToBoundary(t, baselineDisabled, 4)
	baselineEnabled := naturalNeutralAt(t, 0, false, 1)
	baselineEnabled.births = true
	naturalStepToBoundary(t, baselineEnabled, 4)
	restoredDisabled := restoredNatural(t, disabledPath, opts)
	naturalStepToBoundary(t, restoredDisabled, 4)
	naturalStepToBoundary(t, branchRunner, 4)
	if !reflect.DeepEqual(naturalMustHandoff(t, restoredDisabled), naturalMustHandoff(t, baselineDisabled)) {
		t.Fatal("restored disabled run diverged from uninterrupted same-flag run")
	}
	if !reflect.DeepEqual(naturalMustHandoff(t, branchRunner), naturalMustHandoff(t, baselineEnabled)) {
		t.Fatal("restored births branch diverged from the intervention baseline")
	}
	if !reflect.DeepEqual(naturalEventBytes(t, branchRunner), naturalEventBytes(t, baselineEnabled)) ||
		!reflect.DeepEqual(naturalEventBytes(t, restoredDisabled), naturalEventBytes(t, baselineDisabled)) {
		t.Fatal("accepted event bytes diverged from same-flag baselines")
	}
	// A same-flag lineage link is recorded as such and still restores.
	samePath := filepath.Join(t.TempDir(), "intervention-same.bundle")
	if _, err := BranchNaturalCheckpoint(disabledPath, opts, false, samePath); err != nil {
		t.Fatal(err)
	}
	sameSections, _, err := checkpoint.ReadFile(samePath)
	if err != nil {
		t.Fatal(err)
	}
	if m, err := decodeNaturalManifest(sameSections[2].Data); err != nil || m.births || m.depth != 1 || m.parent != disabledDigest {
		t.Fatalf("same-flag intervention not recorded: %+v err=%v", m, err)
	}
	// The disabling direction is refused before publication: an enabled
	// boundary's h0 topology carries pending request wakes that a
	// births-disabled continuation may not evaluate.
	reverseDir := t.TempDir()
	reversePath := filepath.Join(reverseDir, "intervention-disabled.bundle")
	if _, err := BranchNaturalCheckpoint(enabledPath, enabledOpts, false, reversePath); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("disabled a births-enabled lineage: %v", err)
	}
	if entries, err := os.ReadDir(reverseDir); err != nil || len(entries) != 0 {
		t.Fatalf("refused branch left entries behind: %v %v", entries, err)
	}
	// Branching demands the parent scenario and never clobbers.
	branchDir := t.TempDir()
	wrongScenario := opts
	wrongScenario.Seed = 5
	dest := filepath.Join(branchDir, "rejected-branch.bundle")
	if _, err := BranchNaturalCheckpoint(disabledPath, wrongScenario, true, dest); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branched from a misdeclared parent: %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial branch publication: %v", err)
	}
	parentBytes, err := os.ReadFile(disabledPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BranchNaturalCheckpoint(disabledPath, opts, true, disabledPath); !errors.Is(err, os.ErrExist) {
		t.Fatalf("branch clobbered the parent bundle: %v", err)
	}
	after, err := os.ReadFile(disabledPath)
	if err != nil || !bytes.Equal(parentBytes, after) {
		t.Fatal("parent bundle modified by refused branch")
	}
	if entries, err := os.ReadDir(branchDir); err != nil || len(entries) != 0 {
		t.Fatalf("branch left entries behind: %v", entries)
	}
}

func TestNaturalCheckpointBranchDepthBound(t *testing.T) {
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1}
	_, root, rootDigest := naturalCaptureRoot(t, 0, false, 1)
	// One depth-1 child for lineage forgery tests, then chain to the frozen
	// bound: depth 63 is restorable, depth 64 is refused before publication
	// because it could never be restored.
	child := filepath.Join(t.TempDir(), "depth-1.bundle")
	childDigest, err := BranchNaturalCheckpoint(root, opts, false, child)
	if err != nil {
		t.Fatal(err)
	}
	current := child
	for i := 2; i <= naturalMaxBranchDepth; i++ {
		next := filepath.Join(t.TempDir(), fmt.Sprintf("depth-%d.bundle", i))
		if _, err := BranchNaturalCheckpoint(current, opts, false, next); err != nil {
			t.Fatalf("branch to depth %d: %v", i, err)
		}
		current = next
	}
	if _, _, err := RestoreNaturalCheckpoint(current, opts); err != nil {
		t.Fatalf("depth-63 bundle refused: %v", err)
	}
	deepDir := t.TempDir()
	tooDeep := filepath.Join(deepDir, "too-deep.bundle")
	if _, err := BranchNaturalCheckpoint(current, opts, false, tooDeep); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("published an unrestorable depth: %v", err)
	}
	if entries, err := os.ReadDir(deepDir); err != nil || len(entries) != 0 {
		t.Fatalf("over-deep branch left entries behind: %v %v", entries, err)
	}
	// A forged manifest claiming over-deep ancestry is rejected at decode.
	forged := naturalRewriteBundle(t, current, func(s []checkpoint.Section) {
		naturalEditManifest(t, s, func(m *naturalManifest) { m.depth = naturalMaxBranchDepth + 1 })
	})
	if f, _, err := RestoreNaturalCheckpoint(forged, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("accepted over-deep lineage: %v", err)
	}
	// Forged lineage links fail closed: a root capture must record no parent,
	// and a branch child must continue its parent at the same verified head.
	badRoot := naturalRewriteBundle(t, root, func(s []checkpoint.Section) {
		naturalEditManifest(t, s, func(m *naturalManifest) { m.parent = childDigest })
	})
	if f, _, err := RestoreNaturalCheckpoint(badRoot, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("root bundle accepted a forged parent: %v", err)
	}
	if rootDigest == childDigest {
		t.Fatal("root and child digests must differ")
	}
	for name, edit := range map[string]func(*naturalManifest){
		"parent-head": func(m *naturalManifest) { m.parentHead.TipID++ },
		"depth-zero":  func(m *naturalManifest) { m.depth = 0 },
	} {
		bad := naturalRewriteBundle(t, child, func(s []checkpoint.Section) {
			naturalEditManifest(t, s, edit)
		})
		if f, _, err := RestoreNaturalCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("forged %s accepted: %v", name, err)
		}
	}
}

func TestNaturalCheckpointRejectsDamageTruncationSwapAndCrossVersion(t *testing.T) {
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1}
	_, path, _ := naturalCaptureRoot(t, 0, false, 1)
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Damage to any section is caught by the store digest or the manifest's
	// independent section hashes; rehashing the envelope alone is not enough.
	for i := range sections {
		bad := append([]checkpoint.Section(nil), sections...)
		bad[i].Data = bytes.Clone(bad[i].Data)
		bad[i].Data[len(bad[i].Data)/2] ^= 1
		corrupt := filepath.Join(t.TempDir(), "corrupt-natural.bundle")
		if _, err := checkpoint.WriteFile(corrupt, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreNaturalCheckpoint(corrupt, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("accepted rehashed section %d: %v", i, err)
		}
	}
	// Swapped section payloads are caught by the manifest section hashes.
	for _, swap := range [][2]int{{0, 1}, {3, 4}} {
		bad := append([]checkpoint.Section(nil), sections...)
		bad[swap[0]].Data, bad[swap[1]].Data = bytes.Clone(bad[swap[1]].Data), bytes.Clone(bad[swap[0]].Data)
		swapped := filepath.Join(t.TempDir(), "swapped-natural.bundle")
		if _, err := checkpoint.WriteFile(swapped, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreNaturalCheckpoint(swapped, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("accepted swapped sections %v: %v", swap, err)
		}
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{wire[:len(wire)-1], wire[:len(wire)/2], append(bytes.Clone(wire), 0)} {
		bad := filepath.Join(t.TempDir(), "truncated-natural.bundle")
		if err := os.WriteFile(bad, malformed, 0600); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreNaturalCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("accepted malformed bundle: %v", err)
		}
	}
	// Bidirectional cross-version rejection: v1/S6/v2/v3/v4 bundles never
	// decode as v5 and vice versa — no implicit migration in either direction.
	_, foodflowPath, _ := foodFlowSaveAt(t, 3, 2)
	_, socialPath, _ := socialFoodCaptureNeutral(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4})
	s6, err := NewSurvival(SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	s6Path := filepath.Join(t.TempDir(), "s6-natural-reject.bundle")
	if _, err := s6.SaveCheckpoint(s6Path); err != nil {
		t.Fatal(err)
	}
	capacityPath := filepath.Join(t.TempDir(), "capacity-natural-reject.bundle")
	if _, err := mustCapacityAtH0(t, CapacityOptions{Yield: 3, Seed: 0, Workers: 1}).SaveCheckpoint(capacityPath); err != nil {
		t.Fatal(err)
	}
	reproductionPath := filepath.Join(t.TempDir(), "reproduction-natural-reject.bundle")
	if _, err := mustReproductionAtH0(t, ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}).SaveCheckpoint(reproductionPath); err != nil {
		t.Fatal(err)
	}
	for name, foreign := range map[string]string{"food-flow": foodflowPath, "social-food": socialPath, "s6": s6Path, "capacity": capacityPath, "reproduction": reproductionPath} {
		if f, _, err := RestoreNaturalCheckpoint(foreign, opts); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("accepted foreign %s bundle: %v", name, err)
		}
	}
	v1opts := FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}
	if f, _, err := RestoreFoodFlowCheckpoint(path, v1opts); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("food-flow accepted v5 bundle: %v", err)
	}
	v2opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	if f, _, err := RestoreSocialFoodCheckpoint(path, v2opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("social-food accepted v5 bundle: %v", err)
	}
	s6opts := SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1}
	if f, _, err := RestoreSurvivalCheckpoint(path, s6opts); f != nil || !errors.Is(err, ErrSurvivalCheckpoint) {
		t.Fatalf("S6 accepted v5 bundle: %v", err)
	}
	v3opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	if f, _, err := RestoreCapacityCheckpoint(path, v3opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("capacity accepted v5 bundle: %v", err)
	}
	v4opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}
	if f, _, err := RestoreReproductionCheckpoint(path, v4opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("reproduction accepted v5 bundle: %v", err)
	}
}

func TestNaturalCheckpointRejectsForgedScenarioFlagAndPolicy(t *testing.T) {
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Births = true
	_, rootPath, _ := naturalCaptureRoot(t, 0, false, 1)
	branchPath := filepath.Join(t.TempDir(), "reject-branch.bundle")
	if _, err := BranchNaturalCheckpoint(rootPath, opts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	expectRejection := func(name string, bundle string, expected NaturalRestoreOptions) {
		t.Helper()
		if f, _, err := RestoreNaturalCheckpoint(bundle, expected); f != nil || !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("%s: restored=%v err=%v", name, f, err)
		}
	}
	// Wrong declared scenario or worker count.
	for _, wrong := range []NaturalRestoreOptions{
		{Patches: 2, R: 7, Seed: 0, Workers: 1},
		{Patches: 2, R: 8, Seed: 7, Workers: 1},
		{Patches: 3, R: 8, Seed: 0, Workers: 1},
		{Patches: 2, R: 8, Seed: 0, Workers: 0},
		{Patches: 2, R: 8, Seed: 0, Workers: 1, FounderTenure: true},
	} {
		expectRejection(fmt.Sprintf("wrong-options-%+v", wrong), rootPath, wrong)
	}
	// Flag after the fact: a births-enabled continuation requires a recorded
	// branch.
	expectRejection("flag-after-fact", rootPath, enabledOpts)
	// Rewriting the manifest flag alone cannot forge an intervention: the
	// journal configuration no longer matches.
	forgedFlag := naturalRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		naturalEditManifest(t, s, func(m *naturalManifest) { m.births = true })
	})
	expectRejection("forged-manifest-flag", forgedFlag, enabledOpts)
	// Forging the fixture flag (or the raw flag byte) fails manifest decode
	// or run-identity validation.
	forgedTenure := naturalRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		naturalEditManifest(t, s, func(m *naturalManifest) { m.tenure = true })
	})
	expectRejection("forged-manifest-tenure", forgedTenure, opts)
	rawFlag := naturalRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		raw := s[2].Data
		if len(raw) <= naturalManifestFlagOffset {
			t.Fatalf("manifest layout changed: len=%d", len(raw))
		}
		raw[naturalManifestFlagOffset] = 2
		sum := sha256.Sum256(raw[:len(raw)-sha256.Size])
		copy(raw[len(raw)-sha256.Size:], sum[:])
	})
	expectRejection("raw-flag-byte", rawFlag, opts)
	// A branch bundle into which the parent's flag-false journal is spliced
	// back (with a matching manifest hash) still fails the config match.
	spliced := naturalRewriteBundle(t, branchPath, func(s []checkpoint.Section) {
		parent, _, err := checkpoint.ReadFile(rootPath)
		if err != nil {
			t.Fatal(err)
		}
		s[0].Data = bytes.Clone(parent[0].Data)
		naturalEditManifest(t, s, func(m *naturalManifest) { m.sections[0] = sha256.Sum256(s[0].Data) })
	})
	expectRejection("spliced-parent-journal", spliced, enabledOpts)
	// Manifest field forgeries with a valid manifest digest.
	for name, edit := range map[string]func(*naturalManifest){
		"patches":     func(m *naturalManifest) { m.patches = 1 },
		"budget":      func(m *naturalManifest) { m.r = 7 },
		"seed":        func(m *naturalManifest) { m.seed = 7 },
		"steps":       func(m *naturalManifest) { m.steps = 2 },
		"time":        func(m *naturalManifest) { m.time = sim.SimTime(NaturalHour) },
		"parent":      func(m *naturalManifest) { m.parent = [32]byte{1} },
		"depth":       func(m *naturalManifest) { m.depth = 1 },
		"depth-huge":  func(m *naturalManifest) { m.depth = naturalMaxBranchDepth + 1 },
		"fingerprint": func(m *naturalManifest) { m.policyFingerprint[0] ^= 1 },
		"boundary":    func(m *naturalManifest) { m.boundary = true },
	} {
		forged := naturalRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
			naturalEditManifest(t, s, edit)
		})
		expectRejection("manifest-"+name, forged, opts)
	}
	// Tampered policy content with a rehashed manifest is not a fork: the
	// pinned frozen policy must verify against the independent constructor.
	// The budget byte decodes as a canonical same-ref policy with budget
	// 31/32 — semantically drifted, structurally valid — and fails closed.
	drifted := naturalRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		policy, _, err := decodeNaturalStrategySection(s[4].Data)
		if err != nil {
			t.Fatal(err)
		}
		policyOffset := 4 + 4 + 2
		if !bytes.Equal(s[4].Data[policyOffset:policyOffset+len(policy)], policy) {
			t.Fatal("strategy layout changed")
		}
		decoded, err := strategy.DecodeNaturalPolicy(policy)
		if err != nil {
			t.Fatal(err)
		}
		decoded.Budget.Evaluations--
		changed, err := strategy.EncodeNaturalPolicy(decoded)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt := append(bytes.Clone(s[4].Data[:policyOffset]), changed...)
		rebuilt = append(rebuilt, s[4].Data[policyOffset+len(policy):]...)
		s[4].Data = rebuilt
		if _, _, err := decodeNaturalStrategySection(s[4].Data); err != nil {
			t.Fatalf("forged strategy section is not structurally valid: %v", err)
		}
		naturalEditManifest(t, s, func(m *naturalManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("tampered-policy", drifted, opts)
	// A forged extra binding (a newborn slot without genome rows) fails the
	// dynamic-table cross-check: no bindings without genome rows. The forged
	// actor sits above the fixture's fixed blocks, so only the exact
	// genome-row comparison can reject it.
	fixtureExpected := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 3, Workers: 1, Births: true, FounderTenure: true}
	fixture := naturalJournalFixture(t, 3, true, 1, 5)
	fixturePath := filepath.Join(t.TempDir(), "fixture-resume.bundle")
	if _, err := fixture.SaveResumeCheckpoint(fixturePath); err != nil {
		t.Fatal(err)
	}
	genesisLast, err := NaturalHighestFixedEntityID(2)
	if err != nil {
		t.Fatal(err)
	}
	extraBinding := naturalRewriteBundle(t, fixturePath, func(s []checkpoint.Section) {
		s[4].Data = forgeNaturalBindings(t, s[4].Data,
			naturalStrategyBinding{actor: genesisLast + 50, refVersion: NaturalRuleVersion})
		naturalEditManifest(t, s, func(m *naturalManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("binding-without-genome-row", extraBinding, fixtureExpected)
	// A truncated binding table fails likewise: every genome row must be bound.
	shortBinding := naturalRewriteBundle(t, fixturePath, func(s []checkpoint.Section) {
		s[4].Data = forgeNaturalBindings(t, s[4].Data)
		naturalEditManifest(t, s, func(m *naturalManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("genome-row-without-binding", shortBinding, fixtureExpected)
	// An unsorted (duplicate-actor) binding table fails structurally.
	unsorted := naturalRewriteBundle(t, fixturePath, func(s []checkpoint.Section) {
		_, bindings, err := decodeNaturalStrategySection(s[4].Data)
		if err != nil || len(bindings) < 2 {
			t.Fatalf("binding table shape: %v", err)
		}
		bindings[0], bindings[1] = bindings[1], bindings[0]
		s[4].Data = encodeNaturalBindingTable(strategyBytesOf(t, s[4].Data), bindings)
		naturalEditManifest(t, s, func(m *naturalManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("unsorted-bindings", unsorted, fixtureExpected)
	// The untouched bundles still restore after every rejection.
	if f, got, err := RestoreNaturalCheckpoint(rootPath, opts); err != nil || f == nil || got == [32]byte{} {
		t.Fatalf("valid bundle no longer restores: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(branchPath, enabledOpts); err != nil {
		t.Fatalf("branch bundle no longer restores: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(fixturePath, fixtureExpected); err != nil {
		t.Fatalf("fixture bundle no longer restores: %v", err)
	}
}

// strategyBytesOf returns the pinned policy bytes of an existing strategy
// section.
func strategyBytesOf(t *testing.T, data []byte) []byte {
	t.Helper()
	policy, _, err := decodeNaturalStrategySection(data)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func encodeNaturalBindingTable(policy []byte, bindings []naturalStrategyBinding) []byte {
	out := append([]byte(nil), naturalStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, naturalStrategyFormat)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policy)))
	out = append(out, policy...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(bindings)))
	for _, b := range bindings {
		out = binary.BigEndian.AppendUint64(out, uint64(b.actor))
		out = binary.BigEndian.AppendUint32(out, b.refVersion)
	}
	return out
}

// forgeNaturalBindings rewrites the strategy section's binding table: with
// no extra bindings the table loses its last record (a genome row left
// unbound); with one, the added actor gains a record without genome rows.
func forgeNaturalBindings(t *testing.T, data []byte, extra ...naturalStrategyBinding) []byte {
	t.Helper()
	policy, bindings, err := decodeNaturalStrategySection(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) == 0 && len(bindings) > 0 {
		bindings = bindings[:len(bindings)-1]
	}
	bindings = append(bindings, extra...)
	return encodeNaturalBindingTable(policy, bindings)
}

// TestNaturalCheckpointRejectsWakeTopology proves the exact wake-topology
// expectation sets (v5 pulse/claim wakes plus the reserved WakeBirth class,
// no supersets): every altered-but-generically-valid queue fails the
// neutral protocol validator and the full restore.
func TestNaturalCheckpointRejectsWakeTopology(t *testing.T) {
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1}
	f, path, _ := naturalCaptureRoot(t, 0, false, 1)
	claim0, err := NaturalPhaseTime(0, NaturalPhaseClaim)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*scheduler.Snapshot)
	}{
		{"missing claim wake", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == 1 && s.Wakes[i].Cause == scheduler.WakeNeedThreshold {
					s.Wakes = append(s.Wakes[:i], s.Wakes[i+1:]...)
					return
				}
			}
			t.Fatal("no claim wake")
		}},
		{"extra audit wake", func(s *scheduler.Snapshot) {
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 2, At: 2, Cause: scheduler.WakeAudit})
		}},
		{"wrong claim time", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == 1 && s.Wakes[i].Cause == scheduler.WakeNeedThreshold {
					s.Wakes[i].At = 2
					return
				}
			}
			t.Fatal("no claim wake")
		}},
		{"missing clock pulse wake", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == naturalClockActor {
					s.Wakes = append(s.Wakes[:i], s.Wakes[i+1:]...)
					return
				}
			}
			t.Fatal("no clock wake")
		}},
		{"live actor claim wakes dropped", func(s *scheduler.Snapshot) {
			kept := s.Wakes[:0]
			for _, wake := range s.Wakes {
				if wake.Actor != 16 {
					kept = append(kept, wake)
				}
			}
			s.Wakes = kept
		}},
		{"duplicate claim wake", func(s *scheduler.Snapshot) {
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: naturalClockActor, At: claim0, Cause: scheduler.WakeNeedThreshold})
		}},
		{"all claim wakes late", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Cause == scheduler.WakeNeedThreshold {
					s.Wakes[i].At = 2
				}
			}
		}},
		{"forged birth wake at h0", func(s *scheduler.Snapshot) {
			// A reserved WakeBirth claim on a registered founder at h0: no
			// newborn exists, so no reservation may exist.
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 2, At: claim0, Cause: scheduler.WakeBirth})
		}},
		{"claim wake relabeled as birth wake", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == 5 && s.Wakes[i].Cause == scheduler.WakeNeedThreshold {
					s.Wakes[i].Cause = scheduler.WakeBirth
					return
				}
			}
			t.Fatal("no claim wake")
		}},
		{"clock pulse wake relabeled as birth wake", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == naturalClockActor {
					s.Wakes[i].Cause = scheduler.WakeBirth
					return
				}
			}
			t.Fatal("no clock wake")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, wire := naturalAlterPortable(t, f, tc.change)
			if err := verifyNaturalNeutralScheduler(f.k, f.Journal(), changed.Snapshot(), f.births, f.patches); !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatalf("neutral topology validator did not reject test state: %v", err)
			}
			forged := naturalRewriteBundle(t, path, func(s []checkpoint.Section) {
				s[3].Data = wire
				naturalEditManifest(t, s, func(m *naturalManifest) { m.sections[2] = sha256.Sum256(wire) })
			})
			// The journal-aware decoder alone accepts a valid generic portable
			// queue; only the exact neutral protocol check rejects it.
			if r, _, err := RestoreNaturalCheckpoint(forged, opts); r != nil || !errors.Is(err, ErrNaturalCheckpoint) {
				t.Fatalf("accepted invalid topology: runner=%v err=%v", r, err)
			}
		})
	}
	// Publication refuses a runner whose live scheduler no longer matches the
	// neutral protocol, and publishes nothing.
	changed, _ := naturalAlterPortable(t, f, func(s *scheduler.Snapshot) {
		s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 2, At: 2, Cause: scheduler.WakeAudit})
	})
	f.sched = changed
	absentDir := t.TempDir()
	if digest, err := f.SaveCheckpoint(filepath.Join(absentDir, "unpublished.bundle")); digest != [32]byte{} || !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("published invalid topology: digest=%x err=%v", digest, err)
	}
	if entries, err := os.ReadDir(absentDir); err != nil || len(entries) != 0 {
		t.Fatalf("partial publication: entries=%v err=%v", entries, err)
	}
}

func TestNaturalCheckpointRefusesDriftAndNeverClobbers(t *testing.T) {
	opts := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Births = true
	f, path, digest := naturalCaptureRoot(t, 0, false, 1)
	wireBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A failed no-clobber publication never replaces an existing complete file.
	if _, err := f.SaveCheckpoint(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("clobbered existing destination: %v", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(wireBytes, unchanged) {
		t.Fatal("existing bundle modified")
	}
	for _, e := range []string{"bad-dir/natural.bundle", filepath.Join(t.TempDir(), ".checkpoint-forbidden")} {
		if _, err := f.SaveCheckpoint(e); err == nil {
			t.Fatalf("published unsafe path %s", e)
		}
	}
	// The projection reconstruction guards the save: a tampered hourly
	// projection no longer matches the history-derived trace, so nothing is
	// published.
	originalChecks := f.checkpoints
	tampered := append([]NaturalCheckpoint(nil), originalChecks...)
	tampered[0].Metrics.Alive = 3
	f.checkpoints = tampered
	absentProjection := filepath.Join(t.TempDir(), "tampered-projection.bundle")
	if saved, err := f.SaveCheckpoint(absentProjection); saved != [32]byte{} || !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("published a tampered projection: digest=%x err=%v", saved, err)
	}
	if _, err := os.Stat(absentProjection); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial publication: %v", err)
	}
	f.checkpoints = originalChecks
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later. The bound table is a map, so the revert must rebuild it —
	// a header copy would alias the mutated entries.
	original := make(map[sim.EntityID]*strategy.NaturalBound, len(f.bound))
	for id, bound := range f.bound {
		original[id] = bound
	}
	registry := strategy.NewNaturalRegistry(2)
	changed := f.bound[1].Policy()
	changed.Budget.Candidates--
	if err := registry.Register(changed); err != nil {
		t.Fatal(err)
	}
	for id := range f.bound {
		rebound, err := registry.Bind(strategy.NaturalBinding{Actor: id, Ref: changed.Ref})
		if err != nil {
			t.Fatal(err)
		}
		f.bound[id] = rebound
	}
	absent := filepath.Join(t.TempDir(), "altered-natural-policy.bundle")
	if saved, err := f.SaveCheckpoint(absent); saved != [32]byte{} || !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("published altered policy: digest=%x err=%v", saved, err)
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial publication: %v", err)
	}
	f.bound = original
	// The untouched runner still saves and restores with a stable digest.
	if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "again.bundle")); err != nil {
		t.Fatal(err)
	}
	if restored, got, err := RestoreNaturalCheckpoint(path, opts); err != nil || got != digest || restored == nil {
		t.Fatalf("valid bundle restore: %x %v", got, err)
	}
	if _, _, err := RestoreNaturalCheckpoint(path, enabledOpts); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("births-enabled restore accepted a disabled root bundle")
	}
}

// TestNaturalCheckpointLedgerAndAllocatorRestore pins the P15 ledger and the
// allocator discipline across restore: the hourly headroom rows round trip
// exactly, a lowered-threshold run never becomes a bundle, and the restored
// allocator's last ID is the max roster ID.
func TestNaturalCheckpointLedgerAndAllocatorRestore(t *testing.T) {
	expected := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 3, Workers: 1, Births: true, FounderTenure: true}
	f := naturalJournalFixture(t, 3, true, 1, 5)
	resumePath := filepath.Join(t.TempDir(), "ledger-resume.bundle")
	if _, err := f.SaveResumeCheckpoint(resumePath); err != nil {
		t.Fatal(err)
	}
	restored := restoredNatural(t, resumePath, expected)
	if !reflect.DeepEqual(restored.Checkpoints(), f.Checkpoints()) {
		t.Fatal("hourly ledger rows diverged across restore")
	}
	last := f.Checkpoints()
	tip := last[len(last)-1]
	if tip.AllocatorLast != tip.Newborns[len(tip.Newborns)-1].ID {
		t.Fatalf("allocator last %d is not the max roster ID %d", tip.AllocatorLast, tip.Newborns[len(tip.Newborns)-1].ID)
	}
	if restored.allocator.Last() != tip.AllocatorLast {
		t.Fatalf("restored allocator last %d", restored.allocator.Last())
	}
	// A lowered-threshold run halts fail-loud and never becomes a bundle.
	lowered, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 3, Workers: 1,
		CeilingThresholds: map[NaturalCeiling]int64{NaturalCeilingHistoryEvents: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := lowered.Step(context.Background()); err == nil || ok {
		t.Fatalf("lowered threshold did not halt: %v %v", ok, err)
	}
	var hitErr *NaturalCeilingHitError
	if !errors.As(lowered.halted, &hitErr) {
		t.Fatalf("halt is not a typed ceiling hit: %v", lowered.halted)
	}
	if _, err := lowered.SaveCheckpoint(filepath.Join(t.TempDir(), "lowered.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("published a bundle from a lowered-threshold runner")
	}
}
