package cluster

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type Graph struct {
	nodes map[types.UID]*unstructured.Unstructured
	edges map[types.UID][]edge
}

type edge struct {
	fromNode types.UID
	toNode   types.UID
	reason   string
}

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

func (g *Graph) GetNodes() map[types.UID]*unstructured.Unstructured {
	return g.nodes
}

func (g *Graph) link() {
	for fromUID, fromNode := range g.nodes {
		for toUID, toNode := range g.nodes {
			if fromUID == toUID {
				continue
			}
			owners := toNode.GetOwnerReferences()
			for _, owner := range owners {
				if owner.UID == fromUID {
					reason := fromNode.GetKind() + " owns " + toNode.GetKind()
					g.addEdge(fromUID, toUID, reason)
					break
				}
			}
		}
	}
}

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
