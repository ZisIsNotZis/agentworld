# Ticket 31 — VS2 lane L2b (candidate feature derivations + escalation predicate)

Status: implemented on `feature/31-vs2-l2b`; **not merged** to master. Branch base `e9a0a79`
(the verified L4c tip = L4b + L4c, so the hermetic matrix can be re-run). Started
2026-10-13T23:36+08:00 (CST), actor: worker (sole writer). Code + hermetic tests + a hermetic matrix
run; no live model call, no spend.

## Objective

Declare and implement the VS2 candidate **feature derivations** the L1/L2 tickets explicitly deferred
(L1 ticket 25 recorded gap 1; L2 ticket 26 recorded gap 1), so the C.2 value model can rank: the exact
integer fixed-point derivations for `decision_sensitivity`, `urgency`, `impact`, `propagation_risk`,
`prior_reliability`, and `last_denial`, and the §4.3 `EscalationRequest` predicate body; version the
feature set `vs2-features/1` and bind it into the run identity so a report names it; and re-run the
hermetic VS2 matrix untuned to record the new VS2-EC-3 verdict.

## Contract (what this lane owns)

- Sole writer of `internal/world/natural_cognition.go` (roster-builder feature construction),
  `internal/cognition/protocol/**` (the §4.3 predicate, the observation-only feature projection, the
  feature-set version) and `internal/cognition/client.go` (the new model+feature identity digest).
- The owning spec `.scratch/24-vs2-attention/spec.md` (Appendix C.1 derivations, §4.2/§4.3 notes,
  Appendix A.2/C.2 `FeatureSet`, §11 operator-confirmable item).
- The additive `cmd/vs2` delta authorised by the supervisor (see below): one `feature_set` report field,
  one `model_feature_identity_digest` report field, and the `ValueModelSpec.FeatureSet` binding; every
  other cmd/vs2 behaviour and existing field byte-identical.
- This ticket; the L4c ticket log `.scratch/30-vs2-l4c/spec.md` (the cmd/vs2 delta record).

Explicit non-goals / preserved surface: no change to `cmd/vs1/**`, `internal/experiment/**`,
`internal/kernel`, `internal/scheduler`, `internal/component`, `internal/sim`, `internal/strategy`, the
frozen `natural/5..9` policy bytes, or the frozen v5–v9 evidence. No live model call, no spend.

## Declared derivations (implemented exactly as the owning spec now states)

Feature-set version `vs2-features/1` (`protocol.FeatureSetVersion`); fixed-point scale
`DerivedFeatureScale = 1000`; frozen-threshold constants restated in `protocol` and cross-checked
against the world's exported frozen constants in `internal/world`.

- `urgency` = `1000 · ((12 − energy) + hunger) / 36` — survival proximity; monotone non-increasing in
  `energy`, non-decreasing in `hunger`.
- `impact` = `1000 · (min(capital,2) + min(granary_stock,3)) / 5` — birth-gate / investment proximity.
- `propagation_risk` = `1000 · min(stocked_home_slots,8) / 8` — contested-slot risk.
- `decision_sensitivity` = `1000 · (Σ_τ max(0, 2 − |value_τ − threshold_τ|)) / 16` over the eight declared
  frozen-boundary pairs `(energy,5)`, `(energy,3)`, `(granary_stock,1)`, `(granary_stock,3)`,
  `(capital,2)`, `(hunger,24)`, `(stocked_home_slots,1)`, `(last_stored_meal_hour,hour)`.
- `last_denial` encoding: `""` / `"gather"` / `"build"`.
- `prior_reliability` = the declared neutral world-level aggregate `DeclaredPriorReliability = 500`
  (version `vs2-prior/1`): a declared, versioned, reversible default, the same for every candidate of a
  cell, never read by the predicate (EC-6).
- `escalation_requested` = the declared predicate `vs2-escalation/1` (observation-only, fails closed):
  request iff `urgency ≥ 500` **and** `decision_sensitivity ≥ 500`.

## Implementation

- `internal/cognition/protocol/features.go` (new): `FeatureSetVersion`, `DerivedFeatureScale`, the
  restated frozen-threshold constants, `PriorReliabilityVersion`/`DeclaredPriorReliability`,
  `ObservationFeatures(obs)` (the observation-only projection, including `last_denial`), and the
  derived helpers. It leaves `PriorReliability` and `EscalationRequested` zero, so it is strictly
  observation-only.
- `internal/cognition/protocol/priority.go`: `EscalationPredicateVersion`,
  `DeclaredEscalationPredicate()`, and `EscalationRequest(p, obs)` — the pure §4.3 predicate; it
  evaluates only observation-only rows and fails closed for `prior_reliability`/unknown/empty.
- `internal/cognition/protocol/valuemodel.go`: `ValueModelSpec` gains `FeatureSet`; it rides
  `WakePolicy.Identity()/Fingerprint()`, so the checkpoint cognition block binds the feature set
  (VS2-EC-5). A nil `Priority` is unaffected (VS1 bytes unchanged).
