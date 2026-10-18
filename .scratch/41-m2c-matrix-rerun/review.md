# Review — ticket 41: the M2c matrix run and its verdict

Branch `feature/41-m2c-matrix-rerun`. Reviewer-launch HEAD reported by the diff tool: `3da26d41c0da`, working tree clean, **no untracked files** — so the claimed tip `3da26d4` and a clean tree are confirmed. Base `ae0396f` is confirmed indirectly (every gate transcript in `.scratch/41-m2c-matrix-rerun/gates.log` records `revision ae0396f`).

**What this review could not do, and why (declared before any finding).** `watchdog_diff` does not inspect committed ranges, and I have no shell, so I could not: view the `ae0396f..3da26d4` diff; verify that commit `5ffab6b` (the pre-run declaration) predates the launch; verify the `sha256` of the committed byte copy `.scratch/41-m2c-matrix-rerun/m2c-evaluation.json` against `.tmp/41-m2c-evaluation.json`; or confirm the branch changes no Go file. Each of those is a **check the supervisor must run** (commands at the end). Everything below is verified against files present in the checkout.

---

## 1. VERDICT REPRODUCIBILITY — reproduced, exactly

**The band rule is the pre-declared one.** `vrd2.py:250-253`: `band = max(max(vals) − mu, mu − min(vals))`, i.e. the max absolute single-replicate deviation from the mean — exactly ticket 38 §7.4's "`max(envHi−mu, mu−envLo)`", and the pair band is "the **wider of the two arms'** half-widths" (same sentence). I re-derived every arm's mean and band from the record's own per-replicate vectors (§8.4) and they match the scorer's transcript (`m2c-scores.log`) to 4 dp with no residual:

| arm | d_H from vectors (deficits/1444) | mean | band | printed |
|---|---|---|---|---|
| `P` | 0, 0, 91/1444=0.063019 | 0.021006 | 0.042013 | 0.0210 (±0.0420) ✔ |
| `U` | 266/1444, 311/1444, 219/1444 | 0.183749 | 0.032087 | 0.1837 (±0.0321) ✔ |
| `A` | 0, 74/1444, 53/1444 | 0.029317 | 0.029317 | 0.0293 (±0.0293) ✔ |
| `A1` | 1/1444, 10/1444, 1/1444 | 0.002770 | 0.004155 | 0.0028 (±0.0042) ✔ |
| `A0` | 0,0,0 | 0 | 0 | 0.0000 (±0.0000) ✔ |

`d_S` likewise (`/N0 = /10`): P 0/0/0.1 → mean 0.0333, band 0.0667; U 0.3/0.4/0.6 → 0.4333, 0.1667; A 0/0.1/0.1 → 0.0667, 0.0667; A1 0/0.1/0 → 0.0333, 0.0667 — all printed identically. So the "3-replicate wider-envelope rule" claim in the record is true of the implementation, and the implementation is the declaration.

**Floors are the declared ones**: `m2c-scores.log:3` prints `actor-hours 0.01 (14.4 actor-hours at H_T = 1444), starvation deaths 0.10, births 0.50; a tier separates only above max(band, floor)`, matching ticket 38 §7.4's table (`ε_H=0.01, ε_S=0.1, ε_B=0.5`) and `docs/VALIDATION.md`'s "Uncertainty" bullet.

**The two primary verdicts follow.**
- `A` vs `U`: `|0.0293 − 0.1837| = 0.1544` > `max(0.0321, 0.01)` → **SEPARATES, `A` more viable**, by 4.8× the band ✔ (`scores.log` prints "decided by the band").
- `A` vs `P`: `|0.0293 − 0.0210| = 0.0083` ≤ `max(0.0420, 0.01)` → **TIE** ✔; and the record's extra claim that the floor alone would also tie it is arithmetically true (`0.0083 < 0.01`) — verified, not just asserted.

