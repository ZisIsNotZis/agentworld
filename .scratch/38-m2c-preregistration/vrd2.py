#!/usr/bin/env python3
"""Ticket 38 (M2c part 1): the repaired viability metric, and the pre-declared M2c order.

Zero live calls. This script is the evaluator the M2c criterion is declared against
(.scratch/38-m2c-preregistration/spec.md section 7). It implements:

  vrd/1   the withdrawn objective (ticket 37): the equal-weight sum of the actor-hour,
          fertility and starvation shortfalls below the no-AI control's outcome;
  P1      vrd/2, the chosen repair: the same three components plus a persistence
          predicate, compared as a banded lexicographic order (survival, then deaths
          avoided, then reproduction);
  P2      vrd/2f, the rejected alternative: fertility floored at one birth and
          additive precedence weights 10/1/1;
  P3      vrd/2s, the rejected alternative: survival and starvation only, births
          reported separately.

Modes:
  --scores PATH   ticket 37's committed per-cell vectors (the recorded matrix);
  --live PATH     an evaluation JSON from a live run; applies the pre-declared
                  band rule and prints a verdict per declared pair;
  --self-test     the declared ordering tests T1-T6 on worlds W0-W3, the band-rule
                  fixtures, and the loader identity check. Must pass before any score
                  is read (spec section 7.6, gate 1);
  --expect-fail   the negative control: every declared comparison is inverted and must
                  be detected as a FAIL.

Absence stays absent: a missing required field is a hard failure, never a zero.
"""

import argparse
import itertools
import json
import os
import sys

# Declared worlds (spec section 6) as state vectors: H, B, S, alive at the terminal sample.
WORLDS = {
    "W0": {"label": "recorded A0 control (the floor)", "H": 1444.0, "B": 2.0, "S": 0.0, "alive_end": 1},
    "W1": {"label": "sterile but fully surviving (synthetic)", "H": 1450.0, "B": 0.0, "S": 0.0, "alive_end": 10},
    "W2": {"label": "recorded R replicate 3: extinct at H, one birth", "H": 1305.0, "B": 1.0, "S": 0.0, "alive_end": 0},
    "W3": {"label": "survives, one birth short, one starvation death (synthetic)", "H": 1444.0, "B": 1.0, "S": 1.0, "alive_end": 1},
}

# The anchor as recorded (ticket 37 scores.json A0 rows are identical across replicates).
ANCHOR = {"H": 1444.0, "B": 2.0, "S": 0.0, "N0": 10.0}

# Declared priority test: the four worlds must come out in this order.
DECLARED_WORLD_ORDER = ["W0", "W1", "W3", "W2"]

# P2's declared precedence weights (spec section 4).
P2_WEIGHTS = {"H": 10.0, "S": 1.0, "B": 1.0}

REQUIRED_ROW_KEYS = ("arm", "replicate", "actor_hours", "births", "starvation", "alive_end")


def fail(msg):
    print("FAIL: " + msg, file=sys.stderr)
    raise SystemExit(2)


def load_json(path):
    if not os.path.exists(path):
        fail("missing artefact " + path + " (absence is not zero)")
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)


def components(w, t):
    """The four tier components of a world (H,B,S) against the anchor t."""
    if t["H"] <= 0 or t["B"] <= 0 or t["N0"] <= 0:
        fail("anchor is not viable (H_T=%.3f B_T=%.3f N0=%.3f): the metric is undefined"
             % (t["H"], t["B"], t["N0"]))
    return {
        "e": 1.0 if w["alive_end"] == 0 else 0.0,
        "H": max(0.0, (t["H"] - w["H"]) / t["H"]),
        "S": max(0.0, (w["S"] - t["S"]) / t["N0"]),
        "B": max(0.0, (t["B"] - w["B"]) / t["B"]),
    }


def vrd1(w, t):
    """The withdrawn objective, exactly as ticket 37 computed it."""
    c = components(w, t)
    return c["H"] + c["B"] + c["S"]


