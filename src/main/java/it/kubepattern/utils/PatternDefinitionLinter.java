package it.kubepattern.utils;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import it.kubepattern.domain.entities.cluster.relationships.RelationshipType;
import it.kubepattern.domain.entities.filter.FilterOperator;
import it.kubepattern.domain.entities.pattern.PatternSeverity;
import it.kubepattern.domain.entities.pattern.PatternType;
import it.kubepattern.domain.entities.pattern.PatternTopology;
import it.kubepattern.exception.MalformedPatternException;

import java.util.HashSet;
import java.util.Set;
import java.util.Arrays;
import java.util.List;

public class PatternDefinitionLinter {
    private static final ObjectMapper objectMapper = new ObjectMapper();

    public static void lint(String json) throws MalformedPatternException {
        try {
            JsonNode rootNode = objectMapper.readTree(json);

            // Root validations
            lintVersion(rootNode.path("version").asText());
            lintKind(rootNode.path("kind").asText());

            // Metadata
            JsonNode metadataNode = rootNode.path("metadata");
            lintMetadata(metadataNode);

            // Spec
            JsonNode specNode = rootNode.path("spec");
            lintSpec(specNode);

        } catch (JsonProcessingException jsonProcessingException) {
            throw new MalformedPatternException("pattern definition is not a valid json");
        }
    }

    private static void lintVersion(String version) throws MalformedPatternException {
        if (version == null || version.isEmpty() || version.equals("null")) {
            throw new MalformedPatternException("version is null or empty");
        }

        String expectedPatternRegex = "[a-zA-Z0-9.-]+/v[a-zA-Z0-9]+";

        if (!version.matches(expectedPatternRegex)) {
            throw new MalformedPatternException(
                    "'" + version + "' it is not a valid version. " +
                            "Expected format: '<domain>/v<version>' (es. kubepattern.it/v1)."
            );
        }
    }

    private static void lintKind(String kind) throws MalformedPatternException {
        if (kind == null || kind.isEmpty() || kind.equals("null")) {
            throw new MalformedPatternException("kind is null or empty");
        }
        if (!kind.equals("Pattern")) {
            throw new MalformedPatternException("kind must be 'Pattern', found: " + kind);
        }
    }

    private static void lintMetadata(JsonNode metadata) throws MalformedPatternException {
        if (metadata == null || metadata.isEmpty()) {
            throw new MalformedPatternException("metadata is null or empty");
        }

        lintMetadataName(metadata.get("name").asText());
        lintMetadataDisplayName(metadata.get("displayName").asText());
        lintMetadataPatternType(metadata.get("patternType").asText());
        lintMetadataSeverity(metadata.get("severity").asText());
        lintMetadataCategory(metadata.get("category").asText());
        lintMetadataGitUrl(metadata.path("gitUrl").asText());
        lintMetadataDocUrl(metadata.path("docUrl").asText());
        lintMetadataDescription(metadata.path("description").asText());
    }

    private static void lintMetadataName(String metadataName) throws MalformedPatternException {
        if (metadataName == null || metadataName.isEmpty() || metadataName.equals("null")) {
            throw new MalformedPatternException("metadata.name is null or empty");
        }
        if (!metadataName.matches("[a-zA-Z0-9.-]+")) {
            throw new MalformedPatternException(
                    "metadata.name contains invalid characters. Correct format: [a-zA-Z0-9.-]+"
            );
        }
    }

    private static void lintMetadataDisplayName(String metadataDisplayName) throws MalformedPatternException {
        if (metadataDisplayName == null || metadataDisplayName.isEmpty() || metadataDisplayName.equals("null")) {
            throw new MalformedPatternException("metadata.displayName is null or empty");
        }
    }

    private static void lintMetadataPatternType(String patternType) throws MalformedPatternException {
        if (patternType == null || patternType.isEmpty() || patternType.equals("null")) {
            throw new MalformedPatternException("metadata.patternType is null or empty");
        }
        try {
            PatternType.valueOf(patternType);
        } catch (IllegalArgumentException e) {
            List<String> validValues = Arrays.stream(PatternType.values())
                    .map(Enum::name)
                    .toList();
            throw new MalformedPatternException(
                    "metadata.patternType must be one of " + validValues + ", found: " + patternType
            );
        }
    }

