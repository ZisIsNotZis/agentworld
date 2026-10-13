# Ticket 26 — VS2 lane L2 (world escalation gate + checkpoint binding)

Status: implemented on `feature/26-vs2-l2`; **not merged** to master. Branch base
`7d9b22dc49ae5d9b185bb5c50246adbc163834d1` (the verified L1 tip, 2026-10-13). Started
2026-10-13T20:34+08:00 (CST), actor: worker (single writer). Code + hermetic tests only.

## Objective

Implement VS2 lane L2 exactly as declared in `.scratch/24-vs2-attention/spec.md` §4.1 (items 3–5),
§7 VS2-EC-2/VS2-EC-5 and §10 (lane board L2): generalize the world's hour-only wake gate to the L1
`protocol.EscalationSelector` seam over a per-hour `[]EscalationCandidate` roster built from the
claim-wake set; refuse a priority policy without its selector at construction with the typed
`ErrNaturalRunner`; and make the checkpoint cognition block bind the full wake policy (including the
VS2 priority-policy identity) with a restore-time fingerprint refusal. The nil-`Priority`/
nil-`Selector` paths stay byte-identical to VS1.

## Contract (what this lane owns)

- Sole writer of `internal/world/natural_cognition.go` (generalized gate + `Selector` seam + the
  construction-time refusal), the claim-loop hunks of `internal/world/natural.go` (`Natural.allocate`),
  `internal/world/natural_cognitive_checkpoint.go` (additive cognition-block wire extension + the
  restore-time policy-fingerprint refusal helper), and the new test file
  `internal/world/natural_cognition_priority_test.go`.
- Read-only everywhere else (one call site added in `internal/world/natural_checkpoint.go:restoreNaturalSections`).

Explicit non-goals (owned by other lanes or out of scope):

- No edit to `internal/cognition/**` (L1's protocol surface is consumed read-only; the world imports
  only `internal/cognition/protocol`, per the existing import guard).
- No change to `cmd/**`, `internal/kernel`, `internal/scheduler`, `internal/component`, `internal/sim`,
  `internal/strategy`, or the frozen `natural/5..9` policy bytes / frozen v5–v9 evidence.
- No L3 work (the arm-local accumulator, allocation records, PRNG, price adapter) and no L4 work
  (`cmd/vs2`, `internal/experiment`, the priority-arm wiring).
- No change to the `NaturalCognition` half-wired-seam refusal that VS1 already declares.

## Requirements (from dispatch + VS2 spec lane board L2)

1. `NaturalCognition` gains an optional `Selector protocol.EscalationSelector` field (nil => the
   `protocol`/`WakePolicy` path).
2. `NaturalCognition.validate` refuses `WakePolicy.Priority != nil && Selector == nil` with the typed
   `ErrNaturalRunner`, mirroring the existing half-wired seam refusal (§4.1 item 5, VS2-EC-2).
3. `Natural.allocate` builds the per-hour escalation candidate roster from the claim-wake set and routes
   selection through the selector, in the design's declared order (roster sorted by actor, §4.5/§8.1);
   only the selected actors enter the Role A seam.
4. The checkpoint cognition block wire is extended additively and byte-compatibly to carry the
   canonical priority-policy identity; the full-policy fingerprint is recomputed over the decoded
   policy; restore refuses a supplied policy whose fingerprint differs from the recorded one
   (VS2-EC-5). Existing VS1 bundles decode/restore unchanged.
5. Hermetic tests for all of the above.

## Measurable acceptance indicators

- `go build ./...` passes.
- `go vet ./...` passes.
- Targeted `go test -count=1 -timeout 20m ./internal/world/ -run 'Cognitive|Cognition|Checkpoint|Replay|Priority|Escalation'` passes.
- `go test -count=1 -race -timeout 20m ./internal/world/ -run 'Cognitive|Cognition|Checkpoint|Replay|Priority|Escalation'` passes.
- The full `go test -count=1 -timeout 30m ./internal/world/` passes.
- `go test -count=1 ./internal/cognition/...` passes.
- `gofmt -l` over every tracked `.go` file is empty.
- `git diff --check` is clean.
- A test fails if the priority/selector refusal is removed, if the selection routing or roster feed is
  removed, if the priority identity is not carried by the wire, or if the restore accepts a mismatched
  fingerprint.

