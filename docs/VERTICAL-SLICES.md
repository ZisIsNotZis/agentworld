# Vertical-slice roadmap

## Document status

This is the active plan. `docs/ROADMAP.md`, `docs/P0-SPEC.md` and `docs/P0-IMPLEMENTATION-PLAN.md`
are **HISTORICAL** for planning purposes: ROADMAP.md records the pre-pivot phase plan,
P0-IMPLEMENTATION-PLAN.md the substrate implementation sequence, and P0-SPEC.md the survival-world
design contract. The P0/P1 exclusion of **language-model strategy editing is superseded**: both
language-model strategy editing and cognition are in this plan's FIRST slice (VS1), not deferred to a
later phase. P0-SPEC.md remains the design contract for the survival-world substrate only, and its
language-model exclusion no longer binds planning. The dated running record of events lives only in
[CHANGELOG.md](../CHANGELOG.md), which is a record of events and never a source of truth.

## Why this document exists

The operator's raised drift concern is partially confirmed: at the pivot the repository contained
**no language-model call path at all**, no cognitive-value scheduler, no intervention ledger, no
allocation-policy comparison, and no strategy mutation or rollout; `TokenCost` was hard-coded zero in
every reporter and `WakeStrategyInvalidation` had no producer. `docs/VISION.md`'s central claim —
that *selective* spending of expensive reasoning preserves civilization-level fidelity better than
*uniform* or *periodic* invocation at the same budget — has never been tested. Roughly 89% of
post-foundation effort went to a 16-actor synthetic economy, and the last five tickets measured
increasingly fine properties of constants the project had frozen itself. The independent adversarial
audit behind this reading is `.scratch/audit-vision-fidelity.md`.

The plan is vertical slices: every released version runs every concept end-to-end, and each slice
raises the fidelity ceiling of the hard concepts rather than adding one more concept layer at a time.

## The rule

> **Every released version is a complete, runnable civilization simulator. Every concept exists and
> runs end-to-end in every version. Slices raise the *fidelity and capability ceiling* of the hard
> concepts; they never introduce a concept that was absent before.**

A slice is accepted only when a single documented command produces a running world in which every
concept in the inventory below is demonstrably exercised, with recorded evidence and a falsifiable
exit criterion. Where verification requires subjective judgement, the uncertainty is stated.

## Final target

The last version is a **self-evolving universe scaled as high as our own compute allows** — the ceiling
comes from the machine, never from an arbitrary design cap (P15) — in which:

- **agents evolve themselves during the run.** Self-modification is not an offline authoring step: at
  key moments the system kicks in, changes behaviour logic, and the change takes effect inside the
  running world, with its measured effect reported (better or worse) rather than asserted.
- **every modification is durable, attributed, and externally auditable.** A change survives restart,
  names its proposer, parent, cause and validation, and can be verified by an outside party from the
  recorded ledger and replay **without trusting any agent's account of it**.
- **agents see things.** Every decision — model or cheap strategy — is made from the actor's own
  observation boundary; the differentiator is reasoning *under* a boundary, not privileged insight.

Milestones and slices are the same object: **M*n* = VS*n***, six of each.

## Concept inventory (must be present and runnable in every slice)

| # | Concept | "Present and runnable" means, at minimum |
|---|---------|------------------------------------------|
| C1 | Event-driven world time | Deterministic simulated time advances; every state change has an authorized cause |
| C2 | Actors with needs and stakes | Persistent individuals whose survival/reproduction depends on their own decisions |
| C3 | Resources and economy | Production, consumption, storage, spoilage, transfer, and a conservation-checked ledger |
| C4 | Strategy as versioned data | Behaviour is frozen, versioned, byte-pinned data, not code paths |
| C5 | Observation boundary | An actor's decision may consume only what that actor can observe; no privileged state |
| C6 | **Language-model intervention** | A real model call changes what an actor does, with recorded prompt/response |
| C7 | **Cognitive allocation** | A wake policy decides *which* decisions get expensive reasoning, under a budget |
| C8 | **Intervention ledger** | Every model intervention records cause, cost, patch, and measured result |
| C9 | **Strategy self-modification** | The system can emit a new strategy version with a parent ref, validated before use |
| C10 | **Evolution and selection** | Variants are compared and either propagate or are rolled back, with lineage retained |
| C11 | Budget and cost accounting | Compute, calls, and tokens are first-class, instrumented, fail-loud resources |
| C12 | Events, replay, determinism | Any run replays to the same state; model outputs replay from the ledger, never re-called |
| C13 | Metrics and paired experiments | Declared arms, declared metrics, multi-seed comparison, honest nulls |
| C14 | Scale and heterogeneity | Many actors, and actors that genuinely differ from one another |
| C15 | Institutions and society | Rules and organizations that arise from incentives and persist through enforcement |
| C16 | History and calibration | Model-conditional results, held-out comparison, sensitivity rather than one timeline |

**Coverage invariant (must hold at every slice boundary):** no C-row is empty. Later slices deepen
rows; they do not create them.

## Slices

### VS1 — Mind in the loop
*The first version that is not an LLM-free simulator.* Everything the project currently does keeps
running (C1–C4, C11–C14 in their existing measured form); what changes is that a **real model**
becomes part of the world's causal chain, under two distinct roles:

- **Role A — the mind of an actor.** At a declared decision seam, a chosen actor's decision is made
  by a model call whose input is exactly that actor's observation (C5) and whose output is an action.
- **Role B — the designer of behaviour.** Given trajectory evidence, the model proposes a **strategy
  patch** as a new versioned strategy (C4/C9): parent ref, proposer, declared intent, validation
  before use, recorded apply-or-rollback.

Minimum viable form of the hard concepts: C7 is a *declared wake policy object* (in VS1 the simplest
honest one — uniform/periodic — because it is the baseline the central claim must beat); C8 is the
ledger; C10 is one measured variation-and-selection round; C15 is the existing wage contract treated
as a proto-institution; C16 is model-conditional labelling only.

**Deliverables.** One documented command that runs a world in which models actually decide and
actually rewrite strategy; a recorded-call ledger sufficient for exact replay; live token/call cost
accounting with a fail-loud ceiling; paired arms with a declared metric.

**Exit criteria (falsifiable).** (a) A run in which removing the model provably changes outcomes, at
identical seeds — i.e. the model is load-bearing, not decoration. (b) Byte-exact replay of that run
with zero model calls during replay. (c) Every intervention has cause/cost/patch/result. (d) A
strategy patch is proposed, validated, applied, and measured; and a deliberately broken patch is
rejected and rolled back. (e) The privacy boundary holds: a model call's input contains nothing the
actor could not observe. (f) Costs reported per call, per actor-hour, and per run.

