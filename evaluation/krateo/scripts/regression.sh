#!/usr/bin/env bash
# Regression of a new engine on the Krateo case study, with spec.for unset: 3 in-cluster scored runs (expected
# 55/0/0/121, 25/25 probes) and the composition lifecycle (expected 26/26, reverted == baseline).
# The image must already be in the profile (minikube -p kp-krateo image load <tar>).
# The lifecycle re-creates demo-app-2, which renames the CDC's RoleBindings, so the ground truth is regenerated
# at the end (krateo_oracle.py truth) and its diff is printed: only RoleBinding renames are expected.
# Results: measurements/regression/<commit>/krateo/. Usage: regression.sh <commit>
set -uo pipefail
KRATEO="$(cd "$(dirname "$0")/.." && pwd)"
EVAL="$(cd "$KRATEO/.." && pwd)"
C="${1:?usage: regression.sh <commit>}"
source "$KRATEO/env/krateo.env"
export KP_COMMIT="$C"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
S="$EVAL/scripts"; OUT="$EVAL/measurements/regression/$C/krateo"; mkdir -p "$OUT"
step() { printf '\n==> [%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

step "install $C on $KP_PROFILE (CRDs re-applied)"
kubectl apply -f "$EVAL/../charts/kubepattern/crds/" >/dev/null
KP_SKIP_LOAD=1 "$EVAL/env/install/00-kubepattern.sh" >/dev/null && echo "installed $KP_IMAGE"
snap="$EVAL/measurements/raw/krateo/regr-$C-snapshot"
"$KRATEO/scripts/krateo_oracle.py" snapshot "$snap" >/dev/null
echo "oracle before: $("$KRATEO/scripts/krateo_oracle.py" check "$snap/objects.json" | tail -1)" | tee "$OUT/oracle-before.txt"

step "3 in-cluster scored runs"
for r in 1 2 3; do
  dir="$("$S/run-once.sh" "krateo/regr-$C-r$r" | tail -1)"
  "$S/score.py" "$dir/smells.json" --truth "$KRATEO/scenario/ground-truth.csv" | tail -1
done | tee "$OUT/scored.txt"

step "composition lifecycle (KubePattern $C and Kyverno 1:1)"
LIFECYCLE_LABEL="lifecycle-regr-$C" "$KRATEO/scripts/lifecycle.sh"
LIFECYCLE_LABEL="lifecycle-regr-$C" "$KRATEO/scripts/lifecycle_report.py" | tee "$OUT/lifecycle.txt"

step "ground truth after the lifecycle"
snap2="$EVAL/measurements/raw/krateo/regr-$C-snapshot-after"
"$KRATEO/scripts/krateo_oracle.py" snapshot "$snap2" >/dev/null
"$KRATEO/scripts/krateo_oracle.py" truth "$snap2/objects.json" | tail -1
git -C "$EVAL/.." diff --stat -- evaluation/krateo/scenario/ | tee "$OUT/ground-truth-diff.txt"
step done
