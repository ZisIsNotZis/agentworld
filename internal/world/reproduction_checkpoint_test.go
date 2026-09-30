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

// reproductionManifestFlagOffset is the manifest offset of the raw births
// flag byte: magic + 11 u32 constants + 21 u64 constants + yield + seed.
const reproductionManifestFlagOffset = 4 + 11*4 + 21*8 + 8 + 8

func reproductionCaptureNeutral(t *testing.T, options ReproductionOptions) (*Reproduction, string, [32]byte) {
	t.Helper()
	f := mustReproductionAtH0(t, options)
	if f.steps != 1 || f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 || len(f.Journal()[0].Attempts) != 0 {
		t.Fatalf("not a neutral h0 pulse boundary: steps=%d time=%d checkpoints=%d", f.steps, f.sched.Time(), len(f.checkpoints))
	}
	path := filepath.Join(t.TempDir(), "reproduction-neutral-h0.bundle")
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	return f, path, digest
}

func reproductionRestoreStepToHour(t *testing.T, f *Reproduction, hour int) {
	t.Helper()
	for len(f.Checkpoints()) <= hour {
		ok, err := f.Step(context.Background())
		if err != nil || !ok {
			t.Fatalf("hour %d: processed=%t err=%v", len(f.Checkpoints()), ok, err)
		}
	}
}

func mustReproductionHandoff(t *testing.T, f *Reproduction) ReproductionHandoff {
	t.Helper()
	h, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func reproductionEditManifest(t *testing.T, sections []checkpoint.Section, edit func(*reproductionManifest)) {
	t.Helper()
	m, err := decodeReproductionManifest(sections[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	edit(&m)
	sections[2].Data = encodeReproductionManifest(m)
}

func reproductionRewriteBundle(t *testing.T, path string, edit func([]checkpoint.Section)) string {
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
	dest := filepath.Join(t.TempDir(), "forged-reproduction.bundle")
	if _, err := checkpoint.WriteFile(dest, sections); err != nil {
		t.Fatal(err)
	}
	return dest
}

// reproductionAlterPortable exports an altered-but-generically-valid portable
// scheduler state: the generic scheduler accepts the change, so only the
// reproduction-specific protocol checks can reject it.
func reproductionAlterPortable(t *testing.T, f *Reproduction, change func(*scheduler.Snapshot)) (*scheduler.Scheduler, []byte) {
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

func TestReproductionCheckpointNeutralCaptureRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts ReproductionOptions
	}{{"disabled", ReproductionOptions{Yield: 3, Seed: 0, Workers: 4}},
		{"births-enabled", ReproductionOptions{Yield: 3, Seed: 0, Workers: 1, Births: true}}} {
		t.Run(tc.name, func(t *testing.T) {
			f, path, digest := reproductionCaptureNeutral(t, tc.opts)
			before, err := f.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			restored, got, err := RestoreReproductionCheckpoint(path, tc.opts)
			if err != nil || got != digest {
				t.Fatalf("restore: digest=%x want=%x err=%v", got, digest, err)
			}
			after, err := restored.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("handoff did not round trip through the neutral bundle")
			}
			if !reflect.DeepEqual(f.Checkpoints(), restored.Checkpoints()) || !reflect.DeepEqual(f.Journal(), restored.Journal()) {
				t.Fatal("h0 projection or journal changed across restore")
			}
			// The bundle does not pin the worker count: any count restores.
			for _, workers := range []int{1, 2, 4} {
				rebind := tc.opts
				rebind.Workers = workers
				if _, _, err := RestoreReproductionCheckpoint(path, rebind); err != nil {
					t.Fatalf("worker count %d refused: %v", workers, err)
				}
			}
			sections, _, err := checkpoint.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := decodeReproductionManifest(sections[2].Data)
			if err != nil {
				t.Fatal(err)
			}
			wantFounders, err := expectedReproductionFounderGenomes(tc.opts.Seed)
			if err != nil {
				t.Fatal(err)
			}
			if m.depth != 0 || m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) ||
				m.births != tc.opts.Births || m.yield != tc.opts.Yield || m.seed != tc.opts.Seed ||
				m.steps != 1 || m.time != 0 || !reflect.DeepEqual(m.founders, wantFounders) {
				t.Fatalf("root manifest lineage/scenario wrong: %+v", m)
			}
			fingerprint, err := strategy.ReproductionPolicyFingerprint(strategy.FrozenReproductionPolicy())
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
			// a resume bundle is refused at the h0 boundary by definition.
			reproductionRestoreStepToHour(t, f, 1)
			if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "late.bundle")); !errors.Is(err, ErrReproductionCheckpoint) {
				t.Fatalf("published non-neutral boundary: %v", err)
			}
			h0 := mustReproductionAtH0(t, tc.opts)
			if _, err := h0.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "h0-resume.bundle")); !errors.Is(err, ErrReproductionCheckpoint) {
				t.Fatal("published a resume bundle at the neutral root boundary")
			}
		})
	}
}