### VS2 — Attention (the central claim)
Deepens C7/C8/C11/C13 from "present" to "tested". Adds the value-based priority layer
(`expected civilizational value / expected cost`), escalation requests, and protected exploration.

**Status.** The M2 row of the milestone table is the one home for what the mechanism and the runs established. This section carries the question, the declared order in which the claim may be tested, and the metric's bounds.

## Milestone: exactly one, at the end

The roadmap has **exactly one milestone**, `M_end`, and it is the end. There is no intermediate
milestone, no gate, no progress report and no confirmation request: the worksets below are internal
steps of that one milestone, and the operator hears from the project once — when `M_end` delivers.

**`M_end` — the central claim measured on trajectory shape.**

- **Deliverable.** On a world that runs (W1), one recorded `luna:max` always-on reference curve set
  (W2), the per-hour KPI curves of every declared condition — model × reasoning effort × kick-in timing
  — each replicated across the declared cells and seeds (W3), and the fuzzy curve-match distance to the
  reference with its stability across cells, seeds and replicates (W4). Report form: the curves, the
  distance table, the stability table, and a TLDR.
- **Acceptance.** The claim is *supported* iff selective spending **beats** uniform and periodic
  spending at equal budget on the declared curve distance, and the effect reproduces under the
  pre-declared replication rule; it is *not supported* iff no allocation policy separates from the
  reference-matched baseline; it is *unresolvable here* iff the measured detection limit exceeds every
  observed separation — each verdict stated with the number that decides it. The operative "beats"
  reading was fixed before the data in the pre-registration (ticket 58 §3.2): the operator's word is
  "preserves ... **better than**" (`docs/VISION.md`), so a tie is the not-supported category, never
  support.
- **Result (2026-10-28).** W1–W4 are complete. The reference (W2) is alive and model-driven (mean
  15.7 alive, envelope 12–18, vs 11 at `A0`), but the model's answer equals the frozen cascade's
  action in 96.6 % (`luna`) / 94.7 % (`sol`) of calls at `R@max`, and in 97.9–99.9 % at the budgeted
  arms: the thinking and timing axes are null, `gpt-6-sol` is not separable from `luna`, `grok-4.7`
  is inadmissible at `max`, and the timing null reproduces at `f11-s2`. **The central claim is not
  supported on this world at this design** — a tie is the declared not-supported category (ticket 58
  §3.2), never equivalence. The full tables and the verdict are in ticket 62; the per-stage records
  are tickets 58–61. `M_end` is delivered for the operator's review.
- **Review.** `need_user_review`, once, on this single deliverable. Nothing before it requires the
  operator, and nothing before it is reported as a result. The deliverable is
  `.scratch/62-w4-curve-match/spec.md` plus the four stage records; the question for the operator is
  whether the measured answer (a decisive negative on this world, with the model nearly duplicating
  the frozen policy) answers the work order.

The earlier M1–M6 slice table at the end of this document is **historical record**, not roadmap: M1 and
M2 were milestones of the pre-restatement plan, and the workset order above supersedes them.

## Worksets (the work order)

The operator restated the order on 2026-10-26 ([VISION.md](VISION.md), `EXPLICIT`). One workset runs at a
time: a successor does not start until its predecessor's exit is measured and recorded. The rung table
below is a **diagnostic of the world and of the instrument**, not the work order; it is kept because its
measurements are what W1 and W2 consume.

### `W1_world_runs` — a founding population that does not die out

- **Goal.** The declared no-cognition default world keeps its founding cohort at the horizon (alive at
  `H` ≥ founders, age deaths outnumbering starvation deaths, the cohort replaced by births) **and is
expressive**: a model that kicks
  in has somewhere to act, rather than a world already in desperate death. The operator's words
  (2026-10-26): "we never try to make a perfect seed, we just wanna prove this seed can work, can be
  expressive, so our model which kick-in at least have moderate expressibility rather than desperate
  death". So the verdict is taken on the **declared default cell** (`f10-s0`, `H = 144`), never on a
  seed picked because it passed; the 128-cell grid is re-run as a regression, not as a search.
- **Why.** Ticket 43 measured 0/128 cells; a world that dies of its own aging clock cannot host any
  comparison, so everything downstream is unreadable until this holds. Measured binding order: founder
  aging, then birth economics (fertility 0.20–0.42 against the ≈1.0–1.5 replacement need), then food.
- **Do.** Finish the cause: the decision-instant fertility census (ticket 45, $0), then the smallest
  change the census names (a policy-floor version, or a world rule if the policy is not the cause), then
  re-sweep the 128 cells at `$0`.
- **Non-do.** No new ecology mechanisms; no metric work; no reference cost probe; no live model call.
- **AC.** On the declared default cell (`f10-s0`, `H = 144`, no cognition): (a) **the band holds** —
  `alive_end ≥ founders` and at least one birth; (b) **not a desperate death** — age deaths outnumber
  starvation deaths, so the population is limited by its own lifespan and land rather than dying hungry;
  (c) **expressive** — the founding cohort is replaced within one horizon, `births ≥ founders`; (d) the
  128-cell grid re-runs as a regression and every failing cell names its binding clause and cause.
  *Amendment (2026-10-26, after tickets 49–52): the earlier "zero starvation deaths" clause was a
  die-off criterion from ticket 43 and is wrong for a world with births — a population that overshoots
  its land and dies back is the modelled limit working, not a failure. What must not happen is that
  starvation outnumbers age, which is the operator's "desperate death" made measurable.*
  *Amendment (2026-10-27, ticket 55): clause (c) was "two actors hold the joint birth gate for at
  least 24 of 144 hours", measured with the pre-v8 symmetric gate at hourly checkpoints. Since v8 the
  implemented gate is asymmetric and is evaluated at the birth-reply instant, so the counter read 0
  gate-hours in a run that admitted 12 births (ticket 54). The clause is restated on replacement
  fertility, a quantity the gate exists to produce; the stale counter survives only as a diagnostic.
  No data was re-run. Record: [`.scratch/55-w1-criterion/spec.md`](../.scratch/55-w1-criterion/spec.md).*
- **Non-goal of record.** A search over seeds, founder counts or horizons for a cell that passes. If
  `f10-s0` cannot be made viable without changing the *world's* rules, that is `W1` failing and a
  decision for the operator — not a licence to pick another seed.
