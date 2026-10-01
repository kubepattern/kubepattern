#!/usr/bin/env bash
# Applies the controlled cases: seeds.yaml (widgets, RESTActions, RBAC) and the seeded CompositionDefinitions.
source "$(dirname "$0")/lib.sh"
log "seeds"
kubectl apply --server-side --force-conflicts -f "$KRATEO/scenario/seeds.yaml"
kubectl apply -f "$KRATEO/scenario/seeds-definitions.yaml"
for cd in github-scaffolding-with-composition-page portal-composition-page-generic; do
  kubectl -n kp-krateo-a wait "compositiondefinitions.core.krateo.io/$cd" --for=condition=Ready --timeout=10m
done
kubectl wait crd/portalcompositionpagegenerics.composition.krateo.io --for=condition=Established --timeout=5m
kubectl apply -f "$KRATEO/scenario/seeds-compositions.yaml"
kubectl -n kp-krateo-a wait portalcompositionpagegenerics.composition.krateo.io/page-generic-a --for=condition=Ready --timeout=10m
# Ready=True is also reported for a failed release: check Helm.
helm -n kp-krateo-a list -a
