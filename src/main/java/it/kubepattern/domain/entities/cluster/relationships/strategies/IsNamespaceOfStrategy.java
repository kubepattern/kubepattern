package it.sigemi.domain.entities.cluster.relationships.strategies;

import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.relationships.K8sRelationship;
import it.sigemi.domain.entities.cluster.K8sResource;

import static it.sigemi.domain.entities.cluster.relationships.RelationshipType.IS_NAMESPACE_OF;

public class IsNamespaceOfStrategy implements RelationshipStrategy {

    @Override
    public void analyze(K8sCluster cluster) {
        for (K8sResource namespace : cluster.getResourcesByKind("Namespace")) {
            for (K8sResource resource : cluster.getResourcesByNamespace(namespace.getName())) {
                cluster.addRelationship(namespace, resource, new K8sRelationship(IS_NAMESPACE_OF));
            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        return false;
    }
}
