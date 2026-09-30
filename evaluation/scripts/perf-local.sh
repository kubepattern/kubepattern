#!/usr/bin/env bash
# One measured out-of-cluster run of the extracted binary (RQ4).
# Authenticates as the KubePattern ServiceAccount (token) so the audit filter applies, and runs
# under /usr/bin/time -v. Prints one JSON line: wall, peak RSS, CPU, smells, API requests, phases.
# Usage: perf-local.sh <label>
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
label="${1:?usage: perf-local.sh <label>}"
work="$EVAL/results/raw/perf/$label/$(date -u +%Y%m%dT%H%M%S%NZ)"
mkdir -p "$work"
kc="$EVAL/env/sa.kubeconfig"
if [[ ! -f "$kc" || $(find "$kc" -mmin +600 | wc -l) -gt 0 ]]; then
  server="$(kubectl config view --minify --context "$KP_PROFILE" -o jsonpath='{.clusters[0].cluster.server}')"
  token="$(kubectl -n "$KP_NAMESPACE" create token "${KP_RELEASE}-sa" --duration=24h)"
  kubectl config --kubeconfig="$kc" set-cluster kp --server="$server" --certificate-authority="$HOME/.minikube/ca.crt" --embed-certs >/dev/null
  kubectl config --kubeconfig="$kc" set-credentials sa --token="$token" >/dev/null
  kubectl config --kubeconfig="$kc" set-context kp --cluster=kp --user=sa >/dev/null
  kubectl config --kubeconfig="$kc" use-context kp >/dev/null
  chmod 600 "$kc"
fi
printf 'saveInNamespace: true\ntargetNamespace: "%s"\n' "$KP_NAMESPACE" > "$work/config.yaml"
since="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
( cd "$work" && KUBECONFIG="$kc" /usr/bin/time -v "$KP_BIN" > run.log 2> time.txt ) || echo "run exited non-zero" >&2
kubectl get smells.kubepattern.dev -A -o json > "$work/smells.json"
jq '.items | length' "$work/smells.json" > "$work/smells.count"
sleep 2
kubectl -n kube-system logs "kube-apiserver-$KP_PROFILE" --since-time="$since" | grep '^{"kind":"Event"' \
  | jq -c --arg sa "$KP_SA" 'select(.user.username == $sa)' > "$work/audit.jsonl" || true
python3 - "$work" "$label" "$EVAL/scripts" <<'PY'
import json, re, sys, os
work, label, scripts = sys.argv[1:]
t = open(os.path.join(work, "time.txt")).read()
def grab(pat):
    m = re.search(pat, t); return m.group(1) if m else None
wall = grab(r"Elapsed \(wall clock\) time \(h:mm:ss or m:ss\): (\S+)")
parts = [float(x) for x in wall.split(":")] if wall else []
wall_s = sum(p * 60 ** i for i, p in enumerate(reversed(parts))) if parts else None
sys.path.insert(0, scripts)
from audit_summary import summarise
a = summarise(os.path.join(work, "audit.jsonl"))
log = open(os.path.join(work, "run.log")).read() + t
row = {"label": label, "wall_s": wall_s, "max_rss_mb": round(int(grab(r"Maximum resident set size \(kbytes\): (\d+)") or 0) / 1024, 1),
       "user_s": float(grab(r"User time \(seconds\): (\S+)") or 0), "sys_s": float(grab(r"System time \(seconds\): (\S+)") or 0),
       "smells": int(open(os.path.join(work, "smells.count")).read()), "api_requests": a.get("requests", 0),
       "by_verb": a.get("by_verb", {}), "phases": {k: v["duration_s"] for k, v in a.get("phases", {}).items()},
       "timeout": bool(re.search(r"would exceed context deadline|context deadline exceeded", log)), "dir": work}
print(json.dumps(row))
PY
