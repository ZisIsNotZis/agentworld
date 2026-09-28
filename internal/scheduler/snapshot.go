package scheduler

import (
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"sort"
)

type KernelAnchor struct {
	Version  sim.WorldVersion
	OriginID uint64
	TipID    sim.EventID
	TipTime  sim.SimTime
	TipHash  [32]byte
}

func anchor(h kernel.Head) KernelAnchor {
	return KernelAnchor{h.Version, h.OriginID, h.TipID, h.TipTime, h.TipHash}
}

type Snapshot struct {
	Fibers    []Fiber
	Wakes     []Wake
	Time      sim.SimTime
	Closed    sim.SimTime
	HasClosed bool
	NextToken uint64
	Kernel    KernelAnchor
}

// Snapshot is taken under the coordinator lock, at a quiescent step boundary.
func (s *Scheduler) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &s.state
	out := Snapshot{Time: st.time, Closed: st.closed, HasClosed: st.hasClosed, NextToken: st.nextToken, Kernel: anchor(st.head), Wakes: st.queue.wakes()}
	out.Fibers = make([]Fiber, 0, len(st.fibers))
	for _, f := range st.fibers {
		out.Fibers = append(out.Fibers, copyFiber(f))
	}
	sort.Slice(out.Fibers, func(i, j int) bool { return out.Fibers[i].Actor < out.Fibers[j].Actor })
	return out
}

func Restore(k *kernel.Kernel, workers int, evaluate Evaluator, snap Snapshot) (*Scheduler, error) {
	s, err := New(k, workers, evaluate)
	if err != nil {
		return nil, err
	}
	st := &s.state
	if snap.Kernel != anchor(st.head) || snap.Time < 0 || snap.Closed < 0 ||
		(snap.HasClosed && snap.Closed != snap.Time) || (!snap.HasClosed && (snap.Closed != 0 || snap.Time != 0)) ||
		(st.head.TipID != 0 && (!snap.HasClosed || snap.Closed < st.head.TipTime)) {
		return nil, ErrInvalidState
	}
	st.time, st.closed, st.hasClosed, st.nextToken = snap.Time, snap.Closed, snap.HasClosed, snap.NextToken
	tokens := make(map[uint64]bool)
	for _, fiber := range snap.Fibers {
		f := copyFiber(fiber)
		if sim.ValidateEntityID(f.Actor) != nil || f.Revision == 0 || (f.Lifecycle != Alive && f.Lifecycle != Stopped) {
			return nil, ErrInvalidState
		}
		if _, exists := st.fibers[f.Actor]; exists {
			return nil, ErrInvalidState
		}
		if f.Activity != nil {
			a := f.Activity
			if f.Lifecycle != Alive || a.Token == 0 || a.Token > snap.NextToken || tokens[a.Token] || a.Deadline <= snap.Time || (snap.HasClosed && a.Deadline <= snap.Closed) ||
				(a.InterruptAt != nil && (*a.InterruptAt < snap.Time || (snap.HasClosed && *a.InterruptAt <= snap.Closed) || *a.InterruptAt > a.Deadline)) {
				return nil, ErrInvalidState
			}
			tokens[a.Token] = true
		}
		st.fibers[f.Actor] = f
	}
	for _, w := range snap.Wakes {
		f, ok := st.fibers[w.Actor]
		if !ok || f.Lifecycle != Alive || w.At < snap.Time || (snap.HasClosed && w.At <= snap.Closed) || w.Cause < WakeInterruption || w.Cause > WakeAudit || st.queue.index[w] != nil {
			return nil, ErrInvalidState
		}
		if w.Cause == WakeCompletion && (f.Activity == nil || f.Activity.InterruptAt != nil || f.Activity.Deadline != w.At) {
			return nil, ErrInvalidState
		}
		if w.Cause == WakeInterruption && (f.Activity == nil || f.Activity.InterruptAt == nil || *f.Activity.InterruptAt != w.At) {
			return nil, ErrInvalidState
		}
		st.queue.add(w)
	}
	for id, f := range st.fibers {
		if f.Activity != nil {
			needed := Wake{id, f.Activity.Deadline, WakeCompletion}
			if f.Activity.InterruptAt != nil {
				needed = Wake{id, *f.Activity.InterruptAt, WakeInterruption}
			}
			if st.queue.index[needed] == nil {
				return nil, ErrInvalidState
			}
		}
		st.refresh(id)
		computed := st.fibers[id]
		if f.HasNextWake != computed.HasNextWake || f.NextWake != computed.NextWake {
			return nil, ErrInvalidState
		}
	}
	return s, nil
}
