#!/usr/bin/env python3
"""Stage S of ticket 58: the declared §3/§3.2 rule on cell f11-s2 (stability).

READ-ONLY. Loads the recorded Stage-S evaluation (arms A0,R,P,U,A in one cell),
the Stage-T f10-s0 evaluation (to ask whether the Stage-T pattern reproduces) and
the derived accounting. No cell, no model call, no spend. The rule is the
pre-registered one (`.scratch/58-w3-preregistration/spec.md` §3 and §3.2) applied
verbatim; `paired()`/`power()` are the Stage-T implementation unchanged, so the
two cells' numbers are comparable. No rule is tuned here and no cell is pooled.

The unit is one condition's per-hour KPI curve over 3 replicates in one cell.
The reference is the IN-RUN `R@max` (Stage S draws its own reference replicates
on f11-s2; Stage T's f10-s0 R rows are a different cell and are never pooled).

  * `d(t) = mean_A(t) - mean_B(t)` over the 3 replicates of each condition;
  * `RMSE(d)` over hours; `noise` = RMSE over hours of the pooled within-condition
    SD at each hour; `SNR = RMSE(d)/noise` (NaN when both are deterministic);
  * a separation at hour t iff `|d(t)|` exceeds the replicate envelope half-width
    of *both* conditions; a KPI separates iff `SNR >= 2` AND some separating run
    is >= 24 consecutive hours;
  * `power` = the smallest constant per-KPI difference separable at 3 replicates
    and the observed bands (§3 "power");
  * direction is description only; no scalar weighting exists anywhere.

Usage:
  python3 analyze.py --eval evaluation.json \
      --timing-eval ../59-w3-timing/evaluation.json \
      --wall wall.json --out analysis.md --json analysis.json
"""
import argparse
import json
import math
import os
import sys

SCALARS = ["population", "births_cum", "deaths_starve_cum", "deaths_age_cum", "max_capital"]
INSTRUMENT_EXTRA = ["wages_cum"]  # carried by curve/1 but not a §3 scalar
HISTS = ["capital_hist", "granary_hist"]
SNR_THRESHOLD = 2.0
SUSTAINED_HOURS = 24

DIRECTION = {
    "population": "higher",
    "births_cum": "higher",
    "deaths_starve_cum": "lower",
    "deaths_age_cum": "lower",
    "max_capital": None,
    "wages_cum": None,
}

ARRAY_KEYS = {
    "population": "alive_by_hour",
    "births_cum": "births_cum",
    "deaths_starve_cum": "deaths_starve_cum",
    "deaths_age_cum": "deaths_age_cum",
    "max_capital": "max_capital",
    "wages_cum": "wages_cum",
}


def mean_sd(xs):
    n = len(xs)
    if n == 0:
        return 0.0, 0.0
    m = sum(xs) / n
    if n == 1:
        return m, 0.0
    var = sum((x - m) ** 2 for x in xs) / (n - 1)
    return m, math.sqrt(var)


def wasserstein1(a, b):
    if not a or not b:
        return float("nan")
    a, b = sorted(a), sorted(b)
    i = j = 0
    prev = min(a[0], b[0])
    total = 0.0
    while i < len(a) or j < len(b):
        if i >= len(a):
            nxt = b[j]
        elif j >= len(b):
            nxt = a[i]
        else:
            nxt = min(a[i], b[j])
        total += abs(i / len(a) - j / len(b)) * (nxt - prev)
        while i < len(a) and a[i] == nxt:
            i += 1
        while j < len(b) and b[j] == nxt:
            j += 1
        prev = nxt
    return total


def hist_samples(rows):
    out = []
    for bucket, count in enumerate(rows):
        n = int(round(count))
        out.extend([float(bucket)] * max(0, n))
    return out or [0.0]


def hist_w1(a_rows, b_rows):
    h = min(len(a_rows), len(b_rows))
    if h == 0:
        return float("nan")
    return sum(wasserstein1(hist_samples(a_rows[i]), hist_samples(b_rows[i]))
               for i in range(h)) / h


def series(cell, kpi):
    return [float(v) for v in cell[ARRAY_KEYS[kpi]]]


