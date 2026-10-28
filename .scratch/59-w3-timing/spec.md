# Ticket 58 — W3 Stage T: timing (`A` vs `P`/`U` at N = 256, every arm against `R@max`)

- Record kind: one live stage of the W3 pre-registration (`.scratch/58-w3-preregistration/spec.md`,
  committed `8f0f47b`), its recorded cells, the declared §3 comparison and §3.2 claim comparison, and
  the stage verdict. No world, policy, metric, contract, reference-model or code change.
- Author: worker, 2026-10-28.
- Source: the pre-registration's Stage T (§4) and comparison rule (§3/§3.2), declared before the
  first W3 live cell. This record applies that rule; it does not change it.
- Inputs: the `R@max` reference rows of Stage E (`.scratch/58-w3-effort/spec.md` §1 — three
  replicates, **not re-run here**), `curve/1` (`cmd/vs2/curves.go`, ticket 35 — frozen, not edited),
  `W1` PASS (ticket 55), `W2` (ticket 57).

## 0. Verdict

**Not supported at this cell.** Under the declared §3.2 rule, `A` **matches** both `P` and `U`: no
declared scalar KPI separates in either direction (no KPI holds `SNR ≥ 2` together with a
`≥ 24 h` separating run), and no baseline separates against `A`. `A` is not worse anywhere and not
better anywhere that the rule can resolve.

This is "**no detectable effect at this design**", never equivalence. Two facts must be read beside
the verdict, because they explain it:

1. **The replicates carry no noise on the declared scalars.** Within every arm the three replicates
   produce *identical* scalar KPI series (0 differing hours; `kpi-series.tsv`) — while genuinely
   differing in their decisions (`A`'s replicates deviate from the frozen action at 5 / 5 / 6 calls
   and select different actors at 60 hours between r1 and r3; `A`'s completion tokens differ 158,504
   / 169,044 / 181,263). So the paired rule's noise denominator is exactly 0 and `SNR` is
   `n/a (both deterministic)` for `A−P` and `A−U` — the Go instrument reports the same. The `SNR`
   gate therefore cannot fire, and the separation test
   rests on the `≥ 24 h` sustained clause alone: the longest separating run is **14 h** (`A−U`,
   population and births, h131–h144) and **6 h** (`A−P`, h134–h139). Both are short of 24 h, so the
   verdict would be the same even if `SNR` were taken as infinite.
2. **The two worlds are nearly the same world.** `U`'s trajectory is *identical to the no-AI `A0`
   control at every hour* (0 differing hours), `A` differs from `U`/`A0` at 28 hours (from h110) and
   from `P` at 13 hours (from h122). Mechanically: the model's answer equalled the frozen cascade
   action in **98.0 %** (`P`), **99.9 %** (`U`) and **97.9 %** (`A`) of calls. When the answer almost
   never depends on when it is asked, *when* it is asked cannot move the curve far.

**`A0` control reading:** the no-AI cells ran with zero cognition, zero calls and identical scalar
series across their three replicates (endpoints 11 alive / 12 births / 1 starvation / 10 age deaths
/ 1 max capital at H); `A0` is the arm the budgeted arms collapse onto — `U` exactly, `P` at 33
hours, `A` at 28.

Stage-T cost: **2208 calls, $0.8803 derived** (recorded charge $0.00 — the gateway is unpriced),
wall **1 h 47 min** for the 12-cell matrix, inside the stage's 6 h declared wall clock.

## 1. What ran

One cell (`f10-s0`, 10 founders, seed 0), arms `A0`, `P`, `U`, `A`, 3 replicates each, model
`github_copilot/gpt-6-luna`, wire `responses`, effort `max`, claim contract `natural-claim/v4`,
`H = 144`, `N = 256` (the compiled-in `DeclaredCallCeiling` of the budgeted arms; no flag), output
bound 32768, response bound 1 MiB, `-call-concurrency 10`, `-parallel 3`, live gateway, spend
authorised for this stage only. `R` was **not** run: §2 of the pre-registration declares the Stage-E
`R@max` three rows (`t57max.R.r1`, `w3max.R.r1`, `w3max.R.r2`, same cell, same world digests, same
condition) as the reference.

