#!/usr/bin/env python3
"""Stage E of ticket 58: the declared §3 paired curve rule, R@<effort> vs R@max.

READ-ONLY. Loads recorded evaluation JSONs (no cell, no model call, no spend),
groups their cell rows by condition (`<arm>@<effort>`) and by world cell
(`f<founders>-s<seed>`), and computes for every non-reference effort:

  * the paired difference d(t) = A(t) - B(t) over the 3 replicates,
  * RMSE(d), the replicate-noise RMSE and SNR = RMSE(d)/noise (the frozen
    ticket-35 `curve/1` formula, re-implemented here verbatim so the numbers are
    reproducible without the Go binary),
  * the first separating hour and the longest separating run, where an hour
    separates when |d(t)| exceeds the replicate envelope of both conditions,
  * Wasserstein-1: pooled samples for scalar KPIs, per-hour bucket distributions
    averaged over hours for the histogram families,
  * the power of the design: the smallest constant per-KPI difference the
    declared rule could separate at the observed bands and the observed
    replicate noise,
  * the direction of a separation (description only, §3; never a weight).

No scalar is computed. A tie is reported as "no detectable effect at this
design", never as equivalence.

The sealed rule is applied to the five scalar KPIs of §3. `wages_cum` is carried
by the `curve/1` instrument but is not one of §3's declared scalars; it is shown
as instrument-only and never enters a verdict. The histogram families are not
scalars: they are carried by Wasserstein-1 over buckets, as §3 declares.

Usage:
  python3 analyze.py --max-eval <fresh-max-evaluation.json> \
      --max-extra <ticket-57-evaluation.json> \
      --effort none=<eval.json> low=<eval.json> ... \
      --wall wall.json --out analysis.md --json analysis.json
"""
import argparse
import json
import math
import os
import sys

SCALARS = ["population", "births_cum", "deaths_starve_cum", "deaths_age_cum", "max_capital"]
INSTRUMENT_EXTRA = ["wages_cum"]  # carried by curve/1 but not declared in ticket-58 §3
HISTS = ["capital_hist", "granary_hist"]
SNR_THRESHOLD = 2.0
SUSTAINED_HOURS = 24
T95_N3 = 4.303

# The declared directions of §3 (description only, not a weight). None means the
# KPI has no declared direction and a separation is reported without one.
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
    """Bucket-index sample of one hourly bucket vector (normalised distribution)."""
    out = []
    for bucket, count in enumerate(rows):
        n = int(round(count))
        out.extend([float(bucket)] * max(0, n))
    return out or [0.0]


def hist_w1(a_rows, b_rows):
    """Per-hour W1 over bucket distributions, averaged over the paired hours.

    Each condition's hourly vector is the mean bucket count over its 3
    replicates, normalised to a distribution over bucket indices; W1 is computed
    in bucket units each hour and averaged over hours (ticket-35 §3 definition).
    """
    h = min(len(a_rows), len(b_rows))
    if h == 0:
        return float("nan")
    return sum(wasserstein1(hist_samples(a_rows[i]), hist_samples(b_rows[i]))
               for i in range(h)) / h


def series(cell, kpi):
    if kpi in ARRAY_KEYS:
        return [float(v) for v in cell[ARRAY_KEYS[kpi]]]
    raise KeyError(kpi)


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
            "tag": "%s.r%d" % (source_tag, row["replicate"]),
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


def paired(cell, kpi, a_cells, b_cells):
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


