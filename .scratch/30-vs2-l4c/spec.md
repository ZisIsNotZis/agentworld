# Ticket 30 — VS2 lane L4c (the equal-budget attention experiment runner `cmd/vs2`)

Status: implemented on `feature/30-vs2-l4c`; **not merged** to master. Branch base
`feature/29-vs2-l4b` tip `37f902e` (master `e6d5466` + the L4b replay/selector wiring). Started
2026-10-13T23:11+08:00 (CST), actor: worker (single writer). Code + hermetic tests + a hermetic
matrix run; no live model call, no spend.

## Objective

Implement VS2 lane L4c exactly as declared in `.scratch/24-vs2-attention/spec.md` §3/§4 (the priority
/ allocation object), §5 (equal-budget accounting), §6 (the frozen fidelity metric), §7 (VS2-EC-1..9),
§9 (output) and §10 (lane board L4): the experiment runner `cmd/vs2/**` over the natural world
(H=168 h), mirroring `cmd/vs1` structure and reusing `internal/experiment` for the shared metric. The
runner wires the L3 arm-local online accumulator (`internal/cognition.PrioritySelector`) into the L2
world `Selector` seam, produces one `allocation` intervention record per escalated hour (C8), reports
every arm's realized call count and under-budget flag, and emits the frozen eight-indicator divergence
vector with the §6 no-divergence normalisation plus the VS2-EC-1..9 bands. Checkpoint/restore/replay
uses L4b's `ExportJournalWithReplaySelector` / `SaveResumeCheckpointWithReplaySelector` /
`NaturalRestoreOptions.CognitiveReplaySelector`.

## Contract (what this lane owns — sole writer)

- New `cmd/vs2/**` (package main): `doc.go`, `config.go`, `selector.go`, `harness.go`, `evidence.go`,
  `run.go`, `report.go`, `bands.go`, `privacy.go`, `replay.go`, `text.go`, `main.go`, and the tests
  `accounting_test.go`, `bands_test.go`, `e2e_test.go`.

Explicit non-goals (other lanes / frozen surface):

- No edit to `internal/world/**` (consumed read-only), `internal/cognition/**` (L1-L3), `cmd/vs1/**`,
  `internal/experiment/**`, `internal/kernel`, `internal/scheduler`, `internal/component`,
  `internal/sim`, `internal/strategy`, the frozen `natural/5..9` policy bytes, or the frozen v5–v9
  evidence. No live model call; spend is not authorised.

## Requirements and measurable acceptance indicators

1. Equal-budget arms A0/R/P/U/A (spec §8.1) over the declared world (mechanism v6, base policy
   natural/7, `-r 8`, births enabled, H=168, workers 1). R is the full reference (every candidate in
   its own stream, nil Priority); P/U/A carry a non-nil `PriorityPolicy` and are served by the L3
   arm-local accumulator through the world `Selector` seam.
2. Ceiling N=256 for P/U/A; the declared per-hour quota `q(h) = floor(N/H) + (1 if h < N mod H else 0)`
   with `Σ q(h) = N`; each arm stops at N, never exceeds it, realizes below N only on a thinner
   roster, and the report records every budgeted arm's realized count and under-budget flag (VS2-EC-2).
3. The divergence vector is computed over the same full H for every budgeted arm against R, with the
   `(raw == -1) ? H+1 : raw` first-divergence normalisation and direction-aware dominance (VS2-EC-3/4).
4. A hermetic `FakeClient` run produces a complete machine-readable report: revision binding (model +
   wire identity digest), arms, cells, fidelity, realized counts, and the EC-1..9 bands.
5. Priority arms checkpoint, save, restore, and replay byte-exact with zero model calls (L4b APIs).
6. VS2-EC-1..9 all present and exercised; the live gateway is refused unless explicitly authorised.
7. `go build ./...`, `go vet ./...`, `gofmt -l` empty, `git diff --check` clean,
   `go test -count=1 -timeout 20m ./cmd/vs2/...`.

