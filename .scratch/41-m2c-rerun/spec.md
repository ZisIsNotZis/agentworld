# Ticket 41 — M2c part 3: the re-run of the M2c matrix under the transport reserve

- Record kind: the run record of the M2c re-run. Created before the run, because what the re-run's
  accounting is expected to say must be declared before the numbers exist.
- Status: **not run.** No live call, no ledger, no run root and no artefact exist for this ticket; §2 is
  empty by declaration, not by omission.
- Inputs, in order: the pre-registered criterion (`.scratch/38-m2c-preregistration/spec.md` §7), the repair
  and re-pre-registration this run executes (`.scratch/40-m2c-transport-reserve/spec.md` §3, §7), and
  ticket 39's diagnosis of the first run (`.scratch/39-m2c-matrix/spec.md` §11).
- Authority: ticket 38 §8.1 — hard cap `$1.30` per run and 7 000 physical attempts cumulative across runs,
  with the operator stop at `$1.04` realised spend or a projection above `$1.30` (ticket 40 §7).

## 1. The expected accounting state of every budgeted cell (declared before the run)

Every budgeted cell of this matrix reports `under_budget: true`, because the schedule now selects
`N − R = 248` decisions inside the unchanged `N = 256` attempt ceiling: the quota-bound cells (`U`, `A`,
`A1`) report `realized_calls = 248` with `budget_shortfall = 8`, and `P`'s cells report their own
gate-bound count (244 in run 1) with the matching shortfall (12), as they already did before the reserve.
This is the **expected** state of every budgeted cell, not a lost decision and not an alarm: 248 is the
whole decision budget the arms are compared on, 8 is the declared transport reserve left unspent for
retries, and `VS2-EC-2` counts `under-budget` as a diagnostic without failing on it. What alarms a cell is
`halted`, or `attempts − decisions > 8`.

## 2. The run itself

Not yet run. When it lands, this section records, at the revision the run executes: the launch command and
the detached pid, the run root, the per-cell rows (`realized_calls`, attempts, `halted`, `ceiling_hit`),
the realised spend and the attempt total against the §8.1 caps, and the scorer's inventory verdict — which
is either a 15-cell complete matrix that the pre-registered rule may score, or `NOT RUN` again, never a
smaller experiment scored as a result.