```
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND LITELLM_BASE_URL=http://127.0.0.1:4000 \
LITELLM_MODEL=github_copilot/gpt-6-luna \
.tmp/59-vs2 -arms A0,P,U,A -cells f10-s0 -reps 3 -hours 144 -effort max \
  -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 3 -budget-min 600 \
  -root .scratch/59-w3-timing/runs \
  -out .scratch/59-w3-timing/evaluation.json -out-txt .scratch/59-w3-timing/evaluation.txt
```

The exact invocation is kept as `run.sh`; `starts.json` records the launch epoch (18:19:51 UTC).
The binary was built from `d743856` (the tree at launch); commit `ed29dad` landed at 18:26 UTC, while
this run was in flight, and touched no world, arm, ledger-history or verdict statistic — §3 records
what it changed and how it was verified.

- **Matrix completeness: 12/12 declared cells, 3 replicates each, 0 skipped**
  (`evaluation.txt`: `CELL INVENTORY declared=12 ran=12 skipped=0`). Every cell reports a complete
  145-hour (h0..h144) series. `status: live/real-model`, `live: true`, `model_id:
  github_copilot/gpt-6-luna`, `wire_mode: responses`, `declared_effort: max`,
  `claim_contract: natural-claim/v4`, `call_concurrency: 10`.
- **Replay:** the in-process replay lane re-ran all nine priority cells from their ledgers through a
  dead-network transport; every cell reports `identical`, `replay_mismatches` empty
  (`evaluation.txt`, `REPLAY SUBSET`). `A0` has no seam and no calls, so it has no replay row.
- **No deviations and no interruptions.** The matrix ran as declared in one launch (3 cells
  concurrently × call concurrency 10; the budgeted arms issue one call at a time by construction, so
  the real in-flight load was ≤ 3). No `429`, no timeout, no retried attempt: 2208 physical attempts
  = 2208 applied decisions. The stage's wall clock was 1 h 47 min (cell segments 1.83–2.32 ks each).
  *Provenance note:* the worker's session hung and the host suspended ~2 min after the run wrote its
  evaluation; that affected only the record-keeping turn, not any cell — no cell was resumed, and
  `-resume` was never needed.
- **`A0` identity evidence.** The report's `VS2-EC-1` band reads `[FAIL]` with
  `A0 cells checked=1 … mismatches=f10-s0:branch_digest,events,…`. That band compares `A0` against
  the *ticket-19 `natural/7` × `v6`* recording, which is a different world version than this stage's
  `natural/14` × `v14`, so the comparison is not evaluable as a pass/fail here (Stage E, which ran no
  `A0` cell, got `not-evaluable`). The `A0` control is pinned instead by the committed hermetic test
  `go test ./cmd/vs2 -run TestDeclaredWorldA0ReproducesTicket55 -count=1` → `ok 17.2s`, and by this
  run's own `A0` rows (zero calls, identical across replicates). Reported, not repaired (no code
  change in this stage).

## 2. Accounting

Recorded charge is **$0.00**: the gateway exposes no price entry for `github_copilot/gpt-6-luna`
(`price_version = unpriced`, `cost_micros = 0`). The derived price is arithmetic at the ticket-35
published rates ($0.10/M input, $0.50/M output), the same rates and method as Stage E. Wall is
launch to `evaluation.json` for the whole matrix; the per-arm column is the sum of its three cells'
own recorded `duration_ms`.

| arm | calls (3 reps) | prompt tokens | completion tokens | derived $ | cell seconds | wake hours | calls/hour |
|---|---:|---:|---:|---:|---:|---:|---:|
| `A0` | 0 | 0 | 0 | 0.0000 | 9 | 0 | 0 |
| `P` | 720 | 452,739 | 484,607 | 0.2876 | 6,443 | 24 | 10 |
| `U` | 744 | 467,727 | 489,415 | 0.2915 | 6,149 | 144 | 1–2 |
| `A` | 744 | 467,802 | 508,811 | 0.3012 | 6,103 | 144 | 1–2 |
| **fresh total (Stage T)** | **2,208** | **1,388,268** | **1,482,833** | **0.8803** | wall 6,443.7 s = 1 h 47 min | | |
| `R@max` (Stage-E rows, not re-run) | 4,574 | 2,876,424 | 3,185,073 | 1.8802 | (previously recorded) | 144 | ~10 |

