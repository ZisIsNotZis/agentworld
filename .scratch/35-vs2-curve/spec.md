# Ticket 35 — VS2 curve study (noise floor, effort ladder, depth/redundancy)

Status: dispatched 2026-10-17, actor: supervisor, worker: sole writer on branch
`feature/35-vs2-curve` (branched from `master` at `3faa480`). Live spend is authorised in-session by
the supervisor within a hard cap of **$25** for this lane.

This document is the study contract. The **acceptance criterion below is pre-declared and committed
before any Stage-1 call**; it is not tuned after seeing results.

## Why this study exists (measured facts, not re-derived)

The ticket-34 live matrix (`.scratch/34-vs2-live-matrix/spec.md`) was run at the model's default
reasoning effort because the codec never sent `reasoning.effort`, and with a 512-token output bound.
Measured facts carried forward (verified by the supervisor, 2026-10-16/17):

1. The whole ticket-34 matrix cost **$4.54 for 26 337 calls** ($0.17 per 1000 calls) at gpt-6-luna
   prices ($0.10/M input, $0.50/M output); ledgers `.tmp/34-vs2-runs/*/calls.jsonl`.
2. **100% of ticket-34 model failures were reasoning truncation** (`status: incomplete`); zero schema
   errors. Rates: A 11.4%, P 7.5%, R 9.2%, U 10.9%.
3. Truncation was **not** the cause of the EC-3 falsifier: at f10-s0 both A and P had exactly 20
   truncated calls out of ~250, yet divergence differed 0.81 vs 0.07.
4. Actual cause: the adaptive arm's **coverage collapsed**. At f10-s0, A spent 43–49 calls on each of
   ~5 actors and 1–3 calls on three others, while P covered the entire roster (10–12 actors) at every
   gated hour. `vs2-value/1` has no marginal/staleness term and no coverage floor, so a static
   priority ranking produces a fixed winner set.
5. `reasoning.effort` **is** honoured by the gateway (contrary to an earlier ticket-34 note).
   Supported: `none`, `low`, `medium`, `high`, `xhigh`, `max`; `minimal` and `auto` are rejected.
6. Measured output cost/latency per effort on 40 real requests (cap 4096): `none` median 18 tok /
   $0.05 per 1k; `low` 215 / $0.17; `medium` 300 / $0.23; `high` 510 / $0.37; `max` 1838 / $1.13.
   At `max` with cap 16384 on 50 real requests: 50/50 completed, median 1907, p90 5709, p99 13826,
   max 14001 — so a 4096 cap truncates ~22% of max-effort calls. The gateway accepts
   `max_output_tokens` up to at least 200000.
7. Decision stochasticity (3 repeats per request): self-consistency 48–64% for
   none/low/medium/high and 84% for max; a single sample equals its own level's majority 81–85% of
   the time; 3-sample majority vs the max-effort majority: none 17%, low 72%, medium 72%, high 83%.
   Probe artefacts: `.tmp/35-curve-results.json`, `.tmp/35-maxtok-results.json`,
   `.tmp/35-probe.py`, `.tmp/35-curve.py`, `.tmp/35-maxtok.py`, `.tmp/35-efforts.py`.

## Supervisor decisions (confirmed, binding for this ticket)

1. **Output cap for all live runs: 32768.** A cap is free headroom; truncation is the expensive
   failure. A truncation at 32768 is recorded as a **decision failure**, not a reason to raise the cap
   again.
2. **Effort ladder: `none/low/medium/high/max`.** The declared live default is **`low`**.
3. **Horizon 96 h**, cells **f10-s0 and f12-s0**, **3 replicates per condition**.
4. **Budget axis stays call count** (the existing EC-2 machinery is untouched); every cell must ALSO
   report realized input/output token totals so an equal-token re-analysis is possible later.
   `-votes k` (default 1) is implemented and recorded but stays **unused** in this lane.
5. **Stages and spend** (hard cap $25 total live spend; stop and report before exceeding):
   - **Stage 1 (gate):** full-wake arm at `max`, 2 cells × 3 reps = 6 runs (~$7.7).
   - **Stage 2:** full-wake arm at `none`, `low`, `medium`, `high`, 2 cells × 3 reps = 24 runs (~$4.2).
   - **Stage 3:** arms A0/P/U/A at `low`, 2 cells × 3 reps = 24 runs (~$0.6).
6. **The criterion is pre-declared below and committed before any Stage-1 call.**
7. **Integrity requirement (verbatim).** The ticket-34 verdict (central claim NOT accepted under EC-3
   as written) STANDS and is not rejudged; the curve methodology is a NEW criterion pre-registered
   before data; it must not be tuned after seeing results; its first duty is to measure the noise
   floor, which may show the earlier A-vs-P difference was within replicate noise.
