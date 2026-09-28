# Conceptual architecture

This document defines responsibility boundaries, not final implementation technology.

## 1. World kernel

The authoritative event-driven state transition system.

Responsibilities:

- validate intentions against state and rules;
- schedule activity duration and interruption;
- resolve concurrent effects from a consistent snapshot;
- commit events and advance simulated time;
- compute perceptions and information delivery;
- enforce conservation and causal invariants.

The kernel does not ask a language model to decide covered deterministic facts.

## 2. Actor fibers and component system

Every v0 person is an explicit actor fiber represented by a small engine record containing identity, revision, component and policy references, current activity, and next wake. This record defines how the engine schedules a person; it does not enumerate what a human can contain.

Actor and world state uses a unified logical component protocol. Every component has stable type and field identifiers, a schema, access rules, causal update rules, projections, versions, and migration metadata.

Physical implementations may differ behind the protocol:

- stable high-frequency components use compiled types, columnar storage, and specialized batch code;
- evolving or sparse components use registry-defined schemas and typed dynamic storage;
- a dynamic component may later receive an optimized implementation without changing its logical identity.

Strategies, rules, queries, events, adjudicators, and projections cannot branch on storage implementation. A conformance suite verifies equivalent observable semantics. Mutable implementation-specific access remains inside the component subsystem.

Dormant actors consume storage but no active execution slot. A time wheel or priority queue wakes actors at meaningful events. A bounded worker pool executes ready fibers. Host-language goroutines may implement workers without requiring one permanent goroutine per person.

Hot state remains memory-resident and data-oriented. Immutable history is stored separately. Storage measurements, not assumptions, decide later tiering or distribution. The detailed P0 contract is in [P0-SPEC.md](P0-SPEC.md).

## 3. Strategy runtime

A bounded virtual machine executes typed strategy graphs or bytecode. Strategies can:

- inspect actor-visible state;
- evaluate utilities and constraints;
- invoke bounded search or planning;
- update permitted private state;
- submit intentions;
- request cognitive escalation.

Strategies cannot inspect hidden world state, another actor's private state, or mutate authoritative state.

Shared immutable modules plus actor-specific patches avoid copying a complete program per person.

## 4. Cognitive scheduler

The scheduler allocates language-model and expensive-search budgets. Candidate priority is conceptually based on:

```text
priority = expected_civilizational_value / expected_cost
```

Candidate features include numerical estimates of impact, uncertainty, decision sensitivity, novelty, propagation potential, structural position, urgency, and prior strategy reliability.

The formula is a versioned hypothesis, not a fixed truth. It must expose feature values, uncertainty, selected action, cost, and realized downstream value.

Budget channels include exploitation of known high-value candidates, anomaly response, and protected exploration of ordinary actors.

## 5. Model manager

A model manager receives isolated cases containing compact state, the reason for escalation, relevant summaries, and evidence-query handles. It may propose:

- a one-time intention;
- a local strategy patch;
- a shared strategy-module candidate;
- a request for missing evidence;
- a new primitive or model-refinement proposal.

All outputs are schemas, not direct code or world mutations.

## 6. Adjudication and refinement

Separate logical roles evaluate:

- physical and procedural feasibility;
- strategy safety and information access;
- rule expressiveness gaps;
- institution-macro compilation;
- complexity budget;
- causal and historical validity.

A model may fill one or more roles operationally, but permissions and records remain separate. Unresolved proposals enter a sandbox rather than becoming reality by assertion.

## 7. Numerical projection layer

Structured and semantic state maps into versioned metric vectors used for:

- actor and world distance;
- scheduler priority;
- branch comparison;
- anomaly detection;
- calibration;
- rollout and rollback decisions.

A metric registry records schema, units, range, uncertainty, provenance, aggregation rule, and known blind spots. Text remains available for explanation and evidence retrieval, but selection algorithms consume declared metrics.

## 8. Experiment runner

The runner creates branches from a common checkpoint. It can compare:

- different actor strategies;
- different cognitive schedulers;
- different allocation budgets;
- different rule or institution implementations;
- policy interventions.

Beam-style development runs retain multiple promising branches according to a declared metric vector and diversity constraint. No single scalar score silently defines a good civilization.

## 9. Event store and checkpoints

The event store contains append-only, content-addressed segments. Checkpoints contain current state, pending activities, log positions, random-stream positions, schemas, and version references.

Branch records identify parent checkpoint, changes, compatibility, metrics, and evidence. Compatibility classes are `compatible`, `migrated`, `causally-tainted`, and `invalid`.

## 10. Analysis tools

Agents and developers inspect trajectories through layered summaries and evidence queries:

```text
current state
recent salient events
life-stage summaries
long-term traits and changes
original events and causal chains
```

Deterministic tools detect repetition, impossible knowledge, resource imbalance, causal gaps, stalled goals, and cohort anomalies before model-based analysis.
