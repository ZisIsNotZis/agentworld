"""Phase B poll: one bounded observation of the detached matrix, appended to phase-b-poll.log.

Liveness = runner pid exists AND the per-cell call ledgers grow (spec section 5); the log file is
never read as evidence. Spend is computed from each call row's usage at $0.10/M in, $0.50/M out.
"""
import json, glob, collections, subprocess, sys, datetime

PID = "2725555"


def calls():
    per = collections.defaultdict(lambda: {"calls": 0, "in": 0, "out": 0})
    bad = []
    for p in sorted(glob.glob(".tmp/39-m2c-runs/*/calls.jsonl")):
        key = p.split("/")[-2]
        for line in open(p):
            try:
                r = json.loads(line)
            except Exception:
                continue
            if r.get("kind") != "call":
                continue
            t = per[key]
            t["calls"] += 1
            u = r.get("usage") or {}
            t["in"] += u.get("prompt_tokens") or 0
            t["out"] += u.get("completion_tokens") or 0
            if str(r.get("status") or "") != "ok":
                bad.append("%s:%s:%s:%s" % (key, r.get("seq"), r.get("status"), r.get("http_status")))
    return per, bad


alive = subprocess.run(["ps", "-p", PID, "-o", "pid=,etime=,stat="],
                       capture_output=True, text=True).stdout.strip()
per, bad = calls()
tc = sum(v["calls"] for v in per.values())
ti = sum(v["in"] for v in per.values())
to = sum(v["out"] for v in per.values())
spend = ti / 1e6 * 0.10 + to / 1e6 * 0.50
now = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
arm = collections.defaultdict(int)
for k, v in per.items():
    arm[k.split("-")[0]] += v["calls"]
row = {
    "utc": now,
    "pid": alive or "GONE",
    "cells_with_ledger": len(per),
    "calls_total": tc,
    "per_arm": dict(arm),
    "in_tok": ti,
    "out_tok": to,
    "spend_usd": round(spend, 4),
    "non_ok": bad,
}
with open(".scratch/39-m2c-matrix/phase-b-poll.log", "a") as f:
    f.write(json.dumps(row, sort_keys=True) + "\n")
print(json.dumps(row, indent=2, sort_keys=True))
