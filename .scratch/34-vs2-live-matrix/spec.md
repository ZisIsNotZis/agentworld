# Ticket 34 — VS2 live matrix (the central claim, real-model arm)

## Wire constraint discovered at smoke gate (2026-10-16, first live smoke run)

Status: discovered and repaired under supervisor authorisation; the primary matrix had not started.
The first live smoke run (`-cells f12-s0 -arms P -live`, budget 30 min, run 1) FAILED the gate: all
256 Role A calls were recorded `http-error` (HTTP 400), zero tokens, and every Role A intervention
carried `result.status = "rejected"`, `reason = "malformed"`, `role_a.source = "fallback"`. The world
therefore ran the frozen cascade on every escalated decision — a fallback-only trajectory, exactly
what VS1 EC-2 names a falsifier, never evidence. The ledger recorded every refusal faithfully
(fail-loud behaviour working as declared); the report still emitted with 256 spent calls and 0
applied model actions, so the run is evidence of the constraint, not a VS2 result. The run-1
artefacts are preserved at `.tmp/34-vs2-smoke-run1-refused/`
(`calls.jsonl` sha256 `1ace3e1ca5d30a2b8e05bc1db8f12c61f8cd95a858bae894bab6adc9b87c48fe`,
evaluation JSON `afda972a9db14f360b15b37a5d6a1b771e5634b2d947a10228b4be47c4ad6f1e`, text
`6adc3905d2b6311928ddaf46f81803058f9519f76599f6eb2a02e2a158020e4a`).

