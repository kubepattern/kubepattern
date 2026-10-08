#!/usr/bin/env bash
# Composition lifecycle (the Krateo counterpart of mutate.sh): six mutations applied through Krateo
# (composition values, a deleted referrer, a new endpoint Secret, a deleted composition, a deleted
# definition), then reverted. At each phase both tools run once:
#   KubePattern: scripts/run-once.sh   -> measurements/raw/krateo/lifecycle-<phase>/
#   Kyverno:     scripts/kyverno-run.sh -> measurements/raw/kyverno/krateo/lifecycle-<phase>/ (equivalent policy set)
# plus an oracle snapshot of the state. Phases: baseline, mutated, reverted. Report: lifecycle_report.py.
set -euo pipefail
KRATEO="$(cd "$(dirname "$0")/.." && pwd)"
EVAL="$(cd "$KRATEO/.." && pwd)"
source "$KRATEO/env/krateo.env"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
S="$EVAL/scripts"; SC="$KRATEO/scenario"
: "${LIFECYCLE_LABEL:=lifecycle}"

phase() {
  local p="$1"
  "$KRATEO/scripts/krateo_oracle.py" snapshot "$EVAL/measurements/raw/krateo/$LIFECYCLE_LABEL-$p/oracle" >/dev/null
  "$S/run-once.sh" "krateo/$LIFECYCLE_LABEL-$p" >/dev/null
  "$S/kyverno-run.sh" "krateo/$LIFECYCLE_LABEL-$p" >/dev/null 2>&1
  echo "phase $p done"
}
ready_release() {  # namespace composition: the CDC reports Ready even for a failed release, so check Helm
  local ns="$1" c="$2"
  kubectl -n "$ns" wait "springbootapps.composition.krateo.io/$c" --for=condition=Ready --timeout=10m >/dev/null
  until helm -n "$ns" list -a -o json | jq -e --arg c "$c" 'map(select(.name | startswith($c + "-"))) | length == 1 and .[0].status == "deployed"' >/dev/null; do sleep 3; done
}

echo "==> equivalent policy set"
kubectl delete validatingpolicies.policies.kyverno.io krateo-restaction-manager-missing --ignore-not-found >/dev/null
kubectl apply -f "$KRATEO/kyverno/equivalent/" >/dev/null
# Narrowing a policy's match (best-effort N4 covers 3 kinds, the 1:1 one only Markdown) leaves the old results
# of the no-longer-matched resources in the PolicyReports, and the snapshot never settles: start from empty reports.
kubectl delete policyreports.wgpolicyk8s.io -A --all >/dev/null
kubectl delete clusterpolicyreports.wgpolicyk8s.io --all >/dev/null
phase baseline

echo "==> mutations L1-L6"
kubectl apply -f "$SC/lifecycle/demo-app-mutated.yaml"                                  # L1, L3
kubectl -n kp-krateo-a delete panels.widgets.templates.krateo.io panel-links            # L2
kubectl apply -f "$SC/lifecycle/prometheus-endpoint.yaml"                               # L4
kubectl -n tenant-b delete springbootapps.composition.krateo.io demo-app-2              # L5
kubectl -n kp-krateo-a delete compositiondefinitions.core.krateo.io github-scaffolding-with-composition-page  # L6
# wait until Krateo has converged
until ! kubectl -n demo-app get restactions.templates.krateo.io cve-table >/dev/null 2>&1; do sleep 3; done
until ! kubectl -n demo-app get restactions.templates.krateo.io replicaset-demo-app-backend-86ccc4747f-demo-app >/dev/null 2>&1; do sleep 3; done
kubectl wait --for=delete namespace/demo-app-2 --timeout=10m 2>/dev/null || true
until ! kubectl -n kp-krateo-a get compositiondefinitions.core.krateo.io github-scaffolding-with-composition-page >/dev/null 2>&1; do sleep 3; done
phase mutated

echo "==> revert"
kubectl apply -f "$SC/compositions.yaml"
kubectl apply --server-side --force-conflicts -f "$SC/seeds.yaml" >/dev/null
kubectl delete -f "$SC/lifecycle/prometheus-endpoint.yaml"
kubectl apply -f "$SC/seeds-definitions.yaml"
ready_release tenant-a demo-app
ready_release tenant-b demo-app-2
until kubectl -n demo-app get restactions.templates.krateo.io cve-table >/dev/null 2>&1; do sleep 3; done
until kubectl -n demo-app-2 get navmenuitems.widgets.templates.krateo.io demo-app-2-nav-menu-item >/dev/null 2>&1; do sleep 3; done
kubectl -n kp-krateo-a wait compositiondefinitions.core.krateo.io/github-scaffolding-with-composition-page --for=condition=Ready --timeout=10m
phase reverted
