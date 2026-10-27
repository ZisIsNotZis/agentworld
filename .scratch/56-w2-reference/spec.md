# Ticket 56 — W2: `luna:max` always-on reference on the W1 world

- Record kind: one code change (the declared W1 world threaded through `cmd/vs2`), the A0 reproduction
  of the ticket-55 cell, an incomplete-by-environment `R@max` cost probe, and the claim-contract check.
  Workset `W2_reference_luna_max`.
- Author: worker, 2026-10-27.
- Question: can the `R@max` reference curve (every candidate every hour, no call ceiling) be recorded
  on the world W1 passed on, at a cost reported before scaling?
- **Verdict: the world and the code are ready and A0 reproduces exactly; the full `R@max` cell has not
  been recorded.** The live process was killed twice by the execution environment (not by a typed
  runner failure or the spend gate) after 47 recorded calls covering simulated hours 0–4. That partial
  cell is kept as the declared **cost probe**; the full-cell wall and spend are projections, below.
  A complete W2 reference cell is still outstanding.

## 1. What changed

Only `cmd/vs2`; no world mechanism, no strategy policy, no metric, no contract version.

- **One declared-world constructor.** `declaredWorld(f, seed)` (`cmd/vs2/config.go`) is the single home
  of the declared W1 world: mechanism `world.NaturalMechanismV14`, base policy
  `strategy.FrozenNaturalPolicy14()`, `Yield = 1`, `BirthCost = 2`, patches 2, `R = 8`, workers 1,
  `H = 144` (`DeclaredHorizonHours` was 168). The `fixture` struct now carries the world axes, so
  genesis, restore, branch, the model-call fixture identity and the evaluation all read one value;
  there are no `-yield`/`-birth-cost`/`-mechanism` flags that could desynchronise them.
- **Run identity.** The evaluation now records `yield` and `birth_cost` beside `base_policy`,
  `mechanism` and `horizon_hours`; the revision string names `base natural/14; mechanism v14; yield 1;
  birth-cost 2; H=144`. The model-call fixture identity is produced by the world itself
  (`naturalRunIdentity`) and therefore already carried mechanism 14; a test now pins its recorded
  request bytes.
- **Accepted-history replay.** `extractAccepted` replayed every cell's history under a hard-coded
  `NaturalMechanismV6` registry, so a v14 A0 run first failed in the evidence stage with
  `accepted history replay: event integrity failure`. It now uses the cell's own mechanism. This was
  the only runner-machinery divergence found; the world was not tuned.
- **Tests.** `TestDeclaredWorldA0ReproducesTicket55` pins the W1 cell through the runner's own genesis
  (neutral h0 pulse + births-enable branch) and step loop; `TestHermeticEndToEndReport` asserts the
  recorded world identity and that every recorded Role A request carries mechanism 14 / base ref
  natural/14; the quota-schedule shape test keeps the pre-W2 horizon 168 as an invariant-in-`H`
  regression.

## 2. A0 reproduction through the runner

Command (hermetic, no model, H=144 by default):

```
go run ./cmd/vs2 -arms A0 -cells f10-s0 -replay=false -budget-min 30 \
  -root .scratch/56-w2-reference/a0-runs \
  -out .scratch/56-w2-reference/a0-evaluation.json \
  -out-txt .scratch/56-w2-reference/a0-evaluation.txt
```

Reading, exactly the ticket-55 committed cell (`.scratch/55-w1-criterion/spec.md` §4):

| quantity | runner (`a0-evaluation.json`) | ticket 55 | equal |
|---|---|---|---|
| alive_end | 11 | 11 | yes |
| births | 12 | 12 | yes |
| starvation deaths | 1 | 1 | yes |
| age deaths | 10 | 10 | yes |
| balance Produced / Energy | 1145 / 184 | 1145 / 184 | yes |

No divergence: the runner's own machinery (h0 pulse, births-enable branch, step loop) reproduces the
world the ticket-55 sweep measured, and the balance closes (`10 − 10 − 1 + 12 = 11`). The evaluation
records `mechanism: 14`, `yield: 1`, `birth_cost: 2`, `horizon_hours: 144`. The `TestDeclaredWorldA0`
test repeats this assertion in CI (14.8 s).

## 3. Claim-contract check (`natural-claim/v4` vs v14 / natural-14)

**Compatible; no bump and no edit was made.**

