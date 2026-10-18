# Ticket 39 — M2c part 2: execute the pre-registered arm matrix, phase A (pre-live gates and launch)

- Base revision: `279f0ba` (master, tree clean; ticket 38 merged).
- Branch: `feature/39-m2c-matrix`.
- Author: worker agent (ticket 39), under the advance-phase plan.
- The criterion is **already pre-registered** in ticket 38 (`.scratch/38-m2c-preregistration/spec.md`
  §7, commit `6072af3`, tightened by §R7 before any live call). This ticket executes it; it does not
  re-declare, re-weight or re-interpret any part of it. Where this file restates a §7 value it is a
  citation, and §7 owns the value.
- Scope of this ticket, phase A: **pre-live gates, launch, liveness proof, checkpoint.** Scoring,
  the verdict and the curve corroboration belong to the polling/scoring worker (phase B, §6).

## 1. Goal

M2c question (`docs/VERTICAL-SLICES.md`, M2c row), under the M2a-passed contract and the M2b metric:

> Under `natural-claim/v4` at cell `f10-s0`, horizon `H = 144`, does any **budgeted allocation arm**
> (`A` adaptive on `vs2-value/2`, `A1` adaptive on `vs2-value/1`) separate from the **periodic** (`P`)
> and **uniform-random** (`U`) arms on the `vrd/2` order (persistence `e`, actor-hours `d_H`, starvation
> deaths `d_S`, births `d_B`), at equal declared call ceiling `N = 256` and with the no-AI control `A0`
> as the floor?

The rule that answers it is ticket 38 §7.4–§7.6: band + per-tier effect floor `|mu_X − mu_Y| >
max(B, ε_tier)`, tier 1 by unanimity, verdict ∈ {SEPARATION, SECONDARY-ONLY, NO SEPARATION,
NOT RUN}, with the falsifier declared in §7.6 and the inventory precondition of §7.4.

## 2. Declared run parameters (cited from ticket 38 §7.1; not re-declared here)

| item | value |
|---|---|
| arms | `A0` (no-AI control, 0 calls), `P` (periodic), `U` (uniform-random), `A` (adaptive, `vs2-value/2`), `A1` (adaptive, `vs2-value/1`) |
| cell | `f10-s0` (founders 10, seed 0) |
| horizon | `H = 144` (samples 0..144) |
| replicates | 3 per arm → 15 scored cells (3 × `A0` + 12 model-bearing) |
| decision contract | `natural-claim/v4` |
| applied effort | `low`; `-max-output 32768`, `-response-bound 1048576`, `-attempt-timeout 300`, `-call-deadline 900`, `-parallel 8` |
| budget axis | declared call ceiling `N = 256` per budgeted arm-run |
| `R` (full wake) | excluded, declared non-goal (§7.1) |

Spend authority for this ticket, from ticket 38 §8.1: **hard cap $2.00 and 4 000 physical attempts**,
expected $0.41–$0.50. The harness enforces calls, not dollars (`budgetFor` sets `MaxCostMicros: 1<<60`
and the report cost is `UNPRICED`), so the dollar cap is enforced by the operator poll rule of §8.1:
every 15 min compute realised spend from every `calls.jsonl` `usage` row at $0.10/M input and $0.50/M
output; stop the runner and report a partial matrix if realised spend exceeds **$1.60** or the realised
rate projects above **$2.00**.

Expected wall clock (ticket 38 §8.2): 3–4 h, deadline 6 h, `-budget-min 720` so the harness wall clock
never truncates the matrix before the operator deadline.

## 3. The exact commands (verbatim; ticket 38 §8.4 steps 0–5)

