package it.sigemi.application.configuration;

import it.sigemi.domain.entities.cluster.relationships.RelationshipGenerator;
import it.sigemi.domain.entities.cluster.relationships.strategies.*;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import java.util.List;

@Configuration
public class RelationshipAnalyzerConfig {

    @Bean
    public DeploymentToReplicaSetStrategy deploymentToReplicaSetStrategy() {
        return new DeploymentToReplicaSetStrategy();
    }

    @Bean
    public ReplicaSetToPodStrategy replicaSetToPodStrategy() {
        return new ReplicaSetToPodStrategy();
    }

    @Bean
    public PodToVolumeStrategy podToVolumeStrategy() {
        return new PodToVolumeStrategy();
    }

    @Bean
    public IsNamespaceOfStrategy isNamespaceOfStrategy() {
        return new IsNamespaceOfStrategy();
    }

    @Bean
    public ServiceToPodStrategy serviceToPodStrategy() { return new  ServiceToPodStrategy(); }

    @Bean
    public UsesServiceAccountStrategy usesServiceAccountStrategy() { return new UsesServiceAccountStrategy(); }

    @Bean
    public UsesConfigStrategy  usesConfigStrategy() { return new UsesConfigStrategy(); }

    // Relationship Generator
    @Bean
    public RelationshipGenerator relationshipEngine(List<RelationshipStrategy> strategies) {return new RelationshipGenerator(strategies);}
}