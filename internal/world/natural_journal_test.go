package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// The natural journal tests (ticket 15 persistence lane). The births-bearing
// evidence comes from the declared founder-tenure fixture (see
// natural_checkpoint.go): the default organic cell collapses into the
// recorded poverty trap and never births, so post-birth states are always
// constructed explicitly.

// naturalJournalFixture steps a fresh fixture world to the given checkpoint
// hour (the fixture's first checkpoint is naturalFixtureStartHour).
func naturalJournalFixture(t *testing.T, seed uint64, births bool, workers, hour int) *Natural {
	t.Helper()
	f, err := newNaturalFixtureRunner(2, 8, seed, births, workers)
	if err != nil {
		t.Fatal(err)
	}
	naturalStepToBoundary(t, f, hour)
	return f
}

// naturalStepToBoundary steps until the runner's last hourly checkpoint is
// at least the given hour (the fixture's checkpoints start at hour 2, so a
// checkpoint-count predicate would overshoot).
func naturalStepToBoundary(t *testing.T, f *Natural, hour int) {
	t.Helper()
	for {
		checks := f.Checkpoints()
		if len(checks) > 0 && checks[len(checks)-1].Hour >= hour {
			return
		}
		ok, err := f.Step(context.Background())
		if err != nil {
			t.Fatalf("hour %d step %d: %v", hour, f.steps, err)
		}
		if !ok {
			t.Fatalf("hour %d: run ended early at step %d", hour, f.steps)
		}
	}
}

