package analysis

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/linter"
)

const benchPattern = `apiVersion: kubepattern.dev/v1
kind: Pattern
metadata: {name: flux}
spec:
  displayName: F
  category: Cost
  severity: LOW
  message: m
  target: {kind: OCIRepository, apiVersion: source.toolkit.fluxcd.io/v1, plural: ocirepositories}
  dependencies:
    - {id: hr, kind: HelmRelease, apiVersion: helm.toolkit.fluxcd.io/v2, plural: helmreleases}
  relationships:
    matchNone:
      - with: hr
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.chartRef.name, operator: EQUALS}
          - {targetPath: kind, dependencyPath: spec.chartRef.kind, operator: EQUALS}
          - {targetPath: metadata.namespace, dependencyPath: metadata.namespace, operator: EQUALS}
`

// BenchmarkFluxPattern measures the quadratic matching (targets x dependencies) of a
// three-criteria Pattern on 1,500 targets and 1,500 dependencies.
func BenchmarkFluxPattern(b *testing.B) {
	n := 1500
	var objs []unstructured.Unstructured
	for i := 0; i < n; i++ {
		objs = append(objs, unstructured.Unstructured{Object: map[string]any{"apiVersion": "source.toolkit.fluxcd.io/v1", "kind": "OCIRepository",
			"metadata": map[string]any{"name": fmt.Sprintf("src-%d", i), "namespace": "a", "uid": fmt.Sprintf("s%d", i)}}})
		objs = append(objs, unstructured.Unstructured{Object: map[string]any{"apiVersion": "helm.toolkit.fluxcd.io/v2", "kind": "HelmRelease",
			"metadata": map[string]any{"name": fmt.Sprintf("hr-%d", i), "namespace": "a", "uid": fmt.Sprintf("h%d", i)},
			"spec":     map[string]any{"chartRef": map[string]any{"name": fmt.Sprintf("src-%d", i+n/10), "kind": "OCIRepository"}}}})
	}
	g := cluster.NewGraph()
	g.Build(objs)
	p, err := linter.Lint([]byte(benchPattern))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := &fakeWriter{}
		if err := NewEngine(g, w).Run(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}
