#!/usr/bin/env bash
# RQ6d / H5: detection latency after a dependency-side change, KubePattern (CronJob) vs Kyverno
# (background scan) at equal periods. Both tools observe the same change at the same time.
#   change   delete ScheduledBackup kp-db-a/db-live-daily (M02): Cluster db-live becomes a smell
#   reverse  re-create it: the smell disappears
# Each change is applied at a random offset in [0, PERIOD) s after the previous one was observed by
# both tools. Latency = first poll (every POLL s) showing the new state - time of the change.
# Output: results/raw/kyverno/staleness/<run>/{events.csv,jobs.json,cronjob.json}
# Preconditions (set by the caller): KubePattern CronJob unsuspended on the same period as Kyverno's
# backgroundScanInterval. Usage: REPS=5 PERIOD=300 staleness.sh
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
: "${REPS:=5}" "${PERIOD:=300}" "${POLL:=2}" "${TIMEOUT:=1200}"
PATTERN=cnpg-cluster-without-scheduledbackup NS=kp-db-a TARGET=db-live
out="$EVAL/results/raw/kyverno/staleness/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$out"
echo "rep,direction,offset_s,t_change,tool,t_observed,latency_s" > "$out/events.csv"

kp_smell() {
  kubectl get smells.kubepattern.dev -n "$NS" -o json | jq --arg p "$PATTERN" --arg t "$TARGET" \
    '[.items[] | select(.spec.pattern.name == $p and .spec.target.name == $t)] | length > 0'
}
kyv_smell() {
  kubectl get policyreports.wgpolicyk8s.io -n "$NS" -o json | jq --arg p "$PATTERN" --arg t "$TARGET" \
    '[.items[] | select(.scope.kind == "Cluster" and .scope.name == $t) | (.results // [])[]
      | select(.policy == $p and .result == "fail")] | length > 0'
}
backup() {
  case "$1" in
    delete) kubectl -n "$NS" delete scheduledbackups.postgresql.cnpg.io db-live-daily >/dev/null ;;
    create) cat <<'YAML' | kubectl apply -f - >/dev/null ;;
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: db-live-daily
  namespace: kp-db-a
spec:
  schedule: "0 0 0 1 1 *"
  cluster:
    name: db-live
YAML
  esac
}
# Waits until both tools report `want` (true = smell), logging the first observation of each.
observe() {
  local rep="$1" dir="$2" off="$3" t0="$4" want="$5" kp="" kyv=""
  while [[ -z "$kp" || -z "$kyv" ]]; do
    local now; now=$(date +%s.%N)
    if [[ -z "$kp" && "$(kp_smell)" == "$want" ]]; then kp=$now; echo "$rep,$dir,$off,$t0,kubepattern,$kp,$(echo "$kp - $t0" | bc)" >> "$out/events.csv"; fi
    if [[ -z "$kyv" && "$(kyv_smell)" == "$want" ]]; then kyv=$now; echo "$rep,$dir,$off,$t0,kyverno,$kyv,$(echo "$kyv - $t0" | bc)" >> "$out/events.csv"; fi
    if (( ${now%.*} - ${t0%.*} > TIMEOUT )); then echo "timeout rep=$rep dir=$dir" >&2; return 1; fi
    sleep "$POLL"
  done
  printf 'rep %s %-7s offset=%3ss  kubepattern=%6.1fs  kyverno=%6.1fs\n' "$rep" "$dir" "$off" \
    "$(echo "$kp - $t0" | bc)" "$(echo "$kyv - $t0" | bc)" >&2
}

# Start from the clean state, observed by both tools.
[[ "$(kp_smell)" == false && "$(kyv_smell)" == false ]] || { echo "start state is not clean" >&2; exit 1; }
for rep in $(seq 1 "$REPS"); do
  for dir in appear disappear; do
    off=$(( RANDOM % PERIOD )); sleep "$off"
    t0=$(date +%s.%N)
    if [[ $dir == appear ]]; then backup delete; observe "$rep" "$dir" "$off" "$t0" true
    else backup create; observe "$rep" "$dir" "$off" "$t0" false; fi
  done
done
kubectl -n "$KP_NAMESPACE" get jobs -o json > "$out/jobs.json"
kubectl -n "$KP_NAMESPACE" get cronjob "${KP_RELEASE}-cronjob" -o json > "$out/cronjob.json"
echo "$out"
