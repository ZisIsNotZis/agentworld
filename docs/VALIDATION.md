# Validation strategy

## Validation claim

The project validates explicit mechanisms and budget-allocation methods. It does not validate a run because the resulting story sounds plausible.

## Quantitative state contract

Every comparison-relevant semantic property must project into numerical features. Each feature definition includes:

- name and stable identifier;
- source fields or evidence;
- transformation and aggregation;
- unit, bounds, missing-value behavior, and uncertainty;
- version and calibration dataset;
- failure modes and information loss.

Raw text may support an interpretation, but a comparison must cite the metric version and input evidence.

## Metric families

Initial metric families should cover:

- population, mortality, morbidity, and age structure;
- energy, food, material stocks, and production;
- wealth, coercive power, and their distributions;
- network connectivity, dependence, trust, and conflict;
- knowledge, skill, technology, and diffusion;
- institutions, compliance, enforcement cost, and stability;
- actor goal progress, strategy failure, and behavioral diversity;
- uncertainty, causal reach, and computation consumed.

Metrics form a vector. Scalar aggregation is allowed only when weights and sensitivity are visible. Branch selection also preserves diversity so one optimized score does not collapse the simulated world into a narrow artifact.

## Invariants

Automated checks should cover:

- conservation of modeled resources;
- valid births, deaths, identities, and relationships;
- ownership, debt, and transaction balance;
- authorized causes for state changes;
- legal information provenance for beliefs and actions;
- deterministic replay for covered operations;
- no mutation from rejected intentions;
- valid strategy and rule versions at every event;
- checkpoint and event-log consistency.

## Bounded renewable-food pilot evidence

The synthetic food-flow v1 reporter (for example, `go run ./cmd/foodflow -q=8 -seed=0 -workers=1 -hours=168`; also test q=3/0, seeds 0/7 and workers 1/4) checks hourly produced = consumed + held + stock and initial energy + consumed = remaining energy + basal spent + cap loss. It samples fixed early/middle/late hours, population-change transitions, 1/8/16 actor states, linked accepted-event counts and unlinked rejection counts, including rejected Gather action totals as well as distinct denial reasons; stopped actors' hunger must remain fixed; immutable checkpoint and accepted history/journal byte sizes are measured separately. Frozen gates distinguish abundant viability, scarce equal first-eight-hour access with subsequent decline, and zero-flow extinction. See [the scoped run record](../.scratch/10-food-flow-pilot/spec.md) for commands and values. This checks internal consistency and conditional fixture behavior only: selected 16-actor seeds, no social dynamics, no human calibration, no scale extrapolation. Peak Go heap is sampled after scheduler steps rather than continuously; Linux `ru_maxrss` is the process high-water RSS.

## Bounded social-food v2 pilot evidence

The social-food v2 reporter (`go run ./cmd/socialfood -q=3 -seed=0 -workers=1 -hours=24 -out=<new file>`; also the predeclared q=8 hours-24 and q=0 hours-12 prefixes, seeds 0/7, workers 1/4) forks one verified neutral h0 checkpoint into recorded enable/disable children and reports both. Automated checks cover hourly conservation identities on both branches, exact journal-versus-witnessed dyad counter agreement, ledger given/received equal to the gift count, zero social evidence on the disabled branch, restored accepted-history replay with per-attempt event-link verification, and immutable no-clobber publication of every bundle and report. Distributed metrics are the directed dyad assistance fraction `(gifts+1)/(requests+2)` (a numerical witness ratio, not moral trust) and per-patch consumed Gini with an explicit zero-total definition. Reports are labeled `synthetic/model-conditional`; workers 1 and 4 must agree on all reported evidence fields. See [the scoped run record](../.scratch/12-social-food-transfer/spec.md) for commands, branch comparisons and donor-cost observations.

## Productive-capacity v3 pilot evidence

