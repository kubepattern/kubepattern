package it.kubepattern.domain.entities.cluster.relationships.strategies;

import io.kubernetes.client.openapi.models.V1PersistentVolumeClaim;
import io.kubernetes.client.openapi.models.V1Pod;
import io.kubernetes.client.openapi.models.V1Volume;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.K8sRelationship;
import it.kubepattern.domain.entities.cluster.K8sResource;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;
import lombok.extern.slf4j.Slf4j;

@Slf4j
public class PodToVolumeStrategy implements RelationshipStrategy {

    @Override
    public void analyze(K8sCluster cluster) {
        for (K8sResource pod : cluster.getResourcesByKind("Pod")) {
            V1Pod v1Pod = (V1Pod) pod.getObject();
            if (v1Pod.getSpec() != null && v1Pod.getSpec().getVolumes() != null) {
                for (V1Volume volume : v1Pod.getSpec().getVolumes()) {
                    if (hasPersistentVolumeClaim(volume)) {
                        assert volume.getPersistentVolumeClaim() != null;
                        String pvcName = volume.getPersistentVolumeClaim().getClaimName();
                        K8sResource pvcResource = findPvcByName(cluster, pvcName);
                        if (pvcResource != null) {
                            cluster.addRelationship(pod, pvcResource, new K8sRelationship(RelationshipType.MOUNTS));
                        }
                    }
                }
            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        if (resource.getKind().equals("Pod")) {
            return hasPodVolumes(resource);
        }
        return resource.getKind().equals("PersistentVolumeClaim");
    }

    private boolean hasPodVolumes(K8sResource resource) {
        if (resource.getObject() instanceof V1Pod v1Pod) {
            return v1Pod.getSpec() != null
                    && v1Pod.getSpec().getVolumes() != null
                    && !v1Pod.getSpec().getVolumes().isEmpty();
        }
        return false;
    }

    private boolean hasPersistentVolumeClaim(V1Volume volume) {
        return volume.getPersistentVolumeClaim() != null;
    }

    private K8sResource findPvcByName(K8sCluster cluster, String pvcName) {
        for (K8sResource resource : cluster.getResourcesByKind("PersistentVolumeClaim")) {
            V1PersistentVolumeClaim pvc = (V1PersistentVolumeClaim) resource.getObject();
            if (pvc.getMetadata() != null
                    && pvc.getMetadata().getName() != null
                    && pvc.getMetadata().getName().equals(pvcName)) {
                return resource;
            }
        }
        return null;
    }
}