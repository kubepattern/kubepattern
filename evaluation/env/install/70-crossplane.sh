#!/usr/bin/env bash
# Crossplane v2 core. Operations are alpha and need --enable-operations; the Functions
# (one Pod each) belong to the scenario, not to the install.
source "$(dirname "$0")/lib.sh"
log "Crossplane $CROSSPLANE_VERSION"
helm upgrade --install crossplane crossplane --repo https://charts.crossplane.io/stable \
  --version "$CROSSPLANE_VERSION" --namespace crossplane-system --create-namespace \
  --set 'args={--enable-operations}' --wait --timeout 10m
wait_deploys crossplane-system
