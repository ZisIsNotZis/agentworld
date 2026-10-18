# Ticket 41 poll log

Liveness is the runner **pid** *and* per-cell **ledger growth** — never a pid file, never a log line (the
report is written only at exit, so an empty log proves nothing). Spend is recomputed from every
`kind == "call"` row's `usage` at $0.10/M input and $0.50/M output. Stop rule: realised spend > $1.04, or a
projection above $1.30 — stop by the recorded pid only, never `pkill -f`.

Table columns: poll time (UTC) · runner pid alive (etime) · cells with a ledger · charged attempts
(cumulative) · Δ attempts since the previous poll · tokens in/out · spend (cumulative) · note.

| # | UTC | pid alive | ledgers | attempts | Δ | in/out tok | spend | note |
|---|---|---|---|---|---|---|---|---|
| 0 | 2026-10-18T16:55:06Z (t+9 s) | yes, `Ssl` 00:09 | 11 of 12 | 28 | — (first) | 17 634 / 3 351 | $0.0034 | launched 16:54:57Z, pid 2759873; 12 cell dirs, `A-f10-s0-r1` had not yet charged a call |
| 1 | 2026-10-18T16:59:53Z (t+4:56) | yes, `Ssl` 04:56 | 12 of 12 | 980 | +952 in 287 s | 616 077 / 278 881 | $0.2010 | all 12 model-bearing cells live, one wave (no queued tail); rate ≈ 201 attempts/min, ≈ 2× the 8-lane aggregate of ticket 39; loadavg 1.31 on 8 cores; one `http-error 400` (content policy) at `A-f10-s0-r2:243`; projection ≈ 2 976 attempts ≈ $0.61, inside the cap |
| 2 | 2026-10-18T17:04:29Z (t+9:32) | yes, `Ssl` 09:32 | 12 of 12 | 1 964 | +984 in 276 s (≈ 214/min) | 1 234 754 / 520 437 | $0.3837 | every cell growing in the interval (smallest +29, `A-f10-s0-r1`); no new non-ok row; projection unchanged at ≈ $0.58–0.61 for the full matrix, ~5 min of work left |
| 3 | 2026-10-18T17:09:03Z (t+14:05) | yes, `Ssl` 14:05 | 12 of 12 | 2 841 | +877 in 274 s (≈ 192/min) | 1 786 303 / 710 575 | $0.5339 | 5 cells are at their whole decision budget (`A-r2`, `A-r3`, `A1-r1`, `U-r1`, `U-r3` = 248 attempts exactly, no retry spent), `P-r1`/`P-r2` stopped at their gate-bound 244; 4 cells still growing (`A-r1` 211, `A1-r3` 222, `U-r2` 206, `P-r3` 228); ≈ 135 attempts of work left; log still 0 B and no evaluation JSON, both correct before exit |
