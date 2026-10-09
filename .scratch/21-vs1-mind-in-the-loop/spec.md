# VS1 — Mind in the loop (design, ticket 21)

Status: design frozen; implementation complete, gated and merged. VS1's eleven frozen exit criteria
(EC-1..EC-11) all PASS in the hermetic evidence, the full `make check` passed at the gated revision
`ebe5fd1`, and the work landed on master as merge commit `586757a`; the real-model arm remains
blocked on spend authorisation (no real model call has been made). This file is the single source of
truth for VS1. It is edited in place when the design changes; it is never a running log.

Authority: the product decisions fixed by the operator in `docs/VERTICAL-SLICES.md` (2026-09-29) and
the founding documents (`docs/VISION.md`, `docs/PRINCIPLES.md`, `docs/ARCHITECTURE.md`,
`docs/P0-SPEC.md`, `docs/adr/0001`–`0005`). Where this file and any other document disagree, this
file is the VS1 contract and the disagreement is listed in Appendix D for the separate reconciliation
pass.

Companion decision record: `docs/adr/0006-cognitive-layer-recorded-model-calls.md`.

## 1. Issue

`docs/VERTICAL-SLICES.md` makes M1 = VS1 the first version that is not an LLM-free simulator. An
independent adversarial audit (`.scratch/audit-vision-fidelity.md`) confirmed the triggering facts:
the repository contains no model call path at all, `TokenCost` is hard-coded to `0` in every
reporter, `WakeStrategyInvalidation` has never had a producer, and ADR 0002/0004 promise a strategy
mutation and rollback end state that has no code. VS1 must put a real model into the world's causal
chain under two roles (Role A: the mind of an actor at a declared decision seam; Role B: the in-run
designer of behaviour logic) with recorded calls, exact replay, cost accounting, and an auditable,
revertible modification record.

This ticket produces the contract and the ADR. Implementation is the follow-on lanes of §14.

## 2. Scope, non-goals, and the supersession of P0's language-model exclusion

### 2.1 In scope

- One **declared Role A decision seam**: the claim-instant action of an actor, decided by a real
  model call under the observation boundary (§4).
- One **Role B mechanism**: in-run proposal, validation, sandbox counterfactual, application,
  measurement, and rollback of a **versioned strategy patch** expressed as data (§5, §10).
- A **model-call protocol** through the project's LiteLLM gateway for `zai/glm-5.3-flash`, specified
  as an interface; the implementation is a later lane (§6).
- A **recorded-call ledger** with request hashes, response bytes, model identity, usage, cost,
  sampling parameters, and latency; replay reads the ledger and never calls a model (§7).
- **Cost accounting and an instrumented, fail-loud cognitive budget ceiling** under the P15
  discipline (§8).
- An **intervention ledger (C8)** with cause, cost, patch, and measured result, sufficient for
  external verification without trusting any agent (§9).
- The **VS1 experiment arms**, the declared cells, and the **scaffold of the frozen fidelity metric**
  that VS2 gates on (§11).
- **Frozen exit criteria** (§12), the proof method and evidence plan (§13), the lane board (§14), and
  risks and stop conditions (§15).

### 2.2 Non-goals (explicitly deferred, with the slice that owns them)

- **Value-based cognitive allocation** (`expected civilizational value / expected cost`), escalation
  requests, and protected exploration: VS2. VS1's C7 instance is deliberately the honest baseline
  that VS2 must beat (§11.1).
- **Sustained evolution** (strategy populations, staged rollout, generations of selection): VS3. VS1
  implements exactly one measured variation-and-selection round per proposal.
- **Scale and heterogeneity, space, mobility, weather, uncertainty, conflict**: VS4. VS1 does not
  widen the world; it exercises cognition on the measured natural-limits substrate (tickets 15–20).
- **Institutions, trade networks, culture, diffusion**: VS5. VS1 keeps the ticket-20 wage contract
  as the unchanged proto-institution.
- **Historical calibration, conditional branches, sensitivity analysis**: VS6. VS1 stays
  model-conditional and makes no historical claim.
- **Model-emitted code, arbitrary strategy structure, or model-written rules**: permanently out of
  scope; see ADR 0006 and §5.2.
- **Isolated batched management** (P8: one call serving several actors): deferred; VS1 calls the model
  once per escalated seam decision so the actor observation boundary is trivially isolated.
- **A real-world policy claim**: never in scope (P14).

### 2.3 Supersession of P0's exclusion

`docs/ROADMAP.md` lists **"language-model strategy editing"** in the P0 phase's *Exclude* list, and
`docs/P0-SPEC.md` §2 excludes **language-model calls**. Those exclusions were the correct contract for
the P0 phase and remain an accurate description of the recorded P0 evidence. **VS1 supersedes them:
from M1 onward language-model calls and in-run strategy editing are in scope and required.** The
ROADMAP's other P0 exclusions (general institution emergence, detailed physical simulation, global
history, distributed execution) remain in force. Retiring the exclusion text in `docs/ROADMAP.md`
and correcting `README.md`'s present-tense claim that language models "wake selectively" is audit
corrective action 3, a separate later pass; it is **not** performed here.

## 3. Concept coverage (C1–C16) in VS1

Every row of the roadmap's concept inventory must be present and runnable at the slice boundary. The
following table is the VS1 coverage claim and its evidence pointer. "Present" never means "deepened":
later slices raise the ceiling of a row without creating it.

| # | Concept | Present and runnable in VS1 as | Evidence pointer |
|---|---------|-------------------------------|------------------|
| C1 | Event-driven world time | Unchanged: the deterministic kernel advances simulated time; every state change is an authorized, hash-chained event (§4.4 reuses the existing claim instant; no new phase instant) | `internal/kernel`, `internal/scheduler`; the run's accepted history |
| C2 | Actors with needs and stakes | Unchanged: explicit actor fibers, survival, death, births, heredity | recorded natural lane |
| C3 | Resources and economy | Unchanged: production, consumption, storage, spoilage, transfer, conservation-checked ledgers | recorded natural lane, G1–G3 gates |
| C4 | Strategy as versioned data | Extended: the frozen `natural/*` refs plus the **patched generation format** of §5.2 (typed parameter vectors, byte-pinned, fingerprinted, parent-linked) | §5.2, §10.1, Appendix C |
| C5 | Observation boundary | Asserted and now consumed by a real decider: every Role A request is built from the frozen `strategy.NaturalObservation` value only, by an allow-list encoder | §4.5, §6.2, EC-6 |
| C6 | Language-model intervention | New: a real `zai/glm-5.3-flash` call at the declared seam changes what an actor does, with the prompt/request and the response bytes recorded | §4, §6, §7 |
| C7 | Cognitive allocation | New: a declared `WakePolicy` object; the VS1 instance is the honest uniform/periodic baseline (all actors in selected hours) | §11.1 |
| C8 | Intervention ledger | New: one append-only, hash-chained record per intervention with cause/cost/patch/result | §9, EC-4 |
| C9 | Strategy self-modification | New: the system emits a new strategy version with a parent ref, validated before use | §5.2, §5.3 |
| C10 | Evolution and selection | One measured round: sandbox counterfactual → apply or roll back, with lineage retained | §5.3, §10.2, EC-5 |
| C11 | Budget and cost accounting | New: calls/tokens/cost per call, per actor-hour, per run, under an instrumented fail-loud ceiling | §8, EC-7 |
| C12 | Events, replay, determinism | Extended: model outputs replay from the ledger, never re-called; the no-AI path stays byte-identical | §7, EC-1, EC-3 |
| C13 | Metrics and paired experiments | New arms A0/A1/A1i/A2/A3 at declared cells; the frozen fidelity-vector scaffold is emitted | §11, EC-2 |
| C14 | Scale and heterogeneity | Present in its existing measured form: the P=32 / 256-founder substrate with its recorded instrumented ceiling halt, plus one declared scale/ceiling probe; scale is not raised (VS4 owns it) | §11.4 |
| C15 | Institutions and society | Unchanged: the ticket-20 wage contract (rules 516–519) as the proto-institution under `natural/9` | ticket-20 evidence |
| C16 | History and calibration | Model-conditional labelling only: every report stays `synthetic/model-conditional`; no historical calibration claim | §12 EC-11 |

