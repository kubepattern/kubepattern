#!/usr/bin/env bash
# Applies the evaluated Patterns. Usage: apply-patterns.sh [platform...]  (default: all platforms)
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
platforms=("$@")
[[ ${#platforms[@]} -eq 0 ]] && platforms=($(ls "$EVAL/patterns" | grep -v "^_"))
for p in "${platforms[@]}"; do
  find "$EVAL/patterns/$p" -maxdepth 1 -name '*.yaml' -print0 | xargs -0 -I{} kubectl apply -f {}
done
