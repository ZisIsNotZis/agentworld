# Surplus phase boundary and seed robustness (measurement, ticket 19)

Status: claimed

## Issue

Ticket 18 established that surplus is *reachable* for the first time at sub-saturation density, but its verdict was FAILED under the frozen all-four-cells quantifier because the gating arm `natural/7` reached the indicator in only 2 of 4 cells (seed 0 yes at h32, seed 7 never), while `natural/5`, `natural/6` and the banking-only `natural/7b` reached it in all four. The design's own honest-null list already declared the density **phase boundary between ½ and 1 unmeasured**. Two consequences follow: the reachability claim currently rests on four cells per arm and two seeds, and the next step in the operator's arc (exchange/trade, now meaningful because surplus exists at all) needs a settled substrate — a map of *where* surplus is reliable and *who* holds it.

This ticket measures. It introduces **no mechanism, no constant, no band and no policy**; it sweeps the already-declared axis (`-founders`, the frozen policies, the frozen v6 mechanism) and reports the phase map verbatim.

## Frozen matrix and gates (parent-frozen as a measurement ticket; every band is reused verbatim from ticket 18, nothing new is declared)

Bands: the frozen **V8-A2** (per-actor `k≥2 ∧ granary≥1 ∧ trailing-24h SF′>0` sustained ≥12 consecutive hours, first such hour ≤96), **V8-A3** (filing ≤h48, first committed birth ≤h96 on the under-capacity patch), the **V8-A4** descriptive taxonomy and **V8-A2b** (no saturated-patch actor reaches the indicator), each evaluated exactly as the v8 code already does.

Matrix (all on world v6, `-births=both`, so the enabled child carries the bands and the disabled child doubles as the births-off record):

- **Density sweep:** `f ∈ {10, 11, 12, 13}` × seeds `{0,1,2,3,4,5,6,7}` × policies `{natural/6, natural/7, natural/7b}` × workers `{1}` = 96 runs.
- **Saturation controls:** `f = 16` × seeds `{0,1,2,3,4,5,6,7}` × policies `{natural/6, natural/7}` × workers `{1}` = 16 runs (the arithmetic NULL must reproduce on every seed).
- **Policy invariance anchors:** `f = 12` × seeds `{0,1}` × policy `natural/5` × workers `{1}` = 2 runs (the strongest arm; already 4/4 at two seeds).
- **Worker determinism:** one density cell (`f=12`, seed 0, `natural/7`) at workers 4 = 1 run, digest-compared to its workers-1 twin.

Deliverable — the **phase map**, per (f, policy): the count of seeds where V8-A2 holds out of 8, the first-sustained hour for each holding seed, the A2b/A3 outcomes, the A4 taxonomy bucket, and the enabled child's births and survival. Plus a **exchange-precondition measurement computed from the existing reports** (no reporter change): for each cell, the hours in which a surplus-bearing actor coexists with at least one alive actor that holds no cultivated stock and no granary food (the heterogeneity that would make a transfer meaningful), and the identity of the surplus holders (founder generation vs newborn generation), so the trade ticket knows whether surplus concentration is generational.

## Expected outcomes and falsification

- **Expected:** A2 reachability increases monotonically with `f` toward the saturated arithmetic NULL at `f=16`; `natural/7b` ≥ `natural/7` in reachability (the ticket-18 store-falsifier pattern); every `f=16` cell NULL on every seed; births reachable wherever A2 holds.
- **Falsified if:** any `f=16` saturated control reaches A2 (the Malthusian arithmetic reading would be wrong), or reachability is non-monotone in `f` in a way the record must explain, or a policy ordering contradicts the ticket-18 store result.
- A null or anti-monotone result is a **valid outcome** and must be reported verbatim; no band, constant or seed may be changed after the first run.

## Constraints

- No code change of any kind is required or authorised: `cmd/natural` already carries `-founders`, `-policy`, `-mechanism` and `-births`. If a lane concludes a reporter field is needed, STOP and report instead.
- Every run is a fresh verified neutral root under the frozen two-flag discipline; reports land in `.tmp/natural9-*.json`; the evaluation is written to `.tmp/natural9-evaluation.txt` and summarised append-only in this ticket.
- The v5/v6/v7/v8 evidence and every frozen verdict stay untouched; this ticket adds measurements, not verdicts. Ticket 18's FAILED verdict stands as recorded.
- `token_cost = 0`; reports stay `synthetic/model-conditional`; costs reported against the inherited budgets.

## Lane board

One measurement lane (runs the matrix with the existing binary, writes the evaluator and the phase map, appends the ticket record) → parent verification (spot-check the phase map against the JSONs) → the next design ticket on a settled substrate.

## Comments

- 2026-09-29 — assistant: Opened as a parent-frozen measurement ticket after the v8 merge (`716ded4`), to convert a seed-fragile verdict into a characterised phase boundary and to measure the exchange precondition before the trade arc opens. No mechanism, constant, band or policy is introduced; every band is reused verbatim from ticket 18.
- 2026-09-29 — assistant: **Ruling recorded — the exchange precondition is to be both bracketed and computed exactly, without any source change.** The lane showed that the reports carry no per-hour per-actor capital or granary stock, so the frozen wording's coexistence clause is not exactly decidable from them (the k=0 clause is exact only for the never-capital subclass and the per-hour granary clause for nobody), and it correctly refused to pick a proxy silently. Ruling: (1) report BOTH bounds from the existing reports and label them as bounds, not measurements — an over-approximation (hours where a surplus-bearing actor coexists with an alive actor whose `alive_hours_with_k_ge1` is 0, with the granary clause disclosed per cohabitant from end-of-run holdings) and a strict under-approximation (that cohabitant must end granary-empty), with the reason for the gap stated verbatim; (2) **preferred upgrade within the lane's ownership**: the retained checkpoint bundles carry the full accepted history, so a `.tmp/` analysis tool may restore each bundle through the world package's own restore path and step it to recover exact per-hour per-actor holdings — no reporter change and no source edit, which is all the ticket's constraint forbids — provided the replay is first verified to reproduce the run (against the report's recorded digests, `alive_by_hour` and per-actor survival), with a documented fallback to the bracketed bounds for any cell whose replay does not reproduce; (3) the exactly-computable generational identity of surplus holders (founder vs newborn) is a first-class deliverable because the trade ticket needs it. If a per-hour holdings reporter field still seems worth having afterwards, the lane records the recommendation for a separate ticket with its motivating evidence rather than adding it here.
