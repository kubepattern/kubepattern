#!/usr/bin/env bash
# QPS validation on kp-eval: baseline, whole-cluster runs a814e4a vs wp0 (scored), then the RQ4 S2 subset
# with wp0 (compare with measurements/gcfix/scale-after, a814e4a at 5 QPS), cleanup and a final baseline.
# Kyverno's reports and background controllers are scaled to 0 during the measurements (they were not
# installed when measurements/gcfix/scale-after was measured) and restored at the end.
set -euo pipefail
cd "$(dirname "$0")/.."
source env/versions.env
kubectl config use-context "$KP_PROFILE" >/dev/null
out=measurements/qps; mkdir -p "$out"; : > "$out/whole-cluster.jsonl"
restore() { kubectl -n kyverno scale deploy kyverno-reports-controller kyverno-background-controller --replicas=1 >/dev/null; echo "kyverno controllers restored"; }
trap restore EXIT
baseline() { local d; d=$(./scripts/run-once.sh "$1" | tail -1); ./scripts/score.py "$d/smells.json" | tail -1; }
echo "== baseline: $(baseline qps/baseline-before)"
kubectl -n kyverno scale deploy kyverno-reports-controller kyverno-background-controller --replicas=0 >/dev/null
for r in 1 2 3; do
  for v in a814e4a wp0; do
    line=$(KP_BIN="$PWD/bin/kubepattern-$v" ./scripts/perf-local.sh "qps/whole-$v-r$r" | tail -1)
    d=$(jq -r .dir <<<"$line"); sc=$(./scripts/score.py "$d/smells.json" | tail -1)
    jq -c --arg v "$v" --arg r "$r" --arg sc "$sc" '. + {engine: $v, rep: ($r|tonumber), score: $sc}' <<<"$line" | tee -a "$out/whole-cluster.jsonl"
  done
done
echo "== S2 subset (wp0)"
SCALE_OUT=measurements/qps/scale RUN_PREFIX=qps- KP_BIN="$PWD/bin/kubepattern-wp0" ./scripts/scale.py s2-subset
./scripts/scale.py cleanup
echo "== baseline: $(baseline qps/baseline-after)"
echo done
