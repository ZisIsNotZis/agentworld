# Ticket 42 — M2d: replicate the M2c result at a second cell and a second seed

- Record kind: the pre-registration and run record of M2d. §1–§5 are the declaration and were committed
  **before any live call**; the gates (§6), the launch (§7) and the measured result (§8–§9) were added
  after, in that order, each in its own commit.
- Base revision: `d38a86b` (master, ticket 41 merged). Branch: `feature/42-m2d-replication`.
- Author: worker agent (ticket 42), under the advance-phase plan.
- Inputs, in order: the pre-registered criterion `.scratch/38-m2c-preregistration/spec.md` §7 (arms, cell,
  horizon, replicates, contract, KPI set, band + effect floors, verdict rule, falsifier, inventory
  precondition) and §8 (run plan, gates, detachment, caps), whose values this file **cites and does not
  re-declare**; the M2c result and its scope `.scratch/41-m2c-matrix-rerun/spec.md` §9–§10 — `A` vs `U`
  separates (`0.1544 > band 0.0321`), `A` vs `P` ties (`0.0083 ≤ max(0.0420, 0.01)`, no detectable effect
  at 3 replicates) — and the owning cell-void rule in its §9.2; the harness's declared cell list
  `cmd/vs2/config.go:249-272` (`f{10,11,12} × s{0,1,2,3}` primary, `f12 × s{4,5,6,7}` sensitivity-seed
  extension).
- What this ticket answers: **does the M2c result reproduce at a second cell and a second seed?** It is a
  replication of a declared result, not a new criterion, not a redesign of any policy, and not a test of
  the central claim.

## 1. Goal

Run the M2c matrix — unchanged arms, unchanged contract, unchanged budget axis, unchanged scorer — at
**two cells that M2c did not run**, and report for each cell whether the two declared primary pairs behave
as they did at `f10-s0`:

> Under `natural-claim/v4` at `f12-s0` and at `f10-s1`, horizon `H = 144`, does `A` (`vs2-value/2`)
> separate from uniform-random `U` on the `vrd/2` order (as it did at `f10-s0`), and does `A` fail to
> separate from periodic `P` on any tier (as it did at `f10-s0`)?

The M2c record states the tie is "no detectable effect at 3 replicates" and names replication as the way to
answer it (ticket 41 §9); the frozen VS2 rule's "in both cells" clause has so far been met only vacuously,
because every verdict after ticket 35's re-plan is single-cell (ticket 38 §7.7). M2d is that replication.

## 2. Declared parameters (cited from ticket 38 §7.1 and ticket 40 §3, not re-declared)

| item | value |
|---|---|
| arms | `A0` (no-AI control, anchor, 0 calls), `P` (periodic), `U` (uniform-random), `A` (adaptive, `vs2-value/2`), `A1` (adaptive, `vs2-value/1`) — the same five as M2c, so `A1`'s pairs (secondary) and `P` vs `U` are reported beside the primary pairs exactly as before |
| cells | **`f12-s0`** (founders 12, seed 0 — the second cell) and **`f10-s1`** (founders 10, seed 1 — the second seed); §2.1 states why that is the harness's own naming |
| horizon | `H = 144` (samples 0..144) |
| replicates | 3 per arm per cell → 15 scored cells per cell, 30 scored cells for the ticket |
| decision contract | `natural-claim/v4` (the M2a-passed, M2c-used contract) |
| applied effort | `low`; `-max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900` |
| budget axis | the declared call ceiling `N = 256` physical attempts per budgeted arm-run, of which the schedule selects at most `N − R_t = 248` decisions; `R_t = 8` is the declared transport reserve (`DeclaredTransportReserveAttempts`, ticket 40 §3) — unchanged |
| metric | `vrd/2` — the priority order (tier 1 persistence `e`, tier 2 actor-hours `d_H`, tier 3 starvation deaths `d_S`, tier 4 births `d_B`) against the `A0` anchor, exactly as ticket 38 §7.2 and `.scratch/37-fidelity-metric/spec.md` declare it, scored by the unchanged `.scratch/38-m2c-preregistration/vrd2.py` |
| per-tier effect floors | `ε_H = 0.01`, `ε_S = 0.1`, `ε_B = 0.5` (ticket 38 §7.4) — cited, not re-chosen |
| band rule | per arm and tier, the 3 replicates give a mean `mu` and the single-condition envelope half-width `max(envHi−mu, mu−envLo)`; tiers 2-4 separate iff `|mu_X − mu_Y| > max(B, ε_tier)` with `B` the wider of the two arms' half-widths; tier 1 separates only on unanimity (one arm extinct in all three replicates, the other persisting in all three) (ticket 38 §7.4) |
| verdict rule | an arm outranks another iff the highest separating tier is one where it is better; no separating tier = a tie. Per cell: **SEPARATION** iff `A` vs `P` or `A` vs `U` separates; **SECONDARY-ONLY** iff no primary pair separates but a secondary pair does; **NO SEPARATION** if nothing separates (ticket 38 §7.4) |
| `R` (full wake) | excluded, declared non-goal (ticket 38 §7.1); no `R` cell is run, so `VS2-EC-3`/`VS2-EC-4` remain `[not-evaluable]`, as in ticket 41 §6 |
| scorer | `.scratch/38-m2c-preregistration/vrd2.py`, **unmodified**; no threshold, floor, weight, arm, tier order or inventory constant of it is edited by this ticket |

