package world

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
)

// Tests for the declared founder locus-m fixture (the frozen genome-variant
// matrix class of ticket 14). The option is additive: nil/empty keeps the
// frozen all-neutral derivation byte-identically, and the declared set is
// part of run identity — checkpoint manifests and journal verification pin
// it, so a neutral root under a variant option and a variant root under the
// default option are both rejected with nothing published.

// reproductionVariantLocusM is the recorded fixture: actors 1-8 carry
// locus-m allele 7, actors 9-16 allele 5; loci y/w stay neutral.
func reproductionVariantLocusM() []int64 {
	out := make([]int64, ReproductionFounderCount)
	for i := range out {
		if i < 8 {
			out[i] = 7
		} else {
			out[i] = 5
		}
	}
	return out
}

// TestReproductionNeutralRootDigestPinned pins the sha256 of five neutral h0
// root bundles recorded from the pre-option revision (a592b44). Any layout,
// codec, seed-derivation or field-semantics drift that changes a nil-option
// bundle fails here, so the additive option cannot silently fork the
// frozen scenario.
func TestReproductionNeutralRootDigestPinned(t *testing.T) {
	hex := func(d [32]byte) string { return fmt.Sprintf("%x", d[:]) }
	pinned := []struct {
		q      int64
		seed   uint64
		births bool
		digest string
	}{
		{8, 0, false, "7578fa68e3723e91cef28510249ece8bf75910e3d8a85cd47ef00f28d9eeb462"},
		{8, 0, true, "f93402ed1d4007aa782ac00c406e38a14d85b549e353b4e2aaacdcd333b3c17d"},
		{0, 0, false, "1109a246b39f5e2e235b6f2e35e71004f2a4b39435ad624503fcea295178ee31"},
		{5, 7, false, "d4e0df3e6d68c6785aae2d60b0ec53d59a305553627dde411c00d8f31e8d6d34"},
		{3, 7, true, "edc44718d90c995697294c304a5fb7045bcad628648293853296b83f40e02f53"},
	}
	for _, p := range pinned {
		f, err := NewReproduction(ReproductionOptions{Yield: p.q, Seed: p.seed, Workers: 1, Births: p.births})
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := f.Step(context.Background()); err != nil || !ok {
			t.Fatalf("q%d s%d births=%t: processed=%v err=%v", p.q, p.seed, p.births, ok, err)
		}
		digest, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "root.bundle"))
		if err != nil {
			t.Fatal(err)
		}
		if got := hex(digest); got != p.digest {
			t.Fatalf("q%d s%d births=%t neutral root digest drifted: got %s want %s", p.q, p.seed, p.births, got, p.digest)
		}
	}
}

// TestReproductionFounderVariantWorldConstruction walks the recorded fixture
// world: the locus-m rows carry the declared alleles, every other founder
// column stays on the frozen derivation, the declared rows equal the
// recorded journal identity, and the h0 pulse stays inside the frozen
// extended conservation with the alive invariant intact.
func TestReproductionFounderVariantWorldConstruction(t *testing.T) {
	f, err := NewReproduction(ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true, FounderLocusM: reproductionVariantLocusM()})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatalf("h0 pulse: processed=%v err=%v", ok, err)
	}
	frozen, err := expectedReproductionFounderGenomes(0)
	if err != nil {
		t.Fatal(err)
	}
	check := f.Checkpoints()[0]
	for i := range check.Founders {
		g := check.Founders[i].Genome
		wantM := int64(7)
		if i >= 8 {
			wantM = 5
		}
		if g.LocusM != wantM || g.LocusY != ReproductionGenomeNeutral || g.LocusW != ReproductionGenomeNeutral {
			t.Fatalf("founder %d locus drift: got (m=%d y=%d w=%d) want m=%d y=6 w=6", i+1, g.LocusM, g.LocusY, g.LocusW, wantM)
		}
		if g.DeathHour != frozen[i].Genome.DeathHour || g.BirthHour != 0 || g.ParentA != 0 || g.ParentB != 0 {
			t.Fatalf("founder %d left the frozen derivation outside locus m: %+v vs %+v", i+1, g, frozen[i].Genome)
		}
	}
	if !equalReproductionFounderGenomes(f.declaredFounders, f.journalConfig().Founders) {
		t.Fatal("declared founder rows differ from the recorded journal identity")
	}
	reproductionAssertConservation(t, f)
	reproductionAssertJournalClock(t, f)
}

