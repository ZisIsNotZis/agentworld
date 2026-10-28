#!/bin/sh
# Stage S cell: arms A0,R,P,U,A on f11-s2, H=144, N=256 (budgeted P/U/A), effort max, live.
# Declared in ../58-w3-preregistration/spec.md §4 Stage S as amended (2026-10-28):
# f11-s2 only. R@max is drawn in this cell (the in-run reference), not carried in.
set -e
cd /home/z/vibe/agentworld
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/gpt-6-luna
exec .tmp/61-vs2 -arms A0,R,P,U,A -cells f11-s2 -reps 3 -hours 144 -effort max \
  -call-concurrency 10 -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 3 -budget-min 600 \
  -root .scratch/61-w3-stability/runs \
  -out .scratch/61-w3-stability/evaluation.json -out-txt .scratch/61-w3-stability/evaluation.txt
