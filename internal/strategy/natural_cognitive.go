package strategy

import (
	"agentworld/internal/sim"
	"sort"
)

// Role B versioned behaviour data (spec sections 5 and 10). A cognitive
// generation is a *declared, typed, byte-pinned* strategy version: the frozen
// natural/7 (or natural/7b) cascade with a bounded integer parameter vector
// substituted at its declared decision points. It is data, never code, and it
// lives in a DISTINCT id/format space (NaturalCognitiveRefID /
// NaturalCognitiveFormatVersion) so no patched ref can ever collide with the
// frozen natural/5..11 refs or their formats. The all-parent-values vector
// reproduces the parent's decisions exactly (pinned by the reduction test).
//
// This file also hosts the parameterized claim and birth-request cascades
// shared with the frozen natural/7 lanes: natural7Claim and
// natural7BirthRequest delegate here with the frozen thresholds, so there is
// exactly one home for the cascade logic.

// NaturalCognitiveRefID is the distinct strategy-ref id of a cognitive
// generation. It is deliberately outside the frozen "natural" numeric space.
const NaturalCognitiveRefID = "natural-cog"

// NaturalCognitiveFormatVersion is the distinct strategy format of a
// cognitive generation, outside the frozen format set {5,6,7,8,9,11}.
const NaturalCognitiveFormatVersion uint32 = 100

// CognitiveParamID names one declared, patchable strategy-side parameter
// (Appendix C). The contract's home is this package (the owner of the
// versioned behaviour data); internal/cognition/protocol re-exports it.
type CognitiveParamID string

const (
	CognitiveParamClaimProvisionEnergyFloor CognitiveParamID = "claim.provision_energy_floor"
	CognitiveParamClaimBankEnergyCeiling    CognitiveParamID = "claim.bank_energy_ceiling"
	CognitiveParamClaimGatherEnergyFloor    CognitiveParamID = "claim.gather_energy_floor"
	CognitiveParamBagBuildEnergyFloor       CognitiveParamID = "bag.build_energy_floor"
	CognitiveParamBagBuildStartEnergyFloor  CognitiveParamID = "bag.build_start_energy_floor"
	CognitiveParamBagBuildFinishEnergyFloor CognitiveParamID = "bag.build_finish_energy_floor"
	CognitiveParamBirthFilingEnergyFloor    CognitiveParamID = "birth.filing_energy_floor"
)

// CognitiveParameter is one declared contract row (Appendix C).
type CognitiveParameter struct {
	ID            CognitiveParamID
	Meaning       string
	DecisionPoint string
	ParentValue   int64
	DomainMin     int64
	DomainMax     int64
}

// cognitiveParameterOrder is the canonical contract order; it fixes the
// canonical wire order of the parameter vector.
var cognitiveParameterOrder = []CognitiveParamID{
	CognitiveParamClaimProvisionEnergyFloor,
	CognitiveParamClaimBankEnergyCeiling,
	CognitiveParamClaimGatherEnergyFloor,
	CognitiveParamBagBuildEnergyFloor,
	CognitiveParamBagBuildStartEnergyFloor,
	CognitiveParamBagBuildFinishEnergyFloor,
	CognitiveParamBirthFilingEnergyFloor,
}