The capacity v3 reporter (`go run ./cmd/capacity -q=8 -seed=0 -workers=1 -hours=168 -branch=enabled -out=<new file>`; also the predeclared q∈{0,3,5,8} × seeds 0/7 × workers 1/4 × enabled/disabled cells with q0 at h12 and the rest at h168, plus founder-B enabled cells and one full-cost q8 run) forks one verified neutral h0 checkpoint into recorded enable/disable children (founder-B, actor 1 granary 4, enabled-only) and reports each. Automated checks cover the frozen G1–G3 conservation identities re-verified independently at every hour on every branch, restored-history head agreement, digest equality between each published branch bundle and its restored child, typed journal classification (a gather is admitted iff its typed rejection label is admitted; denials carry typed labels only, and an unknown label is a hard failure), keyed-attempt event-link verification against the restored accepted history, zero capital evidence on every disabled branch and at q0 with extinction by h11, and immutable no-clobber publication of every bundle and report with stray positional arguments rejected. Distributed metrics are per-patch wild-consumption Gini and capital-held Gini, each with an explicit zero-total-equals-zero-inequality definition. The predeclared `surplus-flow-48h` sums hourly capital-yield-minus-stored-meal ledger deltas over hours 120–168, clamped to the available hours below h168. Workers 1 and 4 produce identical evidence fields; only the worker count and the checkpoint directory differ. Recorded band outcomes: q8-enabled 16 alive with all 16 actors self-sustaining at k=3 and +1,520 surplus flow; q5-enabled 16 alive, 10 self-sustaining, +656; q3-enabled 14 alive, 6 self-sustaining, +178; q0 extinct at h11 with zero capital events on both branches; disabled baselines exactly 6/10/16 alive with zero capital; founder-B changes no survival or capital outcome in any cell. See [the scoped run record](../.scratch/13-productive-capacity/spec.md) for all verbatim values, the falsification verdict and cost measurements. Reports are labeled `synthetic/model-conditional`: no historical calibration, no takeoff guarantee, and no P0 scale extrapolation.

## Cognitive-allocation experiments

Given the same initial checkpoint, random streams, compute budget, and model access, compare allocation policies such as:

- fixed periodic waking;
- random sampling;
- structural-importance heuristics;
- anomaly-triggered waking;
- expected-value scheduling;
- mixtures with protected exploration.

Measure:

- model calls, tokens, CPU time, memory, and wall time;
- detected strategy failures;
- information-boundary violations;
- actor plausibility audits;
- macro-state divergence from a higher-budget reference;
- rare but consequential events captured or missed;
- realized value and calibration error of predicted priority.

## Beam-style development runs

At selected checkpoints, run several engine or allocation strategies in parallel. Periodically evaluate their metric vectors, causal validity, and diversity.

A beam controller may prune branches only under a declared rule. Preserve branch metadata and representative rejected branches so selection bias remains inspectable.

Use a higher-budget reference only as a comparative approximation, never as unquestioned ground truth.

## Human and social plausibility

Evaluate whether behavior follows available information, motives, ability, relationships, incentives, and institutions. Test cooperation and exploitation under scarcity, anonymity, unequal power, enforcement, public-goods dilemmas, rumor, and external threat.

A peaceful or unfamiliar society is not automatically invalid. It is valid only if the mechanisms and maintenance costs that sustain it are inspectable.

## Historical calibration

Compare distributions and mechanisms rather than exact named events. Candidate checks include carrying capacity, mortality, settlement growth, disease transmission, trade, inequality, state capacity, conflict, knowledge accumulation, and technology adoption.

Hold out periods or regions where practical. Prevent actor policies from receiving future historical outcomes.

## Model diagnostics

Use:

- ablation of supposedly important mechanisms;
- parameter sensitivity and uncertainty analysis;
- independent implementations of critical rules;
- adversarial seeds and red-team societies;
- random audits of low-priority actors;
- macro-to-micro reconciliation;
- causal backtracking from late anomalies to earliest divergence.

## Acceptance discipline

Every reported result states:

- code, rule, strategy, metric, and seed versions;
- branch ancestry and checkpoint compatibility;
- budget and model configuration;
- observed result distribution;
- sensitivity and known limitations;
- whether the claim is internal consistency, historical calibration, or model-conditional policy evidence.
