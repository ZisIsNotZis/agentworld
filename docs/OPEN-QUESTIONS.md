# Open questions

These questions are deliberate research work, not silently settled assumptions.

Phase references name the active plan's slices (VS1–VS6, [VERTICAL-SLICES.md](VERTICAL-SLICES.md)); the pre-pivot P0–P4 phase plan ([ROADMAP.md](ROADMAP.md)) is historical.

## Q1 — Civilization-level distance

**Question:** Which metric vector distinguishes materially different worlds without imposing one preferred civilization?

**Current answer (fixture f10-s0, H=144; M2b ticket 37, repaired by M2c part 1 ticket 38, 2026-10-18):** a three-component demographic viability order, `vrd/2` — surviving actor-hours, cumulative births and cumulative starvation deaths, read below the no-AI control's own realised outcome in the declared priority order (population survived, then deaths avoided, then stability/reproduction). It distinguishes the recorded arms in the declared direction (an extinct, zero-birth arm ranks last with a categorical persistence separation; a high-churn arm ranks above it but below the control) and it imposes no preferred civilization beyond persist, reproduce and avoid starvation deaths, a floor the world already reaches without cognition. The R-anchored divergence vector is rejected as an objective at this fixture: the reference is extinct (0-1 births, 0 alive at the terminal sample), so it rewards losing population. Metric contract: [VALIDATION.md](VALIDATION.md); evidence and records: [`.scratch/37-fidelity-metric/spec.md`](../.scratch/37-fidelity-metric/spec.md) and [`.scratch/38-m2c-preregistration/spec.md`](../.scratch/38-m2c-preregistration/spec.md).

**Cross-fixture state (tickets 41–42):** `vrd/2` now has three scored cells — `f10-s0`, `f12-s0` and `f10-s1` — scored against the **committed `f10-s0` anchor**, with each cell's own control recorded beside it as a supplementary reading that moved no verdict ([VALIDATION.md](VALIDATION.md)). It separates arms in both directions and ties plausibly, so its cross-fixture behaviour is measured rather than assumed; what it does not do is produce a stable ordering of the *AI* arms, which is a property of those arms and not of the metric (see Q2 and Q13). At `f12-s0` every arm including the control loses population to age deaths (13 of 16 deaths), so the tier-3 statistic there measures damage avoidance against arms that harm the control's own outcome. **At all three cells the control itself ends at one alive** (`f10-s0` 1 of 10, `f12-s0` 1 of 12, `f10-s1` 1 of 10), so the anchor is a floor on a collapsing trajectory and not on a viable world — rung 2 of the claim ladder did not hold on that world. **Update 2026-10-27:** the world was rebuilt under `W1` (mechanisms v8–v14, ticket 55); the declared default cell now ends at 11 of 10 alive with 12 births, so the claim can be retested on a viable anchor. The three scored cells above remain the historical record of the pre-v14 world.

**Still open:** cross-horizon validity, a fixture whose control is itself not viable (it needs a declared substitute floor), the power of a 3-replicate design (ticket 41 measured a minimum detectable difference of 42–61 actor-hours at the observed bands, against a between-arm difference of 12), the identity/diversity of survivors and age-death timing are not represented, and the repair traded away the ability to express any tradeoff between its tiers (a difference on a tier outranks any difference on a lower tier, by construction — so one actor-hour of survival outranks any reproduction difference **and any number of starvation deaths**, i.e. "deaths avoided", the second declared priority, can never compensate a survival deficit, and that tradeoff is declared as a witness world rather than left implicit; the stability tier is a reproduction proxy, not a churn measure). The rankings are declared, not calibrated: `vrd/2` has no weights at all, so its sensitivity check is the six tier permutations plus the terminal-sample convention, not a weight sweep.

**Experiment:** Compare expert judgments, causal interventions, and metric rankings across controlled branches; the first such comparison on recorded branches is ticket 37's, and the next is the M2c matrix, whose criterion is pre-declared in ticket 38 §7 and whose live run is ticket 39 (arms `A0/P/U/A/A1`, cell f10-s0, H=144, 3 replicates, contract `natural-claim/v4`).

**Decision needed:** none for M2c to run — the metric is declared, the criterion is pre-declared and the anchor is reproducible at zero spend. Beam pruning still needs the cross-fixture part of this question and a declared aggregation of `vrd/2`'s tuple.