// CognitiveParameterContract returns the declared Appendix C rows in canonical
// order. The parent values are the frozen natural/7 thresholds (v6); the
// declared domains are derived from the mechanism's own bounds.
func CognitiveParameterContract() []CognitiveParameter {
	return []CognitiveParameter{
		{ID: CognitiveParamClaimProvisionEnergyFloor, Meaning: "minimum own energy to provision",
			DecisionPoint: "claim h+1us", ParentValue: naturalPolicyTProvisionV7, DomainMin: 1, DomainMax: 8},
		{ID: CognitiveParamClaimBankEnergyCeiling, Meaning: "maximum own energy at which the bank may be drawn as a meal",
			DecisionPoint: "claim h+1us", ParentValue: naturalPolicyTBankV7, DomainMin: 0, DomainMax: 8},
		{ID: CognitiveParamClaimGatherEnergyFloor, Meaning: "minimum own energy to claim a gather",
			DecisionPoint: "claim h+1us", ParentValue: 1, DomainMin: 1, DomainMax: 8},
		{ID: CognitiveParamBagBuildEnergyFloor, Meaning: "minimum own energy to invest the held unit",
			DecisionPoint: "bag decision h+20m+2us", ParentValue: naturalPolicyTFloorV6, DomainMin: 1, DomainMax: 8},
		{ID: CognitiveParamBagBuildStartEnergyFloor, Meaning: "minimum own energy to start a first point",
			DecisionPoint: "bag decision h+20m+2us", ParentValue: naturalPolicyTStartV6, DomainMin: 1, DomainMax: 8},
		{ID: CognitiveParamBagBuildFinishEnergyFloor, Meaning: "minimum own energy to finish a point in progress",
			DecisionPoint: "bag decision h+20m+2us", ParentValue: naturalPolicyTFinishV6, DomainMin: 1, DomainMax: 8},
		{ID: CognitiveParamBirthFilingEnergyFloor, Meaning: "minimum own energy to file a birth request",
			DecisionPoint: "birth request h+40m+5us", ParentValue: naturalPolicyTFileV6, DomainMin: 1, DomainMax: 8},
	}
}

// CognitiveParameterDomain returns the declared domain of one contract id.
func CognitiveParameterDomain(id CognitiveParamID) (min, max int64, ok bool) {
	for _, row := range CognitiveParameterContract() {
		if row.ID == id {
			return row.DomainMin, row.DomainMax, true
		}
	}
	return 0, 0, false
}

// NaturalCognitiveParams is the full substituted parameter vector: every
// declared contract parameter carries a value. The zero value is all zero,
// which is NOT the parent vector (use FrozenCognitiveParams for that).
type NaturalCognitiveParams struct {
	ClaimProvisionEnergyFloor int64
	ClaimBankEnergyCeiling    int64
	ClaimGatherEnergyFloor    int64
	BagBuildEnergyFloor       int64
	BagBuildStartEnergyFloor  int64
	BagBuildFinishEnergyFloor int64
	BirthFilingEnergyFloor    int64
}

// FrozenCognitiveParams returns the declared frozen natural/7 parent vector.
func FrozenCognitiveParams() NaturalCognitiveParams {
	return NaturalCognitiveParams{
		ClaimProvisionEnergyFloor: naturalPolicyTProvisionV7,
		ClaimBankEnergyCeiling:    naturalPolicyTBankV7,
		ClaimGatherEnergyFloor:    1,
		BagBuildEnergyFloor:       naturalPolicyTFloorV6,
		BagBuildStartEnergyFloor:  naturalPolicyTStartV6,
		BagBuildFinishEnergyFloor: naturalPolicyTFinishV6,
		BirthFilingEnergyFloor:    naturalPolicyTFileV6,
	}
}

// Value returns the value of one contract id.
func (p NaturalCognitiveParams) Value(id CognitiveParamID) (int64, bool) {
	switch id {
	case CognitiveParamClaimProvisionEnergyFloor:
		return p.ClaimProvisionEnergyFloor, true
	case CognitiveParamClaimBankEnergyCeiling:
		return p.ClaimBankEnergyCeiling, true
	case CognitiveParamClaimGatherEnergyFloor:
		return p.ClaimGatherEnergyFloor, true
	case CognitiveParamBagBuildEnergyFloor:
		return p.BagBuildEnergyFloor, true
	case CognitiveParamBagBuildStartEnergyFloor:
		return p.BagBuildStartEnergyFloor, true
	case CognitiveParamBagBuildFinishEnergyFloor:
		return p.BagBuildFinishEnergyFloor, true
	case CognitiveParamBirthFilingEnergyFloor:
		return p.BirthFilingEnergyFloor, true
	}
	return 0, false
}

// Values returns the vector as a map keyed by contract id.
func (p NaturalCognitiveParams) Values() map[CognitiveParamID]int64 {
	out := make(map[CognitiveParamID]int64, len(cognitiveParameterOrder))
	for _, id := range cognitiveParameterOrder {
		value, _ := p.Value(id)
		out[id] = value
	}
	return out
}

