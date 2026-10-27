package strategy

// Natural v14 policy lane (ticket 50). Policy natural/14 (format 14, ref
// {natural,14}, budget 32/32) is natural/13 VERBATIM except for exactly one
// observation-validation change: the founder's declared home patch is checked
// against the round-robin placement of mechanism v10 instead of the frozen
// ID-block placement.
//
// Mechanism v10 keeps every v9 rule (the band-wide mate pool, the household
// consent gate, the declared granary yield) and changes only who lives where:
// founder i lives on patch (i-1) mod P, so a band smaller than the land is
// spread across every patch. The natural/13 request cascade assumes the
// blocked placement (founder i on patch (i-1)/8) and would refuse a v10
// founder's own observation; natural/14 replaces that one clause with the
// placement the mechanism declares — no clause is dropped, and every other
// validation (the claims, the kin walk, the addressee rotation, the budget)
// is unchanged. Old refs stay byte-frozen; natural/14 is legal under
// mechanism v10 only.
const NaturalPolicyFormatV14 uint32 = 14 // natural/14: natural/13 with round-robin founder placement

// FrozenNaturalPolicy14 constructs the declared natural/14 policy, not a
// tunable family: format 14, ref {natural,14}, budget 32/32.
func FrozenNaturalPolicy14() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV14,
		Ref:           NaturalRef{ID: "natural", Version: 14},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicy14 mirrors validNaturalPolicy for the natural/14 ref.
func validNaturalPolicy14(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy14()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// frozenNatural14Bound reports whether the bound carries the frozen natural/14
// identity. The 14 cascade refuses to evaluate under any other bound content.
func frozenNatural14Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV14 &&
		b.policy.Ref == FrozenNaturalPolicy14().Ref
}

// natural14ClaimIdentity is the natural/13 decision identity with the
// round-robin founder placement declared.
func natural14ClaimIdentity(b *NaturalBound) naturalClaimIdentity {
	return naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget,
		Placement: naturalPlacementRoundRobin}
}

// Evaluate14 is the frozen natural/14 hourly decision cascade: natural/13
// verbatim, validating the round-robin founder home patch.
func (b *NaturalBound) Evaluate14(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNatural14Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalClaimCascade(natural14ClaimIdentity(b), obs, true, frozenNaturalClaimThresholds)
}

// EvaluateBirthRequest14 is the frozen natural/14 proposer decision at
// h+40m+5µs: natural/13 verbatim except the founder-home validation, which
// follows the round-robin placement.
func (b *NaturalBound) EvaluateBirthRequest14(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural14Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalBirthRequestCascade(natural14ClaimIdentity(b), obs, naturalPolicyTFileV6, naturalAddresseeBandNonKin)
}

// EvaluateBirthReply14 is the frozen natural/14 consenter decision at
// h+40m+6µs: natural/13 verbatim.
func (b *NaturalBound) EvaluateBirthReply14(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural14Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReplyAt(b, obs, natural13ConsentEnergy, true)
}
