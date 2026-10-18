# Ticket 37 — M2b: fidelity metric and reference anchor

- Base revision: `fa3f579` (master, tree clean, nothing pushed).
- Branch: `feature/37-fidelity-metric`.
- Author: worker agent (ticket 37), under the advance-phase plan.
- Date opened: 2026-10-18T14:09Z.
- Pre-declared before any scored comparison. This section is commit 1; later sections are appended
  after it and may not alter it (corrections, if any, are recorded as superseding notes with reason).

## Question

`docs/VERTICAL-SLICES.md` M2b: *Is "designed to approach the R reference" a valid objective at all?*
The R reference goes extinct at this fixture and horizon (ticket 35: 0-1 births, ≤1 alive at h143),
so fidelity-to-R rewards a trajectory that loses population. Ticket 36 withdrew the reading that
cognition suppresses reproduction (a full-wake `natural-claim/v4` replicate reproduced at the
no-AI control's rate, 2 births). The open question is the anchor, not an AI-arm effect: this is Q1
in `docs/OPEN-QUESTIONS.md`.

## Scope

In scope: (1) a declared civilization-level distance measure with a defended anchor; (2) a
falsifiable demonstration that it discriminates recorded arms; (3) a comparison of at least two
materially different candidate measures on the recorded artefacts; (4) the owning-document updates
that record the answer (`docs/OPEN-QUESTIONS.md`, `docs/VALIDATION.md`, `docs/VERTICAL-SLICES.md`).

Out of scope, and not claimed: any arm-superiority verdict (the frozen EC-3/EC-4 dominance rule is
untouched); any change to the frozen v1/v2 claim-encoder path; any re-opening of ticket 34/35/36
verdicts; any statement that selective allocation wins; any live model call; any Go instrument
wiring for M2c (M2c owns applying the measure to the calibrated instrument).

## Authoritative inputs (read, not re-derived)

| Input | sha256 / seam | Use |
|---|---|---|
| `.tmp/35-v2-evaluation.json` | `8efd819533d56b419d74639f6e6ba559fb678f013df7120bf6b8c3102d4bae3d` | primary discrimination matrix: contract `natural-claim/v2`, f10-s0, H=144, 3 replicates, arms A0/R/P/U/A/A1, per-hour projections + recorded fidelity rows |
| `.tmp/35-stage3-evaluation.json` | `808de6230b0a7491171eaa037444e4e34229a83ecdec4368dafdc1a22076b8ed` | secondary (v1 contract) recorded set: A0/P/U/A, f10-s0, H=144, 3 replicates |
| `.tmp/36-v4-evaluation.json` | `8aa4342eab551fd8de53d0a5ef0cbcec3f3eb79976993c7ed8325d9218146463` | corroboration for the anchor argument: one full-wake v4 replicate (2 births, survives to h143) |
| `.tmp/35-stage2-none-evaluation.json` | `6fb3c2ecc161a6c99ac7b9a579f8d017bf507203339ddf792b92ab6f16f2f14b` | recorded v1 R effort ladder (`none` reproduces, `low`/`high` die) — context only |
| `internal/experiment/fidelity.go:54` `DivergenceVector` | frozen eight-indicator vector | the current "distance to R" |
| `internal/experiment/projections.go:19` `Projections` | hourly projection extraction | the recorded state vector source |
| `cmd/vs2/report.go:400` `buildFidelity` / `:455` `referenceRun` | report seam | how the reference is chosen and which arms are scored |
| `cmd/vs2/config.go:156` `isBudgeted` | arm filter | A0 and R are never scored |
| `internal/cognition/protocol/valuemodel.go:165` `AdjustedPriority` | `vs2-value/2` | **correction:** this is the allocation priority model, not a distance metric |

**Correction recorded before scoring.** `vs2-value/2` is the value/priority model that decides which
actor wakes (`AdjustedPriority` = projected value scaled by a staleness term and a coverage floor,
`internal/cognition/protocol/valuemodel.go:165`). It is not, and does not contain, a distance to R.
The distance-to-R is `experiment.DivergenceVector` over the eight frozen indicators
(`internal/experiment/fidelity.go:54`), consumed by `buildFidelity` (`cmd/vs2/report.go:400`).
The two seams are separate; this ticket treats the second as "the current metric".

## Pre-declared acceptance criterion (AC-37)

Q1a is answered for this project only when all five hold.

**(a) Declared measure.** One civilization-level distance measure is written with its state vector,
weights, anchor, units, bounds, validity precondition and information loss in `docs/VALIDATION.md`
(its owning home), versioned `vrd/1`.

**(b) Defended anchor.** The anchor is argued in terms the user's priorities name — population
survived, deaths avoided, stability — not "because R does it", and the argument survives its own
falsifier: the R reference itself scores as non-viable under the measure (its distance is strictly
greater than the no-AI control's, with R spending more calls).

**(c) Discrimination (fixed before computing).** On `.tmp/35-v2-evaluation.json` (f10-s0, H=144,
contract `natural-claim/v2`, 3 replicates), the recommended measure ranks the pre-registered worse
arm strictly above the pre-registered better arm, by a margin of at least **0.10** distance units in
**every 3×3 replicate pairing**:

| Pair | worse arm | better arm | fixed survival reason (from the artefact) |
|---|---|---|---|
| D1 | `A1` | `A` | equal declared budget (253 vs 256 realised calls/replicate, quota 256): A1 ends h144 with 0 alive and 0 births in all 3 replicates; A ends with 2/1/2 alive and 2 births in all 3. |
| D2 | `R` | `A0` | R spends 1 082/1 166/1 305 calls versus A0's 0: R ends h144 with 0 alive and 0/0/1 births; A0 ends with 1 alive and 2 births in all 3 replicates. |
| D3 | `R` | `P` | R spends 1 082/1 166/1 305 calls versus P's 244: same survival reason as D2 (A0 and P are identical on these KPIs). |

The threshold and pair set are fixed here. A miss on any pair, in any pairing, is a FAIL and Q1a is
not answered; the failing pair and its arithmetic are recorded verbatim and no weight is re-tuned
after seeing the result (a changed weighting is a new pre-registration in a follow-up ticket).

**(d) Robustness.** Each of the three declared weights is doubled in turn (three one-at-a-time
variants). Criterion (c) must hold in all three variants. This is a declared sensitivity check, not
a search for weights that pass.

**(e) Candidate comparison and slice.** At least two materially different candidate measures are
computed on the same recorded artefacts; their mechanics, their disagreement on recorded arms and
the tradeoff are stated; the observation that would change the recommendation is named. The
user-facing slice is a table of every recorded arm scored under the old and the new measure plus one
self-contained SVG figure showing the extinction-vs-survival ranking.

**Safe way to see failure (negative control).** The scoring script must expose `--self-test`, which
(1) recomputes the frozen seven observable indicator sequences (all but decision-error, whose
ledger is not in the evaluation artefact) from the projections and requires agreement with the
recorded `fidelity` rows at ≤1e-9 — so a failure of the new measure cannot be blamed on a broken
loader or a different implementation of the old one; (2) scores a synthetic perfect arm equal to the
control and requires `D = 0`; (3) scores a synthetic extinct arm (zero alive from hour 48, zero
births) and requires `D ≥ 1.0`; (4) in `--expect-fail` mode flips one declared pair and requires the
check to report FAIL. The script is `.scratch/37-fidelity-metric/score.py`; all of its outputs are
recorded in this ticket.

## Candidate measures (declared before scoring)

### C1 — status quo: reference-trajectory distance (`fid/1`)

State vector: the frozen eight indicators of `experiment.DivergenceVector` against the R arm —
population, births, starvation deaths, age deaths, capital-histogram TV, granary-histogram TV,
exchange volume, decision-error rate. Anchor: the R run. Mechanics: each indicator is an hourly
absolute divergence summarised by its mean; the instrument declares **no weights** (it is a vector),
so the only scalar ranking it supports is an undeclared aggregation. For arithmetic this ticket
uses the equal-weight mean of the eight indicator means and labels that choice an **inference**.
The recorded rows do not carry the replicate they were computed against; the ticket matches them to
each R replicate and reports which reference was actually used.

### C2 — recommended: viability-referenced distance (`vrd/1`)

Outcome vector of a run over the declared horizon `[0, H]` (`H = 144`, samples 0..144):

| Symbol | Quantity | Source field | Unit |
|---|---|---|---|
| `H_x` | surviving actor-hours, `Σ_{t=0..H} alive_x(t)` | `alive_by_hour` | actor-hours |
| `B_x` | cumulative births at `H` | `births_cum[H]` | births |
| `S_x` | cumulative starvation deaths at `H` | `deaths_starve_cum[H]` | deaths |

Anchor `T` = the mean outcome vector of the recorded **A0 (no-AI control)** replicates at the same
fixture, seed and horizon: `H_T`, `B_T`, `S_T`. `N0` = `alive_x(0)` (founding population; 10 in the
recorded matrix; arms whose `N0` differ are not comparable and are refused).

Viability floor: `V = { H ≥ H_T, B ≥ B_T, S ≤ S_T }`. Distance:

```
D(x) = w_H · max(0, (H_T − H_x) / H_T)
     + w_B · max(0, (B_T − B_x) / max(1, B_T))
     + w_S · max(0, (S_x − S_T) / N0)
```

Declared weights `w_H = w_B = w_S = 1`. Each term is dimensionless; `D ≥ 0`; `D = 0` means the run
is at or above the control's achieved outcome on all three (the measure is a distance to a set, not
to a point, and does not penalise doing better than the control on any component).

**Anchor defence (stated before computing).** The target is the outcome the world already reaches
with *no cognitive intervention at all*: it survived to the horizon, it reproduced at its own rate,
and it suffered zero starvation deaths. Approaching that outcome is worth doing in the user's own
terms — population survived, deaths avoided, stability — and it requires no claim that R is good.
The control is deterministic, so the floor has zero sampling noise, and it is recorded in the same
matrix as every arm. It is a floor, not an optimum: exceeding it is not penalised.

**Validity precondition (checked, refused rather than defaulted).** `H_T > 0`, `B_T > 0`,
`alive_T(H) > 0`, and `N0 > 0`. If the control itself is extinct at the horizon or produces no
births, the viability floor is not viable and `vrd/1` is **undefined** for that fixture; the run
must declare a substitute floor in its own ticket and may never silently substitute R. On the two
recorded matrices here the precondition holds (A0: `H_T = 1444`, `B_T = 2`, alive at h144 = 1,
`N0 = 10`), so the measure is defined.

**Information lost (declared):** age-death timing, wealth and granary distributions, exchange,
action mix, decision-error rate, and *who* survives (identity, kin and diversity). `vrd/1` is a
demographic viability distance, not a full fidelity metric; the frozen vector remains a diagnostic
alongside it.

**Tie rule:** arms at `D = 0` are tied and reported as tied. The measure does not fabricate a
ranking out of sampling noise.

### C3 — alternative: endpoint-survival objective (`end/1`)

Same projections, no anchor: order arms by terminal sample alive at `H` (descending), then cumulative
births (descending), then starvation deaths (ascending). It is the naive reading of "population
survived" and it is materially different in what it rewards: only the endpoint counts. It is
expected to rank `U` first (3/2/1 alive at h144 against the control's 1) because U replaces
starved actors with newborns; it does not see the 4-6 starvation deaths or the mid-run collapse.

## Pre-registered hypotheses about the current metric

- H1: under the equal-weight scalarisation of `fid/1`, `A1` has the lowest mean distance among the
  recorded budgeted arms (P, U, A, A1), i.e. the extinct arm is ranked "closest to R".
- H2: `fid/1` gives the population indicator a weight of `1/(8·patches) = 1/16` per actor against
  `1.0` per birth or per starvation death, so survival is almost invisible in the vector's own
  scale.
- H3: the recorded fidelity rows were computed against one R replicate chosen without a declared
  replicate rule; matching the rows against R1/R2/R3 identifies which one, and that identity is
  reported as a defect of the anchor, not a property of R.

H1-H3 are predictions, not acceptance criteria; they are recorded here so the result cannot be
presented as fitted.

## Method

1. Commit this file (commit 1, acceptance before comparison).
2. Write `.scratch/37-fidelity-metric/score.py` (stdlib only) that reads the evaluation JSONs, refuses
   missing fields rather than defaulting, implements `fid/1` recomputation, `vrd/1`, `end/1`, the
   three sensitivity variants, and `--self-test`.
3. Run it on `.tmp/35-v2-evaluation.json` (primary) and `.tmp/35-stage3-evaluation.json`
   (secondary); record exact commands, timestamps, outputs and exit codes here.
4. Generate `.scratch/37-fidelity-metric/ranking.svg` (stdlib SVG) showing both measures per arm,
   extinct arms marked.
5. Append results, candidate comparison, anchor verdict and discrimination evidence to this file.
6. Update the owning documents: `docs/VALIDATION.md` (metric contract `vrd/1`),
   `docs/OPEN-QUESTIONS.md` Q1 (answer + pointer), `docs/VERTICAL-SLICES.md` M2b row (status +
   pointer; state plainly that M2b passing is not evidence for the central claim).
7. Run the gates and record them.

**Cost:** zero live model calls. Every number is computed from recorded ledgers/evaluation JSON.
If a live call were judged unavoidable the work stops and reports it as a blocker with its smallest
call count.

## Gates (before declaring done)

`. .tmp/go-env.sh` (the toolchain in the main worktree), then `go build ./...`, `go vet ./...`,
`gofmt -l` on tracked `.go` files, `go test -timeout 30m ./...`, and `git diff --check fa3f579`.
No Go file is expected to change; the gates are regression evidence for the revision the ticket
lands on.

## Refutation summary

AC-37 fails if: the measure cannot be written without an arbitrary post-hoc weight choice; the
anchor argument needs R to be viable (it does not, and R is scored non-viable); any declared pair
misses the 0.10 margin in any replicate pairing or in any sensitivity variant; the self-test's
loader identity or synthetic controls fail; or the candidate comparison cannot name a materially
different alternative with a stated tradeoff.

---

# Results (appended after commit 1; the pre-declaration above is unchanged)

## Revision and commands

- Base `fa3f579`; pre-declaration commit `528f0e0` on `feature/37-fidelity-metric`.
- Working tree at scoring time: pre-declaration commit plus the analysis files only (no source change).
- Scoring command, run 2026-10-18T14:11Z from the worktree root, exit 0:

```
python3 .scratch/37-fidelity-metric/score.py \
  --primary /home/z/vibe/agentworld/.tmp/35-v2-evaluation.json \
  --secondary /home/z/vibe/agentworld/.tmp/35-stage3-evaluation.json \
  --corroboration /home/z/vibe/agentworld/.tmp/36-v4-evaluation.json \
  --self-test --json-out .scratch/37-fidelity-metric/scores.json \
  --svg .scratch/37-fidelity-metric/ranking.svg
```

- Outputs committed beside this file: `score.py` (sha256 `78c330bd…`), `score-run.log` (sha256
  `9fb63e64…`, the full transcript with per-pair arithmetic), `scores.json` (sha256 `d709dfa3…`),
  `ranking.svg` (sha256 `90f6041f…`, byte-identical before and after the review fixes), and
  `negative-control.log` (sha256 `f81872bc…`, the `--expect-fail` transcript behind gate row 9).
  Every number below is from one of those logs or `scores.json`.
- **Live model calls: 0.** All quantities come from the recorded evaluation JSONs.

## The recorded matrix, scored two ways (primary: `natural-claim/v2`, f10-s0, H=144, 3 reps)

`alive@143` is the hour tickets 35/36 report; `alive@144` is the terminal sample the projections end
on. `fid/1` is the equal-weight mean of the recorded eight indicator means (inference: the instrument
declares no weights). `vrd/1` is the declared viability-referenced distance (§C2 above). The frozen
instrument never scores A0 or R (`cmd/vs2/config.go:156`), so they have no recorded row and their
`mean8` cell is `not-scored`, not zero.
AC-37(e) asks for a table of every recorded arm under the old measure, so the `fid/1` 7-ind column
closes that gap: the same seven ledger-free indicators the loader-identity check recomputes, against
the reference the recorded rows actually used (R3), computed for all six arms on one basis. The eighth
indicator, decision-error, needs the call ledger and is in the evaluation artefact for no arm, so this
column is a seven-of-eight measurement and is not numerically comparable to the eight-indicator
`mean8` column; for the four budgeted arms, `mean8` remains the recorded value.

| arm | rep | calls | alive@143 | alive@144 | births | starvation | actor-hours | `fid/1` mean8 | `fid/1` 7-ind vs R3 | `vrd/1` |
|---|---|---|---|---|---|---|---|---|---|---|
| A0 | 1-3 | 0 | 2 | 1 | 2 | 0 | 1444 | not scored | 0.2094 | 0.0000 |
| R | 1 | 1082 | 0 | 0 | 0 | 1 | 1082 | not scored | 0.3068 | 1.3507 |
| R | 2 | 1166 | 0 | 0 | 0 | 0 | 1166 | not scored | 0.1617 | 1.1925 |
| R | 3 | 1305 | 1 | 0 | 1 | 0 | 1305 | not scored | 0.0000 | 0.5963 |
| P | 1 | 244 | 2 | 1 | 2 | 0 | 1444 | 0.1756 | 0.1987 | 0.0000 |
| P | 2 | 244 | 2 | 1 | 2 | 0 | 1444 | 0.1761 | 0.1986 | 0.0000 |
| P | 3 | 244 | 2 | 1 | 2 | 0 | 1444 | 0.1755 | 0.1992 | 0.0000 |
| U | 1 | 256 | 3 | 3 | 3 | 5 | 1114 | 0.8931 | 1.0196 | 0.7285 |
| U | 2 | 256 | 2 | 2 | 3 | 4 | 1199 | 0.8459 | 0.9624 | 0.5697 |
| U | 3 | 256 | 2 | 1 | 2 | 6 | 1034 | 0.9299 | 1.0597 | 0.8839 |
| A | 1 | 256 | 2 | 2 | 2 | 0 | 1443 | 0.2055 | 0.2316 | 0.0007 |
| A | 2 | 256 | 2 | 1 | 2 | 0 | 1444 | 0.2034 | 0.2305 | 0.0000 |
| A | 3 | 256 | 2 | 2 | 2 | 2 | 1264 | 0.4170 | 0.4760 | 0.3247 |
| A1 | 1 | 253 | 0 | 0 | 0 | 0 | 1166 | 0.1519 | 0.1718 | 1.1925 |
| A1 | 2 | 253 | 0 | 0 | 0 | 0 | 1166 | 0.1524 | 0.1729 | 1.1925 |
| A1 | 3 | 253 | 0 | 0 | 0 | 0 | 1166 | 0.1518 | 0.1715 | 1.1925 |

Arm means and the two rankings:

| arm | mean calls | mean `fid/1` | rank (lower = closer to R) | mean `fid/1` 7-ind | mean `vrd/1` | rank (lower = more viable) |
|---|---|---|---|---|---|---|
| A0 | 0 | not scored | — | 0.2094 | 0.0000 | 1 (tied) |
| P | 244 | 0.1757 | 2 | 0.1988 | 0.0000 | 1 (tied) |
| A | 255.7 | 0.2753 | 3 | 0.3127 | 0.1084 | 3 |
| U | 256 | 0.8897 | 4 | 1.0139 | 0.7274 | 4 |
| R | 1184.3 | not scored | — | 0.1562 | 1.0465 | 5 |
| A1 | 253 | **0.1520** | **1 (closest to R)** | 0.1721 | **1.1925** | **6 (least viable)** |

On the consistent seven-indicator basis every recorded arm is ordered: R 0.1562 (the reference itself;
its r3 sits at distance 0 by construction), **A1 0.1721**, P 0.1988, A0 0.2094, A 0.3127, U 1.0139.
Scoring the two arms the instrument omits does not soften the inversion: the extinct, zero-birth A1
remains the arm closest to R of every arm that is not R, and the no-AI control is farther from R than
the extinct arm is.

**The inversion is total: the arm the current instrument calls closest to R is the arm the declared
viability measure calls least viable, and it is extinct in all three replicates.**

Figure: `.scratch/37-fidelity-metric/ranking.svg` (self-contained SVG, two panels, same arm order).
Left panel: `fid/1`; its lowest bar is A1, solid red (extinct). Right panel: `vrd/1`; A0 and P sit
at 0 and A1 is the highest bar. The left panel plots the instrument's recorded rows only, so A0 and R
are absent from it; their old-measure values are the supplementary seven-indicator column above,
which exists in the table and the log, not in the figure.

## What the current metric actually sums (H1 and H2)

`DivergenceVector` (`internal/experiment/fidelity.go:54`) sums, per hour, absolute differences of raw
counts (births, starvation deaths, age deaths, exchange), `|Δalive|/16` for population, and
total-variation distances of the capital and granary histograms. Sixteen is `8 × patches` with
`patches = 2` (`cmd/vs2/config.go:316`); the observed population sequence is a constant 0.0625 per
one-actor difference, which confirms the normaliser. Nothing normalises the birth or death counts,
and nothing declares weights.

Equal-weight contributions to the scalar (each indicator mean ÷ 8), from the run log:

| arm | population | births | starvation | age | capital | granary | decision-error | total |
|---|---|---|---|---|---|---|---|---|
| A1 | 0.0075 (5 %) | **0.1207 (79 %)** | 0 | 0.0009 (1 %) | 0.0098 (6 %) | 0.0118 (8 %) | 0.0015 (1 %) | 0.1520 |
| P | 0.0075 (4 %) | 0.1198 (68 %) | 0 | 0 | 0.0217 (12 %) | 0.0250 (14 %) | 0.0018 (1 %) | 0.1757 |
| A | 0.0066 (2 %) | 0.1198 (44 %) | 0.0592 (22 %) | 0.0083 (3 %) | 0.0376 (14 %) | 0.0420 (15 %) | 0.0017 (1 %) | 0.2753 |
| U | 0.0143 (2 %) | 0.1664 (19 %) | **0.4560 (51 %)** | 0.1287 (14 %) | 0.0548 (6 %) | 0.0669 (8 %) | 0.0025 (0 %) | 0.8897 |

The arithmetic of the inversion:

1. **A whole-horizon one-actor population difference contributes 0.0599 to its indicator and 0.0075
   to the scalar; one birth difference contributes 0.9655 and 0.1207.** Survival is worth 16× less
   than a birth count in the vector's own scale (H2 confirmed: `1/16` vs `1`).
2. **The birth term is direction-blind.** `|0 − 1| = |2 − 1| = 1`, so A1 (0 births) and A (2 births)
   both score ≈0.120 in the scalar's largest term against a reference that produced 1 birth. The
   metric cannot tell failing to reproduce from reproducing at the control's rate.
3. **What separates A1 from P is wealth-state resemblance to R's decline.** A1's capital + granary
   contributions are 0.0216 versus P's 0.0467; A1's dying, capital-depleted trajectory tracks R's
   dying, capital-depleted one. That is the whole 0.0237 gap between "closest to R" (A1) and the arm
   that reproduces exactly like the no-AI control (P).
4. **The control is invisible.** `isBudgeted` excludes A0 and R (`cmd/vs2/config.go:156`), so the
   instrument never computes A0's distance to R. The metric had no way to notice that its anchor is
   worse than doing nothing: A0 matches P's demographics with 0 calls, while R spends 1 082-1 305.

So yes — the current instrument prefers the extinct R-like arm, and the arithmetic above is why. The
precise statement is about the *fidelity metric*, not `vs2-value/2`: the latter is the allocation
priority model (`internal/cognition/protocol/valuemodel.go:165`) and carries no distance term.

## The current anchor is not even a fixed reference (H3)

The reported `fid/1` rows do not say which R replicate they were computed against. Recomputing the
seven ledger-free indicators from the projections and matching all 12 recorded rows identifies
**R replicate 3** (tol 1e-9, no row matches R1 or R2). R3 is the mildest reference: the only R
replicate with a birth (h5) and 1 alive at h143. The cause is structural: `referenceRun`
(`cmd/vs2/report.go:455`) matches `(arm, F, seed)` only, `sortRuns` (`:466`) does not order by
replicate, and runs are appended in goroutine-completion order (`cmd/vs2/main.go:223-227`), so the
reference is whichever R replicate lands first in the slice. A re-run of the report could pick R1 or
R2 and change every absolute number.

Reference dependence, seven-indicator means (run log):

| arm | vs R1 | vs R2 | vs R3 (the recorded reference) |
|---|---|---|---|
| A1 | 0.167 | **0.020** | 0.172 |
| P | 0.493 | 0.347 | 0.199 |
| A | 0.485-0.522 | 0.372-0.609 | 0.231-0.476 |
| U | 0.948-1.046 | 1.092-1.191 | 0.962-1.060 |

The ranking `A1 < P < A < U` holds against **every** R replicate, so the "extinct arm is closest"
finding is not an artifact of the R3 pick; the magnitudes are (A1's distance shrinks 8.5× when R2 is
the reference). It remains a defect that the reference identity is undeclared and that no reference
uncertainty is propagated.

## Candidate comparison and recommendation

| candidate | anchor | state vector | recorded winner | what it rewards | failure the recorded data exposes |
|---|---|---|---|---|---|
| C1 `fid/1` (status quo) | the R trajectory | 8 frozen indicators | **A1** (0.1520), then P (0.1757) | resemblance to R: depleted wealth state + a birth count within one of R's | A1 is extinct with 0 births; the control A0 is not scored at all |
| C2 `vrd/1` (recommended) | the no-AI control's realised outcome as a viability floor | actor-hours H, births B, starvation deaths S | **A0 and P** (0.0000), then A (0.1084), U (0.7274), R (1.0465), A1 (1.1925) | persistence + reproduction at the world's own no-cognition floor, deaths avoided | ties at 0 cannot be ordered; identity/diversity of survivors and age-death timing are lost |
| C3 `end/1` (alternative) | none (reproduction / endpoint survival as the objective) | terminal alive, births, −starvation | **U**, then A, A0, P, R, A1 | headcount at the horizon | U buys its 2.33 mean endpoint headcount with 4-6 starvation deaths and 15-28 % fewer actor-hours; it ignores the path |

The three candidates rank the same recorded arms differently: **A1 wins under C1 and loses under C2;
U wins under C3 and is fourth under C2; A0/P win under C2 and are unranked under C1.** They are
materially different in anchor and in what they count, not merely different weights on one vector.

**Recommendation: C2 (`vrd/1`), with C1 retained as a diagnostic vector and C3 rejected as a
standalone objective.** The reason is the user's stated priority at M2b: population survived, deaths
avoided, stability. C1 anchors on a trajectory that is strictly worse than the control at 4-5× its
budget; C3 rewards an endpoint that hides churn. C2 pays for that with declared weights, a
dimensionless construction and a validity precondition — and it cannot order ties at 0, which is the
honest answer when several arms are at the viability floor.

**Observation that would change the recommendation:** a recorded arm with `B > B_T`, `S ≤ S_T` and
`H ≥ H_T` — growth at no survival cost. C2 scores it 0 like the control and cannot prefer it; at that
point the objective should gain a declared growth component (or C3's ordering becomes relevant), and
the weights are re-pre-registered. No recorded arm does this: U has `B > B_T` but `H < H_T` and
`S > S_T` in all three replicates. A second trigger: if a future fixture's A0 control is itself
extinct at the horizon, C2 is undefined and a declared substitute floor is required — never R.

## Anchor verdict

Under `vrd/1`, R scores 0.5963-1.3507 while spending 1 082-1 305 calls; A0 scores 0.0000 with 0
calls; P scores 0.0000 with 244. **"Designed to approach the R reference" is not a valid objective
at this fixture: it is a distance to a trajectory that is less viable than doing nothing, purchased
with 4-5× the periodic budget.** The defended anchor is the no-AI control's own achieved outcome —
survived to the horizon, 2 births, 0 starvation deaths — because it is the world's own floor, it is
deterministic, and it is recorded in the same matrix as every arm. It is not an optimum and is not
claimed as one; exceeding it is not penalised.

Ticket 36's v4 corroboration, quoted as its recorded fact rather than recomputed here: the one
full-wake `natural-claim/v4` replicate (1325 calls) ends with 2 births and 1 alive at h144 (2 at
h143) and 1 starvation death. The v4 artefact contains no A0 control and is a different contract, so
`vrd/1` is **undefined** there and none is computed; the cross-contract rule forbids substituting the
v2 control. The R extinction under v1/v2 is therefore not intrinsic to full cognition, which is why
the anchor question is about the reference, not an AI-arm effect.

## Known pathology in `vrd/1`: the birth term outranks the survival term

The anchor is argued in the user's priority order — population survived, deaths avoided, stability —
and the three terms are written in that order. The weights do not share that order. With `w_H = w_B =
w_S = 1` and `B_T = 2`, one birth is worth `1/B_T = 0.5`, while losing every founder for the whole
horizon is worth 1.0: the birth term carries half the survival axis, and the ranking can invert. Two
worlds, from the declared formula:

| world | `H` | `B` | `S` | terms | `vrd/1` |
|---|---|---|---|---|---|
| sterile but fully surviving (synthetic: ten founders alive at every one of the 145 samples, no birth) | 1450 | 0 | 0 | 0 + 1.0 + 0 | **1.0000** |
| extinct at h144 after one birth (recorded R r3) | 1305 | 1 | 0 | 0.0963 + 0.5 + 0 | **0.5963** |

**A fully-surviving, death-free world that never reproduces scores strictly worse than a world that
goes extinct with one birth.** This is not a corner of the declared weights: the inversion holds for
every `w_B` above `≈ 0.19`, so removing it would take a five-fold reduction of a declared weight of
1.0, and none of the three declared one-at-a-time variants comes near that — doubling `w_B` widens
the inversion, while doubling `w_H` leaves R r3 at 0.6925 and doubling `w_S` leaves it at 0.5963,
both still ranked more viable than the sterile world's 1.0000. Re-weighting after seeing the data is
exactly what AC-37(c) forbids, so no weight is changed here. Instead both worlds are exercised by
`score.py --self-test` (`score-run.log:79-80`), and the sterile case is *pinned*, so a later change
to the weights or the normalisation fails the self-test until the change is re-declared.

This is a **declared known pathology of the objective**, not a footnote. It does not overturn AC-37
as written — D1/D2/D3 test the ordering of recorded arms and no declared pair sits on the inverted
edge — but it does mean `vrd/1` is not sound without qualification, and it is a named prerequisite
of the M2c record (`docs/VERTICAL-SLICES.md`, M2c row) that the birth term be repaired or normalised
there before any arm comparison may be read.

## Discrimination evidence on recorded runs (AC-37c/d)

Pre-registered pairs, all 3×3 replicate pairings, margin threshold 0.10:

| pair | worse → better | min margin | pairings | verdict |
|---|---|---|---|---|
| D1 | A1 → A (253 vs 256 calls) | **+0.8679** | 9/9 | PASS |
| D2 | R → A0 (1 082-1 305 vs 0 calls) | **+0.5963** | 9/9 | PASS |
| D3 | R → P (1 082-1 305 vs 244 calls) | **+0.5963** | 9/9 | PASS |

Sensitivity, each weight doubled in turn (`scores.json`):

| variant | D1 min margin | D2/D3 min margin | verdict |
|---|---|---|---|
| `w_H × 2` | +0.9357 | +0.6925 | PASS |
| `w_B × 2` | +1.8679 | +1.0963 | PASS |
| `w_S × 2` | +0.6679 | +0.5963 | PASS |

**Negative control (`score.py --self-test`, in the same run, `score-run.log:76-81`):** loader identity
PASS — all 12 recorded rows are reproduced from the projections for the seven ledger-free indicators
at 1e-9, so the script is reading the same artefact and implementing the same old metric; synthetic
perfect arm (the control) scores exactly 0.0000; synthetic extinct arm (0 alive after h48, 0 births)
scores 1.6676 ≥ 1.0; **synthetic over-floor survivor** (all ten founders alive at every one of the
145 samples, `H = 1450 > H_T = 1444`) scores exactly 0.0000, which exercises the strict `>` in the
`(H_T − H_x)/H_T` term — the case the ticketed version claimed safe without an artefact; **synthetic
sterile survivor** (the same survival, zero births) scores 1.0000 against R r3's 0.5963 and is pinned
as the declared known pathology above; the declared pair A1→A passes while the inverted pair A→A1 is
detected as FAIL.

**The `--expect-fail` mode is now a real inversion, not a flipped boolean.** It re-runs each declared
pair with worse and better swapped and requires every swapped pair to FAIL (`score.py:312-337`); the
transcript is committed as `negative-control.log`, whose three lines read `NEGATIVE-CONTROL:
INVERSION-DETECTED (A1 vs A) / (A0 vs R) / (P vs R)` and end in `NEGATIVE CONTROL PASS: every swapped
pair was detected as a failure`. As wired before this review round the mode inverted the *verdict* on
the unchanged pair list, so `main()` printed `NEGATIVE CONTROL PASS` unconditionally; that is repaired.
The repaired mode's ability to fail was exercised by a bounded mutation of an isolated copy of the
script (`/tmp/37mut/score.py`, declared pairs written in their swapped order): it prints `NEGATIVE
CONTROL FAIL: every swapped pair was detected as a failure` and exits 2.

**Secondary recorded matrix** (`natural-claim/v1`, stage 3, f10-s0, H=144, 3 reps; no R arm, so no
`fid/1`): `vrd/1` arm means A0 0.0000, P 0.0000, A 0.4024, U 0.7113; A's r3 is extinct (0 alive, 0
births) and scores 1.1925. The same ordering — survivors at the floor, the high-churn arm above, the
extinct replicate far above — reproduces on the independent v1-contract matrix. Corroboration only:
the pre-registered pairs are the v2 matrix above.

## Limitations and what was not exercised

- **One fixture, one horizon.** f10-s0, H=144. The frozen rule's "both cells" clause is met only
  vacuously (ticket 35 re-plan); `vrd/1`'s anchor is validated at this fixture only. No f12-s0.
- **Model-conditional.** This is recorded model output at one cell; no real-world claim (P14).
- **Weights are declared, not calibrated.** The one-at-a-time doubling is a bounded sensitivity
  check, not a search. No weight was changed after scoring.
- **`vrd/1`'s birth term outranks its survival term.** A sterile but fully surviving world scores
  1.000 while an extinct world with one birth scores 0.5963 (§Known pathology). This is a declared
  defect of the objective, not a slip of the arithmetic; M2c must repair or normalise the term
  before any arm comparison may be read.
- **Information loss is real and declared:** who survives (identity, kin, diversity), age-death
  timing, wealth/granary distributions, exchange, action mix, decision-error rate. `vrd/1` is a
  demographic viability distance; it is not a total fidelity metric.
- **Ties at 0 are not ordered.** A0, P and A r1/r2 all reach the floor. The measure reports the tie;
  it does not invent a tiebreak.
- **The decision-error indicator was not recomputed** (the call ledger is not in the evaluation
  JSON); loader identity covers 7 of the 8 frozen indicators. The recorded eighth indicator is used
  as recorded in the scalar.
- **No Go instrument.** `vrd/1` is declared in `docs/VALIDATION.md` and implemented in
  `.scratch/37-fidelity-metric/score.py` over recorded artefacts. M2c must implement it in the
  evaluator and check the implementation against `scores.json` before using it on a live run. That is
  a scope decision recorded here, not an omission: this ticket's acceptance is the measure, the
  anchor and the discrimination, with zero live spend.
- **The reference-selection defect is not repaired here.** `referenceRun` ignores replicate and
  `sortRuns` does not order by it, so the diagnostic vector's reference is undeclared. Repairing it
  inside M2b would change ticket 35's recorded diagnostic rows and the EC-3/EC-4 inputs, which this
  ticket is not authorised to re-open. M2c should either declare the reference explicitly (a fixed
  replicate or the replicate band) or drop the diagnostic from live use.
- **Raw evidence lives in transient `.tmp`.** The evaluation JSONs remain at the paths and sha256s
  above (pre-existing project practice); this ticket commits the derived scoring table, JSON and log
  so every claim here is re-checkable without them. If `.tmp` is cleared, the raw per-hour data is
  gone; the numbers in this ticket survive.
- **Not claimed:** any arm is better than another (EC-3/EC-4 dominance is untouched); the central
  claim; that selective allocation wins. M2b passing means the pre-declared criterion is met at this
  one fixture, with the birth-term pathology above declared and unrepaired.

## AC-37 verdict

| part | status | evidence |
|---|---|---|
| (a) declared measure | PASS | §C2 above; `docs/VALIDATION.md` "Viability-referenced distance `vrd/1`" |
| (b) defended anchor | PASS | anchor verdict above: R scores 0.5963-1.3507 vs A0 0.0000, at 1 082-1 305 vs 0 calls |
| (c) discrimination | PASS | D1/D2/D3, min margins +0.8679/+0.5963/+0.5963, 9/9 pairings each |
| (d) robustness | PASS | three single-weight doublings, all PASS |
| (e) candidates + slice | PASS | C1/C2/C3 comparison; **every one of the six recorded arms is scored under both measures** — old: the recorded `mean8` for P/A/U/A1 and the seven-indicator recomputation against R3 for A0 (0.2094) and R (0.1562), new: `vrd/1` for all six; `ranking.svg` |

**What M2b passing establishes, exactly:** (i) the anchor is defined and defended — the no-AI
control's own realised outcome, deterministic and recorded in the same matrix; (ii) the
discrimination requirement pre-declared in AC-37(c)/(d) is met at this fixture — D1/D2/D3 pass with a
minimum margin of +0.5963 and hold under all three one-at-a-time weight doublings; (iii) the measure
carries a declared known pathology — birth-term dominance, §Known pathology — which survives every
declared sensitivity variant and which M2c must repair or normalise.

What that does **not** establish: that `vrd/1` is a sound objective without qualification, that it can
adjudicate the central claim, or that any arm is better than another. The fidelity metric is
superseded as the *stated* objective, and the M2 milestone and the central claim remain **not
accepted**. M2c (allocation on the calibrated instrument) is the next stage; it owns the Go
implementation of `vrd/1`, must reproduce `scores.json`, and must repair or normalise the birth term
before any arm comparison it produces may be read.

---

# Gates (recovery run, appended 2026-10-18)

## Why this section exists

The gate run that should have preceded `f9259fe` never finished: its worker was killed by a 30-minute wall clock, its worktree and its untracked `gates.log` were deleted, and the commit that says "record M2b passed" therefore carried a verdict with no gate evidence behind it in the tree.
The last state that worker showed was a **FAIL** of `TestNaturalMechanismSeamBidirectionalMismatch` in `agentworld/internal/world` during its full-suite run.
That output is unrecoverable, so it is re-derived by triage below rather than quoted.
Recovery scope: gates and wording only — no new analysis, no live model call, no re-tuning of any declared weight, and no edit to the pre-declaration above.

## Triage: the reported `internal/world` failure

**Verdict: not introduced by this branch, and not reproducible at `f9259fe`.**

1. The branch changes no Go file: `git diff --stat fa3f579..f9259fe -- '*.go' internal/ cmd/ Makefile go.mod` is empty, and all nine changed paths are `.scratch/`, `docs/` or `CHANGELOG.md`. A branch that changes neither source nor test cannot change this test's behaviour.
2. The identical command passes on the branch and on the baseline `fa3f579`, five runs each (rows 4-5), and again fifty times on the branch (row 6): 55 branch executions and 5 baseline executions, no failure.
3. The full suite on the branch passes, and it includes that test.

"Pre-existing" is deliberately not asserted: the failing log died with its worktree, so what is established is that the failure is not reproducible at `f9259fe` and cannot have been introduced by it.
The surviving explanation is a nondeterministic or environmental failure in that one run.
Nothing is repaired under this heading, because nothing here is this ticket's to repair.

Branch output, verbatim (`feature/37-fidelity-metric` at `f9259fe`):

```
# worktree: /home/z/vibe/worktrees/agentworld/wt-fidelity
# revision: f9259fed805000e450d8060e1fdda3a53270af33 (feature/37-fidelity-metric)
# date: 2026-10-18T14:36:57Z
$ . .tmp/go-env.sh   # GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24, go1.24.13
$ cd /home/z/vibe/worktrees/agentworld/wt-fidelity && go test -run 'TestNaturalMechanismSeamBidirectionalMismatch' -count=5 ./internal/world
--- begin output ---
ok  	agentworld/internal/world	0.591s
--- end output ---
exit=0 wall=5s
```

Baseline output, verbatim (detached worktree at `fa3f579`):

```
# worktree: /home/z/vibe/worktrees/agentworld/wt-baseline
# revision: fa3f5798a97c0265659bb6f942ecaacdd476dd20 (HEAD)
# date: 2026-10-18T14:37:02Z
$ . .tmp/go-env.sh   # GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24, go1.24.13
$ cd /home/z/vibe/worktrees/agentworld/wt-baseline && go test -run 'TestNaturalMechanismSeamBidirectionalMismatch' -count=5 ./internal/world
--- begin output ---
ok  	agentworld/internal/world	0.582s
--- end output ---
exit=0 wall=6s
```

## Gate table

Every row below was personally observed in this recovery run; a command that did not run is marked not run rather than green.
`go` is go1.24.13 from `/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`, sourced with `. .tmp/go-env.sh`, `GOTOOLCHAIN=local`.

| # | Check | Exact command (cwd = branch worktree) | Result |
|---|---|---|---|
| 1 | build | `go build ./...` | exit 0 — PASS |
| 2 | vet | `go vet ./...` | exit 0 — PASS |
| 3 | gofmt on tracked `.go` | `git ls-files -z '*.go' \| xargs -0 gofmt -l` (288 files) | no output, exit 0 — PASS |
| 4 | single-test triage, branch | `go test -run 'TestNaturalMechanismSeamBidirectionalMismatch' -count=5 ./internal/world` at `f9259fe` | `ok agentworld/internal/world 0.591s`, exit 0 — PASS (5 executions) |
| 5 | single-test triage, baseline | the same command at `fa3f579` | `ok agentworld/internal/world 0.582s`, exit 0 — PASS (5 executions) |
| 6 | single-test repeat | the same command with `-count=50` at `f9259fe` | `ok agentworld/internal/world 5.879s`, exit 0 — PASS (50 executions) |
| 7 | full suite | `go test -timeout 30m ./...` at `f9259fe`, detached, log committed as `gates-full.log` | exit 0, 17 packages `ok` and 1 with no test files, `internal/world 553.949s` — PASS |
| 8 | scoring reproducibility | `python3 .scratch/37-fidelity-metric/score.py --primary /home/z/vibe/agentworld/.tmp/35-v2-evaluation.json --secondary /home/z/vibe/agentworld/.tmp/35-stage3-evaluation.json --corroboration /home/z/vibe/agentworld/.tmp/36-v4-evaluation.json --self-test --json-out /tmp/37-scores-recheck.json --svg /tmp/37-ranking-recheck.svg` | exit 0; `/tmp/37-scores-recheck.json` and `/tmp/37-ranking-recheck.svg` **byte-identical** to the committed `scores.json` and `ranking.svg` — PASS (re-run against the revised `score.py` at 2026-10-18T14:58:50Z, still byte-identical) |
| 9 | negative control | `python3 .scratch/37-fidelity-metric/score.py --primary … --secondary … --corroboration … --self-test --expect-fail` (each declared pair is checked with worse and better swapped and is required to FAIL) | exit 0; three `NEGATIVE-CONTROL: INVERSION-DETECTED` lines (A1 vs A, A0 vs R, P vs R) and `NEGATIVE CONTROL PASS: every swapped pair was detected as a failure`; full transcript committed as `negative-control.log` — PASS |
| 10 | whitespace/conflict check | `git diff --check fa3f579` | exit 0 — PASS |
| 11 | race suite (`make race`) | not run | **not run** — not in this ticket's pre-declared gate list; recorded rather than claimed |
| 12 | full suite on `fa3f579` | not run | **not run** — the single test was triaged on the baseline; the branch is the revision that would land, and its own suite is the gate |

**Pre-existing / introduced split.** No gate is red, so no gate is attributable to this branch.
The one reported red — `TestNaturalMechanismSeamBidirectionalMismatch` in the interrupted run — is recorded as **not reproduced, not introduced**, with rows 4-7 as the evidence; it is neither a pre-existing failure this ticket introduces evidence for nor a branch regression, and it is left alone.

## Full-suite log

Committed at `.scratch/37-fidelity-metric/gates-full.log` so it survives the worktree. Verbatim:

```
# worktree: /home/z/vibe/worktrees/agentworld/wt-fidelity
# revision: f9259fed805000e450d8060e1fdda3a53270af33 (feature/37-fidelity-metric)
# date: 2026-10-18T14:38:12Z
# $ . .tmp/go-env.sh  (go1.24.13, GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24)
# $ go test -timeout 30m ./...
--- begin output ---
ok  	agentworld/cmd/capacity	4.904s
ok  	agentworld/cmd/foodflow	6.055s
ok  	agentworld/cmd/natural	20.602s
ok  	agentworld/cmd/reproduction	6.685s
ok  	agentworld/cmd/socialfood	7.188s
?   	agentworld/cmd/survival	[no test files]
ok  	agentworld/cmd/vs1	74.807s
ok  	agentworld/cmd/vs2	19.966s
ok  	agentworld/internal/checkpoint	0.006s
ok  	agentworld/internal/cognition	0.431s
ok  	agentworld/internal/cognition/protocol	0.004s
ok  	agentworld/internal/component	3.235s
ok  	agentworld/internal/experiment	0.006s
ok  	agentworld/internal/kernel	0.037s
ok  	agentworld/internal/scheduler	0.141s
ok  	agentworld/internal/sim	0.005s
ok  	agentworld/internal/strategy	0.854s
ok  	agentworld/internal/world	553.949s
--- end output ---
exit=0
# finished: 2026-10-18T14:49:38Z
```

`internal/world` took 553.9 s here against the 985.4 s the Makefile records at `c1d7810`: the pre-declared `-timeout 30m` never came near expiry on this machine.
The 30-minute limit that killed the earlier worker was its own wall clock, not the suite's.

## Reconciliation of the claim

- **AC-37 (a)-(e) are met by the evidence in `e77916b`.** The measure, anchor, discrimination, robustness, candidate comparison and slice are recorded above, and row 8 re-derives their committed artefacts byte-identically from the unchanged input sha256s (4/4 match the table in "Authoritative inputs").
- **The gates that matter are green or accounted for**: build, vet, gofmt, full suite and `git diff --check` pass (rows 1-3, 7, 10); the race suite and the baseline full suite are declared not run (rows 11-12) rather than assumed green.
- Therefore **"M2b passed" stands**, and the wording in `CHANGELOG.md` and in the M2b row of `docs/VERTICAL-SLICES.md` is unchanged by this section. It was narrowed afterwards by the post-review pass recorded below, which is a separate change and not a re-reading of this evidence.
- One ordering correction is recorded rather than rewritten: the verdict preceded its gate evidence. `f9259fe` asserted the verdict while the evidence did not yet exist and while the interrupted run had shown a red; this section is the evidence arriving afterwards, and it matches. Amending that commit would hide the sequence instead of repairing it.
- **The full-suite log is bound to `f9259fe`, not to the revision that lands.** Rows 1-3 and 10 were re-run at the post-review revision (below), but `go test -timeout 30m ./...` was not: no `.go` file changed after `f9259fe` (`git diff --stat f9259fe..HEAD -- '*.go' cmd/ internal/ Makefile go.mod go.sum` is empty, and every later commit touches only `.scratch/`, `docs/` and `CHANGELOG.md`), so `gates-full.log` already covers every line of code in the landing revision. It is not a fresh run of the tip, and no fresh green is claimed; re-running a 16-minute suite to re-measure unchanged Go code would buy no information.
- Bound: these gates are regression evidence for this revision plus a reproduction of the scoring artefacts. They do not re-run the M2b analysis, and they do not make M2b evidence for the central claim, which remains not accepted.

## Post-review pass (2026-10-18T14:58Z)

The independent review returned *merge OK with notes* and three P1 text-level findings: the
`vrd/1` birth-term pathology, AC-37(e)'s missing A0/R rows, and a negative-control mode that could
not fail. All three are repaired above, with no re-scoring of the recorded arms, no re-weighting and
no live call:

- **Birth-term pathology declared and measured** (§Known pathology). The two example worlds are now
  self-test cases (`score-run.log:79-80`), the sterile case pinned; the over-floor case exercises the
  strict `>` that no recorded arm reaches.
- **AC-37(e) closed without a caveat.** `A0` and `R` are scored under the old measure for the first
  time, from `frozen_indicators()` against the reference the recorded rows actually used (R3), and
  the eighth indicator is footnoted as unavailable in the evaluation artefact.
- **Gate row 9 is now executable evidence.** The `--expect-fail` mode performs a real pair inversion
  and its transcript is committed as `negative-control.log`; that the mode can fail is shown by a
  bounded mutation of an isolated copy (`/tmp/37mut/score.py`), which exits 2 with `NEGATIVE CONTROL
  FAIL`.
- Rows 1-3 (build, vet, gofmt), 8 (scoring reproducibility) and 10 (`git diff --check fa3f579`) were
  re-executed after these changes; row 7 (the full suite) was not, for the reason recorded above.
