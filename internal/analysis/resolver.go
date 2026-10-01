package analysis

import (
	"fmt"
	"log/slog"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/fieldpath"
	"kubepattern-go/internal/linter"
)

// GraphReader defines an interface for retrieving unstructured objects from a graph by their unique identifier.
type GraphReader interface {
	GetByUID(uid types.UID) (*unstructured.Unstructured, bool)
	IsParentOwner(parent, child types.UID) bool
}

// EvaluateRelationships checks if a target satisfies the rules defined in the relationships
// block. tb binds the target's "[@]" anchors (nil when the pattern has none): every
// relationship is evaluated for the same target element.
func EvaluateRelationships(
	target *unstructured.Unstructured,
	deps map[string][]*unstructured.Unstructured,
	rels linter.Relationships,
	g GraphReader,
	tb fieldpath.Binding,
) bool {
	// matchAll — all relationships must be satisfied
	for i := range rels.MatchAll {
		if !evalRelationshipConfig(target, deps[rels.MatchAll[i].With], compile(&rels.MatchAll[i], target, tb), g, tb) {
			return false
		}
	}

	// matchAny — at least one relationship must be satisfied (ignored if empty)
	if len(rels.MatchAny) > 0 {
		anyPassed := false
		for i := range rels.MatchAny {
			if evalRelationshipConfig(target, deps[rels.MatchAny[i].With], compile(&rels.MatchAny[i], target, tb), g, tb) {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}

	// matchNone — no relationships must be satisfied
	for i := range rels.MatchNone {
		if evalRelationshipConfig(target, deps[rels.MatchNone[i].With], compile(&rels.MatchNone[i], target, tb), g, tb) {
			return false
		}
	}

	return true
}

// compiledRel is a relationship prepared for one target binding: its paths are parsed and
// the target side of every criterion is extracted once, not once per dependency candidate.
type compiledRel struct {
	rel         *linter.Relationship
	depPaths    []*fieldpath.Path
	depAnchored bool
	targetVals  [][]string // per criterion
	// impossible is true when a criterion has no target value: no dependency can satisfy it.
	impossible bool
}

func compile(rel *linter.Relationship, target *unstructured.Unstructured, tb fieldpath.Binding) *compiledRel {
	cr := &compiledRel{rel: rel, depPaths: DependencyPaths(*rel)}
	for _, p := range cr.depPaths {
		if p.HasAnchor() {
			cr.depAnchored = true
			break
		}
	}
	cr.targetVals = make([][]string, len(rel.Criteria))
	for i := range rel.Criteria {
		c := &rel.Criteria[i]
		cr.targetVals[i] = sideValues(target.Object, c.TargetPath, c.TargetValue, c.TargetDefault, c.TargetTransform, tb)
		if len(cr.targetVals[i]) == 0 {
			cr.impossible = true
		}
	}
	return cr
}

// evalRelationshipConfig evaluates a single relationship configuration
func evalRelationshipConfig(target *unstructured.Unstructured, depCandidates []*unstructured.Unstructured, cr *compiledRel, g GraphReader, tb fieldpath.Binding) bool {
	// If there are no candidates for this dependency, the relationship cannot exist
	if len(depCandidates) == 0 {
		return false
	}
	if cr.impossible {
		return false
	}

	// The relationship is considered "satisfied" if it holds true with AT LEAST ONE of the candidate dependencies
	for _, dep := range depCandidates {
		if matchRelationship(target, dep, cr, g, tb) {
			return true
		}
	}

	return false
}

// matchRelationship routes the logic based on the relationship type (custom vs. k8s native).
// For custom, selects and selectedBy the dependency's "[@]" anchors are enumerated here: the
// relationship holds with dep when some element binding satisfies it.
func matchRelationship(target, dep *unstructured.Unstructured, cr *compiledRel, g GraphReader, tb fieldpath.Binding) bool {
	switch cr.rel.Type {
	case linter.RelationshipOwns:
		return evalOwns(target, dep, g)

	case linter.RelationshipOwnedBy:
		return evalOwnedBy(target, dep, g)

	case linter.RelationshipCustom, linter.RelationshipSelects, linter.RelationshipSelectedBy:
		if !cr.depAnchored {
			return matchBound(target, dep, cr, tb, nil)
		}
		for _, db := range fieldpath.Enumerate(dep.Object, cr.depPaths, nil) {
			if matchBound(target, dep, cr, tb, db) {
				return true
			}
		}
		return false

	default:
		slog.Warn("Unhandled relationship type", "type", cr.rel.Type)
		return false
	}
}

// matchBound evaluates a custom or selector relationship with both sides' elements bound.
func matchBound(target, dep *unstructured.Unstructured, cr *compiledRel, tb, db fieldpath.Binding) bool {
	rel := cr.rel
	switch rel.Type {
	case linter.RelationshipSelects:
		if !selectorMatches(target.Object, rel.Selector(), tb, dep.GetLabels()) {
			return false
		}
	case linter.RelationshipSelectedBy:
		if !selectorMatches(dep.Object, rel.Selector(), db, target.GetLabels()) {
			return false
		}
	}
	// For the custom relationship to be valid between target and dep, ALL criteria must be satisfied.
	for i := range rel.Criteria {
		c := &rel.Criteria[i]
		depVals := sideValues(dep.Object, c.DependencyPath, c.DependencyValue, c.DependencyDefault, c.DependencyTransform, db)
		if len(depVals) == 0 {
			return false
		}
		switch c.Operator {
		case linter.CriteriaEquals:
			if !evalOperatorEquals(cr.targetVals[i], depVals) {
				return false
			}
		default:
			slog.Warn("Criteria operator not yet implemented", "operator", c.Operator)
			return false
		}
	}
	return true
}

// DependencyPaths returns the dependency-side paths of a relationship (they define the
// dependency's "[@]" anchors).
func DependencyPaths(rel linter.Relationship) []*fieldpath.Path {
	var out []*fieldpath.Path
	for _, c := range rel.Criteria {
		if c.DependencyPath != "" {
			out = append(out, fieldpath.MustParse(c.DependencyPath))
		}
		if c.DependencyDefault != nil && c.DependencyDefault.Path != "" {
			out = append(out, fieldpath.MustParse(c.DependencyDefault.Path))
		}
	}
	if rel.Type == linter.RelationshipSelectedBy {
		out = append(out, fieldpath.MustParse(rel.Selector()))
	}
	return out
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

// selectorMatches reports whether a label selector found at path in obj matches lbls.
// Both selector shapes are accepted: a metav1.LabelSelector (matchLabels / matchExpressions;
// an empty one selects everything) and a plain label map as in Service.spec.selector (an
// empty map selects nothing). A missing selector selects nothing.
func selectorMatches(obj map[string]any, path string, b fieldpath.Binding, lbls map[string]string) bool {
	set := labels.Set(lbls)
	for _, v := range fieldpath.MustParse(path).Values(obj, b) {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		_, hasLabels := m["matchLabels"]
		_, hasExprs := m["matchExpressions"]
		if hasLabels || hasExprs || len(m) == 0 && isLabelSelectorPath(path) {
			var ls metav1.LabelSelector
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &ls); err != nil {
				slog.Warn("invalid label selector", "path", path, "error", err)
				continue
			}
			sel, err := metav1.LabelSelectorAsSelector(&ls)
			if err != nil {
				slog.Warn("invalid label selector", "path", path, "error", err)
				continue
			}
			if sel.Matches(set) {
				return true
			}
			continue
		}
		if len(m) == 0 {
			continue
		}
		matched := true
		for k, want := range m {
			if got, ok := lbls[k]; !ok || fmt.Sprintf("%v", want) != got {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// isLabelSelectorPath reports whether an empty map at path is a metav1.LabelSelector, where
// {} selects everything (NetworkPolicy podSelector, namespaceSelector, labelSelector), rather
// than a label map, where {} selects nothing (Service spec.selector). Fields whose name ends
// in "Selector" follow the LabelSelector convention.
func isLabelSelectorPath(path string) bool {
	return strings.HasSuffix(path, "Selector")
}

// -------------------------
// Custom Logic
// -------------------------

// sideValues extracts the values of one side of a criterion: a literal, or the path's
// values, else the default (another path or a literal); each is rewritten by the transform.
func sideValues(obj map[string]any, path string, literal *string, def *linter.ValueDefault, tr *linter.Transform, b fieldpath.Binding) []string {
	var raw []any
	if literal != nil {
		raw = []any{*literal}
	} else {
		raw = fieldpath.MustParse(path).Values(obj, b)
	}
	if len(raw) == 0 && def != nil {
		if def.Value != nil {
			raw = []any{*def.Value}
		} else if def.Path != "" {
			raw = fieldpath.MustParse(def.Path).Values(obj, b)
		}
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		// Normalize to string to facilitate comparison between types extracted from YAML
		str, isString := v.(string)
		if !isString {
			str = fmt.Sprintf("%v", v)
		}
		if str, ok := linter.ApplyTransform(tr, str); ok {
			out = append(out, str)
		}
	}
	return out
}

// evalOperatorEquals checks if there is an intersection between the values extracted from the target and the dependency.
// If even a single target value equals a dependency value, it returns true.
func evalOperatorEquals(targetVals []string, depVals []string) bool {
	for _, t := range targetVals {
		for _, d := range depVals {
			if t == d {
				return true
			}
		}
	}
	return false
}
