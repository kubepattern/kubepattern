package analysis

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	"kubepattern-go/internal/linter"
)

// compileSelectors: both selector shapes, the empty-selector conventions and the explicit
// emptySelector option, invalid selectors, and element bindings.
func TestCompileSelectors(t *testing.T) {
	web := labels.Set{"app": "web", "tier": "front"}
	none := labels.Set{}
	obj := func(sel any) map[string]any {
		return map[string]any{"spec": map[string]any{"selector": sel, "podSelector": sel}}
	}
	tests := []struct {
		name  string
		sel   any
		path  string
		empty linter.EmptySelector
		// want: nil means no selector (selects nothing); otherwise the verdict on each label set.
		want []bool // [web, none]
	}{
		{"plain map", map[string]any{"app": "web"}, "spec.selector", "", []bool{true, false}},
		{"plain map, two labels", map[string]any{"app": "web", "tier": "back"}, "spec.selector", "", []bool{false, false}},
		{"plain map with a non-string value", map[string]any{"replicas": int64(1)}, "spec.selector", "", []bool{false, false}},
		{"matchLabels", map[string]any{"matchLabels": map[string]any{"tier": "front"}}, "spec.selector", "", []bool{true, false}},
		{"matchExpressions In", map[string]any{"matchExpressions": []any{map[string]any{"key": "app", "operator": "In", "values": []any{"web", "api"}}}}, "spec.selector", "", []bool{true, false}},
		{"matchExpressions NotIn", map[string]any{"matchExpressions": []any{map[string]any{"key": "app", "operator": "NotIn", "values": []any{"web"}}}}, "spec.selector", "", []bool{false, true}},
		{"matchExpressions Exists", map[string]any{"matchExpressions": []any{map[string]any{"key": "tier", "operator": "Exists"}}}, "spec.selector", "", []bool{true, false}},
		{"matchExpressions DoesNotExist", map[string]any{"matchExpressions": []any{map[string]any{"key": "tier", "operator": "DoesNotExist"}}}, "spec.selector", "", []bool{false, true}},
		{"matchLabels and matchExpressions", map[string]any{"matchLabels": map[string]any{"app": "web"}, "matchExpressions": []any{map[string]any{"key": "tier", "operator": "In", "values": []any{"back"}}}}, "spec.selector", "", []bool{false, false}},
		{"invalid operator", map[string]any{"matchExpressions": []any{map[string]any{"key": "app", "operator": "Near"}}}, "spec.selector", "", nil},
		{"invalid matchLabels type", map[string]any{"matchLabels": "app=web"}, "spec.selector", "", nil},
		{"not a map", "app=web", "spec.selector", "", nil},
		// Empty selectors: the conventions when emptySelector is unset ...
		{"empty map in 'selector' (Service)", map[string]any{}, "spec.selector", "", nil},
		{"empty map in '*Selector' (NetworkPolicy)", map[string]any{}, "spec.podSelector", "", []bool{true, true}},
		{"empty matchLabels in 'selector' (LabelSelector)", map[string]any{"matchLabels": map[string]any{}}, "spec.selector", "", []bool{true, true}},
		{"empty matchExpressions", map[string]any{"matchExpressions": []any{}}, "spec.selector", "", []bool{true, true}},
		// ... and the explicit option, which overrides them (PodDisruptionBudget: {} selects all).
		{"empty map, emptySelector all", map[string]any{}, "spec.selector", linter.EmptySelectorAll, []bool{true, true}},
		{"empty map, emptySelector none", map[string]any{}, "spec.podSelector", linter.EmptySelectorNone, nil},
		{"empty matchLabels, emptySelector none", map[string]any{"matchLabels": map[string]any{}}, "spec.selector", linter.EmptySelectorNone, nil},
		{"non-empty selector ignores emptySelector", map[string]any{"app": "web"}, "spec.selector", linter.EmptySelectorAll, []bool{true, false}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sels := compileSelectors(obj(tc.sel), tc.path, nil, tc.empty)
			if tc.want == nil {
				if len(sels) != 0 {
					t.Fatalf("got %d selectors, want none", len(sels))
				}
				return
			}
			if len(sels) != 1 {
				t.Fatalf("got %d selectors, want 1", len(sels))
			}
			if got := sels[0].Matches(web); got != tc.want[0] {
				t.Errorf("matches %v = %v, want %v", web, got, tc.want[0])
			}
			if got := sels[0].Matches(none); got != tc.want[1] {
				t.Errorf("matches {} = %v, want %v", got, tc.want[1])
			}
		})
	}

	// The "*Selector" convention follows the field name, however the path addresses it.
	np := map[string]any{"spec": map[string]any{"podSelector": map[string]any{}, "selectors": []any{map[string]any{}}}}
	if sels := compileSelectors(np, "spec['podSelector']", nil, ""); len(sels) != 1 || !sels[0].Matches(none) {
		t.Errorf("quoted podSelector {}: got %v, want a selector of everything", sels)
	}
	if sels := compileSelectors(np, "spec.selectors[0]", nil, ""); len(sels) != 0 {
		t.Errorf("selectors[0] {}: got %v, want none", sels)
	}

	// A missing selector selects nothing.
	if sels := compileSelectors(map[string]any{}, "spec.selector", nil, linter.EmptySelectorAll); len(sels) != 0 {
		t.Errorf("missing selector: got %d selectors, want none", len(sels))
	}
	// [*] collects every element's selector; [@] only the bound one.
	refs := map[string]any{"spec": map[string]any{"refs": []any{
		map[string]any{"labelSelector": map[string]any{"matchLabels": map[string]any{"a": "1"}}},
		map[string]any{"labelSelector": map[string]any{"matchLabels": map[string]any{"b": "2"}}},
	}}}
	if n := len(compileSelectors(refs, "spec.refs[*].labelSelector", nil, "")); n != 2 {
		t.Errorf("[*]: got %d selectors, want 2", n)
	}
	sels := compileSelectors(refs, "spec.refs[@].labelSelector", map[string]int{"spec.refs[@]": 1}, "")
	if len(sels) != 1 || !sels[0].Matches(labels.Set{"b": "2"}) || sels[0].Matches(labels.Set{"a": "1"}) {
		t.Errorf("[@] bound to 1: got %v", sels)
	}
}

