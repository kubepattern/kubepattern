#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "cert-manager $CERT_MANAGER_VERSION"
helm upgrade --install cert-manager cert-manager --repo https://charts.jetstack.io \
  --version "$CERT_MANAGER_VERSION" --namespace cert-manager --create-namespace \
  --set crds.enabled=true
wait_deploys cert-manager
