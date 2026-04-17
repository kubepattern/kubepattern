package kube

import (
	"context"
	"fmt"
	"kubepattern-go/internal/analysis"
	"log/slog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	smellGroup       = "kubepattern.dev"
	smellVersion     = "v1"
	smellResource    = "smells"
	defaultNamespace = "default"
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

	if namespace == "" {
		namespace = defaultNamespace
	}

	obj := toUnstructured(smell, namespace)

	dynClient := w.client.DynamicClient()

	// 1. Fetch the existing resource to get its resourceVersion in O(1) time
	existing, err := dynClient.Resource(smellGVR).Namespace(namespace).Get(ctx, smell.CRDName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// 2a. Smell does not exist yet — create it.
			obj.SetLabels(map[string]string{"lastScan": w.scanId})
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

	labels["lastScan"] = w.scanId
	obj.SetLabels(labels)

	_, err = dynClient.Resource(smellGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update smell %q: %w", smell.CRDName, err)
	}
	slog.Info("Writing smell complete.")
	return nil
}

// CleanOldScans removes smells that have been solved or their pattern is no longer installed
func (w *SmellWriter) CleanOldScans() {
	res := w.client.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    smellGroup,
		Version:  smellVersion,
		Resource: smellResource,
	})

	list, err := res.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, obj := range list.Items {
		labels := obj.GetLabels()

		if labels == nil || labels["lastScan"] != w.scanId {
			err := res.Namespace(obj.GetNamespace()).Delete(context.Background(), obj.GetName(), metav1.DeleteOptions{})
			if err != nil {
				slog.Error("Failed to delete old smell", "name", obj.GetName(), "error", err)
			}
		}
	}
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
