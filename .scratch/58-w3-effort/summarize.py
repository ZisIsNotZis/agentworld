#!/usr/bin/env python3
"""Summarise a Stage-E effort's recorded cells into two human-auditable tables.

READ-ONLY over the recorded run directories and the evaluation. Writes, beside
the effort's evaluation:

  calls.tsv        one row per recorded Role A call: cell, seq, hour, actor, the
                   model's action, the frozen action it replaced, source, status,
                   http, attempt, latency, tokens, request hash. The hour is
                   joined from the intervention that references the call, never
                   from ledger position (the ledger is ordered by completion
                   under call concurrency).
  kpi-series.tsv   the per-hour KPI series the evaluation carries, for every
                   cell/replicate of the effort.

Usage:
  python3 summarize.py --dir <effort-dir>
"""
import argparse
import json
import os


def read_ledger(path):
    calls, interventions = [], []
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            rec = json.loads(line)
            if rec.get("kind") == "call":
                calls.append(rec)
            else:
                interventions.append(rec)
    return calls, interventions


def write_kpis(evaluation, out):
    with open(out, "w", encoding="utf-8") as fh:
        fh.write("cell\treplicate\thour\talive\tbirths_cum\tstarvation_cum\tage_cum\tmax_capital"
                 "\twages_cum\tcapital_hist\tgranary_hist\n")
        for cell in evaluation["cells"]:
            if not cell.get("ran") or not cell.get("hours"):
                continue
            label = "R-f%d-s%d-r%d" % (cell["f"], cell["seed"], cell["replicate"])
            for i, hour in enumerate(cell["hours"]):
                fh.write("\t".join([
                    label, str(cell["replicate"]), str(hour),
                    str(cell["alive_by_hour"][i]), str(cell["births_cum"][i]),
                    str(cell["deaths_starve_cum"][i]), str(cell["deaths_age_cum"][i]),
                    str(cell["max_capital"][i]), str(cell["wages_cum"][i]),
                    "[" + ",".join(str(v) for v in cell["capital_hist"][i]) + "]",
                    "[" + ",".join(str(v) for v in cell["granary_hist"][i]) + "]",
                ]) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True, help="effort directory holding runs/ and evaluation.json")
    args = ap.parse_args()
    d = args.dir
    evaluation = json.load(open(os.path.join(d, "evaluation.json"), encoding="utf-8"))

    calls_out = os.path.join(d, "calls.tsv")
    total = 0
    with open(calls_out, "w", encoding="utf-8") as fh:
        fh.write("cell\tseq\thour\tactor\taction\tfrozen_action\tsource\tstatus\thttp\tattempt"
                 "\tlatency_ms\tprompt_tokens\tcompletion_tokens\trequest_hash\n")
        for cell in evaluation["cells"]:
            if not cell.get("ran") or not cell.get("hours"):
                continue
            label = "R-f%d-s%d-r%d" % (cell["f"], cell["seed"], cell["replicate"])
            ledger = os.path.join(d, "runs", label, "calls.jsonl")
            if not os.path.exists(ledger):
                print(f"skip {label}: no ledger at {ledger}")
                continue
            calls, interventions = read_ledger(ledger)
            by_seq = {call["seq"]: call for call in calls}
            rows = []
            for inv in interventions:
                role_a = inv.get("role_a") or {}
                # call_refs is the attempt range of one logical call. Under
                # concurrency another record can interleave between a retry's
                # attempts, so the range is filtered to actual call records;
                # every surviving ref is an attempt of the same request.
                refs = [by_seq[r] for r in (inv.get("call_refs") or []) if r in by_seq]
                if not refs:
                    raise SystemExit(f"{label}: intervention {inv['seq']} references no call record")
                hashes = {c["request_hash"] for c in refs}
                if len(hashes) != 1:
                    raise SystemExit(f"{label}: intervention {inv['seq']} refs mixed requests {hashes}")
                call = max(refs, key=lambda c: c["seq"])  # the used (final) attempt
                rows.append((label, call["seq"], inv["cause"]["hour"], inv["cause"]["actor"],
                             role_a.get("action", ""), role_a.get("frozen_action", ""),
                             role_a.get("source", ""), call.get("status", ""),
                             call.get("http_status", ""), call.get("attempt", 1),
                             round(call.get("latency_micros", 0) / 1000.0, 1),
                             call.get("usage", {}).get("prompt_tokens", 0),
                             call.get("usage", {}).get("completion_tokens", 0),
                             call["request_hash"]))
            rows.sort()
            for row in rows:
                fh.write("\t".join(str(v) for v in row) + "\n")
            total += len(rows)
            print(f"{label}: {len(rows)} calls")
    print(f"wrote {calls_out} ({total} calls)")
    kpis_out = os.path.join(d, "kpi-series.tsv")
    write_kpis(evaluation, kpis_out)
    print(f"wrote {kpis_out}")


if __name__ == "__main__":
    main()
