package it.kubepattern.domain.entities.cluster.relationships.strategies;

import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.K8sResource;
import io.kubernetes.client.openapi.models.V1Pod;
import io.kubernetes.client.openapi.models.V1PodSpec;
import io.kubernetes.client.openapi.models.V1ServiceAccount;

import it.kubepattern.domain.entities.cluster.relationships.K8sRelationship;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;

import java.util.Objects;

public class UsesServiceAccountStrategy implements RelationshipStrategy {

    @Override
    public void analyze(K8sCluster cluster) {
        for (K8sResource podResource : cluster.getResourcesByKind("Pod")) {
            V1Pod pod = (V1Pod) podResource.getObject();

            if (pod == null || pod.getSpec() == null || pod.getMetadata() == null) {
                continue;
            }

            V1PodSpec podSpec = pod.getSpec();
            String podNamespace = pod.getMetadata().getNamespace();

            String targetSaName = podSpec.getServiceAccountName();

            if (targetSaName == null || targetSaName.isEmpty()) {
                targetSaName = "default";
            }

            for (K8sResource saResource : cluster.getResourcesByKind("ServiceAccount")) {
                V1ServiceAccount sa = (V1ServiceAccount) saResource.getObject();

                if (sa == null || sa.getMetadata() == null) {
                    continue;
                }

                String saName = sa.getMetadata().getName();
                String saNamespace = sa.getMetadata().getNamespace();

                if (targetSaName.equals(saName) && Objects.equals(podNamespace, saNamespace)) {
                    cluster.addRelationship(podResource, saResource,
                            new K8sRelationship(RelationshipType.USES_SA));

                    break;
                }
            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        if (resource == null || resource.getKind() == null) {
            return false;
        }

        String kind = resource.getKind();
        return kind.equals("Pod") || kind.equals("ServiceAccount");
    }
}