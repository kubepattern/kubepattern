#!/usr/bin/env bash
# Sets spec.for on the installed Patterns from a durations table (default: for/durations.csv),
# or removes it (--clear), which restores the behaviour of the evaluated engine (Active at once).
# Patterns that are not installed are skipped. Usage: set-for.sh [--clear] [table.csv]
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
clear=false; [[ "${1:-}" == --clear ]] && { clear=true; shift; }
table="${1:-$EVAL/for/durations.csv}"
installed="$(kubectl get patterns.kubepattern.dev -o name | sed 's|.*/||')"
tail -n +2 "$table" | while IFS=, read -r pattern _suite dur _rest; do
  grep -qx "$pattern" <<<"$installed" || continue
  if $clear; then
    kubectl patch pattern.kubepattern.dev "$pattern" --type=json -p '[{"op":"remove","path":"/spec/for"}]' >/dev/null 2>&1 || true
    echo "$pattern: for removed"
  else
    kubectl patch pattern.kubepattern.dev "$pattern" --type=merge -p "{\"spec\":{\"for\":\"$dur\"}}" >/dev/null
    echo "$pattern: for=$dur"
  fi
done
