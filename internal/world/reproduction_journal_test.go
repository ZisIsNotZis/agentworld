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

func reproductionVerifiedHead(t *testing.T, f *Reproduction) kernel.PortableHead {
	t.Helper()
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func reproductionRecanonicalJournal(t *testing.T, data []byte, edit func(*reproductionJournalWire)) []byte {
	t.Helper()
	if len(data) < 12+sha256.Size || !bytes.Equal(data[:4], reproductionJournalMagic[:]) {
		t.Fatal("journal framing changed")
	}
	bodyLen := int(binary.BigEndian.Uint32(data[8:12]))
	var wire reproductionJournalWire
	if err := json.Unmarshal(data[12:12+bodyLen], &wire); err != nil {
		t.Fatal(err)
	}
	edit(&wire)
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), reproductionJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, ReproductionJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func TestReproductionJournalExportRestoreRoundTrip(t *testing.T) {
	// The q8 births-enabled fixture commits its first birth at h4, so an h6
	// journal carries birth attempts, a newborn's reserved WakeBirth claim,
	// and the world-caused pulse rules across several hours.
	options := ReproductionOptions{Yield: 8, Seed: 0, Workers: 4, Births: true}
	original := reproductionRunTo(t, options, 6)
	exported, err := original.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	var birthRequests, birthReplies, wakeClaims int
	for _, b := range original.Journal() {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "birth-request":
				birthRequests++
			case "birth-reply":
				birthReplies++
			}
			if a.BirthWake {
				wakeClaims++
			}
		}
	}
	if birthRequests == 0 || birthReplies == 0 || wakeClaims == 0 {
		t.Fatalf("fixture lacks birth evidence: requests=%d replies=%d wakeClaims=%d", birthRequests, birthReplies, wakeClaims)
	}
	other := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}, 6)
	if err := other.RestoreJournal(exported, reproductionVerifiedHead(t, other)); err != nil {
		t.Fatalf("journal restore: %v", err)
	}
	if !reflect.DeepEqual(other.Journal(), original.Journal()) {
		t.Fatal("restored journal differs from the original evidence")
	}
	again, err := other.ExportJournal()
	if err != nil || !bytes.Equal(again, exported) {
		t.Fatalf("restored export: %v", err)
	}
	// The births flag is run identity even at h0 where the world bytes are
	// identical: an h0 journal never restores onto the opposite flag.
	enabled := mustReproductionAtH0(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true})
	disabled := mustReproductionAtH0(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1})
	h0, err := enabled.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if err := disabled.RestoreJournal(h0, reproductionVerifiedHead(t, disabled)); !errors.Is(err, ErrReproductionJournal) {
		t.Fatalf("flag flipped after the fact: %v", err)
	}
	if err := enabled.RestoreJournal(mustReproductionAtH0(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1}).mustExportJournal(t), reproductionVerifiedHead(t, enabled)); !errors.Is(err, ErrReproductionJournal) {
		t.Fatal("enabled runner accepted a disabled journal")
	}
	// A journal from another seed or yield never restores.
	for _, wrong := range []ReproductionOptions{
		{Yield: 3, Seed: 0, Workers: 1, Births: true},
		{Yield: 8, Seed: 7, Workers: 1, Births: true},
	} {
		mismatched := reproductionRunTo(t, wrong, 6)
		if err := mismatched.RestoreJournal(exported, reproductionVerifiedHead(t, mismatched)); !errors.Is(err, ErrReproductionJournal) {
			t.Fatalf("restored foreign scenario %+v: %v", wrong, err)
		}
	}
	// Truncation and bit damage are rejected by framing and digest.
	for _, malformed := range [][]byte{exported[:12], exported[:len(exported)/2], append(bytes.Clone(exported), 0)} {
		if err := disabled.RestoreJournal(malformed, reproductionVerifiedHead(t, disabled)); !errors.Is(err, ErrReproductionJournal) {
			t.Fatalf("accepted malformed journal: %v", err)
		}
	}
	damaged := bytes.Clone(exported)
	damaged[len(damaged)/2] ^= 1
	if err := disabled.RestoreJournal(damaged, reproductionVerifiedHead(t, disabled)); !errors.Is(err, ErrReproductionJournal) {
		t.Fatalf("accepted damaged journal: %v", err)
	}
}

