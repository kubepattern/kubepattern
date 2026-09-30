#!/usr/bin/env bash
# Creates the dedicated evaluation profile with API-server audit logging to stdout.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
source "$HERE/versions.env"

# minikube copies ~/.minikube/files/* into the VM; kubeadm mounts /etc/ssl/certs into the apiserver.
mkdir -p "$HOME/.minikube/files/etc/ssl/certs"
cp "$HERE/audit-policy.yaml" "$HOME/.minikube/files/etc/ssl/certs/audit-policy.yaml"

minikube start -p "$KP_PROFILE" \
  --driver=kvm2 --container-runtime=docker \
  --cpus="$MINIKUBE_CPUS" --memory="$MINIKUBE_MEMORY" --disk-size="$MINIKUBE_DISK" \
  --kubernetes-version="$KUBERNETES_VERSION" \
  --extra-config=apiserver.audit-policy-file=/etc/ssl/certs/audit-policy.yaml \
  --extra-config=apiserver.audit-log-path=-

minikube -p "$KP_PROFILE" addons enable metrics-server
kubectl config use-context "$KP_PROFILE"
kubectl get nodes -o wide