## Q2 — Cognitive intervention value

**Question:** How can the scheduler predict that expensive reasoning will improve civilization-level fidelity before downstream effects occur?

**Current hypothesis:** Combine impact, uncertainty, decision sensitivity, novelty, propagation, structural position, and cost, then calibrate against realized branch differences.

**Experiment:** Evaluate competing scheduling policies from identical checkpoints and budgets.

**Measured state (tickets 41–42, contract `natural-claim/v4`, `vrd/2`, H=144, 3 replicates, 248 decisions per budgeted arm):** on three cells no allocation arm beat the no-AI control `A0` — the adaptive arm `A` ties `A0` on every cell, the periodic arm `P` ties it on two and is worse on `f12-s0`, and the uniform arm `U` is worse on `f12-s0`. The only separations found are between AI arms: `A` beats `U` at `f10-s0` and `f12-s0` and does not at `f10-s1`; `A` versus `P` ties at `f10-s0` and `f10-s1` and separates, thinly, at `f12-s0`. So the scheduler's value is **unmeasured at these cells**, not merely uncalibrated: there is no observed improvement over spending nothing to calibrate against. Records: [`.scratch/41-m2c-matrix-rerun/spec.md`](../.scratch/41-m2c-matrix-rerun/spec.md), [`.scratch/42-m2d-replication/spec.md`](../.scratch/42-m2d-replication/spec.md).

**Decision needed:** Q13 asks whether this world contains a reasoning premium at all; until that is answered, no allocation mechanism can be shown to have value here. A provisional version for VS2 was assumed rather than measured; permanent resolution is not expected.

## Q3 — Human plausibility

**Question:** Which stable human capacities and tendencies must exist, and which behaviors should emerge from culture and institutions?

**Current hypothesis:** Model plural motives and bounded cognition; evaluate behavior conditionally rather than enforcing a fixed historical outcome.

**Experiment:** Adversarial social scenarios, historical mechanism calibration, and blinded trajectory review.

**Decision needed:** Minimal actor state for VS1 strategy design.

## Q4 — Strategy representation

**Question:** Should v0 use behavior trees, utility rules, bounded planners, bytecode, or a hybrid?

**Current hypothesis:** A typed graph or bytecode with utilities and bounded search can remain lightweight, mutable, and auditable.

**Experiment:** Prototype common survival behaviors and measure expressiveness, execution cost, mutation validity, and explainability.

**Decision needed:** Before VS1 implementation.

## Q5 — Identity across fidelity changes

**Question:** What state must remain explicit so a previously routine actor can become high-fidelity without inventing a new personality?

**Current hypothesis:** Preserve dispositions, goals, beliefs, relationships, skills, body state, commitments, salient memories, and strategy ancestry.

**Experiment:** Hide long low-detail periods, reconstruct high-impact decisions, and compare against continuously detailed reference actors.

**Decision needed:** Actor schema for VS1; refined before any population compression.

## Q6 — Batched model isolation

**Question:** Can unrelated private cases safely and economically share a model call?

**Current hypothesis:** Namespaced schemas, evidence handles, shuffled controls, and output validation reduce contamination.

**Experiment:** Compare single-case and multiple batch sizes for leakage, omission, order effects, quality, and actual cost.

**Decision needed:** During VS2.

## Q7 — Model-refinement authority

**Question:** When may adjudication add a primitive, state variable, approximation, or institution macro?

**Current hypothesis:** Separate feasibility, expressiveness, complexity, and causal-compatibility decisions; require sandbox evidence and versioned rollout.

**Experiment:** Attempt representative technology and institution proposals against an intentionally incomplete model.

**Decision needed:** Minimal primitive registry before VS1; extension protocol before VS3.

## Q8 — Abstract physical fidelity

**Question:** What physical abstraction is sufficient for technological feasibility without simulating microscopic physics?

**Current hypothesis:** Model materials, energy, tolerances, processes, skills, tools, and testable outputs at progressively refined levels.

**Experiment:** Reproduce known simple technology prerequisites, failure rates, and independent rediscovery without naming the technology to actors.

**Decision needed:** Basic survival processes are already implemented in the substrate; advanced production later.

## Q9 — Institution emergence

**Question:** When does repeated interaction become a norm, organization, authority, or compiled institution?

**Current hypothesis:** Use measurable persistence, membership, shared state, enforcement, expectations, and coordination effects.

