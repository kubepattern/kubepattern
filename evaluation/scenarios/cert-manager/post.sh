#!/usr/bin/env bash
# probe G8 set-up, idempotent: Certificate api is first issued by ci-old-ca, then rotated to
# ci-new-ca; the retained CertificateRequest (revisionHistoryLimit: 3) still names ci-old-ca.
set -euo pipefail
if ! kubectl -n kp-certs get certificate api >/dev/null 2>&1; then
  cat <<'YAML' | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: api
  namespace: kp-certs
spec:
  secretName: api-tls
  dnsNames: ["api.kp-certs.svc"]
  revisionHistoryLimit: 3
  issuerRef: {name: ci-old-ca, kind: ClusterIssuer}
YAML
  kubectl -n kp-certs wait certificate/api --for=condition=Ready --timeout=120s
  kubectl -n kp-certs patch certificate api --type=merge -p '{"spec":{"issuerRef":{"name":"ci-new-ca"}}}'
  sleep 5
  kubectl -n kp-certs wait certificate/api --for=condition=Ready --timeout=120s
fi
kubectl -n kp-certs get certificaterequests -o custom-columns=NAME:.metadata.name,ISSUER:.spec.issuerRef.name
