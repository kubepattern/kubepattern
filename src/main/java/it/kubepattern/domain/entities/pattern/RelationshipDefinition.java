package it.kubepattern.domain.entities.pattern;

import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;

@Getter
@Setter
@ToString
@AllArgsConstructor
public class RelationshipDefinition {
    private String id;
    private RelationshipType relationshipType;
    private boolean required;
    private boolean shared;
    private int weight;
    private String[] resources;

}
