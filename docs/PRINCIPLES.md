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

VS2 implements this mechanism: value-based selective waking, actor-raised escalation requests, and protected exploration are built on master `48b4882` and verified hermetically, though the mechanism is not yet accepted as the central claim (the real-model arm is blocked on spend authorisation and the value model's weights and feature derivations await operator confirmation). VS1's baseline wake policy is the declared uniform/periodic object with the honest baseline only ([VERTICAL-SLICES.md](VERTICAL-SLICES.md), VS1 C7).

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

## P15 — Limits come from nature or from our compute, never from arbitrary design caps

Operator decision, 2026-09-29. Modeled populations, stocks, and production must be bounded by modeled natural constraints: accessible resources, land and space, energy and material conversion, upkeep, regeneration, mortality, and agents' own choices. A global design cap such as "at most N births", a legible ID range that silently caps a population, or a hard ceiling on an institution is prohibited as a *modeling* device.

The only legitimate hard limits are the simulator's own finite compute and storage. Those limits must be:

- instrumented and reported as the binding constraint (which ceiling, at what value, at which hour);
- fail-loud, never silent truncation of a trajectory;
- resolved case by case, with the outcome reported verbatim: hitting a compute ceiling means either the run outgrew the harness — a successful result to be recorded and scaled — or the design is at fault — a defect to be diagnosed and fixed. It is never grounds for quietly lowering the mechanism.

A frozen experimental gate may still bound an *experiment* (a horizon, a fixture size, a sampling schedule), and a versioned fixture may still declare its own bounds for reproducibility; but such bounds are experiment parameters, and no mechanism may depend on them to appear viable. When a bound in a fixture is doing modeling work, it is a P15 violation and must be replaced by a modeled constraint.

## P16 — A bulk text edit checks its own anchors and its own output size

2026-10-26: a documentation sync parsed a file into `start := index(A)`, `end := index(B)` and replaced
`text[start:end]` without checking the order. `B` occurred *before* `A`, the slice was empty, and
`strings.Replace(text, "", new, -1)` inserted the replacement between every character of a 46 KB file,
producing a 55 MB commit that reached the remote. The rule that prevents the whole class: a scripted
edit asserts `end > start`, asserts the matched block's length is plausible, and asserts the result's
size changed by less than 2× before it writes; a slice whose end precedes its start is a defect signal,
never an empty replacement. History was rewritten (`--force-with-lease`) to remove the blob, which is a
cost the assertion would have avoided.
