#!/usr/bin/env bash
# Cluster API core + kubeadm bootstrap/control-plane + CAPD, without clusterctl.
# Release manifests use ${VAR:=default}; envsubst cannot expand that form, so a small
# Python filter does it. Workload Clusters in the scenario stay paused: no machines are built.
source "$(dirname "$0")/lib.sh"
log "Cluster API $CAPI_VERSION + CAPD"
export CLUSTER_TOPOLOGY=true
base="https://github.com/kubernetes-sigs/cluster-api/releases/download/${CAPI_VERSION}"
subst() {
  python3 -c '
import os, re, sys
pat = re.compile(r"\$\{([A-Za-z0-9_]+)(?::=([^}]*))?\}")
sys.stdout.write(pat.sub(lambda m: os.environ.get(m.group(1), m.group(2) or ""), sys.stdin.read()))'
}
for asset in core-components bootstrap-components control-plane-components infrastructure-components-development; do
  curl -fsSL "$base/$asset.yaml" | subst | kubectl apply --server-side -f -
done
for ns in capi-system capi-kubeadm-bootstrap-system capi-kubeadm-control-plane-system capd-system; do
  wait_deploys "$ns"
done
