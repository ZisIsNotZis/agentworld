# Ticket 62 — W4 / `M_end`: the four axes judged on the curve

- Record kind: the W4 curve judgement and the `M_end` deliverable, built from the four recorded W3
  stages. No live cell, no code change, no metric change.
- Author: manager, 2026-10-28.
- Source: the W3 pre-registration (ticket 58, `8f0f47b`; amendments `36083db`, `ac83ddd`) and the
  operator's EXPLICIT order (`docs/VISION.md`, 2026-10-26).
- Inputs: W1 PASS (ticket 55), W2 reference (ticket 57), Stage E (ticket 58, `f71d44b`), Stage T
  (ticket 59, `a03d95e`/`1adc3aa`), Stage M (ticket 60, `a385ca5`), Stage S (ticket 61, `6072bc5`),
  and the frozen `curve/1` rule (`docs/VALIDATION.md`, ticket 35).

## 0. TLDR

**The central claim is not supported on the declared world at this design. Selective spending does
not beat uniform or periodic spending at equal budget; it does not even separate from them.** The
four axes the operator asked for read:

| fact | answer on the declared world | record |
|---|---|---|
| thinking | **no detectable effect** — `none/low/medium/high` never separate from `R@max` on any declared KPI | Stage E |
| timing | **no detectable effect** — `A` matches both `P` and `U`; at the second cell all four conditions are identical to `A0` | Stages T, S |
| model | **no detectable effect** between `luna` and `gpt-6-sol`; `grok-4.7` **cannot run at `max`** (upstream HTTP 400) | Stage M |
| stability | **the null reproduces** at `f11-s2`; the `f10-s0` sub-threshold departures vanish | Stage S |

The mechanical reason is visible in the decision ledger: the model's answer equals the frozen
cascade's action in **96.6 %** (`luna`) and **94.7 %** (`sol`) of calls, and the no-AI, uniform,
periodic and adaptive arms produce the *same scalar trajectory* at `f11-s2`. The four-action claim
decision leaves almost nothing for reasoning to decide.

This is "**no detectable effect at this design**", never equivalence. The rule's 24-hour sustained
clause is what fails for every observed difference; the model is unpriced at the gateway (recorded
charge $0.00; $10.52 derived at the published ticket-35 rates, of which $1.08 is the aborted
over-concurrent first attempt).

## 1. The comparison rule applied

The declared rule is `curve/1` (ticket 35, frozen before the data): for each KPI, the paired per-hour
difference between two conditions, its RMSE over hours divided by the replicate-noise RMSE (`SNR`),
and a separation only when `SNR ≥ 2` **and** the difference stays outside the replicate envelope for
**≥ 24 consecutive hours**, in every declared cell. No scalar weighting exists. A tie is "no
detectable effect at this design", never "no effect".

The §3.2 claim comparison, declared in ticket 58 before the first W3 cell:

- **not supported** when all three of `A/P/U` tie in every cell, or a baseline beats `A`;
- **supported** only when `A` beats both baselines in every cell and the pattern reproduces;
- **unresolvable here** when the measured detection limit exceeds every observed separation. It does
  not fire: the observed population/birth deficits are above the reported power (2.24/2.88 in Stage
  M; 1.66/2.26 in Stage E); what fails is the 24-hour sustained clause, which the rule declares as
  "no detectable effect at this design", not "unresolvable".

Both declared cells are "all three tie", so the verdict is **not supported**. The `M_end` outline's
"matches or beats" is read as the *advantage* claim the operator stated ("preserves ... better"); a
tie is not an advantage. That reading was fixed before the data in ticket 58 §3.2.

## 2. The distance tables

### 2.1 Thinking (Stage E, `f10-s0`, 5 efforts × 3 reps)

Each effort against `R@max`; no KPI in any pair passes the rule (all ten pairs `NOT distinguishable`
in the fixed instrument as well). The most suggestive sub-threshold row:

| effort | population SNR / first / longest run / power | births SNR / first / run / power |
|---|---|---|
| `none` | 1.256 / h124 / 9 h / 1.66 | 1.253 / h124 / 11 h / 2.26 |
| `low` | 0.34 / – / 0 / 2.51 | 0.67 / – / 0 / 2.93 |
| `medium` | 0.35 / – / 0 / 2.02 | 0.62 / – / 0 / 2.70 |
| `high` | 0.42 / – / 0 / 2.19 | 0.57 / – / 0 / 2.76 |

