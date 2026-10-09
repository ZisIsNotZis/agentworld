package world

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"agentworld/internal/checkpoint"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The mechanism-axis seam tests (ticket 17 L3, world lane). The frozen v7
// design (.scratch/17-surplus-mechanism/spec.md) is binding: the mechanism
// axis (NaturalOptions.Mechanism / NaturalRestoreOptions.Mechanism, zero
// value ≡ v5) is run identity carried in the manifest's mechanism block and
// the journal config with bidirectional {5,6} refusal; the ref/mechanism
// consistency rule admits natural/5|6 under either mechanism and requires
// mechanism v6 for natural/7|7b; rule 515 "provision" commits at the frozen
// claim instant and hands the converted unit to the normal bag decision; and
// the v5 zero value publishes byte-identical bundles and journals to the
// pre-L3 behaviour (digest pins captured at 0cca026, the post-L1-review
// commit that already contained the L2 delivery).

// Pre-L3 (0cca026) golden digests of the neutral v5 world, captured before
// any L3 edit: the h0 root bundle and the hour-5 resume bundle of
// (P=2, R=8, seed 0, births off, one worker), the exported journal sha256 at
// hours 5 and 9, the births-enabled root (P=2, R=8, seed 3, two workers),
// the founder-tenure fixture's hour-6 resume (seed 3, births, two workers)
// and its exported journal sha256. The zero value must reproduce every pin.
const (
	naturalMechanismGoldenRoot           = "97a14338dba9b92c68204571472a1c6ef168be35cea52cbb673d16ecfe8faa94"
	naturalMechanismGoldenResume5        = "875c51c4884ba59331896c8225d694332ea3ee1c939d463a6704cfce5711f343"
	naturalMechanismGoldenJournal5       = "8c17b4239ac5e7ab3577862c594ad4ded3507fcc33093d424957994301983400"
	naturalMechanismGoldenJournal9       = "49d473f8aa5479df41fe0e2f628e29c2cb26e41b37fb22748d553a55439c0ac0"
	naturalMechanismGoldenBirthsRoot     = "e84edd58d2fa71fcdf8d24601631f4820763b2bf8ae3cd5a3fb13f99fc5e523c"
	naturalMechanismGoldenFixtureResume6 = "5ce0732d82b3a8088202a66789dfe3187db587b03902f1072f4d5f39d1a9bced"
	naturalMechanismGoldenFixtureJournal = "723f0ea5b1f28af63e135c77d12884ec3a92ba7c52f739eae965b2b9d86eda40"
)

func naturalMechanismSeamDigest(t *testing.T, digest [32]byte) string {
	t.Helper()
	return hex.EncodeToString(digest[:])
}

