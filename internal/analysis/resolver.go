package analysis

import (
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/linter" // Adjust the import path to your module
)

// GraphReader defines an interface for retrieving unstructured objects from a graph by their unique identifier.
type GraphReader interface {
	GetByUID(uid types.UID) (*unstructured.Unstructured, bool)
	IsParentOwner(parent, child types.UID) bool
}

// EvaluateRelationships checks if a target satisfies the rules defined in the relationships block.
func EvaluateRelationships(
	target *unstructured.Unstructured,
	deps map[string][]*unstructured.Unstructured,
	rels linter.Relationships,
	g GraphReader, // <-- Uso l'interfaccia!
) bool {
	slog.Info("Evaluating relationships")
	// matchAll ? all relationships must be satisfied
	for _, rel := range rels.MatchAll {
		if !evalRelationshipConfig(target, deps[rel.With], rel, g) { // <-- Passato g
			return false
		}
	}

	// matchAny ? at least one relationship must be satisfied (ignored if empty)
	if len(rels.MatchAny) > 0 {
		anyPassed := false
		for _, rel := range rels.MatchAny {
			if evalRelationshipConfig(target, deps[rel.With], rel, g) { // <-- Passato g
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}

	// matchNone ? no relationships must be satisfied
	for _, rel := range rels.MatchNone {
		if evalRelationshipConfig(target, deps[rel.With], rel, g) { // <-- Passato g
			return false
		}
	}

	slog.Info("Evaluated relationships")

	return true
}

// evalRelationshipConfig evaluates a single relationship configuration
func evalRelationshipConfig(target *unstructured.Unstructured, depCandidates []*unstructured.Unstructured, rel linter.Relationship, g GraphReader) bool { // <-- Aggiunto g alla firma
	// If there are no candidates for this dependency, the relationship cannot exist
	if len(depCandidates) == 0 {
		return false
	}

	// The relationship is considered "satisfied" if it holds true with AT LEAST ONE of the candidate dependencies
	for _, dep := range depCandidates {
		if matchRelationship(target, dep, rel, g) {
			return true
		}
	}

	return false
}

// matchRelationship routes the logic based on the relationship type (custom vs. k8s native)
func matchRelationship(target, dep *unstructured.Unstructured, rel linter.Relationship, g GraphReader) bool {
	switch rel.Type {
	case linter.RelationshipCustom:
		return evalCustomCriteria(target, dep, rel.Criteria)

	case linter.RelationshipOwns:
		return evalOwns(target, dep, g)

	case linter.RelationshipOwnedBy:
		return evalOwnedBy(target, dep, g)

	default:
		slog.Warn("Unhandled relationship type", "type", rel.Type)
		return false
	}
}

// -------------------------
// Kubernetes Native Logic
// -------------------------
func evalOwns(target, dep *unstructured.Unstructured, g GraphReader) bool {
	return g.IsParentOwner(target.GetUID(), dep.GetUID())
}

func evalOwnedBy(target, dep *unstructured.Unstructured, g GraphReader) bool {
	return evalOwns(dep, target, g)
}

// -------------------------
// Custom Logic
// -------------------------

// evalCustomCriteria evaluates the list of custom criteria.
// For the custom relationship to be valid between target and dep, ALL criteria must be satisfied.
func evalCustomCriteria(target, dep *unstructured.Unstructured, criteria []linter.Criteria) bool {
	for _, c := range criteria {
		if !evalSingleCriterion(target, dep, c) {
			return false
		}
	}
	return true
}

func evalSingleCriterion(target, dep *unstructured.Unstructured, c linter.Criteria) bool {
	// Uses the getFieldValues function already written in filters.go
	targetVals, tFound := getFieldValues(target.Object, c.TargetPath)
	depVals, dFound := getFieldValues(dep.Object, c.DependencyPath)

	// If either path does not exist, the criterion fails
	if !tFound || !dFound {
		return false
	}

	switch c.Operator {
	case linter.CriteriaEquals:
		return evalOperatorEquals(targetVals, depVals)
	// You can easily add CONTAINS and LABEL_SELECTOR here in the future
	default:
		slog.Warn("Criteria operator not yet implemented", "operator", c.Operator)
		return false
	}
}

// evalOperatorEquals checks if there is an intersection between the values extracted from the target and the dependency.
// If even a single target value equals a dependency value, it returns true.
func evalOperatorEquals(targetVals []any, depVals []any) bool {
	for _, t := range targetVals {
		// Normalize to string to facilitate comparison between types extracted from YAML
		tStr := fmt.Sprintf("%v", t)

		for _, d := range depVals {
			dStr := fmt.Sprintf("%v", d)
			if tStr == dStr {
				return true
			}
		}
	}
	return false
}
