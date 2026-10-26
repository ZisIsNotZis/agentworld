package strategy

// Natural v11 policy lane (ticket 45/46). Policy natural/11 (format 11, ref
// {natural,11}, budget 32/32) is natural/7 VERBATIM except for exactly one
// behavioural difference: the birth-request addressee rule.
//
// Ticket 45 measured cell f10-s0 and found that the frozen policy loses 133 of
// 135 birth opportunities because it always files to the same mate (the
// successor rule). natural/11 keeps every other choice — the claim cascade
// with the frozen thresholds and provision enabled, the birth-request margins
// and own-energy filing floor TFile = 4, and the natural/7 birth-reply
// cascade — and replaces only the addressee with the deterministic rotation
//
//	addressee = mates[hour % len(mates)]
//
// over the already-ascending-sorted same-patch roster. No new observation
// field, no change to claims, floors, budgets, or anything else. Ref 10 is
// deliberately skipped: the tests pin natural/10 as an unknown ref.
const NaturalPolicyFormatV11 uint32 = 11 // natural/11: natural/7 with the rotating addressee

// FrozenNaturalPolicy11 constructs the declared natural/11 policy, not a
// tunable family: format 11, ref {natural,11}, budget 32/32. As in every
// frozen natural ref, every actor of the cap-free roster binds the single
// frozen ref its world configured; the genome, not the policy ref, carries
// heredity.
func FrozenNaturalPolicy11() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV11,
		Ref:           NaturalRef{ID: "natural", Version: 11},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicy11 mirrors validNaturalPolicy for the natural/11 ref: the
// declared format and ref with the same budget-ceiling semantics, so a
// same-ref budget variant stays registrable and fails closed only at restore —
// VerifyFrozenNaturalPolicy compares the complete wire content against the
// executable's frozen policy.
func validNaturalPolicy11(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy11()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// frozenNatural11Bound reports whether the bound carries the frozen natural/11
// identity. The 11 cascade refuses to evaluate under any other bound content;
// as in v6/v7/v9, the frozen v5/v6 methods cannot carry the converse guard
// (their bytes are frozen), so the seam must call the matching cascade for the
// bound ref.
func frozenNatural11Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV11 &&
		b.policy.Ref == FrozenNaturalPolicy11().Ref
}

// Evaluate11 is the frozen natural/11 hourly decision cascade: the natural/7
// Evaluate verbatim — structural validation, InvalidSelf degradation, the bag
// decision (3/3/4/4), the denied-gather fallback, at most one stored meal per
// actor-hour, budget semantics, provision enabled with the frozen thresholds.
// The bound must carry the frozen natural/11 identity or the evaluation fails
// closed with ErrInvalidNaturalPolicy.
func (b *NaturalBound) Evaluate11(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNatural11Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalClaimCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, true, frozenNaturalClaimThresholds)
}

// EvaluateBirthRequest11 is the frozen natural/11 proposer decision at
// h+40m+5µs: the natural/7 EvaluateBirthRequest verbatim — living, free
// request slot, one request per actor-hour, honest self-reported claims with
// the explicit test override, the capital and granary margins verbatim v5, and
// the own-energy filing floor TFile = 4 — except the addressee, which rotates
// with the hour over the ascending same-patch roster instead of always being
// the actor's successor. The bound must carry the frozen natural/11 identity
// or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthRequest11(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural11Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalBirthRequestCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, naturalPolicyTFileV6, naturalAddresseeRotate)
}

// EvaluateBirthReply11 is the frozen natural/11 consenter decision at
// h+40m+6µs: the natural/7 birth-reply cascade verbatim — ascending requester
// order, first acceptable pending accepted, lowest-ID refusal with its typed
// reason, one reply per actor-hour, claims-only view of the requester, and the
// own-energy accept floor TFile = 4. The bound must carry the frozen
// natural/11 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthReply11(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural11Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReply(b, obs)
}
