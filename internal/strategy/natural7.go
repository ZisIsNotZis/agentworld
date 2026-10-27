package strategy

import (
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

// natural7Claim is the shared natural/7 | natural/7b claim cascade entry point.
// The provision flag is fixed by the bound identity at the entry points above —
// never by observation content — so each frozen ref has exactly one
// deterministic cascade. The cascade body lives in natural_cognitive.go
// (naturalClaimCascade); this entry passes the frozen thresholds, so the
// frozen refs' behaviour is byte-identical.
func natural7Claim(b *NaturalBound, obs NaturalObservation, provision bool) (NaturalChoice, error) {
	return naturalClaimCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, provision, frozenNaturalClaimThresholds)
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
	return naturalBirthRequestCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, naturalPolicyTFileV6, naturalAddresseeSuccessor)
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
	return natural7BirthReplyAt(b, obs, naturalPolicyTFileV6, false)
}

// natural7BirthReplyAt is the shared v6-verbatim birth-reply cascade with the
// consenter's own consent conjunction supplied by the caller. The frozen refs
// pass naturalPolicyTFileV6 = 4 and the full own wealth gate; natural/13 passes
// the household gate 2, where only the consenter's energy matters.
func natural7BirthReplyAt(b *NaturalBound, obs NaturalReplyObservation, acceptFloor int64, household bool) (NaturalReplyChoice, error) {
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
	ownOK := energy >= acceptFloor
	if !household {
		ownOK = granary >= naturalConsentMinGranary && capital >= naturalConsentMinCapital && ownOK
	}
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
