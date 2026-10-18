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
  --live PATH     an evaluation JSON from a live run; asserts the declared cell
                  inventory, then applies the pre-declared band rule and prints a
                  verdict per declared pair, or NOT RUN if the inventory is not met;
  --check-inventory PATH  the same inventory assertion alone (spec section 8.4);
  --self-test     the declared ordering tests T1-T7 on worlds W0-W5, the band-rule and
                  inventory fixtures, and the loader identity check. Must pass before any
                  score is read (spec section 7.6, gate 1);
  --expect-fail   the negative control: every declared comparison is inverted and must
                  be detected as a FAIL;
  --write-smoke PATH  write the declared synthetic 15-cell artefact the --live CLI smoke
                  scores (shape and plumbing only, never a result about any arm).

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
    # W4/W5 are the declared tier-2-over-tier-3 witness pair (spec section 6, T7): W4 is
    # one actor-hour better and three starvation deaths worse than W5, and tier 2 decides it.
    "W4": {"label": "survives at the control's hours and births, three starvation deaths (synthetic)",
           "H": 1444.0, "B": 2.0, "S": 3.0, "alive_end": 1},
    "W5": {"label": "one actor-hour short of the control, no starvation deaths (W4's comparator)",
           "H": 1443.0, "B": 2.0, "S": 0.0, "alive_end": 1},
}

# The anchor as recorded (ticket 37 scores.json A0 rows are identical across replicates).
ANCHOR = {"H": 1444.0, "B": 2.0, "S": 0.0, "N0": 10.0}

# T4's declared order over the four originals, and T7's over the widened witness set.
CORE_WORLDS = ["W0", "W1", "W2", "W3"]
DECLARED_WORLD_ORDER = ["W0", "W1", "W3", "W2"]
DECLARED_WITNESS_ORDER = ["W0", "W1", "W3", "W4", "W5", "W2"]

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
# Declared ordering tests T1-T7 (spec section 6).
# ---------------------------------------------------------------------------