def load(path):
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def cells_of(evaluation, source_tag):
    out = []
    for row in evaluation.get("cells", []):
        if not row.get("ran") or not row.get("hours"):
            continue
        effort = row.get("effort") or "model-default"
        out.append({
            "condition": "%s@%s" % (row["arm"], effort),
            "cell": "f%d-s%d" % (row["f"], row["seed"]),
            "replicate": row["replicate"],
            "tag": "%s.%s.r%d" % (source_tag, row["arm"], row["replicate"]),
            "row": row,
        })
    return out


def avg_rows(cells, family):
    rows = [c["row"][family] for c in cells]
    h = min(len(r) for r in rows)
    width = min(len(r[0]) for r in rows)
    out = []
    for i in range(h):
        out.append([sum(r[i][b] for r in rows) / len(rows) for b in range(width)])
    return out


def power(comp):
    """Smallest constant difference the declared rule could separate here."""
    noise = comp["noise"]
    band = comp["band"]
    snr_floor = SNR_THRESHOLD * noise if noise > 0 else float("nan")
    h = len(band)
    if h < SUSTAINED_HOURS:
        band_floor = float("nan")
    else:
        band_floor = min(max(band[i:i + SUSTAINED_HOURS])
                         for i in range(h - SUSTAINED_HOURS + 1))
    floors = [x for x in (snr_floor, band_floor) if not math.isnan(x)]
    return {
        "noise_floor": snr_floor,
        "band_floor": band_floor,
        "smallest_separable": max(floors) if floors else float("nan"),
        "max_band": max(band) if band else float("nan"),
    }


def paired(kpi, a_cells, b_cells):
    a = [series(c["row"], kpi) for c in a_cells]
    b = [series(c["row"], kpi) for c in b_cells]
    h = min(min(len(s) for s in a), min(len(s) for s in b))
    sq_diff = sq_noise = 0.0
    sep, band, diffs = [], [], []
    for i in range(h):
        av = [s[i] for s in a]
        bv = [s[i] for s in b]
        am, _ = mean_sd(av)
        bm, _ = mean_sd(bv)
        d = am - bm
        diffs.append(d)
        sq_diff += d * d
        dof = (len(av) - 1) + (len(bv) - 1)
        noise = 0.0
        if dof > 0:
            inner = sum((v - am) ** 2 for v in av) + sum((v - bm) ** 2 for v in bv)
            noise = math.sqrt(inner / dof)
        sq_noise += noise * noise
        a_lo, a_hi = min(av), max(av)
        b_lo, b_hi = min(bv), max(bv)
        width = max(a_hi - am, am - a_lo, b_hi - bm, bm - b_lo)
        band.append(width)
        sep.append(abs(d) > width)
    rmse = math.sqrt(sq_diff / h)
    noise = math.sqrt(sq_noise / h)
    snr = rmse / noise if noise > 0 else float("nan")
    first = next((i for i, s in enumerate(sep) if s), -1)
    run_start, run_len, best_start, best_len = -1, 0, -1, 0
    for i, s in enumerate(sep):
        if not s:
            run_start, run_len = -1, 0
            continue
        if run_start < 0:
            run_start = i
        run_len = i - run_start + 1
        if run_len > best_len:
            best_start, best_len = run_start, run_len
    direction = ""
    if best_len > 0:
        mean_run = sum(diffs[best_start:best_start + best_len]) / best_len
        direction = "higher" if mean_run > 0 else "lower" if mean_run < 0 else "flat"
    comp = {
        "kpi": kpi, "hours": h,
        "rmse_d": rmse, "noise": noise, "snr": snr,
        "first_separation": first,
        "run_start": best_start, "run_length": best_len,
        "separation_direction": direction,
        "max_abs_d": max((abs(d) for d in diffs), default=0.0),
        "mean_d_at_H": diffs[-1] if diffs else 0.0,
        "band": band,
    }
    comp["detectable"] = ((not math.isnan(snr)) and snr >= SNR_THRESHOLD
                          and best_len >= SUSTAINED_HOURS)
    if not comp["detectable"]:
        comp["separation_direction"] = ""
    comp["power"] = power(comp)
    return comp


def quality(kpi, direction):
    want = DIRECTION.get(kpi)
    if want is None or direction in ("", "flat"):
        return None
    return "favourable" if direction == want else "unfavourable"


