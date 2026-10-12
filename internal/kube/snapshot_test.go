package kube

// Ground truth on recorded cluster snapshots, with no cluster: a snapshot holds the
// discovery documents and every object that the evaluated Patterns fetched on kp-eval or
// kp-krateo (snapshot_record_test.go). The test serves them through fake discovery and
// dynamic clients, runs the whole pipeline of main (lint, FetchForPattern, graph, engine)
// and scores every row of the evaluation's ground truth:
//   - the original Patterns must give the predicted engine verdict (engine_expected);
//   - the v2 Patterns must give the true verdict (truth) on every limitation probe.
//
// The snapshots, Patterns and ground truth are a copy of the replication package
// (testdata/replication/README.md).

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/linter"
)

// evalDir holds the copy of the replication package used by the snapshot tests.
const evalDir = "testdata/replication"

// snapshot is the recorded state of a cluster.
type snapshot struct {
	Context   string                    `json:"context"`
	Recorded  string                    `json:"recorded"`
	Engine    string                    `json:"engine"`
	Resources []*metav1.APIResourceList `json:"resources"` // every served group version, preferred first per group
	Preferred []*metav1.APIResourceList `json:"preferred"`
	Objects   []snapshotObject          `json:"objects"`
}

type snapshotObject struct {
	Group    string                 `json:"group"`
	Version  string                 `json:"version"`
	Resource string                 `json:"resource"`
	Object   map[string]interface{} `json:"object"`
}

func loadSnapshot(t *testing.T, path string) *snapshot {
	t.Helper()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Skipf("no snapshot at %s (record it with -tags snapshot)", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var s snapshot
	if err := json.NewDecoder(gz).Decode(&s); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return &s
}

// client serves the snapshot through fake clients: discovery, a RESTMapper built from it,
// and a dynamic client whose objects are stored under their recorded resource.
func (s *snapshot) client(t *testing.T) *Client {
	t.Helper()
	disco := fakeDiscovery{
		FakeDiscovery: &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: s.Resources}},
		preferred:     s.Preferred,
	}
	groupResources, err := restmapper.GetAPIGroupResources(disco)
	if err != nil {
		t.Fatal(err)
	}
	listKinds := map[schema.GroupVersionResource]string{}
	for _, l := range s.Resources {
		gv, err := schema.ParseGroupVersion(l.GroupVersion)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range l.APIResources {
			if !strings.Contains(r.Name, "/") {
				listKinds[gv.WithResource(r.Name)] = r.Kind + "List"
			}
		}
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	for _, o := range s.Objects {
		u := &unstructured.Unstructured{Object: o.Object}
		gvr := schema.GroupVersionResource{Group: o.Group, Version: o.Version, Resource: o.Resource}
		if err := dyn.Tracker().Create(gvr, u, u.GetNamespace()); err != nil {
			t.Fatalf("%s %s/%s: %v", gvr, u.GetNamespace(), u.GetName(), err)
		}
	}
	return newClient(disco, dyn, restmapper.NewDiscoveryRESTMapper(groupResources))
}

// loadPatterns lints the Pattern files of dirs (non-recursive, as kubectl apply -f dir/);
// a later file replaces an earlier one with the same name (the v2 overlay).
func loadPatterns(t *testing.T, dirs ...string) []*linter.PatternAsCode {
	t.Helper()
	byName := map[string]*linter.PatternAsCode{}
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			p, err := linter.Lint(data)
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			p.Metadata.UID = "uid-" + p.Metadata.Name
			byName[p.Metadata.Name] = p
		}
	}
	var out []*linter.PatternAsCode
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metadata.Name < out[j].Metadata.Name })
	return out
}

// platformDirs returns dir/<platform> for every platform directory not starting with "_".
func platformDirs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "_") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// smellKey identifies a Smell as the ground truth does.
type smellKey struct{ pattern, kind, namespace, name string }

func (k smellKey) String() string {
	return fmt.Sprintf("%s %s %s/%s", k.pattern, k.kind, k.namespace, k.name)
}

type keyWriter struct{ smells map[smellKey]bool }

func (w *keyWriter) Write(_ context.Context, s analysis.Smell) error {
	w.smells[smellKey{s.PatternName, s.Target.Kind, s.Target.Namespace, s.Target.Name}] = true
	return nil
}

