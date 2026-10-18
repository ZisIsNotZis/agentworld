#!/usr/bin/env python3
"""Ticket 37 (M2b) analysis over recorded VS2 evaluation artefacts. Zero live calls.

Reads only evaluation JSON already produced by the frozen runner. Implements:
  fid/1  the current eight-indicator divergence vector to R (recorded rows, with a
         seven-indicator recomputation from projections used as a loader identity check);
  vrd/1  the declared viability-referenced distance (candidate C2, recommended);
  end/1  the endpoint-survival order (candidate C3, alternative).

Absence stays absent: a quantity the artefact does not carry is reported unknown and
the script exits non-zero rather than defaulting it to zero. Run with --self-test to
exercise the safe failure path before trusting any scored output.
"""

import argparse
import json
import math
import os
import sys

VRD_WEIGHTS = {"H": 1.0, "B": 1.0, "S": 1.0}
REQUIRED_CELL_KEYS = (
    "arm", "replicate", "hours", "alive_by_hour", "births_cum",
    "deaths_starve_cum", "deaths_age_cum", "capital_hist", "granary_hist",
    "wages_cum", "realized_calls",
)
DECLARED_ARMS = ("A0", "R", "P", "U", "A", "A1")


def fail(msg):
    print("ERROR: " + msg, file=sys.stderr)
    raise SystemExit(2)


def load_doc(path):
    if not os.path.exists(path):
        fail("missing artefact " + path + " (absence is not zero)")
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)


def cells_of(doc, path):
    out = {}
    for row in doc.get("cells") or []:
        for key in REQUIRED_CELL_KEYS:
            if key not in row:
                fail("%s: cell %s/%s lacks required field %s (absence is not zero)" %
                     (path, row.get("arm"), row.get("replicate"), key))
        out[(row["arm"], row["replicate"])] = row
    return out


def outcome(row):
    """The vrd/1 state vector over samples 0..H, plus diagnostics."""
    alive = row["alive_by_hour"]
    if len(alive) != len(row["hours"]):
        fail("cell %s/%s: alive_by_hour length %d != hours length %d" %
             (row["arm"], row["replicate"], len(alive), len(row["hours"])))
    return {
        "H": float(sum(alive)),
        "B": float(row["births_cum"][-1]),
        "S": float(row["deaths_starve_cum"][-1]),
        "N0": float(alive[0]),
        "alive_end": int(alive[-1]),
        "alive_143": int(alive[-2]) if len(alive) > 1 else int(alive[-1]),
        "age_deaths": int(row["deaths_age_cum"][-1]),
        "calls": int(row["realized_calls"]),
        "hours": len(alive),
    }


def anchor_of(cells, fixture):
    """Mean outcome of the A0 control replicates at the fixture (the declared floor)."""
    reps = [outcome(row) for (arm, _), row in cells.items() if arm == "A0"]
    if not reps:
        return None, "no A0 control in this artefact"
    t = {k: sum(r[k] for r in reps) / len(reps) for k in ("H", "B", "S", "N0")}
    # Validity precondition, checked rather than defaulted.
    alive_ok = any(r["alive_end"] > 0 for r in reps)
    if not (t["H"] > 0 and t["B"] > 0 and t["N0"] > 0 and alive_ok):
        return None, ("control is not viable (H_T=%.1f B_T=%.1f N0=%.1f alive_end>0=%s), "
                      "so vrd/1 is undefined for this fixture" % (t["H"], t["B"], t["N0"], alive_ok))
    return t, None


def vrd(o, t, weights=VRD_WEIGHTS):
    """Declared viability-referenced distance vrd/1. Lower is more viable; 0 = at the floor."""
    dh = max(0.0, (t["H"] - o["H"]) / t["H"])
    db = max(0.0, (t["B"] - o["B"]) / max(1.0, t["B"]))
    ds = max(0.0, (o["S"] - t["S"]) / t["N0"])
    return weights["H"] * dh + weights["B"] * db + weights["S"] * ds


def tv(a, b):
    """Total-variation distance between two histograms (frozen TotalVariation)."""
    sa, sb = sum(a), sum(b)
    if sa == 0 and sb == 0:
        return 0.0
    total = 0.0
    for i in range(min(len(a), len(b))):
        pa = a[i] / sa if sa else 0.0
        pb = b[i] / sb if sb else 0.0
        total += abs(pa - pb)
    return total / 2.0