Cumulative derived spend against the operator's ~$14: W2 $0.63 + Stage E $5.90 + Stage T $0.88 =
**$7.41**, i.e. ~$6.6 derived remains for Stages M and S.

### 2.1 Budget status of the budgeted arms (VS2-EC-2)

| arm | realized calls / rep | ceiling N | usable budget (N − 8 reserve) | under budget | halted | budget shortfall vs N | decision errors | fallbacks / inadmissible |
|---|---:|---:|---:|---|---|---:|---:|---:|
| `P` | 240 (×3) | 256 | 248 | true | false | 16 | 0 | 0 |
| `U` | 248 (×3) | 256 | 248 | true | false | 8 | 0 | 0 |
| `A` | 248 (×3) | 256 | 248 | true | false | 8 | 0 | 0 |

`evaluation.txt`: `budgeted cells=3 ceiling N=256 over-ceiling=0 under-budget=3 missing-ceiling-rows=0
ceiling-halts=0` → `[PASS] VS2-EC-2`. Every call returned HTTP 200 with `source: model`
(`agreement.json`, `calls.tsv`): **zero fallback, zero malformed/inadmissible answer, zero
content-policy refusal.** The model's answer departed from the frozen cascade action at 14 of 720
`P` calls, 1 of 744 `U` calls and 16 of 744 `A` calls; per replicate (`A`: 5 / 5 / 6 deviations,
`P`: 6 / 4 / 4, `U`: 0 / 0 / 1).

**Realized-budget asymmetry (state it, do not smooth it).** `U` and `A` realize **248** = the whole
usable budget (`N − 8` transport reserve, ticket 40); `P` realizes **240**, because its declared
6-hour gate sees only 24 wake hours (h0, 6, …, 138) before `H = 144`, and its per-hour quota
(8–13 selected per gated hour) exhausts those hours first. The difference is **8 calls (3.3 %)**,
arises from `P`'s declared period gate, and involves no truncation or halt: `P`'s ledger has zero
allocation shortfall at every gated hour. The comparison is therefore "equal declared budget
(N = 256), realized 240 vs 248"; a reader who wants realized-equal arms cannot have it in this
declared design, and this record does not re-frame the rule.

## 3. The rule applied (declared in §3; no new rule)

`analyze.py` implements the pre-registered `curve/1` statistics literally over the recorded
evaluation JSONs (read-only, no cell, no model call) and reproduces the frozen instrument's scalar
columns exactly (e.g. `A@max` vs `P@max` population: `rmse_d` 0.2877, noise 0.0000, `snr` n/a,
firstsep 122, run_len 6 — identical to `curve-report.txt`). Its `paired()`/`power()` code is the
Stage-E implementation unchanged, so the two stages' numbers are comparable; it was re-verified
against Stage E's published rows (`R@none` vs `R@max` population: 1.0446 / 0.8318 / 1.2558 / 124 / 9).

- `d(t) = mean_A(t) − mean_B(t)` over each condition's 3 replicates; `RMSE(d)` over hours.
- `noise` = RMSE over hours of the pooled within-condition SD at each hour;
  `SNR = RMSE(d)/noise`, `n/a` when both conditions are deterministic (never ∞).
- a **separation** at hour `t` iff `|d(t)|` exceeds both conditions' replicate envelope half-width; a
  KPI **separates** iff `SNR ≥ 2` **and** some separating run is `≥ 24` consecutive hours.
- the verdict is per scalar KPI, `{population, births_cum, deaths_starve_cum, deaths_age_cum,
  max_capital}`; `wages_cum` is carried by the instrument but is not a §3 scalar and never enters a
  verdict; the histogram families are carried by Wasserstein-1, not by the scalar rule.