## 4. Role A — the declared decision seam

### 4.1 The decision, the instant, the actors

- **Decision**: the actor's **claim-instant action** — exactly the choice the frozen `natural/*`
  cascade makes at the claim phase: `gather`, `eat-stored`, `provision`, `wait`.
- **Instant**: `NaturalPhaseClaim`, i.e. `h·3600s + 1µs` of simulated hour `h` (the frozen lattice of
  `internal/world/natural_contract.go`; VS1 adds **no new phase instant**).
- **Actors**: every living roster actor woken at that instant (`WakeNeedThreshold`, or `WakeBirth` for
  a newborn's reserved first claim) with an empty bag — exactly the existing claim topology — **and
  selected by the declared wake policy** (§11.1). In the VS1 baseline instance the selection is all
  such actors in the selected hours; in no other hour is any decision escalated.
- **One decision per selected actor-hour.** A selected actor's claim decision is made by the model;
  every other decision instant of that hour (bag decision, build, meal, denial fallback, birth,
  labour) remains the frozen policy's.

### 4.2 Why reasoning could matter at this seam

1. The claim decision is a genuine intertemporal allocation under uncertainty: the frozen cascade is
   a memoryless threshold rule that gathers whenever a stocked home slot is visible, otherwise draws
   the bank at a survival floor or converts banked stock into build material. It cannot represent that
   the eight home slots are contested by up to eight neighbours at the gather completion 20 minutes
   later, that a granary unit spoils on a 24-hour clock, that capital `k ≥ 2` is the birth gate, or
   that provisioning a granary unit buys an investment unit at zero hours while self-provisioning
   spends one.
2. The seam has **measured decision sensitivity on the recorded substrate**. Ticket 19's phase map
   (`.tmp/natural9-evaluation.txt`) shows the same instant's rule differing between the provision arm
   `natural/7` and the banking-only arm `natural/7b`, flipping the A2 surplus indicator from **2/8 to
   7/8 seeds at f=12** (the fragile density) and reversing the ordering at f=13. Decision quality at
   this instant is therefore not cosmetic on the recorded world.
3. The choice is causally consequential downstream (capital formation, granary state, birth gate,
   survival), and it is made from an observation the actor already possesses (§4.5), so the model
   would be reasoning *under* the boundary rather than around it.

Honest counter-consideration, stated so it is not discovered later: at the reliable cells (f=10, f=11)
the frozen policy already reaches the surplus indicator in 8/8 seeds, so the *headroom for
improvement* is small there; f=12 is declared the primary sensitivity cell and f=10/11 are declared
"no-harm at reliability" cells. VS1 gates on **load-bearing divergence**, not on improvement; whether
divergence is *better* is VS2's question.

### 4.3 The legal action set and the validation ladder

The escalated action set is a property of the **seam and the world mechanism**, not of the bound
behaviour ref: on the VS1 mechanism (v6, rule 515 admitted) it is
`{gather, eat-stored, provision, wait}`. `provision` is included deliberately: confining the model
to the fallback ref's own vocabulary would prejudge the experiment. The set is derived from the
world's own admission rather than assumed: on a mechanism whose provision contract does not admit the
action, a model `provision` is a typed level-2 world refusal, never a silent substitution. The model never mutates the world: it
names an action, and the *existing* world paths validate and commit it (P4).

Three validation levels, each with a distinct recorded outcome:

| Level | Checked by | Failure outcome |
|---|---|---|
| 1 — schema and admissibility | the cognition layer, from the observation alone | `malformed`/`inadmissible`: the declared **fallback** (the frozen cascade's own choice at that instant) decides; recorded with the typed reason; counted in the decision-error rate; zero mutation |
| 2 — world preconditions | the existing runner and contracts | `refused`: the world's existing typed refusal/denial path runs unchanged (gather allocator denial → the frozen denial loop; provision precondition → the typed `provision-refused` attempt; no silent substitution); recorded; counted in the decision-error rate; zero mutation |
| 3 — transport/infrastructure | the model client | retry per §6.5; on exhaustion a **fail-loud typed halt** with the ledger persisted and the world state untouched |

Level 1 checks only what the observation makes impossible: `gather` requires at least one stocked
home slot in the observation; `eat-stored` requires `granary_stock ≥ 1` and
`last_stored_meal_hour < hour`; `provision` requires the actor's bag to be empty,
`granary_stock ≥ 1`, and `last_stored_meal_hour < hour`; `wait` is always admissible. It never
applies the frozen policy's own energy thresholds — overriding those is the point of the seam — and
preconditions that are *not* visible in the observation (such as the provisioned horizon envelope)
stay level-2 world refusals. The model never names a slot: the
coordinator derives the claim's target slot from the same observation (lowest-index stocked home
slot, the frozen rule) and records the derivation; the gather allocator then performs its existing
deterministic admission and rotation.

### 4.4 Where the seam lives in the runner

The gather admission for a claim is computed *before* the batch, in the coordinator
(`Natural.allocate`), which today evaluates the frozen cascade for every claim-wake actor. The seam is
therefore integrated in two phases, with **exactly one model call per escalated claim decision**:

1. **Allocation phase** (coordinator, before the batch, on the batch's common snapshot): for each
   claim-wake actor selected by the wake policy, build the observation, call the model once, record
   the call, validate, and use the resulting action in place of the frozen cascade's choice for the
   claim decision.
2. **Evaluation phase** (per-actor batch step): the claim case consumes the decision recorded in
   phase 1 from runner state; it does not call the model again and does not re-derive the decision.

Both phases read the same world version, so the request hash covers exactly the observation the
frozen policy would have seen. The batch's existing deterministic flow (admissions, proposals,
conflicts, commit) is unchanged.

### 4.5 Observation-boundary input

The request is built **only** from the frozen `strategy.NaturalObservation` value, which carries the
actor's own energy, hunger, bag units, capital, work-in-progress, invested units, last build hour,
granary stock, last stored meal hour, home patch, the eight home-slot stocks, and the denial guard.
The frozen type has no field for tenure mode or owner, debts, spoilage counters, allocator state,
regeneration budget `R`, seeds, other actors' private state, or kin. The request also carries the
actor's **own bound strategy parameter vector** (its own behaviour logic, part of its own state under
P3) so the model can override deliberately, and the declared metadata of §6.2. It carries nothing
else; the allow-list encoder of §6.2 makes that checkable rather than asserted, and the import-graph
test of EC-6 makes it structural.

## 5. Role B — in-run behaviour-logic modification

Role B is the *designer* role: a model reads trajectory evidence and proposes a new versioned
behaviour strategy. It never emits code and never touches the world.

### 5.1 Trigger points (what evidence, at what moment, causes a kick-in)

The modification policy is a declared object, part of run identity. The VS1 instance:

- **Moment**: at each declared **audit hour** `h ∈ {24, 48, 72, 96, 120, 144, 168}` — a pulse boundary,
  i.e. a quiescent checkpoint boundary, so no running activity changes semantics (ADR 0002). The live
  loop audits `{24,48,72,96,120,144}`; `h168` is the terminal horizon, where a 24-hour sandbox fork
  would exceed the frozen envelope, so it is the measurement boundary rather than a live audit
  (§14.1 reduction 3).
- **Evidence**: the runner assembles a **trajectory evidence packet** for the trailing 24-hour window
  from its own committed hourly checkpoints (the same rows the reporter already publishes):
  `alive(h)`, cumulative births and deaths by cause, per-actor capital and granary holdings, per-patch
  mouths and `Σk`, committed build/meal counts, and the running maxima. No actor-private state beyond
  these published aggregate/state rows enters the packet (§6.2).
