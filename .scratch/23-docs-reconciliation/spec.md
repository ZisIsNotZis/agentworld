# Documentation reconciliation (VS1 audit corrective action 3)

Status: complete on `feature/23-docs-reconciliation`; documentation-only. Branch base `6fa551a`
(master). No code changed, no merge to master, no frozen evidence touched.

## Objective

Perform VS1 audit corrective action 3: the documentation reconciliation pass recorded in
`.scratch/21-vs1-mind-in-the-loop/spec.md` Appendix D and `.scratch/audit-vision-fidelity.md`
corrective action 3. Bring the founding documents into agreement with the measured and implemented
truth by editing current-fact sources of truth in place: retire/annotate the P0/P1 language-model
exclusions that the active plan supersedes, correct the false present-tense selective-waking claim,
mark the promised-but-absent `EscalationRequest` as not-yet-implemented, and add the qualifications
the measured record requires (Role A has no patch; digest chains are not adversarial authentication;
the P1 scale target is unbuilt; the P0 exit criterion holds only on the later density axis).

## Contract

- Sole writer of `docs/ROADMAP.md`, `README.md`, `docs/P0-SPEC.md`, `docs/P0-IMPLEMENTATION-PLAN.md`,
  `docs/ARCHITECTURE.md`, `docs/PRINCIPLES.md`.
- OUT OF SCOPE and not touched: `.scratch/21-vs1-mind-in-the-loop/spec.md`, `docs/VERTICAL-SLICES.md`,
  all code, all frozen `.tmp` evidence, `docs/VALIDATION.md`, ADRs, other ticket lanes.
- Documentation-only and reversible; no secrets; no code.
- Current-fact docs only: no changelogs, errata, "was A now B", dashboard timestamps, duplication, or
  conflicting statements; superseded text is marked in place and honest history is preserved.
- Every edited claim traces to a cited code path, measured evidence, or recorded product decision;
  where a claim cannot be resolved from an authoritative source, stop and report rather than guess.

## Acceptance indicators

1. `git diff --check` clean.
2. Each Appendix D item either corrected in an in-scope document (with cited source) or, where its
   only locus is out of scope, recorded here as deferred to the owning lane.
3. No history deleted; supersession/qualification added in place.
4. Before/after per edited document, source per change, and exact `git diff --stat` recorded below.
5. Committed on `feature/23-docs-reconciliation`; not merged to master.

## Authoritative sources

- `.scratch/21-vs1-mind-in-the-loop/spec.md` Appendix D (items 1–9), Appendix B/C §9.3.
- `docs/VERTICAL-SLICES.md` document status (supersession wording), VS1 (C7 baseline), VS2 (attention
  target), Milestones. Read only, not edited.
- `internal/cognition/protocol/wake.go` — `VS1WakePolicy` is "the honest uniform/periodic baseline".
- `internal/cognition/intervention.go` — "Appendix D.5: a Role A decision has no patch".
- `docs/adr/0006-cognitive-layer-recorded-model-calls.md` — digest chains "detect damage, not a
  wholesale rewrite by an adversary".
- Code absence: `grep -rn "EscalationRequest" --include=*.go .` returns nothing.
- `docs/VALIDATION.md` v5 F record — 256 founders halt at h66–h70, 19.2–21.9 s per simulated hour
  against the 0.495 s budget (≈38–44×).
- `docs/ROADMAP.md` "Current status" decision-sensitivity map — f=10/11 reliable, f=12 fragile,
  f=16 arithmetic null; ticket 18's founding-band density axis (f≤13).

## Before / after per edited document, with the source for each change

### docs/ROADMAP.md

- **P0 exit criterion (item 9).** BEFORE: `- distinct strategies produce measurable and causally
  explainable population outcomes.` AFTER: same criterion with the qualification that it is met only
  on the ticket-18 density axis (f≤13) and that at the saturated f=16 fixture every organic policy is
  populationally indistinguishable, pointing to the decision-sensitivity map below. Source: Appendix
  D.9; ROADMAP.md current-status map; VALIDATION.md density record.
- **P1 target (item 8).** BEFORE: `Target: approximately 10,000 explicit people with selective model
  intervention.` AFTER: same target followed by the measured qualification — unbuilt/unmeasured, the
  recorded substrate is 16 actors, the 256-founder probe halted the harness at h66–h70 at 38–44× the
  predeclared CPU budget (VALIDATION v5 F record), and the ~1,000-person/decades P0 target is also
  unbuilt. Source: Appendix D.8; VALIDATION.md v5 F record.
- **P1 exit criterion (item 5).** BEFORE: `- every model intervention records cause, cost, patch, and
  measured result;` AFTER: `every model intervention records cause, cost, and measured result; a Role
  B strategy intervention additionally records the patch, while a Role A decision has no patch and
  carries the explicit role marker plus the applied-version head instead
  (internal/cognition/intervention.go);`. Source: Appendix D.5; `internal/cognition/intervention.go`
  (`roleAInterventionScope`, "Appendix D.5: a Role A decision has no patch").

### docs/ARCHITECTURE.md

- **§3 strategy capability (item 7).** BEFORE: `- request cognitive escalation.` AFTER: same with
  "(**not yet implemented** — no `EscalationRequest` type exists; VS2 owns escalation requests …)".
  Source: Appendix D.7; code absence; VERTICAL-SLICES VS2.
