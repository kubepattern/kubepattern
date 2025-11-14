package it.sigemi.application.service.pattern;

import it.sigemi.domain.entities.pattern.PatternDefinition;
import it.sigemi.repository.pattern_as_code.IPatternDefinitionRepository;
import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;
import java.io.IOException;
import java.util.List;

@Service
@AllArgsConstructor
public class FetchPatternDefinitionService {

    IPatternDefinitionRepository patternDefinitionRepository;

    public PatternDefinition fetchPatternDefinition(String patternName) throws IOException, InterruptedException {
        return patternDefinitionRepository.getPatternDefinitionByName(patternName);
    }

    public List<PatternDefinition> getAllPatternDefinitions() throws IOException, InterruptedException {
        return patternDefinitionRepository.getAllPatternDefinitions();
    }
}