def key(w, t):
    """vrd/2: the lexicographic tier key (persist, actor-hours, starvation, births)."""
    c = components(w, t)
    return (c["e"], c["H"], c["S"], c["B"])


def vrd2f(w, t):
    """P2: additive, fertility floored at one birth, precedence weights 10/1/1."""
    c = components(w, t)
    floor = max(0.0, (1.0 - w["B"]) / 1.0)
    return P2_WEIGHTS["H"] * c["H"] + P2_WEIGHTS["S"] * c["S"] + P2_WEIGHTS["B"] * floor


def vrd2s(w, t):
    """P3: survival and starvation only; births are a separate reported KPI."""
    c = components(w, t)
    return c["H"] + c["S"]


def fmt_key(k):
    return "(e=%d, dH=%.4f, dS=%.4f, dB=%.4f)" % k


def order_by(score, worlds, t):
    return sorted(worlds, key=lambda name: score(worlds[name], t))


# ---------------------------------------------------------------------------
# Declared ordering tests T1-T6 (spec section 6).
# ---------------------------------------------------------------------------

def declared_world_tests(t):
    """Return (passed, lines). T1-T6 are the acceptance of the metric choice."""
    ws = WORLDS
    lines = []
    ok = True

    d1 = {n: vrd1(w, t) for n, w in ws.items()}
    lines.append("T1 vrd/1 reproduces the declared pathology on W1/W2:")
    lines.append("   W1=%.4f  W2=%.4f  W3=%.4f  W0=%.4f" % (d1["W1"], d1["W2"], d1["W3"], d1["W0"]))
    t1 = abs(d1["W1"] - 1.0) <= 1e-9 and abs(d1["W2"] - 0.5962603878116344) <= 1e-9 and d1["W2"] < d1["W1"]
    lines.append("   %s: W1 == 1.0000 and W2 == 0.5963 and W2 < W1" % ("PASS" if t1 else "FAIL"))
    ok = ok and t1

    keys = {n: key(w, t) for n, w in ws.items()}
    for n in ("W0", "W1", "W2", "W3"):
        lines.append("   vrd/2 key %s = %s" % (n, fmt_key(keys[n])))
    t2 = keys["W1"] < keys["W2"]
    lines.append("T2 %s: W1 (sterile survivor) outranks W2 (extinct, one birth)" % ("PASS" if t2 else "FAIL"))
    ok = ok and t2
    t3 = keys["W3"] < keys["W2"]
    lines.append("T3 %s: W3 (survivor, one starvation death) outranks W2" % ("PASS" if t3 else "FAIL"))
    ok = ok and t3

    got = sorted(ws, key=lambda n: keys[n])
    t4 = got == DECLARED_WORLD_ORDER
    lines.append("T4 %s: the full order is %s (declared %s)"
                 % ("PASS" if t4 else "FAIL", " < ".join(got), " < ".join(DECLARED_WORLD_ORDER)))
    ok = ok and t4

    t5 = keys["W0"] == (0.0, 0.0, 0.0, 0.0)
    lines.append("T5 %s: the recorded A0 control is exactly at the floor" % ("PASS" if t5 else "FAIL"))
    ok = ok and t5

    synthetic_copy = {"H": 1444.0, "B": 2.0, "S": 0.0, "alive_end": 1}
    t6 = key(synthetic_copy, t) == keys["W0"]
    lines.append("T6 %s: a synthetic copy of A0 ties the recorded A0 exactly" % ("PASS" if t6 else "FAIL"))
    ok = ok and t6

    lines.append("")
    lines.append("rejected alternatives on the same four worlds (spec section 4):")
    lines.append("   P2 vrd/2f: " + " < ".join(order_by(vrd2f, ws, t)))
    lines.append("   order P2: " + "  ".join("%s=%.4f" % (n, vrd2f(ws[n], t)) for n in ("W0", "W1", "W2", "W3")))
    p2_ok = vrd2f(ws["W1"], t) < vrd2f(ws["W2"], t)
    lines.append("   P2 T2 %s (W1 %.4f vs W2 %.4f); required weight ratio w_H/w_B > 1/%.5f = %.2f"
                 % ("PASS" if p2_ok else "FAIL", vrd2f(ws["W1"], t), vrd2f(ws["W2"], t),
                    components(ws["W2"], t)["H"], 1.0 / components(ws["W2"], t)["H"]))
    lines.append("   P3 vrd/2s: " + " < ".join(order_by(vrd2s, ws, t)))
    lines.append("   order P3: " + "  ".join("%s=%.4f" % (n, vrd2s(ws[n], t)) for n in ("W0", "W1", "W2", "W3")))
    p3_t2 = vrd2s(ws["W1"], t) < vrd2s(ws["W2"], t)
    p3_t3 = vrd2s(ws["W3"], t) < vrd2s(ws["W2"], t)
    lines.append("   P3 T2 %s (W1 %.4f vs W2 %.4f); P3 T3 %s (W3 %.4f vs W2 %.4f)"
                 % ("PASS" if p3_t2 else "FAIL", vrd2s(ws["W1"], t), vrd2s(ws["W2"], t),
                    "PASS" if p3_t3 else "FAIL", vrd2s(ws["W3"], t), vrd2s(ws["W2"], t)))
    return ok, lines


