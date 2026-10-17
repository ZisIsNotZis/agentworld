# Changelog

This file is a **record of events**, derived from the git history. It is **not a source of truth**
for anything: current truth lives in the authoritative documents — the active plan
[docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md), the survival-substrate contract
[docs/P0-SPEC.md](docs/P0-SPEC.md), the substrate implementation record
[docs/P0-IMPLEMENTATION-PLAN.md](docs/P0-IMPLEMENTATION-PLAN.md), the validation strategy
[docs/VALIDATION.md](docs/VALIDATION.md), and the historical phase plan
[docs/ROADMAP.md](docs/ROADMAP.md). Revisions are abbreviated commit hashes.

## 2026-09-28

- **Ticket 01 — Foundation documentation.** Revision `dff8c25`.
- **Ticket 02 — P0 architecture and interface design.** Revision `dff8c25`.
- **Ticket 03 — P0 foundation implementation.** Revision `dff8c25`.
- **Ticket 04 — P0 transactional transitions and replay.** Revision `248acac`.
- **Ticket 05 — P0 actor fibers and scheduling.** Revision `521a3f2`.
- **Ticket 06 — P0 observation and intention boundary.** Revision `325aa52`.
- **Ticket 07 — P0 survival vertical slice.** Revision `080eb3d`.
- **Ticket 08 — Durable checkpoint and branch lineage.** Revision `c7ad43b`.
- **Ticket 09 — P0 scale and causal-validity expansion (bounded survival-collapse diagnosis).** Revision `31be3e1`.
- **Ticket 10 — Renewable food-flow pilot.** Revision `05f199f`.
- **Ticket 11 — First social interaction mechanism (design).** Revision `d2343b9`.

## 2026-09-29

- **Ticket 12 — Social food transfer v2 pilot.** Revision `628c558`.

## 2026-09-30

- **Ticket 13 — Productive capacity and surplus (design, v3).** Revision `d9f8861`.
- **Ticket 14 — Reproduction and genetics (design, v4).** Revision `77c5729`.

## 2026-10-05

- **Ticket 15 — Natural production limits and resource-determined population (design, v5).** Revision `92734f2`.
- **Ticket 16 — Surplus-generating behaviour (design, v6 policy over the v5 world).** Revision `d5df2cc`.

## 2026-10-06

- **Ticket 17 — Surplus reachability under the frozen v5 mechanism (design, v7).** Revision `43f1d38`.
- **Ticket 18 — Surplus via reproduction or land economics (design, v8).** Revision `89bd4ff`.
- **Ticket 19 — Surplus phase boundary and seed robustness (measurement).** Revision `1fe9768`.

## 2026-10-09

- **Ticket 20 — Exchange and trade (design, v9).** Revision `d758fbc`.
- **Pivot to vertical slices.** The independent vision-fidelity audit and operator drift concern were recorded, and the vertical-slice roadmap was frozen as the active plan. Revisions `e840c5d`, `e86a780`.
- **Ticket 21 — VS1 mind in the loop (design).** Records the VS1 design contract (spec `.scratch/21-vs1-mind-in-the-loop/spec.md`) and ADR `docs/adr/0006-cognitive-layer-recorded-model-calls.md`. Revision `e90f3c0`.
- **Ticket 21 — VS1 lane L1: model client, recorded-call ledger, replay, cognitive budget.** Revision `2692b37` (with `f8aa81b`).
- **Ticket 21 — VS1 lane L2: Role A decision seam, wake policy, observation encoder.** Revision `1e8497b`.
- **Ticket 21 — VS1 step 0: transport-free cognition protocol surface.** Revision `f9d14f3`.
- **Ticket 21 — VS1 lane L3: Role B versioned strategy patch lifecycle and checkpoint binding.** Revision `7826c42`.
- **Ticket 21 — VS1 lane L4: hermetic experiment runner, arms, bands, recorded-seam replay.** Revision `5e0bdaf`.

## 2026-10-10

- **Ticket 21 — VS1 completion gaps: EC-6 matrix evidence, EC-8 twin, live Role B path.** Revision `72fcb27`.
- **Ticket 21 — VS1 review-repair: active-generation seam identity, per-call sandbox parent, band evidence.** Revision `82549fc`.
- **Ticket 21 — VS1 residual closure: A2/A3 replay, divergent-parent fixture, band guards.** Revision `3ee7bf5`.
- **Ticket 21 — VS1 terminal closure: band completeness guards, fixture replay scope, declared notes.** Revision `f5ff597`.
- **Ticket 21 — VS1 documentation: current-truth evidence, status, changelog.** Revision `c1d7810`.
- **P15 — derive the `race` timeout from measured compute (90m) and document the `test` bound.** Revision `1605820`.
- **Merge — the VS1 vertical slice and the ticket-20 v9 wage exchange landed on `master`.** Merge commit `586757a` (parents `8ec0aa4` + `ebe5fd1`); the merged tree is identical to the gated revision `ebe5fd1`, whose clean full `make check` (60.27 minutes, no failures, no data races) therefore applies to it.
- **Operator decision — the application model.** The in-world model is `github-copilot/gpt-6-luna` (replacing the earlier `zai/glm-5.3-flash` choice), reached through the project's LiteLLM gateway; the gateway base URL, auth source, JSON-mode/seed support and price data are to be discovered from the environment by the gateway-binding lane, and the cognitive ceiling is declared and instrumented per P15. Recorded in `docs/VERTICAL-SLICES.md` (Product decisions) and `.scratch/21-vs1-mind-in-the-loop/spec.md` section 16.

