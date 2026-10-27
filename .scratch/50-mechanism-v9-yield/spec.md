# Ticket 50 — mechanism v9: land yield as a declared axis, and the food ledger

- Record kind: one mechanism version adding a single declared world parameter, plus the calibration
  measurement that uses it. Zero live spend. Workset `W1_world_runs`.

## 1. What ticket 49 established

Mechanism v8 + `natural/13` (band-wide mate pool, household consent gate) fixed fertility and exposed
the next wall. `f10-s0`, `H = 144`: **births 12** (was 2), starvation deaths **9**, age deaths 9,
`alive_end` 4 of 10; the 128-cell grid is 0/128 in the same way. Reading per ticket 49 §4:
**FOOD-SHORT** — births are ample, starvation is the binder.

The land is the cause: a patch yields at most **one granary unit per cultivated slot per hour**
(`NaturalSlotsPerPatch = 8`, `R ≤ 8`), so patch 0 — which ID placement fills with 8 founders — sits
exactly at basal break-even, and every birth on it is paid for by somebody's starvation. The world's
carrying capacity is therefore `patches × 8`, with no surplus where the band actually stands.

## 2. The declared change (mechanism v9)

`NaturalMechanismV9` = `NaturalMechanismV8` rules plus **one declared parameter**:

- `NaturalOptions.Yield int64` / `RestoreOptions.Yield` — the granary units granted per cultivated slot
  per patch-hour. `0` means the v8 default of `1`; `1..NaturalSlotsPerPatch` are admitted. It is world
  and run identity end to end (journal config, checkpoint manifest, mismatch rejection), exactly like
  `R`, and it exists **only** under v9 — every earlier mechanism refuses a non-default value.
- Everything else is v8 verbatim: births, consent, kin, the age clock, spoilage, wear, the phase
  instants, the payment rules and every frozen ledger invariant (the yield simply scales the rule-508
  grant, and the conservation ledger must keep balancing).

## 3. The calibration (this ticket's measurement)

1. **Food ledger first.** On `f10-s0` under v8, print the final `NaturalBalance` components
   (`Produced, Granted, Unrealized, Gathered, Consumed, Spoiled, GranaryStock, StoredMeals, BirthGranaryPaid,
   BasalSpent, Energy`) so the wall is quantified rather than asserted.
2. **Yield sweep.** `f10-s0`, `H = 144`, v9 + `natural/13`, `Yield ∈ {2,3,4}`, reporting the W1
   criterion (`alive_end ≥ founders`, starvation deaths, births, gate-hours).
3. **Choose the smallest yield that passes**, then re-run the 128-cell grid at that yield. The declared
   preference order: the smallest yield whose `f10-s0` cell passes (a) `alive_end ≥ 10`, (b) zero
   starvation deaths, (c) ≥ 1 birth, and (d) gate-hours ≥ 24. Scarcity must survive the calibration: a
   yield that makes the gate trivially always-on (gate-hours = 144 for every cell) overshoots and is
   rejected in favour of the smaller one.
4. Record the chosen value in the ticket record as the world's declared fertility, with the ledger
   numbers that justify it.

## 4. Non-goals

- No change to slots per patch, patch count, founder placement, the age clock, kin, consent, births or
  the food *prices* (`R`, spoilage, basal cost, meal gain).
- No new action, phase instant or population cap.
- No live model call; no `cmd/vs2` change; nothing in `docs/`.

## 5. Results (measured; appended by the ticket-50 implementation run)

### 5.1 The food ledger on `f10-s0` under v8 (the wall, quantified)

`ledger-f10-s0.tsv`, one row: policy, founders, seed, declared founders, `alive_end`, births,
starvation deaths, age deaths, world-gate hours, policy-gate hours, then the final `NaturalBalance`
components `Produced, Granted, Unrealized, Gathered, Consumed, Spoiled, GranaryStock, StoredMeals,
BirthGranaryPaid, BasalSpent, Energy`:

```
natural/13 10 0 10 | alive 4 | births 12 | starve 9 | age 9 | gate 1/1
Produced 1492  Granted 551   Unrealized 277  Gathered 1479  Consumed 1430  Spoiled 127
GranaryStock 38  StoredMeals 218  BirthGranaryPaid 24  BasalSpent 1820  Energy 34
```

The wall is not total food: 1492 wild units were produced and 1430 were consumed, and 551 cultivated
units were granted — yet 9 of 18 ever-living actors starved. The binder is **access** to the food,
not its quantity (see 5.3).

### 5.2 Yield sweep, mechanism v9 + `natural/13` (`yield-sweep.tsv`, extended 5–8)

