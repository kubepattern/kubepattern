package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// ResourceFetcher is the interface that internal/repository/kubernetes/client.go will implement.
type ResourceFetcher interface {
	FetchAll(ctx context.Context) ([]unstructured.Unstructured, error)
	FetchByNamespace(ctx context.Context, namespace string) ([]unstructured.Unstructured, error)
}

// Graph is the Kubernetes cluster abstraction
type Graph struct {
	nodes map[types.UID]*unstructured.Unstructured
	edges map[types.UID][]edge
}

// edge represents the relationship between two nodes
type edge struct {
	fromNode types.UID
	toNode   types.UID
	reason   string
}

// NewGraph creates an instance of a Graph
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[types.UID]*unstructured.Unstructured),
		edges: make(map[types.UID][]edge),
	}
}

// Build populates the graph nodes from a fresh scan
func (g *Graph) Build(resources []unstructured.Unstructured) {
	// Reset the map for a fresh analysis
	clear(g.nodes)
	clear(g.edges)

	for i := range resources {
		res := &resources[i]
		uid := res.GetUID()
		if uid != "" {
			g.nodes[uid] = res
		}
	}

	g.link()
}

// GetNodes exposes a Graph's nodes to other packages
func (g *Graph) GetNodes() map[types.UID]*unstructured.Unstructured {
	return g.nodes
}

func (g *Graph) GetEdges() map[types.UID][]edge {
	return g.edges
}

func (g *Graph) isParentOwner(child, parent types.UID) bool {
	owned, exists := g.nodes[child]
	if !exists || owned == nil {
		return false
	}

	for _, r := range owned.GetOwnerReferences() {
		// direct owner
		if r.UID == parent {
			return true
		}

		if g.isParentOwner(r.UID, parent) {
			return true
		}
	}

	return false
}

// link checks for edges between nodes in Graph based on the Kubernetes ownership mechanism
func (g *Graph) link() {
	slog.Info("Linking graph using ownership")

	for toUID, toNode := range g.nodes {
		owners := toNode.GetOwnerReferences()

		for _, owner := range owners {
			// Se l'owner esiste nei nostri nodi, creiamo l'arco
			if fromNode, exists := g.nodes[owner.UID]; exists {
				reason := fromNode.GetKind() + " owns " + toNode.GetKind()
				g.addEdge(owner.UID, toUID, reason)
			}
		}
	}
	slog.Info("Linking graph complete")
}

// addEdge create an edge between two nodes
func (g *Graph) addEdge(from, to types.UID, reason string) {
	g.edges[from] = append(g.edges[from], edge{
		fromNode: from,
		toNode:   to,
		reason:   reason,
	})
}

// PrintTerminal prints graph in ASCII format as a tree
func (g *Graph) PrintTerminal() {
	fmt.Println("\n=== Kubernetes Resource Graph ===")

	for fromUID, node := range g.nodes {
		kind := node.GetKind()
		name := node.GetName()
		fmt.Printf("📦 [%s] %s\n", kind, name)

		if edges, ok := g.edges[fromUID]; ok && len(edges) > 0 {
			for i, e := range edges {
				if targetNode, exists := g.nodes[e.toNode]; exists {
					prefix := " ├──"
					if i == len(edges)-1 {
						prefix = " └──"
					}
					fmt.Printf("%s [%s] %s (Reason: %s)\n",
						prefix, targetNode.GetKind(), targetNode.GetName(), e.reason)
				}
			}
		}
	}
	fmt.Println(strings.Repeat("=", 33))
}

// PrintGraphviz generates output in DOT format for image generation
func (g *Graph) PrintGraphviz() {
	fmt.Println("digraph KubernetesCluster {")
	fmt.Println("  rankdir=LR;")
	fmt.Println("  node [shape=box, style=filled, fillcolor=lightgrey, fontname=\"Helvetica\"];")

	// 1. Defining all nodes
	for uid, node := range g.nodes {
		// Kind and Name label
		label := fmt.Sprintf("%s\\n%s", node.GetKind(), node.GetName())
		// Print: "uid-del-nodo" [label="Deployment\nmy-app"];
		fmt.Printf("  \"%s\" [label=\"%s\"];\n", uid, label)
	}

	fmt.Println("")

	// 2. Drawing edges
	for fromUID, edges := range g.edges {
		for _, e := range edges {
			// Print: "uid-parent" -> "uid-child" [label="owns"];
			fmt.Printf("  \"%s\" -> \"%s\" [label=\"%s\", fontsize=10];\n",
				fromUID, e.toNode, e.reason)
		}
	}

	fmt.Println("}")
}
