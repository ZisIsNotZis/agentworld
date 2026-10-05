package world

import (
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
)

// Natural v5 evidence journal (ticket 15 persistence lane). The wire mirrors
// the reproduction v4 seam: canonical framing around a canonical JSON body,
// a scheduler-aware decoder, and a replay that is deliberately the trusted
// production runner rather than raw rule admission. A journal digest detects
// damage, not forgery; the enclosing checkpoint authenticates the whole
// bundle and independently retains its verified kernel head. The
// configuration is run identity: P (patches), the regeneration budget R, the
// seed, the two births flags (the v4 Boundary/Births discipline: Boundary is
// the births flag under which the journal's first pulse committed, Births the
// continuation flag — they differ only on a Branch lineage), the declared
// founder-tenure fixture flag (the births-capable genesis; see
// natural_checkpoint.go), and the pinned policy ref plus its exact content
// fingerprint. The neutral h0 world records no accepted birth-dependent
// action, so the kernel history alone cannot authenticate these values.
//
// Attempt coverage: every runner-recorded v5 attempt kind (gather, wait,
// eat-stored, eat, build with its typed slot-claim refusal notes,
// build-denied-eat, rest, paired-meal, fallback-eatstored, birth-request,
// birth-reply with its typed accept/refusal/silent evidence and the
// allocator-collision lost-birth note) is journaled and replayed
// attempt-for-attempt. The world-caused pulse rules (501 regen-split, 502
// spoilage, 503 wear, 504 basal, 508 cultivated-grant, 513 birth-expiry, 514
// age-death) commit no attempt rows — they are typed in the counts and
// verified through the full accepted event partition, whose every byte must
// equal the trusted replay, including rule 512 refusals (recorded as typed
// birth-reply evidence) and rule 511's entity-create births. There is no
// roster-derived attempt cap anywhere: the declared compute envelopes are
// this file's byte bound and the checkpoint store's section limit (P15).
const (
	NaturalJournalFormatVersion uint32 = 5
	MaxNaturalJournalBytes             = 16 << 20
	MaxNaturalJournalBatches           = NaturalStepCeiling
)

var ErrNaturalJournal = errors.New("invalid natural evidence journal")

// Distinct v5 journal magic: no earlier pilot's journal (v1 AWFJ, S6/v2
// AWSJ, v3 AWCJ, v4 AWRJ) can decode as a natural v5 journal, and vice
// versa; the format versions are disjoint as well (1/2/3/4/5).
var naturalJournalMagic = [4]byte{'A', 'W', 'N', 'J'}

// NaturalJournalConfig is the journal's run identity.
type NaturalJournalConfig struct {
	Patches int
	R       int64
	Seed    uint64
	// Births is the continuation flag: the recorded intervention on a branch
	// child, the runner's own flag otherwise. Boundary is the births flag of
	// the journal's first committed pulse — the flag under which the shared
	// first batch (and its wake topology) was produced. The two differ only
	// on a Branch lineage, where the shared first-hour prefix was produced
	// under Boundary and every later batch under Births.
	Births   bool
	Boundary bool
	// FounderTenure declares the births-capable founder-tenure fixture
	// genesis (natural_checkpoint.go). A neutral world and a fixture world
	// with the same P/R/seed are different runs; the flag keeps them apart.
	// CultivationDisabled declares the cultivation-ablation intervention
	// (matrix cell D); it is run identity like the births flags.
	FounderTenure       bool
	CultivationDisabled bool
	Policy              strategy.NaturalRef
	PolicyFingerprint   [32]byte
}

// NaturalJournalCounts is the typed coverage manifest of one journal: the
// runner-recorded attempt kinds plus the world-caused pulse rules counted
// from the accepted event partition (which carries no attempt rows).
type NaturalJournalCounts struct {
	GathersAdmitted, GathersDenied uint32
	EatStored, Waits, Eats         uint32
	Builds, BuildRefusals          uint32
	BuildDeniedEats                uint32
	PairedMeals, FallbackEatStored uint32
	Rests, NoStoredMealNotes       uint32
	BirthRequests, BirthReplies    uint32
	BirthsAccepted, BirthRefusals  uint32
	BirthRefusalsSilent            uint32
	LostBirths, BirthWakeClaims    uint32

	RegenSplits, Spoilage uint32
	Wear, Basal           uint32
	Grants, Expiries      uint32
	AgeDeaths             uint32
	BirthAllocations      uint32
}

