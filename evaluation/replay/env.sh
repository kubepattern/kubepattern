# Shared settings of the offline replay (sourced by every replay script).
# The replay runs a real kube-apiserver + etcd with the CRDs of the nine platforms and no
# controllers; objects that controllers would create are added by scenario.sh (the "shim").
REPLAY="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL="$(cd "$REPLAY/.." && pwd)"
source "$EVAL/env/versions.env"
: "${REPLAY_STATE:=$REPLAY/.state}"            # certs, etcd data, logs, kubeconfig (git-ignored)
: "${REPLAY_BIN:=$REPLAY/.bin}"                # pinned binaries (git-ignored)
: "${REPLAY_CACHE:=$REPLAY/.cache}"            # downloaded CRD manifests (git-ignored)
: "${REPLAY_PORT:=6443}"
: "${ETCD_VERSION:=v3.6.5}"
export KUBECONFIG="$REPLAY_STATE/kubeconfig"
export PATH="$REPLAY_BIN:$PATH"
