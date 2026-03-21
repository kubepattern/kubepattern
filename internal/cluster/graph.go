package cluster

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Graph struct {
	// Nodes keyed by UID
	Nodes map[string]unstructured.Unstructured
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]unstructured.Unstructured),
	}
}

// Build populates the graph nodes from a fresh scan
func (g *Graph) Build(resources []unstructured.Unstructured) {
	// Reset the map for a fresh analysis
	g.Nodes = make(map[string]unstructured.Unstructured)

	for _, res := range resources {
		uid := string(res.GetUID())
		if uid != "" {
			g.Nodes[uid] = res
		}
	}
}
