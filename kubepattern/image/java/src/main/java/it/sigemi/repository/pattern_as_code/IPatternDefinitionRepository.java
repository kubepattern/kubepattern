package it.sigemi.repository.pattern_as_code;

import it.sigemi.domain.entities.pattern.PatternDefinition;
import org.springframework.stereotype.Repository;

import java.io.IOException;
import java.util.List;

@Repository
public interface IPatternDefinitionRepository {
    PatternDefinition getPatternDefinitionByName(String name) throws IOException, InterruptedException;
    List<PatternDefinition> getAllPatternDefinitions() throws IOException, InterruptedException;
}
