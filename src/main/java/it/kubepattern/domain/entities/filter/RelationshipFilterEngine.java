package it.sigemi.domain.entities.filter;

import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.K8sPatternResource;
import it.sigemi.domain.entities.pattern.RelationshipDefinition;
import it.sigemi.domain.entities.pattern.PatternDefinition;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

@Slf4j
@AllArgsConstructor
public class RelationshipFilterEngine {

    public List<K8sPatternResource> filterResources(PatternDefinition def, List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sPatternResource> resourcesToFilterOut = new ArrayList<>();
        int minRelPoints = def.getMinRelPoints();
        log.info("Minimum points to match: {}", minRelPoints);

        //Grouping resources by their name for quick access
        Map<String, List<K8sPatternResource>> resourceMap = resources.stream()
                .collect(Collectors.groupingBy(K8sPatternResource::getName));

        //Retrieve all leaders
        List<K8sPatternResource> leaders = resources.stream()
                .filter(r -> r.getName().equals(def.getLeaderId()))
                .toList();

        for (K8sPatternResource leader : leaders) {
            boolean requiredRelationshipsSatisfied = true;
            int unsharedPoint = 0;

            for (RelationshipDefinition relationship : def.getRelationships()) {
                String leaderId = relationship.getResources()[0];
                String followerId = relationship.getResources()[1];

                // Verify that we are processing the correct leader
                if (!leader.getName().equals(leaderId)) {
                    continue;
                }

                // Retrieve follower candidates
                List<K8sPatternResource> followerCandidates = resourceMap.getOrDefault(followerId, Collections.emptyList());

                // Default: no candidates found for follower
                boolean existsRelationWithAnyCandidate = false;

                //Verify relationships with all follower candidates
                for (K8sPatternResource follower : followerCandidates) {
                    if (cluster.isNeighbour(leader.getResource(), follower.getResource(), relationship.getRelationshipType())) {
                        existsRelationWithAnyCandidate = true;
                        break; // No need to check further if one match is found
                    }
                }

                /*
                 * shared=true  => At least one relationship must be found. (Positive Match)
                 * shared=false => Must be no matches found (Negative Match)
                 */

                if (relationship.isRequired()) {

                    if (relationship.isShared()) {
                        // Required relationship but not found
                        if (!existsRelationWithAnyCandidate) {
                            requiredRelationshipsSatisfied = false;
                            log.info("Required Shared failed: {} -> {}. Expected neighbour, found none.", leaderId, followerId);
                        }
                    } else {
                        // Relationship must NOT exist but was found
                        if (existsRelationWithAnyCandidate) {
                            requiredRelationshipsSatisfied = false;
                            log.info("Required NOT Shared failed: {} -> {}. Expected NO neighbour, found one.", leaderId, followerId);
                        }
                    }
                } else {
                    // If not required, accumulate points if matched with share criteria
                    if (relationship.isShared()) {
                        if (existsRelationWithAnyCandidate) {
                            unsharedPoint += relationship.getWeight();
                            log.debug("Points +{} (Shared matched): {} -> {}", relationship.getWeight(), leaderId, followerId);
                        }
                    } else {
                        if (!existsRelationWithAnyCandidate) {
                            unsharedPoint += relationship.getWeight();
                            log.debug("Points +{} (Not Shared matched): {} -> {}", relationship.getWeight(), leaderId, followerId);
                        }
                    }
                }
            }

            // Filtering phase for leaders not meeting criteria
            if (!requiredRelationshipsSatisfied) {
                resourcesToFilterOut.add(leader); // missing required relationships
            } else if (unsharedPoint < minRelPoints) {
                resourcesToFilterOut.add(leader); // points below threshold
                log.debug("Leader {} filtered: Points {} < Min {}", leader.getName(), unsharedPoint, minRelPoints);
            }
        }

        // Removing filtered resources from the main list
        resources.removeAll(resourcesToFilterOut);

        return resources;
    }

}
