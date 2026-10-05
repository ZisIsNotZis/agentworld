package strategy

import (
	"agentworld/internal/sim"
	"sort"
)

// Natural v7 policy lane (ticket 17 L2). The frozen design in
// .scratch/17-surplus-mechanism/spec.md (including the recorded corrigenda)
// is binding. It arms two policies on the unchanged v5 observation and view
// types:
//
//	natural/7 (format 7, ref {natural,7}, budget 32/32) — the v6 cascade
//	  with exactly one new claim affordance: rule-515 provision, the policy
//	  side of the v7 mechanism change ("a meal taken as worksite provision
//	  instead of eaten").
//	natural/7b (format 8, ref {natural,8}, budget 32/32) — the predeclared
//	  mechanism-necessity ablation: the same banking floors with the
//	  provision choice DISABLED, so 7b exercises only v5 affordances. It is
//	  the self-falsifier of the design: if (v6,n/7b) already flips the A2
//	  band that (v6,n/6) leaves out, the M2 claim is falsified, the
//	  mechanism was unnecessary, and v7 must be re-cut as a policy ticket on
//	  the unchanged v5 world. Giving the ablation its own frozen ref keeps
//	  the design's single-variable ladder expressible: (v6,n/6) vs
//	  (v6,n/7b) isolates the banking floors, (v6,n/7b) vs (v6,n/7) isolates
//	  provisioning — the predicted flip step.
//
// Everything else is the frozen v6 cascade of natural6.go inherited
// verbatim — the same shared observation, view, and choice types with the
// same field sets (no new observation field: the provision choice is one new
// action constant, not a new channel), the same structural validation,
// memoryless evaluation, the same budget semantics, and the same frozen
// runner phase placement. The world's rule 515 enforces the once-per-actor-
// hour draw discipline on provisions by sharing the LastStoredMealHour
// guard; the policy cascade below is exactly the design's text.
//
// # The natural/7 claim cascade (bag = 0, quoted from the frozen design)
//
// In order: Gather if a stocked home slot is visible; else Provision iff
// granary >= 1 AND energy >= 4; else EatStored iff granary >= 1 AND
// energy <= 3 AND LastStoredMealHour < hour; else Wait. The denied-gather
// fallback is VERBATIM v6 — provisioning is deliberately absent from the
// denial path, and the denial fallback keeps drawing the bank at any energy
// exactly as v6 does. The bag decision is verbatim v6 (Build iff k+wip < 8
// AND energy >= 3 AND (granary >= 1 OR wip >= 1 OR energy >= 4), else Eat;
// constants 3/3/4/4), and the birth filing/accept margins are verbatim v6
// (own energy >= 4).
//
// # Thresholds are derived, not declared (frozen design)
//
// Reserve R* = 2. The provision floor 4 = R*+2 ("pay the worksite only from
// the reserve plus the full price" — after the pulse the body holds >= 3 =
// R*+1). The bank floor 3 = R*+1 ("the bank is food only at the survival
// floor"): above it a stocked larder funds the worksite or nothing, never
// the body.
const (
	NaturalPolicyFormatV7  uint32 = 7 // natural/7: the provision candidate
	NaturalPolicyFormatV7b uint32 = 8 // natural/7b: the ablation, ref {natural,8}

	// naturalPolicyTProvisionV7 = R*+2 = 4: provision only from the reserve
	// plus the full price; the same value as the frozen v6 TStart/TFile,
	// derived independently by the design's reserve arithmetic.
	naturalPolicyTProvisionV7 int64 = 4
	// naturalPolicyTBankV7 = R*+1 = 3: the bank is food only at the survival
	// floor; the same value as the frozen v6 TFloor/TFinish, derived
	// independently by the design's reserve arithmetic.
	naturalPolicyTBankV7 int64 = 3
)

// FrozenNaturalPolicy7 constructs the declared natural/7 policy, not a
// tunable family: format 7, ref {natural,7}, budget 32/32. As in v5/v6,
// every actor of the cap-free roster binds the single frozen ref its world
// configured; the genome, not the policy ref, carries heredity.
func FrozenNaturalPolicy7() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV7,
		Ref:           NaturalRef{ID: "natural", Version: 7},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// FrozenNaturalPolicy7b constructs the declared natural/7b ablation policy:
// format 8, ref {natural,8}, budget 32/32. Its content differs from
// natural/7 only in the disabled provision affordance, but it is a distinct
// frozen ref so the mechanism-necessity arm is a named policy in the
// experiment matrix, never a runtime switch on natural/7.
func FrozenNaturalPolicy7b() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV7b,
		Ref:           NaturalRef{ID: "natural", Version: 8},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicy7 mirrors validNaturalPolicy for the natural/7 ref: the
// declared format and ref with the same budget-ceiling semantics, so a
// same-ref budget variant stays registrable and fails closed only at
// restore — VerifyFrozenNaturalPolicy compares the complete wire content
// against the executable's frozen policy.
func validNaturalPolicy7(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy7()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// validNaturalPolicy7b mirrors validNaturalPolicy for the natural/7b ref.
func validNaturalPolicy7b(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy7b()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// frozenNatural7Bound reports whether the bound carries the frozen natural/7
// identity. The 7 cascade refuses to evaluate under any other bound content;
// frozenNatural7bBound is its 7b twin. As in v6, the frozen v5 methods
// cannot carry the converse guard (their bytes are frozen), so the seam must
// call the matching cascade for the bound ref.
func frozenNatural7Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV7 &&
		b.policy.Ref == FrozenNaturalPolicy7().Ref
}

func frozenNatural7bBound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV7b &&
		b.policy.Ref == FrozenNaturalPolicy7b().Ref
}

