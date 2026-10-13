# ADR 0006 — Cognitive layer with recorded model calls

Status: accepted for VS1 (M1)

## Context

`docs/VERTICAL-SLICES.md` (operator-confirmed) makes VS1 the first version that is not an LLM-free
simulator: a real model must drive part of the world's decisions and modify behaviour logic during
the run. An independent adversarial audit (`.scratch/audit-vision-fidelity.md`) established that
neither exists: the repository contains no model call path at all, `TokenCost` is hard-coded to `0` in
every reporter, `WakeStrategyInvalidation` has no producer, and ADR 0002's promise of in-run strategy
updates with parent, proposer, validation, and rollout state has no implementation. The founding
documents also promised concepts the code lacked (`docs/ARCHITECTURE.md` §5 "a model manager may
propose a one-time intention; a local strategy patch", P11's sandbox → staged rollout → rollback, P7's
selective waking) while the world, the event store, and the versioned-strategy runtime already existed
and were measured.

The design question is where a nondeterministic, external, paid, opaque component enters a system
whose value proposition is exact replay and auditable causation. Four questions had to be answered
together: how a call is recorded, what a model may see, what it may change, and what bounds its cost.

## Decision

A **cognitive layer** exists beside the world and never inside it. Model calls are recorded events.

1. **Recorded calls.** Every model call is an append-only, hash-chained ledger record carrying the
   canonical request bytes and hash, the full response bytes, model id, endpoint, sampling parameters,
   usage, cost, latency, attempt index, and status. The ledger is flushed before a response is
   consumed and is bound by the checkpoint manifest (an omitempty cognition block), so a used call can
   never be unrecorded and a restore refuses a missing or mismatched ledger.
2. **Replay never calls a model.** Replay reads the ledger and re-presents each request hash; a miss
   is a hard error. Replay mode is incapable of network access. Sampling temperature may be above
   zero: the model's nondeterminism is captured as recorded events, not wished away, and results are
   labelled model-conditional.
3. **The actor-decision seam.** A declared seam (VS1: the claim-instant action `gather | eat-stored |
   provision | wait` at `h+1µs`) is the only escalated decision. Its input is built from the frozen
   actor observation value by an allow-list encoder; the model receives no privileged state, no other
   actor's private state, and no world reader. The model names an action; the existing kernel and
   contracts validate and commit it; every refusal path mutates nothing.
4. **Strategy modification is versioned data, never code.** The model proposes a patch as a typed
   integer delta vector over a declared, bounded parameter contract (`docs/adr/0002`); the harness
   validates it, runs a deterministic counterfactual fork (patched versus parent, seam disabled),
   applies it atomically to the roster at a quiescent boundary if admissible, measures it, and rolls
   it back when the declared criterion fails. Lineage, proposer, validation, sandbox verdict, apply
   boundary, affected actors, and rollback are recorded and survive restart.
5. **Cost is a modelled resource.** Calls, tokens, and cost are instrumented per call, per
   actor-hour, and per run, under a declared ceiling that comes from our compute and authorised spend
   (P15), is fail-loud, and can never silently substitute a cheap decision for a model call.
6. **The world is untouched.** The seam is an optional, nil-tolerant interface typed with `sim` and
   `strategy` values; with it absent, every recorded world, manifest, and policy byte is unchanged,
   and the previous evidence stays valid.

## Alternatives considered

- **Re-call the model during replay.** Rejected. It would make replay nondeterministic and
  unrepeatable, spend money on a verification pass, and destroy the property that makes the evidence
  auditable: a recorded run's own history reproduces exactly. The whole point of a nondeterministic
  component in a deterministic simulator is that its outputs are events.
