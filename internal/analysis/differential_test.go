package analysis

// Differential tests against the legacy engine (legacy_test.go): on randomly generated
// objects and Patterns restricted to the original constructs (keys and "[*]" paths, string
// EQUALS filters, numeric and array-size filters, custom / owns / ownedBy relationships),
// the current engine must emit exactly the same Smells.

import (
	"context"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/linter"
)

type gen struct{ r *rand.Rand }

var (
	genKeys = []string{"a", "b", "c"}
	// Scalars include the string forms of the non-string scalars ("1", "true"): a filter
	// comparing them must keep its legacy verdict.
	genScalars = []any{"x", "y", "z", "", "1", "true", int64(0), int64(1), int64(2), 1.5, true, false, nil}
	// EQUALS filter values never equal the string form of a non-string scalar: there the
	// typed EQUALS (G10) deliberately differs from the legacy one (TestTypedEqualsDiffers).
	genEqualsValues = []string{"x", "y", "z", ""}
	genNumbers      = []string{"0", "1", "2"}
)

func (g *gen) pick(n int) int { return g.r.IntN(n) }

func (g *gen) value(depth int) any {
	n := g.pick(10)
	switch {
	case depth >= 3 || n < 4:
		if g.pick(2) == 0 {
			return []string{"x", "y"}[g.pick(2)] // frequent values, so that criteria match
		}
		return genScalars[g.pick(len(genScalars))]
	case n < 7:
		m := map[string]any{}
		for i := 0; i < 1+g.pick(3); i++ {
			m[genKeys[g.pick(len(genKeys))]] = g.value(depth + 1)
		}
		return m
	default:
		var arr []any
		for i := 0; i < g.pick(4); i++ {
			arr = append(arr, g.value(depth+1))
		}
		if arr == nil {
			return []any{}
		}
		return arr
	}
}

func (g *gen) object(kind, name string) map[string]any {
	spec := map[string]any{}
	for _, k := range genKeys {
		if g.pick(4) != 0 {
			spec[k] = g.value(1)
		}
	}
	return map[string]any{
		"apiVersion": "test.example.com/v1",
		"kind":       kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": []string{"ns1", "ns2"}[g.pick(2)],
			"uid":       "uid-" + name,
		},
		"spec": spec,
	}
}

func (g *gen) path() string {
	switch g.pick(12) {
	case 0:
		return "metadata.name"
	case 1:
		return "metadata.namespace"
	case 2:
		return "kind"
	case 3:
		return "metadata.ownerReferences[*].name"
	}
	var b strings.Builder
	if g.pick(8) == 0 {
		b.WriteString(".")
	}
	b.WriteString("spec")
	for i := 0; i < 1+g.pick(2)+g.pick(2); i++ {
		b.WriteString("." + genKeys[g.pick(len(genKeys))])
		if g.pick(3) == 0 {
			b.WriteString("[*]")
		}
	}
	return b.String()
}

var genOperators = []linter.FilterOperator{
	linter.FilterExists, linter.FilterIsEmpty, linter.FilterEquals,
	linter.FilterGreaterThan, linter.FilterGreaterOrEqual, linter.FilterLessThan, linter.FilterLessOrEqual,
	linter.FilterArraySizeEquals, linter.FilterArraySizeGreaterThan, linter.FilterArraySizeGreaterOrEqual,
	linter.FilterArraySizeLessThan, linter.FilterArraySizeLessOrEqual,
}

func (g *gen) condition() linter.FilterCondition {
	op := genOperators[g.pick(len(genOperators))]
	c := linter.FilterCondition{Path: g.path(), Operator: op}
	switch op {
	case linter.FilterExists, linter.FilterIsEmpty:
	case linter.FilterEquals:
		for i := 0; i < 1+g.pick(2); i++ {
			c.Values = append(c.Values, genEqualsValues[g.pick(len(genEqualsValues))])
		}
	default:
		c.Values = []string{genNumbers[g.pick(len(genNumbers))]}
	}
	return c
}

