#!/usr/bin/env bash
# Builds KubePattern at $KP_COMMIT (default: the engine evaluated as-is), loads the image
# into the evaluation profile and extracts the static binary to $KP_BIN for out-of-cluster runs (RQ4).
# Usage: [KP_COMMIT=<sha>] build-image.sh [build|load|extract|all]   (default: all)
set -euo pipefail
EVAL="$(cd "$(dirname "$0")/.." && pwd)"
source "$EVAL/env/versions.env"
STEP="${1:-all}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

build() {
  git -C "$EVAL/.." archive "$KP_COMMIT" | tar -x -C "$WORK"
  podman build -t "$KP_IMAGE" "$WORK"
}
load() {
  podman save -o "$WORK/image.tar" "$KP_IMAGE"
  minikube -p "$KP_PROFILE" image load "$WORK/image.tar"
}
extract() {
  mkdir -p "$(dirname "$KP_BIN")"
  cid="$(podman create "$KP_IMAGE")"
  podman cp "$cid:/app/kubepattern" "$KP_BIN"
  podman rm "$cid" >/dev/null
  ls -l "$KP_BIN"
}

case "$STEP" in
  build) build ;;
  load) load ;;
  extract) extract ;;
  all) build; load; extract ;;
  *) echo "unknown step $STEP" >&2; exit 2 ;;
esac