# ---------------------------------------------------------------------------
# The recorded matrix (ticket 37's committed per-cell vectors).
# ---------------------------------------------------------------------------

def recorded_cells(path):
    doc = load_json(path)
    results = doc.get("results") or []
    if not results:
        fail("%s carries no results block" % path)
    primary = results[0]
    rows = primary.get("table") or []
    if not rows:
        fail("%s carries no recorded cell table" % path)
    cells = {}
    for row in rows:
        for k in REQUIRED_ROW_KEYS:
            if k not in row:
                fail("recorded cell %s/r%s lacks %s (absence is not zero)" % (row.get("arm"), row.get("replicate"), k))
        cells[(row["arm"], row["replicate"])] = {
            "H": float(row["actor_hours"]), "B": float(row["births"]),
            "S": float(row["starvation"]), "alive_end": int(row["alive_end"]),
            "alive_143": int(row["alive_143"]), "calls": int(row["calls"]),
            "vrd1": float(row["vrd"]),
        }
    anchor = primary.get("anchor")
    if not anchor:
        fail("%s carries no anchor block" % path)
    return cells, {"H": float(anchor["H"]), "B": float(anchor["B"]),
                   "S": float(anchor["S"]), "N0": float(anchor["N0"])}, primary


def arm_aggregate(cells, arm, anchor=None):
    t = anchor or ANCHOR
    reps = sorted(r for (a, r) in cells if a == arm)
    if not reps:
        return None
    comp = [components(cells[(arm, r)], t) for r in reps]
    return {
        "reps": reps,
        "extinct_all": all(c["e"] == 1.0 for c in comp),
        "persist_all": all(c["e"] == 0.0 for c in comp),
        "extinct143_all": all(cells[(arm, r)]["alive_143"] == 0 for r in reps),
        "persist143_all": all(cells[(arm, r)]["alive_143"] > 0 for r in reps),
        "H": sum(c["H"] for c in comp) / len(comp),
        "S": sum(c["S"] for c in comp) / len(comp),
        "B": sum(c["B"] for c in comp) / len(comp),
        "band_H": band(comp, "H"), "band_S": band(comp, "S"), "band_B": band(comp, "B"),
    }


def band(comp, name):
    vals = [c[name] for c in comp]
    mu = sum(vals) / len(vals)
    return max(max(vals) - mu, mu - min(vals))


def arm_key(a):
    return (0 if a["persist_all"] else (1 if a["extinct_all"] else 0.5),
            round(a["H"], 12), round(a["S"], 12), round(a["B"], 12))


