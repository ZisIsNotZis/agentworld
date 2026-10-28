#!/usr/bin/env python3
"""Derived accounting for Stage M, read-only: calls, tokens, derived cost, wall.

The gateway records every model in this stage as unpriced (cost_micros = 0,
price_version = unpriced), so the recorded charge is $0 and the derived price is
arithmetic at the ticket-35 published rates ($0.10/M input, $0.50/M output) —
the same rates and method as `.scratch/58-w3-effort/accounting.py` and
`.scratch/59-w3-timing/accounting.py`.

Wall clock is each model's launch epoch (starts.json) to its evaluation.json.

Usage:
  python3 accounting.py --eval gpt-6-sol=path/evaluation.json \
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
            "decision_errors": cell.get("decision_errors"),
            "applied_model_decisions": cell.get("applied_model_decisions"),
            "halted": cell.get("halted"),
            "prompt_tokens": cell["prompt_tokens"],
            "completion_tokens": cell["completion_tokens"],
            "segment_duration_s": round(cell.get("duration_ms", 0) / 1000.0, 1),
        })
    return out, ev


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--eval", action="append", required=True, help="<name>=<evaluation.json>")
    ap.add_argument("--reference", default="")
    ap.add_argument("--reference-extra", default="")
    ap.add_argument("--starts", default="")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    starts = {}
    if args.starts and os.path.exists(args.starts):
        starts = json.load(open(args.starts, encoding="utf-8"))

    out = {"models": {}}
    total = {"calls": 0, "prompt_tokens": 0, "completion_tokens": 0, "derived_usd": 0.0}
    for spec in args.eval:
        name, path = spec.split("=", 1)
        arms, ev = summarise(path)
        a = arms.get("R")
        if a is None:
            raise SystemExit("%s: no ran R cell in %s" % (name, path))
        a["derived_usd"] = round(a["prompt_tokens"] * RATE_IN + a["completion_tokens"] * RATE_OUT, 4)
        a["model_id"] = ev.get("model_id")
        a["wire_mode"] = ev.get("wire_mode")
        raw = starts.get(name, 0.0)
        start = float(raw["launch_epoch"] if isinstance(raw, dict) else raw)
        if start:
            a["launch_to_eval_seconds"] = round(os.path.getmtime(path) - start, 1)
        out["models"][name] = a
        for k in total:
            total[k] += a[k]
    total["derived_usd"] = round(total["derived_usd"], 4)
    out["stage_total"] = total

    if args.reference and os.path.exists(args.reference):
        ref, _ = summarise(args.reference, arms=["R"])
        if args.reference_extra and os.path.exists(args.reference_extra):
            extra, _ = summarise(args.reference_extra, arms=["R"])
            for k in ("calls", "prompt_tokens", "completion_tokens"):
                ref["R"][k] += extra["R"][k]
            ref["R"]["cells"] += extra["R"]["cells"]
        r = ref["R"]
        r["derived_usd"] = round(r["prompt_tokens"] * RATE_IN + r["completion_tokens"] * RATE_OUT, 4)
        r["source"] = ("Stage-E rows (max/evaluation.json 2 fresh + ticket-57 replicate 1); "
                       "not re-run in Stage M")
        out["reference"] = r

    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(out, fh, indent=2, sort_keys=True)
        fh.write("\n")
    print(json.dumps(out, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
