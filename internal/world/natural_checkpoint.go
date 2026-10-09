package world

import (
	"agentworld/internal/checkpoint"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sort"
)

// Natural v5 checkpoint bundles (ticket 15 persistence lane) are a
// separately versioned format with a distinct manifest: they never load or
// migrate food-flow v1, social-food v2, survival S6, capacity v3,
// reproduction v4, or any other pilot's sections, and no earlier pilot loads
// theirs. The pilot root (SaveCheckpoint) exists only at the neutral h0
// pulse boundary, before any claim, capital, or birth event; a
// pulse-boundary continuation bundle (SaveResumeCheckpoint) may be captured
// at any later hourly boundary, after births included. Enabling births for a
// continued run happens only through an explicit recorded Branch whose
// intervention is carried in the manifest lineage and re-pinned in the
// journal configuration; the pinned policy bytes are never edited. A restore
// refuses any manifest whose recorded scenario does not exactly match the
// declared options; the worker count is never taken from the bundle.
//
// # The declared founder-tenure fixture
//
// The default organic cell collapses into the recorded poverty trap (no
// births ever commit), so every post-birth state is constructed explicitly
// through a declared, deterministic fixture genesis: the founder-tenure
// fixture. It lifts the neutral seed-derived genesis by two ledger-coherent
// pulse cycles of pre-history (patches at pulses 2) plus two cultivated
// slots and a stocked granary for founders 1 and 2, placing both parents at
// the frozen surplus-gate margins (k=2, granary=3, energy=5) so births
// commit under the frozen policy within the first pulses. The fixture is
// run identity (the journal config's FounderTenure flag and the manifest's
// tenure flag): its world, replay, and projection reconstruction all derive
// from the same declared function of (P, R, seed), and no other genesis can
// pass as it. Its first pulse is naturalFixtureStartHour, so a fixture
// runner is recognized exactly by its first committed checkpoint hour.
const naturalCheckpointFormat uint32 = 1

// naturalFixtureStartHour is the first pulse hour of the declared
// founder-tenure fixture: its pre-history occupies pulses 0 and 1, and
// pulses == hour at every pulse pins the start.
const naturalFixtureStartHour = 2

// naturalFixtureMinBudget is the smallest regeneration budget for which the
// fixture's ledger pre-history is coherent: patch 0 needs
// produced(8) + granted(6) ≤ R·2 and the idle patches need 8 ≤ R·2.
const naturalFixtureMinBudget int64 = 7

// naturalMaxBranchDepth bounds lineage chains so a forged manifest cannot
// claim an unbounded ancestry, and so Branch refuses pre-publication any
// child that could never be restored.
const naturalMaxBranchDepth = 63

var ErrNaturalCheckpoint = errors.New("invalid or incompatible natural v5 checkpoint")

// naturalManifestPolicyFormatSlot is the manifest's policy-format slot: the
// position of the declared policy format inside the pinned u32 constant
// block written by encodeNaturalManifest. It is one of the parameterized
// slots of the frozen manifest layout (ticket 16 D1 seam): its value is run
// identity — naturalPolicyFormats, i.e. 5 for a natural/5 bundle, 6 for a
// natural/6 bundle, 7 for natural/7 and 8 for natural/7b — and every other
// constant stays byte-pinned, so the manifest width and field order are
// unchanged.
const naturalManifestPolicyFormatSlot = 6

// naturalManifestMechanismWorldFormatSlot is the manifest's world-format
// slot (the frozen NaturalFormatVersion position): the first slot of the
// mechanism block and the one that fixes the declared mechanism at decode.
const naturalManifestMechanismWorldFormatSlot = 1

// The manifest's mechanism block (ticket 17): the four world-version slots
// of the pinned u32 constant block — format, schema, rule, projection — are
// written from one mechanism value and decode only as the exact v5 tuple
// (5,5,5,5) or the exact v6 tuple (6,6,6,6). The v5 zero value writes the
// frozen constants, so every recorded v5 bundle keeps its bytes; the width
// and field order of the manifest are unchanged.
var naturalManifestMechanismSlots = []int{1, 3, 4, 5}

// naturalPolicyFormats is the closed set of manifest policy-format values a
// decoder accepts; anything else fails closed.
func naturalPolicyFormats() map[uint32]bool {
	return map[uint32]bool{strategy.NaturalPolicyFormatV5: true, strategy.NaturalPolicyFormatV6: true,
		strategy.NaturalPolicyFormatV7: true, strategy.NaturalPolicyFormatV7b: true,
		strategy.NaturalPolicyFormatV9: true}
}

// naturalManifestPolicyRef resolves the manifest's declared policy identity
// from its policy-format slot: 5 is natural/5, 6 is natural/6, 7 is
// natural/7, 8 is natural/7b, 9 is natural/9 (ticket 20), anything else
// fails closed (decode already rejects foreign slots).
func (m naturalManifest) naturalManifestPolicyRef() (strategy.NaturalRef, error) {
	switch m.policyFormat {
	case strategy.NaturalPolicyFormatV5:
		return strategy.FrozenNaturalPolicy().Ref, nil
	case strategy.NaturalPolicyFormatV6:
		return strategy.FrozenNaturalPolicyV6().Ref, nil
	case strategy.NaturalPolicyFormatV7:
		return strategy.FrozenNaturalPolicy7().Ref, nil
	case strategy.NaturalPolicyFormatV7b:
		return strategy.FrozenNaturalPolicy7b().Ref, nil
	case strategy.NaturalPolicyFormatV9:
		return strategy.FrozenNaturalPolicy9().Ref, nil
	}
	return strategy.NaturalRef{}, ErrNaturalCheckpoint
}

// Distinct v5 magics: no earlier pilot's bundle or strategy section can
// decode as a natural v5 manifest or strategy section, and vice versa
// (bundles AWFM/AWS2/AWC3/AWR4/AWSM, strategies AWFS/AWSF/AWCT/AWRT).
var naturalCheckpointMagic = [4]byte{'A', 'W', 'N', '5'}
var naturalStrategyMagic = [4]byte{'A', 'W', 'N', 'T'}

const (
	naturalMaxStrategyPolicyBytes        = 128
	naturalStrategyFormat         uint32 = 1
)

// NaturalRestoreOptions is the declared scenario a bundle must match
// exactly: the fixture shape of NaturalOptions (P, R, seed, workers, births,
// mechanism) plus the lane's declared founder-tenure fixture flag, the
// declared cultivation-ablation intervention flag, and the declared policy
// ref (ticket 16 D1 seam; the zero value is exactly natural/5, so every
// recorded v5 expectation keeps its bytes). The mechanism axis (ticket 17)
// is declared with the same zero value ≡ v5 default-preservation and is
// matched bidirectionally against the manifest's mechanism block: a v5
// bundle refuses a v6 expectation and vice versa, before any restore work.
// The worker count is the caller's choice, never the bundle's.
type NaturalRestoreOptions struct {
	Patches             int
	R                   int64
	Seed                uint64
	Workers             int
	Births              bool
	FounderTenure       bool
	CultivationDisabled bool
	PolicyRef           strategy.NaturalRef
	Mechanism           NaturalMechanism

	// Founders is the declared founding-band fixture axis (ticket 18): the
	// bundle must materialize exactly this founder prefix of the frozen ID
	// space, matched bidirectionally against the manifest's founders field
	// before any restore work or branch publication. The zero value is the
	// full frozen block 8P (default-preservation: every recorded world
	// restores unchanged); an explicit 8P is the same spelling of the same
	// world.
	Founders int

	// CognitiveLedgerPath, when set for a cognitive bundle, is the
	// recorded-call ledger the manifest cognition block binds: a missing,
	// short, or mismatched ledger refuses the restore (spec section 7.2). The
	// zero value performs no ledger check.
	CognitiveLedgerPath string

	// CognitiveReplaySeam and CognitiveReplayWake, when set for a seam-active
	// cognitive bundle, are the recorded Role A seam the reference replay
	// wires so the recorded seam decisions reproduce from the recorded-call
	// ledger without a model call (spec section 7.3). They are replay
	// machinery, not live state: the returned runner carries no live seam.
	CognitiveReplaySeam ClaimSeam
	CognitiveReplayWake protocol.WakePolicy
}

// policyRef resolves the declared policy identity: the zero value is the
// frozen natural/5 ref (D1 default-preservation).
func (o NaturalRestoreOptions) policyRef() strategy.NaturalRef {
	if o.PolicyRef == (strategy.NaturalRef{}) {
		return strategy.FrozenNaturalPolicy().Ref
	}
	return o.PolicyRef
}

// mechanism resolves the declared mechanism axis: the zero value is the
// frozen v5 mechanism (default-preservation).
func (o NaturalRestoreOptions) mechanism() NaturalMechanism {
	if o.Mechanism == 0 {
		return NaturalMechanismV5
	}
	return o.Mechanism
}

func (o NaturalRestoreOptions) validate() error {
	if !NaturalPatchCountValid(o.Patches) {
		return fmt.Errorf("%w: patches %d outside the derived ID-space precondition", ErrNaturalCheckpoint, o.Patches)
	}
	if o.R < 0 || o.R > NaturalSlotsPerPatch {
		return fmt.Errorf("%w: R %d outside [0,%d]", ErrNaturalCheckpoint, o.R, NaturalSlotsPerPatch)
	}
	if o.Workers < 1 {
		return fmt.Errorf("%w: workers %d", ErrNaturalCheckpoint, o.Workers)
	}
	if o.FounderTenure && o.R < naturalFixtureMinBudget {
		return fmt.Errorf("%w: fixture R %d below the coherent pre-history bound %d", ErrNaturalCheckpoint, o.R, naturalFixtureMinBudget)
	}
	// The cultivation ablation is declared on the organic genesis only: no
	// frozen matrix cell combines it with the founder-tenure fixture.
	if o.CultivationDisabled && o.FounderTenure {
		return fmt.Errorf("%w: cultivation ablation with the founder-tenure fixture is not a declared scenario", ErrNaturalCheckpoint)
	}
	if !NaturalFoundersValid(o.Patches, o.Founders) {
		return fmt.Errorf("%w: founders %d outside the derived founding-band bound [1,%d] (zero = default 8P)", ErrNaturalCheckpoint, o.Founders, NaturalSlotsPerPatch*o.Patches)
	}
	// The fixture's coherent pre-history seats founders 1 and 2, so a
	// fixture cell at f == 1 is not a declared scenario (f == 0 is the
	// full-block default, E is f == 8P, E12 is f == 12).
	if o.FounderTenure && o.Founders == 1 {
		return fmt.Errorf("%w: founder-tenure fixture at founders 1 is not a declared scenario (the pre-history seats founders 1 and 2)", ErrNaturalCheckpoint)
	}
	if _, err := strategy.FrozenNaturalPolicyForRef(o.policyRef()); err != nil {
		return fmt.Errorf("%w: policy ref %+v is not in the frozen natural ref set {natural/5, natural/6, natural/7, natural/7b, natural/9}: %v", ErrNaturalCheckpoint, o.PolicyRef, err)
	}
	if !NaturalMechanismValid(o.mechanism()) {
		return fmt.Errorf("%w: mechanism %d is not an admitted mechanism version", ErrNaturalCheckpoint, uint16(o.Mechanism))
	}
	if !naturalRefLegalUnderMechanism(o.mechanism(), o.policyRef()) {
		return fmt.Errorf("%w: policy ref %+v is not legal under mechanism %d (natural/7 and natural/7b require mechanism >= v6; natural/9 requires mechanism v7)", ErrNaturalCheckpoint, o.policyRef(), uint16(o.mechanism()))
	}
	return nil
}

// newNaturalReferenceRunner constructs the independently built executable of
// one declared scenario: the neutral NewNatural world, or the declared
// founder-tenure fixture world. It is the trusted replay's genesis, the save
// paths' independent constructor, and the restore skeleton.
func newNaturalReferenceRunner(expected NaturalRestoreOptions) (*Natural, error) {
	if err := expected.validate(); err != nil {
		return nil, err
	}
	// The declared ref resolves against the executable's frozen ref set
	// before any constructor runs: the reference runner and the world must
	// bind the same policy bytes.
	policy, err := strategy.FrozenNaturalPolicyForRef(expected.policyRef())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	if expected.FounderTenure {
		return newNaturalFixtureRunnerForPolicy(expected.Patches, expected.R, expected.Seed, expected.Births, expected.Workers, policy, expected.mechanism(), expected.Founders)
	}
	return NewNatural(NaturalOptions{Patches: expected.Patches, R: expected.R, Seed: expected.Seed,
		Workers: expected.Workers, Births: expected.Births, CultivationDisabled: expected.CultivationDisabled,
		PolicyRef: policy.Ref, Mechanism: expected.mechanism(), Founders: expected.Founders})
}

// NewNaturalFixture builds the declared founder-tenure fixture world for the
// experiment lane (frozen matrix cell E and the persistence lane's
// after-births bundles). It is a declared experiment fixture — a deterministic
// ledger-coherent function of exactly (P, R, seed) plus the continuation
// flags — not a genesis variant of the ecology and not a state-injection
// point: no caller-supplied slots, tenures, or ledgers exist. The fixture
// stays run identity everywhere (journal config, checkpoint manifest,
// restore, branch) with the existing bidirectional mismatch rejection; its
// first pulse is naturalFixtureStartHour, so its first committed checkpoint
// is hour 2.
func NewNaturalFixture(patches int, r int64, seed uint64, births bool, workers int) (*Natural, error) {
	return newNaturalFixtureRunner(patches, r, seed, births, workers)
}

// NewNaturalFixtureForPolicy builds the declared founder-tenure fixture world
// under one policy ref of the executable's frozen natural ref set — the
// ticket-16 experiment-lane seam (supervisor-approved scoped exception,
// 2026-09-29, recorded in .scratch/16-surplus-behaviour/spec.md): the only
// exported genesis entry point that carries a policy ref, so the experiment
// lane can create the v6 fixture cells (matrix cell E under natural/6). It is
// a thin fail-closed wrapper over the internal path: the ref is resolved
// through strategy.FrozenNaturalPolicyForRef (the zero ref and any ref
// outside {natural/5, natural/6} are refused with a typed error — use
// NewNaturalFixture for the frozen default), the policy must be exactly the
// frozen content of that ref, and it stays a deterministic ledger-coherent
// function of exactly (P, R, seed) plus the continuation flags with no
// caller-supplied slots, tenures, or ledgers. The natural/5 ref is
// default-preserving: it builds byte-identical bundles to NewNaturalFixture
// (pinned by test). The ref remains run identity everywhere (journal config,
// checkpoint manifest, restore, branch) with the existing bidirectional
// mismatch rejection.
func NewNaturalFixtureForPolicy(patches int, r int64, seed uint64, births bool, workers int, policyRef strategy.NaturalRef) (*Natural, error) {
	return NewNaturalFixtureForMechanism(patches, r, seed, births, workers, policyRef, NaturalMechanismV5)
}

// NewNaturalFixtureForMechanism builds the declared founder-tenure fixture
// world under one policy ref of the executable's frozen natural ref set and
// one declared mechanism version — the ticket-17 experiment-lane seam
// (supervisor-approved scoped extension of the ticket-16 fixture exception,
// 2026-09-29, recorded in .scratch/17-surplus-mechanism/spec.md), so the
// experiment lane can create the matrix cell E fixture genesis on world v6.
// It is thin delegation to the internal mechanism-aware path: no state
// injection, no caller-supplied slots, tenures, or ledgers; the ref/mechanism
// legality is the frozen construction rule (naturalRefLegalUnderMechanism:
// natural/7 and natural/7b are refused under the v5 zero value), unknown or
// zero refs fail closed, and the natural/5 + v5 combination is
// default-preserving — byte-identical bundles to NewNaturalFixture and to
// NewNaturalFixtureForPolicy (pinned by test). The mechanism is run identity
// everywhere (checkpoint manifest mechanism block, journal config,
// bidirectional {5,6} mismatch rejection).
func NewNaturalFixtureForMechanism(patches int, r int64, seed uint64, births bool, workers int, policyRef strategy.NaturalRef, mechanism NaturalMechanism) (*Natural, error) {
	return NewNaturalFixtureForConfig(patches, r, seed, births, workers, policyRef, mechanism, 0)
}

