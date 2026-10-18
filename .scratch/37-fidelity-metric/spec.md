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
