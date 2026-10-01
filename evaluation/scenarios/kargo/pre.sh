#!/usr/bin/env bash
set -euo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
kubectl apply -f "$dir/projects.yaml"
for p in kp-kargo-1 kp-kargo-2; do
  until kubectl get ns "$p" >/dev/null 2>&1; do sleep 2; done
done