// TestReproductionFounderVariantAllNeutralOverrideEqualsNil pins that an
// all-neutral declared override constructs byte-identically to the frozen
// derivation: same root digest, same declared rows.
func TestReproductionFounderVariantAllNeutralOverrideEqualsNil(t *testing.T) {
	build := func(loci []int64) [32]byte {
		t.Helper()
		f, err := NewReproduction(ReproductionOptions{Yield: 5, Seed: 7, Workers: 1, Births: true, FounderLocusM: loci})
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := f.Step(context.Background()); err != nil || !ok {
			t.Fatalf("h0 pulse: processed=%v err=%v", ok, err)
		}
		digest, err := f.SaveCheckpoint(filepath.Join(t.TempDir(), "root.bundle"))
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	neutral := make([]int64, ReproductionFounderCount)
	for i := range neutral {
		neutral[i] = ReproductionGenomeNeutral
	}
	if build(nil) != build(neutral) {
		t.Fatal("all-neutral override diverges from the frozen derivation")
	}
}

// TestReproductionFounderVariantSaveRestoreRoundtrip publishes the variant
// h0 root and restores it under the declared options at a different worker
// count; the neutral declaration and every malformed option are rejected.
func TestReproductionFounderVariantSaveRestoreRoundtrip(t *testing.T) {
	expected := ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true, FounderLocusM: reproductionVariantLocusM()}
	f, err := NewReproduction(expected)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatalf("h0 pulse: processed=%v err=%v", ok, err)
	}
	path := filepath.Join(t.TempDir(), "variant-root.bundle")
	digest, err := f.SaveCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, got, err := RestoreReproductionCheckpoint(path, ReproductionOptions{Yield: 8, Seed: 0, Workers: 4, Births: true, FounderLocusM: reproductionVariantLocusM()})
	if err != nil {
		t.Fatal(err)
	}
	if got != digest {
		t.Fatal("variant root digest drift on restore")
	}
	if len(restored.Checkpoints()) != 1 {
		t.Fatal("variant restore lost the h0 projection")
	}
	neutral := ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}
	if _, _, err := RestoreReproductionCheckpoint(path, neutral); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("variant root restored under the neutral declaration: err=%v", err)
	}
	for _, bad := range [][]int64{make([]int64, 15), append(reproductionVariantLocusM(), 5), variantOutOfBoundsAlleles(t)} {
		_, _, err := RestoreReproductionCheckpoint(path, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true, FounderLocusM: bad})
		if !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("malformed option accepted: %v err=%v", bad, err)
		}
	}
	if _, err := NewReproduction(ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true, FounderLocusM: []int64{4}}); !errors.Is(err, ErrReproductionRunner) {
		t.Fatalf("malformed override accepted at construction: err=%v", err)
	}
}

// variantOutOfBoundsAlleles builds an out-of-bounds allele list for the
// rejection fixture.
func variantOutOfBoundsAlleles(t *testing.T) []int64 {
	t.Helper()
	out := reproductionVariantLocusM()
	out[3] = 8
	return out
}

