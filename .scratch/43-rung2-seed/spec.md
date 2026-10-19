# Ticket 43 — Rung 2 (seed): can the frozen world reach a good ending with no cognition?

- Record kind: the pre-registration and run record of rung 2. §1–§5 are declared **before any cell of
  this ticket is run**; §6 onward is appended after the sweep.
- Author: worker agent (ticket 43), reporting to the product owner.
- Inputs: `docs/VERTICAL-SLICES.md` (VS2 rung table; M2 stage table), `docs/VALIDATION.md` (`curve/1`,
  `vrd/2`, the committed-anchor caveat), `docs/OPEN-QUESTIONS.md` Q13 route 0, `docs/P0-SPEC.md`,
  `cmd/vs2` (runner and flags), `.scratch/42-m2d-replication/spec.md` §8–§9 (how `A0` was run and what
  the anchors are), `.scratch/37-fidelity-metric/scores.json` (committed anchor `H_T=1444`, `B_T=2`,
  `S_T=0`, `N0=10`).
- Question: does the world reach an ending worth protecting with **no** cognition at all? The rung's
  criterion is fixed by `docs/VERTICAL-SLICES.md` and is **not** relaxed here.

## 1. The criterion as a pass/fail predicate

For one cell (fixture `f` founders, seed `s`, horizon `H`), the runner's `A0` arm is the no-AI control:
`wireStack` returns `f.SetCognition(nil)` for `ArmA0` (`cmd/vs2/harness.go:103`), so the cell makes no
model call and spends nothing.

Let, from the cell's exported hourly series (`cmd/vs2/report.go`, `experiment.Projections`):

- `N0    = alive_by_hour[0]` (the founding cohort; a genesis sanity check requires `N0 == f`);
- `alive_end = alive_by_hour[H]` (the terminal sample, sample index `H`);
- `births = births_cum[H]`;
- `starve = deaths_starve_cum[H]`.

Then the rung-2 criterion is the predicate

```
PASS_cell  :=  (alive_end >= N0)  AND  (starve == 0)  AND  (births >= 1)
```

exactly the declared wording "alive at `H` at least the founding cohort, zero starvation deaths, at least
one birth". A cell passes only when all three hold; the ticket verdict is **PASS** iff at least one cell
in the declared grid passes, and **FAIL** otherwise. The passing fixture and seed are named verbatim.

## 2. The declared grid (fixed before any result is read)

The world's own admissible founder range is `1 <= f <= 8*P = 16` (`internal/world/natural.go:573`,
`NaturalFoundersValid`; `P = 2` here), and the declared primary seeds are `0..3` with `f = 12` extended
by the declared sensitivity seeds `4..7` (`cmd/vs2/config.go:249-263`). This ticket declares a sweep that
**exhausts the founder axis over the union of the declared seed sets**, so no admissible founding cohort
is left untested and no fixture is picked by eye:

| axis | declared values | count |
|---|---|---|
| founders `f` | `1, 2, 3, ..., 16` (every admissible value) | 16 |
| seeds `s` | `0, 1, 2, 3, 4, 5, 6, 7` (primary `0..3` + declared sensitivity `4..7`) | 8 |
| horizon `H` | `144` (held equal to the recorded cells for comparability) | 1 |

**Declared grid: 16 x 8 = 128 cells.** It contains, as a strict subset, the mandatory primary cells
`f ∈ {10,11,12} × s ∈ {0,1,2,3}` (12 cells). `f = 1` cannot satisfy the criterion (reproduction needs
two parents) but is included because the grid is declared by the world's admissible range, not by a
guess about which cells can pass.

Compute estimate: `A0` at `f = 10..12` measured 6.5 s per cell at `H = 168` (`.tmp/30-vs2-evaluation.json`,
`duration_ms = 6552`), and the 16-founder cells measured 0.05–0.08 s per simulated hour (ticket 15).
At `H = 144` this is at most ≈7 s per cell; 128 cells at `-parallel 8` is ≈2 min, far inside the 90-minute
compute bound. If a cell proves slower, the grid is not silently truncated: the run logs which cells were
skipped by the wall-clock budget and that result is reported as an incomplete inventory.

