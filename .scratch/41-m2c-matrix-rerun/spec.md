# Ticket 41 — M2c part 3: re-run the M2c matrix under the transport reserve, and score it

- Record kind: the run record of the M2c re-run. Written and committed **before the launch**, because the
  accounting state the re-run is expected to produce must be declared before the numbers exist.
- Status at commit: pre-live gates PASS (§6), gateway reachable (§6), launch declared in §5.7 and executed
  immediately after this commit; §7 records the pid and the UTC launch second, §8 stays empty until the
  artefact exists.
- Base revision: `ae0396f` (master, ticket 40 merged). Branch: `feature/41-m2c-matrix-rerun`.
- Author: worker agent (ticket 41), under the advance-phase plan.
- Inputs, in order: the pre-registered criterion `.scratch/38-m2c-preregistration/spec.md` §7 (arms, cell,
  horizon, replicates, contract, KPI set, band + effect floors `ε_H = 0.01, ε_S = 0.1, ε_B = 0.5`, verdict
  rule, falsifier) and §8 (run plan, gates, detachment, caps); the diagnosis of the first matrix run
  `.scratch/39-m2c-matrix/spec.md` §10–§11 (why `A1-f10-s0-r1` halted and why the artefact was `NOT RUN`);
  the repair and re-pre-registration `.scratch/40-m2c-transport-reserve/spec.md` §3 (the reserve `R_t = 8`)
  and §7 (the costed plan and caps for this run).
- The criterion is **not** re-declared here. Where this file restates a §7 value it is a citation, and §7
  owns the value.

## 1. Goal

Produce the **first complete 15-cell M2c artefact** at the repaired budget axis, and let the
pre-registered rule (§7.4–§7.6) produce a verdict from it once:

> Under `natural-claim/v4` at cell `f10-s0`, horizon `H = 144`, does any budgeted allocation arm (`A` on
> `vs2-value/2`, `A1` on `vs2-value/1`) separate from periodic (`P`) and uniform-random (`U`) on the
> `vrd/2` order (persistence `e`, actor-hours `d_H`, starvation deaths `d_S`, births `d_B`), at equal
> declared call ceiling `N = 256` physical attempts and equal declared decision budget `N − R_t = 248`
> (`R_t = 8`), with the no-AI control `A0` as the floor?

Ticket 39 answered nothing: one cell halted on the call ceiling and the scorer printed `NOT RUN`. This
ticket is the same matrix, once, at the revision where the schedule leaves the reserve unspent. It
declares no new threshold, floor, arm or cell.

## 2. Declared parameters (cited from ticket 38 §7.1 and ticket 40 §3, not re-declared)

