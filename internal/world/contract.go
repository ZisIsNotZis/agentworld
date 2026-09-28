// Package world is the actor-facing intention boundary. Its callbacks receive
// value observations, not the trusted scheduler's Reader or kernel authority.
package world

import (
	"agentworld/internal/sim"
	"context"
)

type CacheStock struct {
	CacheID sim.EntityID
	Stock   sim.Value
}

type Observation struct {
	Actor        sim.EntityID
	Time         sim.SimTime
	Version      sim.WorldVersion
	Energy       sim.Value
	PublicCaches []CacheStock
}

type WithdrawEnergy struct {
	CacheID         sim.EntityID
	ObservedVersion sim.WorldVersion
}

type Intention struct {
	WithdrawEnergy WithdrawEnergy
}

type Decide func(context.Context, Observation) (Intention, error)

type Status uint8

const (
	Accepted Status = iota + 1
	Rejected
)

type Reason uint8

const (
	NoReason Reason = iota
	StaleObservation
	InvisibleCache
	InvalidTarget
	UnavailableStock
	InsufficientStock
	IneligibleActor
	InvalidNumber
	CacheCollision
)

// Outcome is a typed attempt record; EventID is zero for every rejection.
// It contains no component values, reader, authority, or raw kernel event.
type Outcome struct {
	Actor   sim.EntityID
	Action  WithdrawEnergy
	Status  Status
	Reason  Reason
	Time    sim.SimTime
	Key     string
	EventID sim.EventID
}

// Batch anchors the in-memory attempt journal to the scheduler's committed
// kernel head. This is not crash-atomic storage or replayable rejection history.
type Batch struct {
	Time     sim.SimTime
	Version  sim.WorldVersion
	TipID    sim.EventID
	TipHash  [32]byte
	Attempts []Outcome
}
