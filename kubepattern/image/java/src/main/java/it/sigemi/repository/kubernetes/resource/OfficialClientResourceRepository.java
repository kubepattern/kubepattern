package it.sigemi.repository.kubernetes.resource;

import io.kubernetes.client.common.KubernetesListObject;
import io.kubernetes.client.openapi.*;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.BatchV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.apis.StorageV1Api;
import it.sigemi.domain.entities.cluster.K8sResource;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Repository;

import java.util.*;

@Slf4j
@Repository
@Getter
@Setter
@AllArgsConstructor
public class OfficialClientResourceRepository implements IK8sResourceRepository {

    private final ApiClient client;
    private final CoreV1Api coreV1Api;
    private final AppsV1Api appsV1Api;
    private final BatchV1Api batchApi;
    private final StorageV1Api storageV1Api;
    
    private enum ResourceConfig {
        POD("v1", "Pod", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedPod(ns).execute(),
                repo -> repo.coreV1Api.listPodForAllNamespaces().execute()),

        DEPLOYMENT("apps/v1", "Deployment", true,
                (repo, ns) -> repo.appsV1Api.listNamespacedDeployment(ns).execute(),
                repo -> repo.appsV1Api.listDeploymentForAllNamespaces().execute()),

        REPLICA_SET("apps/v1", "ReplicaSet", true,
                (repo, ns) -> repo.appsV1Api.listNamespacedReplicaSet(ns).execute(), repo  -> repo.appsV1Api.listReplicaSetForAllNamespaces().execute()),

        SERVICE("v1", "Service", true, (repo, ns) -> repo.coreV1Api.listNamespacedService(ns).execute(), repo -> repo.coreV1Api.listServiceForAllNamespaces().execute()),

        CONFIG_MAP("v1", "ConfigMap", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedConfigMap(ns).execute(),
                repo -> repo.coreV1Api.listConfigMapForAllNamespaces().execute()),

        SERVICE_ACCOUNT("v1", "ServiceAccount", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedServiceAccount(ns).execute(),
                repo -> repo.coreV1Api.listServiceAccountForAllNamespaces().execute()),

        LIMIT_RANGE("v1", "LimitRange", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedLimitRange(ns).execute(),
                repo -> repo.coreV1Api.listLimitRangeForAllNamespaces().execute()),

        RESOURCE_QUOTAS("v1", "ResourceQuota", true, (repo, ns) -> repo.coreV1Api.listNamespacedResourceQuota(ns).execute(),
                repo -> repo.coreV1Api.listResourceQuotaForAllNamespaces().execute()),

        SECRET("v1", "Secret", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedSecret(ns).execute(),
                repo -> repo.coreV1Api.listSecretForAllNamespaces().execute()),

        JOB("batch/v1", "Job", true,
                (repo, ns) -> repo.batchApi.listNamespacedJob(ns).execute(),
                repo -> repo.batchApi.listJobForAllNamespaces().execute()),

        CRON_JOB("batch/v1", "CronJob", true,
                (repo, ns) -> repo.batchApi.listNamespacedCronJob(ns).execute(),
                repo -> repo.batchApi.listCronJobForAllNamespaces().execute()),

        PERSISTENT_VOLUME_CLAIM("v1", "PersistentVolumeClaim", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedPersistentVolumeClaim(ns).execute(),
                repo -> repo.coreV1Api.listPersistentVolumeClaimForAllNamespaces().execute()),

        NAMESPACE("v1", "Namespace", false,
                null,
                repo -> repo.coreV1Api.listNamespace().execute()),

        NODE("v1", "Node", false,
                null,
                repo -> repo.coreV1Api.listNode().execute()),

        STORAGE_CLASS("storage.k8s.io/v1", "StorageClass", false,
                null,
                repo -> repo.storageV1Api.listStorageClass().execute());

        private final String apiVersion;
        private final String kind;
        private final boolean namespaced;
        private final NamespacedFetcher namespacedFetcher;
        private final ClusterFetcher clusterFetcher;

        ResourceConfig(String apiVersion, String kind, boolean namespaced,
                       NamespacedFetcher namespacedFetcher, ClusterFetcher clusterFetcher) {
            this.apiVersion = apiVersion;
            this.kind = kind;
            this.namespaced = namespaced;
            this.namespacedFetcher = namespacedFetcher;
            this.clusterFetcher = clusterFetcher;
        }

