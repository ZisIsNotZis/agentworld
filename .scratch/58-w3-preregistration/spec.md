# Ticket 58 — W3 pre-registration: the four axes on the W1 world, judged on the curve

- Record kind: pre-registration of the W3 experiment and its comparison rule, declared before the
  first W3 live cell. No world change, no model call.
- Author: manager, 2026-10-27.
- Source: the operator's EXPLICIT order (`docs/VISION.md`, 2026-10-26): (1) the world runs, (2)
  `luna:max` always-on is the reference, (3) then the effect of model, thinking, kick-in timing and
  stability, (4) compared on per-hour KPI curves fuzzy-matched against the reference, never on the
  terminal outcome alone.
- Inputs: `W1` PASS (ticket 55), `W2` reference cell recorded (ticket 57), `curve/1`
  (`docs/VALIDATION.md`, ticket 35 — frozen, not edited here).

## 1. What W3 must answer

Four facts, each against the recorded reference curve:

| fact | question | axis |
|---|---|---|
| model | does a different model change the curve? | `-model` |
| thinking | does reasoning effort change the curve, and how much? | `-effort` |
| timing | at equal budget, does *when* the model is asked change the curve? | arms `P/U/A` vs `R` |
| stability | does a measured difference reproduce at another seed and another founder count? | cell set |

W3 does not decide the central claim; it supplies the measured facts and the reference-matched
distances that W4 judges. A null on any axis is a recorded result, not a stopped route.

## 2. Fixed world and reference

- World: mechanism v14 + `natural/14`, `Yield = 1`, `BirthCost = 2`, patches 2, `R = 8`, workers 1,
  `H = 144`, births enabled after the neutral h0 pulse.
- Declared cell: `f10-s0` (tickets 55/57). Stability cells: `f10-s1` and `f11-s2` (both pass the W1
  clauses on the committed no-AI grid; chosen before any W3 arm ran).
- Reference: arm `R` (`FullWakePolicy`, no priority), model `github_copilot/gpt-6-luna`, wire
  `responses`, effort `max`, claim contract `natural-claim/v4`, no call ceiling, 3 replicates per
  cell. Replicate 1 of `f10-s0` is the ticket-57 cell; two more replicates are run in W3.
- The reference is a **comparative proxy**, never ground truth (`docs/VALIDATION.md`).

## 3. The comparison rule (declared; `curve/1` reused, not tuned)

The unit is one condition's per-hour KPI curve, 3 replicates in one cell. The declared scalar KPIs are
`population`, `births_cum`, `deaths_starve_cum`, `deaths_age_cum`, `max_capital`; the histogram
families `capital_hist` and `granary_hist` are carried by Wasserstein-1 over buckets. `H = 144`.

- **Band:** mean ± 95 % CI plus the replicate envelope from the 3 replicates of the condition.
- **Paired difference:** for conditions A and B, `d(t) = A(t) − B(t)` at every hour.
- **Noise:** `RMSE(d)` over hours divided by `RMSE` of the replicate noise of the same condition at
  the same hours. `SNR = RMSE(d) / noise`.
- **Detectable separation:** `SNR ≥ 2` **and** `|d(t)|` outside the replicate envelope for at least
  **24 consecutive hours**.
- **Every cell:** a condition claim must hold in every declared cell of the matrix it is made on;
  otherwise it is "no detectable effect at this design", never equivalence.
- **Power:** beside every observed difference, report the smallest per-KPI difference separable at 3
  replicates at the observed bands. A difference below it is not evidence.
- **No scalar:** no weighted sum is defined. A condition never "wins" by one scalar; the per-KPI
  vector is the result.
- **Direction (description only, not a weight):** population and births higher is better for
  viability; starvation and age deaths lower is better. These directions describe a separation; they
  do not aggregate it.

### 3.1 The reference distance

For a condition C, the **reference distance** on KPI k is the paired C−R difference above, reported
as `SNR`, the first separating hour, the longest separating run, and the Wasserstein-1 over the
histogram families. C **matches** R on k when no detectable separation exists; C **differs** from R
when one does, with the direction stated. "Closer to R" means a smaller `RMSE(d)` with the same
noise denominator; it never means more faithful to reality.

### 3.2 The claim comparison (W4's input, not W3's verdict)

For the equal-budget claim, compare `A` against `P` and `U` with the same paired rule at the same
`N = 256` and the same cell/seeds/replicates.

- `A` **beats** a baseline B on KPI k when the A−B difference is favourable with a detectable
  separation (3.0) and no KPI separates against A.
- `A` **matches** B when no KPI separates in either direction.
- `A` is **worse** when a baseline separates against it on any KPI.
- **Supported** (M_end): `A` beats both `P` and `U` in every declared cell, and the pattern
  reproduces at a second cell and seed.
- **Not supported:** all three tie in every cell, or a baseline beats `A`.
- **Unresolvable here:** the reported detection limit exceeds every observed separation.

## 4. Declared matrix and order

One stage runs to a recorded verdict before the next starts. A stage's cost is measured, not assumed.