// WithValue returns a copy with one contract id substituted; ok is false for
// an unknown id.
func (p NaturalCognitiveParams) WithValue(id CognitiveParamID, value int64) (NaturalCognitiveParams, bool) {
	switch id {
	case CognitiveParamClaimProvisionEnergyFloor:
		p.ClaimProvisionEnergyFloor = value
	case CognitiveParamClaimBankEnergyCeiling:
		p.ClaimBankEnergyCeiling = value
	case CognitiveParamClaimGatherEnergyFloor:
		p.ClaimGatherEnergyFloor = value
	case CognitiveParamBagBuildEnergyFloor:
		p.BagBuildEnergyFloor = value
	case CognitiveParamBagBuildStartEnergyFloor:
		p.BagBuildStartEnergyFloor = value
	case CognitiveParamBagBuildFinishEnergyFloor:
		p.BagBuildFinishEnergyFloor = value
	case CognitiveParamBirthFilingEnergyFloor:
		p.BirthFilingEnergyFloor = value
	default:
		return p, false
	}
	return p, true
}

// InDomain reports whether every value of the vector lies in its declared
// domain.
func (p NaturalCognitiveParams) InDomain() bool {
	for _, row := range CognitiveParameterContract() {
		value, _ := p.Value(row.ID)
		if value < row.DomainMin || value > row.DomainMax {
			return false
		}
	}
	return true
}

// NaturalCognitiveGeneration is one declared, byte-pinned behaviour-logic
// version. It never names a version that is not exactly parent.Version+1, and
// it records the parent ref and fingerprint so lineage can be walked from the
// generation alone.
type NaturalCognitiveGeneration struct {
	Ref               NaturalRef // {ID: NaturalCognitiveRefID, Version: ParentRef.Version+1}
	BaseRef           NaturalRef // the frozen base the runner is configured with (natural/7 or natural/7b)
	Provision         bool       // the inherited provision affordance (false only for a natural/7b base)
	ParentRef         NaturalRef // exact parent (a frozen base or the previous generation)
	ParentFingerprint [32]byte
	Params            NaturalCognitiveParams
}

// IsCognitiveRef reports whether a ref names the distinct cognitive id space.
func IsCognitiveRef(ref NaturalRef) bool {
	return ref.ID == NaturalCognitiveRefID && ref.Version > 0
}

// IsCognitiveBase reports whether a ref is a frozen base the cognitive lane
// admits (natural/7 with provision, or natural/7b without).
func IsCognitiveBase(ref NaturalRef) bool {
	return ref == FrozenNaturalPolicy7().Ref || ref == FrozenNaturalPolicy7b().Ref
}

// BaseProvision reports the inherited provision affordance of a base ref.
func BaseProvision(base NaturalRef) bool {
	return base != FrozenNaturalPolicy7b().Ref
}

// NewNaturalCognitiveGeneration builds and validates a child generation from a
// parent ref and fingerprint. base and provision are the inherited root
// identity (a frozen base for the first generation, the head's own base for a
// chained one). The child version is exactly parent.Version+1.
func NewNaturalCognitiveGeneration(parent NaturalRef, parentFingerprint [32]byte, base NaturalRef, provision bool, params NaturalCognitiveParams) (NaturalCognitiveGeneration, error) {
	if !IsCognitiveBase(base) {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalPolicy
	}
	if parent != base && !IsCognitiveRef(parent) {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalPolicy
	}
	if parent.Version >= ^uint32(0) {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalPolicy
	}
	if !params.InDomain() {
		return NaturalCognitiveGeneration{}, ErrInvalidNaturalPolicy
	}
	return NaturalCognitiveGeneration{
		Ref:               NaturalRef{ID: NaturalCognitiveRefID, Version: parent.Version + 1},
		BaseRef:           base,
		Provision:         provision,
		ParentRef:         parent,
		ParentFingerprint: parentFingerprint,
		Params:            params,
	}, nil
}

