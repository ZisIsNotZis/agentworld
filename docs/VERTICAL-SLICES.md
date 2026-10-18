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

**Status (master, tickets 37-42).** The slice's mechanism questions are settled and the claim's own test is not: `natural-claim/v4` makes the actor's answers admissible (0/40 and 0/1 325 refused) and `vrd/2` replaces the R-anchored distance with a viability order against the no-AI control, but the arm comparison at 248 decisions per arm returned no advantage over doing nothing at any of the three scored cells. `A` ties `A0` on every cell, `P` ties it on two cells and is worse on `f12-s0`, `U` is worse on `f12-s0`, and the only separations found are between AI arms, which is why the M2c pattern did not replicate (ticket 42). **The central claim is untested rather than supported**: before another allocation arm is run, the open question is whether this world contains a reasoning premium at all — Q13 in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md), awaiting the user's choice of route. Records: `.scratch/41-m2c-matrix-rerun/spec.md`, `.scratch/42-m2d-replication/spec.md`.

**Status (branch `feature/35-vs2-curve`, ticket 35, added on top of the ticket-34 record below).**
The VS2 question is now asked with a **curve criterion** and the axes of **depth (reasoning effort) and
redundancy (replicates)** are first-class. The criterion is pre-registered in
`.scratch/35-vs2-curve/spec.md` and was committed before any live call: the unit of analysis is the
per-hour KPI curve of a condition replicated 3×; the band is the mean ± 95 % CI plus the replicate
envelope; between conditions, `SNR` = RMSE over hours of the paired difference divided by RMSE over
hours of the replicate noise, with the first-separation hour, the longest separating run and a
Wasserstein-1 distance; a condition is distinguishable only if some KPI holds SNR ≥ 2 with a ≥ 24-hour
separation in every cell, and **no condition is called better on a scalar**.

