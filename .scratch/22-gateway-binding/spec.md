# Ticket 22 — Gateway binding (VS1 real-model arm enablement)

Status: in progress (worker lane). Repair branch `feature/22-gateway-binding-v2` off `38e3dd2`
(the `feature/22-gateway-binding` tip); original lane branch `feature/22-gateway-binding`.
Baseline revision at lane start: `6fa551a9f3eadd55ed934805e1c3fd0f36415ddf` (master, 2026-10-13).

A post-completion independent static review returned FAIL with one P1 and several P2 findings; the
repair round is recorded in the "Repair round (v2)" section below.

## Objective

Bind the agentworld cognition HTTP client to the real LiteLLM gateway contract so VS1's real-model
arm (`.scratch/21-vs1-mind-in-the-loop/spec.md` §6, §16.1) can run once spend is authorised.

This lane is **CODE + HERMETIC TESTS ONLY**. No real gateway/model call and no HTTP to the gateway is
made here; the live smoke is a later authorised lane.

## Contract (what this lane owns)

Sole writer of: `internal/cognition/client.go` + its tests; new `internal/cognition/gateway*.go` +
tests; the `cmd/vs1` endpoint/mode configuration additions + a disabled-by-default `gateway-probe`
subcommand; the §6.1 wire-contract current-fact correction in the ticket-21 spec; a wire-contract
addendum in `docs/adr/0006-cognitive-layer-recorded-model-calls.md`. Read-only everywhere else.

Explicit non-goals: no change to `internal/world`, `internal/strategy`, `internal/kernel`,
`internal/scheduler`, `internal/sim`, the frozen `natural/5..9` policy bytes, `.tmp` frozen evidence,
or another lane's files; no live gateway contact; no secret in any file or this ticket.

## Requirements (from dispatch)