- **power** (smallest separable difference): a constant offset δ gives `RMSE(d) = δ`, so the SNR
  gate needs `δ ≥ 2·noise`; the 24-hour gate needs `δ` to exceed the band at every hour of some
  24-hour window. The reported power is the larger floor.
- **direction** is description only (population/births higher is better; starvation/age deaths
  lower); nothing is summed or ranked, and no scalar "winner" exists.
- **histogram distance** is computed here from the per-hour bucket vectors (each condition's hourly
  vector = mean count over its 3 replicates, normalised to a distribution over bucket indices; W1 in
  bucket units per hour, averaged over hours). This is computed independently so the record does not
  rest on the instrument's histogram column: `curves.go` computed
  `wassersteinHistograms(histRows(...))` over replicate-major series, so the index it integrated over
  was the hour index, not the bucket index (printed values exceeded the bucket range, e.g. 20.14 for a
  7-bucket granary family). That defect was handed to the manager in the Stage-E record and **fixed
  during this stage** in `ed29dad` (commit `fix(vs2,cognition): correct histogram W1 shape and exact
  call refs under concurrency`, 2026-10-28 02:26 +0800 = 18:26 UTC, 7 min after this run launched).
  The cells ran on the pre-fix binary (built from `d743856` at 18:19 UTC); nothing in the run, the
  world, the ledgers or any statistic used in a verdict depends on the histogram column, and
  `curve-report.txt` here is regenerated with the fixed instrument. Verified: over the 60 scalar rows
  of the five conditions the fixed report's `rmse_d`, `noise`, `snr`, `firstsep` and `run_len` are
  identical to the pre-fix report's, and its histogram W1 now equals `analyze.py`'s independent value
  exactly (e.g. `A@max` vs `U@max`: `capital_hist` 0.0226, `granary_hist` 0.1085). No code change was
  made by this stage.

## 4. KPI tables

Reference: `R@max`, cell `f10-s0`, 3 replicates (Stage E). Its band at H: population mean 15.67
(95 % CI half-width 7.99, replicate envelope 12..18, mean envelope half-width 0.503); births 20.00
(16..24); starvation 4.33 (2..7); age deaths 10.00 (10..10); max_capital 1.00 (1..1)
(`.scratch/58-w3-effort/spec.md` §4.1 — cited, not restated as new data).

### 4.1 `A@max` − `P@max`, N = 256 both sides, 3 replicates each

`first` = first separating hour (−1 none); `run` = longest separating run, `@` its start; `power` =
smallest separable constant difference; `dir` = direction of a *detected* separation (blank = none).

| KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | dir | W1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| population | 0.288 | 0.000 | n/a | 122 | 6 @134 | 0.000 | 1.000 | +1.000 | – | 0.083 |
| births_cum | 0.275 | 0.000 | n/a | 122 | 6 @134 | 0.000 | 1.000 | 0.000 | – | 0.076 |
| deaths_starve_cum | 0.083 | 0.000 | n/a | 144 | 1 @144 | 0.000 | 1.000 | −1.000 | – | 0.007 |
| deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| max_capital | 0.083 | 0.000 | n/a | 143 | 1 @143 | 0.000 | 1.000 | 0.000 | – | 0.007 |
| `capital_hist` | – | – | – | – | – | – | – | – | – | 0.012 |
| `granary_hist` | – | – | – | – | – | – | – | – | – | 0.037 |

**No KPI separates.** `A − P` differs at all at only 13 hours (h122–h144); the largest sustained
separating run is 6 hours.

### 4.2 `A@max` − `U@max`, N = 256 both sides, 3 replicates each

| KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | dir | W1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| population | 0.914 | 0.000 | n/a | 110 | 14 @131 | 0.000 | 3.000 | +3.000 | – | 0.310 |
| births_cum | 0.914 | 0.000 | n/a | 110 | 14 @131 | 0.000 | 3.000 | +3.000 | – | 0.352 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| max_capital | 0.166 | 0.000 | n/a | 130 | 2 @132 | 0.000 | 1.000 | 0.000 | – | 0.014 |
| `capital_hist` | – | – | – | – | – | – | – | – | – | 0.023 |
| `granary_hist` | – | – | – | – | – | – | – | – | – | 0.109 |