// Validate re-checks a decoded generation's invariants.
func (g NaturalCognitiveGeneration) Validate() error {
	if !IsCognitiveRef(g.Ref) || !IsCognitiveBase(g.BaseRef) || !g.Params.InDomain() {
		return ErrInvalidNaturalPolicy
	}
	if g.ParentRef != g.BaseRef && !IsCognitiveRef(g.ParentRef) {
		return ErrInvalidNaturalPolicy
	}
	if g.Ref.Version != g.ParentRef.Version+1 {
		return ErrInvalidNaturalPolicy
	}
	return nil
}

// thresholds maps the parameter vector onto the claim cascade's decision
// points.
func (g NaturalCognitiveGeneration) thresholds() naturalClaimThresholds {
	return naturalClaimThresholds{
		ProvisionEnergyFloor:   g.Params.ClaimProvisionEnergyFloor,
		BankEnergyCeiling:      g.Params.ClaimBankEnergyCeiling,
		GatherEnergyFloor:      g.Params.ClaimGatherEnergyFloor,
		BuildEnergyFloor:       g.Params.BagBuildEnergyFloor,
		BuildStartEnergyFloor:  g.Params.BagBuildStartEnergyFloor,
		BuildFinishEnergyFloor: g.Params.BagBuildFinishEnergyFloor,
	}
}

// EvaluateClaim evaluates the patched claim + bag cascade for one actor. It
// stamps the frozen base ref so the runner's own ref discipline is unchanged;
// the generation identity lives in the cognitive state, the ledger, and the
// manifest cognition block.
func (g NaturalCognitiveGeneration) EvaluateClaim(patches int, actor sim.EntityID, obs NaturalObservation) (NaturalChoice, error) {
	if err := g.Validate(); err != nil {
		return NaturalChoice{}, err
	}
	id := naturalClaimIdentity{Actor: actor, Patches: patches, Ref: g.BaseRef, Budget: FrozenNaturalPolicy7().Budget}
	return naturalClaimCascade(id, obs, g.Provision, g.thresholds())
}

// EvaluateBirthRequest evaluates the patched proposer filing cascade for one
// actor (the declared birth.filing_energy_floor point).
func (g NaturalCognitiveGeneration) EvaluateBirthRequest(patches int, actor sim.EntityID, obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if err := g.Validate(); err != nil {
		return NaturalRequestChoice{}, err
	}
	id := naturalClaimIdentity{Actor: actor, Patches: patches, Ref: g.BaseRef, Budget: FrozenNaturalPolicy7().Budget}
	return naturalBirthRequestCascade(id, obs, g.Params.BirthFilingEnergyFloor, naturalAddresseeSuccessor)
}

// naturalClaimIdentity is the bound identity the claim/birth cascades validate
// against: the bound actor, patch count, the ref the decision observes, and
// the frozen budget.
type naturalClaimIdentity struct {
	Actor   sim.EntityID
	Patches int
	Ref     NaturalRef
	Budget  NaturalBudget
}

// naturalClaimThresholds are the parameterized decision points of the claim and
// bag cascade.
type naturalClaimThresholds struct {
	ProvisionEnergyFloor   int64
	BankEnergyCeiling      int64
	GatherEnergyFloor      int64
	BuildEnergyFloor       int64
	BuildStartEnergyFloor  int64
	BuildFinishEnergyFloor int64
}

// frozenNaturalClaimThresholds is the declared natural/7 parent vector.
var frozenNaturalClaimThresholds = naturalClaimThresholds{
	ProvisionEnergyFloor:   naturalPolicyTProvisionV7,
	BankEnergyCeiling:      naturalPolicyTBankV7,
	GatherEnergyFloor:      1,
	BuildEnergyFloor:       naturalPolicyTFloorV6,
	BuildStartEnergyFloor:  naturalPolicyTStartV6,
	BuildFinishEnergyFloor: naturalPolicyTFinishV6,
}

