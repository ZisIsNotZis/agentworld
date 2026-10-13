# Ticket 22 — Gateway binding (VS1 real-model arm enablement)

Status: in progress (worker lane). Branch `feature/22-gateway-binding`.
Baseline revision at lane start: `6fa551a9f3eadd55ed934805e1c3fd0f36415ddf` (master, 2026-10-13).

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
  (**underscore**; ticket-21 §6.1 writes the hyphen spelling `github-copilot/gpt-6-luna`).
  `model_info`: `mode=responses`, `supported_endpoints=['/v1/responses']` only (no
  `/v1/chat/completions`), `max_input_tokens 272000`, `max_output_tokens 128000`,
  `supports_response_schema/reasoning/streaming/function_calling true`, and **no** input/output cost
  fields (unpriced).
- Therefore §6.1's assumed `POST {base}/v1/chat/completions` with `response_format: json_object` is
  contradicted by the metadata; the client must speak both wires and record which was used.

## Decisions taken in this lane (recorded here)

- **Cost**: `RecorderConfig.Price` becomes a `Pricer` interface (defined in `gateway.go`, implemented
  by `PriceTable` via methods in the same package, plus an `UnpricedPrice` sentinel). An unpriced
  attempt records `cost_micros: 0` with `cost_unpriced: true` (typed), never a fabricated price. The
  priced path leaves `cost_unpriced` empty so existing ledger bytes are unchanged.
- **Wire mode recording**: the recorder reads an optional `WireModeReporter` from the transport and
  records `wire_mode` (omitempty). Fake/replay transports report none, so existing ledger bytes are
  unchanged; the real gateway transport reports its configured mode.
- **Model-id mismatch**: the wire model id is configuration (`DefaultGatewayModel =
  "github_copilot/gpt-6-luna"`, the metadata underscore spelling). The recorder records the model
  actually addressed: a transport implementing `TransportIdentity` (the real gateway) supplies it via
  `WireModel()`, else the request's declared `Model` is recorded unchanged (so hermetic bytes are
  identical). The hyphen/underscore mismatch and the legacy canonical-request model are recorded
  below.
- **Network boundary**: the real transport (`gateway.go`) is the single declared network file; the
  structural guard test is narrowed to forbid `net` everywhere in the package *except* that file, and
  keeps the world-import prohibition everywhere.
- **Gateway probe**: `vs1 gateway-probe` refuses unless `-authorize` or
  `AGENTWORLD_GATEWAY_PROBE_AUTHORIZED=1` is set; it only reads `GET /v1/models` (no spend).

## Evidence

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
fake path still records `model_id = zai/glm-5.3-flash` and no secret.

**Requirement 4 unpriced cost.** `TestGatewayUnpricedCostIsTypedNotZero`: `UnpricedPrice` records
`price_version: "unpriced"`, `cost_micros: 0`, `CallResult.CostUnpriced == true`; a `PriceTable`
records its real price.

**Hyphen/underscore model-id mismatch.** The gateway metadata exposes `github_copilot/gpt-6-luna`
(underscore); ticket-21 §6.1 and `docs/VERTICAL-SLICES.md` write the hyphen spelling
`github-copilot/gpt-6-luna`. This lane's declared default is the underscore (wire) spelling and the
spec §6.1 correction records the mismatch; `docs/VERTICAL-SLICES.md` (not this lane's file) keeps the
hyphen spelling.

## Residual risks / follow-ups

- The canonical-request encoder in `internal/cognition/observation.go` (outside this lane's write
  scope) still embeds the legacy `DefaultModelID = zai/glm-5.3-flash`; the real arm's recorded wire
  model comes from the transport, so the recorded `request_bytes` model field and the recorded
  `model_id` differ until a later schema-versioned encoder edit. `docs/VERTICAL-SLICES.md` confirms
  the glm id is the superseded choice.
- An unpriced call charges nothing, so the cost ceiling cannot bind for the application model until a
  price is declared; the ADR addendum records this.
- `docs/VERTICAL-SLICES.md` (another lane's file) still carries the hyphen model id and the superseded
  glm reference; a reconciliation pass should align it.
- The live gateway metadata was not re-fetched in this lane (no HTTP to the gateway allowed); the
  discovered facts are the dispatch's, encoded in `DefaultGatewayModel`, `WireModeForEndpoints`, the
  probe, and the §6 correction. The disabled probe is the authorised later-lane verification path.
- The structural network guard was narrowed to permit `net` only in the new `gateway.go`; the guard
  still fails on `net` anywhere else and on any `internal/world` import.

## Commit

Committed on `feature/22-gateway-binding` (not merged to master): see `git log -1` on that branch.
