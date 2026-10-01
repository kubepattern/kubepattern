#!/usr/bin/env bash
# Rebuilds the kp-eval evaluation state on a fresh profile (README "Reproduce" order), then checks that the
# as-is engine reproduces the RQ2 baseline. The KubePattern image is not loaded (KP_SKIP_LOAD): load it with
# `minikube -p kp-eval image load <tar>` before any in-cluster run.
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
cd "$EVAL"
source env/versions.env
kubectl config use-context "$KP_PROFILE" >/dev/null
step() { printf '\n==> [%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
for s in env/install/[1-9]*.sh; do
  [[ $s == *95-kyverno* ]] && continue
  step "$s"; "$s" >/dev/null
done
step "Kyverno (RQ6 state: scan every 5m, read RBAC, GlobalContextEntries, 1:1 policies)"
KYVERNO_SCAN_INTERVAL=5m env/install/95-kyverno.sh >/dev/null
kubectl apply -f comparison/kyverno/rbac.yaml -f comparison/kyverno/globalcontext/ >/dev/null
kubectl apply -f comparison/kyverno/equivalent/ >/dev/null
step "KubePattern chart (image $KP_IMAGE, not loaded)"
KP_SKIP_LOAD=1 env/install/00-kubepattern.sh >/dev/null
step "scenarios and Patterns"
./scripts/apply-scenario.sh >/dev/null
./scripts/apply-patterns.sh >/dev/null
./scripts/verify_fields.py | tail -2
step "baseline with the as-is engine (out of cluster), until it settles"
for i in 1 2 3 4 5 6; do
  line=$(KP_BIN="$EVAL/bin/kubepattern-69d4ffd" ./scripts/perf-local.sh rebuild/baseline-69d4ffd | tail -1)
  d=$(jq -r .dir <<<"$line")
  if ./scripts/score.py "$d/smells.json" > "$d/score.txt"; then grep '^TOTAL' "$d/score.txt"; step "baseline reproduced"; exit 0; fi
  echo "attempt $i: $(grep '^TOTAL' "$d/score.txt")"; sed -n '/MISMATCHES/,$p' "$d/score.txt" | head -8; sleep 60
done
step "baseline NOT reproduced"; exit 1
