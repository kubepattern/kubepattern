package it.kubepattern.application.configuration;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class PatternDefinitionConfig {
    @Bean
    public String gitBaseUrl() {
        return "https://raw.githubusercontent.com/kubepattern/registry/main/definitions";
    }
    @Bean
    public String gitToken() {
        return "";
    }

    @Bean
    public String gitBranch() {
        return "main";
    }
}