func (w *keyWriter) Prune(context.Context, string, map[string]struct{}) error { return nil }

// runPipeline mirrors main: fetch per Pattern (a failed fetch skips the Pattern), one graph,
// then the engine. It returns the Smells and the skipped Patterns.
func runPipeline(t *testing.T, c *Client, patterns []*linter.PatternAsCode) (map[smellKey]bool, []string) {
	t.Helper()
	ctx := context.Background()
	var all []unstructured.Unstructured
	var valid []*linter.PatternAsCode
	var skipped []string
	for _, p := range patterns {
		res, err := c.FetchForPattern(ctx, p, all)
		all = append(all, res...)
		if err != nil {
			skipped = append(skipped, p.Metadata.Name+": "+err.Error())
			continue
		}
		valid = append(valid, p)
	}
	g := cluster.NewGraph()
	g.Build(all)
	w := &keyWriter{smells: map[smellKey]bool{}}
	e := analysis.NewEngine(g, w)
	for _, p := range valid {
		if err := e.Run(ctx, p); err != nil {
			t.Fatalf("%s: %v", p.Metadata.Name, err)
		}
	}
	return w.smells, skipped
}

type truthRow struct {
	key                          smellKey
	truth, engineExpected, class string
}

func loadTruth(t *testing.T, files ...string) []truthRow {
	t.Helper()
	var rows []truthRow
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(fh)
		header, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		col := map[string]int{}
		for i, h := range header {
			col[h] = i
		}
		for {
			rec, err := r.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			rows = append(rows, truthRow{
				key:            smellKey{rec[col["pattern"]], rec[col["kind"]], rec[col["namespace"]], rec[col["name"]]},
				truth:          rec[col["truth"]],
				engineExpected: rec[col["engine_expected"]],
				class:          rec[col["class"]],
			})
		}
		fh.Close()
	}
	return rows
}

// score is the outcome of a suite.
type score struct {
	tp, fp, fn, tn           int
	probes, probesAsExpected int
	probesTrue               int
	failures                 []string
	unlisted                 []string
	verdicts                 map[smellKey]bool // observed verdict of every scored row
}

// scoreSmells scores the Smells like scripts/score.py. probeOracle decides what a limitation
// probe must give: "engine" (engine_expected) or "truth"; probe-scope rows always expect
// engine_expected (they are gaps of the given Patterns, kept as given).
func scoreSmells(rows []truthRow, smells map[smellKey]bool, probeOracle string) score {
	s := score{verdicts: map[smellKey]bool{}}
	listed := map[smellKey]bool{}
	patterns := map[string]bool{}
	for _, r := range rows {
		listed[r.key] = true
		patterns[r.key.pattern] = true
		if r.class == "blocked-upstream" {
			continue
		}
		observed := smells[r.key]
		s.verdicts[r.key] = observed
		obs := map[bool]string{true: "smell", false: "clean"}[observed]
		switch r.class {
		case "positive", "negative", "near-miss":
			switch {
			case r.truth == "smell" && observed:
				s.tp++
			case r.truth == "smell":
				s.fn++
			case observed:
				s.fp++
			default:
				s.tn++
			}
			if obs != r.truth {
				s.failures = append(s.failures, fmt.Sprintf("%s %v: truth %s, observed %s", r.class, r.key, r.truth, obs))
			}
		default: // probe-G*, probe-scope
			s.probes++
			want := r.engineExpected
			if probeOracle == "truth" && r.class != "probe-scope" {
				want = r.truth
			}
			if obs == r.engineExpected {
				s.probesAsExpected++
			}
			if obs == r.truth {
				s.probesTrue++
			}
			if obs != want {
				s.failures = append(s.failures, fmt.Sprintf("%s %v: want %s (truth %s, engine_expected %s), observed %s", r.class, r.key, want, r.truth, r.engineExpected, obs))
			}
		}
	}
	for k := range smells {
		if patterns[k.pattern] && !listed[k] {
			s.unlisted = append(s.unlisted, k.String())
		}
	}
	sort.Strings(s.unlisted)
	return s
}

