#!/usr/bin/env python3
"""Stage M of ticket 58: each non-reference model's declared §3.1 distance to `R@max`.

READ-ONLY. Loads recorded evaluation JSONs (no cell, no model call, no spend) and
applies the pre-registered rule of `.scratch/58-w3-preregistration/spec.md` §3 and
§3.1 verbatim. No rule is introduced, tuned or re-derived here: `paired()`,
`power()`, `wasserstein1()`, `hist_w1()` and the separation gate are the Stage-E/T
implementations unchanged, so the three stages' numbers are directly comparable.

  * `d(t) = mean_A(t) - mean_B(t)` over each condition's 3 replicates, per hour;
  * `RMSE(d)` over hours; `noise` = RMSE over hours of the pooled within-condition
    SD at each hour; `SNR = RMSE(d)/noise` (n/a when both conditions are
    deterministic);
  * a separation at hour t iff `|d(t)|` exceeds the replicate envelope half-width
    of *both* conditions; a KPI separates iff `SNR >= 2` AND some separating run
    is >= 24 consecutive hours;
  * `power` = the smallest constant per-KPI difference separable at 3 replicates
    and the observed bands (§3 "power");
  * direction is description only; no scalar weighting exists anywhere here and
    no model is ranked (§3 "No scalar", §4 "Which model is best is not claimed").

The histogram families are carried by Wasserstein-1 over buckets computed here
from the per-hour bucket vectors (independent of the frozen instrument's column).

Usage:
  python3 analyze.py --model-eval sol=<path> \
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
# the pre-registration's short names (§4 Stage M), so a condition reads `luna@max`.
SHORT_NAMES = {"github_copilot/gpt-6-luna": "luna", "github_copilot/gpt-6-sol": "sol",
               "github_copilot/grok-4.7": "grok-4.7"}

# §3 direction, description only. None = no declared direction.
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


def model_short(ev):
    """The record's short name for a model id (the pre-registration's vocabulary)."""
    mid = ev.get("model_id") or "unknown"
    return SHORT_NAMES.get(mid, mid.split("/")[-1])


def cells_of(evaluation, source_tag, model_tag):
    out = []
    for row in evaluation.get("cells", []):
        if not row.get("ran") or not row.get("hours"):
            continue
        effort = row.get("effort") or "model-default"
        out.append({
            "condition": "%s@%s" % (model_tag, effort),
            "model": model_tag,
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


def compare(name_a, cells_a, name_b, cells_b):
    out = {"a": name_a, "b": name_b, "cell": cells_a[0]["cell"],
           "replicates_a": len(cells_a), "replicates_b": len(cells_b), "kpis": {}}
    for kpi in SCALARS + INSTRUMENT_EXTRA:
        comp = paired(kpi, cells_a, cells_b)
        comp["wasserstein"] = wasserstein1([v for c in cells_a for v in series(c["row"], kpi)],
                                           [v for c in cells_b for v in series(c["row"], kpi)])
        comp["direction"] = DIRECTION.get(kpi)
        out["kpis"][kpi] = comp
    for family in HISTS:
        out["kpis"][family] = {
            "kpi": family, "kind": "histogram",
            "wasserstein": hist_w1(avg_rows(cells_a, family), avg_rows(cells_b, family)),
        }
    out["separations"] = [{"kpi": k, "direction": out["kpis"][k]["separation_direction"]}
                          for k in SCALARS if out["kpis"][k]["detectable"]]
    return out


def endpoints(cells, kpis=SCALARS):
    out = {}
    for kpi in kpis:
        rows = [series(c["row"], kpi) for c in cells]
        out[kpi] = sum(r[-1] for r in rows) / len(rows)
    return out


def agreement(a_cells, b_cells):
    """Descriptive: at how many hours do two conditions' scalars differ at all?"""
    ra, rb = a_cells[0]["row"], b_cells[0]["row"]
    h = min(len(ra["hours"]), len(rb["hours"]))
    hours = [k for k in range(h)
             if any(series(ra, kpi)[k] != series(rb, kpi)[k] for kpi in SCALARS)]
    return {"hours_any_scalar_differ": len(hours),
            "first_differ_hour": hours[0] if hours else -1,
            "last_differ_hour": hours[-1] if hours else -1,
            "identical": not hours}


