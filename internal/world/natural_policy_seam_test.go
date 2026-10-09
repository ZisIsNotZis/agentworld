package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// The policy-selection seam tests (ticket 16, D1 surface). The declared
// option is NaturalOptions.PolicyRef with the zero value exactly natural/5;
// the ref is run identity end to end (journal config, checkpoint manifest,
// bidirectional mismatch rejection, ref-vs-fingerprint fail-closed), and the
// runner dispatches the frozen cascade matching its bound ref. All existing
// v5 suites stay untouched and green: the zero-value pins below prove the
// default publishes byte-identical v5 bundles and journals.

var (
	naturalSeamRefV5 = strategy.FrozenNaturalPolicy().Ref
	naturalSeamRefV6 = strategy.FrozenNaturalPolicyV6().Ref
)

// naturalSeamFingerprint fingerprints one frozen policy of the ref set.
func naturalSeamFingerprint(t *testing.T, ref strategy.NaturalRef) [32]byte {
	t.Helper()
	policy, err := strategy.FrozenNaturalPolicyForRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := strategy.FrozenNaturalPolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

// naturalSeamRun builds an organic runner over the shared fixture shape
// (P=2, R=8, seed 7, one worker) under the declared ref (the zero value
// resolves to natural/5).
func naturalSeamRun(t *testing.T, ref strategy.NaturalRef, births bool) *Natural {
	t.Helper()
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 7, Workers: 1, Births: births, PolicyRef: ref})
	if err != nil {
		t.Fatal(err)
	}
	// Commit the neutral h0 pulse: the root capture boundary.
	naturalStepToBoundary(t, f, 0)
	if ref != (strategy.NaturalRef{}) && f.ref != ref {
		t.Fatalf("runner bound %+v, want %+v", f.ref, ref)
	}
	if _, err := strategy.FrozenNaturalPolicyForRef(f.ref); err != nil {
		t.Fatalf("runner bound a ref outside the frozen set: %v", err)
	}
	return f
}

// naturalSeamExpectation is the declared restore scenario for the shared
// fixture shape; a zero PolicyRef keeps its natural/5 meaning.
func naturalSeamExpectation(ref strategy.NaturalRef, workers int, births bool) NaturalRestoreOptions {
	return NaturalRestoreOptions{Patches: 2, R: 8, Seed: 7, Workers: workers, Births: births, PolicyRef: ref}
}

// naturalSeamSave publishes a bundle at the runner's current boundary.
func naturalSeamSave(t *testing.T, f *Natural, name string, resume bool) (string, [32]byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	var digest [32]byte
	var err error
	if resume {
		digest, err = f.SaveResumeCheckpoint(path)
	} else {
		digest, err = f.SaveCheckpoint(path)
	}
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return path, digest
}

