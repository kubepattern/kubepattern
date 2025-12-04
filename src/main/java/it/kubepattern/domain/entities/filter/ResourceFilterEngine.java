package it.kubepattern.domain.entities.filter;

import it.kubepattern.domain.entities.cluster.K8sPatternResource;
import it.kubepattern.domain.entities.cluster.K8sResource;
import it.kubepattern.domain.entities.pattern.PatternDefinition;
import lombok.extern.slf4j.Slf4j;

import java.util.ArrayList;
import java.util.List;

@Slf4j
public class ResourceFilterEngine {

    public List<K8sPatternResource> filterResources(PatternDefinition patternDefinition, List<K8sPatternResource> resources) {
        List<K8sPatternResource> filteredResources = new ArrayList<>();
        for (K8sPatternResource resource : resources){
            ResourceFilter resourceFilter = patternDefinition.getResourceFilterById(resource.getName());
            if(applyResourceFilter(resource.getResource(), resourceFilter)){
                filteredResources.add(resource);
            }
        }
        return filteredResources;
    }

    public static List<K8sResource> getFilteredResources(List<K8sResource> resources, ResourceFilter filter) {
        List<K8sResource> filteredResources = new ArrayList<>();
        for (K8sResource resource : resources) {
            if(applyResourceFilter(resource, filter)){
                log.debug("Applying resource {} to filter {}", resource, filter);
                filteredResources.add(resource);
            }
        }

        return filteredResources;
    }

    public static boolean applyResourceFilter(K8sResource resource, ResourceFilter filter) {
        if (filter != null) {
            return filter.applyFilter(resource.getJsonObject());
        }
        return true;
    }
}
