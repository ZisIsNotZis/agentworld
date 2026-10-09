package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

// The C7 cognitive-allocation policy object (spec section 11.1). It is a
// declared object, part of run identity. The VS1 instance is deliberately the
// honest uniform/periodic baseline: all actors in the selected hours, periodic
// in time. The escalation set is a pure function of (policy, hour, roster) and
// consumes no world random stream, so the frozen substrate's RNG positions are
// untouched; the roster is not consulted at all, which is exactly the "uniform
// over actors" property.
var ErrInvalidWakePolicy = errors.New("invalid wake policy")

// WakePolicy is the declared escalation policy (spec section 11.1).
type WakePolicy struct {
	Name        string `json:"name"`
	PeriodHours int    `json:"period_hours"`
	PhaseHours  int    `json:"phase_hours"`
}

// VS1WakePolicy is the declared VS1 instance: uniform over actors, periodic in
// time, every 6 hours from phase 0.
func VS1WakePolicy() WakePolicy {
	return WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0}
}

// FullWakePolicy is the declared high-budget reference instance (period 1).
func FullWakePolicy() WakePolicy {
	return WakePolicy{Name: "periodic", PeriodHours: 1, PhaseHours: 0}
}

// Validate refuses a policy that cannot define a periodic escalation set.
func (p WakePolicy) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidWakePolicy)
	}
	if p.PeriodHours < 1 {
		return fmt.Errorf("%w: period %d", ErrInvalidWakePolicy, p.PeriodHours)
	}
	if p.PhaseHours < 0 || p.PhaseHours >= p.PeriodHours {
		return fmt.Errorf("%w: phase %d outside [0,%d)", ErrInvalidWakePolicy, p.PhaseHours, p.PeriodHours)
	}
	return nil
}

// Escalates reports whether hour is in the escalation set: h ≡ phase (mod
// period). It is a pure function of the policy and the hour; an invalid
// policy escalates nothing rather than panicking.
func (p WakePolicy) Escalates(hour int) bool {
	if p.PeriodHours < 1 || p.PhaseHours < 0 || p.PhaseHours >= p.PeriodHours {
		return false
	}
	offset := (hour - p.PhaseHours) % p.PeriodHours
	if offset < 0 {
		offset += p.PeriodHours
	}
	return offset == 0
}

// Identity is the canonical wire identity of the policy, for the checkpoint
// cognition block.
func (p WakePolicy) Identity() string {
	body, err := canonicalJSON(p)
	if err != nil {
		panic(err)
	}
	return string(body)
}

// Fingerprint is SHA-256 over the canonical identity.
func (p WakePolicy) Fingerprint() [32]byte {
	return sha256.Sum256([]byte(p.Identity()))
}

// IsVS1Baseline reports whether the policy is exactly the declared VS1
// uniform/periodic instance.
func (p WakePolicy) IsVS1Baseline() bool {
	return p == VS1WakePolicy()
}

// canonicalJSON is the transport-free deterministic JSON encoding the wake
// policy identity is pinned with: struct field order is source declaration
// order, map keys are sorted, and HTML escaping is off.
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
