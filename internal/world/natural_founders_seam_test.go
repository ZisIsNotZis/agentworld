package world

// Ticket-18 founding-band fixture axis (L1 world seam). The design frozen in
// .scratch/18-surplus-via-reproduction/spec.md is binding: the Founders axis
// (NaturalOptions.Founders / NaturalRestoreOptions.Founders, zero value ≡
// the full frozen block 8P) is run identity carried by the checkpoint
// manifest's omitempty founders field and the journal config with
// bidirectional mismatch refusal; genesis materializes exactly f founder
// rows from founder-stream positions 0..f−1; G3 becomes 11·f; and the
// default publishes byte-identical bundles and journals to the pre-change
// behavior (digest pins captured at b155a37 in the detached worktree
// /tmp/aw-pre18 via `git worktree add --detach`, mirroring the ticket-17
// mechanism-seam capture discipline).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"agentworld/internal/strategy"
)

// Pre-change (b155a37) golden digests of the default founding-band world,
// captured before any axis edit in the detached worktree at b155a37: the h0
// root bundle, the hour-5 resume bundle, and the exported journal digests at
// hours 5 and 9 of (P=2, R=8, seed 0, births off, one worker). Both default
// spellings must reproduce every pin.
const (
	naturalFoundersGoldenRoot     = "97a14338dba9b92c68204571472a1c6ef168be35cea52cbb673d16ecfe8faa94"
	naturalFoundersGoldenResume5  = "875c51c4884ba59331896c8225d694332ea3ee1c939d463a6704cfce5711f343"
	naturalFoundersGoldenJournal5 = "8c17b4239ac5e7ab3577862c594ad4ded3507fcc33093d424957994301983400"
	naturalFoundersGoldenJournal9 = "49d473f8aa5479df41fe0e2f628e29c2cb26e41b37fb22748d553a55439c0ac0"
)

// naturalFoundersGate3Anchor pins the frozen (mechanism v6, natural/7,
// f=16) A cell's patch-1001-visible columns, digest-computed from the
// recorded evidence .tmp/natural7-n7-A-r8-s0-w1.json: the hourly world regen
// split (granted/produced/unrealized per pulse, h0..h168), the hourly world
// alive count (h0..h168 — zero births ever filed in this cell, so founders
// only), and the final raw rows of the eight patch-1001 founders (IDs 1..8;
// 29 authoritative state fields each; reporter-derived aggregates such as
// wild_meals are deliberately excluded). Byte scheme, identical in the
// capture script and in naturalFoundersGate3Digest below: for h=0..168 one
// "regen h granted produced unrealized" line and one "alive h n" line, then
// one "actor …" line per patch-1001 founder ascending by ID, every line
// '\n'-terminated, sha256 over the whole stream.
const naturalFoundersGate3Anchor = "8745d98f859fdbfcbe93def2d3e3c7d2e2028104da907c8491fae62ba43569fd"

func naturalFoundersDigest(t *testing.T, digest [32]byte) string {
	t.Helper()
	return hex.EncodeToString(digest[:])
}

func naturalFoundersJournalDigest(t *testing.T, f *Natural) string {
	t.Helper()
	data, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	return naturalFoundersDigest(t, sha256.Sum256(data))
}

func naturalFoundersManifest(t *testing.T, path string) naturalManifest {
	t.Helper()
	sections, _, m, err := naturalCheckedSections(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = sections
	return m
}

func naturalFoundersOptions(seed uint64, births bool, founders int) NaturalOptions {
	return NaturalOptions{Patches: 2, R: 8, Seed: seed, Workers: 1, Births: births,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: founders}
}

func naturalFoundersNeutralRunner(t *testing.T, founders int) *Natural {
	t.Helper()
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Founders: founders})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	return f
}