func mustReproductionAtH0(t *testing.T, options ReproductionOptions) *Reproduction {
	t.Helper()
	f, err := NewReproduction(options)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := f.Step(context.Background())
	if err != nil || !ok {
		t.Fatalf("h0 pulse: processed=%t err=%v", ok, err)
	}
	return f
}

func (f *Reproduction) mustExportJournal(t *testing.T) []byte {
	t.Helper()
	data, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReproductionJournalForgedEvidenceAfterTheFact(t *testing.T) {
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}, 6)
	exported, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	pristine := bytes.Clone(exported)
	head := reproductionVerifiedHead(t, f)
	config := f.journalConfig()
	at := f.sched.Time()
	reject := func(name string, data []byte) {
		t.Helper()
		if batches, err := DecodeReproductionJournalWithScheduler(data, f.k, head, f.sched, config); batches != nil || !errors.Is(err, ErrReproductionJournal) {
			t.Fatalf("%s: decoded=%v err=%v", name, batches, err)
		}
	}
	reject("damaged", func() []byte { d := bytes.Clone(exported); d[len(d)/2] ^= 1; return d }())
	// A linked attempt relabeled as unlinked loses its event link. Gather
	// attempts are keyless by runner design (the gather activity is guarded
	// by its own commit), so the forgery targets a keyed build attempt.
	buildFound := false
	reject("forged-unlinked-build", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) {
		for bi, b := range w.Batches {
			for ai, a := range w.Batches[bi].Attempts {
				if a.Kind == "build" && a.Accepted {
					w.Batches[bi].Attempts[ai].Accepted = false
					_ = b
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
	reject("forged-birth-decision", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) {
		for bi, b := range w.Batches {
			for ai, a := range b.Attempts {
				if a.Kind == "birth-reply" && a.Birth.Decision == ReproductionReplyAccept {
					w.Batches[bi].Attempts[ai].Birth.Decision = ReproductionReplyRefuse
					return
				}
			}
		}
		t.Fatal("fixture produced no accepted birth reply")
	}))
	// A forged typed note breaks the recomputed counts.
	fresh := f.mustExportJournal(t)
	reject("forged-note", reproductionRecanonicalJournal(t, fresh, func(w *reproductionJournalWire) {
		for bi, b := range w.Batches {
			for ai, a := range b.Attempts {
				if a.Kind == "build" {
					w.Batches[bi].Attempts[ai].NoMeal = capacityNoStoredMealNote
					return
				}
			}
		}
		t.Fatal("fixture produced no build attempt")
	}))
	// Tampered counts, tampered config (flag, fingerprint, founder genomes),
	// an unknown attempt kind, and a forged head all fail closed against the
	// canonical wire and the trusted replay.
	reject("forged-counts", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) { w.Counts.Builds++ }))
	reject("forged-counts-births", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) { w.Counts.BirthReplies++ }))
	reject("forged-config-flag", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) { w.Config.Births = !w.Config.Births }))
	reject("forged-config-fingerprint", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) { w.Config.PolicyFingerprint[0] ^= 1 }))
	reject("forged-founder-locus", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) {
		w.Config.Founders[0].Genome.LocusM = 7
	}))
	reject("forged-founder-extra", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) {
		w.Config.Founders = append(w.Config.Founders, ReproductionFounderGenomeRow{Actor: 17,
			Genome: ReproductionGenomeState{LocusM: 6, LocusY: 6, LocusW: 6, BirthHour: 0, DeathHour: 120, DiedHour: ReproductionNeverHour}})
	}))
	reject("unknown-kind", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) {
		for bi, b := range w.Batches {
			if len(b.Attempts) > 0 {
				w.Batches[bi].Attempts[0].Kind = "salvage"
				return
			}
		}
		t.Fatal("fixture produced no attempts")
	}))
	reject("forged-head", reproductionRecanonicalJournal(t, exported, func(w *reproductionJournalWire) { w.Head.TipID++ }))
	// The untouched journal still decodes.
	if batches, err := DecodeReproductionJournalWithScheduler(pristine, f.k, head, f.sched, config); err != nil || len(batches) == 0 {
		t.Fatalf("valid journal rejected after forgeries: %v", err)
	}
	// Partial decode API agrees with the scheduler-aware one.
	if batches, err := DecodeReproductionJournal(pristine, f.k, head, at, config); err != nil || len(batches) == 0 {
		t.Fatalf("history-only decode: %v", err)
	}
	// An out-of-range journal config is rejected before any replay.
	bad := config
	bad.Yield = ReproductionSlotsPerPatch + 1
	if _, err := DecodeReproductionJournal(pristine, f.k, head, at, bad); !errors.Is(err, ErrReproductionJournal) {
		t.Fatalf("accepted out-of-range config: %v", err)
	}
	bad = config
	bad.Founders = nil
	if _, err := DecodeReproductionJournal(pristine, f.k, head, at, bad); !errors.Is(err, ErrReproductionJournal) {
		t.Fatalf("accepted founderless config: %v", err)
	}
}

