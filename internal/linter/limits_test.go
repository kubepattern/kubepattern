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
`), "selectorPath is only valid"},
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