// TestNaturalFoundersSeamZeroDigestIdentity proves the axis default is
// byte-inert: the zero value reproduces the pre-change b155a37 pins (h0
// root, h5 resume, journal at h5 and h9), the explicit 8P spelling publishes
// byte-identical bundles and journals to the zero value, and both default
// spellings encode manifests with the founders field omitted.
func TestNaturalFoundersSeamZeroDigestIdentity(t *testing.T) {
	dir := t.TempDir()
	def := naturalFoundersNeutralRunner(t, 0)
	root, err := def.SaveCheckpoint(dir + "/root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if got := naturalFoundersDigest(t, root); got != naturalFoundersGoldenRoot {
		t.Fatalf("zero-value root digest %s diverges from the pre-change pin", got)
	}
	if m := naturalFoundersManifest(t, dir+"/root.bundle"); m.founders != 0 {
		t.Fatalf("default manifest carries founders %d, want omitted (0)", m.founders)
	}
	naturalRunTo(t, def, 5)
	resume, err := def.SaveResumeCheckpoint(dir + "/resume5.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if got := naturalFoundersDigest(t, resume); got != naturalFoundersGoldenResume5 {
		t.Fatalf("zero-value resume digest %s diverges from the pre-change pin", got)
	}
	if got := naturalFoundersJournalDigest(t, def); got != naturalFoundersGoldenJournal5 {
		t.Fatalf("zero-value journal digest %s diverges from the pre-change pin", got)
	}
	naturalRunTo(t, def, 9)
	if got := naturalFoundersJournalDigest(t, def); got != naturalFoundersGoldenJournal9 {
		t.Fatalf("zero-value journal digest %s diverges from the pre-change pin", got)
	}
	// The explicit 8P spelling is the same world, not a second one: every
	// bundle and journal byte-identical to the zero-value run.
	explicit := naturalFoundersNeutralRunner(t, NaturalSlotsPerPatch*2)
	explicitRoot, err := explicit.SaveCheckpoint(dir + "/explicit-root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if explicitRoot != root {
		t.Fatal("explicit-8P root digest diverges from the zero-value root")
	}
	if m := naturalFoundersManifest(t, dir+"/explicit-root.bundle"); m.founders != 0 {
		t.Fatalf("explicit-8P manifest carries founders %d, want omitted (0)", m.founders)
	}
	naturalRunTo(t, explicit, 5)
	explicitResume, err := explicit.SaveResumeCheckpoint(dir + "/explicit-resume5.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if explicitResume != resume {
		t.Fatal("explicit-8P resume digest diverges from the zero-value resume")
	}
	if got := naturalFoundersJournalDigest(t, explicit); got != naturalFoundersGoldenJournal5 {
		t.Fatalf("explicit-8P journal digest %s diverges from the pre-change pin", got)
	}
}

// TestNaturalFoundersSeamBandRoundTripAndMismatchRefusals proves the band is
// run identity end to end: an f=12 bundle round-trips under an f=12
// expectation and refuses the default (and vice versa) before any restore
// work; Branch refuses both directions before publishing anything; the
// journal identity refuses cross-band restores both ways; and the explicit
// 8P spelling restores under both default spellings.
func TestNaturalFoundersSeamBandRoundTripAndMismatchRefusals(t *testing.T) {
	dir := t.TempDir()
	f12, err := NewNatural(naturalFoundersOptions(5, false, 12))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f12.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	path12 := dir + "/f12-root.bundle"
	if _, err := f12.SaveCheckpoint(path12); err != nil {
		t.Fatal(err)
	}
	if m := naturalFoundersManifest(t, path12); m.founders != 12 {
		t.Fatalf("f12 manifest carries founders %d, want 12", m.founders)
	}
	f16, err := NewNatural(naturalFoundersOptions(5, false, 0))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f16.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	path16 := dir + "/f16-root.bundle"
	if _, err := f16.SaveCheckpoint(path16); err != nil {
		t.Fatal(err)
	}
	fx16, err := NewNatural(naturalFoundersOptions(5, false, NaturalSlotsPerPatch*2))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := fx16.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	pathExplicit := dir + "/explicit-root.bundle"
	if _, err := fx16.SaveCheckpoint(pathExplicit); err != nil {
		t.Fatal(err)
	}
	if m := naturalFoundersManifest(t, pathExplicit); m.founders != 0 {
		t.Fatalf("explicit-8P manifest carries founders %d, want omitted", m.founders)
	}
	ref := strategy.FrozenNaturalPolicy7().Ref
	expect12 := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 5, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6, Founders: 12}
	expect0 := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 5, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6}
	expectExplicit := expect0
	expectExplicit.Founders = NaturalSlotsPerPatch * 2
	if restored, _, err := RestoreNaturalCheckpoint(path12, expect12); err != nil || restored.founders != 12 || len(restored.roster) != 12 {
		t.Fatalf("f12 bundle did not round-trip under the f12 expectation: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(path12, expect0); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("f12 bundle accepted under the default expectation")
	}
	if _, _, err := RestoreNaturalCheckpoint(path16, expect12); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("default bundle accepted under the f12 expectation")
	}
	for _, expectation := range []NaturalRestoreOptions{expect0, expectExplicit} {
		if _, _, err := RestoreNaturalCheckpoint(path16, expectation); err != nil {
			t.Fatalf("default bundle refused under founders=%d: %v", expectation.Founders, err)
		}
		if _, _, err := RestoreNaturalCheckpoint(pathExplicit, expectation); err != nil {
			t.Fatalf("explicit-8P bundle refused under founders=%d: %v", expectation.Founders, err)
		}
	}
	// Branch refuses both directions before publication; the destinations
	// must not exist afterwards.
	dest1, dest2 := dir+"/never-1.bundle", dir+"/never-2.bundle"
	if _, err := BranchNaturalCheckpoint(path12, expect0, true, dest1); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branch accepted a mismatched f12 root: %v", err)
	}
	if _, err := BranchNaturalCheckpoint(path16, expect12, true, dest2); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branch accepted a mismatched default root: %v", err)
	}
	for _, dest := range []string{dest1, dest2} {
		if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("mismatched branch published %s: %v", dest, err)
		}
	}
	dest3 := dir + "/f12-child.bundle"
	if _, err := BranchNaturalCheckpoint(path12, expect12, true, dest3); err != nil {
		t.Fatalf("matched f12 branch failed: %v", err)
	}
	if m := naturalFoundersManifest(t, dest3); m.founders != 12 {
		t.Fatalf("branch child manifest carries founders %d, want 12", m.founders)
	}
	// Journal identity: a cross-band journal never restores, both ways.
	_, head12, err := f12.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	_, head16, err := f16.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	journal12, err := f12.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	journal16, err := f16.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if err := f16.RestoreJournal(journal12, head16); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("default runner accepted an f12 journal: %v", err)
	}
	if err := f12.RestoreJournal(journal16, head12); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("f12 runner accepted a default journal: %v", err)
	}
}

