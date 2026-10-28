#!/bin/sh
# Stage M, model github_copilot/grok-4.7 — DECLARED BUT NOT RUN TO A CELL.
#
# This is the pre-registration's §4 Stage-M command for the third declared model,
# kept executable so the refusal is reproducible. It never ran as a cell: the
# pre-flight smoke (`smoke.sh`, 2 h, 1 replicate, same flags) returned HTTP 400 on
# all 20 calls with zero prompt/completion tokens and zero applied model
# decisions, because the upstream refuses `reasoning.effort = "max"` for this
# model (`grok-admissibility.txt`). Running the full cell would produce ~4,300
# level-1 fallbacks and three no-AI trajectories under a model's name; §4 of the
# stage record declines it and states the exact failure instead of substituting a
# model or an effort.
#
# Reproduce the refusal (cheap, no cell):
#   python3 .scratch/60-w3-model/grok-admissibility.py
#   sh .scratch/60-w3-model/smoke.sh github_copilot/grok-4.7 .scratch/60-w3-model/grok-4.7
set -e
cd /home/z/vibe/agentworld
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/grok-4.7
exec .tmp/60-vs2 -model github_copilot/grok-4.7 -wire responses -arms R -cells f10-s0 -reps 3 \
  -hours 144 -effort max -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 3 -budget-min 600 \
  -root .scratch/60-w3-model/grok-4.7/runs \
  -out .scratch/60-w3-model/grok-4.7/evaluation.json \
  -out-txt .scratch/60-w3-model/grok-4.7/evaluation.txt
