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

func capacityVerifiedHead(t *testing.T, f *Capacity) kernel.PortableHead {
	t.Helper()
	_, head, err := f.k.ExportHistory()
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func capacityRecanonicalJournal(t *testing.T, data []byte, edit func(*capacityJournalWire)) []byte {
	t.Helper()
	if len(data) < 12+sha256.Size || !bytes.Equal(data[:4], capacityJournalMagic[:]) {
		t.Fatal("journal framing changed")
	}
	bodyLen := int(binary.BigEndian.Uint32(data[8:12]))
	var wire capacityJournalWire
	if err := json.Unmarshal(data[12:12+bodyLen], &wire); err != nil {
		t.Fatal(err)
	}
	edit(&wire)
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), capacityJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, CapacityJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func TestCapacityJournalExportRestoreRoundTrip(t *testing.T) {
	options := CapacityOptions{Yield: 3, Seed: 7, Workers: 4, Enabled: true}
	original := capacityRunTo(t, options, 24)
	exported, err := original.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	other := capacityRunTo(t, options, 24)
	if err := other.RestoreJournal(exported, capacityVerifiedHead(t, other)); err != nil {
		t.Fatalf("journal restore: %v", err)
	}
	if !reflect.DeepEqual(other.Journal(), original.Journal()) {
		t.Fatal("restored journal differs from the original evidence")
	}
	again, err := other.ExportJournal()
	if err != nil || !bytes.Equal(again, exported) {
		t.Fatalf("restored export: %v", err)
	}
	// The intervention flag is run identity even where the world bytes are
	// flag-neutral: an h0 journal never restores onto the opposite flag.
	enabled := mustCapacityAtH0(t, CapacityOptions{Yield: 3, Seed: 7, Workers: 1, Enabled: true})
	disabled := mustCapacityAtH0(t, CapacityOptions{Yield: 3, Seed: 7, Workers: 1})
	h0, err := enabled.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	if err := disabled.RestoreJournal(h0, capacityVerifiedHead(t, disabled)); !errors.Is(err, ErrCapacityJournal) {
		t.Fatalf("flag flipped after the fact: %v", err)
	}
	// A journal from another seed, yield, or endowment never restores.
	for _, wrong := range []CapacityOptions{
		{Yield: 8, Seed: 7, Workers: 1, Enabled: true},
		{Yield: 3, Seed: 0, Workers: 1, Enabled: true},
		{Yield: 3, Seed: 7, Workers: 1, Enabled: true, Founders: []CapacityFounder{{Actor: 1, Granary: 4}}},
	} {
		mismatched := capacityRunTo(t, wrong, 24)
		if err := mismatched.RestoreJournal(exported, capacityVerifiedHead(t, mismatched)); !errors.Is(err, ErrCapacityJournal) {
			t.Fatalf("restored foreign scenario %+v: %v", wrong, err)
		}
	}
	// Truncation and bit damage are rejected by framing and digest.
	for _, malformed := range [][]byte{exported[:12], exported[:len(exported)/2], append(bytes.Clone(exported), 0)} {
		if err := disabled.RestoreJournal(malformed, capacityVerifiedHead(t, disabled)); !errors.Is(err, ErrCapacityJournal) {
			t.Fatalf("accepted malformed journal: %v", err)
		}
	}
	damaged := bytes.Clone(exported)
	damaged[len(damaged)/2] ^= 1
	if err := disabled.RestoreJournal(damaged, capacityVerifiedHead(t, disabled)); !errors.Is(err, ErrCapacityJournal) {
		t.Fatalf("accepted damaged journal: %v", err)
	}
}

func mustCapacityAtH0(t *testing.T, options CapacityOptions) *Capacity {
	t.Helper()
	f, err := NewCapacity(options)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := f.Step(context.Background())
	if err != nil || !ok {
		t.Fatalf("h0 pulse: processed=%t err=%v", ok, err)
	}
	return f
}

func TestCapacityJournalForgedEvidenceAfterTheFact(t *testing.T) {
	f := capacityRunTo(t, CapacityOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}, 24)
	exported, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	pristine := bytes.Clone(exported)
	head := capacityVerifiedHead(t, f)
	config := f.journalConfig()
	at := f.sched.Time()
	reject := func(name string, data []byte) {
		t.Helper()
		if batches, err := DecodeCapacityJournalWithScheduler(data, f.k, head, f.sched, config); batches != nil || !errors.Is(err, ErrCapacityJournal) {
			t.Fatalf("%s: decoded=%v err=%v", name, batches, err)
		}
	}
	reject("damaged", func() []byte { d := bytes.Clone(exported); d[len(d)/2] ^= 1; return d }())
	// A denial relabeled as an admitted gather has no event link.
	denialFound := false
	exported = capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) {
		for bi, b := range w.Batches {
			for ai, a := range b.Attempts {
				if a.Kind == "gather" && !a.Accepted {
					w.Batches[bi].Attempts[ai].Accepted = true
					w.Batches[bi].Attempts[ai].Rejection = FoodFlowGatherAdmitted
					denialFound = true
					return
				}
			}
		}
	})
	if !denialFound {
		t.Fatal("fixture produced no gather denial to forge")
	}
	reject("forged-accepted-denial", exported)
	// A forged typed note breaks the recomputed counts.
	fresh, err := f.ExportJournal()
	if err != nil {
		t.Fatal(err)
	}
	reject("forged-note", capacityRecanonicalJournal(t, fresh, func(w *capacityJournalWire) {
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
	// Tampered counts, tampered config flag, and an unknown attempt kind all
	// fail closed against the canonical wire and the trusted replay.
	reject("forged-counts", capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) { w.Counts.Builds++ }))
	reject("forged-config-flag", capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) { w.Config.Enabled = !w.Config.Enabled }))
	reject("forged-config-fingerprint", capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) { w.Config.PolicyFingerprint[0] ^= 1 }))
	reject("forged-founder", capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) {
		w.Config.Founders = append(w.Config.Founders, CapacityFounder{Actor: 2, Granary: 1})
	}))
	reject("unknown-kind", capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) {
		for bi, b := range w.Batches {
			if len(b.Attempts) > 0 {
				w.Batches[bi].Attempts[0].Kind = "salvage"
				return
			}
		}
		t.Fatal("fixture produced no attempts")
	}))
	reject("forged-head", capacityRecanonicalJournal(t, exported, func(w *capacityJournalWire) { w.Head.TipID++ }))
	// The untouched journal still decodes.
	if batches, err := DecodeCapacityJournalWithScheduler(pristine, f.k, head, f.sched, config); err != nil || len(batches) == 0 {
		t.Fatalf("valid journal rejected after forgeries: %v", err)
	}
	// Partial decode API agrees with the scheduler-aware one.
	if batches, err := DecodeCapacityJournal(pristine, f.k, head, at, config); err != nil || len(batches) == 0 {
		t.Fatalf("history-only decode: %v", err)
	}
	// An out-of-range journal config is rejected before any replay.
	bad := config
	bad.Yield = CapacitySlotsPerPatch + 1
	if _, err := DecodeCapacityJournal(pristine, f.k, head, at, bad); !errors.Is(err, ErrCapacityJournal) {
		t.Fatalf("accepted out-of-range config: %v", err)
	}
}

