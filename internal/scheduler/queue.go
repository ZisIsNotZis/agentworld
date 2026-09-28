package scheduler

import (
	"agentworld/internal/sim"
	"container/heap"
	"sort"
)

type WakeCause uint8

const (
	WakeInterruption WakeCause = iota + 1
	WakeCompletion
	WakePerceivedEvent
	WakeNeedThreshold
	WakeBirth
	WakeCommitment
	WakeStrategyInvalidation
	WakeAudit
)

type Wake struct {
	Actor sim.EntityID
	At    sim.SimTime
	Cause WakeCause
}

type wakeEntry struct {
	Wake
	index int
}

type wakeQueue struct {
	items   []*wakeEntry
	index   map[Wake]*wakeEntry
	byActor map[sim.EntityID]map[Wake]struct{}
}

func newWakeQueue() wakeQueue {
	return wakeQueue{index: make(map[Wake]*wakeEntry), byActor: make(map[sim.EntityID]map[Wake]struct{})}
}
func (q wakeQueue) Len() int { return len(q.items) }
func (q wakeQueue) Less(i, j int) bool {
	a, b := q.items[i].Wake, q.items[j].Wake
	if a.At != b.At {
		return a.At < b.At
	}
	if a.Actor != b.Actor {
		return a.Actor < b.Actor
	}
	return a.Cause < b.Cause
}
func (q wakeQueue) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
	q.items[i].index, q.items[j].index = i, j
}
func (q *wakeQueue) Push(value any) {
	e := value.(*wakeEntry)
	e.index = len(q.items)
	q.items = append(q.items, e)
	q.index[e.Wake] = e
	if q.byActor[e.Actor] == nil {
		q.byActor[e.Actor] = make(map[Wake]struct{})
	}
	q.byActor[e.Actor][e.Wake] = struct{}{}
}
func (q *wakeQueue) Pop() any {
	n := len(q.items) - 1
	e := q.items[n]
	q.items[n] = nil
	q.items = q.items[:n]
	delete(q.index, e.Wake)
	delete(q.byActor[e.Actor], e.Wake)
	if len(q.byActor[e.Actor]) == 0 {
		delete(q.byActor, e.Actor)
	}
	return e
}
func (q *wakeQueue) add(w Wake) bool {
	if q.index[w] != nil {
		return false
	}
	heap.Push(q, &wakeEntry{Wake: w})
	return true
}
func (q *wakeQueue) remove(w Wake) bool {
	if e := q.index[w]; e != nil {
		heap.Remove(q, e.index)
		return true
	}
	return false
}
func (q *wakeQueue) due() []Wake {
	if q.Len() == 0 {
		return nil
	}
	at := q.items[0].At
	var out []Wake
	for q.Len() > 0 && q.items[0].At == at {
		out = append(out, heap.Pop(q).(*wakeEntry).Wake)
	}
	// Heap extraction is ordered by (time, actor, cause).
	return out
}
func (q *wakeQueue) wakes() []Wake {
	out := make([]Wake, 0, q.Len())
	for _, e := range q.items {
		out = append(out, e.Wake)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		if out[i].Actor != out[j].Actor {
			return out[i].Actor < out[j].Actor
		}
		return out[i].Cause < out[j].Cause
	})
	return out
}
func (st *state) refresh(actor sim.EntityID) {
	f := st.fibers[actor]
	f.HasNextWake = false
	for w := range st.queue.byActor[actor] {
		if !f.HasNextWake || w.At < f.NextWake {
			f.HasNextWake, f.NextWake = true, w.At
		}
	}
	if !f.HasNextWake {
		f.NextWake = 0
	}
	st.fibers[actor] = f
}
func (st *state) addWake(w Wake) {
	if !st.queue.add(w) {
		return
	}
	f := st.fibers[w.Actor]
	if !f.HasNextWake || w.At < f.NextWake {
		f.HasNextWake, f.NextWake = true, w.At
		st.fibers[w.Actor] = f
	}
}

func (st *state) removeWake(w Wake) {
	if st.queue.remove(w) {
		f := st.fibers[w.Actor]
		if f.HasNextWake && f.NextWake == w.At {
			st.refresh(w.Actor)
		}
	}
}

func (st *state) validWake(w Wake) error {
	f, ok := st.fibers[w.Actor]
	if !ok || f.Lifecycle != Alive || w.Cause < WakeInterruption || w.Cause > WakeAudit || w.Cause == WakeCompletion || w.Cause == WakeInterruption || w.At < st.time || (st.hasClosed && w.At <= st.closed) {
		return ErrInvalidState
	}
	return nil
}
func (st *state) schedule(w Wake) error {
	if err := st.validWake(w); err != nil {
		return err
	}
	st.addWake(w)
	return nil
}
func (s *Scheduler) Schedule(w Wake) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.schedule(w)
}
