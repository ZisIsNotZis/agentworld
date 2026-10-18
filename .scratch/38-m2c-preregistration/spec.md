# Ticket 38 — M2c part 1: repair the metric's declared pathology, and pre-declare the M2c arm comparison

- Base revision: `8ea4a79` (master, tree clean, nothing pushed).
- Branch: `feature/38-m2c-preregistration`.
- Author: worker agent (ticket 38), under the advance-phase plan.
- Date opened: 2026-10-18T15:40Z.
- **Live model calls in this ticket: 0.** Everything here is a declared formula, a declared
  criterion, a declared run plan, and arithmetic over recorded artefacts.
- **This section is commit 1 and is pre-declared.** Later sections are appended after it and may not
  alter it; a correction, if any, is recorded as a superseding note with its reason. In particular the
  metric choice in §5 and the criterion in §7 were fixed **before** any arm was scored under the
  repaired metric.

## 1. Question, and the prerequisite this ticket owns

`docs/VERTICAL-SLICES.md`, M2c row: *"Under the M2a contract and the M2b metric, does any budgeted
allocation arm separate from the periodic and uniform arms on demographic KPIs?"* — with a named
prerequisite: **"before any arm comparison may be read, M2c must first repair or normalise `vrd/1`'s
birth term — the declared known pathology — and record whether that is a repair or a re-pre-registration
with new weights."**

Ticket 37 declared the pathology (`docs/VALIDATION.md`, `vrd/1`; record
`.scratch/37-fidelity-metric/spec.md` §Known pathology): the birth term outranks the survival term, so a
fully-surviving sterile world scores **1.0000** while an extinct world with one birth (recorded `R` r3)
scores **0.5963**. No declared sensitivity variant removed it.

This ticket therefore does three things and nothing else: (1) it repairs the term as a **design
decision** among materially different normalisations (§3–§6); (2) it **pre-declares the M2c criterion**
(§7); (3) it costs and specifies the run **ticket 39** will execute (§8). It does not run an arm
comparison, and it does not touch the central claim.

## 2. What is fixed before any design choice

Fixed by the ticket and not open here:

- **The priority the metric must obey**, already used to defend `vrd/1`'s anchor: **population survived,
  deaths avoided, stability.** This is an *ordering*, not a tradeoff, and the repair must make the
  metric's ordering agree with it.
- **The anchor.** `T` is the no-AI control `A0`'s own realised outcome at the same fixture, seed and
  horizon; the viability set is `{H ≥ H_T, B ≥ B_T, S ≤ S_T}`; `D = 0` means at or above the floor.
  The anchor is a floor, not an optimum, and `A0` is deterministic with zero live spend.
- **The state vector**, already recorded in every VS2 evaluation artefact and unchanged here:
  `H_x = Σ_{t=0..H} alive_x(t)` actor-hours, `B_x = births_x(H)`, `S_x = starvation deaths_x(H)`,
  `N0 = alive_x(0)`, plus `alive_x(H)` for the persistence predicate below.
- **The validity precondition** (`H_T > 0`, `B_T > 0`, `alive_T(H) > 0`, `N0 > 0`), refused rather
  than defaulted; a fixture whose control is not viable needs its own declared substitute floor and
  may never substitute `R`.
- **Frozen:** the v1/v2 claim-encoding path, the EC-3/EC-4 vector dominance rule, and the ticket
  34/35/36/37 verdicts.

## 3. Why the defect is structural, not cosmetic

`vrd/1` is an equal-weight sum of three normalised shortfalls:

```
vrd/1(x) = d_H + d_B + d_S,  d_H = max(0,(H_T−H_x)/H_T)
                             d_B = max(0,(B_T−B_x)/max(1,B_T))
                             d_S = max(0,(S_x−S_T)/N0)
```

At the recorded fixture `H_T = 1444`, `B_T = 2`, `S_T = 0`, `N0 = 10`.

An additive form has no way to say "this component is decided first". Three consequences, all derived
from the declared formula and the declared priority sentence — none from arm data:

1. **A birth quantum swamps a late-survival deficit.** A world that goes extinct at the last sample
   scores `d_H = 1/1444 = 0.00069`; a world that never reproduces scores `d_B = 1.0` and one birth
   short of the control scores `0.5`. Any weighted sum therefore needs `w_H/w_B > 1/0.00069 ≈ 1444`
   to keep a late extinction ahead of a sterile survivor — and the required ratio grows without bound
   as extinction moves later. **No declared weight ratio can express the declared priority**, so the
   repair cannot be a re-weighting.
2. **One starvation death outranks a small survival deficit.** At equal weights one excess starvation
   death (`1/N0 = 0.1`) outweighs a survival shortfall of `0.096` — the recorded `R` r3 deficit. A
   world that survives the horizon but loses one actor to hunger can rank below one that went extinct.
3. **The tabulated pair is the visible corner of this.** Sterile-but-surviving `1.0000` > extinct-one-
   birth `0.5963` is exactly (1) with `w_H = w_B = 1`.

The defect is the **aggregation rule**, not the arithmetic of the components: ticket 37 said so
("a defect of the objective, not of the arithmetic") and this ticket acts on it.

## 4. Candidates (declared before scoring)

All three keep the state vector, the anchor and the validity precondition. They differ only in how the
three shortfalls become an order.

### P1 — `vrd/2`: precedence-ordered viability (banded lexicographic)

The order is a tuple compared tier by tier, highest priority first:

