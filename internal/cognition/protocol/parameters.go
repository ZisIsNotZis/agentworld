package protocol

import "agentworld/internal/strategy"

// The frozen Appendix C values for the VS1 base cascade natural/7 (v6) and its
// natural/7b ablation, which share the same thresholds. These are the declared
// frozen values of the bound cascade, not tunables: the patchable generation
// substitutes them at the declared decision points.
var frozenNatural7Parameters = map[ParamID]int64{
	// The natural/7 claim cascade (strategy natural7.go): provision iff
	// granary >= 1 and energy >= 4; the bank is food only at energy <= 3.
	ParamClaimProvisionEnergyFloor: 4,
	ParamClaimBankEnergyCeiling:    3,
	// A living claimer gathers whenever a stocked home slot is visible
	// (energy == 0 short-circuits before the claim), so the gather floor is 1.
	ParamClaimGatherEnergyFloor: 1,
	// The v6-verbatim bag decision (strategy natural6.go, 3/4/4): the shared
	// investment guard (and finish leg), the re-pioneer leg, and the birth
	// filing/accept floor.
	ParamBagBuildEnergyFloor:       3,
	ParamBagBuildStartEnergyFloor:  4,
	ParamBagBuildFinishEnergyFloor: 3,
	ParamBirthFilingEnergyFloor:    4,
}

// FrozenNaturalClaimParameters returns the declared Appendix C frozen
// parameter vector of the actor's bound cascade. The VS1 seam's declared base
// policy is natural/7 (spec section 11.3), whose ablation natural/7b shares
// the same thresholds; every other bound ref returns an empty vector, because
// the declared parameter contract is defined there and a seam bound to another
// cascade is outside the VS1 base.
func FrozenNaturalClaimParameters(ref strategy.NaturalRef) map[ParamID]int64 {
	if ref != strategy.FrozenNaturalPolicy7().Ref && ref != strategy.FrozenNaturalPolicy7b().Ref {
		return map[ParamID]int64{}
	}
	out := make(map[ParamID]int64, len(frozenNatural7Parameters))
	for id, value := range frozenNatural7Parameters {
		out[id] = value
	}
	return out
}
