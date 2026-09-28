# P0 executable specification

Budget: 240 lines — the end-to-end P0 contract keeps module boundaries, interfaces, lifecycle, delivery slices, and acceptance gates together for one-pass implementation review.

Status: design draft

## 1. Purpose

P0 tests whether an event-driven world with explicit actor fibers, evolvable typed state, numerical projections, append-only evidence, checkpoints, and deterministic replay can run a small survival society for decades on one machine.

P0 is a vertical architecture experiment, not a complete model of humanity.

## 2. Scope

Target approximately 1,000 people in one bounded region over several simulated decades.

Include:

- birth, aging, death, kinship, health, energy, hunger, location, weather, resources, movement, gathering, eating, simple exchange, injury, and conflict;
- explicit actor fibers represented as state plus wake conditions;
- built-in and runtime-defined components behind one interface;
- deterministic rules, strategies with bounded search, events, projections, checkpoints, branches, and replay;
- escalation requests recorded for later analysis.

Exclude language-model calls, general institution emergence, distributed execution, molecular physics, natural-language dialogue, and claims about real policy.

## 3. Runtime modules

```text
ExperimentRunner
  ├── WorldKernel
  ├── FiberScheduler
  ├── StrategyRuntime
  ├── ComponentSystem
  │     ├── SchemaRegistry
  │     ├── BuiltinStore
  │     └── DynamicStore
  ├── ProjectionSystem
  ├── EventStore
  ├── CheckpointStore
  └── ValidationSystem
```

The kernel is the only authority that commits world state. Strategies and components do not append authoritative events directly. Stores do not make domain decisions.

## 4. Stable engine records

The actor core is deliberately small:

```text
ActorCore {
  actor_id
  revision
  lifecycle
  component_set_ref
  policy_set_ref
  current_activity_ref?
  next_wake
}
```

These fields support identity, concurrency, scheduling, and lookup. They do not define what a human contains.

Other entities such as locations, resources, items, and groups use the same entity identity and component model where practical.

## 5. Unified component protocol

Every component has a logical descriptor:

```text
ComponentDescriptor {
  component_type_id
  schema_version
  fields
  access_policy
  transition_rules
  projections
  dependencies
  migration_paths
  storage_class
}
```

Consumers use the same operations regardless of storage class:

```text
Describe(type) -> descriptor
Has(entity, type, world_version, authority) -> bool
Read(entity, selector, world_version, authority) -> typed view
PlanPatch(entity, patch, world_version, authority) -> accepted | rejected
CommitPatch(validated_patch, commit_context) -> component delta
Scan(type, selector, world_version, authority) -> typed batch
Project(entity_or_batch, projection, world_version) -> metric result
```

`Read` returns immutable snapshot views. Only the kernel may commit a previously validated patch. Missing, zero, unknown, and inapplicable are distinct states.

## 6. Physical component implementations

### Built-in hot components

Use compiled schemas, typed columnar storage, batch operations, and specialized transition code for stable high-frequency concepts such as location, lifecycle, energy, hunger, and wake state.

### Dynamic components

Use registry-defined schemas and sparse typed storage. Runtime registration requires types, bounds, access, update rules, projections, migration behavior, and complexity cost.

### Promotion

A dynamic component may be promoted to an optimized implementation while retaining its type ID and logical schema. A conformance suite must demonstrate equivalent reads, validation, commits, serialization, and projections.

No consumer may branch on whether a component is built-in or dynamic. Optimization is internal to the component system.

## 7. Typed value system

P0 supports a bounded set of value kinds:

```text
bool, integer, scalar, probability, enum, time, duration,
entity_ref, event_ref, vector, distribution,
record, optional, list, set, sparse_map
```

Each field declares bounds or units, missing-value behavior, and whether uncertainty is permitted. Names resolve to stable numeric IDs before hot-loop execution. Arbitrary untyped maps are not authoritative state.

## 8. State change protocol

```text
Observation
  → Strategy decision
  → Intention or EscalationRequest
  → deterministic validation
  → validated intention
  → simultaneous conflict resolution
  → component patch plan
  → atomic commit
  → events and metric deltas
  → future wakes
```

A rejected intention produces diagnostic evidence but no component mutation.

For S5, a trusted world runner adapts S4's engine-only raw evaluator to `Decide(context.Context, Observation) (Intention, error)`. An observation copies only the ready actor's ID, timestamp, snapshot version, own energy, and explicitly configured public cache IDs and stock values; it carries no component reader, authority, kernel handle, patches, private components, or other actors' energy. The actor's `WithdrawEnergy{CacheID, ObservedVersion}` is an attempt, not a patch. The rule rechecks the supplied version, trusted visibility, present cache stock of at least one joule, actor energy eligibility, and finite, exactly one-joule deltas in both stored scalar fields (comparing their exact binary-rational values, not rounded float subtraction) against the common kernel snapshot. It emits one proposal transferring one joule from cache stock to actor energy. The world assigns actor identity and fixed-width `(time, actor)` key; a shared cache-stock field admits at most one winner per timestamp even when multiple units remain. Invalid and collision-losing attempts never patch state.