def compare(name_a, cells_a, name_b, cells_b):
    out = {"a": name_a, "b": name_b, "cell": cells_a[0]["cell"],
           "replicates_a": len(cells_a), "replicates_b": len(cells_b), "kpis": {}}
    for kpi in SCALARS + INSTRUMENT_EXTRA:
        comp = paired(kpi, cells_a, cells_b)
        comp["wasserstein"] = wasserstein1([v for c in cells_a for v in series(c["row"], kpi)],
                                           [v for c in cells_b for v in series(c["row"], kpi)])
        comp["quality"] = quality(kpi, comp["separation_direction"]) if comp["detectable"] else None
        out["kpis"][kpi] = comp
    for family in HISTS:
        out["kpis"][family] = {
            "kpi": family, "kind": "histogram",
            "wasserstein": hist_w1(avg_rows(cells_a, family), avg_rows(cells_b, family)),
        }
    seps = [k for k in SCALARS if out["kpis"][k]["detectable"]]
    out["separations"] = [{"kpi": k,
                           "direction": out["kpis"][k]["separation_direction"],
                           "quality": out["kpis"][k]["quality"]} for k in seps]
    out["against_a"] = [s for s in out["separations"] if s["quality"] == "unfavourable"]
    out["for_a"] = [s for s in out["separations"] if s["quality"] == "favourable"]
    out["neutral"] = [s for s in out["separations"] if s["quality"] is None]
    return out


def endpoints(cells, kpis=SCALARS):
    out = {}
    for kpi in kpis:
        rows = [series(c["row"], kpi) for c in cells]
        out[kpi] = sum(r[-1] for r in rows) / len(rows)
    return out


def agreement_pair(ra, rb, kpis=SCALARS):
    h = min(len(ra["hours"]), len(rb["hours"]))
    hours = [k for k in range(h)
             if any(series(ra, kpi)[k] != series(rb, kpi)[k] for kpi in kpis)]
    return {
        "hours_any_scalar_differ": len(hours),
        "first_differ_hour": hours[0] if hours else -1,
        "last_differ_hour": hours[-1] if hours else -1,
        "identical": not hours,
    }


def fmt(x, digits=3):
    if isinstance(x, float) and math.isnan(x):
        return "n/a"
    if isinstance(x, float):
        return ("%%.%df" % digits) % x
    return str(x)


def baseline_outcome(pair):
    """The §3.2 outcome of A against one baseline (A is the `a` side)."""
    if pair["against_a"]:
        return "worse"
    if pair["for_a"]:
        return "beats"
    return "matches"


def resolution_state(pair):
    """Is every observed separation below the reported detection limit?"""
    below = True
    observed = 0.0
    for kpi in SCALARS:
        c = pair["kpis"][kpi]
        observed = max(observed, c["max_abs_d"])
        if c["detectable"]:
            return "detected"
        lim = c["power"]["smallest_separable"]
        if not math.isnan(lim) and c["max_abs_d"] >= lim:
            below = False
    if observed == 0.0:
        return "identical curves (zero difference at every hour)"
    return "unresolvable" if below else "sub-threshold"


def pattern_of(conds, order=("A", "P", "U", "A0")):
    """The categorical Stage-T pattern: A vs P/U, and U's identity with A0."""
    pairs = {
        "A_vs_P": compare("A", conds["A"], "P", conds["P"]),
        "A_vs_U": compare("A", conds["A"], "U", conds["U"]),
    }
    outcomes = {b: baseline_outcome(pairs["A_vs_" + b]) for b in ("P", "U")}
    u_a0 = agreement_pair(conds["U"][0]["row"], conds["A0"][0]["row"])
    return {
        "per_baseline": outcomes,
        "A_vs_P_separations": len(pairs["A_vs_P"]["separations"]),
        "A_vs_U_separations": len(pairs["A_vs_U"]["separations"]),
        "U_vs_A0": u_a0,
        "pattern_label": "A matches P and U; U identical to A0"
        if outcomes["P"] == outcomes["U"] == "matches" and u_a0["identical"]
        else "A %s P, A %s U; U %s A0" % (
            outcomes["P"], outcomes["U"],
            "identical to" if u_a0["identical"] else "differs from"),
    }, pairs