**The criterion this ticket is judged by is the same one, evaluated at two new cells.** The only new
declaration M2d makes is the *replication* rule (§3) — how the two per-cell verdicts are combined into the
one sentence a reader wants — and the choice of which two cells (§2.1). No band, floor, tier, arm,
horizon, contract, effort or decision budget is re-tuned, and nothing that M2c tied is re-run in the hope of
a different answer: the M2c cells are **not** re-run at all.

### 2.1 The two cells, and why these two

The harness names a cell `f<founders>-s<seed>` (`cmd/vs2/main.go:373-382`); `-cells` selects any of them.
The declared primary list is `f{10,11,12} × s{0,1,2,3}` (`cmd/vs2/config.go:249-272`), and the declared
sensitivity-seed extension is `f12 × s{4,5,6,7}` (`config.go:256-263`, reached only through
`-sensitivity`). M2c ran `f10-s0`. This ticket therefore declares:

- **`f12-s0` — the second cell**, named in the stage's own question (`docs/VERTICAL-SLICES.md`, M2d row) and
  the declared second fixture: founders change, seed held at 0.
- **`f10-s1` — the second seed**, the declared primary cell that changes the seed and holds the founders at
  M2c's 10. Chosen over `f10-s2`/`f10-s3` for no reason beyond being the first declared seed after 0;
  chosen over the sensitivity-seed extension (`f12-s4..s7`) because that extension is declared for a
  sensitivity question at f = 12 and would move both axes at once.

One axis changes per cell, which is what makes the two experiments interpretable separately: `f12-s0`
varies founders, `f10-s1` varies the seed. Both are declared primary cells, so neither is an exploratory
fixture. Seed enters run identity (`declaredFixture` at `cmd/vs2/config.go:318`; the founder-lifetime
stream is seeded from it, `internal/world/natural.go:865`), so `f10-s1` is a genuinely different world and
not a relabelling of `f10-s0`.

### 2.2 Two invocations, one per cell — and why that is not an option

The declared scorer asserts a **15-cell inventory per artefact**: `inventory_problems` requires
`inventory.ran == 15`, exactly 3 scored replicates per arm, and keys every cell by `(arm, replicate)`
(`vrd2.py:428-470`, `cells_of` at `:405-424`). A single invocation over both cells would emit 30 cells and
be refused as `NOT RUN` ("30 scored cell(s) present, declared 15") — and its `(arm, replicate)` keying would
collapse the two cells' rows onto each other. So:

- **two invocations**, one per cell, **each with its own fresh run root, its own log and its own evaluation
  JSON**, each carrying the declared 15-cell inventory for its cell and each scored by the unchanged
  `vrd2.py --live` / `--check-inventory`;
- the per-cell artefacts are the evidence; there is no combined 30-cell artefact, and none is claimed.

This replaces ticket 41 §5.7's "one invocation" requirement, which existed because M2c ran one cell and the
scorer takes one artefact. Reusing the declared machinery unmodified is the binding constraint; one
invocation per cell is its consequence, not a deviation from it.

### 2.3 Concurrency, capacity reasoning and expected wall clock

Ticket 41 measured the whole 15-cell matrix at **≈17 min** with `-parallel 12` (12 model-bearing cells in
one wave), 2 948 calls, on this machine: `nproc` = 8, `Mem available` 15 GB, load average ≈0.29-0.56 at
launch and 1.31 at peak, runner RSS ≈220 MB, `/` with >300 GB free. The work is gateway-latency bound (one
call 2.1-7.8 s of wall time against a few ms of CPU), so lanes are cheap and the gateway is the real
resource.

