package kube

import (
	"context"
	"sort"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/linter"
)

// fakeDiscovery serves preferred as the preferred resources, or all its Resources when
// preferred is nil (the client-go fake returns nothing for ServerPreferredResources).
type fakeDiscovery struct {
	*discoveryfake.FakeDiscovery
	preferred []*metav1.APIResourceList
}

func (f fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	if f.preferred != nil {
		return f.preferred, nil
	}
	return f.Resources, nil
}

type served struct {
	gv         string
	kind       string
	resource   string
	verbs      []string
	categories []string
	namespaced bool
}

var testResources = []served{
	{"v1", "Pod", "pods", []string{"get", "list"}, nil, true},
	{"v1", "Pod", "pods/status", []string{"get"}, nil, true},
	{"widgets.templates.krateo.io/v1beta1", "Markdown", "markdowns", []string{"get", "list", "watch"}, nil, true},
	{"widgets.templates.krateo.io/v1beta1", "Markdown", "markdowns/status", []string{"get", "list"}, nil, true},
	{"widgets.templates.krateo.io/v1beta1", "Paragraph", "paragraphs", []string{"get", "list"}, nil, true},
	{"widgets.templates.krateo.io/v1beta1", "Review", "reviews", []string{"create"}, nil, true},
	{"helm.toolkit.fluxcd.io/v2", "HelmRelease", "helmreleases", []string{"list"}, []string{"all", "fluxcd-appliers"}, true},
	{"kustomize.toolkit.fluxcd.io/v1", "Kustomization", "kustomizations", []string{"list"}, []string{"fluxcd-appliers"}, true},
	{"source.toolkit.fluxcd.io/v1", "OCIRepository", "ocirepositories", []string{"list"}, []string{"all"}, true},
	{"core.krateo.io/v1alpha1", "CompositionDefinition", "compositiondefinitions", []string{"list"}, nil, true},
	{"composition.krateo.io/v1-2-0", "FireworksApp", "fireworksapps", []string{"list"}, nil, true},
	{"apps/v1", "ReplicaSet", "replicasets", []string{"list"}, nil, true},
}

func obj(apiVersion, kind, ns, name string, fields map[string]any) *unstructured.Unstructured {
	m := map[string]any{"apiVersion": apiVersion, "kind": kind,
		"metadata": map[string]any{"name": name, "namespace": ns, "uid": kind + "-" + name}}
	for k, v := range fields {
		m[k] = v
	}
	return &unstructured.Unstructured{Object: m}
}

// newTestClient builds a Client over fake discovery, dynamic client and RESTMapper. Listing
// a resource in notServed answers 404, as for a CRD that is not installed.
func newTestClient(t *testing.T, notServed []schema.GroupVersionResource, objs ...runtime.Object) *Client {
	t.Helper()
	byGV := map[string]*metav1.APIResourceList{}
	var lists []*metav1.APIResourceList
	listKinds := map[schema.GroupVersionResource]string{}
	mapper := meta.NewDefaultRESTMapper(nil)
	for _, r := range testResources {
		l, ok := byGV[r.gv]
		if !ok {
			l = &metav1.APIResourceList{GroupVersion: r.gv}
			byGV[r.gv] = l
			lists = append(lists, l)
		}
		l.APIResources = append(l.APIResources, metav1.APIResource{Name: r.resource, Kind: r.kind, Verbs: r.verbs, Categories: r.categories, Namespaced: r.namespaced})
		if strings.Contains(r.resource, "/") {
			continue
		}
		gv, _ := schema.ParseGroupVersion(r.gv)
		listKinds[gv.WithResource(r.resource)] = r.kind + "List"
		mapper.AddSpecific(gv.WithKind(r.kind), gv.WithResource(r.resource), gv.WithResource(strings.ToLower(r.kind)), meta.RESTScopeNamespace)
	}
	for _, gvr := range notServed {
		listKinds[gvr] = "UnservedList" // the fake requires a list kind before any reactor runs
	}
	disco := fakeDiscovery{FakeDiscovery: &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: lists}}}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
	for _, gvr := range notServed {
		gr := gvr.GroupResource()
		dyn.PrependReactor("list", gvr.Resource, func(a clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(gr, "")
		})
	}
	return newClient(disco, dyn, mapper)
}

