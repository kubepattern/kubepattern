package it.sigemi.domain.entities.pattern;

import it.sigemi.domain.entities.cluster.RelationshipDefinition;
import it.sigemi.domain.entities.cluster.commonRelationships.CommonRelationshipDefinition;
import it.sigemi.domain.entities.filter.ResourceFilter;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;

import java.util.List;
import java.util.Map;
import java.util.Set;

@Getter
@Setter
@ToString
public class PatternDefinition {
    private K8sPatternMetadata metadata;
    private String leaderId;
    private PatternTopology patternTopology;
    private String message;
    private String[] actors;
    private Map<String, String> resources;// <ResourceId, Kind>
    private Map<String , ResourceFilter> resourceFilters;// ResourceId, ResourceFilter
    private List<RelationshipDefinition> relationships;
    private List<CommonRelationshipDefinition> commonRelationshipDefinitions;


    public String getKindById(String id){
        return resources.get(id);
    }

    public ResourceFilter getResourceFilterById(String id){
        return resourceFilters.get(id);
    }

    public Set<String> getResourceIds(){
        return resources.keySet();
    }
}