// naturalSeamManifest decodes a bundle's manifest.
func naturalSeamManifest(t *testing.T, path string) naturalManifest {
	t.Helper()
	sections, _, err := checkpoint.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := decodeNaturalManifest(sections[2].Data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestNaturalPolicySeamZeroValueDigestIdentity proves the D1
// default-preservation: a zero-value runner and an explicit natural/5
// runner publish byte-identical bundles and journals, and each other's
// bundles restore under each other's expectations.
func TestNaturalPolicySeamZeroValueDigestIdentity(t *testing.T) {
	zero := naturalSeamRun(t, strategy.NaturalRef{}, false)
	explicit := naturalSeamRun(t, naturalSeamRefV5, false)
	if zero.ref != explicit.ref || zero.ref != naturalSeamRefV5 {
		t.Fatalf("zero value must be exactly natural/5: %+v vs %+v", zero.ref, explicit.ref)
	}
	rootZero, digestZero := naturalSeamSave(t, zero, "zero-root.bundle", false)
	_, digestExplicit := naturalSeamSave(t, explicit, "explicit-root.bundle", false)
	if digestZero != digestExplicit {
		t.Fatalf("h0 bundle digests diverge: %x vs %x", digestZero, digestExplicit)
	}
	naturalRunTo(t, zero, 5)
	naturalRunTo(t, explicit, 5)
	if !reflect.DeepEqual(zero.Journal(), explicit.Journal()) {
		t.Fatal("zero-value and explicit natural/5 journals diverge")
	}
	if !reflect.DeepEqual(zero.Checkpoints(), explicit.Checkpoints()) {
		t.Fatal("zero-value and explicit natural/5 checkpoints diverge")
	}
	journalZero, err := zero.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	journalExplicit, err := explicit.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(journalZero, journalExplicit) {
		t.Fatal("zero-value and explicit natural/5 exported journals are not byte-identical")
	}
	resumeZero, digestZeroResume := naturalSeamSave(t, zero, "zero-resume.bundle", true)
	resumeExplicit, digestExplicitResume := naturalSeamSave(t, explicit, "explicit-resume.bundle", true)
	if digestZeroResume != digestExplicitResume {
		t.Fatalf("resume bundle digests diverge: %x vs %x", digestZeroResume, digestExplicitResume)
	}
	// The zero-value bundle is a natural/5 bundle in every pin: manifest
	// slot, content fingerprint, and the journal wire configuration.
	m := naturalSeamManifest(t, rootZero)
	if m.policyFormat != strategy.NaturalPolicyFormatV5 || m.policyFingerprint != naturalSeamFingerprint(t, naturalSeamRefV5) {
		t.Fatalf("zero-value manifest pins %+v / %x, want natural/5", m.policyFormat, m.policyFingerprint)
	}
	var wire naturalJournalWire
	if err := json.Unmarshal(journalZero[12:len(journalZero)-32], &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Config.Policy != naturalSeamRefV5 || wire.Config.PolicyFingerprint != naturalSeamFingerprint(t, naturalSeamRefV5) {
		t.Fatalf("zero-value journal config carries %+v, want natural/5", wire.Config.Policy)
	}
	// Cross-restore: each runner's resume bundle restores under the other's
	// declared expectation (zero value and explicit ref are the same
	// scenario), with a different worker count.
	if _, _, err := RestoreNaturalCheckpoint(resumeExplicit, naturalSeamExpectation(strategy.NaturalRef{}, 4, false)); err != nil {
		t.Fatalf("explicit bundle under the zero-value expectation: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(resumeZero, naturalSeamExpectation(naturalSeamRefV5, 2, false)); err != nil {
		t.Fatalf("zero-value bundle under the explicit expectation: %v", err)
	}
}

// TestNaturalPolicySeamV6RoundTripCarriesRef proves the v6 ref is run
// identity end to end: the manifest pins format 6 and the v6 fingerprint,
// save/restore/branch carry and re-verify the ref, and the restored runner
// continues deterministically.
func TestNaturalPolicySeamV6RoundTripCarriesRef(t *testing.T) {
	f6 := naturalSeamRun(t, naturalSeamRefV6, false)
	root, digestRoot := naturalSeamSave(t, f6, "v6-root.bundle", false)
	m := naturalSeamManifest(t, root)
	if m.policyFormat != strategy.NaturalPolicyFormatV6 || m.policyFingerprint != naturalSeamFingerprint(t, naturalSeamRefV6) {
		t.Fatalf("v6 manifest pins format %d fingerprint %x", m.policyFormat, m.policyFingerprint)
	}
	if exported, err := f6.ExportJournal(); err != nil {
		t.Fatal(err)
	} else {
		var wire naturalJournalWire
		if err := json.Unmarshal(exported[12:len(exported)-32], &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Config.Policy != naturalSeamRefV6 || wire.Config.PolicyFingerprint != naturalSeamFingerprint(t, naturalSeamRefV6) {
			t.Fatalf("v6 journal config carries %+v", wire.Config.Policy)
		}
	}
	naturalRunTo(t, f6, 4)
	resume, _ := naturalSeamSave(t, f6, "v6-resume.bundle", true)
	restored, digestRestored, err := RestoreNaturalCheckpoint(resume, naturalSeamExpectation(naturalSeamRefV6, 2, false))
	if err != nil {
		t.Fatalf("v6 resume restore: %v", err)
	}
	if digestRestored == ([32]byte{}) {
		t.Fatal("empty restore digest")
	}
	if restored.ref != naturalSeamRefV6 {
		t.Fatalf("restored runner bound %+v, want natural/6", restored.ref)
	}
	if !reflect.DeepEqual(restored.Journal(), f6.Journal()) || !reflect.DeepEqual(restored.Checkpoints(), f6.Checkpoints()) {
		t.Fatal("v6 restore changed the journal or the projection")
	}
	sections, _, err := checkpoint.ReadFile(resume)
	if err != nil {
		t.Fatal(err)
	}
	if exported, err := restored.ExportJournal(); err != nil || !bytes.Equal(exported, sections[0].Data) {
		t.Fatalf("restored v6 journal config not carried: %v", err)
	}
	// The handoff carries the ref; the export-time portable-bytes ledger
	// measurement is taken on both runners so the symmetric instrumentation
	// state keeps the continuations comparable.
	if naturalMustHandoff(t, restored).Policy != naturalSeamRefV6 {
		t.Fatal("v6 handoff lost the policy ref")
	}
	if naturalMustHandoff(t, f6).Policy != naturalSeamRefV6 {
		t.Fatal("original v6 handoff lost the policy ref")
	}
	// The restored runner continues in lockstep with the original.
	naturalRunTo(t, restored, 6)
	naturalRunTo(t, f6, 6)
	if !reflect.DeepEqual(restored.Journal(), f6.Journal()) || !reflect.DeepEqual(restored.Checkpoints(), f6.Checkpoints()) {
		t.Fatal("v6 continuation diverged across the restore boundary")
	}
	// Branch: the births intervention on the v6 root keeps the ref identity
	// in the child's manifest, journal re-pin, and restored bindings.
	childPath := filepath.Join(t.TempDir(), "v6-child.bundle")
	if _, err := BranchNaturalCheckpoint(root, naturalSeamExpectation(naturalSeamRefV6, 1, false), true, childPath); err != nil {
		t.Fatalf("v6 branch: %v", err)
	}
	child := naturalSeamManifest(t, childPath)
	if child.policyFormat != strategy.NaturalPolicyFormatV6 || child.policyFingerprint != naturalSeamFingerprint(t, naturalSeamRefV6) ||
		child.depth != 1 || child.parent != digestRoot || !child.births {
		t.Fatalf("v6 branch manifest: format %d depth %d births %v", child.policyFormat, child.depth, child.births)
	}
	childRunner, _, err := RestoreNaturalCheckpoint(childPath, naturalSeamExpectation(naturalSeamRefV6, 1, true))
	if err != nil {
		t.Fatalf("v6 branch child restore: %v", err)
	}
	if childRunner.ref != naturalSeamRefV6 {
		t.Fatalf("branch child bound %+v, want natural/6", childRunner.ref)
	}
}

// TestNaturalPolicySeamBidirectionalRefMismatch proves the mismatch
// rejection both ways: a natural/5 bundle is refused under a natural/6
// expectation and a natural/6 bundle is refused under the zero-value
// (v5-only) expectation — the required old-executable behaviour — with
// nothing published, at checkpoint and journal level alike.
func TestNaturalPolicySeamBidirectionalRefMismatch(t *testing.T) {
	f5 := naturalSeamRun(t, naturalSeamRefV5, false)
	f6 := naturalSeamRun(t, naturalSeamRefV6, false)
	path5, _ := naturalSeamSave(t, f5, "v5-root.bundle", false)
	path6, _ := naturalSeamSave(t, f6, "v6-root.bundle", false)
	if _, _, err := RestoreNaturalCheckpoint(path5, naturalSeamExpectation(naturalSeamRefV6, 1, false)); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("natural/5 bundle accepted under a natural/6 expectation: %v", err)
	}
	// The zero-value expectation IS the old v5-only executable's expectation:
	// refusing the v6 bundle is required behaviour, not a failure.
	if _, _, err := RestoreNaturalCheckpoint(path6, naturalSeamExpectation(strategy.NaturalRef{}, 1, false)); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v6 bundle accepted under a v5-only expectation: %v", err)
	}
	// An unknown expectation ref is refused before any work.
	unknown := naturalSeamExpectation(strategy.NaturalRef{ID: "natural", Version: 10}, 1, false)
	if err := unknown.validate(); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("unknown expectation ref validated: %v", err)
	}
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 7, Workers: 1, PolicyRef: strategy.NaturalRef{ID: "natural", Version: 9}}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatalf("unknown option ref accepted: %v", err)
	}
	// Branch refuses before publication both ways: the destinations must not
	// exist after the refusals.
	dest5 := filepath.Join(t.TempDir(), "never-v5.bundle")
	if _, err := BranchNaturalCheckpoint(path5, naturalSeamExpectation(naturalSeamRefV6, 1, false), true, dest5); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branch mismatched v5 root: %v", err)
	}
	dest6 := filepath.Join(t.TempDir(), "never-v6.bundle")
	if _, err := BranchNaturalCheckpoint(path6, naturalSeamExpectation(strategy.NaturalRef{}, 1, false), true, dest6); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branch mismatched v6 root: %v", err)
	}
	for _, dest := range []string{dest5, dest6} {
		if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("mismatched branch published %s: %v", dest, err)
		}
	}
	// Journal level: the same genesis, the same kernel head, only the policy
	// identity differs — each runner refuses the other's journal.
	head5 := naturalVerifiedHead(t, f5)
	journal5, err := f5.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	journal6, err := f6.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if err := f5.RestoreJournal(journal6, head5); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("v5 runner accepted a natural/6 journal: %v", err)
	}
	if err := f6.RestoreJournal(journal5, head5); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("v6 runner accepted a natural/5 journal: %v", err)
	}
}

