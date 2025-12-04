package it.kubepattern.domain.entities.cluster;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.ToString;

import java.util.Objects;

@AllArgsConstructor
@ToString
@Getter
public class K8sPatternResource {
    private String name;
    private K8sResource resource;

    public String getNamespace() {
        return resource != null ? resource.getNamespace() : null;
    }

    public String getUid() {
        return resource != null ? resource.getUid() : null;
    }

    public String getKind() {
        return resource != null ? resource.getKind() : null;
    }

    public String getApiVersion() {
        return resource != null ? resource.getApiVersion() : null;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        K8sPatternResource that = (K8sPatternResource) o;

        return Objects.equals(name, that.name) &&
                Objects.equals(resource, that.resource);
    }

    @Override
    public int hashCode() {
        return Objects.hash(name, resource);
    }
}