The `none` deficit (−2.0 population, −3.0 births at H) is above the power and lasts 9–11 h; it is
recorded as a non-separation because it is shorter than 24 h. Stage E also recorded 11 level-1
fallbacks (8 contract-failing answers, 3 upstream HTTP 400 content-policy; 0.05 %), well below any
rate that could move a curve.

### 2.2 Timing (Stage T, `f10-s0`; Stage S, `f11-s2`)

`A − P` and `A − U`, equal ceiling `N = 256`, 3 replicates:

| pair / cell | KPI | SNR | first | longest run | power | d at H | verdict |
|---|---|---:|---:|---:|---:|---:|---|
| A−P / `f10-s0` | population | n/a (both deterministic) | 122 | 6 h | 0.000 | +1.00 | tie |
| A−U / `f10-s0` | population | n/a | 110 | 14 h | 0.000 | +3.00 | tie |
| A−P / `f11-s2` | all | n/a | – | 0 h | 0.000 | 0.00 | identical |
| A−U / `f11-s2` | all | n/a | – | 0 h | 0.000 | 0.00 | identical |

At `f10-s0`, `U` is identical to the no-AI `A0` at every hour; `A` differs from `A0`/`U` at 28
hours and from `P` at 13 hours, all sub-threshold. At `f11-s2`, `A`, `P`, `U` and `A0` are identical
on all five scalar KPIs at every hour. At `f10-s0` the realized budget is 240 (P) vs 248 (U/A) under the shared
`N = 256` ceiling; at `f11-s2` all three realize 248; the asymmetry is recorded and does not explain a separation because there is
none.

### 2.3 Model (Stage M, `f10-s0`, `R@max`, 3 reps)

| model | calls | population SNR / first / run / power | births SNR / first / run / power | deviation from frozen |
|---|---:|---|---|---:|
| `luna` (reference) | 4,574 | – | – | 3.41 % |
| `gpt-6-sol` | 4,424 | 0.900 / h134 / 7 h / 2.24 | 1.02 / h133 / 12 h / 2.88 | 5.31 % |
| `grok-4.7` | 0 | **HTTP 400 at `effort=max`** (20/20 smoke calls; 200 with the reasoning block removed) | – | – |

`sol` matches `luna`: no KPI passes the rule, although `sol−luna` is −3.33 alive and −5.67 births at
H and the decision seam differs 1.56× more often. The 12-hour longest run is below the 24-hour
clause. `grok` is an admissibility failure, not a null; no substitute and no effort re-map was made.

## 3. The stability table

| cell | A0 at H | A, P, U at H | A−P / A−U | reading |
|---|---|---|---|---|
| `f10-s0` | 11 alive / 12 births / 1 starvation / 10 age | P 13 / 15 / 2 / 10; U 11 / 12 / 1 / 10; A 14 / 15 / 1 / 10 (A differs from A0 at 28 h, sub-threshold) | tie / tie | null |
| `f11-s2` | 12 / 12 / 0 / 11 | identical to A0 at every hour | identical / identical | null reproduces |

The `f11-s2` A0 reading reproduces the committed W1 no-AI grid row exactly. The Stage-T
sub-threshold departures do not reproduce; the categorical pattern does.

## 4. What this establishes, and what it does not

**Establishes (on the declared world, two cells, H = 144, 3 replicates):**
1. The model can be a load-bearing part of the world in principle (the `R@max` reference differs from
   `A0`: mean 15.7 alive, envelope 12–18, vs 11; 24 vs 12 births on the ticket-57 cell), but at the budgeted arms it almost always chooses the
   frozen action, so *when* and *with how much effort* it is asked does not move the declared KPIs.
2. A different reasoning effort (including `none`) is not distinguishable from `max` on the curve.
3. A same-family model (`gpt-6-sol`) is not distinguishable from `luna`; a different-family model
   (`grok-4.7`) is not runnable at the declared `max` condition.
4. The `A` vs `P/U` null reproduces at a second cell; there was no detected effect at `f10-s0` to reproduce.