// Evaluate7 is the frozen natural/7 hourly decision cascade: the v6 Evaluate
// verbatim — structural validation, InvalidSelf degradation, the bag
// decision (3/3/4/4), the denied-gather fallback, at most one stored meal
// per actor-hour, budget semantics — except the claim cascade's bank legs,
// which become: Provision iff granary >= 1 AND energy >= 4; else EatStored
// iff granary >= 1 AND energy <= 3 AND LastStoredMealHour < hour; else Wait.
// The bound must carry the frozen natural/7 identity or the evaluation fails
// closed with ErrInvalidNaturalPolicy.
func (b *NaturalBound) Evaluate7(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNatural7Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7Claim(b, obs, true)
}

// Evaluate7b is the frozen natural/7b hourly decision cascade — the 7
// cascade with the provision affordance disabled: claim Gather if a stocked
// home slot is visible; else EatStored iff granary >= 1 AND energy <= 3 AND
// LastStoredMealHour < hour; else Wait. The banking floors still bite (the
// bank is refused above the survival floor), so the arm isolates exactly the
// provisioning conversion against natural/6. The bound must carry the frozen
// natural/7b identity or the evaluation fails closed.
func (b *NaturalBound) Evaluate7b(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNatural7bBound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7Claim(b, obs, false)
}

// natural7Claim is the shared natural/7 | natural/7b claim cascade. The
// provision flag is fixed by the bound identity at the entry points above —
// never by observation content — so each frozen ref has exactly one
// deterministic cascade. The structure below is the frozen v6 Evaluate
// verbatim up to the claim's bank legs.
func natural7Claim(b *NaturalBound, obs NaturalObservation, provision bool) (NaturalChoice, error) {
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
	if bag == 1 { // frozen v6 bag decision, verbatim: Build | Eat
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
		if capital+wip < naturalKMax && energy >= naturalPolicyTFloorV6 &&
			(granary >= 1 || (wip >= 1 && energy >= naturalPolicyTFinishV6) || energy >= naturalPolicyTStartV6) {
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
	if len(candidates) > b.policy.Budget.Candidates || len(candidates) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted // all-or-nothing: never a sorted prefix
		return choice, nil
	}
	if len(candidates) > 0 {
		choice.Kind, choice.TargetSlot, choice.Evaluated, choice.Fallback = NaturalGather, candidates[0], len(candidates), NaturalNoFallback
		return choice, nil
	}
	// The natural/7 banking floors, in frozen order. Provision converts one
	// granary unit into the bag (rule 515; the world shares the once-per-
	// hour draw discipline); the bank is food only at the survival floor.
	if provision && granary >= 1 && energy >= naturalPolicyTProvisionV7 {
		choice.Kind, choice.Evaluated, choice.Fallback = NaturalProvision, 1, NaturalNoFallback
		return choice, nil
	}
	if granary >= 1 && energy <= naturalPolicyTBankV7 && lastMeal < int64(obs.Hour) { // one stored meal per actor-hour at most
		choice.Kind, choice.Evaluated, choice.Fallback = NaturalEatStored, 1, NaturalNoFallback
	}
	return choice, nil
}

// EvaluateBirthRequest7 is the frozen natural/7 proposer decision at
// h+40m+5µs: the v6 EvaluateBirthRequestV6 verbatim — living, free request
// slot, one request per actor-hour, the deterministic same-patch addressee
// rotation, honest self-reported claims with the explicit test override, the
// capital and granary margins verbatim v5, and the own-energy filing floor
// TFile = 4. The bound must carry the frozen natural/7 identity or the
// evaluation fails closed.
func (b *NaturalBound) EvaluateBirthRequest7(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural7Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthRequest(b, obs)
}

// EvaluateBirthRequest7b is the frozen natural/7b proposer decision:
// identical to natural/7 (the ablation changes only the claim cascade). The
// bound must carry the frozen natural/7b identity or the evaluation fails
// closed.
func (b *NaturalBound) EvaluateBirthRequest7b(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural7bBound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthRequest(b, obs)
}

// natural7BirthRequest is the shared v6-verbatim birth-request cascade for
// both 7 identities (structure of EvaluateBirthRequestV6, same margins).
func natural7BirthRequest(b *NaturalBound, obs NaturalRequestObservation) (NaturalRequestChoice, error) {
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
	// filing floor TFile = 4 — the actor's own state must already sit at the
	// gate margins the runner will re-check on fresh truth. The capital and
	// granary margins are verbatim v5 (world.NaturalConsentMinCapital/
	// Granary); a reported claim never rescues own eligibility.
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

// EvaluateBirthReply7 is the frozen natural/7 consenter decision at
// h+40m+6µs: the v6 EvaluateBirthReplyV6 verbatim — ascending requester
// order, first acceptable pending accepted, lowest-ID refusal with its typed
// reason, one reply per actor-hour, claims-only view of the requester, and
// the own-energy accept floor TFile = 4. The bound must carry the frozen
// natural/7 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthReply7(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural7Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReply(b, obs)
}

// EvaluateBirthReply7b is the frozen natural/7b consenter decision:
// identical to natural/7 (the ablation changes only the claim cascade). The
// bound must carry the frozen natural/7b identity or the evaluation fails
// closed.
func (b *NaturalBound) EvaluateBirthReply7b(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural7bBound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReply(b, obs)
}

// natural7BirthReply is the shared v6-verbatim birth-reply cascade for both
// 7 identities (structure of EvaluateBirthReplyV6, same margins).
func natural7BirthReply(b *NaturalBound, obs NaturalReplyObservation) (NaturalReplyChoice, error) {
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
	// own granary >= 3 ∧ own k >= 2 ∧ own energy >= 4; the reported-claims
	// margins and the kin clause are verbatim v5.
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