// TestReproductionJournalReplayCatchesRawKernelBypass proves the spec's
// negative fixture: a schema-valid, in-bounds proposal committed straight
// onto the kernel (bypassing the runner's ownership guard) breaks the journal
// replay, and the replayed ownership audit names the exact violation. A
// cross-owner stored-meal forgery mutates actor 1's granary under actor 2's
// cause; a locus tamper writes an immutable genome allele under the age-death
// rule, which may write died-hour only.
func TestReproductionJournalReplayCatchesRawKernelBypass(t *testing.T) {
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}, 2)
	head := f.k.SnapshotHead()
	_, authority, _ := f.k.Snapshot()
	forged := kernel.Proposal{Key: "reproduction/forged-cross-owner", Time: head.TipTime + 1,
		Cause: kernel.Cause{Actor: 2}, Rule: ReproductionEatStoredRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: ReproductionGranaryTypeID, SchemaVersion: ReproductionSchemaVersion,
			Field: ReproductionGranaryLastStoredMealHourField, Value: sim.IntegerValue(0)}}}
	plan, err := f.k.Plan(forged, authority)
	if err != nil {
		t.Fatalf("kernel unexpectedly rejected the in-range bypass: %v", err)
	}
	if _, _, err := f.k.CommitBatchAtHead(head, []kernel.Plan{plan}); err != nil {
		t.Fatalf("raw-kernel bypass did not commit: %v", err)
	}
	// The ownership audit the runner applies before planning rejects the
	// replayed proposal rebuilt from the committed deltas.
	event := f.k.Events()[len(f.k.Events())-1]
	patches := make([]component.Patch, 0, len(event.Deltas))
	for _, d := range event.Deltas {
		patches = append(patches, component.Patch{Entity: d.Entity, Component: d.Component, SchemaVersion: d.SchemaVersion, Field: d.Field, Value: d.After})
	}
	replayed := kernel.Proposal{Key: event.Key, Time: event.Time, Cause: event.Cause, Rule: event.Rule, RuleVersion: event.RuleVersion, Patches: patches}
	if err := ReproductionCheckOwnership(replayed); !errors.Is(err, ErrReproductionContract) {
		t.Fatalf("cross-owner mutation survived the replay ownership audit: %v", err)
	}
	if data, err := f.ExportJournal(); err == nil || data != nil {
		t.Fatalf("journal export accepted a bypassed history: %v", err)
	}
	// A genome locus tamper under rule 410 is schema-valid and
	// kernel-admissible at the component level (rule 410 may write the
	// genome's birth-granary-paid), but no runner proposal ever writes a
	// locus: the replayed ownership audit rejects it and the trusted replay
	// never produces the event.
	head = f.k.SnapshotHead()
	_, authority, _ = f.k.Snapshot()
	locus := kernel.Proposal{Key: "reproduction/forged-locus", Time: head.TipTime + 1,
		Cause: kernel.Cause{Actor: 2}, Rule: ReproductionBirthReplyRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{{Entity: 3, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion,
			Field: ReproductionGenomeLocusMField, Value: sim.IntegerValue(7)}}}
	plan, err = f.k.Plan(locus, authority)
	if err != nil {
		t.Fatalf("kernel unexpectedly rejected the locus tamper: %v", err)
	}
	if _, _, err := f.k.CommitBatchAtHead(head, []kernel.Plan{plan}); err != nil {
		t.Fatalf("raw-kernel locus tamper did not commit: %v", err)
	}
	if err := ReproductionCheckOwnership(kernel.Proposal{Key: "reproduction/forged-locus", Time: event.Time,
		Cause: kernel.Cause{Actor: 2}, Rule: ReproductionBirthReplyRule, RuleVersion: ReproductionRuleVersion,
		Patches: []component.Patch{{Entity: 3, Component: ReproductionGenomeTypeID, SchemaVersion: ReproductionSchemaVersion,
			Field: ReproductionGenomeLocusMField, Value: sim.IntegerValue(7)}}}); !errors.Is(err, ErrReproductionContract) {
		t.Fatalf("locus tamper survived the replay ownership audit: %v", err)
	}
	if data, err := f.ExportJournal(); err == nil || data != nil {
		t.Fatalf("journal export accepted a locus tamper: %v", err)
	}
}

