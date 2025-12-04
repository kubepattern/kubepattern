package it.kubepattern.application.service.pattern;

import it.kubepattern.domain.entities.pattern.PatternDefinition;
import it.kubepattern.exception.MalformedPatternException;
import it.kubepattern.repository.pattern_as_code.IPatternDefinitionRepository;
import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;
import java.util.List;

@Service
@AllArgsConstructor
public class FetchPatternDefinitionService {

    IPatternDefinitionRepository patternDefinitionRepository;

    public PatternDefinition fetchPatternDefinition(String patternName) throws MalformedPatternException, Exception {
        return patternDefinitionRepository.getPatternDefinitionByName(patternName);
    }

    public List<PatternDefinition> getAllPatternDefinitions() throws MalformedPatternException, Exception {
        return patternDefinitionRepository.getAllPatternDefinitions();
    }
}
