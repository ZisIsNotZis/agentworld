#!/bin/sh
# Stage-M pre-flight smoke: does the model accept the declared request and answer
# inside natural-claim/v4? Two simulated hours, one replicate, arm R, one call
# currency. NOT a cell: no KPI from these runs enters the record.
set -e
cd /home/z/vibe/agentworld
. .tmp/go-env.sh
export VS2_SPEND_AUTHORIZATION=AUTHORIZED-VS2-SPEND
export LITELLM_BASE_URL=http://127.0.0.1:4000
model="$1"; dir="$2"
exec .tmp/60-vs2 -model "$model" -wire responses -arms R -cells f10-s0 -reps 1 -hours 2 \
  -effort max -call-concurrency 10 -max-output 32768 -response-bound 1048576 \
  -attempt-timeout 300 -call-deadline 900 -claim-contract natural-claim/v4 \
  -live -parallel 1 -budget-min 10 -replay=false \
  -root "$dir/smoke/runs" -out "$dir/smoke/evaluation.json" -out-txt "$dir/smoke/evaluation.txt"
