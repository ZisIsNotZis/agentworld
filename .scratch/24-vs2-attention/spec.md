# VS2 — Attention (the central claim) (design, ticket 24)

Status: **implemented** across lanes L1–L4c and L2b on master `48b4882`; the hermetic matrix reports
VS2-EC-1..9 present, with VS2-EC-3 a clean negative. The central claim is **not accepted**: the
real-model arm is blocked on the operator spend gate (§11.3 item 1) and the value-model weights and
feature derivations are a reversible operator-confirmable default (§11.3 item 5). This file is the
single source of truth for VS2 and is edited in place when the design changes; it is never a running
log. The dated implementation record, the gate table and the raw evidence live in
`.scratch/33-vs2-l5/spec.md`.

Implementation revision binding (master `48b4882`): L1 `8ecf4d0`, L2 `53b094a`, L3 `c72ba9d`, L4a
`57770d2` (+ nit `e6d5466`), L4b `b97ac5b`, L4c `4a0ad28`, L2b `48b4882`.

Authority: the product decisions fixed by the operator in `docs/VERTICAL-SLICES.md` (VS2 section, the
concept inventory, the cross-cutting invariants, the product decisions, and the frozen fidelity metric
definition), the founding documents (`docs/VISION.md`, `docs/PRINCIPLES.md` P7, `docs/ARCHITECTURE.md`
§4), and `docs/adr/0003`, `docs/adr/0004`, `docs/adr/0006`. Where this file and another document
disagree, this file is the VS2 contract and the disagreement is listed in Appendix D.

Pattern: this contract is written in the shape and rigor of the delivered VS1 contract
`.scratch/21-vs1-mind-in-the-loop/spec.md` (frozen slice contract, concept-coverage table with real
symbols, declared reductions, falsifiable exit criteria, lane board). VS2 **reuses** VS1's mechanisms;
it does not invent parallel infrastructure.

Companion decision record: `docs/adr/0007-value-based-cognitive-allocation.md`.

## 1. Issue

`docs/VERTICAL-SLICES.md` makes **M2 = VS2 "Attention (the central claim)"**: the first version in which
the project tests its own reason to exist — that *selective* spending of expensive reasoning preserves
civilization-level fidelity better than *uniform* or *periodic* invocation **at the same budget**.

The central claim has never been instantiated. The adversarial audit
`.scratch/audit-vision-fidelity.md` established that at foundation the repository contained no model
call path, no cognitive-value scheduler, no allocation-policy comparison, and that `TokenCost` was
hard-coded to zero in every reporter. VS1 (ticket 21) closed the model-call gap: it put a real model
into the world's causal chain under Role A (an actor's mind at the declared claim seam) and Role B
(in-run behaviour-logic modification as versioned data), with recorded calls, exact replay, cost
accounting, and a fail-loud cognitive ceiling. VS1's C7 instance is deliberately the **honest
baseline** — a uniform/periodic `WakePolicy` — which the roadmap states the central claim must beat.

### 1.1 What VS1 already delivers (verified at master `46b1b2b`)

- **C7 present.** A declared wake-policy object: `protocol.WakePolicy{Name, PeriodHours, PhaseHours}`
  in `internal/cognition/protocol/wake.go`, with `VS1WakePolicy()` (periodic 6 h) and
  `FullWakePolicy()` (period 1 = every hour), consumed by the world through
  `internal/world/natural_cognition.go` (`NaturalCognition.WakePolicy`, `naturalSeamEscalates`,
  `naturalSeamOverride`) and wired by `cmd/vs1/config.go` (`wakeFor`) and `cmd/vs1/harness.go`
  (`wireStack`). The escalation set is a pure function of `(policy, hour, roster)` and, in the VS1
  instance, the roster is not consulted at all (uniform over actors). It consumes no world random
  stream (VS1 EC-8).
- **C8 present.** A hash-chained intervention ledger: `internal/cognition/intervention.go`
  (`AppendRoleAIntervention`, `RoleAInterventionInput`, `ReadInterventionRecords`), the record and kinds
  in `internal/cognition/codec.go` (`InterventionRecord`, `InterventionKind`, `InterventionRoleA`,
  `interventionKinds`), and `internal/cognition/ledger.go` (`Ledger.AppendIntervention`,
  `VerifyLedgerData`).
- **C11 present.** A declared, instrumented, fail-loud cognitive budget:
  `internal/cognition/budget.go` (`CognitiveBudget`, `CognitiveAccount`, `CognitiveCeiling`,
  `CognitiveRow`, `CognitiveCeilingHit`, `PriceTable`) and `internal/cognition/client.go` (`Usage`), with
  the four ceiling rows
  `cognitive-calls` / `cognitive-prompt-tokens` / `cognitive-completion-tokens` / `cognitive-cost`; the
  arm ceilings are declared in `cmd/vs1/config.go` (`budgetFor`).
- **C13 present.** Paired arms `ArmA0/A1/A1i/A2/A3` (`cmd/vs1/config.go`, `declaredArms`), the frozen
  eight-indicator divergence-vector scaffold (`cmd/vs1/report.go`: `fidelityRow`, `fidelityIndicator`,
  `buildFidelity`, `divergenceVector`, `summarize`, `decisionErrorIndicator`) and the frozen
  exit-criteria bands (`cmd/vs1/bands.go`: `evaluateBands`, `ec1`..`ec5`, `ec7`..`ec11`; EC-6 is
  `privacyBand` in `cmd/vs1/privacy.go`, not an `ec6`).
- **VS1 evidence.** Hermetic evidence PASS on all eleven frozen VS1 criteria (EC-1..EC-11) at revision
  `vs1-hermetic-fake/2`; the full `make check` passed at the gated revision. **Its declared reductions
  are carried into this contract**, because VS2 builds on the same surface
  (`.scratch/21-vs1-mind-in-the-loop/spec.md` §14.1; restated in §12 below).

### 1.2 What VS2 adds

- A **value-based priority layer**: selection of *which* decisions get reasoning, by a declared
  expected-civilizational-value / expected-cost priority (`docs/ARCHITECTURE.md` §4), not by a time
  period.
- **Escalation requests raised by an actor from its own observation only** (C5): the frozen claim path
  evaluates a declared predicate over the actor's own `strategy.NaturalObservation` and raises a typed
  request that feeds the priority layer. This is the `EscalationRequest` concept the code has never had
  (`docs/ARCHITECTURE.md` §3; `.scratch/21-vs1-mind-in-the-loop/spec.md` Appendix D.7).
- **Protected exploration**: a declared fraction of the budget samples ordinary (non-high-priority)
  actors so low-probability innovation is not systematically excluded (P7, `docs/ARCHITECTURE.md` §4).
- **Equal-budget arms over the same world and seeds**: a full reference (model on every decision),
  periodic, uniform-random, and adaptive-priority, each on the identical world, seed, mechanism, base
  policy, and horizon under the same declared call ceiling (the world/seed/mechanism/base-policy/horizon
  identity and the call ceiling are matched; the candidate stream is derived per arm and is explicitly
  not asserted identical; §5.3).
- The **frozen fidelity metric** instantiated as VERTICAL-SLICES VS2 defines it (the per-indicator
  divergence sequence from the high-budget reference, never a single scalar), used as a **gating**
  criterion rather than the VS1 scaffold.

### 1.3 Honest current status (stated before the design, so it is not discovered later)

1. **VS2 cannot be accepted without the real-model arm.** VS1's hermetic evidence proves the *mechanism*
   is load-bearing (the hermetic `FakeClient` content changes outcomes) but not that a real model is;
   no real model call has been made. `docs/VALIDATION.md` and the VS1 spec §16 say so verbatim, and the
   M1 row in `docs/VERTICAL-SLICES.md` carries "the real-model arm blocked on spend authorisation." VS2
   inherits that blocker: allocation value is a claim about *real* reasoning, so a hermetic-only VS2 can
   at most prove the *allocation machinery* is load-bearing, never the central claim.
2. **The application model has no price entry.** The gateway advertises `github_copilot/gpt-6-luna`
   with **no** input/output cost fields; `docs/adr/0006` (Addendum, ticket 22) records that cost is the
   typed *unpriced* state (`UnpricedPrice`, `price_version: "unpriced"`, `cost_micros: 0`). **Because an
   unpriced call charges nothing, the cost ceiling cannot bind for it**, so *cost is not an available
   budget axis* for VS2 (§5). The cost axis becomes usable only once a price is declared or the cost
   ceiling is explicitly waived for an accepted run.
3. **VS1's C7 baseline is uniform/periodic, and no measured value term exists anywhere in the code.**
   The priority formula of `docs/ARCHITECTURE.md` §4 exists only as design text. VS2 is the first
   attempt to instantiate it, and its result is unknown in advance.
4. **A clean negative is a valid, decisive result.** If no allocation policy differentiates at equal
   budget, that is "equally decisive and reportable" per `docs/VERTICAL-SLICES.md`, because it would
   show the decision space does not yet reward reasoning. This contract treats the negative as a
   first-class outcome (§8.4, VS2-EC-9).

## 2. Scope, non-goals, and non-goals' owning slices

### 2.1 In scope

- The **value-based priority layer**: the expected-civilizational-value / expected-cost object, its
  versioned numerical projection (P6/ADR 0003), and its feature set (impact, uncertainty, decision
  sensitivity, novelty, propagation potential, structural position, urgency, prior reliability, and the
  explicit escalation request), per `docs/ARCHITECTURE.md` §4.
- **Escalation requests from the actor's own observation** (C5), raised inside the existing claim
  evaluation, never from privileged state.
- **Protected exploration** as a declared budget channel.
- **Equal-budget arms** R (reference, model on every decision), P (periodic), U (uniform-random),
  A (adaptive-priority), plus the unchanged A0 no-AI control, over the declared world and seeds.
- The **frozen fidelity metric** instantiated and used as a gate, with the vector-dominance comparison.
- **Falsifiable exit criteria** (§7), the experiment matrix (§8), the proof method (§9), the lane board
  (§10), and risks/stop conditions/open questions (§11).

### 2.2 Non-goals (explicitly deferred, with the owning slice)

- **Sustained evolution** (strategy populations, sandboxed counterfactual testing at scale, staged
  rollout, selection over generations of strategies): **VS3**. VS2 keeps VS1's single-round Role B
  lifecycle unchanged and treats it as a source of mid-run strategy updates only for VS2-EC-5.
- **Scale and heterogeneity, space, mobility, weather, uncertainty, conflict, per-actor parameter
  jitter**: **VS4**. VS2 does not widen the world; it runs on the same measured natural-limits substrate
  (tickets 15–20) at the same fixture family (f=10–12, `-r 8`, mechanism v6, `natural/7`).
- **Institutions, trade networks, culture, knowledge diffusion**: **VS5**. VS2 leaves the ticket-20
  wage contract as the unchanged proto-institution (`natural/9` is not the VS2 base policy).
- **Historical calibration, conditional branches, sensitivity to alternative projections**: **VS6**.
  VS2 stays model-conditional and makes no historical claim.
- **Batched management** (P8, one call serving several actors): deferred (as in VS1); VS2 keeps one call
  per escalated decision so the observation boundary is trivially isolated and the per-decision unit of
  value is well defined.
- **Model-emitted code, arbitrary strategy structure, or model-written rules**: permanently out of scope
  (ADR 0006, ADR 0002).
- **A real-world policy claim**: never in scope (P14).

## 3. Concept-coverage delta (C7/C8/C11/C13), with evidence at master `46b1b2b`

