# Ticket 47 — `natural/12`: the public kin walk in the proposer's observation

- Record kind: one additive observation field plus one new frozen policy ref; declared before the
  measurement runs. Zero live spend. Part of workset `W1_world_runs`.
- Author: agent (product owner), 2026-10-26.
- Input: ticket 46 (`.scratch/46-natural11-addressee/spec.md`) — the addressee rotation removed the
  "always the same impoverished mate" pattern (refusals 135 → 99, gate-hours 1 → 46) and produced 50
  **`kin`** refusals, because the proposer's observation carries only same-patch mate **IDs** while the
  reply side already receives the kin relation (`KinRelated`, `KinDistance` — the code's own comment
  calls it *the PUBLIC kin walk*).

## 1. The declared change

1. **Observation (additive, public only).** `NaturalRequestObservation` gains
   `MateViews []NaturalMateView` with `NaturalMateView{ID, KinRelated, KinDistance}`, one entry per
   `PatchMates` entry, in the same ascending order, populated by the world's
   `birthRequestDecision` from `NaturalKinDistance(lookup, proposer, mate)` — the same walk the reply
   side uses. No private state is exposed: energy and granary stay out; only the walk the design already
   declares public.
2. **Policy `natural/12`** (format 12, ref `{natural,12}`, budget 32/32): `natural/11` verbatim except
   the addressee, which rotates over the **non-kin** mates only,
   `addressee = nonKin[hour % len(nonKin)]`; when no non-kin mate exists the policy waits instead of
   filing a request the world must refuse.
3. **Validation.** `natural/12` fails closed (`ErrInvalidNaturalObservation`) unless `MateViews` matches
   `PatchMates` entry for entry (same length, same IDs, distances `1..naturalKinWalkLimit` when related,
   `0` when not). The frozen refs 5/6/7/7b/9/11 ignore the new field, so their bytes and behaviour are
   unchanged.
4. Wiring is the same pattern as ticket 46: registry, mechanism legality (v6 and v7), the closed
   checkpoint/journal format sets, the world dispatch, and the pinned observation field-set tests
   (extended deliberately with a comment naming this ticket, never deleted).

## 2. The declared measurement

Same census (`census.go`, extended with the `natural/12` case), cell `f10-s0`, `H = 144`, one replicate,
zero model calls, three refs side by side:

| ref | what it tests |
|---|---|
| `natural/7` | the frozen control |
| `natural/11` | rotation, kin-blind (ticket 46) |
| `natural/12` | rotation over the public non-kin walk |

Pre-declared reading, in this order:

- **`natural/12` beats `natural/11`** on accepted replies with zero `kin` refusals: the kin walk is the
  binder and this ticket fixed it.
- **`natural/12` ties `natural/11`** on accepted replies while `kin` refusals drop to zero: the addressee
  information was not the remaining cap; the consent state at the decision instant is, and the next
  lever is a world rule (household gate, ticket 44 §5) that needs the operator's word.
- **`natural/12` loses** to `natural/11`, or `alive_at_H` falls: the kin filter removes mates whose
  `kin` refusal was the only way a request could still reach a consenter that the rotation would
  otherwise have skipped — investigate before any further change.

No verdict is claimed beyond this cell; the 128-cell grid is a separate step after the reading.

## 3. Non-goals

- No world rule change: the consent conjunction, the kin depth (`naturalKinProhibitedDepth = 2`), the
  household-gate design, the age clock and the food supply all stay as they are.
- No private state in the proposer's observation.
- No change to `cmd/vs2`, the model contract (`natural-claim/v4`), or any recorded ref's behaviour.
