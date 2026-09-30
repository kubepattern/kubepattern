#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "KubeVela $KUBEVELA_VERSION"
helm upgrade --install kubevela vela-core --repo https://kubevela.github.io/charts \
  --version "$KUBEVELA_VERSION" --namespace vela-system --create-namespace \
  --set multicluster.enabled=false --wait --timeout 10m
wait_deploys vela-system
