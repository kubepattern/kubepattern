#!/usr/bin/env bash
# Harvests apiserver audit events (JSON lines on the apiserver's stdout) for DURATION seconds,
# every INTERVAL seconds, so that kubelet log rotation cannot drop events of long windows (RQ6c).
# Events are de-duplicated by (auditID, stage).
# Usage: audit-harvest.sh <out.jsonl> <duration-s> [interval-s]
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
out="${1:?usage: audit-harvest.sh <out.jsonl> <duration-s> [interval-s]}"
duration="${2:?}"; interval="${3:-120}"
raw="$out.raw"; : > "$raw"
end=$(( $(date +%s) + duration ))
since="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
while :; do
  now=$(date +%s); next="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  (( now >= end )) && last=1 || last=0
  kubectl --context "$KP_PROFILE" -n kube-system logs "kube-apiserver-$KP_PROFILE" --since-time="$since" \
    | grep '^{"kind":"Event"' >> "$raw" || true
  since="$(date -u -d "@$(( now - 5 ))" +%Y-%m-%dT%H:%M:%SZ)"   # 5 s overlap, removed by the de-duplication
  (( last )) && break
  sleep $(( interval < end - now ? interval : end - now ))
done
jq -c -s 'unique_by([.auditID, .stage]) | sort_by(.requestReceivedTimestamp) | .[]' "$raw" > "$out"
rm -f "$raw"
echo "$(wc -l < "$out") events -> $out" >&2
