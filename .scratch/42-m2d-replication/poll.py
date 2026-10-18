import glob
import json
import os
import sys
import time

# Ticket 42 liveness/spend poll. Takes one run root (one cell's invocations) and tallies per
# cell-directory: kind == "call" rows are charged attempts (not decisions), and spend is the
# operator-side estimate at $0.10/M input and $0.50/M output — the gateway exposes no price, so the
# report's own cost field is UNPRICED and the dollar cap is enforced here.
root = sys.argv[1] if len(sys.argv) > 1 else '.tmp/42-runs-f12-s0'
prev = sys.argv[2] if len(sys.argv) > 2 else None   # previous cumulative spend, for the realised rate
IN_USD, OUT_USD = 0.10, 0.50

per = {}
bad = []
for p in sorted(glob.glob(root + '/*/calls.jsonl')):
    key = p.split('/')[-2]
    t = per.setdefault(key, {"calls": 0, "in": 0, "out": 0, "last_mtime": 0.0})
    for line in open(p):
        try:
            r = json.loads(line)
        except Exception:
            continue
        if r.get('kind') != 'call':
            continue
        t["calls"] += 1
        u = r.get('usage') or {}
        t["in"] += u.get('prompt_tokens') or 0
        t["out"] += u.get('completion_tokens') or 0
        st = str(r.get('status') or '')
        if st != 'ok':
            bad.append("%s:%s:%s:%s" % (key, r.get('seq'), st, r.get('http_status')))
    t["last_mtime"] = os.path.getmtime(p)

tc = ti = to = 0
print("%-16s %6s %9s %9s %8s" % ("cell", "calls", "in_tok", "out_tok", "idle_s"))
now = time.time()
for k in sorted(per):
    t = per[k]
    tc += t["calls"]; ti += t["in"]; to += t["out"]
    print("%-16s %6d %9d %9d %8.0f" % (k, t["calls"], t["in"], t["out"], now - t["last_mtime"]))
print("%-16s %6d %9d %9d" % ("TOTAL", tc, ti, to))
spend = ti / 1e6 * IN_USD + to / 1e6 * OUT_USD
print("spend_usd=%.4f" % spend)
print("per_attempt_usd=%.3e" % (spend / tc if tc else 0.0))
print("cell_dirs=%d with_ledger=%d" % (len(glob.glob(root + '/*')), len(per)))
if prev is not None:
    d = spend - float(prev)
    print("delta_usd=%.4f" % d)
if bad:
    print("non-ok:", bad)
