# Shared helpers for the install scripts.
set -euo pipefail
EVAL="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null

log() { printf '\n==> %s\n' "$*"; }

# Wait until every Deployment in a namespace is Available.
wait_deploys() {
  local ns="$1" timeout="${2:-10m}"
  kubectl -n "$ns" wait --for=condition=Available deploy --all --timeout="$timeout"
}