func TestReproductionCheckpointBirthsAsymmetryAndBranchRecordsIntervention(t *testing.T) {
	opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 4}
	enabledOpts := opts
	enabledOpts.Births = true
	_, disabledPath, disabledDigest := reproductionCaptureNeutral(t, opts)
	_, enabledPath, enabledDigest := reproductionCaptureNeutral(t, enabledOpts)
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
	// The v4 h0 pulse is births-asymmetric in wake topology by frozen design
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
	branchDigest, err := BranchReproductionCheckpoint(disabledPath, opts, true, branchPath)
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
	m, err := decodeReproductionManifest(branchSections[2].Data)
	if err != nil || !m.births || m.depth != 1 || m.parent != disabledDigest || m.parentHead != m.head {
		t.Fatalf("branch lineage not recorded: %+v err=%v", m, err)
	}
	branchRunner, _, err := RestoreReproductionCheckpoint(branchPath, enabledOpts)
	if err != nil {
		t.Fatal(err)
	}
	if !branchRunner.births {
		t.Fatal("restored run does not match the intervention flag")
	}
	// The restored child's journal configuration matches the recorded
	// intervention. ExportJournal's scheduler-aware reconciliation replays
	// under the runner's own flag, which cannot reproduce the parent-boundary
	// portable bytes of a branch child, so the config match is proven through
	// the history-aware decode against the child's own verified head.
	branchHead := reproductionVerifiedHead(t, branchRunner)
	interventionConfig := branchRunner.journalConfig()
	interventionConfig.Boundary = m.boundary
	if batches, err := DecodeReproductionJournal(branchSections[0].Data, branchRunner.k, branchHead, branchRunner.sched.Time(), interventionConfig); err != nil || len(batches) == 0 {
		t.Fatalf("restored journal config does not match the recorded intervention: %v", err)
	}
	// A same-flag lineage link is recorded as such and still restores.
	samePath := filepath.Join(t.TempDir(), "intervention-same.bundle")
	if _, err := BranchReproductionCheckpoint(disabledPath, opts, false, samePath); err != nil {
		t.Fatal(err)
	}
	sameSections, _, err := checkpoint.ReadFile(samePath)
	if err != nil {
		t.Fatal(err)
	}
	if m, err := decodeReproductionManifest(sameSections[2].Data); err != nil || m.births || m.depth != 1 || m.parent != disabledDigest {
		t.Fatalf("same-flag intervention not recorded: %+v err=%v", m, err)
	}
	// The disabling direction is refused before publication: an enabled
	// root's h0 boundary carries pending request wakes that a births-disabled
	// continuation would refuse to evaluate, so such a child could never run.
	reverseDir := t.TempDir()
	reversePath := filepath.Join(reverseDir, "intervention-disabled.bundle")
	if _, err := BranchReproductionCheckpoint(enabledPath, enabledOpts, false, reversePath); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("disabled a births-enabled lineage: %v", err)
	}
	if entries, err := os.ReadDir(reverseDir); err != nil || len(entries) != 0 {
		t.Fatalf("refused branch left entries behind: %v %v", entries, err)
	}
}