def recorded_report(cells, anchor, primary):
    lines = []
    lines.append("recorded matrix: contract=%s horizon=%s cells=%d arms=%s"
                 % (primary.get("contract"), primary.get("horizon"), len(cells),
                    ",".join(primary.get("arms") or [])))
    lines.append("anchor: H_T=%.1f B_T=%.1f S_T=%.1f N0=%.1f" % (anchor["H"], anchor["B"], anchor["S"], anchor["N0"]))

    # Loader identity: vrd/1 recomputed from the recorded vectors must equal the recorded values.
    worst = 0.0
    for (arm, rep), w in sorted(cells.items()):
        worst = max(worst, abs(vrd1(w, anchor) - w["vrd1"]))
    lines.append("loader identity: |vrd/1 recomputed - recorded| max = %.3e (tolerance 1e-12) — %s"
                 % (worst, "PASS" if worst <= 1e-12 else "FAIL"))
    if worst > 1e-12:
        fail("vrd/1 reproduction failed: the recorded vectors are not the declared state vector")

    arms = []
    for arm in ("A0", "R", "P", "U", "A", "A1"):
        if any(a == arm for (a, _) in cells):
            arms.append(arm)
    lines.append("")
    lines.append("per-replicate keys (vrd/2) and the recorded vrd/1 value:")
    for (arm, rep) in sorted(cells, key=lambda k: (arms.index(k[0]), k[1])):
        w = cells[(arm, rep)]
        lines.append("   %-3s r%d calls=%4d H=%4d B=%d S=%d alive@H=%d  %s  vrd/1=%.4f"
                     % (arm, rep, w["calls"], w["H"], w["B"], w["S"], w["alive_end"],
                        fmt_key(key(w, anchor)), w["vrd1"]))

    lines.append("")
    lines.append("arm aggregates and order (tier 1 = persisted in every replicate):")
    aggs = {a: arm_aggregate(cells, a, anchor) for a in arms}
    for a in sorted(arms, key=lambda x: arm_key(aggs[x])):
        g = aggs[a]
        lines.append("   %-3s persisted=%s extinct=%s muH=%.4f(+/-%.4f) muS=%.4f(+/-%.4f) muB=%.4f(+/-%.4f)"
                     % (a, g["persist_all"], g["extinct_all"], g["H"], g["band_H"], g["S"], g["band_S"],
                        g["B"], g["band_B"]))
    order2 = sorted(arms, key=lambda x: arm_key(aggs[x]))
    lines.append("vrd/2 order (more viable first): " + " < ".join(order2))

    mu1 = {}
    for a in arms:
        vals = [cells[(a, r)]["vrd1"] for r in aggs[a]["reps"]]
        mu1[a] = sum(vals) / len(vals)
    order1 = sorted(arms, key=lambda x: mu1[x])
    lines.append("vrd/1 order (lower = more viable):  " + " < ".join("%s(%.4f)" % (a, mu1[a]) for a in order1))

    for label, fn in (("P2 vrd/2f", vrd2f), ("P3 vrd/2s", vrd2s)):
        mu = {}
        for a in arms:
            mu[a] = sum(fn(cells[(a, r)], anchor) for r in aggs[a]["reps"]) / len(aggs[a]["reps"])
        order = sorted(arms, key=lambda x: mu[x])
        lines.append("%s order: " % label + " < ".join("%s(%.4f)" % (a, mu[a]) for a in order))
        moved = [(x, y) for i, x in enumerate(arms) for y in arms[i + 1:]
                 if (order2.index(x) < order2.index(y)) != (order.index(x) < order.index(y))]
        lines.append("   pairs whose relative order differs from vrd/2: %s"
                     % (", ".join("%s/%s" % p for p in moved) if moved else "none"))

    flips = []
    for i, x in enumerate(arms):
        for y in arms[i + 1:]:
            if (order1.index(x) < order1.index(y)) != (order2.index(x) < order2.index(y)):
                flips.append((x, y, mu1[x], mu1[y]))
    lines.append("pairs whose relative order changed between vrd/1 and vrd/2: %d" % len(flips))
    for x, y, mx, my in flips:
        lines.append("   %s vs %s  (vrd/1 %s %.4f vs %.4f)" % (x, y, "<" if mx < my else ">", mx, my))

    lines.append("")
    lines.append("ticket 37's declared pairs re-checked under vrd/2, every 3x3 replicate pairing:")
    pairs = [("D1", "A1", "A"), ("D2", "R", "A0"), ("D3", "R", "P")]
    for name, worse, better in pairs:
        rows = []
        passed = True
        for wr in sorted(r for (a, r) in cells if a == worse):
            for br in sorted(r for (a, r) in cells if a == better):
                kw = key(cells[(worse, wr)], anchor)
                kb = key(cells[(better, br)], anchor)
                ok = kw > kb  # the worse arm must rank below the better one
                passed = passed and ok
                rows.append((wr, br, ok, kw[0], kb[0]))
        lines.append("   %s worse=%s better=%s: %s over %d pairings (%d decided at tier 1)"
                     % (name, worse, better, "PASS" if passed else "FAIL", len(rows),
                        sum(1 for r in rows if r[3] != r[4])))
        for wr, br, ok, ew, eb in rows:
            lines.append("      %s r%d (e=%d) vs %s r%d (e=%d): %s" % (worse, wr, ew, better, br, eb, "ok" if ok else "INVERTED"))

    lines.append("")
    lines.append("precedence sensitivity (all six orders of tiers 2-4, recorded arm aggregates):")
    base = order2
    changed = []
    for perm in itertools.permutations(("H", "S", "B")):
        def pk(a):
            g = aggs[a]
            return (arm_key(g)[0],) + tuple(round(g[n], 12) for n in perm)
        o = sorted(arms, key=pk)
        if o != base:
            changed.append((perm, o))
    lines.append("   orders identical to the declared one: %d of 6" % (6 - len(changed)))
    for perm, o in changed:
        lines.append("   tier order %s -> %s" % ("/".join(perm), " < ".join(o)))

    lines.append("")
    lines.append("terminal-sample sensitivity (tier 1 read at h=H-1 instead of h=H):")
    aggs143 = {}
    for a in arms:
        reps = aggs[a]["reps"]
        extinct143 = all(cells[(a, r)]["alive_143"] == 0 for r in reps)
        aggs143[a] = (0 if not extinct143 else 1, aggs[a]["H"], aggs[a]["S"], aggs[a]["B"])
    order143 = sorted(arms, key=lambda x: aggs143[x])
    lines.append("   order: " + " < ".join(order143))
    lines.append("   changed by the convention: %s"
                 % ("none" if order143 == order2 else ", ".join(
                     "%s/%s" % (x, y) for i, x in enumerate(arms) for y in arms[i + 1:]
                     if (order2.index(x) < order2.index(y)) != (order143.index(x) < order143.index(y)))))
    return lines


