# P0 transactional transitions and replay

Status: done

## Issue

S0–S2 prove a read-only unified component protocol. P0 still cannot change state through an authoritative kernel, record causal events, or replay a transition.

## Objective

Implement the narrow S3 vertical slice: snapshot-bound typed patch planning, atomic deterministic commit by a kernel-owned capability, append-only typed events, and replay that reconstructs identical state and numerical projections for built-in and dynamic components.

## Acceptance criteria

- Public strategy-facing readers expose no commit capability.
- Stale/invalid/unauthorized patches fail without partial state mutation or event append.
- A batch of simultaneous patches has explicit deterministic ordering and conflict behavior independent of caller order.
- Accepted transitions produce typed events with world versions, cause, rule/schema/projection references, component deltas, and metric deltas.
- Replay from a seed and event sequence reaches the same authoritative state and metrics.
- Tests cover both storage implementations, atomic failure, conflict, replay tampering and deterministic ordering.
- Bounded validation uses the local Go toolchain and records exact results.

Need-review: yes
Need-test-cases: yes

## Blockers

None.

## Comments

- 2026-09-28 — assistant: Opened after S0–S2 passed local gates and independent re-review at `dff8c25`. One writer owns S3; review follows implementation.
- 2026-09-28 — worker: Implemented snapshot-bound existing-field patch staging, kernel-owned sorted atomic commit and hash-chained typed event log, bounded closed-value codec, and seed/registry replay. Conflict losers are whole proposals; all plans are checked before collision resolution; accepted keys are unique across the log. S3 is in-process only and metadata validation is not S5 intention validation; executable migrations, durable disk transactions, and trusted log authentication remain later work. Updated `docs/P0-SPEC.md` contract. Evidence on working tree: local Go toolchain `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH"`; `timeout 180s make check` passed (format, formatter failure probe, tests, race tests, vet, build); `timeout 90s go test ./internal/kernel ./internal/sim -count=1 -run 'Test(TransactionalTransitionConformance|EnergyStorageTransitionEventEquivalence|CommitRejectsInvalidPlansAtomically|ConflictAndOrderingIndependentOfInputOrder|VersionOverflowReservesOnlyWinners|MultiComponentPatchStagesAtomically|ReplayRejectsTampering|ValueCodecRoundTrip|ValueCodecRejectsMalformedOrDeepInput)$' -v` passed; `git diff --check` passed; `test -z "$(git diff --cached --name-only)"` passed. Review gate remains open for parent independent review; no commit or push.
- 2026-09-28 — worker: Addressed reviewer P1/P2 blockers. A committed timestamp closes its batch, including across successive calls; replay now groups accepted same-time events, replans from the shared snapshot, and compares ordered bodies and hashes. Added tests for rejecting a same-time or earlier follow-up without state/log mutation, replaying a two-event same-time batch with a collision loser, and rejecting a rehashed same-time key reorder. Clarified in `docs/P0-SPEC.md` that S3 field deltas serve as typed payload, visibility awaits perception, and accepted-only replay cannot attest rejected contenders. Latest working-tree gates: `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH"; timeout 180s make check` passed; focused kernel tests passed; `git diff --check` and no-staged-files check passed.
- 2026-09-28 — assistant: Fresh read-only re-review `c6a1fcd4-b448-468f-83f3-bce3c4fca26d` verified both fixes, found no issue, verdict OK with notes. Parent ran `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check && git diff --check` successfully on the reviewed working tree. S3 does not claim crash durability, cross-schema migration, rejected-intention evidence, or visibility semantics.