Declared for this ticket: **two invocations run sequentially, each `-parallel 12`.** Expected wall clock
**≈17 min per cell, ≈35 min total**, with a 6 h deadline and `-budget-min 240` per invocation.

- **Not concurrent.** Two concurrent invocations at 12 lanes each would put 24 lanes on the single gateway,
  roughly doubling the measured aggregate of 105-116 calls/min at 8 lanes and ≈200/min at 12 (ticket 41
  §2.1, poll rows 1-3). That load is untested; throttling would raise per-lane latency and any retry
  draws on that cell's `R_t = 8` reserve — the reserve exists for exactly that, but burning it on an
  untested load is a spend risk this ticket does not need, since the sequential plan costs the same wall
  clock as two 6-lane waves would and keeps one pid, one stop rule and one log unambiguous.
- Concurrency is a throughput knob, not a criterion: it changes how many cells are in flight, never an
  arm's schedule, seed, decision budget, contract or band. Recorded as such.

### 2.4 The expected accounting state of every budgeted cell

Every budgeted cell reports `under_budget: true`: quota-bound arms (`U`, `A`, `A1`) select `248` decisions
inside the unchanged `256`-attempt ceiling and report `budget_shortfall: 8`; `P` reports its own gate-bound
count below the decision budget (M2c measured 244/244/228 at f10-s0; a different count at a new cell is
expected and is not a fault). What alarms a cell is `halted`, or `attempts − decisions > 8`.

## 3. The replication rule — declared before any live call

Two cells give two per-cell verdicts. Combined, as one of exactly three labels:

| label | condition |
|---|---|
| **REPLICATED** | **both** cells reproduce the M2c pattern: at each cell `A` vs `U` separates with `A` more viable, **and** `A` vs `P` is a tie on every tier |
| **PARTIALLY REPLICATED** | exactly **one** cell reproduces the M2c pattern, and at the other cell at least one of the two primary pairs behaves differently (the separation ties, or the tie separates) |
| **FAILED TO REPLICATE** | **neither** cell reproduces the M2c pattern |

Stated so the label cannot launder a disconfirmation:

- **Direction is part of the pattern.** A cell where `A` vs `U` separates but `U` is the more viable arm is
  **not** a reproduction, whatever the other cell does; it is reported per cell as a reversed separation and
  caps the overall label at PARTIALLY REPLICATED.
- **A separating `A` vs `P` is a failure of the null at that cell**, in either direction, and is reported
  with the tier, the statistic, the band and the floor that produced it — never as "closer to M2c's finding"
  because the pair separated at some tier rather than none.
- **A tie is reported as a tie with its power**, exactly as ticket 41 §9 requires: which tier came closest,
  the difference, the band, the floor, and whether the band or the floor decided it. "No detectable effect
  at 3 replicates" is the only reading a tie licenses here.
- Ties and separations are otherwise read from the scorer's own output, per cell, with no aggregation across
  cells, no pooling of replicates across cells and no reweighting: **two cells are two experiments**, and
  the ticket's claim is the pair of them, not an average.

## 4. The falsifier — what makes me report "the M2c result does not replicate"

I report **FAILED TO REPLICATE** if and only if, under §3, neither cell reproduces the M2c pattern. That is
a decisive, reportable outcome and not a ticket failure: the M2 stage's exit criterion states that a clean
negative is equally decisive ("the decision space does not yet reward reasoning"), and a failure here says
the M2c result is cell-specific or seed-specific until shown otherwise.

**The ways this could report a false replication** — the gate for each, declared now:

1. **Evaluator gate (before any live call is read).** `vrd2.py --self-test` must reproduce every committed
   `vrd/1` and `vrd/2` value, pass T1-T7, tie a synthetic copy of `A0` exactly, and — in `--expect-fail`
   mode — detect the inverted comparison as a FAIL. A scorer that cannot fail is not evidence.
2. **Anchor gate (zero spend, before the live matrix).** `A0` at **both** cells, `-reps 3`, zero calls, must
   produce **3 of 3 identical replicates per cell**. The control's vector at a cell is what `vrd/2` divides
   by; a drifting anchor voids the cell before any score is read. The anchor vectors are recorded so the
   live artefact's `A0` rows can be compared with them afterwards.
3. **Cell-shape gate (zero spend).** The four budgeted arms must run to `H = 144` in non-live (hermetic)
   mode at both cells without halting, so an unbuildable or ceiling-halting cell is found before spend.