// naturalClaimCascade is the one home of the natural/7 claim + bag cascade:
// the frozen v6 Evaluate verbatim up to the claim's bank legs, with the
// decision thresholds supplied by the caller. natural7Claim delegates here
// with the frozen thresholds, so the frozen refs' behaviour is byte-identical.
func naturalClaimCascade(id naturalClaimIdentity, obs NaturalObservation, provision bool, th naturalClaimThresholds) (NaturalChoice, error) {
	if obs.Actor != id.Actor || obs.Ref != id.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours || obs.LastDenial > NaturalBuildDenied ||
		len(obs.HomeSlots) > naturalSlotsPerPatch {
		return NaturalChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalChoice{Kind: NaturalWait, ObservedVersion: obs.WorldVersion, Ref: id.Ref, Fallback: NaturalNoCandidate}
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
	if err != nil || obs.BagSource.Kind() != sim.EntityRefKind || obs.BagSource.State() != sim.Present || !naturalPatchEntity(id.Patches, home) {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	if expected, ok := naturalFounderHomePatch(id.Patches, obs.Actor); ok && expected != home {
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
	if bag == 1 { // frozen v6 bag decision, verbatim: Build | Eat
		if id.Budget.Candidates < 1 || id.Budget.Evaluations < 1 {
			choice.Fallback = NaturalBudgetExhausted
			return choice, nil
		}
		if obs.LastDenial == NaturalBuildDenied {
			// The world refused this hour's build (no free wild slot with
			// stock 0); report Build unavailable and fall back to the meal.
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEat, 1, NaturalNoFallback
			return choice, nil
		}
		if capital+wip < naturalKMax && energy >= th.BuildEnergyFloor &&
			(granary >= 1 || (wip >= 1 && energy >= th.BuildFinishEnergyFloor) || energy >= th.BuildStartEnergyFloor) {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalBuild, 1, NaturalNoFallback
		} else {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEat, 1, NaturalNoFallback
		}
		return choice, nil
	}
	if obs.LastDenial == NaturalGatherDenied { // denied-gather fallback, verbatim v6
		// Provisioning is deliberately absent from the denial path, and the
		// denial fallback draws the bank at any energy exactly as v6 does.
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
	if len(candidates) > id.Budget.Candidates || len(candidates) > id.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted // all-or-nothing: never a sorted prefix
		return choice, nil
	}
	if len(candidates) > 0 && energy >= th.GatherEnergyFloor {
		choice.Kind, choice.TargetSlot, choice.Evaluated, choice.Fallback = NaturalGather, candidates[0], len(candidates), NaturalNoFallback
		return choice, nil
	}
	// The banking floors, in frozen order. Provision converts one granary unit
	// into the bag (rule 515; the world shares the once-per-hour draw
	// discipline); the bank is food only at the survival floor.
	if provision && granary >= 1 && energy >= th.ProvisionEnergyFloor {
		choice.Kind, choice.Evaluated, choice.Fallback = NaturalProvision, 1, NaturalNoFallback
		return choice, nil
	}
	if granary >= 1 && energy <= th.BankEnergyCeiling && lastMeal < int64(obs.Hour) { // one stored meal per actor-hour at most
		choice.Kind, choice.Evaluated, choice.Fallback = NaturalEatStored, 1, NaturalNoFallback
	}
	return choice, nil
}

// naturalAddresseeRule declares how the proposer picks the same-patch mate it
// files the birth request to. The rule is fixed by the bound ref's cascade
// entry point — never by observation content — so each frozen ref has exactly
// one deterministic addressee sequence.
type naturalAddresseeRule int

const (
	// naturalAddresseeSuccessor is the frozen v5/v6/v7/v7b/v9 rule: the
	// lowest same-patch mate ID greater than the actor, else mates[0].
	naturalAddresseeSuccessor naturalAddresseeRule = iota
	// naturalAddresseeRotate is the natural/11 rule: mates[hour % len(mates)]
	// over the already-ascending-sorted same-patch roster.
	naturalAddresseeRotate
)

// naturalAddressee applies one declared addressee rule to the ascending-sorted
// non-empty mate roster.
func naturalAddressee(rule naturalAddresseeRule, mates []sim.EntityID, actor sim.EntityID, hour int) sim.EntityID {
	if rule == naturalAddresseeRotate {
		return mates[hour%len(mates)]
	}
	addressee := mates[0] // wraparound successor of the highest same-patch ID
	for _, mate := range mates {
		if mate > actor {
			addressee = mate
			break
		}
	}
	return addressee
}

// naturalBirthRequestCascade is the one home of the proposer filing cascade:
// the frozen v6 EvaluateBirthRequestV6 structure with the own-energy filing
// floor and the addressee rule supplied by the caller. natural7BirthRequest
// delegates here with the frozen floor and successor rule, so the frozen refs'
// behaviour is byte-identical; natural11BirthRequest supplies the rotation.
func naturalBirthRequestCascade(id naturalClaimIdentity, obs NaturalRequestObservation, filingFloor int64, rule naturalAddresseeRule) (NaturalRequestChoice, error) {
	if obs.Actor != id.Actor || obs.Ref != id.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours ||
		obs.OwnRequest.Status > NaturalRequestExpired ||
		obs.OwnRequest.LastRequestHour < naturalNeverHour || obs.OwnRequest.LastRequestHour > int64(obs.Hour) ||
		(obs.OwnRequest.Status == NaturalRequestIdle && obs.OwnRequest.LastRequestHour != naturalNeverHour) ||
		(obs.OwnRequest.Status != NaturalRequestIdle && obs.OwnRequest.LastRequestHour < 0) ||
		!naturalPatchEntity(id.Patches, obs.HomePatch) ||
		(obs.TestClaims != nil && (obs.TestClaims.Capital < 0 || obs.TestClaims.Capital > naturalKMax ||
			obs.TestClaims.Granary < 0 || obs.TestClaims.Granary > naturalGranaryStockMax)) {
		return NaturalRequestChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalRequestChoice{Kind: NaturalRequestWait, ObservedVersion: obs.WorldVersion, Ref: id.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, naturalKMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, naturalGranaryStockMax)
	if !energyOK || !capitalOK || !granaryOK {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	founderHome, isFounder := naturalFounderHomePatch(id.Patches, obs.Actor)
	if isFounder && founderHome != obs.HomePatch {
		return NaturalRequestChoice{}, ErrInvalidNaturalObservation
	}
	mates := append([]sim.EntityID(nil), obs.PatchMates...)
	sort.Slice(mates, func(i, j int) bool { return mates[i] < mates[j] })
	for i, mate := range mates {
		if !naturalRosterEntity(id.Patches, mate) || mate == obs.Actor || (i > 0 && mates[i-1] == mate) {
			return NaturalRequestChoice{}, ErrInvalidNaturalObservation
		}
		// Founder placement is public fixed geometry: a founder mate on a
		// different patch than the observed home patch is a malformed roster.
		// Newborn placement stays runner-supplied public data; the world
		// re-gates same-patch on fresh truth at the reply.
		if home, ok := naturalFounderHomePatch(id.Patches, mate); ok && home != obs.HomePatch {
			return NaturalRequestChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if obs.OwnRequest.Status == NaturalRequestPending || obs.OwnRequest.LastRequestHour >= int64(obs.Hour) {
		return choice, nil // one request per actor-hour; a pending request resolves first
	}
	// The frozen v6 proposer condition: the v5 margins with the own-energy
	// filing floor — the actor's own state must already sit at the gate margins
	// the runner will re-check on fresh truth. The capital and granary margins
	// are verbatim v5 (world.NaturalConsentMinCapital/Granary); a reported
	// claim never rescues own eligibility.
	if capital < naturalConsentMinCapital || granary < naturalConsentMinGranary || energy < filingFloor {
		return choice, nil
	}
	if id.Budget.Candidates < 1 || id.Budget.Evaluations < 1 ||
		len(mates) > id.Budget.Candidates || len(mates) > id.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted
		return choice, nil
	}
	if len(mates) == 0 {
		return choice, nil
	}
	addressee := naturalAddressee(rule, mates, obs.Actor, obs.Hour)
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
