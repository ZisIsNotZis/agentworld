# Ticket 51 — mechanism v11: the yield axis scales the wild land too

- Record kind: one mechanism version with a single rule change, plus its calibration. Zero spend.
  Workset `W1_world_runs`.

## 1. What tickets 49–50 established

- v8 + `natural/13` fixed fertility (births 2 → 12 on `f10-s0`) and exposed starvation.
- v9 added the `Yield` axis, but it scales only the **cultivated** grant (rule 508). Raising it to 8
  poured food into a few granaries (`Granted 8224`, `Spoiled 1913` over 144 h) without feeding anyone:
  the starvation deaths are landless actors (`capital 0, granary 0`) on a patch whose **wild** stock is
  5–7 units for 6–10 mouths — under one unit per actor-hour.
- v10 (round-robin founder placement) left the same arithmetic: at yield 1 and 2 the cell still starves
  (20 and 18 starvation deaths).

So food is not short in total — 1492 wild + 551 granted units were produced in 144 h against 1430
consumed — it is short **where the landless stand**, because the only food a landless actor can reach is
the wild fill, and that is capped at one unit per slot per hour no matter how fertile the land is.

## 2. The declared change (mechanism v11)

`NaturalMechanismV11` = `NaturalMechanismV10` rules plus one clause: **the `Yield` axis scales the wild
fill as well as the cultivated grant** — each wild (uncultivated) slot refills `Yield` granary units per
patch-hour instead of 1, and nothing else about wild fill, spoilage, claiming, gathering, eating,
cultivation or the owners' grants changes. `Yield = 1` is v10 verbatim, so the axis stays a single
declared number with 1 as the neutral value; the mechanism version exists only because the arithmetic
differs from v10 when `Yield > 1`.

Rationale, declared: land fertility is a property of the land, not of whether someone has built on it.
v9's version made fertility conditional on cultivation, which is why it could not feed the landless.

## 3. The calibration

1. `f10-s0`, `H = 144`, v11 + `natural/14`, `Yield ∈ {1,2,3,4}`, reporting `alive_end`, births,
   starvation, age, gate-hours, plus the `NaturalBalance` line.
2. Choose the **smallest** yield meeting (a) `alive_end ≥ 10`, (b) starvation deaths = 0, (c) ≥ 1 birth,
   (d) gate-hours ≥ 24; reject overshoot (gate-hours = 144 everywhere).
3. Run the 128-cell grid (16 founders × 8 seeds) at the chosen yield and report the pass count under the
   same four clauses.
4. Append the numbers to this file.

## 4. Non-goals

- No change to slots per patch, patch count, founder placement, kin, consent, births, the age clock, the
  basal cost, spoilage, meal gain or the per-actor granary.
- No new action or phase instant; no population cap.
- No live model call; no `cmd/vs2` change; nothing in `docs/`.
