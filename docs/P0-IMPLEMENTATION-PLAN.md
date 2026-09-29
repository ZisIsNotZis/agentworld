# P0 implementation plan

Status: S0–S6 bounded architecture smoke and S7a single-machine checkpoint implemented; full P0 remains open

This plan sequences the executable work described by [P0-SPEC](P0-SPEC.md) and preserves the unified component boundary accepted in [ADR 0005](adr/0005-unified-hybrid-components.md). Go is provisional while P0 measures correctness and cost.

## Implemented foundation

### S0 — Reproducible project gates

- A dependency-free Go module establishes the provisional language baseline.
- The Make targets `fmt-check`, `test`, `race`, `vet`, `build`, and `bench` are the repository gates.
- No placeholder command is provided before a runner exists.

### S1 — Stable simulation values

- Numeric IDs, exact microsecond simulation time and durations, world versions, and checked entity allocation establish stable primitives.
- A project-owned counter-based random stream makes output a function only of seed, stream ID, and position.
- The closed value algebra retains its declared kind while distinguishing present, missing, unknown, and inapplicable states.
- Composite values are validated, defensively copied, and canonically ordered where order has no semantic meaning.

### S2 — Unified immutable components

- A validating builder freezes component descriptors into an immutable schema registry.
- Energy uses a hidden compiled columnar store; fatigue uses a hidden registry-driven sparse store.
- Consumers receive only `component.Reader` and immutable read, scan, and projection results.
- Authorization is an opaque snapshot capability. The foundation serves exactly one world version.
- Scalar batch columns provide bulk copies without per-field interface dispatch.
- Affine projections include typed value state, uncertainty, projection and schema versions, unit, source fields, entity identity, and world-version provenance.
- One read-protocol conformance suite covers both storage classes; a separate same-schema energy test checks logical read/scan/project equivalence.

## Transition and scheduling slices

- S3 adds kernel-held patch planning and commit authority, authoritative events, atomicity, and deterministic in-process replay. Cross-schema migration and durable storage remain later work.
- S4 adds dormant actor fibers, indexed wake conditions, timed activities, deterministic ordered worker batches, and in-memory scheduler snapshots validated against a matching kernel head. It does not add disk durability or strategy semantics.
- S5 adds an actor-specific observation and one public-cache withdrawal intention, checked against the common kernel snapshot. A single-winner cache field collision links accepted actions to deterministic energy/stock events; a separate in-memory journal retains rejected outcomes without state mutation. It does not provide a strategy VM, crash-atomic attempt storage, or replay of rejections.

### S6 — Bounded strategy and survival demo

- Format-v1 strategy data pins an exact policy version with a fixed candidate/evaluation budget; actor bindings override parameter values without copying action specifications. A value-only observation limits strategy access to own energy/hunger and configured public food views.
- A separate survival adapter revalidates choices against the common scheduler snapshot; hourly wakes, one-winner cache contention, finite food and energy, complement hunger, and zero-energy stopping provide a bounded causal loop. It does not modify the S5 runner or its observation contract.
- `go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1` prints JSON including accounting, rejected reasons, CPU/wall durations in nanoseconds (`cpu_time_ns`, `wall_time_ns`) and accepted-state replay. Changing `-eat-cost=0` with the same seed tests policy sensitivity; altering `-seed` tests resource sensitivity.
- The typed attempt journal includes the pinned reference and rejected attempts. Accepted-event replay verifies the same seed/registry, canonical event bytes, tip, scanned components and projections, but cannot reconstruct wakes or authenticate policy provenance by itself.

### S7a — Durable survival checkpoint boundary

- `cmd/survival` can create a checkpoint at a quiescent hourly step, resume with a different worker count in a new process, and fork with an explicit future Eat-weight change. It uses an immutable, no-clobber, atomically published five-section bundle with a digest and parent lineage reference.
- Restore independently constructs the scenario schema/policy, verifies genesis, accepted event replay and projections, scheduler portable head/fibers/wakes within the scenario horizon, exact actor bindings, typed attempt links and original config, and manifest identity before returning a world. Parent state and projection hashes must match the full same-tip head or a replayed accepted prefix. Incompatible versions/configuration are rejected; no migration is available.
- A branch keeps the original historical policy weight in the journal, verifies the common source checkpoint, and records the active intervention and parent digest/head. Rejection reasons are retained but not independently regenerated; full perception history, adversarial authentication and per-step durability remain outside this slice.

### Synthetic food-flow pilot (bounded follow-up, not P0 completion)

The independent v1 runner, evidence journal and version-isolated five-section checkpoints now support q8/q3/q0 at 16 actors over 168 hours. Its hourly basal event affects living actors only; stopped actors do not accumulate hunger after death. The reporter counts all rejected Gather attempts both by action and by denial reason. `cmd/foodflow` runs and reports the fixed fixture with seed/worker controls, absolute-hour prefixes, checkpoint publication and cross-process resume. Frozen viability/scarcity/extinction gates and full-run measured bytes/costs are recorded in the [pilot evidence](../.scratch/10-food-flow-pilot/spec.md). No S6 rule or checkpoint format is changed. The outcome is internal accounting and model-conditional behavior, not human survival validity or an extrapolation to P0 scale.

### Synthetic social-food pilot (v2 experiment/reporting seam, not P0 completion)

The v2 runner, journal and audited branch checkpoints are reported by `cmd/socialfood`, which publishes a verified neutral h0 bundle, records enable/disable interventions as branch children sharing that parent digest, and continues each child under the recorded flag. The reporter checks hourly produced = consumed + held + stock and initial-energy balances on every hour, cross-checks journal attempts against final witnessed dyad counters and ledger given/received units, replays accepted history to verify event links, and reports alive trajectories, survivor sets, directed dyad assistance fractions, per-patch consumed Gini (zero total defined as zero inequality), privileged gift claim-versus-truth audit and measured compute/storage costs with zero tokens. The predeclared q3/q8/q0 seed and worker matrix is recorded in the [ticket evidence](../.scratch/12-social-food-transfer/spec.md). The outcome is internal accounting and model-conditional behavior of a frozen synthetic policy: no genealogy claim, no cooperation claim, and no P0 scale extrapolation. No v1 or S6 bytes are changed.

## Next slices

S6 is an architecture smoke, not an endurance or causal-validity demonstration: the seed-7 maximum-bound probe (256 actors, requested 240 hours) ends with all actors stopped at hour 29. The four finite caches have no replenishment and become inedible by hour 21. One-winner-per-cache-hour contention and ordered actor keys visibly bias access and constrain hourly throughput; their separate contribution to extinction timing has not been isolated by an intervention. The P0 gap remains a renewable survival ecology with time-based needs and capacity-valid allocation, plus the specified birth, aging, death, health, location, weather, movement, social actions, longer horizons, fuller perception evidence, and measured scaling toward approximately 1,000 actors over decades. The separate bounded food-flow pilot tests that next mechanism; the earlier S6 probe's trajectory and costs remain in `.scratch/09-p0-scale-and-validity/spec.md`, not a claim of P0 completion. Future P0 work must still measure new scales and include the missing mechanisms before drawing endurance or causal-validity conclusions.

The S0–S5 tests do not establish durable persistence, migration, or full P0 promotion equivalence. Performance results are baselines, not pass/fail thresholds.
