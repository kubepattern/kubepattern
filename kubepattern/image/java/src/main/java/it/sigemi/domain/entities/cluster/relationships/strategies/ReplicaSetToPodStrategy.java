package it.sigemi.domain.entities.cluster.relationships.strategies;

import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.K8sRelationship;
import it.sigemi.domain.entities.cluster.K8sResource;
import it.sigemi.domain.entities.cluster.relationships.RelationshipType;

public class ReplicaSetToPodStrategy implements RelationshipStrategy {
    @Override
    public void analyze(K8sCluster cluster) {
        for (var replicaSet : cluster.getResourcesByKind("ReplicaSet")) {
            for (var pod : cluster.getResourcesByKind("Pod")) {
                if (replicaSet.hasSameLabelValue(pod, "app")) {
                    cluster.addRelationship(replicaSet, pod, new K8sRelationship(RelationshipType.MANAGES));
                }
            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        return resource.getKind().equals("ReplicaSet") || resource.getKind().equals("Pod");
    }
}
