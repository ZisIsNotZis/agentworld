# Ticket 34 — VS2 live matrix (the central claim, real-model arm)

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
