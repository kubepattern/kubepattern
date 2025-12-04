package it.kubepattern.application.service.cluster;

import io.kubernetes.client.openapi.ApiException;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.exception.NamespaceNotFoundException;
import it.kubepattern.repository.kubernetes.cluster.IK8sClusterRepository;
import jakarta.annotation.PostConstruct;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Service
@Slf4j
@AllArgsConstructor
public class ClusterLifecycleService {

    IK8sClusterRepository IK8SClusterRepository;
    ClusterPopulateService clusterPopulateService;


    @PostConstruct
    public void connectToCluster() {
        try {
            IK8SClusterRepository.connect();
        } catch (Exception e) {
            log.error("Error connecting to Kubernetes cluster: {}", e.getMessage());
        }
    }

    public K8sCluster getCluster() throws ApiException {
        return IK8SClusterRepository.getActiveCluster();
    }

    public K8sCluster getClusterAndPopulate() throws ApiException {
        clusterPopulateService.populateAllNamespaces(IK8SClusterRepository.getActiveCluster());
        return IK8SClusterRepository.getActiveCluster();
    }

    public K8sCluster getNamespaceAndPopulate(String namespace) throws NamespaceNotFoundException, ApiException {
        clusterPopulateService.populateByNamespace(IK8SClusterRepository.getActiveCluster(), namespace);
        return IK8SClusterRepository.getActiveCluster();
    }

    public String getClusterAsJson() throws ApiException {

        return getClusterAndPopulate().getAsJson();

    }
}
