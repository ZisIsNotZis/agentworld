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
