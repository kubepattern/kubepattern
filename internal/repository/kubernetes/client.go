package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Client struct {
	discoveryClient discovery.DiscoveryInterface
	dynamicClient   dynamic.Interface
}

func NewClient(config *rest.Config) (*Client, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{
		discoveryClient: disco,
		dynamicClient:   dyn,
	}, nil
}

// GetAllResources returns a flat slice of all objects in the cluster
func (c *Client) GetAllResources(ctx context.Context) ([]unstructured.Unstructured, error) {
	var allObjects []unstructured.Unstructured

	// 1. Discover all API groups and resources
	resourceLists, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed discovery: %w", err)
	}

	for _, list := range resourceLists {
		gv, _ := schema.ParseGroupVersion(list.GroupVersion)

		for _, resource := range list.APIResources {
			// Skip subresources (e.g., pods/log) and resources we can't list
			if !contains(resource.Verbs, "list") || isSubresource(resource.Name) {
				continue
			}

			gvr := gv.WithResource(resource.Name)

			// 2. Fetch all instances of this resource
			list, err := c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				// We log and continue because we might not have permissions for everything
				continue
			}

			allObjects = append(allObjects, list.Items...)
		}
	}
	return allObjects, nil
}

func isSubresource(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			return true
		}
	}
	return false
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