func TestReproductionCheckpointBranchForkMatchesBaselines(t *testing.T) {
	for _, workers := range []int{1, 4} {
		t.Run(fmt.Sprintf("q8-w%d", workers), func(t *testing.T) {
			opts := ReproductionOptions{Yield: 8, Seed: 0, Workers: workers}
			enabledOpts := opts
			enabledOpts.Births = true
			const hour = 6 // the q8/seed0 fixture commits its first birth at h4
			baselineDisabled := reproductionRunTo(t, opts, hour)
			// The intervention baseline applies the recorded decision at the
			// same verified boundary the branch child continues from: the h0
			// pulse ran births-disabled, so the h0 request window is absent.
			baselineEnabled := mustReproductionAtH0(t, opts)
			baselineEnabled.births = true
			reproductionRestoreStepToHour(t, baselineEnabled, hour)
			_, rootPath, rootDigest := reproductionCaptureNeutral(t, opts)
			branchPath := filepath.Join(t.TempDir(), "fork-births.bundle")
			if _, err := BranchReproductionCheckpoint(rootPath, opts, true, branchPath); err != nil {
				t.Fatal(err)
			}
			restoredDisabled, gotRoot, err := RestoreReproductionCheckpoint(rootPath, opts)
			if err != nil || gotRoot != rootDigest {
				t.Fatalf("root restore: %x %v", gotRoot, err)
			}
			restoredEnabled, _, err := RestoreReproductionCheckpoint(branchPath, enabledOpts)
			if err != nil {
				t.Fatalf("branch restore: %v", err)
			}
			reproductionRestoreStepToHour(t, restoredDisabled, hour)
			reproductionRestoreStepToHour(t, restoredEnabled, hour)
			if !reflect.DeepEqual(mustReproductionHandoff(t, restoredDisabled), mustReproductionHandoff(t, baselineDisabled)) {
				t.Fatal("restored disabled run diverged from uninterrupted same-flag run")
			}
			if !reflect.DeepEqual(mustReproductionHandoff(t, restoredEnabled), mustReproductionHandoff(t, baselineEnabled)) {
				t.Fatal("restored births branch diverged from the intervention baseline")
			}
			if !reflect.DeepEqual(reproductionEventBytes(t, restoredEnabled), reproductionEventBytes(t, baselineEnabled)) ||
				!reflect.DeepEqual(reproductionEventBytes(t, restoredDisabled), reproductionEventBytes(t, baselineDisabled)) {
				t.Fatal("accepted event bytes diverged from same-flag baselines")
			}
			enabledJournal, disabledJournal := baselineEnabled.Journal(), baselineDisabled.Journal()
			birthEvidence := func(batches []ReproductionBatch) int {
				count := 0
				for _, b := range batches {
					for _, a := range b.Attempts {
						if a.Kind == "birth-request" || a.Kind == "birth-reply" {
							count++
						}
					}
				}
				return count
			}
			if birthEvidence(disabledJournal) != 0 {
				t.Fatal("disabled branch produced birth evidence")
			}
			if birthEvidence(enabledJournal) == 0 {
				t.Fatal("births branch produced no birth evidence")
			}
			if baselineEnabled.Checkpoints()[hour].Metrics.Births == 0 {
				t.Fatalf("births branch produced no births by h%d", hour)
			}
			diff := -1
			for i := range enabledJournal {
				if !reflect.DeepEqual(enabledJournal[i], disabledJournal[i]) {
					diff = i
					break
				}
			}
			if diff <= 0 {
				t.Fatalf("first differing batch %d is not after the shared h0 prefix", diff)
			}
			for i := 0; i < diff; i++ {
				if !reflect.DeepEqual(enabledJournal[i], disabledJournal[i]) {
					t.Fatalf("shared prefix broken at batch %d", i)
				}
			}
		})
	}
}

func TestReproductionCheckpointBranchDepthBound(t *testing.T) {
	opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}
	_, root, rootDigest := reproductionCaptureNeutral(t, opts)
	// One depth-1 child for lineage forgery tests, then chain to the frozen
	// bound: depth 63 is restorable, depth 64 is refused before publication
	// because it could never be restored.
	child := filepath.Join(t.TempDir(), "depth-1.bundle")
	childDigest, err := BranchReproductionCheckpoint(root, opts, false, child)
	if err != nil {
		t.Fatal(err)
	}
	current := child
	for i := 2; i <= reproductionMaxBranchDepth; i++ {
		next := filepath.Join(t.TempDir(), fmt.Sprintf("depth-%d.bundle", i))
		if _, err := BranchReproductionCheckpoint(current, opts, false, next); err != nil {
			t.Fatalf("branch to depth %d: %v", i, err)
		}
		current = next
	}
	if _, _, err := RestoreReproductionCheckpoint(current, opts); err != nil {
		t.Fatalf("depth-63 bundle refused: %v", err)
	}
	deepDir := t.TempDir()
	tooDeep := filepath.Join(deepDir, "too-deep.bundle")
	if _, err := BranchReproductionCheckpoint(current, opts, false, tooDeep); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("published an unrestorable depth: %v", err)
	}
	if entries, err := os.ReadDir(deepDir); err != nil || len(entries) != 0 {
		t.Fatalf("over-deep branch left entries behind: %v %v", entries, err)
	}
	// A forged manifest claiming over-deep ancestry is rejected at decode.
	forged := reproductionRewriteBundle(t, current, func(s []checkpoint.Section) {
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.depth = reproductionMaxBranchDepth + 1 })
	})
	if f, _, err := RestoreReproductionCheckpoint(forged, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("accepted over-deep lineage: %v", err)
	}
	// Forged lineage links fail closed: a root capture must record no parent,
	// and a branch child must continue its parent at the same verified head.
	badRoot := reproductionRewriteBundle(t, root, func(s []checkpoint.Section) {
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.parent = childDigest })
	})
	if f, _, err := RestoreReproductionCheckpoint(badRoot, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("root bundle accepted a forged parent: %v", err)
	}
	if rootDigest == childDigest {
		t.Fatal("root and child digests must differ")
	}
	for name, edit := range map[string]func(*reproductionManifest){
		"parent-head": func(m *reproductionManifest) { m.parentHead.TipID++ },
		"depth-zero":  func(m *reproductionManifest) { m.depth = 0 },
	} {
		bad := reproductionRewriteBundle(t, child, func(s []checkpoint.Section) {
			reproductionEditManifest(t, s, edit)
		})
		if f, _, err := RestoreReproductionCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("forged %s accepted: %v", name, err)
		}
	}
}

