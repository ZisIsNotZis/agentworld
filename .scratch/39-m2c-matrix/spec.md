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
> deaths `d_S`, births `d_B`), at equal declared call ceiling `N = 256` and equal declared decision budget
> `N − R = 248` (`R = 8`, ticket 40 §3), and with the no-AI control `A0`
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
| budget axis | declared call ceiling `N = 256` physical attempts per budgeted arm-run, of which the schedule selects at most `N − R = 248` decisions (`R = 8` transport reserve, ticket 40 §3) |
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

## 6. Results — final state, 2026-10-18T16:14:25Z (phase B filled the completion columns)

The run finished on its own at **16:14:25Z** (43 min 59 s after launch), 3 020 physical attempts, **$0.6007**.
§10 owns the completion gate and the verdict; this table is the inventory it is computed from. Per-cell call
counts are `kind == "call"` ledger rows, i.e. charged attempts, not decisions.

| arm | cell | reps | charged calls (r1/r2/r3) | `halted` | `inadmissible` |
|---|---|---|---|---|---|
| A0 | f10-s0 | 3/3 | 0 (0/0/0) | none | — (0 calls) |
| P | f10-s0 | 3/3 | 716 (244/228/244) — under the ceiling by construction (periodic cadence) | none | 0 |
| U | f10-s0 | 3/3 | 768 (256/256/256) | none | 0 |
| A | f10-s0 | 3/3 | 768 (256/256/256) | none | 0 |
| A1 | f10-s0 | 3/3 | 768 (256/256/256) | **r1 halted at the call ceiling, hour 143** | 0 |
| **total** | | | **3 020 calls, $0.6007** | | |

The verdict is **NOT RUN** (§10.5): `A1-f10-s0-r1` halted at its call ceiling one sample short of the
horizon, so §7.4's inventory precondition is not met. `vrd/2` keys, band/floor decisions, the six-permutation
sensitivity and the `h = H−1` variant are therefore **not** reported as a comparison — `vrd2.py --live`
refuses to print one.

## 7. What this ticket may not conclude

Everything in ticket 38 §7.7, unchanged: not the central claim (one cell, no spend-budget comparison, no
marginal-value-of-reasoning measurement), no cross-cell/cross-seed generality, no claim outside `vrd/2`'s
order, no depth/effort claim, no fidelity claim, no re-opening of tickets 34–37, and no "better"
language beyond "more viable under `vrd/2`". Phase A adds nothing to that list: **a launched run is not a
result.**

## 8. Stop and failure routes

- Any gate failure, or an unreachable gateway, means **do not launch**: the failure and its evidence are
  recorded and returned as a blocker. (All gates of §4 passed and the gateway answered, so the launch was
  made.)
- To stop a run, kill the recorded runner pid (`2725555`) only. Never `pkill -f`.
- A partial matrix (a skipped, missing or `halted` cell) is reported as incomplete and never scored as a
  smaller experiment (§7.4 inventory precondition; §8.4 step 5b returns `NOT RUN`, exit 3).

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

### 9.4 Liveness: pid AND ledger growth, five polls over 6 minutes

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
| 15:36:14 (t+5:48) | yes, `Ssl` 05:48 | 8 | 636 | **+91 in 48 s** (mid-interval 545 at 15:35:26) | 399 556 / 137 967 | $0.1089 |

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

### 9.7 Status at the moment this ticket returns

Last poll before returning: **`2026-10-18T15:36:14Z`**, pid `2725555` alive (`Ssl`, elapsed `05:48`),
**636 calls**, **$0.1089** spent, 8 cells growing (+91 calls in 48 s), 11 of 15 cell directories present,
load average `0.80`. The run was **demonstrably running when this ticket was committed**, which is all
phase A claims.

## 10. Phase B — poll, completion gate, verdict (`NOT RUN`)

### 10.1 Poll record

The runner was polled in bounded intervals under the §5 liveness rule (pid **and** ledger growth, never the
log). All 21 intervals are in `phase-b-poll.log` (committed, one JSON object per interval, produced by
`phase-b-poll.py`, committed beside it). Call counts are `kind == "call"` rows; spend is
`prompt_tokens/1e6×$0.10 + completion_tokens/1e6×$0.50` over the whole run root.

| UTC | runner | elapsed | calls (cum) | Δ | per arm A/A1/P/U | spend |
|---|---|---|---|---|---|---|
| 15:37:28 | 2725555 | 07:02 | 780 | — | 194/86/303/197 | $0.1331 |
| 15:39:59 | 2725555 | 09:33 | 1 057 | +277 | 267/119/407/264 | $0.1803 |
| 15:42:31 | 2725555 | 12:05 | 1 304 | +247 | 335/151/484/334 | $0.2383 |
| 15:45:03 | 2725555 | 14:37 | 1 591 | +287 | 405/186/598/402 | $0.2862 |
| 15:47:36 | 2725555 | 17:09 | 1 877 | +286 | 478/249/677/473 | $0.3348 |
| 15:50:08 | 2725555 | 19:41 | 2 102 | +225 | 551/322/712/517 | $0.3741 |
| 15:52:43 | 2725555 | 22:17 | 2 233 | +131 | 587/370/716/560 | $0.4301 |
| 15:55:00 | 2725555 | 24:34 | 2 359 | +126 | 620/430/716/593 | $0.4521 |
| 15:56:46 | 2725555 | 26:19 | 2 455 | +96 | 645/476/716/618 | $0.4695 |
| 15:58:31 | 2725555 | 28:04 | 2 538 | +83 | 668/509/716/645 | $0.4838 |
| 16:00:16 | 2725555 | 29:49 | 2 597 | +59 | 689/524/716/668 | $0.5104 |
| 16:01:59 | 2725555 | 31:32 | 2 674 | +77 | 712/555/716/691 | $0.5403 |
| 16:03:39 | 2725555 | 33:13 | 2 768 | +94 | 735/601/716/716 | $0.5567 |
| 16:05:19 | 2725555 | 34:53 | 2 859 | +91 | 757/646/716/740 | $0.5731 |
| 16:07:00 | 2725555 | 36:33 | 2 944 | +85 | 768/693/716/767 | $0.5877 |
| 16:07:26 | 2725555 | 36:59 | 2 958 | +14 | 768/706/716/768 | $0.5901 |
| 16:09:06 | 2725555 | 38:39 | 2 989 | +31 | 768/737/716/768 | $0.5954 |
| 16:10:46 | 2725555 | 40:20 | 3 003 | +14 | 768/751/716/768 | $0.5980 |
| 16:12:07 | 2725555 | 41:41 | 3 003 | +0 | 768/751/716/768 | $0.5980 |
| 16:13:23 | 2725555 | 42:56 | 3 010 | +7 | 768/758/716/768 | $0.5990 |
| 16:14:38 | **GONE** | — | 3 020 | +10 | 768/768/716/768 | $0.6007 |

