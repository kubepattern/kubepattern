package it.sigemi.domain.entities.filter;

import it.sigemi.domain.entities.cluster.K8sResource;
import lombok.extern.slf4j.Slf4j;

import java.util.ArrayList;
import java.util.List;

@Slf4j
public class ResourceFilterEngine {
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
