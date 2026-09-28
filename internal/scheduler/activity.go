package scheduler

import "agentworld/internal/sim"

type Activity struct {
	Token       uint64
	Deadline    sim.SimTime
	InterruptAt *sim.SimTime // pending for this token; activity stays live until the due wake
}

func (st *state) start(actor sim.EntityID, duration sim.Duration) (uint64, error) {
	f, ok := st.fibers[actor]
	if !ok || f.Lifecycle != Alive || f.Activity != nil || duration <= 0 || st.nextToken == ^uint64(0) || f.Revision == ^uint64(0) {
		return 0, ErrInvalidState
	}
	deadline, err := st.time.Add(duration)
	if err != nil {
		return 0, err
	}
	if st.hasClosed && deadline <= st.closed {
		return 0, ErrInvalidState
	}
	st.nextToken++
	f.Activity = &Activity{Token: st.nextToken, Deadline: deadline}
	f.Revision++
	st.fibers[actor] = f
	st.addWake(Wake{actor, deadline, WakeCompletion})
	return st.nextToken, nil
}
func (s *Scheduler) Start(actor sim.EntityID, duration sim.Duration) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.start(actor, duration)
}
func (st *state) finish(actor sim.EntityID, token uint64, at sim.SimTime, interrupt bool) error {
	f, ok := st.fibers[actor]
	if !ok || f.Activity == nil || token == 0 || f.Activity.Token != token {
		return ErrStaleActivity
	}
	if f.Revision == ^uint64(0) {
		return ErrInvalidState
	}
	if interrupt {
		if f.Activity.InterruptAt != nil || at < st.time || at > f.Activity.Deadline || (st.hasClosed && at <= st.closed) {
			return ErrInvalidState
		}
		st.removeWake(Wake{actor, f.Activity.Deadline, WakeCompletion})
		f = st.fibers[actor]
		f.Activity.InterruptAt = &at
		f.Revision++
		st.fibers[actor] = f
		st.addWake(Wake{actor, at, WakeInterruption})
		return nil
	}
	if f.Activity.InterruptAt != nil {
		st.removeWake(Wake{actor, *f.Activity.InterruptAt, WakeInterruption})
	} else {
		st.removeWake(Wake{actor, f.Activity.Deadline, WakeCompletion})
	}
	f = st.fibers[actor]
	f.Activity = nil
	f.Revision++
	st.fibers[actor] = f
	return nil
}
func (s *Scheduler) Cancel(actor sim.EntityID, token uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.finish(actor, token, 0, false)
}
func (s *Scheduler) Interrupt(actor sim.EntityID, token uint64, at sim.SimTime) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.finish(actor, token, at, true)
}