// TestNaturalFoundersSeamStreamPositionPin proves the founder-stream prefix
// rule: founder i of an f=12 band draws lifetime stream position i, so its
// lifetime equals the frozen (default) world's founder i lifetime for every
// i ≤ f — the recorded founder lifetimes are a prefix of the frozen ones.
func TestNaturalFoundersSeamStreamPositionPin(t *testing.T) {
	frozen := naturalFoundersNeutralRunner(t, 0)
	band := naturalFoundersNeutralRunner(t, 12)
	frozenFounders := frozen.Checkpoints()[0].Founders
	bandFounders := band.Checkpoints()[0].Founders
	if len(bandFounders) != 12 {
		t.Fatalf("f12 genesis materialized %d founder rows, want 12", len(bandFounders))
	}
	for i := range bandFounders {
		if bandFounders[i].Genome.DeathHour != frozenFounders[i].Genome.DeathHour {
			t.Fatalf("founder %d lifetime %d diverges from the frozen world's founder %d lifetime %d",
				i+1, bandFounders[i].Genome.DeathHour, i+1, frozenFounders[i].Genome.DeathHour)
		}
	}
}

// TestNaturalFoundersSeamSeatingF12 proves the frozen placement at the
// gating cell: founders 1–8 seat on patch 1001 and founders 9–12 on patch
// 1002 by the frozen ID-placed home mapping, and every other genesis
// property stays frozen (initial endowment, idle request, neutral genome,
// wild slots, the patch-1001/1002 yields, and the genesis allocator at the
// frozen last fixed entity 2000+8P regardless of f).
func TestNaturalFoundersSeamSeatingF12(t *testing.T) {
	band := naturalFoundersNeutralRunner(t, 12)
	check := band.Checkpoints()[0]
	if len(check.Founders) != 12 || len(check.Newborns) != 0 {
		t.Fatalf("f12 genesis roster: %d founders, %d newborns", len(check.Founders), len(check.Newborns))
	}
	home1001, err := NaturalPatchID(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	home1002, err := NaturalPatchID(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range check.Founders {
		actor := i + 1
		want := home1001
		if actor > NaturalSlotsPerPatch {
			want = home1002
		}
		if a.Bag.Source != want {
			t.Fatalf("founder %d seats on %d, want %d", actor, a.Bag.Source, want)
		}
		if a.Body.Energy != NaturalFounderInitialEnergy || a.Body.LastGatherHour != NaturalNeverHour {
			t.Fatalf("founder %d body not at the frozen initial state: %+v", actor, a.Body)
		}
		if a.Bag.Units != 0 || a.Worksite != (NaturalWorksiteState{LastBuildHour: NaturalNeverHour}) {
			t.Fatalf("founder %d bag/worksite not at the frozen initial state", actor)
		}
		if a.Granary != (NaturalGranaryState{LastStoredMealHour: NaturalNeverHour}) {
			t.Fatalf("founder %d granary not at the frozen initial state: %+v", actor, a.Granary)
		}
		if a.Genome.ParentA != 0 || a.Genome.ParentB != 0 || a.Genome.BirthHour != 0 ||
			a.Genome.DiedHour != NaturalNeverHour || a.Genome.BirthGranaryPaid != 0 {
			t.Fatalf("founder %d genome not at the frozen initial state: %+v", actor, a.Genome)
		}
		if a.Request != NaturalIdleRequestState() {
			t.Fatalf("founder %d request not idle: %+v", actor, a.Request)
		}
	}
	if check.AllocatorLast != 2016 { // the frozen genesis base 2000+8P, regardless of f
		t.Fatalf("genesis allocator last %d, want the frozen 2000+8P base", check.AllocatorLast)
	}
	for _, patch := range check.Patches {
		if patch.Yield != 8 {
			t.Fatalf("patch yield %d diverges from the declared R", patch.Yield)
		}
	}
	for _, slots := range check.Slots {
		if len(slots) != NaturalSlotsPerPatch {
			t.Fatalf("patch carries %d slots", len(slots))
		}
		for _, slot := range slots {
			// Wild and ownerless (the frozen placement); the h0 pulse's own
			// regen split has already filled every empty wild slot with its
			// frozen single unit (world wild fill 16 at h0 in the frozen A cell).
			if slot.Mode != NaturalTenureWild || slot.Owner != 0 || slot.Stock != 1 || slot.Cultivated != 0 {
				t.Fatalf("slot not at the frozen wild genesis after the h0 pulse: %+v", slot)
			}
		}
	}
}

// TestNaturalFoundersSeamBoundsAndGateMutations proves the derived bound is
// live at every seam: the options/restore validation refuses 8P+1, negative
// bands, and cross-P bands; the conservation gate's new precondition refuses
// an empty founder block, a block above the frozen partition, and a
// cross-patch reordering of the positional founder rows (founder i's row
// must be the prefix row of founder i). Within-patch lifetime permutation is
// content, not identity — the gate validates rows, the stream draws them —
// and stays admitted by design.
func TestNaturalFoundersSeamBoundsAndGateMutations(t *testing.T) {
	if !NaturalFoundersValid(2, 0) || !NaturalFoundersValid(2, 1) || !NaturalFoundersValid(2, 16) {
		t.Fatal("admitted bands refused")
	}
	if NaturalFoundersValid(2, 17) || NaturalFoundersValid(2, -1) || NaturalFoundersValid(1, 9) || !NaturalFoundersValid(1, 8) {
		t.Fatal("derived bound admits an out-of-band value")
	}
	for _, founders := range []int{17, -1, -8} {
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Founders: founders}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("NewNatural admitted founders %d", founders)
		}
	}
	if _, err := NewNatural(NaturalOptions{Patches: 1, R: 8, Seed: 0, Workers: 1, Founders: 12}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatal("NewNatural admitted a cross-P band (12 founders on P=1)")
	}
	if _, err := newNaturalReferenceRunner(NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Founders: 17}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("restore options admitted founders 8P+1")
	}
	if _, err := newNaturalReferenceRunner(NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, FounderTenure: true, Founders: 1}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("restore options admitted the incoherent f=1 fixture (the pre-history seats founders 1 and 2)")
	}
	// Gate mutations over a real neutral genesis: the bound and the
	// positional-prefix rule are live.
	patches, slots, founders, err := naturalGenesisProjection(2, 8, 0, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	last := mustNaturalGenesisLast(2)
	gate := func(rows []NaturalActorState) error {
		_, err := NaturalCheckConservationFor(NaturalMechanismV5, 2, patches, slots, rows, nil, last)
		return err
	}
	if err := gate(founders); err != nil {
		t.Fatalf("frozen genesis refused: %v", err)
	}
	if err := gate(append(append([]NaturalActorState(nil), founders...), founders[len(founders)-1])); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("gate admitted a founder block above the frozen partition")
	}
	if err := gate(founders[:0]); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("gate admitted an empty founder block")
	}
	reordered := append([]NaturalActorState(nil), founders...)
	reordered[0], reordered[8] = reordered[8], reordered[0]
	if err := gate(reordered); !errors.Is(err, ErrNaturalContract) {
		t.Fatal("gate admitted a cross-patch reordering of the founder rows")
	}
	permutation := append([]NaturalActorState(nil), founders...)
	permutation[0], permutation[1] = permutation[1], permutation[0]
	if reflect.DeepEqual(permutation[0], founders[0]) {
		t.Fatal("frozen stream produced identical adjacent lifetimes; the content mutation proves nothing")
	}
	if err := gate(permutation); err != nil {
		t.Fatalf("gate refused a within-patch lifetime permutation: %v", err)
	}
}