def frozen_indicators(b, r):
    """Recompute the seven indicators derivable without the call ledger (mean per hour)."""
    n = min(len(b["hours"]), len(r["hours"]))
    per_actor_slots = 16.0  # 8 * patches, patches = 2 (cmd/vs2/config.go:316)
    seqs = {name: [] for name in ("population", "births", "deaths/starvation", "deaths/age",
                                  "capital-histogram", "granary-histogram", "exchange-volume")}
    for i in range(n):
        seqs["population"].append(abs(b["alive_by_hour"][i] - r["alive_by_hour"][i]) / per_actor_slots)
        seqs["births"].append(abs(b["births_cum"][i] - r["births_cum"][i]))
        seqs["deaths/starvation"].append(abs(b["deaths_starve_cum"][i] - r["deaths_starve_cum"][i]))
        seqs["deaths/age"].append(abs(b["deaths_age_cum"][i] - r["deaths_age_cum"][i]))
        seqs["capital-histogram"].append(tv(b["capital_hist"][i], r["capital_hist"][i]))
        seqs["granary-histogram"].append(tv(b["granary_hist"][i], r["granary_hist"][i]))
        seqs["exchange-volume"].append(abs(b["wages_cum"][i] - r["wages_cum"][i]))
    return {k: sum(v) / len(v) for k, v in seqs.items()}


def recorded_rows(doc):
    """The recorded fid/1 rows: arm -> list of {indicator: mean}. No replicate field exists."""
    rows = {}
    for row in doc.get("fidelity") or []:
        rows.setdefault(row["arm"], []).append(
            {ind["name"]: ind["mean"] for ind in row["indicators"]})
    return rows


def match_reference(cells, doc):
    """Identify which R replicate the recorded fid/1 rows were computed against.

    The report's referenceRun matches (arm, F, seed) only, so with three R replicates the
    reference identity is not declared. Match each recorded row to (arm replicate, R
    replicate) using the seven recomputable indicator means; report every match.
    """
    rows = recorded_rows(doc)
    r_reps = sorted(rep for (arm, rep) in cells if arm == "R")
    if not rows or not r_reps:
        return {}, "no recorded fid/1 rows or no R replicates"
    refs = {}
    unexplained = []
    for arm, arm_rows in rows.items():
        reps = sorted(rep for (a, rep) in cells if a == arm)
        if len(reps) != len(arm_rows):
            unexplained.append("%s rows=%d replicates=%d" % (arm, len(arm_rows), len(reps)))
            continue
        for rr in r_reps:
            assign = {}
            used = set()
            ok = True
            for rep in reps:
                values = frozen_indicators(cells[(arm, rep)], cells[("R", rr)])
                hits = [i for i, row in enumerate(arm_rows)
                        if all(abs(values[k] - row[k]) <= 1e-9 for k in values)]
                if len(hits) != 1:
                    ok = False
                    break
                assign[rep] = hits[0]
                used.add(hits[0])
            if ok and len(used) == len(arm_rows):
                for rep, row_index in assign.items():
                    refs[(arm, rep)] = (rr, row_index)
        if not any(a == arm for (a, _) in refs):
            unexplained.append(arm + ":no-reference-match")
    if unexplained:
        return refs, "unexplained recorded rows: " + ", ".join(unexplained)
    return refs, None


def pair_check(cells, t, pair, threshold=0.10):
    """All 3x3 replicate pairings of (worse, better) under vrd/1."""
    worse, better = pair
    margins = []
    ok = True
    for wa in sorted(rep for (a, rep) in cells if a == worse):
        for ba in sorted(rep for (a, rep) in cells if a == better):
            dw = vrd(outcome(cells[(worse, wa)]), t)
            db = vrd(outcome(cells[(better, ba)]), t)
            margin = dw - db
            margins.append(((wa, ba), dw, db, margin))
            if margin < threshold:
                ok = False
    return ok, margins