**Tier structure verified.** Tier 1: the scorer prints `persisted=True extinct=False` for all five arms (`scores.log:6-11`), i.e. unanimity, no separation ✔. Tier 4: `dB = 0.0000 (±0.0000)` on every arm and every pair, so nothing approaches `0.5` ✔ (with the caveat in §4 below). Tier 3: the only `d_S` gaps above `max(band,0.1)` are `A/U` 0.3667, `P/U` 0.4000, `A1/U` 0.4000, `A0/U` 0.4333 — **exactly the `U` pairs** ✔, and every one of them already separates at tier 2, so the highest-separating-tier rule makes tier 3 decide nothing ✔. `A0` vs `A` ties at 0.0293 = band 0.0293 under the strict comparison ✔.

**No pair depends on the voided `A1-f10-s0-r2`.** Neither primary pair contains `A1`. For completeness I re-derived all four `A1` pairs with `r2` dropped (`A1` d_H → 0.0007, band → 0): `A` vs `A1` 0.0286 ≤ 0.0293 tie→tie; `A1` vs `P` 0.0203 ≤ 0.0420 tie→tie; `A1` vs `U` 0.1830 > 0.0321 sep→sep; `A1` vs `A0` 0.0007 < floor 0.01 tie→tie. §8.2's robustness claim is **true**, and it was the right thing to check before taking the decision.

**Verdict rule applied correctly**: ticket 38 §7.4 — "SEPARATION iff at least one **primary** pair separates" — one does → `SEPARATION` ✔. The falsifier (§7.6) is not met, and it is also not what the run reports.

One nit worth stating because it recurs: the headline "at 248 decisions per arm" (§9, §10, and the VERTICAL-SLICES row) is exact for `U`/`A`/`A1` but **not for `P`**, which is gate-bound at 244/244/228 (§8 table; independently corroborated by poll row 3 of `poll-log.md`, which names `P-r3` as the 228 cell). The arms are compared at an equal *declared ceiling*, which is the declared axis, so nothing is wrong — the summary sentence is just looser than the tables.

## 2. THE CELL VOID — recorded properly; two placement gaps

- The rule fires on the right fact: `m2c-contract-gate.log` → `A1 {'applied': 740, 'rejected': 4, 'inadmissible': 1, 'malformed': 3}`, `proposed action counts {'eat_granary_food': 1}`, `refusing gate counts {'illegal-action': 1}`. One cell, one answer, one attempt. Sums reconcile with §8.2's residual-loss table exactly (applied 2 939 + rejected 9 = 2 948, of which inadmissible 1 + malformed 8 = 9; per arm 742+2, 740+4, 714+2, 743+1 = 744, 744, 716, 744 = 2 948 = the charged-call total).
- Cell-level void is recorded with the A1 pairs marked **not reportable** (§8.3 rows, §9.2, §10 last-but-one bullet) ✔.
- Replicate count shown as **2 of 3 used** (§8, bolded) ✔.
- No A1 number presented as a complete result: §8.3's A1 rows are labelled "sensitivity only … not a result this record reports", and the verdict does not use them ✔ — with two residual risks noted as P2 below (the §8.3/§8.4 A1 rows carry no in-line void flag, and one tier-1 sentence says "all five arms persist in all three replicates" while the scorer's own header line says "secondary: 2", which counts `A1 vs U`).
- **Consistency with the pre-registration's own sentence: confirmed verbatim.** Ticket 38 §7.6 gate 3: *"Every model-bearing cell must record 0 `inadmissible` answers. A nonzero count voids **that cell**."* Reading (B) is the literal reading; reading (A) ("voids the run") is the re-interpretation. §9.2 says exactly this and states the authority (product-owner decision of 2026-10-18), and it does not touch the pre-registered criterion, which is correct.
- The one thing missing is **placement**: the new "cell void vs run void" rule's only home is a run record (`.scratch/41-m2c-matrix-rerun/spec.md` §9.2). Ticket 38 §7.6 — the document a future run reads — still says only "voids that cell" and stops, and `docs/VALIDATION.md`'s "Uncertainty" bullet still says the verdict is refused "unless the declared inventory is complete (15 cells, 3 replicates per arm, no run halted)" without the void clause. M2d will hit this again. P2, one sentence in each place.