type naturalJournalWire struct {
	Format        uint32
	Head          kernel.PortableHead
	SchedulerTime sim.SimTime
	Config        NaturalJournalConfig
	Counts        NaturalJournalCounts
	Batches       []NaturalBatch
}

// validNaturalJournalConfig checks the declared run identity's shape before
// any replay: a valid patch count, the budget inside the frozen range, the
// fixture's declared budget coherence, and the frozen policy identity.
func validNaturalJournalConfig(config NaturalJournalConfig) bool {
	if !NaturalPatchCountValid(config.Patches) || config.R < 0 || config.R > NaturalSlotsPerPatch {
		return false
	}
	if config.FounderTenure && config.R < naturalFixtureMinBudget {
		return false
	}
	if config.CultivationDisabled && config.FounderTenure {
		return false
	}
	return config.Policy == strategy.FrozenNaturalPolicy().Ref
}

func naturalJournalFingerprint() ([32]byte, error) {
	return strategy.NaturalPolicyFingerprint(strategy.FrozenNaturalPolicy())
}

// journalConfig pins the natural journal's run identity under the
// coordinator lock. An altered same-ref policy must never export under the
// frozen fingerprint: the fingerprint is left at a value no decoder accepts.
func (f *Natural) journalConfig() NaturalJournalConfig {
	fingerprint, err := naturalJournalFingerprint()
	if err != nil || f.k == nil || f.bound[1] == nil || f.bound[1].Policy() != strategy.FrozenNaturalPolicy() {
		fingerprint = [32]byte{}
	}
	return NaturalJournalConfig{Patches: f.patches, R: f.r, Seed: f.seed, Births: f.births, Boundary: f.births,
		FounderTenure: f.naturalRunnerTenure(), CultivationDisabled: f.cultivationDisabled,
		Policy: f.ref, PolicyFingerprint: fingerprint}
}

