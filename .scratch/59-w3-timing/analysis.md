# Stage T analysis — timing (`A` vs `P`/`U` at N=256, and every arm's distance to `R@max`)

Rule (pre-registered, `.scratch/58-w3-preregistration/spec.md` §3): a scalar KPI separates when
`SNR = RMSE(d)/noise >= 2` **and** `|d(t)|` exceeds the replicate envelope for >= 24
consecutive hours. The histogram families are carried by Wasserstein-1 over buckets computed
here from the per-hour bucket vectors (the frozen `curve/1` histogram column is defective —
its integral runs over the hour index — and is not used). No scalar weighting exists; a tie is
'no detectable effect at this design', never equivalence.

Reference: `R@max`, cell `f10-s0`, 3 replicates (t57max.R.r1, w3max.R.r1, w3max.R.r2) — the Stage-E rows, not re-run here.

## Accounting

| arm | calls (3 reps) | prompt tok | completion tok | derived $ | cell seconds (sum of 3 reps) |
|---|---:|---:|---:|---:|---:|
| A0 | 0 | 0 | 0 | 0.0000 | 9 |
| P | 720 | 452739 | 484607 | 0.2876 | 6443 |
| U | 744 | 467727 | 489415 | 0.2915 | 6149 |
| A | 744 | 467802 | 508811 | 0.3012 | 6103 |
| **fresh total** | **2208** | **1388268** | **1482833** | **0.8803** | stage wall 107.4 min |
| R@max (Stage-E rows, not re-run) | 4574 | 2876424 | 3185073 | 1.8802 | (previously recorded) |

## §3.2 — the equal-budget comparisons (`A` vs `P`, `A` vs `U`)

### `A@max` − `P@max` (N = 256 both sides), cell `f10-s0`, 3 replicates each

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction (A−P) | power (smallest separable) | max abs d | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|
| population | 0.288 | 0.000 | n/a | 122 | 6 (134) | - | 0.000 | 1.000 | 0.083 |
| births_cum | 0.275 | 0.000 | n/a | 122 | 6 (134) | - | 0.000 | 1.000 | 0.076 |
| deaths_starve_cum | 0.083 | 0.000 | n/a | 144 | 1 (144) | - | 0.000 | 1.000 | 0.007 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| max_capital | 0.083 | 0.000 | n/a | 143 | 1 (143) | - | 0.000 | 1.000 | 0.007 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | 0.012 |
| granary_hist | – | – | – | – | – | – | – | – | 0.037 |

No KPI separates in either direction.

### `A@max` − `U@max` (N = 256 both sides), cell `f10-s0`, 3 replicates each

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction (A−U) | power (smallest separable) | max abs d | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|
| population | 0.914 | 0.000 | n/a | 110 | 14 (131) | - | 0.000 | 3.000 | 0.310 |
| births_cum | 0.914 | 0.000 | n/a | 110 | 14 (131) | - | 0.000 | 3.000 | 0.352 |
| deaths_starve_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| max_capital | 0.166 | 0.000 | n/a | 130 | 2 (132) | - | 0.000 | 1.000 | 0.014 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | 0.023 |
| granary_hist | – | – | – | – | – | – | – | – | 0.109 |

No KPI separates in either direction.

## §3.1 — each arm's distance to `R@max` (same cell `f10-s0`, 3 replicates each)

### `A@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|
| population | 0.988 | 0.705 | 1.400 | 133 | 3 (133) | - | 1.411 | 3.667 | 0.457 |
| births_cum | 1.560 | 0.999 | 1.562 | 133 | 7 (138) | - | 1.998 | 5.000 | 0.729 |
| deaths_starve_cum | 0.756 | 0.556 | 1.360 | 144 | 1 (144) | - | 1.112 | 3.333 | 0.280 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| max_capital | 0.157 | 0.152 | 1.033 | -1 | 0 (-1) | - | 0.303 | 0.667 | 0.000 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | 0.034 |
| granary_hist | – | – | – | – | – | – | – | – | 0.170 |

No KPI separates: `A@max` matches `R@max` at this design.

