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

## Secondary lane skipped by declared note: the N = 512 budget-sensitivity extension

Operator decision 3 authorised the `N = 512` extension as a SECONDARY lane, skippable with a declared
skip note. **Skipped, declared:** the runner CLI exposes no declared ceiling override —
`DeclaredCallCeiling` is a frozen constant (`cmd/vs2/config.go`, spec §11.2 forbids editing a
declared constant after its first run) and the flag set (`cmd/vs2/main.go`: arms, cells, sensitivity,
out, out-txt, root, recorded, src, budget-min, parallel, replay, live, wire) carries no `-ceiling`.
Running the extension would require a `cmd/**` source edit outside this ticket's authorised repair
scope (two gateway-wire repairs only), so the lane is not taken. No claim is made about `N = 512`.

Probe-call accounting (all live calls spent outside the harness, recorded nowhere else): 1
connectivity probe pre-repair-1 (≈ 92 tokens), 3 repair-1 probes (temperature/top_p/json_object,
≈ 200 tokens), 3 repair-2 probes (output bound 512/1024/128+low, ≈ 1 500 tokens). Harness runs:
smoke run 1 (256 refused calls, 0 tokens), smoke run 2 (256 malformed, ≈ 87 000 tokens), smoke run 3
(256 calls, ≈ 170 000 tokens), plus the primary matrix and the sensitivity extension declared below.

## Live smoke gate (run 3) — PASSED (2026-10-16)

