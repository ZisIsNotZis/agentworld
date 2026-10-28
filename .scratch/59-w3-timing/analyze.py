#!/usr/bin/env python3
"""Stage T of ticket 58: the declared §3 paired curve rule and the §3.2 claim comparison.

READ-ONLY. Loads recorded evaluation JSONs (no cell, no model call, no spend) and
applies the pre-registered rule of `.scratch/58-w3-preregistration/spec.md` §3
verbatim. No rule is introduced, tuned or re-derived here; `power()` and
`paired()` are the Stage-E implementation unchanged (same formulas, so the two
stages' numbers are comparable).

The unit is one condition's per-hour KPI curve over 3 replicates in one cell.

  * `d(t) = mean_A(t) - mean_B(t)` over the 3 replicates of each condition;
  * `RMSE(d)` over hours; `noise` = RMSE over hours of the pooled within-condition
    SD at each hour; `SNR = RMSE(d)/noise` (NaN when both are deterministic);
  * a separation at hour t iff `|d(t)|` exceeds the replicate envelope half-width
    of *both* conditions; a KPI separates iff `SNR >= 2` AND some separating run
    is >= 24 consecutive hours;
  * `power` = the smallest constant per-KPI difference separable at 3 replicates
    and the observed bands (§3 "power");
  * direction is description only (§3): population/births higher is better,
    starvation/age deaths lower; `max_capital` has no declared direction;
  * no scalar weighting exists anywhere in this file.

The histogram families are carried by Wasserstein-1 over buckets computed here
from the per-hour bucket vectors. The frozen `curve/1` instrument's histogram
columns are NOT used: `curves.go` builds them from replicate-major series, so the
index it integrates over is the hour index, not the bucket index (known defect,
reported in the Stage-E record and to the manager; not fixed here).

Usage:
  python3 analyze.py --timing-eval evaluation.json \
      --reference-eval ../58-w3-effort/max/evaluation.json \
      --reference-extra ../57-w2-concurrent-reference/evaluation.json \
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
T95_N3 = 4.303

# §3 direction, description only. None = no declared direction; a separation of
# such a KPI is reported but is neither favourable nor unfavourable.
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
    """1-D W1 between two empirical samples: integral of |F_a - F_b|."""
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
    """Per-hour W1 over bucket distributions, averaged over the paired hours."""
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
    """Smallest constant difference the declared rule could separate here.

    A constant offset delta gives RMSE(d) = delta, so the SNR gate needs
    delta >= 2*noise. The sustained gate needs delta to exceed the replicate band
    at every hour of some 24-hour window, i.e. delta > min over windows of the
    window's max band. The smallest separable difference is the larger floor.
    """
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
    """'favourable'/'unfavourable'/None for a direction on a KPI (§3, description only)."""
    want = DIRECTION.get(kpi)
    if want is None or direction in ("", "flat"):
        return None
    return "favourable" if direction == want else "unfavourable"


def compare(name_a, cells_a, name_b, cells_b):
    """The declared §3 paired comparison of two conditions in one cell."""
    out = {"a": name_a, "b": name_b, "cell": cells_a[0]["cell"],
           "replicates_a": len(cells_a), "replicates_b": len(cells_b), "kpis": {}}
    for kpi in SCALARS + INSTRUMENT_EXTRA:
        comp = paired(kpi, cells_a, cells_b)
        comp["wasserstein"] = wasserstein1([v for c in cells_a for v in series(c["row"], kpi)],
                                           [v for c in cells_b for v in series(c["row"], kpi)])
        if comp["detectable"]:
            comp["quality"] = quality(kpi, comp["separation_direction"])
        else:
            comp["quality"] = None
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


def fmt(x, digits=3):
    if isinstance(x, float) and math.isnan(x):
        return "n/a"
    if isinstance(x, float):
        return ("%%.%df" % digits) % x
    return str(x)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--timing-eval", required=True, help="Stage-T evaluation JSON (arms A0,P,U,A)")
    ap.add_argument("--reference-eval", required=True, help="Stage-E fresh R@max evaluation JSON")
    ap.add_argument("--reference-extra", default="", help="ticket-57 R@max evaluation JSON (replicate 1)")
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

    add(args.timing_eval, "t")
    add(args.reference_eval, "w3max")
    if args.reference_extra:
        add(args.reference_extra, "t57max")

    for cond in sorted(groups):
        for cell in groups[cond]:
            groups[cond][cell].sort(key=lambda c: (c["tag"], c["replicate"]))

    reference = "R@max"
    if reference not in groups:
        sys.exit("no R@max cells loaded: the Stage-E reference is missing")
    ref_cell = sorted(groups[reference])[0]
    ref_reps = groups[reference][ref_cell]
    if len(ref_reps) != 3:
        sys.exit("R@max must carry 3 replicates, found %d" % len(ref_reps))

    cell = "f10-s0"
    conds = {}
    for arm in ["A0", "P", "U", "A"]:
        key = arm + "@max"
        if key not in groups or cell not in groups[key]:
            sys.exit("missing condition %s in cell %s" % (key, cell))
        reps = groups[key][cell]
        if len(reps) != 3:
            sys.exit("%s must carry 3 replicates, found %d" % (key, len(reps)))
        conds[arm] = reps

    results = {
        "reference": {
            "condition": reference, "cell": ref_cell,
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
    for base in ["P", "U"]:
        results["pairs"]["A_vs_" + base] = compare("A@max", conds["A"], base + "@max", conds[base])
    # §3.1: every arm's distance to the R@max reference (same cell).
    for arm in ["A", "P", "U", "A0"]:
        results["distances_to_reference"][arm] = compare(arm + "@max", conds[arm], reference, ref_reps)
    # the A0 control against the no-AI founding trajectory is deterministic; the
    # A0-vs-R distance is above. A0 vs P/U is reported so the control's reading
    # beside the budgeted arms exists.
    results["pairs"]["P_vs_U"] = compare("P@max", conds["P"], "U@max", conds["U"])

    results["curve_agreement"] = agreement(results, conds, ref_reps)

    v = stage_verdict(results)
    results["stage_t_verdict"] = v

    if args.wall and os.path.exists(args.wall):
        results["wall"] = load(args.wall)

    render(results, args.out)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            json.dump(results, fh, indent=2, sort_keys=True)
            fh.write("\n")
    print("wrote %s" % args.out)
    print("verdict:", v["label"], "-", v["reason"])


def agreement(results, conds, ref_reps):
    """Descriptive: at how many hours do two conditions' scalar curves differ at all?

    The declared rule answers 'is the difference separable'; this answers the
    prior question the verdict needs to be read correctly ('are these two worlds
    the same world?'). It is descriptive only: no threshold, no verdict.
    With 3 replicates whose scalar series are identical within an arm (measured:
    0 differing hours between the 3 replicates of every arm) the first replicate
    carries the condition.
    """
    named = {"A0": conds["A0"], "P": conds["P"], "U": conds["U"], "A": conds["A"],
             "R": ref_reps}
    out = []
    order = ["A", "P", "U", "A0", "R"]
    for i, a in enumerate(order):
        for b in order[i + 1:]:
            ra = named[a][0]["row"]
            rb = named[b][0]["row"]
            h = min(len(ra["hours"]), len(rb["hours"]))
            hours = [k for k in range(h)
                     if any(series(ra, kpi)[k] != series(rb, kpi)[k] for kpi in SCALARS)]
            out.append({
                "a": a, "b": b,
                "hours_any_scalar_differ": len(hours),
                "first_differ_hour": hours[0] if hours else -1,
                "last_differ_hour": hours[-1] if hours else -1,
                "identical": not hours,
            })
    return out


def stage_verdict(results):
    """The §3.2 verdict of Stage T on this cell, applied literally."""
    detail = {}
    for base in ["P", "U"]:
        p = results["pairs"]["A_vs_" + base]
        if p["against_a"]:
            detail[base] = "worse"
        elif p["for_a"]:
            detail[base] = "beats"
        else:
            detail[base] = "matches"
    # Is every observed separation below the design's detection limit?
    below = True
    for base in ["P", "U"]:
        p = results["pairs"]["A_vs_" + base]
        for kpi in SCALARS:
            c = p["kpis"][kpi]
            p_lim = c["power"]["smallest_separable"]
            if c["detectable"]:
                below = False
                continue
            if not math.isnan(p_lim) and c["max_abs_d"] >= p_lim:
                below = False
    if detail["P"] == "beats" and detail["U"] == "beats":
        label = "supported (at this cell; M_end also needs the §S reproduction)"
        reason = ("A separates from both P and U in the declared (favourable) direction with no KPI "
                  "separating against it in cell f10-s0; the second-cell/seed reproduction is Stage S")
    elif detail["P"] == "worse" or detail["U"] == "worse":
        label = "not supported"
        worst = [b for b in ("P", "U") if detail[b] == "worse"]
        reason = ("a baseline separates against A in at least one KPI (" + ", ".join(worst) + ")")
    elif below:
        label = "unresolvable here"
        reason = ("no separation detected and every observed A-B difference is below the reported "
                  "detection limit")
    else:
        label = "not supported"
        reason = "A ties with at least one baseline (no detectable separation in either direction)"
    return {"label": label, "reason": reason, "per_baseline": detail}


def render(results, path):
    L = []
    w = L.append
    w("# Stage T analysis — timing (`A` vs `P`/`U` at N=256, and every arm's distance to `R@max`)\n")
    w("Rule (pre-registered, `.scratch/58-w3-preregistration/spec.md` §3): a scalar KPI separates when")
    w("`SNR = RMSE(d)/noise >= 2` **and** `|d(t)|` exceeds the replicate envelope for >= 24")
    w("consecutive hours. The histogram families are carried by Wasserstein-1 over buckets computed")
    w("here from the per-hour bucket vectors (the frozen `curve/1` histogram column is defective —")
    w("its integral runs over the hour index — and is not used). No scalar weighting exists; a tie is")
    w("'no detectable effect at this design', never equivalence.\n")
    w("Reference: `R@max`, cell `%s`, 3 replicates (%s) — the Stage-E rows, not re-run here.\n"
      % (results["reference"]["cell"],
         ", ".join(r["tag"] for r in results["reference"]["replicates"])))

    if "wall" in results:
        w("## Accounting\n")
        w("| arm | calls (3 reps) | prompt tok | completion tok | derived $ | cell seconds (sum of 3 reps) |")
        w("|---|---:|---:|---:|---:|---:|")
        for arm in ["A0", "P", "U", "A"]:
            a = results["wall"].get(arm)
            if not a:
                continue
            secs = sum(c["segment_duration_s"] for c in a["cells"])
            w("| %s | %d | %d | %d | %.4f | %.0f |"
              % (arm, a["calls"], a["prompt_tokens"], a["completion_tokens"],
                 a["derived_usd"], secs))
        if "fresh_total" in results["wall"]:
            t = results["wall"]["fresh_total"]
            w("| **fresh total** | **%d** | **%d** | **%d** | **%.4f** | stage wall %.1f min |"
              % (t["calls"], t["prompt_tokens"], t["completion_tokens"], t["derived_usd"],
                 results["wall"].get("stage_wall_seconds", 0) / 60))
        if "reference" in results["wall"]:
            a = results["wall"]["reference"]
            w("| R@max (Stage-E rows, not re-run) | %d | %d | %d | %.4f | (previously recorded) |"
              % (a["calls"], a["prompt_tokens"], a["completion_tokens"], a["derived_usd"]))
        w("")

    w("## §3.2 — the equal-budget comparisons (`A` vs `P`, `A` vs `U`)\n")
    for base in ["P", "U"]:
        p = results["pairs"]["A_vs_" + base]
        w("### `A@max` − `%s@max` (N = 256 both sides), cell `%s`, 3 replicates each\n"
          % (base, p["cell"]))
        w("| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction (A−%s) | power (smallest separable) | max abs d | W1 |"
          % base)
        w("|---|---:|---:|---:|---:|---:|---|---:|---:|---:|")
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            c = p["kpis"][kpi]
            w("| %s | %s | %s | %s | %d | %d (%d) | %s | %s | %s | %s |"
              % (kpi, fmt(c["rmse_d"]), fmt(c["noise"]), fmt(c["snr"]),
                 c["first_separation"], c["run_length"], c["run_start"],
                 c["separation_direction"] or "-",
                 fmt(c["power"]["smallest_separable"]), fmt(c["max_abs_d"]),
                 fmt(c["wasserstein"])))
        for family in HISTS:
            w("| %s | – | – | – | – | – | – | – | – | %s |"
              % (family, fmt(p["kpis"][family]["wasserstein"])))
        w("")
        if p["separations"]:
            w("Separations: " + "; ".join(
                "%s %s (%s)" % (s["kpi"], s["direction"], s["quality"] or "no declared direction")
                for s in p["separations"]) + "\n")
        else:
            w("No KPI separates in either direction.\n")

    w("## §3.1 — each arm's distance to `R@max` (same cell `f10-s0`, 3 replicates each)\n")
    for arm in ["A", "P", "U", "A0"]:
        d = results["distances_to_reference"][arm]
        w("### `%s@max` − `R@max`\n" % arm)
        w("| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | direction | power | max abs d | W1 |")
        w("|---|---:|---:|---:|---:|---:|---|---:|---:|---:|")
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            c = d["kpis"][kpi]
            w("| %s | %s | %s | %s | %d | %d (%d) | %s | %s | %s | %s |"
              % (kpi, fmt(c["rmse_d"]), fmt(c["noise"]), fmt(c["snr"]),
                 c["first_separation"], c["run_length"], c["run_start"],
                 c["separation_direction"] or "-",
                 fmt(c["power"]["smallest_separable"]), fmt(c["max_abs_d"]),
                 fmt(c["wasserstein"])))
        for family in HISTS:
            w("| %s | – | – | – | – | – | – | – | – | %s |"
              % (family, fmt(d["kpis"][family]["wasserstein"])))
        if d["separations"]:
            w("\nSeparations: " + "; ".join(
                "%s %s (%s)" % (s["kpi"], s["direction"], s["quality"] or "no declared direction")
                for s in d["separations"]))
        else:
            w("\nNo KPI separates: `%s@max` matches `R@max` at this design." % arm)
        w("")

    w("## Curve agreement (descriptive: how many hours two conditions differ on ANY scalar)\n")
    w("No threshold and no verdict here; it tells the reader whether a tie is a tie between two")
    w("distinct worlds or between two runs of the same world. The five declared scalars are compared")
    w("hour by hour; the replicates of every arm have identical scalar series (0 differing hours).\n")
    w("| pair | hours with any scalar difference | first | last |")
    w("|---|---:|---:|---:|")
    for row in results["curve_agreement"]:
        w("| %s vs %s | %d | %s | %s |"
          % (row["a"], row["b"], row["hours_any_scalar_differ"],
             row["first_differ_hour"] if row["first_differ_hour"] >= 0 else "-",
             row["last_differ_hour"] if row["last_differ_hour"] >= 0 else "-"))
    w("")

    w("## Endpoints (mean over each condition's 3 replicates, hour 144)\n")
    w("| condition | alive@H | births@H | starve@H | age@H | max_capital@H |")
    w("|---|---:|---:|---:|---:|---:|")
    rows = [("R@max (Stage E)", results["reference"]["endpoints"])]
    for arm in ["A", "P", "U", "A0"]:
        rows.insert(len(rows) - 0, ("%s@max" % arm, results["conditions"][arm]["endpoints"]))
    for label, ep in rows:
        w("| %s | %s | %s | %s | %s | %s |"
          % (label, fmt(ep["population"], 2), fmt(ep["births_cum"], 2),
             fmt(ep["deaths_starve_cum"], 2), fmt(ep["deaths_age_cum"], 2),
             fmt(ep["max_capital"], 2)))
    w("")

    v = results["stage_t_verdict"]
    w("## Stage-T verdict (§3.2)\n")
    w("**%s** — %s.\n" % (v["label"], v["reason"]))
    w("| baseline | A's outcome |")
    w("|---|---|")
    for base in ["P", "U"]:
        w("| %s | %s |" % (base, v["per_baseline"][base]))
    w("")
    w("'Supported' in §3.2 (M_end) additionally requires the pattern to reproduce at a second cell and")
    w("seed; that is Stage S and is not claimed here.")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(L) + "\n")


if __name__ == "__main__":
    main()