// NewNaturalFixtureForConfig builds the declared founder-tenure fixture
// world under the full declared identity — one policy ref of the
// executable's frozen natural ref set, one mechanism version, and one
// founding band — the ticket-18 fixture seam of the founding-band axis (one
// general form; NewNaturalFixture, NewNaturalFixtureForPolicy and
// NewNaturalFixtureForMechanism stay delegating shims over it with the
// default band). founders follows NaturalOptions.Founders exactly: the zero
// value is the full frozen block 8P (default-preserving, byte-identical to
// the existing seams, pinned by test), otherwise 1 ≤ f ≤ 8P via
// NaturalFoundersValid, refused fail-closed otherwise — and the fixture's
// coherent pre-history seats founders 1 and 2, so f ≥ 2 (f == 0 allowed as
// the default spelling). No state injection: it stays a deterministic,
// ledger-coherent function of exactly (P, R, seed, f) plus the continuation
// flags, and the axis is run identity everywhere (checkpoint manifest
// founders field, journal config, bidirectional mismatch rejection).
func NewNaturalFixtureForConfig(patches int, r int64, seed uint64, births bool, workers int, policyRef strategy.NaturalRef, mechanism NaturalMechanism, founders int) (*Natural, error) {
	policy, err := strategy.FrozenNaturalPolicyForRef(policyRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	if !NaturalFoundersValid(patches, founders) {
		return nil, fmt.Errorf("%w: founders %d outside the derived founding-band bound", ErrNaturalCheckpoint, founders)
	}
	return newNaturalFixtureRunnerForPolicy(patches, r, seed, births, workers, policy, mechanism, founders)
}

// newNaturalFixtureRunner builds the declared founder-tenure fixture world
// under its frozen default policy identity (natural/5, the zero value of the
// D1 policy-selection seam).
func newNaturalFixtureRunner(patches int, r int64, seed uint64, births bool, workers int) (*Natural, error) {
	return newNaturalFixtureRunnerForPolicy(patches, r, seed, births, workers, strategy.FrozenNaturalPolicy(), NaturalMechanismV5, 0)
}

// newNaturalFixtureRunnerForPolicy builds the declared founder-tenure fixture
// world under one policy of the executable's frozen natural ref set — the
// fixture genesis of the declared scenario at restore time. The policy must
// be exactly the frozen content of a ref admitted for the requested
// mechanism: {natural/5, natural/6} under the v5 zero value, plus natural/7
// and natural/7b under mechanism v6 (the per-mechanism gate is
// naturalRefLegalUnderMechanism); anything else fails closed. It is a deterministic ledger-coherent function of
// exactly (P, R, seed) plus the continuation flags — not a genesis variant of
// the ecology and not a state-injection point: no caller-supplied slots,
// tenures, or ledgers exist. The fixture stays run identity everywhere
// (journal config, checkpoint manifest, restore, branch) with the existing
// bidirectional mismatch rejection; its first pulse is
// naturalFixtureStartHour, so its first committed checkpoint is hour 2.
func newNaturalFixtureRunnerForPolicy(patches int, r int64, seed uint64, births bool, workers int, policy strategy.NaturalPolicy, mechanism NaturalMechanism, founders int) (*Natural, error) {
	if !NaturalPatchCountValid(patches) || r < naturalFixtureMinBudget || r > NaturalSlotsPerPatch || workers < 1 ||
		!NaturalMechanismValid(mechanism) || !naturalRefLegalUnderMechanism(mechanism, policy.Ref) ||
		!strategy.ValidFrozenNaturalPolicy(policy) || !NaturalFoundersValid(patches, founders) ||
		(founders != 0 && founders < 2) {
		return nil, ErrNaturalCheckpoint
	}
	seeds, actors, err := naturalFixtureSeeds(mechanism, patches, r, seed, founders)
	if err != nil {
		return nil, err
	}
	// The genesis gate reads the same declared projection mirror the journal
	// reconstruction reads, so the seeds and the shadow cannot drift apart.
	patchStates, slotStates, founderStates, err := naturalGenesisProjection(patches, r, seed, true, founders, mechanism)
	if err != nil {
		return nil, err
	}
	if _, err = NaturalCheckConservationFor(mechanism, patches, patchStates, slotStates, founderStates, nil, mustNaturalGenesisLast(patches)); err != nil {
		return nil, fmt.Errorf("%w: fixture genesis breaks frozen conservation: %v", ErrNaturalCheckpoint, err)
	}
	reg, err := NaturalRegistryFor(mechanism)
	if err != nil {
		return nil, err
	}
	policies := strategy.NewNaturalRegistry(patches)
	if err = policies.RegisterFrozenNatural(policy); err != nil {
		return nil, err
	}
	allocator, err := NaturalGenesisAllocator(patches)
	if err != nil {
		return nil, err
	}
	f := &Natural{registry: reg, policies: policies, ref: policy.Ref, mechanism: mechanism, founders: naturalFoundersResolved(patches, founders), patches: patches, seed: seed, r: r, births: births,
		bound: make(map[sim.EntityID]*strategy.NaturalBound), allocator: allocator,
		ledger: newFrozenNaturalLedger(), thresholds: frozenNaturalThresholds(),
		pulseLifecycle: make(map[sim.EntityID]scheduler.Lifecycle), current: make(map[sim.EntityID]strategy.NaturalAction),
		denied: make(map[sim.EntityID]int64), deniedBuild: make(map[sim.EntityID]int64), meals: make(map[sim.EntityID]NaturalMealIntent),
		birthWakes: make(map[sim.EntityID]int64), refusals: make(map[NaturalRefusalReason]int64)}
	f.k, err = kernel.New(reg, 0, seeds)
	if err != nil {
		return nil, err
	}
	f.sched, err = scheduler.New(f.k, workers, f.evaluate)
	if err != nil {
		return nil, err
	}
	for i := range actors {
		actor := sim.EntityID(i + 1)
		bound, err := policies.Bind(strategy.NaturalBinding{Actor: actor, Ref: policy.Ref})
		if err != nil {
			return nil, err
		}
		f.bound[actor] = bound
		if err = f.sched.Register(actor); err != nil {
			return nil, err
		}
		f.roster = append(f.roster, actor)
		f.pulseLifecycle[actor] = scheduler.Alive
	}
	if err = f.sched.Register(naturalClockActor); err != nil {
		return nil, err
	}
	start, err := NaturalHourTime(naturalFixtureStartHour)
	if err != nil {
		return nil, err
	}
	if err = f.sched.Schedule(scheduler.Wake{Actor: naturalClockActor, At: start, Cause: scheduler.WakeAudit}); err != nil {
		return nil, err
	}
	return f, nil
}

// naturalRunnerTenure reports whether the runner's genesis is the declared
// founder-tenure fixture. Both genesis kinds are exact first-pulse
// properties: the neutral h0 world always commits its h0 checkpoint, the
// fixture's ledger pre-history begins at naturalFixtureStartHour, and no
// world can produce the other's first checkpoint hour.
func (f *Natural) naturalRunnerTenure() bool {
	return f != nil && len(f.checkpoints) > 0 && f.checkpoints[0].Hour == naturalFixtureStartHour
}

// naturalFixtureSeeds builds the fixture kernel seeds: the neutral
// seed-derived genesis lifted by the declared pre-history. The founding band
// rides the same axis (f founders; f ≥ 2 is a precondition of the coherent
// pre-history, which seats founders 1 and 2).
func naturalFixtureSeeds(mechanism NaturalMechanism, patches int, r int64, seed uint64, founders int) ([]component.ComponentSeed, []NaturalActorState, error) {
	if founders != 0 && founders < 2 {
		return nil, nil, fmt.Errorf("%w: founder-tenure fixture needs founders ≥ 2 (the pre-history seats founders 1 and 2), got %d", ErrNaturalCheckpoint, founders)
	}
	seeds, actors, err := naturalGenesisSeeds(mechanism, patches, r, seed, founders)
	if err != nil {
		return nil, nil, err
	}
	rewrite := func(entity sim.EntityID, componentType sim.ComponentTypeID, values map[sim.FieldID]int64, refs map[sim.FieldID]sim.EntityID) {
		for i := range seeds {
			if seeds[i].Entity != entity || seeds[i].Component != componentType {
				continue
			}
			for j := range seeds[i].Fields {
				if v, ok := values[seeds[i].Fields[j].Field]; ok {
					seeds[i].Fields[j].Value = sim.IntegerValue(v)
				}
				if ref, ok := refs[seeds[i].Fields[j].Field]; ok {
					if value, err := sim.EntityRefValue(ref); err == nil {
						seeds[i].Fields[j].Value = value
					}
				}
			}
		}
	}
	// Patch 0: two ledger-coherent pulse cycles with four cultivated slots
	// (founders 1 and 2, two each) and four wild gathers.
	patch0, err := NaturalPatchID(patches, 0)
	if err != nil {
		return nil, nil, err
	}
	unrealized := 2*r - 14
	rewrite(patch0, NaturalPatchTypeID, map[sim.FieldID]int64{
		NaturalPatchPulsesField: 2, NaturalPatchProducedField: 8,
		NaturalPatchGrantedField: 6, NaturalPatchUnrealizedField: unrealized}, nil)
	tenureOwners := []sim.EntityID{1, 1, 2, 2}
	for slotIndex, owner := range tenureOwners {
		slotID, err := NaturalSlotID(patches, 0, slotIndex)
		if err != nil {
			return nil, nil, err
		}
		ownerRef, err := sim.EntityRefValue(owner)
		if err != nil {
			return nil, nil, err
		}
		rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{
			NaturalSlotGatheredField:         2,
			NaturalSlotTenureModeField:       int64(NaturalTenureCultivated),
			NaturalSlotTenureCultivatedField: 1,
		}, map[sim.FieldID]sim.EntityID{NaturalSlotTenureOwnerField: owner})
		_ = ownerRef
	}
	// The remaining patches carry their own two-pulse idle pre-history: wild
	// slots filled, everything else unrealized.
	for p := 1; p < patches; p++ {
		idle, err := NaturalPatchID(patches, p)
		if err != nil {
			return nil, nil, err
		}
		rewrite(idle, NaturalPatchTypeID, map[sim.FieldID]int64{
			NaturalPatchPulsesField: 2, NaturalPatchProducedField: 8,
			NaturalPatchGrantedField: 0, NaturalPatchUnrealizedField: 2*r - 8}, nil)
		for slotIndex := 0; slotIndex < NaturalSlotsPerPatch; slotIndex++ {
			slotID, err := NaturalSlotID(patches, p, slotIndex)
			if err != nil {
				return nil, nil, err
			}
			rewrite(slotID, NaturalSlotTypeID, map[sim.FieldID]int64{NaturalSlotStockField: 1}, nil)
		}
	}
	// Founders 1 and 2 sit exactly at the surplus-gate margins: k=2
	// (invested 4, created 2), granary 3 (cultivated-total 3), energy 5 with
	// basal-spent 6 (the creation identity 11 = 5 + 6).
	for _, founder := range []sim.EntityID{1, 2} {
		rewrite(founder, NaturalWorksiteTypeID, map[sim.FieldID]int64{
			NaturalWorksiteCapitalField: 2, NaturalWorksiteInvestedUnitsField: 4,
			NaturalWorksitePointsCreatedField: 2}, nil)
		rewrite(founder, NaturalGranaryTypeID, map[sim.FieldID]int64{
			NaturalGranaryStockField: 3, NaturalGranaryCultivatedTotalField: 3}, nil)
		rewrite(founder, NaturalBodyTypeID, map[sim.FieldID]int64{
			NaturalBodyEnergyField: 5, NaturalBodyBasalSpentField: 6}, nil)
	}
	return seeds, actors, nil
}

// fixturePatchStates and fixtureSlotStates mirror the fixture pre-history as
// projection state; the genesis conservation gate and the journal's
// projection reconstruction both read them, so the seeds and the shadow
// cannot drift apart.
func fixturePatchStates(patches int, r int64) []NaturalPatchState {
	out := make([]NaturalPatchState, patches)
	out[0] = NaturalPatchState{Yield: r, Pulses: 2, Produced: 8, Granted: 6, Unrealized: 2*r - 14}
	for p := 1; p < patches; p++ {
		out[p] = NaturalPatchState{Yield: r, Pulses: 2, Produced: 8, Granted: 0, Unrealized: 2*r - 8}
	}
	return out
}

func fixtureSlotStates(patches int, r int64) [][]NaturalSlotState {
	out := make([][]NaturalSlotState, patches)
	first := make([]NaturalSlotState, NaturalSlotsPerPatch)
	owners := []sim.EntityID{1, 1, 2, 2}
	for i, owner := range owners {
		first[i] = NaturalSlotState{Gathered: 2, Mode: NaturalTenureCultivated, Owner: owner, Cultivated: 1}
	}
	out[0] = first
	for p := 1; p < patches; p++ {
		idle := make([]NaturalSlotState, NaturalSlotsPerPatch)
		for i := range idle {
			idle[i] = NaturalSlotState{Stock: 1}
		}
		out[p] = idle
	}
	return out
}

func mustNaturalGenesisLast(patches int) sim.EntityID {
	last, err := NaturalHighestFixedEntityID(patches)
	if err != nil {
		return 0
	}
	return last
}

// newFrozenNaturalLedger and frozenNaturalThresholds build the frozen
// compute-ceiling ledger state for reference runners. The persistence lane
// publishes bundles only at the frozen thresholds: a lowered-threshold run
// halts fail-loud before any export, and its evidence never becomes a
// bundle (the restore re-checks the frozen set exactly).
func newFrozenNaturalLedger() *naturalLedger {
	ledger, err := newNaturalLedger(nil)
	if err != nil {
		panic(err) // unreachable: nil overrides always validate
	}
	return ledger
}

func frozenNaturalThresholds() map[NaturalCeiling]uint64 {
	frozen := NaturalFrozenCeilingThresholds()
	out := make(map[NaturalCeiling]uint64, len(frozen))
	for ceiling, value := range frozen {
		out[ceiling] = value
	}
	return out
}

// journalGenesisKind is the persistence lane's view of a runner's genesis:
// the journal configuration with the genesis kind resolved from the
// committed checkpoints (see naturalRunnerTenure).
func (f *Natural) journalGenesisKind() NaturalJournalConfig {
	config := f.journalConfig()
	config.FounderTenure = f.naturalRunnerTenure()
	return config
}

type naturalManifest struct {
	patches           int
	r                 int64
	seed              uint64
	births            bool // the continuation flag: the recorded intervention on a branch child
	boundary          bool // the births flag of the shared first-hour prefix
	tenure            bool // the declared founder-tenure fixture genesis
	cultivation       bool // the declared cultivation-ablation intervention (cell D)
	steps             int
	time              sim.SimTime
	mechanism         NaturalMechanism // the declared mechanism axis: zero ≡ v5, 6 = mechanism v6
	founders          int              // the declared founding band: zero ≡ the full frozen block 8P
	policyFormat      uint32           // the declared policy identity: 5, 6, 7, 8 and 9 inside the frozen natural ref set
	head              kernel.PortableHead
	policyFingerprint [32]byte // exact frozen policy content fingerprint
	parent            [32]byte // parent bundle digest; zero only for a root capture
	parentHead        kernel.PortableHead
	depth             uint32
	sections          [4][32]byte // journal, history, scheduler, strategy

	// cognitive is the trailing omitempty cognition block (spec section 7.2);
	// nil for every non-cognitive manifest, which keeps its bytes unchanged.
	// cognitiveWire is the canonical block bytes appended after the founders
	// field.
	cognitive     *naturalCognitiveBlock
	cognitiveWire []byte
}

