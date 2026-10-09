# Vertical-slice roadmap (draft — pending operator confirmation)

## Why this document exists

An independent adversarial audit (`.scratch/audit-vision-fidelity.md`, 2026-09-29) confirmed the
operator's concern: after twenty tickets the repository contains **no language-model call path at
all**, no cognitive-value scheduler, no intervention ledger, no allocation-policy comparison, and no
strategy mutation or rollout, while `docs/VISION.md`'s central claim — that *selective* spending of
expensive reasoning preserves civilization-level fidelity better than *uniform* or *periodic*
invocation at the same budget — has never been tested. Roughly 89% of post-foundation effort went to a
16-actor synthetic economy, and the last five tickets measured increasingly fine properties of
constants the project had frozen itself.

The cause is not laziness but **horizontal development**: concepts were added one layer at a time
world → economy → ecology → exchange, with the cognitive layer deferred to a future phase that no
ticket ever reached. This roadmap replaces that with vertical slices.

## The rule

> **Every released version is a complete, runnable civilization simulator. Every concept exists and
> runs end-to-end in every version. Slices raise the *fidelity and capability ceiling* of the hard
> concepts; they never introduce a concept that was absent before.**

A slice is accepted only when a single documented command produces a running world in which every
concept in the inventory below is demonstrably exercised, with recorded evidence and a falsifiable
exit criterion. Where verification requires subjective judgement, the uncertainty is stated.

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

**Deliverable.** Equal-budget arms over the same world and seeds: full reference (model on every
decision), periodic, uniform-random, and adaptive-priority.

**Exit criteria.** At equal budget, adaptive beats periodic and random on a **declared** fidelity
metric; the result holds across seeds; it does not depend on a single scalar objective; and identity
and replay remain valid across strategy updates. A clean negative — no allocation policy
differentiates — is an equally decisive and reportable outcome, because it would show the decision
space does not yet reward reasoning.

**Metric definition (needs operator confirmation).** Fidelity is measured as *divergence of the
budgeted run's civilization-level trajectory from the high-budget reference run* (population,
births/deaths, wealth distribution, exchange volume, and the recorded decision-error rate), not as a
single scalar.

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

## Milestones

- **M1 — "A mind in the loop" = VS1 accepted.** The project is no longer an LLM-free simulator: a real
  model drives part of the world's decisions, its interventions are costed and replayed exactly, and
  it can rewrite its own strategy under audit.
- **M2 — "First evidence on the central claim" = VS2 accepted.** Selective allocation beats (or
  decisively fails to beat) uniform and periodic spending at equal budget, across seeds.
- **M3 — "Self-improving" = VS3 accepted.** Strategies measurably improve over generations under
  audited mutation.

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

## What stops, and what stays

**Stops:** new ecology mechanisms and further phase-boundary measurement. The ecology questions that
were answered remain answered; no new horizontal rung is opened.

**Stays:** the measured substrate — deterministic kernel with atomic commits and exact replay,
conservation-checked ledgers, byte-pinned versioned strategies, checkpoint branching, ceiling
instrumentation, and above all the **ticket 18/19 artifacts**: the measured map of where decisions
matter (f=10/11 reliable, f=12 fragile, f=16 arithmetic null) and the datum that decision quality
changes outcomes exactly there (natural/7 2/8 vs natural/7b 7/8 at f=12). Those are the substrate the
VS2 experiment stands on.

**Also required (audit corrective action 3), before or inside VS1:** docs reconciliation — mark the
old phase plan as historical, record the honest verdict table, retire ticket 13's "productive forces
emerged" overstatement, and correct `README.md`'s present-tense claim that language models "wake
selectively" (no such mechanism exists yet).

## Ticket sketch for VS1 (detailed by the worker; product decisions fixed here)

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

## Open decisions for the operator

1. **Scope of M1:** VS1 alone (a running model-in-the-loop world), or VS1+VS2 (through first evidence
   on the central claim)?
2. **Fidelity metric:** accept "divergence from the high-budget reference run, across several
   civilization-level indicators, not one scalar"?
3. **Both AI roles in VS1?** (Recommended yes — the operator's ideal names both intervention and
   self-evolution; a VS1 with Role A only would not satisfy "all concepts present".)
4. **Land ticket 20** (complete, reviewed, gate re-running) as the closing of the ecology arc, or
   leave the branch unmerged?
