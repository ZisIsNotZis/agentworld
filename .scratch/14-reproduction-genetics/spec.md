# Reproduction and genetics (design)

Status: claimed

## Issue

The capacity v3 pilot produced endogenous, quantified surplus (frozen indicator: actors self-sustaining at k≥2 with granary ≥1, surplus flow measured per cell), and the operator's recorded sequencing makes reproduction the next mechanism now that surplus exists. The world still has a fixed population of 16 actors with no birth, no genome, no heredity, no age, and no derived kinship; "affinity" in v2 is synthetic and explicitly non-genealogical. Reproduction must not repeat the zero-sum trap: births without a surplus precondition collapse back to ticket 09's extinction dynamics.

## Operator direction (recorded 2026-09-29)

The operator authorized the post-surplus arc broadly: reproduction and genetics first, then exchange/trade, money, and richer social concepts. This ticket covers the first step; each subsequent mechanism gets its own design ticket after its predecessor lands.

## Objective

Design — not implement — the minimal reproduction-and-genetics mechanism such that:

- birth is a checked transition with real preconditions (two living parents' independent consent, energy/time cost, surplus gate tied to the frozen capacity indicator), never spontaneous spawning;
- a heritable genome (versioned component) transmits traits through deterministic crossover/mutation on the existing SplitMix64 seeded streams — fully replayable, no hidden randomness;
- death and age become first-class (the P0-SPEC wake causes already reserve birth; aging bounds lifetimes so generations turn over);
- kinship is derived from the reproduction graph, never hand-declared — replacing the synthetic v2 affinity roster when social mechanisms are later re-merged;
- population dynamics are endogenous: the frozen gates must admit growth, stability, AND decline/extinction as outcomes, with the surplus gate tested by a branch intervention (births-enabled vs births-disabled from one verified neutral checkpoint, ticket-12/13 discipline);
- per-actor-year byte/CPU budgets remain bounded at the 16-actor fixture and state the path to the P0 ~1,000-actor target (runtime entity allocation now becomes load-bearing).

## Design questions (must be answered with exact numbers)

1. Genome representation: which traits, bounds, encoding, and which capacity/survival parameters they modulate (e.g., yield efficiency, wear resistance, gather priority weight, metabolic rate) — every trait effect must be integer-exact and conservation-preserving.
2. Mating model: how two actors meet/consent given the current hour lattice; what is public vs private (v2 privacy discipline); inbreeding/kin-distance constraints from the derived kinship graph.
3. Costs: gestation/parenting energy and time, who pays, what happens on parent death mid-gestation.
4. Birth placement in the frozen clock and the runner's wake-cause handling (new entity allocation, initial components, strategy binding for a newborn — policy ref inheritance vs fresh binding is a strategy-registry question).
5. Aging/mortality: trait-influenced or fixed; death bookkeeping consistent with frozen "dead actors' state" conventions (worksite decay continues, yield stops, granary frozen).
6. Inheritance: does capital/granary transfer to offspring (operator's earlier note: dead granaries frozen inaccessible — inheritance is the first institutional question; if deferred, say so explicitly and keep the deferral).
7. Population bounds and the surplus gate: exact threshold expression over the frozen capacity metrics; carrying-capacity interaction with k_eq and q.
8. What observation is actor-visible vs privileged; conformance-test requirement as in v2/v3.

## Required designer outputs

Chosen mechanism plus rejected alternatives with reasons; all frozen constants (integer-exact, bounds-declared); clock/phase changes; gate list with concrete numeric thresholds, negative fixtures, and falsification criteria for "heritable variation affects population outcomes"; experiment matrix from verified neutral checkpoints (births on/off branches at minimum; surplus-gate interventions); metric definitions (population trajectory, genome diversity, kinship density, per-actor-year bytes); lane board; stop conditions. Constraints identical to ticket 13: no v1/v2/v3 byte changes (separate v4 version), determinism/replay/checkpoint discipline unchanged, no post-hoc tuning, negative results valid.

Need-review: yes
Need-test-cases: yes

## Comments

EOF
echo done