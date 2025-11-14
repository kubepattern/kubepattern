package it.sigemi.application.service.pattern;

import io.kubernetes.client.openapi.ApiException;
import it.sigemi.application.service.cluster.ClusterLifecycleService;
import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.K8sPatternResource;
import it.sigemi.domain.entities.cluster.K8sResource;
import it.sigemi.domain.entities.cluster.RelationshipDefinition;
import it.sigemi.domain.entities.filter.ResourceFilter;
import it.sigemi.domain.entities.filter.ResourceFilterEngine;
import it.sigemi.domain.entities.pattern.K8sPattern;
import it.sigemi.domain.entities.pattern.PatternDefinition;
import it.sigemi.domain.entities.pattern.PatternEngine;
import it.sigemi.exception.MalformedPatternException;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Async;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;


@Slf4j
@AllArgsConstructor
@Service
public class PatternAnalysisOrchestratorService {

    ClusterLifecycleService clusterLifecycleService;
    FetchPatternDefinitionService fetchPatternDefinitionService;
    PatternService patternService;

    @Async
    public void startNamespaceAnalysis(String namespace) throws Exception {
        log.info("Starting namespace analysis...");
        K8sCluster cluster= clusterLifecycleService.getClusterAndPopulate();
        if(cluster.hasNamespace(namespace)){
            analyzePatterns(cluster);
        }else {
            throw new ApiException("Namespace not found");
        }
        log.info("Namespace analysis completed.");
    }

    @Async
    public void startClusterAnalysis() throws Exception {
        K8sCluster cluster = clusterLifecycleService.getClusterAndPopulate();
        analyzePatterns(cluster);
    }

    public void analyzePatterns(K8sCluster cluster) throws Exception{
        log.info("Analyzing patterns...");

        for (PatternDefinition patternDefinition : fetchPatternDefinitionService.getAllPatternDefinitions()) {
            if(patternDefinition==null){
                log.error("pattern definition is null");
            }else{
                try{
                    List<K8sPattern> patterns = analyze(patternDefinition, cluster);
                    log.info("Patterns {}", patterns);
                    savePatterns(patterns);
                }catch (MalformedPatternException e){
                    log.info(e.getMessage());
                }catch (Exception e) {
                    log.error("Error during cluster static analysis: {}", e.getMessage());
                    break;
                }
            }
        }
    }

    public void savePatterns(List<K8sPattern> patterns) throws ApiException {
        for(K8sPattern pattern: patterns) {
            log.info(pattern.toString());
            patternService.savePattern(pattern);
        }
    }

    public List<K8sPattern> analyze(PatternDefinition patternDefinition, K8sCluster cluster) throws ApiException {

        log.info("{}", cluster.getResourcesByKind("ResourceQuota"));
        log.info("RESOURCES");
        for (K8sResource resource : retrieveResources(patternDefinition.getResources().values().toArray(new String[0]),  cluster)) {
            log.info("Resource {}", resource);
        }

        List<K8sPatternResource> resources = retrievePatternResources(patternDefinition, cluster);

        log.info("RETRIEVED RESOURCES");
        for (K8sPatternResource resource : resources) {
            log.info("Resource {}", resource);
        }
        resources = filterResources(patternDefinition, resources);
        log.info("FILTERED RESOURCES");
        for (K8sPatternResource resource : resources) {
            log.info("Resource {}", resource);
        }
        resources = filterResourcesWithRelationships(patternDefinition, resources, cluster);
        log.info("RELATION FILTERED RESOURCES");
        for (K8sPatternResource resource : resources) {
            log.info("Resource {}", resource);
        }
        PatternEngine p = new PatternEngine();
        return p.generatePatterns(patternDefinition, resources, cluster);
    }

    public List<K8sResource> retrieveResources(String[] kinds, K8sCluster cluster) {
        List<K8sResource> resources = new ArrayList<>();
        for (String kind : kinds) {
            resources.addAll(cluster.getResourcesByKind(kind));
        }
        return resources;
    }

