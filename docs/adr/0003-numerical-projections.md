# ADR 0003 — Numerical projections for comparison

Status: accepted

## Context

The simulator contains semantic, relational, and possibly textual state, but cognitive scheduling, branch selection, calibration, and optimization require measurable values. Pure prose judgments cannot support reproducible comparisons.

A single civilization score would also encode hidden values and invite optimization artifacts.

## Decision

Require every comparison-relevant property to expose a versioned numerical projection with units or bounds, uncertainty, provenance, aggregation, calibration, and declared information loss.

Use metric vectors as the primary comparison representation. Scalar objectives are local experiment choices whose weights and sensitivity must be visible. Beam-style runs also enforce diversity and causal-validity constraints.

Language models may propose scores or annotations, but downstream decisions consume structured values linked to evidence, and predicted values are calibrated against observed outcomes.

## Consequences

- Metrics and projections become first-class versioned artifacts.
- Text remains useful for explanation but cannot be the sole basis of optimization.
- Metric design can bias the simulated society, so alternative projections and sensitivity analyses are required.
- A final world-distance function is deliberately not selected yet.