// TestCapacityJournalReplayCatchesRawKernelBypass proves the spec's negative
// fixture: a schema-valid, in-bounds proposal committed straight onto the
// kernel (bypassing the runner's ownership guard) breaks the journal replay,
// and the replayed ownership audit names the exact violation. A cross-owner
// stored-meal forgery mutates actor 1's granary under actor 2's cause.
func TestCapacityJournalReplayCatchesRawKernelBypass(t *testing.T) {
	f := capacityRunTo(t, CapacityOptions{Yield: 3, Seed: 0, Workers: 1, Enabled: true}, 2)
	head := f.k.SnapshotHead()
	_, authority, _ := f.k.Snapshot()
	forged := kernel.Proposal{Key: "capacity/forged-cross-owner", Time: head.TipTime + 1,
		Cause: kernel.Cause{Actor: 2}, Rule: CapacityEatStoredRule, RuleVersion: CapacityRuleVersion,
		Patches: []component.Patch{{Entity: 1, Component: CapacityGranaryTypeID, SchemaVersion: CapacitySchemaVersion,
			Field: CapacityGranaryLastStoredMealHourField, Value: sim.IntegerValue(0)}}}
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
	if err := CapacityCheckOwnership(replayed); !errors.Is(err, ErrCapacityContract) {
		t.Fatalf("cross-owner mutation survived the replay ownership audit: %v", err)
	}
	// The journal replay is the second line of defense: the trusted runner
	// never produces this event, so export fails closed and the forged event
	// cannot hide in any published bundle.
	if data, err := f.ExportJournal(); err == nil || data != nil {
		t.Fatalf("journal export accepted a bypassed history: %v", err)
	}
}

