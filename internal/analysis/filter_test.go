package analysis

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	kubepatternv1 "kubepattern-go/api/v1"
)

// createMockPod returns a mock Unstructured object for testing purposes.
func createMockPod() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      "test-pod",
				"namespace": "default",
				"labels": map[string]any{
					"app":  "frontend",
					"tier": "web",
				},
				"annotations": map[string]any{
					"empty-annotation": "",
				},
			},
			"spec": map[string]any{
				"replicas":           3,
				"serviceAccountName": "", // Used to test IS_EMPTY
				"containers": []any{
					map[string]any{
						"name":  "nginx",
						"image": "nginx:1.24",
						"ports": []any{
							map[string]any{"containerPort": 80},
							map[string]any{"containerPort": 443},
						},
					},
					map[string]any{
						"name":  "sidecar",
						"image": "fluentd:v1",
					},
				},
			},
		},
	}
}

func TestEvalCondition(t *testing.T) {
	obj := createMockPod()

	tests := []struct {
		name     string
		cond     kubepatternv1.FilterCondition
		expected bool
	}{
		// --- Operator: EXISTS ---
		{
			name:     "EXISTS - field exists",
			cond:     kubepatternv1.FilterCondition{Path: "metadata.name", Operator: kubepatternv1.FilterExists},
			expected: true,
		},
		{
			name:     "EXISTS - field does not exist",
			cond:     kubepatternv1.FilterCondition{Path: "metadata.creationTimestamp", Operator: kubepatternv1.FilterExists},
			expected: false,
		},

		// --- Operator: IS_EMPTY ---
		{
			name:     "IS_EMPTY - field does not exist (considered empty)",
			cond:     kubepatternv1.FilterCondition{Path: "spec.nodeName", Operator: kubepatternv1.FilterIsEmpty},
			expected: true,
		},
		{
			name:     "IS_EMPTY - field exists but string is empty",
			cond:     kubepatternv1.FilterCondition{Path: "spec.serviceAccountName", Operator: kubepatternv1.FilterIsEmpty},
			expected: true,
		},
		{
			name:     "IS_EMPTY - field exists and has a value",
			cond:     kubepatternv1.FilterCondition{Path: "metadata.name", Operator: kubepatternv1.FilterIsEmpty},
			expected: false,
		},

		// --- Operator: EQUALS ---
		{
			name:     "EQUALS - exact string match",
			cond:     kubepatternv1.FilterCondition{Path: "metadata.labels.app", Operator: kubepatternv1.FilterEquals, Values: []string{"frontend"}},
			expected: true,
		},
		{
			name:     "EQUALS - string mismatch",
			cond:     kubepatternv1.FilterCondition{Path: "metadata.labels.app", Operator: kubepatternv1.FilterEquals, Values: []string{"backend"}},
			expected: false,
		},
		{
			name:     "EQUALS - array match with wildcard (finds 'sidecar')",
			cond:     kubepatternv1.FilterCondition{Path: "spec.containers[*].name", Operator: kubepatternv1.FilterEquals, Values: []string{"sidecar"}},
			expected: true,
		},

		// --- Numeric Operators ---
		{
			name:     "GREATER_THAN - match",
			cond:     kubepatternv1.FilterCondition{Path: "spec.replicas", Operator: kubepatternv1.FilterGreaterThan, Values: []string{"2"}},
			expected: true,
		},
		{
			name:     "LESS_THAN - match",
			cond:     kubepatternv1.FilterCondition{Path: "spec.replicas", Operator: kubepatternv1.FilterLessThan, Values: []string{"5"}},
			expected: true,
		},

		// --- Array Size Operators ---
		{
			name:     "ARRAY_SIZE_EQUALS - array has 2 elements",
			cond:     kubepatternv1.FilterCondition{Path: "spec.containers", Operator: kubepatternv1.FilterArraySizeEquals, Values: []string{"2"}},
			expected: true,
		},
		{
			name:     "ARRAY_SIZE_GREATER_THAN - array has more than 1 element",
			cond:     kubepatternv1.FilterCondition{Path: "spec.containers", Operator: kubepatternv1.FilterArraySizeGreaterThan, Values: []string{"1"}},
			expected: true,
		},
		{
			name:     "ARRAY_SIZE_EQUALS - fail, array does not have 3 elements",
			cond:     kubepatternv1.FilterCondition{Path: "spec.containers", Operator: kubepatternv1.FilterArraySizeEquals, Values: []string{"3"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evalCondition(obj, tt.cond)
			if result != tt.expected {
				t.Errorf("evalCondition() failed for %q: expected %v, got %v", tt.name, tt.expected, result)
			}
		})
	}
}

// TestGetFieldValues tests value extraction, specifically the [*] wildcard behavior.
func TestGetFieldValues(t *testing.T) {
	obj := createMockPod()

	tests := []struct {
		name          string
		path          string
		expectedFound bool
		expectedLen   int
		expectedVals  []any
	}{
		{
			name:          "Simple path",
			path:          "metadata.name",
			expectedFound: true,
			expectedLen:   1,
			expectedVals:  []any{"test-pod"},
		},
		{
			name:          "Non-existent path",
			path:          "spec.nodeName",
			expectedFound: false,
			expectedLen:   0,
			expectedVals:  nil,
		},
		{
			name:          "Path with wildcard (container names)",
			path:          "spec.containers[*].name",
			expectedFound: true,
			expectedLen:   2,
			expectedVals:  []any{"nginx", "sidecar"},
		},
		{
			name:          "Path with double wildcard (container ports)",
			path:          "spec.containers[*].ports[*].containerPort",
			expectedFound: true,
			expectedLen:   2,
			expectedVals:  []any{80, 443}, // Matches ports of the nginx container
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vals, found := getFieldValues(obj.Object, tt.path)

			if found != tt.expectedFound {
				t.Errorf("getFieldValues() found = %v, expected %v", found, tt.expectedFound)
			}

			if len(vals) != tt.expectedLen {
				t.Errorf("getFieldValues() length = %d, expected %d", len(vals), tt.expectedLen)
			}

			if tt.expectedLen > 0 {
				for i, v := range tt.expectedVals {
					if reflect.TypeOf(v) != reflect.TypeOf(vals[i]) {
						t.Logf("Note: type mismatch at index %d: expected %T, got %T", i, v, vals[i])
					}
				}
			}
		})
	}
}

