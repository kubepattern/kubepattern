package it.kubepattern.repository.kubernetes.resource;

import io.kubernetes.client.common.KubernetesListObject;
import io.kubernetes.client.common.KubernetesObject;
import io.kubernetes.client.util.generic.dynamic.DynamicKubernetesObject;
import io.kubernetes.client.openapi.*;
import io.kubernetes.client.openapi.apis.*;
import io.kubernetes.client.openapi.models.V1ListMeta;
import it.kubepattern.domain.entities.cluster.K8sResource;
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
    private final CustomObjectsApi customObjectsApi;

    private enum ResourceConfig {

        //Krateo Resources - usando CustomObjectFetcher
        KRATEO_TABLE("widgets.templates.krateo.io/v1beta1", "Table", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "tables"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "tables")),

        KRATEO_MARKDOWN("widgets.templates.krateo.io/v1beta1", "Markdown", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "markdowns"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "markdowns")),

        KRATEO_ROW("widgets.templates.krateo.io/v1beta1", "Row", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "rows"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "rows")),

        KRATEO_COLUMN("widgets.templates.krateo.io/v1beta1", "Column", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "columns"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "columns")),

        KRATEO_PAGES("widgets.templates.krateo.io/v1beta1", "Page", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "pages"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "pages")),

        KRATEO_PARAGRAPH("widgets.templates.krateo.io/v1beta1", "Paragraph", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "paragraphs"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "paragraphs")),

        KRATEO_DATAGRID("widgets.templates.krateo.io/v1beta1", "DataGrid", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "datagrids"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "datagrids")),

        KRATEO_PIECHART("widgets.templates.krateo.io/v1beta1", "PieChart", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "piecharts"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "piecharts")),
        KRATEO_LINECHART("widgets.templates.krateo.io/v1beta1", "LineChart", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "linecharts"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "linecharts")),
        KRATEO_BRACHART("widgets.templates.krateo.io/v1beta1", "BarChart", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "barcharts"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "barcharts")),

        KRATEO_FILTERS("widgets.templates.krateo.io/v1beta1", "Filters", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "filters"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "filters")),

        KRATEO_PANEL("widgets.templates.krateo.io/v1beta1", "Panel", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "panels"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "panels")),

        KRATEO_TABLIST("widgets.templates.krateo.io/v1beta1", "TabList", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "tablists"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "tablists")),

        KRATEO_NAVMENUITEM("widgets.templates.krateo.io/v1beta1", "NavMenuItem", true,
                (repo, ns) -> repo.fetchCustomResource("widgets.templates.krateo.io", "v1beta1", ns, "navmenuitems"),
                repo -> repo.fetchCustomResourceAllNamespaces("widgets.templates.krateo.io", "v1beta1", "navmenuitems")),

        KRATEO_RESTACTION("templates.krateo.io/v1", "RESTAction", true, (repo, ns) -> repo.fetchCustomResource("templates.krateo.io/v1", "v1", ns, "restactions"),
                repo -> repo.fetchCustomResourceAllNamespaces("templates.krateo.io", "v1", "restactions")),
        // Kubernetes Resources

        POD("v1", "Pod", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedPod(ns).execute(),
                repo -> repo.coreV1Api.listPodForAllNamespaces().execute()),

        DEPLOYMENT("apps/v1", "Deployment", true,
                (repo, ns) -> repo.appsV1Api.listNamespacedDeployment(ns).execute(),
                repo -> repo.appsV1Api.listDeploymentForAllNamespaces().execute()),

        REPLICA_SET("apps/v1", "ReplicaSet", true,
                (repo, ns) -> repo.appsV1Api.listNamespacedReplicaSet(ns).execute(),
                repo -> repo.appsV1Api.listReplicaSetForAllNamespaces().execute()),

        SERVICE("v1", "Service", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedService(ns).execute(),
                repo -> repo.coreV1Api.listServiceForAllNamespaces().execute()),

        CONFIG_MAP("v1", "ConfigMap", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedConfigMap(ns).execute(),
                repo -> repo.coreV1Api.listConfigMapForAllNamespaces().execute()),

        LIMIT_RANGE("v1", "LimitRange", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedLimitRange(ns).execute(),
                repo -> repo.coreV1Api.listLimitRangeForAllNamespaces().execute()),

        RESOURCE_QUOTAS("v1", "ResourceQuota", true,
                (repo, ns) -> repo.coreV1Api.listNamespacedResourceQuota(ns).execute(),
                repo -> repo.coreV1Api.listResourceQuotaForAllNamespaces().execute()),

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

    private KubernetesListObject fetchCustomResource(String group, String version, String namespace, String plural) throws ApiException {
        try {
            Object result = customObjectsApi.listNamespacedCustomObject(group, version, namespace, plural).execute();
            return convertToKubernetesListObject(result);
        } catch (ApiException e) {

            if (e.getCode() == 403) {
                log.debug("Access forbidden to custom resource {}/{}. Skipping.", group, plural);
                return convertToKubernetesListObject(Collections.emptyMap());
            }

            // SE è un 404, significa che la CRD non esiste nel cluster: restituiamo una lista vuota.
            if (e.getCode() == 404) {
                log.debug("Custom resource {}/{} not found (CRD likely missing). Returning empty list.", group, plural);
                return convertToKubernetesListObject(Collections.emptyMap());
            }

            log.error("Error fetching custom resource {}/{} in namespace {}", group, plural, namespace, e);
            log.info(e.getMessage());
            return convertToKubernetesListObject(Collections.emptyMap());
        }
    }

    private KubernetesListObject fetchCustomResourceAllNamespaces(String group, String version, String plural) throws ApiException {
        try {
            Object result = customObjectsApi.listCustomObjectForAllNamespaces(group, version, plural).execute();
            return convertToKubernetesListObject(result);
        } catch (ApiException e) {
            // SE è un 404, significa che la CRD non esiste nel cluster: restituiamo una lista vuota.
            if (e.getCode() == 404) {
                log.debug("Custom resource {}/{} not found (CRD likely missing). Returning empty list.", group, plural);
                return convertToKubernetesListObject(Collections.emptyMap());
            }

            if (e.getCode() == 403) {
                log.debug("Access forbidden to custom resource {}/{}. Skipping.", group, plural);
                return convertToKubernetesListObject(Collections.emptyMap());
            }

            log.error("Error fetching custom resource {}/{} for all namespaces", group, plural, e);
            log.info(e.getMessage());
            return convertToKubernetesListObject(Collections.emptyMap());
        }
    }

    @SuppressWarnings("unchecked")
    private KubernetesListObject convertToKubernetesListObject(Object result) {
        if (result instanceof KubernetesListObject) {
            return (KubernetesListObject) result;
        }

        // Se è un LinkedTreeMap o un oggetto generico, creiamo un wrapper
        if (result instanceof Map) {
            Map<String, Object> resultMap = (Map<String, Object>) result;

            return new KubernetesListObject() {
                @Override
                public List<? extends KubernetesObject> getItems() {
                    if (resultMap.containsKey("items") && resultMap.get("items") instanceof List) {
                        List<Object> items = (List<Object>) resultMap.get("items");

                        return items.stream()
                                .filter(item -> item instanceof Map)
                                .map(item -> createDynamicKubernetesObject((Map<String, Object>) item))
                                .toList();
                    }
                    return new ArrayList<>();
                }

                @Override
                public String getApiVersion() {
                    return resultMap.containsKey("apiVersion") ?
                            (String) resultMap.get("apiVersion") : null;
                }

                @Override
                public String getKind() {
                    return resultMap.containsKey("kind") ?
                            (String) resultMap.get("kind") : null;
                }

                @Override
                public V1ListMeta getMetadata() {
                    if (resultMap.containsKey("metadata") && resultMap.get("metadata") instanceof Map) {
                        // Opzionale: convertire i metadata se necessario
                        return null;
                    }
                    return null;
                }
            };
        }

        throw new IllegalArgumentException("Cannot convert result to KubernetesListObject: " + result.getClass().getName());
    }

    //CRD
    private DynamicKubernetesObject createDynamicKubernetesObject(Map<String, Object> itemMap) {
        com.google.gson.Gson gson = new com.google.gson.Gson();
        com.google.gson.JsonObject jsonObject = gson.toJsonTree(itemMap).getAsJsonObject();
        return new DynamicKubernetesObject(jsonObject);
    }

    /*private DynamicKubernetesObject createDynamicKubernetesObject(Map<String, Object> itemMap) {
        try {
            // Convertiamo la Map in JsonObject preservando i tipi corretti
            com.google.gson.Gson gson = new com.google.gson.GsonBuilder()
                    .serializeNulls()
                    .create();

            // Prima convertiamo in stringa JSON e poi in JsonObject
            // Questo preserva meglio i tipi originali
            String jsonString = gson.toJson(itemMap);
            com.google.gson.JsonObject jsonObject = gson.fromJson(jsonString, com.google.gson.JsonObject.class);

            // Creiamo il DynamicKubernetesObject dal JsonObject
            // Questo preserverà automaticamente spec, status, metadata e tutti gli altri campi
            return new DynamicKubernetesObject(jsonObject);
        } catch (Exception e) {
            log.error("Error creating DynamicKubernetesObject from map", e);
            // Fallback: creiamo un oggetto vuoto
            return new DynamicKubernetesObject(new com.google.gson.JsonObject());
        }
    }*/


    // All resources By Kind in a Namespace
    public List<K8sResource> getResources(String kind, String namespace) throws ApiException {
        ResourceConfig config = findResourceConfig(kind);
        try{
            if (config.namespaced && namespace != null) {
                return fetchNamespacedResources(config, namespace);
            } else if (!config.namespaced) {
                return fetchClusterResources(config);
            }
        } catch (Exception e) {
            log.error("Error fetching resources {} in namespace {}: {}", kind, namespace,e.getMessage());
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
                log.info(e.getMessage());
                return Collections.emptyList();
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

        try {
            KubernetesListObject listObject = config.namespacedFetcher.fetch(this, namespace);
            return convertToK8sResources(listObject, config.apiVersion, config.kind);
        } catch (ApiException e) {

            if (e.getCode() == 403) {
                log.debug("Access forbidden for resource kind: {} in namespace: {}. Skipping.", config.kind, namespace);
                return Collections.emptyList();
            }

            if (e.getCode() == 404) {
                return Collections.emptyList();
            }
            log.info(e.getMessage());
            return Collections.emptyList();
        }
    }

    private List<K8sResource> fetchClusterResources(ResourceConfig config) throws ApiException {
        if (config.clusterFetcher == null) {
            throw new IllegalStateException(
                    String.format("No cluster fetcher for %s", config.kind)
            );
        }

        try {
            KubernetesListObject listObject = config.clusterFetcher.fetch(this);
            return convertToK8sResources(listObject, config.apiVersion, config.kind);
        } catch (ApiException e) {
            if (e.getCode() == 403) {
                log.debug("Access forbidden for cluster resource kind: {}. Skipping.", config.kind);
                return Collections.emptyList();
            }
            if (e.getCode() == 404) {
                return Collections.emptyList();
            }
            log.info(e.getMessage());
            return Collections.emptyList();
        }
    }

    private List<K8sResource> convertToK8sResources(
            KubernetesListObject listObject, String apiVersion, String kind) {
        return listObject.getItems().stream()
                .map(item -> new K8sResource(apiVersion, kind, item))
                .toList();
    }
}