The operator stop (§3, ticket 38 §8.1) was **never crossed**: peak realised spend $0.6007 against the
$1.60 stop and the $2.00 cap (30 % of the cap); peak attempt count 3 020 against the 4 000 operator cap.
No signal was sent to the runner: it exited on its own, writing `.tmp/39-m2c-evaluation.json` at
16:14:25Z with its ledgers flushed. The +0 interval at 16:12:07 is one lagging poll, not a stall (the
following two intervals grew).

### 10.2 Completion gate (§7.4 inventory precondition + §8 of this ticket)

| condition | measured | result |
|---|---|---|
| one runner, one run root, order of §3 step 7 | `setsid` pid `2725555`, `.tmp/39-m2c-runs`, 15 cell directories | PASS |
| every declared arm has exactly 3 replicates | `A0` 3, `P` 3, `U` 3, `A` 3, `A1` 3 = 15 cell rows | PASS |
| `ran == 15` | `inventory {declared: 15, ran: 15, skipped: null}` | PASS |
| no skipped cell | `skipped: null` | PASS |
| contract gate `inadmissible == 0` | `A 0, A1 0, P 0, U 0` (`m2c-contract-gate.log`) | PASS |
| each arm's evaluation rows present | one evaluation JSON carries all five arms' cells (§3: the declared scoring path exists only for one invocation) | PASS |
| **no cell halted at its call ceiling** | **`A1-f10-s0-r1`: `halted: true`, `ceiling_hit: "cognitive-calls"`, 144 samples (0..143) instead of 145** | **FAIL** |

One condition fails, so the matrix is **truncated** and §7.4 applies verbatim: "Anything else prints
`NOT RUN` and no verdict."

### 10.3 The halted cell — evidence and mechanism

`A1-f10-s0-r1`, from `.tmp/39-m2c-evaluation.json` and its ledger `.tmp/39-m2c-runs/A1-f10-s0-r1/calls.jsonl`:

- `ceiling_rows[0]`: `ceiling cognitive-calls, threshold 256, observed 256, projected_binding_hour 143` —
the ceiling it hit is the declared budget axis `N = 256`, not a token or cost ceiling (those rows report
threshold `2^60` and are ~7×10^12× away, i.e. twelve orders of magnitude).
- `hours` ends at `143`; `alive_by_hour` has **144** samples, so the terminal sample `h = H = 144` — the one
that carries tier 1's persistence predicate — **does not exist for this cell**. Its `H = 1442` is a sum
over hours 0..143 against 1437–1444 summed over 0..144 for the other cells: the two are not comparable.
- The ledger holds **256 charged `call` rows**, spanning actor-hours 0..142, with `attempt: 2` on exactly one
of them (a single retry) and one `status: timeout` row (seq 606). It needed a 257th attempt for hour 143's
decision, `AllowCall` refused it, and the typed fail-loud halt ended the cell at hour 143.
- **The arm has no headroom at this ceiling.** `A1`'s quota schedule demands two decisions in 112–113 of the
145 hours; `A1-f10-s0-r2` and `-r3` finish the horizon on exactly 256 attempts, i.e. with zero spare. One
charged retry is enough to truncate the cell. The single `-attempt-timeout 300` expiry (seq 606, retried once,
`attempt: 2`) is therefore not a cosmetic transport event on this arm: it cost the sample and the run its
admissibility.
- This is also why §7.4 names the case: the truncated replicate's own spread is artificially small
(`A1` envelope half-width 4.0 actor-hours over `1437/1442/1444`), and a band that narrow would make any
neighbouring pair look separated. The defect is in the run, not in the metric, and the guard exists to stop
exactly that reading.

### 10.4 The four commands of §6, in order, and what they returned

| # | command (verbatim, cwd `/home/z/vibe/agentworld`) | exit | result |
|---|---|---|---|
| a | `python3 .tmp/35-admis.py .tmp/39-m2c-runs` | 0 | `A {'applied': 767, 'rejected': 1, 'inadmissible': 0, …}`, `A1 {…'inadmissible': 0…}`, `P {…0…}`, `U {…0…}` — **contract gate PASS** (`m2c-contract-gate.log`) |
| b | `python3 .scratch/38-m2c-preregistration/vrd2.py --check-inventory .tmp/39-m2c-evaluation.json` | **3** | `cell inventory (spec section 8.4): FAIL` / `1 cell(s) did not complete (halted at the call ceiling or not run): A1/r1` / `M2c verdict (spec section 7.4): NOT RUN` (`m2c-inventory-gate.log`) |
| c | `python3 .scratch/38-m2c-preregistration/vrd2.py --live .tmp/39-m2c-evaluation.json --scores .scratch/37-fidelity-metric/scores.json --log .scratch/39-m2c-matrix/m2c-scores.log` | **3** | identical refusal, no per-pair line printed, `m2c-scores.log` is 4 lines (`m2c-live.out`) |
| d | `.tmp/39-vs2-bin -curve-in .tmp/39-m2c-evaluation.json -curve-out .scratch/39-m2c-matrix/m2c-curve.txt` | 0 | 480-line curve report; 9 of 10 pairs `DISTINGUISHABLE`, `A@low vs P@low` not distinguishable (`m2c-curve.txt`) — **corroboration only, and it pools the truncated `A1` r1 into the `A1` band, so it cannot rescue or contradict a verdict it was declared not to decide** |

