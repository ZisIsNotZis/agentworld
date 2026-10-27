# Ticket 52 — mechanism v12: a granary unit feeds a person for two hours

- Record kind: one mechanism version with a single declared price change, plus its calibration.
  Zero spend. Workset `W1_world_runs`.

## 1. The arithmetic ticket 51 exposes

The natural food loop is an exact treadmill: a gather moves **one** unit into a bag that holds
**one** unit; eating a unit restores **one** energy; basal drain is **one energy per hour**
(`NaturalGenomeDenominator / locusM`, exactly 1 at the neutral locus 6, 1.2 at locus 5, 0.86 at locus 7).
So an actor that does nothing but gather and eat ends the 144-hour horizon with zero energy to spare,
and every hour spent building, giving birth (2 granary + 2 energy), or unlucky at locus 5 comes directly
out of its life. No land fertility can help: the bottleneck is the **conversion**, not the supply —
which is why ticket 51's wild-yield version produced more food (`Spoiled` 443–1523 units) and still
starved 12–21 of 20–25 newborns.

## 2. The declared change (mechanism v12)

`NaturalMechanismV12` = `NaturalMechanismV11` rules plus one price:

- **Eating one granary unit restores 2 energy instead of 1.** One declared constant
  (`naturalMealEnergyV12 int64 = 2`), applied in `NaturalConsumeFor`/its mechanism-parameterized form;
  `Yield = 1` and every other rule of v11 unchanged, so v12 differs from v11 only in this price.
- Rationale, declared: a granary unit is a *meal* — two hours of life, not one. At 2 the loop has a
  real surplus (one gather-hour funds two hours of life), which is the condition for building, for
  births, and for surviving a bad locus without turning the world into a treadmill.

Nothing else moves: the age clock, basal rates, bag capacity, gather, build, births, kin, consent,
spoilage, land yield, placement. The energy ledger must keep balancing (`Consumed` and `BasalSpent`
identity in `natural_contract_math.go` is the invariant to watch).

## 3. The calibration

1. `f10-s0`, `H = 144`, v12 + `natural/14`, `Yield ∈ {1, 2}` — report `alive_end`, births, starvation,
   age, gate-hours and the `NaturalBalance` line for each.
2. Choose the smallest yield that meets (a) `alive_end ≥ 10`, (b) starvation deaths = 0, (c) ≥ 1 birth,
   (d) gate-hours ≥ 24, rejecting overshoot (gate-hours = 144 everywhere). If `Yield = 1` passes, record
   that the yield axis is not needed at all.
3. Run the 128-cell grid at the chosen yield; report the pass count under the same four clauses.
4. Append `## 5. Results` to this file with the numbers and the chosen world parameters.

## 4. Non-goals

- No change to basal rates, bag capacity, gather, build, births, kin, consent, spoilage, land yield,
  placement or population size.
- No new action or phase instant; no population cap; no live model call; no `cmd/vs2` change; nothing in
  `docs/`.

## 5. Results

### 5.1 What was built

`NaturalMechanismV12` (world axis version 12) = v11 rules plus one declared price: an eaten bag unit is a
two-hour meal. One declared constant (`naturalMealEnergyV12 = 2`) with one reader (`naturalMealEnergyFor`),
which `NaturalConsumeFor` grants and every G3 energy identity books; below v12 that reader returns the
frozen 1, so v5–v11 keep their bytes term for term (the frozen `internal/world` suite and the v11 sweep
row reproduce unchanged, §5.2). The cap is respected: the unit is always consumed and whatever of the
meal does not fit under `NaturalEnergyCapacity` is counted spill in `CapLost`, exactly as the frozen
one-unit meal does at the cap, so

    newborn energy + Consumed + StoredMeals == Energy + BasalSpent + CapLost

balances with the consumed term weighted by the declared meal price (the aggregate and per-newborn forms
follow). The declared cap-loss envelope widens to 2·168 under v12 (a declared envelope, never a
behaviour cap). Wiring: validity, `worldVersion`/`RuleVersion`/projection, registry and descriptor names,
the v11 yield axis / round-robin founding band / yield-scaled wild fill (same predicates), the v6-family
provisioned ledger, the checkpoint manifest's derived mechanism slot, the journal's closed sets and the
CLI mechanism axis. `natural/14` stays the policy; it is legal exactly under the round-robin mechanisms.

