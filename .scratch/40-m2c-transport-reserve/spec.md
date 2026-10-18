# Ticket 40 — M2c part 2b: give the quota schedule a declared transport reserve, and re-pre-register the matrix

- Base revision: `279f0ba` (master, ticket 38 merged; ticket 39's branch is the working base of this worktree).
- Branch: `feature/40-m2c-transport-reserve`.
- Author: worker agent (ticket 40), under the advance-phase plan.
- Inputs, in order: ticket 39 `.scratch/39-m2c-matrix/spec.md` §11 (the diagnosis, bound to the committed
  evaluation JSON `sha256 56d3b300…4df26` and the run root `.tmp/39-m2c-runs`), then the supervisor
  decision recorded in §2 below.
- This ticket **makes the code change and the re-pre-registration**. It does **not** run the live matrix;
  the re-run is ticket 41, and until it lands the M2c verdict is `NOT RUN` (ticket 39 §10.5).

## 1. What this ticket is

Ticket 39 ran the M2c matrix once, spent `$0.6007` and 3 020 physical attempts, and could not compute a
verdict: cell `A1-f10-s0-r1` halted on `cognitive-calls` at hour 143 with `threshold 256 observed 256`.
§11 diagnosed the cause as a **plan** error, not an infrastructure fault and not a legitimate arm outcome:

1. the declared schedule spends **100 %** of `N` by construction — `q(h)` is defined so that
   `Σ_{h<H} q(h) = N`, so a *completing* `U`/`A`/`A1` cell also ends on exactly its last permitted attempt;
2. the schedule counts **decisions** while the account charges **physical attempts** (`AllowCall` is
   charged per attempt, once per retry inside `Recorder.Call`), and one retried 300 s timeout therefore
   bought no decision while costing a decision's worth of budget;
3. at zero headroom, that single retryable transport event truncates the cell; eight of the twelve
   model-bearing cells finished at exactly 100 % of the ceiling, so the margin was one HTTP status code.

Ticket 34 recorded the same class of halt and left the reading to a milestone review; ticket 39 §11.5
costed seven repairs and stated the one question it could not settle. That reading is now made (§2).

## 2. The decision this ticket records (taken by the product owner; not re-opened here)

**Chosen: option v-b — keep the frozen instrument, give the schedule a declared transport reserve.**

- The per-cell attempt ceiling stays `DeclaredCallCeiling = 256` (`cmd/vs2/config.go`), and `VS2-EC-2`'s
  criterion — "an (N+1)-th attempt is the typed fail-loud halt" (`cmd/vs2/bands.go:139`) — is **untouched**,
  as is the halt path (`internal/cognition/budget.go` `AllowCall`/`Charge`, `internal/cognition/client.go`
  retry loop, `cmd/vs2/run.go` `halted`).
- What changes is the **schedule**: it selects at most `N − R` decisions and leaves `R` attempts unconsumed
  for retries. `R = 8`; the arithmetic is in §3.1.
- The reserve applies identically to every budgeted arm, so the equal-budget comparison that `VS2-EC-2`
  enforces is preserved.
- **Why v-b and not v-a**: v-a (`N` = decisions, attempts = `N + R`) achieves the same protection but edits
  the frozen `VS2-EC-2` exit criterion of ticket 34, which ticket 38 §7.7 forbids without user authority.
  v-b reaches the same place inside the instrument that already exists.

**This is a re-pre-registration, and it is legitimate on one ground only**: no verdict has ever been
computed for M2c. The only artefact is `NOT RUN` (`§5`/`.scratch/39-m2c-matrix/spec.md` §10.5), so nothing
is being reworded to fit data — there is no data to fit. Ticket 38 §7.4–§7.7 (band, per-tier effect floors,
unanimity, falsifier, inventory precondition, exclusions) is **unchanged in text and in value**; only the
declared budget axis and the cost arithmetic that follows from it change. Every changed sentence is listed
in §6 with its before and after.

## 3. The constant, its home, and its arithmetic

### 3.1 `R = 8`

`internal/cognition/priority.go` declares it once:

```go
const DeclaredTransportReserveAttempts uint64 = 8
```

The arithmetic, from the declared harness parameters rather than from a guess: `DefaultMaxAttempts = 3`
(`internal/cognition/client.go:52`), so one decision can consume at most 3 physical attempts. `R = 8`
covers **two fully retried calls (2 × 3 = 6) plus two single retries (2 × 1 = 2)**, i.e. 6 + 2 = 8. It is
a reserve, not a second ceiling: it is spent only by retries, and a cell that never retries must finish
with all `R` attempts intact (§4.2 test 2).

### 3.2 The derivation has one home

```go
func DecisionBudget(ceiling uint64) uint64 // N - R, floored at 0
```

Both the live selector (`PrioritySelector.remaining`, `PrioritySelector.quota`) and the runner's recorded
schedule (`cmd/vs2/config.go` `quotaSchedule`, which feeds `evaluation.quota_schedule` in the report) call
this one function, so the arm's schedule and the recorded schedule cannot drift; a test pins the two
together (`TestQuotaScheduleMirrorsSelector`).

The schedule is now

```
q(h) = floor((N-R)/H) + (1 if h < (N-R) mod H else 0)   ⇒   Σ_{h<H} q(h) = N - R
```

which for the declared matrix (`N = 256`, `H = 144`) is `q(h) = 2` for `h < 104`, `1` for `h ≥ 104`,
summing to **248**.

### 3.3 What deliberately does not change

| thing | why it stays |
|---|---|
| `DeclaredCallCeiling = 256` | the fail-loud attempt ceiling is the frozen instrument; the reserve lives inside it |
| `VS2-EC-2`'s criterion string and the `(N+1)`-th-attempt halt | ticket 38 §7.7 forbids re-opening tickets 34–37 without user authority |
| `AllowCall`/`Charge`, the `Recorder` retry loop, `run.go`'s `halted` path | the halt path is untouched, so it still fires exactly as declared (§4.2 test 4) |
| the protected-exploration reserve `E = ceil(q·N)` | it is a declared fraction of the **ceiling**, not of the reduced decision budget; at `q = 25 %`, `N = 256`, `E = 64 ≤ 248`, so the schedule can still supply it in full (pinned by `TestProtectedExplorationBound`, unchanged) |
| the `A0` anchor, `vrd/2`, the `natural-claim/v4` contract, `H`, the replicates | no bearing on the budget axis |

## 4. The code change

### 4.1 Files and seams

| file | change |
|---|---|
| `internal/cognition/priority.go` | `DeclaredTransportReserveAttempts = 8`; `DecisionBudget(N)`; `PrioritySelector.decisions` seeded at construction; `remaining()` and `quota()` spend `decisions`, not `callCeiling`; constructor refuses a ceiling at or below `R` (a cell that could make no decision is refused, not silently degenerate) |
| `internal/cognition/protocol/priority.go` | the `EscalationSelector` doc sentence "stops once it has spent N" → "stops once it has spent its decision budget, the declared call ceiling N less the declared transport reserve" — the interface comment restated the old invariant |
| `cmd/vs2/config.go` | `quotaSchedule` divides the **decision** budget, so the recorded `quota_schedule` sums to 248; `DeclaredCallCeiling` and `budgetFor` docs name the reserve and the pre-N stop |
| `internal/cognition/priority_test.go` | expectations that encoded `Σq = N` and "spends exactly N" now read `DecisionBudget(N)`; two tests renamed to match what they pin (§4.3) |
| `internal/cognition/priority_reserve_test.go` | new: the derivation and its floor |
| `cmd/vs2/accounting_test.go` | new: the declared `(N, R) = (256, 8)` pair and three joint cell tests (§4.2) |

Report-side consequences, all expected and none a gate failure: a quota-bound cell now reports
`realized_calls ≈ 248`, `under_budget: true` and `budget_shortfall 8`, and `VS2-EC-2`'s band detail counts
it in `under-budget` (that counter is diagnostic; the band fails only on `over-ceiling`, `missing-ceiling-rows`
or `ceiling-halts`, `cmd/vs2/bands.go:168-172`). `P` is unaffected in kind: its period gate supplies fewer
decisions than the decision budget, so the reserve is not its binding stop (`TestPeriodicArmReserveIsNotItsBindingStop`).

### 4.2 The tests that pin the repair (all in `cmd/vs2/accounting_test.go`)

| test | what it pins | result |
|---|---|---|
| `TestDeclaredScheduleLeavesTheTransportReserve` | the declared pair: `N = 256`, `R = 8`, `Σq = 248`, and `248 + 8 = 256` | PASS |
| `TestCellWithoutRetriesLeavesTheReserveUnspent` | `U`/`A`/`A1`: 248 decisions in 248 attempts, no halt, **exactly `R` attempts unspent** | PASS |
| `TestCellRetryingWithinTheReserveStillCompletes` | `U`/`A`/`A1`: 8 retries spread inside the reserve → 248 decisions, 256 attempts, **no halt**, 0 unspent | PASS |
| `TestReservePlusOneRetryStillHaltsTypedOnTheNPlusOnethAttempt` | `A1` with `R + 1` retries: the 257th attempt is refused with `Ceiling: cognitive-calls, Threshold: 256, Observed: 257` in the last hour, **after all 248 decisions were made** — `VS2-EC-2`'s halt fires exactly as before | PASS |
| `TestPeriodicArmReserveIsNotItsBindingStop` | `P`'s gate binds first; it ends with at least `R` unspent and never halts | PASS |

The two tests that exercise `VS2-EC-2`'s halt directly are **unchanged** and still green:
`internal/cognition/budget_test.go` `TestBudgetAllowCallRefusesBeforeSpend` and `internal/cognition/client_test.go`
`TestRecorderBudgetCeilingHaltsBeforeSpend`. No file under `internal/cognition/budget.go`, `client.go`,
`ledger.go`, `cmd/vs2/bands.go` or `cmd/vs2/run.go` is touched, which is the mechanical form of "the
instrument is untouched".

### 4.3 Renamed tests (name now matches what is pinned)

| old | new |
|---|---|
| `TestPrioritySelectorExactNOnAmpleStream` | `TestPrioritySelectorExactDecisionBudgetOnAmpleStream` |
| `TestPrioritySelectorStopsAtNAndHonoursBudget` | `TestPrioritySelectorStopsAtDecisionBudgetAndHonoursBudget` (its `N = 2` fixture became `N = 10`, which declares the same two-decision schedule at `R = 8`) |
| `TestArmStopsAtCeiling` (cmd/vs2) | `TestArmStopsAtDecisionBudget` |

### 4.4 The negative control (gate probe)

The new tests must be able to fail on the defect they repair. Bounded mutation, isolated to one line:
`DeclaredTransportReserveAttempts` temporarily set to `0` (which reproduces the pre-fix schedule exactly),
`go test` on three of the new tests, then restored:

```
--- FAIL: TestDeclaredScheduleLeavesTheTransportReserve   declared transport reserve R = 0, want the declared 8
--- FAIL: TestCellWithoutRetriesLeavesTheReserveUnspent   U made 256 decisions in 256 attempts, want 248/248
--- FAIL: TestCellRetryingWithinTheReserveStillCompletes  U halted inside the reserve: {Ceiling:cognitive-calls Threshold:256 Observed:257 Hour:160}
FAIL  agentworld/cmd/vs2
```

The third line reproduces ticket 39 §11.1's halt signature (`threshold 256, observed 257`) on the new
fixture, so the tests detect the defect and the repair. Evidence: `mutation-R0.log`. The constant was
restored to `8` before any further run; `git diff` after restoration shows only the intended changes.

## 5. What ticket 39's `$0.6007` bought

It is **not wasted, and it produced no verdict**. For `$0.6007` and 3 020 attempts, ticket 39 delivered
everything in §11.6 except a score:

1. **The `A0` anchor**, exactly — three replicates at `H = 1444`, `B = 2`, `S = 0`, `alive@144 = 1`.
2. **The contract held** — `inadmissible: 0` on all four model-bearing arms, 3 019 decisions, 8 malformed
   answers rejected by `natural-claim/v4` rather than applied.
3. **The realised cost model** — `$1.989e-4` per physical attempt (this is the price §7 uses for ticket 41),
   against ticket 38 §8.1's projected `$1.5e-4`–`$4.2e-4` band.
4. **The ceiling-path proof** — a crossing is typed, the ledger is flushed before the charge, the cell is
   marked, the run stops, and the crossing is visible in the cell row and the scorer's inventory check.
5. **The diagnosis itself** (§11), which is what this ticket implements.

What it did not buy: a band, a tier, a pair comparison or a verdict. `NOT RUN` is not a negative result and
nothing in ticket 39's artefact says whether any budgeted arm separates from `P` or `U` (§11.6, §10.9).

## 6. The re-pre-registration: every declared sentence that changes

Only the budget axis and the arithmetic derived from it change. Nothing in ticket 38 §7.4–§7.7 changes,
and no sentence anywhere is reworded to fit a computed result, because none exists.

| # | file, location | before | after (current fact) |
|---|---|---|---|
| 1 | ticket 38 §7 preamble | "Fixed here, before any live call of the M2c matrix." | the same, plus: the budget axis is the one item re-declared after the first run produced no verdict (`NOT RUN`, ticket 39 §10.5); the reserve is ticket 40 §3; §7.4–§7.7 are unchanged |
| 2 | ticket 38 §7.1, budget-axis row | "the declared call ceiling `N = 256` per budgeted arm-run (unchanged)" | "the declared call ceiling `N = 256` **physical attempts** per budgeted arm-run (unchanged), of which the schedule selects at most `N − R = 248` decisions; `R = 8` is the declared transport reserve (ticket 40 §3)" |
| 3 | ticket 38 §7.3 | "12 model-bearing runs at `N = 256` calls (`A0` runs 3 times at 0 calls). Ceiling 3 072 physical attempts, because `AllowCall` is charged per attempt" | "12 model-bearing runs at `N − R = 248` decisions each (`A0` runs 3 times at 0 calls), inside a ceiling of `N = 256` physical attempts, because `AllowCall` is charged per attempt: 12 × 248 = **2 976 decisions**, 12 × 256 = **3 072 attempts**, with `R = 8` per cell left unconsumed for retries" |
| 4 | ticket 38 §8.1, lower bound | "`$0.199 × 3 072 / 1 325 = $0.46`" | "`$0.199 × 2 976 / 1 325 = $0.45` at the expected 2 976 attempts, `$0.46` at the 3 072-attempt ceiling" |
| 5 | ticket 38 §8.1, upper bound | "`244·3 + 256·3 + 256·3 + 253·3 = 3 027` calls → **≈ $1.28**" | "`244·3 + 248·3 + 248·3 + 248·3 = 2 964` calls → **≈ $1.25**" (`P`'s 244 is its own recorded gate-bound rate and the reserve does not bind it; the three quota-bound arms are 248 by schedule) |
| 6 | ticket 38 §8.1, caps | "**Hard cap for ticket 39: $2.00, and 4 000 physical attempts.**" and its "1.56×" justification | "**Hard cap for one M2c matrix run: $1.20, and 7 000 physical attempts cumulative across runs**", sized from the realised `$1.989e-4` per attempt instead of the pre-run projection (ticket 39 used 3 020 attempts and `$0.6007`; a full re-run adds at most 3 072, leaving 908 for one cell re-run) |
| 7 | ticket 38 §8.1, stop rule | "more than **$1.60** … projects above **$2.00**" | "more than **$0.96** … projects above **$1.20**" (the same 80 % rule, against the new cap) |
| 8 | ticket 38 §8.2 | "3 072 attempts → **169–194 min**" | "2 976 expected / 3 072 ceiling attempts → **163–194 min**; realised for a full matrix on this machine: **44 min** (ticket 39 §11.5)" |
| 9 | ticket 38 §8 heading | "The run plan ticket 39 executes (costed)" | "The M2c run plan (costed)" — one plan, executed by ticket 39 and re-executed by ticket 41 |
| 10 | ticket 39 §1, the M2c question | "at equal declared call ceiling `N = 256`" | "at equal declared call ceiling `N = 256` and equal declared decision budget `N − R = 248` (`R = 8`, ticket 40 §3)" |
| 11 | ticket 39 §2, budget-axis row | "declared call ceiling `N = 256` per budgeted arm-run" | "declared call ceiling `N = 256` physical attempts per budgeted arm-run, of which the schedule selects at most 248 decisions (`R = 8` reserve, ticket 40 §3)" |
| 12 | ticket 39 §11.5, closing paragraph | "The choice between (v-a) and (v-b) turns on one question that is not mine to settle … No option is recommended here." | the question was settled by the product owner: option **v-b**, recorded and implemented in ticket 40, re-run in ticket 41; the raised caps (7 000 attempts cumulative, `$1.20` per run) are named there, and the choice did not turn on the cap clauses inside (i)/(ii)/(v-a)/(v-b) |
| 13 | ticket 39 §11.7 | "the options of §11.5 are proposals for the product owner, not applied" | "the options of §11.5 were proposals for the product owner; ticket 40 records the decision (v-b) and implements it, and ticket 41 owns the re-run. No live call, re-run or scoring change happened here" |
| 14 | `docs/VERTICAL-SLICES.md`, M2c row | "the live matrix is not run … (≈3 027 calls, ≈$1.28 expected, hard cap $2.00, 3-4 h) for ticket 39. Zero live calls so far." | the current state: matrix run once (ticket 39) and refused as `NOT RUN` (one halted cell), the halt diagnosed as a budget/attempt unit mismatch, re-pre-registered with a declared transport reserve (ticket 40), arms now make 248 decisions each, re-run pending (ticket 41) at ≈2 976 attempts / ≈$0.59 expected |

`docs/VALIDATION.md` §51's M2c rule sentence and the M2c criterion of ticket 38 §7.4–§7.7 are unchanged:
the inventory precondition, the band, the effect floors, the verdict alphabet and the falsifier are
untouched. Item 14 is a propagated repair (`@propagate_down`): the row quoted §8.1's figures, which items
4–6 change, and it also carried ticket 39's outcome that no doc-layer home had recorded yet.

## 7. The costed plan for ticket 41 (the re-run)

Basis: ticket 39's **realised** price, not ticket 38's pre-run projection — `$0.6007 / 3 020 attempts =
$1.989e-4` per physical attempt (`halt-evidence.json`).

| item | value |
|---|---|
| model-bearing cells | 12 (3 × `P`, 3 × `U`, 3 × `A`, 3 × `A1`) |
| decisions per quota-bound cell | 248 (`N − R`) |
| attempts, expected (no retries) | 12 × 248 = **2 976** |
| attempts, ceiling (every reserve consumed) | 12 × 256 = **3 072** |
| `A0` cells | 3, at 0 calls, zero spend |
| **expected spend** | 2 976 × `$1.989e-4` = **≈ $0.59** |
| **worst case at the ceiling with every reserve spent** | 3 072 × `$1.989e-4` = **≈ $0.61** |
| declared cap for the re-run | **$1.20** (≈2× the worst case; supervisor-raised from $0.41–$0.50/$2.00 when the matrix was priced for one run) |
| declared attempt cap | **7 000 cumulative** (ticket 39's 3 020 + this run's ≤3 072 = 6 092, leaving 908 for one cell re-run). This exceeds the earlier 4 000-attempt operator cap, which was sized for a single run |
| cumulative project spend after the re-run | `$0.6007 + ≈$0.61 = ≈$1.21` across both runs, far under the user's `$50` |
| wall clock | 3–4 h projected (ticket 38 §8.2), 6 h deadline, `-budget-min 720`; realised 44 min for a full matrix at `-parallel 8` (ticket 39 §11.5) |
| per-cell attempt ratio to verify after the run | `attempts − decisions ≤ 8` for every cell, and `realized_calls = 248` for every quota-bound cell with no halt |

The operator poll rule stays as declared: every 15 min compute realised spend from every `calls.jsonl`
`usage` row at `$0.10/M` input and `$0.50/M` output; stop the runner and report a partial matrix if
realised spend exceeds **$0.96** (80 % of `$1.20`) or the realised rate projects above `$1.20`.

## 8. Gates (each recorded verbatim, with its result)

| gate | result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l` on tracked `.go` | empty |
| `git diff --check 279f0ba` | PASS |
| `go test ./internal/cognition/... ./cmd/vs2/` | PASS (`internal/cognition` 0.4 s, `internal/cognition/protocol` 0.0 s, `cmd/vs2` 13.2 s) |
| full suite `go test ./...` detached under `setsid nohup` | PASS — 17 packages ok, 1 with no test files (18 = `go list ./...`), 0 FAIL; `internal/world` 560 s. The suite compiled the logic-final source; three comments in `internal/cognition/priority.go` were rewrapped afterwards (comment-only, no code token), and the two packages that own the change were re-run at the final revision (`unit-tests-final.log`, PASS) |
| `vrd2.py --self-test` (zero spend) | PASS — `evaluator-gate.log` |
| `vrd2.py --live` on ticket 39's committed artefact (zero spend) | exit 3, **`NOT RUN`** — the scorer still refuses the ticket-39 matrix, which is the check that this re-pre-registration does not re-read the old artefact (`m2c-scores-rerun.log`) |
| the live matrix | **not run** (ticket 41), and nothing pushed |

## 9. What this ticket may not conclude

No verdict, no band, no tier, no arm comparison, and no claim about `A1`. `NOT RUN` stands until ticket 41
produces a complete 15-cell artefact. Nothing here revisits `VS2-EC-2`, `vrd/2`, the `natural-claim/v4`
contract, the effect floors or the inventory precondition.

## 10. Evidence paths

| artefact | home |
|---|---|
| this record | `.scratch/40-m2c-transport-reserve/spec.md` |
| the reserve tests, verbose | `.scratch/40-m2c-transport-reserve/reserve-tests.log` |
| unit suites (`internal/cognition/...`, `cmd/vs2`) | `.scratch/40-m2c-transport-reserve/unit-tests.log`, and `unit-tests-final.log` at the committed revision |
| full suite, detached | `.scratch/40-m2c-transport-reserve/full-suite.log` |
| negative control (`R = 0` mutation) | `.scratch/40-m2c-transport-reserve/mutation-R0.log` |
| evaluator gate (zero spend) | `.scratch/40-m2c-transport-reserve/evaluator-gate.log` |
| logged-data score run on ticket 39's artefact (zero spend) | `.scratch/40-m2c-transport-reserve/m2c-scores-rerun.log` |
| the code | `internal/cognition/priority.go`, `internal/cognition/priority_reserve_test.go`, `cmd/vs2/config.go`, `cmd/vs2/accounting_test.go` |
| the diagnosis this implements | `.scratch/39-m2c-matrix/spec.md` §11 |