func naturalBundleFlag(flag bool) byte {
	if flag {
		return 1
	}
	return 0
}

// naturalManifestConstants are the pinned frozen v5 constants every manifest
// carries: world and format versions, the ID geometry, the inherited
// physiology, the genome and consent constants, the declared spoilage
// convention, the widened horizon envelopes, and the birth economics. The
// policy-format slot (naturalManifestPolicyFormatSlot) pins natural/5 here;
// encodeNaturalManifest substitutes the manifest's own declared format and
// decodeNaturalManifest accepts exactly the frozen set, so the recorded v5
// constants and the slot position stay byte-stable.
func naturalManifestConstants() ([]uint32, []int64) {
	u32s := []uint32{
		naturalCheckpointFormat, NaturalFormatVersion, NaturalJournalFormatVersion, uint32(NaturalSchemaVersion),
		NaturalRuleVersion, uint32(NaturalProjectionVersion), strategy.NaturalPolicyFormatV5,
		NaturalSlotsPerPatch, NaturalHorizonHours, NaturalMaxPatchesFixedIDSpace,
		NaturalDrawsPerBirth, uint32(NaturalTypeBlockFirst), uint32(NaturalTypeBlockLast),
		NaturalPatchIDBase, NaturalSlotIDBase, NaturalStepCeiling,
	}
	u64s := []int64{
		int64(NaturalHour), int64(NaturalGatherDuration), int64(NaturalBuildDuration),
		NaturalFounderInitialEnergy, NaturalEnergyCapacity, NaturalHungerCapacity, NaturalBagCapacity,
		NaturalPointCostWip, NaturalKMax, NaturalWipMax, NaturalSpoilagePeriod,
		NaturalBasalDebtMax, NaturalWearDebtMax, NaturalNeverHour,
		NaturalGenomeLocusMin, NaturalGenomeLocusMax, NaturalGenomeDenominator,
		NaturalLifetimeBase, NaturalLifetimeSpan,
		NaturalConsentMinGranary, NaturalConsentMinCapital, NaturalConsentMinEnergy,
		int64(NaturalKinProhibitedDepth), int64(NaturalKinWalkLimit),
		NaturalGranaryStockMax, NaturalSpoilDebtMax, NaturalSpoiledMax,
		NaturalBasalSpentMax, NaturalBirthGranaryPaidMax,
		NaturalParentGranaryCost, NaturalParentEnergyCost, NaturalNewbornEnergy,
	}
	return u32s, u64s
}

func encodeNaturalManifest(m naturalManifest) []byte {
	u32s, u64s := naturalManifestConstants()
	u32s = append([]uint32(nil), u32s...)
	u32s[naturalManifestPolicyFormatSlot] = m.policyFormat
	// The mechanism block: the four world-version slots carry the declared
	// mechanism's versions. The v5 zero value writes the frozen constants
	// verbatim, so every recorded v5 manifest keeps its bytes.
	for _, slot := range naturalManifestMechanismSlots {
		u32s[slot] = m.mechanism.worldVersion()
	}
	out := append([]byte(nil), naturalCheckpointMagic[:]...)
	for _, v := range u32s {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	for _, v := range u64s {
		out = binary.BigEndian.AppendUint64(out, uint64(v))
	}
	out = binary.BigEndian.AppendUint64(out, uint64(m.patches))
	out = binary.BigEndian.AppendUint64(out, uint64(m.r))
	out = binary.BigEndian.AppendUint64(out, m.seed)
	out = append(out, naturalBundleFlag(m.births))
	out = append(out, naturalBundleFlag(m.boundary))
	out = append(out, naturalBundleFlag(m.tenure))
	out = append(out, naturalBundleFlag(m.cultivation))
	out = binary.BigEndian.AppendUint32(out, uint32(m.steps))
	out = binary.BigEndian.AppendUint64(out, uint64(m.time))
	out = appendJournalHead(out, m.head)
	out = append(out, m.policyFingerprint[:]...)
	out = append(out, m.parent[:]...)
	out = appendJournalHead(out, m.parentHead)
	out = binary.BigEndian.AppendUint32(out, m.depth)
	for _, hash := range m.sections {
		out = append(out, hash[:]...)
	}
	// The founders field rides omitempty exactly like the mechanism block:
	// the declared default (the full frozen block 8P) writes nothing, so
	// every recorded manifest keeps its bytes and both default spellings
	// encode identically; a non-default band extends the digest-covered body
	// by exactly one u64. The cognition block (spec section 7.2) is the
	// trailing omitempty block after the founders field: absent for every
	// non-cognitive manifest.
	if m.founders != 0 {
		out = binary.BigEndian.AppendUint64(out, uint64(m.founders))
	}
	if len(m.cognitiveWire) > 0 {
		out = append(out, m.cognitiveWire...)
	}
	digest := sha256.Sum256(out)
	return append(out, digest[:]...)
}

func decodeNaturalManifest(data []byte) (naturalManifest, error) {
	var m naturalManifest
	if len(data) < sha256.Size {
		return m, ErrNaturalCheckpoint
	}
	body := data[:len(data)-sha256.Size]
	if sha256.Sum256(body) != [32]byte(data[len(data)-sha256.Size:]) {
		return m, ErrNaturalCheckpoint
	}
	// The manifest is fixed-width at the declared default; a non-default
	// founding band extends the body by exactly its omitempty u64 field and
	// the trailing cognition block extends it further. Anything shorter is a
	// truncation; exact consumption below rejects padding and trailing fields.
	manifestBodyWidth := len(encodeNaturalManifest(naturalManifest{})) - sha256.Size
	if len(body) < manifestBodyWidth {
		return m, ErrNaturalCheckpoint
	}
	r := journalReader{data: body}
	if !bytes.Equal(r.take(4), naturalCheckpointMagic[:]) {
		return m, ErrNaturalCheckpoint
	}
	u32s, u64s := naturalManifestConstants()
	formats := naturalPolicyFormats()
	mechanismSlots := make(map[int]bool, len(naturalManifestMechanismSlots))
	for _, slot := range naturalManifestMechanismSlots {
		mechanismSlots[slot] = true
	}
	for i, expected := range u32s {
		v := r.u32()
		switch {
		case i == naturalManifestMechanismWorldFormatSlot:
			// The mechanism block's world-format slot fixes the declared
			// mechanism: 5 is the frozen v5 zero value, 6 is mechanism v6,
			// 7 is mechanism v7 (ticket 20), anything else fails closed.
			if v != uint32(NaturalMechanismV5.worldVersion()) && v != uint32(NaturalMechanismV6.worldVersion()) &&
				v != uint32(NaturalMechanismV7.worldVersion()) {
				return m, ErrNaturalCheckpoint
			}
			m.mechanism = NaturalMechanismV5
			if v == uint32(NaturalMechanismV6.worldVersion()) {
				m.mechanism = NaturalMechanismV6
			}
			if v == uint32(NaturalMechanismV7.worldVersion()) {
				m.mechanism = NaturalMechanismV7
			}
			expected = v
		case mechanismSlots[i]:
			// The remaining mechanism block slots must carry the same
			// mechanism's versions exactly (no mixed-tuple manifests).
			expected = m.mechanism.worldVersion()
		case i == naturalManifestPolicyFormatSlot:
			// The parameterized policy slot: the declared policy identity.
			if !formats[v] {
				return m, ErrNaturalCheckpoint
			}
			m.policyFormat = v
			continue
		}
		if v != expected {
			return m, ErrNaturalCheckpoint
		}
	}
	for _, expected := range u64s {
		if int64(r.u64()) != expected {
			return m, ErrNaturalCheckpoint
		}
	}
	// Ref/mechanism consistency at the byte level (fail closed): the
	// provision policies exist only under mechanism >= v6, and natural/9
	// (the wage-contract policy) only under mechanism v7.
	manifestRef, err := m.naturalManifestPolicyRef()
	if err != nil || !naturalRefLegalUnderMechanism(m.mechanism, manifestRef) {
		return naturalManifest{}, ErrNaturalCheckpoint
	}
	m.patches, m.r, m.seed = int(r.u64()), int64(r.u64()), r.u64()
	for _, flag := range []*bool{&m.births, &m.boundary, &m.tenure, &m.cultivation} {
		switch byte(r.u8()) {
		case 0:
			*flag = false
		case 1:
			*flag = true
		default:
			return naturalManifest{}, ErrNaturalCheckpoint
		}
	}
	m.steps, m.time = int(r.u32()), sim.SimTime(r.u64())
	m.head = r.head()
	copy(m.policyFingerprint[:], r.take(32))
	copy(m.parent[:], r.take(32))
	m.parentHead = r.head()
	m.depth = r.u32()
	for i := range m.sections {
		copy(m.sections[i][:], r.take(32))
	}
	// The omitempty founding band: absent at the default, one u64 otherwise.
	// The trailing cognition block (spec 7.2) is self-describing via its magic,
	// so a cognitive manifest with no founders is unambiguous.
	if len(body) > manifestBodyWidth && !hasNaturalCognitiveBlock(body[manifestBodyWidth:]) {
		m.founders = int(r.u64())
	}
	if r.remaining() > 0 {
		block, consumed, err := decodeNaturalCognitiveBlock(body[len(body)-r.remaining():])
		if err != nil || consumed != r.remaining() {
			return m, ErrNaturalCheckpoint
		}
		r.take(consumed)
		m.cognitive = &block
	}
	// Boundary and lineage rules. time == 0 ↔ steps == 1: the only zero-time
	// bundle is the neutral h0 pulse capture (or a Branch child of one); any
	// later bundle is a pulse-boundary continuation. Continuation bundles
	// stand alone in the lineage: the births intervention is recorded only
	// on h0 children, so a Branch parent is always the neutral root.
	if r.bad || r.remaining() != 0 ||
		!NaturalPatchCountValid(m.patches) || m.r < 0 || m.r > NaturalSlotsPerPatch ||
		(m.tenure && m.r < naturalFixtureMinBudget) ||
		(m.cultivation && m.tenure) ||
		!NaturalFoundersValid(m.patches, m.founders) ||
		(m.tenure && m.founders == 1) ||
		m.steps < 1 || m.steps > NaturalStepCeiling ||
		m.time < 0 || m.time > sim.SimTime(NaturalHorizonHours)*sim.SimTime(NaturalHour) ||
		m.time%sim.SimTime(NaturalHour) != 0 {
		return naturalManifest{}, ErrNaturalCheckpoint
	}
	if (m.time == 0) != (m.steps == 1) {
		return naturalManifest{}, ErrNaturalCheckpoint
	}
	// The fixture's first pulse is naturalFixtureStartHour, so a fixture
	// bundle never sits at time zero (the neutral h0-only root).
	if m.tenure && m.time < sim.SimTime(naturalFixtureStartHour)*sim.SimTime(NaturalHour) {
		return naturalManifest{}, ErrNaturalCheckpoint
	}
	// A root capture is taken as-is: the recorded intervention flag equals
	// the boundary's wake topology. Only a branch child (or a continuation
	// of one, whose journal carries both flags) may record an intervention
	// that differs from the boundary.
	if m.depth == 0 && m.time == 0 && m.births != m.boundary {
		return naturalManifest{}, ErrNaturalCheckpoint
	}
	if m.time == 0 {
		if m.tenure {
			return naturalManifest{}, ErrNaturalCheckpoint
		}
		if m.depth == 0 {
			if m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) {
				return naturalManifest{}, ErrNaturalCheckpoint
			}
		} else {
			// A branch child continues its parent at the same verified boundary.
			if m.depth > naturalMaxBranchDepth || m.parent == ([32]byte{}) || m.parentHead != m.head {
				return naturalManifest{}, ErrNaturalCheckpoint
			}
		}
	} else if m.depth != 0 || m.parent != ([32]byte{}) || m.parentHead != (kernel.PortableHead{}) {
		// A pulse-boundary continuation stands alone in the lineage: the
		// births intervention is recorded only on h0 children.
		return naturalManifest{}, ErrNaturalCheckpoint
	}
	return m, nil
}

type naturalStrategyBinding struct {
	actor      sim.EntityID
	refVersion uint32
}

// decodeNaturalStrategySection splits the pinned strategy section into the
// exact frozen policy bytes and the variable-length per-entity binding
// table (entity→ref). The roster is cap-free, so the table has no declared
// record bound: its length is pinned exactly by the world's genome rows at
// verification, and its bytes by the enclosing store's section limit. Content
// is verified against an independently constructed executable and against
// the world's genome rows by verifyNaturalStrategy.
func decodeNaturalStrategySection(data []byte) (policy []byte, bindings []naturalStrategyBinding, err error) {
	r := journalReader{data: data}
	if !bytes.Equal(r.take(4), naturalStrategyMagic[:]) || r.u32() != naturalStrategyFormat {
		return nil, nil, ErrNaturalCheckpoint
	}
	policyLen := int(r.u16())
	if r.bad || policyLen < 1 || policyLen > naturalMaxStrategyPolicyBytes {
		return nil, nil, ErrNaturalCheckpoint
	}
	policy = r.take(policyLen)
	if r.bad {
		return nil, nil, ErrNaturalCheckpoint
	}
	count := int(r.u32())
	if r.bad || count < 1 || count*12 > r.remaining() {
		return nil, nil, ErrNaturalCheckpoint
	}
	bindings = make([]naturalStrategyBinding, 0, count)
	previous := sim.EntityID(0)
	for i := 0; i < count; i++ {
		b := naturalStrategyBinding{actor: sim.EntityID(r.u64()), refVersion: r.u32()}
		if r.bad || b.actor <= previous {
			return nil, nil, ErrNaturalCheckpoint
		}
		previous = b.actor
		bindings = append(bindings, b)
	}
	if r.bad || r.remaining() != 0 {
		return nil, nil, ErrNaturalCheckpoint
	}
	return policy, bindings, nil
}