func (s score) report(t *testing.T, suite string, nSmells int) {
	t.Logf("%s: %d Smells, TP %d FP %d FN %d TN %d, probes %d (as engine_expected %d, true %d), unlisted %d",
		suite, nSmells, s.tp, s.fp, s.fn, s.tn, s.probes, s.probesAsExpected, s.probesTrue, len(s.unlisted))
	for _, f := range s.failures {
		t.Errorf("%s: %s", suite, f)
	}
	for _, u := range s.unlisted {
		t.Errorf("%s: unlisted Smell %s", suite, u)
	}
}

type suite struct {
	name, snapshot, oracle string
	patternDirs            []string
	truth                  []string
}

func evalSuites(t *testing.T) []suite {
	rq2Truth, _ := filepath.Glob(filepath.Join(evalDir, "scenarios", "*", "ground-truth.csv"))
	v1 := platformDirs(t, filepath.Join(evalDir, "patterns"))
	v2 := append(append([]string{}, v1...), platformDirs(t, filepath.Join(evalDir, "patterns-v2"))...)
	krateo := filepath.Join(evalDir, "krateo")
	krateoTruth := filepath.Join(krateo, "scenario", "ground-truth.csv")
	return []suite{
		{"rq2-v1", "kp-eval", "engine", v1, rq2Truth},
		{"rq2-v2", "kp-eval", "truth", v2, rq2Truth},
		{"krateo-v1", "kp-krateo", "engine", []string{filepath.Join(krateo, "patterns")}, []string{krateoTruth}},
		{"krateo-v2", "kp-krateo", "truth", []string{filepath.Join(krateo, "patterns"), filepath.Join(krateo, "patterns-v2")},
			[]string{krateoTruth, filepath.Join(krateo, "scenario", "ground-truth-kyverno-only.csv")}},
	}
}

func snapshotPath(name string) string {
	return filepath.Join(evalDir, "snapshot", name+".json.gz")
}

// TestSnapshotGroundTruth scores the original and the v2 Patterns on both snapshots.
func TestSnapshotGroundTruth(t *testing.T) {
	for _, s := range evalSuites(t) {
		t.Run(s.name, func(t *testing.T) {
			snap := loadSnapshot(t, snapshotPath(s.snapshot))
			smells, skipped := runPipeline(t, snap.client(t), loadPatterns(t, s.patternDirs...))
			for _, sk := range skipped {
				t.Errorf("pattern skipped: %s", sk)
			}
			sc := scoreSmells(loadTruth(t, s.truth...), smells, s.oracle)
			sc.report(t, s.name, len(smells))
		})
	}
}

