package linter

import (
	"strings"
	"testing"
)

const head = `apiVersion: kubepattern.dev/v1
kind: Pattern
metadata: {name: p}
spec:
  displayName: P
  category: Test
  severity: LOW
  message: m
`

func TestLintDSLExtensions(t *testing.T) {
	dep := `  dependencies:
    - {id: d, kind: Pod, apiVersion: v1, plural: pods}
`
	target := `  target: {kind: Service, apiVersion: v1, plural: services}
`
	rel := func(body string) string {
		return head + target + dep + "  relationships:\n    matchNone:\n      - with: d\n" + body
	}
	tests := []struct {
		name, yaml, err string
	}{
		{"element-scoped criteria", rel(`        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.items[@].name", operator: EQUALS}
          - {targetPath: kind, dependencyPath: "spec.items[@].kind", operator: EQUALS}
`), ""},
		{"defaults and transforms", rel(`        type: custom
        criteria:
          - targetPath: apiVersion
            targetTransform: {apiGroup: true}
            dependencyPath: spec.ref.group
            dependencyDefault: {value: ""}
            operator: EQUALS
          - targetPath: metadata.namespace
            dependencyPath: spec.ref.namespace
            dependencyDefault: {path: metadata.namespace}
            dependencyTransform: {regex: "^(.+)$"}
            operator: EQUALS
          - targetPath: metadata.name
            dependencyPath: "spec.items[*].type"
            dependencyTransform: {split: {separator: "@", index: -1}}
            operator: EQUALS
`), ""},
		{"selectedBy with selector path and criteria", rel(`        type: selectedBy
        selectorPath: "spec.refs[@].labelSelector"
        criteria:
          - {targetPath: kind, dependencyPath: "spec.refs[@].kind", operator: EQUALS}
`), ""},
		{"quoted keys", rel(`        type: custom
        criteria:
          - {targetPath: "metadata.labels['app.kubernetes.io/name']", dependencyPath: metadata.name, operator: EQUALS}
`), ""},
		{"wildcard before anchor", rel(`        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.a[*].b[@].c", operator: EQUALS}
`), "'[*]' cannot precede '[@]'"},
		{"invalid path", rel(`        type: custom
        criteria:
          - {targetPath: "metadata..name", dependencyPath: metadata.name, operator: EQUALS}
`), "empty segment"},
		{"two transforms", rel(`        type: custom
        criteria:
          - {targetPath: apiVersion, targetTransform: {apiGroup: true, lowercase: true}, dependencyPath: x, operator: EQUALS}
`), "exactly one of apiGroup, split, regex or lowercase"},
		{"regex without capture group", rel(`        type: custom
        criteria:
          - {targetPath: apiVersion, targetTransform: {regex: "^v1$"}, dependencyPath: x, operator: EQUALS}
`), "exactly one capture group"},
		{"default with path and value", rel(`        type: custom
        criteria:
          - {targetPath: a, dependencyPath: b, dependencyDefault: {path: c, value: d}, operator: EQUALS}
`), "exactly one of path or value"},
		{"empty split separator", rel(`        type: custom
        criteria:
          - {targetPath: a, dependencyPath: b, dependencyTransform: {split: {separator: "", index: 0}}, operator: EQUALS}
`), "split.separator is empty"},
		{"selectorPath on custom", rel(`        type: custom
        selectorPath: spec.selector
        criteria:
          - {targetPath: a, dependencyPath: b, operator: EQUALS}
`), "only valid for types selects and selectedBy"},
		{"anchor in dependency filter", head + target + `  dependencies:
    - id: d
      kind: Pod
      apiVersion: v1
      plural: pods
      filters:
        matchAll:
          - {path: "spec.containers[@].name", operator: EXISTS}
`, "'[@]' is not allowed"},
		{"anchor in target filter", head + `  target:
    kind: Service
    apiVersion: v1
    plural: services
    filters:
      matchAll:
        - {path: "spec.ports[@].name", operator: EQUALS, valuesFrom: metadata.name}
`, ""},
		{"valuesFrom with values", head + `  target:
    kind: Service
    apiVersion: v1
    plural: services
    filters:
      matchAll:
        - {path: a, operator: EQUALS, values: [x], valuesFrom: b}
`, "either values or valuesFrom"},
		{"valuesFrom with another operator", head + `  target:
    kind: Service
    apiVersion: v1
    plural: services
    filters:
      matchAll:
        - {path: a, operator: GREATER_THAN, valuesFrom: b}
`, "valuesFrom is supported only with operator EQUALS"},
		{"kind wildcard", head + `  target: {kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}
`, ""},
		{"kind wildcard with plural", head + `  target: {kind: "*", apiVersion: v1, plural: pods}
`, "plural must be empty when kind is '*'"},
		{"category", head + `  target: {category: managed}
`, ""},
		{"category and kind", head + `  target: {category: managed, kind: Pod}
`, "only one of kind, category or kinds"},
		{"kinds list", head + `  target:
    kinds:
      - {kind: Markdown, apiVersion: widgets.templates.krateo.io/v1beta1, plural: markdowns}
      - {kind: "*", apiVersion: templates.krateo.io/v1}
      - {category: krateo}
`, ""},
		{"kinds entry without plural", head + `  target:
    kinds:
      - {kind: Markdown, apiVersion: widgets.templates.krateo.io/v1beta1}
`, "kinds[0].pluralName is empty"},
		{"fromTarget", head + target + `  dependencies:
    - id: d
      fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind, pluralPath: status.resource}
`, ""},
		{"fromTarget without kindPath", head + target + `  dependencies:
    - id: d
      fromTarget: {apiVersionPath: status.apiVersion}
`, "needs kindPath and exactly one of apiVersionPath or apiVersion"},
		{"fromTarget with a fixed apiVersion", head + target + `  dependencies:
    - id: d
      fromTarget: {apiVersion: constraints.gatekeeper.sh/v1beta1, kindPath: spec.crd.spec.names.kind}
`, ""},
		{"fromTarget with kind", head + target + `  dependencies:
    - id: d
      kind: Pod
      fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind}
`, "cannot be combined"},
		{"literal side", rel(`        type: custom
        criteria:
          - {targetValue: pages, dependencyPath: "spec.items[@].resource", operator: EQUALS}
`), ""},
		{"path and literal", rel(`        type: custom
        criteria:
          - {targetPath: a, targetValue: pages, dependencyPath: b, operator: EQUALS}
`), "either targetPath or targetValue"},
		{"two literals", rel(`        type: custom
        criteria:
          - {targetValue: a, dependencyValue: b, operator: EQUALS}
`), "compares two literals"},
		{"missing target", head, "spec.target.kind is empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Lint([]byte(tc.yaml))
			if tc.err == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error containing %q, got %v", tc.err, err)
			}
		})
	}
}

