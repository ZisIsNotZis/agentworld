package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
	"sync"
)

// Reproduction v4 policy lane (ticket 14). The frozen design in
// .scratch/14-reproduction-genetics/spec.md is binding: every threshold below
// is quoted from it and must stay identical to the trusted world contract in
// internal/world/reproduction_contract*.go. The v3 capacity cascade (claim:
// Gather|EatStored|Wait; bag decision: Build|Eat; denial fallback
// EatStored|Rest) extends unchanged with two new phases: the birth request at
// h+40m+5µs (proposer files an addressed request with self-reported k/granary
// claims) and the birth reply at h+40m+6µs (consenter decides from its own
// private state, the public kin distance, and the requester's claims — never
// the requester's truth). A choice is one intention at one frozen clock
// instant; the runner owns phase placement, the kin walk, the surplus gate on
// fresh truth, and admission, and the world rechecks state and version.
const (
	ReproductionPolicyFormatV4 uint32 = 4
	ReproductionMaxFounders           = 16
	ReproductionMaxBirths             = 16
	ReproductionFirstNewbornID        = 10001
	ReproductionMaxCandidates         = 32

	reproductionHorizonHours  = 168
	reproductionSlotsPerPatch = 8

	reproductionEnergyCapacity int64 = 12
	reproductionHungerCapacity int64 = 24
	reproductionBagCapacity    int64 = 1
	reproductionKMax           int64 = 8
	reproductionWipMax         int64 = 1
	reproductionGranaryMax     int64 = 8

	// Frozen consent thresholds: the consenter's own-state margins and the
	// reported-claim margins are the same numbers (world.ReproductionConsent
	// MinGranary/Capital/Energy), and the kin rule prohibits related pairs
	// sharing an ancestor at walk depth ≤2 (world.ReproductionKinProhibited
	// Depth) within the runner's ≤10-reference walk.
	reproductionConsentMinGranary  int64 = 3
	reproductionConsentMinCapital  int64 = 2
	reproductionConsentMinEnergy   int64 = 3
	reproductionKinProhibitedDepth       = 2
	reproductionKinWalkLimit             = 10

	reproductionNeverHour int64 = -1

	// The ten-minute rest of a denied gatherer with an empty granary, the v1
	// rest window; it stays inside the frozen hour's remaining phases.
	ReproductionRestDuration sim.Duration = 600 * 1e6
)

var (
	ErrInvalidReproductionPolicy      = errors.New("invalid reproduction policy")
	ErrUnknownReproductionPolicy      = errors.New("unknown reproduction policy reference")
	ErrInvalidReproductionBinding     = errors.New("invalid reproduction actor binding")
	ErrInvalidReproductionObservation = errors.New("invalid reproduction observation")
)

type ReproductionRef struct {
	ID      string
	Version uint32
}

type ReproductionBudget struct{ Candidates, Evaluations int }

// ReproductionPolicy is pinned value-only content: the frozen pilot is
// exactly the {reproduction,4} reference with budget 32/32. The decision
// rules (the v3 cascade, the frozen proposer condition, the deterministic
// same-patch addressee, and the consent conjunction) are frozen code in the
// Evaluate methods, so they cannot drift at registration; the declared
// identity and format are compared against this constructor, and the budget
// is bounded by its declared ceiling. Any same-ref budget variant that still
// validates here fails later at restore: VerifyReproductionPolicy compares
// the complete wire content against this executable's frozen policy.
type ReproductionPolicy struct {
	FormatVersion uint32
	Ref           ReproductionRef
	Budget        ReproductionBudget
}

// FrozenReproductionPolicy constructs the declared v4 pilot, not a tunable
// family. All actors of the full roster — founders {1..16} and newborns
// {10001..10016} — bind this single frozen policy: the genome, not the
// policy ref, carries heredity.
func FrozenReproductionPolicy() ReproductionPolicy {
	return ReproductionPolicy{
		FormatVersion: ReproductionPolicyFormatV4,
		Ref:           ReproductionRef{ID: "reproduction", Version: 4},
		Budget:        ReproductionBudget{Candidates: ReproductionMaxCandidates, Evaluations: ReproductionMaxCandidates},
	}
}

