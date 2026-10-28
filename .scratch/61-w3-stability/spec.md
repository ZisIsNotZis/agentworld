# Ticket 58 — W3 Stage S: stability on `f11-s2` (`A` vs `P`/`U` at N=256, in-run `R@max`)

- Record kind: one live stage of the W3 pre-registration (`.scratch/58-w3-preregistration/spec.md`,
  committed `8f0f47b`; Stage S amended at `ac83ddd` to one cell, before any Stage-S cell), its
  recorded cells, the declared §3/§3.2 comparison, and the stability verdict. No world, policy,
  metric, contract, reference-model or code change.
- Author: worker, 2026-10-28.
- Source: the pre-registration's Stage S (§4, as amended 2026-10-28: `f11-s2` only) and comparison
  rule (§3, §3.2), declared before the first W3 live cell. This record applies that rule; it does
  not change it, and it pools no cell.
- Inputs: `W1` PASS (ticket 55, committed 128-cell no-AI grid
  `.scratch/55-w1-criterion/grid-128-w1.tsv`), Stage T on `f10-s0`
  (`.scratch/59-w3-timing/spec.md` — the pattern whose reproduction is tested), Stage E
  (`.scratch/58-w3-effort/spec.md`), `curve/1` (`cmd/vs2/curves.go`, ticket 35 — frozen, not
  edited).

## 0. Verdict

**The Stage-T pattern reproduces at `f11-s2`.** On cell `f11-s2` (11 founders, seed 2), `A@max`
**matches** `P@max` and **matches** `U@max`: no scalar KPI separates in either direction under the
declared rule. In fact the four conditions `A`, `P`, `U` and the no-AI `A0` are **identical on all
five declared scalar KPIs at every hour** (0 differing hours, endpoints 12 alive / 12 births /
0 starvation / 11 age / 2 max capital). The `U` ≡ `A0` identity of Stage T is reproduced exactly.

Per §3.2 the Stage-S verdict is therefore **not supported** — the same "all three tie" outcome as
`f10-s0`, now at a second seed and founder count:

| baseline | `A`'s outcome at `f11-s2` | because |
|---|---|---|
| `P` | **matches** | no KPI separates; the two curves are identical at every hour |
| `U` | **matches** | no KPI separates; the two curves are identical at every hour |

This is "**no detectable effect at this design**", never equivalence, and not "resolved": the
reported smallest separable difference is 0.000 on these pairs only because both sides are
deterministic — a degenerate limit, not high sensitivity (§5).

**The reproduction is stronger than a repetition of the Stage-T numbers.** At `f10-s0`, `A` and `P`
still differed from `A0` at 28 and 33 hours (sub-threshold, but present). At `f11-s2` they do not
differ from `A0` at all. The categorical pattern (A matches P and U; U identical to A0) reproduces;
the sub-threshold departures seen in the first cell *vanish* in this one.

**The A0 control reproduces the committed W1 reading.** All three `A0` replicates read
alive **12** / births **12** / starvation **0** / age **11** at H = 144 (max_capital 2), exactly the
`f11-s2` row of the committed no-AI grid (`.scratch/55-w1-criterion/grid-128-w1.tsv`:
`11  2  12  12  0  11  yes  -`). The budgeted arms collapse onto that same trajectory.

**The only scalar variation in the cell is `R@max` replicate 3.** `R` r1 and r2 are identical to
`A0` at every hour; `R` r3 differs at 7 hours (h138–h144) and ends 11 alive / 11 births. The paired
distance of every arm to the in-run `R@max` therefore reads `SNR` 0.816 (population/births,
max_capital), no 24-hour run, **no separation** (§4.3).

**No halt, no skip, no fallback.** 15/15 declared cells ran live in one launch; every call returned
HTTP 200 with `source: model`; zero decision errors, zero fallbacks, zero retries; the replay lane
re-ran all 12 priority cells `identical`.

## 1. What ran

One cell (`f11-s2`: 11 founders, seed 2), five arms `A0`, `R`, `P`, `U`, `A`, 3 replicates each,
model `github_copilot/gpt-6-luna`, wire `responses`, effort `max`, claim contract
`natural-claim/v4`, `H = 144`, `N = 256` for the budgeted arms (the compiled-in
`DeclaredCallCeiling`; no flag), output bound 32768, response bound 1 MiB, `-call-concurrency 10`,
`-parallel 3`, `-budget-min 600`, live gateway, spend authorised for this stage only. Unlike Stage T,
`R@max` is **drawn in this cell** (3 fresh replicates); it is the in-run reference, not a carry-in
from `f10-s0`.

