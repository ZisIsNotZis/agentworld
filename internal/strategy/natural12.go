package strategy

// Natural v12 policy lane (ticket 47). Policy natural/12 (format 12, ref
// {natural,12}, budget 32/32) is natural/11 VERBATIM except for exactly one
// behavioural difference: the birth-request addressee rule.
//
// Ticket 46 measured cell f10-s0 and found natural/11's rotation blind in a
// second way: 50 of its 150 filings were refused as kin, because the proposer's
// observation carried only the same-patch IDs while the reply side already
// received the PUBLIC kin walk. natural/12 keeps every other choice — the
// claim cascade with the frozen thresholds and provision enabled, the
// birth-request margins and own-energy filing floor TFile = 4, and the natural/7
// birth-reply cascade — and replaces only the addressee with the deterministic
// rotation over the NON-KIN mates
//
//	addressee = nonKin[hour % len(nonKin)]
//
// over the MateViews (one entry per PatchMates entry, same ascending order)
// whose public walk reports KinRelated false. With no non-kin mate the policy
// waits instead of filing a request the world must refuse. The observation's
// kin view fails closed (ErrInvalidNaturalObservation) unless it matches the
// roster entry for entry. No change to claims, floors, or budgets.
const NaturalPolicyFormatV12 uint32 = 12 // natural/12: natural/11 with the non-kin addressee

// FrozenNaturalPolicy12 constructs the declared natural/12 policy, not a
// tunable family: format 12, ref {natural,12}, budget 32/32. As in every
// frozen natural ref, every actor of the cap-free roster binds the single
// frozen ref its world configured; the genome, not the policy ref, carries
// heredity.
func FrozenNaturalPolicy12() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV12,
		Ref:           NaturalRef{ID: "natural", Version: 12},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicy12 mirrors validNaturalPolicy for the natural/12 ref: the
// declared format and ref with the same budget-ceiling semantics, so a
// same-ref budget variant stays registrable and fails closed only at restore —
// VerifyFrozenNaturalPolicy compares the complete wire content against the
// executable's frozen policy.
func validNaturalPolicy12(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy12()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// frozenNatural12Bound reports whether the bound carries the frozen natural/12
// identity. The 12 cascade refuses to evaluate under any other bound content;
// as in v6/v7/v9/v11, the frozen earlier methods cannot carry the converse
// guard (their bytes are frozen), so the seam must call the matching cascade
// for the bound ref.
func frozenNatural12Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV12 &&
		b.policy.Ref == FrozenNaturalPolicy12().Ref
}

// Evaluate12 is the frozen natural/12 hourly decision cascade: the natural/7
// (and natural/11) Evaluate verbatim — structural validation, InvalidSelf
// degradation, the bag decision (3/3/4/4), the denied-gather fallback, at most
// one stored meal per actor-hour, budget semantics, provision enabled with the
// frozen thresholds. The bound must carry the frozen natural/12 identity or
// the evaluation fails closed with ErrInvalidNaturalPolicy.
func (b *NaturalBound) Evaluate12(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNatural12Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalClaimCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, true, frozenNaturalClaimThresholds)
}

// EvaluateBirthRequest12 is the frozen natural/12 proposer decision at
// h+40m+5µs: the natural/11 EvaluateBirthRequest verbatim — living, free
// request slot, one request per actor-hour, honest self-reported claims, the
// capital and granary margins verbatim v5, and the own-energy filing floor
// TFile = 4 — except the addressee, which rotates with the hour over the
// non-kin mates of the public kin walk and waits when there are none. The bound
// must carry the frozen natural/12 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthRequest12(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural12Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalBirthRequestCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, naturalPolicyTFileV6, naturalAddresseeNonKin)
}

// EvaluateBirthReply12 is the frozen natural/12 consenter decision at
// h+40m+6µs: the natural/7 birth-reply cascade verbatim — ascending requester
// order, first acceptable pending accepted, lowest-ID refusal with its typed
// reason, one reply per actor-hour, claims-only view of the requester, and the
// own-energy accept floor TFile = 4. The bound must carry the frozen natural/12
// identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthReply12(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural12Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReply(b, obs)
}
