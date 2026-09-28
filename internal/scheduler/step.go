package scheduler

import (
	"agentworld/internal/component"
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"context"
	"sort"
	"sync"
)

type ReadyFiber struct {
	Fiber  Fiber
	At     sim.SimTime
	Causes []WakeCause
}

type SnapshotView struct {
	Reader    component.Reader
	Authority component.Authority
	Version   sim.WorldVersion
}

type EffectKind uint8

const (
	EffectSchedule EffectKind = iota + 1
	EffectStart
	EffectCancel
	EffectInterrupt
	EffectStop
)

type Effect struct {
	Kind     EffectKind
	Actor    sim.EntityID
	Wake     Wake
	Duration sim.Duration
	Token    uint64
	At       sim.SimTime
	// IfKey applies this effect only if the proposal with this key commits.
	IfKey string
}

type Evaluation struct {
	Proposals []kernel.Proposal
	Effects   []Effect
}

type Evaluator func(context.Context, ReadyFiber, SnapshotView) (Evaluation, error)

func (st *state) clone() state {
	copyState := *st
	copyState.fibers = make(map[sim.EntityID]Fiber, len(st.fibers))
	for id, f := range st.fibers {
		copyState.fibers[id] = copyFiber(f)
	}
	copyState.queue = newWakeQueue()
	for _, w := range st.queue.wakes() {
		copyState.queue.add(w)
	}
	return copyState
}

func (st *state) apply(e Effect) error {
	switch e.Kind {
	case EffectSchedule:
		if e.Wake.Actor != e.Actor {
			return ErrInvalidState
		}
		return st.schedule(e.Wake)
	case EffectStart:
		_, err := st.start(e.Actor, e.Duration)
		return err
	case EffectCancel:
		return st.finish(e.Actor, e.Token, 0, false)
	case EffectInterrupt:
		return st.finish(e.Actor, e.Token, e.At, true)
	case EffectStop:
		f, ok := st.fibers[e.Actor]
		if !ok || f.Lifecycle != Alive || f.Activity != nil || f.Revision == ^uint64(0) {
			return ErrInvalidState
		}
		f.Lifecycle, f.Revision = Stopped, f.Revision+1
		st.fibers[e.Actor] = f
		for w := range st.queue.byActor[e.Actor] {
			st.queue.remove(w)
		}
		st.refresh(e.Actor)
		return nil
	default:
		return ErrInvalidState
	}
}

// Step closes one complete timestamp. Workers only receive the common immutable
// reader; the coordinator alone plans and commits. On any failure the draft is
// discarded, including consumed wakes and activity completions.
func (s *Scheduler) Step(ctx context.Context) ([]kernel.Event, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx == nil {
		return nil, false, ErrInvalidState
	}
	if !s.kernel.SnapshotHead().Same(s.state.head) {
		return nil, false, ErrStaleWorld
	}
	if s.state.queue.Len() == 0 {
		return nil, false, nil
	}
	draft := s.state.clone()
	wakes := draft.queue.due()
	at := wakes[0].At
	draft.time = at
	var ready []ReadyFiber
	for _, w := range wakes {
		if len(ready) == 0 || ready[len(ready)-1].Fiber.Actor != w.Actor {
			f := draft.fibers[w.Actor]
			ready = append(ready, ReadyFiber{Fiber: f, At: at})
		}
		r := &ready[len(ready)-1]
		r.Causes = append(r.Causes, w.Cause)
		if w.Cause == WakeCompletion || w.Cause == WakeInterruption {
			f := draft.fibers[w.Actor]
			if f.Activity == nil || f.Revision == ^uint64(0) ||
				(w.Cause == WakeCompletion && (f.Activity.Deadline != at || f.Activity.InterruptAt != nil)) ||
				(w.Cause == WakeInterruption && (f.Activity.InterruptAt == nil || *f.Activity.InterruptAt != at)) {
				return nil, false, ErrInvalidState
			}
			f.Activity = nil
			f.Revision++
			draft.fibers[w.Actor] = f
		}
	}
	for i := range ready {
		draft.refresh(ready[i].Fiber.Actor)
		ready[i].Fiber = copyFiber(draft.fibers[ready[i].Fiber.Actor])
	}
	view := SnapshotView{s.state.head.Reader, s.state.head.Authority, s.state.head.Version}
	type result struct {
		value Evaluation
		err   error
	}
	results := make([]result, len(ready))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := s.workers
	if workers > len(ready) {
		workers = len(ready)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i].value, results[i].err = s.evaluate(ctx, ready[i], view)
			}
		}()
	}
	for i := range ready {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	var proposals []kernel.Proposal
	for _, r := range results {
		if r.err != nil {
			return nil, false, r.err
		}
		for _, p := range r.value.Proposals {
			if p.Time != at {
				return nil, false, ErrInvalidState
			}
			proposals = append(proposals, p)
		}
	}
	if !s.kernel.SnapshotHead().Same(s.state.head) {
		return nil, false, ErrStaleWorld
	}
	plans := make([]kernel.Plan, 0, len(proposals))
	for _, p := range proposals {
		plan, err := s.kernel.Plan(p, view.Authority)
		if err != nil {
			return nil, false, err
		}
		plans = append(plans, plan)
	}
	// Match kernel's whole-proposal field collision rule before staging
	// conditional effects. CommitBatchAtHead still validates every plan.
	sort.Slice(proposals, func(i, j int) bool { return proposals[i].Key < proposals[j].Key })
	winners := make(map[string]bool)
	seen := make(map[[3]uint64]bool)
	for _, p := range proposals {
		conflict := false
		for _, patch := range p.Patches {
			if seen[[3]uint64{uint64(patch.Entity), uint64(patch.Component), uint64(patch.Field)}] {
				conflict = true
			}
		}
		if !conflict {
			winners[p.Key] = true
			for _, patch := range p.Patches {
				seen[[3]uint64{uint64(patch.Entity), uint64(patch.Component), uint64(patch.Field)}] = true
			}
		}
	}
	if draft.hasClosed && at <= draft.closed {
		return nil, false, ErrInvalidState
	}
	draft.closed, draft.hasClosed = at, true
	for _, r := range results {
		for _, e := range r.value.Effects {
			if e.IfKey != "" {
				found := false
				for _, p := range r.value.Proposals {
					if p.Key == e.IfKey {
						found = true
						break
					}
				}
				if !found {
					return nil, false, ErrInvalidState
				}
				if !winners[e.IfKey] {
					continue
				}
			}
			if err := draft.apply(e); err != nil {
				return nil, false, err
			}
		}
	}
	events, committedHead, err := s.kernel.CommitBatchAtHead(s.state.head, plans)
	if err != nil {
		return nil, false, err
	}
	if s.afterCommit != nil {
		s.afterCommit()
	}
	draft.head = committedHead
	s.state = draft
	return events, true, nil
}