```
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND LITELLM_BASE_URL=http://127.0.0.1:4000 \
LITELLM_MODEL=github_copilot/gpt-6-luna \
.tmp/61-vs2 -arms A0,R,P,U,A -cells f11-s2 -reps 3 -hours 144 -effort max \
  -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 3 -budget-min 600 \
  -root .scratch/61-w3-stability/runs \
  -out .scratch/61-w3-stability/evaluation.json \
  -out-txt .scratch/61-w3-stability/evaluation.txt
```

The exact invocation is `run.sh`; `starts.json` records launch epoch 2026-10-28T04:11:10Z on tree
`ac83ddd`. `evaluation.json` was written at 2026-10-28T07:20:44Z, i.e. stage wall **11,374.5 s =
3 h 10 min**, inside the stage's declared 6 h wall clock. One launch, one process; no cell halted,
none was resumed (`-resume` was never needed).

- **Matrix completeness: 15/15 declared cells, 3 replicates each, 0 skipped**
  (`evaluation.txt`: `CELL INVENTORY declared=15 ran=15 skipped=0`). Every cell carries a complete
  145-hour (h0..h144) series. Run identity: `status: live/real-model`, `live: true`, `model_id:
  github_copilot/gpt-6-luna`, `wire_mode: responses`, `declared_effort: max`, `claim_contract:
  natural-claim/v4`, `call_concurrency: 10`, mechanism v14 + `natural/14`, `H = 144`, `N = 256`,
  `value-model vs2-value/2`.
- **Replay:** all 12 priority cells (`R`, `P`, `U`, `A` × 3) re-ran from their ledgers through a
  dead-network transport and report `identical`; `replay_mismatches` is empty (`evaluation.txt`,
  `REPLAY SUBSET`). `A0` has no seam and no call, so it has no replay row.
- **No 429, no timeout, no retried attempt.** 6,684 physical attempts = 6,684 applied decisions
  (`calls.tsv`: every row `attempt = 1`, `status = ok`, `http = 200`, `source = model`). Decision
  errors and level-1/level-2 fallbacks are **0 in all 15 cells**.
- **`A0` control identity.** The cell's `A0` rows read 12 / 12 / 0 / 11 at H and are identical
  across their three replicates; this is the committed `f11-s2` W1 row (ticket 55). As in Stage T,
  the report's `VS2-EC-1` band reads `[FAIL]` because it compares this run's `A0` against the
  ticket-19 `natural/7` × `v6` recording, a different world version (`mismatches=f11-s2:
  alive_by_hour,branch_digest,event_body_bytes,events,history_bytes`). The control is instead pinned
  by the committed grid row and by the in-run `A0` rows; the committed hermetic test
  `go test ./cmd/vs2 -run TestDeclaredWorldA0ReproducesTicket55 -count=1` → `ok 4.7s` passes.
  Reported, not repaired (no code change in this stage).

## 2. Accounting

Recorded charge is **$0.00**: the gateway exposes no price entry for `github_copilot/gpt-6-luna`
(`price_version = unpriced`, `cost_micros = 0`). The derived price is arithmetic at the ticket-35
published rates ($0.10/M input, $0.50/M output), the same rates and method as Stages E/T/M. Wall is
launch to `evaluation.json` for the whole matrix; the per-arm column is the sum of its three cells'
recorded `duration_ms`.

| arm | calls (3 reps) | prompt tokens | completion tokens | derived $ | cell seconds | wake hours | calls/hour |
|---|---:|---:|---:|---:|---:|---:|---:|
| `A0` | 0 | 0 | 0 | 0.0000 | 66 | 0 | 0 |
| `R` | 4,452 | 2,800,139 | 2,716,989 | 1.6385 | 9,570 | 144 | ~10 |
| `P` | 744 | 467,913 | 456,580 | 0.2751 | 6,906 | 24 | ~10 |
| `U` | 744 | 468,060 | 409,450 | 0.2515 | 6,677 | 144 | 1–2 |
| `A` | 744 | 467,865 | 455,363 | 0.2745 | 6,888 | 144 | 1–2 |
| **fresh total (Stage S)** | **6,684** | **4,203,977** | **4,038,382** | **2.4396** | wall 11,374.5 s = 3 h 10 min | | |

