package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/fieldpath"
	"kubepattern-go/internal/linter"
)

// Engine orchestrates the full analysis lifecycle for a single pattern.
type Engine struct {
	graph  *cluster.Graph
	writer SmellWriter
}

// SmellWriter is the interface that report/ will implement to persist smells on the cluster.
type SmellWriter interface {
	Write(ctx context.Context, smell Smell) error
	// Prune deletes the smells previously produced by the pattern identified by patternUID
	// whose name is not in keep. It is called once per pattern, after the pattern has been
	// evaluated, so a pattern only ever removes smells it no longer produces.
	Prune(ctx context.Context, patternUID string, keep map[string]struct{}) error
}

// NewEngine creates an Engine with the given graph and smell writer.
func NewEngine(graph *cluster.Graph, writer SmellWriter) *Engine {
	return &Engine{
		graph:  graph,
		writer: writer,
	}
}

// Run executes the full analysis pipeline for a single pattern:
//  1. Fetch target candidates from the graph
//  2. Fetch dependency candidates from the graph
//  3. Filter targets by their filters
//  4. Filter targets by their relationships against the dependencies
//  5. Create and write a Smell for each target that satisfies the relationships
//  6. Prune smells previously emitted by this pattern that are no longer live
func (e *Engine) Run(ctx context.Context, pattern *linter.PatternAsCode) error {
	slog.Info("running pattern", "pattern", pattern.Metadata.Name)

	nodes := e.graph.GetNodes()
	// live holds every smell this pattern produces in this run, whether or not its write
	// succeeded: a failed write must never cause a still-valid smell to be pruned.
	live := make(map[string]struct{})

	// --- Step 1 & 3: fetch and filter target candidates ---
	// A target whose paths contain "[@]" is examined element by element: each candidate
	// carries the element bindings that pass the target filters.
	targets := SelectTargets(nodes, pattern.Spec)

	if len(targets) == 0 {
		if !pattern.Spec.Target.EmitOnEmpty {
			slog.Info("no target candidates found, skipping pattern", "pattern", pattern.Metadata.Name)
			return e.prune(ctx, pattern, live)
		}

		// Absence Patterns
		slog.Info("no targets found, emitting synthetic smell", "pattern", pattern.Metadata.Name)
		smell := buildSyntheticSmell(pattern)
		live[smell.CRDName] = struct{}{}
		if err := e.writer.Write(ctx, smell); err != nil {
			slog.Error("failed to write synthetic smell",
				"pattern", pattern.Metadata.Name,
				"error", err,
			)
		}
		return e.prune(ctx, pattern, live)
	}

	// --- Step 2 & 3: fetch and filter dependency candidates ---
	// Built once — same candidates for all targets.
	deps := e.buildDependencies(nodes, pattern.Spec.Dependencies)

	// --- Step 4 & 5: evaluate relationships and emit smells ---
	for _, target := range targets {
		// The target is defective when the relationships hold for at least one of its
		// element bindings (a single, empty binding when the pattern has no "[@]").
		satisfied := false
		for _, tb := range target.Bindings {
			if EvaluateRelationships(target.Object, deps, pattern.Spec.Relationships, e.graph, tb) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			// Relationships are not satisfied — no smell for this target.
			continue
		}

		smell := buildSmell(pattern, target.Object)
		live[smell.CRDName] = struct{}{}
		if err := e.writer.Write(ctx, smell); err != nil {
			slog.Error("failed to write smell",
				"pattern", pattern.Metadata.Name,
				"target", target.Object.GetName(),
				"error", err,
			)
			// Log and continue — one write failure should not stop the whole analysis.
		}
	}

	// --- Step 6: remove smells this pattern is no longer producing ---
	return e.prune(ctx, pattern, live)
}

// prune removes the stale smells of a pattern. A pattern without a UID cannot be matched
// to its smells, so nothing is deleted for it.
func (e *Engine) prune(ctx context.Context, pattern *linter.PatternAsCode, live map[string]struct{}) error {
	if pattern.Metadata.UID == "" {
		slog.Warn("pattern has no UID, skipping prune", "pattern", pattern.Metadata.Name)
		return nil
	}
	return e.writer.Prune(ctx, pattern.Metadata.UID, live)
}

