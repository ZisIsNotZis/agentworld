# Surplus-generating behaviour (design, v6 policy over the v5 world)

Status: claimed

## Issue

Ticket 15 built a resource-determined world (land as the only production input: patch regeneration budget R split exactly R = cultivated + wildFilled + unrealized; capital occupies a slot; spoilage replaces the granary cap; cap-free allocator IDs; instrumented compute ceilings) and then **falsified its own A-band**: under the frozen `natural/5` policy the organic ecology collapses into a poverty trap. Every actor reaches k=0 by ~h24 and can never build again, because the frozen bag-decision floor (`energy ≥ 5 ∨ granary ≥ 1`) is unreachable once energy is pinned below 5 with an empty granary under net-zero gather+eat. The birth gate (k≥2 ∧ granary≥3) is therefore unreachable, the population is sterile and static until the founder die-off, and extinction follows at h141/h143. Conservation is green at every hour of every cell, so this is the predeclared P15 "design is at fault" branch **at the policy/behaviour layer**.

The cultivation-disabled ablation sharpens the diagnosis: D is hourly-identical to C (births-off) in every cell, so **under the frozen behaviour rule, capital accumulation contributes nothing to survival**. The mechanism works; the behaviour cannot use it. That makes a behaviour-only fix the highest-value next step: exchange, money and institutions all require a world where accumulation is *worth* doing and where surplus can exist.

## Operator direction (recorded)

The operator's sequencing is productive capacity → surplus → reproduction/genetics → affect/motives → exchange/credit, with exchange, money and richer social concepts next in the arc. Ticket 15 delivered the capacity mechanism but its frozen policy cannot generate surplus, so the behaviour rule must be fixed before exchange is meaningful. Operator principle P15 (docs/PRINCIPLES.md) governs: limits come from modelled nature or from our compute, never from arbitrary design caps; a design defect is diagnosed and fixed, and no frozen experimental number is tuned post hoc.

## Objective

Design (not implement) a v6 behaviour policy over the **unchanged** v5 world (no mechanism, constant, conservation, or checkpoint-format change) such that:

- behaviour can convert gathered food into durable productive capacity under scarcity — i.e. the poverty trap is escapable by an actor's own choices, without any rule that hides a subsidy;
- accumulation is genuinely risky (starvation must remain reachable) so that takeoff, stagnation and collapse are all live outcomes;
- the fixed 168 h horizon, R ∈ {0,8}, P=2 default, cap-free roster and all frozen v5 identities are preserved;
- the design states exactly what behaviour distinguishes the new policy from `natural/5` (e.g. saving thresholds, storing before eating, deferring consumption, reserving food for builds, abandoning untenable plots) and why each rule has a defensible in-world reading rather than being tuned to produce growth;
- both prior verdicts can be re-evaluated against the same matrix so the comparison is causal: the v5 poverty trap result is the control.

## Design questions (must be answered with exact numbers)

1. Which single behavioural change (or minimal set) escapes the trap, and what is its in-world justification? Candidates to weigh and reject with reasons: an energy-saving rule (do not eat when energy is already safe), a store-then-build rule (bank a unit instead of eating while energy > threshold), a build-affordability rule keyed to stored food rather than current energy, plot abandonment/re-tenure timing, or gather-target selection.
2. Does the mechanism need any change to be escapable at all (record honestly if the answer is yes, with the exact constant/identity that forces it — P15 forbids tuning, but a genuine mechanism defect must be named)?
3. What replaces the arbitrary-looking TLow = 5 floor? Any new threshold must have an in-world reading (e.g. "build only if you can survive the two build hours") or be derived rather than declared.
4. Starvation risk: what keeps over-investment fatal so collapse remains reachable?
5. Does the surplus indicator (k ≥ 2 ∧ granary ≥ 1 sustained, measured surplus flow) become reachable, and what are the predeclared bands for it?
6. Interaction with reproduction: with a working behaviour, does the birth gate become reachable, and what does that imply for the (already-frozen) v4/v5 reproduction bands?
7. What must stay identical so the v5 evidence remains a valid control (policy bytes are versioned; the world and its checkpoints are untouched)?

## Required designer outputs

Chosen behaviour rule(s) with exact integer thresholds and their in-world reading; rejected alternatives with reasons; the predeclared gate list (integrity gates unchanged, plus outcome bands for surplus, growth, stagnation and collapse, with falsification criteria); the experiment matrix (at minimum: v6 policy vs v5 policy on the same cells, R ∈ {0,8}, seeds, workers, births on/off, plus the cultivation ablation and the compute-ceiling cell to confirm the P15 instrumentation still functions); metric definitions (surplus flow, capital persistence, time-to-first-build, deaths by cause); lane board; stop conditions.

## Constraints

- v1–v5 world, mechanisms, constants, checkpoints and recorded evidence stay byte-frozen; v6 is a new policy version only.
- The frozen v5 A-band is NOT re-tuned: it stays recorded as falsified under `natural/5`; v6 gets its own predeclared bands.
- Determinism, replay, checkpoint/branch discipline unchanged.
- No post-hoc tuning; a null result (v6 also stagnates, or collapses faster) is valid and must be reportable verbatim.
- Honest disclosure if the fix requires a mechanism change after all, with the exact reason.

Need-review: yes
Need-test-cases: yes

## Comments