## 3. THE READING IS NARROW ENOUGH — yes

I read every claim-bearing sentence in the run record (§Verdict header, §9, §9.1, §9.2, §10, §12), the results tables, the `docs/VERTICAL-SLICES.md` M2c row, and the two other places the verdict is echoed. There are exactly **two** homes for the verdict: the run record and the VERTICAL-SLICES M2c row (`VALIDATION.md` owns the metric, not the result; `CHANGELOG.md` says nothing — see P2). Both pre-empt the two misreadings in the same paragraph as the claim:

- *"It is not 'adaptive beats periodic': that pair is a tie, and periodic is the baseline a selective allocator most needs to beat."* (`spec.md:408`) and again in §10 and in VERTICAL-SLICES.
- *"Not the central claim. Selective allocation beating uniform/periodic at equal budget is not established by one cell"* (§10), *"No A1 claim at all"*, *"no cross-cell, cross-seed or cross-fixture claim, and the frozen rule's 'in both cells' clause is vacuous at one cell"*, *"no 'better' language beyond 'more viable under `vrd/2`'"*.
- Scope qualifiers appear in the verdict sentence itself, not in a footnote: "at 248 decisions per arm, on **one cell** (`f10-s0`), one contract (`natural-claim/v4`), one horizon (`H = 144`), three replicates".
- `docs/VERTICAL-SLICES.md:242` closes with *"What this establishes and does not: a separation at one cell under one contract, never the central claim; the selective-vs-uniform **spend** comparison that claim needs belongs to M2d/M4"*, and the section footer repeats *"the central claim and the M2 milestone remain not accepted"* ✔.

No sentence I found claims settlement or promotes the tier-2 separation to "selective allocation works". Two smaller honesty gaps (P2, both about what a reader is *not* told rather than a claim):

- **`VS2-EC-3`/`VS2-EC-4` print `[not-evaluable]` on this artefact** (`m2c-evaluation.txt:79-84`: `complete A/P/U cells=0/12`, `fidelity rows=0`, divergence vector `none (no complete A/P/U vs R cell)`, all 12 short projections `refused (no-reference)`) because `R` is a declared non-goal (§2). The record's §6 transcribes and explains the two *FAILs* (EC-1, EC-6) but says nothing about the two *not-evaluable* criteria. A reader of the M2 record could conclude the frozen EC-3/EC-4 vector dominance was evaluated and merely not reported. §10's "no override of the frozen EC-3/EC-4 dominance rule" is not the same statement as "not evaluable here".
- **"Tier 4 ties everywhere" needs its mechanism stated.** `d_B = max(0, (B_T − B_x)/B_T)` (`vrd2.py:98`) is **one-sided**: any arm at or above the control's births scores 0. `U`'s extra births therefore *cannot* show up, and no arm can ever win tier 4 — only lose it. The record notes the effect ("do not move the mean component at all under the declared normalisation") but presents tier 4 as a tie. That is arithmetically right and inferentially weak; one clause would prevent "the arms reproduce equally" being read off it.

## 4. THE TIE IS THE INTERESTING RESULT — the report says the right thing, but not the power thing

What the record says plainly, and it is right: `A` vs `P` is a tie **because the periodic arm is the baseline a selective allocator must beat and it did not** ("periodic is the baseline a selective allocator most needs to beat", §9/§10; "not distinguishable from periodic on any tier", §10).

The honest counterweight exists but is thin. §10's last bullet: *"this run is not evidence that `A` is indistinguishable from `P` in general — it is a tie at one cell, under one contract, inside bands as wide as `0.0420` actor-hours."* §9 adds that the floor, not only the band, ties it (`0.0083 < 0.01`). What the record never states is **power**, and the numbers behind it are in its own tables:

