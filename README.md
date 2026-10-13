# Agent World

Agent World is an experimental, event-driven civilization simulator.

Its goal is to maximize civilization-level causal fidelity under finite compute and language-model budgets. Every person persists as an individual actor in the initial design, but most behavior runs through inexpensive executable strategies. The design reserves expensive reasoning for novel, uncertain, or potentially high-impact decisions; the language-model call path is implemented and used by both delivered slices of the active plan ([docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)) — VS1 (M1), whose mechanism is complete with hermetic evidence PASS on all eleven frozen exit criteria, and VS2 (M2, attention), whose value-based allocation mechanism is implemented and verified hermetically. The real-model arm of both slices remains blocked on spend authorisation, so the VS2 central claim — that selective spending preserves fidelity better than uniform or periodic spending at equal budget — is **not** accepted.

The project does not aim to replay recorded history exactly. It aims to produce alternative histories whose individual behavior, institutions, technologies, and macro-level outcomes remain causally explainable under explicit human, material, and informational constraints.

## Status

The P0 substrate is the experimental foundation, while the active plan is the vertical-slice roadmap ([docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md); VS1 delivered, VS2's attention mechanism delivered, VS2's central claim unaccepted). Go is the provisional implementation language. S0–S5 provide simulation primitives, immutable components, a kernel, scheduling and an actor-safe intention boundary. S6 adds a bounded autonomous survival demo. A single-machine P0 checkpoint now persists its accepted history, scheduler state and typed attempt journal. Full P0 survival is open. Language-model integration is **not** deferred future-phase work: it is the subject of the first slice (VS1) of the active plan ([docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)).

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

## Natural-limits v5 pilot (synthetic, model-conditional)

The separately versioned natural v5 fixture re-grounds production in land: each of P patches has 8 slots (wild or cultivated), the single natural production input is the patch regeneration budget R per patch-hour distributed cultivated-first-then-wild (remainder typed unrealized), granaries are bounded by declared spoilage (S=24) instead of a cap, births are cap-free with allocator-issued entity IDs, and the compute ceilings are the frozen kernel/scheduler/runner constants, instrumented hourly with a headroom ledger and a typed `NaturalCeilingHit` that halts fail-loud. `cmd/natural` never edits policy bytes: it publishes a verified neutral h0 checkpoint captured with BIRTHS DISABLED (the v5 h0 pulse is births-asymmetric by frozen design), records the births-disable (same-flag lineage) and births-ENABLE interventions as branch children sharing the parent digest (the disabling direction is refused by the persistence layer), and continues each child to `-hours`.

go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=both -out=.tmp/natural-A-r8-s0-w1.json
go run ./cmd/natural -r=0 -seed=0 -workers=1 -hours=12 -births=both -out=.tmp/natural-B-r0-s0-w1.json
go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=disabled -cultivation-disabled -out=.tmp/natural-D-ablation-r8-s0-w1.json
go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=enabled -founder-tenure -out=.tmp/natural-E-fixture-r8-s0-w1.json
go run ./cmd/natural -patches=32 -r=8 -seed=0 -workers=1 -hours=168 -births=enabled -out=.tmp/natural-F-ceiling-p32-r8-s0-w1.json

