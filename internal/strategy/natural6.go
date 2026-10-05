package strategy

import (
	"agentworld/internal/sim"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

// Natural v6 policy lane (ticket 16). The frozen design in
// .scratch/16-surplus-behaviour/spec.md is binding: the v6 bag decision and
// the birth energy floor below are quoted from it, and everything else is the
// frozen v5 cascade of natural.go inherited verbatim — the same shared
// observation, view, and choice types with the same field sets, the same
// structural validation, memoryless evaluation, the same budget semantics,
// and the same frozen runner phase placement. v6 runs on the unchanged v5
// world; no mechanism, constant, conservation, or checkpoint-format change.
//
// # The two v6 changes (quoted from the frozen design)
//
// Bag decision (h+20m+2µs), replacing the v5 floor (energy >= 5 or granary
// >= 1): Build iff k+wip < 8 AND energy >= 3 AND (granary >= 1 OR wip >= 1
// OR energy >= 4), else Eat. Birth filing (h+40m+5µs) and accept (h+40m+6µs)
// only at own energy >= 4 (was >= 3); every other frozen margin, the consent
// conjunction, the addressee rotation, and claim honesty stay verbatim v5.
//
// The observable delta set against natural/5 is differential-tested per cell
// in natural6_test.go: (a) E in {1,2} and granary >= 1 flips Build -> Eat;
// (b) E = 3, granary = 0, wip >= 1 flips Eat -> Build; (c) E = 4, granary =
// 0, wip = 0 flips Eat -> Build; and (d) E = 4, granary = 0, wip >= 1 flips
// Eat -> Build — the rule-implied family the frozen design's delta
// enumeration missed: its TFinish = 3 reading ("a started point has one hour
// left and a sunk unit; finish it if you stay above the reserve") fires the
// wip leg at every E >= 3, so E = 4 with outstanding wip builds through both
// the finish and the start leg (recorded supervisor ruling, ticket 16).
// Everything else is identical, including the descent family E >= 5: with or
// without wip both policies build there, and at k+wip >= 8 both eat.
//
// # Thresholds are derived, not declared (frozen design)
//
// Reserve R* = 2 (a fed body holds 2 forever: the pulse takes 1, a meal
// returns it; from 1 the next pulse kills). TFloor = TFinish = 3 = R*+1
// ("never invest your gathered unit unless one unfed hour above the reserve
// remains"; "finish it if you stay above the reserve" — one shared guard,
// two readings, hence one value declared twice below). TStart = TFile = 4 =
// R*+2 ("a point costs two unfed hours from an empty larder — pay only from
// reserve plus the full price"; "the birth price is four units per parent").
// The fed band [3,4] with granary >= 1 reads against the world's own frozen
// workhouse threshold 5 ("the world feeds any worker below five from the
// larder"); the descent clause E >= 5 is kept verbatim from v5.
const (
	NaturalPolicyFormatV6 uint32 = 6

	naturalPolicyTFloorV6  int64 = 3 // R*+1: the shared v6 investment guard (and TFinish)
	naturalPolicyTFinishV6 int64 = 3 // R*+1: finish the sunk point — the wip leg's floor
	naturalPolicyTStartV6  int64 = 4 // R*+2: re-pioneer from an empty larder
	naturalPolicyTFileV6   int64 = 4 // R*+2: the birth filing/accept own-energy floor
)

// FrozenNaturalPolicyV6 constructs the declared v6 policy, not a tunable
// family: format 6, ref {natural,6}, budget 32/32. As in v5, every actor of
// the cap-free roster binds the single frozen ref its world configured; the
// genome, not the policy ref, carries heredity.
func FrozenNaturalPolicyV6() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV6,
		Ref:           NaturalRef{ID: "natural", Version: 6},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicyV6 mirrors validNaturalPolicy for the v6 ref: the
// declared format and ref with the same budget-ceiling semantics, so a
// same-ref budget variant stays registrable and fails closed only at
// restore — VerifyFrozenNaturalPolicy compares the complete wire content
// against the executable's frozen policy.
func validNaturalPolicyV6(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicyV6()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// ValidFrozenNaturalPolicy reports whether p is exactly the frozen content
// of a policy in the executable's frozen natural ref set {natural/5,
// natural/6} — the declared identity, the format matching the ref version,
// and the frozen 32/32 budget. This is the ref-set predicate for the D1
// policy-selection seam; v5's own validation (validNaturalPolicy) is
// untouched and stays v5-only, so a v5-only executable keeps rejecting the
// v6 ref everywhere it validated policy content before.
func ValidFrozenNaturalPolicy(p NaturalPolicy) bool {
	return p == FrozenNaturalPolicy() || p == FrozenNaturalPolicyV6()
}

// FrozenNaturalPolicyForRef resolves a reference of the frozen natural ref
// set {natural/5, natural/6} to its frozen content, with the format matching
// the ref version. Any other reference — {natural,7}, a foreign ID, or the
// zero ref — fails closed with ErrUnknownNaturalPolicy. This is the single
// resolution point for the seam's binding site: the world resolves its
// configured policy ref here, registers the result with
// RegisterFrozenNatural, and binds every actor by the same ref.
func FrozenNaturalPolicyForRef(ref NaturalRef) (NaturalPolicy, error) {
	if ref == FrozenNaturalPolicy().Ref {
		return FrozenNaturalPolicy(), nil
	}
	if ref == FrozenNaturalPolicyV6().Ref {
		return FrozenNaturalPolicyV6(), nil
	}
	return NaturalPolicy{}, ErrUnknownNaturalPolicy
}

// RegisterFrozenNatural registers one policy of the frozen natural ref set
// under its own ref, applying the same per-ref shape validation the frozen
// Register applies to v5 (declared format and ref, budget within the frozen
// ceiling). It is the v6-aware registration path for the policy-selection
// seam: the frozen v5 Register keeps its v5-only validation untouched and
// continues to reject the v6 ref. Registering both frozen refs into one
// registry is supported; Bind still resolves exactly the bound ref.
func (r *NaturalRegistry) RegisterFrozenNatural(policy NaturalPolicy) error {
	if r == nil || !(validNaturalPolicy(policy) || validNaturalPolicyV6(policy)) {
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

// frozenNaturalV6Bound reports whether the bound carries the frozen v6
// identity. The v6 cascade refuses to evaluate under any other bound
// content: a bound v5 policy must never take the v6 path. The converse
// guard cannot live in the frozen v5 methods (their bytes are frozen), so
// the seam must call the matching cascade for the bound ref — the digest-
// identity, fingerprint-pin, and bidirectional-mismatch tests of the seam
// lane pin that discipline end to end.
func frozenNaturalV6Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV6 &&
		b.policy.Ref == FrozenNaturalPolicyV6().Ref
}

// EvaluateV6 is the frozen v6 hourly decision cascade: the v5 Evaluate
// verbatim — claim cascade, denial fallbacks, at most one stored meal per
// actor-hour, InvalidSelf degradation, structural errors — except the bag
// decision, which builds iff k+wip < 8 AND energy >= 3 AND (granary >= 1 OR
// wip >= 1 OR energy >= 4), else eats. The bound must carry the frozen v6
// identity or the evaluation fails closed with ErrInvalidNaturalPolicy.
func (b *NaturalBound) EvaluateV6(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNaturalV6Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	if obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
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
	if bag == 1 { // frozen v6 bag decision: Build | Eat
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
		// The v6 rule, frozen code. The shared guard energy >= TFloor (= 3)
		// serves both the survival floor and the finish leg: TFinish = TFloor
		// = 3, so `wip >= 1 && energy >= TFinish` under this guard is exactly
		// the frozen rule text's wip leg at every E >= 3. The start leg fires
		// at TStart = 4; the granary leg keeps the world's fed-larder clause.
		if capital+wip < naturalKMax && energy >= naturalPolicyTFloorV6 &&
			(granary >= 1 || (wip >= 1 && energy >= naturalPolicyTFinishV6) || energy >= naturalPolicyTStartV6) {
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

// EvaluateBirthRequestV6 is the frozen v6 proposer decision at h+40m+5µs:
// the v5 EvaluateBirthRequest verbatim — living, free request slot, one
// request per actor-hour, the deterministic same-patch addressee rotation,
// honest self-reported claims with the explicit test override — except the
// own-energy filing floor, raised to TFile = 4 (was naturalConsentMinEnergy
// = 3); the capital and granary margins stay verbatim. The bound must carry
// the frozen v6 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthRequestV6(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNaturalV6Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	if obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
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
	// The frozen v6 proposer condition: the v5 margins with the own-energy
	// filing floor raised to TFile = 4 — the actor's own state must already
	// sit at the gate margins the runner will re-check on fresh truth. The
	// capital and granary margins are verbatim v5 (world.NaturalConsentMin-
	// Capital/Granary); a reported claim never rescues own eligibility.
	if capital < naturalConsentMinCapital || granary < naturalConsentMinGranary || energy < naturalPolicyTFileV6 {
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

// EvaluateBirthReplyV6 is the frozen v6 consenter decision at h+40m+6µs: the
// v5 EvaluateBirthReply verbatim — ascending requester order, first
// acceptable pending accepted, lowest-ID refusal with its typed reason, one
// reply per actor-hour, claims-only view of the requester — except the
// own-energy accept floor, raised to TFile = 4 (was naturalConsentMinEnergy
// = 3). The reported-claims margins (k >= 2, granary >= 3), the kin clause,
// and the consent-reason precedence stay verbatim. The bound must carry the
// frozen v6 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthReplyV6(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNaturalV6Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	if obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
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
	// The frozen consent conjunction with the v6 own-energy accept floor:
	// own granary >= 3 ∧ own k >= 2 ∧ own energy >= 4 (was 3); the reported-
	// claims margins and the kin clause are verbatim v5.
	ownOK := granary >= naturalConsentMinGranary && capital >= naturalConsentMinCapital && energy >= naturalPolicyTFileV6
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

// # Format-6 wire (codec untouched)
//
// The natural policy wire is the untouched AWNP framing of natural_codec.go:
// the complete frozen content (format, ref identity, budget) in fixed
// big-endian order with exact framing, 22 bytes for the frozen refs. The
// functions below are the ref-set-aware twins: identical framing, the
// validation set extended to the frozen ref set {natural/5, natural/6}. A
// v5-only executable keeps refusing v6 bytes because EncodeNaturalPolicy,
// DecodeNaturalPolicy, and VerifyNaturalPolicy validate v5 content only and
// stay byte-frozen.

// appendNaturalPolicyWire frames p with the untouched natural framing. The
// framing is mirrored from natural_codec.go; byte identity for v5 content is
// pinned by test (TestNaturalV6Format6Codec cross-checks both encoders).
func appendNaturalPolicyWire(p NaturalPolicy) []byte {
	out := make([]byte, 0, naturalPolicyMaxBytes)
	out = append(out, naturalPolicyMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, p.FormatVersion)
	out = append(out, byte(len(p.Ref.ID)))
	out = append(out, p.Ref.ID...)
	out = binary.BigEndian.AppendUint32(out, p.Ref.Version)
	out = append(out, byte(p.Budget.Candidates), byte(p.Budget.Evaluations))
	return out
}

// EncodeFrozenNaturalPolicy encodes a policy of the frozen natural ref set
// in the canonical natural wire. Same-ref budget variants encode too,
// mirroring EncodeNaturalPolicy's semantics; VerifyFrozenNaturalPolicy
// rejects variant restorations against the frozen expectation.
func EncodeFrozenNaturalPolicy(p NaturalPolicy) ([]byte, error) {
	if !validNaturalPolicy(p) && !validNaturalPolicyV6(p) {
		return nil, ErrInvalidNaturalPolicy
	}
	return appendNaturalPolicyWire(p), nil
}

// DecodeFrozenNaturalPolicy accepts only complete, bounded, canonical
// natural content of the frozen ref set. An old v5-only executable refuses
// the same bytes: its canonical check re-encodes through the v5-only
// validation, so a v6 wire can never decode there.
func DecodeFrozenNaturalPolicy(data []byte) (NaturalPolicy, error) {
	const minBytes = 4 + 4 + 1 + 1 + 4 + 1 + 1
	if len(data) < minBytes || len(data) > naturalPolicyMaxBytes || !bytes.Equal(data[:4], naturalPolicyMagic[:]) {
		return NaturalPolicy{}, ErrInvalidNaturalPolicyEncoding
	}
	length := int(data[8])
	if length < 1 || length > MaxRefIDBytes || len(data) != minBytes-1+length {
		return NaturalPolicy{}, ErrInvalidNaturalPolicyEncoding
	}
	refEnd := 9 + length
	p := NaturalPolicy{
		FormatVersion: binary.BigEndian.Uint32(data[4:8]),
		Ref: NaturalRef{
			ID:      string(data[9:refEnd]),
			Version: binary.BigEndian.Uint32(data[refEnd : refEnd+4]),
		},
		Budget: NaturalBudget{
			Candidates:  int(data[refEnd+4]),
			Evaluations: int(data[refEnd+5]),
		},
	}
	canonical, err := EncodeFrozenNaturalPolicy(p)
	if err != nil || !bytes.Equal(data, canonical) {
		return NaturalPolicy{}, ErrInvalidNaturalPolicyEncoding
	}
	return p, nil
}

// VerifyFrozenNaturalPolicy fails closed on malformed bytes and same-ref
// semantic drift for either frozen ref, including a budget that differs from
// the frozen 32/32. The expected policy must be exactly the frozen content
// of a ref-set member from the current executable, never from the checkpoint
// being verified.
func VerifyFrozenNaturalPolicy(data []byte, expected NaturalPolicy) error {
	if !ValidFrozenNaturalPolicy(expected) {
		return ErrInvalidNaturalPolicy
	}
	actual, err := DecodeFrozenNaturalPolicy(data)
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrNaturalPolicyMismatch
	}
	return nil
}

// FrozenNaturalPolicyFingerprint fingerprints either frozen policy's wire.
// For natural/5 content it equals NaturalPolicyFingerprint (pinned by test).
func FrozenNaturalPolicyFingerprint(p NaturalPolicy) ([sha256.Size]byte, error) {
	encoded, err := EncodeFrozenNaturalPolicy(p)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}
