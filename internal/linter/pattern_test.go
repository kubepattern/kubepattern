package linter

import (
	"strings"
	"testing"
)

func TestLint(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectError   bool
		errorContains string
	}{
		// ---------------------------------------------------------
		// SUCCESS CASES
		// ---------------------------------------------------------
		{
			name: "Valid - Orphaned Krateo Page (custom relation)",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: krateo-page-not-referenced
  displayName: Orphaned Krateo Page
  category: ReferencesKrateo
  severity: MEDIUM
spec:
  message: "Page is orphaned."
  target:
    kind: Page
    apiVersion: krateo.io/v1
  dependencies:
    - id: navmenuitem
      kind: NavMenuItem
      apiVersion: krateo.io/v1
  relationships:
    matchNone:
      - with: navmenuitem
        type: custom
        criteria:
          - targetPath: metadata.name
            dependencyPath: spec.resourcesRefs.items[*].name
            operator: EQUALS
`,
			expectError: false,
		},
		{
			name: "Valid - Naked Pod (ownedBy relations)",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: naked-pod
  displayName: Naked Pod
  category: Architecture
  severity: HIGH
spec:
  message: "Pod is a Naked Pod."
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: replicaset
      kind: ReplicaSet
      apiVersion: apps/v1
    - id: statefulset
      kind: StatefulSet
      apiVersion: apps/v1
  relationships:
    matchNone:
      - with: replicaset
        type: ownedBy
      - with: statefulset
        type: ownedBy
`,
			expectError: false,
		},
		{
			name: "Valid - Dangling Service (selects relation and target filters)",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: dangling-service
  displayName: Dangling Service
  category: Networking
  severity: CRITICAL
spec:
  message: "Service does not route to any Pods."
  target:
    kind: Service
    apiVersion: v1
    filters:
      matchNone:
        - path: metadata.name
          operator: EQUALS
          values:
            - kubernetes
  dependencies:
    - id: target-pods
      kind: Pod
      apiVersion: v1
  relationships:
    matchNone:
      - with: target-pods
        type: selects
`,
			expectError: false,
		},

		// ---------------------------------------------------------
		// ERROR CASES (Bad Paths)
		// ---------------------------------------------------------
		{
			name:          "Error - Empty File",
			yamlContent:   ``,
			expectError:   true,
			errorContains: "yaml input is empty",
		},
		{
			name: "Error - Invalid APIVersion Format",
			yamlContent: `
apiVersion: invalid-version
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "is not a valid apiVersion. Expected format",
		},
		{
			name: "Error - Duplicate Dependency ID",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: dep1
      kind: Deployment
      apiVersion: apps/v1
    - id: dep1
      kind: Service
      apiVersion: v1
`,
			expectError:   true,
			errorContains: "spec.dependencies[1].id 'dep1' is not unique",
		},
		{
			name: "Error - Relationship referencing non-existent dependency",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  relationships:
    matchAll:
      - with: ghost-dependency
        type: ownedBy
`,
			expectError:   true,
			errorContains: "does not match any dependency id",
		},
		{
			name: "Error - Custom Relationship without criteria",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: dep1
      kind: ReplicaSet
      apiVersion: apps/v1
  relationships:
    matchAny:
      - with: dep1
        type: custom
`,
			expectError:   true,
			errorContains: "of type 'custom' must have at least one criteria",
		},
		{
			name: "Error - OwnedBy Relationship with unexpected criteria",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: dep1
      kind: ReplicaSet
      apiVersion: apps/v1
  relationships:
    matchAny:
      - with: dep1
        type: ownedBy
        criteria:
          - targetPath: metadata.name
            dependencyPath: metadata.name
            operator: EQUALS
`,
			expectError:   true,
			errorContains: "of type 'ownedBy' must not declare criteria",
		},
		{
			name: "Error - Filter EQUALS without values",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
    filters:
      matchAll:
        - path: metadata.namespace
          operator: EQUALS
`,
			expectError:   true,
			errorContains: "values is empty for operator EQUALS",
		}, // --- YAML UNMARSHAL ERROR ---
		{
			name:          "Error - Invalid YAML format",
			yamlContent:   "apiVersion: \t\ninvalid-\n- yaml-:::",
			expectError:   true,
			errorContains: "pattern definition is not valid yaml",
		},
		// --- ROOT & METADATA ERRORS ---
		{
			name: "Error - Empty APIVersion",
			yamlContent: `
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "apiVersion is empty",
		},
		{
			name: "Error - Empty Kind",
			yamlContent: `
apiVersion: kubepattern.dev/v1
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "kind is empty",
		},
		{
			name: "Error - Invalid Kind",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Deployment
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "kind must be 'PatternAsCode' or 'Pattern'",
		},
		{
			name: "Error - Metadata Empty Name",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "metadata.name is empty",
		},
		{
			name: "Error - Metadata Invalid Name",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: invalid_name_with_underscores!
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "metadata.name contains invalid characters",
		},
		{
			name: "Error - Metadata Empty DisplayName",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "metadata.displayName is empty",
		},
		{
			name: "Error - Metadata Empty Category",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "metadata.category is empty",
		},
		{
			name: "Error - Metadata Empty Severity",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "metadata.severity is empty",
		},
		{
			name: "Error - Metadata Invalid Severity",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: UNKNOWN
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "metadata.severity must be one of",
		},
		// --- SPEC ERRORS ---
		{
			name: "Error - Empty Message",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  target:
    kind: Pod
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "spec.message is empty",
		},
		// --- TARGET & DEPENDENCY ERRORS ---
		{
			name: "Error - Target Empty Kind",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    apiVersion: v1
`,
			expectError:   true,
			errorContains: "spec.target.kind is empty",
		},
		{
			name: "Error - Target Empty APIVersion",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
`,
			expectError:   true,
			errorContains: "spec.target.apiVersion is empty",
		},
		{
			name: "Error - Dependency Missing Fields",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - kind: Pod # Missing ID
      apiVersion: v1
`,
			expectError:   true,
			errorContains: "spec.dependencies[0].id is empty",
		},
		// --- FILTERS ERRORS ---
		{
			name: "Error - Filter Empty Path",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
    filters:
      matchAll:
        - operator: EXISTS
`,
			expectError:   true,
			errorContains: "path is empty",
		},
		{
			name: "Error - Filter Invalid Operator",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
    filters:
      matchAll:
        - path: metadata.name
          operator: MAGIC
`,
			expectError:   true,
			errorContains: "operator 'MAGIC' is not valid",
		},
		{
			name: "Error - Filter EXISTS with values (should be empty)",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
    filters:
      matchAll:
        - path: metadata.labels.env
          operator: EXISTS
          values:
            - "production"
`,
			expectError:   true,
			errorContains: "values should be empty for operator EXISTS",
		},
		// --- RELATIONSHIPS ERRORS ---
		{
			name: "Error - Relationship Empty With",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: dep1
      kind: ReplicaSet
      apiVersion: apps/v1
  relationships:
    matchAll:
      - type: ownedBy # Missing with
`,
			expectError:   true,
			errorContains: "with is empty",
		},
		{
			name: "Error - Criteria Empty TargetPath",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: dep1
      kind: ReplicaSet
      apiVersion: apps/v1
  relationships:
    matchAll:
      - with: dep1
        type: custom
        criteria:
          - dependencyPath: metadata.name
            operator: EQUALS
`,
			expectError:   true,
			errorContains: "targetPath is empty",
		},
		{
			name: "Error - Criteria Invalid Operator",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
  dependencies:
    - id: dep1
      kind: ReplicaSet
      apiVersion: apps/v1
  relationships:
    matchAll:
      - with: dep1
        type: custom
        criteria:
          - targetPath: metadata.name
            dependencyPath: metadata.name
            operator: RANDOM
`,
			expectError:   true,
			errorContains: "operator 'RANDOM' is not valid",
		},
		// --- VALID EXISTS FILTER (to cover the true path) ---
		{
			name: "Valid - EXISTS filter",
			yamlContent: `
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: exists-test
  displayName: Test
  category: Test
  severity: LOW
spec:
  message: "Test"
  target:
    kind: Pod
    apiVersion: v1
    filters:
      matchAll:
        - path: metadata.labels
          operator: EXISTS
`,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Lint([]byte(tc.yamlContent))

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error containing '%s', but got nil", tc.errorContains)
				}
				if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain '%s', got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, but got: %v", err)
				}
			}
		})
	}
}
