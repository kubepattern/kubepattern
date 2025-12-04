package it.kubepattern.domain.entities.cluster.relationships.strategies;

import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.K8sRelationship;
import it.kubepattern.domain.entities.cluster.K8sResource;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;

import java.util.List;

public class DeploymentToReplicaSetStrategy implements RelationshipStrategy{
    @Override
    public void analyze(K8sCluster cluster) {
        for (K8sResource deployment : cluster.getResourcesByKind("Deployment")) {
            for (K8sResource replicaSet : cluster.getResourcesByKind("ReplicaSet")) {
                if (deployment.hasSameLabelValue(replicaSet, "app")) {
                    cluster.addRelationship(deployment, replicaSet, new K8sRelationship(RelationshipType.OWNS));
                }
            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        return List.of("Deployment", "ReplicaSet").contains(resource.getKind());
    }
}
