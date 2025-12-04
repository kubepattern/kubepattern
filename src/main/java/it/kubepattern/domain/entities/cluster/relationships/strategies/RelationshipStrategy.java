package it.kubepattern.domain.entities.cluster.relationships.strategies;

import io.kubernetes.client.apimachinery.KubernetesResource;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.K8sResource;

public interface RelationshipStrategy {
    void analyze(K8sCluster cluster);

    boolean involve(K8sResource resource);
}
