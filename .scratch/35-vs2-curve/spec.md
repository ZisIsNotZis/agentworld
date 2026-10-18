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

**Repair (supervisor option 1, authorised before any Stage-1 cell).** The recorded-response bound is
now a declared per-run parameter: `-response-bound` (`RecorderConfig.MaxResponseBytes` /
`GatewayConfig.MaxResponseBytes`). The hermetic default stays the spec's 64 KiB and hermetic record
bytes are unchanged (the applied bound and body length are only recorded when a non-default bound is
applied). Live runs declare **1 MiB**. The applied bound is run identity, recorded in the revision
string and on every call record beside `applied_reasoning_effort`/`applied_max_output_tokens`, and
every call record also carries that response body's actual buffered byte length, so the headroom is
auditable rather than assumed. Recorded bodies stay raw: `encrypted_content` is never stripped.

**Defect found and fixed (stated plainly).** The 64 KiB recorded-body bound was itself a silent
decision-loss channel at max effort: ~30 % of max-effort calls exceeded it, were recorded `oversized`
with no extractable content, and fell back to the frozen cascade with no model action — invisible
except as an `oversized` status in the ledger. The storage implication is why the bound is raised for
live runs only rather than globally: a max-effort body is ~90 KB on average (~350 MB for Stage 1's 3888
calls), which the hermetic 64 KiB bound and the default record path were never sized for.

**Post-repair probe (gate for cells, verbatim).** Same smoke command plus `-response-bound 1048576`:
10 call records, **10 `ok`, 0 `oversized`, 0 other statuses**; every record carries
`applied_reasoning_effort: "max"`, `applied_max_output_tokens: 32768`, `response_byte_bound: 1048576`;
recorded body lengths 15 462–120 239 bytes (three of ten over the old 64 KiB bound, confirming the loss
channel). No new status class appeared. Artefacts: `.tmp/35-smoke-live-runs/`, `.tmp/35-smoke-live.json`,
`.tmp/35-smoke-live.txt`, log `.tmp/35-smoke-live.log`; unit check
`internal/cognition/client_applied_bound_test.go` (1 MiB bound accepts a 100 KiB body and records
bound+length; the hermetic default still records `oversized`).

## Stage 1 progress (2026-10-18, live)

First launch (00:10) was healthy but was killed by my own cleanup: `pkill -f 35-vs2-bin` matched the
wrapper's own `bash -c` command line and terminated the shell that owned the run. 193 calls, ~$0.22 of
max-effort probe-grade spend, abandoned — not replicate data. The pid file named the wrapper
`bash -c`, not the binary, and an empty log until exit is normal (the report is printed at the end), so
"empty log + missing pid" was a bad liveness test, not evidence of death.