func naturalMechanismSeamJournalDigest(t *testing.T, f *Natural) string {
	t.Helper()
	data, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func naturalMechanismSeamV7Options(seed uint64, births bool) NaturalOptions {
	return NaturalOptions{Patches: 2, R: 8, Seed: seed, Workers: 2, Births: births,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6}
}

// TestNaturalMechanismSeamZeroValueDigestIdentity proves the L3
// default-preservation: a world constructed without the new option (and its
// explicit NaturalMechanismV5 spelling) publishes byte-identical bundles and
// journals to the pre-L3 behaviour at 0cca026 — root capture, pulse-boundary
// resume, births-enabled root, founder-tenure fixture resume, and the
// exported journals — and the zero-value manifest keeps the frozen
// mechanism-block constants.
func TestNaturalMechanismSeamZeroValueDigestIdentity(t *testing.T) {
	dir := t.TempDir()
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	rootDigest, err := f.SaveCheckpoint(dir + "/root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if got := naturalMechanismSeamDigest(t, rootDigest); got != naturalMechanismGoldenRoot {
		t.Fatalf("zero-value root digest %s diverges from the pre-L3 pin", got)
	}
	naturalRunTo(t, f, 5)
	resumeDigest, err := f.SaveResumeCheckpoint(dir + "/resume.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if got := naturalMechanismSeamDigest(t, resumeDigest); got != naturalMechanismGoldenResume5 {
		t.Fatalf("zero-value resume digest %s diverges from the pre-L3 pin", got)
	}
	if got := naturalMechanismSeamJournalDigest(t, f); got != naturalMechanismGoldenJournal5 {
		t.Fatalf("zero-value journal digest %s diverges from the pre-L3 pin", got)
	}
	naturalRunTo(t, f, 9)
	if got := naturalMechanismSeamJournalDigest(t, f); got != naturalMechanismGoldenJournal9 {
		t.Fatalf("zero-value journal digest %s diverges from the pre-L3 pin", got)
	}
	fb, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 3, Workers: 2, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := fb.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	birthsRoot, err := fb.SaveCheckpoint(dir + "/births-root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if got := naturalMechanismSeamDigest(t, birthsRoot); got != naturalMechanismGoldenBirthsRoot {
		t.Fatalf("zero-value births root digest %s diverges from the pre-L3 pin", got)
	}
	fx, err := newNaturalFixtureRunner(2, 8, 3, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, fx, 6)
	fxResume, err := fx.SaveResumeCheckpoint(dir + "/fixture-resume.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if got := naturalMechanismSeamDigest(t, fxResume); got != naturalMechanismGoldenFixtureResume6 {
		t.Fatalf("zero-value fixture resume digest %s diverges from the pre-L3 pin", got)
	}
	if got := naturalMechanismSeamJournalDigest(t, fx); got != naturalMechanismGoldenFixtureJournal {
		t.Fatalf("zero-value fixture journal digest %s diverges from the pre-L3 pin", got)
	}
	// The explicit v5 spelling is the same world, not a second one: the two
	// constructors bind the same ref and mechanism and publish identical
	// bytes.
	explicit, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Mechanism: NaturalMechanismV5})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.ref != f.ref || explicit.mechanism != f.mechanism || f.mechanism != NaturalMechanismV5 {
		t.Fatalf("zero value is not exactly the v5 mechanism: %+v / %d", explicit.ref, explicit.mechanism)
	}
	if ok, err := explicit.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	explicitRoot, err := explicit.SaveCheckpoint(dir + "/explicit-root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if explicitRoot != rootDigest {
		t.Fatal("explicit-v5 and zero-value roots diverge")
	}
	// The zero-value manifest keeps the frozen mechanism block: the version
	// slots read back exactly as before L3.
	m := naturalMechanismSeamManifest(t, dir+"/root.bundle")
	if m.mechanism != NaturalMechanismV5 {
		t.Fatalf("zero-value manifest carries mechanism %d", m.mechanism)
	}
}