// TestNaturalFoundersSeamFixtureForConfig proves the fixture seam: f=0
// through NewNaturalFixtureForConfig is digest-identical to the existing
// wrappers, the E12 shape (f=12, mechanism v6, natural/7) constructs, seats
// 1–8/9–12 and round-trips, and the incoherent bands are refused.
func TestNaturalFoundersSeamFixtureForConfig(t *testing.T) {
	dir := t.TempDir()
	// The fixture has no neutral h0 root (its first pulse is
	// naturalFixtureStartHour), so the default-preservation comparison is at
	// a pulse-boundary resume, exactly where the frozen fixture pins live.
	existing, err := newNaturalFixtureRunner(2, 8, 3, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	seam, err := NewNaturalFixtureForConfig(2, 8, 3, true, 2, strategy.FrozenNaturalPolicy().Ref, NaturalMechanismV5, 0)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, existing, 6)
	naturalStepToBoundary(t, seam, 6)
	existingRoot, err := existing.SaveResumeCheckpoint(dir + "/fixture-existing.bundle")
	if err != nil {
		t.Fatal(err)
	}
	seamRoot, err := seam.SaveResumeCheckpoint(dir + "/fixture-seam.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if seamRoot != existingRoot {
		t.Fatal("f=0 fixture seam diverges from the existing wrapper")
	}
	if naturalFoundersJournalDigest(t, seam) != naturalFoundersJournalDigest(t, existing) {
		t.Fatal("f=0 fixture seam journal diverges from the existing wrapper")
	}
	e12, err := NewNaturalFixtureForConfig(2, 8, 3, true, 2, strategy.FrozenNaturalPolicy7().Ref, NaturalMechanismV6, 12)
	if err != nil {
		t.Fatal(err)
	}
	if e12.founders != 12 || len(e12.roster) != 12 {
		t.Fatalf("E12 fixture carries founders %d roster %d", e12.founders, len(e12.roster))
	}
	home1001, err := NaturalPatchID(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	home1002, err := NaturalPatchID(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	frozenFixture, err := newNaturalFixtureRunner(2, 8, 3, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, frozenFixture, 2)
	frozenRows := frozenFixture.Checkpoints()[0].Founders
	naturalStepToBoundary(t, e12, 2)
	rows := e12.Checkpoints()[0].Founders
	for i := range rows {
		want := home1001
		if i+1 > NaturalSlotsPerPatch {
			want = home1002
		}
		if rows[i].Bag.Source != want {
			t.Fatalf("fixture founder %d seats on %d, want %d", i+1, rows[i].Bag.Source, want)
		}
		if rows[i].Genome.DeathHour != frozenRows[i].Genome.DeathHour {
			t.Fatalf("fixture founder %d lifetime diverges from the frozen fixture's founder %d", i+1, i+1)
		}
	}
	naturalStepToBoundary(t, e12, 6)
	resume := t.TempDir() + "/e12-resume.bundle"
	if _, err := e12.SaveResumeCheckpoint(resume); err != nil {
		t.Fatal(err)
	}
	if m := naturalFoundersManifest(t, resume); m.founders != 12 {
		t.Fatalf("E12 resume manifest carries founders %d, want 12", m.founders)
	}
	expect := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 3, Workers: 1, Births: true, FounderTenure: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6, Founders: 12}
	if _, _, err := RestoreNaturalCheckpoint(resume, expect); err != nil {
		t.Fatalf("E12 resume refused under the E12 expectation: %v", err)
	}
	expectDefault := expect
	expectDefault.Founders = 0
	if _, _, err := RestoreNaturalCheckpoint(resume, expectDefault); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("E12 resume accepted under the default expectation")
	}
	for _, founders := range []int{1, 17} {
		if _, err := NewNaturalFixtureForConfig(2, 8, 3, true, 2, strategy.FrozenNaturalPolicy().Ref, NaturalMechanismV5, founders); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("fixture seam admitted founders %d", founders)
		}
	}
}

