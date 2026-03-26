package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/kube"
	"kubepattern-go/internal/linter"
	"kubepattern-go/internal/registry"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// --- Kubernetes client ---
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		slog.Error("failed to build kubeconfig", "error", err)
		os.Exit(1)
	}

	kubeClient, err := kube.NewClient(restConfig)
	if err != nil {
		slog.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	// --- Step 1: build the graph ---
	slog.Info("fetching cluster resources...")
	resources, err := kubeClient.FetchAll(ctx)
	if err != nil {
		slog.Error("failed to fetch cluster resources", "error", err)
		os.Exit(1)
	}

	graph := cluster.NewGraph()
	graph.Build(resources)
	slog.Info("graph built", "nodes", len(graph.GetNodes()))

	// --- Step 2: fetch patterns from the GitHub registry ---
	slog.Info("fetching patterns from registry...")
	ghConfig := definitions.LoadConfig()
	ghClient := definitions.NewClient(ghConfig)

	rawPatterns, err := ghClient.ReadAllDefinitions()
	if err != nil {
		slog.Error("failed to fetch patterns from registry", "error", err)
		os.Exit(1)
	}
	slog.Info("patterns fetched", "count", len(rawPatterns))

	// --- Step 3: lint patterns ---
	var patterns []*linter.PatternAsCode
	for filename, data := range rawPatterns {
		p, err := linter.Lint(data)
		if err != nil {
			// A malformed pattern is logged and skipped — it does not stop the analysis.
			slog.Warn("skipping invalid pattern", "file", filename, "error", err)
			continue
		}
		patterns = append(patterns, p)
		slog.Info("pattern loaded", "name", p.Metadata.Name)
	}

	if len(patterns) == 0 {
		slog.Warn("no valid patterns found, exiting")
		os.Exit(0)
	}

	// --- Step 4: run analysis ---
	smellWriter, err := kube.NewSmellWriter(restConfig)
	if err != nil {
		slog.Error("failed to create smell writer", "error", err)
		os.Exit(1)
	}

	engine := analysis.NewEngine(graph, smellWriter)
	if err := engine.RunAll(ctx, patterns); err != nil {
		// RunAll collects partial errors — log but do not exit with failure
		// since some patterns may have succeeded.
		slog.Warn("analysis completed with some errors", "error", err)
	}

	slog.Info("analysis complete")
}
