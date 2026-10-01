#!/usr/bin/env bash
# Applies a scenario: base state, then seeds, then the optional dynamic steps (post.sh).
# Usage: apply-scenario.sh <platform>...   (no argument: every scenario)
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
kubectl config use-context "$KP_PROFILE" >/dev/null
platforms=("$@")
[[ ${#platforms[@]} -eq 0 ]] && platforms=($(find "$EVAL/scenarios" -mindepth 1 -maxdepth 1 -type d -printf "%f\n" | sort))
for p in "${platforms[@]}"; do
  dir="$EVAL/scenarios/$p"
  echo "==> scenario $p"
  [[ -x "$dir/pre.sh" ]] && "$dir/pre.sh"
  for f in base.yaml seeds.yaml; do
    [[ -f "$dir/$f" ]] && kubectl apply --server-side --force-conflicts -f "$dir/$f"
  done
  [[ -x "$dir/post.sh" ]] && "$dir/post.sh"
done
