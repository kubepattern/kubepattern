package kube

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ReadAllDefinitions retrieves all "patterns" resources in the cluster and maps their names to their JSON-encoded data.
func (c *Client) ReadAllDefinitions(ctx context.Context) (map[string][]byte, error) {
	gvr := schema.GroupVersionResource{
		Group:    "kubepattern.dev",
		Version:  "v1",
		Resource: "patterns",
	}

	items, err := c.listResource(ctx, gvr, metav1.NamespaceAll)
	if err != nil {
		return nil, fmt.Errorf("failed to list patterns from cluster: %w", err)
	}

	patterns := make(map[string][]byte)
	for _, item := range items {
		// Unstructured to JSON conversion.
		data, err := json.Marshal(item.Object)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal pattern %s: %w", item.GetName(), err)
		}
		patterns[item.GetName()+".json"] = data
	}

	return patterns, nil
}

// InstalledPatternUIDs returns the UIDs of every Pattern read from the cluster, including
// patterns that fail linting or are skipped: their smells must not be treated as orphans.
func InstalledPatternUIDs(raw map[string][]byte) map[string]struct{} {
	uids := make(map[string]struct{}, len(raw))
	for _, data := range raw {
		var p struct {
			Metadata struct {
				UID string `json:"uid"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(data, &p); err == nil && p.Metadata.UID != "" {
			uids[p.Metadata.UID] = struct{}{}
		}
	}
	return uids
}
