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

func socialFoodCaptureNeutral(t *testing.T, options SocialFoodOptions) (*SocialFood, string, [32]byte) {
	t.Helper()
	f, err := NewSocialFood(options)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := f.Step(context.Background())
	if err != nil || !ok {
		t.Fatalf("neutral h0 pulse: processed=%t err=%v", ok, err)
	}
	if f.steps != 1 || f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 || len(f.Journal()[0].Attempts) != 0 {
		t.Fatalf("not a neutral h0 pulse boundary: steps=%d time=%d checkpoints=%d", f.steps, f.sched.Time(), len(f.checkpoints))
	}
	path := filepath.Join(t.TempDir(), "social-neutral-h0.bundle")
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	return f, path, digest
}

func socialFoodStepToHour(t *testing.T, f *SocialFood, hour int) {
	t.Helper()
	for len(f.Checkpoints()) <= hour {
		ok, err := f.Step(context.Background())
		if err != nil || !ok {
			t.Fatalf("hour %d: processed=%t err=%v", len(f.Checkpoints()), ok, err)
		}
	}
}

func socialFoodRewriteBundle(t *testing.T, path string, edit func([]checkpoint.Section)) string {
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
	dest := filepath.Join(t.TempDir(), "forged-social.bundle")
	if _, err := checkpoint.WriteFile(dest, sections); err != nil {
		t.Fatal(err)
	}
	return dest
}

func socialFoodEditManifest(t *testing.T, sections []checkpoint.Section, edit func(*socialFoodManifest)) {
	t.Helper()
	m, err := decodeSocialFoodManifest(sections[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	edit(&m)
	sections[2].Data = encodeSocialFoodManifest(m)
}

func socialFoodAlterPortable(t *testing.T, f *SocialFood, change func(*scheduler.Snapshot)) (*scheduler.Scheduler, []byte) {
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

func socialFoodSocialAttempts(journal []SocialFoodBatch) map[string]int {
	counts := map[string]int{}
	for _, b := range journal {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "request", "gift", "refuse", "expire":
				counts[a.Kind]++
			}
		}
	}
	return counts
}

func TestSocialFoodCheckpointNeutralCaptureRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts SocialFoodOptions
	}{{"disabled", SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4}}, {"enabled", SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}}} {
		t.Run(tc.name, func(t *testing.T) {
			f, path, digest := socialFoodCaptureNeutral(t, tc.opts)
			before, err := f.Handoff()
			if err != nil {
				t.Fatal(err)
			}
			restored, got, err := RestoreSocialFoodCheckpoint(path, tc.opts)
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
			sections, _, err := checkpoint.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m, err := decodeSocialFoodManifest(sections[2].Data)
			if err != nil {
				t.Fatal(err)
			}
			if m.depth != 0 || m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) || !m.enabled != !tc.opts.Enabled || m.yield != tc.opts.Yield || m.seed != tc.opts.Seed || m.steps != 1 || m.time != 0 {
				t.Fatalf("root manifest lineage/scenario wrong: %+v", m)
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
			socialFoodStepToHour(t, f, 1)
			if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "late.bundle")); !errors.Is(err, ErrSocialFoodCheckpoint) {
				t.Fatalf("published non-neutral boundary: %v", err)
			}
		})
	}
}

