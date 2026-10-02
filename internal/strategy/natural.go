package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
	"sync"
)

// Natural v5 policy lane (ticket 15). The frozen design in
// .scratch/15-natural-limits/spec.md (including the recorded corrigenda) is
// binding: every threshold below is quoted from it and must stay identical to
// the trusted world contract in internal/world/natural_contract*.go. The v3
// capacity cascade (claim Gather|EatStored|Wait; bag decision Build|Eat;
// denial fallback EatStored|Rest; at most one stored meal per actor-hour) and
// the v4 birth request/reply discipline (self-reported k/granary claims, the
// frozen consent conjunction, one request and at most one reply per
// actor-hour, typed refusals) are inherited verbatim and adapted to v5's
// land-based capital and cap-free roster. A choice is one intention at one
// frozen clock instant; the runner owns phase placement, build-completion
// pairing, the kin walk, the surplus gate on fresh truth, and admission, and
// the world rechecks state and version.
//
// # Build-denial choice contract with the runner (v5 land claim)
//
// A rule-505 build completion must claim the lowest-index free wild slot with
// stock 0 in the builder's home patch (world.NaturalBuild). Slot tenure is
// privileged (frozen Q7), so this policy can never foresee the refusal. The
// v5 contract is therefore denial-driven, exactly parallel to the v3/v4
// denied-gather discipline:
//
//  1. The world refuses an unclaimable completion with a typed
//     world.NaturalBuildRefusalError and ZERO mutation: nothing converts, the
//     bag keeps its held unit.
//  2. For the same actor-hour the runner re-evaluates the bag decision with
//     LastDenial = NaturalBuildDenied. The policy then reports Build
//     unavailable by returning Kind = NaturalEat — the held wild unit becomes
//     the meal — and never re-emits NaturalBuild in that hour. This is the
//     frozen "no free slot" leg of the bag decision: the policy side cannot
//     see tenure, so unavailability reaches it only as this denial guard.
//  3. The runner commits the resulting consume under its own phase
//     discipline; the world rechecks state, hour, and bag before applying
//     rule 507. This policy guarantees the choice, not the placement.
//
// A NaturalBuildDenied observation with an empty bag is structurally
// inconsistent (the refusal leaves the bag held) and is rejected as an
// error. A GatherDenied observation with bag = 1 stays inert on the frozen
// v3/v4 path order: the bag decision precedes the denial branch.
//
// # Slot visibility (frozen Q7)
//
// The claim view lists home-patch slot identities with their public stock
// values only. Cultivated slots never hold standing stock, so a tenured
// slot's stock reads exactly like an empty wild slot's: no view field can
// carry tenure mode, owner, the assigned cultivation slot, any debt carry
// (basal/yield/wear/spoil), a spoilage counter, allocator state, R, or a
// stream seed, and no field carries another actor's anything. Tenure stays
// privileged so self-reported birth claims remain unverifiable by the
// consenter (recorded design decision).
//
// # Budget
//
// The frozen lane budget is 32/32 (design: "policy natural/5, budget 32/32").
// Candidate sets are all-or-nothing: an over-budget claim set, courtship
// roster, or pending list yields the typed NaturalBudgetExhausted fallback —
// a declared compute-budget event (P15), never a sorted prefix and never a
// population cap. Roster and pending-list inputs therefore carry no
// structural length limit; only the fixture-frozen slot count (8 per patch)
// is structural.
const (
	NaturalPolicyFormatV5 uint32 = 5
	NaturalMaxCandidates         = 32

	naturalHorizonHours  = 168
	naturalSlotsPerPatch = 8

	naturalEnergyCapacity int64 = 12
	naturalHungerCapacity int64 = 24
	naturalBagCapacity    int64 = 1
	naturalKMax           int64 = 8
	naturalWipMax         int64 = 1
	// The granary has no cap in v5: spoilage (S=24) bounds it. The policy
	// reads own stock against the frozen horizon envelope only.
	naturalGranaryStockMax int64 = 8 * naturalHorizonHours // 1344
	// Frozen bag decision: build iff energy >= 5 or the granary holds a
	// fallback meal, and never with k+wip >= 8 (frozen code, as in v3/v4).
	naturalPolicyTLow int64 = 5
	naturalNeverHour  int64 = -1

	// Frozen consent thresholds and derived-kinship limits, verbatim v4
	// (world.NaturalConsentMinGranary/Capital/Energy, NaturalKinProhibited
	// Depth, NaturalKinWalkLimit).
	naturalConsentMinGranary  int64 = 3
	naturalConsentMinCapital  int64 = 2
	naturalConsentMinEnergy   int64 = 3
	naturalKinProhibitedDepth       = 2
	naturalKinWalkLimit             = 10

	// The ten-minute rest of a denied gatherer with an empty granary, the v1
	// rest window; it stays inside the frozen hour's remaining phases.
	NaturalRestDuration sim.Duration = 600 * 1e6
)