**No KPI separates.** `A − U` is the larger of the two: population and births are +3 at H with a
14-hour separating run (h131–h144, i.e. to the very end of the horizon) — the closest this stage
comes to a separation, 10 hours short of the declared gate, and with `SNR` undefined rather than
large.

### 4.3 Each arm's distance to `R@max` (§3.1, same cell and seeds; reference = Stage-E rows)

| arm | KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | W1 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `A` | population | 0.988 | 0.705 | 1.400 | 133 | 3 @133 | 1.411 | 3.667 | −1.667 | 0.457 |
| `A` | births_cum | 1.560 | 0.999 | 1.562 | 133 | 7 @138 | 1.998 | 5.000 | −5.000 | 0.729 |
| `A` | deaths_starve_cum | 0.756 | 0.556 | 1.360 | 144 | 1 @144 | 1.112 | 3.333 | −3.333 | 0.280 |
| `A` | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | 0.000 |
| `A` | max_capital | 0.157 | 0.152 | 1.033 | −1 | 0 | 0.303 | 0.667 | 0.000 | 0.000 |
| `A` | `granary_hist` | – | – | – | – | – | – | – | – | 0.170 |
| `P` | population | 1.145 | 0.705 | 1.623 | 133 | 7 @133 | 1.411 | 3.667 | −2.667 | 0.540 |
| `P` | births_cum | 1.722 | 0.999 | 1.724 | 133 | 12 @133 | 1.998 | 5.667 | −5.000 | 0.805 |
| `P` | deaths_starve_cum | 0.730 | 0.556 | 1.312 | −1 | 0 | 1.112 | 2.667 | −2.333 | 0.274 |
| `P` | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | 0.000 |
| `P` | max_capital | 0.164 | 0.152 | 1.080 | −1 | 0 | 0.303 | 0.667 | 0.000 | 0.007 |
| `P` | `granary_hist` | – | – | – | – | – | – | – | – | 0.175 |
| `U` | population | 1.537 | 0.705 | **2.178** | 131 | 14 @131 | 1.411 | 5.333 | −4.667 | 0.639 |
| `U` | births_cum | 2.229 | 0.999 | **2.232** | 131 | 14 @131 | 1.998 | 8.000 | −8.000 | 0.929 |
| `U` | deaths_starve_cum | 0.756 | 0.556 | 1.360 | 144 | 1 @144 | 1.112 | 3.333 | −3.333 | 0.280 |
| `U` | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | 0.000 |
| `U` | max_capital | 0.171 | 0.152 | 1.125 | −1 | 0 | 0.303 | 0.667 | 0.000 | 0.014 |
| `U` | `granary_hist` | – | – | – | – | – | – | – | – | 0.204 |
| `A0` | population | 1.537 | 0.705 | **2.178** | 131 | 14 @131 | 1.411 | 5.333 | −4.667 | 0.639 |
| `A0` | births_cum | 2.229 | 0.999 | **2.232** | 131 | 14 @131 | 1.998 | 8.000 | −8.000 | 0.929 |
| `A0` | deaths_starve_cum | 0.756 | 0.556 | 1.360 | 144 | 1 @144 | 1.112 | 3.333 | −3.333 | 0.280 |
| `A0` | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | 0.000 |
| `A0` | max_capital | 0.171 | 0.152 | 1.125 | −1 | 0 | 0.303 | 0.667 | 0.000 | 0.014 |
| `A0` | `granary_hist` | – | – | – | – | – | – | – | – | 0.204 |

**No arm separates from `R@max`.** `U` (and therefore its identical twin `A0`) is the only condition
whose `SNR` reaches the declared 2.0 gate (population 2.18, births 2.23) — and it still fails because
its separating run is 14 h, not 24 h. `A` is the *closest* condition to `R` by `RMSE(d)` on
population and births (0.988/1.560 vs `P` 1.145/1.722, `U`/`A0` 1.537/2.229) and the smallest
endpoint gap on population (−1.67 vs `P` −2.67, `U`/`A0` −4.67), tying `P` on births (−5.00 each);
§3.1 reads that as "closer", never as "better" and never as faithfulness. `deaths_age_cum` is 10 in all 12 cell rows of every condition (SNR `n/a`),
and `wages_cum` is 0 everywhere (reported `n/a (both deterministic)` by the instrument).
`capital_hist` W1 for the same pairs: `A` 0.034, `P` 0.034, `U` 0.037, `A0` 0.037.

