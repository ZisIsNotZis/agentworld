package scheduler

import (
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"errors"
	"sync"
)

var ErrInvalidState = errors.New("invalid scheduler state")
var ErrStaleActivity = errors.New("stale activity token")
var ErrStaleWorld = errors.New("scheduler kernel head changed")

type Lifecycle uint8

const (
	Alive Lifecycle = iota + 1
	Stopped
)

type Fiber struct {
	Actor       sim.EntityID
	Revision    uint64
	Lifecycle   Lifecycle
	Activity    *Activity
	NextWake    sim.SimTime
	HasNextWake bool
}

func copyFiber(f Fiber) Fiber {
	if f.Activity != nil {
		a := *f.Activity
		if a.InterruptAt != nil {
			at := *a.InterruptAt
			a.InterruptAt = &at
		}
		f.Activity = &a
	}
	return f
}

type Scheduler struct {
	mu       sync.Mutex
	kernel   *kernel.Kernel
	evaluate Evaluator
	workers  int
	state    state
	// afterCommit lets tests force an external writer into the post-commit gap.
	afterCommit func()
}

type state struct {
	fibers    map[sim.EntityID]Fiber
	queue     wakeQueue
	time      sim.SimTime
	closed    sim.SimTime
	hasClosed bool
	nextToken uint64
	head      kernel.Head
}

func New(k *kernel.Kernel, workers int, evaluate Evaluator) (*Scheduler, error) {
	if k == nil || workers < 1 || evaluate == nil {
		return nil, ErrInvalidState
	}
	h := k.SnapshotHead()
	if h.TipID != 0 && h.TipTime < 0 {
		return nil, ErrInvalidState
	}
	s := &Scheduler{kernel: k, workers: workers, evaluate: evaluate}
	s.state = state{fibers: make(map[sim.EntityID]Fiber), queue: newWakeQueue(), head: h}
	if h.TipID != 0 {
		s.state.time, s.state.closed, s.state.hasClosed = h.TipTime, h.TipTime, true
	}
	return s, nil
}

func (s *Scheduler) Register(actor sim.EntityID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sim.ValidateEntityID(actor) != nil {
		return ErrInvalidState
	}
	if _, exists := s.state.fibers[actor]; exists {
		return ErrInvalidState
	}
	s.state.fibers[actor] = Fiber{Actor: actor, Lifecycle: Alive, Revision: 1}
	return nil
}

func (s *Scheduler) Fiber(actor sim.EntityID) (Fiber, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.state.fibers[actor]
	return copyFiber(f), ok
}

func (s *Scheduler) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.queue.Len()
}

func (s *Scheduler) Time() sim.SimTime {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.time
}
