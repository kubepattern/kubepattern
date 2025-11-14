package it.sigemi.application.service.cluster;

import io.kubernetes.client.openapi.ApiException;
import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.repository.kubernetes.resource.OfficialClientResourceRepository;
import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@AllArgsConstructor
public class ClusterPopulateService {

    OfficialClientResourceRepository repository;
    ClusterRelationshipService clusterRelationshipService;

    void populateByNamespace(K8sCluster cluster, String namespace) throws ApiException {
        try {
            repository.getAllNamespaceResources(namespace).forEach(cluster::addResource);
            clusterRelationshipService.analyzeForRelationship(cluster);
        } catch (ApiException e) {
            throw new RuntimeException(e);
        }
    }

    void populateAllNamespaces(K8sCluster cluster) throws ApiException {
        repository.getAllResources().forEach(cluster::addResource);
        clusterRelationshipService.analyzeForRelationship(cluster);
    }
}