# ---------------------------------------------------------------------------
# The pre-declared M2c verdict (spec section 7.4) over a live evaluation JSON.
# ---------------------------------------------------------------------------

def live_cells(path):
    doc = load_json(path)
    cells = {}
    for row in doc.get("cells") or []:
        alive = row.get("alive_by_hour")
        if not alive:
            fail("live cell %s/r%s lacks alive_by_hour (absence is not zero)" % (row.get("arm"), row.get("replicate")))
        cells[(row["arm"], row["replicate"])] = {
            "H": float(sum(alive)), "B": float(row["births_cum"][-1]),
            "S": float(row["deaths_starve_cum"][-1]), "alive_end": int(alive[-1]),
            "alive_143": int(alive[-2]) if len(alive) > 1 else int(alive[-1]),
            "calls": int(row.get("realized_calls", 0)),
        }
    return cells, doc


TIER_LABEL = {"H": "actor-hours", "S": "starvation deaths", "B": "births"}
TIER_LOWER_BETTER = {"H": True, "S": True, "B": False}


def pair_verdict(aggs, x, y, tiers=("H", "S", "B"), extinct="extinct_all", persist="persist_all"):
    """Apply the pre-declared band rule to one pair: (winner, why, separates)."""
    gx, gy = aggs[x], aggs[y]
    if gx[extinct] and gy[persist]:
        return (y, "tier 1: %s extinct in all replicates, %s persists in all" % (x, y), 1)
    if gy[extinct] and gx[persist]:
        return (x, "tier 1: %s extinct in all replicates, %s persists in all" % (y, x), 1)
    for name in tiers:
        d = gx[name] - gy[name]
        b = max(gx["band_" + name], gy["band_" + name])
        if abs(d) > b:
            winner = x if (d < 0) == TIER_LOWER_BETTER[name] else y
            if b == 0:
                note = " (both deterministic: no replicate noise, a structural difference)"
            elif min(gx["band_" + name], gy["band_" + name]) == 0:
                note = " (one-sided noise floor)"
            else:
                note = ""
            return (winner, "tier %s: |%.4f - %.4f| = %.4f > band %.4f%s"
                    % (TIER_LABEL[name], gx[name], gy[name], abs(d), b, note), 1)
    return (None, "tie: no tier separates", 0)