def declared_world_tests(t):
    """Return (passed, lines). T1-T7 are the acceptance of the metric choice."""
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

    got = sorted(CORE_WORLDS, key=lambda n: keys[n])
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
    core = {n: ws[n] for n in CORE_WORLDS}
    lines.append("   P2 vrd/2f: " + " < ".join(order_by(vrd2f, core, t)))
    lines.append("   order P2: " + "  ".join("%s=%.4f" % (n, vrd2f(ws[n], t)) for n in CORE_WORLDS))
    p2_ok = vrd2f(ws["W1"], t) < vrd2f(ws["W2"], t)
    lines.append("   P2 T2 %s (W1 %.4f vs W2 %.4f); required weight ratio w_H/w_B > 1/%.5f = %.2f"
                 % ("PASS" if p2_ok else "FAIL", vrd2f(ws["W1"], t), vrd2f(ws["W2"], t),
                    components(ws["W2"], t)["H"], 1.0 / components(ws["W2"], t)["H"]))
    lines.append("   P3 vrd/2s: " + " < ".join(order_by(vrd2s, core, t)))
    lines.append("   order P3: " + "  ".join("%s=%.4f" % (n, vrd2s(ws[n], t)) for n in CORE_WORLDS))
    p3_t2 = vrd2s(ws["W1"], t) < vrd2s(ws["W2"], t)
    p3_t3 = vrd2s(ws["W3"], t) < vrd2s(ws["W2"], t)
    lines.append("   P3 T2 %s (W1 %.4f vs W2 %.4f); P3 T3 %s (W3 %.4f vs W2 %.4f)"
                 % ("PASS" if p3_t2 else "FAIL", vrd2s(ws["W1"], t), vrd2s(ws["W2"], t),
                    "PASS" if p3_t3 else "FAIL", vrd2s(ws["W3"], t), vrd2s(ws["W2"], t)))

    # T7 is printed after the rejected alternatives so the committed section-R2 transcript's
    # first block stays verbatim; it is a declared ordering test like T2-T6.
    lines.append("")
    lines.append("T7 the tier-2-over-tier-3 witness pair W4/W5 (spec section 6):")
    for n in ("W4", "W5"):
        lines.append("   vrd/2 key %s = %s" % (n, fmt_key(keys[n])))
    got6 = sorted(ws, key=lambda n: keys[n])
    t7 = got6 == DECLARED_WITNESS_ORDER and keys["W4"] < keys["W5"]
    lines.append("T7 %s: the order is %s (declared %s); W4 survives one actor-hour better than W5 and "
                 "three starvation deaths worse, and tier 2 decides it"
                 % ("PASS" if t7 else "FAIL", " < ".join(got6), " < ".join(DECLARED_WITNESS_ORDER)))
    ok = ok and t7
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

    ks = sorted(cells)
    rep_flips = []
    for i, x in enumerate(ks):
        for y in ks[i + 1:]:
            before = vrd1(cells[x], anchor) < vrd1(cells[y], anchor)
            after = key(cells[x], anchor) < key(cells[y], anchor)
            if before != after:
                rep_flips.append((x, y, vrd1(cells[x], anchor), vrd1(cells[y], anchor)))
    lines.append("replicate-level comparisons the repair corrects: %d of %d cell pairs"
                 % (len(rep_flips), len(ks) * (len(ks) - 1) // 2))
    for x, y, dx, dy in rep_flips:
        lines.append("   %s/r%d vs %s/r%d: vrd/1 %.4f vs %.4f (extinct-at-H ranked better) -> vrd/2 ranks the "
                     "persisting cell better" % (x[0], x[1], y[0], y[1], dx, dy))

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
    cells, halted = cells_of(doc)
    return cells, halted, doc


LIVE_ROW_KEYS = ("arm", "replicate", "ran", "halted", "alive_by_hour", "births_cum",
                 "deaths_starve_cum", "realized_calls")


def cells_of(doc):
    """The state vector of every scored cell row, plus the rows that are not complete runs."""
    cells = {}
    halted = set()
    for row in doc.get("cells") or []:
        label = "%s/r%s" % (row.get("arm"), row.get("replicate"))
        for k in LIVE_ROW_KEYS:
            if k not in row:
                fail("live cell %s lacks %s (absence is not zero)" % (label, k))
        if not row["alive_by_hour"]:
            fail("live cell %s has an empty alive_by_hour (absence is not zero)" % label)
        if row["halted"] or not row["ran"]:
            halted.add(label)
        cells[(row["arm"], row["replicate"])] = {
            "H": float(sum(row["alive_by_hour"])), "B": float(row["births_cum"][-1]),
            "S": float(row["deaths_starve_cum"][-1]), "alive_end": int(row["alive_by_hour"][-1]),
            "alive_143": int(row["alive_by_hour"][-2]) if len(row["alive_by_hour"]) > 1
            else int(row["alive_by_hour"][-1]),
            "calls": int(row["realized_calls"]),
        }
    return cells, halted


# The declared M2c inventory (spec section 7.1): 5 arms x 3 replicates = 15 scored
# cells, of which A0's 3 are script-only (0 calls) and 12 are model-bearing. A cell
# that errored is omitted from the artefact's cells array and counted in
# inventory.skipped, so a partial matrix must be refused by the scorer rather than
# scored as a smaller experiment: with fewer than 3 replicates the band shrinks
# towards zero and any difference becomes a "separation".
DECLARED_LIVE_ARMS = ("A0", "P", "U", "A", "A1")
DECLARED_REPLICATES = 3
DECLARED_LIVE_CELLS = len(DECLARED_LIVE_ARMS) * DECLARED_REPLICATES
DECLARED_HOURS = 144


def inventory_problems(doc, cells, halted):
    """The declared cell-inventory assertion of spec section 8.4 (step 5b)."""
    problems = []
    inv = doc.get("inventory")
    if not isinstance(inv, dict):
        problems.append("the artefact carries no inventory block")
        inv = {}
    if inv.get("ran") != DECLARED_LIVE_CELLS:
        problems.append("inventory.ran = %s, declared %d (3 x A0 + 12 model-bearing)"
                        % (inv.get("ran"), DECLARED_LIVE_CELLS))
    if inv.get("declared") != DECLARED_LIVE_CELLS:
        problems.append("inventory.declared = %s, declared %d" % (inv.get("declared"), DECLARED_LIVE_CELLS))
    if inv.get("skipped"):
        problems.append("inventory.skipped = %d cell(s): %s"
                        % (len(inv["skipped"]), ", ".join(str(s) for s in inv["skipped"])))
    if len(cells) != DECLARED_LIVE_CELLS:
        problems.append("%d scored cell(s) present, declared %d" % (len(cells), DECLARED_LIVE_CELLS))
    for arm in DECLARED_LIVE_ARMS:
        n = sum(1 for (a, _) in cells if a == arm)
        if n != DECLARED_REPLICATES:
            problems.append("%s carries %d replicate(s), declared %d" % (arm, n, DECLARED_REPLICATES))
    if halted:
        problems.append("%d cell(s) did not complete (halted at the call ceiling or not run): %s"
                        % (len(halted), ", ".join(sorted(halted))))
    return problems


TIER_LABEL = {"H": "actor-hours", "S": "starvation deaths", "B": "births"}
TIER_LOWER_BETTER = {"H": True, "S": True, "B": False}

# Declared per-tier effect floors (spec section 7.4): a tier separates only when the
# replicate-mean difference clears the replicate band AND this floor, so a zero band
# cannot promote a practically nil difference to the headline.
EFFECT_FLOOR = {"H": 0.01, "S": 0.1, "B": 0.5}

# A replicate mean of three equal quanta can sit ~1e-17 above the quantum, so the
# comparison ignores representation error below 1e-12 — four orders of magnitude below
# the smallest difference any tier can carry (one actor-hour is 6.9e-4).
FLOAT_TOLERANCE = 1e-12


def pair_verdict(aggs, x, y, tiers=("H", "S", "B"), extinct="extinct_all", persist="persist_all"):
    """Apply the pre-declared band rule to one pair: (winner, why, separates)."""
    gx, gy = aggs[x], aggs[y]
    if gx[extinct] and gy[persist]:
        return (y, "tier 1: %s extinct in all replicates, %s persists in all" % (x, y), 1)
    if gy[extinct] and gx[persist]:
        return (x, "tier 1: %s extinct in all replicates, %s persists in all" % (y, x), 1)
    inside = []
    for name in tiers:
        d = gx[name] - gy[name]
        b = max(gx["band_" + name], gy["band_" + name])
        floor = EFFECT_FLOOR[name]
        if abs(d) > max(b, floor) + FLOAT_TOLERANCE:
            winner = x if (d < 0) == TIER_LOWER_BETTER[name] else y
            note = " ; the replicate band is zero, so the effect floor is what makes it meaningful" if b == 0 else \
                   (" ; one-sided noise floor" if min(gx["band_" + name], gy["band_" + name]) == 0 else "")
            return (winner, "tier %s: |%.4f - %.4f| = %.4f > max(band %.4f, floor %.4f), decided by the %s%s"
                    % (TIER_LABEL[name], gx[name], gy[name], abs(d), b, floor,
                       "band" if b >= floor else "effect floor", note), 1)
        inside.append((TIER_LABEL[name], abs(d), b, floor))
    return (None, "tie: no tier separates — " + "; ".join(
        "%s |%.4f| <= max(band %.4f, floor %.4f)" % t for t in inside), 0)


def verdict(cells, anchor):
    """Apply the pre-declared band rule. Returns (verdict, lines)."""
    arms = []
    for arm in ("A0", "R", "P", "U", "A", "A1"):
        if any(a == arm for (a, _) in cells):
            arms.append(arm)
    aggs = {a: arm_aggregate(cells, a, anchor) for a in arms}
    short = [a for a in arms if len(aggs[a]["reps"]) != DECLARED_REPLICATES]
    if short:
        return ("NOT RUN", ["declared pairs: NOT RUN — %s carries %s replicate(s), declared %d; a truncated "
                             "arm has a narrower band and would separate on noise"
                             % (", ".join(short), "/".join(str(len(aggs[a]["reps"])) for a in short),
                                DECLARED_REPLICATES)])
    lines = ["declared effect floors (spec section 7.4): actor-hours %.2f (%.1f actor-hours at H_T = %.0f), "
             "starvation deaths %.2f, births %.2f; a tier separates only above max(band, floor)"
             % (EFFECT_FLOOR["H"], EFFECT_FLOOR["H"] * anchor["H"], anchor["H"],
                EFFECT_FLOOR["S"], EFFECT_FLOOR["B"]),
             "per-arm bands:"]
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

    # (f) zero band, practically nil difference (one actor-hour of 1 444): the effect floor
    # must refuse it, which the band alone cannot do.
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1443, 2, 0, 1)] * 3})
    v, _ = verdict(c, anchor)
    lines.append("fixture f (1 actor-hour of a 1 444-actor-hour control, band 0): %s" % v)
    ok = ok and v == "NO SEPARATION"

    # (g) the same rule at the starvation tier: exactly one death is inside the floor, three
    # deaths clear it.
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1444, 2, 1, 1)] * 3})
    v, _ = verdict(c, anchor)
    lines.append("fixture g1 (exactly one starvation death, band 0): %s" % v)
    ok = ok and v == "NO SEPARATION"
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1444, 2, 3, 1)] * 3})
    v, _ = verdict(c, anchor)
    lines.append("fixture g2 (three starvation deaths, band 0): %s" % v)
    ok = ok and v == "SEPARATION"

    # (h) a real difference on a zero band must still separate: the floor is a threshold,
    # not a blanket refusal.
    c = cells({"P": [(1444, 2, 0, 1)] * 3, "A": [(1420, 2, 0, 1)] * 3})
    v, vlines = verdict(c, anchor)
    lines.append("fixture h (24 actor-hours of the control, band 0): %s" % v)
    ok = ok and v == "SEPARATION" and any("decided by the effect floor" in line for line in vlines)

    lines.append("band rule fixtures: %s" % ("PASS" if ok else "FAIL"))
    return ok, lines


