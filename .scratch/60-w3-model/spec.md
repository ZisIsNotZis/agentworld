# Ticket 60 — W3 Stage M (pre-registration ticket 58): model (`gpt-6-sol` against the `luna` reference)

- Record kind: one live stage of the W3 pre-registration (`.scratch/58-w3-preregistration/spec.md`,
  committed `8f0f47b`, Stage M amended at `36083db`), its recorded cells, the declared §3.1
  reference distance, and the stage verdict. No world, policy, metric, contract, reference-model or
  code change.
- Author: worker, 2026-10-28.
- Source: the pre-registration's Stage M (§4) and comparison rule (§3/§3.1), declared before the
  first W3 live cell. This record applies that rule; it does not change it.
- Inputs: the `R@max` reference rows of Stage E (`.scratch/58-w3-effort/spec.md` §1 — three
  replicates, **not re-run here**), `curve/1` (`cmd/vs2/curves.go`, ticket 35 — frozen, not edited),
  `W1` PASS (ticket 55), `W2` (ticket 57), Stage T (`.scratch/59-w3-timing/spec.md`).

## 0. Verdict

**`github_copilot/gpt-6-sol` matches `luna` at this design.** No declared scalar KPI separates
`sol@max` from `luna@max` in cell `f10-s0`: no KPI holds `SNR ≥ 2` together with a `≥ 24 h`
separating run. This is "**no detectable effect at this design**", never equivalence.

**`github_copilot/grok-4.7` could not be run at the declared condition, and is recorded as an
admissibility failure, not as a null.** Its arm needs `reasoning.effort = max` to hold the
model-only comparison, and the upstream refuses that value for this model with HTTP 400 on every
call (20/20 in the pre-flight smoke, 0 tokens, 0 applied decisions); the same body without the
`reasoning` field returns 200 (§1.2, `grok-admissibility.txt`). No model was substituted and no
effort was re-mapped.

Three facts must be read beside the verdict:

1. **This is the first W3 stage whose `SNR` gate is live.** Unlike Stage T — where every arm's three
   replicates produced identical scalar series and `noise` was 0 — here the replicates genuinely
   differ (`noise` 1.118 on population, 1.440 on births), so `SNR` is defined and takes values
   0.65–1.02. The null is not an artefact of a degenerate denominator; it is a small
   difference against a real noise floor.
2. **The difference is present but not sustained.** `sol−luna` at H is −3.33 alive and −5.67 births,
   and the two curves differ on some scalar at 33 of 144 hours (from h109). Both observed gaps sit
   **above** the reported detection limit (population power 2.24, births 2.88 — §5), yet the longest
   continuous separating run is **12 h** on `births_cum` (h133–h144) and 7 h on `population`. The
   rule's sustained clause is what fails, exactly as in Stage T.
3. **The models decide differently far more often than the curve can see.** `sol` returned the
   frozen cascade's own action in **94.69 %** of calls (4189/4424) against `luna`'s **96.59 %**
   (4418/4574); every `sol` deviation is `eat-stored` (235 calls), where `luna` also takes `wait` at
   5 calls. So the decision seam is measurably model-sensitive (5.31 % vs 3.41 % deviation, 1.56×)
   while the declared five scalar KPI curves are not separable at 3 replicates. Reported, not
   smoothed.

