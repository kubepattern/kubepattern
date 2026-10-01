#!/usr/bin/env bash
# Creates the dedicated minikube profile kp-krateo (same kvm2 setup and audit policy as kp-eval).
# kp-eval is stopped first to free memory; its state is kept (minikube start -p kp-eval restores it).
set -euo pipefail
KRATEO="$(cd "$(dirname "$0")/.." && pwd)"
source "$KRATEO/env/krateo.env"
if minikube -p kp-eval status --format '{{.Host}}' 2>/dev/null | grep -q Running; then
  minikube stop -p kp-eval
fi
"$KRATEO/../env/minikube-up.sh"