```sh
# 0. toolchain
. /home/z/vibe/agentworld/.tmp/go-env.sh

# 1. zero-spend gates at the landing revision
go build ./... && go vet ./... && git ls-files -z '*.go' | xargs -0 gofmt -l && git diff --check 279f0ba
go build -o .tmp/39-vs2-bin ./cmd/vs2

# 2. evaluator gate (ticket 38 §7.6 gate 1) — the committed scorer must still reproduce ticket 37
python3 .scratch/38-m2c-preregistration/vrd2.py --self-test \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/39-m2c-matrix/evaluator-gate.log

# 3. anchor gate (ticket 38 §7.6 gate 2, §8.4 step 3) — zero spend
.tmp/39-vs2-bin -arms A0 -cells f10-s0 -reps 3 -hours 144 -replay=false \
  -claim-contract natural-claim/v4 -root .tmp/39-a0-runs \
  -out .tmp/39-a0-evaluation.json -out-txt .tmp/39-a0-evaluation.txt

# 4. curve step (the command ticket 38 §8.4 step 7 runs; exercised pre-launch on a recorded artefact)
.tmp/39-vs2-bin -curve-in .tmp/36-v4-evaluation.json -curve-out .tmp/39-curve-gate.txt

# 5. contract gate (ticket 38 §7.6 gate 3) — the `inadmissible` count
python3 .tmp/35-admis.py .tmp/36-v4-runs

# 6. gateway probe (one call, the same wire/model/effort the harness sends)
curl -s -m 300 -X POST http://127.0.0.1:4000/v1/responses -H 'Content-Type: application/json' -d '{...}'

# 7. the live matrix (spend authorised for this run only), detached
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/gpt-6-luna
setsid nohup .tmp/39-vs2-bin \
  -arms A0,P,U,A,A1 -cells f10-s0 -reps 3 -hours 144 \
  -effort low -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 8 -budget-min 720 \
  -root .tmp/39-m2c-runs \
  -out .tmp/39-m2c-evaluation.json -out-txt .tmp/39-m2c-evaluation.txt \
  > .tmp/39-m2c-run.log 2>&1 &
echo "runner pid $!"

# 8. liveness (every poll; pid AND ledger growth, never a pid file, never log content)
ps -p <pid> -o pid=,etime=,stat= && wc -l .tmp/39-m2c-runs/*/calls.jsonl
```

**One runner, one run root, per-arm run directories.** §8.4 step 4 declares a **single** `cmd/vs2`
invocation for all five arms, and the scorer of §8.4 step 5b/6 takes **one** evaluation JSON
(`vrd2.py --live` accepts a single path; `cells_of` keys cells by `(arm, replicate)`), so the declared
scoring path only exists if the matrix is one invocation. Splitting the matrix into per-arm invocations
would produce five evaluation JSONs and no declared way to score them. The runner already writes one
directory per arm-replicate under the single root — `.tmp/39-m2c-runs/<arm>-f10-s0-r{1,2,3}/`, each with
its own `calls.jsonl` ledger — which is the per-arm granularity the liveness check needs.

## 4. Gates (each recorded verbatim with its result before any live call)

All of §4 was measured and committed **before** the launch of §9. Raw transcripts committed beside this
file: `gates.log` (build/vet/gofmt/diff-check/binary/curve), `evaluator-gate.log` (the scorer
self-test), `anchor-gate.json` (the `A0` re-run vector), `gateway-probe.txt` (the probe).

| # | gate | exact command (cwd = branch worktree) | result |
|---|---|---|---|
| 1 | build | `. .tmp/go-env.sh && go build ./...` | exit 0 — **PASS** (go1.24.13 linux/amd64, `GOTOOLCHAIN=local`) |
| 2 | vet | `go vet ./...` | exit 0 — **PASS** |
| 3 | gofmt on tracked `.go` | `git ls-files -z '*.go' \| xargs -0 gofmt -l` | no output, exit 0 — **PASS** |
| 4 | whitespace/conflict vs the base | `git diff --check 279f0ba` | exit 0 — **PASS** |
| 5 | runner binary | `go build -o .tmp/39-vs2-bin ./cmd/vs2` | exit 0 — **PASS** |
| 6 | evaluator gate (ticket 38 §7.6 gate 1) | `python3 .scratch/38-m2c-preregistration/vrd2.py --self-test --scores .scratch/37-fidelity-metric/scores.json --log .scratch/39-m2c-matrix/evaluator-gate.log` | exit 0; T1–T7 PASS, every band/floor fixture PASS, `SELF-TEST: PASS` — **PASS** |
| 7 | anchor gate (ticket 38 §7.6 gate 2, §8.4 step 3) | `.tmp/39-vs2-bin -arms A0 -cells f10-s0 -reps 3 -hours 144 -replay=false -claim-contract natural-claim/v4 -root .tmp/39-a0-runs -out .tmp/39-a0-evaluation.json -out-txt .tmp/39-a0-evaluation.txt` | exit 0 in 5.9 s wall; **3/3 identical replicates**, each `H = 1444`, `B = 2`, `S = 0`, alive@143 = 2, alive@144 = 1, `realized_calls = 0`, `halted = false`, identical `projection_hash 33b7692ad323…` — **PASS** (`anchor-gate.json`) |
| 8 | curve step (ticket 38 §8.4 step 7) | `.tmp/39-vs2-bin -curve-in .tmp/36-v4-evaluation.json -curve-out .tmp/39-curve-gate.txt` | exit 0; 58-line report written, carries the noise floor, the per-condition accounting and the decision-rule statement — **PASS** (the command runs and produces the §7.5 corroboration quantity at zero spend) |
| 9 | contract gate (ticket 38 §7.6 gate 3) | `python3 .tmp/35-admis.py .tmp/36-v4-runs` | `tally per arm (role-a records):` `R {'applied': 1323, 'rejected': 2, 'inadmissible': 0, 'malformed': 2, 'other': 0}` — the `inadmissible` count is read from `role_a.fallback_reason == "inadmissible"` over every `calls.jsonl` in the run root; **0** on the recorded `natural-claim/v4` run, and the same command is the gate phase B runs on `.tmp/39-m2c-runs` before reading a verdict — **PASS** |
| 10 | gateway reachability | the one-call probe of `gateway-probe.txt` | HTTP 200 in 4.195 s, body `status: completed`, model echoed `github_copilot/gpt-6-luna`, usage 27 in / 12 out (≈$0.000009), `output_text` a valid JSON object under `text.format=json_object` with `reasoning.effort=low`; zero-spend `/health/liveliness` 200 and `/v1/models` lists 97 ids including the declared model — **PASS** |