// naturalFoundersGate3Actor is one patch-1001 founder's authoritative row at
// the final hour, in the exact field order of the anchor digest.
type naturalFoundersGate3Actor struct {
	ID, Home, Energy, Hunger, BasalSpent, CapLost, LastGather, Bag, Capital,
	Wip, WearDebt, Invested, PointsCreated, PointsDecayed, LastBuild,
	Stock, CultivatedTotal, StoredMeals, LastStoredMeal, SpoilDebt, Spoiled,
	LocusM, LocusY, LocusW, BirthHour, DeathHour, DiedHour, BirthGranaryPaid,
	Provisioned int64
}

func naturalFoundersGate3Digest(cultivated, wild, unrealized, alive []int64, actors []naturalFoundersGate3Actor) string {
	h := sha256.New()
	for hour := range alive {
		fmt.Fprintf(h, "regen %d %d %d %d\n", hour, cultivated[hour], wild[hour], unrealized[hour])
	}
	for hour := range alive {
		fmt.Fprintf(h, "alive %d %d\n", hour, alive[hour])
	}
	for _, a := range actors {
		fmt.Fprintf(h, "actor %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d\n",
			a.ID, a.Home, a.Energy, a.Hunger, a.BasalSpent, a.CapLost, a.LastGather, a.Bag, a.Capital,
			a.Wip, a.WearDebt, a.Invested, a.PointsCreated, a.PointsDecayed, a.LastBuild,
			a.Stock, a.CultivatedTotal, a.StoredMeals, a.LastStoredMeal, a.SpoilDebt, a.Spoiled,
			a.LocusM, a.LocusY, a.LocusW, a.BirthHour, a.DeathHour, a.DiedHour, a.BirthGranaryPaid,
			a.Provisioned)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func naturalFoundersGate3WorldColumns(t *testing.T, f *Natural) string {
	t.Helper()
	checks := f.Checkpoints()
	if len(checks) != 169 {
		t.Fatalf("run committed %d hourly checkpoints, want 169", len(checks))
	}
	cultivated := make([]int64, 169)
	wild := make([]int64, 169)
	unrealized := make([]int64, 169)
	alive := make([]int64, 169)
	for hour, check := range checks {
		cultivated[hour] = check.Balance.Granted
		wild[hour] = check.Balance.Produced
		unrealized[hour] = check.Balance.Unrealized
		if hour > 0 {
			cultivated[hour] -= checks[hour-1].Balance.Granted
			wild[hour] -= checks[hour-1].Balance.Produced
			unrealized[hour] -= checks[hour-1].Balance.Unrealized
		}
		for _, a := range check.Founders {
			if naturalAlive(a) {
				alive[hour]++
			}
		}
	}
	final := checks[168]
	actors := make([]naturalFoundersGate3Actor, 0, 8)
	for i := 0; i < 8; i++ {
		a := final.Founders[i]
		actors = append(actors, naturalFoundersGate3Actor{
			ID: int64(i + 1), Home: int64(a.Bag.Source), Energy: a.Body.Energy, Hunger: a.Body.Hunger,
			BasalSpent: a.Body.BasalSpent, CapLost: a.Body.CapLost, LastGather: a.Body.LastGatherHour,
			Bag: a.Bag.Units, Capital: a.Worksite.Capital, Wip: a.Worksite.Wip, WearDebt: a.Worksite.WearDebt,
			Invested: a.Worksite.InvestedUnits, PointsCreated: a.Worksite.PointsCreated,
			PointsDecayed: a.Worksite.PointsDecayed, LastBuild: a.Worksite.LastBuildHour,
			Stock: a.Granary.Stock, CultivatedTotal: a.Granary.CultivatedTotal, StoredMeals: a.Granary.StoredMeals,
			LastStoredMeal: a.Granary.LastStoredMealHour, SpoilDebt: a.Granary.SpoilDebt, Spoiled: a.Granary.Spoiled,
			LocusM: a.Genome.LocusM, LocusY: a.Genome.LocusY, LocusW: a.Genome.LocusW,
			BirthHour: a.Genome.BirthHour, DeathHour: a.Genome.DeathHour, DiedHour: a.Genome.DiedHour,
			BirthGranaryPaid: a.Genome.BirthGranaryPaid, Provisioned: a.Granary.Provisioned,
		})
	}
	return naturalFoundersGate3Digest(cultivated, wild, unrealized, alive, actors)
}

// TestNaturalFoundersSeamGate3Patch1001Identity is the Gate-3 harness of the
// frozen ladder. What is compared, stated explicitly:
//
//  1. The recorded anchor: the frozen (mechanism v6, natural/7, f=16) A
//     cell's patch-1001-visible columns (hourly world regen split, hourly
//     world alive count, and the final raw rows of patch-1001 founders
//     1..8), digest-pinned from the recorded evidence above.
//  2. The live f=16 twin of the same cell recomputes those columns from its
//     own hourly checkpoints and must reproduce the anchor — the
//     recorded-byte anchoring of this gate. When the recorded file is
//     present in the tree, its columns are digest-computed identically and
//     must equal the pin as well.
//  3. The live f=12 twin's patch-1001 columns — the patch-1001 slice of the
//     hourly per-patch regen split, the patch-1001 founders' full rows, and
//     the patch-1001 alive count — must be hour-identical to the f=16
//     twin's: inside one world, patch 1001 is a closed economy, so the only
//     changed variable is patch 1002's mouth count. The patch-1002 columns
//     must differ somewhere, or the comparison would be vacuous.
func TestNaturalFoundersSeamGate3Patch1001Identity(t *testing.T) {
	twin16, err := NewNatural(naturalFoundersOptions(0, true, 0))
	if err != nil {
		t.Fatal(err)
	}
	twin12, err := NewNatural(naturalFoundersOptions(0, true, 12))
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, twin16, 168)
	naturalRunTo(t, twin12, 168)
	if got := naturalFoundersGate3WorldColumns(t, twin16); got != naturalFoundersGate3Anchor {
		t.Fatalf("live f=16 twin diverges from the recorded frozen A cell: %s", got)
	}
	// Belt: when the recorded evidence file ships in the tree, digest it the
	// same way and re-check the pin against the recording itself.
	if raw, err := os.ReadFile("../../.tmp/natural7-n7-A-r8-s0-w1.json"); err == nil {
		var wire struct {
			Branches []struct {
				RegenSplit struct {
					CultivatedByHour []int64 `json:"cultivated_by_hour"`
					WildByHour       []int64 `json:"wild_by_hour"`
					UnrealizedByHour []int64 `json:"unrealized_by_hour"`
				} `json:"regen_split"`
				Population struct {
					AliveByHour []int64 `json:"alive_by_hour"`
				} `json:"population"`
				Actors []struct {
					ID                 int64 `json:"id"`
					HomePatch          int64 `json:"home_patch"`
					Energy             int64 `json:"energy"`
					Hunger             int64 `json:"hunger"`
					BasalSpent         int64 `json:"basal_spent"`
					CapLost            int64 `json:"energy_cap_lost"`
					LastGatherHour     int64 `json:"last_gather_hour"`
					BagUnits           int64 `json:"bag_units"`
					Capital            int64 `json:"capital"`
					Wip                int64 `json:"wip"`
					WearDebt           int64 `json:"wear_debt"`
					InvestedUnits      int64 `json:"invested_units"`
					PointsCreated      int64 `json:"points_created"`
					PointsDecayed      int64 `json:"points_decayed"`
					LastBuildHour      int64 `json:"last_build_hour"`
					GranaryStock       int64 `json:"granary_stock"`
					CultivatedTotal    int64 `json:"cultivated_total"`
					StoredMeals        int64 `json:"stored_meals"`
					LastStoredMealHour int64 `json:"last_stored_meal_hour"`
					SpoilDebt          int64 `json:"spoil_debt"`
					Spoiled            int64 `json:"spoiled"`
					LocusM             int64 `json:"locus_m"`
					LocusY             int64 `json:"locus_y"`
					LocusW             int64 `json:"locus_w"`
					BirthHour          int64 `json:"birth_hour"`
					DeathHour          int64 `json:"death_hour"`
					DiedHour           int64 `json:"died_hour"`
					BirthGranaryPaid   int64 `json:"birth_granary_paid"`
					Provisioned        int64 `json:"provisioned"`
				} `json:"actors"`
			} `json:"branches"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		br := wire.Branches[0]
		var actors []naturalFoundersGate3Actor
		for _, a := range br.Actors {
			if a.HomePatch == 1001 {
				actors = append(actors, naturalFoundersGate3Actor{
					ID: a.ID, Home: a.HomePatch, Energy: a.Energy, Hunger: a.Hunger, BasalSpent: a.BasalSpent,
					CapLost: a.CapLost, LastGather: a.LastGatherHour, Bag: a.BagUnits, Capital: a.Capital,
					Wip: a.Wip, WearDebt: a.WearDebt, Invested: a.InvestedUnits,
					PointsCreated: a.PointsCreated, PointsDecayed: a.PointsDecayed, LastBuild: a.LastBuildHour,
					Stock: a.GranaryStock, CultivatedTotal: a.CultivatedTotal, StoredMeals: a.StoredMeals,
					LastStoredMeal: a.LastStoredMealHour, SpoilDebt: a.SpoilDebt, Spoiled: a.Spoiled,
					LocusM: a.LocusM, LocusY: a.LocusY, LocusW: a.LocusW, BirthHour: a.BirthHour,
					DeathHour: a.DeathHour, DiedHour: a.DiedHour, BirthGranaryPaid: a.BirthGranaryPaid,
					Provisioned: a.Provisioned,
				})
			}
		}
		if got := naturalFoundersGate3Digest(br.RegenSplit.CultivatedByHour, br.RegenSplit.WildByHour,
			br.RegenSplit.UnrealizedByHour, br.Population.AliveByHour, actors); got != naturalFoundersGate3Anchor {
			t.Fatalf("recorded evidence digest %s diverges from the anchor pin", got)
		}
	}
	checks16, checks12 := twin16.Checkpoints(), twin12.Checkpoints()
	if len(checks12) != len(checks16) {
		t.Fatalf("checkpoint counts diverge: %d vs %d", len(checks12), len(checks16))
	}
	for hour := range checks16 {
		a, b := checks16[hour], checks12[hour]
		// Patch-1001 slice of the hourly per-patch regen split.
		for _, field := range []struct{ got, want int64 }{
			{b.Patches[0].Granted - prevOf(checks12, hour, 0, "granted"), a.Patches[0].Granted - prevOf(checks16, hour, 0, "granted")},
			{b.Patches[0].Produced - prevOf(checks12, hour, 0, "produced"), a.Patches[0].Produced - prevOf(checks16, hour, 0, "produced")},
			{b.Patches[0].Unrealized - prevOf(checks12, hour, 0, "unrealized"), a.Patches[0].Unrealized - prevOf(checks16, hour, 0, "unrealized")},
		} {
			if field.got != field.want {
				t.Fatalf("h%d: patch-1001 regen split diverged (%+v)", hour, field)
			}
		}
		// Patch-1001 actor ledgers and alive-by-hour.
		if !reflect.DeepEqual(b.Founders[:8], a.Founders[:8]) {
			t.Fatalf("h%d: patch-1001 founder rows diverged from the frozen cell", hour)
		}
		for i := 0; i < 8; i++ {
			if naturalAlive(b.Founders[i]) != naturalAlive(a.Founders[i]) {
				t.Fatalf("h%d: patch-1001 alive-by-hour diverged at founder %d", hour, i+1)
			}
		}
	}
	// The treatment must actually bite on the under-capacity patch.
	bites := false
	for hour := range checks16 {
		if !reflect.DeepEqual(checks12[hour].Founders[8:12], checks16[hour].Founders[8:12]) {
			bites = true
			break
		}
	}
	if !bites {
		t.Fatal("patch-1002 founder rows never diverged; the f=12 treatment did not bite and the gate proves nothing")
	}
}

func prevOf(checks []NaturalCheckpoint, hour, patch int, field string) int64 {
	if hour == 0 {
		return 0
	}
	switch field {
	case "granted":
		return checks[hour-1].Patches[patch].Granted
	case "produced":
		return checks[hour-1].Patches[patch].Produced
	default:
		return checks[hour-1].Patches[patch].Unrealized
	}
}
