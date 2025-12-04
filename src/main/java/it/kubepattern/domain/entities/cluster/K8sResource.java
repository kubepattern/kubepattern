package it.sigemi.domain.entities.cluster;

import io.kubernetes.client.common.KubernetesObject;
import io.kubernetes.client.openapi.JSON;
import lombok.*;
import java.util.Collections;
import java.util.Map;
import java.util.Objects;


@Getter
@Setter
@Builder
@AllArgsConstructor
public class K8sResource {
    private String uid;
    private String apiVersion;
    private String name;
    private String kind;
    private String namespace;
    private Map<String, String> labels;
    private Map<String, String> annotations;
    private KubernetesObject object;

    public K8sResource(KubernetesObject object) {
        this.object = object;
        this.apiVersion = object.getApiVersion();
        this.name = object.getMetadata().getName();
        this.kind = object.getKind();
        this.uid = object.getMetadata().getUid();
        this.namespace = object.getMetadata().getNamespace();
        this.labels = object.getMetadata().getLabels() != null ? object.getMetadata().getLabels() : Collections.emptyMap();
        this.annotations = object.getMetadata().getAnnotations() != null ? object.getMetadata().getAnnotations() : Collections.emptyMap();
    }

    public K8sResource(String apiVersion, String kind, KubernetesObject object) {
        this.object = object;
        this.apiVersion = apiVersion;
        this.kind = kind;
        this.name = object.getMetadata().getName();
        this.namespace = object.getMetadata().getNamespace();
        this.uid = object.getMetadata().getUid();
        this.labels = object.getMetadata().getLabels() != null ? object.getMetadata().getLabels() : Collections.emptyMap();
        this.annotations = object.getMetadata().getAnnotations() != null ? object.getMetadata().getAnnotations() : Collections.emptyMap();
    }

    public boolean sameKind(K8sResource other) {
        return this.kind.equals(other.kind);
    }

    public boolean sameNamespace(K8sResource other) {
        if(this.namespace == null || this.namespace.isEmpty()) {
            return other.namespace == null || other.namespace.isEmpty();
        }
        return this.namespace.equals(other.namespace);
    }

    public boolean sameApiVersion(K8sResource other) {
        return this.apiVersion.equals(other.apiVersion);
    }

    public boolean sameName(K8sResource other) {
        return this.name.equals(other.name);
    }

    public boolean hasSameLabelValue (K8sResource other, String labelKey) {
        if(this.labels.containsKey(labelKey) && other.labels.containsKey(labelKey)) {
            return this.labels.get(labelKey).equals(other.labels.get(labelKey));
        }
        return false;
    }

    public boolean hasSameAnnotationValue (K8sResource other, String annotationKey) {
        if(this.annotations.containsKey(annotationKey) && other.annotations.containsKey(annotationKey)) {
            return this.annotations.get(annotationKey).equals(other.annotations.get(annotationKey));
        }
        return false;
    }

    public boolean isClusterScoped() {
        return namespace == null || namespace.isEmpty();
    }

    public String getJsonObject() {
        return JSON.serialize(object);
    }

    @Override
    public String toString() {
        return namespace + "." + apiVersion + "." + kind + "." + name;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) {
            return true;
        }
        if (obj == null || getClass() != obj.getClass()) {
            return false;
        }
        K8sResource that = (K8sResource) obj;


        boolean thisUidPresent = (this.uid != null && !this.uid.isEmpty());
        boolean thatUidPresent = (that.uid != null && !that.uid.isEmpty());

        if (thisUidPresent && thatUidPresent) {
            return this.uid.equals(that.uid);
        }


        if (thisUidPresent != thatUidPresent) {
            return false;
        }


        if (isClusterScoped()) {

            return Objects.equals(this.name, that.name) &&
                    Objects.equals(this.kind, that.kind) &&
                    Objects.equals(this.apiVersion, that.apiVersion);
        } else {

            return Objects.equals(this.name, that.name) &&
                    Objects.equals(this.kind, that.kind) &&
                    Objects.equals(this.namespace, that.namespace) &&
                    Objects.equals(this.apiVersion, that.apiVersion);
        }
    }

    @Override
    public int hashCode() {


        if (uid != null && !uid.isEmpty()) {
            return Objects.hash(uid);
        }


        if (isClusterScoped()) {
            return Objects.hash(name, kind, apiVersion);
        } else {
            return Objects.hash(name, kind, namespace, apiVersion);
        }
    }
}