Cumulative derived spend against the operator's ~$14: W2 $0.63 + Stage E $5.90 + Stage T $0.88 +
Stage M $1.30 + Stage S $2.44 = **$11.15**, i.e. ~$2.85 derived remains. The stage came in **under**
its ~$2.8 derived estimate. `f10-s1` was **not** run: the Stage-S amendment (2026-10-28, before any
Stage-S cell) narrowed the stage to `f11-s2` because one extra cell with its own `R@max` reference
fits the remaining budget and both declared cells do not. `f10-s1` remains a named follow-up.

### 2.1 Budget status of the budgeted arms (VS2-EC-2)

| arm | realized calls / rep | ceiling N | usable budget (N − 8 reserve) | under budget | halted | budget shortfall vs N | decision errors | fallbacks / inadmissible |
|---|---:|---:|---:|---|---|---:|---:|---:|
| `P` | 248 (×3) | 256 | 248 | true | false | 8 | 0 | 0 |
| `U` | 248 (×3) | 256 | 248 | true | false | 8 | 0 | 0 |
| `A` | 248 (×3) | 256 | 248 | true | false | 8 | 0 | 0 |

`evaluation.txt`: `budgeted cells=3 ceiling N=256 over-ceiling=0 under-budget=3 missing-ceiling-rows=0
ceiling-halts=0` → `[PASS] VS2-EC-2`. **`P` realizes the whole usable budget here (248), unlike
`f10-s0` (240).** The difference is cell-specific: the 6-hour gate has 24 wake hours in both cells,
but at `f11-s2` it finds 248 candidates inside them, so no shortfall arises; there is no truncation
and no halt. This removes the 8-call realized-budget asymmetry that Stage T reported — the three
budgeted arms are realized-equal at `f11-s2` (248 each), while the declared budget (`N = 256`) is
equal by construction.

`R` realizes 1,480 / 1,486 / 1,486 calls against its 4,096 fail-loud ceiling; it is a full-universe
reference and is never truncated. Every call in the matrix returned `source: model`, `status: ok`,
HTTP 200, first attempt.

**What the model did:** the answer equalled the frozen cascade action in **99.7 %** of calls —
`R` 4,439/4,452, `P` 743/744, `U` 742/744, `A` 742/744. Model deviations from the frozen action:
`R` 13 (r1 2, r2 4, r3 7), `P` 1 (r2), `U` 2 (r1, r2), `A` 2 (r1, r2). The cell's scalar
invariance is therefore measured on a model that almost never departs from the policy's own action;
the handful of departures are recorded below and do not move the five scalars (§6).

## 3. The rule applied (declared in §3; no new rule)

`analyze.py` implements the pre-registered `curve/1` statistics literally over the recorded
evaluation JSONs (read-only, no cell, no model call). Its `paired()`/`power()` code is the Stage-E /
Stage-T implementation unchanged, so the three stages' numbers are comparable. It was re-verified on
the Stage-T file before this cell's data was used: `A−P` population 0.288 / run 6 @134, `A−U` 0.914 /
run 14 @131, `U` = `A0` at 0 hours — identical to the Stage-T record.

- `d(t) = mean_A(t) − mean_B(t)` over each condition's 3 replicates; `RMSE(d)` over hours.
- `noise` = RMSE over hours of the pooled within-condition SD at each hour;
  `SNR = RMSE(d)/noise`, `n/a` when both conditions are deterministic (never ∞).
- a **separation** at hour `t` iff `|d(t)|` exceeds both conditions' replicate envelope half-width; a
  KPI **separates** iff `SNR ≥ 2` **and** some separating run is `≥ 24` consecutive hours.
- the verdict is per scalar KPI, `{population, births_cum, deaths_starve_cum, deaths_age_cum,
  max_capital}`; `wages_cum` is carried by the instrument but is not a §3 scalar and never enters a
  verdict; the histogram families are carried by Wasserstein-1, not by the scalar rule.
- **power** (smallest separable difference): a constant offset δ gives `RMSE(d) = δ`, so the SNR
  gate needs `δ ≥ 2·noise`; the 24-hour gate needs δ to exceed the band at every hour of some
  24-hour window. The reported power is the larger floor.