| Yield | alive_end | births | starvation | age | world-gate h | policy-gate h |
|---|---|---|---|---|---|---|
| 2 | 7 | 14 | 4 | 13 | 1 | 1 |
| 3 | 9 | 19 | 11 | 9 | 20 | 19 |
| 4 | 9 | 19 | 11 | 9 | 20 | 19 |
| 5 | 9 | 19 | 11 | 9 | 20 | 1 |
| 6 | 9 | 19 | 12 | 8 | 20 | 19 |
| 7 | 10 | 20 | 11 | 9 | 14 | 1 |
| 8 | 10 | 18 | 8 | 10 | 9 | 9 |

No admitted yield (`1..NaturalSlotsPerPatch`) meets the frozen W1 reading: starvation deaths are never
zero, and gate-hours never reach 24. Raising the yield raises `Granted` (1824 → 8224 over 144 h) without
saving the starving: they are the landless.

### 5.3 Death analysis (`ledger-yield3-deaths.tsv`, yield 3)

All 11 starvation deaths occur on patch 1002 (the two-founder patch), and every one of them is a
landless actor (`capital 0`, `granary 0`) at death. The patch is food-saturated per capita: 6–10 living
actors compete for the 5–7 wild units standing on it at the death hour (one granary unit per wild slot
per hour), i.e. under one unit per actor-hour before births. The trace of actor 10 shows the same:
granary savings 33 → 0 while it pays births, energy pinned at 1, then starvation with wild stock on
its home patch. Mechanism v9's yield sells food *into a few granaries*, where it spoils (`Spoiled`
rises to 1913 at yield 3); it cannot feed an actor with no land and no stored meal.

### 5.4 Mechanism v10 + natural/14 (the placement lever)

`NaturalMechanismV10` = v9 with founder `i` on patch `(i-1) mod P` (founders round-robin, not in
ID-blocks of eight), `NaturalFounderPatchIDFor(m, …)` versioning the geometry so v5–v9 stay
byte-identical. `natural/14` = natural/13 with the founder-home observation clause evaluated under the
round-robin placement; it is legal under v10 only, natural/13 stays legal under v8/v9 only.

Measured on `f10-s0`, `H=144`, v10 + natural/14 (`yield-sweep-v10.tsv`):

| Yield | alive_end | births | starvation | age | world-gate h | policy-gate h |
|---|---|---|---|---|---|---|
| 1 | 4 | 18 | 20 | 4 | 103 | 91 |
| 2 | 5 | 22 | 18 | 9 | 6 | 4 |
| 3 | 4 | 19 | 13 | 12 | 6 | 4 |
| 4 | 3 | 19 | 13 | 13 | 6 | 4 |
| 5 | 3 | 19 | 13 | 13 | 6 | 4 |
| 6 | 3 | 19 | 13 | 13 | 6 | 4 |
| 7 | 3 | 19 | 13 | 13 | 6 | 4 |
| 8 | 3 | 19 | 13 | 13 | 6 | 4 |

Round-robin placement raises the conception gate-hours (births 18–22) but the cohort still collapses:
at yield 8 the world grants 2736 granary units and spoils 2285 of them while 13 actors starve and only
3 live. The placement lever alone therefore does not meet W1 either.

### 5.5 Reading and consequence

- The v9 axis is a clean declared parameter: it is run identity end to end, refused below v9, and it
  scales exactly the rule-508 per-slot grant while every conservation identity keeps balancing.
- The v10 axis is a clean declared placement change: round-robin genesis, v5–v9 byte-identical.
- **Neither lever moves `f10-s0` to W1.** The measured binder is food *access*: a landless newborn
  (no cultivated slot, no stored meal, one wild unit per patch-hour shared with every other landless
  actor) starves regardless of how much granary food the landed households hold or spoil. That is the
  next lever, outside this ticket's declared non-goals.
- **No yield is chosen and the 128-cell grid is not run**: the selection rule ("smallest yield whose
  `f10-s0` cell passes") has no admissible witness in 1..8 under v8, v9 or v10. Recording a chosen
  yield would be false. This is the ticket's measurement result, not an implementation defect.
- Ticket 49's v8 record and this ticket's v9 record stand as what each lever alone bought: v8 bought
  births (12 at `f10-s0`) and v9 bought food quantity (551 → 8224 granted), but the surviving band did
  not grow because the newborn is landless.

### 5.6 Diagnostic 128-cell grid at yield 8 (`grid-128-yield8.tsv`)

Because §3 selects a yield no admissible cell actually meets, the grid was run as a **diagnostic at
yield 8** (the largest admitted yield, the one closest to the f10-s0 reading) — not as a chosen yield.
v9 + natural/13, f = 1..16 × seed = 0..7, H = 144:

- **0/128** cells meet `alive_end ≥ 10 ∧ starvation deaths = 0 ∧ births ≥ 1`.
- 28/128 reach `alive_end ≥ 10`; 9/128 record zero starvation deaths; 14/128 hold the joint gate for
  ≥ 24 hours — but no cell holds all of them together.
- The widest cell reaches `alive_end 17`; starvation deaths reach 32 in the worst cell.

The grid confirms on the whole lattice what `f10-s0` shows: the yield axis buys food, not survivors.
