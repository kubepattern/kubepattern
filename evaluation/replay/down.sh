#!/usr/bin/env bash
# Stops the replay API server and etcd. With --purge, also deletes the etcd data and certs.
set -euo pipefail
source "$(dirname "$0")/env.sh"
for p in apiserver etcd; do
  [[ -f "$REPLAY_STATE/$p.pid" ]] && kill "$(cat "$REPLAY_STATE/$p.pid")" 2>/dev/null || true
  rm -f "$REPLAY_STATE/$p.pid"
done
sleep 2
[[ "${1:-}" == "--purge" ]] && rm -rf "$REPLAY_STATE"
echo "replay stopped"