- **Admissible action vocabulary.** The v4 vocabulary table has exactly four claim actions (`gather`,
  `eat-stored`, `provision`, `wait` → labels `take_slot_food_into_bag`, `eat_granary_food`,
  `take_granary_food_into_bag`, `wait_this_hour`). The v14 mechanism's legal set is
  `naturalClaimActions(v14)`; because `naturalHasProvisionLedger(v14)` is true (v14 has the yield
  axis), it is the same four actions. Every one has a declared display label, so the v4 encoder
  accepts them and the response ladder maps labels back to the same frozen tokens.
- **Fixture identity.** The v4 request writes `run.fixture = {patches, r, founders, mechanism, births}`,
  `run.base_ref` and `run.head_ref`, produced from the world. For the declared cell this is
  `mechanism: 14`, `base_ref: natural/14`, `founders: 10`, `patches: 2`, `r: 8`, `births: true`.
  The v4 key set is unchanged, so the request bytes remain a legal `natural-claim/v4` request; the
  live probe's recorded requests were accepted by the live gate and the test above checks the real
  recorded request bytes.
- **Latent gap, disclosed and not changed.** `cognition.FrozenNaturalClaimParameters` (and
  `protocol.FrozenNaturalClaimParameters`, which the task named) only knows `natural/7` and
  `natural/7b`; for `natural/14` it returns an empty parameter vector. This does not affect the
  declared v4 run because v4 deliberately carries **no** strategy parameter block (the head ref
  identifies the vector). It *would* matter for a `natural-claim/v1` or `/v2` run on natural/14,
  which would send an empty strategy block. Making the parameter contract learn natural/14 is a
  strategy/contract change and is left to the manager.
- **Not changed.** The privacy sentinel in `cmd/vs2/privacy.go` remains a synthetic encoder fixture
  under mechanism 6 / natural-7; it certifies the encoders' key sets, not a run's world. The run's own
  requests are scanned by the same band and carry mechanism 14.

## 4. The `R@max` cost probe (partial cell, 5 of 144 simulated hours)

ONE live cell was authorised; this is its only retained evidence. The first launch was cut at the
turn boundary (60 calls, directory overwritten), the second launch was cut by the execution
environment after 47 calls; `calls.jsonl` holds 94 hash-chained records (47 calls + 47 interventions)
and the run directory has no final evaluation because the process never reached its report phase.
**This is a cost/latency probe, not a completed cell and not the W2 reference curve.**

Declared command and bounds (max-effort necessities, not tuning):

```
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND go run ./cmd/vs2 \
  -arms R -cells f10-s0 -reps 1 -effort max -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 8 -budget-min 600 \
  -root .scratch/56-w2-reference/r-max-runs \
  -out .scratch/56-w2-reference/r-max-evaluation.json \
  -out-txt .scratch/56-w2-reference/r-max-evaluation.txt
```

- Attempt timeout 300 s and call deadline 900 s: the frozen 30 s/90 s defaults cut measured max-effort
  calls (ticket 35 §"Attempt timeout").
- Output bound 32768 tokens (ticket 35 decision 1); recorded-response bound 1 MiB (the 64 KiB hermetic
  bound marked ~30 % of max-effort calls `oversized`; ticket 35 §"Response bound").
- `-budget-min 600` is 600 minutes; the process did not hit it.

Recorded probe (`r-max-runs/R-f10-s0-r1/calls.jsonl`, per-call table `r-max-probe-calls.tsv`):

| quantity | reading |
|---|---|
| simulated hours covered | 0–4 (hours 0–3 complete at 10 calls each; hour 4 partial at 7 calls) |
| call records | 47, all `status=ok`, HTTP 200, zero retries/timeouts |
| decisions | 47, all `source=model` `action=gather`; 0 fallback, 0 inadmissible, 0 malformed |
| prompt / completion tokens | 29 571 / 20 079 (mean 629 / 427 per call) |
| latency per call | min 3.1 s, median 5.4 s, mean 6.9 s, max 22.7 s |
| latency by hour | h0–2 mean 4.9 s (max 8.1 s); h3–4 mean 10.5 s (max 22.7 s) |
| wall clock | first call start → last call end = 324.9 s |
| recorded price | `price_version=unpriced`, `cost_micros=0` |
| gateway-reported price | none: every response carries `usage.cost=null` and top-level `cost=null` |

There is **no per-hour KPI series** from the probe: no checkpoint/evaluation was written before the
kill, and the ledger records decisions, not world state. The recorded requests show every one of the
10 founders was escalated in each covered hour, which is the `R` full-wake property.