def fmt(x, digits=3):
    if isinstance(x, float) and math.isnan(x):
        return "n/a"
    if isinstance(x, float):
        return ("%%.%df" % digits) % x
    return str(x)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model-eval", action="append", required=True,
                    help="<name>=<evaluation.json> for a non-reference model")
    ap.add_argument("--reference-eval", required=True, help="Stage-E fresh R@max evaluation JSON")
    ap.add_argument("--reference-extra", default="", help="ticket-57 R@max evaluation JSON")
    ap.add_argument("--accounting", default="", help="JSON of per-model accounting")
    ap.add_argument("--agreement", action="append", default=[],
                    help="<name>=<agreement.json> produced by summarize.py")
    ap.add_argument("--out", required=True)
    ap.add_argument("--json", default="")
    args = ap.parse_args()

    groups = {}
    sources = []

    def add(path, tag):
        ev = load(path)
        mt = model_short(ev)
        sources.append({"path": path, "tag": tag, "model": mt,
                        "model_id": ev.get("model_id"), "revision": ev.get("revision"),
                        "live": ev.get("live"), "effort": ev.get("declared_effort"),
                        "wire_mode": ev.get("wire_mode"),
                        "claim_contract": ev.get("claim_contract"),
                        "call_concurrency": ev.get("call_concurrency"),
                        "horizon_hours": ev.get("horizon_hours")})
        for c in cells_of(ev, tag, mt):
            groups.setdefault(c["condition"], {}).setdefault(c["cell"], []).append(c)

    add(args.reference_eval, "w3max")
    if args.reference_extra:
        add(args.reference_extra, "t57max")
    model_paths = {}
    for spec in args.model_eval:
        name, path = spec.split("=", 1)
        model_paths[name] = path
        add(path, "m" + name)

    for cond in groups:
        for cell in groups[cond]:
            groups[cond][cell].sort(key=lambda c: (c["tag"], c["replicate"]))

    ref_cond = "luna@max"
    cell = "f10-s0"
    if ref_cond not in groups:
        sys.exit("no luna@max reference loaded (conditions: %s)" % sorted(groups))
    ref_reps = groups[ref_cond][cell]
    if len(ref_reps) != 3:
        sys.exit("the luna reference must carry 3 replicates, found %d" % len(ref_reps))

    results = {
        "reference": {
            "condition": ref_cond, "cell": cell,
            "replicates": [{"tag": c["tag"], "calls": c["row"]["realized_calls"],
                            "prompt_tokens": c["row"]["prompt_tokens"],
                            "completion_tokens": c["row"]["completion_tokens"],
                            "hours": len(c["row"]["hours"])} for c in ref_reps],
            "endpoints": endpoints(ref_reps),
        },
        "models": {}, "distances_to_reference": {}, "sources": sources,
    }
    for name in model_paths:
        ev = load(model_paths[name])
        mt = model_short(ev)
        cond = "%s@max" % mt
        if cond not in groups or cell not in groups[cond]:
            sys.exit("missing condition %s in cell %s" % (cond, cell))
        reps = groups[cond][cell]
        if len(reps) != 3:
            sys.exit("%s must carry 3 replicates, found %d" % (cond, len(reps)))
        results["models"][name] = {
            "condition": cond, "cell": cell, "replicates": len(reps),
            "model_id": ev.get("model_id"), "wire_mode": ev.get("wire_mode"),
            "endpoints": endpoints(reps),
            "replicates_detail": [{
                "tag": c["tag"], "realized_calls": c["row"]["realized_calls"],
                "applied_model_decisions": c["row"].get("applied_model_decisions"),
                "decision_errors": c["row"].get("decision_errors"),
                "halted": c["row"].get("halted"),
                "prompt_tokens": c["row"]["prompt_tokens"],
                "completion_tokens": c["row"]["completion_tokens"],
                "duration_ms": c["row"].get("duration_ms"),
                "hours": len(c["row"]["hours"]),
            } for c in reps],
        }
        results["distances_to_reference"][name] = compare(cond, reps, ref_cond, ref_reps)
        results["distances_to_reference"][name]["agreement"] = agreement(reps, ref_reps)

    for spec in args.agreement:
        name, path = spec.split("=", 1)
        if os.path.exists(path):
            results.setdefault("decision_agreement", {})[name] = load(path)

    if args.accounting and os.path.exists(args.accounting):
        results["accounting"] = load(args.accounting)

    verdicts = {}
    for name, d in results["distances_to_reference"].items():
        seps = d["separations"]
        verdicts[name] = {
            "separates_from_reference": bool(seps),
            "kpis": seps,
            "label": ("differs from R@max on " + ", ".join(s["kpi"] for s in seps)
                      if seps else "no detectable effect at this design"),
        }
    results["stage_m_verdict"] = verdicts

    render(results, args.out)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            json.dump(results, fh, indent=2, sort_keys=True)
            fh.write("\n")
    print("wrote %s" % args.out)
    for name, v in verdicts.items():
        print("verdict %s: %s" % (name, v["label"]))