Two observations, recorded rather than dismissed:

- **`VS2-EC-1` FAILs in the anchor report, and it is pre-existing and structural, not a regression.**
  The evidence line is `A0 cells checked=1 missing=0 declared=12; mismatches=f10-s0:alive_by_hour,
  f10-s0:event_body_bytes, f10-s0:events, f10-s0:history_bytes`. Two discriminating checks: at `H = 168`
  (the frozen VS2 horizon) the same cell reports `mismatches=none` and still `FAIL` for `checked=1
  declared=12`, so the four mismatches are the declared `H = 144` horizon truncation that ticket 35
  already carries, and the FAIL itself is the one-cell inventory; and ticket 35's recorded
  `natural-claim/v2` evaluation (`v2`), which runs the same cell at the same horizon, reports the
  **identical** FAIL with the **identical four mismatches** (`mismatches=f10-s0:alive_by_hour,
  f10-s0:event_body_bytes,f10-s0:events,f10-s0:history_bytes`). `VS2-EC-1` is satisfied only by the full
  12-cell `A0` matrix, which this ticket does not run; it is not one of the ticket-38 gates and it does
  not gate the launch. Recorded here so phase B does not read it as new.
- **The gateway probe is real spend, $0.000009 of the $2.00 cap**, one call, and its `calls.jsonl`-free
  cost is counted in §9's spend line.

## 5. Run bookkeeping

- **UTC launch time: `2026-10-18T15:30:26Z`** (recorded in §9.1).
- **Runner pid: `2725555`** (recorded in §9.1).
- **Run root:** `.tmp/39-m2c-runs` (one directory per arm-replicate, each with `calls.jsonl`).
- **Log:** `.tmp/39-m2c-run.log` (the report is written only at exit, so an empty or growing log proves
  nothing about liveness — ticket 38 §8.3).
- **Liveness = runner pid exists AND each arm's `calls.jsonl` grows.** Never a pid file, never log
  content. The poll script that produced §9.4 is committed verbatim as `poll.py` beside this file
  (`sha256 2a38560f…`, byte-identical to the `.tmp/39-poll.py` the polls actually ran).

## 6. Results (phase A fills only the bookkeeping rows)

Inventory at the last poll (15:34:56Z): 11 of 15 declared cells have a run directory (3 × `A0` complete +
8 model-bearing running); 4 model-bearing cells are queued behind the `-parallel 8` semaphore. **The run
was still in flight when this ticket was committed** — phase A records the launch, not the outcome.

| arm | cell | reps | realized calls at last poll (15:34:56Z) | halted | `inadmissible` (phase B) |
|---|---|---|---|---|---|
| A0 | f10-s0 | 3/3 complete | 0 (0/0/0) | none | — |
| P | f10-s0 | 3/3 running | 188 (64/63/61) | none observed | — |
| U | f10-s0 | 2/3 running, r1 queued | 124 (62/62) | none observed | — |
| A | f10-s0 | 2/3 running, r3 queued | 120 (59/61) | none observed | — |
| A1 | f10-s0 | 1/3 running, r1/r2 queued | 56 | none observed | — |
| **total** | | | **488 calls, $0.0840** | | |