### Stage E — thinking (first)

- Model `github_copilot/gpt-6-luna`, arm `R`, cell `f10-s0`, 3 replicates, efforts
  `none`, `low`, `medium`, `high`, `max`. (`xhigh` is added only if the E curve separates and the
  budget allows; it is not part of the primary matrix.)
- Reference condition inside E: `R@max`.
- Verdict: for each effort, its reference distance (3.1) and the E-stage power. This answers the
  thinking fact on its own; it does not choose the effort for the other axes.
- Reproduce commands and raw ledgers kept under `.scratch/58-w3-effort/`.

### Stage T — timing (second)

- Model `luna`, effort `max` (the reference condition, so the only change is *when* the model is
  asked), cell `f10-s0`, arms `A0`, `R`, `P`, `U`, `A`, 3 replicates, `N = 256` for P/U/A, `A0`
  zero-call, `R` no ceiling (the reference).
- Verdict: the paired A−P and A−U comparisons (3.2) per KPI, with the power; this is the shape W4
  judges.
- Record under `.scratch/59-w3-timing/`.

### Stage M — model (third)

- Effort `max` (the reference condition, so the only change is the model); arm `R`; cell `f10-s0`; 3
  replicates; models `github_copilot/gpt-6-luna` (reference), `github_copilot/gpt-6-sol` (same family),
  and `github_copilot/grok-4.7` (a different family). All three are declared `responses`-wire models
  with a reasoning-effort field, so the only change is the model. The wire and the model are recorded
  in run identity.
  *Amendment (2026-10-27, before the first Stage-M cell): the originally declared third model was a
  chat-wire model (`zai/glm-5.3-flash`). The gateway client refuses a non-empty reasoning effort on
  the chat wire (the wire has no effort field), so a chat-wire arm cannot be run at "matched effort"
  without either omitting effort (confounding the model with the thinking axis) or inventing a wire
  mapping. It is replaced by `grok-4.7`, which keeps the matched-`max` design; the chat-wire case is
  recorded as a capability gap, not measured. No Stage-M cell had run when this was amended.*
- Verdict: each non-reference model's distance to the `luna` reference curve (3.1), with its wire
  named. Which model is "best" is not claimed: the reference is the benchmark.
- Record under `.scratch/60-w3-model/`.

### Stage S — stability (fourth)

- Repeat the decisive comparison (the Stage-T arms at `max`, model `luna`) on cells
  `f10-s1` and `f11-s2`, 3 replicates each, with `R@max` reference replicates on each cell.
- Verdict: the 3.2 pattern in each cell, and whether it reproduces; the cells/seeds are named, and a
  null cell is reported as a null cell, never pooled away.
- Record under `.scratch/61-w3-stability/`.
  *Amendment (2026-10-28, before any Stage-S cell): Stage S runs **`f11-s2` only**. The derived spend
  after E/T/M is $9.16 of the operator's ~$14 (¥100); one extra cell with its `R@max` reference
  (≈$2.8) fits, both declared cells (≈$5.6) do not. One second cell with a different founder count
  still answers "does the pattern reproduce"; `f10-s1` is a named follow-up if the operator raises
  the budget. The comparison rule and the verdict categories are unchanged.*

**Stop rules.** A stage that produces "no detectable effect at this design" is recorded and the next
stage proceeds. A stage that cannot complete inside its declared wall clock reports the skip and its
cause; it is not silently dropped. M_end cannot be delivered until E/T/M/S each has a recorded
verdict; a missing stage is a missing fact, not a negative.

## 5. Spend and wall clock

- The gateway records `github_copilot/gpt-6-luna` as **unpriced** (`cost_micros = 0`); the recorded
  charge is $0. The manager tracks the **derived** price at the ticket-35 rates ($0.10/M input,
  $0.50/M output) for the operator: W2's cell was $0.63 derived.
- The operator's global budget is ¥100 (~$14). Each stage reports its derived cost when it closes; if
  the remaining derived budget would be exceeded, the manager reports before the next stage and does
  not start it silently. A stage is never truncated mid-cell to fit a derived estimate; the recorded
  charge, not the estimate, is what the gateway bills.
- Wall clock is bounded per stage; a max-effort cell takes ~68 min at in-hour concurrency 10. The
  declared wall clock for a stage is 6 h; cells not completed inside it are recorded skipped with the
  cause.
- No live cell starts before this document is committed. No stage's rule is changed after its first
  cell; a rule found wrong is a new pre-registration and the old stage's data is reported as void.

## 6. Non-goals and limits

- No new world mechanism, policy, metric, contract version or reference model change.
- No scalar "best effort/model" claim; the operator asked for the effect, and the reference is a
  benchmark, not an optimum.
- The model and timing comparisons are model-conditional and synthetic; they are not claims about
  real civilizations.
- A tie is "no detectable effect at this design", never "no effect".
- The reference replicates are stochastic; the band is estimated from 3, so a small band can hide a
  difference the design cannot resolve. That is what the power number reports.
