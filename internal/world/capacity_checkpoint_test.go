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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func capacityCaptureNeutral(t *testing.T, options CapacityOptions) (*Capacity, string, [32]byte) {
	t.Helper()
	f := mustCapacityAtH0(t, options)
	if f.steps != 1 || f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 || len(f.Journal()[0].Attempts) != 0 {
		t.Fatalf("not a neutral h0 pulse boundary: steps=%d time=%d checkpoints=%d", f.steps, f.sched.Time(), len(f.checkpoints))
	}
	path := filepath.Join(t.TempDir(), "capacity-neutral-h0.bundle")
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	return f, path, digest
}

func capacityStepToHour(t *testing.T, f *Capacity, hour int) {
	t.Helper()
	for len(f.Checkpoints()) <= hour {
		ok, err := f.Step(context.Background())
		if err != nil || !ok {
			t.Fatalf("hour %d: processed=%t err=%v", len(f.Checkpoints()), ok, err)
		}
	}
}

func capacityRewriteBundle(t *testing.T, path string, edit func([]checkpoint.Section)) string {
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
	dest := filepath.Join(t.TempDir(), "forged-capacity.bundle")
	if _, err := checkpoint.WriteFile(dest, sections); err != nil {
		t.Fatal(err)
	}
	return dest
}

func capacityEditManifest(t *testing.T, sections []checkpoint.Section, edit func(*capacityManifest)) {
	t.Helper()
	m, err := decodeCapacityManifest(sections[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	edit(&m)
	sections[2].Data = encodeCapacityManifest(m)
}

func capacityAlterPortable(t *testing.T, f *Capacity, change func(*scheduler.Snapshot)) (*scheduler.Scheduler, []byte) {
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

// capacityManifestFlagOffset is the manifest offset of the raw intervention
// flag byte: magic + 11 u32 constants + 17 u64 constants + yield + seed.
const capacityManifestFlagOffset = 4 + 11*4 + 17*8 + 8 + 8

func TestCapacityCheckpointNeutralCaptureRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts CapacityOptions
	}{{"disabled", CapacityOptions{Yield: 3, Seed: 0, Workers: 4}},
		{"enabled", CapacityOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}},
		{"founder-B", CapacityOptions{Yield: 8, Seed: 0, Workers: 4, Enabled: true, Founders: []CapacityFounder{{Actor: 1, Granary: 4}}}}} {
		t.Run(tc.name, func(t *testing.T) {
			f, path, digest := capacityCaptureNeutral(t, tc.opts)
			before, err := f.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			restored, got, err := RestoreCapacityCheckpoint(path, tc.opts)
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
				if _, _, err := RestoreCapacityCheckpoint(path, rebind); err != nil {
					t.Fatalf("worker count %d refused: %v", workers, err)
				}
			}
			sections, _, err := checkpoint.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := decodeCapacityManifest(sections[2].Data)
			if err != nil {
				t.Fatal(err)
			}
			wantFounders := capacityNormalizeFounders(tc.opts.Founders)
			if m.depth != 0 || m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) ||
				!m.enabled != !tc.opts.Enabled || m.yield != tc.opts.Yield || m.seed != tc.opts.Seed ||
				m.steps != 1 || m.time != 0 || !reflect.DeepEqual(m.founders, wantFounders) {
				t.Fatalf("root manifest lineage/scenario wrong: %+v", m)
			}
			fingerprint, err := strategy.CapacityPolicyFingerprint(strategy.FrozenCapacityPolicy())
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
			// Saving refuses any boundary that is not the neutral h0 pulse.
			capacityStepToHour(t, f, 1)
			if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "late.bundle")); !errors.Is(err, ErrCapacityCheckpoint) {
				t.Fatalf("published non-neutral boundary: %v", err)
			}
		})
	}
}

