#!/usr/bin/env bash
# Triggers one analysis run from the (suspended) CronJob and archives everything it produced:
#   measurements/raw/<label>/<run-id>/{job.json,pod.log,smells.json,audit.jsonl,summary.json}
# Usage: run-once.sh <label>
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
label="${1:?usage: run-once.sh <label>}"
run_id="$(date -u +%Y%m%dT%H%M%SZ)"
out="$EVAL/measurements/raw/$label/$run_id"
mkdir -p "$out"

since="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
job="kp-${run_id,,}"
kubectl -n "$KP_NAMESPACE" create job "$job" --from="cronjob/${KP_RELEASE}-cronjob" >/dev/null
kubectl -n "$KP_NAMESPACE" wait "job/$job" --for=condition=Complete --timeout=15m >/dev/null

kubectl -n "$KP_NAMESPACE" get job "$job" -o json > "$out/job.json"
kubectl -n "$KP_NAMESPACE" logs "job/$job" > "$out/pod.log"
kubectl get smells.kubepattern.dev -A -o json > "$out/smells.json"
# Audit events of the KubePattern ServiceAccount (apiserver stdout, JSON lines).
sleep 2
kubectl -n kube-system logs "kube-apiserver-$KP_PROFILE" --since-time="$since" \
  | grep '^{"kind":"Event"' \
  | jq -c --arg sa "$KP_SA" 'select(.user.username == $sa)' > "$out/audit.jsonl" || true

jq -n --arg label "$label" --arg run "$run_id" --arg job "$job" \
  --slurpfile j "$out/job.json" --slurpfile s "$out/smells.json" \
  --argjson calls "$(wc -l < "$out/audit.jsonl")" '{
    label: $label, run: $run, job: $job,
    start: $j[0].status.startTime, completion: $j[0].status.completionTime,
    smells: ($s[0].items | length), apiRequests: $calls }' | tee "$out/summary.json"
echo "$out"