## 3. The proof method

1. **Zero model spend.** Run the declared grid with `-arms A0` only, no `-live`. The evaluation artefact
   is then required to show, for every cell, `realized_calls == 0`, `prompt_tokens == 0`,
   `completion_tokens == 0` and `cost_micros == 0`. No gateway endpoint is contacted
   (`authorizeLive(false)` returns nil without live transport; `A0` holds no ledger at all). This is the
   zero-spend proof, read from the runner's own accounting, not asserted.
2. **Agreement with the recorded cells.** The same run must reproduce the recorded `A0` anchors at the
   three already-scored cells: `f10-s0` actor-hours 1 444, births 2, starvation 0, `alive_end` 1 of 10;
   `f12-s0` 1 700 / 5 / 3, 1 of 12; `f10-s1` 1 496 / 2 / 0, 1 of 10. A mismatch voids the sweep before
   any verdict is read.
3. **Per-cell verdicts.** Compute the §1 predicate for all 128 cells from the exported series.
4. **Binding-parameter analysis if FAIL.** Measure, per cell, the death composition
   (`deaths_age_cum[H]`, `deaths_starve_cum[H]`), the birth timing (first and last hour at which
   `births_cum` increases), the peak and terminal population, and the variation of `alive_end` with `f`.
   Name the mechanism the measurements point to — founder aging, birth rate/economics, food supply, or
   founding cohort size — and quote the numbers; no mechanism is asserted without its measured evidence.
5. **No Go source change.** This ticket is measurement only. It adds no flag and changes no world
   parameter. If the runner lacked a needed flag the report would be a blocker, not an implementation.

## 4. Environment prerequisite (no Go toolchain on this machine)

The repository has no system Go. Installed a user-local Go `>= 1.24` without `sudo`:

```
curl -fL -o /tmp/go1.24.5.linux-amd64.tar.gz https://go.dev/dl/go1.24.5.linux-amd64.tar.gz
tar -xzf /tmp/go1.24.5.linux-amd64.tar.gz -C /home/z/.local
export PATH=/home/z/.local/go/bin:$PATH
```

- Installed version: **go1.24.5 linux/amd64**
- Install path: **/home/z/.local/go** (binary `/home/z/.local/go/bin/go`); nothing written under
  `/usr/local` or any system directory; no `sudo`.
- Verified: `go version`; `go build ./cmd/vs2`; `go vet ./cmd/vs2 ./internal/...` (recorded with their
  output in §6, including anything that did not finish).

## 5. Exclusions (declared)

No live model call and no spend; no world-parameter change; no fixture chosen after seeing results; no
edit to the rung 1/3/4/5 status cells; no rewrite of recorded M2 rows; no new ecology mechanism; no touch
to `.scratch/41-*` or `.scratch/42-*`; no Go source change anywhere. Raw runner output is reduced to one
small aggregate artefact plus a command log under this ticket directory; full runner roots stay in the
git-ignored `.tmp` and are not claimed as durable evidence.

## 6. Run record

### 6.1 Environment and builds

- `go version` → **go version go1.24.5 linux/amd64** (user-local, `/home/z/.local/go/bin/go`).
- `go build ./cmd/vs2` → **exit 0**, real 12.1 s.
- `go vet ./cmd/vs2 ./internal/...` → **exit 0**, real 7.7 s (completed; no output).
- No Go source file was created, edited or deleted. `git status` showed no change under `cmd/` or
  `internal/`.

### 6.2 The declared sweep ran completely, at zero spend

