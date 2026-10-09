# Roadmap

> **Status: historical background.** The D0 and P0–P4 phase plan below is the pre-pivot plan. The
> active plan is [VERTICAL-SLICES.md](VERTICAL-SLICES.md), which organises work into vertical slices.
> In the active plan **language-model strategy editing and cognition are in its FIRST slice (VS1)**;
> they are no longer deferred to P1. The P0/P1 exclusion of language-model strategy editing is
> superseded. Current per-version status and the measured decision-sensitivity map are recorded under
> P0 below.

Each phase exists to test a risky assumption. Population targets may change after measurement.

## D0 — Foundation documentation

Deliver the vision, principles, architecture boundaries, validation method, glossary, open questions, and first architectural decisions.

Exit when a fresh reader can state the objective, non-goals, v0 scope, hard constraints, and unresolved hypotheses without relying on prior conversation.

## P0 — Explicit-fiber survival world (historical)

The executable design contract is [P0-SPEC.md](P0-SPEC.md).

Target: approximately 1,000 explicit people over several simulated decades on one machine.

Include:

- event-driven world time;
- persistent actor fibers;
- compact in-memory hot state;
- bounded worker execution;
- resources, energy, food, weather, birth, death, kinship, movement, and conflict;
- a minimal typed strategy runtime with bounded search;
- append-only events, checkpoints, branches, replay, and numerical projections.

Exclude:

- language-model strategy editing (**superseded** — cognition and strategy editing are in the first
  slice of the active [vertical-slice plan](VERTICAL-SLICES.md));
- general institution emergence;
- detailed physical simulation;
- global history;
- distributed execution.

Exit criteria:

- deterministic covered events replay to the same state;
- invariants hold over long seeded runs;
- state and event storage are measured per actor-year;
- CPU and memory profiles identify actual bottlenecks;
- distinct strategies produce measurable and causally explainable population outcomes.

### Current status (pre-pivot substrate)

Every comparison-relevant result is model-conditional. The honest per-version verdicts, recorded
verbatim in [VALIDATION.md](VALIDATION.md) and the per-ticket evidence:

- **v3** (productive capacity) — SUPPORTED on its frozen criteria, with the corrected reading that
  actors exploit the decreed capital yield rule (one food unit per capital point per pulse); a decreed
  yield rule was exploited, **nothing "emerged"**.
- **v4** (reproduction/genetics) — SUPPORTED: both frozen verdicts (births create endogenous
  population dynamics; heritable variation affects population outcomes).
- **v5** (natural limits) — FALSIFIED: the frozen A-band (the organic ecology is a sterile aging-only
  die-off).
- **v6** (surplus behaviour) — FALSIFIED.
- **v7** (provision mechanism) — FAILED.
- **v8** (founding-band density curve) — FAILED under its frozen all-four-cells quantifier (the
  surplus indicator flips in only 2 of 4 gating natural/7 cells at f=12).
- **v9** (wage-contract exchange) — FAILED on one falsified non-distortion clause (settlements on the
  saturated patch), while trade itself occurs robustly with exact conservation.

Measured decision-sensitivity map: **f=10/11 reliable, f=12 fragile, f=16 arithmetic null**. Decision
quality changes outcomes exactly there — at f=12 the provision policy natural/7 reaches the surplus
indicator in 2/8 runs against 7/8 for the banking-only natural/7b ablation. This substrate is what
VS1/VS2 build on.

## P1 — Budgeted cognition (historical)

This phase was where the pre-pivot plan deferred model reasoning. In the active plan cognition is in
the first slice (VS1) and the allocation experiment is VS2; see
[VERTICAL-SLICES.md](VERTICAL-SLICES.md).

Target: approximately 10,000 explicit people with selective model intervention.

Include:

- cognitive scheduler and rate limits;
- actor escalation requests;
- protected random exploration;
- isolated batched management;
- layered trajectory summaries and evidence queries;
- versioned strategy patches;
- sandbox, counterfactual test, staged rollout, and rollback;
- beam-style comparison of allocation policies.

Exit criteria:

- no detected cross-case private-information leakage in the test suite;
- every model intervention records cause, cost, patch, and measured result;
- at equal budget, at least one adaptive scheduler outperforms periodic and random baselines on declared metrics;
- performance holds across seeds and does not depend on one scalar objective;
- identity and replay remain valid across strategy updates.

## P2 — Social scale and institutions (historical)

Target: approximately 100,000 explicit people across multiple regions.

Include:

- migration and trade networks;
- culture and knowledge diffusion;
- organizations and institutional primitives;
- institution macro compilation and decompilation audits;
- technology composition;
- multiple interacting scales of analysis;
- checkpoint-based policy and institution branches.

Exit criteria:

- explicit-fiber performance is measured against a declared machine budget;
- macro aggregates reconcile with individual events;
- institutions arise and persist only through inspectable incentives and enforcement;
- rare innovations are not eliminated by scheduling or batching;
- model-conditional policy reports expose assumptions and sensitivity.

## P3 — Historical mechanism calibration

Select bounded regions and periods with usable evidence. Calibrate without requiring exact historical replay.

Exit criteria:

- target mechanisms reproduce plausible distributions across held-out seeds or periods;
- future historical knowledge is absent from actor inputs;
- divergences can be classified as valid alternatives, missing mechanisms, or implementation defects;
- uncertainty is reported rather than hidden by narrative explanation.

## P4 — Scale decision

Use P0–P3 evidence to choose among:

- continuing with explicit fibers;
- introducing inactive-state compression;
- aggregating selected populations;
- partitioning across processes or machines;
- changing runtime language or storage architecture.

No scaling technique is adopted merely because eventual billions are imaginable. The decision is based on measured cost, lost fidelity, and expected research value.