func TestApplyTransform(t *testing.T) {
	cases := []struct {
		tr       *Transform
		in, out  string
		accepted bool
	}{
		{nil, "x", "x", true},
		{&Transform{APIGroup: true}, "cert-manager.io/v1", "cert-manager.io", true},
		{&Transform{APIGroup: true}, "v1", "", true},
		{&Transform{Split: &SplitTransform{Separator: "@", Index: 0}}, "def@v1", "def", true},
		{&Transform{Split: &SplitTransform{Separator: "/", Index: -1}}, "a/b/c", "c", true},
		{&Transform{Split: &SplitTransform{Separator: "/", Index: 5}}, "a/b", "", false},
		{&Transform{Regex: "name=([^&]+)"}, "/call?name=ra&ns=a", "ra", true},
		{&Transform{Regex: "name=([^&]+)"}, "/call", "", false},
		{&Transform{Lowercase: true}, "MixedCase", "mixedcase", true},
	}
	for _, c := range cases {
		got, ok := ApplyTransform(c.tr, c.in)
		if got != c.out || ok != c.accepted {
			t.Errorf("ApplyTransform(%+v, %q) = %q, %v; want %q, %v", c.tr, c.in, got, ok, c.out, c.accepted)
		}
	}
}

// The anchors of each side must form one nested chain, a default needs a per-element path,
// and emptySelector is a selector option.
func TestLintAnchorChainsDefaultsAndEmptySelector(t *testing.T) {
	targetWith := func(filters string) string {
		return head + "  target:\n    kind: Service\n    apiVersion: v1\n    plural: services\n" + filters
	}
	deps := `  dependencies:
    - {id: d, kind: Pod, apiVersion: v1, plural: pods}
    - {id: e, kind: Pod, apiVersion: v1, plural: pods}
`
	rels := func(body string) string { return "  relationships:\n    matchNone:\n" + body }
	tests := []struct {
		name, yaml, err string
	}{
		{"target anchors on two arrays (filter and criterion)", targetWith(`    filters:
      matchAll:
        - {path: "spec.ports[@].name", operator: EXISTS}
`) + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: "spec.ips[@]", dependencyPath: status.podIP, operator: EQUALS}
`), "the target paths anchor two independent arrays, 'spec.ips[@]' and 'spec.ports[@]'"},
		{"target anchors on two arrays (two relationships)", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: "spec.a[@].name", dependencyPath: metadata.name, operator: EQUALS}
      - with: e
        type: custom
        criteria:
          - {targetPath: "spec.b[@].name", dependencyPath: metadata.name, operator: EQUALS}
`), "the target paths anchor two independent arrays"},
		{"target anchors on two elements of the same parent", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: "spec.a[0].b[@]", dependencyPath: metadata.name, operator: EQUALS}
          - {targetPath: "spec.a[1].b[@]", dependencyPath: metadata.name, operator: EQUALS}