def verdict(cells, anchor):
    """Apply the pre-declared band rule. Returns (verdict, lines)."""
    arms = []
    for arm in ("A0", "R", "P", "U", "A", "A1"):
        if any(a == arm for (a, _) in cells):
            arms.append(arm)
    aggs = {a: arm_aggregate(cells, a, anchor) for a in arms}
    lines = ["per-arm bands:"]
    for a in arms:
        g = aggs[a]
        lines.append("   %-3s persisted=%s extinct=%s dH=%.4f(+/-%.4f) dS=%.4f(+/-%.4f) dB=%.4f(+/-%.4f)"
                     % (a, g["persist_all"], g["extinct_all"], g["H"], g["band_H"], g["S"], g["band_S"],
                        g["B"], g["band_B"]))

    declared = [("A", "P", "primary"), ("A", "U", "primary"),
                ("A1", "P", "secondary"), ("A1", "U", "secondary"),
                ("A", "A1", "secondary"), ("P", "U", "secondary")]
    base = {}
    lines.append("declared pairs:")
    primary_sep = secondary_sep = 0
    for x, y, kind in declared:
        if x not in aggs or y not in aggs:
            lines.append("   %s vs %s (%s): NOT RUN — an arm is absent from the artefact" % (x, y, kind))
            continue
        winner, why, sep = pair_verdict(aggs, x, y)
        base[(x, y)] = (winner, sep)
        if sep and kind == "primary":
            primary_sep += 1
        if sep and kind == "secondary":
            secondary_sep += 1
        lines.append("   %s vs %s (%s): %s — %s"
                     % (x, y, kind, ("SEPARATES, more viable: " + winner) if sep else "TIE", why))
    if "A0" in aggs:
        for a in ("P", "U", "A", "A1"):
            if a in aggs:
                winner, why, sep = pair_verdict(aggs, a, "A0")
                lines.append("   %s vs A0 (diagnostic): %s — %s"
                             % (a, ("SEPARATES, more viable: " + winner) if sep else "TIE", why))

    # Declared sensitivity: all six tier orders and the h=H-1 persistence convention.
    moved_prec = []
    for perm in itertools.permutations(("H", "S", "B")):
        for key, ref in base.items():
            got = pair_verdict(aggs, key[0], key[1], tiers=perm)
            if (got[0], got[2]) != ref:
                moved_prec.append((key, perm, ref, (got[0], got[2], got[1])))
    lines.append("precedence sensitivity: %d of %d declared-pair verdicts change over the six tier orders"
                 % (len(moved_prec), len(base) * 6))
    for key, perm, ref, got in moved_prec:
        lines.append("   %s vs %s under %s: %s -> %s (%s)"
                     % (key[0], key[1], "/".join(perm), ref, got[:2], got[2]))
    moved_term = []
    for key, ref in base.items():
        got = pair_verdict(aggs, key[0], key[1], extinct="extinct143_all", persist="persist143_all")
        if (got[0], got[2]) != ref:
            moved_term.append((key, ref, (got[0], got[2], got[1])))
    lines.append("terminal-sample sensitivity (persistence at h=H-1): %d of %d verdicts change"
                 % (len(moved_term), len(base)))
    for key, ref, got in moved_term:
        lines.append("   %s vs %s: %s -> %s (%s)" % (key[0], key[1], ref, got[:2], got[2]))

    verdict_name = ("SEPARATION" if primary_sep else
                    "SECONDARY-ONLY" if secondary_sep else "NO SEPARATION")
    lines.append("M2c verdict (spec section 7.4): %s (primary separating pairs: %d, secondary: %d)"
                 % (verdict_name, primary_sep, secondary_sep))
    return verdict_name, lines