`vrd/2` keys, band/floor decisions, the verdict, the six-permutation sensitivity, the `h = H−1`
persistence variant and the curve corroboration are **phase B** outputs and are deliberately absent
here: this ticket launches the run, it does not score it.

## 7. What this ticket may not conclude

Everything in ticket 38 §7.7, unchanged: not the central claim (one cell, no spend-budget comparison, no
marginal-value-of-reasoning measurement), no cross-cell/cross-seed generality, no claim outside `vrd/2`'s
order, no depth/effort claim, no fidelity claim, no re-opening of tickets 34–37, and no "better"
language beyond "more viable under `vrd/2`". Phase A adds nothing to that list: **a launched run is not a
result.**

## 9. Launch record (phase A: launched, demonstrably running, checkpointed)

### 9.1 What was launched

- **Launched: yes.** One detached runner, started `2026-10-18T15:30:26Z` (process start `23:30:25` local =
  `15:30:25Z`; the date was read immediately before the `setsid`).
- **Runner pid: `2725555`** (`setsid nohup` child; `ps -p 2725555 -o sid=,pgid=` gives `2725555` for both, so
  it leads its own session and does not die with the launching shell). Binary argv[0] `.tmp/39-vs2-bin`.
- **Run root: `.tmp/39-m2c-runs`** — one directory per arm-replicate
  (`<arm>-f10-s0-r{1,2,3}/`), each with its own `calls.jsonl` ledger.
- **Log: `.tmp/39-m2c-run.log`** (stdout+stderr, `< /dev/null` on stdin). It is `0 B` during the run and
  the report is written only at exit; **it is not evidence of liveness** (§5).
- **Artefacts written at exit:** `.tmp/39-m2c-evaluation.json`, `.tmp/39-m2c-evaluation.txt`.

Exact command (cwd `/home/z/vibe/agentworld`, after `. .tmp/go-env.sh` and the three exports of §3 step 7):

```
setsid nohup .tmp/39-vs2-bin -arms A0,P,U,A,A1 -cells f10-s0 -reps 3 -hours 144 \
  -effort low -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 8 -budget-min 720 \
  -root .tmp/39-m2c-runs \
  -out .tmp/39-m2c-evaluation.json -out-txt .tmp/39-m2c-evaluation.txt \
  > .tmp/39-m2c-run.log 2>&1 < /dev/null &
```

### 9.2 Why `-parallel 8` (capacity reasoning, measured)

`-parallel 8` is the value ticket 38 §7.1/§8.4 declares, and the measurement says it is also the right
fit, so the declared value was not touched:

- **The work is gateway-latency bound, not CPU bound.** Observed load average during the run: `1.17`
  before launch, then `0.05`, `0.11`, `0.29` at the polls, on 8 cores. One call takes 2.1–7.8 s of wall
  time (`latency_micros` 2.1e6–1.2e7 in the ledgers) while costing a few ms of CPU, so 8 concurrent cells
  wait on the gateway rather than compete for cores.
- **Memory and disk are far from binding:** runner RSS 220 MB, `Mem available` 14 GB, one cell directory
  22 MB, 385 GB free on `/`.
- **Concurrency is per cell** (`main.go`'s semaphore is acquired per arm-replicate task), so 8 slots means
  8 cells in flight, each making its own serial calls. The 12 model-bearing cells therefore overlap in two
  waves: 8 running now, the remaining 4 queued behind the semaphore (see §9.4 — queued, not stalled).
  Within a cell the calls are serial by construction (one decision per hour), so a larger `-parallel`
  would raise the number of concurrent cells, not the depth of any cell's reasoning.

### 9.3 The 3 zero-call `A0` runs

`A0` runs inside the same invocation (3 replicates, 0 model calls by construction). All three finished
within the first seconds of the run and their directories `.tmp/39-m2c-runs/A0-f10-s0-r{1,2,3}/` exist
with the two recorded bundles each. They carry **no `calls.jsonl`** — a cell with zero calls writes no
ledger, so the absence of a ledger for `A0` is the correct state and not a stall. The same three cells
were independently reproduced by gate 7 (`anchor-gate.json`: `H = 1444`, `B = 2`, `S = 0`, identical
`projection_hash`), which is the zero-spend proof that the in-matrix `A0` rows will carry the same vector.

### 9.4 Liveness: pid AND ledger growth, four polls

Call counts are `kind == "call"` rows in each cell's `calls.jsonl` (ledger line counts also include
`allocation` and `role-a` rows, so the raw `wc -l` number is larger than the call count). Spend is
computed from each call row's `usage.prompt_tokens`/`completion_tokens` at $0.10/M in, $0.50/M out.

