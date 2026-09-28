# Renewable food-flow pilot

Status: claimed

## Issue

S6's closed four-cache Eat/Rest scenario is an architecture smoke. At 256 actors it reliably ends by hour 28–34 because finite food is exhausted; one-winner-per-cache-hour conflict biases allocation toward low actor IDs; hunger is derived from energy rather than elapsed time. It cannot test a viable survival ecology or support P0 scaling inference.

## Objective

Build a separate versioned, synthetic 16-actor/168-hour food-flow scenario while preserving S6 and its checkpoints. Model renewable but finite production, access and gathering capacity, actor-held food, consumption, and elapsed-time physiological need. Ensure numerical resource/energy accounting, independent actor identity, bounded strategies, event-driven time, evidence and replay.

## Acceptance criteria

- Declare resource inflow, capacity, energy conversion, basal need, actor access, and uncertainty as typed numerical rules; log all production, gathering, consumption and losses with causal provenance.
- Multiple independent claims succeed in the same time window when accessible units and declared capacity suffice; no stock overdraw, duplicate reward, or systematic low-ID priority under symmetric claimants.
- An abundant mass-balance-feasible fixture keeps a predeclared survivor fraction through 168 hours; zero-inflow and scarce fixtures decline for inspectable reasons, with no artificial food creation.
- Need advances with elapsed time even after rejected intentions. Rest is a timed activity, not the sole source of starvation.
- At hours 1, 4, 12, 24, 72, 120, 168 and every extinction boundary, record population, energy/independent hunger, per-patch stocks/inflow/gathered/consumed, successful and rejected claims, and representative low/mid/high-ID trajectories.
- At every boundary, reconcile initial + produced − consumed − losses = current stocks and every actor-state change to events. Tests compare worker counts, seed variation, accepted replay, attempt links, and checkpoint continuation; changed rule/strategy/projection versions reject S6 checkpoint as incompatible.
- Record full-horizon wall/CPU, peak RSS/heap, event/journal/checkpoint bytes and token cost; measure rather than extrapolate to 1,000 actors or decades.
- Independent review and mandatory quality gates pass. A horizon endpoint alone is not acceptance.

Need-review: yes
Need-test-cases: yes

## Boundaries

This pilot is synthetic and pre-social: no birth, kinship, sharing, exchange, institutions, migration, historical calibration, LLM intervention, or claim of a plausible civilization. Preserve the S6 reference scenario and its history format; do not silently change its physiology or replay.

## Frozen pilot fixtures and lane board

The synthetic format-v1 pilot uses 16 actors, two eight-slot patches, one-unit slot and bag capacity, initial energy 11/hunger 0/empty slots and bags, hourly production `q` per patch, one-unit elapsed-hour basal energy cost, and separate hunger state. At h=0 production occurs without a basal pulse, then actors start Gather at 0h+1µs and complete before h=1; h=1..167 applies basal and production before that hour's actor activities at h+1µs; h=168 applies basal only. The first eight gathering opportunities are h=0..7, so q=3 yields exactly 24 produced units per patch during that window. Gathering capacity is eight independent slots/hour, not q; available stock bounds actual admissions. Gather transfers a unit with source-patch provenance into a bag; Eat consumes it before the next pulse. Blocked offers are unrealized inflow; energy-cap spill is separately accounted. Abundant `q=8` must retain at least 15/16 through 168h with no capacity rejection while all are eligible; scarce `q=3` must have at most 8/16 alive at 72h and exactly three successes per actor in the first eight hours while all claim; zero `q=0` must stop all by hour 11. These thresholds are predeclared before any implementation run. All branches require exact stock/held/energy accounting and deterministic worker-order independence; a horizon endpoint alone never passes.

Multi-seam implementation; one writer per checkout at a time, each stage exits and hands off its tested public contract before the next writer. S6 files and checkpoint bytes are out of scope.

