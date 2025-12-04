package it.kubepattern.repository.kubernetes.resource;

import io.kubernetes.client.openapi.ApiException;
import it.kubepattern.domain.entities.cluster.K8sResource;
import org.springframework.stereotype.Repository;

import java.io.IOException;
import java.util.List;

@Repository
public interface IK8sResourceRepository {
    List<K8sResource> getAllNamespaceResources(String namespace) throws IOException, ApiException;
}