// naturalGenomeRowEntities enumerates the world's genome rows at one
// verified version through the frozen registry's own identity projection:
// every actor row (founder or registered newborn) carries exactly one, and
// nothing else does.
func naturalGenomeRowEntities(view scheduler.SnapshotView) ([]sim.EntityID, error) {
	batch, err := view.Reader.Project(component.ProjectRequest{Component: NaturalGenomeTypeID,
		Projection: sim.ProjectionID(NaturalGenomeLocusMField), WorldVersion: view.Version, Authority: view.Authority})
	if err != nil {
		return nil, err
	}
	out := make([]sim.EntityID, 0, batch.Len())
	for i := 0; i < batch.Len(); i++ {
		metric, err := batch.At(i)
		if err != nil {
			return nil, err
		}
		out = append(out, metric.Provenance().Entity)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// verifyNaturalStrategy compares the section's policy content and the
// binding table against a freshly constructed executable runner and against
// the world's own genome rows: the bound entity set must be exactly the
// genome-row set (every bound entity has genome rows, and no genome row is
// left unbound) in ascending order. Founders are additionally checked
// against the fresh constructor's own bindings. A well-formed same-ref
// changed policy fails closed here, not only at content decode time.
func verifyNaturalStrategy(data []byte, fresh *Natural, view scheduler.SnapshotView) error {
	policy, bindings, err := decodeNaturalStrategySection(data)
	if err != nil {
		return err
	}
	if fresh == nil || fresh.bound[1] == nil || strategy.VerifyFrozenNaturalPolicy(policy, fresh.bound[1].Policy()) != nil {
		return ErrNaturalCheckpoint
	}
	founders := fresh.founders
	genomeRows, err := naturalGenomeRowEntities(view)
	if err != nil {
		return err
	}
	if len(bindings) != len(genomeRows) {
		return ErrNaturalCheckpoint
	}
	for i, b := range bindings {
		if b.actor != genomeRows[i] || b.refVersion != fresh.ref.Version {
			return ErrNaturalCheckpoint
		}
		if b.actor <= sim.EntityID(founders) &&
			(fresh.bound[b.actor] == nil ||
				fresh.bound[b.actor].Binding() != (strategy.NaturalBinding{Actor: b.actor, Ref: fresh.ref})) {
			return ErrNaturalCheckpoint
		}
	}
	return nil
}

// encodeNaturalStrategy pins the frozen policy content and one binding
// record per roster entity (founders first, then newborns in registration
// order — both ascending).
func encodeNaturalStrategy(f *Natural) ([]byte, error) {
	if f == nil || f.bound[1] == nil {
		return nil, ErrNaturalCheckpoint
	}
	policy := f.bound[1].Policy()
	policyBytes, err := strategy.EncodeFrozenNaturalPolicy(policy)
	if err != nil {
		return nil, ErrNaturalCheckpoint
	}
	if len(f.roster) == 0 {
		return nil, ErrNaturalCheckpoint
	}
	out := append([]byte(nil), naturalStrategyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, naturalStrategyFormat)
	out = binary.BigEndian.AppendUint16(out, uint16(len(policyBytes)))
	out = append(out, policyBytes...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(f.roster)))
	previous := sim.EntityID(0)
	for _, id := range f.roster {
		bound := f.bound[id]
		if id <= previous || bound == nil || bound.Policy() != policy ||
			bound.Binding() != (strategy.NaturalBinding{Actor: id, Ref: f.ref}) {
			return nil, ErrNaturalCheckpoint
		}
		previous = id
		out = binary.BigEndian.AppendUint64(out, uint64(id))
		out = binary.BigEndian.AppendUint32(out, bound.Binding().Ref.Version)
	}
	return out, nil
}

// naturalExpectedBoundaryWakes builds the exact wake set at a pulse
// boundary: one claim wake per living actor — the reserved WakeBirth class
// for a newborn whose first claim is still pending, a NeedThreshold claim
// otherwise — the birth-request audit wake when births are enabled, and the
// clock's next pulse. The frozen WakeBirth class is part of every
// expectation set, so a missing, extra, or mis-caused reservation fails the
// exact match. At the final horizon boundary the pulse schedules nothing and
// the exact expectation set is empty.
func naturalExpectedBoundaryWakes(births, labourLane bool, hour int, living []sim.EntityID, reserved map[sim.EntityID]int64) (map[scheduler.Wake]bool, error) {
	// At the final horizon boundary the pulse schedules nothing, so the exact
	// expectation set is empty. NaturalPhaseTime refuses h >= NaturalHorizonHours
	// (the horizon hour has no claim instant), so this must return before the
	// claim lookup rather than surface a raw contract error.
	if hour == NaturalHorizonHours {
		return map[scheduler.Wake]bool{}, nil
	}
	claim, err := NaturalPhaseTime(hour, NaturalPhaseClaim)
	if err != nil {
		return nil, err
	}
	// The shared negotiation-filing instant carries an audit wake when
	// either lane can file at it: births enabled, or the wage-contract lane
	// (ticket 20, natural/9 evaluates its rule-516 filing there even with
	// births off). The decision and settle wakes are scheduled mid-hour and
	// consumed mid-hour, so they are never pending at a pulse boundary.
	var requestAt sim.SimTime
	if births || labourLane {
		requestAt, err = NaturalPhaseTime(hour, NaturalPhaseBirthRequest)
		if err != nil {
			return nil, err
		}
	}
	expected := make(map[scheduler.Wake]bool, len(living)+2)
	for _, id := range living {
		if reserved[id] != 0 {
			// A reserved first wake is always the current hour's claim; any
			// other reservation would be a wake-topology violation.
			if reserved[id] != int64(hour) {
				return nil, ErrNaturalCheckpoint
			}
			expected[scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeBirth}] = true
		} else {
			expected[scheduler.Wake{Actor: id, At: claim, Cause: scheduler.WakeNeedThreshold}] = true
		}
		if births || labourLane {
			expected[scheduler.Wake{Actor: id, At: requestAt, Cause: scheduler.WakeAudit}] = true
		}
	}
	if hour < NaturalHorizonHours {
		next, err := NaturalHourTime(hour + 1)
		if err != nil {
			return nil, err
		}
		expected[scheduler.Wake{Actor: naturalClockActor, At: next, Cause: scheduler.WakeAudit}] = true
	}
	return expected, nil
}

// verifyNaturalBoundaryTopology proves a pulse-boundary portable state is
// exactly the runner's own clock protocol: one idle fiber per roster entity
// with the frozen alive invariant, plus the clock fiber, and the exact
// expected wake set with no missing or extra wakes (no supersets).
func verifyNaturalBoundaryTopology(snap scheduler.Snapshot, births, labourLane bool, hour int, roster []sim.EntityID, lifecycle map[sim.EntityID]scheduler.Lifecycle, reserved map[sim.EntityID]int64) error {
	if snap.Time != sim.SimTime(hour)*sim.SimTime(NaturalHour) || len(snap.Fibers) != len(roster)+1 {
		return ErrNaturalCheckpoint
	}
	byActor := make(map[sim.EntityID]scheduler.Fiber, len(snap.Fibers))
	for _, fiber := range snap.Fibers {
		byActor[fiber.Actor] = fiber
	}
	if _, ok := byActor[naturalClockActor]; !ok {
		return ErrNaturalCheckpoint
	}
	living := make([]sim.EntityID, 0, len(roster))
	for _, id := range roster {
		fiber, ok := byActor[id]
		if !ok || fiber.Activity != nil || fiber.Lifecycle != lifecycle[id] {
			return ErrNaturalCheckpoint
		}
		if lifecycle[id] == scheduler.Alive {
			living = append(living, id)
		}
	}
	expected, err := naturalExpectedBoundaryWakes(births, labourLane, hour, living, reserved)
	if err != nil {
		return err
	}
	if len(expected) != len(snap.Wakes) {
		return ErrNaturalCheckpoint
	}
	for _, wake := range snap.Wakes {
		if !expected[wake] {
			return ErrNaturalCheckpoint
		}
		delete(expected, wake)
	}
	if len(expected) != 0 {
		return ErrNaturalCheckpoint
	}
	return nil
}

// verifyNaturalNeutralScheduler proves the portable queue is exactly the
// wake set produced by the runner's own neutral h0 clock protocol: one
// committed pulse batch with no claim, capital, or birth evidence, no open
// activities, every founder fiber alive at registration revision, the clock
// fiber armed for the next pulse, one claim wake per founder, the
// births-enabled request wakes exactly when the flag demands them, and no
// WakeBirth reservation (no newborn exists at h0).
func verifyNaturalNeutralScheduler(k *kernel.Kernel, journal []NaturalBatch, snap scheduler.Snapshot, births, labourLane bool, patches, founders int) error {
	if k == nil || len(journal) != 1 || len(journal[0].Attempts) != 0 || journal[0].Time != 0 {
		return ErrNaturalCheckpoint
	}
	if snap.Time != 0 || !snap.HasClosed || snap.Closed != 0 || snap.NextToken != 0 {
		return ErrNaturalCheckpoint
	}
	if founders < 1 || founders > NaturalSlotsPerPatch*patches || len(snap.Fibers) != founders+1 {
		return ErrNaturalCheckpoint
	}
	roster := make([]sim.EntityID, 0, founders)
	lifecycle := make(map[sim.EntityID]scheduler.Lifecycle, founders)
	for actor := sim.EntityID(1); actor <= sim.EntityID(founders); actor++ {
		roster = append(roster, actor)
		lifecycle[actor] = scheduler.Alive
	}
	if err := verifyNaturalBoundaryTopology(snap, births, labourLane, 0, roster, lifecycle, nil); err != nil {
		return err
	}
	for _, fiber := range snap.Fibers {
		if fiber.Revision != 1 {
			return ErrNaturalCheckpoint
		}
	}
	return nil
}

// naturalBoundsTable compiles gate G7's declared field bounds from one
// mechanism's registry. Integer fields only; ref fields carry no bounds.
type naturalBoundsTable map[sim.ComponentTypeID]map[sim.FieldID][2]int64

func naturalBoundsTableForRegistry(mechanism NaturalMechanism) (naturalBoundsTable, error) {
	reg, err := NaturalRegistryFor(mechanism)
	if err != nil {
		return nil, err
	}
	out := make(naturalBoundsTable)
	for _, typ := range reg.TypeIDs() {
		descriptor, err := reg.Describe(typ)
		if err != nil {
			return nil, err
		}
		fields := make(map[sim.FieldID][2]int64)
		for _, f := range descriptor.Fields {
			if f.Type.Kind != sim.IntegerKind || !f.Bounds.HasMinimum || !f.Bounds.HasMaximum {
				continue
			}
			fields[f.ID] = [2]int64{int64(f.Bounds.Minimum), int64(f.Bounds.Maximum)}
		}
		out[typ] = fields
	}
	return out, nil
}

// naturalProjectionBounds checks gate G7 on one reconstructed hourly
// projection: every integer field of every patch, slot, and actor row
// (founder or newborn, genome rows included) sits inside its declared
// descriptor bounds. The mechanism selects the granary field list: the v5
// zero value checks the frozen six fields; mechanism v6 additionally checks
// the provisioned ledger against its declared [0,168] envelope.
func naturalProjectionBounds(check NaturalCheckpoint, table naturalBoundsTable, mechanism NaturalMechanism) error {
	within := func(typ sim.ComponentTypeID, field sim.FieldID, value int64) error {
		bounds, ok := table[typ][field]
		if !ok || value < bounds[0] || value > bounds[1] {
			return fmt.Errorf("%w: component %d field %d value %d outside declared bounds", ErrNaturalCheckpoint, typ, field, value)
		}
		return nil
	}
	for i, p := range check.Patches {
		if _, err := NaturalPatchID(len(check.Patches), i); err != nil {
			return err
		}
		for field, value := range map[sim.FieldID]int64{
			NaturalPatchYieldField: p.Yield, NaturalPatchPulsesField: p.Pulses, NaturalPatchProducedField: p.Produced,
			NaturalPatchGrantedField: p.Granted, NaturalPatchUnrealizedField: p.Unrealized} {
			if err := within(NaturalPatchTypeID, field, value); err != nil {
				return err
			}
		}
		for j, s := range check.Slots[i] {
			if _, err := NaturalSlotID(len(check.Patches), i, j); err != nil {
				return err
			}
			for _, spec := range []struct {
				field sim.FieldID
				value int64
			}{{NaturalSlotStockField, s.Stock}, {NaturalSlotGatheredField, s.Gathered},
				{NaturalSlotTenureModeField, int64(s.Mode)}, {NaturalSlotTenureCultivatedField, s.Cultivated}} {
				if err := within(NaturalSlotTypeID, spec.field, spec.value); err != nil {
					return err
				}
			}
		}
	}
	actors := make([]NaturalNewbornState, 0, len(check.Founders)+len(check.Newborns))
	for i := range check.Founders {
		actors = append(actors, NaturalNewbornState{ID: sim.EntityID(i + 1), State: check.Founders[i]})
	}
	actors = append(actors, check.Newborns...)
	for _, entry := range actors {
		a := entry.State
		fields := []struct {
			typ   sim.ComponentTypeID
			field sim.FieldID
			value int64
		}{
			{NaturalBagTypeID, NaturalBagUnitsField, a.Bag.Units},
			{NaturalBodyTypeID, NaturalBodyEnergyField, a.Body.Energy},
			{NaturalBodyTypeID, NaturalBodyHungerField, a.Body.Hunger},
			{NaturalBodyTypeID, NaturalBodyBasalSpentField, a.Body.BasalSpent},
			{NaturalBodyTypeID, NaturalBodyCapLostField, a.Body.CapLost},
			{NaturalBodyTypeID, NaturalBodyConsumedField, a.Body.Consumed},
			{NaturalBodyTypeID, NaturalBodyLastGatherHourField, a.Body.LastGatherHour},
			{NaturalBodyTypeID, NaturalBodyBasalDebtField, a.Body.BasalDebt},
			{NaturalWorksiteTypeID, NaturalWorksiteCapitalField, a.Worksite.Capital},
			{NaturalWorksiteTypeID, NaturalWorksiteWipField, a.Worksite.Wip},
			{NaturalWorksiteTypeID, NaturalWorksiteWearDebtField, a.Worksite.WearDebt},
			{NaturalWorksiteTypeID, NaturalWorksiteInvestedUnitsField, a.Worksite.InvestedUnits},
			{NaturalWorksiteTypeID, NaturalWorksitePointsCreatedField, a.Worksite.PointsCreated},
			{NaturalWorksiteTypeID, NaturalWorksitePointsDecayedField, a.Worksite.PointsDecayed},
			{NaturalWorksiteTypeID, NaturalWorksiteLastBuildHourField, a.Worksite.LastBuildHour},
			{NaturalGranaryTypeID, NaturalGranaryStockField, a.Granary.Stock},
			{NaturalGranaryTypeID, NaturalGranaryCultivatedTotalField, a.Granary.CultivatedTotal},
			{NaturalGranaryTypeID, NaturalGranaryStoredMealsField, a.Granary.StoredMeals},
			{NaturalGranaryTypeID, NaturalGranaryLastStoredMealHourField, a.Granary.LastStoredMealHour},
			{NaturalGranaryTypeID, NaturalGranarySpoilDebtField, a.Granary.SpoilDebt},
			{NaturalGranaryTypeID, NaturalGranarySpoiledField, a.Granary.Spoiled},
			{NaturalGenomeTypeID, NaturalGenomeLocusMField, a.Genome.LocusM},
			// (The mechanism-v6 provisioned row is appended below.)
			{NaturalGenomeTypeID, NaturalGenomeLocusYField, a.Genome.LocusY},
			{NaturalGenomeTypeID, NaturalGenomeLocusWField, a.Genome.LocusW},
			{NaturalGenomeTypeID, NaturalGenomeBirthHourField, a.Genome.BirthHour},
			{NaturalGenomeTypeID, NaturalGenomeDeathHourField, a.Genome.DeathHour},
			{NaturalGenomeTypeID, NaturalGenomeDiedHourField, a.Genome.DiedHour},
			{NaturalGenomeTypeID, NaturalGenomeBirthGranaryPaidField, a.Genome.BirthGranaryPaid},
			{NaturalBirthRequestTypeID, NaturalBirthRequestHourField, a.Request.Hour},
			{NaturalBirthRequestTypeID, NaturalBirthRequestReportedCapitalField, a.Request.ReportedCapital},
			{NaturalBirthRequestTypeID, NaturalBirthRequestReportedGranaryField, a.Request.ReportedGranary},
			{NaturalBirthRequestTypeID, NaturalBirthRequestStatusField, int64(a.Request.Status)},
			{NaturalBirthRequestTypeID, NaturalBirthRequestLastRequestHourField, a.Request.LastRequestHour},
			{NaturalBirthRequestTypeID, NaturalBirthRequestLastReplyHourField, a.Request.LastReplyHour},
		}
		if mechanism == NaturalMechanismV6 {
			// The provisioned ledger exists only under mechanism v6 and is
			// checked against its declared envelope; the v5 zero value keeps
			// exactly the frozen field set (its G2a pins the counter to zero
			// and the frozen bounds table has no field 7).
			fields = append(fields, struct {
				typ   sim.ComponentTypeID
				field sim.FieldID
				value int64
			}{NaturalGranaryTypeID, NaturalGranaryProvisionedField, a.Granary.Provisioned})
		}
		for _, spec := range fields {
			if err := within(spec.typ, spec.field, spec.value); err != nil {
				return fmt.Errorf("actor %d: %w", entry.ID, err)
			}
		}
	}
	return nil
}

