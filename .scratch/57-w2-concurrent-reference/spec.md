# Ticket 57 — W2: the `luna:max` reference cell, made feasible by concurrent Role A calls

- Record kind: one code change (concurrent, order-stable Role A decisions plus a ledger-resume
  recovery path), its hermetic determinism evidence, and the **full recorded `R@max` reference cell**
  on the declared W1 world with its per-hour KPI series. Workset `W2_reference_luna_max`.
- Author: worker, 2026-10-27. Predecessor: ticket 56 (the world threaded through `cmd/vs2`, the A0
  reproduction and the 47-call cost probe).
- Question: can the `R@max` reference cell (every candidate every hour, no call ceiling) finish in
  wall time on a world that runs, and does decoupling the in-hour call latency leave the recorded
  world unchanged?
- **Verdict: yes to both, and the reference is recorded.** The cell ran to `H = 144`: **1,579 Role A
  calls, 993,037 prompt + 1,061,251 completion tokens, 0 refused answers, 0 errors, 67.6 min of live
  wall clock** (4.65 h of summed call latency, so the calls overlapped ~5× on average), ending at
  **17 alive of 10 founders, 24 births, 7 starvation deaths, 10 age deaths**. The live ledger replays
  byte-exact with zero model calls (`R-f10-s0-r1: identical`). No world mechanism, policy, metric or
  contract changed.

## 1. What changed

Only the runner and the shared call path. No world rule, no strategy policy, no claim contract, no
metric.

- **The decide/apply split (the core).** `internal/world/natural_cognition.go`: the old
  `naturalSeamOverride` did decide+apply in one step. It is split into `naturalSeamDecide` (one
  recorded call, touches no world state, therefore safe to run concurrently) and `naturalSeamApply`
  (writes the actor's effective claim choice). `naturalSeamRequest` builds the complete request —
  observation, strategy, identity **and `FrozenAction`** — as a pure read. `naturalSeamOverride` is
  now decide-then-apply, so the serial path is unchanged.
- **`Natural.allocate` (`internal/world/natural.go`).** For an hour with `ClaimConcurrency > 1` it
  now: (1) evaluates every waking actor's frozen choice, (2) builds every selected actor's request,
  (3) decides the selected requests under a bounded worker pool, (4) **applies the responses serially
  in the existing wake order**. The claim/admission loop that follows consumes the same choices in the
  same order, so the gather allocator, the admissions and the committed events are untouched.
- **Scope: only the no-Priority wake policy.** `naturalClaimConcurrency()` returns 1 unless
  `f.cognition != nil && f.cognition.WakePolicy.Priority == nil && ClaimConcurrency >= 2`. The budgeted
  arms (`P`, `U`, `A`, which carry a non-nil `Priority` and an `EscalationSelector`) and the selector
  path therefore keep their byte-for-byte serial selection **and call sequence**; `A0` has no seam and
  makes no calls. Pinned by `TestNaturalClaimConcurrencySerialDefault`.
- **`NaturalCognition.ClaimConcurrency int`** is the declared bound (zero/one = serial; negative is a
  construction error). `cmd/vs2 -call-concurrency N` (default 1) sets it, and the applied value is
  recorded in the evaluation (`call_concurrency`) and in the revision string.
- **Shared call path made safe.** `CognitiveAccount`'s counters (`AllowCall`, `Charge`, `Hit`,
  `Totals`, `ObserveHour`, `Rows`) are mutex-guarded so a concurrent `Charge` cannot lose an increment
  and a crossing is recorded exactly once; `ReplaySeam.served` is atomic. The ledger already serialised
  its own writes. **No change** to pricing, ceilings, retry policy, ledger record shape or apply order.
- **Replay carries the recorded contract.** `cognition.NewReplaySeamForContract` re-encodes and
  re-validates under the run's declared claim contract. `NewReplaySeam` (v1) is unchanged for VS1; the
  v2/v4 paths were silently re-encoding v1 bytes, so every resolve would have been a ledger miss — the
  v4 reference cell could not have reconciled its journal or replayed. Found while wiring this ticket,
  fixed here.