Declared reading of the price (recorded because the spec states it, not derived): the change is rule
507's bag meal — `NaturalConsume`/`NaturalConsumeFor` — and **the stored-meal draw (`NaturalEatStored`)
keeps its frozen one energy**, because §2 names `NaturalConsumeFor` as the one site. That asymmetry is
large in this world: at `f10-s0` yield 1, 1184 of the 1373 meals eaten over the horizon are stored meals.

### 5.2 Yield sweep, `f10-s0`, H = 144, v12 + `natural/14` (`yield-sweep-v12.tsv`)

| Yield | alive_end | births | starvation | age | world-gate h | policy-gate h |
|---|---|---|---|---|---|---|
| 1 | 7 | 37 | 36 | 4 | 33 | 5 |
| 2 | 17 | 57 | 43 | 7 | 7 | 6 |

`NaturalBalance` lines (columns `Produced, Granted, Unrealized, Gathered, Consumed, Spoiled,
GranaryStock, StoredMeals, BirthGranaryPaid, BasalSpent, Energy`; `InitialEnergy = 110`):

| Yield | line |
|---|---|
| 1 | 716, 1572, 32, 710, 189, 187, 82, 1184, 74, 1934, 34 |
| 2 | 720, 3090, 415, 712, 199, 990, 165, 1769, 114, 2656, 75 |

Both lines close the G3 identity with the declared price (`110 + 8·births + 2·Consumed + StoredMeals =
Energy + BasalSpent + CapLost`; yield 2 spills 110 energy at the cap). Harness parity: the same
construction from ticket 51 still prints its recorded v11 yield-1 row verbatim (`4, 18, 20, 4, 103, 91,
…`), so the differences above are the mechanism's, not the harness's.

### 5.3 The selection

- Yield 1: alive 7 (< 10) ✗, starvation 36 ✗, births 37 ✓, gate-hours 33 ✓ → **fails**.
- Yield 2: alive 17 ✓, starvation 43 ✗, births 57 ✓, gate-hours 7 (< 24) ✗ → **fails**.

**No admissible yield meets the four clauses, so no yield is chosen and no world parameters are adopted.**
The price does what §2 declares arithmetically — one gather-hour now funds two hours of life, births rise
37/57 against v11's 18/25, and the cohort keeps more survivors at yield 2 (17 against 8) — but the deaths
are unchanged in kind: they are landless newborns with no cultivated slot and no stored granary, eating
from a wild fill that one gather per hour starves, and every extra birth is another mouth at the same
fill. The meal price buys a longer life per unit, not access to a second unit.

### 5.4 Diagnostic 128-cell grid at yield 1 (`grid-128-yield1.tsv`)

Because §3.2 has no witness, the grid was run as a **diagnostic at Yield 1** — the setting that isolates
v12's one declared difference from v11 (identical wild fill, identical founding band) — not as a chosen
yield. v12 + `natural/14`, f = 1..16 × seed = 0..7, H = 144, 128 cells:

- **0/128** cells meet `alive_end ≥ 10 ∧ starvation deaths = 0 ∧ births ≥ 1 ∧ gate-hours ≥ 24`.
- Clause by clause: 35/128 reach `alive_end ≥ 10`; 8/128 record zero starvation deaths; 120/128 record at
  least one birth; 70/128 hold the gate for ≥ 24 hours. No cell holds them together.
- The widest cell reaches `alive_end 17`; starvation deaths reach 60 in the worst cell.

The calibration is negative and is recorded as such: mechanism v12 is a clean, single-price version with
every conservation identity balancing, and its measured effect on `f10-s0` and on the 128-cell lattice is
more births and more survivors at yield 2, with starvation deaths never at zero. No further rule change
was improvised; the next lever is food *access* for the landless newborn, outside this ticket's non-goals.
