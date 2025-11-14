package it.sigemi.domain.entities.cluster.relationships;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;
import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.relationships.strategies.RelationshipStrategy;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import java.util.List;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class RelationshipDefinitionGeneratorTest {

    @Mock
    private RelationshipStrategy strategy1;

    @Mock
    private RelationshipStrategy strategy2;

    private RelationshipGenerator relationshipGenerator;
    private K8sCluster cluster;

    @BeforeEach
    void setUp() {
        relationshipGenerator = new RelationshipGenerator(List.of(strategy1, strategy2));
        cluster = new K8sCluster();
    }

    @Test
    void shouldExecuteAllStrategies() {
        // When
        relationshipGenerator.discoverRelationships(cluster);

        // Then
        verify(strategy1, times(1)).analyze(cluster);
        verify(strategy2, times(1)).analyze(cluster);
    }

    @Test
    void shouldContinueOnStrategyException() {
        // Given
        doThrow(new RuntimeException("Strategy error")).when(strategy1).analyze(cluster);

        // When
        relationshipGenerator.discoverRelationships(cluster);

        // Then
        verify(strategy1, times(1)).analyze(cluster);
        verify(strategy2, times(1)).analyze(cluster);
    }

    @Test
    void shouldReturnAllRegisteredStrategies() {
        // When
        List<RelationshipStrategy> strategies = relationshipGenerator.getStrategies();

        // Then
        assertEquals(2, strategies.size());
        assertTrue(strategies.contains(strategy1));
        assertTrue(strategies.contains(strategy2));

        System.out.println(strategies.getFirst().toString());
        System.out.println(strategies.get(1).toString());
        System.out.println(cluster);
    }
}
