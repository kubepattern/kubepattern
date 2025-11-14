package it.sigemi.domain.entities.cluster;

import it.sigemi.domain.entities.cluster.relationships.RelationshipType;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;
import org.jgrapht.graph.DefaultEdge;

import java.util.Map;
import java.util.Objects;

@Setter
@Getter
@ToString
@AllArgsConstructor
public class K8sRelationship extends DefaultEdge {
    private final RelationshipType type;
    private Map<String, Object> metadata;
    private String fromId;
    private String toId;

    public K8sRelationship(RelationshipType type) {
        this.type = type;
    }

    public K8sRelationship(RelationshipType type, String fromId, String toId) {
        this.type = type;
        this.fromId = fromId;
        this.toId = toId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        K8sRelationship that = (K8sRelationship) o;
        return Objects.equals(fromId, that.fromId) && Objects.equals(toId, that.toId) &&  Objects.equals(type, that.type);
    }

    @Override
    public int hashCode() {
        return type != null ? type.hashCode() : 0;
    }
}