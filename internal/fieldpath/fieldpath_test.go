package fieldpath

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func obj() map[string]any {
	return map[string]any{
		"kind": "ExternalSecret",
		"metadata": map[string]any{
			"name": "mixed",
			"labels": map[string]any{
				"app.kubernetes.io/name": "web",
				"tier":                   "push",
			},
		},
		"spec": map[string]any{
			"suspend": true,
			"data": []any{
				map[string]any{"sourceRef": map[string]any{"storeRef": map[string]any{"name": "ss-g1", "kind": "ClusterSecretStore"}}},
				map[string]any{"sourceRef": map[string]any{"storeRef": map[string]any{"name": "css-g1b", "kind": "SecretStore"}}},
				map[string]any{"secretKey": "no-source"},
			},
			"pipeline": []any{
				map[string]any{"steps": []any{"a", "b"}},
				map[string]any{"steps": []any{"c"}},
			},
		},
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{"", "a..b", "a.", "[*]", "a.[*]", "a[x]", "a[-1]", "a['b", "a[''].c", "a]b", "a[*]b", "a[0"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q): expected an error", bad)
		}
	}
	for _, good := range []string{"kind", ".metadata.name", "spec.data[*].x", "spec.data[@].x", "a[0].b", "metadata.labels['app.kubernetes.io/name']", `metadata.labels["x/y"]`, "a[*][*]", "metadata.labels.['a.b']", "metadata.labels.managed-by"} {
		if _, err := Parse(good); err != nil {
			t.Errorf("Parse(%q): unexpected error %v", good, err)
		}
	}
}

func TestValues(t *testing.T) {
	o := obj()
	cases := []struct {
		path string
		want []any
	}{
		{"metadata.name", []any{"mixed"}},
		{"spec.suspend", []any{true}},
		{"spec.data[*].sourceRef.storeRef.name", []any{"ss-g1", "css-g1b"}},
		{"spec.data[1].sourceRef.storeRef.kind", []any{"SecretStore"}},
		{"spec.data[7].sourceRef", nil},
		{"metadata.labels['app.kubernetes.io/name']", []any{"web"}},
		{"metadata.labels.tier", []any{"push"}},
		{"spec.pipeline[*].steps[*]", []any{"a", "b", "c"}},
		{"spec.data.name", nil},
		{"spec.missing[*].x", nil},
		// An unbound anchor behaves like [*].
		{"spec.data[@].sourceRef.storeRef.kind", []any{"ClusterSecretStore", "SecretStore"}},
	}
	for _, c := range cases {
		got := MustParse(c.path).Values(o, nil)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Values(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestEnumerateCorrelatesElements(t *testing.T) {
	o := obj()
	name := MustParse("spec.data[@].sourceRef.storeRef.name")
	kind := MustParse("spec.data[@].sourceRef.storeRef.kind")
	bs := Enumerate(o, []*Path{name, kind}, nil)
	if len(bs) != 3 {
		t.Fatalf("expected 3 bindings (one per element), got %d", len(bs))
	}
	var pairs []string
	for _, b := range bs {
		n, k := name.Values(o, b), kind.Values(o, b)
		if len(n) == 0 {
			continue // the element without sourceRef
		}
		pairs = append(pairs, n[0].(string)+"/"+k[0].(string))
	}
	want := []string{"ss-g1/ClusterSecretStore", "css-g1b/SecretStore"}
	if !reflect.DeepEqual(pairs, want) {
		t.Errorf("pairs = %v, want %v", pairs, want)
	}
}

func TestEnumerateNestedAndProduct(t *testing.T) {
	o := obj()
	inner := MustParse("spec.pipeline[@].steps[@]")
	got := []string{}
	for _, b := range Enumerate(o, []*Path{inner}, nil) {
		got = append(got, inner.Values(o, b)[0].(string))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "a,b,c" {
		t.Errorf("nested anchors: got %v", got)
	}

	// Two independent arrays: the cartesian product of their elements.
	a := MustParse("spec.data[@].sourceRef")
	b := MustParse("spec.pipeline[@].steps")
	if n := len(Enumerate(o, []*Path{a, b}, nil)); n != 3*2 {
		t.Errorf("product: got %d bindings, want 6", n)
	}

	// No anchors: the base binding only. Missing array: no binding.
	if bs := Enumerate(o, []*Path{MustParse("metadata.name")}, nil); len(bs) != 1 {
		t.Errorf("no anchors: got %d bindings, want 1", len(bs))
	}
	if bs := Enumerate(o, []*Path{MustParse("spec.missing[@].x")}, nil); len(bs) != 0 {
		t.Errorf("missing array: got %d bindings, want 0", len(bs))
	}

	// A binding fixed by the caller is kept.
	base := Binding{}.With(a.Anchors()[0], 1)
	bs := Enumerate(o, []*Path{a}, base)
	if len(bs) != 1 || a.Values(o, bs[0])[0].(map[string]any)["storeRef"].(map[string]any)["name"] != "css-g1b" {
		t.Errorf("base binding not kept: %v", bs)
	}
}

func TestAnchorIdentityAndRules(t *testing.T) {
	p1 := MustParse("spec.data[@].a")
	p2 := MustParse(".spec.data[@].b")
	if p1.Anchors()[0] != p2.Anchors()[0] {
		t.Errorf("paths with the same prefix must share the anchor: %v vs %v", p1.Anchors(), p2.Anchors())
	}
	if MustParse("spec.x[@].y").WildcardBeforeAnchor() {
		t.Error("no [*] before [@]")
	}
	if !MustParse("spec.x[*].y[@]").WildcardBeforeAnchor() {
		t.Error("expected [*] before [@]")
	}
	if MustParse("spec.x[*]").HasAnchor() || !MustParse("spec.x[@]").HasAnchor() {
		t.Error("HasAnchor")
	}
}

func TestRaw(t *testing.T) {
	o := obj()
	v, ok := MustParse("spec.data[*]").Raw(o, nil)
	if !ok || len(v.([]any)) != 3 {
		t.Errorf("Raw(spec.data[*]) = %v, %v", v, ok)
	}
	p := MustParse("spec.pipeline[@].steps")
	v, ok = p.Raw(o, Binding{}.With(p.Anchors()[0], 0))
	if !ok || len(v.([]any)) != 2 {
		t.Errorf("Raw with binding = %v, %v", v, ok)
	}
	if _, ok := p.Raw(o, nil); ok {
		t.Error("Raw with an unbound anchor must fail")
	}
}