- **The replay lane now replays the reference.** `runReplaySubset` skips only `A0`; `R` (nil Priority)
  is replayed beside the budgeted arms, because the W2 reference cell is now a recorded deliverable and
  its ledger-only replay is its determinism evidence.
- **`-resume <cell-dir>` (recovery path).** `cmd/vs2/resume.go` completes an interrupted live cell from
  its run directory: it truncates the ledger back to its last *complete* hour, replays the recorded
  prefix hermetically through `ReplaySeam` (zero model calls, zero spend) to the exact state the live
  process was in, continues live to `H` appending to the **same** hash-chained ledger, and publishes
  the ordinary evaluation with `resume_from_hour` naming the join. The cell's token/cost totals are
  summed from the whole ledger, never only the continuation. Unit-pinned by
  `TestCompleteLedgerHoursAndTruncate`.

**Rejected branches.**
- *Raise `-parallel` instead of concurrency*: `-parallel` is cells, not calls; there is one cell, and
  parallel cells would not shorten it.
- *Unbounded goroutines per actor*: refused; the pool width is the declared `N` and never more than
  the pending count (`naturalSeamDecideConcurrent`).
- *Parallelise the priority arms too*: refused. Their selection and spent-call count are online state
  owned by the selector; concurrency there would change the selection sequence, i.e. the experiment.
- *Apply responses in completion order*: refused — the world would change. That is exactly what the
  determinism test falsifies.
- *Re-create the world for the resume instead of replaying the ledger*: refused — the only record of
  the recorded hours is the ledger, and replaying it is the C12 guarantee, not a workaround.
- *A second live cell after the environment killed the first*: refused by the ticket's one-cell
  authorisation; the kill was repaired by resume, which spends only the missing hours.

## 2. Determinism proof (hermetic, no spend)

`internal/world/natural_claim_concurrency_test.go` (full-wake `R`-shaped fixture, 8 founders, 8
simulated hours, 50 ms fake latency, a per-actor scripted answer so a swapped response would change
the world):

| test | what it pins | result |
|---|---|---|
| `TestNaturalClaimConcurrencyDeterministic` | (a) the concurrent run's wall clock overlaps its calls — `concurrent < serial/2` for the same 64 calls at 50 ms each; (b) **journal bytes and every hourly checkpoint identical** to the serial run; every hour selects all 8 actors and the **per-actor-hour decision map is identical** (apply order pinned, no response swapped between actors); (c) a **ledger replay** (claim concurrency 3, a *different* concurrency) serves all 64 calls from the ledger with **zero model calls** and reproduces the same journal | PASS (17.8 s) |
| `TestNaturalClaimConcurrencyAboveSelectedSet` | a declared concurrency (64) larger than the selected count is safe and equals serial | PASS |
| `TestNaturalClaimConcurrencySerialDefault` | 0 and 1 mean serial; a priority-bearing policy stays serial even when 8 is declared | PASS |
| `TestNaturalClaimConcurrencyCeilingHaltsOnce` | a typed ceiling hit in a concurrent batch stops the step with `*CognitiveCeilingHitError`, is recorded exactly once and is sticky | PASS |

Run `-race` clean:

```
go test ./internal/world -run TestNaturalClaimConcurrency -race -count=1   # ok, 17.8s
```

Also re-run, unaffected by the change: `TestNaturalNilSeamDigestIdentity` (nil-seam digests unchanged),
`TestNaturalSeamInadmissibleOutputZeroMutation`, `TestNaturalSeamEscalatesSelectedHoursAndDiverges`,
`TestNaturalSeamReplayJournalAndCheckpoint`, `TestNaturalSeamOnlyCheckpointBindsLedger`,
`TestNaturalSeamRestoreWithoutReplayRefused`, `TestNaturalCognitiveCheckpointDivergingJournalRoundTrip`,
`TestNaturalPrioritySeamCheckpointReplayRoundTrip`, `TestNaturalNilPriorityReplaySelectorBytesIdentical`,
and in `cmd/vs2`: `TestHermeticEndToEndReport`, `TestReplaySubsetIdentical`,
`TestDeclaredWorldA0ReproducesTicket55`, `TestPriorityCheckpointReplayRoundTrip`,
`TestCompleteLedgerHoursAndTruncate`.