func TestReproductionCheckpointRejectsDamageTruncationSwapAndCrossVersion(t *testing.T) {
	opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}
	_, path, _ := reproductionCaptureNeutral(t, opts)
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
		corrupt := filepath.Join(t.TempDir(), "corrupt-reproduction.bundle")
		if _, err := checkpoint.WriteFile(corrupt, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreReproductionCheckpoint(corrupt, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("accepted rehashed section %d: %v", i, err)
		}
	}
	// Swapped section payloads are caught by the manifest section hashes.
	for _, swap := range [][2]int{{0, 1}, {3, 4}} {
		bad := append([]checkpoint.Section(nil), sections...)
		bad[swap[0]].Data, bad[swap[1]].Data = bytes.Clone(bad[swap[1]].Data), bytes.Clone(bad[swap[0]].Data)
		swapped := filepath.Join(t.TempDir(), "swapped-reproduction.bundle")
		if _, err := checkpoint.WriteFile(swapped, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreReproductionCheckpoint(swapped, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("accepted swapped sections %v: %v", swap, err)
		}
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{wire[:len(wire)-1], wire[:len(wire)/2], append(bytes.Clone(wire), 0)} {
		bad := filepath.Join(t.TempDir(), "truncated-reproduction.bundle")
		if err := os.WriteFile(bad, malformed, 0600); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreReproductionCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("accepted malformed bundle: %v", err)
		}
	}
	// v1/S6/v2/v3 bundles never decode as v4 and vice versa: no implicit
	// migration in either direction.
	_, foodflowPath, _ := foodFlowSaveAt(t, 3, 2)
	_, socialPath, _ := socialFoodCaptureNeutral(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4})
	s6, err := NewSurvival(SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	s6Path := filepath.Join(t.TempDir(), "s6-reproduction-reject.bundle")
	if _, err := s6.SaveCheckpoint(s6Path); err != nil {
		t.Fatal(err)
	}
	capacityPath := filepath.Join(t.TempDir(), "capacity-reproduction-reject.bundle")
	if _, err := mustCapacityAtH0(t, CapacityOptions{Yield: 3, Seed: 0, Workers: 1}).SaveCheckpoint(capacityPath); err != nil {
		t.Fatal(err)
	}
	for name, foreign := range map[string]string{"food-flow": foodflowPath, "social-food": socialPath, "s6": s6Path, "capacity": capacityPath} {
		if f, _, err := RestoreReproductionCheckpoint(foreign, opts); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("accepted foreign %s bundle: %v", name, err)
		}
	}
	v1opts := FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}
	if f, _, err := RestoreFoodFlowCheckpoint(path, v1opts); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("food-flow accepted v4 bundle: %v", err)
	}
	v2opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	if f, _, err := RestoreSocialFoodCheckpoint(path, v2opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("social-food accepted v4 bundle: %v", err)
	}
	s6opts := SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1}
	if f, _, err := RestoreSurvivalCheckpoint(path, s6opts); f != nil || !errors.Is(err, ErrSurvivalCheckpoint) {
		t.Fatalf("S6 accepted v4 bundle: %v", err)
	}
	v3opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	if f, _, err := RestoreCapacityCheckpoint(path, v3opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("capacity accepted v4 bundle: %v", err)
	}
}

