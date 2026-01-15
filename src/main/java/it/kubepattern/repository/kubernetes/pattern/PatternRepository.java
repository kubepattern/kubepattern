package it.kubepattern.repository.kubernetes.pattern;

import io.kubernetes.client.openapi.ApiException;
import io.kubernetes.client.openapi.apis.CustomObjectsApi;
import it.kubepattern.application.configuration.AppConfig;
import it.kubepattern.domain.entities.cluster.K8sPatternResource;
import it.kubepattern.domain.entities.pattern.K8sPattern;
import it.kubepattern.domain.entities.pattern.K8sPatternScore;
import it.kubepattern.utils.PatternIdGenerator;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.slf4j.Slf4j;
import org.jspecify.annotations.NonNull;
import org.springframework.stereotype.Repository;
import java.util.*;

@Slf4j
@Repository
@Getter
@Setter
@AllArgsConstructor
public class PatternRepository implements IK8sPatternRepository {
    private static final String GROUP = "kubepattern.it";
    private static final String VERSION = "v1";
    private static final String PLURAL = "k8spatterns";
    private final AppConfig appConfig;

    private CustomObjectsApi api;

    @Override
    public void savePattern(K8sPattern pattern) {
        Map<String, Object> crBody = new HashMap<>();
        crBody.put("apiVersion", GROUP + "/" + VERSION);
        crBody.put("kind", "K8sPattern");
        String namespace = appConfig.getReport().getTargetNamespace();
        if(appConfig.getReport().isSaveInNamespace()) {
            if(pattern.getResources().length == 0 || pattern.getResources()[0].getNamespace() == null) {
                log.warn("⚠️ Cannot save K8sPattern '{}' in namespace because the first resource has no namespace defined. Saving in default namespace.", pattern.getMetadata().getName());
            }else {
                namespace = pattern.getResources()[0].getNamespace();
            }
        }

        // Metadata
        crBody.put("metadata", parseCRDMetadata(pattern, namespace));

        // Spec
        crBody.put("spec", parseCRDSpec(pattern));
        if(appConfig.getReport().isSaveInNamespace()) {
            namespace = pattern.getResources()[0].getNamespace();
        }

        // API Call
        try {
            api.createNamespacedCustomObject(
                    GROUP,
                    VERSION,
                    namespace,
                    PLURAL,
                    crBody
            ).execute();

            log.info("✅ Instance of K8sPattern '{}' created in namespace {}.", pattern.getMetadata().getName(), namespace);
        }catch (ApiException e) {
            if(e.toString().contains("AlreadyExists")) {
                log.info("↗️ Pattern {} already exists in namespace {}.", pattern.getMetadata().getName(), namespace);
            }else{
                log.info(e.getMessage());
            }
        }
    }

    private static @NonNull List<Map<String, Object>> getResourcesList(K8sPattern pattern) {
        List<Map<String, Object>> resourcesList = new ArrayList<>();

        for (K8sPatternResource patternResource : pattern.getResources()) {
            Map<String, Object> resourceMap = new HashMap<>();
            resourceMap.put("name", patternResource.getResource().getName());
            if(patternResource.getNamespace() != null) {
                resourceMap.put("namespace", patternResource.getNamespace());
            }
            resourceMap.put("uid", patternResource.getUid());
            resourceMap.put("role", patternResource.getName());

            resourcesList.add(resourceMap);
        }
        return resourcesList;
    }

    private static @NonNull List<Map<String, Object>> getScores(K8sPattern pattern) {
        List<Map<String, Object>> scoresList = new ArrayList<>();
        for (K8sPatternScore score : pattern.getScores()) {
            Map<String, Object> scoreMap = new HashMap<>();
            scoreMap.put("category", score.getCategory());
            scoreMap.put("score", score.getScore());
            scoresList.add(scoreMap);
        }
        return scoresList;
    }

    public static @NonNull Map<String, Object> parseCRDSpec(K8sPattern pattern) {
        Map<String, Object> spec = new HashMap<>();
        spec.put("type", pattern.getMetadata().getType().name());
        spec.put("name", pattern.getMetadata().getName());
        spec.put("referenceLink", pattern.getMetadata().getDocUrl());
        spec.put("message", pattern.generateMessage());
        spec.put("severity", pattern.getMetadata().getSeverity());
        //spec.put("confidence", pattern.calculateConfidence());
        spec.put("resources", getResourcesList(pattern));
        return spec;
    }

    public static @NonNull Map<String, Object> parseCRDMetadata(K8sPattern pattern, String namespace) {
        Map<String, Object> metadata = new HashMap<>();
        String crdName;
        crdName = pattern.getMetadata().getName()
                .toLowerCase()
                .replace(' ', '-')
                .concat("-" + PatternIdGenerator.generateHash(pattern));

        metadata.put("name", crdName);
        metadata.put("namespace", namespace);
        return metadata;
    }

    @SuppressWarnings("unchecked")
    @Override
    public void deletePatterns() {
        log.info("Starting deletion of all K8sPatterns in all namespaces...");
        try {
            // 1. Fetch all K8sPatterns across all namespaces.
            Object response = api.listClusterCustomObject(
                    GROUP,
                    VERSION,
                    PLURAL
            ).execute();

            if (response instanceof Map) {
                Map<String, Object> result = (Map<String, Object>) response;
                List<Map<String, Object>> items = (List<Map<String, Object>>) result.get("items");

                if (items == null || items.isEmpty()) {
                    log.info("No patterns found to delete.");
                    return;
                }

                log.info("Found {} patterns to delete.", items.size());

                // 2. Iterate and delete each resource in its specific namespace
                for (Map<String, Object> item : items) {
                    Map<String, Object> metadata = (Map<String, Object>) item.get("metadata");
                    String name = (String) metadata.get("name");
                    String namespace = (String) metadata.get("namespace");

                    try {
                        api.deleteNamespacedCustomObject(
                                GROUP,
                                VERSION,
                                namespace,
                                PLURAL,
                                name
                        ).execute();
                        log.info("Deleted pattern '{}' in namespace '{}'.", name, namespace);
                    } catch (ApiException e) {
                        // 404 Not Found is acceptable
                        if (e.getCode() != 404) {
                            log.error("Error deleting pattern '{}' in namespace '{}': {}", name, namespace, e.getMessage());
                        }
                    }
                }
            }
        } catch (ApiException e) {
            log.error("Failed to list patterns for deletion. API Message: {}", e.getMessage());
        } catch (Exception e) {
            log.error("Unexpected error during pattern deletion: {}", e.getMessage());
        }
    }
}