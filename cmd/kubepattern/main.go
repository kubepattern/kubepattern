package main

import (
	"context"
	"kubepattern-go/internal/config"
	"log/slog"
	"os"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/cluster"
	"kubepattern-go/internal/fieldpath"
	"kubepattern-go/internal/kube"
	"kubepattern-go/internal/linter"
)

func main() {
	// runStart is the instant of this run's observations (a Pattern's spec.for is measured against it).
	runStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

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

	// The rate limits must be set before the clients are created: they copy the config.
	restConfig.QPS, restConfig.Burst = appCfg.Client.RateLimits()
	slog.Info("kubernetes client rate limits", "qps", restConfig.QPS, "burst", restConfig.Burst)

	kubeClient, err := kube.NewClient(restConfig)
	if err != nil {
		slog.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
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

	smellWriter := kube.NewSmellWriter(
		kubeClient,
		appCfg.SaveInNamespace,
		appCfg.TargetNamespace,
		string(uuid.NewUUID()),
		runStart,
	)

	// Smells of Patterns that are no longer installed are removed at the end of every run,
	// including the runs that exit early. Smells of installed but skipped patterns are kept.
	installed := kube.InstalledPatternUIDs(rawPatterns)
	pruneOrphans := func() {
		if err := smellWriter.PruneOrphans(ctx, installed); err != nil {
			slog.Warn("failed to prune orphaned smells", "error", err)
		}
	}

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
		pruneOrphans()
		os.Exit(0)
	}

	slog.Info("fetching cluster resources using lazy fetch...")

	var validPatterns []*linter.PatternAsCode
	var allResources []unstructured.Unstructured

	for _, pattern := range patterns {
		res, err := fetchForPattern(ctx, kubeClient, pattern, allResources)

		if len(res) > 0 {
			allResources = append(allResources, res...)
		}

		if err != nil {
			slog.Warn("skipping pattern due to resource fetch failure (e.g. missing RBAC)",
				"pattern", pattern.Metadata.Name,
				"error", err)
			continue
		}

		validPatterns = append(validPatterns, pattern)

		slog.Info("resources fetched for pattern", "pattern", pattern.Metadata.Name)
	}

	patterns = validPatterns

	if len(patterns) == 0 {
		slog.Warn("no patterns can be evaluated due to missing resource access, exiting")
		pruneOrphans()
		os.Exit(0)
	}

	slog.Info("fetched cluster resources", "count", len(allResources))

	graph := cluster.NewGraph()
	graph.Build(allResources)
	slog.Info("graph built", "nodes", len(graph.GetNodes()))

	// --- Step 4: run analysis (each pattern prunes its own stale smells) ---
	engine := analysis.NewEngine(graph, smellWriter)
	if err := engine.RunAll(ctx, patterns); err != nil {
		// RunAll collects partial errors — log but do not exit with failure
		// since some patterns may have succeeded.
		slog.Warn("analysis completed with some errors", "error", err)
	}
	pruneOrphans()

	slog.Info("analysis complete")
}

// fetchForPattern lists the resource types a pattern reads, with their owners, and returns
// the objects not fetched before. Kind wildcards and categories are expanded through
// discovery and recorded in the pattern (Resolved). Dependencies whose types are derived
// from the targets (fromTarget) are resolved after the targets are available; a derived
// type that the API server does not serve has no instances.
func fetchForPattern(ctx context.Context, c *kube.Client, pattern *linter.PatternAsCode, fetched []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	spec := &pattern.Spec
	targetRes, err := c.ExpandRefs(spec.Target.Refs())
	if err != nil {
		return nil, err
	}
	if !spec.Target.IsSingleKind() {
		spec.Target.Resolved = kindSet(targetRes)
	}
	reqResources := targetRes
	for i := range spec.Dependencies {
		dep := &spec.Dependencies[i]
		if dep.FromTarget != nil {
			continue
		}
		depRes, err := c.ExpandRefs(dep.Refs())
		if err != nil {
			return nil, err
		}
		if !dep.IsSingleKind() {
			dep.Resolved = kindSet(depRes)
		}
		reqResources = append(reqResources, depRes...)
	}

	res, err := c.FetchSelectedWithInheritance(reqResources, ctx)
	if err != nil {
		return res, err
	}

	var all []unstructured.Unstructured // the targets are read from everything fetched so far
	for i := range spec.Dependencies {
		dep := &spec.Dependencies[i]
		if dep.FromTarget == nil {
			continue
		}
		if all == nil {
			all = append(append([]unstructured.Unstructured{}, fetched...), res...)
		}
		dep.Resolved = linter.KindSet{}
		for _, r := range derivedResources(all, spec.Target, dep.FromTarget) {
			if r.Resource == "" {
				gvr, err := c.GetGVR(schema.FromAPIVersionAndKind(r.APIVersion, r.Kind))
				if err != nil {
					slog.Warn("derived kind is not served, no instances", "pattern", pattern.Metadata.Name, "apiVersion", r.APIVersion, "kind", r.Kind)
					dep.Resolved.Add(r.APIVersion, r.Kind)
					continue
				}
				r.Resource = gvr.Resource
			}
			dep.Resolved.Add(r.APIVersion, r.Kind)
			more, err := c.FetchSelectedWithInheritance([]kube.Resource{r}, ctx)
			res = append(res, more...)
			if err != nil {
				if apierrors.IsNotFound(err) {
					slog.Warn("derived kind is not served, no instances", "pattern", pattern.Metadata.Name, "apiVersion", r.APIVersion, "kind", r.Kind)
					continue
				}
				return res, err
			}
		}
	}
	return res, nil
}

// derivedResources reads the apiVersion, kind and plural of a fromTarget dependency from
// every object of the target's types.
func derivedResources(objs []unstructured.Unstructured, t linter.Target, ft *linter.FromTarget) []kube.Resource {
	isTarget := analysis.TargetMatcher(t)
	seen := map[string]bool{}
	var out []kube.Resource
	first := func(obj map[string]any, path string) string {
		if path == "" {
			return ""
		}
		for _, v := range fieldpath.MustParse(path).Values(obj, nil) {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		return ""
	}
	for i := range objs {
		o := &objs[i]
		if !isTarget(o) {
			continue
		}
		apiVersion := ft.APIVersion
		if apiVersion == "" {
			apiVersion = first(o.Object, ft.APIVersionPath)
		}
		r := kube.Resource{
			APIVersion: apiVersion,
			Kind:       first(o.Object, ft.KindPath),
			Resource:   first(o.Object, ft.PluralPath),
		}
		if r.APIVersion == "" || r.Kind == "" || seen[r.APIVersion+"|"+r.Kind] {
			continue
		}
		seen[r.APIVersion+"|"+r.Kind] = true
		out = append(out, r)
	}
	return out
}

func kindSet(res []kube.Resource) linter.KindSet {
	set := linter.KindSet{}
	for _, r := range res {
		set.Add(r.APIVersion, r.Kind)
	}
	return set
}
