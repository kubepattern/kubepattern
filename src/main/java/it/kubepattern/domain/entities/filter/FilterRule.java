package it.kubepattern.domain.entities.filter;

import com.jayway.jsonpath.JsonPath;
import com.jayway.jsonpath.PathNotFoundException;
import lombok.*;
import lombok.extern.slf4j.Slf4j;

import java.util.Collections;
import java.util.Set;
import java.util.stream.Collectors;

@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
@ToString
@Slf4j
public class FilterRule {
    String jsonQuery;//default JSON_PATH
    FilterOperator operator;
    String[] values;
    FilterQueryEngine filterQueryEngine;

    public FilterRule(String jsonQuery, FilterOperator operator, String [] values) {
        this.jsonQuery = jsonQuery;
        this.operator = operator;
        this.values = values;
        this.filterQueryEngine= FilterQueryEngine.JSON_PATH;
    }

    public boolean match(String jsonResource) {
        return switch (operator) {
            case EQUALS -> matchEquals(jsonResource);
            case NOT_EQUALS -> matchNotEquals(jsonResource);
            case EXISTS -> matchExists(jsonResource);
            case NOT_EXISTS -> matchNotExists(jsonResource);
            case LESS_OR_EQUAL -> matchLessOrEqual(jsonResource);
            case GREATER_OR_EQUAL -> matchGreaterOrEqual(jsonResource);
            case LESS_THAN -> matchLess(jsonResource);
            case GREATER_THAN -> matchGreater(jsonResource);
            case IS_EMPTY -> matchIsEmpty(jsonResource);
            case ARRAY_SIZE_EQUALS -> matchArraySizeEquals(jsonResource);
            case ARRAY_SIZE_LESS_THAN -> matchArraySizeLessThan(jsonResource);
            case ARRAY_SIZE_GREATER_THAN -> matchArraySizeGreaterThan(jsonResource); // Uncomment if this operator exists
            case ARRAY_SIZE_GREATER_OR_EQUAL -> matchArraySizeGreaterThanOrEqualTo(jsonResource);
            case ARRAY_SIZE_LESS_OR_EQUAL -> matchArraySizeLessThanOrEqualTo(jsonResource);
        };
    }

    /**
     * Helper method that executes the JSONPath query and always returns a list of results.
     * It handles PathNotFoundException and converts single results into lists.
     */
    private java.util.List<Object> getResults(String jsonResource) {
        try {
            JsonPath jsonPath = JsonPath.compile(jsonQuery);
            Object result = jsonPath.read(jsonResource);

            if (result instanceof java.util.Collection<?>) {
                // The result is already a collection (JSON array)
                return new java.util.ArrayList<>((java.util.Collection<?>) result);
            } else {
                // The result is a single value (or null). We wrap it in a list.
                return Collections.singletonList(result);
            }
        } catch (PathNotFoundException e) {
            // The path does not exist, so no results.
            return Collections.emptyList();
        } catch (Exception e) {
            log.error("Unexpected error while reading JSONPath query '{}': {}", jsonQuery, e.getMessage(), e);
            return Collections.emptyList();
        }
    }

    // --- Existence Methods ---

    boolean matchExists(String jsonResource) {
        // Exists if our helper query returns *at least one* result.
        return !getResults(jsonResource).isEmpty();
    }

    boolean matchNotExists(String jsonResource) {
        // Does not exist if our helper query returns *no* results.
        return getResults(jsonResource).isEmpty();
    }

    // --- Value Comparison Methods (handling arrays) ---

    boolean matchEquals(String jsonResource) {
        java.util.List<Object> results = getResults(jsonResource);
        if (results.isEmpty() || values == null || values.length == 0) {
            return false;
        }

        // Checks if *at least one* of the values in 'values' is present in the results
        Set<String> targetValues = java.util.Arrays.stream(values).collect(Collectors.toSet());

        for (Object item : results) {
            if (item != null && targetValues.contains(item.toString())) {
                return true; // Found a match
            }
        }
        return false; // No result matched
    }

    boolean matchNotEquals(String jsonResource) {
        // The simplest logic for NOT_EQUALS is "it is not true that it is EQUALS"
        // (No result matched the target values)
        return !matchEquals(jsonResource);
    }

