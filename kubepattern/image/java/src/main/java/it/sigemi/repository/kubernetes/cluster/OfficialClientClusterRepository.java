package it.sigemi.repository.kubernetes.cluster;

import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.ApiException;
import it.sigemi.domain.entities.cluster.K8sCluster;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Repository;

@Repository
@Slf4j
public class OfficialClientClusterRepository implements IK8sClusterRepository {

    private final ApiClient apiClient;
    private K8sCluster activeCluster;

    public OfficialClientClusterRepository(ApiClient apiClient) {
        this.apiClient = apiClient;
    }

    @Override
    public void connect() throws ApiException {
        log.info("Connecting to Kubernetes cluster...");
        if (activeCluster == null) {
            String clusterUrl = apiClient.getBasePath();
            String clusterName = extractClusterName();
            activeCluster = new K8sCluster(clusterName, clusterUrl);
            checkClusterStatus();
            log.info("Connected to cluster: {} at {}", clusterName, clusterUrl);
        }
    }

    @Override
    public void checkClusterStatus() throws ApiException {
        if (activeCluster == null || apiClient == null || apiClient.getBasePath() == null) {
            throw new ApiException("Cluster not connected");
        }
    }

    @Override
    public K8sCluster getActiveCluster() throws ApiException {
        if (activeCluster == null) {
            throw new ApiException("");
        }
        return activeCluster;
    }

    private String extractClusterName() {
        try {
            return apiClient.getAuthentication("BearerToken") != null
                    ? "kubernetes-cluster"
                    : "local-cluster";
        } catch (Exception e) {
            return "unknown-cluster";
        }
    }
}
