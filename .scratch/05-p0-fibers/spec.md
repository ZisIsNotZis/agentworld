# P0 actor fibers and scheduling

Status: done

## Issue

The kernel can commit and replay state transitions, but simulated actors cannot yet sleep, wake, perform timed activities, or react to interruptions. P0 needs a deterministic event-driven time loop without one resident goroutine per person.

## Objective

Implement the narrow S4 slice: explicit actor-fiber records, wake conditions, pending activities, deterministic ready batches, bounded evaluation workers, and checkpointable in-memory scheduler state. Integrate with the S3 kernel without allowing worker completion order to affect committed outcomes.

## Acceptance criteria

- Each actor has persistent identity, activity, and next wake as data; dormant actors consume no resident goroutine.
- Simulated time advances to the next meaningful wake/event, not a wall-clock tick.
- Activity completion and interruption have explicit deterministic lifecycle and no duplicate wake.
- Workers may evaluate ready actors concurrently but produce an ordered result batch from a common world snapshot.
- Scheduler state can be snapshotted and restored in memory with equivalent next-wake behavior; disk durability is not claimed.
- Tests cover ties, cancellations, interruptions, stale activities, deterministic worker-order independence, and large dormant populations.
- Existing S0–S3 tests and quality gates remain green.

Need-review: yes
Need-test-cases: yes

## Blockers

None.

## Comments

- 2026-09-28 — assistant: Opened after S3 commit `248acac` and clean fast-forward integration to master. S4 is a distinct writer-owned slice.
- 2026-09-28 — worker: Implemented in-memory actor fibers, indexed cancellable coalescing wake heap, checked activity tokens, deterministic single-snapshot bounded worker step, conditional winner effects, atomic kernel head guard, and validated in-memory snapshot/restore. Added lifecycle, tie, event non-interruption, worker-order, rollback, corruption, and 20,000-dormant-fiber tests plus benchmark. Updated P0 specification and implementation plan. On working tree with local Go 1.24, `timeout 180s make check` passed (format, test, race, vet, build); `timeout 90s go test -race ./internal/scheduler -count=5` passed; `timeout 90s go test ./internal/scheduler -run '^$' -bench '^BenchmarkDormantRegistrationAndJump$' -benchtime=3x -benchmem` observed 5,057,774 ns/op, 6,301,408 B/op, 273 allocs/op for 20,000 actors and one time jump; queue measured 1 before and 0 after; worker count bound was verified for 1 and 4. `git diff --check` and no-staged-files check passed on the final source revision. S4 intentionally has no disk persistence or exclusive kernel write lock; external kernel writers are detected at guarded commit but not prevented after publication. Independent review remains open; no commit or push.
- 2026-09-28 — worker: Addressed follow-up fresh-review blockers (three P1, one P2). A future interruption now stays token-bound and pending, leaving earlier wakes active and preventing replacement until delivery; cancellation invalidates its timer. Guarded kernel commit returns its post-commit head under the lock, and a deterministic post-commit external-writer interleaving test proves the scheduler does not adopt that writer's tip. In-memory snapshot origin IDs reject other seeded kernels even at world version 0 and empty tip. The heap maintains per-actor wake indexes and incrementally updates minima; removing a minimum recomputes only that actor. Added pending/earlier-wake/cancel/restore/corrupt-timer, interleaving, wrong-genesis, min-removal tests and a 10,000-queued-actor benchmark. Current local Go 1.24 evidence: `timeout 180s make check` passed; `timeout 90s go test -race ./internal/scheduler ./internal/kernel -count=5` passed; `timeout 90s go test ./internal/scheduler -run '^$' -bench '^Benchmark(ManyQueuedActors|DormantRegistrationAndJump)$' -benchtime=3x -benchmem` measured 10,000 queue entries at 9,253,577 ns/op, 7,996,834 B/op, 30,305 allocs/op, and 20,000 dormant fibers/one jump at 9,119,503 ns/op, 6,300,970 B/op, 280 allocs/op. `git diff --check` and no-staged-files check passed on the final source revision. Independent re-review still required; no commit or push.
- 2026-09-28 — worker: Resolved remaining P2: `EffectStop` removes wakes through `wakeQueue.byActor` rather than sorting/scanning the full queue for each actor. Added `TestEffectStopManyActorsClearsOnlyTheirWakes`, which stops 1,000 actors with two future wakes each, preserves a survivor wake, and validates restored continuation. `BenchmarkEffectStopManyActors` (step timing only; 10 iterations each) observed 1,000 actors at 6,786,893 ns/op, 1,522,585 B/op, 7,115 allocs/op and 4,000 actors at 34,381,631 ns/op, 6,942,876 B/op, 28,216 allocs/op. On this revision with the local Go 1.24 toolchain, `timeout 180s make check` passed; `timeout 90s go test -race ./internal/scheduler ./internal/kernel -count=5` passed; `git diff --check` and no-staged-files check passed.
- 2026-09-28 — assistant: Fresh final review `4ad6a5ef-a4b0-480d-926b-25289bcce6df` verified the stop-index fix and its 1,000-actor test, no remaining findings, verdict OK with notes. Parent independently ran `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check`, `timeout 90s go test -race ./internal/scheduler ./internal/kernel -count=5`, and `git diff --check` successfully on the final working tree. Process-local snapshot origin is not a durable checkpoint identity; external kernel writes after publication are detected at the next step.