Run 3 (post repair 2): `-cells f12-s0 -arms P -live`, `VS2_SPEND_AUTHORIZATION` env (value set for
the run only), `LITELLM_BASE_URL=http://127.0.0.1:4000`, `LITELLM_MODEL=github_copilot/gpt-6-luna`,
`-budget-min 30 -parallel 1`, binary built from commit `6006e98`. Wall ≈ 18 min (1075 s cell
duration), exit 0, report emitted. **Gate PASS:** 256 real gateway calls recorded in the hash-chained
ledger (240 `ok`, 16 `malformed` — reasoning-incomplete at the approved 512 bound, each recorded
fail-loud and counted as decision errors, indicator 8); 182 model actions applied (`role_a.source =
"model"`), 74 fallback (16 malformed + 58 other refused/inert reasons) — not fallback-only; no auth
or wire error. Priority replay `P-f12-s0: identical` (zero model calls, VS2-EC-5). Bindings: model
`github_copilot/gpt-6-luna`, wire `responses`, model identity digest
`66f1270482c7472ac66022c1c4d31ac48620b0e4129b065cb65ae11fde260a62`, model+feature identity digest
`55f368ad603928e625c6f39d0a30ffa64c199c3b6365371b93b573f89a45919f` — both identical to the recorded
hermetic binding (ticket 33). The cell's branch digest `c4c20c4b8e085a0836485ecf91affd33fce8785cafaa
6d159e9203052bf8f555` matches the regenerated ticket-19 recording for f12-s0 — the VS2-EC-1
continuity witness holds at this cell. Tokens: 102 563 prompt / 67 767 completion (reported, never
spend — the model is unpriced, cost accounting waived per operator decision 6). Smoke run-3
artefacts: `.tmp/34-vs2-smoke-evaluation.json` sha256
`94edee9dcfbe159306ea0e8e382ba295836310ed76eaf4d7672005d5bdfc479e`, `.tmp/34-vs2-smoke-evaluation.txt`
sha256 `24a78411045c2f0544ac93e3582ccf0c54b6d248ba8154ce57444662656cc5bf`, ledger
`6a625a3cecb1426faa34eadad344a17a40eccc6f2899b12b03138a496baca997`. Bands (partial smoke inventory,
recorded verbatim): EC-1 not-evaluable, EC-2 PASS, EC-3 not-evaluable, EC-4 not-evaluable, EC-5 PASS,
EC-6 PASS, EC-7 not-evaluable, EC-8 PASS, EC-9 PASS.

**Primary matrix declared before starting** (contract step 3): all 12 declared primary cells
(`f ∈ {10,11,12}` × `s ∈ {0..3}`), arms `A0,R,P,U,A`, `-live`, **`-budget-min 600`, `-parallel 4`**
(generous sizing above the 480 suggestion for measured live latency ≈ 4.1 s/call: R ≈ 12 cells ×
≈ 1 340 calls ≈ 92 min serial each ⇒ ≈ 3 waves ≈ 4.6 h, budgeted arms ≈ 2.5 h, replay + report
margin; total ≈ 7.3 h < 8 h bound; 48 h lane deadline governs). Root `.tmp/34-vs2-runs`, outputs
`.tmp/34-vs2-evaluation.{json,txt}`.

Primary matrix progress note (2026-10-16 ~19:40+08:00, ≈ 2.5 h into the 600-min bound): process
alive, 27 cell dirs created, 10 cells finalised, R wave 3 of 4 running. Sampled finished R cells:
1 330–1 602 realized calls each, ≈ 92 % `ok`, ≈ 8 % `malformed` (reasoning-incomplete at the approved
512 bound, recorded as decision errors), all far under the 4 096 reference ceiling — complete
references. Scheduling note: the runner's cell start order follows semaphore handoff, not spawn
order (observed: first acquirers were tasks 1, 3, 36, 59); harmless — cells are independent and
bands aggregate per cell; the declared inventory is what the report records.

## Primary live matrix — result (2026-10-16T17:12 → 2026-10-17T03:12+08:00, declared bound consumed)

Command (as declared above): all 12 primary cells × arms `A0,R,P,U,A`, `-live`, `-budget-min 600
-parallel 4`, root `.tmp/34-vs2-runs`, outputs `.tmp/34-vs2-evaluation.{json,txt}`, binary from
commit `3e9a878`. Wall 600 min (the declared bound expired during the replay phase and one cell);
exit 0; report emitted. **Inventory: 59/60 declared cells ran; 1 skipped, verbatim:**
`R-f11-s0: model unavailable: 3 attempts exhausted` — the budget-expiry context cancellation cut the
cell in flight at ≈ 687 calls (partial ledger preserved in its run dir; no final bundle). Its
P/U/A partners ran and are recorded with `reason: no-reference` rows.

Aggregate ledger (all 60 run dirs): 26 337 call records, 10 237 596 prompt tokens, 7 038 503
completion tokens — reported, never spend (unpriced model; operator decision 6).

Realized calls per arm (live): R 1 262–1 794 (ceiling 4 096; 11 cells, all complete references);
P 235–256 (one cell halted, below); U 226–256; A 226–256; under-budget budgeted cells 28/36 (the
population and claim-wake stream thin under live decisions — reported, never equalized, §5.3).

**VS2-EC-1..9 statuses, verbatim from the live report:**

- `VS2-EC-1` **PASS** — A0 byte-identical at all 12 cells (branch digests, event counts/body bytes,
  hourly population) against the regenerated ticket-19 recordings.
- `VS2-EC-2` **FAIL** — `budgeted cells=36 ceiling N=256 over-ceiling=0 under-budget=28
  missing-ceiling-rows=0 ceiling-halts=1 missing=none`. The one halt: `P-f10-s1`, typed
  `CognitiveCeilingHitError` on `cognitive-calls`. Anatomy (from its ledger, preserved): 256 call
  records = 253 logical calls + 3 retried transport timeouts (each 30.0 s, `status: timeout`,
  `attempt: 1`, retried once as `attempt: 2`); the budget charges every physical attempt while the
  P selector schedules logical calls, so the 257th attempted call was refused by the fail-loud
  ceiling — the §5.2 instrument firing on a live-only condition (the hermetic FakeClient never
  times out, so retries never occurred there). The cell's export was refused (short projection
  145/169 recorded). Verbatim band outcome: FAIL. The milestone review owns the reading
  (attempt-charged vs logical-call ceiling); no source edit was authorised or made.
- `VS2-EC-3` **FAIL** — `complete A/P/U cells=10/12; adaptive-dominates=0;
  clean-negative(mutually non-dominating)=9; baseline-strictly-dominates-adaptive=1; falsified at
  f10-s0`. On the live matrix adaptive NEVER dominates and is strictly dominated once (f10-s0). Per
  spec §8.4 this is the declared falsifying outcome, distinct from the clean negative: it falsifies
  the value model `vs2-value/1` as instantiated on this substrate+model at equal budget. The
  declared acceptance rule for the central claim does not fire.
- `VS2-EC-4` **FAIL** — `fidelity rows=32 each carrying 8 indicators …; incomplete=28`. All 32 rows
  DO carry 8 indicators; the band's completeness counter flags any row whose §6-normalised
  FirstDivergenceHour is < 1 — i.e. raw first divergence at hour 0 (normalised 0 by the declared
  `raw == -1 ? H+1 : raw` rule — legitimate for live runs where budgeted arms diverge from R at the
  first decision hour) plus the three `no-reference` f11-s0 rows. Anatomy recorded; verbatim band
  outcome: FAIL.
- `VS2-EC-5` **FAIL** — `priority cells replayed=35 mismatches=35`. The replay phase started after
  the budget expired: every replay task is recorded `skipped:budget` (the report's replay list is
  verbatim below), and the band counts non-`identical` outcomes as mismatches. This is an evidence
  GAP from the declared bound, not a replay divergence: live replay evidence exists only for the
  smoke cell (`P-f12-s0: identical`, zero model calls); the hermetic matrix carried 36/36 identical.
  The CLI has no replay-only mode; completing the 35 live replays would re-run the budgeted matrix
  (~9 000 calls) and was not authorised. Recorded as a disclosed gap for the milestone review.
- `VS2-EC-6` **PASS** — `import closure … violations=0; sentinel+recorded Role A requests
  scanned=24940 (recorded=24939) … privileged markers=0; allocation records scanned=3733, markers=0`.
- `VS2-EC-7` **PASS** — `A cells=12 reserved E=ceil(q*N)=64 recorded exploration sum=672; P/U
  exploration empty; violations=0` (672 ≤ 12×64 = 768).
- `VS2-EC-8` **PASS** — `value model vs2-value/1 … budgeted cells=36 with recorded candidate
  scores=35` (the halted cell's scores live in its ledger; the report counts 35).
- `VS2-EC-9` **PASS** — band emitted per its hermetic wording; see the honesty caveat above: the
  report's static `status`/`revision`/EC-9 strings are L4c hermetic constants and are WRONG for a
  live run. The true status of THIS run is: live, real gateway calls, spend authorised
  (VS2_SPEND_AUTHORIZATION set for the run only), model `github_copilot/gpt-6-luna` via
  `http://127.0.0.1:4000`, wire `responses`.