func resourceNames(res []Resource) []string {
	var out []string
	for _, r := range res {
		out = append(out, r.APIVersion+"/"+r.Kind+"="+r.Resource)
	}
	sort.Strings(out)
	return out
}

func objectNames(objs []unstructured.Unstructured) []string {
	var out []string
	for _, o := range objs {
		out = append(out, o.GetKind()+"/"+o.GetName())
	}
	sort.Strings(out)
	return out
}

func expectList(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestExpandRefs(t *testing.T) {
	c := newTestClient(t, nil)

	res, err := c.ExpandRefs([]linter.ResourceRef{{Kind: "Pod", APIVersion: "v1", PluralName: "pods"}})
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "concrete", resourceNames(res), "v1/Pod=pods")

	// kind "*": the listable kinds of the group version, without subresources.
	res, err = c.ExpandRefs([]linter.ResourceRef{{Kind: "*", APIVersion: "widgets.templates.krateo.io/v1beta1"}})
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "wildcard", resourceNames(res),
		"widgets.templates.krateo.io/v1beta1/Markdown=markdowns", "widgets.templates.krateo.io/v1beta1/Paragraph=paragraphs")

	// category: every listable kind that declares it, across groups.
	res, err = c.ExpandRefs([]linter.ResourceRef{{Category: "fluxcd-appliers"}})
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "category", resourceNames(res),
		"helm.toolkit.fluxcd.io/v2/HelmRelease=helmreleases", "kustomize.toolkit.fluxcd.io/v1/Kustomization=kustomizations")

	// A category nobody declares gives no resource and no error.
	res, err = c.ExpandRefs([]linter.ResourceRef{{Category: "nobody"}})
	if err != nil || len(res) != 0 {
		t.Errorf("unknown category: got %v, %v; want no resource and no error", res, err)
	}

	// A list is the union.
	res, err = c.ExpandRefs([]linter.ResourceRef{{Kind: "Pod", APIVersion: "v1", PluralName: "pods"}, {Category: "fluxcd-appliers"}})
	if err != nil || len(res) != 3 {
		t.Errorf("list: got %v, %v; want 3 resources", resourceNames(res), err)
	}

	// An unknown group version is an error, as a missing CRD is.
	if _, err := c.ExpandRefs([]linter.ResourceRef{{Kind: "*", APIVersion: "nothing.example.com/v1"}}); err == nil {
		t.Error("unknown group version: want an error")
	}
}

func lintPattern(t *testing.T, y string) *linter.PatternAsCode {
	t.Helper()
	p, err := linter.Lint([]byte(y))
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	p.Metadata.UID = "uid-p"
	return p
}

const patternHead = `apiVersion: kubepattern.dev/v1
kind: Pattern
metadata: {name: p}
spec:
  displayName: P
  category: Test
  severity: LOW
  message: m
`