- **Status.** **PASS (2026-10-27, ticket 55) on the declared cell.** Declared default world:
  mechanism v14 + policy `natural/14`, `Yield = 1`, `BirthCost = 2`, patches 2, `R = 8`, `f10-s0`,
  `H = 144`, no cognition. The cell reads **alive 11 of 10, births 12, starvation 1, age 10**: (a) 11 ≥
  10, (b) 10 ≥ 1, (c) 12 ≥ 10, and the balance closes (10 − 10 − 1 + 12 = 11). The 128-cell grid at
  the same parameters passes **41/128** under (a)–(c); the 87 failures are classified by binding clause
  and cause (f=1 founder-count extinction; f=2–4 boom–bust starvation; f=5+ mixed survival and
  replacement) in `.scratch/55-w1-criterion/grid-128-w1.tsv`. The grid is a regression, not a seed
  search. Tickets 45–54 built the world: mate pool and household consent (v8), land yield (v9), wild
  yield (v11), founder placement (v10), one granary unit = two hours of life (v12), a declared birth
  price (v13), and a commons floor that keeps half of every patch wild (v14).
- **Decision it needed, and how it was settled.** The last open issue was the stale gate counter, not
  the world: the calibration admitted 12 conceptions while the pre-v8 boundary counter read 0. The
  clause was restated on replacement fertility rather than changing the world to satisfy a gate it no
  longer has; the rejected branch (more world rules to reach 24 boundary gate-hours) is recorded in
  ticket 55 §2. Record: [`.scratch/55-w1-criterion/spec.md`](../.scratch/55-w1-criterion/spec.md).
  The earlier world-rule analysis stays in
  [`.scratch/48-natural12-grid/spec.md`](../.scratch/48-natural12-grid/spec.md).

### `W2_reference_luna_max` — `luna:max` always-on as the benchmark

- **Goal.** A recorded `R@max` run (`github_copilot/gpt-6-luna`, effort `max`, every candidate every
  hour, no call ceiling) whose per-hour KPI curves are the benchmark every other condition is matched
  against, with its measured cost per cell.
- **Why.** It is the operator's reference and the only condition that can define "close enough". The
  recorded programme demoted `R` to a diagnostic and made the no-AI endpoint the anchor; that is the
  deviation this workset reverses. `R` already exists in the runner (`ArmR`, `FullWakePolicy`, ceiling
  4 096) and the effort ladder already accepts `max`, so no code is missing, only the role and a cost
  measurement.
- **Do.** One-cell cost probe at `-arms R -effort max` (cost reported before any grid), then the
  reference curves on the W1 fixture, recorded durably.
- **Non-do.** No budgeted arms in this workset; no metric change; no grid.
- **AC.** A recorded `R@max` cell produces the full per-hour KPI series at the W1 fixture, its price is
  reported to the operator before scaling, and the run's identity names model, effort and wire.
  Falsifier: `R@max` cannot complete a cell inside the declared wall clock or exceeds the reported cap.
- **Status.** **PASS (2026-10-27, tickets 56–57) on the declared cell.** The runner now carries the
  declared W1 world in one constructor (`declaredWorld`: mechanism v14, `natural/14`, `Yield = 1`,
  `BirthCost = 2`, `H = 144`) and records it in the evaluation; A0 reproduces the ticket-55 cell
  exactly. The live cell `R-f10-s0` is recorded (`.scratch/57-w2-concurrent-reference/`): **1,579 Role
  A calls** (every candidate every hour, no budget stop, ceiling 4,096), **993,037 prompt / 1,061,251
  completion tokens**, **0 fallback / 0 inadmissible / 0 errors**, 64 decisions diverged from the
  frozen policy; the gateway is unpriced (`cost_micros=0`), **$0.63 derived** at the published
  ticket-35 rates, inside the workset cap. KPI at `H`: **alive 17** (from 10 founders), births 24,
  starvation 7, age deaths 10; the full 144-hour series is `evaluation.json` and `kpi-series.tsv`.
  Wall clock **67.6 min** over 4.65 h of summed call latency: in-hour Role A calls are now issued
  concurrently (`-call-concurrency 10`, applied serially in wake order, R/no-Priority only) and the
  execution environment's first-segment kill was repaired by a ledger-replay `-resume`
  (`resume_from_hour=129`). Journal reconciliation and the replay lane both reproduce the whole cell
  from the ledger with zero model calls (`identical`). The run identity names model, effort and wire.
  The falsifier did not fire. The reference is **one cell**; the additional reference cells the
  stability axis needs are run inside W3's declared matrix, because W2's one-cell allowance is spent.
  A0/J1 evidence: ticket 56; concurrency, resume, and the reference record: ticket 57.

### `W3_axes` — model, thinking, kick-in timing, stability

- **Goal.** The four facts the operator asked for, each against the W2 reference: which model, which
  reasoning effort, when to kick in, and whether the effect reproduces.
- **Why.** This is the requested research; the recorded programme measured only timing (rungs 4 and 5) and
  two efforts, and never the model axis at all.
- **Do.** Add the model as a declared axis (`-model`, recorded in run identity — today it is hard-wired to
  `DefaultModelID`, so "a different model" is not runnable); run the effort ladder
  (`none…max`) and the timing arms (`P`, `U`, `A`) at equal budget; replicate across cells, seeds and reps.
- **Non-do.** No new world mechanism; no unrecorded arm; no spend before its criterion is pre-registered.
- **AC.** Each axis has a pre-registered comparison rule and a recorded verdict, with the reference curve
  as the baseline; the stability verdict names the cells and seeds it ran on.
- **Status.** **Complete (2026-10-28, tickets 58–61) under the pre-registration ticket 58** (the curve
  rule and stages E→T→M→S, declared before the first W3 cell; ticket 62 is the W4 judgement). All
  four facts are recorded:
  - **thinking** — no effort (`none/low/medium/high`) separates from `R@max` on any declared KPI at
    `f10-s0`; the `none` deficit (−2.0 population, −3.0 births at H) lasts 9–11 h, below the 24 h
    clause (ticket 58, `f71d44b`).
  - **timing** — `A` matches both `P` and `U` at `N = 256`; at `f10-s0` the longest A−U separation is
    14 h; at `f11-s2` `A`, `P`, `U` and `A0` are identical on every declared scalar at every hour
    (tickets 59/61, `a03d95e`, `6072bc5`).
  - **model** — `gpt-6-sol` matches `luna` (SNR 0.65–1.02, longest run 12 h, though it deviates from
    the frozen action 5.31 % vs 3.41 % of calls); `grok-4.7` cannot run at the declared `max` — the
    upstream returns HTTP 400 for `reasoning.effort=max`, and 200 without the reasoning block — so it
    is recorded as an admissibility failure, not a null (ticket 60, `a385ca5`).
  - **stability** — the null reproduces at `f11-s2`; the `f10-s0` sub-threshold departures vanish.
  The four stages cost **$10.52 derived** (including the $1.08 aborted first E attempt and the
  $0.63 ticket-57 carry-in) and ~7 h 54 min; the gateway charge is **$0.00** (the model is unpriced).
  The decision ledger explains the null: the model's answer equals the frozen cascade's action in
  96.6 % (`luna`) / 94.7 % (`sol`) of calls at `R@max`, and in 97.9–99.9 % at the budgeted arms.
  The two instrument defects found in E were fixed in `ed29dad`. W4/M_end judges the result:
  **the central claim is not supported** (ticket 62, `.scratch/62-w4-curve-match/spec.md`).

