#!/usr/bin/env bash
# What the controllers do after the mutations of ../scripts/mutate.sh:
#   M01: the garbage collector deletes the CertificateRequest owned by Certificate web;
#   M07: the applicationset-controller deletes as-list-1's Application (empty generator).
# (M05's generated ExternalSecret is not created by the replay, so nothing to delete.)
set -euo pipefail
source "$(dirname "$0")/env.sh"
kubectl -n kp-certs delete certificaterequest web-1 --ignore-not-found >/dev/null
kubectl -n argocd delete application as-list-1-solo --ignore-not-found >/dev/null