def score_matrix(path, label):
    doc = load_doc(path)
    cells = cells_of(doc, path)
    contract = doc.get("claim_contract", "?")
    hours = doc.get("horizon_hours", "?")
    arms_present = sorted({a for (a, _) in cells})
    t, why = anchor_of(cells, label)
    print("\n== %s ==" % label)
    print("artefact: %s" % path)
    print("status=%s contract=%s horizon=%s replicates=%s arms=%s" %
          (doc.get("status"), contract, hours, doc.get("replicates"), ",".join(arms_present)))
    if t is None:
        print("vrd/1: UNDEFINED — %s" % why)
    else:
        print("viability floor (A0 control, mean over replicates): H_T=%.1f B_T=%.1f S_T=%.1f N0=%.1f"
              % (t["H"], t["B"], t["S"], t["N0"]))

    rows = recorded_rows(doc)
    refs, refwhy = match_reference(cells, doc)
    if refwhy:
        print("fid/1 reference identity: %s" % refwhy)
    else:
        users = sorted({("R%d" % rr) for (rr, _) in refs.values()})
        print("fid/1 reference identity: recorded rows match reference %s (recomputed seven "
              "indicators, tol 1e-9; mapping %s)" %
              (",".join(users), ", ".join("%s/r%d->R%d(row %d)" % (a, r, rr, idx)
                                          for (a, r), (rr, idx) in sorted(refs.items()))))

    # The old measure for every arm, including the two the instrument never scores: the
    # same seven ledger-free indicators the loader-identity check recomputes, against the
    # reference the recorded rows actually used. The eighth indicator (decision-error) is
    # not in the evaluation artefact for any arm, so this is a seven-of-eight measurement.
    ref_reps = sorted({rr for (rr, _) in refs.values()})
    fid7_ref = "R%d" % ref_reps[0] if len(ref_reps) == 1 else None

    table = []
    for (arm, rep) in sorted(cells, key=lambda k: (DECLARED_ARMS.index(k[0]) if k[0] in DECLARED_ARMS else 99, k[1])):
        o = outcome(cells[(arm, rep)])
        d = vrd(o, t) if t is not None else None
        arm_rows = rows.get(arm) or []
        match = refs.get((arm, rep))
        old_mean = None
        if match is not None and 0 <= match[1] < len(arm_rows):
            old_mean = sum(arm_rows[match[1]].values()) / len(arm_rows[match[1]])
        fid7 = None
        if fid7_ref is not None:
            fid7 = sum(frozen_indicators(cells[(arm, rep)], cells[("R", ref_reps[0])]).values()) / 7.0
        table.append({"arm": arm, "replicate": rep, "calls": o["calls"], "alive_143": o["alive_143"],
                      "alive_end": o["alive_end"], "births": int(o["B"]), "starvation": int(o["S"]),
                      "actor_hours": int(o["H"]), "vrd": d, "fid_mean8": old_mean,
                      "fid7": fid7, "fid7_reference": fid7_ref})
        print("  %-3s r%d calls=%4d alive@143=%d alive@144=%d births=%d starve=%d H=%4d  vrd/1=%s  fid_mean8=%s  fid7(%s)=%s" %
              (arm, rep, o["calls"], o["alive_143"], o["alive_end"], int(o["B"]), int(o["S"]), int(o["H"]),
               "%.4f" % d if d is not None else "unknown",
               "%.4f" % old_mean if old_mean is not None else "not-scored",
               fid7_ref or "-", "%.4f" % fid7 if fid7 is not None else "unknown"))

    if label == "primary" and rows:
        print("fid/1 per-indicator means and their equal-weight contribution (mean/8) to the arm scalar:")
        names = ("population", "births", "deaths/starvation", "deaths/age",
                 "capital-histogram", "granary-histogram", "exchange-volume",
                 "decision-error-rate(level-1+level-2)")
        for arm in sorted(rows):
            means = {n: sum(r.get(n, float("nan")) for r in rows[arm]) / len(rows[arm]) for n in names}
            total = sum(means.values()) / 8.0
            share = ", ".join("%s=%.4f(%.0f%%)" % (n.split("(")[0], means[n] / 8.0, 100.0 * (means[n] / 8.0) / total) for n in names)
            print("  %-3s mean8=%.4f  %s" % (arm, total, share))

    if label == "primary" and rows:
        r_reps = sorted(rep for (a, rep) in cells if a == "R")
        print("fid/1 reference sensitivity: seven-indicator mean of each arm replicate vs each R replicate "
              "(the recorded rows used R3; the instrument declares no replicate rule):")
        for arm in sorted(rows):
            for rep in sorted(r for (a, r) in cells if a == arm):
                vals = {rr: sum(frozen_indicators(cells[(arm, rep)], cells[("R", rr)]).values()) / 7.0
                        for rr in r_reps}
                print("  %-3s r%d  %s" % (arm, rep, "  ".join("vs R%d=%.4f" % (rr, vals[rr]) for rr in r_reps)))

    # Old-metric rank (equal-weight mean; the instrument itself declares no weights).
    old_means = {}
    for r in table:
        if r["fid_mean8"] is not None:
            old_means.setdefault(r["arm"], []).append(r["fid_mean8"])
    old_rank = sorted(((a, sum(v) / len(v)) for a, v in old_means.items()), key=lambda kv: kv[1])
    if old_rank:
        print("fid/1 equal-weight mean8 ranking (arm mean; lower = closer to R): " +
              " < ".join("%s(%.4f)" % (a, v) for a, v in old_rank))
    # Old metric over every recorded arm, including A0 and R (AC-37e), on the consistent
    # seven-indicator basis. R r3 is the reference itself and sits at 0 by construction.
    seven = {}
    for r in table:
        if r["fid7"] is not None:
            seven.setdefault(r["arm"], []).append(r["fid7"])
    if seven:
        seven_rank = sorted(((a, sum(v) / len(v)) for a, v in seven.items()), key=lambda kv: kv[1])
        print("fid/1 seven-indicator mean vs %s, every arm (no decision-error term, so not "
              "comparable to mean8): %s" %
              (fid7_ref, " < ".join("%s(%.4f)" % (a, v) for a, v in seven_rank)))
        print("  (%s is the reference itself (replicate %d) and scores 0 by construction; among "
              "the other arms the extinct A1 remains closest to R)" % (fid7_ref, ref_reps[0]))
    # New-metric arm means.
    if t is not None:
        by_arm = {}
        for r in table:
            by_arm.setdefault(r["arm"], []).append(r["vrd"])
        means = {a: sum(v) / len(v) for a, v in by_arm.items()}
        print("vrd/1 arm means (lower = more viable): " +
              " < ".join("%s(%.4f)" % (a, v) for a, v in sorted(means.items(), key=lambda kv: kv[1])))
        endm = {}
        for a in {r["arm"] for r in table}:
            rs = [r for r in table if r["arm"] == a]
            endm[a] = (sum(r["alive_end"] for r in rs) / len(rs),
                       sum(r["births"] for r in rs) / len(rs),
                       -sum(r["starvation"] for r in rs) / len(rs))
        print("end/1 (mean endpoint alive, then mean births, then -mean starvation; best to worst): " +
              " > ".join("%s" % a for a, _ in sorted(endm.items(), key=lambda kv: kv[1], reverse=True)))
    return {"label": label, "path": path, "contract": contract, "horizon": hours,
            "arms": arms_present, "anchor": t, "anchor_error": why,
            "reference_matches": {"%s/r%d" % k: "R%d(row %d)" % v for k, v in refs.items()},
            "table": table}