func TestCapacityCheckpointCaptureIsFlagNeutralAndBranchRecordsIntervention(t *testing.T) {
	opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 4}
	enabledOpts := opts
	enabledOpts.Enabled = true
	_, disabledPath, disabledDigest := capacityCaptureNeutral(t, opts)
	_, enabledPath, enabledDigest := capacityCaptureNeutral(t, enabledOpts)
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
	// The world is flag-neutral at h0: only the journal configuration and the
	// manifest record the flag; history, scheduler and pinned policy bytes
	// are identical, so both flags share the same verified prefix.
	for _, index := range []int{1, 3, 4} {
		if !bytes.Equal(disabledSections[index].Data, enabledSections[index].Data) {
			t.Fatalf("flag leaked into section %d", index)
		}
	}
	if bytes.Equal(disabledSections[0].Data, enabledSections[0].Data) {
		t.Fatal("journal configuration did not pin the flag")
	}
	restoredDisabled, _, err := RestoreCapacityCheckpoint(disabledPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	restoredEnabled, _, err := RestoreCapacityCheckpoint(enabledPath, enabledOpts)
	if err != nil {
		t.Fatal(err)
	}
	if restoredDisabled.Checkpoints()[0] != restoredEnabled.Checkpoints()[0] || !reflect.DeepEqual(restoredDisabled.Journal()[0], restoredEnabled.Journal()[0]) {
		t.Fatal("h0 checkpoint or first batch not shared between flags")
	}
	// An explicit branch intervention flips the flag into a child bundle whose
	// lineage records the parent digest and head; policy bytes stay untouched.
	branchPath := filepath.Join(t.TempDir(), "intervention-enabled.bundle")
	branchDigest, err := BranchCapacityCheckpoint(disabledPath, opts, true, branchPath)
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
	m, err := decodeCapacityManifest(branchSections[2].Data)
	if err != nil || !m.enabled || m.depth != 1 || m.parent != disabledDigest || m.parentHead != m.head {
		t.Fatalf("branch lineage not recorded: %+v err=%v", m, err)
	}
	branchRunner, _, err := RestoreCapacityCheckpoint(branchPath, enabledOpts)
	if err != nil {
		t.Fatal(err)
	}
	if !branchRunner.enabled {
		t.Fatal("restored run does not match the intervention flag")
	}
	if exported, err := branchRunner.ExportJournal(); err != nil || !bytes.Equal(exported, branchSections[0].Data) {
		t.Fatal("restored journal config does not match the recorded intervention")
	}
	reversePath := filepath.Join(t.TempDir(), "intervention-disabled.bundle")
	if _, err := BranchCapacityCheckpoint(enabledPath, enabledOpts, false, reversePath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreCapacityCheckpoint(reversePath, opts); err != nil {
		t.Fatalf("reverse intervention rejected: %v", err)
	}
	// A no-op intervention is recorded as such and still restores.
	samePath := filepath.Join(t.TempDir(), "intervention-same.bundle")
	if _, err := BranchCapacityCheckpoint(enabledPath, enabledOpts, true, samePath); err != nil {
		t.Fatal(err)
	}
	sameSections, _, err := checkpoint.ReadFile(samePath)
	if err != nil {
		t.Fatal(err)
	}
	if m, err := decodeCapacityManifest(sameSections[2].Data); err != nil || !m.enabled || m.depth != 1 || m.parent != enabledDigest {
		t.Fatalf("same-flag intervention not recorded: %+v err=%v", m, err)
	}
}

func TestCapacityCheckpointBranchForkMatchesBaselines(t *testing.T) {
	for _, tc := range []struct {
		name     string
		q        int64
		seed     uint64
		founders []CapacityFounder
	}{
		{"q3", 3, 0, nil},
		{"q8-founderB", 8, 0, []CapacityFounder{{Actor: 1, Granary: 4}}},
	} {
		for _, workers := range []int{1, 4} {
			t.Run(fmt.Sprintf("%s-w%d", tc.name, workers), func(t *testing.T) {
				opts := CapacityOptions{Yield: tc.q, Seed: tc.seed, Workers: workers, Founders: tc.founders}
				enabledOpts := opts
				enabledOpts.Enabled = true
				const hour = 12
				if len(tc.founders) > 0 {
					// Founder endowments exist only on the enabled branch
					// (NewCapacity rejects founders×disabled), so the founder
					// lineage is captured enabled and must refuse a disabled
					// intervention outright.
					rootOpts := opts
					rootOpts.Enabled = true
					baselineEnabled := capacityRunTo(t, rootOpts, hour)
					_, rootPath, rootDigest := capacityCaptureNeutral(t, rootOpts)
					restoredRoot, gotRoot, err := RestoreCapacityCheckpoint(rootPath, rootOpts)
					if err != nil || gotRoot != rootDigest {
						t.Fatalf("root restore: %x %v", gotRoot, err)
					}
					capacityStepToHour(t, restoredRoot, hour)
					if !reflect.DeepEqual(mustCapacityHandoff(t, restoredRoot), mustCapacityHandoff(t, baselineEnabled)) {
						t.Fatal("restored founder root diverged from uninterrupted run")
					}
					branchPath := filepath.Join(t.TempDir(), "fork-disabled.bundle")
					if _, err := BranchCapacityCheckpoint(rootPath, rootOpts, false, branchPath); err == nil {
						t.Fatal("branch to disabled published over a founder lineage")
					}
					if _, err := os.Stat(branchPath); !os.IsNotExist(err) {
						t.Fatal("refused founder branch published a file")
					}
					return
				}
				opts.Enabled = false
				baselineEnabled := capacityRunTo(t, enabledOpts, hour)
				baselineDisabled := capacityRunTo(t, opts, hour)
				_, rootPath, rootDigest := capacityCaptureNeutral(t, opts)
				branchPath := filepath.Join(t.TempDir(), "fork-enabled.bundle")
				if _, err := BranchCapacityCheckpoint(rootPath, opts, true, branchPath); err != nil {
					t.Fatal(err)
				}
				restoredDisabled, gotRoot, err := RestoreCapacityCheckpoint(rootPath, opts)
				if err != nil || gotRoot != rootDigest {
					t.Fatalf("root restore: %x %v", gotRoot, err)
				}
				restoredEnabled, _, err := RestoreCapacityCheckpoint(branchPath, enabledOpts)
				if err != nil {
					t.Fatalf("branch restore: %v", err)
				}
				capacityStepToHour(t, restoredDisabled, hour)
				capacityStepToHour(t, restoredEnabled, hour)
				if !reflect.DeepEqual(mustCapacityHandoff(t, restoredDisabled), mustCapacityHandoff(t, baselineDisabled)) {
					t.Fatalf("restored disabled run diverged from uninterrupted same-flag run")
				}
				if !reflect.DeepEqual(mustCapacityHandoff(t, restoredEnabled), mustCapacityHandoff(t, baselineEnabled)) {
					t.Fatalf("restored enabled run diverged from uninterrupted same-flag run")
				}
				if !reflect.DeepEqual(capacityEventBytes(t, restoredEnabled), capacityEventBytes(t, baselineEnabled)) ||
					!reflect.DeepEqual(capacityEventBytes(t, restoredDisabled), capacityEventBytes(t, baselineDisabled)) {
					t.Fatal("accepted event bytes diverged from same-flag baselines")
				}
				enabledJournal, disabledJournal := baselineEnabled.Journal(), baselineDisabled.Journal()
				capitalEvidence := func(batches []CapacityBatch) int {
					count := 0
					for _, b := range batches {
						for _, a := range b.Attempts {
							switch a.Kind {
							case "build", "paired-meal", "fallback-eatstored", "eat-stored":
								count++
							}
						}
					}
					return count
				}
				if capitalEvidence(disabledJournal) != 0 {
					t.Fatal("disabled branch produced capital evidence")
				}
				if capitalEvidence(enabledJournal) == 0 {
					t.Fatal("enabled branch produced no capital evidence")
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
}

func mustCapacityHandoff(t *testing.T, f *Capacity) CapacityHandoff {
	t.Helper()
	h, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestCapacityCheckpointBranchDepthBound(t *testing.T) {
	opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	_, root, rootDigest := capacityCaptureNeutral(t, opts)
	// One depth-1 child for lineage forgery tests, then chain to the frozen
	// bound: depth 63 is restorable, depth 64 is refused before publication
	// because it could never be restored.
	child := filepath.Join(t.TempDir(), "depth-1.bundle")
	childDigest, err := BranchCapacityCheckpoint(root, opts, false, child)
	if err != nil {
		t.Fatal(err)
	}
	current := child
	for i := 2; i <= capacityMaxBranchDepth; i++ {
		next := filepath.Join(t.TempDir(), fmt.Sprintf("depth-%d.bundle", i))
		if _, err := BranchCapacityCheckpoint(current, opts, false, next); err != nil {
			t.Fatalf("branch to depth %d: %v", i, err)
		}
		current = next
	}
	if _, _, err := RestoreCapacityCheckpoint(current, opts); err != nil {
		t.Fatalf("depth-63 bundle refused: %v", err)
	}
	deepDir := t.TempDir()
	tooDeep := filepath.Join(deepDir, "too-deep.bundle")
	if _, err := BranchCapacityCheckpoint(current, opts, false, tooDeep); !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("published an unrestorable depth: %v", err)
	}
	if entries, err := os.ReadDir(deepDir); err != nil || len(entries) != 0 {
		t.Fatalf("over-deep branch left entries behind: %v %v", entries, err)
	}
	// A forged manifest claiming over-deep ancestry is rejected at decode.
	forged := capacityRewriteBundle(t, current, func(s []checkpoint.Section) {
		capacityEditManifest(t, s, func(m *capacityManifest) { m.depth = capacityMaxBranchDepth + 1 })
	})
	if f, _, err := RestoreCapacityCheckpoint(forged, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("accepted over-deep lineage: %v", err)
	}
	// Forged lineage links fail closed: a root capture must record no parent,
	// and a branch child must continue its parent at the same verified head.
	badRoot := capacityRewriteBundle(t, root, func(s []checkpoint.Section) {
		capacityEditManifest(t, s, func(m *capacityManifest) { m.parent = childDigest })
	})
	if f, _, err := RestoreCapacityCheckpoint(badRoot, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("root bundle accepted a forged parent: %v", err)
	}
	if rootDigest == childDigest {
		t.Fatal("root and child digests must differ")
	}
	for name, edit := range map[string]func(*capacityManifest){
		"parent-head": func(m *capacityManifest) { m.parentHead.TipID++ },
		"depth-zero":  func(m *capacityManifest) { m.depth = 0 },
	} {
		bad := capacityRewriteBundle(t, child, func(s []checkpoint.Section) {
			capacityEditManifest(t, s, edit)
		})
		if f, _, err := RestoreCapacityCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("forged %s accepted: %v", name, err)
		}
	}
}

func TestCapacityCheckpointRejectsDamageTruncationSwapAndCrossVersion(t *testing.T) {
	opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	_, path, _ := capacityCaptureNeutral(t, opts)
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
		corrupt := filepath.Join(t.TempDir(), "corrupt-capacity.bundle")
		if _, err := checkpoint.WriteFile(corrupt, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreCapacityCheckpoint(corrupt, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("accepted rehashed section %d: %v", i, err)
		}
	}
	// Swapped section payloads are caught by the manifest section hashes.
	for _, swap := range [][2]int{{0, 1}, {3, 4}} {
		bad := append([]checkpoint.Section(nil), sections...)
		bad[swap[0]].Data, bad[swap[1]].Data = bytes.Clone(bad[swap[1]].Data), bytes.Clone(bad[swap[0]].Data)
		swapped := filepath.Join(t.TempDir(), "swapped-capacity.bundle")
		if _, err := checkpoint.WriteFile(swapped, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreCapacityCheckpoint(swapped, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("accepted swapped sections %v: %v", swap, err)
		}
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{wire[:len(wire)-1], wire[:len(wire)/2], append(bytes.Clone(wire), 0)} {
		bad := filepath.Join(t.TempDir(), "truncated-capacity.bundle")
		if err := os.WriteFile(bad, malformed, 0600); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreCapacityCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("accepted malformed bundle: %v", err)
		}
	}
	// v1/S6/v2 bundles never decode as v3 and vice versa: no implicit migration.
	_, foodflowPath, _ := foodFlowSaveAt(t, 3, 2)
	_, socialPath, _ := socialFoodCaptureNeutral(t, SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4})
	s6, err := NewSurvival(SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	s6Path := filepath.Join(t.TempDir(), "s6-capacity-reject.bundle")
	if _, err := s6.SaveCheckpoint(s6Path); err != nil {
		t.Fatal(err)
	}
	for name, foreign := range map[string]string{"food-flow": foodflowPath, "social-food": socialPath, "s6": s6Path} {
		if f, _, err := RestoreCapacityCheckpoint(foreign, opts); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("accepted foreign %s bundle: %v", name, err)
		}
	}
	v1opts := FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}
	if f, _, err := RestoreFoodFlowCheckpoint(path, v1opts); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("food-flow accepted v3 bundle: %v", err)
	}
	v2opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	if f, _, err := RestoreSocialFoodCheckpoint(path, v2opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("social-food accepted v3 bundle: %v", err)
	}
	s6opts := SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1}
	if f, _, err := RestoreSurvivalCheckpoint(path, s6opts); f != nil || !errors.Is(err, ErrSurvivalCheckpoint) {
		t.Fatalf("S6 accepted v3 bundle: %v", err)
	}
}

func TestCapacityCheckpointRejectsForgedScenarioFlagAndPolicy(t *testing.T) {
	opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Enabled = true
	founderOpts := opts
	founderOpts.Founders = []CapacityFounder{{Actor: 1, Granary: 4}}
	founderOpts.Enabled = true
	_, rootPath, rootDigest := capacityCaptureNeutral(t, opts)
	_, founderPath, _ := capacityCaptureNeutral(t, founderOpts)
	branchPath := filepath.Join(t.TempDir(), "reject-branch.bundle")
	if _, err := BranchCapacityCheckpoint(rootPath, opts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	expectRejection := func(name string, bundle string, expected CapacityOptions) {
		t.Helper()
		if f, _, err := RestoreCapacityCheckpoint(bundle, expected); f != nil || !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("%s: restored=%v err=%v", name, f, err)
		}
	}
	// Wrong declared scenario or worker count.
	for _, wrong := range []CapacityOptions{
		{Yield: 8, Seed: 0, Workers: 1},
		{Yield: 3, Seed: 7, Workers: 1},
		{Yield: 3, Seed: 0, Workers: 0},
		{Yield: 3, Seed: 0, Workers: 1, Founders: founderOpts.Founders},
	} {
		expectRejection(fmt.Sprintf("wrong-options-%+v", wrong), rootPath, wrong)
	}
	expectRejection("founder-bundle-as-neutral", founderPath, opts)
	// The founder bundle does restore under its declared endowment.
	if f, _, err := RestoreCapacityCheckpoint(founderPath, founderOpts); err != nil || f == nil {
		t.Fatalf("founder bundle refused under its declared endowment: %v", err)
	}
	// Flag after the fact: an enabled continuation requires a recorded branch.
	expectRejection("flag-after-fact", rootPath, enabledOpts)
	// Rewriting the manifest flag alone cannot forge an intervention: the
	// journal configuration no longer matches.
	forgedFlag := capacityRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		capacityEditManifest(t, s, func(m *capacityManifest) { m.enabled = true })
	})
	expectRejection("forged-manifest-flag", forgedFlag, enabledOpts)
	// A branch bundle into which the parent's flag-false journal is spliced
	// back (with a matching manifest hash) still fails the config match.
	spliced := capacityRewriteBundle(t, branchPath, func(s []checkpoint.Section) {
		parent, _, err := checkpoint.ReadFile(rootPath)
		if err != nil {
			t.Fatal(err)
		}
		s[0].Data = bytes.Clone(parent[0].Data)
		capacityEditManifest(t, s, func(m *capacityManifest) { m.sections[0] = sha256.Sum256(s[0].Data) })
	})
	expectRejection("spliced-parent-journal", spliced, enabledOpts)
	// Manifest field forgeries with a valid manifest digest.
	for name, edit := range map[string]func(*capacityManifest){
		"yield":       func(m *capacityManifest) { m.yield = 8 },
		"seed":        func(m *capacityManifest) { m.seed = 7 },
		"steps":       func(m *capacityManifest) { m.steps = 2 },
		"time":        func(m *capacityManifest) { m.time = sim.SimTime(CapacityHour) },
		"parent":      func(m *capacityManifest) { m.parent = rootDigest },
		"depth":       func(m *capacityManifest) { m.depth = 1 },
		"depth-huge":  func(m *capacityManifest) { m.depth = capacityMaxBranchDepth + 1 },
		"fingerprint": func(m *capacityManifest) { m.policyFingerprint[0] ^= 1 },
		"founder":     func(m *capacityManifest) { m.founders = []CapacityFounder{{Actor: 1, Granary: 4}} },
		"founder-bad": func(m *capacityManifest) { m.founders = []CapacityFounder{{Actor: 1, Granary: CapacityGranaryMax + 1}} },
	} {
		forged := capacityRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
			capacityEditManifest(t, s, edit)
		})
		expectRejection("manifest-"+name, forged, opts)
	}
	// A raw non-binary flag byte fails manifest decoding.
	rawFlag := capacityRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		raw := s[2].Data
		if len(raw) <= capacityManifestFlagOffset {
			t.Fatalf("manifest layout changed: len=%d", len(raw))
		}
		raw[capacityManifestFlagOffset] = 2
		sum := sha256.Sum256(raw[:len(raw)-sha256.Size])
		copy(raw[len(raw)-sha256.Size:], sum[:])
	})
	expectRejection("raw-flag-byte", rawFlag, opts)
	// Tampered policy content with a rehashed manifest is not a fork: the
	// pinned frozen policy must verify against the independent constructor.
	// The budget byte decodes as a canonical same-ref policy with budget
	// 15/16 — semantically drifted, structurally valid — and fails closed.
	drifted := capacityRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		policy, _, err := decodeCapacityStrategySection(s[4].Data)
		if err != nil {
			t.Fatal(err)
		}
		policyOffset := 4 + 4 + 2
		if !bytes.Equal(s[4].Data[policyOffset:policyOffset+len(policy)], policy) {
			t.Fatal("strategy layout changed")
		}
		decoded, err := strategy.DecodeCapacityPolicy(policy)
		if err != nil {
			t.Fatal(err)
		}
		decoded.Budget.Evaluations--
		changed, err := strategy.EncodeCapacityPolicy(decoded)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt := append(bytes.Clone(s[4].Data[:policyOffset]), changed...)
		rebuilt = append(rebuilt, s[4].Data[policyOffset+len(policy):]...)
		s[4].Data = rebuilt
		if _, _, err := decodeCapacityStrategySection(s[4].Data); err != nil {
			t.Fatalf("forged strategy section is not structurally valid: %v", err)
		}
		capacityEditManifest(t, s, func(m *capacityManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("tampered-policy", drifted, opts)
	// The untouched bundles still restore after every rejection.
	if f, got, err := RestoreCapacityCheckpoint(rootPath, opts); err != nil || f == nil || got == [32]byte{} {
		t.Fatalf("valid bundle no longer restores: %v", err)
	}
	if _, _, err := RestoreCapacityCheckpoint(founderPath, founderOpts); err != nil {
		t.Fatalf("founder bundle no longer restores: %v", err)
	}
}

func TestCapacityCheckpointRejectsWakeTopology(t *testing.T) {
	opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	f, path, _ := capacityCaptureNeutral(t, opts)
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
				if s.Wakes[i].Actor == capacityClockActor {
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
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 3, At: 2, Cause: scheduler.WakeNeedThreshold})
		}},
		{"all claim wakes late", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Cause == scheduler.WakeNeedThreshold {
					s.Wakes[i].At = 2
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, wire := capacityAlterPortable(t, f, tc.change)
			if err := verifyCapacityNeutralScheduler(f.k, f.Journal(), changed.Snapshot()); !errors.Is(err, ErrCapacityCheckpoint) {
				t.Fatalf("neutral topology validator did not reject test state: %v", err)
			}
			forged := capacityRewriteBundle(t, path, func(s []checkpoint.Section) {
				s[3].Data = wire
				capacityEditManifest(t, s, func(m *capacityManifest) { m.sections[2] = sha256.Sum256(wire) })
			})
			// The journal-aware decoder alone accepts a valid generic portable
			// queue; only the exact neutral protocol check rejects it.
			if r, _, err := RestoreCapacityCheckpoint(forged, opts); r != nil || !errors.Is(err, ErrCapacityCheckpoint) {
				t.Fatalf("accepted invalid topology: runner=%v err=%v", r, err)
			}
		})
	}
	// Publication refuses a runner whose live scheduler no longer matches the
	// neutral protocol, and publishes nothing.
	changed, _ := capacityAlterPortable(t, f, func(s *scheduler.Snapshot) {
		s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 2, At: 2, Cause: scheduler.WakeAudit})
	})
	f.sched = changed
	absentDir := t.TempDir()
	if digest, err := f.SaveCheckpoint(filepath.Join(absentDir, "unpublished.bundle")); digest != [32]byte{} || !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("published invalid topology: digest=%x err=%v", digest, err)
	}
	entries, err := os.ReadDir(absentDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial publication: entries=%v err=%v", entries, err)
	}
}

