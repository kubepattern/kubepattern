package analysis

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kubepatternv1 "kubepattern-go/api/v1"
)

const (
	testAppsAPIVersion = "apps/v1"
	testKindRS         = "ReplicaSet"
	testRSDepName      = "my-rs-dep"
)

func TestEvaluateRelationships(t *testing.T) {
	// 1. Set up mock resources
	podObj := &unstructured.Unstructured{
		Object: map[string]any{
			testFieldAPIVersion: testAPIVersion,
			testFieldKind:       testKindPod,
			testFieldMeta: map[string]any{
				testFieldName: "my-dep-b458fd6bb4-twq66",
				"ownerReferences": []any{
					map[string]any{
						testFieldAPIVersion: testAppsAPIVersion,
						testFieldKind:       testKindRS,
						testFieldName:       "my-dep-b458fd6bb4",
						"uid":               "fake-uid-1234",
					},
				},
			},
		},
	}

	validRS := &unstructured.Unstructured{
		Object: map[string]any{
			testFieldAPIVersion: testAppsAPIVersion,
			testFieldKind:       testKindRS,
			testFieldMeta: map[string]any{
				testFieldName: "my-dep-b458fd6bb4",
			},
		},
	}

	invalidRS := &unstructured.Unstructured{
		Object: map[string]any{
			testFieldAPIVersion: testAppsAPIVersion,
			testFieldKind:       testKindRS,
			testFieldMeta: map[string]any{
				testFieldName: "another-rs-9999",
			},
		},
	}

	// 2. Set up the relationship rule (Custom EQUALS on ownerReferences)
	customRelRule := kubepatternv1.Relationship{
		With: testRSDepName,
		Type: kubepatternv1.RelationshipCustom,
		Criteria: []kubepatternv1.Criteria{
			{
				TargetPath:     "metadata.ownerReferences[*].name",
				DependencyPath: pathMetadataName,
				Operator:       kubepatternv1.CriteriaEquals,
			},
		},
	}

	// 3. Define test cases using a table-driven approach
	tests := []struct {
		name          string
		target        *unstructured.Unstructured
		deps          map[string][]*unstructured.Unstructured
		relationships kubepatternv1.Relationships
		want          bool
	}{
		{
			name:   "MatchAll - Success with valid ReplicaSet",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				testRSDepName: {validRS},
			},
			relationships: kubepatternv1.Relationships{
				MatchAll: []kubepatternv1.Relationship{customRelRule},
			},
			want: true,
		},
		{
			name:   "MatchAll - Failure with invalid ReplicaSet",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				testRSDepName: {invalidRS},
			},
			relationships: kubepatternv1.Relationships{
				MatchAll: []kubepatternv1.Relationship{customRelRule},
			},
			want: false,
		},
		{
			name:   "MatchNone - Success when ReplicaSet does NOT match",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				testRSDepName: {invalidRS},
			},
			relationships: kubepatternv1.Relationships{
				MatchNone: []kubepatternv1.Relationship{customRelRule},
			},
			want: true, // It wants NONE to match, and indeed it doesn't, so relationships are satisfied
		},
		{
			name:   "MatchNone - Failure when ReplicaSet matches",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				testRSDepName: {validRS},
			},
			relationships: kubepatternv1.Relationships{
				MatchNone: []kubepatternv1.Relationship{customRelRule},
			},
			want: false, // It wants NONE to match, but it DOES match, so relationships fail
		},
		{
			name:   "Empty dependencies array - Should fail for MatchAll",
			target: podObj,
			deps: map[string][]*unstructured.Unstructured{
				testRSDepName: {}, // No candidates found in the cluster
			},
			relationships: kubepatternv1.Relationships{
				MatchAll: []kubepatternv1.Relationship{customRelRule},
			},
			want: false,
		},
	}

	// 4. Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRelationships(tt.target, tt.deps, tt.relationships, nil)
			if got != tt.want {
				t.Errorf("EvaluateRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}