- **direction** is description only (population/births higher is better; starvation/age deaths
  lower); nothing is summed or ranked, and no scalar "winner" exists.
- **histogram distance** is computed here from the per-hour bucket vectors (each condition's hourly
  vector = mean count over its 3 replicates, normalised to a distribution over bucket indices; W1 in
  bucket units per hour, averaged over hours), independently of the instrument's histogram column.
- **no pooling.** The reference is the in-run `R@max` of `f11-s2`. `f10-s0` is cited only as the
  pattern under test (§5) and as a descriptive cross-cell endpoint table (§4.5), never as data in a
  paired statistic here.

**Cross-check.** The frozen instrument over this evaluation file (`curve-report.txt`) prints the
same scalar rows: `A@max` vs `P@max` and `A@max` vs `U@max` are 0.0000 / 0.0000 / `n/a` / `-1` / 0 on
all six scalar columns; `A@max` vs `R@max` population 0.0732 / 0.0897 / 0.8165 / `-1` / 0. They are
identical to `analyze.py`'s, and every pair is reported "NOT distinguishable at this noise floor".

## 4. KPI tables

In-run reference `R@max`: 3 replicates (r1, r2, r3). At H its population mean is 11.67 (envelope
11..12), births 11.67 (11..12), starvation 0 (0..0), age deaths 11 (11..11), max_capital 2 (2..2).
Only r3 ends at 11; r1 and r2 end at 12. The replicate noise of the reference is therefore the only
noise in the three compared pairs below.

### 4.1 `A@max` − `P@max`, N = 256 both sides, cell `f11-s2`, 3 replicates each

`first` = first separating hour (−1 none); `run` = longest separating run, `@` its start; `power` =
smallest separable constant difference; `dir` = direction of a *detected* separation (blank = none).

| KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | dir | W1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| population | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| births_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| max_capital | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| `capital_hist` | – | – | – | – | – | – | – | – | – | 0.000 |
| `granary_hist` | – | – | – | – | – | – | – | – | – | 0.000 |

**The two curves are identical at every hour.** No KPI separates; there is no difference to detect.

### 4.2 `A@max` − `U@max`, N = 256 both sides, cell `f11-s2`, 3 replicates each

| KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | dir | W1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| population | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| births_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| max_capital | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| `capital_hist` | – | – | – | – | – | – | – | – | – | 0.000 |
| `granary_hist` | – | – | – | – | – | – | – | – | – | 0.000 |

**The two curves are identical at every hour.** No KPI separates.

### 4.3 Each arm's distance to the in-run `R@max` (§3.1, same cell and seeds)

`A`, `P`, `U` and `A0` have identical curves in this cell, so their four rows are numerically
identical; the table states all four rather than pretending they are independent measurements.

| arm | KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | W1 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `A`,`P`,`U`,`A0` | population | 0.073 | 0.090 | 0.816 | −1 | 0 | 0.179 | 0.333 | +0.333 | 0.016 |
| `A`,`P`,`U`,`A0` | births_cum | 0.073 | 0.090 | 0.816 | −1 | 0 | 0.179 | 0.333 | +0.333 | 0.016 |
| `A`,`P`,`U`,`A0` | deaths_starve_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | 0.000 |
| `A`,`P`,`U`,`A0` | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | 0.000 |
| `A`,`P`,`U`,`A0` | max_capital | 0.039 | 0.048 | 0.816 | −1 | 0 | 0.096 | 0.333 | 0.000 | 0.005 |
| `A`,`P`,`U`,`A0` | `capital_hist` | – | – | – | – | – | – | – | – | 0.001 |
| `A`,`P`,`U`,`A0` | `granary_hist` | – | – | – | – | – | – | – | – | 0.003 |

**No arm separates from `R@max`.** The +0.333 endpoint gap is `A`/`P`/`U`/`A0` at 12 vs `R`'s
3-replicate mean 11.67 — one reference replicate, `R` r3. `R` r1 and r2 are identical to `A0` at
every scalar hour, so the "distance to the reference" is almost entirely the reference's own
replicate spread; `SNR` 0.816 with no separating run. The histogram distances are correspondingly
tiny (capital 0.001, granary 0.003 bucket units per hour).