**Primary-matrix verdict inputs (for the milestone review, not praise):** the declared central-claim
acceptance rule does not fire — EC-3 is falsified at f10-s0 with adaptive never dominating on the
live matrix; the real-model arm is no longer blocked: it ran, and the result is the falsifying
outcome §8.4 declares reportable. Central claim: **NOT accepted; value model as instantiated
falsified at equal budget on f10-s0, mutually non-dominating elsewhere.**

Artefacts: `.tmp/34-vs2-evaluation.json` sha256
`83ced3b3e3911292c0051f5fce87b507608352437950ba7f7fc774d5a80dae7e`; `.tmp/34-vs2-evaluation.txt`
sha256 `ccf13a40f8b0b2c8646fbbf068f98988ec13f477ed090dca704043ac605b58ad`; per-cell run dirs with
hash-chained ledgers under `.tmp/34-vs2-runs/`. Bindings: revision (branch tip) `3e9a878` (source
fix commits `95c80b1`, `6006e98`); model identity digest
`66f1270482c7472ac66022c1c4d31ac48620b0e4129b065cb65ae11fde260a62`; model+feature identity digest
`55f368ad603928e625c6f39d0a30ffa64c199c3b6365371b93b573f89a45919f`; seeds: primary `f ∈ {10,11,12} ×
s ∈ {0..3}`, sensitivity `f=12 × s ∈ {4..7}`; wall 600 min declared bound (consumed).

## Sensitivity-seed extension — attempt 1 (partial; gateway outage) and attempt 2

Attempt 1 (`-cells f12-s4,f12-s5,f12-s6,f12-s7 -arms R,P,U,A -live`, `-budget-min 420 -parallel 4`,
root `.tmp/34-vs2-sens-runs`, binary from commit `85d422e`, run 2026-10-17T03:21 → 07:39+08:00):
exit 0, report emitted, **inventory 7/16 ran, 9 skipped verbatim** — `R-f12-s4`, `R-f12-s5`,
`R-f12-s6`, `R-f12-s7`, `U-f12-s5`, `U-f12-s6`, `A-f12-s4`, `A-f12-s5`, `A-f12-s6`, each
`model unavailable: 3 attempts exhausted`. Anatomy (ledgers preserved): the cells that started late
hit a transport timeout-storm (30 s timeouts, 3 attempts each, zero-byte responses) in the
07:2x–07:3x window; the litellm process did not restart (up since 2026-10-16T15:37), so the outage
was upstream/load, not local. Ran: P f12-s4..s7 all realized 256; U s4 (226) s7 (244); A s7 (256).
With all four R cells lost, the artifact computes zero fidelity rows: EC-1/EC-3/EC-4 not-evaluable,
EC-2 PASS (7 budgeted cells, 0 over-ceiling, 0 halts), EC-5 PASS (7/7 replays identical, zero model
calls), EC-6/7/8/9 PASS. Artefacts: `.tmp/34-vs2-sensitivity-evaluation.json` sha256
`367a44fc7242b49c2f9b27d40eead14988ca928b5d194aaa3f2a028d0422b225`, text
`e64587faa2162e7b7a808c6ce4aa05af70beaf207cffe01acea19c50f075e067`; failed-cell ledgers + report
preserved at `.tmp/34-vs2-sens-attempt1-partial/` (the runner would RemoveAll its root on a rerun).
The extension cannot inform the declared seed-robustness question (§8.2) without references, and the
no-progress root-error condition (gateway unavailable) has since cleared (probe: HTTP 200, 2.1 s).
**Attempt 2 declared before starting:** identical command shape, fresh root `.tmp/34-vs2-sens2-runs`,
outputs `.tmp/34-vs2-sensitivity2-evaluation.{json,txt}`, `-budget-min 420 -parallel 4`, binary from
the then-current branch tip. If attempt 2 fails wholesale with the same root error, the lane stops
and reports per the no-progress rule.

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