- `internal/cognition/client.go`: new `ModelFeatureIdentityDigest(model, wire, sampling, featureSet)` —
  a separate digest that folds the feature-set version into the model identity; `ModelIdentityDigest`
  is unchanged, so VS1's recorded model/sampling identity digest stays byte-identical.
- `internal/world/natural_cognition.go`: `naturalEscalationRoster` evaluates the declared predicate per
  claim-wake actor and `naturalCandidateFeatures` fills the projection, the declared world-level prior,
  and the request bit/reason; the removed `naturalFeatureInt` moved to
  `protocol.featureObservationInt` (one home).
- `cmd/vs2` (additive, supervisor-authorised; see below): the report names the feature set and the
  model+feature identity, and the declared value model carries the feature-set version.

## Tests added / updated

- `internal/cognition/protocol/features_test.go` (new): every derived row non-vacuous on a fixture; each
  declared direction (monotonicity); the `last_denial` encoding; `EscalationRequest` observation-only
  purity (it cannot read `prior_reliability`, an unknown row, or an empty predicate — all fail closed)
  and the declared predicate instance firing on a boundary-adjacent fixture.
- `internal/cognition/protocol/wake_test.go`: `TestWakePolicyIdentityBindsFeatureSet` pins that
  `ValueModel.FeatureSet` changes the policy identity/fingerprint (VS2-EC-5 binding).
- `internal/world/natural_cognition_priority_test.go`:
  `TestNaturalCandidateFeaturesObservationOnly` (updated for the new signature and the derived rows),
  `TestNaturalFeatureThresholdConstantsCrossCheck` (protocol constants vs the world's frozen constants),
  `TestNaturalEscalationRosterCarriesDerivedFeatures`.
- `cmd/vs2/e2e_test.go`: the report carries `feature_set = vs2-features/1` and a 64-hex
  `model_feature_identity_digest` equal to the recomputed digest.
- Existing pinned reductions retained and still passing: the C.2 all-zero-weight → uniform reduction
  (`valuemodel_test.go`), the nil-`Priority` VS1 byte-identity (`wake_test.go`), the world
  construction/gate/checkpoint tests.

## Recorded decisions

1. **Escalation-predicate instance and the prior are declared defaults while tasks 25/26 left them
   unspecified.** The predicate instance `vs2-escalation/1` and the neutral prior
   `DeclaredPriorReliability = 500` (`vs2-prior/1`) are declared in `internal/cognition/protocol` and
   evaluated by the world; both are declared, versioned, reversible defaults pending operator
   confirmation (spec §11.3 item 5), not tuned to any matrix outcome.
2. **`prior_reliability` is a declared constant aggregate at this revision.** No calibrated per-region
   statistic exists, so the row is the declared neutral cell prior; the versioned field and the
   derivation home make a future calibration a data change, not a contract change.
3. **The feature-set version is bound in two places.** It rides the policy/value-model identity (and
   therefore the checkpoint cognition block, VS2-EC-5) and a separate model+feature identity digest that
   the report names; the existing `ModelIdentityDigest` is untouched.
4. **`cmd/vs2` delta is additive only (supervisor-authorised).** Required because "consume only" left no
   path for internal code to make a report name the feature set without mutating `ModelIdentityDigest`,
   which would have changed VS1's recorded model/sampling identity digest. Exact delta (base `e9a0a79` →
   `5c56098`):
   - `cmd/vs2/config.go:59` — `FeatureSet: protocol.FeatureSetVersion` in `declaredValueModel()`.
   - `cmd/vs2/report.go:128-133` — the `model_feature_identity_digest` and `feature_set` fields on the
     `evaluation` report struct.
   - `cmd/vs2/main.go:154,162-163,182` — the feature-identity digest computation, the two populated
     report fields, and the feature-set token in `revisionString()`.
   - `cmd/vs2/text.go:18` — the text report line naming the model+feature identity and feature set.
   - `cmd/vs2/e2e_test.go` — the report assertions plus the `encoding/hex` import.
   Every other cmd/vs2 behaviour and existing report field is byte-identical.

## Requirements / acceptance

`go build ./...`, `go vet ./...`, `gofmt -l` clean, `git diff --check` clean; the targeted `internal/world`
and `internal/cognition` suites; `go test ./cmd/vs2/...`; and a fresh hermetic matrix run. The derivations
are **not** tuned to make VS2-EC-3 pass; the verdict is reported verbatim below.

## Commands and results (2026-10-13/14, CST)

Go toolchain is not on the ambient PATH: `GOROOT=/home/z/vibe/agentworld/.tmp/go-toolchain/root/usr/lib/go-1.24`,
`PATH=$GOROOT/bin:$PATH`, `GOTOOLCHAIN=local` (`go1.24.13 linux/amd64`).