def cross_cell(conds_t, conds_s):
    """Descriptive cross-cell endpoints. No pooling, no verdict, no curve test."""
    out = {}
    for arm in ("A0", "P", "U", "A"):
        et = endpoints(conds_t[arm])
        es = endpoints(conds_s[arm])
        out[arm] = {
            "f10-s0": et,
            "f11-s2": es,
            "delta_s_minus_t": {k: es[k] - et[k] for k in SCALARS},
        }
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--eval", required=True, help="Stage-S evaluation JSON (f11-s2, arms A0,R,P,U,A)")
    ap.add_argument("--timing-eval", required=True, help="Stage-T evaluation JSON (f10-s0, arms A0,P,U,A)")
    ap.add_argument("--wall", default="", help="JSON of per-arm accounting and wall clock")
    ap.add_argument("--out", required=True)
    ap.add_argument("--json", default="")
    args = ap.parse_args()

    groups = {}
    sources = []

    def add(path, tag):
        ev = load(path)
        sources.append({"path": path, "tag": tag, "revision": ev.get("revision"),
                        "live": ev.get("live"), "effort": ev.get("declared_effort"),
                        "model_id": ev.get("model_id"), "wire_mode": ev.get("wire_mode"),
                        "claim_contract": ev.get("claim_contract"),
                        "call_concurrency": ev.get("call_concurrency"),
                        "horizon_hours": ev.get("horizon_hours")})
        for c in cells_of(ev, tag):
            groups.setdefault(c["condition"], {}).setdefault(c["cell"], []).append(c)

    add(args.eval, "s")
    add(args.timing_eval, "t")

    for cond in sorted(groups):
        for cell in groups[cond]:
            groups[cond][cell].sort(key=lambda c: (c["tag"], c["replicate"]))

    cell = "f11-s2"
    conds = {}
    for arm in ("A0", "R", "P", "U", "A"):
        key = arm + "@max"
        if key not in groups or cell not in groups[key]:
            sys.exit("missing condition %s in cell %s" % (key, cell))
        reps = groups[key][cell]
        if len(reps) != 3:
            sys.exit("%s must carry 3 replicates, found %d" % (key, len(reps)))
        conds[arm] = reps
    ref_reps = conds["R"]

    # Stage T's f10-s0 conditions (the pattern being tested for reproduction).
    tcell = "f10-s0"
    conds_t = {}
    for arm in ("A0", "P", "U", "A"):
        key = arm + "@max"
        if key not in groups or tcell not in groups[key]:
            sys.exit("missing Stage-T condition %s in cell %s" % (key, tcell))
        conds_t[arm] = groups[key][tcell]

    results = {
        "reference": {
            "condition": "R@max", "cell": cell,
            "replicates": [{"tag": c["tag"], "calls": c["row"]["realized_calls"],
                            "prompt_tokens": c["row"]["prompt_tokens"],
                            "completion_tokens": c["row"]["completion_tokens"],
                            "hours": len(c["row"]["hours"])} for c in ref_reps],
            "endpoints": endpoints(ref_reps),
        },
        "conditions": {},
        "pairs": {},
        "distances_to_reference": {},
        "sources": sources,
    }
    for arm, reps in conds.items():
        results["conditions"][arm] = {
            "condition": arm + "@max", "cell": cell, "replicates": len(reps),
            "endpoints": endpoints(reps),
            "replicates_detail": [{
                "tag": c["tag"], "realized_calls": c["row"]["realized_calls"],
                "applied_model_decisions": c["row"].get("applied_model_decisions"),
                "call_ceiling": c["row"].get("call_ceiling"),
                "under_budget": c["row"].get("under_budget"),
                "halted": c["row"].get("halted"),
                "budget_shortfall": c["row"].get("budget_shortfall"),
                "decision_errors": c["row"].get("decision_errors"),
                "prompt_tokens": c["row"]["prompt_tokens"],
                "completion_tokens": c["row"]["completion_tokens"],
                "duration_ms": c["row"].get("duration_ms"),
                "hours": len(c["row"]["hours"]),
            } for c in reps],
        }

    # §3.2: the equal-budget claim comparison A vs P and A vs U at N = 256.
    for base in ("P", "U"):
        results["pairs"]["A_vs_" + base] = compare("A@max", conds["A"], base + "@max", conds[base])
    # §3.1: every arm's distance to the IN-RUN R@max reference (same cell/seeds).
    for arm in ("A", "P", "U", "A0"):
        results["distances_to_reference"][arm] = compare(arm + "@max", conds[arm], "R@max", ref_reps)
    # the A0 control against U: the Stage-T pattern said they are identical.
    results["pairs"]["A0_vs_U"] = compare("A0@max", conds["A0"], "U@max", conds["U"])

    results["curve_agreement"] = []
    named = {"A": conds["A"][0]["row"], "P": conds["P"][0]["row"],
             "U": conds["U"][0]["row"], "A0": conds["A0"][0]["row"],
             "R": ref_reps[0]["row"]}
    order = ["A", "P", "U", "A0", "R"]
    for i, a in enumerate(order):
        for b in order[i + 1:]:
            row = agreement_pair(named[a], named[b])
            row.update({"a": a, "b": b})
            results["curve_agreement"].append(row)

    results["stage_s_verdict"] = stage_verdict(results)
    pattern_s, pairs_s = pattern_of(conds)
    pattern_t, _ = pattern_of(conds_t)
    results["pattern_f10s0"] = pattern_t
    results["pattern_f11s2"] = pattern_s
    results["pattern_reproduction"] = {
        "f10s0": pattern_t["pattern_label"],
        "f11s2": pattern_s["pattern_label"],
        "same_per_baseline": pattern_s["per_baseline"] == pattern_t["per_baseline"],
        "same_u_a0_identity": pattern_s["U_vs_A0"]["identical"] == pattern_t["U_vs_A0"]["identical"],
        "outcome": reproduction_outcome(results, pattern_t, pattern_s),
    }
    results["cross_cell"] = cross_cell(conds_t, conds)

    if args.wall and os.path.exists(args.wall):
        results["wall"] = load(args.wall)

    render(results, args.out)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            json.dump(results, fh, indent=2, sort_keys=True)
            fh.write("\n")
    print("wrote %s" % args.out)
    print("f11-s2 verdict:", results["stage_s_verdict"]["label"])
    print("reproduction:", results["pattern_reproduction"]["outcome"])


