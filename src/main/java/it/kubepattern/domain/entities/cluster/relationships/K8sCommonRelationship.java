package it.kubepattern.domain.entities.cluster.relationships;

import it.kubepattern.domain.entities.cluster.K8sPatternResource;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;

import java.util.Set;
@Getter
@Setter
@EqualsAndHashCode
@ToString
public class K8sCommonRelationship {
    private Set<K8sPatternResource> relationships;
    private int points;
    public K8sCommonRelationship(Set<K8sPatternResource> relationships,  int points) {
        this.relationships = relationships;
        this.points = points;
    }
}
