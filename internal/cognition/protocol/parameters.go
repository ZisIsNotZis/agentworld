package protocol

import "agentworld/internal/strategy"

// The declared Appendix C frozen parameter vector of the VS1 base cascade
// natural/7 (v6). The contract and its values live in internal/strategy; this
// package re-exports the lookup so the world and the request builder share one
// home for the fact.
var frozenNatural7Parameters = strategy.FrozenCognitiveParams().Values()

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
