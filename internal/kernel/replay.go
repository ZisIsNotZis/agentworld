package kernel

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"bytes"
)

// Replay accepts only events reproducible by the same registry and seed.
// Accepted events do not prove which competing proposals lost a collision.
// The hash chain has no external trusted tip and is not an authenticity proof.
func Replay(registry component.Registry, version sim.WorldVersion, seeds []component.ComponentSeed, events []Event) (*Kernel, error) {
	k, err := New(registry, version, seeds)
	if err != nil {
		return nil, err
	}
	var previous [32]byte
	for start := 0; start < len(events); {
		end := start + 1
		for end < len(events) && events[end].Time == events[start].Time {
			end++
		}
		// A timestamp is a single batch: every accepted event in the group
		// was planned against the same snapshot, not its predecessor's result.
		plans := make([]Plan, 0, end-start)
		for i := start; i < end; i++ {
			event := events[i]
			if event.ID != sim.EventID(len(k.events)+i-start+1) || event.BeforeVersion != k.version+sim.WorldVersion(i-start) || event.AfterVersion != event.BeforeVersion+1 || event.PreviousHash != previous || event.Kind != "component-patch" {
				return nil, ErrEventIntegrity
			}
			hash, err := hashEvent(event)
			if err != nil || hash != event.Hash {
				return nil, ErrEventIntegrity
			}
			previous = event.Hash
			patches := make([]component.Patch, len(event.Deltas))
			for j, delta := range event.Deltas {
				patches[j] = component.Patch{Entity: delta.Entity, Component: delta.Component, SchemaVersion: delta.SchemaVersion, Field: delta.Field, Value: delta.After}
			}
			plan, err := k.Plan(Proposal{Key: event.Key, Time: event.Time, Cause: event.Cause, Rule: event.Rule, RuleVersion: event.RuleVersion, Patches: patches}, k.authority)
			if err != nil {
				return nil, ErrEventIntegrity
			}
			plans = append(plans, plan)
		}
		accepted, err := k.CommitBatch(plans)
		if err != nil || len(accepted) != end-start {
			return nil, ErrEventIntegrity
		}
		for i, actual := range events[start:end] {
			expected := accepted[i]
			actualBytes, e1 := actual.Bytes()
			expectedBytes, e2 := expected.Bytes()
			if e1 != nil || e2 != nil || !bytes.Equal(actualBytes, expectedBytes) || actual.Hash != expected.Hash {
				return nil, ErrEventIntegrity
			}
		}
		start = end
	}
	return k, nil
}