func TestReproductionCheckpointRejectsForgedScenarioFlagAndPolicy(t *testing.T) {
	opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Births = true
	_, rootPath, _ := reproductionCaptureNeutral(t, opts)
	branchPath := filepath.Join(t.TempDir(), "reject-branch.bundle")
	if _, err := BranchReproductionCheckpoint(rootPath, opts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	expectRejection := func(name string, bundle string, expected ReproductionOptions) {
		t.Helper()
		if f, _, err := RestoreReproductionCheckpoint(bundle, expected); f != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("%s: restored=%v err=%v", name, f, err)
		}
	}
	// Wrong declared scenario or worker count.
	for _, wrong := range []ReproductionOptions{
		{Yield: 8, Seed: 0, Workers: 1},
		{Yield: 3, Seed: 7, Workers: 1},
		{Yield: 3, Seed: 0, Workers: 0},
	} {
		expectRejection(fmt.Sprintf("wrong-options-%+v", wrong), rootPath, wrong)
	}
	// Flag after the fact: a births-enabled continuation requires a recorded
	// branch.
	expectRejection("flag-after-fact", rootPath, enabledOpts)
	// Rewriting the manifest flag alone cannot forge an intervention: the
	// journal configuration no longer matches.
	forgedFlag := reproductionRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.births = true })
	})
	expectRejection("forged-manifest-flag", forgedFlag, enabledOpts)
	// A branch bundle into which the parent's flag-false journal is spliced
	// back (with a matching manifest hash) still fails the config match.
	spliced := reproductionRewriteBundle(t, branchPath, func(s []checkpoint.Section) {
		parent, _, err := checkpoint.ReadFile(rootPath)
		if err != nil {
			t.Fatal(err)
		}
		s[0].Data = bytes.Clone(parent[0].Data)
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.sections[0] = sha256.Sum256(s[0].Data) })
	})
	expectRejection("spliced-parent-journal", spliced, enabledOpts)
	// Manifest field forgeries with a valid manifest digest.
	for name, edit := range map[string]func(*reproductionManifest){
		"yield":          func(m *reproductionManifest) { m.yield = 8 },
		"seed":           func(m *reproductionManifest) { m.seed = 7 },
		"steps":          func(m *reproductionManifest) { m.steps = 2 },
		"time":           func(m *reproductionManifest) { m.time = sim.SimTime(ReproductionHour) },
		"parent":         func(m *reproductionManifest) { m.parent = [32]byte{1} },
		"depth":          func(m *reproductionManifest) { m.depth = 1 },
		"depth-huge":     func(m *reproductionManifest) { m.depth = reproductionMaxBranchDepth + 1 },
		"fingerprint":    func(m *reproductionManifest) { m.policyFingerprint[0] ^= 1 },
		"founder-locus":  func(m *reproductionManifest) { m.founders[0].Genome.LocusM = 7 },
		"founder-death":  func(m *reproductionManifest) { m.founders[1].Genome.DeathHour = 200 },
		"founder-parent": func(m *reproductionManifest) { m.founders[2].Genome.ParentA = 4 },
		"founder-drop":   func(m *reproductionManifest) { m.founders = m.founders[:15] },
	} {
		forged := reproductionRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
			reproductionEditManifest(t, s, edit)
		})
		expectRejection("manifest-"+name, forged, opts)
	}
	// A raw non-binary flag byte fails manifest decoding.
	rawFlag := reproductionRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		raw := s[2].Data
		if len(raw) <= reproductionManifestFlagOffset {
			t.Fatalf("manifest layout changed: len=%d", len(raw))
		}
		raw[reproductionManifestFlagOffset] = 2
		sum := sha256.Sum256(raw[:len(raw)-sha256.Size])
		copy(raw[len(raw)-sha256.Size:], sum[:])
	})
	expectRejection("raw-flag-byte", rawFlag, opts)
	// Tampered policy content with a rehashed manifest is not a fork: the
	// pinned frozen policy must verify against the independent constructor.
	// The budget byte decodes as a canonical same-ref policy with budget
	// 31/32 — semantically drifted, structurally valid — and fails closed.
	drifted := reproductionRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		policy, _, err := decodeReproductionStrategySection(s[4].Data)
		if err != nil {
			t.Fatal(err)
		}
		policyOffset := 4 + 4 + 2
		if !bytes.Equal(s[4].Data[policyOffset:policyOffset+len(policy)], policy) {
			t.Fatal("strategy layout changed")
		}
		decoded, err := strategy.DecodeReproductionPolicy(policy)
		if err != nil {
			t.Fatal(err)
		}
		decoded.Budget.Evaluations--
		changed, err := strategy.EncodeReproductionPolicy(decoded)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt := append(bytes.Clone(s[4].Data[:policyOffset]), changed...)
		rebuilt = append(rebuilt, s[4].Data[policyOffset+len(policy):]...)
		s[4].Data = rebuilt
		if _, _, err := decodeReproductionStrategySection(s[4].Data); err != nil {
			t.Fatalf("forged strategy section is not structurally valid: %v", err)
		}
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("tampered-policy", drifted, opts)
	// A forged extra binding (a newborn slot without genome rows) fails the
	// dynamic-table cross-check: no bindings without genome rows.
	extraBinding := reproductionRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		s[4].Data = forgeReproductionBindings(t, s[4].Data,
			reproductionStrategyBinding{actor: ReproductionFirstNewbornID, refVersion: ReproductionRuleVersion})
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("binding-without-genome-row", extraBinding, opts)
	// A truncated binding table fails likewise: every genome row must be bound.
	shortBinding := reproductionRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		s[4].Data = forgeReproductionBindings(t, s[4].Data)
		reproductionEditManifest(t, s, func(m *reproductionManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("genome-row-without-binding", shortBinding, opts)
	// The untouched bundles still restore after every rejection.
	if f, got, err := RestoreReproductionCheckpoint(rootPath, opts); err != nil || f == nil || got == [32]byte{} {
		t.Fatalf("valid bundle no longer restores: %v", err)
	}
	if _, _, err := RestoreReproductionCheckpoint(branchPath, enabledOpts); err != nil {
		t.Fatalf("branch bundle no longer restores: %v", err)
	}
}

// forgeReproductionBindings rewrites the strategy section's binding table:
// with no extra bindings the table loses its last record (a genome row left
// unbound); with one, the first newborn slot gains a record without genome
// rows.
func forgeReproductionBindings(t *testing.T, data []byte, extra ...reproductionStrategyBinding) []byte {
	t.Helper()
	policy, bindings, err := decodeReproductionStrategySection(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) == 0 && len(bindings) > 0 {
		bindings = bindings[:len(bindings)-1]
	}
	bindings = append(bindings, extra...)
	out := append([]byte(nil), reproductionStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, reproductionStrategyFormat)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policy)))
	out = append(out, policy...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(bindings)))
	for _, b := range bindings {
		out = binary.BigEndian.AppendUint64(out, uint64(b.actor))
		out = binary.BigEndian.AppendUint32(out, b.refVersion)
	}
	return out
}

