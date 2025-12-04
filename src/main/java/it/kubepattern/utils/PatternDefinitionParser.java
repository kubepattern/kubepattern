package it.kubepattern.utils;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import it.kubepattern.domain.entities.pattern.RelationshipDefinition;
import it.kubepattern.domain.entities.pattern.PatternTopology;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;
import it.kubepattern.domain.entities.pattern.CommonRelationshipDefinition;
import it.kubepattern.domain.entities.filter.FilterOperator;
import it.kubepattern.domain.entities.filter.FilterRule;
import it.kubepattern.domain.entities.filter.ResourceFilter;
import it.kubepattern.domain.entities.pattern.K8sPatternMetadata;
import it.kubepattern.domain.entities.pattern.PatternDefinition;
import it.kubepattern.domain.entities.pattern.PatternSeverity;
import it.kubepattern.domain.entities.pattern.PatternType;
import it.kubepattern.exception.MalformedPatternException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.util.*;

public class PatternDefinitionParser{
    private static final Logger logger = LoggerFactory.getLogger(PatternDefinitionParser.class);
    private static final ObjectMapper objectMapper = new ObjectMapper();

    public static PatternDefinition fromJson(String json) throws IOException, MalformedPatternException {
        PatternDefinition pattern = new PatternDefinition();
        K8sPatternMetadata metadata = new K8sPatternMetadata();
        pattern.setMetadata(metadata);
        // Root
        JsonNode rootNode = objectMapper.readTree(json);
        parseRoot(pattern, rootNode);

        // Metadata
        JsonNode metadataNode = rootNode.path("metadata");
        parseMetadata(pattern, metadataNode);

        // Spec
        JsonNode specNode = rootNode.path("spec");
        parseSpec(pattern, specNode);

        // Relationships
        JsonNode commonRelationshipsNode =  specNode.path("commonRelationships");
        parseCommonRelationships(pattern, commonRelationshipsNode);
        JsonNode relationshipsNode =  specNode.path("relationships");
        parseRelationships(pattern, relationshipsNode);

        if(specNode.has("minRelationshipPoints")){
            int minRelPoint = specNode.path("minRelationshipPoints").asInt();
            pattern.setMinRelPoints(minRelPoint);
        }else{
            pattern.setMinRelPoints(0);
        }

        if(specNode.has("minCommonRelationshipPoints")){
            int minRelPoint = specNode.path("minRelationshipPoints").asInt();
            pattern.setMinRelPoints(minRelPoint);
        }else{
            pattern.setMinRelPoints(0);
        }

        logger.info("pattern definition parsed successfully");

        return pattern;
    }

    private static void parseRoot(PatternDefinition pattern,  JsonNode rootNode) {
        pattern.getMetadata().setVersion(rootNode.path("version").asText());
        logger.info(rootNode.toString());
        try{
            logger.info("{}", PatternSeverity.valueOf(rootNode.path("severity").asText()));
            pattern.getMetadata().setSeverity(PatternSeverity.valueOf(rootNode.path("severity").asText()));
        }catch(Exception e){
            pattern.getMetadata().setSeverity(PatternSeverity.INFO);
        }
    }

    private static void parseMetadata(PatternDefinition pattern,  JsonNode metadataNode) {
        pattern.getMetadata().setName(metadataNode.path("name").asText());
        String patternTypeStr = metadataNode.path("patternType").asText();
        pattern.getMetadata().setType(PatternType.valueOf(patternTypeStr.toUpperCase()));
        pattern.getMetadata().setDocUrl(metadataNode.path("docUrl").asText());
        pattern.getMetadata().setGitUrl(metadataNode.path("gitUrl").asText());
        pattern.getMetadata().setDescription(metadataNode.path("description").asText());
    }

    private static void parseSpec(PatternDefinition pattern, JsonNode specNode) {
        pattern.setMessage(specNode.path("message").asText());
        pattern.setPatternTopology(PatternTopology.valueOf(specNode.path("topology").asText().toUpperCase()));
        JsonNode actorsNode = specNode.path("actors");
        List<String> actorsList = new ArrayList<>();

        if (actorsNode.isArray()) {
            for (JsonNode actorNode : actorsNode) {
                actorsList.add(actorNode.asText());
            }
        }

        pattern.setActors(actorsList.toArray(new String[0]));

        // Parse Resources
        JsonNode resourcesNode = specNode.path("resources");
        parseResources(pattern, resourcesNode);
    }