var (
	ErrInvalidNaturalPolicy      = errors.New("invalid natural policy")
	ErrUnknownNaturalPolicy      = errors.New("unknown natural policy reference")
	ErrInvalidNaturalBinding     = errors.New("invalid natural actor binding")
	ErrInvalidNaturalObservation = errors.New("invalid natural observation")
)

type NaturalRef struct {
	ID      string
	Version uint32
}

type NaturalBudget struct{ Candidates, Evaluations int }

// NaturalPolicy is pinned value-only content: the frozen v5 policy is exactly
// the {natural,5} reference with budget 32/32. The decision rules (the v3
// cascade with the v5 build-denial leg, the frozen proposer condition, the
// deterministic same-patch addressee, and the consent conjunction) are frozen
// code in the Evaluate methods, so they cannot drift at registration; the
// declared identity and format are compared against this constructor, and the
// budget is bounded by its declared ceiling. Any same-ref budget variant that
// still validates here fails later at restore: VerifyNaturalPolicy compares
// the complete wire content against this executable's frozen policy.
type NaturalPolicy struct {
	FormatVersion uint32
	Ref           NaturalRef
	Budget        NaturalBudget
}

// FrozenNaturalPolicy constructs the declared v5 policy, not a tunable
// family. Every actor of the cap-free roster — founders {1..8P} and ANY
// allocator-issued newborn ID — binds this single frozen ref: the genome, not
// the policy ref, carries heredity.
func FrozenNaturalPolicy() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV5,
		Ref:           NaturalRef{ID: "natural", Version: 5},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

func validNaturalRef(ref NaturalRef) bool {
	return ref.ID != "" && len(ref.ID) <= MaxRefIDBytes && ref.Version != 0
}