func naturalVerifiedHead(t *testing.T, f *Natural) kernel.PortableHead {
	t.Helper()
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func naturalMustExportJournal(t *testing.T, f *Natural) []byte {
	t.Helper()
	data, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func naturalRecanonicalJournal(t *testing.T, data []byte, edit func(*naturalJournalWire)) []byte {
	t.Helper()
	if len(data) < 12+sha256.Size || !bytes.Equal(data[:4], naturalJournalMagic[:]) {
		t.Fatal("journal framing changed")
	}
	bodyLen := int(binary.BigEndian.Uint32(data[8:12]))
	var wire naturalJournalWire
	if err := json.Unmarshal(data[12:12+bodyLen], &wire); err != nil {
		t.Fatal(err)
	}
	edit(&wire)
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), naturalJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, NaturalJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

// TestNaturalJournalExportRestoreRoundTrip proves the export/restore seam on
// the fixture world at h16, whose journal carries the full v5 evidence set:
// gathers, builds, eat-stored meals, paired and fallback meals, rests, birth
// requests, accepted and refused replies, the reserved WakeBirth claims, and
// the world-caused pulse rules including a birth-expiry. The journal
// restores onto independently run twins at different worker counts and
// re-exports byte-identically.
func TestNaturalJournalExportRestoreRoundTrip(t *testing.T) {
	const seed = 3
	original := naturalJournalFixture(t, seed, true, 4, 16)
	exported := naturalMustExportJournal(t, original)
	counts := map[string]int{}
	wakeClaims, refusals, accepts := 0, 0, 0
	for _, b := range original.Journal() {
		for _, a := range b.Attempts {
			counts[a.Kind]++
			switch {
			case a.BirthWake:
				wakeClaims++
			case a.Kind == "birth-reply" && a.Birth.Decision == NaturalReplyAccept:
				accepts++
			case a.Kind == "birth-reply":
				refusals++
			}
		}
	}
	for _, kind := range []string{"gather", "build", "eat-stored", "rest", "birth-request", "birth-reply"} {
		if counts[kind] == 0 {
			t.Fatalf("fixture lacks %s evidence: %+v", kind, counts)
		}
	}
	if wakeClaims == 0 || accepts == 0 || refusals == 0 {
		t.Fatalf("fixture lacks birth evidence: claims=%d accepts=%d refusals=%d", wakeClaims, accepts, refusals)
	}
	journal := original.Journal()
	last := original.Checkpoints()
	tip := last[len(last)-1]
	// The world-caused rules are typed in the counts and verified through the
	// accepted event partition.
	wire := naturalJournalWire{}
	bodyLen := int(binary.BigEndian.Uint32(exported[8:12]))
	if err := json.Unmarshal(exported[12:12+bodyLen], &wire); err != nil {
		t.Fatal(err)
	}
	if int64(wire.Counts.BirthAllocations) != tip.Balance.Births || wire.Counts.BirthAllocations == 0 {
		t.Fatalf("allocation count %d does not match the committed births %d", wire.Counts.BirthAllocations, tip.Balance.Births)
	}
	if wire.Counts.RegenSplits == 0 || wire.Counts.Grants == 0 || wire.Counts.Spoilage == 0 ||
		wire.Counts.Wear == 0 || wire.Counts.Basal == 0 {
		t.Fatalf("world-caused rules not covered: %+v", wire.Counts)
	}
	if wire.Counts.Expiries == 0 {
		t.Fatalf("birth-expiry not covered: %+v", wire.Counts)
	}
	// Restore onto independently run twins at other worker counts.
	for _, workers := range []int{1, 2} {
		other := naturalJournalFixture(t, seed, true, workers, 16)
		if err := other.RestoreJournal(exported, naturalVerifiedHead(t, other)); err != nil {
			t.Fatalf("workers=%d journal restore: %v", workers, err)
		}
		if !reflect.DeepEqual(other.Journal(), journal) {
			t.Fatalf("workers=%d restored journal differs", workers)
		}
		again, err := other.ExportJournal()
		if err != nil || !bytes.Equal(again, exported) {
			t.Fatalf("workers=%d re-export: %v", workers, err)
		}
	}
	// The births and fixture flags are run identity even where world bytes
	// would coincide: the fixture's first-hour journal never restores onto
	// the opposite flag.
	enabled := naturalJournalFixture(t, seed, true, 1, naturalFixtureStartHour)
	disabled := naturalJournalFixture(t, seed, false, 1, naturalFixtureStartHour)
	h0, err := enabled.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if err := disabled.RestoreJournal(h0, naturalVerifiedHead(t, disabled)); !errors.Is(err, ErrNaturalJournal) {
		t.Fatal("flag flipped after the fact")
	}
	if err := enabled.RestoreJournal(naturalMustExportJournal(t, disabled), naturalVerifiedHead(t, enabled)); !errors.Is(err, ErrNaturalJournal) {
		t.Fatal("enabled runner accepted a disabled journal")
	}
	// A journal from another seed or budget never restores.
	for _, wrong := range []NaturalRestoreOptions{
		{Patches: 2, R: 8, Seed: 9, Workers: 1, Births: true, FounderTenure: true},
		{Patches: 2, R: 7, Seed: seed, Workers: 1, Births: true, FounderTenure: true},
		{Patches: 1, R: 8, Seed: seed, Workers: 1, Births: true, FounderTenure: true},
	} {
		mismatched, err := newNaturalFixtureRunner(wrong.Patches, wrong.R, wrong.Seed, true, 1)
		if err != nil {
			t.Fatal(err)
		}
		naturalStepToBoundary(t, mismatched, 4)
		if err := mismatched.RestoreJournal(exported, naturalVerifiedHead(t, mismatched)); !errors.Is(err, ErrNaturalJournal) {
			t.Fatalf("restored foreign scenario %+v", wrong)
		}
	}
	// Truncation and bit damage are rejected by framing and digest.
	for _, malformed := range [][]byte{exported[:12], exported[:len(exported)/2], append(bytes.Clone(exported), 0)} {
		if err := disabled.RestoreJournal(malformed, naturalVerifiedHead(t, disabled)); !errors.Is(err, ErrNaturalJournal) {
			t.Fatal("accepted malformed journal")
		}
	}
	damaged := bytes.Clone(exported)
	damaged[len(damaged)/2] ^= 1
	if err := disabled.RestoreJournal(damaged, naturalVerifiedHead(t, disabled)); !errors.Is(err, ErrNaturalJournal) {
		t.Fatal("accepted damaged journal")
	}
	// The neutral h0 journal of a default world exports and re-exports.
	neutral := naturalNeutralAt(t, 0, false, 1)
	if data := naturalMustExportJournal(t, neutral); len(data) == 0 {
		t.Fatal("empty neutral journal")
	}
}

// naturalNeutralAt builds a default NewNatural world stepped to hour.
func naturalNeutralAt(t *testing.T, seed uint64, births bool, workers int) *Natural {
	t.Helper()
	f, err := NewNatural(NaturalOptions{Patches: 2, R: 8, Seed: seed, Workers: workers, Births: births})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := f.Step(context.Background()); err != nil || !ok {
		t.Fatalf("h0 pulse: %v %v", ok, err)
	}
	return f
}

// TestNaturalJournalForgedEvidenceAfterTheFact recanonicalizes the wire
// after every forgery, so only the semantic checks (counts, config identity,
// trusted replay) can reject.
func TestNaturalJournalForgedEvidenceAfterTheFact(t *testing.T) {
	f := naturalJournalFixture(t, 3, true, 1, 16)
	exported := naturalMustExportJournal(t, f)
	pristine := bytes.Clone(exported)
	head := naturalVerifiedHead(t, f)
	config := f.journalGenesisKind()
	at := f.sched.Time()
	reject := func(name string, data []byte) {
		t.Helper()
		if batches, err := DecodeNaturalJournalWithScheduler(data, f.k, head, f.sched, config); batches != nil || !errors.Is(err, ErrNaturalJournal) {
			t.Fatalf("%s: decoded=%v err=%v", name, batches, err)
		}
	}
	reject("damaged", func() []byte { d := bytes.Clone(exported); d[len(d)/2] ^= 1; return d }())
	// A linked attempt relabeled as unlinked loses its event link.
	buildFound := false
	reject("forged-unlinked-build", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) {
		for bi := range w.Batches {
			for ai := range w.Batches[bi].Attempts {
				if a := &w.Batches[bi].Attempts[ai]; a.Kind == "build" && a.Accepted {
					a.Accepted = false
					buildFound = true
					return
				}
			}
		}
	}))
	if !buildFound {
		t.Fatal("fixture produced no linked build attempt to forge")
	}
	// A forged birth-reply decision breaks the trusted replay.
	reject("forged-birth-decision", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) {
		for bi := range w.Batches {
			for ai := range w.Batches[bi].Attempts {
				if a := &w.Batches[bi].Attempts[ai]; a.Kind == "birth-reply" && a.Birth.Decision == NaturalReplyAccept {
					a.Birth.Decision = NaturalReplyRefuse
					return
				}
			}
		}
		t.Fatal("fixture produced no accepted birth reply")
	}))
	// A forged typed note breaks the recomputed counts.
	reject("forged-note", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) {
		for bi := range w.Batches {
			for ai := range w.Batches[bi].Attempts {
				if a := &w.Batches[bi].Attempts[ai]; a.Kind == "build" {
					a.NoMeal = capacityNoStoredMealNote
					return
				}
			}
		}
		t.Fatal("fixture produced no build attempt")
	}))
	// A forged build-refusal note on an accepted build breaks the counts and
	// the replay (the recorded build committed).
	reject("forged-refusal-note", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) {
		for bi := range w.Batches {
			for ai := range w.Batches[bi].Attempts {
				if a := &w.Batches[bi].Attempts[ai]; a.Kind == "build" && a.Accepted {
					a.Refused = naturalBuildRefusalNote(NaturalBuildRefusalNoWildSlot)
					return
				}
			}
		}
		t.Fatal("fixture produced no build attempt")
	}))
	// Tampered counts (attempt and world-rule coverage), tampered config
	// (flags, fixture flag, fingerprint, patches, budget), an unknown
	// attempt kind, and a forged head all fail closed.
	reject("forged-counts", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Counts.Builds++ }))
	reject("forged-counts-world", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Counts.Wear++ }))
	reject("forged-counts-allocations", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Counts.BirthAllocations++ }))
	reject("forged-config-flag", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Config.Births = !w.Config.Births }))
	reject("forged-config-boundary", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Config.Boundary = !w.Config.Boundary }))
	reject("forged-config-tenure", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Config.FounderTenure = false }))
	reject("forged-config-fingerprint", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Config.PolicyFingerprint[0] ^= 1 }))
	reject("forged-config-patches", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Config.Patches = 1 }))
	reject("forged-config-budget", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Config.R = 7 }))
	reject("unknown-kind", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) {
		for bi := range w.Batches {
			if len(w.Batches[bi].Attempts) > 0 {
				w.Batches[bi].Attempts[0].Kind = "salvage"
				return
			}
		}
		t.Fatal("fixture produced no attempts")
	}))
	reject("forged-head", naturalRecanonicalJournal(t, exported, func(w *naturalJournalWire) { w.Head.TipID++ }))
	// The untouched journal still decodes through both APIs.
	if batches, err := DecodeNaturalJournalWithScheduler(pristine, f.k, head, f.sched, config); err != nil || len(batches) == 0 {
		t.Fatalf("valid journal rejected after forgeries: %v", err)
	}
	if batches, err := DecodeNaturalJournal(pristine, f.k, head, at, config); err != nil || len(batches) == 0 {
		t.Fatalf("history-only decode: %v", err)
	}
	// An out-of-range journal config is rejected before any replay.
	bad := config
	bad.R = NaturalSlotsPerPatch + 1
	if _, err := DecodeNaturalJournal(pristine, f.k, head, at, bad); !errors.Is(err, ErrNaturalJournal) {
		t.Fatal("accepted out-of-range config")
	}
	bad = config
	bad.Patches = NaturalMaxPatchesFixedIDSpace + 1
	if _, err := DecodeNaturalJournal(pristine, f.k, head, at, bad); !errors.Is(err, ErrNaturalJournal) {
		t.Fatal("accepted out-of-range patch count")
	}
}