The command (full text in `commands.txt`) ran `-arms A0` only, no `-live`, `-reps 1`, `-hours 144`,
`-parallel 8`, `-budget-min 60` (stricter than the ticket's 90-minute compute bound), over the 128 declared
cells. The evaluation artefact records:

| check | value |
|---|---|
| inventory | `declared = 128`, `ran = 128`, `skipped = none` |
| halted cells | 0 |
| cells with `realized_calls != 0`, or nonzero prompt/completion tokens, or nonzero `cost_micros` | **0 of 128** |
| arms in the artefact | `{"A0"}` only |
| live transport | none (`-live` absent; `authorizeLive(false)` returns before any gateway config) |

**Zero model calls and zero spend are therefore proven by the runner's own accounting**: every cell is
`A0`, and `A0` attaches no cognition (`cmd/vs2/harness.go:103` returns `f.SetCognition(nil)`), so there is
no ledger, no client and no call path. The arms column is `A0` alone, so no other arm can have spent.

### 6.3 Reproduction of the recorded anchors

The same run reproduces the three already-scored `A0` cells exactly, which validates the horizon, the
fixture and the binary against the recorded evidence before any verdict is read:

| cell | recorded `(actor−hours, births, starvation, alive_end)` | this run | match |
|---|---|---|---|
| `f10-s0` | `(1444, 2, 0, 1 of 10)` | `(1444, 2, 0, 1 of 10)` | yes |
| `f12-s0` | `(1700, 5, 3, 1 of 12)` | `(1700, 5, 3, 1 of 12)` | yes |
| `f10-s1` | `(1496, 2, 0, 1 of 10)` | `(1496, 2, 0, 1 of 10)` | yes |

### 6.4 Per-cell result — the 12 declared primary cells

`PASS` is the §1 predicate. Full 128-row table: `a0-grid.tsv`; machine-readable: `a0-grid.json`.

| f | seed | H | N0 | births | starvation deaths | alive at H | age deaths | verdict |
|---|---|---|---|---|---|---|---|---|
| 10 | 0 | 144 | 10 | 2 | 0 | 1 | 11 | **FAIL** (`alive_end<N0`) |
| 10 | 1 | 144 | 10 | 2 | 0 | 1 | 11 | **FAIL** (`alive_end<N0`) |
| 10 | 2 | 144 | 10 | 2 | 0 | 0 | 12 | **FAIL** (`alive_end<N0`) |
| 10 | 3 | 144 | 10 | 2 | 0 | 0 | 12 | **FAIL** (`alive_end<N0`) |
| 11 | 0 | 144 | 11 | 3 | 2 | 0 | 12 | **FAIL** |
| 11 | 1 | 144 | 11 | 3 | 1 | 1 | 12 | **FAIL** |
| 11 | 2 | 144 | 11 | 3 | 1 | 0 | 13 | **FAIL** |
| 11 | 3 | 144 | 11 | 3 | 1 | 0 | 13 | **FAIL** |
| 12 | 0 | 144 | 12 | 5 | 3 | 1 | 13 | **FAIL** |
| 12 | 1 | 144 | 12 | 4 | 2 | 0 | 14 | **FAIL** |
| 12 | 2 | 144 | 12 | 4 | 2 | 0 | 14 | **FAIL** |
| 12 | 3 | 144 | 12 | 4 | 2 | 0 | 14 | **FAIL** |

Across the **whole 128-cell grid**, **0 cells pass**. Clause failures (a cell can fail more than one):
`alive_end < N0` in **127/128**; `starvation deaths > 0` in **80/128**; `births < 1` in **34/128**. The only
cell that reaches `alive_end >= N0` is `f6-s6` (`alive_end = 6 = N0`, 9 births) and it fails with **9
starvation deaths**. `alive_end` never exceeds 6 anywhere in the grid.

### 6.5 Verdict

**FAIL — no declared fixture and seed gives the no-AI control a good ending. Rung 2 does not hold.**

The criterion needs all three clauses at once. No cell holds the first (`alive_end >= N0`) together with
zero starvation, and no cell holds all three. The grid exhausted the world's own admissible founder range
(`1..16`) at every declared primary and declared sensitivity seed (`0..7`) at `H = 144`.

### 6.6 Binding-parameter analysis (measured, not asserted)

**(1) Founder aging is the primary binder.** In the declared primary cells the founding cohort and its
few offspring die mostly of **age**, not hunger: `age_deaths` are 11–14 per cell and account for **81.2 %–100 %**
of all deaths (`f10-s0` and `f10-s1`: 11 of 11, 100 %). Age deaths begin at **h99–h103** and the last
primary-cell age death is at **h138–h144** (`a0-grid.tsv`: `f10-s2` 138, `f10-s0` and `f11-s0` 144).
`births` begin at **h5** and recur as late as **h68** in the primary cells (`f11-s1`; h25 at `f11-s0`,
h23 at `f12-s0`; the f=10 cells are all `[5,6]`, f=11 seeds 0–3 are `[5,7,25]`, `[5,7,68]`, `[5,7,9]`,
`[5,7,8]`, f=12 is `[5,6,7,8,23]` at s0 and `[5,6,7,8]` at s1–3), and across the whole grid they recur as
late as **h124** (`f6-s6`; h117 at `f12-s6`). In the `f=10` cells, where there is a survivor it is the
latest-born (11 age deaths, no starvation deaths); the aggregate artefact records no per-actor identity at
the starvation cells, so nothing here establishes which actor survives there. The horizon is about one
lifespan: **age deaths run h99–h144 in the primary cells (h97 grid-wide) and leave one survivor or none
there**.
*Counterfactual, from the same measured series:* at `f10-s0` the other two clauses already hold (2 births,
0 starvation), and the cell's peak population is 12 at h6 with 11 age deaths and `alive_end = 1`; with age
mortality removed the same trajectory would end with all **12** actors alive (`10 + 2 − 0`) and would
**PASS**. So at the recorded primary founders, founder aging is the clause that binds.

**(2) Birth rate/economics is the second binder, and it is independent.** Reproduction is front-loaded and
far below replacement: at the primary cells `births / N0 = 0.20 / 0.27 / 0.33–0.42`, against the ≥ 1.0
needed for a cohort to replace itself; across the grid 34/128 cells produce **zero** births (`f=1, 8, 9,
16` for every seed, and some `f7`/`f15` seeds), and at those cells no amount of aging relief would pass
the `births >= 1` clause. `f=16` produces 0 births and `alive_end = 0` (16 age deaths); larger founding
cohorts therefore do **not** buy a good ending.

**(3) Food supply is a co-binder at larger cohorts, not the primary one.** Starvation deaths occur in
80/128 cells, 280 of the grid's 1 290 deaths (21.7 %; age deaths 1 010, 78.3 %), and rise with foundering
(`f≥5` shows 0–9 starvation deaths, the minimum 0 at `f = 8, 9, 10, 16` and 3–5 at `f = 5`). But at `f=10` starvation is **0** in all eight seeds and the cells
still fail on age and births, so food is not what blocks the declared primary cells.

**(4) Founding cohort size does not bind and does not help.** `alive_end` is non-monotonic and never
reaches `N0` for `f>1`: `f=8, 9, 16` end at 0 with zero births; `f≥10` end at 0–3; the grid's single
`alive_end >= N0` cell (`f6-s6`) is a starvation-collapse exception. Increasing the founding band moves
the outcome between age-only die-off and starvation die-off; it never produces replacement.

Summary: the world cannot reach a good ending with no cognition because **the founding cohort ages out
inside the horizon while reproduction is too front-loaded and too low to replace it**, with food supply
turning that collapse into starvation at larger cohorts. The binding order is founder aging, then birth
economics, then food supply; founder count is not a lever.

### 6.7 Durable artefacts

| file | content | sha256 |
|---|---|---|
| `a0-grid.json` | 128 per-cell rows (all fields above + zero-spend accounting) and the grid summary, reduced from the 6.0 MB runner evaluation | `056ad80c22da7d260c24cb237065d9b56665bca3762c71d3bf3301f0e2a859a9` |
| `a0-grid.tsv` | the same 128 rows as a flat table (`f`, `seed`, `horizon`, `N0`, `alive_end`, `births`, `starvation_deaths`, `age_deaths`, `actor_hours`, peak, birth/death hours, verdict) | `0e0f8f165ac14d8d1b0734fc2613dcca7a2f8e50fc75b17ef345e0c49eb10e9c` |
| `runner-report.txt` | the runner's own text report (25.6 KB) | `8cdef925be129a31f6216da51e011032ac688fe7fc618f85ae8b5c79118133cb` |
| `commands.txt` | exact toolchain, build, vet and run commands | — |
| `reduce-a0-grid.py` | the reduction script (source evaluation → the two aggregate artefacts) | — |

The source evaluation artefact was `.tmp/43-a0-evaluation.json`, sha256
`16d6c2e843f86fbe44e3407eb19135af81fbb17c1a70a65949ced68ae311524b`, 6 308 883 bytes, 128 cells;
`a0-grid.json` records that hash and is a strict field subset, not a transcription. The full runner roots
(7.4 MB) stay in the git-ignored `.tmp/43-a0-runs/`; no fact of this ticket depends on them.

### 6.8 Residual risks and limits

- The verdict is at the frozen world, `H = 144`, seeds 0–7. It does not bound a different substrate, a
  longer horizon, or a world change; the rung's criterion is the declared one.
- `-reps 1` was used because `A0` is deterministic and the three recorded anchors reproduced exactly
  3/3, matching ticket 42's anchor gate. The grid therefore has no replicate noise, by construction.
- The runner's `VS2-EC-1` band printed FAIL in this artefact because it compares against ticket-19
  recordings at the 12 declared cells and at `H = 168`; that is an artefact-of-horizon mismatch, not a
  defect in the sweep, and the rung-2 verdict reads only the per-cell series.
- This ticket is measurement only: no flag was missing, so no blocker was raised and no code changed.

### 6.9 Independent review and the checks it could not run

A fresh independent reviewer examined the merge at commit `03b0467` and returned **BLOCK** on the record,
not on the measurement: the FAIL verdict, 0/128 cells, zero spend and binding analysis all reproduced, and
it found two P1 record defects (a birth/age-hour sentence contradicted by this ticket's own `a0-grid.json`,
and a `commands.txt` that runs a binary it never builds). Its report is copied verbatim, with its source
named, to [`review.md`](review.md); the corrections it asked for are in this commit and the verdict is
unchanged. A delta review of the correction commit `44363f4` returned **OK with notes** with no verdict
change; its report is copied verbatim, with its source named, to [`review-delta.md`](review-delta.md), and
its four report-only items are landed in the same commit. Its suggested figures were re-derived from `a0-grid.json` and extended where the artefact
differs: births begin at h5 and the grid's latest birth is h124 (`f6-s6`), not h117. The reviewer had no shell, so it could not run three checks; they were run here and all three
pass:

| check | command | output |
|---|---|---|
| no Go source change | `git show --stat 03b0467 -- cmd internal` | empty output, exit 0 (no file under `cmd/` or `internal/` changed) |
| only the two rung-2/Q13 status lines changed in `docs/` | `git diff bca7ddc 03b0467 -- docs` | two hunks: the `docs/VERTICAL-SLICES.md` rung-2 status cell and the `docs/OPEN-QUESTIONS.md` Q13 status line; no other docs line |
| pushed commit equal to the local HEAD at review time | `git ls-remote origin master` | `03b0467d490f3af1f3659d44d8471c81779cc769 refs/heads/master`, equal to `HEAD` |

Pre-declaration evidence: the criterion's pre-declaration is evidenced by commit order — it was committed
in `bca7ddc`, 37 minutes before the run commit `03b0467` — while the declared grid's pre-declaration rests
on §2's text; the grid is exhaustive over the founder axis (`1..16` at every declared seed), so no
post-hoc grid could have hidden a passing cell.