- **Trigger predicate** (any one holds ⇒ solicit exactly one proposal):
  - T1 `no birth committed in the trailing 24 h`; or
  - T2 `no committed build in the trailing 24 h`; or
  - T3 `max per-actor capital at h < 2` (the birth gate's capital margin unreachable for the roster).
  If none holds, the audit hour records a zero-cost `no-trigger-audit` intervention. The readings that
  motivated T1–T3 are the recorded binding problems of the v5–v7 arcs (M2/M3: the capital climb and
  the birth channel are the unreachable ones).
- **At most one proposal per audit hour**, chained on the current head ref; every proposal, admitted
  or not, is recorded.

This is the in-run kick-in required by the roadmap's cross-cutting invariant 8: the cause is evidence
available during the run at a declared moment, never an offline authoring step. VS3 replaces this
declared predicate with measured selection pressure.

### 5.2 The patch format — versioned strategy data, never code

A patch is a **new versioned strategy** in the `natural/*` family per ADR 0002:

- **New ref**: `{id: "natural-cog", version: parent.version + 1}` — a **distinct ref id and format
  space** outside the frozen `natural/5..9` refs and formats, so no patched ref can ever collide with
  a frozen one; strictly monotonic; versions are never inferred, and a patch can never name a version
  that is not exactly `parent.version + 1`.
- **Parent ref**: exact, must equal the current head ref (no implicit inheritance).
- **Payload**: a bounded vector of **typed integer deltas** over the declared parameter contract of
  Appendix C. Each parameter has an id, a meaning, the decision point it modulates, the frozen parent
  value, and a domain. A patch may not touch world-side contract constants, add or remove rules,
  change the mechanism version, alter the observation boundary, change the policy budget, or read
  state the fallback cascade cannot read.
- The patched generation is the declared parent cascade with the parameter vector substituted at the
  declared decision points; **the all-parent-values vector reproduces the parent's decisions exactly**
  (a pinned reduction test), and legs keep their declared order.
- Content is canonical, byte-pinned, and fingerprinted with the existing `natural` codec discipline
  (SHA-256 over the canonical wire); the codec, not the model, owns the bytes.
- **Proposer intent** is recorded as free text and is never terminal for any decision (P6).

### 5.3 Lifecycle: propose → validate → sandbox → apply → measure → rollback

1. **Propose**: one model call (`role: "strategy-patch"`) with the evidence packet and the parameter
   contract; the response is a patch proposal or an explicit `propose: false`.
2. **Validate** (deterministic, before any world effect): decode; parent-ref equality; every parameter
   id in the contract; every value inside its domain; at least one delta; no forbidden target. A
   failure is a typed rejection, zero mutation, recorded. This is the path a **deliberately broken
   patch** takes (§12 EC-5b).
3. **Sandbox** (minimum honest counterfactual for VS1): fork the run's own committed checkpoint at the
   audit hour through the world package's restore path, register and bind the candidate generation,
   run the fork deterministically to the horizon **with the seam disabled** (no model calls), and run
   the unpatched parent continuation from the same fork. Admit the patch iff the patched branch
   strictly improves the trigger indicator that fired and no declared gating band or invariant
   regresses; otherwise reject and record the sandbox verdict. The sandbox evaluates the *strategy*
   change only, on deterministic compute — an honest, cheap, single-variable comparison; it is not a
   prediction of the model's own future calls.
4. **Apply**: at the same quiescent boundary, register the generation and rebind the whole living
   roster to the new head ref atomically. A newborn after the boundary takes the frozen fallback
   until the next audit-hour rebind (a declared VS1 reduction: inheritance at birth is not
   implemented). Record the affected actor set.
5. **Measure**: at the next audit hour, record the live window's declared indicator deltas for the
   applied generation (model-conditioned, labelled).
6. **Rollback** (mandatory path, not optional): at that same audit hour, re-run the sandbox on the
   *current* head versus its parent; if the head no longer improves the trigger indicator, rebind the
   roster back to the parent ref, retain the patched generation and its lineage in the registry as
   immutable history, and record the rollback with its cause. A rollback never deletes evidence.

### 5.4 Effectiveness in the running world, durability, restart

The binding is runner state (the strategy section already carries policy content and per-actor
bindings; strategies are not kernel components). On apply the run's **manifest cognition block**
(§7.2) is extended — for cognitive runs only — with (a) the canonical wire and fingerprint of every
registered cognitive generation, (b) the per-actor binding table, and (c) the lineage chain of
proposal/validation/apply digests. A restart from a checkpoint at or after the apply boundary
restores the generation and its bindings, and the next claim decision is made under the new
generation (ADR 0002's rule): the runner's frozen base ref is not changed by a rebind — it remains the
journal, genome, manifest policy-format-slot, and strategy-section identity, and the head generation
rides the cognition block and the runner's cognitive state. A restart from an earlier checkpoint
restores the parent generation; that is correct causality, and the ledger records which boundary took
effect. The manifest binds the cognition block of §7.2 so a mismatched or missing ledger refuses the
restore. The apply binds the living roster at the boundary; a newborn's first claim before the next
rebind takes the frozen fallback (a declared VS1 reduction: the apply binds the roster present at the
boundary, and later newborns are re-bound only at the next audit-hour rebind). Journal replay keys the
active generation on one `ApplyHour` per bundle, so the implemented path restores the generation that
took effect at that boundary; an apply following a mid-run rollback within the same run is not yet
replayed from a single bundle (a second declared VS1 reduction, closed by the verification lane's
full-ledger replay).

### 5.5 Lineage and attribution record

Every patch records: proposer (model id + call id, or reserved `harness`), parent ref and fingerprint,
child ref and fingerprint, the intent string, the delta vector, the validator identity and verdict,
the sandbox branch digest and window, the apply boundary and affected actor digest, the measured
window, and the rollback state. This is the record ADR 0002 requires (parent, proposer, reason,
validation result, rollout state, affected actors), and it is what an outside party verifies in §9.3.

## 6. The model-call protocol

### 6.1 Gateway and interface (specified, not implemented)

- **Model**: `zai/glm-5.3-flash`, reached through the project's **LiteLLM gateway** over an
  OpenAI-compatible `POST {base}/v1/chat/completions`.
- **Interface** (the world never sees the client implementation; direction of dependency in §6.6):

```go
// This interface is defined where it is consumed and implemented by the
// cognition client; only internal/sim and internal/strategy types appear.
type SeamDecider interface {
    DecideActorSeam(ctx context.Context, req ActorSeamRequest) (ActorSeamDecision, error)
    ProposeStrategyPatch(ctx context.Context, req PatchRequest) (PatchProposal, error)
}
```

- **Configuration** (environment, never recorded): `AGENTWORLD_MODEL_GATEWAY` (base URL),
  credential from the environment. The ledger records the endpoint **host+path** (no query, no
  credential) and `credential_present: true`; **no secret material is ever recorded**.

### 6.2 Request schema and observation-boundary construction

The **canonical request** is a JSON object encoded by an explicit allow-list encoder (each field
written by name; no reflection over structs, no map iteration, no world reader in scope):

```json
{
  "schema_version": 1,
  "role": "actor-decision",
  "seam": "natural.claim",
  "prompt_template": "natural-claim/v1",
  "model": "zai/glm-5.3-flash",
  "run": { "fixture": {"patches": 2, "r": 8, "founders": 12, "mechanism": 6, "births": true},
            "seed": 0, "base_ref": {"id": "natural", "version": 7},
            "head_ref": {"id": "natural", "version": 7},
            "identity_digest": "<sha256 of the run identity block>" },
  "actor": { "id": 5, "hour": 6 },
  "observation": {
    "energy": 6, "hunger": 2, "bag_units": 0, "capital": 1, "wip": 0,
    "invested_units": 1, "last_build_hour": 4, "granary_stock": 3,
    "last_stored_meal_hour": 2, "home_patch": 1001,
    "home_slots": [{"slot": 2001, "stock": 1}, {"slot": 2002, "stock": 0}],
    "last_denial": "none"
  },
  "strategy": { "parameters": {"claim.provision_energy_floor": 4, "...": 0} },
  "actions": ["gather", "eat-stored", "provision", "wait"],
  "sampling": {"temperature": 0.7, "top_p": 1.0, "max_tokens": 128, "seed": null}
}
```

The gateway prompt is produced from this canonical request by the versioned template
`natural-claim/v1` (system message = the fixed role/task/action-vocabulary text; user message = the
canonical request). The template version is part of run identity. The **request hash** is
`sha256(canonical request bytes)`; the request bytes are recorded verbatim.

**Role B request**: same envelope with `role: "strategy-patch"`, the evidence packet of §5.1 in place
of `observation`, the parameter contract of Appendix C, and the current head ref. The evidence packet
contains only the declared world-level aggregate/state fields listed in §5.1 — **no actor's private
state** (P8 cross-case isolation). The two roles have separate schemas and separate validation.

### 6.3 Response schema

`actor-decision` content (a JSON object):

```json
{"schema_version": 1, "action": "gather" | "eat-stored" | "provision" | "wait",
 "rationale": "<≤512 chars, recorded only>", "confidence": 0.0-1.0 }
```

`strategy-patch` content:

```json
{"schema_version": 1, "propose": true,
 "parent_ref": {"id": "natural", "version": 7},
 "deltas": [{"param": "claim.provision_energy_floor", "value": 3}],
 "intent": "<≤512 chars>",
 "prediction": {"indicator": "capital_peak", "direction": "increase"}}
```

`rationale`, `intent`, `confidence`, and `prediction` are **recorded and never terminal** (P6). The
full HTTP response body is recorded verbatim (bounded at 64 KiB); an oversized body is recorded by
digest and length as `oversized` and takes the fallback. `usage.prompt_tokens` and
`usage.completion_tokens` are recorded per attempt.

### 6.4 Sampling parameters

VS1 instance, part of run identity and recorded per call: `temperature 0.7`, `top_p 1.0`,
`max_tokens 128`, `seed` absent (sent only if the gateway supports it; recorded as sent or absent
either way), `response_format {"type": "json_object"}` requested where the gateway accepts it, with
the declared fallback of a plain completion plus local JSON extraction (recorded per call). A
temperature above zero is deliberate and permitted: the ledger captures whatever the model returns,
so the run is reproducible from its recorded events even though the model is not reproducible (§7).

### 6.5 Retry, timeout, failure handling

- Declared harness parameters (run identity): per-attempt timeout **30 s**, per-call deadline
  **90 s**, retry attempts **3**, retry only on transport errors, 429, and 5xx, with a bounded
  exponential backoff. Retrying a schema-invalid completion is forbidden: that is model behaviour,
  not infrastructure.
- **Attempts are recorded**: every physical attempt is a call record; the first attempt that yields a
  well-formed completion is the *used* attempt and is what replay returns.
- On exhaustion: a typed `ModelUnavailable` halt. The ledger record is persisted, the world state is
  untouched (the call happens at a decision instant before any proposal is staged), and the halt is a
  first-class recorded outcome, exactly like a compute-ceiling hit.
- Malformed, inadmissible, or world-refused responses never corrupt the world: the validation ladder
  of §4.3 guarantees zero mutation on every failure path, and the fallback/refusal is recorded with
  its typed reason and cost.

### 6.6 Privacy proof obligations (EC-6)

1. **Structural**: the request-builder package must not transitively import `internal/world`; the
   world package must not import the model client (the seam is an interface with `sim`/`strategy`
   types only, wired by `cmd/`). Both directions are asserted by an import-graph test.
2. **Enumerated**: the canonical request is produced by an allow-list encoder; no field can be added
   without editing the encoder and the schema version.
3. **Adversarial**: a test constructs a world whose privileged fields hold unique sentinel values
   (tenure owner/mode, spoil debt, allocator state, `R`, seed, other actors' energy/hunger/capital/
   granary, kin) and asserts that no canonical request byte contains any sentinel, for every seam
   instance in a driven run.
4. **Equivalence**: the observation reconstructed for a request is byte-identical to the observation
   the frozen cascade consumes at the same instant on the same version.

## 7. Determinism, the recorded-call ledger, and replay

### 7.1 The recorded-call ledger

One append-only, per-lane ledger file (JSON records, one per line, canonical encoding), each record
carrying `prev_hash`/`hash` over its canonical bytes. It is flushed to disk **before** a response is
consumed, so a used call can never be missing from the record. Record fields (Appendix B): sequence,
role, request hash and request bytes, full response body bytes, extracted content, model id, endpoint
host+path, sampling parameters as sent, usage, computed cost and price-table version, attempt index,
status, wall-clock start and latency (non-causal), and the hash chain.

### 7.2 Checkpoint binding

The checkpoint manifest gains an **omitempty trailing `cognition` block**, present only for lanes with
a non-nil seam: ledger file digest and record count at the boundary, wake-policy identity and
fingerprint, base/head ref fingerprints and lineage head, cognitive ceiling thresholds, the
model/sampling identity digest, the canonical wire and fingerprint of every registered generation,
the per-actor binding table, and the lineage chain. Decode consumes the block exactly as the existing omitempty
`founders` field is consumed, so **every recorded non-cognitive manifest keeps its bytes**. Restore
refuses a missing, short, or mismatched ledger.

### 7.3 Replay rule

Replay of a recorded lane reads the ledger and **never calls a model**:

- Every escalated seam decision re-presents its exact request hash; a hit returns the recorded used
  attempt's response bytes, which are then re-validated deterministically. A miss (`ErrLedgerMiss`),
  a request-hash mismatch, or a hash-chain break is a hard error, never a live call.
- The world's reference replay (`ExportJournalWithReplay`, `RestoreNaturalCheckpoint`) wires the
  recorded Role A seam through `CognitiveReplaySeam`/`CognitiveReplayWake`: each escalated claim
  instant is resolved from the ledger by request hash and never calls a model. The seam is injected
  as a `world.ClaimSeam` (internal/cognition's `ReplaySeam`) so `internal/world` imports no client and
  no ledger. A seam-active bundle whose replay seam is absent fails closed, so deleting the
  ledger-resolution path is a restore failure, not a silent frozen re-run. This is what makes a
  seam-active run's `ExportJournalWithReplay` reconciliation and its checkpoint restore exact.
- The replay client is incapable of network access (a test-only sentinel client that fails on any
  request is wired during replay), and the replay asserts **zero gateway requests**.
- The replay reproduces the accepted event chain, the hourly projections, the intervention ledger,
  and the call ledger byte-for-byte. Latency and wall-clock fields are excluded from every causal
  digest because they are not causal state.
- `temperature > 0` is captured, not wished away: the model is a nondeterministic component, and the
  ledger makes its outputs recorded events. A fresh run at the same seed and no ledger may differ;
  that run's own ledger is again replayable. Results are labelled model-conditional (C16/EC-11).

## 8. Cost accounting and the cognitive ceiling (C11/P15)

### 8.1 Budget object

```go
type CognitiveBudget struct {
    MaxCalls            uint64 // our authorised spend, not a modelling device
    MaxPromptTokens     uint64
    MaxCompletionTokens uint64
    MaxCostMicros       int64  // price-table-derived, versioned
}
```

The ceiling value comes from **our compute and authorised spend**, is declared in run identity, and
is justified by the pre-run projection of §8.3 — never chosen to make a mechanism viable (P15).

### 8.2 Instrumentation and fail-loud

`calls`, `prompt_tokens`, `completion_tokens`, and `cost_micros` are added to the existing ceiling
ledger as rows `cognitive-calls`, `cognitive-prompt-tokens`, `cognitive-completion-tokens`,
`cognitive-cost` — observed hourly, with trailing rate and projected binding hour, exactly like the
existing ceilings. A crossing produces a typed fail-loud halt with the full ledger record persisted,
and the report states the binding ceiling, its value, the hour, and the P15 verdict verbatim
(harness outgrown vs design defect). The ceiling never truncates a trajectory and never silently
substitutes a cheap decision for a model call.

### 8.3 Reporting units

Per call: attempts, tokens, cost, latency. Per actor-hour: `calls / Σ_h alive(h)`, likewise tokens and
cost, with `Σ_h alive(h)` computed from the hourly checkpoints. Per run: totals, ceiling rows, and the
pre-run projection. The no-AI arm keeps `token_cost = 0` and its exact key set; AI arms replace the
hard-coded zero with the recorded totals additively.

### 8.4 P15 classification of every number this design declares

| Declared number | Classification |
|---|---|
| `MaxCalls` / token / cost ceilings | our compute and authorised spend; instrumented, fail-loud, reported; never a modelling device |
| retry attempts (3), per-attempt timeout (30 s), per-call deadline (90 s) | harness liveness parameters; recorded in run identity; P15 permits experiment bounds |
| wake-policy period (6 h) and audit period (24 h) | allocation-policy and modification-policy parameters — the very objects VS2/VS3 test |
| trigger thresholds (24 h window, `capital < 2`) | declared trajectory-evidence thresholds of the modification policy; no world mechanism depends on them |
| 12–13 experiment cells, 168-hour horizon | declared experiment bounds (P15 explicitly permits a frozen gate to bound an experiment) |
| no population, lifetime, capital, or institution cap | none is introduced; the world's modelled constraints are unchanged |

## 9. The intervention ledger (C8)

### 9.1 Record shape

One hash-chained record per intervention (Appendix B): identity, `kind` (`role-a`, `patch-propose`,
`patch-validate`, `patch-apply`, `patch-measure`, `patch-rollback`, `no-trigger-audit`),
`cause` (wake policy + seam + actor + hour for Role A; trigger predicate + evidence-packet digest +
audit hour for Role B), `cost` (call refs and the aggregated tokens/cost/engine time), `patch` (parent
and child refs, delta vector, intent, patch digest, affected-actor digest), `validation` (verdict,
typed reason, sandbox branch digest and window), and `result` (measured window and indicator deltas,
or a typed `pending` with the declared window that the run resolves).

### 9.2 Completeness

Every intervention has cause, cost, patch-or-role, and result; every call record is referenced by
exactly one intervention; there are no orphan calls (EC-4). A `result` that cannot be measured
(e.g. the run halts first) is recorded as `unavailable` with its typed reason, never fabricated.

### 9.3 What an outside party needs to verify a modification without trusting any agent

Given the run directory, an external party can:

1. Read the run identity block and recompute the identity digest.
2. Verify both ledgers' hash chains and that the manifest's cognition block matches the ledger heads.
3. Recompose every request from the recorded request bytes and check `sha256(request) == request_hash`.
4. Re-run the deterministic world from the genesis (or a checkpoint) with replay-only mode and
   confirm the accepted event chain, hourly projections, escalated decisions, and applied strategy
   versions reproduce byte-for-byte with **zero model calls**.
5. Verify a patch from data alone: parent ref and fingerprint, child wire, the reduction property
   (all-parent-values ⇒ parent decisions), the parameter domains, and the lineage chain to the root
   strategy.
6. Verify the measured result: recompute the declared indicator window from the committed events and
   projections rather than from any summary text.

Honest limits, stated verbatim because they bound the claim: a digest chain detects damage, not a
wholesale rewrite by an adversary with write access to every artifact; the model's outputs are opaque
(verification establishes *what was recorded and applied*, not that the output was "good"); and the
world's laws are trusted as declared source code, not re-proved.

## 10. Patch validation and the sandbox

### 10.1 Admissibility

A patch is admissible iff: it decodes against the versioned schema; `parent_ref` equals the current
head ref; the child version is exactly `parent.version + 1`; every delta names a contract parameter
(Appendix C) and lies inside its declared domain; the delta set is non-empty; the canonical wire
fingerprints cleanly; and the all-parent-values reduction reproduces the parent's decisions on the
existing decision-grid differential test. Broken, forged, out-of-domain, unknown-parameter, and
wrong-parent patches are rejected here with typed reasons and **zero world mutation**.

### 10.2 Minimum honest sandbox / counterfactual

The sandbox is a deterministic fork pair from the run's own committed checkpoint at the audit hour:
(patched head) versus (unpatched parent), both run with the seam disabled, same seed, same mechanism,
same horizon window. The checkpoint is a seam-active bundle (the run's own), so the fork restores it
through the recorded-seam replay of §7.3: the past escalated decisions are reproduced from the ledger
with no model call, and the fork's own continuation is seam-disabled. Admit iff the patched branch
strictly improves the trigger indicator that fired
and no declared gating band or invariant regresses. The sandbox is not a model-conditioned
prediction and does not depend on the model; its cost is deterministic engine compute, instrumented
in the ceiling ledger like any other compute.

### 10.3 Mandatory rollback

The rollback path is mandatory and exercised both by the declared fixtures (EC-5) and by the live
loop (§5.3 step 6): rebinding the affected roster to the parent ref, retaining the patched generation
and its lineage immutably, and recording the rollback with its cause. A rollback that leaves a child
ref bound, or that deletes evidence, is a stop condition.

## 11. VS1 experiment arms, cells, and the fidelity-metric scaffold

### 11.1 The C7 wake-policy object and its VS1 instance

```go
type WakePolicy struct {          // declared object, part of run identity
    Name        string            // "periodic" (VS1 instance)
    PeriodHours int               // 6
    PhaseHours  int               // 0
}
```

Escalation set: every claim decision at hours `h ≡ phase (mod period)`; all actors in those hours
(uniform over actors, periodic in time). The set is a pure function of `(policy, hour, roster)` and
consumes **no world random stream**, so the frozen substrate's RNG positions are untouched. The
`full` instance (`period = 1`) is the high-budget reference. VS2 replaces this instance with
value-based allocation; VS1's instance is deliberately the honest baseline the central claim must
beat, and because it is uniform it structurally excludes no actor.

### 11.2 Arms

| Arm | Seam | Wake policy | Role B | Purpose | Calls (approx.) |
|---|---|---|---|---|---|
| A0 no-AI | frozen cascade only | — | off | control; the recorded ticket-19 substrate re-verified byte-identical | 0 |
| A1 role-a | model | `periodic(6)` | off | load-bearing test (EC-2); low-budget arm | ≈300/cell |
| A1i role-a-inert | model called, responses recorded, **actions discarded** | `periodic(6)` | off | proves the divergence is caused by the model's *content*, not by the act of calling (EC-2) | ≈300/cell |
| A2 role-a+role-b | model | `periodic(6)` | on | the full VS1 mechanism (§5, §10) | A1 + ≈7 proposals + sandbox compute |
| A3 reference | model | `full` | on | the high-budget reference run for the fidelity vector (comparative, never ground truth) | ≈1 700–2 000/cell |

### 11.3 Cells

World mechanism **v6** (rule 515 present), base policy **`natural/7`** (the richest claim vocabulary,
and the arm whose recorded instant-rule difference produced the f=12 sensitivity datum), births
enabled, `-r 8`, horizon 168:

- primary/declared cells: `f ∈ {10, 11, 12}` × seeds `{0, 1, 2, 3}` × workers 1 = **12 cells**;
- determinism twin: `f=12, s=0`, workers 4 (digest-identical to its workers-1 twin);
- the A0 evidence for all 12 cells already exists as `.tmp/natural9-n7-f{10,11,12}-s{0..3}-w1.json`
  and is the no-AI arm's recorded substrate; one cell must be re-run through the VS1 harness and
  reproduce it byte-identically (EC-1). If those artifacts are not retained, the no-AI arm is re-run
  through the VS1 harness with a nil seam (the arm is deterministic and cheap), rather than
  abandoning the control.

If the authorised cognitive ceiling cannot fund A3 at all 12 cells, the declared reduction is A3 at
`f=12` × seeds `{0..3}` (the declared sensitivity density) with the remaining reference cells reported
as **not run under the authorised budget** — an honest gap, never a silently cheaper substitute.

### 11.4 The scale/ceiling probe (C14/C11/P15, descriptive)

One declared probe: `P=32` (256 founders), the full founder block, a lowered `MaxCalls`, the A1 arm.
Its required outcome is the typed fail-loud **cognitive** ceiling halt with the ledger intact and
replayable, and the P15 verdict reported verbatim. It is descriptive (not a gating band); it exists
to show the cognitive ceiling is instrumented at scale and that the layer runs on the existing
substrate. It does not raise anyone's scale and makes no claim about the 256-founder world beyond the
recorded ceiling event.

### 11.5 The fidelity metric scaffold (frozen for VS2; emitted, not gated, in VS1)

**No single scalar.** For a budgeted run `B` and the reference run `R` at the same fixture and seed,
the metric is the per-indicator divergence **sequence** over hours plus declared per-indicator
summaries (max, mean, first-divergence hour):

| # | Indicator | Hourly value | Divergence |
|---|---|---|---|
| 1 | population | `alive(h)` | `abs(B−R) / (8P)` |
| 2 | births | cumulative births | the absolute difference over the declared horizon envelope |
| 3 | deaths | cumulative by cause | as (2), per cause |
| 4 | wealth distribution | capital histogram over `k ∈ [0, KMax]` | total-variation distance of the two histograms |
| 5 | wealth distribution | granary-stock histogram over declared buckets | total-variation distance |
| 6 | exchange volume | cumulative wage settlements (0 under `natural/7`; the indicator is present and reported as zero) | as (2) |
| 7 | decision-error rate | cumulative escalated decisions with a malformed/inadmissible/refused outcome, over cumulative escalated decisions | absolute difference of the two ratios |

The vector is the metric; a weighted sum is explicitly not defined. VS2 gates on it (with dominance
over the vector rather than a scalar winner); VS1 emits it and reports the reference's own
model-conditional status.

## 12. Frozen exit criteria

Frozen before any run; a band miss is an outcome reported verbatim and is never tuned. Cells are
§11.3 unless stated.

- **EC-1 mechanism identity / continuity.** The no-AI arm through the VS1 harness is byte-identical to
  the recorded `.tmp/natural9-n7-*` evidence at all 12 cells (accepted-history digests, hourly
  projections, event counts); non-cognitive checkpoint manifests are byte-unchanged (omitempty
  cognition block); the manifest/strategy-section extensions exist only for cognitive lanes.
  *Falsified by* any byte difference.
- **EC-2 load-bearing model, no hidden coupling.** `role-a-inert` ≡ `no-AI` byte-exact at all 12
  cells; at every declared cell the `role-a` arm's trajectory differs from `no-AI` in ≥1 declared
  indicator hour; and the intervention ledger shows ≥1 **applied** model decision per cell whose
  action differs from the frozen cascade's action at the same seam. *Falsified by* inert ≠ no-AI, by a
  cell with zero divergence, or by divergence arising only through fallback/refusal paths.
- **EC-3 byte-exact replay, zero model calls.** Every recorded AI cell replays byte-exact from its
  genesis: same accepted history, hourly projections, escalation decisions, and ledgers, with zero
  gateway requests, and a deliberately mutated ledger byte is detected. *Falsified by* any mismatch or
  any call.
- **EC-4 intervention completeness (C8).** 100 % of intervention records carry cause, cost,
  patch-or-role, and result; every call record is referenced by exactly one intervention; zero orphan
  calls; every unmeasurable result carries a typed reason rather than a fabricated value. *Falsified
  by* any missing field or orphan.
- **EC-5 patch lifecycle (C9/C10).** (a) the declared scripted proposal fixture runs
  propose→validate→sandbox→apply→measure→rollback end to end and replays byte-exact; (b) each
  deliberately broken patch fixture (out-of-domain value, unknown parameter, parent-ref mismatch,
  schema violation, sandbox invariant breach) is rejected at validation with a typed reason and zero
  world mutation; (c) the declared regressing-patch fixture is applied and then rolled back with the
  parent ref restored for every affected actor and the lineage retained; (d) the live A2 cells report
  how many model-proposed patches were admitted, applied, measured, and rolled back — the honest null
  "the model proposed no admissible patch in this cell" is valid and reported verbatim. *Falsified by*
  a broken patch reaching application, a rollback leaving a child ref bound, or a missing
  validation/lineage record.
- **EC-6 privacy boundary.** Every Role A request is produced by the allow-list encoder from the
  frozen `NaturalObservation`; the import-graph test proves the request builder does not transitively
  import `internal/world` and the world does not import the client; the adversarial sentinel test
  finds no privileged marker in any request; no other actor's private state appears; Role B packets
  contain only the declared aggregate fields. *Falsified by* one marker or one structural violation.
- **EC-7 costs reported (C11).** Every AI cell reports per-call usage/cost/latency, per-actor-hour and
  per-run totals, and the four cognitive ceiling rows; a binding ceiling is a typed fail-loud halt
  with the P15 verdict. *Falsified by* a call without cost fields or a ceiling breach recorded as
  anything other than a typed halt.
- **EC-8 determinism and invariance.** The escalation set is a pure function of `(policy, hour,
  roster)` and consumes no world random stream (proved by the RNG-state/position identity of the
  no-model path plus the inert gate); the `f=12, s=0, w=4` twin is digest-identical to its workers-1
  twin. *Falsified by* a stream consumption difference or a twin mismatch.
- **EC-9 coverage and supersession.** Every C1–C16 row is present and runnable with the §3 evidence
  pointer, and the supersession statement of §2.3 is part of the delivered contract. *Falsified by*
  an empty or unreachable row.
- **EC-10 no arbitrary design caps (P15).** The cognitive budget is a declared compute ceiling,
  instrumented and fail-loud, never a world mechanism; the §11.4 probe demonstrates the cognitive
  ceiling firing with the ledger intact and the P15 verdict reported; no world mechanism depends on the
  budget. *Falsified by* a mechanism that needs the budget to appear viable, or a silent ceiling.
- **EC-11 honest labels.** Every report is `synthetic/model-conditional`; `token_cost` is the recorded
  total in AI arms and 0 in no-AI arms; no result is presented as real-world evidence. *Falsified by*
  an unlabelled or overclaimed report.

## 13. Proof method and evidence plan

- **Hermetic by default.** `make check` (and every unit test) must not touch the network: the client
  is exercised against a stub and against recorded fixture ledgers. A live gateway call happens only
  in a declared live run.
- **Declared live smoke** before any matrix: one AI cell at the smallest declared fixture, producing
  a real ledger with a real call, a real cost, and a successful replay — the only evidence that proves
  the gateway contract, the auth path, and the JSON object mode.
- **Matrix**: the §11.3 cells per arm, each run fresh (new checkpoint directory, no clobber), one
  report per cell, evaluator output written to `.tmp/21-*`, evidence quoted verbatim in the ticket's
  later evidence record.
- **Fixtures**: the scripted proposal, broken-patch, and regressing-patch fixtures are recorded ledger
  fixtures (a replay-mode client with a hand-authored ledger), so the mechanism's paths are proven
  deterministically and hermetically.
- **Zero-use gates**: EC-1 and EC-2's inert arm run first; a failure is a stop, not a tuning
  opportunity.
- **Independent review**: two fresh reviewers (code and evidence) after the implementation lanes.
- **Limits disclosed**: model-conditional; digest chains detect damage, not replacement; no real-world
  claim.

## 14. Lane board (one writer per file area; frozen bytes untouched)

| Lane | Scope (sole writer) | Depends on |
|---|---|---|
| L1 client + ledger + replay + budget | `internal/cognition/{client,ledger,replay,budget,codec}.go`; `internal/cognition/replay_seam.go` (the ledger-resolving Role A seam, added by L4 with the replay extension); `internal/cognition/protocol/**` (+ tests) | — |
| L2 seam, wake policy, observation encoder | `internal/cognition/{observation,wake,rolea,intervention}.go` (+ tests); `internal/world/natural_cognition.go`; the claim-phase hunks of `internal/world/natural.go`; `internal/world/natural_cognition_imports_test.go` | L1 |
| L3 patch format + lifecycle + persistence | `internal/strategy/{natural_cognitive.go,natural_cognitive_codec.go}` (and the `natural7.go` extraction of the shared claim/birth cascades into `natural_cognitive.go`); `internal/cognition/{patch,roleb,sandbox}.go` (+ tests); `internal/world/{natural_cognitive_strategy.go,natural_cognitive_checkpoint.go}` (+ tests); the manifest/restore hunks of `internal/world/natural_checkpoint.go`; the cognitive journal-replay hunk of `internal/world/natural_journal.go`; and the state-driven dispatch hunks of `internal/world/natural.go` | L1, L2 |
| L4 experiment runner + arms + metrics + report | `cmd/vs1/**` (new command); reporter sections and bands; the recorded-seam replay/restore wiring of `internal/world/{natural_journal.go,natural_checkpoint.go}` (see §7.3), the `SetCognition` seam setter in `internal/world/natural_cognitive_strategy.go`, and `internal/world/natural_cognitive_replay_test.go` | L1–L3 |
| L5 docs | this file's evidence record, the ADR, and (separate later pass) the reconciliation edits | L4 |
| L6 review | two fresh reviewers, no shared context with the workers | L4 |

No lane may touch `internal/kernel`, `internal/scheduler`, `internal/component`, `internal/sim`, the
frozen `natural/5..9` policy bytes, the frozen v5–v9 evidence, or another lane's files. The seam is
nil-tolerant: a nil seam leaves every code path and every recorded run byte-identical.

### 14.1 Declared VS1 reductions (implemented-vs-designed gaps, current truth)

The design above is the contract; these are the places the VS1 implementation is deliberately
smaller, each carried honestly into the evidence rather than papered over.

1. **Newborn inheritance.** The apply binds the living roster present at the audit boundary; a
   newborn's first claim before the next rebind takes the frozen fallback. Inheritance at birth is
   not implemented (§5.3 step 4, §5.4).
2. **Generation-aware replay keys on one `ApplyHour`.** A bundle restores the generation that took
   effect at its recorded boundary; an apply following a mid-run rollback within the same run is not
   yet replayed from a single bundle (§5.4). The full-ledger replay is the verification lane's.
3. **Live audit set.** The live Role B loop audits `{24,48,72,96,120,144}`; `h168` is the terminal
   horizon, where a 24-hour sandbox fork would exceed the frozen 168-hour envelope, so it is the
   measurement boundary rather than a live audit. §5.1's declared set records the design intent.
4. **Extinct-roster audits are no-ops.** An audit hour with no living roster records a zero-cost
   `no-effect` audit (`roster-extinct`) instead of a rebind; there is nothing to bind or measure.
5. **A3 cells.** All 12 declared A3 reference cells were funded in the hermetic matrix; the §11.3
   A3 reduction (f=12 x seeds 0–3) applies only to an authorised-spend live run.
6. **Hermetic model.** Every arm runs L1's deterministic FakeClient: the mechanism is proven
   load-bearing, while real-model load-bearing awaits spend authorisation (§13, §16). Costs are
   modelled price-table charges over fake usage, never real spend.
7. **Sandbox gating regression is halt-or-extinction (code-review P2-12).** §10.2's "no declared
   gating band or invariant regresses" is implemented as the deterministic fork's ceiling-halt or
   population-extinction signal, not a per-band G1–G3 comparison; a branch that regresses an
   individual declared band without halting or dying is not caught by the sandbox. A later lane
   evaluates the full band set per branch.
8. **EC-6's adversarial proof is a marker/key-set scan (code-review P2-14).** The matrix-time EC-6
   check feeds a fully populated sentinel observation through the allow-list encoder and scans every
   recorded request for privileged markers and unexpected keys; it does not construct §6.6.3's world
   whose privileged fields hold unique sentinel values and drive a run against it. That construction
   is the committed unit test (internal/cognition/observation_test.go); the matrix re-derives the
   key-set and marker scan from the recorded requests.
9. **A Role A intervention records the level-1 outcome, not the world verdict (code-review P2-10
   residual).** The role-a record is appended before the world commits the claim, so its `result` is
   the level-1 acceptance/fallback. The world verdict is the committed journal attempt; the
   evaluation joins it to the escalated (hour, actor) set and counts level-2 refusals in fidelity
   indicator 7 and each cell report. Writing the verdict back onto the intervention record would
   require a post-commit ledger append and is left to a later lane.
10. **Scripted-fixture replay is outside the declared replay scope.** EC-3's replay subset covers the
    declared matrix AI cells whose generation state is empty — every A1 cell and the apply-free
    A2/A3 cells — replayed from genesis with zero model calls. The two scripted-proposer fixtures
    (A2f, A2g) are not replayed: the subset is the matrix's, and both fixtures are hand-authored
    lifecycle harnesses, not declared matrix cells. Reduction 2's single-`ApplyHour` bundle boundary
    is not the reason for their omission — neither fixture has an apply following a mid-run rollback
    (A2f applies once at h48 and rolls back at h72; A2g applies once at h48 and keeps the head) — so
    EC-3 attributes the fixtures to this scope boundary, not to reduction 2. With that boundary,
    §12 EC-5(a)'s "and replays byte-exact" clause is likewise deferred: the fixtures run their
    propose→validate→sandbox→apply→measure lifecycle end to end, and the byte-exact genesis replay of
    a scripted fixture (and of a matrix cell carrying applied generation state) is left to the
    full-ledger replay lane.

Two further facts are declared rather than chased, because neither changes the delivered contract:

- The `R2` commit message's numeric detail for the A2g h72 sandbox re-check ("observed 3 > 2 over
  [72,96]") is not independently checkable from the regenerated evidence ledger: the ledger records
  the admission verdict and the divergent-parent sandbox, not that raw window comparison.
- The earlier reviews' numbered P2 finding lists are not stored as a single artifact in the
  repository; the findings live in the review messages with their repairs in the commits, and items
  7–9 above are the current-truth record of the ones that remain open.

## 15. Risks and stop conditions

Risks: the gateway is an external dependency (availability, rate limits, cost) mitigated by the
ledger/replay design but not removed for live runs; the model may be uninformative or error-prone
(measured by the decision-error rate, not hidden); network latency lands in the coordinator's
pre-batch path (bounded by the declared deadlines, measured separately from engine compute); the
sandbox multiplies deterministic compute per proposal (instrumented); the rebind is runner state, so
its audit authority is the ledger + manifest + strategy section rather than the kernel event chain
(declared limitation, Appendix D); and the f=10/11 reliable cells may show divergence without
improvement (reported, and it is VS2's question).

Stop conditions (a fired condition halts the lane and reports, never patches around it): any byte
drift in the frozen substrate or in a recorded non-cognitive manifest; a model call during replay; a
world mutation on any refusal/fallback path; a broken patch reaching application; a rollback leaving
a child ref bound; a request containing a privileged marker; a silent ceiling; a decision made from
privileged state; an unlabelled report; any band, constant, or cell edited after its first run.

## 16. Open questions for the operator

1. **Spend authorisation and the gateway contract.** The design needs (a) the authorised cognitive
   ceiling (calls/tokens/spend) for the matrix — the arithmetic in §11.2 gives roughly 32 k calls for
   the full plan (≈11 k across A1, A1i and A2, ≈22 k for A3), up to ×3 worst case with retries, plus
   deterministic sandbox compute; (b) the price-table version and values used to convert usage to
   cost; (c) the gateway base URL and whether it honours `response_format: json_object` and a `seed`
   parameter. Without (a) the A3 reduction of §11.3 applies; without (c) the declared
   plain-completion fallback applies.
2. **One seam or two.** This design escalates exactly one decision (the claim instant) so the model's
   causal footprint is exactly one instant per selected actor-hour. If the operator intends the
   unit-conversion decision (bag decision, `h+20m+2µs`) to be model-decided in VS1 as well, that is a
   second seam with its own action vocabulary and cost, and it should be declared now rather than added
   mid-experiment.
3. **Patch scope.** The declared patchable set (Appendix C) is the strategy-side threshold vector only;
   world-side contract constants (consent margins, point cost, spoilage, wear) are deliberately not
   patchable. Confirm this boundary is the intended notion of "behaviour logic" for VS1, or name the
   additional strategy-side parameters that should be in the contract.
4. **Rebind authority.** The design makes the mid-run strategy switch runner state recorded in the
   ledger, manifest, and strategy section. Confirm that this satisfies the operator's "durable,
   attributed, externally auditable, revertible" requirement, or require the rebind to be modelled as
   a kernel rule with its own event.
5. **Cell/seed selection and the A0 reuse.** The declared 12 cells reuse the recorded ticket-19
   `natural/7 × v6` evidence as the no-AI arm. Confirm the cells (f=10/11/12 × seeds 0–3) and that
   reusing recorded evidence as the control arm is acceptable (EC-1 requires one fresh byte-identity
   reproduction through the VS1 harness).

## Appendix A — Types (the implementable contract)

```go
type SeamDecider interface {
    DecideActorSeam(ctx context.Context, req ActorSeamRequest) (ActorSeamDecision, error)
    ProposeStrategyPatch(ctx context.Context, req PatchRequest) (PatchProposal, error)
}

type ActorSeamRequest struct {          // built only from strategy values
    Seam        string                  // "natural.claim"
    Actor       sim.EntityID
    Hour        int
    Version     sim.WorldVersion
    Observation strategy.NaturalObservation
    Strategy    map[ParamID]int64       // the actor's own bound parameter vector
    Actions     []string                // the legal set for this seam and mechanism
    Identity    RunIdentity
}

type ActorSeamDecision struct {
    Action      string                  // gather | eat-stored | provision | wait
    Rationale   string
    Confidence  float64
    Source      string                  // "model" | "fallback"
    CallRef     uint64                  // ledger sequence
    Reason      string                  // typed fallback reason when Source == "fallback"
}

type PatchRequest struct {
    Role        string                  // "strategy-patch"
    AuditHour   int
    Evidence    TrajectoryEvidence      // §5.1 aggregate packet only
    ParentRef   strategy.NaturalRef
    Parameters  []ParameterSpec         // Appendix C
    Identity    RunIdentity
}

type PatchProposal struct {
    Propose     bool
    ParentRef   strategy.NaturalRef
    Deltas      []ParameterDelta
    Intent      string
    CallRef     uint64
}
```

## Appendix B — Ledger record schemas

**Call record** (append-only, hash-chained): `seq`, `kind: "call"`, `role`, `request_hash` (32 B),
`request_bytes`, `response_body` (full, bounded 64 KiB), `content` (extracted), `model_id`,
`endpoint` (host+path), `credential_present`, `sampling {temperature, top_p, max_tokens, seed}`,
`usage {prompt_tokens, completion_tokens}`, `cost_micros`, `price_version`, `attempt`,
`status ∈ {ok, malformed, inadmissible, oversized, transport-error, http-error, timeout}`,
`started_at_unix_micros`, `latency_micros`, `prev_hash`, `hash`.

**Intervention record**: `seq`, `kind ∈ {identity, role-a, patch-propose, patch-validate, patch-apply,
patch-measure, patch-rollback, no-trigger-audit}`, `cause {wake_policy, seam, actor, hour, sim_time,
trigger, evidence_digest}`, `call_refs[]`, `cost {calls, prompt_tokens, completion_tokens,
cost_micros, engine_micros}`, `role_a {action, source, reason, confidence, fallback_reason}`,
`patch {parent_ref, child_ref, parent_fingerprint, child_fingerprint, deltas[], intent, patch_digest,
scope, affected_actors_digest}`, `validation {verdict, reason, sandbox_branch_digest, window_hours}`,
`result {status ∈ {applied, measured, rolled-back, rejected, no-effect, unavailable}, window_hours,
indicator_deltas, reason}`, `prev_hash`, `hash`.

## Appendix C — Patch parameter contract (VS1)

| Parameter id | Modulates | Parent cascade point | Frozen value (`natural/7`, v6) | Domain | In-world reading |
|---|---|---|---|---|---|
| `claim.provision_energy_floor` | minimum own energy to provision | claim `h+1µs` | 4 | [1, 8] | provisioning pays only from the reserve |
| `claim.bank_energy_ceiling` | maximum own energy at which the bank may be drawn as a meal | claim `h+1µs` | 3 | [0, 8] | the bank is survival food, not regular food |
| `claim.gather_energy_floor` | minimum own energy to claim a gather | claim `h+1µs` | 1 | [1, 8] | do not spend the hour gathering while starving |
| `bag.build_energy_floor` | minimum own energy to invest the held unit | bag decision `h+20m+2µs` | 3 | [1, 8] | the shared investment guard |
| `bag.build_start_energy_floor` | minimum own energy to start a first point | bag decision | 4 | [1, 8] | re-pioneer only from a full reserve |
| `bag.build_finish_energy_floor` | minimum own energy to finish a point in progress | bag decision | 3 | [1, 8] | finish a sunk point at the survival floor |
| `birth.filing_energy_floor` | minimum own energy to file a birth request | birth request `h+40m+5µs` | 4 | [1, 8] | the proposer's filing floor |

Each parameter is an integer with a declared unit (energy units), a declared domain derived from the
mechanism's own bounds, and an effect point. The all-frozen-values vector must reproduce the parent's
decisions exactly on the existing decision-grid differential test. A patch is a **subset** of these
ids with values in their domains; no other field is patchable.

## Appendix D — Contradictions and gaps found in the founding documents

Recorded here for the separate reconciliation pass; none is resolved by this design except where
§2.3 says so.

1. **`docs/ROADMAP.md` P0 *Exclude* / `docs/P0-SPEC.md` §2 vs the M1=VS1 product decision.** P0
   excludes language-model calls and LM strategy editing; VS1 requires both. Resolved by supersession
   (§2.3); the ROADMAP text still needs the reconciliation edit.
2. **`docs/VERTICAL-SLICES.md` marks the fidelity metric both "(needs operator confirmation)" and, in
   its confirmed-decisions section, "frozen as stated below".** Same document, two statuses. VS1
   follows the frozen reading and scaffolds it (§11.5); the document should be made consistent in the
   reconciliation pass.
3. **`README.md` present-tense claim that language models "wake selectively".** No such mechanism
   exists, and VS1 does not create one: VS1's C7 instance is the uniform/periodic baseline; value-based
   selective waking is VS2. The README sentence remains false until VS2 and needs the audit's
   corrective edit.
4. **`docs/ARCHITECTURE.md` §4 / `docs/PRINCIPLES.md` P7 (priority formula, escalation requests,
   protected exploration) vs VS1's declared baseline.** VS1 implements a declared wake-policy object
   with the honest baseline and no escalation requests; those documents describe the VS2 end state.
   Not a defect, but the "present in every version" invariant should be read as "the concept's object
   exists", not "the deep mechanism exists".
5. **`docs/VERTICAL-SLICES.md` C8 wording "every model intervention records cause, cost, patch, and
   measured result" vs a Role A intervention having no patch.** A Role A decision has no patch. This
   design records `patch` as the explicit role marker plus the applied-version head for Role A, and
   reserves the patch block for Role B (Appendix B). The roadmap wording should be tightened.
6. **`docs/VERTICAL-SLICES.md` cross-cutting invariant 7 "without trusting any agent" vs
   `docs/P0-SPEC.md` §13 "digest chains detect damage, not malicious replacement".** Both are true and
   jointly bound the claim; §9.3 states the limit verbatim rather than overclaiming.
7. **`docs/P0-SPEC.md` §2/§10 "escalation requests" and `docs/ARCHITECTURE.md` §4 "may request
   cognitive escalation" vs the code: no `EscalationRequest` type exists.** VS1 does not add one
   (VS2 owns it); the document promises a concept the code does not have and the reconciliation pass
   should mark it as not-yet-implemented.
8. **`docs/ROADMAP.md` P1 (≈10 000 actors) and its P0 exit criteria vs the recorded evidence
   (16-actor substrate, 256-founder ceiling halt at h66–h70).** The roadmap's phase targets and the
   measured substrate differ by orders of magnitude; VS1 runs at the substrate's measured scale and
   says so (§11.3–11.4). No design change claimed.
9. **`docs/ROADMAP.md` P0 exit criterion "distinct strategies produce measurable and causally
   explainable population outcomes" vs its own record** that all organic policies are populationally
   indistinguishable at the saturated fixture and differentiate only on the later density axis. The
   roadmap text should carry the qualification the ticket-19 map established.