Relaunch 00:21, truly detached: `setsid nohup sh .tmp/35-run-stage1.sh > .tmp/35-stage1.log 2>&1 <
/dev/null &`, binary pid 1475000 with `sid=pgid=1475000`, root wiped first (`rm -rf
.tmp/35-stage1-runs`), same declared command. Observed throughput: 25.5 calls/min in the first 2:42,
13.3/min over the next 8 min, 6.5/min over the next 8 min (mean latency ~74 s/call under real load
versus the probe's 17.7 s at `-parallel 8`); at the measured rate stage 1 needs ~10 h wall, so the run
is left detached and polled. No non-`ok` status recorded so far.

Poll log (stage 1, detached pid 1475000; `python3 .tmp/35-poll.py` appends to
`.tmp/35-stage1.progress`, one line per poll: epoch, total calls, calls/min since the previous poll,
projected hours remaining, status histogram):

| local time | total calls | calls/min (since prev poll) | statuses | projection |
|---|---|---|---|---|
| 00:56 | 332 | — | all `ok` | — (first sample) |
| 01:05 | 409 | 8.53 | all `ok` (409/409) | ~6.8 h remaining |
| 01:15 | 479 | 7.42 | all `ok` | ~7.7 h |
| 01:24 | 560 | 8.58 | all `ok` | ~6.5 h |
| 01:34 | 638 | 8.23 | all `ok` | ~6.6 h |
| 01:43 | 732 | 10.13 | all `ok` (732/732) | ~5.2 h |
| 01:52 | 840 | 11.29 | all `ok` | ~4.5 h |
| 02:02 | 948 | 11.32 | all `ok` | ~4.3 h |
| 02:11 | 1073 | 13.17 | all `ok` | ~3.6 h |
| 02:21 | 1228 | 16.16 | all `ok` (1228/1228) | ~2.7 h |
| 02:31 | 1405 | 18.45 | all `ok` | ~2.2 h |
| 02:40 | 1608 | 21.29 | all `ok` (1608/1608) | ~1.8 h |
| 02:50 | 1851 | 25.36 | all `ok` | ~1.3 h |
| 02:59 | 2120 | 28.21 | all `ok` (2120/2120) | ~1.0 h |
| 03:09 | 2380 | 27.71 | all `ok` | ~0.9 h |
| 03:18 | 2662 | 29.58 | all `ok` | ~0.7 h |
| 03:28 | 2949 | 30.08 | all `ok` (2949/2949) | ~0.5 h |

Concurrency check (supervisor request): one throwaway probe at `-parallel 2`, cell `f10-s9` (not a
declared cell), H=4, max effort, run while stage 1 was live → 21 calls in 307.9 s = 4.09 calls/min
(0.5 calls/worker/min ≈ 29 s/call). Stage 1 at that moment ran 7.06 calls/min across 6 cells
(0.88 calls/worker/min ≈ 68 s/call). Stage 1 has only 6 cells, so `-parallel 8` yields at most 6
concurrent calls: the concurrency knob is already saturated by the cell count, and the per-call
latency (29-68 s at max effort under live load, versus 17.7 s in the idle probe) is provider-side.
Conclusion: no throughput gain is available from raising `-parallel`; the stage runs at what the
provider gives, and the projection stands at ~7 h from 01:05.

## Resume instructions (state at 01:15, stage 1 live and detached)

Running: stage 1, binary `.tmp/35-vs2-bin` (sha256 recorded below), pid in `.tmp/35-stage1.pid`, log
`.tmp/35-stage1.log`, root `.tmp/35-stage1-runs`, outputs `.tmp/35-stage1-evaluation.{json,txt}`.
Liveness: `python3 .tmp/35-poll.py` (prints total, rate, ETA, status histogram and appends to
`.tmp/35-stage1.progress`). Kill by the exact pid only; never `pkill -f`.

Remaining work, in order:

1. **Stage 1 gate.** When `.tmp/35-stage1-evaluation.json` exists: record its sha256 and the text's, and
   reject the stage if any cell is short of H=72, halted, or carries non-`ok` statuses beyond genuine
   model failures (`incomplete`/transport reported with counts). Report the stage-1 noise floor
   (`-curve-in .tmp/35-stage1-evaluation.json`).
2. **Stage 2.** `setsid nohup sh .tmp/35-run-stage2.sh > .tmp/35-stage2.log 2>&1 < /dev/null &` — four
   declared invocations (R at none/low/medium/high), 2 cells × 3 reps each, roots
   `.tmp/35-stage2-<effort>-runs`.
3. **Stage 3.** `setsid nohup sh .tmp/35-run-stage3.sh > .tmp/35-stage3.log 2>&1 < /dev/null &` — A0/P/U/A
   at low, 2 cells × 3 reps, root `.tmp/35-stage3-runs`.
4. **Curve report over every stage**:
   `.tmp/35-vs2-bin -curve-in .tmp/35-stage1-evaluation.json,.tmp/35-stage2-none-evaluation.json,.tmp/35-stage2-low-evaluation.json,.tmp/35-stage2-medium-evaluation.json,.tmp/35-stage2-high-evaluation.json,.tmp/35-stage3-evaluation.json -curve-out .tmp/35-curve-report.txt`
   (each file's path names its stage, which is how the report groups comparisons).
5. **Gates**: `go build ./...`, `go vet ./...`, `gofmt -l`, `git diff --check`, and
   `go test -timeout 30m ./...` (the `internal/world` package alone takes ~13.5 min; a 600 s
   per-package timeout is a false failure — ticket 34 recorded 814.994 s).
6. **Docs**: `CHANGELOG.md` (dated entries for this ticket), `docs/VERTICAL-SLICES.md` (M2 row and the VS2
   section: depth and redundancy are now first-class axes, the acceptance criterion is curve-based, the
   ticket-34 NOT-accepted status stands), and `.scratch/24-vs2-attention/spec.md` only if a status line
   must point here.
7. **Record** every artefact's sha256 and the exact commands in this file, then commit; do not push.

**Gates at code tip (before stage 1 finished), verbatim** (`sh .tmp/35-gates.sh` →
`.tmp/35-gates.out`): `go build ./...` PASS; `go vet ./...` PASS; `gofmt -l` (tracked .go) clean after
one reformat commit; `git diff --check` clean; `go test -timeout 30m ./...` **all packages ok**, with
`internal/world` at 995.587 s and `cmd/vs2` at 57.225 s, zero FAIL lines. (A bare `go test ./...` fails
`internal/world` on the 600 s per-package default timeout; that is the package's duration, not a
regression — ticket 34 recorded 814.994 s.)

## Stage 1 result (live, 2026-10-18 00:21 → 04:52) and the corrected design

Command as declared (72 h, f10-s0 + f12-s0, 3 reps, `-effort max -max-output 32768
-response-bound 1048576 -attempt-timeout 300 -call-deadline 900 -live -parallel 8 -budget-min 300`),
exit 0, report emitted, **6/6 cells complete, 5106 call records, 5106 `ok`, zero non-`ok` statuses**.
Artefacts: `.tmp/35-stage1-evaluation.json` sha256
`c75c807f4e13c2183a856bd9eaadcc6358c6868a8b639b46eb22007f7fc0e4b2`, `.tmp/35-stage1-evaluation.txt`
`193f388b3fe0eb8d3c7d6efb6707e78cddf3f73c8d5f068567dcdf79794d6121`, curve report
`.tmp/35-stage1-curve.txt`, progressive ledger `.tmp/35-stage1.progress`. Calls per run 781–931
(full-wake at 72 h), latency median 11.3 s / p90 37.9 s / mean 17.7 s / max 213.1 s, tokens 1 018 433 in
+ 5 942 598 out (f10-s0, 3 reps) and 1 172 065 in + 6 004 623 out (f12-s0, 3 reps). Model decisions
applied 4316 of 5106 escalations; the 790 fallbacks are 731 **`inadmissible`** (the model proposed an
action the observation forbids — a genuine model outcome, recorded `ok`) and 59 `malformed`; this is the
decision-error rate the report shows (19.6 % at f10-s0, 12.0 % at f12-s0) and it is not a harness
failure.

**FINDING A — the aggregate noise floor at max effort is ~1 actor / ≤5 % relative.** From the stage-1
curve report and the per-hour replicate bands: population range across the 3 replicates is at most 1
actor (mean range 0.18 at f10-s0, 0.68 at f12-s0; worst relative SD ~3.7–4.4 %), births cumulative max
range 1, starvation deaths max range 1, age deaths exactly 0 in every replicate, exchange volume exactly
0 in every replicate, max capital max range 1. The model's decision-level stochasticity (measured at
48–64 % three-sample self-consistency) therefore largely washes out of the aggregate KPI curves at this
fixture. Recorded as the study's noise floor, not as a result about any condition.

**FINDING B — 72 h is the wrong horizon, corrected by supervisor re-plan.** At 72 h the world is in its
growth phase (population 10→11–13, births 1–2, starvation 0–1, age deaths and exchange identically
zero), so the largest possible between-condition effect is about one actor — the size of the noise
floor. The die-off that gives the fixture dynamic range happens between roughly h96 and h156 (ticket-34
f10-s0: 12 at h96, 10 at h108, 5 at h120, 3 at h132, 1 at h144, 0 at h156). Stage 1 therefore stands as
the pilot that establishes the noise floor, **not** as the reference for comparisons.

### Re-plan (supervisor, supersedes the stage 2/3 paragraph above; recorded before the first new call)

1. **Horizon 144 h for all remaining stages** (captures the crash through population ≈ 1).
2. **Cell f10-s0 only** (the ticket-19 decision-sensitivity map marks f10/f11 reliable and f12 fragile),
   **3 replicates**.
3. **Stage 2 = effort ladder at full wake: `none`, `low`, `high`, `max`** — `medium` is dropped because
   its decision-level agreement equals `low` at 1.4× the cost, so it adds cost without information.
4. **Stage 3 unchanged**: arms A0/P/U/A at effort `low`, same cell and replicates.
5. New artefact roots and report names; stage 1 is not overwritten:
   `.tmp/35-stage2-<effort>-runs` / `.tmp/35-stage2-<effort>-evaluation.{json,txt}`,
   `.tmp/35-stage3-runs` / `.tmp/35-stage3-evaluation.{json,txt}`.
6. Estimated spend ≈ $10, taking the lane to ≈ $16.5 against the $25 hard cap; stop and report if
   measured cost runs materially above it, and report if per-run calls differ materially from ~1600 at
   144 h.

**The both-cell clause is met only trivially from here on.** The frozen decision rule requires a
separation "in **both** cells" (the declared f10-s0 and f12-s0). This re-plan reduces every remaining
condition to the single cell f10-s0, so for stages 2 and 3 and for every v2 verdict the both-cell
condition is satisfied *vacuously* — there is one cell and the separation holds in it — and never
substantively, because no second cell can disagree. A separation that would reproduce at f12 is therefore
**untested** for every result after stage 1, and each verdict section below says so.

**Report-labelling defect fixed before the next live report** (`cmd/vs2`): the report's status, label,
honest-limit and EC-9 arm text are now parameterised by run mode. A live run prints
`status: live/real-model`, a label naming the real model, endpoint, applied effort/output-bound/
response-bound, the run-only spend authorisation and the UNPRICED cost state (never a fabricated
spend), and an EC-9 arm line that says the real-model arm RAN and that no acceptance is claimed here.
Hermetic reports keep their exact previous text, and EC-9 now checks the label against the run mode
instead of the frozen hermetic constant.

## Defect recorded, not hidden: the running stages used the pre-label binary

`.tmp/35-vs2-bin` was built at 00:26, before the mode-parameterised label commit (`df54d93`, 04:55), so
the reports emitted by stage 1 and by stages 2/3 carry the stale hermetic header: `status:
synthetic/model-conditional`, `label: … hermetic FakeClient (no network, no authorised spend) … awaits
real-model spend authorisation`, and an EC-9 arm line saying the real-model arm is blocked on spend —
all FALSE for a live run. **The data is sound**: per-call ledgers, KPI series, call/token counts and
curve statistics are produced by unchanged code paths and are correct. Only the report header, label
and EC-9 arm text are stale. Repair after the runs finish, without re-spending: rebuild with the label
fix, regenerate each stage's report from its recorded evaluation JSON through a report-only path, and
record the sha256 of the original and the regenerated artefacts side by side with the exact difference.
The originals stay in place; nothing is silently rewritten.

Correction of an intermediate reading (supervisor, then corrected): a count of 3634 "calls in
`.tmp/35-stage2-none-runs`" was the SUM over the three replicate directories, not one run. The per-run
count under `none` is 1280–1442 calls over 144 h, i.e. ~9.4 calls per simulated hour for a population
of ~10–12, and there is no evidence of population growth under `none`. Per-run, not per-root, is the
unit everywhere in this ticket.

Stage-2 result format (fixed before reading the data, so it is not fitted to it): for each effort level
and each replicate, report population at hours 0/24/48/72/96/120/143, final births, starvation deaths,
age deaths, and realised calls/tokens; state the effective effect size next to the replicate band,
because the 72 h band was ~1 actor and the dynamic range lives in the 144 h crash. No level is called
better; separation and noise are reported side by side.

## Stage 2 results (live, 144 h, cell f10-s0, 3 replicates; effort ladder at full wake)

Declared command per effort: `.tmp/35-run-stage2.sh` → `-arms R -cells f10-s0 -reps 3 -hours 144
-effort <e> -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 -live
-parallel 8 -budget-min 720`; roots `.tmp/35-stage2-<effort>-runs`, reports
`.tmp/35-stage2-<effort>-evaluation.{json,txt}`.

Per-replicate curves (population at h 0/24/48/72/96/120/143, final cumulative births/starvation/age
deaths, realised calls and tokens):

| effort | rep | population | births | starvation | age deaths | calls | in/out tokens | decision errors |
|---|---|---|---|---|---|---|---|---|
| none | r1 | 10 11 11 11 11 5 2 | 2 | 1 | 10 | 1363 | 587 353 / 26 893 | 803/1362 |
| none | r2 | 10 12 12 12 12 5 2 | 2 | 0 | 10 | 1442 | 620 968 / 28 381 | 818/1441 |
| none | r3 | 10 11 10 10 10 5 2 | 2 | 2 | 9 | 1280 | 551 147 / 24 733 | 616/1279 |
| low | r1 | 10 11 11 11 11 4 1 | 1 | 0 | 11 | 1306 | 562 642 / 287 179 | 713/1305 |
| low | r2 | 10 10 10 10 10 3 0 | 0 | 0 | 10 | 1166 | 502 588 / 248 240 | 704/1166 |
| low | r3 | 10 10 10 10 10 3 0 | 0 | 0 | 10 | 1170 | 502 587 / 248 172 | 678/1166 |
| high | r1 | 10 11 11 11 11 4 1 | 1 | 0 | 10 | 1304 | 562 209 / 624 125 | 586/1304 |
| high | r2 | 10 10 10 10 10 3 0 | 0 | 0 | 10 | 1168 | 502 589 / 562 693 | 520/1166 |
| high | r3 | 10 10 10 10 10 3 0 | 0 | 0 | 10 | 1166 | 502 588 / 568 924 | 458/1166 |
| max | — | *pending (still running; report when it lands)* | | | | | | |

Exchange volume (`wages_cum`) is identically zero at every effort and every replicate at this cell, so
it carries no information here and is reported as such rather than dropped.

**Effect size against the replicate band, stated side by side (no level is called better):**

- `none` vs `low`: births cumulative differs by 1.6 RMSE over hours (SNR 4.04, first separation at hour
  7, 138 consecutive separating hours); max capital SNR **4.83** (first separation hour 6, **117**
  consecutive hours, run starting h28); population SNR 1.74 with a non-contiguous 44-hour run;
  starvation deaths SNR 1.42 and age deaths SNR 1.43 do not separate. So the pair is
  **distinguishable** under the pre-declared rule via births and max capital, and the replicate band for
  those KPIs is small (noise 0.40 and 0.35) relative to the difference (1.63 and 1.69).
- `none` vs `high`: the same picture — births SNR 4.06 (separation running 138 hours from hour 7), max
  capital SNR **4.81** (first separation h6, 115 hours — this is the row an earlier draft mis-transcribed
  into the `none` vs `low` line); population 1.74. **Distinguishable** via births and max capital.
- `low` vs `high`: **not distinguishable at this noise floor** — no KPI reaches SNR 2 with a sustained
  separation. Their per-replicate population curves are effectively the same shape (h143 = 0/1/0 at
  both), and the decision-error rates differ (low 55–60 %, high 39–45 %) without that difference
  resolving into the aggregate KPIs.
- **Both-cell clause**: these verdicts are single-cell (f10-s0), so the rule's "in both cells" condition
  is met vacuously (see the re-plan note); reproduction at f12-s0 is untested.
- The replicate band at this horizon is larger than at 72 h: population noise 0.66 actors and births
  noise 0.40 births, because the crash amplifies sampling differences (h120→h143 differ by one to two
  actors between replicates). The 72 h band (~1 actor ceiling, ≤5 % relative) was therefore the wrong
  yardstick for comparisons, which is exactly why the horizon was corrected to 144 h.
- Comparability caveat, recorded: the *volume* of model decisions differs by effort (the escalation
  stream and its error mix differ), so these are curves of the same fixture under different decision
  policies, not an equal-contribution comparison; the call-count axis (realised calls, reported above)
  is the declared budget axis and is not equalised.

## Stage 3 result (live, 144 h, cell f10-s0, 3 replicates; arms A0/P/U/A at `low`)

Command: `.tmp/35-run-stage3.sh` → `-arms A0,P,U,A -cells f10-s0 -reps 3 -hours 144 -effort low
-max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 -live -parallel 8
-budget-min 720`; root `.tmp/35-stage3-runs`, report `.tmp/35-stage3-evaluation.{json,txt}`. 12/12 cells
ran, 2265 call records, all `ok`.

| arm | rep | population h0/24/48/72/96/120/143 | births | starvation | age deaths | calls | in/out tokens | errors |
|---|---|---|---|---|---|---|---|---|
| A0 | r1–r3 | 10 12 12 12 12 5 2 | 2 | 0 | 11 | 0 | 0/0 | — |
| P | r1 | 10 12 12 12 12 5 2 | 2 | 0 | 11 | 244 | 105 233 / 49 738 | 127/244 |
| P | r2 | 10 12 12 12 12 5 2 | 2 | 0 | 11 | 244 | 105 233 / 50 560 | 125/244 |
| P | r3 | 10 12 12 12 12 5 2 | 2 | 0 | 11 | 244 | 105 233 / 50 543 | 121/244 |
| U | r1 | 10 11 10 10 9 5 3 | 3 | 4 | 6 | 256 | 110 442 / 54 376 | 100/256 |
| U | r2 | 10 11 10 10 10 4 1 | 1 | 1 | 9 | 256 | 110 379 / 53 585 | 113/256 |
| U | r3 | 10 9 9 8 7 5 2 | 3 | 6 | 6 | 256 | 110 439 / 54 728 | 69/256 |
| A | r1 | 10 12 12 12 12 5 2 | 2 | 0 | 11 | 256 | 110 434 / 58 070 | 105/256 |
| A | r2 | 10 12 12 12 12 5 2 | 2 | 0 | 10 | 256 | 110 407 / 52 069 | 106/256 |
| A | r3 | 10 10 10 10 10 3 0 | 0 | 0 | 10 | 253 | 109 054 / 53 881 | 134/253 |

**The finding stage 3 exposes is the decision contract, not the allocation policy.** Per-arm role-a
tallies from `.tmp/35-stage3-runs/*/calls.jsonl` (summed over replicates, `python3 .tmp/35-admis.py`):

| arm | applied | rejected | inadmissible | malformed | rejected share |
|---|---|---|---|---|---|
| A | 420 | 345 | 321 | 24 | 45 % |
| P | 359 | 373 | 338 | 35 | 51 % |
| U | 486 | 282 | 258 | 24 | 37 % |

40–53 % of every arm's model answers are **well-formed JSON proposing an action the world refuses**, and
schema-malformed is only 2–6 %. This corrects the ticket-34 reading that "100 % of model failures were
truncation": at `low` effort with a 32768 bound truncation is gone, and the dominant loss is
admissibility. The proposed-action × refusing-gate table is unambiguous:

| proposed action | refusing gate | count |
|---|---|---|
| `eat-stored` | no granary stock, or a stored meal was already taken this hour | 882 |
| `gather` | no stocked home slot | 32 |
| `provision` | bag not empty, or no granary stock, or a meal already taken | 3 |

So the model answers `eat-stored` almost always, and that action is inadmissible for most hours. The
consequence is that this matrix compares **admissibility luck, not allocation quality**: P's applied
decisions turn its world into A0's DEMOGRAPHIC world exactly (12→2 population, 2 births, 0 starvation,
11 age deaths in every replicate; its wealth and granary state still differ), A reproduces A0 in two replicates and diverges once (0 births, 0 alive at h143), and U — the
arm with the most applied decisions (486) — diverges hardest (3/2/1 alive, 3/3/1 births, 4/6/1
starvation deaths). Recorded as the stage-3 result; no arm is called better, and the capacity to
distinguish allocation policies is not demonstrated by this data.