The committed `m2c-curve.txt` is the report verbatim with the generator's trailing column padding removed (content unchanged, `git diff --check` clean); every other evidence file is committed byte-for-byte as produced.

No fifth run, no re-scoring, no hand computation substituted for `vrd2.py`. Steps (b) and (c) refused by
their own declared rule, which is the machinery working as pre-registered.

### 10.5 Verdict

**`NOT RUN`** — the declared inventory is not met (`A1-f10-s0-r1` halted at its call ceiling at hour 143),
so per ticket 38 §7.4 and §8 of this ticket the matrix is reported as incomplete and is **never scored as a
smaller experiment**. No primary pair (`A` vs `P`, `A` vs `U`) is reported as separating or not separating;
no band, no effect floor, no six-permutation sensitivity, no `h = H−1` variant is reported, because each of
those would be a comparison the declared rule forbids on this artefact. The question M2c asks is therefore
**still open**, not answered negatively.

### 10.6 Descriptive per-arm state vectors (an inventory, not a verdict)

Reported so the next run can be compared against this one, and explicitly **not** a `vrd/2` comparison —
no pair below has been tested against a band or a floor, and `A1`'s r1 column is not comparable to any
other cell's (144 samples vs 145).

| arm | H (r1/r2/r3) | mean H | births B | starvation S | alive@H | charged calls | `halted` |
|---|---|---|---|---|---|---|---|
| A0 | 1444/1444/1444 | 1444.0 | 2/2/2 | 0/0/0 | 1/1/1 | 0 | no |
| P | 1444/1353/1444 | 1413.7 | 2/2/2 | 0/1/0 | 1/1/1 | 244/228/244 | no |
| U | 1184/1010/1181 | 1125.0 | 2/3/2 | 3/7/3 | 1/2/1 | 256/256/256 | no |
| A | 1419/1444/1391 | 1418.0 | 2/2/2 | 1/0/1 | 1/1/1 | 256/256/256 | no |
| A1 | 1442/1444/1437 | 1441.0 (mixes a 144- and two 145-sample sums) | 2/2/2 | 0/0/0 | 2/1/1 | 256/256/256 | **r1 halted** |

Two facts of record that do not depend on the failed cell: the `A0` anchor reproduced exactly in-matrix
(`H = 1444, B = 2, S = 0, alive@144 = 1` in all three replicates, matching gate 7 of §4), and **no cell of
any arm went extinct** (`alive@H ≥ 1` everywhere, so tier 1's unanimity rule had nothing to decide even on
the complete cells).

### 10.7 Spend and inventory

| item | value |
|---|---|
| physical attempts | 3 020 of the harness cap 3 072 and the operator cap 4 000 |
| tokens | 1 897 637 input / 821 860 output |
| realised spend | **$0.6007** ($0.10/M in, $0.50/M out) against the $1.60 operator stop and the $2.00 hard cap |
| wall clock | 15:30:26Z → 16:14:25Z = 43 min 59 s (ticket 38 §8.2 declared 3–4 h, deadline 6 h) |
| per-arm charged calls | `A0` 0; `P` 716; `U` 768; `A` 768; `A1` **768** (the report prints 512 — see 10.8 item 2) |

### 10.8 Defects and anomalies found in the report (recorded, not repaired here)