    private static void lintMetadataSeverity(String severity) throws MalformedPatternException {
        if (severity == null || severity.isEmpty() || severity.equals("null")) {
            throw new MalformedPatternException("metadata.severity is null or empty");
        }

        try {
            PatternSeverity.valueOf(severity);
        } catch (IllegalArgumentException e) {
            List<String> validValues = Arrays.stream(PatternSeverity.values())
                    .map(Enum::name)
                    .toList();
            throw new MalformedPatternException(
                    "metadata.severity must be one of " + validValues + ", found: " + severity
            );
        }
    }

    private static void lintMetadataCategory(String category) throws MalformedPatternException {
        if (category == null || category.isEmpty() || category.equals("null")) {
            throw new MalformedPatternException("metadata.category is null or empty");
        }
        if (!category.matches("[a-zA-Z0-9-]+")) {
            throw new MalformedPatternException(
                    "metadata.category contains invalid characters. Correct format: [a-z0-9-]+"
            );
        }
    }

    private static void lintMetadataGitUrl(String gitUrl) throws MalformedPatternException {
        if (gitUrl != null && !gitUrl.isEmpty() && !gitUrl.equals("null")) {
            if (!gitUrl.matches("https?://.*")) {
                throw new MalformedPatternException(
                        "metadata.gitUrl must be a valid URL starting with https://"
                );
            }
        }
    }

    private static void lintMetadataDocUrl(String docUrl) throws MalformedPatternException {
        if (docUrl != null && !docUrl.isEmpty() && !docUrl.equals("null")) {
            if (!docUrl.matches("https?://.*")) {
                throw new MalformedPatternException(
                        "metadata.docUrl must be a valid URL starting with https://"
                );
            }
        }
    }

    private static void lintMetadataDescription(String description) throws MalformedPatternException {
        if (description != null && !description.isEmpty() && !description.equals("null")) {
            if (description.length() < 10) {
                throw new MalformedPatternException(
                        "metadata.description must be at least 10 characters long"
                );
            }
        }
    }

    private static void lintSpec(JsonNode spec) throws MalformedPatternException {
        if (spec == null || spec.isEmpty()) {
            throw new MalformedPatternException("spec is null or empty");
        }

        lintSpecMessage(spec.path("message").asText());
        PatternTopology patternTopology = lintSpecTopology(spec.path("topology").asText());
        JsonNode resourcesNode = spec.path("resources");
        lintResources(resourcesNode, patternTopology);

        JsonNode actorsNode = spec.path("actors");
        lintActors(actorsNode, resourcesNode);

        JsonNode commonRelationshipsNode = spec.path("commonRelationships");
        JsonNode relationshipsNode = spec.path("relationships");
        lintRelationships(commonRelationshipsNode, relationshipsNode, resourcesNode);
    }

    private static void lintSpecMessage(String message) throws MalformedPatternException {
        if (message == null || message.isEmpty() || message.equals("null")) {
            throw new MalformedPatternException("spec.message is null or empty");
        }
    }

    private static PatternTopology lintSpecTopology(String topology) throws MalformedPatternException {
        if (topology == null || topology.isEmpty() || topology.equals("null")) {
            throw new MalformedPatternException("spec.topology is null or empty");
        }

        try {
            return PatternTopology.valueOf(topology);
        } catch (IllegalArgumentException e) {
            List<String> validValues = Arrays.stream(PatternTopology.values())
                    .map(Enum::name)
                    .toList();
            throw new MalformedPatternException(
                    "spec.topology must be one of " + validValues + ", found: " + topology
            );
        }
    }

