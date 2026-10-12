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

// PreparedRelationships is a relationships block with the dependency side of every
// relationship extracted once: it does not depend on the target, so the matching of each
// target only compares the target's values with the prepared ones.
type PreparedRelationships struct {
	matchAll, matchAny, matchNone []*preparedRel
}

// preparedRel is one relationship with its dependency candidates. For custom, selects and
// selectedBy, views holds one entry per candidate and element binding of the dependency's
// "[@]" anchors that can satisfy the criteria; the relationship holds when the target
// matches at least one view.
type preparedRel struct {
	rel   *linter.Relationship
	cands []*unstructured.Unstructured
	views []depView
}

// depView is what a relationship reads from one dependency under one element binding.
type depView struct {
	// vals holds the dependency side of each criterion; none is empty.
	vals [][]string
	// labels are the dependency's labels (selects).
	labels labels.Set
	// selectors are the dependency's selectors under the binding (selectedBy); never empty.
	selectors []labels.Selector
}

// PrepareRelationships extracts the dependency side of every relationship.
func PrepareRelationships(deps map[string][]*unstructured.Unstructured, rels linter.Relationships) *PreparedRelationships {
	prepare := func(group []linter.Relationship) []*preparedRel {
		out := make([]*preparedRel, len(group))
		for i := range group {
			out[i] = prepareRel(&group[i], deps[group[i].With])
		}
		return out
	}
	return &PreparedRelationships{
		matchAll:  prepare(rels.MatchAll),
		matchAny:  prepare(rels.MatchAny),
		matchNone: prepare(rels.MatchNone),
	}
}

func prepareRel(rel *linter.Relationship, cands []*unstructured.Unstructured) *preparedRel {
	pr := &preparedRel{rel: rel, cands: cands}
	switch rel.Type {
	case linter.RelationshipCustom, linter.RelationshipSelects, linter.RelationshipSelectedBy:
	default:
		return pr
	}
	depPaths := fieldpath.ParseAll(rel.DependencyPaths())
	anchored := false
	for _, p := range depPaths {
		if p.HasAnchor() {
			anchored = true
			break
		}
	}
	for _, dep := range cands {
		bindings := []fieldpath.Binding{nil}
		if anchored {
			bindings = fieldpath.Enumerate(dep.Object, depPaths, nil)
		}
		for _, db := range bindings {
			if v, ok := makeView(rel, dep, db); ok {
				pr.views = append(pr.views, v)
			}
		}
	}
	return pr
}

// makeView reads a dependency under one binding; ok is false when the view can never
// satisfy the relationship (a criterion side without values, or no selector).
func makeView(rel *linter.Relationship, dep *unstructured.Unstructured, db fieldpath.Binding) (depView, bool) {
	v := depView{vals: make([][]string, len(rel.Criteria))}
	for i := range rel.Criteria {
		c := &rel.Criteria[i]
		v.vals[i] = sideValues(dep.Object, c.DependencyPath, c.DependencyValue, c.DependencyDefault, c.DependencyTransform, db)
		if len(v.vals[i]) == 0 {
			return v, false
		}
	}
	switch rel.Type {
	case linter.RelationshipSelects:
		v.labels = labels.Set(dep.GetLabels())
	case linter.RelationshipSelectedBy:
		v.selectors = compileSelectors(dep.Object, rel.Selector(), db, rel.EmptySelector)
		if len(v.selectors) == 0 {
			return v, false
		}
	}
	return v, true
}

