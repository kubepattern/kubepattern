package analysis

// The matching of the engine before the limitations update (dev @ 50e0222, unchanged since
// a848c98), kept verbatim with a "legacy" prefix as the oracle of the differential tests:
// a Pattern that uses only the original constructs must get the same verdicts.

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/linter"
)

// legacyRun returns the names of the targets the legacy engine emits a Smell for.
func legacyRun(nodes map[types.UID]*unstructured.Unstructured, g GraphReader, p *linter.PatternAsCode) []string {
	t := p.Spec.Target
	targets := legacyFilterResources(nodes, t.Kind, t.APIVersion, t.Filters)
	deps := make(map[string][]*unstructured.Unstructured, len(p.Spec.Dependencies))
	for _, d := range p.Spec.Dependencies {
		deps[d.ID] = legacyFilterResources(nodes, d.Kind, d.APIVersion, d.Filters)
	}
	var out []string
	for _, target := range targets {
		if legacyEvaluateRelationships(target, deps, p.Spec.Relationships, g) {
			out = append(out, target.GetName())
		}
	}
	return out
}

// -------------------------
// filter.go
// -------------------------

func legacyFilterResources(nodes map[types.UID]*unstructured.Unstructured, kind string, apiVersion string, filters linter.Filters) []*unstructured.Unstructured {
	var candidates []*unstructured.Unstructured
	for _, node := range nodes {
		if !matchKind(node, kind, apiVersion) {
			continue
		}
		if !legacyMatchFilters(node, filters) {
			continue
		}
		candidates = append(candidates, node)
	}
	return candidates
}

func legacyMatchFilters(node *unstructured.Unstructured, filters linter.Filters) bool {
	for _, cond := range filters.MatchAll {
		if !legacyEvalCondition(node, cond) {
			return false
		}
	}
	if len(filters.MatchAny) > 0 {
		anyPassed := false
		for _, cond := range filters.MatchAny {
			if legacyEvalCondition(node, cond) {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}
	for _, cond := range filters.MatchNone {
		if legacyEvalCondition(node, cond) {
			return false
		}
	}
	return true
}

func legacyEvalCondition(node *unstructured.Unstructured, cond linter.FilterCondition) bool {
	values, found := legacyGetFieldValues(node.Object, cond.Path)

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
		return legacyCompareNumeric(values, cond.Values, func(a, b int) bool { return a > b })
	case linter.FilterGreaterOrEqual:
		return legacyCompareNumeric(values, cond.Values, func(a, b int) bool { return a >= b })
	case linter.FilterLessThan:
		return legacyCompareNumeric(values, cond.Values, func(a, b int) bool { return a < b })
	case linter.FilterLessOrEqual:
		return legacyCompareNumeric(values, cond.Values, func(a, b int) bool { return a <= b })
	case linter.FilterArraySizeEquals:
		return legacyCompareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a == b })
	case linter.FilterArraySizeGreaterThan:
		return legacyCompareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a > b })
	case linter.FilterArraySizeGreaterOrEqual:
		return legacyCompareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a >= b })
	case linter.FilterArraySizeLessThan:
		return legacyCompareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a < b })
	case linter.FilterArraySizeLessOrEqual:
		return legacyCompareArraySize(node.Object, cond.Path, cond.Values, func(a, b int) bool { return a <= b })
	}
	return false
}

func legacyCompareNumeric(fieldValues []any, targetValues []string, cmp func(a, b int) bool) bool {
	if len(fieldValues) == 0 || len(targetValues) == 0 {
		return false
	}
	target, err := strconv.Atoi(targetValues[0])
	if err != nil {
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

func legacyCompareArraySize(obj map[string]any, path string, targetValues []string, cmp func(a, b int) bool) bool {
	if len(targetValues) == 0 {
		return false
	}
	target, err := strconv.Atoi(targetValues[0])
	if err != nil {
		return false
	}
	val, ok := legacyGetFieldRaw(obj, path)
	if !ok {
		return false
	}
	items, ok := val.([]any)
	if !ok {
		return false
	}
	return cmp(len(items), target)
}

func legacyGetFieldValues(obj map[string]any, path string) ([]any, bool) {
	path = strings.TrimPrefix(path, ".")
	results := legacyNavigatePath(any(obj), strings.Split(path, "."))
	return results, len(results) > 0
}

func legacyGetFieldRaw(obj map[string]any, path string) (any, bool) {
	path = strings.TrimPrefix(path, ".")
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

func legacyNavigatePath(current any, parts []string) []any {
	if len(parts) == 0 {
		if current == nil {
			return nil
		}
		return []any{current}
	}
	part := parts[0]
	rest := parts[1:]
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
			results = append(results, legacyNavigatePath(item, rest)...)
		}
		return results
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil
	}
	next, ok := m[part]
	if !ok {
		return nil
	}
	return legacyNavigatePath(next, rest)
}

// -------------------------
// resolver.go
// -------------------------

func legacyEvaluateRelationships(target *unstructured.Unstructured, deps map[string][]*unstructured.Unstructured, rels linter.Relationships, g GraphReader) bool {
	for _, rel := range rels.MatchAll {
		if !legacyEvalRelationshipConfig(target, deps[rel.With], rel, g) {
			return false
		}
	}
	if len(rels.MatchAny) > 0 {
		anyPassed := false
		for _, rel := range rels.MatchAny {
			if legacyEvalRelationshipConfig(target, deps[rel.With], rel, g) {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}
	for _, rel := range rels.MatchNone {
		if legacyEvalRelationshipConfig(target, deps[rel.With], rel, g) {
			return false
		}
	}
	return true
}

func legacyEvalRelationshipConfig(target *unstructured.Unstructured, depCandidates []*unstructured.Unstructured, rel linter.Relationship, g GraphReader) bool {
	if len(depCandidates) == 0 {
		return false
	}
	for _, dep := range depCandidates {
		if legacyMatchRelationship(target, dep, rel, g) {
			return true
		}
	}
	return false
}

func legacyMatchRelationship(target, dep *unstructured.Unstructured, rel linter.Relationship, g GraphReader) bool {
	switch rel.Type {
	case linter.RelationshipCustom:
		for _, c := range rel.Criteria {
			if !legacyEvalSingleCriterion(target, dep, c) {
				return false
			}
		}
		return true
	case linter.RelationshipOwns:
		return g.IsParentOwner(target.GetUID(), dep.GetUID())
	case linter.RelationshipOwnedBy:
		return g.IsParentOwner(dep.GetUID(), target.GetUID())
	default:
		return false
	}
}

func legacyEvalSingleCriterion(target, dep *unstructured.Unstructured, c linter.Criteria) bool {
	targetVals, tFound := legacyGetFieldValues(target.Object, c.TargetPath)
	depVals, dFound := legacyGetFieldValues(dep.Object, c.DependencyPath)
	if !tFound || !dFound {
		return false
	}
	switch c.Operator {
	case linter.CriteriaEquals:
		for _, t := range targetVals {
			tStr := fmt.Sprintf("%v", t)
			for _, d := range depVals {
				if tStr == fmt.Sprintf("%v", d) {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}
