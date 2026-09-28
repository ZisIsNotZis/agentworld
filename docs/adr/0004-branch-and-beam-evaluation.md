# ADR 0004 — Branch and beam evaluation

Status: accepted for development experiments

## Context

The best cognitive-intervention policy is unknown and cannot be chosen reliably from intuition. Different allocation strategies may produce delayed effects, exploit different metric weaknesses, or preserve different rare outcomes.

## Decision

Support branches from a shared checkpoint and evaluate multiple cognitive schedulers or engine strategies under matched budgets and random inputs.

Development experiments may use beam-style continuation: periodically evaluate causal validity, a declared metric vector, uncertainty, cost, and branch diversity; retain several non-dominated candidates rather than one scalar-score winner.

Record pruning decisions and preserve representative rejected-branch evidence.

## Consequences

- Strategy selection becomes an empirical comparison.
- Branch ancestry and matched-budget configuration are required.
- Beam search increases development compute but can reduce commitment to a poor early policy.
- Results remain conditional on metric definitions and must include sensitivity analysis.