## Implementation

### `internal/world/natural_cognition.go`

- `NaturalCognition` gains `Selector protocol.EscalationSelector`.
- `validate` adds `if c.WakePolicy.Priority != nil && c.Selector == nil { return fmt.Errorf("%w: priority policy requires an escalation selector", ErrNaturalRunner) }`.
- `naturalSeamEscalates(hour)` (hour-only) is replaced by `naturalSeamSelection(hour, roster)`: it calls
  the wired `Selector.Select` when non-nil, else the degenerate `protocol.WakePolicy.Select` (the
  nil-`Priority` path); it returns the selected actor set as a membership map. A nil cognition returns a
  nil map (selects nothing).
- `naturalSeamBudget()` returns the declared budget view: `{CallsBudget: N, CallsRemaining: N}` from
  `Priority.CallCeiling`, or the zero budget on the nil-`Priority` path (the selector owns the mutating
  spent-call count).
- `naturalClaimWake{id, actor, obs}` holds one claim-wake actor and its frozen observation.
- `naturalEscalationRoster(hour, wakes)` builds the `[]EscalationCandidate` from the claim-wake set and
  sorts it by ascending actor id. `naturalCandidateFeatures(obs)` fills the declared observation-only
  C.1 raw rows via `naturalFeatureInt` (the integer projection).
- `naturalSeamOverride` no longer gates on the hour internally: the caller (`allocate`) has already
  selected the actor.

### `internal/world/natural.go` (`Natural.allocate`)

The loop is split into: (1) collect the claim-wake set for the hour, validating each actor and computing
its observation once; (2) when `f.cognition != nil`, call `naturalSeamSelection(h, naturalEscalationRoster(h, wakes))`
once for the hour; (3) evaluate each wake and call `naturalSeamOverride` only for selected actors. The
claim admission (`naturalAllocateGathers`), the claim order, and the `WakeNeedThreshold`/`WakeBirth`
filter are unchanged, so a nil-cognition run is byte-identical.

### `internal/world/natural_cognitive_checkpoint.go`

- `encodeNaturalCognitiveBlock` appends, after the bindings and only when `WakePolicy.Priority != nil`, a
  length-prefixed canonical full-policy identity (`WakePolicy.Identity()`), which embeds the priority
  policy. A nil `Priority` writes nothing, so the VS1 block bytes are unchanged.
- `decodeNaturalCognitiveBlock` reads the trailing identity, unmarshals it into a `protocol.WakePolicy`,
  requires a non-nil `Priority` and matching `Name`/`PeriodHours`/`PhaseHours`, and reinstates
  `WakePolicy.Priority`. A VS1 bundle has no trailing bytes and decodes with a nil `Priority` unchanged.
- `verifyCognitiveWakePolicy(block, supplied)` recomputes the fingerprint over the decoded full policy
  and requires it to equal the recorded `WakeFingerprint`; when a supplied policy is present it must
  match the recorded fingerprint too. A nil block verifies nothing; a zero supplied policy is not
  compared.

### `internal/world/natural_checkpoint.go`

`restoreNaturalSections` calls `verifyCognitiveWakePolicy(m.cognitive, expected.CognitiveReplayWake)`
first, so a mismatched bundle/supplied policy is refused before any world is rebuilt.

## Recorded gaps / under-specification (implemented to the declared shape, not widened)

1. **Candidate feature derivation is only partially declared.** Appendix C.1 declares the *sources* of
   every feature but no derivation for the derived rows (`decision_sensitivity`, `urgency`, `impact`,
   `propagation_risk`), no aggregate row for `prior_reliability`, no string encoding for `last_denial`,
   and L1 explicitly deferred the `EscalationRequest` predicate (ticket 25 gap 1). L2 therefore fills
   only the directly-sourced observation-only rows (`energy`, `hunger`, `capital`, `granary_stock`,
   `last_stored_meal_hour`, `stocked_home_slots` = count of own home slots with stock ≥ 1); the derived
   rows, `prior_reliability`, `last_denial`, and `escalation_requested` stay zero. The C.2 weights all
   sit on the derived rows, so the value model cannot rank until a later round declares those
   derivations; the roster/selection plumbing implemented here is otherwise complete.
