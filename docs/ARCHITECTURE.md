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
- request cognitive escalation (implemented by VS2 as the actor-raised, observation-only
  `EscalationRequest` predicate; [VERTICAL-SLICES.md](VERTICAL-SLICES.md), VS2).

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

VS2 (attention) implements this mechanism: the value-based priority layer, actor-raised escalation
requests, and protected exploration of ordinary actors are built on master `48b4882` and verified
hermetically, but the mechanism is not yet accepted as the central claim (the real-model arm is blocked
on spend authorisation, and the value model's weights and feature derivations await operator
confirmation). VS1's baseline allocation policy remains the declared uniform/periodic `WakePolicy`
([VERTICAL-SLICES.md](VERTICAL-SLICES.md), VS1 C7; `internal/cognition/protocol/wake.go`,
`VS1WakePolicy`).

## 5. Model manager

A model manager receives isolated cases containing compact state, the reason for escalation, relevant summaries, and evidence-query handles. It may propose:

- a one-time intention;
- a local strategy patch;
- a shared strategy-module candidate;
- a request for missing evidence;
- a new primitive or model-refinement proposal.

All outputs are schemas, not direct code or world mutations.

### 5.1 The Role A decision seam and its declared action vocabulary

The first implemented manager seam is Role A: the actor's claim-instant action (`natural.claim`,
`internal/cognition`, `internal/world/natural_cognition.go`). Its request is one canonical JSON object
built by an allow-list encoder, and it is versioned by contract (`natural-claim/v1`, `/v2`, `/v4`,
`internal/cognition/observation.go`). `v1` and `v2` carry the world's own field names; `v4` is the
display-vocabulary contract: it states the fact a decision depends on rather than the raw history
behind it (a gate's outcome rather than the timestamp that gates it, a count of the food waiting rather
than eight per-slot rows), it carries no row no decision reads, no policy-constant vector, and its
system prompt states the situation, a persona card, the goal in the actor's own terms, and the four
world facts no field name can carry.

**Declared exception to one-name-one-thing: each claim action has two names.** The internal IDs
(`gather`, `eat-stored`, `provision`, `wait`) are frozen wire and storage tokens: they appear in every
recorded request, in the world's claim-action set, in the role-a intervention records and in the
admissibility ladder, and renaming them would invalidate every recorded ledger and every strategy
fingerprint. The display labels (`take_slot_food_into_bag`, `eat_granary_food`,
`take_granary_food_into_bag`, `wait_this_hour`) are the canonical domain names: they say what the world
does, they are what the model is asked for, and they are what a reader should use. The two are bound by
exactly one table, `protocol.ClaimActionVocabularyTable`, with helpers `ClaimActionLabel` /
`ClaimActionID` and the test `TestClaimActionVocabularyIsABijection`: every ID has exactly one label,
every label has exactly one ID, no token repeats on either side. The v4 ladder maps a label back to its
ID before running the one admissibility ladder, so the world only ever sees a frozen token, and it
accepts no second spelling: an unmapped label is inadmissible, never an alias. The migration path is a
future contract version in which the internal IDs are renamed to the labels with a replay-preserving
alias table, so an old ledger still replays; until that version exists, the duplication is declared
here, in `.scratch/36-harness-vocabulary/spec.md`, and in the table's own comment.

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