VS2 deepens four rows from "present" to "tested." "Deepened" means the row now has a falsifiable
experiment and a gated criterion, backed by the named symbols. Later slices raise other rows.

| # | Concept | VS1 (present) — evidence at `46b1b2b` | VS2 adds (tested) |
|---|---------|----------------------------------------|-------------------|
| C7 | Cognitive allocation | `internal/cognition/protocol/wake.go`: `WakePolicy`, `VS1WakePolicy`, `FullWakePolicy`, `Escalates`, `Validate`, `Identity`, `Fingerprint`; `internal/cognition/wake.go` aliases; consumed by `internal/world/natural_cognition.go` (`NaturalCognition`, `naturalSeamEscalates`, `naturalSeamOverride`); wired by `cmd/vs1/config.go` (`wakeFor`) / `cmd/vs1/harness.go` (`wireStack`) | A declared value-based priority object (`PriorityPolicy`, `WakePolicy.Priority`) whose arm-local online selector picks the escalation set over each arm's own candidate stream by expected value / expected cost; actor-raised escalation requests; protected exploration. Tested by the R/P/U/A equal-budget arms (§7 VS2-EC-3). |
| C8 | Intervention ledger | `internal/cognition/intervention.go`: `AppendRoleAIntervention`, `RoleAInterventionInput`, `ReadInterventionRecords`; `internal/cognition/codec.go`: `InterventionRecord`, `InterventionRoleA`, `interventionKinds`; `internal/cognition/ledger.go`: `AppendIntervention` | A new additive `allocation` intervention kind carrying, per escalation hour, the candidate roster size, the per-candidate priority scores, the selected set, the protected-exploration set, the budget state, and the policy-PRNG position — so an allocation decision is auditable like every other intervention. Tested by VS2-EC-9. |
| C11 | Budget and cost accounting | `internal/cognition/budget.go`: `CognitiveBudget`, `CognitiveAccount`, `CognitiveCeiling`, `CognitiveRow`, `CognitiveCeilingHit`, `PriceTable`; ceilings `cmd/vs1/config.go`: `budgetFor` | Equal budget defined on the **call axis** as a declared ceiling `N` per budgeted arm, with every arm's realized call count reported and the divergence measured over the same full declared horizon, enforced fail-loud, with the unpriced-model interaction named. Tested by VS2-EC-2. |
| C13 | Metrics and paired experiments | `cmd/vs1/report.go`: `fidelityRow`, `fidelityIndicator`, `buildFidelity`, `divergenceVector`, `decisionErrorIndicator`; arms `cmd/vs1/config.go`: `ArmA0`..`ArmA3`, `declaredArms`; bands `cmd/vs1/bands.go` | Equal-budget arms R/P/U/A over the same world/seeds, with the frozen vector used as a **gate** by vector dominance, multi-seed, and no single scalar. Tested by VS2-EC-3/EC-4. |

The coverage invariant holds: no C-row becomes empty; C1–C6, C9, C10, C12, C14–C16 keep VS1's present
form (C9/C10 the single-round Role B lifecycle, C16 model-conditional labelling only).

## 4. The priority / allocation object

The design constraint (task and `docs/ARCHITECTURE.md` §4) is that this be an **extension of VS1's
declared `WakePolicy` object**, not a parallel scheduler subsystem, and that the escalation set remain a
**deterministic, world-RNG-free function of the declared policy identity, the arm's own ordered
candidate stream, and the budget state** (extending VS1 EC-8). The value projection (`ExpectedValue` /
`ExpectedCost` / `Priority`) is a pure integer fixed-point function; selection over the arm's stream is
a **deterministic online accumulator** whose spent-call count and policy-PRNG position are recorded run
state (§4.6). Each budgeted arm selects online over its **own realized candidate stream** and simply
stops once it has spent the declared ceiling `N`; it never exceeds `N`, and it needs no run-total
candidate count, no model-free pre-pass, and no reservoir. P is period-gated; U and A spend at most the
declared per-hour quota `q(h)` (§5.1).

### 4.1 Extension surface over VS1's `WakePolicy`

VS1's object is `internal/cognition/protocol/wake.go: WakePolicy{Name, PeriodHours, PhaseHours}` with a
canonical wire identity (`Identity`, `Fingerprint`) over its struct fields, consumed by the world's
`NaturalCognition`. VS2 extends it **additively and byte-compatibly**:

1. `WakePolicy` gains one optional pointer field `Priority *PriorityPolicy` carrying the json tag
   `priority,omitempty` (Appendix A.1). When nil, the struct's canonical bytes are **unchanged**, so
   `VS1WakePolicy().Identity()`, `VS1WakePolicy().Fingerprint()`, and every recorded VS1 cognitive
   manifest stay byte-identical (VS2-EC-1). `VS1WakePolicy()` and `FullWakePolicy()` are untouched.
2. A new `PriorityPolicy` struct holds the declared allocation instance: the strategy
   (`PriorityStrategy`), the integer fixed-point exploration fraction, the declared policy-PRNG seed,
   the value-model spec, and the declared call ceiling `N` (Appendix A.1).
3. `WakePolicy` keeps a pure selection method for the degenerate path (nil `Priority`), which
   generalizes `Escalates(hour)` to the roster:

```go
// In internal/cognition/protocol/wake.go (VS2 extension surface).
// Select is the degenerate selector used when Priority is nil: it is a pure
// function of (policy, hour, roster, budget state), consumes no world random
// stream, and selects every candidate (all claim-wake actors in a selected
// hour). It serves the reference R (FullWakePolicy) and the VS1 path. A budgeted
// arm carries a non-nil Priority and is served by the deterministic arm-local
// online accumulator of protocol.EscalationSelector (item 4), not by this method.
func (p WakePolicy) Select(hour int, roster []EscalationCandidate, budget BudgetState) (Selection, error)
```

4. The priority-bearing selection is a **deterministic online accumulator** declared by
   `protocol.EscalationSelector`. It is seeded from the declared call ceiling `N` and the policy PRNG
   seed; its spent-call count and PRNG position are recorded run state. It is **online**: it consumes
   each hour's roster as it arrives, spends at most the declared per-hour quota `q(h)` for U and A (or
   is period-gated for P), and stops once it has spent `N`. It never needs the run-total candidate
   count, never exceeds `N`, and holds no reservoir or model-free pre-pass. The pure `WakePolicy.Select`
   above is its degenerate nil-`Priority` case.

```go
// In internal/cognition/protocol/priority.go (new).
type EscalationSelector interface {
    Select(hour int, roster []EscalationCandidate, budget BudgetState) (Selection, error)
}
```

5. `NaturalCognition` gains an optional selector seam but keeps the nil-tolerant VS1 path:

```go
// In internal/world/natural_cognition.go (VS2 extension surface).
type NaturalCognition struct {
    Seam       ClaimSeam
    WakePolicy protocol.WakePolicy
    // Selector is optional. When non-nil the runner-provided online accumulator
    // owns the policy PRNG, the spent-call count, and the budget state; when nil
    // the world uses the pure WakePolicy.Select (or the VS1 hour gate when
    // Priority is nil). Either way the escalation set is reproducible and
    // consumes no world random stream.
    Selector protocol.EscalationSelector // nil => protocol/WakePolicy path
}
```

   A half-wired priority policy is a **construction-time refusal**: `NaturalCognition.validate` returns
   the typed `ErrNaturalRunner` error for `WakePolicy.Priority != nil && Selector == nil`, mirroring the
   existing half-wired seam refusal at `internal/world/natural_cognition.go:46-60` (a nil `Seam` or an
   invalid `WakePolicy`). A declared priority arm can therefore never silently degrade to the VS1 hour
   gate.

   The world builds `roster []EscalationCandidate` from the claim-wake set (the exact set VS1's
   `allocate` enumerates) and calls `Select` once per candidate hour. `Selection` carries the selected
   actor set, the protected-exploration set, the recorded scores, the spent-call count, the per-hour
   shortfall, and the PRNG position (Appendix A.1). Because the accumulator's state is a function of the
   deterministic candidate stream and the declared policy identity, replay re-derives the identical
   sequence.

   `naturalSeamEscalates(hour)` and the claim loop of `Natural.allocate`
   (`internal/world/natural.go`, the loop over `f.sched.Snapshot().Wakes` for
   `WakeNeedThreshold`/`WakeBirth`) are generalized from an hour-only predicate to a per-roster
   selection. With `Selector == nil` and `Priority == nil`, the world takes the VS1 path exactly.

The alternative — a free-standing scheduler that reads world state directly — is **rejected** (ADR 0007):
it would break the observation boundary (C5), make replay non-derivable from reproduced state, and
create a second allocation subsystem beside VS1's object.

### 4.2 The candidate universe and the feature vector

**Candidate universe `U(arm)`: the arm's own realized candidate stream.** The frozen claim topology
wakes every living roster actor at the claim instant — `WakeNeedThreshold`, or `WakeBirth` for a
newborn's reserved first claim — and that is exactly the set VS1's `Natural.allocate` enumerates and
`FullWakePolicy` escalates. VS2 does not fix this set in advance: reasoning changes the trajectory, so
each arm's stream is endogenous and arm-specific. Only realized counts are reported.

**No model-free pre-pass.** Selection is a declared online rule over each arm's own stream (§5.1): each
arm consumes its own candidates hour by hour and stops once it has spent the declared ceiling `N`. No
run-total candidate count is computed before the run and none is part of the run identity; there is no
model-free pre-pass and no reservoir, and no arm needs to know its stream's length in advance. The only
model-free count in the contract is the A0 control trajectory's claim-wake count, used solely to
project the expected call scale and choose declared constants (§5.1) and never a normative input or
asserted equal to a model-bearing arm's count.

**Feature vector.** Each candidate carries a feature vector. The features derived from the actor and
the actor-raised escalation request are **observation-only** — they read that actor's own
`strategy.NaturalObservation` (C5), consumed through the existing boundary
(`internal/cognition/observation.go` encoder discipline). One feature, `prior_reliability`, is a
scheduler-level input: a declared world-level aggregate row (a versioned cell statistic) of the kind
the VS1 Role B `TrajectoryEvidence` shape already publishes
(`internal/cognition/protocol/protocol.go`). It carries no other actor's private state, and the
escalation-request predicate itself (EC-6) never reads it. Features (Appendix C.1): `energy`, `hunger`,
`capital`, `granary_stock`, `last_stored_meal_hour`, `stocked_home_slots`, `last_denial`, and the
derived `decision_sensitivity`, `urgency`, `impact`, `propagation_risk`, `prior_reliability`, and the
`escalation_requested` bit of §4.3.

**Feature-set version and declared derivations (Appendix C.1).** The feature set is versioned
`vs2-features/1`; the version rides `ValueModelSpec.FeatureSet`, so it is bound into the policy
identity/fingerprint (and the checkpoint cognition block, VS2-EC-5) and named by the report. The
derived rows are **declared exact integer fixed-point functions of the actor's own observation**:
`decision_sensitivity` is the summed proximity of the actor's own rows to the frozen claim/birth
decision boundaries; `urgency` is the fraction of the combined survival reserve already consumed;
`impact` is the fraction of the frozen birth/consent margins met; `propagation_risk` is the stocked
fraction of the frozen home-slot resource; `last_denial` is the declared string encoding.
`prior_reliability` is the one scheduler-level row: the declared versioned world-level aggregate (a
neutral prior cell statistic in this instance), the same for every candidate of a cell and never read
by the escalation predicate (EC-6). The derivations and the §4.3 predicate body are a declared,
versioned, reversible default pending operator confirmation (Appendix C.1/C.2, §11).