# ---------------------------------------------------------------------------
# Band-rule fixtures: the verdict machinery must be able to fail.
# ---------------------------------------------------------------------------

def band_fixtures():
    """Synthetic cell sets exercising the band rule in every direction."""
    ok = True
    lines = []
    anchor = dict(ANCHOR)

    def cells(reps_by_arm):
        out = {}
        for arm, reps in reps_by_arm.items():
            for i, (H, B, S, alive_end) in enumerate(reps, start=1):
                out[(arm, i)] = {"H": float(H), "B": float(B), "S": float(S),
                                 "alive_end": int(alive_end), "alive_143": int(alive_end), "calls": 0}
        return out

    # (a) two stochastic arms whose survival envelopes overlap: must be a tie.
    c = cells({"P": [(1444, 2, 0, 1), (1444, 2, 0, 1), (1444, 2, 0, 1)],
               "A": [(1443, 2, 0, 2), (1444, 2, 0, 1), (1264, 2, 2, 2)]})
    v, _ = verdict(c, anchor)
    lines.append("fixture a (A's replicate spread covers a 1 actor-hour mean difference): %s" % v)
    ok = ok and v == "NO SEPARATION"

    # (b) an extinct arm against a persisting one: tier 1 must separate.
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1166, 0, 0, 0)] * 3})
    v, _ = verdict(c, anchor)
    lines.append("fixture b (extinct vs persisting, unanimous): %s" % v)
    ok = ok and v == "SEPARATION"

    # (c) same survivor, more starvation deaths, beyond the band: separates.
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1200, 2, 4, 1)] * 3})
    v, _ = verdict(c, anchor)
    lines.append("fixture c (four starvation deaths beyond the survival band): %s" % v)
    ok = ok and v == "SEPARATION"

    # (d) identical arms everywhere: a tie, not a win.
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1444, 2, 0, 1)] * 3})
    v, _ = verdict(c, anchor)
    lines.append("fixture d (identical arms): %s" % v)
    ok = ok and v == "NO SEPARATION"

    # (e) a pair whose verdict must change when the precedence of tiers 2-4 changes: the
    # sensitivity machinery must be able to fire, not merely print zero.
    c = cells({"P": [(1300, 2, 0, 1)] * 3, "A": [(1444, 2, 2, 1)] * 3})
    v, vlines = verdict(c, anchor)
    fired = 0
    for line in vlines:
        if line.startswith("precedence sensitivity: "):
            fired = int(line.split()[2])
    lines.append("fixture e (survival-first says A, deaths-first says P): verdict %s, precedence "
                 "sensitivity fires on %d verdict(s)" % (v, fired))
    ok = ok and fired > 0

    lines.append("band rule fixtures: %s" % ("PASS" if ok else "FAIL"))
    return ok, lines


