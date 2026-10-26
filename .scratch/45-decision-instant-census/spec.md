# Ticket 45 — Decision-instant census: births are lost to addressee choice, not to the gate

- Record kind: zero-spend census over recorded no-AI runs (`-arms A0`); no world change, no policy
  change, no model call. Part of workset `W1_world_runs`.
- Author: agent (product owner), 2026-10-26.
- Question (ticket 44 §3c): the joint birth gate is held by two actors for 107 of 144 hours under
  `natural/5`, yet the same policy files 260 requests, is refused `consent-own` 135 times and births 3.
  Is the cap (i) a decision-instant state that the hourly boundary flatters, or (ii) the policy's own
  consent rule being stricter than the state it is given?
- Code: [`census.go`](census.go), read-only; reproduction commands in its header.

## 1. What was measured

For every `birth-reply` decision the census records the consenter's full gate triple
(`energy`, `granary`, `capital`) at the two adjacent hourly checkpoints and what the consenter did
inside that hour (an accepted `build`, a `paired-meal`), then counts, at the reply hour, the
**alternative mates**: co-located, alive, non-kin, unrelated actors other than the actual addressee who
hold the joint gate.

The bracket is monotone in the frozen rules, so a boundary failure can be read as a genuine instant
failure for two of the three components: the `h+1` pulse grants regen (reply granary ≤ boundary
granary) and wears capital (reply capital ≥ boundary capital); only energy moves the other way (basal
cost is paid at the pulse), so an energy-only boundary failure is not decisive.

## 2. Result

Cell `f10-s0`, `H = 144`, one replicate (the control is deterministic):

| policy | accepts | `consent-own` refusals | refusals with ≥1 gate-holding alternative mate | refusals with none | joint-gate hours (world floor) | alive at H |
|---|---|---|---|---|---|---|
| `natural/7` (default) | 3 | 135 | **133** | 2 | 1 | 1 |
| `natural/5` | 4 | 135 | **132** | 3 | 107 | 1 |
| `natural/9` | 2 | 1 | 1 | 0 | 125 | 1 |

Every refusal in every policy is classified `explained-end`: the consenter's own state fails the gate
at the boundary, so the refusal is not a threshold artefact. Under `natural/5` the 135 refusals come
from **two consenters only**, each with `granary = 0, energy = 2` at both boundaries, over 135
distinct hours — the proposers keep asking the same impoverished mate.

**The world offers the option and the policy does not take it.** In 133 of 135 refusals under the
default policy (132/135 under `natural/5`) exactly one *other* co-located, non-kin, unrelated actor held
the full gate at the same hour, and the policy asked the mate it cannot have instead. The three
accepted replies under `natural/7` all occurred in hours where no alternative existed (`f10-s0` h4–h5,
h7) — i.e. when the policy asked the only available mate.

So the answer to (i) vs (ii) is **neither**: the cap is **addressee selection**. The consent gate, the
economy and the decision instants are not the binder at this point; the frozen script's addressee rule
is. This also explains ticket 44's puzzle in the same stroke: `natural/5`'s 107 gate-hours belong to
actors who are not the ones being asked.

The addressee rule is policy-level, not a world law, so the smallest change W1 can make is a **new
policy version with no world change** — the branch ticket 44 called "policy floor", in its true form:
choose the mate that can actually consent.

## 3. The change this points to (not implemented in this ticket)

`natural/10`: `natural/7` verbatim except the addressee rule, which must pick a mate able to consent.
Two candidates, cheapest first:

1. **Rotation without new information.** Sort the co-located non-kin mates and pick
   `index = hour % len(mates)`. Needs no new observation field, no privacy change, no world change; it
   converts the current "always ask the same mate" into one birth opportunity per mate per cycle, i.e.
   roughly `len(mates)-fold` more chances.
2. **Gate-aware choice.** Requires exposing a co-located mate's state (capital is public on the
   patch; energy is private body state), i.e. an observation-contract change. Strictly better if the
   privacy boundary admits a coarse "fed" signal, and it is the only version that *explains* the
   choice to a reader.

Both keep the proposer's own obligations (it still holds the gate and pays 2 granary + 2 energy), the
age-death clock and every world rule. Old policy refs stay byte-frozen.

## 4. What this does not establish

- One cell (`f10-s0`), one replicate each, three policies. The 128-cell grid must be re-run after the
  change (`$0`), and `alive_end` is 1 in every policy measured here, so the fix's effect on survival is
  unmeasured.
- The alternative-mate count is read at the `h+1` boundary, the same bracket as the refusal; the
  direction of its error is to understate the alternatives (a mate can gain the gate inside the hour),
  never to invent one.
- Rotation may still fail: the proposer must hold its own gate at the reply instant *after* paying, and
  ticket 44 measured the components alternating rather than co-holding. That is exactly what the
  post-change grid measures.
- Nothing here bounds a different substrate, horizon or food supply.