// TestReproductionJournalReplayBounds pins gate G7 on the reconstruction
// path: every integer field of every hourly projection — genome rows
// included — is bounds-checked against the frozen registry, and unauthorized
// delta shapes are rejected instead of ignored.
func TestReproductionJournalReplayBounds(t *testing.T) {
	table, err := reproductionBoundsTableForRegistry()
	if err != nil {
		t.Fatal(err)
	}
	f := reproductionRunTo(t, ReproductionOptions{Yield: 8, Seed: 0, Workers: 1, Births: true}, 6)
	for _, check := range f.Checkpoints() {
		if err := reproductionProjectionBounds(check, table); err != nil {
			t.Fatalf("h%d legitimate projection failed bounds: %v", check.Hour, err)
		}
	}
	if len(f.Checkpoints()[6].Newborns) == 0 {
		t.Fatal("fixture produced no newborn rows to bound")
	}
	for name, mutate := range map[string]func(*ReproductionCheckpoint){
		"locus-above-max":  func(c *ReproductionCheckpoint) { c.Founders[0].Genome.LocusM = ReproductionGenomeLocusMax + 1 },
		"locus-below-min":  func(c *ReproductionCheckpoint) { c.Founders[1].Genome.LocusY = ReproductionGenomeLocusMin - 1 },
		"newborn-locus":    func(c *ReproductionCheckpoint) { c.Newborns[0].State.Genome.LocusW = 9 },
		"negative-stock":   func(c *ReproductionCheckpoint) { c.Founders[2].Granary.Stock = -1 },
		"capital-over-max": func(c *ReproductionCheckpoint) { c.Founders[3].Worksite.Capital = ReproductionKMax + 1 },
		"debt-over-max":    func(c *ReproductionCheckpoint) { c.Founders[4].Body.BasalDebt = ReproductionBasalDebtMax + 1 },
		"hour-below-min":   func(c *ReproductionCheckpoint) { c.Founders[5].Body.LastGatherHour = ReproductionNeverHour - 1 },
		"patch-over-max": func(c *ReproductionCheckpoint) {
			c.Patches[0].Unrealized = ReproductionHorizonHours*ReproductionSlotsPerPatch + 1
		},
		"slot-stock-over": func(c *ReproductionCheckpoint) { c.Slots[1][0].Stock = 2 },
		"status-over-max": func(c *ReproductionCheckpoint) { c.Founders[6].Request.Status = 5 },
	} {
		forged := f.Checkpoints()[6]
		mutate(&forged)
		if err := reproductionProjectionBounds(forged, table); !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("%s: forged projection passed G7: %v", name, err)
		}
	}
	// The shadow reconstruction rejects unauthorized component/field/rule
	// shapes instead of ignoring them.
	var p reproductionProjection
	for i := range p.patches {
		p.patches[i].Yield = 8
	}
	for name, spec := range map[string]struct {
		rule  sim.RuleID
		delta component.FieldDelta
	}{
		"patch-yield-ledger":  {ReproductionProduceRule, component.FieldDelta{Entity: 1001, Component: ReproductionPatchTypeID, Field: ReproductionPatchYieldField, After: sim.IntegerValue(1)}},
		"slot-patch-ref":      {ReproductionProduceRule, component.FieldDelta{Entity: 2001, Component: ReproductionSlotTypeID, Field: ReproductionSlotPatchField, After: sim.IntegerValue(0)}},
		"bag-source-patch":    {ReproductionGatherRule, component.FieldDelta{Entity: 1, Component: ReproductionBagTypeID, Field: ReproductionBagSourceField, After: sim.IntegerValue(0)}},
		"unknown-body-field":  {ReproductionBasalRule, component.FieldDelta{Entity: 1, Component: ReproductionBodyTypeID, Field: sim.FieldID(99), After: sim.IntegerValue(0)}},
		"unknown-entity":      {ReproductionBasalRule, component.FieldDelta{Entity: 99, Component: ReproductionBodyTypeID, Field: ReproductionBodyEnergyField, After: sim.IntegerValue(1)}},
		"unknown-component":   {ReproductionBasalRule, component.FieldDelta{Entity: 1, Component: sim.ComponentTypeID(0xDEAD), Field: ReproductionBodyEnergyField, After: sim.IntegerValue(1)}},
		"locus-under-413":     {ReproductionAgeDeathRule, component.FieldDelta{Entity: 1, Component: ReproductionGenomeTypeID, Field: ReproductionGenomeLocusMField, After: sim.IntegerValue(7)}},
		"died-hour-under-406": {ReproductionWearRule, component.FieldDelta{Entity: 1, Component: ReproductionGenomeTypeID, Field: ReproductionGenomeDiedHourField, After: sim.IntegerValue(3)}},
		"paid-under-404":      {ReproductionBasalRule, component.FieldDelta{Entity: 1, Component: ReproductionGenomeTypeID, Field: ReproductionGenomeBirthGranaryPaidField, After: sim.IntegerValue(2)}},
	} {
		if err := p.applyPatchDelta(spec.rule, spec.delta); !errors.Is(err, ErrReproductionCheckpoint) {
			t.Fatalf("%s: unauthorized delta accepted: %+v", name, spec.delta)
		}
	}
	// The only genome writes a patch may carry: died-hour under 404/413 and
	// birth-granary-paid under 410.
	if err := p.applyPatchDelta(ReproductionAgeDeathRule, component.FieldDelta{Entity: 1, Component: ReproductionGenomeTypeID, Field: ReproductionGenomeDiedHourField, After: sim.IntegerValue(3)}); err != nil {
		t.Fatalf("rule-413 died-hour rejected: %v", err)
	}
	if p.deaths.age != 1 {
		t.Fatal("age death not attributed")
	}
	if err := p.applyPatchDelta(ReproductionBirthReplyRule, component.FieldDelta{Entity: 2, Component: ReproductionGenomeTypeID, Field: ReproductionGenomeBirthGranaryPaidField, After: sim.IntegerValue(2)}); err != nil {
		t.Fatalf("rule-410 birth-granary-paid rejected: %v", err)
	}
	// The bag source ref decodes present and missing refs in allocations.
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	ref, _ := sim.EntityRefValue(1001)
	for _, delta := range []component.FieldDelta{
		{Entity: 10001, Component: ReproductionBagTypeID, Field: ReproductionBagSourceField, After: ref},
		{Entity: 10001, Component: ReproductionBagTypeID, Field: ReproductionBagSourceField, After: missing},
	} {
		if err := p.applyAllocationDelta(delta); err != nil {
			t.Fatalf("allocation ref rejected: %v", err)
		}
	}
	if p.staging[10001].Bag.Source != 0 {
		t.Fatal("missing ref not applied")
	}
	// A founder entity can never be staged as a newborn allocation.
	if err := p.applyAllocationDelta(component.FieldDelta{Entity: 1, Component: ReproductionBagTypeID, Field: ReproductionBagUnitsField, After: sim.IntegerValue(0)}); !errors.Is(err, ErrReproductionCheckpoint) {
		t.Fatal("founder allocation accepted")
	}
	// The frozen founder derivation is the seed's founder stream.
	want, err := expectedReproductionFounderGenomes(0)
	if err != nil {
		t.Fatal(err)
	}
	if !validReproductionFounderGenomes(want) {
		t.Fatal("derived founder rows invalid")
	}
	other, err := expectedReproductionFounderGenomes(7)
	if err != nil {
		t.Fatal(err)
	}
	if equalReproductionFounderGenomes(want, other) {
		t.Fatal("founder derivation ignores the seed")
	}
	if _, err := DecodeReproductionJournal(nil, f.k, reproductionVerifiedHead(t, f), 0, f.journalConfig()); !errors.Is(err, ErrReproductionJournal) {
		t.Fatal("empty journal accepted")
	}
}
