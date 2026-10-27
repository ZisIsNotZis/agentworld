#!/usr/bin/env python3
"""Derived accounting for Stage E, read-only: calls, tokens, derived cost, wall.

The gateway records `github_copilot/gpt-6-luna` as unpriced (cost_micros = 0),
so the recorded charge is $0 and the derived price is arithmetic at the
ticket-35 published rates ($0.10/M input, $0.50/M output). Wall clock is the
elapsed time from the recorded launch to the effort's evaluation.json, i.e. the
whole effort including its 3 concurrent cells and the in-process ledger replay.

Usage:
  python3 accounting.py --root .scratch/58-w3-effort --start-epoch <epoch> \
      --out .scratch/58-w3-effort/wall.json
"""
import argparse
import json
import os
import time

RATE_IN = 0.10 / 1_000_000
RATE_OUT = 0.50 / 1_000_000


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True)
    ap.add_argument("--start-epoch", type=float, default=0.0,
                    help="start epoch applied to every effort without its own entry")
    ap.add_argument("--starts", default="", help="JSON map effort -> start epoch (per wave)")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    starts = {}
    if args.starts and os.path.exists(args.starts):
        starts = json.load(open(args.starts, encoding="utf-8"))
    out = {}
    for effort in ["none", "low", "medium", "high", "max"]:
        d = os.path.join(args.root, effort)
        ev_path = os.path.join(d, "evaluation.json")
        if not os.path.exists(ev_path):
            continue
        ev = json.load(open(ev_path, encoding="utf-8"))
        calls = prompt = output = 0
        cells = []
        for cell in ev["cells"]:
            if not cell.get("ran"):
                continue
            calls += cell["realized_calls"]
            prompt += cell["prompt_tokens"]
            output += cell["completion_tokens"]
            cells.append({
                "label": "R-f%d-s%d-r%d" % (cell["f"], cell["seed"], cell["replicate"]),
                "calls": cell["realized_calls"],
                "prompt_tokens": cell["prompt_tokens"],
                "completion_tokens": cell["completion_tokens"],
                "segment_duration_s": round(cell.get("duration_ms", 0) / 1000.0, 1),
            })
        wall = os.path.getmtime(ev_path) - starts.get(effort, args.start_epoch)
        out[effort] = {
            "calls": calls,
            "prompt_tokens": prompt,
            "completion_tokens": output,
            "derived_usd": round(prompt * RATE_IN + output * RATE_OUT, 4),
            "wall_seconds": round(wall, 1),
            "cells": cells,
        }
    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(out, fh, indent=2, sort_keys=True)
        fh.write("\n")
    print(json.dumps(out, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
