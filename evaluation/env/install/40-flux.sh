#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "Flux $FLUX_VERSION"
kubectl apply --server-side -f "https://github.com/fluxcd/flux2/releases/download/${FLUX_VERSION}/install.yaml"
wait_deploys flux-system
