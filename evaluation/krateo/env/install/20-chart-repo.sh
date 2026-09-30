#!/usr/bin/env bash
# Packages the springboot-app blueprint and serves it from an in-cluster Helm repository
# (nginx + ConfigMap holding index.yaml and the .tgz) at $CHART_REPO_URL. Nothing is published outside the cluster.
source "$(dirname "$0")/lib.sh"
BP="$KRATEO/blueprint"
dist="$BP/dist"; rm -rf "$dist"; mkdir -p "$dist"

log "packaging $BLUEPRINT_NAME $BLUEPRINT_VERSION (with its vendored dependency)"
# update (not build): it resolves the pinned version by URL, without a local "helm repo add"
helm dependency update "$BP/$BLUEPRINT_NAME"
helm lint "$BP/$BLUEPRINT_NAME"
helm package "$BP/$BLUEPRINT_NAME" --version "$BLUEPRINT_VERSION" -d "$dist"
helm repo index "$dist" --url "$CHART_REPO_URL"

log "in-cluster chart repository ($CHART_REPO_URL)"
kubectl create namespace "$CHART_REPO_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "$CHART_REPO_NAMESPACE" create configmap charts \
  --from-file=index.yaml="$dist/index.yaml" \
  --from-file="$BLUEPRINT_NAME-$BLUEPRINT_VERSION.tgz"="$dist/$BLUEPRINT_NAME-$BLUEPRINT_VERSION.tgz" \
  --dry-run=client -o yaml | kubectl apply --server-side --force-conflicts -f -
kubectl apply -f - <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: charts
  namespace: $CHART_REPO_NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels: {app: charts}
  template:
    metadata:
      labels: {app: charts}
      annotations:
        kp-eval/charts-digest: "$(cat "$dist"/* | sha1sum | cut -c1-12)"
    spec:
      containers:
        - name: nginx
          image: docker.io/library/nginx:1.29-alpine
          ports: [{containerPort: 80}]
          resources:
            requests: {cpu: 10m, memory: 16Mi}
          volumeMounts:
            - {name: charts, mountPath: /usr/share/nginx/html, readOnly: true}
      volumes:
        - name: charts
          configMap: {name: charts}
---
apiVersion: v1
kind: Service
metadata:
  name: charts
  namespace: $CHART_REPO_NAMESPACE
spec:
  selector: {app: charts}
  ports: [{port: 80, targetPort: 80}]
YAML
kubectl -n "$CHART_REPO_NAMESPACE" rollout status deploy/charts --timeout=5m
kubectl -n "$CHART_REPO_NAMESPACE" run charts-probe --rm -i --restart=Never --image=docker.io/library/busybox:1.37 -- \
  wget -qO- "$CHART_REPO_URL/index.yaml" | grep -E 'version|urls' -A1