// emptySelector end to end: a PodDisruptionBudget's empty selector selects every Pod of its
// namespace only when the Pattern says so.
func TestEmptySelectorOption(t *testing.T) {
	pattern := func(option string) string {
		return header + `  target: {kind: PodDisruptionBudget, apiVersion: policy/v1, plural: poddisruptionbudgets}
  dependencies:
    - {id: pod, kind: Pod, apiVersion: v1, plural: pods}
  relationships:
    matchNone:
      - with: pod
        type: selects
` + option + `        criteria:
          - {targetPath: metadata.namespace, dependencyPath: metadata.namespace, operator: EQUALS}
`
	}
	objects := `
apiVersion: v1
kind: Pod
metadata: {name: web-1, namespace: a, labels: {app: web}}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: all-pods, namespace: a}
spec: {selector: {}}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: web, namespace: a}
spec: {selector: {matchLabels: {app: web}}}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: empty-ns, namespace: b}
spec: {selector: {}}
`
	// Unset: an empty map in a field named "selector" selects nothing.
	expectSmells(t, run(t, pattern(""), objects, nil), "all-pods", "empty-ns")
	// all: the PDB semantics; empty-ns still has no Pod in its namespace.
	expectSmells(t, run(t, pattern("        emptySelector: all\n"), objects, nil), "empty-ns")
	// none on a LabelSelector-shaped empty selector.
	expectSmells(t, run(t, pattern("        emptySelector: none\n"), strings.ReplaceAll(objects, "spec: {selector: {}}", "spec: {selector: {matchLabels: {}}}"), nil), "all-pods", "empty-ns")
}