    boolean matchGreater(String jsonResource) {
        java.util.List<Object> results = getResults(jsonResource);
        if (results.isEmpty()) {
            return false;
        }

        try {
            int targetValue = Integer.parseInt(values[0]);
            for (Object item : results) {
                if (item == null) continue;
                try {
                    int valueResource = Integer.parseInt(item.toString());
                    if (valueResource > targetValue) {
                        return true; // Found one that matches
                    }
                } catch (NumberFormatException nfe) {
                    log.warn("Cannot compare non-numeric value '{}' for query '{}'", item, jsonQuery);
                }
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for GREATER_THAN: {}", (Object) values, e);
        }
        return false; // No match
    }

    boolean matchGreaterOrEqual(String jsonResource) {
        java.util.List<Object> results = getResults(jsonResource);
        if (results.isEmpty()) {
            return false;
        }

        try {
            int targetValue = Integer.parseInt(values[0]);
            for (Object item : results) {
                if (item == null) continue;
                try {
                    int valueResource = Integer.parseInt(item.toString());
                    if (valueResource >= targetValue) {
                        return true; // Found one that matches
                    }
                } catch (NumberFormatException nfe) {
                    log.warn("Cannot compare non-numeric value '{}' for query '{}'", item, jsonQuery);
                }
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for GREATER_OR_EQUAL: {}", (Object) values, e);
        }
        return false; // No match
    }

    boolean matchLess(String jsonResource) {
        java.util.List<Object> results = getResults(jsonResource);
        if (results.isEmpty()) {
            return false;
        }

        try {
            int targetValue = Integer.parseInt(values[0]);
            for (Object item : results) {
                if (item == null) continue;
                try {
                    int valueResource = Integer.parseInt(item.toString());
                    if (valueResource < targetValue) {
                        return true; // Found one that matches
                    }
                } catch (NumberFormatException nfe) {
                    log.warn("Cannot compare non-numeric value '{}' for query '{}'", item, jsonQuery);
                }
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for LESS_THAN: {}", (Object) values, e);
        }
        return false; // No match
    }

    boolean matchLessOrEqual(String jsonResource) {
        java.util.List<Object> results = getResults(jsonResource);
        if (results.isEmpty()) {
            return false;
        }

        try {
            int targetValue = Integer.parseInt(values[0]);
            for (Object item : results) {
                if (item == null) continue;
                try {
                    int valueResource = Integer.parseInt(item.toString());
                    if (valueResource <= targetValue) {
                        return true; // Found one that matches
                    }
                } catch (NumberFormatException nfe) {
                    log.warn("Cannot compare non-numeric value '{}' for query '{}'", item, jsonQuery);
                }
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for LESS_OR_EQUAL: {}", (Object) values, e);
        }
        return false; // No match
    }

    // --- Size/Content Methods (Original Logic OK) ---
    // These methods expect the query to RETURN a container (String, Map, Collection)

    boolean matchIsEmpty(String jsonResource) {
        // This method uses the original 'matchExists' logic to check if the path exists.
        // We must use the original 'matchExists' logic (just try/catch)
        boolean pathExists;
        Object value = null;
        try {
            JsonPath jsonPath = JsonPath.compile(jsonQuery);
            value = jsonPath.read(jsonResource);
            pathExists = true;
        } catch (PathNotFoundException e) {
            pathExists = false;
        }

        if (pathExists) {
            if (value instanceof String) {
                return ((String) value).isEmpty();
            } else if (value instanceof java.util.Collection) {
                return ((java.util.Collection<?>) value).isEmpty();
            } else if (value instanceof java.util.Map) {
                return ((java.util.Map<?, ?>) value).isEmpty();
            } else {
                return value == null;
            }
        }
        return false;
    }

    /**
     * Helper for the 'matchArraySize' methods.
     * These methods expect the query (e.g., '.status.containerStatuses')
     * to return a single object that is a Collection, and they check its size.
     */
    private Integer getCollectionSize(String jsonResource) {
        try {
            JsonPath jsonPath = JsonPath.compile(jsonQuery);
            Object value = jsonPath.read(jsonResource);
            if (value instanceof java.util.Collection) {
                return ((java.util.Collection<?>) value).size();
            }
        } catch (PathNotFoundException e) {
            // Path not found, not an error, just no match
        } catch (Exception e) {
            log.warn("Error during getCollectionSize for query '{}': {}", jsonQuery, e.getMessage());
        }
        return null; // Not a collection or path not found
    }

    boolean matchArraySizeEquals(String jsonResource) {
        try {
            Integer size = getCollectionSize(jsonResource);
            if (size != null) {
                int targetSize = Integer.parseInt(values[0]);
                return size == targetSize;
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for ARRAY_SIZE_EQUALS: {}", (Object) values, e);
        }
        return false;
    }

    boolean matchArraySizeGreaterThan(String jsonResource) {
        try {
            Integer size = getCollectionSize(jsonResource);
            if (size != null) {
                int targetSize = Integer.parseInt(values[0]);
                return size > targetSize;
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for ARRAY_SIZE_GREATER_THAN: {}", values, e);
        }
        return false;
    }

    boolean matchArraySizeLessThan(String jsonResource) {
        try {
            Integer size = getCollectionSize(jsonResource);
            if (size != null) {
                int targetSize = Integer.parseInt(values[0]);
                return size < targetSize;
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for ARRAY_SIZE_LESS_THAN: {}", values, e);
        }
        return false;
    }

    boolean matchArraySizeGreaterThanOrEqualTo(String jsonResource) {
        try {
            Integer size = getCollectionSize(jsonResource);
            if (size != null) {
                int targetSize = Integer.parseInt(values[0]);
                return size >= targetSize;
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for ARRAY_SIZE_GREATER_OR_EQUAL: {}", values, e);
        }
        return false;
    }

    boolean matchArraySizeLessThanOrEqualTo(String jsonResource) {
        try {
            Integer size = getCollectionSize(jsonResource);
            if (size != null) {
                int targetSize = Integer.parseInt(values[0]);
                return size <= targetSize;
            }
        } catch (NumberFormatException | ArrayIndexOutOfBoundsException e) {
            log.error("Invalid or missing target value for ARRAY_SIZE_LESS_OR_EQUAL: {}", values, e);
        }
        return false;
    }
}