def power(comp):
    """Smallest constant difference the declared rule could separate here.

    With a constant offset delta the paired difference is delta every hour, so
    RMSE(d) = delta and the SNR gate needs delta >= 2*noise. The sustained gate
    needs delta to exceed the replicate band at every hour of some window of 24
    consecutive hours, i.e. delta > min over windows of the window's max band.
    The smallest separable difference is the larger of the two floors.
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


def fmt(x, digits=3):
    if isinstance(x, float) and math.isnan(x):
        return "n/a"
    if isinstance(x, float):
        return ("%%.%df" % digits) % x
    return str(x)


def favourable(kpi, direction):
    want = DIRECTION.get(kpi)
    if want is None or direction in ("", "flat"):
        return None
    return direction == want


def verdict_of(kpis):
    separated = []
    for kpi in SCALARS:
        comp = kpis[kpi]
        if comp.get("detectable"):
            separated.append({
                "kpi": kpi,
                "direction": comp["separation_direction"],
                "declared": DIRECTION.get(kpi),
                "favourable": favourable(kpi, comp["separation_direction"]),
            })
    return {
        "differs": bool(separated),
        "driving_kpis": [s["kpi"] for s in separated],
        "separations": separated,
        "statement": ("differs from R@max on " + ", ".join(s["kpi"] for s in separated)
                      if separated else
                      "no detectable effect vs R@max at this design"),
    }


def reference_band(ref_reps):
    out = {}
    for kpi in SCALARS:
        rows = [series(c["row"], kpi) for c in ref_reps]
        h = min(len(r) for r in rows)
        mean_h, half_h, env = [], [], []
        for i in range(h):
            vals = [r[i] for r in rows]
            m, sd = mean_sd(vals)
            half = T95_N3 * sd / math.sqrt(len(vals)) if len(vals) > 1 else 0.0
            mean_h.append(m)
            half_h.append(half)
            env.append((min(vals), max(vals)))
        out[kpi] = {
            "mean_at_H": mean_h[-1], "ci_half_at_H": half_h[-1],
            "envelope_at_H": env[-1],
            "mean_envelope_half_width": sum(
                max(hi - m, m - lo) for (lo, hi), m in zip(env, mean_h)) / h,
            "max_envelope_half_width": max(
                max(hi - m, m - lo) for (lo, hi), m in zip(env, mean_h)),
        }
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--max-eval", required=True, help="fresh R@max evaluation JSON")
    ap.add_argument("--max-extra", default="", help="ticket-57 R@max evaluation JSON (replicate 1)")
    ap.add_argument("--effort", action="append", default=[],
                    help="name=<evaluation.json>, repeatable; every non-max effort")
    ap.add_argument("--wall", default="", help="JSON of per-effort accounting and wall clock")
    ap.add_argument("--out", required=True)
    ap.add_argument("--json", default="")
    args = ap.parse_args()

    groups = {}
    sources = []

    def add(path, tag):
        ev = load(path)
        sources.append({"path": path, "tag": tag, "revision": ev.get("revision"),
                        "live": ev.get("live"), "effort": ev.get("declared_effort")})
        for c in cells_of(ev, tag):
            groups.setdefault(c["condition"], {}).setdefault(c["cell"], []).append(c)

    add(args.max_eval, "w3max")
    if args.max_extra:
        add(args.max_extra, "t57max")
    for spec in args.effort:
        name, _, path = spec.partition("=")
        if not path:
            sys.exit("bad --effort %r, want name=path" % spec)
        add(path, "w3" + name)

    reference = "R@max"
    if reference not in groups:
        sys.exit("no R@max cells loaded")
    ref_cell = sorted(groups[reference])[0]
    ref_reps = sorted(groups[reference][ref_cell], key=lambda c: (c["tag"], c["replicate"]))
    if len(ref_reps) != 3:
        sys.exit("R@max must carry 3 replicates, found %d" % len(ref_reps))

    results = {
        "reference": {
            "condition": reference, "cell": ref_cell,
            "replicates": [{"tag": c["tag"], "calls": c["row"]["realized_calls"],
                            "prompt_tokens": c["row"]["prompt_tokens"],
                            "completion_tokens": c["row"]["completion_tokens"]}
                           for c in ref_reps],
            "band": reference_band(ref_reps),
        },
        "efforts": {},
        "sources": sources,
    }

    for effort in ["none", "low", "medium", "high"]:
        condition = "R@" + effort
        if condition not in groups:
            continue
        cell = sorted(groups[condition])[0]
        cand_reps = sorted(groups[condition][cell], key=lambda c: c["replicate"])
        out = {"condition": condition, "cell": cell, "replicates": len(cand_reps), "kpis": {}}
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            out["kpis"][kpi] = paired(cell, kpi, cand_reps, ref_reps)
            out["kpis"][kpi]["wasserstein"] = wasserstein1(
                [v for c in cand_reps for v in series(c["row"], kpi)],
                [v for c in ref_reps for v in series(c["row"], kpi)])
        a_hist = {f: avg_rows(cand_reps, f) for f in HISTS}
        b_hist = {f: avg_rows(ref_reps, f) for f in HISTS}
        for family in HISTS:
            out["kpis"][family] = {
                "kpi": family, "kind": "histogram",
                "wasserstein": hist_w1(a_hist[family], b_hist[family]),
            }
        out["verdict"] = verdict_of(out["kpis"])
        results["efforts"][effort] = out

    if args.wall and os.path.exists(args.wall):
        results["wall"] = load(args.wall)

    render(results, args.out)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            json.dump(results, fh, indent=2, sort_keys=True)
            fh.write("\n")
    print("wrote %s" % args.out)


def render(results, path):
    L = []
    w = L.append
    w("# Stage E analysis — thinking (`-effort`) against `R@max` (ticket 58 §3)\n")
    w("Rule (pre-registered): a scalar KPI separates when `SNR = RMSE(d)/noise >= 2` **and**")
    w("`|d(t)|` exceeds the replicate envelope for >= 24 consecutive hours. The histogram families")
    w("are carried by Wasserstein-1 over buckets. Every KPI is reported; no scalar is defined.")
    w("A tie is 'no detectable effect at this design', never equivalence.\n")
    w("Reference: `R@max`, cell `%s`, 3 replicates (%s).\n"
      % (results["reference"]["cell"],
         ", ".join(r["tag"] for r in results["reference"]["replicates"])))
    w("Candidates: `R@<effort>` in the same cell, 3 fresh replicates each.\n")

    if "wall" in results:
        w("## Accounting\n")
        w("| effort | calls | prompt tok | completion tok | derived $ | wall (s) | wall (min) |")
        w("|---|---:|---:|---:|---:|---:|---:|")
        for effort in ["none", "low", "medium", "high", "max"]:
            a = results["wall"].get(effort)
            if not a:
                continue
            w("| %s | %d | %d | %d | %.2f | %.0f | %.1f |"
              % (effort, a["calls"], a["prompt_tokens"], a["completion_tokens"],
                 a["derived_usd"], a["wall_seconds"], a["wall_seconds"] / 60))
        w("")

    w("## Per-effort KPI comparisons (effort − max)\n")
    for effort in ["none", "low", "medium", "high"]:
        out = results["efforts"].get(effort)
        if not out:
            continue
        w("### `R@%s` vs `R@max`\n" % effort)
        w("| KPI | RMSE(d) | noise | SNR | first sep h | longest run (start) | dir | smallest separable | band floor | 2×noise | W1 |")
        w("|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|")
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            c = out["kpis"][kpi]
            p = c["power"]
            w("| %s | %s | %s | %s | %d | %d (%d) | %s | %s | %s | %s | %s |"
              % (kpi, fmt(c["rmse_d"]), fmt(c["noise"]), fmt(c["snr"]),
                 c["first_separation"], c["run_length"], c["run_start"],
                 c["separation_direction"] or "-",
                 fmt(p["smallest_separable"]), fmt(p["band_floor"]), fmt(p["noise_floor"]),
                 fmt(c["wasserstein"])))
        for family in HISTS:
            w("| %s | – | – | – | – | – | – | – | – | – | %s |"
              % (family, fmt(out["kpis"][family]["wasserstein"])))
        w("\n**Verdict `R@%s`:** %s\n" % (effort, out["verdict"]["statement"]))
        if out["verdict"]["differs"]:
            for s in out["verdict"].get("separations", []):
                w("- %s: %s (declared better: %s; %s)" % (
                    s["kpi"], s["direction"], s["declared"],
                    "favourable" if s["favourable"] else
                    "no declared direction" if s["favourable"] is None else "unfavourable"))
            w("")
        w("| KPI | mean d at H | observed max |d| | smallest separable | reading |")
        w("|---|---:|---:|---:|---|")
        for kpi in SCALARS:
            c = out["kpis"][kpi]
            p = c["power"]["smallest_separable"]
            if c["max_abs_d"] == 0:
                reading = "no difference observed"
            elif math.isnan(p):
                reading = "not evaluable (no noise estimate)"
            elif p == 0:
                reading = "any nonzero difference would separate"
            elif abs(c["max_abs_d"]) < p:
                reading = "observed max |d| below the smallest separable delta"
            else:
                reading = "observed max |d| at/above the smallest separable delta; the paired rule did not separate (SNR/run length)"
            w("| %s | %s | %s | %s | %s |" % (kpi, fmt(c["mean_d_at_H"]),
                                              fmt(c["max_abs_d"]), fmt(p), reading))
        w("")

    w("## Reference band (`R@max`, 3 replicates)\n")
    w("| KPI | mean@H | 95% CI half-width@H | envelope@H | mean envelope half-width | max envelope half-width |")
    w("|---|---:|---:|---|---:|---:|")
    for kpi in SCALARS:
        b = results["reference"]["band"][kpi]
        w("| %s | %s | %s | %s..%s | %s | %s |" % (
            kpi, fmt(b["mean_at_H"]), fmt(b["ci_half_at_H"]),
            fmt(b["envelope_at_H"][0]), fmt(b["envelope_at_H"][1]),
            fmt(b["mean_envelope_half_width"]), fmt(b["max_envelope_half_width"])))
    w("")

    w("## What the data establishes\n")
    ties = [e for e in ["none", "low", "medium", "high"]
            if e in results["efforts"] and not results["efforts"][e]["verdict"]["differs"]]
    diffs = [e for e in ["none", "low", "medium", "high"]
             if e in results["efforts"] and results["efforts"][e]["verdict"]["differs"]]
    if diffs:
        w("Detectable separation from `R@max`: %s." % ", ".join("`R@%s`" % e for e in diffs))
    else:
        w("No effort separates from `R@max` on any declared scalar KPI at this design.")
    if ties:
        w("No detectable effect at this design: %s (never equivalence)."
          % ", ".join("`R@%s`" % e for e in ties))
    w("The limit per effort is the smallest-separable column above; a difference below it is not")
    w("evidence. The histogram distances are reported, not tested by the 24-hour scalar rule.")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(L) + "\n")


if __name__ == "__main__":
    main()
