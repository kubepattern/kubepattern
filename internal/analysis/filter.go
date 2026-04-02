package analysis

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/linter"
)

// FilterResources returns the graph nodes that match the kind,
// apiVersion, and all the filters provided. It can be used interchangeably
// for both Target and Dependency definitions.
func FilterResources(
	nodes map[types.UID]*unstructured.Unstructured,
	kind string,
	apiVersion string,
	filters linter.Filters,
) []*unstructured.Unstructured {
	slog.Info("Filtering resources")
	// Using the zero-value slice here is perfectly idiomatic in Go
	var candidates []*unstructured.Unstructured

	for _, node := range nodes {
		if !matchKind(node, kind, apiVersion) {
			continue
		}
		if !matchFilters(node, filters) {
			continue
		}
		candidates = append(candidates, node)
	}
	slog.Info("Filtered resources")
	return candidates
}

// matchKind verifies that the node matches the given kind and apiVersion.
func matchKind(node *unstructured.Unstructured, kind string, apiVersion string) bool {
	if node.GetKind() != kind {
		return false
	}
	if apiVersion != "" && node.GetAPIVersion() != apiVersion {
		return false
	}
	return true
}

// matchFilters applies matchAll, matchAny, and matchNone conditions to the node.
func matchFilters(node *unstructured.Unstructured, filters linter.Filters) bool {
	// matchAll — every condition must be true
	for _, cond := range filters.MatchAll {
		if !evalCondition(node, cond) {
			return false
		}
	}

	// matchAny — at least one condition must be true (skipped if the list is empty)
	if len(filters.MatchAny) > 0 {
		anyPassed := false
		for _, cond := range filters.MatchAny {
			if evalCondition(node, cond) {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}

	// matchNone — no condition must be true
	for _, cond := range filters.MatchNone {
		if evalCondition(node, cond) {
			return false
		}
	}

	return true
}

// evalCondition evaluates a single FilterCondition against the node.
func evalCondition(node *unstructured.Unstructured, cond linter.FilterCondition) bool {
	values, found := getFieldValues(node.Object, cond.Path)

	switch cond.Operator {

	case linter.FilterExists:
		return found && len(values) > 0

	case linter.FilterIsEmpty:
		if !found || len(values) == 0 {
			return true
		}
		for _, v := range values {
			switch val := v.(type) {
			case string:
				if val != "" {
					return false
				}
			case []any:
				if len(val) > 0 {
					return false
				}
			case map[string]any:
				if len(val) > 0 {
					return false
				}
			default:
				if v != nil {
					return false
				}
			}
		}
		return true

	case linter.FilterEquals:
		if !found {
			return false
		}
		for _, fieldVal := range values {
			str, ok := fieldVal.(string)
			if !ok {
				continue
			}
			for _, expected := range cond.Values {
				if str == expected {
					return true
				}
			}
		}
		return false

	case linter.FilterGreaterThan:
		return compareNumeric(values, cond.Values, func(a, b int) bool { return a > b })

	case linter.FilterGreaterOrEqual:
		return compareNumeric(values, cond.Values, func(a, b int) bool { return a >= b })

	case linter.FilterLessThan:
		return compareNumeric(values, cond.Values, func(a, b int) bool { return a < b })

	case linter.FilterLessOrEqual:
		return compareNumeric(values, cond.Values, func(a, b int) bool { return a <= b })

	case linter.FilterArraySizeEquals:
		return compareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a == b })

	case linter.FilterArraySizeGreaterThan:
		return compareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a > b })

	case linter.FilterArraySizeGreaterOrEqual:
		return compareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a >= b })

	case linter.FilterArraySizeLessThan:
		return compareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a < b })

	case linter.FilterArraySizeLessOrEqual:
		return compareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a <= b })
	}

	return false
}

// compareNumeric compares the numeric value extracted from the field with the target
// using the provided comparison function.
func compareNumeric(fieldValues []any, targetValues []string, cmp func(a, b int) bool) bool {
	if len(fieldValues) == 0 || len(targetValues) == 0 {
		return false
	}
	target, err := strconv.Atoi(targetValues[0])
	if err != nil {
		slog.Warn("compareNumeric: target value is not a number", "value", targetValues[0])
		return false
	}
	for _, v := range fieldValues {
		n, err := strconv.Atoi(strings.TrimSpace(fmt.Sprintf("%v", v)))
		if err != nil {
			continue
		}
		if cmp(n, target) {
			return true
		}
	}
	return false
}

// compareArraySize navigates the field and compares the size of the found collection.
func compareArraySize(obj map[string]any, path string, targetValues []string, cmp func(a, b int) bool) bool {
	if len(targetValues) == 0 {
		return false
	}
	target, err := strconv.Atoi(targetValues[0])
	if err != nil {
		slog.Warn("compareArraySize: target value is not a number", "value", targetValues[0])
		return false
	}

	val, ok := getFieldRaw(obj, path)
	if !ok {
		return false
	}
	items, ok := val.([]any)
	if !ok {
		return false
	}
	return cmp(len(items), target)
}

// getFieldValues navigates a path inside the unstructured object and returns
// all found values. It supports the [*] wildcard to iterate over arrays.
func getFieldValues(obj map[string]any, path string) ([]any, bool) {
	path = strings.TrimPrefix(path, ".")
	results := navigatePath(any(obj), strings.Split(path, "."))
	return results, len(results) > 0
}

// getFieldRaw navigates a simple path without expanding wildcards.
// It is used by compareArraySize to retrieve the raw array.
func getFieldRaw(obj map[string]any, path string) (any, bool) {
	path = strings.TrimPrefix(path, ".")
	// Removes [*] from the path if present — we want the array itself, not its elements
	path = strings.ReplaceAll(path, "[*]", "")

	parts := strings.Split(path, ".")
	current := any(obj)
	for _, part := range parts {
		if part == "" {
			continue
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// navigatePath recursively navigates the path on the unstructured data.
// When it encounters a segment with [*], it iterates over all array elements.
func navigatePath(current any, parts []string) []any {
	if len(parts) == 0 {
		if current == nil {
			return nil
		}
		return []any{current}
	}

	part := parts[0]
	rest := parts[1:]

	// Segment with array wildcard, e.g., "volumes[*]" or "items[*]"
	if strings.HasSuffix(part, "[*]") {
		fieldName := strings.TrimSuffix(part, "[*]")

		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		arrayVal, ok := m[fieldName]
		if !ok {
			return nil
		}
		items, ok := arrayVal.([]any)
		if !ok {
			return nil
		}

		var results []any
		for _, item := range items {
			results = append(results, navigatePath(item, rest)...)
		}
		return results
	}

	// Normal segment
	m, ok := current.(map[string]any)
	if !ok {
		return nil
	}
	next, ok := m[part]
	if !ok {
		return nil
	}
	return navigatePath(next, rest)
}