### `W4_curve_match` — the judgement is the curve, not the endpoint

- **Goal.** The comparison criterion is the fuzzy distance between per-hour KPI trajectories and the W2
  reference curves, per KPI and aggregated under a declared order.
- **Why.** The operator asked for curve matching "instead of only looking at the result".
- **Do.** Restore the divergence-from-reference objective (withdrawn by M2b only because the reference was
  itself extinct) on top of the existing curve machinery (`cmd/vs2/curves.go`: per-hour KPI series,
  3-replicate bands, Wasserstein-1, sustained separation), declare the aggregation, and declare its power.
- **Non-do.** No scalar-only "one arm is better" verdict; `vrd/2` stays as a diagnostic.
- **AC.** A declared matcher that scores any run against the reference curves, reproduces on recorded
  cells, and states its detection limit.
- **Status.** **Complete (2026-10-28, ticket 62).** The instrument was built inside W3 and its
  judgement rules were declared before the W3 grid (ticket 58). The result is
  `.scratch/62-w4-curve-match/spec.md`: 45 recorded cell rows, 35,830 decisions, $10.52 derived,
  **the central claim is not supported** — `A` ties `P` and `U`, the thinking and model axes are
  null, and the timing null reproduces at `f11-s2`.

### Superseded ordering: claim-establishment rungs (diagnostics)

**Claim-establishment rungs (declared order; a later rung may not be read before every earlier rung holds).** The order is the operator's (2026-10-19), restated here because the recorded programme departed from it: a harness that does not state the decision tests nothing, a world with no good ending has nothing for cognition to protect, a reasoning effect that has not been measured cannot be shown to be stable, and an allocation policy is only meaningful on top of a measured, stable premium. The rule is the stage rule below — a successor may not start before its predecessor passes — with two rungs the stage programme never ran placed before the two it ran first.

| rung | question | done when | status | spend |
|---|---|---|---|---|
| **1 · harness** | Does the request state the decision the actor actually faces, and do the answers fall inside what the world admits? | zero inadmissible answers in a declared probe with a falsifier: the model's own account of its options must agree with the code's admissibility ladder | **HOLDS** (M2a, ticket 36): 0/40 inadmissible under `natural-claim/v4`, 20/20 cells and 5/5 exact action sets agree | 40 probe calls |
| **2 · seed (SUPERSEDED 2026-10-27 — W1 passed on the rebuilt world; ticket 55)** | Does the world reach an ending worth protecting with **no** cognition at all? | a declared fixture and seed whose no-AI control is no worse at the horizon than at founding — alive at `H` at least the founding cohort, zero starvation deaths, at least one birth — measured at zero spend. The criterion is declared here, which is the operator's "a good ending even with zero kick-in" made falsifiable | **FAILED (measured on the whole admissible founder grid × declared seeds, ticket 43, 128 cells, `$0`).** Swept every admissible founder count `f = 1..16` × every declared primary and sensitivity seed `s = 0..7` at `H = 144`, `-arms A0` only: **0 of 128 cells pass**. `alive_end < N0` in 127/128, starvation deaths > 0 in 80/128, zero births in 34/128; the only cell reaching `alive_end >= N0` (`f6-s6`, 6 of 6) fails with 9 starvation deaths, and `alive_end` never exceeds 6 anywhere. The three recorded cells reproduce exactly (`f10-s0` 1 444/2/0, `f12-s0` 1 700/5/3, `f10-s1` 1 496/2/0, each `alive_end` 1). Binding mechanisms, measured: **founder aging is primary** — age deaths are 81–100 % of primary-cell deaths, begin at h99–h103 and end at h138–h144, while births begin at h5 and recur as late as h68 in the primary cells (`f11-s1`; h124 at `f6-s6` across the grid), so the control dies of age inside about one lifespan (at `f10-s0` the other two clauses already hold, so removing age mortality would end the same trajectory at 12 alive and pass — an unexecuted counterfactual, 12 = 10 + 2 − 0, assuming no induced starvation or birth change); **birth economics is the independent second binder** — `births/N0 = 0.20–0.42` at the primary cells, zero births at `f = 1, 8, 9, 16`; **food supply co-binds at larger cohorts** — starvation is 21.7 % of the grid's deaths and 0 at `f = 10`; **founder count is not a lever** — the outcome never improves to replacement. The control is a die-off under founder aging, so the `vrd/2` anchor is a floor on a collapsing trajectory, not on a viable world. Passing rung 2 needs either a fixture outside the declared seed sets, a world change, or a relaxed criterion — the sweep bounds seeds `0..7` at `H = 144` only (`spec.md` §6.8 states the limit); a world change collides with the product decision that the ecology arc is closed, and the world change and the criterion are the operator's call. Record: [`.scratch/43-rung2-seed/spec.md`](../.scratch/43-rung2-seed/spec.md) (declaration §1–§5 before any cell; per-cell table `a0-grid.tsv`; 128-row aggregate `a0-grid.json`) **Rung 2 is not yet diagnosed, only failed.** Ticket 44's census (ticket 44, `.scratch/44-fertility-calibration/`) shows the fertility gate is not intrinsically rare — two actors hold it for 107 of 144 hours under `natural/5` and 125 under `natural/9`, and the rarity under `natural/7` is caused by its own provisioning rule, not by the economy — while `natural/5` still refuses 135 times `consent-own` and births 3. The remaining cap therefore sits at the decision instants (`h+40m+5µs` request, `h+40m+6µs` reply), which an hourly-boundary census cannot see; the next step is that decision-instant census (read-only, zero spend), which decides between a policy-floor change (no world change) and a world rule. The household-gate design and its counterfactual (37–284 eligible pair-hours against 1–4) are recorded there as a candidate, not an adopted change.  | $0 (compute only) |
| **3 · influence** | How much does reasoning change the outcome? An effort ladder on the reference model first, then a different model at matched effort. | two conditions are distinguishable under `curve/1` ([VALIDATION.md](VALIDATION.md)) on a rung-2 fixture | **PARTIAL, and void without rung 2.** `high` versus `low` ran at one cell (`f10-s0`, ticket 35) and no KPI reached SNR 2 (population 0.49, births 0.82, starvation 0.05, age deaths 0.23, max capital 1.29) in a world whose population was capped at 12 and extinct by h156; `max` effort and every other model, including the `deepseek/deepseek-flash` candidate, were **unrun at the time** (W3 ran the effort ladder and the model axis in 2026-10-28, tickets 58–60; result ticket 62) | ≤ $1.10 per matrix |
| **4 · stability** | Does a measured difference reproduce across cells, seeds and replicates? | the pre-declared replication rule fires in the declared direction at a second cell and a second seed | **RAN** (M2d, ticket 42), uninterpretable without rungs 2–3: neither cell reproduced the M2c pattern — `f12-s0` separated both pairs in `A`'s favour, `f10-s1` tied both — and no AI arm separated from `A0` at any cell. **W3 Stage S re-ran the stability comparison on the W1 world (ticket 61): the timing null reproduces (ticket 62).** | ≤ $1.10 per matrix |
| **5 · kick-in strategy** | On a measured and stable premium, does selective allocation beat periodic and uniform spending at equal budget? | a rung-4-stable separation from `P` and `U` on a rung-2 fixture | **RAN BEFORE ITS PREDECESSORS, NULL** (M2c, ticket 41): `A` tied `P`, and the one separation it found over `U` did not reproduce; no arm has ever separated from doing nothing. **W3 Stage T/S re-ran the comparison on the W1 world (tickets 59/61): `A` ties `P` and `U` (ticket 62).** | ≤ $1.10 per matrix |