- Band `0.0420` × `H_T = 1444` = **60.7 actor-hours** ≈ 4.2 % of the control's actor-hours.
- Observed `A` vs `P` difference `0.0083` = **12.0 actor-hours** = 0.8 % — 5× inside the band, **and below the declared minimum effect size** (`ε_H = 0.01` = 14.4 actor-hours). So even a perfectly noiseless 12-actor-hour difference would be declared a tie. "No effect detectable at the declared resolution", not "no effect".
- The band is the max single-replicate excursion, so at 3 replicates the **minimum detectable difference** for `A` vs `P` is ≈ 0.029–0.042 (42–61 actor-hours, 3–4 %). The run has power only for effects of the size it found against `U` (0.1544 = 223 actor-hours).
- Same-arm replicate spread **dwarfs** the between-arm difference: `P`'s deficits are 0, 0, 91 actor-hours (one replicate 6.3 % below both siblings, with the one starvation death); `A`'s are 0, 74, 53. Two of three `P` replicates sit exactly at the anchor and one carries the whole band.
- Therefore the record's sentence *"this tie is not an artefact of a wide band alone"* is true **of the declared 3-replicate estimate** and should not be read as robustness: drop `P-r3` and the declared rule (`band → 0`, `mu → 0`) would give `0.0293 > max(0, 0.01)` — a *separation*. Drop-one is not a declared variant and the record was right not to run it as a result, but a reader needs to know the tie is decided by one `P` replicate's excursion.

**What a reader should conclude about power:** at 3 replicates and this band the test cannot resolve any policy effect below ~4 % of actor-hours; a "tie" at `A` vs `P` is a statement that no effect of that size was seen, not that the arms are equivalent, and the experiment's own replication noise (0–6 %) is the dominating term. M2d must raise replicates or widen the horizon before a tie against `P` can carry weight. None of this changes the verdict; all of it belongs in §10 in one sentence (P2).

## 5. RIGOUR OF THE RUN RECORD — sound, with one wrong evidence cell

Verified from artefacts:

- **15 cells, 3 replicates per arm, 0 halted**: `m2c-evaluation.txt:18-34` lists 15 rows, column `halt` = `false` on all; `m2c-inventory-gate.log` → `PASS — 15 of 15 declared cells scored, 3 replicates per arm, 0 halted, 0 skipped`. Realized counts 248/248/248 (`U`,`A`,`A1`), 244/244/228 (`P`), 0 (`A0`) ✔.
- **`attempts − decisions = 0` in every cell**: `m2c-contract-gate.log` role-a rows per arm 744/744/716/744 = 2 948 = the charged-call total in §8 and in `poll-log.md` poll 4 (2 948). Any retry anywhere in a cell would make that arm's call rows exceed its role-a rows (each cell's decisions are all present), so the matrix-wide identity is proven, not asserted; the whole `R_t = 8` reserve was unspent ✔.
- **The two transport events voided nothing else**: one `http-error 400` (content policy) at `A-f10-s0-r2:243` (poll 1) and the one `inadmissible` at `A1-f10-s0-r2:627` (`seq 627`, cause `{actor 8, hour 139, vs2-A1, natural.claim}`). Both produced an in-attempt fallback — consistent with the arm-level call/decision identity above ✔. Contract tallies per arm show `inadmissible 0` for `A`, `P`, `U` ✔.
- **Spend arithmetic**: 1 853 633 × $0.10/M + 768 547 × $0.50/M = $0.185363 + $0.384274 = **$0.5696** ✔, the same totals in poll 4 and poll 5 (final), and 2 948 of 3 980 remaining attempts on the 7 000 cumulative cap; 44 % of the $1.30 hard cap and never near the $1.04 stop ✔. Wall clock 16:54:57Z → between 17:11:37Z and 17:12:24Z ≈ 17 min ✔.
- **The four declared post-run commands, in order**: `m2c-contract-gate.log` header `17:12:47Z` → `m2c-inventory-gate.log` `17:13:10Z` → `m2c-scores.log` (no timestamp header; sits between per §12 and `poll-log.md` row 6) → `m2c-curve-gate.log` `17:13:24Z`. Order consistent; the scores log alone is untimestamped, which is the only reason this is not fully self-proving.
- **Curve corroboration agrees on both primary pairs**: `m2c-curve.txt:389` `A@low vs P@low: NOT distinguishable`, `:414` `A@low vs U@low: DISTINGUISHABLE (KPI population …)`, and it additionally distinguishes `P@low vs U@low`, `A1@low vs P@low`, `A1@low vs A@low` (`granary_hist[0]`) and `A0@low` vs each — exactly the disagreement set §8.3/§9 declares ✔.
- **Six tier permutations change 0 of 36; `h = H−1` changes 0 of 6**: printed by the scorer (`m2c-scores.log`, last two lines) ✔.
- **Pre-launch gates**: `gates.log` verbatim (`go1.24.13`, build/vet/gofmt/diff-check at `ae0396f`, binary 11 954 258 B), `anchor-gate.json` three identical replicates (`alive_h143 2`, `alive_h144 1`, `realized_calls 0`, `halted false`, identical `projection_hash 33b7692a…`) ✔, probe transcript present ✔, gates timestamped 16:53:52Z–16:54:18Z, before the 16:54:57Z launch ✔.
- **Declaration predates launch — not verifiable here.** The record claims commit `5ffab6b` for §1–§8.5; timestamps inside the record are self-consistent but a commit's date is not in the tree. Supervisor check below.

