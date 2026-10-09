// Command vs1 is the VS1 experiment runner (spec section 11): the paired arms
// A0/A1/A1i/A2/A3 over the declared cells, the C7 wake-policy wiring, the
// descriptive scale/ceiling probe, the seven-indicator divergence-vector
// scaffold, and the frozen exit-criteria bands of spec section 12 and of
// docs/VERTICAL-SLICES.md's VS1 criteria (a)-(f).
//
// It is hermetic: every model response comes from L1's deterministic
// FakeClient, so no network call is made and no spend is authorised. The run
// therefore demonstrates that the *mechanism* is load-bearing (fake content
// changes outcomes); it cannot and does not claim that a real model is
// load-bearing, which awaits spend authorisation (spec section 16).
//
// The no-AI arm is the recorded ticket-19 natural/7 x v6 evidence
// (.tmp/natural9-n7-*.json); the runner re-runs every declared cell through
// this harness with a nil seam and reports byte-identity (EC-1).
package main
