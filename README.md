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

## Productive-capacity v3 pilot (synthetic, model-conditional)

The separately versioned capacity v3 fixture adds private per-actor worksites and granaries to the same 16-actor/two-patch/168-hour shape: a build (20 minutes, one held bag unit) invests into work-in-progress, two builds complete one capital point (k ∈ [0,8]) that yields exactly one food unit per pulse into the owner's granary [0,8] and survives exactly six hourly depreciation pulses; wild production and v1 meal physiology are unchanged. `cmd/capacity` never edits policy bytes: it publishes a verified neutral h0 checkpoint, records the enable/disable intervention (and the founder-B endowment — actor 1 granary 4, enabled-only, because a capital endowment is impossible under the frozen h0 conservation) as branch children sharing the parent digest, continues each child to `-hours`, and reports per branch.

```sh
mkdir -p .tmp
go run ./cmd/capacity -q=8 -seed=0 -workers=1 -hours=168 -branch=enabled -out=.tmp/capacity-q8-s0-w1-enabled.json
go run ./cmd/capacity -q=5 -seed=7 -workers=4 -hours=168 -branch=both -out=.tmp/capacity-q5-s7-w4.json
go run ./cmd/capacity -q=0 -seed=0 -workers=1 -hours=12 -branch=both -out=.tmp/capacity-q0-s0-w1.json
# Recorded founder-B variant (enabled-only):
go run ./cmd/capacity -q=3 -seed=0 -workers=1 -hours=168 -branch=enabled -founder=B -out=.tmp/capacity-q3-s0-w1-enabled-founderB.json
```

`-branch` selects `enabled`, `disabled`, or `both` (default); bundles (`capacity-neutral-h0.bundle`, `capacity-branch-enabled.bundle`, `capacity-branch-disabled.bundle`) are published into `-checkpoint-dir`, which must not exist (default: a fresh temporary directory that is kept and reported). The JSON goes to stdout and, if `-out` is given, to a previously absent file; existing outputs are never reused or overwritten, and stray positional arguments are rejected. Reports are labeled `synthetic/model-conditional` and include the neutral-root digest plus each branch digest (fork lineage), hourly alive checkpoints with per-hour conservation re-verification, per-actor energy/hunger/capital/wip/granary stock/invested units/points created and decayed/yield total and unrealized/stored versus wild meals/survival hours, aggregate ledgers, the predeclared `surplus-flow-48h` window (hours 120–168 capital yield minus stored meals, clamped to the available hours below h168), per-patch wild-consumption Gini and capital-held Gini (zero total is defined as zero inequality), typed build/paired-meal/fallback and gather-denial counts, restored accepted-history link checks, event/history/journal/checkpoint bytes and wall/CPU/peak-RSS costs with zero model tokens. A disabled branch or a q0 cell recording any capital event fails the run. Full predeclared matrix results and the falsification verdict: [ticket evidence](.scratch/13-productive-capacity/spec.md).

## Reproduction-and-genetics v4 pilot (synthetic, model-conditional)

The separately versioned reproduction v4 fixture adds a runtime entity allocator, a heritable 3-locus integer genome (loci m/y/w, alleles [5,7], neutral 6, byte-identical to v3 at neutral), biparental consented birth gated by the frozen surplus indicator, aging with immutable lifetimes (96+draw mod 48), and kinship derived only from genome parent rows. `cmd/reproduction` never edits policy bytes: it publishes a verified neutral h0 checkpoint captured with BIRTHS DISABLED (the v4 h0 pulse is births-asymmetric by frozen design, so a disabled root is the only legal branch parent), records the births-disable (same-flag lineage) and births-ENABLE interventions as branch children sharing the parent digest (the disabling direction is refused by the persistence layer), continues each child to `-hours`, and reports per branch.

```sh
mkdir -p .tmp
go run ./cmd/reproduction -q=8 -seed=0 -workers=1 -hours=168 -births=both -out=.tmp/reproduction-q8-s0-w1.json
go run ./cmd/reproduction -q=5 -seed=7 -workers=4 -hours=168 -births=both -out=.tmp/reproduction-q5-s7-w4.json
go run ./cmd/reproduction -q=0 -seed=0 -workers=1 -hours=12 -births=both -out=.tmp/reproduction-q0-s0-w1.json
# Recorded genome-variant fixture (actors 1-8 locus-m=7, 9-16 locus-m=5; runner option, not a policy change; births-enabled-only):
go run ./cmd/reproduction -q=5 -seed=0 -workers=1 -hours=168 -births=enabled -genome-variant -out=.tmp/reproduction-gv-q5-s0-w1.json
# Equality fixture against a recorded ticket-13 capacity report (disabled neutral cells only):
go run ./cmd/reproduction -q=3 -seed=0 -workers=1 -hours=95 -births=disabled -v3-reference=.tmp/capacity-q3-s0-w1-enabled.json -out=.tmp/reproduction-equality-q3-s0-w1-h95.json
```

`-births` selects `enabled`, `disabled`, or `both` (default); bundles (`reproduction-neutral-h0.bundle`, `reproduction-branch-enabled.bundle`, `reproduction-branch-disabled.bundle`) are published into `-checkpoint-dir`, which must not exist (default: a fresh temporary directory that is kept and reported). The JSON goes to stdout and, if `-out` is given, to a previously absent file; existing outputs are never reused or overwritten, and stray positional arguments are rejected. Reports are labeled `synthetic/model-conditional` and include the neutral-root digest plus each branch digest (fork lineage), the population trajectory (alive-by-hour with births and cause-attributed deaths per hour and the extinction hour), genome diversity (integer-exact mean pairwise L1 over living actors, distinct genomes, per-locus allele frequencies at the frozen hours h1/4/12/24/72/96/120/144/168), kinship density (related living-pair fraction, mean finite kin distance, per-actor living children, inbreeding-refusal count), birth economics (births, filings, typed refusal histogram, privileged claim-versus-truth divergences), per-actor ledgers with genomes and lineage, typed event counts with restored accepted-history link checks, the extended frozen conservation re-verified at every hour, event/history/journal/checkpoint bytes and wall/CPU/peak-RSS costs with zero model tokens. A births-disabled branch or a q0 cell recording any birth evidence fails the run; q0 must be extinct by h11 on both branches. The declared founder fixture is part of run identity: checkpoint manifests and journal verification pin it, so a neutral root under a variant option and a variant root under the default option are both rejected with nothing published. Full predeclared matrix results, both falsification verdicts, and the defect/fix record: [ticket evidence](.scratch/14-reproduction-genetics/spec.md).

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
