package main

import (
	"context"
	"kubepattern-go/internal/config"
	"log/slog"
	"os"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/kube"
	"kubepattern-go/internal/linter"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// --- Kubernetes client ---
	// 1. In-Cluster config
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		slog.Info("in-cluster config not found, falling back to kubeconfig")

		// 2. Fallback to Out-Of-Cluster config
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}

		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			slog.Error("failed to build kubeconfig", "error", err)
			os.Exit(1)
		}
	}

	kubeClient, err := kube.NewClient(restConfig)
	if err != nil {
		slog.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	// --- Step 0: Load App Configuration ---
	configPath := "/app/config/config.yaml"
	// Fallback per test in locale
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.yaml"
	}

	appCfg, err := config.Load(configPath)
	if err != nil {
		slog.Warn("config file not found or invalid, using defaults", "error", err)
		appCfg = &config.AppConfig{}
	} else {
		slog.Info("configuration loaded successfully")
	}

	// --- Step 1: fetch patterns from the Kubernetes registry ---
	var rawPatterns map[string][]byte

	slog.Info("Fetching patterns from Kubernetes registry...")
	rawPatterns, err = kubeClient.ReadAllDefinitions(ctx)
	if err != nil {
		slog.Error("failed to fetch patterns from cluster", "error", err)
		os.Exit(1)
	}

	slog.Info("patterns fetched successfully", "count", len(rawPatterns))

	// --- Step 2: lint patterns ---
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

	var listResourcesGroup []kube.Resource

	for _, pattern := range patterns {
		rg := kube.Resource{
			APIVersion: pattern.Spec.Target.APIVersion,
			Kind:       pattern.Spec.Target.Kind,
			Resource:   pattern.Spec.Target.PluralName,
		}
		listResourcesGroup = append(listResourcesGroup, rg)

		for _, dependency := range pattern.Spec.Dependencies {
			rg := kube.Resource{
				APIVersion: dependency.APIVersion,
				Kind:       dependency.Kind,
				Resource:   dependency.PluralName,
			}

			listResourcesGroup = append(listResourcesGroup, rg)
		}
	}
	slog.Info("Fetching this GroupVersionKind")
	for _, rg := range listResourcesGroup {
		str := "- " + rg.APIVersion + "/" + rg.Kind + "/"
		slog.Info(str)
	}

	// --- Step 3: lazy fetching ---
	slog.Info("fetching cluster resources using lazy fetch...")

	resources, err := kubeClient.FetchSelectedWithInheritance(listResourcesGroup, ctx)
	if err != nil {
		slog.Error("failed to fetch cluster resources", "error", err)
		os.Exit(1)
	}

	slog.Info("fetched cluster resources", "count", len(resources), "resources", len(resources))

	graph := cluster.NewGraph()
	graph.Build(resources)
	slog.Info("graph built", "nodes", len(graph.GetNodes()))

	graph.PrintGraphviz()

	// --- Step 4: run analysis ---
	smellWriter := kube.NewSmellWriter(
		kubeClient,
		appCfg.Analysis.SaveInNamespace,
		appCfg.Analysis.TargetNamespace,
	)

	engine := analysis.NewEngine(graph, smellWriter)
	if err := engine.RunAll(ctx, patterns); err != nil {
		// RunAll collects partial errors — log but do not exit with failure
		// since some patterns may have succeeded.
		slog.Warn("analysis completed with some errors", "error", err)
	}

	slog.Info("analysis complete")
}
