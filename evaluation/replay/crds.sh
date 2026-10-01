#!/usr/bin/env bash
# Downloads the CRDs of the nine evaluated platforms at the versions pinned in
# ../env/versions.env and installs them on the replay API server (no controllers).
set -euo pipefail
source "$(dirname "$0")/env.sh"
mkdir -p "$REPLAY_CACHE"
cd "$REPLAY_CACHE"

get() { [[ -s "$1" ]] || curl -sSfL -o "$1" "$2" || { echo "download failed: $2" >&2; rm -f "$1"; return 1; }; }
gh_raw() { get "$1" "https://raw.githubusercontent.com/$2"; }

# cert-manager, CloudNativePG, Flux, Cluster API: release manifests.
get cert-manager.yaml "https://github.com/cert-manager/cert-manager/releases/download/$CERT_MANAGER_VERSION/cert-manager.crds.yaml"
get cnpg.yaml "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-${CNPG_VERSION%.*}/releases/cnpg-$CNPG_VERSION.yaml"
get flux.yaml "https://github.com/fluxcd/flux2/releases/download/$FLUX_VERSION/install.yaml"
for c in core-components control-plane-components bootstrap-components infrastructure-components-development; do
  get "capi-$c.yaml" "https://github.com/kubernetes-sigs/cluster-api/releases/download/$CAPI_VERSION/$c.yaml"
done

# External Secrets: the chart version equals the application tag.
gh_raw eso.yaml "external-secrets/external-secrets/v$ESO_CHART_VERSION/deploy/crds/bundle.yaml"

# Argo CD: the CRDs of the application version shipped by the pinned chart.
get argo-cd.tgz "https://github.com/argoproj/argo-helm/releases/download/argo-cd-$ARGOCD_CHART_VERSION/argo-cd-$ARGOCD_CHART_VERSION.tgz"
argo_app="$(tar xzf argo-cd.tgz -O argo-cd/Chart.yaml | awk '/^appVersion:/{print $2}' | tr -d '"')"
for c in application applicationset appproject; do
  gh_raw "argocd-$c.yaml" "argoproj/argo-cd/$argo_app/manifests/crds/$c-crd.yaml"
done

# Kargo, Crossplane, KubeVela: CRD files in the repositories at the pinned tags.
for c in projects stages warehouses freights; do
  gh_raw "kargo-$c.yaml" "akuity/kargo/v$KARGO_VERSION/charts/kargo/resources/crds/kargo.akuity.io_$c.yaml"
done
for c in apiextensions.crossplane.io_compositeresourcedefinitions apiextensions.crossplane.io_compositions \
         pkg.crossplane.io_functions ops.crossplane.io_operations ops.crossplane.io_cronoperations \
         ops.crossplane.io_watchoperations; do
  gh_raw "crossplane-$c.yaml" "crossplane/crossplane/v$CROSSPLANE_VERSION/cluster/crds/$c.yaml"
done
for c in applications componentdefinitions traitdefinitions definitionrevisions; do
  gh_raw "kubevela-$c.yaml" "kubevela/kubevela/v$KUBEVELA_VERSION/charts/vela-core/crds/core.oam.dev_$c.yaml"
done

python3 "$REPLAY/crds.py" ./*.yaml > "$REPLAY_STATE/crds.yaml"
kubectl apply --server-side --force-conflicts -f "$REPLAY_STATE/crds.yaml" >/dev/null
# KubePattern's own CRDs.
kubectl apply --server-side --force-conflicts -f "$EVAL/../charts/kubepattern/crds/" >/dev/null
kubectl wait --for=condition=Established crd --all --timeout=120s >/dev/null
echo "CRDs installed: $(kubectl get crd --no-headers | wc -l)"
