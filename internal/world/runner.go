package world

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/scheduler"
	"agentworld/internal/sim"
	"context"
	"errors"
	"sort"
	"sync"
)

var ErrInvalidRunner = errors.New("invalid world runner configuration")

type collected struct {
	outcome  Outcome
	proposal *kernel.Proposal
}

type attemptBatch struct {
	mu       sync.Mutex
	attempts map[sim.EntityID]collected
}

// Runner owns the trusted S4 evaluator adapter. Callers that construct it and
// schedule wakes are trusted harness code; only Decide is an actor callback.
// Its facade methods serialize with Step, and callbacks must not re-enter it.
type Runner struct {
	mu      sync.Mutex
	sched   *scheduler.Scheduler
	decide  Decide
	public  map[sim.EntityID]map[sim.EntityID]bool
	active  *attemptBatch
	journal []Batch
	latest  map[sim.EntityID]Outcome
}

// public maps each actor to the cache IDs it may observe. Configuration is
// copied at construction and cannot be changed by actor callbacks.
func New(k *kernel.Kernel, workers int, public map[sim.EntityID][]sim.EntityID, decide Decide) (*Runner, error) {
	if k == nil || decide == nil || workers < 1 {
		return nil, ErrInvalidRunner
	}
	r := &Runner{decide: decide, public: make(map[sim.EntityID]map[sim.EntityID]bool), latest: make(map[sim.EntityID]Outcome)}
	for actor, caches := range public {
		if sim.ValidateEntityID(actor) != nil {
			return nil, ErrInvalidRunner
		}
		r.public[actor] = make(map[sim.EntityID]bool, len(caches))
		for _, id := range caches {
			if sim.ValidateEntityID(id) != nil || r.public[actor][id] {
				return nil, ErrInvalidRunner
			}
			r.public[actor][id] = true
		}
	}
	var err error
	r.sched, err = scheduler.New(k, workers, r.evaluate)
	return r, err
}

func (r *Runner) Register(actor sim.EntityID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sched.Register(actor)
}

func (r *Runner) Schedule(w scheduler.Wake) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sched.Schedule(w)
}

func (r *Runner) evaluate(ctx context.Context, ready scheduler.ReadyFiber, view scheduler.SnapshotView) (scheduler.Evaluation, error) {
	actor := ready.Fiber.Actor
	energy, exists, err := readValue(view, actor, component.EnergyTypeID, component.EnergyReserveField)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	if !exists {
		energy, err = sim.AbsentValue(sim.ScalarKind, sim.Missing)
		if err != nil {
			return scheduler.Evaluation{}, err
		}
	}
	ids := make([]sim.EntityID, 0, len(r.public[actor]))
	for id := range r.public[actor] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	caches := make([]CacheStock, 0, len(ids))
	for _, id := range ids {
		stock, err := stockView(view, id)
		if err != nil {
			return scheduler.Evaluation{}, err
		}
		caches = append(caches, CacheStock{CacheID: id, Stock: stock})
	}
	action, err := r.decide(ctx, Observation{Actor: actor, Time: ready.At, Version: view.Version, Energy: energy, PublicCaches: caches})
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	withdraw := action.WithdrawEnergy
	outcome := Outcome{Actor: actor, Action: withdraw, Status: Rejected, Time: ready.At, Key: attemptKey(ready.At, actor)}
	proposal, reason, err := validate(view, actor, ready.At, r.public[actor], withdraw)
	if err != nil {
		return scheduler.Evaluation{}, err
	}
	outcome.Reason = reason
	entry := collected{outcome: outcome}
	if reason == NoReason {
		entry.proposal = &proposal
	}
	r.active.mu.Lock()
	r.active.attempts[actor] = entry
	r.active.mu.Unlock()
	if entry.proposal == nil {
		return scheduler.Evaluation{}, nil
	}
	return scheduler.Evaluation{Proposals: []kernel.Proposal{proposal}}, nil
}

// Step publishes a journal batch and per-actor outcomes only after the
// scheduler successfully commits (including a rejected-only empty batch).
func (r *Runner) Step(ctx context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = &attemptBatch{attempts: make(map[sim.EntityID]collected)}
	defer func() { r.active = nil }()
	events, processed, err := r.sched.Step(ctx)
	if err != nil || !processed {
		return processed, err
	}
	byKey := make(map[string]sim.EventID, len(events))
	for _, event := range events {
		byKey[event.Key] = event.ID
	}
	snap := r.sched.Snapshot()
	batch := Batch{Time: snap.Time, Version: snap.Kernel.Version, TipID: snap.Kernel.TipID, TipHash: snap.Kernel.TipHash}
	for _, entry := range r.active.attempts {
		o := entry.outcome
		if entry.proposal != nil {
			if id := byKey[o.Key]; id != 0 {
				o.Status, o.Reason, o.EventID = Accepted, NoReason, id
			} else {
				o.Reason = CacheCollision
			}
		}
		batch.Attempts = append(batch.Attempts, o)
	}
	sort.Slice(batch.Attempts, func(i, j int) bool { return batch.Attempts[i].Key < batch.Attempts[j].Key })
	for _, outcome := range batch.Attempts {
		r.latest[outcome.Actor] = outcome
	}
	r.journal = append(r.journal, batch)
	return true, nil
}

// OutcomeFor exposes only the requested actor's last processed attempt to the
// trusted harness. It never passes another actor's outcome into Decide.
func (r *Runner) OutcomeFor(actor sim.EntityID) (Outcome, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	outcome, ok := r.latest[actor]
	return outcome, ok
}

func (r *Runner) Journal() []Batch {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Batch, len(r.journal))
	for i, b := range r.journal {
		out[i] = b
		out[i].Attempts = append([]Outcome(nil), b.Attempts...)
	}
	return out
}