### 4.4 Curve agreement (descriptive; no threshold, no verdict)

Hours at which two conditions' declared scalars differ *at all* (first replicate; the three
replicates of every arm have identical scalar series):

| pair | hours differing | first | last | | pair | hours differing | first | last |
|---|---:|---:|---:|---|---:|---:|---:|
| `A` vs `P` | 13 | 122 | 144 | | `P` vs `A0` | 33 | 110 | 144 |
| `A` vs `U` | 28 | 110 | 144 | | `P` vs `R` | 37 | 108 | 144 |
| `A` vs `A0` | 28 | 110 | 144 | | `U` vs `A0` | **0** | – | – |
| `A` vs `R` | 37 | 108 | 144 | | `U` vs `R` | 37 | 108 | 144 |
| `P` vs `U` | 33 | 110 | 144 | | `A0` vs `R` | 37 | 108 | 144 |

### 4.5 Endpoints (mean over each condition's 3 replicates)

| condition | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| `R@max` (Stage E) | 15.67 | 20.00 | 4.33 | 10.00 | 1.00 |
| `A@max` | 14.00 | 15.00 | 1.00 | 10.00 | 1.00 |
| `P@max` | 13.00 | 15.00 | 2.00 | 10.00 | 1.00 |
| `U@max` | 11.00 | 12.00 | 1.00 | 10.00 | 1.00 |
| `A0@max` | 11.00 | 12.00 | 1.00 | 10.00 | 1.00 |

## 5. Stage power and the verdict per KPI

**Distances to `R@max`** (the reference comparison carries the design's power, because the reference
is the only condition with replicate noise):

| KPI | smallest separable (power) | observed max abs d | reading |
|---|---:|---:|---|
| population | 1.41 | 3.67 (`A`,`P`) / 5.33 (`U`,`A0`) | observed differences exceed the power, yet `SNR` 1.4–2.2 and runs ≤ 14 h |
| births_cum | 2.00 | 5.0–8.0 | same: large endpoint gaps, no sustained separation |
| deaths_starve_cum | 1.11 | 2.67–3.33 | `SNR` 1.31–1.36, runs ≤ 1 h |
| deaths_age_cum | 0.00 | 0.00 | deterministic 10 in all 12 rows: no information |
| max_capital | 0.30 | 0.67 | `SNR` 1.03–1.13, no separating run |

**`A` vs `P` and `A` vs `U`** (identical-world comparisons, so the replicate noise is 0):

| pair | KPI | power | observed max abs d | sustained run | reading |
|---|---|---:|---:|---:|---|
| `A−P` | population | 0.000 (degenerate) | 1.00 | 6 h | difference present but not sustained; `SNR` undefined |
| `A−P` | births_cum | 0.000 | 1.00 | 6 h | same |
| `A−U` | population | 0.000 | 3.00 | 14 h | same, larger and longer, still < 24 h |
| `A−U` | births_cum | 0.000 | 3.00 | 14 h | same |
| both | `deaths_age_cum`, `wages_cum` | 0.000 | 0.00 | – | deterministic in all 12 rows |

The reported `power = 0.000` for `A−P`/`A−U` is a **degenerate limit**, not high sensitivity: the
replicate band is zero-width because the three replicates of each arm are the same scalar
trajectory. The honest statement of this design's power is therefore the sustained clause: a
difference that exists for fewer than 24 hours cannot be resolved however large it is, and the
largest observed one lasts 14 hours.

**Verdict, per §3.2 (applied literally):**

| baseline | A's outcome | because |
|---|---|---|
| `P` | **matches** | no KPI separates in either direction (max sustained run 6 h) |
| `U` | **matches** | no KPI separates in either direction (max sustained run 14 h) |