**One evidence table is wrong.** §8.4's `U` row mislabels its replicates. Measured from `.scratch/41-m2c-matrix-rerun/m2c-evaluation.json` (block offsets pinned on `A1-f10-s0-r1` and `A1-f10-s0-r2`, whose `deaths_starve_cum` tail = 1 confirms §8.4's "the `A1` `S = 1` replicate is `r2`"):

| `U` | measured in artefact | §8.4 table |
|---|---|---|
| r1 | `births_cum` ends 4, `deaths_starve_cum` ends 6 | `B 2`, `S 3` |
| r2 | 2, 3 | `B 2`, `S 4` |
| r3 | 2, 4 | `B 4`, `S 6` |

The table's pairs are the measured pairs **rotated one replicate step** (`table r1 = measured r2`, `r2 = r3`, `r3 = r1`). Means and bands are order-invariant, so no verdict, tier, floor or band in this record changes — I re-derived all five arms with the corrected values and every printed figure is unchanged (`U` mean `S = 13/3/10 = 0.4333`, band 0.1667). But §8.4 is offered as "the per-replicate vectors behind the means", and §9's prose repeats the row ("replicates `1178/1133/1225`") and then adds a figure that exists in no replicate: *"`U`'s extra births (2/4/6 against the control's 2)"* — no `U` replicate has 6 births, and 6 is `U r1`'s **starvation** count. `P`'s row is coherent (its `1353` replicate is the 228-decision `r3` per poll row 3, and it carries the one starvation death). `A1`'s row is correct at `r2` as checked.

## 6. WHAT THIS DOES TO THE STAGE — mostly right

- `docs/VERTICAL-SLICES.md:242` reads: *"the matrix is complete and its verdict is `SEPARATION` on one primary pair only — `A` vs `U`, while `A` vs `P` ties (ticket 41)"*, then *"it is not 'adaptive beats periodic'"*, then *"What this establishes and does not: a separation at one cell under one contract, never the central claim"*. It does **not** say "passed", and it does not read as a settled question ✔. The section footer and the M2 stage note both repeat that the central claim and the milestone are not accepted ✔.
- Two coherence gaps at stage level (P2): (i) the Status cell still opens *"**Part 1 done** 2026-10-18, ticket 38"* — the verdict is now the third part, so the terminal status ("answered: one separating primary pair, one tie") is buried in a 3 000-character cell rather than stated as the stage's outcome, while M2a and M2b both carry a bold **PASSED** label; a reader cannot tell from the table whether M2c has discharged its question or is still mid-flight. (ii) M2d reads *"Not started. Requires M2c."* — the stage rule is "a later stage may not begin before its predecessor passes", so if M2d is now unblocked that should say so, and if M2c's answer does not count as a pass, M2d is mislabelled. One clause fixes both, and it is a *product* decision (does an answered-but-partial M2c unlock M2d?) that the record should not leave to inference.
- M2d is correctly named as the next step in both the M2c row and §10 ✔.