8. **Docs to update (and nothing else):** this file, `CHANGELOG.md`, `docs/VERTICAL-SLICES.md` (M2 row
   and the VS2 section), and `.scratch/24-vs2-attention/spec.md` only if a status line needs to point
   at this study. `docs/ROADMAP.md` is historical and is not rewritten.

## Operational rules (declared before running)

- Pre-declare before spending; commit in chunks (the harness kills a child at 30 minutes and the
  session can drop); detached background runs with pid files and bounded poll loops, as in ticket 34.
- Cells run with `-parallel 4`.
- Stop and report on: gateway throttling persisting after bounded retries; a preflight failure; spend
  approaching $25; two consecutive cells failing with the same root error.
- **Single-call latency for `max` effort is measured before Stage 2 is sized.** If projected Stage-1
  wall time exceeds 6 hours, the horizon drops to **72 h for ALL stages**, declared before running, so
  every condition stays identical.
- Never log the API key or the spend-token value. `VS2_SPEND_AUTHORIZATION` is set for a run only.

## The pre-declared acceptance criterion (NEW; frozen at commit of this file)

### Unit of analysis

The unit is the **per-hour KPI curve of a condition, replicated 3 times**. A *condition* is an
(arm, applied effort) pair. A *cell* is (founders, seed). Replicates of one cell differ only in model
sampling: the world seed is fixed, `temperature` is rejected by the model and `seed` is ignored.

`x_k(condition, cell, r, h)` is the value of KPI `k` at hour `h` of replicate `r ∈ {1,2,3}`.

### KPI set (all already exported by `internal/experiment.Projections`)

Population (alive by hour), births cumulative, starvation deaths cumulative, age deaths cumulative,
capital histogram (buckets `0..8`), granary histogram (buckets `0..5`), wages/exchange cumulative, max
capital, and the decision-error rate already in the report. The histograms enter the curve analysis as
one KPI per bucket (`capital_hist[b]`, `granary_hist[b]`). The decision-error rate is a per-cell scalar
(no hourly series) and is reported with its own band and scalar SNR.

### Band and stability (per condition, cell, KPI)

- **Band** at hour `h`: mean over replicates `mu(h)`, the 95% CI `mu(h) ± t·s(h)/√n` with
  `t = t_{0.975,2} = 4.303` for `n = 3` (Student t, `n-1` degrees of freedom), and the full
  `[min, max]` envelope over replicates.
- **Stability**: across-replicate coefficient of variation `CV(h) = s(h)/|mu(h)|` for `mu(h) ≠ 0`
  (hours with `mu(h) = 0` are excluded and counted). Report the mean CV over hours and the worst
  (largest) hour.
- `n = 1` is reported as a degenerate band (width 0) and flagged; it is never called a stable result.

### Between two conditions (paired by cell and seed)

For a shared (cell, KPI) and every hour with all replicates present:

- `d(h) = mu_A(h) - mu_B(h)`.
- Replicate noise `sigma(h)` = pooled within-condition SD at that hour,
  `sqrt((SS_A(h) + SS_B(h)) / ((n_A-1) + (n_B-1)))`, with `SS(h) = Σ_r (x_r(h) - mu(h))²`.
- **SNR_k = RMSE_h(d) / RMSE_h(sigma)** — the root-mean-square over hours of the paired difference,
  divided by the root-mean-square over hours of the replicate noise. Reported per cell; cells are
  never pooled, because different cells are different worlds.
- **Replicate band of the difference** at hour `h`:
  `B(h) = max(envHi_A(h) - mu_A(h), mu_A(h) - envLo_A(h), envHi_B(h) - mu_B(h), mu_B(h) - envLo_B(h))`
  (the wider single-condition envelope half-width).
- **First-separation hour** = the smallest `h` with `|d(h)| > B(h)`; `-1` when no hour separates.
  Also reported: the start hour and length of the longest separating run.
- **Wasserstein-1** between the per-hour distributions: for a scalar KPI, pool all `(h, r)` values of
  each condition into one empirical 1-D sample and report `W1` (integral of |F_A - F_B|). For a
  histogram bucket set, normalise each hour's bucket vector to a probability distribution over bucket
  indices, compute the 1-D `W1` in bucket units, and average over hours.
- **Degenerate denominator**: when one condition is deterministic (replicate SD 0 at every hour — the
  A0 control is deterministic by construction, having no cognition), the denominator uses the
  non-degenerate condition's variance and the SNR is flagged `one-sided noise floor`. When both
  conditions are deterministic, SNR is reported `n/a (both deterministic)` with the raw RMSE of the
  difference.

### The decision rule

> A condition is **distinguishable** from another iff some KPI has `SNR >= 2` **and** that KPI's paired
> difference exceeds the replicate band for **at least 24 consecutive hours**, in **both** cells.
> Otherwise the pair is **not distinguishable at this noise floor**.

**No condition may be called better on a scalar.** The report states numbers, the noise floor, and what
the data does and does not establish. Direction/dominance language is reserved to the frozen EC-3
instrument and to the milestone review, and the ticket-34 verdict is not rejudged here.

