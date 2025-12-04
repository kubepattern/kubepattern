package it.kubepattern.domain.entities.cluster.relationships.strategies;

import io.kubernetes.client.openapi.models.*;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.K8sRelationship;
import it.kubepattern.domain.entities.cluster.K8sResource;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;

import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

public class UsesConfigStrategy implements RelationshipStrategy {

    @Override
    public void analyze(K8sCluster cluster) {
        for (K8sResource podResource : cluster.getResourcesByKind("Pod")) {
            V1Pod pod = (V1Pod) podResource.getObject();

            if (pod == null || pod.getSpec() == null || pod.getMetadata() == null) {
                continue;
            }

            V1PodSpec podSpec = pod.getSpec();
            String podNamespace = pod.getMetadata().getNamespace();

            for (K8sResource configMapResource : cluster.getResourcesByKind("ConfigMap")) {
                V1ConfigMap configMap = (V1ConfigMap) configMapResource.getObject();

                if (configMap == null || configMap.getMetadata() == null) {
                    continue;
                }

                String configMapName = configMap.getMetadata().getName();
                String cmNamespace = configMap.getMetadata().getNamespace();

                // Namespace of Pod and ConfigMap must be the same
                if (configMapName == null || !Objects.equals(podNamespace, cmNamespace)) {
                    continue;
                }

                boolean podUsesThisConfigMap = false;

                // --- Check 1 configMap as Volume ---
                if (podSpec.getVolumes() != null) {
                    for (V1Volume volume : podSpec.getVolumes()) {
                        V1ConfigMapVolumeSource cmVolume = volume.getConfigMap();

                        if (cmVolume != null && configMapName.equals(cmVolume.getName())) {
                            podUsesThisConfigMap = true;
                            break;
                        }
                    }
                }

                if (podUsesThisConfigMap) {
                    cluster.addRelationship(podResource, configMapResource, new K8sRelationship(RelationshipType.USES_CONFIG));
                    continue; // Next ConfigMap
                }

                // --- Check 2 & 3: La ConfigMap as Env Var ---

                // Check all containers
                List<V1Container> allContainers = new ArrayList<>(podSpec.getContainers());
                if (podSpec.getInitContainers() != null) {
                    allContainers.addAll(podSpec.getInitContainers());
                }

                for (V1Container container : allContainers) {
                    if (podUsesThisConfigMap) {
                        break;
                    }

                    // Check 2a: 'envFrom'
                    if (container.getEnvFrom() != null) {
                        for (V1EnvFromSource envFrom : container.getEnvFrom()) {
                            V1ConfigMapEnvSource cmEnvSource = envFrom.getConfigMapRef();
                            if (cmEnvSource != null && configMapName.equals(cmEnvSource.getName())) {
                                podUsesThisConfigMap = true;
                                break;
                            }
                        }
                    }

                    if (podUsesThisConfigMap) {
                        break;
                    }

                    // Check 2b: 'env'
                    if (container.getEnv() != null) {
                        for (V1EnvVar envVar : container.getEnv()) {
                            V1EnvVarSource valueFrom = envVar.getValueFrom();
                            if (valueFrom != null) {
                                V1ConfigMapKeySelector cmKeyRef = valueFrom.getConfigMapKeyRef();
                                if (cmKeyRef != null && configMapName.equals(cmKeyRef.getName())) {
                                    podUsesThisConfigMap = true;
                                    break;
                                }
                            }
                        }
                    }
                }

                if (podUsesThisConfigMap) {
                    cluster.addRelationship(podResource, configMapResource, new K8sRelationship(RelationshipType.USES_CONFIG));
                }

            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        return false;
    }
}
