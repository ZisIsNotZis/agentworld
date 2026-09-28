package sim

import "errors"

var ErrNegativeTime = errors.New("simulation time and duration must be non-negative")

// SimTime is an exact count of microseconds from the simulation epoch.
type SimTime int64

// Duration is an exact non-negative count of microseconds.
type Duration int64

type WorldVersion uint64

func NewSimTime(microseconds int64) (SimTime, error) {
	if microseconds < 0 {
		return 0, ErrNegativeTime
	}
	return SimTime(microseconds), nil
}

func NewDuration(microseconds int64) (Duration, error) {
	if microseconds < 0 {
		return 0, ErrNegativeTime
	}
	return Duration(microseconds), nil
}

func (t SimTime) Add(d Duration) (SimTime, error) {
	if t < 0 || d < 0 {
		return 0, ErrNegativeTime
	}
	if int64(d) > int64(^uint64(0)>>1)-int64(t) {
		return 0, ErrOverflow
	}
	return t + SimTime(d), nil
}

func (t SimTime) Sub(other SimTime) (Duration, error) {
	if t < 0 || other < 0 {
		return 0, ErrNegativeTime
	}
	if other > t {
		return 0, ErrNegativeTime
	}
	return Duration(t - other), nil
}

func (d Duration) Add(other Duration) (Duration, error) {
	if d < 0 || other < 0 {
		return 0, ErrNegativeTime
	}
	if int64(other) > int64(^uint64(0)>>1)-int64(d) {
		return 0, ErrOverflow
	}
	return d + other, nil
}
