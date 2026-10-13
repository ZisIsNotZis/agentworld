# Ticket 33 — VS2 lane L5 (evidence record + current-state docs reconciliation)

Status: implemented on `feature/32-vs2-l5`; **not merged** to master. Branch base master `48b4882`
(the verified VS2 L2b merge). Started/worked 2026-10-14T00:55+08:00 (CST), actor: worker (sole writer).
Documentation-only: no code change, no live model call, no spend, no frozen evidence touched, no
independent review (L6 owns it).

## Objective

Record the verified VS2 delivery evidence in one durable ticket, and bring the current-fact documents
into agreement with the landed VS2 **mechanism** while keeping the VS2 **central claim** explicitly
unaccepted (spec `.scratch/24-vs2-attention/spec.md` §10 lane L5; VS2-EC-9). The mechanism is delivered
and verified hermetically; the central claim awaits the operator spend gate and the value-model
confirmation.

## Contract (what this lane owns — sole writer)

- This evidence record.
- `docs/VERTICAL-SLICES.md` (VS2 status), `README.md` (VS2/present-tense claims), `CHANGELOG.md`,
  `docs/adr/0007-value-based-cognitive-allocation.md` (status), and `.scratch/24-vs2-attention/spec.md`
  (status/revision binding).
- **Justified current-fact flips** of the ticket-23 VS2-ownership annotations that VS2 delivery
  falsified: `docs/ARCHITECTURE.md` §3/§4, `docs/P0-SPEC.md` §2/§10, `docs/PRINCIPLES.md` P7. Those
  three documents carried the ticket-23 annotation "**not yet implemented** — no `EscalationRequest`
  type exists; VS2 owns escalation requests" (and, in `ARCHITECTURE.md` §4, "not the implemented
  state"), created by ticket 23 (`.scratch/23-docs-reconciliation/spec.md`) precisely because VS2 was
  the named owner. VS2 has now landed that mechanism, so leaving the annotations would leave three
  conflicting false present-tense statements in current-fact SSOTs. The flips are minimal (the
  annotation sentence only) and reversible.
- Explicit non-goals preserved: no `internal/**` or `cmd/**` change; no frozen `.tmp`/v5–v9 evidence
  touched; no independent review; no claim of the central claim; no merge.

## Lane delivery on master `48b4882` (verified)

| Lane | Scope | Merge revision |
|---|---|---|
| L1 | additive priority protocol surface + integer value model | `8ecf4d0` |
| L2 | world escalation gate + checkpoint policy binding | `53b094a` |
| L3 | cognition wiring: online allocation accumulator, `allocation` records, price adapter | `c72ba9d` |
| L4a | extract shared `internal/experiment` mechanics + migrate `cmd/vs1` | `57770d2` (+ doc nit `e6d5466`) |
| L4b | recorded-seam replay/restore selector wiring | `b97ac5b` |
| L4c | equal-budget attention experiment runner `cmd/vs2` | `4a0ad28` |
| L2b | candidate feature derivations + `vs2-escalation/1` predicate | `48b4882` |

L4a byte-identically preserved the VS1 evidence: the `fidelity` array exact JSON bytes are identical
between base and branch, SHA-256
`fb49e0103b175169479a221cbf2a082d363e2b5979206d1e8c0490ecd8b896e6` (recorded in
`.scratch/28-vs2-l4a/spec.md`; the L4a review reproduced the byte-identity).

## Independent verifications (recorded PASSED)

- **L4a** — the reviewed tip's VS1 `fidelity` byte-identity was reproduced independently.
- **L4b** — three falsifying mutations were run and each was detected.
- **L4c** — equal-budget accounting and the real failing band were reproduced.
- **L2b** — 2771/2771 recomputed `expected_value` values matched the recorded projections; the feature
  derivations were judged faithful; the VS1 digest was unchanged.

## Combined-master gates at `48b4882` (recorded)