1. **`A1` is outside the report's own accounting bands.** `budgetedArms = {P, U, A}` (`cmd/vs2/config.go:32`),
so `VS2-EC-2` prints `budgeted cells=3 … ceiling-halts=0` while a fourth budgeted arm did halt. `VS2-EC-3/4`
have `fidelity rows=0` (no `R` reference, which §7.1 excludes by design) and are `not-evaluable`; `VS2-EC-5`
replays 11 priority cells, not 12. **The halt is therefore visible only in the cell rows and in `vrd2.py`'s
inventory check — the report's own bands do not see it.**
2. **A halted cell reports `realized_calls: 0` while it charged 256 attempts.** `cmd/vs2/run.go:189` reads the
calls only `if !cr.halted`, so `A1-f10-s0-r1` reports `realized_calls 0`, `under_budget true`,
`budget_shortfall 256`, `allocation_records 0`, `escalated_decisions 0` — yet carries `prompt_tokens 160542`
and `completion_tokens 125597`. The curve report's per-condition table consequently prints `A1@low … calls
512` with the tokens of three full cells. Anyone reading the accounting table off the report gets `A1`'s call
count wrong by 256.
3. **`final_bundle_error: "invalid natural evidence journal"` on all 12 model-bearing cells** (and on the
recorded ticket-36 `R` cell; the zero-call `A0` cells have none). Pre-existing, model-bearing-cell-specific,
and unrelated to the halt: it is a post-run resume-bundle export failure, and the projections the verdict
reads were written. Recorded so it is not read as new.
4. Three non-`ok` call rows in the whole matrix (2 × `http-error 400` content policy: `A-f10-s0-r1:63`,
`P-f10-s0-r3:311`; 1 × `timeout`: `A1-f10-s0-r1:606`). All four arms record `inadmissible: 0`; the role-a
tally is `A {applied 767, rejected 1, malformed 1}`, `A1 {764, 3, 3}`, `P {713, 3, 3}`, `U {767, 1, 1}` — 8
malformed answers in total, every one of them rejected by the contract rather than applied, i.e. the contract
held. The single retried attempt is nonetheless what truncated `A1` r1.

### 10.9 What this does not support

Ticket 38 §7.7 unchanged, plus the verdict: this is **not** a negative result. `NOT RUN` says nothing about
whether a budgeted arm separates from `P` or `U` — it says the artefact cannot answer it. In particular it
is **not** evidence that the arms are indistinguishable, and the curve report's `DISTINGUISHABLE` lines are
not a substitute (they are corroboration by declaration, they do not carry the `vrd/2` order, and they pool
the truncated replicate). One cell, one contract (`natural-claim/v4`), one horizon (`H = 144`), 3 replicates,
`N = 256`; the central claim, and the selective-vs-uniform **spend** comparison, stay with M2d/M4.

### 10.10 Evidence paths and the next action

| artefact | home |
|---|---|
| evaluation JSON the gate read (byte copy, `sha256 56d3b300…4df26`, 774 395 B) | `.scratch/39-m2c-matrix/m2c-evaluation.json` (live original: `.tmp/39-m2c-evaluation.json`) |
| contract gate transcript | `.scratch/39-m2c-matrix/m2c-contract-gate.log` |
| inventory gate transcript | `.scratch/39-m2c-matrix/m2c-inventory-gate.log` |
| scorer transcript / its log | `.scratch/39-m2c-matrix/m2c-live.out`, `.scratch/39-m2c-matrix/m2c-scores.log` |
| curve corroboration | `.scratch/39-m2c-matrix/m2c-curve.txt` |
| poll record and its script | `.scratch/39-m2c-matrix/phase-b-poll.log`, `phase-b-poll.py` |
| run root (126 MB of bundles; not tracked) | `.tmp/39-m2c-runs/` |

**Next action (not done here):** the options and their costs are §11.5. Two clauses of the first draft of
this paragraph are repaired by §11.2 and §11.5: a single-cell re-run *is* mechanically available
(`-arms A1 -reps 1`), so the obstacle is the scoring artefact, not the CLI; and **raising `N` adds no
headroom**, because the per-hour quota is derived from `N` and always sums to it (§11.1) — an arm at
`N = 264` would still end with zero spare. Any change here is at the level of a declared run parameter, so
it is proposed, not applied.

## 11. Diagnosis — what stopped `A1-f10-s0-r1`

Diagnosis only: **zero live calls, no re-run, no scoring change, no edit to any declared rule or to the scorer.** The verdict stays `NOT RUN` (§10.5). Bound to the artefact of §10: the committed evaluation JSON `sha256 56d3b300…4df26` (byte-identical to `.tmp/39-m2c-evaluation.json`) and the run root `.tmp/39-m2c-runs`. Every number below is re-derived by `diagnose.py` from those two inputs, whose output is committed as `halt-evidence.json`; the script computes no band, no effect floor and no pair comparison, so nothing here is a score.

### 11.1 Q1 — the halt path, its unit, its number, its source

The chain, end to end:

| step | where | what it does |
|---|---|---|
| the declared number | `cmd/vs2/config.go:37` | `const DeclaredCallCeiling uint64 = 256` |
| the per-arm ceiling | `cmd/vs2/config.go:172-184` | `budgetFor(arm)`: `MaxCalls: calls` = 256 for every arm but `R` (4 096); the other three axes are `1<<60` (`MaxCostMicros: 1 << 60`, `cmd/vs2/config.go:181`), so cost is not the binding axis |
| one account per cell | `cmd/vs2/run.go:93` → `cmd/vs2/harness.go:123-127` | `wireStack(…, budgetFor(spec.Arm), …)` builds `NewCognitiveAccount(budget)` once per arm-replicate and hands it to that cell's `Recorder` |
| the pre-call guard | `internal/cognition/client.go:353` | `if err := r.budget.AllowCall(req.Hour); err != nil { return CallResult{}, err }` — **inside the retry loop**, so every physical attempt passes it |
| the refusal | `internal/cognition/budget.go:163-170` | `if a.calls+1 > a.budget.MaxCalls { return a.cross(CognitiveCeilingCalls, …) }` |
| the charge | `internal/cognition/budget.go:184-186` | `a.calls++` per attempt, then `if a.calls > a.budget.MaxCalls` |
| the halt | `cmd/vs2/run.go:106-113` | the typed `*CognitiveCeilingHitError` sets `cr.halted = true` and `break`s the step loop — the cell stops, it does not continue on fallback decisions |
| the blind spot | `cmd/vs2/run.go:189` | `if !cr.halted { … realizedCalls++ }` — a halted cell reports `realized_calls: 0` (§10.8 item 2) |

**The unit is one cell-run (one arm × one replicate × one cell), counting physical attempts, over the whole simulated horizon.** Not per actor (`budget.go` has no actor dimension), not per arm (each of the three `A1` replicates owns its own account, which is why the arm total is 768), not per wall-clock window and not per simulated hour (nothing resets the account). Nor is it a dollar ceiling: `MaxCostMicros` is `1<<60` and the report cost is `UNPRICED`, which is why §3 spends by the operator poll rule instead.

**The halt, in the ledger.** `A1-f10-s0-r1` has 655 rows: 144 `allocation`, 256 `call`, 255 `role-a`. The last two rows are `seq 652`, a `call` at actor-hour 142 with `status: ok`, and `seq 654`, an `allocation` at hour 143 with `quota 1`, `roster_size 2`, `shortfall 0`, `selected [2017]` — and **no `call` row after it**. The 257th attempt that hour-143 decision required was refused. The cell row records the same crossing in typed form:

```
ceiling_rows[0] = {"ceiling": "cognitive-calls", "threshold": 256, "observed": 256,
                   "rate_last_hour": 256, "projected_binding_hour": 143}
