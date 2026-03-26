package registry

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type KubernetesFetcher struct {
	client dynamic.Interface
}

// NewKubernetesFetcher initializes a client capable of reading Custom Resources.
func NewKubernetesFetcher(config *rest.Config) (*KubernetesFetcher, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &KubernetesFetcher{client: dynClient}, nil
}

// ReadAllDefinitions implements the Fetcher interface.
func (k *KubernetesFetcher) ReadAllDefinitions(ctx context.Context) (map[string][]byte, error) {
	gvr := schema.GroupVersionResource{
		Group:    "kubepattern.dev",
		Version:  "v1",
		Resource: "patterns",
	}

	unstructuredList, err := k.client.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list patterns from cluster: %w", err)
	}

	patterns := make(map[string][]byte)
	for _, item := range unstructuredList.Items {
		// Convertiamo l'oggetto Unstructured in JSON.
		// Il tuo linter usa yaml.Unmarshal, che supporta nativamente anche il JSON!
		data, err := json.Marshal(item.Object)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal pattern %s: %w", item.GetName(), err)
		}
		patterns[item.GetName()+".json"] = data
	}

	return patterns, nil
}
