# Productive capacity and surplus (design)

Status: claimed

## Issue

Current fixtures are strictly zero-sum. Production `q` is an exogenous frozen constant; no actor can raise output. In q3 the equilibrium is structural: 3 units per patch-hour vs 8 needing 1 each, so exactly 3 survivors per patch (6/16), and the observed social-food outcome was near-determined — gifts only redistributed a fixed quantity, and one unit of food equals exactly one hour of life, so donors died and recipients survived by margin alone. No mechanism exists for investment, technique, capital, storage, or labor reallocation, therefore no endogenous surplus, no takeoff/decline dynamics, and nothing for later reproduction or credit systems to build on.

## Operator direction (recorded 2026-09-28)

Sequencing decided by the operator: (1) productive capacity first; (2) endogenous surplus; (3) only then reproduction/genetics (reproduction without surplus is meaningless when only 6 of 16 survive); (4) affect/motive richness (selfishness, altruism, opportunism); (5) reciprocity/credit (loan/repay) only after both surplus and richer motives exist — credit before surplus and before motives was judged uninteresting. Ticket 12 is acknowledged as a valid infrastructure demo with unsurprising, expected conclusions.

## Objective

Design — not implement — the minimal productive-forces mechanism such that:

- actors can convert current consumption into future productive capacity (a real trade-off under uncertainty, so identical worlds can diverge);
- food can endogenously exceed subsistence carrying capacity, making sustained surplus possible;
- v1/v2 fixture, policy, journal and checkpoint bytes are preserved; the mechanism is a separately versioned v3 fixture;
- a surplus indicator is defined precisely enough to gate future reproduction work;
- frozen gates permit both takeoff and collapse, and state what would falsify "productive forces emerged".

## Design questions (must be answered with exact numbers)

1. Production function: how does invested effort/capital/technique change per-hour output? Diminishing returns? Maintenance/depreciation?
2. What is invested: time (activity choice), food units, durable tools, or a combination?
3. Storage: bag capacity is 1 today; investment above one unit's worth requires either a store (per actor? per patch? shared?) or work-in-progress counters. Ownership and appropriation rules of improved capacity must be explicit — this is the seed of later institutional work.
4. Hourly choice set per actor and its phase placement relative to Gather/Eat.
5. Heterogeneity: initial endowment, and whether heterogeneity is required for takeoff to be possible.
6. What makes takeoff possible but not guaranteed (threshold/positive feedback without preordaining success), and what the collapse mode is (depreciation spiral, over-investment starvation).
7. What observation is actor-visible vs privileged (privacy discipline as in v2).

## Required designer outputs

Chosen mechanism plus rejected alternatives with reasons; exact frozen constants (all thresholds, costs, yields, depreciation, storage bounds); clock/phase changes; the gate list with concrete numeric thresholds and negative fixtures; metric definitions (surplus, capital stock, productivity, per-actor-year bytes and CPU); a lane board (contract → policy → runner → persistence → experiment → review); stop conditions.

## Constraints

- No changes to v1/v2 bytes; v3 is separately versioned and incompatible-by-default as before.
- Determinism, replay, checkpoint/branch discipline identical to ticket 12; comparisons branch from one verified neutral checkpoint with recorded interventions.
- All gates predeclared before any experiment run; a worse or null result (no takeoff anywhere in the matrix) is a valid outcome and must be reported verbatim.
- Synthetic/model-conditional labeling; no historical calibration claim.

Need-review: yes
Need-test-cases: yes

## Comments

EOF
echo created