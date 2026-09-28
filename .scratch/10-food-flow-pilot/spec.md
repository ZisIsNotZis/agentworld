# Renewable food-flow pilot

Status: done

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

The synthetic format-v1 pilot uses 16 actors, two eight-slot patches, one-unit slot and bag capacity, initial energy 11/hunger 0/empty slots and bags, hourly production `q` per patch, one-unit elapsed-hour basal energy cost, and separate hunger state. At h=0 production occurs without a basal pulse, then actors start Gather at 0h+1µs and complete before h=1; h=1..167 applies basal to living actors and production before that hour's actor activities at h+1µs; h=168 applies basal to living actors only. Stopped actors retain their last energy and hunger. The first eight gathering opportunities are h=0..7, so q=3 yields exactly 24 produced units per patch during that window. Gathering capacity is eight independent slots/hour, not q; available stock bounds actual admissions. Gather transfers a unit with source-patch provenance into a bag; Eat consumes it before the next pulse. Blocked offers are unrealized inflow; energy-cap spill is separately accounted. Abundant `q=8` must retain at least 15/16 through 168h with no capacity rejection while all are eligible; scarce `q=3` must have at most 8/16 alive at 72h and exactly three successes per actor in the first eight hours while all claim; zero `q=0` must stop all by hour 11. These thresholds are predeclared before any implementation run. All branches require exact stock/held/energy accounting and deterministic worker-order independence; a horizon endpoint alone never passes.

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
- 2026-09-28 — assistant: Pure allocator seam matches stocked distinct slots with seed/hour/patch-rotating actor admission. Initial review `f121cad4-4891-44d8-a7e9-89695845c347` found unbounded input work and weak seed/rematch coverage. Worker added pre-allocation 16-stock/16-claim bounds, exact two-seed/two-patch winners, and non-prefix rematching tests. Fresh re-review `25c9070c-1a74-4f3e-a482-26a097ecff6a` found no issue (OK with notes). Parent ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully; committed as `846c426` on feature branch.
- 2026-09-28 — assistant: Runner implemented separate h0..168 production/basal/activity loop; initial integration exposed policy RestDuration=60m versus frozen 10m. Runner writer stopped; strategy-lane owner corrected pinned RestDuration to 10m and tests, then runner resumed and honored returned choice duration. q8 ended h168 with 16 alive/no capacity rejections; q3 h72 had 6 alive and exactly three gathers per actor h0..7; q0 stopped all h11; worker 1/4 event bytes and journals matched. Two fresh reviews `90f789ff-e5b3-436c-a734-15192c3a2c0b` and `bdb48b1a-6a09-419f-875b-3b3bf34c138f` found no source defect but requested a failed-step rollback test. Its first form cancelled too early; second used a deterministic post-evaluator context boundary, asserted 16 attempts/6 admissions, no state/journal/handoff change, and retry equivalence. Final review `005638eb-092c-466b-b445-0702ae4d1737` found no issue. Parent ran PATH-prefixed `timeout 180s make check`, `git diff --check`, and 20 focused race-test repetitions successfully. Committed as `a5dd5ff` on feature branch; persistence and full-horizon cost evidence remain outstanding.
- 2026-09-28 — assistant: Journal seam added bounded typed attempt/activity codec and scenario/head binding. Initial review `5a6cc6aa-13c9-4ec4-8032-949572123d3a` found open activities not reconciled with scheduler and incomplete Gather/Eat delta ownership checks. Worker added scheduler-aware complete decode/export/restore, marked history-only decode partial, checked complete actor/slot/source field sets and added replayable forged-Wait/foreign-actor tests. Fresh re-review `05bac4bd-3e17-46dd-ae8b-5e4c2a595e44` found no issue (OK with notes). Parent ran PATH-prefixed `timeout 180s make check` and `git diff --check` successfully; committed as `44ee4ab` on feature branch. Rejected reasons remain recorded evidence, not independent recomputation.
- 2026-09-28 — assistant: Checkpoint writer paused before mutation because FoodFlowBound exposed only actor/ref, not pinned policy content. Sequential strategy owner added value-copy Policy() and bounded canonical content encoding/verification; same-ref changed budgets or RestDuration fail against independently constructed expected policy. Fresh review `99c0b2bd-1d9a-4956-9c14-ff5567e10c0b` found no issue (OK with notes). First full `make check` timed out during world race suite at 180s; parent reran PATH-prefixed `GOMAXPROCS=4 timeout 180s make check` successfully. Committed as `00fe89c` on feature branch; checkpoint writer resumed with full-content verification.
- 2026-09-28 — assistant: Food-flow checkpoint candidate added distinct pilot five-section bundle, exact policy/fixture binding and cross-process continuation q8/q3/q0. Two fresh reviews `504f63d2-a2c2-420d-86ad-15498011fc77` and `28a1df6f-cddf-4b17-bc11-3012379734b2` found P1 acceptance of missing/extra non-completion wakes. Worker added pilot-specific exact topology validation on save/restore and validly re-encoded negative tests. Two re-reviews `895e7f95-f7f3-44c3-8351-3117e733f4a7` and `1064f326-d9ab-4daa-b074-a1827d0512c7` found no behavior defect, suggested h168 and extra-wake test coverage. Worker added those cases; final review `b7055dcf-8799-4b59-8dfd-ba394e1d6c35` found no issue (OK). Parent ran PATH-prefixed `GOMAXPROCS=4 timeout 180s make check` and `git diff --check` successfully; committed as `fd42832` on feature branch. Hashes are not adversarial signatures.