// TestNaturalPolicySeamForgedRefAndFingerprintFailClosed proves the
// fail-closed pins: a foreign manifest slot decodes nowhere, a manifest
// whose slot disagrees with its strategy content is refused by the
// ref-resolved verification, a forged fingerprint misses the restore pin,
// and a journal config whose fingerprint disagrees with its declared ref
// never encodes or verifies.
func TestNaturalPolicySeamForgedRefAndFingerprintFailClosed(t *testing.T) {
	f6 := naturalSeamRun(t, naturalSeamRefV6, false)
	path, _ := naturalSeamSave(t, f6, "v6-root.bundle", false)
	expectV6 := naturalSeamExpectation(naturalSeamRefV6, 1, false)
	// (a) A foreign slot (format 7) is not a decodable manifest.
	foreign := naturalRewriteBundle(t, path, func(sections []checkpoint.Section) {
		naturalEditManifest(t, sections, func(m *naturalManifest) { m.policyFormat = 7 })
	})
	if _, _, err := RestoreNaturalCheckpoint(foreign, expectV6); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("foreign policy slot accepted: %v", err)
	}
	// (b) A manifest relabelled natural/5 (slot and fingerprint both moved to
	// the v5 identity) still carries v6 strategy bytes: the reference runner
	// resolves the declared ref, and the content verification fails closed.
	relabelled := naturalRewriteBundle(t, path, func(sections []checkpoint.Section) {
		naturalEditManifest(t, sections, func(m *naturalManifest) {
			m.policyFormat = strategy.NaturalPolicyFormatV5
			m.policyFingerprint = naturalSeamFingerprint(t, naturalSeamRefV5)
		})
	})
	if _, _, err := RestoreNaturalCheckpoint(relabelled, naturalSeamExpectation(naturalSeamRefV5, 1, false)); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v6 bytes under a natural/5 manifest accepted: %v", err)
	}
	// (c) The correct slot with a forged fingerprint misses the restore pin.
	forgedFingerprint := naturalRewriteBundle(t, path, func(sections []checkpoint.Section) {
		naturalEditManifest(t, sections, func(m *naturalManifest) { m.policyFingerprint = [32]byte{} })
	})
	if _, _, err := RestoreNaturalCheckpoint(forgedFingerprint, expectV6); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("forged manifest fingerprint accepted: %v", err)
	}
	// (d) Journal side: a config whose fingerprint disagrees with its
	// declared ref never encodes, and an unknown ref never validates.
	head := naturalVerifiedHead(t, f6)
	disagreed := f6.journalConfig()
	disagreed.PolicyFingerprint = naturalSeamFingerprint(t, naturalSeamRefV5)
	if _, err := EncodeNaturalJournal(f6.Journal(), f6.k, head, f6.sched.Time(), disagreed); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("natural/6 config with a natural/5 fingerprint encoded: %v", err)
	}
	unknown := f6.journalConfig()
	unknown.Policy = strategy.NaturalRef{ID: "natural", Version: 10}
	if _, err := EncodeNaturalJournal(f6.Journal(), f6.k, head, f6.sched.Time(), unknown); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("unknown journal ref encoded: %v", err)
	}
}

