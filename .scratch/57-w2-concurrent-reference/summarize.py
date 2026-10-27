#!/usr/bin/env python3
"""Summarise the ticket-57 R@max reference cell (no network, no spend).

Reads the recorded ledger and the evaluation and writes two human-auditable
tables beside them:

  calls.tsv        one row per recorded Role A call (seq, hour, actor, the
                   model's action, the frozen action it replaced, status,
                   latency, tokens). The hour is joined from the intervention
                   record that follows each call (votes=1 in this run).
  kpi-series.tsv   the per-hour KPI series the evaluation carries (alive,
                   cumulative births, cumulative starvation and age deaths,
                   max capital, and the capital/granary distributions).

Usage:
  python3 summarize.py --ledger runs2/R-f10-s0-r1/calls.jsonl \
      --evaluation evaluation.json
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


def write_calls(calls, interventions, out):
    # The ledger interleaves calls and interventions under concurrency, so the
    # join is by the intervention's recorded call_refs, never by position.
    by_seq = {call["seq"]: call for call in calls}
    rows = []
    for inv in interventions:
        role_a = inv.get("role_a") or {}
        for ref in inv.get("call_refs") or []:
            call = by_seq.get(ref)
            if call is None:
                raise SystemExit(f"intervention {inv['seq']} references unknown call {ref}")
            rows.append((call["seq"], inv["cause"]["hour"], inv["cause"]["actor"],
                         role_a.get("action", ""), role_a.get("frozen_action", ""),
                         role_a.get("source", ""), call["status"], call["http_status"],
                         call["attempt"], round(call["latency_micros"] / 1000.0, 1),
                         call["usage"]["prompt_tokens"], call["usage"]["completion_tokens"],
                         call["request_hash"]))
    rows.sort()
    with open(out, "w", encoding="utf-8") as fh:
        fh.write("seq\thour\tactor\taction\tfrozen_action\tsource\tstatus\thttp\tattempt\tlatency_ms\tprompt_tokens\tcompletion_tokens\trequest_hash\n")
        for row in rows:
            fh.write("\t".join(str(v) for v in row) + "\n")


def write_kpis(evaluation, out):
    cell = evaluation["cells"][0]
    with open(out, "w", encoding="utf-8") as fh:
        fh.write("hour\talive\tbirths_cum\tstarvation_cum\tage_cum\tmax_capital\tcapital_hist\tgranary_hist\n")
        for i, hour in enumerate(cell["hours"]):
            fh.write("\t".join([
                str(hour), str(cell["alive_by_hour"][i]), str(cell["births_cum"][i]),
                str(cell["deaths_starve_cum"][i]), str(cell["deaths_age_cum"][i]),
                str(cell["max_capital"][i]),
                "[" + ",".join(str(v) for v in cell["capital_hist"][i]) + "]",
                "[" + ",".join(str(v) for v in cell["granary_hist"][i]) + "]",
            ]) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ledger", default=os.path.join("runs2", "R-f10-s0-r1", "calls.jsonl"))
    ap.add_argument("--evaluation", default="evaluation.json")
    ap.add_argument("--outdir", default=".")
    args = ap.parse_args()
    calls, interventions = read_ledger(args.ledger)
    write_calls(calls, interventions, os.path.join(args.outdir, "calls.tsv"))
    with open(args.evaluation, encoding="utf-8") as fh:
        evaluation = json.load(fh)
    write_kpis(evaluation, os.path.join(args.outdir, "kpi-series.tsv"))
    print(f"calls={len(calls)} interventions={len(interventions)}")
    print(f"wrote {os.path.join(args.outdir, 'calls.tsv')} and {os.path.join(args.outdir, 'kpi-series.tsv')}")


if __name__ == "__main__":
    main()
