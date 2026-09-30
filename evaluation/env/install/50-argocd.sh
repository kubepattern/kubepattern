#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
log "Argo CD (chart $ARGOCD_CHART_VERSION)"
helm upgrade --install argocd argo-cd --repo https://argoproj.github.io/argo-helm \
  --version "$ARGOCD_CHART_VERSION" --namespace argocd --create-namespace \
  --set dex.enabled=false --set notifications.enabled=false
wait_deploys argocd
kubectl -n argocd rollout status statefulset/argocd-application-controller --timeout=10m