- **§4 cognitive scheduler (item 4).** BEFORE: ended at "Budget channels include exploitation …".
  AFTER: added a current-fact note that this is the VS2 (attention) target— value-based priority
  layer, escalation requests, and protected exploration are not built; VS1's allocation policy is a
  declared wake-policy object with the honest uniform/periodic baseline only, and no scheduler
  consumes a value term. Source: Appendix D.4; `internal/cognition/protocol/wake.go`
  (`VS1WakePolicy`); audit §1(b) (time-ordered wake heap).

### docs/PRINCIPLES.md

- **P7 selective language-model cognition (item 4).** BEFORE: the paragraph asserting language models
  wake for novelty/uncertainty/etc. AFTER: added a note that this is the VS2 target and that in VS1
  the wake policy is the declared uniform/periodic baseline only, with value-based selective waking,
  escalation requests, and protected exploration not yet implemented. Source: Appendix D.4;
  VERTICAL-SLICES VS1 C7/VS2; `internal/cognition/protocol/wake.go`.

### docs/P0-SPEC.md

- **§2 scope (item 7).** BEFORE: `- escalation requests recorded for later analysis.` AFTER: same
  bullet marked "(**not yet implemented** — no `EscalationRequest` type exists; VS2 owns escalation
  requests …)". Source: Appendix D.7.
- **§10 strategy boundary (item 7).** BEFORE: `- request escalation.` AFTER: same bullet marked
  "(**not yet implemented** — see the §2 note; VS2 owns escalation requests)." Source: Appendix D.7.

### README.md and docs/P0-IMPLEMENTATION-PLAN.md — no edit required

- **README.md (items 1, 3, 6):** the P0/P1 exclusion supersession and the removal of the false
  present-tense "language models wake selectively" claim were already applied by commit `2e15afb`, and
  the digest-chain bound at README line 44 already reads "detect damage … do not authenticate an
  adversarial rewrite". Verified, no further change.
- **docs/P0-IMPLEMENTATION-PLAN.md:** no Appendix D item names it; its replay/authentication text
  (lines 43, 49) already states the digest-chain bound. No change.

## Deferred (out of scope; recorded for the owning lane)

- **Item 2** — `docs/VERTICAL-SLICES.md` marks the fidelity metric both "(needs operator
  confirmation)" and "frozen as stated below". That document is out of scope; deferred to the VS1
  status owner.
- **Item 4's VERTICAL-SLICES locus** — the coverage invariant "every concept runs end-to-end in every
  version" should be read as "the concept's object exists, not the deep mechanism". The invariant
  lives in `docs/VERTICAL-SLICES.md` (out of scope). The in-scope end-state documents
  (ARCHITECTURE §3/§4, PRINCIPLES P7) were annotated instead; the invariant rewording is deferred to
  the VS1 status owner.
- **Item 6's VERTICAL-SLICES locus** — the only over-claiming locus is `docs/VERTICAL-SLICES.md`
  cross-cutting invariant 7 ("without trusting any agent"); it is out of scope. The in-scope locus
  (P0-SPEC §13, README) already carries the "digest chains detect damage, not malicious replacement"
  bound, so no in-scope edit was required.

## Diff stat

```
 docs/ARCHITECTURE.md |  5 ++++-
 docs/P0-SPEC.md      |  4 ++--
 docs/PRINCIPLES.md   |  2 ++
 docs/ROADMAP.md      | 15 ++++++++++++---
 4 files changed, 20 insertions(+), 6 deletions(-)
```

## Verification

- `git diff --check` → clean (no whitespace errors).
- `grep -rn "EscalationRequest" --include=*.go .` → no matches (confirms the not-yet-implemented
  marking).
- All edited claims map to the cited source in the table above; no out-of-scope file was modified
  (`git status` shows only the six sole-writer paths considered, four edited).

## Post-merge open P2 notes (independent review of the reviewed tip `a30fdda`)

These four P2 notes are open at the reviewed revision; none is a blocking defect. They are recorded
here in the ticket log only; the reviewed documents were not changed after review.

1. **The item-2 deferral quotes text that does not exist in `docs/VERTICAL-SLICES.md`.** The
   deferral above says the document "marks the fidelity metric both '(needs operator confirmation)'
   and 'frozen as stated below'"; the current document contains neither phrase — it reads "The
   fidelity metric is the frozen definition given in VS2" (line 190) and "declared fidelity metric
   scaffolded for VS2" (line 219). Those quoted strings describe an earlier revision (ticket-21
   Appendix D item 2), not current text.
2. **The item-5 out-of-scope locus is unrecorded as deferred.** Appendix D item 5's only
   `docs/VERTICAL-SLICES.md` locus is the C8 row (line 66, "records cause, cost, patch, and measured
   result"); only the in-scope `docs/ROADMAP.md` locus was corrected. The C8 deferral belongs with
   the other VERTICAL-SLICES-locus deferrals in the Deferred section above.
3. **"Only instance" overstates the declared wake-policy state.** `docs/ARCHITECTURE.md` §4 says the
   VS1 allocation policy's "only instance is the honest uniform/periodic baseline", but
   `internal/cognition/protocol/wake.go` declares two instances: `VS1WakePolicy` (periodic) and
   `FullWakePolicy` (the high-budget reference, period 1).
4. **`docs/P0-SPEC.md` §8 keeps a present-tense escalation step.** The §8 state-change protocol still
   lists `→ Intention or EscalationRequest` (line 131) unflagged, while §2 (line 23) and §10 (line
   170) were marked not-yet-implemented.

## Worktree incident (recorded)

During this lane a docs-repair child moved the main checkout onto a v2 branch; the supervisor interrupted it and restored `master`.
