package fieldpath

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// canonical writes a parsed path back with writeToken (the form used for anchor identity).
func canonical(p *Path) string {
	var b strings.Builder
	for i, t := range p.tokens {
		writeToken(&b, t, i == 0)
	}
	return b.String()
}

var fuzzSeeds = []string{
	"spec.data[*].name", "spec.data[@].name", "spec.a[@].b[@].c", "a[0]", "a[12].b",
	"metadata.labels['app.kubernetes.io/name']", `metadata.annotations["a.b/c"]`, ".spec.x",
	"a..b", "a.", "[*]", "a[", "a]", "a[x]", "a[-1]", "a['']", "a['b'", "a[*]b", "a.[*]",
	"['a.b'].c", "a['x.y'][@].z", "a[@][@]", "spec.selector",
}

// FuzzParse: Parse never panics; a parsed path's canonical form parses to the same tokens;
// evaluation never panics on a sample object.
func FuzzParse(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	obj := map[string]any{
		"a":    []any{map[string]any{"b": []any{map[string]any{"c": "x"}}, "x.y": []any{map[string]any{"z": 1}}}, "s", nil, []any{1, 2}},
		"spec": map[string]any{"x": true, "data": []any{map[string]any{"name": "n"}}, "selector": map[string]any{}},
		"a.b":  map[string]any{"c": "dotted"},
	}
	f.Fuzz(func(t *testing.T, raw string) {
		p, err := Parse(raw)
		if err != nil {
			return
		}
		c := canonical(p)
		// A quoted key holding both quote characters, or a quote and a dot, has no exact
		// canonical spelling; the canonical form only names anchors.
		hasQuote := false
		for _, tk := range p.tokens {
			if tk.kind == tokKey && strings.ContainsAny(tk.key, `'"`) {
				hasQuote = true
			}
		}
		if !hasQuote {
			q, err := Parse(c)
			if err != nil {
				t.Fatalf("canonical %q of %q does not parse: %v", c, raw, err)
			}
			if fmt.Sprint(q.tokens) != fmt.Sprint(p.tokens) {
				t.Fatalf("canonical %q of %q parses to %v, want %v", c, raw, q.tokens, p.tokens)
			}
		}
		_ = p.Values(obj, nil)
		_, _ = p.Raw(obj, nil)
		_ = p.HasWildcard()
		_ = p.LastKey()
		bindings := Enumerate(obj, []*Path{p}, nil)
		// The union of the values under every binding of the path's own anchors equals the
		// values with the anchors unbound (an unbound anchor reads every element).
		if !p.WildcardBeforeAnchor() {
			var union []string
			for _, b := range bindings {
				for _, v := range p.Values(obj, b) {
					union = append(union, fmt.Sprint(v))
				}
			}
			var all []string
			for _, v := range p.Values(obj, nil) {
				all = append(all, fmt.Sprint(v))
			}
			sort.Strings(union)
			sort.Strings(all)
			if p.HasAnchor() && strings.Join(union, "|") != strings.Join(all, "|") {
				t.Fatalf("%q: union over bindings %v, unbound %v", raw, union, all)
			}
		}
	})
}