**Experiment:** Compare explicit individual execution with compiled macros and test behavioral equivalence.

**Decision needed:** During VS5.

## Q10 — Explicit fiber scaling limit

**Question:** How many persistent actor fibers can one machine support at useful event rates and history retention?

**Current hypothesis:** Data-oriented dormant state plus bounded workers supports the prototype range; permanent goroutines and per-action database transactions do not.

**Experiment:** Benchmark memory per actor, events per second, cache behavior, checkpoint throughput, and log volume from VS1 onward.

**Decision needed:** Before VS4 scale-up, based on measurements.

## Q11 — Checkpoint causal compatibility

**Question:** Which code and model changes permit continuation without invalidating experimental conclusions?

**Current hypothesis:** Classify changes as compatible, migrated, causally tainted, or invalid and retain branch ancestry.

**Experiment:** Create representative schema, scheduling, and rule changes and verify replay and divergence diagnostics.

**Decision needed:** Before the VS1 checkpoint format is frozen.

## Q12 — Scientific and policy claims

**Question:** What evidence is sufficient to report a historical mechanism or policy result responsibly?

**Current hypothesis:** Require multi-seed distributions, assumptions, sensitivity, calibration status, failure modes, and missing real-world evidence.

**Experiment:** Produce trial reports from controlled branches and audit whether readers can distinguish model-conditional evidence from real-world proof.

**Decision needed:** Before public experimental results.

## Q13 — Reasoning premium in this world

**Question:** Does this world contain any reasoning premium — can a short-sighted policy be measurably worse than a policy that reasons about consequences — or is the decision space shallow enough that any competent policy reaches the control's outcome?

**From:** Q1's cross-fixture result and Q2's measured state; M2c and M2d, tickets 41–42. The observation that raises it is that the adaptive arm ties the no-AI control on all three cells, and the two baselines it does not tie are themselves no better than the control. If no policy in this world can do better than the control, then selective spending of reasoning has nothing to win, and the central claim is untestable here rather than false.

**Author:** agent (inference from the recorded matrices, ticket 42 review).

**Status:** answered for the declared world; open only for richer worlds. **Update (2026-10-28):** W3/W4 ran the four axes on the viable W1 world and the answer is a detected null: no effort, timing or model condition separates from the reference at this design; the model nearly duplicates the frozen policy (94.7–99.9 % agreement), and the timing null reproduces at `f11-s2`. The central claim is not supported (tickets 58–62). **Update (2026-10-27):** `W1_world_runs` passed on the declared cell after the world rebuild (mechanism v14 + `natural/14`, `Yield = 1`, `BirthCost = 2`, `f10-s0`: alive 11 of 10, births 12, starvation 1, age 10; ticket 55, [`.scratch/55-w1-criterion/spec.md`](../.scratch/55-w1-criterion/spec.md)). The 0/128 result below is the historical measurement on the pre-v14 world and is superseded as a statement about the current world. Q13's own question — does this world contain a reasoning premium — was answered by `W3_axes`/`W4_curve_match` (tickets 58–62); no route in the table below starts. The earlier route-0 account follows as history: route 0 (rung 2, zero spend) was **measured**: no fixture in the declared seed sets gave the pre-v14 no-AI control a good ending at `H = 144`. Ticket 43 swept every admissible founder count `f = 1..16` × seeds `0..7` at `H = 144`, `-arms A0` only, **128 cells, 0 passing**: `alive_end < N0` in 127/128, starvation > 0 in 80/128, zero births in 34/128; the only cell reaching `alive_end >= N0` (`f6-s6`) fails on 9 starvation deaths. The binding mechanisms were founder aging first (81–100 % of primary-cell deaths; births from h5 and as late as h68 at `f11-s1`, h124 across the grid at `f6-s6`; age deaths from h99–h103 to h138–h144, the last at `f10-s2` h138 and `f10-s0`/`f11-s0` h144), then birth economics (`births/N0 = 0.20–0.42` at the primary cells; zero births at `f = 1, 8, 9, 16`), then food supply (21.7 % of grid deaths, 0 at `f = 10`); founder count was not a lever. The three recorded `A0` cells reproduce exactly. Ticket 44 then falsified the first reading of the cause: the fertility gate was ample under other frozen policies (two actors hold it 107 of 144 hours under `natural/5`), so the rarity under the base policy was its own provisioning rule, and the remaining cap sat at the decision instants, not in the economy. Ticket 45 measured that instant and found the cap in the **policy's addressee rule**: on `f10-s0`, 133 of 135 `consent-own` refusals were hours where exactly one other co-located non-kin actor held the full gate while the script asked a mate with `granary 0, energy 2` ([`.scratch/45-decision-instant-census/spec.md`](../.scratch/45-decision-instant-census/spec.md)). Tickets 46–54 then changed the policy, placement, yield, meal, birth price and commons rule until the world passed W1. Record and durable evidence: [`.scratch/43-rung2-seed/spec.md`](../.scratch/43-rung2-seed/spec.md), per-cell table `.scratch/43-rung2-seed/a0-grid.tsv`, 128-row aggregate `.scratch/43-rung2-seed/a0-grid.json`, runner text `runner-report.txt`.