// TestReproductionFounderVariantBranchAndJournalIdentity walks the recorded
// enable intervention from a variant root: the child shares the parent
// digest lineage and restores only under the declared options, a failed
// declaration publishes nothing, and the journal verifies under the declared
// fixture and fails under the neutral one (and the neutral root's journal
// fails under the variant declaration).
func TestReproductionFounderVariantBranchAndJournalIdentity(t *testing.T) {
	expected := ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: false, FounderLocusM: reproductionVariantLocusM()}
	root, err := NewReproduction(expected)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := root.Step(context.Background()); err != nil || !ok {
		t.Fatalf("h0 pulse: processed=%v err=%v", ok, err)
	}
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "variant-root.bundle")
	rootDigest, err := root.SaveCheckpoint(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	// Run identity: the variant root must not collide with the pinned
	// all-neutral digest of the same (q, seed, births) scenario.
	if fmt.Sprintf("%x", rootDigest[:]) == "7578fa68e3723e91cef28510249ece8bf75910e3d8a85cd47ef00f28d9eeb462" {
		t.Fatal("variant root digest equals the pinned neutral digest")
	}
	childPath := filepath.Join(dir, "variant-enabled.bundle")
	childDigest, err := BranchReproductionCheckpoint(rootPath, expected, true, childPath)
	if err != nil {
		t.Fatal(err)
	}
	child, got, err := RestoreReproductionCheckpoint(childPath, ReproductionOptions{Yield: 8, Seed: 0, Workers: 4, Births: true, FounderLocusM: reproductionVariantLocusM()})
	if err != nil {
		t.Fatal(err)
	}
	if got != childDigest {
		t.Fatal("variant branch digest drift on restore")
	}
	if len(child.Checkpoints()) != 1 {
		t.Fatal("variant branch lost the h0 projection")
	}
	// A failed declaration must publish nothing.
	refusedPath := filepath.Join(dir, "never.bundle")
	if _, err := BranchReproductionCheckpoint(rootPath, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: false}, true, refusedPath); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("variant root branched under the neutral declaration: err=%v", err)
	}
	if _, err := os.Stat(refusedPath); !os.IsNotExist(err) {
		t.Fatalf("refused variant branch published a file: %v", err)
	}
	if _, _, err := RestoreReproductionCheckpoint(childPath, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatalf("variant child restored under the neutral declaration: err=%v", err)
	}
	// Journal identity, both directions.
	data, err := root.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, head, err := root.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	config := root.journalConfig()
	if _, err := decodeReproductionJournal(data, root.k, head, root.sched.Time(), config, root.declaredFounders); err != nil {
		t.Fatalf("variant journal refused under the declared fixture: %v", err)
	}
	if _, err := decodeReproductionJournal(data, root.k, head, root.sched.Time(), config, mustReproductionFounderGenomes(0)); !errors.Is(err, ErrReproductionJournal) {
		t.Fatalf("variant journal accepted under the neutral declaration: err=%v", err)
	}
	neutral := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1}, 0)
	neutralData, err := neutral.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	_, neutralHead, err := neutral.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	neutralConfig := neutral.journalConfig()
	if _, err := decodeReproductionJournal(neutralData, neutral.k, neutralHead, neutral.sched.Time(), neutralConfig, mustReproductionFounderGenomes(0)); err != nil {
		t.Fatalf("neutral journal refused under its own derivation: %v", err)
	}
	if _, err := decodeReproductionJournal(neutralData, neutral.k, neutralHead, neutral.sched.Time(), neutralConfig, reproductionVariantLocusMRows()); !errors.Is(err, ErrReproductionJournal) {
		t.Fatalf("neutral journal accepted under the variant declaration: err=%v", err)
	}
}

// reproductionVariantLocusMRows derives the declared variant rows for the
// neutral-direction journal rejection fixture.
func reproductionVariantLocusMRows() []ReproductionFounderGenomeRow {
	rows, err := reproductionDeclaredFounderGenomes(ReproductionOptions{Seed: 0, FounderLocusM: reproductionVariantLocusM()})
	if err != nil {
		panic(err)
	}
	return rows
}

// TestReproductionFounderVariantBirthsWalkConservation runs the recorded
// fixture with births enabled to h12 and re-verifies the frozen gates and
// journal clock on every hourly checkpoint; heredity stays inside the frozen
// allele bounds for every living actor.
func TestReproductionFounderVariantBirthsWalkConservation(t *testing.T) {
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true, FounderLocusM: reproductionVariantLocusM()}, 12)
	if f.Checkpoints()[12].Metrics.Births == 0 {
		t.Fatal("variant q8 births-enabled fixture produced no births by h12")
	}
	reproductionAssertConservation(t, f)
	reproductionAssertJournalClock(t, f)
	final := f.Checkpoints()[12]
	assertLocus := func(id sim.EntityID, g ReproductionGenomeState) {
		if g.LocusM < ReproductionGenomeLocusMin || g.LocusM > ReproductionGenomeLocusMax ||
			g.LocusY < ReproductionGenomeLocusMin || g.LocusY > ReproductionGenomeLocusMax ||
			g.LocusW < ReproductionGenomeLocusMin || g.LocusW > ReproductionGenomeLocusMax {
			t.Fatalf("actor %d heredity left the frozen bounds: %+v", id, g)
		}
	}
	for i := range final.Founders {
		assertLocus(sim.EntityID(i+1), final.Founders[i].Genome)
	}
	for _, nb := range final.Newborns {
		assertLocus(nb.ID, nb.State.Genome)
	}
}