// naturalProjection is the shadow state of the hourly projection trace,
// rebuilt from verified accepted deltas rather than a second mutable ledger.
type naturalProjection struct {
	patches  []NaturalPatchState
	slots    [][]NaturalSlotState
	founders []NaturalActorState
	newborns []NaturalNewbornState
	deaths   struct {
		age        int64
		starvation int64
	}
	birthRequests, divergences int64
	refusals                   map[NaturalRefusalReason]int64
	staging                    map[sim.EntityID]*NaturalActorState // newborn rows of the entity-create event in flight
}

func naturalDeltaRef(v sim.Value) (sim.EntityID, error) {
	if v.State() == sim.Missing {
		return 0, nil
	}
	return v.EntityRef()
}

// knownEntity reports whether the entity's rows pre-exist in the shadow
// projection: the patches, their slots, the founders, and every newborn
// registered so far.
func (p *naturalProjection) knownEntity(patches int, id sim.EntityID) bool {
	switch {
	case naturalPatchEntity(patches, id),
		naturalSlotEntity(patches, id),
		naturalFounderEntity(patches, id):
		return true
	}
	_, ok := p.newbornIndex(id)
	return ok
}

func (p *naturalProjection) actor(id sim.EntityID) (*NaturalActorState, error) {
	if naturalFounderEntity(len(p.patches), id) {
		return &p.founders[id-1], nil
	}
	for i := range p.newborns {
		if p.newborns[i].ID == id {
			return &p.newborns[i].State, nil
		}
	}
	return nil, ErrNaturalCheckpoint
}

