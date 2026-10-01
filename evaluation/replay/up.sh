#!/usr/bin/env bash
# Starts etcd and kube-apiserver (pinned to KUBERNETES_VERSION) on localhost, without controllers.
# Writes $REPLAY_STATE/kubeconfig with a static admin token. Idempotent: an already running
# replay API server is reused.
set -euo pipefail
source "$(dirname "$0")/env.sh"
mkdir -p "$REPLAY_STATE" "$REPLAY_BIN"

fetch() { [[ -x "$REPLAY_BIN/$1" ]] || { curl -sSfL -o "$REPLAY_BIN/$1" "$2"; chmod +x "$REPLAY_BIN/$1"; }; }
fetch kube-apiserver "https://dl.k8s.io/release/$KUBERNETES_VERSION/bin/linux/amd64/kube-apiserver"
fetch kubectl "https://dl.k8s.io/release/$KUBERNETES_VERSION/bin/linux/amd64/kubectl"
if [[ ! -x "$REPLAY_BIN/etcd" ]]; then
  curl -sSfL "https://github.com/etcd-io/etcd/releases/download/$ETCD_VERSION/etcd-$ETCD_VERSION-linux-amd64.tar.gz" \
    | tar xz -C "$REPLAY_BIN" --strip-components=1 "etcd-$ETCD_VERSION-linux-amd64/etcd"
fi

if kubectl get --raw /readyz >/dev/null 2>&1; then echo "replay API server already running"; exit 0; fi

cd "$REPLAY_STATE"
[[ -f sa.key ]] || { openssl genrsa -out sa.key 2048 2>/dev/null; openssl rsa -in sa.key -pubout -out sa.pub 2>/dev/null; }
token="$(cat token 2>/dev/null || head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n' | tee token)"
echo "$token,admin,admin,system:masters" > tokens.csv

nohup etcd --data-dir "$REPLAY_STATE/etcd" --listen-client-urls http://127.0.0.1:2379 \
  --advertise-client-urls http://127.0.0.1:2379 --listen-peer-urls http://127.0.0.1:2380 \
  --log-level warn > etcd.log 2>&1 &
echo $! > etcd.pid
nohup kube-apiserver --etcd-servers=http://127.0.0.1:2379 --bind-address=127.0.0.1 \
  --advertise-address=127.0.0.1 --secure-port="$REPLAY_PORT" --cert-dir="$REPLAY_STATE/pki" \
  --token-auth-file="$REPLAY_STATE/tokens.csv" --authorization-mode=RBAC \
  --service-account-issuer=https://kubernetes.default.svc.cluster.local \
  --service-account-key-file="$REPLAY_STATE/sa.pub" \
  --service-account-signing-key-file="$REPLAY_STATE/sa.key" \
  --service-cluster-ip-range=10.96.0.0/16 --enable-priority-and-fairness=false \
  > apiserver.log 2>&1 &
echo $! > apiserver.pid

cat > kubeconfig <<KC
apiVersion: v1
kind: Config
clusters: [{name: replay, cluster: {server: "https://127.0.0.1:$REPLAY_PORT", insecure-skip-tls-verify: true}}]
users: [{name: admin, user: {token: "$token"}}]
contexts: [{name: replay, context: {cluster: replay, user: admin}}]
current-context: replay
KC
for _ in $(seq 1 60); do kubectl get --raw /readyz >/dev/null 2>&1 && { echo "replay API server ready ($KUBERNETES_VERSION)"; exit 0; }; sleep 1; done
echo "API server did not become ready; see $REPLAY_STATE/apiserver.log" >&2; tail -20 apiserver.log >&2; exit 1
