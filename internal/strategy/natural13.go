package strategy

// Natural v13 policy lane (ticket 49). Policy natural/13 (format 13, ref
// {natural,13}, budget 32/32) is natural/12 VERBATIM except for exactly one
// behavioural difference: the consenter's own-energy accept floor.
//
// Ticket 49 declares mechanism v8: the mate pool is the whole band (the world
// now hands every living unrelated roster actor to the proposer) and the
// household consent gate lets the consenter conceive on energy >= 2 while the
// proposer keeps the full wealth gate. natural/13 keeps the non-kin rotation
// over that band-wide PatchMates, keeps the proposer's filing floor at
// naturalPolicyTFileV6 = 4, and lowers only the reply consent floor:
//
//	ownOK = granary >= 3 AND capital >= 2 AND energy >= 2
//
// every other choice — the claim cascade, the request cascade's claims and
// margins, the reply taxonomy — is natural/12 verbatim. Old refs stay
// byte-frozen; natural/13 is legal under mechanism v8 only.
const NaturalPolicyFormatV13 uint32 = 13 // natural/13: natural/12 with the household consent floor

// natural13ConsentEnergy is the consenter's own-energy accept floor under the
// v8 household gate (world.NaturalHouseholdMinEnergy = 2), applied with the
// same constant the world gate uses.
const natural13ConsentEnergy int64 = 2

// FrozenNaturalPolicy13 constructs the declared natural/13 policy, not a
// tunable family: format 13, ref {natural,13}, budget 32/32.
func FrozenNaturalPolicy13() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV13,
		Ref:           NaturalRef{ID: "natural", Version: 13},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicy13 mirrors validNaturalPolicy for the natural/13 ref: the
// declared format and ref with the same budget-ceiling semantics.
func validNaturalPolicy13(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy13()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// frozenNatural13Bound reports whether the bound carries the frozen natural/13
// identity. The 13 cascade refuses to evaluate under any other bound content.
func frozenNatural13Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV13 &&
		b.policy.Ref == FrozenNaturalPolicy13().Ref
}

// Evaluate13 is the frozen natural/13 hourly decision cascade: the natural/7
// (natural/11, natural/12) Evaluate verbatim. The bound must carry the frozen
// natural/13 identity or the evaluation fails closed.
func (b *NaturalBound) Evaluate13(obs NaturalObservation) (NaturalChoice, error) {
	if !frozenNatural13Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalClaimCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, true, frozenNaturalClaimThresholds)
}

// EvaluateBirthRequest13 is the frozen natural/13 proposer decision at
// h+40m+5µs: natural/12 verbatim — the frozen v6 filing floor TFile = 4 and
// the non-kin rotation — over the band-wide PatchMates mechanism v8 supplies.
// The bound must carry the frozen natural/13 identity or the evaluation fails
// closed.
func (b *NaturalBound) EvaluateBirthRequest13(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural13Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return naturalBirthRequestCascade(naturalClaimIdentity{Actor: b.actor, Patches: b.patches, Ref: b.policy.Ref, Budget: b.policy.Budget}, obs, naturalPolicyTFileV6, naturalAddresseeBandNonKin)
}

// EvaluateBirthReply13 is the frozen natural/13 consenter decision at
// h+40m+6µs: the natural/7 birth-reply cascade verbatim except the own-energy
// accept floor, which drops to the household gate 2 (natural/12 uses 4). The
// bound must carry the frozen natural/13 identity or the evaluation fails
// closed.
func (b *NaturalBound) EvaluateBirthReply13(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural13Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReplyAt(b, obs, natural13ConsentEnergy, true)
}
