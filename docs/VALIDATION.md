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

## Reproduction-and-genetics v4 pilot evidence

The reproduction v4 reporter (`go run ./cmd/reproduction -q=8 -seed=0 -workers=1 -hours=168 -births=both -out=<new file>`; also the predeclared q∈{0,3,5,8} × seeds 0/7 × workers 1/4 births{enabled,disabled} cells with q0 at h12 and the rest at h168, the recorded genome-variant enabled cells, full-cost runs, and 3 v3-equality comparisons) forks one verified births-disabled neutral h0 checkpoint into recorded intervention children (the v4 h0 pulse is births-asymmetric by frozen design, so a disabled root is the only branch parent and the disabling direction is refused) and reports each. Automated checks cover the extended frozen conservation identities re-verified independently at every hour on every branch, restored-history head agreement, digest equality between each published branch bundle and its restored child, typed journal classification (including birth requests/replies, typed refusal reasons, and the allocation-collision lost-birth evidence), keyed-attempt event-link verification against the restored accepted history, zero birth evidence on every disabled branch and at q0 with extinction by h11 on both branches, and immutable no-clobber publication of every bundle and report with stray positional arguments rejected. Distributed metrics are the integer-exact mean pairwise genome L1 with per-locus allele frequencies at the frozen hours, the derived-kinship density (related living-pair fraction, mean finite walk distance, per-actor living children), and the birth-economics record (filings, typed refusal histogram, privileged claim-versus-truth divergences, lost-birth collisions). Workers 1 and 4 produce identical evidence fields; only the worker count and the checkpoint directory differ. Recorded band outcomes: q8-enabled 16 births and peak 20 alive (first birth h5, founder die-off complete by h134/h141, final 0 — growth then collapse under the birth cap plus aging); q8-disabled zero birth events with the age die-off complete by h141/h143; q5-enabled 16 births, peak 20/21, final 0; q3-enabled 11 births, peak 18, final 0 (inside the predeclared uncertain band); q0 extinct at h11 with zero birth attempts on both branches. All three equality fixtures match the recorded ticket-13 evidence (v4-disabled ≡ v3 through h95 at q3/q8 and full-state at q0 h12; the recorded v3 baseline is the full capacity pilot, whose capital machinery v4 always inherits — a first q3 comparison against the capital-disabled v3 variant diverged from h17 and is recorded verbatim as a baseline-selection finding, not a tuning). The genome-variant fixture drives the locus-m allele-7 frequency among the living from the founding 0.50 to 1.00 by h72 (max shift +0.50 ≥ the predeclared 0.1) with allele-5 carriers extinct while 7 persists, and commits 2 second-generation births per variant cell against 0 in the entire neutral grid. Both predeclared falsification verdicts are SUPPORTED exactly as frozen, including non-universality. The matrix also surfaced a latent runner defect (registerNewborn misread a newborn parent's genome patch as a second newborn row and refused the run), fixed under a recorded scoped exception with digest-identity proof on recomputed neutral cells and fail-closed negative tests. See [the scoped run record](../.scratch/14-reproduction-genetics/spec.md) for all verbatim values, both verdicts, the defect/fix record and cost measurements. Reports are labeled `synthetic/model-conditional`: no historical calibration, no takeoff guarantee, and no P0 scale extrapolation.

## Natural-limits v5 pilot evidence

The natural v5 reporter (`go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=both -out=<new file>`; also the predeclared matrix: A R=8 births-on × seeds 0/7 × workers 1/4 to h168, B R=0 × births{enabled,disabled} × seeds × workers to h12, C R=8 births-off × seeds × workers to h168, D the cultivation-disabled ablation (births-disabled), E the founder-tenure fixture cells (births-enabled-only), and F the 256-founder compute-ceiling cell with its workers-4 replay twin) forks one verified births-disabled neutral h0 checkpoint into recorded intervention children (the v5 h0 pulse is births-asymmetric by frozen design, so a disabled root is the only branch parent and the disabling direction is refused) and reports each; fixture cells publish the declared founder-tenure fixture's first h2 pulse-boundary continuation as their root. Automated checks cover the frozen G1/G2a/G2b/G3 conservation identities — including the per-patch-hour regen identity R = cultivated + wild + unrealized, the all-tenure-holders land identity, and the spoilage extended granary ledger — re-verified independently at every hour of every branch, restored-history head agreement, digest equality between each published bundle and its restored child, typed journal classification (including flat rule-508 grants, build-denial re-eats, birth requests/replies and allocation-collision lost-birth evidence), keyed-attempt event-link verification against the restored accepted history, zero birth evidence on every births-disabled branch, zero births at R=0, and zero build events in every ablation cell, and immutable no-clobber publication of every bundle and report with stray positional arguments rejected. Distributed metrics are the per-hour regen split with cultivated slots per patch, granary inflow/spoiled/stored aggregates, the trailing-24h Malthus index (Σmeals − Σbasal), per-actor ledgers with privileged tenure counts, and the full 14-row compute-ceiling headroom ledger quoted verbatim with every typed `NaturalCeilingHit` and its P15 verdict classification. Workers 1 and 4 produce identical evidence fields after stripping timing; the ceiling cell's workers-4 twin is checkpoint-hour- and journal-prefix-identical to the halting run at the hit hour. Recorded outcomes, verbatim: the organic ecology sits in the recorded poverty trap — the land-rush ratchet pins every actor at k=0 by ~h24, the birth gate (both parents k≥2 ∧ granary≥3) is unreachable, so A (births-on) is behaviorally identical to C (births-off): static 16 founders, zero birth requests, all deaths from aging (zero starvation), extinct at h141 (seed 0) / h143 (seed 7) with alive(168)=0 and |alive(168) − alive(120)| = 7/9 — the frozen A-band `alive(168) ∈ [8,18] ∧ |Δ| ≤ 6` is FALSIFIED on both limbs in all four A cells, reported and not tuned, with conservation green at every hour (the predeclared P15 "design is at fault" branch at the policy/behaviour layer). B: extinct at h11 with zero birth evidence on both flags in all eight cells. D: zero builds in all four cells (the denial loop engaged 1,896 (seed 0) / 1,963 (seed 7) times per cell, identical across worker counts), and the ablation's own finding: the D alive trajectories are hourly-identical to C in every cell — under the frozen policy cultivation changes no population quantity at all; its only footprint is the ledger (C received 288 cultivated units and ate every one of them as stored meals, plus 1,496 wild meals, while D ate 0 stored and its own wild meals — no cultivated unit sat unconsumed). E: the initial tenure foothold does change the outcome — 5 (seed 0) / 4 (seed 7) committed births against 231/210 filings, peak 17 alive at h3, but 10 starvation deaths follow (the first two at h4 are the gate-margin founders themselves, while h11/h12 are the first newborns' deaths against the organic h97/h100) and extinction moves to h132 (earlier than the organic h141) / h147 (later than the organic h143) — the foothold does not rescue the ecology. F: typed `NaturalCeilingHit` on the `history-events` ceiling at exactly h70 (spec estimate h40–60 or `history-patches` ≈ h100; observed between, at the projected binding hour h71 with rate 800 events/h), 256 alive = P·R exactly, conservation green, zero turnover (the poverty-trap context is recorded in the verdict evidence), workers-4 replay identical — classified "outgrew the harness" by the frozen band while the predeclared CPU budget is MISSED verbatim (19.2 s per simulated hour against the 0.5 s 3×-v4 budget; the 16-founder cells measured 0.05–0.08 s/h, inside; per-actor-year bytes and CPU are labeled extrapolation-only). See [the scoped run record](../.scratch/15-natural-limits/spec.md) for all verbatim values, the falsified A-band, the ceiling verdict, and the two supervisor-approved experiment seams. Reports are labeled `synthetic/model-conditional`: no historical calibration, no takeoff guarantee, and no P0 scale extrapolation.

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