func TestFilterResources(t *testing.T) {
	pod1 := createMockPod() // Kind: Pod, Name: test-pod

	pod2 := createMockPod()
	pod2.Object["metadata"].(map[string]any)["name"] = "another-pod"

	svc1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]any{"name": "my-svc"},
		},
	}

	nodes := map[types.UID]*unstructured.Unstructured{
		"uid-pod1": pod1,
		"uid-pod2": pod2,
		"uid-svc1": svc1,
	}

	tests := []struct {
		name       string
		kind       string
		apiVersion string
		filters    kubepatternv1.Filters
		wantCount  int
	}{
		{
			name:       "Filter only by Kind (Pod) - finds 2",
			kind:       "Pod",
			apiVersion: "v1",
			filters:    kubepatternv1.Filters{},
			wantCount:  2,
		},
		{
			name:       "Filter by different Kind (Service) - finds 1",
			kind:       "Service",
			apiVersion: "v1",
			filters:    kubepatternv1.Filters{},
			wantCount:  1,
		},
		{
			name:       "MatchAll - find only one pod1",
			kind:       "Pod",
			apiVersion: "v1",
			filters: kubepatternv1.Filters{
				MatchAll: []kubepatternv1.FilterCondition{
					{Path: "metadata.name", Operator: kubepatternv1.FilterEquals, Values: []string{"test-pod"}},
				},
			},
			wantCount: 1,
		},
		{
			name:       "MatchNone - exclude pod1, find pod2",
			kind:       "Pod",
			apiVersion: "v1",
			filters: kubepatternv1.Filters{
				MatchNone: []kubepatternv1.FilterCondition{
					{Path: "metadata.name", Operator: kubepatternv1.FilterEquals, Values: []string{"test-pod"}},
				},
			},
			wantCount: 1,
		},
		{
			name:       "MatchAny - one true and one false condition - finds pod1",
			kind:       "Pod",
			apiVersion: "v1",
			filters: kubepatternv1.Filters{
				MatchAny: []kubepatternv1.FilterCondition{
					{Path: "metadata.name", Operator: kubepatternv1.FilterEquals, Values: []string{"test-pod"}},
					{Path: "metadata.name", Operator: kubepatternv1.FilterEquals, Values: []string{"does-not-exist"}},
				},
			},
			wantCount: 1,
		},
		{
			name:       "MatchAny - all false conditions - finds 0",
			kind:       "Pod",
			apiVersion: "v1",
			filters: kubepatternv1.Filters{
				MatchAny: []kubepatternv1.FilterCondition{
					{Path: "metadata.name", Operator: kubepatternv1.FilterEquals, Values: []string{"foo"}},
					{Path: "metadata.name", Operator: kubepatternv1.FilterEquals, Values: []string{"bar"}},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterResources(nodes, tt.kind, tt.apiVersion, tt.filters)
			if len(got) != tt.wantCount {
				t.Errorf("FilterResources() = %d results, expected %d", len(got), tt.wantCount)
			}
		})
	}
}

// TestCompareErrors validates the behavior of comparison methods when provided with invalid or unexpected inputs.
func TestCompareErrors(t *testing.T) {
	res := compareNumeric([]any{3}, []string{"abc"}, func(a, b int) bool { return a > b })
	if res != false {
		t.Errorf("compareNumeric with invalid target should have returned false")
	}

	obj := map[string]any{"items": []any{1, 2}}
	res = compareArraySize(obj, "items", []string{"xyz"}, func(a, b int) bool { return a > b })
	if res != false {
		t.Errorf("compareArraySize with invalid target should have returned false")
	}

	obj2 := map[string]any{"not-an-array": "hello"}
	res = compareArraySize(obj2, "not-an-array", []string{"1"}, func(a, b int) bool { return a > b })
	if res != false {
		t.Errorf("compareArraySize on non-array field should have returned false")
	}
}

// TestMoreOperators validates the functionality of FilterGreaterOrEqual and FilterLessOrEqual on the mocked pod object.
func TestMoreOperators(t *testing.T) {
	obj := createMockPod()

	condGreaterOrEqual := kubepatternv1.FilterCondition{Path: "spec.replicas", Operator: kubepatternv1.FilterGreaterOrEqual, Values: []string{"3"}}
	if !evalCondition(obj, condGreaterOrEqual) {
		t.Errorf("FilterGreaterOrEqual failed (3 >= 3 should be true)")
	}

	condLessOrEqual := kubepatternv1.FilterCondition{Path: "spec.replicas", Operator: kubepatternv1.FilterLessOrEqual, Values: []string{"3"}}
	if !evalCondition(obj, condLessOrEqual) {
		t.Errorf("FilterLessOrEqual failed (3 <= 3 should be true)")
	}
}
