# Surplus reachability under the frozen v5 mechanism (design, v7)

Status: claimed

## Issue

Ticket 15 built the resource-determined world; ticket 16 proved (with a green control gate and byte-identical v5-arm reproductions) that the poverty trap is escapable at the *build* decision — the v6 policy `Build iff k+wip<8 ∧ E≥3 ∧ (granary≥1 ∨ wip≥1 ∨ E≥4)` does build early and more (first build at h0, 128 committed builds versus v5's 112, 384 cultivated grants versus 288) — **but surplus is still unreachable in every organic cell**: the predeclared A2 band is OUT (first_surplus_bearing_hour −1, zero surplus-bearing actor-hours), the granary never accumulates (run-max aggregate 16 = one unit per actor), 368 of 384 granted units are re-drawn as stored meals, 16 spoil, and the net surplus flow over the trailing 24 h is 0. The verdict is FALSIFIED, alongside the still-standing falsified v5 A-band.

The binding mechanism is now identified and it is **not** the policy: the runner's frozen auto-feed ("workhouse") rule draws a stored meal from the granary whenever `energy < 5 ∧ granary ≥ 1 ∧ LastStoredMealHour < hour` — `naturalPairIntentTLow = 5`. Any actor working in the fed band `E ∈ [3,4]` therefore eats exactly one granary unit per hour automatically, so at k=1 the grant (1/h) is cancelled by the draw (1/h) and capital income can never compound; only the ticket-16 **E cells** (an initial founder-tenure foothold) reached sustained surplus at all (first sustained h31, run-max granary 87/70). The v6 policy has no way to decline that meal: the feed is a world rule, not a choice.

So the honest next step is a **mechanism** change (v7), not another behaviour tweak: under the v5 mechanism no policy can make surplus reachable from a zero-capital start, and the ticket-16 design itself recorded this "granary flicker" as a live risk that has now been observed as the binding constraint.

## Operator direction (recorded)

Sequencing: productive capacity → surplus → reproduction/genetics → affect/motives → exchange/credit, with exchange/money next in the arc; and principle P15 (limits come from modelled nature or our compute, never arbitrary design caps). Exchange and money are meaningless while the organic ecology cannot accumulate anything, so surplus reachability must be fixed first — with the v5/v6 evidence as the control and the auto-feed rule named as the diagnosed defect rather than tuned around.

## Objective

Design (not implement) v7 such that an actor starting with zero capital and no granary can, by its own choices, reach sustained surplus (the frozen indicator `k≥2 ∧ granary≥1` with positive trailing-24h surplus flow sustained ≥12 h) — while keeping:

- starvation and collapse reachable (no hidden subsidy, no cap on nature);
- the land/regeneration/spoilage/allocator/ceiling mechanics and every recorded identity intact where possible, with any change named exactly and justified in-world;
- the v5 and v6 evidence byte-frozen as controls, and the v6 falsified verdict standing;
- the change minimal enough to be attributable: the ticket must state which single mechanism change is expected to flip A2, and predeclare the band whose outcome would falsify that expectation.

## Design questions (must be answered with exact numbers)

1. Which mechanism change makes surplus reachable from zero capital, with its in-world reading? Candidates to weigh and reject with reasons: (a) the auto-feed threshold (e.g. draw from the granary only below E=2, so a worker can hold E∈[3,4] without draining the store); (b) making the draw a *choice* the policy can decline (turning the workhouse rule into a typed intention), which is a mechanism+policy change; (c) capital income shape (grant per point per hour versus a yield that grows with contiguous tenure); (d) wear period or build cost; (e) spoilage period.
2. How does the chosen change keep over-investment fatal and the ecology bounded by land (the 8 slots/patch and the regeneration budget R must remain the binding production constraint)?
3. Does the change require a new world version (v6 world) or can it be an option on the v5 world? State the honesty cost of each, given P15 and the byte-frozen evidence.
4. What are the predeclared bands for v7 (surplus reachability, capital persistence, population/birth reachability, non-universality, and what would falsify the diagnosis that the auto-feed rule was the binding constraint)?
5. Experiment matrix: v5-policy and v6-policy arms on the v7 world plus the v7 policy (if the design keeps a policy change), the cultivation ablation, and the ceiling cell; with the v5 and v6 recorded evidence as controls; plus the specific single-variable comparison that isolates the mechanism change.
6. What must stay identical so the v5/v6 evidence remains a valid control, and what is the exact diff surface?
7. If the honest conclusion is that surplus cannot be made reachable without also changing reproduction/land economics, say so with the evidence rather than forcing a fix.

## Required designer outputs

Chosen change with its in-world reading and exact constants; rejected alternatives with reasons; the new version's identity (world/policy refs); predeclared gates with numeric thresholds and falsification criteria; the experiment matrix; metric definitions; lane board; stop conditions; and an explicit statement of whether the v6 behaviour rule is retained, replaced, or made optional.

## Constraints

- v1–v6 world bytes, policies, checkpoints and all recorded evidence stay frozen; v7 is a new version (or a new world option) and never re-tunes a recorded number.
- No post-hoc tuning; a null result (surplus still unreachable) is valid and must be reportable verbatim, with the diagnosis re-examined rather than the band relaxed.
- Determinism, replay and checkpoint/branch discipline unchanged; compute ceilings stay instrumented with the P15 verdict table.

Need-review: yes
Need-test-cases: yes

## Comments
