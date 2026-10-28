#!/usr/bin/env python3
"""Stage M admissibility probe: which `reasoning.effort` values does each declared
responses-wire model accept?

Why it exists: the Stage-M smoke cell for `github_copilot/grok-4.7` returned HTTP
400 on every call (see `grok-4.7/smoke/evaluation.txt`). This probe isolates the
offending request field so the record can state the exact failure instead of
guessing at it. It sends the minimal `/v1/responses` body (model + one user turn
+ the effort under test) and prints the status. It is a wire probe, not a cell:
no KPI, no world, no ledger, and nothing from it enters the stage's comparison.

Cost: one short call per (model, effort) pair at the gateway's own minimum; all
models are recorded unpriced in this gateway.

Usage: python3 grok-admissibility.py > grok-admissibility.txt
"""
import json
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:4000"
INPUT = [{"role": "user", "content": [{"type": "input_text", "text": "Reply with {\"ok\":true}"}]}]
MODELS = ["github_copilot/grok-4.7", "github_copilot/gpt-6-sol"]
EFFORTS = ["none", "minimal", "low", "medium", "high", "xhigh", "max"]


def post(path, body, timeout=180):
    req = urllib.request.Request(BASE + path, data=json.dumps(body).encode(),
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            d = json.load(r)
            usage = d.get("usage") or {}
            return r.status, d.get("status"), usage.get("output_tokens")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()[:160], None


def main():
    print("base: %s  path: /v1/responses  input: one user turn" % BASE)
    print("\n# 1. the offending field: do the harness's other request fields work?")
    full = {"model": "github_copilot/grok-4.7", "input": INPUT,
            "max_output_tokens": 32768, "reasoning": {"effort": "max"},
            "text": {"format": {"type": "json_object"}}}
    for drop in [None, "reasoning", "max_output_tokens", "text"]:
        body = dict(full)
        if drop:
            body.pop(drop)
        print("  grok-4.7 full%s -> %s" % ("" if drop is None else " minus " + drop, post("/v1/responses", body)))

    print("\n# 2. the effort matrix (model x reasoning.effort)")
    print("  %-28s %-8s %-6s %-12s %s" % ("model", "effort", "http", "status", "output_tokens"))
    for model in MODELS:
        for eff in EFFORTS:
            code, status, out = post("/v1/responses",
                                     {"model": model, "input": INPUT, "reasoning": {"effort": eff}})
            print("  %-28s %-8s %-6s %-12s %s" % (model, eff, code, status, out))


if __name__ == "__main__":
    main()
