# Open questions

These questions are deliberate research work, not silently settled assumptions.

Phase references name the active plan's slices (VS1–VS6, [VERTICAL-SLICES.md](VERTICAL-SLICES.md)); the pre-pivot P0–P4 phase plan ([ROADMAP.md](ROADMAP.md)) is historical.

## Q1 — Civilization-level distance

**Question:** Which metric vector distinguishes materially different worlds without imposing one preferred civilization?

**Current answer (fixture f10-s0, H=144; M2b ticket 37, repaired by M2c part 1 ticket 38, 2026-10-18):** a three-component demographic viability order, `vrd/2` — surviving actor-hours, cumulative births and cumulative starvation deaths, read below the no-AI control's own realised outcome in the declared priority order (population survived, then deaths avoided, then stability/reproduction). It distinguishes the recorded arms in the declared direction (an extinct, zero-birth arm ranks last with a categorical persistence separation; a high-churn arm ranks above it but below the control) and it imposes no preferred civilization beyond persist, reproduce and avoid starvation deaths, a floor the world already reaches without cognition. The R-anchored divergence vector is rejected as an objective at this fixture: the reference is extinct (0-1 births, 0 alive at the terminal sample), so it rewards losing population. Metric contract: [VALIDATION.md](VALIDATION.md); evidence and records: [`.scratch/37-fidelity-metric/spec.md`](../.scratch/37-fidelity-metric/spec.md) and [`.scratch/38-m2c-preregistration/spec.md`](../.scratch/38-m2c-preregistration/spec.md).

**Still open:** cross-fixture and cross-horizon validity (f12-s0 untested; a fixture whose control is itself not viable needs a declared substitute floor), the identity/diversity of survivors and age-death timing are not represented, and the repair traded away the ability to express any tradeoff between its tiers (one actor-hour of survival outranks any reproduction difference, by construction; the stability tier is a reproduction proxy, not a churn measure). The rankings are declared, not calibrated: `vrd/2` has no weights at all, so its sensitivity check is the six tier permutations plus the terminal-sample convention, not a weight sweep.

**Experiment:** Compare expert judgments, causal interventions, and metric rankings across controlled branches; the first such comparison on recorded branches is ticket 37's, and the next is the M2c matrix, whose criterion is pre-declared in ticket 38 §7 and whose live run is ticket 39 (arms `A0/P/U/A/A1`, cell f10-s0, H=144, 3 replicates, contract `natural-claim/v4`).

**Decision needed:** none for M2c to run — the metric is declared, the criterion is pre-declared and the anchor is reproducible at zero spend. Beam pruning still needs the cross-fixture part of this question and a declared aggregation of `vrd/2`'s tuple.

## Q2 — Cognitive intervention value

**Question:** How can the scheduler predict that expensive reasoning will improve civilization-level fidelity before downstream effects occur?

**Current hypothesis:** Combine impact, uncertainty, decision sensitivity, novelty, propagation, structural position, and cost, then calibrate against realized branch differences.

**Experiment:** Evaluate competing scheduling policies from identical checkpoints and budgets.

**Decision needed:** A provisional version for VS2; permanent resolution is not expected.

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