func TestReproductionCheckpointRejectsWakeTopology(t *testing.T) {
	opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}
	f, path, _ := reproductionCaptureNeutral(t, opts)
	claim0, err := ReproductionPhaseTime(0, ReproductionPhaseClaim)
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
				if s.Wakes[i].Actor == reproductionClockActor {
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
			// A second claim-class wake on the clock fiber at the claim
			// instant: generically valid, wrong for the neutral protocol.
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: reproductionClockActor, At: claim0, Cause: scheduler.WakeNeedThreshold})
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, wire := reproductionAlterPortable(t, f, tc.change)
			if err := verifyReproductionNeutralScheduler(f.k, f.Journal(), changed.Snapshot(), f.births); !errors.Is(err, ErrReproductionCheckpoint) {
				t.Fatalf("neutral topology validator did not reject test state: %v", err)
			}
			forged := reproductionRewriteBundle(t, path, func(s []checkpoint.Section) {
				s[3].Data = wire
				reproductionEditManifest(t, s, func(m *reproductionManifest) { m.sections[2] = sha256.Sum256(wire) })
			})
			// The journal-aware decoder alone accepts a valid generic portable
			// queue; only the exact neutral protocol check rejects it.
			if r, _, err := RestoreReproductionCheckpoint(forged, opts); r != nil || !errors.Is(err, ErrReproductionCheckpoint) {
				t.Fatalf("accepted invalid topology: runner=%v err=%v", r, err)
			}
		})
	}
	// Publication refuses a runner whose live scheduler no longer matches the
	// neutral protocol, and publishes nothing.
	changed, _ := reproductionAlterPortable(t, f, func(s *scheduler.Snapshot) {
		s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 2, At: 2, Cause: scheduler.WakeAudit})
	})
	f.sched = changed
	absentDir := t.TempDir()
	if digest, err := f.SaveCheckpoint(filepath.Join(absentDir, "unpublished.bundle")); digest != [32]byte{} || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("published invalid topology: digest=%x err=%v", digest, err)
	}
	entries, err := os.ReadDir(absentDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial publication: entries=%v err=%v", entries, err)
	}
}