### 4.3 Escalation requests from the actor's own observation (C5)

At the existing claim instant (`h·3600s + 1µs`) the frozen claim evaluation already produces the
actor's `strategy.NaturalObservation`. VS2 adds one declared, pure predicate over that observation:

```go
// In internal/cognition/protocol/priority.go (new).
// EscalationRequest is a pure predicate over the actor's own observation. It is
// evaluated by the world at the claim instant and carried into the candidate's
// feature vector. It can read nothing else.
func EscalationRequest(p PolicyPredicate, obs strategy.NaturalObservation) (bool, EscalationReason)
```

The declared instance is `vs2-escalation/1`: the actor requests escalation when `urgency ≥ 500` and
`decision_sensitivity ≥ 500` (the exact body and the fixed-point derivations are in Appendix C.1).
The predicate evaluates only the observation-only rows returned by `ObservationFeatures` and **fails
closed** for an empty predicate or for any row it may not read (`prior_reliability`, an unknown name),
so it can never read the scheduler-level aggregate (EC-6).

It is not a new phase instant (VS1 adds none), not a new kernel rule, and not model-emitted code. It is
recorded with its typed reason on the candidate and, when it leads to an allocation decision, on the
`allocation` intervention record. This is the "explicit escalation" wake cause of P7 and the
`EscalationRequest` of `docs/ARCHITECTURE.md` §3 that VS1 did not have (VS1 spec Appendix D.7).

### 4.4 Expected value / expected cost

The priority is a **versioned numerical projection** (P6, ADR 0003) — never prose, never terminal from a
model:

```go
// Appendix A.2 / C.2.
type ValueModelSpec struct {
    Version        string          // run identity; the versioned hypothesis
    FeatureSet     string          // declared candidate feature-set version (Appendix C.1); part of run identity
    Weights        []FeatureWeight // integer fixed-point weights, declared
    CostProxy      CostProxy       // how a call's cost is estimated (§5)
}

// ExpectedValue and ExpectedCost are pure integer fixed-point functions of the
// feature vector. price is the protocol-local CallPrice (Appendix A.2): the
// package cannot take cognition.PriceTable, which imports this package.
func ExpectedValue(m ValueModelSpec, f ValueFeatures) int64
func ExpectedCost(m ValueModelSpec, f ValueFeatures, price CallPrice) int64

// Priority is the declared ratio, integer fixed-point (no floats: float
// arithmetic is not reproducible across platforms).
func Priority(m ValueModelSpec, f ValueFeatures, price CallPrice) int64
```

`CallPrice` is a protocol-local cost type (Appendix A.2). It cannot be
`internal/cognition`'s `PriceTable`, because `internal/cognition` imports `protocol` and the import
would cycle; the runner adapts a declared `PriceTable` to `protocol.CallPrice` at the boundary when a
price exists.

- **Expected civilizational value** is a monotone function of decision sensitivity (direction and
  distance to the frozen thresholds), urgency (energy/hunger relative to survival), impact (capital and
  birth-gate proximity), propagation potential (home-slot contention visible in the observation), the
  escalation-request bit, and the prior reliability of that feature region (a declared, versioned
  hypothesis, calibrated over cells).
- **Expected cost** is the marginal cost of one call. While the model is **unpriced** the cost is a
  declared *proxy* (`CostProxy`): a constant per call plus a projected token term from the frozen
  sampling instance; it is reported as a proxy, never as spend. When a `PriceTable` entry exists the
  proxy is replaced by `PriceTable.CostMicros` over projected usage (`internal/cognition/budget.go`).
- **Priority = expected value / expected cost**, integer fixed-point.
- The value model **exposes feature values, uncertainty, selected action, cost, and realized downstream
  value** (P6, ARCHITECTURE §4): the allocated calls' realized effect on the declared indicators is
  measured and the **calibration error of predicted priority vs realized effect** is reported
  (`docs/VALIDATION.md`, cognitive-allocation experiments). VS2-EC-8.

### 4.5 Protected exploration

P7: "a protected exploration budget samples ordinary actors so low-probability innovation is not
systematically excluded." Of the declared ceiling `N`, the selector reserves `ceil(q·N)` calls, with
`q ∈ [0,1)` declared as an integer fixed-point fraction (`ExplorationFraction` scaled by
`ExplorationScale = 1_000_000`, so `ceil(q·N) = (N·ExplorationFraction + ExplorationScale − 1) /
ExplorationScale` in integer arithmetic), spent online on **uniform draws over the candidates the arm
meets** using the **policy's own declared PRNG**, independent of priority and inside the same per-hour
quota `q(h)` (§5.1); the remaining `N − ceil(q·N)` calls are the exploitation quota (ranked by
priority). It is an online rule, not a subset fixed in advance and not a reservoir held over the whole
stream. `q = 0` is the pure-exploitation arm used to isolate the exploration channel (VS2-EC-7).

**A's per-hour interleaving rule (declared, so two implementers build the same A arm).** With
`E = ceil(q·N)` the reserved exploration calls (integer arithmetic as above), define the cumulative
exploration target through hour `h` as `T(h+1) = ceil(E·(h+1)/H)` and hour `h`'s exploration allocation
as `e(h) = T(h+1) − T(h)`. At hour `h`, arm A offers up to its per-hour quota `q(h)` slots over that
hour's candidates in the declared deterministic order (roster sorted by `(actor)`); the first
`min(e(h), slots offered)` slots are **exploration** slots and the remaining slots are **exploitation**
slots. An exploration slot spends on a uniform draw from the hour's candidates not already selected
that hour, using the policy PRNG; an exploitation slot spends on the highest-`Priority` remaining
candidate (ties broken by actor id). The hour stops when its roster or quota is exhausted and the arm
stops at `N`. `T` is a declared fixed schedule independent of any realized stream, so exploration never
exceeds `E` and the rule is deterministic; with `q = 0`, `E = 0`, and every slot is exploitation.

### 4.6 Determinism, the policy PRNG, and no world RNG (extends EC-8)

- The escalation set is a **deterministic function of the declared policy identity, the arm's own
  ordered candidate stream, and the budget state** plus the policy's own declared PRNG stream,
  evaluated by the online accumulator whose state (spent-call count, PRNG position) is recorded run
  state. It **consumes no world random stream**, so the frozen substrate's RNG positions
  are untouched, exactly as VS1 EC-8 requires. The value projection (`Priority`) is a pure function;
  only selection over the stream is stateful.
- **uniform-random** and **protected exploration** draw from a declared PRNG (`PriorityPolicy.RandSeed`,
  part of run identity) advanced in a declared deterministic order (the candidate roster sorted by
  `(actor)` within the hour). The stream is **not** the world stream and is **not** drawn from world
  state; its seed and consumed position are recorded on the `allocation` record, so replay re-derives
  the identical selection.
- The `full` reference (`FullWakePolicy`, period 1, `Priority == nil`) is byte-identical to VS1's A3
  selection: every candidate, no PRNG.
- Because the selection is reproducible from the deterministic candidate stream, the recorded policy
  identity, and the recorded accumulator state, **replay re-derives the escalation set without a model
  call** and verifies it against the recorded `allocation` records (VS2-EC-5).

## 5. Equal-budget accounting

### 5.1 Definition (model-conditional)

The model-conditional setting has three candidate budget axes — **calls**, **tokens**, and **cost** —
and they are not interchangeable:

- **Cost is unusable while the model is unpriced** (§1.3.2): an unpriced call records
  `cost_micros: 0`, `price_version: "unpriced"`, so no cost ceiling can bind and cost cannot equalize
  arms. A cost-equalized comparison is a **declared operator gate** (§11), run only after a price is
  declared or the cost ceiling is waived.
- **Tokens are endogenous to the model**: for a fixed prompt template and sampling instance, the
  completion length is the model's, so per-call tokens are not equal across arms a priori. Tokens are
  therefore a **reported secondary axis**, bounded by a declared per-call token ceiling, never the
  equalization axis.
- **Calls are the deterministic, arms-controlled axis.** Every arm escalates an integer number of
  decisions through the *same* recorded-call path (`internal/cognition/rolea.go`), so calls are exactly
  controllable and replayable.

**Equal budget is defined on the call axis as a declared CEILING `N` per budgeted arm (P, U, A).** All
budgeted arms run the same declared horizon `H = 168 h`; each makes selection decisions **online over
its own candidate stream** — the claim-wakes its own trajectory produces, which are endogenous and
arm-specific because reasoning changes the trajectory — and simply **stops once it has spent `N`
calls**. `N` is a **declared ceiling, not a promised count**: each arm realizes exactly `N` when its
own roster is ample enough to fill its declared schedule (U/A: at least `q(h)` candidates at each hour;
P: at least `N` candidates across its selected hours) and reports a realized count below `N` otherwise;
no arm may exceed `N`.

- **Policy rules (all online, all ending at the ceiling `N`).**
  - **P (periodic):** escalate candidates only at hours `h` satisfying the declared period/phase
    (`h ≡ PhaseHours (mod PeriodHours)`); within those hours select candidates in a declared
    deterministic order (roster sorted by `(actor)`); stop at `N`. P has no per-hour quota, so its
    `Selection` and `AllocationRecord` set `Quota = remaining budget (N − CallsSpent)` and
    `Shortfall = 0` (there is no `q(h)` for it to fall short of).
  - **U (uniform-random):** at each hour spend at most the declared per-hour quota `q(h)` on candidates
    chosen by the policy PRNG; stop at `N`.
  - **A (adaptive-priority):** rank candidates online by the declared priority `Priority` (§4.4); at
    each hour spend at most `q(h)`; stop at `N`. Of each hour's `≤ q(h)` slots, the first
    `min(e(h), slots offered)` are exploration slots and the rest are priority-ranked exploitation
    slots (`e(h) = T(h+1) − T(h)`, `T(t) = ceil(E·t/H)`, `E = ceil(q·N)`), per the declared §4.5
    interleaving rule.
- **The per-hour quota `q(h)`** is a **declared fixed schedule** that depends on no arm's realized
  stream. Illustratively `q(h) = floor(N/H) + (1 if h < (N mod H) else 0)`, so `Σ_{h<H} q(h) = N` (at
  `N = 256`, `H = 168`: `q(h) = 2` for the first 88 hours, `1` for the rest). It is recorded once and
  frozen before any run. If an hour's roster is smaller than `q(h)`, the selector records the
  **shortfall** for that hour and spends what the roster offers; a run whose cumulative roster never
  reaches `N` is an **under-budget cell**, reported and interpreted, not a mis-constructed arm (§5.3).
- **R (reference)** is the full policy: it escalates every candidate in its own stream under a
  separately declared generous ceiling, and is never a budgeted competitor.

`N` is a **declared experiment parameter** (P15-permitted: "a frozen experimental gate may bound an
experiment"), frozen before any run. The proposed value is `N = 256` calls per cell (≈1/8 of the VS1 A3
projection of ≈1 700–2 000 calls/cell; the VS1 A1 periodic-6 arm realized ≈300/cell). `N` is listed in
§11 for operator confirmation. `N` is fixed from the model-free A0 (no-AI) projection before any run
(§5.1 "Model-free reference count") and is **not derived from any model-bearing arm's realized candidate
count** (those are endogenous, arm-specific, and unknown before the run); the budget is declared, not
fitted to a model arm.

