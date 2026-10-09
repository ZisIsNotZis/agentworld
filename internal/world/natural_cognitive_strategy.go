package world

import (
	"fmt"
	"sort"

	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"agentworld/internal/strategy"
)

// The Role B strategy state in the runner (spec sections 5 and 10). A cognitive
// generation is the active declared behaviour-logic version; the runner
// consults it for the claim (and bag) decision and the birth-request filing of
// every actor it binds. The state is nil-tolerant: with no generation active,
// naturalEvaluate and naturalEvaluateBirthRequest take the frozen ref switch
// and every recorded run stays byte-identical.
//
// The runner's frozen ref (f.ref) is NOT changed by a rebind: it remains the
// frozen base identity the journal, genome, manifest policy-format slot, and
// strategy section are pinned to. The head generation is runner cognitive
// state, recorded in the ledger, the strategy section's binding table, and the
// manifest cognition block (spec 5.4/7.2), and the decision is made under it.

// NaturalCognitiveStrategy is the active generation and the exact actor set
// bound to it. It is immutable once published: ActivateCognitiveGeneration
// builds a new value and swaps it in atomically.
type NaturalCognitiveStrategy struct {
	generation strategy.NaturalCognitiveGeneration
	bound      map[sim.EntityID]bool
	lineage    []strategy.NaturalRef // root generation ref .. head generation ref
}

// Generation returns the active generation.
func (c *NaturalCognitiveStrategy) Generation() strategy.NaturalCognitiveGeneration {
	return c.generation
}

// Binds reports whether an actor carries the active generation.
func (c *NaturalCognitiveStrategy) Binds(actor sim.EntityID) bool {
	return c != nil && c.bound[actor]
}

// BoundActors returns the active generation's bound actors in ascending order.
func (c *NaturalCognitiveStrategy) BoundActors() []sim.EntityID {
	if c == nil {
		return nil
	}
	out := make([]sim.EntityID, 0, len(c.bound))
	for actor, bound := range c.bound {
		if bound {
			out = append(out, actor)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Lineage returns the generation refs from root to head.
func (c *NaturalCognitiveStrategy) Lineage() []strategy.NaturalRef {
	return append([]strategy.NaturalRef(nil), c.lineage...)
}

// evaluateClaim evaluates the patched claim + bag cascade for a bound actor.
// handled is false when no generation is active or the actor is not bound, in
// which case the caller takes the frozen path.
func (c *NaturalCognitiveStrategy) evaluateClaim(patches int, actor sim.EntityID, obs strategy.NaturalObservation) (strategy.NaturalChoice, bool, error) {
	if c == nil || !c.bound[actor] {
		return strategy.NaturalChoice{}, false, nil
	}
	choice, err := c.generation.EvaluateClaim(patches, actor, obs)
	if err != nil {
		return strategy.NaturalChoice{}, true, err
	}
	return choice, true, nil
}

// evaluateBirthRequest evaluates the patched proposer filing cascade for a
// bound actor.
func (c *NaturalCognitiveStrategy) evaluateBirthRequest(patches int, actor sim.EntityID, obs strategy.NaturalRequestObservation) (strategy.NaturalRequestChoice, bool, error) {
	if c == nil || !c.bound[actor] {
		return strategy.NaturalRequestChoice{}, false, nil
	}
	choice, err := c.generation.EvaluateBirthRequest(patches, actor, obs)
	if err != nil {
		return strategy.NaturalRequestChoice{}, true, err
	}
	return choice, true, nil
}

// ActivateCognitiveGeneration atomically rebinds the whole living roster to a
// generation at a quiescent boundary: every named actor must be a living
// roster member. The previous state is replaced only after the whole new value
// is built, so a failure leaves the runner untouched.
func (f *Natural) ActivateCognitiveGeneration(gen strategy.NaturalCognitiveGeneration, actors []sim.EntityID) error {
	if f == nil {
		return ErrNaturalRunner
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activateCognitiveGenerationLocked(gen, actors, nil)
}

// activateCognitiveGenerationLocked is the shared body; lineage supplies the
// generation refs from root to head (nil derives it as parent-chain + head).
func (f *Natural) activateCognitiveGenerationLocked(gen strategy.NaturalCognitiveGeneration, actors []sim.EntityID, lineage []strategy.NaturalRef) error {
	if err := gen.Validate(); err != nil {
		return fmt.Errorf("%w: cognitive generation: %v", ErrNaturalRunner, err)
	}
	// A rebind is a quiescent-boundary operation: never while a batch is
	// pending.
	if f.pending != nil {
		return fmt.Errorf("%w: cognitive rebind outside a quiescent boundary", ErrNaturalRunner)
	}
	bound := make(map[sim.EntityID]bool, len(actors))
	seen := make(map[sim.EntityID]bool, len(actors))
	for _, id := range actors {
		if seen[id] {
			continue
		}
		seen[id] = true
		if !f.rosterContains(id) || !f.pulseLifecycleAlive(id) {
			return fmt.Errorf("%w: cognitive actor %d is not a living roster member", ErrNaturalRunner, id)
		}
		bound[id] = true
	}
	if len(bound) == 0 {
		return fmt.Errorf("%w: cognitive rebind binds no actor", ErrNaturalRunner)
	}
	if lineage == nil {
		lineage = []strategy.NaturalRef{gen.Ref}
	}
	f.cognitiveStrategy = &NaturalCognitiveStrategy{generation: gen, bound: bound, lineage: append([]strategy.NaturalRef(nil), lineage...)}
	return nil
}

// ClearCognitiveGeneration removes the active generation (the frozen path
// again). A rollback to a frozen parent uses this.
func (f *Natural) ClearCognitiveGeneration() error {
	if f == nil {
		return ErrNaturalRunner
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil {
		return fmt.Errorf("%w: cognitive rollback outside a quiescent boundary", ErrNaturalRunner)
	}
	f.cognitiveStrategy = nil
	return nil
}

// ActiveCognitiveGeneration returns the active generation, if any.
func (f *Natural) ActiveCognitiveGeneration() (strategy.NaturalCognitiveGeneration, bool) {
	if f == nil {
		return strategy.NaturalCognitiveGeneration{}, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cognitiveStrategy == nil {
		return strategy.NaturalCognitiveGeneration{}, false
	}
	return f.cognitiveStrategy.generation, true
}

// CognitiveActorBound reports whether an actor carries the active generation.
func (f *Natural) CognitiveActorBound(id sim.EntityID) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cognitiveStrategy.Binds(id)
}

// pulseLifecycleAlive reports whether the actor is registered alive.
func (f *Natural) pulseLifecycleAlive(id sim.EntityID) bool {
	state, ok := f.pulseLifecycle[id]
	return ok && state == scheduler.Alive
}

// SetCognition attaches the optional Role A decision seam to a runner after
// construction, or clears it when c is nil. It exists because a births-enabled
// VS1 cell is restored from the recorded births-disabled root branch (spec
// section 11.3), and a restored runner cannot carry construction-time options;
// the seam is runner state like the active generation, wired before the first
// step and only at a quiescent boundary. A half-wired seam is refused, exactly
// as at construction, so a nil/valid seam leaves the frozen path unchanged.
func (f *Natural) SetCognition(c *NaturalCognition) error {
	if f == nil {
		return ErrNaturalRunner
	}
	if err := c.validate(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pending != nil {
		return fmt.Errorf("%w: cognitive seam wired outside a quiescent boundary", ErrNaturalRunner)
	}
	f.cognition = c
	return nil
}