### `P@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|
| population | 1.145 | 0.705 | 1.623 | 133 | 7 (133) | - | 1.411 | 3.667 | 0.540 |
| births_cum | 1.722 | 0.999 | 1.724 | 133 | 12 (133) | - | 1.998 | 5.667 | 0.805 |
| deaths_starve_cum | 0.730 | 0.556 | 1.312 | -1 | 0 (-1) | - | 1.112 | 2.667 | 0.274 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| max_capital | 0.164 | 0.152 | 1.080 | -1 | 0 (-1) | - | 0.303 | 0.667 | 0.007 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | 0.034 |
| granary_hist | – | – | – | – | – | – | – | – | 0.175 |

No KPI separates: `P@max` matches `R@max` at this design.

### `U@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|
| population | 1.537 | 0.705 | 2.178 | 131 | 14 (131) | - | 1.411 | 5.333 | 0.639 |
| births_cum | 2.229 | 0.999 | 2.232 | 131 | 14 (131) | - | 1.998 | 8.000 | 0.929 |
| deaths_starve_cum | 0.756 | 0.556 | 1.360 | 144 | 1 (144) | - | 1.112 | 3.333 | 0.280 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| max_capital | 0.171 | 0.152 | 1.125 | -1 | 0 (-1) | - | 0.303 | 0.667 | 0.014 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | 0.037 |
| granary_hist | – | – | – | – | – | – | – | – | 0.204 |

No KPI separates: `U@max` matches `R@max` at this design.

### `A0@max` − `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|
| population | 1.537 | 0.705 | 2.178 | 131 | 14 (131) | - | 1.411 | 5.333 | 0.639 |
| births_cum | 2.229 | 0.999 | 2.232 | 131 | 14 (131) | - | 1.998 | 8.000 | 0.929 |
| deaths_starve_cum | 0.756 | 0.556 | 1.360 | 144 | 1 (144) | - | 1.112 | 3.333 | 0.280 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| max_capital | 0.171 | 0.152 | 1.125 | -1 | 0 (-1) | - | 0.303 | 0.667 | 0.014 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | 0.037 |
| granary_hist | – | – | – | – | – | – | – | – | 0.204 |

No KPI separates: `A0@max` matches `R@max` at this design.

## Curve agreement (descriptive: how many hours two conditions differ on ANY scalar)

No threshold and no verdict here; it tells the reader whether a tie is a tie between two
distinct worlds or between two runs of the same world. The five declared scalars are compared
hour by hour; the replicates of every arm have identical scalar series (0 differing hours).

| pair | hours with any scalar difference | first | last |
|---|---:|---:|---:|
| A vs P | 13 | 122 | 144 |
| A vs U | 28 | 110 | 144 |
| A vs A0 | 28 | 110 | 144 |
| A vs R | 37 | 108 | 144 |
| P vs U | 33 | 110 | 144 |
| P vs A0 | 33 | 110 | 144 |
| P vs R | 37 | 108 | 144 |
| U vs A0 | 0 | - | - |
| U vs R | 37 | 108 | 144 |
| A0 vs R | 37 | 108 | 144 |

## Endpoints (mean over each condition's 3 replicates, hour 144)

| condition | alive@H | births@H | starve@H | age@H | max_capital@H |
|---|---:|---:|---:|---:|---:|
| R@max (Stage E) | 15.67 | 20.00 | 4.33 | 10.00 | 1.00 |
| A@max | 14.00 | 15.00 | 1.00 | 10.00 | 1.00 |
| P@max | 13.00 | 15.00 | 2.00 | 10.00 | 1.00 |
| U@max | 11.00 | 12.00 | 1.00 | 10.00 | 1.00 |
| A0@max | 11.00 | 12.00 | 1.00 | 10.00 | 1.00 |

## Stage-T verdict (§3.2)

**not supported** — A ties with at least one baseline (no detectable separation in either direction).

| baseline | A's outcome |
|---|---|
| P | matches |
| U | matches |

'Supported' in §3.2 (M_end) additionally requires the pattern to reproduce at a second cell and
seed; that is Stage S and is not claimed here.
