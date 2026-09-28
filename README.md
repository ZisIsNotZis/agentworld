# Agent World

Agent World is an experimental, event-driven civilization simulator.

Its goal is to maximize civilization-level causal fidelity under finite compute and language-model budgets. Every person persists as an individual actor in the initial design, but most behavior runs through inexpensive executable strategies. Language models wake selectively for novel, uncertain, or potentially high-impact decisions.

The project does not aim to replay recorded history exactly. It aims to produce alternative histories whose individual behavior, institutions, technologies, and macro-level outcomes remain causally explainable under explicit human, material, and informational constraints.

## Status

The project is in its P0 experimental phase. Go is the provisional implementation language. S0–S5 provide simulation primitives, immutable components, a kernel, scheduling and an actor-safe intention boundary. S6 adds a bounded autonomous survival demo. A single-machine P0 checkpoint now persists its accepted history, scheduler state and typed attempt journal; full P0 survival and language-model integration remain future work.

## Development

Go 1.24 or newer is required. The current foundation was validated with Go 1.24.13.

```sh
make fmt-check
make test
make race
make vet
make build
make bench
```

`make check` runs all gates except benchmarks. The module currently uses only the Go standard library.

Run the reproducible bounded demo:

```sh
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1
```

The JSON report records initial/final energy, food and hunger distribution, accepted actions and rejected reasons, one-unit-per-action dissipation, simulated hours, process CPU and wall durations in `cpu_time_ns` and `wall_time_ns` (nanoseconds), zero token cost, and accepted-state replay. Compare with `-eat-cost=0` or a different `-seed` to test sensitivity. Replay reconstructs accepted component state; the checkpoint additionally verifies typed historical attempts, policy bindings and pending scheduler state.

Create a checkpoint after five hourly steps, resume it in a new process (including with a different worker count), or fork it with a changed future Eat weight:

```sh
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=1 -eat-cost=1 -checkpoint-hour=5 -checkpoint=run.checkpoint
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1 -resume=run.checkpoint
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1 -resume=run.checkpoint -branch-eat-cost=0 -checkpoint=fork.checkpoint
```

Checkpoint paths must be new files in existing directories. `-eat-cost` on restore names the original run weight; `-branch-eat-cost` applies only after the verified prefix. The branch command publishes the fork boundary before continuing to the final report. `-checkpoint-hour` counts additional steps when resuming; omit it to continue until no wakes remain. Reports from checkpoint commands do not measure CPU/wall time (their timing fields are zero). The bundle digest and parent digest detect damage and identify ancestry but do not authenticate an adversarial rewrite; retain an expected digest independently if that matters.

## Synthetic renewable-food pilot (separate from S6)

The fixed food-flow v1 fixture has 16 actors, two eight-slot patches and 168 simulated hours. It is **synthetic, model-conditional** evidence about internal accounting and the declared strategy, not historical calibration or evidence of P0 performance at 1,000 actors/decades. `-q` is production **per patch per hour** (8 abundant, 3 scarce, 0 zero); `-hours` is an absolute reporting prefix of the fixed 168-hour fixture, not a new simulation rule.

```sh
go run ./cmd/foodflow -q=8 -seed=0 -workers=1 -hours=168
go run ./cmd/foodflow -q=3 -seed=7 -workers=4 -hours=168
go run ./cmd/foodflow -q=0 -seed=0 -workers=1 -hours=168
# Immutable checkpoint at completed h4, then continue in another process:
mkdir -p .tmp
go run ./cmd/foodflow -q=3 -seed=7 -workers=1 -hours=168 -checkpoint-hour=4 -checkpoint=.tmp/foodflow-h4.bundle
go run ./cmd/foodflow -q=3 -seed=7 -workers=4 -hours=168 -resume=.tmp/foodflow-h4.bundle
```

Use a **new** checkpoint filename for each publication. Without `-checkpoint`, the reporter creates and removes a measurement bundle under ignored `.tmp/`; a supplied `-checkpoint` without `-checkpoint-hour` publishes at `-hours`. The JSON includes h0, h1/4/12/24/72/120/168 when reached and population-change boundaries, patch stock/production/gathering/consumption, actor 1/8/16 energy and independent hunger plus prior-hour attempts, accepted event links, rejected Gather totals with separate denial-reason counts, and exact food/energy balances. Elapsed-hour basal need affects living actors only: once stopped, their energy and hunger remain fixed even while world production pulses continue. It reports wall/process CPU nanoseconds, Linux peak RSS, sampled peak live Go heap, serialized event-body/history/journal/checkpoint bytes and zero model tokens. Times include construction or restore, execution, verification and serialization/checkpoint publication but not `go run` compilation or JSON output. Full fixture results and limitations: [pilot evidence](.scratch/10-food-flow-pilot/spec.md).

## Start here

- [Vision](docs/VISION.md)
- [Design principles](docs/PRINCIPLES.md)
- [Conceptual architecture](docs/ARCHITECTURE.md)
- [P0 executable specification](docs/P0-SPEC.md)
- [P0 implementation plan](docs/P0-IMPLEMENTATION-PLAN.md)
- [Validation](docs/VALIDATION.md)
- [Roadmap](docs/ROADMAP.md)
- [Open questions](docs/OPEN-QUESTIONS.md)
- [Glossary](docs/GLOSSARY.md)

## Material limitations

- A plausible simulated society is not evidence that the same policy will work in reality.
- Historical calibration tests mechanisms and distributions, not exact event reproduction.
- Unknown natural laws cannot be validated by language-model judgment alone.
- Self-modification remains versioned, tested, auditable, and reversible.
