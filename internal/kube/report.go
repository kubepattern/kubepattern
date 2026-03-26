package kube

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"kubepattern-go/internal/analysis"
)

const (
	smellGroup       = "kubepattern.dev"
	smellVersion     = "v1"
	smellResource    = "smells"
	defaultNamespace = "kubepattern-analysis-ns"
)

var smellGVR = schema.GroupVersionResource{
	Group:    smellGroup,
	Version:  smellVersion,
	Resource: smellResource,
}

// SmellWriter writes Smell CRDs to the Kubernetes cluster.
type SmellWriter struct {
	client          dynamic.Interface
	saveInNamespace bool
	targetNamespace string
}

// NewSmellWriter creates a SmellWriter using the provided REST config and namespace preferences.
func NewSmellWriter(config *rest.Config, saveInNamespace bool, targetNamespace string) (*SmellWriter, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client for smell writer: %w", err)
	}
	return &SmellWriter{
		client:          dynClient,
		saveInNamespace: saveInNamespace,
		targetNamespace: targetNamespace,
	}, nil
}

// Write persists a Smell as a CRD.
// The namespace is chosen based on the writer's configuration.
func (w *SmellWriter) Write(ctx context.Context, smell analysis.Smell) error {
	var namespace string

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

	// 1. Fetch the existing resource to get its resourceVersion
	existing, err := w.client.Resource(smellGVR).Namespace(namespace).Get(ctx, smell.CRDName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// 2a. Smell does not exist yet — create it.
			_, err = w.client.Resource(smellGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
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

	_, err = w.client.Resource(smellGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update smell %q: %w", smell.CRDName, err)
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
					"namespace":  smell.Target.Namespace, // Mantiene l'info originale per debug
					"uid":        smell.Target.UID,
				},
			},
		},
	}
}