// G6 end to end: a kind-wildcard target and a category dependency are expanded through
// discovery, fetched, recorded in the Pattern, and matched by the engine.
func TestFetchForPatternKindSets(t *testing.T) {
	c := newTestClient(t, nil,
		obj("widgets.templates.krateo.io/v1beta1", "Markdown", "a", "md-used", nil),
		obj("widgets.templates.krateo.io/v1beta1", "Paragraph", "a", "para-unused", nil),
		obj("helm.toolkit.fluxcd.io/v2", "HelmRelease", "a", "hr", map[string]any{"spec": map[string]any{"widgetRef": "md-used"}}),
		obj("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "a", "ks", map[string]any{"spec": map[string]any{"widgetRef": "elsewhere"}}),
		obj("source.toolkit.fluxcd.io/v1", "OCIRepository", "a", "oci", map[string]any{"spec": map[string]any{"widgetRef": "para-unused"}}),
	)
	p := lintPattern(t, patternHead+`  target: {kind: "*", apiVersion: widgets.templates.krateo.io/v1beta1}
  dependencies:
    - {id: appliers, category: fluxcd-appliers}
  relationships:
    matchNone:
      - with: appliers
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: spec.widgetRef, operator: EQUALS}
`)
	objs, err := c.FetchForPattern(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The OCIRepository is not an applier: it is not fetched.
	expectList(t, "fetched", objectNames(objs), "HelmRelease/hr", "Kustomization/ks", "Markdown/md-used", "Paragraph/para-unused")
	if _, ok := p.Spec.Target.Resolved[linter.GVKKey("widgets.templates.krateo.io/v1beta1", "Paragraph")]; !ok {
		t.Errorf("target Resolved = %v, want the widget kinds", p.Spec.Target.Resolved)
	}
	if len(p.Spec.Dependencies[0].Resolved) != 2 {
		t.Errorf("dependency Resolved = %v, want HelmRelease and Kustomization", p.Spec.Dependencies[0].Resolved)
	}
	expectList(t, "smells", runEngine(t, p, objs), "para-unused")
}

// G6 fromTarget: the dependency's kinds are read from the targets; a served kind is
// fetched, a kind that is not served (404, or unknown to the RESTMapper) has no instances.
func TestFetchForPatternFromTarget(t *testing.T) {
	def := func(name string, status map[string]any) *unstructured.Unstructured {
		return obj("core.krateo.io/v1alpha1", "CompositionDefinition", "a", name, map[string]any{"status": status})
	}
	c := newTestClient(t, []schema.GroupVersionResource{{Group: "composition.krateo.io", Version: "v1-0-0", Resource: "ghostapps"}},
		def("fireworks", map[string]any{"apiVersion": "composition.krateo.io/v1-2-0", "kind": "FireworksApp", "resource": "fireworksapps"}),
		def("unused", map[string]any{"apiVersion": "composition.krateo.io/v1-2-0", "kind": "FireworksApp", "resource": "fireworksapps"}),
		def("ghost", map[string]any{"apiVersion": "composition.krateo.io/v1-0-0", "kind": "GhostApp", "resource": "ghostapps"}),
		def("unmapped", map[string]any{"apiVersion": "unknown.example.com/v1", "kind": "Unknown"}),
		def("pending", map[string]any{}),
		obj("composition.krateo.io/v1-2-0", "FireworksApp", "a", "app-1", map[string]any{"metadata": map[string]any{
			"name": "app-1", "namespace": "a", "uid": "app-1",
			"labels": map[string]any{"krateo.io/composition-definition-name": "fireworks"}}}),
	)
	p := lintPattern(t, patternHead+`  target: {kind: CompositionDefinition, apiVersion: core.krateo.io/v1alpha1, plural: compositiondefinitions}
  dependencies:
    - id: composition
      fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind, pluralPath: status.resource}
  relationships:
    matchNone:
      - with: composition
        type: custom
        criteria:
          - {targetPath: metadata.name, dependencyPath: "metadata.labels['krateo.io/composition-definition-name']", operator: EQUALS}
`)
	objs, err := c.FetchForPattern(context.Background(), p, nil)
	if err != nil {
		t.Fatalf("an unserved derived kind must not fail the pattern: %v", err)
	}
	if n := len(p.Spec.Dependencies[0].Resolved); n != 3 {
		t.Errorf("dependency Resolved = %v, want FireworksApp, GhostApp and Unknown", p.Spec.Dependencies[0].Resolved)
	}
	expectList(t, "smells", runEngine(t, p, objs), "ghost", "pending", "unmapped", "unused")

	// A fixed apiVersion instead of a path.
	p = lintPattern(t, patternHead+`  target: {kind: CompositionDefinition, apiVersion: core.krateo.io/v1alpha1, plural: compositiondefinitions}
  dependencies:
    - id: composition
      fromTarget: {apiVersion: composition.krateo.io/v1-2-0, kindPath: status.kind}
`)
	c = newTestClient(t, nil, def("fireworks", map[string]any{"kind": "FireworksApp"}),
		obj("composition.krateo.io/v1-2-0", "FireworksApp", "a", "app-1", nil))
	objs, err = c.FetchForPattern(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "fetched", objectNames(objs), "CompositionDefinition/fireworks", "FireworksApp/app-1")
}

// Targets fetched by an earlier Pattern are still read to derive the kinds (the client
// lists each resource type once per run).
func TestFetchForPatternFromTargetFetchedEarlier(t *testing.T) {
	c := newTestClient(t, nil,
		obj("core.krateo.io/v1alpha1", "CompositionDefinition", "a", "fireworks", map[string]any{"status": map[string]any{"apiVersion": "composition.krateo.io/v1-2-0", "kind": "FireworksApp"}}),
		obj("composition.krateo.io/v1-2-0", "FireworksApp", "a", "app-1", nil),
	)
	first := lintPattern(t, patternHead+`  target: {kind: CompositionDefinition, apiVersion: core.krateo.io/v1alpha1, plural: compositiondefinitions}
`)
	earlier, err := c.FetchForPattern(context.Background(), first, nil)
	if err != nil || len(earlier) != 1 {
		t.Fatalf("first pattern: %v, %v", objectNames(earlier), err)
	}
	second := lintPattern(t, patternHead+`  target: {kind: CompositionDefinition, apiVersion: core.krateo.io/v1alpha1, plural: compositiondefinitions}
  dependencies:
    - id: composition
      fromTarget: {apiVersionPath: status.apiVersion, kindPath: status.kind}
`)
	objs, err := c.FetchForPattern(context.Background(), second, earlier)
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "fetched by the second pattern", objectNames(objs), "FireworksApp/app-1")
}

// A missing group version in a kind wildcard skips the pattern, as a missing CRD does.
func TestFetchForPatternUnknownGroupVersion(t *testing.T) {
	c := newTestClient(t, nil)
	p := lintPattern(t, patternHead+`  target: {kind: "*", apiVersion: nothing.example.com/v1}
`)
	if _, err := c.FetchForPattern(context.Background(), p, nil); err == nil {
		t.Error("want an error")
	}
}

// Owners of fetched objects are fetched too (inheritance), also for kinds found by discovery.
func TestFetchForPatternFetchesOwners(t *testing.T) {
	pod := obj("v1", "Pod", "a", "web-1", nil)
	pod.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web", UID: "ReplicaSet-web"}})
	c := newTestClient(t, nil, pod, obj("apps/v1", "ReplicaSet", "a", "web", nil))
	p := lintPattern(t, patternHead+`  target: {kinds: [{kind: "*", apiVersion: v1}]}
`)
	objs, err := c.FetchForPattern(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "fetched", objectNames(objs), "Pod/web-1", "ReplicaSet/web")
}

type memWriter struct{ names []string }

func (w *memWriter) Write(_ context.Context, s analysis.Smell) error {
	w.names = append(w.names, s.Target.Name)
	return nil
}

func (w *memWriter) Prune(context.Context, string, map[string]struct{}) error { return nil }

func runEngine(t *testing.T, p *linter.PatternAsCode, objs []unstructured.Unstructured) []string {
	t.Helper()
	g := cluster.NewGraph()
	g.Build(objs)
	w := &memWriter{}
	if err := analysis.NewEngine(g, w).Run(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	sort.Strings(w.names)
	return w.names
}

// A type that cannot be listed (RBAC) skips every pattern that reads it, also a later one:
// the failed type is not recorded as fetched (D2), so no pattern runs on missing data.
func TestFetchForPatternListFailureIsNotCached(t *testing.T) {
	c := newTestClient(t, nil,
		obj("helm.toolkit.fluxcd.io/v2", "HelmRelease", "a", "hr", nil),
		obj("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "a", "ks", nil),
	)
	dyn := c.dynamicClient.(*dynamicfake.FakeDynamicClient)
	dyn.PrependReactor("list", "kustomizations", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "kustomize.toolkit.fluxcd.io", Resource: "kustomizations"}, "", nil)
	})
	byCategory := lintPattern(t, patternHead+`  target: {category: fluxcd-appliers}
`)
	if _, err := c.FetchForPattern(context.Background(), byCategory, nil); err == nil || !apierrors.IsForbidden(err) {
		t.Fatalf("category with a forbidden kind: err = %v, want Forbidden", err)
	}
	byKind := lintPattern(t, patternHead+`  target: {kind: Kustomization, apiVersion: kustomize.toolkit.fluxcd.io/v1, plural: kustomizations}
`)
	if _, err := c.FetchForPattern(context.Background(), byKind, nil); err == nil {
		t.Fatal("a later pattern on the forbidden kind must fail too, not run on no data")
	}
	listable := lintPattern(t, patternHead+`  target: {kind: HelmRelease, apiVersion: helm.toolkit.fluxcd.io/v2, plural: helmreleases}
`)
	objs, err := c.FetchForPattern(context.Background(), listable, nil)
	if err != nil {
		t.Fatal(err)
	}
	// HelmReleases were listed (and cached) before the forbidden list failed.
	if len(objs) != 0 {
		t.Errorf("HelmReleases fetched again: %v", objectNames(objs))
	}
}

