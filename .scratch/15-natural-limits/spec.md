# Natural production limits and resource-determined population (design, v5)

Status: claimed

## Issue

Ticket 14 (v4 reproduction/genetics) delivered both frozen verdicts as SUPPORTED, but its outcome was structurally shaped by arbitrary design caps rather than by modeled nature: a global `MaxBirths = 16` ceiling, a legible newborn ID range (10001..10016) that silently caps the population at 32, and a runner allocation bound that exists only because of that range. Every enabled cell grew to 18–21 then collapsed to zero *because* the cap plus founder aging ended generational turnover — so the population trajectory was determined by our bookkeeping, not by resources.

Worse, the v3/v4 productive mechanism has no natural input at all: a capital point yields one food unit per hour forever, from nothing, so aggregate food is bounded only by per-actor capital bounds (KMax = 8) and the horizon. Carrying capacity is therefore not modeled; it is imposed by fixture bounds. Per P15 this is prohibited as a modeling device.

## Operator direction (recorded 2026-09-29)

"Continue; do not set unreasonable hard caps. Let natural resources determine the limits. The only limit is our compute limit. If we hit the compute limit, then either the run succeeded (we outgrew the harness) or our design has a problem, and that must be analyzed case by case." Recorded durably as principle P15 in `docs/PRINCIPLES.md`.

## P15 audit of the current world (by inspection, to be confirmed by the designer)

| Item | Current form | P15 status |
|---|---|---|
| `ReproductionMaxBirths = 16` | global lifetime cap on births | **violation** — pure bookkeeping, no natural referent |
| newborn IDs 10001..10016, runner allocation bound 32 | population capped at 32 by ID legibility | **violation** — same cap expressed as a range |
| capital yield (`CapacityYieldPerPoint = 1` food/hour/point, per actor) | food created with no land, material, or upkeep input | **violation in substance** — makes carrying capacity fake even without an explicit cap |
| `CapacityKMax = 8`, `CapacityGranaryMax = 8`, 8 slots/patch, bag 1 | finite storage/space | likely legitimate (space/storage), but each needs an explicit natural referent or replacement |
| aging lifetime 96–143 h | biological mortality, trait-independent | legitimate (natural mortality), keep unless the designer finds a better referent |
| `horizon = 168 h`, fixture sizes, sampling schedule | experiment parameters | legitimate under P15, but see below |
| kernel ceilings (`MaxHistoryPatches` 1<<18, `MaxHistoryValueNodes` 1<<19, `MaxHistoryBytes` 128 MiB), scheduler capacities | compute/storage limits | legitimate as *compute* limits, but must be instrumented, fail-loud, and analyzed case by case (P15) |
| frozen bands/falsification criteria in tickets 13/14 | experiment gates | legitimate; v4 evidence is preserved as historical record and is not re-tuned |

## Objective

Design (not implement) v5 such that population, production, and wealth are bounded **only** by modeled nature and by our compute, and such that hitting a compute ceiling is a diagnosable, reportable event rather than a fixture bound in disguise:

- remove the global birth cap and the ID-range cap; entity identity comes from the kernel allocator with no population ceiling other than compute;
- give production a real natural input so that carrying capacity is a consequence of the model (e.g. capital occupying land/slots, material or upkeep cost per unit of yield, patch regeneration taking time, or a combination) — with an explicit, integer-exact accounting story for every unit of food created;
- make reproduction resource-determined endogenously: the existing surplus gate, consent, kin rules, and costs already gate births, so with the caps gone the population trajectory must be decided by ecology (and decline/extinction must remain reachable);
- instrument the compute limits: the reporter must identify the binding ceiling, the value reached, and the hour, and the ticket must state the P15 case-by-case analysis rule for ceiling hits;
- specify what "we outgrew the harness" looks like versus "our design has a problem", with concrete diagnostics that distinguish them.

## Design questions (must be answered with exact numbers)

1. What natural input bounds production? Land/slots occupied by capital? Material units consumed? Upkeep food per point per hour? Patch regeneration and depletion? Each option's conservation identity must be exact.
2. What bounds per-actor capital and storage, and what is their natural referent (space, structure, spoilage)? If no referent exists, what replaces them?
3. How does the reproduction mechanism become cap-free while staying deterministic and replayable: unbounded entity IDs, unbounded roster, unbounded binding table, per-hour birth throughput limits (if any) with natural referents?
4. What is the expected qualitative behavior: exponential growth into the compute ceiling, logistic saturation at carrying capacity, oscillation, or collapse? State the honest expectation and the falsification criteria for each.
5. Compute instrumentation: which counters (actors, rows, events, history bytes, journal bytes, checkpoint bytes, fibers, wakes, wall/CPU per simulated hour) and which thresholds constitute "ceiling hit"; what diagnostic output distinguishes a successful overgrowth from a defect.
6. Does wealth need to become transferable now (inheritance, trade) or stay deferred to the trade/currency ticket? State which, and why, given that v5 changes what "wealth" bounds.
7. What observation/privacy discipline applies to the new production inputs (who can see land, stock, upkeep)?

## Required designer outputs

Chosen mechanism plus rejected alternatives with reasons; exact frozen constants with bounds and natural referents; clock/phase changes; gate list with numeric thresholds, negative fixtures, and falsification criteria; the compute-ceiling instrumentation contract; experiment matrix from verified neutral checkpoints including at least one cell expected to reach resource saturation and one to reach a compute ceiling; metric definitions (population, production inputs, per-actor-year bytes, CPU per simulated hour); lane board; stop conditions.

## Constraints

- v1–v4 fixtures, policies, journals, bundles and evidence stay byte-frozen and are never re-tuned; v5 is a separate version.
- Determinism, replay, checkpoint/branch discipline unchanged (unbounded rosters and binding tables must remain exact).
- No post-hoc tuning; a null, collapse, or ceiling-hit result is a valid outcome and must be reported verbatim.
- Any fixture bound that does modeling work must be replaced by a modeled constraint (P15).

Need-review: yes
Need-test-cases: yes

## Comments
