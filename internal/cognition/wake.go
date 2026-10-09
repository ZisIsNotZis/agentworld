package cognition

import "agentworld/internal/cognition/protocol"

// The C7 wake-policy object lives in internal/cognition/protocol so
// internal/world can consume it without importing the model client. These
// aliases and wrappers keep the historical cognition spelling.
type WakePolicy = protocol.WakePolicy

var ErrInvalidWakePolicy = protocol.ErrInvalidWakePolicy

// VS1WakePolicy is the declared VS1 instance: uniform over actors, periodic in
// time, every 6 hours from phase 0.
func VS1WakePolicy() WakePolicy { return protocol.VS1WakePolicy() }

// FullWakePolicy is the declared high-budget reference instance (period 1).
func FullWakePolicy() WakePolicy { return protocol.FullWakePolicy() }
