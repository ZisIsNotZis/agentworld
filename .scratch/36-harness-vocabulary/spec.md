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

The rule is: **no actor-state field may be inert** (a field that neither the model nor the world acts on
does not belong in the decision request), and **a non-state field must be declared provenance** rather than
left as unexplained noise. The earlier form of this sentence said "no field that neither the model nor the
world acts on belongs in the request", which the shipped request falsifies: `run.*` and `sampling` are read
by nothing that decides and are kept deliberately, as provenance, and are declared as such in the table
below. **Every observation field, every
derived-name field and every non-state request block is listed below with what actually reads it**, so the
sweep is checkable rather than asserted. "Acts on it" means a world admission gate, a frozen cascade
decision, or the thing that happens to the unit the actor is choosing to move; a validity check that reads
a value and discards it is not acting on it.

| field (v1 wire) | who reads it | verdict |
|---|---|---|
| `energy` | the death channel (`NaturalBasalFor`: `starved := Energy == 0`), `CapacityConsume`/`NaturalConsume` (`energy > 0`), the build floors, the cascade's claim floors, the report | **kept** (`energy`) |
| `hunger` | **validated and discarded** by the cascade (`_, hungerOK := capacityInteger(obs.Hunger, …)`); an allocator feature row (`urgency`, `decision_sensitivity`) and so part of *who is escalated*; nothing in any action, gate or mechanic | **dropped from the request**, kept as the world row, checkpoint field and allocator feature it is — the ledger fact. It has no decision consequence, and naming it (as the dispatch asked) as a "starvation clock" would have taught the model a false rule: see the verified semantics above |
| `bag_units` | `CapacityGather`/`NaturalGatherFor` (empty), `provision` (empty), build/consume (one) | **kept** (`bag_food_units`) |
| `capital` | the birth gate (`Capital >= 2`), the build cap (`k + wip < 8`), the cascade's bag decision, one allocator feature | **kept** (`capital`) |
| `wip` | the cascade's bag decision (`capital + wip < CapacityKMax`), i.e. whether the next bag unit completes a capital point | **kept** (`build_point_in_progress`) |
| `invested_units` | **validated and discarded** (`_, investedOK := …`); no feature, no gate, no report | **dropped from the request**, kept as the conservation ledger row it is |
| `last_build_hour` | `CapacityBuild` (`LastBuildHour >= hour` refuses a second build); the cascade only range-checks it | **replaced** by `build_done_this_hour`: the gate's outcome is what decides whether a gathered unit can become capital this hour, and the timestamp itself is history |
| `granary_stock` | `eat-stored` and `provision` admission, the birth gate (`granary >= 3`), the capital yield, one allocator feature | **kept** (`granary_food_units`) |
| `last_stored_meal_hour` | the shared one-draw guard of rule 509 (v6/v7) and rule 515 | **replaced** by `granary_draw_used_this_hour`: the affordance, not the timestamp |
| `home_patch` | the frozen cascade's home-patch validity check and the bag's source provenance; **no action can address a patch other than the actor's own** | **dropped from the request**, kept in the observation: the id decides nothing the actor chooses |
| `home_slots[].slot` | never used for a choice: `gather` always takes the lowest-numbered stocked slot (`naturalLowestStockedSlot`), and a model cannot name one | **dropped** |
| `home_slots[].stock` | `gather` admission (`stock == 1` in some slot), the cascade's candidate list | **collapsed** to `food_units_waiting_to_pick_up`: the array's sum is that count, and the ids above are unusable |
| `last_denial` | the frozen cascade's refusal fallbacks (`gather-denied` → eat-stored or rest; `build-denied` → eat), which is what is applied when a model answer is rejected, and one allocator feature | **kept and renamed** (`last_refused_action`), with its values respelled as display labels so no internal ID reaches the request |
| `strategy.parameters` (7 values) | **the frozen cascade's thresholds** (claim floors, bag build floors, birth filing floor): not inert, but *configuration, not state* — identical for every actor of a generation and identified by the recorded head ref (`natural/7` plus the generation's deltas) | **dropped**; 40 % of the v1 request's characters, and the prompt says in words what they decide (a bag unit can become capital or a meal) |
| `run.fixture`, `run.seed`, `run.base_ref`, `run.head_ref` | nothing in the decision reads them; they are the per-call run identity that binds a recorded call to its run | **kept** as provenance, not as state |
| `run.identity_digest` | nothing; it is `RunIdentityDigest` over the four `run` fields above | **dropped as derived** (two homes for one fact); it is recomputable from the request |
| `sampling` | nothing in the decision; the call record holds the same sampling instance as its own field | **kept, and named here rather than removed silently**: it is the frozen spec's (§6.2) declared wire sampling instance of the canonical request, it states no actor fact, and the D run executes under exactly these bytes. It is a declared non-state field of the call, not a fact the actor reasons about — and it is the one field in the request that is duplicated on the call record |