## Experiment evidence (working tree on `feature/10-food-flow-pilot`, base `fd42832`)

Reproduce from repository root with Go 1.24 and `GOMAXPROCS=4`. The PATH prefix below is this machine's ignored local toolchain; use your installed Go on a fresh checkout:

```sh
mkdir -p .tmp
PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" GOMAXPROCS=4 timeout 40s go build -o .tmp/foodflow-postmortem ./cmd/foodflow
for q in 8 3 0; do
  for seed in 0 7; do
    for workers in 1 4; do
      GOMAXPROCS=4 timeout 120s .tmp/foodflow-postmortem -q="$q" -seed="$seed" -workers="$workers" -hours=168 > ".tmp/foodflow-postmortem-q${q}-s${seed}-w${workers}.json"
    done
  done
done
```

The 12 **postmortem-revised** full-horizon JSON outputs (ignored `.tmp/foodflow-postmortem-q*-s*-w*.json`) and raw binaries/checkpoints remain ignored; previous `.tmp/foodflow-q*-s*-w*.json` files and their historical ticket values predate the alive-only basal correction and are **not valid evidence** for the current runner. Do not use any of these files as durable evidence if the working tree changes. JSON `status` explicitly says **synthetic/model-conditional**. Fixture/rule/projection format v1, pinned strategy `food-flow@1`, 16 actors, two home patches; no LLM call, `token_cost=0`. Each report checks every hourly mass/energy balance, accepted attempt event ID/key/time/source, rejected attempts' lack of event ID, and the frozen gates at h168. `checkpoint_digest` matches between workers 1/4 for each same (q, seed); per-seed hourly traces, action links, aggregate rejections and serialized section byte counts match. Different seeds select different scarce survivors and checkpoint digests. The postmortem-revised h168 checkpoint digests (identical for workers 1 and 4 with the same q/seed) are:

| q | seed 0 digest | seed 7 digest |
|---:|---|---|
| 8 | `6ae343f61d2212c3e68085cd27e3064bef704a99f541c7c50fc5d17694bd0722` | `8f8b92d5d7def60753e4a4bebdd6c727450e522196276d64f05d4f0a0b9f1e76` |
| 3 | `c7d7c2b49644086edc60d83bd4f5fc736a7bbe56346485ef45bd8e1a5f5e50d0` | `f66c2a690c56186932fc31f9172139f29f1b1014d85f17539c4882a34b1b956b` |
| 0 | `3094150b2edb833de1099f84038ab8758ed1f759603c62807175f87efc0e8784` | `b08897f41de3b715926dcde82d18bb57af098efbde8fd1eada93b8f86086e1c2` |

Digests detect corruption, not malicious replacement.

### Hourly trajectories and root cause

Notation: `stock/produced/gathered/consumed` is **per patch** (both patches have equal totals); `alive` counts energy > 0 at the completed hourly pulse, *before* that hour’s actor claims. Held food and energy spill are zero at all selected pulse boundaries; all 169 hourly checks satisfy produced = consumed + held + stock and initial energy 176 + consumed = energy + basal spent + cap loss. `h168` does basal only, no production or claims.

