# Review — ticket 42 (M2d replication), `feature/42-m2d-replication` @ `6abd169`

**Scope.** Checkout verified at the branch tip: `watchdog_diff` reports no working-tree changes against reviewer-launch HEAD `6abd16948f9a`, and `.git/modules/agentworld/logs/HEAD` ends at `6abd169` ("ticket 42 results and verdict: FAILED TO REPLICATE …"), base `d38a86b` (ticket 41 follow-up, master). Parents' declared artefacts read: `.scratch/42-m2d-replication/{spec.md,poll-log.md,f12-s0-scores.log,f10-s1-scores.log,evaluator-gate.log,gates.log,curve-and-contract-gates.log,gateway-probe.txt}`, `.tmp/42-{f12-s0,f10-s1}-evaluation.{json,txt}`, the run roots, the unchanged scorer `.scratch/38-m2c-preregistration/vrd2.py`, the anchor `.scratch/37-fidelity-metric/scores.json`, the M2c record `.scratch/41-m2c-matrix-rerun/{spec.md,m2c-scores.log}`, `docs/VERTICAL-SLICES.md`, `CHANGELOG.md`, `.gitignore`, and `cmd/vs2/{bands.go,run.go}` + `internal/cognition/allocation.go` where the report's own semantics decide a reading.