    private static void lintResources(JsonNode resources, PatternTopology patternTopology) throws MalformedPatternException {
        if (resources == null || !resources.isArray() || resources.isEmpty()) {
            throw new MalformedPatternException("spec.resources must be a non-empty array");
        }

        Set<String> resourceIds = new HashSet<>();

        for (int i = 0; i < resources.size(); i++) {
            JsonNode resource = resources.get(i);
            lintResource(resource, i, resourceIds);
        }

        lintLeaderInResources(resources, patternTopology);
    }

    private static void lintResource(JsonNode resource, int index, Set<String> resourceIds)
            throws MalformedPatternException {

        String resourceType = resource.path("resource").asText();
        if (resourceType == null || resourceType.isEmpty() || resourceType.equals("null")) {
            throw new MalformedPatternException(
                    "spec.resources[" + index + "].resource is null or empty"
            );
        }

        String id = resource.path("id").asText();
        if (id == null || id.isEmpty() || id.equals("null")) {
            throw new MalformedPatternException(
                    "spec.resources[" + index + "].id is null or empty"
            );
        }

        if (resourceIds.contains(id)) {
            throw new MalformedPatternException(
                    "Duplicate resource id found: " + id
            );
        }
        resourceIds.add(id);

        JsonNode filters = resource.path("filters");
        if (!filters.isMissingNode()) {
            lintFilters(filters, index);
        }
    }

    private static void lintLeaderInResources(JsonNode resources, PatternTopology patternTopology) throws MalformedPatternException {
        boolean found = false;
        boolean leader;
        for (JsonNode resource : resources) {
            leader = resource.has("leader") && resource.get("leader").asBoolean();
            if(leader && !found) {
                found = true;
            }else if(leader) {
                throw new MalformedPatternException("multiple leaders found. In topology: " + patternTopology + " leader must be unique");
            }
        }
        if(!found) {
            throw new MalformedPatternException("leader not found.");
        }
    }

    private static void lintFilters(JsonNode filters, int resourceIndex)
            throws MalformedPatternException {

        if (filters.has("matchAll")) {
            lintFilterArray(filters.path("matchAll"), "matchAll", resourceIndex);
        }
        if (filters.has("matchNone")) {
            lintFilterArray(filters.path("matchNone"), "matchNone", resourceIndex);
        }
        if (filters.has("matchAny")) {
            lintFilterArray(filters.path("matchAny"), "matchAny", resourceIndex);
        }
    }

    private static void lintFilterArray(JsonNode filterArray, String filterType, int resourceIndex)
            throws MalformedPatternException {

        if (!filterArray.isArray()) {
            throw new MalformedPatternException(
                    "spec.resources[" + resourceIndex + "].filters." + filterType + " must be an array"
            );
        }

        for (int i = 0; i < filterArray.size(); i++) {
            JsonNode filter = filterArray.get(i);
            lintFilter(filter, filterType, resourceIndex, i);
        }
    }

    private static void lintFilter(JsonNode filter, String filterType, int resourceIndex, int filterIndex)
            throws MalformedPatternException {

        String key = filter.path("key").asText();
        if (key == null || key.isEmpty() || key.equals("null")) {
            throw new MalformedPatternException(
                    "spec.resources[" + resourceIndex + "].filters." + filterType +
                            "[" + filterIndex + "].key is null or empty"
            );
        }

        String operator = filter.path("operator").asText();
        if (operator == null || operator.isEmpty() || operator.equals("null")) {
            throw new MalformedPatternException(
                    "spec.resources[" + resourceIndex + "].filters." + filterType +
                            "[" + filterIndex + "].operator is null or empty"
            );
        }

        try {
            FilterOperator.valueOf(operator);
        } catch (IllegalArgumentException e) {
            List<String> validValues = Arrays.stream(FilterOperator.values())
                    .map(Enum::name)
                    .toList();
            throw new MalformedPatternException(
                    "spec.resources[" + resourceIndex + "].filters." + filterType +
                            "[" + filterIndex + "].operator must be one of " + validValues +
                            ", found: " + operator
            );
        }

        JsonNode values = filter.path("values");
        if (!values.isArray()) {
            throw new MalformedPatternException(
                    "spec.resources[" + resourceIndex + "].filters." + filterType +
                            "[" + filterIndex + "].values must be an array"
            );
        }
    }

