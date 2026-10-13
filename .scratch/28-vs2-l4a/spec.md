# Ticket 28 — VS2 lane L4a (shared `internal/experiment` extraction + `cmd/vs1` migration)

Status: implemented on `feature/28-vs2-l4a`; **not merged** to master. Branch base `2e4a25e`
(the verified L1/L2/L3 master tip). Started 2026-10-13T22:31+08:00 (CST), actor: worker (single
writer). Code + hermetic tests only.

## Objective

Implement VS2 lane L4a exactly as declared in `.scratch/24-vs2-attention/spec.md` §5.3, §6, §7, §9
and §10 (lane board L4): extract the shared, reusable experiment mechanics (`divergenceVector`,
`summarize`, `decisionErrorIndicator`, and the hourly-projection extraction they consume) into one
new package `internal/experiment/**`, and migrate `cmd/vs1` to consume it instead of its private
copies, with VS1 EC-1/EC-2 byte-identity re-proved. The extracted `DivergenceVector` computes over
the full declared horizon `H` and refuses an arm whose projection is shorter than `H`, rather than
inheriting the scaffold's `min(len(b.Hours), len(r.Hours))` truncation.

## Contract (what this lane owns — sole writer)

- New `internal/experiment/projections.go`: `Projections`, `ExtractProjections`, `AliveActors`
  (moved from `cmd/vs1/evidence.go`), plus the private capital/granary histogram helpers.
- New `internal/experiment/fidelity.go`: `FidelityIndicator`, `Arm`, `DivergenceVector`, `Summarize`,
  `DecisionErrorIndicator`, `TotalVariation`, `ModelEscalatedKeySet`, `Level2RefusalCounts`,
  `ShortProjectionError` (moved from `cmd/vs1/report.go`).
- New tests `internal/experiment/fidelity_test.go`, `internal/experiment/projections_test.go`.
- `cmd/vs1/{evidence,load,report,run}.go` migrated to consume `internal/experiment`;
  `cmd/vs1/{vs1_test,residual_test}.go` updated to the moved symbols.

Explicit non-goals (other lanes / frozen surface):

- No change to `internal/world`, `internal/cognition`, `internal/kernel`, `internal/scheduler`,
  `internal/component`, `internal/sim`, the frozen `natural/5..9` policy bytes, or the frozen v5–v9
  evidence. No `cmd/vs2` (that is L4b).

## Requirements and measurable acceptance indicators

1. The fidelity vector, summaries, and decision-error indicator have **one home** in
   `internal/experiment` (no duplicate copy left in `cmd/vs1`).
2. `DivergenceVector(b, r Arm, horizonHours int)` computes over the full declared horizon (`H+1`
   hourly samples, hours `0..H`) and refuses an arm whose projection is shorter, returning a typed
   `*ShortProjectionError` naming the arm, its actual sample count, and the required count. It must
   not truncate to `min(len(b), len(r))`.
3. `cmd/vs1` consumes the shared package; the evaluation/report for a recorded cell is
   **byte-identical** before and after the migration.
4. `go build ./...`, `go vet ./...`, `go test -count=1 ./internal/experiment/... ./cmd/vs1/...`,
   `gofmt -l` (empty), and `git diff --check` all pass. `git status --porcelain` shows only the
   intended paths.

## Implementation