**Not verifiable with my toolset:** no shell, so I could not run `git diff --stat d38a86b`, `go build/vet/gofmt`, `vrd2.py`, or `35-admis.py`. I substituted primary-file evidence (raw JSON arrays, plain-text reflog, `.gitignore`, grep for the ledger's rejection strings). Commands a supervisor must still run are named in §7.

---

## 1. Recompute the verdict — the numbers are right

`vrd2.py`'s live path takes its anchor from `--scores` (`vrd2.py:886-888` → `recorded_cells`), not from the artefact, so **every cell is scored against `f10-s0`'s committed control** `H_T=1444, B_T=2, S_T=0, N0=10` (confirmed in `.scratch/37-fidelity-metric/scores.json:21-25`). Components are `components()` at `:80-91`; tier order is `pair_verdict(..., tiers=("H","S","B"))` at `:196-215`, which returns at the **first** separating tier after the tier-1 unanimity test; the band is the single-condition half-width `max(max−μ, μ−min)` at `:249-252`; a tier separates iff `|Δ| > max(band, floor)`.

I recomputed from raw arrays, not from the transcript:

| cell | arm | S replicates (verified in artefact) | mean dS (÷N0=10) | band dS | H replicates | mean dH | band dH |
|---|---|---|---|---|---|---|---|
| `f12-s0` | `A` | 4/3/2 (`deaths_starve_cum[-1]`, lines 57530/53453/49376) | 0.3000 | 0.1000 | 1487/1677/1701 | 0.0000 | 0.0000 |
| `f12-s0` | `P` | 5/4/5 (28983/24899/33064) | 0.4667 | 0.0667 | 1513/1620/1518 | 0.0000 | 0.0000 |
| `f12-s0` | `U` | 7/7/8 (44699→7, 37142→7, 41219→8) | 0.7333 | 0.0667 | 1367/1422/1435 | **0.0249** | **0.0284** |
| `f12-s0` | `A0` | 3/3/3 (783) | 0.3000 | 0.0000 | 1700×3 | 0.0000 | 0.0000 |
| `f10-s1` | `U` | 1/1/4 | 0.2000 | 0.2000 | 1395/1387/**1163** | 0.0893 | 0.1053 |
| `f10-s1` | `A` | 0/0/1 | 0.0333 | 0.0667 | 1495/1496/1451 | 0.0000 | 0.0000 |

I summed `alive_by_hour` by hand for the one replicate that decides `f10-s1`: `U` block at line 40538 = **1163** (10×5, 11, 12×17, 11×11, 10×14, 9×36, 8×19, 7×9, 6×4, 5×8, 4, 3×3, 2×4, 1×13), i.e. `dH_U = (1444−1315)/1444 = 0.0893`, band `(1395−1315)/1444 = 0.1053`. The hand sum reproduces the scorer's figure exactly.

**The four claimed statistics hold exactly:**
- `f12-s0` `A` vs `U`: `|0.3000−0.7333| = 0.4333 > max(0.1000, 0.1000)` — 4.3× threshold ✓
- `f12-s0` `A` vs `P`: `|0.3000−0.4667| = 0.1667 > max(0.1000, 0.1000)` — 1.7× ✓
- `f10-s1` `A` vs `U`: `|0.0000−0.0893| = 0.0893 ≤ max(0.1053, 0.0100)` ✓
- `f10-s1` `A` vs `P`: `|0.0000−0.0333| = 0.0333 ≤ max(0.0667, 0.1000)` ✓

**Tier logic applied correctly.** Both `f12-s0` separations are reported at tier 3, and tier 2 was evaluated first and failed (`A` vs `P`: both `dH=0`, Δ=0; `A` vs `U`: 0.0249 ≤ its own band 0.0284). No pair is decided at a tier below one that had already separated — `pair_verdict` returns at the first hit, and the six-order sensitivity in both transcripts (`0 of 36`) confirms no order dependence. The `f10-s0` M2c pair is not re-run, so no cross-cell pooling could occur.

## 2. Is the `f12-s0` separation an artefact of the cell's harshness?

**No — but the record explains the mechanism wrongly, and the honest framing is "damage avoidance", not "better viability".**

All five arms' absolute outcome per cell (from §8.1, cross-checked against raw arrays for the decisive arms; age deaths verified for `A0` at both cells — `deaths_age_cum[-1]` = **13** at `f12-s0`, **11** at `f10-s1`):

| cell | `A0` | `P` | `U` | `A` | `A1` |
|---|---|---|---|---|---|
| `f12-s0` | 1700 h / 5 B / 3 S / 13 age | 1550 h / 5 B / 4.67 S | 1408 h / 5 B / 7.33 S | 1622 h / 4 B / 3.0 S | 1569 h / 3.33 B / 3.0 S |
| `f10-s1` | 1496 / 2 B / 0 S / 11 age | 1496 / 2 / 0 | 1315 / 2 / 2.0 | 1481 / 2 / 0.33 | 1495.3 / 2 / 0 |

Three facts map the comparison:

1. **Tier 2 was not dead at `f12-s0`, as §8.1 claims — only *nearly* dead.** `U`'s three replicates (1367/1422/1435) are *below* the 1444 anchor, so `dH_U = 0.0249 ± 0.0284`, not 0. It missed separating `A` from `U` by 0.0035 — one third of one actor-hour-rate unit. The verdict is unchanged, but the sentence "every arm's every replicate exceeds 1 444 actor-hours … tier 2 cannot separate anything there" is false and is load-bearing prose (Finding P2-1).
2. **The separation is not an amplification of one quantised unit.** It is corroborated in the *unscored* dimension: against the cell's own control, `A` is 78.3 actor-hours short and `P` is 149.7 short (I re-derived both: `1621.7/1550.3` vs `1700`), and `A`'s starvation mean equals the control's exactly (3.0 vs 3.0) while `P` and `U` are strictly worse than the control at tier 3 (`P vs A0` and `U vs A0` both separate in `A0`'s favour in the transcript). So `f12-s0`'s ordering — `A0 ≥ A ≈ A1 > P > U` — is a real ordering in both dimensions.
3. **What it measures is damage avoidance, not improvement.** `A` vs `A0` **ties** at `f12-s0` (and at `f10-s0`, `.scratch/41-m2c-matrix-rerun/m2c-scores.log:19`, and at `f10-s1`). In no cell of this project has an AI arm separated from *doing nothing*. At `f12-s0` the adaptive arm's advantage is that it does not harm as much as the periodic/random arms do.

Quantising honestly: `Δ = 0.1667` is **1.667 starvation deaths** against a **1.0-death** threshold (`0.1 × N0 = 1.0`), i.e. 14 % vs 8 % of a 12-founder population — and §10.4's own check is right that a single `A` replicate at `S=5` would widen `A`'s band to 1.67 deaths and turn the pair into a tie. Note also that 13 of the 16 deaths at `f12-s0` are *age* deaths; the deciding statistic describes a secondary mortality channel in a churn-dominated world. That context is absent from §8.1.

**`f10-s1` is the opposite failure.** All ten declared and diagnostic pairs tie, including every arm against `A0`, and `P`'s three replicates are numerically identical to `A0`'s (1496/2/0). The cell has no range: every arm sits within 45 actor-hours and 1 death of the control, while the tier-2 band that would have to be beaten is 152 actor-hours (a single `U`-r3 excursion of 232) and the tier-3 band is 0.2 dS. Nothing in that cell could have separated, in either direction.

## 3. The verdict category: right by the table, wrong as a summary

**The table was pre-declared and is the one the data falls in.** Reflog timestamps (epoch → UTC via `1792281600` offset, calibrated against poll 1): `5f3d0a3` "pre-declare the M2d replication" at **17:28:25Z**; gates `01d64a7` 17:29:35Z; supervisor's spend answer `083c1bf` 17:29:51Z; cell 1 launched **17:29:55Z**; cell 2 launched **17:56:30Z**. The §3 table therefore predates both live cells (by 90 s and 28 min) and no cell had produced data when it was written. By the literal conditions — neither cell has both `A` vs `U` separating *and* `A` vs `P` tying — **FAILED TO REPLICATE** is the only label the table permits. The worker did not relabel after seeing data, which is the right discipline.

**But it is not the honest summary, for two reasons that the ticket's own §9 notices without drawing out:**

- **The `f12-s0` "failure" is a stronger result for the adaptive arm.** The half that did not reproduce is the *non-detection* (`A` vs `P` tie), and at `f12-s0` that non-detection broke in `A`'s favour. §3 requires reproducing a null result obtained at 3 replicates, where the M2c record itself says the tie is "no detectable effect at 3 replicates" — a non-detection is not a reproducible property of the world, it is a property of the noise. The rule therefore maps "the effect grew" and "the effect vanished" onto the same label.
- **The `f10-s1` "failure" is a power failure, not an effect failure.** Every arm tied with the control there; the cell carried no information about any pair, so it neither replicated nor disconfirmed anything. Calling it a failed replication mistakes "unresolvable" for "absent" — the exact error ticket 41 §9 warned about, applied one level up.

**The summary sentence I would put in place of the bare label** (keeping the label, because the rule was pre-declared and may not be relabelled post hoc):

> Neither new cell reproduced M2c's exact pair pattern, but the pattern did not simply fail: at `f12-s0` both primary pairs separated in the adaptive arm's favour — the tie against periodic broke, thinly (1.67 starvation deaths against a 1-death threshold) — while at `f10-s1` nothing separated at all, including every arm against the no-AI control, because that cell has no range (all five arms within 45 actor-hours and one death of the control, against a 152 actor-hour noise band). The M2c result is therefore cell-dependent in both halves, and the decisive new fact is that in no cell of this project has the adaptive arm yet separated from doing nothing.

## 4. The falsifier and the honest end state

- The §4 falsifier fired as declared, and the record states the consequence: §9 "the specific pair pattern of `f10-s0` did not survive a change of cell"; §10.1 "It **does not support** 'the M2c result is a property of the mechanism rather than of `f10-s0`' … the answer at these two cells is no"; §10.1 also refuses the opposite over-read ("the mechanism is cell-specific and the M2c result was luck") and explains why two cells cannot distinguish it from seed noise. That is the correct scope statement.
- **M2c's row is not retroactively rewritten.** `docs/VERTICAL-SLICES.md:242` still reads "Answered 2026-10-18, ticket 41 — one primary pair separated and one tied … at one cell (`f10-s0`) … the stage is not 'passed' and its question is not settled in the general case". No document claims M2c's verdict was stable, and none rewrites it.
- **But the docs do not carry the new fact at all** (Finding P1-1). `docs/VERTICAL-SLICES.md:243` — the owning row for this stage — still says "**In progress, ticket 42**" and describes the deliverable in the future tense ("Deliverable: for each cell, whether … reported as replicated, partially replicated, or failed to replicate"), and `CHANGELOG.md` has no ticket-42 event (its last entries are tickets 39/40/41). The stage row's question ("Does any M2c separation reproduce at f12-s0 and other seeds?") is now answered: **partly — one pair yes, one pair no, and one cell could not tell.** A reader of the project's authoritative documents cannot discover that the first negative M2 result exists. This is the one substantive gap in the deliverable.

## 5. Run integrity — verified

| claim | evidence | result |
|---|---|---|
| 2 cells × 5 arms × 3 replicates | both artefacts `inventory {declared:15, ran:15, skipped:0}`, 15 cell objects each; scorer prints `inventory PASS — 15 of 15` | ✓ |
| 0 halted, 0 not-run | grep `"halted": true` / `"ran": false` → **no matches** in either artefact | ✓ |
| `realized_calls` per budgeted cell | `f12-s0`: `A` 249/248/248, `A1` 246/248/248, `P` 248×3, `U` 248/245/245 = **2969**; `f10-s1`: all 248 = **2976**; totals equal the claimed charged attempts exactly | ✓ |
| `inadmissible` per cell | the string appears **nowhere** in either run root (`fallback_reason` is the field; `35-admis.py:31-38`); `malformed` counts match §8's table per arm (`A`1, `A1`4, `P`1, `U`0 at `f12-s0`) | ✓ 0/0, no cell voided |
| spend $1.0666 | recomputed from the poll log's own token totals at $0.10/M in + $0.50/M out: in 1 866 152+1 869 796 = 3 735 948 → $0.3736; out 648 918+737 049 = 1 385 967 → $0.6930; **total $1.0666**; per cell $0.5111 / $0.5555 | ✓ inside `$1.30` stop, `$1.40` cap |
| reserve behaviour | 2 recorded timeouts at `f12-s0` (`A-f12-s0-r1:323`, `A1-f12-s0-r1:411`), absorbed; `A-r1` = 249 calls/248 decisions = 1 retry ≤ `R_t=8`; `f10-s1` 0 timeouts, 0 retries; no cell halted; ceiling 256 never reached | ✓ |
| pre-declaration predates the second cell's launch | `5f3d0a3` 17:28:25Z < launch 17:29:55Z (cell 1) < 17:56:30Z (cell 2) | ✓ |
| anchor determinism | `projection_hash` identical in all three `A0` rows at each cell (`e9b7c0f5…` / `00d8c5f9…`), 0 calls — matching §6 row 7's *post-correction* labels | ✓ |
| cumulative attempt cap | 3 020 + 2 948 + 5 945 = **11 913 vs the supervisor's 9 500** | breach, recorded (§10.5) — P2-5 |

Two small accounting loose ends: `U` realized only 245 decisions in 2 of 3 `f12-s0` replicates (shortfall 11, not the declared 8) with no explanation anywhere in the record, and §8's "max `attempts − decisions` per cell = 1" does not close against the totals (2969 attempts vs 2966 escalated hours, with only 2 timeouts recorded). Neither touches a verdict; both are exactly the "step that does not add up" that a record should close in one line (P2-6).

## 6. What M2 now knows, and what the next experiment has to be

**Cell-stable comparisons (3 cells: `f10-s0`, `f12-s0`, `f10-s1`):**

- **`A` vs `A0` (adaptive vs no AI): tie, tie, tie — 3 of 3, identical verdicts.** This is the most reproducible fact in the project and it is a negative one: the adaptive policy has never improved demographic viability over doing nothing. Everything else is a comparison between ways of *using* 248 calls, and one of those ways (uniform-random) is worse than not calling at all in 2 of 2 cells where it separated.
- **`U` vs `A0`: `A0` better at `f10-s0` and `f12-s0`, tie at `f10-s1`.** Uniform-random allocation harms viability where the world is tight and does nothing where it is not. Reproducible in sign at 2/2 informative cells.
- **`A` vs `U`: separates `A`-better at `f10-s0` (tier 2, 0.1544) and `f12-s0` (tier 3, 0.4333); tie at `f10-s1`.** Cell-stable in sign, unstable in significance, and the one M2c claim that survived — but only at cells where random does visible harm, i.e. `A` is "less harmful", not "better than doing nothing".
- **`A` vs `P`: tie, tie, separation (thin, A-favourable).** Not cell-stable in either direction; unresolved everywhere. **And the periodic baseline is not strong:** at `f10-s1` `P`'s three replicates are numerically identical to the control's (1496/2/0, 248 applied model decisions that changed no demographic indicator), and at `f12-s0` `P` is *worse* than the control (150 fewer actor-hours, 1.67 more starvation deaths). So "periodic is the baseline a selective allocator has to beat" overstates it — at two of three cells `P` is indistinguishable from doing nothing, and a tie with it carries little information.
- **`A` vs `A1` (`vs2-value/2` vs `vs2-value/1`): tie at both new cells**, with identical bands. The value model's version makes no detectable difference, so the "selective" effect is not attributable to the value model's discrimination.

**Why no single cell can carry it.** The instrument's own noise is the same size as the effect. Arm-internal replicate sd is 117–132 actor-hours (8–9 % of the control; `f12-s0` `A`: 1487/1677/1701 → sd 117; `f10-s1` `U`: 1395/1387/1163 → sd 132), and the declared band rule compares a *mean difference* against the *maximum single-replicate deviation* (`vrd2.py:249-252`), so the per-cell threshold is set by whichever arm happens to have one noisy triplet: observed thresholds 14.4 → 152 actor-hours across the three cells, a 10× spread from pure noise. The adaptive-vs-control gap is ≈78 actor-hours (5 %) at `f12-s0`; nothing in this design can resolve that against 8–9 % replicate noise.

**A properly powered next experiment, from these numbers:**

1. **Fix the instrument before adding cells.** `vrd2.py` hard-codes `H_T/B_T/S_T/N0` from `f10-s0` (`:55`, `:886-888`). At `f12-s0` that disabled tier 2 for four of five arms (`dH = 0` for every arm at or above 1444, including arms that beat the anchor) and divided deaths by 10 at a 12-founder cell. A replication can never test M2c's deciding tier at a different-founders cell until the anchor is per-cell. That is a criterion change, so it needs its own pre-registration, not a patch.
2. **Select cells for free.** `A0` is deterministic, 0 calls, seconds per cell. Screen all 12 declared primary cells with `A0` and pre-declare as primary only those where the control's own `S ≥ 1` and `B ≥ 1` — a control-only, arm-independent property, so it is not outcome selection. The data says why: `f10-s0` (`S=0`) and `f10-s1` (`S=0`) were structurally incapable of showing a starvation effect; `f12-s0` (`S=3`) was not.
3. **Replicates.** The max-deviation band at n=3 gives a threshold of ≈0.5–1.2σ; per-cell significance for a 3 % (≈45 actor-hour) effect needs **12–20 replicates per arm** under an SE-based band, or **~150** under the current max-deviation rule at `σ≈125`. Concretely: 15 reps × 3 arms (A, P, U) ≈ **$2.0 and ≈2 h per cell** at the measured `$0.0444` per model-bearing cell-replicate.
4. **Cheapest defensible design: many cells, few reps, paired across cells.** 8 pressure cells × 5 reps × 3 arms ≈ **$5.3 and ≈5 h**, testing the *direction* of `A − P` within each cell by sign/rank rather than demanding per-cell separation. A consistent 1.67-death / 78-actor-hour advantage over 8 cells is defensible where no single cell can separate; a per-cell-significance design for the same effect costs ~$40+ and half a day.
5. **Put `A0` in the primary pair set.** Three matrices have spent all their power on `A` vs `P` and `A` vs `U` while the question that decides the project — does selective allocation beat doing nothing — has never been asked with power in any cell.
6. **Declare a fourth label.** §3's three labels force an uninformative cell (`f10-s1`: all arms tie with the control) and an improved cell (`f12-s0`: the null broke) into the same bucket. The next pre-registration needs "**no resolution at this cell**" (pre-declared as: fewer than one declared pair separates from the control, so the cell cannot discriminate) beside REPLICATED / PARTIALLY REPLICATED / FAILED TO REPLICATE. Also declare a minimum detectable difference per cell up front, computed from the control's screen, and refuse to score a cell whose MDD exceeds the effect being sought.

---

## Findings

### P1-1 — The authoritative docs do not carry this ticket's result (`docs/VERTICAL-SLICES.md:243`, `CHANGELOG.md`)
The M2d stage row still reads "**In progress, ticket 42**" and states the deliverable in the future tense; `CHANGELOG.md`'s last event is ticket 41. The stage's own question is now answered (one pair reproduced, one pair broke, one cell unresolvable), and this is the project's first negative M2 result. The M2c row is fine and must not be rewritten — the gap is that nothing in the docs supersedes "in progress", so a docs reader cannot find the verdict at all. Precedent is explicit: ticket 41's branch updated its own stage row ("repair the M2c slice row") and CHANGELOG before merge.
*Smallest fix:* rewrite the M2d row's status with the two per-cell verdicts, the honest summary sentence from §3 above, and the pointer `.scratch/42-m2d-replication/spec.md` §9-§10 — and add one `CHANGELOG.md` bullet in the ticket-39/40/41 style. No source or Go change.

### P1-2 — The primary evidence is in git-ignored `.tmp`, while §9 claims it is "all under commit `HEAD`" (`spec.md` §9 evidence table; `.gitignore:2`)
`.gitignore` lists `.tmp/`, and `.scratch/42-m2d-replication/` holds only logs. So `.tmp/42-f12-s0-evaluation.json`, `.tmp/42-f10-s1-evaluation.json`, the run roots, `.tmp/42-*-curve.txt` and `.tmp/42-anchorcheck-*` are not in the branch and are deletable by the user at any time; the per-replicate vectors and the 5 945 call rows exist nowhere else (the spec's §8.1 vector table is a transcription — a second home for facts the artefact owns). Tickets 39 and 41 committed the equivalent (`/.scratch/41-m2c-matrix-rerun/m2c-evaluation.json`, `anchor-gate.json`), so this breaks the M2 stage's own pattern.
*Smallest fix:* commit a reduced extract per cell — exactly `vrd2.py`'s `LIVE_ROW_KEYS` per cell (`arm, replicate, ran, halted, alive_by_hour, births_cum, deaths_starve_cum, realized_calls`; ~11 KB/cell) into `.scratch/42-m2d-replication/`, which lets `vrd2.py --live` reproduce every number in §9; then correct the table's header to name which rows are durable and which are local-only.

### P2-1 — §8.1's "tier 2 cannot separate anything at `f12-s0`" is numerically false (`spec.md` §8.1, first bullet)
It states "every arm's every replicate exceeds 1 444 actor-hours, so `dH … is exactly 0 for all five arms`". `U`'s replicates are 1367/1422/1435, giving `dH_U = 0.0249 ± 0.0284` — which the adjacent score transcript prints. Tier 2 *did* evaluate `A` vs `U` there and missed separating by 0.0035.
*Smallest fix:* replace that clause with "`A`, `A1`, `P` and `A0` sit at 1 444 or above in every replicate (`dH = 0`); `U` (1 367/1 422/1 435) carries `dH = 0.0249 ± 0.0284`, inside its own band, so tier 2 separates nothing — narrowly for `A` vs `U`."

### P2-2 — The label is table-correct but misdescribes the two cells (headline question; `spec.md` §3, §9)
Discussed in §3 above. `FAILED TO REPLICATE` conflates an improved result (`f12-s0`) with an uninformative one (`f10-s1`), and it requires a non-detection to replicate.
*Smallest fix:* keep the label (pre-declared, may not be relabelled), add the one-sentence summary from §3 immediately after it in §9, and add the fourth label to the next pre-registration's vocabulary.

### P2-3 — The most decision-relevant facts are in the transcript but not in the framing (§8.2 diagnostics; §10)
`A` vs `A0` ties at all three cells, `P`'s `f10-s1` replicates are identical to the control's, and `P`/`U` are *worse* than the control at `f12-s0`. §10's scope guard ("No arm is called better than another") is correct but leaves the reader with "adaptive > random, unknown vs periodic" instead of "adaptive is no better than doing nothing anywhere, and the baselines it beats are themselves worse than doing nothing".
*Smallest fix:* one paragraph in §10 stating the `A`-vs-`A0` invariance across the three cells and that `f12-s0`'s separations are damage-avoidance relative to arms that harm the control's outcome — with the age-death context (13 of 16 deaths at `f12-s0` are age deaths) so the tier-3 statistic's scope is visible.

### P2-4 — A declared falsifier was not run in this ticket (§4 item 1 vs §6 row 6)
§4.1 declares "`vrd2.py --self-test` … **and — in `--expect-fail` mode — detect the inverted comparison as a FAIL**". §6 row 6 records only the `--self-test` run, and no `--expect-fail` transcript exists under `.scratch/42-m2d-replication/`. Coverage does exist at ticket 38 (`.scratch/38-m2c-preregistration/spec.md:789`) against the byte-identical unmodified scorer, so this is a declared-check-not-evidenced defect, not a hole in the argument.
*Smallest fix:* add to §6 row 6 either the `--expect-fail` run and its exit code, or a citation that ticket 38 gate 7 covers the unmodified scorer.

### P2-5 — The cumulative attempt cap was breached knowingly, by 2 413 attempts (`spec.md` §10.5)
The worker escalated correctly (its own §5.1 states 5 968 used against a 5 952–6 144 requirement, i.e. the supervisor's amended 9 500 was already unreachable), received "run the full matrix", executed it, and recorded 11 913 > 9 500 afterwards. Following the supervisor's explicit instruction ranks above the worker's inference, and the disclosure is honest — but a numeric guardrail known to be unsatisfiable *before* the spend should have been re-put to the supervisor in the same message that processed the answer, not reconciled after.
*Smallest fix:* none retroactively; for the next ticket, derive the stop rule from the cap at run time ("stop when remaining attempts < required attempts") so an inconsistent pair of numbers cannot both stand.

### P2-6 — Two accounting passages do not close (`spec.md` §8 table, `poll-log.md` poll 10)
(a) `U` realized 245 decisions in 2 of 3 `f12-s0` replicates — shortfall 11 against §2.4's declared 8 — with no cause recorded anywhere; (b) §8 asserts "max `attempts − decisions` per cell = 1", but the totals are 2 969 attempts against 2 966 escalated hours with only 2 timeouts recorded; (c) poll 10 asserts `A-f10-s1-r1` "has drawn on its `R_t = 8`", while the final artefact shows 248 calls / 248 decisions and no timeout row at that cell (§8 correctly says "no timeout").
*Smallest fix:* one sentence in §8 giving the decision total per cell and the cause of `U`'s shortfall (or "cause not established"), and a note in poll 10 that the predicted retry did not materialise.

---

## Verdicts

**(a) Are the numbers right? — Yes.** The four claimed statistics, the per-replicate vectors for every decisive arm, the means, the bands, the tier ordering, the anchor, the inventory, the spend (recomputed to the cent from token totals) and the pre-registration ordering all reproduce from the artefacts. The single factual error found (P2-1) changes no verdict and is contradicted two lines away by the scorer's own transcript.

**(b) Is "FAILED TO REPLICATE" the honest summary? — Not on its own.** It is the correct application of the pre-declared §3 table, and relabelling it after seeing data would be worse. But the honest reading is that **the M2c pattern is cell-dependent in both halves**: the `A`-vs-`U` advantage reproduced and grew at `f12-s0` and was unresolvable at `f10-s1`; the `A`-vs-`P` null held at `f10-s1` for want of power and broke, thinly, in `A`'s favour at `f12-s0`. One cell strengthened the claim, one could not see it, and the pre-declared vocabulary has no word for either. With the summary sentence in §3 above, the record becomes accurate; without it, a downstream reader will read a partly stronger, partly uninformative result as a disconfirmation.

**(c) Is the branch safe to integrate? — OK with notes.** No Go or behavioural change (17 commits, all record/doc; `gates.log` build/vet/gofmt/diff-check clean at the declaration revision), clean working tree at `6abd169`, spend and attempts inside the dollar caps, both cells fully inventoried with 0 halted and 0 inadmissible, and the record's own limits stated. **Merge verdict: OK with notes — fix P1-1 (stage row + CHANGELOG) and P1-2 (durable KPI extract) first**, since both are cheap and the SSOT would otherwise contradict the record. P2-1 to P2-6 are report-only.

**Commands a supervisor should run to close what I could not:** `git diff --stat d38a86b` (confirm no `*.go` change), `go build ./... && go vet ./... && git ls-files -z '*.go' | xargs -0 gofmt -l`, `python3 .scratch/38-m2c-preregistration/vrd2.py --check-inventory .tmp/42-f12-s0-evaluation.json` and `--check-inventory .tmp/42-f10-s1-evaluation.json`, `python3 .tmp/35-admis.py .tmp/42-runs-f12-s0 .tmp/42-runs-f10-s1`, and `python3 .scratch/38-m2c-preregistration/vrd2.py --expect-fail --scores .scratch/37-fidelity-metric/scores.json` (closes P2-4).