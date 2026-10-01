#!/usr/bin/env bash
# QPS validation on kp-krateo (check the baseline first: krateo_oracle.py check, run-once.sh): alternating out-of-cluster runs
# of a814e4a (prune, client-go default 5 QPS) and wp0 (prune + 50/100 QPS + linter), each scored.
set -euo pipefail
cd "$(dirname "$0")/../.."
source krateo/env/krateo.env
source env/versions.env
kubectl config use-context "$KP_PROFILE" >/dev/null
out=results/qps/krateo; mkdir -p "$out"; : > "$out/runs.jsonl"
score() { ./scripts/score.py "$1" --truth krateo/scenario/ground-truth.csv | tail -1; }
for r in 1 2 3; do
  for v in a814e4a wp0; do
    line=$(KP_BIN="$PWD/bin/kubepattern-$v" ./scripts/perf-local.sh "krateo/qps-$v-r$r" | tail -1)
    d=$(jq -r .dir <<<"$line"); sc=$(score "$d/smells.json")
    jq -c --arg v "$v" --arg r "$r" --arg sc "$sc" '. + {engine: $v, rep: ($r|tonumber), score: $sc}' <<<"$line" | tee -a "$out/runs.jsonl"
  done
done
echo done