// ablations replace one construct of every v2 Pattern with what the original DSL could
// express instead. Each limitation probe resolved by the v2 Patterns must revert to the
// predicted engine verdict under at least one ablation of its class: the construct is
// necessary, and the probe detects its absence.
//   - "[@]" becomes "[*]" (uncorrelated elements);
//   - a default path becomes the path itself (the fallback field read unconditionally, e.g.
//     the referrer's namespace instead of an explicit override); a literal default is dropped;
//   - split, regex and lowercase transforms are dropped (the raw string is compared); a
//     criterion with an apiGroup transform is dropped (the group could not be read at all);
//   - selector relationships, the owner filter, typed EQUALS filters and the conditions or
//     criteria on quoted keys are dropped;
//   - a list of kinds keeps its first kind, and a dependency derived from the targets is
//     dropped (one concrete kind each, as the original DSL required).
var ablations = []struct {
	name    string
	classes []string // probe classes this construct resolves
	apply   func(*linter.PatternAsCode)
}{
	{"[@] -> [*]", []string{"probe-G1", "probe-G2"}, func(p *linter.PatternAsCode) {
		mapPaths(p, func(s string) string { return strings.ReplaceAll(s, "[@]", "[*]") })
	}},
	{"no defaults", []string{"probe-G3"}, func(p *linter.PatternAsCode) {
		eachCriterion(p, func(c *linter.Criteria) {
			if d := c.TargetDefault; d != nil && d.Path != "" {
				c.TargetPath = d.Path
			}
			if d := c.DependencyDefault; d != nil && d.Path != "" {
				c.DependencyPath = d.Path
			}
			c.TargetDefault, c.DependencyDefault = nil, nil
		})
	}},
	{"no transforms", []string{"probe-G4"}, func(p *linter.PatternAsCode) {
		dropCriteria(p, func(c linter.Criteria) bool {
			return (c.TargetTransform != nil && c.TargetTransform.APIGroup) || (c.DependencyTransform != nil && c.DependencyTransform.APIGroup)
		})
		eachCriterion(p, func(c *linter.Criteria) { c.TargetTransform, c.DependencyTransform = nil, nil })
	}},
	{"no selectors", []string{"probe-G7"}, func(p *linter.PatternAsCode) {
		keep := func(rels []linter.Relationship) []linter.Relationship {
			var out []linter.Relationship
			for _, r := range rels {
				if r.Type != linter.RelationshipSelects && r.Type != linter.RelationshipSelectedBy {
					out = append(out, r)
				}
			}
			return out
		}
		r := &p.Spec.Relationships
		r.MatchAll, r.MatchAny, r.MatchNone = keep(r.MatchAll), keep(r.MatchAny), keep(r.MatchNone)
	}},
	{"no quoted keys", []string{"probe-G5", "probe-G7"}, func(p *linter.PatternAsCode) {
		quoted := func(path string) bool { return strings.Contains(path, "['") || strings.Contains(path, `["`) }
		dropConditions(p, func(c linter.FilterCondition) bool { return quoted(c.Path) || quoted(c.ValuesFrom) })
		dropCriteria(p, func(c linter.Criteria) bool { return quoted(c.TargetPath) || quoted(c.DependencyPath) })
	}},
	{"no kind sets", []string{"probe-G6"}, func(p *linter.PatternAsCode) {
		// One concrete kind per target or dependency, as the original DSL required; a
		// dependency whose kinds were derived from the targets has no instances.
		t := &p.Spec.Target
		if len(t.Kinds) > 0 {
			t.Kind, t.APIVersion, t.PluralName, t.Kinds = t.Kinds[0].Kind, t.Kinds[0].APIVersion, t.Kinds[0].PluralName, nil
		}
		for i := range p.Spec.Dependencies {
			d := &p.Spec.Dependencies[i]
			if len(d.Kinds) > 0 {
				d.Kind, d.APIVersion, d.PluralName, d.Kinds = d.Kinds[0].Kind, d.Kinds[0].APIVersion, d.Kinds[0].PluralName, nil
			}
			if d.FromTarget != nil {
				// No instances: a served kind with a filter that no object passes.
				d.FromTarget, d.Kind, d.APIVersion, d.PluralName = nil, "Namespace", "v1", "namespaces"
				d.Filters = linter.Filters{MatchAll: []linter.FilterCondition{{Path: "metadata.uid", Operator: linter.FilterIsEmpty}}}
			}
		}
	}},
	{"no owner filter", []string{"probe-G8"}, func(p *linter.PatternAsCode) {
		dropConditions(p, func(c linter.FilterCondition) bool { return c.Path == "metadata.ownerReferences" })
	}},
	{"no typed EQUALS", []string{"probe-G10"}, func(p *linter.PatternAsCode) {
		dropConditions(p, func(c linter.FilterCondition) bool {
			if c.Operator != linter.FilterEquals || len(c.Values) == 0 {
				return false
			}
			for _, v := range c.Values {
				if v != "true" && v != "false" {
					return false
				}
			}
			return true
		})
	}},
}

func eachCriterion(p *linter.PatternAsCode, f func(*linter.Criteria)) {
	r := &p.Spec.Relationships
	for _, group := range [][]linter.Relationship{r.MatchAll, r.MatchAny, r.MatchNone} {
		for i := range group {
			for j := range group[i].Criteria {
				f(&group[i].Criteria[j])
			}
		}
	}
}

func dropCriteria(p *linter.PatternAsCode, drop func(linter.Criteria) bool) {
	r := &p.Spec.Relationships
	for _, group := range [][]linter.Relationship{r.MatchAll, r.MatchAny, r.MatchNone} {
		for i := range group {
			var keep []linter.Criteria
			for _, c := range group[i].Criteria {
				if !drop(c) {
					keep = append(keep, c)
				}
			}
			group[i].Criteria = keep
		}
	}
}

func eachFilters(p *linter.PatternAsCode, f func(*linter.Filters)) {
	f(&p.Spec.Target.Filters)
	for i := range p.Spec.Dependencies {
		f(&p.Spec.Dependencies[i].Filters)
	}
}