## 2026-10-13

- **Ticket 22 — Gateway binding: bind the cognition client to the real LiteLLM gateway contract.** Revision `cf825b3` (reviewed tip `559ab6e`; merged-master integration evidence `46b1b2b`).
- **Ticket 23 — Documentation reconciliation: reconcile the founding docs with the measured VS1 truth.** Revision `b968334` (reviewed tip `a30fdda`; review notes `7f66802`).
- **Ticket 24 — VS2 attention design (the central claim).** Lands the accepted VS2 value-based cognitive allocation design and ADR `docs/adr/0007-value-based-cognitive-allocation.md`. Revision `b44ac3e` (reviewed tip `ed27c85`; review-residual record `fa4eba4`).
- **Ticket 25 — VS2 lane L1: additive priority protocol surface and integer value model.** Revision `8ecf4d0` (reviewed tip `7d9b22d`).
- **Ticket 26 — VS2 lane L2: world escalation gate and checkpoint policy binding.** Revision `53b094a` (reviewed tip `c02e6eb`; L2 review observations `2e4a25e`).
- **Ticket 27 — VS2 lane L3: cognition wiring (online allocation accumulator, `allocation` records, price adapter).** Revision `c72ba9d` (reviewed tip `ce30246`).
- **Ticket 28 — VS2 lane L4a: extract the shared experiment mechanics into `internal/experiment` and migrate `cmd/vs1`.** Revision `57770d2` (reviewed tip `7526205`; ticket-doc nit `e6d5466`).
- **Ticket 29 — VS2 lane L4b: recorded-seam replay/restore selector wiring.** Revision `b97ac5b`.

## 2026-10-17

- **Ticket 34 — VS2 live matrix (the central claim, real-model arm).** Branch `feature/34-vs2-live`
  (base `00de40f`, the post-reset tree). Live smoke gate PASSED (`P-f12-s0`, 256 calls, 182 model
  actions applied, replay identical). Primary live matrix 59/60 cells (arms `A0,R,P,U,A`, model
  `github_copilot/gpt-6-luna` via the local LiteLLM gateway, wire `responses`, ≈ 26 337 call
  records): VS2-EC-1/6/7/8/9 PASS; **VS2-EC-3 FAIL — a budgeted baseline strictly dominates adaptive
  at f10-s0 and adaptive dominates nowhere**, the declared §8.4 falsifying outcome for the value
  model `vs2-value/1` as instantiated; VS2-EC-2 FAIL (one fail-loud ceiling halt, `P-f10-s1`, caused
  by live transport retries being budget-charged per physical attempt); VS2-EC-4/5 FAIL (hour-0
  divergence rows; replay phase lost to the declared wall bound). The central claim is **not
  accepted**. Sensitivity extension (f=12, seeds 4–7) stopped by the declared no-progress rule after
  two attempts (upstream gateway throttling; 7/16 and 5/16 cells ran; artifact preserved).
  Discovered wire constraints, repaired under supervisor authorisation: the application model
  rejects `temperature` and `top_p` on the responses wire, and its reasoning output consumed the
  declared 128-token bound, so the responses codec omits the two parameters and applies a fixed
  `max_output_tokens: 512` (declared `FrozenSampling()` unchanged, recorded on every call record).
  Evidence record: `.scratch/34-vs2-live-matrix/spec.md`; branch tip `48090d0` (docs finalised at `afef7d2`; the run record's lane tip `53ff3a0` precedes the docs commits).

## 2026-10-14

- **Ticket 30 — VS2 lane L4c: equal-budget attention experiment runner `cmd/vs2`.** Revision `4a0ad28`.
- **Ticket 31 — VS2 lane L2b: candidate feature derivations and escalation predicate.** Revision `48b4882` (spec commit `2517e3a`, implementation commit `5c56098`).
- **Ticket 33 — VS2 lane L5: evidence record and current-state docs reconciliation.** Merge `2ef513b` (lane commit `ed438ef`, revision addendum `8ab4015`).
