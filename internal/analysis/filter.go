package analysis

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/fieldpath"
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
	return filterNodes(nodes, func(n *unstructured.Unstructured) bool { return matchKind(n, kind, apiVersion) }, filters)
}

// FilterDependency returns the candidates of a dependency: the nodes of its resource types
// that pass its filters.
func FilterDependency(nodes map[types.UID]*unstructured.Unstructured, dep linter.Dependency) []*unstructured.Unstructured {
	if dep.IsSingleKind() {
		return FilterResources(nodes, dep.Kind, dep.APIVersion, dep.Filters)
	}
	return filterNodes(nodes, kindMatcher(dep.Refs(), dep.Resolved), dep.Filters)
}

func filterNodes(nodes map[types.UID]*unstructured.Unstructured, match func(*unstructured.Unstructured) bool, filters linter.Filters) []*unstructured.Unstructured {
	var candidates []*unstructured.Unstructured
	for _, node := range nodes {
		if !match(node) {
			continue
		}
		if !matchFilters(node, filters, nil) {
			continue
		}
		candidates = append(candidates, node)
	}
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

// kindMatcher matches the nodes of a set of resource types: concrete kinds, every kind of a
// group version (kind "*"), and the types resolved at run time (categories, fromTarget).
func kindMatcher(refs []linter.ResourceRef, resolved linter.KindSet) func(*unstructured.Unstructured) bool {
	return func(n *unstructured.Unstructured) bool {
		if _, ok := resolved[linter.GVKKey(n.GetAPIVersion(), n.GetKind())]; ok {
			return true
		}
		for _, r := range refs {
			switch {
			case r.Category != "":
				continue // resolved by discovery
			case r.Kind == linter.KindWildcard:
				if n.GetAPIVersion() == r.APIVersion {
					return true
				}
			default:
				if matchKind(n, r.Kind, r.APIVersion) {
					return true
				}
			}
		}
		return false
	}
}

// matchFilters applies matchAll, matchAny, and matchNone conditions to the node, with the
// array elements of "[@]" paths bound by b.
func matchFilters(node *unstructured.Unstructured, filters linter.Filters, b fieldpath.Binding) bool {
	// matchAll — every condition must be true
	for _, cond := range filters.MatchAll {
		if !evalCondition(node, cond, b) {
			return false
		}
	}

	// matchAny — at least one condition must be true (skipped if the list is empty)
	if len(filters.MatchAny) > 0 {
		anyPassed := false
		for _, cond := range filters.MatchAny {
			if evalCondition(node, cond, b) {
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
		if evalCondition(node, cond, b) {
			return false
		}
	}

	return true
}

// evalCondition evaluates a single FilterCondition against the node.
func evalCondition(node *unstructured.Unstructured, cond linter.FilterCondition, b fieldpath.Binding) bool {
	path := fieldpath.MustParse(cond.Path)
	values := path.Values(node.Object, b)
	found := len(values) > 0

	switch cond.Operator {

	case linter.FilterExists:
		return found

	case linter.FilterIsEmpty:
		if !found {
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
		expected := cond.Values
		if cond.ValuesFrom != "" {
			expected = nil
			for _, v := range fieldpath.MustParse(cond.ValuesFrom).Values(node.Object, b) {
				if str, ok := scalarString(v); ok {
					expected = append(expected, str)
				}
			}
		}
		for _, fieldVal := range values {
			// Strings, booleans and numbers compare by their string form (G10): a filter
			// can test spec.suspend against "true".
			str, ok := scalarString(fieldVal)
			if !ok {
				continue
			}
			for _, e := range expected {
				if str == e {
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
		return compareArraySize(node.Object, cond.Path, cond.Values, b, func(a, b int) bool { return a == b })

	case linter.FilterArraySizeGreaterThan:
		return compareArraySize(node.Object, cond.Path, cond.Values, b, func(a, b int) bool { return a > b })

	case linter.FilterArraySizeGreaterOrEqual:
		return compareArraySize(node.Object, cond.Path, cond.Values, b, func(a, b int) bool { return a >= b })

	case linter.FilterArraySizeLessThan:
		return compareArraySize(node.Object, cond.Path, cond.Values, b, func(a, b int) bool { return a < b })

	case linter.FilterArraySizeLessOrEqual:
		return compareArraySize(node.Object, cond.Path, cond.Values, b, func(a, b int) bool { return a <= b })
	}

	return false
}

// scalarString returns the string form of a string, boolean or number; maps, arrays and
// nil are not scalars.
func scalarString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case bool, int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", val), true
	}
	return "", false
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
func compareArraySize(obj map[string]any, path string, targetValues []string, b fieldpath.Binding, cmp func(a, b int) bool) bool {
	if len(targetValues) == 0 {
		return false
	}
	target, err := strconv.Atoi(targetValues[0])
	if err != nil {
		slog.Warn("compareArraySize: target value is not a number", "value", targetValues[0])
		return false
	}

	val, ok := fieldpath.MustParse(path).Raw(obj, b)
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
	results := fieldpath.MustParse(path).Values(obj, nil)
	return results, len(results) > 0
}