**Projection for the full cell (not a measurement).** The hermetic full-cell dry run (`-replay=false`,
no spend) made **1,434 calls** over 144 h; the probe observed 10 calls/hour, i.e. ≈1,440 calls. At the
probe's all-hours mean of 6.9 s/call that is **≈2.8 h**; at the h3–4 rate of 10.5 s/call (the
reasoning tokens and latency are still rising: completion-token mean 224 at h0 → 773 at h4) it is
**≈4.2 h**; the honest projection is **≈3–4.5 h** and could exceed 5 h if the late-hour trend holds.
The provider-side latency is the binding wall-clock cost, not the runner.

## 5. Price

The declared application model is unpriced: the recorder uses `cognition.UnpricedPrice{}`, every call
record has `price_version=unpriced` / `cost_micros=0`, and the gateway returns `cost=null` (it exposes
token batch/AIU counters only, no monetary field). **Recorded spend for the probe: $0.00.**

A price can be derived against the ticket-35 published gpt-6-luna rates ($0.10 per 1M input tokens,
$0.50 per 1M output tokens): probe cost = 29 571 × 0.1e-6 + 20 079 × 0.5e-6 = **≈$0.0130**. Projected
full cell at the probe's per-call token means (1,434 calls × 629 input, 427 output) = **≈$0.40**; if
the h4 completion-token rate (773) holds, **≈$0.55**. That is inside the one-matrix $1.10 cap at the
recorded rates; it is an estimate, not a gateway charge.

## 6. Limits

- **W2's acceptance is not met yet.** A recorded `R@max` cell with its full per-hour KPI series does
  not exist. What exists is a 47-call, 5-hour cost probe plus a projected wall/spend for the full cell.
  The falsifier ("cannot complete a cell inside the declared wall clock or exceeds the reported cap")
  was not observed; the obstruction was the execution environment killing the process at the turn
  boundary, twice, not a typed runner or spend failure.
- The probe covers hours 0–4 only; no demographic or wealth KPI can be read from it, and no curve can
  be emitted. The 1,434-call full-cell count is itself from a hermetic fallback run (v4 answers
  malformed under the v1-shaped fake), so it is a proxy for call count, not a live full-cell count.
- The probe answered `gather` every time; the world's `R` arm asks every actor every hour, and the v4
  request is accepted with zero refused answers. Whether a full live cell preserves that admissibility
  rate over 144 h is unmeasured.
- The model-call fixture identity carries mechanism/patches/`R`/founders/births and the base/head refs,
  but not `Yield`/`BirthCost`; adding them would change the `natural-claim/v4` request bytes (a
  contract decision, not taken). They are recorded in the evaluation, the revision string and the
  runner's journal/checkpoint identity.
- The runner's old bands remain pinned to the old world: `VS2-EC-1` compares A0 against the ticket-19
  v6 recorded evidence and therefore reports FAIL on the v14 world until that recorded fixture is
  regenerated; `VS2-EC-6` reports FAIL on an A0-only run because no Role A request exists to scan.
  Neither is a defect in this ticket's work; both are partial/old-evidence artefacts of the band set.
- The `natural/14` empty-parameter fallback noted in §3 is untested here and blocks any future v1/v2
  contract run on this world.

## 7. Reproducibility

Toolchain: `cd /home/z/vibe/agentworld && . .tmp/go-env.sh` (go1.24.13).

```
# A0 cell on the declared world (no model, no spend)
go run ./cmd/vs2 -arms A0 -cells f10-s0 -replay=false -budget-min 30 \
  -root .scratch/56-w2-reference/a0-runs \
  -out .scratch/56-w2-reference/a0-evaluation.json \
  -out-txt .scratch/56-w2-reference/a0-evaluation.txt

# the same cell through the ticket-55 sweep (independent reproduction)
go run ./.scratch/55-w1-criterion/sweep.go -f 10 -seed 0 -hours 144 \
  -policy natural/14 -yield 1 -birth-cost 2

# the probe's per-call table and tallies (no spend, reads the ledger)
python3 .scratch/56-w2-reference/summarize.py \
  --ledger .scratch/56-w2-reference/r-max-runs/R-f10-s0-r1/calls.jsonl

# changed-package checks
go build ./...
gofmt -l cmd/vs2
go test ./cmd/vs2 -count=1
```

The live cell command is in §4. Do not run it again without the manager's decision: the one-cell spend
authorisation is consumed, and the next attempt must be launched detached so it survives the turn.