**Does not establish:**
- equivalence (the rule forbids it);
- that no reasoning premium can exist in a richer world — the four-action claim contract and the
  wage economy leave almost no decision for reasoning to change; VS4's stakes/heterogeneity and
  VS3's strategy evolution are the routes that could create one;
- anything about real civilizations: the result is synthetic and model-conditional, and the
  reference is a comparative proxy, never ground truth;
- the behaviour of `grok-4.7` at a runnable effort, or of any chat-wire model at matched effort (the
  wire has no effort field; that path is a recorded capability gap);
- a second seed at the `f10` founder count (`f10-s1` was dropped by the derived-budget amendment
  before Stage S; one second cell was run).

## 5. Power and limits

- Stage E's smallest separable differences: population 1.66–2.51, births 2.26–2.93; the `none`
  deficit is above it in magnitude but below the 24-hour run.
- Stage T's replicates are identical (noise 0), so the `SNR` gate is undefined and the sustained
  clause is the only bound; the largest observed difference (A−U, 3 actors) lasts 14 h.
- Stage M has real noise (population 1.118, births 1.440); `SNR` 0.65–1.02; the observed
  `sol−luna` deficits are above the power (2.24/2.88) but last ≤ 12 h.
- Stage S's arms are identical, so the power is degenerate (0) and the sustained clause is the only
  bound; the null is categorical, not a small effect.
- Two cells (`f10-s0`, `f11-s2`), one horizon (144 h), three replicates, one claim contract
  (`natural-claim/v4`), one world (mechanism v14, `natural/14`, yield 1, birth cost 2).
- `H = 144` is one founder lifespan; a premium that needs a longer horizon or richer institutions
  is outside this design.
- The gateway records the model as unpriced; the derived spend is an estimate, not a charge.
- Stage E's raw hash-chained ledgers (~1.1 GB) and final checkpoint bundles were deleted after
  extraction; the committed evaluations, per-call TSVs and KPI series support every reported number,
  but a byte-level ledger replay of those cells would require re-running them. Stages T/M/S retain
  their evaluation JSONs and calls TSVs; their replay verdicts are in the records.

## 6. Cost of the answer

| stage | cells | calls | derived $ | wall |
|---|---|---:|---:|---:|
| W2 reference carry-in (ticket 57) | 1 | 1,579 | 0.63 | (67.6 min) |
| E fresh (thinking) | 14 | 20,935 | 4.19 | 2 h 10 min |
| E aborted over-concurrent attempt | 0 | 7,466 attempts | 1.08 | 12 min |
| T (timing) | 12 | 2,208 | 0.88 | 1 h 47 min |
| M (model) | 3 | 4,424 | 1.30 | 35 min |
| S (stability) | 15 | 6,684 | 2.44 | 3 h 10 min |
| **total W3 recorded** | **45** | **35,830** | **10.52** | **~7 h 54 min** |

Recorded gateway charge: **$0.00** (the model is unpriced; the derived figure uses the published
ticket-35 rates). The W2 reference cell's $0.63 is inside the Stage-E row (the ticket-57 carry-in).
The operator's global budget is ¥100 (~$14): the derived total is $10.52, and the remaining ~$3.5
was not spent (the second stability seed was dropped by the pre-declared amendment).

## 7. Reproduction

Every stage record carries its exact command, binary build, cells, seeds, replicates and analysis
script. The quickest read:

```
.scratch/58-w3-effort/spec.md
.scratch/59-w3-timing/spec.md
.scratch/60-w3-model/spec.md
.scratch/61-w3-stability/spec.md
```

The stage analyses are read-only over the recorded evaluation JSONs; no cell needs to be re-run to
check a number. The reference is the ticket-57 cell plus the Stage-E max replicates; the W1 cell is
ticket 55; the world and policy are frozen at v14/`natural/14`.

## 8. The milestone

`M_end` is delivered as this record plus the four stage records and their raw evaluations. The
acceptance is the operator's review: does the answer — **the central claim is not supported on this
world; the thinking, timing and model axes are null at this design; the null reproduces** — answer
the question the work order asked? The review is `need_user_review`, once.

The project does not stop here. The next roadmap is VS3 (evolution) and VS4 (stakes and scale), where
the decision space can be made deep enough for a reasoning premium to exist; that route is the
operator's to choose.