func TestCapacityCheckpointRefusesDriftAndNeverClobbers(t *testing.T) {
	opts := CapacityOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Enabled = true
	f, path, digest := capacityCaptureNeutral(t, opts)
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
	for _, e := range []string{"bad-dir/capacity.bundle", filepath.Join(t.TempDir(), ".checkpoint-forbidden")} {
		if _, err := f.SaveCheckpoint(e); err == nil {
			t.Fatalf("published unsafe path %s", e)
		}
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later.
	original := f.bound
	registry := strategy.NewCapacityRegistry()
	changed := f.bound[0].Policy()
	changed.Budget.Candidates--
	if err := registry.Register(changed); err != nil {
		t.Fatal(err)
	}
	for i := range f.bound {
		bound, err := registry.Bind(strategy.CapacityBinding{Actor: sim.EntityID(i + 1), Ref: changed.Ref})
		if err != nil {
			t.Fatal(err)
		}
		f.bound[i] = bound
	}
	absent := filepath.Join(t.TempDir(), "altered-capacity-policy.bundle")
	if saved, err := f.SaveCheckpoint(absent); saved != [32]byte{} || !errors.Is(err, ErrCapacityCheckpoint) {
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
	if _, err := BranchCapacityCheckpoint(path, wrongScenario, true, dest); !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatalf("branched from a misdeclared parent: %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial branch publication: %v", err)
	}
	if _, err := BranchCapacityCheckpoint(path, opts, true, path); !errors.Is(err, os.ErrExist) {
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
	if restored, got, err := RestoreCapacityCheckpoint(path, opts); err != nil || got != digest || restored == nil {
		t.Fatalf("valid bundle restore: %x %v", got, err)
	}
	if _, _, err := RestoreCapacityCheckpoint(path, enabledOpts); !errors.Is(err, ErrCapacityCheckpoint) {
		t.Fatal("enabled restore accepted a disabled root bundle")
	}
}