**Stage-T verdict: not supported.** `A` beats neither baseline under the rule; no baseline beats `A`
(`deaths_age_cum`/`wages_cum` are deterministic and the only favourable-looking gaps fall short of
the sustained gate). `unresolvable here` does **not** apply: that clause is for a detection limit
that exceeds every observed separation, and here the reported limit (0.000 on the paired arms,
1.41–2.00 against the reference) is *below* the observed separations, which fail the sustained
condition instead. "Supported" (M_end) would additionally require the pattern to reproduce at a
second cell and seed; that is Stage S and is not claimed here.

## 6. Limits

- **3 replicates that are identical on the scalars — not a stuck RNG.** The three replicates of every
  arm share the cell seed but make different draws: they select different actors (`A`, 60 of 144
  hours between r1 and r3), choose a different action at 1–6 calls each, and return different prose
  lengths. Their five scalar KPI series are nevertheless identical at every hour. The design
  therefore has *no* scalar noise floor here — a measured insensitivity of these KPIs to the
  decisions, not a determinism artefact of the harness. The §3 rule was written for a
  noise-denominated design; on this matrix its `SNR` gate is undefined (the instrument prints
  `n/a (both deterministic)`) and only the 24-hour sustained clause is live. This is a property of
  the recorded run, not a defect of the rule; it is reported, and the rule was not re-tuned.
- **The `granary_hist` family is *not* replicate-identical**: `P` and `A` differ across replicates at
  17 hours each (e.g. `P` r1 `[5,0,0,2,0,4,0]` vs r2/r3 `[4,1,0,2,0,4,0]` at h128) while their
  scalars coincide. The histogram W1 values above are therefore computed over mean bucket vectors,
  as §3 declares, and the scalar verdicts are unaffected.
- **The realized budgets are not equal**: `P` 240, `U`/`A` 248 (§2.1). The declared budget
  (`N = 256`) is equal; the realized calls differ by 8 (3.3 %) because of `P`'s 6-hour gate. No
  truncation, no halt, no shortfall.
- **Timing is confounded with call *count and placement* by design** — that is the axis. `P` looks at
  24 hours × ~10 actors, `U`/`A` at 144 hours × 1–2 actors. The record does not separate "when" from
  "how many per hour"; the pre-registration did not ask it to.
- **The model barely changes its answer**: 97.9–99.9 % of calls returned the frozen cascade's own
  action (16/744, 1/744, 14/720 deviations — §0, §2.1). The null therefore has a *mechanism*, not
  just a power story: the decision content is
  nearly independent of the observation's timing, so the world trajectory is nearly identical
  whichever hours are watched. This is model-conditional (`luna` at `max`, `natural-claim/v4`) and
  says nothing about whether a different model would behave the same — that is Stage M's question.
- **The reference is not re-drawn.** `R@max` is the Stage-E three rows (ticket-57 replicate 1 + two
  fresh), as §2 declares; the reference is a comparative proxy, never ground truth
  (`docs/VALIDATION.md`).
- **`curve/1`'s histogram column was defective at run time** (§3): the cells ran on the pre-fix
  binary; the fix (`ed29dad`) landed during the run and was used only to regenerate
  `curve-report.txt`, whose scalar rows are unchanged and whose histogram rows now agree with the
  independent computation. The `call_refs` range encoding this run's ledgers use is the one the same
  commit replaced: an auditor joining calls by `call_refs` must filter to `kind == "call"` and check
  the request hash (as `summarize.py` does); the world, replay and evaluation are unaffected.
- **`VS2-EC-1` reads FAIL for a version reason** (§1): the band compares this run's `A0` against the
  ticket-19 `natural/7` × `v6` recording; the world here is `natural/14` × `v14`. The `A0` control is
  pinned by the committed test and by its own zero-call rows.
- **One cell only.** `f10-s0`. A cell claim that is not reproduced at `f10-s1`/`f11-s2` is exactly
  that; Stage S is where reproduction is judged.
- **Synthetic, model-conditional.** Nothing here is a claim about real civilizations, and no
  condition is called better.

## 7. Artefacts

