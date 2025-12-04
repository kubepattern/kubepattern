package it.sigemi.domain.entities.pattern;

import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.K8sPatternResource;
import it.sigemi.domain.entities.cluster.K8sResource;
import it.sigemi.domain.entities.cluster.relationships.K8sCommonRelationship;
import lombok.extern.slf4j.Slf4j;

import java.util.*;
import java.util.stream.Collectors;

@Slf4j
public class PatternEngine {


    public List<K8sPatternResource> retrievePatternResources(PatternDefinition patternDefinition, K8sCluster cluster) {
        List<K8sPatternResource> patternResources = new ArrayList<>();

        for(String id : patternDefinition.getResources().keySet()){
            String kind = patternDefinition.getResources().get(id);
            List<K8sResource> resources = cluster.getResourcesByKind(kind);
            for (K8sResource resource : resources){
                patternResources.add(new K8sPatternResource(id, resource));
            }
        }

        return patternResources;
    }

    public List<K8sPattern> generatePatterns(PatternDefinition definition, List<K8sPatternResource> resources, K8sCluster cluster) {
        if (definition.getPatternTopology() == null) {
            return new ArrayList<>();
        }

        switch (definition.getPatternTopology()) {
            case LEADER_FOLLOWER -> {
                return generateLeaderFollowerPatterns(definition, resources, cluster);
            }
            case SINGLE -> {
                return generateSinglePatterns(definition, resources, cluster);
            }
            default -> {
                return new ArrayList<>();
            }
        }
    }

    //Veto/Scoring.

    private List<K8sCommonRelationship> generateLeaderFollowerCommonRelationships(PatternDefinition definition, List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sCommonRelationship> result = new ArrayList<>();
        List<CommonRelationshipDefinition> commonDefs = definition.getCommonRelationshipDefinitions();

        // Soglia minima definita nel pattern (default a 0 se null)
        int minPoints = definition.getMinCommonRelPoints();

        // 1. Ottimizzazione: Raggruppa le risorse per ID per evitare cicli O(N^2) su risorse non pertinenti
        Map<String, List<K8sPatternResource>> resourcesById = resources.stream()
                .collect(Collectors.groupingBy(K8sPatternResource::getName));

        // Identifica chi è il Leader e chi è il Follower dalla definizione
        // Assumiamo che actors[0] sia leader e actors[1] sia follower, o usiamo definition.getLeaderId()
        String leaderId = definition.getLeaderId();

        // Trova l'ID del follower (assumendo che ci siano solo 2 attori principali in Leader-Follower)
        String followerId = null;

        for(String actor: definition.getActors()){
            if(!actor.equals(leaderId)) {
                followerId = actor;
            }
        }

        if (followerId == null || !resourcesById.containsKey(leaderId) || !resourcesById.containsKey(followerId)) {
            log.warn("Missing actors for Leader-Follower pattern: Leader={}, Follower={}", leaderId, followerId);
            return result;
        }

        List<K8sPatternResource> leaders = resourcesById.get(leaderId);
        List<K8sPatternResource> followers = resourcesById.get(followerId);

        // 2. Itera solo sulle coppie pertinenti (Leader x Follower)
        for (K8sPatternResource leader : leaders) {
            for (K8sPatternResource follower : followers) {

                // Evita self-comparison se leaderId e followerId fossero uguali (edge case)
                if (leader.equals(follower)) continue;

                boolean isVetoed = false;
                double currentPoints = 0; // Uso double se i pesi sono float, altrimenti int

                for (CommonRelationshipDefinition relDef : commonDefs) {
                    // Verifica se esiste un "Vicino Comune" (es. stesso Volume, stesso Nodo)
                    boolean hasCommonNeighbour = cluster.sameNeighbour(
                            leader.getResource(),
                            follower.getResource(),
                            relDef.getRelationshipType()
                    );

                    // --- LOGICA VETO vs SCORING ---
                    if (relDef.isRequired()) {
                        // CASO VETO (Mandatory o Forbidden)
                        if (relDef.isShared()) {
                            // REQUIRED: TRUE, SHARED: TRUE -> Devono avere il comune
                            if (!hasCommonNeighbour) {
                                isVetoed = true;
                                log.debug("Vetoed: Missing required shared common relation {} between {} and {}",
                                        relDef.getRelationshipType(), leader.getName(), follower.getName());
                                break;
                            }
                        } else {
                            // REQUIRED: TRUE, SHARED: FALSE -> NON devono avere il comune
                            if (hasCommonNeighbour) {
                                isVetoed = true;
                                log.debug("Vetoed: Found forbidden shared common relation {} between {} and {}",
                                        relDef.getRelationshipType(), leader.getName(), follower.getName());
                                break;
                            }
                        }
                    } else {
                        // CASO SCORING (Punteggio)
                        if (relDef.isShared()) {
                            // REQUIRED: FALSE, SHARED: TRUE -> Punti se c'è
                            if (hasCommonNeighbour) {
                                currentPoints += relDef.getWeight();
                            }
                        } else {
                            // REQUIRED: FALSE, SHARED: FALSE -> Punti se NON c'è
                            if (!hasCommonNeighbour) {
                                currentPoints += relDef.getWeight();
                            }
                        }
                    }
                }

                // 3. Valutazione Finale della Coppia
                if (!isVetoed && currentPoints >= minPoints) {
                    // Crea un set per la relazione (evita duplicati interni alla struttura dati)
                    Set<K8sPatternResource> pair = new HashSet<>();
                    pair.add(leader);
                    pair.add(follower);

                    // Aggiungi al risultato.
                    // Nota: Poiché iteriamo liste disgiunte (Leaders x Followers), non c'è rischio di duplicati (A,B) vs (B,A)
                    // a meno che leaderId == followerId (Pod affinity), nel qual caso servirebbe un controllo extra.
                    result.add(new K8sCommonRelationship(pair, (int) currentPoints));
                }
            }
        }

        return result;
    }