def stage_verdict(results):
    """The §3.2 verdict of Stage S on f11-s2, applied literally."""
    detail = {b: baseline_outcome(results["pairs"]["A_vs_" + b]) for b in ("P", "U")}
    states = {b: resolution_state(results["pairs"]["A_vs_" + b]) for b in ("P", "U")}
    if detail["P"] == "beats" and detail["U"] == "beats":
        label = "supported at this cell"
        reason = ("A separates from both P and U in the declared (favourable) direction with no KPI "
                  "separating against it in cell f11-s2")
    elif detail["P"] == "worse" or detail["U"] == "worse":
        label = "not supported"
        worst = [b for b in ("P", "U") if detail[b] == "worse"]
        reason = "a baseline separates against A in at least one KPI (" + ", ".join(worst) + ")"
    elif states["P"] == "unresolvable" and states["U"] == "unresolvable":
        label = "unresolvable here"
        reason = ("no separation detected and every observed A-B difference is below the reported "
                  "detection limit")
    else:
        label = "not supported"
        reason = "A ties with at least one baseline (no detectable separation in either direction)"
    return {"label": label, "reason": reason, "per_baseline": detail,
            "resolution_state": states}


def reproduction_outcome(results, pattern_t, pattern_s):
    if results["stage_s_verdict"]["label"] == "unresolvable here":
        return "unresolved at this cell"
    if pattern_s["per_baseline"] != pattern_t["per_baseline"]:
        return "fails to reproduce"
    if pattern_s["U_vs_A0"]["identical"] != pattern_t["U_vs_A0"]["identical"]:
        return "fails to reproduce"
    return "reproduces"


