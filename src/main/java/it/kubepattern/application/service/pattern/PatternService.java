package it.kubepattern.application.service.pattern;

import io.kubernetes.client.openapi.ApiException;
import it.kubepattern.domain.entities.pattern.K8sPattern;
import it.kubepattern.repository.kubernetes.pattern.IK8sPatternRepository;
import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@AllArgsConstructor
public class PatternService {
    private IK8sPatternRepository patternRepository;
    public void savePattern(K8sPattern pattern) throws ApiException {
        patternRepository.savePattern(pattern);
    }
}
