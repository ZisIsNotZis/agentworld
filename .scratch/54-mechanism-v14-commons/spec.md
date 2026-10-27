# Ticket 54 — mechanism v14: the commons rule (a patch always keeps half its land wild)

- Record kind: one mechanism version with a single admission rule, plus its calibration. Zero spend.
  Workset `W1_world_runs`.

## 1. Where tickets 49–53 leave the world

v12 (one unit = two hours of life) removed the treadmill; v13 added the birth price. The world now
behaves as a boom–bust: at `f10-s0`, `Yield=2, BirthCost=2` the band reaches **alive 17** with 57 births
but **43 starvation deaths against 7 age deaths**; at `Yield=1, BirthCost=2` it reaches alive 7 with 36
starvation against 4 age. Raising the birth price collapses births instead of damping the boom (at
`BirthCost=4`: 27 births, 31 starvation; at 6–8: extinction).

Ticket 50's death analysis found the mechanism: the starved are **landless** actors (`capital 0`,
`granary 0`) — a patch with all eight slots cultivated has no wild unit for anyone who owns no slot, so a
newborn on that patch has no food source at all, whatever the land's fertility. Cultivated yield belongs
to its owner and spoils there (ticket 52: `Spoiled` 990 while 43 starved).

## 2. The declared change (mechanism v14)

`NaturalMechanismV14` = `NaturalMechanismV13` plus one admission rule:

- **A patch must always keep at least half of its slots wild.** A build (slot claim) that would leave
  fewer than `NaturalSlotsPerPatch/2` wild slots on that patch is refused on the existing typed path
  (`no-free-slot`); everything else about building, tenure, decay, gathering, cultivation yield, spoilage
  and eating is unchanged.
- Rationale, declared: the wild land is a commons, and a commons that can be fully enclosed leaves the
  next generation with nothing to eat — which is exactly the measured death. Half is the declared floor,
  the smallest fraction that guarantees a landless actor finds food on its own patch.

Only v14 admits the rule; v5–v13 keep their admission exactly.

## 3. The calibration

1. `f10-s0`, `H = 144`, v14 + `natural/14`, over `Yield ∈ {1,2}` × `BirthCost ∈ {2,4}` (the four cells
   that remain informative): report `alive_end`, births, starvation, age, gate-hours.
2. Choose the smallest pair meeting the amended `W1` criterion: `alive_end ≥ 10`, **age deaths ≥
   starvation deaths**, births ≥ 1, gate-hours ≥ 24, not overshot (gate-hours ≠ 144 everywhere).
3. Run the 128-cell grid at the chosen pair; report the pass count.
4. Append `## 5. Results` and commit.

## 4. Non-goals

- No change to basal, bag, gather, build cost, tenure decay, kin, consent, births, spoilage, placement,
  slots per patch or the age clock.
- No new action or phase instant; no population cap; no live model call; no `cmd/vs2` change; nothing in
  `docs/`.

## 5. Results

### 5.1 What was built

`NaturalMechanismV14` (world axis version 14) = the v13 rule table plus exactly one admission rule: a
patch always keeps at least `NaturalCommonsWildMin = NaturalSlotsPerPatch/2 = 4` wild slots. The rule
lives in the one slot-claim site (`NaturalBuildFor`, the pure claim `NaturalBuildProposalFor` rebuilds),
counts the wild slots on the builder's home patch, and refuses a completion whose claim would leave
fewer than four: same typed `NaturalBuildRefusalError` path (`NaturalBuildRefusalNoWildSlot`, the
journal note `no-wild-slot`), zero mutation — nothing converts and the bag keeps its unit. Every other
rule is v13 verbatim: the birth price axis (`naturalHasBirthCostAxis`), the two-energy meal, the yield
axis, the yield-scaled wild fill, the round-robin founding band, the provisioned ledger, basal, bag,
kin, consent, spoilage and the age clock. v5–v13 are untouched; `naturalKeepsCommons` is true for v14
only, and the boundary test (`TestNaturalV14CommonsFloor`) pins both directions: a claim that leaves
exactly half the patch wild is admitted under v14, the claim that would take the last wild slot of the
half is refused under v14 and admitted under v13.

### 5.2 Calibration table, `f10-s0`, H = 144, v14 + `natural/14` (`calibration.tsv`)

Column order: policy, yield, birth-cost, founders, seed, N0, alive_end, births, starvation deaths, age
deaths, world-gate h, policy-gate h, then the eleven `NaturalBalance` fields.

