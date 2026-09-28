# P0 observation and intention boundary

Status: done

## Issue

Actor fibers can wake and the kernel can commit patches, but the evaluator still produces low-level kernel proposals. P0 requires a safe actor-facing Observation → Intention → world-validation boundary before introducing strategies or model cognition.

## Objective

Implement S5 as a narrow end-to-end boundary: actor-specific observation derived from authorized state, typed attempted actions, deterministic precondition validation and simultaneous conflict resolution, explicit accepted/rejected intention evidence, and one scheduler step that submits world outcomes without granting actor code component-patch or kernel-commit authority.

## Acceptance criteria

- Actor-facing evaluator cannot receive hidden world state, another actor's private state, a kernel handle, or patch authority.
- Intention encodes an attempt, not guaranteed results; covered outcomes are determined by world rules.
- All actors at one timestamp decide from a consistent prior world version; scheduling completion order cannot decide conflicts.
- Rejected intentions are logged with reason and do not change authoritative state.
- Accepted actions create causally linked typed events with correct actor-visible consequences.
- Tests cover hidden-state leakage, stale knowledge/version, invalid preconditions, conflicting simultaneous attempts, rejection without mutation, and replay of accepted world changes.
- Preserve S0–S4 gates; keep domain scope minimal and explicitly defer general strategy VM and complete survival simulation.

Need-review: yes
Need-test-cases: yes

## Blockers

None.

## Comments

- 2026-09-28 — assistant: Opened after S4 commit `521a3f2` was reviewed, validated, and fast-forwarded to master. One delegated writer owns the S5 seam.
- 2026-09-28 — worker: Implemented a dynamic cache-stock descriptor, actor-safe observation/withdrawal rule and serialized trusted scheduler runner, deterministic per-cache collision outcomes, and separate in-memory attempt journal. Added focused black-box world tests for privacy, preconditions, stale versions, ordering across workers, atomic failure, linked events, and accepted replay. Scoped review and independent reviewer verdict still pending; no commit or push. Validation on working tree: `go test -count=10 ./internal/world`, `make check` (including race, vet, build), and `git diff --check` passed with local Go 1.24 toolchain.
- 2026-09-28 — worker: Follow-up P1 conservation review found float64 stock subtraction at 9007199254740994 can round by two, as can symmetric energy addition. Reproduced both with failing black-box tests; world rule now requires each stored scalar delta to equal exactly one joule or rejects as `InvalidNumber`. Added tests for both rejection/no mutation/no accepted event and a representable boundary withdrawal that still commits. On this working-tree revision, `go test -count=20 ./internal/world`, `timeout 180s make check` (test, race, vet, build), and `git diff --check` passed with local Go 1.24 toolchain. No commit/push/staged files; independent review still pending.
- 2026-09-28 — worker: Follow-up P1 fractional review showed float64 `after-before == 1` rounds a non-unit exact binary difference for energy `0.1 -> 1.1`. Added a failing regression, then changed validation to compare each stored operand as an exact `big.Rat`; this rejects the non-unit fractional transfer as `InvalidNumber` without mutation or accepted event while retaining valid fractional `0.5 -> 1.5` and large-boundary transfers. On the updated working tree, `go test -count=20 ./internal/world`, `timeout 180s make check` (including race, vet, build), and `git diff --check` passed using local Go 1.24.
- 2026-09-28 — assistant: Fresh final review `73bde88e-fe7b-42e2-86e2-c8770923b64c` confirmed exact stored-value conservation and no new issue, verdict OK. Parent independently ran `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check`, 20 repeated world tests, and `git diff --check` successfully on this revision. Rejected-attempt journal is in-memory and not reconstructible from accepted-only kernel replay; actor callback boundary is not a hostile-code sandbox.