// Evaluate checks the relationships for a target under its element binding tb.
func (p *PreparedRelationships) Evaluate(target *unstructured.Unstructured, g GraphReader, tb fieldpath.Binding) bool {
	// matchAll — all relationships must be satisfied
	for _, pr := range p.matchAll {
		if !pr.holds(target, g, tb) {
			return false
		}
	}

	// matchAny — at least one relationship must be satisfied (ignored if empty)
	if len(p.matchAny) > 0 {
		anyPassed := false
		for _, pr := range p.matchAny {
			if pr.holds(target, g, tb) {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			return false
		}
	}

	// matchNone — no relationships must be satisfied
	for _, pr := range p.matchNone {
		if pr.holds(target, g, tb) {
			return false
		}
	}

	return true
}

// holds reports whether the relationship is satisfied with AT LEAST ONE dependency
// candidate (for custom, selects and selectedBy: with one of its element bindings).
func (pr *preparedRel) holds(target *unstructured.Unstructured, g GraphReader, tb fieldpath.Binding) bool {
	rel := pr.rel
	switch rel.Type {
	case linter.RelationshipOwns:
		for _, dep := range pr.cands {
			if evalOwns(target, dep, g) {
				return true
			}
		}
		return false

	case linter.RelationshipOwnedBy:
		for _, dep := range pr.cands {
			if evalOwnedBy(target, dep, g) {
				return true
			}
		}
		return false

	case linter.RelationshipCustom, linter.RelationshipSelects, linter.RelationshipSelectedBy:
		if len(pr.views) == 0 {
			return false
		}
		// The target side is extracted once per target binding; a criterion with no target
		// value cannot be satisfied by any dependency.
		targetVals := make([][]string, len(rel.Criteria))
		for i := range rel.Criteria {
			c := &rel.Criteria[i]
			targetVals[i] = sideValues(target.Object, c.TargetPath, c.TargetValue, c.TargetDefault, c.TargetTransform, tb)
			if len(targetVals[i]) == 0 {
				return false
			}
		}
		var targetSelectors []labels.Selector
		var targetLabels labels.Set
		switch rel.Type {
		case linter.RelationshipSelects:
			targetSelectors = compileSelectors(target.Object, rel.Selector(), tb, rel.EmptySelector)
			if len(targetSelectors) == 0 {
				return false
			}
		case linter.RelationshipSelectedBy:
			targetLabels = labels.Set(target.GetLabels())
		}
		for i := range pr.views {
			if pr.views[i].matches(rel, targetVals, targetSelectors, targetLabels) {
				return true
			}
		}
		return false

	default:
		slog.Warn("Unhandled relationship type", "type", rel.Type)
		return false
	}
}

// matches evaluates a custom or selector relationship between the target values and one
// dependency view. For the relationship to hold, ALL criteria must be satisfied.
func (v *depView) matches(rel *linter.Relationship, targetVals [][]string, targetSelectors []labels.Selector, targetLabels labels.Set) bool {
	switch rel.Type {
	case linter.RelationshipSelects:
		if !anySelects(targetSelectors, v.labels) {
			return false
		}
	case linter.RelationshipSelectedBy:
		if !anySelects(v.selectors, targetLabels) {
			return false
		}
	}
	for i := range rel.Criteria {
		switch rel.Criteria[i].Operator {
		case linter.CriteriaEquals:
			if !evalOperatorEquals(targetVals[i], v.vals[i]) {
				return false
			}
		default:
			slog.Warn("Criteria operator not yet implemented", "operator", rel.Criteria[i].Operator)
			return false
		}
	}
	return true
}

func anySelects(selectors []labels.Selector, lbls labels.Set) bool {
	for _, s := range selectors {
		if s.Matches(lbls) {
			return true
		}
	}
	return false
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

// compileSelectors returns the label selectors found at path in obj under binding b, as
// selectors that select something; a missing selector, an invalid one or one that selects
// nothing yields none. Both shapes are accepted: a metav1.LabelSelector (matchLabels /
// matchExpressions) and a plain label map as in Service.spec.selector. A selector without
// requirements selects what empty says; when empty is unset, a LabelSelector or an empty
// map in a field named "*Selector" selects everything (NetworkPolicy podSelector: {}) and an
// empty map in any other field selects nothing (an empty Service selector).
func compileSelectors(obj map[string]any, path string, b fieldpath.Binding, empty linter.EmptySelector) []labels.Selector {
	var out []labels.Selector
	for _, v := range fieldpath.MustParse(path).Values(obj, b) {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if sel, ok := compileSelector(m, path, empty); ok {
			out = append(out, sel)
		}
	}
	return out
}

func compileSelector(m map[string]any, path string, empty linter.EmptySelector) (labels.Selector, bool) {
	_, hasLabels := m["matchLabels"]
	_, hasExprs := m["matchExpressions"]
	isLabelSelector := hasLabels || hasExprs

	if !isLabelSelector && len(m) > 0 {
		set := labels.Set{}
		for k, val := range m {
			set[k] = fmt.Sprintf("%v", val)
		}
		return labels.SelectorFromSet(set), true
	}

	var ls metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &ls); err != nil {
		slog.Warn("invalid label selector", "path", path, "error", err)
		return nil, false
	}
	if len(ls.MatchLabels) == 0 && len(ls.MatchExpressions) == 0 {
		switch empty {
		case linter.EmptySelectorAll:
			return labels.Everything(), true
		case linter.EmptySelectorNone:
			return nil, false
		}
		if isLabelSelector || isLabelSelectorPath(path) {
			return labels.Everything(), true
		}
		return nil, false
	}
	sel, err := metav1.LabelSelectorAsSelector(&ls)
	if err != nil {
		slog.Warn("invalid label selector", "path", path, "error", err)
		return nil, false
	}
	return sel, true
}

// isLabelSelectorPath reports whether an empty map at path is a metav1.LabelSelector, where
// {} selects everything (NetworkPolicy podSelector, namespaceSelector, labelSelector), rather
// than a label map, where {} selects nothing (Service spec.selector). Fields whose name ends
// in "Selector" follow the LabelSelector convention, however the path addresses them
// (spec.podSelector, spec['podSelector']).
func isLabelSelectorPath(path string) bool {
	return strings.HasSuffix(fieldpath.MustParse(path).LastKey(), "Selector")
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
