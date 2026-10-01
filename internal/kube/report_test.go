package kube

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"kubepattern-go/internal/analysis"
)

// t0 is the start of the first test run.
var t0 = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

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
	return NewSmellWriter(&Client{dynamicClient: dyn}, true, "kubepattern", "scan-1", t0)
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

func forSmell(d time.Duration) analysis.Smell {
	return analysis.Smell{
		CRDName: "p-target", PatternName: "p", PatternUID: "A", Name: "p", Category: "Test",
		Message: "unused since {{smell.since}} ({{smell.phase}})", For: d,
		Target: analysis.SmellTarget{APIVersion: "v1", Kind: "ConfigMap", Name: "cm", Namespace: "ns1", UID: "u1"},
	}
}

func getSmell(t *testing.T, w *SmellWriter) *unstructured.Unstructured {
	t.Helper()
	got, err := w.client.dynamicClient.Resource(smellGVR).Namespace("ns1").Get(context.Background(), "p-target", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get smell: %v", err)
	}
	return got
}

func TestWritePromotesPendingToActiveAfterFor(t *testing.T) {
	w := newTestWriter()
	steps := []struct {
		offset time.Duration
		phase  string
	}{
		{0, phasePending},                // first observation
		{30 * time.Minute, phasePending}, // held for 30m < 1h
		{time.Hour, phaseActive},         // held for 1h
		{90 * time.Minute, phaseActive},
	}
	for _, s := range steps {
		w.runStart = t0.Add(s.offset)
		if err := w.Write(context.Background(), forSmell(time.Hour)); err != nil {
			t.Fatalf("Write at +%v: %v", s.offset, err)
		}
		got := getSmell(t, w)
		since, _, _ := unstructured.NestedString(got.Object, "spec", "since")
		phase, _, _ := unstructured.NestedString(got.Object, "spec", "phase")
		msg, _, _ := unstructured.NestedString(got.Object, "spec", "message")
		if since != "2026-10-01T10:00:00Z" || phase != s.phase || got.GetLabels()[phaseLabel] != s.phase {
			t.Fatalf("at +%v: since=%q phase=%q label=%q, want since 2026-10-01T10:00:00Z, phase %s",
				s.offset, since, phase, got.GetLabels()[phaseLabel], s.phase)
		}
		if want := "unused since 2026-10-01T10:00:00Z (" + s.phase + ")"; msg != want {
			t.Fatalf("at +%v: message = %q, want %q", s.offset, msg, want)
		}
	}
}

func TestWriteWithoutForIsActiveAtFirstObservation(t *testing.T) {
	w := newTestWriter()
	if err := w.Write(context.Background(), forSmell(0)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if phase, _, _ := unstructured.NestedString(getSmell(t, w).Object, "spec", "phase"); phase != phaseActive {
		t.Fatalf("phase = %q, want %s", phase, phaseActive)
	}
}

func TestWriteKeepsSuppress(t *testing.T) {
	w := newTestWriter()
	if err := w.Write(context.Background(), forSmell(0)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := getSmell(t, w)
	if err := unstructured.SetNestedField(got.Object, true, "spec", "suppress"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.client.dynamicClient.Resource(smellGVR).Namespace("ns1").Update(context.Background(), got, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("suppress smell: %v", err)
	}
	if err := w.Write(context.Background(), forSmell(0)); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	if suppress, _, _ := unstructured.NestedBool(getSmell(t, w).Object, "spec", "suppress"); !suppress {
		t.Fatal("suppress was reset by the update")
	}
}

func TestWriteFallsBackToCreationTimestamp(t *testing.T) {
	// A smell written before spec.since existed: its creation time is its first observation.
	old := smellObj("p-target", "ns1", "A")
	old.SetCreationTimestamp(metav1.NewTime(t0.Add(-2 * time.Hour)))
	old.Object["spec"] = map[string]any{"name": "p", "category": "Test"}
	w := newTestWriter(old)
	if err := w.Write(context.Background(), forSmell(time.Hour)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := getSmell(t, w)
	since, _, _ := unstructured.NestedString(got.Object, "spec", "since")
	phase, _, _ := unstructured.NestedString(got.Object, "spec", "phase")
	if since != "2026-10-01T08:00:00Z" || phase != phaseActive {
		t.Fatalf("since=%q phase=%q, want 2026-10-01T08:00:00Z and %s", since, phase, phaseActive)
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