| tier | component | better | source |
|---|---|---|---|
| 1 | `e(x) = 1` iff `alive_x(H) = 0` (extinct **at the terminal sample**), else `0` | lower | `alive_by_hour[H]` |
| 2 | `d_H(x) = max(0,(H_T−H_x)/H_T)` | lower | `alive_by_hour` |
| 3 | `d_S(x) = max(0,(S_x−S_T)/N0)` | lower | `deaths_starve_cum` |
| 4 | `d_B(x) = max(0,(B_T−B_x)/B_T)` | lower | `births_cum` |

`key(x) = (e, d_H, d_S, d_B)`, ordered lexicographically; `x` is at the floor iff
`key(x) = (0,0,0,0)`. `x` outranks `y` iff `key(x) < key(y)` under that lexicographic order.

Mechanics and why the tiers are these: the priority sentence is read literally. "**Population
survived**" is a predicate first (did the population exist at the horizon) and a quantity second
(actor-hours); actor-hours alone ranks an extinct world above a surviving one whenever the extinct
world accumulated more population-time, which is a bounded-horizon artefact of the aggregate and not a
reading of "survived". "**Deaths avoided**" is the starvation component. "**Stability**" — persistence
without churn — is proxied by reproduction at the control's rate, which is the only stability-relevant
quantity the recorded projections carry; the proxy and its information loss are declared in §6.

Bands: a tier difference counts only when it clears the replicate band (§7.4). Within a band the
comparison descends to the next tier, so the order is not decided by sampling noise.

### P2 — `vrd/2f`: fertility floored, precedence weights in the additive form

Same components, but the fertility term is normalised against a **viability floor** — one birth, the
minimum evidence that the reproduction channel works — rather than against the control's absolute
`B_T`: `b_f = max(0,(1−B_x)/1) ∈ {0,1}`. Aggregation stays additive with declared precedence weights
`w_H = 10, w_S = 1, w_B = 1` ("one order of magnitude"), i.e.
`D2f = 10·d_H + d_S + b_f`.

It is materially different: every world with at least one birth clears the fertility axis entirely, so
`R` r3 (1 birth) rises from a `0.5` deficit to `0`, and the axis stops distinguishing 1 birth from 20.
It keeps a scalar, which the M2c report can carry directly.

### P3 — `vrd/2s`: drop fertility from the distance, report it separately

The distance is over survival and deaths only: `D2s = d_H + d_S`, with births reported as their own KPI
with their own band and **no role in the ordering**. Materially different again: births cannot make an
arm worse, and several arms collapse to the floor.

### What each candidate costs

| candidate | what it gains | what it pays |
|---|---|---|
| P1 `vrd/2` | the declared priority holds **by construction**, at any horizon and for any fixture whose control is viable | no scalar: the metric is a tuple, so a "distance" number is not reported; reproduction can never compensate even one actor-hour of survival |
| P2 `vrd/2f` | keeps a scalar; the fertility axis stops over-weighting a 2-birth control | still an additive sum, so it inherits §3's defect — and §5 shows it fails the declared priority test |
| P3 `vrd/2s` | simplest; the pathology's term is gone | drops reproduction from the defended floor, and with equal weights still lets one starvation death outrank a survival deficit |

## 5. Chosen: P1 `vrd/2` — and it is a **repair**, not a re-pre-registration

**Choice: P1.** It is the only candidate whose ordering agrees with the declared priority by
construction (§6 shows the test), it changes **no weight**, **no component definition**, **no anchor**
and **no validity precondition** — the components are exactly `vrd/1`'s — and it is the smallest change
that removes the defect (the aggregation rule). The `max(1,·)` clamp of `vrd/1` is dropped only because
the validity precondition already guarantees `B_T > 0`, under which the clamp is an identity; nothing
else about the fertility arithmetic changes.

**Repair or re-pre-registration? A repair.** No declared weight is created, changed or removed, because
after the repair there are no weights: the same three numbers are compared in the declared priority
order instead of being summed. The measure's identifier changes to `vrd/2` because its *ordering
function* changed, which is a version bump under `docs/VALIDATION.md`, not a re-weighting.

**What the repair invalidates, stated plainly:**

- **Every recorded `vrd/1` magnitude** (0.5963, 0.7274, 1.0465, 1.1925, …) is a `vrd/1` number and is
  **not comparable** to any `vrd/2` key. `vrd/1` is retained in `docs/VALIDATION.md` as the withdrawn
  objective, with its reason (this section).
- **The three declared sensitivity variants of AC-37(d)** (double each weight in turn) are void: there
  are no weights to double. Their replacement is a declared **precedence sensitivity** — all six
  permutations of tiers 2–4 — plus the terminal-sample convention check (§7.5, results §R4).
- **The `vrd/1` tie rule** ("arms at `D = 0` are tied") becomes "arms with identical keys are tied";
  the same tie is reported, no tiebreak is invented.
- **Not invalidated:** ticket 37's anchor verdict (the R anchor is not a valid objective; the no-AI
  control is), its AC-37(c)/(d) *verdicts*, and its identification of the arm the frozen instrument
  preferred. §R3 verifies each under the repaired metric and records which survive. Because tier 1
  decides those pairs, their margins become categorical rather than numeric.

**The observation that would change this recommendation:**

1. A recorded arm that ties the control on tiers 1–3 and differs only on tier 4 — the recommendation
   stands, but it then means "reproduction is the only axis on which allocation differs", and that is
   reported as such, not as a general separation.
2. A project decision that reproduction may **compensate** a survival deficit (an r-selected world where
   churn is legitimate). That is a change of the priority sentence itself; the metric must then be
   re-pre-registered with that priority, because lexicographic precedence would be wrong.
3. A fixture whose control is sterile at the horizon (`B_T = 0`): `d_B` is then undefined and `vrd/2`
   refuses exactly as `vrd/1` does. A declared substitute floor is required; `R` is never it.
