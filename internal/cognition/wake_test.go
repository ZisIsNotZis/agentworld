package cognition_test

import (
	"testing"

	"agentworld/internal/cognition"
)

// TestWakePolicyVS1InstanceIsDeclaredBaseline is the C7 wake-policy guard
// (spec 11.1): the VS1 instance is exactly the declared uniform/periodic
// baseline, the object is deterministic, and the escalation set is the pure
// periodic set. Mutating the policy's period, phase, or modulo logic makes a
// case here fail.
func TestWakePolicyVS1InstanceIsDeclaredBaseline(t *testing.T) {
	p := cognition.VS1WakePolicy()
	if p.Name != "periodic" || p.PeriodHours != 6 || p.PhaseHours != 0 {
		t.Fatalf("VS1 instance = %+v, want periodic/6/0", p)
	}
	if !p.IsVS1Baseline() {
		t.Fatal("VS1WakePolicy is not recognised as the declared baseline")
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("declared VS1 instance does not validate: %v", err)
	}
	// Deterministic object: two constructions are identical in every respect.
	q := cognition.VS1WakePolicy()
	if p != q || p.Identity() != q.Identity() || p.Fingerprint() != q.Fingerprint() {
		t.Fatal("VS1 wake policy is not deterministic")
	}
	if p.Fingerprint() != (cognition.WakePolicy{Name: "periodic", PeriodHours: 6, PhaseHours: 0}).Fingerprint() {
		t.Fatal("fingerprint does not cover the declared content")
	}

	// Escalation set: exactly hours congruent to phase modulo period.
	var selected []int
	for h := 0; h < 168; h++ {
		if p.Escalates(h) {
			selected = append(selected, h)
		}
		if p.Escalates(h) != p.Escalates(h+6) {
			t.Fatalf("escalation is not 6-periodic at h=%d", h)
		}
	}
	want := []int{0, 6, 12, 18, 24, 30, 36, 42, 48, 54, 60, 66, 72, 78, 84, 90, 96, 102, 108, 114, 120, 126, 132, 138, 144, 150, 156, 162}
	if len(selected) != len(want) {
		t.Fatalf("selected %d hours, want %d: %v", len(selected), len(want), selected)
	}
	for i := range want {
		if selected[i] != want[i] {
			t.Fatalf("selected[%d] = %d, want %d", i, selected[i], want[i])
		}
	}

	// The full reference instance selects every hour.
	full := cognition.FullWakePolicy()
	for h := 0; h < 24; h++ {
		if !full.Escalates(h) {
			t.Fatalf("full policy skipped hour %d", h)
		}
	}

	// Validation refusals; an invalid policy escalates nothing rather than
	// panicking.
	for _, bad := range []cognition.WakePolicy{
		{},
		{Name: "periodic"},
		{Name: "periodic", PeriodHours: 6, PhaseHours: 6},
		{Name: "periodic", PeriodHours: 6, PhaseHours: -1},
	} {
		if bad.Validate() == nil {
			t.Fatalf("invalid policy accepted: %+v", bad)
		}
		if bad.Escalates(0) {
			t.Fatalf("invalid policy escalated an hour: %+v", bad)
		}
	}
}