### 4.4 Curve agreement (descriptive; no threshold, no verdict)

Hours at which two conditions' declared scalars differ *at all* (first replicate). Within each arm
the three replicates have identical scalar series (0 differing hours), and in this cell the four
non-`R` arms share a scalar series:

| pair | hours differing | first | last | | pair | hours differing | first | last |
|---|---:|---:|---:|---|---:|---:|---:|
| `A` vs `P` | **0** | – | – | | `P` vs `A0` | **0** | – | – |
| `A` vs `U` | **0** | – | – | | `P` vs `R` | **0** | – | – |
| `A` vs `A0` | **0** | – | – | | `U` vs `A0` | **0** | – | – |
| `A` vs `R` | **0** | – | – | | `U` vs `R` | **0** | – | – |
| `P` vs `U` | **0** | – | – | | `A0` vs `R` | **0** | – | – |

The table uses `R` r1; `R` r3 is the one exception — it differs from the common curve at 7 hours
(h138–h144), ending 11 alive / 11 births. The other two `R` replicates do not differ at any hour.

### 4.5 Endpoints (mean over each condition's 3 replicates)

| condition | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| `R@max` (in-run) | 11.67 | 11.67 | 0.00 | 11.00 | 2.00 |
| `A@max` | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |
| `P@max` | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |
| `U@max` | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |
| `A0@max` | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |

Cross-cell endpoints for the four comparable arms (descriptive only; two different world draws, no
pooling, no test):

| arm | `f10-s0` alive/births/starve/age/maxcap | `f11-s2` alive/births/starve/age/maxcap | `f11-s2` − `f10-s0` |
|---|---|---|---|
| `A0` | 11 / 12 / 1 / 10 / 1 | 12 / 12 / 0 / 11 / 2 | +1 / 0 / −1 / +1 / +1 |
| `P` | 13 / 15 / 2 / 10 / 1 | 12 / 12 / 0 / 11 / 2 | −1 / −3 / −2 / +1 / +1 |
| `U` | 11 / 12 / 1 / 10 / 1 | 12 / 12 / 0 / 11 / 2 | +1 / 0 / −1 / +1 / +1 |
| `A` | 14 / 15 / 1 / 10 / 1 | 12 / 12 / 0 / 11 / 2 | −2 / −3 / −1 / +1 / +1 |

## 5. Stage power and the verdict per KPI

**`A` vs `P` and `A` vs `U`** (identical-world comparisons, so the replicate noise is 0):

| pair | KPI | power | observed max abs d | sustained run | reading |
|---|---|---:|---:|---:|---|
| `A−P` | all five scalars | 0.000 (degenerate) | 0.000 | 0 | the curves are the same curve |
| `A−U` | all five scalars | 0.000 (degenerate) | 0.000 | 0 | the curves are the same curve |

The reported `power = 0.000` is a **degenerate limit**, not high sensitivity: the replicate band is
zero-width because every replicate of each arm follows the same scalar trajectory, and the two
conditions are the same trajectory. No difference exists to resolve, so this cell cannot support a
"beats" claim and does not need to — §3.2 classifies an exact tie as **matches**.

**Distances to `R@max`** (the reference comparison carries the only real power, because `R` is the
only condition with replicate noise):

| KPI | smallest separable (power) | observed max abs d | reading |
|---|---:|---:|---|
| population | 0.179 | 0.333 | `SNR` 0.816, no separating hour: not detectable |
| births_cum | 0.179 | 0.333 | same |
| deaths_starve_cum | 0.000 | 0.000 | deterministic 0 in all 15 cell rows: no information |
| deaths_age_cum | 0.000 | 0.000 | deterministic 11 in all 15 cell rows: no information |
| max_capital | 0.096 | 0.333 | `SNR` 0.816, no separating hour |
| `granary_hist` | – (no scalar test) | – | W1 0.003 |
| `capital_hist` | – (no scalar test) | – | W1 0.001 |

**Verdict, per §3.2 (applied literally):**

| baseline | `A`'s outcome | because |
|---|---|---|
| `P` | **matches** | no KPI separates in either direction (identical curves) |
| `U` | **matches** | no KPI separates in either direction (identical curves) |

