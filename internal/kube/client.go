package kube

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Client wraps the Kubernetes discovery and dynamic clients.
type Client struct {
	discoveryClient discovery.DiscoveryInterface
	dynamicClient   dynamic.Interface
	cached          []schema.GroupVersionResource
}

func NewClient(config *rest.Config) (*Client, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return &Client{
		discoveryClient: disco,
		dynamicClient:   dyn,
	}, nil
}

// DynamicClient returns the underlying dynamic client interface.
func (c *Client) DynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// FetchAll returns a flat slice of all listable objects in the cluster.
// Discovery errors from individual API groups are logged and skipped rather than
// aborting the entire scan — this is intentional: clusters with unhealthy CRD
// extensions still return partial results from ServerPreferredResources.
func (c *Client) FetchAll(ctx context.Context) ([]unstructured.Unstructured, error) {
	return c.getResources(ctx, metav1.NamespaceAll)
}

// FetchByNamespace returns all listable objects within a specific namespace.
func (c *Client) FetchByNamespace(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	return c.getResources(ctx, namespace)
}

func (c *Client) getResources(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	// ServerPreferredResources may return a partial result alongside an error
	// (e.g., when some API extensions are unavailable). We log the error but
	// continue with whatever was successfully discovered.
	apiGroups, discoveryErr := c.discoveryClient.ServerPreferredResources()
	if discoveryErr != nil {
		slog.Warn("partial discovery failure; some resource types may be missing",
			"error", discoveryErr)
	}
	if apiGroups == nil {
		return nil, fmt.Errorf("discovery returned no API groups: %w", discoveryErr)
	}

	var allObjects []unstructured.Unstructured

	for _, apiGroup := range apiGroups {
		gv, err := schema.ParseGroupVersion(apiGroup.GroupVersion)
		if err != nil {
			slog.Warn("skipping unparseable GroupVersion",
				"groupVersion", apiGroup.GroupVersion, "error", err)
			continue
		}

		for _, resource := range apiGroup.APIResources {
			if !canList(resource.Verbs) || isSubresource(resource.Name) {
				continue
			}

			gvr := gv.WithResource(resource.Name)
			items, err := c.listResource(ctx, gvr, namespace)
			if err != nil {
				// Log at debug level: permission gaps are expected in most clusters
				slog.Debug("skipping resource",
					"gvr", gvr.String(), "namespace", namespace, "error", err)
				continue
			}

			allObjects = append(allObjects, items...)
		}
	}

	return allObjects, nil
}

// listResource fetches all instances of a single GVR, optionally scoped to a namespace.
func (c *Client) listResource(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]unstructured.Unstructured, error) {
	var result *unstructured.UnstructuredList
	var err error

	if namespace == metav1.NamespaceAll {
		result, err = c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// isSubresource reports whether a resource name refers to a subresource (e.g. "pods/log").
func isSubresource(name string) bool {
	return strings.Contains(name, "/")
}

// canList reports whether the resource supports the "list" verb.
func canList(verbs []string) bool {
	for _, v := range verbs {
		if v == "list" {
			return true
		}
	}
	return false
}

type Resource struct {
	APIVersion string
	Kind       string
	Resource   string
}

func (c *Client) cachedGVR(gvr schema.GroupVersionResource) bool {
	for _, r := range c.cached {
		if gvr.Group == r.Group && gvr.Version == r.Version && gvr.Resource == r.Resource {
			return true
		}
	}

	return false
}

func (c *Client) FetchSelected(resources []Resource, ctx context.Context) ([]unstructured.Unstructured, error) {
	var allObjects []unstructured.Unstructured
	for _, resource := range resources {
		gvk := schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind)

		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: resource.Resource,
		}

		if c.cachedGVR(gvr) {
			continue
		}

		list, err := c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, err
		}

		allObjects = append(allObjects, list.Items...)
	}

	return allObjects, nil
}

func (c *Client) FetchSelectedWithInheritance(resources []Resource, ctx context.Context) ([]unstructured.Unstructured, error) {
	var allObjects []unstructured.Unstructured
	for _, resource := range resources {
		gvk := schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind)

		gvr := schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: resource.Resource,
		}

		if c.cachedGVR(gvr) {
			continue
		}

		list, err := c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, err
		}

		for _, r := range list.Items {
			owners := r.GetOwnerReferences()
			for _, owner := range owners {
				gvk := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
				gvr := schema.GroupVersionResource{
					Group:    gvk.Group,
					Version:  gvk.Version,
					Resource: owner.OpenAPIModelName(),
				}
				if c.cachedGVR(gvr) {
					break
				} else {
					var inherited []Resource
					inherited = append(inherited, Resource{
						APIVersion: owner.APIVersion,
						Kind:       owner.Kind,
						Resource:   owner.OpenAPIModelName(),
					})
					var fetched, err = c.FetchSelectedWithInheritance(inherited, ctx)
					if err != nil {
						return nil, err
					}
					allObjects = append(allObjects, fetched...)
				}
			}
		}

		allObjects = append(allObjects, list.Items...)
	}

	return allObjects, nil
}
