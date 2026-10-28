# Stage S analysis — stability on `f11-s2` (`A` vs `P`/`U` at N=256, distances to in-run `R@max`)

Rule (pre-registered, `.scratch/58-w3-preregistration/spec.md` §3, Stage S amended to one cell):
a scalar KPI separates when `SNR = RMSE(d)/noise >= 2` **and** `|d(t)|` exceeds the replicate
envelope for >= 24 consecutive hours. The histogram families are carried by Wasserstein-1 over
buckets computed here from the per-hour bucket vectors. No scalar weighting exists; a tie is
'no detectable effect at this design', never equivalence; no cell is pooled.

Reference: **in-run** `R@max`, cell `f11-s2`, 3 fresh replicates (s.R.r1, s.R.r2, s.R.r3). It is not the f10-s0
reference; a second cell is a different world draw and is compared, never merged.

## Accounting

| arm | calls (3 reps) | prompt tok | completion tok | derived $ | cell seconds (sum of 3 reps) |
|---|---:|---:|---:|---:|---:|
| A0 | 0 | 0 | 0 | 0.0000 | 66 |
| R | 4452 | 2800139 | 2716989 | 1.6385 | 9570 |
| P | 744 | 467913 | 456580 | 0.2751 | 6906 |
| U | 744 | 468060 | 409450 | 0.2515 | 6677 |
| A | 744 | 467865 | 455363 | 0.2745 | 6888 |
| **fresh total (Stage S)** | **6684** | **4203977** | **4038382** | **2.4396** | stage wall 189.6 min |

Recorded charge is $0.00 (the gateway is unpriced); the derived price uses the ticket-35
rates ($0.10/M input, $0.50/M output), the same method as Stages E/T/M.

## §3.2 — the equal-budget comparisons (`A` vs `P`, `A` vs `U`)

### `A@max` − `P@max` (N = 256 both sides), cell `f11-s2`, 3 replicates each

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction (A−P) | power (smallest separable) | max abs d | d at H | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| births_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| max_capital | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.000 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.000 |

No KPI separates in either direction.

### `A@max` − `U@max` (N = 256 both sides), cell `f11-s2`, 3 replicates each

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction (A−U) | power (smallest separable) | max abs d | d at H | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| births_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| max_capital | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.000 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.000 |

No KPI separates in either direction.

## §3.1 — each arm's distance to the in-run `R@max` (cell `f11-s2`, 3 replicates each)

### `A@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | d at H | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| births_cum | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| max_capital | 0.039 | 0.048 | 0.816 | -1 | 0 (-1) | - | 0.096 | 0.333 | 0.000 | 0.005 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.001 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.003 |

No KPI separates: `A@max` matches `R@max` at this design.

### `P@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | d at H | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| births_cum | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| max_capital | 0.039 | 0.048 | 0.816 | -1 | 0 (-1) | - | 0.096 | 0.333 | 0.000 | 0.005 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.001 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.003 |

No KPI separates: `P@max` matches `R@max` at this design.

### `U@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | d at H | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| births_cum | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| max_capital | 0.039 | 0.048 | 0.816 | -1 | 0 (-1) | - | 0.096 | 0.333 | 0.000 | 0.005 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.001 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.003 |

No KPI separates: `U@max` matches `R@max` at this design.

### `A0@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | d at H | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| births_cum | 0.073 | 0.090 | 0.816 | -1 | 0 (-1) | - | 0.179 | 0.333 | 0.333 | 0.016 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| max_capital | 0.039 | 0.048 | 0.816 | -1 | 0 (-1) | - | 0.096 | 0.333 | 0.000 | 0.005 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.001 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.003 |

No KPI separates: `A0@max` matches `R@max` at this design.

## A0 control and the A0−U identity (descriptive)

`A0` makes zero calls. At `f11-s2` its three replicates are identical on the scalars
(endpoints population=12.00, births_cum=12.00, deaths_starve_cum=0.00, deaths_age_cum=11.00, max_capital=2.00).
`U@max` vs `A0`: 0 hours with any scalar difference (first -, last -); identical.
The paired A0−U rule, for completeness (both may be deterministic, so SNR is `n/a`):
no KPI separates

## Curve agreement (descriptive: hours two conditions differ on ANY scalar)

No threshold and no verdict; it tells the reader whether a tie is between two distinct worlds
or two runs of the same world. First replicate of each condition.

| pair | hours with any scalar difference | first | last |
|---|---:|---:|---:|
| A vs P | 0 | - | - |
| A vs U | 0 | - | - |
| A vs A0 | 0 | - | - |
| A vs R | 0 | - | - |
| P vs U | 0 | - | - |
| P vs A0 | 0 | - | - |
| P vs R | 0 | - | - |
| U vs A0 | 0 | - | - |
| U vs R | 0 | - | - |
| A0 vs R | 0 | - | - |

## Endpoints at H = 144 (mean over each condition's 3 replicates)

| condition | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| R@max (in-run) | 11.67 | 11.67 | 0.00 | 11.00 | 2.00 |
| A@max | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |
| P@max | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |
| U@max | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |
| A0@max | 12.00 | 12.00 | 0.00 | 11.00 | 2.00 |

## Stage-S verdict on `f11-s2` (§3.2)

**not supported** — A ties with at least one baseline (no detectable separation in either direction).

| baseline | A's outcome | resolution state |
|---|---|---|
| P | matches | identical curves (zero difference at every hour) |
| U | matches | identical curves (zero difference at every hour) |

## Reproduction of the Stage-T f10-s0 pattern

- `f10-s0` (Stage T, cited): **A matches P and U; U identical to A0**
- `f11-s2` (this cell): **A matches P and U; U identical to A0**
- **reproduces**

## Cross-cell endpoints (descriptive; no pooling, no verdict)

| arm | f10-s0 alive/births/starve/age/maxcap | f11-s2 alive/births/starve/age/maxcap | f11-s2 − f10-s0 (vector) |
|---|---|---|---|
| A0 | 11.0/12.0/1.0/10.0/1.0 | 12.0/12.0/0.0/11.0/2.0 | 1.0/0.0/-1.0/1.0/1.0 |
| P | 13.0/15.0/2.0/10.0/1.0 | 12.0/12.0/0.0/11.0/2.0 | -1.0/-3.0/-2.0/1.0/1.0 |
| U | 11.0/12.0/1.0/10.0/1.0 | 12.0/12.0/0.0/11.0/2.0 | 1.0/0.0/-1.0/1.0/1.0 |
| A | 14.0/15.0/1.0/10.0/1.0 | 12.0/12.0/0.0/11.0/2.0 | -2.0/-3.0/-1.0/1.0/1.0 |

The cross-cell table is a description of two different world draws, not a test: the declared
rule is per cell, and this stage declares one cell. The §3.2 'supported' category would
additionally need `A` to beat both baselines in every declared cell; the amended Stage S has
one cell, so a reproduction here answers the stability question, not M_end.