    private List<K8sPattern> generateLeaderFollowerPatterns(PatternDefinition definition, List<K8sPatternResource> resources, K8sCluster cluster) {
        // 1. Genera le relazioni comuni valide
        List<K8sCommonRelationship> validRelationships = generateLeaderFollowerCommonRelationships(definition, resources, cluster);
        List<K8sPattern> patterns = new ArrayList<>();

        // 2. Trasforma ogni relazione valida in un Pattern Match
        for (K8sCommonRelationship rel : validRelationships) {
            List<K8sPatternScore> scores = new ArrayList<>();
            scores.add(new K8sPatternScore("CommonRelationships", "Relationship Score", rel.getPoints()));

            K8sPattern pattern = K8sPattern.builder()
                    .metadata(definition.getMetadata())
                    .message(definition.getMessage()) // Qui potresti voler interpolare i nomi delle risorse nel messaggio
                    .resources(rel.getRelationships().toArray(new K8sPatternResource[0]))
                    .scores(scores)
                    .build();
            patterns.add(pattern);
        }

        return patterns;
    }

    private List<K8sPattern> generateSinglePatterns(PatternDefinition definition, List<K8sPatternResource> resources, K8sCluster cluster) {
        List<K8sPattern> patterns = new ArrayList<>();
        // Nota: Si presume che 'resources' contenga solo risorse già filtrate che matchano i criteri "Single".
        // I Single Pattern non usano commonRelationships.

        for (K8sPatternResource patternRes : resources) {
            if (patternRes.getName().equals(definition.getLeaderId())) {
                K8sPattern pattern = K8sPattern.builder()
                        .metadata(definition.getMetadata())
                        .message(definition.getMessage())
                        .resources(new K8sPatternResource[]{patternRes})
                        .scores(new ArrayList<>()) // Nessuno score relazionale per Single
                        .build();
                patterns.add(pattern);
            }
        }
        return patterns;
    }
}