def discrimination(results, pairs, threshold=0.10, expect_fail=False):
    print("\n== discrimination (AC-37c/d) ==")
    primary = results[0]
    doc = load_doc(primary["path"])
    cells = cells_of(doc, primary["path"])
    t, why = anchor_of(cells, "primary")
    if t is None:
        fail("discrimination requires a defined anchor: " + why)
    verdicts = []
    for (worse, better) in pairs:
        # The negative control swaps the declared order and then requires the check to FAIL.
        # Flipping the verdict boolean instead would print PASS unconditionally.
        checked = (better, worse) if expect_fail else (worse, better)
        ok, margins = pair_check(cells, t, checked, threshold)
        detected = (not ok) if expect_fail else ok
        verdicts.append({"pair": "%s_vs_%s" % checked, "ok": detected,
                         "min_margin": min(m for _, _, _, m in margins)})
        print("%s: %s (%s vs %s) min margin %.4f over %d replicate pairings" %
              ("NEGATIVE-CONTROL" if expect_fail else "PAIR",
               ("INVERSION-DETECTED" if detected else "NOT-DETECTED") if expect_fail
               else ("PASS" if ok else "FAIL"),
               checked[0], checked[1], min(m for _, _, _, m in margins), len(margins)))
        for (wa, ba), dw, db, margin in margins:
            print("   %s r%d=%.4f  %s r%d=%.4f  margin=%+.4f" %
                  (checked[0], wa, dw, checked[1], ba, db, margin))
    return verdicts


