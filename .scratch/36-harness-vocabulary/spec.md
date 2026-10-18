# Ticket 36 — harness vocabulary and comprehension test

Status: dispatched 2026-10-18, actor: supervisor, worker: sole writer on branch
`feature/36-harness-vocabulary` (branched from `master` at `8269765`).

## Why this ticket exists

Ticket 35 measured the defect: in the v1 matrix **40–53 % of every arm's model answers were rejected as
inadmissible** (A 321/420, P 338/359, U 258/486), and 882 of 917 rejections proposed `eat-stored` with no
granary food or the hour's draw already used. The request the model received was a bare data dump: no
task, no goal, no field glossary, no action semantics, a policy-constant vector nobody asked for, and
four action names that actively mislead (`gather` does not gather from the world, `provision` does not
provision the granary).

## The four code facts this ticket builds on (verified in the code before any change)

| action | what the world actually does | admission (the ladder, `internal/cognition/rolea.go`) |
|---|---|---|
| `gather` (rule 506, `CapacityGather`/`NaturalGatherFor`) | transfers the **1 unit sitting in one of the actor's home slots into the bag**. Wild food arrives by the world pulse (`CapacityProduceWild`), never by this action. | some home slot `stock == 1`; bag `0`; `energy > 0`; `LastGatherHour < hour`; the slot's `Gathered <= hour` |
| `provision` (rule 515, `NaturalProvision`) | **withdraws 1 granary unit into the bag**: `stock−1, provisioned+1, bag+1`, body untouched. It does **not** stock the granary. | bag `0`; `granary >= 1`; `LastStoredMealHour < hour` |
| `eat-stored` (rule 509, `NaturalEatStoredFor`) | **eats 1 granary unit directly**: `stock−1, stored meals+1`, bag untouched. | `granary >= 1`; under mechanisms v6/v7 `LastStoredMealHour < hour` |
| `wait` | nothing | always admitted |

The shared guard is real: `NaturalEatStoredFor` (`m != NaturalMechanismV5 && LastStoredMealHour >= hour →
ErrNaturalContract`) and `NaturalProvision` (`LastStoredMealHour >= hour → Err`) both refuse a second draw,
so **at most one granary draw per hour of either kind**, and `wait` is the only always-legal action. The
live fixture runs mechanism 6 (v6), so both guards are active.

**Correction to the dispatch, verified before building on it: `hunger` is not the starvation clock.** The
dispatch asked for `hunger` to be renamed to a name saying it is the starvation clock, "verify exact
semantics first". Verified: `NaturalBasalFor` sets `starved := a.Body.Energy == 0`, so **energy is the death
channel**; `hunger` rises 1 per elapsed hour, is capped at `NaturalHungerCapacity = 24`, falls 1 per meal
(wild or stored), and never kills anything. It is a meal-deficit counter. Acting on the verification rather
than on the suggested name is the point of the check, and the ticket records both the check and its
consequence (below).

## Deliverable A — the declared vocabulary `natural-claim/v4`

`natural-claim/v4` is a new contract (`ClaimContractV4`, schema version 4). The internal IDs of the four
claim actions, the world's action set, and every recorded ledger are **unchanged**, so replay and strategy
fingerprints are unaffected. `v1` and `v2` keep their exact previous bytes when selected (pinned by
`TestActorSeamRequestV1AndV2BytesAreUnchanged`). v4 does **not** state the admissible action set: that is
`v2`'s shortcut and stating it again would make the comprehension test measure nothing.

### The request, before and after

| v1 key | v4 key | why the name changed |
|---|---|---|
| `energy` | `energy` | already states the fact; it is the survival resource and the death channel |
| `hunger` | *(dropped)* | see the inert-row audit below |
| `bag_units` | `bag_food_units` | says what is in the bag |
| `capital` | `capital` | already states the fact |
| `wip` | `build_point_in_progress` | `wip` is jargon; the fact is progress toward one capital point |
| `invested_units` | *(dropped)* | inert-row audit |
| `last_build_hour` | `build_done_this_hour` | a timestamp no decision reads; the gate's outcome is what matters (a gathered unit cannot be built again in the same hour) |
| `granary_stock` | `granary_food_units` | says what the number counts |
| `last_stored_meal_hour` | `granary_draw_used_this_hour` | the affordance it gates (one granary draw per hour), not a timestamp |
| `home_patch` | *(dropped)* | inert-row audit |
| `home_slots[{slot,stock}]` | `food_units_waiting_to_pick_up` | the per-slot ids are unusable (gather always takes the lowest-numbered stocked slot and the model cannot name one) and the array's sum is the same fact |
| `last_denial: "gather-denied"` | `last_refused_action: "take_slot_food_into_bag"` | the value named an internal ID; it now names the action in the declared vocabulary |
| `strategy.parameters` (7 values) | *(dropped)* | policy constants of the actor's bound cascade, not state. The recorded head ref identifies them (frozen base vector plus the generation's deltas), so no provenance is lost |
| `run.identity_digest` | *(dropped)* | it is a hash of the `run` block the request already carries, i.e. the one field derivable from other fields in the same request |
| `actions: ["gather", …]` | `actions: ["take_slot_food_into_bag", …]` | the display labels |

