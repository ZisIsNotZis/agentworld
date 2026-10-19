Produced by a fresh independent reviewer at commit `03b0467`; copied here unedited by the fixer from `/home/z/.pi/agent/sessions/--home-z-vibe-agentworld--/subagent-artifacts/24aaf3a3-41a9-40e8-8f53-8aa9ec2766f9_reviewer_output.md`, its source.

## Review

**Target:** `.scratch/43-rung2-seed/` at HEAD `03b0467d490f3af1f3659d44d8471c81779cc769` (reflog: parent `bca7ddc`, on `master`; `origin/master` = `03b0467`; working tree clean vs HEAD per `watchdog_diff`). The worker's central claim — rung 2 **FAILS**, 0/128 cells pass — is **independently reproduced** from the tracked artefacts and I found no defect in the verdict itself. Two P1 defects are in the *record*, not the measurement.

### Correct (re-derived, not trusted)
- **Zero-spend / zero-call proof holds.** `.scratch/43-rung2-seed/a0-grid.json` (`arms: ["A0"]`, every cell `realized_calls/prompt_tokens/completion_tokens/cost_micros` = 0, `zero_spend.cells_nonzero_calls_or_cost: 0`, `inventory {declared:128, ran:128, skipped:null}`) and `runner-report.txt:12,17ff` agree. Structurally consistent with `cmd/vs2/harness.go:102-103` (`ArmA0` → `SetCognition(nil)`, no ledger).
- **Verdict arithmetic is exact.** From `a0-grid.tsv` I recomputed: 127/128 `alive_end<N0`; 80/128 starvation>0; 34/128 zero births (f=1,8,9,16 all seeds + f7-s3, f15-s2); exactly one cell reaches `alive_end≥N0` (`f6-s6`, 6≥6, 9 starvation deaths); `alive_end` max 6. Totals also close: starvation 280, age 1010, all deaths 1290 (21.7 % / 78.3 %) — matches `spec.md` §6.6.
- **Criterion is the declared rung-2 criterion, not a weakened restatement.** `PASS := alive_end>=N0 ∧ starve==0 ∧ births>=1` (`spec.md` §1, `reduce-a0-grid.py` 58-64) is verbatim the `docs/VERTICAL-SLICES.md:119` "done when", including the all-three conjunction. No cell is scored "alive ≥ 1".
- **Anchors reproduce against the independent record.** `f10-s0` 1444/2/0/1 (`scores.json:23,56` H=1444, N0=10), `f12-s0` 1700/5/3/1 and `f10-s1` 1496/2/0/1 (`.scratch/42-m2d-replication/spec.md` §8.1, samples 0..144). H=144 is also *more* favourable than the runner default `DeclaredHorizonHours=168` (`cmd/vs2/config.go:228`), so the FAIL is conservative, not relaxed.
- **Grid exhaustiveness is real, not a truncated sweep.** `NaturalFoundersValid` binds `1 ≤ f ≤ 8·patches = 16` (`internal/world/natural.go:580-589`); `cmd/vs2/config.go:249-263` gives primary seeds 0-3 and sensitivity 4-7. 16×8 = 128, all present in the tsv, none skipped.
- **VS2-EC-1 FAIL explanation is correct, not an excuse.** The band compares against ticket-19 recordings whose declared horizon is 168 (`config.go:228`, `main.go:42`), while this run used `-hours 144`; hence `alive_by_hour/events/history_bytes` mismatches at every recorded cell (`runner-report.txt:151-157`). It is not the rung-2 verdict.
- **Docs agree with `docs/VALIDATION.md:68`** ("a die-off under founder aging", control ends at one alive) — no contradiction of the recorded anchor caveat. Rung 1/3/4/5 rows and the M2 row carry no ticket-43 content.
- **Binding mechanism is measured, not narrated.** Age deaths 11-14, 81.25-100 % of primary-cell deaths (min at `f12-s0` 13/16), first age death h99-h103; the zero-birth cells make the birth clause independently binding; starvation 0 at all f=10 seeds. A FAIL with this evidence satisfies acceptance (c).

