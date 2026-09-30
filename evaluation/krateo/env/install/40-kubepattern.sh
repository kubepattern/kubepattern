#!/usr/bin/env bash
# KubePattern on kp-krateo: the shared image/chart scripts, pointed at this profile by krateo.env.
# Engine = $KP_COMMIT (default: the as-is engine 69d4ffd); CronJob suspended, runs are triggered by run-once.sh.
source "$(dirname "$0")/lib.sh"
log "KubePattern $KP_COMMIT on $KP_PROFILE"
podman image exists "$KP_IMAGE" || "$EVAL/scripts/build-image.sh" build
"$EVAL/env/install/00-kubepattern.sh" "$@"
# `minikube image load` once returned 0 without loading the image (pods then fail with ErrImageNeverPull).
for _ in 1 2 3; do
  minikube -p "$KP_PROFILE" image ls | grep -qx "$KP_IMAGE" && exit 0
  "$EVAL/scripts/build-image.sh" load
done
minikube -p "$KP_PROFILE" image ls | grep -qx "$KP_IMAGE" || { echo "image $KP_IMAGE not loaded" >&2; exit 1; }
