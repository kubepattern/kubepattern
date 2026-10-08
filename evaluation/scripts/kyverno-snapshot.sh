#!/usr/bin/env bash
# Waits until the Kyverno PolicyReports of the RQ6 policies settle, then archives them and their
# smells.json conversion:
#   measurements/raw/kyverno/<label>/<run-id>/{reports.json,smells.json,summary.json}
# Settled = the (policy, resource, result) fingerprint is identical across two polls POLL seconds
# apart and, when SINCE (epoch seconds) is set, every result of the RQ6 policies is newer than SINCE.
# Usage: SINCE=<epoch> kyverno-snapshot.sh <label>
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
label="${1:?usage: kyverno-snapshot.sh <label>}"
: "${SINCE:=0}" "${POLL:=20}" "${TIMEOUT:=1800}"
run_id="$(date -u +%Y%m%dT%H%M%SZ)"
out="$EVAL/measurements/raw/kyverno/$label/$run_id"
mkdir -p "$out"
policies="$(kubectl get validatingpolicies.policies.kyverno.io -o jsonpath='{.items[*].metadata.name}')"

dump() {
  kubectl get policyreports.wgpolicyk8s.io,clusterpolicyreports.wgpolicyk8s.io -A -o json
}
start=$(date +%s); prev=""
while :; do
  dump > "$out/reports.json"
  fp="$(jq -r --arg pols "$policies" '($pols | split(" ")) as $p
        | [.items[] | .scope as $s | (.results // [])[] | select(.policy as $x | $p | index($x))
           | "\(.policy)|\($s.kind)|\($s.namespace // "")|\($s.name)|\(.result)"] | sort | join("\n")' "$out/reports.json" | sha1sum | cut -c1-12)"
  read -r n oldest npol < <(jq -r --arg pols "$policies" '($pols | split(" ")) as $p
        | [.items[] | (.results // [])[] | select(.policy as $x | $p | index($x))]
        | "\(length) \(map(.timestamp.seconds) | min // 0) \(map(.policy) | unique | length)"' "$out/reports.json")
  want=$(wc -w <<< "$policies")
  printf '%s  results=%s policies=%s/%s oldest=%s fp=%s\n' "$(date -u +%T)" "$n" "$npol" "$want" "$oldest" "$fp" >&2
  if [[ "$fp" == "$prev" && "$npol" -eq "$want" && "$oldest" -ge "$SINCE" ]]; then break; fi
  if (( $(date +%s) - start > TIMEOUT )); then echo "timeout waiting for reports to settle" >&2; exit 1; fi
  prev="$fp"; sleep "$POLL"
done

"$EVAL/scripts/kyverno_reports_to_smells.py" "$out/reports.json" --out "$out/smells.json" --summary "$out/summary.json" \
  --policies $policies
jq --arg label "$label" --arg run "$run_id" --argjson since "$SINCE" --argjson settled "$(date +%s)" \
  '. + {label: $label, run: $run, since: $since, settledAt: $settled}' "$out/summary.json" > "$out/summary.tmp" \
  && mv "$out/summary.tmp" "$out/summary.json"
echo "$out"
