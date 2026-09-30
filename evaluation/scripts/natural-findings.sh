#!/usr/bin/env bash
# RQ2 natural findings: unused definitions of the KubeVela built-in catalogue (vela-system).
# Oracle: an independent jq computation over the live objects (definitions minus every type
# referenced by an Application). Writes results/natural/kubevela.csv and a summary.
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
out="$EVAL/results/natural"; mkdir -p "$out"
kubectl apply -f "$EVAL/patterns/kubevela/natural/"
run="$("$EVAL/scripts/run-once.sh" natural | tail -1)"
kubectl delete -f "$EVAL/patterns/kubevela/natural/" >/dev/null

apps="$(kubectl get applications.core.oam.dev -A -o json)"
used_c="$(jq '[.items[].spec.components[].type | sub("@.*$"; "")] | unique' <<<"$apps")"
used_t="$(jq '[.items[].spec.components[] | (.traits // [])[] | .type | sub("@.*$"; "")] | unique' <<<"$apps")"
{
  echo "kind,name,oracle,kubepattern"
  for kind in ComponentDefinition TraitDefinition; do
    used="$used_c"; [[ $kind == TraitDefinition ]] && used="$used_t"
    pattern="kubevela-builtin-${kind,,}-not-used"
    kubectl get "${kind,,}s.core.oam.dev" -n vela-system -o json | jq -r --argjson used "$used" \
      --slurpfile s "$run/smells.json" --arg p "$pattern" --arg k "$kind" '
      ($s[0].items | map(select(.spec.pattern.name == $p) | .spec.target.name)) as $flagged
      | .items[].metadata.name as $n
      | [$k, $n, (if ($used | index($n)) then "used" else "unused" end),
         (if ($flagged | index($n)) then "unused" else "used" end)] | @csv'
  done
} | tr -d '"' > "$out/kubevela.csv"
python3 - "$out/kubevela.csv" <<'PY'
import csv, sys
rows = list(csv.DictReader(open(sys.argv[1])))
for kind in ("ComponentDefinition", "TraitDefinition"):
    r = [x for x in rows if x["kind"] == kind]
    unused = sum(x["oracle"] == "unused" for x in r)
    agree = sum(x["oracle"] == x["kubepattern"] for x in r)
    print(f"{kind}: {len(r)} built-in, {unused} unused per oracle, KubePattern agrees on {agree}/{len(r)}")
PY
