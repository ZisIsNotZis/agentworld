# Stage E analysis — thinking (`-effort`) against `R@max` (ticket 58 §3)

Rule (pre-registered): a scalar KPI separates when `SNR = RMSE(d)/noise >= 2` **and**
`|d(t)|` exceeds the replicate envelope for >= 24 consecutive hours. The histogram families
are carried by Wasserstein-1 over buckets. Every KPI is reported; no scalar is defined.
A tie is 'no detectable effect at this design', never equivalence.

Reference: `R@max`, cell `f10-s0`, 3 replicates (t57max.r1, w3max.r1, w3max.r2).

Candidates: `R@<effort>` in the same cell, 3 fresh replicates each.

## Accounting

| effort | calls | prompt tok | completion tok | derived $ | wall (s) | wall (min) |
|---|---:|---:|---:|---:|---:|---:|
| none | 4365 | 2744739 | 224197 | 0.39 | 1050 | 17.5 |
| low | 4538 | 2849212 | 890702 | 0.73 | 1971 | 32.8 |
| medium | 4557 | 2864510 | 1152512 | 0.86 | 1502 | 25.0 |
| high | 4487 | 2820898 | 1360539 | 0.96 | 1528 | 25.5 |
| max | 2995 | 1883387 | 2123822 | 1.25 | 4151 | 69.2 |

## Per-effort KPI comparisons (effort − max)

### `R@none` vs `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | dir | smallest separable | band floor | 2×noise | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 1.045 | 0.832 | 1.256 | 124 | 9 (124) | - | 1.664 | 0.000 | 1.664 | 0.464 |
| births_cum | 1.417 | 1.131 | 1.253 | 124 | 11 (124) | - | 2.261 | 0.000 | 2.261 | 0.687 |
| deaths_starve_cum | 0.602 | 0.578 | 1.041 | -1 | 0 (-1) | - | 1.157 | 0.000 | 1.157 | 0.232 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| max_capital | 0.152 | 0.189 | 0.803 | -1 | 0 (-1) | - | 0.378 | 0.000 | 0.378 | 0.005 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.032 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.238 |

**Verdict `R@none`:** no detectable effect vs R@max at this design

| KPI | mean d at H | observed max |d| | smallest separable | reading |
|---|---:|---:|---:|---|
| population | -1.667 | 3.667 | 1.664 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| births_cum | -3.000 | 4.000 | 2.261 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_starve_cum | -1.333 | 2.333 | 1.157 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_age_cum | 0.000 | 0.000 | 0.000 | no difference observed |
| max_capital | 0.333 | 0.667 | 0.378 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |

### `R@low` vs `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | dir | smallest separable | band floor | 2×noise | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.429 | 1.256 | 0.341 | -1 | 0 (-1) | - | 2.512 | 0.000 | 2.512 | 0.115 |
| births_cum | 0.983 | 1.465 | 0.671 | -1 | 0 (-1) | - | 2.930 | 0.000 | 2.930 | 0.347 |
| deaths_starve_cum | 0.667 | 0.570 | 1.170 | -1 | 0 (-1) | - | 1.141 | 0.000 | 1.141 | 0.251 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| max_capital | 0.144 | 0.203 | 0.707 | -1 | 0 (-1) | - | 0.407 | 0.000 | 0.407 | 0.025 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.019 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.071 |

**Verdict `R@low`:** no detectable effect vs R@max at this design

| KPI | mean d at H | observed max |d| | smallest separable | reading |
|---|---:|---:|---:|---|
| population | -1.667 | 2.333 | 2.512 | observed max |d| below the smallest separable delta |
| births_cum | -3.667 | 4.000 | 2.930 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_starve_cum | -2.000 | 2.333 | 1.141 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_age_cum | 0.000 | 0.000 | 0.000 | no difference observed |
| max_capital | 0.333 | 0.667 | 0.407 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |

