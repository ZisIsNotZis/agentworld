# ADR 0001 — Event-driven explicit actor fibers

Status: accepted for v0

## Context

The simulator must preserve individual identity while supporting populations larger than a continuously reasoning multi-agent system. Permanent host-language goroutines waste scheduling and stack resources when most actors are dormant. Statistical aggregation may lose rare innovation and identity before its error is understood.

## Decision

Represent every v0 person as an explicit actor fiber stored as data. Wake fibers only for meaningful events and execute ready work through a bounded worker pool.

A fiber is a logical contract, not a required runtime primitive. The implementation language remains undecided until prototype requirements and benchmarks justify it.

## Consequences

- Individual state remains inspectable and addressable.
- Dormant people do not consume active execution slots.
- The design requires an event queue, wake conditions, compact state, and deterministic conflict handling.
- Future aggregation or distribution remains possible but requires measured evidence and fidelity tests.