| poll (UTC) | runner pid alive | cells with a ledger | calls (total) | Δ calls since previous poll | in/out tokens | spend (cumulative) |
|---|---|---|---|---|---|---|
| 15:31:26 (t+60 s) | yes, `Ssl` 00:59 | 8 | 115 | — (first) | 0/0 at that instant, see note | — |
| 15:31:33 (t+67 s) | yes | 8 | 123 | +8 in 7 s | 77 419 / 23 462 | $0.0195 |
| 15:32:37 (t+2:11) | yes, `Ssl` 02:11 | 8 | 235 | **+112 in 64 s** | 147 302 / 51 085 | $0.0403 |
| 15:33:45 (t+3:19) | yes, `Ssl` 03:19 | 8 | 351 | **+116 in 68 s** | 220 346 / 78 373 | $0.0612 |
| 15:34:56 (t+4:30) | yes, `Ssl` 04:30 | 8 | 488 | **+137 in 71 s** | 306 514 / 106 693 | $0.0840 |

Note on the first row: at 15:31:26 the ledgers already existed with 258 lines; the per-call token tally in
that first read was taken by a tally that looked for the wrong usage keys, so its token columns are
recorded as unmeasured rather than as zero. The call counts from 15:31:33 on are from the corrected tally
(`.tmp/39-poll.py`, `kind == "call"`, `usage.prompt_tokens`/`usage.completion_tokens` — the keys the call
record actually carries).

**Per-cell call counts at the last poll (15:34:56Z):**

| cell | calls | in tok | out tok | state |
|---|---|---|---|---|
| A0-f10-s0-r1 | 0 | 0 | 0 | complete (no ledger, 0 calls) |
| A0-f10-s0-r2 | 0 | 0 | 0 | complete |
| A0-f10-s0-r3 | 0 | 0 | 0 | complete |
| P-f10-s0-r1 | 64 | 40 264 | 13 082 | running |
| P-f10-s0-r2 | 63 | 39 632 | 13 482 | running |
| P-f10-s0-r3 | 61 | 38 377 | 12 085 | running |
| U-f10-s0-r2 | 62 | 39 021 | 13 937 | running |
| U-f10-s0-r3 | 62 | 39 032 | 13 107 | running |
| U-f10-s0-r1 | — | — | — | **queued**, no directory yet (parallel-8 semaphore) |
| A-f10-s0-r1 | 59 | 36 508 | 13 366 | running |
| A-f10-s0-r2 | 61 | 38 387 | 13 581 | running |
| A-f10-s0-r3 | — | — | — | **queued**, no directory yet |
| A1-f10-s0-r3 | 56 | 35 293 | 14 053 | running |
| A1-f10-s0-r1 | — | — | — | **queued**, no directory yet |
| A1-f10-s0-r2 | — | — | — | **queued**, no directory yet |

**Every arm has demonstrated liveness** (P 3/3 cells growing, U 2/3, A 2/3, A1 1/3, A0 3/3 complete), and
**no running cell failed to grow in any interval** — the smallest per-cell growth over the three measured
intervals was `A1-f10-s0-r3` `+13`, `+15`, `+15`. The four cells with no ledger are **queued behind the
`-parallel 8` semaphore, not stalled**: the runner reports exactly 8 cells in flight, the semaphore is
per-cell, and cells start as slots free (`A1-f10-s0-r3` started while `A1-f10-s0-r1` was still queued
because goroutines acquire the semaphore in scheduler order, not declaration order). This is stated
plainly rather than assumed: **at this poll 8 of the 12 model-bearing runs are running and 4 have not
started.**

### 9.5 Observed rate, projection, spend

- **Observed call rate: 1.7–1.9 calls/s aggregate ≈ 105–116 calls/min**, i.e. ~13–15 calls/min per lane,
  matching the recorded ticket-36 lane rate (15.8–18.2 calls/min) with 8 lanes in parallel.
