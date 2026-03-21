package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ClusterClient struct {
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient
}

func NewClient(config *rest.Config) (*ClusterClient, error) {
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return &ClusterClient{
		dynamicClient:   dyn,
		discoveryClient: disco,
	}, nil
}

// GetAllResources fetches everything the tool has permission to see
func (c *ClusterClient) GetAllResources(ctx context.Context) error {
	// 1. Discover all API groups and resources (including CRDs)
	resourceLists, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, list := range resourceLists {
		gv, _ := schema.ParseGroupVersion(list.GroupVersion)

		for _, resource := range list.APIResources {
			// Filter: skip subresources (like pods/log) and ensure we can "list" them
			if !contains(resource.Verbs, "list") || resource.Name == "" || isSubresource(resource.Name) {
				continue
			}

			gvr := gv.WithResource(resource.Name)

			// 2. Fetch the data dynamically
			list, err := c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				fmt.Printf("Skipping %s: %v\n", gvr.String(), err)
				continue
			}

			for _, item := range list.Items {
				fmt.Printf("Found: %s/%s in %s\n", item.GetKind(), item.GetName(), item.GetNamespace())
				// Pass item to your analyzer here
			}
		}
	}
	return nil
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func isSubresource(name string) bool {
	return len(name) > 0 && (contains([]string{"status", "exec", "log", "proxy"}, name) || (len(name) > 0 && name[0] == '/'))
}