4. A demonstrated need for a single scalar (e.g. a beam controller pruning on one number, `docs/OPEN-QUESTIONS.md` Q1's beam-pruning clause). P1 would then need a declared aggregation of the tuple, and that aggregation is a new pre-registration.

## 6. The declared priority-agreement tests (pre-registered, before scoring)

Four worlds, all stated as state vectors. `W0` and `W2` are recorded; `W1` and `W3` are synthetic and
are declared here so that the test does not depend on any arm.

| world | `H` | `B` | `S` | alive at `H` | what it is |
|---|---|---|---|---|---|
| `W0` | 1444 | 2 | 0 | 1 | the recorded `A0` control (deterministic, 3 identical replicates) — the floor |
| `W1` | 1450 | 0 | 0 | 10 | sterile but fully surviving (ticket 37's pinned pathology world) |
| `W2` | 1305 | 1 | 0 | 0 | recorded `R` replicate 3: extinct at `H`, one birth |
| `W3` | 1444 | 1 | 1 | 1 | survives the horizon, one birth short of the control, one actor lost to starvation |

Declared tests, fixed now:

- **T1 (reproduction of the declared pathology).** `vrd/1` must reproduce `W1 = 1.0000`, `W2 = 0.5963`
  (tolerance `1e-9`) and must rank `W2 < W1`; a failure means the loader or the arithmetic is not the
  declared one.
- **T2 (the tabulated inversion is repaired).** `W1` must outrank `W2` under the chosen metric.
- **T3 (survival outranks deaths and reproduction).** `W3` must outrank `W2`.
- **T4 (the full priority order).** The chosen metric must order the four worlds
  `W0 ≺ W1 ≺ W3 ≺ W2` — survived, then deaths avoided, then reproduction, exactly the sentence.
- **T5 (the floor is exact).** All three recorded `A0` replicates must have key `(0,0,0,0)`.
- **T6 (control identity).** A synthetic copy of `A0` must tie the recorded `A0` exactly.

These six are the *acceptance* of this ticket's deliverable 1. They are arithmetic on declared worlds,
not an arm comparison.

## 7. The pre-declared M2c criterion (M2c-AC)

Fixed here, before any live call of the M2c matrix.

### 7.1 Arms, cells, horizon, replicates

| item | value |
|---|---|
| arms | `A0` (no-AI control, anchor, 0 calls), `P` (periodic), `U` (uniform-random), `A` (adaptive, `vs2-value/2`), `A1` (adaptive, `vs2-value/1` — the same-contract value-model ablation) |
| cell | `f10-s0` (founders 10, seed 0) |
| horizon | `H = 144` (samples 0..144) |
| replicates | 3 per arm |
| decision contract | `natural-claim/v4` (the M2a-passed contract) |
| applied effort | `low`; output bound 32 768; response bound 1 048 576; attempt timeout 300 s; call deadline 900 s; `-parallel 8` |
| budget axis | the declared call ceiling `N = 256` per budgeted arm-run (unchanged) |
| `R` | **excluded** (declared non-goal, §7.7) |

Single cell is declared deliberately: ticket 35's re-plan runs one cell, and the frozen rule's "in both
cells" clause is therefore met only vacuously. That is stated, not hidden; M2d owns the second cell.

### 7.2 KPI set

- **Primary (the order):** `vrd/2`'s four tiers — persistence `e`, actor-hours `d_H`, starvation deaths
  `d_S`, births `d_B`.
- **Reported alongside (diagnostic, not decisive):** population by hour, cumulative births, cumulative starvation deaths, cumulative age deaths, max capital, and the decision-error rate. The ticket-35 demographic curve set is not redefined here; it is re-used as corroboration only.

### 7.3 Cost of the matrix

12 model-bearing runs at `N = 256` calls (`A0` runs 3 times at 0 calls). Ceiling 3 072 physical
attempts, because `AllowCall` is charged per attempt (`internal/cognition/client.go:353`). Expected
spend and the hard cap are declared in §8.

### 7.4 The noise band and the decision rule

Per arm and tier component, the 3 replicates give a mean `mu`, the envelope `[envLo, envHi]`, and the
single-condition envelope half-width `max(envHi−mu, mu−envLo)`. For a pair `(X,Y)` and a tier:

- **Tier 1 (persistence)** separates iff **one arm is extinct in all three replicates and the other
  persists in all three** (unanimity, no noise model needed).
- **Tiers 2–4** separate iff `|mu_X − mu_Y| > B`, where `B` is the **wider of the two arms' envelope
  half-widths**. This is ticket 35's `B(h)` statistic applied to a horizon scalar instead of an hour,
  with the same conservative property: a difference inside either arm's own replicate spread is not a
  separation.

An arm `X` **outranks** `Y` iff the highest tier on which they separate is a tier on which `X` is
better. If no tier separates, the pair is a **tie** and is reported as a tie.

**Verdict rule.**

- **SEPARATION** iff at least one **primary** pair separates: `A` vs `P`, or `A` vs `U`.
- **SECONDARY-ONLY** iff no primary pair separates but at least one secondary pair does: `A1` vs `P`,
  `A1` vs `U`, `A` vs `A1`, `P` vs `U`, or any arm vs `A0`.
- **NO SEPARATION** iff no pair separates on any tier.
- Direction is reported as *more viable under `vrd/2`*, which is the declared objective. It is **not**
  reported as "selective allocation wins": that is the central claim and it is out of reach here.

### 7.5 Corroboration and sensitivity (declared, not decisive)

- **Curve corroboration:** ticket 35's criterion is computed on the demographic KPI curves
  (`SNR = RMSE_h(d)/RMSE_h(sigma) ≥ 2` with a ≥ 24-hour consecutive separation), and its verdict is
  reported beside the tier order. If the two disagree, **both** are reported and the tier order stands,
  because it is the declared objective; a disagreement is itself a recorded finding about the
  instrument.
- **Precedence sensitivity:** all six permutations of tiers 2–4 are computed; any pair whose verdict
  changes under a permutation is reported with the permutation that changes it.
- **Terminal-sample sensitivity:** tier 1 is declared at the terminal sample `h = H`. The variant that
  reads persistence at `h = H−1` is computed and reported; a run that sits on that boundary (`R` r3 is
  the recorded one: 1 alive at h143, 0 at h144) is named.
- **No weight search, no re-tuning.** A verdict produced by a variant that is not the declared one is
  reported as a variant, never as the result.

### 7.6 The falsifier — what makes me report "no separation"

I report **NO SEPARATION** if and only if, under §7.4, every pair among `{P, U, A, A1}` has no
separating tier. This is a decisive, reportable outcome (the M2 exit criterion says so explicitly: "a
clean negative … is an equally decisive and reportable outcome"), not a failure of the ticket.

**Safe way to see the failure that matters** — the ways this could report a false separation:

1. **Evaluator gate (before any live call is read).** The scorer must reproduce every `vrd/1` and
   `vrd/2` value committed in this ticket from the committed recorded vectors, pass T1–T6, tie a
   synthetic copy of `A0` exactly, and — in `--expect-fail` mode — detect the inverted comparison
   (`W2` outranking `W1`) as a FAIL. A scorer that cannot fail is not evidence.
2. **Anchor gate (zero spend, before the live matrix).** The `A0` zero-spend re-run at the landing
   revision must reproduce `H = 1444, B = 2, S = 0` and three identical replicates. A drifting anchor
   voids the run before any score is read.
3. **Contract gate.** Every model-bearing cell must record **0 `inadmissible`** answers. A nonzero
   count voids that cell: ticket 35 showed a matrix under a lossy contract measures admissibility luck,
   not allocation quality.
4. **Band gate.** No difference smaller than the §7.4 band may be called a separation, in either
   direction; ties are printed as ties.

### 7.7 What M2c may NOT conclude

- **Not the central claim.** Selective allocation beating uniform/periodic at equal budget is *not*
  established by one cell. The central claim needs the selective-vs-uniform comparison under a **spend**
  budget across cells and seeds, with the marginal value of reasoning measured — M2d/M4 territory. A
  separation here says only that two arms differ in viability at one cell under one contract.
- **No cross-cell, cross-seed or cross-fixture generality.** One cell; the "both cells" clause is
  vacuous; M2d owns replication.
- **No claim outside `vrd/2`'s order.** Nothing about identity, kin or diversity of survivors, age-death
  timing, wealth or granary distributions, exchange, action mix, or the decision-error rate as an
  objective — `vrd/2` does not represent them.
- **No depth/effort claim** (effort is fixed at `low`), **no real-world fidelity claim**
  (model-conditional, P14), **no re-opening of tickets 34/35/36/37**, and **no override of the frozen
  EC-3/EC-4 dominance rule**.
- **No "better" language beyond the declared objective**: the report says "more viable under `vrd/2`",
  never "the better policy".

## 8. The run plan ticket 39 executes (costed)

### 8.1 Counts, tokens, dollars

Recorded basis, from the ledgers and reports of the runs this matrix re-uses:

| source | calls | input tok | output tok | $ at $0.10/M in, $0.50/M out |
|---|---|---|---|---|
| ticket 36 `R` @ v4, H=144 (one cell-replicate) | 1 325 | 824 408 | 234 007 | **$0.199** |
| ticket 35 v2 `P` (per replicate) | 244 | 324 961 | 107 504 | $0.086 |
| ticket 35 v2 `U` (per replicate) | 256 | 342 936 | 130 839 | $0.100 |
| ticket 35 v2 `A` (per replicate) | 256 | 341 904 | 124 489 | $0.096 |
| ticket 35 v2 `A1` (per replicate) | 253 | 336 966 | 114 237 | $0.091 |

Two bounds, both derived from recorded data rather than assumed:

- **Lower bound — scale the v4 full-wake measurement by call count.** `$0.199 × 3 072 / 1 325 = $0.46`.
  This assumes budgeted arms' calls are as cheap as `R`'s (`622` in / `177` out per call).
- **Upper bound — use the recorded per-arm token profiles.** Budgeted arms' recorded calls carried
  ~1 332 input and ~440 output tokens each; scaling the input side by the v4/v2 input ratio measured on
  `R` (`622/443 = 1.40`) gives ≈`1 870` in / ≈`460` out per call, i.e. ≈`$4.2e-4` per call. Then
  `244·3 + 256·3 + 256·3 + 253·3 = 3 027` calls → **≈ $1.28**.

**Hard cap for ticket 39: $2.00, and 4 000 physical attempts.** Justification: `$1.28` is the
pessimistic recorded-token estimate and `$2.00` is 1.56× it, so the cap absorbs a retry storm (ticket
34's EC-2 halt was caused by retries charged per attempt) without reaching the user's money in any
plausible outcome; the attempt cap of `3 072` (12 × 256) is enforced by the harness itself and 4 000
leaves room for the three `A0` runs and one re-run of a cell. The project has ≈$30 of headroom under
the user's $50 cap and the user said lesser is better, so the cap is stated as a ceiling on one ticket,
not a budget to spend. **The harness enforces calls, not dollars** (`budgetFor` sets
`MaxCostMicros: 1<<60` and the report records cost as UNPRICED because the gateway exposes no price
entry): the dollar cap is therefore enforced by the operator's poll rule below, and that limit is
stated rather than implied.

**Stop rule (operator, during the run):** every 15 minutes, compute spend from the recorded `usage`
rows of every `calls.jsonl` at the declared prices. If realised spend exceeds **$1.60**, or the
realised rate projects a total above **$2.00**, stop the runner and report — the run then stands as a
partial matrix and is reported as incomplete, never scored as a smaller result.

### 8.2 Wall clock

Measured `R`@v4 lane rate: 15.8–18.2 calls/min at `-parallel 8` (1 325 calls in 84 min). 3 072 attempts
→ **169–194 min**. Declared: **3–4 h expected, deadline 6 h**; `-budget-min 720` so the harness's own
wall-clock budget never truncates the matrix before the operator's deadline. ETA is model-latency
bound, not CPU bound; the machine is not the constraint.

### 8.3 Detachment and liveness

- Launched detached with **`setsid nohup`**; the runner pid is the shell's `$!`, recorded in the ticket
  when the run starts.
- **Liveness = the runner pid still exists AND every `calls.jsonl` under the run root grows.** Never a
  pid file, never log content (the report is written only at exit, so an empty log proves nothing).
- Polling every 15 min; on exit, the evaluation JSON's `halted` / `under_budget` fields and the EC
  bands are read before anything is scored.

### 8.4 The exact commands

```sh
# --- 0. toolchain (the toolchain lives in the main worktree) ---
. /home/z/vibe/agentworld/.tmp/go-env.sh     # go1.24.13, GOTOOLCHAIN=local

# --- 1. gates at the landing revision (no Go change is expected in ticket 39 either) ---
go build ./... && go vet ./... && git ls-files -z '*.go' | xargs -0 gofmt -l && git diff --check 8ea4a79

# --- 2. evaluator gate: the scorer must reproduce this ticket's committed values first ---
python3 .scratch/38-m2c-preregistration/vrd2.py --self-test \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/38-m2c-preregistration/reproduction-39.log

# --- 3. anchor gate: zero-spend A0 re-run at the landing revision ---
go build -o .tmp/39-vs2-bin ./cmd/vs2
.tmp/39-vs2-bin -arms A0 -cells f10-s0 -hours 144 -replay=false \
  -claim-contract natural-claim/v4 -root .tmp/39-a0-runs \
  -out .tmp/39-a0-evaluation.json -out-txt .tmp/39-a0-evaluation.txt
# expect H=1444 B=2 S=0 alive@143=2 alive@144=1, three identical replicates, 0 calls

# --- 4. the live matrix (spend is authorised for this run only) ---
VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND \
LITELLM_BASE_URL=http://127.0.0.1:4000 \
LITELLM_MODEL=github_copilot/gpt-6-luna \
setsid nohup .tmp/39-vs2-bin \
  -arms A0,P,U,A,A1 -cells f10-s0 -reps 3 -hours 144 \
  -effort low -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 8 -budget-min 720 \
  -root .tmp/39-m2c-runs \
  -out .tmp/39-m2c-evaluation.json -out-txt .tmp/39-m2c-evaluation.txt \
  > .tmp/39-m2c-run.log 2>&1 &
echo "runner pid $!"

# --- 5. liveness (every poll; pid + ledger growth, never a pid file, never log content) ---
ps -p <runner pid> >/dev/null && wc -l .tmp/39-m2c-runs/*/calls.jsonl | tail -1

# --- 6. scoring (after exit 0; the same scorer, over the live evaluation JSON) ---
python3 .scratch/38-m2c-preregistration/vrd2.py \
  --live .tmp/39-m2c-evaluation.json \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/38-m2c-preregistration/m2c-scores.log
```

The gate order is not cosmetic: **step 2 before step 4** is the M2c row's requirement ("M2c owns the
evaluator implementation of `vrd/1` and must reproduce the recorded scores before using it on a live
run"), and **step 3 before step 4** is the anchor's reproducibility requirement. Both are zero spend.

### 8.5 The anchor's reproducibility, checked here rather than asserted

`vrd/2`'s anchor `A0` is script-only: `-arms A0` makes **zero** model calls, and the cognition seam is
nil-tolerant, so the no-cognition world is untouched by the claim contract. This ticket confirms that
at the landing revision, with zero spend (§R1): the `A0` re-run reproduces the committed ticket-37
vector `H = 1444, B = 2, S = 0`, and running the same `A0` cell under `-claim-contract natural-claim/v4`
produces **byte-identical projections** to `natural-claim/v1` — so running the AI arms under `v4` does
not change the anchor, and the anchor costs nothing to re-measure inside the live matrix.

## 9. What would make this ticket fail

- A chosen metric whose ordering does **not** satisfy T2–T4 on the declared worlds.
- A claim that the repair is a re-weighting, or silence about which recorded numbers it invalidates.
- A criterion that could not fail (no declared falsifier, no band, no negative control), or that lets
  M2c speak for the central claim.
- A run plan whose cap is not derived from recorded cost data, or whose anchor is assumed rather than
  re-measured.
- Any live call. **This ticket's budget is 0 calls, $0.00.**

---

# Results (appended after commit 1; the pre-declaration above is unchanged)

Everything below was produced after the pre-declaration commit `6072af3`, with **0 live model calls**.
Files committed beside this record: `vrd2.py` (the scorer), `self-test.log`, `recorded.log`,
`negative-control.log`, `live-smoke.log`, and `anchor-check.json`. Inputs: this ticket's own
`vrd2.py`, ticket 37's committed `.scratch/37-fidelity-metric/scores.json` (the recorded per-cell
vectors), and two zero-spend `cmd/vs2` runs of the `A0` cell.

## R1 — The anchor is reproducible with zero spend, and the claim contract cannot move it

The `A0` arm is script-only: 0 model calls, no cognition, deterministic. Re-measured at the landing
revision (`8ea4a79` + this branch, no Go change), two ways:

| run | command | result |
|---|---|---|
| contract `natural-claim/v1` | `.tmp/38-vs2-bin -arms A0 -cells f10-s0 -hours 144 -replay=false -root .tmp/38-a0-runs -out .tmp/38-a0-evaluation.json -out-txt .tmp/38-a0-evaluation.txt` | exit 0, `H = 1444`, `B = 2`, `S = 0`, 11 age deaths, alive@143 = 2, alive@H = 1, 145 samples |
| contract `natural-claim/v4` | the same cell with `-claim-contract natural-claim/v4` | **byte-identical projections** on the eight compared fields (`hours`, `alive_by_hour`, `births_cum`, `deaths_starve_cum`, `deaths_age_cum`, `capital_hist`, `granary_hist`, `wages_cum`) |

Both reproduce ticket 37's committed anchor exactly (`H_T = 1444`, `B_T = 2`, `S_T = 0`, `N0 = 10`,
alive@143 = 2, alive@H = 1), and `anchor-check.json` records the commands, the vector and the sha256 of
each ephemeral evaluation JSON. **Therefore: the anchor is reproducible under the same fixture and
horizon without any live spend, and running the AI arms under `natural-claim/v4` does not change it** —
the contract shapes the model-facing request bytes at the seam, which a no-cognition arm never builds.
Ticket 39 re-runs the same cell as a pre-run gate (§8.4 step 3) rather than trusting this.

## R2 — The repair passes every declared ordering test; both alternatives fail one

`python3 .scratch/38-m2c-preregistration/vrd2.py --self-test --scores .scratch/37-fidelity-metric/scores.json`
— exit 0, transcript `self-test.log`. The transcript's first block, verbatim:

```
T1 vrd/1 reproduces the declared pathology on W1/W2:
   W1=1.0000  W2=0.5963  W3=0.6000  W0=0.0000
   PASS: W1 == 1.0000 and W2 == 0.5963 and W2 < W1
   vrd/2 key W0 = (e=0, dH=0.0000, dS=0.0000, dB=0.0000)
   vrd/2 key W1 = (e=0, dH=0.0000, dS=0.0000, dB=1.0000)
   vrd/2 key W2 = (e=1, dH=0.0963, dS=0.0000, dB=0.5000)
   vrd/2 key W3 = (e=0, dH=0.0000, dS=0.1000, dB=0.5000)
T2 PASS: W1 (sterile survivor) outranks W2 (extinct, one birth)
T3 PASS: W3 (survivor, one starvation death) outranks W2
T4 PASS: the full order is W0 < W1 < W3 < W2 (declared W0 < W1 < W3 < W2)
T5 PASS: the recorded A0 control is exactly at the floor
T6 PASS: a synthetic copy of A0 ties the recorded A0 exactly
```

`vrd/2` orders the four worlds exactly as the priority sentence does — **survived** (all but `W2`;
`W2` extinct), then **deaths avoided** (`W0`/`W1` lose nobody to hunger, `W3` loses one), then
**stability/reproduction** (`W0` at the control's rate, `W1` sterile). The pathology is gone at the
pair ticket 37 tabulated, and at the second inversion (`W3` vs `W2`) that the same defect produces.

The two rejected candidates, on the same four worlds (same transcript):

```
   P2 vrd/2f: W0 < W3 < W2 < W1
   order P2: W0=0.0000  W1=1.0000  W2=0.9626  W3=0.1000
   P2 T2 FAIL (W1 1.0000 vs W2 0.9626); required weight ratio w_H/w_B > 1/0.09626 = 10.39
   P3 vrd/2s: W0 < W1 < W2 < W3
   order P3: W0=0.0000  W1=0.0000  W2=0.0963  W3=0.1000
   P3 T2 PASS (W1 0.0000 vs W2 0.0963); P3 T3 FAIL (W3 0.1000 vs W2 0.0963)
```

- **P2 fails T2** at its declared 10:1 weight ratio, and the failure is quantitative rather than
  incidental: the ratio needed to rescue it is `1/d_H(extinction) = 10.39` at `R` r3 — a number read
  off the world being judged — and it grows without bound as the extinction moves later. That is §3's
  argument, measured.
- **P3 repairs the tabulated pair but fails T3**: dropping the birth term leaves an equal-weight sum in
  which one starvation death (`0.1`) still outranks a survival shortfall (`0.0963`), so an extinct
  world still beats a surviving one. It also flattens every ≥1-birth survivor to the floor, which
  discards the anchor's declared birth component.

**`vrd/2` is therefore the only candidate of the three whose ordering agrees with the declared
priority on all four worlds, which is what T2–T4 require.**

## R3 — The recorded matrix: the repair changes what the metric means, and no recorded ordering

`python3 .scratch/38-m2c-preregistration/vrd2.py --scores .scratch/37-fidelity-metric/scores.json` —
exit 0, transcript `recorded.log`. This is **verification, not the M2c comparison**: these are ticket
35's `natural-claim/v2` arms, which by the M2c row "do not answer" M2c.

- **Loader identity: exact.** Recomputing `vrd/1` from the committed per-cell vectors reproduces every
  recorded value with max absolute error `0.000e+00` (tolerance `1e-12`). The scorer reads the same
  state vector ticket 37 read.
- **Per-replicate vrd/2 keys** (all 18 cells in `recorded.log`); the arm aggregates (tier 1 = persisted
  in every replicate, `mu` = replicate mean, `±` = the single-condition envelope half-width):

| arm | persisted | `mu d_H` | `mu d_S` | `mu d_B` |
|---|---|---|---|---|
| A0 | yes | 0.0000 ± 0.0000 | 0.0000 ± 0.0000 | 0.0000 ± 0.0000 |
| P | yes | 0.0000 ± 0.0000 | 0.0000 ± 0.0000 | 0.0000 ± 0.0000 |
| A | yes | 0.0418 ± 0.0829 | 0.0667 ± 0.1333 | 0.0000 ± 0.0000 |
| U | yes | 0.2274 ± 0.0577 | 0.5000 ± 0.1000 | 0.0000 ± 0.0000 |
| R | **no (extinct 3/3)** | 0.1798 ± 0.0836 | 0.0333 ± 0.0667 | 0.8333 ± 0.3333 |
| A1 | **no (extinct 3/3)** | 0.1925 ± 0.0000 | 0.0000 ± 0.0000 | 1.0000 ± 0.0000 |

- **Orders under the four measures, recorded arm means:**

| measure | order (more viable first) |
|---|---|
| `vrd/1` (withdrawn) | A0(0.0000) < P(0.0000) < A(0.1084) < U(0.7274) < R(1.0465) < A1(1.1925) |
| `vrd/2` (declared) | A0 < P < A < U < R < A1 |
| P2 `vrd/2f` (rejected) | A0(0.0000) < P(0.0000) < A(0.4845) < **R(2.4982) < U(2.7738)** < A1(2.9252) |
| P3 `vrd/2s` (rejected) | A0(0.0000) < P(0.0000) < A(0.1084) < **A1(0.1925) < R(0.2132) < U(0.7274)** |

- **`vrd/1` → `vrd/2`: 0 of 15 arm pairs change relative order** (arm means). At replicate level the
  scorer recomputes every cell pair and reports `replicate-level comparisons the repair corrects: 2 of
  153 cell pairs`, verbatim in `recorded.log`: `R`/r3 (extinct at the terminal sample after 1 305
  actor-hours, one birth) against `U`/r1 (`0.7285`) and `U`/r3 (`0.8839`) — both persisting cells with 5
  and 6 starvation deaths. `vrd/1` ranked the extinct replicate better in both; `vrd/2` ranks the
  persisting cell better, because tier 1 now decides before actor-hours. That is the priority sentence
  applied to recorded data and the one place the repair is not order-preserving — deliberately. The
  repair changes what the numbers *mean* (a lexicographic key instead of an additive total, extinction
  categorical) without moving any recorded **arm**; the two corrected comparisons are exactly the class
  of inversion §3 item 2 describes. The two rejected candidates do move arms, which is what makes them
  materially different rather than cosmetic: P2 inverts `R`/`U`; P3 moves `A1` from last to third,
  inverting `R`/`A1`, `U`/`A1` and `R`/`U`.
- **Ticket 37's declared pairs, re-checked under `vrd/2` over every 3×3 replicate pairing:** D1
  (`A1`→`A`), D2 (`R`→`A0`) and D3 (`R`→`P`) each PASS 9/9, and **all 27 pairings are decided at tier 1**
  (the worse arm is extinct in every replicate, the better arm persists in every replicate). The
  margins stop being numbers and become categorical.
- **Precedence sensitivity (the replacement for AC-37(d)'s weight doublings):** 4 of the 6 tier orders
  reproduce the recorded order exactly; the two that differ (`S/H/B`, `S/B/H`) put `A1` above `R` —
  i.e. only the relative order of the two extinct arms is precedence-sensitive, and no surviving arm
  moves under any permutation.
- **Terminal-sample sensitivity:** reading persistence at `h = H−1` instead of `h = H` moves `R` above
  `U` (order `A0 < P < A < R < U < A1`), because `R` r3 is the boundary case — 1 alive at h143, 0 at the
  terminal sample. The convention is declared at `h = H`, the same sample `vrd/1` used for `H` and `B`;
  the variant is recorded here so the boundary is visible rather than silent.

## R4 — The criterion's machinery can fail, and one `--live` CLI path works

- **Band-rule fixtures** (`self-test.log`), exercising §7.4 in both directions:
  `fixture a` (an arm whose replicate spread covers the mean difference) → **NO SEPARATION**;
  `fixture b` (extinct vs persisting, unanimous) → **SEPARATION** at tier 1;
  `fixture c` (four starvation deaths beyond the survival band) → **SEPARATION**;
  `fixture d` (identical arms) → **NO SEPARATION**; `fixture e` (an arm with more actor-hours and more
  starvation deaths than the other) → **SEPARATION**, with the declared precedence sensitivity **firing
  on 3 verdicts**, i.e. the sensitivity machinery detects a case where tier order decides the answer. A
  rule that always separates fails `a`/`d`; a rule that never separates fails `b`/`c`; a sensitivity
  machine that can only print zero fails `e`.
- **Negative control** (`python3 … --expect-fail --scores …`, exit 0, transcript
  `negative-control.log`): the inverted claims `W2 < W1`, `W2 < W3`, `W2 < W0` and `A1 < A` are each
  required to **hold = False**, and the run prints `NEGATIVE CONTROL PASS` only if all four do.
- **`--live` CLI smoke** (`live-smoke.log`) against a three-arm synthetic artefact at
  `/tmp/38-live-smoke.json` (shape and plumbing only, **not** a result about any arm): the band rule
  reports a deterministic-vs-deterministic difference as a structural separation, a
  deterministic-identical pair as a tie, an absent arm as `NOT RUN — an arm is absent from the
  artefact` rather than a pass, and the two declared sensitivity blocks (six tier orders, `h=H−1`
  persistence) are emitted for every declared pair. Ticket 39's scoring command is therefore a command
  that runs and produces every quantity §7.5 promises, not only the verdict.

## R5 — Which of ticket 37's recorded conclusions survive

| ticket-37 conclusion | under `vrd/2` | why |
|---|---|---|
| `fid/1` is not a valid objective; the R anchor rewards extinction | **survives** | unchanged; `R` is now ranked below every persisting arm categorically (tier 1) |
| the no-AI control `A0` is the defended anchor, a floor not an optimum | **survives** | anchor, viability set and validity precondition untouched; `A0`'s key is exactly `(0,0,0,0)` |
| AC-37(c) discrimination: D1/D2/D3 pass in every replicate pairing | **survives, strengthened** | 9/9 each, all decided at tier 1; the margins are categorical instead of numeric |
| AC-37(d) robustness under three one-at-a-time weight doublings | **void, replaced** | there are no weights; the declared replacement is the six tier permutations (R3), under which no surviving arm's order moves |
| AC-37(e): the arm the old instrument called closest to R (`A1`) is the least viable | **survives** | `A1` is extinct in 3/3 with 0 births; it ranks last under `vrd/2` |
| recorded `vrd/1` magnitudes (0.5963, 0.7274, 1.0465, 1.1925, …) | **invalidated as comparable numbers** | they are `vrd/1` values; the `vrd/2` key is not a scalar and no `vrd/1` number may be compared with it. The recorded **arm** order is invariant, and 2 of 153 replicate comparisons are corrected as §3 item 2 requires (R3) |
| the pathology itself | **withdrawn as current fact, kept with its reason** | `docs/VALIDATION.md` records `vrd/1` as withdrawn and why; the reason (an additive form cannot express a priority order) is what stops the same defect returning |
| ties at `D = 0` are reported as ties | **survives, restated** | ties are now identical keys; `A0` and all three `P` replicates share `(0,0,0,0)` and are reported tied |

Nothing in ticket 37's *verdict* is overturned, and this ticket re-opens none of tickets 34–37.

## R6 — Gates, limits, and what this ticket did not do

- **Gates.** No Go file changed on this branch (`git diff --stat 8ea4a79..HEAD -- '*.go' cmd/ internal/
  Makefile go.mod go.sum` is empty; the branch touches `.scratch/`, `docs/` and `CHANGELOG.md` only), so
  the full Go suite is **not claimed** — it would re-measure unchanged code. Run and recorded:
  `go build ./...` (exit 0, needed anyway to build the zero-spend `A0` binary), `go vet ./...`,
  `gofmt -l` over tracked `.go` files, and `git diff --check 8ea4a79` — see the gate table below.
- **Live model calls: 0. Spend: $0.00.**
- **Limits of the repair.** `vrd/2` cannot express a tradeoff between tiers: a survival difference of
  one actor-hour outranks any reproduction difference, by construction. Its "stability" tier is a
  reproduction proxy, not a variance/churn measure, and it inherits every information loss ticket 37
  declared for `vrd/1` (identity, kin and diversity of survivors, age-death timing, wealth and granary
  distributions, exchange, action mix, decision-error rate). Its persistence predicate is read at the
  terminal sample, and R3 records the boundary case that convention creates.
- **Not done, deliberately.** No live call; no arm comparison under `vrd/2` beyond the recorded v1/v2
  matrix; no Go evaluator change; no claim about any policy; no re-opening of tickets 34/35/36/37; no
  push.

## Gate table

| # | check | exact command (cwd = branch worktree) | result |
|---|---|---|---|
| 1 | build | `. /home/z/vibe/agentworld/.tmp/go-env.sh && go build ./...` | exit 0 — PASS |
| 2 | vet | `go vet ./...` | exit 0 — PASS |
| 3 | gofmt on tracked `.go` | `git ls-files -z '*.go' \| xargs -0 gofmt -l` | no output, exit 0 — PASS |
| 4 | whitespace/conflict | `git diff --check 8ea4a79` | exit 0 — PASS |
| 5 | scorer self-test + declared tests | `python3 .scratch/38-m2c-preregistration/vrd2.py --self-test --scores .scratch/37-fidelity-metric/scores.json --log …/self-test.log` | exit 0, `SELF-TEST: PASS` |
| 6 | recorded verification | `… --scores .scratch/37-fidelity-metric/scores.json --log …/recorded.log` | exit 0, loader identity `0.000e+00` |
| 7 | negative control | `… --expect-fail --scores …` | exit 0, `NEGATIVE CONTROL PASS` |
| 8 | `--live` CLI smoke | `… --live /tmp/38-live-smoke.json --scores …` | exit 0 |
| 9 | anchor reproduction, 0 spend | two `cmd/vs2 -arms A0` runs under `natural-claim/v1` and `natural-claim/v4` | exit 0 both; vectors identical and equal to the committed anchor (`anchor-check.json`) |
| 10 | full Go suite | `go test -timeout 30m ./...` | **not run — no Go file changed on this branch**; recorded rather than claimed |
| 11 | `make race` | — | **not run** — not in this ticket's gate list |

## Revision and commands

- Base `8ea4a79`; pre-declaration commit `6072af3` on `feature/38-m2c-preregistration`.
- Scoring commands, verbatim (all exit 0):

```
python3 .scratch/38-m2c-preregistration/vrd2.py --self-test \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/38-m2c-preregistration/self-test.log
python3 .scratch/38-m2c-preregistration/vrd2.py \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/38-m2c-preregistration/recorded.log
python3 .scratch/38-m2c-preregistration/vrd2.py --expect-fail \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/38-m2c-preregistration/negative-control.log
python3 .scratch/38-m2c-preregistration/vrd2.py --live /tmp/38-live-smoke.json \
  --scores .scratch/37-fidelity-metric/scores.json \
  --log .scratch/38-m2c-preregistration/live-smoke.log
```
