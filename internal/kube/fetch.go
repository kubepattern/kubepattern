package kube

import (
	"context"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubepattern-go/internal/analysis"
	"kubepattern-go/internal/fieldpath"
	"kubepattern-go/internal/linter"
)

// FetchForPattern lists the resource types a pattern reads, with their owners, and returns
// the objects not fetched before. Kind wildcards and categories are expanded through
// discovery and recorded in the pattern (Resolved). Dependencies whose types are derived
// from the targets (fromTarget) are resolved after the targets are available; a derived
// type that the API server does not serve has no instances.
func (c *Client) FetchForPattern(ctx context.Context, pattern *linter.PatternAsCode, fetched []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
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

	for i := range spec.Dependencies {
		dep := &spec.Dependencies[i]
		if dep.FromTarget == nil {
			continue
		}
		dep.Resolved = linter.KindSet{}
		// The targets are read from everything fetched so far: the objects of earlier
		// patterns and those of this one.
		for _, r := range derivedResources(*spec, dep.FromTarget, fetched, res) {
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
			more, err := c.FetchSelectedWithInheritance([]Resource{r}, ctx)
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
// the target candidates (the target filters apply). With "[@]" in the paths, each element
// names one type, read from that element only.
func derivedResources(spec linter.Spec, ft *linter.FromTarget, objSets ...[]unstructured.Unstructured) []Resource {
	isTarget := analysis.TargetSelector(spec)
	paths := fieldpath.ParseAll(ft.Paths())
	seen := map[string]bool{}
	var out []Resource
	first := func(obj map[string]any, path string, b fieldpath.Binding) string {
		if path == "" {
			return ""
		}
		for _, v := range fieldpath.MustParse(path).Values(obj, b) {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		return ""
	}
	for _, objs := range objSets {
		for i := range objs {
			o := &objs[i]
			if _, ok := isTarget(o); !ok {
				continue
			}
			for _, b := range fieldpath.Enumerate(o.Object, paths, nil) {
				apiVersion := ft.APIVersion
				if apiVersion == "" {
					apiVersion = first(o.Object, ft.APIVersionPath, b)
				}
				r := Resource{
					APIVersion: apiVersion,
					Kind:       first(o.Object, ft.KindPath, b),
					Resource:   first(o.Object, ft.PluralPath, b),
				}
				key := linter.GVKKey(r.APIVersion, r.Kind)
				if r.APIVersion == "" || r.Kind == "" || seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, r)
			}
		}
	}
	return out
}

func kindSet(res []Resource) linter.KindSet {
	set := linter.KindSet{}
	for _, r := range res {
		set.Add(r.APIVersion, r.Kind)
	}
	return set
}
