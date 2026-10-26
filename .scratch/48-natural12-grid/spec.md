# Ticket 48 — the 128-cell grid under `natural/12`: the addressee is not the binder

- Record kind: zero-spend regression grid over the declared founder/seed axis, three no-AI policies.
  Part of workset `W1_world_runs`. Code: [`sweep.go`](sweep.go); data: `grid-natural7.tsv`,
  `grid-natural12.tsv` (16 founders × 8 seeds × `H = 144`, one replicate, zero model calls).

## 1. Result

| policy | cells | pass `alive_end≥N0 ∧ starve=0 ∧ births≥1` | max `alive_end` | births | starvation deaths | age deaths | cells with ≥1 birth |
|---|---|---|---|---|---|---|---|
| `natural/7` (frozen control) | 128 | **0** | 6 | 269 | 280 | 1010 | 94 |
| `natural/12` (kin-aware addressee) | 128 | **0** | 3 | 267 | 236 | 1028 | 96 |

`natural/12` changes 70 of 128 cells and is a **wash or slightly worse**: it removes 50 → 0 `kin`
refusals on the declared cell but files far fewer requests (a non-kin-only rule waits when the patch is
one lineage), so births are unchanged in total and the survivor maximum falls from 6 to 3.

## 2. What this closes

Three policy versions now exist and none of them makes the world run: `natural/7` (135 `consent-own`
refusals, 3 accepts), `natural/11` (rotation, kin-blind: 99 `consent-own` + 50 `kin`, 4 accepts),
`natural/12` (non-kin rotation: 0 refusals, 4 accepts). Addressability is therefore **not** the binder:
the best-informed addressee rule files 4 requests where the blind rule filed 153, and the population
still ends at 1 in every policy. On the whole grid the survivor maximum *falls* from 6 to 3.

The refusal order in the frozen consent rule explains the apparent contradiction. `consent-own` is
checked **before** `kin`, so a kin mate whose own state fails the wealth gate is counted as
`consent-own`. Under `natural/11`, 99 of those 149 kin-or-poor refusals are kin.

`explain.go` measures the pool directly at `f10-s0`, over all 144 checkpoints, counting every
same-patch mate slot seen by an actor who holds the proposer gate (`k ≥ 2, granary ≥ 3, energy ≥ 4`):

| policy | proposer-hours | same-patch mate slots | of them kin | non-kin | non-kin holding the consenter gate |
|---|---|---|---|---|---|
| `natural/12` | 136 | 370 | **368** | 2 | 2 |
| `natural/7` | 136 | 370 | 234 | 136 | **2** |

Two different walls, in this order: under the blind rule 136 mate slots are non-kin but only **2** of
them hold the wealth gate; under the kin-aware rule the proposer skips the kin, and only **2** non-kin
slots exist at all. Both walls are properties of the population, not of the policy: a patch holds five
founders, and after one generation the co-located pool is one lineage — 368 of 370 slots are kin.

So `W1_world_runs` cannot pass inside the frozen world by policy alone, and the remaining levers are
world rules of different kinds: the **kin rule** relative to patch size (depth 2 in a five-founder patch,
no migration), the **two-parent wealth conjunction** at the decision instant (ticket 44 §5's household
gate), and the **early food economy** that decides how many founders survive to breed. Each changes a
rule the operator owns, and this record does not choose between them.

## 3. Why the choice is put to the operator rather than taken

Ticket 44's standing direction is *keep the age-death clock, calibrate fertility*, which covers
threshold calibration inside the existing rules. Every remaining lever changes something the operator
owns: the kin rule changes the simulated society's incest boundary, the household gate changes who may
consent to a birth, and the food economy changes the survival pressure itself. Each is reversible (a new
mechanism version with the old ones byte-frozen), but none is a threshold.

## 4. Limits

- The counterfactual behind the household gate is arithmetic over recorded state, not a run; the gate's
  actual effect, including the food ceiling at larger cohorts (ticket 18: food supply equals basal burn
  at `N = 16`), is what the post-change grid must measure.
- One replicate per cell; the control is deterministic, so no replicate noise is measured here.
- The grid's verdict is about the frozen world; nothing here bounds a different substrate.
