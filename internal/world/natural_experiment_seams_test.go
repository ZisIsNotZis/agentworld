package world

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"agentworld/internal/sim"
)

// The experiment lane's two declared seams (ticket 15, supervisor-approved
// scoped exception): the exported founder-tenure fixture constructor and the
// declared cultivation-ablation intervention. Both are run identity, never
// mechanism or policy changes; these tests pin exactly that.

// TestNaturalNewNaturalFixtureExportMatchesInternalPath proves the exported
// constructor builds exactly the declared deterministic function of its
// arguments: identical checkpoints and an identical published continuation
// digest against the internal path, restore only under the declared
// fixture flag, and bidirectional fixture/neutral bundle substitution
// refusal.
func TestNaturalNewNaturalFixtureExportMatchesInternalPath(t *testing.T) {
	const seed = uint64(5)
	exported, err := NewNaturalFixture(2, 8, seed, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	internal, err := newNaturalFixtureRunner(2, 8, seed, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, exported, 3)
	naturalStepToBoundary(t, internal, 3)
	if !reflect.DeepEqual(exported.Checkpoints(), internal.Checkpoints()) {
		t.Fatal("exported fixture constructor diverged from the internal path")
	}
	exportedPath := filepath.Join(t.TempDir(), "exported-fixture.bundle")
	exportedDigest, err := exported.SaveResumeCheckpoint(exportedPath)
	if err != nil {
		t.Fatal(err)
	}
	internalPath := filepath.Join(t.TempDir(), "internal-fixture.bundle")
	internalDigest, err := internal.SaveResumeCheckpoint(internalPath)
	if err != nil {
		t.Fatal(err)
	}
	if exportedDigest != internalDigest {
		t.Fatalf("fixture continuation digests diverge: %x vs %x", exportedDigest, internalDigest)
	}
	restored, digest, err := RestoreNaturalCheckpoint(exportedPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 4, Births: true, FounderTenure: true})
	if err != nil || digest != exportedDigest {
		t.Fatalf("fixture restore: digest=%x want=%x err=%v", digest, exportedDigest, err)
	}
	if !reflect.DeepEqual(naturalMustHandoff(t, restored), naturalMustHandoff(t, internal)) {
		t.Fatal("restored fixture diverged from the internal fixture at the boundary")
	}
	// Bidirectional substitution refusal: a fixture bundle is not a neutral
	// bundle and vice versa, at every restore and branch seam.
	if _, _, err := RestoreNaturalCheckpoint(exportedPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: true}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("fixture bundle restored without the fixture flag: %v", err)
	}
	_, neutralPath, _ := naturalCaptureRoot(t, seed, false, 1)
	if _, _, err := RestoreNaturalCheckpoint(neutralPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: false, FounderTenure: true}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("neutral bundle restored as a fixture: %v", err)
	}
	if _, err := BranchNaturalCheckpoint(exportedPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: true, FounderTenure: true}, true, filepath.Join(t.TempDir(), "child.bundle")); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branched from a fixture bundle: %v", err)
	}
	// The fixture and the ablation are never a declared combination.
	if _, _, err := RestoreNaturalCheckpoint(exportedPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: true, FounderTenure: true, CultivationDisabled: true}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("fixture+ablation combination accepted: %v", err)
	}
}

