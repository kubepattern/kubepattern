package it.kubepattern.application.configuration;

import it.kubepattern.domain.entities.filter.FilterQueryEngine;
import it.kubepattern.domain.entities.filter.RelationshipFilterEngine;
import it.kubepattern.domain.entities.filter.ResourceFilterEngine;
import it.kubepattern.domain.entities.pattern.PatternEngine;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class PatternAnalysisConfig {

    @Bean
    public RelationshipFilterEngine relationshipFilterEngine() {
        return new RelationshipFilterEngine();
    }

    @Bean
    public PatternEngine patternEngine() {
        return new PatternEngine();
    }

    @Bean
    public ResourceFilterEngine resourceFilterEngine() {
        return new ResourceFilterEngine();
    }

    @Bean
    public FilterQueryEngine filterQueryEngine() {
        return FilterQueryEngine.JSON_PATH;
    }

}
