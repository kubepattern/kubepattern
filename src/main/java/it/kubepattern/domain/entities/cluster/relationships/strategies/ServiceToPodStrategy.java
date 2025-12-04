package it.kubepattern.domain.entities.cluster.relationships.strategies;

import io.kubernetes.client.openapi.models.V1Service;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.K8sRelationship;
import it.kubepattern.domain.entities.cluster.K8sResource;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;

import java.util.List;
import java.util.Map;

public class ServiceToPodStrategy implements RelationshipStrategy {

    @Override
    public void analyze(K8sCluster cluster) {
        List<K8sResource> services = cluster.getResourcesByKind("Service");
        List<K8sResource> pods = cluster.getResourcesByKind("Pod");

        for (K8sResource serviceResource : services) {
            V1Service service = (V1Service) serviceResource.getObject();

            Map<String, String> selector = service.getSpec().getSelector();

            if (selector == null || selector.isEmpty()) {
                continue;
            }

            for (K8sResource podResource : pods) {
                if (!serviceResource.sameNamespace(podResource)) {
                    continue;
                }

                if (podMatchesSelector(podResource, selector)) {
                    cluster.addRelationship(
                            serviceResource,
                            podResource,
                            new K8sRelationship(RelationshipType.EXPOSES)
                    );
                }
            }
        }
    }

    /**
     * Verifica se il Pod ha tutte le label richieste dal selector del Service
     */
    private boolean podMatchesSelector(K8sResource pod, Map<String, String> selector) {
        Map<String, String> podLabels = pod.getLabels();

        // Tutte le label del selector devono essere presenti nel Pod con lo stesso valore
        for (Map.Entry<String, String> selectorEntry : selector.entrySet()) {
            String key = selectorEntry.getKey();
            String value = selectorEntry.getValue();

            // Se il Pod non ha questa label o il valore è diverso, non matcha
            if (!podLabels.containsKey(key) || !podLabels.get(key).equals(value)) {
                return false;
            }
        }

        return true;
    }

    @Override
    public boolean involve(K8sResource resource) {
        return List.of("Service", "Pod").contains(resource.getKind());
    }
}