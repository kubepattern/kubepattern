#!/usr/bin/env bash
# The self-referencing clash (Pooler named like the Cluster it points to) is blocked upstream
# by the CNPG admission webhook: record the rejection for the RQ3 oracle table.
set -uo pipefail
out="$(cat <<'YAML' | kubectl apply -f - 2>&1
apiVersion: postgresql.cnpg.io/v1
kind: Pooler
metadata:
  name: db-live
  namespace: kp-db-a
spec:
  cluster:
    name: db-live
  instances: 0
  type: rw
  pgbouncer:
    poolMode: session
YAML
)"
echo "self-referencing Pooler: $out"
kubectl -n kp-db-a wait clusters.postgresql.cnpg.io/db-live --for=condition=Ready --timeout=300s || true