**Live-cell determinism.** Two independent full-size checks ran on the recorded cell itself: the
runner's journal reconciliation (`SaveResumeCheckpointWithReplaySelector`, which re-runs the whole
144 hours from the ledger and compares the scheduler bytes) completed, and the replay lane re-ran the
cell from genesis with a dead network transport and reported `R-f10-s0-r1: identical` (accepted
history, projection hash and history bytes), `replay_mismatches: none`.

## 3. The recorded `R@max` reference cell

Declared world (ticket 55, unchanged): mechanism **v14**, base policy **natural/14**, **Yield 1**,
**BirthCost 2**, patches 2, `R = 8`, workers 1, cell **f10-s0** (10 founders, seed 0), **H = 144**.

Command (one cell, live, spend authorised for this run only):

```
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND go run ./cmd/vs2 \
  -arms R -cells f10-s0 -reps 1 -effort max -call-concurrency 10 \
  -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 1 -budget-min 600 \
  -root .scratch/57-w2-concurrent-reference/runs2 \
  -out .scratch/57-w2-concurrent-reference/evaluation.json \
  -out-txt .scratch/57-w2-concurrent-reference/evaluation.txt
```

Run identity (verbatim in `evaluation.json.revision`): model `github_copilot/gpt-6-luna`, wire
`responses`, effort `max`, output bound 32768, response bound 1048576, contract `natural-claim/v4`,
votes 1, replicates 1, **call-concurrency 10**, base `natural/14`, mechanism v14, yield 1, birth-cost 2,
`H=144`, ceiling N=4096, cell `R-f10-s0-r1`.

| quantity | reading |
|---|---|
| Role A calls (`realized_calls`) | **1,579** — every candidate, every hour, no budget stop (`under_budget: false`) |
| decisions | 1,579 escalated = 1,579 applied model = 1,579; **0 fallback, 0 inadmissible, 0 decision errors**; 64 diverged from the frozen action |
| physical attempts | 1,579, all `attempt=1`, all `status=ok`, all HTTP 200; **0 retries, 0 timeouts, 0 oversized** |
| prompt / completion tokens | **993,037 / 1,061,251** |
| gateway price | `price_version=unpriced`, `cost_micros=0` (the gateway exposes `cost=null`); **derived** at the ticket-35 published rates ($0.10 / $0.50 per 1M): **$0.6299** |
| latency per call | min 2.49 s, **median 7.95 s**, mean 10.61 s, p95 23.21 s, max 218.51 s |
| live wall clock | **4,053 s = 67.6 min** (two segments: 3,551.5 s for 1,329 calls over hours 0–128; 501.6 s for 250 calls over hours 129–143) |
| summed call latency | 16,740 s = **4.65 h** — what a serial cell would have cost in wall time; the concurrent cell is **~1.1 h** |
| effective in-hour concurrency | mean **4.98** (summed latency / batch wall), max 8.8; e.g. hour 0: 10 calls in **9.0 s** wall against 55.5 s of latency |
| KPI identity | tip `4a5986f1…`, projection `ec2ae848…`, branch `ad0360fe…` |
| replay | `R-f10-s0-r1: identical`, zero model calls |

**KPI series** (all 144 hours in `kpi-series.tsv`, the machine series in
`evaluation.json.cells[0]`):

| hour | alive | births (cum) | starvation (cum) | age deaths (cum) | max capital |
|---|---|---|---|---|---|
| 0 | 10 | 0 | 0 | 0 | 0 |
| 24 | 10 | 0 | 0 | 0 | 1 |
| 48 | 10 | 0 | 0 | 0 | 1 |
| 72 | 10 | 0 | 0 | 0 | 1 |
| 96 | 10 | 0 | 0 | 0 | 1 |
| 100 | 9 | 0 | 0 | 1 | 1 |
| 104 | 8 | 0 | 0 | 2 | 1 |
| 106 | 9 | 1 | 0 | 2 | 2 |
| 112 | 13 | 6 | 0 | 3 | 2 |
| 120 | 13 | 11 | 1 | 7 | 2 |
| 128 | 13 | 15 | 4 | 8 | 1 |
| 136 | 16 | 21 | 6 | 9 | 1 |
| 144 | **17** | **24** | **7** | **10** | 1 |