def render(results, path):
    L = []
    w = L.append
    w("# Stage M analysis — model: each non-reference model's distance to `R@max`\n")
    w("Rule (pre-registered, `.scratch/58-w3-preregistration/spec.md` §3/§3.1, unchanged): a scalar")
    w("KPI separates when `SNR = RMSE(d)/noise >= 2` **and** `|d(t)|` exceeds the replicate envelope")
    w("for >= 24 consecutive hours. The histogram families are carried by Wasserstein-1 over buckets.")
    w("No scalar weighting exists, and no model is ranked: the `luna` reference is the benchmark, not")
    w("an optimum.\n")
    w("Identity of the runs compared:\n")
    w("| run | model_id | wire | effort | contract | H | replicates |")
    w("|---|---|---|---|---|---:|---:|")
    for s in results["sources"]:
        w("| %s | %s | %s | %s | %s | %s | – |"
          % (s["tag"], s["model_id"], s["wire_mode"], s["effort"], s["claim_contract"],
             s["horizon_hours"]))
    w("")

    if "accounting" in results:
        w("## Accounting (calls, tokens, derived cost, wall)\n")
        w("| model | calls | prompt tok | completion tok | derived $ | cell seconds (sum of reps) | wall |")
        w("|---|---:|---:|---:|---:|---:|---:|")
        for name, a in sorted(results["accounting"].get("models", {}).items()):
            secs = sum(c["segment_duration_s"] for c in a["cells"])
            w("| %s | %d | %d | %d | %.4f | %.0f | %.1f min |"
              % (name, a["calls"], a["prompt_tokens"], a["completion_tokens"],
                 a["derived_usd"], secs, a.get("launch_to_eval_seconds", 0) / 60))
        tot = results["accounting"].get("stage_total")
        if tot:
            w("| **stage total (Stage M)** | **%d** | **%d** | **%d** | **%.4f** | | |"
              % (tot["calls"], tot["prompt_tokens"], tot["completion_tokens"],
                 tot["derived_usd"]))
        ref = results["accounting"].get("reference")
        if ref:
            w("| luna@max (Stage-E rows, not re-run) | %d | %d | %d | %.4f | (previously recorded) | |"
              % (ref["calls"], ref["prompt_tokens"], ref["completion_tokens"],
                 ref["derived_usd"]))
        w("")

    if "decision_agreement" in results:
        w("## Decision agreement with the frozen cascade (descriptive)\n")
        w("| condition | calls | action == frozen action | fraction | sources | statuses | wake hours | latency p50 / p95 / max ms |")
        w("|---|---:|---:|---:|---|---|---:|---|")
        for name, by_arm in sorted(results["decision_agreement"].items()):
            for arm, a in sorted(by_arm.items()):
                w("| %s %s | %d | %d | %.4f | %s | %s | %d | %.0f / %.0f / %.0f |"
                  % (name, arm, a["calls"], a["action_equals_frozen"],
                     a["action_equals_frozen_fraction"], ",".join(a["sources"]),
                     ",".join(a["statuses"]), a["wake_hours"],
                     a["latency_ms_p50"], a["latency_ms_p95"], a["latency_ms_max"]))
        w("")

    w("## §3.1 — each non-reference model's distance to `luna@max` (cell `%s`)\n"
      % results["reference"]["cell"])
    w("`first` = first separating hour (-1 none); `run` = longest separating run and its start;")
    w("`power` = smallest separable constant difference; `dir` = direction of a *detected* separation.\n")
    for name, d in results["distances_to_reference"].items():
        w("### `%s@max` − `luna@max` (3 replicates each)\n" % name)
        w("| KPI | RMSE(d) | noise | SNR | first | run (start) | power | max abs d | d at H | dir | W1 |")
        w("|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|")
        for kpi in SCALARS + INSTRUMENT_EXTRA:
            c = d["kpis"][kpi]
            w("| %s | %s | %s | %s | %d | %d (%d) | %s | %s | %s | %s | %s |"
              % (kpi, fmt(c["rmse_d"]), fmt(c["noise"]), fmt(c["snr"]),
                 c["first_separation"], c["run_length"], c["run_start"],
                 fmt(c["power"]["smallest_separable"]), fmt(c["max_abs_d"]),
                 fmt(c["mean_d_at_H"]), c["separation_direction"] or "-",
                 fmt(c["wasserstein"])))
        for family in HISTS:
            w("| %s | – | – | – | – | – | – | – | – | – | %s |"
              % (family, fmt(d["kpis"][family]["wasserstein"])))
        w("")
        if d["separations"]:
            w("Separations: " + "; ".join("%s %s" % (s["kpi"], s["direction"])
                                          for s in d["separations"]) + "\n")
        else:
            w("No KPI separates: `%s@max` matches `luna@max` at this design (no detectable effect "
              "at this design, never equivalence).\n" % name)
        ag = d.get("agreement")
        if ag:
            w("Curve agreement (descriptive): the declared scalars differ at all at **%d** hours"
              % ag["hours_any_scalar_differ"])
            if not ag["identical"]:
                w("(first h%d, last h%d)." % (ag["first_differ_hour"], ag["last_differ_hour"]))
            else:
                w("— the two conditions are the same world at every hour.")
            w("")

    w("## Endpoints (mean over each condition's 3 replicates, hour 144)\n")
    w("| condition | alive@H | births@H | starve@H | age@H | max_capital@H |")
    w("|---|---:|---:|---:|---:|---:|")
    rows = [("luna@max (reference)", results["reference"]["endpoints"])]
    for name, m in results["models"].items():
        rows.append(("%s@max" % name, m["endpoints"]))
    for label, ep in rows:
        w("| %s | %s | %s | %s | %s | %s |"
          % (label, fmt(ep["population"], 2), fmt(ep["births_cum"], 2),
             fmt(ep["deaths_starve_cum"], 2), fmt(ep["deaths_age_cum"], 2),
             fmt(ep["max_capital"], 2)))
    w("")

    w("## Stage-M verdict (§3.1)\n")
    w("The reference is the benchmark; a model is neither ranked nor called better.\n")
    w("| model | relation to `luna@max` | KPIs that separate |")
    w("|---|---|---|")
    for name, v in results["stage_m_verdict"].items():
        w("| %s | %s | %s |"
          % (name, "differs" if v["separates_from_reference"] else "matches",
             ", ".join("%s (%s)" % (s["kpi"], s["direction"]) for s in v["kpis"]) or "–"))
    w("")
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(L) + "\n")


if __name__ == "__main__":
    main()
