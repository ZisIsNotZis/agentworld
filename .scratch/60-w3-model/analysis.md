# Stage M analysis — model: each non-reference model's distance to `R@max`

Rule (pre-registered, `.scratch/58-w3-preregistration/spec.md` §3/§3.1, unchanged): a scalar
KPI separates when `SNR = RMSE(d)/noise >= 2` **and** `|d(t)|` exceeds the replicate envelope
for >= 24 consecutive hours. The histogram families are carried by Wasserstein-1 over buckets.
No scalar weighting exists, and no model is ranked: the `luna` reference is the benchmark, not
an optimum.

Identity of the runs compared:

| run | model_id | wire | effort | contract | H | replicates |
|---|---|---|---|---|---:|---:|
| w3max | github_copilot/gpt-6-luna | responses | max | natural-claim/v4 | 144 | – |
| t57max | github_copilot/gpt-6-luna | responses | max | natural-claim/v4 | 144 | – |
| msol | github_copilot/gpt-6-sol | responses | max | natural-claim/v4 | 144 | – |

## Accounting (calls, tokens, derived cost, wall)

| model | calls | prompt tok | completion tok | derived $ | cell seconds (sum of reps) | wall |
|---|---:|---:|---:|---:|---:|---:|
| gpt-6-sol | 4424 | 2777574 | 2052464 | 1.3040 | 5137 | 35.2 min |
| **stage total (Stage M)** | **4424** | **2777574** | **2052464** | **1.3040** | | |
| luna@max (Stage-E rows, not re-run) | 4574 | 2876424 | 3185073 | 1.8802 | (previously recorded) | |

## Decision agreement with the frozen cascade (descriptive)

| condition | calls | action == frozen action | fraction | sources | statuses | wake hours | latency p50 / p95 / max ms |
|---|---:|---:|---:|---|---|---:|---|
| sol R | 4424 | 4189 | 0.9469 | model | ok | 144 | 6186 / 12023 / 43453 |

## §3.1 — each non-reference model's distance to `luna@max` (cell `f10-s0`)

`first` = first separating hour (-1 none); `run` = longest separating run and its start;
`power` = smallest separable constant difference; `dir` = direction of a *detected* separation.

### `sol@max` − `luna@max` (3 replicates each)

| KPI | RMSE(d) | noise | SNR | first | run (start) | power | max abs d | d at H | dir | W1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| population | 1.006 | 1.118 | 0.900 | 134 | 7 (137) | 2.236 | 4.000 | -3.333 | - | 0.340 |
| births_cum | 1.471 | 1.440 | 1.021 | 133 | 12 (133) | 2.880 | 5.667 | -5.667 | - | 0.522 |
| deaths_starve_cum | 0.488 | 0.649 | 0.752 | -1 | 0 (-1) | 1.299 | 2.333 | -2.333 | - | 0.177 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | 0.000 | 0.000 | 0.000 | - | 0.000 |
| max_capital | 0.138 | 0.212 | 0.654 | -1 | 0 (-1) | 0.423 | 0.667 | 0.000 | - | 0.002 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | 0.000 | 0.000 | 0.000 | - | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.018 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.066 |

No KPI separates: `sol@max` matches `luna@max` at this design (no detectable effect at this design, never equivalence).

Curve agreement (descriptive): the declared scalars differ at all at **33** hours
(first h109, last h144).

## Endpoints (mean over each condition's 3 replicates, hour 144)

| condition | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| luna@max (reference) | 15.67 | 20.00 | 4.33 | 10.00 | 1.00 |
| sol@max | 12.33 | 14.33 | 2.00 | 10.00 | 1.00 |

## Stage-M verdict (§3.1)

The reference is the benchmark; a model is neither ranked nor called better.

| model | relation to `luna@max` | KPIs that separate |
|---|---|---|
| sol | matches | – |