func dropConditions(p *linter.PatternAsCode, drop func(linter.FilterCondition) bool) {
	keep := func(cs []linter.FilterCondition) []linter.FilterCondition {
		var out []linter.FilterCondition
		for _, c := range cs {
			if !drop(c) {
				out = append(out, c)
			}
		}
		return out
	}
	eachFilters(p, func(f *linter.Filters) {
		f.MatchAll, f.MatchAny, f.MatchNone = keep(f.MatchAll), keep(f.MatchAny), keep(f.MatchNone)
	})
}

// mapPaths rewrites every path of a Pattern.
func mapPaths(p *linter.PatternAsCode, m func(string) string) {
	eachFilters(p, func(f *linter.Filters) {
		for _, cs := range [][]linter.FilterCondition{f.MatchAll, f.MatchAny, f.MatchNone} {
			for i := range cs {
				cs[i].Path, cs[i].ValuesFrom = m(cs[i].Path), m(cs[i].ValuesFrom)
			}
		}
	})
	eachCriterion(p, func(c *linter.Criteria) {
		c.TargetPath, c.DependencyPath = m(c.TargetPath), m(c.DependencyPath)
		for _, d := range []*linter.ValueDefault{c.TargetDefault, c.DependencyDefault} {
			if d != nil {
				d.Path = m(d.Path)
			}
		}
	})
	r := &p.Spec.Relationships
	for _, group := range [][]linter.Relationship{r.MatchAll, r.MatchAny, r.MatchNone} {
		for i := range group {
			group[i].SelectorPath = m(group[i].SelectorPath)
		}
	}
	for i := range p.Spec.Dependencies {
		if ft := p.Spec.Dependencies[i].FromTarget; ft != nil {
			ft.APIVersionPath, ft.KindPath, ft.PluralPath = m(ft.APIVersionPath), m(ft.KindPath), m(ft.PluralPath)
		}
	}
}

// TestSnapshotAblation removes each construct from the v2 Patterns and checks that the
// probes it resolves revert to the predicted engine verdict.
func TestSnapshotAblation(t *testing.T) {
	for _, s := range evalSuites(t) {
		if s.oracle != "truth" {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			snap := loadSnapshot(t, snapshotPath(s.snapshot))
			rows := loadTruth(t, s.truth...)
			base, _ := runPipeline(t, snap.client(t), loadPatterns(t, s.patternDirs...))
			var v2Dirs []string
			for _, d := range s.patternDirs {
				if strings.Contains(d, "patterns-v2") {
					v2Dirs = append(v2Dirs, d)
				}
			}
			v2 := map[string]bool{}
			for _, p := range loadPatterns(t, v2Dirs...) {
				v2[p.Metadata.Name] = true
			}
			// The limitation probes the v2 Patterns resolve.
			resolved := map[smellKey]truthRow{}
			for _, r := range rows {
				if strings.HasPrefix(r.class, "probe-G") && r.truth != r.engineExpected && base[r.key] == (r.truth == "smell") {
					resolved[r.key] = r
				}
			}
			revertedBy := map[smellKey][]string{}
			for _, ab := range ablations {
				patterns := loadPatterns(t, s.patternDirs...)
				for _, p := range patterns {
					if v2[p.Metadata.Name] {
						ab.apply(p)
					}
				}
				ablated, _ := runPipeline(t, snap.client(t), patterns)
				reverted, checked := 0, 0
				for k, r := range resolved {
					if !slices.Contains(ab.classes, r.class) {
						continue
					}
					checked++
					if ablated[k] == (r.engineExpected == "smell") {
						reverted++
						revertedBy[k] = append(revertedBy[k], ab.name)
					}
				}
				var changed []string
				for k := range ablated {
					if _, probe := resolved[k]; !base[k] && !probe {
						changed = append(changed, "+"+k.String())
					}
				}
				for k := range base {
					if _, probe := resolved[k]; !ablated[k] && !probe {
						changed = append(changed, "-"+k.String())
					}
				}
				sort.Strings(changed)
				t.Logf("%s: %d/%d probes of its classes reverted; other Smells changed: %v", ab.name, reverted, checked, changed)
			}
			for k, r := range resolved {
				if len(revertedBy[k]) == 0 {
					t.Errorf("%s %v: no ablation of its class reverts it to %s", r.class, k, r.engineExpected)
				}
			}
			t.Logf("%d resolved probes, each reverted by the ablation of its construct", len(resolved))
		})
	}
}