| file | what it is |
|---|---|
| `spec.md` | this record |
| `evaluation.json`, `evaluation.txt` | the run's own report: 12 cells, per-cell realized calls / under-budget / halt, bands, replay, headroom |
| `calls.tsv` | one row per recorded Role A call (2,208): cell, seq, hour, actor, action, frozen action, source, status, http, attempt, latency, tokens, request hash |
| `kpi-series.tsv` | the per-hour KPI series (five scalars + two histogram families) for all 12 cells |
| `agreement.json` | per arm: calls, `action == frozen_action` counts, source/status sets, wake hours, latency p50/p95/max |
| `analysis.md`, `analysis.json` | the §3 / §3.2 comparison, power, direction, curve-agreement, endpoints and the verdict, produced by `analyze.py` |
| `curve-report.txt` | the frozen `curve/1` report over all five conditions (`-curve-in`), all ten pairs, emitted by the instrument **after** the histogram fix `ed29dad`; used as the scalar cross-check |
| `analyze.py` | the read-only §3 analysis (pairing, SNR, separation, power, W1) |
| `summarize.py` | regenerates `calls.tsv`, `kpi-series.tsv`, `agreement.json` from a cell's ledger and evaluation |
| `accounting.py`, `wall.json`, `starts.json` | calls, tokens, derived cost, wall clock per arm; the launch epoch |
| `run.sh` | the exact invocation |
| `runs/<arm>-f10-s0-r<N>/natural-{root,branch-enabled}.bundle` | the declared genesis each cell ran from (29 KB each) |

**Not committed, deliberately:** the raw hash-chained ledgers (9 priority cells, ~9–10 MB each; the
replay lane's copies added ~150 MB under `runs/replay/`) and each cell's ~7.8 MB final checkpoint
bundle. They were removed after extraction; the world state is bound by the `tip_hash` /
`projection_hash` in `evaluation.json`, the replay verdict is recorded there, and `calls.tsv` is the
human-auditable substitute. Re-deriving the state from a ledger needs a re-run; Stage E took the same
decision for the same reason.

## 8. Reproduction

Toolchain: `cd /home/z/vibe/agentworld && . .tmp/go-env.sh` (go1.24.13), then
`go build -o .tmp/59-vs2 ./cmd/vs2`.

```
# the live matrix (spend-gated; $0.88 derived, 1 h 47 min). One invocation, 12 cells, no wave split:
sh .scratch/59-w3-timing/run.sh

# read-only analysis (no cell, no spend)
python3 .scratch/59-w3-timing/summarize.py --dir .scratch/59-w3-timing
python3 .scratch/59-w3-timing/accounting.py --dir .scratch/59-w3-timing \
  --reference .scratch/58-w3-effort/max/evaluation.json \
  --reference-extra .scratch/57-w2-concurrent-reference/evaluation.json \
  --starts .scratch/59-w3-timing/starts.json --out .scratch/59-w3-timing/wall.json
python3 .scratch/59-w3-timing/analyze.py \
  --timing-eval .scratch/59-w3-timing/evaluation.json \
  --reference-eval .scratch/58-w3-effort/max/evaluation.json \
  --reference-extra .scratch/57-w2-concurrent-reference/evaluation.json \
  --wall .scratch/59-w3-timing/wall.json \
  --out .scratch/59-w3-timing/analysis.md --json .scratch/59-w3-timing/analysis.json

# the frozen instrument over the same files (its scalar rows are the cross-check and are identical to
# the pre-fix report's; its histogram rows are the fixed `ed29dad` computation and agree with
# analyze.py's independent W1)
.tmp/59-vs2 -curve-in .scratch/59-w3-timing/evaluation.json,\
.scratch/58-w3-effort/max/evaluation.json,.scratch/57-w2-concurrent-reference/evaluation.json \
  -curve-out .scratch/59-w3-timing/curve-report.txt

# the A0 control's identity on the declared world (hermetic, no spend)
go test ./cmd/vs2 -run TestDeclaredWorldA0ReproducesTicket55 -count=1

# no code changed by this stage; the declared gate still holds
go build ./...
```