// TestNaturalJournalReplayCatchesRawKernelBypass proves the persistence
// negative fixture: schema-valid, in-bounds proposals committed straight
// onto the kernel (bypassing the runner's ownership guard) break the journal
// replay, and the replayed ownership audit names the exact violation. A
// tenure forgery writes slot ownership under rule 501, which may write slot
// stock only; a genome forgery writes an immutable allele under rule 514,
// which may write died-hour only.
func TestNaturalJournalReplayCatchesRawKernelBypass(t *testing.T) {
	f := naturalNeutralAt(t, 0, false, 1)
	head := f.k.SnapshotHead()
	_, authority, _ := f.k.Snapshot()
	slotID, err := NaturalSlotID(2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	ownerRef, err := sim.EntityRefValue(3)
	if err != nil {
		t.Fatal(err)
	}
	forged := kernel.Proposal{Key: "natural/forged-tenure", Time: head.TipTime + 1,
		Cause: kernel.Cause{World: true}, Rule: NaturalRegenSplitRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{{Entity: slotID, Component: NaturalSlotTypeID, SchemaVersion: NaturalSchemaVersion,
			Field: NaturalSlotTenureOwnerField, Value: ownerRef}}}
	plan, err := f.k.Plan(forged, authority)
	if err != nil {
		t.Fatalf("kernel unexpectedly rejected the in-range tenure bypass: %v", err)
	}
	if _, _, err := f.k.CommitBatchAtHead(head, []kernel.Plan{plan}); err != nil {
		t.Fatalf("raw-kernel tenure bypass did not commit: %v", err)
	}
	event := f.k.Events()[len(f.k.Events())-1]
	patches := make([]component.Patch, 0, len(event.Deltas))
	for _, d := range event.Deltas {
		patches = append(patches, component.Patch{Entity: d.Entity, Component: d.Component, SchemaVersion: d.SchemaVersion, Field: d.Field, Value: d.After})
	}
	replayed := kernel.Proposal{Key: event.Key, Time: event.Time, Cause: event.Cause, Rule: event.Rule, RuleVersion: event.RuleVersion, Patches: patches}
	if err := NaturalCheckOwnership(replayed, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("tenure forgery survived the replay ownership audit: %v", err)
	}
	if data, err := f.ExportJournal(); err == nil || data != nil {
		t.Fatalf("journal export accepted a bypassed history: %v", err)
	}
	// A genome locus tamper under rule 514 is schema-valid and
	// kernel-admissible at the component level (rule 514 may write the
	// genome's died-hour), but no runner proposal ever writes a locus: the
	// replayed ownership audit rejects it and the trusted replay never
	// produces the event.
	head = f.k.SnapshotHead()
	_, authority, _ = f.k.Snapshot()
	locus := kernel.Proposal{Key: "natural/forged-locus", Time: head.TipTime + 1,
		Cause: kernel.Cause{World: true}, Rule: NaturalAgeDeathRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{{Entity: 3, Component: NaturalGenomeTypeID, SchemaVersion: NaturalSchemaVersion,
			Field: NaturalGenomeLocusMField, Value: sim.IntegerValue(7)}}}
	plan, err = f.k.Plan(locus, authority)
	if err != nil {
		t.Fatalf("kernel unexpectedly rejected the locus tamper: %v", err)
	}
	if _, _, err := f.k.CommitBatchAtHead(head, []kernel.Plan{plan}); err != nil {
		t.Fatalf("raw-kernel locus tamper did not commit: %v", err)
	}
	if err := NaturalCheckOwnership(kernel.Proposal{Key: "natural/forged-locus", Time: head.TipTime + 1,
		Cause: kernel.Cause{World: true}, Rule: NaturalAgeDeathRule, RuleVersion: NaturalRuleVersion,
		Patches: []component.Patch{{Entity: 3, Component: NaturalGenomeTypeID, SchemaVersion: NaturalSchemaVersion,
			Field: NaturalGenomeLocusMField, Value: sim.IntegerValue(7)}}}, 2); !errors.Is(err, ErrNaturalContract) {
		t.Fatalf("locus tamper survived the replay ownership audit: %v", err)
	}
	if data, err := f.ExportJournal(); err == nil || data != nil {
		t.Fatalf("journal export accepted a locus tamper: %v", err)
	}
}

// TestNaturalJournalCountsPinTheTypedSet pins the typed attempt coverage on
// a synthetic batch: every v5 attempt kind and world-caused pulse rule has
// its own count, including the kinds the organic fixture windows do not
// reach (build refusals with their denial-loop re-eats, the
// allocator-collision lost birth, and the age death).
func TestNaturalJournalCountsPinTheTypedSet(t *testing.T) {
	at, _ := NaturalPhaseTime(2, NaturalPhaseClaim)
	batches := []NaturalBatch{{
		Time: at, Version: 1,
		Attempts: []NaturalAttempt{
			{Actor: 1, Kind: "gather", Accepted: true},
			{Actor: 2, Kind: "gather", Rejection: FoodFlowGatherCapacity},
			{Actor: 3, Kind: "eat-stored", Key: "k1"},
			{Actor: 4, Kind: "wait"},
			{Actor: 5, Kind: "eat", Key: "k2"},
			{Actor: 6, Kind: "build", Key: "k3", Refused: naturalBuildRefusalNote(NaturalBuildRefusalNoWildSlot)},
			{Actor: 6, Kind: "build-denied-eat", Key: "k4"},
			{Actor: 7, Kind: "paired-meal", Key: "k5", NoMeal: capacityNoStoredMealNote},
			{Actor: 8, Kind: "fallback-eatstored", Key: "k6"},
			{Actor: 9, Kind: "rest"},
			{Actor: 1, Kind: "birth-request", Key: "k7", BirthWake: true},
			{Actor: 2, Kind: "birth-reply", Birth: NaturalBirthEvidence{Decision: NaturalReplyAccept, Newborn: 2017}},
			{Actor: 3, Kind: "birth-reply", Birth: NaturalBirthEvidence{Decision: NaturalReplyRefuse, Reason: NaturalRefusalGateThresholds}},
			{Actor: 4, Kind: "birth-reply", Birth: NaturalBirthEvidence{Decision: NaturalReplyRefuseSilent, Reason: NaturalRefusalProposerDead}},
			{Actor: 5, Kind: "birth-reply", Lost: naturalLostCollision, Birth: NaturalBirthEvidence{Decision: NaturalReplyAccept, Newborn: 2017}},
		}}}
	events := []kernel.Event{
		{Rule: NaturalRegenSplitRule}, {Rule: NaturalRegenSplitRule},
		{Rule: NaturalSpoilageRule}, {Rule: NaturalWearRule}, {Rule: NaturalBasalRule},
		{Rule: NaturalGrantRule}, {Rule: NaturalBirthExpireRule}, {Rule: NaturalAgeDeathRule},
		{Rule: NaturalBirthReplyRule, Kind: kernel.KindEntityCreate},
	}
	counts := naturalJournalCounts(batches, events)
	want := NaturalJournalCounts{
		GathersAdmitted: 1, GathersDenied: 1, EatStored: 1, Waits: 1, Eats: 1,
		Builds: 1, BuildRefusals: 1, BuildDeniedEats: 1,
		PairedMeals: 1, FallbackEatStored: 1, Rests: 1, NoStoredMealNotes: 1,
		BirthRequests: 1, BirthReplies: 4, BirthsAccepted: 2, BirthRefusals: 1,
		BirthRefusalsSilent: 1, LostBirths: 1, BirthWakeClaims: 1,
		RegenSplits: 2, Spoilage: 1, Wear: 1, Basal: 1, Grants: 1,
		Expiries: 1, AgeDeaths: 1, BirthAllocations: 1,
	}
	if counts != want {
		t.Fatalf("counts diverge: %+v", counts)
	}
	// The bounds-table helper rejects no legitimate projection field.
	table, err := naturalBoundsTableForRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(table[NaturalGranaryTypeID]) != 6 || len(table[NaturalGenomeTypeID]) != 7 {
		t.Fatalf("bounds table shape changed: %v", table)
	}
	if at == 0 {
		t.Fatal("phase instants changed")
	}
}