ceiling_hit = "cognitive-calls"      halted = true      samples = 144 (hours 0..143)
```

(`rate_last_hour` equals the whole-run total because `ObserveHour` is called once at finalize, so this row records threshold and observed correctly but is not a trailing rate; the two token axes read 160 542 and 125 597 against a threshold of `2^60`, twelve orders of magnitude away.)

**The trigger is one retried attempt.** `seq 606` is `status: timeout`, `attempt: 1`, `latency_micros: 300001867` — exactly the declared `-attempt-timeout 300` — and `seq 607` retries it, `attempt: 2`, `status: ok`. `retryable()` (`internal/cognition/client.go:179-187`) returns true for `timeout`, so the retry is the declared behaviour. That retry bought no decision: matrix-wide, 3 020 charged calls produced 3 019 `role-a` decisions, and **the single difference is this row**. `A1`'s declared quota schedule is `q(h) = floor(N/H) + (1 if h < N mod H)` (`internal/cognition/priority.go:250-259`, whose comment says "so sum_{h<H} q(h) = N"), i.e. `q(h) = 2` for h < 112 and `1` for h ≥ 112, summing to exactly 256 over the 144 decision hours (hours 0..143 — the horizon's 145 samples 0..144 contain 144 decision hours). So the cell reached 256 attempts at hour 142 and hour 143's decision needed attempt 257.

**Where 256 comes from.** The value is not computed at run time. Ticket 38 §7.1 declares it as the budget axis (`| budget axis | the declared call ceiling N = 256 per budgeted arm-run (unchanged) |`) and §7.3 declares that it is charged in physical attempts ("Ceiling 3 072 physical attempts, because `AllowCall` is charged per attempt (`internal/cognition/client.go:353`)"). The value itself is inherited from the VS2 design: `.scratch/24-vs2-attention/spec.md:477` proposes `N = 256` calls per cell, and §5.1 there fixes it from the **model-free `A0` claim-wake projection** and the VS1 periodic-6 arm (≈300/cell) — deliberately "not derived from any model-bearing arm's realized candidate count". In other words **`N` was sized from the control's candidate supply, never from an escalation rate**, which matters for Q4.

### 11.2 Q2 — why `r1` and not `r2`/`r3`

All three `A1` replicates are at exactly 256 charged attempts; they are not distinguished by spend or by escalation rate at all, but by one retried attempt.

| replicate | charged calls | decisions (`role-a`) | decision hours with a call | decision rate (per sim. hour) | wall span | calls/min | first hour at 250 / 254 / 255 / 256 | left under 256 | non-`ok` rows |
|---|---|---|---|---|---|---|---|---|---|
| `A1-r1` | 256 | **255** | 143 (h0..h142) | 1.783 | 15:45:38→16:13:57 (28.3 min) | 9.04 | 136 / 140 / 141 / **142** | 0 | 1 `timeout` (h127, retried) |
| `A1-r2` | 256 | 256 | 144 (h0..h143) | 1.778 | 15:48:54→16:10:02 (21.1 min) | 12.12 | 137 / 141 / 142 / **143** | 0 | none |
| `A1-r3` | 256 | 256 | 144 | 1.778 | 15:30:26→15:49:59 (19.5 min) | 13.10 | 137 / 141 / 142 / **143** | 0 | none |

Read off that table:

- **By spend: no.** Call count is identical (256 = 100.0 % of the ceiling for all three); the quota schedule, not the arm, fixes it. By *dollars* `r1` is the most expensive cell in the matrix (`$0.0789` against `$0.0629`/`$0.0466`) but only because its answers were longer (125 597 output tokens against 93 629/61 046) — output verbosity, not the halt, which is a call-count event.
- **By escalation count: no.** All three *planned* 256 decisions (the quota schedule is a declaration, not an outcome); `r1` realized 255 because the retry consumed the unit that hour 143's decision needed.
- **By decision mix: no.** The ten founding actors' escalation counts are `{1:13, 2:2, 3:4, 4:1, 5:8, 6:4, 7:4, 8:8, 9:12, 10:8}` in `r1` and in `r2`; `r3` differs only in three actors (`2:3, 9:14, 10:14`). The two newborn actors differ across all three, as they must on temperature-sampled trajectories.
- **By rate: only in wall clock.** `r1` ran at 9.0 calls/min against 12.1/13.1 — because the 300 s timeout is 5 of its 28.3 minutes. Per simulated hour nothing differs: the schedule is fixed.
- **The one difference that matters:** the cumulative-attempt milestone table. From hour 127 (`q = 1`) `r1` is exactly **one attempt ahead** of `r2`/`r3` for the rest of the run: 241 vs 240 at h127, 254 vs 253 at h140, 256 vs 255 at h142, and at h143 `r1` has 0 remaining while they spend their 256th on that hour's decision. `r1` is not an outlier; it is `r2`/`r3` shifted by one retried attempt, and one attempt is the whole of `A1`'s headroom.

### 11.3 Q3 — every arm against its ceiling

Ceiling `N = 256` per cell; `A0` carries no account (`cmd/vs2/harness.go:103` returns before the ledger is created) and no ceiling.

| arm | charged calls r1 / r2 / r3 | % of the 256 ceiling | left under the ceiling | hits |
|---|---|---|---|---|
| `A0` | 0 / 0 / 0 | — (no account, 0 calls by construction) | — | 0 |
| `P` | 244 / 228 / 244 | 95.31 / 89.06 / 95.31 | 12 / 28 / 12 | 0 |
| `U` | 256 / 256 / 256 | **100.00 / 100.00 / 100.00** | **0 / 0 / 0** | 0 |
| `A` | 256 / 256 / 256 | **100.00 / 100.00 / 100.00** | **0 / 0 / 0** | 0 |
| `A1` | 256 / 256 / 256 | **100.00 / 100.00 / 100.00** | **0 / 0 / 0** | **1 (`r1`)** |
| arm totals | `P` 716/768, `U` 768/768, `A` 768/768, `A1` 768/768 | 93.23 / 100.00 / 100.00 / 100.00 | | |

**There was no second near-miss because there was no miss: eight of the twelve model-bearing cells finished on exactly their last permitted attempt, with zero reserve.** `U`×3, `A`×3 and `A1`×2 are at 100.0 % of the ceiling *and complete*, which is only possible because every one of their attempts bought a decision. The margin between those eight cells and `A1-r1` is a single retried attempt, and the matrix came within one HTTP status code of losing a second cell: `A-r1` carries one `http-error 400` (content-policy, `seq 63`, h12), which `retryable()` classifies as **not** retryable, so it consumed exactly one attempt; had it been a `timeout`/`5xx`/`429` it would have been retried, taken that zero-headroom cell to a 257th attempt, and halted it identically. `P-r3`'s identical `400` (`seq 311`, h72) is not a near miss: `P` had 12 attempts of headroom.

`P` is the only arm with headroom, and it has it by accident of its own gate rather than by declaration: it escalates only at `h ≡ 0 (mod 6)` (`cmd/vs2/config.go:51`), so at most 24 decision hours can supply calls, and its three runs supplied 244/228/244 against a ceiling of 256. The four "equal-budget" arms are therefore not equal in headroom: `P` is structurally under-supplied, and `U`/`A`/`A1` are structurally *exactly*-supplied by the quota formula.

### 11.4 Q4 — classification

**It is a plan error, and specifically the one the phrase "sized from a different escalation rate" does not name.** It is not an infrastructure fault and it is not a legitimate arm outcome the criterion left unhandled.

**Not (b), an infrastructure fault.** A 300 s attempt timeout is an external event, but nothing malfunctioned: the retry fired the declared `retryable` rule, each attempt's record was appended and fsynced before it was charged (`internal/cognition/client.go:392` then `:423`; `internal/cognition/ledger.go:130-140`), the crossing was typed, the cell was marked `halted`, the other 14 cells finished, the report was written, and the operator stop was never approached (peak `$0.6007` of the `$1.60`/`$2.00` limits). The harness did exactly what the design's §5.2 (`.scratch/24-vs2-attention/spec.md`) and `VS2-EC-2` declare. Calling this infrastructure would blame the transport for a scheduling decision taken in the plan.

**Not (a), "the budget doing what a budgeted arm should do".** §7.4 *has* the rule — "no cell whose run halted at its call ceiling … Anything else prints `NOT RUN`" — and the rule fired correctly; there is no gap in the verdict machinery. What is wrong is the plan underneath it, in three linked places:

1. **The quota schedule spends 100 % of `N` by construction.** `q(h)` is defined so that `Σ_{h<H} q(h) = N` (`internal/cognition/priority.go:250-259`), so a *completing* `U`/`A`/`A1` cell also ends on exactly its last permitted attempt (§11.3). Headroom is not a parameter of the declared design; it is zero for every arm the formula binds.
2. **The ceiling counts a different unit from the schedule.** The schedule counts *decisions* (`s.spent += uint64(len(selected))` at `internal/cognition/priority.go:149,171,208`); the account counts *physical attempts* (`a.calls++` per attempt, `internal/cognition/budget.go:184-186`), and the frozen criterion says so outright: `VS2-EC-2`'s string is "an (N+1)-th attempt is the typed fail-loud halt" (`cmd/vs2/bands.go:141`). One retried attempt therefore buys no decision and costs a decision's worth of budget. Ticket 38 §7.3 restated the attempt charging but §7.1 restated `N = 256` as the arm's *call* budget, so the pre-registration inherited the mismatch without resolving it.
3. **The consequence was already in the project's own record.** Ticket 34 recorded the identical halt — "256 call records = 253 logical calls + 3 retried transport timeouts … the budget charges every physical attempt while the P selector schedules logical calls, so the 257th attempted call was refused" (`.scratch/34-vs2-live-matrix/spec.md:153-160`) — and wrote that "the milestone review owns the reading (attempt-charged vs logical-call ceiling); no source edit was authorised or made". `docs/VERTICAL-SLICES.md:163` carries the same sentence. Ticket 38 §8.1 then used that halt to justify the *dollar* cap ("the cap absorbs a retry storm (ticket 34's EC-2 halt was caused by retries charged per attempt)") while leaving the *attempt* headroom at zero.

**Not (c) as phrased.** `N = 256` was not sized from an escalation rate at all — it came from the model-free `A0` candidate count (§11.1), and the arm's escalation count is a declared schedule that always spends exactly `N`. The error is not the number; it is that the number is enforced in a unit (`attempts`) the arm does not schedule in (`decisions`), combined with a schedule that leaves the difference at zero.

**What this decides for the rerun question.** A rerun at the identical declaration is *legitimate* — the artefact genuinely fails the declared inventory and the rule that refuses it is sound — but it is **not a fix**: at zero headroom the same single retryable transport event truncates the same arm again, and nothing about `r1` was exceptional (§11.2). The rule that is incomplete is not the verdict rule; it is the inherited declaration that one number can be both the arm's decision budget and the harness's attempt ceiling. A one-event sample cannot estimate the retry rate, so no completion probability is quoted here; what is certain is the structure, not its failure frequency.

### 11.5 Q5 — options, with costs

Costs are estimated from this run's own measurements — `$0.6007` for 3 020 attempts, i.e. `$1.989e-4` per attempt (`halt-evidence.json`), and `$0.0366–$0.0789` per model-bearing cell — not from ticket 38's pre-run projection, whose upper bound (`$4.2e-4`/call) was 2.1× the realised price. Attempt counts are exact. Wall clock is the realised 44 min for a full matrix (15–28 min per lane, two waves at `-parallel 8`).

| # | option | $ cost | time | new declaration needed | what it does to the cause |
|---|---|---|---|---|---|
| i | keep the declaration, re-run `A1-r1` only at the identical `N` | **$0.05–0.08** (256 attempts) | 20–30 min run + 1 review cycle | **yes** — a merge rule, a stopping rule, and a rewritten `§10.2` gate row (see below) | **nothing**: same zero headroom, so it is the same coin flip |
| ii | raise the per-arm ceiling for all arms, re-run the whole matrix | **$0.63** (3 168 attempts at `N=264`) | 45–60 min + re-pre-registration + review | yes — 5 sentences (below) | **nothing**: `q` is derived from `N` and still sums to it, so headroom stays 0 |
| iii | declare budget exhaustion an arm state; score the terminal sample at h143 | **$0.00** | 2–4 h scorer + declaration + review | yes — 6 sentences **and** a scorer change | **hides** it: the transport loss becomes an arm state |
| iv | shorten the horizon so the declared `N` suffices | **$0.00** | — | yes — `H` is cited in §7.1, the M2c question and every tier | **nothing**: `Σq(h) = N` for *any* `H`; shorter horizon, same zero headroom |
| v-a | split the ceilings: `N` = decisions, attempts = `N + R` | **$0.63** + code/test | 45–60 min run + 2–4 h work | yes — and it changes `VS2-EC-2`'s frozen criterion | **fixes** it, at the price of touching the instrument |
| v-b | keep the 256 attempt ceiling, declare a transport reserve inside it (`Σq(h) = N − R`) | **$0.63** + one constant | 45–60 min run + 1–2 h work | yes — one design-parameter sentence + one constant | **fixes** it, at the price of changing what the arms spend |
| v-c | stop spending on M2c now; record `NOT RUN`, carry the question to M2d | **$0.00** | 0 | no | leaves it open; spends nothing |

Per option, the sentences and conclusions that change:

- **(i)** Mechanically available — `-arms A1 -reps 1` produces exactly `A1-f10-s0-r1` — but `main.go:190` `os.RemoveAll(*root)` wipes a root at launch and `prepareRunDir` refuses an existing cell directory (`cmd/vs2/harness.go:24-33`), so a second invocation needs a fresh root, and `vrd2.py --live` takes **one** path (`vrd2.py:837`). The matrix could only be scored by merging two artefacts, which contradicts §3's premise ("the declared scoring path only exists if the matrix is one invocation") and §10.2's "one runner, one run root" gate row. A merge also needs a **stopping rule** declared in advance (if the re-run is truncated again, does it re-run again?), otherwise which artefact is reported becomes a choice made after seeing the data. Invalidates: §3's premise sentence and §10.2's gate row; nothing already concluded (there is no verdict) and none of §10.6's vectors for the other 14 cells. Within the currently declared caps (`$2.00`, 4 000 attempts: 3 276 total).
- **(ii)** Needs ticket 38 §7.1's budget-axis row, §7.3's "12 model-bearing runs at `N = 256` … 3 072 physical attempts", §8.1's `3 027`-call arithmetic and its "1.56×" justification, ticket 39 §2's citation table, and the inherited "`N = 256` calls per cell" of `.scratch/24-vs2-attention/spec.md:477`. It also changes the arms, not just the margin: at `N = 264`, `q(h) = 2` for h < 120 instead of 112, so `U`/`A`/`A1` make 264 decisions instead of 256 and the per-hour pressure is different — a different experiment. Invalidates the 14 complete cells of this run (different `N`, cannot be combined), and it **exceeds the currently declared 4 000-attempt cap** (3 020 + 3 168 = 6 188), so it needs fresh authority even though the dollars stay under `$2.00` (`$1.23` cumulative). Does not touch `vrd/2`, the `v4` contract or the `A0` anchor. **Its declared purpose is not achieved**: `Σq(h) = N` holds for the new `N` too.
- **(iii)** The biggest change for the least spend, and the one that hides the most. It edits ticket 38 §7.1 (`horizon H = 144 (samples 0..144)`), §7.4's inventory-precondition sentence, §7.5's "terminal-sample sensitivity" (the variant becomes the primary), the effect floor's basis (`ε₂ = 0.01` is 1 % of the control's 1 444 actor-hours; the control's h143 sum is 1 443), and ticket 39 §2 — plus a scorer change, because `cells_of` sums **all** samples for `H` and reads `alive_by_hour[-2]` as "alive@143" (`vrd2.py:405-424`), which for a 144-sample cell is h142, not h143. It would hide (1) that a *transport attempt*, not the allocation policy, consumed the arm's last unit — hour 143's decision was never made, so labelling this an arm state is false; (2) that 8 of 12 cells sat at 100 % of the ceiling with zero reserve, so the next retryable event truncates the next cell the same way; (3) the h144 sample on every cell, including the boundary case §7.5 names. It is also **post hoc**: the amendment would be chosen after seeing which cell failed, which is the selection effect the precondition exists to prevent. It does not "score a truncated matrix" if it is declared — but it cannot make this artefact a result, only a differently-labelled one.
- **(iv)** Is void as a repair, and the reason is structural: `q(h) = floor(N/H) + (1 if h < N mod H else 0)` sums to `N` for **every** `H`, so shortening the horizon raises `q(h)` and leaves the arm on exactly its last permitted attempt. It changes `H` (cited in §7.1, in the M2c question's horizon, and in the basis of all four tiers' `H`-sums), invalidates every `H = 144` baseline in tickets 35–39, and buys nothing.
- **(v-a)** Makes `N` mean what the M2c question says it means ("equal declared call ceiling"), keeps the 256-decision schedule and therefore the arms, and gives retries a separate fail-loud ceiling. Cost: `budget.go` (`AllowCall`/`Charge`), `client.go:353`, the selector's `CallCeiling` guard (`priority.go:115-117`, which refuses a budget that disagrees with the declared `N`), tests, one §7.1/§7.3 sentence pair, and a full re-run (`$0.63`, 3 072 attempts — over the current 4 000-attempt cap with run 1). It **changes the frozen instrument**: `VS2-EC-2`'s criterion string (`cmd/vs2/bands.go:141`) declares the `(N+1)`-th *attempt* as the halt, so this option edits a ticket-34 exit criterion, and §7.7 forbids re-opening tickets 34–37 without authority. It does not change `vrd/2`.
- **(v-b)** Keeps `VS2-EC-2` and `N = 256` exactly as declared and gives the *schedule* a declared reserve: `Σq(h) = N − R` for a declared `R` (e.g. 8 retried attempts; one call can spend at most 3 attempts, `DefaultMaxAttempts = 3`, `internal/cognition/client.go:52`). The ceiling still fires on the `(N+1)`-th attempt, so `VS2-EC-2` and the halt path are untouched; what changes is that the schedule stops selecting at `N − R` decisions and leaves `R` attempts unconsumed for retries. Cost: one declared constant, the quota function's numerator, one design-parameter sentence, tests, and a full re-run (`$0.63`, 3 072 attempts, over the current 4 000-attempt cap). It **changes what the arms spend** (`N − R = 248` decisions instead of 256 for `U`/`A`/`A1`), which is a change to the arms rather than to the instrument — the opposite tradeoff to (v-a).
- **(v-c)** The only option that spends nothing: `NOT RUN` stands as M2c's answer, the `$0.6007` buys the anchor, contract, cost and ceiling-path evidence of §11.6, and the open question moves to M2d. It invalidates nothing and answers nothing.

The choice between (v-a) and (v-b) turned on one question: **is `N` the arm's decision budget or the harness's attempt ceiling?** The M2c question, the quota formula and ticket 24 §5.1 say decisions; `VS2-EC-2` says attempts; the two cannot both be `256` on an arm whose schedule spends everything. The product owner settled it in favour of **(v-b)** — `N = 256` stays the attempt ceiling, the schedule pays `N − R` decisions, `R = 8` — recorded and implemented in `.scratch/40-m2c-transport-reserve/spec.md`, with the re-run in ticket 41. The authority under which the re-run executes is raised to **7 000 cumulative physical attempts and `$1.20` per run** (ticket 40 §7): the cap clauses inside (i), (ii), (v-a) and (v-b) above were written against the first run's `4 000`/`$2.00`, and the choice between the options did not turn on them — it turned on the frozen instrument (`§7.7`). The "sentences that change" lists inside (i)–(v-c) are this section's own analysis of each option's blast radius; where they name ticket 38 §7.1, §7.3 or §8.1, ticket 40 §6 is the record of what actually changed.

### 11.6 Q6 — what the current artefacts still support without scoring

Supported, from the committed artefact and the run root:

- **A complete inventory statement.** 14 of 15 cells and 11 of 12 model-bearing cells ran the full horizon (145 samples, h0..h144); one cell (`A1-f10-s0-r1`) ran 144 samples (h0..h143) and is `halted` with `ceiling_hit: cognitive-calls`. `inventory {declared 15, ran 15, skipped null}`, 15 cell directories, one runner (`§10.2`).
- **The `A0` anchor, exactly.** Three in-matrix replicates at `H = 1444`, `B = 2`, `S = 0`, `alive@144 = 1`, reproducing the zero-spend gate of §4 row 7 (`anchor-gate.json`) — a measurement of the control, not a comparison.
- **The contract held.** `inadmissible: 0` on all four model-bearing arms (`m2c-contract-gate.log`); 3 019 decisions, 8 malformed answers all rejected by `natural-claim/v4` rather than applied; 3 non-`ok` call rows in 3 020 (2 content-policy `400`, 1 `timeout`).
- **The ceiling path works and is measurable.** A crossing is typed, the ledger is flushed before the charge, the cell is marked, the run stops, and the crossing is visible in the cell row and in `vrd2.py`'s inventory check — with the two report blind spots already recorded in §10.8 items 1–2 (a halted cell reports `realized_calls: 0`; `A1` is outside `budgetedArms`, so `VS2-EC-2`'s own band did not see the halt).
- **A realised cost model.** 3 020 attempts, `$0.6007`, 1 897 637 in / 821 860 out = `$1.989e-4` per attempt, against ticket 38 §8.1's projected band (`$1.5e-4` lower / `$4.2e-4` upper).
- **Descriptive state vectors at h144 for the 14 complete cells** (`§10.6`) — an inventory to compare a future run against, explicitly not a comparison.
- **The full-horizon terminal sample exists for every cell except `A1-r1`**, and every cell's h143 sample exists; every cell is alive at h143 (2 or 3), and every complete cell at h144 (1 or 2). That is a property of the tier-1 predicate's input at the two candidate samples and decides nothing on its own.

Not supported — and not to be inferred from any of the above:

- **No verdict, and no pair, under §7.4.** The precondition is artefact-wide ("A verdict is computed only over an artefact carrying the declared inventory … Anything else prints `NOT RUN`"), so not even `A` vs `P` — a pair that never touches `A1` — may be read. `NOT RUN` is not a negative result and says nothing about whether any arm separates.
- **No tier, band, effect floor, permutation or `h = H−1` variant.** Each is a comparison the declared rule refuses on this artefact (`§10.4` returns 3, `§10.5`).
- **No arm-level statement about `A1`.** Two complete replicates plus one 144-sample replicate cannot give an envelope; `A1`'s `H` would mix 144- and 145-sample sums (`1437/1442/1444`), which is the comparison §7.4 excludes by name.
- **No substitute from the curve report.** Its 9-of-10 `DISTINGUISHABLE` lines are declared corroboration, do not carry the `vrd/2` order, and pool the truncated replicate (`§10.4` item d).
- **No score computed here.** `diagnose.py` reads only counts, hours and state vectors; it prints no difference, band or verdict. The `h143` numbers in `halt-evidence.json` are raw sums at a named sample, not a re-scored matrix.

### 11.7 What this section did not do

No live call, no re-run, no scorer or ledger edit, no change to ticket 38 §7 (the reserve's re-declaration is ticket 40's), and no push. The declared verdict remains `NOT RUN`. The options of §11.5 were proposals for the product owner; ticket 40 records the decision (option v-b) and implements it, and ticket 41 owns the re-run.