def smoke_artefact():
    """A synthetic artefact in the live evaluation shape: the declared 15-cell inventory
    (5 arms x 3 replicates, 145 hourly samples). Shape and plumbing only — never a result
    about any arm. Its replicates are deterministic except A's, so it exercises both the
    band path and the zero-band effect-floor path."""
    hours = list(range(DECLARED_HOURS + 1))

    def row(arm, replicate, alive_flat, alive_end, births, starve, calls):
        return {"arm": arm, "replicate": replicate, "hours": hours,
                "alive_by_hour": [alive_flat] * DECLARED_HOURS + [alive_end],
                "births_cum": [0 if h < 100 else births for h in hours],
                "deaths_starve_cum": [0 if h < 50 else starve for h in hours],
                "deaths_age_cum": [0] * len(hours),
                "wages_cum": [2 * h for h in hours],
                "max_capital": [12 + h // 24 for h in hours],
                "capital_hist": [[max(0, 5 - h // 48)] for h in hours],
                "granary_hist": [[3] for h in hours],
                "realized_calls": calls, "ran": True, "halted": False}

    cells = []
    for rep in (1, 2, 3):
        cells.append(row("A0", rep, 10, 4, 2, 0, 0))
        cells.append(row("P", rep, 10, 4, 2, 0, 244))
        cells.append(row("U", rep, 8, 1, 2, 5, 256))
        cells.append(row("A1", rep, 9, 0, 1, 0, 253))
    for rep, (flat, end, starve) in enumerate(((9, 2, 2), (9, 4, 2), (8, 10, 3)), start=1):
        cells.append(row("A", rep, flat, end, 2, starve, 256))
    return {"status": "smoke-fixture (shape and plumbing only; not a result about any arm)",
            "claim_contract": "natural-claim/v4", "horizon_hours": DECLARED_HOURS,
            "inventory": {"declared": DECLARED_LIVE_CELLS, "ran": DECLARED_LIVE_CELLS, "skipped": []},
            "cells": cells}


def inventory_fixtures():
    """The declared inventory assertion: a complete artefact scores, a truncated or halted
    one is refused as NOT RUN rather than scored as a smaller experiment."""
    ok = True
    lines = []

    def variant(mutate):
        doc = json.loads(json.dumps(smoke_artefact()))
        mutate(doc)
        return doc

    def drop_arm(doc):
        doc["cells"] = [c for c in doc["cells"] if c["arm"] != "A1"]
        doc["inventory"]["ran"] = 12
        doc["inventory"]["skipped"] = ["A1-f10-s0-r1:model-unavailable"]

    def drop_replicate(doc):
        doc["cells"] = [c for c in doc["cells"] if not (c["arm"] == "A" and c["replicate"] == 3)]
        doc["inventory"]["ran"] = 14

    def halt_cell(doc):
        for c in doc["cells"]:
            if c["arm"] == "U" and c["replicate"] == 2:
                c["halted"] = True

    def understate_ran(doc):
        doc["inventory"]["ran"] = 14

    cases = [
        ("the declared 15-cell artefact", variant(lambda d: None), False),
        ("one declared arm absent (A1 dropped, ran 12)", variant(drop_arm), True),
        ("one declared arm with two replicates (A/r3 dropped, ran 14)", variant(drop_replicate), True),
        ("one cell halted at its call ceiling", variant(halt_cell), True),
        ("inventory.ran short of the artefact's cells", variant(understate_ran), True),
    ]
    for label, doc, expect_refused in cases:
        cells, halted = cells_of(doc)
        problems = inventory_problems(doc, cells, halted)
        refused = bool(problems)
        lines.append("inventory fixture (%s): %s%s"
                     % (label, "NOT RUN" if refused else "scored",
                        " — " + problems[0] if problems else ""))
        ok = ok and refused == expect_refused

    shortened = variant(drop_replicate)
    cells, _ = cells_of(shortened)
    v, _ = verdict(cells, dict(ANCHOR))
    lines.append("verdict over the two-replicate arm: %s (must be NOT RUN)" % v)
    ok = ok and v == "NOT RUN"

    lines.append("inventory fixtures: %s" % ("PASS" if ok else "FAIL"))
    return ok, lines


def reachability_check(cells, anchor):
    """The effect floor must not make the criterion confirmatory by construction: on the recorded v2
    shape it still yields one tie and one separation. A reachability check on ticket-35 v2 aggregates,
    NOT an M2c result (v2 is the wrong contract to answer M2c)."""
    aggs = {a: arm_aggregate(cells, a, anchor) for a in ("P", "U", "A")}
    ap = pair_verdict(aggs, "A", "P")
    au = pair_verdict(aggs, "A", "U")
    ok = ap[2] == 0 and au[2] == 1
    lines = ["reachability on the recorded v2 shape (ticket-35 aggregates; not an M2c result):",
             "   A vs P: %s — %s" % ("SEPARATES" if ap[2] else "TIE", ap[1]),
             "   A vs U: %s — %s" % ("SEPARATES" if au[2] else "TIE", au[1]),
             "   %s: with the effect floors in force the rule still yields both a tie and a separation"
             % ("PASS" if ok else "FAIL")]
    return ok, lines


def self_test(cells_path):
    lines = []
    ok_worlds, world_lines = declared_world_tests(dict(ANCHOR))
    lines += world_lines
    ok_bands, band_lines = band_fixtures()
    lines.append("")
    lines += band_lines
    ok_inventory, inventory_lines = inventory_fixtures()
    lines.append("")
    lines += inventory_lines
    ok = ok_worlds and ok_bands and ok_inventory
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
        ok_reach, reach_lines = reachability_check(cells, anchor)
        lines.append("")
        lines += reach_lines
        ok = ok and ok_reach
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
    ap.add_argument("--check-inventory", metavar="PATH",
                    help="assert the declared M2c cell inventory of a live evaluation JSON")
    ap.add_argument("--write-smoke", metavar="PATH",
                    help="write the declared synthetic 15-cell artefact the --live smoke scores")
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
    elif args.write_smoke:
        doc = smoke_artefact()
        directory = os.path.dirname(args.write_smoke)
        if directory:
            os.makedirs(directory, exist_ok=True)
        with open(args.write_smoke, "w", encoding="utf-8") as fh:
            json.dump(doc, fh, indent=1)
        lines.append("wrote the declared %d-cell smoke artefact to %s (shape only: not a result about "
                     "any arm)" % (DECLARED_LIVE_CELLS, args.write_smoke))
    elif args.check_inventory or args.live:
        path = args.check_inventory or args.live
        cells, halted, doc = live_cells(path)
        problems = inventory_problems(doc, cells, halted)
        lines.append("live artefact: contract=%s horizon=%s status=%s"
                     % (doc.get("claim_contract"), doc.get("horizon_hours"), doc.get("status")))
        lines.append("cell inventory (spec section 8.4): %s"
                     % ("PASS — %d of %d declared cells scored, %d replicates per arm, 0 halted, 0 skipped"
                        % (len(cells), DECLARED_LIVE_CELLS, DECLARED_REPLICATES) if not problems else "FAIL"))
        for problem in problems:
            lines.append("   " + problem)
        if problems:
            lines.append("M2c verdict (spec section 7.4): NOT RUN — the declared inventory is not met; the "
                         "matrix is reported as incomplete and never scored as a smaller experiment")
            rc = 3
        elif args.check_inventory:
            pass
        else:
            anchor_doc = {}
            if args.scores:
                _, anchor_doc, _ = recorded_cells(args.scores)
            anchor = anchor_doc or dict(ANCHOR)
            v, vlines = verdict(cells, anchor)
            lines += vlines
            if v == "NOT RUN":
                rc = 3
    else:
        if args.scores:
            cells, anchor, primary = recorded_cells(args.scores)
            lines += recorded_report(cells, anchor, primary)
        else:
            fail("nothing to do: pass --scores, --live, --self-test, --check-inventory or --expect-fail")

    text = "\n".join(lines) + "\n"
    sys.stdout.write(text)
    if args.log:
        with open(args.log, "w", encoding="utf-8") as fh:
            fh.write(text)
    raise SystemExit(rc)


if __name__ == "__main__":
    main()
