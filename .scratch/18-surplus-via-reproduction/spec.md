# Surplus via reproduction or land economics (design, v8)

Status: claimed

## Issue

Ticket 17 (v7) tested the last store-side lever and returned the predeclared honest null. With both control gates green (world v6 reproduces the frozen v5 trajectories field-for-field with zero provisions on the control arms), the `natural/7` arm provisioned 288 units and committed 320 builds per A cell — the land at maximum throughput (16 slots × 20 tenure cycles) — yet **V7-A2 stayed OUT under both the per-actor and union readings** and extinction was unchanged at h141/h143. The self-falsifier did not fire: `natural/7b` (the same banking floors with provisioning disabled) also leaves A2 OUT while banking to a run-max granary of 96, so M2 stands and v7 is not reducible to a policy-only ticket.

The diagnosed mechanism, now measured three times over (tickets 15, 16, 17):

- **M1 energy ratchet:** metabolism drains exactly one energy per hour at the neutral locus, every meal returns exactly one, gather+eat is net zero, a build hour permanently spends one, and **the newborn's 8-unit endowment is the only exogenous energy channel in the world**.
- **M2 wealth cannot fund capital:** the granary ledger is written only by spoilage/grant/eat-stored/birth-payment; ticket 17 added exactly one missing conversion (provision, stock→bag) and the economy promptly saturated as a **reinvestment machine** — every cultivated unit is re-provisioned (SF′ counts it as a withdrawal), eaten, or spoiled, so no standing stock ever accumulates.
- **M3 k=1 is a zero-margin household:** flat one-unit-per-tenured-slot grants equal one-unit-per-hour metabolism, so SF ≡ 0 at k=1 and the mandatory climb to k≥2 is what the economy prices.

Therefore, as the frozen design's Q7 stated, surplus reachability now requires changing **reproduction or land economics** — not the store. This ticket is that change.

## Operator direction (recorded)

Sequencing: productive capacity → surplus → reproduction/genetics → affect/motives → exchange/credit, with exchange and money next in the arc; principle P15 (limits come from modelled nature or our compute, never arbitrary design caps); frozen gates are never re-tuned post hoc, and a null result is a valid outcome reported verbatim. The operator has repeatedly asked for the ecology to be decided by natural resources rather than by bookkeeping. This ticket must therefore make the surplus indicator reachable **by a modelled mechanism with a defensible in-world reading**, not by loosening a band.

## Objective

Design (not implement) v8 such that an actor population starting from zero capital and empty granaries can reach the frozen surplus indicator (`k≥2 ∧ granary≥1` with positive trailing-24 h surplus flow sustained ≥12 consecutive hours, at the per-actor reading) through a change to **reproduction or land economics**, while:

- starvation, collapse and extinction remain reachable and no hidden subsidy exists (the v7 diet of verifiable identities must keep holding: G1 with the provisioned term, G2a′, G2b/land, G3, plus whatever new identity the chosen mechanism requires);
- land/regeneration/spoilage/allocator/ceiling mechanics stay the binding production constraint where the design keeps them;
- the v5/v6/v7 worlds, policies, evidence and verdicts stay byte-frozen as controls, and every earlier verdict stands (v5 A-band falsified, v6 FALSIFIED, v7 FAILED);
- the change is minimal and attributable: the ticket must name the single mechanism change expected to flip A2 and predeclare the band whose outcome would falsify that expectation.

## Design questions (must be answered with exact numbers)

1. Which lever, with its in-world reading? Candidates to weigh and reject with reasons:
   (a) **reproduction reachability** — make the birth gate attainable so generational turnover becomes the energy channel (the newborn endowment is the only exogenous energy, so climbers would be *born* into footholds, inverting the causal order of the surplus problem);
   (b) **land economics** — fewer founders relative to slots, patch growth, higher R, or a yield shape that leaves a margin above metabolism per actor;
   (c) **intergenerational transfer** — granary and/or capital inheritance at death (the institutional question deliberately deferred since ticket 13), which would let a lineage accumulate across generations where an individual cannot;
   (d) any combination, if and only if the design states why a single lever cannot suffice.
2. What exactly does the chosen lever change, and what is its natural referent (soil, labour, kinship, structure)? No declared threshold without a derivation.
3. How do the conservation identities extend (or stay identical) — every unit of food, energy and capital accounted for, including inheritance if chosen?
4. What keeps the ecology bounded: does the change create a new positive-feedback loop, and what stops it (land, wear, disease, senescence, spoilage)?
5. What are the predeclared bands for v8 (surplus reachability at the per-actor reading, capital persistence, birth reachability, non-universality, and what would falsify the diagnosis that the store was not the binding constraint)?
6. Experiment matrix: which arms are needed to isolate the lever (v5/v6/v7 evidence as controls; new world/policy versions as required), including the cultivation ablation, the fixture-cells and the compute-ceiling cell, and the specific single-variable comparison.
7. If the honest conclusion is that surplus cannot be made reachable from a zero-capital start under any of these levers without abandoning a frozen identity or a P15 boundary, say so with the evidence instead of forcing a fix.

## Required designer outputs

Chosen lever(s) with exact constants and in-world readings; rejected alternatives with reasons; the new version's identity (world/policy axes) and the honest cost of any new axis; predeclared gates with numeric thresholds and falsification criteria; the experiment matrix and the single-variable ladder; metric definitions; lane board; stop conditions; and an explicit statement of what happens to the v7 provisioning rule and the natural/5|6|7|7b policy family.

## Constraints

- v1–v7 world bytes, policies, checkpoints and all recorded evidence stay frozen; v8 is a new version (mechanism and/or policy) and never re-tunes a recorded number.
- No post-hoc tuning; a null result must be reportable verbatim with the diagnosis re-examined rather than the band relaxed.
- Determinism, replay and checkpoint/branch discipline unchanged; the compute ceilings stay instrumented with the P15 verdict table.
- Any change to reproduction must respect the v4 consent/privacy discipline (independent donor/consenter intentions, no leaked truth, derived kinship only from the reproduction graph).

Need-review: yes
Need-test-cases: yes

## Comments

- 2026-09-29 — assistant: Opened from ticket 17's FAILED verdict and its recorded Q7 escalation, while ticket 17's parent-owned F cells were still running (the F band concerns the compute-ceiling instrumentation, not this ticket's conclusion, which rests on the A-cell evidence). All v5/v6/v7 verdicts stand as frozen controls.