## Declared before the first live call: horizon, throughput, wall-time

Measured single-call latency at `max` effort with the 32768 bound (probe calls spent outside the
harness, recorded here and nowhere else; `.tmp/35-latency.py`, `.tmp/35-latency2.py`,
`.tmp/35-latency8.py` with their `.out` files):

| probe | n | mean latency | median | p90 | max | throughput |
|---|---|---|---|---|---|---|
| max, cap 32768, parallel 4, 12 mixed requests | 12 | 37.7 s | 22.1 s | — | 94.9 s | 6.4 calls/min |
| max, cap 32768, parallel 4, 24 random ticket-34 requests | 24 | 28.0 s | 20.6 s | 36.6 s | 154.6 s | 6.98 calls/min |
| max, cap 32768, parallel 8, 32 random ticket-34 requests | 32 | 17.7 s | 13.7 s | 29.6 s | 62.5 s | 22.62 calls/min |

**Decisions (supervisor, before any Stage-1 cell ran):**

1. **Horizon 72 h for ALL stages.** At 96 h the full-wake arm is ~864 calls/run and Stage 1 projects to
   >6 h wall at the measured p4 rate; the pre-declared rule above fires, so every stage runs at 72 h
   (≈648 calls per full-wake run, 3888 calls for Stage 1). The criterion is unchanged: changing a
   declared design parameter is not a criterion change.
2. **`-parallel 8` for all live stages.** Throughput scales 3.24× from p4 to p8 (6.98 → 22.62
   calls/min) with zero 429s and zero errors measured, so the declared concurrency is 8. Stage 1
   projects to ~2.9 h wall (3888 calls at 22.6 calls/min).
3. Probe cost: 68 real calls at `max` (~$0.9 at the measured $1.13 per 1000), spent outside the
   harness and recorded as probe spend, never as study data.

## Design and cost

| stage | condition | cells | reps | runs | ~calls | est. spend |
|---|---|---|---|---|---|---|
| 1 (gate) | R @ max | f10-s0, f12-s0 | 3 | 6 | ~5 400 | ~$7.7 |
| 2 | R @ none, low, medium, high | f10-s0, f12-s0 | 3 | 24 | ~21 600 | ~$4.2 |
| 3 | A0, P, U, A @ low | f10-s0, f12-s0 | 3 | 24 | ~4 600 | ~$0.6 |

Live spend cap **$25**; the table totals ~$12.5. Per-cell run directories are
`<arm>-f<F>-s<S>-r<N>` under one root per stage, each with its own ledger and checkpoint bundles.

## Environment (verified)

`VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND` (value set for a run only),
`LITELLM_BASE_URL=http://127.0.0.1:4000`, `LITELLM_MODEL=github_copilot/gpt-6-luna`, wire
`responses`. Go: `. .tmp/go-env.sh` (GOROOT under `.tmp/go-toolchain`, `GOTOOLCHAIN=local`). The
gateway needs no credential today.

## Live smoke before Stage 1 (2026-10-17): applied provenance works, response bound bites

Command: `.tmp/35-run-smoke.sh` → `-arms P -cells f10-s0 -hours 4 -effort max -max-output 32768 -live
-parallel 2 -attempt-timeout 300 -call-deadline 900 -budget-min 10 -replay=false`, binary
`.tmp/35-vs2-bin` sha256 `e5ed84025c1be43c4eacc9fc0b32b1bf7ec81c4cf53e4dd37ce0e90e30f7bf9f`.

Results, verbatim from `.tmp/35-smoke-live-runs/P-f10-s0-r1/calls.jsonl`: 10 call records, 7 `ok`,
3 `oversized`. Every record carries the applied wire settings
(`applied_reasoning_effort: "max"`, `applied_max_output_tokens: 32768`) beside the unchanged declared
`sampling` (`max_tokens: 128`) — the ticket-34 provenance lie is fixed and visible per call.

Two harness bounds had to be re-declared for live max effort, and one is a blocker:

1. **Attempt timeout (repaired, declared flags `-attempt-timeout`/`-call-deadline`).** The frozen 30 s
   per-attempt bound (hermetic-era) cuts measured max-effort calls (mean 28 s, p90 36.6 s, max 154.6 s);
   the first smoke attempt recorded 2 `timeout` statuses and retried. Live runs now declare 300 s per
   attempt and a 900 s call deadline, recorded in the revision string.
2. **Response-body bound (BLOCKER, escalated).** `MaxResponseBytes` is the spec's 64 KiB recorded-body
   bound. A max-effort `/v1/responses` body carries ~7 `reasoning` items of ~13 KB each
   (`encrypted_content`, unused by the world) plus the 659-byte `message` item, i.e. ~90 KB, so ~30 % of
   max-effort calls are recorded `oversized` and their decision falls back to the frozen cascade. Probe
   of one raw body (`35-bodysize.py`) records the shape; the world reads only `output_text`.