func TestReproductionCheckpointRefusesDriftAndNeverClobbers(t *testing.T) {
	opts := ReproductionOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Births = true
	f, path, digest := reproductionCaptureNeutral(t, opts)
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
	for _, e := range []string{"bad-dir/reproduction.bundle", filepath.Join(t.TempDir(), ".checkpoint-forbidden")} {
		if _, err := f.SaveCheckpoint(e); err == nil {
			t.Fatalf("published unsafe path %s", e)
		}
	}
	// The projection reconstruction guards the save: a tampered hourly
	// projection no longer matches the history-derived trace, so nothing is
	// published. The surgery swaps the runner's own slice (never the clone
	// semantics of Checkpoints, whose empty newborn slice is nil) and is
	// reverted by restoring the original header.
	originalChecks := f.checkpoints
	tampered := append([]ReproductionCheckpoint(nil), originalChecks...)
	tampered[0].Metrics.DistinctGenomes = 99
	f.checkpoints = tampered
	absentProjection := filepath.Join(t.TempDir(), "tampered-projection.bundle")
	if saved, err := f.SaveCheckpoint(absentProjection); saved != [32]byte{} || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("published a tampered projection: digest=%x err=%v", saved, err)
	}
	if _, err := os.Stat(absentProjection); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial publication: %v", err)
	}
	f.checkpoints = originalChecks
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later. The bound table is a map, so the revert must rebuild it —
	// a header copy would alias the mutated entries.
	original := make(map[sim.EntityID]*strategy.ReproductionBound, len(f.bound))
	for id, bound := range f.bound {
		original[id] = bound
	}
	registry := strategy.NewReproductionRegistry()
	changed := f.bound[1].Policy()
	changed.Budget.Candidates--
	if err := registry.Register(changed); err != nil {
		t.Fatal(err)
	}
	for id := range f.bound {
		rebound, err := registry.Bind(strategy.ReproductionBinding{Actor: id, Ref: changed.Ref})
		if err != nil {
			t.Fatal(err)
		}
		f.bound[id] = rebound
	}
	absent := filepath.Join(t.TempDir(), "altered-reproduction-policy.bundle")
	if saved, err := f.SaveCheckpoint(absent); saved != [32]byte{} || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("published altered policy: digest=%x err=%v", saved, err)
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial publication: %v", err)
	}
	f.bound = original
	// Branching demands the parent scenario and never clobbers.
	branchDir := t.TempDir()
	wrongScenario := opts
	wrongScenario.Yield = 8
	dest := filepath.Join(branchDir, "rejected-branch.bundle")
	if _, err := BranchReproductionCheckpoint(path, wrongScenario, true, dest); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("branched from a misdeclared parent: %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial branch publication: %v", err)
	}
	if _, err := BranchReproductionCheckpoint(path, opts, true, path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("branch clobbered the parent bundle: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(wireBytes, after) {
		t.Fatal("parent bundle modified by refused branch")
	}
	entries, err := os.ReadDir(branchDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("branch left entries behind: %v", entries)
	}
	// The untouched runner still saves and restores with a stable digest.
	if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "again.bundle")); err != nil {
		t.Fatal(err)
	}
	if restored, got, err := RestoreReproductionCheckpoint(path, opts); err != nil || got != digest || restored == nil {
		t.Fatalf("valid bundle restore: %x %v", got, err)
	}
	if _, _, err := RestoreReproductionCheckpoint(path, enabledOpts); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatal("births-enabled restore accepted a disabled root bundle")
	}
}

