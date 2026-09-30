# Shared helpers for the Krateo install scripts.
set -euo pipefail
KRATEO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EVAL="$(cd "$KRATEO/.." && pwd)"
source "$KRATEO/env/krateo.env"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null

log() { printf '\n==> %s\n' "$*"; }

wait_deploys() {
  local ns="$1" timeout="${2:-15m}"
  kubectl -n "$ns" wait --for=condition=Available deploy --all --timeout="$timeout"
}