| q | h | alive (seed 0 / seed 7) | per-patch stock/produced/gathered/consumed | actor 1 E,H (seed 0 / 7) | actor 8 E,H (seed 0 / 7) | actor 16 E,H |
|---|---:|---:|---|---|---|---|
| 8 | 1 | 16 / 16 | 8/16/8/8 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 8 | 4 | 16 / 16 | 8/40/32/32 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 8 | 12 | 16 / 16 | 8/104/96/96 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 8 | 24 | 16 / 16 | 8/200/192/192 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 8 | 72 | 16 / 16 | 8/584/576/576 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 8 | 120 | 16 / 16 | 8/968/960/960 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 8 | 168 | 16 / 16 | 0/1344/1344/1344 | 11,1 / 11,1 | 11,1 / 11,1 | 11,1 |
| 3 | 1 | 16 / 16 | 3/6/3/3 | 11,1 / 11,1 | 10,1 / 11,1 | 10,1 |
| 3 | 4 | 16 / 16 | 3/15/12/12 | 8,4 / 9,3 | 7,4 / 8,4 | 7,4 |
| 3 | 12 | 16 / 16 | 3/39/36/36 | 3,9 / 4,8 | 2,9 / 3,9 | 2,9 |
| 3 | 16→17 | 16→6 / 16→6 | 3/51→54/48→51/48→51 | 1,11 / 1,11 | 0,11 / 1,11 | 0,11 at h17 |
| 3 | 24 | 6 / 6 | 3/75/72/72 | 1,11 / 1,11 | 0,11 / 1,11 | 0,11 |
| 3 | 72 | 6 / 6 | 3/219/216/216 | 1,11 / 1,11 | 0,11 / 1,11 | 0,11 |
| 3 | 120 | 6 / 6 | 3/363/360/360 | 1,11 / 1,11 | 0,11 / 1,11 | 0,11 |
| 3 | 168 | 6 / 6 | 0/504/504/504 | 1,11 / 1,11 | 0,11 / 1,11 | 0,11 |
| 0 | 1 | 16 / 16 | 0/0/0/0 | 10,1 / 10,1 | 10,1 / 10,1 | 10,1 |
| 0 | 4 | 16 / 16 | 0/0/0/0 | 7,4 / 7,4 | 7,4 / 7,4 | 7,4 |
| 0 | 10→11 | 16→0 / 16→0 | 0/0/0/0 | 1,10→0,11 / 1,10→0,11 | 1,10→0,11 / 1,10→0,11 | 1,10→0,11 |
| 0 | 12,24,72,120,168 | 0 / 0 | 0/0/0/0 | 0,11 / 0,11 | 0,11 / 0,11 | 0,11 |

At q8, 16 actors Gather and Eat independently every hour (2,688 of each across the run), no capacity denials, 16 remain alive h168. Per patch 1,344 produced = 1,344 consumed; at h168 no stock remains. At q3, exactly 8 claims and 3 successes **per actor** in h0..7 for both seeds; each early hour admits 3 per patch and rejects 5 per patch as `capacity` (170 rejected total before h17, not `no_stock`). At h1 the report now records 6 accepted Gather and **10 rejected Gather**, with 10 `capacity` reason counts; over the full run `rejected.gather=170` and `rejected.capacity=170`, so the action tally and reason breakdown agree. At h16 all 16 have E≥1; basal h17 stops 10 actors after repeated denied gathers. Seed 0 survivors h17/h168 are `[1,2,3,10,11,12]`; seed 7 `[1,2,8,9,10,11]`. The 3 survivors per patch then consume exactly 3 new units hourly and stabilize at E=1; dead actors remain at E=0,H=11 after h17 without further basal events. This irreversible death rule and loss of eligible competitors produce a survivor lock-in, **not** social adaptation. Seed rotation ensures the first-eight-hour fairness gate but does not imply long-run equity or historical realism. q3 ends 6 alive at both h72 and h168, with 1,008 Gather and 1,008 Eat linked events (2,016 accepted links), 170 capacity denials and 504 per-patch units produced/consumed. At q0 all 16 actors choose Wait during h0..10 (176 unlinked fallback attempts), energy decreases by one per pulse regardless of the failed food search, all stop on basal h11. No Gather/Eat occurs; world audit pulses continue to h168 but stopped actor bodies remain E=0,H=11 without further basal events. Zero resources are never created or consumed.

Concrete seeded lived links: q3 seed 7 actor 1 h0 Gather accepted event #3 and Eat #9, actor 8 Gather #5 and Eat #11, actor 16 Gather denied `capacity` (no event). At h16→17, actor 1 and 8 consume and remain E=1; actor 16 has E=0 and no further attempts. At h167 actor 1 Gather #3513 and Eat #3519, actor 8 Gather #3515 and Eat #3521; actor 16 remains stopped. q3 seed 0 actor 8 instead misses h0 and has E=0 by h17. For q8 actor 1/8/16 remain E=11,H=1 through h168 due to hourly units; q0 the same three decline to E=0,H=11 by h11. All links refer to the accepted event stream; the journal retains rejected reasons but cannot independently recompute a past denial from accepted events alone.

### Full-horizon cost and scope

All values below measured from the **final reporter source** in this working tree, `GOMAXPROCS=4`, each CLI process started separately. Wall and process CPU include runner construction, 168h execution, evidence verification, serialization and temporary checkpoint publication; exclude Go build and JSON emission. Event bytes are sums of canonical event bodies (not history framing); history/journal/checkpoint are serialized bytes. Linux `ru_maxrss` is high-water RSS, while `peak_heap_alloc_sampled_bytes` is only live Go heap sampled after each step and after serialization, so transient peaks may be higher. Environment/GC scheduling affect costs. Numbers are **not** P0 1,000-actor/decades estimates.

