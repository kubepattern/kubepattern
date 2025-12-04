package it.sigemi.domain.entities.pattern;

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
    private boolean shared;
}