def render(results, path):
    L = []
    w = L.append
    w("# Stage S analysis — stability on `f11-s2` (`A` vs `P`/`U` at N=256, distances to in-run `R@max`)\n")
    w("Rule (pre-registered, `.scratch/58-w3-preregistration/spec.md` §3, Stage S amended to one cell):")
    w("a scalar KPI separates when `SNR = RMSE(d)/noise >= 2` **and** `|d(t)|` exceeds the replicate")
    w("envelope for >= 24 consecutive hours. The histogram families are carried by Wasserstein-1 over")
    w("buckets computed here from the per-hour bucket vectors. No scalar weighting exists; a tie is")
    w("'no detectable effect at this design', never equivalence; no cell is pooled.\n")
    w("Reference: **in-run** `R@max`, cell `f11-s2`, 3 fresh replicates (%s). It is not the f10-s0"
      % ", ".join(r["tag"] for r in results["reference"]["replicates"]))
    w("reference; a second cell is a different world draw and is compared, never merged.\n")

    w("## Accounting\n")
    if "wall" in results:
        W = results["wall"]
        w("| arm | calls (3 reps) | prompt tok | completion tok | derived $ | cell seconds (sum of 3 reps) |")
        w("|---|---:|---:|---:|---:|---:|")
        for arm in ("A0", "R", "P", "U", "A"):
            a = W.get(arm)
            if not a:
                continue
            secs = sum(c["segment_duration_s"] for c in a["cells"])
            w("| %s | %d | %d | %d | %.4f | %.0f |"
              % (arm, a["calls"], a["prompt_tokens"], a["completion_tokens"],
                 a["derived_usd"], secs))
        if "fresh_total" in W:
            t = W["fresh_total"]
            w("| **fresh total (Stage S)** | **%d** | **%d** | **%d** | **%.4f** | stage wall %.1f min |"
              % (t["calls"], t["prompt_tokens"], t["completion_tokens"], t["derived_usd"],
                 W.get("stage_wall_seconds", 0) / 60))
        w("")
        w("Recorded charge is $0.00 (the gateway is unpriced); the derived price uses the ticket-35")
        w("rates ($0.10/M input, $0.50/M output), the same method as Stages E/T/M.\n")
    else:
        w("_wall.json absent; accounting not rendered._\n")

    w("## §3.2 — the equal-budget comparisons (`A` vs `P`, `A` vs `U`)\n")
    for base in ("P", "U"):
        p = results["pairs"]["A_vs_" + base]
        w("### `A@max` − `%s@max` (N = 256 both sides), cell `f11-s2`, 3 replicates each\n"
          % base)
        w("| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction (A−%s) | power (smallest separable) | max abs d | d at H | W1 |"
          % base)
        w("|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|")
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            c = p["kpis"][kpi]
            w("| %s | %s | %s | %s | %d | %d (%d) | %s | %s | %s | %s | %s |"
              % (kpi, fmt(c["rmse_d"]), fmt(c["noise"]), fmt(c["snr"]),
                 c["first_separation"], c["run_length"], c["run_start"],
                 c["separation_direction"] or "-",
                 fmt(c["power"]["smallest_separable"]), fmt(c["max_abs_d"]),
                 fmt(c["mean_d_at_H"]), fmt(c["wasserstein"])))
        for family in HISTS:
            w("| %s | – | – | – | – | – | – | – | – | – | %s |"
              % (family, fmt(p["kpis"][family]["wasserstein"])))
        w("")
        if p["separations"]:
            w("Separations: " + "; ".join(
                "%s %s (%s)" % (s["kpi"], s["direction"], s["quality"] or "no declared direction")
                for s in p["separations"]) + "\n")
        else:
            w("No KPI separates in either direction.\n")

    w("## §3.1 — each arm's distance to the in-run `R@max` (cell `f11-s2`, 3 replicates each)\n")
    for arm in ("A", "P", "U", "A0"):
        d = results["distances_to_reference"][arm]
        w("### `%s@max` − `R@max`\n" % arm)
        w("| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | d at H | W1 |")
        w("|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|")
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            c = d["kpis"][kpi]
            w("| %s | %s | %s | %s | %d | %d (%d) | %s | %s | %s | %s | %s |"
              % (kpi, fmt(c["rmse_d"]), fmt(c["noise"]), fmt(c["snr"]),
                 c["first_separation"], c["run_length"], c["run_start"],
                 c["separation_direction"] or "-",
                 fmt(c["power"]["smallest_separable"]), fmt(c["max_abs_d"]),
                 fmt(c["mean_d_at_H"]), fmt(c["wasserstein"])))
        for family in HISTS:
            w("| %s | – | – | – | – | – | – | – | – | – | %s |"
              % (family, fmt(d["kpis"][family]["wasserstein"])))
        if d["separations"]:
            w("\nSeparations: " + "; ".join(
                "%s %s (%s)" % (s["kpi"], s["direction"], s["quality"] or "no declared direction")
                for s in d["separations"]))
        else:
            w("\nNo KPI separates: `%s@max` matches `R@max` at this design." % arm)
        w("")

    w("## A0 control and the A0−U identity (descriptive)\n")
    a0u = results["pairs"]["A0_vs_U"]
    ident = next(r for r in results["curve_agreement"] if r["a"] in ("A0", "U") and r["b"] in ("A0", "U"))
    w("`A0` makes zero calls. At `f11-s2` its three replicates are identical on the scalars")
    w("(endpoints %s)." % ", ".join("%s=%s" % (k, fmt(v, 2)) for k, v in results["conditions"]["A0"]["endpoints"].items()))
    w("`U@max` vs `A0`: %d hours with any scalar difference (first %s, last %s); %s."
      % (ident["hours_any_scalar_differ"],
         ident["first_differ_hour"] if ident["first_differ_hour"] >= 0 else "-",
         ident["last_differ_hour"] if ident["last_differ_hour"] >= 0 else "-",
         "identical" if ident["identical"] else "not identical"))
    w("The paired A0−U rule, for completeness (both may be deterministic, so SNR is `n/a`):")
    w("no KPI separates" if not a0u["separations"] else
      "separations: " + "; ".join("%s %s" % (s["kpi"], s["direction"]) for s in a0u["separations"]))
    w("")

    w("## Curve agreement (descriptive: hours two conditions differ on ANY scalar)\n")
    w("No threshold and no verdict; it tells the reader whether a tie is between two distinct worlds")
    w("or two runs of the same world. First replicate of each condition.\n")
    w("| pair | hours with any scalar difference | first | last |")
    w("|---|---:|---:|---:|")
    for row in results["curve_agreement"]:
        w("| %s vs %s | %d | %s | %s |"
          % (row["a"], row["b"], row["hours_any_scalar_differ"],
             row["first_differ_hour"] if row["first_differ_hour"] >= 0 else "-",
             row["last_differ_hour"] if row["last_differ_hour"] >= 0 else "-"))
    w("")

    w("## Endpoints at H = 144 (mean over each condition's 3 replicates)\n")
    w("| condition | alive@H | births@H | starve@H | age@H | max_capital@H |")
    w("|---|---:|---:|---:|---:|---:|")
    rows = [("R@max (in-run)", results["reference"]["endpoints"])]
    for arm in ("A", "P", "U", "A0"):
        rows.insert(len(rows), ("%s@max" % arm, results["conditions"][arm]["endpoints"]))
    for label, ep in rows:
        w("| %s | %s | %s | %s | %s | %s |"
          % (label, fmt(ep["population"], 2), fmt(ep["births_cum"], 2),
             fmt(ep["deaths_starve_cum"], 2), fmt(ep["deaths_age_cum"], 2),
             fmt(ep["max_capital"], 2)))
    w("")

    v = results["stage_s_verdict"]
    w("## Stage-S verdict on `f11-s2` (§3.2)\n")
    w("**%s** — %s.\n" % (v["label"], v["reason"]))
    w("| baseline | A's outcome | resolution state |")
    w("|---|---|---|")
    for base in ("P", "U"):
        w("| %s | %s | %s |" % (base, v["per_baseline"][base], v["resolution_state"][base]))
    w("")
    r = results["pattern_reproduction"]
    w("## Reproduction of the Stage-T f10-s0 pattern\n")
    w("- `f10-s0` (Stage T, cited): **%s**" % r["f10s0"])
    w("- `f11-s2` (this cell): **%s**" % r["f11s2"])
    w("- **%s**" % r["outcome"])
    w("")
    w("## Cross-cell endpoints (descriptive; no pooling, no verdict)\n")
    w("| arm | f10-s0 alive/births/starve/age/maxcap | f11-s2 alive/births/starve/age/maxcap | f11-s2 − f10-s0 (vector) |")
    w("|---|---|---|---|")
    for arm in ("A0", "P", "U", "A"):
        c = results["cross_cell"][arm]
        fmt_ep = lambda e: "/".join(fmt(e[k], 1) for k in SCALARS)
        d = c["delta_s_minus_t"]
        w("| %s | %s | %s | %s |"
          % (arm, fmt_ep(c["f10-s0"]), fmt_ep(c["f11-s2"]), fmt_ep(d)))
    w("")
    w("The cross-cell table is a description of two different world draws, not a test: the declared")
    w("rule is per cell, and this stage declares one cell. The §3.2 'supported' category would")
    w("additionally need `A` to beat both baselines in every declared cell; the amended Stage S has")
    w("one cell, so a reproduction here answers the stability question, not M_end.")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(L) + "\n")


if __name__ == "__main__":
    main()
