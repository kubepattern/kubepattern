package kube

import (
	"context"
	"fmt"
	"kubepattern-go/internal/analysis"
	"log/slog"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	smellGroup    = "kubepattern.dev"
	smellVersion  = "v1"
	smellResource = "smells"

	// lastScanLabel records the id of the run that last wrote the smell (traceability only).
	lastScanLabel = "lastScan"
	// patternUIDLabel tags every Smell with the UID of the Pattern that produced it, so each
	// pattern prunes only its own stale smells and never those of other patterns.
	patternUIDLabel = "kubepattern.dev/pattern-uid"
)

var smellGVR = schema.GroupVersionResource{
	Group:    smellGroup,
	Version:  smellVersion,
	Resource: smellResource,
}

// SmellWriter writes Smell CRDs to the Kubernetes cluster.
type SmellWriter struct {
	client          *Client
	saveInNamespace bool
	targetNamespace string
	scanId          string
}

// NewSmellWriter creates a SmellWriter using the existing kube.Client and namespace preferences.
func NewSmellWriter(client *Client, saveInNamespace bool, targetNamespace, scanId string) *SmellWriter {
	return &SmellWriter{
		client:          client,
		saveInNamespace: saveInNamespace,
		targetNamespace: targetNamespace,
		scanId:          scanId,
	}
}

// Write persists a Smell as a CRD.
// The namespace is chosen based on the writer's configuration.
func (w *SmellWriter) Write(ctx context.Context, smell analysis.Smell) error {
	var namespace string
	slog.Info("Writing smell.")

	if w.saveInNamespace {
		namespace = smell.Target.Namespace

		// Fallback: cluster-scoped resources (es. Node, Namespace) to use targetNamespace.
		if namespace == "" {
			namespace = w.targetNamespace
		}
	} else {
		namespace = w.targetNamespace
	}

	obj := toUnstructured(smell, namespace)

	dynClient := w.client.DynamicClient()

	// 1. Fetch the existing resource to get its resourceVersion in O(1) time
	existing, err := dynClient.Resource(smellGVR).Namespace(namespace).Get(ctx, smell.CRDName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// 2a. Smell does not exist yet — create it.
			obj.SetLabels(map[string]string{lastScanLabel: w.scanId, patternUIDLabel: smell.PatternUID})
			_, err = dynClient.Resource(smellGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create smell %q: %w", smell.CRDName, err)
			}
			return nil
		}
		// 2b. A different error occurred during Get
		return fmt.Errorf("failed to get smell %q: %w", smell.CRDName, err)
	}

	// 3. Smell already exists — set the required resourceVersion before updating
	obj.SetResourceVersion(existing.GetResourceVersion())

	labels := existing.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[lastScanLabel] = w.scanId
	labels[patternUIDLabel] = smell.PatternUID
	obj.SetLabels(labels)

	_, err = dynClient.Resource(smellGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update smell %q: %w", smell.CRDName, err)
	}
	slog.Info("Writing smell complete.")
	return nil
}

// Prune deletes every Smell labeled as owned by patternUID whose name is not in keep.
// It is called once per pattern, after all its live smells have been evaluated, so a
// pattern only ever removes smells that it produced before and no longer produces.
// Patterns skipped in this run (e.g. missing RBAC) are never pruned, and the run context
// bounds the deletions: once the run deadline is exceeded, no further smell is deleted.
func (w *SmellWriter) Prune(ctx context.Context, patternUID string, keep map[string]struct{}) error {
	res := w.client.dynamicClient.Resource(smellGVR)

	list, err := res.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", patternUIDLabel, patternUID),
	})
	if err != nil {
		return fmt.Errorf("failed to list smells for pattern %q: %w", patternUID, err)
	}

	var errs []string
	for _, obj := range list.Items {
		if _, ok := keep[obj.GetName()]; ok {
			continue
		}
		if err := res.Namespace(obj.GetNamespace()).Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
			slog.Error("Failed to delete stale smell", "name", obj.GetName(), "error", err)
			errs = append(errs, fmt.Sprintf("%s: %v", obj.GetName(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to prune %d stale smell(s): %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// PruneOrphans deletes the smells whose producing Pattern is no longer installed: smells
// labeled with a pattern UID not in installed, and smells without the label (written by a
// version that did not label them and not re-written since).
func (w *SmellWriter) PruneOrphans(ctx context.Context, installed map[string]struct{}) error {
	res := w.client.dynamicClient.Resource(smellGVR)

	list, err := res.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list smells: %w", err)
	}

	var errs []string
	for _, obj := range list.Items {
		if _, ok := installed[obj.GetLabels()[patternUIDLabel]]; ok {
			continue
		}
		if err := res.Namespace(obj.GetNamespace()).Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
			slog.Error("Failed to delete orphaned smell", "name", obj.GetName(), "error", err)
			errs = append(errs, fmt.Sprintf("%s: %v", obj.GetName(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to prune %d orphaned smell(s): %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// toUnstructured converts a Smell into an unstructured Kubernetes object
func toUnstructured(smell analysis.Smell, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": smellGroup + "/" + smellVersion,
			"kind":       "Smell",
			"metadata": map[string]any{
				"name":      smell.CRDName,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"name":      smell.Name,
				"category":  smell.Category,
				"message":   smell.Message,
				"severity":  string(smell.Severity),
				"reference": smell.Reference,
				"suppress":  smell.Suppress,
				"pattern": map[string]any{
					"name":    smell.PatternName,
					"version": smell.PatternVersion,
				},
				"target": map[string]any{
					"apiVersion": smell.Target.APIVersion,
					"kind":       smell.Target.Kind,
					"name":       smell.Target.Name,
					"namespace":  smell.Target.Namespace,
					"uid":        smell.Target.UID,
				},
			},
		},
	}
}
