# Ticket 44 — Fertility calibration: why births stop

- Record kind: zero-spend census plus the design of the change it points to. No world change, no code change, no model call.
- Author: agent (product owner), 2026-10-19.
- Question: rung 2 FAILS (0/128 no-AI cells reach a good ending, `.scratch/43-rung2-seed/`). The operator's direction is to keep the age-death clock and calibrate fertility. Which layer actually caps fertility?
- Inputs: `docs/VERTICAL-SLICES.md` (rung table), `internal/world/natural_contract.go` (gate constants), `internal/world/natural_contract_math.go` (`NaturalSurplusGateAllows`, `NaturalBirth`), `internal/strategy/natural6.go`/`natural7.go` (the policy's filing and consent floors), `.tmp/43-a0-evaluation.json` (the 128-cell sweep's per-hour granary and capital histograms).

## 1. Method

`census.go` rebuilds one `A0` cell exactly as the runner does (births-disabled genesis pulse, branch to births enabled, policy `natural/7`, mechanism v6), steps to `H = 144`, and reads every hourly `NaturalCheckpoint`:

- per-actor gate components: `granary ≥ 3`, `capital ≥ 2`, `energy ≥ 3` (the world gate) and `energy ≥ 4` (the policy's own filing/consent floor);
- co-location and kinship from `Bag.Source` and `Genome.ParentA/ParentB`;
- the checkpoint's cumulative `Metrics.Births`, `Metrics.BirthRequests` and `Metrics.Refusals` (typed), which name the refusing rule;
- the journal's attempt kinds and build-refusal notes.

It counts, per cell: eligible **pair-hours** (two co-located, non-kin, unrelated adults, symmetric gate), and, as a counterfactual only, **household pair-hours** (one actor holding the gate plus any co-located non-kin adult with `energy ≥ 2`). Nothing was changed in the world; the counterfactual is arithmetic over the same recorded state.

## 2. Measurement (three cells, H = 144, zero model calls)

| | `f10-s0` | `f12-s0` | `f6-s0` |
|---|---|---|---|
| births / birth requests filed | 2 / 138 | 5 / 105 | 2 / 106 |
| refusal histogram | `consent-own` 135 | `consent-own` 95 | `consent-own` 11 (13 replies total) |
| peak alive / alive at H | 12 / 1 | 16 / 1 | 8 / 1 |
| age deaths / starvation deaths | 11 / 0 | 13 / 3 | 5 / 2 |
| max granary, any actor | 46 | 29 | 34 |
| max capital, any actor | 3 | 3 | 3 |
| max energy, any actor | 11 | 11 | 11 |
| hours with ≥2 actors holding the world gate | **1** | **4** | 2 |
| eligible pair-hours (symmetric gate, co-located, non-kin) | 3 265 | 4 306 | 1 462 |
| hours with ≥2 actors at `granary ≥ 3` | 43 | 42 | 35 |
| household pair-hours (one gate-holder, partner fed) | **136** | **37** | **284** |
| builds attempted / refused `no-wild-slot` | 315 / 96 | 277 / 96 | 127 / 6 |
| provisions (rule 515) | 144 | 152 | 29 |

## 3. Finding: the gate is a conjunction of rare states, and the refusal names it

The policy is not idle and the world is not out of food. The frozen policy files a birth request in almost every hour it can (138, 105, 106 filings), and **the typed refusal is `consent-own` in essentially every case**: the addressee's own state fails the consent formula. The world's own gate (`NaturalSurplusGateAllows`) is the same formula, so the two layers agree and the binding condition is the **joint state of two actors at one patch-hour**:

`capital ≥ 2 ∧ granary ≥ 3 ∧ energy ≥ 3` for **both** parents,

and the economy holds that jointly for **1–4 hours out of 144** while individually each component is common (43 hours with two actors at `granary ≥ 3`; `max capital 3`; `max energy 11`; `max granary 46`). The components alternate rather than co-hold: building consumes the stored meal (`build-denied-eat` 96 attempts, and 96 builds refused for `no-wild-slot`), so granary is spent to raise capital and capital is what the same actor must keep while granary recovers. The observed funnel is therefore **~100–140 filings → 2–5 births**, with fertility ≈ 0.2–0.42 per founder per lifetime against the ≈ 1.0–1.5 replacement needs.

This is the mechanism behind ticket 43's extinction and it explains the earlier records: v4's "final 0 everywhere" and v5/v8's "extinction h141/h143" are the same arithmetic. It is not a code regression. The pre-v4 world looked alive because no actor could die of age at all; from v4 onward the world has a fixed 96–143 h death clock and a fertility gate that almost never opens.


## 3b. Falsification of the first reading: the gate is ample under other frozen policies

Before acting on §3, the same census was run over the frozen policy ref set on the same fixtures (`-policy natural/5|6|7|7b|9`, mechanism matched, H = 144, zero model calls). Result at `f10-s0`:

| policy | birth requests | births | hours with >=2 actors at `k>=2, granary>=3, energy>=3` | hours with >=2 at `k>=2, granary>=3, energy>=4` | refusals |
|---|---|---|---|---|---|
| natural/5 | 260 | 3 | **107** | **107** | `consent-own` 135 |
| natural/6 | 138 | 2 | 1 | 1 | `consent-own` 135 |
| natural/7 | 138 | 2 | 1 | 1 | `consent-own` 135 |
| natural/7b | 138 | 2 | 1 | 1 | `consent-own` 135 |
| natural/9 | 3 | 1 | 125 | 106 | `consent-own` 1 (6 starvation deaths, 4 age) |

So the joint gate is **not** intrinsically rare: under `natural/5` two actors hold it for **107 of 144 hours** and even at `energy >= 4` for 107 hours, and `natural/9` reaches 125 hours. The rarity in `natural/7` is caused by its own provisioning rule (rule 515: 144 provisions at `f10-s0`, each converting a granary unit into a bag unit for building), which is exactly the granary drain the first reading mistook for an economic law.

This narrows the cause but does not yet name it: under `natural/5` the state is ample and the script still refuses **135** times with `consent-own` and produces 3 births. Either (i) the consent instant (`h+40m+6µs`) differs from the hourly boundary this census samples — eating, gathering and provisioning all happen at `h+40m`, so the boundary state can flatter the actor — or (ii) the script's consent rule is stricter than the state it observes. The census cannot distinguish them, and the difference decides whether the fix is a **policy floor** (no world change) or a **world rule**.

## 3c. Therefore the next measurement, not the next design

Instrument the decision instances themselves: at every `birth-request` and `birth-reply` attempt, record the proposer's and consenter's `(energy, granary, capital, patch, kin)` as seen by the rule that decided, and the typed refusal. That is a read-only instrumentation of the existing evidence path (the journal already carries `BirthEvidence` with reported and true capital/granary), costs nothing, and settles (i) vs (ii). The household-gate design in §5 stays declared but is **not** the recommended first move until that measurement lands: if (i) holds, no world change is needed and the fix is a policy version whose floors match the state it is given; if (ii) holds, the world rule is the lever.

## 3d. What this changes about §5

§5's household gate remains a candidate, with its measured counterfactual (37-284 household pair-hours against 1-4 under `natural/7`). It is no longer the first move, and its justification cannot rest on "the conjunction is unreachable": under `natural/5` it is reachable for 107 hours. The dependency chain is now: decision-instant census -> policy-floor change (likely, cheap) -> re-run the 128 cells -> only if fertility is still short, the household gate.

## 4. Why not simply move one threshold

Loosening one component does not help: the joint count is 1–4 hours, and even `capital ≥ 1` instead of `≥ 2` raises it only to 2–7 hours (`hours_with_2plus_g3_k1_e3` = 2, 7, 10). The conjunction, not any single threshold, is the defect. Fertility needs a gate whose satisfaction is *reachable as a state*, not a lottery over four components.

## 5. Proposed change (not implemented; needs the operator's word)

Replace the symmetric two-parent wealth gate with a **household gate**, keeping every other rule and the death clock:

- proposer (household head): alive, `capital ≥ 2`, `granary ≥ 3`, `energy ≥ 3`, and pays the existing 2 granary + 2 energy;
- consenter (partner): alive, co-located, non-kin, unrelated, `energy ≥ 2` (fed, not wealthy).

Measured counterfactual on the same runs: eligible pair-hours rise from **1–4** to **37–284** per cell (a 9–70× increase), which is the order of magnitude fertility needs to reach replacement. The change is additive: a new world axis version, with v5/v6/v7 byte-frozen and every recorded bundle still restorable, plus a matching policy version whose consent floor is the household rule (the existing policy stays byte-frozen).

What it does not fix, and must be measured after the change: the food ceiling still equals basal burn at `N = 16` (ticket 18), so a replacement-level population will press against it and the surplus channel stays live; whether that yields a stationary population, an oscillation, or a new collapse is exactly what the post-change 128-cell sweep must answer.

## 6. Limits

- Three cells (`f10-s0`, `f12-s0`, `f6-s0`), one seed each, `H = 144`, `-reps 1` (the control is deterministic; ticket 43 reproduced the recorded anchors exactly).
- The census reads hourly boundary state, not the intra-hour instants at which filing and consent are evaluated (`h+40m+5µs`, `h+40m+6µs`). A gate that holds only mid-hour could be missed; the direction of that error is to **understate** reachability, so the finding is conservative.
- The household counterfactual is arithmetic over recorded state; it assumes a partner at `energy ≥ 2` would consent, which the policy's reply rule and the world's consent rule would have to be changed to allow. It is not a run.
- Nothing here bounds a different substrate, a longer horizon, or a changed food supply.
