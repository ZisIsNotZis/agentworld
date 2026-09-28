# P0 survival vertical slice

Status: done

## Issue

S0–S5 prove state, events, scheduling, and one actor-safe intention, but there is no executable strategy or continuously running survival scenario. The first real vertical demo must show autonomous actors responding to hunger and finite resources over simulated time.

## Objective

Implement a bounded, typed, evolvable strategy representation with candidate generation, numerical utility evaluation, and bounded search; use it in a minimal survival scenario that wakes explicit actors, observes permitted state, attempts gathering/eating/resting or equivalent causal actions, updates hunger/energy/resources, records events and projections, and can replay its accepted state changes.

## Acceptance criteria

- Strategy is versioned executable data, not arbitrary Go callbacks or unrestricted host code; actor-specific parameters/overrides can differ without copying all shared nodes.
- Strategy reads only actor-visible observations and has a deterministic computation budget and safe fallback when no candidate is valid.
- At least one self-contained demo runs dozens of distinct actors through meaningful wake/action/outcome cycles and reports population/resource/health or hunger metrics, rejected attempts, elapsed simulated time, CPU time, token cost (zero for P0), and replay result.
- Behavior is not an authored sequence of outcomes; altering strategy parameters or resource seed produces a measurable, causal difference.
- Actor identity and finite resources remain consistent; no energy/resource creation via action arithmetic or rejected attempt.
- Tests cover strategy determinism, bounded search, ties, invalid/missing observation, strategy version/policy reference, resource contention, replay, and outcome sensitivity.
- Full quality gates and independent review pass before integration.

Need-review: yes
Need-test-cases: yes

## Blockers

None.

## Lane board

This is multi-seam: the reusable strategy-data contract and the world/demo integration are independently testable. Sequential writers share one checkout but have exclusive file ownership and a handoff boundary.

| Lane | Cwd / branch | Decision and claimed files | Authority and gate | Handoff |
|---|---|---|---|---|
| strategy | `agentworld`, `feature/07-survival-slice` | `internal/strategy/**` only: versioned data policy, bounded candidate evaluation, tests | Sole writer in first stage; focused strategy tests and `make check` | Public API, tests, limitations, exact revision/diff |
| integration | same checkout, after strategy exits | `internal/world/**`, `cmd/**`, scenario fixtures, scoped docs/ticket; use strategy public API, no strategy source edits | Sole writer in second stage; end-to-end demo, replay, sensitivity and full gates | Demo command/output, resource accounting, diff and risks |
| review | same checkout after writers exit | Entire scoped diff, read-only | Fresh independent review | Findings and verdict |

The integration lane may request a strategy-contract change through the supervisor; it must not silently edit the first lane's files. No concurrent mutation occurs.

## Comments