func naturalMechanismSeamManifest(t *testing.T, path string) naturalManifest {
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

// TestNaturalMechanismSeamV6RoundTripCarriesMechanism proves the mechanism
// axis is run identity end to end under mechanism v6: the manifest's
// mechanism block reads back v6 (all four version slots), the journal config
// carries the mechanism, a root and a pulse-boundary resume save/restore
// (and branch) carry it, and the restored runner continues deterministically
// with the original.
func TestNaturalMechanismSeamV6RoundTripCarriesMechanism(t *testing.T) {
	f6, err := NewNatural(naturalMechanismSeamV7Options(0, false))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f6.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	dir := t.TempDir()
	root, err := f6.SaveCheckpoint(dir + "/v6-root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	_ = root
	m := naturalMechanismSeamManifest(t, dir+"/v6-root.bundle")
	if m.mechanism != NaturalMechanismV6 || m.policyFormat != strategy.NaturalPolicyFormatV7 {
		t.Fatalf("v6 manifest carries mechanism %d policy format %d", m.mechanism, m.policyFormat)
	}
	// The manifest's mechanism block is the four version slots, all carrying
	// the v6 world version, while every other pinned constant keeps its
	// frozen value.
	sections, _, err := checkpoint.ReadFile(dir + "/v6-root.bundle")
	if err != nil {
		t.Fatal(err)
	}
	u32s, _ := naturalManifestConstants()
	for slot, want := range u32s {
		got := binaryU32At(t, sections[2].Data, slot)
		frozen := want
		switch slot {
		case naturalManifestPolicyFormatSlot:
			frozen = uint32(strategy.NaturalPolicyFormatV7) // the bound natural/7 policy slot
		default:
			for _, mechanismSlot := range naturalManifestMechanismSlots {
				if slot == mechanismSlot {
					frozen = 6
				}
			}
		}
		if got != frozen {
			t.Fatalf("manifest u32 slot %d carries %d, want %d", slot, got, frozen)
		}
	}
	if exported, err := f6.ExportJournal(); err != nil {
		t.Fatal(err)
	} else if !sameNaturalJournalWireMechanism(t, exported, NaturalMechanismV6) {
		t.Fatal("v6 journal config lost the mechanism")
	}
	naturalRunTo(t, f6, 4)
	if _, err := f6.SaveResumeCheckpoint(dir + "/v6-resume.bundle"); err != nil {
		t.Fatal(err)
	}
	expectation := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 3, Births: false,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6}
	restored, digestRestored, err := RestoreNaturalCheckpoint(dir+"/v6-resume.bundle", expectation)
	if err != nil {
		t.Fatalf("v6 resume restore: %v", err)
	}
	if restored.ref != strategy.FrozenNaturalPolicy7().Ref || restored.mechanism != NaturalMechanismV6 {
		t.Fatalf("restored runner carries ref %+v mechanism %d", restored.ref, restored.mechanism)
	}
	if !reflect.DeepEqual(restored.Journal(), f6.Journal()) || !reflect.DeepEqual(restored.Checkpoints(), f6.Checkpoints()) {
		t.Fatal("v6 restore changed the journal or the projection")
	}
	// The restored runner's journal re-exports byte-identically (canonicality
	// under the carried mechanism).
	sections, _, err = checkpoint.ReadFile(dir + "/v6-resume.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if exported, err := restored.ExportJournal(); err != nil || !sameNaturalBytes(exported, sections[0].Data) {
		t.Fatalf("restored v6 journal not carried: %v", err)
	}
	// A branch child carries the parent's mechanism and restores under the
	// declared v6 expectation (publish-nothing on refusal is covered by the
	// mismatch test below).
	child := dir + "/v6-branch.bundle"
	childDigest, err := BranchNaturalCheckpoint(dir+"/v6-root.bundle", expectation, true, child)
	if err != nil {
		t.Fatalf("v6 branch: %v", err)
	}
	if childDigest == root {
		t.Fatal("branch child digest equals the root")
	}
	branch, _, err := RestoreNaturalCheckpoint(child, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatalf("v6 branch restore: %v", err)
	}
	if branch.mechanism != NaturalMechanismV6 || branch.ref != strategy.FrozenNaturalPolicy7().Ref {
		t.Fatalf("branch child carries mechanism %d ref %+v", branch.mechanism, branch.ref)
	}
	// The restored runner continues in lockstep with the original.
	naturalRunTo(t, restored, 8)
	naturalRunTo(t, f6, 8)
	if !reflect.DeepEqual(restored.Journal(), f6.Journal()) {
		t.Fatal("restored v6 runner diverged from the original")
	}
	_ = digestRestored
	// The branch child continues deterministically: a second restore of the
	// same child at a different worker count reproduces its journals and
	// projections exactly (the child's first-hour prefix was produced under
	// the parent's births-disabled boundary, the recorded two-flag lineage).
	twin, _, err := RestoreNaturalCheckpoint(child, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 4, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatalf("v6 branch twin restore: %v", err)
	}
	naturalRunTo(t, branch, 7)
	naturalRunTo(t, twin, 7)
	if !reflect.DeepEqual(branch.Journal(), twin.Journal()) || !reflect.DeepEqual(branch.Checkpoints(), twin.Checkpoints()) {
		t.Fatal("v6 branch child restore is not worker-count deterministic")
	}
}

func binaryU32At(t *testing.T, manifest []byte, slot int) uint32 {
	t.Helper()
	offset := 4 + 4*slot
	if offset+4 > len(manifest) {
		t.Fatalf("manifest too short for slot %d", slot)
	}
	return uint32(manifest[offset])<<24 | uint32(manifest[offset+1])<<16 | uint32(manifest[offset+2])<<8 | uint32(manifest[offset+3])
}

func sameNaturalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameNaturalJournalWireMechanism decodes the journal wire and reports
// whether the config carries exactly the declared mechanism.
func sameNaturalJournalWireMechanism(t *testing.T, data []byte, want NaturalMechanism) bool {
	t.Helper()
	var wire naturalJournalWire
	if err := json.Unmarshal(data[12:len(data)-sha256.Size], &wire); err != nil {
		t.Fatalf("journal wire: %v", err)
	}
	return wire.Config.Mechanism == want
}

// TestNaturalMechanismSeamBidirectionalMismatch proves the bidirectional
// {5,6} refusal: a v5 bundle refuses a v6 expectation and a v6 bundle
// refuses a v5 expectation (typed error, before any restore work), a
// mismatched Branch publishes nothing, and a journal from the other
// mechanism never restores.
func TestNaturalMechanismSeamBidirectionalMismatch(t *testing.T) {
	dir := t.TempDir()
	f5, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 7, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f5.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := f5.SaveCheckpoint(dir + "/v5-root.bundle"); err != nil {
		t.Fatal(err)
	}
	path5 := dir + "/v5-root.bundle"
	f6, err := NewNatural(naturalMechanismSeamV7Options(7, false))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f6.Step(context.Background()); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := f6.SaveCheckpoint(dir + "/v6-root.bundle"); err != nil {
		t.Fatal(err)
	}
	path6 := dir + "/v6-root.bundle"
	expectV5 := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 7, Workers: 1}
	expectV6 := NaturalRestoreOptions{Patches: 2, R: 8, Seed: 7, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6}
	if _, _, err := RestoreNaturalCheckpoint(path5, expectV6); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v5 bundle accepted under a v6 expectation: %v", err)
	}
	if _, _, err := RestoreNaturalCheckpoint(path6, expectV5); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("v6 bundle accepted under a v5 expectation: %v", err)
	}
	// A v6 bundle under a v6 expectation with the wrong seed is still a
	// mismatch (the mechanism check adds to, never replaces, the scenario
	// match).
	if _, _, err := RestoreNaturalCheckpoint(path6, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 8, Workers: 1,
		PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6}); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("v6 bundle accepted under a different seed")
	}
	// Branch refuses before publication both ways; the destinations must not
	// exist after the refusals.
	dest1 := dir + "/never-1.bundle"
	if _, err := BranchNaturalCheckpoint(path5, expectV6, true, dest1); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branch mismatched v5 root: %v", err)
	}
	dest2 := dir + "/never-2.bundle"
	if _, err := BranchNaturalCheckpoint(path6, expectV5, true, dest2); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("branch mismatched v6 root: %v", err)
	}
	for _, dest := range []string{dest1, dest2} {
		if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("mismatched branch published %s: %v", dest, err)
		}
	}
	// A mixed-tuple manifest (one mechanism block slot moved to a foreign
	// version) never decodes.
	m := naturalMechanismSeamManifest(t, path5)
	sections, _, err := checkpoint.ReadFile(path5)
	if err != nil {
		t.Fatal(err)
	}
	forged := append([]byte(nil), sections[2].Data...)
	offset := 4 + 4*naturalManifestMechanismWorldFormatSlot
	forged[offset] = 9
	if _, err := decodeNaturalManifest(forged); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("foreign mechanism slot %d accepted (manifest declared %+v)", forged[offset], m.mechanism)
	}
	// Journal level: the other mechanism's journal never restores (same
	// genesis, same kernel head shape apart from the axis — the config
	// identity refuses).
	_, head5, err := f5.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	journal6, err := f6.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if err := f5.RestoreJournal(journal6, head5); !errors.Is(err, ErrNaturalJournal) {
		t.Fatalf("v5 runner accepted a mechanism-v6 journal: %v", err)
	}
}

