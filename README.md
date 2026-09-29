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

## Social-food v2 pilot (synthetic, model-conditional)

The separately versioned social-food v2 fixture adds private addressed food requests and independent donor consent on top of the same 16-actor/two-patch/168-hour shape: a never-gathered recipient may hold and eat a gifted unit with source provenance, and every gift is an atomic donor-bag−1/recipient-bag+1 transfer with zero aggregate food delta. `cmd/socialfood` never edits policy bytes: it publishes a verified neutral h0 checkpoint, records explicit enable/disable branch interventions in both children's manifest lineage, continues each child to `-hours`, and reports per branch.

```sh
mkdir -p .tmp
go run ./cmd/socialfood -q=3 -seed=0 -workers=1 -hours=24 -out=.tmp/socialfood-q3-s0-w1-h24.json
go run ./cmd/socialfood -q=3 -seed=7 -workers=4 -hours=24 -out=.tmp/socialfood-q3-s7-w4-h24.json
go run ./cmd/socialfood -q=8 -seed=0 -workers=1 -hours=24 -out=.tmp/socialfood-q8-s0-w1-h24.json
go run ./cmd/socialfood -q=0 -seed=0 -workers=4 -hours=12 -out=.tmp/socialfood-q0-s0-w4-h12.json
# Cost measurement of one full horizon on the enabled branch only:
go run ./cmd/socialfood -q=3 -seed=0 -workers=1 -hours=168 -branch=enabled -out=.tmp/socialfood-q3-s0-w1-h168-enabled.json
```

`-branch` selects `enabled`, `disabled`, or `both` (default); bundles (`social-neutral-h0.bundle`, `social-branch-enabled.bundle`, `social-branch-disabled.bundle`) are published into `-checkpoint-dir`, which must not exist (default: a fresh temporary directory that is kept and reported). The JSON goes to stdout and, if `-out` is given, to a previously absent file; existing outputs are never reused or overwritten, and stray positional arguments are rejected. Reports are labeled `synthetic/model-conditional` and include the neutral-root digest plus both branch digests (fork lineage), per-hour alive checkpoints, per-actor consumed/energy/hunger/survival and ledger totals, actor 1/8/16 hourly traces, witnessed request/gift/refusal/expiry counts with a privileged claim-versus-truth gift audit, directed dyad assistance fractions `(gifts+1)/(requests+2)`, per-patch consumed Gini (zero total is defined as zero inequality), gift donor costs in units, event/history/journal/checkpoint bytes and wall/CPU/peak-RSS costs with zero model tokens. When both branches run, a comparison section records alive-trajectory differences, survivor sets and donor costs. Full predeclared matrix results and limitations: [ticket evidence](.scratch/12-social-food-transfer/spec.md).

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
