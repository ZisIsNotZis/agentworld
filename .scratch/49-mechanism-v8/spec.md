# Ticket 49 — mechanism v8: the band-wide mate pool and the household consent gate

- Record kind: world-rule change under a new mechanism version, plus its matching policy; declared
  before any measurement. Zero live spend. Workset `W1_world_runs`.
- Author: agent (product owner), 2026-10-26, on the standing direction *keep the age-death clock,
  calibrate fertility* and on the measurements of tickets 44–48.

## 1. What the measurements leave

- `f10-s0`, all 144 checkpoint hours, counted over actors holding the proposer gate: 370 same-patch mate
  slots, **368 kin**, 2 non-kin — a patch holds five founders, so one generation leaves one lineage
  (ticket 48).
- Under the blind rule those 136 non-kin slots hold the consenter's wealth gate in **2** cases.
- So two independent rules bind, and every policy lever is exhausted (`natural/7`, `natural/11`,
  `natural/12`; grid 0/128 twice).

## 2. The declared change (mechanism v8)

`NaturalMechanismV8` = `NaturalMechanismV6` rules plus exactly two changes, both about who may conceive:

1. **The mate pool is the band, not the patch.** A proposer may file to any living, unrelated,
   non-kin adult in the roster; the co-location requirement and the `cross-patch` refusal disappear.
   Justification: the substrate models a founding band on two adjacent patches with no movement
   mechanism; treating co-residence as a mating precondition made reproduction a function of an
   unmodelled constraint. Nothing else about patches changes — gathering, building, storage, food and
   the `Bag.Source` home stay exactly as they are.
2. **The household consent gate.** The proposer keeps the full wealth gate
   (`capital ≥ 2 ∧ granary ≥ 3 ∧ energy ≥ 3`) and still pays the birth cost; the consenter only has to
   be **fed** (`energy ≥ 2`). This is ticket 44 §5's recorded design, applied with the same constants.

Everything else is v6 verbatim: the age-death clock, the food/patch economy (R, regen, spoilage, wear),
the request/reply phase instants, the refusal taxonomy, the birth payment, the runner's fresh-truth
re-validation, and every frozen ledger and invariant. The refusal reasons keep their meaning:
`consent-own` = the consenter is not fed (`energy < 2`), `consent-claims` = the proposer's claims fail,
`kin` = within the prohibited depth, `gate-thresholds` = the proposer's own fresh-truth gate fails.
`cross-patch` becomes unreachable and its value stays reserved.

## 3. The matching policy (natural/13)

`natural/13` = `natural/12` verbatim (kin-aware addressee over the band-wide pool now) with the consent
floor matched to the household gate (`energy ≥ 2` instead of the filing floor of 4 for the consenter).
The proposer's filing floor stays unchanged. Old refs stay byte-frozen; `natural/13` is legal under
mechanism v8 only.

## 4. The declared measurement

1. `f10-s0`, `H = 144`, `-arms A0`-equivalent no-AI run under mechanism v8 + `natural/13`: report
   `alive_end`, births, starvation deaths, age deaths, gate-hours (both floors).
2. The 128-cell grid (16 founders × 8 seeds) under the same settings, reduced exactly as ticket 48's
   `sweep.go` does.
3. Pre-declared reading:
   - **PASS** — `f10-s0` meets `alive_end ≥ 10`, zero starvation deaths, ≥ 1 birth, and ≥ 24 hours with
     two actors holding the joint gate: `W1_world_runs` is met and the plan moves to W2.
   - **FERTILITY-SHORT** — births rise but the cohort still ends below its founding size: the birth rate
     is still the binder, and the next lever is the birth cost / filing frequency inside the same
     mechanism family.
   - **FOOD-SHORT** — starvation deaths appear at larger cohorts while births are ample: the food
     economy is the binder (ticket 18's food ceiling equals basal burn at `N = 16`), and the next lever
     is the food rule, not fertility.
   No other reading is permitted by this ticket.

## 5. Non-goals

- No movement model, no new action, no new phase instant.
- No change to the age clock, the food/patch economy, kin depth, or the request/reply taxonomy.
- No live model call; no `cmd/vs2` change; nothing in `docs/`.
