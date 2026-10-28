#!/usr/bin/env python3
"""Summarise the Stage-T evaluation into two human-auditable tables.

READ-ONLY over the recorded run directories and the evaluation. Writes, beside
the stage's evaluation:

  calls.tsv        one row per recorded Role A call: cell, seq, hour, actor, the
                   model's action, the frozen action it replaced, source, status,
                   http, attempt, latency, tokens, request hash. The hour is
                   joined from the intervention that references the call, never
                   from ledger position (the ledger is ordered by completion
                   under retries). A0 makes no calls and contributes no rows.
  kpi-series.tsv   the per-hour KPI series the evaluation carries, for every
                   cell/replicate of every arm.

Derived from `.scratch/58-w3-effort/summarize.py` (same logic; the label is
`<arm>-f<F>-s<S>-r<N>` because Stage T holds four arms in one evaluation).

Usage:
  python3 summarize.py --dir .scratch/59-w3-timing
"""
import argparse
import json
import os


def read_ledger(path):
    """Split a ledger into call records and role-a interventions.

    A budgeted arm's ledger also carries `allocation` records (the per-hour
    selection). They are not interventions: they reference no call, so a caller
    that treats every non-call record as an intervention fails on them (Stage E
    never saw one: its only arm was R, which has no priority layer).
    """
    calls, interventions = [], []
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            rec = json.loads(line)
            if rec.get("kind") == "call":
                calls.append(rec)
            elif rec.get("kind") == "role-a":
                interventions.append(rec)
    return calls, interventions


def read_calls(path):
    out = []
    with open(path, encoding="utf-8") as fh:
        for row in fh.readlines()[1:]:
            f = row.rstrip("\n").split("\t")
            out.append({"cell": f[0], "seq": int(f[1]), "hour": int(f[2]), "actor": f[3],
                        "action": f[4], "frozen_action": f[5], "source": f[6], "status": f[7],
                        "http": f[8], "attempt": int(f[9]), "latency_ms": float(f[10]),
                        "arm": f[0].split("-")[0]})
    return out


def write_kpis(evaluation, out):
    with open(out, "w", encoding="utf-8") as fh:
        fh.write("cell\treplicate\thour\talive\tbirths_cum\tstarvation_cum\tage_cum\tmax_capital"
                 "\twages_cum\tcapital_hist\tgranary_hist\n")
        for cell in evaluation["cells"]:
            if not cell.get("ran") or not cell.get("hours"):
                continue
            label = "%s-f%d-s%d-r%d" % (cell["arm"], cell["f"], cell["seed"], cell["replicate"])
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
    ap.add_argument("--dir", required=True, help="stage directory holding runs/ and evaluation.json")
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
            label = "%s-f%d-s%d-r%d" % (cell["arm"], cell["f"], cell["seed"], cell["replicate"])
            ledger = os.path.join(d, "runs", label, "calls.jsonl")
            if not os.path.exists(ledger):
                if cell["arm"] == "A0":
                    continue  # no cognition, no call ledger
                print(f"skip {label}: no ledger at {ledger}")
                continue
            calls, interventions = read_ledger(ledger)
            by_seq = {call["seq"]: call for call in calls}
            rows = []
            for inv in interventions:
                role_a = inv.get("role_a") or {}
                # call_refs is the attempt range of one logical call; under
                # retries the range is filtered to actual call records and every
                # surviving ref must be an attempt of the same request.
                refs = [by_seq[r] for r in (inv.get("call_refs") or []) if r in by_seq]
                if not refs:  # a role-a intervention always references its call
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

    # agreement.json: what the model actually decided, per arm — the reading the
    # curve alone cannot give. `action == frozen_action` counts the calls whose
    # answer was the frozen cascade's own action (the fallback the seam would
    # have used anyway); latency is the recorded per-attempt wall time.
    agreement = {}
    by_arm = {}
    for row in read_calls(calls_out):
        by_arm.setdefault(row["arm"], []).append(row)
    for arm, rs in sorted(by_arm.items()):
        lat = sorted(r["latency_ms"] for r in rs)
        agreement[arm] = {
            "calls": len(rs),
            "action_equals_frozen": sum(1 for r in rs if r["action"] == r["frozen_action"]),
            "action_equals_frozen_fraction": round(
                sum(1 for r in rs if r["action"] == r["frozen_action"]) / len(rs), 4),
            "sources": sorted({r["source"] for r in rs}),
            "statuses": sorted({r["status"] for r in rs}),
            "wake_hours": len({r["hour"] for r in rs}),
            "latency_ms_p50": round(lat[len(lat) // 2], 1),
            "latency_ms_p95": round(lat[int(len(lat) * 0.95)], 1),
            "latency_ms_max": round(lat[-1], 1),
        }
    with open(os.path.join(d, "agreement.json"), "w", encoding="utf-8") as fh:
        json.dump(agreement, fh, indent=2, sort_keys=True)
        fh.write("\n")
    print(f"wrote {os.path.join(d, 'agreement.json')}")
    kpis_out = os.path.join(d, "kpi-series.tsv")
    write_kpis(evaluation, kpis_out)
    print(f"wrote {kpis_out}")


if __name__ == "__main__":
    main()
