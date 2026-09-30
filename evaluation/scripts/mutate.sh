#!/usr/bin/env bash
# RQ2b mutation testing: one mutation operator per pattern (scenarios/mutations.csv).
# baseline run -> apply all mutations -> run -> revert (re-apply scenarios) -> run.
# Expected: applied = baseline + exactly the mutated targets; reverted = baseline (stale Smells GC'd).
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
S="$EVAL/scripts"
# MUTATION_RUNNER: the analysis runner (RQ6 uses scripts/kyverno-run.sh); it must print the run directory last.
RUN="${MUTATION_RUNNER:-$S/run-once.sh}"
base="$("$RUN" "${MUTATION_LABEL:-mutation}-baseline" | tail -1)"

echo "==> applying mutations"
kubectl -n kp-certs delete certificate web
kubectl -n kp-db-a delete scheduledbackups.postgresql.cnpg.io db-live-daily
cat <<'YAML' | kubectl apply -f -
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: pool-rw
  namespace: kp-db-a
  annotations: {cnpg.io/reconciliationLoop: disabled}
spec:
  instances: 1
  storage: {size: 1Gi}
YAML
kubectl -n kp-secrets-a delete externalsecret default-kind
kubectl delete clusterexternalsecret ces
kubectl -n kp-flux-a patch helmrelease podinfo --type=merge -p '{"spec":{"chartRef":{"name":"podinfo-chart-v2"}}}'
kubectl -n argocd patch applicationset as-list-1 --type=json -p '[{"op":"replace","path":"/spec/generators/0/list/elements","value":[]}]'
kubectl -n argocd delete application guestbook-a
kubectl -n kp-kargo-2 delete stage dev
kubectl delete cronoperation yearly-report
kubectl delete compositeresourcedefinition xcaches.eval.kubepattern.dev
kubectl -n kp-vela-a patch applications.core.oam.dev a2 --type=json -p '[
  {"op":"replace","path":"/spec/components/1/type","value":"kp-configmap"},
  {"op":"replace","path":"/spec/components/1/traits","value":[{"type":"kp-team-label"}]}]'
kubectl -n kp-capi-a patch machinedeployment md-workers --type=merge -p '{"spec":{"template":{"spec":{"infrastructureRef":{"name":"dmt-rotated-v2"}}}}}'
# let controllers converge (AppSet deletes its Application, CES children are garbage-collected)
for i in $(seq 1 30); do
  kubectl -n argocd get application as-list-1-solo >/dev/null 2>&1 || kubectl -n kp-secrets-b get externalsecret ces >/dev/null 2>&1 || break
  sleep 2
done
applied="$("$RUN" "${MUTATION_LABEL:-mutation}-applied" | tail -1)"

echo "==> reverting"
kubectl -n kp-db-a delete clusters.postgresql.cnpg.io pool-rw
"$S/apply-scenario.sh" >/dev/null
sleep 20
reverted="$("$RUN" "${MUTATION_LABEL:-mutation}-reverted" | tail -1)"

python3 - "$EVAL/scenarios/mutations.csv" "$base" "$applied" "$reverted" "${MUTATION_OUT:-$EVAL/results/mutation}" <<'PY'
import csv, json, os, sys
muts, base, applied, reverted, out = sys.argv[1:]
def load(d):
    return {(s["spec"]["pattern"]["name"], s["spec"]["target"]["kind"], s["spec"]["target"].get("namespace") or "", s["spec"]["target"]["name"])
            for s in json.load(open(os.path.join(d, "smells.json")))["items"]}
B, A, R = load(base), load(applied), load(reverted)
rows = list(csv.DictReader(open(muts)))
expected = {(r["pattern"], r["kind"], r["namespace"], r["name"]) for r in rows}
os.makedirs(out, exist_ok=True)
with open(os.path.join(out, "mutation.csv"), "w", newline="") as f:
    w = csv.writer(f)
    w.writerow(["id", "operator", "pattern", "target", "killed", "restored"])
    for r in rows:
        k = (r["pattern"], r["kind"], r["namespace"], r["name"])
        w.writerow([r["id"], r["operator"], r["pattern"], "/".join(x for x in k[2:] if x), k in A and k not in B, k not in R])
killed = len(expected & (A - B))
extra = (A - B) - expected
lost = B - A
print(f"mutants: {len(rows)}  killed: {killed}  unexpected new smells: {len(extra)}  lost baseline smells: {len(lost)}  reverted==baseline: {R == B}")
for x in sorted(extra): print("  extra", x)
for x in sorted(lost): print("  lost ", x)
sys.exit(0 if killed == len(rows) and not extra and not lost and R == B else 1)
PY
