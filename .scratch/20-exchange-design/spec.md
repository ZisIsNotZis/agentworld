# Exchange and trade (design, v9)

Status: claimed

## Issue

The operator's arc after reproduction/genetics is exchange and trade, then money, then richer social concepts. Until now that arc had no substrate: the ecology produced no surplus, so there was nothing to exchange. Ticket 18 changed that — surplus is reachable at sub-saturation density, the holders are frequently second-generation newborns, and the v4 consent/privacy machinery now runs organically — and ticket 19 is measuring the phase boundary (where surplus is reliable, who holds it, and for how many hours a surplus holder coexists with an alive actor holding nothing) precisely so this design can choose cells where exchange is *possible* rather than hypothetical.

What exists as tradable value today is deliberately thin: food in a bag (capacity 1) and in a granary (spoils on a 24-hour clock), capital points that are **immovable by construction** (possession-by-identity: only the owner's own intentions may touch a worksite, and tenure follows the owner's slots), labour time (one gather and one build per actor-hour, and the frozen auto-feed rule ties a worker's meal to the granary), and land (8 slots per patch, claimed by build completion). There is no fungible token, no transfer of capital, no contracts, no reputation, and no enforcement. Any exchange design must therefore say what is being traded and why both sides want it — a double coincidence that the current goods barely support.

## Operator direction (recorded)

Sequencing: productive capacity → surplus → reproduction/genetics → exchange/trade and money, with affect/motive richness (selfishness, altruism, opportunism) before reciprocity/credit. Principle P15 governs (limits come from modelled nature or our compute, never arbitrary caps); P10 forbids preinstalling modern institutions as necessities; every frozen gate is declared before its runs and never tuned afterwards, and a null result is a valid outcome reported verbatim.

## Objective

Design (not implement) v9, the minimal exchange mechanism that makes voluntary, consenting, correctly-recorded trade possible on the v8 world, such that:

- the traded goods are real and conserved (every unit accounted, no minting, no hidden subsidy);
- both sides gain for a reason the model can state (heterogeneous endowments or needs that the density measurement will have quantified);
- the recording discipline of the earlier tickets holds at full strength: independent intentions on both sides, no private-state leak, a typed atomic settlement, journal linkage that replay can verify, and rejection rather than silent partial settlement;
- the world stays bounded and starvation/collapse stay reachable (no new positive-feedback loop without a natural bound);
- the mechanism is attributable: the ticket names the single new mechanism and predeclares the band whose outcome would falsify the claim that trade occurred and mattered.

## Design questions (must be answered with exact numbers)

1. **What is traded, and what is the double coincidence?** Weigh and reject with reasons: (a) food-for-labour, i.e. a typed work contract where a food-rich actor pays granary stock for another actor's gathered unit or build labour (a wage in embryo); (b) a plot/tenure market where cultivated slots can be transferred (rent or sale) — noting that this immediately touches the frozen possession-by-identity rule and the all-tenure-holders land identity; (c) a **new durable, transferable good** (tools) that a holder can make and trade for food, giving a genuine two-good economy and a destination for money later; (d) credit/debt with deferred settlement (the operator's later rung, which needs enforcement or reputation and therefore richer motives).
2. **Consent and settlement semantics:** what is the offer/accept protocol in the frozen hourly lattice (which phase instants, how many per actor-hour, how it composes with gather/build/birth), and what makes settlement atomic in one kernel batch with zero-mutation refusal otherwise? Note the lessons already learned: a policy's consent floor can differ from the contract's, a policy refusal must be respected at the policy level, and a partial settlement path corrupts ledgers.
3. **Conservation identities:** the exact extensions (or the proof that no frozen identity changes), including what happens to `birthGranaryPaid`, the granary ledger's `provisioned`/`spoiled` terms, the inventory intake identity `gathered + provisioned = consumed + held + invested`, and the land identity if tenure can move.
4. **Why does either side accept?** State the in-world reading for each rule and each threshold (no constant chosen because it makes trade appear); say what the measurement from ticket 19 must show for the design's premise to hold.
5. **What is deferred, and what would the money rung need?** State explicitly what this ticket does not do (credit, money, prices, institutions) and what evidence a later money design would need (e.g. a fungible good with a stable exchange ratio and repeated trade).
6. **Interaction with reproduction and the phase boundary:** does trade interact with the birth gate, the generational concentration of surplus, or the mouths-vs-slots oscillator? What must stay frozen so the v5–v8 evidence remains valid controls?
7. **Falsifiers:** what result would show that the exchange mechanism did not in fact produce trade (e.g. zero filings of offers, offers only ever accepted by one side, no measurable redistribution), and what would show it distorted the ecology rather than operating within it?

## Required designer outputs

Chosen mechanism with exact constants and in-world readings; rejected alternatives with reasons; the new version identity (world/policy axes) and its honest cost; predeclared bands with numeric thresholds and falsification criteria (including a null-trade band); the experiment matrix and the single-variable ladder, referencing the ticket-19 phase map for cell selection; metric definitions; lane board; stop conditions. If the honest conclusion is that exchange cannot be made meaningful with the goods the v8 world has, say so with the evidence and name what must be added first — do not invent a trade-flavoured mechanism that nothing has a reason to use.

## Constraints

- v1–v8 world bytes, policies, checkpoints and all recorded evidence stay frozen; v9 is a new version, never a re-tune.
- No post-hoc tuning; a null result (no trade occurs, or trade occurs without effect) is valid and must be reportable verbatim.
- Determinism, replay and checkpoint/branch discipline unchanged; compute ceilings stay instrumented with the P15 verdict table.
- Any transferable good must respect the conservation ledger and the privacy discipline (a trader must never see the counterparty's private state beyond what the counter-offer protocol publishes).

Need-review: yes
Need-test-cases: yes

## Comments

- 2026-09-29 — assistant: Opened after the v8 merge (`716ded4`), while ticket 19 measures the density phase boundary and the exchange precondition. The design may be drafted in parallel but its cell selection must reference the ticket-19 phase map once that lands.