2. **The `Select` budget argument's provenance is not declared.** The interface doc says the accumulator
   is "seeded from the declared call ceiling N (BudgetState.CallsBudget)" while also stating the
   accumulator owns the spent-call count and budget state. L2 passes the declared ceiling
   (`CallsBudget = CallsRemaining = Priority.CallCeiling`) each hour and leaves the mutating
   `CallsSpent` to the selector; a later round should confirm whether the world or the selector owns
   `CallsRemaining` on subsequent hours.
3. **The recorded-seam replay does not carry a `Selector`.** `replayNaturalJournalWithCognitive`
   (`internal/world/natural_journal.go`, not an L2 lane file) wires
   `NaturalCognition{Seam: ..., WakePolicy: seamReplay.wake}` with no `Selector`. With L2's new
   construction refusal, a priority-bearing VS2 replay would be refused by `SetCognition` until L4
   extends that replay wiring (or the checkpoint restore path) to supply the arm-local selector. This
   is out of L2's declared file scope and is recorded for L4; it does not affect any existing VS1
   (nil-`Priority`) bundle.
4. **The trailing wire section carries the full-policy identity, not the priority sub-object alone.**
   `protocol.PriorityPolicy` has no exported canonical-identity method and L2 may not edit
   `internal/cognition/**`, so the extension reuses the exported `WakePolicy.Identity()` (which embeds
   the priority policy) as the carried canonical bytes. The recorded fingerprint is still computed over
   the full policy, so the priority policy is bound as VS2-EC-5 requires.

## Commands and results (2026-10-13, CST)

Go toolchain is not on the ambient PATH: use
`GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`, `PATH=$GOROOT/bin:$PATH`
(`go1.24.13 linux/amd64`).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test -count=1 ./internal/cognition/...` | passed (`cognition 1.033s`, `protocol 0.003s`) |
| `go test -count=1 -timeout 20m ./internal/world/ -run 'Cognitive\|Cognition\|Checkpoint\|Replay\|Priority\|Escalation'` | passed `ok … 309.429s` (run before the new test file was added; covered the pre-existing cognition/checkpoint/replay suite) |
| `go test -count=1 ./internal/world/ -run '<the 8 new test names>' -v` | all 8 `PASS` (`ok … 6.485s`) |
| `go test -count=1 -race -timeout 20m ./internal/world/ -run 'Cognitive\|Cognition\|Checkpoint\|Replay\|Priority\|Escalation'` | passed `ok … 649.524s` (includes the new test file) |
| `go test -count=1 -timeout 30m ./internal/world/` (full package) | passed `ok … 796.002s` (EXIT=0) |
| `gofmt -l` over every tracked `.go` file | empty (clean) |
| `git diff --check` | clean |

New test names (`internal/world/natural_cognition_priority_test.go`):
`TestNaturalCognitionRequiresSelectorForPriorityPolicy`,
`TestNaturalSelectorGateReproducesVS1EscalationPath`, `TestNaturalSelectorSeamDrivesEscalation`,
`TestNaturalCandidateFeaturesObservationOnly`, `TestNaturalCognitiveBlockPriorityIdentityRoundTrip`,
`TestNaturalCognitiveBlockNilPriorityBytesUnchanged`, `TestNaturalVerifyCognitiveWakePolicy`,
`TestNaturalSeamRestoreRefusesMismatchedWakePolicy`.

## Revision binding

- Branch: `feature/26-vs2-l2`; base: `7d9b22dc49ae5d9b185bb5c50246adbc163834d1` (L1 tip).
- Source/test blob hashes at this revision (working tree, pre-commit):
  - `internal/world/natural.go` `588f259ec0ea12945ab2260440ad455a4a949167`
  - `internal/world/natural_cognition.go` `469da92e595d4178db81d012a2993026276b6e23`
  - `internal/world/natural_cognitive_checkpoint.go` `7121d9aeaab73482e5388255a48849031b405f3c`
  - `internal/world/natural_checkpoint.go` `d2bfea3f1a95a3d47508c0d15c01819f7610a85b`
  - `internal/world/natural_cognition_priority_test.go` `d6efa20233e40e1cee6b35bb960007bc92c2edcf`
- Commit hash on `feature/26-vs2-l2`: recorded in the revision addendum below.