| item | value |
|---|---|
| arms | `A0` (no-AI control, anchor, 0 calls), `P` (periodic), `U` (uniform-random), `A` (adaptive, `vs2-value/2`), `A1` (adaptive, `vs2-value/1`) |
| cell | `f10-s0` (founders 10, seed 0) |
| horizon | `H = 144` (samples 0..144) |
| replicates | 3 per arm → 15 scored cells (3 × `A0` + 12 model-bearing) |
| decision contract | `natural-claim/v4` |
| applied effort | `low`; `-max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900` |
| budget axis | declared call ceiling `N = 256` physical attempts per budgeted arm-run, of which the schedule selects at most `N − R_t = 248` decisions; `R_t = 8` is the declared transport reserve (`DeclaredTransportReserveAttempts`, ticket 40 §3) |
| concurrency | `-parallel 12` (see §2.1; the sole deviation from ticket 38 §7.1's declared `-parallel 8`) |
| `R` (full wake) | excluded, declared non-goal (ticket 38 §7.1) |

### 2.1 `-parallel 12`: the capacity reasoning, measured before the launch

Ticket 38 §7.1 declares `-parallel 8`, the value ticket 39 ran (44 min for the full matrix, §11.5). The
launch instruction for this run authorises 12 **if the machine plainly supports it**, and the measurement
says it does. This is a throughput knob, not a criterion: it changes how many cells are in flight, not any
arm's schedule, seed, decision budget, contract or band. Recorded as a deviation so it cannot be read as a
silent re-declaration of §7.1.

- `nproc` = **8**; `/proc/loadavg` at decision time `0.56 0.61 1.04`; `Mem available` **15 GB** of 22 GB;
  `/` has **384 GB** free.
- The work is **gateway-latency bound, not CPU bound**: one call costs 2.1–7.8 s of wall time
  (`latency_micros`, ticket 39 §11.5 ledgers) and a few ms of CPU. At `-parallel 8` ticket 39 measured
  load average **0.05–0.29 on 8 cores** (≤ 4 % of the machine) and runner RSS **220 MB** — i.e. 8 lanes
  used a twenty-fifth of the CPU and 1.5 % of the memory.
- 12 lanes therefore still leave ~25× CPU headroom and ~45× memory headroom (≈ 330 MB projected at 12
  lanes against 15 GB available), and the probe call measured 2.84 s (§6 gate 11).
- The gain is structural: the semaphore is per cell (`cmd/vs2/main.go:190-210`), so at 8 lanes the 12
  model-bearing cells run as 8 + 4 and the 4-cell second wave dominates the tail (ticket 39 §9.4 observed
  exactly that). At 12 all twelve start in one wave.
- **Accepted risk, stated:** 12 concurrent cells raise the concurrent load on the single gateway
  (ticket 39 measured 105–116 calls/min aggregate at 8 lanes). If the gateway throttles, per-lane latency
  rises and every lane slows together; a retried attempt then draws on the cell's `R_t = 8` reserve, which
  is exactly what the reserve exists for. Cells are independent (per-cell seeded RNG, per-cell account), so
  no cell's decisions depend on another's schedule.

## 3. The expected accounting state of every budgeted cell (declared before the run)

Every budgeted cell of this matrix reports `under_budget: true`, because the schedule now selects
`N − R_t = 248` decisions inside the unchanged `N = 256` attempt ceiling: the quota-bound cells (`U`, `A`,
`A1`) report `realized_calls = 248` with `budget_shortfall = 8`, and `P`'s cells report their own gate-bound
count (244 in run 1) with the matching shortfall (12), as they already did before the reserve. This is the
**expected** state of every budgeted cell, not a lost decision and not an alarm: 248 is the whole decision
budget the arms are compared on, 8 is the declared transport reserve left unspent for retries, and
`VS2-EC-2` counts `under-budget` as a diagnostic without failing on it. What alarms a cell is `halted`, or
`attempts − decisions > 8`.

## 4. Caps, and the stop rule

| item | value |
|---|---|
| hard cap, this run | **`$1.30`** (ticket 38 §8.1 as re-recorded by ticket 40 §7) |
| operator stop | realised spend **> `$1.04`** (80 % of `$1.30`), or the realised rate projecting above `$1.30` |
| attempts, cumulative across runs | **7 000**; ticket 39 used **3 020**, so ≤ **3 980** remain for this run |
| attempts, this run | ≤ 3 072 (`12 × 256`) by the harness ceiling; **2 976** expected (12 × 248) |
| expected spend | 2 976 × `$1.989e-4`/attempt (ticket 39 realised) = **≈ $0.59**; worst case at the ceiling **≈ $0.61** |
| wall clock | 3–4 h projected (ticket 38 §8.2), 6 h deadline, `-budget-min 720`; last realised matrix at `-parallel 8`: **44 min** |

The harness enforces calls, not dollars (`budgetFor` sets `MaxCostMicros: 1<<60`; the report cost is
`UNPRICED` because the gateway exposes no price), so the dollar cap is enforced by the operator poll rule:
every poll, spend is recomputed from every `calls.jsonl` `usage` row at `$0.10/M` input and `$0.50/M`
output, and the run is stopped **by its recorded pid only** (never `pkill -f`) if the stop rule trips.
A stopped run is reported **incomplete** and scored only if its artefact is the complete 15-cell matrix;
otherwise the verdict is `NOT RUN`.

## 5. The exact commands

### 5.1 Toolchain

```sh
. /home/z/vibe/agentworld/.tmp/go-env.sh     # go1.24.13, GOTOOLCHAIN=local
```

### 5.2 Zero-spend gates (recorded verbatim, §6)

```sh
go build ./...
go vet ./...
git ls-files -z '*.go' | xargs -0 gofmt -l
git diff --check ae0396f
go build -o .tmp/41-vs2-bin ./cmd/vs2
```

### 5.3 Evaluator gate (ticket 38 §7.6 gate 1)

```sh
python3 .scratch/38-m2c-preregistration/vrd2.py --self-test \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/41-m2c-matrix-rerun/evaluator-gate.log
```

### 5.4 Anchor gate (ticket 38 §7.6 gate 2, §8.4 step 3) and the curve step (§7.5, §8.4 step 7)

```sh
.tmp/41-vs2-bin -arms A0 -cells f10-s0 -reps 3 -hours 144 -replay=false \
  -claim-contract natural-claim/v4 -root .tmp/41-a0-runs \
  -out .tmp/41-a0-evaluation.json -out-txt .tmp/41-a0-evaluation.txt
# expect 3/3 replicates at H=1444 B=2 S=0 alive@143=2 alive@144=1, 0 calls

.tmp/41-vs2-bin -curve-in .tmp/36-v4-evaluation.json -curve-out .tmp/41-curve-gate.txt
```

### 5.5 Contract gate on recorded ledgers (ticket 38 §7.6 gate 3, exercised pre-launch; re-run on this run's ledgers after exit)

```sh
python3 .tmp/35-admis.py .tmp/36-v4-runs      # pre-launch, on the recorded v4 run
python3 .tmp/35-admis.py .tmp/41-m2c-runs     # post-run, on this run's ledgers (expect 0 inadmissible per arm)
```

### 5.6 Gateway probe (one call, real spend ≈ $0.00001)

`curl -s -m 300 -X POST http://127.0.0.1:4000/v1/responses -H 'Content-Type: application/json'
--data-binary @.tmp/41-probe-request.json` — the same wire/model/effort/`text.format` the harness sends.

### 5.7 The live matrix (spend authorised for this run only), detached

```sh
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/gpt-6-luna
setsid nohup .tmp/41-vs2-bin \
  -arms A0,P,U,A,A1 -cells f10-s0 -reps 3 -hours 144 \
  -effort low -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 12 -budget-min 720 \
  -root .tmp/41-m2c-runs \
  -out .tmp/41-m2c-evaluation.json -out-txt .tmp/41-m2c-evaluation.txt \
  > .tmp/41-m2c-run.log 2>&1 &
echo "runner pid $!"
```

**One invocation, one log, one run root, one directory and one ledger per arm-replicate.** The run root is
**fresh** (`.tmp/41-m2c-runs`, never `.tmp/39-m2c-runs`): `cmd/vs2/main.go:190` wipes the root at launch
(`os.RemoveAll(*root)`), and ticket 39's ledgers are the bound evidence of its `halt-evidence.json`
(`sha256 56d3b300…4df26` evaluation and the `.tmp/39-m2c-runs` root), so a shared root would destroy them.
Per-arm granularity is a subdirectory with its own `calls.jsonl`
(`.tmp/41-m2c-runs/<arm>-f10-s0-r{1,2,3}/`), which is what the liveness and spend polls read. The matrix
must be **one** invocation: the declared scorer `vrd2.py --live` takes a single evaluation JSON and keys
cells by `(arm, replicate)` (ticket 39 §3), so five per-arm invocations would produce five artefacts and no
declared way to score them.

### 5.8 Liveness (every poll: pid **and** ledger growth; never a pid file, never log content)

```sh
ps -p <pid> -o pid=,etime=,stat=
python3 .scratch/41-m2c-matrix-rerun/poll.py     # per-cell kind=="call" counts, tokens, spend
```

### 5.9 Post-run checks, in the order ticket 39 §9.6 declares

```sh
python3 .tmp/35-admis.py .tmp/41-m2c-runs                                             # contract gate
python3 .scratch/38-m2c-preregistration/vrd2.py --check-inventory .tmp/41-m2c-evaluation.json
python3 .scratch/38-m2c-preregistration/vrd2.py --live .tmp/41-m2c-evaluation.json \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/41-m2c-matrix-rerun/m2c-scores.log
.tmp/41-vs2-bin -curve-in .tmp/41-m2c-evaluation.json -curve-out .tmp/41-m2c-curve.txt
```

## 6. Gates recorded verbatim before the launch (all passed, all zero spend except the probe)

Raw transcripts beside this file: `gates.log` (build/vet/gofmt/diff-check/binary), `evaluator-gate.log`,
`anchor-gate.json`, `curve-and-contract-gates.log`, `gateway-probe.txt`. All at revision `ae0396f`.

| # | gate | command | result |
|---|---|---|---|
| 1 | build | `go build ./...` | exit 0 — **PASS** (go1.24.13 linux/amd64) |
| 2 | vet | `go vet ./...` | exit 0 — **PASS** |
| 3 | gofmt on tracked `.go` | `git ls-files -z '*.go' \| xargs -0 gofmt -l` | no output, exit 0 — **PASS** |
| 4 | whitespace/conflict vs base | `git diff --check ae0396f` | exit 0 — **PASS** |
| 5 | runner binary | `go build -o .tmp/41-vs2-bin ./cmd/vs2` | exit 0, 11 954 258 B — **PASS** |
| 6 | evaluator gate (§7.6 gate 1) | `vrd2.py --self-test --scores …/scores.json` | exit 0, T1–T7 PASS, `SELF-TEST: PASS` — **PASS** |
| 7 | anchor gate (§7.6 gate 2, §8.4 step 3) | `.tmp/41-vs2-bin -arms A0 … -reps 3` | exit 0 in 5.8 s; **3/3 identical replicates**, each `H = 1444, B = 2, S = 0`, alive@143 = 2, alive@144 = 1, `realized_calls = 0`, `halted = false`, identical `projection_hash 33b7692ad323…` (the same hash as ticket 39's anchor) — **PASS** (`anchor-gate.json`) |
| 8 | curve step (§7.5, §8.4 step 7) | `.tmp/41-vs2-bin -curve-in .tmp/36-v4-evaluation.json -curve-out .tmp/41-curve-gate.txt` | exit 0, 58-line report written — **PASS** |
| 9 | contract gate (§7.6 gate 3), pre-launch | `python3 .tmp/35-admis.py .tmp/36-v4-runs` | `R {'applied': 1323, 'rejected': 2, 'inadmissible': 0, 'malformed': 2, 'other': 0}` — the `inadmissible` count is **0** on the recorded `natural-claim/v4` run, and the same command is the gate re-run on this run's ledgers before any verdict — **PASS** |
| 10 | reserve, live in the artefact | the anchor run's recorded `quota_schedule` | 144 entries summing to **248** at `call_ceiling = 256` — the schedule the run will spend, measured before the launch — **PASS** |
| 11 | gateway reachability | §5.6 probe | `/health/liveliness` 200 in 1.7 ms; `/v1/models` 97 ids, declared model present; one call **HTTP 200 in 2.844 s**, `status: completed`, model echoed `github_copilot/gpt-6-luna`, usage 27 in / 12 out (≈ $0.000009), `output_text` a valid JSON object under `text.format=json_object` with `reasoning.effort=low` — **PASS** |

Two observations from the anchor report, **pre-existing and structural**, recorded so they are not read as
new or as regressions, and identical in kind to ticket 39 §4's:

- **`VS2-EC-1` FAILs**: `A0 cells checked=1 missing=0 declared=12; mismatches=f10-s0:alive_by_hour,
  f10-s0:event_body_bytes, f10-s0:events, f10-s0:history_bytes`. The four mismatches are the declared
  `H = 144` truncation ticket 35 already carries; the FAIL itself is the one-cell inventory. `VS2-EC-1` is
  satisfied only by the full 12-cell `A0` matrix, it is not one of the ticket-38 gates, and it does not gate
  the launch.
- **`VS2-EC-6` FAILs** with `Role A requests scanned=3 (recorded=0)`. The band's fail condition is
  `scan.recordedRoleA == 0` (`cmd/vs2/privacy.go:365-368`), so an `A0`-only anchor — which makes zero model
  calls by construction — can never satisfy it. On the full matrix the 12 model-bearing cells supply
  recorded Role A requests, and the band is read there. No violation, marker or key violation is reported.

## 7. Launch record

Launched. The pid cannot be recorded before it exists; nothing else was deferred.

| item | value |
|---|---|
| UTC launch time | **`2026-10-18T16:54:57Z`** (process `lstart` `Mon Oct 19 00:54:57 2026` at `+0800`; the host's UTC offset was read in the same minute, `date -u` `2026-10-18T16:55:06Z` vs `date` `2026-10-19T00:55:06+0800`) |
| runner pid | **`2759873`** — the `cmd/vs2` process itself, `sid = pgid = pid = 2759873` (`Ssl`, so it leads its new session and does not die with the launching shell), argv[0] `.tmp/41-vs2-bin`. Its parent `2759870` is the `/bin/bash -c` wrapper `setsid nohup` was invoked through, and it is not the runner: stopping this run means killing **2759873** and only that pid. |
| run root | `.tmp/41-m2c-runs` (fresh; one directory per arm-replicate) |
| log | `.tmp/41-m2c-run.log` (the report is written only at exit, so an empty log proves nothing) |
| evaluation artefact | `.tmp/41-m2c-evaluation.json`, `.tmp/41-m2c-evaluation.txt` |
| first observation, t+9 s | 12 cell directories, **11** with a `calls.jsonl`, 28 charged attempts, 17 634 in / 3 351 out tokens, $0.0034 — running, not queued-dead: `A-f10-s0-r1` is the twelfth and had not yet charged a call |

The polling record is `.scratch/41-m2c-matrix-rerun/poll-log.md`, committed as it is written.

## 8. Results — filled from the artefact, 2026-10-18T17:13Z

Per-cell rows are `kind == "call"` ledger rows (charged attempts), never decisions; `halted`,
`realized_calls`, `under_budget` and `budget_shortfall` are read from the evaluation JSON
(`.tmp/41-m2c-evaluation.json`, 775 911 B), not from the log. `attempts − decisions` is the `call` row count
minus the `role-a` row count in the same ledger.

| arm | cell | reps | charged calls (r1/r2/r3) | `realized_calls` (r1/r2/r3) | attempts − decisions | `halted` | `inadmissible` |
|---|---|---|---|---|---|---|---|
| A0 | f10-s0 | 3/3 | 0 (0/0/0) | 0 (0/0/0) | 0 | none | — (0 calls) |
| P | f10-s0 | 3/3 | 716 (244/244/228) — gate-bound, below the 248 decision budget | 244/244/228 | 0/0/0 | none | 0 |
| U | f10-s0 | 3/3 | 744 (248/248/248) | 248/248/248 | 0/0/0 | none | 0 |
| A | f10-s0 | 3/3 | 744 (248/248/248) | 248/248/248 | 0/0/0 | none | 0 |
| A1 | f10-s0 | 3/3 | 744 (248/248/248) | 248/248/248 | 0/0/0 | none | **1** (`r2`) |
| **total** | | 15/15 | **2 948 calls, $0.5696** | | **0** | **0** | **1** |

Tokens: 1 853 633 in / 768 547 out = `1 853 633 × $0.10/M + 768 547 × $0.50/M` = **$0.5696** (the report's own
cost field is `UNPRICED`; the gateway exposes no price). Attempts: **2 948** of the 3 980 remaining on the
7 000 cumulative operator cap; spend **$0.5696** against the `$1.04` operator stop and the `$1.30` hard cap
(44 % of the cap, and 4 % below the §4 expected `≈ $0.59`). Wall clock: launch `16:54:57Z` → exit between
`17:11:37Z` and `17:12:24Z` = **≈ 17 min** at `-parallel 12`, against ticket 39's 44 min at `-parallel 8`
and the declared 3–4 h projection.

### 8.1 The declared post-run checks, one by one

| # | check (§8.1) | measured | verdict |
|---|---|---|---|
| 1 | `attempts − decisions ≤ 8` per model-bearing cell | **0** in all 12 cells: 2 948 call rows and 2 948 `role-a` rows, so not one attempt was spent on a retry and the whole `R_t = 8` reserve went unspent in every cell | **PASS** |
| 2 | `realized_calls = 248` per quota-bound cell, `under_budget`, `budget_shortfall 8` | `U`, `A`, `A1`: 248/248/248 each, `under_budget = true`, `budget_shortfall = 8`; `P` gate-bound at 244/244/228 with shortfall 12/12/28 | **PASS** |
| 3 | 0 halted cells | `halted = false` in all 15 cell rows | **PASS** |
| 4 | inventory `ran == 15` | `cell inventory (spec section 8.4): PASS — 15 of 15 declared cells scored, 3 replicates per arm, 0 halted, 0 skipped` (exit 0) | **PASS** |
| 5 | 0 `inadmissible` per model-bearing arm (§7.6 gate 3) | `A {applied 742, rejected 2, inadmissible 0, malformed 2}`, `P {714, 2, 0, 2}`, `U {743, 1, 0, 1}`, **`A1 {740, 4, inadmissible 1, malformed 3}`** | **FAIL** — see §8.2 |
| 6 | spend and attempts against §4's caps | 2 948 attempts of 3 980 available; `$0.5696` of `$1.30`, never near the `$1.04` stop | **PASS** |

### 8.2 The one gate that fires: `A1-f10-s0-r2` records 1 `inadmissible` answer

Ticket 38 §7.6 gate 3, in the text that was fixed before any live call: *"Every model-bearing cell must
record **0 `inadmissible`** answers. A nonzero count voids that cell."*

The single event, from `A1-f10-s0-r2`'s ledger:

| field | value |
|---|---|
| ledger row | `seq 627`, `kind: "role-a"`, `cause: {actor: 8, hour: 139, wake_policy: vs2-A1, seam: natural.claim}` |
| the answer | decoded to `action: "eat_granary_food"` — an action name outside the `natural-claim/v4` vocabulary |
| the contract's response | `source: "fallback"`, `reason: "inadmissible"`, `fallback_reason: "inadmissible"`, `frozen_action: "gather"` — **rejected, not applied**; the decision ran on the frozen action |
| what it cost | the same 1 attempt as any other decision (that cell's `attempts − decisions = 0`), and 1 of its 248 decisions |
| `35-admis.py`'s classification | `proposed x gate: eat_granary_food illegal-action 1` — the same row: the legacy world gate would also have refused that action |
| the only other non-`ok` event in the matrix | 1 `http-error 400` (content policy) at `A-f10-s0-r2:243`, single attempt, no retry burst, no reserve consumed |

**This is a decision for the product owner, not a judgement this record may make**, because it decides
whether the pre-registered artefact may be scored at all. The measured facts it turns on:

- Run 1 (ticket 39) recorded **0** `inadmissible` on this same contract, with 8 `malformed` answers; this run
  records **1** `inadmissible` in 2 948 answers (0.034 %), against 8 `malformed`.
- The contract **rejected** the answer; it was not applied under a lossy contract, which is the pathology
  §7.6 gate 3 exists to catch ("measures admissibility luck, not allocation quality").
- §7.4's inventory precondition (the list of things that print `NOT RUN`) names a **skipped** cell and a
  **halted** cell, not a voided one; §7.6 gate 3 says "voids **that cell**" and does not say what a voided
  cell does to the verdict.
- The verdict is robust to the handling: `A1-f10-s0-r2` is the mild outlier of its arm
  (`H = 1434` against its siblings' 1443/1443), and dropping it changes **no** pair's verdict — `A vs A1`
  stays a TIE (`|0.0293 − 0.0007| = 0.0286 ≤ band 0.0293`), `A1 vs U` still separates, `A1 vs P` stays a TIE.
  **Neither primary pair (`A` vs `P`, `A` vs `U`) involves `A1` at all.**
- The three readings and what each records: **(A) strict** — the cell is void, so the declared inventory of
  15 scored cells at 3 replicates per arm is not met, the artefact is `NOT RUN`, and a fifth live run of the
  matrix (≈ $0.57) is the only way to a verdict; **(B) cell-level void** — `A1`-involving pairs are reported
  as not computable over a voided cell, the primary pairs are scored from complete cells, and the verdict
  below stands; **(C) recorded, not voiding** — the event is recorded as a contract rejection at 0.03 %, the
  full 15 cells are scored, and the verdict below stands.

`A vs P` and `A vs U` are unaffected by the choice; only the A1 pairs' *reportability* and whether a re-run
is owed differ. Until the reading is recorded in §9, **the recorded verdict is not yet fixed**.

### 8.3 What the scorer returns on this artefact (computed, §9 records the verdict)

The four declared post-run commands of §5.9 were run in order (`m2c-contract-gate.log`,
`m2c-inventory-gate.log`, `m2c-scores.log`, `m2c-curve.txt`). Per-arm bands, `vrd/2`:

| arm | tier 1 persistence | `d_H` (± half-width) | `d_S` (±) | `d_B` (±) |
|---|---|---|---|---|
| `A0` | persists 3/3 | 0.0000 (±0.0000) | 0.0000 (±0.0000) | 0.0000 (±0.0000) |
| `P` | persists 3/3 | 0.0210 (±0.0420) | 0.0333 (±0.0667) | 0.0000 (±0.0000) |
| `U` | persists 3/3 | 0.1837 (±0.0321) | 0.4333 (±0.1667) | 0.0000 (±0.0000) |
| `A` | persists 3/3 | 0.0293 (±0.0293) | 0.0667 (±0.0667) | 0.0000 (±0.0000) |
| `A1` | persists 3/3 | 0.0028 (±0.0042) | 0.0333 (±0.0667) | 0.0000 (±0.0000) |

Per pair (`|mu_X − mu_Y| > max(B, ε)` separates; `ε_H = 0.01`, `ε_S = 0.1`, `ε_B = 0.5`):

| pair | class | result | by how much | what decided it |
|---|---|---|---|---|
| `A` vs `P` | primary | **TIE** | actor-hours 0.0083, starvation deaths 0.0333, births 0.0000 | **band** (0.0420 and 0.0667) — both differences are far inside it |
| `A` vs `U` | primary | **SEPARATES**, more viable: `A` | actor-hours 0.1544 | **band** (0.0321) — 4.8× the band, and 15× the tier-2 floor |
| `A1` vs `P` | secondary | TIE | 0.0182 / 0.0000 / 0.0000 | band (0.0420 / 0.0667) |
| `A1` vs `U` | secondary | SEPARATES, more viable: `A1` | 0.1810 | band (0.0321) |
| `A` vs `A1` | secondary | TIE | 0.0265 / 0.0333 / 0.0000 | band (0.0293) |
| `P` vs `U` | secondary | SEPARATES, more viable: `P` | 0.1627 | band (0.0420) |
| `A0` vs each | diagnostic | `A0` vs `P`, `A0` vs `A`, `A0` vs `A1` TIE; `A0` vs `U` SEPARATES (`A0` better) | 0.1837 vs `U` | band (0.0321), flagged one-sided noise floor |

No tier-1 separation anywhere: **all five arms persist in all three replicates** (every `H` and every
replicate's terminal population > 0, §8.4). Tier 4 (births) ties on every pair: `d_B = 0.0000` for `A0`, `P`,
`U`, `A`, `A1` alike, so the 0.5 floor is never approached. Tier 3 (starvation deaths) does separate the three
`U` pairs (`U` 0.4333 against `P`/`A1` 0.0333 and `A` 0.0667: 0.4000 and 0.3667, both above
`max(0.1667, 0.1)`), but the rule reads the **highest** separating tier, and those pairs already separate at
tier 2, so tier 3 decides no pair. Within `{P, U, A, A1}` the pairs that do not separate anywhere are
`A` vs `P`, `A1` vs `P` and `A` vs `A1`.

Sensitivity, reported and not decisive (§7.5): the **six tier permutations change 0 of 36 declared-pair
verdicts**, and the **`h = H−1` terminal-sample variant changes 0 of 6**. No weight search, no re-tuning.

**Curve corroboration (§7.5, decisive for nothing): agrees with the tier order on both primary pairs** —
`A@low vs U@low` `DISTINGUISHABLE (population, SNR ≥ 2 with a ≥ 24-hour separation)` and
`A@low vs P@low` `NOT distinguishable at this noise floor`. It additionally calls `A1 vs P`, `A1 vs A`
(`granary_hist[0]`), `P vs U` and `A0` vs each distinguishable; the two instruments therefore agree on the
primary question and disagree on some secondary ones, which §7.5 already declares can happen and resolves in
favour of the tier order (the declared objective).

### 8.4 The per-replicate vectors behind the means

`H` = Σ population over samples 0..144, `B` = cumulative births at `H`, `S` = cumulative starvation deaths at
`H`; the `A0` control is `H = 1444, B = 2, S = 0` in all three replicates (§6 gate 7).

| arm | `H` (r1/r2/r3) | `B` | `S` |
|---|---|---|---|
| `A0` | 1444 / 1444 / 1444 | 2 / 2 / 2 | 0 / 0 / 0 |
| `P` | 1444 / 1444 / 1353 | 2 / 2 / 2 | 0 / 0 / 1 |
| `U` | 1178 / 1133 / 1225 | 2 / 2 / 4 | 3 / 4 / 6 |
| `A` | 1444 / 1370 / 1391 | 2 / 2 / 2 | 0 / 1 / 1 |
| `A1` | 1443 / 1434 / 1443 | 2 / 2 / 2 | 0 / **1** / 0 |

The `A1` `S = 1` replicate is `r2`, the cell carrying the inadmissible answer (§8.2).

### 8.1 The post-run checks this table is judged by (declared before the run)

1. **`attempts − decisions ≤ 8`** for every model-bearing cell — no cell may spend more of the ceiling on
   retries than the declared reserve.
2. **`realized_calls = 248`** for every quota-bound cell (`U`, `A`, `A1`), with `under_budget: true` and
   `budget_shortfall: 8`; `P`'s cells report their own gate-bound count (244 in run 1) and no halt.
3. **0 halted cells.**
4. **Inventory `ran == 15`** (`vrd2.py --check-inventory` exit 0: 15 of 15 declared cells, 3 replicates per
   arm, 0 skipped) — the precondition §7.4 puts before any verdict.
5. **0 `inadmissible`** answers per model-bearing arm (§7.6 gate 3).
6. Spend and attempts against §4's caps.

A failure of 1–5 means the artefact is a partial matrix: it is reported **incomplete** and the verdict is
`NOT RUN`, never a smaller experiment scored as a result.

## 9. Scoring, and the verdict

Run only after §8.1's six checks pass, in the order of §5.9, applying `vrd/2` **exactly as pre-declared**:
tier 1 by unanimity across the three replicates, tiers 2–4 iff `|mu_X − mu_Y| > max(B, ε_tier)` with
`ε_H = 0.01`, `ε_S = 0.1`, `ε_B = 0.5`, the wider of the two arms' envelope half-widths as `B`, and the
verdict alphabet `{SEPARATION, SECONDARY-ONLY, NO SEPARATION, NOT RUN}` with `A` vs `P` and `A` vs `U` as the
only primary pairs. No threshold, floor, tier order or weight may be adjusted after the numbers exist.

## 10. What this ticket may not conclude

Ticket 38 §7.7, unchanged, plus the verdict's own scope: one cell (`f10-s0`), one contract
(`natural-claim/v4`), one horizon (`H = 144`), 3 replicates, 248 decisions per budgeted arm. **Not the
central claim** — selective allocation beating uniform/periodic at equal budget is not established by one
cell; that needs the selective-vs-uniform comparison under a **spend** budget across cells and seeds with
the marginal value of reasoning measured, which is M2d/M4 territory. No cross-cell, cross-seed or
cross-fixture generality. No claim outside `vrd/2`'s order. No depth/effort claim (effort fixed at `low`),
no real-world fidelity claim, no re-opening of tickets 34–37, no override of the frozen EC-3/EC-4 rule. No
"better" language beyond "more viable under `vrd/2`". **`NOT RUN` is not a negative result.**

## 11. Evidence paths

| artefact | home |
|---|---|
| this record | `.scratch/41-m2c-matrix-rerun/spec.md` |
| zero-spend gate transcript | `.scratch/41-m2c-matrix-rerun/gates.log` |
| evaluator gate | `.scratch/41-m2c-matrix-rerun/evaluator-gate.log` |
| anchor gate vector | `.scratch/41-m2c-matrix-rerun/anchor-gate.json` |
| curve step + contract gate | `.scratch/41-m2c-matrix-rerun/curve-and-contract-gates.log` |
| gateway probe | `.scratch/41-m2c-matrix-rerun/gateway-probe.txt` |
| poll log and its script | `.scratch/41-m2c-matrix-rerun/poll-log.md`, `poll.py` |
| post-run contract gate, inventory gate, scorer, curve | filled in §12 when the run exits |
| run root (untracked; bundles + ledgers) | `.tmp/41-m2c-runs/` |

## 12. Post-run artefacts

Filled when the run exits: the contract-gate transcript, the inventory-gate transcript, the scorer's log,
the curve corroboration and the verdict, each beside this file.
