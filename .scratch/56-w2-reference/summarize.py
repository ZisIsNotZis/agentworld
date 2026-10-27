#!/usr/bin/env python3
"""Ticket 56 R@max ledger summary: calls, tokens, statuses, outcomes, per-hour KPIs.

Reads the recorded ledger and evaluation JSON of the one live R@max cell and
prints a compact table. No model calls, no spend. The recorded evaluation JSON is
the authoritative per-hour KPI series; this script only tallies.
"""
import argparse
import base64
import collections
import json


def read_jsonl(path):
    with open(path) as f:
        return [json.loads(line) for line in f if line.strip()]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ledger", required=True)
    ap.add_argument("--eval", default="")
    args = ap.parse_args()

    records = read_jsonl(args.ledger)
    calls = [r for r in records if "content" in r]
    decisions = [r for r in records if "call_refs" in r]

    statuses = collections.Counter(r["status"] for r in calls)
    prompt = sum(r["usage"]["prompt_tokens"] for r in calls)
    completion = sum(r["usage"]["completion_tokens"] for r in calls)
    latency = sorted(r["latency_micros"] / 1e6 for r in calls)

    outcomes = collections.Counter()
    applied = rejected = 0
    for d in decisions:
        ra = d.get("role_a") or {}
        outcomes[(ra.get("source"), ra.get("reason", ""), ra.get("action", ""))] += 1
        if ra.get("source") == "model":
            applied += 1
        else:
            rejected += 1

    print("ledgers: calls=%d decisions=%d" % (len(calls), len(decisions)))
    print("statuses: " + ", ".join("%s=%d" % kv for kv in sorted(statuses.items())))
    print("tokens: prompt=%d completion=%d total=%d" % (prompt, completion, prompt + completion))
    if latency:
        print("latency_s: min=%.1f median=%.1f max=%.1f mean=%.1f" % (
            latency[0], latency[len(latency) // 2], latency[-1], sum(latency) / len(latency)))
    print("decisions applied_model=%d fallback=%d" % (applied, rejected))
    print("outcomes:")
    for (source, reason, action), n in outcomes.most_common():
        print("  source=%s reason=%s action=%s n=%d" % (source, reason or "-", action or "-", n))

    # Gateway-reported cost: the responses wire reports cost=null for this model.
    gateway_costs = set()
    for r in calls:
        try:
            body = json.loads(base64.b64decode(r["response_body"]))
        except Exception:
            continue
        gateway_costs.add(str((body.get("usage") or {}).get("cost")))
    print("gateway_reported_usage_cost: " + ", ".join(sorted(gateway_costs)))

    if not args.eval:
        return
    ev = json.load(open(args.eval))
    print("evaluation: status=%s model=%s effort=%s wire=%s claim=%s horizon=%d" % (
        ev.get("status"), ev.get("model_id"), ev.get("declared_effort"), ev.get("wire_mode"),
        ev.get("claim_contract"), ev.get("horizon_hours")))
    print("world: base_policy=%s mechanism=%s yield=%s birth_cost=%s" % (
        ev.get("base_policy"), ev.get("mechanism"), ev.get("yield"), ev.get("birth_cost")))
    for c in ev.get("cells", []):
        print("cell %s ran=%s calls=%d prompt=%d completion=%d cost_micros=%d duration_ms=%d" % (
            c.get("arm"), c.get("ran"), c.get("realized_calls"), c.get("prompt_tokens"),
            c.get("completion_tokens"), c.get("cost_micros"), c.get("duration_ms")))
        series = {
            "alive": c.get("alive_by_hour"),
            "births": c.get("births_cum"),
            "starvation": c.get("deaths_starve_cum"),
            "age": c.get("deaths_age_cum"),
        }
        for name, values in series.items():
            if not values:
                continue
            marks = []
            for h in (0, 24, 48, 72, 96, 120, 143, 144):
                if h < len(values):
                    marks.append("h%d=%d" % (h, values[h]))
            print("  %s: %s (final=%s)" % (name, " ".join(marks), values[-1]))


if __name__ == "__main__":
    main()