func (g *gen) filters() linter.Filters {
	var f linter.Filters
	if g.pick(3) != 0 {
		return f
	}
	for i := 0; i < g.pick(3); i++ {
		f.MatchAll = append(f.MatchAll, g.condition())
	}
	for i := 0; i < g.pick(3); i++ {
		f.MatchAny = append(f.MatchAny, g.condition())
	}
	for i := 0; i < g.pick(2); i++ {
		f.MatchNone = append(f.MatchNone, g.condition())
	}
	return f
}

func (g *gen) relationship(depIDs []string) linter.Relationship {
	rel := linter.Relationship{With: depIDs[g.pick(len(depIDs))]}
	switch g.pick(10) {
	case 0:
		rel.Type = linter.RelationshipOwns
	case 1:
		rel.Type = linter.RelationshipOwnedBy
	default:
		rel.Type = linter.RelationshipCustom
		for i := 0; i < 1+g.pick(2); i++ {
			tp, dp := g.path(), g.path()
			if g.pick(2) == 0 {
				dp = tp // the same field on both sides matches often
			}
			rel.Criteria = append(rel.Criteria, linter.Criteria{TargetPath: tp, DependencyPath: dp, Operator: linter.CriteriaEquals})
		}
	}
	return rel
}

func (g *gen) pattern() *linter.PatternAsCode {
	p := &linter.PatternAsCode{APIVersion: "kubepattern.dev/v1", Kind: "Pattern"}
	p.Metadata.Name = "diff"
	p.Metadata.UID = "uid-diff"
	p.Spec.DisplayName, p.Spec.Category, p.Spec.Severity, p.Spec.Message = "D", "Test", linter.SeverityLow, "m"
	p.Spec.Target = linter.Target{Kind: "T", APIVersion: "test.example.com/v1", PluralName: "ts", Filters: g.filters()}
	var ids []string
	for i := 0; i < 1+g.pick(2); i++ {
		id := fmt.Sprintf("d%d", i)
		ids = append(ids, id)
		p.Spec.Dependencies = append(p.Spec.Dependencies, linter.Dependency{
			ID: id, Kind: fmt.Sprintf("D%d", i), APIVersion: "test.example.com/v1", PluralName: fmt.Sprintf("d%ds", i), Filters: g.filters(),
		})
	}
	for i := 0; i < g.pick(2); i++ {
		p.Spec.Relationships.MatchAll = append(p.Spec.Relationships.MatchAll, g.relationship(ids))
	}
	for i := 0; i < g.pick(3); i++ {
		p.Spec.Relationships.MatchAny = append(p.Spec.Relationships.MatchAny, g.relationship(ids))
	}
	for i := 0; i < g.pick(3); i++ {
		p.Spec.Relationships.MatchNone = append(p.Spec.Relationships.MatchNone, g.relationship(ids))
	}
	return p
}

func (g *gen) objects(p *linter.PatternAsCode) []unstructured.Unstructured {
	kinds := []string{"T"}
	for _, d := range p.Spec.Dependencies {
		kinds = append(kinds, d.Kind)
	}
	var objs []unstructured.Unstructured
	for _, k := range kinds {
		for i := 0; i < 2+g.pick(5); i++ {
			objs = append(objs, unstructured.Unstructured{Object: g.object(k, fmt.Sprintf("%s-%d", strings.ToLower(k), i))})
		}
	}
	// Random ownership (no self-ownership), so that owns / ownedBy and the owner-name paths
	// have something to find.
	for i := range objs {
		if g.pick(3) != 0 {
			continue
		}
		owner := objs[g.pick(len(objs))]
		if owner.GetUID() == objs[i].GetUID() {
			continue
		}
		objs[i].Object["metadata"].(map[string]any)["ownerReferences"] = []any{map[string]any{
			"apiVersion": owner.GetAPIVersion(), "kind": owner.GetKind(), "name": owner.GetName(), "uid": string(owner.GetUID()),
		}}
	}
	return objs
}

