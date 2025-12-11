package it.kubepattern.application.configuration;

import lombok.Data;
import lombok.Getter;
import lombok.Setter;
import lombok.Value;
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

        enum PatternRegistryType {
            GITHUB
        }
        private String url;
        private PatternRegistry.PatternRegistryType type = PatternRegistry.PatternRegistryType.GITHUB;
        private String repositoryBranch = "main";
        private String repositoryToken = "";
        private String organizationName = "kubepattern";
        private String repositoryName = "registry";
    }
}
