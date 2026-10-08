#!/usr/bin/env bash
# RQ6d robustness of Kyverno, the counterpart of RQ5 events E3 (missing CRD) and E4/E5 (narrowed RBAC).
#   R1a  a policy whose target CRD is not installed
#   R1b  a GlobalContextEntry (dependency) whose CRD is not installed, read by a policy on an existing kind
#   R2   kargo.akuity.io removed from the aggregated controller role, then restored
# After each step it records the per-policy result counts, the policy/GCE status and controller log lines.
# Output: measurements/raw/kyverno/robustness/<run>/{<step>.json,<step>.log,apply-*.txt}
# Usage: kyverno-robustness.sh
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
R="$EVAL/comparison/kyverno/robustness"
ns="$KYVERNO_NAMESPACE"
out="$EVAL/measurements/raw/kyverno/robustness/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$out"

counts() {
  kubectl get policyreports.wgpolicyk8s.io,clusterpolicyreports.wgpolicyk8s.io -A -o json \
    | jq -c '[.items[] | (.results // [])[] | select(.source == "KyvernoValidatingPolicy")]
             | group_by(.policy) | map({(.[0].policy): (group_by(.result) | map({(.[0].result): length}) | add)}) | add // {}'
}
# Waits at least MIN s, then until the result counts are identical across two polls 20 s apart.
settle() {
  local min="$1" prev="" cur end=$(( $(date +%s) + 900 ))
  sleep "$min"
  while cur="$(counts)"; [[ "$cur" != "$prev" ]]; do
    (( $(date +%s) > end )) && { echo "settle timeout" >&2; break; }
    prev="$cur"; sleep 20
  done
}
record() {
  local step="$1" since="$2"
  jq -n --arg step "$step" --argjson counts "$(counts)" \
    --argjson vpol "$(kubectl get validatingpolicies.policies.kyverno.io -o json | jq '[.items[] | {name: .metadata.name, ready: (.status.conditionStatus.ready // null), conditions: (.status.conditionStatus.conditions // [])}]')" \
    --argjson gce "$(kubectl get globalcontextentries.kyverno.io -o json | jq '[.items[] | {name: .metadata.name, status: .status}]')" \
    '{step: $step, counts: $counts, validatingPolicies: $vpol, globalContextEntries: $gce}' > "$out/$step.json"
  for c in reports-controller admission-controller background-controller; do
    kubectl -n "$ns" logs "deploy/kyverno-$c" --since-time="$since" 2>/dev/null \
      | grep -i -E 'error|fail|forbidden|kratix|promise|kargo|not found' | sed "s/^/[$c] /" || true
  done > "$out/$step.log"
  printf '%-22s %s\n' "$step" "$(jq -c '.counts | with_entries(select(.key | test("kargo|kratix|robustness|certmanager")))' "$out/$step.json")" >&2
  printf '%-22s total=%s  log lines=%s\n' "" "$(jq -c '[.counts[] | to_entries[]] | group_by(.key) | map({(.[0].key): (map(.value) | add)}) | add' "$out/$step.json")" "$(wc -l < "$out/$step.log")" >&2
}
restart_reports() {
  kubectl -n "$ns" rollout restart deploy/kyverno-reports-controller >/dev/null
  kubectl -n "$ns" rollout status deploy/kyverno-reports-controller --timeout=5m >/dev/null
}
iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }

t=$(iso); record R0-baseline "$t"

echo "== R1a: target CRD not installed" >&2
t=$(iso)
kubectl apply -f "$R/crd-not-installed.yaml" > "$out/apply-R1a.txt" 2>&1 && echo "exit=0" >> "$out/apply-R1a.txt" || echo "exit=$?" >> "$out/apply-R1a.txt"
cat "$out/apply-R1a.txt" >&2
echo "== R1b: dependency CRD not installed" >&2
kubectl apply -f "$R/dependency-crd-not-installed.yaml" > "$out/apply-R1b.txt" 2>&1 && echo "exit=0" >> "$out/apply-R1b.txt" || echo "exit=$?" >> "$out/apply-R1b.txt"
cat "$out/apply-R1b.txt" >&2
settle 60; record R1-applied "$t"
t=$(iso); restart_reports; settle 90; record R1-after-rescan "$t"
kubectl delete --ignore-not-found -f "$R/crd-not-installed.yaml" -f "$R/dependency-crd-not-installed.yaml" >/dev/null

echo "== R2: RBAC without kargo.akuity.io" >&2
t=$(iso)
kubectl apply -f "$R/rbac-without-kargo.yaml" >/dev/null
settle 60; record R2-narrowed "$t"
t=$(iso); restart_reports; settle 90; record R2-narrowed-rescan "$t"

echo "== R2: RBAC restored" >&2
t=$(iso)
kubectl apply -f "$EVAL/comparison/kyverno/rbac.yaml" >/dev/null
settle 60; record R2-restored "$t"
t=$(iso); restart_reports; settle 90; record R2-restored-rescan "$t"
echo "$out"
