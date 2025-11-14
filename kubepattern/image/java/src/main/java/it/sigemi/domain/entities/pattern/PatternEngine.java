package it.sigemi.domain.entities.pattern;

import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.K8sPatternResource;
import it.sigemi.domain.entities.cluster.commonRelationships.CommonRelationshipDefinition;
import lombok.extern.slf4j.Slf4j;

import java.util.*;

@Slf4j
public class PatternEngine {

    public List<K8sPattern> generatePatterns(PatternDefinition definition, List<K8sPatternResource> resources, K8sCluster cluster) {

        switch (definition.getPatternTopology()) {
            case LEADER_FOLLOWER ->  {
                return generateLeaderFollowerPatterns(definition, resources, cluster);
            }
            case SINGLE -> {
                return generateSinglePatterns(definition, resources, cluster);
            }
            case null, default -> {
                return new ArrayList<>();
            }
        }
    }

    private List<K8sCommonRelationship> generateLeaderFollowerCommonRelationships(PatternDefinition definition, List<K8sPatternResource>  resources, K8sCluster cluster) {
        List<K8sCommonRelationship> result = new ArrayList<>();
        List<CommonRelationshipDefinition> commonRelationshipDefinitions = definition.getCommonRelationshipDefinitions();
        int points;
        for (K8sPatternResource leader : resources) {
            for(K8sPatternResource follower : resources) {
                points = 0;
                for(CommonRelationshipDefinition commonRelationshipDefinition : commonRelationshipDefinitions) {
                    if(!leader.equals(follower)) {
                        if(cluster.sameNeighbour(leader.getResource(), follower.getResource(), commonRelationshipDefinition.getRelationshipType())){
                            points ++;
                            log.info(String.valueOf(commonRelationshipDefinition.getRelationshipType()));
                        }else if(commonRelationshipDefinition.isRequired()) {
                            points = 0;
                            break;
                        }
                    }
                }
                if(points > 0){
                    if(result.isEmpty()) {
                        result.add(new K8sCommonRelationship(Set.of(leader, follower), points));
                    }
                    for (K8sCommonRelationship k8sCommonRelationship : result) {
                        if(!k8sCommonRelationship.getRelationships().equals(Set.of(leader, follower))){
                            result.add(new K8sCommonRelationship(Set.of(leader, follower), points));
                        }
                    }
                }
            }
        }

        return result;
    }

    private List<K8sPattern> generateLeaderFollowerPatterns(PatternDefinition definition,  List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sCommonRelationship> k8sCommonRelationships = generateLeaderFollowerCommonRelationships(definition, resources, cluster);
        List<K8sPattern> patterns = new ArrayList<>();

        for (K8sCommonRelationship k8sCommonRelationship : k8sCommonRelationships) {
            List<K8sPatternScore> scores = new ArrayList<>();
            scores.add(new K8sPatternScore("CommonRelationships", "", k8sCommonRelationship.getPoints()));
            K8sPattern pattern = K8sPattern.builder()
                    .metadata(definition.getMetadata())
                    .message(definition.getMessage())
                    .resources(k8sCommonRelationship.getRelationships().toArray(new K8sPatternResource[0]))
                    .scores(scores)
                    .build();
            patterns.add(pattern);
        }

        return patterns;
    }

    private List<K8sPattern> generateSinglePatterns(PatternDefinition definition, List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sPattern> patterns = new ArrayList<>();
        log.info("Generating patterns for {}", resources);

        for(K8sPatternResource patternRes : resources) {
            if(patternRes.getName().equals(definition.getLeaderId())){
                K8sPattern pattern = K8sPattern.builder()
                        .metadata(definition.getMetadata())
                        .message(definition.getMessage())
                        .resources(new K8sPatternResource[] {patternRes})
                        .scores(new ArrayList<>())
                        .build();
                patterns.add(pattern);
            }
        }
        return patterns;
    }
}
