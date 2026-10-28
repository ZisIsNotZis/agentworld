#!/usr/bin/env python3
"""Derived accounting for Stage T, read-only: calls, tokens, derived cost, wall.

The gateway records `github_copilot/gpt-6-luna` as unpriced (cost_micros = 0), so
the recorded charge is $0 and the derived price is arithmetic at the ticket-35
published rates ($0.10/M input, $0.50/M output) — the same rates and method as
`.scratch/58-w3-effort/accounting.py`.

Wall clock is launch-to-evaluation.json for the whole stage (the four arms ran as
one matrix), plus each cell's own recorded `duration_ms` segment.

`reference` is the Stage-E R@max rows, reported beside the fresh arms for the
reader's total; it is not a Stage-T charge and is not counted in `fresh_total`.

Usage:
  python3 accounting.py --dir .scratch/59-w3-timing \
      --reference ../58-w3-effort/max/evaluation.json \
      --reference-extra ../57-w2-concurrent-reference/evaluation.json \
      --starts starts.json --out wall.json
"""
import argparse
import json
import os

RATE_IN = 0.10 / 1_000_000
RATE_OUT = 0.50 / 1_000_000


def summarise(path, arms=None):
    ev = json.load(open(path, encoding="utf-8"))
    out = {}
    for cell in ev["cells"]:
        if not cell.get("ran"):
            continue
        arm = cell["arm"]
        if arms is not None and arm not in arms:
            continue
        a = out.setdefault(arm, {"calls": 0, "prompt_tokens": 0, "completion_tokens": 0,
                                 "cells": []})
        a["calls"] += cell["realized_calls"]
        a["prompt_tokens"] += cell["prompt_tokens"]
        a["completion_tokens"] += cell["completion_tokens"]
        a["cells"].append({
            "label": "%s-f%d-s%d-r%d" % (arm, cell["f"], cell["seed"], cell["replicate"]),
            "calls": cell["realized_calls"],
            "call_ceiling": cell.get("call_ceiling"),
            "under_budget": cell.get("under_budget"),
            "halted": cell.get("halted"),
            "budget_shortfall": cell.get("budget_shortfall"),
            "decision_errors": cell.get("decision_errors"),
            "prompt_tokens": cell["prompt_tokens"],
            "completion_tokens": cell["completion_tokens"],
            "segment_duration_s": round(cell.get("duration_ms", 0) / 1000.0, 1),
        })
    return out, ev


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True)
    ap.add_argument("--reference", default="")
    ap.add_argument("--reference-extra", default="")
    ap.add_argument("--starts", default="")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    out, ev = summarise(os.path.join(args.dir, "evaluation.json"))
    start = 0.0
    if args.starts and os.path.exists(args.starts):
        start = float(json.load(open(args.starts, encoding="utf-8"))["launch_epoch"])
    wall = os.path.getmtime(os.path.join(args.dir, "evaluation.json")) - start

    total = 0.0
    for arm, a in out.items():
        a["derived_usd"] = round(a["prompt_tokens"] * RATE_IN + a["completion_tokens"] * RATE_OUT, 4)
        total += a["derived_usd"]
    out["fresh_total"] = {
        "calls": sum(a["calls"] for k, a in out.items() if k != "fresh_total"),
        "prompt_tokens": sum(a["prompt_tokens"] for k, a in out.items() if k != "fresh_total"),
        "completion_tokens": sum(a["completion_tokens"] for k, a in out.items() if k != "fresh_total"),
    }
    out["fresh_total"]["derived_usd"] = round(total, 4)
    out["stage_wall_seconds"] = round(wall, 1)

    if args.reference and os.path.exists(args.reference):
        ref, _ = summarise(args.reference, arms=["R"])
        if args.reference_extra and os.path.exists(args.reference_extra):
            extra, _ = summarise(args.reference_extra, arms=["R"])
            for k in ("calls", "prompt_tokens", "completion_tokens"):
                ref["R"][k] += extra["R"][k]
            ref["R"]["cells"] += extra["R"]["cells"]
        r = ref["R"]
        r["derived_usd"] = round(r["prompt_tokens"] * RATE_IN + r["completion_tokens"] * RATE_OUT, 4)
        r["source"] = "Stage-E rows (max/evaluation.json 2 fresh + ticket-57 replicate 1); not re-run in Stage T"
        out["reference"] = r

    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(out, fh, indent=2, sort_keys=True)
        fh.write("\n")
    print(json.dumps(out, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
