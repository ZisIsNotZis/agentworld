# ADR 0007 — Value-based cognitive allocation as a deterministic, world-RNG-free policy function

Status: proposed for VS2 (M2)

## Context

`docs/VERTICAL-SLICES.md` makes VS2 the slice that tests the central claim: selective spending of
expensive reasoning preserves civilization-level fidelity better than uniform or periodic invocation at
the same budget. VS1 (ticket 21, ADR 0006) put a real model into the world's causal chain, but its C7
allocation instance is deliberately the honest baseline — a declared uniform/periodic `WakePolicy`
(`internal/cognition/protocol/wake.go`) whose escalation set is a pure function of `(policy, hour)` and,
in that instance, does not consult the roster at all (VS1 EC-8).

VS2 must add the value-based priority layer of `docs/ARCHITECTURE.md` §4 and `docs/PRINCIPLES.md` P7:
`priority = expected_civilizational_value / expected_cost`, actor-raised escalation requests, and
protected exploration. The design question is where this layer lives and what invariants it must keep,
because the substrate's value proposition is exact replay and auditable causation, and because VS1
already froze a `WakePolicy` object that the world consumes through a nil-tolerant interface.

Three properties bind the answer:

1. **The observation boundary is the model's world (C5).** Any input to the priority layer must be
   something the actor could observe; an allocation policy must not reason from privileged state.
2. **Replay is recorded-event determinism (ADR 0006).** The escalation set a run made must be
   re-derivable during replay, or every allocation decision has to be recorded and trusted separately.
3. **No world RNG (extends VS1 EC-8).** The frozen substrate's random positions must be untouched, so
   the allocation layer cannot advance the world random stream.

## Decision

The value-based allocation layer is a **declared property of VS1's existing `WakePolicy` object**, not a
new scheduler subsystem, and it obeys three invariants:

1. **Extension, not parallel infrastructure.** `protocol.WakePolicy` gains one additive, `omitempty`
   `Priority *PriorityPolicy` field and a roster/budget-aware `Select` method that generalizes the
   existing `Escalates(hour)`. A nil `Priority` reproduces the VS1 path byte-for-byte, so every recorded
   VS1 policy identity, cognitive manifest, and run stays valid. The world consumes the layer through the
   same nil-tolerant seam pattern VS1 used for the Role A `ClaimSeam`: `NaturalCognition` gains an
   optional `protocol.EscalationSelector`, and with it absent the world takes the VS1 path.

