#!/usr/bin/env bash
# RQ6 only: Kyverno as the comparison baseline. Default install (admission webhooks included, for
# realism); the background controller and the reports controller are enabled by default.
# KYVERNO_SCAN_INTERVAL overrides the background-scan interval (chart default 1h).
source "$(dirname "$0")/lib.sh"
: "${KYVERNO_SCAN_INTERVAL:=1h}"
log "Kyverno (chart $KYVERNO_CHART_VERSION, background scan every $KYVERNO_SCAN_INTERVAL)"
helm upgrade --install kyverno kyverno --repo https://kyverno.github.io/kyverno/ \
  --version "$KYVERNO_CHART_VERSION" --namespace "$KYVERNO_NAMESPACE" --create-namespace \
  --set features.backgroundScan.backgroundScanInterval="$KYVERNO_SCAN_INTERVAL"
wait_deploys "$KYVERNO_NAMESPACE"