- **Let the model emit code (a host-language strategy or arbitrary executable structure).** Rejected.
  It contradicts P3 and ADR 0002 (strategy is typed, versioned, byte-pinned data), and it replaces a
  bounded validation problem with a sandboxing and trust problem whose failure mode is arbitrary
  world access. `docs/ARCHITECTURE.md` §5 already rules: all outputs are schemas, not direct code or
  world mutations. A bounded parameter vector over the declared decision points is expressible as
  data, testable for a parent reduction, and reversible by reference.
- **Grant the model privileged state (a world reader, aggregate truth, or other actors' state).** Re-
  jected. It violates C5, P5, and P7, and it would make the project's differentiator untestable:
  reasoning *under* an observation boundary is the product; reasoning with omniscience is a different
  experiment with no causal claim about people. The boundary is enforced structurally (the request
  builder cannot import the world package) and tested adversarially with sentinel privileged values.
- **Batch several actors into one call (P8's amortization).** Deferred, not adopted: isolation,
  namespacing, and contamination tests are prerequisite work; VS1's per-decision calls make the
  boundary trivially isolated. A later slice may revisit it.
- **Record only a summary of each call instead of exact bytes.** Rejected. An auditor must be able to
  recompute the request hash and re-validate the decision from the record alone; a summary makes the
  ledger an assertion rather than evidence.
- **Emit a scalar "value of reasoning" score and schedule on it.** Rejected for VS1 and per ADR 0003:
  the VS1 wake policy is the honest uniform/periodic baseline; value-based allocation is VS2 and must
  be evaluated on a metric vector, never one scalar.

## Consequences

- The existing no-AI substrate remains byte-identical and its evidence stays valid; the cognitive
  layer adds evidence and a new artifact class (ledgers) rather than changing the world.
- Determinism becomes *recorded-event* determinism. A recorded run replays exactly; a fresh run at the
  same seed is a new model-conditional event stream with its own ledger.
- The gateway becomes a run dependency for live runs: availability, rate limits, and cost are
  operational facts, mitigated (not removed) by bounded retries, deadlines, a fail-loud transport
  failure, and a hermetic test path that never reaches the network.
- The checkpoint contract grows an omitempty cognition block and the strategy section grows cognitive
  generation content and bindings for cognitive lanes; non-cognitive bundles and manifests are
  byte-unchanged.
- Mid-run strategy switching is runner state recorded in the ledger, manifest, and strategy section,
  not a kernel rule. Its audit authority is therefore the ledger plus the manifest binding rather than
  the kernel event chain — a declared limitation, listed as an open question in the VS1 spec.
- External auditability is bounded honestly: the ledgers and the accepted history let an outside party
  verify what was recorded, attributed, applied, and measured without trusting any agent; digest
  chains detect damage, not a wholesale rewrite by an adversary with write access to every artifact,
  and the model's outputs remain opaque judgments whose quality is measured, not proved.
- A sandbox counterfactual per proposal adds deterministic compute and wall-clock time; it is
  instrumented in the same ceiling ledger as every other resource.

## Relationship to ADR 0002 and ADR 0004

ADR 0002 supplies the substrate this decision stands on: strategies are typed, executable, versioned
data with exact parent refs, and every change records parent, proposer, reason, validation result,
rollout state, and affected actors, with running activities retaining committed semantics and the next
eligible decision observing the new reference. ADR 0006 adds the two things ADR 0002 assumed but never
had: a **model as proposer during the run** and a **recorded, replayable call path** to attribute the
proposal and its cost. It does not weaken ADR 0002's discipline — patches stay data, remain validated
before use, and remain revertible with lineage retained.

ADR 0004 (branch and beam evaluation) is honoured in its minimal form: the sandbox is a single
deterministic fork pair evaluated under matched conditions, and its verdict is recorded. Multi-
candidate beam-style evaluation with diversity constraints is not adopted in VS1; it belongs to VS3's
sustained evolution, once a population of strategy variants exists to retain. ADR 0003 is honoured by
refusing a single scalar objective and emitting a per-indicator divergence vector instead.

## Addendum — the real gateway wire contract (ticket 22)

Discovery against the running LiteLLM gateway (`GET /v1/models`, metadata only, no spend) fixed the
wire contract the VS1 real-model arm needs. This addendum is code plus hermetic tests; no live
model call has been made.

- **Model identity.** The application model is exposed as `github_copilot/gpt-6-luna` (underscore).
  The VS1 spec's earlier hyphen spelling `github-copilot/gpt-6-luna` is not exposed by the gateway.
  There is one declared application-model identity (`cognition.DefaultModelID`, the same value as
  `DefaultGatewayModel`); the gateway's configured model comes from `$LITELLM_MODEL`, and the
  canonical request encoder and the recorded run-identity model digest are derived from the declared
  identity rather than a superseded literal.
- **Model identity is recorded.** The recorded run-identity model digest is derived from the
  application model, the declared wire, and the frozen sampling instance, so the checkpoint binding
  names the model and wire a run actually used and two runs on different models or wires cannot share
  an identity.
- **Output mode is recorded per call.** The requested JSON output mode (`json_object` or a plain
  completion) is recorded on the call record when the transport declares it; the hermetic fake and
  replay transports declare none, so their recorded bytes are unchanged.
- **Two wires, one recorded mode.** The client speaks the OpenAI-compatible chat wire
  (`POST /v1/chat/completions`, the original assumption, kept as the hermetic default) and the
  Responses wire (`POST /v1/responses`), which the application model's metadata advertises as its
  only supported endpoint. A `WireMode` selects the request path, the request body, the JSON output
  mode (`response_format` versus `text.format`), and the usage fields (`prompt_tokens` /
  `completion_tokens` versus `input_tokens` / `output_tokens`). The wire mode reaches the ledger
  through the recorded endpoint host+path, so no record field changes and existing records stay
  byte-identical.
- **Configuration, never recorded.** The base URL (`$LITELLM_BASE_URL`), the bearer credential
  (`$LITELLM_API_KEY`), the wire model, and the wire are configuration. Only the endpoint host+path
  and `credential_present` reach the ledger; no secret material is ever recorded.
- **Unpriced is a typed state.** The application model's metadata carries no input/output cost fields.
  Cost computation is pluggable (`Pricer`); when no entry exists the default `UnpricedPrice` records
  `price_version: "unpriced"` with `cost_micros: 0`, a typed state rather than a fabricated zero. The
  typed `Cost.Unpriced` state and the reserved `unpriced` version must agree, so the recorded unpriced
  flag is derived from them. A declared `PriceTable` still prices a priced model exactly as before.
  Because an unpriced call charges nothing, the cost ceiling cannot bind for it: a live unpriced run
  must declare a price or explicitly waive the cost ceiling.
- **Oversized bodies are labelled.** An oversized response is recorded by digest and length; because
  the transport buffers only a bounded prefix, the record marks the digest/length as a truncated
  prefix so an auditor knows the digest is recomputable from that prefix and does not claim the full
  body.
- **One network boundary.** The real transport (`internal/cognition/gateway.go`) is the package's
  single declared network file; the structural guard test permits `net` only there and keeps the
  fake, replay, ledger, recorder, and world-boundary prohibitions everywhere else.
- **Disabled by default.** `cmd/vs1 gateway-probe` is the only command that contacts the gateway. It
  performs one metadata request and refuses unless `-authorize` or
  `AGENTWORLD_GATEWAY_PROBE_AUTHORIZED=1` is set, so the later authorised live lane enables it with
  one flag. The bearer credential is read from the environment only (there is no `-api-key` flag), so
  it cannot appear in the process table, and `DiscoverGatewayContract` itself refuses an unauthorized
  config with a typed error without opening a connection.

Retries, the 30 s attempt timeout and 90 s call deadline, ledger recording, replay, and the budget
remain the Recorder's (unchanged); the transport performs exactly one attempt and never records.