Shape: a flat founding cohort to hour 99 (the founders all age out together from h100), a birth wave
from h106 that more than replaces them, and a small starvation drift from h120 while the population
grows past the founders' 10 to 17. Capital accumulation stays at the single build unit for most of the
run and reaches 2 from h106; the granary histogram ends `[8,3,1,2,3,0,0]` over seven stock bins. The balance closes: `10 − 10 − 7 + 24 = 17`. Per-actor coverage
Gini 0.51 (min 1 call, max 142) — newborns enter mid-run and are called from their own birth hour on,
which is the `R` full-wake property. Ages and starvation are the only death causes; no other death
cause appears in the series.

## 4. Limits

- **The ledger's record order is nondeterministic under concurrency.** Within an hour the calls and
  their interventions are appended in completion order. The **apply order** is the fixed wake order,
  and the world state, accepted history, checkpoints and journals are identical to the serial run
  (proved hermetically and by the full-cell replay). Replay resolves by request hash, not by ledger
  position, so the record order carries no meaning; a reader must not read it as the decision order.
- **A concurrent batch may cross the call ceiling by at most its own width.** `AllowCall` is a
  pre-spend guard, so with concurrency N up to N calls can pass before any `Charge` lands; every one is
  recorded and the typed hit still halts the run exactly once. No ceiling was crossed here
  (1,579 of 4,096). Pricing, ceilings and retry policy are otherwise unchanged.
- **The cell was completed in two live segments.** The execution environment killed the first segment
  after 1,329 calls (hours 0–128, 3,551.5 s) at a 60-minute tool window; a second attempt is outside
  this ticket's one-cell authorisation, so the missing hours were replayed from the ledger to the
  hour-129 boundary (zero model calls) and continued live. `resume_from_hour: 129` is recorded in the
  evaluation. The cell is one continuous hash-chained ledger and one continuous world; the resume is a
  recovery of the process, not a second run. An earlier attempt (before that) was killed at the turn
  boundary after 38 calls / 4 simulated hours; its directory was discarded as an incomplete duplicate.
- **The resume segment's own `CognitiveAccount` saw only the continuation's 250 calls.** The cell's
  reported token/cost totals are summed over the whole ledger, but a per-segment account view (e.g.
  `ceiling_rows`) is not continuous across a resume. For a reference arm with a 4,096 ceiling this is
  immaterial; a budgeted arm resumed this way would need the account seeded from the ledger first.
- **The price is derived, not charged.** The gateway exposes no price entry for this model; the
  recorder keeps the typed `unpriced` state and the token totals, and $0.6299 is arithmetic against
  the published ticket-35 rates.
- `-call-concurrency` is a runner scheduling parameter, not world identity: it is recorded in the
  evaluation and the revision string, and it deliberately does not enter the world journal or the
  `natural-claim/v4` request bytes (those carry the world, not the scheduler).
- Older bands still report not-evaluable on an `R`-only cell (`VS2-EC-1` needs A0 cells, `EC-2/3/4/7/8`
  need budgeted cells); that is the report working as declared, not a defect of this cell. `EC-5`,
  `EC-6` and `EC-9` pass on this run.
- The raw ledger is 60 MB (max-effort response bodies); it is committed gzipped (`calls.jsonl.gz`,
  38 MB) with `calls.tsv` as the human-auditable per-call table. The 8 MB final checkpoint bundle is
  not committed: it is reproducible by replaying the committed ledger (its state is bound by the
  tip/projection hashes in the evaluation).

## 5. Cost

- **Recorded spend: $0.00 charged** (unpriced model, `cost_micros=0`); **derived $0.6299** at
  $0.10 per 1M input and $0.50 per 1M output tokens over 993,037 / 1,061,251 tokens.
- Inside the W2 one-cell authorisation (≤ $1.10). The aborted 38-call attempt and the resume replay
  cost are included in the derived figure only insofar as their tokens are in the final ledger; the
  replay itself made **no** model calls.