    private static void parseResources(PatternDefinition pattern, JsonNode resourcesNode) {
        Map<String, String> resources =  new HashMap<>();
        Map<String, ResourceFilter> resourceFilters = new HashMap<>();

        if (resourcesNode.isArray()) {
            for (JsonNode resourceNode : resourcesNode) {
                String id = resourceNode.path("id").asText();
                String kind = resourceNode.path("resource").asText();
                resources.put(id, kind);
                if(resourceNode.has("leader")) {
                    if(resourceNode.get("leader").asBoolean()) {
                        pattern.setLeaderId(id);
                    }
                }
                resourceFilters.put(id, parseResourceFilter(resourceNode.path("filters")));
            }
        }

        pattern.setResources(resources);
        pattern.setResourceFilters(resourceFilters);
    }

    private static ResourceFilter parseResourceFilter (JsonNode filtersNode) {
        List<FilterRule> matchAllRules = new ArrayList<>();
        List<FilterRule> matchNoneRules = new ArrayList<>();
        List<FilterRule> matchAnyRules = new ArrayList<>();

        if (!filtersNode.isMissingNode()) {
            if (filtersNode.has("matchAll")) {
                matchAllRules = matchFilterRulesNodeParsing(filtersNode.path("matchAll"));
            }
            if (filtersNode.has("matchNone")) {
                matchNoneRules = matchFilterRulesNodeParsing(filtersNode.path("matchNone"));
            }
            if (filtersNode.has("matchAny")) {
                matchAnyRules = matchFilterRulesNodeParsing(filtersNode.path("matchAny"));
            }
        }
        return new ResourceFilter(matchAllRules, matchAnyRules, matchNoneRules);
    }

    private static List<FilterRule> matchFilterRulesNodeParsing(JsonNode matchNode) {
        List<FilterRule> ruleList = new ArrayList<>();
        for (JsonNode filterNode : matchNode) {
            String key = filterNode.path("key").asText();
            String operator = filterNode.path("operator").asText();
            FilterOperator filterOperator = FilterOperator.valueOf(operator.toUpperCase());
            JsonNode valuesNode = filterNode.path("values");
            String[] values = objectMapper.convertValue(valuesNode, String[].class);
            FilterRule filter = new FilterRule(key, filterOperator, values);
            ruleList.add(filter);
        }
        return ruleList;
    }

    private static void parseCommonRelationships(PatternDefinition pattern, JsonNode node) {
        List<CommonRelationshipDefinition> relationships = new ArrayList<>();
        if (node.isArray()) {
            for (JsonNode relNode : node) {
                String id = relNode.path("id").asText();
                String typeStr = relNode.path("type").asText();
                String[] resourceIds = objectMapper.convertValue(relNode.path("resourceIds"), String[].class);
                boolean required = relNode.path("required").asBoolean();
                int weight = relNode.path("weight").asInt();
                boolean shared = relNode.path("shared").asBoolean();
                CommonRelationshipDefinition relationship = new CommonRelationshipDefinition(id, RelationshipType.valueOf(typeStr.toUpperCase()), resourceIds, required, weight, shared);
                relationships.add(relationship);
            }
        }
        pattern.setCommonRelationshipDefinitions(relationships);
    }

    private static void parseRelationships(PatternDefinition pattern, JsonNode node) {
        List<RelationshipDefinition> relationships = new ArrayList<>();
        if (node.isArray()) {
            for (JsonNode relNode : node) {
                String id = relNode.path("id").asText();
                String typeStr = relNode.path("type").asText();
                String[] resourceIds = objectMapper.convertValue(relNode.path("resourceIds"), String[].class);
                boolean required = relNode.path("required").asBoolean();
                int weight = relNode.path("weight").asInt();
                boolean shared = relNode.path("shared").asBoolean();
                RelationshipDefinition relationship = new RelationshipDefinition(id, RelationshipType.valueOf(typeStr.toUpperCase()), required, shared, weight, resourceIds);
                relationships.add(relationship);
            }
        }
        pattern.setRelationships(relationships);
    }
}

