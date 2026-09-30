#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "CloudNativePG $CNPG_VERSION"
kubectl apply --server-side -f \
  "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-${CNPG_VERSION%.*}/releases/cnpg-${CNPG_VERSION}.yaml"
wait_deploys cnpg-system