**Root cause (decoded from the ledger's base64 response body, run 1 call 1):**

> `litellm.BadRequestError: Github_copilotException - {"error":{"message":"Unsupported parameter:
> 'temperature' is not supported with this model.","code":"invalid_request_body"}}` — model group
> `github_copilot/gpt-6-luna` failed; no fallback attempted.

The declared `FrozenSampling()` (`internal/cognition/client.go`, temperature 0.7, top_p 1, max_tokens
128) is emitted verbatim by the responses wire codec; the application model rejects `temperature`
on the `github_copilot` wire, and the gateway's per-model `drop_params: true` does not cover it.
This is a gateway wire constraint, not a spend or auth failure (gateway answered 200 on probes).

**Authorised repair (supervisor, minimal scope):** omit `temperature` from the outgoing responses-wire
request in `internal/cognition/gateway.go`; omit `top_p` too if a raw probe shows the same refusal.
Raw probes on `/v1/responses` (before the repair): (1) without `temperature`, with `top_p: 1` →
HTTP 400, verbatim `Unsupported parameter: 'top_p' is not supported with this model.`; (2) without
both, with a system prompt not containing the word `json` → HTTP 400, the `json_object` mode
requirement that input mention json (probe artifact only — the shipped `GatewaySystemPrompt`
does contain "JSON"); (3) without both, with the json-mentioning prompt → HTTP 200 `completed`,
usage flowing. So the repair omits BOTH `temperature` and `top_p` from `responsesRequestBody`
(pointer + omitempty, never set); `FrozenSampling()` itself is UNCHANGED (the declared instance
still rides every call record's `sampling` field and the model identity digest); the
chat-completions codec is untouched; no frozen evidence changed. Constraint recorded here as the
discovered wire fact: **the declared application model accepts no `temperature` and no `top_p` on
the responses wire; the declared sampling instance is therefore recorded-but-not-applied for the
two rejected fields, and `max_output_tokens` (from the frozen `MaxTokens: 128`) is applied.**
Three probe calls (≈200 tokens) were spent on the raw probes outside the harness and are recorded
here and nowhere else.

Also recorded (honesty caveat, no code change authorised): the report's static labels are hermetic
constants baked at L4c — `Status: "synthetic/model-conditional"`, `Revision: "…no authorised spend"`,
and the VS2-EC-9 evidence string "real-model arm: blocked on spend" — all three are inaccurate for a
live run. The true status of this ticket's runs (live, spend authorised, model
`github_copilot/gpt-6-luna` via the local gateway) is recorded here in the ticket; the acceptance
verdict belongs to the milestone review.

**Second wire constraint (reasoning output bound), same section, discovered at smoke run 2.** After the
temperature/top_p omission, smoke run 2 (`-cells f12-s0 -arms P -live`, 30-min bound) reached the
gateway (200s, ~2.1 s average, tokens flowing) but decoded 252/256 calls `malformed` (4 `ok` by
luck): every Role A action fell back — `role_a.source = "fallback"` on all 256 interventions — a
fallback-only run, not a live VS2 result. Root cause: `github_copilot/gpt-6-luna` is a reasoning
model; on `/v1/responses` the outgoing `max_output_tokens` bounds reasoning + text together, and the
frozen `MaxTokens: 128` is consumed entirely by reasoning (`reasoning_tokens: 128`), returning
status `incomplete` with no message text. Run-2 artefacts preserved at
`.tmp/34-vs2-smoke-run2-refused/` (`calls.jsonl` sha256
`ca454badae6a3cac8ff94b4d477f3fac90f116082aa441589d7f04d771ad194f`, evaluation JSON
`7677062c880b072fb666d129ef611adf21d5d8bf4524e0ea3d463ef3e9486c7b`).

**Authorised repair 2 (supervisor, option A):** the responses codec sends a fixed
`max_output_tokens: 512` (`responsesOutputTokenBound`, `internal/cognition/gateway.go`); probe
evidence with the exact recorded request payload: 512 → `completed`, 227 output tokens (207
reasoning + 20 text), valid schema JSON; 1024 → completed, 299; 128 + `reasoning:{effort:"low"}` →
still incomplete (the gateway ignores the reasoning parameter). 512 is ~2.3× the observed need. The
**applied output bound (512) now differs from the declared `FrozenSampling().MaxTokens` (128)**:
`FrozenSampling()` and every call record's `sampling` field stay byte-unchanged; the declared
instance records intent, the responses wire applies the approved 512 bound because the model+wire
bounds reasoning with the same field; the chat-completions codec still sends the declared 128. The
option to omit the bound entirely (model default 128 000) was rejected for per-call spend risk; a
different model and stopping the lane were rejected as unnecessary.

## Environment repairs before the smoke (2026-10-16)

The dispatch's environment facts were stale in three ways; each was repaired before the run and none
touched `internal/**`/`cmd/**` source (until the authorised gateway repair above):

1. **Go toolchain.** `.tmp/go-toolchain` was wiped with the rest of `.tmp`; no Go existed on the
   machine. Re-provisioned `go1.24.13 linux/amd64` from `go.dev/dl` into the recorded GOROOT path
   `.tmp/go-toolchain/root/usr/lib/go-1.24`, tarball sha256 verified against the published value
   `1fc94b57134d51669c72173ad5d49fd62afb0f1db9bf3f798fd98ee423f8d730` before extraction.
2. **Recorded ticket-19 evidence (VS2-EC-1 comparison target).** `.tmp/natural9-n7-f{10,11,12}-s{0..3}-w1.json`
   were wiped. Regenerated deterministically from the frozen substrate at `00de40f` with the exact
   ticket-19 command shape (`cmd/natural -mechanism v6 -policy natural/7 -r 8 -hours 168 -births both
   -founders f -seed s -workers 1`, patches default 2, one fresh checkpoint dir per cell), 12/12 OK.
   Substrate identity corroborated by the committed digest pins
   (`go test ./internal/world -run TestNaturalFounders -count=1` → ok, 68.99s). Residual, disclosed:
   the originals are gone, so byte-comparison against them is impossible; the regenerated files are
   the declared evidence home (`.tmp/**` is in this ticket's scope; VS1 spec §11.3 declared the
   no-AI-rerun fallback for exactly this loss).
3. **Gateway auth.** The local litellm (pid 174194, config `~/litellm.yml`) exposes
   `github_copilot/gpt-6-luna` (mode `responses`, only `/v1/responses`) with no master key; runs need
   no `LITELLM_API_KEY`. `LITELLM_BASE_URL=http://127.0.0.1:4000`,
   `LITELLM_MODEL=github_copilot/gpt-6-luna`.

Preflight at branch tip: `go build ./...` PASS (60s), `go vet ./...` PASS (44s).

Status: dispatched 2026-10-16T15:52+08:00, actor: supervisor, worker: sole writer on branch
`feature/34-vs2-live`. Operator authorised real-model spend in-session (2026-10-16); all §11.3
operator decisions were delegated to the agent and are recorded below as confirmed defaults.

## Operator decisions confirmed this ticket (supervisor, delegated authority)

1. **Spend gate PASSED.** Live VS2 authorised on the application model
   `github_copilot/gpt-6-luna` via the LiteLLM gateway (`$LITELLM_BASE_URL`, wire `responses`).
   Gateway health probed 2026-10-16 (model listed in `/v1/models`).
2. **Value model** `vs2-value/1`, feature set `vs2-features/1` incl. `vs2-escalation/1`:
   confirmed as declared (`.scratch/24-vs2-attention/spec.md` Appendix C).
3. **Call ceiling** `N = 256` for P/U/A. The `N = 512` budget-sensitivity extension is authorised
   as a SECONDARY lane: run only after the primary matrix and the §8.2 f=12 sensitivity-seed
   extension complete within the wall-clock budget; skipping it requires only a declared skip note.
4. **Period arm / quota schedule**: as declared in `.scratch/24-vs2-attention/spec.md` §5.1/§8.1.
5. **Exploration fraction** `q = 0.25`; no q-sensitivity sweep.
6. **Cost accounting WAIVED**: the gateway exposes no price entry for the application model; the
   equal-budget axis is realised call count only. Proxy cost is reported as a proxy, never spend.
7. **Acceptance** (VS2-EC-9): hermetic-only remains NOT accepted. The real-model matrix result —
   including a clean negative — is decisive once evidence is bound to a revision.

## Environment facts (verified 2026-10-16)

- Gateway `http://127.0.0.1:4000` (local litellm proxy running); `github_copilot/gpt-6-luna`
  served. `LITELLM_BASE_URL=http://127.0.0.1:4000`, `LITELLM_MODEL=github_copilot/gpt-6-luna`;
  the credential source is `$LITELLM_API_KEY` (gateway answered an unauthenticated models list;
  if a live call 401s, use the key the gateway process was started with and record nothing of it).
- Live gate: `VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND` plus `-live` (cmd/vs2/gateway.go).
- Go toolchain: `GOROOT=$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24`, `PATH=$GOROOT/bin:$PATH`,
  `GOTOOLCHAIN=local`.
- Repo history is a single commit `00de40f` (history was reset); branch from that point. The
  ticket-33 evidence tree is contained in it; ticket 33 recorded master gates at `48b4882`
  pre-reset — treat `00de40f` as the working baseline and re-run `go build ./... && go vet ./...`
  as the preflight check.

## Contract (worker — sole writer on `feature/34-vs2-live`)

Scope: `cmd/vs2` runs and their artefacts, `.tmp/**` evidence, this ticket, `CHANGELOG.md`,
`docs/VERTICAL-SLICES.md` (M2 status only), `.scratch/24-vs2-attention/spec.md` (status/revision
binding). Exclusions: no `internal/**` or `cmd/**` source edits unless a defect blocks the run (then
stop-and-report rather than improvise); no frozen v5–v9 evidence touched; no push without consent.

Steps:

1. Preflight: `go build ./...`, `go vet ./...`, branch from `00de40f`.
2. **Live smoke first (gate):** run ONE primary cell with `-live`, a budgeted arm (e.g.
   `-cells f12-s0 -arms P`), bounded output path. PASS = real gateway calls recorded in the
   intervention ledger, report emitted, no auth/wire errors. FAIL = stop and report; do NOT
   proceed to the matrix.
3. **Primary live matrix:** all 12 declared primary cells, arms `A0,R,P,U,A`, `-live`,
   `-budget-min` sized generously (declare the chosen value here before starting). Report artefacts
   to `.tmp/34-vs2-evaluation.{json,txt}` with sha256 recorded in this ticket. Bind revision
   (branch tip), model identity digest, seeds, wall time.
4. **§8.2 sensitivity-seed extension** (`-sensitivity`, f=12 seeds 4–7) under the same budget rule.
5. Secondary (authorised, may skip with a note): `N = 512` budget-sensitivity extension.
6. Record VS2-EC-1..9 statuses verbatim from the live report in this ticket. A0 must remain
   byte-identical to the recorded evidence (EC-1) — flag any deviation as a defect, not a result.
7. Update `CHANGELOG.md`, `docs/VERTICAL-SLICES.md` M2 row (mechanism + live evidence status,
   central claim accepted only if the declared acceptance rule in VS2 fires), spec status section.
8. Leave the acceptance verdict to the milestone review; the worker records evidence, not praise.

Budgets: preflight ≤ 15 min; smoke ≤ 30 min; primary matrix wall budget declared at start
(suggestion: `-budget-min 480`, `-parallel 4` to stay gentle on the local gateway); total lane
deadline 48 h from dispatch. No-progress rule: two consecutive failed cells with the same root
error → stop and report with logs and the failing request/response record.
