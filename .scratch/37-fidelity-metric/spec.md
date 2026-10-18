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

- Outputs committed beside this file: `score.py` (sha256 `b738059c…`), `score-run.log` (sha256
  `ae74c1e7…`, the full transcript with per-pair arithmetic), `scores.json` (sha256 `f78dda6b…`),
  `ranking.svg` (sha256 `90f6041f…`). Every number below is from that log or `scores.json`.
- **Live model calls: 0.** All quantities come from the recorded evaluation JSONs.

## The recorded matrix, scored two ways (primary: `natural-claim/v2`, f10-s0, H=144, 3 reps)

`alive@143` is the hour tickets 35/36 report; `alive@144` is the terminal sample the projections end
on. `fid/1` is the equal-weight mean of the recorded eight indicator means (inference: the instrument
declares no weights). `vrd/1` is the declared viability-referenced distance (§C2 above). A0 and R are
**not scored by the frozen instrument** (`cmd/vs2/config.go:156`), so their `fid/1` cell is
`not-scored`, not zero.

| arm | rep | calls | alive@143 | alive@144 | births | starvation | actor-hours | `fid/1` mean8 | `vrd/1` |
|---|---|---|---|---|---|---|---|---|---|
| A0 | 1-3 | 0 | 2 | 1 | 2 | 0 | 1444 | not scored | 0.0000 |
| R | 1 | 1082 | 0 | 0 | 0 | 1 | 1082 | not scored | 1.3507 |
| R | 2 | 1166 | 0 | 0 | 0 | 0 | 1166 | not scored | 1.1925 |
| R | 3 | 1305 | 1 | 0 | 1 | 0 | 1305 | not scored | 0.5963 |
| P | 1 | 244 | 2 | 1 | 2 | 0 | 1444 | 0.1756 | 0.0000 |
| P | 2 | 244 | 2 | 1 | 2 | 0 | 1444 | 0.1761 | 0.0000 |
| P | 3 | 244 | 2 | 1 | 2 | 0 | 1444 | 0.1755 | 0.0000 |
| U | 1 | 256 | 3 | 3 | 3 | 5 | 1114 | 0.8931 | 0.7285 |
| U | 2 | 256 | 2 | 2 | 3 | 4 | 1199 | 0.8459 | 0.5697 |
| U | 3 | 256 | 2 | 1 | 2 | 6 | 1034 | 0.9299 | 0.8839 |
| A | 1 | 256 | 2 | 2 | 2 | 0 | 1443 | 0.2055 | 0.0007 |
| A | 2 | 256 | 2 | 1 | 2 | 0 | 1444 | 0.2034 | 0.0000 |
| A | 3 | 256 | 2 | 2 | 2 | 2 | 1264 | 0.4170 | 0.3247 |
| A1 | 1 | 253 | 0 | 0 | 0 | 0 | 1166 | 0.1519 | 1.1925 |
| A1 | 2 | 253 | 0 | 0 | 0 | 0 | 1166 | 0.1524 | 1.1925 |
| A1 | 3 | 253 | 0 | 0 | 0 | 0 | 1166 | 0.1518 | 1.1925 |

Arm means and the two rankings:

| arm | mean calls | mean `fid/1` | rank (lower = closer to R) | mean `vrd/1` | rank (lower = more viable) |
|---|---|---|---|---|---|
| A0 | 0 | not scored | — | 0.0000 | 1 (tied) |
| P | 244 | 0.1757 | 2 | 0.0000 | 1 (tied) |
| A | 255.7 | 0.2753 | 3 | 0.1084 | 3 |
| U | 256 | 0.8897 | 4 | 0.7274 | 4 |
| R | 1184.3 | not scored | — | 1.0465 | 5 |
| A1 | 253 | **0.1520** | **1 (closest to R)** | **1.1925** | **6 (least viable)** |

**The inversion is total: the arm the current instrument calls closest to R is the arm the declared
viability measure calls least viable, and it is extinct in all three replicates.**

Figure: `.scratch/37-fidelity-metric/ranking.svg` (self-contained SVG, two panels, same arm order).
Left panel: `fid/1`; its lowest bar is A1, hatched red (extinct). Right panel: `vrd/1`; A0 and P sit
at 0 and A1 is the highest bar. The control and the reference are unscoreable on the left by
construction.

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

**Negative control (`score.py --self-test`, in the same run):** loader identity PASS — all 12 recorded
rows are reproduced from the projections for the seven ledger-free indicators at 1e-9, so the script
is reading the same artefact and implementing the same old metric; synthetic perfect arm (the
control) scores exactly 0.0000; synthetic extinct arm (0 alive after h48, 0 births) scores 1.6676 ≥
1.0; the declared pair A1→A passes while the inverted pair A→A1 is detected as FAIL. Run with
`--expect-fail`, the script exits with `NEGATIVE CONTROL PASS`, so the check that would catch a
measure rewarding extinction was exercised, not assumed.

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
  claim; that selective allocation wins. M2b passing means the metric is defensible.

## AC-37 verdict

| part | status | evidence |
|---|---|---|
| (a) declared measure | PASS | §C2 above; `docs/VALIDATION.md` "Viability-referenced distance `vrd/1`" |
| (b) defended anchor | PASS | anchor verdict above: R scores 0.5963-1.3507 vs A0 0.0000, at 1 082-1 305 vs 0 calls |
| (c) discrimination | PASS | D1/D2/D3, min margins +0.8679/+0.5963/+0.5963, 9/9 pairings each |
| (d) robustness | PASS | three single-weight doublings, all PASS |
| (e) candidates + slice | PASS | C1/C2/C3 comparison, table above, `ranking.svg` |

**M2b is answered for this fixture:** the fidelity metric is superseded as an objective by `vrd/1`,
the anchor is the no-AI viability floor, and the measure discriminates recorded arms in the declared
direction. The M2 milestone and the central claim remain **not accepted**; M2c (allocation on the
calibrated instrument) is the next stage and owns the Go implementation of `vrd/1`.