def self_test(cells_path):
    lines = []
    ok_worlds, world_lines = declared_world_tests(dict(ANCHOR))
    lines += world_lines
    ok_bands, band_lines = band_fixtures()
    lines.append("")
    lines += band_lines
    ok = ok_worlds and ok_bands
    if cells_path:
        cells, anchor, primary = recorded_cells(cells_path)
        lines.append("")
        rec = recorded_report(cells, anchor, primary)
        lines += rec
        worst = 0.0
        for (arm, rep), w in sorted(cells.items()):
            worst = max(worst, abs(vrd1(w, anchor) - w["vrd1"]))
        lines.append("")
        lines.append("recorded loader identity: max |vrd/1 recomputed - recorded| = %.3e — %s"
                     % (worst, "PASS" if worst <= 1e-12 else "FAIL"))
        ok = ok and worst <= 1e-12
        a0 = [w for (a, _), w in cells.items() if a == "A0"]
        t5 = all(key(w, anchor) == (0.0, 0.0, 0.0, 0.0) for w in a0)
        lines.append("T5 on every recorded A0 replicate: %s" % ("PASS" if t5 else "FAIL"))
        ok = ok and t5
    lines.append("")
    lines.append("SELF-TEST: %s" % ("PASS" if ok else "FAIL"))
    return ok, lines


def expect_fail(cells_path):
    """The negative control: the inverted comparison must be detected as a FAIL."""
    t = dict(ANCHOR)
    detected = []
    for name, x, y in (("T2", "W1", "W2"), ("T3", "W3", "W2"), ("T4", "W0", "W2")):
        good = key(WORLDS[x], t) < key(WORLDS[y], t)
        inverted = key(WORLDS[y], t) < key(WORLDS[x], t)
        detected.append((name, good, inverted))
    lines = []
    ok = True
    for name, good, inverted in detected:
        lines.append("NEGATIVE-CONTROL %s: declared order holds=%s ; inverted claim also holds=%s "
                     "(must be False)" % (name, good, inverted))
        ok = ok and good and not inverted
    if cells_path:
        cells, anchor, primary = recorded_cells(cells_path)
        aggs = {a: arm_aggregate(cells, a) for a in ("A0", "R", "P", "U", "A", "A1")
                if any(x == a for (x, _) in cells)}
        # A1 is extinct, A persists: the inverted claim must be detectable.
        inv = arm_key(aggs["A1"]) < arm_key(aggs["A"])
        lines.append("NEGATIVE-CONTROL recorded A1<A (the extinct arm declared more viable): "
                     "inverted claim holds=%s (must be False)" % inv)
        ok = ok and not inv
    lines.append("NEGATIVE CONTROL %s: every inverted comparison was detected as a failure"
                 % ("PASS" if ok else "FAIL"))
    return ok, lines


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--scores", help="ticket 37's committed scores.json (recorded vectors)")
    ap.add_argument("--live", help="an evaluation JSON from a live run")
    ap.add_argument("--log", help="write the transcript here as well as stdout")
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--expect-fail", action="store_true")
    args = ap.parse_args()

    lines = []
    rc = 0
    if args.expect_fail:
        ok, lines = expect_fail(args.scores)
        if not ok:
            rc = 2
    elif args.self_test:
        ok, lines = self_test(args.scores)
        if not ok:
            rc = 2
    elif args.live:
        cells, doc = live_cells(args.live)
        anchor_doc = {}
        if args.scores:
            _, anchor_doc, _ = recorded_cells(args.scores)
        anchor = anchor_doc or dict(ANCHOR)
        lines.append("live artefact: contract=%s horizon=%s status=%s"
                     % (doc.get("claim_contract"), doc.get("horizon_hours"), doc.get("status")))
        v, vlines = verdict(cells, anchor)
        lines += vlines
    else:
        if args.scores:
            cells, anchor, primary = recorded_cells(args.scores)
            lines += recorded_report(cells, anchor, primary)
        else:
            fail("nothing to do: pass --scores, --live, --self-test or --expect-fail")

    text = "\n".join(lines) + "\n"
    sys.stdout.write(text)
    if args.log:
        with open(args.log, "w", encoding="utf-8") as fh:
            fh.write(text)
    raise SystemExit(rc)


if __name__ == "__main__":
    main()
