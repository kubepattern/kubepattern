package cluster

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func node(name string, owners ...string) unstructured.Unstructured {
	u := unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "K",
		"metadata": map[string]any{"name": name, "uid": name}}}
	var refs []any
	for _, o := range owners {
		refs = append(refs, map[string]any{"apiVersion": "v1", "kind": "K", "name": o, "uid": o})
	}
	if refs != nil {
		u.Object["metadata"].(map[string]any)["ownerReferences"] = refs
	}
	return u
}

func TestIsParentOwner(t *testing.T) {
	g := NewGraph()
	g.Build([]unstructured.Unstructured{
		node("deploy"),
		node("rs", "deploy"),
		node("pod", "rs"),
		node("orphan"),
		node("multi", "orphan", "rs"),   // two owners: the second leads to deploy
		node("dangling", "not-fetched"), // owner not in the graph
		// An ownership cycle: a -> b -> c -> a.
		node("a", "c"),
		node("b", "a"),
		node("c", "b"),
		node("self", "self"),
	})
	tests := []struct {
		parent, child string
		want          bool
	}{
		{"rs", "pod", true},
		{"deploy", "pod", true}, // transitive
		{"pod", "deploy", false},
		{"deploy", "deploy", false},
		{"deploy", "multi", true},
		{"orphan", "multi", true},
		{"orphan", "pod", false},
		{"not-fetched", "dangling", true}, // a direct reference counts even if not fetched
		{"deploy", "dangling", false},
		{"deploy", "missing", false},
		{"a", "b", true},
		{"b", "a", true}, // through the cycle
		{"deploy", "a", false},
		{"self", "self", true},
		{"deploy", "self", false},
	}
	for _, tc := range tests {
		if got := g.IsParentOwner(types.UID(tc.parent), types.UID(tc.child)); got != tc.want {
			t.Errorf("IsParentOwner(%s, %s) = %v, want %v", tc.parent, tc.child, got, tc.want)
		}
	}
}