---

## Findings

- **Fixed:** nothing. This is a review-only task and I applied no edit.
- **Finding: P1** — A second, contradicting record for ticket 41 is committed: `.scratch/41-m2c-rerun/spec.md` (title "the re-run of the M2c matrix under the transport reserve") states *"Status: **not run.** No live call, no ledger, no run root and no artefact exist for this ticket"* and *"§2 … Not yet run."* against the delivered 15-cell, `$0.5696` run. It is tracked (`.gitignore` excludes only `.pi/`, `.tmp/`, `__pycache__`), it sits in the same ticket namespace (`ls .scratch/41*` returns both), and its content post-dates ticket 40 (`N − R = 248`), so it is a superseded draft of this ticket, not a predecessor. Two homes for the ticket-41 plan fact, one of them false. **Smallest fix:** `git rm .scratch/41-m2c-rerun/spec.md` (git history is the archive; no fact is lost).
- **Finding: P1** — `.scratch/41-m2c-matrix-rerun/spec.md:428` (§8.4 `U` row) mislabels replicates, and `:428` (§9) states births no replicate has: measured `U` = `r1 B4/S6, r2 B2/S3, r3 B2/S4` against the table's `B 2/2/4`, `S 3/4/6` (a one-step rotation) and the prose's "2/4/6". Evidence: `m2c-evaluation.json`, `births_cum`/`deaths_starve_cum` tails of the three `U` blocks; offsets pinned on `A1 r1`/`A1 r2`. No verdict changes (means and bands are order-invariant; I re-derived all five arms and every printed figure still holds). **Smallest fix:** replace the `U` row's `B`/`S` columns with `4/2/2` and `6/3/4`, re-check its `H` column in the same pass, and delete the "(2/4/6 …)" parenthetical in §9.
- **Finding: P2** — No power statement anywhere. The record has "not evidence that `A` is indistinguishable from `P` in general … inside bands as wide as 0.0420" but never the numbers that follow from its own tables: band 60.7 actor-hours (4.2 % of control), observed difference 12.0 actor-hours, below the declared floor 14.4 actor-hours, minimum detectable difference ≈ 42–61 actor-hours at 3 replicates, and same-arm replicate excursions of 0/0/91 (`P`) and 0/74/53 (`A`) that dwarf the between-arm difference; dropping `P-r3` would flip the declared rule to a separation. **Fix:** one sentence in §10.
- **Finding: P2** — `VS2-EC-3` and `VS2-EC-4` print `[not-evaluable]` on the shipped artefact (`m2c-evaluation.txt:79-84`; no `R` cell, all 12 short projections `refused (no-reference)`), while §6 transcribes only the two `FAIL`s (EC-1, EC-6). **Fix:** add the two `not-evaluable` rows to §6 with the reason (`R` excluded by §2).
- **Finding: P2** — Tier 4 is one-sided by construction (`vrd2.py:98`, `max(0, …)`), so "tier 4 ties everywhere" cannot mean "equal fertility": `U`'s extra births are clipped to 0 and no arm can win that tier. **Fix:** one clause beside the tier-4 sentence.
- **Finding: P2** — The cell-void-vs-run-void rule lives only in the run record (§9.2). Ticket 38 §7.6 and `docs/VALIDATION.md`'s "Uncertainty" bullet still carry the unamended wording, so the next run (M2d) must re-decide it. **Fix:** one sentence each, forward-pointing to §9.2 and its authority.
- **Finding: P2** — `CHANGELOG.md` has no entry for ticket 41 (nor for tickets 39/40; the last entry is ticket 38, `:220`), although the file's own convention is one dated entry per ticket with the revision. Also note `CHANGELOG.md:255` records the M2c **hard cap `$2.00`**, while ticket 40's re-costing and this record use **`$1.30`** — harmless for a "record of events, not a source of truth" file, but two caps for one run will read as a discrepancy to a future auditor. **Fix:** one entry with the tip revision and the $0.5696/$1.30 figures.
- **Finding: P2** — A1 rows carry no in-line void flag: §8.3/§8.4 list A1's three replicates and §9 says "all five arms persist in all three replicates", while the scorer's own summary line counts `secondary: 2` (including the now-void `A1 vs U`). Every one of these is disclosed elsewhere in the same file, so nothing is claimed, but a reader entering at the table sees a complete arm. **Fix:** append "(`r2` voided, §9.2)" to the A1 rows and reconcile the printed `secondary: 2` in §8.3.
- **Finding: P2** — Two small precision/binding items: "at 248 decisions per arm" is exact for `U`/`A`/`A1` but `P` is gate-bound at 244/244/228 (declare the worst case as "≤ 248, `P` gate-bound"); and the record never states that this branch changes no Go file — every gate and the run binary were at `ae0396f`, so the evidence binds to the base revision only if the branch is docs-and-scratch-only. **Fix:** one sentence each.

