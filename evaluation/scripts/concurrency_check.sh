#!/usr/bin/env bash
# D8 check: two overlapping analysis runs (an in-cluster Job and the local binary, both at
# $KP_COMMIT) must not delete each other's Smells. After both finish, the Smell snapshot is
# scored against the ground truth: exit code 0 means no Smell was lost.
# Usage: [KP_COMMIT=<sha>] concurrency_check.sh
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
out="$EVAL/measurements/raw/concurrency-$KP_COMMIT/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$out"

# The local run starts 5 s after the Job, so both write and prune in overlapping windows.
# CONC_WAIT=ready starts it as soon as the Job's container runs instead: a run at 50 QPS takes
# a few seconds, so a fixed 5 s delay may not overlap at all.
job="kp-conc-$(date -u +%H%M%S)"
kubectl -n "$KP_NAMESPACE" create job "$job" --from="cronjob/${KP_RELEASE}-cronjob" >/dev/null
if [[ "${CONC_WAIT:-}" == ready ]]; then
  until [[ "$(kubectl -n "$KP_NAMESPACE" get pods -l "job-name=$job" -o jsonpath='{.items[0].status.containerStatuses[0].state.running.startedAt}' 2>/dev/null)" ]]; do sleep 0.2; done
else
  sleep 5
fi
"$EVAL/scripts/perf-local.sh" "concurrency-$KP_COMMIT" > "$out/local.json"
kubectl -n "$KP_NAMESPACE" wait "job/$job" --for=condition=Complete --timeout=10m >/dev/null
kubectl -n "$KP_NAMESPACE" logs "job/$job" > "$out/job.log"
kubectl get smells.kubepattern.dev -A -o json > "$out/smells.json"

echo "overlapping runs at $KP_COMMIT: $(jq '.items | length' "$out/smells.json") smells after both finished"
"$EVAL/scripts/score.py" "$out/smells.json" | tail -1
