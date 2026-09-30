#!/usr/bin/env bash
# Kyverno on kp-krateo: the RQ6 install (same chart pin), background scan every 5 minutes,
# plus the read-only RBAC for the Krateo API groups used by the policies.
source "$(dirname "$0")/lib.sh"
# Run from evaluation/: helm would take ./kyverno (the policy folder) as a local chart named "kyverno".
cd "$EVAL"
KYVERNO_SCAN_INTERVAL="${KYVERNO_SCAN_INTERVAL:-5m}" "$EVAL/env/install/95-kyverno.sh"
kubectl apply -f "$KRATEO/kyverno/rbac.yaml"
