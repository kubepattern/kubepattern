package it.sigemi.application.service.pattern;

import it.sigemi.domain.entities.pattern.PatternDefinition;
import it.sigemi.exception.MalformedPatternException;
import it.sigemi.repository.pattern_as_code.IPatternDefinitionRepository;
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