The caps are the operator's "limited budget" made numeric from measurement: $1.10 is what the largest matrix actually run cost (ticket 42: 5 945 attempts, $1.0666), so a rung needing more than one matrix returns to the discussion before spending. A rung's spend is authorised only after its criterion is pre-registered, as M2c and M2d did.

**Why the M2c and M2d nulls are not negative evidence.** With rung 2 failing, the only tier any arm moved was starvation deaths relative to arms that damage a die-off, and every arm including the control ends at one survivor; with rung 3 unmeasured, no arm had a premium to allocate. The nulls are equally consistent with three worlds — this world has no reasoning premium, the premium is invisible because there is nothing left to preserve, or the premium exists below what three replicates can resolve (ticket 41's detection limit: 42–61 actor-hours against a 12-actor-hour difference) — and the records do not distinguish them. That is what `curve/1`'s power number and Q13's routes exist to settle.

**Deliverable.** Equal-budget arms over the same world and seeds: full reference (model on every
decision), periodic, uniform-random, and adaptive-priority.

**Exit criteria.** At equal budget, adaptive beats periodic and random on a **declared** fidelity
metric; the result holds across seeds; it does not depend on a single scalar objective; and identity
and replay remain valid across strategy updates. A clean negative — no allocation policy
differentiates — is an equally decisive and reportable outcome, because it would show the decision
space does not yet reward reasoning.

**Declared metric.** `vrd/2` ([VALIDATION.md](VALIDATION.md)) — the priority-ordered viability order below the no-AI control's own outcome. The originally frozen definition, divergence from the high-budget reference run, is withdrawn as an objective; the eight-indicator vector survives as a diagnostic only.

**Metric supersession and repair (M2b ticket 37, repaired by M2c part 1 ticket 38, 2026-10-18).** The eight-indicator vector is retained as a diagnostic, but it is not the objective: at this fixture the reference itself is extinct, so approaching it rewards losing population. The declared objective is `vrd/2`, the priority-ordered viability order below the no-AI control's own realised outcome ([VALIDATION.md](VALIDATION.md); records `.scratch/37-fidelity-metric/spec.md` and `.scratch/38-m2c-preregistration/spec.md`). The frozen EC-3/EC-4 vector dominance rule is unchanged: `vrd/2` scores trajectories against a viability floor, and does not by itself declare one arm better than another. Ticket 37's declared pathology — its birth term outranked its survival term, so a fully-surviving sterile world scored 1.000 against an extinct one-birth world's 0.5963 — is repaired: `vrd/1` is withdrawn, `vrd/2` compares the same three components in the declared priority order (survival, then deaths avoided, then reproduction) instead of summing them, and no weight was changed. Arms are ranked by `vrd/2` only under a criterion pre-declared before the run (ticket 38 §7).

**Limitation.** The high-budget reference is a comparative proxy, never ground truth: it is itself a
model-conditional run, so a smaller divergence means "closer to the reference run", not "more
faithful to reality". [VALIDATION.md](VALIDATION.md) states the same bound for higher-budget
references.

### VS3 — Evolution
Deepens C9/C10 and C8 into a sustained loop: a population of strategy variants, sandboxed
counterfactual testing before rollout, staged rollout with rollback, lineage and provenance for every
patch, and selection pressure over generations of strategies. Exit criterion: measurable improvement
over successive strategy generations, with no unaudited mutation of reality and every rollback
recorded.

### VS4 — Stakes and scale
Widens the decision space so reasoning has something to buy — per-actor heterogeneity, space and
mobility, uncertainty/weather, conflict — and raises scale toward the P0 target. The world mechanisms
stopped in the ecology arc are re-entered here *because the allocation experiment needs them*, not as
a goal in themselves. Exit criterion: the VS2 result survives at scale and heterogeneity; the
marginal value of reasoning is measured, not assumed.

### VS5 — Society
Institutions and organizations from incentives and enforcement; trade networks; culture and knowledge
diffusion; multi-scale analysis. Exit criterion: institutions arise and persist only through
inspectable incentives, and rare innovations are not eliminated by scheduling or batching.

### VS6 — History
Calibration against bounded periods, conditional branches, sensitivity analysis, uncertainty
reported rather than narrated.

## Milestones (= slices)

| Milestone | Slice | Status | Accepted when |
|---|---|---|---|
| **M1** | VS1 | **Implementation complete; hermetic evidence PASS on all eleven frozen exit criteria (EC-1..EC-11); the real-model arm has since run live (tickets 57–61, 2026-10-28).** | **A mind in the loop.** The project is no longer an LLM-free simulator: a real model drives part of the world's decisions *and* modifies behaviour logic during the run, its interventions are costed and replayed exactly, and the modification is recorded with lineage and applied or rolled back under audit. |
| **M2** | VS2 | **Mechanism delivered and verified hermetically on master `48b4882` (VS2-EC-1..9 present; VS2-EC-3 a clean negative). The real-model arm ran on branch `feature/34-vs2-live` (ticket 34): on the declared primary live matrix VS2-EC-3 FAILS — a budgeted baseline strictly dominates adaptive at f10-s0 and adaptive dominates no cell — so the declared acceptance rule does not fire and the value model `vs2-value/1` is falsified as instantiated; VS2-EC-2/4/5 live failures and the incomplete sensitivity extension are recorded with their anatomy in ticket 34. Ticket 35 added a pre-registered curve criterion with depth (effort ladder) and redundancy (3 replicates) as first-class axes, and found the dominant defect is admissibility, not allocation: 40-53 % of v1 model answers were refused as inadmissible and `natural-claim/v2` removes that (0 in a 6 580-call run); `vs2-value/2` fixes the coverage collapse (Gini 0.14-0.23 vs 0.49-0.59) yet periodic sampling shows zero demographic effect and the v1-value-model arm is the closest to the R reference, whose anchor is itself unvalidated (Q1). No arm is called better; the central claim and the M2 milestone are not accepted. Tickets 37-42 then ran the stage programme: M2a passed (contract `natural-claim/v4`, 0/40 and 0/1 325 inadmissible answers), M2b passed (fidelity-to-R rejected as an objective; `vrd/2` adopted with a declared birth-dominance pathology and its repair), M2c answered on one cell (`A` separated from `U` at 0.1544 against a 0.0321 band; `A` tied `P` at 0.0083 inside a 0.0420 band, i.e. no detectable effect at 3 replicates rather than no effect), and M2d failed to replicate it (ticket 42: at `f12-s0` both pairs separate in `A`'s favour, at `f10-s1` both tie, and one `inadmissible` answer voided a single cell). The decisive measurement across all three cells is negative: `A` ties the no-AI control `A0` everywhere, `P` ties it on two cells and is worse on the third, `U` is worse on `f12-s0`, and no AI arm beats doing nothing at 248 decisions per arm. The M2c pattern is therefore cell-dependent in both halves, no single-cell result in this stage can carry the claim, and **Rung 2 of the claim ladder (VS2) did not hold on the world those runs used, which is why these nulls cannot be read as negative evidence:** the no-AI control ends at one alive in all three scored cells (`f10-s0` 1 of 10, `f12-s0` 1 of 12, `f10-s1` 1 of 10), so the anchor is a die-off and every separation found was damage avoidance inside it. What the stage actually established is the instrument (contract, metric, budgeted matrix at ~$0.57 and 17 minutes per 15 cells) plus the elimination of its own first hypotheses.** | **First evidence on the central claim.** Selective allocation beats — or decisively fails to beat — uniform and periodic spending at equal budget, across seeds, on the declared multi-indicator fidelity metric. |
| **M3** | VS3 | Not started | **Self-improving.** Strategies measurably improve over generations under audited mutation, with provenance and rollback for every patch. |
| **M4** | VS4 | Not started | **Stakes and scale.** The central-claim result survives heterogeneity and scale; the marginal value of reasoning is measured, not assumed. |
| **M5** | VS5 | Not started | **Society.** Institutions and organizations arise and persist only through inspectable incentives and enforcement. |
| **M6** | VS6 | Not started | **History.** Model-conditional calibration, conditional branches, sensitivity rather than one timeline. |


### M2 stages (declared order; a later stage may not start before its predecessor passes)

M2 is one milestone whose acceptance is still open. Its work is now staged, because ticket 35 showed the
instrument and the metric were being asked to answer a question neither could carry: a policy comparison
was run on a decision contract that cost 40-53 % of its answers to inadmissibility, and judged against a
reference anchor that rewards extinction. Each stage's acceptance criterion is fixed by its own ticket
**before** it runs; this table declares only the order and the rule that a stage may not begin before its
predecessor has passed. Mapped onto the VS2 rungs: M2a is rung 1, M2b is the instrument every rung needs
rather than a rung of its own, M2c is rung 5 and M2d is rung 4 — so as recorded this programme ran rungs
1, 5 and 4 and never ran rungs 2 and 3, which is the departure the rung order names.

| Stage | Question it settles | Status |
|---|---|---|
| **M2a — decision-contract calibration** | Does the model-facing request state the decision the actor actually faces, and do the actor's answers fall inside what the world admits? | **PASSED 2026-10-18, ticket 36**, against the criterion pre-declared in `.scratch/36-harness-vocabulary/spec.md` before the first probe call: **0/40 inadmissible** answers under `natural-claim/v4` (v1 baseline 86.7 %, v2 shortcut 0 %), and the model's own account of what it can do agrees with the code's admissibility ladder on **20/20 cells and 5/5 exact sets** (the falsifier). The criterion contained no action-variety threshold, so this stage claims none. |
| **M2b — fidelity metric and reference anchor** | Is "designed to approach the R reference" a valid objective at all? The R reference itself goes extinct at this fixture and horizon (ticket 35: `R` at 0-1 births and ≤1 alive at h143), so fidelity-to-R rewards a trajectory that loses population. Ticket 36 withdrew the stronger reading that cognition *suppresses* reproduction: under `natural-claim/v4`, the one full-wake replicate reproduced at the no-AI control's rate (2 births). The question is therefore the anchor, not an AI-arm effect. This is Q1 in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md): a civilization-level distance measure with a defended anchor. | **PASSED 2026-10-18, ticket 37**, against the criterion pre-declared in `.scratch/37-fidelity-metric/spec.md` before the first scored comparison, with zero live calls, at one fixture, one horizon and three replicates. **No:** fidelity-to-R is not a valid objective at this fixture. The objective is superseded by the viability-referenced measure to the no-AI control's own outcome ([VALIDATION.md](VALIDATION.md)); the eight-indicator vector is retained as a diagnostic. That measure was declared as `vrd/1` and is now `vrd/2` after M2c part 1 (ticket 38) repaired `vrd/1`'s declared birth-term pathology — a repair that changed no weight, component or anchor and left every recorded arm's relative order unchanged. Evidence: R scores 0.60-1.35 against A0's 0.00 while spending 1 082-1 305 calls versus 0; the extinct zero-birth `A1` arm is *closest to R* under the old instrument's equal-weight scalarisation (0.152 vs P 0.176) and *least viable* under the viability measure (1.193 vs P 0.000); all three pre-registered worse/better pairs pass in 9/9 replicate pairings (min margin +0.5963) and under three one-at-a-time weight doublings. The frozen instrument never scored A0 or R; ticket 37 scores both under its equal-weight scalarisation for AC-37(e), and the diagnostic's reference replicate is undeclared (recorded rows use R replicate 3). **What the pass establishes, and only this:** (i) the anchor is defined and defended — the no-AI control's own realised outcome, deterministic and recorded in the same matrix; (ii) the pre-declared discrimination requirement is met at this fixture; (iii) the measure it declared carried a **declared known pathology** — its birth term outranked its survival term, so a fully-surviving sterile world scored 1.000 while an extinct world with one birth scored 0.5963, and no declared sensitivity variant removed the inversion; ticket 38 repaired that pathology as `vrd/2` and records which of this stage's conclusions survive (`.scratch/38-m2c-preregistration/spec.md`, R5). M2b passing is therefore **not** evidence for the central claim. |
| **M2c — allocation policy on the calibrated instrument** | Under the M2a contract and the M2b metric, does any budgeted allocation arm separate from the periodic and uniform arms on demographic KPIs? (Ticket 35's arm comparisons are v1/v2-contract results and do not answer this.) | **Answered 2026-10-18, ticket 41 — one primary pair separated and one tied; the stage is not "passed" and its question is not settled in the general case.** The matrix ran once on a complete 15-cell artefact (ticket 41: 2 948 attempts, $0.5696, 0 halted, at most 248 decisions per budgeted arm under the `N = 256` ceiling with the `R = 8` transport reserve; `P` gate-bound at 244/244/228), after ticket 38 pre-registered the criterion (`.scratch/38-m2c-preregistration/spec.md` §7), ticket 39's first matrix printed `NOT RUN` (14 of 15 cells; one halted on its 257th attempt at the ceiling, because the quota schedule spent 100 % of `N` by construction while the ceiling charges physical attempts), and ticket 40 repaired the budget axis with the declared reserve. The answer at one cell (`f10-s0`), one contract (`natural-claim/v4`), one horizon (`H = 144`) and 3 replicates: **`A` vs `U` separates** — `A` more viable under `vrd/2`, actor-hours `0.1544` against a band of `0.0321`, 4.8× the band and 15× the declared tier-2 floor `0.01` — and **`A` vs `P` ties** — `0.0083` against a band of `0.0420`, 5.1× inside it and also below the `0.01` floor, so the reading is **no detectable effect at 3 replicates, never "no effect"**: the between-arm difference is 12 actor-hours, `P-r3` alone moves the pair exactly onto the boundary, the smallest difference separable at 3 replicates is 42-61 actor-hours, and same-arm replicate excursions reach 91 actor-hours. The pair that ties is the baseline a selective allocator has to beat, so the verdict is **not** "adaptive beats periodic"; tier 1 (persistence) separates nothing, tier 4 ties by one-sided construction, and the whole separation rests on one tier, one pair, one cell. The contract gate fired once (1 `inadmissible` in 2 948) and cell `A1-f10-s0-r2` is voided under the owning rule (ticket 41 §9.2), so every `A1` pair is reported not reportable and no `A1` claim is made. **This answer is what unblocks M2d:** the open question is now empirical replication at a second cell and a second seed, not a redesign of the policy. Part 1 of the stage delivered the instrument repair it owned: `vrd/1`'s birth-term pathology is repaired as **`vrd/2`** ([VALIDATION.md](VALIDATION.md)) — no weight changed, the aggregation alone, from an equal-weight sum to the declared priority order — and its order is demonstrated on the two worlds ticket 37 tabulated plus four more, including a declared witness for the tier-2-over-tier-3 tradeoff. **The central claim and the M2 milestone are not accepted.** Record: `.scratch/41-m2c-matrix-rerun/spec.md`. |
| **M2d — replication across cells and seeds** | Does any M2c separation reproduce at f12-s0 and other seeds? Every verdict after ticket 35's single-cell re-plan is single-cell, and the frozen rule's "in both cells" condition has been met only vacuously. | **Answered 2026-10-18, ticket 42 — `FAILED TO REPLICATE` under the rule pre-declared before the first live call, and the honest summary is that the M2c pattern is cell-dependent in both halves.** Two new cells: the declared second cell **`f12-s0`** and the second-seed cell **`f10-s1`** (cell labels are `f<founders>-s<seed>`, and seeds 0-3 at f = 10, 11, 12 are declared primary, so a second seed at the M2c founders is a declared primary cell), arms `A0/P/U/A/A1`, 3 replicates, horizon `H = 144`, contract `natural-claim/v4`, effort `low`, at most 248 decisions per arm under the unchanged `N = 256` ceiling with the `R_t = 8` reserve: **5 945 attempts, `$1.0666`**, 0 halted, 0 `inadmissible`, both artefacts at the full 15-cell inventory, and the three-label rule, its falsifier and four pre-run gates committed 90 s before the first live call (`5f3d0a3`, `17:28:25Z`). **Per cell:** at **`f12-s0`** the `A`-vs-`U` separation reproduced and grew (`0.4333` against a `0.1` threshold, tier 3, 4.3×) and the `A`-vs-`P` null **broke, thinly, in `A`'s favour** (`0.1667` against `0.1`, band-decided: 1.67 starvation deaths against a 1-death band, and a single `A` replicate at `S = 5` would have tied it) — reported as a failure of the null at that cell, never as support for the policy; at **`f10-s1`** neither primary pair separated (`A` vs `U` `0.0893 ≤ band 0.1053`, a tie carried by one random-arm replicate; `A` vs `P` `0.0000` against the `0.0144` floor) and every arm tied with the no-AI control, so that cell carried no information about any pair. The label is the only verdict the pre-declared table permits for "neither cell reproduces the pattern" and is kept unrelabelled, because revising a vocabulary after seeing data is the selection effect the pre-registration exists to prevent; that vocabulary has no word for a cell that strengthened the claim or for one that could not see it, so the next pre-registration must declare a fourth category, "no resolution at this cell". **What these two cells positively establish is a negative: `A` vs `A0` ties at all three cells, so no AI arm has ever separated from doing nothing.** The `f12-s0` separations are damage avoidance relative to arms that harm the control's outcome — `P` and `U` are both *worse* than `A0` there, `P`'s `f10-s1` replicates are numerically identical to the control's (248 applied decisions that moved no indicator), and 13 of `f12-s0`'s 16 deaths are age deaths, so the tier-3 statistic describes a minority mortality channel in a churn-dominated world. Two cells, two seeds, 3 replicates, one contract and 248 decisions per arm are still not the central claim and do not settle the stage; the M2c row above stands at one cell and is neither superseded nor rewritten. Record: `.scratch/42-m2d-replication/spec.md` §9-§10, with the durable per-cell KPI extract, the reproduction transcript and the independent review beside it. |

No stage of M2 changes the M2 milestone's own status: **the central claim and the M2 milestone remain not
accepted**, and M2a is instrument calibration, not evidence for the claim.

With M2d answered, the next question is not another allocation arm but whether this world contains a
reasoning premium at all: if no policy can do better than the no-AI control, selective spending has nothing
to win here and the claim is untestable in this substrate rather than false. That question, its three
routes and their costs are recorded as **Q13** in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md), awaiting the user's
choice; no route has been started.


## Cross-cutting invariants (apply from VS1 onward)

1. **Determinism with a nondeterministic component.** Model calls are recorded events; replay reads
   the ledger and never re-invokes a model. Any nondeterminism is captured, not wished away.
2. **Cost is a modelled resource.** Tokens and calls are budgeted, instrumented, and fail loud on
   ceiling breach; the P15 principle (limits come from modelled nature or our compute, never from an
   arbitrary design cap) applies to the cognitive budget.
3. **No unaudited mutation.** Every strategy change carries parent, proposer, validation, and rollout
   state, and every rollback is recorded.
4. **The observation boundary is the model's world.** A model sees what its actor observes; the
   project's differentiator is reasoning *under* a boundary, not privileged insight.
5. **Model-conditional labelling.** Results are labelled synthetic/model-conditional; simulation is
   not real-world proof.
6. **Honest nulls.** Frozen gates are never tuned after seeing data; nulls and falsifications are
   reported verbatim.
7. **External auditability of every modification.** The record of a self-modification must be
   sufficient for an outside party to verify it from the ledger and replay alone, without trusting the
   proposing agent, the running world, or this project's own summary of what happened.
8. **In-run kick-in.** Self-modification triggers from evidence available *during* the run, at declared
   key moments; an offline-authoring-only mechanism does not satisfy the concept (C9/C10).

## Product decisions

- The roadmap has exactly one milestone, `M_end` (top of this document); the pre-restatement
  slice milestones M1–M6 are historical record only.
- VS1 carries **both** AI roles: Role A (an actor's mind at a declared decision seam) and Role B
  (in-run behaviour-logic modification); Role B must kick in *during* the run.
- The work order is the four worksets above (`W1_world_runs` → `W2_reference_luna_max` → `W3_axes` →
  `W4_curve_match`), and a successor does not start before its predecessor's exit is measured.
- The declared default world, adopted at W1's exit (ticket 55): mechanism `NaturalMechanismV14` with
  policy `natural/14`, `Yield = 1`, `BirthCost = 2`, patches 2, `R = 8`, fixture `f10-s0`, `H = 144`.
  `W2`–`W4` run on this world; a change to it is a new declared world, not an edit.
- The declared comparison criterion is the per-hour KPI curve distance to the `luna:max` always-on
  reference, with the aggregation declared before the grid runs (`W4_curve_match`); `vrd/2`
  ([VALIDATION.md](VALIDATION.md)) survives as a diagnostic, not as the objective.
- Live spend is capped at one measured matrix per workset (≤ $1.10 at the recorded cost) until the
  operator says otherwise, and `R@max`'s price is reported before it is scaled. W2's one-cell
  allowance is spent (derived $0.63; the gateway itself records the model as unpriced, so the charge
  is imputed at the published rates, never fabricated). Further reference cells run inside W3's
  declared matrix.
- The recorded `R@max` reference cell is `f10-s0` on the declared world, model
  `github_copilot/gpt-6-luna`, wire `responses`, effort `max`, claim contract `natural-claim/v4`,
  `-call-concurrency 10`. Its per-hour KPI series is the baseline W3/W4 compare against; it is a
  model-conditional comparative proxy, never ground truth.
- Intra-cell call concurrency is a **runner scheduling parameter**, not a world mechanism: it applies
  only to the no-Priority reference wake policy, every request is built from its own observation
  before any call, and responses are applied serially in the fixed wake order. It is recorded in the
  run identity; a replay at a different concurrency reproduces the same world. Ledger *record* order
  within an hour is nondeterministic; decision/apply order is not.
- Application model: **`github_copilot/gpt-6-luna`** (operator decision; this gateway-exposed underscore id is the single declared application-model identity and supersedes the earlier `zai/glm-5.3-flash` choice); orchestration subagents: **`deepseek/deepseek-flash`**.
- The ecology arc is closed: no new ecology work follows it. The substrate it produced is the
  instrument VS1/VS2 build on.

## What stops, and what stays

**Stops:** new ecology mechanisms and further phase-boundary measurement. The ecology questions that
were answered remain answered; no new horizontal rung is opened.

**Stays:** the measured substrate — deterministic kernel with atomic commits and exact replay,
conservation-checked ledgers, byte-pinned versioned strategies, checkpoint branching, ceiling
instrumentation, and above all the **ticket 18/19 artifacts**: the measured map of where decisions
matter (f=10/11 reliable, f=12 fragile, f=16 arithmetic null) and the datum that decision quality
changes outcomes exactly there — at f=12 ticket 18's four gating cells (seeds {0,7} × workers {1,4})
split natural/7 2/4 versus natural/7b 4/4, while ticket 19's eight-seed phase map splits natural/7
2/8 versus natural/7b 7/8. Those are the substrate the VS2 experiment stands on.

## Ticket sketch for VS1

1. **VS1 design ticket** — the cognitive layer contract and a new ADR: model-call protocol, the
   recorded-call ledger, replay semantics, cost accounting and ceiling, the observation-boundary
   interface for model input, the strategy-patch lifecycle (propose → validate → apply/rollback), the
   declared decision seam, the declared arms, and the frozen exit criteria.
2. **Model client + ledger + replay** — GLM 5.3 Flash through the project's LiteLLM gateway, with
   deterministic replay from the ledger and live cost accounting.
3. **Role A** — the actor-mind seam, observation-bounded prompts, action validation.
4. **Role B** — strategy patch proposal, validation, apply, lineage, rollback.
5. **Experiment runner and metrics** — paired arms, per-call/actor-hour/run cost reporting, the
   declared fidelity metric scaffolded for VS2.
6. **Fresh independent review, gates, docs, merge.**