**Unverified by this review (supervisor must run):**
```sh
git rev-parse HEAD                                        # expect 3da26d41c0da…
git log -1 --format='%h %cI' 5ffab6b                      # expect before 2026-10-18T16:54:57Z
git merge-base --is-ancestor 5ffab6b HEAD && echo ok      # declaration is an ancestor
git diff --stat ae0396f..HEAD                             # expect docs/ and .scratch/ only, no *.go
sha256sum .scratch/41-m2c-matrix-rerun/m2c-evaluation.json .tmp/41-m2c-evaluation.json  # expect 6b3c4085e917…
python3 .scratch/38-m2c-preregistration/vrd2.py --live .tmp/41-m2c-evaluation.json \
  --scores .scratch/37-fidelity-metric/scores.json   # optional: re-emits m2c-scores.log verbatim
```

---

## Three verdicts

**(a) Is the verdict reproducible from the artefacts? YES.** Every statistic the verdict rests on reproduces exactly: the band is the pre-declared max-deviation wider-envelope rule over 3 replicates (`vrd2.py:250-253` = ticket 38 §7.4), and I re-derived all five arms' `d_H`, `d_S`, bands and every pair decision from the record's vectors to 4 dp with no residual; the floors are the declared `0.01/0.1/0.5`; tier 1 unanimity, tier 4 ties and tier 3 separating only the `U` pairs are all confirmed by the scorer's own per-arm output; `A` vs `U` separates at 0.1544 > 0.0321 and `A` vs `P` ties at 0.0083 ≤ max(0.0420, 0.01); no pair touches the voided `A1-r2`, and dropping it changes no A1 pair's verdict either. Caveats: the §8.4 `U` row mislabels replicates (no verdict effect), and the commit-date and byte-hash claims need the shell checks above.

**(b) Is the reading honestly narrow? YES.** Both likely misreadings are negated in the same paragraph as the claim ("not 'adaptive beats periodic'", "not the central claim", "periodic is the baseline a selective allocator has to beat"), the scope qualifiers sit inside the verdict sentence, `docs/VERTICAL-SLICES.md` closes its M2c row with "never the central claim", and the milestone footer still says the central claim is not accepted. The gaps are omissions, not over-claims: no power statement (the tie is 5× inside a band driven by one `P` replicate and also below the declared floor, so "no detectable effect at 3 replicates" is the correct reading, not "no effect"), tier 4's one-sided clip, and the undisclosed `[not-evaluable]` EC-3/EC-4 rows.

**(c) Is the branch safe to integrate? OK with notes.** There is no P0: no code changed in the verdict's path (gates and binary at `ae0396f`), the pre-registered rule was applied as written, and every number the verdict stands on is reproducible. Two P1 documentation defects should be fixed in the same pass before merge — the committed duplicate record that says the ticket was "not run", and the rotated `U` row in §8.4 plus §9's "2/4/6" — plus the P2 list above, which is one-sentence-each copy work. None of them changes the verdict, the tiers, the bands, the void or the spend, and none requires re-running anything.