Corresponding curve comparison (`-curve-in .tmp/35-stage3-evaluation.json`, `.tmp/35-stage3-curve.txt`),
verbatim per pair and with the numbers that matter:

- **A0 vs P: every population/birth/starvation/age-death curve is identical** (RMSE 0.0000, "both
  deterministic (identical curves)"); the only KPI that separates is `granary_hist[0]`. P's world *is*
  the no-model world on the primary KPIs.
- **A0 vs A: population SNR 0.918, births 0.920, age deaths 1.633 — none separates**; the pair is
  distinguishable only through `max_capital` (SNR 8.522, first separation h17, 109 consecutive hours
  from h35), i.e. a structural capital difference, not a demographic one.
- **P vs U: population SNR 3.430 (first separation h6, 95-hour run from h20)** and starvation deaths
  SNR 2.122 (120-hour run from h25) — U is separated from both P and A0.
- **A vs U** separates through starvation deaths; **A vs P** only through `max_capital`.

Methodological caveat this exposes: against the deterministic A0 control the denominator is a one-sided
noise floor, so a structural difference reaches SNR ≥ 2 easily (max capital 0 vs 2). "Distinguishable
from A0" is therefore weak evidence; the informative comparisons are between two stochastic arms (A vs
U, P vs U), where the replicate band is real. Those outcomes measure how much of each arm's model answer
survives admissibility, not which policy is better.

**`n/a (both deterministic)`, and what depended on the earlier `+Inf` convention** (corrected after
review; the frozen criterion says that when both conditions are deterministic the SNR is reported
`n/a (both deterministic)` with the raw RMSE of the difference, never as an infinite ratio). The first
version of this report printed `+Inf` for that case. Recomputing with the literal rule changes exactly
three verdict lines across the three curve reports, and nothing else:

1. **`A0 vs P` (this stage-3 report) flipped from DISTINGUISHABLE to NOT distinguishable** — its only
   driving KPI was `granary_hist[0]`, whose SNR was `+Inf` because both conditions are deterministic.
   That single convention produced the pair's verdict, so the corrected statement is that P is not
   distinguishable from A0 at all.
2. **`A0 vs A1` and `A1 vs P` (v2 and depth reports) keep their DISTINGUISHABLE verdict, but their
   driving KPI changes**: it was `population` via `+Inf` (both deterministic) and is now `max_capital`
   (SNR 16.57 under a one-sided noise floor, first separation h5, 133 consecutive hours), which is a real
   ratio. The verdicts stand; only their attribution was wrong.
3. No other pair changed. Every remaining `+Inf` row either had no sustained separation or was not a
   driver, and all `n/a` rows are now marked as the rule's declared state.

## EC-1 at 144 h is a horizon artefact, not a regression (proved for free)

