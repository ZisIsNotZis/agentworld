#!/usr/bin/env python3
"""Ticket 39 diagnosis: re-derive every number in spec section 11 from the run.

Zero spend, zero scoring. It reads only the committed evaluation JSON and the
run root's call ledgers, and prints (or writes) the facts the diagnosis cites:
the halt chain, per-cell calls against the ceiling, the three A1 replicates'
retry accounting, and the terminal-sample state vectors. It computes no band, no
effect floor and no pair comparison: those are the scorer's job and the scorer
refused (spec section 10.4).

    python3 .scratch/39-m2c-matrix/diagnose.py \
        --run-root .tmp/39-m2c-runs \
        --eval .scratch/39-m2c-matrix/m2c-evaluation.json \
        --out .scratch/39-m2c-matrix/halt-evidence.json
"""
import argparse
import base64
import collections
import glob
import json
import os

CEILING = 256
PRICE_IN = 0.10 / 1e6
PRICE_OUT = 0.50 / 1e6


def call_hour(rec):
    """The simulated hour of a call record, read from its canonical request."""
    return json.loads(base64.b64decode(rec["request_bytes"]))["actor"]["hour"]


def read_cell(path):
    rows = [json.loads(line) for line in open(path)]
    return {
        "calls": [r for r in rows if r["kind"] == "call"],
        "role_a": [r for r in rows if r["kind"] == "role-a"],
        "allocation": [r for r in rows if r["kind"] == "allocation"],
    }


def cell_facts(cell, rows):
    calls, alloc = rows["calls"], rows["allocation"]
    per_hour = collections.Counter(call_hour(r) for r in calls)
    prompt = sum(r["usage"]["prompt_tokens"] for r in calls)
    completion = sum(r["usage"]["completion_tokens"] for r in calls)
    non_ok = [{"seq": r["seq"], "hour": call_hour(r), "status": r["status"],
               "attempt": r["attempt"], "latency_micros": r["latency_micros"],
               "http_status": r.get("http_status")} for r in calls if r["status"] != "ok"]
    quota = [r["allocation"]["quota"] for r in alloc]
    # The hour at which the cell's charged attempts first reach each milestone.
    milestone, running = {}, 0
    for hour in sorted(per_hour):
        running += per_hour[hour]
        for target in (250, 254, 255, 256):
            if target not in milestone and running >= target:
                milestone[target] = hour
    return {
        "cell": cell,
        "charged_calls": len(calls),
        "decisions": len(rows["role_a"]),
        "ceiling_pct": round(100.0 * len(calls) / CEILING, 2),
        "remaining_under_ceiling": CEILING - len(calls),
        "decision_hours_with_a_call": len(per_hour),
        "hours_with_two_calls": sum(1 for n in per_hour.values() if n == 2),
        "quota_schedule_sum": sum(quota),
        "quota_histogram": dict(collections.Counter(quota)),
        "milestone_hour": milestone,
        "retried_attempts": [{"seq": r["seq"], "hour": call_hour(r), "attempt": r["attempt"],
                              "status": r["status"]} for r in calls if r["attempt"] > 1],
        "non_ok": non_ok,
        "prompt_tokens": prompt,
        "completion_tokens": completion,
        "cost_usd": round(prompt * PRICE_IN + completion * PRICE_OUT, 4),
        "cost_usd_exact": prompt * PRICE_IN + completion * PRICE_OUT,
        "actor_calls": dict(sorted(collections.Counter(
            json.loads(base64.b64decode(r["request_bytes"]))["actor"]["id"] for r in calls).items())),
    }


def terminal_vectors(path):
    doc = json.load(open(path))
    out = []
    for c in sorted(doc["cells"], key=lambda c: (c["arm"], c["replicate"])):
        alive, births, starve = c["alive_by_hour"], c["births_cum"], c["deaths_starve_cum"]
        out.append({
            "cell": "%s-r%d" % (c["arm"], c["replicate"]),
            "samples": len(alive),
            "H_at_last_sample": sum(alive),
            "H_at_h143": sum(alive[:-1]) if len(alive) > 1 else None,
            "births_last_sample": births[-1], "starvation_last_sample": starve[-1],
            "alive_last_sample": alive[-1],
            "alive_h143": alive[-2] if len(alive) > 1 else None,
            "halted": c["halted"],
            "ceiling_hit": c.get("ceiling_hit"),
            "ceiling_rows_calls": next((r for r in (c.get("ceiling_rows") or [])
                                        if r["ceiling"] == "cognitive-calls"), None),
        })
    return {"quota_schedule_len": len(doc["quota_schedule"]),
            "quota_schedule_sum": sum(doc["quota_schedule"]),
            "inventory": doc["inventory"], "cells": out}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-root", default=".tmp/39-m2c-runs")
    ap.add_argument("--eval", default=".scratch/39-m2c-matrix/m2c-evaluation.json")
    ap.add_argument("--out")
    args = ap.parse_args()

    cells = []
    for path in sorted(glob.glob(os.path.join(args.run_root, "*", "calls.jsonl"))):
        cell = os.path.basename(os.path.dirname(path))
        cells.append(cell_facts(cell, read_cell(path)))

    total_calls = sum(c["charged_calls"] for c in cells)
    total_decisions = sum(c["decisions"] for c in cells)
    evidence = {
        "ceiling": CEILING,
        "totals": {
            "charged_calls": total_calls, "decisions": total_decisions,
            "retried_attempts": total_calls - total_decisions,
            "cost_usd": round(sum(c["cost_usd_exact"] for c in cells), 4),
            "price_per_attempt_usd": round(sum(c["cost_usd_exact"] for c in cells) / total_calls, 8)
            if total_calls else None,
        },
        "cells": cells,
        "terminal": terminal_vectors(args.eval),
    }
    text = json.dumps(evidence, indent=1) + "\n"
    if args.out:
        with open(args.out, "w") as fh:
            fh.write(text)
        print("wrote %s" % args.out)
    else:
        print(text)


if __name__ == "__main__":
    main()