def sensitivity(results, pairs, threshold=0.10):
    print("\n== sensitivity: each weight doubled in turn ==")
    primary = results[0]
    cells = cells_of(load_doc(primary["path"]), primary["path"])
    t, why = anchor_of(cells, "primary")
    if t is None:
        fail("sensitivity requires a defined anchor: " + why)
    out = []
    for name in ("H", "B", "S"):
        w = dict(VRD_WEIGHTS)
        w[name] *= 2.0
        ok_all = True
        detail = []
        for pair in pairs:
            worse, better = pair
            ok = True
            worst = None
            for wa in sorted(rep for (a, rep) in cells if a == worse):
                for ba in sorted(rep for (a, rep) in cells if a == better):
                    m = vrd(outcome(cells[(worse, wa)]), t, w) - vrd(outcome(cells[(better, ba)]), t, w)
                    worst = m if worst is None else min(worst, m)
                    if m < threshold:
                        ok = False
            ok_all = ok_all and ok
            detail.append("%s_vs_%s=%s(min %+.4f)" % (worse, better, "PASS" if ok else "FAIL", worst))
        out.append({"variant": name + "x2", "ok": ok_all})
        print("w_%s x2: %s  %s" % (name, "PASS" if ok_all else "FAIL", "  ".join(detail)))
    return out


def self_test(results):
    """The declared safe way to see failure."""
    print("\n== self-test (negative controls) ==")
    primary = results[0]
    cells = cells_of(load_doc(primary["path"]), primary["path"])
    t, why = anchor_of(cells, "primary")
    doc = load_doc(primary["path"])
    refs, refwhy = match_reference(cells, doc)
    if refwhy or len(refs) != len([k for k in cells if k[0] in recorded_rows(doc)]):
        print("loader identity: FAIL (%s, %d/%d rows matched)" %
              (refwhy or "unmatched", len(refs), len([k for k in cells if k[0] in recorded_rows(doc)])))
        return False
    print("loader identity: PASS (%d recorded rows recomputed from projections, tol 1e-9)" % len(refs))
    # Synthetic perfect arm: the control itself must score 0.
    a0 = dict(cells[("A0", 1)])
    perfect = outcome(a0)
    dperfect = vrd(perfect, t)
    print("synthetic perfect arm (= control): vrd/1=%.4f (require 0)" % dperfect)
    if abs(dperfect) > 1e-12:
        return False
    # Synthetic extinct arm: zero alive from hour 48, no births.
    extinct = {"alive_by_hour": [10] * 48 + [0] * (len(a0["alive_by_hour"]) - 48),
               "births_cum": [0] * len(a0["births_cum"]),
               "deaths_starve_cum": [0] * len(a0["deaths_starve_cum"]),
               "deaths_age_cum": a0["deaths_age_cum"], "hours": a0["hours"],
               "realized_calls": 0}
    dext = vrd(outcome(extinct), t)
    print("synthetic extinct arm (0 alive after h48): vrd/1=%.4f (require >= 1.0)" % dext)
    if dext < 1.0:
        return False
    # Strict over-floor survivor: all ten founders alive at every one of the 145 samples,
    # so H = 1450 > H_T = 1444 and the (H_T - H_x) term is exercised with a negative
    # numerator. The declared claim is that this clips to 0; no recorded arm reaches it.
    over = {"alive_by_hour": [10] * len(a0["alive_by_hour"]),
            "births_cum": a0["births_cum"],
            "deaths_starve_cum": a0["deaths_starve_cum"],
            "deaths_age_cum": a0["deaths_age_cum"], "hours": a0["hours"],
            "realized_calls": 0}
    dover = vrd(outcome(over), t)
    print("synthetic over-floor survivor (10 alive at every sample, H=1450 > H_T=1444): "
          "vrd/1=%.4f (require 0)" % dover)
    if abs(dover) > 1e-12:
        return False
    # The declared known pathology, measured rather than asserted in prose: with B_T = 2 the
    # birth term alone equals the whole survival axis, so a sterile but fully surviving world
    # scores 1.000, above an extinct world with one birth (R r3, 0.5963). Pinned so a silent
    # change to the weights or normalisation fails here and forces a re-declaration.
    sterile = {"alive_by_hour": [10] * len(a0["alive_by_hour"]),
               "births_cum": [0] * len(a0["births_cum"]),
               "deaths_starve_cum": [0] * len(a0["deaths_starve_cum"]),
               "deaths_age_cum": a0["deaths_age_cum"], "hours": a0["hours"],
               "realized_calls": 0}
    dsterile = vrd(outcome(sterile), t)
    ref_reps = sorted({rr for (rr, _) in refs.values()})
    r_rep = ref_reps[0] if len(ref_reps) == 1 else 3
    d_r3 = vrd(outcome(cells[("R", r_rep)]), t)
    print("synthetic sterile survivor (10 alive at every sample, 0 births): vrd/1=%.4f versus "
          "extinct R r%d %.4f — DECLARED KNOWN PATHOLOGY (birth term dominates; pinned, "
          "repair or normalise before any arm comparison)" % (dsterile, r_rep, d_r3))
    if abs(dsterile - 1.0) > 1e-9 or not dsterile > d_r3:
        return False
    # A mutant that rewards extinction must be caught by the declared pair check.
    ok, _ = pair_check(cells, t, ("A1", "A"))
    ok_inverted, _ = pair_check(cells, t, ("A", "A1"))
    print("declared pair D1 A1(worse)>A(better): %s ; inverted pair detected as FAIL: %s" %
          ("PASS" if ok else "FAIL", "YES" if not ok_inverted else "NO"))
    return ok and not ok_inverted