// TestNaturalMechanismSeamRefConsistencyNegatives proves the ref/mechanism
// consistency rule fails closed at every construction seam: natural/7 and
// natural/7b are refused under the v5 mechanism (the zero value included)
// and admitted under v6; natural/9 and foreign mechanisms are refused under
// both; the restore options apply the same rule.
func TestNaturalMechanismSeamRefConsistencyNegatives(t *testing.T) {
	ref7 := strategy.FrozenNaturalPolicy7().Ref
	ref7b := strategy.FrozenNaturalPolicy7b().Ref
	for _, mechanism := range []NaturalMechanism{NaturalMechanismV5, 0} {
		mechanism := mechanism
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref7, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("natural/7 accepted under mechanism %d", mechanism)
		}
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref7b, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("natural/7b accepted under mechanism %d", mechanism)
		}
	}
	// Both provision policies construct under mechanism v6 and dispatch the
	// matching cascade.
	for _, ref := range []strategy.NaturalRef{ref7, ref7b} {
		f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref, Mechanism: NaturalMechanismV6})
		if err != nil {
			t.Fatalf("ref %+v under mechanism v6 refused: %v", ref, err)
		}
		if f.ref != ref || f.mechanism != NaturalMechanismV6 {
			t.Fatalf("runner bound %+v/%d for ref %+v", f.ref, f.mechanism, ref)
		}
	}
	// Unknown refs and unadmitted mechanisms fail under every axis.
	// Ticket 20 consequence (authorised sample move): natural/9 joined the frozen ref set; the unknown sample moves to 10.
	if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: strategy.NaturalRef{ID: "natural", Version: 10}, Mechanism: NaturalMechanismV6}); !errors.Is(err, ErrNaturalRunner) {
		t.Fatal("natural/10 accepted under mechanism v6")
	}
	// Ticket 20 consequence (authorised sample move, not a behaviour
	// change): mechanism 7 became an admitted axis value when the wage
	// contract landed, so the unadmitted sample moves to the next
	// undeclared value 9.
	for _, mechanism := range []NaturalMechanism{1, 2, 4, 5, 8, 9, 65535} {
		if _, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Mechanism: mechanism}); !errors.Is(err, ErrNaturalRunner) {
			t.Fatalf("mechanism %d accepted", mechanism)
		}
		if err := (NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Mechanism: mechanism}).validate(); !errors.Is(err, ErrNaturalCheckpoint) {
			t.Fatalf("restore mechanism %d validated", mechanism)
		}
	}
	// The restore options apply the same ref/mechanism rule.
	if err := (NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref7}).validate(); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatalf("natural/7 restore expectation validated under the v5 zero value: %v", err)
	}
	if err := (NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref7, Mechanism: NaturalMechanismV6}).validate(); err != nil {
		t.Fatalf("natural/7 restore expectation refused under mechanism v6: %v", err)
	}
	if err := (NaturalRestoreOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, PolicyRef: ref7b, Mechanism: NaturalMechanismV6}).validate(); err != nil {
		t.Fatalf("natural/7b restore expectation refused under mechanism v6: %v", err)
	}
	// And a journal config with a provision ref under the v5 zero value
	// never validates.
	config := NaturalJournalConfig{Patches: 2, R: 8, Seed: 0, Policy: ref7}
	if !validNaturalJournalConfig(NaturalJournalConfig{Patches: 2, R: 8, Seed: 0, Policy: strategy.FrozenNaturalPolicyV6().Ref}) {
		t.Fatal("natural/6 under the v5 zero value journal config refused")
	}
	if validNaturalJournalConfig(config) {
		t.Fatal("natural/7 journal config accepted under the v5 zero value")
	}
	config.Mechanism = NaturalMechanismV6
	if !validNaturalJournalConfig(config) {
		t.Fatal("natural/7 journal config refused under mechanism v6")
	}
}

