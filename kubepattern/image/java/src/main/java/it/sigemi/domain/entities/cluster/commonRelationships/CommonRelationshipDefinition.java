package it.sigemi.domain.entities.cluster.commonRelationships;

import it.sigemi.domain.entities.cluster.relationships.RelationshipType;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;


@Getter
@Setter
@ToString
@AllArgsConstructor
public class CommonRelationshipDefinition {
    private String id;
    private RelationshipType relationshipType;
    private String[] resources;
    private boolean required;
    private int weight;
    private boolean shared;// if true the relationship may be shared, false if must not be shared

    public boolean hasResource(String name) {
        for (String resource : resources) {
            if (resource.equals(name)) {
                return true;
            }
        }
        return false;
    }
}