func TestSocialFoodCheckpointCaptureIsFlagNeutralAndBranchRecordsIntervention(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 4}
	enabledOpts := opts
	enabledOpts.Enabled = true
	_, disabledPath, disabledDigest := socialFoodCaptureNeutral(t, opts)
	_, enabledPath, enabledDigest := socialFoodCaptureNeutral(t, enabledOpts)
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
	restoredDisabled, _, err := RestoreSocialFoodCheckpoint(disabledPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	restoredEnabled, _, err := RestoreSocialFoodCheckpoint(enabledPath, enabledOpts)
	if err != nil {
		t.Fatal(err)
	}
	if restoredDisabled.Checkpoints()[0] != restoredEnabled.Checkpoints()[0] || !reflect.DeepEqual(restoredDisabled.Journal()[0], restoredEnabled.Journal()[0]) {
		t.Fatal("h0 checkpoint or first batch not shared between flags")
	}
	// An explicit branch intervention flips the flag into a child bundle whose
	// lineage records the parent digest and head; policy bytes stay untouched.
	branchPath := filepath.Join(t.TempDir(), "intervention-enabled.bundle")
	if _, err := BranchSocialFoodCheckpoint(disabledPath, opts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	branchSections, _, err := checkpoint.ReadFile(branchPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(branchSections[4].Data, disabledSections[4].Data) || !bytes.Equal(branchSections[1].Data, disabledSections[1].Data) || !bytes.Equal(branchSections[3].Data, disabledSections[3].Data) {
		t.Fatal("branch edited serialized policy, history or scheduler bytes")
	}
	m, err := decodeSocialFoodManifest(branchSections[2].Data)
	if err != nil || !m.enabled || m.depth != 1 || m.parent != disabledDigest || m.parentHead != m.head {
		t.Fatalf("branch lineage not recorded: %+v err=%v", m, err)
	}
	branchRunner, _, err := RestoreSocialFoodCheckpoint(branchPath, enabledOpts)
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
	if _, err := BranchSocialFoodCheckpoint(enabledPath, enabledOpts, false, reversePath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreSocialFoodCheckpoint(reversePath, opts); err != nil {
		t.Fatalf("reverse intervention rejected: %v", err)
	}
	// A no-op intervention is recorded as such and still restores.
	samePath := filepath.Join(t.TempDir(), "intervention-same.bundle")
	if _, err := BranchSocialFoodCheckpoint(enabledPath, enabledOpts, true, samePath); err != nil {
		t.Fatal(err)
	}
	sameSections, _, err := checkpoint.ReadFile(samePath)
	if err != nil {
		t.Fatal(err)
	}
	if m, err := decodeSocialFoodManifest(sameSections[2].Data); err != nil || !m.enabled || m.depth != 1 || m.parent != enabledDigest {
		t.Fatalf("same-flag intervention not recorded: %+v err=%v", m, err)
	}
}

func TestSocialFoodCheckpointNeutralForkSharesPrefixAndDiverges(t *testing.T) {
	for _, tc := range []struct {
		q    int64
		seed uint64
		hour int
	}{{3, 0, 24}, {3, 7, 24}, {8, 0, 24}, {8, 7, 24}, {0, 0, 12}, {0, 7, 12}} {
		for _, workers := range []int{1, 4} {
			name := fmt.Sprintf("q%d-seed%d-w%d", tc.q, tc.seed, workers)
			t.Run(name, func(t *testing.T) {
				opts := SocialFoodOptions{Yield: tc.q, Seed: tc.seed, Workers: workers}
				enabledOpts := opts
				enabledOpts.Enabled = true
				baselineEnabled := socialFoodRunTo(t, enabledOpts, tc.hour)
				baselineDisabled := socialFoodRunTo(t, opts, tc.hour)
				wantAlive := map[int64]int{3: 6, 8: 16, 0: 0}[tc.q]
				if baselineEnabled.Checkpoints()[tc.hour].Alive != wantAlive || baselineDisabled.Checkpoints()[tc.hour].Alive != wantAlive {
					t.Fatalf("q%d fixture drift: alive enabled=%d disabled=%d want=%d", tc.q, baselineEnabled.Checkpoints()[tc.hour].Alive, baselineDisabled.Checkpoints()[tc.hour].Alive, wantAlive)
				}
				_, rootPath, rootDigest := socialFoodCaptureNeutral(t, opts)
				branchPath := filepath.Join(t.TempDir(), "fork-enabled.bundle")
				if _, err := BranchSocialFoodCheckpoint(rootPath, opts, true, branchPath); err != nil {
					t.Fatal(err)
				}
				restoredDisabled, gotRoot, err := RestoreSocialFoodCheckpoint(rootPath, opts)
				if err != nil || gotRoot != rootDigest {
					t.Fatalf("root restore: %x %v", gotRoot, err)
				}
				restoredEnabled, _, err := RestoreSocialFoodCheckpoint(branchPath, enabledOpts)
				if err != nil {
					t.Fatalf("branch restore: %v", err)
				}
				if mustSocialFoodHandoff(t, restoredDisabled).Enabled || !mustSocialFoodHandoff(t, restoredEnabled).Enabled {
					t.Fatal("restored runs do not match their manifest intervention flags")
				}
				socialFoodStepToHour(t, restoredDisabled, tc.hour)
				socialFoodStepToHour(t, restoredEnabled, tc.hour)
				if !reflect.DeepEqual(mustSocialFoodHandoff(t, restoredDisabled), mustSocialFoodHandoff(t, baselineDisabled)) {
					t.Fatal("restored disabled run diverged from uninterrupted same-flag run")
				}
				if !reflect.DeepEqual(mustSocialFoodHandoff(t, restoredEnabled), mustSocialFoodHandoff(t, baselineEnabled)) {
					t.Fatal("restored enabled run diverged from uninterrupted same-flag run")
				}
				if !reflect.DeepEqual(socialFoodEventBytes(t, restoredEnabled), socialFoodEventBytes(t, baselineEnabled)) || !reflect.DeepEqual(socialFoodEventBytes(t, restoredDisabled), socialFoodEventBytes(t, baselineDisabled)) {
					t.Fatal("accepted event bytes diverged from same-flag baselines")
				}
				enabledJournal, disabledJournal := baselineEnabled.Journal(), baselineDisabled.Journal()
				if len(enabledJournal) != len(disabledJournal) {
					t.Fatalf("batch counts differ: %d/%d", len(enabledJournal), len(disabledJournal))
				}
				enabledCounts, disabledCounts := socialFoodSocialAttempts(enabledJournal), socialFoodSocialAttempts(disabledJournal)
				if tc.q == 3 {
					if reflect.DeepEqual(enabledJournal, disabledJournal) {
						t.Fatal("no divergence after the enable intervention")
					}
					if enabledCounts["request"] == 0 || enabledCounts["gift"] == 0 || enabledCounts["refuse"] == 0 {
						t.Fatalf("missing enabled social evidence: %+v", enabledCounts)
					}
					if len(disabledCounts) != 0 {
						t.Fatalf("disabled branch transferred: %+v", disabledCounts)
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
					if len(socialFoodSocialAttempts([]SocialFoodBatch{enabledJournal[diff]})) == 0 {
						t.Fatalf("divergence at batch %d carries no social evidence", diff)
					}
					if reflect.DeepEqual(baselineEnabled.Checkpoints(), baselineDisabled.Checkpoints()) {
						t.Fatal("no state divergence after gifts")
					}
					t.Logf("q%d seed%d w%d fork: prefix batches %d, enabled social=%v, h%d alive=%d", tc.q, tc.seed, workers, diff, enabledCounts, tc.hour, baselineEnabled.Checkpoints()[tc.hour].Alive)
				} else {
					// q8 and q0 offer no social opportunity: both flagged
					// continuations must stay byte-identical from one neutral
					// checkpoint.
					if !reflect.DeepEqual(enabledJournal, disabledJournal) || len(enabledCounts) != 0 {
						t.Fatalf("unexpected social opportunity at q%d: %+v", tc.q, enabledCounts)
					}
				}
			})
		}
	}
}

func mustSocialFoodHandoff(t *testing.T, f *SocialFood) SocialFoodHandoff {
	t.Helper()
	h, err := f.Handoff()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSocialFoodCheckpointForkFromBranchDepthTwo(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Enabled = true
	baseline := socialFoodRunTo(t, opts, 12)
	_, rootPath, rootDigest := socialFoodCaptureNeutral(t, opts)
	firstPath := filepath.Join(t.TempDir(), "fork-1.bundle")
	firstDigest, err := BranchSocialFoodCheckpoint(rootPath, opts, true, firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(t.TempDir(), "fork-2.bundle")
	secondDigest, err := BranchSocialFoodCheckpoint(firstPath, enabledOpts, false, secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest || secondDigest == rootDigest {
		t.Fatal("lineage bundles are not distinct")
	}
	sections, _, err := checkpoint.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := decodeSocialFoodManifest(sections[2].Data)
	if err != nil || m.depth != 2 || m.parent != firstDigest || m.enabled {
		t.Fatalf("depth-2 lineage not recorded: %+v err=%v", m, err)
	}
	restored, _, err := RestoreSocialFoodCheckpoint(secondPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	socialFoodStepToHour(t, restored, 12)
	if !reflect.DeepEqual(mustSocialFoodHandoff(t, restored), mustSocialFoodHandoff(t, baseline)) {
		t.Fatal("depth-2 disable continuation diverged from the uninterrupted disabled run")
	}
	// Chain depth is bounded so forged manifests cannot claim deep ancestry.
	forged := socialFoodRewriteBundle(t, secondPath, func(s []checkpoint.Section) {
		m, err := decodeSocialFoodManifest(s[2].Data)
		if err != nil {
			t.Fatal(err)
		}
		m.depth = socialFoodMaxBranchDepth + 1
		s[2].Data = encodeSocialFoodManifest(m)
	})
	if f, _, err := RestoreSocialFoodCheckpoint(forged, opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("accepted over-deep lineage: %v", err)
	}
}

func TestSocialFoodCheckpointRejectsDamageTruncationAndCrossVersion(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	_, path, _ := socialFoodCaptureNeutral(t, opts)
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
		corrupt := filepath.Join(t.TempDir(), "corrupt-social.bundle")
		if _, err := checkpoint.WriteFile(corrupt, bad); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreSocialFoodCheckpoint(corrupt, opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
			t.Fatalf("accepted rehashed section %d: %v", i, err)
		}
	}
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{wire[:len(wire)-1], wire[:len(wire)/2], append(bytes.Clone(wire), 0)} {
		bad := filepath.Join(t.TempDir(), "truncated-social.bundle")
		if err := os.WriteFile(bad, malformed, 0600); err != nil {
			t.Fatal(err)
		}
		if f, _, err := RestoreSocialFoodCheckpoint(bad, opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
			t.Fatalf("accepted malformed bundle: %v", err)
		}
	}
	// v1/S6 bundles never decode as v2 and vice versa: no implicit migration.
	_, foodflowPath, _ := foodFlowSaveAt(t, 3, 2)
	s6, err := NewSurvival(SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1})
	if err != nil {
		t.Fatal(err)
	}
	s6Path := filepath.Join(t.TempDir(), "s6-social-reject.bundle")
	if _, err := s6.SaveCheckpoint(s6Path); err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []string{foodflowPath, s6Path} {
		if f, _, err := RestoreSocialFoodCheckpoint(foreign, opts); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
			t.Fatalf("accepted foreign bundle %s: %v", filepath.Base(foreign), err)
		}
	}
	if f, _, err := RestoreFoodFlowCheckpoint(path, FoodFlowOptions{Yield: 3, Seed: 7, Workers: 1}); f != nil || !errors.Is(err, ErrFoodFlowCheckpoint) {
		t.Fatalf("food-flow accepted v2 bundle: %v", err)
	}
	if f, _, err := RestoreSurvivalCheckpoint(path, SurvivalOptions{Actors: 2, Hours: 2, Seed: 7, Workers: 1, EatWeight: 1}); f != nil || !errors.Is(err, ErrSurvivalCheckpoint) {
		t.Fatalf("S6 accepted v2 bundle: %v", err)
	}
}

func TestSocialFoodCheckpointRejectsForgedScenarioAndFlagAfterFact(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Enabled = true
	_, rootPath, rootDigest := socialFoodCaptureNeutral(t, opts)
	branchPath := filepath.Join(t.TempDir(), "reject-branch.bundle")
	if _, err := BranchSocialFoodCheckpoint(rootPath, opts, true, branchPath); err != nil {
		t.Fatal(err)
	}
	expectRejection := func(name, bundle string, expected SocialFoodOptions) {
		t.Helper()
		if f, _, err := RestoreSocialFoodCheckpoint(bundle, expected); f != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
			t.Fatalf("%s: restored=%v err=%v", name, f, err)
		}
	}
	// Wrong declared scenario or worker count.
	for _, wrong := range []SocialFoodOptions{{Yield: 8, Seed: 0, Workers: 1}, {Yield: 3, Seed: 7, Workers: 1}, {Yield: 3, Seed: 0, Workers: 0}} {
		expectRejection(fmt.Sprintf("wrong-options-%+v", wrong), rootPath, wrong)
	}
	// Flag after the fact: an enabled continuation requires a recorded branch.
	expectRejection("flag-after-fact", rootPath, enabledOpts)
	// Rewriting the manifest flag alone cannot forge an intervention: the
	// journal configuration no longer matches.
	forgedFlag := socialFoodRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		socialFoodEditManifest(t, s, func(m *socialFoodManifest) { m.enabled = true })
	})
	expectRejection("forged-manifest-flag", forgedFlag, enabledOpts)
	// A branch bundle into which the parent's flag-false journal is spliced
	// back (with a matching manifest hash) still fails the config match.
	spliced := socialFoodRewriteBundle(t, branchPath, func(s []checkpoint.Section) {
		s[0].Data = bytes.Clone(rootJournal(t, rootPath))
		socialFoodEditManifest(t, s, func(m *socialFoodManifest) { m.sections[0] = sha256.Sum256(s[0].Data) })
	})
	expectRejection("spliced-parent-journal", spliced, enabledOpts)
	// Wrong overrides are a different scenario.
	overrides := opts
	overrides.TestClaimOverrides = map[sim.EntityID]int64{8: 0}
	expectRejection("wrong-overrides", rootPath, overrides)
	// A branch must declare the parent scenario exactly, overrides included.
	_, overridePath, _ := socialFoodCaptureNeutral(t, overrides)
	dest := filepath.Join(t.TempDir(), "override-branch.bundle")
	if _, err := BranchSocialFoodCheckpoint(overridePath, opts, true, dest); !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("branched an override scenario without declaring overrides: %v", err)
	}
	if _, err := BranchSocialFoodCheckpoint(overridePath, overrides, true, dest); err != nil {
		t.Fatalf("declared override branch refused: %v", err)
	}
	branchOverrideOpts := overrides
	branchOverrideOpts.Enabled = true
	if f, _, err := RestoreSocialFoodCheckpoint(dest, branchOverrideOpts); f == nil || err != nil {
		t.Fatalf("override branch restore: %v", err)
	}
	// Manifest field forgeries with a valid manifest digest.
	for name, edit := range map[string]func(*socialFoodManifest){
		"yield":      func(m *socialFoodManifest) { m.yield = 8 },
		"seed":       func(m *socialFoodManifest) { m.seed = 7 },
		"steps":      func(m *socialFoodManifest) { m.steps = 2 },
		"time":       func(m *socialFoodManifest) { m.time = sim.SimTime(SocialFoodHour) },
		"parent":     func(m *socialFoodManifest) { m.parent = rootDigest },
		"depth":      func(m *socialFoodManifest) { m.depth = 1 },
		"depth-huge": func(m *socialFoodManifest) { m.depth = socialFoodMaxBranchDepth + 1 },
		"flow-fp":    func(m *socialFoodManifest) { m.flowPolicy[0] ^= 1 },
		"social-fp":  func(m *socialFoodManifest) { m.socialPolicy[0] ^= 1 },
	} {
		forged := socialFoodRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
			socialFoodEditManifest(t, s, edit)
		})
		expectRejection("manifest-"+name, forged, opts)
	}
	// A raw non-binary flag byte fails manifest decoding.
	rawFlag := socialFoodRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		raw := s[2].Data
		const socialFoodManifestFlagOffset = 92 // magic + constants + yield + seed
		raw[socialFoodManifestFlagOffset] = 2
		sum := sha256.Sum256(raw[:len(raw)-sha256.Size])
		copy(raw[len(raw)-sha256.Size:], sum[:])
	})
	expectRejection("raw-flag-byte", rawFlag, opts)
	// Tampered policy content with a rehashed manifest is not a fork: the
	// pinned frozen policy must verify against the independent constructor.
	drifted := socialFoodRewriteBundle(t, rootPath, func(s []checkpoint.Section) {
		flow, social, _, err := decodeSocialFoodStrategySection(s[4].Data)
		if err != nil {
			t.Fatal(err)
		}
		flowOffset := 4 + 4 + 2
		if !bytes.Equal(s[4].Data[flowOffset:flowOffset+len(flow)], flow) {
			t.Fatal("strategy layout changed")
		}
		social[len(social)-9] ^= 1 // last tie affinity
		rebuilt := append([]byte(nil), s[4].Data[:4+4+2+len(flow)]...)
		rebuilt = binary.BigEndian.AppendUint16(rebuilt, uint16(len(social)))
		rebuilt = append(rebuilt, social...)
		rebuilt = binary.BigEndian.AppendUint32(rebuilt, SocialFoodActorCount)
		for i := 0; i < SocialFoodActorCount; i++ {
			rebuilt = binary.BigEndian.AppendUint64(rebuilt, uint64(i+1))
			rebuilt = binary.BigEndian.AppendUint32(rebuilt, strategy.FoodFlowPolicyFormatV1)
			rebuilt = binary.BigEndian.AppendUint32(rebuilt, strategy.SocialFoodPolicyFormatV2)
		}
		s[4].Data = rebuilt
		if _, _, _, err := decodeSocialFoodStrategySection(s[4].Data); err != nil {
			t.Fatalf("forged strategy section is not structurally valid: %v", err)
		}
		socialFoodEditManifest(t, s, func(m *socialFoodManifest) { m.sections[3] = sha256.Sum256(s[4].Data) })
	})
	expectRejection("tampered-policy", drifted, opts)
	// The untouched bundle still restores after every rejection.
	if f, got, err := RestoreSocialFoodCheckpoint(rootPath, opts); err != nil || f == nil || got == [32]byte{} {
		t.Fatalf("valid bundle no longer restores: %v", err)
	}
}

