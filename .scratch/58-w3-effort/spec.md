# Ticket 58 — W3 Stage E: thinking (`-effort`) on the W1 world, judged on the curve

- Record kind: one live stage of the W3 pre-registration (`.scratch/58-w3-preregistration/spec.md`,
  committed `8f0f47b`), its recorded cells, the declared §3 comparison against `R@max`, and the
  stage verdict. The pre-registration was amended at `36083db` (Stage M's third model only, before
  any Stage-M cell); Stage E's text and rule are unchanged by that amendment. No world, policy, metric, contract, reference-model or code change.
- Author: worker, 2026-10-28.
- Source: the pre-registration's Stage E (§4) and comparison rule (§3), declared before the first
  W3 live cell. This record applies that rule; it does not change it.
- Inputs: `W1` PASS (ticket 55), the `R@max` reference cell (ticket 57), `curve/1`
  (`cmd/vs2/curves.go`, ticket 35 — frozen, not edited).

## 0. Verdict

**No effort of `github_copilot/gpt-6-luna` produces a detectable curve difference from `R@max` on
the declared cell at this design.** For every effort in `{none, low, medium, high}` and every
declared scalar KPI, `SNR < 2` or the separating run is shorter than 24 hours; no KPI separates.
This is **"no detectable effect at this design"**, never equivalence: the smallest separable
difference per KPI is reported below and several observed differences sit below it.

The declared `curve/1` instrument over all six evaluation files returns the same outcome for all ten
pairs: `NOT distinguishable at this noise floor` (`curve-report.txt`, §7).

There is one suggestive, sub-threshold pattern: `R@none` is lower than `R@max` on `population`
(−2.0 at H) and `births_cum` (−3.0 at H), separating from hour 124 for 9 and 11 hours
respectively — short of the declared 24-hour run, with SNR 1.26 and 1.25. It is recorded as a
non-separation.

## 1. What ran

One cell (`f10-s0`, 10 founders, seed 0), arm `R` (full wake, no priority), model
`github_copilot/gpt-6-luna`, wire `responses`, claim contract `natural-claim/v4`, output bound
32768, response bound 1 MiB, `H = 144`, `-call-concurrency 10`, live gateway, spend authorised for
this stage only. Each effort ran its declared replicates in its own root and out directory:

| effort | reps run live here | evaluation |
|---|---:|---|
| `none` | 3 | `none/evaluation.json` |
| `low` | 3 | `low/evaluation.json` |
| `medium` | 3 | `medium/evaluation.json` |
| `high` | 3 | `high/evaluation.json` |
| `max` | 2 | `max/evaluation.json` |