The stage-3 report's EC-1 band reads, verbatim: `A0 cells checked=1 missing=0 declared=12;
mismatches=f10-s0:alive_by_hour,f10-s0:event_body_bytes,f10-s0:events,f10-s0:history_bytes; …` →
FAIL. The comparison target is the recorded ticket-19 evidence, produced at **168 h**, while the A0 cell
ran at **144 h**, so the four mismatches are the expected truncation of every hourly series and event
stream, not a substrate change.

Proof, at zero spend because A0 makes no model calls: `.tmp/35-run-ec1.sh` →
`.tmp/35-vs2-bin -arms A0 -cells f10-s0 -hours 168 -replay=false`, root `.tmp/35-ec1-runs`, report
`.tmp/35-ec1-evaluation.json` sha256 `ed01e3229d93def1fceb34e8450c5fc4d081dabda5d52538569d152eb3234bb8`,
text `ed761e124a65662f220291cbeec30eb23739ceba3701b1db9cc0817607e4dd6e`. Its EC-1 line, verbatim:
`A0 cells checked=1 missing=0 declared=12; mismatches=none`. **The A0 cell is byte-identical to the
recorded ticket-19 evidence at the matching horizon**, so the continuity witness holds and the 144 h
FAIL is a horizon artefact. (The band still prints FAIL because it requires all 12 declared cells to be
present; the honest statement is "mismatches=none at the declared cell, checked at the evidence's own
168 h horizon".)

## Admissibility quantification and the `natural-claim/v2` fix (validated by a bounded probe)

**Quantification (from the stage-3 ledgers, no spend, `.tmp/35-admis.py`)**: of 917
rejected-as-inadmissible answers across the three arms, 882 proposed `eat-stored` and were refused for
"no granary stock, or a stored meal already taken this hour"; 32 proposed `gather` with no stocked home
slot; 3 proposed `provision` with a non-empty bag or no granary. So the loss is one dominant,
mechanical mistake: the model cannot see which of the legal actions the frozen gates admit this hour.

**Contract fix, behind a NEW version**: `natural-claim/v2`
(`ActorSeamSchemaVersionAdmissible = 2`, `NaturalClaimPromptTemplateAdmissible`) adds one field,
`admissible_actions`, to the canonical request. It is computed by the same level-1 ladder the response
check runs (`admissibleActions` → `admissibleAction`), from the request's own observation only
(granary stock, last stored meal hour, bag units, home slots), so no privileged state and no other
actor's data enters the request and the EC-6 observation boundary is unchanged; one ladder, one home.
`natural-claim/v1` remains the declared default, its request bytes are byte-identical (the new field is
`omitempty`), and a run records `claim_contract` in run identity and on every cell row. The response
schema version follows the contract, so a schema-1 answer to a v2 request is a recorded `malformed`
fallback (unit-tested in `internal/cognition/claim_contract_test.go`).

**Bounded validation probe** (`.tmp/35-claimv2-probe.py`, results `.tmp/35-claimv2-results.json`): 30
recorded inadmissible requests from stage 3, replayed at `effort=low` with the 32768 bound under each
contract — 60 real calls, 33 365 input + 9 000 output tokens ≈ $0.008 of probe spend, declared here and
nowhere else.

| contract | calls | admissible | inadmissible | malformed | inadmissible rate | answers |
|---|---|---|---|---|---|---|
| v1 (baseline) | 30 | 3 | 26 | 0 | **86.7 %** | 26 `eat-stored`, 4 `gather` |
| v2 (`admissible_actions` stated) | 30 | 30 | 0 | 0 | **0 %** | 29 `gather`, 1 `eat-stored` |

The defect is essentially the whole of the admissibility loss, and stating the admitted set removes it
in this sample (and costs *fewer* output tokens: 3 987 versus 5 013). Per instruction, the allocation
fix (`vs2-value/2`: marginal/staleness term plus a per-actor coverage floor) and any re-run are NOT
started; they await the supervisor's go-ahead. This probe is a 30-request sample at one effort level on
one cell's requests, not a matrix result, and it does not by itself establish a between-arm effect.

## Second declared study: `natural-claim/v2` + `vs2-value/2` (supervisor go-ahead, declared before running)

Two declarations, both fixed before the run:

1. **`vs2-value/2`** = `vs2-value/1` plus exactly two terms, integer fixed point, no other change:
   (i) **marginal/staleness**: an actor reasoned about `d` hours ago keeps a priority factor ramping
   linearly from `DefaultStalenessFloorBasisPoints` (25 %) at `d = 0` to full at
   `DefaultStalenessRefreshHours = 12`; (ii) **coverage floor**: an actor below its fair share of the
   realised budget (`spent / roster size`) is boosted by `1 + belowShare`, so the boost vanishes exactly
   at share. The feature set version is untouched; selection/weighting only. Unit-tested:
   `TestValueModelV2KeepsCoverageNonDegenerate` (static-observation actor set with persistent urgency:
   vs2-value/1 covers only the first slots and collapses, vs2-value/2 covers every actor within one call
   of every other), and the all-zero-weight uniform reduction still holds.
2. **Coverage is now a reported instrument** (supervisor decision B): every cell carries
   `actor_coverage` (per-actor realised call count), `coverage_min`, `coverage_max` and `coverage_gini`,
   for every budgeted arm.

**Arm set and contract.** One run, contract `natural-claim/v2`, effort `low`, H=144, cell f10-s0, 3
replicates: `A0` (no model), `R` (full-wake reference), `P`, `U`, `A` (`vs2-value/2`), and **`A1`** —
the same adaptive arm and the same request contract but `vs2-value/1`, the cost-benefit pair that
isolates the value-model change from the contract change. `A1` is budgeted like `A` but is not part of
the declared matrix inventory, so the declared-arms list is unchanged.

Command: `.tmp/35-run-v2.sh` → `.tmp/35-vs2-v2bin -arms A0,R,P,U,A,A1 -cells f10-s0 -reps 3 -hours 144
-effort low -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900
-claim-contract natural-claim/v2 -live -parallel 8 -budget-min 720 -root .tmp/35-v2-runs`,
outputs `.tmp/35-v2-evaluation.{json,txt}`. Expected ≈ 7 000 calls ≈ $1.2, taking lane spend to ≈ $18 of
the $25 cap.

**Acceptance framing (pre-declared).** Compare each budgeted arm against the v2 `R` reference and
against each other on the KPI curves, using the replicate band as the denominator (SNR), stating the
horizon's dynamic range; report coverage and admissibility per arm alongside. No arm is called better:
the outcome is whether a separation exceeds the replicate band and by how much. If `A` under
`vs2-value/2` still cannot separate from `P` or from `A0` while its coverage is flat, that is a decisive
negative for this value model as instantiated and is reported as such.

**Cross-contract rule.** No v1 run is compared with any v2 run. Every stage-2 result stands as a
statement about thinking depth under contract v1, with the declared caveat that ~50 % of v1 answers
were inadmissible.

## v2 run result (live, 144 h, effort=low, contract `natural-claim/v2`, cell f10-s0, 3 reps)

Command as declared (`.tmp/35-run-v2.sh`), exit 0, 18/18 cells, **6580 call records, all `ok`**
(`.tmp/35-v2-evaluation.json` sha256 `8efd819533d56b419d74639f6e6ba559fb678f013df7120bf6b8c3102d4bae3d`,
text `857318f46f5c20516daa9b3a89099ba81a48f7ebedc743c950105f8de6563a3d`, curve report
`.tmp/35-v2-curve.txt`). Cost from recorded tokens: 2 921 449 input + 992 927 output = **$0.789** at the
declared prices — a projection made at the time of writing; the authoritative figure is the spend section
below ($13.17 of the $25 cap for the whole lane at completion).

**1. The contract fix works.** Role-a tallies (`.tmp/35-admis.py .tmp/35-v2-runs`): R 3527 applied /
26 rejected, A 763/5, A1 752/7, P 726/6, U 758/10 — and **zero inadmissible in every arm** (the
remaining rejections are all `malformed`, 0.7-1.3 % — exactly A 5/768 = 0.65 %, R 26/3553 = 0.73 %,
P 6/732 = 0.82 %, A1 7/759 = 0.92 %, U 10/768 = 1.30 %). Against v1's 40-53 % inadmissible this is the
whole defect removed. The report label is correct too (`status: live/real-model`).

**2. The value-model fix works on the thing it was built for.** Per-actor coverage (per replicate):

| arm | value model | coverage min-max | Gini | applied/rejected | calls | in/out tokens |
|---|---|---|---|---|---|---|
| R | — | 100-141, 100-141, 31-141 | 0.061 / 0.056 / 0.123 | 3527/26 | 1082-1305 | 1 574 682 / 515 858 |
| P | vs2-value/2 | 17-24 (all three) | 0.061 | 726/6 | 244×3 | 324 961 / 107 504 |
| U | vs2-value/2 | 3-36, 2-37, 3-42 | 0.303 / 0.334 / 0.367 | 758/10 | 256×3 | 342 936 / 130 839 |
| **A** | **vs2-value/2** | **15-34, 15-33, 4-40** | **0.146 / 0.142 / 0.229** | **763/5** | 256×3 | 341 904 / 124 489 |
| **A1** | **vs2-value/1** | **2-76, 2-73, 2-83** | **0.502 / 0.490 / 0.589** | **752/7** | 253×3 | 336 966 / 114 237 |

The collapse is gone under `vs2-value/2`, and the `A1` arm — same contract, v1 value model — still
collapses, so the collapse was the value model, not the contract.

**3. What the demographic curves say (and it cuts against the redesign).**

| arm | population h0/24/48/72/96/120/143 (per replicate) | births | starvation | age deaths |
|---|---|---|---|---|
| A0 | 10,12,12,12,12,5,2 (identical ×3) | 2 | 0 | 11 |
| P | identical to A0 ×3 | 2 | 0 | 11 |
| A | 10,12,12,12,12,5,2 (r1, r2) and 10,12,11,10,10,3,2 (r3) | 2 | 0/0/2 | 10/11/8 |
| U | 10,10,7,9,8,6,3 / 10,11,9,9,9,5,2 / 10,10,10,7,6,4,2 | 3/3/2 | 5/4/6 | 5/7/5 |
| A1 | 10,10,10,10,10,3,0 ×3 | 0 | 0 | 10 |
| R | 10,10,9,9,9,3,0 / 10,10,10,10,10,3,0 / 10,11,11,11,11,4,1 | 0/0/1 | 1/0/0 | 9/10/11 |

**4. Two conclusions, stated plainly.**

> **(a) Periodically-sampled model reasoning has ZERO DEMOGRAPHIC effect in this fixture.** P applies 244
> model decisions per replicate (all three applied 241-243 of 244) and **every demographic KPI —
> population, births, starvation deaths, age deaths, wages/exchange — is identical to A0 at every hour
> and in every replicate (RMSE 0.0000)**. It is NOT identical on wealth state: **`max_capital` differs at
> 70-76 hours per replicate (76/70/74), first divergence at h19 (r1) and h13 (r2, r3)**, and
> `capital_hist[2]`/`[3]` differ over the same hours (the earlier draft's "every KPI, every replicate,
> RMSE 0.0000" was an overclaim, corrected here). That demographic null can no longer be explained by
> admissibility loss, because `natural-claim/v2` removed it; it is a real property of the periodic arm's
> decision points at this cell and horizon.

> **(b) The v1 value model — whose concentrated coverage I had called a defect — is by far the closest to
> the R reference, while the "fixed" v2 model is indistinguishable from A0 *on the demographic KPIs*.** On
> the declared fidelity metric the arm labelled buggy was **the closest to R**, not better (dominance
> language is reserved to the frozen EC-3 instrument). A1 vs R is the only pair in the whole run that is
> **not distinguishable** (population SNR 0.346, births 0.817, starvation 0.817, age 0.777, max capital
> 1.764), while A vs R separates on births (SNR 4.03, 139-hour run). A0 vs A does not separate on any
> **demographic** KPI (population 0.83, births 1.63, starvation 0.82, age 0.91) but the tool's own verdict
> for that pair is **DISTINGUISHABLE via `capital_hist[0]`** (SNR 2.39, first separation h14, 120
> consecutive hours) — a wealth-state separation, not a demographic one. Likewise A vs P is
> distinguishable only through `capital_hist[0]`; the demographic separation is what fails, not all
> separation.

**5. The likely reason is the metric's own problem, not an arm ranking.** R (full cognition) produces 0-1
births against A0's 2 and dies out (0.33-1 alive at h143), so "closer to R" currently means "converges on
the no-birth dying trajectory". Fidelity anchored on R therefore rewards suppressing reproduction. Any
claim that A1 "wins" would be a claim that extinction-matched behaviour is desirable, which the spec never
says. **The R anchor is an unvalidated objective**; the open question of a civilization-level distance
measure (Q1) is where this belongs and it is not resolved here.

**SNR table (replicate band as denominator; the fully measured table is `.tmp/35-v2-curve.txt`).**
Dynamic range of this horizon, stated so the numbers are readable: population spans 10-12 (growth cap) to
0-3 (crash) and saturates at the fixture's cap until h96, so population separates mainly through the crash
timing; births span 0-3 and starvation deaths 0-6, which is why they are the primary discriminators here.
Against the deterministic A0 the denominator is one-sided (its replicate SD is 0 everywhere), so any
demographic difference reaches SNR ≥ 2 and `Inf` appears where one side is flat and the other is not —
read those rows as "A0 is deterministic here", not as an infinitely strong effect. Where **both**
conditions are deterministic the frozen rule reports **`n/a (both deterministic)`** with the raw RMSE and
not a ratio, so those KPIs cannot drive a verdict; the table below is regenerated from
`.tmp/35-v2-curve.txt` (`python3 .tmp/35-table.py`) rather than transcribed, after the review found two
transcription errors in an earlier draft.

| pair | population | births | starvation | age deaths | max capital | verdict |
|---|---|---|---|---|---|---|
| A0 vs A1 | n/a | n/a | n/a | n/a | 16.57 | distinguishable via `max_capital` (one-sided floor) |
| A0 vs A | 0.83 | 1.63 | 0.82 | 0.91 | 4.36 | distinguishable via `capital_hist[0]` |
| A0 vs P | n/a | n/a | n/a | n/a | 3.87 | distinguishable via `granary_hist[0]` (SNR 15.43, 46 h run) |
| A0 vs R | 3.09 | 4.07 | 0.82 | 0.89 | 48.30 | distinguishable via population |
| A0 vs U | 3.78 | 1.07 | 7.40 | 5.25 | 2.84 | distinguishable via population |
| A1 vs A | 2.59 | 40.76 | 0.82 | 0.75 | 8.16 | distinguishable via population |
| A1 vs P | n/a | n/a | n/a | n/a | 8.38 | distinguishable via `max_capital` (one-sided floor) |
| A1 vs R | 0.35 | 0.82 | 0.82 | 0.78 | 1.76 | **NOT distinguishable** |
| A1 vs U | 2.19 | 4.79 | 7.40 | 5.21 | 6.68 | distinguishable via population |
| A vs P | 0.83 | 1.63 | 0.82 | 0.91 | 1.44 | distinguishable via `capital_hist[0]` |
| A vs R | 1.68 | 4.03 | 0.35 | 0.32 | 10.59 | distinguishable via births |
| A vs U | 2.50 | 1.05 | 4.07 | 4.30 | 0.96 | distinguishable via population |
| P vs R | 3.09 | 4.07 | 0.82 | 0.89 | 10.55 | distinguishable via population |
| P vs U | 3.78 | 1.07 | 7.40 | 5.25 | 1.31 | distinguishable via population |
| R vs U | 1.60 | 3.20 | 5.72 | 4.34 | 7.79 | distinguishable via births |

`n/a` in this table means both conditions are deterministic on that KPI (the frozen rule's declared state),
not "no difference" and not "an infinite difference" — the raw RMSE and the separation hours are in the
curve report. In the v2 run the pair `A0 vs P` is distinguishable through the **granary** histogram
(`granary_hist[0]` SNR 15.43, 46 consecutive hours from h37; max capital only 3 hours, so it does not
drive), which is a real ratio on a one-sided floor: P's food storage differs from A0 even though every
demographic KPI is identical. In the stage-3 (v1) run the same pair's only driver was `granary_hist[0]`
at `+Inf`, so it fell to `n/a` and the pair became NOT distinguishable — that is the difference between a
real ratio and the `+Inf` convention.

No arm is called better. What the table establishes is which separations exceed the replicate band and by
how much; what it does not establish is that any of them is an improvement, because the reference the
metric measures against is the unvalidated R anchor of point 5.

## Report regeneration (originals kept, both hashed)

The stage-1/2/3 reports were emitted by a binary built before the mode-parameterised label fix
(`df54d93`). They were regenerated from their own recorded evaluation JSONs by the report-only path
`.tmp/35-vs2-finish-bin -relabel …` (no cell ran, no spend). What changed in each regenerated file, and
nothing else: `live: true` (inferred for the legacy JSONs from the recorded applied bounds),
`status: synthetic/model-conditional → live/real-model`, the label (hermetic "no authorised spend" text →
the live text naming model, wire, applied effort/output/response bounds, run-only spend authorisation and
the UNPRICED cost state), the added `applied:` header line (`applied: effort=… output-bound=…
response-bound=… votes=… replicates=…`, which the earlier binary did not print — added to the change list
after review; the path now detects its absence and reports it), and the VS2-EC-9 arm line ("blocked on
spend" → "the live-model arm RAN; no acceptance is claimed here"). Every measured field, series, count and
band decision is byte-identical to the original; the relabelled artefacts were re-generated after the
change-list repair and are byte-identical to the first regeneration (`sha256sum -c` OK on all seven).

| artefact | sha256 |
|---|---|
| `.tmp/35-stage1-evaluation.json` (original) | `c75c807f4e13c2183a856bd9eaadcc6358c6868a8b639b46eb22007f7fc0e4b2` |
| `.tmp/35-stage1-evaluation-relabelled.json` | `6885d29ae8546924f420c0fa09af0944c70f0ea42d2e81234b1458eac2b0fed6` |
| `.tmp/35-stage2-none-evaluation.json` (original) | `6fb3c2ecc161a6c99ac7b9a579f8d017bf507203339ddf792b92ab6f16f2f14b` |
| `.tmp/35-stage2-none-evaluation-relabelled.txt` | `09fce4db307fa96de17b0895b00876781f57bf9e32c1c1f6a64852b8e1274c0a` |
| `.tmp/35-stage2-low-evaluation.json` (original) | `d579aa77b3246ea4ff9240d6161c8f15457e11997429d7943a1006406f5ae8e3` |
| `.tmp/35-stage2-high-evaluation.json` (original) | `c39d4a61ee1099ca38fed5025a92294d2ac63a1e974b2b240eeb5e91dcc08c37` |
| `.tmp/35-stage3-evaluation.json` (original) | `808de6230b0a7491171eaa037444e4e34229a83ecdec4368dafdc1a22076b8ed` |
| `.tmp/35-stage3-evaluation-relabelled.json` | `de2af4bac6faf08be91bf6e26d77f53aff5e6ac562825ff41b4b8b76e4c1a1f9` |
| `.tmp/35-curve-report.txt` (stages 1-3, post-`n/a`-repair) | `14665daaec7b0b06dd3a3e2fde38088a5a76928c631dc9be22c12af31ae846cf` |
| `.tmp/35-curve-report.txt.pre-p14` (superseded: `+Inf` convention + old column label) | `2f1642d66c7350202d501e01e7a724256616fe345d840a0d50ba53b7059bbafa` |

Progressive evidence: `.tmp/35-stage1.progress` (poll log), `.tmp/35-stage23.progress`, per-cell
hash-chained ledgers and checkpoint bundles under `.tmp/35-stage{1,2-*,3,v2}-runs/`.

## Budget reallocation (supervisor, 2026-10-18 ~08:45): v1 ladder truncated, one v2 max run declared

**Stage 2 `max` (v1) STOPPED** at 2246 calls (R-f10-s0-r1 749, r2 719, r3 778 — all `ok`) by its exact
pid 1851847, on the supervisor's decision. Reason: it was completing a ladder under contract v1, where
40-53 % of every arm's answers were inadmissible, so a v1 max-effort point could not be compared with
anything now trusted and would have spent the lane's last margin on a superseded configuration. The
partial artefacts stay at `.tmp/35-stage2-max-runs/` and are **not** a replicate. **The v1 effort ladder is
therefore INCOMPLETE at its top rung** — `none`, `low` and `high` completed and are internally consistent
with each other, and none of them is comparable to any v2 run; the ladder is truncated, not complete.

**Declared before starting (one run, the pair the depth question reduces to):** full-wake `R` at
`effort=max` under **`natural-claim/v2`**, H=144, cell f10-s0, 3 replicates — the same contract as the v2
low reference already held, so low-vs-max at one contract is the contrast that matters.

Command: `.tmp/35-run-v2max.sh` → `.tmp/35-vs2-v2bin -arms R -cells f10-s0 -reps 3 -hours 144 -effort max
-max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 -claim-contract
natural-claim/v2 -live -parallel 8 -budget-min 600 -root .tmp/35-v2max-runs`, outputs
`.tmp/35-v2max-evaluation.{json,txt}`. Expected ≈ 3 600 calls ≈ **$5.7**, taking lane spend to ≈ $23 of the
$25 cap. No `high` under v2 is added: it would land on the cap with no margin for a repair.

**HARD STOP after this run**: no further live model calls in this lane, whatever the result. If the
projected spend would exceed $25, the run is not started and the supervisor is told first.

## Re-grading the held evidence under alternative metric families (zero cost, Q1 input)

Computed from the recorded artefacts only (`.tmp/35-regrade.py` over `.tmp/35-v2-evaluation.json` and the
`.tmp/35-v2-runs/*/calls.jsonl` ledgers; v2 run, 144 h, f10-s0, 3 replicates). Mean with the replicate SD
in brackets — the band is next to every number, because a difference inside it is not a difference.

| arm | pop@96 | pop@120 | pop@143 | births | starvation | age deaths | capital Gini | granary Gini | coverage Gini | calls | r-dist |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A | 11.33 (1.15) | 4.33 (1.15) | 2.00 (0.00) | 2.00 (0.00) | 0.67 (1.15) | 9.67 (1.53) | 0.85 (0.06) | 0.86 (0.00) | 0.17 (0.05) | 256 (0) | 1.576 |
| A0 | 12.00 (0.00) | 5.00 (0.00) | 2.00 (0.00) | 2.00 (0.00) | 0.00 (0.00) | 11.00 (0.00) | 0.89 (0.00) | 0.86 (0.00) | 0.00 (0.00) | 0 (0) | 1.833 |
| A1 | 10.00 (0.00) | 3.00 (0.00) | 0.00 (0.00) | 0.00 (0.00) | 0.00 (0.00) | 10.00 (0.00) | 0.00 (0.00) | 0.00 (0.00) | 0.53 (0.05) | 253 (0) | 0.205 |
| P | 12.00 (0.00) | 5.00 (0.00) | 2.00 (0.00) | 2.00 (0.00) | 0.00 (0.00) | 11.00 (0.00) | 0.89 (0.00) | 0.86 (0.00) | 0.06 (0.00) | 244 (0) | 1.833 |
| R | 10.00 (1.00) | 3.33 (0.58) | 0.33 (0.58) | 0.33 (0.58) | 0.33 (0.58) | 10.00 (1.00) | 0.00 (0.00) | 0.00 (0.00) | 0.08 (0.04) | 1184 (113) | 0.686 |
| U | 7.67 (1.53) | 5.00 (1.00) | 2.33 (0.58) | 2.67 (0.58) | 5.00 (1.00) | 5.67 (1.15) | 0.78 (0.11) | 0.83 (0.05) | 0.33 (0.03) | 256 (0) | 1.747 |

Separation statistic per pair, `|mean difference| / pooled replicate SD` (identical curves report `-`;
`inf` means both sides are deterministic and the means differ):

| pair | pop@96 | pop@120 | pop@143 | births | starvation | age deaths | capital Gini | granary Gini | coverage Gini | calls |
|---|---|---|---|---|---|---|---|---|---|---|
| **A vs P** | 0.82 | 0.82 | – | – | 0.82 | 1.23 | 0.82 | – | **3.20** | inf |
| **A vs A0** | 0.82 | 0.82 | – | – | 0.82 | 1.23 | 0.82 | – | **4.98** | inf |
| A vs A1 | 1.63 | 1.63 | inf | inf | 0.82 | 0.31 | 18.78 | inf | 6.88 | inf |
| A vs R | 1.23 | 1.10 | **4.08** | **4.08** | 0.37 | 0.26 | 18.78 | inf | 2.11 | 11.66 |
| A1 vs R | 0.00 | 0.82 | 0.82 | 0.82 | 0.82 | 0.00 | – | – | 9.58 | 11.69 |
| R vs A0 | 2.83 | **4.08** | **4.08** | **4.08** | 0.82 | 1.41 | inf | inf | 3.01 | 14.87 |
| U vs A0 | **4.01** | 0.00 | 0.82 | 1.63 | **7.07** | **6.53** | 1.41 | 0.82 | 15.00 | inf |
| P vs A0 | – | – | – | – | – | – | – | – | inf | inf |

**Which indicators can even see the differences.** Population, births, starvation deaths and wealth
dispersion all separate `A1`/`R` from `A0`/`P`/`A` and separate `U` from everything — but **no DEMOGRAPHIC
indicator separates `A` from `P` or `A` from `A0`**: every demographic value for that pair sits at or below
1.23 pooled SDs, i.e. inside the replicate band. Among the indicators sampled here the only one outside the
band for that pair is **coverage Gini** (3.20 vs P, 4.98 vs A0) and the realised call count — process
measures, not civilization outcomes.

Two caveats on reading that table as a verdict, both of which the review caught: (i) this table is a
different instrument from the frozen criterion — `|Δmean| / pooled replicate SD` over five sampled hours
— so its `inf`/`–` cells are not the criterion's SNR and it can miss a separation that a full-curve test
finds; (ii) the tool's own full-curve verdicts do separate this pair on **wealth state**: `A0 vs A` and
`A vs P` are DISTINGUISHABLE via `capital_hist[0]` (SNR 2.39, 120 consecutive hours from h25), and in the
v2 run `A0 vs P` is distinguishable via `granary_hist[0]`. So the accurate statement is: **the DEMOGRAPHIC
separation fails for `A` vs `P`/`A0`; a wealth-state and process separation exists.** The value model
changes *how* the budget is spent and, at this fixture and horizon, nothing the demographic KPIs resolve.

**The indicators rank the arms differently from the R-anchored fidelity**, which is the Q1 evidence: by
r-distance the order is `A1 (0.205) < R (0.686) < A (1.576) < U (1.747) < A0 = P (1.833)`, i.e. the arm
closest to the reference is the one that produces **zero births and dies out**, and full cognition R sits
second. By births and survival the order reverses: `A ≈ A0 = P (2.00 births, 2 alive) > U (2.67/2.33) >
R (0.33/0.33) > A1 (0/0)`. So the current R-anchored metric rewards convergence on the no-birth,
dying trajectory, and a reproduction-preserving metric would rank almost the opposite way. Neither is
declared correct here; this is exactly the unvalidated-anchor problem and the empirical input to Q1.

**Behavioural trace (why A ≠ P despite identical demographics)**: the applied action mix per arm is
distinct — A `gather` 56 % / `eat-stored` 42 %, P 71 %/27 %, A1 72 %/27 %, R 80 %/18 %, U 38 %/59 %
(fallbacks ≤ 1 % in every arm). Different decisions, identical demographic outcome for A vs P: the
fixture's gates, not the allocation, absorb the difference.

### Launch record for the v2 max run (liveness discipline)

The first v2-max launch (pid file 2094671) was mis-recorded: `.pid` held the wrapper's pid, the log was
empty, and an earlier `-root` wipe raced the writer, so its ledgers sat at 0 bytes while the process was
still alive. A second launch ran alongside it and both were stopped by exact pid
(`kill 2094682 2105743`), the root was wiped, and one launch was made with `setsid nohup … &`. The
recorded runner pid is the `vs2-bin` process itself; liveness is verified by three independent facts —
the pid exists, the three `calls.jsonl` ledgers grow materially, and the artefacts appear — never by log
content, because the report is written only at exit.

## Spend accounting (from recorded per-call usage, every root)

Cost at the declared gpt-6-luna prices ($0.10/M input, $0.50/M output) over the recorded token totals of
every harness ledger in this lane:

| root | calls | input tokens | output tokens | cost |
|---|---|---|---|---|
| `.tmp/35-stage1-runs` (R@max, 72 h, 2 cells × 3) | 5 106 | 2 190 498 | 11 947 221 | $6.193 |
| `.tmp/35-stage2-none-runs` (R@none, 144 h) | 4 085 | 1 759 468 | 80 007 | $0.216 |
| `.tmp/35-stage2-low-runs` (R@low, 144 h) | 3 642 | 1 567 817 | 783 591 | $0.549 |
| `.tmp/35-stage2-high-runs` (R@high, 144 h) | 3 638 | 1 567 386 | 1 755 742 | $1.035 |
| `.tmp/35-stage2-max-runs` (R@max, v1, STOPPED at 2 246 calls) | 2 246 | 967 924 | 5 608 622 | $2.901 |
| `.tmp/35-stage3-runs` (A0/P/U/A@low, v1) | 2 265 | 976 854 | 477 550 | $0.336 |
| `.tmp/35-v2-runs` (v2 contract, 6 arms @low) | 6 580 | 2 921 449 | 992 927 | $0.789 |
| `.tmp/35-v2max-runs` (R@max, v2 — in flight) | see progress note | | | |
| smoke + `-parallel 2` probe | 47 | 18 888 | 97 421 | $0.050 |
| **harness total so far** | **27 699** | **12 010 318** | **21 981 875** | **$12.19** |

Outside the harness (probe spend, recorded here and nowhere else): 68 max-effort latency probes ≈ $0.90,
60 contract probes ≈ $0.008, one body-shape probe ≈ $0.02, and the abandoned first stage-1 launch (193
calls ≈ $0.22). **Lane spend ≈ $13.4 of the $25 hard cap** at the v2-max launch; the v2-max run is
projected at ≈ $3.6 (full-wake at 144 h, 3 replicates), taking the lane to ≈ **$17**, i.e. below the
supervisor's $23 estimate and inside the cap with margin.

v2-max progress: 89 calls at 09:01, 126 at 09:11, 164 at 09:20 (all `ok`); mean max-effort latency
30.9 s (median 26.9 s) with 3 concurrent cells, i.e. **≈ 3.5-4 calls/min, so the ≈ 3 600-call target
needs ≈ 15-17 h wall**, not the ≈ 4 h the earlier per-call latency suggested. The rate is reported, not
worked around: no throughput lever is left inside the declared design (one cell × 3 replicates caps
concurrency at 3, and the latency is provider-side at max effort). **Hard stop after this run: no further live model calls in this
lane, whatever the result.** When it lands: `-relabel` it, hash it, re-run the curve report
(`-curve-in` the v2 and v2max relabelled evaluations) for the low-vs-max contrast at one contract, and
append the result here.

## Depth rung substitution: max@v2 STOPPED, high@v2 declared (supervisor decision, 2026-10-18 09:2x)

**`max` at contract v2 is STOPPED** by its exact runner pid 2105928 after 165 calls (all `ok`), preserved
at `.tmp/35-v2max-partial-runs/` and **not** a replicate. **`max`@v2 is therefore deliberately unrun.**

Supervisor reasoning, recorded verbatim:

> - The max point at v2 costs 15-17 h of wall time for one data point, at 3 concurrent cells capped by
>   provider-side latency.
> - We already have a v1 depth signal that suggests the interesting transition is above `none`, not above
>   `low`: under v1, `none` reproduced A0's 2 births while `low` full-wake fell to 0-1 births. Under v2 the
>   same structure appears.
> - Decision-level agreement with max is 83 % at `high` versus 72 % at `low`, at ~1/4 the latency and ~1/4
>   the cost per call.
> - Therefore `high` is the right third rung: if `high` does not separate from `low` on the demographic
>   curves, then the limiting factor is the fixture's resolution (population capped at 12, extinction by
>   h156, zero births under full cognition), not the depth of thinking, and `max` would be an expensive way
>   to learn the same thing. If `high` DOES separate, that is a real depth effect and it earns a later,
>   properly budgeted max run.

**Liveness rule for this lane (supersedes pid files):** liveness is decided by the *runner* pid plus ledger
growth and never by a pid file alone. Twice the pid file named a wrapper `bash -c`, which read as "dead"
while the runner was alive (and once the diagnosis caused a healthy run to be killed); an empty log proves
nothing either way, because the report is written only at exit.

**Declared before launching:** full-wake `R` at `effort=high` under `natural-claim/v2`, H=144, cell
f10-s0, 3 replicates, same bounds/timeouts (`-max-output 32768 -response-bound 1048576
-attempt-timeout 300 -call-deadline 900`), `-parallel 8`, detached. Command `.tmp/35-run-v2high.sh` →
`.tmp/35-vs2-v2bin -arms R -cells f10-s0 -reps 3 -hours 144 -effort high … -root .tmp/35-v2high-runs`,
outputs `.tmp/35-v2high-evaluation.{json,txt}`. Expected ≈ 3 600 calls ≈ **$1.8** and ≈ 3 h wall. **Hard stop
after it: no further live model calls in this lane.**

**Acceptance framing (already pre-declared):** compare low vs high curves against the replicate band,
report SNR per KPI, call no arm better, and state the fixture's dynamic range alongside. If `high` separates
from `low` on births/population, say so; if it does not, state that the fixture cannot resolve depth at this
budget.

## Depth result: full-wake at `high` under contract v2 (144 h, f10-s0, 3 reps)

Command as declared (`.tmp/35-run-v2high.sh`), exit 0, 3/3 cells, **3 417 call records, all `ok`**
(`.tmp/35-v2high-evaluation.json` sha256 `fa28644877bec6a40045d64006fa3d4a606776e3d7d36e858e4273569eddaae6`,
text `9a9dea0ffecd5ba6a760c007954165fa7c501547953251ca8095c2cd6fc2623a`, relabelled JSON
`8278d3f818919c83d438b41734915524b39ed466a5ea5dbb484b15f4b34601b6`). Cost from recorded tokens:
1 514 123 input + 1 139 543 output = **$0.721** (well under the ≈$1.8 estimate), taking **total lane
harness spend to $13.17 over 31 191 calls**, ≈ $14.4 including out-of-harness probes — inside the $25 cap.

Per replicate: population h0/48/96/120/143 = 10,10,10,3,0 (r1), 10,9,9,3,0 (r2), 10,10,10,3,0 (r3);
**births 0 in all three**, starvation 0/1/0, age deaths 10/9/10; calls 1166/1085/1166; coverage
100-141, 30-141, 100-141 (Gini 0.056/0.123/0.056); decision errors 20/12/24 of 1166/1085/1166.

**The depth comparison (pre-declared framing): `high` does NOT separate from `low` on any KPI.**

| pair | population | births | starvation | age deaths | max capital | verdict |
|---|---|---|---|---|---|---|
| **R@high vs R@low** | 0.487 | 0.817 | 0.054 | 0.225 | 1.291 | **NOT distinguishable** |

Every KPI separation is below SNR 2 (demographic and wealth alike), and no separation runs 24 consecutive
hours. **Both-cell clause**: single cell (f10-s0), so the rule's "in both cells" condition is met
vacuously — depth reproduction at f12-s0 is untested. So at this
fixture and horizon **the depth of thinking is not resolvable**: `low` and `high` produce the same
demographic trajectory (both die out by h143 with zero births), and `A1@low` is also not distinguishable
from `R@high` (population 0.817, births 0.000, starvation 0.817, age 0.817). Per the pre-declared reading,
the limiting factor is the **fixture's resolution** — population capped at 12, extinction by h156, zero
births under any full-cognition arm — not the depth of thinking. `max` would have been an expensive way to
learn the same thing, which is why it is deliberately unrun.

Everything else in the same curve report is unchanged from the v2 section above: `A` vs `P`/`A0` still fail
to separate on every **demographic** KPI (they separate on wealth state and process measures only), `U`
separates from everything, and `A1` remains the arm closest to `R` — i.e. the unvalidated-anchor problem
(Q1) is untouched by the depth result.

## Completion record (2026-10-18, branch `feature/35-vs2-curve`)

**Commits** (in order): `2c7fbcc` pre-registered criterion · `99e3a6c` 72 h + `-parallel 8` declaration ·
`fa79953` applied effort/bound provenance + `-votes` · `6c08536` flags, KPI export, replicate dirs, curve
statistics + tests · `8562195` smoke + live attempt/call timeouts · `fb27465` live-only 1 MiB response
bound + per-call applied bound/body length · `df54d93` stage-1 result + re-plan + mode-corrected labels +
`-relabel` · later commits: stage-2/3 results, EC-1 continuity, admissibility quantification and
`natural-claim/v2`, `vs2-value/2` + coverage reporting + regression test, v2 result, re-grading, docs,
spend accounting, max→high substitution.

**Artefact hashes (sha256)**

| artefact | sha256 |
|---|---|
| `.scratch/35-vs2-curve/spec.md` at its **pre-registration** commit `2c7fbcc` (the frozen criterion; this file changes with each later commit, so a self-referential hash would be unstable — the current value is in the final commit's message) | `89a0c3b4bf4eac26dbdff4ca9874ae6f67c35fbbcd7d0055fb0ffb2e405de9cd` |
| `.tmp/35-stage1-evaluation.json` | `c75c807f4e13c2183a856bd9eaadcc6358c6868a8b639b46eb22007f7fc0e4b2` |
| `.tmp/35-stage2-none-evaluation.json` | `6fb3c2ecc161a6c99ac7b9a579f8d017bf507203339ddf792b92ab6f16f2f14b` |
| `.tmp/35-stage2-low-evaluation.json` | `d579aa77b3246ea4ff9240d6161c8f15457e11997429d7943a1006406f5ae8e3` |
| `.tmp/35-stage2-high-evaluation.json` | `c39d4a61ee1099ca38fed5025a92294d2ac63a1e974b2b240eeb5e91dcc08c37` |
| `.tmp/35-stage3-evaluation.json` | `808de6230b0a7491171eaa037444e4e34229a83ecdec4368dafdc1a22076b8ed` |
| `.tmp/35-v2-evaluation.json` | `8efd819533d56b419d74639f6e6ba559fb678f013df7120bf6b8c3102d4bae3d` |
| `.tmp/35-v2-evaluation.txt` | `857318f46f5c20516daa9b3a89099ba81a48f7ebedc743c950105f8de6563a3d` |
| `.tmp/35-v2-curve.txt` (post-`n/a`-repair) | `b8d9f181d1601343381911fc78591d3a2f261f07d46462b82ae9004d14487a2a` |
| `.tmp/35-v2-curve.txt.pre-p14` (superseded) | `62d9e989e451f6be2bb21970a66e6502e5294f33aa32dae4d618fe9283b32feb` |
| `.tmp/35-curve-report.txt` (stages 1-3, post-`n/a`-repair) | `14665daaec7b0b06dd3a3e2fde38088a5a76928c631dc9be22c12af31ae846cf` |
| `.tmp/35-ec1-evaluation.json` (A0@168 h continuity) | `ed01e3229d93def1fceb34e8450c5fc4d081dabda5d52538569d152eb3234bb8` |
| `.tmp/35-claimv2-results.json` (v1 vs v2 probe) | `059a256143813cab38d4defc254366a7edb9b3fc9dfea447ebdf23375cc60411` |
| `cmd/vs2/curves.go` | `0404f81534daf569b0483e0e2085cbc9c91d595673ececd6a4c062ed01d5d8d1` |
| `cmd/vs2/curves_test.go` | `529c568ebba13e1ec0f09f47171cebb2afa92c57ae2d6ba24cb65742e5f83b11` |
| `.tmp/35-v2high-evaluation.json` (depth run, `high`@v2) | `fa28644877bec6a40045d64006fa3d4a606776e3d7d36e858e4273569eddaae6` |
| `.tmp/35-v2high-evaluation-relabelled.json` | `8278d3f818919c83d438b41734915524b39ed466a5ea5dbb484b15f4b34601b6` |
| `.tmp/35-depth-curve.txt` (low vs high and the v2 arms, post-`n/a`-repair) | `f5bdcbe6a799e5ca8306ac9fe9098f3ca4211f97d16e8753c6da6bcc2813a0fd` |
| `.tmp/35-depth-curve.txt.pre-p14` (superseded) | `068767ed390bab1e7c96b7897ea4555863f8935950162f3228935b56abf81e51` |

**Gates at the final tree** (`sh .tmp/35-gates2.sh` → `.tmp/35-gates2.out`): `go build ./...` PASS,
`go vet ./...` PASS, `gofmt -l` (tracked .go) clean, `git diff --check` clean, and
`go test -timeout 30m ./internal/cognition/... ./cmd/vs2 ./internal/experiment` all ok. The full-package
gate `go test -timeout 30m ./...` passed earlier at the same code (`internal/world` 1501.272 s, zero FAIL
lines, `.tmp/35-gates.out`). **Final lane spend: $13.17 over 31 191 recorded harness calls (≈ $14.4 with
out-of-harness probes), inside the $25 cap. `max`@v2 is deliberately unrun** — the depth rung at `high` did
not separate from `low`, so a 15-17 h single point would have bought nothing.

**Final statement of what this lane establishes and what it does not.**
Establishes: the harness now records what it applies (effort, output bound, response bound, body length)
per call; the ticket-34 harness bounds (30 s attempt, 64 KiB body) were silent live decision-loss
channels and are fixed for live runs; the dominant loss in the v1 matrix was **admissibility** (40-53 %)
and a versioned contract change removes it (`natural-claim/v2`: 0 inadmissible in 6 580 calls); the
coverage collapse was the value model and `vs2-value/2` fixes it (Gini 0.14-0.23 vs 0.49-0.59 in the same
run); and the aggregate noise floor at this fixture is ~1 actor / ≤5 % relative. Does not establish: that
any allocation policy is better. Periodic sampling reproduces the no-AI world on **every demographic KPI**
at 144 h (its wealth state and granary state still differ); no **demographic** indicator separates A from P
or from A0 (they separate on wealth state and process measures only); and the candidate metric families
rank the arms in **opposite orders**, because **the R anchor is unvalidated** (full cognition suppresses
reproduction and dies out, so fidelity-to-R rewards extinction). **Open question Q1 (a civilization-level
distance measure) is not resolved here and is the blocking question for any further VS2 claim.** The
ticket-34 verdict (central claim NOT accepted under EC-3 as written) stands unchanged, and the M2
milestone is not accepted.

## Post-review corrections (no new spend, no new data)

An independent review returned "OK with notes" with pre-registration **INTACT** (criterion and executable
rule committed before any live call; no evidence of post-hoc tuning; no headline direction reverses) and
four P1 factual overclaims plus several P2s, all repaired in this record, in `CHANGELOG.md`, in
`docs/VERTICAL-SLICES.md`, in the curve reports and in the curve-report code:

- **P1-1** "every KPI … RMSE 0.0000" → the accurate demographic form, with the wealth divergence stated
  (`max_capital` differs at 70-76 hours per replicate, first divergence h19/h13/h13; `capital_hist[2]`/`[3]`
  over the same hours). Verified independently here before editing.
- **P1-2** "indistinguishable from A0" and "separate only through process measures" now carry the
  demographic qualifier, and the passages name the wealth-state separations the tool actually reports
  (`capital_hist[0]` for `A0 vs A` and `A vs P`; `granary_hist[0]` for `A0 vs P` in the v2 run).
- **P1-3** the criterion's **both-cell clause** is now stated where the single-cell re-plan is recorded and
  referenced by each single-cell verdict: the clause is met vacuously, never substantively, and f12-s0
  reproduction is untested after stage 1.
- **P1-4** both-deterministic comparisons now report **`n/a (both deterministic)`** with the raw RMSE
  instead of `SNR = +Inf` (code + tests + report label + every table), and the three verdict lines that
  depended on the `+Inf` convention are named: `A0 vs P` (stage 3) **flipped** DISINGUISHABLE →
  NOT distinguishable; `A0 vs A1` and `A1 vs P` kept their verdicts but their driving KPI changed from
  `population` (`+Inf`) to `max_capital` (real ratio). No other verdict changed.
- **P2s** the stage-2 `none` vs `low` row (`max_capital` **4.83 / 117 h**, not 4.81 / 115 — that is the
  `high` vs `none` row); A's population row (`h0 = 10` in all replicates); the rejection-rate band
  (0.7-1.3 %, per-arm figures given); the stale `≈$19.5 of the $25 cap` projection (labelled as a
  projection and pointed at the spend section, which is authoritative); "the arm labelled buggy was the
  better one" → "**was the closest to R**" (dominance language stays with the frozen EC-3 instrument);
  the `-relabel` difference list now includes the `applied:` header line it had omitted (the path detects
  its absence and reports it); and the curve report's noise-floor columns are relabelled `mean@H`,
  `ci_lo@H`, `ci_hi@H`, `envelope@H`, `meanCV`, `worstCV` with a line stating that the columns are the band
  at the final hour — the earlier `band@H` label named something other than what the column held.

**What the review could not verify, and who did.** The review had no shell, so three mechanical claims were
checked by the supervisor and are recorded here with the commands:

1. `git diff --name-status 3faa480..HEAD` → **6 added, 20 modified**, and the set matches the expected
   one (the six additions are exactly `cmd/vs2/curves.go`, `cmd/vs2/curves_test.go`,
   `internal/cognition/claim_contract_test.go`, `internal/cognition/client_applied_bound_test.go`,
   `internal/cognition/priority_coverage_test.go` and this study record).
2. **The criterion was not edited after data existed.** Diffing the pre-declared section between the
   pre-registration commit and HEAD (`git show 2c7fbcc:.scratch/35-vs2-curve/spec.md` vs
   `git show HEAD:…`, slicing `## The pre-declared acceptance criterion` to the next `## `) yields
   **no differences — 63 lines byte-identical**. Only design parameters changed afterwards, each recorded
   as such (e.g. the horizon note "the criterion is unchanged").
3. **The `P vs A0` wealth divergence is real**: recomputing from `.tmp/35-v2-evaluation.json`,
   `max_capital` differs from A0 in **76/70/74 hours** across the three P replicates, first divergence at
   h19/h13/h13, with `capital_hist[2]` and `[3]` following it; every demographic series is identical.