// ExportJournal publishes committed evidence only, not a disk bundle. The
// coordinator is locked across the history and scheduler consistency checks,
// and the journal is reconciled against a trusted replay before publication.
// The reconciliation replays under the runner's own births flag and compares
// the scheduler bytes, which is exact for every live runner and for restores
// of roots and pulse-boundary continuations; a restored Branch child carries
// its parent's boundary topology, so its journal verifies through the
// history-aware decode and the checkpoint restore paths instead.
func (f *Natural) ExportJournal() ([]byte, error) {
	if f == nil {
		return nil, ErrNaturalJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil || f.halted != nil {
		return nil, ErrNaturalJournal
	}
	_, head, err := f.k.ExportHistory()
	if err != nil {
		return nil, err
	}
	before, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || schedulerHead != head {
		return nil, ErrNaturalJournal
	}
	data, err := encodeNaturalJournal(f.journal, f.k, head, f.sched.Time(), f.journalConfig())
	if err != nil {
		return nil, err
	}
	reference, err := replayNaturalJournal(f.journal, f.journalConfig())
	if err != nil {
		return nil, err
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != head || !bytes.Equal(before, want) {
		return nil, ErrNaturalJournal
	}
	after, afterHead, err := f.sched.ExportPortable()
	if err != nil || afterHead != head || !bytes.Equal(before, after) {
		return nil, ErrNaturalJournal
	}
	return data, nil
}

// RestoreJournal replaces only the journal and only after full validation.
// Kernel, scheduler and other runner state are checkpoint responsibilities.
// It will not silently import a journal from another seed, budget, patch
// count, births flag, fixture flag, or policy fingerprint. Like
// ExportJournal, it pins Boundary to the runner's own births flag and
// therefore fails closed on a runner derived from a Branch child (whose
// boundary topology belongs to the parent); such lineages restore through
// the history-aware decode and the checkpoint restore paths instead.
func (f *Natural) RestoreJournal(data []byte, verified kernel.PortableHead) error {
	if f == nil {
		return ErrNaturalJournal
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.k == nil || f.sched == nil || f.pending != nil {
		return ErrNaturalJournal
	}
	batches, err := decodeNaturalJournalWithScheduler(data, f.k, verified, f.sched, f.journalConfig())
	if err != nil {
		return err
	}
	f.journal = batches
	return nil
}

// EncodeNaturalJournal validates accepted history and typed evidence at a
// closed timestamp under the frozen run identity. It does not certify open
// activities or scheduler wakes; ExportJournal and the scheduler-aware
// decoder do that at publication and restore.
func EncodeNaturalJournal(batches []NaturalBatch, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config NaturalJournalConfig) ([]byte, error) {
	return encodeNaturalJournal(batches, k, verified, schedulerTime, config)
}

func encodeNaturalJournal(batches []NaturalBatch, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config NaturalJournalConfig) ([]byte, error) {
	if err := verifyNaturalJournal(batches, k, verified, schedulerTime, config); err != nil {
		return nil, err
	}
	wire := naturalJournalWire{Format: NaturalJournalFormatVersion, Head: verified, SchedulerTime: schedulerTime,
		Config: config, Counts: naturalJournalCounts(batches, k.Events()), Batches: batches}
	body, err := json.Marshal(wire)
	if err != nil || len(body)+12+sha256.Size > MaxNaturalJournalBytes {
		return nil, ErrNaturalJournal
	}
	out := append([]byte(nil), naturalJournalMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, NaturalJournalFormatVersion)
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	out = append(out, body...)
	digest := sha256.Sum256(out)
	return append(out, digest[:]...), nil
}

// DecodeNaturalJournal validates canonical wire, run identity (including the
// two births flags, the fixture flag, the policy ref and content
// fingerprint), the typed counts over the full v5 attempt set, and the
// entire accepted event partition. This is history-only partial validation:
// a schedulerTime cannot prove open fibers, activity tokens, pending wakes
// (the reserved WakeBirth first claims included), or the runner's
// outstanding maps.
func DecodeNaturalJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config NaturalJournalConfig) ([]NaturalBatch, error) {
	return decodeNaturalJournal(data, k, verified, schedulerTime, config)
}

func decodeNaturalJournal(data []byte, k *kernel.Kernel, verified kernel.PortableHead, schedulerTime sim.SimTime, config NaturalJournalConfig) ([]NaturalBatch, error) {
	if k == nil {
		return nil, ErrNaturalJournal
	}
	if len(data) < 12+sha256.Size || len(data) > MaxNaturalJournalBytes || !bytes.Equal(data[:4], naturalJournalMagic[:]) ||
		binary.BigEndian.Uint32(data[4:8]) != NaturalJournalFormatVersion || int(binary.BigEndian.Uint32(data[8:12])) != len(data)-12-sha256.Size {
		return nil, ErrNaturalJournal
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return nil, ErrNaturalJournal
	}
	var wire naturalJournalWire
	decoder := json.NewDecoder(bytes.NewReader(data[12:len(body)]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || wire.Format != NaturalJournalFormatVersion || wire.Head != verified ||
		wire.SchedulerTime != schedulerTime || !sameNaturalJournalConfig(wire.Config, config) ||
		wire.Counts != naturalJournalCounts(wire.Batches, k.Events()) {
		return nil, ErrNaturalJournal
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, data[12:len(body)]) || verifyNaturalJournal(wire.Batches, k, verified, schedulerTime, wire.Config) != nil {
		return nil, ErrNaturalJournal
	}
	return wire.Batches, nil
}

// DecodeNaturalJournalWithScheduler is the complete checkpoint seam. The
// scheduler must first be restored independently against the verified head.
// Its canonical portable state must match the trusted replay of these
// batches, including open activities, tokens, pending wakes (the reserved
// WakeBirth first claims included) and stopped fibers.
func DecodeNaturalJournalWithScheduler(data []byte, k *kernel.Kernel, verified kernel.PortableHead, sched *scheduler.Scheduler, config NaturalJournalConfig) ([]NaturalBatch, error) {
	return decodeNaturalJournalWithScheduler(data, k, verified, sched, config)
}

func decodeNaturalJournalWithScheduler(data []byte, k *kernel.Kernel, verified kernel.PortableHead, sched *scheduler.Scheduler, config NaturalJournalConfig) ([]NaturalBatch, error) {
	if sched == nil {
		return nil, ErrNaturalJournal
	}
	before, head, err := sched.ExportPortable()
	if err != nil || head != verified {
		return nil, ErrNaturalJournal
	}
	batches, err := decodeNaturalJournal(data, k, verified, sched.Time(), config)
	if err != nil {
		return nil, ErrNaturalJournal
	}
	reference, err := replayNaturalJournal(batches, config)
	if err != nil {
		return nil, ErrNaturalJournal
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != verified || !bytes.Equal(before, want) {
		return nil, ErrNaturalJournal
	}
	after, afterHead, err := sched.ExportPortable()
	if err != nil || afterHead != verified || !bytes.Equal(before, after) {
		return nil, ErrNaturalJournal
	}
	return batches, nil
}

func sameNaturalJournalConfig(a, b NaturalJournalConfig) bool {
	return a.Patches == b.Patches && a.R == b.R && a.Seed == b.Seed &&
		a.Births == b.Births && a.Boundary == b.Boundary && a.FounderTenure == b.FounderTenure &&
		a.CultivationDisabled == b.CultivationDisabled &&
		a.Policy == b.Policy && a.PolicyFingerprint == b.PolicyFingerprint
}

// naturalJournalCounts recomputes the typed coverage manifest: attempt kinds
// from the recorded batches, the world-caused pulse rules from the accepted
// event partition. The two sources are exact complements — every committed
// instant is one batch, and every event is committed at a batch instant.
func naturalJournalCounts(batches []NaturalBatch, events []kernel.Event) NaturalJournalCounts {
	var c NaturalJournalCounts
	for _, b := range batches {
		for _, a := range b.Attempts {
			switch a.Kind {
			case "gather":
				if a.Accepted {
					c.GathersAdmitted++
				} else {
					c.GathersDenied++
				}
			case "eat":
				c.Eats++
			case "eat-stored":
				c.EatStored++
			case "wait":
				c.Waits++
			case "build":
				c.Builds++
				if a.Refused != "" {
					c.BuildRefusals++
				}
			case "build-denied-eat":
				c.BuildDeniedEats++
			case "paired-meal":
				c.PairedMeals++
			case "fallback-eatstored":
				c.FallbackEatStored++
			case "rest":
				c.Rests++
			case "birth-request":
				c.BirthRequests++
			case "birth-reply":
				c.BirthReplies++
				switch a.Birth.Decision {
				case NaturalReplyAccept:
					c.BirthsAccepted++
				case NaturalReplyRefuse:
					c.BirthRefusals++
				case NaturalReplyRefuseSilent:
					c.BirthRefusalsSilent++
				}
			default:
				continue
			}
			if a.NoMeal != "" {
				c.NoStoredMealNotes++
			}
			if a.BirthWake {
				c.BirthWakeClaims++
			}
			if a.Lost != "" {
				c.LostBirths++
			}
		}
	}
	for _, ev := range events {
		switch ev.Rule {
		case NaturalRegenSplitRule:
			c.RegenSplits++
		case NaturalSpoilageRule:
			c.Spoilage++
		case NaturalWearRule:
			c.Wear++
		case NaturalBasalRule:
			c.Basal++
		case NaturalGrantRule:
			c.Grants++
		case NaturalBirthExpireRule:
			c.Expiries++
		case NaturalAgeDeathRule:
			c.AgeDeaths++
		}
		if ev.Kind == kernel.KindEntityCreate {
			c.BirthAllocations++
		}
	}
	return c
}

// VerifyNaturalJournal is the exported full verification: canonical framing,
// run identity, typed counts, and a trusted replay whose accepted event
// partition must match the kernel's byte for byte.
func VerifyNaturalJournal(batches []NaturalBatch, k *kernel.Kernel, verified kernel.PortableHead, at sim.SimTime, config NaturalJournalConfig) error {
	return verifyNaturalJournal(batches, k, verified, at, config)
}

func verifyNaturalJournal(batches []NaturalBatch, k *kernel.Kernel, verified kernel.PortableHead, at sim.SimTime, config NaturalJournalConfig) error {
	if k == nil || !validNaturalJournalConfig(config) ||
		at < 0 || at > sim.SimTime(NaturalHorizonHours)*sim.SimTime(NaturalHour) || len(batches) > MaxNaturalJournalBatches {
		return ErrNaturalJournal
	}
	fingerprint, err := naturalJournalFingerprint()
	if err != nil || fingerprint != config.PolicyFingerprint {
		return ErrNaturalJournal
	}
	_, head, err := k.ExportHistory()
	if err != nil || head != verified {
		return ErrNaturalJournal
	}
	reference, err := replayNaturalJournal(batches, config)
	if err != nil || reference.sched.Time() != at {
		return ErrNaturalJournal
	}
	_, expectedHead, err := reference.k.ExportHistory()
	if err != nil || expectedHead != verified {
		return ErrNaturalJournal
	}
	got, expected := k.Events(), reference.k.Events()
	if len(got) != len(expected) {
		return ErrNaturalJournal
	}
	for i := range got {
		original, e1 := got[i].Bytes()
		trusted, e2 := expected[i].Bytes()
		if e1 != nil || e2 != nil || !bytes.Equal(original, trusted) || got[i].Hash != expected[i].Hash || got[i].PreviousHash != expected[i].PreviousHash {
			return ErrNaturalJournal
		}
	}
	return nil
}

// replayNaturalJournal rebuilds a trusted reference runner from the pinned
// configuration and steps it through the recorded batches. The reference is
// also the source of the runner-only state (dynamic roster, reserved
// WakeBirth reservations, denial evidence, deferred meals, the allocator's
// last ID and the ceiling ledger) that a checkpoint restore re-assembles;
// determinism makes it authoritative. The reference replays the shared
// first-hour prefix under the boundary flag and applies the recorded
// intervention after the first batch: a Branch lineage's journal is
// reproducible from genesis only under that two-flag schedule, and for every
// other bundle boundary and continuation flag are equal.
func replayNaturalJournal(batches []NaturalBatch, config NaturalJournalConfig) (*Natural, error) {
	if len(batches) > MaxNaturalJournalBatches {
		return nil, ErrNaturalJournal
	}
	reference, err := newNaturalReferenceRunner(NaturalRestoreOptions{Patches: config.Patches, R: config.R,
		Seed: config.Seed, Workers: 1, Births: config.Boundary, FounderTenure: config.FounderTenure,
		CultivationDisabled: config.CultivationDisabled})
	if err != nil {
		return nil, ErrNaturalJournal
	}
	for i, b := range batches {
		processed, stepErr := reference.Step(context.Background())
		if stepErr != nil || !processed || !sameNaturalJournalBatch(b, reference.journal[i]) {
			return nil, ErrNaturalJournal
		}
		if i == 0 {
			reference.births = config.Births
		}
	}
	return reference, nil
}

func sameNaturalJournalBatch(got, expected NaturalBatch) bool {
	if got.Time != expected.Time || got.Version != expected.Version || got.TipID != expected.TipID || got.TipHash != expected.TipHash || len(got.Attempts) != len(expected.Attempts) {
		return false
	}
	for i, a := range got.Attempts {
		// The reference replay re-derives every attempt deterministically
		// from the pinned configuration, including allocator denial reasons,
		// build-refusal notes and birth evidence, so the recorded attempt
		// must match exactly.
		if a != expected.Attempts[i] {
			return false
		}
	}
	return true
}

// naturalBoundaryBirths recovers the boundary flag of a committed journal:
// the first hour's request instant closes a batch exactly when that first
// pulse ran births-enabled (the request wakes exist only then), so the
// journal itself records the flag its shared prefix was produced under.
func naturalBoundaryBirths(journal []NaturalBatch, firstHour int) bool {
	at, err := NaturalPhaseTime(firstHour, NaturalPhaseBirthRequest)
	if err != nil {
		return false
	}
	for _, batch := range journal {
		if batch.Time == at {
			return true
		}
		if batch.Time > at {
			return false
		}
	}
	return false
}
