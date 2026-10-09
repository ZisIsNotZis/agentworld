# Independent adversarial vision-fidelity audit — operator-raised drift concern

- **Trigger (operator, verbatim intent):** "AI has a chance to intervene, to dynamically modify the
  strategy in real time, and let it self-evolve — rather than a very boring simulation machine. Have
  many of our core designs never been verified at all? I feel we have drifted far from our original
  ideal."
- **Audit:** fresh-context, read-only, adversarial, run `a0066d96-dc4e-4646-8cab-7ee4ef54836e`;
  full report in this directory (`audit-vision-fidelity.md`).
- **Verdict: DRIFT PARTIALLY CONFIRMED.** Facts confirmed: the differentiating design (cognitive-value
  scheduling, model calls, intervention ledger, allocation-policy comparison, strategy
  sandbox/rollout/rollback, escalation requests) is 0% implemented and 0% exercised; ADR 0002/0004
  promise an end-state with no code; the repo has no LLM call path at all; `TokenCost` is hard-coded 0
  in every reporter; `WakeStrategyInvalidation` has zero producers. ~89% of post-foundation effort
  went to a 16-actor synthetic economy; the last five tickets measured finer properties of constants
  the project itself froze (v5 supply 16 = burn 16 ⇒ sterile null at the declared fixture), then
  measured a way out via a density axis the project also invented. Mitigations that keep the verdict
  "partial": the substitution was declared (P0's roadmap excludes LLM strategy editing), every result
  is labelled model-conditional, every null is reported verbatim, and the substrate produced is the
  instrument the missing experiment needs — tickets 18-19 inadvertently delivered the most valuable
  artifact for it: a measured map of where decisions matter (f=10/11 reliable, f=12 fragile, f=16
  arithmetic null) and a datum that decision quality changes outcomes exactly there (natural/7 2/8 vs
  natural/7b 7/8 at f=12).
- **Audit's 3 cheapest corrective actions (value-to-central-claim / cost):** (1) run a stub-oracle
  allocation experiment on the existing fixture at equal oracle-call budget (periodic, uniform-random,
  threshold-aimed adaptive, full reference) over f=10-12 × seeds 0-7 using only existing seams; (2)
  widen the decision space with a per-actor heterogeneity axis before adding more economics, reusing
  the ticket-18/19 machinery to produce a decision-sensitivity map; (3) reconcile the docs with the
  measured state (P1 not started; honest verdict table; retire ticket-13's "productive forces emerged"
  overstatement; correct README's present-tense claim that language models "wake selectively").
- **Operator decision requested:** whether to stop the materialist rung-by-rung arc here and build the
  missing subject (budgeted selective reasoning) on the substrate just measured.