**Model-free reference count.** The only model-free count in the contract is the deterministic A0
(no-AI) control trajectory's claim-wake count, computed from the frozen substrate with no model in the
loop. It is used **only** to project the expected call scale and to choose declared constants — the
ceiling `N`, the periodic arm's declared `PeriodHours` (so that the selected hours supply at least `N`
candidates), and the quota schedule `q(h)`. It is reported as an A0-based projection, is **never** a
normative input, and is **never** asserted to equal any model-bearing arm's realized count: reasoning
changes the trajectory, so every arm's stream is endogenous and only realized counts are reported.

### 5.2 Enforcement (fail-loud, P15)

- Every budgeted arm's fail-loud ceiling is `CognitiveBudget.MaxCalls = N`: any attempt to spend an
  `(N+1)`-th call is the typed `CognitiveCeilingHitError` fail-loud halt
  (`internal/cognition/budget.go`), never silent truncation or padding. A hit is a gate failure for
  every budgeted arm: the declared rule must stop at `N` on its own, so reaching the ceiling by overrun
  means the rule overspent.
- The harness records **each arm's realized call count** from the call ledger and reports it per cell;
  it does **not** assert `realized == N`. A run whose realized count is below `N` (for example
  population extinction before the horizon supplies `N` candidates) is a reported **under-budget
  outcome**, not a mis-constructed arm.
- The reference arm's ceiling is separately declared and generous (`MaxCalls ≥` the projected full
  universe, e.g. 4 096); if its stream exceeds it the reference is **incomplete** and the cell is
  reported, never truncated to a cheaper reference.
- No arm pads a shortfall with duplicate calls or continues past `N` with a re-selection.
- All four cognitive-ceiling rows (`cognitive-calls`, `cognitive-prompt-tokens`,
  `cognitive-completion-tokens`, `cognitive-cost`) are reported per cell.

### 5.3 Fair comparison and the evaluation window

All arms run on the **identical world, seed, mechanism, base policy, and horizon `H`**, and every arm
selects through the same declared interface on **its own realized candidate stream**. The candidate
stream is **not** asserted identical across arms: it is derived per run from that run's own claim wakes
and is therefore policy-dependent (an arm that reasons differently changes the trajectory and hence
later wakes). The match is on declared world/seed/mechanism/base-policy/horizon identity and on the
declared call ceiling `N`, with the realized count reported per arm.

**Evaluation window: the same full horizon `H` for every arm.** Each arm's frozen divergence vector
(§6) is computed over the whole declared horizon `H`, not over a per-arm prefix. There is **no
matched-prefix or matched-realized-count evaluation**: the primary gate is taken at full `H` with every
arm's realized call count disclosed, so the reader sees the realized spend alongside the divergence.
When every budgeted arm realizes `N` (the normal case when the roster is ample relative to `N`), equal
budget is exactly equal spend over `H`. An **under-budget cell** — an arm whose realized count is below
`N` because its stream ended early (e.g. population extinction) — is **reported and interpreted** with
its realized count disclosed; it is not silently equalized by truncating the other arms to a common
prefix. The reference R is the comparative target for the fidelity metric (§6), never ground truth, and
never a budgeted competitor.

**The experiment layer computes over the full `H`; it does not inherit the scaffold's `min` prefix.**
The VS1 scaffold's `divergenceVector` (`cmd/vs1/report.go:317-318`) truncates to
`n := min(len(b.Hours), len(r.Hours))`. With every arm running the declared horizon `H` this is a no-op
(and it is a no-op at every VS1 horizon, so `cmd/vs1`'s migration to `internal/experiment` is
byte-unchanged), but a short arm's projection would silently shorten the comparison and contradict this
section. The extracted `internal/experiment` mechanics (L4, §10) therefore compute the vector over the
**full declared `H`** and **flag and refuse** any arm whose hourly projection is shorter than `H`: the
report records the arm, its actual projection length, and the missing hours, and the cell is reported as
an incomplete/short projection — never silently compared over `min(b, r)` hours.

## 6. The frozen fidelity metric (instantiated for VS2)

The metric is `docs/VERTICAL-SLICES.md`'s VS2 definition, instantiated on the VS1 scaffold
(`cmd/vs1/report.go: divergenceVector`, `summarize`, `decisionErrorIndicator`) and unchanged in
definition:

> Fidelity is measured as *divergence of the budgeted run's civilization-level trajectory from the
> high-budget reference run* (population, births/deaths, wealth distribution, exchange volume, and the
> recorded decision-error rate), not as a single scalar.

The **reference arm** is R (the `full` policy: model on every decision). For a budgeted arm `B` and R at
the same fixture and seed, the metric is the per-indicator divergence **sequence** over hours plus the
per-indicator summaries `Max` and `Mean` (both **lower-is-better** — smaller divergence is closer to the
reference) and `FirstDivergenceHour` (**higher-is-better** — divergence begins later).

**No-divergence encoding (declared once, here).** The reused scaffold's `summarize`
(`cmd/vs1/report.go:349-357`) initialises `FirstDivergenceHour` to `-1` and sets it only at the first
hour whose hourly value is non-zero, so `-1` means "the indicator never diverged over `H`" — the
**best** outcome, not the worst. Higher-is-better is therefore defined on a **normalised** value so the
declared direction holds for every arm and indicator:

```
NormalizedFirstDivergenceHour = (raw == -1) ? H + 1 : raw
```

`H + 1` is strictly greater than any real first-divergence hour `≤ H`, so a never-diverged indicator is
best under the declared direction. The direction-aware dominance comparison (§7 VS2-EC-3/EC-4) applies
this normalisation uniformly and defines **no other** special case for the sentinel: an identical
(better) arm with raw `-1` must not lose to a diverged arm. This is the single definition of the
encoding; the raw `-1` and its normalised value are both reported.

| # | Indicator | Hourly value | Divergence |
|---|---|---|---|
| 1 | population | `alive(h)` | `abs(B−R) / (8P)` |
| 2 | births | cumulative births | absolute difference over the horizon |
| 3 | deaths/starvation | cumulative starvation deaths | as (2) |
| 4 | deaths/age | cumulative age deaths | as (2) |
| 5 | capital histogram | capital histogram over `k ∈ [0, KMax]` | total-variation distance |
| 6 | granary-stock histogram | granary-stock histogram over declared buckets | total-variation distance |
| 7 | exchange volume | cumulative wage settlements | as (2) |
| 8 | decision-error rate | cumulative escalated decisions with a malformed/inadmissible/refused outcome over cumulative escalated decisions | absolute difference of the two ratios |

The scaffold emits these **eight** indicators (`cmd/vs1/report.go: divergenceVector`), with deaths split
by cause into `deaths/starvation` and `deaths/age` (indices 3 and 4); the founding
`docs/VERTICAL-SLICES.md` metric definition groups them as "births/deaths". VS2 gates on all eight.

The vector is the metric; **a weighted sum is explicitly not defined** (ADR 0003). VS2 uses the vector
as a gate by **dominance** (§7 VS2-EC-3/EC-4).

**Honest limitation, stated verbatim** (VERTICAL-SLICES VS2 "Limitation", `docs/VALIDATION.md`): the
high-budget reference is a **comparative proxy, never ground truth**: it is itself a model-conditional
run, so a smaller divergence means "closer to the reference run", not "more faithful to reality". Every
VS2 result is `synthetic/model-conditional`.

## 7. Frozen exit criteria (VS2-EC-1..VS2-EC-9)

Frozen before any run; a miss is an outcome reported verbatim, never tuned. Cells are §8 unless stated.

- **VS2-EC-1 — deterministic allocation, no world RNG (extends EC-8).** The escalation set is a
  **deterministic, world-RNG-free** function of the declared policy identity, the arm's own ordered
  candidate stream, the budget state, and the recorded accumulator state (spent-call count +
  policy-PRNG position); it consumes no world random stream. The value projection (`Priority`) is a pure
  integer fixed-point function. The nil-priority `WakePolicy` canonical identity bytes are unchanged from
  VS1 (`VS1WakePolicy().Identity()` and `.Fingerprint()` byte-identical); the A0 control through the VS2
  harness is byte-identical to VS1's recorded A0 (`.tmp/natural9-n7-*`, per the VS1 EC-1 substrate).
  *Proof method:* RNG-state/position identity test on the no-priority path; replay re-derivation of the
  accumulator from recorded state; a policy-identity byte test; A0 tip/branch digest identity.
  *Falsified by* any world-RNG consumption difference, any policy-identity byte change, or any A0 drift.

- **VS2-EC-2 — equal-budget ceiling enforced fail-loud, realized counts reported.** `N` is a declared
  call **ceiling** per budgeted arm (P, U, A), and each arm selects **online over its own realized
  candidate stream** and stops at `N`. Every arm realizes exactly `N` when its own roster is ample
  enough to fill its declared schedule and reports a realized count below `N` otherwise; P is
  period-gated and U/A spend at most the declared per-hour quota `q(h)` (§5.1); R escalates its whole
  stream. The report records **every arm's realized call count** from the call ledger; it does **not**
  assume `realized == N` and it does not equalize arms by truncating any evaluation prefix. The four
  cognitive-ceiling rows are present per cell; any attempt to spend an `(N+1)`-th call is the typed
  fail-loud halt (`CognitiveCeilingHitError`). A `Priority != nil` policy paired with `Selector == nil`
  is refused at construction (`ErrNaturalRunner`, §4.1), so no budgeted arm runs without its
  ceiling-enforcing selector. *Proof method:* per-arm realized-call counts from the call ledger; the
  per-hour shortfall records and the quota schedule `q(h)`; ceiling rows; the fail-loud fixture; the
  construction-refusal fixture.
  *Falsified by* a silently truncated or padded arm, an arm that exceeds `N`, a missing ceiling row, a
  report that omits the realized counts, or an accepted `Priority != nil && Selector == nil`.

