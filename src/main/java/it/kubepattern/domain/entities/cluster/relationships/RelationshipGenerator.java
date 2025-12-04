package it.kubepattern.domain.entities.cluster.relationships;

import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.strategies.RelationshipStrategy;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.List;

@Setter
@Getter
@AllArgsConstructor
@Component
@Slf4j
public class RelationshipGenerator {

    List<RelationshipStrategy> strategies;

    public void discoverRelationships(K8sCluster cluster) {
        for (RelationshipStrategy strategy : strategies) {
            try {
                log.debug("Executing strategy: {}", strategy.getClass().getSimpleName());
                strategy.analyze(cluster);
            } catch (Exception e) {
                log.error("Error executing strategy: {}", strategy.getClass().getSimpleName(), e);
            }
        }
        logRegisteredStrategies();
    }

    public void logRegisteredStrategies() {
        log.info("Registered {} relationship strategies:", strategies.size());
        strategies.forEach(s -> log.info("  - {}", s.getClass().getSimpleName()));
    }
}
