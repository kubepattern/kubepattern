#!/usr/bin/env bash
# Kargo needs cert-manager (self-signed webhook certs) and an admin account.
# The admin password is random and kept in env/.kargo-admin (git-ignored): the evaluation never logs in.
source "$(dirname "$0")/lib.sh"
log "Kargo $KARGO_VERSION"
secret_file="$EVAL/env/.kargo-admin"
if [[ ! -f "$secret_file" ]]; then
  umask 077
  printf 'password=%s\nsigningKey=%s\n' "$(openssl rand -base64 24)" "$(openssl rand -base64 32)" > "$secret_file"
fi
source "$secret_file"
hash="$(htpasswd -bnBC 10 "" "$password" | tr -d ':\n')"
helm upgrade --install kargo oci://ghcr.io/akuity/kargo-charts/kargo \
  --version "$KARGO_VERSION" --namespace kargo --create-namespace \
  --set-string api.adminAccount.passwordHash="$hash" \
  --set-string api.adminAccount.tokenSigningKey="$signingKey" \
  --wait --timeout 10m
wait_deploys kargo