// fromTarget with "[@]": each element of the target names one type, its kind and plural
// read from the same element; targets excluded by the target filters derive nothing.
func TestFetchForPatternFromTargetElements(t *testing.T) {
	c := newTestClient(t, []schema.GroupVersionResource{{Group: "composition.krateo.io", Version: "v1-0-0", Resource: "ghostapps"}},
		obj("core.krateo.io/v1alpha1", "CompositionDefinition", "a", "multi", map[string]any{"spec": map[string]any{"enabled": "yes", "generates": []any{
			map[string]any{"apiVersion": "composition.krateo.io/v1-2-0", "kind": "FireworksApp", "plural": "fireworksapps"},
			map[string]any{"apiVersion": "composition.krateo.io/v1-0-0", "kind": "GhostApp", "plural": "ghostapps"},
			map[string]any{"apiVersion": "composition.krateo.io/v1-2-0", "kind": "NoPlural"},
		}}}),
		obj("core.krateo.io/v1alpha1", "CompositionDefinition", "a", "disabled", map[string]any{"spec": map[string]any{"enabled": "no", "generates": []any{
			map[string]any{"apiVersion": "apps/v1", "kind": "ReplicaSet", "plural": "replicasets"},
		}}}),
		obj("composition.krateo.io/v1-2-0", "FireworksApp", "a", "app-1", nil),
		obj("apps/v1", "ReplicaSet", "a", "rs-1", nil),
	)
	p := lintPattern(t, patternHead+`  target:
    kind: CompositionDefinition
    apiVersion: core.krateo.io/v1alpha1
    plural: compositiondefinitions
    filters:
      matchAll:
        - {path: spec.enabled, operator: EQUALS, values: ["yes"]}
  dependencies:
    - id: generated
      fromTarget: {apiVersionPath: "spec.generates[@].apiVersion", kindPath: "spec.generates[@].kind", pluralPath: "spec.generates[@].plural"}
`)
	objs, err := c.FetchForPattern(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	expectList(t, "fetched", objectNames(objs), "CompositionDefinition/disabled", "CompositionDefinition/multi", "FireworksApp/app-1")
	var resolved []string
	for k := range p.Spec.Dependencies[0].Resolved {
		resolved = append(resolved, k)
	}
	sort.Strings(resolved)
	expectList(t, "resolved", resolved,
		"composition.krateo.io/v1-0-0|GhostApp", "composition.krateo.io/v1-2-0|FireworksApp", "composition.krateo.io/v1-2-0|NoPlural")
}
