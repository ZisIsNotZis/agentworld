// Command vs2 is the VS2 attention experiment runner (spec
// .scratch/24-vs2-attention/spec.md sections 4-9 and the lane board section
// 10, lane L4c). It runs the equal-budget arms A0/R/P/U/A over the declared
// natural world (mechanism v6, base policy natural/7, births enabled, H=168 h)
// and emits the frozen eight-indicator divergence vector plus the VS2 exit
// criteria VS2-EC-1..VS2-EC-9.
//
// It is hermetic by default: every model response comes from the L1
// deterministic FakeClient, so no network call is made and no spend is
// authorised. The value projection and the arm-local online accumulator (L1/L3)
// are exercised end to end; the run demonstrates the equal-budget ALLOCATION
// MACHINERY, never the central claim, which awaits real-model spend
// authorisation (VS2-EC-9, spec section 1.3). A live run is refused unless the
// operator explicitly authorises spend through the declared gate.
//
// The reference arm R (FullWakePolicy, every candidate each hour) is the
// comparative target of the fidelity metric, never ground truth (spec section
// 6); every arm's divergence is computed over the same full horizon H and
// every arm's realized call count is reported (spec section 5.3).
package main