// naturalSeamObs builds a structurally complete bag-decision observation for
// founder 1 over patch 1001 at hour 1 (the exact decision-time projection of
// a checkpoint state: post-pulse fields, the gathered bag unit, no denial).
func naturalSeamObs(ref strategy.NaturalRef, energy, capital, wip, granary int64) strategy.NaturalObservation {
	source, err := sim.EntityRefValue(1001)
	if err != nil {
		panic(err)
	}
	return strategy.NaturalObservation{
		Actor: 1, Ref: ref, Hour: 1, WorldVersion: 1,
		Energy: sim.IntegerValue(energy), Hunger: sim.IntegerValue(0), BagUnits: sim.IntegerValue(1),
		Capital: sim.IntegerValue(capital), Wip: sim.IntegerValue(wip), GranaryStock: sim.IntegerValue(granary),
		InvestedUnits: sim.IntegerValue(0), LastBuildHour: sim.IntegerValue(-1), LastStoredMealHour: sim.IntegerValue(-1),
		BagSource: source,
	}
}

// TestNaturalPolicySeamV6CascadeDispatch proves the runner dispatches the
// cascade matching its bound ref on the exact delta cells: at the recorded
// trap state (E=4, granary 0, wip 0) the frozen v5 cascade eats while the
// v6 cascade — and only the v6-bound runner's dispatch — builds, and at the
// survival-floor cell (E=2, granary 1) the flip is the reverse. It also pins
// why the dispatch exists: the frozen v5 Evaluate cannot detect a v6 bound,
// while EvaluateV6 refuses a v5 bound.
func TestNaturalPolicySeamV6CascadeDispatch(t *testing.T) {
	f5 := naturalSeamRun(t, naturalSeamRefV5, false)
	f6 := naturalSeamRun(t, naturalSeamRefV6, false)
	// Delta family (c): E=4, granary 0, wip 0 — v5 eats, v6 re-pioneers.
	obsC := naturalSeamObs(naturalSeamRefV6, 4, 0, 0, 0)
	if got, err := f5.naturalEvaluate(1, naturalSeamObs(naturalSeamRefV5, 4, 0, 0, 0), strategy.NaturalLabourOfferGuard{}); err != nil || got.Kind != strategy.NaturalEat {
		t.Fatalf("v5-bound dispatch at the trap state: kind %d err %v", got.Kind, err)
	}
	if got, err := f6.naturalEvaluate(1, obsC, strategy.NaturalLabourOfferGuard{}); err != nil || got.Kind != strategy.NaturalBuild {
		t.Fatalf("v6-bound dispatch at the trap state: kind %d err %v", got.Kind, err)
	}
	// Delta family (a): E=2, granary 1 — v5 builds off the larder, v6 eats.
	if got, err := f5.naturalEvaluate(1, naturalSeamObs(naturalSeamRefV5, 2, 0, 0, 1), strategy.NaturalLabourOfferGuard{}); err != nil || got.Kind != strategy.NaturalBuild {
		t.Fatalf("v5-bound dispatch at the survival floor: kind %d err %v", got.Kind, err)
	}
	if got, err := f6.naturalEvaluate(1, naturalSeamObs(naturalSeamRefV6, 2, 0, 0, 1), strategy.NaturalLabourOfferGuard{}); err != nil || got.Kind != strategy.NaturalEat {
		t.Fatalf("v6-bound dispatch at the survival floor: kind %d err %v", got.Kind, err)
	}
	// The hazard the dispatch exists for: the frozen v5 Evaluate accepts a
	// v6 bound (its only ref check is obs.Ref against the bound's own ref)
	// and reports the v5 semantics, while EvaluateV6 refuses a v5 bound.
	if got, err := f6.bound[1].Evaluate(obsC); err != nil || got.Kind != strategy.NaturalEat {
		t.Fatalf("frozen v5 cascade no longer accepts a v6 bound: kind %d err %v", got.Kind, err)
	}
	if _, err := f5.bound[1].EvaluateV6(naturalSeamObs(naturalSeamRefV5, 4, 0, 0, 0)); !errors.Is(err, strategy.ErrInvalidNaturalPolicy) {
		t.Fatalf("v6 cascade accepted a v5 bound: %v", err)
	}
	// And the runner's post-evaluation identity check pins the choice to the
	// runner's own ref in every dispatch.
	choice, err := f6.naturalEvaluate(1, obsC, strategy.NaturalLabourOfferGuard{})
	if err != nil || choice.Ref != f6.ref {
		t.Fatalf("dispatched choice carries %+v, want the runner ref %+v", choice.Ref, f6.ref)
	}
}