`internal/experiment/projections.go` (new): the hourly indicator sequences (`Projections`) and
`ExtractProjections`, moved verbatim from `cmd/vs1/evidence.go` (`aliveActors` → exported
`AliveActors`, because `cmd/vs1`'s Role B evidence packet still needs it). The capital edge
`declaredKMax` and the granary bucket edges move with the extraction but stay package-private (only
`ExtractProjections` reads them).

`internal/experiment/fidelity.go` (new): `DivergenceVector`, `Summarize`, `DecisionErrorIndicator`,
`TotalVariation`, the hourly-count helpers (`cumulativeHourly`/`rate`, private), and the level-2 join
(`ModelEscalatedKeySet`, `Level2RefusalCounts`). The `cellRun` inputs are replaced by an arm-agnostic
`Arm{Label, Projections, Patches, Interventions, Journal}` value, so the metric is reusable by the
VS2 runner. `DivergenceVector` takes the declared horizon `horizonHours` and computes over `H+1`
samples, checking **both** arms' projection lengths first. The `hourlyCounts` window, previously the
hardcoded `169` (and `h > 168`), is parameterized by the sample count; `cmd/vs1` passes
`cr.fp.Hours+1 = 169`, so behaviour is unchanged.

`cmd/vs1/report.go` (modified): drops the private `fidelityIndicator`, `divergenceVector`,
`summarize`, `decisionErrorIndicator`, `hourlyCounts`, `cumulativeHourly`, `level2RefusalCounts`,
`modelEscalatedKeySet`, `rate`, `totalVariation`, `min`; keeps `buildFidelity`/`referenceRun`/
`level2RefusalTotal`/`sortRuns`. A new `fidelityArm(*cellRun) experiment.Arm` adapter feeds the
shared metric. `fidelityRow.Indicators` is now `[]experiment.FidelityIndicator` (identical JSON
tags). `level2RefusalTotal` reads the shared `Level2RefusalCounts`.

`cmd/vs1/{evidence,load,run}.go` (modified): `projections`/`extractProjections`/`aliveActors` replaced
by `experiment.Projections`/`experiment.ExtractProjections`/`experiment.AliveActors`.

## Byte identity evidence (VS1 EC-1/EC-2)

Go toolchain is not on the ambient PATH: use
`GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH`, `GOTOOLCHAIN=local` (`go1.24.13 linux/amd64`).

Two binaries were built from the same worktree: `vs1-base` at base `2e4a25e` (before any edit) and
`vs1-branch` after the migration. Both ran the recorded f12-s0 cell.

**Exact-byte (no masking) — `-reuse` of the committed A1/A3 bundles** (restores the final bundles,
re-derives projections/ledgers/replay and the whole report; no re-simulation, so wall-clock fields are
loaded from the prior report rather than re-measured):

| Artifact | SHA-256 |
|---|---|
| base `base-reuse1.json` (base `2e4a25e` binary) | `34a3ffe490d38f7f12344d5b2b45da5c324b51ea9206be99ab67e1afe75df052` |
| base `base-reuse2.json` (control, same binary, second run) | `34a3ffe490d38f7f12344d5b2b45da5c324b51ea9206be99ab67e1afe75df052` |
| branch `branch-reuse.json` | `34a3ffe490d38f7f12344d5b2b45da5c324b51ea9206be99ab67e1afe75df052` |

`diff base-reuse1.json branch-reuse.json` → empty (**BYTE-IDENTICAL**). The control pair
(`base-reuse1` vs `base-reuse2`, same base binary) is byte-identical too, confirming the reuse report
is fully deterministic.

**Fresh hermetic run** (`-arms A0,A1,A3 -cells f12-s0`, probe/twin/fixture off; each run ~4 min):

| Section | Result |
|---|---|
| `fidelity` array exact JSON bytes | identical, SHA-256 `fb49e0103b175169479a221cbf2a082d363e2b5979206d1e8c0490ecd8b896e6` (base and branch) |
| `bands` array exact JSON bytes | identical |
| full report after masking only the wall-clock fields `duration_ms` / `call_latency_total_micros` | identical |
| raw `duration_ms`/`latency` (base vs branch) | differ (A0 `6328→6180`, A1 `10755/27147→11781/28927`, A3 `116164/141655→116269/148082`) — these are `time.Now` measurements, not report content |

The fresh-run `fidelity` row carries all **eight** indicators over **169** samples
(`population`, `births`, `deaths/starvation`, `deaths/age`, `capital-histogram`,
`granary-histogram`, `exchange-volume`, `decision-error-rate(level-1+level-2)`), i.e. the full
declared 168-hour horizon, byte-identical across the migration.

## Commands and results (2026-10-13, CST)

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test -count=1 ./internal/experiment/... ./cmd/vs1/...` | passed (`experiment 0.008s`, `cmd/vs1 134.908s`) |
| `gofmt -l` over every tracked `.go` file (`.tmp` pruned) | empty (clean) |
| `git diff --check` | clean |
| `git status --porcelain` | only the six modified `cmd/vs1` files + new `internal/experiment/` + this ticket |

## Revision binding

- Branch: `feature/28-vs2-l4a`; base: `2e4a25e74ebffade6ac1f322e8ded54ef23c10eb` (master).
- Source/test blob hashes at this revision:
  - `internal/experiment/projections.go` `1b7fe46301a63754691aa54207378a1b28c5f201`
  - `internal/experiment/fidelity.go` `618887e00aefada86807d3c7bfb1893857da3154`
  - `internal/experiment/fidelity_test.go` `ff3075ffeb03dd81f7e7b6d2e5d93fcf53a41fd2`
  - `internal/experiment/projections_test.go` `a69341f1b9771799e792850c08e0f0d66b9ed7a0`
  - `cmd/vs1/evidence.go` `fbe98ad52ad67ae25f15baca5a78610f53b8642e`
  - `cmd/vs1/load.go` `8f8a4667fef50458fd47be077743acffca935480`
  - `cmd/vs1/report.go` `8f2b143979dc3a2797ad1b5ac5b8515b686c3c25`
  - `cmd/vs1/run.go` `26d4490ec5ee7abec62d1a57d0fac96b4b29b42d`
  - `cmd/vs1/vs1_test.go` `348b0d630399bac293d7f1d0bbb18fa0633157bc`
  - `cmd/vs1/residual_test.go` `b5728e635faa7bf472362f7324883134c494385e`
- L4a commit on `feature/28-vs2-l4a`: `403b95d0b5678c742de96ac6cf0c0c53c8fcdc5e` (the commit that added `internal/experiment/**`, migrated `cmd/vs1`, and added this ticket; this line records its hash).

## Residual risks / declared gaps

1. **Wall-clock fields are not reproducible** and are out of scope for byte identity: the report
   carries `duration_ms` and `call_latency_total_micros` measured with `time.Now`, which differ
   between any two runs (shown by the fresh-run table). The `-reuse` comparison confirms that when
   those fields are not re-measured the entire report is byte-identical; the fresh-run comparison
   masks only those two fields and is otherwise identical. No fidelity/band/cell content differs.
2. **`cmd/vs1` drops a fidelity row on a refused (short) arm.** The shared metric refuses a
   projection shorter than `H`; VS1's declared arms all run the full horizon, so this is unreachable
   and produced no observable change. Recording the short arm in the report is VS2's concern (L4b).
3. **`Level2RefusalCounts` sample window parameterized.** VS1 previously hardcoded 169; `cmd/vs1`
   passes `fp.Hours+1`. For every declared matrix cell that is 169, so the count is unchanged (the
   fresh-run and reuse reports are identical).
4. **`internal/experiment` imports `internal/world`/`internal/cognition`/`internal/sim`** (read-only)
   so the metric can read the published projections and journal; no forbidden package was modified.
