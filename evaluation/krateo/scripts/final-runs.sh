#!/usr/bin/env bash
# Final measurements, all on the same cluster state (the ground truth of scenario/ground-truth.csv):
#   KubePattern r1-r3, Kyverno equivalent r1-r3, Kyverno best-effort r1-r3, and the verbatim run of the given
#   panel-not-referenced (v1-0-10) for both tools. Reports are reset whenever the policy set changes
#   (stale results of no-longer-matched resources would otherwise remain, see README).
set -euo pipefail
KRATEO="$(cd "$(dirname "$0")/.." && pwd)"
EVAL="$(cd "$KRATEO/.." && pwd)"
source "$KRATEO/env/krateo.env"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
S="$EVAL/scripts"; K="$KRATEO/kyverno"
reset_reports() {
  kubectl delete policyreports.wgpolicyk8s.io -A --all >/dev/null
  kubectl delete clusterpolicyreports.wgpolicyk8s.io --all >/dev/null
}
policy_set() {  # equivalent | best-effort
  kubectl delete validatingpolicies.policies.kyverno.io krateo-restaction-manager-missing --ignore-not-found >/dev/null
  kubectl apply -f "$K/equivalent/" >/dev/null
  [[ "$1" == best-effort ]] && kubectl apply -f "$K/best-effort/" >/dev/null
  reset_reports
}

kubectl apply -f "$KRATEO/patterns/" >/dev/null
kubectl apply -f "$K/globalcontext/" >/dev/null
for r in 1 2 3; do "$S/run-once.sh" "krateo/kp-r$r" | tail -1; done

policy_set equivalent
for r in 1 2 3; do "$S/kyverno-run.sh" "krateo/equivalent-r$r" 2>/dev/null | tail -1; done
policy_set best-effort
for r in 1 2 3; do "$S/kyverno-run.sh" "krateo/best-effort-r$r" 2>/dev/null | tail -1; done

echo "==> verbatim panel-not-referenced (PortalBlueprintPage v1-0-10)"
policy_set equivalent
kubectl apply -f "$K/verbatim/" >/dev/null
reset_reports
kubectl apply -f "$KRATEO/patterns/verbatim/" >/dev/null
"$S/run-once.sh" krateo/kp-verbatim-final | tail -1
SINCE=0 TIMEOUT=900 "$S/kyverno-run.sh" krateo/verbatim 2>/dev/null | tail -1 || echo "kyverno verbatim: reports did not settle (see README)"
# restore the scored state
kubectl apply -f "$KRATEO/patterns/" >/dev/null
policy_set equivalent
kubectl delete globalcontextentries.kyverno.io portalblueprintpages-v1-0-10.composition.krateo.io --ignore-not-found >/dev/null
