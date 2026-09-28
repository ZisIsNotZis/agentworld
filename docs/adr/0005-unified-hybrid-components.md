# ADR 0005 — Unified hybrid component model

Status: accepted for P0

## Context

Some actor and world concepts are universal, frequently accessed, and performance-sensitive. Examples include lifecycle, location, energy, current activity, and wake time. Encoding all state as generic key-value records would add avoidable lookup, allocation, serialization, and validation costs.

The complete human and social ontology is not known in advance. Encoding every possible property as a host-language field would prevent runtime model evolution and make schema changes require global recompilation and migration.

## Decision

Expose built-in and dynamic state through one logical component protocol. Components share stable type and field identifiers, schemas, typed values, access rules, causal update rules, metric projections, versions, and migration metadata.

Allow multiple physical implementations behind that protocol:

- built-in hot components use compiled types, columnar storage, and specialized code;
- dynamic components use registry-defined schemas and typed sparse storage;
- a dynamic component may later receive an optimized physical implementation without changing its logical identity or consumer interface.

The actor core contains only engine-level scheduling and identity references. It does not enumerate the possible contents of a person.

## Required equivalence

For the same logical component and world version, every implementation must agree on:

- schema-visible fields and missing-value semantics;
- read and write authorization;
- validation and state-transition results;
- serialization meaning;
- event and projection outputs;
- migration compatibility.

Implementation-specific mutable access is forbidden outside the component subsystem.

## Consequences

- Strategies, rules, adjudicators, events, queries, and projections use one protocol.
- Common concepts retain efficient layouts and batch operations.
- New concepts can be registered at runtime without modifying the actor core.
- The registry, conformance suite, and migration system become critical infrastructure.
- The protocol must support batch access so abstraction does not force slow per-field virtual dispatch in hot loops.