The third `R@max` replicate is the ticket-57 reference cell
(`.scratch/57-w2-concurrent-reference/evaluation.json`, replicate 1), carried into the analysis as
the pre-registration §2 declares ("Replicate 1 of `f10-s0` is the ticket-57 cell; two more
replicates are run in W3"). Mixing is defensible because the three rows are the same declared world
and the same condition: the fresh max cells' `root_digest` (`13e8b484…`) and `branch_digest`
(`ad0360fe…`) are byte-identical to the ticket-57 cell's, and model, wire, effort, contract,
output/response bounds, `H` and call concurrency are identical. The replicate index labels are
arbitrary; the paired rule pairs hours, not replicate ids. The three `R@max` rows are therefore
`t57max.r1`, `w3max.r1`, `w3max.r2`.

Command (per effort; `-reps 3`, and `-reps 2` for `max`):

```
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND LITELLM_MODEL=github_copilot/gpt-6-luna \
.tmp/58-vs2 -arms R -cells f10-s0 -reps <3|2> -hours 144 -effort <E> \
  -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 3 -budget-min 600 \
  -root .scratch/58-w3-effort/<E>/runs \
  -out .scratch/58-w3-effort/<E>/evaluation.json \
  -out-txt .scratch/58-w3-effort/<E>/evaluation.txt
```

The exact invocation is kept as `<E>/run.sh` and `starts.json` records each wave's start epoch.
All five evaluations report `status: live/real-model`, `live: true`, `call_concurrency: 10`,
`claim_contract: natural-claim/v4`, `model_id: github_copilot/gpt-6-luna`, and a completed 145-hour
(h0..h144) series for every cell. Each effort's in-process replay lane re-ran its cells from the
ledger through a dead-network transport: every cell reports `identical` and the evaluation's
`replay_mismatches` is empty (`evaluation.txt`, `REPLAY SUBSET`).

### 1.1 The first attempt was aborted for upstream rate limiting (not truncated by budget)

The first launch ran all five efforts at once (15 cells × call concurrency 10 = 150 in-flight
requests). The upstream GitHub Copilot endpoint returned `429 too many requests`
(`response_body`: `litellm.RateLimitError: Github_copilotException - too many requests`) on ~15 %
of physical attempts; throughput saturated at ~600 successful calls/min regardless of demand. That is
a data-quality hazard: three exhausted attempts halt a cell fail-loud, and one `medium` cell halted at
hour 24 after exactly such an exhaustion. The attempt was stopped at 12 minutes after 7,466 physical
attempts (1,133 rejected, 1 exhausted; `aborted-rate-limit.json`), its run directories were discarded
before any analysis, and no cell from it appears in this record.

The stage was re-run from zero at a demand inside the upstream limit: two efforts at a time
(6 cells × 10 = 60 in-flight). The re-run saw 5 × 429 on 20,942 attempts and **no exhausted call, no
halt**, and every cell completed to `H = 144`. The aborted attempt cost ~$1.08 derived (below); it is
reported, not hidden. This is the only deviation from the declared run schedule, and it changed no
rule, cell, replicate or world.

## 2. Accounting

Recorded charge is **$0.00**: the gateway exposes no price entry for `github_copilot/gpt-6-luna`
(`price_version = unpriced`, `cost_micros = 0`). The derived price below is arithmetic at the
ticket-35 published rates ($0.10/M input, $0.50/M output). Wall clock is launch to the effort's
`evaluation.json`, i.e. including the 3 concurrent cells and the in-process replay.

| effort | decisions | physical attempts | prompt tokens | completion tokens | derived $ | wall | decision errors | applied model / fallback |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `none` | 4,365 | 4,365 | 2,744,739 | 224,197 | 0.3866 | 17.5 min | 0 | 4,365 / 0 |
| `low` | 4,533 | 4,538 | 2,849,212 | 890,702 | 0.7303 | 32.8 min | 6 | 4,527 / 6 |
| `medium` | 4,556 | 4,557 | 2,864,510 | 1,152,512 | 0.8627 | 25.0 min | 3 | 4,553 / 3 |
| `high` | 4,486 | 4,487 | 2,820,898 | 1,360,539 | 0.9624 | 25.5 min | 2 | 4,484 / 2 |
| `max` (2 fresh reps) | 2,995 | 2,995 | 1,883,387 | 2,123,822 | 1.2502 | 69.2 min | 0 | 2,995 / 0 |
| `max` ticket-57 carry-in | 1,579 | 1,579 | 993,037 | 1,061,251 | 0.6299 | (67.6 min) | 0 | 1,579 / 0 |
| **fresh total** | **20,935** | **20,942** | **13,162,746** | **5,751,772** | **4.1922** | | **11** | |
| aborted first attempt | 7,466 attempts | | 4,016,922 | 1,353,430 | 1.0780 | 11.5 min | 1 exhaust | |
| **stage-derived total** | | | | | **5.90** | **2 h 21 min** | | |

- Every cell's reported `realized_calls` is the physical-attempt count; a retried 429 adds an
  attempt without adding a decision. `decisions = escalated_decisions`.
- The 11 decision errors are the declared level-1 fallbacks (the frozen action is kept):
  `low` 6, `medium` 3, `high` 2, `none`/`max` 0. Eight are model answers that failed the contract;
  three are upstream `ContentPolicyViolationError` calls (HTTP 400, non-retryable), which the seam
  records as `fallback:malformed` too. Two further calls had their first attempt time out and
  succeeded on retry (not errors). The per-cell counts are in `evaluation.json`; they are part of
  the condition's recorded behaviour, not silently repaired.
- **Derived cost overrun:** the stage was estimated at ~$3–4 derived; it closed at **$5.90 derived**
  ($4.82 for the recorded matrix including the ticket-57 carry-in, plus $1.08 for the aborted first
  attempt). The gateway charged $0. Counting W2's $0.63, ~$7.5 of the operator's ~$14 overall
  derived budget remains; the manager should decide before Stage T.

## 3. The rule applied (declared in §3; no new rule)

`analyze.py` re-implements the pre-registered `curve/1` statistics literally over the recorded
evaluation JSONs (read-only, no cell, no model call):

- per hour, `d(t) = mean_A(t) − mean_B(t)` over the 3 replicates;
- `RMSE(d)` over hours; `noise` = RMSE over hours of the pooled within-condition SD at each hour;
  `SNR = RMSE(d)/noise` (undefined when both are deterministic, reported `n/a`, never ∞);
- a **separation** at hour `t` iff `|d(t)|` exceeds both conditions' replicate envelope half-width;
  a KPI **separates** iff `SNR ≥ 2` **and** some separating run is ≥ 24 consecutive hours. The
  verdict is per scalar KPI, `{population, births_cum, deaths_starve_cum, deaths_age_cum,
  max_capital}`; `wages_cum` is carried by the instrument but is not a §3 scalar and never enters a
  verdict; the histogram families are carried by Wasserstein-1, not by the scalar rule.
- **Power** (smallest separable difference): a constant offset δ has `RMSE(d) = δ`, so the SNR gate
  needs `δ ≥ 2·noise`; the 24-hour gate needs δ to exceed the band at every hour of some 24-hour
  window, i.e. `δ > min over windows of the window's max band`. The reported power is the larger of
  the two floors. The band floor is 0.000 for every effort because the founding hours are identical
  across replicates (population 10, no deaths, no births); the binding floor is `2·noise`.
- **Direction** is description only: a separation's sign on the mean difference, with the declared
  reading (population/births higher is better for viability; starvation/age deaths lower). Nothing
  is summed or ranked.
- Histogram distance: each condition's hourly bucket vector is the mean count over its 3 replicates
  normalised to a distribution over bucket indices; W1 is computed in bucket units each hour and
  averaged over hours (ticket-35 §3 definition).
- Cross-check: `analyze.py`'s scalar rows reproduce the Go `-curve-in` report's `rmse_d`, `noise`,
  `snr`, `firstsep` and `run_len` exactly (e.g. `R@none` vs `R@max` population: 1.0446 / 0.8318 /
  1.2558 / 124 / 9). The Go report is kept as `curve-report.txt`. It was regenerated with the
  instrument at `8e4f23b` (the model-axis commit that landed while the cells ran); the only
  difference from the state that ran them (`8f0f47b`) is the added `models:` header line — every
  statistical row is byte-identical (verified by diff).

## 4. KPI tables

### 4.1 The reference band (`R@max`, 3 replicates, cell `f10-s0`)

| KPI | mean@H | 95 % CI half@H | envelope@H | mean envelope half-width | max envelope half-width |
|---|---:|---:|---|---:|---:|
| population | 15.667 | 7.986 | 12..18 | 0.503 | 3.667 |
| births_cum | 20.000 | 9.937 | 16..24 | 0.738 | 4.000 |
| deaths_starve_cum | 4.333 | 6.252 | 2..7 | 0.329 | 3.333 |
| deaths_age_cum | 10.000 | 0.000 | 10..10 | 0.000 | 0.000 |
| max_capital | 1.000 | 0.000 | 1..1 | 0.092 | 0.667 |

The reference is stochastic: at H the population envelope is 12..18 and births 16..24 over three
replicates. That envelope, not the standard error, is what the declared separation rule uses.

### 4.2 Paired comparison, every effort vs `R@max` (effort − max)

`first` = first separating hour (−1 none); `run` = longest separating run, `@` its start; `power` =
smallest separable constant difference at this design; `dir` = direction of a *detected* separation
(blank = none detected).

| effort | KPI | RMSE(d) | noise | SNR | first | run | power | dir | W1 |
|---|---|---:|---:|---:|---:|---:|---:|---|---:|
| none | population | 1.045 | 0.832 | 1.256 | 124 | 9 @124 | 1.664 | – | 0.464 |
| none | births_cum | 1.417 | 1.131 | 1.253 | 124 | 11 @124 | 2.261 | – | 0.687 |
| none | deaths_starve_cum | 0.602 | 0.578 | 1.041 | −1 | 0 | 1.157 | – | 0.232 |
| none | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | – | 0.000 |
| none | max_capital | 0.152 | 0.189 | 0.803 | −1 | 0 | 0.378 | – | 0.005 |
| low | population | 0.429 | 1.256 | 0.341 | −1 | 0 | 2.512 | – | 0.115 |
| low | births_cum | 0.983 | 1.465 | 0.671 | −1 | 0 | 2.930 | – | 0.347 |
| low | deaths_starve_cum | 0.667 | 0.570 | 1.170 | −1 | 0 | 1.141 | – | 0.251 |
| low | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | – | 0.000 |
| low | max_capital | 0.144 | 0.203 | 0.707 | −1 | 0 | 0.407 | – | 0.025 |
| medium | population | 0.349 | 1.009 | 0.346 | −1 | 0 | 2.018 | – | 0.094 |
| medium | births_cum | 0.832 | 1.348 | 0.617 | −1 | 0 | 2.696 | – | 0.299 |
| medium | deaths_starve_cum | 0.637 | 0.694 | 0.917 | −1 | 0 | 1.388 | – | 0.255 |
| medium | deaths_age_cum | 0.055 | 0.068 | 0.816 | −1 | 0 | 0.136 | – | 0.009 |
| medium | max_capital | 0.161 | 0.198 | 0.816 | −1 | 0 | 0.395 | – | 0.014 |
| high | population | 0.455 | 1.095 | 0.415 | −1 | 0 | 2.190 | – | 0.221 |
| high | births_cum | 0.790 | 1.380 | 0.572 | −1 | 0 | 2.761 | – | 0.349 |
| high | deaths_starve_cum | 0.441 | 0.621 | 0.710 | −1 | 0 | 1.243 | – | 0.170 |
| high | deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | – | 0.000 |
| high | max_capital | 0.136 | 0.209 | 0.649 | −1 | 0 | 0.418 | – | 0.005 |

`wages_cum` is 0 for every replicate of every condition (no wage payment occurs on this world), so
its comparison is `n/a (both deterministic)` with `RMSE(d) = 0`.

### 4.3 Histogram families (Wasserstein-1 over buckets, mean over hours)

| effort | `capital_hist` | `granary_hist` |
|---|---:|---:|
| none | 0.032 | 0.238 |
| low | 0.019 | 0.071 |
| medium | 0.020 | 0.066 |
| high | 0.016 | 0.085 |

These are descriptive distances only; §3 gives the histogram families no separation test. `none`
is the furthest on both families, in the same direction as its sub-threshold population/births
difference.

### 4.4 Curve endpoints (mean over the condition's 3 replicates)

| effort | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| max (reference) | 15.67 | 20.00 | 4.33 | 10.00 | 1.00 |
| none | 13.67 | 17.00 | 3.00 | 10.00 | 1.33 |
| low | 14.00 | 16.33 | 2.33 | 10.00 | 1.33 |
| medium | 15.33 | 17.00 | 2.00 | 9.67 | 1.00 |
| high | 14.33 | 17.00 | 2.67 | 10.00 | 1.00 |

## 5. Stage power and the verdict per KPI

The smallest separable difference at 3 replicates and the observed bands (the `power` column above):

| KPI | separable at this design, per effort | reading |
|---|---|---|
| population | 1.66 (none), 2.51 (low), 2.02 (medium), 2.19 (high) | observed max \|d\| 1.33–3.67; none reached a 24 h run |
| births_cum | 2.26, 2.93, 2.70, 2.76 | observed max \|d\| 3.0–4.0; none reached a 24 h run |
| deaths_starve_cum | 1.16, 1.14, 1.39, 1.24 | observed max \|d\| 1.67–2.33; SNR 0.71–1.17 |
| deaths_age_cum | 0 (deterministic 10 in 14 of 15 cell rows) | no difference observed (medium r2 has 9) |
| max_capital | 0.38–0.42 | observed max \|d\| 0.67; SNR 0.65–0.82 |

**Verdict, per effort:** `R@none`, `R@low`, `R@medium`, `R@high` each **match** `R@max` under the
declared rule — i.e. **no detectable effect at this design**. No KPI separates in any effort
(SNR ≥ 2 and a ≥ 24-hour run in every cell; there is one declared cell). This is a detection limit,
not evidence of equality: the endpoint gaps (e.g. `none` −2.0 alive / −3.0 births, `low` −3.7
births at H) are at or near the smallest separable difference, but the paired rule does not resolve
them at 3 replicates.

**Stage power:** with 3 replicates and the observed replicate noise, the design resolves a sustained
per-hour difference of about **1.7–2.9 population/births**, **1.1–1.4 starvation deaths** and
**0.4 max-capital units** per hour. Anything smaller is not evidence at this design, however large
its endpoint difference looks.

## 6. Limits

- **3 replicates, not 30.** The band is estimated from 3, and the `R@max` envelope is wide
  (population 12..18 at H). The power column is the honest limit; a small true effect is
  indistinguishable from replicate noise here.
- **The carried-in reference replicate is not a fresh draw.** `R@max` mixes the ticket-57 cell with
  two fresh cells (same world digests, same condition, §1). It is a declared replicate by §2, but a
  reader should know its provenance.
- **Decision errors are not zero for every effort.** 11 declared level-1 fallbacks (8
  contract-failing model answers, 3 upstream content-policy 400s) occur across
  `low`/`medium`/`high`; `none` and `max` have none. The KPI curves are the effective decisions
  including those fallbacks; the counts are in §2 and per cell in each `evaluation.json`.
- **Two deterministic KPIs carry no information here.** `deaths_age_cum` is 10 in 14 of the 15 cell
  rows (medium r2 has 9) and `wages_cum` is 0 everywhere; their SNR is `n/a (both
  deterministic)`, not ∞.
- **The 24-hour band floor is 0.000** because the founding 100 hours are identical across
  replicates. The binding power floor is `2·noise`, not the band.
- **Ledger `call_refs` under concurrency + retry.** The recorder's attempt-range expansion
  (`callRefRange`) assumes a call's physical attempts are contiguous ledger records. Under call
  concurrency another record can interleave between a retry's attempts, so a `role-a` record's
  `call_refs` can include a foreign record (observed: `low/runs/R-f10-s0-r3`, intervention 977
  references 975/976, where 975 is another actor's `role-a`). The world, replay and evaluation are
  unaffected (replay resolves by request hash), but an auditor joining calls by `call_refs` must
  filter to `kind == "call"` and check the request hash. `summarize.py` does this; the defect is
  reported to the manager as a follow-up, not fixed here (no code change in this stage).
- **`curve/1`'s histogram columns are not usable as a bucket distance.** `curves.go` computes
  `wassersteinHistograms(histRows(...))` with replicate-major series, so the "bucket index" it
  integrates over is the hour index; its printed values exceed the bucket range (e.g. 22.07 for a
  7-bucket granary family). The scalar columns are correct and are cross-checked above. The §3
  histogram distance used here is computed by `analyze.py` from the per-hour bucket vectors. The
  frozen instrument was not edited; this is a defect to hand to the manager.
- **The upstream endpoint rate-limits at ~600 successful calls/min**; the first all-at-once attempt
  hit `429 too many requests` on ~15 % of attempts and was aborted at 12 minutes. The recorded
  matrix is the re-run at 60 in-flight, which saw 5 × 429 and no exhausted call.
- **The gateway is unpriced.** The recorded charge is $0; all dollar figures are derived at the
  ticket-35 published rates. Stage-derived total $5.90 (including the aborted attempt's $1.08)
  exceeds the ~$3–4 pre-registration estimate; the next stage's budget is the manager's call.
- **`xhigh` was not run**, as §4 declares it only if the E curve separates and the budget allows;
  it does not separate.
- The model/timing comparisons are model-conditional and synthetic; nothing here is a claim about
  real civilizations, and no condition is called better.

## 7. Artefacts

| file | what it is |
|---|---|
| `spec.md` | this record |
| `none/`, `low/`, `medium/`, `high/`, `max/` | per effort: `evaluation.json`, `evaluation.txt`, `run.sh`, `calls.tsv` (one row per decision: cell, seq, hour, actor, action, frozen action, source, status, http, attempt, latency, tokens, request hash), `kpi-series.tsv` (per hour and replicate) |
| `analysis.md`, `analysis.json` | the §3 per-KPI comparison, power, direction and verdict tables produced by `analyze.py` |
| `curve-report.txt` | the frozen `curve/1` report over all six evaluations (`-curve-in`), all ten pairs |
| `analyze.py` | the read-only §3 analysis (pairing, SNR, separation, power, W1) |
| `summarize.py` | regenerates `calls.tsv` / `kpi-series.tsv` from a cell's ledger and evaluation |
| `accounting.py`, `wall.json`, `starts.json` | calls, tokens, derived cost, wall clock per effort; the launch epochs |
| `aborted-rate-limit.json` | per-cell attempt/429/exhaustion counts of the discarded first attempt |
| `*/runs/<cell>/natural-{root,branch-enabled}.bundle` | the declared genesis each cell ran from (32 KB each) |

**Not committed, deliberately:** the raw hash-chained ledgers (14 files, ~1.1 GB uncompressed under
`*/runs/`) and the 7.5 MB final checkpoint bundle of each cell (reproducible by replaying the
ledger, and its state is bound by the tip/projection hashes in `evaluation.json`); both were removed
after extraction. Ticket 57 committed one 38 MB gzipped ledger for one cell; 14 such files would be
hundreds of MB in the repo for no analytical value. This record commits the per-call `calls.tsv`
(the human-auditable substitute the ticket permits) and the evaluations, and the raw ledgers are
removed after extraction. The world state is bound by the tip/projection hashes in each
`evaluation.json`, and the replay verdict is recorded there; re-deriving it needs a re-run.

## 8. Reproduction

Toolchain: `cd /home/z/vibe/agentworld && . .tmp/go-env.sh` (go1.24.13), then
`go build -o .tmp/58-vs2 ./cmd/vs2`.

```
# the live matrix (spend-gated; ~$4.19 derived for the 14 fresh cell rows)
sh .scratch/58-w3-effort/none/run.sh      # and low, medium, high, max — wave the first two
                                          # (60 in-flight is inside the upstream limit), then next two,
                                          # then max

# read-only analysis (no cell, no spend)
python3 .scratch/58-w3-effort/analyze.py \
  --max-eval .scratch/58-w3-effort/max/evaluation.json \
  --max-extra .scratch/57-w2-concurrent-reference/evaluation.json \
  --effort none=.scratch/58-w3-effort/none/evaluation.json \
  --effort low=.scratch/58-w3-effort/low/evaluation.json \
  --effort medium=.scratch/58-w3-effort/medium/evaluation.json \
  --effort high=.scratch/58-w3-effort/high/evaluation.json \
  --wall .scratch/58-w3-effort/wall.json \
  --out .scratch/58-w3-effort/analysis.md --json .scratch/58-w3-effort/analysis.json

# the frozen instrument over the same files
.tmp/58-vs2 -curve-in .scratch/58-w3-effort/none/evaluation.json,\
.scratch/58-w3-effort/low/evaluation.json,.scratch/58-w3-effort/medium/evaluation.json,\
.scratch/58-w3-effort/high/evaluation.json,.scratch/58-w3-effort/max/evaluation.json,\
.scratch/57-w2-concurrent-reference/evaluation.json \
  -curve-out .scratch/58-w3-effort/curve-report.txt

# human tables from a recorded effort
python3 .scratch/58-w3-effort/summarize.py --dir .scratch/58-w3-effort/<E>
python3 .scratch/58-w3-effort/accounting.py --root .scratch/58-w3-effort \
  --starts .scratch/58-w3-effort/starts.json --out .scratch/58-w3-effort/wall.json

# no code changed by this stage; the declared gate still holds
go build ./...
go test ./cmd/vs2 -count=1
```
