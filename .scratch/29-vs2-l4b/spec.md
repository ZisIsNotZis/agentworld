# Ticket 29 — VS2 lane L4b (priority-bearing world replay / selector wiring)

Status: implemented on `feature/29-vs2-l4b`; **not merged** to master. Branch base `e6d5466`
(master after the L4a integration merge `57770d2` + the L4a ticket doc-nit fix). Started
2026-10-13T22:57+08:00 (CST), actor: worker (single writer). Code + hermetic tests only.

## Objective

Implement VS2 lane L4b exactly as declared in `.scratch/24-vs2-attention/spec.md` §7 (VS2-EC-5),
§10 (lane board L4: the wiring of the L3 arm-local accumulator into the world's `Selector` seam)
and the L2 ticket `.scratch/26-vs2-l2/spec.md` gap 3 / the L2 review observation 3: the
recorded-seam replay (`naturalSeamReplay`) and the checkpoint restore options carry only the
recorded `ClaimSeam` and `WakePolicy`; with L2's construction refusal
(`WakePolicy.Priority != nil && Selector == nil`) a priority-bearing VS2 run cannot be
journal-replayed, checkpointed, or restored. L4b extends that replay/restore wiring to
carry/supply the arm-local `protocol.EscalationSelector` so a priority run reconciles, saves,
and restores with its own deterministic selection sequence and no model call, while the
nil-Priority VS1 path stays byte-identical.

## Contract (what this lane owns — sole writer)

- `internal/world/natural_journal.go`: `naturalSeamReplay` gains the optional per-replay selector
  factory; `seamReplayOfSelector`; the replay wires the fresh selector into
  `NaturalCognition`; `ExportJournalWithReplaySelector` (the legacy `ExportJournalWithReplay`
  becomes a nil-selector wrapper over the shared internal body).
- `internal/world/natural_checkpoint.go`: `SaveResumeCheckpointWithReplaySelector` (the legacy
  `SaveResumeCheckpointWithReplay` becomes a nil-selector wrapper); `NaturalRestoreOptions` gains
  `CognitiveReplaySelector`; `restoreNaturalSections` threads it into the seam replay.
- New hermetic test file `internal/world/natural_priority_replay_test.go`.

Explicit non-goals (other lanes / frozen surface):

- No edit to `internal/cognition/**` (L3 owns the `PrioritySelector` accumulator; L4b consumes it
  read-only through the L1 `protocol.EscalationSelector` interface).
- No `cmd/vs2` (that is L4c). No change to `internal/kernel`, `internal/scheduler`,
  `internal/component`, `internal/sim`, `internal/strategy`, the frozen `natural/5..9` policy
  bytes, or the frozen v5–v9 evidence.

## Requirements and measurable acceptance indicators

1. A priority-bearing run can be journal-exported, checkpointed, and restored: the
   recorded-seam replay wires the arm-local `protocol.EscalationSelector`, so L2's
   `Priority != nil && Selector == nil` construction refusal does not reject the replay.
2. The replay re-derives the recorded selection sequence from recorded state with **zero model
   calls**: the restored history equals the live history, and every replay selector's selection
   sequence (hour, selected set, `CallsSpent`, `PRNGPos`) equals the live arm's.
3. The selector is supplied as a **per-replay factory**, not a live instance: one save, export, or
   restore runs the reference replay more than once and each replay walks the journal from
   genesis, so each invocation must start from the selector's genesis state. Reusing one
   advanced selector would diverge at the second replay and must fail the round trip.
4. The nil-Priority VS1 path is byte-identical: `ExportJournalWithReplaySelector(..., nil)` and
   `SaveResumeCheckpointWithReplaySelector(..., nil)` produce exactly the bytes/digest of the
   legacy methods.
5. `go build ./...`, `go vet ./...`, the targeted world tests, `go test ./internal/cognition/...`,
   `gofmt -l` (empty), and `git diff --check` all pass; `git status --porcelain` shows only the
   three intended paths.

## Implementation

`naturalSeamReplay` (natural_journal.go) gains `newSelector func() protocol.EscalationSelector`.
`seamReplayOf` is unchanged (nil factory) so every existing VS1 call site is untouched;
`seamReplayOfSelector` adds the factory and returns nil for a nil seam exactly as `seamReplayOf`
(a selector is only meaningful on a cognitive path, which requires a seam).
`replayNaturalJournalWithCognitive` builds `NaturalCognition{Seam, WakePolicy}` and, when the
factory is non-nil, sets `Selector = seamReplay.newSelector()` for that replay before
`SetCognition`.

`ExportJournalWithReplay` and `SaveResumeCheckpointWithReplay` are now thin wrappers over a shared
internal body that takes a prepared `*naturalSeamReplay`; the new
`ExportJournalWithReplaySelector` / `SaveResumeCheckpointWithReplaySelector` pass
`seamReplayOfSelector(...)`. `NaturalRestoreOptions.CognitiveReplaySelector` mirrors
`CognitiveReplaySeam`/`CognitiveReplayWake` as replay machinery, and `restoreNaturalSections`
builds the seam replay with it.

## Tests (hermetic, no network)

`internal/world/natural_priority_replay_test.go`:

- `TestNaturalPrioritySeamCheckpointReplayRoundTrip` — a live adaptive arm
  (`StrategyAdaptive`, `CallCeiling 64`, `RandSeed 7`, `ExplorationFraction 250000`, horizon 168)
  over a ledger-backed `RoleASeam`; `ExportJournalWithReplaySelector`, then
  `SaveResumeCheckpointWithReplaySelector`, then `RestoreNaturalCheckpoint` with a
  `ReplaySeam`. Asserts: the arm escalated at least one actor; the restored checkpoints equal the
  live checkpoints; the replay made zero model calls; the replay seam served recorded decisions;
  and every selector built by each entry point's factory reproduced the live selection sequence
  (hour, selected set, `CallsSpent`, `PRNGPos`).
- `TestNaturalNilPriorityReplaySelectorBytesIdentical` — on the VS1 path, the selector-aware
  export and save are byte-/digest-identical to the legacy methods, with zero model calls.

Falsifiability (checked by hand): removing the `wired.Selector = seamReplay.newSelector()` line
makes the round trip fail (`priority ExportJournalWithReplaySelector: invalid natural evidence
journal`), because L2's guard refuses a priority policy with no selector.

## Commands and results (2026-10-13, CST)

Go toolchain is not on the ambient PATH: use
`GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`, `PATH=$GOROOT/bin:$PATH`,
`GOTOOLCHAIN=local` (`go1.24.13 linux/amd64`).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test -count=1 -timeout 20m ./internal/world/ -run 'Cognitive\|Cognition\|Checkpoint\|Replay\|Priority\|Seam\|Digest'` | passed `ok … 508.672s` |
| `go test -count=1 ./internal/cognition/...` | passed (`cognition 1.106s`, `protocol 0.007s`) |
| `go test -count=1 -timeout 20m ./internal/world/ -run 'TestNaturalPrioritySeamCheckpointReplayRoundTrip\|TestNaturalNilPriorityReplaySelectorBytesIdentical' -v` | both `PASS` (`ok … 17.178s`) |
| `gofmt -l` over every tracked `.go` file | empty (clean) |
| `git diff --check` | clean |

The full `./internal/world/` package (~13 min) was **not** run (the dispatch makes it optional
unless budget clearly allows). Every test file that calls the changed functions
(`ExportJournalWithReplay`, `SaveResumeCheckpointWithReplay`, `restoreNaturalSections`,
`replayNaturalJournalWithCognitive`) matches the targeted `-run` pattern and passed, so the
changed paths are exercised.

## Revision binding

- Branch: `feature/29-vs2-l4b`; base: `e6d5466981a718e847795c426abc63b4917b57df` (master).
- Source/test blob hashes at this revision (pre-commit):
  - `internal/world/natural_journal.go` `75056e4522369ba54f8fce329128d5fef042faa2`
  - `internal/world/natural_checkpoint.go` `9a7b6587876656783b5dd7140fe0a6b409d8f871`
  - `internal/world/natural_priority_replay_test.go` `36267a840e2f58ed585fa16faf55f325c20ca0ce`
- Commit hash on `feature/29-vs2-l4b`: recorded in the revision addendum below.

## Revision addendum

- L4b commit on `feature/29-vs2-l4b`: `5a27c67` (adds the two in-scope source files, the new test
  file, and this ticket).

## Residual risks / declared gaps

1. **The selector's live state is not persisted in the checkpoint manifest.** Restore reproduces
   the accumulator by replaying the journal from genesis with a fresh selector, so the
   `CallsSpent`/`PRNGPos` are re-derived, not stored. This matches VS2-EC-1's declared proof
   ("replay re-derivation of the accumulator from recorded state"). A hypothetical warm restore
   that did not walk the journal would need the recorded accumulator state, which the wire does
   not carry; it is out of the declared restore path.
2. **The selector is supplied by the caller, not reconstructed by the world.** The world imports
   only `internal/cognition/protocol` (EC-6 import guard), so it cannot construct the L3
   `cognition.PrioritySelector`. The caller (L4c `cmd/vs2`, or a test) supplies the per-replay
   factory; a priority-bearing replay without one is refused by the construction guard, which is
   the intended fail-closed behaviour.
3. **L4c wiring is not exercised end to end.** This lane proves the world replay/restore seam; the
   declared live smoke cell and the matrix remain L4c/L5.