        @FunctionalInterface
        interface NamespacedFetcher {
            KubernetesListObject fetch(OfficialClientResourceRepository repo, String namespace) throws ApiException;
        }

        @FunctionalInterface
        interface ClusterFetcher {
            KubernetesListObject fetch(OfficialClientResourceRepository repo) throws ApiException;
        }
    }

    // All resources By Kind in a Namespace
    public List<K8sResource> getResources(String kind, String namespace) throws ApiException {
        ResourceConfig config = findResourceConfig(kind);

        if (config.namespaced && namespace != null) {
            return fetchNamespacedResources(config, namespace);
        } else if (!config.namespaced) {
            return fetchClusterResources(config);
        }

        throw new IllegalArgumentException(
                String.format("Resource %s requires a namespace", kind)
        );
    }

    //All resources by a List of Kind
    public List<K8sResource> getResourcesByKinds(List<String> kinds, String namespace) throws ApiException {
        List<K8sResource> allResources = new ArrayList<>();

        for (String kind : kinds) {
            try {
                allResources.addAll(getResources(kind, namespace));
            } catch (ApiException e) {
                log.error("Error fetching resources of kind: {} in namespace: {}", kind, namespace, e);
                throw new ApiException("Error fetching resources of kind: " + kind + " in namespace: " + namespace);
            }
        }

        return allResources;
    }

    // All resources in a namespace
    public List<K8sResource> getAllNamespaceResources(String namespace) throws ApiException {
        List<K8sResource> allResources = new ArrayList<>();

        for (ResourceConfig config : ResourceConfig.values()) {
            if (config.namespaced) {
                try {
                    allResources.addAll(fetchNamespacedResources(config, namespace));
                } catch (ApiException e) {
                    log.error("Error fetching {} in namespace: {}", config.kind, namespace, e);
                }
            }
        }

        return allResources;
    }

    // All Available Resources (namespace + cluster-scoped)
    public List<K8sResource> getAllResources() throws ApiException {
        List<K8sResource> allResources = new ArrayList<>();

        for (ResourceConfig config : ResourceConfig.values()) {
            try {
                if (config.namespaced && config.clusterFetcher != null) {
                    // cluster-wide (Pod, Deployment)
                    allResources.addAll(fetchClusterResources(config));
                } else if (!config.namespaced) {
                    // cluster-scoped (Node, Namespace, StorageClass)
                    allResources.addAll(fetchClusterResources(config));
                }
            } catch (ApiException e) {
                log.error("Error fetching all {}", config.kind, e);
                throw new ApiException(e.getMessage());
            }
        }

        return allResources;
    }

    // Helpers
    private ResourceConfig findResourceConfig(String kind) {
        return Arrays.stream(ResourceConfig.values())
                .filter(rc -> rc.kind.equalsIgnoreCase(kind))
                .findFirst()
                .orElseThrow(() -> new IllegalArgumentException("Unknown resource kind: " + kind));
    }

    private List<K8sResource> fetchNamespacedResources(ResourceConfig config, String namespace) throws ApiException {
        if (config.namespacedFetcher == null) {
            throw new IllegalStateException(
                    String.format("No namespaced fetcher for %s", config.kind)
            );
        }

        KubernetesListObject listObject = config.namespacedFetcher.fetch(this, namespace);
        return convertToK8sResources(listObject, config.apiVersion, config.kind);
    }

    private List<K8sResource> fetchClusterResources(ResourceConfig config) throws ApiException {
        if (config.clusterFetcher == null) {
            throw new IllegalStateException(
                    String.format("No cluster fetcher for %s", config.kind)
            );
        }

        KubernetesListObject listObject = config.clusterFetcher.fetch(this);
        return convertToK8sResources(listObject, config.apiVersion, config.kind);
    }

    private List<K8sResource> convertToK8sResources(
            KubernetesListObject listObject, String apiVersion, String kind) {
        return listObject.getItems().stream()
                .map(item -> new K8sResource(apiVersion, kind, item))
                .toList();
    }
}