| q | seed | workers | wall ns | CPU ns | peak RSS bytes | sampled peak heap bytes | events | event body / history / journal / checkpoint bytes |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| 8 | 0 | 1 | 5,405,663,996 | 7,137,209,000 | 257,863,680 | 207,618,000 | 8,400 | 7,857,696 / 8,437,580 / 881,323 / 9,320,932 |
| 8 | 0 | 4 | 5,278,601,160 | 8,239,620,000 | 231,952,384 | 213,581,744 | 8,400 | same |
| 8 | 7 | 1 | 5,733,536,634 | 7,677,975,000 | 227,479,552 | 213,551,200 | 8,400 | same |
| 8 | 7 | 4 | 5,204,132,183 | 8,040,760,000 | 286,871,552 | 212,398,400 | 8,400 | same |
| 3 | 0 | 1 | 2,765,580,798 | 3,695,834,000 | 115,855,360 | 62,566,352 | 3,530 | 3,203,436 / 3,452,160 / 390,503 / 3,844,692 |
| 3 | 0 | 4 | 2,826,524,232 | 4,181,882,000 | 99,442,688 | 89,393,064 | 3,530 | same |
| 3 | 7 | 1 | 2,818,100,263 | 3,740,737,000 | 112,984,064 | 54,435,960 | 3,530 | same |
| 3 | 7 | 4 | 2,711,744,421 | 4,109,589,000 | 112,402,432 | 62,607,984 | 3,530 | same |
| 0 | 0 | 1 | 574,204,956 | 753,382,000 | 17,031,168 | 7,517,096 | 512 | 345,792 / 389,292 / 31,132 / 422,453 |
| 0 | 0 | 4 | 631,918,984 | 823,019,000 | 16,445,440 | 7,527,080 | 512 | same |
| 0 | 7 | 1 | 581,075,733 | 746,051,000 | 17,534,976 | 7,522,312 | 512 | same |
| 0 | 7 | 4 | 599,423,163 | 795,244,000 | 16,830,464 | 7,526,856 | 512 | same |

The highest observed peak RSS here is 286,871,552 bytes (~274 MiB) at q8 seed7 worker4; zero simulated token cost on a complete 168h run is by design (there is no LLM). `cmd/foodflow` tests exercise the three full gates, rejected Gather action/reason equality at h1 and h168, positional-argument rejection without report or checkpoint, report links/cost fields, no-clobber and worker-changed checkpoint resume. Actual separate-process q3 seed7 h18 **after** the deaths (checkpoint 568,999 bytes) and q0 seed0 h12 **after** extinction (183,246 bytes) each restore from workers 1 to 4 with the same h168 trajectories and checkpoint digests as uninterrupted runs (`f66c2a69…` and `3094150b…`). S6 CLI h1 checkpoint/resume (2 actors/2h, workers 1→4) still has `ReplayOK=true`, 2→4 events; its checkpoint format and behavior were not edited. The original validation paragraph from before the alive-only basal change is superseded; final quality-gate results for this revision are recorded below.

- 2026-09-28 — assistant (postmortem revision): Integrated review found the positional-argument CLI parsing hole and missing rejected-Gather action count, and independently corrected stopped-actor basal behavior in runner/journal. CLI now rejects non-flag arguments before world construction, with a built-CLI subprocess test proving nonzero exit, no report and no checkpoint for both leading and trailing stray args. Rejected Gather is counted once by action and independently by reason; q3 h1 has 10 rejected Gathers/10 capacity reasons, full q3 170/170. Re-ran all 12 full horizons after the runner/journal changes; table, examples, digests and costs above replace the invalid old baseline. No policy or P0-scale inference changed. Focused CLI/world tests passed, including q3 h18 cross-process continuation with 3,530 events and `f66c2a69…` final digest. `git diff --check` passed.
- 2026-09-28 — assistant: Fresh final integrated reviews `c9bd18c8-4adc-4f3c-a3d6-a083475b368a` and `c77f5482-3674-47d4-878b-328f1f51877b` found no remaining behavior defect (OK with notes). Parent fixed cold-checkout `.tmp` command setup and added h16 alive assertion to q3 checkpoint test. A cold `GOMAXPROCS=4 timeout 180s make check` expired during the world race suite after its ordinary tests passed; exact noncached `go test -race ./internal/world -count=1` then passed in 172.283s under a 480s cap, followed by `GOMAXPROCS=4 timeout 180s make check` passing all gates (world race 157.332s). `git diff --check` passed. Reviewed integration delta is atop feature commit `fd42832`; prior component seams each have separate fresh review evidence above. Remaining limitations: synthetic pre-social rules, rejection reasons not independently recomputable, digest not adversarial authentication, sampled heap may miss peaks, no P0 1,000-actor/decades claim.
