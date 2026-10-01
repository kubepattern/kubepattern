#!/usr/bin/env bash
# RQ8 on kp-eval: does spec.for remove snapshot false positives without hiding persistent smells, and at what latency?
# Every installed evaluation Pattern gets a copy "<name>-for" with spec.for=$FOR; both sets are evaluated in the same
# out-of-cluster runs ($KP_BIN), every $PERIOD s. Events (between runs):
#   after run 1  CAPI rotation step 1 (dmt-rotated-v3 created, unreferenced)            -> transient
#                Certificate web deleted (ci-web unreferenced: a GitOps delete-and-recreate) -> transient
#                ClusterIssuer ci-rq8-new created, never referenced                      -> new persistent smell
#   after run 2  forced Kyverno scan (transients still present), then
#                rotation step 2 (md-rotated -> dmt-rotated-v3; dmt-rotated-v2 is left behind) and Certificate web back
#   runs 3..RUNS no further event
# Each run's Smells are split by set and scored against the RQ2 ground truth: the base set as is, the copies with
# --active-only after stripping the suffix. Output: results/rq8/eval/{timeline.csv,scores.csv,kyverno.csv}.
# At the end the events are reverted (scenario re-applied) and the copies deleted.
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
: "${KP_BIN:?set KP_BIN to an engine with spec.for}"; : "${PERIOD:=150}"; : "${RUNS:=5}"; : "${FOR:=4m}"
S="$EVAL/scripts"; OUT="$EVAL/results/rq8/eval"; mkdir -p "$OUT"
names="$(kubectl get patterns.kubepattern.dev -o name | sed 's|.*/||' | grep -v -- '-for$')"
echo "run,t_s,set,pattern,kind,namespace,name,phase,since" > "$OUT/timeline.csv"
echo "run,t_s,set,P,N,TP,FP,FN,TN,prec,rec,probes,unlisted" > "$OUT/scores.csv"

cleanup() {
  kubectl -n kp-capi-a patch machinedeployment md-rotated --type=merge \
    -p '{"spec":{"template":{"spec":{"infrastructureRef":{"name":"dmt-rotated-v2"}}}}}' >/dev/null 2>&1 || true
  kubectl -n kp-capi-a delete dockermachinetemplate dmt-rotated-v3 --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clusterissuer ci-rq8-new --ignore-not-found >/dev/null 2>&1 || true
  "$S/apply-scenario.sh" cert-manager >/dev/null 2>&1 || true
  for n in $names; do kubectl delete pattern.kubepattern.dev "$n-for" --ignore-not-found >/dev/null 2>&1 || true; done
}
trap cleanup EXIT

for n in $names; do
  kubectl get pattern.kubepattern.dev "$n" -o json \
    | jq --arg f "$FOR" '{apiVersion, kind, metadata: {name: (.metadata.name + "-for"), labels: .metadata.labels}, spec: (.spec + {for: $f})}' \
    | kubectl apply -f - >/dev/null
done
start=$(date +%s)

score_set() {  # $1 run, $2 t, $3 set (base|for), $4 smells.json of the set
  local line
  if [[ "$3" == for ]]; then line=$("$S/score.py" "$4" --active-only | grep "^TOTAL" || true)
  else line=$("$S/score.py" "$4" | grep "^TOTAL" || true); fi
  awk -v r="$1" -v t="$2" -v s="$3" '{printf "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n", r, t, s, $2, $3, $4, $5, $6, $7, $8, $9, $10$11, $12}' <<<"$line" >> "$OUT/scores.csv"
}
run() {  # $1 = run number
  local t line dir
  t=$(( $(date +%s) - start ))
  line=$("$S/perf-local.sh" "rq8/run$1" | tail -1); dir=$(jq -r .dir <<<"$line")
  jq '{items: [.items[] | select(.spec.pattern.name | endswith("-for") | not)]}' "$dir/smells.json" > "$dir/base.json"
  jq '{items: [.items[] | select(.spec.pattern.name | endswith("-for")) | .spec.pattern.name |= sub("-for$"; "")]}' "$dir/smells.json" > "$dir/for.json"
  for set in base for; do
    jq -r --arg r "$1" --arg t "$t" --arg s "$set" '.items[] | [$r, $t, $s, .spec.pattern.name, .spec.target.kind, (.spec.target.namespace // ""), .spec.target.name, (.spec.phase // "Active"), (.spec.since // "")] | @csv' \
      "$dir/$set.json" >> "$OUT/timeline.csv"
    score_set "$1" "$t" "$set" "$dir/$set.json"
  done
  echo "run $1 at +${t}s ($(jq -c '{wall_s, api_requests}' <<<"$line")): base $(tail -2 "$OUT/scores.csv" | head -1 | cut -d, -f4-) | for(Active) $(tail -1 "$OUT/scores.csv" | cut -d, -f4-)"
}
wait_until() { local now; now=$(date +%s); (( $1 > now )) && sleep $(( $1 - now )); return 0; }

run 1
cat <<'YAML' | kubectl apply -f - >/dev/null
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: DockerMachineTemplate
metadata: {name: dmt-rotated-v3, namespace: kp-capi-a}
spec: {template: {spec: {}}}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: ci-rq8-new}
spec: {selfSigned: {}}
YAML
kubectl -n kp-certs delete certificate web --wait=true >/dev/null
wait_until $(( start + PERIOD )); run 2
kdir=$("$S/kyverno-run.sh" rq8-kyverno 2>/dev/null | tail -1)
echo "pattern,kind,namespace,name" > "$OUT/kyverno.csv"
jq -r '.items[] | select(.spec.target.name | test("^(dmt-rotated-v3|ci-web|ci-rq8-new)$")) | [.spec.pattern.name, .spec.target.kind, (.spec.target.namespace // ""), .spec.target.name] | @csv' \
  "$kdir/smells.json" >> "$OUT/kyverno.csv"
echo "kyverno fail on the event objects after run 2: $(tail -n +2 "$OUT/kyverno.csv" | tr '\n' ' ')"
kubectl -n kp-capi-a patch machinedeployment md-rotated --type=merge \
  -p '{"spec":{"template":{"spec":{"infrastructureRef":{"name":"dmt-rotated-v3"}}}}}' >/dev/null
"$S/apply-scenario.sh" cert-manager >/dev/null 2>&1
for r in $(seq 3 "$RUNS"); do wait_until $(( start + (r - 1) * PERIOD )); run "$r"; done
echo done