// TestCapacityJournalReplayBounds pins gate G7 on the reconstruction path:
// every integer field of every hourly projection is bounds-checked against
// the frozen registry, and unauthorized delta shapes are rejected instead of
// ignored.
func TestCapacityJournalReplayBounds(t *testing.T) {
	table, err := capacityBoundsTableForRegistry()
	if err != nil {
		t.Fatal(err)
	}
	f := capacityRunTo(t, CapacityOptions{Yield: 8, Seed: 0, Workers: 1, Enabled: true}, 8)
	for _, check := range f.Checkpoints() {
		if err := capacityProjectionBounds(check, table); err != nil {
			t.Fatalf("h%d legitimate projection failed bounds: %v", check.Hour, err)
		}
	}
	for name, mutate := range map[string]func(*CapacityCheckpoint){
		"capital-above-max":  func(c *CapacityCheckpoint) { c.Actors[0].Worksite.Capital = CapacityKMax + 1 },
		"negative-stock":     func(c *CapacityCheckpoint) { c.Actors[1].Granary.Stock = -1 },
		"wear-debt-over-max": func(c *CapacityCheckpoint) { c.Actors[2].Worksite.WearDebt = CapacityWearDebtMax + 1 },
		"hour-below-min":     func(c *CapacityCheckpoint) { c.Actors[3].Body.LastGatherHour = CapacityNeverHour - 1 },
		"patch-over-max":     func(c *CapacityCheckpoint) { c.Patches[0].Unrealized = CapacityHorizonHours*CapacitySlotsPerPatch + 1 },
		"slot-stock-over":    func(c *CapacityCheckpoint) { c.Slots[1][0].Stock = 2 },
	} {
		forged := f.Checkpoints()[8]
		forged.Hour = 8
		mutate(&forged)
		if err := capacityProjectionBounds(forged, table); !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("%s: forged projection passed G7: %v", name, err)
		}
	}
	// The shadow reconstruction rejects unauthorized component/field shapes.
	var patches [CapacityPatchCount]CapacityPatchState
	var slots [CapacityPatchCount][CapacitySlotsPerPatch]CapacitySlotState
	var actors [CapacityActorCount]CapacityActorState
	for _, delta := range []component.FieldDelta{
		{Entity: 1001, Component: CapacityPatchTypeID, Field: CapacityPatchYieldField, After: sim.IntegerValue(1)},
		{Entity: 1, Component: CapacityWorksiteTypeID, Field: sim.FieldID(99), After: sim.IntegerValue(0)},
		{Entity: 99, Component: CapacityBodyTypeID, Field: CapacityBodyEnergyField, After: sim.IntegerValue(1)},
		{Entity: 1, Component: sim.ComponentTypeID(0xDEAD), Field: CapacityBodyEnergyField, After: sim.IntegerValue(1)},
	} {
		if err := applyCapacityCheckpointDelta(&patches, &slots, &actors, delta); !errors.Is(err, ErrCapacityCheckpoint) {
			t.Fatalf("unauthorized delta accepted: %+v -> %v", delta, err)
		}
	}
	// The bag source ref decodes present and missing refs.
	missing, _ := sim.AbsentValue(sim.EntityRefKind, sim.Missing)
	ref, _ := sim.EntityRefValue(1001)
	if err := applyCapacityCheckpointDelta(&patches, &slots, &actors, component.FieldDelta{Entity: 1, Component: CapacityBagTypeID, Field: CapacityBagSourceField, After: ref}); err != nil {
		t.Fatalf("present ref rejected: %v", err)
	}
	if actors[0].Bag.Source != 1001 {
		t.Fatal("present ref not applied")
	}
	if err := applyCapacityCheckpointDelta(&patches, &slots, &actors, component.FieldDelta{Entity: 1, Component: CapacityBagTypeID, Field: CapacityBagSourceField, After: missing}); err != nil {
		t.Fatalf("missing ref rejected: %v", err)
	}
	if actors[0].Bag.Source != 0 {
		t.Fatal("missing ref not applied")
	}
}
