# Ticket 27 — VS2 lane L3 (cognition wiring: arm-local accumulator, allocation records, PRNG, price adapter)

Status: implemented on `feature/27-vs2-l3`; **not merged** to master. Branch base `7d9b22d`
(the verified L1 tip). Started 2026-10-13T20:45+08:00 (CST), actor: worker (single writer).
Code + hermetic tests only.

## Objective

Implement VS2 lane L3 exactly as declared in `.scratch/24-vs2-attention/spec.md` §3, §4.1/§4.5/§4.6,
§5.1, §10 (lane board L3), and Appendices A.1/A.2/B, consuming the L1 protocol surface landed at
`7d9b22d`. L3 is the cognition-side wiring: the arm-local deterministic online accumulator behind
`protocol.EscalationSelector`, the additive `allocation` intervention record, the declared policy PRNG,
and the `cognition.PriceTable` → `protocol.CallPrice` adapter. The nil-`Priority` VS1 path is untouched.

## Contract (what this lane owns — sole writer)

- New `internal/cognition/priority.go`: the `PrioritySelector` arm-local deterministic online accumulator
  (`NewPrioritySelector`, `Select` + `CallsSpent`/`PRNGPos`/`CallCeiling`), the fixed per-hour quota
  `q(h) = floor(N/H) + (1 if h < N mod H else 0)`, the stop-at-N semantics, the declared §4.5
  exploration/exploitation interleaving for arm A, the declared policy PRNG with its recorded position,
  and the `PriceTable.CallPrice()` adapter.
- New `internal/cognition/allocation.go`: `AppendAllocationIntervention` (the `allocation` record
  producer) and `AllocationResultSelected`.
- `internal/cognition/codec.go`: the additive `InterventionAllocation` kind, the `AllocationRecord`
  payload, and the omitempty `InterventionRecord.Allocation` field; existing record bytes unchanged.
- `internal/cognition/wake.go`: the VS2 priority/allocation type aliases and declared constants.
- New tests `internal/cognition/priority_test.go`, `internal/cognition/allocation_test.go`.
- Read-only everywhere else.

Explicit non-goals (owned by other lanes or out of scope):

- No change to `internal/world/**` (L2 roster builder, `NaturalCognition.Selector` seam, checkpoint
  binding, construction-time refusal), `cmd/**` (L4 runner/arms/experiment extraction, `cmd/vs1`
  migration), `internal/kernel`, `internal/scheduler`, `internal/component`, `internal/sim`, or
  `internal/strategy`; no change to the frozen `natural/5..9` policy bytes or the frozen v5–v9 evidence.
- No live model call, no `EscalationRequest` predicate/feature derivation (L2 world wiring; L1 deferred
  it), no checkpoint wire extension (L2), no fidelity metric (L4).

## Requirements (from VS2 spec lane board L3 + dispatch)

1. The arm-local deterministic online accumulator seeded with the call ceiling `N` and `RandSeed`,
   adapted to `protocol.EscalationSelector`; its spent-call count and PRNG position are recorded run
   state; it stops at `N`, never exceeds it, and is period-gated for P.
2. The fixed per-hour quota `q(h) = floor(N/H) + (1 if h < N mod H else 0)`, with `Σ_{h<H} q(h) = N`,
   independent of any realized stream.