    private static void lintActors(JsonNode actors, JsonNode resources)
            throws MalformedPatternException {

        if (actors == null || !actors.isArray() || actors.isEmpty()) {
            throw new MalformedPatternException("spec.actors must be a non-empty array");
        }

        Set<String> resourceIds = new HashSet<>();
        for (JsonNode resource : resources) {
            resourceIds.add(resource.path("id").asText());
        }

        for (int i = 0; i < actors.size(); i++) {
            String actorId = actors.get(i).asText();
            if (!resourceIds.contains(actorId)) {
                throw new MalformedPatternException(
                        "spec.actors[" + i + "] references unknown resource: " + actorId
                );
            }
        }
    }

    private static void lintRelationships(JsonNode commonRelationships, JsonNode relationships,
                                          JsonNode resources) throws MalformedPatternException {

        Set<String> resourceIds = new HashSet<>();
        for (JsonNode resource : resources) {
            resourceIds.add(resource.path("id").asText());
        }

        Set<String> relationshipIds = new HashSet<>();

        if (commonRelationships != null && commonRelationships.isArray()) {
            for (int i = 0; i < commonRelationships.size(); i++) {
                lintRelationship(commonRelationships.get(i), "commonRelationships", i,
                        resourceIds, relationshipIds);
            }
        }

        if (relationships != null && relationships.isArray()) {
            for (int i = 0; i < relationships.size(); i++) {
                lintRelationship(relationships.get(i), "relationships", i,
                        resourceIds, relationshipIds);
            }
        }
    }

    private static void lintRelationship(JsonNode relationship, String arrayName, int index,
                                         Set<String> resourceIds, Set<String> relationshipIds)
            throws MalformedPatternException {

        String id = relationship.path("id").asText();
        if (id == null || id.isEmpty() || id.equals("null")) {
            throw new MalformedPatternException(
                    "spec." + arrayName + "[" + index + "].id is null or empty"
            );
        }

        if (relationshipIds.contains(id)) {
            throw new MalformedPatternException(
                    "Duplicate relationship id found: " + id
            );
        }
        relationshipIds.add(id);

        String type = relationship.path("type").asText();
        if (type == null || type.isEmpty() || type.equals("null")) {
            throw new MalformedPatternException(
                    "spec." + arrayName + "[" + index + "].type is null or empty"
            );
        }

        try {
            RelationshipType.valueOf(type);
        } catch (IllegalArgumentException e) {
            List<String> validValues = Arrays.stream(RelationshipType.values())
                    .map(Enum::name)
                    .toList();
            throw new MalformedPatternException(
                    "spec." + arrayName + "[" + index + "].type must be one of " +
                            validValues + ", found: " + type
            );
        }

        JsonNode weight = relationship.path("weight");
        if (!weight.isMissingNode()) {
            double weightValue = weight.asDouble();
            if (weightValue < 0 || weightValue > 1) {
                throw new MalformedPatternException(
                        "spec." + arrayName + "[" + index + "].weight must be between 0 and 1"
                );
            }
        }

        JsonNode resourceIdsNode = relationship.path("resourceIds");
        if (resourceIdsNode.isMissingNode() || !resourceIdsNode.isArray() ||
                resourceIdsNode.isEmpty()) {
            throw new MalformedPatternException(
                    "spec." + arrayName + "[" + index + "].resourceIds must be a non-empty array"
            );
        }

        if(resourceIdsNode.size() < 2) {
            throw new MalformedPatternException("spec." + arrayName + "[" + index + "].resourceIds  must contain at least two values");
        }

        for (int i = 0; i < resourceIdsNode.size(); i++) {
            String resourceId = resourceIdsNode.get(i).asText();
            if (!resourceIds.contains(resourceId)) {
                throw new MalformedPatternException(
                        "spec." + arrayName + "[" + index + "].resourceIds[" + i +
                                "] references unknown resource: " + resourceId
                );
            }
        }
    }
}