**Evidence:** `vrd/2` scores for `A0`, `P`, `U`, `A`, `A1` on `f10-s0`, `f12-s0` and `f10-s1`, records in `.scratch/41-m2c-matrix-rerun/spec.md` and `.scratch/42-m2d-replication/spec.md` (durable per-cell extracts: `.scratch/42-m2d-replication/{f12-s0,f10-s1}-live-extract.json`).

**Scope:** the current substrate only — 4 admissible actions per actor, one food loop, H=144, `natural-claim/v4` — at 3 replicates per arm. Nothing here bounds a richer substrate.

**Where this sits in the ladder** ([VERTICAL-SLICES.md](VERTICAL-SLICES.md), VS2 rungs). Q13 is rungs 2 and 3, and the ladder's order makes rung 2 the blocker: with no cognition at all the world ends at one survivor, so there is no good ending for reasoning to protect and no measured premium for an allocator to spend. The recorded programme ran rung 1, then rungs 5 and 4, without rung 2 and with only a void rung 3 — which is why its nulls are uninterpretable rather than negative.

**Routes, with cost and what each would answer:**

| route | what it is | cost | what it answers |
|---|---|---|---|
| **0 (next, rung 2)** | search the declared fixture grid (founders × seed) for a no-AI trajectory that ends at or above its founding cohort with zero starvation deaths and at least one birth; if the frozen world contains none, report the world parameters that bind — birth rate, aging, food supply — rather than picking a fixture by eye | **$0, ≤ 1 h** | whether this world can express a premium at all, and it produces the fixture every later rung needs |
| **A (rung 3)** | compute a scripted policy and a myopic-but-locally-optimal policy offline from the recorded ledgers and score them under `vrd/2` against `A0`, `P`, `U`, `A` on a rung-2 fixture; then the effort ladder on the reference model, then a second model at matched effort | **$0 offline, then ≤ $1.10 per matrix of live calls** | whether reasoning has headroom here: if a simple script already matches `A0`, no allocation scheme can show value; if it loses to `A`, the premium is real and the next step is strengthening the policy |
| **B** | redesign the world so a short-sighted policy visibly loses — more action dimensions, causal delay between action and consequence, resource competition. This is what rung 2 needs if route 0 finds no good ending in the frozen world | **≈ $3–5, 1–2 days** | whether the central claim can be true in a world that can express it; invalidates every existing metric baseline and every M2 arm result |
| **C** | re-scope the claim to "248 model calls do not harm the civilization" | **$0** | nothing new: the three cells already support it, and spending fewer decisions obtains it for free, so the claim is nearly information-free |

**The conflict route B creates:** the product decision that the ecology arc is closed (VERTICAL-SLICES.md) is the operator's, and rung 2 cannot pass without either a world change or a relaxed criterion, so choosing B means suspending that decision. No agent may suspend it.

**Why 0 then A:** route 0 is free and decides whether B is necessary; route A is free in its offline half and decides whether C is self-deception. Both are read on a fixture whose control is a good ending, which is the condition the recorded programme lacked.

**Superseded framing (2026-10-26).** The operator replaced the route choice with an ordered work plan: fix the world first (`W1_world_runs`), then make `luna:max` always-on the reference (`W2_reference_luna_max`), then measure model/effort/timing/stability (`W3_axes`), then judge by per-hour KPI curves against the reference (`W4_curve_match`). Q13's question is answered by W3, not by picking a route; routes B and C remain available only if W1 shows the frozen world cannot run at all, and neither starts without the operator's word.