The runner publishes a separate typed in-memory attempt journal after a successful scheduler step, including accepted and rejected outcomes with actor, action target and observed version, time, key, status, bounded reason, and optional linked event ID. Each batch records scheduler time and its committed kernel version and event tip. Actor callbacks receive neither raw events nor other actors' outcomes. A failed step publishes no outcomes; a successful rejected-only timestamp still closes its wakes. This facade is an in-process API boundary, not a sandbox, crash-atomic journal, or replayable rejection store; the actor-supplied version does not prove genuine memory.

For S3, the kernel plans existing-component field replacements against one snapshot and its read authority. Each proposal has a stable key unique among accepted events and within its batch, non-negative time, actor or authorized-world cause, registered rule ID/version, and schema-versioned patches. The kernel revalidates every plan before commit; any malformed, stale, cross-snapshot, or unauthorized plan aborts the entire batch. Each simulated timestamp is committed in one complete batch: a later batch at the same or earlier time is rejected. The kernel sorts by key and drops a whole later proposal when a field collides with an earlier winner. Each winner advances world version once and emits one typed event; an in-process commit publishes the new snapshot and its append-only hash chain together. S3's trusted evaluator receives readers but not the kernel commit handle; S5's actor callbacks receive neither. S3's actor cause checks only seeded entity identity, and a world cause requires the kernel handle plus snapshot authority; registered metadata does not define per-source write policies or executable domain-intention validation, which belong to S5. S3 rejects cross-schema transitions even if migration paths are listed; executable migration is a later bounded slice.

All decisions at one simulation timestamp read a consistent world version. Worker completion order cannot determine conflict outcomes.

## 9. Fiber scheduling

An actor fiber is persistent logical state, not a retained host-language stack. It is dormant unless a wake condition becomes true.

Wake causes include activity completion, relevant perceived event, need threshold, interruption, birth, scheduled commitment, strategy invalidation, and explicit audit.

In S4, the scheduler keeps one in-memory fiber per actor, with an indexed, cancellable wake heap and checked positive-duration activity tokens. Duplicate causes at one time coalesce into a single actor-ready entry; a token-bound pending interruption keeps its activity live until the interruption wake, even when other wakes occur earlier, and wins an equal-time completion tie. Cancellation removes the pending interruption without a wake. A perceived event alone never interrupts an activity. The next due time is processed as one ordered batch: bounded workers see one immutable kernel reader and authority, and the coordinator collects all results before planning and committing once. Scheduler effects are staged before commit and published only on success; effects conditional on a proposal apply only to a collision winner. Every processed timestamp closes to future wakes at that same time, including empty component batches; post-commit event-derived wakes must target a strictly later microsecond. No permanent actor goroutines, domain intention validator, or strategy representation are introduced in S4.

The S4 scheduler snapshot is an in-memory quiescent-boundary copy of fibers, activities, pending wakes, clock, and token counter. Restore rebuilds and validates the heap against the matching process-local kernel origin, version, and event tip obtained atomically with the kernel reader; a different kernel at the same empty event tip is not a valid in-memory restore target. The scheduler remains the sole intended kernel writer while active; S3 replay alone cannot recover scheduler-only changes, and S4 makes no disk-durability claim.

## 10. Strategy boundary

P0 strategies can:

- read authorized component views and projections;
- generate candidate intentions;
- evaluate typed utility terms;
- perform bounded search;
- update permitted private strategy state through planned patches;
- request escalation.

Strategies cannot inspect storage implementations, access hidden components, register schemas, commit state, or append authoritative events.

The S6 survival experiment uses a pinned format-v1 policy (`Ref{ID, Version}`), complete typed Eat/Rest action specifications, actor-specific parameter overrides, and bounded one-step exhaustive utility evaluation. The value-only observation contains own energy/hunger and configured visible cache stocks; the choice is an intention, not patch authority. Candidate and view caps, stable tie ordering, and typed Wait fallback limit evaluation. This experiment does not settle a final strategy ontology or implement multistep planning.

## 11. Initial numerical projections

P0 registers at least:

- individual energy reserve, hunger pressure, health risk, injury burden, travel cost, resource expectation, action risk, relationship affinity, trust evidence, and decision uncertainty;
- population size and structure, mortality, morbidity, food stock, production, resource depletion, conflict incidence, strategy failure rate, behavioral diversity, and compute cost.

Each result contains value, uncertainty, projection version, and provenance. World comparison uses a metric vector. No permanent universal goodness score exists in P0.

## 12. Initial survival loop

