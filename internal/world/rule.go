package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"fmt"
	"math/big"
)

func attemptKey(at sim.SimTime, actor sim.EntityID) string {
	return fmt.Sprintf("%016x:%016x", uint64(at), uint64(actor))
}

// A float64 subtraction can round a fractional difference to one. Compare
// the exact binary-rational values that will be stored instead.
func exactOneJouleDifference(before, after float64) bool {
	beforeRat := new(big.Rat).SetFloat64(before)
	afterRat := new(big.Rat).SetFloat64(after)
	if beforeRat == nil || afterRat == nil {
		return false
	}
	return new(big.Rat).Sub(beforeRat, afterRat).Cmp(big.NewRat(1, 1)) == 0
}

func readValue(view scheduler.SnapshotView, entity sim.EntityID, typeID sim.ComponentTypeID, field sim.FieldID) (sim.Value, bool, error) {
	v, err := view.Reader.Read(component.ReadRequest{Entity: entity, Component: typeID, Fields: []sim.FieldID{field}, WorldVersion: view.Version, Authority: view.Authority})
	if err == component.ErrComponentMissing {
		return sim.Value{}, false, nil
	}
	if err != nil {
		return sim.Value{}, false, err
	}
	value, err := v.Value(field)
	return value, true, err
}

func stockView(view scheduler.SnapshotView, cache sim.EntityID) (sim.Value, error) {
	value, exists, err := readValue(view, cache, component.CacheStockTypeID, component.CacheStockField)
	if !exists && err == nil {
		value, err = sim.AbsentValue(sim.ScalarKind, sim.Missing)
	}
	return value, err
}

// validate reads the trusted common scheduler snapshot, never the callback's
// mutable observation. One cache-stock field in each proposal enforces the
// deliberately single-winner-per-cache timestamp policy in the kernel.
func validate(view scheduler.SnapshotView, actor sim.EntityID, at sim.SimTime, visible map[sim.EntityID]bool, action WithdrawEnergy) (kernel.Proposal, Reason, error) {
	if action.ObservedVersion != view.Version {
		return kernel.Proposal{}, StaleObservation, nil
	}
	if sim.ValidateEntityID(action.CacheID) != nil {
		return kernel.Proposal{}, InvalidTarget, nil
	}
	if !visible[action.CacheID] {
		return kernel.Proposal{}, InvisibleCache, nil
	}
	stock, exists, err := readValue(view, action.CacheID, component.CacheStockTypeID, component.CacheStockField)
	if err != nil {
		return kernel.Proposal{}, NoReason, err
	}
	if !exists || !stock.IsPresent() {
		return kernel.Proposal{}, UnavailableStock, nil
	}
	quantity, err := stock.Scalar()
	if err != nil || quantity < 1 {
		return kernel.Proposal{}, InsufficientStock, nil
	}
	energy, exists, err := readValue(view, actor, component.EnergyTypeID, component.EnergyReserveField)
	if err != nil {
		return kernel.Proposal{}, NoReason, err
	}
	if !exists || !energy.IsPresent() {
		return kernel.Proposal{}, IneligibleActor, nil
	}
	reserve, err := energy.Scalar()
	if err != nil || reserve < 0 {
		return kernel.Proposal{}, IneligibleActor, nil
	}
	remainingStock, increasedEnergy := quantity-1, reserve+1
	// Both stored scalar deltas must be exactly one joule, including at
	// fractional and large-magnitude float64 precision boundaries.
	if !exactOneJouleDifference(quantity, remainingStock) || !exactOneJouleDifference(increasedEnergy, reserve) {
		return kernel.Proposal{}, InvalidNumber, nil
	}
	remaining, _ := sim.ScalarValue(remainingStock)
	increased, _ := sim.ScalarValue(increasedEnergy)
	return kernel.Proposal{
		Key: attemptKey(at, actor), Time: at, Cause: kernel.Cause{Actor: actor}, Rule: component.WithdrawRuleID, RuleVersion: 1,
		Patches: []component.Patch{
			{Entity: action.CacheID, Component: component.CacheStockTypeID, SchemaVersion: 1, Field: component.CacheStockField, Value: remaining},
			{Entity: actor, Component: component.EnergyTypeID, SchemaVersion: 1, Field: component.EnergyReserveField, Value: increased},
		},
	}, NoReason, nil
}