// TestReproductionSecondGenerationBirthRegisters pins the registered-newborn
// parent case the experiment matrix surfaced: when a first-generation
// newborn becomes a parent, rule 410's event carries the newborn's genome
// allocation plus BOTH parents' birth-granary-paid genome patches, and the
// parents themselves match the newborn-ID filter. The allocation must
// register exactly one newborn (births 14→15), the roster grows by one, and
// the frozen gates, journal clock and wake topology stay green.
func TestReproductionSecondGenerationBirthRegisters(t *testing.T) {
	// The second-generation birth commits at h26's reply instant, so the
	// h26 pulse still counts 14 births and the h27 pulse counts 15.
	f := reproductionRunTo(t, ReproductionOptions{Yield: 5, Seed: 0, Workers: 1, Births: true, FounderLocusM: reproductionVariantLocusM()}, 27)
	checks := f.Checkpoints()
	b26, b27 := checks[26].Metrics.Births, checks[27].Metrics.Births
	if b26 != 14 || b27 != 15 {
		t.Fatalf("second-generation birth counter drift: h26 births %d, h27 births %d", b26, b27)
	}
	var child *ReproductionNewbornState
	for i := range checks[27].Newborns {
		nb := &checks[27].Newborns[i]
		if nb.ID == ReproductionNewbornIDForIndex(14) {
			child = nb
		}
	}
	if child == nil {
		t.Fatal("the second-generation newborn is missing from the h27 projection")
	}
	if child.State.Genome.ParentA == 0 || child.State.Genome.ParentA < ReproductionFirstNewbornID {
		t.Fatalf("the h26 birth is not second-generation: parents (%d,%d)", child.State.Genome.ParentA, child.State.Genome.ParentB)
	}
	if len(checks[27].Newborns) != 15 {
		t.Fatalf("roster growth drift: %d newborns at h27", len(checks[27].Newborns))
	}
	reproductionAssertConservation(t, f)
	reproductionAssertJournalClock(t, f)
}

// ReproductionNewbornIDForIndex names the newborn ID for a zero-based birth
// ordinal (the frozen 10000+births+1 allocation).
func ReproductionNewbornIDForIndex(births int64) sim.EntityID {
	id, err := ReproductionNewbornID(births)
	if err != nil {
		panic(err)
	}
	return id
}

// TestReproductionRegisterNewbornRejectsForgedTwoUnregistered pins the
// fail-closed direction of the same scan: an event carrying TWO distinct
// unregistered newborn genome rows is refused and registers nothing.
func TestReproductionRegisterNewbornRejectsForgedTwoUnregistered(t *testing.T) {
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1}, 0)
	before := len(f.roster)
	replyAt, _ := ReproductionPhaseTime(0, ReproductionPhaseBirthReply)
	forged := kernel.Event{
		ID:   1,
		Kind: kernel.KindEntityCreate,
		Time: replyAt,
		Deltas: []component.FieldDelta{
			{Entity: ReproductionNewbornIDForIndex(0), Component: ReproductionGenomeTypeID, Field: ReproductionGenomeLocusMField},
			{Entity: ReproductionNewbornIDForIndex(1), Component: ReproductionGenomeTypeID, Field: ReproductionGenomeLocusMField},
		},
	}
	if err := f.registerNewborn(forged, 0); !errors.Is(err, ErrReproductionRunner) {
		t.Fatalf("forged two-newborn event accepted: err=%v", err)
	}
	if len(f.roster) != before {
		t.Fatal("a refused event mutated the roster")
	}
}
