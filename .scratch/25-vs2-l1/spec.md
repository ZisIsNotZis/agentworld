# Ticket 25 — VS2 lane L1 (priority protocol surface + value model)

Status: implemented on `feature/25-vs2-l1`; **not merged** to master. Branch base `fa4eba4`
(merged master after ticket 24, 2026-10-13). Started 2026-10-13T20:33+08:00 (CST), actor: worker
(single writer). Code + hermetic tests only.

## Objective

Implement VS2 lane L1 exactly as declared in `.scratch/24-vs2-attention/spec.md` §4, §10 (lane board)
and Appendices A.1/A.2/C: the additive `protocol.Priority` extension of VS1's `WakePolicy`, the
degenerate nil-`Priority` `Select`, the priority/value-model protocol types, and the pure integer
fixed-point value projection. L1 is the pure protocol surface only. The nil-`Priority` path must keep
the VS1 canonical bytes and fingerprint unchanged (VS2-EC-1).

## Contract (what this lane owns)

- Sole writer of `internal/cognition/protocol/wake.go` (add the `Priority` field + `Select`), new
  `internal/cognition/protocol/priority.go`, new `internal/cognition/protocol/valuemodel.go`, and the
  new tests `internal/cognition/protocol/wake_test.go`, `internal/cognition/protocol/valuemodel_test.go`.
- Read-only everywhere else.

Explicit non-goals (owned by other lanes or out of scope):

- No change to `internal/world` (L2 escalation gate, checkpoint binding) or `cmd/vs1` (L4 migration).
- No change to `internal/cognition` (L3 accumulator, allocation records, PRNG, price adapter) or to
  `internal/kernel`, `internal/scheduler`, `internal/component`, `internal/sim`, `internal/strategy`.
- No change to the frozen `natural/5..9` policy bytes or the frozen v5–v9 evidence; no `.tmp` writes
  used as evidence.
- No experiment runner, no `cmd/vs2` (L4), no allocation-record codec (L3), no live model call.

## Requirements (from dispatch + VS2 spec lane board L1)

1. `WakePolicy` gains `Priority *PriorityPolicy` with json tag `priority,omitempty`; nil is VS1.
2. Degenerate `WakePolicy.Select(hour, roster, budget) (Selection, error)`: pure, selects every
   candidate of an escalated hour, consumes no world random stream.
3. `PriorityPolicy`/`PriorityStrategy`/`ExplorationScale`/`PriorityScale`, `PolicyPredicate`,
   `FeatureThreshold`, `BudgetState`, `EscalationReason`, `EscalationCandidate`, `Selection`,
   `EscalationSelector` (Appendix A.1).
4. `ValueModelSpec`, `ValueFeatures` (incl. `EscalationRequested`), `FeatureWeight`, `CostProxy`,
   `CallPrice`, `CandidateScore`, and the pure `ExpectedValue`/`ExpectedCost`/`Priority` projection
   (Appendix A.2/C.2).
5. Hermetic unit tests, including a pinned nil-`Priority` canonical/byte-identity proof that the VS1
   wake-policy bytes are unchanged.

## Measurable acceptance indicators

- `go build ./...` passes.
- `go vet ./...` passes.
- `go test ./internal/cognition/protocol/... ./internal/cognition/...` passes.
- `go test -race ./internal/cognition/protocol/...` passes.
- `gofmt -l` over every tracked `.go` file is empty.
- `git diff --check` clean.
- Exactly one test asserts the VS1 nil-`Priority` identity equals the recorded pre-VS2 bytes
  `{"name":"periodic","period_hours":6,"phase_hours":0}` and fingerprint
  `83c377e50bcf01b3aea9326f38ac78f83aaebc7025d38fa43418e8f54bce66de`; at least one test fails if
  `priority` leaks into that identity, if `Select` stops selecting the roster, or if the priority
  formula/reduction changes.

## Implementation

`internal/cognition/protocol/wake.go` (modified):

- `WakePolicy` field `Priority *PriorityPolicy \`json:"priority,omitempty"\``. `WakePolicy` stays
  comparable (a pointer field is comparable), so the existing `p == VS1WakePolicy()` comparison, the
  world's keyed literals, and the manual checkpoint wire decode are unaffected.
- `func (p WakePolicy) Select(hour int, roster []EscalationCandidate, budget BudgetState) (Selection, error)`
  — builds the selected actor set (roster order) only when `p.Escalates(hour)`, echoes `budget`,
  leaves `Quota`/`Shortfall`/`PRNGPos` zero, returns a non-nil empty slice otherwise.

