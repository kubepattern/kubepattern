#!/usr/bin/env bash
# RQ8 on Krateo (scenario/rq8/): does spec.for hide transient widget orphans without hiding the persistent one?
# Two copies of paragraph-not-referenced are evaluated in the same runs: the scored one (no spec.for, today's
# behaviour) and paragraph-not-referenced-for5m (spec.for: 5m). Runs are out-of-cluster ($KP_BIN) every PERIOD s.
#   run 1           rq8-stale, rq8-wip unreferenced; rq8-moved wired; then the Column is deleted (re-sync starts)
#   run 2           all three unreferenced; then a forced Kyverno scan; then the Column is re-created wiring rq8-wip too
#   runs 3..RUNS    only rq8-stale unreferenced
# Output: measurements/rq8/krateo/{timeline.csv,kyverno.csv}. The scenario objects are removed at the end.
set -euo pipefail
KRATEO="$(cd "$(dirname "$0")/.." && pwd)"
EVAL="$(cd "$KRATEO/.." && pwd)"
source "$KRATEO/env/krateo.env"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
: "${KP_BIN:?set KP_BIN to an engine with spec.for}"; : "${PERIOD:=180}"; : "${RUNS:=4}"; : "${FOR:=5m}"
SC="$KRATEO/scenario/rq8"; OUT="$EVAL/measurements/rq8/krateo"; mkdir -p "$OUT"
COPY="paragraph-not-referenced-for5m"
echo "run,t_s,pattern,namespace,name,phase,since" > "$OUT/timeline.csv"

cleanup() {
  kubectl delete -f "$SC/objects.yaml" --ignore-not-found --wait=true >/dev/null 2>&1 || true
  kubectl delete pattern.kubepattern.dev "$COPY" --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

kubectl get pattern.kubepattern.dev paragraph-not-referenced -o json \
  | jq --arg n "$COPY" --arg f "$FOR" '{apiVersion, kind, metadata: {name: $n, labels: .metadata.labels}, spec: (.spec + {for: $f})}' \
  | kubectl apply -f - >/dev/null
kubectl apply -f "$SC/objects.yaml" >/dev/null
kubectl apply -f "$SC/column-before.yaml" >/dev/null
start=$(date +%s)

run() {  # $1 = run number
  local line dir t
  t=$(( $(date +%s) - start ))
  line=$("$EVAL/scripts/perf-local.sh" "krateo/rq8-run$1" | tail -1)
  dir=$(jq -r .dir <<<"$line")
  jq -r --arg r "$1" --arg t "$t" '.items[] | select(.spec.target.namespace == "kp-krateo-rq8")
    | [$r, $t, .spec.pattern.name, .spec.target.namespace, .spec.target.name, (.spec.phase // "Active"), (.spec.since // "")] | @csv' \
    "$dir/smells.json" >> "$OUT/timeline.csv"
  echo "run $1 at +${t}s: $(jq -c '[.items[] | select(.spec.target.namespace == "kp-krateo-rq8") | "\(.spec.pattern.name | sub("paragraph-not-referenced"; "p")):\(.spec.target.name):\(.spec.phase // "Active")"]' "$dir/smells.json")"
}
wait_until() { local now; now=$(date +%s); (( $1 > now )) && sleep $(( $1 - now )); return 0; }

run 1
kubectl delete -f "$SC/column-before.yaml" --wait=true >/dev/null            # re-sync starts
wait_until $(( start + PERIOD )); run 2
kdir=$("$EVAL/scripts/kyverno-run.sh" krateo/rq8-kyverno 2>/dev/null | tail -1)
echo "pattern,namespace,name" > "$OUT/kyverno.csv"
jq -r '.items[] | select(.spec.target.namespace == "kp-krateo-rq8") | [.spec.pattern.name, .spec.target.namespace, .spec.target.name] | @csv' \
  "$kdir/smells.json" >> "$OUT/kyverno.csv"
echo "kyverno fail after run 2: $(tail -n +2 "$OUT/kyverno.csv" | tr '\n' ' ')"
kubectl apply -f "$SC/column-after.yaml" >/dev/null                           # re-sync done, rq8-wip wired
for r in $(seq 3 "$RUNS"); do wait_until $(( start + (r - 1) * PERIOD )); run "$r"; done
echo done