- **VS2-EC-3 — adaptive dominates periodic and random across seeds on the vector.** For each declared
  cell, adaptive's per-indicator divergence vector **weakly dominates** both periodic's and random's with
  **strict improvement in ≥1 indicator**, in **≥ the declared number of seeds** (frozen: all of the
  cell's declared seeds; the report also shows the per-seed table). Dominance is per-summary and
  direction-aware: `Max` and `Mean` are **lower-is-better** (adaptive ≤ the other's) and
  `FirstDivergenceHour` is **higher-is-better** (adaptive ≥ the other's) **on the normalised value**
  declared once in §6 (`raw == -1 ? H+1 : raw`). *Proof method:* dominance over the frozen vector per
  summary with the declared directions, computed on the normalised summaries (the §6 `-1 → H+1`
  no-divergence normalisation applied uniformly to every arm and indicator); the per-cell, per-seed
  divergence tables must show each raw `-1` alongside its normalised value; a committed fixture in which
  an identical (never-diverged) arm's raw `-1` is compared under the declared direction and is **not**
  judged worse than a diverged arm (the sentinel-inversion regression).
  *Falsified by* a cell/seed in which a budgeted baseline (periodic or random) **strictly dominates**
  adaptive in a summary under its declared direction. A **clean negative** — no budgeted policy
  dominates any other, so adaptive neither dominates nor is dominated — is **not** an EC-3 miss: it is
  the declared decisive outcome of VS2-EC-9, reported verbatim (§8.4), and §8.4 keeps it distinct from
  the "a baseline dominates adaptive" case.

- **VS2-EC-4 — multi-indicator, no single scalar.** The comparison is vector dominance over all declared
  indicators, each under its declared direction and the §6 no-divergence normalisation (`-1 → H+1`); no
  weighted sum is defined; a scalar win without vector dominance does **not** count as beating.
  *Proof method:* the report carries every indicator (raw and normalised `FirstDivergenceHour`); a
  committed fixture in which an arm wins a scalar aggregate but fails vector dominance is reported as
  NOT beating.
  *Falsified by* a gate that reduces the vector to one number or drops the §6 normalisation.

- **VS2-EC-5 — identity and replay valid across strategy updates.** A declared cell carrying a mid-run
  Role B apply (the VS1 lifecycle) replays byte-exact from genesis with zero model calls — accepted
  history, hourly projections, **allocation decisions**, and both ledgers — and the selector re-derives
  the identical escalation set during replay. The checkpoint manifest's cognition block records the
  policy fingerprint; **at master `46b1b2b` the block wire carries only `Name`/`PeriodHours`/`PhaseHours`
  and restore never compares a supplied policy against the recorded fingerprint** (`internal/world/`
  `natural_cognitive_checkpoint.go`; the 32-byte `WakeFingerprint` is written and decoded but not
  compared). VS2 adds, as new work owned by L2 (§10): an additive, byte-compatible extension of the
  cognition-block wire carrying the canonical priority-policy identity, fingerprint recomputation over
  the full policy (so the priority policy is bound), and a restore-time refusal when the
  declared/supplied policy fingerprint differs from the recorded one. Existing VS1 cognitive bundles
  decode unchanged. *Proof method:* the replay lane; a mutated-policy-fingerprint negative test (changed
  `Priority`, refused restore).
  *Falsified by* any replay mismatch, any model call during replay, or an accepted mismatched policy.

- **VS2-EC-6 — escalation requests observation-bounded (C5, extends EC-6).** The **actor-raised
  escalation request and its predicate** derive only from the actor's own
  `strategy.NaturalObservation`: `EscalationRequest` reads nothing else, and every actor-derived
  candidate feature is observation-only. The **scheduler-level priority feature** `prior_reliability`
  is a declared world-level aggregate row (a versioned cell statistic, §4.2), never another actor's
  private state. The import graph holds (request builder imports no `internal/world`; world imports no
  client); the sentinel scan finds no privileged marker in any recorded request or `allocation` record
  and no other actor's private state. *Proof method:* import-graph test; sentinel + recorded-request
  scan; the request predicate's observation-only type signature; the declared aggregate-row schema for
  `prior_reliability`.
  *Falsified by* one marker, one structural violation, or a feature that reads another actor's private
  state.

- **VS2-EC-7 — protected exploration present and bounded.** With declared `q > 0` (integer fixed-point),
  the selector reserves `ceil(q·N)` of the ceiling `N`, computed as `(N·ExplorationFraction +
  ExplorationScale − 1) / ExplorationScale` in integer arithmetic, for **uniform online draws over
  ordinary candidates the arm meets**, inside the per-hour quota `q(h)` and split across the horizon by
  the declared §4.5 interleaving rule; the exploration set is
  non-empty whenever the arm's own stream and `N` permit; the set is deterministic from the declared
  policy PRNG and recorded on the `allocation` record; the `q = 0` arm isolates pure exploitation.
  *Proof method:* allocation records; the `q > 0` and `q = 0` fixtures.
  *Falsified by* an absent, over-budget, or non-reproducible exploration set.

- **VS2-EC-8 — value model declared and calibrated (P6/ADR 0003).** The value model is a versioned,
  integer fixed-point projection that exposes feature values, uncertainty, selected action, cost, and
  realized downstream value; the **calibration error of predicted priority vs realized effect** is
  reported; the model version is part of run identity. *Proof method:* the `allocation` record schema;
  the calibration report; a float-free arithmetic check.
  *Falsified by* a terminal prose/float priority, a missing realized-value field, or an unversioned
  model.

- **VS2-EC-9 — honest labels, real-model gate, negative valid.** Every report is
  `synthetic/model-conditional`; the real-model arm is labelled **blocked on spend authorisation** and no
  acceptance is claimed without it; a clean negative (no budgeted policy dominates any other) is
  reported verbatim and is a valid, decisive outcome. *Proof method:* report labels; the band logic that
  emits and accepts the negative. *Falsified by* an unlabelled/overclaimed report or a suppressed null.

## 8. Experiment matrix

### 8.1 Arms

| Arm | Selection | Priority layer | Budget | Purpose |
|---|---|---|---|---|
| A0 no-AI | none | nil (no policy) | 0 | control; must reproduce the recorded ticket-19 `natural/7 × v6` evidence byte-exact (VS2-EC-1) |
| R reference | `FullWakePolicy` (every candidate in its own stream) | nil | ceiling ≥ projected A0 count (e.g. 4 096) | the high-budget reference for the fidelity vector; comparative, never ground truth |
| P periodic | period-gated: candidates in hours `h ≡ phase (mod PeriodHours)`, selected in a declared deterministic order, stop at `N` | `PriorityPolicy{Strategy: periodic}` | ceiling `N` (realized count reported) | uniform/periodic baseline the central claim must beat |
| U uniform-random | arm-local uniform draw inside the per-hour quota `q(h)` | `PriorityPolicy{Strategy: uniform-random}` | ceiling `N` | random-sampling baseline |
| A adaptive-priority | online priority ranking + protected exploration, inside the per-hour quota `q(h)` | `PriorityPolicy{Strategy: adaptive}` | ceiling `N` | the candidate under test |

Only A0 and R carry a **nil** `Priority` (no priority layer); every budgeted arm P, U, A carries a
non-nil `PriorityPolicy`, because only a priority-bearing online selector can realize the arm's declared
rule — the periodic strategy included. `N = 256` (§5.1) is the declared ceiling for P, U, A, frozen
before any run; `q = 0.25` protected exploration for A (with a declared `q = 0` ablation at the primary
cell).

**P (periodic), defined exactly:** escalate candidates only at hours satisfying
`h ≡ PhaseHours (mod PeriodHours)`, selecting candidates within those hours in a declared deterministic
order (roster sorted by `(actor)`) and stopping once the arm has spent `N`. It is period-gated,
deterministic, and needs no run-total candidate count. Its realized count is reported: `N` when the
selected hours supply at least `N` candidates over the horizon, below `N` otherwise (an under-budget
cell, §5.3). The declared `PeriodHours` is chosen from the model-free A0 control trajectory (§5.1) so
that the selected hours supply at least `N` candidates, but that projection is never a normative input
and is never asserted to equal the model arm's count.

### 8.2 Cells and seeds

- **Primary cells:** `f ∈ {10, 11, 12}` × seeds `{0, 1, 2, 3}` = **12 cells** (the VS1 declared primary
  cells, for continuity). Each cell runs arms A0/R/P/U/A.
- **Sensitivity-seed extension:** `f = 12` × seeds `{4, 5, 6, 7}` = **4 cells** (f=12 is the declared
  sensitivity density; ticket 19 measured 8 seeds there at f=12). Each runs arms R/P/U/A.
- **Budget-sensitivity (declared extension, optional):** `f = 12` × seed `0` at `N = 512` to show the
  result is not an artifact of one budget.
- **Determinism twin:** `f = 12, s = 0, w = 4`, digest-identical to its workers-1 twin.
- World: mechanism **v6**, base policy **`natural/7`**, `-r 8`, births enabled, horizon 168 — identical
  to VS1. VS4 owns any widening.

### 8.3 Workers

Workers 1 for every matrix cell (so the spec string is stable); the `w = 4` twin is the determinism
witness, exactly as VS1.

### 8.4 Null / negative interpretation

- **Adaptive dominates** (VS2-EC-3): the central claim has its first evidence at equal budget, on the
  declared substrate.
- **Clean negative — no budgeted policy dominates any other** (or the arms are mutually
  non-dominating): the decisive, reportable outcome that the decision space does not (yet) reward
  reasoning at this scale. It is reported verbatim (VS2-EC-9), is **not** an EC-3 miss (EC-3 is
  falsified only when a baseline strictly dominates adaptive), and is **not** a failure of the
  experiment.
- **Periodic or random dominates adaptive:** reported verbatim; it falsifies the value model as
  instantiated and is a valid result, distinct from the clean negative.
- **Reference incomplete / budget shortfall:** reported as an honest gap, never a cheaper substitute.

## 9. Proof method and evidence plan

- **Hermetic by default.** `make check` and every unit test must not touch the network: the L1
  `FakeClient` (`internal/cognition`) supplies every response, and recorded fixture ledgers drive the
  replay paths. Costs are modelled price-table charges over fake usage, never spend.
- **Declared live smoke.** Before any matrix, one live cell at the smallest declared fixture: a real
  ledger with a real call, a real (unpriced or priced) cost record, and a successful replay — the only
  evidence that proves the gateway contract and the allocation wiring against the real model. Gated on
  the spend authorisation (VS2-EC-9).
- **Matrix.** §8.2 cells per arm, each run fresh (new checkpoint directory, no clobber), one report per
  cell, evaluator output written to `.tmp/30-vs2-evaluation.json` (L4c) and
  `.tmp/31-vs2-evaluation.json` (L2b), evidence quoted verbatim in the ticket's evidence
  record; the docs/evidence repair re-run writes `.tmp/34-vs2-evaluation.json`.
- **Fixtures.** A scripted value-model fixture (deterministic candidate scores) exercising
  R/P/U/A-select end to end and replaying byte-exact; a **broken value-model fixture** (out-of-domain
  weight, unknown feature, non-monotone cost) rejected with a typed reason and zero world mutation; a
  `q = 0` fixture and a `q > 0` fixture isolating exploration; a mutated-policy-fingerprint fixture
  refusing restore; a half-wired-policy fixture refusing construction (`Priority != nil &&
  Selector == nil`); an under-budget fixture in which a short roster records the per-hour shortfall and
  the arm reports a realized count below `N`.
- **Zero-use gates.** A0 byte-identity (VS2-EC-1) runs first; a failure is a stop, not a tuning
  opportunity.
- **Independent review.** Two fresh reviewers (code and evidence) after the implementation lanes,
  independent of the workers, per the project's review discipline.
- **Limits disclosed.** Model-conditional; the reference is a proxy; digest chains detect damage, not
  replacement; no real-world claim.

## 10. Lane board (one writer per file area)

| Lane | Scope (sole writer) | Depends on |
|---|---|---|
| L1 priority + value model (protocol) | `internal/cognition/protocol/wake.go` (additive `Priority` field + the degenerate nil-`Priority` `Select`), new `internal/cognition/protocol/{priority.go,valuemodel.go}` (the `EscalationSelector` online selector interface, `PriorityStrategy`, `PolicyPredicate`, the pure value projection) (+ tests); keeps `VS1WakePolicy`/`FullWakePolicy` bytes unchanged | — |
| L2 world escalation gate + checkpoint binding | `internal/world/natural_cognition.go` (generalized gate + `Selector` seam + the construction-time refusal of `Priority != nil && Selector == nil`) (+ tests); the claim-loop hunks of `internal/world/natural.go` (`Natural.allocate`); `internal/world/natural_cognitive_checkpoint.go` (additive cognition-block wire extension carrying the priority-policy identity + restore-time fingerprint refusal) | L1 |
| L3 cognition wiring: arm-local accumulator, allocation records, PRNG, price adapter | new `internal/cognition/{priority.go,allocation.go}` (the arm-local deterministic online accumulator seeded with the call ceiling `N` and `RandSeed`, adapted to `protocol.EscalationSelector`; recorded spent-call count + PRNG position); the `cognition.PriceTable` → `protocol.CallPrice` adapter; `internal/cognition/wake.go` aliases; `internal/cognition/codec.go` (additive `InterventionAllocation` kind + `AllocationRecord`) (+ tests) | L1 |
| L4 experiment runner + arms + equal-budget gate + metric | new `cmd/vs2/**`; the shared reusable mechanics extracted once into `internal/experiment/**` (`divergenceVector`, `summarize`, `decisionErrorIndicator`, fixture/stack wiring); wires the L3 arm-local accumulator into the world's `Selector` seam and computes the equal-budget report with per-arm realized counts over the full horizon; the extracted `divergenceVector` computes over the full declared `H` and flags/refuses any arm whose projection is shorter than `H` rather than inheriting the scaffold's `min(len(b), len(r))` prefix (§5.3); `cmd/vs1` migrated to consume `internal/experiment` **with VS1 EC-1/EC-2 byte-identity re-proved** | L1–L3 |
| L5 docs | this file's evidence record and `docs/adr/0007-value-based-cognitive-allocation.md` | L4 |
| L6 review | two fresh reviewers, no shared context with the workers | L4 |

No lane may touch `internal/kernel`, `internal/scheduler`, `internal/component`, `internal/sim`, the
frozen `natural/5..9` policy bytes, the frozen v5–v9 evidence, or another lane's files. The priority
layer is nil-tolerant: a nil `Priority` leaves every VS1 code path and every recorded run byte-identical.

**Interfaces and dependencies.**

- L1 owns the pure protocol surface (types + `CallPrice` + the pure value projection + the degenerate
  nil-`Priority` `Select`). L2 and L3 both consume the `protocol.EscalationSelector` interface; L2 must
  not import L3 (the world calls an interface, exactly as VS1's `ClaimSeam`).
- L2 owns the roster builder (the per-hour `[]EscalationCandidate` from the claim-wake set) and the
  checkpoint binding work VS2 adds: the additive cognition-block wire extension, the full-policy
  fingerprint recomputation, the restore-time refusal (VS2-EC-5), and the construction-time refusal of
  `Priority != nil && Selector == nil` (§4.1). At `46b1b2b` the block binds only
  `Name`/`PeriodHours`/`PhaseHours` and restore compares no supplied policy, so this is new work with a
  named owner, not existing behaviour.
- L3 owns the arm-local deterministic online accumulator (recorded spent-call count + policy PRNG
  position + budget view) that implements `protocol.EscalationSelector` and the `allocation` record
  producer; L4 wires it into the world.
- L4 is the integrator: it owns `cmd/vs2`, the shared `internal/experiment` extraction, the wiring of the
  L3 arm-local accumulator into the world's `Selector` seam, and the one migration of `cmd/vs1`; it
  re-runs VS1's EC-1/EC-2 to prove the migration changed no world byte.
- The extraction of the fidelity vector and mechanics into `internal/experiment` has **one home** (no
  duplicate copy); `cmd/vs1`'s migration is the price of that single home and is gated by byte-identity
  re-proof. The alternative (leave `cmd/vs1` untouched and duplicate) is recorded in §11.

## 11. Risks, stop conditions, and open operator questions

### 11.1 Risks

- **The real-model arm is blocked on spend** (§1.3): no VS2 acceptance is possible without it; a
  hermetic-only VS2 proves the allocation machinery, not the central claim.
- **The model is unpriced**, so cost cannot be an axis and the cost ceiling cannot bind for a live run;
  a cost-equalized comparison is deferred to a declared operator gate.
- **f=10/11 headroom is small** (the frozen policy already reaches the surplus indicator in 8/8 seeds
  there; VS1 spec §4.2), so divergence may appear without improvement; f=12 is the declared sensitivity
  cell and the primary place a difference can appear. A null at f=10/11 is expected and reported.
- **The value model is a versioned hypothesis** and may be uninformative; the calibration report
  (VS2-EC-8) is the instrument that shows whether it is.
- **Migrating `cmd/vs1` onto `internal/experiment`** touches frozen VS1 code; the risk is bounded by the
  EC-1/EC-2 byte-identity re-proof, and the alternative (duplication) violates one-home discipline.
- **Selection granularity and temporal concentration differ across arms (residual asymmetry,
  disclosed).** Every budgeted arm selects online over its own realized stream under the same declared
  ceiling `N` (§8.1), but their per-hour selection rules differ: U and A carry a **hard per-hour cap
  `q(h)`**, while P is **period-gated and has no per-hour cap** — it escalates every candidate in its
  selected hours in the declared order until it spends `N`. Equal budget therefore means **exactly equal
  spend only under the ample-roster condition** of §5.1/§5.3, in which every arm realizes `N` over `H`;
  when the roster thins, P can **bank calls up to `N` in its selected hours before the thinning**,
  while U and A can end **under-budget** if a late hour offers fewer than `q(h)` candidates or fewer
  than the remaining budget. This is an honest residual asymmetry of an endogenous, arm-specific
  candidate stream at a fixed ceiling, not a silently equalized comparison: every arm's realized count
  is reported and interpreted at the full horizon `H` (§5.3), and the primary cells are chosen so the
  roster is ample relative to `N = 256` (§11.3.2).
- **Network latency** lands in the coordinator's pre-batch path (bounded by the VS1 declared deadlines,
  measured separately from engine compute).

### 11.2 Stop conditions (a fired condition halts the lane and reports, never patches around it)

Any byte drift in the frozen substrate or in the VS1 recorded A0 evidence; a policy-identity byte change
for the nil-priority `WakePolicy`; a model call during replay; a world mutation on any refusal/fallback
path; an arm that silently exceeds or pads its declared ceiling, or whose realized count is not
reported; a silent ceiling; an accepted mismatched policy fingerprint; an escalation request or
candidate feature derived from privileged state; an unlabelled report; any band, constant, `N`, or cell
edited after its first run.

### 11.3 Open questions for the operator

1. **Spend / budget gate.** The real-model arm is blocked on spend authorisation (inherited from VS1)
   and the application model has no price entry, so cost cannot be the equal-budget axis. Confirm: (a)
   authorize the VS2 live matrix (or a single declared live smoke first); (b) declare a price (or
   explicitly waive the cost ceiling and accept call/token equalization only).
2. **The declared call ceiling `N`.** Confirm `N = 256` calls/cell as the ceiling for P, U, A (or name
   another declared value) and whether the `N = 512` budget-sensitivity extension should run.
3. **The periodic period and the quota schedule.** The P arm is period-gated
   (`h ≡ PhaseHours (mod PeriodHours)`, candidates selected in a declared deterministic order, stop at
   `N`). Confirm the declared `PeriodHours` (chosen from the model-free A0 control trajectory so the
   selected hours supply at least `N` candidates) and the quota schedule `q(h) = floor(N/H) + (1 if
   h < (N mod H) else 0)` for U and A (§5.1), or name another declared period and schedule.
4. **Protected exploration fraction.** Confirm `q = 0.25`, and whether a `q`-sensitivity sweep is wanted.
5. **The value-model feature set, its declared derivations, and weights** (Appendix C.1/C.2): confirm
   the declared feature list, the exact integer fixed-point derivations (including the
   `prior_reliability` aggregate, the `last_denial` encoding, and the `vs2-escalation/1` predicate
   body), the integer fixed-point weights, the feature-set version `vs2-features/1`, and the cost
   proxy, or name the additional observation-visible features (C5-compatible) that should be in the
   contract. The derivations and weights are a declared, versioned, reversible default pending this
   confirmation (§4.2, Appendix C.1/C.2).
6. **`cmd/vs1` migration vs. duplication.** Confirm the single-home extraction into `internal/experiment`
   with the gated VS1 migration, or require `cmd/vs1` to stay untouched (accepting a temporary duplicate
   copy of the fidelity mechanics).
7. **Acceptance without the real-model arm.** Confirm that a hermetic-only VS2 is a declared **not
   accepted** state (mechanism proof only), and that a clean negative from a real-model matrix is
   accepted as decisive (VS2-EC-9).

## 12. Declared VS1 reductions inherited by VS2

VS2 builds on VS1's delivered surface, so VS1's declared reductions
(`.scratch/21-vs1-mind-in-the-loop/spec.md` §14.1) are carried forward where they affect VS2:

- Newborn inheritance after apply is not implemented (VS1 §14.1.1) — VS2's VS2-EC-5 replay covers the
  single-apply case; a mid-run rollback-within-a-run is outside the single-bundle replay scope
  (§14.1 item 2) and is left to the full-ledger replay lane.
- The live Role B audit set is `{24,48,72,96,120,144}` (VS1 §14.1.3); `h168` is the measurement
  boundary. This is unchanged in VS2.
- The hermetic model (VS1 §14.1.6): every budgeted VS2 arm runs the deterministic `FakeClient` until
  spend is authorized; costs are modelled price-table charges over fake usage. VS2's value model and
  calibration are therefore proven on fake usage, and the central-claim result awaits the real model.
- A Role A intervention records the level-1 outcome, not the world verdict (VS1 §14.1.9); the
  `allocation` record is likewise appended before the world commits and its `result` is the level-1
  allocation outcome, joined to the committed journal for the indicator-8 rate.
- The sandbox gating regression is halt-or-extinction, not a per-band comparison (VS1 §14.1.7); VS2
  does not change the sandbox and does not gate on it.

## Appendix A — Types (the implementable contract)

### A.1 Extension of `internal/cognition/protocol/wake.go`

```go
// WakePolicy gains one additive, omitempty field. When Priority is nil the
// canonical identity is byte-identical to VS1's.
type WakePolicy struct {
    Name        string          `json:"name"`
    PeriodHours int             `json:"period_hours"`
    PhaseHours  int             `json:"phase_hours"`
    Priority    *PriorityPolicy `json:"priority,omitempty"` // VS2; nil == VS1
}

// PriorityStrategy is the closed set of declared allocation strategies.
type PriorityStrategy string

const (
    StrategyPeriodic      PriorityStrategy = "periodic"
    StrategyUniformRandom PriorityStrategy = "uniform-random"
    StrategyAdaptive      PriorityStrategy = "adaptive"
    StrategyFull          PriorityStrategy = "full"
)

// ExplorationScale is the fixed-point denominator of ExplorationFraction.
const ExplorationScale int64 = 1_000_000

// PriorityScale is the fixed-point denominator of Priority (Appendix C.2):
// Priority = ExpectedValue * PriorityScale / max(1, ExpectedCost).
const PriorityScale int64 = 1_000_000

type PriorityPolicy struct {
    Strategy            PriorityStrategy `json:"strategy"`
    ExplorationFraction int64            `json:"exploration_fraction"` // q = frac/ExplorationScale, q in [0,1)
    RandSeed            uint64           `json:"rand_seed"`            // policy PRNG seed, from run identity
    ValueModel          ValueModelSpec   `json:"value_model"`
    CallCeiling         uint64           `json:"call_ceiling"`         // declared call ceiling N; 0 for the reference
}

// PolicyPredicate is the versioned declared escalation-request predicate
// evaluated over the actor's own observation (§4.3); integer fixed-point and
// observation-only.
type PolicyPredicate struct {
    Version    string             `json:"version"`
    Thresholds []FeatureThreshold `json:"thresholds"`
}

type FeatureThreshold struct { Feature string; Threshold int64 } // integer fixed-point

type BudgetState struct {
    CallsSpent     uint64 `json:"calls_spent"`
    CallsRemaining uint64 `json:"calls_remaining"`
    CallsBudget    uint64 `json:"calls_budget"` // N
}

type EscalationReason string

const (
    EscalationRequestedValue  EscalationReason = "high-value"
    EscalationRequestedByActor EscalationReason = "actor-requested"
    EscalationExploration     EscalationReason = "protected-exploration"
)

type EscalationCandidate struct {
    Actor    sim.EntityID              `json:"actor"`
    Hour     int                       `json:"hour"`
    Features ValueFeatures             `json:"features"`
    Requested bool                     `json:"requested"`
    Reason   EscalationReason          `json:"reason"`
}

type Selection struct {
    Hour        int              `json:"hour"`
    Selected    []sim.EntityID   `json:"selected"`
    Exploration []sim.EntityID   `json:"exploration"`
    Scores      []CandidateScore `json:"scores"`     // per candidate, recorded
    Budget      BudgetState      `json:"budget"`     // CallsSpent/CallsRemaining/CallsBudget
    Quota       int              `json:"quota"`      // U/A: declared per-hour quota q(h); P: remaining budget (§5.1)
    Shortfall   int              `json:"shortfall"`  // U/A: max(0, q(h) - len(roster)); P: 0 (§5.1)
    PRNGPos     uint64           `json:"prng_pos"`   // consumed position of the policy PRNG
}

// EscalationSelector is the deterministic arm-local online accumulator (§4.1
// item 4). Seeded from the declared call ceiling N (CallsBudget) and RandSeed;
// Budget.CallsSpent and PRNGPos are recorded run state, so replay re-derives the
// identical selection sequence. It is online: it consumes each hour's roster as
// it arrives, spends at most the declared per-hour quota q(h) for U and A (or is
// period-gated for P), and stops once it has spent N. It needs no run-total
// candidate count and never exceeds N.
type EscalationSelector interface {
    Select(hour int, roster []EscalationCandidate, budget BudgetState) (Selection, error)
}
```

### A.2 Value model (`internal/cognition/protocol/valuemodel.go`)

```go
type ValueFeatures struct {
    Energy, Hunger, Capital, GranaryStock, LastStoredMealHour, StockedHomeSlots int64
    LastDenial        string
    DecisionSensitivity int64 // fixed-point; distance to the frozen thresholds
    Urgency             int64
    Impact              int64
    PropagationRisk     int64
    PriorReliability    int64
    EscalationRequested int64 // 0/1, from the actor's own EscalationRequest predicate (§4.3)
}

type FeatureWeight struct { Feature string; Weight int64 } // integer fixed-point

type CostProxy struct {
    Version              string
    ConstantMicrosPerCall int64
    ProjectedPromptTokens uint64
    ProjectedCompletionTokens uint64
}

type ValueModelSpec struct {
    Version    string
    FeatureSet string // declared candidate feature-set version (Appendix C.1)
    Weights    []FeatureWeight
    CostProxy  CostProxy
}

// CallPrice is the protocol-local cost type for the value projection. The
// projection lives in protocol, which internal/cognition imports, so it cannot
// take cognition.PriceTable without an import cycle; the runner adapts a
// declared PriceTable to CallPrice at the boundary.
type CallPrice struct {
    Version                  string
    PromptMicrosPerToken     int64
    CompletionMicrosPerToken int64
}

type CandidateScore struct {
    Actor    sim.EntityID `json:"actor"`
    ExpectedValue int64   `json:"expected_value"`
    ExpectedCost  int64   `json:"expected_cost"`
    Priority      int64   `json:"priority"`
    Requested     bool    `json:"requested"`
}
```

## Appendix B — `allocation` intervention record (extends the C8 ledger)

A new additive `InterventionKind` `allocation` appended once per escalated hour, reusing
`Ledger.AppendIntervention` (`internal/cognition/ledger.go`) and the hash chain. It carries a new
omitempty `Allocation *AllocationRecord` field on `InterventionRecord` (`internal/cognition/codec.go`);
existing records are byte-unchanged.

```go
type AllocationRecord struct {
    PolicyFingerprint string           `json:"policy_fingerprint"`
    RosterSize        int              `json:"roster_size"`     // candidates offered at this hour
    Quota             int              `json:"quota"`           // U/A: declared q(h); P: remaining budget (§5.1)
    Shortfall         int              `json:"shortfall"`       // U/A: max(0, q(h) - roster size); P: 0 (§5.1)
    Selected          []sim.EntityID   `json:"selected"`
    Exploration       []sim.EntityID   `json:"exploration"`
    Scores            []CandidateScore `json:"scores"`
    Budget            BudgetState      `json:"budget"`
    CallCeiling       uint64           `json:"call_ceiling"`    // declared ceiling N
    PRNGPos           uint64           `json:"prng_pos"`
    RealizedCalls     uint64           `json:"realized_calls"`  // cumulative calls at this record
}
```

## Appendix C — Declared contracts

### C.1 Candidate feature contract (C5 — observation-only, plus one declared world-level aggregate)

All rows except `prior_reliability` are **observation-only** and read the actor's own
`strategy.NaturalObservation`; `prior_reliability` is a declared world-level aggregate row carrying no
other actor's private state (EC-6).

| Feature | Source | Meaning |
|---|---|---|
| `energy` | own `NaturalObservation.energy` | own energy |
| `hunger` | own `NaturalObservation.hunger` | own hunger |
| `capital` | own `NaturalObservation.capital` | own capital |
| `granary_stock` | own `NaturalObservation.granary_stock` | own granary |
| `last_stored_meal_hour` | own `NaturalObservation.last_stored_meal_hour` | own last stored meal |
| `stocked_home_slots` | count of own `home_slots` with stock ≥ 1 | own visible home slots |
| `last_denial` | own `NaturalObservation.last_denial` | own last denial |
| `decision_sensitivity` | derived from `energy`,`hunger`,`granary_stock`,`capital`, `last_stored_meal_hour`, `stocked_home_slots` | distance to the frozen claim thresholds |
| `urgency` | derived from `energy`,`hunger` | survival proximity |
| `impact` | derived from `capital`,`granary_stock` | birth-gate / investment proximity |
| `propagation_risk` | derived from `stocked_home_slots` | contested-slot risk visible to the actor |
| `prior_reliability` | declared world-level aggregate row (versioned cell statistic) | prior realized value in this region |
| `escalation_requested` | `EscalationRequest` predicate over own `NaturalObservation`, materialized as `ValueFeatures.EscalationRequested` (0/1) | the actor's own explicit request |

**Declared derivations (fixed point).** The feature set is `vs2-features/1`
(`ValueModelSpec.FeatureSet`, part of run identity). The declared constants mirror the frozen claim
thresholds and are cross-checked against the world's exported frozen constants: `DerivedFeatureScale =
1000`; `FeatureEnergyCapacity = 12`; `FeatureHungerCapacity = 24`; `FeatureSlotsPerPatch = 8`;
`FeatureBuildEnergyFloor = 5`; `FeatureConsentMinEnergy = 3`; `FeatureConsentMinCapital = 2`;
`FeatureConsentMinGranary = 3`; `FeatureStoredMealGranary = 1`; `FeatureBoundaryWindow = 2`.
Division is integer division (Go `/` on non-negative values); `min` is the integer minimum.

- `urgency` = `DerivedFeatureScale · ((FeatureEnergyCapacity − energy) + hunger) /
  (FeatureEnergyCapacity + FeatureHungerCapacity)` — survival proximity, monotone non-increasing in
  `energy` and non-decreasing in `hunger`.
- `impact` = `DerivedFeatureScale · (min(capital, FeatureConsentMinCapital) +
  min(granary_stock, FeatureConsentMinGranary)) /
  (FeatureConsentMinCapital + FeatureConsentMinGranary)` — birth-gate / investment proximity, saturating
  when the consent gate (`capital ≥ 2`, `granary_stock ≥ 3`) is met.
- `propagation_risk` = `DerivedFeatureScale · min(stocked_home_slots, FeatureSlotsPerPatch) /
  FeatureSlotsPerPatch` — contested-slot risk, monotone non-decreasing in `stocked_home_slots`.
- `decision_sensitivity` = `DerivedFeatureScale · (Σ_τ max(0, FeatureBoundaryWindow − |value_τ −
  threshold_τ|)) / (FeatureBoundaryWindow · 8)`, over the eight declared `(row, frozen threshold)`
  pairs `τ` = `(energy, 5)`, `(energy, 3)`, `(granary_stock, 1)`, `(granary_stock, 3)`, `(capital, 2)`,
  `(hunger, 24)`, `(stocked_home_slots, 1)`, `(last_stored_meal_hour, hour)` — the bag build-energy
  floor, the two consent floors, the stored-meal availability floor, the gather-availability floor, the
  starvation clock edge, and the stored-meal freshness boundary (`last_stored_meal_hour < hour`). A row
  rises as any own value approaches a boundary, in either direction.
- `last_denial` (string): `""` for no denial, `"gather"` for a gather denial, `"build"` for a build
  denial.
- `prior_reliability` = the declared neutral world-level prior `DeclaredPriorReliability =
  DerivedFeatureScale/2 = 500` (version `vs2-prior/1`): a declared, versioned, reversible default; the
  scheduler-level row, identical for every candidate of a cell and never read by the predicate.
- `escalation_requested` = 1 iff the declared predicate `vs2-escalation/1` holds, i.e. `urgency ≥ 500`
  **and** `decision_sensitivity ≥ 500`; the predicate returns `EscalationRequestedByActor`. It
  evaluates only the observation-only rows and fails closed for an empty predicate or any row it may
  not read (`prior_reliability`, an unknown name).

### C.2 Value-model contract (frozen instance to be confirmed, §11)

| Field | Value |
|---|---|
| `Version` | `vs2-value/1` |
| `FeatureSet` | `vs2-features/1` (the declared feature-set version of Appendix C.1; part of run identity) |
| `Weight(decision_sensitivity)` | 4 |
| `Weight(urgency)` | 3 |
| `Weight(impact)` | 2 |
| `Weight(propagation_risk)` | 2 |
| `Weight(prior_reliability)` | 1 |
| `Weight(escalation_requested)` | 5 |
| `CostProxy.Version` | `vs2-cost-proxy/1` |
| `CostProxy.ConstantMicrosPerCall` | 1 |
| `ExplorationFraction` | `250000` (q = 0.25 over `ExplorationScale = 1_000_000`, integer fixed-point) |
| `PriorityScale` | `1_000_000` (declared `int64` constant; the fixed-point denominator of `Priority`) |
| `Priority` | integer fixed-point `ExpectedValue·PriorityScale / max(1, ExpectedCost)` |

The all-zero weight vector must reduce the priority to the uniform-random baseline (a pinned reduction
test), and the `q = 0` arm must isolate exploitation.

## Appendix D — Contradictions and gaps found in the founding documents

Current disagreements at master `46b1b2b`; items resolved by the ticket-23 reconciliation are marked
resolved rather than restated.

1. **`docs/ARCHITECTURE.md` §3/§4 and `docs/P0-SPEC.md` §2/§10 already record `EscalationRequest` as
   "not yet implemented" and VS2 owns it; `docs/P0-SPEC.md` §8 (the state-change diagram) still names
   `EscalationRequest` without that annotation.** The code has no `EscalationRequest` type at
   `46b1b2b`; VS2 defines it (§4.3, Appendix A.1). The only remaining disagreement is the unannotated
   §8 diagram line.
2. **`docs/ARCHITECTURE.md` §4 / `docs/PRINCIPLES.md` P7 describe value-based priority, escalation
   requests, and protected exploration as the target end state; VS1 implements the declared baseline
   only.** VS2 is that target; the "present in every version" invariant is read as "the concept's object
   exists", and VS2 raises C7 from present to tested.
3. **Resolved at `46b1b2b`.** `docs/VERTICAL-SLICES.md` no longer marks the fidelity metric "(needs
   operator confirmation)"; it states the metric as frozen ("Metric definition (frozen)") and the
   product-decisions section repeats "the frozen definition given in VS2". No disagreement remains.
4. **Resolved at `46b1b2b`.** `README.md` carries no present-tense "wake selectively" claim (the phrase
   does not appear); it describes only the implemented substrate. VS2 may state the claim as fact only
   once the central claim itself is accepted — the real-model arm evidenced and the operator spend gate
   passed (§11.3); landing the mechanism alone does not earn it, and the claim remains **not accepted**
   (status above). No disagreement remains.
5. **`docs/adr/0006` (Addendum) records the application model as unpriced; the founding budget
   discipline (`docs/PRINCIPLES.md` P15) assumes a costed budget.** VS2 resolves the tension by defining
   equal budget on the call axis and treating cost equalization as a declared operator gate (§5, §11),
   rather than fabricating a price or silently dropping the axis.

## Appendix E — Design-review repair record (ticket/evidence record)

This appendix is the **ticket/evidence record** for the VS2 design repairs: it maps each review finding
to the repair now in force in this contract. It is a change record, **not normative contract text** —
the normative contract is §1–§12 and Appendices A–D, which state only current facts; the repair history
lives here. Design-only against master `46b1b2b`; every cited symbol resolves there.

**Prior repair round (design-v4).** Repairs carried from the previous repair commit; retained as part of the finding → repair record.

| Finding | Summary | Repair (location in this contract / ADR) |
|---|---|---|
| P1.1 (blocking, root cause) | The normative equal-budget path needed a **model-free pre-pass** fixing a run-total candidate count so an exact size-`N` subset could be extracted; the candidate stream is endogenous and its length is unknown at start, so the pre-pass is unimplementable. | Removed the pre-pass and the run-total count entirely: each budgeted arm selects **online** over its own stream and **stops at the declared ceiling `N`**; the A0 trajectory is used only to project scale and choose declared constants. §4 intro, §4.2, §5.1, Appendix A.1, ADR 0007 (Decision, Alternatives, Consequences). |
| P1.2 (blocking) | U was declared **reservoir-style** and U/A "**exact-on-own-stream**" selecting exactly `min(N, |U_arm|)` — still an exact uniform size-`N` subset claim over a stream of unknown length. | Replaced by declared online rules ending at `N`: **U** spends at most the per-hour quota `q(h)` on policy-PRNG draws; **A** ranks online by priority and spends at most `q(h)`; both stop at `N`. `q(h)` is a declared fixed schedule depending on no realized stream, with per-hour shortfalls recorded. §4.1 item 4, §4.5, §5.1, §8.1, EC-7 (§7), Appendix A.1/B, ADR 0007. |
| P1.3 (blocking) | The **matched-realized-count / common-matched-prefix** evaluation (`m = min(n_P, n_U, n_A)`) was an inconsistent workaround mixing a full-horizon claim with a prefix evaluation. | Removed the matched count and the per-arm prefix: every arm's divergence vector is computed over the **same full declared horizon `H`**, the primary gate is at full `H`, and every arm's realized count is disclosed; an under-budget cell is reported and interpreted, never equalized. §3 (C11), §5.3, EC-2 (§7), §10 (L4), ADR 0007 (Alternatives, Consequences). |
| P1.4 (blocking) | The **periodic arm P** was defined through an exact-`N` / `|U|`-independent construction (top-up/trim, even-index sampling) inconsistent with an endogenous stream. | P is now **period-gated**: it escalates candidates only at hours `h ≡ PhaseHours (mod PeriodHours)`, in a declared deterministic order (roster sorted by `(actor)`), and stops at `N`; its realized count is reported. §5.1, §8.1, EC-2 (§7), §11.3.3, ADR 0007. |
| P2.1 | §11.2's stop condition fired on "realized calls ≠ `N`", contradicting the declared under-budget outcome. | Reworded: the stop fires on "an arm that silently exceeds or pads its declared ceiling, or whose realized count is not reported". §11.2. |
| P2.2 | EC-3's *Falsified by* clause (any case where adaptive fails to dominate) contradicted its clean-negative sentence. | EC-3 is now falsified only when a budgeted **baseline strictly dominates adaptive**; the mutually non-dominating clean negative is the declared VS2-EC-9 outcome, explicitly not an EC-3 miss, and §8.4 keeps the cases distinct. EC-3 (§7), §8.4. |
| P2.3 | No construction-time refusal for a priority policy without its selector: a budgeted arm could silently fall back to the VS1 hour gate. | Declared a `NaturalCognition.validate` refusal of `Priority != nil && Selector == nil` with the typed `ErrNaturalRunner`, mirroring `internal/world/natural_cognition.go:46-60`, owned by lane L2. §4.1 (item 5), EC-2 (§7), §10 (L2). |
| P2.4 | The declared `escalation_requested` weight (5) was not applicable: `ValueFeatures` had no such field. | Added `EscalationRequested int64` to `ValueFeatures` (0/1 from the actor's own `EscalationRequest` predicate) and named it in the C.1 feature table. Appendix A.2, Appendix C.1. |
| P2.5 | Appendix E.1 retained rows asserting the removed pre-pass / exact-`N` framing. | Removed: this appendix now holds only the current finding → repair map, with no prior-review narrative. Appendix E. |

**Current repair round (design-v5).** The fourth independent review returned FAIL, narrow: the
equal-budget contract is implementable and sound; one blocking (P1) and five P2 findings plus one
non-blocking policy-conformance item remained. The finding → repair map:

| Finding | Summary | Repair (location in this contract / ADR) |
|---|---|---|
| P1 (blocking) | The declared dominance gate **inverts for a never-diverged indicator**. `FirstDivergenceHour` is declared higher-is-better ("divergence begins later"), but the reused scaffold initialises it to `-1` and sets it only at the first non-zero value (`cmd/vs1/report.go:349-357`): `-1` means "never diverged" = **best**. Under "adaptive ≥ other", an identical (better) arm with `-1` loses, so weak dominance can fail and a strictly closer arm can be judged non-dominating, falsely falsifying the central claim. | Declared the no-divergence encoding **once** in §6: normalise `raw == -1` → `H+1`, so never-diverged counts as best and higher-is-better holds. The direction-aware comparison in EC-3/EC-4 references that single declaration; EC-3's proof method adds the normalisation and a sentinel-inversion regression fixture; EC-4 applies the same normalisation. §6, EC-3, EC-4. |
| P2.1 | `divergenceVector` truncates to `min(len(b.Hours), len(r.Hours))` (`cmd/vs1/report.go:317-318`); with all arms at `H` this is a no-op, but a short arm would silently contradict §5.3's full-`H` evaluation. | §5.3, the L4 lane, and ADR 0007 (Consequences) now state that the extracted `internal/experiment` mechanics compute over the full declared `H` and flag/refuse a short projection rather than relying on the scaffold's min-prefix. §5.3, §10 (L4), ADR 0007 (Consequences). |
| P2.2 | The per-hour exploration/exploitation interleaving for arm A was undeclared, so two implementers could produce different A arms. | Declared A's deterministic interleaving rule in §4.5 (cumulative exploration target `T(t) = ceil(E·t/H)`, `E = ceil(q·N)`, hourly exploration `e(h) = T(h+1) − T(h)`; the first `e(h)` slots are exploration, the rest priority-ranked exploitation), referenced from §5.1 (A bullet) and EC-7. §4.5, §5.1, EC-7. |
| P2.3 | `Selection.Quota`/`Selection.Shortfall` were undefined for P (period-gated, no per-hour quota). | Declared P's encoding: `Quota = remaining budget (N − CallsSpent)`, `Shortfall = 0`; the field comments in Appendix A.1 and Appendix B updated. §5.1, Appendix A.1, Appendix B. |
| P2.4 | §5.1 contradicted itself on `N`'s provenance ("intentionally not derived from any candidate count" vs the A0 claim-wake count used to choose `N`). | Reworded: `N` is fixed from the model-free A0 (no-AI) projection before any run and is **not derived from any model-bearing arm's realized candidate count**. §5.1, ADR 0007 (Decision). |
| P2.5 | §11.1 overclaimed "no selection-granularity mismatch": P has an uncapped per-hour schedule while U/A have a hard `q(h)` cap, so equal budget is exact only under the ample-roster condition, and P can bank `N` before a thinning roster while U/A may end under-budget. | Replaced with the honest residual asymmetry (hard per-hour cap vs period-gated uncapped schedule; exact equality only under the ample-roster condition), disclosed with the realized-count reporting. §11.1. |
| P2.6 (non-blocking, policy conformance) | Appendix E carried prior-review narrative ("returned BLOCK ...") inside the fact document. | Appendix E relabelled as the **ticket/evidence record** (not normative contract text); the review-narrative opening removed; the finding → repair map retained; the normative sections state current facts only. Appendix E. |

**Verification record (design-v5).** Authoritative tree master `46b1b2b`; branch
`feature/24-vs2-attention-design-v5`; base commit `a71dd2c`; repair commit `aebe4ef`; final reviewed tip
`ed27c85` (landed as the accepted-design merge `b44ac3e`).
`git diff --check` clean; only this file and `docs/adr/0007-value-based-cognitive-allocation.md`
changed. Commands run: `git diff --check a71dd2c..HEAD` (clean), `git diff --stat a71dd2c..HEAD`
(only the two in-scope files), `git show --stat aebe4ef` (the repair commit: the two in-scope files).
Symbol checks at `46b1b2b`:
`NaturalCognition.validate` half-wired refusal → `internal/world/natural_cognition.go:46-60`;
`ErrNaturalRunner` → `internal/world/natural.go:85`; `CognitiveCeilingHitError` →
`internal/cognition/budget.go:82`; `NaturalObservation` → `internal/strategy/natural.go:337`; claim
wakes → `internal/world/natural.go:1794-1804` (`WakeNeedThreshold`/`WakeBirth`);
`summarize`/`FirstDivergenceHour` sentinel → `cmd/vs1/report.go:349-357`; `divergenceVector` min-prefix
→ `cmd/vs1/report.go:317-318`. All VS2 symbols are design-level (no VS2 code exists at `46b1b2b`).

**Non-blocking review residuals carried from the design-v5 review (recorded 2026-10-13T20:32:02+08:00, CST, by the integrating worker; ticket-log note only, not normative contract text).** The fifth independent review
passed the design at reviewed tip `ed27c85` and recorded three non-blocking residuals for the
implementation lanes. They are logged here as evidence; the normative sections and the ADR stay
unchanged at the reviewed revision, and each names the section a later repair round would touch:

1. **§5.1 `N` provenance.** The `N`-magnitude parenthetical ("≈1/8 of the VS1 A3 projection of
   ≈1 700–2 000 calls/cell; the VS1 A1 periodic-6 arm realized ≈300/cell") rests on VS1 model-arm
   figures that are not normative inputs. A later repair should anchor the magnitude on the **A0
   (no-AI) projection** — the only model-free count the contract admits — and label the A3/A1 figures
   explicitly as **non-normative scale context**.
2. **EC-3 falsifier.** EC-3's *Falsified by* clause should read as **vector-level strict dominance** (a
   budgeted baseline strictly dominating adaptive across the divergence vector), consistent with §8.4's
   vector comparison, rather than "in a summary under its declared direction".
3. **Appendix E revision binding.** The verification record above names the base `a71dd2c` and repair
   `aebe4ef`; it should also name the **final reviewed tip `ed27c85`**.
