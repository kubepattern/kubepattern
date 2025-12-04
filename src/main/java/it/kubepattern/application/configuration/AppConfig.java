package it.sigemi.application.configuration;

import lombok.Data;
import lombok.Getter;
import lombok.Setter;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConfigurationProperties(prefix = "app")
@Getter
@Setter
public class AppConfig {

    private Report report;
    private PatternRegistry patternRegistry;

    AppConfig() {
        report = new Report();
        patternRegistry = new PatternRegistry();
    }

    @Data
    public static class Report {
        private boolean saveInNamespace = false;
        private String targetNamespace = "pattern-analysis-ns";
    }

    @Data
    public static class PatternRegistry {
        private String url;
    }
}
