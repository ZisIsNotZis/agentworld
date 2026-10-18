import json,glob,collections,sys
per=collections.defaultdict(lambda: {"calls":0,"in":0,"out":0})
bad=collections.Counter()
for p in sorted(glob.glob('.tmp/39-m2c-runs/*/calls.jsonl')):
    key=p.split('/')[-2]
    for line in open(p):
        try: r=json.loads(line)
        except Exception: continue
        if r.get('kind')!='call': continue
        t=per[key]; t["calls"]+=1
        u=r.get('usage') or {}
        t["in"]+=u.get('prompt_tokens') or 0
        t["out"]+=u.get('completion_tokens') or 0
        st=str(r.get('status') or '')
        if st!='ok': bad["%s:%s:%s:%s"%(key,r.get('seq'),st,r.get('http_status'))]+=1
tc=ti=to=0
print("%-16s %6s %9s %9s"%("cell","calls","in_tok","out_tok"))
for k in sorted(per):
    t=per[k]; tc+=t["calls"]; ti+=t["in"]; to+=t["out"]
    print("%-16s %6d %9d %9d"%(k,t["calls"],t["in"],t["out"]))
print("%-16s %6d %9d %9d"%("TOTAL",tc,ti,to))
print("spend_usd=%.4f"%(ti/1e6*0.10+to/1e6*0.50))
if bad: print("non-ok:",dict(bad))
