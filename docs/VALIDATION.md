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