- **Projected completion:** `(3 072 − 488) / ~1.8 calls/s ≈ 24 min` of aggregate work, plus the second
  wave's reduced parallelism → **expected exit 16:00Z ± 10 min**, well inside the ticket-38 §8.2 deadline
  of 6 h. (Ticket 38 §8.2's 169–194 min estimate treated the per-lane rate as the aggregate rate; the
  measured aggregate rate is ~7× that, so the run is much faster than declared. Recorded as an
  observation, not as a plan change.)
- **Spend so far: $0.0840** at the last poll ($1.72e-4 per call), of which the gateway probe was
  $0.000009. **Projected total at this rate: ≈ $0.53 for 3 072 calls** — inside ticket 38's declared
  expected band ($0.41–$0.50) to within a cent and far below the $1.60 operator stop and the $2.00 hard cap.
  Physical attempts so far 488 of the 4 000 operator cap and of the 3 072 harness cap.
- **Anomalies, recorded:**
  - One `http-error` at `A-f10-s0-r1` seq 63 with `http_status: 400`, body
    `litellm.BadRequestError: litellm.ContentPolicyViolationError: Content…` — a single-attempt failure
    (`attempt: 1`, no retry burst), after which the cell continued (seq 66 onward, all `status: ok`). It is
    one lost decision in one cell, not a transport storm: 487 of 488 calls are `status: ok`. Phase B must
    read that cell's decision-error rate and the contract gate before scoring; the cell is not `halted`.
  - The log is empty (`0 B`) and no evaluation JSON exists yet, both correct before exit.

### 9.6 Next action for the polling/scoring worker (phase B)

1. **Poll (never `pkill -f`; to stop, kill pid `2725555` only):**

```sh
ps -p 2725555 -o pid=,etime=,stat= ; python3 .tmp/39-poll.py     # per-cell kind=="call" counts + spend
ls -d .tmp/39-m2c-runs/*/ ; wc -l .tmp/39-m2c-runs/*/calls.jsonl  # raw ledger lines (>= call count)
```

2. **Stop condition (ticket 38 §8.1):** recompute spend from every `calls.jsonl` `usage` row every 15 min;
   if realised spend exceeds **$1.60** or the realised rate projects above **$2.00**, kill pid `2725555`
   and report a partial matrix as incomplete. At $0.084 at t+4:30 the projection is ≈$0.53, i.e. the
   trigger is ~19× away and no intervention is expected.
3. **When the pid is gone, in this order:**

```sh
# a. the contract gate on the live ledgers (expect inadmissible = 0 per arm)
python3 .tmp/35-admis.py .tmp/39-m2c-runs
# b. the inventory assertion; exit 3 + "NOT RUN" means a partial matrix, report it as incomplete
python3 .scratch/38-m2c-preregistration/vrd2.py --check-inventory .tmp/39-m2c-evaluation.json
# c. scoring (the declared verdict machinery)
python3 .scratch/38-m2c-preregistration/vrd2.py --live .tmp/39-m2c-evaluation.json \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/39-m2c-matrix/m2c-scores.log
# d. curve corroboration (zero spend, ticket 38 §7.5 / §8.4 step 7)
.tmp/39-vs2-bin -curve-in .tmp/39-m2c-evaluation.json -curve-out .tmp/39-m2c-curve.txt
```

   Read `halted` / `under_budget` per cell first; a halted or skipped cell voids the run as an incomplete
   matrix rather than a smaller experiment (§7.4).
4. **Scope reminder:** phase B may report at most the `vrd/2` order at this one cell (SEPARATION /
   SECONDARY-ONLY / NO SEPARATION / NOT RUN), with the band, the per-tier floors, the six-permutation
   sensitivity and the `h = H−1` variant. Not the central claim (§7.7).


- Any gate failure, or an unreachable gateway, means **do not launch**: the failure and its evidence are
  recorded and returned as a blocker. (All gates of §4 passed and the gateway answered, so the launch was
  made.)
- To stop a run, kill the recorded runner pid (`2725555`) only. Never `pkill -f`.
- A partial matrix (a skipped, missing or `halted` cell) is reported as incomplete and never scored as a
  smaller experiment (§7.4 inventory precondition; §8.4 step 5b returns `NOT RUN`, exit 3).
