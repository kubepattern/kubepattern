package analysis

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubepattern-go/internal/fieldpath"
	"kubepattern-go/internal/linter"
)

// EvaluateRelationships checks the relationships for one target, preparing the
// dependencies on every call; the engine prepares them once per pattern instead.
func EvaluateRelationships(target *unstructured.Unstructured, deps map[string][]*unstructured.Unstructured, rels linter.Relationships, g GraphReader, tb fieldpath.Binding) bool {
	return PrepareRelationships(deps, rels).Evaluate(target, g, tb)
}

// getFieldValues returns every value a path reaches in obj, with no element bound.
func getFieldValues(obj map[string]any, path string) ([]any, bool) {
	results := fieldpath.MustParse(path).Values(obj, nil)
	return results, len(results) > 0
}
