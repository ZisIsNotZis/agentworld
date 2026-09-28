# P0 architecture and interface design

Status: done

## Issue

The foundation documents define the objective and architecture layers but do not yet specify an implementable P0 contract. The actor-state model also needs to reconcile high-performance built-in concepts with runtime-extensible concepts behind one logical interface.

## Objective

Define the P0 vertical architecture, core interfaces, actor-state model, event loop, numerical projection contract, storage boundaries, checkpoint contract, and first end-to-end behavior without prematurely fixing the strategy instruction set or host-language implementation.

## Acceptance criteria

- Built-in and dynamic components share one logical interface.
- Physical storage may optimize built-in components without observable semantic differences.
- Runtime schema evolution, access control, projection, and migration are defined.
- P0 module ownership and prohibited dependencies are explicit.
- One survival loop is specified end to end.
- P0 implementation slices and acceptance gates are actionable.
- Existing architecture documents no longer imply a closed Person structure.

Need-review: yes
Need-test-cases: no

## Blockers

None.

## Comments

- 2026-09-17 — assistant: Started after approval of the hybrid built-in/dynamic component model and unified runtime interface.
- 2026-09-17 — assistant: Added `docs/P0-SPEC.md`, ADR 0005, and updated architecture and navigation. P0 specifies the component protocol, two physical storage classes, promotion equivalence, event lifecycle, survival slice, delivery order, and gates.
- 2026-09-17 — assistant: Fresh scoped self-review checked authority boundaries, open-world extensibility, hot-path feasibility, numerical projections, replay, migration, and premature implementation commitments. Verdict: accepted as a design draft. Strategy representation and host language remain experimental decisions.
- 2026-09-17 — assistant: All local Markdown links resolve and `git diff --check` passes. All documents satisfy the default size budget except `P0-SPEC.md`, whose explicit 240-line budget keeps the complete vertical contract reviewable in one pass.