func validNaturalPolicy(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// The v5 ID-space partition, mirrored exactly from the trusted contract
// (world.NaturalFounderCount, NaturalSlotID, NaturalHighestFixedEntityID):
// actors 1..8P, patches 1001..1000+P, slots 2001..2000+8P, newborns strictly
// above the highest fixed entity ID (the allocator genesis last-ID) with no
// range and no cap. The ranges are disjoint for every P with 8P <= 1000; the
// frozen matrix uses P ∈ {2,32}. Fixed-block IDs — patches, slots, the
// unassigned stretch between founders and patches, zero, and the MaxUint64
// allocator-failure sentinel — are never valid roster entities.

func naturalFounderEntity(patches int, id sim.EntityID) bool {
	return patches >= 1 && id >= 1 && id <= sim.EntityID(naturalSlotsPerPatch*patches)
}

// naturalHighestFixedEntityID is the highest slot of the last patch
// (2000+8P), the world allocator's genesis last-ID.
func naturalHighestFixedEntityID(patches int) (sim.EntityID, error) {
	if patches < 1 {
		return 0, ErrInvalidNaturalBinding
	}
	return sim.EntityID(2000 + naturalSlotsPerPatch*patches), nil
}

func naturalNewbornEntity(patches int, id sim.EntityID) bool {
	last, err := naturalHighestFixedEntityID(patches)
	return err == nil && id > last && id != sim.EntityID(^uint64(0))
}

// naturalRosterEntity is the cap-free range-partition binding predicate: the
// ID-placed founders and ANY allocator-issued newborn ID.
func naturalRosterEntity(patches int, id sim.EntityID) bool {
	return naturalFounderEntity(patches, id) || naturalNewbornEntity(patches, id)
}

func naturalPatchEntity(patches int, id sim.EntityID) bool {
	return patches >= 1 && id >= 1001 && id <= sim.EntityID(1000+patches)
}

func naturalSlotEntity(patches int, id sim.EntityID) bool {
	if patches < 1 {
		return false
	}
	return id >= 2001 && id < sim.EntityID(2001+naturalSlotsPerPatch*patches)
}

// naturalFounderHomePatch is the fixed founder geometry
// (world.NaturalFounderPatchID): founders are ID-placed, 8 per patch.
// Newborns are not ID-placed — their home patch is the proposer's, carried by
// the bag source row.
func naturalFounderHomePatch(patches int, actor sim.EntityID) (sim.EntityID, bool) {
	if !naturalFounderEntity(patches, actor) {
		return 0, false
	}
	return sim.EntityID(1001 + (int(actor)-1)/naturalSlotsPerPatch), true
}

// naturalFirstSlot is the lowest slot ID of a home patch
// (world.NaturalSlotID(patchIndex, 0)).
func naturalFirstSlot(home sim.EntityID) sim.EntityID {
	return sim.EntityID(2001 + (int(home)-1001)*naturalSlotsPerPatch)
}

type NaturalBinding struct {
	Actor sim.EntityID
	Ref   NaturalRef
}

type NaturalRegistry struct {
	mu       sync.RWMutex
	patches  int
	policies map[NaturalRef]NaturalPolicy
}

// NewNaturalRegistry builds the registry for one fixture shape (P patches).
// A patches value below 1 yields no registry at all.
func NewNaturalRegistry(patches int) *NaturalRegistry {
	if patches < 1 {
		return nil
	}
	return &NaturalRegistry{patches: patches, policies: make(map[NaturalRef]NaturalPolicy)}
}

func (r *NaturalRegistry) Register(policy NaturalPolicy) error {
	if r == nil || !validNaturalPolicy(policy) {
		return ErrInvalidNaturalPolicy
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policies == nil {
		return ErrInvalidNaturalPolicy
	}
	if _, exists := r.policies[policy.Ref]; exists {
		return ErrInvalidNaturalPolicy
	}
	r.policies[policy.Ref] = policy // value-only; no caller-owned slices or maps
	return nil
}

// Bind pins one actor of the cap-free roster to the shared frozen policy. No
// actor owns or can mutate the registry's policy, and no newborn ID is out of
// range: any allocator-issued ID binds.
func (r *NaturalRegistry) Bind(binding NaturalBinding) (*NaturalBound, error) {
	if r == nil || !naturalRosterEntity(r.patches, binding.Actor) || !validNaturalRef(binding.Ref) {
		return nil, ErrInvalidNaturalBinding
	}
	r.mu.RLock()
	policy, ok := r.policies[binding.Ref]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownNaturalPolicy
	}
	return &NaturalBound{actor: binding.Actor, patches: r.patches, policy: policy}, nil
}

type NaturalBound struct {
	actor   sim.EntityID
	patches int
	policy  NaturalPolicy
}

func (b *NaturalBound) Binding() NaturalBinding {
	if b == nil {
		return NaturalBinding{}
	}
	return NaturalBinding{Actor: b.actor, Ref: b.policy.Ref}
}

// Policy returns the value-only frozen content; it carries no per-actor state.
func (b *NaturalBound) Policy() NaturalPolicy {
	if b == nil {
		return NaturalPolicy{}
	}
	return b.policy
}

// NaturalSlotView is one home-patch slot's public stock hint. Cultivated
// slots never hold standing stock, so this view cannot distinguish a tenured
// slot from an empty wild one — no tenure crosses the boundary. Stock is
// never a reservation or physical proof; the world validates gather.
type NaturalSlotView struct {
	ID    sim.EntityID
	Stock sim.Value
}

// NaturalDenial is supplied only after the actor's own claim was rejected by
// the world: GatherDenied after a rejected rule-506 gather claim, BuildDenied
// after a rule-505 completion refused for lack of a free wild slot (zero
// mutation; the bag keeps its unit).
type NaturalDenial uint8

const (
	NaturalNoDenial     NaturalDenial = iota
	NaturalGatherDenied               // the actor's own claim-phase gather was rejected
	NaturalBuildDenied                // the actor's own build completion found no free slot
)

// NaturalObservation is the actor-visible v5 claim view: the v4 own-set (own
// granary stock and last-stored-meal hour included) plus the home-patch slot
// stocks and the denial guard. By construction it can carry only the owner's
// own private rows (energy, hunger, bag, worksite, granary stock), the public
// home-patch slot stocks, and the ledger guards the frozen privacy list
// allows. No slot tenure (mode or owner), no assigned cultivation slot, no
// debt carry (basal/yield/wear/spoil), no spoilage counter, no allocator
// state, no R, no stream, no death-hour (not even the actor's own), and no
// other actor's anything has a field here at all. The runner must build it
// from actor-readable components only.
//
// The v3 bag-source semantics hold: BagSource is always a present entity ref
// naming the holder's home patch, whether the bag is empty or held; it
// carries newborn placement (the proposer's patch).
type NaturalObservation struct {
	Actor              sim.EntityID
	Ref                NaturalRef
	Hour               int
	WorldVersion       sim.WorldVersion
	Energy             sim.Value
	Hunger             sim.Value
	BagUnits           sim.Value
	BagSource          sim.Value // present EntityRef: the holder's home patch
	Capital            sim.Value
	Wip                sim.Value
	GranaryStock       sim.Value
	InvestedUnits      sim.Value
	LastBuildHour      sim.Value // -1 until the first committed build
	LastStoredMealHour sim.Value // -1 until the first stored meal
	LastDenial         NaturalDenial
	HomeSlots          []NaturalSlotView
}

type NaturalAction uint8

const (
	NaturalEat       NaturalAction = iota + 1 // eat the held wild unit
	NaturalGather                             // claim one visible home slot
	NaturalEatStored                          // draw the only permitted granary meal
	NaturalBuild                              // invest the held unit into the worksite
	NaturalRest                               // ten-minute rest after a denied gather
	NaturalWait                               // a fallback only, never a planned action
)

type NaturalFallback uint8

const (
	NaturalNoFallback NaturalFallback = iota
	NaturalNoCandidate
	NaturalBudgetExhausted
	NaturalInvalidSelf
)

// TargetSlot identifies the visible Gather affordance; every other action
// targets the actor's own rows implicitly. RestDuration is nonzero only for
// Rest. The runner places the intention at the frozen clock instant.
type NaturalChoice struct {
	Kind            NaturalAction
	TargetSlot      sim.EntityID
	RestDuration    sim.Duration
	ObservedVersion sim.WorldVersion
	Ref             NaturalRef
	Evaluated       int
	Fallback        NaturalFallback
}

// Evaluate is the frozen hourly decision cascade — the v3 cascade verbatim
// plus the v5 build-denial leg — a pure function of one actor-readable
// observation:
//
//	bag decision (bag=1, h+20m+2µs): Build iff (energy>=5 or granary>=1) and
//	  k+wip<8, else Eat; after a BuildDenied denial the same hour's
//	  re-evaluation reports Build unavailable and eats the held unit instead —
//	  never a second Build; the paired stored meal of a successful build is
//	  drawn fresh by the runner at completion and is never decided here;
//	claim (bag=0, h+1µs): visible home stock -> Gather (lowest slot ID), else
//	  granary>=1 and no stored meal yet this hour -> EatStored, else Wait;
//	denied-gather re-evaluation (bag=0 after the actor's own gather was
//	  rejected): EatStored if granary>=1 and none drawn yet this hour, else
//	  Rest — a second gather in the same hour is never proposed.
//
// A stored meal is emitted at most once per actor-hour: every EatStored
// emission requires LastStoredMealHour < the observed hour, so no reiterate
// at the claim, denial, or paired-meal slot of the same hour can double-draw
// against the frozen stored-meals ledger. The runner evaluates bag=0
// observations only at the claim instant or with NaturalGatherDenied set, so
// no cascade path can double-claim. Out-of-range own values leave a Wait
// choice with NaturalInvalidSelf instead of an intention; structurally
// malformed observations are errors.
func (b *NaturalBound) Evaluate(obs NaturalObservation) (NaturalChoice, error) {
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours || obs.LastDenial > NaturalBuildDenied ||
		len(obs.HomeSlots) > naturalSlotsPerPatch {
		return NaturalChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalChoice{Kind: NaturalWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	_, hungerOK := capacityInteger(obs.Hunger, 0, naturalHungerCapacity)
	bag, bagOK := capacityInteger(obs.BagUnits, 0, naturalBagCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, naturalKMax)
	wip, wipOK := capacityInteger(obs.Wip, 0, naturalWipMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, naturalGranaryStockMax)
	_, investedOK := capacityInteger(obs.InvestedUnits, 0, naturalHorizonHours)
	lastBuild, lastBuildOK := capacityInteger(obs.LastBuildHour, naturalNeverHour, naturalHorizonHours-1)
	lastMeal, lastMealOK := capacityInteger(obs.LastStoredMealHour, naturalNeverHour, naturalHorizonHours-1)
	if !energyOK || !hungerOK || !bagOK || !capitalOK || !wipOK || !granaryOK || !investedOK || !lastBuildOK || !lastMealOK ||
		lastBuild > int64(obs.Hour) || lastMeal > int64(obs.Hour) {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	// The bag names its holder's home patch whether empty or held; a founder
	// must name its ID-placed patch, a newborn its proposer-placed patch.
	home, err := obs.BagSource.EntityRef()
	if err != nil || obs.BagSource.Kind() != sim.EntityRefKind || obs.BagSource.State() != sim.Present || !naturalPatchEntity(b.patches, home) {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	if expected, ok := naturalFounderHomePatch(b.patches, obs.Actor); ok && expected != home {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	// A recorded build denial implies the refused unit is still held.
	if obs.LastDenial == NaturalBuildDenied && bag != 1 {
		return NaturalChoice{}, ErrInvalidNaturalObservation
	}
	slots := append([]NaturalSlotView(nil), obs.HomeSlots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].ID < slots[j].ID })
	first := naturalFirstSlot(home)
	for i, view := range slots {
		if view.ID < first || view.ID >= first+naturalSlotsPerPatch || (i > 0 && slots[i-1].ID == view.ID) {
			return NaturalChoice{}, ErrInvalidNaturalObservation
		}
		if _, ok := capacityInteger(view.Stock, 0, 1); !ok {
			return NaturalChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if bag == 1 { // frozen bag decision: Build | Eat
		if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 {
			choice.Fallback = NaturalBudgetExhausted
			return choice, nil
		}
		if obs.LastDenial == NaturalBuildDenied {
			// The world refused this hour's build (no free wild slot with
			// stock 0); report Build unavailable and fall back to the meal.
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEat, 1, NaturalNoFallback
			return choice, nil
		}
		if capital+wip < naturalKMax && (energy >= naturalPolicyTLow || granary >= 1) {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalBuild, 1, NaturalNoFallback
		} else {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEat, 1, NaturalNoFallback
		}
		return choice, nil
	}
	if obs.LastDenial == NaturalGatherDenied { // denied-gather fallback
		if granary >= 1 && lastMeal < int64(obs.Hour) {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEatStored, 1, NaturalNoFallback
		} else {
			choice.Kind, choice.RestDuration, choice.Evaluated, choice.Fallback = NaturalRest, NaturalRestDuration, 1, NaturalNoFallback
		}
		return choice, nil
	}
	candidates := make([]sim.EntityID, 0, len(slots))
	for _, view := range slots {
		if stock, ok := capacityInteger(view.Stock, 0, 1); ok && stock == 1 {
			candidates = append(candidates, view.ID)
		}
	}
	if len(candidates) > b.policy.Budget.Candidates || len(candidates) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted // all-or-nothing: never a sorted prefix
		return choice, nil
	}
	if len(candidates) > 0 {
		choice.Kind, choice.TargetSlot, choice.Evaluated, choice.Fallback = NaturalGather, candidates[0], len(candidates), NaturalNoFallback
		return choice, nil
	}
	if granary >= 1 && lastMeal < int64(obs.Hour) { // one stored meal per actor-hour at most
		choice.Kind, choice.Evaluated, choice.Fallback = NaturalEatStored, 1, NaturalNoFallback
	}
	return choice, nil
}

// NaturalRequestStatus mirrors the world's request-row status values
// (world.NaturalRequestStatus 0..4).
type NaturalRequestStatus uint8

const (
	NaturalRequestIdle     NaturalRequestStatus = iota // 0: never filed
	NaturalRequestPending                              // 1
	NaturalRequestAccepted                             // 2
	NaturalRequestRefused                              // 3
	NaturalRequestExpired                              // 4
)

// NaturalRequestGuard is the requester's own request-row guard. A pending
// request must be finalized, and the frozen one-request-per-actor-hour rule
// blocks any second filing in the same hour (world rule 510 rejects
// LastRequestHour >= the current hour).
type NaturalRequestGuard struct {
	Status          NaturalRequestStatus
	LastRequestHour int64 // -1 while never filed
}

// NaturalClaimOverride is an explicit test injection for the reported
// claims. It never changes the actor's own eligibility or addressee: those
// are decided from the observation's own private state, while the choice
// carries exactly the overridden reported values so claim-vs-truth
// divergence stays constructible and auditable.
type NaturalClaimOverride struct{ Capital, Granary int64 }

// NaturalRequestObservation is the actor-visible birth-request view at the
// frozen h+40m+5µs phase. It carries the actor's own private margins
// (energy, capital, granary stock), its own request guard, the public home
// patch, and the public same-patch roster — no other actor's state, no death
// hours, no debts, no spoilage counters, no allocator state, no streams, and
// no claim-vs-truth divergence data.
type NaturalRequestObservation struct {
	Actor        sim.EntityID
	Ref          NaturalRef
	Hour         int
	WorldVersion sim.WorldVersion
	HomePatch    sim.EntityID // the actor's home patch (its bag source row); public
	Energy       sim.Value
	Capital      sim.Value
	GranaryStock sim.Value
	OwnRequest   NaturalRequestGuard
	PatchMates   []sim.EntityID // same-patch roster excluding self; runner-supplied public data
	TestClaims   *NaturalClaimOverride
}

type NaturalRequestAction uint8

const (
	NaturalRequestWait NaturalRequestAction = iota
	NaturalRequestFile
)

// NaturalRequestChoice files one addressed request. ReportedCapital and
// ReportedGranary are the CLAIM — the actor's reported values, honest
// self-reports of its own observed margins unless TestClaims overrides them
// — never an authoritative account of truth; the world's privileged audit
// alone compares claims against truth.
type NaturalRequestChoice struct {
	Kind            NaturalRequestAction
	Addressee       sim.EntityID
	ReportedCapital int64
	ReportedGranary int64
	ObservedVersion sim.WorldVersion
	Ref             NaturalRef
	Evaluated       int
	Fallback        NaturalFallback
}

// EvaluateBirthRequest is the frozen proposer decision at h+40m+5µs,
// verbatim v4 over the cap-free roster:
//
//	living, request slot free (not Pending, LastRequestHour < hour), and the
//	  frozen proposer condition — the actor's own state at the surplus-gate
//	  margins (k>=2, granary>=3, energy>=3) it must later show on fresh truth;
//	addressee: the directed successor among the same-patch roster (first
//	  mate with a higher ID, wrapping to the lowest), deterministic in the
//	  roster order and independent of the input order;
//	claims: the actor's self-reported capital and granary — its own observed
//	  values, or the explicit test override. The world re-verifies both
//	  parents' truth at the reply; a reported claim is never consulted as
//	  truth by this policy.
//
// Unlike v4 there is no structural roster bound: an over-budget roster
// yields the typed NaturalBudgetExhausted fallback (P15 compute event), so
// the cap-free roster can never turn a mate list into a malformed
// observation.
func (b *NaturalBound) EvaluateBirthRequest(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours ||
		obs.OwnRequest.Status > NaturalRequestExpired ||
		obs.OwnRequest.LastRequestHour < naturalNeverHour || obs.OwnRequest.LastRequestHour > int64(obs.Hour) ||
		(obs.OwnRequest.Status == NaturalRequestIdle && obs.OwnRequest.LastRequestHour != naturalNeverHour) ||
		(obs.OwnRequest.Status != NaturalRequestIdle && obs.OwnRequest.LastRequestHour < 0) ||
		!naturalPatchEntity(b.patches, obs.HomePatch) ||
		(obs.TestClaims != nil && (obs.TestClaims.Capital < 0 || obs.TestClaims.Capital > naturalKMax ||
			obs.TestClaims.Granary < 0 || obs.TestClaims.Granary > naturalGranaryStockMax)) {
		return NaturalRequestChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalRequestChoice{Kind: NaturalRequestWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, naturalKMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, naturalGranaryStockMax)
	if !energyOK || !capitalOK || !granaryOK {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	founderHome, isFounder := naturalFounderHomePatch(b.patches, obs.Actor)
	if isFounder && founderHome != obs.HomePatch {
		return NaturalRequestChoice{}, ErrInvalidNaturalObservation
	}
	mates := append([]sim.EntityID(nil), obs.PatchMates...)
	sort.Slice(mates, func(i, j int) bool { return mates[i] < mates[j] })
	for i, mate := range mates {
		if !naturalRosterEntity(b.patches, mate) || mate == obs.Actor || (i > 0 && mates[i-1] == mate) {
			return NaturalRequestChoice{}, ErrInvalidNaturalObservation
		}
		// Founder placement is public fixed geometry: a founder mate on a
		// different patch than the observed home patch is a malformed roster.
		// Newborn placement stays runner-supplied public data; the world
		// re-gates same-patch on fresh truth at the reply.
		if home, ok := naturalFounderHomePatch(b.patches, mate); ok && home != obs.HomePatch {
			return NaturalRequestChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if obs.OwnRequest.Status == NaturalRequestPending || obs.OwnRequest.LastRequestHour >= int64(obs.Hour) {
		return choice, nil // one request per actor-hour; a pending request resolves first
	}
	// The frozen proposer condition: the actor's own state must already sit
	// at the gate margins the runner will re-check on fresh truth.
	if capital < naturalConsentMinCapital || granary < naturalConsentMinGranary || energy < naturalConsentMinEnergy {
		return choice, nil
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 ||
		len(mates) > b.policy.Budget.Candidates || len(mates) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted
		return choice, nil
	}
	if len(mates) == 0 {
		return choice, nil
	}
	addressee := mates[0] // wraparound successor of the highest same-patch ID
	for _, mate := range mates {
		if mate > obs.Actor {
			addressee = mate
			break
		}
	}
	claims := NaturalClaimOverride{Capital: capital, Granary: granary} // honest self-report
	if obs.TestClaims != nil {
		claims = *obs.TestClaims
	}
	choice.Kind = NaturalRequestFile
	choice.Addressee = addressee
	choice.ReportedCapital = claims.Capital
	choice.ReportedGranary = claims.Granary
	choice.Evaluated = len(mates)
	choice.Fallback = NaturalNoFallback
	return choice, nil
}

// NaturalPendingView is one pending birth request addressed to the observing
// consenter: the requester's public identity, its reported claims, and the
// public derived kin distance. It intentionally has no requester energy,
// granary, capital, bag, loci, birth/death hours, debt, or history fields —
// the consenter never sees requester truth, only the CLAIM.
type NaturalPendingView struct {
	Requester       sim.EntityID
	Addressee       sim.EntityID
	Hour            int
	ReportedCapital int64
	ReportedGranary int64
	KinDistance     int  // public derived kin walk distance; 0 while unrelated
	KinRelated      bool // related within the runner's ≤10-reference walk
}

// NaturalReplyObservation is the actor-visible birth-reply view at the
// frozen h+40m+6µs phase: the consenter's own private margins, its own
// one-reply-per-hour guard, and the pending addressed requests' claims and
// public kin distances. The kin distance is public because births are public
// events; the walk itself is the trusted runner's duty.
type NaturalReplyObservation struct {
	Actor            sim.EntityID
	Ref              NaturalRef
	Hour             int
	WorldVersion     sim.WorldVersion
	Energy           sim.Value
	Capital          sim.Value
	GranaryStock     sim.Value
	OwnLastReplyHour int64 // typed world ledger guard; -1 until first reply
	Pending          []NaturalPendingView
}

type NaturalReplyAction uint8

const (
	NaturalReplyNone NaturalReplyAction = iota // no reply this hour
	NaturalReplyAccept
	// NaturalReplyRefuse is a typed refusal the world commits as rule 512
	// (the two request rows only).
	NaturalReplyRefuse
)

// NaturalReplyReason is the typed consent-phase refusal taxonomy. The
// remaining world refusal reasons (structural, death-between-phases,
// cross-patch, fresh-truth gate) are runner-side: the policy never sees the
// truth they are computed from. There is no birth-cap reason: births are
// uncapped in v5.
type NaturalReplyReason uint8

const (
	NaturalReplyReasonNone          NaturalReplyReason = iota
	NaturalReplyReasonConsentOwn                       // the consenter's own state fails the consent formula
	NaturalReplyReasonConsentClaims                    // the reported claims fail the consent formula
	NaturalReplyReasonKin                              // kin walk distance ≤ 2
)

// NaturalReplyChoice is one reply intention; the runner places it at the
// frozen instant and the world rechecks everything.
type NaturalReplyChoice struct {
	Kind            NaturalReplyAction
	Requester       sim.EntityID
	Reason          NaturalReplyReason
	ObservedVersion sim.WorldVersion
	Ref             NaturalRef
	Evaluated       int
	Fallback        NaturalFallback
}

// EvaluateBirthReply is the frozen consenter decision at h+40m+6µs, verbatim
// v4. It sees its own private state, the public kin distance, and the
// requester's reported claims — never requester truth. The frozen consent
// conjunction in its exact order: own granary>=3 ∧ own k>=2 ∧ own energy>=3
// ∧ reported k>=2 ∧ reported granary>=3 ∧ (kin-distance>=3 ∨ unrelated).
// Addressed requests are evaluated in ascending requester order; the first
// acceptable one is accepted, otherwise the first (lowest-ID) is refused
// with its typed reason — one reply per actor-hour. Structural mismatches
// are observation errors; the surplus gate on fresh truth is the runner's,
// not this policy's. As in the request view there is no structural pending
// bound: an over-budget pending list yields the typed
// NaturalBudgetExhausted fallback.
func (b *NaturalBound) EvaluateBirthReply(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours ||
		obs.OwnLastReplyHour < naturalNeverHour || obs.OwnLastReplyHour > int64(obs.Hour) {
		return NaturalReplyChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalReplyChoice{Kind: NaturalReplyNone, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, naturalKMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, naturalGranaryStockMax)
	if !energyOK || !capitalOK || !granaryOK {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	pending := append([]NaturalPendingView(nil), obs.Pending...)
	sort.Slice(pending, func(i, j int) bool { return pending[i].Requester < pending[j].Requester })
	for i, view := range pending {
		if !naturalRosterEntity(b.patches, view.Requester) || view.Requester == obs.Actor ||
			view.Addressee != obs.Actor || view.Hour != obs.Hour ||
			view.ReportedCapital < 0 || view.ReportedCapital > naturalKMax ||
			view.ReportedGranary < 0 || view.ReportedGranary > naturalGranaryStockMax ||
			(view.KinRelated && (view.KinDistance < 1 || view.KinDistance > naturalKinWalkLimit)) ||
			(!view.KinRelated && view.KinDistance != 0) ||
			(i > 0 && pending[i-1].Requester == view.Requester) {
			return NaturalReplyChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 || obs.OwnLastReplyHour >= int64(obs.Hour) || len(pending) == 0 {
		return choice, nil // a dead consenter never acts; one reply per actor-hour
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 ||
		len(pending) > b.policy.Budget.Candidates || len(pending) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted
		return choice, nil
	}
	ownOK := granary >= naturalConsentMinGranary && capital >= naturalConsentMinCapital && energy >= naturalConsentMinEnergy
	consentReason := func(view NaturalPendingView) NaturalReplyReason {
		switch {
		case !ownOK:
			return NaturalReplyReasonConsentOwn
		case view.ReportedCapital < naturalConsentMinCapital || view.ReportedGranary < naturalConsentMinGranary:
			return NaturalReplyReasonConsentClaims
		case view.KinRelated && view.KinDistance <= naturalKinProhibitedDepth:
			return NaturalReplyReasonKin
		}
		return NaturalReplyReasonNone
	}
	examined := 0
	for _, view := range pending {
		examined++
		if consentReason(view) == NaturalReplyReasonNone {
			choice.Kind = NaturalReplyAccept
			choice.Requester = view.Requester
			choice.Evaluated = examined
			choice.Fallback = NaturalNoFallback
			return choice, nil
		}
	}
	choice.Kind = NaturalReplyRefuse
	choice.Requester = pending[0].Requester
	choice.Reason = consentReason(pending[0])
	choice.Evaluated = examined
	choice.Fallback = NaturalNoFallback
	return choice, nil
}
