#!/bin/sh
# Stage M, model github_copilot/gpt-6-sol: arm R, cell f10-s0, 3 replicates, H=144, effort max,
# natural-claim/v4, responses wire. Declared in ../58-w3-preregistration/spec.md §4 Stage M.
set -e
cd /home/z/vibe/agentworld
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
export LITELLM_MODEL=github_copilot/gpt-6-sol
exec .tmp/60-vs2 -model github_copilot/gpt-6-sol -wire responses -arms R -cells f10-s0 -reps 3 \
  -hours 144 -effort max -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 3 -budget-min 600 \
  -root .scratch/60-w3-model/gpt-6-sol/runs \
  -out .scratch/60-w3-model/gpt-6-sol/evaluation.json \
  -out-txt .scratch/60-w3-model/gpt-6-sol/evaluation.txt