// TestNaturalMechanismSeamProvisionEndToEnd is the v7 provision smoke run on
// world-v6 × natural/7: a provision event commits at the frozen claim
// instant (rule 515, the contract's four-patch proposal), the bag unit
// exists, a build follows via rule 505 in the same hour, the extended G1
// intake identity and the hourly conservation stay green, the journal clock
// holds, and the balance's provisioned counter equals the accepted attempt
// count and the projected ledger sum.
func TestNaturalMechanismSeamProvisionEndToEnd(t *testing.T) {
	f, err := NewNatural(naturalMechanismSeamV7Options(0, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The journal clock holds for the full horizon including provisions.
	naturalAssertJournalClock(t, f)
	naturalAssertConservationForMechanism(t, f, NaturalMechanismV6)

	var first kernel.Event
	firstAccepted := 0
	provisions, accepted := 0, 0
	for _, b := range f.Journal() {
		for _, a := range b.Attempts {
			if a.Kind != "provision" {
				continue
			}
			provisions++
			if !a.Accepted {
				continue
			}
			accepted++
			if firstAccepted == 0 {
				firstAccepted = int(a.Time)
				for _, ev := range f.k.Events() {
					if ev.Key == a.Key {
						first = ev
					}
				}
			}
		}
	}
	if accepted == 0 || first.Key == "" {
		t.Fatalf("no accepted provision event (attempts %d)", provisions)
	}
	// The committed provision sits at the frozen claim instant, is caused by
	// its actor under rule 515, and carries exactly the contract's four
	// patches (granary stock, provisioned, last-stored-meal-hour, bag).
	claim, err := NaturalPhaseTime(firstAccepted/3600000000, NaturalPhaseClaim)
	if err != nil {
		t.Fatal(err)
	}
	if first.Time != claim || first.Rule != NaturalProvisionRule || first.Cause.Actor == 0 || first.Cause.World {
		t.Fatalf("provision event off the claim instant or cause: %+v", first)
	}
	if len(first.Deltas) != 4 {
		t.Fatalf("provision event carries %d deltas, want the contract's 4", len(first.Deltas))
	}
	fields := map[sim.FieldID]bool{}
	for _, d := range first.Deltas {
		if d.Component == NaturalGranaryTypeID {
			fields[d.Field] = true
		}
		if d.Component == NaturalBagTypeID {
			if d.Field != NaturalBagUnitsField {
				t.Fatalf("provision patches a foreign bag field %d", d.Field)
			}
			after, err := d.After.Integer()
			if err != nil || after != 1 {
				t.Fatalf("the provisioned bag unit does not exist: %v %v", after, err)
			}
		}
		if d.SchemaVersion != NaturalMechanismV6.SchemaVersion() {
			t.Fatalf("provision delta carries schema version %d", d.SchemaVersion)
		}
	}
	for _, want := range []sim.FieldID{NaturalGranaryStockField, NaturalGranaryProvisionedField, NaturalGranaryLastStoredMealHourField} {
		if !fields[want] {
			t.Fatalf("provision granary patch set missing field %d", want)
		}
	}
	// A build follows via rule 505 in the same hour: the converted unit
	// takes the normal bag decision and at least one provisioned actor-hour
	// commits its build at the frozen completion instant of its own hour.
	buildsByActorAt := map[sim.EntityID]map[int]bool{}
	for _, ev := range f.k.Events() {
		if ev.Rule != NaturalBuildRule {
			continue
		}
		hour := int(ev.Time / 3600000000)
		start, err := NaturalPhaseTime(hour, NaturalPhaseBuildStart)
		if err != nil {
			t.Fatal(err)
		}
		if ev.Time != NaturalBuildCompletion(start) {
			continue
		}
		if buildsByActorAt[ev.Cause.Actor] == nil {
			buildsByActorAt[ev.Cause.Actor] = map[int]bool{}
		}
		buildsByActorAt[ev.Cause.Actor][hour] = true
	}
	hourOfFirst := firstAccepted / 3600000000
	if _, ok := buildsByActorAt[first.Cause.Actor]; !ok {
		t.Fatalf("the first provisioner %d never built via rule 505", first.Cause.Actor)
	}
	sameHourBuild := false
	for _, ev := range f.k.Events() {
		if ev.Rule != NaturalProvisionRule {
			continue
		}
		if buildsByActorAt[ev.Cause.Actor][int(ev.Time/3600000000)] {
			sameHourBuild = true
			break
		}
	}
	if !sameHourBuild {
		t.Fatal("no provisioned actor-hour committed a same-hour rule-505 build")
	}
	if !containsHour(buildsByActorAt[first.Cause.Actor], hourOfFirst) {
		t.Logf("note: the first provisioner built in a later hour, not h%d", hourOfFirst)
	}
	// The provisioned counters agree across the value-only seams: the
	// balance's aggregate equals the accepted attempt count and the projected
	// ledger sum over founders and newborns.
	final := f.Checkpoints()[len(f.Checkpoints())-1]
	if final.Balance.Provisioned != int64(accepted) {
		t.Fatalf("balance provisioned %d != accepted attempts %d", final.Balance.Provisioned, accepted)
	}
	sum := int64(0)
	for _, founder := range final.Founders {
		sum += founder.Granary.Provisioned
	}
	for _, newborn := range final.Newborns {
		sum += newborn.State.Granary.Provisioned
	}
	if sum != final.Balance.Provisioned {
		t.Fatalf("projected provisioned ledger %d != balance %d", sum, final.Balance.Provisioned)
	}
}

func containsHour(hours map[int]bool, hour int) bool { return hours[hour] }

// naturalAssertConservationForMechanism re-verifies the extended gates at
// every hourly checkpoint under one mechanism, including the extended G1
// intake identity gathered + provisioned = consumed + held + invested.
func naturalAssertConservationForMechanism(t *testing.T, f *Natural, mechanism NaturalMechanism) {
	t.Helper()
	for _, check := range f.Checkpoints() {
		b, err := NaturalCheckConservationFor(mechanism, f.patches, check.Patches, check.Slots, check.Founders, check.Newborns, check.AllocatorLast)
		if err != nil {
			t.Fatalf("h%d conservation: %v", check.Hour, err)
		}
		if b != check.Balance {
			t.Fatalf("h%d balance drift", check.Hour)
		}
		if b.Gathered+b.Provisioned != b.Consumed+b.Held+b.Invested {
			t.Fatalf("h%d extended G1 intake identity broken: gathered %d + provisioned %d != consumed %d + held %d + invested %d",
				check.Hour, b.Gathered, b.Provisioned, b.Consumed, b.Held, b.Invested)
		}
	}
}

// TestNaturalMechanismSeamControlGate2 is the frozen control gate 2: a
// world-v6 × natural/5 run reproduces the world-v5 × natural/5 trajectory
// field for field — the full projected state, balance, allocator cross-check
// and metrics at every hour, and the journal attempts (the ceiling ledger is
// the one excluded pin: its byte-derived rows are version-bound by design,
// digests identical only within each version) — with zero provisions on
// both sides.
//
// Two deliberate properties of this gate, stated so a reader does not
// mistake them for accidents: it is a LIVE-vs-LIVE comparison of two
// independently constructed worlds rather than an anchor against recorded
// bytes (the recorded-byte anchoring lives in the seven golden digest pins
// above, which cover h0/h5/h9 plus the births root and the fixture h6); and
// the two arms use different worker counts (v5 Workers 1, v6 Workers 2) on
// purpose, so the gate doubles as a worker-count determinism check.
func TestNaturalMechanismSeamControlGate2(t *testing.T) {
	v5, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1, Births: true})
	if err != nil {
		t.Fatal(err)
	}
	v6, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 2, Births: true,
		PolicyRef: strategy.FrozenNaturalPolicy().Ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatal(err)
	}
	if err := v5.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := v6.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checks5, checks6 := v5.Checkpoints(), v6.Checkpoints()
	if len(checks5) != len(checks6) {
		t.Fatalf("checkpoint counts diverge: %d vs %d", len(checks5), len(checks6))
	}
	for i := range checks5 {
		a, b := checks5[i], checks6[i]
		if a.Hour != b.Hour || a.Version != b.Version || a.Balance != b.Balance || a.AllocatorLast != b.AllocatorLast ||
			!reflect.DeepEqual(a.Patches, b.Patches) || !reflect.DeepEqual(a.Slots, b.Slots) ||
			!reflect.DeepEqual(a.Founders, b.Founders) || !reflect.DeepEqual(a.Newborns, b.Newborns) ||
			!naturalMetricsEqual(a.Metrics, b.Metrics) {
			t.Fatalf("h%d: mechanism v6 × natural/5 diverged from the frozen v5 trajectory", a.Hour)
		}
	}
	j5, j6 := v5.Journal(), v6.Journal()
	if len(j5) != len(j6) {
		t.Fatalf("journal batch counts diverge: %d vs %d", len(j5), len(j6))
	}
	for i := range j5 {
		if j5[i].Time != j6[i].Time || len(j5[i].Attempts) != len(j6[i].Attempts) {
			t.Fatalf("journal batch %d diverged", i)
		}
		for k := range j5[i].Attempts {
			if j5[i].Attempts[k] != j6[i].Attempts[k] {
				t.Fatalf("journal batch %d attempt %d diverged: %+v vs %+v", i, k, j5[i].Attempts[k], j6[i].Attempts[k])
			}
		}
	}
	for _, f := range []*Natural{v5, v6} {
		count := 0
		for _, b := range f.Journal() {
			for _, a := range b.Attempts {
				if a.Kind == "provision" {
					count++
				}
			}
		}
		if count != 0 || f.Checkpoints()[168].Balance.Provisioned != 0 {
			t.Fatalf("control gate 2 run recorded %d provisions", count)
		}
	}
}

