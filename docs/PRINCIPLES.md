# Design principles

## P1 — Civilization fidelity under budget

Optimize the accuracy of civilization-relevant causal effects, not equal narrative detail for every person. Compute and language-model tokens are scheduled resources.

## P2 — Persistent individual actors

The v0 lower fidelity bound is an explicit actor fiber for every person. An actor fiber is a persistent state machine with an identity, state, strategy reference, current activity, and wake condition. It need not be a resident operating-system thread or goroutine.

Population aggregation is a future option only after evidence shows that explicit fibers are insufficient and that aggregation preserves identity, rare innovation, and macro/micro consistency.

## P3 — Strategy as mutable data

Routine behavior is an executable, typed, composable strategy rather than dynamically compiled host-language code. Strategies may search, plan, learn, inherit, recombine, mutate, and receive versioned patches.

The stable runtime owns safety, permissions, execution limits, and world access. Strategy replacement is atomic, tested, reversible, and traceable.

## P4 — Agents submit intentions

An actor or language model never mutates the world directly. It submits an intention. The world validates preconditions, schedules duration, resolves conflicts, commits events, and distributes perceptions.

## P5 — Objective and subjective state remain separate

Maintain distinct representations for objective reality, perception, belief, memory, public records, and inferred knowledge. A claim by a character is an event, not proof that the claim is true.

## P6 — Numerical projection is mandatory

State may contain structured symbols or text, but every property used for prioritization, comparison, goodness, distance, confidence, or optimization must have an explicit numerical projection.

A numerical projection declares:

- source state and transformation;
- units, bounds, and uncertainty;
- update rule and version;
- information lost by projection;
- calibration or validation method.

Text-only judgments cannot be the terminal input to scheduling, rollout, beam selection, or scientific claims. A language model may propose a score, but the score, rationale references, confidence, and later outcome must be recorded and calibrated.

## P7 — Selective language-model cognition

Language models wake for novelty, uncertainty, strategy failure, high decision sensitivity, high propagation potential, explicit escalation, or audit sampling. Rate limits, shared budgets, deduplication, and measured marginal value prevent uncontrolled self-invocation.

Structural importance is not sufficient. A protected exploration budget samples ordinary actors so low-probability innovation is not systematically excluded.

## P8 — Isolated batched management

Unrelated actors may share one model call to amortize common context, but their private information remains isolated by schema, namespace, output validation, and contamination tests. High-risk decisions may require separate calls.

Managers inspect indexed summaries first and retrieve original evidence on demand. Summaries guide attention; event records remain authoritative.

## P9 — Progressive model refinement

Natural laws do not begin to exist when requested. The simulator may progressively expose a more precise representation of laws already assumed by the world.

Keep separate:

- refinement of an under-specified world model;
- composition of a new technology or action;
- emergence of a social institution;
- an empirical hypothesis proposed by an actor;
- an explicitly separate universe variant.

## P10 — Institutions emerge from mechanisms

Do not preinstall modern institutional outcomes as historical necessities. Provide underlying mechanisms such as trust, authority, membership, coercion, exchange, promises, shared belief, records, and collective assets. Stable compositions may be compiled into efficient, versioned institutional macros.

## P11 — Controlled self-evolution

Self-evolution means proposed changes pass schemas, tests, sandbox trials, counterfactual evaluation, staged rollout, observation, and rollback. The simulation never silently rewrites its own meaning.

## P12 — Append-only evidence and compatible checkpoints

Keep immutable events, periodic state snapshots, indexes, rule and strategy versions, random-stream positions, and branch ancestry. A checkpoint that can technically resume is not necessarily causally valid after a rule change; compatibility status must be explicit.

## P13 — Models are tested, not trusted

Use invariants, adversarial scenarios, ablations, sensitivity analysis, independent implementations, historical calibration, and multi-seed comparisons. Plausible prose is not validation.

## P14 — Simulation is not real-world proof

Policy branches generate model-conditional evidence. Reports state assumptions, sensitivities, failure modes, and missing real-world data. They do not convert simulated success into a claim of real-world effectiveness.