Stage-M cost: **4424 calls, $1.3040 derived** (recorded charge $0.00 — the gateway is unpriced),
wall **35.2 min** for the recorded cell set, inside the stage's 6 h declared wall clock and inside
the ~$3.8 derived estimate (the unspent part is `grok-4.7`'s arm, which never ran).

## 1. What ran

### 1.1 The recorded cells

One cell (`f10-s0`, 10 founders, seed 0), arm `R` (full wake, no priority), 3 replicates, `H = 144`,
model `github_copilot/gpt-6-sol`, wire `responses`, effort `max`, claim contract
`natural-claim/v4`, output bound 32768, response bound 1 MiB, `-call-concurrency 10`,
`-parallel 3`, live gateway, spend authorised for this stage only.

```
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND LITELLM_BASE_URL=http://127.0.0.1:4000 \
LITELLM_MODEL=github_copilot/gpt-6-sol \
.tmp/60-vs2 -model github_copilot/gpt-6-sol -wire responses -arms R -cells f10-s0 -reps 3 \
  -hours 144 -effort max -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 3 -budget-min 600 \
  -root .scratch/60-w3-model/gpt-6-sol/runs \
  -out .scratch/60-w3-model/gpt-6-sol/evaluation.json \
  -out-txt .scratch/60-w3-model/gpt-6-sol/evaluation.txt
```

The exact invocation is kept as `run-gpt-6-sol.sh`; `starts.json` records the launch epoch
(03:07:18 UTC, binary built from `07c9356` — the tree was not modified during the run).

- **Identity is recorded as requested.** `evaluation.json`: `model_id: github_copilot/gpt-6-sol`,
  `wire_mode: responses`, `declared_effort: max`, `claim_contract: natural-claim/v4`,
  `call_concurrency: 10`, `horizon_hours: 144`, `live: true`, `status: live/real-model`,
  `model_identity_digest: 827d4cf2…`. **Every** one of the 4424 Role A call records carries
  `model_id: github_copilot/gpt-6-sol` and `applied_reasoning_effort: max` (checked record by
  record from the ledgers — one distinct value of each).
- **Matrix completeness: 3/3 declared cells of this arm, 3 replicates, 0 skipped**
  (`evaluation.txt`: `declared=3 ran=3 skipped=0`). Each cell carries the complete 145-hour
  (h0..h144) series.
- **No retries, no errors, no halts.** 4424 attempts, all `attempt 1`, all HTTP 200, all
  `status: ok`; `applied_model_decisions` equals `realized_calls` in every cell (1534 / 1431 /
  1459); `decision_errors: 0`, `halted: false`. There was no `429`, no timeout, no
  malformed/inadmissible answer and no content-policy refusal — the first W3 stage with a perfectly
  clean seam.
- **Replay:** the in-process replay lane re-ran all three cells from their ledgers through a
  dead-network transport; every cell reports `identical` (`evaluation.txt`, `REPLAY SUBSET`).
- **`-resume` was not needed:** no cell was interrupted; the launch ran to completion in 35.2 min.

### 1.2 The pre-flight smoke and the `grok-4.7` refusal

Before committing wall clock to a model, each non-reference model was smoke-tested for
admissibility on the declared request (2 simulated hours, 1 replicate, same flags; `smoke.sh`,
runs under `<model>/smoke/`; **not cells — no KPI from them enters any table**).

| model | smoke result | evidence |
|---|---|---|
| `github_copilot/gpt-6-sol` | 20/20 HTTP 200, 20 applied decisions, 0 errors | `gpt-6-sol/smoke/` |
| `github_copilot/grok-4.7` | **20/20 HTTP 400**, 0 applied decisions, 0 prompt/0 completion tokens | `grok-4.7/smoke/evaluation.txt` |

The recorded upstream body of every refused call is
`{"error":{"message":"litellm.BadRequestError: Github_copilotException - Bad Request …
LiteLLM: model group 'github_copilot/grok-4.7' failed with the error above. No fallback was
attempted.","type":"invalid_request_error","code":"400"}}`.

`grok-admissibility.py` isolates the cause on the same wire and model (a wire probe, no cell):
removing the `reasoning` block from the harness's exact request makes it 200; keeping it at
`effort: max` (or `none`) is 400. The effort matrix, one short call per cell:

| model | none | minimal | low | medium | high | xhigh | **max** |
|---|---|---|---|---|---|---|---|
| `github_copilot/grok-4.7` | 400 | 200 | 200 | 200 | 200 | 200 | **400** |
| `github_copilot/gpt-6-sol` | 200 | 400 | 200 | 200 | 200 | 200 | **200** |

So `grok-4.7` is a `responses`-wire reasoning model, but the declared matched condition
(`effort = max`) is not admissible for it at this gateway. Running it at `xhigh` would confound the
model axis with the thinking axis — the exact reason the pre-registration's `36083db` amendment
replaced the chat-wire model — so no `grok-4.7` cell was run, and `run-grok-4.7.sh` is kept as the
declared command that the smoke refused. **No substitute model, no invented wire mapping, no
changed effort.**

Why not run the refused cell anyway, to have 3 more rows? A `grok-4.7` cell at `effort=max` would
record ~4,400 level-1 fallbacks, zero model output and three identical no-AI trajectories under a
model's name — 35 minutes of wall clock and a row set that invites misreading. The refusal is
established at the request level by 20/20 smoke calls and the probe matrix; that is the recorded
failure. It is a **capability gap of the gateway for this model**, not a property of the world.

## 2. Accounting

Recorded charge is **$0.00**: the gateway exposes no price entry for `github_copilot/gpt-6-sol`
(`price_version = unpriced`, `cost_micros = 0`). The derived price is arithmetic at the ticket-35
published rates ($0.10/M input, $0.50/M output), the same rates and method as Stages E and T. Wall
is launch to `evaluation.json`; the per-cell column is the sum of the cells' recorded `duration_ms`.

| model | calls (3 reps) | prompt tokens | completion tokens | derived $ | cell seconds | wall |
|---|---:|---:|---:|---:|---:|---:|
| `sol` | 4,424 (1534/1431/1459) | 2,777,574 | 2,052,464 | 1.3040 | 5,137 (1,600/1,726/1,810) | 35.2 min |
| **fresh total (Stage M)** | **4,424** | **2,777,574** | **2,052,464** | **1.3040** | wall 2,112.7 s | |
| `luna@max` reference (Stage-E rows, not re-run) | 4,574 (1454/1541/1579) | 2,876,424 | 3,185,073 | 1.8802 | (previously recorded) | |
| `grok-4.7` | 0 (refused at the request level) | 0 | 0 | 0.0000 | – | – |

Cumulative derived spend against the operator's ~$14: W2 $0.63 + Stage E fresh $4.19 + E aborted
$1.08 + Stage T $0.88 + Stage M $1.30 = **$8.08**, i.e. ~$5.9 derived remains for Stage S. The stage came in **$2.5 under**
its ~$3.8 estimate because the `grok-4.7` arm was never admissible.

Latency of the recorded calls (`calls.tsv`): p50 **6,186 ms**, p95 **12,023 ms**, max **43,453 ms**
per attempt, with 10 calls in flight inside a simulated hour (call concurrency 10), one attempt per
decision.

## 3. The rule applied (declared in §3; no new rule)

`analyze.py` implements the pre-registered `curve/1` statistics literally over the recorded
evaluation JSONs (read-only, no cell, no model call). Its `paired()`/`power()`/`wasserstein1()` code
is the Stage-E/T implementation unchanged, so the stages' numbers are comparable.

- `d(t) = mean_sol(t) − mean_luna(t)` over each condition's 3 replicates; `RMSE(d)` over hours.
- `noise` = RMSE over hours of the pooled within-condition SD at each hour;
  `SNR = RMSE(d)/noise`, `n/a` when both conditions are deterministic.
- a separation at hour `t` iff `|d(t)|` exceeds both conditions' replicate envelope half-width; a
  KPI **separates** iff `SNR ≥ 2` **and** some separating run is `≥ 24` consecutive hours.
- the verdict is per scalar KPI, `{population, births_cum, deaths_starve_cum, deaths_age_cum,
  max_capital}`; `wages_cum` is carried by the instrument but is not a §3 scalar and never enters a
  verdict; the histogram families are carried by Wasserstein-1, not by the scalar rule.
- **power** (smallest separable difference): a constant offset δ gives `RMSE(d) = δ`, so the SNR
  gate needs `δ ≥ 2·noise`; the sustained gate needs `δ` to exceed the band at every hour of some
  24-hour window. The reported power is the larger floor.
- **direction** is description only; nothing is summed or ranked, and no model is called better.
- **histogram distance** is computed independently from the per-hour bucket vectors (each
  condition's hourly vector = mean count over its 3 replicates, normalised to a distribution over
  bucket indices; W1 in bucket units per hour, averaged over hours).

**Cross-check against the frozen instrument.** `.tmp/60-vs2 -curve-in` over the three evaluation
files (`curve-report.txt`) reproduces every scalar column exactly (e.g. population `rmse_d` 1.0065,
`noise` 1.1178, `snr` 0.9004, `firstsep` 134, `run_len` 7) and its label carries the model id
(`R@max@github_copilot/gpt-6-luna` vs `R@max@github_copilot/gpt-6-sol`); its post-`ed29dad`
histogram rows (capital 0.0183, granary 0.0662) agree with `analyze.py`'s independent W1.

## 4. KPI tables

Reference: `luna@max`, cell `f10-s0`, 3 replicates (Stage E: `t57max.r1`, `w3max.r1`,
`w3max.r2`). Its band at H: population mean 15.67 (envelope 12..18), births 20.00 (16..24),
starvation 4.33 (2..7), age deaths 10.00 (10..10), max_capital 1.00 (1..1)
(`.scratch/58-w3-effort/spec.md` §4.1 — cited, not restated as new data).

### 4.1 `sol@max` − `luna@max` (§3.1, same cell and seeds, 3 replicates each)

`first` = first separating hour (−1 none); `run` = longest separating run, `@` its start; `power` =
smallest separable constant difference; `dir` = direction of a *detected* separation (blank = none);
`d at H` is the mean paired difference at hour 144 (description only).

| KPI | RMSE(d) | noise | SNR | first | run | power | max abs d | d at H | dir | W1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| population | 1.006 | 1.118 | 0.900 | 134 | 7 @137 | 2.236 | 4.000 | −3.333 | – | 0.340 |
| births_cum | 1.471 | 1.440 | 1.021 | 133 | **12 @133** | 2.880 | 5.667 | −5.667 | – | 0.522 |
| deaths_starve_cum | 0.488 | 0.649 | 0.752 | −1 | 0 | 1.299 | 2.333 | −2.333 | – | 0.177 |
| deaths_age_cum | 0.000 | 0.000 | n/a | −1 | 0 | 0.000 | 0.000 | 0.000 | – | 0.000 |
| max_capital | 0.138 | 0.212 | 0.654 | −1 | 0 | 0.423 | 0.667 | 0.000 | – | 0.002 |
| `capital_hist` | – | – | – | – | – | – | – | – | – | 0.018 |
| `granary_hist` | – | – | – | – | – | – | – | – | – | 0.066 |

**No KPI separates.** The nearest miss is `births_cum`: a 12-hour separating run (h133–h144, i.e. to
the very end of the horizon), 12 hours short of the 24-hour gate, at `SNR` 1.02. `deaths_age_cum`
is 10 in all six cell rows of both conditions (deterministic, no information); `wages_cum` is 0
everywhere (instrument: `n/a (both deterministic)`).

### 4.2 Endpoints (mean over each condition's 3 replicates, hour 144)

| condition | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| `luna@max` (reference) | 15.67 | 20.00 | 4.33 | 10.00 | 1.00 |
| `sol@max` | 12.33 | 14.33 | 2.00 | 10.00 | 1.00 |

Per replicate, `sol` ends at 13/14/10 alive with 17/15/11 births and 4/1/1 starvation deaths; `luna`
at 12/18/17 alive with 16/20/24 births and 4/2/7 starvation deaths (`sol`'s replicate 3 sits at the
bottom of `luna`'s envelope, its replicate 2 in the middle). Direction, description only: `sol`
produces fewer births and fewer starvation deaths; the population gap is the net of the two.

### 4.3 Curve agreement and decision agreement (descriptive; no threshold, no verdict)

- **The scalar curves differ at all at 33 of 144 hours** (first h109, last h144) — the two runs are
  not the same world, unlike Stage T's budgeted arms against `A0` (0 hours).
- **Decision equality with the frozen cascade** (`calls.tsv`, `agreement.json`):

| condition | calls | action == frozen action | fraction | per replicate | deviations |
|---|---:|---:|---:|---|---|
| `sol@max` | 4,424 | 4,189 | **94.69 %** | 93.74 / 96.02 / 94.38 % | 235, all `eat-stored` |
| `luna@max` (3 reps) | 4,574 | 4,418 | **96.59 %** | (Stage E §2 + ticket 57) | 87 `eat-stored`, 5 `wait` |

Both models choose `gather` — the frozen cascade's own action — almost always; the difference lives
in the `eat-stored` decision, where `sol` departs 1.56× as often (5.31 % vs 3.41 %). This is the
decision-level reading the curve alone cannot give.

## 5. Stage power and the verdict

| KPI | smallest separable (power) | observed max abs d | reading |
|---|---:|---:|---|
| population | 2.24 | 4.00 | observed gap **exceeds** the limit; the 7-hour run fails the sustained clause |
| births_cum | 2.88 | 5.67 | observed gap **exceeds** the limit; the 12-hour run fails it, 12 h short |
| deaths_starve_cum | 1.30 | 2.33 | exceeds the limit; no 24-hour run |
| deaths_age_cum | 0.00 | 0.00 | deterministic 10 in all six rows: no information |
| max_capital | 0.42 | 0.67 | exceeds the limit; no separating run |

The design's power here is **live**, because both conditions carry replicate noise: `SNR` is a
ratio of two non-zero quantities (0.65–1.02), unlike Stage T where it was undefined. The reason the
observation does not separate is therefore **not** insufficient amplitude (the observed gaps are
1.6–2.0× the detection limit) and **not** a zero denominator; it is the **duration** condition: the
difference appears at h109–h134 and never holds for 24 consecutive hours in a 145-hour horizon of
which only the last 36 differ at all.

**Verdict, per §3.1 (applied literally):**

| model | relation to `luna@max` | because |
|---|---|---|
| `sol@max` | **matches** | no declared KPI separates (`SNR < 2` and run < 24 h on every KPI) |
| `grok-4.7@max` | **not measured — inadmissible** | the gateway refuses `reasoning.effort = max` for this model (HTTP 400, §1.2) |

No model is "best": the reference is the benchmark, and a model that shows no detectable separation
is reported as "no detectable effect at this design", never as equivalent and never as better.
`unresolvable here` does not apply — the detection limit (1.30–2.88) is below the observed
separations, which fail the sustained clause instead.

## 6. Limits

- **One cell only.** `f10-s0`. A cell claim not reproduced at `f10-s1`/`f11-s2` is exactly that;
  Stage S is where reproduction is judged. Stage M makes no reproduction claim.
- **The `grok-4.7` arm is missing, deliberately.** The pre-registration declared three models; two
  ran. The third is refused by the gateway at the matched effort, with the exact failure recorded
  (§1.2) rather than papered over by an effort substitution that would have confounded the model
  axis with the thinking axis. The Stage-M matrix is therefore **2 of 3 models, 6 of 9 cells**; the
  stage verdict is complete for what ran and silent — not negative — for `grok-4.7`.
- **This is a capability gap at the gateway, not a world fact.** The probe is a wire-level test of
  `reasoning.effort`; it says nothing about `grok-4.7`'s behaviour inside this world, because no
  admissible matched-condition request exists to measure it with.
- **The null has a mechanism, and it is weak.** Both models returned the frozen cascade's own action
  in 94.7–96.6 % of calls, so the decision seam moves little in absolute terms and the world it
  produces diverges only late — 33 differing hours out of 144, all late (from h109). This is model-conditional
  (`natural-claim/v4`, effort `max`, this world, this cell) and says nothing about a model that
  disagreed more often.
- **The reference is not re-drawn.** `luna@max` is the Stage-E three rows (ticket-57 replicate 1 +
  two fresh), as §2 declares; the reference is a comparative proxy, never ground truth
  (`docs/VALIDATION.md`).
- **The 3-replicate noise floor is real but small.** `noise` 0.65–1.44 on the declared scalar KPIs;
  the SNRs (0.65–1.02) sit below the gate with the observed differences above the power. More
  replicates would tighten the band, not lengthen the separating run; another cell or a longer
  horizon is what could.
- **Synthetic, model-conditional.** Nothing here is a claim about real civilizations, and no
  condition is called better.
- **Smoke runs and probes are not cells.** `*/smoke/` (2-hour runs) and `grok-admissibility.py`
  (wire probes) exist only to establish admissibility; no KPI, ledger or verdict here derives from
  them.

## 7. Artefacts

| file | what it is |
|---|---|
| `spec.md` | this record |
| `gpt-6-sol/evaluation.json`, `gpt-6-sol/evaluation.txt` | the run's own report: 3 cells, per-cell calls/identity, bands, replay, headroom |
| `gpt-6-sol/calls.tsv` | one row per recorded Role A call (4,424): cell, seq, hour, actor, action, frozen action, source, status, http, attempt, latency, tokens, request hash |
| `gpt-6-sol/kpi-series.tsv` | the per-hour KPI series (five scalars + two histogram families) for all 3 cells |
| `gpt-6-sol/agreement.json` | per condition: calls, `action == frozen_action` counts, source/status sets, wake hours, latency p50/p95/max |
| `gpt-6-sol/smoke/`, `grok-4.7/smoke/` | the pre-flight admissibility runs (2 h, 1 replicate) — evidence, not cells |
| `grok-admissibility.py`, `grok-admissibility.txt` | the wire probe and its captured output: which `reasoning.effort` values each model accepts, and that dropping the `reasoning` block makes the harness's exact request 200 |
| `analysis.md`, `analysis.json` | the §3.1 distance, power, direction, endpoints and verdict, produced by `analyze.py` |
| `curve-report.txt` | the frozen `curve/1` report over the three evaluation files (`-curve-in`), the scalar cross-check |
| `analyze.py` | the read-only §3.1 analysis (pairing, SNR, separation, power, W1) |
| `summarize.py` | regenerates `calls.tsv`, `kpi-series.tsv`, `agreement.json` from a cell's ledger and evaluation |
| `accounting.py`, `wall.json`, `starts.json` | calls, tokens, derived cost, wall clock; the launch epoch |
| `run-gpt-6-sol.sh`, `run-grok-4.7.sh`, `smoke.sh` | the exact invocations (the `grok-4.7` script is the declared command the smoke refused) |

**Not committed, deliberately:** the raw hash-chained ledgers and each cell's final checkpoint
bundle under `gpt-6-sol/runs/` (~50 MB per cell) and the replay lane's copies (~145 MB). They were
removed after extraction; the world state is bound by the `tip_hash`/`projection_hash` in
`evaluation.json`, the replay verdict is recorded there, and `calls.tsv` is the human-auditable
substitute. Stages E and T took the same decision for the same reason.

## 8. Reproduction

Toolchain: `cd /home/z/vibe/agentworld && . .tmp/go-env.sh` (go1.24.13), then
`go build -o .tmp/60-vs2 ./cmd/vs2` from `07c9356`.

```
# the live cell (spend-gated; $1.30 derived, 35 min). One invocation, 3 cells:
sh .scratch/60-w3-model/run-gpt-6-sol.sh

# admissibility evidence for the third declared model (no cell; 18 short calls)
python3 .scratch/60-w3-model/grok-admissibility.py
sh .scratch/60-w3-model/smoke.sh github_copilot/grok-4.7 .scratch/60-w3-model/grok-4.7

# read-only analysis (no cell, no spend). `summarize.py` regenerates the tables
# from the cell ledgers, which §7 removes from the tree after extraction: run it
# from a re-run, or use the committed `calls.tsv` / `kpi-series.tsv` directly.
python3 .scratch/60-w3-model/summarize.py --dir .scratch/60-w3-model/gpt-6-sol
python3 .scratch/60-w3-model/accounting.py \
  --eval gpt-6-sol=.scratch/60-w3-model/gpt-6-sol/evaluation.json \
  --reference .scratch/58-w3-effort/max/evaluation.json \
  --reference-extra .scratch/57-w2-concurrent-reference/evaluation.json \
  --starts .scratch/60-w3-model/starts.json --out .scratch/60-w3-model/wall.json
python3 .scratch/60-w3-model/analyze.py \
  --model-eval sol=.scratch/60-w3-model/gpt-6-sol/evaluation.json \
  --reference-eval .scratch/58-w3-effort/max/evaluation.json \
  --reference-extra .scratch/57-w2-concurrent-reference/evaluation.json \
  --accounting .scratch/60-w3-model/wall.json \
  --agreement sol=.scratch/60-w3-model/gpt-6-sol/agreement.json \
  --out .scratch/60-w3-model/analysis.md --json .scratch/60-w3-model/analysis.json

# the frozen instrument over the same files (scalar cross-check; its histogram rows agree too)
.tmp/60-vs2 -curve-in .scratch/60-w3-model/gpt-6-sol/evaluation.json,\
.scratch/58-w3-effort/max/evaluation.json,.scratch/57-w2-concurrent-reference/evaluation.json \
  -curve-out .scratch/60-w3-model/curve-report.txt

# no code changed by this stage; the declared gate still holds
go build ./...
```
