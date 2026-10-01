package kube

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"kubepattern-go/internal/analysis"
)

func smellObj(name, namespace, patternUID string) *unstructured.Unstructured {
	o := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": smellGroup + "/" + smellVersion,
		"kind":       "Smell",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}}
	if patternUID != "" {
		o.SetLabels(map[string]string{patternUIDLabel: patternUID})
	}
	return o
}

func newTestWriter(objs ...runtime.Object) *SmellWriter {
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{smellGVR: "SmellList"}, objs...)
	return NewSmellWriter(&Client{dynamicClient: dyn}, true, "kubepattern", "scan-1")
}

func remaining(t *testing.T, w *SmellWriter) []string {
	t.Helper()
	list, err := w.client.dynamicClient.Resource(smellGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list smells: %v", err)
	}
	var names []string
	for _, o := range list.Items {
		names = append(names, o.GetName())
	}
	sort.Strings(names)
	return names
}

func TestPruneDeletesOnlyStaleSmellsOfThePattern(t *testing.T) {
	w := newTestWriter(
		smellObj("a-live", "ns1", "A"),
		smellObj("a-stale", "ns2", "A"),
		smellObj("b-other", "ns1", "B"),
	)
	if err := w.Prune(context.Background(), "A", map[string]struct{}{"a-live": {}}); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	got := remaining(t, w)
	if want := []string{"a-live", "b-other"}; !equal(got, want) {
		t.Fatalf("remaining smells = %v, want %v", got, want)
	}
}

func TestPruneOrphansKeepsInstalledPatterns(t *testing.T) {
	w := newTestWriter(
		smellObj("installed", "ns1", "A"),
		smellObj("uninstalled", "ns1", "C"),
		smellObj("unlabelled", "ns1", ""),
	)
	if err := w.PruneOrphans(context.Background(), map[string]struct{}{"A": {}}); err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}
	if got, want := remaining(t, w), []string{"installed"}; !equal(got, want) {
		t.Fatalf("remaining smells = %v, want %v", got, want)
	}
}

func TestWriteLabelsSmellWithPatternUID(t *testing.T) {
	w := newTestWriter()
	smell := analysis.Smell{
		CRDName: "p-target", PatternName: "p", PatternUID: "A", Name: "p", Category: "Test",
		Target: analysis.SmellTarget{APIVersion: "v1", Kind: "ConfigMap", Name: "cm", Namespace: "ns1", UID: "u1"},
	}
	for i := 0; i < 2; i++ { // create, then update
		if err := w.Write(context.Background(), smell); err != nil {
			t.Fatalf("Write #%d: %v", i+1, err)
		}
		got, err := w.client.dynamicClient.Resource(smellGVR).Namespace("ns1").Get(context.Background(), "p-target", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get smell: %v", err)
		}
		if l := got.GetLabels(); l[patternUIDLabel] != "A" || l[lastScanLabel] != "scan-1" {
			t.Fatalf("after write #%d labels = %v", i+1, l)
		}
	}
}

func TestInstalledPatternUIDs(t *testing.T) {
	valid, _ := json.Marshal(map[string]any{"metadata": map[string]any{"name": "p1", "uid": "A"}})
	noUID, _ := json.Marshal(map[string]any{"metadata": map[string]any{"name": "p2"}})
	got := InstalledPatternUIDs(map[string][]byte{"p1.json": valid, "p2.json": noUID, "bad.json": []byte("{")})
	if _, ok := got["A"]; !ok || len(got) != 1 {
		t.Fatalf("InstalledPatternUIDs = %v, want {A}", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