`internal/cognition/protocol/priority.go` (new): the Appendix A.1 types and constants listed in
requirement 3.

`internal/cognition/protocol/valuemodel.go` (new): the Appendix A.2 types and the projection:

- `ExpectedValue(m, f)` = declared weighted sum over the named C.1 features; an unknown feature name
  contributes 0.
- `ExpectedCost(m, f, price)` = priced cost (`price.Version != ""`: per-token rates × projected usage)
  else the declared `CostProxy` (constant + projected-token term).
- `Priority(m, f, price)` = `ExpectedValue·PriorityScale / max(1, ExpectedCost)`, integer arithmetic.

Tests: `wake_test.go` (nil-`Priority` byte/fingerprint identity; non-nil `Priority` encode+round-trip;
degenerate `Select` roster order, unselected hour, determinism, no PRNG; declared strategies/scales)
and `valuemodel_test.go` (C.2 weighted sum = 76; unknown-feature and all-zero-weight reductions;
unpriced vs priced cost; fixed-point ratio; zero-cost `max(1,·)` floor).

## Recorded gaps / under-specification (implemented to the declared shape, not widened)

1. **`EscalationRequest` predicate function is not implemented.** Spec §4.3 declares
   `func EscalationRequest(p PolicyPredicate, obs strategy.NaturalObservation) (bool, EscalationReason)`
   but does not specify the predicate semantics (which feature thresholds, comparison direction, or how
   the observation maps to `ValueFeatures`). Implementing it here would invent semantics; the roster
   builder and the observation encoding are the world/cognition wiring (L2/L3). L1 declares the
   escalation-request/candidate *types* (`PolicyPredicate`, `FeatureThreshold`, `EscalationReason`,
   `EscalationCandidate`) exactly as the design declares; the predicate function is deferred.
2. **`CostProxy` projected-token term has no declared per-token proxy rate.** Appendix A.2's `CostProxy`
   carries only `ConstantMicrosPerCall` + projected prompt/completion tokens, so the unpriced proxy is
   implemented as `ConstantMicrosPerCall + ProjectedPromptTokens + ProjectedCompletionTokens` (the
   projected-token term at unit proxy weight); the priced path uses the `CallPrice` rates over the same
   projected usage. If a proxy rate is intended, a later round should add it to the type.
3. **The C.2 frozen value-model instance is not baked as a production constant.** Spec §11 keeps the
   feature set and weights (4/3/2/2/1/5) operator-confirmable, so L1 provides the generic projection
   and pins the C.2 weights/feature names only in the hermetic tests; no `VS2ValueModel()` constructor
   is added.
4. **Degenerate `Select` never returns an error.** The construction-time refusal of
   `Priority != nil && Selector == nil` is `NaturalCognition.validate` (spec §4.1 item 5), owned by L2;
   L1's `Select` is the nil-`Priority` pure path only and always returns a nil error.

## Commands and results (2026-10-13, CST)

Go toolchain is not on the ambient PATH: use
`GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH` (`go1.24.13 linux/amd64`).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test ./internal/cognition/protocol/... ./internal/cognition/...` | passed (`protocol 0.012s`, `cognition 1.678s`) |
| `go test -race ./internal/cognition/protocol/...` | passed (`1.025s`) |
| `gofmt -l` over every tracked `.go` file (`.git`/`.tmp` pruned) | empty (clean) |
| `git diff --check` | clean |
| `git status --porcelain` | only the 5 in-scope protocol files (2 modified/new tests + 3 source) |

## Revision binding

- Branch: `feature/25-vs2-l1`; base: `fa4eba43f4d57d1cc0e18512e35ed207e126a6f7`.
- Source/test blob hashes at this revision:
  - `internal/cognition/protocol/wake.go` `2b2cb51cbccb4c4bfe4df4f76d9d605a877fda6f`
  - `internal/cognition/protocol/priority.go` `353dbd65b6dd7698e46fd5db3f9d63da5340f44f`
  - `internal/cognition/protocol/valuemodel.go` `da803b51af1f7a8609afd4fcf617876b874917d9`
  - `internal/cognition/protocol/wake_test.go` `65f8cb4a75c1762ed3497b71bd846b57d3642ef9`
  - `internal/cognition/protocol/valuemodel_test.go` `f66013ed01e9dd83e6a2de2f24ef648dc82d132b`
- L1 commit on `feature/25-vs2-l1`: `41f5cc0528211fd6f0e612ee26749ea3bc083c65` (the commit that added
  the five protocol files and this ticket; this addendum records its hash).