| Command | Result |
|---|---|
| `go build ./...` | passed |
| `go vet ./...` | passed |
| `gofmt -l` over every tracked `.go` file | empty (clean) |
| `git diff --check` | clean |
| `go test -count=1 ./internal/cognition/...` | passed (`cognition 1.046s`, `protocol 0.004s`) |
| `go test -count=1 -timeout 20m ./internal/world/ -run 'Cognitive\|Cognition\|Priority\|Seam\|Feature\|Digest\|Checkpoint'` | passed `ok … 487.908s` (wall 8:08.75) |
| `go test -count=1 -timeout 20m ./cmd/vs2/...` | passed `ok agentworld/cmd/vs2 40.008s` |
| `.tmp/vs2-l2b -out .tmp/31-vs2-evaluation.json -out-txt .tmp/31-vs2-evaluation.txt -root .tmp/31-vs2-runs -recorded .tmp -src . -budget-min 50 -parallel 6` | exit 0, wall `459s` (7m39s); 60/60 declared cells ran, 36/36 priority replays identical |

### Hermetic matrix evidence (2026-10-13T23:54:02+08:00 → 2026-10-14T00:01:41+08:00, CST)

Report artifact `.tmp/31-vs2-evaluation.json`
`sha256 986bbbbde06408b83a8ba644e78ba36a7b38380cd6d79ce3356c29c979faca05`;
`.tmp/31-vs2-evaluation.txt`
`sha256 9ab67d41f98b595de4058586a01b16fc896f44b8cb2b8f285f94d02d8a549ec8`.

Revision binding (from the report): model `github_copilot/gpt-6-luna`, wire `responses`, model+wire
identity digest `66f1270482c7472ac66022c1c4d31ac48620b0e4129b065cb65ae11fde260a62` (unchanged from L4c);
model+feature identity digest `55f368ad603928e625c6f39d0a30ffa64c199c3b6365371b93b573f89a45919f`; feature
set `vs2-features/1`; base policy `natural/7`; mechanism `6`; `H=168`; `N=256`; value model `vs2-value/1`;
label `synthetic/model-conditional; hermetic FakeClient (no network, no authorised spend)`.

Realized call counts (call ledger, per arm over the 12 primary cells): R `1166..1512` (ceiling 4096), P
`243..256` (6/12 under budget), U `226..236` (12/12 under budget), A `226..238` (12/12 under budget). No
budgeted arm exceeded `N`; no ceiling halt; every budgeted cell reports the four ceiling rows. The
adaptive arm is no longer degenerate: its per-candidate predicted priority now varies (`predicted_min
2 482 000 000 .. predicted_max 6 430 000 000` across the A cells), where L4c reported a constant zero
projection.

Band statuses (verbatim, never tuned):
`[PASS] VS2-EC-1` (A0 byte-identical at all 12 cells); `[PASS] VS2-EC-2` (36 budgeted cells, no
over-ceiling, no halt); **`[clean-negative] VS2-EC-3`** — evidence verbatim:
`complete A/P/U cells=12/12; adaptive-dominates=6; clean-negative(mutually non-dominating)=6;
baseline-strictly-dominates-adaptive=0; non-dominating cells=f11-s1,f11-s3,f12-s0,f12-s1,f12-s2,f12-s3`;
`[PASS] VS2-EC-4`; `[PASS] VS2-EC-5` (36 priority cells replayed identical, zero model calls);
`[PASS] VS2-EC-6` (24738 recorded Role A requests scanned, no privileged marker; 3686 allocation
records scanned, no marker; import closure clean); `[PASS] VS2-EC-7` (A exploration sum 660 ≤ 12×64);
`[PASS] VS2-EC-8`; `[PASS] VS2-EC-9`.

**The new VS2-EC-3 verdict is `clean-negative`, not a PASS and no longer a FAIL.** At the L4c revision
the constant (all-zero derived) projection made adaptive the declared tie-break and a budgeted baseline
strictly dominated it in 8/12 cells (`[FAIL]`). With the derivations implemented, adaptive now weakly
dominates both baselines with strict improvement in 6/12 cells and is mutually non-dominating in the
other 6/12; **no** cell has a budgeted baseline strictly dominating adaptive (0/12). Per spec §8.4 / VS2
EC-3, the mutually non-dominating clean negative is a valid, decisive VS2-EC-9 outcome, not an EC-3
miss. The derivations were **not** tuned to reach this; the formulas are the declared meanings.

## Operator-confirmable item (spec §11/Appendix C.2)

The candidate feature derivations (`vs2-features/1`), the declared neutral `prior_reliability` aggregate
(`vs2-prior/1`), the `vs2-escalation/1` predicate body, and the C.2 integer weights (4/3/2/2/1/5) are a
**declared, versioned, reversible default pending operator confirmation** (spec §11.3 item 5). They are
recorded here so the default is explicit and reversible; no confirmation is claimed.

## Not accepted / what remains

The **central claim is not accepted**: this is a hermetic (FakeClient) mechanism/accounting proof only.
The real-model arm still awaits spend authorisation (spec §1.3/§9, VS2-EC-9), and the matrix result is a
clean negative at the declared substrate, not evidence that selective reasoning preserves fidelity.

## Revision binding

- Branch: `feature/31-vs2-l2b`; base: `e9a0a79` (L4c tip).
- Implementation commit: `5c56098` (the feature derivations + predicate + the additive cmd/vs2 delta).
- Spec commit: `2517e3a` (the owning spec's declared derivations/feature-set version/predicate).

## Revision addendum

- Ticket commit on `feature/31-vs2-l2b`: recorded below once this ticket is committed.
