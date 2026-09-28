# Foundation documentation

Status: done

## Issue

The project concept exists in discussion but lacks a durable, internally consistent source of truth.

## Objective

Document the approved vision, principles, conceptual architecture, validation method, staged roadmap, terminology, and open research questions without prematurely freezing implementation details.

## Acceptance criteria

- The project objective and non-goals are explicit.
- Quantitative projections are a hard design constraint even when semantic state uses text.
- The v0 population model uses explicit actor fibers as its lower fidelity bound.
- Competing cognitive-allocation strategies can be evaluated through branched or beam-style runs.
- Claims, hypotheses, and open questions are distinguishable.
- A fresh reader can identify the first prototype and its exit criteria.
- Documentation is internally cross-linked and contains no competing source of truth.

Need-review: yes
Need-test-cases: no

## Blockers

None.

## Comments

- 2026-09-17 — assistant: Created from the approved design discussion and began the first durable documentation pass.
- 2026-09-17 — assistant: Added the project entry point, seven topic documents, and four ADRs. Local Markdown links resolve; every authored document remains under the 200-line and 12,000-character default budget; `git diff --check` passes.
- 2026-09-17 — assistant: Fresh scoped self-review covered correctness, completeness, actionability, terminology, premature commitments, and cross-document consistency. Verdict: accepted. The open implementation choices remain explicitly deferred to P0 or P1 experiments.