    public List<K8sPatternResource> retrievePatternResources(PatternDefinition patternDefinition, K8sCluster cluster) {
        List<K8sPatternResource> patternResources = new ArrayList<>();

        for(String id : patternDefinition.getResources().keySet()){
            log.info("RETRIEVED RESOURCE ID {} . {}", id,  patternDefinition.getResources().get(id));
            String kind = patternDefinition.getResources().get(id);
            List<K8sResource> resources = cluster.getResourcesByKind(kind);
            log.info(resources.size() + " resources found");
            for (K8sResource resource : resources){
                patternResources.add(new K8sPatternResource(id, resource));
            }
        }

        return patternResources;
    }

    public List<K8sPatternResource> filterResources(PatternDefinition patternDefinition, List<K8sPatternResource> resources) {
        List<K8sPatternResource> filteredResources = new ArrayList<>();
        for (K8sPatternResource resource : resources){
            ResourceFilter resourceFilter = patternDefinition.getResourceFilterById(resource.getName());
            if(ResourceFilterEngine.applyResourceFilter(resource.getResource(), resourceFilter)){
                filteredResources.add(resource);
            }
        }
        return filteredResources;
    }

    /*public List<K8sPatternResource> filterResourcesWithRelationships(PatternDefinition patternDefinition, List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sPatternResource> toRemove = new ArrayList<>();
        log.info("Filter ing resources with {} relationships", resources.size());
        log.info("Relationships to check: {}", patternDefinition.getRelationships());
        for (K8sPatternResource from : resources){
            for (K8sPatternResource to : resources){
                patternDefinition.getRelationships().forEach((relationship) -> {
                    String first = relationship.getResources()[0];
                    String second = relationship.getResources()[0];
                    if(from.getName().equals(first) && to.getName().equals(second) && !from.equals(to)){
                        boolean neighbours = cluster.getNeighbours(from.getResource(), relationship.getRelationshipType()).contains(to.getResource());
                        neighbours = cluster.getNeighbours(to.getResource(),relationship.getRelationshipType()).contains(from.getResource()) || neighbours;
                        if(!relationship.isShared() && neighbours) {
                            log.info("Removing resource {}", from);
                            log.info("TO {}", to);
                            toRemove.add(from);
                        }else if(relationship.isShared() && !neighbours) {
                            log.info("Removing resource {}", from);
                            log.info("TO {}", to);
                            toRemove.add(from);
                        }
                    }
                });
            }
        }

        resources.removeAll(toRemove);
        log.info("Remaining resources {}", resources.size());
        return resources;
    }*/

    public List<K8sPatternResource> filterResourcesWithRelationships(PatternDefinition patternDefinition, List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sPatternResource> toRemove = new ArrayList<>();
        List<K8sPatternResource> toAdd = new ArrayList<>();
        List<RelationshipDefinition> relationships = patternDefinition.getRelationships();
        if (relationships == null || relationships.isEmpty()) {
            toAdd.addAll(resources);
        }else{
            for (RelationshipDefinition relationshipDefinition : relationships) {
                if (!relationshipDefinition.isShared()) {
                    toRemove.addAll(extractSingleRelationshipResource(resources, relationshipDefinition, cluster));
                }
            }
        }


        resources.removeAll(toRemove);

        return resources;
    }

    public static List<K8sPatternResource> extractSingleRelationshipResource(List<K8sPatternResource> resources, RelationshipDefinition commonRelationshipDefinition, K8sCluster cluster) {
        List<K8sPatternResource> resourceList = new ArrayList<>();
        String leaderName = commonRelationshipDefinition.getResources()[0];
        String followerName = commonRelationshipDefinition.getResources()[1];
        for(K8sPatternResource leader : resources) {
            for(K8sPatternResource follower : resources) {
                if(!leader.equals(follower) && leader.getName().equals(leaderName) && follower.getName().equals(followerName)) {
                    if(cluster.sameNeighbour(leader.getResource(), follower.getResource(), commonRelationshipDefinition.getRelationshipType())){
                        resourceList.add(leader);
                    }
                }
            }
        }
        return resourceList;
    }

}