func validReproductionRef(ref ReproductionRef) bool {
	return ref.ID != "" && len(ref.ID) <= MaxRefIDBytes && ref.Version != 0
}

func validReproductionPolicy(p ReproductionPolicy) bool {
	frozen := FrozenReproductionPolicy()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// validReproductionRosterEntity is the full v4 roster (world
// reproductionActorEntity): the sixteen ID-placed founders and the sixteen
// newborn slots 10001..10016 (next = 10000+births+1). A newborn binds the
// same frozen ref as its parents — a fresh binding, never an inherited one.
func validReproductionRosterEntity(id sim.EntityID) bool {
	return (id >= 1 && id <= ReproductionMaxFounders) ||
		(id >= ReproductionFirstNewbornID && id < ReproductionFirstNewbornID+ReproductionMaxBirths)
}

type ReproductionBinding struct {
	Actor sim.EntityID
	Ref   ReproductionRef
}

type ReproductionRegistry struct {
	mu       sync.RWMutex
	policies map[ReproductionRef]ReproductionPolicy
}

func NewReproductionRegistry() *ReproductionRegistry {
	return &ReproductionRegistry{policies: make(map[ReproductionRef]ReproductionPolicy)}
}

func (r *ReproductionRegistry) Register(policy ReproductionPolicy) error {
	if r == nil || !validReproductionPolicy(policy) {
		return ErrInvalidReproductionPolicy
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policies == nil {
		return ErrInvalidReproductionPolicy
	}
	if _, exists := r.policies[policy.Ref]; exists {
		return ErrInvalidReproductionPolicy
	}
	r.policies[policy.Ref] = policy // value-only; no caller-owned slices or maps
	return nil
}

// Bind pins one actor of the full roster to the shared frozen policy. No
// actor owns or can mutate the registry's policy.
func (r *ReproductionRegistry) Bind(binding ReproductionBinding) (*ReproductionBound, error) {
	if r == nil || !validReproductionRosterEntity(binding.Actor) || !validReproductionRef(binding.Ref) {
		return nil, ErrInvalidReproductionBinding
	}
	r.mu.RLock()
	policy, ok := r.policies[binding.Ref]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownReproductionPolicy
	}
	return &ReproductionBound{actor: binding.Actor, policy: policy}, nil
}

type ReproductionBound struct {
	actor  sim.EntityID
	policy ReproductionPolicy
}

func (b *ReproductionBound) Binding() ReproductionBinding {
	if b == nil {
		return ReproductionBinding{}
	}
	return ReproductionBinding{Actor: b.actor, Ref: b.policy.Ref}
}

// Policy returns the value-only frozen content; it carries no per-actor state.
func (b *ReproductionBound) Policy() ReproductionPolicy {
	if b == nil {
		return ReproductionPolicy{}
	}
	return b.policy
}

// reproductionFounderHomePatch is the fixed founder geometry
// (world.ReproductionFounderPatchID): actors 1..8 share patch 1001 and
// actors 9..16 share patch 1002. Newborns are not ID-placed — their home
// patch is the proposer's, carried by the bag source row.
func reproductionFounderHomePatch(actor sim.EntityID) (sim.EntityID, bool) {
	if actor < 1 || actor > ReproductionMaxFounders {
		return 0, false
	}
	return sim.EntityID(1001 + (int(actor)-1)/reproductionSlotsPerPatch), true
}

// reproductionValidPatch admits exactly the two home patches.
func reproductionValidPatch(id sim.EntityID) bool {
	return id == 1001 || id == 1002
}

// reproductionFirstSlot is the lowest slot ID of a home patch
// (world.ReproductionSlotID(patchIndex, 0)).
func reproductionFirstSlot(home sim.EntityID) sim.EntityID {
	return sim.EntityID(2001 + (int(home)-1001)*reproductionSlotsPerPatch)
}

// ReproductionSlotView is one open-access home-patch slot's public stock
// hint. Stock is never a reservation or physical proof; the world validates
// gather.
type ReproductionSlotView struct {
	ID    sim.EntityID
	Stock sim.Value
}

// ReproductionDenial is supplied only after the actor's own Gather was rejected.
type ReproductionDenial uint8

const (
	ReproductionNoDenial     ReproductionDenial = iota
	ReproductionGatherDenied                    // the actor's own claim-phase gather was rejected
)

// ReproductionObservation is the actor-visible v4 claim view: the v3 own-set
// plus the home-patch slot stocks and the denial guard. By construction it
// can carry only the owner's own private rows (energy, hunger, bag, worksite,
// granary), the public home-patch slot stocks, and the ledger guards the
// frozen privacy list allows. No death-hour (not even the actor's own), no
// debt accumulator, no stream, and no other actor's loci, granary, capital,
// or energy has a field here at all. The runner must build it from
// actor-readable components only.
//
// The one v3 field-semantics delta: BagSource is always a present entity ref
// naming the holder's home patch, empty bag or held — it carries newborn
// placement (proposer's patch) instead of being absent while empty.
type ReproductionObservation struct {
	Actor              sim.EntityID
	Ref                ReproductionRef
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
	LastDenial         ReproductionDenial
	HomeSlots          []ReproductionSlotView
}

type ReproductionAction uint8

const (
	ReproductionEat       ReproductionAction = iota + 1 // eat the held wild unit
	ReproductionGather                                  // claim one visible home slot
	ReproductionEatStored                               // draw the only permitted granary meal
	ReproductionBuild                                   // invest the held unit into the worksite
	ReproductionRest                                    // ten-minute rest after a denied gather
	ReproductionWait                                    // a fallback only, never a planned action
)

type ReproductionFallback uint8

const (
	ReproductionNoFallback ReproductionFallback = iota
	ReproductionNoCandidate
	ReproductionBudgetExhausted
	ReproductionInvalidSelf
)

// TargetSlot identifies the visible Gather affordance; every other action
// targets the actor's own rows implicitly. RestDuration is nonzero only for
// Rest. The runner places the intention at the frozen clock instant.
type ReproductionChoice struct {
	Kind            ReproductionAction
	TargetSlot      sim.EntityID
	RestDuration    sim.Duration
	ObservedVersion sim.WorldVersion
	Ref             ReproductionRef
	Evaluated       int
	Fallback        ReproductionFallback
}

// Evaluate is the frozen hourly decision cascade, byte-for-byte the v3
// capacity discipline over the v4 geometry, a pure function of one
// actor-readable observation:
//
//	bag decision (bag=1, h+20m+2µs): Build iff (energy>=5 or granary>=1) and
//	  k+wip<8, else Eat; the paired stored meal is drawn fresh at build
//	  completion and is the runner's duty, never decided here;
//	claim (bag=0, h+1µs): visible home stock -> Gather (lowest slot ID), else
//	  granary>=1 and no stored meal yet this hour -> EatStored, else Wait;
//	denied-gather re-evaluation (bag=0 after the actor's own gather was
//	  rejected): EatStored if granary>=1 and none drawn yet this hour, else
//	  Rest — a second gather in the same hour is never proposed.
//
// A stored meal is emitted at most once per actor-hour: every EatStored
// emission requires LastStoredMealHour < the observed hour. The paired meal
// of a build is drawn by the runner at completion after fresh granary
// revalidation; this policy only ever reports the Build|Eat intent. The
// runner evaluates bag=0 observations only at the claim instant or with
// ReproductionGatherDenied set, so no cascade path can double-claim.
// Out-of-range own values leave a Wait choice with ReproductionInvalidSelf
// instead of an intention; structurally malformed observations are errors.
func (b *ReproductionBound) Evaluate(obs ReproductionObservation) (ReproductionChoice, error) {
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= reproductionHorizonHours || obs.LastDenial > ReproductionGatherDenied ||
		len(obs.HomeSlots) > reproductionSlotsPerPatch {
		return ReproductionChoice{}, ErrInvalidReproductionObservation
	}
	choice := ReproductionChoice{Kind: ReproductionWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: ReproductionNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, reproductionEnergyCapacity)
	_, hungerOK := capacityInteger(obs.Hunger, 0, reproductionHungerCapacity)
	bag, bagOK := capacityInteger(obs.BagUnits, 0, reproductionBagCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, reproductionKMax)
	wip, wipOK := capacityInteger(obs.Wip, 0, reproductionWipMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, reproductionGranaryMax)
	_, investedOK := capacityInteger(obs.InvestedUnits, 0, reproductionHorizonHours)
	lastBuild, lastBuildOK := capacityInteger(obs.LastBuildHour, reproductionNeverHour, reproductionHorizonHours-1)
	lastMeal, lastMealOK := capacityInteger(obs.LastStoredMealHour, reproductionNeverHour, reproductionHorizonHours-1)
	if !energyOK || !hungerOK || !bagOK || !capitalOK || !wipOK || !granaryOK || !investedOK || !lastBuildOK || !lastMealOK ||
		lastBuild > int64(obs.Hour) || lastMeal > int64(obs.Hour) {
		choice.Fallback = ReproductionInvalidSelf
		return choice, nil
	}
	// The bag names its holder's home patch whether empty or held; a founder
	// must name its ID-placed patch, a newborn its proposer-placed patch.
	home, err := obs.BagSource.EntityRef()
	if err != nil || obs.BagSource.Kind() != sim.EntityRefKind || obs.BagSource.State() != sim.Present || !reproductionValidPatch(home) {
		choice.Fallback = ReproductionInvalidSelf
		return choice, nil
	}
	if expected, ok := reproductionFounderHomePatch(obs.Actor); ok && expected != home {
		choice.Fallback = ReproductionInvalidSelf
		return choice, nil
	}
	slots := append([]ReproductionSlotView(nil), obs.HomeSlots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].ID < slots[j].ID })
	first := reproductionFirstSlot(home)
	for i, view := range slots {
		if view.ID < first || view.ID >= first+reproductionSlotsPerPatch || (i > 0 && slots[i-1].ID == view.ID) {
			return ReproductionChoice{}, ErrInvalidReproductionObservation
		}
		if _, ok := capacityInteger(view.Stock, 0, 1); !ok {
			return ReproductionChoice{}, ErrInvalidReproductionObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if bag == 1 { // frozen bag decision: Build | Eat
		if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 {
			choice.Fallback = ReproductionBudgetExhausted
			return choice, nil
		}
		if capital+wip < reproductionKMax && (energy >= capacityPolicyTLow || granary >= 1) {
			choice.Kind, choice.Evaluated, choice.Fallback = ReproductionBuild, 1, ReproductionNoFallback
		} else {
			choice.Kind, choice.Evaluated, choice.Fallback = ReproductionEat, 1, ReproductionNoFallback
		}
		return choice, nil
	}
	if obs.LastDenial == ReproductionGatherDenied { // denied-gather fallback
		if granary >= 1 && lastMeal < int64(obs.Hour) {
			choice.Kind, choice.Evaluated, choice.Fallback = ReproductionEatStored, 1, ReproductionNoFallback
		} else {
			choice.Kind, choice.RestDuration, choice.Evaluated, choice.Fallback = ReproductionRest, ReproductionRestDuration, 1, ReproductionNoFallback
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
		choice.Fallback = ReproductionBudgetExhausted // all-or-nothing: never a sorted prefix
		return choice, nil
	}
	if len(candidates) > 0 {
		choice.Kind, choice.TargetSlot, choice.Evaluated, choice.Fallback = ReproductionGather, candidates[0], len(candidates), ReproductionNoFallback
		return choice, nil
	}
	if granary >= 1 && lastMeal < int64(obs.Hour) { // one stored meal per actor-hour at most
		choice.Kind, choice.Evaluated, choice.Fallback = ReproductionEatStored, 1, ReproductionNoFallback
	}
	return choice, nil
}

// ReproductionRequestStatus mirrors the world's request-row status values
// (world.ReproductionRequestStatus 0..4).
type ReproductionRequestStatus uint8

const (
	ReproductionRequestIdle     ReproductionRequestStatus = iota // 0: never filed
	ReproductionRequestPending                                   // 1
	ReproductionRequestAccepted                                  // 2
	ReproductionRequestRefused                                   // 3
	ReproductionRequestExpired                                   // 4
)

// ReproductionRequestGuard is the requester's own request-row guard. A
// pending request must be finalized, and the frozen one-request-per-actor-
// hour rule blocks any second filing in the same hour (world rule 409
// rejects LastRequestHour >= the current hour).
type ReproductionRequestGuard struct {
	Status          ReproductionRequestStatus
	LastRequestHour int64 // -1 while never filed
}

// ReproductionClaimOverride is an explicit test injection for the reported
// claims. It never changes the actor's own eligibility or addressee: those
// are decided from the observation's own private state, while the choice
// carries exactly the overridden reported values so claim-vs-truth
// divergence stays constructible and auditable.
type ReproductionClaimOverride struct{ Capital, Granary int64 }

// ReproductionRequestObservation is the actor-visible birth-request view at
// the frozen h+40m+5µs phase. It carries the actor's own private margins
// (energy, capital, granary stock), its own request guard, the public home
// patch, and the public same-patch roster — no other actor's state, no death
// hours, no debts, no streams, and no claim-vs-truth divergence data.
type ReproductionRequestObservation struct {
	Actor        sim.EntityID
	Ref          ReproductionRef
	Hour         int
	WorldVersion sim.WorldVersion
	HomePatch    sim.EntityID // the actor's home patch (its bag source row); public
	Energy       sim.Value
	Capital      sim.Value
	GranaryStock sim.Value
	OwnRequest   ReproductionRequestGuard
	PatchMates   []sim.EntityID // same-patch roster excluding self; runner-supplied public data
	TestClaims   *ReproductionClaimOverride
}

type ReproductionRequestAction uint8

const (
	ReproductionRequestWait ReproductionRequestAction = iota
	ReproductionRequestFile
)

// ReproductionRequestChoice files one addressed request. ReportedCapital and
// ReportedGranary are the CLAIM — the actor's reported values, honest
// self-reports of its own observed margins unless TestClaims overrides them
// — never an authoritative account of truth; the world's privileged audit
// alone compares claims against truth.
type ReproductionRequestChoice struct {
	Kind            ReproductionRequestAction
	Addressee       sim.EntityID
	ReportedCapital int64
	ReportedGranary int64
	ObservedVersion sim.WorldVersion
	Ref             ReproductionRef
	Evaluated       int
	Fallback        ReproductionFallback
}

// EvaluateBirthRequest is the frozen proposer decision at h+40m+5µs:
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
func (b *ReproductionBound) EvaluateBirthRequest(obs ReproductionRequestObservation) (ReproductionRequestChoice, error) {
	founderHome, isFounder := reproductionFounderHomePatch(obs.Actor)
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= reproductionHorizonHours ||
		obs.OwnRequest.Status > ReproductionRequestExpired ||
		obs.OwnRequest.LastRequestHour < reproductionNeverHour || obs.OwnRequest.LastRequestHour > int64(obs.Hour) ||
		(obs.OwnRequest.Status == ReproductionRequestIdle && obs.OwnRequest.LastRequestHour != reproductionNeverHour) ||
		(obs.OwnRequest.Status != ReproductionRequestIdle && obs.OwnRequest.LastRequestHour < 0) ||
		!reproductionValidPatch(obs.HomePatch) || (isFounder && founderHome != obs.HomePatch) ||
		(obs.TestClaims != nil && (obs.TestClaims.Capital < 0 || obs.TestClaims.Capital > reproductionKMax ||
			obs.TestClaims.Granary < 0 || obs.TestClaims.Granary > reproductionGranaryMax)) ||
		len(obs.PatchMates) > ReproductionMaxCandidates {
		return ReproductionRequestChoice{}, ErrInvalidReproductionObservation
	}
	choice := ReproductionRequestChoice{Kind: ReproductionRequestWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: ReproductionNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, reproductionEnergyCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, reproductionKMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, reproductionGranaryMax)
	if !energyOK || !capitalOK || !granaryOK {
		choice.Fallback = ReproductionInvalidSelf
		return choice, nil
	}
	mates := append([]sim.EntityID(nil), obs.PatchMates...)
	sort.Slice(mates, func(i, j int) bool { return mates[i] < mates[j] })
	for i, mate := range mates {
		if !validReproductionRosterEntity(mate) || mate == obs.Actor || (i > 0 && mates[i-1] == mate) {
			return ReproductionRequestChoice{}, ErrInvalidReproductionObservation
		}
		// Founder placement is public fixed geometry: a founder mate on a
		// different patch than the observed home patch is a malformed roster.
		// Newborn placement stays runner-supplied public data; the world
		// re-gates same-patch on fresh truth at the reply.
		if home, ok := reproductionFounderHomePatch(mate); ok && home != obs.HomePatch {
			return ReproductionRequestChoice{}, ErrInvalidReproductionObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if obs.OwnRequest.Status == ReproductionRequestPending || obs.OwnRequest.LastRequestHour >= int64(obs.Hour) {
		return choice, nil // one request per actor-hour; a pending request resolves first
	}
	// The frozen proposer condition: the actor's own state must already sit
	// at the gate margins the runner will re-check on fresh truth.
	if capital < reproductionConsentMinCapital || granary < reproductionConsentMinGranary || energy < reproductionConsentMinEnergy {
		return choice, nil
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 ||
		len(mates) > b.policy.Budget.Candidates || len(mates) > b.policy.Budget.Evaluations {
		choice.Fallback = ReproductionBudgetExhausted
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
	claims := ReproductionClaimOverride{Capital: capital, Granary: granary} // honest self-report
	if obs.TestClaims != nil {
		claims = *obs.TestClaims
	}
	choice.Kind = ReproductionRequestFile
	choice.Addressee = addressee
	choice.ReportedCapital = claims.Capital
	choice.ReportedGranary = claims.Granary
	choice.Evaluated = len(mates)
	choice.Fallback = ReproductionNoFallback
	return choice, nil
}

// ReproductionPendingView is one pending birth request addressed to the
// observing consenter: the requester's public identity, its reported claims,
// and the public derived kin distance. It intentionally has no requester
// energy, granary, capital, bag, loci, birth/death hours, debt, or history
// fields — the consenter never sees requester truth, only the CLAIM.
type ReproductionPendingView struct {
	Requester       sim.EntityID
	Addressee       sim.EntityID
	Hour            int
	ReportedCapital int64
	ReportedGranary int64
	KinDistance     int  // public derived kin walk distance; 0 while unrelated
	KinRelated      bool // related within the runner's ≤10-reference walk
}

// ReproductionReplyObservation is the actor-visible birth-reply view at the
// frozen h+40m+6µs phase: the consenter's own private margins, its own
// one-reply-per-hour guard, and the pending addressed requests' claims and
// public kin distances. The kin distance is public because births are public
// events; the walk itself is the trusted runner's duty.
type ReproductionReplyObservation struct {
	Actor            sim.EntityID
	Ref              ReproductionRef
	Hour             int
	WorldVersion     sim.WorldVersion
	Energy           sim.Value
	Capital          sim.Value
	GranaryStock     sim.Value
	OwnLastReplyHour int64 // typed world ledger guard; -1 until first reply
	Pending          []ReproductionPendingView
}

type ReproductionReplyAction uint8

const (
	ReproductionReplyNone ReproductionReplyAction = iota // no reply this hour
	ReproductionReplyAccept
	// ReproductionReplyRefuse is a typed refusal the world commits as rule
	// 411 (the two request rows only).
	ReproductionReplyRefuse
)

// ReproductionReplyReason is the typed consent-phase refusal taxonomy. The
// remaining world refusal reasons (structural, death-between-phases, birth
// cap, cross-patch, fresh-truth gate) are runner-side: the policy never sees
// the truth they are computed from.
type ReproductionReplyReason uint8

const (
	ReproductionReplyReasonNone          ReproductionReplyReason = iota
	ReproductionReplyReasonConsentOwn                            // the consenter's own state fails the consent formula
	ReproductionReplyReasonConsentClaims                         // the reported claims fail the consent formula
	ReproductionReplyReasonKin                                   // kin walk distance ≤ 2
)

// ReproductionReplyChoice is one reply intention; the runner places it at
// the frozen instant and the world rechecks everything.
type ReproductionReplyChoice struct {
	Kind            ReproductionReplyAction
	Requester       sim.EntityID
	Reason          ReproductionReplyReason
	ObservedVersion sim.WorldVersion
	Ref             ReproductionRef
	Evaluated       int
	Fallback        ReproductionFallback
}

// EvaluateBirthReply is the frozen consenter decision at h+40m+6µs. It sees
// its own private state, the public kin distance, and the requester's
// reported claims — never requester truth. The frozen consent conjunction in
// its exact order: own granary>=3 ∧ own k>=2 ∧ own energy>=3 ∧ reported
// k>=2 ∧ reported granary>=3 ∧ (kin-distance>=3 ∨ unrelated). Addressed
// requests are evaluated in ascending requester order; the first acceptable
// one is accepted, otherwise the first (lowest-ID) is refused with its typed
// reason — one reply per actor-hour. Structural mismatches are observation
// errors; the surplus gate on fresh truth is the runner's, not this policy's.
func (b *ReproductionBound) EvaluateBirthReply(obs ReproductionReplyObservation) (ReproductionReplyChoice, error) {
	if b == nil || obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= reproductionHorizonHours ||
		obs.OwnLastReplyHour < reproductionNeverHour || obs.OwnLastReplyHour > int64(obs.Hour) ||
		len(obs.Pending) > ReproductionMaxCandidates {
		return ReproductionReplyChoice{}, ErrInvalidReproductionObservation
	}
	choice := ReproductionReplyChoice{Kind: ReproductionReplyNone, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: ReproductionNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, reproductionEnergyCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, reproductionKMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, reproductionGranaryMax)
	if !energyOK || !capitalOK || !granaryOK {
		choice.Fallback = ReproductionInvalidSelf
		return choice, nil
	}
	pending := append([]ReproductionPendingView(nil), obs.Pending...)
	sort.Slice(pending, func(i, j int) bool { return pending[i].Requester < pending[j].Requester })
	for i, view := range pending {
		if !validReproductionRosterEntity(view.Requester) || view.Requester == obs.Actor ||
			view.Addressee != obs.Actor || view.Hour != obs.Hour ||
			view.ReportedCapital < 0 || view.ReportedCapital > reproductionKMax ||
			view.ReportedGranary < 0 || view.ReportedGranary > reproductionGranaryMax ||
			(view.KinRelated && (view.KinDistance < 1 || view.KinDistance > reproductionKinWalkLimit)) ||
			(!view.KinRelated && view.KinDistance != 0) ||
			(i > 0 && pending[i-1].Requester == view.Requester) {
			return ReproductionReplyChoice{}, ErrInvalidReproductionObservation
		}
	}
	if energy == 0 || obs.OwnLastReplyHour >= int64(obs.Hour) || len(pending) == 0 {
		return choice, nil // a dead consenter never acts; one reply per actor-hour
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 ||
		len(pending) > b.policy.Budget.Candidates || len(pending) > b.policy.Budget.Evaluations {
		choice.Fallback = ReproductionBudgetExhausted
		return choice, nil
	}
	ownOK := granary >= reproductionConsentMinGranary && capital >= reproductionConsentMinCapital && energy >= reproductionConsentMinEnergy
	consentReason := func(view ReproductionPendingView) ReproductionReplyReason {
		switch {
		case !ownOK:
			return ReproductionReplyReasonConsentOwn
		case view.ReportedCapital < reproductionConsentMinCapital || view.ReportedGranary < reproductionConsentMinGranary:
			return ReproductionReplyReasonConsentClaims
		case view.KinRelated && view.KinDistance <= reproductionKinProhibitedDepth:
			return ReproductionReplyReasonKin
		}
		return ReproductionReplyReasonNone
	}
	examined := 0
	for _, view := range pending {
		examined++
		if consentReason(view) == ReproductionReplyReasonNone {
			choice.Kind = ReproductionReplyAccept
			choice.Requester = view.Requester
			choice.Evaluated = examined
			choice.Fallback = ReproductionNoFallback
			return choice, nil
		}
	}
	choice.Kind = ReproductionReplyRefuse
	choice.Requester = pending[0].Requester
	choice.Reason = consentReason(pending[0])
	choice.Evaluated = examined
	choice.Fallback = ReproductionNoFallback
	return choice, nil
}
