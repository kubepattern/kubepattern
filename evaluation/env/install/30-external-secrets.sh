#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "External Secrets Operator (chart $ESO_CHART_VERSION)"
helm upgrade --install external-secrets external-secrets --repo https://charts.external-secrets.io \
  --version "$ESO_CHART_VERSION" --namespace external-secrets --create-namespace
wait_deploys external-secrets
