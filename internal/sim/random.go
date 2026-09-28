package sim

import "math"

type RandomState struct {
	Seed     uint64
	Stream   StreamID
	Position uint64
}

// RandomStream is a caller-owned counter-based stream. Each result is SplitMix64
// applied to a fixed mixing of seed, stream ID, and position. This algorithm is
// project-owned and versioned by this source rather than a standard-library RNG.
type RandomStream struct{ state RandomState }

func NewRandomStream(state RandomState) RandomStream { return RandomStream{state: state} }

func (r *RandomStream) Uint64() uint64 {
	if r.state.Position == math.MaxUint64 {
		panic(ErrOverflow)
	}
	counter := r.state.Seed ^ rotateLeft(uint64(r.state.Stream), 29) ^ (r.state.Position * 0x9e3779b97f4a7c15)
	r.state.Position++
	counter += 0x9e3779b97f4a7c15
	counter = (counter ^ (counter >> 30)) * 0xbf58476d1ce4e5b9
	counter = (counter ^ (counter >> 27)) * 0x94d049bb133111eb
	return counter ^ (counter >> 31)
}

func (r *RandomStream) Float64() float64 {
	return float64(r.Uint64()>>11) * (1.0 / (1 << 53))
}

func (r RandomStream) State() RandomState { return r.state }

func rotateLeft(v uint64, n uint) uint64 { return (v << n) | (v >> (64 - n)) }