// RunAll runs the analysis pipeline for a slice of patterns.
func (e *Engine) RunAll(ctx context.Context, patterns []*linter.PatternAsCode) error {
	var errs []string

	for _, pattern := range patterns {
		if err := e.Run(ctx, pattern); err != nil {
			errs = append(errs, fmt.Sprintf("pattern %s: %v", pattern.Metadata.Name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("analysis completed with errors:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}

// buildDependencies fetches and filters all dependency candidates from the graph.
// Returns a map of depID → filtered candidates.
func (e *Engine) buildDependencies(
	nodes map[types.UID]*unstructured.Unstructured,
	dependencies []linter.Dependency,
) map[string][]*unstructured.Unstructured {

	deps := make(map[string][]*unstructured.Unstructured, len(dependencies))

	for _, dep := range dependencies {
		candidates := FilterDependency(nodes, dep)
		deps[dep.ID] = candidates
		slog.Debug("dependency candidates fetched",
			"id", dep.ID,
			"kind", dep.Kind,
			"count", len(candidates),
		)
	}

	return deps
}

// TargetCandidate is a target resource with the element bindings that pass the target
// filters.
type TargetCandidate struct {
	Object   *unstructured.Unstructured
	Bindings []fieldpath.Binding
}

// TargetPaths returns every target-side path of a pattern: they define the target's "[@]"
// anchors, shared by the target filters and all relationships.
func TargetPaths(spec linter.Spec) []*fieldpath.Path {
	var out []*fieldpath.Path
	add := func(raw string) {
		if raw != "" {
			out = append(out, fieldpath.MustParse(raw))
		}
	}
	f := spec.Target.Filters
	for _, group := range [][]linter.FilterCondition{f.MatchAll, f.MatchAny, f.MatchNone} {
		for _, c := range group {
			add(c.Path)
			add(c.ValuesFrom)
		}
	}
	r := spec.Relationships
	for _, group := range [][]linter.Relationship{r.MatchAll, r.MatchAny, r.MatchNone} {
		for _, rel := range group {
			for _, c := range rel.Criteria {
				add(c.TargetPath)
				if c.TargetDefault != nil {
					add(c.TargetDefault.Path)
				}
			}
			if rel.Type == linter.RelationshipSelects {
				add(rel.Selector())
			}
		}
	}
	return out
}

// TargetMatcher reports whether a resource is of one of the target's types.
func TargetMatcher(t linter.Target) func(*unstructured.Unstructured) bool {
	if t.IsSingleKind() {
		return func(n *unstructured.Unstructured) bool { return matchKind(n, t.Kind, t.APIVersion) }
	}
	return kindMatcher(t.Refs(), t.Resolved)
}

// SelectTargets returns the target candidates of a pattern: resources of the target's types
// with at least one element binding that passes the target filters.
func SelectTargets(nodes map[types.UID]*unstructured.Unstructured, spec linter.Spec) []TargetCandidate {
	t := spec.Target
	match := TargetMatcher(t)
	paths := TargetPaths(spec)
	var out []TargetCandidate
	for _, node := range nodes {
		if !match(node) {
			continue
		}
		var passing []fieldpath.Binding
		for _, b := range fieldpath.Enumerate(node.Object, paths, nil) {
			if matchFilters(node, t.Filters, b) {
				passing = append(passing, b)
			}
		}
		if len(passing) > 0 {
			out = append(out, TargetCandidate{Object: node, Bindings: passing})
		}
	}
	return out
}

// buildSmell constructs a Smell from the pattern metadata and the failing target.
func buildSmell(pattern *linter.PatternAsCode, target *unstructured.Unstructured) Smell {
	return Smell{
		CRDName:        smellCRDName(pattern.Metadata.Name, target.GetUID()),
		PatternName:    pattern.Metadata.Name,
		PatternUID:     pattern.Metadata.UID,
		PatternVersion: pattern.APIVersion,
		Name:           pattern.Metadata.Name,
		Category:       pattern.Spec.Category,
		Severity:       pattern.Spec.Severity,
		Message:        interpolateMessage(pattern.Spec.Message, target),
		Reference:      pattern.Spec.Reference,
		Suppress:       false,
		For:            pattern.Spec.ForDuration(),
		Target: SmellTarget{
			APIVersion: target.GetAPIVersion(),
			Kind:       target.GetKind(),
			Name:       target.GetName(),
			Namespace:  target.GetNamespace(),
			UID:        string(target.GetUID()),
		},
	}
}

// buildSyntheticSmell builds a Smell without a real target,
// invoked when emitOnEmpty is true, and there are no suitable resources in Cluster.
func buildSyntheticSmell(pattern *linter.PatternAsCode) Smell {
	return Smell{
		CRDName:        fmt.Sprintf("%s-empty", pattern.Metadata.Name),
		PatternName:    pattern.Metadata.Name,
		PatternUID:     pattern.Metadata.UID,
		PatternVersion: pattern.APIVersion,
		Name:           pattern.Metadata.Name,
		Category:       pattern.Spec.Category,
		Severity:       pattern.Spec.Severity,
		Message:        pattern.Spec.Message,
		Reference:      pattern.Spec.Reference,
		Suppress:       false,
		For:            pattern.Spec.ForDuration(),
		Target: SmellTarget{
			Kind: pattern.Spec.Target.Kind,
		},
	}
}

// smellCRDName returns the deterministic CRD name for a smell.
func smellCRDName(patternName string, uid types.UID) string {
	return fmt.Sprintf("%s-%s", patternName, uid)
}

// interpolateMessage replaces {{target.metadata.X}} placeholders in the message
// with the actual values from the target resource. The {{smell.since}} and {{smell.phase}}
// placeholders are left to the writer, which knows the Smell's first observation.
func interpolateMessage(message string, target *unstructured.Unstructured) string {
	replacements := map[string]string{
		"{{target.metadata.name}}":      target.GetName(),
		"{{target.metadata.namespace}}": target.GetNamespace(),
		"{{target.metadata.uid}}":       string(target.GetUID()),
		"{{target.kind}}":               target.GetKind(),
		"{{target.apiVersion}}":         target.GetAPIVersion(),
	}

	for placeholder, value := range replacements {
		message = strings.ReplaceAll(message, placeholder, value)
	}

	return message
}
