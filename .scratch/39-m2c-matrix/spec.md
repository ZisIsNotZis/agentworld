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

Filled in by the second commit of this ticket (§9 records the order in which it was measured).

## 5. Run bookkeeping

- **UTC launch time:** recorded in §9 at launch.
- **Runner pid:** recorded in §9 at launch.
- **Run root:** `.tmp/39-m2c-runs` (one directory per arm-replicate, each with `calls.jsonl`).
- **Log:** `.tmp/39-m2c-run.log` (the report is written only at exit, so an empty or growing log proves
  nothing about liveness — ticket 38 §8.3).
- **Liveness = runner pid exists AND each arm's `calls.jsonl` grows.** Never a pid file, never log
  content.

## 6. Results (empty at launch; phase A fills only the bookkeeping rows)

| arm | cell | reps | realized calls at last poll | halted | `inadmissible` later verified by phase B |
|---|---|---|---|---|---|
| A0 | f10-s0 | 3 | — | — | — |
| P | f10-s0 | 3 | — | — | — |
| U | f10-s0 | 3 | — | — | — |
| A | f10-s0 | 3 | — | — | — |
| A1 | f10-s0 | 3 | — | — | — |

`vrd/2` keys, band/floor decisions, the verdict, the six-permutation sensitivity, the `h = H−1`
persistence variant and the curve corroboration are **phase B** outputs and are deliberately absent
here: this ticket launches the run, it does not score it.

## 7. What this ticket may not conclude

Everything in ticket 38 §7.7, unchanged: not the central claim (one cell, no spend-budget comparison, no
marginal-value-of-reasoning measurement), no cross-cell/cross-seed generality, no claim outside `vrd/2`'s
order, no depth/effort claim, no fidelity claim, no re-opening of tickets 34–37, and no "better"
language beyond "more viable under `vrd/2`". Phase A adds nothing to that list: **a launched run is not a
result.**

## 8. Stop and failure routes

- Any gate failure, or an unreachable gateway, means **do not launch**: the failure and its evidence are
  recorded and returned as a blocker.
- To stop a run, kill the recorded runner pid only. Never `pkill -f`.
- A partial matrix (a skipped, missing or `halted` cell) is reported as incomplete and never scored as a
  smaller experiment (§7.4 inventory precondition; §8.4 step 5b returns `NOT RUN`, exit 3).