**Stage-S verdict on `f11-s2`: not supported** (the "all three tie" category). `A` beats neither
baseline; no baseline beats `A`. `unresolvable here` does **not** apply: that category is for a
detection limit exceeding every observed separation, and the paired arms have no separation at all
(they are the same curve), while the reference distance is below its own power floor.

**Reproduction of the Stage-T `f10-s0` pattern:**

| pattern element | `f10-s0` (Stage T, cited) | `f11-s2` (this cell) | reproduces? |
|---|---|---|---|
| `A` vs `P` | **matches**; 13 hours differ (h122–h144), max run 6 h | **matches**; 0 hours differ | **yes** |
| `A` vs `U` | **matches**; 28 hours differ (h110–h144), max run 14 h | **matches**; 0 hours differ | **yes** |
| `U` vs `A0` | **identical**, 0 hours differ | **identical**, 0 hours differ | **yes** |
| overall | A matches P and U; U identical to A0 | A matches P and U; U identical to A0 | **reproduces** |

The cells/seeds are named and not pooled: `f10-s0` = 10 founders, seed 0; `f11-s2` = 11 founders,
seed 2. The categorical result — the pattern §3.2 must see to say anything about stability — is
reproduced. The *magnitude* of the sub-threshold structure is not: the 13- and 28-hour departures of
`f10-s0` are absent here, so the second cell is a cleaner null (all four non-`R` arms coincide with
`A0`), not a second instance of the same small departures. That distinction is recorded, not
smoothed: a stability claim is about the verdict category, and this table is why the category is
called reproduced while the first cell's sub-threshold differences are not.

**M_end is not delivered by this stage.** §3.2's "supported" requires `A` to beat both `P` and `U`
in every declared cell and the pattern to reproduce at a second cell. The pattern reproduces, but it
is a tie, so the equal-budget claim remains **not supported** — now at two cells (`f10-s0` and
`f11-s2`), with `f10-s1` untested by the Stage-S amendment.

## 6. Limits

- **The cell is more collapsed than `f10-s0`, not merely similar.** All 12 `A`/`P`/`U`/`A0` cells
  are scalar-identical at every hour. This is a real measurement of these KPIs on this world draw,
  not a stuck RNG: the arms wake on different hours and select different actors by construction
  (`P` ~10 actors on 24 hours, `U`/`A` 1–2 actors on 144 hours), and the cell records 5 model
  deviations from the frozen action across `A`/`P`/`U` (and 13 in `R`). The deviations are non-binding for the five scalars at this cell (the model's `wait` /
  `eat-stored` choices at h35/h126 and the late `eat-stored` choices alter no aggregate KPI); the
  `R` r3 deviations at h125–h139 are the only ones that change an endpoint, by one alive.
- **The 3-replicate band is the limit, not the sample.** The scalars are deterministic within a
  condition here (`noise = 0` for every `A`/`P`/`U`/`A0` row), so `SNR` is undefined for the
  equal-budget pairs and the 24-hour clause is the only live gate. For the reference distance the
  only noise is `R`'s own replicate spread (population SD 0.090), and the power is 0.179; the
  observed 0.333 endpoint gap is below it in sustained terms (`SNR` 0.816).
- **Timing is confounded with call count and placement by design** — that is the axis. `P` looks at
  24 hours × ~10 actors; `U`/`A` at 144 hours × 1–2 actors. The record does not separate "when" from
  "how many per hour"; the pre-registration did not ask it to.
- **The model barely changes its answer**: 99.7 % of calls returned the frozen cascade's own action
  (13/4,452 `R`; 1/744 `P`; 2/744 `U`; 2/744 `A`). The null has a mechanism, not just a power
  story: when the answer almost never depends on when it is asked, *when* it is asked cannot move
  the curve. This is model-conditional (`luna` at `max`, `natural-claim/v4`); it says nothing about
  whether another model would behave the same (Stage M measured `sol` as matching on `f10-s0`).
- **One cell.** `f11-s2` only, per the pre-registration amendment. The stage's 6 h wall clock and
  the ~$2.8 estimate bounded the design to one cell with its own reference; `f10-s1` is a named
  follow-up, not a silent omission.
- **The reference is stochastic and one replicate carries the gap.** `R` r3 is the only cell whose
  scalar curve differs from the common one (7 late hours); `R` r1/r2 are identical to `A0`. The
  reference distance therefore describes one draw as much as the condition, and the band is
  estimated from 3.
