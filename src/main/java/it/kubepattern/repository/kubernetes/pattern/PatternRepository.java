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
        // Metadata
        crBody.put("metadata", parseCRDMetadata(pattern, namespace));

        // Spec

        crBody.put("spec", parseCRDSpec(pattern));
        if(appConfig.getReport().isSaveInNamespace()) {
            namespace = pattern.getResources()[0].getNamespace();
        }

        // API Call
        try {
            Object result = api.createNamespacedCustomObject(
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
        spec.put("apiVersion", pattern.getMetadata().getVersion());
        spec.put("type", pattern.getMetadata().getType().name());
        spec.put("name", pattern.getMetadata().getName());
        spec.put("description", pattern.getMetadata().getDescription());
        spec.put("referenceLink", pattern.getMetadata().getDocUrl());
        spec.put("message", pattern.generateMessage());
        spec.put("severity", pattern.getMetadata().getSeverity());
        spec.put("category", pattern.getMetadata().getCategory());
        spec.put("confidence", pattern.calculateConfidence());

        // 5. Scores
        spec.put("scores", getScores(pattern));

        // 6. Resources
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
}