`), "independent arrays"},
		{"target anchors nested in one chain", targetWith(`    filters:
      matchAll:
        - {path: "spec.rules[@].kind", operator: EQUALS, values: [x]}
`) + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: "spec.rules[@].refs[@].name", dependencyPath: metadata.name, operator: EQUALS}
          - {targetPath: "spec.rules[@].namespace", targetDefault: {path: metadata.namespace}, dependencyPath: metadata.namespace, operator: EQUALS}
`), ""},
		{"dependency anchors on two arrays", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.data[@].name", operator: EQUALS}
          - {targetPath: kind, dependencyPath: "spec.dataFrom[@].kind", operator: EQUALS}
`), "spec.relationships.matchNone[0]: the dependency paths anchor two independent arrays, 'spec.data[@]' and 'spec.dataFrom[@]'"},
		{"dependency anchors in a selector and a criterion", targetWith("") + deps + rels(`      - with: d
        type: selectedBy
        selectorPath: "spec.refs[@].labelSelector"
        criteria:
          - {targetPath: kind, dependencyPath: "spec.other[@].kind", operator: EQUALS}
`), "the dependency paths anchor two independent arrays"},
		{"dependency anchors through a default path", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.data[@].name", dependencyDefault: {path: "spec.fallback[@].name"}, operator: EQUALS}
`), "independent arrays"},
		{"each relationship has its own dependency anchors", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.data[@].name", operator: EQUALS}
      - with: d
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "spec.dataFrom[@].name", operator: EQUALS}
`), ""},
		{"target and dependency anchors are independent", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: "spec.a[@].name", dependencyPath: "spec.b[@].name", operator: EQUALS}
`), ""},
		{"dependency default with [*]", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: kind, dependencyPath: "spec.data[*].kind", dependencyDefault: {value: SecretStore}, operator: EQUALS}
`), "dependencyDefault cannot be used with '[*]' in dependencyPath 'spec.data[*].kind'"},
		{"target default with [*]", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: "spec.refs[*].namespace", targetDefault: {path: metadata.namespace}, dependencyPath: metadata.namespace, operator: EQUALS}
`), "targetDefault cannot be used with '[*]'"},
		{"default with [@]", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: kind, dependencyPath: "spec.data[@].kind", dependencyDefault: {value: SecretStore}, operator: EQUALS}
`), ""},
		{"[*] without default", targetWith("") + deps + rels(`      - with: d
        type: custom
        criteria:
          - {targetPath: kind, dependencyPath: "spec.data[*].kind", operator: EQUALS}
`), ""},
		{"emptySelector all", targetWith("") + deps + rels(`      - with: d
        type: selects
        emptySelector: all
`), ""},
		{"emptySelector none on selectedBy", targetWith("") + deps + rels(`      - with: d
        type: selectedBy
        emptySelector: none
`), ""},
		{"emptySelector invalid", targetWith("") + deps + rels(`      - with: d
        type: selects
        emptySelector: some
`), "emptySelector 'some' is not valid"},
		{"emptySelector on custom", targetWith("") + deps + rels(`      - with: d
        type: custom
        emptySelector: all
        criteria:
          - {targetPath: kind, dependencyPath: kind, operator: EQUALS}
