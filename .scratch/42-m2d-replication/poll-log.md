# Ticket 42 poll log

Liveness is the runner **pid** *and* per-cell **ledger growth** — never a pid file, never a log line (the
report is written only at exit, so an empty log proves nothing). Spend is recomputed from every
`kind == "call"` row's `usage` at $0.10/M input and $0.50/M output. Stop rule (§5.3, as amended before the
launch): realised spend **> $1.30**, or a realised rate projecting above the **$1.40** hard cap — stop the
run by its recorded pid only, never `pkill -f`.

Two invocations, sequential, each `-parallel 12` with its own fresh run root (§2.2-§2.3). Cell 1 is
`f12-s0` (root `.tmp/42-runs-f12-s0`); cell 2 is `f10-s1` (root `.tmp/42-runs-f10-s1`) and starts only
after cell 1 exits.

Table columns: poll time (UTC) · runner pid alive (etime, state) · cells with a ledger · charged attempts
(cumulative, and Δ since the previous poll) · tokens in/out · spend (cumulative, and Δ) · realised
per-attempt price · note.

## Cell 1 — `f12-s0`

| # | UTC | pid alive | ledgers | attempts (Δ) | in/out tok | spend (Δ) | $/attempt | note |
|---|---|---|---|---|---|---|---|---|
| 0 | 2026-10-18T17:30:03Z (t+8 s) | yes, `Ssl` 00:08, pid **2772812** (`sid = pgid = pid = 2772812`; parent 2772810 is the `setsid nohup` wrapper, not the runner) | 9 of 12 | 18 (first) | 11 317 / 2 160 | $0.0022 (first) | $1.229e-4 | launched `2026-10-18T17:29:55Z` (process `lstart` `Mon Oct 19 01:29:55 2026` at `+0800`); 12 cell dirs, `A0-f12-s0-r1..3` make no calls by construction, `P-f12-s0-r1/r2` had not yet charged one. Prompt size measured **629 tok/call**, the same as `f10-s0`'s 628, so the §5.1 projection (≈ $1.15 for the 5 952-attempt matrix) holds rather than growing with founders |
| 1 | 2026-10-18T17:34:08Z (t+4:13) | yes, `Ssl` 04:12 | 12 of 12 | 891 (+873 in 245 s ≈ **214/min**) | 560 663 / 164 869 | $0.1385 (+$0.1363) | $1.554e-4 | all 12 model-bearing cells live in one wave; every cell growing (lowest `A-r1` 62, highest 79); one non-ok row: `A1-f12-s0-r1:158` `http-error 400` (content policy, the same class as ticket 41's single 400, one attempt, no retry). **Rate projection**: 5 952 attempts × $1.554e-4 = **≈ $0.93**, below both the $1.30 stop and the $1.15 §5.1 projection, because the realised output tokens are lower than ticket 41's (185/call vs 258); the 12-lane aggregate matches ticket 41's ≈200-214/min |