4. **Contract gate.** Every model-bearing cell must record **0 `inadmissible`** answers. A nonzero count
   voids **that cell** under the owning rule (§5.2) — its arm's pairs at that cell are reported *not
   reportable over a voided cell*, its replicate count as used-of-declared, and the replication label is
   computed over the cells still assessable, stating how many were.
5. **Band gate.** No difference smaller than the §2 band — or, where the band collapses, smaller than that
   tier's declared floor — may be called a separation, in either direction. Ties print as ties.
6. **Inventory and halt gate.** A verdict is computed per cell only over an artefact carrying 15 scored
   cells (3 × `A0` + 12 model-bearing), exactly 3 replicates per arm, no skipped cell and no halted cell.
   Anything else prints `NOT RUN` for that cell, and a cell with no verdict cannot support a replication
   label; the ticket then reports the remaining cells and states the loss.

The same safe-failure discipline applies to my own reading: the two per-cell `vrd2.py --live` transcripts,
the contract gate and the inventory gate are the evidence, and every number in §9 is quoted from them.

## 5. Caps, stop rule, and the post-run checks

### 5.1 Caps

| item | value |
|---|---|
| hard cap, this ticket | **`$1.40`** (operator/supervisor declaration for ticket 42) |
| operator stop | realised spend **> `$1.12`** (80 % of the hard cap) |
| attempts, cumulative across runs | the 7 000 operator cap; 3 020 (ticket 39) + 2 948 (ticket 41) = 5 968 used, so ≤ **1 032** remain — **this run's declared 30 cells exceed that**, see below |
| attempts, this ticket | ≤ 6 144 (`24 × 256`) at the ceiling; 5 952 expected (`24 × 248`) |
| expected spend | measured at ticket 41: `$1.932e-4` per charged attempt → 5 952 × `$1.932e-4` = **≈ $1.15**; the per-cell-replicate spread measured there ($0.040-$0.061 over 12 cells) puts the plausible range at **$1.05-$1.30** |
| wall clock | ≈17 min per cell, ≈35 min total (§2.3), 6 h deadline, `-budget-min 240` per invocation |

**The cumulative-attempt cap and the spend caps conflict with the declared matrix, and that conflict is
recorded rather than resolved silently.** 1 032 attempts remain under the 7 000-attempt cap, but the
declared 30-cell matrix needs 5 952-6 144. And the expected spend `$1.15` exceeds the `$1.12` operator
stop while remaining inside the `$1.40` hard cap: at the measured rate the run reaches `$1.12` at
≈96 % of its own decisions, so applying the stop literally would halt the second cell near its end, void
its artefact as `NOT RUN`, and leave the ticket with one cell — after the spend was already sunk.

The resolution is the supervisor's, not this record's, and the escalation and its answer are recorded in
§5.3 before the launch.

The harness enforces calls, not dollars (`budgetFor` sets `MaxCostMicros: 1<<60`; the report cost is
`UNPRICED` because the gateway exposes no price), so both dollar caps are enforced by the operator poll
rule: every poll, spend is recomputed from every `calls.jsonl` `usage` row at `$0.10/M` input and
`$0.50/M` output, and the run is stopped **by its recorded pid only** (never `pkill -f`) if the stop rule
trips. A stopped run is reported **incomplete**, and scored only if its artefact is the complete 15-cell
matrix for its cell; otherwise that cell's verdict is `NOT RUN`.

### 5.2 The cell-void-versus-run-void rule (owning rule: ticket 41 §9.2, applied here unchanged)

- A **cell** with a nonzero `inadmissible` count is **voided**: its arm's pairs at that cell are reported as
  *not reportable over a voided cell*, its replicate count as used-of-declared (`2 of 3`), and no claim is
  made from it.