// TestNaturalCultivationDisabledAblation runs the declared ablation
// intervention over the poverty-trap window: the frozen policy is unchanged
// (it keeps choosing Build while the bag-decision floor holds), the runner
// routes every such choice through the documented denial loop, and no
// rule-505 event, build attempt, or capital can exist. The journal's run
// identity carries the flag bidirectionally.
func TestNaturalCultivationDisabledAblation(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: false, CultivationDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, f, 24)
	denialEats, builds := 0, 0
	for _, batch := range f.Journal() {
		for _, a := range batch.Attempts {
			switch a.Kind {
			case "build":
				builds++
			case "build-denied-eat":
				denialEats++
			}
		}
	}
	if builds != 0 {
		t.Fatalf("ablation recorded %d build attempts", builds)
	}
	if denialEats == 0 {
		t.Fatal("ablation denial loop never engaged; the intervention is vacuous in this window")
	}
	for _, ev := range f.k.Events() {
		if ev.Rule == NaturalBuildRule {
			t.Fatalf("ablation committed a rule-505 event at %v", ev.Time)
		}
	}
	for _, c := range f.Checkpoints() {
		for i := range c.Founders {
			if c.Founders[i].Worksite.Capital != 0 || c.Founders[i].Worksite.Wip != 0 {
				t.Fatalf("hour %d: ablation capital leaked: %+v", c.Hour, c.Founders[i].Worksite)
			}
		}
		if c.Balance.Capital != 0 || c.Balance.Wip != 0 || c.Balance.TenuredSlots != 0 {
			t.Fatalf("hour %d: ablation aggregate capital/tenure leaked: %+v", c.Hour, c.Balance)
		}
	}
	// The journal export pins the ablation in its run identity: it decodes
	// only under the declared flag.
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	journal, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	config := f.journalConfig()
	if !config.CultivationDisabled {
		t.Fatal("journal config lost the ablation flag")
	}
	if _, err := DecodeNaturalJournal(journal, f.k, head, f.sched.Time(), config); err != nil {
		t.Fatalf("ablation journal refused under its own identity: %v", err)
	}
	wrong := config
	wrong.CultivationDisabled = false
	if _, err := DecodeNaturalJournal(journal, f.k, head, f.sched.Time(), wrong); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("ablation journal accepted under a non-ablation identity: %v", err)
	}
}

// TestNaturalCultivationDisabledGuardsFireOnForgedInput proves the
// fail-loud ablation guards are binding: a forged rule-505 event or a
// committed build attempt is a typed runner error, any other input passes.
func TestNaturalCultivationDisabledGuardsFireOnForgedInput(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: false, CultivationDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []sim.RuleID{NaturalRegenSplitRule, NaturalBuildRule} {
		err := f.naturalAblationEventGuard(rule)
		if rule == NaturalBuildRule && err == nil {
			t.Fatal("forged rule-505 event passed the ablation guard")
		}
		if rule != NaturalBuildRule && err != nil {
			t.Fatalf("unrelated rule %d rejected: %v", rule, err)
		}
	}
	for _, kind := range []string{"gather", "build", "eat"} {
		err := f.naturalAblationAttemptGuard(kind)
		if kind == "build" && err == nil {
			t.Fatal("forged build attempt passed the ablation guard")
		}
		if kind != "build" && err != nil {
			t.Fatalf("unrelated attempt kind %q rejected: %v", kind, err)
		}
	}
}

// TestNaturalCultivationDisabledRunIdentity pins the checkpoint-side run
// identity: an ablation root publishes and restores only under the declared
// flag, the Branch child keeps the flag, and the combination with the
// founder-tenure fixture is refused.
func TestNaturalCultivationDisabledRunIdentity(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 1, Workers: 1, Births: false, CultivationDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, f, 0)
	rootPath := filepath.Join(t.TempDir(), "ablation-root.bundle")
	digest, err := f.SaveCheckpoint(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, restoredDigest, err := RestoreNaturalCheckpoint(rootPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 1, Workers: 4, Births: false, CultivationDisabled: true}); err != nil || restoredDigest != digest {
		t.Fatalf("ablation root restore: err=%v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(rootPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 1, Workers: 1, Births: false}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("ablation root restored without the ablation flag: %v", err)
	}
	// The recorded intervention discipline still applies on top of the
	// ablation flag: the child inherits it and must be declared at restore.
	childPath := filepath.Join(t.TempDir(), "ablation-branch-enabled.bundle")
	if _, err := BranchNaturalCheckpoint(rootPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 1, Workers: 1, Births: false, CultivationDisabled: true}, true, childPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreNaturalCheckpoint(childPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 1, Workers: 1, Births: true, CultivationDisabled: true}); err != nil {
		t.Fatalf("ablation branch child restore: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(childPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 1, Workers: 1, Births: true}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("ablation branch child restored without the ablation flag")
	}
	// The fixture+ablation combination is not a declared scenario anywhere.
	if _, _, err := RestoreNaturalCheckpoint(rootPath, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 1, Workers: 1, Births: false, CultivationDisabled: true, FounderTenure: true}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("ablation+fixture combination accepted: %v", err)
	}
}
