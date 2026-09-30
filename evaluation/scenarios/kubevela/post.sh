#!/usr/bin/env bash
# The pinned-revision reference needs the DefinitionRevision created by KubeVela for kp-versioned.
set -euo pipefail
until kubectl -n kp-vela-a get definitionrevision kp-versioned-v1 >/dev/null 2>&1; do sleep 2; done
cat <<'YAML' | kubectl apply --server-side -f -
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata:
  name: a3
  namespace: kp-vela-a
spec:
  components:
    - name: pinned
      type: kp-versioned@v1
YAML