### Finding P1 — the birth/age-hour claim in the record contradicts its own tracked artefact
`spec.md:181-182` states "`births` occur only at **h5–h8** (the primary cells' birth hours are `[5,6]` at f=10, `[5,7,25]` or `[5,7,8/9]` at f=11, `[5..8,23]` at f=12)" — the sentence refutes itself (25 and 23 are listed as birth hours) and the artefact refutes it further: `a0-grid.json` gives `f11-s1` birth_hours `[5,7,68]` (primary cell, s=1) and `f12-s0` `[5,6,7,8,23]`; the full grid also has `f11-s6` h17, `f11-s7` h64, `f12-s6` h117. Same sentence says age deaths run "to `H` (h140–h144)"; measured last age death at primary cells is h138 (`f10-s2`, `f11-s2`). The identical false statement is propagated to `docs/VERTICAL-SLICES.md:119` ("births occur only at h5–h8, so every actor dies of age inside about one lifespan") and `docs/OPEN-QUESTIONS.md:141` ("births only at h5–h8"). This violates acceptance (f) ("state what was measured") and the repo's current-fact rule; it does not change the verdict (at f=10 the birth hours really are h5-h6 and 100 % of deaths are age).
*Smallest fix:* in all three files replace with the measured range — "births begin h5-h9 and recur as late as h117 (`f12-s6`; h68 at `f11-s1`, h23 at `f12-s0`); age deaths begin h99-h103 and end h138-h144" — and drop the "offspring born at h5-h8 are ~136-139 hours old" inference.

### Finding P1 — `commands.txt` never builds the binary it runs
`commands.txt:19` invokes `/tmp/vs2bin`, but the only build commands recorded are `go build ./cmd/vs2` and `go vet` (`commands.txt:11-14`); no command produces `/tmp/vs2bin`. Acceptance (b) requires "the exact commands that produced them", and the whole sweep is unreproducible from this log as written.
*Smallest fix:* add the missing line, `go build -o /tmp/vs2bin ./cmd/vs2`, between §2 and §3.

### Finding P2 — "the world's whole admissible fixture grid" overstates the sweep
`docs/VERTICAL-SLICES.md:119` headline says "measured on the world's whole admissible fixture grid"; what was swept is every admissible founder count (1..16) × seeds 0..7 at `patches=2`, `H=144`, `-arms A0`, `-reps 1`. The row's own detail is accurate; only the headline is stronger than the evidence.
*Smallest fix:* "on the whole admissible founder grid × declared seeds".

### Finding P2 — the age-mortality counterfactual is an unexecuted inference in a current-fact doc
`docs/VERTICAL-SLICES.md:119` states "removing age mortality would end the same trajectory at 12 alive and pass". `spec.md:182-184` labels it a counterfactual, but it is arithmetic (10+2−0) that assumes persistence of 12 actors for h100-h144 changes neither starvation nor births; no such run exists. The primary binder evidence (81-100 % age deaths, first at h99-h103) does not need it.
*Smallest fix:* label it "an unexecuted counterfactual (12 = 10+2−0, assuming no induced starvation or birth change)".

### Finding P2 — ordering evidence for the grid is the spec's own text only
Acceptance (h): the criterion was declared in a *preceding* commit (`bca7ddc` "docs: declare the claim rungs", 37 min before `03b0467` in `.git/logs/HEAD`), which is sound commit-order evidence for the criterion. For the grid, the only evidence is `spec.md`'s pre-registration statement; a single commit cannot prove intra-commit ordering. Mitigation that makes this non-blocking: the grid is exhaustive over the founder axis at every declared seed, so no post-hoc grid could have hidden a passing cell, and the outcome is a FAIL. Report-only.

### P2 — minor numeric slips
`spec.md` §6.6 "(3) … `f≥5` shows 1–9 starvation deaths" — measured minimum at f≥5 is 0 (f=8,9,10,16), 3 at f=5. P2.

### Limits of this review
- I could not write `.scratch/43-rung2-seed/review.md` (no write tool available in this role); this report is the deliverable.
- `watchdog_diff` does not inspect committed ranges, and I have no shell/git access: **no-Go-source-change (e)**, **other rungs untouched (f)** and **`ls-remote == HEAD` (g)** are verified only indirectly — clean worktree vs HEAD, `origin/master` ref = `03b0467` (updated by a successful push, but not an `ls-remote` audit), and content inspection showing no ticket-43 text in rungs 1/3/4/5 or the M2 row, with all Go seams used (`NaturalFoundersValid`, `ArmA0`, `-replay`, `-out-txt`, `-recorded`) pre-existing. A supervisor should run `git show --stat 03b0467 -- cmd internal`, `git diff bca7ddc 03b0467 -- docs`, and `git ls-remote origin master` to close these.

## Merge verdict
**BLOCK** — not on the measurement (FAIL, 0/128, zero spend and binding analysis all hold up) but on the record: P1-1 (a stated measurement contradicted by the ticket's own `a0-grid.json`, repeated in two authoritative docs) and P1-2 (the sweep's binary has no recorded build command). Both fixes are one-line edits; no repair authority exercised, and I stopped here rather than editing.