2. **A deterministic function of the declared policy, the arm's own ordered candidate stream, and the
   budget state, over the actor's own observation.** The value projection — expected value, expected
   cost, and their integer fixed-point ratio — is a pure function over the candidate feature vector,
   which is derived from the actor's own `strategy.NaturalObservation` (plus one scheduler-level
   world-level aggregate row, a declared versioned cell statistic that carries no other actor's private
   state, matching the VS1 Role B `TrajectoryEvidence` discipline). Escalation requests are a declared
   pure predicate over that observation. Selection over the candidate stream is a deterministic
   **online accumulator** operating on **each arm's own realized stream**: `N` is a declared call
   ceiling (fixed from the model-free A0 (no-AI) projection before any run, never derived from a
   model-bearing arm's realized count), every budgeted arm selects online and stops once it has spent
   `N`, P is period-gated, and U and A spend at most the declared per-hour quota `q(h)`, so no arm
   needs the run-total candidate count (which is endogenous and arm-dependent) and there is no model-free pre-pass, no reservoir, and
   no exact uniform size-`N` subset; the accumulator's recorded state (spent-call count, PRNG position)
   makes the selection and its replay exact. The selector imports no `internal/world`; the world imports
   no client. The projection is a versioned, integer
   fixed-point numerical projection (P6, ADR 0003) — never prose, never a float, never a single scalar
   objective. The projection's cost type is protocol-local (`protocol.CallPrice`), because
   `internal/cognition` imports `protocol` and a `cognition.PriceTable` parameter would cycle.

3. **No world RNG; a declared policy PRNG instead.** The escalation set is a deterministic function of
   the declared policy identity, the arm's own ordered candidate stream, and the budget state, plus the
   policy's own declared PRNG stream, seeded from run identity and advanced in a declared deterministic
   order by the online accumulator. The uniform-random arm and protected exploration draw from that
   stream, not the world stream, and its seed and consumed position are recorded. A `full` reference
   (`Priority == nil`) draws nothing.

Because the selection is reproducible from the deterministic candidate stream, the recorded policy
identity, and the recorded accumulator state, replay re-derives the escalation set with zero model calls
and verifies it against the recorded allocation records. At master `46b1b2b` the checkpoint manifest's
cognition block records the policy fingerprint but restore never compares a supplied policy against it,
and the block wire carries only `Name`/`PeriodHours`/`PhaseHours`; VS2 adds the additive wire extension,
the full-policy fingerprint recomputation, and the restore-time refusal (L2 lane).

## Alternatives considered

- **Fix `|U|` with a model-free pre-pass (or reservoir sampling) so every arm realizes an exact
  uniform size-`N` subset of a fixed candidate universe.** Rejected. Equal budget on the call axis is
  the experiment's foundation ("at the same budget"), but the candidate stream is endogenous and
  arm-dependent — reasoning changes the trajectory — so its length is unknown at start and a model-free
  count cannot fix an exact subset online. The design instead declares `N` as a ceiling, has each arm
  select **online** over its own stream and stop at `N` (a declared per-hour quota `q(h)` for U and A,
  period gating for P), and reports the realized counts; the divergence is measured over the same full
  declared horizon for every arm. This keeps the same-budget comparison honest without asserting a
  stream length the model arm does not produce and without a matched-prefix evaluation.
- **A free-standing scheduler beside the wake policy that reads world state directly.** Rejected. It
  violates the observation boundary (C5), creates a second allocation subsystem beside VS1's object,
  and makes replay depend on world state that only the live run held. The world already consumes the
  wake policy; extending it is the smaller, safer change.
- **Draw allocation randomness from the world random stream.** Rejected. It would shift the frozen
  substrate's RNG positions and break VS1 EC-8 and the byte-identity of every recorded no-priority run.
  The policy carries its own declared stream instead.
- **Emit a scalar "value of reasoning" score and schedule on it.** Rejected (ADR 0003, ADR 0006): the
  metric is a vector; a scalar objective would encode hidden values and invite optimization artifacts.
  Priority is an integer fixed-point ratio of two declared projections, and the fidelity comparison is
  vector dominance, never a scalar winner.
- **Give the priority layer privileged state (undeclared aggregates, other actors' private state).**
  Rejected. It violates C5 and P7, and it makes the project's differentiator — reasoning *under* a
  boundary — untestable. The feature contract admits only the actor's own observation plus one declared
  world-level aggregate row (`prior_reliability`, a versioned cell statistic carrying no other actor's
  private state), and it is tested by the import-graph and sentinel checks.
- **Use real floating-point arithmetic for the expectation.** Rejected. Float arithmetic is not
  reproducible across platforms and would make the "deterministic function" claim false in practice.
  Priority and the exploration fraction are integer fixed-point.

## Consequences

- The allocation layer adds evidence and a new artifact class (allocation records) rather than changing
  the world; with `Priority == nil` every VS1 code path and recorded run is byte-identical, so VS1's
  delivered evidence stays valid.
- Equal budget becomes definable on the deterministic call axis: the model is unpriced (ADR 0006
  Addendum), so cost cannot equalize arms; calls can. The cost-equalized comparison becomes a declared
  operator gate rather than a fabricated price.
- The selector is a deterministic **online** accumulator over each arm's own stream: `N` is a declared
  ceiling, each arm selects online and stops at `N` (reporting a realized count below `N` when its
  stream ends early), and the accumulator's recorded state (spent-call count, PRNG position) makes the
  selection and its replay well defined. Divergence is measured over the same full declared horizon for
  every arm, with realized counts disclosed; there is no matched-prefix comparison and no asserted
  fixed `|U|`. The value vector is computed over the full declared `H`, and an arm whose hourly
  projection is shorter than `H` is flagged and refused rather than silently truncated to the
  scaffold's `min(b, r)` prefix.
- The value model is a versioned hypothesis that must be calibrated: predicted priority is compared to
  realized downstream effect, and the calibration error is reported (P6). An uninformative model is a
  measurable outcome, not a hidden default.
- The priority layer is the first user of a policy PRNG that is not the world stream; run identity grows
  a declared policy seed and the allocation records carry its consumed position.
- The extension is additive and nil-tolerant: `internal/world` still imports no client, and the seam
  remains an interface typed with `sim`/`strategy`/`protocol` values.

## Relationship to ADR 0003, ADR 0004, and ADR 0006

ADR 0003 (numerical projections) is honoured by making the value model a versioned integer projection
and the fidelity comparison a vector, never a scalar. ADR 0004 (branch and beam evaluation) is honoured
in its minimal form: the equal-budget arms are evaluated under matched conditions on the same world,
seeds, mechanism, base policy, and horizon (the candidate universe `U` is derived per run and is not
asserted identical), and no scalar winner is selected. ADR 0006 (the cognitive layer with recorded
model calls) supplies the substrate: the priority layer schedules *which* decisions enter the
recorded-call path that ADR 0006 defined, and it records its selections on the same hash-chained ledger. It does not weaken
ADR 0006's discipline — calls stay recorded, replay never calls a model, and the observation boundary is
the model's world.
