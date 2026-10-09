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