Under that criterion the lane also found that **depth is not resolvable at this fixture**: full-wake at
`high` is **not distinguishable** from full-wake at `low` on any KPI (population SNR 0.49, births 0.82,
starvation 0.05, age deaths 0.23, max capital 1.29 — all below the pre-declared threshold), because the
fixture saturates (population capped at 12, extinction by h156, zero births under full cognition), so
`max`@v2 was deliberately unrun rather than spending 15-17 h of wall time to learn the same thing. Every
verdict after stage 1 is single-cell (f10-s0), so the criterion's "both cells" clause is met vacuously
rather than substantively: reproduction at f12-s0 is untested. The
remaining findings, live and spend-authorised: the ticket-34 ledger's `max_tokens: 128` provenance lie is fixed
(applied effort, output bound and response-body bound are recorded per call beside the declared
sampling); the frozen 30 s attempt timeout and the spec's 64 KiB recorded-body bound were both silent
live decision-loss channels at long-reasoning effort (the body bound rejected ~30 % of max-effort calls
as `oversized` until a live-only 1 MiB bound was declared); **40-53 % of every arm's model answers were
well-formed JSON proposing an action the world refuses** (882 of 917 were `eat-stored` with no granary
stock or a meal already taken), which is the standing explanation for the earlier null — and under the
new `natural-claim/v2` contract, which states the observation-derived admissible action subset, that
falls to 0 inadmissible in a 30-request probe and 0 across a 6 580-call run. `vs2-value/2` (a
staleness term plus a per-actor coverage floor) fixes the coverage collapse it was built for (Gini
0.14-0.23 against `vs2-value/1`'s 0.49-0.59 in the same run, same contract). Two results cut the other
way and are recorded rather than buried: **periodic sampling has zero DEMOGRAPHIC effect at this
fixture** (244 applied model decisions leave population, births, starvation deaths, age deaths and
exchange identical to the no-AI world at every hour and in every replicate, while wealth and granary state
still differ), and **the v1 value model, whose concentrated coverage was called a defect, is the arm
closest to the R reference while the "fixed" model is indistinguishable from A0 *on the demographic
KPIs*** (the frozen instrument still separates them on capital and granary histograms). R produces 0-1
births and dies out in those runs, so **fidelity-to-R rewards a trajectory that loses population — the R
anchor is an unvalidated objective** (open question Q1: a civilization-level distance measure). The earlier
reading that *cognition suppresses reproduction* is **withdrawn** (ticket 36): under `natural-claim/v4`, a
contract that loses no answer to inadmissibility, one full-wake replicate at the same cell, horizon, effort
and bounds produced 2 births — the same as the deterministic no-AI control — so the v1/v2 birth deficits are
not separable from instrument loss, and Q1 rests on the reference's own extinction, not on an AI-arm effect.
No arm is called better, the ticket-34 verdict stands unchanged, and **the central claim and the M2
milestone remain not accepted**. Record: `.scratch/35-vs2-curve/spec.md`,
`.scratch/36-harness-vocabulary/spec.md`.

**Status (branch `feature/34-vs2-live`, ticket 34).** The mechanism is delivered and verified
hermetically (lanes L1–L4c and L2b, master `48b4882`; hermetic matrix: VS2-EC-1..9 present, VS2-EC-3
a clean negative). The real-model arm has now run (ticket 34, spend authorised): on the declared
primary matrix (12 cells × arms `A0,R,P,U/A`, model `github_copilot/gpt-6-luna`, wire `responses`),
VS2-EC-1/6/7/8/9 PASS, and **VS2-EC-3 FAILS: a budgeted baseline strictly dominates adaptive at
f10-s0 and adaptive dominates no cell** (9/10 complete cells mutually non-dominating) — the §8.4
declared falsifying outcome for the value model `vs2-value/1` as instantiated. VS2-EC-2 fails on one
fail-loud ceiling halt caused by live transport retries charged per physical attempt (declared
instrument working; anatomy in ticket 34); VS2-EC-4/5 failures and their anatomy are recorded in
ticket 34. The §8.2 sensitivity-seed extension is incomplete (upstream gateway throttling; two
declared attempts stopped by the no-progress rule). **Neither the central claim nor the M2 milestone
is accepted:** the declared acceptance rule did not fire — the live evidence falsifies the value
model as instantiated rather than supporting selective allocation. Two gateway wire constraints were
discovered and repaired under supervisor authorisation (ticket 34): the application model rejects
`temperature`/`top_p` on the responses wire, and its reasoning output consumed the declared 128-token
output bound, so the responses codec omits the two parameters and applies `max_output_tokens: 512`
while the declared `FrozenSampling()` stays unchanged and recorded.

**Deliverable.** Equal-budget arms over the same world and seeds: full reference (model on every
decision), periodic, uniform-random, and adaptive-priority.

**Exit criteria.** At equal budget, adaptive beats periodic and random on a **declared** fidelity
metric; the result holds across seeds; it does not depend on a single scalar objective; and identity
and replay remain valid across strategy updates. A clean negative — no allocation policy
differentiates — is an equally decisive and reportable outcome, because it would show the decision
space does not yet reward reasoning.

**Metric definition (frozen).** Fidelity is measured as *divergence of the budgeted run's
civilization-level trajectory from the high-budget reference run* (population, births/deaths, wealth
distribution, exchange volume, and the recorded decision-error rate), not as a single scalar.

**Metric supersession and repair (M2b ticket 37, repaired by M2c part 1 ticket 38, 2026-10-18).** The divergence vector above is retained as a diagnostic, but it is not the objective: at this fixture the reference itself is extinct, so approaching it rewards losing population. The declared objective is `vrd/2`, the priority-ordered viability order below the no-AI control's own realised outcome ([VALIDATION.md](VALIDATION.md); records `.scratch/37-fidelity-metric/spec.md` and `.scratch/38-m2c-preregistration/spec.md`). The frozen EC-3/EC-4 vector dominance rule is unchanged: `vrd/2` scores trajectories against a viability floor, and does not by itself declare one arm better than another. Ticket 37's declared pathology — its birth term outranked its survival term, so a fully-surviving sterile world scored 1.000 against an extinct one-birth world's 0.5963 — is repaired: `vrd/1` is withdrawn, `vrd/2` compares the same three components in the declared priority order (survival, then deaths avoided, then reproduction) instead of summing them, and no weight was changed. M2c may now rank arms by `vrd/2`, but only under the criterion pre-declared in ticket 38 and only after the M2c matrix runs (ticket 39).

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
| **M1** | VS1 | **Implementation complete; hermetic evidence PASS on all eleven frozen exit criteria (EC-1..EC-11); the real-model arm blocked on spend authorisation.** | **A mind in the loop.** The project is no longer an LLM-free simulator: a real model drives part of the world's decisions *and* modifies behaviour logic during the run, its interventions are costed and replayed exactly, and the modification is recorded with lineage and applied or rolled back under audit. |
| **M2** | VS2 | **Mechanism delivered and verified hermetically on master `48b4882` (VS2-EC-1..9 present; VS2-EC-3 a clean negative). The real-model arm ran on branch `feature/34-vs2-live` (ticket 34): on the declared primary live matrix VS2-EC-3 FAILS — a budgeted baseline strictly dominates adaptive at f10-s0 and adaptive dominates no cell — so the declared acceptance rule does not fire and the value model `vs2-value/1` is falsified as instantiated; VS2-EC-2/4/5 live failures and the incomplete sensitivity extension are recorded with their anatomy in ticket 34. Ticket 35 added a pre-registered curve criterion with depth (effort ladder) and redundancy (3 replicates) as first-class axes, and found the dominant defect is admissibility, not allocation: 40-53 % of v1 model answers were refused as inadmissible and `natural-claim/v2` removes that (0 in a 6 580-call run); `vs2-value/2` fixes the coverage collapse (Gini 0.14-0.23 vs 0.49-0.59) yet periodic sampling shows zero demographic effect and the v1-value-model arm is the closest to the R reference, whose anchor is itself unvalidated (Q1). No arm is called better; the central claim and the M2 milestone are not accepted. Tickets 37-42 then ran the stage programme: M2a passed (contract `natural-claim/v4`, 0/40 and 0/1 325 inadmissible answers), M2b passed (fidelity-to-R rejected as an objective; `vrd/2` adopted with a declared birth-dominance pathology and its repair), M2c answered on one cell (`A` separated from `U` at 0.1544 against a 0.0321 band; `A` tied `P` at 0.0083 inside a 0.0420 band, i.e. no detectable effect at 3 replicates rather than no effect), and M2d failed to replicate it (ticket 42: at `f12-s0` both pairs separate in `A`'s favour, at `f10-s1` both tie, and one `inadmissible` answer voided a single cell). The decisive measurement across all three cells is negative: `A` ties the no-AI control `A0` everywhere, `P` ties it on two cells and is worse on the third, `U` is worse on `f12-s0`, and no AI arm beats doing nothing at 248 decisions per arm. The M2c pattern is therefore cell-dependent in both halves, no single-cell result in this stage can carry the claim, and what the stage actually established is the instrument (contract, metric, budgeted matrix at ~$0.57 and 17 minutes per 15 cells) plus the elimination of its own first hypotheses.** | **First evidence on the central claim.** Selective allocation beats — or decisively fails to beat — uniform and periodic spending at equal budget, across seeds, on the declared multi-indicator fidelity metric. |
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
predecessor has passed.

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

- M1 scope is **VS1**; milestones equal slices (M*n* = VS*n*), six of each. VS2 follows VS1.
- VS1 carries **both** AI roles: Role A (an actor's mind at a declared decision seam) and Role B
  (in-run behaviour-logic modification); Role B must kick in *during* the run.
- The fidelity metric is the frozen definition given in VS2.
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
