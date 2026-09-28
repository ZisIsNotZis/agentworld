package scheduler

import (
	"agentworld/internal/kernel"
	"agentworld/internal/sim"
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

const (
	PortableFormatVersion uint32 = 1
	MaxPortableBytes             = 8 << 20
	MaxPortableFibers            = 65536
	MaxPortableWakes             = 65536
	portableHeaderSize           = 233
	portableFiberSize            = 52
	portableWakeSize             = 17
	portableDigestSize           = 32
)

var portableMagic = [4]byte{'A', 'W', 'S', 'P'}

// ExportPortable captures a bounded canonical scheduler state at a quiescent
// step boundary. The embedded portable head binds the bytes to the full kernel
// snapshot and history; no process-local authority or origin is encoded. A
// concurrent external kernel writer makes the export fail rather than silently
// adopting an unrelated head. The caller must publish kernel history and these
// bytes together at the same verified boundary.
func (s *Scheduler) ExportPortable() ([]byte, kernel.PortableHead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &s.state
	if len(st.fibers) > MaxPortableFibers || st.queue.Len() > MaxPortableWakes {
		return nil, kernel.PortableHead{}, ErrInvalidState
	}
	if !s.kernel.SnapshotHead().Same(st.head) {
		return nil, kernel.PortableHead{}, ErrStaleWorld
	}
	_, head, err := s.kernel.ExportHistory()
	if err != nil {
		return nil, kernel.PortableHead{}, err
	}
	if !s.kernel.SnapshotHead().Same(st.head) || !portableTipMatches(head, st.head) {
		return nil, kernel.PortableHead{}, ErrStaleWorld
	}
	snap := Snapshot{Time: st.time, Closed: st.closed, HasClosed: st.hasClosed, NextToken: st.nextToken,
		Kernel: anchor(st.head), Wakes: st.queue.wakes(), Fibers: make([]Fiber, 0, len(st.fibers))}
	for _, f := range st.fibers {
		snap.Fibers = append(snap.Fibers, copyFiber(f))
	}
	sort.Slice(snap.Fibers, func(i, j int) bool { return snap.Fibers[i].Actor < snap.Fibers[j].Actor })
	// Restore validates the same internal invariants as ordinary in-process
	// snapshots, without changing that API's strict origin check.
	if _, err := Restore(s.kernel, s.workers, s.evaluate, snap); err != nil {
		return nil, kernel.PortableHead{}, err
	}
	out := make([]byte, 0, portableHeaderSize+len(snap.Fibers)*portableFiberSize+len(snap.Wakes)*portableWakeSize+portableDigestSize)
	out = append(out, portableMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, PortableFormatVersion)
	out = appendPortableHead(out, head)
	out = binary.BigEndian.AppendUint64(out, uint64(snap.Time))
	out = binary.BigEndian.AppendUint64(out, uint64(snap.Closed))
	out = appendBool(out, snap.HasClosed)
	out = binary.BigEndian.AppendUint64(out, snap.NextToken)
	out = binary.BigEndian.AppendUint32(out, uint32(len(snap.Fibers)))
	out = binary.BigEndian.AppendUint32(out, uint32(len(snap.Wakes)))
	for _, f := range snap.Fibers {
		out = binary.BigEndian.AppendUint64(out, uint64(f.Actor))
		out = binary.BigEndian.AppendUint64(out, f.Revision)
		out = append(out, byte(f.Lifecycle))
		out = appendBool(out, f.Activity != nil)
		var token uint64
		var deadline, interruption sim.SimTime
		var hasInterruption bool
		if f.Activity != nil {
			token, deadline = f.Activity.Token, f.Activity.Deadline
			if f.Activity.InterruptAt != nil {
				interruption, hasInterruption = *f.Activity.InterruptAt, true
			}
		}
		out = binary.BigEndian.AppendUint64(out, token)
		out = binary.BigEndian.AppendUint64(out, uint64(deadline))
		out = appendBool(out, hasInterruption)
		out = binary.BigEndian.AppendUint64(out, uint64(interruption))
		out = binary.BigEndian.AppendUint64(out, uint64(f.NextWake))
		out = appendBool(out, f.HasNextWake)
	}
	for _, w := range snap.Wakes {
		out = binary.BigEndian.AppendUint64(out, uint64(w.Actor))
		out = binary.BigEndian.AppendUint64(out, uint64(w.At))
		out = append(out, byte(w.Cause))
	}
	if len(out)+portableDigestSize > MaxPortableBytes {
		return nil, kernel.PortableHead{}, ErrInvalidState
	}
	digest := sha256.Sum256(out)
	out = append(out, digest[:]...)
	if !s.kernel.SnapshotHead().Same(st.head) {
		return nil, kernel.PortableHead{}, ErrStaleWorld
	}
	return out, head, nil
}

// RestorePortable requires a kernel produced by RestoreHistory and its
// independently verified PortableHead. It recomputes the entire kernel head,
// checks it against both the supplied head and the embedded head, and rebinds
// to that kernel's fresh process-local authority. The digest detects accidental
// corruption, not malicious replacement of both history and scheduler bytes;
// the enclosing checkpoint must authenticate the complete bundle.
func RestorePortable(k *kernel.Kernel, workers int, evaluate Evaluator, data []byte, verified kernel.PortableHead) (*Scheduler, error) {
	if k == nil || workers < 1 || evaluate == nil || len(data) < portableHeaderSize+portableDigestSize || len(data) > MaxPortableBytes {
		return nil, ErrInvalidState
	}
	body := data[:len(data)-portableDigestSize]
	if sha256.Sum256(body) != [32]byte(data[len(body):]) {
		return nil, ErrInvalidState
	}
	r := portableReader{data: body}
	magic := r.take(4)
	format := r.u32()
	head := r.head()
	time := sim.SimTime(r.u64())
	closed := sim.SimTime(r.u64())
	hasClosed := r.bool()
	nextToken := r.u64()
	fiberCount, wakeCount := r.u32(), r.u32()
	if r.bad || string(magic) != string(portableMagic[:]) || format != PortableFormatVersion || head != verified ||
		fiberCount > MaxPortableFibers || wakeCount > MaxPortableWakes ||
		portableHeaderSize+int(fiberCount)*portableFiberSize+int(wakeCount)*portableWakeSize != len(body) {
		return nil, ErrInvalidState
	}
	snap := Snapshot{Time: time, Closed: closed, HasClosed: hasClosed, NextToken: nextToken,
		Fibers: make([]Fiber, 0, fiberCount), Wakes: make([]Wake, 0, wakeCount)}
	for i := uint32(0); i < fiberCount; i++ {
		f := Fiber{Actor: sim.EntityID(r.u64()), Revision: r.u64(), Lifecycle: Lifecycle(r.u8())}
		hasActivity := r.bool()
		token := r.u64()
		deadline := sim.SimTime(r.u64())
		hasInterruption := r.bool()
		interruption := sim.SimTime(r.u64())
		f.NextWake = sim.SimTime(r.u64())
		f.HasNextWake = r.bool()
		if r.bad || (i > 0 && snap.Fibers[i-1].Actor >= f.Actor) ||
			(!hasActivity && (token != 0 || deadline != 0 || hasInterruption || interruption != 0)) ||
			(!hasInterruption && interruption != 0) {
			return nil, ErrInvalidState
		}
		if hasActivity {
			f.Activity = &Activity{Token: token, Deadline: deadline}
			if hasInterruption {
				f.Activity.InterruptAt = &interruption
			}
		}
		snap.Fibers = append(snap.Fibers, f)
	}
	for i := uint32(0); i < wakeCount; i++ {
		w := Wake{Actor: sim.EntityID(r.u64()), At: sim.SimTime(r.u64()), Cause: WakeCause(r.u8())}
		if r.bad || (i > 0 && !wakeBefore(snap.Wakes[i-1], w)) {
			return nil, ErrInvalidState
		}
		snap.Wakes = append(snap.Wakes, w)
	}
	if r.bad || r.offset != len(body) {
		return nil, ErrInvalidState
	}
	s, err := New(k, workers, evaluate)
	if err != nil {
		return nil, err
	}
	_, actual, err := k.ExportHistory()
	if err != nil {
		return nil, err
	}
	if actual != verified || !portableTipMatches(actual, s.state.head) || !k.SnapshotHead().Same(s.state.head) {
		return nil, ErrStaleWorld
	}
	snap.Kernel = anchor(s.state.head)
	return Restore(k, workers, evaluate, snap)
}

func portableTipMatches(p kernel.PortableHead, h kernel.Head) bool {
	return p.Version == h.Version && p.TipID == h.TipID && p.TipTime == h.TipTime && p.TipHash == h.TipHash
}
func wakeBefore(a, b Wake) bool {
	if a.At != b.At {
		return a.At < b.At
	}
	if a.Actor != b.Actor {
		return a.Actor < b.Actor
	}
	return a.Cause < b.Cause
}
func appendBool(out []byte, value bool) []byte {
	if value {
		return append(out, 1)
	}
	return append(out, 0)
}
func appendPortableHead(out []byte, h kernel.PortableHead) []byte {
	out = append(out, h.RegistryFingerprint[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(h.GenesisVersion))
	out = append(out, h.GenesisHash[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(h.Version))
	out = binary.BigEndian.AppendUint64(out, uint64(h.TipID))
	out = binary.BigEndian.AppendUint64(out, uint64(h.TipTime))
	out = append(out, h.TipHash[:]...)
	out = append(out, h.SnapshotHash[:]...)
	return append(out, h.ProjectionHash[:]...)
}

type portableReader struct {
	data   []byte
	offset int
	bad    bool
}

func (r *portableReader) take(n int) []byte {
	if r.bad || n < 0 || n > len(r.data)-r.offset {
		r.bad = true
		return nil
	}
	b := r.data[r.offset : r.offset+n]
	r.offset += n
	return b
}
func (r *portableReader) u8() byte {
	b := r.take(1)
	if r.bad {
		return 0
	}
	return b[0]
}
func (r *portableReader) u32() uint32 {
	b := r.take(4)
	if r.bad {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}
func (r *portableReader) u64() uint64 {
	b := r.take(8)
	if r.bad {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
func (r *portableReader) bool() bool {
	b := r.u8()
	if b > 1 {
		r.bad = true
	}
	return b == 1
}
func (r *portableReader) head() kernel.PortableHead {
	var h kernel.PortableHead
	copy(h.RegistryFingerprint[:], r.take(32))
	h.GenesisVersion = sim.WorldVersion(r.u64())
	copy(h.GenesisHash[:], r.take(32))
	h.Version = sim.WorldVersion(r.u64())
	h.TipID = sim.EventID(r.u64())
	h.TipTime = sim.SimTime(r.u64())
	copy(h.TipHash[:], r.take(32))
	copy(h.SnapshotHash[:], r.take(32))
	copy(h.ProjectionHash[:], r.take(32))
	return h
}