| Lane | Exclusive files / decision | Gate and handoff |
|---|---|---|
| domain | `internal/world/foodflow_contract*.go` | Typed IDs, rules, projections, exact fixture and conservation tests |
| policy | `internal/strategy/foodflow*.go` | Pinned value-only bounded choices; deterministic tests |
| allocation | `internal/world/foodflow_allocation*.go` | Injective slot matching, rotation/fairness and capacity tests |
| runner | `internal/world/foodflow.go`, runner tests | Hourly pulses, activities, accepted/rejected evidence, trajectories, full-horizon fixtures |
| persistence | `internal/world/foodflow_journal*.go`, `foodflow_checkpoint*.go` | Typed journal, distinct checkpoint manifest, cross-process continuation |
| experiment | `cmd/foodflow/**`, scoped docs and ticket | Reproducible commands, early/middle/late traces and full-run costs |
| review | Entire integrated change, read-only | Independent findings and disposition |

A contract change crossing ownership returns to the parent for reassignment; no writer silently edits another lane's files. The parent owns integration, benchmark interpretation and acceptance.

## Blockers

No user-only blocker. The design packet from read-only run `c5f86796-7d1d-450c-835b-6a46f0e52447` fixed the causal contract and stage topology; implementation remains unproven.

## Comments

- 2026-09-28 — assistant: Opened from diagnosis ticket 09 at master `31be3e1`. The default next experiment is a model-conditional survival ecology before adding social interaction; this is a reversible technical sequencing choice within the approved project goal.
- 2026-09-28 — assistant: Read-only Designer specified slot-based production/claims, rotating admission, elapsed-time need, timed activities, version isolation, fixed abundant/scarce/zero gates and separate source seams. Recorded these before any implementation run.
- 2026-09-28 — assistant: Resolved domain-worker scope ambiguity before implementation: retain Designer's initial energy 11 and zero-flow extinction at h=11; pin integer-hour pulse/production order and bag source provenance. The earlier worker-suggested energy 10 was rejected because it would silently change a predeclared gate.
- 2026-09-28 — assistant: Clarified first opportunity is h=0+1µs, not h=1; h0 supply is gathered before h1 production. Thus q=3 over h=0..7 yields 24 units per patch and the predeclared three-successes-per-actor fairness gate without inventing a q-based gathering cap.
- 2026-09-28 — assistant: Domain seam implemented separately in foodflow_contract files. A first review found same-hour gather-consume-gather bypass and per-slot conservation/test gaps; worker added typed LastGatherHour (`-1` sentinel) to each actor and includes it in atomic Gather proposal, per-slot pulse bound, and post-commit eight-claim reconciliation. Fresh re-review `e97eac37-c37a-4c4a-ae21-b549cdfcd28c` found no remaining issue (OK with notes). Parent ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully; committed as `8cefb5b` on feature branch. Runner must derive hour from authoritative scheduler time and use the guard in the same proposal.
- 2026-09-28 — assistant: Strategy seam implemented in isolated `internal/strategy/foodflow*.go`: pinned value-only Eat/Gather/Rest/Wait choices, actor binding, deterministic slot ties and budget-overflow Wait. Fresh review `5fff7bd3-ed05-48ff-b34f-a2b04fae9e1e` found no defect (OK with notes); optional P2 test could explicitly mutate the source policy after registration. Parent ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully; committed as `52a15c5` on feature branch. Runner must construct actor-visible observations and independently revalidate physical preconditions.
- 2026-09-28 — assistant: Pure allocator seam matches stocked distinct slots with seed/hour/patch-rotating actor admission. Initial review `f121cad4-4891-44d8-a7e9-89695845c347` found unbounded input work and weak seed/rematch coverage. Worker added pre-allocation 16-stock/16-claim bounds, exact two-seed/two-patch winners, and non-prefix rematching tests. Fresh re-review `25c9070c-1a74-4f3e-a482-26a097ecff6a` found no issue (OK with notes). Parent ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully; runner must feed one authoritative snapshot and form distinct-slot atomic proposals.