3. The declared §4.5 interleaving for arm A: `E = ceil(q·N) = (N·frac + scale − 1)/scale`,
   `T(t) = ceil(E·t/H)`, `e(h) = T(h+1) − T(h)`; the first `min(e(h), slots offered)` slots are
   protected-exploration (uniform policy-PRNG draws over the hour's not-yet-selected candidates) and the
   rest priority-ranked exploitation (highest `Priority`, ties broken by actor id); `q = 0` is pure
   exploitation.
4. The `cognition.PriceTable` → `protocol.CallPrice` adapter (the L1 types are protocol-local;
   `cognition` already imports `protocol`, so the adapter lives on the cognition side).
5. `internal/cognition/wake.go` aliases as the design declares.
6. `internal/cognition/codec.go`: the additive `InterventionAllocation` kind + `AllocationRecord`
   (omitempty; existing ledger bytes unchanged).
7. Hermetic tests: exact-`N` on an ample stream, under-spend on a short stream, deterministic replay
   from recorded state, and byte-identity of existing records.

## Measurable acceptance indicators

- `go build ./...` passes.
- `go vet ./...` passes.
- `go test -count=1 ./internal/cognition/...` passes.
- `go test -count=1 -race ./internal/cognition/...` passes.
- `gofmt -l` over every tracked `.go` file (`.tmp` pruned) is empty.
- `git diff --check` clean.
- Tests prove: exact-`N` for P/U/A on an ample roster and no overspend; under-budget with recorded
  shortfall on a thin roster; identical replay of the selection sequence (including `Budget.CallsSpent`
  and `PRNGPos`) from the declared policy identity + stream; `E` exploration calls exactly on an ample
  stream and empty exploration at `q = 0`; the pre-change intervention golden bytes unchanged by the
  additive `Allocation` field; the allocation record survives the hash-chained ledger, decode, and
  `ReadInterventionRecords`.

## Implementation

`internal/cognition/priority.go` (new): `PrioritySelector` + `NewPrioritySelector(policy, horizonHours,
price)` + `Select`. The constructor refuses a nil `Priority` (the degenerate path), `StrategyFull`, an
undeclared ceiling/horizon, an out-of-range exploration fraction, and an invalid periodic gate. `Select`
dispatches on the declared strategy:

- P: period-gated (`h ≡ phase (mod period)`); in escalated hours selects the roster in actor order up to
  the remaining budget; `Quota = remaining`, `Shortfall = 0`. Non-escalated hours select nothing.
- U: spends at most `q(h)` via distinct policy-PRNG draws over the actor-sorted roster.
- A: offers `min(q(h), remaining, roster)` slots; exploration slots first per §4.5, then priority-ranked
  exploitation.

Every `Select` records the actor-sorted per-candidate `CandidateScore` list, the selected and
exploration sets, the post-selection `BudgetState` (`CallsSpent`/`CallsRemaining`/`CallsBudget`), `Quota`,
`Shortfall`, and `PRNGPos`. `PriceTable.CallPrice()` adapts the declared price table to the
protocol-local price type.

`internal/cognition/allocation.go` (new): `AppendAllocationIntervention(l, in)` builds and appends the
`allocation` record through `Ledger.AppendIntervention`, deriving `CallCeiling`/`RealizedCalls` /
`Budget` from the recorded `Selection.Budget` and the fingerprint from `WakePolicy.Fingerprint()`.

`internal/cognition/codec.go` (modified): `InterventionAllocation InterventionKind = "allocation"` added
to the admitted-kind set; `AllocationRecord` type; omitempty `Allocation *AllocationRecord` field on
`InterventionRecord`.

`internal/cognition/wake.go` (modified): type aliases for the Appendix A.1/A.2 surface
(`PriorityPolicy`, `PriorityStrategy`, `ValueModelSpec`, `ValueFeatures`, `FeatureWeight`, `CostProxy`,
`CallPrice`, `CandidateScore`, `PolicyPredicate`, `FeatureThreshold`, `EscalationCandidate`,
`EscalationReason`, `Selection`, `BudgetState`, `EscalationSelector`) and the declared strategy/scale
constants, mirroring the existing `WakePolicy` alias convention.

## Recorded gaps / under-specification (implemented to the declared shape, not widened)

1. **Budget authority for the accumulator.** §4.1 item 5 says the runner-provided accumulator "owns the
   policy PRNG, the spent-call count, and the budget state", but `Select` also receives a
   `BudgetState`. Resolved by making the accumulator's own state authoritative: the selection depends
   only on the declared policy identity, the candidate stream, and the accumulator state (§4.6), and the
   passed `budget` is the world's declared view used only to refuse a ceiling that disagrees with the
   declared `N`. Honoring an unset `CallsRemaining` as a cap would make an unrelated world view perturb
   the deterministic selection and break replay from recorded state. The recorded `Selection.Budget` is
   the accumulator's post-selection state.
2. **Horizon `H` is not on the `EscalationSelector` interface.** `q(h)` and `T(t)` need `H`, but
   `Select(hour, roster, budget)` carries no horizon. Resolved by seeding it at construction:
   `NewPrioritySelector(policy, horizonHours, price)`; `horizonHours` is the declared evaluation horizon
   (168 h for VS2), part of the arm's fixed accumulator seed. `NewPrioritySelector` refuses `H < 1`.
3. **`wake.go` alias set is not enumerated by the design.** The lane board assigns `internal/cognition/
   wake.go` aliases to L3 but the contract does not list them. Resolved by re-exporting the declared
   Appendix A.1/A.2 type surface plus the closed strategy set and the fixed-point scales, matching the
   existing `WakePolicy` alias convention. No new protocol types are invented.
4. **Allocation record `Result` status is undeclared.** Appendix B fixes the `AllocationRecord` fields
   but not the intervention `Result`. Resolved with the declared constant
   `AllocationResultSelected = "selected"`, `CallRefs: []uint64{}`, and a non-nil empty
   `IndicatorDeltas` (canonical `[]`, never `null`), consistent with the Role A producer.
5. **`call_ceiling` / `realized_calls` provenance.** Appendix B lists `CallCeiling` and `RealizedCalls`
   without a source. Resolved by deriving both from the recorded `Selection.Budget` (`CallsBudget`,
   `CallsSpent`), so the record is a function of the same accumulator state that produced the selection
   and needs no parallel bookkeeping. The runner's separate per-arm realized count comes from the call
   ledger (L4).
6. **Declared PRNG choice.** The design says "the policy's own declared PRNG" without naming an
   algorithm. Resolved with `math/rand` `rand.New(rand.NewSource(RandSeed))` plus an explicit consumed
   draw count recorded as `PRNGPos` (spec Appendix A.1). Deterministic and reproducible within a build;
   the seed is part of run identity.

## Commands and results (2026-10-13, CST)

Go toolchain is not on the ambient PATH: use
`GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH` (`go1.24.13 linux/amd64`).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test -count=1 ./internal/cognition/...` | passed (`cognition 1.019s`, `protocol 0.003s`) |
| `go test -count=1 -race ./internal/cognition/...` | passed (`cognition 2.897s`, `protocol 1.029s`) |
| `gofmt -l` over every tracked `.go` file (`.tmp` pruned) | empty (clean) |
| `git diff --check` | clean |
| `git status --porcelain` | only the six in-scope files (2 modified + 4 new) |

## Revision binding

- Branch: `feature/27-vs2-l3`; base: `7d9b22dc49ae5d9b185bb5c50246adbc163834d1` (L1 tip).
- Source/test blob hashes at this revision:
  - `internal/cognition/priority.go` `a79eb9e158ae5657bc8e2dfdb47ee1286a91467c`
  - `internal/cognition/allocation.go` `2546fa57dba88f43a1b0cbf275d233f8b6a3a806`
  - `internal/cognition/codec.go` `289f9eb3257fd40f015feb39160b345204db8225`
  - `internal/cognition/wake.go` `98fb1bb893769344576f76dcb66d43c127997961`
  - `internal/cognition/priority_test.go` `4ab2ca471fb188783b7b99f47260a558c9b2e38a`
  - `internal/cognition/allocation_test.go` `8477e27abc9b64592c66d1509f05d8f58b649f04`
- L3 commit on `feature/27-vs2-l3`: recorded in the addendum below.
