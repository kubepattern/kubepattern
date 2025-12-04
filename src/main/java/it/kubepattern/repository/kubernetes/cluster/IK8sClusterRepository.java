package it.kubepattern.repository.kubernetes.cluster;

import io.kubernetes.client.openapi.ApiException;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import org.springframework.stereotype.Repository;

@Repository
public interface IK8sClusterRepository {
    void connect() throws ApiException;
    K8sCluster getActiveCluster() throws ApiException;
    void checkClusterStatus() throws ApiException;
}
