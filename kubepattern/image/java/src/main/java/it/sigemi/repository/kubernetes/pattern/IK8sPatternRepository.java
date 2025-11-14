package it.sigemi.repository.kubernetes.pattern;

import io.kubernetes.client.openapi.ApiException;
import it.sigemi.domain.entities.pattern.K8sPattern;
import org.springframework.stereotype.Repository;

@Repository
public interface IK8sPatternRepository {
    void savePattern (K8sPattern k8sPattern) throws ApiException;
}
