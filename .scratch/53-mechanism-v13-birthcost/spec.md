# Ticket 53 — mechanism v13: a declared birth price, and the two-knob calibration

- Record kind: one mechanism version adding a declared price axis, plus the calibration that sets the
  world's two economy parameters. Zero spend. Workset `W1_world_runs`.

## 1. Where tickets 49–52 leave the world

- v8 + `natural/13`: fertility fixed (births 2 → 12), starvation appears.
- v9/v10/v11 (land yield, founder placement, wild yield): more food, still starvation — the loop was a
  treadmill (one unit in, one energy out, one energy per hour of life).
- v12 (one unit = two hours of life): the treadmill is fixed and the world becomes **too fertile**:
  `f10-s0` births 37–63 against a land capacity of about 16 slots, so 25–46 starvation deaths against
  4–14 age deaths. Population booms past the land and dies back hungry — the operator's "desperate
  death" case, merely with the opposite cause.

The two knobs that now decide the world are both prices: **how fertile the land is** (`Yield`, v9/v11)
and **what a birth costs**. The land axis exists; the birth price is hard-coded (2 granary + 2 energy).

## 2. The declared change (mechanism v13)

`NaturalMechanismV13` = `NaturalMechanismV12` plus one declared axis:

- `BirthCost int64` in `NaturalOptions`/`RestoreOptions` — the granary units **and** the energy units paid
  by the proposer for one birth (the existing rule pays 2 and 2). `0` means the v12 default of 2;
  admitted range `0..8`. It is world identity end to end (journal config, checkpoint manifest,
  mismatch rejection) exactly like `R` and `Yield`, and only v13 admits a non-default value.
- Everything else v12 verbatim; the energy ledger identity must keep balancing at every cost.

## 3. The calibration

1. Grid over `Yield ∈ {1,2}` × `BirthCost ∈ {2,4,6,8}` at `f10-s0`, `H = 144`, v13 + `natural/14`,
   reporting `alive_end`, births, starvation, age, gate-hours and the `Balance` line.
2. Choose the **smallest** `(Yield, BirthCost)` pair meeting the amended `W1` criterion: `alive_end ≥ 10`,
   **age deaths ≥ starvation deaths**, births ≥ 1, gate-hours ≥ 24, and not overshot (gate-hours not 144
   everywhere). Prefer the smallest yield, then the smallest cost.
3. Run the 128-cell grid at the chosen pair; report the pass count under the same clauses.
4. Append `## 5. Results` with the grid table, the chosen world parameters, and the grid pass count.

## 4. Non-goals

- No change to basal, bag, gather, build, kin, consent, spoilage, placement or slots.
- No new action or phase instant; no population cap; no live model call; no `cmd/vs2` change; nothing in
  `docs/`.