| Yield | BirthCost | alive_end | births | starvation | age | world-gate h | policy-gate h | W1 |
|---|---|---|---|---|---|---|---|---|
| 1 | 2 | 11 | 12 | 1 | 10 | 0 | 0 | gate ✗ |
| 1 | 4 | 9 | 17 | 11 | 7 | 5 | 1 | alive ✗, age ✗, gate ✗ |
| 2 | 2 | 11 | 13 | 2 | 10 | 0 | 0 | gate ✗ |
| 2 | 4 | 11 | 15 | 6 | 8 | 2 | 1 | age ✗, gate ✗ |

Raw lines:

| Yield | BirthCost | line |
|---|---|---|
| 1 | 2 | 1145, 1100, 75, 1137, 737, 694, 120, 262, 24, 1460, 184 |
| 1 | 4 | 1112, 1032, 176, 1104, 699, 635, 68, 261, 68, 1473, 142 |
| 2 | 2 | 1162, 2214, 632, 1150, 747, 1540, 277, 371, 26, 1675, 191 |
| 2 | 4 | 1160, 2154, 663, 1150, 746, 1499, 233, 362, 60, 1711, 168 |

(`Produced, Granted, Unrealized, Gathered, Consumed, Spoiled, GranaryStock, StoredMeals,
BirthGranaryPaid, BasalSpent, Energy`; `InitialEnergy = 110`. The G3 identity closes with the v12 meal
price and the v13 birth price, e.g. yield 2/cost 2: `110 + 8·13 + 2·747 + 371 = 191 + 1675 + CapLost`.)

### 5.3 The selection

The amended W1 criterion is `alive_end ≥ 10 ∧ age deaths ≥ starvation deaths ∧ births ≥ 1 ∧
gate-hours ≥ 24 ∧ gate-hours ≠ 144`.

- Yield 1, BirthCost 2: alive 11 ✓, age 10 ≥ starvation 1 ✓, births 12 ✓, gate-hours 0 ✗ → fails.
- Yield 1, BirthCost 4: alive 9 ✗, age 7 < starvation 11 ✗, births 17 ✓, gate-hours 5 ✗ → fails.
- Yield 2, BirthCost 2: alive 11 ✓, age 10 ≥ starvation 2 ✓, births 13 ✓, gate-hours 0 ✗ → fails.
- Yield 2, BirthCost 4: alive 11 ✓, age 8 ≥ starvation 6 ✓, births 15 ✓, gate-hours 2 ✗ → fails.

**No pair meets the amended W1 criterion, so no parameters are chosen and no world parameters are
adopted.** The commons floor does what §2 declares to the deaths: at `f10-s0` yield 1 the starving fall
from v12's 36 to 1 and age deaths outnumber starvation (10 against 1) — the landless now find wild food
on their own patch. What it costs is the consent gate: with only four cultivated slots per patch and
five founders per patch, `capital ≥ 2` is rarely reachable, so births fall (12 against v12's 37) and
the gate-hours collapse to 0 at both yield-1/2 cost-2 cells.

### 5.4 Diagnostic 128-cell grid at (Yield 1, BirthCost 2) (`grid-128.tsv`)

Because §3.2 has no witness, the grid was run as a **diagnostic at the smallest pair** (Yield 1,
BirthCost 2) — the setting the sweep's first cell reads — not as a chosen pair. v14 + `natural/14`,
f = 1..16 × seed = 0..7, H = 144, 128 cells:

- **0/128** cells meet `alive_end ≥ 10 ∧ age deaths ≥ starvation deaths ∧ births ≥ 1 ∧
  gate-hours ≥ 24 ∧ gate-hours ≠ 144`.
- Clause by clause: 63/128 reach `alive_end ≥ 10`; 94/128 have age deaths ≥ starvation deaths; 120/128
  record at least one birth; **0/128** hold the world gate for ≥ 24 hours (the maximum is 18, and no
  cell reaches the overshoot ceiling).
- The widest cell reaches `alive_end 18`; starvation deaths peak at 38.

The calibration is negative and is recorded as such: mechanism v14 is a clean, single-rule version
with the wild commons kept, every conservation identity balancing, and the measured death shifted from
starvation to age at `f10-s0` — but no admissible pair holds the gate, and on the 128-cell lattice the
gate clause alone is never met. No further rule was added: the next lever (whether the consent gate
should read *land access* rather than a two-point capital stock) is outside this ticket's non-goals.
