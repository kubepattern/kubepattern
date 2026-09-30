#!/usr/bin/env bash
# Samples per-container CPU (cumulative seconds) and memory working set of every pod in NAMESPACE from
# the kubelet resource metrics, every INTERVAL seconds for DURATION seconds (RQ6c).
# Output TSV: epoch  pod  container  cpu_seconds_total  memory_working_set_bytes
# Usage: NAMESPACE=kyverno footprint-sample.sh <out.tsv> <duration-s> [interval-s]
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
out="${1:?usage: footprint-sample.sh <out.tsv> <duration-s> [interval-s]}"
duration="${2:?}"; interval="${3:-10}"; ns="${NAMESPACE:-kyverno}"
printf 'epoch\tpod\tcontainer\tcpu_seconds_total\tmemory_working_set_bytes\n' > "$out"
end=$(( $(date +%s) + duration ))
while (( $(date +%s) < end )); do
  t=$(date +%s)
  kubectl --context "$KP_PROFILE" get --raw "/api/v1/nodes/$KP_PROFILE/proxy/metrics/resource" \
    | awk -v t="$t" -v ns="$ns" '
        /^container_(cpu_usage_seconds_total|memory_working_set_bytes)\{/ {
          if ($0 !~ "namespace=\"" ns "\"") next
          match($0, /pod="[^"]+"/); pod = substr($0, RSTART + 5, RLENGTH - 6)
          match($0, /container="[^"]+"/); c = substr($0, RSTART + 11, RLENGTH - 12)
          k = pod "\t" c
          if ($1 ~ /^container_cpu/) cpu[k] = $2; else mem[k] = $2
        }
        END { for (k in cpu) printf "%s\t%s\t%s\t%.0f\n", t, k, cpu[k], mem[k] }' >> "$out"
  sleep "$interval"
done
