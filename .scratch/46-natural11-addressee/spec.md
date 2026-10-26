# Ticket 46 — `natural/11`: the addressee rotation, and what its failure measures

- Record kind: one new frozen policy ref plus its measurement on the declared cell; zero live spend.
  Part of workset `W1_world_runs`; the change ticket 45 pointed at.
- Author: agent (product owner), 2026-10-26.
- Input: ticket 45 (`.scratch/45-decision-instant-census/spec.md`) — on `f10-s0` 133 of 135
  `consent-own` refusals were hours in which exactly one other co-located non-kin actor held the full
  gate, while the frozen policy asked the mate it could not have.

## 1. The change

`natural/11` (format 11, ref `{natural,11}`, budget 32/32) is `natural/7` verbatim except the
birth-request addressee: the frozen successor rule (lowest same-patch mate ID above the actor, else
`mates[0]`) becomes the deterministic rotation `mates[hour % len(mates)]` over the ascending same-patch
roster. No new observation field, no floor changed, no world rule changed, every earlier ref
byte-frozen. Ref 10 is skipped because the repository's own tests pin `{natural,10}` as unknown.

Wiring: `internal/strategy/natural11.go`, the registry in `natural6.go`, the policy cascade seam in
`natural_cognitive.go` (the addressee becomes a declared rule parameter, the successor rule unchanged
for 5/6/7/7b/9), the dispatch and mechanism legality in `internal/world/natural.go`, and the closed
format/ref sets in `internal/world/natural_checkpoint.go` and `natural_journal.go`.

## 2. Measurement (cell `f10-s0`, `H = 144`, one replicate, zero model calls)

Same census as ticket 45 (`census.go` here, `-policy natural/11` against `-policy natural/7`):

| | `natural/7` | `natural/11` |
|---|---|---|
| accepted replies | 3 | 4 |
| `consent-own` refusals | 135 | 99 |
| **`kin` refusals** | 0 | **50** |
| other refusals | 0 | 0 |
| hours with ≥2 actors holding the gate | 1 | 46 |
| alive at `H` | 1 | 1 |

## 3. What it establishes — and the next binder

The rotation does what it was designed to do and is still not enough. It stops the "always the same
impoverished mate" pattern (refusals 135 → 99, gate-hours 1 → 46), and it buys **one** extra birth,
because rotation is blind in a second, independent way: **50 of its 150 filings are refused as `kin`**.
The proposer's observation carries only the same-patch *IDs*; kinship is not in it, so a rotating rule
asks its own children and siblings and the world refuses them. Under the frozen successor rule the same
blindness existed but never fired, because that rule's fixed target happened to be non-kin.

So the binder has moved from "which mate does the policy pick" to **"what can the proposer see"**: the
policy cannot choose a valid partner because the observation does not carry the public kin walk, even
though the reply side already receives it (`KinRelated`, `KinDistance` — the code's own comment calls
it *the PUBLIC kin walk*) and the patch geometry makes capital public.

## 4. The lever this points to (next ticket, not this one)

Extend the proposer's observation with the **public** fields the reply side already uses — per
same-patch mate: the kin relation/distance, and the public worksite capital — under a new, additive
observation/policy contract version (a documented claim-contract bump, since the model seam encodes the
same observation). Then the addressee rule can pick a non-kin mate that can actually consent, and the
measurement repeats.

What this ticket deliberately does not do: change the world's kin rule (`naturalKinProhibitedDepth = 2`
in a 10–16 actor population is close to sterility by design), the consent gate, or the household-gate
design of ticket 44 §5. Those are world rules and need the operator's word; the observation extension
is not.

## 5. Limits

- One cell, one replicate, two refs. The 128-cell grid has not been re-run on `natural/11` and is not
  claimed.
- `alive_at_H` is 1 under both refs, so the rotation's effect on survival is unmeasured; the gate-hour
  jump (1 → 46) is a property of a changed trajectory, not a claim about viability.
- The kin count is a lower bound on the information defect: the census counts refusals, not the hours
  in which a non-kin gate-holding mate existed but was not asked.