### `R@medium` vs `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | dir | smallest separable | band floor | 2×noise | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.349 | 1.009 | 0.346 | -1 | 0 (-1) | - | 2.018 | 0.000 | 2.018 | 0.094 |
| births_cum | 0.832 | 1.348 | 0.617 | -1 | 0 (-1) | - | 2.696 | 0.000 | 2.696 | 0.299 |
| deaths_starve_cum | 0.637 | 0.694 | 0.917 | -1 | 0 (-1) | - | 1.388 | 0.000 | 1.388 | 0.255 |
| deaths_age_cum | 0.055 | 0.068 | 0.816 | -1 | 0 (-1) | - | 0.136 | 0.000 | 0.136 | 0.009 |
| max_capital | 0.161 | 0.198 | 0.816 | -1 | 0 (-1) | - | 0.395 | 0.000 | 0.395 | 0.014 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.020 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.066 |

**Verdict `R@medium`:** no detectable effect vs R@max at this design

| KPI | mean d at H | observed max |d| | smallest separable | reading |
|---|---:|---:|---:|---|
| population | -0.333 | 1.333 | 2.018 | observed max |d| below the smallest separable delta |
| births_cum | -3.000 | 3.333 | 2.696 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_starve_cum | -2.333 | 2.333 | 1.388 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_age_cum | -0.333 | 0.333 | 0.136 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| max_capital | 0.000 | 0.667 | 0.395 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |

### `R@high` vs `R@max`

| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | dir | smallest separable | band floor | 2×noise | W1 |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| population | 0.455 | 1.095 | 0.415 | -1 | 0 (-1) | - | 2.190 | 0.000 | 2.190 | 0.221 |
| births_cum | 0.790 | 1.380 | 0.572 | -1 | 0 (-1) | - | 2.761 | 0.000 | 2.761 | 0.349 |
| deaths_starve_cum | 0.441 | 0.621 | 0.710 | -1 | 0 (-1) | - | 1.243 | 0.000 | 1.243 | 0.170 |
| deaths_age_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| max_capital | 0.136 | 0.209 | 0.649 | -1 | 0 (-1) | - | 0.418 | 0.000 | 0.418 | 0.005 |
| wages_cum | 0.000 | 0.000 | n/a | -1 | 0 (-1) | - | 0.000 | 0.000 | n/a | 0.000 |
| capital_hist | – | – | – | – | – | – | – | – | – | 0.016 |
| granary_hist | – | – | – | – | – | – | – | – | – | 0.085 |

**Verdict `R@high`:** no detectable effect vs R@max at this design

| KPI | mean d at H | observed max |d| | smallest separable | reading |
|---|---:|---:|---:|---|
| population | -1.333 | 2.000 | 2.190 | observed max |d| below the smallest separable delta |
| births_cum | -3.000 | 3.000 | 2.761 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_starve_cum | -1.667 | 1.667 | 1.243 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |
| deaths_age_cum | 0.000 | 0.000 | 0.000 | no difference observed |
| max_capital | 0.000 | 0.667 | 0.418 | observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length) |

## Reference band (`R@max`, 3 replicates)

| KPI | mean@H | 95% CI half-width@H | envelope@H | mean envelope half-width | max envelope half-width |
|---|---:|---:|---|---:|---:|
| population | 15.667 | 7.986 | 12.000..18.000 | 0.503 | 3.667 |
| births_cum | 20.000 | 9.937 | 16.000..24.000 | 0.738 | 4.000 |
| deaths_starve_cum | 4.333 | 6.252 | 2.000..7.000 | 0.329 | 3.333 |
| deaths_age_cum | 10.000 | 0.000 | 10.000..10.000 | 0.000 | 0.000 |
| max_capital | 1.000 | 0.000 | 1.000..1.000 | 0.092 | 0.667 |

## What the data establishes

No effort separates from `R@max` on any declared scalar KPI at this design.
No detectable effect at this design: `R@none`, `R@low`, `R@medium`, `R@high` (never equivalence).
The limit per effort is the smallest-separable column above; a difference below it is not
evidence. The histogram distances are reported, not tested by the 24-hour scalar rule.
