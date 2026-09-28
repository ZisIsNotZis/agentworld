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
