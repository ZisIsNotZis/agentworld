#!/usr/bin/env python3
"""Reduce the ticket-43 A0 grid evaluation to the durable per-cell record.

The runner writes a ~6 MB evaluation JSON (it carries capital and granary
histograms), which is too large to track. This script keeps every field the
rung-2 criterion and its binding analysis need, plus the zero-spend accounting,
and writes one aggregate JSON and one flat TSV. Nothing is edited: each value is
copied from the evaluation artefact and the source hash is recorded.

    python3 .scratch/43-rung2-seed/reduce-a0-grid.py \
        .tmp/43-a0-evaluation.json \
        .scratch/43-rung2-seed/a0-grid.json \
        .scratch/43-rung2-seed/a0-grid.tsv
"""

import hashlib
import json
import sys


def first_increase(series):
    for h in range(1, len(series)):
        if series[h] > series[h - 1]:
            return h
    return None


def last_increase(series):
    for h in range(len(series) - 1, 0, -1):
        if series[h] > series[h - 1]:
            return h
    return None


def increase_hours(series):
    return [h for h in range(1, len(series)) if series[h] > series[h - 1]]


def main(argv):
    if len(argv) != 4:
        sys.stderr.write(__doc__.splitlines()[0] + "\n")
        raise SystemExit(2)
    src, dst_json, dst_tsv = argv[1:4]
    with open(src, "rb") as fh:
        raw = fh.read()
    doc = json.loads(raw)
    rows = []
    for c in doc.get("cells") or []:
        alive = c["alive_by_hour"]
        hours = c.get("hours") or list(range(len(alive)))
        n0 = alive[0]
        end = alive[-1]
        births = c["births_cum"][-1]
        starve = c["deaths_starve_cum"][-1]
        age = c["deaths_age_cum"][-1]
        fails = []
        if not end >= n0:
            fails.append("alive_end<N0")
        if starve != 0:
            fails.append("starvation>0")
        if births < 1:
            fails.append("no_birth")
        rows.append({
            "arm": c["arm"],
            "f": c["f"],
            "seed": c["seed"],
            "horizon": c["hours"][-1],
            "N0": n0,
            "alive_end": end,
            "births": births,
            "starvation_deaths": starve,
            "age_deaths": age,
            "actor_hours": sum(alive),
            "peak_alive": max(alive),
            "peak_hour": alive.index(max(alive)),
            "birth_hours": increase_hours(c["births_cum"]),
            "first_birth_hour": first_increase(c["births_cum"]),
            "last_birth_hour": last_increase(c["births_cum"]),
            "first_age_death_hour": first_increase(c["deaths_age_cum"]),
            "last_age_death_hour": last_increase(c["deaths_age_cum"]),
            "first_starvation_hour": first_increase(c["deaths_starve_cum"]),
            "last_starvation_hour": last_increase(c["deaths_starve_cum"]),
            "criterion_pass": not fails,
            "failing_clauses": fails,
            "ran": c.get("ran"),
            "halted": c.get("halted"),
            "realized_calls": c.get("realized_calls"),
            "prompt_tokens": c.get("prompt_tokens"),
            "completion_tokens": c.get("completion_tokens"),
            "cost_micros": c.get("cost_micros"),
            "duration_ms": c.get("duration_ms"),
        })
    rows.sort(key=lambda r: (r["f"], r["seed"]))

    passing = [r for r in rows if r["criterion_pass"]]
    clause_counts = {}
    for r in rows:
        for f in r["failing_clauses"]:
            clause_counts[f] = clause_counts.get(f, 0) + 1
    nonzero_spend = [
        {"f": r["f"], "seed": r["seed"], "realized_calls": r["realized_calls"],
         "cost_micros": r["cost_micros"]}
        for r in rows
        if r["realized_calls"] or r["prompt_tokens"] or r["completion_tokens"] or r["cost_micros"]
    ]
    # age-death share of all deaths, per cell: every death is age + starvation here.
    for r in rows:
        total_deaths = r["age_deaths"] + r["starvation_deaths"]
        r["age_death_share"] = (r["age_deaths"] / total_deaths) if total_deaths else None

    out = {
        "ticket": "43-rung2-seed",
        "source_artefact": src.split("/")[-1],
        "source_sha256": hashlib.sha256(raw).hexdigest(),
        "arms": sorted({r["arm"] for r in rows}),
        "horizon_hours": doc.get("horizon_hours"),
        "inventory": doc.get("inventory"),
        "status": doc.get("status"),
        "grid": {
            "founders": sorted({r["f"] for r in rows}),
            "seeds": sorted({r["seed"] for r in rows}),
            "cells": len(rows),
        },
        "criterion": "(alive_end >= N0) AND (starvation_deaths == 0) AND (births >= 1)",
        "cells_passing": len(passing),
        "passing_cells": [{"f": r["f"], "seed": r["seed"]} for r in passing],
        "clause_failure_counts": clause_counts,
        "zero_spend": {
            "cells_nonzero_calls_or_cost": len(nonzero_spend),
            "detail": nonzero_spend,
            "live_flag_used": False,
        },
        "cells": rows,
    }
    with open(dst_json, "w", encoding="utf-8") as fh:
        json.dump(out, fh, indent=1, sort_keys=True)
        fh.write("\n")

    cols = ["f", "seed", "horizon", "N0", "alive_end", "births", "starvation_deaths",
            "age_deaths", "actor_hours", "peak_alive", "peak_hour",
            "first_birth_hour", "last_birth_hour", "first_age_death_hour",
            "last_age_death_hour", "first_starvation_hour", "last_starvation_hour",
            "criterion_pass", "failing_clauses"]
    with open(dst_tsv, "w", encoding="utf-8") as fh:
        fh.write("\t".join(cols) + "\n")
        for r in rows:
            vals = []
            for k in cols:
                v = r[k]
                if isinstance(v, bool):
                    v = "PASS" if v else "FAIL"
                elif isinstance(v, list):
                    v = ",".join(str(x) for x in v) if v else "-"
                elif v is None:
                    v = "-"
                vals.append(str(v))
            fh.write("\t".join(vals) + "\n")
    print("wrote %s (%d cells, %d bytes) and %s"
          % (dst_json, len(rows), len(raw), dst_tsv))


if __name__ == "__main__":
    main(sys.argv)