func rootJournal(t *testing.T, path string) []byte {
	t.Helper()
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sections[0].Data
}

func TestSocialFoodCheckpointRejectsWakeTopology(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	f, path, _ := socialFoodCaptureNeutral(t, opts)
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
		{"missing clock wake", func(s *scheduler.Snapshot) {
			for i := range s.Wakes {
				if s.Wakes[i].Actor == socialFoodClockActor {
					s.Wakes = append(s.Wakes[:i], s.Wakes[i+1:]...)
					return
				}
			}
			t.Fatal("no clock wake")
		}},
		{"live actor wakes dropped", func(s *scheduler.Snapshot) {
			kept := s.Wakes[:0]
			for _, wake := range s.Wakes {
				if wake.Actor != 16 {
					kept = append(kept, wake)
				}
			}
			s.Wakes = kept
		}},
		{"duplicate claim wake", func(s *scheduler.Snapshot) {
			s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 3, At: 3, Cause: scheduler.WakeNeedThreshold})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, wire := socialFoodAlterPortable(t, f, tc.change)
			if err := verifySocialFoodNeutralScheduler(f.k, f.Journal(), changed.Snapshot()); !errors.Is(err, ErrSocialFoodCheckpoint) {
				t.Fatalf("neutral topology validator did not reject test state: %v", err)
			}
			forged := socialFoodRewriteBundle(t, path, func(s []checkpoint.Section) {
				s[3].Data = wire
				socialFoodEditManifest(t, s, func(m *socialFoodManifest) { m.sections[2] = sha256.Sum256(wire) })
			})
			// The journal-aware decoder alone accepts a valid generic portable
			// queue; only the exact neutral protocol check rejects it.
			if r, _, err := RestoreSocialFoodCheckpoint(forged, opts); r != nil || !errors.Is(err, ErrSocialFoodCheckpoint) {
				t.Fatalf("accepted invalid topology: runner=%v err=%v", r, err)
			}
		})
	}
	// Publication refuses a runner whose live scheduler no longer matches the
	// neutral protocol, and publishes nothing.
	changed, _ := socialFoodAlterPortable(t, f, func(s *scheduler.Snapshot) {
		s.Wakes = append(s.Wakes, scheduler.Wake{Actor: 2, At: 2, Cause: scheduler.WakeAudit})
	})
	f.sched = changed
	absentDir := t.TempDir()
	if digest, err := f.SaveCheckpoint(filepath.Join(absentDir, "unpublished.bundle")); digest != [32]byte{} || !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("published invalid topology: digest=%x err=%v", digest, err)
	}
	entries, err := os.ReadDir(absentDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial publication: entries=%v err=%v", entries, err)
	}
}