func smellNames(t *testing.T, g *cluster.Graph, p *linter.PatternAsCode) []string {
	t.Helper()
	w := &fakeWriter{}
	if err := NewEngine(g, w).Run(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range w.written {
		out = append(out, s.Target.Name)
	}
	sort.Strings(out)
	return out
}

// TestDifferentialAgainstLegacyEngine runs 10,000 random cases.
func TestDifferentialAgainstLegacyEngine(t *testing.T) {
	cases := 10000
	if testing.Short() {
		cases = 1000
	}
	smelly, mixed := 0, 0
	for seed := uint64(1); seed <= uint64(cases); seed++ {
		g := &gen{r: rand.New(rand.NewPCG(seed, 0x6b70))}
		p := g.pattern()
		// The generated Pattern must be one the current linter accepts.
		raw, err := yaml.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := linter.Lint(raw); err != nil {
			t.Fatalf("seed %d: generated pattern does not lint: %v\n%s", seed, err, raw)
		}
		objs := g.objects(p)
		graph := cluster.NewGraph()
		graph.Build(objs)

		want := legacyRun(graph.GetNodes(), graph, p)
		sort.Strings(want)
		got := smellNames(t, graph, p)
		if !reflect.DeepEqual(got, want) {
			o, _ := yaml.Marshal(objs)
			t.Fatalf("seed %d: smells = %v, legacy = %v\npattern:\n%s\nobjects:\n%s", seed, got, want, raw, o)
		}
		if len(want) > 0 {
			smelly++
			if len(want) < countKind(objs, "T") {
				mixed++
			}
		}
	}
	// Guard against a generator that makes every verdict trivially empty.
	if smelly < cases*12/100 || mixed < cases*3/100 {
		t.Errorf("only %d of %d cases have a Smell (%d with smelly and clean targets): the generator is too weak", smelly, cases, mixed)
	}
	t.Logf("%d cases, %d with at least one Smell, %d with both smelly and clean targets", cases, smelly, mixed)
}

func countKind(objs []unstructured.Unstructured, kind string) int {
	n := 0
	for _, o := range objs {
		if o.GetKind() == kind {
			n++
		}
	}
	return n
}

// The path navigation alone, on many more random objects and paths: values (in order) and
// the raw value used by the ARRAY_SIZE_* filters.
func TestDifferentialPathNavigation(t *testing.T) {
	g := &gen{r: rand.New(rand.NewPCG(42, 0x6b70))}
	for i := 0; i < 50000; i++ {
		obj := g.object("T", "t")
		path := g.path()
		got, gotFound := getFieldValues(obj, path)
		want, wantFound := legacyGetFieldValues(obj, path)
		if gotFound != wantFound || !reflect.DeepEqual(got, want) {
			t.Fatalf("path %q on %v: values %v (%v), legacy %v (%v)", path, obj, got, gotFound, want, wantFound)
		}
		for _, n := range genNumbers {
			for _, op := range []linter.FilterOperator{linter.FilterArraySizeEquals, linter.FilterArraySizeGreaterThan} {
				c := linter.FilterCondition{Path: path, Operator: op, Values: []string{n}}
				u := &unstructured.Unstructured{Object: obj}
				if evalCondition(u, c, nil) != legacyEvalCondition(u, c) {
					t.Fatalf("%s %s %s on %v differs from legacy", path, op, n, obj)
				}
			}
		}
	}
}

// G10 is the one intended difference: EQUALS now compares booleans and numbers by their
// string form, where the legacy filter skipped every non-string value.
func TestTypedEqualsDiffers(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"suspend": true, "replicas": int64(3), "ratio": 0.5, "name": "x",
	}}}
	for _, tc := range []struct {
		path, value string
		now, legacy bool
	}{
		{"spec.suspend", "true", true, false},
		{"spec.replicas", "3", true, false},
		{"spec.ratio", "0.5", true, false},
		{"spec.name", "x", true, true},
		{"spec.suspend", "false", false, false},
	} {
		c := linter.FilterCondition{Path: tc.path, Operator: linter.FilterEquals, Values: []string{tc.value}}
		if got := evalCondition(obj, c, nil); got != tc.now {
			t.Errorf("%s EQUALS %s = %v, want %v", tc.path, tc.value, got, tc.now)
		}
		if got := legacyEvalCondition(obj, c); got != tc.legacy {
			t.Errorf("legacy %s EQUALS %s = %v, want %v", tc.path, tc.value, got, tc.legacy)
		}
	}
}
