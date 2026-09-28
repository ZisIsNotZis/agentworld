# Vision

## Objective

Build a long-running civilization simulator that allocates finite computation to the decisions most likely to change civilization-scale outcomes.

The simulator should support persistent individuals, relationships, resources, institutions, technologies, culture, conflict, and historical divergence from a small founding population through increasingly complex societies.

## Central claim

Most people perform most actions through habits, learned procedures, local search, and institutional routines. These actions may be rich in lived experience while remaining mechanically predictable at civilization scale.

High-cost reasoning is therefore not distributed uniformly. It is reserved for decisions whose uncertainty and potential causal impact justify the cost. The system succeeds when this allocation preserves civilization-level fidelity better than uniform or periodic model invocation at the same budget.

## Desired properties

- **Causally grounded:** every state change has an authorized cause and an inspectable chain of events.
- **Self-interested actors:** people act from their own needs, beliefs, loyalties, fears, ambitions, and limitations rather than default helpfulness.
- **Finite-budget:** compute, language-model calls, storage, and analysis are explicit resources.
- **Quantifiable:** semantically rich state has numerical projections sufficient for comparison, prioritization, calibration, and optimization.
- **Adaptive:** simulation and reasoning fidelity increase where expected civilizational value is high.
- **Evolvable:** strategies, institutions, and explicit model coverage can improve without unaudited mutation of reality.
- **Replayable:** runs, branches, rule versions, random streams, and checkpoints are inspectable and reproducible.
- **Data-driven:** design changes follow trajectory evidence and controlled experiments rather than narrative intuition alone.

## What the project is not

- A claim that simulated people are conscious.
- A molecular physics simulator.
- A requirement to reproduce the recorded historical timeline.
- A language model continuously role-playing every person.
- A system in which adjudicators invent convenient facts or natural laws.
- A tool that proves real-world policy outcomes without empirical validation.

## Historical and future use

Historical periods provide known constraints and evidence for calibrating human, material, institutional, and technological mechanisms. Valid runs may diverge greatly from recorded history.

Future simulations should produce conditional branches and sensitivity analyses, not a single authoritative prediction. They can expose assumptions, possible downstream effects, and robust failure modes while remaining bounded by the world model's knowledge.

## Long-term experiment

From a shared checkpoint, run controlled branches that vary policies, institutions, actors, or cognitive-allocation strategies. Compare distributions across many random seeds and state which assumptions drive the result.

The valuable product is not one predicted timeline. It is an inspectable causal map of which conditions make different civilization paths more or less likely.
