# Social food transfer v2 pilot

Status: claimed

## Issue

Food-flow v1 is a synthetic survival mechanism with isolated actors. In q3, ten actors stop at h17 and the three remaining actors per patch acquire all later production. It cannot test whether voluntarily consenting actors redistribute scarce food under private knowledge and self-interest.

## Objective

Implement a separately versioned 16-actor social-food pilot: an actor denied Gather may send one addressed one-unit request to a known same-patch actor; a donor independently accepts or refuses based on its own state and directed affinity. An accepted gift transfers one held unit with source provenance through the authoritative world; refusal, expiry and invalid attempts are typed evidence without transfer.

## Frozen experimental boundaries and gates

- Preserve food-flow v1 and S6 rules, histories, strategy and checkpoint bytes. Social-v2 schemas, policy, journal and manifest reject v1/S6 bundles; no implicit migration.
- Pin directed synthetic affinity/known-dyad roster and policy weights before the first social run. No genealogy claim. Donor sees its own energy/bag/affinity and the requester's self-reported urgency, **not** actual requester energy or private history.
- At most one request and one reply per actor-hour. Request may expire or be withdrawn. No implicit transfer; recipient willingness and donor consent are separate, validated intentions. Donation is a single atomic one-unit donor-bag−1/recipient-bag+1 transfer with unchanged patch source and zero aggregate food delta.
- Social-v2 can let a never-gathered recipient hold/eat a gift; do not weaken food-flow v1's validator. Record witnessed directed requests/gifts/refusals and numerical assistance fraction `(gifts+1)/(requests+2)` plus signed net-food balance with units/provenance, not moral trust.
- Predeclare cooperation, low-reserve refusal, false-urgency and duplicate/expired/unauthorized-transfer negative fixtures. A false claim need not succeed. Measure privileged claim-versus-truth only in experiment audit; never leak truth to donor.
- Use q3 seed 0 and 7 through at least h24, plus q8 through h24 and q0 through h12, workers 1/4. Compare social enabled/disabled branches from a verified social-v2 checkpoint before social action. A worse survival distribution is a valid result; no post hoc tuning for positive outcomes.
- At every boundary reconcile produced = consumed + held + stock and initial energy + consumed = energy + basal spent + cap loss; gift changes neither total. Record donor/recipient Eat, energy/hunger, request/refusal/expiry, per-patch consumption dispersion, actor 1/8/16 and low/mid/high-ID trajectories, replay, checkpoint continuation and compute/storage/token cost.
- Stop on unauthorized or duplicate transfer, consent bypass, resource creation, private-state leak, fixed ID priority, worker divergence, broken event/journal linkage or checkpoint incompatibility. Reaching h24 alone is not acceptance.

## Frozen v2 fixture and clock

The directed known-dyad rings are `1→2→3→4→5→6→7→8→1` and `9→10→11→12→13→14→15→16→9`. Directed affinity is 3 on `4→5`, `8→1`, `12→13`, `16→9`, and 1 on the other listed edges; these are synthetic ties, not genealogy. A living empty-bag actor denied Gather this hour and privately at energy ≤8 may request its outgoing target once, reporting urgency 2. An explicitly injected false-urgency test may change only the submitted claim, never private reserve or recipient identity.

A living same-patch donor with one held unit and energy ≥4 replies at most once/hour. Its consent score is `affinity + 2×(reported urgency==2) − 1 (lost own meal) − 1 (own energy≤6) − 1 (own hunger≥10)`; accept iff score ≥3, otherwise refuse. The donor sees its own state, directed affinity and reported claim, not requester energy. A low-reserve donor energy 3 must refuse. Do not require any gift or survival improvement until a run demonstrates it.

At each hour Gather completes at h+20m+1µs, then request at +1µs, reply/gift at +2µs, finalization and Eat/Rest start at +3µs, and meals/rest complete ten minutes later. Pending requests expire before the next hour. Enabled and disabled v2 share this meal delay; v1 is a separate baseline. Seed and verify a neutral v2 checkpoint at h0 pulse before actor claims; enable/disable social behavior only by an explicit recorded branch intervention, not altered serialized bytes. q3 seeds 0/7 run to at least h24; q8 seeds 0/7 to h24; q0 seeds 0/7 to h12, each with workers 1/4. The observed q3 seed-0 h0 actor 1 successful Gather and actor 8 denied Gather provide an unforced opportunity; an h8 high-affinity request is a hypothesis, not a preordained gift.

v2 bag state retains source patch, original gatherer and last donor. A never-gathered recipient may hold and consume a gift only under v2 validation. Request state is typed (`idle/pending/accepted/refused/expired/withdrawn`); accepted gift atomically changes request and both bags with zero net held-food change. Per-patch `produced = consumed + held + stock`, global `176 + consumed = energy + basal spent + cap loss`; record claim/truth separately in privileged audit. Project directed witnessed assistance `(gifts+1)/(requests+2)` and signed received−given units, with provenance; neither is moral trust.

## Lane board

This is multi-seam. Sequential writers in one feature branch own disjoint source paths; each hands off an independently validated contract and exits before the next writer mutates. The parent owns synthesis, integration and final acceptance.

| Lane | Exclusive source | Gate |
|---|---|---|
| contract | new `internal/world/socialfood_contract*.go` | Typed v2 schema, provenance, request status, pure transfer, conservation and incompatibility tests |
| policy | new `internal/strategy/socialfood*.go` | Bounded addressed request and donor decision, actor-only observations, refusal/privacy tests |
| runner | new `internal/world/socialfood.go`, runner tests | Timed request/reply phases, independent consent, deterministic conflict, rollback and q3/q8/q0 fixtures |
| persistence | new `internal/world/socialfood_journal*.go`, `socialfood_checkpoint*.go` | Accepted/rejected evidence, pending-request topology, subprocess restore and cross-version rejection |
| experiment | `cmd/socialfood/**`, scoped docs/ticket | Predeclared branch comparisons, trajectories, metrics, costs and reproducible command |
| review | entire integrated scope, read-only | Evidence-backed findings and gate disposition |

## Blockers

No user-only blocker. Read-only design run `0730d2a2-3e82-4b46-a8df-1997fb93d384` fixed the dyads, policy weights, phase clock, experimental branches and v2-only state before mutation. Implementation must preserve this contract or record a discovered contradiction before running experiments.

Need-review: yes
Need-test-cases: yes

## Comments

- 2026-09-28 — assistant: Opened from design decision 11 at master `d2343b9`. This is a small synthetic social mechanism, not a historical-humanity validity claim or P0 scale completion.
- 2026-09-28 — assistant: Designer inspected q3 opportunity and pinned directed affinity roster, private request/consent score, phase schedule, neutral-v2 checkpoint branch comparison and metrics before any social run. No v1/S6 changes authorized.
- 2026-09-28 — assistant: Domain seam added disjoint v2 schemas, addressed request, bag provenance and checked atomic Gift. First fresh review `fe615da0-380e-49b4-a9cb-928139c2fb24` found score-inconsistent success fixture, answered-pending request invariant gap, metric provenance gap, and demonstrated raw-kernel consent bypass. Worker corrected checked score/refusal, outstanding request validation, assistance version/unit and net-from-received-minus-given; kept a valid raw-patch counterexample. Fresh re-review `5b6c7943-58a3-43b7-ad29-1f3772f37695` found no issue (OK with notes). Parent ran PATH-prefixed `GOMAXPROCS=4 timeout 180s make check` and `git diff --check` successfully. Runner must retain proposal authority and independently validate donor choice; journal must validate request/consent/transfer linkage.
