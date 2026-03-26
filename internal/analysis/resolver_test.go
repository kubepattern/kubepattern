package analysis

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubepattern-go/internal/linter" // Adjust if your module path is different
)

func TestEvaluateRelationships(t *testing.T) {
	// 1. Setup mock resources
	podObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name": "my-dep-b458fdbb4-twq66",
				"ownerReferences": []any{
					map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "ReplicaSet",
						"name":       "my-dep-b458fdbb4",
						"uid":        "fake-uid-1234",
					},
				},
			},
		},
	}

	validRS := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]any{
				"name": "my-dep-b458fdbb4",
			},
		},
	}

	invalidRS := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]any{
				"name": "another-rs-9999",
			},
		},
	}

	// 2. Setup the relationship rule (Custom EQUALS on ownerReferences)
	customRelRule := linter.Relationship{
		With: "my-rs-dep",
		Type: linter.RelationshipCustom,
		Criteria: []linter.Criteria{
			{
				TargetPath:     "metadata.ownerReferences[*].name",
				DependencyPath: "metadata.name",
				Operator:       linter.CriteriaEquals,
			},
		},
	}

	// 3. Define test cases using table-driven approach
	tests := []struct {
		name          string
		target        *unstructured.Unstructured
		deps          map[string][]*unstructured.Unstructured
		relationships linter.Relationships
		want          bool
	}{
		{
			name:   "MatchAll - Success with valid ReplicaSet",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				"my-rs-dep": {validRS},
			},
			relationships: linter.Relationships{
				MatchAll: []linter.Relationship{customRelRule},
			},
			want: true,
		},
		{
			name:   "MatchAll - Failure with invalid ReplicaSet",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				"my-rs-dep": {invalidRS},
			},
			relationships: linter.Relationships{
				MatchAll: []linter.Relationship{customRelRule},
			},
			want: false,
		},
		{
			name:   "MatchNone - Success when ReplicaSet does NOT match",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				"my-rs-dep": {invalidRS},
			},
			relationships: linter.Relationships{
				MatchNone: []linter.Relationship{customRelRule},
			},
			want: true, // It wants NONE to match, and indeed it doesn't, so relationships are satisfied
		},
		{
			name:   "MatchNone - Failure when ReplicaSet matches",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				"my-rs-dep": {validRS},
			},
			relationships: linter.Relationships{
				MatchNone: []linter.Relationship{customRelRule},
			},
			want: false, // It wants NONE to match, but it DOES match, so relationships fail
		},
		{
			name:   "Empty dependencies array - Should fail for MatchAll",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				"my-rs-dep": {}, // No candidates found in the cluster
			},
			relationships: linter.Relationships{
				MatchAll: []linter.Relationship{customRelRule},
			},
			want: false,
		},
	}

	// 4. Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRelationships(tt.target, tt.deps, tt.relationships)
			if got != tt.want {
				t.Errorf("EvaluateRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}