## 6. Artefacts

| file | what it is |
|---|---|
| `evaluation.json` | the recorded evaluation: run identity, cell report, the full per-hour KPI series, bands, replay subset |
| `evaluation.txt` | the same report rendered |
| `kpi-series.tsv` | hour × (alive, births, starvation, age deaths, max capital, capital/granary histograms) |
| `calls.tsv` | one row per Role A call: seq, hour, actor, action, frozen action, source, status, http, attempt, latency, tokens, request hash |
| `calls.jsonl.gz` | the raw hash-chained ledger (1,579 calls + 1,579 interventions), gzipped |
| `summarize.py` | regenerates `calls.tsv` and `kpi-series.tsv` from the ledger and the evaluation |
| `runs2/R-f10-s0-r1/natural-root.bundle`, `natural-branch-enabled.bundle` | the declared genesis the cell ran from |

In the working tree but deliberately **not committed** (duplicates/derived, both reproducible from the
committed ledger): `runs2/R-f10-s0-r1/calls.jsonl` (60 MB uncompressed ledger; `calls.jsonl.gz` is the
committed copy), `runs2/R-f10-s0-r1/natural-final.bundle` (8 MB final checkpoint; its state is bound by
the tip/projection hashes), and `runs2/replay/` (the replay lane's transient copy of the same
responses). `git status` on a clean tree should show only these three.

## 7. Reproducibility

Toolchain: `cd /home/z/vibe/agentworld && . .tmp/go-env.sh` (go1.24.13).

```
# determinism (hermetic, no spend)
go test ./internal/world -run TestNaturalClaimConcurrency -count=1 -race

# the recorded ledger (needed by the replay lane and by the tables below)
gunzip -k .scratch/57-w2-concurrent-reference/calls.jsonl.gz
# -> calls.jsonl; the runner's replay lane reads <cell-dir>/calls.jsonl, so place it
#    at .scratch/57-w2-concurrent-reference/runs2/R-f10-s0-r1/calls.jsonl
#    (the run directory already holds it uncompressed in the working tree).

# The replay verdict recorded in the evaluation (replay_subset: ["R-f10-s0-r1"],
# replay_mismatches: []) was produced by the runner's own replay lane inside the
# recorded invocation: -replay defaults to true, so after the live cell the runner
# re-ran it from genesis through a ledger-only transport (dead network client) and
# compared accepted history, projection hash and history bytes.
# There is no standalone "-replay-only <cell-dir>" CLI flag yet: the lane runs in
# the same process as the cell. Reviewing the recorded verdict needs no run; to
# re-derive it, invoke the same runner command with the cell's ledger present and
# a replay-only transport (the plumbing replayOneCell uses), which this ticket did
# not expose as a flag - recorded as a gap, not as a hidden step.

# regenerate the two human tables from the recorded artefacts
python3 .scratch/57-w2-concurrent-reference/summarize.py \
  --ledger .scratch/57-w2-concurrent-reference/runs2/R-f10-s0-r1/calls.jsonl \
  --evaluation .scratch/57-w2-concurrent-reference/evaluation.json

# resume an interrupted live cell from its run directory (live, spend-gated)
go run ./cmd/vs2 -arms R -cells f10-s0 -reps 1 -effort max -call-concurrency 10 \
  -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 1 -budget-min 600 \
  -resume .scratch/57-w2-concurrent-reference/runs2/R-f10-s0-r1 \
  -root .scratch/57-w2-concurrent-reference/runs2 \
  -out <evaluation.json> -out-txt <evaluation.txt>

# changed-package checks
go build ./...
go test ./internal/world -run 'TestNaturalClaimConcurrency|TestNaturalSeam|TestNaturalNilSeam|TestNaturalPrioritySeamCheckpointReplayRoundTrip' -count=1
go test ./internal/cognition ./cmd/vs2 -count=1
```

Note on the full `./internal/world` suite: it is ~16 min on an idle machine and the machine was at
load ~16 (8 cores, other worktrees) during this ticket, so the full-suite gate is left to the manager;
the targeted tests above are the evidence for this change.