// TestNaturalMechanismSeamSevenBNoProvisions proves the mechanism-necessity
// ablation arm on world v6: natural/7b records zero provision attempts and
// zero rule-515 events over the full horizon on the same cell where
// natural/7 provisions — the mechanism was actually exercised — and the
// ablation's own trajectory stays conservation-green.
func TestNaturalMechanismSeamSevenBNoProvisions(t *testing.T) {
	options := naturalMechanismSeamV7Options(0, true)
	options.PolicyRef = strategy.FrozenNaturalPolicy7b().Ref
	f, err := NewNatural(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	naturalAssertJournalClock(t, f)
	naturalAssertConservationForMechanism(t, f, NaturalMechanismV6)
	count, gathers, builds := 0, 0, 0
	for _, b := range f.Journal() {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "provision":
				count++
			case "gather":
				if a.Accepted {
					gathers++
				}
			case "build":
				if a.Accepted {
					builds++
				}
			}
		}
	}
	if count != 0 || f.Checkpoints()[168].Balance.Provisioned != 0 {
		t.Fatalf("natural/7b recorded %d provision attempts", count)
	}
	if gathers == 0 || builds == 0 {
		t.Fatal("natural/7b arm was inert; the ablation proved nothing")
	}
	// The twin arm on the same cell does provision, so the zero above isolates
	// exactly the disabled affordance (the mechanism was exercised).
	twin, err := NewNatural(naturalMechanismSeamV7Options(0, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := twin.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	twinProvisions := 0
	for _, b := range twin.Journal() {
		for _, a := range b.Attempts {
			if a.Kind == "provision" && a.Accepted {
				twinProvisions++
			}
		}
	}
	if twinProvisions == 0 {
		t.Fatal("the natural/7 twin provisioned nothing; the ablation comparison is empty")
	}
	if reflect.DeepEqual(twin.Journal(), f.Journal()) {
		t.Fatal("natural/7 and natural/7b journals identical; the ablation did not bite")
	}
}

// TestNaturalMechanismSeamFixtureV6RoundTrip proves the declared
// founder-tenure fixture genesis also carries the mechanism axis: a v6
// fixture runner seeds the granary provisioned ledger (complete-row genesis),
// passes the G2a′ genesis gate, provisions under natural/7, and its
// pulse-boundary bundle round-trips under the declared v6 expectation.
func TestNaturalMechanismSeamFixtureV6RoundTrip(t *testing.T) {
	f, err := newNaturalFixtureRunnerForPolicy(2, 8, 3, true, 2, strategy.FrozenNaturalPolicy7(), NaturalMechanismV6, 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.mechanism != NaturalMechanismV6 || f.ref != strategy.FrozenNaturalPolicy7().Ref {
		t.Fatalf("v6 fixture carries mechanism %d ref %+v", f.mechanism, f.ref)
	}
	// The fixture's first accepted provision commits at the h4 claim (the
	// founders' granary is banked, the home wild slots are gathered out, and
	// energy sits at the provision floor), one step past the h4 boundary.
	naturalStepToBoundary(t, f, 5)
	provisions := 0
	for _, b := range f.Journal() {
		for _, a := range b.Attempts {
			if a.Kind == "provision" && a.Accepted {
				provisions++
			}
		}
	}
	if provisions == 0 {
		t.Fatal("the v6 fixture run provisioned nothing under natural/7")
	}
	naturalAssertJournalClock(t, f)
	naturalAssertConservationForMechanism(t, f, NaturalMechanismV6)
	// The same fixture genesis under the v5 zero value refuses the provision
	// policies (the shared constructor enforces the ref/mechanism rule).
	if _, err := newNaturalFixtureRunnerForPolicy(2, 8, 3, true, 2, strategy.FrozenNaturalPolicy7(), NaturalMechanismV5, 0); !errors.Is(err, ErrNaturalCheckpoint) {
		t.Fatal("v5 fixture accepted natural/7")
	}
	path := t.TempDir() + "/v6-fixture-resume.bundle"
	if _, err := f.SaveResumeCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	m := naturalMechanismSeamManifest(t, path)
	if m.mechanism != NaturalMechanismV6 || m.policyFormat != strategy.NaturalPolicyFormatV7 || !m.tenure {
		t.Fatalf("v6 fixture manifest carries mechanism %d format %d tenure %v", m.mechanism, m.policyFormat, m.tenure)
	}
	restored, _, err := RestoreNaturalCheckpoint(path, NaturalRestoreOptions{Patches: 2, R: 8, Seed: 3, Workers: 3, Births: true,
		FounderTenure: true, PolicyRef: strategy.FrozenNaturalPolicy7().Ref, Mechanism: NaturalMechanismV6})
	if err != nil {
		t.Fatalf("v6 fixture restore: %v", err)
	}
	if !reflect.DeepEqual(restored.Journal(), f.Journal()) || !reflect.DeepEqual(restored.Checkpoints(), f.Checkpoints()) {
		t.Fatal("v6 fixture restore diverged")
	}
}

// Review P2 (L3 coverage gap): none of the seven golden pins above captures a
// committed rule-505 build event's bytes, so a v5 drift that only affects the
// build patch payload could pass every existing pin. This pin is a late-hour
// resume bundle on a builds-bearing v5 cell.
func TestNaturalMechanismSeamLateBuildsResumePinned(t *testing.T) {
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: 0, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	naturalRunTo(t, f, 100)
	builds := int64(0)
	for _, a := range f.Journal() {
		for _, attempt := range a.Attempts {
			if attempt.Kind == "build" && attempt.Key != "" {
				builds++
			}
		}
	}
	if builds == 0 {
		t.Fatal("fixture has no committed build events; the pin would be vacuous")
	}
	digest, err := f.SaveResumeCheckpoint(t.TempDir() + "/resume-h100.bundle")
	if err != nil {
		t.Fatal(err)
	}
	// Recorded independently at the pre-ticket-17 revision 0cca026 (in a
	// throwaway worktree) and reproduced byte-identically at the current
	// revision, so this pin both guards the committed build patch bytes
	// against future drift and documents that the ticket-17 mechanism axis
	// did not change the v5 builds-bearing resume bundle.
	const pinned = "11484b23154072afae0235a2aaa09f00929757265636a97019ff38435dbbda51"
	if got := naturalMechanismSeamDigest(t, digest); got != pinned {
		t.Fatalf("late builds-bearing v5 resume digest drifted: got %s", got)
	}
}