| Gate | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l` | empty (clean) |
| `git diff --check` | clean |
| `go test -timeout 30m ./...` | PASS, 13m43s (`internal/world` 814.994s) |
| `go test -race ./internal/cognition/...` | PASS |
| `go test -race -timeout 30m -run 'Cognitive\|Cognition\|Priority\|Seam\|Digest\|Feature\|Checkpoint\|Replay' ./internal/world/` | PASS 1222.951s, exit 0, no `DATA RACE` |

Go toolchain is not on the ambient PATH: `GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH`, `GOTOOLCHAIN=local` (`go1.24.13 linux/amd64`).

## Hermetic VS2 matrix (L2b revision, `FakeClient`, no spend)

Report artifacts (this revision):
`/home/z/vibe/agentworld/.tmp/31-vs2-evaluation.json`
`sha256 986bbbbde06408b83a8ba644e78ba36a7b38380cd6d79ce3356c29c979faca05`;
`/home/z/vibe/agentworld/.tmp/31-vs2-evaluation.txt`
`sha256 9ab67d41f98b595de4058586a01b16fc896f44b8cb2b8f285f94d02d8a549ec8`.

- Inventory: 60/60 declared cells ran; 36/36 priority replays identical (zero model calls); wall 459s.
- **Inventory scope.** "60/60 declared cells" is the **primary matrix only**: the 12 §8.2 primary
  cells × 5 arms (A0/R/P/U/A). The §8.2 sensitivity-seed extension (`f=12`, seeds 4–7), the §8.2
  optional `N = 512` budget-sensitivity extension and the §8.3 `w = 4` determinism twin were **not run**
  in this evidence record (no `-sensitivity`, no `N` override, no `w = 4`); no claim is made about them.
- Revision binding (from the report): model `github_copilot/gpt-6-luna`, wire `responses`, model+wire
  identity digest `66f1270482c7472ac66022c1c4d31ac48620b0e4129b065cb65ae11fde260a62`; model+feature
  identity digest `55f368ad603928e625c6f39d0a30ffa64c199c3b6365371b93b573f89a45919f`; feature set
  `vs2-features/1`; base policy `natural/7`; mechanism `6`; `H=168`; `N=256`; value model `vs2-value/1`.
- Realized call counts (per the L2b report artifact): R `1166..1512` (ceiling 4096); P `243..256`
  (6/12 under budget); U `226..236` (12/12 under budget); A `226..238` (12/12 under budget). Under-budget
  budgeted cells: **30/36**. No budgeted arm exceeded `N=256`; no ceiling halt; every budgeted cell
  reports the four ceiling rows.
  - **Dispatch reconciliation.** The dispatch stated arm A as `226..256` (10/12 under budget) and the
    under-budget total as 28/36; those are the **L4c** values (`.scratch/30-vs2-l4c/spec.md`,
    `.tmp/30-vs2-evaluation.json`). The **L2b** artifact bound above (hash `986bbbbd…`) records A
    `226..238` (12/12) and 30/36; this ticket records the L2b artifact values. The difference is the
    L2b feature derivations narrowing the adaptive arm's own realized stream.
- Realized-count note: the adaptive arm's 12 cells sum to 2771 realized calls (see disclosed limit b).

### VS2-EC-1..9 verbatim statuses (L2b run)

- `VS2-EC-1` **PASS** — A0 byte-identical at all 12 cells; nil-priority `WakePolicy` canonical identity
  bytes unchanged from VS1.
- `VS2-EC-2` **PASS** — 36 budgeted cells, `N=256`, over-ceiling 0, under-budget 30, missing ceiling
  rows 0, ceiling halts 0; construction refusal and the fail-loud `(N+1)`-th call are unit-exercised.
- `VS2-EC-3` **`clean-negative`** — verbatim: `complete A/P/U cells=12/12; adaptive-dominates=6;
  clean-negative(mutually non-dominating)=6; baseline-strictly-dominates-adaptive=0; non-dominating
  cells=f11-s1,f11-s3,f12-s0,f12-s1,f12-s2,f12-s3`. A clean negative is a valid, decisive VS2-EC-9
  outcome, not an EC-3 miss (spec §8.4).
- `VS2-EC-4` **PASS** — 36 fidelity rows × 8 indicators each (raw + §6-normalised first-divergence);
  scalar-winner/sentinel-inversion fixtures unit-exercised.
- `VS2-EC-5` **PASS** — 36 priority cells replayed byte-exact with zero model calls; mutated-policy
  refusal unit-exercised.
- `VS2-EC-6` **PASS** — import closure clean (request-builder/protocol/world); 24738 scanned Role A
  requests, 0 privileged markers; 3686 `allocation` records, 0 markers.
- `VS2-EC-7` **PASS** — A exploration sum 660 ≤ 12×64 = 768; P/U exploration empty; `q>0`/`q=0`
  fixtures unit-exercised.
- `VS2-EC-8` **PASS** — value model `vs2-value/1` bound in run identity and the allocation-record policy
  fingerprint; 36/36 budgeted cells with recorded candidate scores; float-free projection and the
  all-zero-weight uniform reduction unit-exercised.
- `VS2-EC-9` **PASS** — every report `synthetic/model-conditional`; the real-model arm labelled blocked
  on spend; a clean negative reported verbatim and accepted.

## Declared feature derivations (`vs2-features/1`, spec Appendix C.1/C.2)

`urgency = 1000·((12−energy)+hunger)/36`; `impact = 1000·(min(capital,2)+min(granary,3))/5`;
`propagation_risk = 1000·min(stocked,8)/8`; `decision_sensitivity = 1000·Σ max(0,2−|v−τ|)/16`
(eight pairs); `prior_reliability = 500` (declared neutral, `vs2-prior/1`); predicate
`vs2-escalation/1 = urgency≥500 ∧ decision_sensitivity≥500`. The feature set rides
`ValueModelSpec.FeatureSet` → `WakePolicy.Identity()/Fingerprint()`; a separate
`ModelFeatureIdentityDigest` names it; `ModelIdentityDigest` is unchanged.

## Operator gates (spec §11.3) — open

1. **Spend / budget gate.** Authorize the VS2 live matrix (or a single declared live smoke) and declare a
   price (or explicitly waive the cost ceiling and accept call/token equalization only). The application
   model is unpriced, so cost cannot be the equal-budget axis until then.
2. **Declared call ceiling `N`.** Confirm `N = 256` (and whether the `N = 512` budget-sensitivity
   extension should run).
3. **Periodic period and quota schedule.** Confirm `PeriodHours` and `q(h)`.
4. **Protected exploration fraction.** Confirm `q = 0.25` (and whether a sweep is wanted).
5. **Value-model feature set, derivations and weights.** Confirm the Appendix C.1/C.2 defaults
   (`vs2-features/1`, `vs2-prior/1`, `vs2-escalation/1`, weights 4/3/2/2/1/5) or name replacements.
   These are a declared, versioned, reversible default pending this confirmation.
6. **`cmd/vs1` migration vs. duplication.** Confirm the single-home extraction into `internal/experiment`.
7. **Acceptance without the real-model arm.** Confirm a hermetic-only VS2 is a declared **not accepted**
   state, and that a clean negative from a real-model matrix is accepted as decisive (VS2-EC-9).

## Disclosed limits (kept, not embellished)

- **(a) The central claim is NOT accepted.** The real-model arm awaits the operator spend gate, and the
  feature derivations/predicate/weights are an operator-confirmable reversible default (spec §11/C.2).
- **(b) The highest-weight row `escalation_requested` (weight 5) was identically 0 on all 2771 recorded
  observations.** It is a strict AND (`urgency≥500 ∧ decision_sensitivity≥500`) that never fired in this
  hermetic matrix, so the explicit-escalation path contributed nothing to the verdict. (2771 is the
  adaptive arm's total realized calls across its 12 cells; the count is verifiable from the report,
  while the identically-zero bit was verified in the L2b independent verification.)
- **(c) `last_denial` is the constant `""` at the claim seam** (unweighted), so it did not discriminate.
- **(d) Under-budget is common** — 30/36 budgeted cells at this revision — as the population and the
  claim-wake stream thin; every realized count is reported, never equalized (spec §5.3).
- **(e) The branch-child journal-export boundary** means the matrix checkpoints via
  `SaveResumeCheckpointWithReplaySelector` and exercises `ExportJournalWithReplaySelector` on a
  direct-genesis round trip; no `internal/world` change is made.
- **(f) `cmd/vs2/main.go` reads `protocol.FeatureSetVersion` directly** rather than
  `declaredValueModel().FeatureSet`. The two are equal at this revision; the direct read is a wiring
  detail, not a contract change.

## Files changed by this lane (hashes at this revision, working tree)

| File | git blob hash | Note |
|---|---|---|
| `docs/VERTICAL-SLICES.md` | `dba7d81576ca30b2eae3d8b0b7a8ab04a991d6d7` | VS2 status paragraph + M2 milestone row |
| `README.md` | `d13d71250789d16cdf4a846519dc7c40882806b8` | intro + status VS2 present-tense claims |
| `CHANGELOG.md` | `6e8cca27820d3781bbb205bc11d247a3c54b0b81` | events landed since the last recorded entry |
| `docs/adr/0007-value-based-cognitive-allocation.md` | `9f27c70c1f2974be5b432b9d3eb0cfda2a4414a1` | status line |
| `.scratch/24-vs2-attention/spec.md` | `e8b1b97ca4e4a57c7fe34452862631c147ef37dc` | status + implementation revision binding + design-v5 reviewed tip |
| `docs/ARCHITECTURE.md` | `fc1e6756e8d3669ee8fd8fe1ad85876f1d47db26` | §3/§4 annotation flip |
| `docs/P0-SPEC.md` | `435dd4eb42105e7b7b63eba6e1d15700d3347c26` | §2/§10 annotation flip |
| `docs/PRINCIPLES.md` | `bebb0d7994d5ffa8d96d4d180d89c2bd70e9b9da` | P7 annotation flip |
| `.scratch/33-vs2-l5/spec.md` | this ticket | new |

## Requirements / acceptance

`go build ./...`, `go vet ./...`, `gofmt -l` clean over every tracked `.go` file, `git diff --check`
clean, and every relative markdown link in the edited documents resolves. All were re-run at this
revision (see below).

## Commands and results (2026-10-14, CST)

Go toolchain: `GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH`, `GOTOOLCHAIN=local`.

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `gofmt -l $(git ls-files '*.go')` | empty (clean) |
| `git diff --check` | clean |
| relative-link check over the edited documents | every link resolves |

## Revision binding

- Branch `feature/32-vs2-l5`; base master `48b4882` (the verified L2b merge).
- Lane ticket commit: recorded in the revision addendum below.
- `git checkout master` after the lane work leaves the integration branch clean.

## Revision addendum

- Lane commit on `feature/32-vs2-l5`: `ed438efd42d9acabb1e2bef1d968e0411ca192d3` (VS2 L5 evidence record
  + current-fact docs reconciliation).
- This ticket's blob at that commit: `f9dca38a669fbe7804be90d57f3581bccbcf9ca1`.
- Addendum commit (records the lane commit hash): recorded in the handoff after commit.

## Not accepted / what remains

The **central claim is not accepted**. This lane records a hermetic (FakeClient) mechanism/accounting
proof only; the real-model arm still awaits the operator spend gate, and the clean negative at the
declared substrate is not evidence that selective reasoning preserves fidelity. Independent review of
this lane is L6's, not this ticket's.
