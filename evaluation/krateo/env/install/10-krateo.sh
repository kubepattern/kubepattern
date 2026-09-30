#!/usr/bin/env bash
# Installs Krateo PlatformOps $KRATEO_VERSION with the pinned krateoctl (docs: how-to-guides/install-krateo).
# krateoctl 1.2.1 does not bootstrap secrets, so the three required Secrets are created first
# (key-concepts/krateoctl/secrets), with random values and only when missing (idempotent).
source "$(dirname "$0")/lib.sh"

KCTL="$EVAL/bin/krateoctl-$KRATEOCTL_VERSION"
if [[ ! -x "$KCTL" ]]; then
  log "downloading krateoctl $KRATEOCTL_VERSION"
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  base="https://github.com/krateoplatformops/krateoctl/releases/download/v$KRATEOCTL_VERSION"
  curl -sSfL -o "$tmp/k.tgz" "$base/krateoctl_${KRATEOCTL_VERSION}_linux_amd64.tar.gz"
  curl -sSfL -o "$tmp/sums" "$base/krateoctl_${KRATEOCTL_VERSION}_checksums.txt"
  (cd "$tmp" && grep linux_amd64.tar.gz sums | awk '{print $1"  k.tgz"}' | sha256sum -c -)
  tar -xzf "$tmp/k.tgz" -C "$tmp"
  mkdir -p "$EVAL/bin" && install -m 0755 "$tmp/krateoctl-linux-amd64/krateoctl" "$KCTL"
fi

log "namespace and install secrets"
kubectl create namespace "$KRATEO_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
if ! kubectl -n "$KRATEO_NAMESPACE" get secret krateo-db >/dev/null 2>&1; then
  pass="$(openssl rand -hex 24)"
  kubectl -n "$KRATEO_NAMESPACE" create secret generic jwt-sign-key --from-literal=JWT_SIGN_KEY="$(openssl rand -base64 48 | tr -d '\n')"
  kubectl -n "$KRATEO_NAMESPACE" create secret generic krateo-db --from-literal=DB_USER=krateo-db-user --from-literal=DB_PASS="$pass"
  kubectl -n "$KRATEO_NAMESPACE" create secret generic krateo-db-user --from-literal=username=krateo-db-user --from-literal=password="$pass"
fi

log "krateoctl install plan/apply $KRATEO_VERSION (nodeport, profile $KRATEO_PROFILES)"
"$KCTL" install plan  --version "$KRATEO_VERSION" --type nodeport --profile "$KRATEO_PROFILES" --namespace "$KRATEO_NAMESPACE"
"$KCTL" install apply --version "$KRATEO_VERSION" --type nodeport --profile "$KRATEO_PROFILES" --namespace "$KRATEO_NAMESPACE"

log "waiting for the Krateo deployments"
wait_deploys "$KRATEO_NAMESPACE" 20m
kubectl -n "$KRATEO_NAMESPACE" get deploy,sts
kubectl get crd | grep -E 'krateo\.io' | awk '{print $1}'
