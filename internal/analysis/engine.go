package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	kubepatternv1 "kubepattern-go/api/v1"
	"kubepattern-go/internal/cluster"
)

// Engine orchestrates the full analysis lifecycle for a single pattern.
type Engine struct {
	graph  *cluster.Graph
	writer SmellWriter
}

// SmellWriter is the interface that report/ will implement to persist smells on the cluster.
type SmellWriter interface {
	Write(ctx context.Context, smell Smell) error
	// Prune deletes every Smell owned by the pattern identified by patternUID whose
	// CRDName is not in keep. It is called once per Reconcile, after every live
	// smell for that pattern has been written, so it only ever removes smells that
	// are no longer produced by the current state of the cluster.
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
//  5. Create and write a Smell for each target that failed the relationship check
//  6. Prune smells previously emitted by this pattern that are no longer live
func (e *Engine) Run(ctx context.Context, pattern *kubepatternv1.Pattern) error {
	slog.Info("running pattern", "pattern", pattern.Name)

	nodes := e.graph.GetNodes()
	live := make(map[string]struct{})

	// --- Step 1 & 3: fetch and filter target candidates ---
	targets := FilterResources(
		nodes,
		pattern.Spec.Target.Kind,
		pattern.Spec.Target.APIVersion,
		pattern.Spec.Target.Filters,
	)

	if len(targets) == 0 {
		if !pattern.Spec.Target.EmitOnEmpty {
			slog.Info("no target candidates found, skipping pattern", "pattern", pattern.Name)
			return e.writer.Prune(ctx, string(pattern.GetUID()), live)
		}

		// Absence Patterns
		slog.Info("no targets found, emitting synthetic smell", "pattern", pattern.Name)
		smell := buildSyntheticSmell(pattern)
		live[smell.CRDName] = struct{}{}
		if err := e.writer.Write(ctx, smell); err != nil {
			slog.Error("failed to write synthetic smell",
				"pattern", pattern.Name,
				"error", err,
			)
		}
		return e.writer.Prune(ctx, string(pattern.GetUID()), live)
	}

	// --- Step 2 & 3: fetch and filter dependency candidates ---
	// Built once — same candidates for all targets.
	deps := e.buildDependencies(nodes, pattern.Spec.Dependencies)

	// --- Step 4 & 5: evaluate relationships and emit smells ---
	for _, target := range targets {
		satisfied := EvaluateRelationships(target, deps, pattern.Spec.Relationships, e.graph)
		if !satisfied {
			// Relationships are not satisfied — no smell for this target.
			continue
		}

		smell := buildSmell(pattern, target)
		live[smell.CRDName] = struct{}{}
		if err := e.writer.Write(ctx, smell); err != nil {
			slog.Error("failed to write smell",
				"pattern", pattern.Name,
				"target", target.GetName(),
				"error", err,
			)
			// Log and continue — one write failure should not stop the whole analysis.
		}
	}

	// --- Step 6: remove smells this pattern is no longer producing ---
	return e.writer.Prune(ctx, string(pattern.GetUID()), live)
}

// buildDependencies fetches and filters all dependency candidates from the graph.
// Returns a map of depID → filtered candidates.
func (e *Engine) buildDependencies(
	nodes map[types.UID]*unstructured.Unstructured,
	dependencies []kubepatternv1.Dependency,
) map[string][]*unstructured.Unstructured {

	deps := make(map[string][]*unstructured.Unstructured, len(dependencies))

	for _, dep := range dependencies {
		candidates := FilterResources(nodes, dep.Kind, dep.APIVersion, dep.Filters)
		deps[dep.ID] = candidates
		slog.Debug("dependency candidates fetched",
			"id", dep.ID,
			"kind", dep.Kind,
			"count", len(candidates),
		)
	}

	return deps
}

// buildSmell constructs a Smell from the pattern metadata and the failing target.
func buildSmell(pattern *kubepatternv1.Pattern, target *unstructured.Unstructured) Smell {
	return Smell{
		CRDName:        smellCRDName(pattern.Name, target.GetUID()),
		PatternName:    pattern.Name,
		PatternUID:     string(pattern.GetUID()),
		PatternVersion: kubepatternv1.GroupVersion.String(),
		Name:           pattern.Name,
		Category:       pattern.Spec.Category,
		Severity:       pattern.Spec.Severity,
		Message:        interpolateMessage(pattern.Spec.Message, target),
		Reference:      pattern.Spec.Reference,
		Suppress:       false,
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
func buildSyntheticSmell(pattern *kubepatternv1.Pattern) Smell {
	return Smell{
		CRDName:        fmt.Sprintf("%s-empty", pattern.Name),
		PatternName:    pattern.Name,
		PatternUID:     string(pattern.GetUID()),
		PatternVersion: kubepatternv1.GroupVersion.String(),
		Name:           pattern.Name,
		Category:       pattern.Spec.Category,
		Severity:       pattern.Spec.Severity,
		Message:        pattern.Spec.Message,
		Reference:      pattern.Spec.Reference,
		Suppress:       false,
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
// with the actual values from the target resource.
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
