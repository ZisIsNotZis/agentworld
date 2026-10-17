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
| **M2** | VS2 | **Mechanism delivered and verified hermetically on master `48b4882` (VS2-EC-1..9 present; VS2-EC-3 a clean negative). The real-model arm ran on branch `feature/34-vs2-live` (ticket 34): on the declared primary live matrix VS2-EC-3 FAILS — a budgeted baseline strictly dominates adaptive at f10-s0 and adaptive dominates no cell — so the declared acceptance rule does not fire and the value model `vs2-value/1` is falsified as instantiated; VS2-EC-2/4/5 live failures and the incomplete sensitivity extension are recorded with their anatomy in ticket 34. The central claim and the M2 milestone are not accepted.** | **First evidence on the central claim.** Selective allocation beats — or decisively fails to beat — uniform and periodic spending at equal budget, across seeds, on the declared multi-indicator fidelity metric. |
| **M3** | VS3 | Not started | **Self-improving.** Strategies measurably improve over generations under audited mutation, with provenance and rollback for every patch. |
| **M4** | VS4 | Not started | **Stakes and scale.** The central-claim result survives heterogeneity and scale; the marginal value of reasoning is measured, not assumed. |
| **M5** | VS5 | Not started | **Society.** Institutions and organizations arise and persist only through inspectable incentives and enforcement. |
| **M6** | VS6 | Not started | **History.** Model-conditional calibration, conditional branches, sensitivity rather than one timeline. |

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