## Implementation

- **Arms and policies (`config.go`).** `declaredArms = {A0,R,P,U,A}`; `DeclaredCallCeiling = 256`;
  `DeclaredExplorationFraction = 250000` (q=0.25); `DeclaredReferenceCeiling = 4096`; `PeriodHours = 6`;
  `DeclaredHorizonHours = 168`. `declaredPolicy` returns the zero policy for A0, `FullWakePolicy()` for
  R, and `WakePolicy{Name:"vs2-<arm>", PeriodHours:6, Priority: &PriorityPolicy{...}}` for P/U/A. The
  declared C.2 value-model instance (`vs2-value/1`, weights 4/3/2/2/1/5, `vs2-cost-proxy/1`) is declared
  in `declaredValueModel()`.
- **Selector wiring (`selector.go`).** `armSelector` wraps the L3 `cognition.PrioritySelector`,
  delegates the deterministic selection, and appends one `cognition.AppendAllocationIntervention`
  record per escalated hour (a selected hour, or a U/A hour whose roster fell short of `q(h)`).
  `newSelectorFactory` is the per-replay genesis-state selector factory L4b consumes (no ledger, so the
  world's internal reconciliation replay does not append duplicate records). The model is unpriced
  (`protocol.CallPrice{}`), so the declared `CostProxy` applies and no price is fabricated.
- **Harness/run (`harness.go`, `run.go`).** Mirrors `cmd/vs1`: reproduce the recorded ticket-19 genesis
  (neutral h0 pulse → births-enable branch child), wire the hermetic `FakeClient`/Recorder/`RoleASeam`/
  selector, run to H or a typed ceiling halt, gather the accepted history, the journal, and the call /
  allocation ledgers, then `SetCognitiveCheckpoint` + `SaveResumeCheckpointWithReplaySelector` with the
  per-replay selector factory.
- **Report (`report.go`).** One `cellReport` per arm-in-cell with the realized count / under-budget /
  shortfall accounting, a `calibrationReport` (VS2-EC-8), the eight-indicator `fidelityRow` with raw and
  normalised first-divergence hours, and the `evaluation` revision binding.
- **Bands (`bands.go`, `privacy.go`, `replay.go`).** VS2-EC-1 (A0 byte-identity vs the recorded
  ticket-19 evidence), EC-2 (realized counts / ceiling rows / fail-loud), EC-3 (direction-aware
  dominance with the clean negative distinct from an EC-3 miss), EC-4 (vector completeness, no scalar),
  EC-5 (priority replay lane + the mutated-policy-fingerprint refusal pointer), EC-6 (import-graph +
  sentinel + recorded request/allocation scan), EC-7 (exploration bound), EC-8 (value model + scores),
  EC-9 (labels + live gate + valid negative).
- **Live gate (`gateway.go`).** `-live` (non-default) plus `VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND`
  is required; otherwise the runner refuses with `ErrLiveRefused` before any construction. No live call
  is made. Gateway facts (model `github_copilot/gpt-6-luna`, `mode=responses`, only `/v1/responses`,
  base `http://127.0.0.1:4000`, auth `$LITELLM_API_KEY`) are encoded in `newLiveConfig`.

## Recorded decisions for lane-level details the spec leaves open

1. **Declared value-model instance.** Appendix C.2 keeps the weights operator-confirmable; the runner
   declares the C.2 instance in `declaredValueModel()` (minimal, reversible). Because the L2 roster
   builder leaves the derived feature rows zero (its declared gap), the projected priority is 0 for every
   candidate and the adaptive arm's ranking is the declared tie-break (lowest actor). This is the honest
   instantiation of the value model as it exists at this revision; the calibration instrument reports it.
2. **`RandSeed` provenance.** The policy PRNG seed is the cell seed (part of run identity), so U/A draw
   a stable declared stream.
3. **Allocation-record emission rule.** One record per escalated hour = a selected hour, or a U/A hour
   whose roster fell short of `q(h)` (so the §5.1 per-hour shortfall is recorded). A period-gated P hour
   with no selection records nothing.
4. **Declared wire default = responses.** The application model advertises only `/v1/responses`, so the
   hermetic default records the responses endpoint and the model+wire identity digest names it.
5. **Calibration instrument (EC-8).** The declared minimal instrument: the mean predicted priority of
   escalated candidates (normalised by the arm's max priority) versus the realized divergence rate
   (diverged model decisions / applied model decisions); `calibration_error` is the absolute difference.
6. **EC-3 status vocabulary.** `PASS` when adaptive weakly dominates with strict improvement across all
   declared cells; `FAIL` when a budgeted baseline strictly dominates adaptive; `clean-negative` when
   mutually non-dominating (the valid EC-9 outcome); `partial` when adaptive dominates on fewer than the
   declared cells.
7. **Live authorization spelling.** `VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND` plus `-live`; spend is
   declared not authorised for this lane.

## Evidence — hermetic primary matrix run (2026-10-13, CST)

Go toolchain is not on the ambient PATH: `GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH`, `GOTOOLCHAIN=local` (`go1.24.13 linux/amd64`).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `gofmt -l` over every tracked `.go` file (`.git`/`.tmp` pruned) | empty (clean) |
| `git diff --check` | clean |
| `go test -count=1 -timeout 20m ./cmd/vs2/...` | `ok agentworld/cmd/vs2 41.8s` |
| `.tmp/vs2-l4c -out .tmp/30-vs2-evaluation.json -out-txt .tmp/30-vs2-evaluation.txt -root .tmp/30-vs2-runs -recorded .tmp -src . -budget-min 50 -parallel 6` | exit 0, wall `9m06s`; 60/60 declared cells ran, 36/36 priority replays identical |

Report artifact: `.tmp/30-vs2-evaluation.json`
`sha256 3b2051696cd6be5b5714f084249e244ebbcede83154f3180a2efe8d12dafd8b6`;
`.tmp/30-vs2-evaluation.txt`
`sha256 9b3f05187e67a90c54006545e93232b752c23164f9e00f75b1d995d31e22da27`.

Revision binding (from the report): model `github_copilot/gpt-6-luna`, wire `responses`, model+wire
identity digest `66f1270482c7472ac66022c1c4d31ac48620b0e4129b065cb65ae11fde260a62`; base policy
`natural/7`; mechanism `6`; `H=168`; `N=256`; value model `vs2-value/1`; label
`synthetic/model-conditional; hermetic FakeClient (no network, no authorised spend)`.

Realized call counts (call ledger, per arm over the 12 primary cells):
R `1166..1512` (ceiling 4096, no halt), P `243..256` (6/12 under budget), U `226..236` (12/12 under
budget), A `226..256` (10/12 under budget). No budgeted arm exceeded `N`; no ceiling halt; every
budgeted cell reports the four ceiling rows.

Band statuses (verbatim, never tuned):
`[PASS] VS2-EC-1` (A0 byte-identical to the recorded ticket-19 evidence at all 12 cells);
`[PASS] VS2-EC-2` (36 budgeted cells, no over-ceiling, no halt, four ceiling rows each);
`[FAIL] VS2-EC-3` (complete A/P/U cells 12/12; adaptive dominates 0; clean-negative 4;
baseline strictly dominates adaptive 8 — falsified at f10-s0..f10-s3, f11-s0, f12-s0, f12-s2, f12-s3);
`[PASS] VS2-EC-4`; `[PASS] VS2-EC-5` (36 priority cells replayed identical, zero model calls);
`[PASS] VS2-EC-6` (24777 recorded Role A requests scanned, no privileged marker);
`[PASS] VS2-EC-7` (A exploration 674 ≤ 12×64; P/U exploration empty);
`[PASS] VS2-EC-8`; `[PASS] VS2-EC-9`.

**EC-3 is the honest, decisive VS2-EC-9 outcome, not an implementation failure.** With the L2 roster
builder leaving the derived value features zero, the projected priority is constant and the adaptive
arm's ranking is its declared tie-break; periodic then strictly dominates it on the capital- and
granary-histogram means in 8/12 cells and is mutually non-dominating in 4/12. Spec §8.4 declares
"periodic or random dominates adaptive" a valid result that falsifies the value model as instantiated;
it is reported verbatim.

## Revision binding

- Branch: `feature/30-vs2-l4c`; base: `feature/29-vs2-l4b` tip `37f902e` (master `e6d5466`).
- Implementation commit: `6764672` (`VS2 L4c: implement the equal-budget attention experiment runner cmd/vs2`).
- Source/test blob hashes at this revision:
  - `cmd/vs2/config.go` `cd5b694563f7327e2a82860a4e095b67c2d45fc5`
  - `cmd/vs2/selector.go` `e01db2277b8e94eab1a3a8f269cbcd05a396871c`
  - `cmd/vs2/harness.go` `bb40f7803141113b6dcb9b429164ba91e9082097`
  - `cmd/vs2/run.go` `ca3c69d42e55ca8b64e3358e9b298e62b488271e`
  - `cmd/vs2/report.go` `ec707daf1ffe93792b2a60c7e3cd3f8112231d48`
  - `cmd/vs2/bands.go` `d30d6d37cdedd359868ba02f3623e6fab7eab3f7`
  - `cmd/vs2/privacy.go` `f8a898cb093b6e3fad9161e808879bd6892b285e`
  - `cmd/vs2/replay.go` `7b35155ee437a05af0853fc2f0ca4d50236a38f6`
  - `cmd/vs2/gateway.go` `925fbcf4c0218b88ab68391e46e2504d4417a6b2`
  - `cmd/vs2/main.go` `16d6b10b980ed5d4fbc33898a96a226f2a845ec6`
  - `cmd/vs2/evidence.go` `3d44dfadd04f4f7e49694b4d20dadc340559ee21`
  - `cmd/vs2/text.go` `5e68f655de32a91525c16aca2b07b2159f9640bc`
  - `cmd/vs2/doc.go` `01a02201fdef3bf73e2f3d31a31aedfbccfc6c90`
  - `cmd/vs2/accounting_test.go` `c2de9923e02bd35db5c570509fbfac2620a1325e`
  - `cmd/vs2/bands_test.go` `2deca092f5181293e6fdc2f708c4feb5fd9b31f7`
  - `cmd/vs2/e2e_test.go` `afd9870ff152121ca1fd931f734cf3d289cef310`

## Residual risks / declared gaps

1. **The value model is uninformative at this revision** (the L2 derived features are zero), so the
   central claim is not evidenced: EC-3 is falsified. This is a mechanism/accounting proof, never the
   central claim, which awaits real-model spend authorisation (VS2-EC-9, spec §1.3) and the L2 derived
   feature rows. The unit the runner proves is the equal-budget ALLOCATION MACHINERY, load-bearing and
   replayable.
2. **Under-budget is common at H=168** (28/36 budgeted cells) because the claim-wake stream thins as the
   population dies; the realized count and under-budget flag are reported per cell, never equalized
   (spec §5.3).
3. **The matrix path uses the branch child, for which the world's journal export does not reconcile**
   (pre-existing `exportJournalWithReplay` boundary handling). The runner therefore checkpoints via
   `SaveResumeCheckpointWithReplaySelector` (which recovers the boundary from the journal itself) and the
   `ExportJournalWithReplaySelector` path is exercised by the direct-genesis round-trip test. No
   `internal/world` change is made.
4. **The branch-child limitation is a world property, not an L4c defect.** A later lane could widen the
   world export; it is out of this lane's contract.
5. The `-sensitivity` extension (f=12 seeds 4..7) and the N=512 budget-sensitivity cell are implemented
   behind flags but were not run for this evidence record; the primary matrix is the recorded run.
