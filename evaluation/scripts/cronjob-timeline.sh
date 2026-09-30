#!/usr/bin/env bash
# RQ5: KubePattern in CronJob mode (snapshot semantics). The CronJob runs every 2 minutes;
# after each Job the Smells are archived and the next scripted event is applied:
#   after run 1  E1 fix     remove 3 orphans and give a Stage to a 4th Warehouse
#   after run 2  E2 inject  3 new unused objects
#   after run 3  E3 R1      add a Pattern whose CRD is not installed
#   after run 4  E4 RBAC    narrow read access (no kargo.akuity.io)
#   after run 5  E5 RBAC    restore wildcard read access
#   after run 6  E6 churn   rotation step 1: create the next DockerMachineTemplate
#   after run 7  E6 churn   rotation step 2: switch md-rotated to it
#   after run 8  E7 revert  back to the seeded scenario
#   run 9        end
# Output: results/raw/$TIMELINE_LABEL/run-N/{job.json,pod.log,smells.json,audit.jsonl}
#         (TIMELINE_LABEL defaults to "cronjob"; use e.g. cronjob-fix for another engine version)
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
RAW="$EVAL/results/raw/${TIMELINE_LABEL:-cronjob}"
RUNS=9
CHART="$EVAL/../charts/kubepattern"
helm_kp() { helm upgrade "$KP_RELEASE" "$CHART" -n "$KP_NAMESPACE" --reuse-values "$@" >/dev/null; }

event() {
  case "$1" in
    1) echo "E1 fix"
       kubectl delete clusterissuer ci-unused
       kubectl -n kp-flux-a delete ocirepository orphan-1
       kubectl -n kp-capi-a delete dockermachinetemplate dmt-orphan
       cat <<'YAML' | kubectl apply -f -
apiVersion: kargo.akuity.io/v1alpha1
kind: Stage
metadata:
  name: orphan-consumer
  namespace: kp-kargo-1
spec:
  requestedFreight:
    - origin: {kind: Warehouse, name: orphan}
      sources: {direct: true}
YAML
       ;;
    2) echo "E2 inject"
       cat <<'YAML' | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ci-new-unused
spec:
  selfSigned: {}
---
apiVersion: kargo.akuity.io/v1alpha1
kind: Warehouse
metadata:
  name: new-unused
  namespace: kp-kargo-1
spec:
  interval: 24h
  freightCreationPolicy: Manual
  subscriptions:
    - image: {repoURL: public.ecr.aws/nginx/nginx, constraint: ^1.27.0}
---
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata:
  name: ss-new-unused
  namespace: kp-secrets-a
spec:
  provider:
    fake:
      data: [{key: db-password, value: s3cr3t}]
YAML
       ;;
    3) echo "E3 add R1 (CRD not installed)"; kubectl apply -f "$EVAL/patterns/_robustness/" ;;
    4) echo "E4 narrow RBAC"; helm_kp -f "$EVAL/env/values/rbac-without-kargo.yaml" ;;
    5) echo "E5 restore RBAC"; helm_kp -f "$EVAL/env/values/rbac-wildcard.yaml" ;;
    6) echo "E6 rotation step 1"
       cat <<'YAML' | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: DockerMachineTemplate
metadata:
  name: dmt-rotated-v3
  namespace: kp-capi-a
spec:
  template:
    spec: {}
YAML
       ;;
    7) echo "E6 rotation step 2"
       kubectl -n kp-capi-a patch machinedeployment md-rotated --type=merge \
         -p '{"spec":{"template":{"spec":{"infrastructureRef":{"name":"dmt-rotated-v3"}}}}}' ;;
    8) echo "E7 revert"
       kubectl delete -f "$EVAL/patterns/_robustness/"
       kubectl delete clusterissuer ci-new-unused
       kubectl -n kp-kargo-1 delete warehouse new-unused
       kubectl -n kp-kargo-1 delete stage orphan-consumer
       kubectl -n kp-secrets-a delete secretstore ss-new-unused
       "$EVAL/scripts/apply-scenario.sh" >/dev/null 2>&1
       kubectl -n kp-capi-a delete dockermachinetemplate dmt-rotated-v3 ;;
  esac
}

snapshot() {  # $1 job name, $2 run number
  local out="$RAW/run-$2"; mkdir -p "$out"
  kubectl -n "$KP_NAMESPACE" get job "$1" -o json > "$out/job.json"
  kubectl -n "$KP_NAMESPACE" logs "job/$1" > "$out/pod.log"
  kubectl get smells.kubepattern.dev -A -o json > "$out/smells.json"
  local since; since="$(jq -r .status.startTime "$out/job.json")"
  kubectl -n kube-system logs "kube-apiserver-$KP_PROFILE" --since-time="$since" | grep '^{"kind":"Event"' \
    | jq -c --arg sa "$KP_SA" 'select(.user.username == $sa)' > "$out/audit.jsonl" || true
  echo "run $2: job $1 smells=$(jq '.items|length' "$out/smells.json") $(date -u +%H:%M:%S)"
}

rm -rf "$RAW"; mkdir -p "$RAW"
start="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
helm_kp --set schedule="*/2 * * * *" --set suspend=false
echo "CronJob enabled (*/2), waiting for runs..."
declare -A seen
n=0
while (( n < RUNS )); do
  # scheduled Jobs only (manual ones from run-once.sh are named kp-*), created after the start
  for job in $(kubectl -n "$KP_NAMESPACE" get jobs -o json | jq -r --arg p "${KP_RELEASE}-cronjob-" --arg t "$start" '
      .items | map(select((.metadata.name | startswith($p)) and .metadata.creationTimestamp > $t and (.status.succeeded // 0) >= 1))
      | sort_by(.metadata.creationTimestamp) | .[].metadata.name'); do
    [[ -n "${seen[$job]:-}" ]] && continue
    seen[$job]=1; n=$((n + 1))
    snapshot "$job" "$n"
    (( n < RUNS )) && event "$n"
  done
  sleep 5
done
helm_kp --set suspend=true
echo "CronJob suspended again."
