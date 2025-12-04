package it.kubepattern.application.service.cluster;

import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipGenerator;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.log4j.Log4j;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Service
@Getter
@Setter
@AllArgsConstructor
@Slf4j
public class ClusterRelationshipService {
    private final RelationshipGenerator relationshipGenerator;

    public void analyzeForRelationship(K8sCluster cluster) {
        relationshipGenerator.discoverRelationships(cluster);
    }
}