def svg_figure(results, path):
    primary = results[0]
    rows = primary["table"]
    arms = sorted({r["arm"] for r in rows},
                  key=lambda a: DECLARED_ARMS.index(a) if a in DECLARED_ARMS else 99)
    extinct = {}
    for r in rows:
        extinct.setdefault(r["arm"], []).append(r["alive_end"] == 0)

    W, H = 1180, 590
    pad_l, pad_r, top = 70, 30, 135
    panel_w = (W - pad_l - pad_r - 60) / 2
    out = ['<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">' % (W, H, W, H),
           '<rect width="100%" height="100%" fill="#ffffff"/>',
           '<text x="%d" y="30" font-family="sans-serif" font-size="18" font-weight="bold">M2b: what each measure rewards on the recorded natural-claim/v2 matrix</text>' % pad_l,
           '<text x="%d" y="52" font-family="sans-serif" font-size="13" fill="#444">'
           'f10-s0 · H=144 · 3 replicates · hatched = extinct at h144 · numbers are replicate means</text>' % pad_l]

    def panel(x0, title, subtitle, values, note_unknown):
        out.append('<text x="%.1f" y="%d" font-family="sans-serif" font-size="15" font-weight="bold">%s</text>' % (x0, 92, title))
        out.append('<text x="%.1f" y="%d" font-family="sans-serif" font-size="12" fill="#555">%s</text>' % (x0, 112, subtitle))
        base_y = H - 100
        maxv = max([v for v in values.values() if v is not None] + [0.1])
        barw = panel_w / (len(arms) + 1)
        for i, a in enumerate(arms):
            v = values.get(a)
            cx = x0 + barw * (i + 0.5)
            alive = sorted({r["alive_end"] for r in rows if r["arm"] == a})
            if v is None:
                out.append('<text x="%.1f" y="%.1f" font-family="sans-serif" font-size="12" text-anchor="middle" font-weight="bold">%s</text>' % (cx, base_y + 18, a))
                out.append('<text x="%.1f" y="%.1f" font-family="sans-serif" font-size="10" text-anchor="middle" fill="#999">not scored</text>' % (cx, base_y + 32))
                continue
            h = (v / maxv) * (base_y - top - 10)
            fill = "#d95f5f" if all(extinct.get(a, [True])) else "#4a9a6f"
            out.append('<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" stroke="#333" stroke-width="1"/>' %
                       (cx - barw * 0.34, base_y - h, barw * 0.68, h, fill))
            out.append('<text x="%.1f" y="%.1f" font-family="sans-serif" font-size="11" text-anchor="middle">%s</text>' %
                       (cx, base_y - h - 6, "%.3f" % v))
            out.append('<text x="%.1f" y="%.1f" font-family="sans-serif" font-size="12" text-anchor="middle" font-weight="bold">%s</text>' %
                       (cx, base_y + 18, a))
            out.append('<text x="%.1f" y="%.1f" font-family="sans-serif" font-size="10" text-anchor="middle" fill="#555">alive@H %s</text>' %
                       (cx, base_y + 32, "/".join(str(x) for x in alive)))
        out.append('<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#333"/>' % (x0, base_y, x0 + panel_w, base_y))

    # Old measure: bar = equal-weight mean of the 8 recorded indicator means.
    old = {}
    for a in arms:
        vals = [r["fid_mean8"] for r in rows if r["arm"] == a and r["fid_mean8"] is not None]
        if vals:
            old[a] = sum(vals) / len(vals)
    panel(pad_l, "Current: mean distance to R  (lower = 'closer to R')",
          "fid/1, equal-weight mean of 8 indicators — A0 and R are not scored by the instrument",
          old, note_unknown=False)
    newv = {}
    for a in arms:
        vals = [r["vrd"] for r in rows if r["arm"] == a and r["vrd"] is not None]
        if vals:
            newv[a] = sum(vals) / len(vals)
    panel(pad_l + panel_w + 60, "Declared: viability-referenced distance vrd/1  (lower = more viable)",
          "distance below the no-AI control's outcome; 0 = at or above the floor",
          newv, note_unknown=True)

    out.append('<rect x="%d" y="%d" width="18" height="12" fill="#4a9a6f" stroke="#333"/>' % (pad_l, H - 46))
    out.append('<text x="%d" y="%d" font-family="sans-serif" font-size="12">survived to h144</text>' % (pad_l + 24, H - 36))
    out.append('<rect x="%d" y="%d" width="18" height="12" fill="#d95f5f" stroke="#333"/>' % (pad_l + 200, H - 46))
    out.append('<text x="%d" y="%d" font-family="sans-serif" font-size="12">extinct at h144 (all replicates)</text>' % (pad_l + 224, H - 36))
    out.append('<text x="%d" y="%d" font-family="sans-serif" font-size="11" fill="#777">Source: .tmp/35-v2-evaluation.json (sha256 8efd8195…). Model-conditional evidence; no claim that any arm is better.</text>' % (pad_l, H - 14))
    out.append('</svg>')
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(out) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--primary", required=True)
    ap.add_argument("--secondary")
    ap.add_argument("--corroboration")
    ap.add_argument("--json-out")
    ap.add_argument("--svg")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--expect-fail", action="store_true")
    args = ap.parse_args()

    results = [score_matrix(args.primary, "primary")]
    if args.secondary:
        results.append(score_matrix(args.secondary, "secondary"))
    if args.corroboration:
        doc = load_doc(args.corroboration)
        cells = cells_of(doc, args.corroboration)
        print("\n== corroboration (raw outcomes only; no control in the artefact) ==")
        for (arm, rep) in sorted(cells):
            o = outcome(cells[(arm, rep)])
            print("  %s r%d contract=%s calls=%d alive@H=%d births=%d starve=%d H=%d  vrd/1=unknown "
                  "(no A0 control → anchor undefined)" %
                  (arm, rep, doc.get("claim_contract"), o["calls"], o["alive_end"],
                   int(o["B"]), int(o["S"]), int(o["H"])))

    if args.self_test and not self_test(results):
        fail("self-test failed: the scored output must not be trusted")
    pairs = [("A1", "A"), ("R", "A0"), ("R", "P")]
    verdicts = discrimination(results, pairs, expect_fail=args.expect_fail)
    if args.expect_fail:
        ok = all(v["ok"] for v in verdicts)
        print("NEGATIVE CONTROL %s: every swapped pair was detected as a failure" %
              ("PASS" if ok else "FAIL"))
        if not ok:
            fail("negative control failed: a swapped pair was not detected as a failure")
        return
    sens = sensitivity(results, pairs)
    payload = {"results": results, "discrimination": verdicts, "sensitivity": sens}
    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, indent=2, sort_keys=True)
        print("\nwrote %s" % args.json_out)
    if args.svg:
        svg_figure(results, args.svg)
        print("wrote %s" % args.svg)
    if not all(v["ok"] for v in verdicts) or not all(v["ok"] for v in sens):
        fail("AC-37(c)/(d) FAIL — see the pair and sensitivity tables above")
    print("\nAC-37(c)/(d): PASS on every declared pair and every sensitivity variant")


if __name__ == "__main__":
    main()
