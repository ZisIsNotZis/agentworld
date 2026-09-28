# Durable checkpoint and branch lineage

Status: claimed

## Issue

S6 can replay accepted component events from the original seed but cannot persist and restore scheduler state or branch from a durable checkpoint. Long-running development and policy experiments require a verifiable continuation boundary.

## Objective

Implement the narrow P0 durable checkpoint: persist a versioned kernel/component snapshot, event-log tip and segments or equivalent append-only evidence, actor fibers, pending wakes/activities, strategy references/bindings, deterministic random positions, and metric projection versions. Restore and branch from it without changing causal identity; reject corrupt or incompatible artifacts.

## Acceptance criteria

- A checkpoint can be written, verified, restored in a new process, and continued with the same future events/metrics as uninterrupted execution under the same versions and inputs.
- Branches retain ancestry, intervention/configuration, and explicit compatibility class; changed strategy parameters may deliberately diverge after the shared checkpoint.
- Serialization is canonical and bounded; partial writes cannot masquerade as completed checkpoints.
- Schema/rule/strategy/projection mismatches fail closed unless an explicit migration exists; do not invent migration transforms.
- Accepted event and state evidence is durable enough to inspect; rejection history and actor perception require a scoped decision and honest contract.
- Tests cover round trip, continuation equivalence, branch divergence, tampering/truncation, pending activity/wake restoration, and numerical projection consistency.
- Full quality gates and fresh review pass before integration.

Need-review: yes
Need-test-cases: yes

## Scope decision and lane board

P0 persists accepted events and the typed attempt/outcome journal; full-run rejection counts must survive restore. Actor perception packets are not yet modeled and must not be claimed as complete trajectory. A checkpoint is an immutable, bounded, canonical single-machine bundle published atomically at a quiescent survival boundary. Cross-schema migration fails closed. The initial branch intervention changes only a versioned strategy weight.

This is multi-seam. Writers run sequentially in the shared feature branch, with exclusive source ownership and a durable API handoff after each stage; no two writers share the checkout at once. The parent reviews interfaces and integrates before the next dependent seam.

| Lane | Claimed files / contract | Gate and handoff |
|---|---|---|
| component | `internal/component/snapshot*.go`, tests: canonical logical snapshot and registry fingerprint | Round-trip/equivalence tests; exported API and version contract |
| kernel | `internal/kernel/history*.go`, tests: bounded event decoding, replay and portable history verification | Tamper/replay tests; portable head API |
| scheduler | `internal/scheduler/portable*.go`, tests: cross-process-rebindable state without weakening same-process restore | Pending-wake/activity tests; rebind API |
| strategy | `internal/strategy/codec*.go`, tests: exact versioned policy/binding data | Compatibility/round-trip tests; codec API |
| journal | `internal/world/survival_journal*.go`, tests: ordered typed attempts and event links | Rejection-history integrity tests; codec API |
| storage | `internal/checkpoint/store*.go`, tests: bounded framing, digest, atomic file publication | Fault-injection tests; immutable bundle API |
| integration | `internal/world/survival.go`, new checkpoint tests, CLI and scoped docs | Subprocess continuation/branch demo and full gates |
| review | Entire integrated diff, read-only | Independent findings and verdict |

The exact contract may be narrowed by measured implementation needs, but cannot silently omit the attempt journal, restore equivalence, or corruption checks.

## Blockers

No user-only blocker. The above interface seams need staged implementation and review.

## Comments

- 2026-09-28 — assistant: Opened after first survival demo commit `080eb3d` was integrated. This is not a full global checkpoint design; P0 only needs one-machine durable continuation and evidence.
- 2026-09-28 — assistant: Read-only Designer run `b816ce2b-4e5b-464e-83ce-f522ad284d5f` mapped portable state, atomic bundle publication, branch lineage, and eight ownership seams. Resolved historical rejection evidence in favor of durable typed attempt journal because full-run audit is an approved goal; perception history remains explicitly incomplete.
- 2026-09-28 — assistant: Component seam implemented by worker `235d32df-cca0-4d10-acc0-ddd2cb5718da` and resumed twice to address bounded encoding, aggregate slot validation, and nested decode amplification. Scope exception authorized `internal/component/validate.go` and `internal/sim/value_codec.go` for shared indexed validation and no-materialization structural counting. Final independent review `ab6dd6d7-707a-4601-a140-2d2f256f55df` found no issue (OK with notes); its optional encode-side aggregate-node test remains a follow-up quality improvement. Parent ran PATH-prefixed `timeout 180s make check`, `git diff --check`, and targeted snapshot benchmarks: nested-input rejection 5,482 B/op, over-budget export 217,664 B/op, 100-record/64-field dynamic round trip 16,475,026 B/op. Format-v1 bounds: 32 MiB wire, 65,536 records/field slots, 16,384 total value nodes. Committed component seam at `9c44d38` on feature branch; bundle integrity remains unimplemented.
- 2026-09-28 — assistant: Kernel history seam captured immutable genesis at `New`, exported bounded accepted history, and restored with replay/metric/snapshot verification. Review `b2020cd4-bf64-4c01-8b7b-9588f680ca54` exposed O(n²) duplicate-key scans; worker replaced them with an atomic accepted-key index and added a subprocess restore/continuation test. Fresh re-review `6b288aff-68eb-474a-9b45-c45aaea714d7` found no issue (OK with notes). Parent ran PATH-prefixed `timeout 180s make check`, `git diff --check`, and distinct-time restore benchmarks: 1,024 events 38.3 ms/19.9 MB, 4,096 events 176.9 ms/79.8 MB, one iteration each. Committed as `2a73e46` on feature branch; file publication, scheduler, policy, and attempt journal remained unimplemented.
- 2026-09-28 — assistant: Scheduler portable seam added bounded canonical export and verified-head rebind without weakening in-process restore. Fresh review `09081c0a-ce38-4dfd-b039-4c35a775cb69` found no issue (OK with notes); wake-count over-limit test and worst-case restore measurement remain optional validation gaps. Parent ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully; committed as `3c3af48` on feature branch. Full cross-process world restore still awaits integration.
- 2026-09-28 — assistant: Strategy seam encoded full format-v1 policy content and explicit per-actor overrides. Supervisor authorized narrow `policy.go` change to preserve override presence; restore requires an independently constructed expected registry and rejects same-Ref changed content. Fresh review `be5d882c-be7b-4561-a774-838ba351a4ea` found no implementation defect (OK with notes) and pointed out an unknown-parameter test that accidentally failed at ordering first. Parent corrected the test to mutate the second override ID, then ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully. World integration must supply the expected registry.