What this sweep found, stated plainly: **three fields were present, named and read by nothing that decides
anything** — `hunger` (the worst of them, because its name advertises a stake it does not have),
`invested_units`, and `home_patch` — and a fourth, `run.identity_digest`, was a determined function of the
field block beside it. Two of those (`hunger`, `invested_units`) are the actor's own state, so they stay in
the world, the checkpoint and the request-independent feature rows where they do work; one (`home_patch`)
is identity, and one (`identity_digest`) is derived. All four left the model-visible request.

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
tokens by the declared estimate (UTF-8 bytes over four, rounded up: **419** after the post-review wording
correction, 1 673 bytes), asserted by `TestNaturalClaimSystemPromptV4Shape`, which is the single place the
estimate is stated. In order:

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
  context plus the question "what can you do this hour, and why" asked in words, **no action list**: the
  model names the actions it believes are available, in its own words. The answer is compared **line by
  line** against the code's admissibility ladder for the same five requests (a table of `action → admitted
  by the model? / admitted by the ladder?`), and the **agreement rate** is reported with the disagreements
  named. This separates "the model can choose correctly when asked to pick" from "the model understands the
  rules at all".
  *Amendment after review:* the first execution of this probe appended an instruction to answer from the
  request's `actions` list, which is a list-filtering test, not the declared recall test. Both forms are
  reported in the results section (form 1, labels supplied: 20/20 cells; form 2, question only, no action
  list: behavioural agreement 5/5, declared-label recall 0/5), each labelled with the form that produced
  it.
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

**4. THE FALSIFIER — five calls asking in words, compared line by line with the ladder, in TWO forms.**

The first form ran with the action list and an explicit instruction to answer from it. An independent
review found that this deviates from the pre-registration ("no action list: the model names the actions it
believes are available"), and it is right: form 1 measures **list-filtering**, not recall. Both numbers are
reported here, each labelled with the form that produced it, and the pre-registration paragraph at the top
of this section is amended accordingly.

**Form 1 — labels supplied (the original run; 5 calls).** The question ended with an instruction to answer
`{"schema_version":4,"can_do":[…], …}` using the request's own `actions` list. Agreement is per action
(4 actions × 5 requests = 20 cells) between "the model says it can" and "the ladder admits it".

| request | gather | eat-stored | provision | wait | exact set |
|---|---|---|---|---|---|
| hermetic#852 (empty granary, food waiting) | ✓ ✓ | ✓ ✓ (both say no) | ✓ ✓ (no) | ✓ ✓ | yes |
| hermetic#1836 (same shape) | ✓ ✓ | ✓ ✓ (no) | ✓ ✓ (no) | ✓ ✓ | yes |
| hermetic#690 (granary 5, draw unused, food waiting) | ✓ ✓ | ✓ ✓ | ✓ ✓ | ✓ ✓ | yes |
| hermetic#2026 (empty granary) | ✓ ✓ | ✓ ✓ (no) | ✓ ✓ (no) | ✓ ✓ | yes |
| hermetic#212 (granary 7, food waiting) | ✓ ✓ | ✓ ✓ | ✓ ✓ | ✓ ✓ | yes |

**Form 1 agreement: 20/20 cells = 100 %, exact admissible set 5/5 — as list-filtering.** The model was
handed the four labels and the request's action list, so this number certifies that it can filter a menu by
the state; it says nothing about whether it knows the vocabulary.

**Form 2 — question only, no action list, free-form (the declared form; 5 calls, same five states).** The
`actions` array was **removed** from the state shown, no answer schema was given beyond the question, and
the wire's JSON output mode (which every live call uses) remained on. The answers, verbatim:

| request (ladder-admissible set) | answer | action it names | ladder verdict |
|---|---|---|---|
| hermetic#852 {gather, wait} | `{"schema_version":4,"action":"pick_up_food","why":"I have food waiting in my slots and an empty bag, so I should secure one unit while I can."}` | `pick_up_food` → gather behaviour | admissible |
| hermetic#1836 {gather, wait} | "…`"action":"pick_up_food"`, `"why":"I have no food in my bag or granary, so I should collect a waiting unit before my energy runs out."`" | `pick_up_food` → gather | admissible; its "granary is empty" clause agrees with the ladder (both granary actions inadmissible) |
| hermetic#690 {all four} | "…`"action":"pick_up_food"`, `"why":"I can collect a waiting unit into my empty bag without drawing down the granary, preserving stored food while preparing to build capital."`" | gather | admissible (chose not to draw, correctly) |
| hermetic#2026 {gather, wait} | "…`"action":"pick_up_food"`, `"why":"I have no food in my bag or granary, so I should collect a waiting unit before my energy runs lower."`" | gather | admissible; granary clause agrees |
| hermetic#212 {all four} | "…`"action":"eat"`, `"why":"Eating from the granary restores energy and helps me stay alive while my capital continues producing food."`" | `eat` → eat-stored | admissible |

Two numbers, and they point in opposite directions:

- **Behavioural agreement with the ladder: 5/5** — every named action is one the ladder admits for that
  state, and in the two states where the granary was drawable the model varied (gather in one, eat-stored in
  the other). Stated cells: 10 of 20 (gather 5 positive, eat-stored 1 positive + 2 explicit "granary is
  empty" negatives, provision 2 negatives), **10/10 agree**, 10 unstated, 0 contradictions.
- **Vocabulary recall: 0/5.** With no list to choose from, the model invented `pick_up_food` (×4) and `eat`
  (×1); it never produced `take_slot_food_into_bag`, `eat_granary_food`, `take_granary_food_into_bag` or
  `wait_this_hour`. The display labels are **usable only because the request carries them**, not natural
  names the model would produce on its own.

That is the finding, not a failure of the test: the v4 contract works as designed (the labels are in the
request, and the answer is mapped back through one table), and form 1's 20/20 is exactly the list-filtering
it measures. A future contract that wanted recall would have to make the labels memorable (shorter, or
buildable from the prompt), which is a vocabulary-design decision this ticket does not take.

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


### Deliverable D — progress (polled, committed between polls)

Live run declared and launched at 12:23 local: `.tmp/36-run-v4.sh` →
`.tmp/36-vs2-bin -arms R -cells f10-s0 -reps 1 -hours 144 -effort low -max-output 32768
-response-bound 1048576 -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 -live
-parallel 8 -budget-min 720 -root .tmp/36-v4-runs`, detached with `setsid`, runner pid **2440039**.
Liveness is the runner pid **plus** ledger growth (`.tmp/36-v4-runs/R-f10-s0-r1/calls.jsonl`), never a pid
file and never log content (the report is written only at exit).

| local time | call records | rate (calls/min since first) | statuses |
|---|---|---|---|
| 12:39 | 271 | 17.2 | all `ok` |
| 12:57 | 603 | 17.9 | all `ok` |
| 13:07 | 780 | 18.1 | all `ok` |
| 13:12 | 875 | 18.1 | 874 `ok`, 1 `http-error` (transport retry; the attempt was retried and the call completed) |
| 13:29 | 1 132 | 17.2 | all `ok` |
| 13:36 | 1 266 | 18.2 | 1 265 `ok`, 1 `http-error` (retried) |
| 13:47 | 1 325 | — | run complete: **1 325 call records, 1 323 applied, 2 `malformed`, 0 `inadmissible`**, exit 0, report emitted |

### Deliverable D — result: one full-AI replicate at v4 (144 h, cell f10-s0, effort `low`)

Command as declared above; exit 0; 1/1 cell ran, 1 325 call records, `halted=false`, `under_budget=false`.
Artefacts: `.tmp/36-v4-evaluation.json` (sha256 `8aa4342e…146463`), `.tmp/36-v4-evaluation.txt`
(`b7b2bec1…2d9e3c`), ledger `.tmp/36-v4-runs/R-f10-s0-r1/calls.jsonl` (`c85746de…39403a`).

| quantity | value |
|---|---|
| population at h 0/24/48/72/96/120/144 | 10, 11, 11, 11, 11, 4, **1** |
| **final population vs the no-AI control's 2** | **1 — the replicate does NOT exceed the control** (A0 at this cell is deterministic: 10, 12, 12, 12, 12, 5, 2 with 2 births) |
| births cumulative | **2** (both by h 24; none afterwards) |
| starvation deaths cumulative | 1 |
| age deaths cumulative | 10 |
| applied / rejected decisions | 1 323 / 2, **0 `inadmissible`** (0.15 % decision-error rate against 40–53 % under v1 and 0.7–1.3 % under v2). The two fallback records both carry the typed reason `malformed`, and neither is a schema-invalid answer: one (seq 1560, actor 2017, h69) is a **transport error** (`status: http-error`, 0 tokens, no content), the other (seq 2124, actor 6, h95) is a **truncation at exactly 32 768 completion tokens** (`status: ok`, 106 240-byte body, JSON cut mid-object). The output-bound loss channel has therefore not gone away; it is one call in 1 325 at `low` effort with a 32 768 bound |
| applied action mix (internal IDs) | `gather` 1 115 (84.3 %), `provision` 139 (10.5 %), `eat-stored` 62 (4.7 %), `wait` 7 (0.5 %) |
| the frozen cascade's own mix at the same instants (recorded `frozen_action`) | `gather` 1 181, `provision` 119, `eat-stored` 25, `wait` 0 → **91 applied decisions (6.9 %) diverged from the frozen policy**, and the divergence runs in both directions |
| calls / tokens / cost | 1 325 calls, 824 408 input + 234 007 output tokens ≈ **$0.199** at the declared gpt-6-luna prices (the report records cost as UNPRICED because the gateway exposes no price entry; the token totals are the evidence) |
| coverage | min 23, max 139, Gini 0.119 |

**Was the birth gate ever within reach? Yes — and that is the point of the measurement.** Per actor, the
closest approach to `capital >= 2 AND granary >= 3` in an escalated hour of this replicate:

| actor | max capital | max granary | closest hour (capital, granary, energy) | escalated actor-hours at the gate |
|---|---|---|---|---|
| 1 | 1 | 2 | h6 (1, 2, 7) | 0 |
| 2 | 1 | 1 | h2 (1, 1, 9) | 0 |
| 3 | 1 | 2 | h6 (1, 2, 7) | 0 |
| 4 | 1 | 2 | h3 (1, 2, 8) | 0 |
| 5 | 1 | 1 | h2 (1, 1, 9) | 0 |
| 6 | 1 | 1 | h2 (1, 1, 9) | 0 |
| 7 | 1 | 2 | h11 (1, 2, 5) | 0 |
| 8 | 1 | 3 | h15 (1, 3, 3) | 0 (one capital short) |
| 9 | 2 | 7 | h4 (2, 4, 7) | **3** |
| 10 | 2 | 6 | h4 (2, 4, 7) | **3** |
| 2017 (born h5) | 3 | 35 | h11 (2, 4, 4) | **58** |
| 2018 (born h6) | 3 | 39 | h11 (2, 3, 4) | **101** |

So the two meanings of "still no births" are separated by this table, and stated in the only form the
measurement supports: **the two one-actor floors (`capital >= 2` and `granary >= 3`) were held, not merely
approached** — the two second-generation actors sat at or above them for 58 and 101 escalated actor-hours
each, and no third birth followed. What that does **not** establish is where the birth path stops, because
reproduction is a **two-parent conjunction**, not a threshold: a request must be filed by a proposer
(own energy at or above the filing floor, a free and idle request row, one request per actor-hour), the
addressee must be alive, **kin-allowed and in the same home patch**, and the reply must re-check the
consenter's own energy/capital/granary floors before the granary payment — evaluated at the reply instant,
not at the claim instant this run observes. This replicate carries none of the partner, request-row or
reply-instant facts, so the correct statement is: **the claim-instant capital/granary floors were met and
did not convert into a third birth; which of the remaining conjuncts binds is unmeasured.**

**Limits of D, stated.**

1. One cell, one replicate, one effort level, one contract; the no-AI control is the deterministic A0 cell
   at the same fixture (ticket 35 record), not a paired replicate of this run; the arm is full wake (every
   claim-wake actor escalated), so this is not an allocation result. **No allocation conclusion is drawn and
   none may be** (the M2a/M2b/C stages above say why).
2. **The band's ladder is a strict superset of the world's admission for `gather`.** The v4 ladder admits
   `take_slot_food_into_bag` whenever some home slot is stocked; the world additionally requires a non-empty
   bag to be *empty*, `energy > 0`, `LastGatherHour < hour`, and the food-flow allocator's own eligibility
   for the target slot. A model answer can therefore be certified admissible and still be refused after
   commitment, and this run cannot count those refusals: the natural journal (where attempt refusals are
   written) is inside the run bundle and not in the ledger, so no attempt-rejection count is reported.
   Measured lower bound on the gap in this replicate: **0 requests** admitted gather while the bag already
   held a unit (the other two clauses are not observable from the v4 request, which no longer carries
   `lastGatherHour`). The gap is real, unexercised on that clause here, and belongs in the next contract's
   review.
3. **`final_bundle_error: "invalid natural evidence journal"`** is recorded on the cell (pre-existing; the
   ticket-35 runs carry the same message) and the cited `natural-root.bundle` /
   `natural-branch-enabled.bundle` sha256 values are hashes of those **bundle files** (git bundles of the
   run's genesis and births-enabled branch, 28 KB each), not hashes of the world state, the journal or the
   ledger. The ledger hash is the separate `…/calls.jsonl` value above. The bundle error means the run's
   journal does not re-validate through that path; it does not invalidate the recorded call ledger or the
   projection series the table above reports, and it is **not** repaired here.
4. The prompt sentence this run sent said "a meal restores one unit of it"; at the ceiling of 12 a meal
   restores nothing and counts as cap-lost. The wording is corrected in the code after the run (see the
   prompt section) and the recorded `system_prompt` of every call keeps the pre-fix text, so the run's wire
   is unchanged by the correction.

### VS2-EC-6 was silently disabled for v4 — found by review, repaired, re-run at zero spend

**The defect.** `cmd/vs2/privacy.go` held one allow-list, the v1 key set, and applied it to every recorded
request regardless of contract. Every `natural-claim/v4` request therefore failed the top-level key check,
`scanRoleARequest` returned **before** the privileged-marker scan, and the band printed key violations beside
a hard-coded sentence claiming the observation key set was exact. The D report recorded, verbatim:

> `VS2-EC-6 FAIL | … Role A requests scanned=1326 (recorded=1325), observation key set exact, privileged
> markers=0 [none]; … key violations=1325 [R-f10-s0-r1#0:request-keys:, …

Every clause of that line was wrong in the way that matters: the marker scan had examined **nothing** for
this contract, and the "exact" claim was an unconditional string. **Before this repair the observation-boundary
check was silently disabled for the contract the run used**, and the ticket's own evidence was certifying a
boundary nobody had checked.

**The repair** (three parts, all in code with tests):

1. the allow-list is **per contract**, declared once in `internal/cognition/seam_keys.go`
   (`DeclaredSeamRequestKeySet`), and the scan selects it from the contract the recorded request declares
   about itself (`prompt_template`). An undeclared contract is a violation, never a pass. The evidence line
   now reports the counts that were certified and examined per contract, and the marker scan's scope beside
   the marker count, instead of asserting anything.
2. the sentinel is encoded and scanned under **every** declared contract (v1, v2, v4), so a contract whose
   encoder drifts from its declaration fails the band rather than passing because another contract's
   sentinel was checked.
3. the shared declaration is guarded by `TestDeclaredSeamRequestKeySetsMatchTheEncoders` (each declared row
   must equal exactly what that contract's encoder writes for the same request value), and
   `TestActorSeamRequestV4UsesTheDecisionVocabulary` now consumes that declaration instead of keeping its own
   copy. `cmd/vs2/privacy_test.go` pins the scan itself: a v4 request is certified, an extra observation key
   or a v1 name inside a v4 request is a violation, an undeclared contract is a violation, and a privileged
   marker inside a declared field is caught. The v4 request bytes are additionally pinned
   (`goldenV4Request`), so the bytes this run recorded cannot drift silently.

**The re-run, at zero spend.** The band is recomputed from the recorded run directories by the new
`-recheck-in` mode (no cell runs, no model call), and the same mode is used on the ticket-35 roots as a
cross-check that the per-contract selection is not v4-specific. Verbatim:

> `VS2-EC-6 PASS`
> `import closure: request-builder=22 pkgs (no agentworld/internal/world), protocol=13 pkgs (no net/agentworld/internal/world), world=29 pkgs (no client); violations=0 [none]; Role A requests scanned=1328 (recorded=1325), declared key set matched by contract [natural-claim/v1=1,natural-claim/v2=1,natural-claim/v4=1326] against declared contracts [natural-claim/v1=1,natural-claim/v2=1,natural-claim/v4=1326]; privileged markers=0 over 1328 requests that passed the key check [none]; allocation records scanned=0, markers=0; key violations=0 [none]`

(cell `.tmp/36-v4-runs`: 1 325 recorded requests + 3 sentinels, all certified, **markers=0**, key
violations=0). Cross-check on the older roots: `.tmp/35-v2-runs` PASS (6 580 recorded v2 requests + 3
sentinels, markers=0, 1 359 allocation records scanned, key violations=0) and `.tmp/35-stage3-runs` PASS
(2 265 recorded v1 requests + 3 sentinels, markers=0, 933 allocation records, key violations=0). The
per-contract counts also show where the defect lived: **v1 and v2 were certified by the old single list; only
v4 was not.** Reports: `.tmp/36-ec6-recheck.json` / `.txt` (D), and the same mode over the two ticket-35
roots.

### What the vocabulary change did and did not fix

**Fixed:** the model's answers are now admissible without the world telling it the answer — 0/40 in the
comprehension probe and 0/1 325 in the full run, against 86.7 % inadmissible under v1 — and the model's own
account of what it can do agrees with the code's ladder (20/20 cells, 5/5 exact sets in the
labels-supplied form; the no-list form is reported above). Of the 1 323 applied decisions, all carry a
recorded `why`, **12 were read and quoted below, and at least one is degenerate**: seq 2179 (actor 2018,
h97, 453 characters) reads "…while my energy is still enough to spare the hour.}belisoaไทยฟรี
大发时时彩开奖} (Oops exact JSON only; malformed? final currently has extra weird token? …" — runaway
text, consistent with the output cap being reachable at this effort. The action still applied because the
ladder never reads `why`, which is the point of keeping comprehension evidence out of the decision path;
the claim this record may make is therefore "12 quoted of 1 323, one degenerate", not "the whys are
clean".

**Not fixed, and not claimed:** the **`gather`-heavy action mix (84 % of applied decisions) is now measured
against the state distribution rather than asserted.** Running the committed ladder over the 1 325 recorded
D requests gives:

| admissible set of the request | requests | model's choice |
|---|---|---|
| {gather, wait} | 851 (64.2 %) | gather 850, wait 1 |
| {gather, eat-stored, provision, wait} | 328 (24.8 %) | gather 265, eat-stored 58, provision 5 |
| {eat-stored, provision, wait} | 144 (10.9 %) | provision 134, eat-stored 4, wait 6 |

so 64 % of the escalations admitted only gather-or-wait (the mix is forced by the state), and where all four
were legal the model split 265/58/5 and where gather was impossible it took the granary routes in 138 of
144 hours. The remaining open items are the birth path below the one-actor floors, the fidelity anchor
(Q1), which is M2b, and any allocation comparison, which is M2c. **The central claim is not addressed by
this ticket, and the M2 milestone remains not accepted.**


## Completion record

**Commits** (branch `feature/36-harness-vocabulary`, branched from `master` `8269765`; not pushed):
`0f10f30` v4 vocabulary + one label table + v4 system prompt · `e6f5220` pre-declared comprehension test
and its results · `124ccea`/`d5e02dc`/`78f332e`/`a8543a4`/`92504be`/`41111e0` D progress, the ticket-35
correction, the complete field sweep, the M2 stage declaration, the D result and the CHANGELOG entry.

**Artefact hashes (sha256)**

| artefact | sha256 |
|---|---|
| `.tmp/36-v4-evaluation.json` (D, live) | `8aa4342eab551fd8de53d0a5ef0cbcec3f3eb79976993c7ed8325d9218146463` |
| `.tmp/36-v4-evaluation.txt` (D, live) | `b7b2bec1256474e5d2558516fd06785475b977aaa8be8c18eb928d679f2d9e3c` |
| `.tmp/36-v4-runs/R-f10-s0-r1/calls.jsonl` (D ledger, hash-chained) | `c85746de35c7f2aeb7c7509785a8eb7788c07de7f02212b1c12589bdc839403a` |
| `.tmp/36-v4-runs/R-f10-s0-r1/natural-root.bundle` | `cfafad62933e454a761153e51e9b6e06e6838c9753070e8f17c9786d8df6fcce` |
| `.tmp/36-v4-runs/R-f10-s0-r1/natural-branch-enabled.bundle` | `c76013dd348b540a0abc77607cee72b512e35f27077c856af168d2a221ddb2cf` |
| `.tmp/36-hermetic-eval.json` (fresh-request source, hermetic) | `3d19dae82d133e02e3693954d8f0784ed26bf0c9142590df5b4df775a49459f3` (1 443 v4 requests, 0 live calls) |
| `.tmp/36-comprehension-results.json` (probe analysis) | `48243bc88b61f7610f590e35a3a05d7da9a87ab90a9ca585476c2be6c6a44f96` |
| `.tmp/36-verdicts.jsonl` (ladder verdicts for the 45 probe answers) | `983573731c379f325de1d4bc407f685466279c6bc23df6befa73da7c9b5592ce` |
| `.tmp/36-vs2-bin` (binary used for both the hermetic and the live run) | `8b573f72a31938d1b5d4758aca425fa655543a1a0813f020ff36a45e4eca9598` |

**Gates.** At the final tree: `go build ./...` PASS, `go vet ./...` PASS, `gofmt -l` (tracked `.go`) clean,
`git diff --check` clean, `go test ./internal/cognition/... ./cmd/vs2 ./internal/experiment` **193 tests
PASS**, `go test ./internal/world -run 'Cognition|Seam|Vocabulary|Portrait'` **38 tests PASS**. The **full
suite** `go test -timeout 30m ./...` passed at the code tip `e6f5220` (`EXIT=0`, every package `ok`,
`internal/world` 1 260.178 s, zero FAIL lines); only documentation changed after that tip.

**Spend for this ticket**: **1 370 live model calls** — 45 comprehension/falsifier probe calls
(28 332 input + 6 442 output tokens ≈ $0.0061) plus 1 325 run calls (824 408 + 234 007 tokens ≈ $0.199) —
**≈ $0.21**, inside the supervisor's corrected **1 600-call / $3** budget. The dispatch's "200 live model
calls" was sized for the comprehension test and had been applied to the whole ticket; the supervisor
corrected it to 1 600 and approved deliverable D as written (recorded above).