func TestSocialFoodCheckpointRefusesPolicyDriftAndNeverClobbers(t *testing.T) {
	opts := SocialFoodOptions{Yield: 3, Seed: 0, Workers: 1}
	enabledOpts := opts
	enabledOpts.Enabled = true
	f, path, digest := socialFoodCaptureNeutral(t, opts)
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A failed no-clobber publication never replaces an existing complete file.
	if _, err := f.SaveCheckpoint(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("clobbered existing destination: %v", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(wire, unchanged) {
		t.Fatal("existing bundle modified")
	}
	for _, e := range []string{"bad-dir/social.bundle", filepath.Join(t.TempDir(), ".checkpoint-forbidden")} {
		if _, err := f.SaveCheckpoint(e); err == nil {
			t.Fatalf("published unsafe path %s", e)
		}
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later.
	original := f.social[0]
	registry := strategy.NewSocialFoodRegistry()
	changed := original.Policy()
	changed.Budget.Candidates--
	if err := registry.Register(changed); err != nil {
		t.Fatal(err)
	}
	bound, err := registry.Bind(original.Binding())
	if err != nil {
		t.Fatal(err)
	}
	f.social[0] = bound
	absent := filepath.Join(t.TempDir(), "altered-social-policy.bundle")
	if saved, err := f.SaveCheckpoint(absent); saved != [32]byte{} || !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("published altered policy: digest=%x err=%v", saved, err)
	}
	if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial publication: %v", err)
	}
	// Branching demands the parent scenario and never clobbers.
	branchDir := t.TempDir()
	wrongScenario := opts
	wrongScenario.Yield = 8
	dest := filepath.Join(branchDir, "rejected-branch.bundle")
	if _, err := BranchSocialFoodCheckpoint(path, wrongScenario, true, dest); !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatalf("branched from a misdeclared parent: %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial branch publication: %v", err)
	}
	if _, err := BranchSocialFoodCheckpoint(path, opts, true, path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("branch clobbered the parent bundle: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(wire, after) {
		t.Fatal("parent bundle modified by refused branch")
	}
	entries, err := os.ReadDir(branchDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("branch left entries behind: %v", entries)
	}
	// The untouched runner still saves and restores with a stable digest.
	if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "again.bundle")); !errors.Is(err, ErrSocialFoodCheckpoint) {
		t.Fatal("drifted policy did not fail save")
	}
	f.social[0] = original
	if _, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "again.bundle")); err != nil {
		t.Fatal(err)
	}
	if restored, got, err := RestoreSocialFoodCheckpoint(path, opts); err != nil || got != digest || restored == nil {
		t.Fatalf("valid bundle restore: %x %v", got, err)
	}
}