- A **run** (here: one cell's artefact) is **voided** — `NOT RUN`, no verdict — only when the §4 inventory
  precondition fails (a missing, skipped or incomplete cell) or the halt rule fails (a cell halted at its
  call ceiling).
- A voided cell is *not* re-run to replace it inside this ticket: re-running a cell until it is clean is
  the selection effect the pre-registration exists to prevent.

### 5.3 The spend escalation and its answer, recorded before the launch

Filled before the launch from the supervisor exchange (§5.1's conflict). No live call is made before this
section states the answer.

### 5.4 The post-run checks, in order (ticket 39 §9.6 / ticket 41 §5.9, per cell)

For each cell, on its own root and its own evaluation JSON:

```sh
python3 .tmp/35-admis.py .tmp/42-runs-<cell>                                  # contract gate
python3 .scratch/38-m2c-preregistration/vrd2.py --check-inventory .tmp/42-<cell>-evaluation.json
python3 .scratch/38-m2c-preregistration/vrd2.py --live .tmp/42-<cell>-evaluation.json \
  --scores .scratch/37-fidelity-metric/scores.json --log .scratch/42-m2d-replication/<cell>-scores.log
.tmp/42-vs2-bin -curve-in .tmp/42-<cell>-evaluation.json -curve-out .tmp/42-<cell>-curve.txt
```

The curve step is corroboration only (ticket 38 §7.5): if it disagrees with the tier order, both are
reported and the tier order stands.

### 5.5 The exact commands

Toolchain:

```sh
. /home/z/vibe/agentworld/.tmp/go-env.sh     # go1.24.13, GOTOOLCHAIN=local
```

Zero-spend gates (§6):

```sh
go build ./...
go vet ./...
git ls-files -z '*.go' | xargs -0 gofmt -l
git diff --check d38a86b
go build -o .tmp/42-vs2-bin ./cmd/vs2
python3 .scratch/38-m2c-preregistration/vrd2.py --self-test \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/42-m2d-replication/evaluator-gate.log
.tmp/42-vs2-bin -arms A0 -cells f12-s0,f10-s1 -reps 3 -hours 144 -replay=false \
  -claim-contract natural-claim/v4 -root .tmp/42-a0-runs \
  -out .tmp/42-a0-evaluation.json -out-txt .tmp/42-a0-evaluation.txt
.tmp/42-vs2-bin -arms P,U,A,A1 -cells f12-s0,f10-s1 -reps 1 -hours 144 -replay=false \
  -claim-contract natural-claim/v4 -root .tmp/42-hermetic-runs \
  -out .tmp/42-hermetic-evaluation.json -out-txt .tmp/42-hermetic-evaluation.txt
.tmp/42-vs2-bin -curve-in .tmp/36-v4-evaluation.json -curve-out .tmp/42-curve-gate.txt
python3 .tmp/35-admis.py .tmp/36-v4-runs       # pre-launch contract gate on the recorded v4 run
```

Gateway probe (one call, real spend ≈ $0.00001), the same wire/model/effort/`text.format` the harness
sends: `POST http://127.0.0.1:4000/v1/responses` with the recorded probe request.

The live matrix, one invocation per cell, detached and sequential:

```sh
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/gpt-6-luna
setsid nohup .tmp/42-vs2-bin \
  -arms A0,P,U,A,A1 -cells <cell> -reps 3 -hours 144 \
  -effort low -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 12 -budget-min 240 \
  -root .tmp/42-runs-<cell> \
  -out .tmp/42-<cell>-evaluation.json -out-txt .tmp/42-<cell>-evaluation.txt \
  > .tmp/42-<cell>-run.log 2>&1 &
echo "runner pid $!"
```

Fresh run root per cell (`cmd/vs2/main.go:190` wipes the root at launch), never a re-used root: ticket 41's
ledgers are the bound evidence of its record.

Liveness is the runner **pid** *and* per-cell **ledger growth**, never a pid file and never a log line (the
report is written only at exit). Polls are appended to `.scratch/42-m2d-replication/poll-log.md` and
committed as they are written.

## 6. Zero-spend gates, recorded verbatim before the launch

All at revision `5f3d0a3` (the declaration commit) on branch `feature/42-m2d-replication`, all zero spend
except gate 10 (one probe call, ≈$0.00001). Raw transcripts beside this file: `gates.log`,
`evaluator-gate.log`, `curve-and-contract-gates.log`, `gateway-probe.txt`.

| # | gate | command | result |
|---|---|---|---|
| 1 | build | `go build ./...` | exit 0 — **PASS** (go1.24.13 linux/amd64) |
| 2 | vet | `go vet ./...` | exit 0 — **PASS** |
| 3 | gofmt on tracked `.go` | `git ls-files -z '*.go' \| xargs -0 gofmt -l` | no output, exit 0 — **PASS** |
| 4 | whitespace/conflict vs base | `git diff --check d38a86b` | exit 0 — **PASS** |
| 5 | runner binary | `go build -o .tmp/42-vs2-bin ./cmd/vs2` | exit 0, 11 954 258 B — **PASS** |
| 6 | evaluator gate (§4 falsifier 1) | `vrd2.py --self-test --scores …/scores.json` | exit 0, T1–T7 PASS, `SELF-TEST: PASS` — **PASS** |
| 7 | anchor gate (§4 falsifier 2) | `.tmp/42-vs2-bin -arms A0 -cells f12-s0,f10-s1 -reps 3` | exit 0 in 5.4 s; **6 of 6 cells, 3/3 identical per cell**, `realized_calls = 0`, `halted = false` — **PASS**. `f12-s0`: `H = 1496, B = 2, S = 0`, alive@143 = 1, alive@144 = 1, `projection_hash 00d8c5f98777…` in all three replicates. `f10-s1`: `H = 1700, B = 5, S = 3`, alive@143 = 1, alive@144 = 1, `projection_hash e9b7c0f560d7…` in all three replicates. The two anchors differ from `f10-s0`'s `H = 1444, B = 2, S = 0` because the cells differ — that is the point of the ticket, and what must hold is determinism per cell, which does |
| 8 | cell-shape gate (§4 falsifier 3) | `.tmp/42-vs2-bin -arms P,U,A,A1 -cells f12-s0,f10-s1 -reps 1` (hermetic, `vs2-hermetic-fake/1`) | exit 0 in 1.8 s; 8 of 8 cells, each `realized_calls = 248`, `quota_sum = 248` at `call_ceiling = 256`, `under_budget = true`, `budget_shortfall = 8`, `halted = false` — **PASS**. Both cells build, both arms sets run to `H = 144`, no ceiling halt |
| 9 | curve step (§5.4, ticket 38 §7.5) | `.tmp/42-vs2-bin -curve-in .tmp/36-v4-evaluation.json -curve-out .tmp/42-curve-gate.txt` | exit 0, 58-line report — **PASS** |
| 10 | gateway reachability | §5.5 probe | `/health/liveliness` 200 in 1.6 ms; `/v1/models` 97 ids, declared model present; one call **HTTP 200 in 2.327 s**, `status: completed`, model echoed `github_copilot/gpt-6-luna`, usage 27 in / 12 out (≈ $0.000009) — **PASS** |
| 11 | contract gate, pre-launch (§4 falsifier 4) | `python3 .tmp/35-admis.py .tmp/36-v4-runs` | `R {'applied': 1323, 'rejected': 2, 'inadmissible': 0, 'malformed': 2, 'other': 0}` — **PASS**, and the same command is the gate re-run on this ticket's ledgers before any verdict |

Pre-existing report observations, recorded so they are not read as regressions of this ticket, all
identical in kind to ticket 41 §6: `VS2-EC-1` FAILs on an `A0`-only or partial artefact (it is satisfied
only by the full 12-cell `A0` matrix and is not a gate of this ticket), `VS2-EC-6` FAILs on an `A0`-only
anchor because the band's fail condition is `scan.recordedRoleA == 0` and an `A0` cell makes no model
calls, and `VS2-EC-3`/`VS2-EC-4` print `[not-evaluable]` on any matrix without an `R` cell, which this
arm set does not include (§2). None of the three is read by the verdict.

**No live call has been made when this table was written.** The one probe call of gate 10 is the only
charged request, and it is not part of the matrix.

## 7. Launch record (filled at launch)

Filled at launch.

## 8. Results (filled from the artefacts)

Filled after the run.

## 9. The replication verdict

Filled after the run.

## 10. What this ticket may not conclude

Ticket 38 §7.7 unchanged, plus the replication's own scope:

- **Not the central claim and not the M2 milestone.** Two cells, two seeds, 3 replicates, one contract,
  one horizon, one effort and at most 248 decisions per arm is still not "selective allocation beats
  uniform/periodic at equal budget". That comparison needs a **spend** budget across cells and seeds with the
  marginal value of reasoning measured (M2d's remaining work and M4).
- **No generality beyond the two cells run.** Nothing is claimed for `f11-*`, for seeds 2-3, for the
  sensitivity-seed extension, for other horizons, or for any fixture not run here. A replication at
  `f12-s0` and `f10-s1` says these two cells agree with `f10-s0`; it does not make the finding general.
- **No arm is called better than another** beyond "more viable under `vrd/2`" at these cells, and no "the
  policy is right" reading of a tie: a tie is no detectable effect at 3 replicates, with its power stated.
- **No A1 claim** if its cells are voided, and no re-opening of tickets 34-37, no override of the frozen
  EC-3/EC-4 dominance rule, no depth/effort claim (effort fixed at `low`), and no real-world fidelity claim.
