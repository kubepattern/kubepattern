package analysis

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/linter"
)

type pruneCall struct {
	patternUID string
	keep       map[string]struct{}
}

// fakeWriter records writes and prunes; failWrites makes every Write fail.
type fakeWriter struct {
	written    []Smell
	prunes     []pruneCall
	failWrites bool
}

func (f *fakeWriter) Write(_ context.Context, smell Smell) error {
	f.written = append(f.written, smell)
	if f.failWrites {
		return errors.New("write failed")
	}
	return nil
}

func (f *fakeWriter) Prune(_ context.Context, patternUID string, keep map[string]struct{}) error {
	f.prunes = append(f.prunes, pruneCall{patternUID: patternUID, keep: keep})
	return nil
}

func obj(kind, name, uid string, spec map[string]any) unstructured.Unstructured {
	o := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": "default", "uid": uid},
	}}
	if spec != nil {
		o.Object["spec"] = spec
	}
	return o
}

// unusedConfigMapPattern flags ConfigMaps not mounted by any Pod.
func unusedConfigMapPattern(uid, targetKind string, emitOnEmpty bool) *linter.PatternAsCode {
	return &linter.PatternAsCode{
		APIVersion: "kubepattern.dev/v1",
		Kind:       "Pattern",
		Metadata:   linter.Metadata{Name: "cm-not-used", UID: uid},
		Spec: linter.Spec{
			DisplayName: "ConfigMap not used",
			Category:    "Test",
			Severity:    linter.SeverityLow,
			Message:     "{{target.metadata.name}} is not used",
			Target:      linter.Target{Kind: targetKind, APIVersion: "v1", PluralName: "configmaps", EmitOnEmpty: emitOnEmpty},
			Dependencies: []linter.Dependency{
				{ID: "pod", Kind: "Pod", APIVersion: "v1", PluralName: "pods"},
			},
			Relationships: linter.Relationships{
				MatchNone: []linter.Relationship{{
					With: "pod",
					Type: linter.RelationshipCustom,
					Criteria: []linter.Criteria{{
						TargetPath:     "metadata.name",
						DependencyPath: "spec.volumes[*].configMap.name",
						Operator:       linter.CriteriaEquals,
					}},
				}},
			},
		},
	}
}

func testGraph() *cluster.Graph {
	g := cluster.NewGraph()
	g.Build([]unstructured.Unstructured{
		obj("ConfigMap", "used", "cm-used", nil),
		obj("ConfigMap", "unused", "cm-unused", nil),
		obj("Pod", "app", "pod-app", map[string]any{
			"volumes": []any{map[string]any{"configMap": map[string]any{"name": "used"}}},
		}),
	})
	return g
}

func keys(m map[string]struct{}) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func TestRunPrunesWithLiveSet(t *testing.T) {
	w := &fakeWriter{}
	if err := NewEngine(testGraph(), w).Run(context.Background(), unusedConfigMapPattern("pat-uid", "ConfigMap", false)); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	want := smellCRDName("cm-not-used", types.UID("cm-unused"))
	if len(w.written) != 1 || w.written[0].CRDName != want {
		t.Fatalf("expected exactly the smell %q to be written, got %+v", want, w.written)
	}
	if w.written[0].PatternUID != "pat-uid" {
		t.Errorf("smell PatternUID = %q, want %q", w.written[0].PatternUID, "pat-uid")
	}
	if len(w.prunes) != 1 {
		t.Fatalf("expected one Prune call, got %d", len(w.prunes))
	}
	if p := w.prunes[0]; p.patternUID != "pat-uid" || len(p.keep) != 1 || !keys(p.keep)[want] {
		t.Errorf("Prune(%q, %v), want Prune(%q, {%s})", p.patternUID, keys(p.keep), "pat-uid", want)
	}
}

func TestRunKeepsSmellsWhoseWriteFailed(t *testing.T) {
	w := &fakeWriter{failWrites: true}
	_ = NewEngine(testGraph(), w).Run(context.Background(), unusedConfigMapPattern("pat-uid", "ConfigMap", false))

	want := smellCRDName("cm-not-used", types.UID("cm-unused"))
	if len(w.prunes) != 1 || !keys(w.prunes[0].keep)[want] {
		t.Fatalf("a smell whose write failed must stay in the keep set, got %+v", w.prunes)
	}
}

func TestRunWithoutTargetsPrunesEverything(t *testing.T) {
	w := &fakeWriter{}
	_ = NewEngine(testGraph(), w).Run(context.Background(), unusedConfigMapPattern("pat-uid", "Secret", false))

	if len(w.written) != 0 {
		t.Errorf("expected no writes, got %d", len(w.written))
	}
	if len(w.prunes) != 1 || len(w.prunes[0].keep) != 0 {
		t.Fatalf("expected one Prune with an empty keep set, got %+v", w.prunes)
	}
}

func TestRunEmitOnEmptyKeepsSyntheticSmell(t *testing.T) {
	w := &fakeWriter{}
	_ = NewEngine(testGraph(), w).Run(context.Background(), unusedConfigMapPattern("pat-uid", "Secret", true))

	if len(w.written) != 1 || w.written[0].CRDName != "cm-not-used-empty" {
		t.Fatalf("expected the synthetic smell to be written, got %+v", w.written)
	}
	if len(w.prunes) != 1 || !keys(w.prunes[0].keep)["cm-not-used-empty"] || len(w.prunes[0].keep) != 1 {
		t.Fatalf("expected Prune to keep only the synthetic smell, got %+v", w.prunes)
	}
}

func TestRunWithoutPatternUIDSkipsPrune(t *testing.T) {
	w := &fakeWriter{}
	_ = NewEngine(testGraph(), w).Run(context.Background(), unusedConfigMapPattern("", "ConfigMap", false))

	if len(w.written) != 1 {
		t.Errorf("expected the smell to be written, got %d writes", len(w.written))
	}
	if len(w.prunes) != 0 {
		t.Fatalf("a pattern without UID must not be pruned, got %+v", w.prunes)
	}
}