```text
hunger threshold wakes actor
→ actor observes body, known food, location, weather, and permitted social facts
→ strategy generates eat / gather / request / exchange / steal / wait candidates
→ bounded evaluation selects an intention
→ kernel validates knowledge, access, distance, ability, and resources
→ simultaneous claims on the same resource are resolved deterministically
→ travel or gathering activity is scheduled
→ completion or interruption wakes the actor
→ committed consumption changes food, energy, health, and future wake time
→ events and projections record the causal chain
```

Every arrow must be testable without reading narrative prose.

The bounded S6 demo implements only the eat/rest subset: a trusted survival adapter constructs actor-specific observations from a common scheduler snapshot and independently validates pinned choices. Four finite food-energy caches are visible in restricted pairs; each hourly Eat consumes two stock units, restores one actor energy unit and decreases hunger by one; Rest consumes one actor energy unit and increases hunger by one. Hunger is a dynamic component with a versioned projection and remains the complement of energy to capacity eight. Every accepted action dissipates exactly one unit of total energy plus food. One shared cache field admits one Eat winner per timestamp; losers' attempts do not mutate state and can retry at their next wake. Zero-energy fibers stop; a Rest that reaches zero stops its actor only if that proposal wins, in the same timestamp even at the configured horizon. The finite horizon and bounded population are not claims about full P0 survival.

## 13. Event and checkpoint contracts

An event records ID, simulated time, before and after world versions, kind, cause, actor or authorized world source, rule version, typed payload, visibility, component deltas, and metric deltas. For S3 `component-patch` events, the typed field deltas are the complete typed payload; visibility is deliberately deferred to the perception slice and must not be inferred from an absent visibility field.

A checkpoint records format version, simulation time, world version, entity and component state, pending activities, wake queue, strategy and rule versions, projection versions, random-stream positions, event-log references, branch ancestry, compatibility class, and integrity hashes.

The bounded single-machine survival checkpoint (format 1) is an immutable, atomically published five-section bundle: kernel genesis/accepted-event chain and canonical snapshot/projection head, portable scheduler fibers/activities/wakes, exact registered strategy and actor bindings, typed attempt/outcome journal with original run configuration, and causal manifest/branch lineage. At a completed coordinator step it captures the deterministic scenario seed, initial metrics, random stream ID/position, active Eat weight, original Eat weight, source checkpoint digest/head, and explicit compatibility class. Restore reconstructs expected schemas and policy independently, checks every section and cross-section identity before returning a world, and rejects incompatible seed, schema/rule/projection, policy, or format without migration. A deliberate Eat-weight fork verifies its source checkpoint and retains historical config and attempts before rebinding future actors. Worker count is not causal identity. Accepted events and metric projections are independently replayed; historical rejection reasons are checked for typed structure and event links but cannot be independently recomputed without past observations. Full perception packets are not persisted. Digest chains detect damage, not malicious replacement; a checkpoint is a quiescent boundary, not per-step crash durability. Recovery validates references, queue times, versions, hashes, random streams, and invariants before continuation. S3 replay starts from the same seed and registry, groups accepted events by timestamp into complete batches, and checks canonical bounded event encoding, contiguous IDs and versions, chain hashes, key ordering, schema/rule/projection references, before-values, and recomputed deltas. It cannot recover rejected or collision-losing proposals from accepted-only events; S5 keeps that intention-level evidence only in the separate in-process attempt journal. S6 records the pinned strategy reference in a typed attempt journal and deterministic, bounded world-generated event key; the adapter checks both against its immutable run configuration. Accepted-event replay alone does not recover rejected attempts or wakes. The survival checkpoint binds and durably publishes their portable evidence together. S3's standalone log remains in-process only, without standalone crash durability, disk segments, or fsync. Its hash chain detects accidental or partial alteration but does not authenticate against an adversary who can rewrite the entire chain without a trusted external tip.

## 14. Implementation slices

1. Entity IDs, simulation time, world versions, random streams, and typed values.
2. Schema registry plus one built-in and one dynamic component with conformance tests.
3. Event store, component patch transaction, and deterministic replay.
4. Scheduler, dormant fibers, activities, wakes, and worker batches.
5. Observation, intention, validation, conflict resolution, and commit.
6. Minimal bounded strategy runtime and the survival loop.
7. Projections, invariants, checkpoint recovery, branching, and long seeded runs.
8. Profiling and optimization without changing logical contracts.

## 15. Acceptance gates

P0 is complete when:

- built-in and dynamic implementations pass the same component conformance suite;
- a seeded survival world replays to identical authoritative state and metrics;
- rejected actions never mutate state;
- worker scheduling does not change outcomes;
- checkpoint recovery continues the same causal sequence;
- every compared semantic property has a versioned numerical projection;
- long runs preserve declared invariants;
- bytes per actor-year, events per second, checkpoint throughput, and replay throughput are measured;
- observed limitations and escalation requests determine the next design revision.