// TestReproductionCheckpointResumeBundle exercises the pulse-boundary
// continuation path end to end: capture after at least one birth, restore at
// a different worker count, byte-identical continuation against an
// uninterrupted baseline, and refusals for mid-hour, h0, forged-lineage, and
// mismatched-scenario captures.
func TestReproductionCheckpointResumeBundle(t *testing.T) {
	opts := ReproductionOptions{Yield: 8, Seed: 0, Workers: 4, Births: true}
	const (
		captureHour  = 6 // first q8/seed0 birth commits at h4
		continuation = 9
	)
	baseline := reproductionRunTo(t, opts, continuation)
	f := reproductionRunTo(t, opts, captureHour)
	if f.Checkpoints()[captureHour].Metrics.Births == 0 {
		t.Fatalf("fixture produced no births by h%d", captureHour)
	}
	resumePath := filepath.Join(t.TempDir(), "resume-h6.bundle")
	if _, err := f.SaveResumeCheckpoint(resumePath); err != nil {
		t.Fatal(err)
	}
	// The boundary handoff round trips exactly, including the dynamic roster,
	// the reserved birth wakes and the denial evidence.
	restored, digest, err := RestoreReproductionCheckpoint(resumePath, opts)
	if err != nil {
		t.Fatal(err)
	}
	if boundary, err := restored.Handoff(); err != nil || !reflect.DeepEqual(boundary, mustReproductionHandoff(t, f)) {
		t.Fatalf("resume boundary handoff diverged: %v", err)
	}
	if len(restored.Checkpoints()[captureHour].Newborns) == 0 {
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
	// Continuation at a worker count neither the capture (4) nor this restore
	// used must be byte-identical to the uninterrupted run.
	for _, workers := range []int{1, 2, 3} {
		continued, _, err := RestoreReproductionCheckpoint(resumePath, ReproductionOptions{Yield: opts.Yield, Seed: opts.Seed, Workers: workers, Births: true})
		if err != nil {
			t.Fatalf("workers=%d restore: %v", workers, err)
		}
		reproductionRestoreStepToHour(t, continued, continuation)
		if !reflect.DeepEqual(mustReproductionHandoff(t, continued), mustReproductionHandoff(t, baseline)) {
			t.Fatalf("workers=%d continuation diverged from the uninterrupted run", workers)
		}
		if !reflect.DeepEqual(reproductionEventBytes(t, continued), reproductionEventBytes(t, baseline)) {
			t.Fatalf("workers=%d continuation event bytes diverged", workers)
		}
	}
	// Re-saving the restored boundary reproduces the identical bundle.
	again := filepath.Join(t.TempDir(), "resume-resaved.bundle")
	if resaved, err := restored.SaveResumeCheckpoint(again); err != nil || resaved != digest {
		t.Fatalf("resume re-save digest=%x want=%x err=%v", resaved, digest, err)
	}
	// A capture between instants (in-flight hour) is refused: the claim step
	// leaves the runner mid-hour.
	midHour := reproductionRunTo(t, opts, captureHour)
	if ok, err := midHour.Step(context.Background()); err != nil || !ok {
		t.Fatalf("mid-hour step: %v %v", ok, err)
	}
	if _, err := midHour.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "midhour.bundle")); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatal("published a resume bundle mid-hour")
	}
	// A resume bundle is not a branch parent, and a continuation cannot be
	// captured at the neutral h0 boundary.
	branchDir := t.TempDir()
	if _, err := BranchReproductionCheckpoint(resumePath, opts, false, filepath.Join(branchDir, "nope.bundle")); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatal("branched from a continuation bundle")
	}
	if entries, err := os.ReadDir(branchDir); err != nil || len(entries) != 0 {
		t.Fatalf("refused branch left entries behind: %v %v", entries, err)
	}
	if _, err := mustReproductionAtH0(t, opts).SaveResumeCheckpoint(filepath.Join(t.TempDir(), "h0.bundle")); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatal("published a resume bundle at h0")
	}
	// A continuation restores only under its exact declared scenario.
	for _, wrong := range []ReproductionOptions{
		{Yield: 3, Seed: 0, Workers: 1, Births: true},
		{Yield: 8, Seed: 7, Workers: 1, Births: true},
		{Yield: 8, Seed: 0, Workers: 1},
		{Yield: 8, Seed: 0, Workers: 0, Births: true},
	} {
		if r, _, err := RestoreReproductionCheckpoint(resumePath, wrong); r != nil || !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("mismatched scenario %+v accepted: %v", wrong, err)
		}
	}
	// A forged lineage on a continuation bundle fails at decode.
	forged := reproductionRewriteBundle(t, resumePath, func(s []checkpoint.Section) {
		reproductionEditManifest(t, s, func(m *reproductionManifest) {
			m.parent = [32]byte{9}
			m.parentHead = m.head
			m.depth = 1
		})
	})
	if r, _, err := RestoreReproductionCheckpoint(forged, opts); r != nil || !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("forged continuation lineage accepted: %v", err)
	}
	// A forged wake topology on a continuation bundle fails the boundary
	// check: drop the reserved WakeBirth claim of a newborn born at h5.
	snap := f.sched.Snapshot()
	dropped := false
	kept := snap.Wakes[:0]
	for _, wake := range snap.Wakes {
		if wake.Cause == scheduler.WakeBirth && !dropped {
			dropped = true
			continue
		}
		kept = append(kept, wake)
	}
	if !dropped {
		t.Fatal("fixture has no pending WakeBirth reservation")
	}
	snap.Wakes = kept
	// Repair the fibers' earliest-wake pointers so the generic scheduler
	// accepts the state; only the reproduction boundary check may reject it.
	for i := range snap.Fibers {
		fiber := &snap.Fibers[i]
		fiber.HasNextWake, fiber.NextWake = false, 0
		for _, wake := range snap.Wakes {
			if wake.Actor == fiber.Actor && (!fiber.HasNextWake || wake.At < fiber.NextWake) {
				fiber.HasNextWake, fiber.NextWake = true, wake.At
			}
		}
	}
	altered, err := scheduler.Restore(f.k, 1, f.evaluate, snap)
	if err != nil {
		t.Fatalf("generic scheduler rejected test state: %v", err)
	}
	f.sched = altered
	if _, err := f.SaveResumeCheckpoint(filepath.Join(t.TempDir(), "bad-topology.bundle")); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatal("published a continuation with a dropped WakeBirth reservation")
	}
}
