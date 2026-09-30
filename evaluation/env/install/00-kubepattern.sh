#!/usr/bin/env bash
# KubePattern from the local chart, using the image built at the pinned commit.
# The CronJob is suspended: every analysis run is triggered explicitly by scripts/run-once.sh.
source "$(dirname "$0")/lib.sh"
log "KubePattern ($KP_IMAGE)"
"$EVAL/scripts/build-image.sh" load
helm upgrade --install "$KP_RELEASE" "$EVAL/../charts/kubepattern" \
  --namespace "$KP_NAMESPACE" --create-namespace \
  --set image.repository="${KP_IMAGE%:*}" --set image.tag="${KP_IMAGE##*:}" \
  --set image.pullPolicy=Never --set suspend=true --set example.enabled=false \
  "$@"
kubectl -n "$KP_NAMESPACE" get cronjob