`-births` selects `enabled`, `disabled`, or `both` (default); `-founder-tenure` selects the persistence lane's declared founder-tenure fixture genesis (births-enabled-only; the only genesis under which organic births are reachable — its published root is the fixture's first pulse-boundary continuation, so trajectories start at h2); `-cultivation-disabled` selects the declared ablation intervention (births-disabled-only): the frozen policy is unchanged, the runner routes every Build choice through the documented denial loop, and any build event is a hard defect; `-policy` selects the frozen behaviour ref, `natural/5` (default, byte-compatible with every recorded run), `natural/6` — the ticket-16 surplus-behaviour policy over the unchanged v5 world — or the ticket-17 provision arms `natural/7` and `natural/7b`; `-mechanism` selects the declared world axis, `v5` (default, byte-compatible with every recorded run) or `v6` (the ticket-17 rule-515 "provision" world; `natural/7` and `natural/7b` require it and are refused before publication otherwise); `-founders` selects the ticket-18 declared founding band, the number of founder rows the genesis materializes from the frozen founder-stream prefix — `0` (default) is the default spelling of the full frozen block 8·patches and keeps every recorded run byte-identical, any other value must lie in 1..8·patches (range-checked through `world.NaturalFoundersValid` before any publication, with the founder-tenure fixture additionally refusing f=1 because its coherent pre-history seats founders 1 and 2). The policy ref, the mechanism and the founding band are run identity end to end (journal config, checkpoint manifest, bidirectional mismatch rejection). Under `natural/6` the reporter adds, purely additively, the ticket-16 report sections (per-actor trailing-24h surplus flow with its sustained-hours counter and time-to-first-sustained-surplus, capital persistence, time-to-first build/slot/birth, parent deaths within 24h of a birth payment, and the predeclared v6 bands A2/A3/A4 and B–F); under `natural/7` or `natural/7b` it instead adds the ticket-17 v7 sections — SF′ (provisions disclosed as withdrawals), the GATING per-actor sustained-12h counter with true running maxima alongside the non-gating hour-level union counter, `provisioned` totals per branch and per actor, paired meals per cell, time-to-first provision/banked hour/k≥2 hold/sustained surplus/birth filing/birth, the capital-persistence and parent-death blocks, the V7-A5 non-universality evidence, and the predeclared v7 bands V7-A2/A3/A4/A5 and V7-B–F; at a declared non-default founding band it adds the ticket-18 v8 density sections — the axis and `density_world = f/(8·P)`, per-patch seating with hourly mouths and per-patch Σk, per-actor surplus counters with home-patch attribution, the generation index and birth-cohort table (birth hour, parents, filing kin distance), the 8·births endowment ledger cross-checked inside G3, the parent-death and paired-meal canaries, per-patch time-to-first fields, and the predeclared v8 bands (V8-A2/A2b/A3/A4/A5/K for the organic cells, V8-B/V8-C2/V8-D/V8-E12/V8-F for the cell shapes, with the births-disabled branch carrying V8-C2 as its primary band). A natural/5 report at the default band carries none of the additive keys. Bundles (`natural-neutral-h0.bundle`, `natural-branch-enabled.bundle`, `natural-branch-disabled.bundle`, `natural-fixture-root.bundle`) are published into `-checkpoint-dir`, which must not exist (default: a fresh temporary directory that is kept and reported); the JSON goes to stdout and, if `-out` is given, to a previously absent file; existing outputs are never reused or overwritten, and stray positional arguments are rejected. Reports are labeled `synthetic/model-conditional` and include the root digest plus each branch digest (fork lineage), the population trajectory (alive-by-hour with births and cause-attributed deaths per hour and the extinction hour), the per-hour regen split (cultivated/wild/unrealized) with cultivated slots per patch, granary inflow/spoiled/stored aggregates, the Malthus index (trailing-24h Σmeals − Σbasal), per-actor ledgers (energy, capital, wip, granary, tenure count), typed event counts with restored accepted-history link checks, the frozen conservation identities re-verified independently at every hour (the mechanism-aware G2a′ form under `-mechanism=v6`), the full 14-row compute-ceiling headroom ledger, every typed `NaturalCeilingHit` with its P15 verdict classification (outgrew-harness vs design-defect vs implementation-defect vs runtime-defect, with the evidence fields the frozen table requires), and event/history/journal/checkpoint bytes and wall/CPU/peak-RSS costs measured against the predeclared v4-relative budgets (CPU ≤ 3× v4 per simulated hour, bytes ≤ 2× v4 per simulated hour; misses reported verbatim). A halting ceiling hit is a first-class outcome: the run records the hit, then runs a workers-4 twin (workers-1 when the cell ran at 4) truncated at the hit hour for replay equality. The frozen bands are evaluated verbatim in each report; a falsified band is reported, never tuned. Full predeclared matrix results, the A-band falsification, the ceiling verdict, and the recorded poverty trap: [ticket evidence](.scratch/15-natural-limits/spec.md); the ticket-16 v6 matrix, its green control gate, and the FALSIFIED verdict: [ticket-16 evidence](.scratch/16-surplus-behaviour/spec.md); the ticket-17 mechanism matrix, both green control gates, the FAILED honest null, and the self-falsifier outcome: [ticket-17 evidence](.scratch/17-surplus-mechanism/spec.md); the ticket-18 founding-band density curve and its predeclared V8 bands: [ticket-18 evidence](.scratch/18-surplus-via-reproduction/spec.md).

## Start here

- [Active plan (vertical slices)](docs/VERTICAL-SLICES.md)
- [Vision](docs/VISION.md)
- [Design principles](docs/PRINCIPLES.md)
- [Conceptual architecture](docs/ARCHITECTURE.md)
- [P0 executable specification](docs/P0-SPEC.md)
- [P0 implementation plan](docs/P0-IMPLEMENTATION-PLAN.md)
- [Validation](docs/VALIDATION.md)
- [Roadmap (historical phase plan)](docs/ROADMAP.md)
- [Open questions](docs/OPEN-QUESTIONS.md)
- [Glossary](docs/GLOSSARY.md)
- [Changelog (record of events)](CHANGELOG.md)

## Material limitations

- A plausible simulated society is not evidence that the same policy will work in reality.
- Historical calibration tests mechanisms and distributions, not exact event reproduction.
- Unknown natural laws cannot be validated by language-model judgment alone.
- Any self-modification must remain versioned, tested, auditable, and reversible.
