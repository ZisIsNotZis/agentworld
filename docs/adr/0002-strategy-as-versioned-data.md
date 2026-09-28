# ADR 0002 — Strategy as versioned data

Status: accepted for v0

## Context

Every actor may have inherited, learned, cultural, and individual strategy differences. Language-model managers must update strategies during a run without recompiling host code, losing checkpoint compatibility, or granting direct world mutation.

## Decision

Store strategies as typed, executable, versioned data interpreted by a bounded runtime. Compose shared immutable modules with actor-specific state and patches.

Every change records its parent, proposer, reason, validation result, rollout state, and affected actors. Running activities retain their committed semantics; the next eligible decision observes the new strategy reference.

## Consequences

- Runtime strategy updates become atomic data changes rather than host-code hot reloads.
- Strategies can be inherited, recombined, mutated, tested, shared, and rolled back.
- The runtime requires schemas, capability checks, execution budgets, and migration rules.
- The initial instruction set remains an open implementation decision.