`), "only valid for types selects and selectedBy"},
		{"emptySelector on owns", targetWith("") + deps + rels(`      - with: d
        type: owns
        emptySelector: none
`), "only valid for types selects and selectedBy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Lint([]byte(tc.yaml))
			if tc.err == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error containing %q, got %v", tc.err, err)
			}
		})
	}
}

// When the target is examined element by element, every relationship must read the element;
// fromTarget paths correlate with "[@]" and reject "[*]"; a default path rejects "[*]".
func TestLintElementReadingAndFromTarget(t *testing.T) {
	anchoredTarget := head + `  target:
    kind: RoleBinding
    apiVersion: rbac.authorization.k8s.io/v1
    plural: rolebindings
    filters:
      matchAll:
        - {path: "subjects[@].kind", operator: EQUALS, values: [ServiceAccount]}
  dependencies:
    - {id: sa, kind: ServiceAccount, apiVersion: v1, plural: serviceaccounts}
    - {id: owner, kind: Deployment, apiVersion: apps/v1, plural: deployments}
  relationships:
    matchNone:
      - with: sa
        type: custom
        criteria:
          - {targetPath: "subjects[@].name", dependencyPath: metadata.name, operator: EQUALS}
`
	nested := head + `  target: {kind: Pod, apiVersion: v1, plural: pods}
  dependencies:
    - {id: s, kind: Secret, apiVersion: v1, plural: secrets}
  relationships:
    matchNone:
      - with: s
        type: custom
        criteria:
          - {targetPath: "spec.containers[@].env[@].valueFrom.secretKeyRef.name", dependencyPath: metadata.name, operator: EQUALS}
`
	fromTarget := func(ft string) string {
		return head + `  target: {kind: CompositionDefinition, apiVersion: core.krateo.io/v1alpha1, plural: compositiondefinitions}
  dependencies:
    - id: d
      fromTarget: ` + ft + "\n"
	}
	tests := []struct {
		name, yaml, err string
	}{
		{"every relationship reads the element", anchoredTarget, ""},
		{"a relationship ignores the element", anchoredTarget + `      - with: sa
        type: custom
        criteria:
          - {targetPath: metadata.namespace, dependencyPath: metadata.namespace, operator: EQUALS}
`, "spec.relationships.matchNone[1] does not read the target element 'subjects[@]'"},
		{"owns in an element-scoped pattern", anchoredTarget + `      - with: owner
        type: ownedBy
`, "does not read the target element"},
		{"a relationship reads only the outer element", nested + `      - with: s
        type: custom
        criteria:
          - {targetPath: "spec.containers[@].image", dependencyPath: metadata.name, operator: EQUALS}
`, "does not read the target element 'spec.containers[@].env[@]'"},
		{"a relationship reads the element through its default", anchoredTarget + `      - with: sa
        type: custom
        criteria:
          - {targetPath: metadata.labels.x, targetDefault: {path: "subjects[@].namespace"}, dependencyPath: metadata.namespace, operator: EQUALS}
`, ""},
		{"fromTarget with [@]", fromTarget(`{apiVersionPath: "spec.gen[@].apiVersion", kindPath: "spec.gen[@].kind", pluralPath: "spec.gen[@].plural"}`), ""},
		{"fromTarget with [*]", fromTarget(`{apiVersionPath: "spec.gen[*].apiVersion", kindPath: "spec.gen[*].kind"}`), "'[*]' is not allowed in path 'spec.gen[*].apiVersion'"},
		{"fromTarget on two arrays", fromTarget(`{apiVersionPath: "spec.a[@].apiVersion", kindPath: "spec.b[@].kind"}`), "fromTarget: the paths anchor two independent arrays"},
		{"default path with [*]", head + `  target: {kind: Service, apiVersion: v1, plural: services}
  dependencies:
    - {id: d, kind: Pod, apiVersion: v1, plural: pods}
  relationships:
    matchNone:
      - with: d
        type: custom
        criteria:
          - {targetPath: spec.ns, targetDefault: {path: "spec.items[*].ns"}, dependencyPath: metadata.namespace, operator: EQUALS}
`, "targetDefault.path 'spec.items[*].ns' cannot contain '[*]'"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Lint([]byte(tc.yaml))
			if tc.err == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error containing %q, got %v", tc.err, err)
			}
		})
	}
}
