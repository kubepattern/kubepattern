#!/usr/bin/env bash
# One forced Kyverno background scan (RQ6): restarts the reports controller, which re-evaluates
# every matching resource on start-up, then waits for the reports to settle and archives them.
# Usage: kyverno-run.sh <label>   (prints the archive directory on the last line)
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
label="${1:?usage: kyverno-run.sh <label>}"
since=$(date +%s)
kubectl -n "$KYVERNO_NAMESPACE" rollout restart deploy/kyverno-reports-controller >/dev/null
kubectl -n "$KYVERNO_NAMESPACE" rollout status deploy/kyverno-reports-controller --timeout=5m >/dev/null
SINCE=$since "$EVAL/scripts/kyverno-snapshot.sh" "$label"