- **`VS2-EC-1` reads FAIL for a version reason** (same as Stage T): the band compares this run's
  `A0` against the ticket-19 `natural/7` × `v6` recording; the world here is `natural/14` × `v14`.
  The `A0` control is pinned by the committed grid row and by the committed hermetic test.
- **`deaths_age_cum` and `wages_cum` carry no information here** (11 and 0 in every cell row);
  `SNR` is `n/a (both deterministic)`, not ∞.
- **Synthetic, model-conditional.** Nothing here is a claim about real civilizations, and no
  condition is called better.
- **The amended stage cannot deliver M_end on its own.** One cell answers "does the pattern
  reproduce"; §3.2's "supported" also requires `A` to beat both baselines, which it does not at
  either cell.

## 7. Artefacts

| file | what it is |
|---|---|
| `spec.md` | this record |
| `evaluation.json`, `evaluation.txt` | the run's own report: 15 cells, per-cell realized calls / under-budget / halt, bands, replay, headroom |
| `calls.tsv` | one row per recorded Role A call (6,684): cell, seq, hour, actor, action, frozen action, source, status, http, attempt, latency, tokens, request hash |
| `kpi-series.tsv` | the per-hour KPI series (five scalars + two histogram families) for all 15 cells |
| `agreement.json` | per arm: calls, `action == frozen_action` counts, source/status sets, wake hours, latency p50/p95/max |
| `analysis.md`, `analysis.json` | the §3 / §3.2 comparison, power, direction, distance to the in-run `R@max`, curve agreement, A0 control, cross-cell endpoints and the reproduction verdict, produced by `analyze.py` |
| `curve-report.txt` | the frozen `curve/1` report over this evaluation (`-curve-in`), all ten pairs; scalar rows identical to `analyze.py` |
| `analyze.py` | the read-only §3 analysis (pairing, SNR, separation, power, W1) |
| `summarize.py` | regenerates `calls.tsv`, `kpi-series.tsv`, `agreement.json` from a cell's ledger and evaluation |
| `accounting.py`, `wall.json`, `starts.json` | calls, tokens, derived cost, wall clock per arm; the launch epoch and tree |
| `run.sh` | the exact invocation |
| `runs/<arm>-f11-s2-r<N>/natural-{root,branch-enabled}.bundle` | the declared genesis each cell ran from (30 KB each) |

**Not committed, deliberately:** the raw hash-chained ledgers (12 priority cells, ~300 MB
uncompressed under `runs/`) and each cell's final checkpoint bundle. They were removed after
extraction; the world state is bound by the `tip_hash` / `projection_hash` in `evaluation.json`, the
replay verdict is recorded there, and `calls.tsv` is the human-auditable substitute. Re-deriving the
state from a ledger needs a re-run. Stage T took the same decision for the same reason.

## 8. Reproduction

Toolchain: `cd /home/z/vibe/agentworld && . .tmp/go-env.sh` (go1.24.13), then
`go build -o .tmp/61-vs2 ./cmd/vs2` from tree `ac83ddd`.

```
# the live cell (spend-gated; $2.44 derived, 3 h 10 min). One invocation, 15 cells:
sh .scratch/61-w3-stability/run.sh

# read-only analysis (no cell, no spend)
python3 .scratch/61-w3-stability/summarize.py --dir .scratch/61-w3-stability
python3 .scratch/61-w3-stability/accounting.py --dir .scratch/61-w3-stability \
  --starts .scratch/61-w3-stability/starts.json --out .scratch/61-w3-stability/wall.json
python3 .scratch/61-w3-stability/analyze.py \
  --eval .scratch/61-w3-stability/evaluation.json \
  --timing-eval .scratch/59-w3-timing/evaluation.json \
  --wall .scratch/61-w3-stability/wall.json \
  --out .scratch/61-w3-stability/analysis.md --json .scratch/61-w3-stability/analysis.json

# the frozen instrument over the same file (scalar rows are the cross-check)
.tmp/61-vs2 -curve-in .scratch/61-w3-stability/evaluation.json \
  -curve-out .scratch/61-w3-stability/curve-report.txt

# the A0 control's identity on the declared world (hermetic, no spend)
go test ./cmd/vs2 -run TestDeclaredWorldA0ReproducesTicket55 -count=1

# no code changed by this stage; the declared gate still holds
go build ./...
```
