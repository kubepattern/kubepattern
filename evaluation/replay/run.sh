#!/usr/bin/env bash
# One analysis run on the replay API server.
#   run.sh <engine binary> <label> <pattern dir or file>...
# Replaces the installed Patterns with the given ones (later files override earlier ones with
# the same metadata.name), deletes every Smell, runs the engine once out of cluster with the
# chart's configuration, and writes $REPLAY_STATE/runs/<label>/{smells.json,engine.log}.
set -euo pipefail
source "$(dirname "$0")/env.sh"
bin="$(realpath "${1:?engine binary}")"; label="${2:?label}"; shift 2
out="$REPLAY_STATE/runs/$label"; rm -rf "$out"; mkdir -p "$out/patterns"

# Collect the Patterns, last definition of a name wins.
python3 - "$out/patterns" "$@" <<'PY'
import os, sys, yaml
out, srcs = sys.argv[1], sys.argv[2:]
files = []
for s in srcs:
    if os.path.isdir(s):
        for root, _, names in os.walk(s):
            # natural/ and _robustness/ variants are not part of the RQ2 suite
            if "/natural" in root or "/_robustness" in root:
                continue
            files += [os.path.join(root, n) for n in sorted(names) if n.endswith(".yaml")]
    else:
        files.append(s)
chosen = {}
for f in files:
    for d in yaml.safe_load_all(open(f)):
        if d and d.get("kind") == "Pattern":
            chosen[d["metadata"]["name"]] = d
for name, d in chosen.items():
    with open(os.path.join(out, name + ".yaml"), "w") as fh:
        yaml.safe_dump(d, fh, sort_keys=False)
PY

kubectl create namespace kubepattern --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl delete patterns.kubepattern.dev --all --wait=true >/dev/null
kubectl delete smells.kubepattern.dev -A --all --wait=true >/dev/null
kubectl apply -f "$out/patterns" >/dev/null

cat > "$out/config.yaml" <<'CFG'
saveInNamespace: true
targetNamespace: "kubepattern"
client: {qps: 50, burst: 100}
CFG
start=$(date +%s.%N)
(cd "$out" && "$bin") > "$out/engine.log" 2>&1
end=$(date +%s.%N)
kubectl get smells.kubepattern.dev -A -o json > "$out/smells.json"
printf '%s: %s Patterns, %s Smells in %.2f s\n' "$label" "$(ls "$out/patterns" | wc -l)" \
  "$(python3 -c "import json;print(len(json.load(open('$out/smells.json'))['items']))")" "$(echo "$end - $start" | bc)"
echo "$out"