// applyPatchDelta mirrors every authorized v5 event delta into the shadow
// projection state. Unlisted component/field/rule combinations are rejected
// instead of ignored, so an unexpected accepted delta cannot pass silently;
// the caller's final authoritative-state comparison catches any remaining
// value divergence. Genome writes are restricted to the frozen ownership
// surface: died-hour only by rules 504/514 (attributing the death cause)
// and birth-granary-paid only by rule 511. Slot tenure is written only by
// rules 503 (wear revert) and 505 (build claim); the tenure admission of
// rule 514 is a frozen dead allowance with no v5 flow, so a tenure write
// under it is rejected here — the accepted event partition is the authority.
func (p *naturalProjection) applyPatchDelta(mechanism NaturalMechanism, patches int, rule sim.RuleID, d component.FieldDelta) error {
	integer := func() (int64, error) { return d.After.Integer() }
	switch d.Component {
	case NaturalPatchTypeID:
		if !naturalPatchEntity(patches, d.Entity) || rule != NaturalRegenSplitRule {
			return ErrNaturalCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		v := &p.patches[d.Entity-NaturalPatchIDBase]
		switch d.Field {
		case NaturalPatchPulsesField:
			v.Pulses = n
		case NaturalPatchProducedField:
			v.Produced = n
		case NaturalPatchGrantedField:
			v.Granted = n
		case NaturalPatchUnrealizedField:
			v.Unrealized = n
		default:
			return ErrNaturalCheckpoint
		}
	case NaturalSlotTypeID:
		if !naturalSlotEntity(patches, d.Entity) {
			return ErrNaturalCheckpoint
		}
		index := int(d.Entity - NaturalSlotIDBase)
		v := &p.slots[index/NaturalSlotsPerPatch][index%NaturalSlotsPerPatch]
		switch d.Field {
		case NaturalSlotStockField:
			if rule != NaturalRegenSplitRule && rule != NaturalGatherRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			v.Stock = n
		case NaturalSlotGatheredField:
			if rule != NaturalGatherRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			v.Gathered = n
		case NaturalSlotTenureModeField:
			if rule != NaturalWearRule && rule != NaturalBuildRule && !(rule == NaturalLabourSettlementRule && mechanism == NaturalMechanismV7) {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			v.Mode = NaturalTenureMode(n)
		case NaturalSlotTenureOwnerField:
			if rule != NaturalWearRule && rule != NaturalBuildRule && !(rule == NaturalLabourSettlementRule && mechanism == NaturalMechanismV7) {
				return ErrNaturalCheckpoint
			}
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			v.Owner = id
		case NaturalSlotTenureCultivatedField:
			if rule != NaturalWearRule && rule != NaturalBuildRule && !(rule == NaturalLabourSettlementRule && mechanism == NaturalMechanismV7) {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			v.Cultivated = n
		default:
			return ErrNaturalCheckpoint
		}
	case NaturalBagTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalBagUnitsField:
			// Rule 515 (mechanism v6 only) and rule 518's worker delivery
			// (mechanism v7 only) are the draw rules that write the bag besides
			// gather/consume/build/birth: the provisioned unit enters the bag at
			// the claim instant, the wage delivery empties it at the settle
			// instant. The frozen v5 writer set refuses both, so the shadow
			// refuses them there too.
			switch rule {
			case NaturalGatherRule, NaturalConsumeRule, NaturalBuildRule, NaturalBirthReplyRule:
			case NaturalProvisionRule:
				if mechanism != NaturalMechanismV6 {
					return ErrNaturalCheckpoint
				}
			case NaturalLabourSettlementRule:
				if mechanism != NaturalMechanismV7 {
					return ErrNaturalCheckpoint
				}
			default:
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Bag.Units = n
		default:
			// The bag source row is the actor's home patch; only the newborn
			// allocation writes it (rule 511), never a patch.
			return ErrNaturalCheckpoint
		}
	case NaturalBodyTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalBodyEnergyField:
			a.Body.Energy = n
		case NaturalBodyHungerField:
			a.Body.Hunger = n
		case NaturalBodyBasalSpentField:
			a.Body.BasalSpent = n
		case NaturalBodyCapLostField:
			a.Body.CapLost = n
		case NaturalBodyConsumedField:
			a.Body.Consumed = n
		case NaturalBodyLastGatherHourField:
			a.Body.LastGatherHour = n
		case NaturalBodyBasalDebtField:
			a.Body.BasalDebt = n
		default:
			return ErrNaturalCheckpoint
		}
	case NaturalWorksiteTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		// Rule 503 wear and rule 505 build; mechanism v7 adds rule 518's
		// enumerated settlement writes (invested/wip, plus capital and
		// points-created when the wage completes a point) — the one sanctioned
		// non-builder worksite writer, ticket 20.
		if rule != NaturalWearRule && rule != NaturalBuildRule && !(rule == NaturalLabourSettlementRule && mechanism == NaturalMechanismV7) {
			return ErrNaturalCheckpoint
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalWorksiteCapitalField:
			a.Worksite.Capital = n
		case NaturalWorksiteWipField:
			a.Worksite.Wip = n
		case NaturalWorksiteWearDebtField:
			a.Worksite.WearDebt = n
		case NaturalWorksiteInvestedUnitsField:
			a.Worksite.InvestedUnits = n
		case NaturalWorksitePointsCreatedField:
			a.Worksite.PointsCreated = n
		case NaturalWorksitePointsDecayedField:
			a.Worksite.PointsDecayed = n
		case NaturalWorksiteLastBuildHourField:
			a.Worksite.LastBuildHour = n
		default:
			return ErrNaturalCheckpoint
		}
	case NaturalGranaryTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalGranaryStockField:
			switch rule {
			case NaturalSpoilageRule, NaturalGrantRule, NaturalEatStoredRule, NaturalBirthReplyRule:
			case NaturalProvisionRule:
				if mechanism != NaturalMechanismV6 {
					return ErrNaturalCheckpoint
				}
			case NaturalLabourSettlementRule:
				// The wage settlement moves one stock unit from the employer's
				// granary to the worker's (mechanism v7 only, ticket 20).
				if mechanism != NaturalMechanismV7 {
					return ErrNaturalCheckpoint
				}
			default:
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.Stock = n
		case NaturalGranaryCultivatedTotalField:
			if rule != NaturalGrantRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.CultivatedTotal = n
		case NaturalGranaryStoredMealsField:
			if rule != NaturalEatStoredRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.StoredMeals = n
		case NaturalGranaryProvisionedField:
			// The rule-515 withdrawal ledger: only the provision rule writes
			// it, and only under mechanism v6 (the frozen v5 granary has no
			// field 7, so a v5 replay refuses any write here).
			if rule != NaturalProvisionRule || mechanism != NaturalMechanismV6 {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.Provisioned = n
		case NaturalGranaryWagesPaidField, NaturalGranaryWagesEarnedField:
			// The rule-518 wage ledgers (ticket 20): only the settlement rule
			// writes them, only under mechanism v7 (the v5/v6 granaries have no
			// fields 8 or 9, so those replays refuse any write here).
			if rule != NaturalLabourSettlementRule || mechanism != NaturalMechanismV7 {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			if d.Field == NaturalGranaryWagesPaidField {
				a.Granary.WagesPaid = n
			} else {
				a.Granary.WagesEarned = n
			}
		case NaturalGranaryLastStoredMealHourField:
			// Rule 509's stored meal and rule 515's provision share the
			// once-per-actor-hour draw guard; the hour field is written by
			// both, the provision only under mechanism v6.
			switch rule {
			case NaturalEatStoredRule:
			case NaturalProvisionRule:
				if mechanism != NaturalMechanismV6 {
					return ErrNaturalCheckpoint
				}
			default:
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.LastStoredMealHour = n
		case NaturalGranarySpoilDebtField:
			if rule != NaturalSpoilageRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.SpoilDebt = n
		case NaturalGranarySpoiledField:
			if rule != NaturalSpoilageRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Granary.Spoiled = n
		default:
			return ErrNaturalCheckpoint
		}
	case NaturalGenomeTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		n, err := integer()
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalGenomeDiedHourField:
			switch rule {
			case NaturalBasalRule:
				p.deaths.starvation++
			case NaturalAgeDeathRule:
				p.deaths.age++
			default:
				return ErrNaturalCheckpoint
			}
			a.Genome.DiedHour = n
		case NaturalGenomeBirthGranaryPaidField:
			if rule != NaturalBirthReplyRule {
				return ErrNaturalCheckpoint
			}
			a.Genome.BirthGranaryPaid = n
		default:
			// Loci, birth-hour, death-hour, and parent refs are immutable
			// post-creation; only rule 511's allocation writes them (on the
			// newborn), never a patch.
			return ErrNaturalCheckpoint
		}
	case NaturalBirthRequestTypeID:
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalBirthRequestAddresseeField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			a.Request.Addressee = id
		default:
			n, err := integer()
			if err != nil {
				return err
			}
			switch d.Field {
			case NaturalBirthRequestHourField:
				a.Request.Hour = n
			case NaturalBirthRequestReportedCapitalField:
				a.Request.ReportedCapital = n
			case NaturalBirthRequestReportedGranaryField:
				a.Request.ReportedGranary = n
			case NaturalBirthRequestStatusField:
				a.Request.Status = NaturalRequestStatus(n)
			case NaturalBirthRequestLastRequestHourField:
				a.Request.LastRequestHour = n
			case NaturalBirthRequestLastReplyHourField:
				a.Request.LastReplyHour = n
			default:
				return ErrNaturalCheckpoint
			}
		}
	case NaturalLabourOfferTypeID:
		// The wage-contract offer row (ticket 20, mechanism v7 only): rule
		// 516 files the four filing fields, rule 517 writes the enumerated
		// decision fields, rule 519 expires and clears (status only) — the
		// registry's G5 writer table mirrored exactly.
		if mechanism != NaturalMechanismV7 {
			return ErrNaturalCheckpoint
		}
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalLabourOfferHourField, NaturalLabourOfferLastOfferHourField:
			if rule != NaturalLabourOfferRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			if d.Field == NaturalLabourOfferHourField {
				a.Offer.Hour = n
			} else {
				a.Offer.LastOfferHour = n
			}
		case NaturalLabourOfferAddresseeField:
			if rule != NaturalLabourOfferRule {
				return ErrNaturalCheckpoint
			}
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			a.Offer.Addressee = id
		case NaturalLabourOfferStatusField:
			if rule != NaturalLabourOfferRule && rule != NaturalLabourDecisionRule && rule != NaturalLabourExpireRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Offer.Status = NaturalRequestStatus(n)
		case NaturalLabourOfferLastReplyHourField:
			if rule != NaturalLabourDecisionRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Offer.LastReplyHour = n
		default:
			return ErrNaturalCheckpoint
		}
	case NaturalLabourBookTypeID:
		// The employer engagement row (ticket 20, mechanism v7 only): rule
		// 517 writes the engagement slot and the decision audit, rule 519
		// clears the engagement fields (the audit persists).
		if mechanism != NaturalMechanismV7 {
			return ErrNaturalCheckpoint
		}
		a, err := p.actor(d.Entity)
		if err != nil {
			return err
		}
		switch d.Field {
		case NaturalLabourBookEngagedWorkerField:
			if rule != NaturalLabourDecisionRule && rule != NaturalLabourExpireRule {
				return ErrNaturalCheckpoint
			}
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			a.Book.EngagedWorker = id
		case NaturalLabourBookEngagedHourField:
			if rule != NaturalLabourDecisionRule && rule != NaturalLabourExpireRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Book.EngagedHour = n
		case NaturalLabourBookLastDecisionHourField:
			if rule != NaturalLabourDecisionRule {
				return ErrNaturalCheckpoint
			}
			n, err := integer()
			if err != nil {
				return err
			}
			a.Book.LastDecisionHour = n
		default:
			return ErrNaturalCheckpoint
		}
	default:
		return ErrNaturalCheckpoint
	}
	return nil
}

// applyAllocationDelta mirrors one entity-create delta: a complete newborn
// row written absent-before. The assembled newborn is validated and appended
// by finishBirth. Under mechanism v7 the newborn additionally allocates its
// nine-field granary (the wage ledgers, seeded 0) and its two idle
// wage-contract rows.
func (p *naturalProjection) applyAllocationDelta(mechanism NaturalMechanism, patches int, d component.FieldDelta) error {
	if !naturalNewbornEntity(patches, d.Entity) {
		return ErrNaturalCheckpoint
	}
	if _, exists := p.newbornIndex(d.Entity); exists {
		return ErrNaturalCheckpoint // an existing row is never allocated twice
	}
	state, ok := p.staging[d.Entity]
	if !ok {
		if len(p.staging) >= 1 {
			return ErrNaturalCheckpoint // one entity-create event allocates exactly one newborn
		}
		if p.staging == nil {
			p.staging = make(map[sim.EntityID]*NaturalActorState)
		}
		state = &NaturalActorState{}
		p.staging[d.Entity] = state
	}
	integer := func() (int64, error) { return d.After.Integer() }
	set := func(target *int64) error {
		n, err := integer()
		if err != nil {
			return err
		}
		*target = n
		return nil
	}
	switch d.Component {
	case NaturalBagTypeID:
		switch d.Field {
		case NaturalBagUnitsField:
			return set(&state.Bag.Units)
		case NaturalBagSourceField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Bag.Source = id
			return nil
		}
	case NaturalBodyTypeID:
		switch d.Field {
		case NaturalBodyEnergyField:
			return set(&state.Body.Energy)
		case NaturalBodyHungerField:
			return set(&state.Body.Hunger)
		case NaturalBodyBasalSpentField:
			return set(&state.Body.BasalSpent)
		case NaturalBodyCapLostField:
			return set(&state.Body.CapLost)
		case NaturalBodyConsumedField:
			return set(&state.Body.Consumed)
		case NaturalBodyLastGatherHourField:
			return set(&state.Body.LastGatherHour)
		case NaturalBodyBasalDebtField:
			return set(&state.Body.BasalDebt)
		}
	case NaturalWorksiteTypeID:
		switch d.Field {
		case NaturalWorksiteCapitalField:
			return set(&state.Worksite.Capital)
		case NaturalWorksiteWipField:
			return set(&state.Worksite.Wip)
		case NaturalWorksiteWearDebtField:
			return set(&state.Worksite.WearDebt)
		case NaturalWorksiteInvestedUnitsField:
			return set(&state.Worksite.InvestedUnits)
		case NaturalWorksitePointsCreatedField:
			return set(&state.Worksite.PointsCreated)
		case NaturalWorksitePointsDecayedField:
			return set(&state.Worksite.PointsDecayed)
		case NaturalWorksiteLastBuildHourField:
			return set(&state.Worksite.LastBuildHour)
		}
	case NaturalGranaryTypeID:
		switch d.Field {
		case NaturalGranaryStockField:
			return set(&state.Granary.Stock)
		case NaturalGranaryCultivatedTotalField:
			return set(&state.Granary.CultivatedTotal)
		case NaturalGranaryStoredMealsField:
			return set(&state.Granary.StoredMeals)
		case NaturalGranaryProvisionedField:
			// Mechanism v6 and v7: every newborn granary row carries the
			// provisioned ledger (seeded 0); the v5 allocation never does.
			if mechanism != NaturalMechanismV6 && mechanism != NaturalMechanismV7 {
				return ErrNaturalCheckpoint
			}
			return set(&state.Granary.Provisioned)
		case NaturalGranaryWagesPaidField:
			// Mechanism v7 only (ticket 20): the wage ledgers are seeded 0.
			if mechanism != NaturalMechanismV7 {
				return ErrNaturalCheckpoint
			}
			return set(&state.Granary.WagesPaid)
		case NaturalGranaryWagesEarnedField:
			if mechanism != NaturalMechanismV7 {
				return ErrNaturalCheckpoint
			}
			return set(&state.Granary.WagesEarned)
		case NaturalGranaryLastStoredMealHourField:
			return set(&state.Granary.LastStoredMealHour)
		case NaturalGranarySpoilDebtField:
			return set(&state.Granary.SpoilDebt)
		case NaturalGranarySpoiledField:
			return set(&state.Granary.Spoiled)
		}
	case NaturalGenomeTypeID:
		switch d.Field {
		case NaturalGenomeLocusMField:
			return set(&state.Genome.LocusM)
		case NaturalGenomeLocusYField:
			return set(&state.Genome.LocusY)
		case NaturalGenomeLocusWField:
			return set(&state.Genome.LocusW)
		case NaturalGenomeParentAField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Genome.ParentA = id
			return nil
		case NaturalGenomeParentBField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Genome.ParentB = id
			return nil
		case NaturalGenomeBirthHourField:
			return set(&state.Genome.BirthHour)
		case NaturalGenomeDeathHourField:
			return set(&state.Genome.DeathHour)
		case NaturalGenomeDiedHourField:
			return set(&state.Genome.DiedHour)
		case NaturalGenomeBirthGranaryPaidField:
			return set(&state.Genome.BirthGranaryPaid)
		}
	case NaturalBirthRequestTypeID:
		switch d.Field {
		case NaturalBirthRequestHourField:
			return set(&state.Request.Hour)
		case NaturalBirthRequestAddresseeField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Request.Addressee = id
			return nil
		case NaturalBirthRequestReportedCapitalField:
			return set(&state.Request.ReportedCapital)
		case NaturalBirthRequestReportedGranaryField:
			return set(&state.Request.ReportedGranary)
		case NaturalBirthRequestStatusField:
			n, err := integer()
			if err != nil {
				return err
			}
			state.Request.Status = NaturalRequestStatus(n)
			return nil
		case NaturalBirthRequestLastRequestHourField:
			return set(&state.Request.LastRequestHour)
		case NaturalBirthRequestLastReplyHourField:
			return set(&state.Request.LastReplyHour)
		}
	case NaturalLabourOfferTypeID:
		// Mechanism v7 only (ticket 20): the newborn's idle offer row.
		if mechanism != NaturalMechanismV7 {
			return ErrNaturalCheckpoint
		}
		switch d.Field {
		case NaturalLabourOfferHourField:
			return set(&state.Offer.Hour)
		case NaturalLabourOfferAddresseeField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Offer.Addressee = id
			return nil
		case NaturalLabourOfferStatusField:
			n, err := integer()
			if err != nil {
				return err
			}
			state.Offer.Status = NaturalRequestStatus(n)
			return nil
		case NaturalLabourOfferLastOfferHourField:
			return set(&state.Offer.LastOfferHour)
		case NaturalLabourOfferLastReplyHourField:
			return set(&state.Offer.LastReplyHour)
		}
	case NaturalLabourBookTypeID:
		// Mechanism v7 only (ticket 20): the newborn's idle book row.
		if mechanism != NaturalMechanismV7 {
			return ErrNaturalCheckpoint
		}
		switch d.Field {
		case NaturalLabourBookEngagedWorkerField:
			id, err := naturalDeltaRef(d.After)
			if err != nil {
				return err
			}
			state.Book.EngagedWorker = id
			return nil
		case NaturalLabourBookEngagedHourField:
			return set(&state.Book.EngagedHour)
		case NaturalLabourBookLastDecisionHourField:
			return set(&state.Book.LastDecisionHour)
		}
	default:
		return ErrNaturalCheckpoint
	}
	return ErrNaturalCheckpoint
}

func (p *naturalProjection) newbornIndex(id sim.EntityID) (int, bool) {
	for i := range p.newborns {
		if p.newborns[i].ID == id {
			return i, true
		}
	}
	return 0, false
}

// finishBirth validates the staged newborn of one entity-create event
// against the allocator-derived next-ID rule (genesis base plus committed
// births — no range and no cap), the frozen birth economics, and the actor
// schema at the replay's mechanism (the mechanism-v7 newborn additionally
// carries its two idle wage-contract rows), then appends it in birth order.
func (p *naturalProjection) finishBirth(mechanism NaturalMechanism, patches int, hour int) error {
	if len(p.staging) != 1 {
		return ErrNaturalCheckpoint
	}
	var id sim.EntityID
	var state *NaturalActorState
	for staged, s := range p.staging {
		id, state = staged, s
	}
	next := mustNaturalGenesisLast(patches) + sim.EntityID(len(p.newborns)) + 1
	if id != next || state == nil {
		return ErrNaturalCheckpoint
	}
	if state.Genome.ParentA == 0 || state.Genome.ParentB == 0 || state.Genome.BirthHour != int64(hour) ||
		state.Genome.DiedHour != NaturalNeverHour || state.Genome.BirthGranaryPaid != 0 ||
		!naturalPatchEntity(patches, state.Bag.Source) {
		return ErrNaturalCheckpoint
	}
	if !validNaturalActorFor(mechanism, patches, id, *state) {
		return ErrNaturalCheckpoint
	}
	p.newborns = append(p.newborns, NaturalNewbornState{ID: id, State: *state})
	p.staging = nil
	return nil
}

// naturalGenesisProjection mirrors the declared genesis projection state of
// one scenario: the seed-derived founder rows (the founding-band prefix f of
// the frozen block, stream positions 0..f−1), or the same rows lifted by
// the declared founder-tenure fixture pre-history. tenure requires f ≥ 2 or
// the default (f == 0): the pre-history seats founders 1 and 2. The
// mechanism selects the wage-contract rows: under mechanism v7 every
// founder's projection carries the same idle offer and book rows the seeds
// carry, so the genesis conservation gate and the replay's authoritative
// comparison read one declared state.
func naturalGenesisProjection(patches int, r int64, seed uint64, tenure bool, founders int, mechanism NaturalMechanism) ([]NaturalPatchState, [][]NaturalSlotState, []NaturalActorState, error) {
	if !NaturalFoundersValid(patches, founders) || (tenure && founders == 1) || !NaturalMechanismValid(mechanism) {
		return nil, nil, nil, ErrNaturalCheckpoint
	}
	if founders == 0 {
		block, err := NaturalFounderCount(patches)
		if err != nil {
			return nil, nil, nil, err
		}
		founders = block
	}
	var patchStates []NaturalPatchState
	slots := make([][]NaturalSlotState, patches)
	if tenure {
		patchStates, slots = fixturePatchStates(patches, r), fixtureSlotStates(patches, r)
	} else {
		patchStates = make([]NaturalPatchState, patches)
		for i := range patchStates {
			patchStates[i] = NaturalPatchState{Yield: r}
			slots[i] = make([]NaturalSlotState, NaturalSlotsPerPatch)
		}
	}
	founderRows := make([]NaturalActorState, founders)
	lifetimes := sim.NewRandomStream(sim.RandomState{Seed: seed, Stream: NaturalFounderStream})
	for i := range founderRows {
		lifetime := NaturalLifetimeBase + int64(lifetimes.Uint64()%NaturalLifetimeSpan)
		a, err := NaturalFounderState(patches, sim.EntityID(i+1), lifetime)
		if err != nil {
			return nil, nil, nil, err
		}
		founderRows[i] = a
		if mechanism == NaturalMechanismV7 {
			founderRows[i].Offer = NaturalIdleLabourOfferState()
			founderRows[i].Book = NaturalIdleLabourBookState()
		}
	}
	if tenure {
		// The fixture lift preserves every field the delta does not name
		// (the never-built hour, the never-stored-meal hour, debts,
		// counters), exactly like the seed rewrite.
		for _, founder := range []sim.EntityID{1, 2} {
			a := &founderRows[founder-1]
			a.Worksite = NaturalWorksiteState{Capital: 2, InvestedUnits: 4, PointsCreated: 2,
				Wip: a.Worksite.Wip, WearDebt: a.Worksite.WearDebt, PointsDecayed: a.Worksite.PointsDecayed,
				LastBuildHour: a.Worksite.LastBuildHour}
			a.Granary = NaturalGranaryState{Stock: 3, CultivatedTotal: 3,
				StoredMeals: a.Granary.StoredMeals, LastStoredMealHour: a.Granary.LastStoredMealHour,
				SpoilDebt: a.Granary.SpoilDebt, Spoiled: a.Granary.Spoiled}
			a.Body = NaturalBodyState{Energy: 5, BasalSpent: 6, Hunger: a.Body.Hunger,
				CapLost: a.Body.CapLost, Consumed: a.Body.Consumed,
				LastGatherHour: a.Body.LastGatherHour, BasalDebt: a.Body.BasalDebt}
		}
	}
	return patchStates, slots, founderRows, nil
}

// naturalProjectionFromHistory derives the hourly projection trace from
// verified accepted deltas rather than a second mutable ledger, comparing
// every hourly shadow against the trusted replay's own checkpoints (state,
// balance, and metrics) and the final shadow against the kernel's
// authoritative state. Gates G1/G2a/G2b/G3 are re-checked at every hour and
// gate G7 bounds every field, so a raw-kernel in-range patch bypass cannot
// survive replay silently. The returned trace is the replay's, whose ledger
// rows are the trusted hourly measurements.
func naturalProjectionFromHistory(k *kernel.Kernel, journal []NaturalBatch, config NaturalJournalConfig, reference *Natural) ([]NaturalCheckpoint, error) {
	patches := config.Patches
	mechanism := config.Mechanism
	if !NaturalMechanismValid(mechanism) || !naturalRefLegalUnderMechanism(mechanism, config.Policy) {
		return nil, ErrNaturalCheckpoint
	}
	table, err := naturalBoundsTableForRegistry(mechanism)
	if err != nil {
		return nil, err
	}
	patchStates, slots, founderRows, err := naturalGenesisProjection(patches, config.R, config.Seed, config.FounderTenure, config.Founders, mechanism)
	if err != nil {
		return nil, err
	}
	var p naturalProjection
	p.patches, p.slots, p.founders = patchStates, slots, founderRows
	p.refusals = make(map[NaturalRefusalReason]int64)
	events := k.Events()
	index := 0
	out := reference.Checkpoints()
	firstHour := -1
	for _, batch := range journal {
		hour := int(batch.Time / sim.SimTime(NaturalHour))
		if firstHour < 0 {
			firstHour = hour
		}
		for index < len(events) && events[index].Time == batch.Time {
			event := events[index]
			wrap := func(err error) error {
				return fmt.Errorf("natural event %d (t=%d, rule=%d): %w", index, event.Time, event.Rule, err)
			}
			switch event.Kind {
			case kernel.KindEntityCreate:
				// An entity-create carries both the parent cost patches
				// (existing rows) and the newborn's complete allocation rows
				// (absent-before); each delta routes by whether its row
				// pre-exists, exactly as the kernel partitions the proposal.
				for _, delta := range event.Deltas {
					if p.knownEntity(patches, delta.Entity) {
						if err := p.applyPatchDelta(mechanism, patches, event.Rule, delta); err != nil {
							return nil, wrap(err)
						}
					} else {
						if err := p.applyAllocationDelta(mechanism, patches, delta); err != nil {
							return nil, wrap(err)
						}
					}
				}
				if err := p.finishBirth(mechanism, patches, hour); err != nil {
					return nil, wrap(err)
				}
			case kernel.KindComponentPatch:
				for _, delta := range event.Deltas {
					if err := p.applyPatchDelta(mechanism, patches, event.Rule, delta); err != nil {
						return nil, wrap(err)
					}
				}
			default:
				return nil, wrap(ErrNaturalCheckpoint)
			}
			index++
		}
		if batch.Version != sim.WorldVersion(index) {
			return nil, fmt.Errorf("natural batch t=%d version %d != %d: %w", batch.Time, batch.Version, index, ErrNaturalCheckpoint)
		}
		for _, a := range batch.Attempts {
			switch a.Kind {
			case "birth-request":
				p.birthRequests++
			case "birth-reply":
				if a.Birth.Decision != NaturalReplyAccept {
					p.refusals[a.Birth.Reason]++
				}
				if a.Birth.Diverged {
					p.divergences++
				}
			}
		}
		if batch.Time%sim.SimTime(NaturalHour) != 0 {
			continue
		}
		hourIndex := hour - firstHour
		if hourIndex < 0 || hourIndex >= len(out) || out[hourIndex].Hour != hour {
			return nil, fmt.Errorf("natural hourly order at t=%d: %w", batch.Time, ErrNaturalCheckpoint)
		}
		allocatorLast := mustNaturalGenesisLast(patches) + sim.EntityID(len(p.newborns))
		balance, err := NaturalCheckConservationFor(mechanism, patches, p.patches, p.slots, p.founders, p.newborns, allocatorLast)
		if err != nil {
			return nil, err
		}
		check := NaturalCheckpoint{Hour: hour, Version: batch.Version, Balance: balance,
			Patches: p.patches, Slots: p.slots, Founders: p.founders, Newborns: p.newborns,
			AllocatorLast: allocatorLast}
		alive := 0
		for i := range p.founders {
			if naturalAlive(p.founders[i]) {
				alive++
			}
		}
		for _, newborn := range p.newborns {
			if naturalAlive(newborn.State) {
				alive++
			}
		}
		var prev NaturalBalance
		if hourIndex > 0 {
			prev = out[hourIndex-1].Balance
		}
		check.Metrics = NaturalMetrics{
			Alive: alive, DeathsAge: p.deaths.age, DeathsStarvation: p.deaths.starvation,
			Births: balance.Births, BirthRequests: p.birthRequests, Divergences: p.divergences,
			Refusals:        p.refusals,
			RegenCultivated: balance.Granted - prev.Granted,
			RegenWild:       balance.Produced - prev.Produced,
			RegenUnrealized: balance.Unrealized - prev.Unrealized,
		}
		if err := naturalProjectionBounds(check, table, mechanism); err != nil {
			return nil, fmt.Errorf("natural hour %d: %w", hour, err)
		}
		trusted := out[hourIndex]
		if trusted.Hour != check.Hour || trusted.Version != check.Version || trusted.Balance != check.Balance ||
			trusted.AllocatorLast != check.AllocatorLast ||
			!reflect.DeepEqual(trusted.Patches, check.Patches) || !reflect.DeepEqual(trusted.Slots, check.Slots) ||
			!reflect.DeepEqual(trusted.Founders, check.Founders) || !reflect.DeepEqual(trusted.Newborns, check.Newborns) ||
			!naturalMetricsEqual(trusted.Metrics, check.Metrics) {
			return nil, fmt.Errorf("natural hour %d: shadow projection diverged from the trusted replay: %w", hour, ErrNaturalCheckpoint)
		}
	}
	if index != len(events) {
		return nil, fmt.Errorf("natural unconsumed events %d..%d: %w", index, len(events), ErrNaturalCheckpoint)
	}
	head := k.SnapshotHead()
	view := scheduler.SnapshotView{Reader: head.Reader, Authority: head.Authority, Version: head.Version}
	w, err := worldStateAt(view, mechanism, patches, rosterIDs(p.founders, p.newborns))
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(w.patches, p.patches) || !reflect.DeepEqual(w.slots, p.slots) ||
		len(w.actors) != len(p.founders)+len(p.newborns) {
		return nil, fmt.Errorf("natural authoritative prefix mismatch: %w", ErrNaturalCheckpoint)
	}
	for i := range p.founders {
		if w.actors[sim.EntityID(i+1)] != p.founders[i] {
			return nil, fmt.Errorf("natural founder %d diverged: %w", i+1, ErrNaturalCheckpoint)
		}
	}
	for _, newborn := range p.newborns {
		if w.actors[newborn.ID] != newborn.State {
			return nil, fmt.Errorf("natural newborn %d diverged: %w", newborn.ID, ErrNaturalCheckpoint)
		}
	}
	if len(p.newborns) > 0 {
		if last := p.newborns[len(p.newborns)-1].ID; allocatorLastOf(patches, len(p.newborns)) != last {
			return nil, fmt.Errorf("natural allocator last %d diverges from the max roster ID %d: %w",
				allocatorLastOf(patches, len(p.newborns)), last, ErrNaturalCheckpoint)
		}
	}
	return out, nil
}

func naturalMetricsEqual(a, b NaturalMetrics) bool {
	if a.Alive != b.Alive || a.DeathsAge != b.DeathsAge || a.DeathsStarvation != b.DeathsStarvation ||
		a.Births != b.Births || a.BirthRequests != b.BirthRequests || a.Divergences != b.Divergences ||
		a.RegenCultivated != b.RegenCultivated || a.RegenWild != b.RegenWild || a.RegenUnrealized != b.RegenUnrealized ||
		len(a.Refusals) != len(b.Refusals) {
		return false
	}
	for reason, n := range a.Refusals {
		if b.Refusals[reason] != n {
			return false
		}
	}
	return true
}

func allocatorLastOf(patches int, births int) sim.EntityID {
	return mustNaturalGenesisLast(patches) + sim.EntityID(births)
}

func rosterIDs(founders []NaturalActorState, newborns []NaturalNewbornState) []sim.EntityID {
	out := make([]sim.EntityID, 0, len(founders)+len(newborns))
	for i := range founders {
		out = append(out, sim.EntityID(i+1))
	}
	for _, newborn := range newborns {
		out = append(out, newborn.ID)
	}
	return out
}

// worldStateAt reads the full authoritative state for one roster at one
// version (the runner's own worldState, generalized to an explicit roster
// and mechanism).
func worldStateAt(view scheduler.SnapshotView, mechanism NaturalMechanism, patches int, roster []sim.EntityID) (naturalWorld, error) {
	w := naturalWorld{patches: make([]NaturalPatchState, patches), slots: make([][]NaturalSlotState, patches), actors: make(map[sim.EntityID]NaturalActorState, len(roster))}
	for i := 0; i < patches; i++ {
		patch, err := NaturalPatchID(patches, i)
		if err != nil {
			return naturalWorld{}, err
		}
		n, err := capacityInts(view, patch, NaturalPatchTypeID, NaturalPatchYieldField, NaturalPatchPulsesField, NaturalPatchProducedField, NaturalPatchGrantedField, NaturalPatchUnrealizedField)
		if err != nil {
			return naturalWorld{}, err
		}
		w.patches[i] = NaturalPatchState{Yield: n[0], Pulses: n[1], Produced: n[2], Granted: n[3], Unrealized: n[4]}
		w.slots[i] = make([]NaturalSlotState, NaturalSlotsPerPatch)
		for j := range w.slots[i] {
			slot, err := NaturalSlotID(patches, i, j)
			if err != nil {
				return naturalWorld{}, err
			}
			ints, err := capacityInts(view, slot, NaturalSlotTypeID, NaturalSlotStockField, NaturalSlotGatheredField, NaturalSlotTenureModeField, NaturalSlotTenureCultivatedField)
			if err != nil {
				return naturalWorld{}, err
			}
			ownerValues, err := capacityRead(view, slot, NaturalSlotTypeID, NaturalSlotTenureOwnerField)
			if err != nil {
				return naturalWorld{}, err
			}
			owner, err := capacityRefValue(ownerValues[0])
			if err != nil {
				return naturalWorld{}, err
			}
			w.slots[i][j] = NaturalSlotState{Stock: ints[0], Gathered: ints[1], Mode: NaturalTenureMode(ints[2]), Owner: owner, Cultivated: ints[3]}
		}
	}
	for _, id := range roster {
		a, err := naturalActorState(view, mechanism, id)
		if err != nil {
			return naturalWorld{}, err
		}
		w.actors[id] = a
	}
	return w, nil
}

// frozenThresholdsMatch reports whether the runner's declared ceiling
// thresholds are exactly the frozen set (the only publishable evidence
// state).
func (f *Natural) frozenThresholdsMatch() bool {
	frozen := NaturalFrozenCeilingThresholds()
	if len(f.thresholds) != len(frozen) || len(f.ledger.thresholds) != len(frozen) {
		return false
	}
	for ceiling, value := range frozen {
		if f.thresholds[ceiling] != value || f.ledger.thresholds[ceiling] != value {
			return false
		}
	}
	return true
}

// SaveCheckpoint publishes the neutral h0 pilot bundle: all five sections
// are captured under the coordinator lock, only at the committed neutral h0
// pulse boundary before any claim, capital, or birth event, and never at an
// existing destination. The returned digest identifies the bundle for
// Branch lineage records.
func (f *Natural) SaveCheckpoint(path string) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.halted != nil || f.k == nil || f.sched == nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	if !f.frozenThresholdsMatch() {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	genesisLast, err := NaturalHighestFixedEntityID(f.patches)
	if err != nil || f.allocator.Last() != genesisLast {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	if f.steps != 1 || len(f.journal) != 1 || f.journal[0].Time != 0 || len(f.journal[0].Attempts) != 0 ||
		f.sched.Time() != 0 || len(f.checkpoints) != 1 || f.checkpoints[0].Hour != 0 ||
		len(f.current) != 0 || len(f.denied) != 0 || len(f.deniedBuild) != 0 || len(f.meals) != 0 ||
		len(f.newborns) != 0 || len(f.birthWakes) != 0 ||
		f.deathsAge != 0 || f.deathsStarve != 0 || f.birthFiles != 0 || f.divergences != 0 || len(f.refusals) != 0 {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	// The root is the neutral h0 world only: the fixture's pre-history means
	// its first pulse is naturalFixtureStartHour, which the boundary checks
	// above already refuse; the genesis-kind check below fails closed on any
	// future genesis kind.
	if f.journalGenesisKind().FounderTenure {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	// Do not publish a same-ref altered policy that a fresh executable would
	// reject later: the independent constructor pins the current policy contract.
	fresh, err := newNaturalReferenceRunner(NaturalRestoreOptions{Patches: f.patches, R: f.r, Seed: f.seed,
		Workers: 1, Births: f.births, CultivationDisabled: f.cultivationDisabled, PolicyRef: f.ref, Mechanism: f.mechanism,
		Founders: f.declaredFounders()})
	if err != nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	if f.ref != fresh.ref || f.mechanism != fresh.mechanism {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	return f.saveBundle(path, fresh, f.births, naturalManifest{patches: f.patches, r: f.r, seed: f.seed,
		births: f.births, boundary: f.births, cultivation: f.cultivationDisabled, mechanism: f.mechanism,
		founders: f.declaredFounders(), steps: f.steps, time: f.sched.Time()}, nil)
}

// SaveResumeCheckpoint publishes a pulse-boundary continuation bundle: the
// same five sections at any committed hourly boundary strictly after the
// genesis pulse, births included, with the dynamic roster (founders and
// newborns) in the strategy section's binding table. In-flight activity
// between instants is refused: the capture boundary is quiescent by
// construction (the hour's pulse just committed), and publication never
// replaces an existing destination. The boundary flag is recovered from the
// journal itself, so a continued Branch lineage records both flags.
func (f *Natural) SaveResumeCheckpoint(path string) ([32]byte, error) {
	return f.SaveResumeCheckpointWithReplay(path, nil, protocol.WakePolicy{})
}

// SaveResumeCheckpointWithReplay is the recorded-seam save: a seam-active run
// supplies the ledger-resolved Role A seam so its journal reconciles and the
// published bundle can be restored (ADR 0006 section 7.3).
func (f *Natural) SaveResumeCheckpointWithReplay(path string, seam ClaimSeam, wake protocol.WakePolicy) ([32]byte, error) {
	return f.saveResumeCheckpoint(path, seamReplayOf(seam, wake))
}

func (f *Natural) saveResumeCheckpoint(path string, seamReplay *naturalSeamReplay) ([32]byte, error) {
	if f == nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil || f.halted != nil || f.k == nil || f.sched == nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	if !f.frozenThresholdsMatch() {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	at := f.sched.Time()
	if at <= 0 || at%sim.SimTime(NaturalHour) != 0 || f.steps < 2 || f.steps != len(f.journal) ||
		len(f.current) != 0 || len(f.meals) != 0 {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	hour := int(at / sim.SimTime(NaturalHour))
	if len(f.checkpoints) == 0 || f.checkpoints[len(f.checkpoints)-1].Hour != hour ||
		len(f.checkpoints) != hour-f.checkpoints[0].Hour+1 {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	fresh, err := newNaturalReferenceRunner(NaturalRestoreOptions{Patches: f.patches, R: f.r, Seed: f.seed,
		Workers: 1, Births: f.births, FounderTenure: f.naturalRunnerTenure(), CultivationDisabled: f.cultivationDisabled,
		PolicyRef: f.ref, Mechanism: f.mechanism, Founders: f.declaredFounders()})
	if err != nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	if f.ref != fresh.ref || f.mechanism != fresh.mechanism {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	// The boundary flag is recovered from the journal itself: a continued
	// Branch lineage shares its first-hour prefix with the parent's
	// boundary, so the resume manifest records both the continuation and the
	// boundary flags.
	boundary := naturalBoundaryBirths(f.journal, f.checkpoints[0].Hour)
	m := naturalManifest{patches: f.patches, r: f.r, seed: f.seed, births: f.births, boundary: boundary,
		tenure: f.naturalRunnerTenure(), cultivation: f.cultivationDisabled, mechanism: f.mechanism,
		founders: f.declaredFounders(), steps: f.steps, time: at}
	return f.saveBundle(path, fresh, boundary, m, seamReplay)
}

// saveBundle captures the five sections under the coordinator lock, after
// the wake-topology, strategy, journal-replay, and projection
// reconstruction checks, and writes the bundle atomically.
func (f *Natural) saveBundle(path string, fresh *Natural, boundary bool, m naturalManifest, seamReplay *naturalSeamReplay) ([32]byte, error) {
	strategyBytes, err := encodeNaturalStrategy(f)
	if err != nil {
		return [32]byte{}, err
	}
	// The cognition block (spec 7.2) rides the manifest only for a runner with
	// an active generation and an attached binding; a nil state writes nothing
	// and every non-cognitive manifest keeps its bytes.
	if block, err := f.cognitiveBlockFromState(); err != nil {
		return [32]byte{}, err
	} else if block != nil {
		wire, err := encodeNaturalCognitiveBlock(*block)
		if err != nil {
			return [32]byte{}, err
		}
		m.cognitive, m.cognitiveWire = block, wire
	}
	history, head, err := f.k.ExportHistory()
	if err != nil {
		return [32]byte{}, err
	}
	portable, schedulerHead, err := f.sched.ExportPortable()
	if err != nil || head != schedulerHead {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	snap := f.sched.Snapshot()
	hour := int(snap.Time / sim.SimTime(NaturalHour))
	if m.time == 0 {
		if err := verifyNaturalNeutralScheduler(f.k, f.journal, snap, f.births, f.naturalLabourLane(), f.patches, f.founders); err != nil {
			return [32]byte{}, err
		}
	} else {
		if err := verifyNaturalBoundaryTopology(snap, f.births, f.naturalLabourLane(), hour, f.roster, f.pulseLifecycle, f.birthWakes); err != nil {
			return [32]byte{}, err
		}
	}
	if err := verifyNaturalStrategy(strategyBytes, fresh, f.stateView()); err != nil {
		return [32]byte{}, err
	}
	config := f.journalGenesisKind()
	config.Boundary = boundary
	journalBytes, err := encodeNaturalJournalWithSeam(f.journal, f.k, head, snap.Time, config, seamReplay, f.cognitiveJournalReplay())
	if err != nil {
		return [32]byte{}, err
	}
	reference, err := replayNaturalJournalWithCognitive(f.journal, config, f.cognitiveJournalReplay(), seamReplay)
	if err != nil {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	checks, err := naturalProjectionFromHistory(f.k, f.journal, config, reference)
	if err != nil || !reflect.DeepEqual(checks, cloneNaturalCheckpoints(f.checkpoints)) {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	fingerprint, err := strategy.FrozenNaturalPolicyFingerprint(f.bound[1].Policy())
	if err != nil {
		return [32]byte{}, err
	}
	// The manifest's declared policy identity is pinned twice (ticket 16 D1
	// seam): the policy-format slot and this exact content fingerprint, both
	// derived from the bound policy the strategy section carries.
	m.policyFormat = f.bound[1].Policy().FormatVersion
	m.head = head
	m.policyFingerprint = fingerprint
	for i, part := range [][]byte{journalBytes, history, portable, strategyBytes} {
		m.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(path, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: history},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeNaturalManifest(m)},
		{Name: checkpoint.Scheduler, Version: 1, Data: portable},
		{Name: checkpoint.Strategy, Version: 1, Data: strategyBytes},
	})
}

// stateView is the authoritative read view at the runner's committed head.
func (f *Natural) stateView() scheduler.SnapshotView {
	khead := f.k.SnapshotHead()
	return scheduler.SnapshotView{Reader: khead.Reader, Authority: khead.Authority, Version: khead.Version}
}

// restoreNaturalSections rebuilds an independent executable runner from
// already hash-checked bundle sections. Every section is validated against
// independently constructed registries and the declared scenario before the
// returned runner becomes visible; the dynamic roster, denial evidence,
// reserved WakeBirth reservations, deferred meals, the allocator's last ID,
// and the ceiling ledger come from the trusted journal replay and are
// cross-checked against the restored portable state and the max roster ID.
// The worker count comes from the caller, never from the bundle.
func restoreNaturalSections(sections []checkpoint.Section, m naturalManifest, expected NaturalRestoreOptions) (*Natural, error) {
	fail := func(err error) (*Natural, error) {
		return nil, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	// The reference runner binds the manifest's own declared policy ref: the
	// strategy section, the fingerprint pin, and the journal identity below
	// are all verified against it, so a forged slot/content pairing fails
	// closed even when the manifest bytes decode.
	manifestRef, err := m.naturalManifestPolicyRef()
	if err != nil {
		return fail(err)
	}
	fresh, err := newNaturalReferenceRunner(NaturalRestoreOptions{Patches: m.patches, R: m.r, Seed: m.seed,
		Workers: expected.Workers, Births: m.births, FounderTenure: m.tenure, CultivationDisabled: m.cultivation,
		PolicyRef: manifestRef, Mechanism: m.mechanism, Founders: m.founders})
	if err != nil {
		return fail(err)
	}
	_, genesis, err := fresh.k.ExportHistory()
	if err != nil {
		return fail(err)
	}
	if genesis.GenesisHash != m.head.GenesisHash || genesis.RegistryFingerprint != m.head.RegistryFingerprint {
		return nil, ErrNaturalCheckpoint
	}
	k, head, err := kernel.RestoreHistory(fresh.registry, sections[1].Data)
	if err != nil {
		return fail(err)
	}
	if head != m.head {
		return nil, ErrNaturalCheckpoint
	}
	fresh.k = k
	sched, err := scheduler.RestorePortable(k, expected.Workers, fresh.evaluate, sections[3].Data, head)
	if err != nil {
		return fail(err)
	}
	if sched.Time() != m.time {
		return nil, ErrNaturalCheckpoint
	}
	fresh.sched = sched
	// Run identity decodes under the recorded continuation flag; the
	// scheduler-aware reconciliation replays under the boundary flag, because
	// a Branch child's portable bytes carry the parent's boundary topology
	// (the v5 h0 pulse is births-asymmetric by frozen design).
	identityConfig := NaturalJournalConfig{Patches: m.patches, R: m.r, Seed: m.seed,
		Births: m.births, Boundary: m.boundary, FounderTenure: m.tenure,
		CultivationDisabled: m.cultivation, Mechanism: m.mechanism, Policy: fresh.ref, Founders: m.founders}
	fingerprint, err := naturalJournalFingerprint(fresh.ref)
	if err != nil {
		return fail(err)
	}
	identityConfig.PolicyFingerprint = fingerprint
	seamReplay := seamReplayOf(expected.CognitiveReplaySeam, expected.CognitiveReplayWake)
	// A cognitive manifest re-activates its recorded generation from the apply
	// boundary before the journal reconciles, so a generation that changed the
	// recorded decisions (spec section 5.4) still reproduces without a model
	// call. The block is decoded before the journal so the reference replay
	// carries the same generation the journal was produced under.
	replay, err := cognitiveReplayFromBlock(m.cognitive)
	if err != nil {
		return fail(err)
	}
	journal, err := decodeNaturalJournalWithSeam(sections[0].Data, k, head, sched.Time(), identityConfig, seamReplay, replay)
	if err != nil {
		return fail(err)
	}
	if len(journal) != m.steps {
		return nil, ErrNaturalCheckpoint
	}
	// The identity config already carries both flags: Boundary governs the
	// shared first-hour batch, Births every later batch. A cognitive manifest
	// re-activates its recorded generation from the apply boundary, and the
	// recorded Role A seam replays from the recorded-call ledger, so the
	// reference replay reproduces the recorded decisions without a model call.
	reference, err := replayNaturalJournalWithCognitive(journal, identityConfig, replay, seamReplay)
	if err != nil {
		return fail(err)
	}
	before, head2, err := sched.ExportPortable()
	if err != nil || head2 != head {
		return nil, ErrNaturalCheckpoint
	}
	want, replayHead, err := reference.sched.ExportPortable()
	if err != nil || replayHead != head || !bytes.Equal(before, want) {
		return nil, ErrNaturalCheckpoint
	}
	snap := sched.Snapshot()
	hour := int(m.time / sim.SimTime(NaturalHour))
	if m.time == 0 {
		if err := verifyNaturalNeutralScheduler(k, journal, snap, m.boundary, m.policyFormat == strategy.NaturalPolicyFormatV9, m.patches, naturalFoundersResolved(m.patches, m.founders)); err != nil {
			return fail(err)
		}
	} else {
		// A continuation's captured topology reflects the continuation flag:
		// the pending wakes were scheduled by the latest pulse under it. Only
		// the journal's shared first-hour batch replays under the boundary flag.
		if err := verifyNaturalBoundaryTopology(snap, m.births, m.policyFormat == strategy.NaturalPolicyFormatV9, hour, reference.roster, reference.pulseLifecycle, reference.birthWakes); err != nil {
			return fail(err)
		}
	}
	khead := k.SnapshotHead()
	headView := scheduler.SnapshotView{Reader: khead.Reader, Authority: khead.Authority, Version: khead.Version}
	if err := verifyNaturalStrategy(sections[4].Data, fresh, headView); err != nil {
		return fail(err)
	}
	policyFingerprint, err := strategy.FrozenNaturalPolicyFingerprint(fresh.bound[1].Policy())
	if err != nil {
		return fail(err)
	}
	if policyFingerprint != m.policyFingerprint {
		return nil, ErrNaturalCheckpoint
	}
	checks, err := naturalProjectionFromHistory(k, journal, identityConfig, reference)
	if err != nil {
		return fail(err)
	}
	if len(checks) == 0 || checks[len(checks)-1].Hour != hour {
		return nil, ErrNaturalCheckpoint
	}
	// The binding table is the roster exactly, and the recovered allocator
	// last-ID cross-checks against the max roster ID (fail closed): the
	// genesis base when no birth has happened, the highest newborn otherwise.
	_, tableBindings, err := decodeNaturalStrategySection(sections[4].Data)
	if err != nil {
		return fail(err)
	}
	if len(tableBindings) != len(reference.roster) {
		return nil, ErrNaturalCheckpoint
	}
	for i, b := range tableBindings {
		if b.actor != reference.roster[i] {
			return nil, ErrNaturalCheckpoint
		}
	}
	recoveredLast := reference.allocator.Last()
	genesisLast := mustNaturalGenesisLast(m.patches)
	if recoveredLast != genesisLast &&
		(len(reference.roster) == 0 || recoveredLast != reference.roster[len(reference.roster)-1]) {
		return nil, fmt.Errorf("natural allocator last %d diverges from the max roster ID: %w", recoveredLast, ErrNaturalCheckpoint)
	}
	fresh.allocator = sim.NewEntityAllocator(recoveredLast)
	// Re-bind every roster entity (founders and newborns) to the carried
	// frozen policy ref; the genome, not the ref, carries heredity.
	for _, id := range reference.roster {
		if id <= sim.EntityID(naturalFoundersResolved(m.patches, m.founders)) {
			continue // bound by the fresh constructor
		}
		bound, err := fresh.policies.Bind(strategy.NaturalBinding{Actor: id, Ref: fresh.ref})
		if err != nil {
			return fail(err)
		}
		if _, exists := fresh.bound[id]; exists {
			return nil, ErrNaturalCheckpoint
		}
		fresh.bound[id] = bound
	}
	fresh.roster = append([]sim.EntityID(nil), reference.roster...)
	fresh.newborns = append([]sim.EntityID(nil), reference.newborns...)
	fresh.pulseLifecycle = make(map[sim.EntityID]scheduler.Lifecycle, len(reference.pulseLifecycle))
	for id, state := range reference.pulseLifecycle {
		fresh.pulseLifecycle[id] = state
	}
	fresh.denied = make(map[sim.EntityID]int64, len(reference.denied))
	for id, denied := range reference.denied {
		fresh.denied[id] = denied
	}
	fresh.deniedBuild = make(map[sim.EntityID]int64, len(reference.deniedBuild))
	for id, denied := range reference.deniedBuild {
		fresh.deniedBuild[id] = denied
	}
	fresh.meals = make(map[sim.EntityID]NaturalMealIntent, len(reference.meals))
	for id, intent := range reference.meals {
		fresh.meals[id] = intent
	}
	fresh.birthWakes = make(map[sim.EntityID]int64, len(reference.birthWakes))
	for id, wake := range reference.birthWakes {
		fresh.birthWakes[id] = wake
	}
	fresh.refusals = make(map[NaturalRefusalReason]int64, len(reference.refusals))
	for reason, n := range reference.refusals {
		fresh.refusals[reason] = n
	}
	fresh.deathsAge, fresh.deathsStarve = reference.deathsAge, reference.deathsStarve
	fresh.birthFiles, fresh.divergences = reference.birthFiles, reference.divergences
	fresh.flow = reference.flow
	fresh.hourMeals = reference.hourMeals
	fresh.ledger = &naturalLedger{thresholds: frozenNaturalThresholds(),
		observed: copyCeilingMap(reference.ledger.observed), prev: copyCeilingMap(reference.ledger.prev)}
	// The restored ledger must still hold: the boundary was publishable only
	// without a crossing or a sub-hour projection.
	if ceiling, crossed := fresh.ledger.crossing(); crossed {
		return nil, fmt.Errorf("restored ceiling ledger crosses %q: %w", ceiling, ErrNaturalCheckpoint)
	}
	if _, soon := fresh.ledger.projected(); soon {
		return nil, fmt.Errorf("restored ceiling ledger projects a sub-hour binding: %w", ErrNaturalCheckpoint)
	}
	fresh.journal, fresh.checkpoints, fresh.steps = journal, checks, m.steps
	if len(fresh.roster)+1 != len(snap.Fibers) {
		return nil, ErrNaturalCheckpoint
	}
	// The manifest cognition block re-activates the recorded generation and its
	// exact actor bindings (spec 5.4/7.2).
	if err := restoreCognitiveState(fresh, m.cognitive); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	return fresh, nil
}

func copyCeilingMap(src map[NaturalCeiling]uint64) map[NaturalCeiling]uint64 {
	out := make(map[NaturalCeiling]uint64, len(src))
	for ceiling, value := range src {
		out[ceiling] = value
	}
	return out
}

func naturalCheckedSections(path string) ([]checkpoint.Section, [32]byte, naturalManifest, error) {
	sections, digest, err := checkpoint.ReadFile(path)
	if err != nil {
		return nil, [32]byte{}, naturalManifest{}, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	m, err := decodeNaturalManifest(sections[2].Data)
	if err != nil {
		return nil, [32]byte{}, naturalManifest{}, err
	}
	for i, part := range [][]byte{sections[0].Data, sections[1].Data, sections[3].Data, sections[4].Data} {
		if sha256.Sum256(part) != m.sections[i] {
			return nil, [32]byte{}, naturalManifest{}, ErrNaturalCheckpoint
		}
	}
	return sections, digest, m, nil
}

// naturalManifestMatches reports whether the bundle's recorded scenario is
// exactly the declared one: patches, budget, seed, births flag, the declared
// genesis kind (neutral or founder-tenure fixture), the declared policy ref
// (ticket 16 D1 seam — bidirectional mismatch rejection: a natural/5 bundle
// refuses a natural/6 expectation and vice versa, before any restore work
// and before any publication), and the declared mechanism axis (ticket 17 —
// the same bidirectional rule on the manifest's mechanism block: a v5
// bundle refuses a v6 expectation and vice versa).
func naturalManifestMatches(m naturalManifest, expected NaturalRestoreOptions) bool {
	ref, err := m.naturalManifestPolicyRef()
	if err != nil || ref != expected.policyRef() {
		return false
	}
	if m.mechanism != expected.mechanism() {
		return false
	}
	// The founding band is the same bidirectional rule (ticket 18): a
	// default (8P) bundle refuses an explicit-band expectation and vice
	// versa; both default spellings match each other.
	if naturalFoundersResolved(m.patches, m.founders) != naturalFoundersResolved(expected.Patches, expected.Founders) {
		return false
	}
	return m.patches == expected.Patches && m.r == expected.R && m.seed == expected.Seed &&
		m.births == expected.Births && m.tenure == expected.FounderTenure &&
		m.cultivation == expected.CultivationDisabled
}

// RestoreNaturalCheckpoint rebuilds an independent executable runner from a
// verified bundle — the neutral h0 root, a Branch child, or a
// pulse-boundary continuation with newborns present. A different worker
// count is allowed; the declared scenario (patches, budget, seed, births
// flag, fixture flag) must identify the bundle exactly, so an enabled
// continuation of a disabled root always requires an explicit Branch record.
func RestoreNaturalCheckpoint(path string, expected NaturalRestoreOptions) (*Natural, [32]byte, error) {
	if expected.Workers < 1 {
		return nil, [32]byte{}, ErrNaturalCheckpoint
	}
	if err := expected.validate(); err != nil {
		return nil, [32]byte{}, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	sections, digest, m, err := naturalCheckedSections(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if !naturalManifestMatches(m, expected) {
		return nil, [32]byte{}, ErrNaturalCheckpoint
	}
	if m.cognitive != nil && expected.CognitiveLedgerPath != "" {
		if err := VerifyCognitiveLedger(m.cognitive, expected.CognitiveLedgerPath); err != nil {
			return nil, [32]byte{}, err
		}
	}
	fresh, err := restoreNaturalSections(sections, m, expected)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return fresh, digest, nil
}

// BranchNaturalCheckpoint verifies a neutral bundle end to end under its
// recorded flag, then publishes a child bundle that continues the same world
// under the explicit births intervention. The child's manifest lineage
// records the parent digest and verified parent head, and its journal
// section is re-pinned to the intervention flag; the policy, history and
// scheduler sections stay byte-identical to the verified parent. Publication
// is atomic and never replaces an existing file; any verification failure
// publishes nothing, including a child whose lineage depth would exceed the
// predeclared restore bound.
//
// The v5 intervention is asymmetric by frozen design: the birth-request
// phase exists only when births are enabled, so an enabled boundary's wake
// topology carries pending request wakes that a births-disabled
// continuation would refuse to evaluate. The disabling direction is
// therefore refused before publication — it could only produce a child that
// bricks at the request instant — while enabling births (and same-flag
// lineage links) branches from the verified neutral root exactly as the
// ticket-12/13/14 discipline demands.
func BranchNaturalCheckpoint(path string, expected NaturalRestoreOptions, births bool, dest string) ([32]byte, error) {
	sections, digest, m, err := naturalCheckedSections(path)
	if err != nil {
		return [32]byte{}, err
	}
	if expected.Workers < 1 || !naturalManifestMatches(m, expected) {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	// The only permitted intervention point is the neutral h0 boundary; a
	// non-neutral prefix could not follow the recorded decision and fails
	// closed here.
	if m.time != 0 || m.steps != 1 || m.tenure {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	// Refuse before publication a child whose manifest depth would exceed the
	// restore-time decode bound, and a disabling flip whose pending request
	// wakes no births-disabled continuation may evaluate.
	if m.depth >= naturalMaxBranchDepth {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	if m.boundary && !births {
		return [32]byte{}, ErrNaturalCheckpoint
	}
	// The parent restores under the manifest's own declared policy ref (the
	// expectation match above already pinned it to the declared one).
	manifestRef, err := m.naturalManifestPolicyRef()
	if err != nil {
		return [32]byte{}, err
	}
	parent, err := restoreNaturalSections(sections, m, NaturalRestoreOptions{Patches: m.patches, R: m.r,
		Seed: m.seed, Workers: 1, Births: m.births, FounderTenure: m.tenure, PolicyRef: manifestRef,
		Mechanism: m.mechanism, Founders: m.founders})
	if err != nil {
		return [32]byte{}, err
	}
	config := parent.journalGenesisKind()
	config.Boundary = m.boundary
	config.Births = births
	journalBytes, err := encodeNaturalJournal(parent.journal, parent.k, m.head, m.time, config)
	if err != nil {
		return [32]byte{}, fmt.Errorf("%w: %v", ErrNaturalCheckpoint, err)
	}
	child := m
	child.births = births
	child.boundary = m.boundary
	child.parent = digest
	child.parentHead = m.head
	child.depth = m.depth + 1
	for i, part := range [][]byte{journalBytes, sections[1].Data, sections[3].Data, sections[4].Data} {
		child.sections[i] = sha256.Sum256(part)
	}
	return checkpoint.WriteFile(dest, []checkpoint.Section{
		{Name: checkpoint.Journal, Version: 1, Data: journalBytes},
		{Name: checkpoint.KernelHistory, Version: 1, Data: sections[1].Data},
		{Name: checkpoint.ManifestLineage, Version: 1, Data: encodeNaturalManifest(child)},
		{Name: checkpoint.Scheduler, Version: 1, Data: sections[3].Data},
		{Name: checkpoint.Strategy, Version: 1, Data: sections[4].Data},
	})
}