// TestNaturalPolicySeamV6SmokeBehaviourInEffect runs the v6 policy against
// the v5 control over one organic cell and proves the v6 behaviour is
// actually in effect: the journals are identical until the first delta cell
// — the recorded trap onset, where a founder at E=4 with an empty granary
// and outstanding wip builds under v6 where v5 eats (delta family (d)) —
// and conservation stays green at every hourly checkpoint of both runs.
func TestNaturalPolicySeamV6SmokeBehaviourInEffect(t *testing.T) {
	f5 := naturalSeamRun(t, naturalSeamRefV5, false)
	f6 := naturalSeamRun(t, naturalSeamRefV6, false)
	var flipHour, flipActor = -1, sim.EntityID(0)
	for len(f6.Checkpoints()) < 28 && flipHour < 0 {
		if _, err := f5.Step(context.Background()); err != nil {
			t.Fatalf("v5 control step: %v", err)
		}
		if _, err := f6.Step(context.Background()); err != nil {
			t.Fatalf("v6 step: %v", err)
		}
		// Both runs share the genesis, and both cascades agree on every
		// pre-trap cell, so the journals must be attempt-identical until the
		// first divergent choice - which can also be an attempt the v6 run
		// is missing (a build has no attempt at the decision instant).
		byTime5 := map[sim.SimTime]map[sim.EntityID]string{}
		for _, b := range f5.Journal() {
			m := map[sim.EntityID]string{}
			for _, a := range b.Attempts {
				m[a.Actor] += a.Kind + ";"
			}
			byTime5[b.Time] = m
		}
		for _, b := range f6.Journal() {
			m5, ok := byTime5[b.Time]
			if !ok {
				continue // downstream of the flip: batch topology diverges
			}
			m6 := map[sim.EntityID]string{}
			for _, a := range b.Attempts {
				m6[a.Actor] += a.Kind + ";"
			}
			actors := make([]sim.EntityID, 0, len(m5)+len(m6))
			seen := map[sim.EntityID]bool{}
			for actor := range m5 {
				if !seen[actor] {
					seen[actor] = true
					actors = append(actors, actor)
				}
			}
			for actor := range m6 {
				if !seen[actor] {
					seen[actor] = true
					actors = append(actors, actor)
				}
			}
			sort.Slice(actors, func(i, j int) bool { return actors[i] < actors[j] })
			for _, actor := range actors {
				if m5[actor] != m6[actor] {
					// The decision state is exactly the hour's checkpoint
					// projection (post-pulse, pre-activity), shared by both
					// runs before the flip.
					flipHour = int(b.Time / sim.SimTime(NaturalHour))
					flipActor = actor
					break
				}
			}
			if flipHour >= 0 {
				break
			}
		}
	}
	if flipHour < 0 {
		t.Fatal("no v6 delta cell materialized by h28: the v6 behaviour is not in effect")
	}
	// Let the flip hour fully commit (a build chosen at the decision instant
	// completes within the same hour), then classify it.
	for f6.Checkpoints()[len(f6.Checkpoints())-1].Hour <= flipHour {
		if _, err := f6.Step(context.Background()); err != nil {
			t.Fatalf("v6 step past the flip hour: %v", err)
		}
		if _, err := f5.Step(context.Background()); err != nil {
			t.Fatalf("v5 control step past the flip hour: %v", err)
		}
	}
	st := f6.Checkpoints()[flipHour].Founders[flipActor-1]
	v5Kinds, v6Kinds := "", ""
	bagAt, err := NaturalPhaseTime(flipHour, NaturalPhaseBagDecision)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range f6.Journal() {
		if b.Time == bagAt {
			v6Kinds = naturalSeamKinds(b, flipActor)
		}
	}
	for _, b := range f5.Journal() {
		if b.Time == bagAt {
			v5Kinds = naturalSeamKinds(b, flipActor)
		}
	}
	if v5Kinds != "eat;" || v6Kinds != "" {
		t.Fatalf("first divergence at h%d actor %d is v5=%q v6=%q, want the v5 eat / v6 build flip", flipHour, flipActor, v5Kinds, v6Kinds)
	}
	// Delta family (c)/(d) (the re-pioneering cell and the corrigendum's
	// rule-implied family): E=4, granary 0 - v5 eats, v6 builds.
	if st.Body.Energy != 4 || st.Granary.Stock != 0 || st.Worksite.Capital+st.Worksite.Wip >= 8 {
		t.Fatalf("first flip state E=%d granary=%d wip=%d k=%d is not the recorded delta cell", st.Body.Energy, st.Granary.Stock, st.Worksite.Wip, st.Worksite.Capital)
	}
	// The v6 runner actually built: its build attempt commits at the hour's
	// completion instant; the v5 control ate the unit instead.
	start, err := NaturalPhaseTime(flipHour, NaturalPhaseBuildStart)
	if err != nil {
		t.Fatal(err)
	}
	completion := NaturalBuildCompletion(start)
	if !naturalSeamHasAttempt(f6, completion, flipActor, "build") {
		t.Fatalf("v6 runner did not commit the build for actor %d at t=%v (hour %d)", flipActor, completion, flipHour)
	}
	if !naturalSeamHasAttempt(f5, bagAt, flipActor, "eat") {
		t.Fatalf("v5 control did not commit the eat at the bag decision")
	}
	// Conservation stays green at every hourly checkpoint of both runs, and
	// every v6 attempt sits on the frozen clock.
	naturalAssertConservation(t, f5)
	naturalAssertConservation(t, f6)
	naturalAssertJournalClock(t, f6)
}

// naturalSeamKinds joins one batch's attempt kinds for one actor.
func naturalSeamKinds(b NaturalBatch, actor sim.EntityID) string {
	out := ""
	for _, a := range b.Attempts {
		if a.Actor == actor {
			out += a.Kind + ";"
		}
	}
	return out
}

// naturalSeamHasAttempt reports whether the runner's journal commits an
// attempt of the given kind for the actor at the exact timestamp.
func naturalSeamHasAttempt(f *Natural, at sim.SimTime, actor sim.EntityID, kind string) bool {
	for _, b := range f.Journal() {
		if b.Time != at {
			continue
		}
		for _, a := range b.Attempts {
			if a.Actor == actor && a.Kind == kind {
				return true
			}
		}
	}
	return false
}
