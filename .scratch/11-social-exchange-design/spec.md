# First social interaction mechanism

Status: done

## Issue

Food-flow v1 passes a bounded synthetic survival experiment but isolates actors by home patch and gives each one only gathering/consumption choices. It cannot yet test self-interest, kinship, reciprocity, unequal access, refusal, or social redistribution. Extending population or time without a credible interaction mechanism would be a misleading route toward P0.

## Objective

Select and specify the smallest versioned social interaction slice that adds actor-to-actor information and resource transfer while preserving private knowledge, distinct interests, world authority, numerical projections, replay, checkpoints, and the existing food-flow v1 baseline.

## Acceptance criteria

- Read actual pilot trajectories and identify a social pressure with causal evidence, not an invented story goal.
- Compare at least two materially distinct candidate mechanisms (for example, kin-based sharing versus negotiated exchange) against goal, cost, identification and validation.
- Choose one minimal bounded experiment with predeclared cooperation, refusal, exploitation and scarcity fixtures and explicit numerical observables.
- Specify private/public knowledge boundaries, typed actor intentions, counterpart consent/refusal, transfer conservation, duration/interruption, strategy updates, evidence and checkpoint compatibility.
- Give a multi-seam implementation topology, performance budget and stop gates; do not claim human realism from one positive social trajectory.

Need-review: yes
Need-test-cases: no

## Blockers

No user-only blocker. Read-only design evidence is required before mutation.

## Decision

Choose a separately versioned, addressed one-unit voluntary bag gift between known same-patch actors. A denied gatherer may request; the donor independently accepts or refuses based on its own reserve and directed affinity. Preserve v1/S6, never leak actual requester energy to the donor, and do not require sharing to improve aggregate survival. Reciprocal promises/credit are deferred because a one-commodity immediate gift cannot identify repayment behavior. Compulsory pooling is a later allocation control, not the first social mechanism.

The first experiment is 16 actors, q3 through h24 around the observed h17 bottleneck, plus q8 and q0 controls; seeds 0/7 and workers 1/4. Freeze dyads, policy weights, one-request/one-reply-per-actor-hour budget, cooperation/refusal/false-urgency fixtures, conservation and negative gates before running. Report distributional outcomes and donor cost, not one goodness score. The next feature ticket owns implementation.

## Comments

- 2026-09-28 — assistant: Opened after food-flow pilot `05f199f` integrated to master. The next experiment should test an actual social mechanism rather than increase actor count alone; this is a reversible research sequencing decision.
- 2026-09-28 — assistant: Read-only Designer run `9e5febcd-42fe-4b39-b6f7-5ed3f91f2395` compared kin-directed voluntary gift, reciprocal exchange and compulsory pooling, chose the first from q3 survivor lock-in evidence, and identified v1's never-gathered recipient invariant as requiring a v2-only rule. No code changed. Decision and gates recorded here; successor ticket 12 owns implementation.