### The redundancy and inert-row audit (hard rule, every row checked against the code that reads it)

The rule is: no field may be derivable from another field in the same request, and no field that neither
the model nor the world acts on belongs in the decision request. What was dropped and why:

1. **`hunger` — dropped as inert.** `NaturalBasalFor` decides death from energy alone; the frozen cascade
   validates hunger and then discards the value (`_, hungerOK := capacityInteger(obs.Hunger, …)` in
   `natural6.go`/`natural9.go`), no claim-admission gate reads it, and no action's effect depends on it. It
   remains a world row, a checkpoint field and an allocator feature row (`ObservationFeatures`), where it
   does drive *who* is escalated; it does not drive *what the actor does*. Under the supervisor's rule it
   therefore leaves the model-visible request and stays the ledger fact it is.
2. **`invested_units` — dropped as inert.** Read only by the frozen cascade's validity check (`_,
   investedOK := …`); it is a conservation ledger row (`invested = 2·created + wip`), not a decision fact.
3. **`home_patch` — dropped.** An actor has exactly one home patch and no claim action can address another,
   so the id decides nothing. It stays in `strategy.NaturalObservation` for the frozen cascade's own
   validity check.
4. **`identity_digest` — dropped as derived.** `RunIdentityDigest` is a hash of `fixture`, `seed`,
   `base_ref`, `head_ref`, all of which the request keeps; two homes for one fact, so the derived one goes.
   Provenance holds: the digest is recomputable by the recorded `RunIdentityDigest` from the request alone.
5. **per-slot rows and slot ids — collapsed to one count.** Eight rows of `{slot, stock}` state one fact
   (how much food is waiting) plus an id the action cannot address.
6. **`strategy.parameters` — dropped.** Seven policy constants; measured as 40 % of the v1 request's
   characters and read by no decision.
7. **`last_build_hour` and `last_stored_meal_hour` — replaced by the gated booleans**, not added to them:
   the timestamp and its gate outcome are the same fact, so only the outcome survives.

The other fields are kept because something acts on them: `energy` (death, meal, build floors), `capital`
(birth gate and build cap), `bag_food_units` (gather/provision/build admission), `granary_food_units`
(eat-stored/provision admission, birth gate, capital yield), `granary_draw_used_this_hour` (the shared
one-draw guard), `food_units_waiting_to_pick_up` (gather admission), `build_point_in_progress` and
`build_done_this_hour` (what happens to a gathered unit at the frozen bag decision), `last_refused_action`
(the refused-action record the actor may reason about, renamed so its value is not an internal ID).

### The single-name exception (declared, not glossed over)

Each claim action has two names, which violates one-name-one-thing. It is declared here, in
`docs/ARCHITECTURE.md` §5.1 and in the owning code:

- the **internal ID** (`gather`, `eat-stored`, `provision`, `wait`) is a frozen wire and storage token:
  it is in every recorded request, the world's claim-action set, the role-a intervention records and the
  admissibility ladder, so renaming it would invalidate every ledger and every strategy fingerprint;
- the **display label** (`take_slot_food_into_bag`, `eat_granary_food`, `take_granary_food_into_bag`,
  `wait_this_hour`) is the canonical domain name and the only name the model sees;
- the two are bound by exactly one table, `protocol.ClaimActionVocabularyTable`, with
  `ClaimActionLabel`/`ClaimActionID` and the bijection test `TestClaimActionVocabularyIsABijection`
  (every ID has exactly one label and vice versa, no duplicate on either side);
- the v4 ladder maps the label back to the ID **before** the one admissibility ladder runs, so the world
  only ever sees a frozen token; an unmapped label is inadmissible and never an alias, so no second
  spelling stays alive;
- the **migration path** is a future contract version in which the internal IDs are renamed to the labels
  and a replay-preserving alias table maps the old tokens onto the new ones, so an old ledger still
  replays. That version does not exist yet.

## Deliverable B — the v4 system prompt

`NaturalClaimSystemPromptV4(PersonaFor(actor, portrait, hour))`, `internal/cognition/prompt.go`. Under 700
tokens by the declared estimate (bytes/4), asserted by `TestNaturalClaimSystemPromptV4Shape`. In order:

1. **the situation**, three sentences: one person in a small settlement, one decision per hour, the world
   does what you choose if it is possible and otherwise does what you would have done anyway;
2. **who the actor is** — a persona card (name, age band, temperament, one thing they want, one thing they
   fear), derived deterministically from the actor's own identity rows: entity id (FNV-1a), elapsed hours
   since birth (a declared age band, never a fabricated age in years), and the three genome alleles. It
   never reads a death hour, a parent id, the world seed or another actor's row — the input type
   `protocol.ActorPortrait` has no field for them, so the card cannot state a privileged fact. It is
   stable across hours except the age band, so the actor is the same person all run;
3. **the goal in the actor's own terms** — stay alive, keep food in the granary, build capital, raise a
   child if you can; not a scoring function;
4. **only what renaming could not carry** — what a slot is (food appears on its own, each holds one unit),
   what the granary is (the store capital fills, it rots slowly, the only store a child is fed from), the
   one-granary-draw-per-hour rule with both routes named, and the bag's single unit plus **energy as the
   death channel** (a meal restores one; at zero you die). Nothing else: no field-by-field glossary, and no
   restatement of a number the request already carries (asserted: the prompt contains none of the
   sample's values).

The prompt is the system message of the v4 call only; a call with no applied prompt keeps the fixed gateway
template, so every pre-v4 and every hermetic wire body is unchanged. The applied prompt is recorded
verbatim on the call record (`system_prompt`), because it is a function of the actor's identity rows rather
than of the request bytes, so the record is the only place a reader can see what the model was told.

**Output shape:** `{"schema_version": 4, "action": "<display label>", "why": "<one sentence>"}`. The `why`
is recorded on the role-a intervention as `why` and is read by no decision: the ladder never looks at it, a
missing `why` is not a schema violation, and an over-long one (> 512 bytes) is.

## Deliverable C — the comprehension test (the point of the ticket)

**Pre-declared before the first probe call** (this section is the criterion; it is not tuned after seeing
results).

- **Sample 1 — 20 previously-rejected requests.** From `.tmp/35-stage3-runs/*/calls.jsonl`, the call
  records referenced by an intervention whose `role_a.fallback_reason == "inadmissible"` (the v1 states the
  model got wrong), rendered at v4 by the committed encoder.
- **Sample 2 — 20 fresh requests.** From a **hermetic** v4 run of the committed binary (no live call, no
  spend): `-arms R -cells f10-s0 -hours 144 -claim-contract natural-claim/v4`, whose ledger records the real
  v4 requests the world produces (real portraits, real prompts), sampled across hours and actors. Fresh
  means: never a rejected v1 request, never seen by the model before this probe.
- **Probe configuration:** `gpt-6-luna`, responses wire, `reasoning.effort = low`, `max_output_tokens =
  32768`, JSON output mode, the v4 system prompt as the system message and the v4 request as the user
  message — identical to what a live v4 run sends. 40 calls.
- **Reported for both samples:** (1) the inadmissible rate under v4, against the v1 baseline **86.7 %**
  (ticket 35, 30 requests) and the v2 shortcut **0 %** which collapsed 29/30 answers to `gather`; (2) the
  action distribution (collapsed or varied), with the per-action counts; (3) at least 10 `why` strings
  quoted verbatim, including any nonsense, each judged against its action and the state; (4) a record of
  labels that are outside the declared vocabulary (they are inadmissible, and they are counted separately
  from a wrong action).
- **THE FALSIFIER — a separate 5-call probe.** Five further calls whose user message is the v4 prompt
  context plus the question "what can you do this hour, and why" asked in words, no action list: the model
  names the actions it believes are available, in its own words. The answer is compared **line by line**
  against the code's admissibility ladder for the same five requests (a table of `action → admitted by the
  model? / admitted by the ladder?`), and the **agreement rate** is reported with the disagreements named.
  This separates "the model can choose correctly when asked to pick" from "the model understands the rules
  at all".
- **Cost bound:** 45 calls. Stop before exceeding; the ticket budget is 1,600 calls (see below).

## Deliverable D — one full-AI run at v4

One replicate, `-arms R` (full wake), effort `low`, `-hours 144`, `-cells f10-s0`, contract
`natural-claim/v4`, the ticket-35 live bounds (`-max-output 32768 -response-bound 1048576
-attempt-timeout 300 -call-deadline 900 -parallel 8`), detached with `setsid`, liveness from the runner pid
**plus** ledger growth (never a pid file alone, never a pattern kill). Recorded: whether births occur, and
whether the final population exceeds the no-AI control's 2 (A0 at this cell is deterministic: population
h120 = 5, h143 = 2, births 2). **No allocation conclusions from it**: one replicate, one cell, no control
lab.

## Budget (supervisor correction recorded)

The dispatch said "at most 200 live model calls, hard cap $3". That 200 was sized for the comprehension
test and then applied to the whole ticket, which would have made deliverable D impossible: a 144 h
full-wake replicate is 1082–1305 calls (measured in `.tmp/35-v2-runs/R-f10-s0-r{1,2,3}`), and even the
call-capped arms are 244–256. The supervisor raised the ticket budget to **1,600 live calls, $3 cap
unchanged** (expected ≈ $0.40) and approved D as written. Recorded here so the budget line does not
contradict the executed work.

## Operational rules

- Commit in chunks; detach long runs with `setsid`; verify liveness by runner pid plus ledger growth; record
  artefact hashes; **do not push**; do not touch frozen evidence; the older contracts must stay
  byte-identical when selected (pinned by test).

## Results

### Deliverable C — the comprehension test (45 live calls, $0.0061)

Configuration as pre-declared: `github_copilot/gpt-6-luna`, responses wire, `reasoning.effort = low`,
`max_output_tokens = 32768`, JSON output mode, the v4 system prompt as the system message and the v4 request
as the user message. 45 calls, 28 332 input + 6 442 output tokens = **$0.0061**. Verdicts are not
re-implemented: each answer was put through the committed ladder
(`cognition.ValidateActorSeamContentFor`, contract v4) by `.tmp/36-judge`.

| sample | calls | admissible | inadmissible | malformed | unmapped label | inadmissible rate |
|---|---|---|---|---|---|---|
| 20 previously-rejected v1 states, rendered at v4 | 20 | 20 | 0 | 0 | 0 | **0 %** |
| 20 fresh states (hermetic v4 run, real portraits) | 20 | 20 | 0 | 0 | 0 | **0 %** |
| **v1 baseline (ticket 35, same 30-request probe)** | 30 | 3 | 26 | 0 | — | **86.7 %** |
| **v2 shortcut (ticket 35)** | 30 | 30 | 0 | 0 | — | 0 % (29/30 `gather`) |

**1. Inadmissible rate.** 0/40 under v4, against the v1 baseline 86.7 %. The vocabulary alone (no stated
admissible set, which v4 deliberately does not carry) removes the whole loss the v2 shortcut removed.

**2. Action distribution — varied or collapsed, stated against what the state allowed.** 38/40 answers were
`gather` (internal `gather`), 1 `eat-stored`, 1 `provision`, **0 `wait`** — but that headline is misleading
on its own, because the sample's states mostly admit only two actions:

| admissible set of the request | requests | chosen |
|---|---|---|
| {`gather`, `wait`} | 35 | 35 `gather` |
| {`gather`, `eat-stored`, `provision`, `wait`} | 4 | 3 `gather`, 1 `eat-stored` |
| {`eat-stored`, `provision`, `wait`} | 1 | 1 `provision` |

So **where the state offered a real choice (5 requests with a usable granary draw), the model varied: 3
gather, 1 eat-stored, 1 provision**, and picked a genuinely different action each time the state pointed at
one. Where only {gather, wait} was legal it always took the productive option and never idled. This also
settles an open question from ticket 35: **the v2 probe's 29/30 collapse to `gather` was not caused by
being told the admissible set.** The escalations at this fixture mostly admit only gather-or-wait, so a
near-constant answer is what the state supports. The `wait` count is worth stating plainly: wait was legal
in all 40 requests and was chosen in none.

**3. Why strings (all 40 recorded; 12 quoted verbatim, chosen to span the samples and the actions).**

| # | state | action | `why` | judgement |
|---|---|---|---|---|
| 1 | E2, empty bag/granary, 8 units waiting | gather | "I have food waiting in my slots and an empty bag, so I should secure a unit for a meal or capital." | consistent: both preconditions named, both uses named |
| 3 | E2, empty granary, food waiting | gather | "I should secure a waiting food unit in my bag while the granary is empty and my energy is low." | consistent; the urgency matches E2 |
| 8 | E2, empty granary | gather | "I should secure a food unit from the waiting slots while the granary is empty, so I can eat it when needed to stay alive." | consistent |
| 10 | E4, empty granary | gather | "I can collect food already waiting beside my home without spending the granary draw, preserving energy while keeping the option to invest it in capital." | consistent: knows the draw is a shared, spendable resource |
| 15 | E2, empty granary | gather | "I can secure a unit from the waiting slot food without spending my last available energy on an impossible granary draw." | consistent: refuses an action it cannot take |
| 21 | E2, empty granary | gather | "My bag is empty and food is waiting in my slot, so I can secure a unit without using the granary draw." | consistent |
| 23 | E2, **granary 5**, draw unused | eat-stored | "My energy is low, so eating from the granary now keeps me alive while food continues to accumulate in my slots." | consistent: at E2 with a drawable granary, eating is the survival move |
| 25 | E2, granary 7, 4 waiting | gather | "My energy is sufficient for now, so I'll collect the waiting food without drawing down the granary that feeds a child." | consistent with the stated goal (the granary is the child's food) |
| 34 | E7, granary 1, 0 waiting | provision | "My energy is healthy, so I can use this hour to take food for building up my capital." | consistent: gather impossible (nothing waiting), bag empty → the granary route into a bag that the bag decision will build |
| 37 | E2, granary 45 | gather | "I can collect waiting slot food without drawing down the granary, keeping that store available while I build up my reserves." | consistent |
| 40 | E4, granary 7 | gather | "I can secure a waiting unit without drawing from the granary, keeping its food available while building toward more capital." | consistent |
| 14 | E2, empty granary | gather | "The granary is empty, so I'll take a waiting slot unit into my bag to have food ready for the next hour." | consistent |

No nonsense appeared in this sample, and no `why` contradicts its action or its state. The vocabulary is
used correctly and unprompted: the answers speak of "the granary draw", "waiting slot food", "my bag",
"capital", i.e. the four facts the system prompt added and the renamed fields, not the old jargon.

**4. THE FALSIFIER — five calls asking in words, compared line by line with the ladder.**

Same five fresh states and the same persona prompt, but the question is "what can you do this hour, and
why" and the answer shape is `{"schema_version":4,"can_do":[…],"why":…}`. Agreement is per action
(4 actions × 5 requests = 20 cells) between "the model says it can" and "the ladder admits it".

| request | gather | eat-stored | provision | wait | exact set |
|---|---|---|---|---|---|
| hermetic#852 (empty granary, food waiting) | ✓ ✓ | ✓ ✓ (both say no) | ✓ ✓ (no) | ✓ ✓ | yes |
| hermetic#1836 (same shape) | ✓ ✓ | ✓ ✓ (no) | ✓ ✓ (no) | ✓ ✓ | yes |
| hermetic#690 (granary 5, draw unused, food waiting) | ✓ ✓ | ✓ ✓ | ✓ ✓ | ✓ ✓ | yes |
| hermetic#2026 (empty granary) | ✓ ✓ | ✓ ✓ (no) | ✓ ✓ (no) | ✓ ✓ | yes |
| hermetic#212 (granary 7, food waiting) | ✓ ✓ | ✓ ✓ | ✓ ✓ | ✓ ✓ | yes |

**Agreement 20/20 cells = 100 %, exact admissible set matched 5/5.** Its own words: "I can pick up one of
the eight waiting slot-food units into my empty bag or wait, but there is no granary food to eat or take"; "Your bag is empty, slot food is waiting, and the granary draw is unused, so each listed action is
available this hour". The model's model of the rules and the code's ladder are the same model on this
sample. The falsifier therefore narrows the remaining defect: comprehension is no longer the binding
constraint on the choice, and the near-constant `gather` answer is a property of the state and the goal,
not of a misunderstanding.

**Artefacts** (`.tmp` is git-ignored; hashes in the completion record below): `.tmp/36-comprehension-rejected.json`,
`.tmp/36-comprehension-fresh.json`, `.tmp/36-verdicts.jsonl`, `.tmp/36-comprehension-results.json`,
`.tmp/36-hermetic-runs/` (the fresh-request source: a hermetic v4 run, 1 443 requests, no live call),
and the probe tools `.tmp/36-render/`, `.tmp/36-judge/`, `.tmp/36-probe.py`, `.tmp/36-analyse.py`.

**Limits of this probe, stated.** 40 + 5 calls at one effort level, one model, two state samples; the
rejected sample is biased toward states whose only legal actions are gather and wait (that is why those
answers were rejected); the fresh sample is 87 % such states because the fixture's escalations are. The
historical sample's persona uses the declared neutral portrait (born hour 0, alleles 6/6/6), because a
recorded v1 request does not carry the genome; the persona text is not the variable under test, and the
fresh sample carries the world's real portraits.

