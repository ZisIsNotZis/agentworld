#!/bin/sh
# Stage T cell: arms A0,P,U,A on f10-s0, H=144, N=256 (budgeted P/U/A), effort max, live.
# Declared in ../58-w3-preregistration/spec.md §4 Stage T. R is NOT run here: the three
# Stage-E R@max replicates are the reference (../58-w3-effort/spec.md §1).
set -e
cd /home/z/vibe/agentworld
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/gpt-6-luna
exec .tmp/59-vs2 -arms A0,P,U,A -cells f10-s0 -reps 3 -hours 144 -effort max \
  -call-concurrency 10 -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 3 -budget-min 600 \
  -root .scratch/59-w3-timing/runs \
  -out .scratch/59-w3-timing/evaluation.json -out-txt .scratch/59-w3-timing/evaluation.txt