- 2026-09-28 — assistant: Opened after S5 integration at `325aa52`. This is the first user-visible causal survival demo, not a full P0 completion claim.
- 2026-09-28 — assistant: Classified multi-seam and partitioned sequential strategy and world/demo ownership before writer launch.
- 2026-09-28 — integration writer: Added separate trusted S6 survival adapter, dynamic hunger schema and projection, pinned format-v1 policy fixture with per-actor bindings, repeated hourly wakes, validated Eat/Rest patches, in-memory attempt journal, accepted-only replay comparison of event bytes/tip/scans/projections, and JSON CLI. Kept S5 runner and foundation/strategy package ownership untouched. Source additions: `internal/world/survival.go`, `internal/world/survival_test.go`, `cmd/survival/main.go`; scoped README, P0 spec and plan updated. Base `325aa52`; working tree contains the prior untracked strategy lane and ticket, with no staged files or commit.
- 2026-09-28 — integration writer: `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check` passed (fmt, probe, test, race, vet, build). `timeout 10s git diff --check` passed; `gofmt -l` empty on new files. `git diff --no-index --check /dev/null` emitted no whitespace warnings for each new Go file (returns status 1 for any nonempty diff). Focused `go test ./internal/world` passed. Reviewer gate remains open.
- 2026-09-28 — integration writer: `go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1` with Go 1.24 PATH and 90-second timeout passed: initial energy/food 192/124, final 213/54, 35 Eat, 14 Rest, 527 collisions, 49 dissipated, 48 alive, replay true. Paired `-eat-cost=0` passed: 0 Eat, 192 Rest, 48 ineligible rejections, 48 stopped, final energy/food 0/124, 192 dissipated, replay true. Process CPU/wall are measured per run and vary; no token spending. Test `TestSurvivalPolicyAndSeedSensitivity` also checks changed food and Eat count under weight change, and changed initial resources under seed change.
- 2026-09-28 — integration writer: Scope/review caveats: replay is accepted-state only; no durable attempt history, scheduler checkpoint, policy authentication, or LLM integration. Reported rule/seed metadata are scenario-local; seed PRNG is source-versioned. Pending independent review and parent integration verdict; status remains claimed.
- 2026-09-28 — revived integration writer: Reviewer P1 confirmed: a Rest at one energy on the configured final hour produced energy zero without a later wake or lifecycle Stop. Added `EffectStop{IfKey: proposal key}` after any scheduled next wake when a validated Rest from one energy is proposed; only a collision winner stops, including at horizon, and a losing Rest retains its retry wake. Regression covers winning and collision-losing Rest at both one- and two-hour horizons, and rejected-only zero-energy nonmutation; updated the old zero-energy test, which no longer expects a redundant post-Rest wake. Reviewer P2 resolved by JSON `wall_time_ns` and `cpu_time_ns` duration tags plus test and documentation. No edits to `internal/strategy/**` or foundation packages; reviewer follow-up verdict pending.
- 2026-09-28 — revived integration writer: `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check` passed (fmt-check, failure probe, tests, race, vet, build). Seed-7, 48-actor, 12-hour, four-worker CLI with `-eat-cost=1`: 35 Eat/14 Rest, 527 collision rejections, initial/final energy 192/213 and food 124/54, 49 dissipated, 48 with energy above zero, 12 simulated hours, accepted-state replay true, observed wall 42,244,111 ns and process CPU 62,582,000 ns. Paired `-eat-cost=0`: 0 Eat/192 Rest, 48 stopped, final energy zero/food 124, 6 simulated hours (no unnecessary zero-energy wake), replay true, observed wall 25,991,300 ns and CPU 38,620,000 ns. Performance values vary between runs; token cost zero. `timeout 10s git diff --check` passed; no staged files, commit or push.
- 2026-09-28 — revived integration writer: Representative lived trace via `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 70s go test ./internal/world -run '^TestSurvivalActorOneTrace$' -count=1 -v` passed after adjusting its assertion: actor 1 starts energy 3/hunger 5; hour 1 Eat cache 1001 accepted event 1 (energy 3→4, hunger 5→4, cache 33→31), Eats cache 1001 through hour 5 (energy 7→8, hunger 1→0, stock 25→23), then hour 6 Rest accepted event 16 (energy 8→7, hunger 0→1); Eats again at hours 7, 9 and 11 and Rests at 8, 10 and 12 (hour-12 energy 8→7, hunger 0→1). Twelve accepted actions over twelve wakes; 8 Eat, 4 Rest. Same demo includes collision losers; actor 1's chosen actions all won. Reference trace sampled from event deltas, not authored outcomes.
- 2026-09-28 — assistant: Fresh final review `fee7c5b4-b5ac-4f7b-9e06-3be2c8958a1e` verified final-hour lifecycle and timing units, no remaining issue, verdict OK with notes. Parent independently reran `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check`, 10 repeated world/strategy tests, both 48-actor CLI variants, and `git diff --check` successfully. Accepted-state replay is not proof of rejection-history or scheduler replay; the demo is a small causal mechanism experiment, not historical calibration.
