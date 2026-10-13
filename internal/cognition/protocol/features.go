package protocol

import (
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The declared candidate feature set (spec Appendix C.1). Every row except
// PriorReliability is observation-only: it reads the actor's own
// strategy.NaturalObservation. PriorReliability is the one declared world-level
// aggregate row, filled by the world's roster builder, never by this
// observation-only projection and never read by the escalation predicate
// (EC-6).

// FeatureSetVersion is the declared version of the candidate feature set
// (Appendix C.1). It is part of run identity (carried on ValueModelSpec) and is
// named by the VS2 report, so a report identifies the feature derivations a run
// used.
const FeatureSetVersion = "vs2-features/1"

// DerivedFeatureScale is the declared fixed-point scale of the derived C.1 rows
// (decision_sensitivity, urgency, impact, propagation_risk). Each derived row is
// an integer in [0, DerivedFeatureScale].
const DerivedFeatureScale int64 = 1000

// The frozen claim thresholds the derived rows are measured against. The
// protocol package cannot import internal/world and internal/strategy keeps the
// policy thresholds unexported, so they are restated here as declared constants
// and cross-checked against the world's exported frozen constants in the lane
// test (internal/world/natural_cognition_priority_test.go).
const (
	// FeatureEnergyCapacity is the frozen energy capacity (strategy
	// naturalEnergyCapacity / world CapacityEnergyCapacity).
	FeatureEnergyCapacity int64 = 12
	// FeatureHungerCapacity is the frozen hunger capacity (the starvation clock
	// edge; strategy naturalHungerCapacity).
	FeatureHungerCapacity int64 = 24
	// FeatureSlotsPerPatch is the frozen number of home slots per patch
	// (world CapacitySlotsPerPatch).
	FeatureSlotsPerPatch int64 = 8
	// FeatureBuildEnergyFloor is the frozen bag-decision build energy floor
	// ("build iff energy >= 5 or the granary holds a fallback meal"; strategy
	// naturalPolicyTLow).
	FeatureBuildEnergyFloor int64 = 5
	// FeatureConsentMinEnergy, FeatureConsentMinCapital and
	// FeatureConsentMinGranary are the frozen birth/consent thresholds (world
	// NaturalConsentMinEnergy/Capital/Granary).
	FeatureConsentMinEnergy  int64 = 3
	FeatureConsentMinCapital int64 = 2
	FeatureConsentMinGranary int64 = 3
	// FeatureStoredMealGranary is the frozen stored-meal availability floor
	// ("granary >= 1").
	FeatureStoredMealGranary int64 = 1
	// FeatureBoundaryWindow is the declared half-width of the "near a frozen
	// claim threshold" band used by decision_sensitivity: a value at distance d
	// contributes max(0, FeatureBoundaryWindow-d) proximity.
	FeatureBoundaryWindow int64 = 2
)

// PriorReliabilityVersion is the declared version of the world-level
// prior-reliability aggregate row (Appendix C.1).
const PriorReliabilityVersion = "vs2-prior/1"

// DeclaredPriorReliability is the declared neutral world-level prior-reliability
// cell statistic (Appendix C.1): a declared, versioned, reversible default
// pending operator confirmation (spec section 11). It is the same value for
// every candidate of a cell, carries no other actor's private state, and is
// never read by the escalation predicate. A calibrated per-region statistic
// replaces this constant in a later lane.
const DeclaredPriorReliability int64 = DerivedFeatureScale / 2

// ObservationFeatures derives the declared observation-only C.1 feature rows
// from the actor's own strategy.NaturalObservation (C5): the directly-sourced
// rows, the stocked home-slot count, the declared last_denial encoding, and the
// derived decision_sensitivity/urgency/impact/propagation_risk rows. The
// scheduler-level PriorReliability aggregate and the §4.3
// EscalationRequested bit are left zero here, so this projection is strictly
// observation-only; the world's roster builder fills PriorReliability with the
// declared cell statistic and EscalationRequested from EscalationRequest.
func ObservationFeatures(obs strategy.NaturalObservation) ValueFeatures {
	energy := featureObservationInt(obs.Energy)
	hunger := featureObservationInt(obs.Hunger)
	capital := featureObservationInt(obs.Capital)
	granary := featureObservationInt(obs.GranaryStock)
	lastMeal := featureObservationInt(obs.LastStoredMealHour)
	stocked := int64(0)
	for _, slot := range obs.HomeSlots {
		if featureObservationInt(slot.Stock) >= 1 {
			stocked++
		}
	}
	return ValueFeatures{
		Energy:              energy,
		Hunger:              hunger,
		Capital:             capital,
		GranaryStock:        granary,
		LastStoredMealHour:  lastMeal,
		StockedHomeSlots:    stocked,
		LastDenial:          featureDenialEncoding(obs.LastDenial),
		DecisionSensitivity: derivedDecisionSensitivity(energy, hunger, capital, granary, lastMeal, stocked, int64(obs.Hour)),
		Urgency:             derivedUrgency(energy, hunger),
		Impact:              derivedImpact(capital, granary),
		PropagationRisk:     derivedPropagationRisk(stocked),
	}
}

// derivedUrgency is the declared survival proximity from energy/hunger
// (Appendix C.1): the fraction of the combined survival reserve already
// consumed, (energyCapacity-energy + hunger) / (energyCapacity+hungerCapacity),
// in fixed point. It is monotone non-increasing in energy and non-decreasing in
// hunger.
func derivedUrgency(energy, hunger int64) int64 {
	energy = clampFeature(energy, 0, FeatureEnergyCapacity)
	hunger = clampFeature(hunger, 0, FeatureHungerCapacity)
	deficit := (FeatureEnergyCapacity - energy) + hunger
	return DerivedFeatureScale * deficit / (FeatureEnergyCapacity + FeatureHungerCapacity)
}

// derivedImpact is the declared birth-gate / investment proximity from
// capital/granary_stock (Appendix C.1): the fraction of the frozen consent
// margins already met, (min(capital,2) + min(granary,3)) / (2+3), in fixed
// point. It is monotone non-decreasing in both rows and saturates when the
// birth gate (capital>=2, granary>=3) is met.
func derivedImpact(capital, granary int64) int64 {
	capital = clampFeature(capital, 0, FeatureConsentMinCapital)
	granary = clampFeature(granary, 0, FeatureConsentMinGranary)
	return DerivedFeatureScale * (capital + granary) / (FeatureConsentMinCapital + FeatureConsentMinGranary)
}

// derivedPropagationRisk is the declared contested-slot risk from
// stocked_home_slots (Appendix C.1): the fraction of the frozen home-slot
// resource the actor sees stocked, min(stocked,slotsPerPatch)/slotsPerPatch, in
// fixed point. It is monotone non-decreasing in stocked_home_slots.
func derivedPropagationRisk(stocked int64) int64 {
	stocked = clampFeature(stocked, 0, FeatureSlotsPerPatch)
	return DerivedFeatureScale * stocked / FeatureSlotsPerPatch
}

// derivedDecisionSensitivity is the declared distance to the frozen claim
// thresholds (Appendix C.1): the summed proximity of the actor's own rows to
// the frozen claim/birth decision boundaries, in fixed point. Each declared
// (value, threshold) pair whose distance is below FeatureBoundaryWindow
// contributes FeatureBoundaryWindow-distance; the sum is normalised by the
// number of pairs. Being at or near a boundary (either direction) raises the
// row, because that is where a small state change flips the frozen action.
func derivedDecisionSensitivity(energy, hunger, capital, granary, lastMeal, stocked, hour int64) int64 {
	pairs := [...]featureThresholdPair{
		{energy, FeatureBuildEnergyFloor},   // bag-decision build energy floor
		{energy, FeatureConsentMinEnergy},   // birth/consent energy floor
		{granary, FeatureStoredMealGranary}, // stored-meal availability floor
		{granary, FeatureConsentMinGranary}, // birth/consent granary floor
		{capital, FeatureConsentMinCapital}, // birth/consent capital floor
		{hunger, FeatureHungerCapacity},     // starvation clock edge
		{stocked, FeatureStoredMealGranary}, // gather availability (a stocked slot)
		{lastMeal, hour},                    // stored-meal freshness boundary (last meal < hour)
	}
	var proximity int64
	for _, pair := range pairs {
		d := pair.value - pair.threshold
		if d < 0 {
			d = -d
		}
		if d < FeatureBoundaryWindow {
			proximity += FeatureBoundaryWindow - d
		}
	}
	return DerivedFeatureScale * proximity / (FeatureBoundaryWindow * int64(len(pairs)))
}

// featureThresholdPair is one (observation value, frozen threshold) boundary of
// the decision-sensitivity measure.
type featureThresholdPair struct{ value, threshold int64 }

// featureDenialEncoding is the declared last_denial string encoding (Appendix
// C.1): the actor's own last denial, "" for no denial, else the denied action
// name. An unset/unexpected value encodes as "" rather than inventing a name.
func featureDenialEncoding(d strategy.NaturalDenial) string {
	switch d {
	case strategy.NaturalNoDenial:
		return ""
	case strategy.NaturalGatherDenied:
		return "gather"
	case strategy.NaturalBuildDenied:
		return "build"
	default:
		return ""
	}
}

// featureObservationInt reads an observation value as its integer projection. A
// non-integer observation value contributes zero, matching the frozen
// stocked-slot read (naturalLowestStockedSlot).
func featureObservationInt(v sim.Value) int64 {
	n, err := v.Integer()
	if err != nil {
		return 0
	}
	return n
}

// clampFeature clamps v into [lo, hi].
func clampFeature(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
