#!/bin/sh
# Stage E cell: R@medium, cell f10-s0, H=144, live. Declared in ../spec.md.
set -e
cd /home/z/vibe/agentworld
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_MODEL=github_copilot/gpt-6-luna
exec .tmp/58-vs2 -arms R -cells f10-s0 -reps 3 -hours 144 -effort medium \
  -call-concurrency 10 -max-output 32768 -response-bound 1048576 -attempt-timeout 300 -call-deadline 900 \
  -claim-contract natural-claim/v4 -live -parallel 3 -budget-min 600 \
  -root .scratch/58-w3-effort/medium/runs -out .scratch/58-w3-effort/medium/evaluation.json -out-txt .scratch/58-w3-effort/medium/evaluation.txt
