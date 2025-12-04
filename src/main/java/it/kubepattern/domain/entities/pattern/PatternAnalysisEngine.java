package it.sigemi.domain.entities.pattern;

import io.kubernetes.client.openapi.ApiException;
import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.K8sPatternResource;
import it.sigemi.domain.entities.filter.RelationshipFilterEngine;
import it.sigemi.domain.entities.filter.ResourceFilterEngine;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class PatternAnalysisEngine {

    private final RelationshipFilterEngine relationshipFilterEngine;
    private final PatternEngine patternEngine;
    private final ResourceFilterEngine resourceFilterEngine;

    public List<K8sPattern> analyze(PatternDefinition patternDefinition, K8sCluster cluster) throws ApiException {

        List<K8sPatternResource> resources = patternEngine.retrievePatternResources(patternDefinition, cluster);

        resources = resourceFilterEngine.filterResources(patternDefinition, resources);
        resources = relationshipFilterEngine.filterResources(patternDefinition, resources, cluster);

        return patternEngine.generatePatterns(patternDefinition, resources, cluster);
    }
}
