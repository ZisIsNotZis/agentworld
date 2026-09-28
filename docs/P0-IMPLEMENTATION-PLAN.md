# P0 implementation plan

Status: S0–S5 integrated; bounded strategy and survival behavior are next

This plan sequences the executable work described by [P0-SPEC](P0-SPEC.md) and preserves the unified component boundary accepted in [ADR 0005](adr/0005-unified-hybrid-components.md). Go is provisional while P0 measures correctness and cost.

## Implemented foundation

### S0 — Reproducible project gates

- A dependency-free Go module establishes the provisional language baseline.
- The Make targets `fmt-check`, `test`, `race`, `vet`, `build`, and `bench` are the repository gates.
- No placeholder command is provided before a runner exists.

### S1 — Stable simulation values

- Numeric IDs, exact microsecond simulation time and durations, world versions, and checked entity allocation establish stable primitives.
- A project-owned counter-based random stream makes output a function only of seed, stream ID, and position.
- The closed value algebra retains its declared kind while distinguishing present, missing, unknown, and inapplicable states.
- Composite values are validated, defensively copied, and canonically ordered where order has no semantic meaning.

### S2 — Unified immutable components

- A validating builder freezes component descriptors into an immutable schema registry.
- Energy uses a hidden compiled columnar store; fatigue uses a hidden registry-driven sparse store.
- Consumers receive only `component.Reader` and immutable read, scan, and projection results.
- Authorization is an opaque snapshot capability. The foundation serves exactly one world version.
- Scalar batch columns provide bulk copies without per-field interface dispatch.
- Affine projections include typed value state, uncertainty, projection and schema versions, unit, source fields, entity identity, and world-version provenance.
- One read-protocol conformance suite covers both storage classes; a separate same-schema energy test checks logical read/scan/project equivalence.

## Transition and scheduling slices

- S3 adds kernel-held patch planning and commit authority, authoritative events, atomicity, and deterministic in-process replay. Cross-schema migration and durable storage remain later work.
- S4 adds dormant actor fibers, indexed wake conditions, timed activities, deterministic ordered worker batches, and in-memory scheduler snapshots validated against a matching kernel head. It does not add disk durability or strategy semantics.
- S5 adds an actor-specific observation and one public-cache withdrawal intention, checked against the common kernel snapshot. A single-winner cache field collision links accepted actions to deterministic energy/stock events; a separate in-memory journal retains rejected outcomes without state mutation. It does not provide a strategy VM, crash-atomic attempt storage, or replay of rejections.

## Next slices

Later slices add bounded strategies, survival behavior, durable checkpoints, branching, long-run validation, and performance-driven optimization.

The S0–S5 tests do not establish durable persistence, migration, or full P0 promotion equivalence. Performance results are baselines, not pass/fail thresholds.