1. Preserve existing hermetic behaviour byte-for-byte (FakeClient path, ledger records, replay, every
   existing test's observable output; nil/no-seam paths byte-identical).
2. A wire-mode abstraction with two implementations (chat-completions = existing behaviour,
   responses = `/v1/responses`): request construction, JSON output mode, usage extraction
   (`prompt_tokens`/`completion_tokens`), status/error handling, retry/timeout per §6.5.
3. Model identity + endpoint are configuration + run-identity recorded, never hard-coded, never
   secret-bearing; the ledger keeps recording endpoint host+path and `credential_present`.
4. Cost computation is pluggable with a declared price table, defaulting to a typed **unpriced** state
   rather than a fabricated zero.
5. Hermetic `httptest` tests: both wire modes; malformed/oversized/transport/http/timeout typed
   outcomes; a Responses-API response maps to the same `ActorSeamDecision`/`PatchProposal` types.
6. A `gateway-probe` command that discovers the real contract, disabled unless an explicit
   flag/env authorises it.

## Measurable acceptance indicators

- `go build ./...` passes.
- `go vet ./...` passes.
- `go test ./internal/cognition/... ./cmd/vs1/...` passes.
- `git diff --check` clean (committed tree).
- At least one test fails if the Responses/chat mapping is broken (demonstrated by a recorded
  mutation probe).
- Existing hermetic outputs unchanged: the tests in requirement 1 pass identically.

## Discovered facts (source: dispatch; repo-verified where checkable)

- Gateway base `$LITELLM_BASE_URL` (currently `http://127.0.0.1:4000`), Bearer `$LITELLM_API_KEY`;
  both present in this environment at lane start (values never recorded).
- `GET /v1/models` (metadata, no spend) exposes the application model as `github_copilot/gpt-6-luna`
  (**underscore**; ticket-21 §6.1 records the underscore spelling after its correction).
  `model_info`: `mode=responses`, `supported_endpoints=['/v1/responses']` only (no
  `/v1/chat/completions`), `max_input_tokens 272000`, `max_output_tokens 128000`,
  `supports_response_schema/reasoning/streaming/function_calling true`, and **no** input/output cost
  fields (unpriced).
- Therefore §6.1's assumed `POST {base}/v1/chat/completions` with `response_format: json_object` is
  contradicted by the metadata; the client must speak both wires and record which was used.

## Decisions taken in this lane (recorded here)

- **Cost**: `RecorderConfig.Price` becomes a `Pricer` interface (defined in `gateway.go`, implemented
  by `PriceTable` via methods in the same package, plus an `UnpricedPrice` sentinel). An unpriced
  attempt records `cost_micros: 0` with the reserved `price_version: "unpriced"`; the typed
  `Cost.Unpriced` state and the reserved version are asserted to agree, so the recorded unpriced flag
  is derived from them and no dead field is left. A priced `PriceTable` records its own version, so
  existing ledger bytes are unchanged.
- **Wire mode recording**: the recorded endpoint host+path names the wire
  (`.../v1/chat/completions` or `.../v1/responses`); there is no `wire_mode` record field and no
  `WireModeReporter` interface. Fake/replay transports keep the existing chat spelling, so existing
  ledger bytes are unchanged; the real gateway records its configured wire path.
- **Model identity**: `DefaultModelID` is the single declared application-model identity, the
  operator-decided gateway-exposed `github_copilot/gpt-6-luna` (underscore). `DefaultGatewayModel` is
  that same value; the wire model id is still configuration (`$LITELLM_MODEL`). The canonical request
  encoder writes this declared model, and the recorded run-identity model digest is derived from the
  declared model + declared wire + frozen sampling, so two runs on different models or wires get
  different digests. The recorder records the model actually addressed: a transport implementing
  `TransportIdentity` (the real gateway) supplies it via `WireModel()`, else the request's declared
  `Model` is recorded unchanged (so hermetic bytes are identical). The earlier `zai/glm-5.3-flash`
  choice and the hyphen-spelled `github-copilot/gpt-6-luna` are superseded.
- **Network boundary**: the real transport (`gateway.go`) is the single declared network file; the
  structural guard test is narrowed to forbid `net` everywhere in the package *except* that file, and
  keeps the world-import prohibition everywhere.
- **Output mode**: the requested JSON output mode (`json_object` or plain text) is recorded per call
  as `output_mode` (omitempty) when the transport implements `TransportOutputMode`. Fake/replay
  transports implement neither, so existing ledger bytes are unchanged. An oversized body's digest
  and length are labelled `response_truncated_prefix` because the transport buffers only a bounded
  prefix, not the full body.
- **Gateway probe**: `vs1 gateway-probe` refuses unless `-authorize` or
  `AGENTWORLD_GATEWAY_PROBE_AUTHORIZED=1` is set; it only reads `GET /v1/models` (no spend). The
  credential is environment-only (no `-api-key` flag, so it cannot appear in the process table), and
  `DiscoverGatewayContract` itself refuses an unauthorized config with a typed error and no network.

## Evidence (pre-repair lane run on `feature/22-gateway-binding`)

Baseline revision `6fa551a9f3eadd55ed934805e1c3fd0f36415ddf` (master). All commands run with
`GOROOT=$HOME/../vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24` (the local Go 1.24.13
toolchain; values not recorded).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test ./internal/cognition/...` | passed (cognition 1.4 s, protocol cached) |
| `go test ./cmd/vs1/...` | passed (163 s, full hermetic matrix) |
| `gofmt -l` over the tree | clean |
| `git diff --check` | clean |

**Requirement 1 byte identity (FakeClient path).** A temporary probe (since deleted) recorded one
hermetic `FakeClient` call with `PriceTable{test/1,1,2}` and printed the exact ledger bytes. The same
probe ran in a detached worktree at the baseline revision `6fa551a`; the two hex dumps were
byte-identical (`LEDGER_BYTES_IDENTICAL`). The recorder's fake-path fields are unchanged by
construction: `recordedModel` returns the declared model because the fake implements no
`TransportIdentity`, and `PriceTable.Cost` returns the same micros as the old `CostMicros`.

**Requirement 2/5 wire modes and typed outcomes.** `internal/cognition/gateway_test.go` exercises
both wires against `httptest`: chat request build (`/v1/chat/completions`, `response_format`), usage
`prompt_tokens`/`completion_tokens`; Responses request build (`/v1/responses`, `text.format`), usage
`input_tokens`/`output_tokens`; a Responses body (with a preceding reasoning item) mapping to
`ActorSeamDecision` and `PatchProposal`; HTTP 429 retry then success; HTTP 400 terminal; transport
error; attempt-timeout; malformed body; oversized body by digest+length; and the empty-mapping
fallback.

**Requirement 5 broken-mapping demonstration (mutation probe).** Temporarily changing the Responses
extraction guard (`if part.Type == "output_text" || part.Type == ""` ->
`... "output_text_DISABLED" ...`) made `TestGatewayResponsesWireMapsDecisionAndProposal` fail
(`responses usage (input/output) not mapped to prompt/completion: {0 0}`); the change was reverted.
So the test detects a broken Responses/chat mapping.

**Requirement 3 model/endpoint recording.** `TestGatewayRecordsConfiguredModelAndWireEndpoint`
records `model_id = github_copilot/gpt-6-luna` (configured) and
`endpoint = 127.0.0.1:.../v1/responses` (host+path, names the wire), `credential_present: true`; the
fake path records the same declared `model_id = github_copilot/gpt-6-luna` and no secret (the
hermetic transports report no `TransportIdentity`, so the declared model is recorded unchanged).

**Requirement 4 unpriced cost.** `TestGatewayUnpricedCostIsTypedNotZero`: `UnpricedPrice` records
`price_version: "unpriced"`, `cost_micros: 0`, `CallResult.CostUnpriced == true`; a `PriceTable`
records its real price. `TestRecorderRejectsInconsistentPricer` pins that a `Pricer` whose typed
`Cost.Unpriced` disagrees with its reserved version is refused, so the recorded unpriced flag is
derived from the agreed pair rather than a dead field.

**One declared application-model identity.** `cognition.DefaultModelID` and `cognition.DefaultGatewayModel`
are the one value `github_copilot/gpt-6-luna` (underscore). `TestActorSeamRequestEncoderUsesTheDeclaredApplicationModel`
(observation) and `TestModelIdentityDigestBindsModelAndWire` (gateway) pin that the canonical request
encoder and the recorded model-identity digest use it.

**Model identity digest binds model + wire.** `cognition.ModelIdentityDigest(model, wire, sampling)`
is SHA-256 over the model, wire, and sampling; `cmd/vs1.modelIdentityDigestBytes` derives it from the
declared application model and `declaredGatewayWire` (the wire the harness wires into the stack),
not a literal. `TestModelIdentityDigestBindsModelAndWire` asserts different models and different
wires give different digests and that the empty wire is the declared chat default.

## Residual risks / follow-ups

- An unpriced call charges nothing, so the cost ceiling cannot bind for the application model until a
  price is declared; the ADR addendum records this.
- The live gateway metadata was not re-fetched in this lane (no HTTP to the gateway allowed); the
  discovered facts are the dispatch's, encoded in `DefaultModelID`/`DefaultGatewayModel`,
  `WireModeForEndpoints`, the probe, and the §6 correction. The disabled probe is the authorised
  later-lane verification path. No live model call and no live gateway contact were made in this
  lane.
- The structural network guard was narrowed to permit `net` only in the new `gateway.go`; the guard
  still fails on `net` anywhere else and on any `internal/world` import.
- The oversized-body digest describes only the bounded prefix the transport buffers; the record is
  labelled `response_truncated_prefix: true` so an auditor recomputes it from that prefix and does
  not mistake it for a full-body digest.

## Repair round (v2) — independent review finding → repair map

An independent static review of the lane returned FAIL: one P1 (blocking) and several P2 findings.
All are repaired on branch `feature/22-gateway-binding-v2` (off the lane tip `38e3dd2`). No frozen
recorded evidence and no other lane's files were changed; `.tmp/` is gitignored, so no committed
recorded artifact is touched.

| Finding | Severity | Repair | Where |
|---|---|---|---|
| Defined model/wire not bound into the recorded run identity: `cmd/vs1/run.go` `modelIdentityDigestBytes()` hard-coded `DefaultModelID` + a literal sampling string and ignored the configured model/wire; the canonical encoder wrote the superseded `zai/glm-5.3-flash`. | P1 (blocking) | One declared application-model identity: `DefaultModelID = "github_copilot/gpt-6-luna"` (underscore) with `DefaultGatewayModel = DefaultModelID`; the canonical request encoder emits it; new `cognition.ModelIdentityDigest(model, wire, sampling)` (SHA-256 over model+wire+sampling) replaces the literal digest; `cmd/vs1.modelIdentityDigestBytes()` derives it from `DefaultModelID` and `declaredGatewayWire`. | `internal/cognition/client.go` (`observation.go` uses `DefaultModelID`), `gateway.go`, `cmd/vs1/run.go` |
| P1 golden/test consequence | — | `goldenCallWire` re-captured because `model_id` is part of the call wire; `ledger_test.go` tamper probe now perturbs `DefaultModelID` instead of the removed literal. `TestModelIdentityDigestBindsModelAndWire` proves different models/wires give different digests; `TestActorSeamRequestEncoderUsesTheDeclaredApplicationModel` pins the encoder model. | `internal/cognition/codec_test.go`, `ledger_test.go`, `gateway_test.go`, `observation_test.go` |
| Ticket-21 §6.2 canonical-request example must match the encoder after unification. | P2 | Verified already aligned: §6.2 carries `"model": "github_copilot/gpt-6-luna"` after the earlier ticket-21 correction, and the encoder now produces exactly that value, so they agree (no spec edit needed). | `.scratch/21-vs1-mind-in-the-loop/spec.md` §6.2 |
| Ticket-22 decision bullets described non-existent fields `cost_unpriced` and `wire_mode`. | P2 | Corrected to the real mechanism: unpriced is the reserved `price_version: "unpriced"` (with typed `Cost.Unpriced` asserted to agree); the wire is named by the recorded endpoint host+path (no `wire_mode` field, no `WireModeReporter`). | `.scratch/22-gateway-binding/spec.md` Decisions |
| `Cost.Unpriced` dead; typed-unpriced hinged on the literal `"unpriced"` string. | P2 | `UnpricedPriceVersion` documented as reserved; the Recorder refuses a pricer whose `Cost.Unpriced` disagrees with its version, so the recorded flag derives from the agreed pair. Hermetic (`PriceTable`) bytes unchanged. | `internal/cognition/client.go`, `gateway.go`; `TestRecorderRejectsInconsistentPricer` |
| `DiscoverGatewayContract` did not itself refuse `!cfg.Authorized`. | P2 | It returns `ErrGatewayUnauthorized` before any I/O; `runGatewayProbe` passes `Authorized: true`. `TestDiscoverGatewayContractRefusesUnauthorizedWithoutNetwork` proves no connection is opened. | `internal/cognition/gateway.go`, `cmd/vs1/gateway_probe.go`, tests |
| Requested output mode not recorded per call; `OutputPlain` unexercised. | P2 | New `output_mode` (omitempty) call-record field recorded from an optional `TransportOutputMode` (the real gateway); json/plain covered and the empty hermetic path pinned; plain mode asserted to omit `response_format`/`text`. | `internal/cognition/codec.go`, `client.go`, `gateway.go`, tests |
| Oversized digest/length described a truncated buffer without saying so. | P2 | New `response_truncated_prefix` (omitempty) labels the digest/length as the bounded leading prefix; the oversized test asserts the label. | `internal/cognition/codec.go`, `client.go`, tests |
| `-api-key` flag could leak the credential to the process table. | P2 | Flag removed; the credential is read from `$LITELLM_API_KEY` only. `TestGatewayProbeHasNoAPIKeyFlag` pins that `-api-key` is a usage error. | `cmd/vs1/gateway_probe.go`, test |
| Hyphen → underscore application-model spelling. | P2 | `docs/VERTICAL-SLICES.md` states the gateway-exposed underscore id; the ADR addendum records the underscore id (remaining hyphen mentions only document that the earlier spelling is not exposed). | `docs/VERTICAL-SLICES.md`, `docs/adr/0006-cognitive-layer-recorded-model-calls.md` |

### v2 verification (branch `feature/22-gateway-binding-v2`, 2026-10-13)

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `go test ./internal/cognition/... ./cmd/vs1/...` | passed (`cmd/vs1` 165.333 s, full hermetic matrix; cognition and protocol pass) |
| `go test ./internal/world/ -run 'CognitiveReplay\|Cognitive'` | passed (19.5 s; reads `cognition.DefaultModelID` as a constant) |
| `gofmt -l .` | clean |
| `git diff --check` | clean |

`TestModelIdentityDigestBindsModelAndWire` demonstrates the P1 requirement directly: digests differ
for `other/model` vs `github_copilot/gpt-6-luna` and for `responses` vs `chat-completions`, and the
empty wire equals the declared chat default. EC-1 is unaffected: the A0 arm still wires no seam
(`wireStack` returns nil), so it makes no model call and its digest is never computed.

## Commit

Committed on `feature/22-gateway-binding` (not merged to master): see `git log -1` on that branch.

The v2 repair is committed on `feature/22-gateway-binding-v2` (not merged) as
`Repair ticket 22 gateway binding: bind model+wire into recorded identity, fix P2 findings`;
see `git log -1` on that branch for the exact revision.
