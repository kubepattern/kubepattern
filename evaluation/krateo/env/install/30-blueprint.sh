#!/usr/bin/env bash
# Installs the springboot-app blueprint through a PortalBlueprintPage, then the two compositions.
source "$(dirname "$0")/lib.sh"
export MARKETPLACE_URL PORTAL_BLUEPRINT_PAGE_VERSION BLUEPRINT_NAME BLUEPRINT_VERSION CHART_REPO_URL
export PBP_CRD_VERSION="${PORTAL_BLUEPRINT_PAGE_VERSION//./-}"

wait_cd() {  # namespace name
  kubectl -n "$1" wait "compositiondefinitions.core.krateo.io/$2" --for=condition=Ready --timeout=10m
}

log "CompositionDefinition portal-blueprint-page $PORTAL_BLUEPRINT_PAGE_VERSION"
envsubst < "$KRATEO/scenario/blueprint-definition.yaml" | kubectl apply -f -
wait_cd krateo-system portal-blueprint-page
kubectl wait crd/portalblueprintpages.composition.krateo.io --for=condition=Established --timeout=5m

log "PortalBlueprintPage $BLUEPRINT_NAME (creates the CompositionDefinition of $BLUEPRINT_NAME in demo-system)"
envsubst < "$KRATEO/scenario/blueprint-page.yaml" | kubectl apply -f -
# Ready=True is not enough (it is also reported when the Helm release failed): wait for the definition itself.
cd_name() {
  kubectl -n demo-system get compositiondefinitions.core.krateo.io -o json \
    | jq -r --arg r "$BLUEPRINT_NAME" '.items[] | select(.spec.chart.repo == $r) | .metadata.name'
}
until [[ -n "$(cd_name)" ]]; do sleep 5; done
wait_cd demo-system "$(cd_name)"
kubectl wait crd/springbootapps.composition.krateo.io --for=condition=Established --timeout=5m

log "compositions"
kubectl apply -f "$KRATEO/scenario/compositions.yaml"
kubectl -n tenant-a wait springbootapps.composition.krateo.io/demo-app --for=condition=Ready --timeout=10m
kubectl -n tenant-b wait springbootapps.composition.krateo.io/demo-app-2 --for=condition=Ready --timeout=10m
kubectl get springbootapps.composition.krateo.io -A
